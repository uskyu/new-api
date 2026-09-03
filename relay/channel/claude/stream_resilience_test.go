package claude

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeStreamHandlerDrainsFinalUsageAfterClientDisconnect(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	oldDrainTimeout := constant.ClaudeClientGoneDrainTimeout
	constant.StreamingTimeout = 30
	constant.ClaudeClientGoneDrainTimeout = 1
	t.Cleanup(func() {
		constant.StreamingTimeout = oldStreamingTimeout
		constant.ClaudeClientGoneDrainTimeout = oldDrainTimeout
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pr, pw := io.Pipe()
	t.Cleanup(func() {
		_ = pr.Close()
		_ = pw.Close()
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
	resp := &http.Response{Body: pr}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-3-5-sonnet"},
	}

	type result struct {
		promptTokens     int
		completionTokens int
		err              *types.NewAPIError
	}
	resultChan := make(chan result, 1)
	go func() {
		usage, err := ClaudeStreamHandler(c, resp, info)
		streamResult := result{err: err}
		if usage != nil {
			streamResult.promptTokens = usage.PromptTokens
			streamResult.completionTokens = usage.CompletionTokens
		}
		resultChan <- streamResult
	}()

	_, err := fmt.Fprint(pw, "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-3-5-sonnet\",\"usage\":{\"input_tokens\":100,\"output_tokens\":1}}}\n")
	require.NoError(t, err)
	require.Eventually(t, func() bool { return recorder.Flushed }, time.Second, 10*time.Millisecond)

	cancel()
	_, err = fmt.Fprint(pw, "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":100,\"output_tokens\":25}}\n")
	require.NoError(t, err)
	_, err = fmt.Fprint(pw, "data: [DONE]\n")
	require.NoError(t, err)
	_ = pw.Close()

	select {
	case got := <-resultChan:
		require.Nil(t, got.err)
		require.Equal(t, 100, got.promptTokens)
		require.Equal(t, 25, got.completionTokens)
	case <-time.After(3 * time.Second):
		t.Fatal("Claude stream handler did not finish after draining final usage")
	}
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
}

func TestClaudeStreamHandlerDrainsFinalUsageForOpenAIFormatAfterDisconnect(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-3-5-sonnet"},
	}
	claudeInfo := &ClaudeResponseInfo{Usage: &dto.Usage{}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	_ = HandleStreamResponseData(c, info, claudeInfo, `{"type":"message_start","message":{"id":"msg_1","model":"claude-3-5-sonnet","usage":{"input_tokens":120,"output_tokens":2}}}`)
	_ = HandleStreamResponseData(c, info, claudeInfo, `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":120,"output_tokens":30}}`)

	require.True(t, claudeInfo.Done)
	require.Equal(t, 120, claudeInfo.Usage.PromptTokens)
	require.Equal(t, 30, claudeInfo.Usage.CompletionTokens)
}
