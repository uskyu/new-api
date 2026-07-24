package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useSeparateAnalyticsLogDatabase(t *testing.T) {
	t.Helper()
	previousLogDB := LOG_DB
	previousLogType := common.LogDatabaseType()
	logDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s-logs?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, logDB.AutoMigrate(&Log{}))
	LOG_DB = logDB
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		LOG_DB = previousLogDB
		common.SetLogDatabaseType(previousLogType)
		sqlDB, sqlErr := logDB.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestGetAdminAnalyticsOverviewAggregatesMainAndLogDatabases(t *testing.T) {
	setupAgentBackendTest(t)
	useSeparateAnalyticsLogDatabase(t)
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })

	first := createAgentBackendTestUser(t, "analytics-first")
	second := createAgentBackendTestUser(t, "analytics-second")
	agent := createAgentBackendTestUser(t, "analytics-agent")
	require.NoError(t, DB.Model(first).Updates(map[string]any{"created_at": int64(1_100), "quota": 50, "used_quota": 30}).Error)
	require.NoError(t, DB.Model(second).Updates(map[string]any{"created_at": int64(1_200), "quota": 1_500, "used_quota": 200}).Error)
	require.NoError(t, DB.Model(agent).Updates(map[string]any{"created_at": int64(500), "quota": 20_000, "used_quota": 800}).Error)
	require.NoError(t, DB.Create(&AgentProfile{UserId: agent.Id, Status: AgentStatusEnabled}).Error)

	topups := []*TopUp{
		{UserId: first.Id, Money: 10, TradeNo: "analytics-1", PaymentMethod: PaymentMethodStripe, CompleteTime: 1_300, Status: common.TopUpStatusSuccess},
		{UserId: first.Id, Money: 20, TradeNo: "analytics-2", PaymentMethod: PaymentMethodStripe, CompleteTime: 1_400, Status: common.TopUpStatusSuccess},
		{UserId: second.Id, Money: 31, TradeNo: "analytics-3", PaymentMethod: PaymentMethodCreem, CompleteTime: 1_500, Status: common.TopUpStatusSuccess},
		{UserId: second.Id, Money: 99, TradeNo: "analytics-pending", PaymentMethod: PaymentMethodCreem, CompleteTime: 1_600, Status: common.TopUpStatusPending},
		{UserId: second.Id, Money: 77, TradeNo: "analytics-old", PaymentMethod: PaymentMethodCreem, CompleteTime: 900, Status: common.TopUpStatusSuccess},
	}
	require.NoError(t, DB.Create(&topups).Error)
	require.NoError(t, LOG_DB.Create(&[]Log{
		{UserId: first.Id, CreatedAt: 1_350, Type: LogTypeConsume, ModelName: "gpt-a", Quota: 120},
		{UserId: first.Id, CreatedAt: 1_450, Type: LogTypeConsume, ModelName: "gpt-a", Quota: 80},
		{UserId: second.Id, CreatedAt: 1_550, Type: LogTypeConsume, ModelName: "gpt-b", Quota: 300},
		{UserId: second.Id, CreatedAt: 1_650, Type: LogTypeError, Quota: 900},
		{UserId: second.Id, CreatedAt: 900, Type: LogTypeConsume, Quota: 700},
	}).Error)
	require.NoError(t, DB.Create(&AgentRebateRecord{
		TopUpId:       topups[0].Id,
		TradeNo:       topups[0].TradeNo,
		SourceType:    AgentRebateSourceStripe,
		InviteeUserId: first.Id,
		AgentUserId:   agent.Id,
		PayAmount:     1_000,
		RebateAmount:  100,
		Status:        AgentRebateRecordSettled,
		SettledAt:     1_300,
	}).Error)

	overview, err := GetAdminAnalyticsOverview(AnalyticsOverviewOptions{
		Range: AnalyticsRange{Range: "custom", Start: 1_000, End: 2_000},
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), overview.Metrics.NewUserCount)
	assert.Equal(t, float64(61), overview.Metrics.SuccessfulTopupAmount)
	assert.Equal(t, int64(3), overview.Metrics.SuccessfulTopupCount)
	assert.Equal(t, int64(2), overview.Metrics.TopupUserCount)
	assert.Equal(t, int64(1), overview.Metrics.RepeatTopupUserCount)
	assert.Equal(t, 0.5, overview.Metrics.RepurchaseRate)
	assert.Equal(t, int64(500), overview.Metrics.ConsumeQuota)
	assert.Equal(t, int64(3), overview.Metrics.CallCount)
	assert.Equal(t, int64(2), overview.Metrics.ActiveUserCount)
	require.Len(t, overview.Rankings.UserTopups, 2)
	assert.Equal(t, second.Id, overview.Rankings.UserTopups[0].UserId)
	assert.Equal(t, "analytics-second", overview.Rankings.UserTopups[0].Username)
	require.Len(t, overview.Rankings.AgentContribution, 1)
	assert.Equal(t, agent.Id, overview.Rankings.AgentContribution[0].AgentUserId)
	assert.Equal(t, int64(1_000), overview.Rankings.AgentContribution[0].PayAmount)
	assert.Equal(t, int64(100), overview.Rankings.AgentContribution[0].RebateAmount)
	require.Len(t, overview.Rankings.ModelUsage, 2)
	assert.Equal(t, "gpt-b", overview.Rankings.ModelUsage[0].ModelName)
	assert.Equal(t, int64(300), overview.Rankings.ModelUsage[0].ConsumeQuota)
	assert.Equal(t, int64(1), overview.Rankings.ModelUsage[0].CallCount)
	assert.Equal(t, int64(1), overview.Rankings.ModelUsage[0].ActiveUserCount)
	assert.Equal(t, "gpt-a", overview.Rankings.ModelUsage[1].ModelName)
	assert.Equal(t, int64(2), overview.Rankings.ModelUsage[1].CallCount)
	require.Len(t, overview.Trends, 1)
	assert.Equal(t, int64(500), overview.Trends[0].ConsumeQuota)
}

