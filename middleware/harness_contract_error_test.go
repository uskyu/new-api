package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// harnessContractTestBody is the decoded error envelope. It mirrors the wire
// shape rather than reusing HarnessContractErrorBody so a renamed or dropped
// JSON field would fail the test instead of silently following the struct.
type harnessContractTestBody struct {
	Error struct {
		Code      string         `json:"code"`
		Message   string         `json:"message"`
		Retryable *bool          `json:"retryable"`
		RequestId string         `json:"request_id"`
		Details   map[string]any `json:"details"`
	} `json:"error"`
}

func decodeHarnessContractTestBody(t *testing.T, recorder *httptest.ResponseRecorder) harnessContractTestBody {
	t.Helper()
	var body harnessContractTestBody
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

// TestWriteHarnessContractErrorFallsBackToGeneratedRequestId pins the degraded
// path: a route that forgot RequestId() still answers with a non-empty,
// per-request correlation id, and the same id is echoed in both headers.
func TestWriteHarnessContractErrorFallsBackToGeneratedRequestId(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/internal/v1/gateway/usage-events", func(c *gin.Context) {
		WriteHarnessContractError(c, http.StatusUnauthorized,
			"GATEWAY_UNAUTHORIZED", "a verified Harness credential is required", false,
			map[string]any{"required_scope": "gateway.usage.read"})
	})

	first := httptest.NewRecorder()
	engine.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/internal/v1/gateway/usage-events", nil))
	require.Equal(t, http.StatusUnauthorized, first.Code)

	body := decodeHarnessContractTestBody(t, first)
	assert.Equal(t, "GATEWAY_UNAUTHORIZED", body.Error.Code)
	assert.Equal(t, "a verified Harness credential is required", body.Error.Message)
	require.NotNil(t, body.Error.Retryable)
	assert.False(t, *body.Error.Retryable)
	require.NotEmpty(t, body.Error.RequestId, "the fallback must generate a request id")
	assert.Equal(t, "gateway.usage.read", body.Error.Details["required_scope"])
	assert.Equal(t, body.Error.RequestId, first.Header().Get(NewAPIRequestIdHeader))
	assert.Equal(t, body.Error.RequestId, first.Header().Get(common.RequestIdKey))

	second := httptest.NewRecorder()
	engine.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/internal/v1/gateway/usage-events", nil))
	secondBody := decodeHarnessContractTestBody(t, second)
	assert.NotEqual(t, body.Error.RequestId, secondBody.Error.RequestId,
		"a generated fallback id must be unique per request, not a reused constant")
}

// TestWriteHarnessContractErrorUsesRequestIdMiddleware pins the normal path: the
// id minted by RequestId() is the one carried in the body and in both headers,
// so a platform can correlate a contract error with the gateway log.
func TestWriteHarnessContractErrorUsesRequestIdMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestId())
	var middlewareRequestId string
	engine.GET("/internal/v1/gateway/cost-records", func(c *gin.Context) {
		middlewareRequestId = c.GetString(common.RequestIdKey)
		WriteHarnessContractError(c, http.StatusServiceUnavailable,
			"GATEWAY_UPSTREAM", "the gateway could not read its ledger", true, nil)
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/internal/v1/gateway/cost-records", nil))
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	body := decodeHarnessContractTestBody(t, recorder)
	require.NotEmpty(t, middlewareRequestId)
	assert.Equal(t, middlewareRequestId, body.Error.RequestId)
	assert.Equal(t, middlewareRequestId, recorder.Header().Get(NewAPIRequestIdHeader))
	assert.Equal(t, middlewareRequestId, recorder.Header().Get(common.RequestIdKey))
	assert.Equal(t, "GATEWAY_UPSTREAM", body.Error.Code)
	require.NotNil(t, body.Error.Retryable)
	assert.True(t, *body.Error.Retryable)
}

// TestNativeErrorsOutsideHarnessSurfacesKeepTheirShape is the counterpart of the
// contract tests: ordinary client and model-call errors must not grow
// retryable/request_id fields, and a Harness credential refused on a model route
// still answers the native OpenAI envelope.
func TestNativeErrorsOutsideHarnessSurfacesKeepTheirShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestId())
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		abortWithOpenAiMessage(c, http.StatusUnauthorized, "invalid token")
	})
	engine.POST("/v1/responses", func(c *gin.Context) {
		abortWithHarnessUnauthorized(c)
	})

	for _, tc := range []struct {
		name     string
		path     string
		wantCode string
	}{
		{name: "ordinary client error", path: "/v1/chat/completions", wantCode: ""},
		{name: "harness credential refused on a model route", path: "/v1/responses", wantCode: "GATEWAY_UNAUTHORIZED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, tc.path, nil))
			require.Equal(t, http.StatusUnauthorized, recorder.Code)

			var payload map[string]any
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
			errorObject, ok := payload["error"].(map[string]any)
			require.True(t, ok, "the native shape nests the error object")

			assert.Equal(t, "new_api_error", errorObject["type"])
			assert.Equal(t, tc.wantCode, errorObject["code"])
			assert.NotContains(t, errorObject, "retryable")
			assert.NotContains(t, errorObject, "request_id")
			assert.NotContains(t, payload, "request_id")
			assert.Len(t, errorObject, 3, "native errors keep exactly message/type/code")
			assert.Contains(t, errorObject["message"], "request id: ",
				"correlation stays in the message, not in a contract field")
		})
	}
}
