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

func TestIntelligentRoutingConfigIgnoresLegacyPricingFields(t *testing.T) {
	var config IntelligentRoutingConfig
	require.NoError(t, common.UnmarshalJsonStr(`{"global_channel_order":[1],"global_cost_group":"old","groups":[{"id":"pool","name":"Pool","cost_group":"old","discount_percent":100,"tax_rate_percent":8,"channel_order":[2]}]}`, &config))
	require.NoError(t, ValidateIntelligentRoutingConfig(config))
	assert.Equal(t, "Pool", IntelligentRoutingGroups(config)["pool"])
	serialized, err := IntelligentRoutingConfig2JSONString(config)
	require.NoError(t, err)
	assert.NotContains(t, serialized, "cost_group")
	assert.NotContains(t, serialized, "discount_percent")
	assert.NotContains(t, serialized, "tax_rate_percent")
	assert.NotContains(t, serialized, "global_channel_order")
	assert.NotContains(t, serialized, "token_group_access")
}

func TestIntelligentRoutingDefaultJSONNeedsNoGlobalChannelOrder(t *testing.T) {
	const defaultJSON = `{"pricing_group_order":[],"groups":[]}`
	require.NoError(t, ValidateIntelligentRoutingJSON(defaultJSON))

	var config IntelligentRoutingConfig
	require.NoError(t, common.UnmarshalJsonStr(defaultJSON, &config))
	assert.Empty(t, config.PricingGroupOrder)
	assert.Empty(t, config.Groups)
	assert.Empty(t, IntelligentRoutingGroups(config))
}
