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
	oldInviterQuota := common.QuotaForInviter
	oldInviteeQuota := common.QuotaForInvitee
	paymentSetting := operation_setting.GetPaymentSetting()
	oldComplianceConfirmed := paymentSetting.ComplianceConfirmed
	oldComplianceVersion := paymentSetting.ComplianceTermsVersion
	t.Cleanup(func() {
		common.QuotaForInviter = oldInviterQuota
		common.QuotaForInvitee = oldInviteeQuota
		paymentSetting.ComplianceConfirmed = oldComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = oldComplianceVersion
		DB.Exec("DELETE FROM risk_ip_records")
		DB.Exec("DELETE FROM users")
	})
}

func TestLegacyOAuthInsertKeepsInviteeBonusWithoutRewardingInviter(t *testing.T) {
	setupInviteRewardTest(t)
	common.QuotaForInviter = 900
	common.QuotaForInvitee = 300
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	inviter := User{Username: "legacy-oauth-owner", Password: "test-password", AffCode: "legacy-oauth-owner-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	oauthUser := User{Username: "legacy-oauth-user", InviterId: inviter.Id, Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, oauthUser.Insert(inviter.Id))

	require.NoError(t, DB.First(&oauthUser, oauthUser.Id).Error)
	require.Equal(t, InviteRewardStatusIneligible, oauthUser.InviteRewardStatus)
	require.Equal(t, common.QuotaForNewUser+common.QuotaForInvitee, oauthUser.Quota)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Zero(t, inviter.AffCount)
	require.Zero(t, inviter.AffQuota)
	require.Zero(t, inviter.AffHistoryQuota)
}

func TestOAuthInviteKeepsInviteeBonusWithoutRewardingInviter(t *testing.T) {
	setupInviteRewardTest(t)
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
	require.Equal(t, common.QuotaForNewUser+common.QuotaForInvitee, oauthUser.Quota)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Zero(t, inviter.AffCount)
	require.Zero(t, inviter.AffQuota)
	require.Zero(t, inviter.AffHistoryQuota)

	activatedInviterId, err := ActivatePendingInviteReward(oauthUser.Id, 12000)
	require.NoError(t, err)
	require.Zero(t, activatedInviterId)
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	require.Zero(t, inviter.AffCount)
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
