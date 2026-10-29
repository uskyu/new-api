package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

// newHarnessCorrelationEngine builds a router whose handler runs
// RequireHarnessCorrelation and then records that it was reached. The engine
// injects a verified Harness principal for harnessAuth, so the test drives both
// the Harness path and the untouched non-Harness path.
func newHarnessCorrelationEngine(harnessAuth bool) (*gin.Engine, *bool) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	reached := false
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		if harnessAuth {
			// Set before the middleware runs, the way HarnessJWT would.
			c.Set(HarnessPrincipalContextKey, HarnessPrincipal{
				Subject: "harness-gateway",
				Scopes:  []string{HarnessChatScope},
			})
		}
		c.Next()
	}, RequireHarnessCorrelation(), func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})
	return engine, &reached
}

func TestRequireHarnessCorrelationRejectsMissingOrMalformedIds(t *testing.T) {
	tests := []struct {
		name       string
		requestId  *string
		runId      *string
		wantStatus int
	}{
		{name: "both present", requestId: strPtr("hrq_01ABC"), runId: strPtr("run_01ABC"), wantStatus: http.StatusOK},
		{name: "request id missing", runId: strPtr("run_01ABC"), wantStatus: http.StatusBadRequest},
		{name: "run id missing", requestId: strPtr("hrq_01ABC"), wantStatus: http.StatusBadRequest},
		{name: "both missing", wantStatus: http.StatusBadRequest},
		{name: "both blank", requestId: strPtr(""), runId: strPtr(""), wantStatus: http.StatusBadRequest},
		{name: "request id with space", requestId: strPtr("abc def"), runId: strPtr("run_01ABC"), wantStatus: http.StatusBadRequest},
		{name: "request id with slash", requestId: strPtr("abc/def"), runId: strPtr("run_01ABC"), wantStatus: http.StatusBadRequest},
		{name: "run id with space", requestId: strPtr("hrq_01ABC"), runId: strPtr("run 01"), wantStatus: http.StatusBadRequest},
		{
			name:       "request id at 64 stays accepted",
			requestId:  strPtr(strings.Repeat("a", 64)),
			runId:      strPtr("run_01ABC"),
			wantStatus: http.StatusOK,
		},
		{
			name:       "request id at 65 is rejected",
			requestId:  strPtr(strings.Repeat("a", 65)),
			runId:      strPtr("run_01ABC"),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "run id at 64 stays accepted",
			requestId:  strPtr("hrq_01ABC"),
			runId:      strPtr(strings.Repeat("r", 64)),
			wantStatus: http.StatusOK,
		},
		{
			name:       "run id at 65 is rejected",
			requestId:  strPtr("hrq_01ABC"),
			runId:      strPtr(strings.Repeat("r", 65)),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine, reached := newHarnessCorrelationEngine(true)
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			if tc.requestId != nil {
				req.Header.Set(HarnessRequestIdHeader, *tc.requestId)
			}
			if tc.runId != nil {
				req.Header.Set(HarnessRunIdHeader, *tc.runId)
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)

			assert.Equal(t, tc.wantStatus, recorder.Code)
			// A rejected request must never reach dispatch: that is what makes
			// "not dispatched, not billed" verifiable.
			assert.Equal(t, tc.wantStatus == http.StatusOK, *reached,
				"handler must only be reached for an accepted request")
		})
	}
}

// The contract code travels in the error body, so the platform can branch on it
// instead of parsing the message.
func TestRequireHarnessCorrelationCarriesContractErrorCode(t *testing.T) {
	engine, reached := newHarnessCorrelationEngine(true)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.False(t, *reached)
	body := recorder.Body.String()
	assert.Contains(t, body, `"code":"REQUEST_INVALID"`)
	assert.Contains(t, body, HarnessRequestIdHeader)
}

// An ordinary sk- client never sets these headers and must keep working: the
// whole check is scoped to a verified Harness principal.
func TestRequireHarnessCorrelationLeavesNonHarnessCallersUntouched(t *testing.T) {
	engine, reached := newHarnessCorrelationEngine(false)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code, "a non-Harness request must not be rejected")
	assert.True(t, *reached)
}

func TestSanitizeHarnessCorrelationIdHonoursPerHeaderLimits(t *testing.T) {
	sixtyFour := strings.Repeat("r", 64)
	assert.Equal(t, sixtyFour, sanitizeHarnessCorrelationId(sixtyFour, harnessRunIdMaxLength),
		"a 64-character run id fits the ledger column and is kept")
	assert.Empty(t, sanitizeHarnessCorrelationId(sixtyFour+"r", harnessRunIdMaxLength),
		"a value one character over the column width is dropped rather than truncated")
	assert.Empty(t, sanitizeHarnessCorrelationId("abc def", harnessRunIdMaxLength))
	assert.Empty(t, sanitizeHarnessCorrelationId("", harnessRunIdMaxLength))
}

// TestHarnessCorrelationIdLimitsFitTheirColumns pins that every id this
// middleware accepts still fits the column that stores it.
//
// A longer value would pass the header check, the call would succeed and be
// billed, and the ledger insert would then fail on MySQL with "data too long".
// Ledger failures are logged rather than returned, so the platform would see a
// 200 for a request it can never reconcile — the outcome the required headers
// exist to prevent.
func TestHarnessCorrelationIdLimitsFitTheirColumns(t *testing.T) {
	for _, tc := range []struct {
		value any
		field string
		max   int
	}{
		{&model.HarnessLedgerEvent{}, "HarnessRequestId", harnessRequestIdMaxLength},
		{&model.HarnessLedgerEvent{}, "RunId", harnessRunIdMaxLength},
		{&model.HarnessSettlement{}, "HarnessRequestId", harnessRequestIdMaxLength},
		{&model.HarnessSettlement{}, "HarnessRunId", harnessRunIdMaxLength},
	} {
		parsed, err := schema.Parse(tc.value, &sync.Map{}, schema.NamingStrategy{})
		require.NoError(t, err)
		field := parsed.LookUpField(tc.field)
		require.NotNil(t, field, "%T has no field %s", tc.value, tc.field)
		var width int
		_, err = fmt.Sscanf(field.TagSettings["TYPE"], "varchar(%d)", &width)
		require.NoError(t, err, "%T.%s has unexpected column type %q",
			tc.value, tc.field, field.TagSettings["TYPE"])
		assert.GreaterOrEqual(t, width, tc.max,
			"%T.%s stores %d characters but the middleware accepts %d",
			tc.value, tc.field, width, tc.max)
	}
}

func strPtr(value string) *string { return &value }
