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

// TestCountResourcePoolModelChannels pins the retry-budget input: only enabled
// channels with an enabled ability for the exact requested model count, and the
// count never exceeds the pool's configured order.
func TestCountResourcePoolModelChannels(t *testing.T) {
	oldDB := DB
	oldConfig := setting.GetIntelligentRoutingConfig()
	db, err := gorm.Open(sqlite.Open("file:resource-pool-count-test?mode=memory&cache=shared"), &gorm.Config{})
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

	enabledA := Channel{Type: 1, Key: "key-a", Name: "a", Models: "model-x", Group: "g1", Status: common.ChannelStatusEnabled}
	enabledB := Channel{Type: 1, Key: "key-b", Name: "b", Models: "model-x", Group: "g2", Status: common.ChannelStatusEnabled}
	disabled := Channel{Type: 1, Key: "key-c", Name: "c", Models: "model-x", Group: "g1", Status: common.ChannelStatusManuallyDisabled}
	otherModel := Channel{Type: 1, Key: "key-d", Name: "d", Models: "model-y", Group: "g1", Status: common.ChannelStatusEnabled}
	noAbility := Channel{Type: 1, Key: "key-e", Name: "e", Models: "model-x", Group: "g1", Status: common.ChannelStatusEnabled}
	for _, channel := range []*Channel{&enabledA, &enabledB, &disabled, &otherModel, &noAbility} {
		require.NoError(t, db.Create(channel).Error)
	}
	for _, ability := range []Ability{
		{Group: "g1", Model: "model-x", ChannelId: enabledA.Id, Enabled: true},
		{Group: "g2", Model: "model-x", ChannelId: enabledA.Id, Enabled: true},
		{Group: "g1", Model: "model-x", ChannelId: enabledB.Id, Enabled: true},
		{Group: "g1", Model: "model-x", ChannelId: disabled.Id, Enabled: true},
		{Group: "g1", Model: "model-y", ChannelId: otherModel.Id, Enabled: true},
	} {
		require.NoError(t, db.Create(&ability).Error)
	}

	require.NoError(t, setting.UpdateIntelligentRoutingConfig(setting.IntelligentRoutingConfig{
		Groups: []setting.IntelligentRoutingGroup{{
			ID: "pool-a", Name: "Pool A",
			ChannelOrder: []int{enabledA.Id, enabledB.Id, disabled.Id, otherModel.Id, noAbility.Id},
		}},
	}))

	assert.Equal(t, 2, CountResourcePoolModelChannels("pool-a", "model-x"),
		"the disabled channel, the other model and the channel without an ability must not count")
	assert.Equal(t, 0, CountResourcePoolModelChannels("pool-a", "unknown-model"))
	assert.Equal(t, 0, CountResourcePoolModelChannels("missing-pool", "model-x"))
	assert.Equal(t, 0, CountResourcePoolModelChannels("", "model-x"))
	assert.Equal(t, 0, CountResourcePoolModelChannels("pool-a", ""))
}
