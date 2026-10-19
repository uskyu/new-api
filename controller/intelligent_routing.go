package controller

import (
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	relayhelper "github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type intelligentRoutingResponse struct {
	GlobalChannelOrder  []int                             `json:"global_channel_order"`
	PricingGroupOrder   []string                          `json:"pricing_group_order"`
	PricingGroupOptions []pricingGroupOption              `json:"pricing_group_options"`
	Groups              []setting.IntelligentRoutingGroup `json:"groups"`
	Channels            []intelligentRoutingChannel       `json:"channels"`
}

type intelligentRoutingChannel struct {
	ID            int                              `json:"id"`
	Name          string                           `json:"name"`
	Status        int                              `json:"status"`
	Models        []string                         `json:"models"`
	Priority      int64                            `json:"priority"`
	EnabledGroups []intelligentRoutingAbilityGroup `json:"enabled_groups"`
}

type pricingGroupOption struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Ratio float64 `json:"ratio"`
}

type intelligentRoutingAbilityGroup struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func GetIntelligentRouting(c *gin.Context) {
	channels, err := model.GetAllChannelsWithoutKeys()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	routingConfig := setting.GetIntelligentRoutingConfig()
	inventory := newIntelligentRoutingChannelInventory(channels)
	response := intelligentRoutingResponse{
		GlobalChannelOrder:  keepKnownRoutingChannelIDs(routingConfig.GlobalChannelOrder, inventory.known),
		PricingGroupOrder:   routingConfig.PricingGroupOrder,
		PricingGroupOptions: make([]pricingGroupOption, 0),
		Groups:              keepKnownRoutingGroups(routingConfig.Groups, inventory.known),
		Channels:            make([]intelligentRoutingChannel, 0, len(channels)),
	}
	abilityGroups := make(map[int]map[string]struct{})
	for _, ability := range model.GetAllEnableAbilities() {
		if abilityGroups[ability.ChannelId] == nil {
			abilityGroups[ability.ChannelId] = make(map[string]struct{})
		}
		abilityGroups[ability.ChannelId][ability.Group] = struct{}{}
	}
	usableGroups := setting.GetUserUsableGroupsCopy()
	for id, ratio := range ratio_setting.GetGroupRatioCopy() {
		name := usableGroups[id]
		if name == "" {
			name = id
		}
		response.PricingGroupOptions = append(response.PricingGroupOptions, pricingGroupOption{ID: id, Name: name, Ratio: ratio})
	}
	slices.SortFunc(response.PricingGroupOptions, func(a, b pricingGroupOption) int { return strings.Compare(a.ID, b.ID) })
	for _, channel := range channels {
		if _, ok := inventory.selectable[channel.Id]; !ok {
			continue
		}
		models := channel.GetModels()
		enabledGroups := make([]intelligentRoutingAbilityGroup, 0, len(abilityGroups[channel.Id]))
		for groupID := range abilityGroups[channel.Id] {
			label, ok := usableGroups[groupID]
			if !ok {
				label = groupID
			}
			enabledGroups = append(enabledGroups, intelligentRoutingAbilityGroup{ID: groupID, Label: label})
		}
		slices.SortFunc(enabledGroups, func(a, b intelligentRoutingAbilityGroup) int {
			return strings.Compare(a.ID, b.ID)
		})
		response.Channels = append(response.Channels, intelligentRoutingChannel{
			ID:            channel.Id,
			Name:          channel.Name,
			Status:        channel.Status,
			Models:        models,
			Priority:      channel.GetPriority(),
			EnabledGroups: enabledGroups,
		})
	}
	common.ApiSuccess(c, response)
}

// intelligentRoutingChannelInventory captures which channel IDs the stored
// routing option may still reference and which ones an administrator can pick
// right now. A channel is known while its row exists in any status, so a
// temporarily disabled channel keeps its priority slot; it is selectable only
// while it is enabled and still exposes an enabled model ability.
type intelligentRoutingChannelInventory struct {
	known      map[int]struct{}
	selectable map[int]struct{}
}

func newIntelligentRoutingChannelInventory(channels []*model.Channel) intelligentRoutingChannelInventory {
	inventory := intelligentRoutingChannelInventory{
		known:      make(map[int]struct{}, len(channels)),
		selectable: make(map[int]struct{}, len(channels)),
	}
	enabled := make(map[int]bool)
	for _, ability := range model.GetAllEnableAbilities() {
		enabled[ability.ChannelId] = true
	}
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		inventory.known[channel.Id] = struct{}{}
		if channel.Status != common.ChannelStatusEnabled || !enabled[channel.Id] {
			continue
		}
		inventory.selectable[channel.Id] = struct{}{}
	}
	return inventory
}