func TestGetInactiveUserAnalyticsFiltersAgentsAndPaginates(t *testing.T) {
	setupAgentBackendTest(t)
	now := common.GetTimestamp()
	old := now - 61*24*3600
	recent := now - 2*24*3600

	oldUser := createAgentBackendTestUser(t, "inactive-user")
	oldAgent := createAgentBackendTestUser(t, "inactive-agent")
	recentUser := createAgentBackendTestUser(t, "recent-user")
	disabledUser := createAgentBackendTestUser(t, "disabled-user")
	require.NoError(t, DB.Model(oldUser).Updates(map[string]any{"created_at": old, "last_login_at": 0, "quota": 900}).Error)
	require.NoError(t, DB.Model(oldAgent).Updates(map[string]any{"created_at": old, "last_login_at": old, "quota": 1_100}).Error)
	require.NoError(t, DB.Model(recentUser).Updates(map[string]any{"created_at": old, "last_login_at": recent, "quota": 2_000}).Error)
	require.NoError(t, DB.Model(disabledUser).Updates(map[string]any{"created_at": old, "last_login_at": 0, "quota": 4_000, "status": common.UserStatusDisabled}).Error)
	require.NoError(t, DB.Create(&AgentProfile{UserId: oldAgent.Id, Status: AgentStatusEnabled}).Error)

	result, err := GetInactiveUserAnalytics(&common.PageInfo{Page: 1, PageSize: 1}, InactiveUserAnalyticsOptions{
		Days:        30,
		AccountType: InactiveAccountTypeAll,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), result.Summary.EligibleUserCount)
	assert.Equal(t, int64(2), result.Summary.InactiveUserCount)
	assert.Equal(t, int64(2_000), result.Summary.InactiveBalanceQuota)
	assert.Equal(t, int64(1), result.Summary.NeverLoggedInCount)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, int64(2), result.Total)
	assert.GreaterOrEqual(t, result.Items[0].InactiveDays, 60)

	agents, err := GetInactiveUserAnalytics(&common.PageInfo{Page: 1, PageSize: 10}, InactiveUserAnalyticsOptions{
		Days:        30,
		AccountType: InactiveAccountTypeAgents,
	})
	require.NoError(t, err)
	require.Len(t, agents.Items, 1)
	assert.Equal(t, oldAgent.Id, agents.Items[0].Id)
	assert.True(t, agents.Items[0].IsAgent)

	users, err := GetInactiveUserAnalytics(&common.PageInfo{Page: 1, PageSize: 10}, InactiveUserAnalyticsOptions{
		Days:        30,
		Keyword:     "inactive-user",
		AccountType: InactiveAccountTypeUsers,
	})
	require.NoError(t, err)
	require.Len(t, users.Items, 1)
	assert.Equal(t, oldUser.Id, users.Items[0].Id)
}

func TestGetInactiveUserAnalyticsRejectsExcessiveCustomDays(t *testing.T) {
	setupAgentBackendTest(t)

	_, err := GetInactiveUserAnalytics(&common.PageInfo{Page: 1, PageSize: 10}, InactiveUserAnalyticsOptions{
		Days:        MaxInactiveAnalyticsDays + 1,
		AccountType: InactiveAccountTypeAll,
	})
	require.ErrorContains(t, err, "invalid inactive user query")
}

func TestAgentRebateGroupsReportUsageAndRejectUnavailableGroups(t *testing.T) {
	setupAgentBackendTest(t)
	operator := createAgentBackendTestUser(t, "group-operator")
	agent := createAgentBackendTestUser(t, "group-agent")
	enabled := &AgentRebateGroup{Name: "enabled-group", RebateRate: 1_000, Status: AgentStatusEnabled}
	disabled := &AgentRebateGroup{Name: "disabled-group", RebateRate: 500, Status: AgentStatusDisabled}
	require.NoError(t, DB.Create(enabled).Error)
	require.NoError(t, DB.Create(disabled).Error)
	assert.Equal(t, AgentStatusDisabled, disabled.Status)
	assert.False(t, DB.Migrator().HasColumn(&AgentRebateGroup{}, "agent_count"))

	_, err := UpsertAgentProfile(operator.Id, &AgentProfile{
		UserId:        agent.Id,
		Status:        AgentStatusEnabled,
		RebateGroupId: disabled.Id,
	})
	require.ErrorContains(t, err, "disabled")

	_, err = UpsertAgentProfile(operator.Id, &AgentProfile{
		UserId:        agent.Id,
		Status:        AgentStatusEnabled,
		RebateGroupId: enabled.Id,
	})
	require.NoError(t, err)

	groups, err := GetAllAgentRebateGroups()
	require.NoError(t, err)
	counts := make(map[int]int64, len(groups))
	for _, group := range groups {
		counts[group.Id] = group.AgentCount
	}
	assert.Equal(t, int64(1), counts[enabled.Id])
	assert.Zero(t, counts[disabled.Id])

	_, err = UpsertAgentProfile(operator.Id, &AgentProfile{
		UserId:        agent.Id,
		Status:        AgentStatusEnabled,
		RebateGroupId: 999_999,
	})
	require.ErrorContains(t, err, "does not exist")
}
