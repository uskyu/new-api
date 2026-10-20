package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 邀请奖励（阶段 B）测试
//
// 本文件是阶段 B 唯一新增的测试文件，所需 helper 均以 affiliate 前缀私有定义，
// 不复用也不修改 risk_control_test.go（属于工作流 A）。
// ---------------------------------------------------------------------------

const affiliateTestQuotaPerUnit = 100

// resetAffiliateSetting 把邀请奖励配置恢复为契约默认值（false/false/100/100）。
func resetAffiliateSetting() {
	setting := operation_setting.GetAffiliateSetting()
	setting.InviteeRewardRequireRecharge = false
	setting.InviterWithdrawRequireInviteeRecharge = false
	setting.UnrechargedWithdrawRatio = 100
	setting.RechargedWithdrawRatio = 100
}

// setupAffiliateTest 准备邀请奖励测试所需的表、全局额度与配置，并在结束后还原。
func setupAffiliateTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&AffiliateReward{}))

	prevQuotaPerUnit := common.QuotaPerUnit
	prevQuotaForInviter := common.QuotaForInviter
	prevQuotaForInvitee := common.QuotaForInvitee
	prevQuotaForNewUser := common.QuotaForNewUser
	paymentSetting := operation_setting.GetPaymentSetting()
	prevComplianceConfirmed := paymentSetting.ComplianceConfirmed
	prevComplianceVersion := paymentSetting.ComplianceTermsVersion

	// 划转最小额度校验依赖 QuotaPerUnit，测试用小额度便于构造比例场景
	common.QuotaPerUnit = affiliateTestQuotaPerUnit
	resetAffiliateSetting()

	t.Cleanup(func() {
		DB.Exec("DELETE FROM affiliate_rewards")
		DB.Exec("DELETE FROM logs")
		DB.Exec("DELETE FROM users")
		common.QuotaPerUnit = prevQuotaPerUnit
		common.QuotaForInviter = prevQuotaForInviter
		common.QuotaForInvitee = prevQuotaForInvitee
		common.QuotaForNewUser = prevQuotaForNewUser
		paymentSetting.ComplianceConfirmed = prevComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = prevComplianceVersion
		resetAffiliateSetting()
	})
}

// confirmAffiliateComplianceOn 打开支付合规开关，使注册邀请奖励分支生效。
func confirmAffiliateComplianceOn(t *testing.T) {
	t.Helper()
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
}

