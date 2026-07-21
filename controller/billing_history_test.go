package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func billingHistoryRequest(t *testing.T, path string, targetUserId int, operatorRole int, handler gin.HandlerFunc) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(targetUserId)}}
	ctx.Set("role", operatorRole)
	ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)

	handler(ctx)

	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestAdministratorBillingHistoryEnforcesRoleAndMasksRedemptionCode(t *testing.T) {
	db := setupManageUserTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.Redemption{}))

	user := &model.User{Username: "billing-user", Role: common.RoleCommonUser, AffCode: "bill-user"}
	admin := &model.User{Username: "billing-admin", Role: common.RoleAdminUser, AffCode: "bill-admin"}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(admin).Error)

	require.NoError(t, db.Create(&model.TopUp{
		UserId:     user.Id,
		TradeNo:    "admin-visible-order",
		CreateTime: common.GetTimestamp() - 365*24*60*60,
	}).Error)
	code := common.GetRandomString(32)
	require.NoError(t, db.Create(&model.Redemption{
		Key:          code,
		Name:         "admin-visible-code",
		Quota:        100,
		Status:       common.RedemptionCodeStatusUsed,
		UsedUserId:   user.Id,
		RedeemedTime: common.GetTimestamp(),
	}).Error)

	topupResponse := billingHistoryRequest(
		t,
		"/api/user/1/topups?p=1&page_size=10",
		user.Id,
		common.RoleAdminUser,
		GetUserTopUpsByAdmin,
	)
	assert.Equal(t, true, topupResponse["success"])

	redemptionResponse := billingHistoryRequest(
		t,
		"/api/user/1/redemptions?p=1&page_size=10",
		user.Id,
		common.RoleAdminUser,
		GetUserRedemptionBillsByAdmin,
	)
	assert.Equal(t, true, redemptionResponse["success"])
	responseBytes, err := common.Marshal(redemptionResponse)
	require.NoError(t, err)
	assert.NotContains(t, string(responseBytes), code)
	assert.Contains(t, string(responseBytes), "****"+code[len(code)-4:])

	deniedResponse := billingHistoryRequest(
		t,
		"/api/user/2/topups?p=1&page_size=10",
		admin.Id,
		common.RoleAdminUser,
		GetUserTopUpsByAdmin,
	)
	assert.Equal(t, false, deniedResponse["success"])

	for _, handler := range []gin.HandlerFunc{GetUserTopUpsByAdmin, GetUserRedemptionBillsByAdmin} {
		supportResponse := billingHistoryRequest(
			t,
			"/api/user/1/billing?p=1&page_size=10",
			user.Id,
			common.RoleSupportUser,
			handler,
		)
		assert.Equal(t, false, supportResponse["success"])
	}

	adminAllResponse := billingHistoryRequest(
		t,
		"/api/redemption/bills?p=1&page_size=10",
		0,
		common.RoleAdminUser,
		GetAllRedemptionBills,
	)
	assert.Equal(t, true, adminAllResponse["success"])
	adminAllBytes, err := common.Marshal(adminAllResponse)
	require.NoError(t, err)
	assert.NotContains(t, string(adminAllBytes), code)

	supportAllResponse := billingHistoryRequest(
		t,
		"/api/redemption/bills?p=1&page_size=10",
		0,
		common.RoleSupportUser,
		GetAllRedemptionBills,
	)
	assert.Equal(t, false, supportAllResponse["success"])
}