func keepKnownRoutingChannelIDs(channelIDs []int, known map[int]struct{}) []int {
	kept := make([]int, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		if _, ok := known[channelID]; ok {
			kept = append(kept, channelID)
		}
	}
	return kept
}

func keepKnownRoutingGroups(groups []setting.IntelligentRoutingGroup, known map[int]struct{}) []setting.IntelligentRoutingGroup {
	kept := make([]setting.IntelligentRoutingGroup, 0, len(groups))
	for _, group := range groups {
		group.ChannelOrder = keepKnownRoutingChannelIDs(group.ChannelOrder, known)
		kept = append(kept, group)
	}
	return kept
}

// normalizeIntelligentRoutingChannelOrders prunes only channel IDs whose
// channel row no longer exists. A disabled or otherwise temporarily unusable
// channel keeps its slot so re-enabling it restores the administrator's
// configured priority. A resource pool is removed only when every channel in it
// was deleted; a pool submitted without any channel keeps its identity so
// validation can report it.
func normalizeIntelligentRoutingChannelOrders(config setting.IntelligentRoutingConfig, inventory intelligentRoutingChannelInventory) setting.IntelligentRoutingConfig {
	if config.GlobalChannelOrder != nil {
		config.GlobalChannelOrder = keepKnownRoutingChannelIDs(config.GlobalChannelOrder, inventory.known)
	}
	keptGroups := make([]setting.IntelligentRoutingGroup, 0, len(config.Groups))
	for _, group := range config.Groups {
		if len(group.ChannelOrder) > 0 {
			group.ChannelOrder = keepKnownRoutingChannelIDs(group.ChannelOrder, inventory.known)
			if len(group.ChannelOrder) == 0 {
				continue
			}
		}
		keptGroups = append(keptGroups, group)
	}
	config.Groups = keptGroups
	return config
}

type updateIntelligentRoutingRequest struct {
	GlobalChannelOrder []int                             `json:"global_channel_order"`
	PricingGroupOrder  *[]string                         `json:"pricing_group_order"`
	Groups             []setting.IntelligentRoutingGroup `json:"groups"`
}

func UpdateIntelligentRouting(c *gin.Context) {
	var request updateIntelligentRoutingRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Invalid parameters"))
		return
	}
	pricingOrder := setting.GetIntelligentRoutingConfig().PricingGroupOrder
	if request.PricingGroupOrder != nil {
		pricingOrder = *request.PricingGroupOrder
	}
	config := setting.IntelligentRoutingConfig{
		GlobalChannelOrder: request.GlobalChannelOrder,
		PricingGroupOrder:  pricingOrder,
		Groups:             request.Groups,
	}
	if err := setting.ValidateIntelligentRoutingConfig(config); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage(err.Error()))
		return
	}
	for _, id := range config.PricingGroupOrder {
		if !ratio_setting.ContainsGroupRatio(id) {
			common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Unknown pricing group: "+id))
			return
		}
	}
	if config.GlobalChannelOrder == nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("global_channel_order must be provided"))
		return
	}
	channels, err := model.GetAllChannelsWithoutKeys()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	config = normalizeIntelligentRoutingChannelOrders(config, newIntelligentRoutingChannelInventory(channels))
	if err := validateIntelligentRoutingChannels(config); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage(err.Error()))
		return
	}
	previousConfig := setting.GetIntelligentRoutingConfig()
	config.TokenGroupAccess = previousConfig.TokenGroupAccess
	allowedGroups := setting.IntelligentRoutingGroups(config)
	for tokenID, tokenGroups := range config.TokenGroupAccess {
		filtered := make([]string, 0, len(tokenGroups))
		for _, tokenGroup := range tokenGroups {
			if _, exists := allowedGroups[tokenGroup]; exists {
				filtered = append(filtered, tokenGroup)
			}
		}
		if len(filtered) == 0 {
			delete(config.TokenGroupAccess, tokenID)
		} else {
			config.TokenGroupAccess[tokenID] = filtered
		}
	}
	routingJSON, err := setting.IntelligentRoutingConfig2JSONString(config)
	if err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage(err.Error()))
		return
	}
	if err := model.UpdateOption(setting.IntelligentRoutingOptionKey, string(routingJSON)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"global_channel_order": config.GlobalChannelOrder, "pricing_group_order": config.PricingGroupOrder, "groups": config.Groups})
}

