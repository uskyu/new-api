package router

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/plugins"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListModelsSupportsOpenAIAndGeminiAuthentication(t *testing.T) {
	setupRelayRouterTestDB(t)

	user := model.User{
		Username: "models-user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    100,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		UserId:         user.Id,
		Key:            "modelstestkey",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	engine := gin.New()
	SetRelayRouter(engine)

	tests := []struct {
		name           string
		path           string
		headerName     string
		expectedObject string
		expectedField  string
	}{
		{
			name:           "OpenAI bearer token",
			path:           "/v1/models",
			headerName:     "Authorization",
			expectedObject: "list",
			expectedField:  "data",
		},
		{
			name:          "Gemini API key header",
			path:          "/v1/models",
			headerName:    "x-goog-api-key",
			expectedField: "models",
		},
		{
			name:          "Gemini API key query",
			path:          "/v1/models?key=modelstestkey",
			expectedField: "models",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.headerName != "" {
				value := "modelstestkey"
				if test.headerName == "Authorization" {
					value = "Bearer " + value
				}
				request.Header.Set(test.headerName, value)
			}

			engine.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code)
			var payload map[string]any
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
			assert.Contains(t, payload, test.expectedField)
			assert.NotContains(t, payload, "error")
			if test.expectedObject != "" {
				assert.Equal(t, test.expectedObject, payload["object"])
			}
		})
	}
}

