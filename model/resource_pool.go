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
// Only channels that exist and are enabled contribute: a disabled channel is
// temporarily out of service, so its models must not be offered. The result is
// sorted so the catalog fingerprint cannot depend on map or insert order.
func ResourcePoolModels(poolID string) []string {
	order, ok := setting.GetIntelligentRoutingGroupChannelOrder(poolID)
	if !ok || len(order) == 0 {
		return []string{}
	}
	channels, err := GetChannelsByIds(order)
	if err != nil {
		common.SysLog(common.LogText("failed to load channels for resource pool %q: %v", poolID, err))
		return []string{}
	}
	channelByID := make(map[int]*Channel, len(channels))
	for _, channel := range channels {
		channelByID[channel.Id] = channel
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
			seen[name] = struct{}{}
			models = append(models, name)
		}
	}
	sort.Strings(models)
	return models
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
