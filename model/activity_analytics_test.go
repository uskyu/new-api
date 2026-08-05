package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestUpdateUserUsageTracksLastAPIActivity(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)
	user := createAgentTestUser(t, "activity_user", "ACT1")
	require.Zero(t, user.LastAPIActivityAt)

	updateUserUsedQuotaAndRequestCount(user.Id, 25, 1)

	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	require.Equal(t, 25, updated.UsedQuota)
	require.Equal(t, 1, updated.RequestCount)
	require.Positive(t, updated.LastAPIActivityAt)
}

func TestEnsureUserLastAPIActivityAtColumnCreatesStableTrackingBaseline(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	user := createAgentTestUser(t, "migration_activity_user", "ACT0")
	require.NoError(t, DB.Model(user).Update("last_api_activity_at", 0).Error)

	require.NoError(t, ensureUserLastAPIActivityAtColumn())
	var first User
	require.NoError(t, DB.First(&first, user.Id).Error)
	require.Zero(t, first.LastAPIActivityAt)
	firstBaseline := getUserLastAPIActivityTrackingStartedAt()
	require.Positive(t, firstBaseline)

	require.NoError(t, ensureUserLastAPIActivityAtColumn())
	require.Equal(t, firstBaseline, getUserLastAPIActivityTrackingStartedAt())
}

func TestGetInactiveUserAnalyticsUsesLastAPIActivity(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)
	now := common.GetTimestamp()
	old := now - 100*24*3600
	recent := now - 3*24*3600
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Create(&Option{
		Key: userLastAPIActivityMigrationKey, Value: fmt.Sprintf("%d", now-200*24*3600),
	}).Error)

	inactive := createAgentTestUser(t, "inactive_api_user", "ACT2")
	neverCalled := createAgentTestUser(t, "never_called_user", "ACT3")
	active := createAgentTestUser(t, "active_api_user", "ACT4")
	disabled := createAgentTestUser(t, "disabled_api_user", "ACT5")
	require.NoError(t, DB.Model(inactive).Updates(map[string]interface{}{
		"created_at": old, "last_api_activity_at": old, "quota": 900,
	}).Error)
	require.NoError(t, DB.Model(neverCalled).Updates(map[string]interface{}{
		"created_at": old, "last_api_activity_at": 0, "quota": 1100,
	}).Error)
	require.NoError(t, DB.Model(active).Updates(map[string]interface{}{
		"created_at": old, "last_api_activity_at": recent, "quota": 5000,
	}).Error)
	require.NoError(t, DB.Model(disabled).Updates(map[string]interface{}{
		"created_at": old, "last_api_activity_at": old, "quota": 7000, "status": common.UserStatusDisabled,
	}).Error)

	result, err := GetInactiveUserAnalytics(&common.PageInfo{Page: 1, PageSize: 10}, InactiveUserAnalyticsOptions{
		Days:        30,
		AccountType: InactiveAccountTypeAll,
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), result.Summary.EligibleUserCount)
	require.Equal(t, int64(2), result.Summary.InactiveUserCount)
	require.Equal(t, int64(2000), result.Summary.InactiveBalanceQuota)
	require.Equal(t, int64(1), result.Summary.NeverCalledCount)
	require.Equal(t, int64(1100), result.Summary.NeverCalledBalance)
	require.Len(t, result.Items, 2)
	require.Equal(t, int64(2), result.Total)
}

