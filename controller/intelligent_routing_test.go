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

// TestNormalizeIntelligentRoutingChannelOrdersKeepsDisabledChannels pins the
// routing order cleanup contract: a channel that was deleted is pruned from the
// stored order, while a temporarily disabled channel keeps its slot so that
// re-enabling it restores the administrator's priority order.
func TestNormalizeIntelligentRoutingChannelOrdersKeepsDisabledChannels(t *testing.T) {
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open("file:intelligent-routing-normalize-test?mode=memory&cache=shared"), &gorm.Config{})
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

	enabled := model.Channel{Type: 1, Key: "enabled-key", Status: common.ChannelStatusEnabled, Name: "enabled", Models: "model-a"}
	disabled := model.Channel{Type: 1, Key: "disabled-key", Status: common.ChannelStatusManuallyDisabled, Name: "disabled", Models: "model-a"}
	require.NoError(t, db.Create(&enabled).Error)
	require.NoError(t, db.Create(&disabled).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "model-a", ChannelId: enabled.Id, Enabled: true}).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "model-a", ChannelId: disabled.Id, Enabled: true}).Error)

	const deletedChannelID = 4040
	channels, err := model.GetAllChannelsWithoutKeys()
	require.NoError(t, err)
	normalized := normalizeIntelligentRoutingChannelOrders(setting.IntelligentRoutingConfig{
		GlobalChannelOrder: []int{deletedChannelID, disabled.Id, enabled.Id},
		Groups: []setting.IntelligentRoutingGroup{
			{ID: "mixed", Name: "Mixed", ChannelOrder: []int{deletedChannelID, enabled.Id}},
			{ID: "disabled-only", Name: "Disabled only", ChannelOrder: []int{disabled.Id}},
		},
	}, newIntelligentRoutingChannelInventory(channels))

	// The deleted channel is gone; the disabled one keeps its position.
	assert.Equal(t, []int{disabled.Id, enabled.Id}, normalized.GlobalChannelOrder)
	require.Len(t, normalized.Groups, 2)
	assert.Equal(t, "mixed", normalized.Groups[0].ID)
	assert.Equal(t, []int{enabled.Id}, normalized.Groups[0].ChannelOrder)
	// A pool whose channels all still exist survives even though none is selectable.
	assert.Equal(t, "disabled-only", normalized.Groups[1].ID)
	assert.Equal(t, []int{disabled.Id}, normalized.Groups[1].ChannelOrder)
}

