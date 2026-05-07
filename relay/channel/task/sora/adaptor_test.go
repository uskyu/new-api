package sora

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
)

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
