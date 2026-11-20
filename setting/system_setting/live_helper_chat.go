package system_setting

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	LiveHelperChatEnabledOption  = "LiveHelperChatEnabled"
	LiveHelperChatBaseURLOption  = "LiveHelperChatBaseUrl"
	LiveHelperChatWidgetIDOption = "LiveHelperChatWidgetId"
	LiveHelperChatLocaleOption   = "LiveHelperChatLocale"

	liveHelperChatWidgetIDMaxLength = 50
	liveHelperChatLocaleMaxLength   = 32
)

var (
	liveHelperChatWidgetIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	liveHelperChatLocalePattern   = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	liveHelperChatLocalHosts      = map[string]struct{}{
		"localhost": {},
		"127.0.0.1": {},
		"::1":       {},
	}
)

type LiveHelperChatSettings struct {
	Enabled  bool
	BaseURL  string
	WidgetID string
	Locale   string
}

type LiveHelperChatPublicConfig struct {
	Enabled  bool   `json:"enabled"`
	BaseURL  string `json:"base_url,omitempty"`
	WidgetID string `json:"widget_id,omitempty"`
	Locale   string `json:"locale,omitempty"`
}

func GetLiveHelperChatSettings() LiveHelperChatSettings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	enabled, _ := strconv.ParseBool(strings.TrimSpace(common.Interface2String(common.OptionMap[LiveHelperChatEnabledOption])))
	return LiveHelperChatSettings{
		Enabled:  enabled,
		BaseURL:  strings.TrimSpace(common.Interface2String(common.OptionMap[LiveHelperChatBaseURLOption])),
		WidgetID: strings.TrimSpace(common.Interface2String(common.OptionMap[LiveHelperChatWidgetIDOption])),
		Locale:   strings.TrimSpace(common.Interface2String(common.OptionMap[LiveHelperChatLocaleOption])),
	}
}

// GetLiveHelperChatPublicConfig returns the values that are safe to expose to
// browsers. A missing, disabled, or invalid configuration is always reported as
// disabled so the widget is never loaded with unvalidated values.
func GetLiveHelperChatPublicConfig(allowInsecureLocal bool) LiveHelperChatPublicConfig {
	settings := GetLiveHelperChatSettings()
	if !settings.Enabled {
		return LiveHelperChatPublicConfig{Enabled: false}
	}

	baseURL, err := ValidateLiveHelperChatBaseURL(settings.BaseURL, allowInsecureLocal)
	if err != nil || baseURL == "" {
		return LiveHelperChatPublicConfig{Enabled: false}
	}
	settings.BaseURL = baseURL

	settings.WidgetID, err = ValidateLiveHelperChatWidgetID(settings.WidgetID)
	if err != nil {
		return LiveHelperChatPublicConfig{Enabled: false}
	}

	settings.Locale, err = ValidateLiveHelperChatLocale(settings.Locale)
	if err != nil {
		return LiveHelperChatPublicConfig{Enabled: false}
	}

	return LiveHelperChatPublicConfig{
		Enabled:  true,
		BaseURL:  settings.BaseURL,
		WidgetID: settings.WidgetID,
		Locale:   settings.Locale,
	}
}

// ValidateLiveHelperChatBaseURL accepts an empty value (used to clear the
// setting), but never returns a URL with a non-HTTP(S) scheme, credentials,
// query parameters, or fragments.
func ValidateLiveHelperChatBaseURL(raw string, allowInsecureLocal bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.ContainsAny(raw, " \t\r\n\\") {
		return "", errors.New("LiveHelperChat 地址不能包含空白字符或反斜杠")
	}
	if strings.IndexFunc(raw, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return "", errors.New("LiveHelperChat 地址不能包含控制字符")
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Opaque != "" || parsed.Host == "" {
		return "", errors.New("LiveHelperChat 地址格式不正确")
	}
	if parsed.User != nil {
		return "", errors.New("LiveHelperChat 地址不能包含用户名或密码")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("LiveHelperChat 地址不能包含查询参数或片段")
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	switch parsed.Scheme {
	case "https":
	case "http":
		if !allowInsecureLocal {
			return "", errors.New("生产环境要求 LiveHelperChat 地址使用 HTTPS")
		}
		if _, ok := liveHelperChatLocalHosts[strings.ToLower(parsed.Hostname())]; !ok {
			return "", errors.New("开发环境仅允许 localhost 或 127.0.0.1 使用 HTTP")
		}
	default:
		return "", errors.New("LiveHelperChat 地址仅支持 http 或 https")
	}

	return strings.TrimRight(parsed.String(), "/"), nil
}

func ValidateLiveHelperChatWidgetID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > liveHelperChatWidgetIDMaxLength || !liveHelperChatWidgetIDPattern.MatchString(raw) {
		return "", errors.New("Widget ID 只能包含字母、数字、点、下划线和短横线，且不能超过 50 个字符")
	}
	return raw, nil
}

func ValidateLiveHelperChatLocale(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", nil
	}
	if len(raw) > liveHelperChatLocaleMaxLength || !liveHelperChatLocalePattern.MatchString(raw) {
		return "", errors.New("语言代码只能包含字母、数字、下划线和短横线，且不能超过 32 个字符")
	}
	return raw, nil
}