// TestCloneTokenGroupAccessIsolatesLiveConfig guards a save-time aliasing bug:
// pruning a stale pool must not mutate the live config's map, which previously
// left the in-memory state disagreeing with the stored option.
func TestCloneTokenGroupAccessIsolatesLiveConfig(t *testing.T) {
	live := map[int][]string{
		7: {"pool-a", "pool-b"},
		8: {"pool-a"},
	}
	cloned := cloneTokenGroupAccess(live)
	require.Equal(t, live, cloned)

	// Mutating the clone the way a save does must leave the live map intact.
	delete(cloned, 8)
	cloned[7] = cloned[7][:1]

	assert.Len(t, live, 2, "deleting a token from the clone must not touch the live map")
	assert.Equal(t, []string{"pool-a", "pool-b"}, live[7], "truncating the clone must not shrink the live slice")
	require.Contains(t, live, 8)
	assert.Nil(t, cloneTokenGroupAccess(nil), "a nil grant map must stay nil")
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
		jsplugin.DefaultRegistry.Generation(),
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
	// An unpriced model must not leak any price at all.
	assert.NotContains(t, string(response), `"input":75`)
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

// TestHarnessCatalogExposesUnitPricesFromConfiguredRatios pins the catalog price
// contract: ratios are converted with 1e6 / quota_per_unit (never a hardcoded
// factor), lanes the operator did not configure stay absent, and a Claude cache
// write additionally publishes its 1h tier.
func TestHarnessCatalogExposesUnitPricesFromConfiguredRatios(t *testing.T) {
	previousQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })

	configureHarnessTestPricing(t, nil, map[string]float64{
		"harness-price-plain":  1,
		"harness-price-claude": 1.5,
	}, []string{"harness-price-plain", "harness-price-claude"})

	// Only input/output are configured for this name.
	ordinary := harnessCatalogTokenPricing("harness-price-plain", "USD", 1)
	require.NotNil(t, ordinary.Input)
	assert.InDelta(t, 2.0, *ordinary.Input, 1e-9, "ratio 1 at quota_per_unit 500000 is 2 USD per 1M")
	require.NotNil(t, ordinary.Output)
	assert.InDelta(t, 2.0, *ordinary.Output, 1e-9)
	// Unconfigured lanes must be absent rather than zero.
	assert.Nil(t, ordinary.CacheRead)
	assert.Nil(t, ordinary.CacheWrite)
	assert.Nil(t, ordinary.CacheWrite1h)
	assert.Nil(t, ordinary.ImageInput)

	// The conversion factor follows the configured quota unit, not a constant.
	previousQuotaPerUnit = common.QuotaPerUnit
	common.QuotaPerUnit = 1_000_000
	doubled := harnessCatalogTokenPricing("harness-price-plain", "USD", 1)
	require.NotNil(t, doubled.Input)
	assert.InDelta(t, 1.0, *doubled.Input, 1e-9, "ratio 1 at quota_per_unit 1000000 is 1 USD per 1M")
	common.QuotaPerUnit = previousQuotaPerUnit

	// Claude cache write publishes both the 5m price and the 1h tier.
	oldCacheRatios := ratio_setting.CreateCacheRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateCreateCacheRatioByJSONString(oldCacheRatios)) })
	cacheRatios := ratio_setting.GetCreateCacheRatioCopy()
	cacheRatios["harness-price-claude"] = 1.25
	cacheJSON, err := common.Marshal(cacheRatios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateCreateCacheRatioByJSONString(string(cacheJSON)))

	claude := harnessCatalogTokenPricing("harness-price-claude", "USD", 1)
	require.NotNil(t, claude.Input)
	require.NotNil(t, claude.CacheWrite)
	assert.InDelta(t, 3.0, *claude.Input, 1e-9)
	assert.InDelta(t, 3.75, *claude.CacheWrite, 1e-9)
	require.NotNil(t, claude.CacheWrite1h)
	assert.InDelta(t, 6.0, *claude.CacheWrite1h, 1e-9, "1h cache write is the 5m price times 6/3.75")
}

// TestHarnessCatalogScalesPricesIntoDisplayCurrency pins method A: catalog prices
// are reported in the instance display currency so they match the admin pricing
// UI and the model marketplace, while a tiered expression stays USD because the
// expression engine already prices in USD.
func TestHarnessCatalogScalesPricesIntoDisplayCurrency(t *testing.T) {
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })

	configureHarnessTestPricing(t, nil, map[string]float64{"harness-price-currency": 1}, []string{"harness-price-currency"})

	general := operation_setting.GetGeneralSetting()
	previousDisplayType := general.QuotaDisplayType
	previousExchangeRate := operation_setting.USDExchangeRate
	t.Cleanup(func() {
		general.QuotaDisplayType = previousDisplayType
		operation_setting.USDExchangeRate = previousExchangeRate
	})

	general.QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	usd := harnessCatalogTokenPricing("harness-price-currency", "USD", 1)
	require.NotNil(t, usd.Input)
	assert.InDelta(t, 2.0, *usd.Input, 1e-9)

	// CNY with a 1:1 rate keeps the same numbers, which is the planned setup.
	general.QuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
	operation_setting.USDExchangeRate = 1
	currency, rate := harnessCatalogDisplayCurrency()
	assert.Equal(t, "CNY", currency)
	assert.InDelta(t, 1.0, rate, 1e-9)
	cny := harnessCatalogTokenPricing("harness-price-currency", currency, rate)
	require.NotNil(t, cny.Input)
	assert.InDelta(t, 2.0, *cny.Input, 1e-9)

	// A non-1 rate actually scales, proving the rate is applied.
	operation_setting.USDExchangeRate = 7
	currency, rate = harnessCatalogDisplayCurrency()
	assert.Equal(t, "CNY", currency)
	assert.InDelta(t, 7.0, rate, 1e-9)
	scaled := harnessCatalogTokenPricing("harness-price-currency", currency, rate)
	require.NotNil(t, scaled.Input)
	assert.InDelta(t, 14.0, *scaled.Input, 1e-9)
}