// validateIntelligentRoutingChannels rejects stored orders that reference a
// channel row that no longer exists. Availability is deliberately NOT checked
// here: a channel's enabled status and model abilities are volatile, so a
// temporarily disabled channel keeps its configured priority slot and is
// filtered out later, at read time and at channel-selection time
// (model.GetRandomSatisfiedChannelWithPricingGroups). Rejecting it here would
// make every later save fail until the administrator removes the channel.
func validateIntelligentRoutingChannels(config setting.IntelligentRoutingConfig) error {
	channels, err := model.GetAllChannelsWithoutKeys()
	if err != nil {
		return err
	}
	channelByID := make(map[int]struct{}, len(channels))
	for _, channel := range channels {
		if channel != nil {
			channelByID[channel.Id] = struct{}{}
		}
	}
	for _, channelID := range config.GlobalChannelOrder {
		if _, exists := channelByID[channelID]; !exists {
			return common.NewMessage("Global order contains an unknown channel ID")
		}
	}
	for _, group := range config.Groups {
		if len(group.ChannelOrder) == 0 {
			return common.NewMessage("Each resource pool must contain at least one channel")
		}
		for _, channelID := range group.ChannelOrder {
			if _, exists := channelByID[channelID]; !exists {
				return common.NewMessage("Resource pool contains an unknown channel ID")
			}
		}
	}
	return nil
}

type tokenRoutingAccessResponse struct {
	TokenID   int      `json:"token_id"`
	TokenName string   `json:"token_name"`
	Groups    []string `json:"groups"`
}

type updateTokenRoutingAccessRequest struct {
	Groups []string `json:"groups"`
}

func GetIntelligentRoutingTokenAccess(c *gin.Context) {
	config := setting.GetIntelligentRoutingConfig()
	tokenIDs := make([]int, 0, len(config.TokenGroupAccess))
	for tokenID := range config.TokenGroupAccess {
		tokenIDs = append(tokenIDs, tokenID)
	}
	slices.Sort(tokenIDs)
	items := make([]tokenRoutingAccessResponse, 0, len(tokenIDs))
	for _, tokenID := range tokenIDs {
		token, err := model.GetTokenById(tokenID)
		if err != nil {
			continue
		}
		items = append(items, tokenRoutingAccessResponse{TokenID: tokenID, TokenName: token.Name, Groups: config.TokenGroupAccess[tokenID]})
	}
	common.ApiSuccess(c, gin.H{"items": items})
}

func UpdateIntelligentRoutingTokenAccess(c *gin.Context) {
	tokenID, err := strconv.Atoi(c.Param("id"))
	if err != nil || tokenID <= 0 {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Invalid token ID"))
		return
	}
	token, err := model.GetTokenById(tokenID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request updateTokenRoutingAccessRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Invalid parameters"))
		return
	}
	config := setting.GetIntelligentRoutingConfig()
	allowedGroups := setting.IntelligentRoutingGroups(config)
	seen := make(map[string]struct{}, len(request.Groups))
	for _, group := range request.Groups {
		if _, allowed := allowedGroups[group]; !allowed {
			common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Unknown resource pool: "+group))
			return
		}
		if _, duplicate := seen[group]; duplicate {
			common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Duplicate resource pool: "+group))
			return
		}
		seen[group] = struct{}{}
	}
	access := config.TokenGroupAccess
	if access == nil {
		access = make(map[int][]string)
	}
	if len(request.Groups) == 0 {
		delete(access, tokenID)
	} else {
		access[tokenID] = slices.Clone(request.Groups)
	}
	config.TokenGroupAccess = access
	serialized, err := setting.IntelligentRoutingConfig2JSONString(config)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption(setting.IntelligentRoutingOptionKey, serialized); err != nil {
		common.ApiError(c, err)
		return
	}
	params := tokenAuditParams(c)
	params["token_id"] = tokenID
	params["groups"] = request.Groups
	recordManageAudit(c, "intelligent-routing.token-access.update", params)
	common.ApiSuccess(c, tokenRoutingAccessResponse{TokenID: tokenID, TokenName: token.Name, Groups: request.Groups})
}

// GetHarnessCatalog returns only the logical model and cost data needed by Harness.
// Channel credentials, addresses, overrides, and channel IDs are never serialized.
type harnessCatalogResponse struct {
	QuotaPerUnit float64               `json:"quota_per_unit"`
	PriceUnit    string                `json:"price_unit"`
	Groups       []harnessCatalogGroup `json:"groups"`
}

type harnessCatalogGroup struct {
	ID             string                        `json:"id"`
	Name           string                        `json:"name"`
	Models         []harnessCatalogModel         `json:"models"`
	UnpricedModels []harnessCatalogUnpricedModel `json:"unpriced_models"`
}

type harnessCatalogUnpricedModel struct {
	Name           string   `json:"name"`
	UpstreamModels []string `json:"upstream_models"`
	Reason         string   `json:"reason"`
}

type harnessChannelPluginBindings struct {
	PluginKeys     []string
	RequiresPlugin bool
}

type harnessRoutePricingProviders struct {
	HasBaseRoute              bool
	HasUnboundTaskPluginRoute bool
	PluginKeys                map[string]struct{}
}

