package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetUserBillVisibleDays(t *testing.T) {
	original := paymentSetting.UserBillVisibleDays
	t.Cleanup(func() {
		paymentSetting.UserBillVisibleDays = original
	})

	paymentSetting.UserBillVisibleDays = 7
	require.Equal(t, 7, GetUserBillVisibleDays())

	paymentSetting.UserBillVisibleDays = 0
	require.Equal(t, DefaultUserBillVisibleDays, GetUserBillVisibleDays())

	paymentSetting.UserBillVisibleDays = MaxUserBillVisibleDays + 1
	require.Equal(t, DefaultUserBillVisibleDays, GetUserBillVisibleDays())
}
