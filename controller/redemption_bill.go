package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func respondRedemptionBills(c *gin.Context, bills []*model.RedemptionBill, total int64, pageInfo *common.PageInfo, err error) {
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(bills)
	common.ApiSuccess(c, pageInfo)
}

func GetUserRedemptionBills(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	bills, total, err := model.GetRecentUserRedemptionBills(c.GetInt("id"), c.Query("keyword"), pageInfo)
	respondRedemptionBills(c, bills, total, pageInfo, err)
}

func GetAllRedemptionBills(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	bills, total, err := model.GetAllRedemptionBills(c.Query("keyword"), pageInfo)
	respondRedemptionBills(c, bills, total, pageInfo, err)
}

func GetUserRedemptionBillsByAdmin(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
		return
	}

	pageInfo := common.GetPageQuery(c)
	bills, total, err := model.GetAllRedemptionBillsByUser(userId, c.Query("keyword"), pageInfo)
	respondRedemptionBills(c, bills, total, pageInfo, err)
}