func harnessChannelPluginBindingsFor(channel *model.Channel, generation *jsplugin.RoutingGeneration) harnessChannelPluginBindings {
	if plugin, ok := generation.GetByChannelType(channel.Type); ok {
		return harnessChannelPluginBindings{PluginKeys: []string{plugin.Meta.Key}, RequiresPlugin: true}
	}
	if channel.Type != constant.ChannelTypeTaskPlugin && channel.Type != constant.ChannelTypeNewAPI {
		return harnessChannelPluginBindings{}
	}
	var channelSettings dto.ChannelSettings
	if channel.Setting != nil && *channel.Setting != "" {
		if err := common.Unmarshal([]byte(*channel.Setting), &channelSettings); err != nil {
			return harnessChannelPluginBindings{RequiresPlugin: channel.Type == constant.ChannelTypeTaskPlugin}
		}
	}
	pluginKeys := channelSettings.TaskPluginBindings()
	return harnessChannelPluginBindings{
		PluginKeys:     pluginKeys,
		RequiresPlugin: channel.Type == constant.ChannelTypeTaskPlugin || len(pluginKeys) > 0,
	}
}

func harnessPluginProvidesModel(generation *jsplugin.RoutingGeneration, pluginKey, modelName string) bool {
	for _, plugin := range generation.PluginsByModel(modelName) {
		if plugin.Meta.Key == pluginKey {
			return true
		}
	}
	target, resolved := model.ResolveTaskModelAlias(generation, modelName)
	return resolved && target.PluginKey == pluginKey
}

// harnessCatalogPricing is the price view Harness bills from. It replaces the
// former ratio fields: prices are absolute money amounts in the instance
// display currency, and an absent field means "not configured", never "free".
type harnessCatalogPricing struct {
	Mode          string                               `json:"mode"`
	Currency      string                               `json:"currency"`
	Unit          string                               `json:"unit,omitempty"`
	Input         *float64                             `json:"input,omitempty"`
	Output        *float64                             `json:"output,omitempty"`
	CacheRead     *float64                             `json:"cache_read,omitempty"`
	CacheWrite    *float64                             `json:"cache_write,omitempty"`
	CacheWrite1h  *float64                             `json:"cache_write_1h,omitempty"`
	ImageInput    *float64                             `json:"image_input,omitempty"`
	Price         *float64                             `json:"price,omitempty"`
	Expression    string                               `json:"expression,omitempty"`
	UsageSchema   map[string]jsplugin.UsageFieldSchema `json:"usage_schema,omitempty"`
	UsageExamples []jsplugin.UsageExample              `json:"usage_examples,omitempty"`
}

type harnessCatalogModel struct {
	Name           string                `json:"name"`
	UpstreamModels []string              `json:"upstream_models"`
	Capabilities   []string              `json:"capabilities"`
	Pricing        harnessCatalogPricing `json:"pricing"`
	// Plugin variants reuse the same price shape per reachable task plugin.
	BillingPluginVariants []harnessCatalogPluginVariant `json:"billing_plugin_variants,omitempty"`
}

// harnessCatalogPluginVariant mirrors the plugin-specific billing of a model
// that is served by task plugins, where each plugin may bill differently.
type harnessCatalogPluginVariant struct {
	PluginKey  string                `json:"plugin_key"`
	PluginName string                `json:"plugin_name"`
	Pricing    harnessCatalogPricing `json:"pricing"`
}

const (
	harnessCatalogPricingModeToken      = "token"
	harnessCatalogPricingModeFixed      = "fixed"
	harnessCatalogPricingModeTieredExpr = "tiered_expr"
)

const (
	harnessCatalogTokenPriceUnit   = "1M_tokens"
	harnessCatalogRequestPriceUnit = "request"
)

// harnessCatalogDisplayCurrency returns the instance display currency and the
// rate that converts the canonical USD price into it. It mirrors the admin
// pricing UI and model marketplace so catalog numbers match what an operator
// sees there. TOKENS mode has no monetary display, so prices stay in USD.
func harnessCatalogDisplayCurrency() (string, float64) {
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		return "CNY", operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate)
	case operation_setting.QuotaDisplayTypeCustom:
		return operation_setting.GetCurrencySymbol(), operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate)
	default:
		return "USD", 1
	}
}

