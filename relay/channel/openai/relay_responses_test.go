package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOaiResponsesHandlerNormalizesTimestampAndReasoningUsage(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	body := `{"id":"resp_1","object":"response","created_at":1741382417.0,"model":"gpt-test","output":[],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":99,"reasoning_tokens":4}}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	usage, relayErr := OaiResponsesHandler(c, nil, resp)
	require.Nil(t, relayErr)
	require.Equal(t, 10, usage.PromptTokens)
	require.Equal(t, 5, usage.CompletionTokens)
	require.Equal(t, 99, usage.TotalTokens)
	require.Equal(t, 4, usage.CompletionTokenDetails.ReasoningTokens)

	responseBody := recorder.Body.String()
	require.Contains(t, responseBody, `"created_at":1741382417`)
	require.NotContains(t, responseBody, `"created_at":1741382417.0`)

	var forwarded dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal([]byte(responseBody), &forwarded))
	require.NotNil(t, forwarded.Usage)
	require.NotNil(t, forwarded.Usage.OutputTokensDetails)
	require.Equal(t, 4, forwarded.Usage.OutputTokensDetails.ReasoningTokens)
	require.Equal(t, 99, forwarded.Usage.TotalTokens)
	require.NotContains(t, responseBody, `"prompt_tokens"`)
	require.NotContains(t, responseBody, `"completion_tokens"`)
}

func TestOaiResponsesHandlerPreservesCacheWriteUsageForAccounting(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	body := `{"id":"resp_cache","object":"response","created_at":1741382417,"model":"gpt-test","output":[],"usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105,"input_tokens_details":{"cached_tokens":20,"cache_write_tokens":30}}}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}

	usage, relayErr := OaiResponsesHandler(c, nil, resp)
	require.Nil(t, relayErr)
	require.Equal(t, 20, usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 30, usage.PromptTokensDetails.CacheWriteTokens)
	require.Equal(t, 30, usage.PromptTokensDetails.CacheCreationTokensTotal())
	require.Contains(t, recorder.Body.String(), `"cache_write_tokens":30`)
}

func TestOaiResponsesToChatStreamDoesNotDuplicateCompletedToolCall(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","object":"response","created_at":1741382417,"model":"gpt-test","output":[]}}`,
		``,
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":""}}`,
		``,
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"{\"query\":"}`,
		``,
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"\"status\"}"}`,
		``,
		`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":0,"arguments":"{\"query\":\"status\"}"}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","created_at":1741382417,"model":"gpt-test","output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":"{\"query\":\"status\"}"}],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
		IsStream:    true,
		RelayFormat: types.RelayFormatOpenAI,
	}

	usage, relayErr := OaiResponsesToChatStreamHandler(c, info, resp)
	require.Nil(t, relayErr)
	require.Equal(t, 15, usage.TotalTokens)

	var toolChunks []dto.ChatCompletionsStreamResponse
	for _, line := range strings.Split(recorder.Body.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}
		var chunk dto.ChatCompletionsStreamResponse
		require.NoError(t, common.UnmarshalJsonStr(strings.TrimPrefix(line, "data: "), &chunk))
		if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			toolChunks = append(toolChunks, chunk)
		}
	}

	require.Len(t, toolChunks, 3)
	var arguments strings.Builder
	for _, chunk := range toolChunks {
		require.Len(t, chunk.Choices[0].Delta.ToolCalls, 1)
		toolCall := chunk.Choices[0].Delta.ToolCalls[0]
		require.NotNil(t, toolCall.Index)
		require.Equal(t, 0, *toolCall.Index)
		arguments.WriteString(toolCall.Function.Arguments)
	}
	require.Equal(t, `{"query":"status"}`, arguments.String())
	require.Equal(t, 1, strings.Count(recorder.Body.String(), `"name":"lookup"`))
	require.Equal(t, 1, strings.Count(recorder.Body.String(), "data: [DONE]"))
}

func TestOaiResponsesStreamHandlerNormalizesUsageAndSendsOneDone(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","object":"response","created_at":1741382417.0,"model":"gpt-test","output":[]}}`,
		``,
		`data: {"type":"response.completed","sequence_number":42,"response":{"id":"resp_1","object":"response","created_at":1741382417.25,"model":"gpt-test","output":[],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":99,"input_tokens_details":{"cached_tokens":2,"cache_write_tokens":3},"output_tokens_details":{"reasoning_tokens":6}}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
		IsStream:    true,
	}

	usage, relayErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, relayErr)
	require.Equal(t, 10, usage.PromptTokens)
	require.Equal(t, 5, usage.CompletionTokens)
	require.Equal(t, 99, usage.TotalTokens)
	require.Equal(t, 6, usage.CompletionTokenDetails.ReasoningTokens)
	require.Equal(t, 2, usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 3, usage.PromptTokensDetails.CacheWriteTokens)

	responseBody := recorder.Body.String()
	require.Contains(t, responseBody, `"created_at":1741382417`)
	require.NotContains(t, responseBody, `"created_at":1741382417.0`)
	require.NotContains(t, responseBody, `"created_at":1741382417.25`)
	require.Contains(t, responseBody, `"output_tokens_details":{"reasoning_tokens":6}`)
	require.Contains(t, responseBody, `"sequence_number":42`)
	require.NotContains(t, responseBody, `"prompt_tokens"`)
	require.NotContains(t, responseBody, `"completion_tokens"`)
	require.Equal(t, 1, strings.Count(responseBody, "data: [DONE]"))
}

func TestOaiResponsesStreamHandlerTimeoutSuppressesDone(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 1
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	// Upstream never sends a terminal event: the stream must end via the
	// streaming timeout and must NOT look like a clean completion.
	pr, pw := io.Pipe()
	defer pw.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
		IsStream:    true,
	}

	_, relayErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, relayErr)
	require.NotContains(t, recorder.Body.String(), "[DONE]")
}

func TestOaiResponsesToChatStreamHandlesResponseDoneTerminal(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	// Codex-style terminal event is response.done (no upstream [DONE] sentinel).
	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","object":"response","created_at":1741382417,"model":"gpt-test","output":[]}}`,
		``,
		`data: {"type":"response.output_text.delta","delta":"OK"}`,
		``,
		`data: {"type":"response.done","response":{"id":"resp_1","object":"response","created_at":1741382417,"model":"gpt-test","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":12,"output_tokens":7,"total_tokens":19,"output_tokens_details":{"reasoning_tokens":4}}}}`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
		IsStream:    true,
		RelayFormat: types.RelayFormatOpenAI,
	}

	usage, relayErr := OaiResponsesToChatStreamHandler(c, info, resp)
	require.Nil(t, relayErr)
	require.Equal(t, 12, usage.PromptTokens)
	require.Equal(t, 7, usage.CompletionTokens)
	require.Equal(t, 19, usage.TotalTokens)
	require.Equal(t, 4, usage.CompletionTokenDetails.ReasoningTokens)

	responseBody := recorder.Body.String()
	require.Contains(t, responseBody, `"finish_reason":"stop"`)
	require.NotContains(t, responseBody, `"output_tokens_details"`)
	require.Equal(t, 1, strings.Count(responseBody, "data: [DONE]"))
}