// createAffiliateUser 创建一个测试用户；AffCode 必须唯一。
func createAffiliateUser(t *testing.T, username string, affQuota int) *User {
	t.Helper()
	user := &User{
		Username: username,
		Password: "test-password",
		AffCode:  "aff-" + username,
		Role:     common.RoleCommonUser,
		AffQuota: affQuota,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

// createAffiliateRecord 写入一条邀请奖励台账。
func createAffiliateRecord(t *testing.T, record AffiliateReward) *AffiliateReward {
	t.Helper()
	require.NoError(t, DB.Create(&record).Error)
	return &record
}

// loadAffiliateUser 重新从数据库读取用户。
func loadAffiliateUser(t *testing.T, userId int) *User {
	t.Helper()
	user := &User{}
	require.NoError(t, DB.First(user, userId).Error)
	return user
}

// loadAffiliateRecord 重新从数据库读取台账记录。
func loadAffiliateRecord(t *testing.T, id int) *AffiliateReward {
	t.Helper()
	record := &AffiliateReward{}
	require.NoError(t, DB.First(record, id).Error)
	return record
}

// ---------------------------------------------------------------------------
// 1. 默认关闭时行为不变：withdrawable == AffQuota，locked == 0
// ---------------------------------------------------------------------------

func TestAffiliateDefaultSettingKeepsLegacyBehavior(t *testing.T) {
	setupAffiliateTest(t)

	// 纯 legacy：无台账的历史额度全额可划转
	legacyUser := createAffiliateUser(t, "aff-default-legacy", 3000)
	detail, err := GetAffiliateWithdrawable(legacyUser.Id)
	require.NoError(t, err)
	assert.Equal(t, 3000, detail.Withdrawable)
	assert.Equal(t, 0, detail.Locked)
	assert.Equal(t, 3000, detail.LegacyRemaining)
	assert.Equal(t, 0, detail.UnlockedRemaining)
	assert.Equal(t, 0, detail.LockedRemaining)

	// 有台账但默认设置全关：未解锁记录也按 100% 计入可划转
	inviter := createAffiliateUser(t, "aff-default-ledger", 2000)
	invitee := createAffiliateUser(t, "aff-default-invitee", 0)
	createAffiliateRecord(t, AffiliateReward{
		InviterId:    inviter.Id,
		InviteeId:    invitee.Id,
		InviterQuota: 2000,
		UnlockedAt:   0,
	})
	detail, err = GetAffiliateWithdrawable(inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, inviter.AffQuota, detail.Withdrawable)
	assert.Equal(t, inviter.AffQuota, detail.Withdrawable, "默认设置下 withdrawable 必须等于 AffQuota")
	assert.Equal(t, 0, detail.Locked)
	assert.Equal(t, 2000, detail.LockedRemaining)
	assert.Equal(t, 0, detail.LegacyRemaining)

	// 默认设置下划转行为与旧逻辑一致：AffQuota 全额可划
	user := loadAffiliateUser(t, inviter.Id)
	require.NoError(t, user.TransferAffQuotaToQuota(2000))
	user = loadAffiliateUser(t, inviter.Id)
	assert.Equal(t, 0, user.AffQuota)
	assert.Equal(t, 2000, user.Quota)
}

// ---------------------------------------------------------------------------
// 2. 需充值才解锁：未充值按比例折算，充值后全额解锁
// ---------------------------------------------------------------------------

func TestAffiliateUnlockRequiresInviteeRecharge(t *testing.T) {
	setupAffiliateTest(t)

	setting := operation_setting.GetAffiliateSetting()
	setting.InviterWithdrawRequireInviteeRecharge = true
	setting.UnrechargedWithdrawRatio = 50
	setting.RechargedWithdrawRatio = 100

	inviter := createAffiliateUser(t, "aff-unlock-inviter", 1000)
	invitee := createAffiliateUser(t, "aff-unlock-invitee", 0)
	record := createAffiliateRecord(t, AffiliateReward{
		InviterId:    inviter.Id,
		InviteeId:    invitee.Id,
		InviterQuota: 1000,
		UnlockedAt:   0,
	})

	before, err := GetAffiliateWithdrawable(inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, 500, before.LockedRemaining, "未充值按 50% 折算")
	assert.Equal(t, 500, before.Withdrawable)
	assert.Equal(t, 500, before.Locked)

	// 被邀请人充值 → 解锁
	require.NoError(t, SettleAffiliateRewards(invitee.Id))
	assert.NotZero(t, loadAffiliateRecord(t, record.Id).UnlockedAt)

	after, err := GetAffiliateWithdrawable(inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, 1000, after.Withdrawable)
	assert.Equal(t, 0, after.Locked)
	assert.Equal(t, 1000, after.UnlockedRemaining)
	assert.Equal(t, 0, after.LockedRemaining)
}

// ---------------------------------------------------------------------------
// 3. 幂等：SettleAffiliateRewards 调两次只解锁/发放一次
// ---------------------------------------------------------------------------

func TestAffiliateSettleIsIdempotent(t *testing.T) {
	setupAffiliateTest(t)

	setting := operation_setting.GetAffiliateSetting()
	setting.InviteeRewardRequireRecharge = true

	inviter := createAffiliateUser(t, "aff-idem-inviter", 500)
	invitee := createAffiliateUser(t, "aff-idem-invitee", 0)
	record := createAffiliateRecord(t, AffiliateReward{
		InviterId:            inviter.Id,
		InviteeId:            invitee.Id,
		InviterQuota:         500,
		InviteeQuota:         300,
		InviteeRewardGranted: false,
	})

	require.NoError(t, SettleAffiliateRewards(invitee.Id))
	first := loadAffiliateUser(t, invitee.Id)
	require.Equal(t, 300, first.Quota)
	grantedRecord := loadAffiliateRecord(t, record.Id)
	require.True(t, grantedRecord.InviteeRewardGranted)
	require.NotZero(t, grantedRecord.InviteeRewardGrantedAt)
	firstGrantedAt := grantedRecord.InviteeRewardGrantedAt
	firstUnlockedAt := grantedRecord.UnlockedAt

	require.NoError(t, SettleAffiliateRewards(invitee.Id))
	second := loadAffiliateUser(t, invitee.Id)
	assert.Equal(t, 300, second.Quota, "重复调用不得二次加额度")

	again := loadAffiliateRecord(t, record.Id)
	assert.True(t, again.InviteeRewardGranted)
	assert.Equal(t, firstGrantedAt, again.InviteeRewardGrantedAt)
	assert.Equal(t, firstUnlockedAt, again.UnlockedAt)
	assert.EqualValues(t, 1, countAffiliateGrantLogs(t, invitee.Id), "解锁日志只应写一次")
}

// countAffiliateGrantLogs 统计「邀请奖励解锁」日志条数。
func countAffiliateGrantLogs(t *testing.T, userId int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ? AND content LIKE ?", userId, LogTypeSystem, "邀请奖励解锁%").
		Count(&count).Error)
	return count
}

// ---------------------------------------------------------------------------
// 4. 被邀请人奖励延迟发放：要求充值时注册不发，充值后发放一次
// ---------------------------------------------------------------------------

func TestAffiliateInviteeRewardDeferredUntilRecharge(t *testing.T) {
	setupAffiliateTest(t)

	confirmAffiliateComplianceOn(t)
	setting := operation_setting.GetAffiliateSetting()
	setting.InviteeRewardRequireRecharge = true
	common.QuotaForInviter = 400
	common.QuotaForInvitee = 250
	common.QuotaForNewUser = 0

	inviter := createAffiliateUser(t, "aff-defer-inviter", 0)

	invitee := &User{
		Username:  "aff-defer-invitee",
		Password:  "test-password",
		AffCode:   "aff-defer-invitee",
		InviterId: inviter.Id,
	}
	require.NoError(t, invitee.Insert(inviter.Id))

	inviteeId := invitee.Id
	require.NotZero(t, inviteeId)
	registered := loadAffiliateUser(t, inviteeId)
	assert.Equal(t, 0, registered.Quota, "要求充值时注册阶段不发放被邀请人奖励")

	inviterAfterRegister := loadAffiliateUser(t, inviter.Id)
	assert.Equal(t, 1, inviterAfterRegister.AffCount)
	assert.Equal(t, 400, inviterAfterRegister.AffQuota)

	var records []AffiliateReward
	require.NoError(t, DB.Where("invitee_id = ?", inviteeId).Find(&records).Error)
	require.Len(t, records, 1)
	assert.False(t, records[0].InviteeRewardGranted)
	assert.Zero(t, records[0].UnlockedAt)

	// 首次充值 → 解锁并发放一次
	require.NoError(t, SettleAffiliateRewards(inviteeId))
	assert.Equal(t, 250, loadAffiliateUser(t, inviteeId).Quota)

	// 再次充值（再次调用）不重复发放
	require.NoError(t, SettleAffiliateRewards(inviteeId))
	assert.Equal(t, 250, loadAffiliateUser(t, inviteeId).Quota)
	assert.EqualValues(t, 1, countAffiliateGrantLogs(t, inviteeId))
}

// ---------------------------------------------------------------------------
// 5. 混合池划转上限：超额被拒，刚好等于可划转成功
// ---------------------------------------------------------------------------

func TestAffiliateMixedPoolWithdrawLimit(t *testing.T) {
	setupAffiliateTest(t)

	setting := operation_setting.GetAffiliateSetting()
	setting.InviterWithdrawRequireInviteeRecharge = true
	setting.UnrechargedWithdrawRatio = 50
	setting.RechargedWithdrawRatio = 100

	inviter := createAffiliateUser(t, "aff-mixed-inviter", 2000)
	unlockedInvitee := createAffiliateUser(t, "aff-mixed-unlocked", 0)
	lockedInvitee := createAffiliateUser(t, "aff-mixed-locked", 0)

	unlockedRecord := createAffiliateRecord(t, AffiliateReward{
		InviterId:    inviter.Id,
		InviteeId:    unlockedInvitee.Id,
		InviterQuota: 1000,
		UnlockedAt:   common.GetTimestamp(),
	})
	lockedRecord := createAffiliateRecord(t, AffiliateReward{
		InviterId:    inviter.Id,
		InviteeId:    lockedInvitee.Id,
		InviterQuota: 1000,
		UnlockedAt:   0,
	})

	detail, err := GetAffiliateWithdrawable(inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, 1000, detail.UnlockedRemaining)
	assert.Equal(t, 500, detail.LockedRemaining)
	assert.Equal(t, 1500, detail.Withdrawable)
	assert.Equal(t, 500, detail.Locked)

	// 超额 1 分钱也不允许
	user := loadAffiliateUser(t, inviter.Id)
	err = user.TransferAffQuotaToQuota(1600)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "邀请额度不足")
	assert.Contains(t, err.Error(), "需被邀请人充值后解锁")

	// 刚好等于可划转则成功，且先扣已解锁池、再扣未解锁池
	user = loadAffiliateUser(t, inviter.Id)
	require.NoError(t, user.TransferAffQuotaToQuota(1500))

	user = loadAffiliateUser(t, inviter.Id)
	assert.Equal(t, 500, user.AffQuota)
	assert.Equal(t, 1500, user.Quota)

	assert.Equal(t, 1000, loadAffiliateRecord(t, unlockedRecord.Id).InviterWithdrawn)
	assert.Equal(t, 500, loadAffiliateRecord(t, lockedRecord.Id).InviterWithdrawn)

	after, err := GetAffiliateWithdrawable(inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, 0, after.Withdrawable)
	// 剩余 500 是锁定池里「未达比例上限」的部分，仍不可划转
	assert.Equal(t, 500, after.Locked)
}

