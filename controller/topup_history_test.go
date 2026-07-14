package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func callAdminUserTopUps(t *testing.T, targetUserId int, operatorRole int) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(targetUserId)}}
	ctx.Set("role", operatorRole)
	ctx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/user/%d/topups?p=1&page_size=10", targetUserId), nil)

	GetUserTopUpsByAdmin(ctx)

	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestGetUserTopUpsByAdminEnforcesRoleHierarchy(t *testing.T) {
	db := setupTopupControllerTestDB(t)
	suffix := common.GetRandomString(6)
	commonUser := seedTopupControllerUser(t, db, "bill_"+suffix)
	adminUser := seedTopupControllerUser(t, db, "admin_bill_"+suffix)
	adminUser.Role = common.RoleAdminUser
	require.NoError(t, db.Save(adminUser).Error)

	tradeNo := "admin_user_bill_" + suffix
	require.NoError(t, db.Create(&model.TopUp{
		UserId:     commonUser.Id,
		Amount:     10,
		Money:      10,
		TradeNo:    tradeNo,
		CreateTime: common.GetTimestamp() - 365*24*60*60,
		Status:     common.TopUpStatusSuccess,
	}).Error)

	response := callAdminUserTopUps(t, commonUser.Id, common.RoleAdminUser)
	require.Equal(t, true, response["success"])
	responseBytes, err := common.Marshal(response)
	require.NoError(t, err)
	require.Contains(t, string(responseBytes), tradeNo)

	response = callAdminUserTopUps(t, adminUser.Id, common.RoleAdminUser)
	require.Equal(t, false, response["success"])

	response = callAdminUserTopUps(t, adminUser.Id, common.RoleRootUser)
	require.Equal(t, true, response["success"])
}
