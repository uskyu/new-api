package plugins_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	supplyVideo480NoVideo       = "seedance2.5-480p-token-no-video"
	supplyVideo480WithVideo     = "seedance2.5-480p-token-with-video"
	supplyVideo720NoVideo       = "seedance2.5-720p-token-no-video"
	supplyVideo720WithVideo     = "seedance2.5-720p-token-with-video"
	supplyVideo1080NoVideo      = "seedance2.5-1080p-token-no-video"
	supplyVideo1080WithVideo    = "seedance2.5-1080p-token-with-video"
	supplyVideoMini480NoVideo   = "seedance2-mini-480p-token-no-video"
	supplyVideoMini480WithVideo = "seedance2-mini-480p-token-with-video"
	supplyVideoMini720NoVideo   = "seedance2-mini-720p-token-no-video"
	supplyVideoMini720WithVideo = "seedance2-mini-720p-token-with-video"
	supplyVideoFast480NoVideo   = "seedance2-fast-480p-token-no-video"
	supplyVideoFast480WithVideo = "seedance2-fast-480p-token-with-video"
	supplyVideoFast720NoVideo   = "seedance2-fast-720p-token-no-video"
	supplyVideoFast720WithVideo = "seedance2-fast-720p-token-with-video"
	supplyVideo4KNoVideo        = "seedance2-4k-token-no-video"
	supplyVideo4KWithVideo      = "seedance2-4k-token-with-video"
	supplyVideo2NoVideo         = "seedance2-480p-token-no-video"
	supplyVideo2WithVideo       = "seedance2-480p-token-with-video"
	supplyVideo2_720NoVideo     = "seedance2-720p-token-no-video"
	supplyVideo2_720WithVideo   = "seedance2-720p-token-with-video"
	supplyVideo2_1080NoVideo    = "seedance2-1080p-token-no-video"
	supplyVideo2_1080WithVideo  = "seedance2-1080p-token-with-video"
)

func newSupplyVideoPlugin(t *testing.T) (*jsplugin.Registry, *jsplugin.LoadedPlugin) {
	t.Helper()
	source, err := builtinplugins.Source("supplyvideo")
	require.NoError(t, err)
	registry := jsplugin.NewRegistry()
	plugin, err := registry.RegisterFactory(source, jsplugin.Options{Key: "supplyvideo"})
	require.NoError(t, err)
	return registry, plugin
}

func supplyVideoObject(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, common.Unmarshal(encoded, &result))
	return result
}

func supplyVideoCall(t *testing.T, plugin *jsplugin.LoadedPlugin, hook string, args ...any) map[string]any {
	t.Helper()
	value, err := plugin.Engine.Call(t.Context(), hook, args...)
	require.NoError(t, err, hook)
	return supplyVideoObject(t, value)
}

func supplyVideoDecode(t *testing.T, plugin *jsplugin.LoadedPlugin, model string, body map[string]any) (map[string]any, error) {
	t.Helper()
	value, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "decodeRequest"}, map[string]any{
		"model": model,
		"body":  map[string]any{"kind": "json", "value": body},
	})
	if err != nil {
		return nil, err
	}
	return supplyVideoObject(t, value), nil
}

func supplyVideoRoles(role string, count int) []any {
	items := make([]any, 0, count)
	for index := range count {
		items = append(items, map[string]any{
			"url":  fmt.Sprintf("https://cdn.example/%s-%d.mp4", role, index),
			"role": role,
		})
	}
	return items
}

