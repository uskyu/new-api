package model

import (
	"errors"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInviteRewardTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&RiskIPRecord{}))
	require.NoError(t, DB.Exec("DELETE FROM risk_ip_records").Error)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
	oldNewUserQuota := common.QuotaForNewUser
	oldInviterQuota := common.QuotaForInviter
	oldInviteeQuota := common.QuotaForInvitee
	paymentSetting := operation_setting.GetPaymentSetting()
	oldComplianceConfirmed := paymentSetting.ComplianceConfirmed
	oldComplianceVersion := paymentSetting.ComplianceTermsVersion
	quotaSetting := operation_setting.GetQuotaSetting()
	oldRewardAfterFirstCall := quotaSetting.RewardInviterAfterFirstModelCall
	quotaSetting.RewardInviterAfterFirstModelCall = true
	t.Cleanup(func() {
		common.QuotaForNewUser = oldNewUserQuota
		common.QuotaForInviter = oldInviterQuota
		common.QuotaForInvitee = oldInviteeQuota
		paymentSetting.ComplianceConfirmed = oldComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = oldComplianceVersion
		quotaSetting.RewardInviterAfterFirstModelCall = oldRewardAfterFirstCall
		DB.Exec("DELETE FROM risk_ip_records")
		DB.Exec("DELETE FROM users")
	})
}

func TestLegacyOAuthInsertKeepsInviteeBonusWithoutRewardingInviter(t *testing.T) {
	setupInviteRewardTest(t)
	operation_setting.GetQuotaSetting().RewardInviterAfterFirstModelCall = false
	common.QuotaForInviter = 900
	common.QuotaForInvitee = 300
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	inviter := User{Username: "legacy-oauth-owner", Password: "test-password", AffCode: "legacy-oauth-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	oauthUser := User{
		Username:           "legacy-oauth-user",
		InviterId:          inviter.Id,
		Role:               common.RoleCommonUser,
		Status:             common.UserStatusEnabled,
		InviteRewardStatus: InviteRewardStatusPending,
		FirstModelCallAt:   9999,
	}
	require.NoError(t, oauthUser.Insert(inviter.Id))

	require.NoError(t, DB.First(&oauthUser, oauthUser.Id).Error)
	require.Equal(t, InviteRewardStatusIneligible, oauthUser.InviteRewardStatus)
	require.Zero(t, oauthUser.FirstModelCallAt)
	require.Equal(t, common.QuotaForNewUser+common.QuotaForInvitee, oauthUser.Quota)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Zero(t, inviter.AffCount)
	require.Zero(t, inviter.AffQuota)
	require.Zero(t, inviter.AffHistoryQuota)
}

func TestOAuthInviteKeepsInviteeBonusWithoutRewardingInviter(t *testing.T) {
	setupInviteRewardTest(t)
	operation_setting.GetQuotaSetting().RewardInviterAfterFirstModelCall = false
	common.QuotaForInviter = 900
	common.QuotaForInvitee = 300
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	inviter := User{Username: "oauth-invite-owner", Password: "test-password", AffCode: "oauth-invite-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	oauthUser := User{
		Username:  "oauth-invite-user",
		InviterId: inviter.Id,
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
	}
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return oauthUser.InsertWithTx(tx, inviter.Id)
	}))
	oauthUser.FinalizeOAuthUserCreation(inviter.Id)

	require.NoError(t, DB.First(&oauthUser, oauthUser.Id).Error)
	require.Equal(t, InviteRewardStatusIneligible, oauthUser.InviteRewardStatus)
	require.Zero(t, oauthUser.FirstModelCallAt)
	require.Equal(t, common.QuotaForNewUser+common.QuotaForInvitee, oauthUser.Quota)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Zero(t, inviter.AffCount)
	require.Zero(t, inviter.AffQuota)
	require.Zero(t, inviter.AffHistoryQuota)

	activatedInviterId, err := ActivatePendingInviteReward(oauthUser.Id, 12000)
	require.NoError(t, err)
	require.Zero(t, activatedInviterId)
	require.NoError(t, DB.First(&oauthUser, oauthUser.Id).Error)
	require.Zero(t, oauthUser.FirstModelCallAt)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Zero(t, inviter.AffCount)
}

