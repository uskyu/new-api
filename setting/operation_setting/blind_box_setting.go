package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// BlindBoxSetting 盲盒活动全局配置。
// CycleId 用于限购周期统计：每次活动从关闭切换到开启时生成新的毫秒时间戳。
type BlindBoxSetting struct {
	Enabled             bool  `json:"enabled"`
	CycleId             int64 `json:"cycle_id"`
	OrderTimeoutMinutes int   `json:"order_timeout_minutes"`
}

var blindBoxSetting = BlindBoxSetting{
	Enabled:             false,
	CycleId:             0,
	OrderTimeoutMinutes: 15,
}

func init() {
	config.GlobalConfig.Register("blind_box_setting", &blindBoxSetting)
}

// GetBlindBoxSetting 获取盲盒活动配置。
func GetBlindBoxSetting() *BlindBoxSetting {
	return &blindBoxSetting
}
