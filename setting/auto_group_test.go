package setting

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateMaxTokenAutoGroupsAcceptsAnyPositiveInteger(t *testing.T) {
	original := GetMaxTokenAutoGroups()
	t.Cleanup(func() {
		require.NoError(t, UpdateMaxTokenAutoGroups(fmt.Sprintf("%d", original)))
	})

	require.NoError(t, UpdateMaxTokenAutoGroups("123456"))
	assert.Equal(t, 123456, GetMaxTokenAutoGroups())
}

func TestUpdateMaxTokenAutoGroupsRejectsInvalidValuesWithoutChangingState(t *testing.T) {
	original := GetMaxTokenAutoGroups()
	for _, value := range []string{"", "0", "-1", "1.5", "not-a-number"} {
		t.Run(value, func(t *testing.T) {
			assert.Error(t, UpdateMaxTokenAutoGroups(value))
			assert.Equal(t, original, GetMaxTokenAutoGroups())
		})
	}
}

func TestIntelligentRoutingPercentValidationAndLegacyCostFields(t *testing.T) {
	var config IntelligentRoutingConfig
	require.NoError(t, common.UnmarshalJsonStr(`{"global_channel_order":[1],"global_cost_group":"old","groups":[{"id":"pool","name":"Pool","cost_group":"old","discount_percent":100,"tax_rate_percent":0,"channel_order":[2]}]}`, &config))
	require.NoError(t, ValidateIntelligentRoutingConfig(config))
	assert.Equal(t, "Pool", IntelligentRoutingGroups(config)["pool"])
	serialized, err := IntelligentRoutingConfig2JSONString(config)
	require.NoError(t, err)
	assert.NotContains(t, serialized, "cost_group")
	config.TokenGroupAccess = map[int][]string{7: {"pool"}}
	assert.True(t, TokenCanUseIntelligentRoutingGroupForConfig(config, 7, "pool"))
	assert.False(t, TokenCanUseIntelligentRoutingGroupForConfig(config, 7, "another-pool"))
	assert.False(t, TokenCanUseIntelligentRoutingGroupForConfig(config, 7, "unknown"))

	for _, tc := range []struct {
		name  string
		group IntelligentRoutingGroup
	}{
		{name: "negative discount", group: IntelligentRoutingGroup{ID: "pool", Name: "Pool", DiscountPercent: -0.01}},
		{name: "discount above 100", group: IntelligentRoutingGroup{ID: "pool", Name: "Pool", DiscountPercent: 100.01}},
		{name: "negative tax", group: IntelligentRoutingGroup{ID: "pool", Name: "Pool", TaxRatePercent: -0.01}},
		{name: "tax above 100", group: IntelligentRoutingGroup{ID: "pool", Name: "Pool", TaxRatePercent: 100.01}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Error(t, ValidateIntelligentRoutingConfig(IntelligentRoutingConfig{
				GlobalChannelOrder: []int{1},
				Groups:             []IntelligentRoutingGroup{tc.group},
			}))
		})
	}
}