func TestGeminiRelayHandsLongRunningPredictionToPluginRoute(t *testing.T) {
	setupRelayRouterTestDB(t)
	require.NoError(t, i18n.Init())
	user := model.User{Username: "veo-user", Status: common.UserStatusEnabled, Group: "default", Quota: 100}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.Token{UserId: user.Id, Key: "veotestkey", Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}).Error)

	engine := gin.New()
	SetRelayRouter(engine)
	source, err := plugins.Source("google")
	require.NoError(t, err)
	registry := jsplugin.NewRegistry()
	_, err = registry.RegisterFactory(source, jsplugin.Options{Key: "google"})
	require.NoError(t, err)
	// The plugin route authenticates as in production, then names itself.
	handlers := func(generation *jsplugin.RoutingGeneration, group jsplugin.RouteGroup) []gin.HandlerFunc {
		production := productionPluginRouteHandlers(generation, group)
		return []gin.HandlerFunc{production[0], production[1], production[2], func(c *gin.Context) {
			c.String(http.StatusOK, "plugin %s for user %d", group.Bindings[0].Plugin.Meta.Key, c.GetInt("id"))
		}}
	}
	require.NoError(t, registry.SetGenerationPreparer(newPluginGenerationBuilder(engine.Routes(), nil, handlers).prepare))
	require.Empty(t, registry.RoutingErrors())
	previousRegistry := defaultPluginDispatcher.registry
	defaultPluginDispatcher.registry = registry
	t.Cleanup(func() { defaultPluginDispatcher.registry = previousRegistry })
	engine.NoRoute(defaultPluginDispatcher.dispatch)

	for _, testCase := range []struct {
		method   string
		path     string
		toPlugin bool
	}{
		{method: http.MethodPost, path: "/v1beta/models/veo-3.1-generate-preview:predictLongRunning", toPlugin: true},
		{method: http.MethodGet, path: "/v1beta/models/veo-3.1-generate-preview/operations/task_sample", toPlugin: true},
		{method: http.MethodPost, path: "/v1beta/models/gemini-2.5-flash:generateContent"},
	} {
		t.Run(testCase.path, func(t *testing.T) {
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(`{}`))
			request.Header.Set("x-goog-api-key", "veotestkey")
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)

			if testCase.toPlugin {
				assert.Equal(t, fmt.Sprintf("plugin google for user %d", user.Id), recorder.Body.String())
				return
			}
			// The relay looks for a Gemini channel serving the model; there is none.
			assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"code":"model_not_found"`)
		})
	}
}

func setupRelayRouterTestDB(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalIsMasterNode := common.IsMasterNode
	originalRedisEnabled := common.RedisEnabled
	originalSQLitePath := common.SQLitePath
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")

	common.IsMasterNode = false
	common.RedisEnabled = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	model.LOG_DB = model.DB
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.Token{}, &model.Ability{}))

	t.Cleanup(func() {
		if sqlDB, err := model.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		common.IsMasterNode = originalIsMasterNode
		common.RedisEnabled = originalRedisEnabled
		common.SQLitePath = originalSQLitePath
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	})
}

// TestRelayProtocolRoutesRequireAuthentication pins the route registration and the
// authentication boundary for the four model-call protocols the contract names:
// A1 OpenAI chat, A2 OpenAI Responses, A4 Anthropic Messages and A9 Gemini native.
//
// It deliberately stops at the auth boundary. What each handler does with a valid
// credential depends on a channel that supports that protocol, and asserting a
// protocol-level failure here would confuse "no such upstream" with "the gateway
// cannot route this format". The authenticated paths are covered against a real
// upstream in the acceptance report.
func TestRelayProtocolRoutesRequireAuthentication(t *testing.T) {
	setupRelayRouterTestDB(t)
	engine := gin.New()
	// SetRouter is the full entry point: it also mounts the task-plugin protocol
	// routes, and A2 /v1/responses lives there rather than in SetRelayRouter alone.
	SetRouter(engine, WebAssets{})

	for _, route := range []struct {
		name string
		path string
		body string
	}{
		{"A1 OpenAI chat", "/v1/chat/completions", `{"model":"tier","messages":[]}`},
		{"A2 OpenAI Responses", "/v1/responses", `{"model":"tier","input":"hi"}`},
		{"A4 Anthropic Messages", "/v1/messages", `{"model":"tier","max_tokens":1,"messages":[]}`},
		{"A9 Gemini native", "/v1beta/models/tier:generateContent", `{"contents":[]}`},
	} {
		t.Run(route.name+" rejects an unauthenticated caller", func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(recorder, request)

			// A missing credential must never reach a handler. The exact status is
			// 401 for the authenticated routes; a 404 would mean the route is absent
			// entirely, which is what this case is guarding against.
			assert.NotEqual(t, http.StatusNotFound, recorder.Code,
				"the route must be registered")
			assert.Equal(t, http.StatusUnauthorized, recorder.Code,
				"body=%s", recorder.Body.String())
		})
	}
}

// TestRelayProtocolRoutesRequireChatScope pins that every model-call and task
// protocol a verified Harness principal can reach also demands gateway.chat.
//
// These routes are registered across the /v1 group, the /v1beta group, and the
// task-plugin protocol router. The latter does not inherit /v1's scope check, so
// every operation there needs an explicit guard. A newly added route must fail
// here rather than silently accepting a Harness credential without chat access.
func TestRelayProtocolRoutesRequireChatScope(t *testing.T) {
	setupRelayRouterTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Option{}))
	model.InitOptionMap()

	user := &model.User{Username: "scope-router", Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, model.DB.Create(user).Error)
	serviceToken := &model.Token{
		Name: "scope-router-service", Key: "scope-router-key", UserId: user.Id,
		Status: common.TokenStatusEnabled, ExpiredTime: -1,
		UnlimitedQuota: true, Group: "default",
	}
	require.NoError(t, model.DB.Create(serviceToken).Error)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	require.NoError(t, model.UpdateOption(setting.HarnessJWKSURLOptionKey, harnessE2EJWKS(t, key, "kid-scope")))
	require.NoError(t, model.UpdateOption(setting.HarnessJWTIssuerOptionKey, "harness"))
	require.NoError(t, model.UpdateOption(setting.HarnessJWTAudienceOptionKey, "new-api"))
	subjectMap, err := json.Marshal(map[string]int{"harness-gateway": serviceToken.Id})
	require.NoError(t, err)
	require.NoError(t, model.UpdateOption(setting.HarnessSubjectTokenMapOptionKey, string(subjectMap)))
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM options WHERE key IN ?", []string{
			setting.HarnessJWKSURLOptionKey, setting.HarnessJWTIssuerOptionKey,
			setting.HarnessJWTAudienceOptionKey, setting.HarnessSubjectTokenMapOptionKey,
		})
	})

	engine := gin.New()
	SetRouter(engine, WebAssets{})

	// A credential with no chat scope. The ledger scopes are present so a refusal
	// here cannot be mistaken for a missing-token failure.
	withoutChat := harnessE2ESign(t, key, "kid-scope", "gateway.usage.read", "gateway.cost.read", "gateway.catalog.read")

	for _, route := range []struct {
		name string
		path string
		body string
	}{
		{"A1 OpenAI chat", "/v1/chat/completions", `{"model":"tier","messages":[]}`},
		{"A2 OpenAI Responses", "/v1/responses", `{"model":"tier","input":"hi"}`},
		{"A4 Anthropic Messages", "/v1/messages", `{"model":"tier","max_tokens":1,"messages":[]}`},
		{"A9 Gemini native", "/v1beta/models/tier:generateContent", `{"contents":[]}`},
		{"OpenAI Image generation", "/v1/images/generations", `{"model":"image-model","prompt":"test"}`},
		{"OpenAI Video generation", "/v1/videos", `{"model":"video-model","prompt":"test"}`},
	} {
		t.Run(route.name+" refuses a credential without gateway.chat", func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+withoutChat)
			engine.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusForbidden, recorder.Code,
				"a missing gateway.chat must be refused before channel selection, body=%s",
				recorder.Body.String())
		})
	}

	for _, route := range []struct {
		name   string
		method string
		path   string
	}{
		{"OpenAI Responses retrieval", http.MethodGet, "/v1/responses/response-1"},
		{"OpenAI Video retrieval", http.MethodGet, "/v1/videos/task-1"},
		{"OpenAI Video content", http.MethodGet, "/v1/videos/task-1/content"},
		{"OpenAI Video content HEAD", http.MethodHead, "/v1/videos/task-1/content"},
	} {
		t.Run(route.name+" refuses a credential without gateway.chat", func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(route.method, route.path, nil)
			request.Header.Set("Authorization", "Bearer "+withoutChat)
			engine.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusForbidden, recorder.Code,
				"a missing gateway.chat must be refused before task lookup, body=%s",
				recorder.Body.String())
		})
	}
}
