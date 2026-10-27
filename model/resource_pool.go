package model

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
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
