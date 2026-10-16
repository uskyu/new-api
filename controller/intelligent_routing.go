package controller

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type intelligentRoutingResponse struct {
	GlobalChannelOrder []int                             `json:"global_channel_order"`
	Groups             []setting.IntelligentRoutingGroup `json:"groups"`
	Channels           []intelligentRoutingChannel       `json:"channels"`
	CostGroups         map[string]float64                `json:"cost_groups"`
}

type intelligentRoutingChannel struct {
	ID                int                 `json:"id"`
	Name              string              `json:"name"`
	Status            int                 `json:"status"`
	Models            []string            `json:"models"`
	Priority          int64               `json:"priority"`
	EnabledCostGroups map[string][]string `json:"enabled_cost_groups"`
}

func GetIntelligentRouting(c *gin.Context) {
	channels, err := model.GetAllChannelsWithoutKeys()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	routingConfig := setting.GetIntelligentRoutingConfig()
	globalOrder := routingConfig.GlobalChannelOrder
	if globalOrder == nil {
		globalOrder = []int{}
	}
	response := intelligentRoutingResponse{
		GlobalChannelOrder: globalOrder,
		Groups:             routingConfig.Groups,
		Channels:           make([]intelligentRoutingChannel, 0, len(channels)),
		CostGroups:         ratio_setting.GetGroupRatioCopy(),
	}
	costGroups := make(map[string]struct{})
	if routingConfig.GlobalChannelOrder != nil {
		costGroups[routingConfig.GlobalCostGroup] = struct{}{}
	}
	for _, group := range routingConfig.Groups {
		costGroups[group.CostGroup] = struct{}{}
	}
	for name := range ratio_setting.GetGroupRatioCopy() {
		costGroups[name] = struct{}{}
	}
	for _, channel := range channels {
		if channel == nil || channel.Status != common.ChannelStatusEnabled {
			continue
		}
		models := channel.GetModels()
		enabledCostGroups := make(map[string][]string)
		for costGroup := range costGroups {
			for _, modelName := range models {
				if model.IsChannelEnabledForGroupModel(costGroup, modelName, channel.Id) {
					enabledCostGroups[costGroup] = append(enabledCostGroups[costGroup], modelName)
				}
			}
		}
		response.Channels = append(response.Channels, intelligentRoutingChannel{
			ID:                channel.Id,
			Name:              channel.Name,
			Status:            channel.Status,
			Models:            models,
			Priority:          channel.GetPriority(),
			EnabledCostGroups: enabledCostGroups,
		})
	}
	common.ApiSuccess(c, response)
}

type updateIntelligentRoutingRequest struct {
	GlobalChannelOrder []int                             `json:"global_channel_order"`
	GlobalCostGroup    string                            `json:"global_cost_group"`
	Groups             []setting.IntelligentRoutingGroup `json:"groups"`
}

func UpdateIntelligentRouting(c *gin.Context) {
	var request updateIntelligentRoutingRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Invalid parameters"))
		return
	}
	config := setting.IntelligentRoutingConfig{
		GlobalChannelOrder: request.GlobalChannelOrder,
		GlobalCostGroup:    request.GlobalCostGroup,
		Groups:             request.Groups,
	}
	if err := setting.ValidateIntelligentRoutingConfig(config); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage(err.Error()))
		return
	}
	if config.GlobalChannelOrder == nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("global_channel_order must be provided"))
		return
	}
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
	if config.GlobalChannelOrder != nil && !ratio_setting.ContainsGroupRatio(config.GlobalCostGroup) {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Unknown global cost group: "+config.GlobalCostGroup))
		return
	}
	for _, group := range config.Groups {
		if !ratio_setting.ContainsGroupRatio(group.CostGroup) {
			common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Unknown cost group: "+group.CostGroup))
			return
		}
	}
	if err := model.UpdateOption(setting.IntelligentRoutingOptionKey, string(routingJSON)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"global_channel_order": config.GlobalChannelOrder, "groups": config.Groups})
}

func channelHasEnabledGroupModelAbility(channelID int, groupID string, channelModels []string) bool {
	for _, modelName := range channelModels {
		if model.IsChannelEnabledForGroupModel(groupID, modelName, channelID) {
			return true
		}
	}
	return false
}

