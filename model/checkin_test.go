package model

import (
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
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
		MinQuota:     1000,
		MaxQuota:     1000,
		BonusEnabled: true,
		RequestCountTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 50, MinQuota: 2000, MaxQuota: 2000},
		},
	}
	require.Equal(t, 1000, selectCheckinReward(setting, 10))
	require.Equal(t, 2000, selectCheckinReward(setting, 50))
}

func TestSelectCheckinRewardFallsBackToBaseForInvalidTier(t *testing.T) {
	setting := &operation_setting.CheckinSetting{
		MinQuota:     1000,
		MaxQuota:     1000,
		BonusEnabled: true,
		RequestCountTiers: []operation_setting.CheckinBonusTier{
			{Threshold: 50, MinQuota: 5000, MaxQuota: 2000},
		},
	}
	require.Equal(t, 1000, selectCheckinReward(setting, 50))
}

func TestActiveTiersIndependentPerMetric(t *testing.T) {
	// 两套档位互不干扰：切换指标后各用各的档位
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

// ---------------------------------------------------------------------------
// 签到门槛（功能 2 + 功能 3）
//
// 这些测试用私有 helper 自建数据，不依赖、也不修改 risk_control_test.go 里的
// setupRiskControlTest / createRiskUsers（那属于别的工作流）。
// ---------------------------------------------------------------------------

// setupCheckinEligibilityTest 迁移本测试用到的表，清理 users / checkins / tokens / top_ups 并注册 t.Cleanup。
// 这里显式 AutoMigrate(&Checkin{})：共享的 TestMain 只迁移了 users/tokens/top_ups 等，
// 未包含 checkins；在本文件内自建可避免改动他人的测试文件。
func setupCheckinEligibilityTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Checkin{}, &TopUp{}, &Token{}, &User{}))
	truncate := func() {
		DB.Exec("DELETE FROM checkins")
		DB.Exec("DELETE FROM top_ups")
		DB.Exec("DELETE FROM tokens")
		DB.Exec("DELETE FROM users")
	}
	truncate()
	t.Cleanup(truncate)
}

