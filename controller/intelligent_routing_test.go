package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
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
	response, err := common.Marshal(harnessCatalogGroup{
		ID:             "pool",
		Name:           "Pool",
		Models:         []harnessCatalogModel{},
		UnpricedModels: []harnessCatalogUnpricedModel{},
	})
	require.NoError(t, err)
	json := string(response)
	require.Contains(t, json, `"id":"pool"`)
	require.Contains(t, json, `"name":"Pool"`)
	require.Contains(t, json, `"unpriced_models":[]`)
	require.NotContains(t, json, "cost_group")
	require.NotContains(t, json, "discount_percent")
	require.NotContains(t, json, "tax_rate_percent")
}

func configureHarnessTestPricing(t *testing.T, modelPrices, modelRatios map[string]float64, removePrices []string) {
	t.Helper()
	oldPrices := ratio_setting.ModelPrice2JSONString()
	oldRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(oldPrices))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldRatios))
	})

	prices := ratio_setting.GetModelPriceMap()
	for _, modelName := range removePrices {
		delete(prices, modelName)
	}
	for modelName, value := range modelPrices {
		prices[modelName] = value
	}
	priceJSON, err := common.Marshal(prices)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(priceJSON)))

	ratios := ratio_setting.GetModelRatioCopy()
	for modelName, value := range modelRatios {
		ratios[modelName] = value
	}
	ratioJSON, err := common.Marshal(ratios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(ratioJSON)))
}

func TestHarnessCatalogExcludesUnpricedModelsAndReturnsDiagnostics(t *testing.T) {
	configureHarnessTestPricing(t, map[string]float64{"priced-model": 1}, nil, nil)
	models, unpriced := buildHarnessCatalogModels(
		map[string][]string{
			"priced-model":   {"upstream-priced"},
			"unpriced-model": {"z-upstream", "a-upstream", "z-upstream"},
		},
		map[string]model.Pricing{
			"priced-model":   {ModelName: "priced-model", ModelRatio: 1},
			"unpriced-model": {ModelName: "unpriced-model", ModelRatio: 37.5},
		},
		map[string]*harnessRoutePricingProviders{
			"priced-model":   {HasBaseRoute: true},
			"unpriced-model": {HasBaseRoute: true},
		},
	)

	require.Len(t, models, 1)
	assert.Equal(t, "priced-model", models[0].Name)
	require.Len(t, unpriced, 1)
	assert.Equal(t, harnessCatalogUnpricedModel{
		Name:           "unpriced-model",
		UpstreamModels: []string{"a-upstream", "z-upstream"},
		Reason:         "pricing_not_configured",
	}, unpriced[0])

	response, err := common.Marshal(harnessCatalogGroup{
		ID: "pool", Name: "Pool", Models: models, UnpricedModels: unpriced,
	})
	require.NoError(t, err)
	assert.Contains(t, string(response), `"unpriced_models":[{"name":"unpriced-model"`)
	assert.NotContains(t, string(response), `"model_ratio":37.5`)
}

