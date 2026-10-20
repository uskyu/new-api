package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// 批量状态更新时单条 SQL 的 id 数量上限。SQLite 的绑定变量数量有限制，
// 分块后同样适用于 MySQL / PostgreSQL，且能控制单条语句大小。
const riskStatusBatchChunkSize = 500

// 批量封禁/解禁的跳过原因。前端按 reason 展示提示，无需额外文案映射。
const (
	RiskStatusSkipRootUser              = "root_user"
	RiskStatusSkipInsufficientPrivilege = "insufficient_privilege"
	RiskStatusSkipNotFound              = "not_found"
	RiskStatusSkipAlreadyDisabled       = "already_disabled"
	RiskStatusSkipAlreadyEnabled        = "already_enabled"
)

// RiskStatusSkip 描述批量状态更新中未被处理的账号及原因。
type RiskStatusSkip struct {
	UserId   int    `json:"user_id"`
	Username string `json:"username"`
	Reason   string `json:"reason"` // root_user / insufficient_privilege / not_found / already_disabled / already_enabled
}

// dedupeRiskUserIds 去重并丢弃非法 id，保持首次出现顺序。
func dedupeRiskUserIds(userIds []int) []int {
	seen := make(map[int]struct{}, len(userIds))
	result := make([]int, 0, len(userIds))
	for _, userId := range userIds {
		if userId <= 0 {
			continue
		}
		if _, ok := seen[userId]; ok {
			continue
		}
		seen[userId] = struct{}{}
		result = append(result, userId)
	}
	return result
}

// loadRiskUsersByIDs 分块加载目标用户（软删除用户视为不存在）。
func loadRiskUsersByIDs(userIds []int) (map[int]*User, error) {
	users := make(map[int]*User, len(userIds))
	for start := 0; start < len(userIds); start += riskStatusBatchChunkSize {
		end := start + riskStatusBatchChunkSize
		if end > len(userIds) {
			end = len(userIds)
		}
		var chunk []*User
		if err := DB.Select("id, username, role, status").
			Where("id IN ?", userIds[start:end]).
			Find(&chunk).Error; err != nil {
			return nil, err
		}
		for _, user := range chunk {
			users[user.Id] = user
		}
	}
	return users, nil
}

