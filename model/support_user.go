package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"gorm.io/gorm"
)

type SupportManagedUser struct {
	Id                 int    `json:"id"`
	Username           string `json:"username"`
	DisplayName        string `json:"display_name"`
	Quota              int    `json:"quota"`
	UsedQuota          int    `json:"used_quota"`
	InviterId          int    `json:"inviter_id"`
	InviterUsername    string `json:"inviter_username"`
	InviterDisplayName string `json:"inviter_display_name"`
	IsAgent            bool   `json:"is_agent"`
}

type SupportQuotaDecreaseResult struct {
	UserId      int `json:"user_id"`
	QuotaDelta  int `json:"quota_delta"`
	QuotaBefore int `json:"quota_before"`
	QuotaAfter  int `json:"quota_after"`
}

func setSupportManagedUserAgentFlags(users []*SupportManagedUser) error {
	userIds := make([]int, 0, len(users))
	for _, user := range users {
		userIds = append(userIds, user.Id)
	}
	if len(userIds) == 0 {
		return nil
	}
	if !DB.Migrator().HasTable(&AgentProfile{}) {
		return nil
	}

	var agentUserIds []int
	if err := DB.Model(&AgentProfile{}).
		Where("user_id IN ?", userIds).
		Pluck("user_id", &agentUserIds).Error; err != nil {
		return err
	}
	agents := make(map[int]struct{}, len(agentUserIds))
	for _, userId := range agentUserIds {
		agents[userId] = struct{}{}
	}
	for _, user := range users {
		_, user.IsAgent = agents[user.Id]
	}
	return nil
}

func SearchSupportManagedUsers(keyword string, pageInfo *common.PageInfo) ([]*SupportManagedUser, int64, error) {
	users := make([]*SupportManagedUser, 0)
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return users, 0, nil
	}

	pattern := strings.ReplaceAll(keyword, "!", "!!")
	pattern = strings.ReplaceAll(pattern, "%", "!%")
	pattern = strings.ReplaceAll(pattern, "_", "!_")
	pattern = "%" + pattern + "%"

	query := DB.Model(&User{}).
		Joins("LEFT JOIN users AS inviter ON inviter.id = users.inviter_id").
		Where("users.role = ?", common.RoleCommonUser)
	likeCondition := "users.username LIKE ? ESCAPE '!' OR users.email LIKE ? ESCAPE '!' OR users.display_name LIKE ? ESCAPE '!'"
	if id, err := strconv.Atoi(keyword); err == nil {
		query = query.Where("(users.id = ? OR "+likeCondition+")", id, pattern, pattern, pattern)
	} else {
		query = query.Where("("+likeCondition+")", pattern, pattern, pattern)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.
		Select("users.id, users.username, users.display_name, users.quota, users.used_quota, users.inviter_id, COALESCE(inviter.username, '') AS inviter_username, COALESCE(inviter.display_name, '') AS inviter_display_name").
		Order("users.id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&users).Error; err != nil {
		return nil, 0, err
	}
	if err := setSupportManagedUserAgentFlags(users); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func GetAgentAssignedUsers(keyword string, agentUserId int, pageInfo *common.PageInfo) ([]*SupportManagedUser, int64, error) {
	users := make([]*SupportManagedUser, 0)
	keyword = strings.TrimSpace(keyword)
	query := DB.Model(&User{}).
		Joins("LEFT JOIN users AS inviter ON inviter.id = users.inviter_id").
		Where("users.role = ? AND users.inviter_id > ?", common.RoleCommonUser, 0)
	if agentUserId > 0 {
		query = query.Where("users.inviter_id = ?", agentUserId)
	}
	if keyword != "" {
		pattern := strings.ReplaceAll(keyword, "!", "!!")
		pattern = strings.ReplaceAll(pattern, "%", "!%")
		pattern = strings.ReplaceAll(pattern, "_", "!_")
		pattern = "%" + pattern + "%"
		likeCondition := "users.username LIKE ? ESCAPE '!' OR users.email LIKE ? ESCAPE '!' OR users.display_name LIKE ? ESCAPE '!' OR inviter.username LIKE ? ESCAPE '!' OR inviter.display_name LIKE ? ESCAPE '!'"
		if id, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("(users.id = ? OR users.inviter_id = ? OR "+likeCondition+")", id, id, pattern, pattern, pattern, pattern, pattern)
		} else {
			query = query.Where("("+likeCondition+")", pattern, pattern, pattern, pattern, pattern)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.
		Select("users.id, users.username, users.display_name, users.quota, users.used_quota, users.inviter_id, COALESCE(inviter.username, '') AS inviter_username, COALESCE(inviter.display_name, '') AS inviter_display_name").
		Order("users.id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&users).Error; err != nil {
		return nil, 0, err
	}
	if err := setSupportManagedUserAgentFlags(users); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func GetSupportManagedUserById(id int) (*SupportManagedUser, error) {
	if id <= 0 {
		return nil, errors.New("invalid user id")
	}
	user := &SupportManagedUser{}
	err := DB.Model(&User{}).
		Joins("LEFT JOIN users AS inviter ON inviter.id = users.inviter_id").
		Select("users.id, users.username, users.display_name, users.quota, users.used_quota, users.inviter_id, COALESCE(inviter.username, '') AS inviter_username, COALESCE(inviter.display_name, '') AS inviter_display_name").
		Where("users.id = ? AND users.role = ?", id, common.RoleCommonUser).
		First(user).Error
	if err == nil {
		err = setSupportManagedUserAgentFlags([]*SupportManagedUser{user})
	}
	return user, err
}

func DecreaseUserQuotaBySupport(operatorUserId int, operatorRole int, targetUserId int, quota int, reason string) (*SupportQuotaDecreaseResult, error) {
	if operatorUserId <= 0 || targetUserId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if quota <= 0 {
		return nil, errors.New("quota must be greater than zero")
	}
	if !common.RoleHasPermission(operatorRole, common.PermissionUserQuotaDecrease) {
		return nil, errors.New("no permission to decrease user quota")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("adjustment reason is required")
	}

	result := &SupportQuotaDecreaseResult{UserId: targetUserId, QuotaDelta: -quota}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var target User
		if err := lockForUpdate(tx).Select("id", "role", "quota").First(&target, targetUserId).Error; err != nil {
			return err
		}
		if target.Role != common.RoleCommonUser {
			return errors.New("support users can only decrease common user quota")
		}
		if operatorRole != common.RoleRootUser && operatorRole <= target.Role {
			return errors.New("no permission to manage this user")
		}
		if target.Quota < quota {
			return errors.New("user quota is insufficient")
		}
		result.QuotaBefore = target.Quota
		result.QuotaAfter = target.Quota - quota
		update := tx.Model(&User{}).
			Where("id = ? AND role = ? AND quota = ?", targetUserId, common.RoleCommonUser, target.Quota).
			Update("quota", result.QuotaAfter)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errors.New("user quota changed, please refresh and retry")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := updateUserQuotaCache(targetUserId, result.QuotaAfter); err != nil {
		common.SysLog("failed to update user quota cache: " + err.Error())
	}
	RecordLog(targetUserId, LogTypeManage, fmt.Sprintf("support operator %d decreased user quota, before: %s, after: %s, delta: %s, reason: %s", operatorUserId, logger.LogQuota(result.QuotaBefore), logger.LogQuota(result.QuotaAfter), logger.LogQuota(-quota), reason))
	return result, nil
}