func TestSupplyVideoManifestAndUsageProfiles(t *testing.T) {
	registry, plugin := newSupplyVideoPlugin(t)
	models := []string{
		supplyVideo480NoVideo, supplyVideo480WithVideo,
		supplyVideo720NoVideo, supplyVideo720WithVideo,
		supplyVideo1080NoVideo, supplyVideo1080WithVideo,
		supplyVideoMini480NoVideo, supplyVideoMini480WithVideo,
		supplyVideoMini720NoVideo, supplyVideoMini720WithVideo,
		supplyVideoFast480NoVideo, supplyVideoFast480WithVideo,
		supplyVideoFast720NoVideo, supplyVideoFast720WithVideo,
		supplyVideo4KNoVideo, supplyVideo4KWithVideo,
		supplyVideo2NoVideo, supplyVideo2WithVideo,
		supplyVideo2_720NoVideo, supplyVideo2_720WithVideo,
		supplyVideo2_1080NoVideo, supplyVideo2_1080WithVideo,
	}
	assert.Equal(t, models, plugin.Meta.Models)
	assert.True(t, plugin.Meta.SupportsUpstream(jsplugin.UpstreamKindNewAPI))
	assert.Equal(t, []string{"vendor", "new_api"}, plugin.Meta.Upstreams)
	assert.Equal(t, []jsplugin.ProtocolClaim{{Name: "openai_video"}}, plugin.Meta.Protocols)
	assert.False(t, plugin.Meta.ProtocolSupports("openai_responses", "sync"))

	// Only the create operation carries a model field, so it is the operation
	// indexed per model. Retrieve and content are host-wide routes from the
	// protocol definition below.
	for _, model := range models {
		binding, found := registry.Generation().LookupEndpoint(http.MethodPost, "/v1/videos", model)
		require.True(t, found, "%s must bind to POST /v1/videos", model)
		assert.Same(t, plugin, binding.Plugin)
		_, found = registry.Generation().LookupEndpoint(http.MethodPost, "/v1/responses", model)
		assert.False(t, found, "%s must not claim the Responses endpoint", model)
	}
	definition, found := jsplugin.HostProtocol("openai_video")
	require.True(t, found)
	operations := make(map[string]bool)
	for _, operation := range definition.Operations {
		operations[operation.Name] = true
	}
	assert.True(t, operations["create"])
	assert.True(t, operations["retrieve"])
	assert.True(t, operations["content"])

	wantResolution := map[string]string{
		supplyVideo480NoVideo: "480p", supplyVideo480WithVideo: "480p",
		supplyVideo720NoVideo: "720p", supplyVideo720WithVideo: "720p",
		supplyVideo1080NoVideo: "1080p", supplyVideo1080WithVideo: "1080p",
		supplyVideoMini480NoVideo: "480p", supplyVideoMini480WithVideo: "480p",
		supplyVideoMini720NoVideo: "720p", supplyVideoMini720WithVideo: "720p",
		supplyVideoFast480NoVideo: "480p", supplyVideoFast480WithVideo: "480p",
		supplyVideoFast720NoVideo: "720p", supplyVideoFast720WithVideo: "720p",
		supplyVideo4KNoVideo: "4k", supplyVideo4KWithVideo: "4k",
		supplyVideo2NoVideo: "480p", supplyVideo2WithVideo: "480p",
		supplyVideo2_720NoVideo: "720p", supplyVideo2_720WithVideo: "720p",
		supplyVideo2_1080NoVideo: "1080p", supplyVideo2_1080WithVideo: "1080p",
	}

	usageKeys := make([]string, 0, len(plugin.Meta.UsageSchema))
	for key := range plugin.Meta.UsageSchema {
		usageKeys = append(usageKeys, key)
	}
	slices.Sort(usageKeys)
	require.Equal(t, []string{"resolution", "tokens", "video_input"}, usageKeys)
	tokens := plugin.Meta.UsageSchema["tokens"]
	assert.Equal(t, "number", tokens.Type)
	assert.Equal(t, "token", tokens.Unit)
	assert.Equal(t, "Video generation token unit price", tokens.Description["en"])
	assert.Equal(t, "视频生成 Token 单价", tokens.Description["zh"])
	assert.Equal(t, []string{"480p", "720p", "1080p", "4k"}, plugin.Meta.UsageSchema["resolution"].Enum)
	assert.Equal(t, []string{"none", "video"}, plugin.Meta.UsageSchema["video_input"].Enum)
	assert.Equal(t, "No reference video", plugin.Meta.UsageSchema["video_input"].EnumLabels["none"]["en"])
	assert.Equal(t, "With reference video", plugin.Meta.UsageSchema["video_input"].EnumLabels["video"]["en"])

	assert.Len(t, plugin.Meta.UsageProfiles, 8)
	for _, model := range models {
		resolution := wantResolution[model]
		require.NotEmpty(t, resolution, model)
		videoInput := "none"
		if strings.HasSuffix(model, "-with-video") {
			videoInput = "video"
		}
		schema, examples := plugin.Meta.UsageForModel(model)
		assert.Equal(t, []string{resolution}, schema["resolution"].Enum, model)
		assert.Equal(t, []string{videoInput}, schema["video_input"].Enum, model)
		require.Len(t, examples, 1, model)
		assert.Equal(t, fmt.Sprintf("100k tokens · %s · %s", resolution, map[string]string{"none": "no reference video", "video": "reference video"}[videoInput]), examples[0].Label)
		assert.Equal(t, map[string]any{"tokens": int64(100000), "resolution": resolution, "video_input": videoInput}, examples[0].Facts)

	}
}

