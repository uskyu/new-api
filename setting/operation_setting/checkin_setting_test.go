package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestValidateAndNormalizeCheckinTiers(t *testing.T) {
	// 乱序档位：保存时按 threshold 升序规范化
	raw := `[{"threshold":100,"min_quota":5000,"max_quota":50000},{"threshold":50,"min_quota":2000,"max_quota":20000}]`
	normalized, err := ValidateAndNormalizeCheckinTiers(raw)
	require.NoError(t, err)
	require.Equal(t,
		`[{"threshold":50,"min_quota":2000,"max_quota":20000},{"threshold":100,"min_quota":5000,"max_quota":50000}]`,
		normalized)
}

func TestValidateAndNormalizeCheckinTiersEmpty(t *testing.T) {
	normalized, err := ValidateAndNormalizeCheckinTiers("")
	require.NoError(t, err)
	require.Equal(t, "[]", normalized)
	normalized, err = ValidateAndNormalizeCheckinTiers("null")
	require.NoError(t, err)
	require.Equal(t, "[]", normalized)
}

func TestValidateAndNormalizeCheckinTiersRejectsInvalid(t *testing.T) {
	// 重复门槛
	_, err := ValidateAndNormalizeCheckinTiers(
		`[{"threshold":50,"min_quota":2000,"max_quota":20000},{"threshold":50,"min_quota":3000,"max_quota":30000}]`)
	require.Error(t, err)
	// 负数门槛
	_, err = ValidateAndNormalizeCheckinTiers(
		`[{"threshold":-1,"min_quota":2000,"max_quota":20000}]`)
	require.Error(t, err)
	// 最高奖励低于最低奖励
	_, err = ValidateAndNormalizeCheckinTiers(
		`[{"threshold":50,"min_quota":5000,"max_quota":2000}]`)
	require.Error(t, err)
	// 非法 JSON
	_, err = ValidateAndNormalizeCheckinTiers("not-json")
	require.Error(t, err)
	// 对象而不是数组
	_, err = ValidateAndNormalizeCheckinTiers(`{"threshold":50}`)
	require.Error(t, err)
}

func TestSortedUniqueTiers(t *testing.T) {
	tiers := []CheckinBonusTier{
		{Threshold: 100, MinQuota: 5000, MaxQuota: 50000},
		{Threshold: 50, MinQuota: 2000, MaxQuota: 20000},
		{Threshold: 50, MinQuota: 9999, MaxQuota: 99999}, // 重复门槛保留首条
	}
	unique := SortedUniqueTiers(tiers)
	require.Len(t, unique, 2)
	require.Equal(t, 50, unique[0].Threshold)
	require.Equal(t, 2000, unique[0].MinQuota)
	require.Equal(t, 100, unique[1].Threshold)
	// 不修改原数组
	require.Len(t, tiers, 3)
}

// 验证校验与规范化函数走 common 的 JSON 包装（保证 JSON 一致性约定）。
func TestValidateAndNormalizeRoundTripThroughCommon(t *testing.T) {
	tiers := []CheckinBonusTier{
		{Threshold: 50, MinQuota: 2000, MaxQuota: 20000},
	}
	raw, err := common.Marshal(tiers)
	require.NoError(t, err)
	normalized, err := ValidateAndNormalizeCheckinTiers(string(raw))
	require.NoError(t, err)
	require.Equal(t, string(raw), normalized)
}
