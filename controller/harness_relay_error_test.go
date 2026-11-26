package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newHarnessRelayErrorContext(t *testing.T, requestID string, harness bool) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	if requestID != "" {
		c.Set(common.RequestIdKey, requestID)
	}
	if harness {
		c.Set(middleware.HarnessPrincipalContextKey, middleware.HarnessPrincipal{Subject: "harness-gateway"})
	}
	return c, recorder
}

func TestHarnessUpstreamErrorCodeMapsTheLastAttempt(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		attempts  []harnessUpstreamAttempt
		wantCode  string
		retryable bool
	}{
		{
			name:      "upstream 401 is an auth error",
			attempts:  []harnessUpstreamAttempt{{ChannelId: 1, Status: http.StatusUnauthorized, Code: "bad_response_status_code"}},
			wantCode:  harnessCodeUpstreamAuthError,
			retryable: false,
		},
		{
			name:      "upstream 403 is an auth error",
			attempts:  []harnessUpstreamAttempt{{ChannelId: 1, Status: http.StatusForbidden, Code: "invalid_api_key"}},
			wantCode:  harnessCodeUpstreamAuthError,
			retryable: false,
		},
		{
			name:      "upstream 429 is rate limited",
			attempts:  []harnessUpstreamAttempt{{ChannelId: 1, Status: http.StatusTooManyRequests, Code: "rate_limit_exceeded"}},
			wantCode:  harnessCodeUpstreamRateLimited,
			retryable: true,
		},
		{
			name:      "a non-JSON body is an invalid response",
			attempts:  []harnessUpstreamAttempt{{ChannelId: 1, Status: http.StatusBadGateway, Code: string(types.ErrorCodeBadResponseBody)}},
			wantCode:  harnessCodeUpstreamInvalidResponse,
			retryable: true,
		},
		{
			name:      "any other upstream failure is unavailable",
			attempts:  []harnessUpstreamAttempt{{ChannelId: 1, Status: http.StatusInternalServerError, Code: string(types.ErrorCodeBadResponseStatusCode)}},
			wantCode:  harnessCodeUpstreamUnavailable,
			retryable: true,
		},
		{
			name:      "an empty trail is still unavailable",
			attempts:  nil,
			wantCode:  harnessCodeUpstreamUnavailable,
			retryable: true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			code, retryable := harnessUpstreamErrorCode(testCase.attempts)
			assert.Equal(t, testCase.wantCode, code)
			assert.Equal(t, testCase.retryable, retryable)
		})
	}
}