func TestSupplyVideoDecodeRequest(t *testing.T) {
	_, plugin := newSupplyVideoPlugin(t)

	t.Run("no-video request defaults ratio and preserves normalized duration and resolution", func(t *testing.T) {
		decoded, err := supplyVideoDecode(t, plugin, supplyVideo480NoVideo, map[string]any{
			"model": supplyVideo480NoVideo, "prompt": "a cat", "seconds": 8, "duration": 8,
		})
		require.NoError(t, err)
		assert.Equal(t, "submit", decoded["kind"])
		assert.Equal(t, supplyVideo480NoVideo, decoded["model"])
		assert.Equal(t, "text_to_video", decoded["action"])
		assert.Equal(t, map[string]any{
			"model": supplyVideo480NoVideo, "prompt": "a cat", "seconds": float64(8), "aspect_ratio": "16:9",
		}, decoded["requestBody"])
		facts := supplyVideoCall(t, plugin, "extractUsage", map[string]any{
			"model": supplyVideo480NoVideo, "requestBody": decoded["requestBody"],
		})
		assert.Equal(t, map[string]any{"tokens": float64(0), "resolution": "480p", "video_input": "none"}, facts)
	})

	t.Run("size maps the explicitly documented 720p orientations", func(t *testing.T) {
		for _, size := range []string{"1280x720", "720x1280"} {
			decoded, err := supplyVideoDecode(t, plugin, supplyVideo720NoVideo, map[string]any{
				"prompt": "a cat", "seconds": 8, "size": size,
			})
			require.NoError(t, err, size)
			assert.Equal(t, "720p", supplyVideoCall(t, plugin, "extractUsage", map[string]any{
				"model": supplyVideo720NoVideo, "requestBody": decoded["requestBody"],
			})["resolution"], size)
			assert.Equal(t, size, decoded["requestBody"].(map[string]any)["size"], size)
		}
	})

	t.Run("with-video request keeps its reference and aspect ratio", func(t *testing.T) {
		videoRoles := []any{map[string]any{"url": "https://cdn.example/reference.mp4", "role": "reference_video"}}
		decoded, err := supplyVideoDecode(t, plugin, supplyVideo720WithVideo, map[string]any{
			"model": supplyVideo720WithVideo, "prompt": "a cat", "seconds": 12, "duration": 12,
			"aspect_ratio": "9:16", "resolution": "720p", "video_with_roles": videoRoles,
		})
		require.NoError(t, err)
		assert.Equal(t, "image_to_video", decoded["action"])
		assert.Equal(t, map[string]any{
			"model": supplyVideo720WithVideo, "prompt": "a cat", "seconds": float64(12), "aspect_ratio": "9:16",
			"video_with_roles": videoRoles,
		}, decoded["requestBody"])
		facts := supplyVideoCall(t, plugin, "extractUsage", map[string]any{
			"model": supplyVideo720WithVideo, "requestBody": decoded["requestBody"],
		})
		assert.Equal(t, map[string]any{"tokens": float64(0), "resolution": "720p", "video_input": "video"}, facts)
	})

	t.Run("last frame is accepted only as a paired endpoint frame", func(t *testing.T) {
		imageRoles := []any{
			map[string]any{"url": "https://cdn.example/first.png", "role": "first_frame"},
			map[string]any{"url": "https://cdn.example/last.png", "role": "last_frame"},
		}
		decoded, err := supplyVideoDecode(t, plugin, supplyVideoMini480NoVideo, map[string]any{
			"prompt": "a cat", "seconds": 8, "image_with_roles": imageRoles,
		})
		require.NoError(t, err)
		assert.Equal(t, "image_to_video", decoded["action"])
		assert.Equal(t, imageRoles, decoded["requestBody"].(map[string]any)["image_with_roles"])
	})

	t.Run("reference-image-only mode does not require a frame role", func(t *testing.T) {
		imageRoles := []any{map[string]any{"url": "https://cdn.example/reference.png", "role": "reference_image"}}
		decoded, err := supplyVideoDecode(t, plugin, supplyVideo480NoVideo, map[string]any{
			"prompt": "a cat", "seconds": 8, "image_with_roles": imageRoles,
		})
		require.NoError(t, err)
		assert.Equal(t, "image_to_video", decoded["action"])
		assert.Equal(t, imageRoles, decoded["requestBody"].(map[string]any)["image_with_roles"])
	})

	for _, tc := range []struct {
		name    string
		model   string
		body    map[string]any
		wantErr string
	}{
		{"unsupported model", "not-a-seedance-model", map[string]any{"prompt": "a cat", "seconds": 8}, "model is not supported"},
		{"request model must match selected alias", supplyVideo480NoVideo, map[string]any{"model": supplyVideo720NoVideo, "prompt": "a cat", "seconds": 8}, "model must match the selected model"},
		{"prompt required", supplyVideo480NoVideo, map[string]any{"seconds": 8}, "field prompt is required"},
		{"unknown request field", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "quality": "high"}, "field quality is not supported"},
		{"unsupported operation", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "video_operation": "extend"}, "only video_operation generate is supported"},
		{"audio material is unsupported", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "audio_with_roles": []any{}}, "audio_with_roles is not supported"},
		{"duration must match seconds", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "duration": 9}, "duration must match seconds"},
		{"duration requires seconds", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "duration": 8}, "duration must match seconds"},
		{"seconds lower bound", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 3}, "seconds must be an integer between 4 and 30"},
		{"seconds upper bound", supplyVideo2NoVideo, map[string]any{"prompt": "a cat", "seconds": 16}, "seconds must be an integer between 4 and 15"},
		{"seconds must be integral", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 4.5}, "seconds must be an integer between 4 and 30"},
		{"unsupported aspect ratio", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "aspect_ratio": "2:1"}, "aspect_ratio must be one of"},
		{"resolution must match model", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "resolution": "720p"}, "resolution conflicts with the selected model"},
		{"size conflicts with locked model resolution", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "size": "1280x720"}, "size conflicts with the selected model"},
		{"unsupported size rejected", supplyVideo720NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "size": "2048x2048"}, "size must be one of"},
		{"with-video model requires a reference video", supplyVideo480WithVideo, map[string]any{"prompt": "a cat", "seconds": 8}, "with-video models require at least one reference_video"},
		{"last frame requires first frame", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "image_with_roles": []any{map[string]any{"url": "https://cdn.example/last.png", "role": "last_frame"}}}, "last_frame requires first_frame"},
		{"frame pair is exclusive with reference image mode", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "image_with_roles": []any{
			map[string]any{"url": "https://cdn.example/first.png", "role": "first_frame"},
			map[string]any{"url": "https://cdn.example/last.png", "role": "last_frame"},
			map[string]any{"url": "https://cdn.example/reference.png", "role": "reference_image"},
		}}, "frame pair cannot be combined with reference_image"},
		{"frame pair is exclusive with reference video mode", supplyVideo720WithVideo, map[string]any{"prompt": "a cat", "seconds": 8, "video_with_roles": []any{map[string]any{"url": "https://cdn.example/reference.mp4", "role": "reference_video"}}, "image_with_roles": []any{
			map[string]any{"url": "https://cdn.example/first.png", "role": "first_frame"},
			map[string]any{"url": "https://cdn.example/last.png", "role": "last_frame"},
		}}, "frame pair cannot be combined with reference_video"},
		{"first and last frame are mutually unique", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "image_with_roles": []any{
			map[string]any{"url": "https://cdn.example/first.png", "role": "first_frame"},
			map[string]any{"url": "https://cdn.example/first-again.png", "role": "first_frame"},
		}}, "image_with_roles may contain only one first_frame"},
		{"last frame is unique", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "image_with_roles": []any{
			map[string]any{"url": "https://cdn.example/first.png", "role": "first_frame"},
			map[string]any{"url": "https://cdn.example/last.png", "role": "last_frame"},
			map[string]any{"url": "https://cdn.example/last-again.png", "role": "last_frame"},
		}}, "image_with_roles may contain only one last_frame"},
		{"no-video model rejects reference video", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "video_with_roles": []any{map[string]any{"url": "https://cdn.example/reference.mp4", "role": "reference_video"}}}, "no-video models do not accept reference_video"},
		{"seedance2.5 last frame requires adaptive ratio", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "image_with_roles": []any{
			map[string]any{"url": "https://cdn.example/first.png", "role": "first_frame"},
			map[string]any{"url": "https://cdn.example/last.png", "role": "last_frame"},
		}}, "seedance2.5 last_frame requires aspect_ratio adaptive"},
		{"invalid image role", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "image_with_roles": []any{map[string]any{"url": "https://cdn.example/ref.png", "role": "reference_video"}}}, "image_with_roles.role is not supported"},
		{"invalid material URL", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "image_with_roles": []any{map[string]any{"url": "ftp://cdn.example/first.png", "role": "first_frame"}}}, "image_with_roles.url must be an HTTP or HTTPS URL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := supplyVideoDecode(t, plugin, tc.model, tc.body)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}

	t.Run("material caps follow model families", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			model     string
			maxImages int
			maxVideos int
		}{
			{"Seedance 2.5", supplyVideo480WithVideo, 30, 10},
			{"Seedance 2.0", supplyVideo2WithVideo, 9, 3},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tooManyImages := append([]any{map[string]any{"url": "https://cdn.example/first.png", "role": "first_frame"}}, supplyVideoRoles("reference_image", tc.maxImages)...)
				_, err := supplyVideoDecode(t, plugin, tc.model, map[string]any{
					"prompt": "a cat", "seconds": 8, "video_with_roles": supplyVideoRoles("reference_video", 1), "image_with_roles": tooManyImages,
				})
				require.ErrorContains(t, err, fmt.Sprintf("image_with_roles supports at most %d items", tc.maxImages))

				_, err = supplyVideoDecode(t, plugin, tc.model, map[string]any{
					"prompt": "a cat", "seconds": 8, "video_with_roles": supplyVideoRoles("reference_video", tc.maxVideos+1),
				})
				require.ErrorContains(t, err, fmt.Sprintf("video_with_roles supports at most %d items", tc.maxVideos))
			})
		}
	})
}

