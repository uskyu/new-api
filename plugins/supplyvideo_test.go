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
	"github.com/QuantumNous/new-api/setting/billing_setting"
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

var supplyVideoSecondModels = []string{
	"happyhorse1.1-720p-second",
	"happyhorse1.1-1080p-second",
	"happyhorse1.1-720p-edit-second",
	"grok-video1.5-480p-second",
	"grok-video1.5-720p-second",
	"wan3.0-video-480p",
	"wan3.0-video-720p",
	"wan3.0-video-1080p",
	"wan3.0-video-prime-480p",
	"wan3.0-video-prime-720p",
	"wan3.0-video-prime-1080p",
	"kling4-720p-second-no-video",
	"kling4-720p-second-with-video",
	"kling4-1080p-second-no-video",
	"kling4-1080p-second-with-video",
	"minimax-h3-max-480p-second",
	"minimax-h3-max-768p-second",
	"minimax-h3-max-1080p-second",
}

var supplyVideoPaiCallModels = []string{
	"seedance2-fast-720p-once-1",
	"seedance2-fast-720p-once-2",
	"seedance2-480p-once",
	"seedance2-1080p-once",
	"seedance2-720p-once-1",
	"seedance2-720p-once-2",
	"seedance2-720p-once-3",
	"seedance2.5-720p-once",
}

var supplyVideoPaiSecondModels = []string{
	"seedance2-mini-480p-second",
	"seedance2-mini-720p-second",
	"seedance2-fast-480p-second-1",
	"seedance2-fast-480p-second-2",
	"seedance2-fast-720p-second-1",
	"seedance2-fast-720p-second-2",
	"seedance2-480p-second-1",
	"seedance2-480p-second-2",
	"seedance2-720p-second-1",
	"seedance2-720p-second-2",
	"seedance2-1080p-second-1",
	"seedance2-1080p-second-2",
	"seedance2-4k-second",
	"seedance2.5-480p-second-1",
	"seedance2.5-480p-second-2",
	"seedance2.5-480p-second-3",
	"seedance2.5-480p-second-4",
	"seedance2.5-720p-second-1",
	"seedance2.5-720p-second-2",
	"seedance2.5-720p-second-3",
}