// harnessCatalogTokenPricing reports per-1M-token prices in the display
// currency. Lanes the operator did not configure stay nil so Harness can tell
// "not applicable" apart from an explicit free price.
func harnessCatalogTokenPricing(billingName, currency string, rate float64) harnessCatalogPricing {
	unitPrices := model.DescribeModelUnitPrices(billingName)
	return harnessCatalogPricing{
		Mode:         harnessCatalogPricingModeToken,
		Currency:     currency,
		Unit:         harnessCatalogTokenPriceUnit,
		Input:        scaleHarnessCatalogPrice(unitPrices.Input, rate),
		Output:       scaleHarnessCatalogPrice(unitPrices.Output, rate),
		CacheRead:    scaleHarnessCatalogPrice(unitPrices.CacheRead, rate),
		CacheWrite:   scaleHarnessCatalogPrice(unitPrices.CacheWrite, rate),
		CacheWrite1h: scaleHarnessCatalogPrice(unitPrices.CacheWrite1h, rate),
		ImageInput:   scaleHarnessCatalogPrice(unitPrices.ImageInput, rate),
	}
}

func scaleHarnessCatalogPrice(value *float64, rate float64) *float64 {
	if value == nil || rate == 1 {
		return value
	}
	scaled := *value * rate
	return &scaled
}

func GetHarnessCatalog(c *gin.Context) {
	token, err := model.GetTokenById(c.GetInt("token_id"))
	if err != nil {
		common.ApiErrorStatus(c, http.StatusUnauthorized, common.NewMessage("Invalid service token"))
		return
	}
	if token.ModelLimitsEnabled {
		common.ApiErrorStatus(c, http.StatusForbidden, common.NewMessage("Harness catalog requires a full-model service token"))
		return
	}
	routing := setting.GetIntelligentRoutingConfig()
	authorizedGroups := routing.TokenGroupAccess[token.Id]
	if len(authorizedGroups) == 0 {
		common.ApiErrorStatus(c, http.StatusForbidden, common.NewMessage("Token has no authorized resource pools"))
		return
	}
	authorizedSet := make(map[string]struct{}, len(authorizedGroups))
	for _, groupID := range authorizedGroups {
		authorizedSet[groupID] = struct{}{}
	}
	if len(authorizedSet) == 0 {
		common.ApiErrorStatus(c, http.StatusForbidden, common.NewMessage("Token has no resource pools available"))
		return
	}
	channels, err := model.GetAllChannelsWithoutKeys()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pricingByModel := make(map[string]model.Pricing)
	for _, pricing := range model.GetPricing() {
		pricingByModel[pricing.ModelName] = pricing
	}
	pluginGeneration := jsplugin.DefaultRegistry.Generation()
	channelByID := make(map[int]*model.Channel, len(channels))
	channelPluginsByID := make(map[int]harnessChannelPluginBindings, len(channels))
	for _, channel := range channels {
		if channel != nil {
			channelByID[channel.Id] = channel
			channelPluginsByID[channel.Id] = harnessChannelPluginBindingsFor(channel, pluginGeneration)
		}
	}
	enabledAbilities := model.GetAllEnableAbilities()
	eligiblePricingGroups := make(map[string]struct{}, len(routing.PricingGroupOrder))
	for _, groupID := range routing.PricingGroupOrder {
		if ratio_setting.ContainsGroupRatio(groupID) {
			eligiblePricingGroups[groupID] = struct{}{}
		}
	}
	groupModels := make(map[string]map[string][]string)
	groupPricingByGroup := make(map[string]map[string]model.Pricing)
	groupRoutePricingByGroup := make(map[string]map[string]*harnessRoutePricingProviders)
	collectGroupModels := func(groupID string, channelIDs []int) {
		for _, channelID := range channelIDs {
			channel := channelByID[channelID]
			if channel == nil || channel.Status != common.ChannelStatusEnabled {
				continue
			}
			for _, modelName := range channel.GetModels() {
				var supportedAbilities []model.Ability
				for _, ability := range enabledAbilities {
					if _, enabled := eligiblePricingGroups[ability.Group]; enabled && ability.ChannelId == channelID && (ability.Model == modelName || ratio_setting.RoutingMatchModelName(ability.Model) == ratio_setting.RoutingMatchModelName(modelName)) {
						supportedAbilities = append(supportedAbilities, ability)
					}
				}
				if len(supportedAbilities) == 0 {
					continue
				}
				if groupModels[groupID] == nil {
					groupModels[groupID] = make(map[string][]string)
					groupPricingByGroup[groupID] = make(map[string]model.Pricing)
					groupRoutePricingByGroup[groupID] = make(map[string]*harnessRoutePricingProviders)
				}
				groupModels[groupID][modelName] = append(groupModels[groupID][modelName], mappedModelName(channel, modelName))
				routePricing := groupRoutePricingByGroup[groupID][modelName]
				if routePricing == nil {
					routePricing = &harnessRoutePricingProviders{PluginKeys: make(map[string]struct{})}
					groupRoutePricingByGroup[groupID][modelName] = routePricing
				}
				channelPlugins := channelPluginsByID[channelID]
				if !channelPlugins.RequiresPlugin {
					routePricing.HasBaseRoute = true
				} else if len(channelPlugins.PluginKeys) == 0 {
					routePricing.HasUnboundTaskPluginRoute = true
				} else {
					matchedPlugin := false
					for _, pluginKey := range channelPlugins.PluginKeys {
						if !harnessPluginProvidesModel(pluginGeneration, pluginKey, modelName) {
							continue
						}
						routePricing.PluginKeys[pluginKey] = struct{}{}
						matchedPlugin = true
					}
					if !matchedPlugin {
						routePricing.HasUnboundTaskPluginRoute = true
					}
				}
				pricing, exists := pricingByModel[modelName]
				if exists {
					pricing.SupportedEndpointTypes = nil
					for _, ability := range supportedAbilities {
						for _, endpointType := range common.GetEndpointTypesByChannelType(channel.Type, ability.Model) {
							if !slices.Contains(pricing.SupportedEndpointTypes, endpointType) {
								pricing.SupportedEndpointTypes = append(pricing.SupportedEndpointTypes, endpointType)
							}
						}
					}
					groupPricingByGroup[groupID][modelName] = pricing
				}
			}
		}
	}
	for _, group := range routing.Groups {
		if _, allowed := authorizedSet[group.ID]; !allowed {
			continue
		}
		collectGroupModels(group.ID, group.ChannelOrder)
	}
	if routing.GlobalChannelOrder != nil {
		if _, allowed := authorizedSet[setting.IntelligentRoutingGlobalGroupID]; allowed {
			collectGroupModels(setting.IntelligentRoutingGlobalGroupID, routing.GlobalChannelOrder)
		}
	}

	response := harnessCatalogResponse{Groups: make([]harnessCatalogGroup, 0, len(authorizedGroups))}
	if routing.GlobalChannelOrder != nil {
		if _, allowed := authorizedSet[setting.IntelligentRoutingGlobalGroupID]; allowed {
			models, unpricedModels := buildHarnessCatalogModels(
				groupModels[setting.IntelligentRoutingGlobalGroupID],
				groupPricingByGroup[setting.IntelligentRoutingGlobalGroupID],
				groupRoutePricingByGroup[setting.IntelligentRoutingGlobalGroupID],
				pluginGeneration,
			)
			response.Groups = append(response.Groups, harnessCatalogGroup{
				ID:             setting.IntelligentRoutingGlobalGroupID,
				Name:           setting.IntelligentRoutingGlobalGroupName,
				Models:         models,
				UnpricedModels: unpricedModels,
			})
		}
	}
	for _, group := range routing.Groups {
		if _, allowed := authorizedSet[group.ID]; !allowed {
			continue
		}
		models, unpricedModels := buildHarnessCatalogModels(
			groupModels[group.ID],
			groupPricingByGroup[group.ID],
			groupRoutePricingByGroup[group.ID],
			pluginGeneration,
		)
		response.Groups = append(response.Groups, harnessCatalogGroup{
			ID:             group.ID,
			Name:           group.Name,
			Models:         models,
			UnpricedModels: unpricedModels,
		})
	}
	response.QuotaPerUnit = common.QuotaPerUnit
	response.PriceUnit = "display_currency"
	common.ApiSuccess(c, response)
}

