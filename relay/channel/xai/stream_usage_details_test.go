package xai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestXAIStreamHandlerPreservesUsageTokenDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	streamBody := strings.Join([]string{
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"grok-4","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"}}]}`,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"grok-4","choices":[],"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120,"prompt_tokens_details":{"cached_tokens":64,"text_tokens":36},"completion_tokens_details":{"text_tokens":8,"reasoning_tokens":12}}}`,
		`data: [DONE]`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(streamBody)),
	}
	info := &relaycommon.RelayInfo{DisablePing: true}

	usage, apiErr := xAIStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 100, usage.PromptTokens)
	require.Equal(t, 20, usage.CompletionTokens)
	require.Equal(t, 120, usage.TotalTokens)
	require.Equal(t, 64, usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 36, usage.PromptTokensDetails.TextTokens)
	require.Equal(t, 12, usage.CompletionTokenDetails.ReasoningTokens)
	require.Equal(t, 8, usage.CompletionTokenDetails.TextTokens)
}
