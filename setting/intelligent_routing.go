package setting

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const (
	IntelligentRoutingOptionKey  = "IntelligentRouting"
	IntelligentRoutingContextKey = "intelligent_routing_group"

	IntelligentRoutingGroupHeader        = "X-Route-Group"
	IntelligentRoutingChannelIDHeader    = "X-Route-Channel-ID"
	IntelligentRoutingGroupRatioHeader   = "X-Route-Group-Ratio"
	IntelligentRoutingPricingGroupHeader = "X-Route-Pricing-Group-ID"
)

type IntelligentRoutingGroup struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ChannelOrder []int  `json:"channel_order"`
}

type IntelligentRoutingConfig struct {
	PricingGroupOrder []string                  `json:"pricing_group_order"`
	Groups            []IntelligentRoutingGroup `json:"groups"`
}

var (
	intelligentRoutingMu          sync.RWMutex
	intelligentRoutingPricingRank = make(map[string]int)
	intelligentRoutingConfig      = IntelligentRoutingConfig{
		Groups: []IntelligentRoutingGroup{},
	}
	intelligentRoutingGroupIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

func GetIntelligentRoutingConfig() IntelligentRoutingConfig {
	intelligentRoutingMu.RLock()
	defer intelligentRoutingMu.RUnlock()

	return cloneIntelligentRoutingConfig(intelligentRoutingConfig)
}

// Cached O(1) rank lookup for opt-in procurement pricing groups.
func GetIntelligentRoutingPricingGroupRank(groupID string) (int, bool) {
	intelligentRoutingMu.RLock()
	defer intelligentRoutingMu.RUnlock()
	rank, ok := intelligentRoutingPricingRank[groupID]
	return rank, ok
}

func GetIntelligentRoutingGroupChannelOrder(groupID string) ([]int, bool) {
	intelligentRoutingMu.RLock()
	defer intelligentRoutingMu.RUnlock()
	for _, group := range intelligentRoutingConfig.Groups {
		if group.ID == groupID {
			order := make([]int, len(group.ChannelOrder))
			copy(order, group.ChannelOrder)
			return order, true
		}
	}
	return nil, false
}

func IntelligentRoutingConfig2JSONString(config IntelligentRoutingConfig) (string, error) {
	if err := ValidateIntelligentRoutingConfig(config); err != nil {
		return "", err
	}
	data, err := common.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ValidateIntelligentRoutingJSON(value string) error {
	var config IntelligentRoutingConfig
	if err := common.UnmarshalJsonStr(value, &config); err != nil {
		return err
	}
	return ValidateIntelligentRoutingConfig(config)
}

func UpdateIntelligentRoutingConfigJSON(value string) error {
	var config IntelligentRoutingConfig
	if err := common.UnmarshalJsonStr(value, &config); err != nil {
		return err
	}
	return UpdateIntelligentRoutingConfig(config)
}

func UpdateIntelligentRoutingConfig(config IntelligentRoutingConfig) error {
	if err := ValidateIntelligentRoutingConfig(config); err != nil {
		return err
	}
	intelligentRoutingMu.Lock()
	intelligentRoutingConfig = cloneIntelligentRoutingConfig(config)
	intelligentRoutingPricingRank = make(map[string]int, len(config.PricingGroupOrder))
	for i, id := range config.PricingGroupOrder {
		intelligentRoutingPricingRank[id] = i
	}
	intelligentRoutingMu.Unlock()
	return nil
}

func IntelligentRoutingGroups(config IntelligentRoutingConfig) map[string]string {
	groups := make(map[string]string, len(config.Groups))
	for _, group := range config.Groups {
		groups[group.ID] = group.Name
	}
	return groups
}

func ValidateIntelligentRoutingConfig(config IntelligentRoutingConfig) error {
	seenPricing := make(map[string]struct{}, len(config.PricingGroupOrder))
	for _, id := range config.PricingGroupOrder {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("pricing_group_order contains an empty ID")
		}
		if _, ok := seenPricing[id]; ok {
			return fmt.Errorf("duplicate pricing group ID: %s", id)
		}
		seenPricing[id] = struct{}{}
	}
	groupIDs := make(map[string]struct{}, len(config.Groups))
	for i, group := range config.Groups {
		if !intelligentRoutingGroupIDPattern.MatchString(group.ID) || group.ID == "auto" {
			return fmt.Errorf("groups[%d].id must contain 1-64 letters, numbers, underscores, or hyphens and cannot be auto", i)
		}
		if strings.TrimSpace(group.Name) == "" || len(group.Name) > 128 {
			return fmt.Errorf("groups[%d].name must contain 1-128 characters", i)
		}
		if _, exists := groupIDs[group.ID]; exists {
			return fmt.Errorf("duplicate routing group id: %s", group.ID)
		}
		groupIDs[group.ID] = struct{}{}
		if err := validateChannelOrder(fmt.Sprintf("groups[%d].channel_order", i), group.ChannelOrder); err != nil {
			return err
		}
	}
	return nil
}

func cloneIntelligentRoutingConfig(config IntelligentRoutingConfig) IntelligentRoutingConfig {
	cloned := IntelligentRoutingConfig{
		PricingGroupOrder: slices.Clone(config.PricingGroupOrder),
		Groups:            make([]IntelligentRoutingGroup, len(config.Groups)),
	}
	for i, group := range config.Groups {
		cloned.Groups[i] = group
		cloned.Groups[i].ChannelOrder = slices.Clone(group.ChannelOrder)
	}
	return cloned
}

func validateChannelOrder(field string, channelIDs []int) error {
	seen := make(map[int]struct{}, len(channelIDs))
	for _, channelID := range channelIDs {
		if channelID <= 0 {
			return fmt.Errorf("%s channel ids must be positive", field)
		}
		if _, exists := seen[channelID]; exists {
			return fmt.Errorf("%s contains duplicate channel id %d", field, channelID)
		}
		seen[channelID] = struct{}{}
	}
	return nil
}