func TestGetAgentLeaderboardRanksAndMasksAgents(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)
	now := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.Local)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()

	agentA := createAgentTestUser(t, "alpha_agent", "RANK1")
	agentB := createAgentTestUser(t, "beta_agent", "RANK2")
	require.NoError(t, DB.Create(&AgentProfile{UserId: agentA.Id, Status: AgentStatusEnabled}).Error)
	require.NoError(t, DB.Create(&AgentProfile{UserId: agentB.Id, Status: AgentStatusEnabled}).Error)

	createDownline := func(username string, inviterId int, createdAt int64, quota int) *User {
		user := createAgentTestUser(t, username, fmt.Sprintf("AFF_%s", username))
		require.NoError(t, DB.Model(user).Updates(map[string]interface{}{
			"inviter_id": inviterId,
			"created_at": createdAt,
			"quota":      quota,
		}).Error)
		return user
	}
	a1 := createDownline("alpha_today", agentA.Id, dayStart+60, 800)
	a2 := createDownline("alpha_month", agentA.Id, monthStart+60, 1200)
	b1 := createDownline("beta_today", agentB.Id, dayStart+120, 500)

	require.NoError(t, DB.Create(&AgentRebateRecord{
		TopUpId: 1, TradeNo: "rank-a", SourceType: AgentRebateSourceEPay,
		InviteeUserId: a1.Id, AgentUserId: agentA.Id, PayAmount: 3000,
		RebateAmount: 300, Status: AgentRebateRecordSettled, SettledAt: monthStart + 100,
	}).Error)
	require.NoError(t, DB.Create(&AgentRebateRecord{
		TopUpId: 2, TradeNo: "rank-b", SourceType: AgentRebateSourceEPay,
		InviteeUserId: b1.Id, AgentUserId: agentB.Id, PayAmount: 9000,
		RebateAmount: 900, Status: AgentRebateRecordSettled, SettledAt: monthStart + 200,
	}).Error)
	require.NoError(t, DB.Create(&AgentRebateRecord{
		TopUpId: 3, TradeNo: "rank-a-today-repeat", SourceType: AgentRebateSourceEPay,
		InviteeUserId: a1.Id, AgentUserId: agentA.Id, PayAmount: 4000,
		RebateAmount: 400, Status: AgentRebateRecordSettled, SettledAt: dayStart + 300,
	}).Error)
	require.NoError(t, DB.Create(&AgentRebateRecord{
		TopUpId: 4, TradeNo: "rank-a-today-new", SourceType: AgentRebateSourceEPay,
		InviteeUserId: a2.Id, AgentUserId: agentA.Id, PayAmount: 2000,
		RebateAmount: 200, Status: AgentRebateRecordSettled, SettledAt: dayStart + 400,
	}).Error)

	result, err := getAgentLeaderboardAt(agentB.Id, &common.PageInfo{Page: 1, PageSize: 1}, now)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, int64(2), result.Total)
	require.Equal(t, 1, result.Page)
	require.Equal(t, agentA.Username[:2]+"***"+agentA.Username[len(agentA.Username)-2:], result.Items[0].AgentLabel)
	require.False(t, strings.Contains(result.Items[0].AgentLabel, fmt.Sprintf("%d", agentA.Id)))
	require.Equal(t, int64(2), result.Items[0].MonthNewUserCount)
	require.Equal(t, int64(1), result.Items[0].DayNewUserCount)
	require.Equal(t, int64(6000), result.Items[0].DayTopupAmount)
	require.InDelta(t, 0.5, result.Items[0].MonthRepurchaseRate, 0.0001)
	require.NotNil(t, result.Self)
	require.Equal(t, 2, result.Self.Rank)
	require.True(t, result.Self.IsSelf)

	payload, err := common.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "downline_balance_quota")

	secondPage, err := getAgentLeaderboardAt(agentB.Id, &common.PageInfo{Page: 2, PageSize: 1}, now)
	require.NoError(t, err)
	require.Len(t, secondPage.Items, 1)
	require.True(t, secondPage.Items[0].IsSelf)

	normalizedPage, err := getAgentLeaderboardAt(agentB.Id, &common.PageInfo{Page: -1, PageSize: -1}, now)
	require.NoError(t, err)
	require.Equal(t, 1, normalizedPage.Page)
	require.Equal(t, common.ItemsPerPage, normalizedPage.PageSize)
	require.Len(t, normalizedPage.Items, 2)
}
