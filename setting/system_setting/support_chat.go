package system_setting

import "github.com/QuantumNous/new-api/setting/config"

const (
	// SupportChatRetentionDaysOption 在线客服已关闭会话的留存天数配置键。
	SupportChatRetentionDaysOption = "support_chat.retention_days"
	// SupportChatUIScaleOption 在线客服卡片/窗口整体缩放百分比。
	SupportChatUIScaleOption = "support_chat.ui_scale"
	// SupportChatTextScaleOption 在线客服文字缩放百分比。
	SupportChatTextScaleOption = "support_chat.text_scale"
	// SupportChatProminenceOption 在线客服入口动效显眼度百分比。
	SupportChatProminenceOption = "support_chat.prominence"
	// SupportChatCustomTitleOption 客服入口标题自定义文案。
	SupportChatCustomTitleOption = "support_chat.custom_title"
	// SupportChatCustomSubLoginOption 已登录用户看到的客服入口副标题。
	SupportChatCustomSubLoginOption = "support_chat.custom_sub_login"
	// SupportChatCustomSubGuestOption 未登录访客看到的客服入口副标题。
	SupportChatCustomSubGuestOption = "support_chat.custom_sub_guest"
	// SupportChatBottomOffsetOption 客服入口距屏幕底部的距离（像素）。
	SupportChatBottomOffsetOption = "support_chat.bottom_offset"

	// DefaultSupportChatRetentionDays 默认留存 180 天。
	DefaultSupportChatRetentionDays = 180
)

// SupportChatSettings 在线客服配置。
type SupportChatSettings struct {
	// RetentionDays 已关闭会话及附件的留存天数，0 表示永久保留。
	RetentionDays int `json:"retention_days"`
	// UIScale 客服卡片与聊天窗口的整体缩放百分比，100 为标准尺寸。
	UIScale int `json:"ui_scale"`
	// TextScale 客服入口与聊天窗口的文字缩放百分比，100 为标准字号。
	TextScale int `json:"text_scale"`
	// Prominence 入口闪光、水波与光晕的显眼度百分比，0 为关闭动效。
	Prominence int `json:"prominence"`
	// CustomTitle 客服入口标题，留空使用默认“在线客服”。
	CustomTitle string `json:"custom_title"`
	// CustomSubLogin 已登录用户的入口副标题，留空使用默认文案。
	CustomSubLogin string `json:"custom_sub_login"`
	// CustomSubGuest 未登录访客的入口副标题，留空使用默认文案。
	CustomSubGuest string `json:"custom_sub_guest"`
	// BottomOffset 客服入口距屏幕底部的像素距离，避免遮挡分页等控件。
	BottomOffset int `json:"bottom_offset"`
}

var defaultSupportChatSettings = SupportChatSettings{
	RetentionDays: DefaultSupportChatRetentionDays,
	UIScale:       100,
	TextScale:     100,
	Prominence:    100,
	BottomOffset:  96,
}

func init() {
	config.GlobalConfig.Register("support_chat", &defaultSupportChatSettings)
}

// GetSupportChatSettings 返回当前在线客服配置。
func GetSupportChatSettings() SupportChatSettings {
	return defaultSupportChatSettings
}
