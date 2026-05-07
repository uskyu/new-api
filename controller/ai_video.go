package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const aiVideoEnabledModelsOptionKey = "AIVideoEnabledModels"

var defaultAIVideoSeconds = []string{"10", "15"}
var validAIVideoSeconds = map[string]bool{
	"5":  true,
	"10": true,
	"15": true,
}

type aiVideoModelConfig struct {
	Model   string   `json:"model"`
	Seconds []string `json:"seconds"`
}

type updateAIVideoModelsRequest struct {
	Models  []string             `json:"models"`
	Configs []aiVideoModelConfig `json:"configs"`
}

func normalizeAIVideoModels(models []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(models))
	for _, modelName := range models {
		trimmed := strings.TrimSpace(modelName)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}

func normalizeAIVideoSeconds(seconds []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(seconds))
	for _, second := range seconds {
		trimmed := strings.TrimSpace(second)
		if !validAIVideoSeconds[trimmed] || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	order := map[string]int{"5": 5, "10": 10, "15": 15}
	sort.Slice(result, func(i, j int) bool {
		return order[result[i]] < order[result[j]]
	})
	return result
}

func normalizeAIVideoModelConfigs(configs []aiVideoModelConfig) []aiVideoModelConfig {
	seen := make(map[string]bool)
	result := make([]aiVideoModelConfig, 0, len(configs))
	for _, config := range configs {
		modelName := strings.TrimSpace(config.Model)
		if modelName == "" || seen[modelName] {
			continue
		}
		seconds := normalizeAIVideoSeconds(config.Seconds)
		if len(seconds) == 0 {
			seconds = append([]string(nil), defaultAIVideoSeconds...)
		}
		seen[modelName] = true
		result = append(result, aiVideoModelConfig{
			Model:   modelName,
			Seconds: seconds,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Model < result[j].Model
	})
	return result
}

func modelConfigsFromNames(models []string) []aiVideoModelConfig {
	normalized := normalizeAIVideoModels(models)
	configs := make([]aiVideoModelConfig, 0, len(normalized))
	for _, modelName := range normalized {
		configs = append(configs, aiVideoModelConfig{
			Model:   modelName,
			Seconds: append([]string(nil), defaultAIVideoSeconds...),
		})
	}
	return configs
}

func getAIVideoModelConfigs() []aiVideoModelConfig {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[aiVideoEnabledModelsOptionKey]
	common.OptionMapRWMutex.RUnlock()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []aiVideoModelConfig{}
	}
	var configs []aiVideoModelConfig
	if err := common.UnmarshalJsonStr(raw, &configs); err == nil {
		return normalizeAIVideoModelConfigs(configs)
	}
	var models []string
	if err := common.UnmarshalJsonStr(raw, &models); err == nil {
		return modelConfigsFromNames(models)
	}
	return modelConfigsFromNames(strings.Split(raw, ","))
}

func getAIVideoEnabledModels() []string {
	configs := getAIVideoModelConfigs()
	models := make([]string, 0, len(configs))
	for _, config := range configs {
		models = append(models, config.Model)
	}
	return models
}

func getAIVideoConfigByModel(modelName string) (aiVideoModelConfig, bool) {
	modelName = strings.TrimSpace(modelName)
	for _, config := range getAIVideoModelConfigs() {
		if config.Model == modelName {
			return config, true
		}
	}
	return aiVideoModelConfig{}, false
}

func isAIVideoModelEnabled(modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false
	}
	_, enabled := getAIVideoConfigByModel(modelName)
	return enabled
}

func isAIVideoGatewayChannelType(channelType int) bool {
	return channelType == constant.ChannelTypeOpenAI || channelType == constant.ChannelTypeSora
}

