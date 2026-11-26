package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

// harnessRetryLimit returns the retry-loop bound for this request. Ordinary
// callers keep the configured RetryTimes. A Harness caller routed through a
// resource pool gets at least one attempt per enabled candidate channel for the
// model, capped by service.HarnessMaxRetryLimit. Channel selection itself still
// comes from CacheGetRandomSatisfiedChannel and its configured order.
func harnessRetryLimit(c *gin.Context, modelName string) int {
	configuredRetries := common.RetryTimes
	if _, harness := middleware.GetHarnessPrincipal(c); !harness {
		return configuredRetries
	}
	poolID := common.GetContextKeyString(c, setting.IntelligentRoutingContextKey)
	if poolID == "" {
		return configuredRetries
	}
	candidateCount := model.CountResourcePoolModelChannels(poolID, modelName)
	return service.HarnessRetryBudget(configuredRetries, candidateCount)
}
