package system_setting

import "github.com/QuantumNous/new-api/setting/config"

const (
	// SupportChatRetentionDaysOption 在线客服已关闭会话的留存天数配置键。
	SupportChatRetentionDaysOption = "support_chat.retention_days"

	// DefaultSupportChatRetentionDays 默认留存 180 天。
	DefaultSupportChatRetentionDays = 180
)

// SupportChatSettings 在线客服配置。
type SupportChatSettings struct {
	// RetentionDays 已关闭会话及附件的留存天数，0 表示永久保留。
	RetentionDays int `json:"retention_days"`
}

var defaultSupportChatSettings = SupportChatSettings{
	RetentionDays: DefaultSupportChatRetentionDays,
}

func init() {
	config.GlobalConfig.Register("support_chat", &defaultSupportChatSettings)
}

// GetSupportChatSettings 返回当前在线客服配置。
func GetSupportChatSettings() SupportChatSettings {
	return defaultSupportChatSettings
}
