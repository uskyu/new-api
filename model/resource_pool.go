package model

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// ResourcePoolModels returns the model ids one Harness resource pool can serve.
//
// A pool is an Intelligent Routing group: its candidate channels come from the
// group's configured channel order, and the models it can offer are exactly the
// union of those channels' enabled model lists. Deriving the answer instead of
// storing it means the catalog can never advertise a model the pool cannot
// route, and the platform's model list follows channel changes automatically.
//
// A declared model is only offered when the same channel also has an enabled
// ability for it. Channel declarations and abilities can drift (a manual DB
// edit, a failed ability refresh after an upstream sync), and an enabled
// channel's models string alone is not proof that the channel can actually
// serve the model.
func ResourcePoolModels(poolID string) []string {
	models, _ := ResourcePoolModelsWithError(poolID)
	return models
}

// ResourcePoolModelsWithError is ResourcePoolModels with the database error
// surfaced instead of collapsed into an empty pool. The catalog handler must
// answer 500 rather than publish an empty catalog and advance its version.
func ResourcePoolModelsWithError(poolID string) ([]string, error) {
	order, ok := setting.GetIntelligentRoutingGroupChannelOrder(poolID)
	if !ok || len(order) == 0 {
		return []string{}, nil
	}
	channels, err := GetChannelsByIds(order)
	if err != nil {
		common.SysLog(common.LogText("failed to load channels for resource pool %q: %v", poolID, err))
		return nil, err
	}
	channelByID := make(map[int]*Channel, len(channels))
	for _, channel := range channels {
		channelByID[channel.Id] = channel
	}

	enabledIDs := make([]int, 0, len(order))
	for _, channelID := range order {
		channel := channelByID[channelID]
		if channel != nil && channel.Status == common.ChannelStatusEnabled {
			enabledIDs = append(enabledIDs, channelID)
		}
	}
	if len(enabledIDs) == 0 {
		return []string{}, nil
	}

	// DB is nil in unit tests that only exercise the pure channel-order logic;
	// in that case preserve the historical behavior and skip the ability filter.
	var abilityModels map[int]map[string]struct{}
	var abilityNormalized map[int]map[string]struct{}
	if DB != nil {
		var abilities []Ability
		if err := DB.Where("channel_id IN ? AND enabled = ?", enabledIDs, true).Find(&abilities).Error; err != nil {
			common.SysLog(common.LogText("failed to load abilities for resource pool %q: %v", poolID, err))
			return nil, err
		}
		abilityModels = make(map[int]map[string]struct{}, len(enabledIDs))
		abilityNormalized = make(map[int]map[string]struct{}, len(enabledIDs))
		for _, ability := range abilities {
			name := strings.TrimSpace(ability.Model)
			if name == "" {
				continue
			}
			if abilityModels[ability.ChannelId] == nil {
				abilityModels[ability.ChannelId] = make(map[string]struct{})
				abilityNormalized[ability.ChannelId] = make(map[string]struct{})
			}
			abilityModels[ability.ChannelId][name] = struct{}{}
			if normalized := ratio_setting.RoutingMatchModelName(name); normalized != "" {
				abilityNormalized[ability.ChannelId][normalized] = struct{}{}
			}
		}
	}

	seen := make(map[string]struct{}, len(order))
	models := make([]string, 0, len(order))
	// Walk the configured order rather than the query result so the union stays
	// deterministic: the first channel that lists a model decides its position.
	for _, channelID := range order {
		channel := channelByID[channelID]
		if channel == nil || channel.Status != common.ChannelStatusEnabled {
			continue
		}
		for _, modelName := range channel.GetModels() {
			name := strings.TrimSpace(modelName)
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			if DB != nil && !resourcePoolAbilityAllows(abilityModels[channelID], abilityNormalized[channelID], name) {
				continue
			}
			seen[name] = struct{}{}
			models = append(models, name)
		}
	}
	sort.Strings(models)
	return models, nil
}

// resourcePoolAbilityAllows reports whether the channel has an enabled ability
// for the declared model. Exact names are checked first; the routing-normalized
// name covers the same aliases channel selection uses.
func resourcePoolAbilityAllows(exact, normalized map[string]struct{}, modelName string) bool {
	if _, ok := exact[modelName]; ok {
		return true
	}
	if normalizedName := ratio_setting.RoutingMatchModelName(modelName); normalizedName != "" {
		if _, ok := normalized[normalizedName]; ok {
			return true
		}
	}
	return false
}

// CountResourcePoolModelChannels counts how many enabled channels in the pool's
// configured order can actually serve modelName. It is the retry-budget input
// for a Harness call: the same model may be served by several channels in one
// pool, and every one of them is a candidate the relay may need to try after an
// upstream failure.
//
// A channel counts only when it is enabled and has an enabled ability for the
// model, which is the same predicate channel selection uses. Request-scoped
// filters (path, plugin identity, pricing-group opt-in) are deliberately not
// applied here: they can only narrow the candidate set, so counting them would
// risk underestimating the budget. An overestimate is harmless because selection
// itself stops as soon as the configured order is exhausted.
func CountResourcePoolModelChannels(poolID string, modelName string) int {
	poolID = strings.TrimSpace(poolID)
	modelName = strings.TrimSpace(modelName)
	if poolID == "" || modelName == "" || DB == nil {
		return 0
	}
	order, ok := setting.GetIntelligentRoutingGroupChannelOrder(poolID)
	if !ok || len(order) == 0 {
		return 0
	}
	channels, err := GetChannelsByIds(order)
	if err != nil {
		common.SysLog(common.LogText("failed to load channels for resource pool %q: %v", poolID, err))
		return 0
	}
	enabled := make(map[int]struct{}, len(channels))
	for _, channel := range channels {
		if channel != nil && channel.Status == common.ChannelStatusEnabled {
			enabled[channel.Id] = struct{}{}
		}
	}
	if len(enabled) == 0 {
		return 0
	}

	modelNames := []string{modelName}
	if normalized := ratio_setting.RoutingMatchModelName(modelName); normalized != "" && normalized != modelName {
		modelNames = append(modelNames, normalized)
	}
	var abilities []Ability
	if err := DB.Where("channel_id IN ? AND model IN ? AND enabled = ?", order, modelNames, true).Find(&abilities).Error; err != nil {
		common.SysLog(common.LogText("failed to load abilities for resource pool %q: %v", poolID, err))
		return 0
	}
	counted := make(map[int]struct{}, len(abilities))
	for _, ability := range abilities {
		if _, served := enabled[ability.ChannelId]; !served {
			continue
		}
		counted[ability.ChannelId] = struct{}{}
	}
	return len(counted)
}
