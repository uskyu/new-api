package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newHarnessRelayNormalizeTestContext builds a bare Gin context with a response
// recorder; these helpers only read the Harness principal, context keys and
// response headers, so no request routing or upstream fixture is needed.
func newHarnessRelayNormalizeTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

// setHarnessRelayPrincipal marks the context as a verified Harness caller, the
// same value shape the Harness JWT middleware stores.
func setHarnessRelayPrincipal(c *gin.Context) {
	c.Set(middleware.HarnessPrincipalContextKey, middleware.HarnessPrincipal{Subject: "harness-gateway"})
}

func TestNormalizeHarnessRelayError(t *testing.T) {
	upstreamUnauthorizedBody := types.OpenAIError{
		Message: "upstream rejected the channel key",
		Type:    "invalid_request_error",
		Code:    "invalid_api_key",
	}
	modelNotFoundBody := types.OpenAIError{
		Message: "upstream model not found",
		Type:    "invalid_request_error",
		Code:    string(types.ErrorCodeModelNotFound),
	}

	tests := []struct {
		name          string
		harness       bool
		build         func() *types.NewAPIError
		wantStatus    int
		wantType      types.ErrorType
		wantCode      types.ErrorCode
		wantMessage   string
		wantRelayBody *types.OpenAIError
	}{
		{
			name:    "harness upstream unauthorized becomes bad gateway",
			harness: true,
			build: func() *types.NewAPIError {
				return types.WithOpenAIError(upstreamUnauthorizedBody, http.StatusUnauthorized)
			},
			wantStatus:    http.StatusBadGateway,
			wantType:      types.ErrorTypeOpenAIError,
			wantCode:      "invalid_api_key",
			wantMessage:   upstreamUnauthorizedBody.Message,
			wantRelayBody: &upstreamUnauthorizedBody,
		},
		{
			name:    "harness local unauthorized stays unauthorized",
			harness: true,
			build: func() *types.NewAPIError {
				return types.NewErrorWithStatusCode(errors.New("invalid harness token"), types.ErrorCodeAccessDenied, http.StatusUnauthorized)
			},
			wantStatus:  http.StatusUnauthorized,
			wantType:    types.ErrorTypeNewAPIError,
			wantCode:    types.ErrorCodeAccessDenied,
			wantMessage: "invalid harness token",
		},
		{
			name:    "harness upstream model not found becomes service unavailable",
			harness: true,
			build: func() *types.NewAPIError {
				return types.WithOpenAIError(modelNotFoundBody, http.StatusNotFound)
			},
			wantStatus:    http.StatusServiceUnavailable,
			wantType:      types.ErrorTypeOpenAIError,
			wantCode:      types.ErrorCodeModelNotFound,
			wantMessage:   modelNotFoundBody.Message,
			wantRelayBody: &modelNotFoundBody,
		},
		{
			name:    "non harness upstream unauthorized is untouched",
			harness: false,
			build: func() *types.NewAPIError {
				return types.WithOpenAIError(upstreamUnauthorizedBody, http.StatusUnauthorized)
			},
			wantStatus:    http.StatusUnauthorized,
			wantType:      types.ErrorTypeOpenAIError,
			wantCode:      "invalid_api_key",
			wantMessage:   upstreamUnauthorizedBody.Message,
			wantRelayBody: &upstreamUnauthorizedBody,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newHarnessRelayNormalizeTestContext(t)
			if tc.harness {
				setHarnessRelayPrincipal(c)
			}
			got := normalizeHarnessRelayError(c, tc.build())
			require.NotNil(t, got)

			assert.Equal(t, tc.wantStatus, got.StatusCode)
			assert.Equal(t, tc.wantType, got.GetErrorType())
			assert.Equal(t, tc.wantCode, got.GetErrorCode())
			assert.Equal(t, tc.wantMessage, got.Error(), "error message must not be rewritten")
			if tc.wantRelayBody != nil {
				relayBody, ok := got.RelayError.(types.OpenAIError)
				require.True(t, ok, "relay error body must keep its upstream shape")
				assert.Equal(t, *tc.wantRelayBody, relayBody)
			}
		})
	}

	// A nil error is the only other entry point and must stay nil.
	assert.Nil(t, normalizeHarnessRelayError(newHarnessRelayNormalizeTestContext(t), nil))
}

