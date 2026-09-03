package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestFilterMarketplacePricingByVendorThreshold(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "public", VendorID: 1},
		{ModelName: "locked", VendorID: 2},
		{ModelName: "unassigned"},
		{ModelName: "missing-vendor", VendorID: 99},
	}
	vendors := []model.PricingVendor{
		{ID: 1, Name: "public"},
		{ID: 2, Name: "locked", MarketplaceQuotaThreshold: 100},
	}

	lowPricing, lowVendors := filterMarketplacePricing(pricing, vendors, 99)
	require.Equal(t, []string{"public", "unassigned", "missing-vendor"}, pricingNames(lowPricing))
	require.Equal(t, []int{1}, vendorIDs(lowVendors))

	boundaryPricing, boundaryVendors := filterMarketplacePricing(pricing, vendors, 100)
	require.Equal(t, []string{"public", "locked", "unassigned", "missing-vendor"}, pricingNames(boundaryPricing))
	require.Equal(t, []int{1, 2}, vendorIDs(boundaryVendors))

	require.Len(t, pricing, 4)
	require.Len(t, vendors, 2)
}

func pricingNames(items []model.Pricing) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.ModelName)
	}
	return names
}

func vendorIDs(items []model.PricingVendor) []int {
	ids := make([]int, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}
