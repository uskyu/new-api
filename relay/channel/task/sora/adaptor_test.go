package sora

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestBodyBindsOpaquePassThroughMetadata(t *testing.T) {
	payload := []byte("opaque-sora-request-body")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/octet-stream")
	defer common.CleanupBodyStorage(c)

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	got, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, payload, got)
	require.EqualValues(t, len(payload), info.UpstreamRequestBodySize)
	require.NotNil(t, info.UpstreamRequestGetBody)

	replay, err := info.UpstreamRequestGetBody()
	require.NoError(t, err)
	defer replay.Close()
	replayed, err := io.ReadAll(replay)
	require.NoError(t, err)
	require.Equal(t, payload, replayed)
}

func TestBuildRequestBodyDoesNotBindConvertedJSONMetadata(t *testing.T) {
	payload := []byte(`{"model":"client-model","prompt":"hello"}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	defer common.CleanupBodyStorage(c)

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "upstream-model"}}
	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	_, err = io.ReadAll(body)
	require.NoError(t, err)
	require.Zero(t, info.UpstreamRequestBodySize)
	require.Nil(t, info.UpstreamRequestGetBody)
}

func TestParseTaskResultCompletedVideoURL(t *testing.T) {
	adaptor := &TaskAdaptor{}
	taskInfo, err := adaptor.ParseTaskResult([]byte(`{
		"id":"task_public",
		"task_id":"task_upstream",
		"object":"video",
		"model":"kling-3.0-omni-1080p-ref-audio",
		"status":"completed",
		"progress":100,
		"meta_data":{"url":"https://example.com/result.mp4"},
		"video_url":"https://example.com/result.mp4"
	}`))
	if err != nil {
		t.Fatalf("ParseTaskResult returned error: %v", err)
	}
	if taskInfo.Status != model.TaskStatusSuccess {
		t.Fatalf("status = %q, want %q", taskInfo.Status, model.TaskStatusSuccess)
	}
	if taskInfo.Url != "https://example.com/result.mp4" {
		t.Fatalf("url = %q, want result URL", taskInfo.Url)
	}
}