// ---------------------------------------------------------------------------
// 6. 比例边界：LockedRatio = 0 时未解锁池一分不能划；= 100 时全额可划
// ---------------------------------------------------------------------------

func TestAffiliateLockedRatioBoundaries(t *testing.T) {
	setupAffiliateTest(t)

	setting := operation_setting.GetAffiliateSetting()
	setting.InviterWithdrawRequireInviteeRecharge = true

	inviter := createAffiliateUser(t, "aff-ratio-inviter", 1000)
	invitee := createAffiliateUser(t, "aff-ratio-invitee", 0)
	createAffiliateRecord(t, AffiliateReward{
		InviterId:    inviter.Id,
		InviteeId:    invitee.Id,
		InviterQuota: 1000,
		UnlockedAt:   0,
	})

	// LockedRatio = 0：未解锁池不可划转
	setting.UnrechargedWithdrawRatio = 0
	zero, err := GetAffiliateWithdrawable(inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, 0, zero.LockedRemaining)
	assert.Equal(t, 0, zero.Withdrawable)
	assert.Equal(t, 1000, zero.Locked)

	user := loadAffiliateUser(t, inviter.Id)
	require.Error(t, user.TransferAffQuotaToQuota(affiliateTestQuotaPerUnit))

	// LockedRatio = 100：未解锁池全额可划转
	setting.UnrechargedWithdrawRatio = 100
	full, err := GetAffiliateWithdrawable(inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, 1000, full.LockedRemaining)
	assert.Equal(t, 1000, full.Withdrawable)
	assert.Equal(t, 0, full.Locked)

	user = loadAffiliateUser(t, inviter.Id)
	require.NoError(t, user.TransferAffQuotaToQuota(1000))
	assert.Equal(t, 0, loadAffiliateUser(t, inviter.Id).AffQuota)

	// 越界比例按 0..100 clamp
	setting.UnrechargedWithdrawRatio = 260
	assert.Equal(t, 100, setting.LockedRatio())
	setting.UnrechargedWithdrawRatio = -20
	assert.Equal(t, 0, setting.LockedRatio())
}