func TestRegularInviteRegistrationDefersRewardByDefault(t *testing.T) {
	setupInviteRewardTest(t)
	common.QuotaForInviter = 700
	common.QuotaForInvitee = 200
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	require.True(t, operation_setting.GetQuotaSetting().RewardInviterAfterFirstModelCall)

	inviter := User{Username: "regular-default-owner", Password: "test-password", AffCode: "regular-default-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{Username: "regular-default-user", Password: "test-password", InviterId: inviter.Id}
	require.NoError(t, invitee.InsertRegular(inviter.Id))

	require.Equal(t, InviteRewardStatusPending, invitee.InviteRewardStatus)
	require.Equal(t, common.QuotaForNewUser+common.QuotaForInvitee, invitee.Quota)
	require.Zero(t, invitee.FirstModelCallAt)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Zero(t, inviter.AffCount)
}

func TestRegularInviteRegistrationRewardsImmediatelyWhenDisabled(t *testing.T) {
	setupInviteRewardTest(t)
	common.QuotaForInviter = 600
	common.QuotaForInvitee = 250
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	operation_setting.GetQuotaSetting().RewardInviterAfterFirstModelCall = false

	inviter := User{Username: "regular-immediate-owner", Password: "test-password", AffCode: "regular-immediate-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{Username: "regular-immediate-user", Password: "test-password", InviterId: inviter.Id}
	require.NoError(t, invitee.InsertRegular(inviter.Id))

	require.Equal(t, InviteRewardStatusRewarded, invitee.InviteRewardStatus)
	require.Zero(t, invitee.FirstModelCallAt)
	require.Equal(t, common.QuotaForNewUser+common.QuotaForInvitee, invitee.Quota)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Equal(t, 1, inviter.AffCount)
	require.Equal(t, 600, inviter.AffQuota)
	require.Equal(t, 600, inviter.AffHistoryQuota)

	activatedInviterId, err := ActivatePendingInviteReward(invitee.Id, 11000)
	require.NoError(t, err)
	require.Zero(t, activatedInviterId)
	require.NoError(t, DB.First(&invitee, invitee.Id).Error)
	require.EqualValues(t, 11000, invitee.FirstModelCallAt)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Equal(t, 1, inviter.AffCount)
}

func TestInviteRewardSwitchOnlyAffectsRegistration(t *testing.T) {
	setupInviteRewardTest(t)
	common.QuotaForInviter = 400
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	quotaSetting := operation_setting.GetQuotaSetting()
	quotaSetting.RewardInviterAfterFirstModelCall = true

	inviter := User{Username: "regular-switch-owner", Password: "test-password", AffCode: "regular-switch-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	pending := User{Username: "regular-switch-pending", Password: "test-password", InviterId: inviter.Id}
	require.NoError(t, pending.InsertRegular(inviter.Id))
	quotaSetting.RewardInviterAfterFirstModelCall = false

	activatedInviterId, err := ActivatePendingInviteReward(pending.Id, 12000)
	require.NoError(t, err)
	require.Equal(t, inviter.Id, activatedInviterId)
	require.NoError(t, DB.First(&pending, pending.Id).Error)
	require.Equal(t, InviteRewardStatusRewarded, pending.InviteRewardStatus)
	require.EqualValues(t, 12000, pending.FirstModelCallAt)
}

func TestRegularInviteRegistrationCountsZeroRewardQuota(t *testing.T) {
	setupInviteRewardTest(t)
	common.QuotaForInviter = 0
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	operation_setting.GetQuotaSetting().RewardInviterAfterFirstModelCall = false

	inviter := User{Username: "regular-zero-owner", Password: "test-password", AffCode: "regular-zero-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{Username: "regular-zero-user", Password: "test-password", InviterId: inviter.Id}
	require.NoError(t, invitee.InsertRegular(inviter.Id))
	require.Equal(t, InviteRewardStatusRewarded, invitee.InviteRewardStatus)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Equal(t, 1, inviter.AffCount)
	require.Zero(t, inviter.AffQuota)
	require.Zero(t, inviter.AffHistoryQuota)
}

func TestInviteRegistrationSnapshotAndRewardHelperUseCapturedQuotas(t *testing.T) {
	setupInviteRewardTest(t)
	common.QuotaForNewUser = 100
	common.QuotaForInviter = 200
	common.QuotaForInvitee = 300
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	operation_setting.GetQuotaSetting().RewardInviterAfterFirstModelCall = false

	snapshot := captureInviteRegistrationSnapshot()
	common.QuotaForNewUser = 1000
	common.QuotaForInviter = 2000
	common.QuotaForInvitee = 3000
	operation_setting.GetQuotaSetting().RewardInviterAfterFirstModelCall = true

	require.Equal(t, 100, snapshot.NewUserQuota)
	require.Equal(t, 200, snapshot.InviterRewardQuota)
	require.Equal(t, 300, snapshot.InviteeRewardQuota)
	require.True(t, snapshot.ComplianceConfirmed)
	require.False(t, snapshot.RewardAfterFirstCall)

	inviter := User{Username: "snapshot-owner", Password: "test-password", AffCode: "snapshot-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		rewarded, err := rewardInviterWithTx(tx, inviter.Id, snapshot.InviterRewardQuota)
		require.True(t, rewarded)
		return err
	}))
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Equal(t, 1, inviter.AffCount)
	require.Equal(t, 200, inviter.AffQuota)
	require.Equal(t, 200, inviter.AffHistoryQuota)
}

func TestImmediateRewardFailureRemovesInviteeQuotaAndInvalidates(t *testing.T) {
	setupInviteRewardTest(t)
	snapshot := inviteRegistrationSnapshot{
		NewUserQuota:         100,
		InviterRewardQuota:   200,
		InviteeRewardQuota:   300,
		ComplianceConfirmed:  true,
		RewardAfterFirstCall: false,
	}
	invitee := User{
		Username:           "failed-immediate-user",
		Password:           "test-password",
		AffCode:            "failed-immediate-user-aff",
		InviterId:          999999,
		Quota:              snapshot.NewUserQuota + snapshot.InviteeRewardQuota,
		InviteRewardStatus: InviteRewardStatusRewarded,
	}
	require.NoError(t, DB.Create(&invitee).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return rewardInviterForRegistrationWithTx(tx, &invitee, invitee.InviterId, snapshot)
	}))

	require.NoError(t, DB.First(&invitee, invitee.Id).Error)
	require.Equal(t, InviteRewardStatusInvalid, invitee.InviteRewardStatus)
	require.Equal(t, snapshot.NewUserQuota, invitee.Quota)
	require.Zero(t, invitee.FirstModelCallAt)
}

