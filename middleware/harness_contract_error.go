package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// HarnessContractErrorBody is the error object used by the server-to-server
// Harness surfaces (B1-B4 and the /internal gateway namespace). The model-call
// surfaces deliberately keep their native OpenAI/Claude shape and expose the
// correlation through the X-NewAPI-Request-Id response header instead.
type HarnessContractErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	RequestId string         `json:"request_id"`
	Details   map[string]any `json:"details,omitempty"`
}

// HarnessRequestID returns the gateway request id for the current request and
// guarantees it is non-empty. The normal path is the globally mounted RequestId
// middleware; the fallback keeps direct handler tests and any future route that
// forgets the middleware from emitting an uncorrelatable error.
func HarnessRequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	id := c.GetString(common.RequestIdKey)
	if id == "" && c.Request != nil {
		if value, ok := c.Request.Context().Value(common.RequestIdKey).(string); ok {
			id = value
		}
	}
	if id == "" {
		id = common.NewRequestId()
	}
	c.Header(common.RequestIdKey, id)
	c.Header(NewAPIRequestIdHeader, id)
	return id
}

// WriteHarnessContractError renders the contract envelope and aborts the
// handler chain. It is intentionally separate from abortWithOpenAiMessage so
// ordinary sk- clients and native model errors never grow Harness-only fields.
func WriteHarnessContractError(c *gin.Context, status int, code, message string, retryable bool, details map[string]any) {
	if c == nil {
		return
	}
	requestID := HarnessRequestID(c)
	c.JSON(status, gin.H{"error": HarnessContractErrorBody{
		Code:      code,
		Message:   message,
		Retryable: retryable,
		RequestId: requestID,
		Details:   details,
	}})
	c.Abort()
}

// IsHarnessInternalPath reports whether the request is on the server-to-server
// /internal/v1/gateway namespace. Those routes must answer with the contract
// envelope even when authentication fails before a controller runs.
func IsHarnessInternalPath(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	return strings.HasPrefix(c.Request.URL.Path, "/internal/v1/gateway")
}

// HarnessRetryable is the contract default for an internal read/health error.
// A routing miss is permanent for the caller; a 5xx gateway failure is retryable.
func HarnessRetryable(status int) bool {
	return status >= http.StatusInternalServerError
}
