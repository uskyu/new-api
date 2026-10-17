package ratio_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupTaxRateDefaultsToZeroAndSupportsUpdates(t *testing.T) {
	original := GroupTaxRate2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupTaxRateByJSONString(original))
	})

	require.NoError(t, UpdateGroupTaxRateByJSONString(`{"vip":8.5}`))
	assert.Equal(t, 8.5, GetGroupTaxRate("vip"))
	assert.Equal(t, 0.0, GetGroupTaxRate("default"))
	assert.Equal(t, map[string]float64{"vip": 8.5}, GetGroupTaxRateCopy())
	assert.Equal(t, `{"vip":8.5}`, GroupTaxRate2JSONString())
}

func TestCheckGroupTaxRateRejectsInvalidRates(t *testing.T) {
	original := GroupTaxRate2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupTaxRateByJSONString(original))
	})
	require.NoError(t, UpdateGroupTaxRateByJSONString(`{"vip":8.5}`))

	for _, value := range []string{`{"vip":-0.1}`, `{"vip":100.1}`, `[]`, `not-json`} {
		assert.Error(t, CheckGroupTaxRate(value), value)
		assert.Error(t, UpdateGroupTaxRateByJSONString(value), value)
	}
	assert.NoError(t, CheckGroupTaxRate(`{"vip":0,"svip":100}`))
	assert.Error(t, UpdateGroupTaxRateByJSONString(`{"vip":101}`))
	assert.Equal(t, map[string]float64{"vip": 8.5}, GetGroupTaxRateCopy())
}

func TestFormatMatchingModelNameDoesNotStripBase(t *testing.T) {
	assert.Equal(t, "qwen3-max@thinking:on", FormatMatchingModelName("qwen3-max@thinking:on"))
	assert.Equal(t, "claude-3-7-sonnet-thinking", FormatMatchingModelName("claude-3-7-sonnet-thinking"))
	assert.Equal(t, "gemini-2.5-flash-thinking-*", FormatMatchingModelName("gemini-2.5-flash-thinking-8192"))
	assert.Equal(t, "gpt-4-gizmo-*", FormatMatchingModelName("gpt-4-gizmo-abc"))
}

func TestRoutingMatchModelNameStripsThenWildcards(t *testing.T) {
	assert.Equal(t, "qwen3-max", RoutingMatchModelName("qwen3-max@thinking:on@temperature:0.2"))
	assert.Equal(t, "claude-3-7-sonnet", RoutingMatchModelName("claude-3-7-sonnet-thinking"))
	assert.Equal(t, "gemini-2.5-flash-thinking-*", RoutingMatchModelName("gemini-2.5-flash-thinking-8192"))
	assert.Equal(t, "gpt-5.1-codex-max", RoutingMatchModelName("gpt-5.1-codex-max"))

	geminiSettings := model_setting.GetGeminiSettings()
	old := geminiSettings.ThinkingAdapterEnabled
	geminiSettings.ThinkingAdapterEnabled = true
	t.Cleanup(func() { geminiSettings.ThinkingAdapterEnabled = old })
	assert.Equal(t, "gemini-2.5-flash", RoutingMatchModelName("gemini-2.5-flash-thinking-8192"))
}

func TestRoutingMatchModelNamePreservesExemptAtName(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	original := append([]string(nil), settings.ThinkingModelBlacklist...)
	t.Cleanup(func() { settings.ThinkingModelBlacklist = original })
	settings.ThinkingModelBlacklist = append(original, "re:.*@sha256:.*")

	assert.Equal(t, "opaque@sha256:deadbeef", RoutingMatchModelName("opaque@sha256:deadbeef"))
	assert.Equal(t, "kimi-k2-thinking", RoutingMatchModelName("kimi-k2-thinking"))
}
