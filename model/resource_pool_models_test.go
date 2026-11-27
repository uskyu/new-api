package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestResourcePoolModelsRequiresEnabledAbility pins the A3 invariant: a model
// listed in channels.models is not routable unless that same channel also has
// an enabled ability for it. The test keeps the old "channel only" behavior
// visible by asserting the no-ability channel is excluded.
func TestResourcePoolModelsRequiresEnabledAbility(t *testing.T) {
	oldDB := DB
	oldConfig := setting.GetIntelligentRoutingConfig()
	db, err := gorm.Open(sqlite.Open("file:resource-pool-models-test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() {
		DB = oldDB
		require.NoError(t, setting.UpdateIntelligentRoutingConfig(oldConfig))
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))

	withAbility := Channel{Type: 1, Key: "key-a", Name: "with-ability", Models: "model-x,model-z", Group: "g1", Status: common.ChannelStatusEnabled}
	noAbility := Channel{Type: 1, Key: "key-b", Name: "no-ability", Models: "model-y", Group: "g1", Status: common.ChannelStatusEnabled}
	disabledAbility := Channel{Type: 1, Key: "key-c", Name: "disabled-ability", Models: "model-w", Group: "g1", Status: common.ChannelStatusEnabled}
	disabledChannel := Channel{Type: 1, Key: "key-d", Name: "disabled-channel", Models: "model-v", Group: "g1", Status: common.ChannelStatusManuallyDisabled}
	for _, channel := range []*Channel{&withAbility, &noAbility, &disabledAbility, &disabledChannel} {
		require.NoError(t, db.Create(channel).Error)
	}
	for _, ability := range []Ability{
		{Group: "g1", Model: "model-x", ChannelId: withAbility.Id, Enabled: true},
		{Group: "g1", Model: "model-z", ChannelId: withAbility.Id, Enabled: true},
		{Group: "g1", Model: "model-w", ChannelId: disabledAbility.Id, Enabled: false},
		{Group: "g1", Model: "model-v", ChannelId: disabledChannel.Id, Enabled: true},
	} {
		require.NoError(t, db.Create(&ability).Error)
	}
	require.NoError(t, setting.UpdateIntelligentRoutingConfig(setting.IntelligentRoutingConfig{
		Groups: []setting.IntelligentRoutingGroup{{
			ID: "pool-a", Name: "Pool A",
			ChannelOrder: []int{withAbility.Id, noAbility.Id, disabledAbility.Id, disabledChannel.Id},
		}},
	}))

	models, err := ResourcePoolModelsWithError("pool-a")
	require.NoError(t, err)
	assert.Equal(t, []string{"model-x", "model-z"}, models,
		"only the enabled channel with enabled abilities contributes models")

	// The old wrapper remains source-compatible for callers that only need the
	// list; it intentionally drops the error.
	assert.Equal(t, models, ResourcePoolModels("pool-a"))
}
