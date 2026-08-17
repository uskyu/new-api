package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type QuotaSetting struct {
	EnableFreeModelPreConsume        bool `json:"enable_free_model_pre_consume"`         // 是否对免费模型启用预消耗
	RewardInviterAfterFirstModelCall bool `json:"reward_inviter_after_first_model_call"` // 是否在受邀用户首次模型调用后奖励邀请人
}

// 默认配置
var quotaSetting = QuotaSetting{
	EnableFreeModelPreConsume:        true,
	RewardInviterAfterFirstModelCall: true,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("quota_setting", &quotaSetting)
}

func GetQuotaSetting() *QuotaSetting {
	return &quotaSetting
}
