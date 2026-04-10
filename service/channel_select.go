package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type RetryParam struct {
	Ctx          *gin.Context
	TokenGroup   string
	ModelName    string
	Retry        *int
	resetNextTry bool
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	if param.TokenGroup == "auto" {
		if len(setting.GetAutoGroups()) == 0 {
			return nil, param.TokenGroup, errors.New("auto groups is not enabled")
		}
		userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)
		crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)
		return selectChannelFromGroupChain(
			param,
			GetUserAutoGroup(userGroup),
			constant.ContextKeyAutoGroupIndex,
			constant.ContextKeyAutoGroup,
			crossGroupRetry,
		)
	}

	groups := common.GetContextKeyStringSlice(param.Ctx, constant.ContextKeyFallbackGroupChain)
	if len(groups) == 0 || groups[0] != param.TokenGroup {
		groups = GetModelAwareFallbackGroupChain(param.Ctx, param.TokenGroup, param.ModelName)
	}
	if len(groups) == 0 {
		groups = []string{param.TokenGroup}
	}
	return selectChannelFromGroupChain(
		param,
		groups,
		constant.ContextKeyFallbackGroupIndex,
		constant.ContextKeyUsingGroup,
		len(groups) > 1,
	)
}

func selectChannelFromGroupChain(
	param *RetryParam,
	groups []string,
	indexKey constant.ContextKey,
	activeGroupKey constant.ContextKey,
	allowRetryAcrossGroups bool,
) (*model.Channel, string, error) {
	if len(groups) == 0 {
		return nil, param.TokenGroup, nil
	}

	startGroupIndex := 0
	if lastGroupIndex, exists := common.GetContextKey(param.Ctx, indexKey); exists {
		if idx, ok := lastGroupIndex.(int); ok && idx >= 0 && idx < len(groups) {
			startGroupIndex = idx
		}
	}

	for i := startGroupIndex; i < len(groups); i++ {
		group := groups[i]
		priorityRetry := param.GetRetry()
		if i > startGroupIndex {
			priorityRetry = 0
		}
		logger.LogDebug(param.Ctx, "Selecting group: %s, priorityRetry: %d", group, priorityRetry)

		channel, err := model.GetRandomSatisfiedChannel(group, param.ModelName, priorityRetry)
		if err != nil {
			return nil, group, err
		}
		if channel != nil {
			common.SetContextKey(param.Ctx, indexKey, i)
			common.SetContextKey(param.Ctx, activeGroupKey, group)
			return channel, group, nil
		}

		if !allowRetryAcrossGroups && param.GetRetry() > 0 {
			return nil, group, nil
		}

		logger.LogDebug(param.Ctx, "No available channel in group %s for model %s at priorityRetry %d, trying next group", group, param.ModelName, priorityRetry)
		common.SetContextKey(param.Ctx, indexKey, i+1)
		param.SetRetry(0)
	}

	return nil, groups[len(groups)-1], nil
}
