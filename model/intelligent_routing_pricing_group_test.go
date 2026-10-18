package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHarnessPricingGroupOptInAndPriority(t *testing.T) {
	oldDB := DB
	oldCache := common.MemoryCacheEnabled
	oldConfig := setting.GetIntelligentRoutingConfig()
	if oldConfig.GlobalChannelOrder == nil {
		oldConfig.GlobalChannelOrder = []int{}
	}
	oldRatios := ratio_setting.GroupRatio2JSONString()

	channelSyncLock.RLock()
	oldGroup2Model2Channels := group2model2channels
	oldChannelModelEnabledGroups := channelModelEnabledGroups
	oldChannelsIDM := channelsIDM
	oldChannel2AdvancedCustomConfig := channel2advancedCustomConfig
	channelSyncLock.RUnlock()

	db, err := gorm.Open(sqlite.Open("file:harness-pricing-test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		DB = oldDB
		common.MemoryCacheEnabled = oldCache
		require.NoError(t, setting.UpdateIntelligentRoutingConfig(oldConfig))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		if oldDB != nil {
			InitChannelCache()
		}
		if !oldCache || oldDB == nil {
			channelSyncLock.Lock()
			group2model2channels = oldGroup2Model2Channels
			channelModelEnabledGroups = oldChannelModelEnabledGroups
			channelsIDM = oldChannelsIDM
			channel2advancedCustomConfig = oldChannel2AdvancedCustomConfig
			channelSyncLock.Unlock()
		}
		sqlDB, err := db.DB()
		if err == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	DB = db
	common.MemoryCacheEnabled = true
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"a":1,"b":0.8,"c":0.7}`))
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))

	channelA := Channel{Type: 1, Key: "test", Name: "a", Models: "model-x", Group: "a,b", Status: common.ChannelStatusEnabled}
	channelB := Channel{Type: 1, Key: "test", Name: "b", Models: "model-x", Group: "c", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channelA).Error)
	require.NoError(t, db.Create(&channelB).Error)
	for _, ability := range []Ability{
		{Group: "a", Model: "model-x", ChannelId: channelA.Id, Enabled: true},
		{Group: "b", Model: "model-x", ChannelId: channelA.Id, Enabled: true},
		{Group: "c", Model: "model-x", ChannelId: channelB.Id, Enabled: true},
	} {
		require.NoError(t, db.Create(&ability).Error)
	}
	InitChannelCache()

	config := setting.IntelligentRoutingConfig{
		GlobalChannelOrder: []int{channelA.Id, channelB.Id},
		PricingGroupOrder:  []string{"c", "b"},
	}
	require.NoError(t, setting.UpdateIntelligentRoutingConfig(config))
	groups, err := GetEnabledAbilityGroupsForChannelModel(channelA.Id, "model-x")
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, groups)
	group, eligible, err := GetPreferredPricingGroupForChannelModel(channelA.Id, "model-x")
	require.NoError(t, err)
	require.True(t, eligible)
	require.Equal(t, "b", group)

	chosen, err := GetRandomSatisfiedChannelWithPricingGroups("global", "default", "model-x", 0, []dto.ChannelFilter{}, config.GlobalChannelOrder)
	require.NoError(t, err)
	require.Equal(t, channelA.Id, chosen.Id)

	config.PricingGroupOrder = []string{"c"}
	require.NoError(t, setting.UpdateIntelligentRoutingConfig(config))
	chosen, err = GetRandomSatisfiedChannelWithPricingGroups("global", "default", "model-x", 0, []dto.ChannelFilter{}, config.GlobalChannelOrder)
	require.NoError(t, err)
	require.Equal(t, channelB.Id, chosen.Id)
	chosen, err = GetRandomSatisfiedChannelWithPricingGroups("global", "default", "model-x", 1, []dto.ChannelFilter{}, config.GlobalChannelOrder)
	require.NoError(t, err)
	require.Nil(t, chosen)

	config.PricingGroupOrder = []string{}
	require.NoError(t, setting.UpdateIntelligentRoutingConfig(config))
	chosen, err = GetRandomSatisfiedChannelWithPricingGroups("global", "default", "model-x", 0, []dto.ChannelFilter{}, config.GlobalChannelOrder)
	require.NoError(t, err)
	require.Nil(t, chosen)
}
