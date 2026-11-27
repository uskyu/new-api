package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTextQuotaBillableTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return ctx
}

// TestClaudeCacheOnlyUsageRemainsBillable covers Claude requests whose ordinary
// prompt/completion counts are zero but which still bill through cache writes
// or cache reads. Before #7816 these were zeroed because TotalTokens == 0.
func TestClaudeCacheOnlyUsageRemainsBillable(t *testing.T) {
	for _, tc := range []struct {
		name  string
		usage dto.Usage
	}{
		{
			name: "one-hour cache write only",
			usage: dto.Usage{
				UsageSemantic: "anthropic",
				PromptTokensDetails: dto.InputTokenDetails{
					CachedCreationTokens: 20000,
				},
				ClaudeCacheCreation1hTokens: 20000,
			},
		},
		{
			name: "cache read only",
			usage: dto.Usage{
				UsageSemantic: "anthropic",
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: 100,
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newTextQuotaBillableTestContext(t)
			relayInfo := &relaycommon.RelayInfo{
				RelayFormat:     types.RelayFormatClaude,
				OriginModelName: "claude-opus-4-8",
				StartTime:       time.Now(),
				PriceData: types.PriceData{
					ModelRatio:           2.5,
					CompletionRatio:      5,
					CacheRatio:           0.1,
					CacheCreationRatio:   1.25,
					CacheCreation5mRatio: 1.25,
					CacheCreation1hRatio: 2,
					GroupRatioInfo:       types.GroupRatioInfo{GroupRatio: 1},
				},
			}
			summary := calculateTextQuotaSummary(ctx, relayInfo, &tc.usage)

			assert.Zero(t, summary.PromptTokens)
			assert.Zero(t, summary.CompletionTokens)
			assert.True(t, summary.IsClaudeUsageSemantic)
			assert.True(t, summary.hasBillableUsage())
			assert.Greater(t, summary.Quota, 0)
			t.Logf("input=%d output=%d cache=%d cache_creation=%d quota=%d",
				summary.PromptTokens, summary.CompletionTokens,
				summary.CacheTokens, summary.CacheCreationTokens, summary.Quota)
		})
	}
}

// TestTextQuotaSummaryBillableUsage covers the negative case: no tokens, no
// cache, no auxiliary billable call means the request must not be charged.
func TestTextQuotaSummaryBillableUsage(t *testing.T) {
	ctx := newTextQuotaBillableTestContext(t)
	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "gpt-test",
		StartTime:       time.Now(),
		PriceData: types.PriceData{
			ModelRatio:      1,
			CompletionRatio: 2,
			CacheRatio:      0.1,
			GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 1},
		},
	}
	summary := calculateTextQuotaSummary(ctx, relayInfo, &dto.Usage{})

	assert.False(t, summary.FixedPriceBilling)
	assert.False(t, summary.hasBillableUsage())
	assert.Zero(t, summary.Quota)
}

// TestFixedPriceZeroTokenUsageRemainsBillable covers per-request pricing:
// even with no token counts, a fixed price must still be billable.
func TestFixedPriceZeroTokenUsageRemainsBillable(t *testing.T) {
	ctx := newTextQuotaBillableTestContext(t)
	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "fixed-price-model",
		StartTime:       time.Now(),
		PriceData: types.PriceData{
			UsePrice:       true,
			ModelPrice:     0.02,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
		},
	}
	summary := calculateTextQuotaSummary(ctx, relayInfo, &dto.Usage{})

	require.True(t, summary.FixedPriceBilling)
	assert.True(t, summary.hasBillableUsage())
	assert.Zero(t, summary.TotalTokens)
}
