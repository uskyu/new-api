package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

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

func InitializeAgentModule(c *gin.Context) {
	var req InitializeAgentModuleRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
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
		common.ApiErrorMsg(c, "无效的参数")
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
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, group)
}

func DeleteAgentRebateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的分组 ID")
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
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(profiles)
	common.ApiSuccess(c, pageInfo)
}

func UpsertAgentProfile(c *gin.Context) {
	var req UpsertAgentProfileRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
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
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, profile)
}

func AdjustAgentBalance(c *gin.Context) {
	var req AdjustAgentBalanceRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		common.ApiErrorMsg(c, "调整原因不能为空")
		return
	}
	amountDecimal, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		common.ApiErrorMsg(c, "调整金额格式错误")
		return
	}
	deltaAmount := amountDecimal.Mul(decimal.NewFromInt(100)).Round(0).IntPart()
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

func CreateAgentSelfPromoLink(c *gin.Context) {
	var req UpsertAgentPromoLinkRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
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
		common.ApiErrorMsg(c, "无效的推广链接 ID")
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
		common.ApiErrorMsg(c, "无权删除该推广链接")
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
		common.ApiErrorMsg(c, "无效的参数")
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
		common.ApiErrorMsg(c, "无效的推广链接 ID")
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
	users, total, err := model.GetAgentDownlineUsers(pageInfo, agentUserId, keyword)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
}