// TestWriteHarnessRelayErrorCarriesAttemptsAndRoutingFacts pins the wire shape a
// Harness caller receives when every candidate failed: 503, upstream_unavailable
// (or the mapped code), retryable, and the full masked attempt trail.
func TestWriteHarnessRelayErrorCarriesAttemptsAndRoutingFacts(t *testing.T) {
	c, recorder := newHarnessRelayErrorContext(t, "req_text_1", true)
	attempts := []harnessUpstreamAttempt{
		{ChannelId: 11, Status: http.StatusInternalServerError, Code: string(types.ErrorCodeBadResponseStatusCode), Message: "bad response status code 500"},
		{ChannelId: 12, Status: http.StatusInternalServerError, Code: string(types.ErrorCodeBadResponseStatusCode), Message: "bad response status code 502"},
	}
	body := buildHarnessUpstreamErrorBody(c, "req_text_1", "video-model", "pool-a", attempts,
		string(types.ErrorCodeGetChannelFailed), "no available channel")

	writeHarnessRelayError(c, types.RelayFormatOpenAI, body)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var payload struct {
		Error struct {
			Message   string `json:"message"`
			Type      string `json:"type"`
			Code      string `json:"code"`
			Retryable bool   `json:"retryable"`
			RequestId string `json:"request_id"`
			Details   struct {
				Model      string                   `json:"model"`
				RouteGroup string                   `json:"route_group"`
				RequestId  string                   `json:"request_id"`
				Attempts   []harnessUpstreamAttempt `json:"attempts"`
			} `json:"details"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, harnessCodeUpstreamUnavailable, payload.Error.Code)
	assert.True(t, payload.Error.Retryable)
	assert.Equal(t, "req_text_1", payload.Error.RequestId)
	assert.Equal(t, "video-model", payload.Error.Details.Model)
	assert.Equal(t, "pool-a", payload.Error.Details.RouteGroup)
	assert.Equal(t, "req_text_1", payload.Error.Details.RequestId)
	require.Len(t, payload.Error.Details.Attempts, 2)
	assert.Equal(t, 11, payload.Error.Details.Attempts[0].ChannelId)
	assert.Equal(t, http.StatusInternalServerError, payload.Error.Details.Attempts[0].Status)
	assert.Equal(t, "bad response status code 502", payload.Error.Message,
		"when the loop ran out of candidates the last real upstream failure supplies the message")
}

func TestHarnessTaskSubmissionFailureReturnsStructuredStatusWithAttempts(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		attempt    harnessUpstreamAttempt
		wantCode   string
		retryable  bool
		wantStatus int
	}{
		{
			name:       "all candidates failed",
			attempt:    harnessUpstreamAttempt{ChannelId: 21, Status: http.StatusInternalServerError, Code: "fail_to_fetch_task", Message: "upstream returned 500"},
			wantCode:   harnessCodeUpstreamUnavailable,
			retryable:  true,
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "upstream rejected the key",
			attempt:    harnessUpstreamAttempt{ChannelId: 21, Status: http.StatusUnauthorized, Code: "fail_to_fetch_task", Message: "upstream returned 401"},
			wantCode:   harnessCodeUpstreamAuthError,
			retryable:  false,
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "upstream rate limited",
			attempt:    harnessUpstreamAttempt{ChannelId: 21, Status: http.StatusTooManyRequests, Code: "fail_to_fetch_task", Message: "upstream returned 429"},
			wantCode:   harnessCodeUpstreamRateLimited,
			retryable:  true,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "upstream returned an invalid body",
			attempt:    harnessUpstreamAttempt{ChannelId: 21, Status: http.StatusBadGateway, Code: string(types.ErrorCodeBadResponseBody), Message: "upstream returned a non-JSON body"},
			wantCode:   harnessCodeUpstreamInvalidResponse,
			retryable:  true,
			wantStatus: http.StatusBadGateway,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			c, recorder := newHarnessRelayErrorContext(t, "req_task_1", true)
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
			c.Set("original_model", "video-model")
			recordHarnessUpstreamAttempt(c, testCase.attempt.ChannelId, testCase.attempt.Status, testCase.attempt.Code, testCase.attempt.Message)
			respondTaskSubmissionError(c, &taskdto.TaskError{
				Code:       "fail_to_fetch_task",
				Message:    "upstream returned " + http.StatusText(testCase.attempt.Status),
				StatusCode: testCase.attempt.Status,
			})

			require.Equal(t, testCase.wantStatus, recorder.Code)
			var payload struct {
				Error struct {
					Code      string `json:"code"`
					Retryable bool   `json:"retryable"`
					RequestId string `json:"request_id"`
					Details   struct {
						Model      string                   `json:"model"`
						RouteGroup string                   `json:"route_group"`
						Attempts   []harnessUpstreamAttempt `json:"attempts"`
					} `json:"details"`
				} `json:"error"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
			assert.Equal(t, testCase.wantCode, payload.Error.Code)
			assert.Equal(t, testCase.retryable, payload.Error.Retryable)
			assert.Equal(t, "req_task_1", payload.Error.RequestId)
			assert.Equal(t, "video-model", payload.Error.Details.Model)
			assert.Equal(t, "default", payload.Error.Details.RouteGroup)
			require.Len(t, payload.Error.Details.Attempts, 1)
			assert.Equal(t, testCase.attempt.ChannelId, payload.Error.Details.Attempts[0].ChannelId)
			assert.Equal(t, testCase.attempt.Status, payload.Error.Details.Attempts[0].Status)
		})
	}
}

// TestHarnessRetryLimitUsesPoolCandidateCount wires the budget to the model
// helper: a pool with four enabled candidates offsets a zero RetryTimes setting,
// while an ordinary caller and a Harness caller without a pool keep the
// configured policy exactly.
func TestHarnessRetryLimitUsesPoolCandidateCount(t *testing.T) {
	oldDB := model.DB
	oldConfig := setting.GetIntelligentRoutingConfig()
	oldRetryTimes := common.RetryTimes
	db, err := gorm.Open(sqlite.Open("file:harness-retry-limit-test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	common.RetryTimes = 0
	t.Cleanup(func() {
		model.DB = oldDB
		common.RetryTimes = oldRetryTimes
		require.NoError(t, setting.UpdateIntelligentRoutingConfig(oldConfig))
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))

	order := make([]int, 0, 4)
	for range 4 {
		channel := model.Channel{
			Type: constant.ChannelTypeOpenAI, Key: "harness-retry-key",
			Name: "retry-channel", Models: "model-x", Group: "default",
			Status: common.ChannelStatusEnabled,
		}
		require.NoError(t, db.Create(&channel).Error)
		require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "model-x", ChannelId: channel.Id, Enabled: true}).Error)
		order = append(order, channel.Id)
	}
	require.NoError(t, setting.UpdateIntelligentRoutingConfig(setting.IntelligentRoutingConfig{
		Groups: []setting.IntelligentRoutingGroup{{ID: "pool-r", Name: "Pool R", ChannelOrder: order}},
	}))

	c, _ := newHarnessRelayErrorContext(t, "req_limit_1", true)
	common.SetContextKey(c, setting.IntelligentRoutingContextKey, "pool-r")
	assert.Equal(t, 3, harnessRetryLimit(c, "model-x"),
		"four candidates need retry indexes 0..3 even when the configured retry count is zero")

	plain, _ := newHarnessRelayErrorContext(t, "req_limit_plain", false)
	common.SetContextKey(plain, setting.IntelligentRoutingContextKey, "pool-r")
	assert.Equal(t, 0, harnessRetryLimit(plain, "model-x"),
		"an ordinary client keeps the configured retry policy")

	harnessWithoutPool, _ := newHarnessRelayErrorContext(t, "req_limit_nopool", true)
	assert.Equal(t, 0, harnessRetryLimit(harnessWithoutPool, "model-x"),
		"without a pool the candidate count is unknown and the configured policy stands")
}