// TestHarnessRouteForModelDoesNotTreatOrdinaryRoutesAsPluginBacked pins the fix
// for a live catalog bug: the bundled sora task plugin registers the
// general-purpose OpenAI channel type, so every ordinary OpenAI channel was
// misread as a plugin-only route and its models were reported as unpriced.
func TestHarnessRouteForModelDoesNotTreatOrdinaryRoutesAsPluginBacked(t *testing.T) {
	generation := jsplugin.DefaultRegistry.Generation()

	// A plain OpenAI channel is claimed by the sora plugin through channel type 1.
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI}
	bindings := harnessChannelPluginBindingsFor(channel, generation)
	assert.False(t, bindings.RequiresPlugin, "a channel-type claim alone must not make the channel plugin-only")
	require.Equal(t, "sora", bindings.TypePluginKey)

	// For an ordinary chat model the channel stays a base relay route.
	ordinary := harnessRouteForModel(bindings, generation, "deepseek-v4.1-flash")
	assert.False(t, ordinary.RequiresPlugin, "an ordinary model must remain a base relay route")
	assert.Empty(t, ordinary.PluginKeys)

	// For a model the plugin actually serves the plugin claim applies.
	pluginRoute := harnessRouteForModel(bindings, generation, "sora-2")
	assert.True(t, pluginRoute.RequiresPlugin, "the plugin's own model must be treated as plugin-backed")
	assert.Contains(t, pluginRoute.PluginKeys, "sora")

	// An explicit task-plugin channel keeps requiring a plugin regardless of model.
	taskChannel := &model.Channel{Type: constant.ChannelTypeTaskPlugin}
	taskBindings := harnessChannelPluginBindingsFor(taskChannel, generation)
	assert.True(t, taskBindings.RequiresPlugin)
	assert.True(t, harnessRouteForModel(taskBindings, generation, "some-model").RequiresPlugin)
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
	models, unpriced := buildHarnessCatalogModels(upstreamByModel, pricingByModel, pricedRoute, jsplugin.DefaultRegistry.Generation())
	require.Len(t, models, 1)
	require.Empty(t, unpriced)
	// A plugin-only model has no base price, so the base pricing carries no lanes.
	assert.Equal(t, harnessCatalogPricingModeToken, models[0].Pricing.Mode)
	assert.Nil(t, models[0].Pricing.Input)
	require.Len(t, models[0].BillingPluginVariants, 1)
	assert.Equal(t, "pricing-alpha", models[0].BillingPluginVariants[0].PluginKey)

	allVariantsRoute := map[string]*harnessRoutePricingProviders{
		pricing.ModelName: {PluginKeys: map[string]struct{}{"pricing-alpha": {}, "pricing-beta": {}}},
	}
	models, unpriced = buildHarnessCatalogModels(upstreamByModel, pricingByModel, allVariantsRoute, jsplugin.DefaultRegistry.Generation())
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
		jsplugin.DefaultRegistry.Generation(),
	)
	require.Len(t, models, 1)
	require.Empty(t, unpriced)
	// The alias resolves to qwen3-max's configured ratio of 4, i.e. 8 USD per
	// million input tokens at the default quota unit.
	assert.Equal(t, harnessCatalogPricingModeToken, models[0].Pricing.Mode)
	require.NotNil(t, models[0].Pricing.Input)
	assert.InDelta(t, 8.0, *models[0].Pricing.Input, 1e-9)
}

