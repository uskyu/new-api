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
		setting.IntelligentRoutingChannelIDHeader,
		setting.IntelligentRoutingGroupRatioHeader,
		setting.IntelligentRoutingTaxRatePercentHeader,
	} {
		assert.False(t, ShouldCopyUpstreamHeader(nil, name, []string{"upstream-value"}), "upstream must not override %s", name)
	}
	assert.False(t, ShouldCopyUpstreamHeader(nil, http.CanonicalHeaderKey(setting.IntelligentRoutingGroupRatioHeader), nil))
	assert.True(t, ShouldCopyUpstreamHeader(nil, "X-Upstream-Request-ID", []string{"upstream-request"}))
}

// TestShouldCopyUpstreamHeaderRejectsGatewayRequestId keeps the gateway's own
// request id out of reach of the upstream: that id is how the platform attributes
// a cost to a call, so an upstream echoing the header must not be able to rename
// the request or point the platform at an id the gateway never issued.
func TestShouldCopyUpstreamHeaderRejectsGatewayRequestId(t *testing.T) {
	for _, name := range []string{
		newAPIRequestIdHeader,
		http.CanonicalHeaderKey(newAPIRequestIdHeader),
		"x-newapi-request-id",
		"X-NEWAPI-REQUEST-ID",
	} {
		assert.False(t, ShouldCopyUpstreamHeader(nil, name, []string{"spoofed-id"}),
			"upstream must not override %s", name)
	}

	// Neighbouring headers still pass through, so the filter stays narrow.
	assert.True(t, ShouldCopyUpstreamHeader(nil, "X-NewAPI-Trace-Id", []string{"trace"}))
	assert.True(t, ShouldCopyUpstreamHeader(nil, "X-Newapi-Request-Id-Extra", []string{"extra"}))
}