// createCheckinTestUser 创建一个带唯一用户名 / 邀请码的测试用户。
func createCheckinTestUser(t *testing.T, suffix string) User {
	t.Helper()
	// aff_code 在 MySQL 上是 varchar(32)，username 是 varchar(191)。
	// 用固定长度前缀 + CRC32 派生，保证任意长度的 suffix 都不会超长
	// （SQLite 不强制列长度，只有真实 MySQL 才会暴露该问题）。
	var code [4]byte
	binary.BigEndian.PutUint32(code[:], crc32.ChecksumIEEE([]byte(suffix)))
	short := hex.EncodeToString(code[:])
	user := User{
		Username: "checkin-user-" + suffix,
		Password: "test-password",
		AffCode:  "ck-" + short,
		Role:     common.RoleCommonUser,
	}
	if len(user.Username) > 191 {
		user.Username = "checkin-user-" + short
	}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

// createCheckinTestTopUp 创建一条指定渠道 / 状态的充值记录（amount 单位为元）。
func createCheckinTestTopUp(t *testing.T, userId int, amount int64, provider string, status string, tradeNo string) {
	t.Helper()
	topUp := &TopUp{
		UserId:          userId,
		Amount:          amount,
		Money:           float64(amount),
		TradeNo:         tradeNo,
		PaymentMethod:   "alipay",
		PaymentProvider: provider,
		CreateTime:      time.Now().Unix(),
		Status:          status,
	}
	require.NoError(t, DB.Create(topUp).Error)
}

// createCheckinTestToken 为用户创建一个令牌并设置 accessed_time。
func createCheckinTestToken(t *testing.T, userId int, key string, accessedTime int64) {
	t.Helper()
	token := &Token{
		UserId:       userId,
		Key:          key,
		Name:         "checkin-token-" + key,
		AccessedTime: accessedTime,
		ExpiredTime:  -1,
		Status:       common.TokenStatusEnabled,
	}
	require.NoError(t, DB.Create(token).Error)
}

func eligibilityTestSetting(minRecharge float64, inactiveDays int) *operation_setting.CheckinSetting {
	return &operation_setting.CheckinSetting{
		Enabled:           true,
		MinRechargeAmount: minRecharge,
		InactiveDays:      inactiveDays,
	}
}

func TestGetUserEpayRechargeAmountOnlyCountsEpaySuccess(t *testing.T) {
	setupCheckinEligibilityTest(t)
	user := createCheckinTestUser(t, "epay-sum")

	// 两条成功的 EPAY 充值：100 + 50 = 150 元。
	createCheckinTestTopUp(t, user.Id, 100, PaymentProviderEpay, common.TopUpStatusSuccess, "epay-success-1")
	createCheckinTestTopUp(t, user.Id, 50, PaymentProviderEpay, common.TopUpStatusSuccess, "epay-success-2")
	// 不应计入：非 EPAY 渠道、非 success 状态。
	createCheckinTestTopUp(t, user.Id, 999, PaymentProviderStripe, common.TopUpStatusSuccess, "stripe-success")
	createCheckinTestTopUp(t, user.Id, 888, PaymentProviderEpay, common.TopUpStatusPending, "epay-pending")
	createCheckinTestTopUp(t, user.Id, 777, PaymentProviderEpay, common.TopUpStatusExpired, "epay-expired")

	amount, err := GetUserEpayRechargeAmount(user.Id)
	require.NoError(t, err)
	require.Equal(t, float64(150), amount)

	// 其他用户的充值不计入。
	other := createCheckinTestUser(t, "epay-other")
	createCheckinTestTopUp(t, other.Id, 500, PaymentProviderEpay, common.TopUpStatusSuccess, "epay-other-1")
	amount, err = GetUserEpayRechargeAmount(user.Id)
	require.NoError(t, err)
	require.Equal(t, float64(150), amount)

	// 从未充值返回 0（而不是报错）。
	amount, err = GetUserEpayRechargeAmount(other.Id + 100000)
	require.NoError(t, err)
	require.Equal(t, float64(0), amount)

	// 非法 userId 直接返回 0。
	amount, err = GetUserEpayRechargeAmount(0)
	require.NoError(t, err)
	require.Equal(t, float64(0), amount)
}

func TestGetUserLastApiAccessedTime(t *testing.T) {
	setupCheckinEligibilityTest(t)
	user := createCheckinTestUser(t, "accessed")

	// 无令牌 → 0。
	lastAccessed, err := GetUserLastApiAccessedTime(user.Id)
	require.NoError(t, err)
	require.EqualValues(t, 0, lastAccessed)

	// 多令牌取最大值。
	createCheckinTestToken(t, user.Id, "checkin-key-1", 1000)
	createCheckinTestToken(t, user.Id, "checkin-key-2", 3000)
	createCheckinTestToken(t, user.Id, "checkin-key-3", 2000)
	lastAccessed, err = GetUserLastApiAccessedTime(user.Id)
	require.NoError(t, err)
	require.EqualValues(t, 3000, lastAccessed)

	// 他人令牌不计入。
	other := createCheckinTestUser(t, "accessed-other")
	createCheckinTestToken(t, other.Id, "checkin-key-other", 9000)
	lastAccessed, err = GetUserLastApiAccessedTime(user.Id)
	require.NoError(t, err)
	require.EqualValues(t, 3000, lastAccessed)

	// 非法 userId 返回 0。
	lastAccessed, err = GetUserLastApiAccessedTime(0)
	require.NoError(t, err)
	require.EqualValues(t, 0, lastAccessed)
}

func TestCheckinEligibilityRechargeThreshold(t *testing.T) {
	setupCheckinEligibilityTest(t)

	// 门槛 10 元。
	setting := eligibilityTestSetting(10, 0)

	// 未达门槛：充值 5 元 → recharge_required。
	below := createCheckinTestUser(t, "threshold-below")
	createCheckinTestTopUp(t, below.Id, 5, PaymentProviderEpay, common.TopUpStatusSuccess, "threshold-below-1")
	err := checkCheckinEligibility(below.Id, setting)
	require.Error(t, err)
	require.Contains(t, err.Error(), "还差")

	eligibility, eligErr := GetCheckinEligibility(below.Id, setting)
	require.NoError(t, eligErr)
	require.False(t, eligibility.CanCheckin)
	require.Equal(t, CheckinReasonRechargeRequired, eligibility.Reason)
	require.Equal(t, float64(10), eligibility.MinRechargeAmount)
	require.Equal(t, float64(5), eligibility.UserRechargeAmount)

	// 恰好等于门槛：充值 10 元 → 通过（边界用 <，不是 <=）。
	exact := createCheckinTestUser(t, "threshold-exact")
	createCheckinTestTopUp(t, exact.Id, 10, PaymentProviderEpay, common.TopUpStatusSuccess, "threshold-exact-1")
	require.NoError(t, checkCheckinEligibility(exact.Id, setting))
	eligibility, eligErr = GetCheckinEligibility(exact.Id, setting)
	require.NoError(t, eligErr)
	require.True(t, eligibility.CanCheckin)
	require.Equal(t, CheckinReasonNone, eligibility.Reason)

	// 超过门槛：充值 20 元 → 通过。
	above := createCheckinTestUser(t, "threshold-above")
	createCheckinTestTopUp(t, above.Id, 20, PaymentProviderEpay, common.TopUpStatusSuccess, "threshold-above-1")
	require.NoError(t, checkCheckinEligibility(above.Id, setting))

	// 小额充值累计：3 + 3 + 4 = 10 元恰好达成。
	accumulated := createCheckinTestUser(t, "threshold-accumulated")
	createCheckinTestTopUp(t, accumulated.Id, 3, PaymentProviderEpay, common.TopUpStatusSuccess, "threshold-acc-1")
	createCheckinTestTopUp(t, accumulated.Id, 3, PaymentProviderEpay, common.TopUpStatusSuccess, "threshold-acc-2")
	createCheckinTestTopUp(t, accumulated.Id, 4, PaymentProviderEpay, common.TopUpStatusSuccess, "threshold-acc-3")
	require.NoError(t, checkCheckinEligibility(accumulated.Id, setting))

	// 小数门槛 0.5 元：充值 0.5 元恰好达成（float64 支持小数元）。
	fractionalSetting := eligibilityTestSetting(0.5, 0)
	fractional := createCheckinTestUser(t, "threshold-fractional")
	createCheckinTestTopUp(t, fractional.Id, 1, PaymentProviderEpay, common.TopUpStatusSuccess, "threshold-frac-1")
	require.NoError(t, checkCheckinEligibility(fractional.Id, fractionalSetting))
}

func TestCheckinEligibilityZeroConfigMeansUnlimited(t *testing.T) {
	setupCheckinEligibilityTest(t)
	user := createCheckinTestUser(t, "zero-config")

	// 两个配置都为 0（默认）：从未充值、从未调用 API，也应可签到。
	setting := eligibilityTestSetting(0, 0)
	require.NoError(t, checkCheckinEligibility(user.Id, setting))

	eligibility, err := GetCheckinEligibility(user.Id, setting)
	require.NoError(t, err)
	require.True(t, eligibility.CanCheckin)
	require.Equal(t, CheckinReasonNone, eligibility.Reason)
	require.Equal(t, float64(0), eligibility.MinRechargeAmount)
	require.Equal(t, 0, eligibility.InactiveDays)

	// 仅 InactiveDays=0：即使从未调用 API，充值门槛设为 0 时不受不活跃限制。
	require.NoError(t, checkCheckinEligibility(user.Id, eligibilityTestSetting(0, 0)))

	// 仅 MinRechargeAmount=0：不限制充值，但不活跃仍生效（超期会被拒）。
	require.Error(t, checkCheckinEligibility(user.Id, eligibilityTestSetting(0, 30)))
}

func TestCheckinEligibilityInactiveDays(t *testing.T) {
	setupCheckinEligibilityTest(t)
	now := common.GetTimestamp()
	fiveDaysAgo := now - 5*86400
	fortyDaysAgo := now - 40*86400

	// 从未充值 + 最近 5 天调用过 API + N=30 → 可签到。
	active := createCheckinTestUser(t, "inactive-active")
	createCheckinTestToken(t, active.Id, "inactive-key-active", fiveDaysAgo)
	require.NoError(t, checkCheckinEligibility(active.Id, eligibilityTestSetting(0, 30)))

	// 从未充值 + 最近一次调用在 40 天前 + N=30 → inactive。
	stale := createCheckinTestUser(t, "inactive-stale")
	createCheckinTestToken(t, stale.Id, "inactive-key-stale", fortyDaysAgo)
	err := checkCheckinEligibility(stale.Id, eligibilityTestSetting(0, 30))
	require.Error(t, err)
	require.Contains(t, err.Error(), "未调用 API")

	eligibility, eligErr := GetCheckinEligibility(stale.Id, eligibilityTestSetting(0, 30))
	require.NoError(t, eligErr)
	require.False(t, eligibility.CanCheckin)
	require.Equal(t, CheckinReasonInactive, eligibility.Reason)
	require.EqualValues(t, fortyDaysAgo, eligibility.LastApiUsedAt)
	require.Equal(t, 30, eligibility.InactiveDays)

	// 从未充值 + 从未调用（无令牌）+ N=1 → inactive。
	never := createCheckinTestUser(t, "inactive-never")
	require.Error(t, checkCheckinEligibility(never.Id, eligibilityTestSetting(0, 1)))

	// N=0 时不限制：40 天未调用也可签到。
	require.NoError(t, checkCheckinEligibility(stale.Id, eligibilityTestSetting(0, 0)))
}

func TestCheckinEligibilityRechargedUserBypassesInactive(t *testing.T) {
	setupCheckinEligibilityTest(t)
	now := common.GetTimestamp()

	// 充值过（哪怕是很久的充值）+ 长期不调用 API → 仍可签到（充值解锁优先）。
	recharged := createCheckinTestUser(t, "recharged-inactive")
	createCheckinTestTopUp(t, recharged.Id, 100, PaymentProviderEpay, common.TopUpStatusSuccess, "recharged-1")
	createCheckinTestToken(t, recharged.Id, "recharged-key", now-100*86400)

	// 门槛 10 元已满足，N=30 天。
	setting := eligibilityTestSetting(10, 30)
	require.NoError(t, checkCheckinEligibility(recharged.Id, setting))

	eligibility, err := GetCheckinEligibility(recharged.Id, setting)
	require.NoError(t, err)
	require.True(t, eligibility.CanCheckin)
	require.Equal(t, CheckinReasonNone, eligibility.Reason)
	require.Equal(t, float64(100), eligibility.UserRechargeAmount)

	// 反向确认：同样长期不调用，但从未充值的用户被拒。
	neverRecharged := createCheckinTestUser(t, "never-recharged")
	createCheckinTestToken(t, neverRecharged.Id, "never-recharged-key", now-100*86400)
	err = checkCheckinEligibility(neverRecharged.Id, eligibilityTestSetting(10, 30))
	require.Error(t, err)
	// 未充值先触发充值门槛（按契约顺序：先充值门槛，后不活跃）。
	require.Contains(t, err.Error(), "还差")
}

func TestCheckinEligibilityRechargeAndInactiveCombination(t *testing.T) {
	setupCheckinEligibilityTest(t)
	now := common.GetTimestamp()

	// 组合 A：门槛 100 元 + N=30 天
	// 用户充值 50 元（够解锁不活跃但不够门槛）→ 仍被充值门槛拒绝。
	partial := createCheckinTestUser(t, "combo-partial")
	createCheckinTestTopUp(t, partial.Id, 50, PaymentProviderEpay, common.TopUpStatusSuccess, "combo-partial-1")
	createCheckinTestToken(t, partial.Id, "combo-partial-key", now-100*86400)
	err := checkCheckinEligibility(partial.Id, eligibilityTestSetting(100, 30))
	require.Error(t, err)
	require.Contains(t, err.Error(), "还差")
	require.Contains(t, err.Error(), "50")

	// 组合 B：门槛 10 元 + N=30 天，充值 10 元恰好达标但长期不调用 → 仍可签到。
	exact := createCheckinTestUser(t, "combo-exact")
	createCheckinTestTopUp(t, exact.Id, 10, PaymentProviderEpay, common.TopUpStatusSuccess, "combo-exact-1")
	createCheckinTestToken(t, exact.Id, "combo-exact-key", now-100*86400)
	require.NoError(t, checkCheckinEligibility(exact.Id, eligibilityTestSetting(10, 30)))

	// 组合 C：门槛 10 元 + N=30 天，未充值但最近调用过 → 仍被充值门槛拒绝。
	// 充值门槛（功能 2）与不活跃（功能 3）是两条独立门槛：不活跃的恢复规则
	// 不能绕过充值门槛，只有「累计充值达到门槛」才算满足。
	active := createCheckinTestUser(t, "combo-active")
	createCheckinTestToken(t, active.Id, "combo-active-key", now-2*86400)
	err = checkCheckinEligibility(active.Id, eligibilityTestSetting(10, 30))
	require.Error(t, err)
	require.Contains(t, err.Error(), "还差")

	// 组合 D：门槛 0 + N=30 天，未充值但最近调用过 → 通过（只有不活跃门槛生效）。
	require.NoError(t, checkCheckinEligibility(active.Id, eligibilityTestSetting(0, 30)))
}

func TestUserCheckinBlockedByEligibility(t *testing.T) {
	setupCheckinEligibilityTest(t)

	// 保存并恢复全局签到配置，避免污染其他测试。
	originalSetting := *operation_setting.GetCheckinSetting()
	t.Cleanup(func() {
		*operation_setting.GetCheckinSetting() = originalSetting
	})
	current := operation_setting.GetCheckinSetting()
	current.Enabled = true
	current.MinQuota = 1000
	current.MaxQuota = 1000
	current.BonusEnabled = false
	current.MinRechargeAmount = 10
	current.InactiveDays = 0

	// 未达门槛：UserCheckin 直接返回错误，且不写入 checkin 记录。
	below := createCheckinTestUser(t, "usercheckin-below")
	_, err := UserCheckin(below.Id)
	require.Error(t, err)
	require.Contains(t, err.Error(), "还差")

	var blockedCount int64
	require.NoError(t, DB.Model(&Checkin{}).Where("user_id = ?", below.Id).Count(&blockedCount).Error)
	require.EqualValues(t, 0, blockedCount)

	// 达到门槛后可以签到，并且记录被写入。
	above := createCheckinTestUser(t, "usercheckin-above")
	createCheckinTestTopUp(t, above.Id, 10, PaymentProviderEpay, common.TopUpStatusSuccess, "usercheckin-above-1")
	checkin, err := UserCheckin(above.Id)
	require.NoError(t, err)
	require.NotNil(t, checkin)
	require.Equal(t, above.Id, checkin.UserId)

	var successCount int64
	require.NoError(t, DB.Model(&Checkin{}).Where("user_id = ?", above.Id).Count(&successCount).Error)
	require.EqualValues(t, 1, successCount)

	// 已签到用户再次签到仍然报「今日已签到」。
	_, err = UserCheckin(above.Id)
	require.Error(t, err)
	require.Contains(t, err.Error(), "今日已签到")

	// 恢复功能关闭，避免影响后续测试。
	current.Enabled = false
}

func TestCheckinEligibilityInactiveBlocksUserCheckin(t *testing.T) {
	setupCheckinEligibilityTest(t)

	originalSetting := *operation_setting.GetCheckinSetting()
	t.Cleanup(func() {
		*operation_setting.GetCheckinSetting() = originalSetting
	})
	current := operation_setting.GetCheckinSetting()
	current.Enabled = true
	current.MinQuota = 1000
	current.MaxQuota = 1000
	current.BonusEnabled = false
	current.MinRechargeAmount = 0
	current.InactiveDays = 30

	// 从未充值 + 长期不调用 → UserCheckin 被拒且不落库。
	stale := createCheckinTestUser(t, "usercheckin-stale")
	createCheckinTestToken(t, stale.Id, "usercheckin-stale-key", common.GetTimestamp()-100*86400)
	_, err := UserCheckin(stale.Id)
	require.Error(t, err)
	require.Contains(t, err.Error(), "未调用 API")

	var staleCount int64
	require.NoError(t, DB.Model(&Checkin{}).Where("user_id = ?", stale.Id).Count(&staleCount).Error)
	require.EqualValues(t, 0, staleCount)

	// 最近调用过 API 的用户可以签到。
	fresh := createCheckinTestUser(t, "usercheckin-fresh")
	createCheckinTestToken(t, fresh.Id, "usercheckin-fresh-key", common.GetTimestamp())
	checkin, err := UserCheckin(fresh.Id)
	require.NoError(t, err)
	require.NotNil(t, checkin)

	current.Enabled = false
}

func TestFormatCheckinAmount(t *testing.T) {
	require.Equal(t, "0", formatCheckinAmount(0))
	require.Equal(t, "10", formatCheckinAmount(10))
	require.Equal(t, "0.5", formatCheckinAmount(0.5))
	require.Equal(t, "10.25", formatCheckinAmount(10.25))
	require.Equal(t, "99.9", formatCheckinAmount(99.9))
}
