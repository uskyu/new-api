package service

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func GetFallbackGroupChain(group string) []string {
	return buildGroupFallbackChain(setting.GetGroupFallbacksCopy(), group)
}

func GetModelAwareFallbackGroupChain(ctx *gin.Context, requestedGroup, modelName string) []string {
	if requestedGroup == "" || requestedGroup == "auto" {
		return nil
	}

	chain := GetFallbackGroupChain(requestedGroup)
	if len(chain) == 0 {
		chain = []string{requestedGroup}
	}

	userGroup := common.GetContextKeyString(ctx, constant.ContextKeyUserGroup)
	allowedGroups := GetUserUsableGroups(userGroup)
	modelGroups := model.GetModelEnableGroups(modelName)
	if len(modelGroups) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
		if normalizedModel != "" && normalizedModel != modelName {
			modelGroups = model.GetModelEnableGroups(normalizedModel)
		}
	}
	chain = buildModelGroupFallbackCandidates(requestedGroup, chain, allowedGroups, modelGroups)
	common.SetContextKey(ctx, constant.ContextKeyFallbackGroupChain, chain)
	return chain
}

func EnsureRequestedGroup(c *gin.Context, requestedGroup string) {
	if c == nil || requestedGroup == "" {
		return
	}
	if common.GetContextKeyString(c, constant.ContextKeyRequestedGroup) == "" {
		common.SetContextKey(c, constant.ContextKeyRequestedGroup, requestedGroup)
	}
	if requestedGroup == "auto" {
		return
	}
	if len(common.GetContextKeyStringSlice(c, constant.ContextKeyFallbackGroupChain)) == 0 {
		common.SetContextKey(c, constant.ContextKeyFallbackGroupChain, []string{requestedGroup})
	}
}

func TrackSelectedGroup(c *gin.Context, selectedGroup string) {
	if c == nil || selectedGroup == "" {
		return
	}
	chain := common.GetContextKeyStringSlice(c, constant.ContextKeyFallbackGroupChain)
	if len(chain) == 0 {
		chain = []string{selectedGroup}
	} else if chain[len(chain)-1] != selectedGroup {
		chain = append(chain, selectedGroup)
	}
	common.SetContextKey(c, constant.ContextKeyFallbackGroupChain, chain)
}

func buildModelGroupFallbackCandidates(requestedGroup string, fallbackChain []string, allowedGroups map[string]string, modelGroups []string) []string {
	normalizedRequested := strings.TrimSpace(requestedGroup)
	if normalizedRequested == "" {
		return nil
	}

	allowedSet := types.NewSet[string]()
	for group := range allowedGroups {
		allowedSet.Add(strings.TrimSpace(group))
	}
	allowedSet.Add(normalizedRequested)

	modelSet := types.NewSet[string]()
	for _, group := range modelGroups {
		if normalized := strings.TrimSpace(group); normalized != "" {
			modelSet.Add(normalized)
		}
	}
	modelKnown := modelSet.Len() > 0

	seen := types.NewSet[string]()
	result := make([]string, 0, len(fallbackChain)+allowedSet.Len())
	seen.Add(normalizedRequested)
	result = append(result, normalizedRequested)

	for _, group := range fallbackChain {
		normalized := strings.TrimSpace(group)
		if normalized == "" || seen.Contains(normalized) {
			continue
		}
		if !allowedSet.Contains(normalized) {
			continue
		}
		if normalized != normalizedRequested {
			if !modelKnown || !modelSet.Contains(normalized) {
				continue
			}
		}
		seen.Add(normalized)
		result = append(result, normalized)
	}

	if setting.EnableModelGroupAutoFallback && modelKnown {
		candidateGroups := make([]string, 0, len(allowedGroups))
		for group := range allowedGroups {
			candidateGroups = append(candidateGroups, group)
		}
		sort.Strings(candidateGroups)
		for _, group := range candidateGroups {
			normalized := strings.TrimSpace(group)
			if normalized == "" || seen.Contains(normalized) {
				continue
			}
			if !modelSet.Contains(normalized) || !allowedSet.Contains(normalized) {
				continue
			}
			seen.Add(normalized)
			result = append(result, normalized)
		}
	}

	return result
}

func buildGroupFallbackChain(cfg map[string][]string, start string) []string {
	if start == "" {
		return nil
	}
	chain := []string{start}
	visited := map[string]bool{start: true}
	for i := 0; i < len(chain); i++ {
		group := chain[i]
		for _, fallback := range cfg[group] {
			if fallback == "" || visited[fallback] {
				continue
			}
			visited[fallback] = true
			chain = append(chain, fallback)
		}
	}
	return chain
}

func shouldFallbackStatus(code int) bool {
	switch {
	case code == 429:
		return true
	case code == 408:
		return true
	case code >= 500 && code <= 599:
		return true
	default:
		return false
	}
}

func ShouldFallbackForError(c *gin.Context, err *types.NewAPIError) bool {
	if c == nil || err == nil {
		return false
	}
	requestedGroup := common.GetContextKeyString(c, constant.ContextKeyRequestedGroup)
	if requestedGroup == "" || requestedGroup == "auto" {
		return false
	}
	if len(common.GetContextKeyStringSlice(c, constant.ContextKeyFallbackGroupChain)) <= 1 {
		return false
	}
	if types.IsSkipRetryError(err) {
		return false
	}

	switch err.GetErrorCode() {
	case types.ErrorCodeInvalidRequest,
		types.ErrorCodeSensitiveWordsDetected,
		types.ErrorCodeCountTokenFailed,
		types.ErrorCodeModelPriceError,
		types.ErrorCodeInvalidApiType,
		types.ErrorCodeJsonMarshalFailed,
		types.ErrorCodeReadRequestBodyFailed,
		types.ErrorCodeConvertRequestFailed,
		types.ErrorCodeAccessDenied,
		types.ErrorCodeBadRequestBody,
		types.ErrorCodeModelNotFound,
		types.ErrorCodePromptBlocked,
		types.ErrorCodeInsufficientUserQuota,
		types.ErrorCodePreConsumeTokenQuotaFailed:
		return false
	}

	if err.StatusCode >= 200 && err.StatusCode < 300 {
		return false
	}
	if err.StatusCode == 400 || err.StatusCode == 401 || err.StatusCode == 403 || err.StatusCode == 404 {
		return false
	}

	return true
}

func AdvanceFallbackGroupOnError(c *gin.Context, retryParam *RetryParam, err *types.NewAPIError) bool {
	if c == nil || retryParam == nil || !ShouldFallbackForError(c, err) {
		return false
	}

	chain := common.GetContextKeyStringSlice(c, constant.ContextKeyFallbackGroupChain)
	if len(chain) <= 1 {
		return false
	}

	currentIndex := common.GetContextKeyInt(c, constant.ContextKeyFallbackGroupIndex)
	nextIndex := currentIndex + 1
	if nextIndex >= len(chain) {
		return false
	}

	common.SetContextKey(c, constant.ContextKeyFallbackGroupIndex, nextIndex)
	retryParam.SetRetry(0)
	retryParam.ResetRetryNextTry()
	return true
}
