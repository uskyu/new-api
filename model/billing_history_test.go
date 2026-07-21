package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBillingHistoryVisibilityAndAdministratorScope(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Redemption{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Redemption{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Redemption{}).Error)
	})

	paymentSetting := operation_setting.GetPaymentSetting()
	originalDays := paymentSetting.UserBillVisibleDays
	paymentSetting.UserBillVisibleDays = 7
	t.Cleanup(func() {
		paymentSetting.UserBillVisibleDays = originalDays
	})

	now := common.GetTimestamp()
	topups := []TopUp{
		{UserId: 101, TradeNo: "recent-order", CreateTime: now - 6*24*60*60},
		{UserId: 101, TradeNo: "old-order", CreateTime: now - 30*24*60*60},
		{UserId: 202, TradeNo: "other-order", CreateTime: now},
	}
	require.NoError(t, DB.Create(&topups).Error)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	recentTopups, total, err := GetUserTopUps(101, pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, recentTopups, 1)
	assert.Equal(t, "recent-order", recentTopups[0].TradeNo)

	allTopups, total, err := GetAllTopUpsByUser(101, pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Len(t, allTopups, 2)

	matchedTopups, total, err := SearchAllTopUpsByUser(101, "old-order", pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, matchedTopups, 1)
	assert.Equal(t, "old-order", matchedTopups[0].TradeNo)

	recentKey := common.GetRandomString(32)
	oldKey := common.GetRandomString(32)
	softDeletedKey := common.GetRandomString(32)
	redemptions := []Redemption{
		{Key: recentKey, Name: "recent-code", Quota: 100, Status: common.RedemptionCodeStatusUsed, UsedUserId: 101, RedeemedTime: now - 6*24*60*60},
		{Key: oldKey, Name: "old-code", Quota: 200, Status: common.RedemptionCodeStatusUsed, UsedUserId: 101, RedeemedTime: now - 30*24*60*60},
		{Key: softDeletedKey, Name: "soft-code", Quota: 300, Status: common.RedemptionCodeStatusUsed, UsedUserId: 101, RedeemedTime: now - 24*60*60},
		{Key: common.GetRandomString(32), Name: "other-code", Quota: 400, Status: common.RedemptionCodeStatusUsed, UsedUserId: 202, RedeemedTime: now},
		{Key: common.GetRandomString(32), Name: "unused-code", Quota: 500, Status: common.RedemptionCodeStatusEnabled},
	}
	require.NoError(t, DB.Create(&redemptions).Error)
	require.NoError(t, DB.Delete(&redemptions[2]).Error)

	recentBills, total, err := GetRecentUserRedemptionBills(101, "", pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, recentBills, 2)
	for _, bill := range recentBills {
		assert.Equal(t, 101, bill.UserId)
		assert.True(t, strings.HasPrefix(bill.Code, "****"))
		assert.Len(t, bill.Code, 8)
	}

	allBills, total, err := GetAllRedemptionBillsByUser(101, "", pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	assert.Len(t, allBills, 3)

	matchedBills, total, err := GetAllRedemptionBillsByUser(101, oldKey, pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, matchedBills, 1)
	assert.Equal(t, "****"+oldKey[len(oldKey)-4:], matchedBills[0].Code)
	assert.NotEqual(t, oldKey, matchedBills[0].Code)

	matchedBills, total, err = GetAllRedemptionBillsByUser(101, "%_", pageInfo)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, matchedBills)

	platformBills, total, err := GetAllRedemptionBills("", pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 4, total)
	assert.Len(t, platformBills, 4)

	_, _, err = GetRecentUserRedemptionBills(0, "", pageInfo)
	require.ErrorContains(t, err, "invalid user id")
	_, _, err = GetAllRedemptionBillsByUser(0, "", pageInfo)
	require.ErrorContains(t, err, "invalid user id")
}
