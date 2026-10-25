package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
