package middleware

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	// HarnessRequestIdHeader carries the Harness-side request id (hrq_<ULID>).
	HarnessRequestIdHeader = "X-Harness-Request-Id"
	// HarnessRunIdHeader carries the Harness run id (run_<ULID>).
	HarnessRunIdHeader = "X-Harness-Run-Id"
	// NewAPIRequestIdHeader returns this gateway's own request id.
	NewAPIRequestIdHeader = "X-NewAPI-Request-Id"

	HarnessRequestIdContextKey = "harness_request_id"
	HarnessRunIdContextKey     = "harness_run_id"

	// harnessRequestIdMaxLength and harnessRunIdMaxLength bound the two
	// correlation ids independently: the contract caps the request id at 64 and
	// the run id at 80, so a run id longer than 64 is still accepted.
	harnessRequestIdMaxLength = 64
	harnessRunIdMaxLength     = 80
)

// RequireHarnessCorrelation rejects a Harness model call that carries no usable
// correlation ids.
//
// The contract makes both ids mandatory for a metered call, and the ledger is
// keyed on them: a call missing an id would still be billed but could never be
// reconciled, so it is refused before dispatch. The check is limited to a
// verified Harness principal — an ordinary sk- client is never inspected and
// never rejected here, which keeps the existing API contract for every
// non-Harness caller intact.
func RequireHarnessCorrelation() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, verified := GetHarnessPrincipal(c); !verified {
			c.Next()
			return
		}
		if message, ok := validateHarnessCorrelationId(c, HarnessRequestIdHeader, harnessRequestIdMaxLength); !ok {
			abortWithOpenAiMessage(c, http.StatusBadRequest, message, harnessRequestInvalidCode)
			return
		}
		if message, ok := validateHarnessCorrelationId(c, HarnessRunIdHeader, harnessRunIdMaxLength); !ok {
			abortWithOpenAiMessage(c, http.StatusBadRequest, message, harnessRequestInvalidCode)
			return
		}
		c.Next()
	}
}

// harnessRequestInvalidCode is the contract's error code for a malformed or
// missing correlation id. It is the same code the read APIs return, so the
// platform can branch on one value across both surfaces.
var harnessRequestInvalidCode = types.ErrorCode(service.HarnessErrCodeRequestInvalid)

// validateHarnessCorrelationId reports whether the named header carries an
// acceptable id, and otherwise returns the message to send back. The message
// names the offending header so the platform can fix its caller; the gateway
// never logs the request body for this.
func validateHarnessCorrelationId(c *gin.Context, header string, maxLength int) (string, bool) {
	value := c.GetHeader(header)
	if value == "" {
		return header + " is required", false
	}
	if !harnessCorrelationIdIsWellFormed(value, maxLength) {
		return header + " must be 1 to " + strconv.Itoa(maxLength) + " characters of A-Z, a-z, 0-9, '-' or '_'", false
	}
	return "", true
}

// harnessCorrelationIdIsWellFormed enforces the accepted charset and bound. It is
// the same rule sanitizeHarnessCorrelationId applies when storing the ids, so a
// value accepted here is always one the ledger can correlate.
func harnessCorrelationIdIsWellFormed(value string, maxLength int) bool {
	if value == "" || len(value) > maxLength {
		return false
	}
	for i := range len(value) {
		switch char := value[i]; {
		case char >= 'a' && char <= 'z',
			char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9',
			char == '-',
			char == '_':
		default:
			return false
		}
	}
	return true
}

func GetHarnessRequestId(c *gin.Context) string {
	return harnessCorrelationId(c, HarnessRequestIdContextKey)
}

func GetHarnessRunId(c *gin.Context) string {
	return harnessCorrelationId(c, HarnessRunIdContextKey)
}

func harnessCorrelationId(c *gin.Context, key string) string {
	if value := c.GetString(key); value != "" {
		return value
	}
	if c.Request == nil {
		return ""
	}
	value, _ := c.Request.Context().Value(key).(string)
	return value
}

// sanitizeHarnessCorrelationId accepts an inbound correlation id only when it
// is short and uses characters that stay safe in headers and logs. Anything
// else is treated as absent so unrelated clients are never rejected; the
// Harness-only RequireHarnessCorrelation middleware is what turns that absence
// into a contract error for the platform.
func sanitizeHarnessCorrelationId(value string, maxLength int) string {
	if !harnessCorrelationIdIsWellFormed(value, maxLength) {
		return ""
	}
	return value
}
