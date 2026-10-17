package model

import (
	"slices"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting"
)

func IsChannelEnabledForGroupModel(group string, modelName string, channelID int) bool {
	if group == "" || modelName == "" || channelID <= 0 {
		return false
	}
	if !common.MemoryCacheEnabled {
		return isChannelEnabledForGroupModelDB(group, modelName, channelID)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return false
	}

	if isChannelIDInList(group2model2channels[group][modelName], channelID) {
		return true
	}
	normalized := ratio_setting.RoutingMatchModelName(modelName)
	if normalized != "" && normalized != modelName {
		return isChannelIDInList(group2model2channels[group][normalized], channelID)
	}
	return false
}

func GetEnabledAbilityGroupsForChannelModel(channelID int, modelName string) ([]string, error) {
	modelNames := []string{modelName}
	if normalized := ratio_setting.RoutingMatchModelName(modelName); normalized != "" && normalized != modelName {
		modelNames = append(modelNames, normalized)
	}
	if common.MemoryCacheEnabled {
		channelSyncLock.RLock()
		if channelModelEnabledGroups != nil {
			models := channelModelEnabledGroups[channelID]
			unique := make(map[string]struct{})
			for _, name := range modelNames { for _, id := range models[name] { unique[id] = struct{}{} } }
			channelSyncLock.RUnlock()
			groups := make([]string, 0, len(unique))
			for id := range unique { groups = append(groups,id) }
			slices.Sort(groups)
			return groups, nil
		}
		channelSyncLock.RUnlock()
	}
	var abilities []Ability
	if err := DB.Where("channel_id = ? and enabled = ? and model IN ?", channelID, true, modelNames).Find(&abilities).Error; err != nil {
		return nil, err
	}
	groups := make([]string, 0, len(abilities))
	seen := make(map[string]struct{}, len(abilities))
	for _, ability := range abilities {
		if _, exists := seen[ability.Group]; exists {
			continue
		}
		seen[ability.Group] = struct{}{}
		groups = append(groups, ability.Group)
	}
	slices.Sort(groups)
	return groups, nil
}

// Only inspect the small, cached list of groups attached to this channel and model.
func GetPreferredPricingGroupForChannelModel(channelID int, modelName string) (string, bool, error) {
	groups, err := GetEnabledAbilityGroupsForChannelModel(channelID, modelName)
	if err != nil { return "", false, err }
	chosen, lowest, found := "", 0, false
	for _, group := range groups {
		if rank, ok := setting.GetIntelligentRoutingPricingGroupRank(group); ok && (!found || rank < lowest) {
			chosen, lowest, found = group, rank, true
		}
	}
	return chosen, found, nil
}

func IsChannelEnabledForAnyGroupModel(groups []string, modelName string, channelID int) bool {
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if IsChannelEnabledForGroupModel(g, modelName, channelID) {
			return true
		}
	}
	return false
}

func isChannelEnabledForGroupModelDB(group string, modelName string, channelID int) bool {
	var count int64
	err := DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, modelName, channelID, true).
		Count(&count).Error
	if err == nil && count > 0 {
		return true
	}
	normalized := ratio_setting.RoutingMatchModelName(modelName)
	if normalized == "" || normalized == modelName {
		return false
	}
	count = 0
	err = DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, normalized, channelID, true).
		Count(&count).Error
	return err == nil && count > 0
}

func isChannelIDInList(list []int, channelID int) bool {
	return slices.Contains(list, channelID)
}
