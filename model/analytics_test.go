package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetAdminAnalyticsOverviewAggregatesMainAndIndependentLogDB(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)
	require.NoError(t, DB.AutoMigrate(&Channel{}))

	logDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s_logs?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, logDB.AutoMigrate(&Log{}))
	LOG_DB = logDB
	t.Cleanup(func() {
		sqlDB, err := logDB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	start := int64(1_700_000_000)
	end := start + 7*24*3600 - 1

	agent := createAgentTestUser(t, "analytics_agent", "ANA1")
	userA := createAgentTestUser(t, "analytics_user_a", "ANA2")
	userB := createAgentTestUser(t, "analytics_user_b", "ANA3")
	channel := &Channel{Name: "analytics-channel"}
	require.NoError(t, DB.Create(channel).Error)
	require.NoError(t, DB.Model(userA).Updates(map[string]interface{}{"quota": 0, "used_quota": 100}).Error)
	require.NoError(t, DB.Model(userB).Updates(map[string]interface{}{"quota": int(common.QuotaPerUnit) * 2, "used_quota": 200}).Error)
	require.NoError(t, DB.Model(agent).Updates(map[string]interface{}{"quota": 500, "used_quota": 50}).Error)

	topups := []TopUp{
		{UserId: userA.Id, Money: 10, Amount: 10, TradeNo: "analytics_tu_1", PaymentMethod: "epay", CompleteTime: start + 100, Status: common.TopUpStatusSuccess},
		{UserId: userA.Id, Money: 20, Amount: 20, TradeNo: "analytics_tu_2", PaymentMethod: "stripe", CompleteTime: start + 200, Status: common.TopUpStatusSuccess},
		{UserId: userB.Id, Money: 5, Amount: 5, TradeNo: "analytics_tu_3", PaymentMethod: "epay", CompleteTime: start + 300, Status: common.TopUpStatusSuccess},
		{UserId: userB.Id, Money: 100, Amount: 100, TradeNo: "analytics_tu_pending", PaymentMethod: "epay", CompleteTime: start + 400, Status: common.TopUpStatusPending},
		{UserId: userB.Id, Money: 50, Amount: 50, TradeNo: "analytics_tu_old", PaymentMethod: "epay", CompleteTime: start - 1, Status: common.TopUpStatusSuccess},
	}
	require.NoError(t, DB.Create(&topups).Error)

	require.NoError(t, DB.Create(&AgentRebateRecord{
		TopUpId:       topups[0].Id,
		TradeNo:       "analytics_rebate_1",
		SourceType:    AgentRebateSourceEPay,
		InviteeUserId: userA.Id,
		AgentUserId:   agent.Id,
		PayAmount:     1000,
		RebateAmount:  100,
		Status:        AgentRebateRecordSettled,
		SettledAt:     start + 500,
	}).Error)
	require.NoError(t, DB.Create(&AgentRebateRecord{
		TopUpId:       topups[2].Id,
		TradeNo:       "analytics_rebate_old",
		SourceType:    AgentRebateSourceEPay,
		InviteeUserId: userB.Id,
		AgentUserId:   agent.Id,
		PayAmount:     9000,
		RebateAmount:  900,
		Status:        AgentRebateRecordSettled,
		SettledAt:     start - 1,
	}).Error)
	require.NoError(t, DB.Create(&AgentRedemptionRebateRecord{
		RedemptionId:  9001,
		ReferenceNo:   "analytics_redemption_rebate_1",
		SourceType:    AgentRebateSourceRedemption,
		InviteeUserId: userB.Id,
		AgentUserId:   agent.Id,
		RedeemQuota:   600,
		PayAmount:     600,
		RebateAmount:  60,
		Status:        AgentRebateRecordSettled,
		SettledAt:     start + 600,
	}).Error)

	logs := []Log{
		{UserId: userA.Id, Username: userA.Username, Type: LogTypeConsume, CreatedAt: start + 100, ModelName: "gpt-4", Quota: 100, PromptTokens: 10, CompletionTokens: 20, UseTime: 2, ChannelId: channel.Id},
		{UserId: userB.Id, Username: userB.Username, Type: LogTypeConsume, CreatedAt: start + 200, ModelName: "gpt-4", Quota: 300, PromptTokens: 30, CompletionTokens: 40, UseTime: 4, ChannelId: channel.Id},
		{UserId: userB.Id, Username: userB.Username, Type: LogTypeConsume, CreatedAt: start + 300, ModelName: "claude", Quota: 50, PromptTokens: 5, CompletionTokens: 6, UseTime: 1, ChannelId: 0},
		{UserId: userA.Id, Username: userA.Username, Type: LogTypeError, CreatedAt: start + 400, ModelName: "gpt-4", Quota: 999},
		{UserId: userA.Id, Username: userA.Username, Type: LogTypeConsume, CreatedAt: end + 1, ModelName: "gpt-4", Quota: 999},
	}
	require.NoError(t, logDB.Create(&logs).Error)

	overview, err := GetAdminAnalyticsOverview(AnalyticsOverviewOptions{
		Range: AnalyticsRange{Range: "custom", Start: start, End: end},
		Limit: 10,
	})
	require.NoError(t, err)

	require.Equal(t, int64(3), overview.Metrics.UserCount)
	require.Equal(t, int64(int(common.QuotaPerUnit)*2+500), overview.Metrics.UserBalanceQuota)
	require.Equal(t, int64(350), overview.Metrics.UserUsedQuota)
	require.Equal(t, float64(35), overview.Metrics.SuccessfulTopupAmount)
	require.Equal(t, int64(3), overview.Metrics.SuccessfulTopupCount)
	require.Equal(t, int64(2), overview.Metrics.TopupUserCount)
	require.Equal(t, int64(1), overview.Metrics.RepeatTopupUserCount)
	require.InDelta(t, 0.5, overview.Metrics.RepurchaseRate, 0.0001)
	require.Equal(t, int64(450), overview.Metrics.ConsumeQuota)
	require.Equal(t, int64(3), overview.Metrics.CallCount)
	require.Equal(t, int64(2), overview.Metrics.ActiveUserCount)

	require.Len(t, overview.Distributions.PaymentMethod, 2)
	require.Equal(t, "stripe", overview.Distributions.PaymentMethod[0].PaymentMethod)
	require.Equal(t, float64(20), overview.Distributions.PaymentMethod[0].TopupAmount)
	require.Equal(t, "gpt-4", overview.Distributions.ModelConsumption[0].ModelName)
	require.Equal(t, int64(400), overview.Distributions.ModelConsumption[0].Quota)
	require.Equal(t, int64(2), overview.Distributions.ModelConsumption[0].CallCount)
	require.Equal(t, int64(2), overview.Distributions.ModelConsumption[0].ActiveUserCount)
	require.Equal(t, int64(100), overview.Distributions.ModelConsumption[0].TotalTokens)
	require.InDelta(t, 200, overview.Distributions.ModelConsumption[0].AverageQuota, 0.0001)
	require.InDelta(t, 3, overview.Distributions.ModelConsumption[0].AverageUseTime, 0.0001)
	require.Equal(t, channel.Id, overview.Distributions.ChannelUsage[0].ChannelId)
	require.Equal(t, channel.Name, overview.Distributions.ChannelUsage[0].ChannelName)
	require.Equal(t, int64(400), overview.Distributions.ChannelUsage[0].Quota)

	require.Len(t, overview.Rankings.AgentContribution, 1)
	require.Equal(t, agent.Id, overview.Rankings.AgentContribution[0].AgentUserId)
	require.Equal(t, agent.Username, overview.Rankings.AgentContribution[0].Username)
	require.Equal(t, int64(1600), overview.Rankings.AgentContribution[0].PayAmount)
	require.Equal(t, int64(160), overview.Rankings.AgentContribution[0].RebateAmount)
	require.Equal(t, int64(2), overview.Rankings.AgentContribution[0].TopupCount)
	require.Equal(t, userA.Id, overview.Rankings.UserTopups[0].UserId)
	require.Equal(t, float64(30), overview.Rankings.UserTopups[0].TopupAmount)
	require.Equal(t, userB.Id, overview.Rankings.UserConsumptions[0].UserId)
	require.Equal(t, int64(350), overview.Rankings.UserConsumptions[0].ConsumeQuota)

	require.Equal(t, int64(24*3600), overview.Range.Step)
	require.Len(t, overview.Trends, 7)
	require.Equal(t, float64(35), overview.Trends[0].TopupAmount)
	require.Equal(t, int64(450), overview.Trends[0].ConsumeQuota)
	require.Equal(t, int64(2), overview.Trends[0].ActiveUserCount)
}

func TestAnalyticsLimitCapsRankingsAndDistributions(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)
	require.NoError(t, DB.AutoMigrate(&Channel{}))

	start := int64(1_700_000_000)
	end := start + 24*3600 - 1

	for i := 0; i < 3; i++ {
		user := createAgentTestUser(t, fmt.Sprintf("analytics_limit_%d", i), fmt.Sprintf("AL%d", i))
		require.NoError(t, DB.Create(&TopUp{
			UserId:        user.Id,
			Money:         float64(10 + i),
			Amount:        int64(10 + i),
			TradeNo:       fmt.Sprintf("analytics_limit_tu_%d", i),
			PaymentMethod: fmt.Sprintf("method_%d", i),
			CompleteTime:  start + int64(i),
			Status:        common.TopUpStatusSuccess,
		}).Error)
		require.NoError(t, LOG_DB.Create(&Log{
			UserId:    user.Id,
			Username:  user.Username,
			Type:      LogTypeConsume,
			CreatedAt: start + int64(i),
			ModelName: fmt.Sprintf("model_%d", i),
			Quota:     100 + i,
		}).Error)
	}

	overview, err := GetAdminAnalyticsOverview(AnalyticsOverviewOptions{
		Range: AnalyticsRange{Range: "custom", Start: start, End: end},
		Limit: 2,
	})
	require.NoError(t, err)

	require.Len(t, overview.Distributions.PaymentMethod, 2)
	require.Len(t, overview.Distributions.ModelConsumption, 2)
	require.Len(t, overview.Distributions.ChannelUsage, 1)
	require.Len(t, overview.Rankings.UserTopups, 2)
	require.Len(t, overview.Rankings.UserConsumptions, 2)
	require.Equal(t, float64(12), overview.Rankings.UserTopups[0].TopupAmount)
	require.Equal(t, int64(102), overview.Rankings.UserConsumptions[0].ConsumeQuota)
}
