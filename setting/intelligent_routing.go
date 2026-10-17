package setting

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const (
	IntelligentRoutingOptionKey       = "IntelligentRouting"
	IntelligentRoutingContextKey      = "intelligent_routing_group"
	IntelligentRoutingGlobalGroupID   = "global"
	IntelligentRoutingGlobalGroupName = "Global priority"

	IntelligentRoutingGroupHeader           = "X-Route-Group"
	IntelligentRoutingDiscountPercentHeader = "X-Route-Discount-Percent"
	IntelligentRoutingTaxRatePercentHeader  = "X-Route-Tax-Rate-Percent"
)

type IntelligentRoutingGroup struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	DiscountPercent float64 `json:"discount_percent"`
	TaxRatePercent  float64 `json:"tax_rate_percent"`
	ChannelOrder    []int   `json:"channel_order"`
}

type IntelligentRoutingConfig struct {
	GlobalChannelOrder []int                     `json:"global_channel_order"`
	Groups             []IntelligentRoutingGroup `json:"groups"`
	TokenGroupAccess   map[int][]string          `json:"token_group_access,omitempty"`
}

var (
	intelligentRoutingMu     sync.RWMutex
	intelligentRoutingConfig = IntelligentRoutingConfig{
		GlobalChannelOrder: nil,
		Groups:             []IntelligentRoutingGroup{},
		TokenGroupAccess:   make(map[int][]string),
	}
	intelligentRoutingGroupIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

func GetIntelligentRoutingConfig() IntelligentRoutingConfig {
	intelligentRoutingMu.RLock()
	defer intelligentRoutingMu.RUnlock()

	return cloneIntelligentRoutingConfig(intelligentRoutingConfig)
}

func TokenHasIntelligentRoutingAccess(tokenID int) bool {
	intelligentRoutingMu.RLock()
	defer intelligentRoutingMu.RUnlock()
	return len(intelligentRoutingConfig.TokenGroupAccess[tokenID]) > 0
}

func TokenCanUseIntelligentRoutingGroup(tokenID int, groupID string) bool {
	intelligentRoutingMu.RLock()
	defer intelligentRoutingMu.RUnlock()
	return tokenCanUseIntelligentRoutingGroup(intelligentRoutingConfig, tokenID, groupID)
}

func tokenCanUseIntelligentRoutingGroup(config IntelligentRoutingConfig, tokenID int, groupID string) bool {
	return slices.Contains(config.TokenGroupAccess[tokenID], groupID) && intelligentRoutingGroupExists(config, groupID)
}

func TokenCanUseIntelligentRoutingGroupForConfig(config IntelligentRoutingConfig, tokenID int, groupID string) bool {
	return tokenCanUseIntelligentRoutingGroup(config, tokenID, groupID)
}

func intelligentRoutingGroupExists(config IntelligentRoutingConfig, groupID string) bool {
	if groupID == IntelligentRoutingGlobalGroupID {
		return config.GlobalChannelOrder != nil
	}
	for _, group := range config.Groups {
		if group.ID == groupID {
			return true
		}
	}
	return false
}

func GetIntelligentRoutingGroupChannelOrder(groupID string) ([]int, bool) {
	intelligentRoutingMu.RLock()
	defer intelligentRoutingMu.RUnlock()
	if groupID == IntelligentRoutingGlobalGroupID {
		if intelligentRoutingConfig.GlobalChannelOrder == nil {
			return nil, false
		}
		order := make([]int, len(intelligentRoutingConfig.GlobalChannelOrder))
		copy(order, intelligentRoutingConfig.GlobalChannelOrder)
		return order, true
	}
	for _, group := range intelligentRoutingConfig.Groups {
		if group.ID == groupID {
			order := make([]int, len(group.ChannelOrder))
			copy(order, group.ChannelOrder)
			return order, true
		}
	}
	return nil, false
}

func GetIntelligentRoutingGroupPricing(groupID string) (float64, float64, bool) {
	intelligentRoutingMu.RLock()
	defer intelligentRoutingMu.RUnlock()
	return intelligentRoutingGroupPricing(intelligentRoutingConfig, groupID)
}

func intelligentRoutingGroupPricing(config IntelligentRoutingConfig, groupID string) (float64, float64, bool) {
	if groupID == IntelligentRoutingGlobalGroupID {
		return 0, 0, config.GlobalChannelOrder != nil
	}
	for _, group := range config.Groups {
		if group.ID == groupID {
			return group.DiscountPercent, group.TaxRatePercent, true
		}
	}
	return 0, 0, false
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
	if config.GlobalChannelOrder == nil {
		return fmt.Errorf("global_channel_order must be provided")
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
	if config.GlobalChannelOrder == nil {
		return fmt.Errorf("global_channel_order must be provided")
	}
	if err := ValidateIntelligentRoutingConfig(config); err != nil {
		return err
	}
	intelligentRoutingMu.Lock()
	access := intelligentRoutingConfig.TokenGroupAccess
	if config.TokenGroupAccess == nil {
		config.TokenGroupAccess = access
	}
	intelligentRoutingConfig = cloneIntelligentRoutingConfig(config)
	intelligentRoutingMu.Unlock()
	return nil
}

func IntelligentRoutingGroups(config IntelligentRoutingConfig) map[string]string {
	groups := make(map[string]string, len(config.Groups)+1)
	if config.GlobalChannelOrder != nil {
		groups[IntelligentRoutingGlobalGroupID] = IntelligentRoutingGlobalGroupName
	}
	for _, group := range config.Groups {
		groups[group.ID] = group.Name
	}
	return groups
}

func ValidateIntelligentRoutingConfig(config IntelligentRoutingConfig) error {
	if err := validateChannelOrder("global_channel_order", config.GlobalChannelOrder); err != nil {
		return err
	}
	for tokenID, groups := range config.TokenGroupAccess {
		if tokenID <= 0 {
			return fmt.Errorf("token_group_access contains invalid token id %d", tokenID)
		}
		seen := make(map[string]struct{}, len(groups))
		for _, groupID := range groups {
			if _, exists := seen[groupID]; exists {
				return fmt.Errorf("token_group_access contains duplicate group %s", groupID)
			}
			seen[groupID] = struct{}{}
			if groupID == IntelligentRoutingGlobalGroupID {
				if config.GlobalChannelOrder == nil {
					return fmt.Errorf("token_group_access references an unavailable global group")
				}
				continue
			}
			found := false
			for _, group := range config.Groups {
				if group.ID == groupID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("token_group_access references unknown group %s", groupID)
			}
		}
	}
	groupIDs := make(map[string]struct{}, len(config.Groups))
	for i, group := range config.Groups {
		if !intelligentRoutingGroupIDPattern.MatchString(group.ID) || group.ID == "auto" || group.ID == IntelligentRoutingGlobalGroupID {
			return fmt.Errorf("groups[%d].id must contain 1-64 letters, numbers, underscores, or hyphens and cannot be auto", i)
		}
		if strings.TrimSpace(group.Name) == "" || len(group.Name) > 128 {
			return fmt.Errorf("groups[%d].name must contain 1-128 characters", i)
		}
		if _, exists := groupIDs[group.ID]; exists {
			return fmt.Errorf("duplicate routing group id: %s", group.ID)
		}
		groupIDs[group.ID] = struct{}{}
		if math.IsNaN(group.DiscountPercent) || math.IsInf(group.DiscountPercent, 0) || group.DiscountPercent < 0 || group.DiscountPercent > 100 {
			return fmt.Errorf("groups[%d].discount_percent must be between 0 and 100", i)
		}
		if math.IsNaN(group.TaxRatePercent) || math.IsInf(group.TaxRatePercent, 0) || group.TaxRatePercent < 0 || group.TaxRatePercent > 100 {
			return fmt.Errorf("groups[%d].tax_rate_percent must be between 0 and 100", i)
		}
		if err := validateChannelOrder(fmt.Sprintf("groups[%d].channel_order", i), group.ChannelOrder); err != nil {
			return err
		}
	}
	return nil
}

func cloneIntelligentRoutingConfig(config IntelligentRoutingConfig) IntelligentRoutingConfig {
	cloned := IntelligentRoutingConfig{
		GlobalChannelOrder: slices.Clone(config.GlobalChannelOrder),
		Groups:             make([]IntelligentRoutingGroup, len(config.Groups)),
		TokenGroupAccess:   make(map[int][]string, len(config.TokenGroupAccess)),
	}
	for tokenID, groups := range config.TokenGroupAccess {
		cloned.TokenGroupAccess[tokenID] = slices.Clone(groups)
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
