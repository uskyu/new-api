package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserBillVisibleDaysUsesConfiguredValueAndSafeFallback(t *testing.T) {
	original := paymentSetting.UserBillVisibleDays
	t.Cleanup(func() {
		paymentSetting.UserBillVisibleDays = original
	})

	paymentSetting.UserBillVisibleDays = 7
	assert.Equal(t, 7, GetUserBillVisibleDays())

	for _, invalid := range []int{0, -1, MaxUserBillVisibleDays + 1} {
		paymentSetting.UserBillVisibleDays = invalid
		assert.Equal(t, DefaultUserBillVisibleDays, GetUserBillVisibleDays())
	}
}

func TestPaymentSettingKeepsBillVisibilityDefaultWhenLoadingLegacyConfig(t *testing.T) {
	legacyCompatibleSetting := PaymentSetting{
		AmountOptions:       []int{10, 20},
		AmountDiscount:      map[int]float64{},
		UserBillVisibleDays: DefaultUserBillVisibleDays,
	}
	manager := config.NewConfigManager()
	manager.Register("payment_setting", &legacyCompatibleSetting)

	require.NoError(t, manager.LoadFromDB(map[string]string{
		"payment_setting.amount_options": "[50,100]",
	}))
	assert.Equal(t, []int{50, 100}, legacyCompatibleSetting.AmountOptions)
	assert.Equal(t, DefaultUserBillVisibleDays, legacyCompatibleSetting.UserBillVisibleDays)
}