func TestRegularInviteRegistrationKeepsComplianceGate(t *testing.T) {
	setupInviteRewardTest(t)
	common.QuotaForInviter = 500
	common.QuotaForInvitee = 200
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = false
	operation_setting.GetQuotaSetting().RewardInviterAfterFirstModelCall = false

	inviter := User{Username: "regular-gated-owner", Password: "test-password", AffCode: "regular-gated-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{Username: "regular-gated-user", Password: "test-password", InviterId: inviter.Id}
	require.NoError(t, invitee.InsertRegular(inviter.Id))
	require.Equal(t, InviteRewardStatusIneligible, invitee.InviteRewardStatus)
	require.Equal(t, common.QuotaForNewUser, invitee.Quota)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Zero(t, inviter.AffCount)
}

func TestRegularInviteRegistrationInvalidatesMissingOrDeletedInviter(t *testing.T) {
	setupInviteRewardTest(t)
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	operation_setting.GetQuotaSetting().RewardInviterAfterFirstModelCall = false

	missingInvitee := User{Username: "regular-missing-user", Password: "test-password", InviterId: 999999}
	require.NoError(t, missingInvitee.InsertRegular(999999))
	require.Equal(t, InviteRewardStatusInvalid, missingInvitee.InviteRewardStatus)

	inviter := User{Username: "regular-deleted-owner", Password: "test-password", AffCode: "regular-deleted-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	require.NoError(t, DB.Delete(&inviter).Error)
	deletedInviterId, err := GetUserIdByAffCodeIncludingDeleted(inviter.AffCode)
	require.NoError(t, err)
	require.Equal(t, inviter.Id, deletedInviterId)
	deletedInvitee := User{Username: "regular-deleted-user", Password: "test-password", InviterId: deletedInviterId}
	require.NoError(t, deletedInvitee.InsertRegular(inviter.Id))
	require.Equal(t, InviteRewardStatusInvalid, deletedInvitee.InviteRewardStatus)
}

func TestActivatePendingInviteReward(t *testing.T) {
	setupInviteRewardTest(t)
	common.QuotaForInviter = 1200

	inviter := User{Username: "invite-reward-owner", Password: "test-password", AffCode: "invite-reward-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{
		Username:           "invite-reward-user",
		Password:           "test-password",
		AffCode:            "invite-reward-user-aff",
		InviterId:          inviter.Id,
		InviteRewardStatus: InviteRewardStatusPending,
	}
	require.NoError(t, DB.Create(&invitee).Error)

	activatedInviterId, err := ActivatePendingInviteReward(invitee.Id, 12345)
	require.NoError(t, err)
	require.Equal(t, inviter.Id, activatedInviterId)

	require.NoError(t, DB.First(&invitee, invitee.Id).Error)
	require.Equal(t, InviteRewardStatusRewarded, invitee.InviteRewardStatus)
	require.EqualValues(t, 12345, invitee.FirstModelCallAt)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Equal(t, 1, inviter.AffCount)
	require.Equal(t, 1200, inviter.AffQuota)
	require.Equal(t, 1200, inviter.AffHistoryQuota)

	activatedInviterId, err = ActivatePendingInviteReward(invitee.Id, 54321)
	require.NoError(t, err)
	require.Zero(t, activatedInviterId)
	require.NoError(t, DB.First(&invitee, invitee.Id).Error)
	require.EqualValues(t, 12345, invitee.FirstModelCallAt)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Equal(t, 1, inviter.AffCount)
	require.Equal(t, 1200, inviter.AffQuota)
}

func TestActivatePendingInviteRewardIgnoresHistoricalInvite(t *testing.T) {
	setupInviteRewardTest(t)
	common.QuotaForInviter = 800

	inviter := User{Username: "invite-history-owner", Password: "test-password", AffCode: "invite-history-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{
		Username:  "invite-history-user",
		Password:  "test-password",
		AffCode:   "invite-history-user-aff",
		InviterId: inviter.Id,
		// The database default is intentionally ineligible for pre-upgrade and OAuth users.
	}
	require.NoError(t, DB.Create(&invitee).Error)

	activatedInviterId, err := ActivatePendingInviteReward(invitee.Id, 12345)
	require.NoError(t, err)
	require.Zero(t, activatedInviterId)
	require.NoError(t, DB.First(&invitee, invitee.Id).Error)
	require.Equal(t, InviteRewardStatusIneligible, invitee.InviteRewardStatus)
	require.Zero(t, invitee.FirstModelCallAt)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Zero(t, inviter.AffCount)
	require.Zero(t, inviter.AffQuota)
}

func TestActivatePendingInviteRewardInvalidatesMissingInviter(t *testing.T) {
	setupInviteRewardTest(t)
	invitee := User{
		Username:           "invite-missing-owner-user",
		Password:           "test-password",
		AffCode:            "invite-missing-owner-user-aff",
		InviterId:          999999,
		InviteRewardStatus: InviteRewardStatusPending,
	}
	require.NoError(t, DB.Create(&invitee).Error)

	activatedInviterId, err := ActivatePendingInviteReward(invitee.Id, 16000)
	require.NoError(t, err)
	require.Zero(t, activatedInviterId)
	require.NoError(t, DB.First(&invitee, invitee.Id).Error)
	require.Equal(t, InviteRewardStatusInvalid, invitee.InviteRewardStatus)
	require.EqualValues(t, 16000, invitee.FirstModelCallAt)

	activatedInviterId, err = ActivatePendingInviteReward(invitee.Id, 17000)
	require.NoError(t, err)
	require.Zero(t, activatedInviterId)
	require.NoError(t, DB.First(&invitee, invitee.Id).Error)
	require.EqualValues(t, 16000, invitee.FirstModelCallAt)
}

func TestActivatePendingInviteRewardInvalidatesSoftDeletedInviter(t *testing.T) {
	setupInviteRewardTest(t)
	inviter := User{Username: "invite-deleted-owner", Password: "test-password", AffCode: "invite-deleted-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{
		Username:           "invite-deleted-owner-user",
		Password:           "test-password",
		AffCode:            "invite-deleted-owner-user-aff",
		InviterId:          inviter.Id,
		InviteRewardStatus: InviteRewardStatusPending,
	}
	require.NoError(t, DB.Create(&invitee).Error)
	require.NoError(t, DB.Delete(&inviter).Error)

	activatedInviterId, err := ActivatePendingInviteReward(invitee.Id, 18000)
	require.NoError(t, err)
	require.Zero(t, activatedInviterId)
	require.NoError(t, DB.First(&invitee, invitee.Id).Error)
	require.Equal(t, InviteRewardStatusInvalid, invitee.InviteRewardStatus)
	require.EqualValues(t, 18000, invitee.FirstModelCallAt)
}

func TestActivatePendingInviteRewardCountsSuccessWhenRewardQuotaIsZero(t *testing.T) {
	setupInviteRewardTest(t)
	common.QuotaForInviter = 0

	inviter := User{Username: "invite-zero-owner", Password: "test-password", AffCode: "invite-zero-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{
		Username:           "invite-zero-user",
		Password:           "test-password",
		AffCode:            "invite-zero-user-aff",
		InviterId:          inviter.Id,
		InviteRewardStatus: InviteRewardStatusPending,
	}
	require.NoError(t, DB.Create(&invitee).Error)

	activatedInviterId, err := ActivatePendingInviteReward(invitee.Id, 15000)
	require.NoError(t, err)
	require.Equal(t, inviter.Id, activatedInviterId)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Equal(t, 1, inviter.AffCount)
	require.Zero(t, inviter.AffQuota)
	require.Zero(t, inviter.AffHistoryQuota)
}

func TestRiskInviterIncludesInviteActivationTimes(t *testing.T) {
	setupInviteRewardTest(t)
	inviter := User{Username: "invite-risk-owner", Password: "test-password", AffCode: "invite-risk-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{
		Username:           "invite-risk-user",
		Password:           "test-password",
		AffCode:            "invite-risk-user-aff",
		InviterId:          inviter.Id,
		CreatedAt:          14000,
		FirstModelCallAt:   15000,
		InviteRewardStatus: InviteRewardStatusRewarded,
	}
	require.NoError(t, DB.Create(&invitee).Error)

	items, total, err := ListRiskInviters("invite-risk-owner", "all", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Len(t, items[0].Invitees, 1)
	require.EqualValues(t, 14000, items[0].Invitees[0].CreatedAt)
	require.EqualValues(t, 15000, items[0].Invitees[0].FirstModelCallAt)
	require.Equal(t, InviteRewardStatusRewarded, items[0].Invitees[0].InviteRewardStatus)
}

func TestIsRetryableInviteRewardError(t *testing.T) {
	require.True(t, isRetryableInviteRewardError(errors.New("database is locked")))
	require.True(t, isRetryableInviteRewardError(errors.New("SQLITE_BUSY: database is locked")))
	require.True(t, isRetryableInviteRewardError(errors.New("deadlock detected")))
	require.True(t, isRetryableInviteRewardError(errors.New("could not serialize access due to concurrent update")))
	require.False(t, isRetryableInviteRewardError(errors.New("unique constraint failed")))
}

func TestActivateAlreadyRewardedInviteRecordsFirstCallConcurrentlyWithoutReward(t *testing.T) {
	setupInviteRewardTest(t)
	common.QuotaForInviter = 500
	inviter := User{Username: "invite-rewarded-concurrent-owner", Password: "test-password", AffCode: "invite-rewarded-concurrent-owner-aff", AffCount: 1, AffQuota: 500, AffHistoryQuota: 500}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{
		Username:           "invite-rewarded-concurrent-user",
		Password:           "test-password",
		AffCode:            "invite-rewarded-concurrent-user-aff",
		InviterId:          inviter.Id,
		InviteRewardStatus: InviteRewardStatusRewarded,
	}
	require.NoError(t, DB.Create(&invitee).Error)

	const workers = 12
	start := make(chan struct{})
	errs := make(chan error, workers)
	results := make(chan int, workers)
	var wg sync.WaitGroup
	for index := 0; index < workers; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			inviterId, err := ActivatePendingInviteReward(invitee.Id, 21000)
			results <- inviterId
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	close(results)
	for err := range errs {
		require.NoError(t, err)
	}
	for inviterId := range results {
		require.Zero(t, inviterId)
	}

	require.NoError(t, DB.First(&invitee, invitee.Id).Error)
	require.EqualValues(t, 21000, invitee.FirstModelCallAt)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Equal(t, 1, inviter.AffCount)
	require.Equal(t, 500, inviter.AffQuota)
	require.Equal(t, 500, inviter.AffHistoryQuota)
}

func TestActivatePendingInviteRewardConcurrent(t *testing.T) {
	setupInviteRewardTest(t)
	common.QuotaForInviter = 500

	inviter := User{Username: "invite-concurrent-owner", Password: "test-password", AffCode: "invite-concurrent-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{
		Username:           "invite-concurrent-user",
		Password:           "test-password",
		AffCode:            "invite-concurrent-user-aff",
		InviterId:          inviter.Id,
		InviteRewardStatus: InviteRewardStatusPending,
	}
	require.NoError(t, DB.Create(&invitee).Error)

	const workers = 12
	start := make(chan struct{})
	results := make(chan int, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for index := 0; index < workers; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			inviterId, err := ActivatePendingInviteReward(invitee.Id, 20000)
			results <- inviterId
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	activatedCount := 0
	for inviterId := range results {
		if inviterId != 0 {
			require.Equal(t, inviter.Id, inviterId)
			activatedCount++
		}
	}
	require.Equal(t, 1, activatedCount)

	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Equal(t, 1, inviter.AffCount)
	require.Equal(t, 500, inviter.AffQuota)
	require.Equal(t, 500, inviter.AffHistoryQuota)
}
