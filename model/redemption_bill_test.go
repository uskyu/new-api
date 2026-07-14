package model

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func runRedemptionBillQueryTest(t *testing.T, dialect TestDBDialect) {
	t.Helper()
	setupAgentTestDB(t, dialect)

	paymentSetting := operation_setting.GetPaymentSetting()
	originalDays := paymentSetting.UserBillVisibleDays
	paymentSetting.UserBillVisibleDays = 7
	t.Cleanup(func() {
		paymentSetting.UserBillVisibleDays = originalDays
	})

	suffix := common.GetRandomString(6)
	user := createAgentTestUser(t, fmt.Sprintf("redeem_bill_%s_%s", dialect, suffix), "RB_"+suffix)
	otherUser := createAgentTestUser(t, fmt.Sprintf("other_redeem_%s_%s", dialect, suffix), "ORB_"+suffix)
	now := common.GetTimestamp()
	recentKey := common.GetRandomString(32)
	oldKey := common.GetRandomString(32)
	softDeletedKey := common.GetRandomString(32)

	redemptions := []Redemption{
		{Key: recentKey, Name: "recent-" + suffix, Quota: 100, Status: common.RedemptionCodeStatusUsed, UsedUserId: user.Id, RedeemedTime: now - 6*24*60*60},
		{Key: oldKey, Name: "old-" + suffix, Quota: 200, Status: common.RedemptionCodeStatusUsed, UsedUserId: user.Id, RedeemedTime: now - 30*24*60*60},
		{Key: softDeletedKey, Name: "soft-" + suffix, Quota: 300, Status: common.RedemptionCodeStatusUsed, UsedUserId: user.Id, RedeemedTime: now - 24*60*60},
		{Key: common.GetRandomString(32), Name: "other-" + suffix, Quota: 400, Status: common.RedemptionCodeStatusUsed, UsedUserId: otherUser.Id, RedeemedTime: now},
		{Key: common.GetRandomString(32), Name: "unused-" + suffix, Quota: 500, Status: common.RedemptionCodeStatusEnabled},
	}
	require.NoError(t, DB.Create(&redemptions).Error)
	require.NoError(t, DB.Delete(&redemptions[2]).Error)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	recent, total, err := GetRecentUserRedemptionBills(user.Id, "", pageInfo)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, recent, 2)
	recentIds := make(map[int]struct{}, len(recent))
	for _, bill := range recent {
		require.Equal(t, user.Id, bill.UserId)
		require.Len(t, bill.Code, 8)
		require.True(t, strings.HasPrefix(bill.Code, "****"))
		recentIds[bill.Id] = struct{}{}
	}
	require.Contains(t, recentIds, redemptions[2].Id)

	allByUser, total, err := GetAllRedemptionBillsByUser(user.Id, "", pageInfo)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, allByUser, 3)

	matched, total, err := GetAllRedemptionBillsByUser(user.Id, oldKey, pageInfo)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, matched, 1)
	require.Equal(t, redemptions[1].Id, matched[0].Id)
	require.Equal(t, "****"+oldKey[len(oldKey)-4:], matched[0].Code)

	matched, total, err = GetAllRedemptionBillsByUser(user.Id, "%_", pageInfo)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, matched)

	all, total, err := GetAllRedemptionBills("", pageInfo)
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	require.Len(t, all, 4)
}

func TestRedemptionBillQuerySQLite(t *testing.T) {
	runRedemptionBillQueryTest(t, TestDBDialectSQLite)
}

func TestRedemptionBillQueryMySQL(t *testing.T) {
	if os.Getenv("TEST_MYSQL_DSN") == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}
	runRedemptionBillQueryTest(t, TestDBDialectMySQL)
}

func TestRedemptionBillQueryPostgres(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_DSN") == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	runRedemptionBillQueryTest(t, TestDBDialectPostgres)
}