func TestSupplyVideoDriverResponsesAndArtifacts(t *testing.T) {
	_, plugin := newSupplyVideoPlugin(t)
	const vendorBaseURL = "https://supplyvideo.example"
	const gatewayBaseURL = "https://gateway.example"
	const apiKey = "supply-secret"
	requestBody := map[string]any{"model": supplyVideo480NoVideo, "prompt": "a cat", "seconds": 8.0, "aspect_ratio": "16:9"}

	for _, upstream := range []struct {
		name    string
		baseURL string
		kind    string
	}{
		{"vendor", vendorBaseURL, "vendor"},
		{"new_api", gatewayBaseURL, "new_api"},
	} {
		t.Run(upstream.name, func(t *testing.T) {
			ctx := map[string]any{
				"baseUrl": upstream.baseURL, "apiKey": apiKey, "upstream": map[string]any{"kind": upstream.kind},
				"model": supplyVideo480NoVideo, "requestBody": requestBody,
			}
			submit := supplyVideoCall(t, plugin, "buildSubmitRequest", ctx)
			assert.Equal(t, upstream.baseURL+"/v1/videos", submit["url"])
			assert.Equal(t, http.MethodPost, submit["method"])
			assert.Equal(t, map[string]any{"Authorization": "Bearer " + apiKey, "Content-Type": "application/json"}, submit["headers"])
			assert.Equal(t, requestBody, submit["body"])

			query := supplyVideoCall(t, plugin, "buildQueryRequest", map[string]any{"baseUrl": upstream.baseURL, "apiKey": apiKey, "taskId": "upstream-task"})
			assert.Equal(t, upstream.baseURL+"/v1/videos/upstream-task", query["url"])
			assert.Equal(t, http.MethodGet, query["method"])
			assert.Equal(t, map[string]any{"Authorization": "Bearer " + apiKey}, query["headers"])

			for _, method := range []string{http.MethodGet, http.MethodHead} {
				content := supplyVideoCall(t, plugin, "buildContentRequest", map[string]any{
					"baseUrl": upstream.baseURL, "apiKey": apiKey, "artifactKey": "video", "upstreamTaskId": "task/with space",
					"clientRequest": map[string]any{"method": method},
				})
				assert.Equal(t, upstream.baseURL+"/v1/videos/task%2Fwith%20space/content", content["url"])
				assert.Equal(t, method, content["method"])
				assert.Equal(t, map[string]any{"Authorization": "Bearer " + apiKey}, content["headers"])
			}
		})
	}

	for _, tc := range []struct {
		name string
		body map[string]any
		want string
	}{
		{"bare id", map[string]any{"id": "bare-task"}, "bare-task"},
		{"bare task_id", map[string]any{"task_id": "bare-task-id"}, "bare-task-id"},
		{"data wrapper id", map[string]any{"data": map[string]any{"id": "wrapped-task"}}, "wrapped-task"},
		{"data wrapper task_id", map[string]any{"data": map[string]any{"task_id": "wrapped-task-id"}}, "wrapped-task-id"},
	} {
		t.Run("submit response "+tc.name, func(t *testing.T) {
			parsed := supplyVideoCall(t, plugin, "parseSubmitResponse", nil, map[string]any{"body": tc.body})
			assert.Equal(t, tc.want, parsed["taskId"])
			assert.Equal(t, tc.body, parsed["taskData"])
		})
	}

	for _, tc := range []struct {
		status string
		want   string
	}{
		{"queued", "QUEUED"},
		{"in_progress", "IN_PROGRESS"},
		{"completed", "SUCCESS"},
		{"failed", "FAILURE"},
	} {
		t.Run("status "+tc.status, func(t *testing.T) {
			parsed := supplyVideoCall(t, plugin, "parseTaskResult", nil, map[string]any{"status": tc.status})
			assert.Equal(t, tc.want, parsed["status"])
		})
	}

	t.Run("completed response maps total_tokens to facts and totalTokens", func(t *testing.T) {
		body := map[string]any{"status": "completed", "usage": map[string]any{"total_tokens": 12345}}
		facts := supplyVideoCall(t, plugin, "extractUsageOnComplete", nil, map[string]any{"status": "SUCCESS"}, body)
		assert.Equal(t, map[string]any{"tokens": float64(12345), "totalTokens": float64(12345)}, facts)
		parsed := supplyVideoCall(t, plugin, "parseTaskResult", nil, body)
		assert.Equal(t, "SUCCESS", parsed["status"])
		assert.Equal(t, float64(12345), parsed["totalTokens"])
	})

	t.Run("missing completion usage does not fabricate billing facts", func(t *testing.T) {
		for _, body := range []map[string]any{
			{"status": "completed"},
			{"status": "completed", "usage": map[string]any{}},
			{"status": "completed", "usage": map[string]any{"total_tokens": 0}},
		} {
			facts := supplyVideoCall(t, plugin, "extractUsageOnComplete", nil, map[string]any{"status": "SUCCESS"}, body)
			assert.Empty(t, facts)
		}
	})

	t.Run("openai_video renderer omits upstream identity and media URLs", func(t *testing.T) {
		upstreamURL := "https://upstream.example/internal/video.mp4?signature=secret"
		value, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "render"}, map[string]any{}, map[string]any{
			"task_id": "public-task-id", "status": "SUCCESS", "progress": "100%", "created_at": 10,
			"data": map[string]any{
				"id": "upstream-task-secret", "task_id": "upstream-task-secret",
				"url": upstreamURL, "video_url": upstreamURL,
				"status": "completed", "object": "video",
			},
		})
		require.NoError(t, err)
		encoded, err := common.Marshal(value)
		require.NoError(t, err)
		var response map[string]any
		require.NoError(t, common.Unmarshal(encoded, &response))
		assert.NotContains(t, string(encoded), "upstream-task-secret")
		assert.NotContains(t, string(encoded), upstreamURL)
		assert.Equal(t, "public-task-id", response["id"])
	})

	t.Run("artifact list and content use upstream task identity and authentication", func(t *testing.T) {
		artifactsValue, err := plugin.Engine.Call(t.Context(), "listArtifacts", map[string]any{"status": "SUCCESS"})
		require.NoError(t, err)
		encoded, err := common.Marshal(artifactsValue)
		require.NoError(t, err)
		var artifacts []map[string]any
		require.NoError(t, common.Unmarshal(encoded, &artifacts))
		require.Equal(t, []map[string]any{{"key": "video", "type": "video"}}, artifacts)

		content := supplyVideoCall(t, plugin, "buildContentRequest", map[string]any{
			"baseUrl": vendorBaseURL, "apiKey": apiKey, "artifactKey": "video", "upstreamTaskId": "actual-upstream-id",
			"clientRequest": map[string]any{"method": http.MethodGet},
		})
		assert.Equal(t, vendorBaseURL+"/v1/videos/actual-upstream-id/content", content["url"])
		assert.Equal(t, map[string]any{"Authorization": "Bearer " + apiKey}, content["headers"])
	})
}
