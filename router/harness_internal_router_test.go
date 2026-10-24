package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHarnessInternalRouterRegistersColonBatchGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetHarnessInternalRouter(engine)

	found := map[string]bool{}
	for _, route := range engine.Routes() {
		found[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /internal/v1/gateway/usage-events",
		"POST /internal/v1/gateway/usage-events:batchGet",
		"GET /internal/v1/gateway/cost-records",
		"POST /internal/v1/gateway/cost-records:batchGet",
		"GET /internal/v1/gateway/health",
	} {
		assert.True(t, found[want], "route %s must be registered", want)
	}
}