func TestHarnessCatalogResolvesMappedTaskAliasPluginPrice(t *testing.T) {
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	db, err := gorm.Open(sqlite.Open("file:harness-task-alias-catalog?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.Channel{}))

	const pluginKey = "harness-mapped-alias-price"
	const alias = "harness-task-alias"
	const declaredModel = "harness-declared-task-model"
	source := `
export const meta = {
  apiVersion: 1, key: "harness-mapped-alias-price", name: "Harness Alias Price", version: "1.0.0",
  author: {name: "Test"}, models: ["harness-declared-task-model"], fetchMode: "per_task",
  usageSchema: {seconds: {type: "number", unit: "second"}}
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {}; }
`
	_, err = jsplugin.DefaultRegistry.Register(source, jsplugin.Options{})
	require.NoError(t, err)
	t.Cleanup(func() { jsplugin.DefaultRegistry.Unregister(pluginKey) })

	oldConfig := config.GlobalConfig.ExportAllConfigs()
	t.Cleanup(func() { require.NoError(t, config.GlobalConfig.LoadFromDB(oldConfig)) })
	expression := `tier("base", u("seconds") * 0.4)`
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		billing_setting.PluginBillingExprOption: `{"harness-mapped-alias-price::harness-declared-task-model":"tier(\"base\", u(\"seconds\") * 0.4)"}`,
	}))

	settingJSON := `{"task_plugin_key":"harness-mapped-alias-price"}`
	modelMapping := `{"harness-task-alias":"harness-declared-task-model"}`
	channel := model.Channel{
		Type:         constant.ChannelTypeTaskPlugin,
		Status:       common.ChannelStatusEnabled,
		Name:         "Harness mapped plugin",
		Models:       alias,
		Group:        "default",
		Setting:      &settingJSON,
		ModelMapping: &modelMapping,
	}
	require.NoError(t, db.Create(&channel).Error)

	pricing := model.Pricing{ModelName: alias, ModelRatio: 37.5}
	require.Empty(t, pricing.BillingPluginVariants)
	models, unpriced := buildHarnessCatalogModels(
		map[string][]string{alias: {declaredModel}},
		map[string]model.Pricing{alias: pricing},
		map[string]*harnessRoutePricingProviders{
			alias: {PluginKeys: map[string]struct{}{pluginKey: {}}},
		},
		jsplugin.DefaultRegistry.Generation(),
	)
	require.Len(t, models, 1)
	require.Empty(t, unpriced)
	// The model has no base price; its reachable plugin bills by expression.
	assert.Nil(t, models[0].Pricing.Input)
	require.Len(t, models[0].BillingPluginVariants, 1)
	assert.Equal(t, pluginKey, models[0].BillingPluginVariants[0].PluginKey)
	assert.Equal(t, harnessCatalogPricingModeTieredExpr, models[0].BillingPluginVariants[0].Pricing.Mode)
	assert.Equal(t, expression, models[0].BillingPluginVariants[0].Pricing.Expression)

	modelExpression := `tier("base", u("seconds") * 0.7)`
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":          `{"harness-declared-task-model":"tiered_expr"}`,
		"billing_setting.billing_expr":          `{"harness-declared-task-model":"tier(\"base\", u(\"seconds\") * 0.7)"}`,
		billing_setting.PluginBillingExprOption: `{}`,
	}))
	pricing.BillingPluginVariants = nil
	models, unpriced = buildHarnessCatalogModels(
		map[string][]string{alias: {declaredModel}},
		map[string]model.Pricing{alias: pricing},
		map[string]*harnessRoutePricingProviders{
			alias: {PluginKeys: map[string]struct{}{pluginKey: {}}},
		},
		jsplugin.DefaultRegistry.Generation(),
	)
	require.Len(t, models, 1)
	require.Empty(t, unpriced)
	assert.Equal(t, harnessCatalogPricingModeTieredExpr, models[0].Pricing.Mode)
	assert.Equal(t, modelExpression, models[0].Pricing.Expression)
	require.Len(t, models[0].BillingPluginVariants, 1)
	assert.Equal(t, modelExpression, models[0].BillingPluginVariants[0].Pricing.Expression)
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