func TestHarnessUpstreamFailureCandidate(t *testing.T) {
	tests := []struct {
		name string
		err  *types.NewAPIError
		want bool
	}{
		{
			name: "upstream service unavailable is a real failure",
			err: types.WithOpenAIError(types.OpenAIError{
				Message: "upstream service unavailable",
				Type:    "upstream_error",
				Code:    "upstream_error",
			}, http.StatusServiceUnavailable),
			want: true,
		},
		{
			name: "upstream model not found is a routing miss",
			err: types.WithOpenAIError(types.OpenAIError{
				Message: "upstream model not found",
				Type:    "invalid_request_error",
				Code:    string(types.ErrorCodeModelNotFound),
			}, http.StatusNotFound),
			want: false,
		},
		{
			name: "local error is not an upstream failure",
			err:  types.NewError(errors.New("no available channel"), types.ErrorCodeChannelNoAvailableKey),
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, harnessUpstreamFailureCandidate(tc.err))
		})
	}
}

func TestChooseHarnessRelayFailureKeepsRealUpstreamFailure(t *testing.T) {
	upstream := types.WithOpenAIError(types.OpenAIError{Message: "upstream unavailable"}, http.StatusServiceUnavailable)
	modelMiss := types.NewErrorWithStatusCode(errors.New("model not found"), types.ErrorCodeModelNotFound, http.StatusServiceUnavailable)
	channelMiss := types.NewError(errors.New("no channel"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())

	harness := newHarnessRelayNormalizeTestContext(t)
	harness.Set(middleware.HarnessPrincipalContextKey, middleware.HarnessPrincipal{Subject: "harness"})
	assert.Equal(t, upstream, chooseHarnessRelayFailure(harness, modelMiss, upstream),
		"a real upstream 503 must survive a later model_not_found marker")
	assert.Equal(t, upstream, chooseHarnessRelayFailure(harness, channelMiss, upstream),
		"a real upstream 503 must survive a later no-channel marker")
	assert.Equal(t, modelMiss, chooseHarnessRelayFailure(harness, modelMiss, nil),
		"without a real upstream failure the final routing marker stands")
	assert.Equal(t, upstream, chooseHarnessRelayFailure(harness, upstream, modelMiss),
		"a non-routing final error is not replaced")

	ordinary := newHarnessRelayNormalizeTestContext(t)
	assert.Equal(t, modelMiss, chooseHarnessRelayFailure(ordinary, modelMiss, upstream),
		"ordinary sk- clients keep the historical last-error behavior")
}

func TestSetRelayRetryAfter(t *testing.T) {
	tests := []struct {
		name              string
		contextValue      string
		hasContextValue   bool
		existingHeader    string
		wantHeader        string
		wantInjectionGone bool
	}{
		{
			name:            "uses sanitized upstream value",
			contextValue:    "37",
			hasContextValue: true,
			wantHeader:      "37",
		},
		{
			name:       "falls back without context value",
			wantHeader: "60",
		},
		{
			name:            "existing header is not overwritten",
			contextValue:    "37",
			hasContextValue: true,
			existingHeader:  "42",
			wantHeader:      "42",
		},
		{
			name:              "CRLF value is rejected",
			contextValue:      "30\r\nX-Injected: 1",
			hasContextValue:   true,
			wantHeader:        "60",
			wantInjectionGone: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newHarnessRelayNormalizeTestContext(t)
			if tc.hasContextValue {
				common.SetContextKey(c, constant.ContextKeyUpstreamRetryAfter, tc.contextValue)
			}
			if tc.existingHeader != "" {
				c.Writer.Header().Set("Retry-After", tc.existingHeader)
			}

			setRelayRetryAfter(c)

			assert.Equal(t, tc.wantHeader, c.Writer.Header().Get("Retry-After"))
			if tc.wantInjectionGone {
				assert.Empty(t, c.Writer.Header().Get("X-Injected"))
			}
		})
	}
}
