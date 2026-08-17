package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestQuotaSettingDefaults(t *testing.T) {
	setting := QuotaSetting{
		EnableFreeModelPreConsume:        true,
		RewardInviterAfterFirstModelCall: true,
	}
	data, err := common.Marshal(setting)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"enable_free_model_pre_consume": true,
		"reward_inviter_after_first_model_call": true
	}`, string(data))
	require.True(t, GetQuotaSetting().RewardInviterAfterFirstModelCall)
}