func buildHarnessCatalogModels(
	upstreamNames map[string][]string,
	pricingByModel map[string]model.Pricing,
	routePricingByModel map[string]*harnessRoutePricingProviders,
	pluginGeneration *jsplugin.RoutingGeneration,
) ([]harnessCatalogModel, []harnessCatalogUnpricedModel) {
	models := make([]harnessCatalogModel, 0, len(upstreamNames))
	unpricedModels := make([]harnessCatalogUnpricedModel, 0)
	for modelName, upstreamModels := range upstreamNames {
		pricing, exists := pricingByModel[modelName]
		if !exists {
			continue
		}
		upstreamModels = slices.Clone(upstreamModels)
		slices.Sort(upstreamModels)
		upstreamModels = slices.Compact(upstreamModels)
		pricing = addHarnessTaskAliasPluginPricing(pricing, modelName, pluginGeneration)
		routePricing := routePricingByModel[modelName]
		billingName := harnessCatalogBillingName(modelName, pluginGeneration)
		if !hasHarnessCatalogPricing(billingName, pricing, routePricing) {
			unpricedModels = append(unpricedModels, harnessCatalogUnpricedModel{
				Name:           modelName,
				UpstreamModels: upstreamModels,
				Reason:         "pricing_not_configured",
			})
			continue
		}
		capabilities := make([]string, len(pricing.SupportedEndpointTypes))
		for i, endpointType := range pricing.SupportedEndpointTypes {
			capabilities[i] = string(endpointType)
		}
		pluginKeys := map[string]struct{}(nil)
		if routePricing != nil {
			pluginKeys = routePricing.PluginKeys
		}
		models = append(models, harnessCatalogModel{
			Name:                  modelName,
			UpstreamModels:        upstreamModels,
			Capabilities:          capabilities,
			Pricing:               resolveHarnessCatalogPricing(pricing, billingName),
			BillingPluginVariants: buildHarnessCatalogPluginVariants(pricing.BillingPluginVariants, pluginKeys, billingName),
		})
	}
	slices.SortFunc(models, func(a, b harnessCatalogModel) int {
		return strings.Compare(a.Name, b.Name)
	})
	slices.SortFunc(unpricedModels, func(a, b harnessCatalogUnpricedModel) int {
		return strings.Compare(a.Name, b.Name)
	})
	return models, unpricedModels
}

