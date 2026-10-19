package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAuthorizedRoutingHeaderTest(t *testing.T) {
	t.Helper()
	previous := setting.GetIntelligentRoutingConfig()
	if previous.GlobalChannelOrder == nil {
		previous.GlobalChannelOrder = []int{}
	}
	if previous.TokenGroupAccess == nil {
		previous.TokenGroupAccess = map[int][]string{}
	}
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateIntelligentRoutingConfig(previous))
	})
	config := setting.IntelligentRoutingConfig{
		GlobalChannelOrder: []int{1},
		Groups: []setting.IntelligentRoutingGroup{
			{ID: "pool", Name: "Pool", ChannelOrder: []int{1}},
		},
		TokenGroupAccess: map[int][]string{77: {"global"}},
	}
	require.NoError(t, setting.UpdateIntelligentRoutingConfig(config))
}

func TestAuthorizedRoutingHeaderRequiredOnlyForInference(t *testing.T) {
	setupAuthorizedRoutingHeaderTest(t)
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name, method, path, routeTag, group string
		tokenID int
		forbidden bool
	}{
		{name: "chat without header", method: http.MethodPost, path: "/v1/chat/completions", routeTag: "relay", tokenID: 77, forbidden: true},
		{name: "Claude with whitespace header", method: http.MethodPost, path: "/v1/messages", routeTag: "relay", group: "   ", tokenID: 77, forbidden: true},
		{name: "image generation", method: http.MethodPost, path: "/v1/images/generations", routeTag: "relay", tokenID: 77, forbidden: true},
		{name: "task submission", method: http.MethodPost, path: "/v1/tasks/video", routeTag: "relay", tokenID: 77, forbidden: true},
		{name: "Gemini generation", method: http.MethodPost, path: "/v1beta/models/gemini:generateContent", routeTag: "relay", tokenID: 77, forbidden: true},
		{name: "WebSocket responses handshake", method: http.MethodGet, path: "/v1/responses", routeTag: "relay", tokenID: 77, forbidden: true},
		{name: "WebSocket realtime handshake", method: http.MethodGet, path: "/v1/realtime", routeTag: "relay", tokenID: 77, forbidden: true},
		{name: "authorized group header present", method: http.MethodPost, path: "/v1/chat/completions", routeTag: "relay", tokenID: 77, group: "global"},
		{name: "regular token", method: http.MethodPost, path: "/v1/chat/completions", routeTag: "relay", tokenID: 78},
		{name: "list models", method: http.MethodGet, path: "/v1/models", routeTag: "relay", tokenID: 77},
		{name: "get model", method: http.MethodGet, path: "/v1/models/example", routeTag: "relay", tokenID: 77},
		{name: "Gemini model listing", method: http.MethodGet, path: "/v1beta/models", routeTag: "relay", tokenID: 77},
		{name: "task lookup", method: http.MethodGet, path: "/v1/tasks/123", routeTag: "relay", tokenID: 77},
		{name: "task artifacts", method: http.MethodGet, path: "/v1/tasks/123/artifacts", routeTag: "relay", tokenID: 77},
		{name: "video lookup", method: http.MethodGet, path: "/v1/video/generations/123", routeTag: "relay", tokenID: 77},
		{name: "Harness catalog", method: http.MethodGet, path: "/api/harness/catalog", routeTag: "api", tokenID: 77},
		{name: "CORS preflight", method: http.MethodOptions, path: "/v1/chat/completions", routeTag: "relay", tokenID: 77},
		{name: "non-relay API", method: http.MethodPost, path: "/api/example", routeTag: "api", tokenID: 77},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(tt.method, tt.path, nil)
			ctx.Set(RouteTagKey, tt.routeTag)
			if tt.group != "" {
				ctx.Request.Header.Set(setting.IntelligentRoutingGroupHeader, tt.group)
			}
			allowed := enforceAuthorizedRoutingGroupHeader(ctx, tt.tokenID, tt.group)
			assert.Equal(t, !tt.forbidden, allowed)
			if tt.forbidden {
				assert.Equal(t, http.StatusForbidden, recorder.Code)
				assert.True(t, ctx.IsAborted())
				assert.Contains(t, recorder.Body.String(), "X-Route-Group")
			} else {
				assert.False(t, ctx.IsAborted())
			}
		})
	}
}

func TestAuthorizedRoutingHeaderStillChecksPoolPermissions(t *testing.T) {
	setupAuthorizedRoutingHeaderTest(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	token := &model.Token{Id: 77}
	require.NoError(t, applyIntelligentRoutingGroup(ctx, token, "default", "global"))
	assert.Equal(t, "global", ctx.GetString(setting.IntelligentRoutingContextKey))
	require.ErrorContains(t, applyIntelligentRoutingGroup(ctx, token, "default", "pool"), "not authorized")
	require.ErrorContains(t, applyIntelligentRoutingGroup(ctx, token, "default", "unknown"), "not configured")
}
