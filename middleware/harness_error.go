package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

// HarnessErrorBody is the error object a verified Harness caller receives for a
// model-call failure. It deliberately keeps the OpenAI-compatible message/code
// fields the platform already reads and adds the machine-readable routing facts
// it needs to tell an unpriced/unroutable model from an upstream outage.
//
// The read APIs use service.HarnessReadError for the same envelope on their own
// routes; model calls go through this body so both surfaces agree on
// code/retryable/request_id/details without changing the relay error struct that
// ordinary sk- clients depend on.
type HarnessErrorBody struct {
	Message   string         `json:"message"`
	Type      string         `json:"type"`
	Code      string         `json:"code"`
	Retryable bool           `json:"retryable"`
	RequestId string         `json:"request_id,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

// HarnessErrorEnvelope keeps the error object at the top level. Harness already
// parses body.error for ordinary relay errors, and the read contract uses the
// same nesting, so this shape does not break an existing parser.
type HarnessErrorEnvelope struct {
	Error HarnessErrorBody `json:"error"`
}

// HarnessRequestID returns this gateway's request id from the context the
// RequestId middleware fills, falling back to the response header it set.
func HarnessRequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if id := c.GetString(common.RequestIdKey); id != "" {
		return id
	}
	if c.Request != nil {
		if id := c.Request.Header.Get(common.RequestIdKey); id != "" {
			return id
		}
	}
	if c.Writer != nil {
		return c.Writer.Header().Get(common.RequestIdKey)
	}
	return ""
}

// HarnessRouteGroupID reports which route group served the call: the selected
// resource pool for a Harness request, otherwise the token group. The pool is
// what the catalog advertised, so an unpriced or unavailable answer is easier to
// reconcile against it than against an internal cost group.
func HarnessRouteGroupID(c *gin.Context, fallback string) string {
	if c == nil {
		return fallback
	}
	if pool := common.GetContextKeyString(c, setting.IntelligentRoutingContextKey); pool != "" {
		return pool
	}
	return fallback
}

// WriteHarnessError renders one structured Harness error. It is only called on
// paths that have already verified the Harness principal; ordinary relay errors
// keep using abortWithOpenAiMessage.
func WriteHarnessError(c *gin.Context, status int, body HarnessErrorBody) {
	if c == nil || c.Writer == nil {
		return
	}
	c.JSON(status, HarnessErrorEnvelope{Error: body})
	c.Abort()
	if c.Request != nil {
		logger.LogError(c.Request.Context(), common.LogText("harness error: status=%d code=%q message=%q", status, body.Code, body.Message))
	}
}

// abortWithHarnessModelNotFound answers a Harness call whose model has no
// enabled channel in the requested pool. A model that is retired, not sold in
// this pool, or whose channels are all disabled is a permanent routing failure,
// so it is a 404 with retryable=false rather than the legacy 503: retrying the
// same call cannot succeed until an operator changes the catalog.
func abortWithHarnessModelNotFound(c *gin.Context, modelName string, displayGroup string, routeGroup string) {
	requestID := HarnessRequestID(c)
	body := HarnessErrorBody{
		Message:   noAvailableChannelMessage(c, displayGroup, modelName),
		Type:      "new_api_error",
		Code:      string(types.ErrorCodeModelNotFound),
		Retryable: false,
		RequestId: requestID,
		Details: map[string]any{
			"model":       modelName,
			"route_group": routeGroup,
			"request_id":  requestID,
		},
	}
	WriteHarnessError(c, http.StatusNotFound, body)
}
