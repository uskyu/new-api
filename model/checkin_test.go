package model

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestSelectCheckinTierUsesHighestThreshold(t *testing.T) {
	tiers := []operation_setting.CheckinBonusTier{
		{Threshold: 50, MinQuota: 2000, MaxQuota: 20000},
		{Threshold: 100, MinQuota: 5000, MaxQuota: 50000},
		{Threshold: 200, MinQuota: 20000, MaxQuota: 200000},
	}
	require.Nil(t, SelectCheckinTier(tiers, 10))
	require.Equal(t, 50, SelectCheckinTier(tiers, 50).Threshold)
	require.Equal(t, 100, SelectCheckinTier(tiers, 100).Threshold)
	require.Equal(t, 200, SelectCheckinTier(tiers, 300).Threshold)
}

func TestSelectCheckinRewardUsesTierRange(t *testing.T) {
	setting := &operation_setting.CheckinSetting{
		RequestCountTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 50, MinQuota: 2000, MaxQuota: 2000},
		},
	}
	// 未达标：奖励为 0，不再回退基础签到奖励
	require.Equal(t, 0, selectCheckinReward(setting, 10))
	require.Equal(t, 0, selectCheckinReward(setting, 49))
	// 达标：使用档位区间
	require.Equal(t, 2000, selectCheckinReward(setting, 50))
	require.Equal(t, 2000, selectCheckinReward(setting, 51))
}

func TestSelectCheckinRewardZeroWhenNoTierOrInvalidTier(t *testing.T) {
	// 无效档位（最高奖励低于最低奖励）：奖励为 0
	setting := &operation_setting.CheckinSetting{
		RequestCountTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 50, MinQuota: 5000, MaxQuota: 2000},
		},
	}
	require.Equal(t, 0, selectCheckinReward(setting, 50))
	// 无档位配置：奖励为 0
	emptySetting := &operation_setting.CheckinSetting{}
	require.Equal(t, 0, selectCheckinReward(emptySetting, 100))
}

func TestSelectCheckinRewardOnlyHighestTierNoStacking(t *testing.T) {
	setting := &operation_setting.CheckinSetting{
		RequestCountTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 50, MinQuota: 2000, MaxQuota: 2000},
			{Threshold: 100, MinQuota: 5000, MaxQuota: 5000},
			{Threshold: 200, MinQuota: 20000, MaxQuota: 20000},
		},
	}
	// 命中多个档位时只取 threshold 最高的一档，各档奖励不叠加
	require.Equal(t, 5000, selectCheckinReward(setting, 120))
	require.Equal(t, 20000, selectCheckinReward(setting, 250))
}

func TestActiveTiersIndependentPerMetric(t *testing.T) {
	// 两套档位互不干扰：旧 ActiveTiers API 仍按 bonus_metric 返回对应档位
	setting := &operation_setting.CheckinSetting{
		BonusMetric: "request_count",
		RequestCountTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 20, MinQuota: 5000, MaxQuota: 20000},
			{Threshold: 100, MinQuota: 20000, MaxQuota: 100000},
		},
		QuotaConsumedTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 100000, MinQuota: 10000, MaxQuota: 50000},
		},
	}
	require.Len(t, setting.ActiveTiers(), 2)
	setting.BonusMetric = "quota_consumed"
	require.Len(t, setting.ActiveTiers(), 1)
	require.Equal(t, 100000, setting.ActiveTiers()[0].Threshold)
	// 按次档位未被按额度档位覆盖
	require.Equal(t, 20, setting.RequestCountTiers[0].Threshold)
}

func TestEvaluateCheckinRewardUsesHigherMaxQuota(t *testing.T) {
	setting := &operation_setting.CheckinSetting{
		RequestCountTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 10, MinQuota: 1000, MaxQuota: 5000},
		},
		QuotaConsumedTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 100, MinQuota: 2000, MaxQuota: 20000},
		},
	}

	decision := EvaluateCheckinReward(setting, 10, 100)
	require.Equal(t, 10, decision.RequestCount.Tier.Threshold)
	require.Equal(t, 100, decision.QuotaConsumed.Tier.Threshold)
	require.Equal(t, "quota_consumed", decision.Selected.Metric)
	require.Equal(t, 20000, decision.Selected.Tier.MaxQuota)

	// 两套奖励只选择一套，且固定范围内随机一次；定值档位便于断言。
	setting.RequestCountTiers[0].MinQuota = 7000
	setting.RequestCountTiers[0].MaxQuota = 7000
	setting.QuotaConsumedTiers[0].MinQuota = 9000
	setting.QuotaConsumedTiers[0].MaxQuota = 9000
	reward, decision := SelectCheckinReward(setting, 10, 100)
	require.Equal(t, 9000, reward)
	require.Equal(t, "quota_consumed", decision.Selected.Metric)
}

func TestEvaluateCheckinRewardTiePrefersRequestCount(t *testing.T) {
	setting := &operation_setting.CheckinSetting{
		RequestCountTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 20, MinQuota: 1000, MaxQuota: 5000},
		},
		QuotaConsumedTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 100, MinQuota: 3000, MaxQuota: 5000},
		},
	}
	decision := EvaluateCheckinReward(setting, 20, 100)
	require.Equal(t, "request_count", decision.Selected.Metric)
}

func TestEvaluateCheckinRewardAllowsZeroQuotaUsage(t *testing.T) {
	setting := &operation_setting.CheckinSetting{
		RequestCountTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 10, MinQuota: 1000, MaxQuota: 1000},
		},
		QuotaConsumedTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 1, MinQuota: 2000, MaxQuota: 2000},
		},
	}
	decision := EvaluateCheckinReward(setting, 10, 0)
	require.Equal(t, "request_count", decision.Selected.Metric)
	require.Equal(t, 1000, selectTierReward(decision.Selected.Tier))
}

func TestSelectCheckinRewardOnlyQuotaConsumedMatches(t *testing.T) {
	setting := &operation_setting.CheckinSetting{
		RequestCountTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 100, MinQuota: 1000, MaxQuota: 5000},
		},
		QuotaConsumedTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 1000, MinQuota: 6000, MaxQuota: 6000},
		},
	}
	reward, decision := SelectCheckinReward(setting, 10, 1000)
	require.Nil(t, decision.RequestCount.Tier)
	require.Equal(t, "quota_consumed", decision.Selected.Metric)
	require.Equal(t, 6000, reward)
}

func TestSelectCheckinRewardZeroWhenBothMetricsMiss(t *testing.T) {
	setting := &operation_setting.CheckinSetting{
		RequestCountTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 100, MinQuota: 1000, MaxQuota: 5000},
		},
		QuotaConsumedTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 1000, MinQuota: 6000, MaxQuota: 10000},
		},
	}
	reward, decision := SelectCheckinReward(setting, 10, 100)
	require.Nil(t, decision.Selected.Tier)
	require.Empty(t, decision.Selected.Metric)
	require.Zero(t, reward)

	reward, decision = SelectCheckinReward(nil, 1000, 1000)
	require.Nil(t, decision.Selected.Tier)
	require.Zero(t, reward)
}
