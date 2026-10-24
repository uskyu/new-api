package middleware

import (
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

	harnessCorrelationIdMaxLength = 64
)

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
// else is treated as absent so unrelated clients are never rejected.
func sanitizeHarnessCorrelationId(value string) string {
	if value == "" || len(value) > harnessCorrelationIdMaxLength {
		return ""
	}
	for i := range len(value) {
		switch char := value[i]; {
		case char >= 'a' && char <= 'z',
			char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9',
			char == '-',
			char == '_':
		default:
			return ""
		}
	}
	return value
}
