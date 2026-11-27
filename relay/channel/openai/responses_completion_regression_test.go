package openai

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestOaiResponsesStreamHandlerStopsAfterDeliveredCompletion verifies that a
// successfully delivered response.completed event ends the Responses stream
// immediately: later upstream deltas are not forwarded, usage from the
// completion is preserved, and exactly one local [DONE] is emitted.
func TestOaiResponsesStreamHandlerStopsAfterDeliveredCompletion(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","object":"response","created_at":1741382417,"model":"gpt-test","output":[]}}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","created_at":1741382417,"model":"gpt-test","output":[],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}`,
		``,
		// The upstream keeps sending after completion; these must not be forwarded.
		`data: {"type":"response.output_text.delta","delta":"TRAILING"}`,
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
		DisablePing: true,
	}

	usage, relayErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, relayErr)
	require.Equal(t, 10, usage.PromptTokens)
	require.Equal(t, 5, usage.CompletionTokens)
	require.Equal(t, 15, usage.TotalTokens)

	responseBody := recorder.Body.String()
	require.Contains(t, responseBody, "event: response.completed")
	require.Contains(t, responseBody, `"type":"response.completed"`)
	require.NotContains(t, responseBody, "TRAILING")
	require.Equal(t, 1, strings.Count(responseBody, "data: [DONE]"))
}

type failWriteResponseWriter struct {
	gin.ResponseWriter
	failOnWrite int
	writes      int
}

func (w *failWriteResponseWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.writes >= w.failOnWrite {
		return 0, errors.New("simulated response write failure")
	}
	return w.ResponseWriter.Write(data)
}

// TestOaiResponsesStreamHandlerWriteFailureSuppressesDone verifies that a
// failed SSE write stops the handler, records a stream error, and never emits
// the local [DONE] sentinel that would make a truncated stream look complete.
func TestOaiResponsesStreamHandlerWriteFailureSuppressesDone(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","object":"response","created_at":1741382417,"model":"gpt-test","output":[]}}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","created_at":1741382417,"model":"gpt-test","output":[],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	// Event line, data payload and blank line are three writes per event.
	// Failing the fifth write rejects the completed event payload.
	c.Writer = &failWriteResponseWriter{ResponseWriter: c.Writer, failOnWrite: 5}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
		IsStream:    true,
		DisablePing: true,
	}

	_, relayErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, relayErr)
	require.NotNil(t, info.StreamStatus)
	require.True(t, info.StreamStatus.HasErrors())

	responseBody := recorder.Body.String()
	require.Contains(t, responseBody, `"type":"response.created"`)
	require.NotContains(t, responseBody, `"type":"response.completed"`)
	require.NotContains(t, responseBody, "data: [DONE]")
}
