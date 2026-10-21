package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

// TestPhase4CheckinSettingPersistenceRoundTrip 验证 Phase 4 新增的两个签到配置项
// 能通过通用配置体系（反射 + JSON 序列化）完成「保存到 map → 从 map 恢复」的往返，
// 且默认值为 0（= 功能关闭，升级后行为不变）。
//
// 这是回归保护：新增字段若类型不被 config 包支持，会静默丢失或解析失败，
// 表现为「后台填了门槛但不生效」。
func TestPhase4CheckinSettingPersistenceRoundTrip(t *testing.T) {
	// 1. 默认值必须是 0（功能默认关闭）
	defaults := GetCheckinSetting()
	require.Zero(t, defaults.MinRechargeAmount, "默认充值门槛必须为 0（不限制）")
	require.Zero(t, defaults.InactiveDays, "默认不活跃天数必须为 0（不限制）")

	// 2. 模拟后台保存：ConfigToMap 把结构体转成 option key/value
	exported, err := config.ConfigToMap(defaults)
	require.NoError(t, err)
	require.Equal(t, "0", exported["min_recharge_amount"])
	require.Equal(t, "0", exported["inactive_days"])

	// 3. 模拟从数据库读回非默认值，并验证小数金额（12.5 元）不丢精度
	saved := map[string]string{
		"min_recharge_amount": "12.5",
		"inactive_days":       "30",
	}
	require.NoError(t, config.UpdateConfigFromMap(defaults, saved))
	require.InDelta(t, 12.5, defaults.MinRechargeAmount, 1e-9,
		"小数金额必须保留（int 会丢掉 0.5）")
	require.Equal(t, 30, defaults.InactiveDays)

	// 4. 再次导出，确认往返一致
	reexported, err := config.ConfigToMap(defaults)
	require.NoError(t, err)
	require.Equal(t, "12.5", reexported["min_recharge_amount"])
	require.Equal(t, "30", reexported["inactive_days"])

	// 5. 还原默认值，避免影响同包内其他测试
	require.NoError(t, config.UpdateConfigFromMap(defaults, map[string]string{
		"min_recharge_amount": "0",
		"inactive_days":       "0",
	}))
	require.Zero(t, defaults.MinRechargeAmount)
	require.Zero(t, defaults.InactiveDays)
}