func harnessCatalogBillingName(modelName string, generation *jsplugin.RoutingGeneration) string {
	if target, resolved := model.ResolveTaskModelAlias(generation, modelName); resolved && target.Declared != "" {
		return relayhelper.ResolveBillingModelName(target.Declared)
	}
	return relayhelper.ResolveBillingModelName(modelName)
}

func addHarnessTaskAliasPluginPricing(pricing model.Pricing, modelName string, generation *jsplugin.RoutingGeneration) model.Pricing {
	target, resolved := model.ResolveTaskModelAlias(generation, modelName)
	if !resolved || target.Declared == "" || target.PluginKey == "" {
		return pricing
	}
	if _, exists := harnessPluginPricingVariant(pricing.BillingPluginVariants, target.PluginKey); exists {
		return pricing
	}
	plugin, exists := generation.Get(target.PluginKey)
	if !exists {
		return pricing
	}
	schema, examples := plugin.Meta.UsageForModels(target.Declared, modelName)
	if schema == nil {
		schema = map[string]jsplugin.UsageFieldSchema{}
	}
	expression, hasExpression := billing_setting.ResolveTaskBillingExpr(target.PluginKey, modelName, target.Declared)
	mode := billing_setting.BillingModeRatio
	if hasExpression || billing_setting.GetBillingMode(modelName) == billing_setting.BillingModeTieredExpr || billing_setting.GetBillingMode(target.Declared) == billing_setting.BillingModeTieredExpr {
		mode = billing_setting.BillingModeTieredExpr
	}
	if mode == billing_setting.BillingModeTieredExpr && !billing_setting.TaskExprCompatible(expression, schema) {
		expression = ""
	}
	pricing.BillingPluginVariants = append(pricing.BillingPluginVariants, model.PricingPluginVariant{
		PluginKey:            target.PluginKey,
		PluginName:           plugin.Meta.Name,
		Icon:                 plugin.Meta.Icon,
		BillingExpr:          expression,
		BillingMode:          mode,
		BillingUsageSchema:   jsplugin.CloneUsageSchema(schema),
		BillingUsageExamples: jsplugin.CloneUsageExamples(examples),
	})
	return pricing
}

func hasHarnessCatalogPricing(billingName string, pricing model.Pricing, routes *harnessRoutePricingProviders) bool {
	basePricing := hasHarnessBasePricing(billingName)
	if routes == nil {
		return basePricing
	}
	if routes.HasUnboundTaskPluginRoute || routes.HasBaseRoute && !basePricing {
		return false
	}
	for pluginKey := range routes.PluginKeys {
		variant, found := harnessPluginPricingVariant(pricing.BillingPluginVariants, pluginKey)
		if !found {
			if !hasHarnessBasePricingForTask(billingName) {
				return false
			}
			continue
		}
		if variant.BillingMode == billing_setting.BillingModeTieredExpr {
			if !hasHarnessPluginExpression(variant) {
				return false
			}
			continue
		}
		if !hasHarnessBasePricingForTask(billingName) {
			return false
		}
	}
	if routes.HasBaseRoute || len(routes.PluginKeys) == 0 {
		return basePricing
	}
	return true
}

func hasHarnessBasePricing(modelName string) bool {
	mode := billing_setting.GetBillingMode(modelName)
	if mode == billing_setting.BillingModeTieredExpr {
		expression, exists := billing_setting.GetBillingExpr(modelName)
		return exists && strings.TrimSpace(expression) != ""
	}
	if price, configured := ratio_setting.GetModelPrice(modelName, false); configured {
		return isValidHarnessPrice(price)
	}
	if ratio_setting.HasConfiguredModelRatio(modelName) {
		ratio, _, _ := ratio_setting.GetModelRatio(modelName)
		return isValidHarnessPrice(ratio)
	}
	return false
}