func validateIntelligentRoutingChannels(config setting.IntelligentRoutingConfig) error {
	channels, err := model.GetAllChannelsWithoutKeys()
	if err != nil {
		return err
	}
	channelByID := make(map[int]*model.Channel, len(channels))
	for _, channel := range channels {
		if channel != nil {
			channelByID[channel.Id] = channel
		}
	}
	for _, channelID := range config.GlobalChannelOrder {
		channel := channelByID[channelID]
		if channel == nil {
			return common.NewMessage("Global order contains an unknown channel ID")
		}
		if channel.Status != common.ChannelStatusEnabled {
			return common.NewMessage("Global routing can only use enabled channels")
		}
		if !channelHasEnabledGroupModelAbility(channelID, config.GlobalCostGroup, channel.GetModels()) {
			return common.NewMessage("Global route channel has no enabled model ability in cost group: " + config.GlobalCostGroup)
		}
	}
	if config.GlobalChannelOrder != nil && !ratio_setting.ContainsGroupRatio(config.GlobalCostGroup) {
		return common.NewMessage("Global route references an unknown cost group: " + config.GlobalCostGroup)
	}
	for _, group := range config.Groups {
		if len(group.ChannelOrder) == 0 {
			return common.NewMessage("Each resource pool must contain at least one channel")
		}
		if !ratio_setting.ContainsGroupRatio(group.CostGroup) {
			return common.NewMessage("Resource pool references an unknown cost group: " + group.CostGroup)
		}
		for _, channelID := range group.ChannelOrder {
			channel := channelByID[channelID]
			if channel == nil {
				return common.NewMessage("Resource pool contains an unknown channel ID")
			}
			if channel.Status != common.ChannelStatusEnabled {
				return common.NewMessage("Resource pools can only use enabled channels")
			}
			if !channelHasEnabledGroupModelAbility(channelID, group.CostGroup, channel.GetModels()) {
				return common.NewMessage("Resource pool channel has no enabled model ability in cost group: " + group.CostGroup)
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
	owner, err := model.GetUserById(token.UserId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ownerGroups := service.GetUserUsableGroups(owner.Group)
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
		costGroup, _ := setting.GetIntelligentRoutingCostGroup(group)
		if _, usable := ownerGroups[costGroup]; !usable {
			common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Resource pool cost group is not available to the token owner: "+costGroup))
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
	common.ApiSuccess(c, tokenRoutingAccessResponse{TokenID: tokenID, Groups: request.Groups})
}

// GetHarnessCatalog returns only the logical model and cost data needed by Harness.
// Channel credentials, addresses, overrides, and channel IDs are never serialized.
type harnessCatalogResponse struct {
	QuotaPerUnit float64               `json:"quota_per_unit"`
	PriceUnit    string                `json:"price_unit"`
	Groups       []harnessCatalogGroup `json:"groups"`
}

type harnessCatalogGroup struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	CostGroup string                `json:"cost_group"`
	Ratio     float64               `json:"ratio"`
	Models    []harnessCatalogModel `json:"models"`
}

type harnessCatalogModel struct {
	Name                 string   `json:"name"`
	UpstreamModels       []string `json:"upstream_models"`
	Capabilities         []string `json:"capabilities"`
	QuotaType            int      `json:"quota_type"`
	ModelRatio           float64  `json:"model_ratio"`
	ModelPrice           float64  `json:"model_price"`
	CompletionRatio      float64  `json:"completion_ratio"`
	CacheRatio           *float64 `json:"cache_ratio,omitempty"`
	CreateCacheRatio     *float64 `json:"create_cache_ratio,omitempty"`
	ImageRatio           *float64 `json:"image_ratio,omitempty"`
	AudioRatio           *float64 `json:"audio_ratio,omitempty"`
	AudioCompletionRatio *float64 `json:"audio_completion_ratio,omitempty"`
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
	channelByID := make(map[int]*model.Channel, len(channels))
	for _, channel := range channels {
		if channel != nil {
			channelByID[channel.Id] = channel
		}
	}
	groupModels := make(map[string]map[string][]string)
	collectGroupModels := func(groupID string, channelIDs []int) {
		for _, channelID := range channelIDs {
			channel := channelByID[channelID]
			if channel == nil || channel.Status != common.ChannelStatusEnabled {
				continue
			}
			for _, modelName := range channel.GetModels() {
				costGroup := routing.GlobalCostGroup
				if groupID != setting.IntelligentRoutingGlobalGroupID {
					groupConfig := findIntelligentRoutingGroup(routing, groupID)
					if groupConfig == nil {
						continue
					}
					costGroup = groupConfig.CostGroup
				}
				if !model.IsChannelEnabledForGroupModel(costGroup, modelName, channelID) {
					continue
				}
				if groupModels[groupID] == nil {
					groupModels[groupID] = make(map[string][]string)
				}
				groupModels[groupID][modelName] = append(groupModels[groupID][modelName], mappedModelName(channel, modelName))
			}
		}
	}
	for _, group := range routing.Groups {
		if _, allowed := authorizedSet[group.ID]; !allowed {
			continue
		}
		collectGroupModels(group.ID, group.ChannelOrder)
	}
	response := harnessCatalogResponse{Groups: make([]harnessCatalogGroup, 0, len(authorizedGroups))}
	if routing.GlobalChannelOrder != nil {
		if _, allowed := authorizedSet[setting.IntelligentRoutingGlobalGroupID]; allowed {
			collectGroupModels(setting.IntelligentRoutingGlobalGroupID, routing.GlobalChannelOrder)
			globalGroup := harnessCatalogGroup{
				ID:        setting.IntelligentRoutingGlobalGroupID,
				Name:      setting.IntelligentRoutingGlobalGroupName,
				CostGroup: routing.GlobalCostGroup,
				Ratio:     ratio_setting.GetGroupRatio(routing.GlobalCostGroup),
				Models:    buildHarnessCatalogModels(groupModels[setting.IntelligentRoutingGlobalGroupID], pricingByModel),
			}
			response.Groups = append(response.Groups, globalGroup)
		}
	}
	for _, group := range routing.Groups {
		if _, allowed := authorizedSet[group.ID]; !allowed {
			continue
		}
		groupPricing := make(map[string]model.Pricing, len(groupModels[group.ID]))
		for modelName := range groupModels[group.ID] {
			pricing, ok := pricingByModel[modelName]
			if !ok {
				continue
			}
			pricing.SupportedEndpointTypes = nil
			for _, channelID := range group.ChannelOrder {
				channel := channelByID[channelID]
				if channel == nil || channel.Status != common.ChannelStatusEnabled || !slices.Contains(channel.GetModels(), modelName) {
					continue
				}
				for _, endpointType := range common.GetEndpointTypesByChannelType(channel.Type, modelName) {
					if !slices.Contains(pricing.SupportedEndpointTypes, endpointType) {
						pricing.SupportedEndpointTypes = append(pricing.SupportedEndpointTypes, endpointType)
					}
				}
			}
			groupPricing[modelName] = pricing
		}
		response.Groups = append(response.Groups, harnessCatalogGroup{
			ID:        group.ID,
			Name:      group.Name,
			CostGroup: group.CostGroup,
			Ratio:     ratio_setting.GetGroupRatio(group.CostGroup),
			Models:    buildHarnessCatalogModels(groupModels[group.ID], groupPricing),
		})
	}
	response.QuotaPerUnit = common.QuotaPerUnit
	response.PriceUnit = "new_api_quota"
	common.ApiSuccess(c, response)
}

func buildHarnessCatalogModels(upstreamNames map[string][]string, pricingByModel map[string]model.Pricing) []harnessCatalogModel {
	models := make([]harnessCatalogModel, 0, len(upstreamNames))
	for modelName, upstreamModels := range upstreamNames {
		pricing, ok := pricingByModel[modelName]
		if !ok {
			continue
		}
		upstreamModels = slices.Clone(upstreamModels)
		slices.Sort(upstreamModels)
		upstreamModels = slices.Compact(upstreamModels)
		capabilities := make([]string, len(pricing.SupportedEndpointTypes))
		for i, endpointType := range pricing.SupportedEndpointTypes {
			capabilities[i] = string(endpointType)
		}
		models = append(models, harnessCatalogModel{
			Name:                 modelName,
			UpstreamModels:       upstreamModels,
			Capabilities:         capabilities,
			QuotaType:            pricing.QuotaType,
			ModelRatio:           pricing.ModelRatio,
			ModelPrice:           pricing.ModelPrice,
			CompletionRatio:      pricing.CompletionRatio,
			CacheRatio:           pricing.CacheRatio,
			CreateCacheRatio:     pricing.CreateCacheRatio,
			ImageRatio:           pricing.ImageRatio,
			AudioRatio:           pricing.AudioRatio,
			AudioCompletionRatio: pricing.AudioCompletionRatio,
		})
	}
	slices.SortFunc(models, func(a, b harnessCatalogModel) int {
		return strings.Compare(a.Name, b.Name)
	})
	return models
}

func findIntelligentRoutingGroup(config setting.IntelligentRoutingConfig, groupID string) *setting.IntelligentRoutingGroup {
	for i := range config.Groups {
		if config.Groups[i].ID == groupID {
			return &config.Groups[i]
		}
	}
	return nil
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