func stringListContainsExact(items []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func selectAIVideoGatewayChannel(modelName, group string) (*model.Channel, error) {
	var channels []*model.Channel
	err := model.DB.
		Where("status = ?", common.ChannelStatusEnabled).
		Order("priority desc").
		Find(&channels).Error
	if err != nil {
		return nil, err
	}

	for _, channel := range channels {
		if channel == nil || !isAIVideoGatewayChannelType(channel.Type) {
			continue
		}
		if !stringListContainsExact(channel.GetGroups(), group) {
			continue
		}
		if stringListContainsExact(channel.GetModels(), modelName) {
			return channel, nil
		}
	}

	return nil, fmt.Errorf("AI video model %s has no configured OpenAI/Sora gateway channel in group %s", modelName, group)
}

func GetAIVideoModels(c *gin.Context) {
	common.ApiSuccess(c, getAIVideoModelConfigs())
}

func GetAdminAIVideoModels(c *gin.Context) {
	available := normalizeAIVideoModels(model.GetEnabledModels())
	common.ApiSuccess(c, gin.H{
		"available": available,
		"enabled":   getAIVideoEnabledModels(),
		"configs":   getAIVideoModelConfigs(),
	})
}

func UpdateAdminAIVideoModels(c *gin.Context) {
	req := updateAIVideoModelsRequest{}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	configs := normalizeAIVideoModelConfigs(req.Configs)
	if len(configs) == 0 && len(req.Models) > 0 {
		configs = modelConfigsFromNames(req.Models)
	}
	data, err := common.Marshal(configs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption(aiVideoEnabledModelsOptionKey, string(data)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"enabled": getAIVideoEnabledModels(),
		"configs": configs,
	})
}

func extractAIVideoRequestModelAndSeconds(c *gin.Context) (string, string, error) {
	if c.Request.Method != http.MethodPost {
		return "", "", nil
	}
	contentType := c.ContentType()
	if strings.Contains(contentType, "multipart/form-data") || strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
		formData, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return "", "", err
		}
		modelName := ""
		if values := formData.Value["model"]; len(values) > 0 {
			modelName = strings.TrimSpace(values[0])
		}
		seconds := ""
		if values := formData.Value["seconds"]; len(values) > 0 {
			seconds = strings.TrimSpace(values[0])
		}
		return modelName, seconds, nil
	}
	var req struct {
		Model   string `json:"model"`
		Seconds string `json:"seconds"`
	}
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return "", "", err
	}
	return strings.TrimSpace(req.Model), strings.TrimSpace(req.Seconds), nil
}

func PrepareAIVideoRelayContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt("id")
		userCache, err := model.GetUserCache(userID)
		if err != nil {
			common.ApiError(c, err)
			c.Abort()
			return
		}
		modelName, seconds, err := extractAIVideoRequestModelAndSeconds(c)
		if err != nil {
			common.ApiError(c, err)
			c.Abort()
			return
		}
		if c.Request.Method == http.MethodPost {
			if modelName == "" {
				common.ApiErrorMsg(c, "model is required")
				c.Abort()
				return
			}
			config, enabled := getAIVideoConfigByModel(modelName)
			if !enabled {
				common.ApiErrorMsg(c, fmt.Sprintf("AI video model %s is not enabled", modelName))
				c.Abort()
				return
			}
			if seconds != "" && !common.StringsContains(config.Seconds, seconds) {
				common.ApiErrorMsg(c, fmt.Sprintf("AI video model %s does not allow %s seconds", modelName, seconds))
				c.Abort()
				return
			}
		}
		usingGroup := strings.TrimSpace(c.Query("group"))
		if usingGroup == "" {
			usingGroup = userCache.Group
		}
		if usingGroup != userCache.Group && !service.GroupInUserUsableGroups(userCache.Group, usingGroup) {
			common.ApiErrorMsg(c, "group access denied")
			c.Abort()
			return
		}
		common.SetContextKey(c, constant.ContextKeyUserId, userID)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, usingGroup)
		common.SetContextKey(c, constant.ContextKeyUserGroup, userCache.Group)
		userCache.WriteContext(c)
		tempToken := &model.Token{
			UserId: userID,
			Name:   fmt.Sprintf("console-ai-video-%s", usingGroup),
			Group:  usingGroup,
		}
		if err := middleware.SetupContextForToken(c, tempToken); err != nil {
			common.ApiError(c, err)
			c.Abort()
			return
		}
		c.Set(relaycommon.ContextKeyConsoleRelayRequest, true)
		if c.Request.Method == http.MethodPost {
			channel, err := selectAIVideoGatewayChannel(modelName, usingGroup)
			if err != nil {
				common.ApiError(c, err)
				c.Abort()
				return
			}
			if setupErr := middleware.SetupContextForSelectedChannel(c, channel, modelName); setupErr != nil {
				common.ApiError(c, setupErr.Err)
				c.Abort()
				return
			}
			c.Set("relay_mode", relayconstant.RelayModeVideoSubmit)
		}
		c.Next()
	}
}

func FetchAIVideoTask(c *gin.Context) {
	c.Set("relay_mode", relayconstant.RelayModeVideoFetchByID)
	RelayTaskFetch(c)
}
