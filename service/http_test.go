package service

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
)

func TestShouldCopyUpstreamHeaderPreservesLocalIntelligentRoutingPricing(t *testing.T) {
	for _, name := range []string{
		setting.IntelligentRoutingGroupHeader,
		setting.IntelligentRoutingDiscountPercentHeader,
		setting.IntelligentRoutingTaxRatePercentHeader,
	} {
		assert.False(t, ShouldCopyUpstreamHeader(nil, name, []string{"upstream-value"}), "upstream must not override %s", name)
	}
	assert.False(t, ShouldCopyUpstreamHeader(nil, http.CanonicalHeaderKey(setting.IntelligentRoutingDiscountPercentHeader), nil))
	assert.True(t, ShouldCopyUpstreamHeader(nil, "X-Upstream-Request-ID", []string{"upstream-request"}))
}
