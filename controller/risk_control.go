package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	riskBatchActionDisable = "disable"
	riskBatchActionEnable  = "enable"

	// riskBatchMaxUserIds 是直接按 user_ids 批量操作时单次请求的最大 id 数。
	riskBatchMaxUserIds = 2000

	riskBatchScopeUserIds  = "user_ids"
	riskBatchScopeSharedIP = "shared_ip"
	riskBatchScopeInviter  = "inviter"
)

var (
	riskSharedIPSortFields = map[string]struct{}{
		"user_count":    {},
		"event_count":   {},
		"last_seen_at":  {},
		"first_seen_at": {},
		"ip":            {},
	}
	riskInviterSortFields = map[string]struct{}{
		"direct_invite_count": {},
		"inviter_id":          {},
		"created_at":          {},
		"aff_quota":           {},
	}
)

// normalizeRiskSortBy 只允许白名单内的排序字段，非法值回退默认值。
func normalizeRiskSortBy(raw string, allowed map[string]struct{}, fallback string) string {
	raw = strings.TrimSpace(raw)
	if _, ok := allowed[raw]; ok {
		return raw
	}
	return fallback
}

// normalizeRiskOrder 只接受 asc | desc，其余回退 desc。
func normalizeRiskOrder(raw string) string {
	if strings.TrimSpace(raw) == "asc" {
		return "asc"
	}
	return "desc"
}

func normalizeRiskBatchAction(raw string) (string, bool) {
	action := strings.TrimSpace(raw)
	if action == riskBatchActionDisable || action == riskBatchActionEnable {
		return action, true
	}
	return "", false
}

// dedupeRiskBatchUserIds 去重并丢弃非法 id，保持首次出现顺序。
func dedupeRiskBatchUserIds(userIds []int) []int {
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

func GetRiskControlOverview(c *gin.Context) {
	overview, err := model.GetRiskOverview()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, overview)
}

