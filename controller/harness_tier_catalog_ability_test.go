package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const harnessTierCatalogAbilityModelX = "model-x"

// TestHarnessTierCatalogAbilityFilteringIsPerPool pins the A3 ability filter at
// pool granularity. The same model id may be declared by channels in two pools:
// only the pool whose channel has an enabled ability for it may advertise the
// model. A channel that merely lists model-x in channels.models must not make it
// routable, even though another pool can serve the same model id.
func TestHarnessTierCatalogAbilityFilteringIsPerPool(t *testing.T) {
	setupHarnessTierCatalogTestDB(t, map[string][]string{
		"chat":  {harnessTierCatalogAbilityModelX},
		"video": {harnessTierCatalogAbilityModelX},
	})

	// The fixture creates an enabled ability on both channels. Drop only the
	// chat channel's ability, leaving the models declaration intact, which is the
	// drift this filter exists to survive.
	var chatChannel model.Channel
	require.NoError(t, model.DB.Where("name = ?", "harness-tier-catalog-chat").First(&chatChannel).Error)
	assert.Contains(t, chatChannel.GetModels(), harnessTierCatalogAbilityModelX,
		"the chat channel must still declare model-x")

	result := model.DB.Model(&model.Ability{}).
		Where("channel_id = ? AND model = ?", chatChannel.Id, harnessTierCatalogAbilityModelX).
		Update("enabled", false)
	require.NoError(t, result.Error)
	require.Equal(t, int64(1), result.RowsAffected,
		"the fixture must have exactly one chat ability to disable")

	// A price exists for model-x, so the video pool advertises it. The chat pool
	// must still omit it: pricing never overrides a missing ability.
	priceHarnessTierCatalogModels(t, harnessTierCatalogAbilityModelX)
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "chat", DisplayName: "通用对话", Kind: setting.HarnessCatalogKindText},
		{ID: "video", DisplayName: "视频", Kind: setting.HarnessCatalogKindVideo},
	})

	recorder := requestHarnessTierCatalog(t, "", true)
	require.Equal(t, http.StatusOK, recorder.Code)

	_, items := catalogItems(t, recorder)
	byID := make(map[string]map[string]any, len(items))
	for _, item := range items {
		byID[item["id"].(string)] = item
	}

	chat := byID["chat"]
	require.NotNil(t, chat)
	assert.Equal(t, []any{}, chat["models"],
		"a declared model without an enabled ability must not be advertised")
	assert.NotContains(t, chat["models"], harnessTierCatalogAbilityModelX)

	video := byID["video"]
	require.NotNil(t, video)
	assert.Equal(t, []any{harnessTierCatalogAbilityModelX}, video["models"],
		"the pool with an enabled ability and a price must advertise the same model")

	// Pin the model layer directly as well: the A3 result must be the pool-level
	// ability predicate, not an accidental side effect of pricing or ordering.
	assert.NotContains(t, model.ResourcePoolModels("chat"), harnessTierCatalogAbilityModelX)
	assert.Contains(t, model.ResourcePoolModels("video"), harnessTierCatalogAbilityModelX)
}
