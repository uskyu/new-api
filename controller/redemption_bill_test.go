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

func callAdminUserRedemptionBills(t *testing.T, targetUserId int, operatorRole int) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(targetUserId)}}
	ctx.Set("role", operatorRole)
	ctx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/user/%d/redemptions?p=1&page_size=10", targetUserId), nil)

	GetUserRedemptionBillsByAdmin(ctx)

	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestGetUserRedemptionBillsByAdminEnforcesRoleAndMasksCode(t *testing.T) {
	db := setupTopupControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Redemption{}))
	suffix := common.GetRandomString(6)
	commonUser := seedTopupControllerUser(t, db, "redemption_bill_"+suffix)
	adminUser := seedTopupControllerUser(t, db, "admin_redemption_bill_"+suffix)
	adminUser.Role = common.RoleAdminUser
	require.NoError(t, db.Save(adminUser).Error)

	code := common.GetRandomString(32)
	require.NoError(t, db.Create(&model.Redemption{
		Key:          code,
		Name:         "admin-bill-" + suffix,
		Quota:        100,
		Status:       common.RedemptionCodeStatusUsed,
		UsedUserId:   commonUser.Id,
		RedeemedTime: common.GetTimestamp(),
	}).Error)

	response := callAdminUserRedemptionBills(t, commonUser.Id, common.RoleAdminUser)
	require.Equal(t, true, response["success"])
	responseBytes, err := common.Marshal(response)
	require.NoError(t, err)
	require.NotContains(t, string(responseBytes), code)
	require.Contains(t, string(responseBytes), "****"+code[len(code)-4:])

	response = callAdminUserRedemptionBills(t, adminUser.Id, common.RoleAdminUser)
	require.Equal(t, false, response["success"])

	response = callAdminUserRedemptionBills(t, adminUser.Id, common.RoleRootUser)
	require.Equal(t, true, response["success"])
}
