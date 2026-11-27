package claude

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newClaudeUsageFinalizeTestContext(t *testing.T, model string) (*gin.Context, *relaycommon.RelayInfo, *ClaudeResponseInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: model},
	}
	info.SetEstimatePromptTokens(1200)
	return c, info, &ClaudeResponseInfo{Usage: &dto.Usage{}}
}

func sendClaudeStreamEvent(t *testing.T, c *gin.Context, info *relaycommon.RelayInfo, state *ClaudeResponseInfo, event string) {
	t.Helper()
	require.Nil(t, HandleStreamResponseData(c, info, state, event))
}

// TestClaudeStreamFinalUsageKeepsCacheOnlyRefusalZeros covers a fully cached
// prompt that refuses before streaming any content: the upstream explicitly
// reports zero ordinary input and output, so finalization must not overwrite
// the counts with estimates, and the cache creation fields must survive.
func TestClaudeStreamFinalUsageKeepsCacheOnlyRefusalZeros(t *testing.T) {
	c, info, state := newClaudeUsageFinalizeTestContext(t, "claude-opus-4-8")

	sendClaudeStreamEvent(t, c, info, state, `{"type":"message_start","message":{"id":"msg_1","model":"claude-opus-4-8","usage":{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":20000,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":20000}}}}`)
	sendClaudeStreamEvent(t, c, info, state, `{"type":"message_delta","delta":{"stop_reason":"refusal"},"usage":{"output_tokens":0}}`)
	sendClaudeStreamEvent(t, c, info, state, `{"type":"message_stop"}`)

	HandleStreamFinalResponse(c, info, state)

	assert.Equal(t, 0, state.Usage.PromptTokens)
	assert.Equal(t, 0, state.Usage.CompletionTokens)
	assert.Equal(t, 0, state.Usage.TotalTokens)
	assert.Equal(t, 20000, state.Usage.PromptTokensDetails.CachedCreationTokens)
	assert.Equal(t, 20000, state.Usage.ClaudeCacheCreation1hTokens)
	assert.Equal(t, "anthropic", state.Usage.UsageSemantic)
}

// TestClaudeStreamFinalUsageKeepsCacheReadOnBrokenStream covers a stream cut
// short before message_delta: the cache read input must be preserved, while
// the missing completion count is estimated and must not lower the reported
// value.
func TestClaudeStreamFinalUsageKeepsCacheReadOnBrokenStream(t *testing.T) {
	c, info, state := newClaudeUsageFinalizeTestContext(t, "claude-opus-4-8")

	sendClaudeStreamEvent(t, c, info, state, `{"type":"message_start","message":{"id":"msg_2","model":"claude-opus-4-8","usage":{"input_tokens":0,"output_tokens":1,"cache_read_input_tokens":50000}}}`)
	sendClaudeStreamEvent(t, c, info, state, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`)

	HandleStreamFinalResponse(c, info, state)

	assert.False(t, state.Done)
	assert.Equal(t, 0, state.Usage.PromptTokens, "cache-only input must not be re-estimated")
	assert.Equal(t, 50000, state.Usage.PromptTokensDetails.CachedTokens)
	assert.Equal(t, 2, state.Usage.CompletionTokens)
	assert.Greater(t, state.Usage.CompletionTokens, 1)
	assert.Equal(t, "anthropic", state.Usage.UsageSemantic)
}

// TestClaudeStreamFinalUsageEstimatesZeroCompletionAfterText covers a normal
// end where the upstream reported zero completion tokens despite streamed
// text: only the missing output is estimated, cache fields are preserved.
func TestClaudeStreamFinalUsageEstimatesZeroCompletionAfterText(t *testing.T) {
	c, info, state := newClaudeUsageFinalizeTestContext(t, "claude-opus-4-8")

	sendClaudeStreamEvent(t, c, info, state, `{"type":"message_start","message":{"id":"msg_3","model":"claude-opus-4-8","usage":{"input_tokens":1000,"output_tokens":0,"cache_read_input_tokens":30,"cache_creation_input_tokens":50}}}`)
	sendClaudeStreamEvent(t, c, info, state, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`)
	sendClaudeStreamEvent(t, c, info, state, `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":0}}`)
	sendClaudeStreamEvent(t, c, info, state, `{"type":"message_stop"}`)

	HandleStreamFinalResponse(c, info, state)

	assert.True(t, state.Done)
	assert.Equal(t, 1000, state.Usage.PromptTokens)
	assert.Equal(t, 2, state.Usage.CompletionTokens)
	assert.Equal(t, 1002, state.Usage.TotalTokens)
	assert.Equal(t, 30, state.Usage.PromptTokensDetails.CachedTokens)
	assert.Equal(t, 50, state.Usage.PromptTokensDetails.CachedCreationTokens)
	assert.Equal(t, "anthropic", state.Usage.UsageSemantic)
}

// TestClaudeStreamFinalUsageDeltaZeroInputKeepsStartInput covers the
// message_delta usage patch: a zero input_tokens in message_delta must not
// erase the input counted at message_start.
func TestClaudeStreamFinalUsageDeltaZeroInputKeepsStartInput(t *testing.T) {
	c, info, state := newClaudeUsageFinalizeTestContext(t, "claude-opus-4-8")

	sendClaudeStreamEvent(t, c, info, state, `{"type":"message_start","message":{"id":"msg_4","model":"claude-opus-4-8","usage":{"input_tokens":1000,"output_tokens":1}}}`)
	sendClaudeStreamEvent(t, c, info, state, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`)
	sendClaudeStreamEvent(t, c, info, state, `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":0,"output_tokens":50}}`)
	sendClaudeStreamEvent(t, c, info, state, `{"type":"message_stop"}`)

	HandleStreamFinalResponse(c, info, state)

	assert.True(t, state.Done)
	assert.Equal(t, 1000, state.Usage.PromptTokens)
	assert.Equal(t, 50, state.Usage.CompletionTokens)
	assert.Equal(t, 1050, state.Usage.TotalTokens)
	assert.Equal(t, "anthropic", state.Usage.UsageSemantic)
}
