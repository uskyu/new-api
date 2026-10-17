package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestValidateIntelligentRoutingChannelsUsesAnyEnabledAbilityGroup(t *testing.T) {
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open("file:intelligent-routing-controller-test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	channel := model.Channel{Type: 1, Key: "test-key", Status: common.ChannelStatusEnabled, Name: "enabled", Models: "gpt-4o", Group: "unrelated"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "unrelated", Model: "gpt-4o", ChannelId: channel.Id, Enabled: true}).Error)
	err = validateIntelligentRoutingChannels(setting.IntelligentRoutingConfig{
		GlobalChannelOrder: []int{channel.Id},
		Groups:             []setting.IntelligentRoutingGroup{{ID: "pool", Name: "Pool", ChannelOrder: []int{channel.Id}}},
	})
	require.NoError(t, err)
}

func TestIntelligentRoutingAPIContractOmitsLegacyCostGroupFields(t *testing.T) {
	response, err := common.Marshal(intelligentRoutingResponse{
		GlobalChannelOrder: []int{1},
		Groups:             []setting.IntelligentRoutingGroup{{ID: "pool", Name: "Pool", DiscountPercent: 12.5, TaxRatePercent: 8}},
		Channels:           []intelligentRoutingChannel{{ID: 1, EnabledGroups: []intelligentRoutingAbilityGroup{{ID: "group-a", Label: "Group A"}}}},
	})
	require.NoError(t, err)
	json := string(response)
	for _, field := range []string{`"discount_percent":12.5`, `"tax_rate_percent":8`, `"enabled_groups"`} {
		require.Contains(t, json, field)
	}
	for _, field := range []string{"global_cost_group", "cost_groups", "enabled_cost_groups", "cost_group", `"ratio"`} {
		require.NotContains(t, json, field)
	}
}

func TestHarnessCatalogGroupReturnsDiscountAndTaxWithoutLegacyRatio(t *testing.T) {
	response, err := common.Marshal(harnessCatalogGroup{
		ID: "pool", Name: "Pool", DiscountPercent: 7.25, TaxRatePercent: 9.5,
	})
	require.NoError(t, err)
	json := string(response)
	require.Contains(t, json, `"discount_percent":7.25`)
	require.Contains(t, json, `"tax_rate_percent":9.5`)
	require.NotContains(t, json, "cost_group")
	require.NotContains(t, json, `"ratio"`)
}
