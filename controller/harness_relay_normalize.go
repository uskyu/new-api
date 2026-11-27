package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

// harnessUpstreamErrorTypes are the error types produced after an upstream
// response was actually received. A local routing/selection error is
// ErrorTypeNewAPIError and must not be mistaken for an upstream auth failure.
func harnessUpstreamErrorTypes(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	switch err.GetErrorType() {
	case types.ErrorTypeOpenAIError, types.ErrorTypeClaudeError, types.ErrorTypeUpstreamError:
		return true
	default:
		return false
	}
}

// normalizeHarnessRelayError maps upstream auth failures away from the
// caller-authentication 401/403 class and keeps upstream model_not_found on the
// V2.3 503 wire status. It never changes an ordinary sk- client's response.
func normalizeHarnessRelayError(c *gin.Context, err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		return nil
	}
	if _, ok := middleware.GetHarnessPrincipal(c); !ok {
		return err
	}
	if harnessUpstreamErrorTypes(err) && (err.StatusCode == http.StatusUnauthorized || err.StatusCode == http.StatusForbidden) {
		// An upstream key failure is a gateway upstream problem, not a failed
		// Harness credential. The body keeps the upstream's native shape; only
		// the status class changes so the platform does not re-authenticate.
		err.StatusCode = http.StatusBadGateway
	}
	if err.GetErrorCode() == types.ErrorCodeModelNotFound && err.StatusCode != http.StatusServiceUnavailable {
		err.StatusCode = http.StatusServiceUnavailable
	}
	return err
}

// harnessUpstreamFailureCandidate reports whether this error is a real upstream
// failure that must win over a later "no candidate" marker. A upstream
// model_not_found is excluded because it describes a routing miss, not a
// provider outage.
func harnessUpstreamFailureCandidate(err *types.NewAPIError) bool {
	return harnessUpstreamErrorTypes(err) && err.GetErrorCode() != types.ErrorCodeModelNotFound
}

// chooseHarnessRelayFailure makes a real upstream failure survive a later
// no-candidate marker. It only applies to Harness callers; ordinary clients keep
// the historical last-error behavior.
func chooseHarnessRelayFailure(c *gin.Context, final, upstream *types.NewAPIError) *types.NewAPIError {
	if final == nil || upstream == nil {
		return final
	}
	if _, ok := middleware.GetHarnessPrincipal(c); !ok {
		return final
	}
	switch final.GetErrorCode() {
	case types.ErrorCodeModelNotFound, types.ErrorCodeGetChannelFailed:
		return upstream
	default:
		return final
	}
}

// setRelayRetryAfter copies a sanitized upstream Retry-After when present and
// falls back to a conservative 60 seconds. It is called before any 429 response
// is written, including streaming errors.
func setRelayRetryAfter(c *gin.Context) {
	if c == nil || c.Writer.Header().Get("Retry-After") != "" {
		return
	}
	if value := strings.TrimSpace(common.GetContextKeyString(c, constant.ContextKeyUpstreamRetryAfter)); validRetryAfter(value) {
		c.Header("Retry-After", value)
		return
	}
	c.Header("Retry-After", "60")
}

func validRetryAfter(value string) bool {
	if value == "" || len(value) > 256 || strings.ContainsAny(value, "\r\n") {
		return false
	}
	// RFC 9110 allows either delta-seconds or an HTTP-date. Anything else must
	// fall back to the gateway's conservative default rather than become an
	// invalid response header.
	if seconds, err := strconv.ParseUint(value, 10, 32); err == nil {
		return seconds <= 86400
	}
	_, err := http.ParseTime(value)
	return err == nil
}
