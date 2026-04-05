package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTopupControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	var (
		db  *gorm.DB
		err error
	)
	if dsn := os.Getenv("TEST_POSTGRES_DSN"); dsn != "" {
		common.UsingSQLite = false
		common.UsingMySQL = false
		common.UsingPostgreSQL = true
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	} else if dsn := os.Getenv("TEST_MYSQL_DSN"); dsn != "" {
		common.UsingSQLite = false
		common.UsingMySQL = true
		common.UsingPostgreSQL = false
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	} else {
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	}
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.Log{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedTopupControllerUser(t *testing.T, db *gorm.DB, suffix string) *model.User {
	t.Helper()
	user := &model.User{
		Username:    "notify_" + suffix,
		Password:    "hashed-password",
		DisplayName: "notify_" + suffix,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "AFFN" + suffix,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func buildEpayNotifyRequest(t *testing.T, values map[string]string) *http.Request {
	t.Helper()
	form := url.Values{}
	for k, v := range values {
		form.Set(k, v)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/user/epay/notify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestEpayNotifyNoDoubleCredit(t *testing.T) {
	db := setupTopupControllerTestDB(t)

	prevPayAddress := operation_setting.PayAddress
	prevEpayID := operation_setting.EpayId
	prevEpayKey := operation_setting.EpayKey
	t.Cleanup(func() {
		operation_setting.PayAddress = prevPayAddress
		operation_setting.EpayId = prevEpayID
		operation_setting.EpayKey = prevEpayKey
	})
	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "partner-test"
	operation_setting.EpayKey = "secret-test"

	suffix := common.GetRandomString(6)
	user := seedTopupControllerUser(t, db, suffix)
	tradeNo := "notify_trade_" + suffix
	topup := &model.TopUp{
		UserId:        user.Id,
		Amount:        12,
		Money:         12,
		TradeNo:       tradeNo,
		PaymentMethod: "epay",
		CreateTime:    common.GetTimestamp(),
		Status:        common.TopUpStatusPending,
	}
	require.NoError(t, db.Create(topup).Error)

	before, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	require.NotNil(t, before)

	params := epay.GenerateParams(map[string]string{
		"pid":          operation_setting.EpayId,
		"type":         "alipay",
		"trade_no":     "epay_" + suffix,
		"out_trade_no": tradeNo,
		"name":         "topup",
		"money":        "12",
		"trade_status": epay.StatusTradeSuccess,
	}, operation_setting.EpayKey)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = buildEpayNotifyRequest(t, params)
	EpayNotify(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "success", recorder.Body.String())

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = buildEpayNotifyRequest(t, params)
	EpayNotify(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "success", recorder.Body.String())

	after, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	require.NotNil(t, after)

	updatedTopUp := model.GetTopUpByTradeNo(tradeNo)
	require.NotNil(t, updatedTopUp)
	require.Equal(t, common.TopUpStatusSuccess, updatedTopUp.Status)
	require.NotZero(t, updatedTopUp.CompleteTime)

	expectedQuotaDelta := int(float64(topup.Amount) * common.QuotaPerUnit)
	require.Equal(t, before.Quota+expectedQuotaDelta, after.Quota)
}
