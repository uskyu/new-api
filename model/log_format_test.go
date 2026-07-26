package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

// TestFormatUserLogsStripsQuotaSaturation verifies the admin-only quota
// saturation marker (nested under other.admin_info) is removed for non-admin
// log views, since formatUserLogs strips the whole admin_info object.
func TestFormatUserLogsStripsQuotaSaturation(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price": 0.004,
		"admin_info": map[string]interface{}{
			"quota_saturation": map[string]interface{}{
				"op":      "QuotaFromDecimal",
				"kind":    "overflow",
				"clamped": common.MaxQuota,
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	_, hasAdminInfo := parsed["admin_info"]
	require.False(t, hasAdminInfo, "admin_info (and nested quota_saturation) must be stripped for non-admin views")
	// Non-admin billing fields remain visible.
	require.Contains(t, parsed, "model_price")
}

func TestFormatUserRefundLogKeepsDisplayFieldsAndStripsAdminInfo(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"batch_id":      12,
		"source_log_id": 34,
		"source":        "wallet",
		"ratio":         60,
		"reason":        "provider incident",
		"admin_info": map[string]interface{}{
			"operator_id": 99,
		},
	})
	logs := []*Log{{
		Type:        LogTypeRefund,
		Quota:       120,
		ChannelId:   7,
		ChannelName: "private channel name",
		ModelName:   "example-model",
		Other:       other,
	}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.NotContains(t, parsed, "admin_info")
	require.Equal(t, "provider incident", parsed["reason"])
	require.Equal(t, "wallet", parsed["source"])
	require.Equal(t, float64(60), parsed["ratio"])
	require.Equal(t, 120, logs[0].Quota)
	require.Zero(t, logs[0].ChannelId)
	require.Equal(t, "example-model", logs[0].ModelName)
	require.Empty(t, logs[0].ChannelName)
}