func GetRiskControlSharedIPs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	minUsers, _ := strconv.Atoi(c.Query("min_users"))
	sortBy := normalizeRiskSortBy(c.Query("sort_by"), riskSharedIPSortFields, "last_seen_at")
	order := normalizeRiskOrder(c.Query("order"))
	items, total, err := model.ListRiskSharedIPs(
		strings.TrimSpace(c.Query("source")),
		strings.TrimSpace(c.DefaultQuery("search_type", "ip")),
		strings.TrimSpace(c.Query("keyword")),
		minUsers,
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
		sortBy,
		order,
		strings.TrimSpace(c.Query("ban_state")),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetRiskControlInviters(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	sortBy := normalizeRiskSortBy(c.Query("sort_by"), riskInviterSortFields, "direct_invite_count")
	order := normalizeRiskOrder(c.Query("order"))
	items, total, err := model.ListRiskInviters(
		strings.TrimSpace(c.Query("keyword")),
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
		sortBy,
		order,
		strings.TrimSpace(c.Query("ban_state")),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// GetRiskControlInviterInvitees 返回指定邀请人的直接被邀请用户分页列表。
// id 非法或不存在时返回空列表（不返回错误），由前端展示空态。
func GetRiskControlInviterInvitees(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	inviterId, err := strconv.Atoi(c.Param("id"))
	if err != nil || inviterId <= 0 {
		pageInfo.SetTotal(0)
		pageInfo.SetItems([]*model.RiskUserSummary{})
		common.ApiSuccess(c, pageInfo)
		return
	}
	items, total, err := model.ListRiskInvitees(inviterId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetRiskControlUser(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiError(c, errors.New("invalid user id"))
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	records, err := model.ListRiskIPRecordsByUser(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"id":            user.Id,
		"username":      user.Username,
		"display_name":  user.DisplayName,
		"email":         user.Email,
		"status":        user.Status,
		"quota":         user.Quota,
		"group":         user.Group,
		"inviter_id":    user.InviterId,
		"last_login_at": user.LastLoginAt,
		"last_login_ip": user.LastLoginIp,
		"ip_records":    records,
	})
}

type riskBatchStatusRequest struct {
	Action  string `json:"action"`
	UserIds []int  `json:"user_ids"`
	IP      string `json:"ip"`
	Source  string `json:"source"`
}

type riskBatchStatusResponse struct {
	Action    string                 `json:"action"`
	Requested int                    `json:"requested"`
	Updated   int                    `json:"updated"`
	Skipped   []model.RiskStatusSkip `json:"skipped"`
}

func riskBatchAuditAction(action string) string {
	if action == riskBatchActionEnable {
		return "risk.batch_enable"
	}
	return "risk.batch_disable"
}

// respondRiskBatchStatus 记录一条管理员审计日志并返回批量操作结果。
// auditParams 只放统计/范围信息（不写邮箱、不写完整 id 列表）。
func respondRiskBatchStatus(c *gin.Context, action string, requested int, updated int, skipped []model.RiskStatusSkip, scope string, auditParams map[string]interface{}) {
	if skipped == nil {
		skipped = make([]model.RiskStatusSkip, 0)
	}
	params := map[string]interface{}{
		"count":     updated,
		"requested": requested,
		"skipped":   len(skipped),
		"scope":     scope,
	}
	for key, value := range auditParams {
		params[key] = value
	}
	recordManageAudit(c, riskBatchAuditAction(action), params)
	common.ApiSuccess(c, riskBatchStatusResponse{
		Action:    action,
		Requested: requested,
		Updated:   updated,
		Skipped:   skipped,
	})
}

// BatchRiskControlUserStatus 按显式 user_ids 批量封禁 / 解禁账号。
func BatchRiskControlUserStatus(c *gin.Context) {
	var req riskBatchStatusRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	action, ok := normalizeRiskBatchAction(req.Action)
	if !ok {
		common.ApiErrorMsg(c, "invalid action, expected disable or enable")
		return
	}
	userIds := dedupeRiskBatchUserIds(req.UserIds)
	if len(userIds) == 0 {
		common.ApiErrorMsg(c, "user_ids is required")
		return
	}
	if len(userIds) > riskBatchMaxUserIds {
		common.ApiErrorMsg(c, "user_ids exceeds the limit of 2000")
		return
	}
	updated, skipped, err := model.BatchUpdateRiskUserStatus(c.GetInt("role"), action, userIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	respondRiskBatchStatus(c, action, len(userIds), updated, skipped, riskBatchScopeUserIds, map[string]interface{}{
		"user_ids_count": len(userIds),
	})
}

// BatchRiskControlSharedIPStatus 对指定 IP（可限定来源）关联的全部账号批量封禁 / 解禁。
func BatchRiskControlSharedIPStatus(c *gin.Context) {
	var req riskBatchStatusRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	action, ok := normalizeRiskBatchAction(req.Action)
	if !ok {
		common.ApiErrorMsg(c, "invalid action, expected disable or enable")
		return
	}
	ip := model.NormalizeRiskIP(req.IP)
	if ip == "" {
		common.ApiErrorMsg(c, "invalid ip")
		return
	}
	source := strings.TrimSpace(req.Source)
	if source != "" && source != model.RiskIPSourceLogin && source != model.RiskIPSourceToken {
		common.ApiErrorMsg(c, "invalid source, expected login or token")
		return
	}
	userIds, err := model.ResolveRiskUserIDsBySharedIP(ip, source)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(userIds) == 0 {
		common.ApiSuccess(c, riskBatchStatusResponse{
			Action:  action,
			Skipped: make([]model.RiskStatusSkip, 0),
		})
		return
	}
	updated, skipped, err := model.BatchUpdateRiskUserStatus(c.GetInt("role"), action, userIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	respondRiskBatchStatus(c, action, len(userIds), updated, skipped, riskBatchScopeSharedIP, map[string]interface{}{
		"ip":     ip,
		"source": source,
	})
}

// BatchRiskControlInviterStatus 对指定邀请人的全部被邀请账号批量封禁 / 解禁。
func BatchRiskControlInviterStatus(c *gin.Context) {
	inviterId, err := strconv.Atoi(c.Param("id"))
	if err != nil || inviterId <= 0 {
		common.ApiErrorMsg(c, "invalid inviter id")
		return
	}
	var req riskBatchStatusRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	action, ok := normalizeRiskBatchAction(req.Action)
	if !ok {
		common.ApiErrorMsg(c, "invalid action, expected disable or enable")
		return
	}
	userIds, err := model.ResolveRiskUserIDsByInviter(inviterId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(userIds) == 0 {
		common.ApiSuccess(c, riskBatchStatusResponse{
			Action:  action,
			Skipped: make([]model.RiskStatusSkip, 0),
		})
		return
	}
	updated, skipped, err := model.BatchUpdateRiskUserStatus(c.GetInt("role"), action, userIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	respondRiskBatchStatus(c, action, len(userIds), updated, skipped, riskBatchScopeInviter, map[string]interface{}{
		"inviter_id": inviterId,
	})
}