// BatchUpdateRiskUserStatus 按操作者角色批量封禁（disable）或解禁（enable）用户。
//
// 过滤规则（服务端二次校验，前端选择仅作交互）：
//   - root 账号永远跳过（root_user）；
//   - 非 root 操作者只能操作角色低于自己的账号（insufficient_privilege）；
//   - 目标不存在（含软删除）跳过（not_found）；
//   - 状态已满足时幂等跳过（already_disabled / already_enabled）。
//
// 更新使用 `WHERE id IN ? AND status = ?` 条件更新，按 500 个 id 分块并放在
// 同一个事务中；updated 以实际 RowsAffected 为准。事务提交后清理这些用户与
// 令牌的 Redis 缓存（Redis 未启用时为空操作），使被封禁用户的请求立即 403。
func BatchUpdateRiskUserStatus(operatorRole int, action string, userIds []int) (int, []RiskStatusSkip, error) {
	skipped := make([]RiskStatusSkip, 0)
	var targetStatus int
	var fromStatus int
	switch action {
	case "disable":
		targetStatus = common.UserStatusDisabled
		fromStatus = common.UserStatusEnabled
	case "enable":
		targetStatus = common.UserStatusEnabled
		fromStatus = common.UserStatusDisabled
	default:
		return 0, skipped, fmt.Errorf("invalid risk-control batch action: %s", action)
	}

	ids := dedupeRiskUserIds(userIds)
	if len(ids) == 0 {
		return 0, skipped, nil
	}
	users, err := loadRiskUsersByIDs(ids)
	if err != nil {
		return 0, skipped, err
	}

	eligibleIds := make([]int, 0, len(ids))
	for _, userId := range ids {
		user := users[userId]
		if user == nil {
			skipped = append(skipped, RiskStatusSkip{UserId: userId, Reason: RiskStatusSkipNotFound})
			continue
		}
		if user.Role == common.RoleRootUser {
			skipped = append(skipped, RiskStatusSkip{UserId: userId, Username: user.Username, Reason: RiskStatusSkipRootUser})
			continue
		}
		// 与 controller 的 canManageTargetRole 语义保持一致：root 可操作所有非 root
		// 账号，其他管理员只能操作角色严格低于自己的账号。
		if operatorRole != common.RoleRootUser && user.Role >= operatorRole {
			skipped = append(skipped, RiskStatusSkip{UserId: userId, Username: user.Username, Reason: RiskStatusSkipInsufficientPrivilege})
			continue
		}
		if user.Status == targetStatus {
			reason := RiskStatusSkipAlreadyEnabled
			if targetStatus == common.UserStatusDisabled {
				reason = RiskStatusSkipAlreadyDisabled
			}
			skipped = append(skipped, RiskStatusSkip{UserId: userId, Username: user.Username, Reason: reason})
			continue
		}
		eligibleIds = append(eligibleIds, userId)
	}
	if len(eligibleIds) == 0 {
		return 0, skipped, nil
	}

	updated := 0
	err = DB.Transaction(func(tx *gorm.DB) error {
		for start := 0; start < len(eligibleIds); start += riskStatusBatchChunkSize {
			end := start + riskStatusBatchChunkSize
			if end > len(eligibleIds) {
				end = len(eligibleIds)
			}
			result := tx.Model(&User{}).
				Where("id IN ? AND status = ?", eligibleIds[start:end], fromStatus).
				Update("status", targetStatus)
			if result.Error != nil {
				return result.Error
			}
			updated += int(result.RowsAffected)
		}
		return nil
	})
	if err != nil {
		return 0, skipped, err
	}

	// 事务提交后再清理缓存：失败只记录日志，不影响本次响应。
	InvalidateUsersCacheAndTokens(eligibleIds)
	return updated, skipped, nil
}

// InvalidateUsersCacheAndTokens 清理用户缓存及其全部令牌缓存。
// Redis 未启用时直接返回；单个用户清理失败只记录日志，不中断其余用户。
func InvalidateUsersCacheAndTokens(userIds []int) {
	if !common.RedisEnabled || len(userIds) == 0 {
		return
	}
	for _, userId := range dedupeRiskUserIds(userIds) {
		if err := invalidateUserCache(userId); err != nil {
			common.SysLog(fmt.Sprintf("failed to invalidate user cache for user %d: %s", userId, err.Error()))
		}
		if err := InvalidateUserTokensCache(userId); err != nil {
			common.SysLog(fmt.Sprintf("failed to invalidate tokens cache for user %d: %s", userId, err.Error()))
		}
	}
}

// ResolveRiskUserIDsBySharedIP 从 risk_ip_records 解析指定 IP 关联的 distinct 用户 id。
// source 为空表示全部来源，否则只接受 login / token。
func ResolveRiskUserIDsBySharedIP(ip string, source string) ([]int, error) {
	normalizedIP := NormalizeRiskIP(ip)
	if normalizedIP == "" {
		return []int{}, nil
	}
	tx := DB.Model(&RiskIPRecord{}).Where("ip = ?", normalizedIP)
	if source == RiskIPSourceLogin || source == RiskIPSourceToken {
		tx = tx.Where("source = ?", source)
	}
	ids := make([]int, 0)
	if err := tx.Distinct("user_id").Order("user_id ASC").Pluck("user_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// ResolveRiskUserIDsByInviter 解析指定邀请人的全部直接被邀请用户 id。
func ResolveRiskUserIDsByInviter(inviterId int) ([]int, error) {
	if inviterId <= 0 {
		return []int{}, nil
	}
	ids := make([]int, 0)
	if err := DB.Model(&User{}).
		Where("inviter_id = ?", inviterId).
		Order("id ASC").
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
