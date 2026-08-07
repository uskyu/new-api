package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

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
	items, total, err := model.ListRiskSharedIPs(
		strings.TrimSpace(c.Query("source")),
		strings.TrimSpace(c.DefaultQuery("search_type", "ip")),
		strings.TrimSpace(c.Query("keyword")),
		minUsers,
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
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
	items, total, err := model.ListRiskInviters(
		strings.TrimSpace(c.Query("keyword")),
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
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
		"group":         user.Group,
		"inviter_id":    user.InviterId,
		"last_login_at": user.LastLoginAt,
		"last_login_ip": user.LastLoginIp,
		"ip_records":    records,
	})
}
