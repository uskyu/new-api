package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func SearchSupportUsers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	users, total, err := model.SearchSupportManagedUsers(strings.TrimSpace(c.Query("keyword")), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
}

func GetSupportUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid user id")
		return
	}
	user, err := model.GetSupportManagedUserById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, user)
}