// ---------------------------------------------------------------------------
// 7. 重复划转不滚超额：连续两次划转后总额不超过 lockedLimit
// ---------------------------------------------------------------------------

func TestAffiliateRepeatedWithdrawDoesNotExceedLockedLimit(t *testing.T) {
	setupAffiliateTest(t)

	setting := operation_setting.GetAffiliateSetting()
	setting.InviterWithdrawRequireInviteeRecharge = true
	setting.UnrechargedWithdrawRatio = 50
	setting.RechargedWithdrawRatio = 100

	inviter := createAffiliateUser(t, "aff-repeat-inviter", 1000)
	invitee := createAffiliateUser(t, "aff-repeat-invitee", 0)
	record := createAffiliateRecord(t, AffiliateReward{
		InviterId:    inviter.Id,
		InviteeId:    invitee.Id,
		InviterQuota: 1000,
		UnlockedAt:   0,
	})

	// lockedLimit = floor(1000 * 50 / 100) = 500
	user := loadAffiliateUser(t, inviter.Id)
	require.NoError(t, user.TransferAffQuotaToQuota(300))

	user = loadAffiliateUser(t, inviter.Id)
	require.NoError(t, user.TransferAffQuotaToQuota(200))
	assert.Equal(t, 500, loadAffiliateRecord(t, record.Id).InviterWithdrawn)

	// 第三次：额度已用完
	user = loadAffiliateUser(t, inviter.Id)
	require.Error(t, user.TransferAffQuotaToQuota(affiliateTestQuotaPerUnit))

	withdrawn := loadAffiliateRecord(t, record.Id).InviterWithdrawn
	assert.LessOrEqual(t, withdrawn, 500, "重复划转不得把比例滚成超额")

	detail, err := GetAffiliateWithdrawable(inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, 0, detail.Withdrawable)
	// 剩余 500 是超出比例上限的锁定额（AffQuota - withdrawable）
	assert.Equal(t, 500, detail.Locked)
}