func TestHarnessCatalogPricingRequiresConfiguredBilling(t *testing.T) {
	configureHarnessTestPricing(t,
		map[string]float64{"harness-free-price": 0},
		map[string]float64{"harness-free-ratio": 0},
		nil,
	)
	previousSelfUseMode := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() { operation_setting.SelfUseModeEnabled = previousSelfUseMode })

	assert.True(t, hasHarnessBasePricing("harness-free-price"))
	assert.True(t, hasHarnessBasePricing("harness-free-ratio"))
	fallbackRatio, fallbackAccepted, _ := ratio_setting.GetModelRatio("harness-unpriced-fallback")
	require.True(t, fallbackAccepted)
	assert.Equal(t, 37.5, fallbackRatio)
	assert.False(t, hasHarnessBasePricing("harness-unpriced-fallback"))

	var builtinExpressionModel string
	for modelName := range billing_setting.GetBuiltinBillingExprCopy() {
		if _, hasPrice := ratio_setting.GetModelPrice(modelName, false); hasPrice || ratio_setting.HasConfiguredModelRatio(modelName) {
			continue
		}
		if billing_setting.GetBillingMode(modelName) != billing_setting.BillingModeTieredExpr {
			continue
		}
		if expression, ok := billing_setting.GetBillingExpr(modelName); ok && expression != "" {
			builtinExpressionModel = modelName
			break
		}
	}
	require.NotEmpty(t, builtinExpressionModel)
	assert.True(t, hasHarnessBasePricing(builtinExpressionModel))

	savedConfig := config.GlobalConfig.ExportAllConfigs()
	t.Cleanup(func() { require.NoError(t, config.GlobalConfig.LoadFromDB(savedConfig)) })
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"harness-custom-tiered":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"harness-custom-tiered":"tier(\"base\", p * 2 + c * 8)"}`,
	}))
	assert.True(t, hasHarnessBasePricing("harness-custom-tiered"))
}

func TestHarnessChannelPluginBindingsCaptureOnlyConfiguredProviders(t *testing.T) {
	channelSettings := `{"task_plugin_key":"pricing-alpha","task_extend_plugin_keys":["pricing-beta","pricing-alpha"]}`
	channel := &model.Channel{Type: constant.ChannelTypeNewAPI, Setting: &channelSettings}
	bindings := harnessChannelPluginBindingsFor(channel, jsplugin.DefaultRegistry.Generation())
	assert.True(t, bindings.RequiresPlugin)
	assert.Equal(t, []string{"pricing-alpha", "pricing-beta"}, bindings.PluginKeys)

	unboundTaskChannel := &model.Channel{Type: constant.ChannelTypeTaskPlugin}
	bindings = harnessChannelPluginBindingsFor(unboundTaskChannel, jsplugin.DefaultRegistry.Generation())
	assert.True(t, bindings.RequiresPlugin)
	assert.Empty(t, bindings.PluginKeys)
}

func TestHarnessCatalogIncludesPluginOnlyPricesAndRequiresEveryReachableVariant(t *testing.T) {
	configureHarnessTestPricing(t, nil, nil, nil)
	usageSchema := map[string]jsplugin.UsageFieldSchema{
		"seconds": {Type: "number", Unit: "second"},
	}
	pricing := model.Pricing{
		ModelName:  "harness-plugin-only-model",
		ModelRatio: 37.5,
		BillingPluginVariants: []model.PricingPluginVariant{
			{
				PluginKey:          "pricing-alpha",
				PluginName:         "Pricing Alpha",
				BillingMode:        billing_setting.BillingModeTieredExpr,
				BillingExpr:        `tier("base", u("seconds") * 0.4)`,
				BillingUsageSchema: usageSchema,
			},
			{PluginKey: "pricing-beta", PluginName: "Pricing Beta", BillingMode: billing_setting.BillingModeRatio},
		},
	}
	pricingByModel := map[string]model.Pricing{pricing.ModelName: pricing}
	upstreamByModel := map[string][]string{pricing.ModelName: {"upstream-task-model"}}
	pricedRoute := map[string]*harnessRoutePricingProviders{
		pricing.ModelName: {PluginKeys: map[string]struct{}{"pricing-alpha": {}}},
	}
	models, unpriced := buildHarnessCatalogModels(upstreamByModel, pricingByModel, pricedRoute)
	require.Len(t, models, 1)
	require.Empty(t, unpriced)
	assert.Equal(t, 0.0, models[0].ModelRatio)
	require.Len(t, models[0].BillingPluginVariants, 1)
	assert.Equal(t, "pricing-alpha", models[0].BillingPluginVariants[0].PluginKey)

	allVariantsRoute := map[string]*harnessRoutePricingProviders{
		pricing.ModelName: {PluginKeys: map[string]struct{}{"pricing-alpha": {}, "pricing-beta": {}}},
	}
	models, unpriced = buildHarnessCatalogModels(upstreamByModel, pricingByModel, allVariantsRoute)
	assert.Empty(t, models)
	require.Len(t, unpriced, 1)
}

func TestHarnessCatalogExportsRelayResolvedAliasPrice(t *testing.T) {
	configureHarnessTestPricing(t, nil, map[string]float64{"qwen3-max": 4}, []string{"qwen3-max"})
	alias := "qwen3-max@thinking:on@temperature:0.2"
	models, unpriced := buildHarnessCatalogModels(
		map[string][]string{alias: {"upstream-qwen3-max"}},
		map[string]model.Pricing{alias: {ModelName: alias, ModelRatio: 37.5}},
		map[string]*harnessRoutePricingProviders{alias: {HasBaseRoute: true}},
	)
	require.Len(t, models, 1)
	require.Empty(t, unpriced)
	assert.Equal(t, 0, models[0].QuotaType)
	assert.Equal(t, 4.0, models[0].ModelRatio)
	assert.Zero(t, models[0].ModelPrice)
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
	if oldConfig.GlobalChannelOrder == nil {
		oldConfig.GlobalChannelOrder = []int{}
	}
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