var supplyVideoBananaModels = []string{
	"Nano Banana 2.1 1K",
	"Nano Banana 2.1 2K",
	"Nano Banana 2.1 4K",
	"Nano Banana Pro 1K",
	"Nano Banana Pro 2K",
	"Nano Banana Pro 4K",
}

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
	tokenModels := []string{
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
	var models []string
	models = append(models, tokenModels...)
	models = append(models, supplyVideoSecondModels...)
	models = append(models, supplyVideoPaiCallModels...)
	models = append(models, supplyVideoPaiSecondModels...)
	models = append(models, supplyVideoBananaModels...)
	assert.Equal(t, models, plugin.Meta.Models)
	assert.Equal(t, "1.3.0", plugin.Meta.Version)
	assert.True(t, plugin.Meta.SupportsUpstream(jsplugin.UpstreamKindNewAPI))
	assert.Equal(t, []string{"vendor", "new_api"}, plugin.Meta.Upstreams)
	assert.ElementsMatch(t, []string{"cdn.paisio.online", "files.toapis.xyz", "ark-acg-cn-beijing.tos-cn-beijing.volces.com", "gateway.megarichglobal.com"}, plugin.Meta.AllowedHosts)
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

	assert.Len(t, plugin.Meta.UsageProfiles, 12)
	for _, model := range tokenModels {
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

	for _, model := range supplyVideoSecondModels {
		schema, examples := plugin.Meta.UsageForModel(model)
		require.Contains(t, schema, "input_seconds", model)
		require.Contains(t, schema, "output_seconds", model)
		assert.Equal(t, "second", schema["input_seconds"].Unit, model)
		assert.Equal(t, "second", schema["output_seconds"].Unit, model)
		assert.Equal(t, "Input video unit price", schema["input_seconds"].Description["en"], model)
		assert.Equal(t, "Output video unit price", schema["output_seconds"].Description["en"], model)
		require.Len(t, examples, 1, model)
		assert.Equal(t, map[string]any{"output_seconds": int64(5), "input_seconds": int64(0)}, examples[0].Facts)
	}

	for _, model := range supplyVideoPaiCallModels {
		schema, examples := plugin.Meta.UsageForModel(model)
		assert.Empty(t, schema, model)
		assert.Empty(t, examples, model)
	}
	for _, model := range supplyVideoPaiSecondModels {
		schema, examples := plugin.Meta.UsageForModel(model)
		require.Contains(t, schema, "input_seconds", model)
		require.Contains(t, schema, "output_seconds", model)
		assert.Equal(t, "second", schema["output_seconds"].Unit, model)
		require.Len(t, examples, 1, model)
		assert.Equal(t, map[string]any{"output_seconds": int64(4), "input_seconds": int64(0)}, examples[0].Facts)
	}

	for _, model := range supplyVideoBananaModels {
		schema, examples := plugin.Meta.UsageForModel(model)
		require.Len(t, schema, 1, model)
		field, found := schema["image_count"]
		require.True(t, found, model)
		assert.Equal(t, "number", field.Type, model)
		assert.Equal(t, "count", field.Unit, model)
		assert.Equal(t, "Image generation unit price", field.Description["en"], model)
		assert.Equal(t, "图片生成单价", field.Description["zh"], model)
		require.Len(t, examples, 1, model)
		assert.Equal(t, "1 image", examples[0].Label, model)
		assert.Equal(t, map[string]any{"image_count": int64(1)}, examples[0].Facts, model)
	}
}

func TestSupplyVideoSchemaAcceptsUpstreamTaskExpressions(t *testing.T) {
	_, plugin := newSupplyVideoPlugin(t)

	tokenSchema, _ := plugin.Meta.UsageForModel(supplyVideo480NoVideo)
	require.NoError(t, billing_setting.SmokeTestTaskExpr(`tier("total_tokens", u("tokens") * 2.29 / 1000000)`, tokenSchema))

	expressions := []string{
		`tier("video_seconds", u("output_seconds") * 0.02)`,
		`tier("video_edit_seconds", u("output_seconds") * 0.08)`,
		`u("input_seconds") > u("output_seconds") ? tier("input_longer", u("output_seconds") * 0.12 + u("input_seconds") * 0.12) : tier("output_covers_input", u("output_seconds") * 0.23)`,
		`tier("video_seconds", u("output_seconds") * 0.05 + u("input_seconds") * 0.06)`,
	}
	for _, model := range supplyVideoSecondModels {
		schema, _ := plugin.Meta.UsageForModel(model)
		for _, expression := range expressions {
			require.NoError(t, billing_setting.SmokeTestTaskExpr(expression, schema), "%s: %s", model, expression)
		}
	}

	paiCallSchema, _ := plugin.Meta.UsageForModel(supplyVideoPaiCallModels[0])
	require.NoError(t, billing_setting.SmokeTestTaskExpr(`tier("per_video", 0.52)`, paiCallSchema))
	for _, model := range supplyVideoPaiSecondModels {
		schema, _ := plugin.Meta.UsageForModel(model)
		require.NoError(t, billing_setting.SmokeTestTaskExpr(`tier("video_seconds", u("output_seconds") * 0.04)`, schema), model)
	}

	for _, model := range supplyVideoBananaModels {
		schema, _ := plugin.Meta.UsageForModel(model)
		require.NoError(t, billing_setting.SmokeTestTaskExpr(`tier("images", u("image_count") * 0.009)`, schema), model)
	}
}

func TestSupplyVideoBananaImageModels(t *testing.T) {
	_, plugin := newSupplyVideoPlugin(t)
	const bananaModel = "Nano Banana Pro 2K"
	const vendorBaseURL = "https://supplyvideo.example"
	const apiKey = "supply-secret"
	const supplierImageURL = vendorBaseURL + "/v1/tasks/upstream-banana/artifacts/image/content"

	decoded, err := supplyVideoDecode(t, plugin, bananaModel, map[string]any{"prompt": "a banana"})
	require.NoError(t, err)
	wantBody := map[string]any{
		"model": bananaModel, "prompt": "a banana", "aspect_ratio": "1:1", "n": float64(1),
	}
	assert.Equal(t, "submit", decoded["kind"])
	assert.Equal(t, bananaModel, decoded["model"])
	assert.Equal(t, "text_to_image", decoded["action"])
	assert.Equal(t, wantBody, decoded["requestBody"])

	explicit, err := supplyVideoDecode(t, plugin, bananaModel, map[string]any{
		"model": bananaModel, "prompt": "a banana", "aspect_ratio": "1:1", "n": 1,
	})
	require.NoError(t, err)
	assert.Equal(t, wantBody, explicit["requestBody"])

	submit := supplyVideoCall(t, plugin, "buildSubmitRequest", map[string]any{
		"baseUrl": vendorBaseURL, "apiKey": apiKey, "model": bananaModel, "requestBody": decoded["requestBody"],
	})
	assert.Equal(t, vendorBaseURL+"/v1/videos", submit["url"])
	assert.Equal(t, http.MethodPost, submit["method"])
	assert.Equal(t, map[string]any{"Authorization": "Bearer " + apiKey, "Content-Type": "application/json"}, submit["headers"])
	assert.Equal(t, wantBody, submit["body"])

	for _, tc := range []struct {
		name    string
		body    map[string]any
		wantErr string
	}{
		{"model mismatch", map[string]any{"model": "Nano Banana Pro 1K", "prompt": "a banana"}, "model must match the selected model"},
		{"missing prompt", map[string]any{"n": 1}, "field prompt is required"},
		{"n above one", map[string]any{"prompt": "a banana", "n": 2}, "n must be 1"},
		{"n zero", map[string]any{"prompt": "a banana", "n": 0}, "n must be 1"},
		{"n must be an integer", map[string]any{"prompt": "a banana", "n": "1"}, "n must be 1"},
		{"unsupported aspect ratio", map[string]any{"prompt": "a banana", "aspect_ratio": "16:9"}, "aspect_ratio must be 1:1"},
		{"seconds rejected", map[string]any{"prompt": "a banana", "seconds": 8}, "field seconds is not supported"},
		{"duration rejected", map[string]any{"prompt": "a banana", "duration": 8}, "field duration is not supported"},
		{"resolution rejected", map[string]any{"prompt": "a banana", "resolution": "2K"}, "field resolution is not supported"},
		{"size rejected", map[string]any{"prompt": "a banana", "size": "1024x1024"}, "field size is not supported"},
		{"video operation rejected", map[string]any{"prompt": "a banana", "video_operation": "generate"}, "field video_operation is not supported"},
		{"image roles rejected", map[string]any{"prompt": "a banana", "image_with_roles": []any{map[string]any{"url": "https://cdn.example/a.png", "role": "reference_image"}}}, "field image_with_roles is not supported"},
		{"video roles rejected", map[string]any{"prompt": "a banana", "video_with_roles": []any{map[string]any{"url": "https://cdn.example/a.mp4", "role": "reference_video"}}}, "field video_with_roles is not supported"},
		{"audio roles rejected", map[string]any{"prompt": "a banana", "audio_with_roles": []any{map[string]any{"url": "https://cdn.example/a.mp3", "role": "reference_audio"}}}, "field audio_with_roles is not supported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := supplyVideoDecode(t, plugin, bananaModel, tc.body)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}

	t.Run("usage facts bill one image", func(t *testing.T) {
		assert.Equal(t, map[string]any{"image_count": float64(1)}, supplyVideoCall(t, plugin, "extractUsage", map[string]any{
			"model": bananaModel, "requestBody": decoded["requestBody"],
		}))
		assert.Equal(t, map[string]any{"image_count": float64(1)}, supplyVideoCall(t, plugin, "extractUsage", map[string]any{
			"model": bananaModel, "requestBody": decoded["requestBody"], "usagePurpose": "billing_ratios",
		}))

		imageBody := map[string]any{
			"status": "completed", "model": bananaModel, "progress": 100, "media_type": "image",
			"image_url": supplierImageURL, "url": supplierImageURL,
		}
		for _, ctx := range []map[string]any{
			{"model": bananaModel},
			{"model": "nano-banana-alias", "upstreamModel": bananaModel},
			{},
		} {
			assert.Equal(t, map[string]any{"image_count": float64(1)}, supplyVideoCall(t, plugin, "extractUsageOnComplete", ctx, map[string]any{"status": "SUCCESS"}, imageBody))
		}
		assert.Empty(t, supplyVideoCall(t, plugin, "extractUsageOnComplete", map[string]any{"model": bananaModel}, map[string]any{"status": "FAILURE"}, imageBody))

		videoBody := map[string]any{"status": "completed", "usage": map[string]any{"total_tokens": 12345}}
		assert.Equal(t, map[string]any{"tokens": float64(12345), "totalTokens": float64(12345)}, supplyVideoCall(t, plugin, "extractUsageOnComplete", map[string]any{"model": supplyVideo480NoVideo}, map[string]any{"status": "SUCCESS"}, videoBody))
	})

	t.Run("artifact listing follows the result media type", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			task map[string]any
			want []map[string]any
		}{
			{"media type", map[string]any{"status": "SUCCESS", "data": map[string]any{"media_type": "image"}}, []map[string]any{{"key": "image", "type": "image"}}},
			{"image URL", map[string]any{"status": "SUCCESS", "data": map[string]any{"image_url": supplierImageURL}}, []map[string]any{{"key": "image", "type": "image"}}},
			{"image content URL", map[string]any{"status": "SUCCESS", "data": map[string]any{"url": supplierImageURL + "?signature=abc"}}, []map[string]any{{"key": "image", "type": "image"}}},
			{"video result", map[string]any{"status": "SUCCESS", "data": map[string]any{"video_url": "https://cdn.example/result.mp4"}}, []map[string]any{{"key": "video", "type": "video"}}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				value, err := plugin.Engine.Call(t.Context(), "listArtifacts", tc.task)
				require.NoError(t, err)
				encoded, err := common.Marshal(value)
				require.NoError(t, err)
				var artifacts []map[string]any
				require.NoError(t, common.Unmarshal(encoded, &artifacts))
				assert.Equal(t, tc.want, artifacts)
			})
		}

		value, err := plugin.Engine.Call(t.Context(), "listArtifacts", map[string]any{
			"status": "IN_PROGRESS", "data": map[string]any{"media_type": "image"},
		})
		require.NoError(t, err)
		encoded, err := common.Marshal(value)
		require.NoError(t, err)
		var artifacts []map[string]any
		require.NoError(t, common.Unmarshal(encoded, &artifacts))
		assert.Empty(t, artifacts)
	})

	t.Run("image content request authenticates only to the supplier host", func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			content := supplyVideoCall(t, plugin, "buildContentRequest", map[string]any{
				"baseUrl": vendorBaseURL, "apiKey": apiKey, "artifactKey": "image", "upstreamTaskId": "upstream-banana",
				"clientRequest": map[string]any{"method": method},
				"data":          map[string]any{"media_type": "image", "image_url": supplierImageURL, "url": supplierImageURL},
			})
			assert.Equal(t, supplierImageURL, content["url"])
			assert.Equal(t, method, content["method"])
			assert.Equal(t, map[string]any{"Authorization": "Bearer " + apiKey}, content["headers"])
		}

		fallback := supplyVideoCall(t, plugin, "buildContentRequest", map[string]any{
			"baseUrl": vendorBaseURL, "apiKey": apiKey, "artifactKey": "image", "upstreamTaskId": "upstream-banana",
			"clientRequest": map[string]any{"method": http.MethodGet},
			"data":          map[string]any{"media_type": "image", "url": supplierImageURL},
		})
		assert.Equal(t, supplierImageURL, fallback["url"])

		_, err := plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{
			"baseUrl": vendorBaseURL, "apiKey": apiKey, "artifactKey": "image", "upstreamTaskId": "upstream-banana",
			"clientRequest": map[string]any{"method": http.MethodGet},
			"data":          map[string]any{"media_type": "image", "image_url": "https://cdn.example/v1/tasks/upstream-banana/artifacts/image/content"},
		})
		require.ErrorContains(t, err, "host")
		assert.NotContains(t, err.Error(), apiKey)

		_, err = plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{
			"baseUrl": vendorBaseURL, "apiKey": apiKey, "artifactKey": "image", "upstreamTaskId": "upstream-banana",
			"clientRequest": map[string]any{"method": http.MethodGet},
			"data":          map[string]any{"media_type": "image", "image_url": "http://supplyvideo.example/v1/tasks/upstream-banana/artifacts/image/content"},
		})
		require.ErrorContains(t, err, "host")
		assert.NotContains(t, err.Error(), apiKey)

		_, err = plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{
			"baseUrl": vendorBaseURL, "apiKey": apiKey, "artifactKey": "image", "upstreamTaskId": "upstream-banana",
			"clientRequest": map[string]any{"method": http.MethodPost},
			"data":          map[string]any{"image_url": supplierImageURL},
		})
		require.ErrorContains(t, err, "GET and HEAD")
	})

	t.Run("render publishes a gateway-relative image artifact", func(t *testing.T) {
		upstreamURL := vendorBaseURL + "/v1/tasks/upstream-banana-secret/artifacts/image/content"
		value, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "render"}, map[string]any{}, map[string]any{
			"task_id": "public banana/task", "status": "SUCCESS", "progress": "100%", "created_at": 12,
			"data": map[string]any{
				"id": "upstream-banana-secret", "task_id": "upstream-banana-secret", "status": "completed",
				"media_type": "image", "image_url": upstreamURL, "url": upstreamURL,
			},
		})
		require.NoError(t, err)
		encoded, err := common.Marshal(value)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "upstream-banana-secret")
		assert.NotContains(t, string(encoded), "supplyvideo.example")
		assert.NotContains(t, string(encoded), "https://")
		var response map[string]any
		require.NoError(t, common.Unmarshal(encoded, &response))
		assert.Equal(t, "video", response["object"])
		assert.Equal(t, "image", response["media_type"])
		assert.Equal(t, "public banana/task", response["id"])
		wantPath := "/v1/tasks/public%20banana%2Ftask/artifacts/image/content"
		assert.Equal(t, wantPath, response["image_url"])
		assert.Equal(t, wantPath, response["url"])
	})

	t.Run("render keeps video results unchanged", func(t *testing.T) {
		value, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "render"}, map[string]any{}, map[string]any{
			"task_id": "public-video", "status": "SUCCESS", "progress": "100%", "created_at": 13,
			"data": map[string]any{"status": "completed", "video_url": "https://cdn.example/result.mp4"},
		})
		require.NoError(t, err)
		encoded, err := common.Marshal(value)
		require.NoError(t, err)
		var response map[string]any
		require.NoError(t, common.Unmarshal(encoded, &response))
		assert.Equal(t, "video", response["object"])
		assert.NotContains(t, response, "media_type")
		assert.NotContains(t, response, "image_url")
		assert.NotContains(t, response, "url")
	})
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
		{"video models reject n", supplyVideo480NoVideo, map[string]any{"prompt": "a cat", "seconds": 8, "n": 1}, "field n is not supported"},
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

	t.Run("per-second families normalize their own contracts", func(t *testing.T) {
		t.Run("happyhorse reference image", func(t *testing.T) {
			imageRoles := []any{map[string]any{"url": "https://cdn.example/reference.png", "role": "reference_image"}}
			decoded, err := supplyVideoDecode(t, plugin, "happyhorse1.1-720p-second", map[string]any{
				"prompt": "a horse", "seconds": 5, "aspect_ratio": "9:16", "image_with_roles": imageRoles,
			})
			require.NoError(t, err)
			assert.Equal(t, "image_to_video", decoded["action"])
			assert.Equal(t, map[string]any{
				"model": "happyhorse1.1-720p-second", "prompt": "a horse", "seconds": float64(5),
				"aspect_ratio": "9:16", "image_with_roles": imageRoles,
			}, decoded["requestBody"])
			assert.Equal(t, map[string]any{"output_seconds": float64(0), "input_seconds": float64(0)}, supplyVideoCall(t, plugin, "extractUsage", map[string]any{
				"model": "happyhorse1.1-720p-second", "requestBody": decoded["requestBody"],
			}))
		})

		t.Run("grok first frame at the one-second lower bound", func(t *testing.T) {
			imageRoles := []any{map[string]any{"url": "https://cdn.example/first.png", "role": "first_frame"}}
			decoded, err := supplyVideoDecode(t, plugin, "grok-video1.5-480p-second", map[string]any{
				"prompt": "a rocket", "seconds": 1, "image_with_roles": imageRoles,
			})
			require.NoError(t, err)
			assert.Equal(t, map[string]any{
				"model": "grok-video1.5-480p-second", "prompt": "a rocket", "seconds": float64(1),
				"aspect_ratio": "16:9", "image_with_roles": imageRoles,
			}, decoded["requestBody"])
		})

		t.Run("wan automatic duration with mixed reference media", func(t *testing.T) {
			imageRoles := []any{map[string]any{"url": "https://cdn.example/reference.png", "role": "reference_image"}}
			videoRoles := []any{map[string]any{"url": "https://cdn.example/reference.mp4", "role": "reference_video"}}
			audioRoles := []any{map[string]any{"url": "https://cdn.example/reference.mp3", "role": "reference_audio"}}
			decoded, err := supplyVideoDecode(t, plugin, "wan3.0-video-720p", map[string]any{
				"prompt": "a river", "seconds": -1, "aspect_ratio": "adaptive",
				"image_with_roles": imageRoles, "video_with_roles": videoRoles, "audio_with_roles": audioRoles,
			})
			require.NoError(t, err)
			assert.Equal(t, "image_to_video", decoded["action"])
			assert.Equal(t, map[string]any{
				"model": "wan3.0-video-720p", "prompt": "a river", "seconds": float64(-1),
				"aspect_ratio": "adaptive", "image_with_roles": imageRoles,
				"video_with_roles": videoRoles, "audio_with_roles": audioRoles,
			}, decoded["requestBody"])
		})

		t.Run("kling with-video defaults to auto and requires a video", func(t *testing.T) {
			videoRoles := []any{map[string]any{"url": "https://cdn.example/source.mp4", "role": "reference_video"}}
			decoded, err := supplyVideoDecode(t, plugin, "kling4-720p-second-with-video", map[string]any{
				"prompt": "a dancer", "seconds": 10, "video_with_roles": videoRoles,
			})
			require.NoError(t, err)
			assert.Equal(t, map[string]any{
				"model": "kling4-720p-second-with-video", "prompt": "a dancer", "seconds": float64(10),
				"aspect_ratio": "auto", "video_with_roles": videoRoles,
			}, decoded["requestBody"])
		})

		t.Run("minimax frame pair requires adaptive", func(t *testing.T) {
			imageRoles := []any{
				map[string]any{"url": "https://cdn.example/first.png", "role": "first_frame"},
				map[string]any{"url": "https://cdn.example/last.png", "role": "last_frame"},
			}
			decoded, err := supplyVideoDecode(t, plugin, "minimax-h3-max-768p-second", map[string]any{
				"prompt": "a lighthouse", "seconds": 5, "aspect_ratio": "adaptive", "image_with_roles": imageRoles,
			})
			require.NoError(t, err)
			assert.Equal(t, imageRoles, decoded["requestBody"].(map[string]any)["image_with_roles"])
		})

		t.Run("PAI per-second accepts string seconds and exposes billing ratios", func(t *testing.T) {
			decoded, err := supplyVideoDecode(t, plugin, "seedance2-mini-480p-second", map[string]any{
				"prompt": "a small boat", "seconds": "4",
			})
			require.NoError(t, err)
			assert.Equal(t, "text_to_video", decoded["action"])
			assert.Equal(t, map[string]any{
				"model": "seedance2-mini-480p-second", "prompt": "a small boat", "seconds": float64(4),
			}, decoded["requestBody"])
			ctx := map[string]any{"model": "seedance2-mini-480p-second", "requestBody": decoded["requestBody"]}
			assert.Equal(t, map[string]any{"output_seconds": float64(4), "input_seconds": float64(0)}, supplyVideoCall(t, plugin, "extractUsage", ctx))
			ctx["usagePurpose"] = "billing_ratios"
			assert.Equal(t, map[string]any{"seconds": float64(4)}, supplyVideoCall(t, plugin, "extractUsage", ctx))
			submit := supplyVideoCall(t, plugin, "buildSubmitRequest", map[string]any{
				"baseUrl": "https://supplier.example", "apiKey": "key",
				"model": "seedance2-mini-480p-second", "requestBody": decoded["requestBody"],
			})
			assert.Equal(t, map[string]any{
				"model": "seedance2-mini-480p-second", "prompt": "a small boat", "seconds": "4", "duration": float64(4),
			}, submit["body"])
		})

		t.Run("PAI per-call returns no usage facts", func(t *testing.T) {
			decoded, err := supplyVideoDecode(t, plugin, "seedance2-fast-720p-once-1", map[string]any{
				"prompt": "a rocket", "seconds": 4, "aspect_ratio": "16:9",
			})
			require.NoError(t, err)
			assert.Equal(t, map[string]any{
				"model": "seedance2-fast-720p-once-1", "prompt": "a rocket", "seconds": float64(4), "aspect_ratio": "16:9",
			}, decoded["requestBody"])
			assert.Empty(t, supplyVideoCall(t, plugin, "extractUsage", map[string]any{
				"model": "seedance2-fast-720p-once-1", "requestBody": decoded["requestBody"],
			}))
		})

		t.Run("PAI unconfirmed duration range accepts an integer", func(t *testing.T) {
			decoded, err := supplyVideoDecode(t, plugin, "seedance2-720p-once-2", map[string]any{
				"prompt": "a rocket", "seconds": 4,
			})
			require.NoError(t, err)
			assert.Equal(t, float64(4), decoded["requestBody"].(map[string]any)["seconds"])
		})
	})

	for _, tc := range []struct {
		name    string
		model   string
		body    map[string]any
		wantErr string
	}{
		{"happyhorse rejects reference video", "happyhorse1.1-1080p-second", map[string]any{"prompt": "a horse", "seconds": 5, "video_with_roles": supplyVideoRoles("reference_video", 1)}, "video_with_roles supports at most 0 items"},
		{"happyhorse rejects n", "happyhorse1.1-720p-second", map[string]any{"prompt": "a horse", "seconds": 5, "n": 1}, "field n is not supported"},
		{"happyhorse frame cannot mix reference images", "happyhorse1.1-720p-second", map[string]any{"prompt": "a horse", "seconds": 5, "image_with_roles": []any{
			map[string]any{"url": "https://cdn.example/first.png", "role": "first_frame"},
			map[string]any{"url": "https://cdn.example/reference.png", "role": "reference_image"},
		}}, "frame pair cannot be combined with reference_image"},
		{"grok seconds lower bound", "grok-video1.5-480p-second", map[string]any{"prompt": "a rocket", "seconds": 0}, "seconds must be an integer between 1 and 15"},
		{"grok seconds upper bound", "grok-video1.5-720p-second", map[string]any{"prompt": "a rocket", "seconds": 16}, "seconds must be an integer between 1 and 15"},
		{"wan frame cannot mix reference video", "wan3.0-video-480p", map[string]any{"prompt": "a river", "seconds": 8, "image_with_roles": supplyVideoRoles("first_frame", 1), "video_with_roles": supplyVideoRoles("reference_video", 1)}, "frame pair cannot be combined with reference_video"},
		{"wan audio requires media", "wan3.0-video-1080p", map[string]any{"prompt": "a river", "seconds": 8, "audio_with_roles": supplyVideoRoles("reference_audio", 1)}, "audio_with_roles requires image or video material"},
		{"kling with-video requires a video", "kling4-1080p-second-with-video", map[string]any{"prompt": "a dancer", "seconds": 10}, "with-video models require at least one reference_video"},
		{"kling no-video rejects a video", "kling4-720p-second-no-video", map[string]any{"prompt": "a dancer", "seconds": 10, "video_with_roles": supplyVideoRoles("reference_video", 1)}, "this model does not accept reference_video"},
		{"minimax text-only adaptive is rejected", "minimax-h3-max-480p-second", map[string]any{"prompt": "a lighthouse", "seconds": 5, "aspect_ratio": "adaptive"}, "text-only generation does not support aspect_ratio adaptive"},
		{"minimax frames require adaptive", "minimax-h3-max-1080p-second", map[string]any{"prompt": "a lighthouse", "seconds": 5, "image_with_roles": []any{
			map[string]any{"url": "https://cdn.example/first.png", "role": "first_frame"},
			map[string]any{"url": "https://cdn.example/last.png", "role": "last_frame"},
		}}, "frame inputs require aspect_ratio adaptive"},
		{"minimax total material cap", "minimax-h3-max-480p-second", map[string]any{"prompt": "a lighthouse", "seconds": 5, "image_with_roles": supplyVideoRoles("reference_image", 9), "video_with_roles": supplyVideoRoles("reference_video", 3), "audio_with_roles": supplyVideoRoles("reference_audio", 3)}, "reference materials must not exceed 12 items"},
		{"PAI rejects multimodal materials", "seedance2-mini-480p-second", map[string]any{"prompt": "a boat", "seconds": 4, "image_with_roles": supplyVideoRoles("reference_image", 1)}, "PAI multimodal materials are not supported yet"},
		{"PAI rejects n", "seedance2-mini-480p-second", map[string]any{"prompt": "a boat", "seconds": 4, "n": 1}, "field n is not supported"},
		{"PAI rejects locked resolution", "seedance2-mini-480p-second", map[string]any{"prompt": "a boat", "seconds": 4, "resolution": "480p"}, "resolution must not be sent"},
		{"PAI per-call range lower bound", "seedance2-fast-720p-once-1", map[string]any{"prompt": "a rocket", "seconds": 3}, "seconds must be an integer between 4 and 15"},
		{"PAI per-second range upper bound", "seedance2-mini-480p-second", map[string]any{"prompt": "a boat", "seconds": 16}, "seconds must be an integer between 4 and 15"},
		{"PAI unconfirmed range sanity bound", "seedance2-fast-480p-second-2", map[string]any{"prompt": "a boat", "seconds": 61}, "seconds must be an integer between 1 and 60"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := supplyVideoDecode(t, plugin, tc.model, tc.body)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
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
			{"status": "completed", "usage": map[string]any{"output_seconds": 0, "input_seconds": 3}},
		} {
			facts := supplyVideoCall(t, plugin, "extractUsageOnComplete", nil, map[string]any{"status": "SUCCESS"}, body)
			assert.Empty(t, facts)
		}
	})

	t.Run("completed per-second response maps output and input seconds", func(t *testing.T) {
		body := map[string]any{
			"status": "completed",
			"usage":  map[string]any{"charge_type": "per_second", "output_seconds": 5, "input_seconds": 10},
		}
		facts := supplyVideoCall(t, plugin, "extractUsageOnComplete", nil, map[string]any{"status": "SUCCESS"}, body)
		assert.Equal(t, map[string]any{"output_seconds": float64(5), "input_seconds": float64(10)}, facts)

		body = map[string]any{
			"status": "completed",
			"usage":  map[string]any{"charge_type": "per_second", "output_seconds": 8},
		}
		facts = supplyVideoCall(t, plugin, "extractUsageOnComplete", nil, map[string]any{"status": "SUCCESS"}, body)
		assert.Equal(t, map[string]any{"output_seconds": float64(8)}, facts)
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

	t.Run("openai_video renderer publishes measured seconds without upstream identity", func(t *testing.T) {
		value, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "render"}, map[string]any{}, map[string]any{
			"task_id": "public-second-task", "status": "SUCCESS", "progress": "100%", "created_at": 11,
			"data": map[string]any{
				"id": "upstream-second-secret", "status": "completed",
				"usage": map[string]any{"charge_type": "per_second", "output_seconds": 5, "input_seconds": 2},
			},
		})
		require.NoError(t, err)
		encoded, err := common.Marshal(value)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "upstream-second-secret")
		var response map[string]any
		require.NoError(t, common.Unmarshal(encoded, &response))
		assert.Equal(t, map[string]any{"output_seconds": float64(5), "input_seconds": float64(2)}, response["usage"])
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

	t.Run("content prefers the stored result URL without leaking supplier credentials", func(t *testing.T) {
		content := supplyVideoCall(t, plugin, "buildContentRequest", map[string]any{
			"baseUrl": vendorBaseURL, "apiKey": apiKey, "artifactKey": "video", "upstreamTaskId": "actual-upstream-id",
			"clientRequest": map[string]any{"method": http.MethodHead},
			"data":          map[string]any{"status": "completed", "video_url": "https://cdn.example/result.mp4?sig=abc"},
		})
		assert.Equal(t, "https://cdn.example/result.mp4?sig=abc", content["url"])
		assert.Equal(t, http.MethodHead, content["method"])
		headers := content["headers"].(map[string]any)
		assert.NotEmpty(t, headers["User-Agent"])
		assert.NotContains(t, headers, "Authorization")
	})
}
