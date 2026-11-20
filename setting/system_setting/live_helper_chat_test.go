package system_setting

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLiveHelperChatBaseURL(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		allowInsecure bool
		want          string
		wantErr       bool
	}{
		{
			name: "https base",
			raw:  "https://chat.example.com",
			want: "https://chat.example.com",
		},
		{
			name: "https subpath with trailing slash",
			raw:  "https://chat.example.com/index.php/",
			want: "https://chat.example.com/index.php",
		},
		{
			name:          "development http localhost",
			raw:           "http://localhost:8080",
			allowInsecure: true,
			want:          "http://localhost:8080",
		},
		{
			name:          "development http loopback ip",
			raw:           "http://127.0.0.1:8080/lhc/",
			allowInsecure: true,
			want:          "http://127.0.0.1:8080/lhc",
		},
		{
			name:          "development http IPv6 loopback",
			raw:           "http://[::1]:8080",
			allowInsecure: true,
			want:          "http://[::1]:8080",
		},
		{
			name:    "http localhost in production",
			raw:     "http://localhost:8080",
			wantErr: true,
		},
		{
			name:          "http public host in development",
			raw:           "http://chat.example.com",
			allowInsecure: true,
			wantErr:       true,
		},
		{
			name:    "javascript scheme",
			raw:     "javascript:alert(1)",
			wantErr: true,
		},
		{
			name:    "data scheme",
			raw:     "data:text/html,<script>alert(1)</script>",
			wantErr: true,
		},
		{
			name:    "credentials",
			raw:     "https://user:pass@chat.example.com",
			wantErr: true,
		},
		{
			name:    "query parameter",
			raw:     "https://chat.example.com/?widget=1",
			wantErr: true,
		},
		{
			name:    "fragment",
			raw:     "https://chat.example.com/#widget",
			wantErr: true,
		},
		{
			name:    "backslash",
			raw:     `https://chat.example.com\path`,
			wantErr: true,
		},
		{
			name:    "empty host",
			raw:     "https://",
			wantErr: true,
		},
		{
			name: "empty value",
			raw:  " ",
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ValidateLiveHelperChatBaseURL(test.raw, test.allowInsecure)
			if test.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestValidateLiveHelperChatIdentifiers(t *testing.T) {
	t.Run("widget id", func(t *testing.T) {
		got, err := ValidateLiveHelperChatWidgetID("  site-1.example_test  ")
		require.NoError(t, err)
		assert.Equal(t, "site-1.example_test", got)

		_, err = ValidateLiveHelperChatWidgetID(strings.Repeat("a", 51))
		require.Error(t, err)

		_, err = ValidateLiveHelperChatWidgetID("site with space")
		require.Error(t, err)

		got, err = ValidateLiveHelperChatWidgetID("")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("locale", func(t *testing.T) {
		got, err := ValidateLiveHelperChatLocale(" en/ ")
		require.NoError(t, err)
		assert.Equal(t, "en", got)

		got, err = ValidateLiveHelperChatLocale("zh-CN")
		require.NoError(t, err)
		assert.Equal(t, "zh-CN", got)

		_, err = ValidateLiveHelperChatLocale("en/us")
		require.Error(t, err)

		_, err = ValidateLiveHelperChatLocale(strings.Repeat("a", 33))
		require.Error(t, err)

		got, err = ValidateLiveHelperChatLocale("")
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestGetLiveHelperChatPublicConfig(t *testing.T) {
	previous := common.OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		LiveHelperChatEnabledOption:  "true",
		LiveHelperChatBaseURLOption:  "https://chat.example.com/index.php/",
		LiveHelperChatWidgetIDOption: "site-1",
		LiveHelperChatLocaleOption:   "en/",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})

	config := GetLiveHelperChatPublicConfig(false)
	require.True(t, config.Enabled)
	assert.Equal(t, "https://chat.example.com/index.php", config.BaseURL)
	assert.Equal(t, "site-1", config.WidgetID)
	assert.Equal(t, "en", config.Locale)

	common.OptionMapRWMutex.Lock()
	common.OptionMap[LiveHelperChatEnabledOption] = "false"
	common.OptionMapRWMutex.Unlock()
	config = GetLiveHelperChatPublicConfig(false)
	require.False(t, config.Enabled)
	assert.Empty(t, config.BaseURL)

	common.OptionMapRWMutex.Lock()
	common.OptionMap[LiveHelperChatEnabledOption] = "true"
	common.OptionMap[LiveHelperChatBaseURLOption] = "javascript:alert(1)"
	common.OptionMapRWMutex.Unlock()
	config = GetLiveHelperChatPublicConfig(false)
	require.False(t, config.Enabled)
	assert.Empty(t, config.BaseURL)

	common.OptionMapRWMutex.Lock()
	common.OptionMap[LiveHelperChatBaseURLOption] = "http://localhost:8080"
	common.OptionMapRWMutex.Unlock()
	config = GetLiveHelperChatPublicConfig(false)
	require.False(t, config.Enabled)

	config = GetLiveHelperChatPublicConfig(true)
	require.True(t, config.Enabled)
	assert.Equal(t, "http://localhost:8080", config.BaseURL)
}
