package middleware

import (
	"context"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func RequestId() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := common.NewRequestId()
		c.Set(common.RequestIdKey, id)
		ctx := context.WithValue(c.Request.Context(), common.RequestIdKey, id)
		harnessRequestId := sanitizeHarnessCorrelationId(c.GetHeader(HarnessRequestIdHeader))
		if harnessRequestId != "" {
			c.Set(HarnessRequestIdContextKey, harnessRequestId)
			ctx = context.WithValue(ctx, HarnessRequestIdContextKey, harnessRequestId)
		}
		harnessRunId := sanitizeHarnessCorrelationId(c.GetHeader(HarnessRunIdHeader))
		if harnessRunId != "" {
			c.Set(HarnessRunIdContextKey, harnessRunId)
			ctx = context.WithValue(ctx, HarnessRunIdContextKey, harnessRunId)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Header(common.RequestIdKey, id)
		c.Header(NewAPIRequestIdHeader, id)
		c.Next()
	}
}
