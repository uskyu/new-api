package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
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

func TestIntelligentRoutingAPIContractContainsChannelsWithoutPricing(t *testing.T) {
	response, err := common.Marshal(intelligentRoutingResponse{
		GlobalChannelOrder: []int{1},
		Groups:             []setting.IntelligentRoutingGroup{{ID: "pool", Name: "Pool", ChannelOrder: []int{1}}},
		Channels:           []intelligentRoutingChannel{{ID: 1, EnabledGroups: []intelligentRoutingAbilityGroup{{ID: "group-a", Label: "Group A"}}}},
	})
	require.NoError(t, err)
	json := string(response)
	require.Contains(t, json, `"enabled_groups"`)
	require.NotContains(t, json, "global_cost_group")
	require.NotContains(t, json, "cost_group")
	require.NotContains(t, json, "discount_percent")
	require.NotContains(t, json, "tax_rate_percent")
}

func TestHarnessCatalogGroupContainsOnlyResourcePoolIdentityAndModels(t *testing.T) {
	response, err := common.Marshal(harnessCatalogGroup{ID: "pool", Name: "Pool"})
	require.NoError(t, err)
	json := string(response)
	require.Contains(t, json, `"id":"pool"`)
	require.Contains(t, json, `"name":"Pool"`)
	require.NotContains(t, json, "cost_group")
	require.NotContains(t, json, "discount_percent")
	require.NotContains(t, json, "tax_rate_percent")
}

func TestSetIntelligentRoutingResponseHeadersUsesCheckedPricingGroupAndChannelTax(t *testing.T) {
	oldDB := model.DB
	oldMainDatabaseType := common.MainDatabaseType()
	oldMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&model.Ability{}, &model.Channel{}))
	t.Cleanup(func() {
		model.DB = oldDB
		common.SetMainDatabaseType(oldMainDatabaseType)
		common.MemoryCacheEnabled = oldMemoryCache
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	oldRatios := ratio_setting.GroupRatio2JSONString()
	oldTaxRates := ratio_setting.GroupTaxRate2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		require.NoError(t, ratio_setting.UpdateGroupTaxRateByJSONString(oldTaxRates))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"group-a":0.8,"group-b":0.8,"group-c":0.7}`))
	oldConfig := setting.GetIntelligentRoutingConfig()
	t.Cleanup(func() { require.NoError(t, setting.UpdateIntelligentRoutingConfig(oldConfig)) })
	require.NoError(t, setting.UpdateIntelligentRoutingConfig(setting.IntelligentRoutingConfig{GlobalChannelOrder: []int{}, PricingGroupOrder: []string{"group-c", "group-a"}}))
	require.NoError(t, ratio_setting.UpdateGroupTaxRateByJSONString(`{"group-a":8,"group-b":8,"group-c":5}`))

	for _, ability := range []model.Ability{
		{Group: "group-a", Model: "model-x", ChannelId: 42, Enabled: true},
		{Group: "group-b", Model: "model-x", ChannelId: 42, Enabled: true},
		{Group: "group-c", Model: "model-x", ChannelId: 42, Enabled: true},
		{Group: "group-c", Model: "model-y", ChannelId: 42, Enabled: true},
	} {
		require.NoError(t, db.Create(&ability).Error)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(c, setting.IntelligentRoutingContextKey, setting.IntelligentRoutingGlobalGroupID)

	channel := &model.Channel{Id: 42, OtherSettings: `{"supplier_tax_rate_percent":6.5}`}
	require.NoError(t, db.Create(channel).Error)
	apiErr := setIntelligentRoutingResponseHeaders(c, channel, "model-y")
	require.Nil(t, apiErr)
	assert.Equal(t, "42", c.Writer.Header().Get(setting.IntelligentRoutingChannelIDHeader))
	assert.Equal(t, "0.7", c.Writer.Header().Get(setting.IntelligentRoutingGroupRatioHeader))
	assert.Equal(t, "group-c", c.Writer.Header().Get(setting.IntelligentRoutingPricingGroupHeader))
	assert.Equal(t, "6.5", c.Writer.Header().Get(setting.IntelligentRoutingTaxRatePercentHeader))

	apiErr = setIntelligentRoutingResponseHeaders(c, channel, "model-x")
	require.Nil(t, apiErr)
	assert.Equal(t, "group-c", c.Writer.Header().Get(setting.IntelligentRoutingPricingGroupHeader))

	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(c, setting.IntelligentRoutingContextKey, setting.IntelligentRoutingGlobalGroupID)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"group-a":0.8,"group-b":0.8,"group-c":0.8}`))
	require.NoError(t, ratio_setting.UpdateGroupTaxRateByJSONString(`{"group-a":8,"group-b":8,"group-c":8}`))
	apiErr = setIntelligentRoutingResponseHeaders(c, channel, "model-x")
	require.Nil(t, apiErr)
	assert.Equal(t, "42", c.Writer.Header().Get(setting.IntelligentRoutingChannelIDHeader))
	assert.Equal(t, "6.5", c.Writer.Header().Get(setting.IntelligentRoutingTaxRatePercentHeader))
	assert.Equal(t, "group-c", c.Writer.Header().Get(setting.IntelligentRoutingPricingGroupHeader))
}
