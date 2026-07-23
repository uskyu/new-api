package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

const maxAgentWithdrawImportFileSize int64 = 5 << 20

type InitializeAgentModuleRequest struct {
	DefaultRebateRate int `json:"default_rebate_rate"`
}

type UpsertAgentProfileRequest struct {
	UserId        int    `json:"user_id"`
	Status        int    `json:"status"`
	RebateGroupId int    `json:"rebate_group_id"`
	CustomRate    int    `json:"custom_rate"`
	Remark        string `json:"remark"`
}

type AdjustAgentBalanceRequest struct {
	AgentUserId int    `json:"agent_user_id"`
	Amount      string `json:"amount"`
	Reason      string `json:"reason"`
}

type UpsertAgentPromoLinkRequest struct {
	Id          int    `json:"id"`
	AgentUserId int    `json:"agent_user_id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Status      int    `json:"status"`
	LandingPage string `json:"landing_page"`
	Remark      string `json:"remark"`
}

type UpsertAgentRebateGroupRequest struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	RebateRate int    `json:"rebate_rate"`
	Status     int    `json:"status"`
	Remark     string `json:"remark"`
}

type AgentUpgradeRequestPayload struct {
	TargetUserId int    `json:"target_user_id"`
	TargetRate   int    `json:"target_rate"`
	Remark       string `json:"remark"`
}

type AgentUpgradeReviewPayload struct {
	Approve    bool   `json:"approve"`
	TargetRate int    `json:"target_rate"`
	Remark     string `json:"remark"`
}

type AgentWithdrawRequestPayload struct {
	AccountName string `json:"account_name"`
	AccountNo   string `json:"account_no"`
	Amount      string `json:"amount"`
	Remark      string `json:"remark"`
}

type TransferAgentDownlineUserRequest struct {
	SourceAgentUserId int    `json:"source_agent_user_id"`
	TargetAgentUserId int    `json:"target_agent_user_id"`
	DownlineUserId    int    `json:"downline_user_id"`
	PromoLinkId       int    `json:"promo_link_id"`
	Remark            string `json:"remark"`
}

type AssignAgentDownlineUserRequest struct {
	TargetAgentUserId int    `json:"target_agent_user_id"`
	DownlineUserId    int    `json:"downline_user_id"`
	PromoLinkId       int    `json:"promo_link_id"`
	Remark            string `json:"remark"`
}

type ChangeAgentDownlineUserRequest struct {
	TargetAgentUserId int    `json:"target_agent_user_id"`
	DownlineUserId    int    `json:"downline_user_id"`
	PromoLinkId       int    `json:"promo_link_id"`
	Remark            string `json:"remark"`
}

type SupportAgentProfileView struct {
	Id                  int    `json:"id"`
	UserId              int    `json:"user_id"`
	Username            string `json:"username"`
	DisplayName         string `json:"display_name"`
	Status              int    `json:"status"`
	RebateBalanceAmount int64  `json:"rebate_balance_amount"`
}

func writeAgentConflict(c *gin.Context, err error) bool {
	var conflictErr *model.AgentRateConflictError
	if !errors.As(err, &conflictErr) {
		return false
	}
	c.JSON(200, gin.H{
		"success": false,
		"message": conflictErr.Error(),
		"data": gin.H{
			"conflicts": conflictErr.Conflicts,
		},
	})
	return true
}

func GetAgentBootstrapStatus(c *gin.Context) {
	status, err := service.GetAgentBootstrapStatus()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func GetAgentAdminOverview(c *gin.Context) {
	overview, err := model.GetAgentAdminOverview()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, overview)
}

func GetAgentDailyMetrics(c *gin.Context) {
	agentUserId, _ := strconv.Atoi(c.Query("agent_user_id"))
	metrics, err := model.GetAgentDailyMetrics(agentUserId, c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, metrics)
}

func InitializeAgentModule(c *gin.Context) {
	var req InitializeAgentModuleRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	status, err := service.InitializeAgentModule(req.DefaultRebateRate)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func GetAgentRebateGroups(c *gin.Context) {
	groups, err := model.GetAllAgentRebateGroups()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, groups)
}

func UpsertAgentRebateGroup(c *gin.Context) {
	var req UpsertAgentRebateGroupRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	group, err := model.UpsertAgentRebateGroup(c.GetInt("id"), &model.AgentRebateGroup{
		Id:         req.Id,
		Name:       req.Name,
		RebateRate: req.RebateRate,
		Status:     req.Status,
		Remark:     req.Remark,
	})
	if err != nil {
		if writeAgentConflict(c, err) {
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, group)
}

func DeleteAgentRebateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid rebate group ID")
		return
	}
	if err := model.DeleteAgentRebateGroup(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, true)
}

func GetAgentProfiles(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")
	profiles, total, err := model.GetAgentProfiles(pageInfo, keyword)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if c.GetInt("role") < common.RoleAdminUser {
		safeProfiles := make([]*SupportAgentProfileView, 0, len(profiles))
		for _, profile := range profiles {
			safeProfiles = append(safeProfiles, &SupportAgentProfileView{
				Id:                  profile.Id,
				UserId:              profile.UserId,
				Username:            profile.Username,
				DisplayName:         profile.DisplayName,
				Status:              profile.Status,
				RebateBalanceAmount: profile.RebateBalanceAmount,
			})
		}
		pageInfo.SetTotal(int(total))
		pageInfo.SetItems(safeProfiles)
		common.ApiSuccess(c, pageInfo)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(profiles)
	common.ApiSuccess(c, pageInfo)
}

func UpsertAgentProfile(c *gin.Context) {
	var req UpsertAgentProfileRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	profile, err := model.UpsertAgentProfile(c.GetInt("id"), &model.AgentProfile{
		UserId:        req.UserId,
		Status:        req.Status,
		RebateGroupId: req.RebateGroupId,
		CustomRate:    req.CustomRate,
		Remark:        req.Remark,
	})
	if err != nil {
		if writeAgentConflict(c, err) {
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, profile)
}

func AdjustAgentBalance(c *gin.Context) {
	var req AdjustAgentBalanceRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	if c.GetInt("role") == common.RoleSupportUser && req.AgentUserId == c.GetInt("id") {
		common.ApiErrorMsg(c, "support users cannot adjust their own agent balance")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		common.ApiErrorMsg(c, "adjustment reason is required")
		return
	}
	amountDecimal, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		common.ApiErrorMsg(c, "invalid adjustment amount")
		return
	}
	deltaAmountValue := amountDecimal.Mul(decimal.NewFromInt(100)).Round(0).BigInt()
	if !deltaAmountValue.IsInt64() {
		common.ApiErrorMsg(c, "adjustment amount exceeds supported range")
		return
	}
	deltaAmount := deltaAmountValue.Int64()
	adjustment, err := model.AdjustAgentRebateBalance(req.AgentUserId, c.GetInt("id"), deltaAmount, reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, adjustment)
}

func GetAgentAdjustments(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	agentUserId, _ := strconv.Atoi(c.Query("agent_user_id"))
	adjustments, total, err := model.GetAgentAdjustments(pageInfo, agentUserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(adjustments)
	common.ApiSuccess(c, pageInfo)
}

func GetAgentSelfSummary(c *gin.Context) {
	summary, err := model.GetAgentSelfSummary(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func GetAgentSelfDailyMetrics(c *gin.Context) {
	metrics, err := model.GetAgentDailyMetrics(c.GetInt("id"), c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, metrics)
}

func GetAgentSelfRebateRecords(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.GetAgentRebateRecords(pageInfo, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}

func GetAgentSelfAdjustments(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	adjustments, total, err := model.GetAgentAdjustments(pageInfo, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(adjustments)
	common.ApiSuccess(c, pageInfo)
}

func GetAgentSelfDownlines(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")
	users, total, err := model.GetAgentDownlineUsers(pageInfo, c.GetInt("id"), keyword)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
}

func CreateAgentUpgradeRequest(c *gin.Context) {
	var req AgentUpgradeRequestPayload
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	profile, err := model.DirectUpgradeDownlineToAgent(c.GetInt("id"), req.TargetUserId, req.TargetRate, req.Remark)
	if err != nil {
		if writeAgentConflict(c, err) {
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, profile)
}

func GetAgentUpgradeRequests(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := c.Query("status")
	requests, total, err := model.GetAgentUpgradeRequests(pageInfo, status)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(requests)
	common.ApiSuccess(c, pageInfo)
}

func ReviewAgentUpgradeRequest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid request ID")
		return
	}
	var req AgentUpgradeReviewPayload
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	request, err := model.ReviewAgentUpgradeRequest(id, c.GetInt("id"), req.Approve, req.TargetRate, req.Remark)
	if err != nil {
		if writeAgentConflict(c, err) {
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, request)
}

func GetAgentSelfPromoLinks(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	links, total, err := model.GetAgentPromoLinks(pageInfo, c.GetInt("id"), "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(links)
	common.ApiSuccess(c, pageInfo)
}

func GetAgentSelfPromoLinkStats(c *gin.Context) {
	stats, err := model.GetAgentPromoLinkStats(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func CreateAgentWithdrawRequest(c *gin.Context) {
	var req AgentWithdrawRequestPayload
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	amountDecimal, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		common.ApiErrorMsg(c, "invalid withdrawal amount")
		return
	}
	amount := amountDecimal.Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	request, err := model.CreateAgentWithdrawRequest(c.GetInt("id"), req.AccountName, req.AccountNo, amount, req.Remark)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, request)
}

func GetAgentSelfWithdrawRequests(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := c.Query("status")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	requests, total, err := model.GetAgentWithdrawRequests(pageInfo, c.GetInt("id"), status, startDate, endDate)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(requests)
	common.ApiSuccess(c, pageInfo)
}

func GetAgentWithdrawRequests(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := c.Query("status")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	requests, total, err := model.GetAgentWithdrawRequests(pageInfo, 0, status, startDate, endDate)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(requests)
	common.ApiSuccess(c, pageInfo)
}

func ExportAgentWithdrawRequests(c *gin.Context) {
	status := c.Query("status")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	content, batchNo, err := model.ExportAgentWithdrawRequests(status, startDate, endDate)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filename := fmt.Sprintf("agent-withdraw-%s.csv", batchNo)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(200, "text/csv; charset=utf-8", content)
}

func ImportAgentWithdrawResults(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAgentWithdrawImportFileSize+(1<<20))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			common.ApiErrorMsg(c, "import file is too large")
			return
		}
		common.ApiErrorMsg(c, "an import file is required")
		return
	}
	defer file.Close()
	if header.Size > maxAgentWithdrawImportFileSize {
		common.ApiErrorMsg(c, "import file is too large")
		return
	}
	result, err := model.ImportAgentWithdrawResults(file)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func CreateAgentSelfPromoLink(c *gin.Context) {
	var req UpsertAgentPromoLinkRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	promoLink, err := model.UpsertAgentPromoLink(c.GetInt("id"), &model.AgentPromoLink{
		AgentUserId: c.GetInt("id"),
		Name:        req.Name,
		Code:        req.Code,
		Status:      req.Status,
		LandingPage: req.LandingPage,
		Remark:      req.Remark,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, promoLink)
}

func DeleteAgentSelfPromoLink(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid promotion link ID")
		return
	}
	var links []*model.AgentPromoLinkView
	pageInfo := &common.PageInfo{Page: 1, PageSize: 1000}
	links, _, err = model.GetAgentPromoLinks(pageInfo, c.GetInt("id"), "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	owned := false
	for _, link := range links {
		if link.Id == id {
			owned = true
			break
		}
	}
	if !owned {
		common.ApiErrorMsg(c, "promotion link does not belong to this agent")
		return
	}
	if err := model.DeleteAgentPromoLink(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, true)
}

func GetAgentPromoLinks(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	agentUserId, _ := strconv.Atoi(c.Query("agent_user_id"))
	keyword := c.Query("keyword")
	links, total, err := model.GetAgentPromoLinks(pageInfo, agentUserId, keyword)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(links)
	common.ApiSuccess(c, pageInfo)
}

func UpsertAgentPromoLink(c *gin.Context) {
	var req UpsertAgentPromoLinkRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	promoLink, err := model.UpsertAgentPromoLink(c.GetInt("id"), &model.AgentPromoLink{
		Id:          req.Id,
		AgentUserId: req.AgentUserId,
		Name:        req.Name,
		Code:        req.Code,
		Status:      req.Status,
		LandingPage: req.LandingPage,
		Remark:      req.Remark,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, promoLink)
}

func DeleteAgentPromoLink(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid promotion link ID")
		return
	}
	if err := model.DeleteAgentPromoLink(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, true)
}

func GetAgentPromoLinkStats(c *gin.Context) {
	agentUserId, _ := strconv.Atoi(c.Query("agent_user_id"))
	stats, err := model.GetAgentPromoLinkStats(agentUserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func GetAgentDownlineUsers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	agentUserId, _ := strconv.Atoi(c.Query("agent_user_id"))
	keyword := c.Query("keyword")
	if c.GetInt("role") < common.RoleAdminUser {
		users, total, err := model.GetAgentTransferDownlineUsers(pageInfo, agentUserId, keyword)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		pageInfo.SetTotal(int(total))
		pageInfo.SetItems(users)
		common.ApiSuccess(c, pageInfo)
		return
	}
	users, total, err := model.GetAgentDownlineUsers(pageInfo, agentUserId, keyword)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
}

func GetAgentAssignedUsers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	agentUserId, _ := strconv.Atoi(c.Query("agent_user_id"))
	users, total, err := model.GetAgentAssignedUsers(c.Query("keyword"), agentUserId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
}

func TransferAgentDownlineUser(c *gin.Context) {
	var req TransferAgentDownlineUserRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	if c.GetInt("role") == common.RoleSupportUser && (req.SourceAgentUserId == c.GetInt("id") || req.TargetAgentUserId == c.GetInt("id")) {
		common.ApiErrorMsg(c, "support users cannot transfer downlines to or from their own agent account")
		return
	}
	if err := model.TransferAgentDownlineUser(c.GetInt("id"), req.SourceAgentUserId, req.TargetAgentUserId, req.DownlineUserId, req.PromoLinkId, req.Remark); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, true)
}

func AssignAgentDownlineUser(c *gin.Context) {
	var req AssignAgentDownlineUserRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	if err := model.AssignAgentDownlineUser(c.GetInt("id"), req.TargetAgentUserId, req.DownlineUserId, req.PromoLinkId, req.Remark); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, true)
}

func ChangeAgentDownlineUser(c *gin.Context) {
	var req ChangeAgentDownlineUserRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	if c.GetInt("role") == common.RoleSupportUser && req.TargetAgentUserId == c.GetInt("id") {
		common.ApiErrorMsg(c, "support users cannot assign downlines to their own agent account")
		return
	}
	if err := model.ChangeAgentDownlineUser(c.GetInt("id"), req.TargetAgentUserId, req.DownlineUserId, req.PromoLinkId, req.Remark); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, true)
}
