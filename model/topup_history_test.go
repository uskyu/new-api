package model

import (
	"fmt"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func runUserTopUpVisibilityTest(t *testing.T, dialect TestDBDialect) {
	t.Helper()
	setupAgentTestDB(t, dialect)

	paymentSetting := operation_setting.GetPaymentSetting()
	originalDays := paymentSetting.UserBillVisibleDays
	paymentSetting.UserBillVisibleDays = 7
	t.Cleanup(func() {
		paymentSetting.UserBillVisibleDays = originalDays
	})

	suffix := common.GetRandomString(6)
	user := createAgentTestUser(t, fmt.Sprintf("bill_%s_%s", dialect, suffix), "BILL_"+suffix)
	otherUser := createAgentTestUser(t, fmt.Sprintf("other_bill_%s_%s", dialect, suffix), "OTHER_BILL_"+suffix)
	now := common.GetTimestamp()
	topups := []TopUp{
		{UserId: user.Id, Amount: 10, Money: 10, TradeNo: "recent_" + suffix, CreateTime: now - 6*24*60*60, Status: common.TopUpStatusSuccess},
		{UserId: user.Id, Amount: 20, Money: 20, TradeNo: "old_" + suffix, CreateTime: now - 8*24*60*60, Status: common.TopUpStatusSuccess},
		{UserId: otherUser.Id, Amount: 30, Money: 30, TradeNo: "other_" + suffix, CreateTime: now, Status: common.TopUpStatusSuccess},
	}
	require.NoError(t, DB.Create(&topups).Error)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	recent, total, err := GetUserTopUps(user.Id, pageInfo)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, recent, 1)
	require.Equal(t, "recent_"+suffix, recent[0].TradeNo)

	searchRecent, total, err := SearchUserTopUps(user.Id, "old_"+suffix, pageInfo)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, searchRecent)

	all, total, err := GetAllTopUpsByUser(user.Id, pageInfo)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, all, 2)
	for _, topup := range all {
		require.Equal(t, user.Id, topup.UserId)
	}

	searchAll, total, err := SearchAllTopUpsByUser(user.Id, "old_"+suffix, pageInfo)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, searchAll, 1)
	require.Equal(t, "old_"+suffix, searchAll[0].TradeNo)
}

func TestUserTopUpVisibilitySQLite(t *testing.T) {
	runUserTopUpVisibilityTest(t, TestDBDialectSQLite)
}

func TestUserTopUpVisibilityMySQL(t *testing.T) {
	if os.Getenv("TEST_MYSQL_DSN") == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}
	runUserTopUpVisibilityTest(t, TestDBDialectMySQL)
}

func TestUserTopUpVisibilityPostgres(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_DSN") == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	runUserTopUpVisibilityTest(t, TestDBDialectPostgres)
}
