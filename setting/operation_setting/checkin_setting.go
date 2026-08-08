package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// CheckinBonusTier defines one active-reward tier.
type CheckinBonusTier struct {
	Threshold int `json:"threshold"`
	MinQuota  int `json:"min_quota"`
	MaxQuota  int `json:"max_quota"`
}

// CheckinSetting 签到功能配置
type CheckinSetting struct {
	Enabled        bool   `json:"enabled"`         // 是否启用签到功能
	MinQuota       int    `json:"min_quota"`       // 签到最小额度奖励
	MaxQuota       int    `json:"max_quota"`       // 签到最大额度奖励
	CaptchaEnabled bool   `json:"captcha_enabled"` // 是否启用签到图形验证码
	CaptchaKind    string `json:"captcha_kind"`    // math / digit
	BonusEnabled   bool   `json:"bonus_enabled"`   // 是否启用活跃阶梯奖励
	BonusMetric    string `json:"bonus_metric"`    // request_count / quota_consumed

	// 两套独立的活跃奖励阶梯：按昨日调用次数 / 按昨日消耗额度。
	// 切换 BonusMetric 时互不覆盖，各自独立编辑与保存。
	RequestCountTiers  []CheckinBonusTier `json:"request_count_tiers"`  // 按次档位（threshold 为次数）
	QuotaConsumedTiers []CheckinBonusTier `json:"quota_consumed_tiers"` // 按额度档位（threshold 为额度）
}

// 默认配置
var checkinSetting = CheckinSetting{
	Enabled:            false,  // 默认关闭
	MinQuota:           1000,   // 默认最小额度 1000 (约 0.002 USD)
	MaxQuota:           10000,  // 默认最大额度 10000 (约 0.02 USD)
	CaptchaEnabled:     false,  // 默认不开启验证码
	CaptchaKind:        "math", // 默认数学算式
	BonusEnabled:       false,  // 默认不开启活跃奖励
	BonusMetric:        "request_count",
	RequestCountTiers:  []CheckinBonusTier{},
	QuotaConsumedTiers: []CheckinBonusTier{},
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("checkin_setting", &checkinSetting)
}

// GetCheckinSetting 获取签到配置
func GetCheckinSetting() *CheckinSetting {
	return &checkinSetting
}

// ActiveTiers 返回当前生效指标对应的那套档位
func (s *CheckinSetting) ActiveTiers() []CheckinBonusTier {
	if s.BonusMetric == "quota_consumed" {
		return s.QuotaConsumedTiers
	}
	return s.RequestCountTiers
}

// IsCheckinEnabled 是否启用签到功能
func IsCheckinEnabled() bool {
	return checkinSetting.Enabled
}

// GetCheckinQuotaRange 获取签到额度范围
func GetCheckinQuotaRange() (min, max int) {
	return checkinSetting.MinQuota, checkinSetting.MaxQuota
}