func hasHarnessBasePricingForTask(modelName string) bool {
	mode := billing_setting.GetBillingMode(modelName)
	if mode == billing_setting.BillingModeTieredExpr {
		expression, exists := billing_setting.GetBillingExpr(modelName)
		return exists && strings.TrimSpace(expression) != "" && !billingexpr.UsesFixedPricing(expression)
	}
	return hasHarnessBasePricing(modelName)
}

func hasHarnessPluginExpression(variant model.PricingPluginVariant) bool {
	expression := strings.TrimSpace(variant.BillingExpr)
	return expression != "" && !billingexpr.UsesFixedPricing(expression) &&
		billing_setting.TaskExprCompatible(expression, variant.BillingUsageSchema)
}

func harnessPluginPricingVariant(variants []model.PricingPluginVariant, pluginKey string) (model.PricingPluginVariant, bool) {
	for _, variant := range variants {
		if variant.PluginKey == pluginKey {
			return variant, true
		}
	}
	return model.PricingPluginVariant{}, false
}

// resolveHarnessCatalogPricing selects the model's billing mode with the
// precedence tiered expression > fixed price > token ratio and returns the
// prices Harness bills from. Token and fixed prices are converted into the
// instance display currency; a tiered expression already yields USD, so its
// output is passed through unscaled.
func resolveHarnessCatalogPricing(pricing model.Pricing, billingName string) harnessCatalogPricing {
	currency, rate := harnessCatalogDisplayCurrency()
	if billing_setting.GetBillingMode(billingName) == billing_setting.BillingModeTieredExpr {
		// The expression engine prices lanes in USD (see pkg/billingexpr/expr.md),
		// so the expression output is never scaled by the display-currency rate.
		return harnessCatalogPricing{
			Mode:          harnessCatalogPricingModeTieredExpr,
			Currency:      "USD",
			Expression:    billingExprOrEmpty(billingName),
			UsageSchema:   pricing.BillingUsageSchema,
			UsageExamples: pricing.BillingUsageExamples,
		}
	}
	if price, configured := ratio_setting.GetModelPrice(billingName, false); configured {
		return harnessCatalogPricing{
			Mode:     harnessCatalogPricingModeFixed,
			Currency: currency,
			Unit:     harnessCatalogRequestPriceUnit,
			Price:    scaleHarnessCatalogPrice(&price, rate),
		}
	}
	return harnessCatalogTokenPricing(billingName, currency, rate)
}

func isValidHarnessPrice(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

// billingExprOrEmpty returns the configured billing expression, if the model
// uses expression-based billing.
func billingExprOrEmpty(modelName string) string {
	expr, ok := billing_setting.GetBillingExpr(modelName)
	if !ok {
		return ""
	}
	return expr
}

// buildHarnessCatalogPluginVariants exposes only providers reachable through
// the current resource pool so Harness sees the matching plugin prices. A
// ratio-mode variant bills the model's token lanes, so it carries the model's
// token unit prices; an expression variant carries its own expression and usage
// schema.
func buildHarnessCatalogPluginVariants(
	variants []model.PricingPluginVariant,
	pluginKeys map[string]struct{},
	billingName string,
) []harnessCatalogPluginVariant {
	if len(variants) == 0 || len(pluginKeys) == 0 {
		return nil
	}
	currency, rate := harnessCatalogDisplayCurrency()
	result := make([]harnessCatalogPluginVariant, 0, len(pluginKeys))
	for _, variant := range variants {
		if _, reachable := pluginKeys[variant.PluginKey]; !reachable {
			continue
		}
		variantPricing := harnessCatalogTokenPricing(billingName, currency, rate)
		if variant.BillingMode == billing_setting.BillingModeTieredExpr {
			variantPricing = harnessCatalogPricing{
				Mode:          harnessCatalogPricingModeTieredExpr,
				Currency:      "USD",
				Expression:    variant.BillingExpr,
				UsageSchema:   variant.BillingUsageSchema,
				UsageExamples: variant.BillingUsageExamples,
			}
		}
		result = append(result, harnessCatalogPluginVariant{
			PluginKey:  variant.PluginKey,
			PluginName: variant.PluginName,
			Pricing:    variantPricing,
		})
	}
	slices.SortFunc(result, func(a, b harnessCatalogPluginVariant) int {
		return strings.Compare(a.PluginKey, b.PluginKey)
	})
	return result
}

func mappedModelName(channel *model.Channel, modelName string) string {
	mapping := channel.GetModelMapping()
	if strings.TrimSpace(mapping) == "" || mapping == "{}" {
		return modelName
	}
	var models map[string]string
	if err := common.UnmarshalJsonStr(mapping, &models); err != nil {
		return modelName
	}
	if mapped, ok := models[modelName]; ok && mapped != "" {
		return mapped
	}
	return modelName
}
