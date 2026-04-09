package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func GetFallbackGroupChain(group string) []string {
	return buildGroupFallbackChain(setting.GetGroupFallbacksCopy(), group)
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
	if len(GetFallbackGroupChain(requestedGroup)) <= 1 {
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

	requestedGroup := common.GetContextKeyString(c, constant.ContextKeyRequestedGroup)
	chain := GetFallbackGroupChain(requestedGroup)
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