// ---------------------------------------------------------------------------
// 8. legacy 可提：无台账的历史 AffQuota 全额可划转
// ---------------------------------------------------------------------------

func TestAffiliateLegacyQuotaWithdrawable(t *testing.T) {
	setupAffiliateTest(t)

	setting := operation_setting.GetAffiliateSetting()
	setting.InviterWithdrawRequireInviteeRecharge = true
	setting.UnrechargedWithdrawRatio = 0

	// 无任何台账记录的历史邀请额度
	legacyUser := createAffiliateUser(t, "aff-legacy-user", 800)

	detail, err := GetAffiliateWithdrawable(legacyUser.Id)
	require.NoError(t, err)
	assert.Equal(t, 800, detail.LegacyRemaining)
	assert.Equal(t, 800, detail.Withdrawable, "legacy 额度不受未充值比例限制")
	assert.Equal(t, 0, detail.Locked)

	user := loadAffiliateUser(t, legacyUser.Id)
	require.NoError(t, user.TransferAffQuotaToQuota(800))
	user = loadAffiliateUser(t, legacyUser.Id)
	assert.Equal(t, 0, user.AffQuota)
	assert.Equal(t, 800, user.Quota)

	// 台账总额大于 AffQuota 的异常数据：legacy 取 0，不出现负可划转
	abnormalInviter := createAffiliateUser(t, "aff-legacy-abnormal", 500)
	abnormalInvitee := createAffiliateUser(t, "aff-legacy-abnormal-invitee", 0)
	createAffiliateRecord(t, AffiliateReward{
		InviterId:    abnormalInviter.Id,
		InviteeId:    abnormalInvitee.Id,
		InviterQuota: 900,
		UnlockedAt:   common.GetTimestamp(),
	})
	abnormal, err := GetAffiliateWithdrawable(abnormalInviter.Id)
	require.NoError(t, err)
	assert.Equal(t, 0, abnormal.LegacyRemaining)
	assert.Equal(t, 500, abnormal.Withdrawable, "异常数据下可划转不得超过 AffQuota")
	assert.Equal(t, 0, abnormal.Locked)
}

// ---------------------------------------------------------------------------
// 附：注册路径在默认设置下仍然「注册即发」，且台账只落一条
// ---------------------------------------------------------------------------

func TestAffiliateRegistrationGrantsInviteeRewardImmediatelyByDefault(t *testing.T) {
	setupAffiliateTest(t)

	confirmAffiliateComplianceOn(t)
	common.QuotaForInviter = 0
	common.QuotaForInvitee = 120
	common.QuotaForNewUser = 0

	inviter := createAffiliateUser(t, "aff-reg-inviter", 0)
	invitee := &User{
		Username:  "aff-reg-invitee",
		Password:  "test-password",
		AffCode:   "aff-reg-invitee",
		InviterId: inviter.Id,
	}
	require.NoError(t, invitee.Insert(inviter.Id))

	registered := loadAffiliateUser(t, invitee.Id)
	assert.Equal(t, 120, registered.Quota, "默认设置下注册即发放被邀请人奖励")

	// 即使邀请人奖励为 0，aff_count 与台账也要落
	inviterAfter := loadAffiliateUser(t, inviter.Id)
	assert.Equal(t, 1, inviterAfter.AffCount)
	assert.Equal(t, 0, inviterAfter.AffQuota)

	var records []AffiliateReward
	require.NoError(t, DB.Where("invitee_id = ?", invitee.Id).Find(&records).Error)
	require.Len(t, records, 1)
	assert.True(t, records[0].InviteeRewardGranted)
	assert.Equal(t, 120, records[0].InviteeQuota)

	// 幂等：Settle 不应再加额度
	require.NoError(t, SettleAffiliateRewards(invitee.Id))
	assert.Equal(t, 120, loadAffiliateUser(t, invitee.Id).Quota)
}

func TestAffiliateComputeWithdrawableIgnoresUnknownUser(t *testing.T) {
	setupAffiliateTest(t)

	_, err := GetAffiliateWithdrawable(0)
	require.Error(t, err)

	detail, err := GetAffiliateWithdrawable(987654)
	require.Error(t, err)
	assert.Nil(t, detail)
}
