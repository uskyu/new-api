package service

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type TokenDetails struct {
	TextTokens  int
	AudioTokens int
}

type QuotaInfo struct {
	InputDetails  TokenDetails
	OutputDetails TokenDetails
	ModelName     string
	UsePrice      bool
	ModelPrice    float64
	ModelRatio    float64
	GroupRatio    float64
}

func hasCustomModelRatio(modelName string, currentRatio float64) bool {
	defaultRatio, exists := ratio_setting.GetDefaultModelRatioMap()[modelName]
	if !exists {
		return true
	}
	return currentRatio != defaultRatio
}

func calculateAudioQuota(info QuotaInfo) (int, *common.QuotaClamp) {
	quota, clamp, _ := calculateAudioQuotaWithBasis(info)
	return quota, clamp
}

// calculateAudioQuotaWithBasis additionally returns the amount before the
// customer-side group ratio, which the procurement ledger needs. The group ratio
// is applied as one factor to that amount, so the charged quota is unchanged
// while the purchase cost stops depending on who called.
func calculateAudioQuotaWithBasis(info QuotaInfo) (int, *common.QuotaClamp, decimal.Decimal) {
	if info.UsePrice {
		modelPrice := decimal.NewFromFloat(info.ModelPrice)
		quotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		groupRatio := decimal.NewFromFloat(info.GroupRatio)

		preGroupQuota := modelPrice.Mul(quotaPerUnit)
		quota, clamp := common.QuotaFromDecimalChecked(preGroupQuota.Mul(groupRatio))
		return quota, clamp, preGroupQuota
	}

	completionRatio := decimal.NewFromFloat(ratio_setting.GetCompletionRatio(info.ModelName))
	audioRatio := decimal.NewFromFloat(ratio_setting.GetAudioRatio(info.ModelName))
	audioCompletionRatio := decimal.NewFromFloat(ratio_setting.GetAudioCompletionRatio(info.ModelName))

	groupRatio := decimal.NewFromFloat(info.GroupRatio)
	modelRatio := decimal.NewFromFloat(info.ModelRatio)

	inputTextTokens := decimal.NewFromInt(int64(info.InputDetails.TextTokens))
	outputTextTokens := decimal.NewFromInt(int64(info.OutputDetails.TextTokens))
	inputAudioTokens := decimal.NewFromInt(int64(info.InputDetails.AudioTokens))
	outputAudioTokens := decimal.NewFromInt(int64(info.OutputDetails.AudioTokens))

	quota := decimal.Zero
	quota = quota.Add(inputTextTokens)
	quota = quota.Add(outputTextTokens.Mul(completionRatio))
	quota = quota.Add(inputAudioTokens.Mul(audioRatio))
	quota = quota.Add(outputAudioTokens.Mul(audioRatio).Mul(audioCompletionRatio))

	preGroupQuota := quota.Mul(modelRatio)
	quota = preGroupQuota.Mul(groupRatio)

	// If ratio is not zero and quota is less than or equal to zero, set quota to 1
	if !groupRatio.IsZero() && !modelRatio.IsZero() && quota.LessThanOrEqual(decimal.Zero) {
		quota = decimal.NewFromInt(1)
	}

	intQuota, clamp := common.QuotaFromDecimalChecked(quota)
	return intQuota, clamp, preGroupQuota
}

func PreWssConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.RealtimeUsage) error {
	if relayInfo.UsePrice {
		return nil
	}
	userQuota, err := model.GetUserQuota(relayInfo.UserId, false)
	if err != nil {
		return err
	}

	token, err := model.GetTokenByKey(strings.TrimPrefix(relayInfo.TokenKey, "sk-"), false)
	if err != nil {
		return err
	}

	modelName := relayInfo.OriginModelName
	textInputTokens := usage.InputTokenDetails.TextTokens
	textOutTokens := usage.OutputTokenDetails.TextTokens
	audioInputTokens := usage.InputTokenDetails.AudioTokens
	audioOutTokens := usage.OutputTokenDetails.AudioTokens
	groupRatio := ratio_setting.GetGroupRatio(relayInfo.UsingGroup)
	modelRatio, _, _ := ratio_setting.GetModelRatio(modelName)

	autoGroup, exists := common.GetContextKey(ctx, constant.ContextKeyAutoGroup)
	if exists {
		groupRatio = ratio_setting.GetGroupRatio(autoGroup.(string))
		logger.LogDebug(ctx, "final group ratio: %f", groupRatio)
		relayInfo.UsingGroup = autoGroup.(string)
	}

	actualGroupRatio := groupRatio
	userGroupRatio, ok := ratio_setting.GetGroupGroupRatio(relayInfo.UserGroup, relayInfo.UsingGroup)
	if ok {
		actualGroupRatio = userGroupRatio
	}

	quotaInfo := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  textInputTokens,
			AudioTokens: audioInputTokens,
		},
		OutputDetails: TokenDetails{
			TextTokens:  textOutTokens,
			AudioTokens: audioOutTokens,
		},
		ModelName:  modelName,
		UsePrice:   relayInfo.UsePrice,
		ModelRatio: modelRatio,
		GroupRatio: actualGroupRatio,
	}

	quota, clamp, preGroupQuota := calculateAudioQuotaWithBasis(quotaInfo)
	noteQuotaClamp(relayInfo, clamp)
	relayInfo.ProcurementBasis = types.NewProcurementBasis(preGroupQuota, actualGroupRatio)

	if userQuota < quota {
		return fmt.Errorf("user quota is not enough, user quota: %s, need quota: %s", logger.FormatQuota(userQuota), logger.FormatQuota(quota))
	}

	if !token.UnlimitedQuota && token.RemainQuota < quota {
		return fmt.Errorf("token quota is not enough, token remain quota: %s, need quota: %s", logger.FormatQuota(token.RemainQuota), logger.FormatQuota(quota))
	}

	err = PostConsumeQuota(relayInfo, quota, 0, false)
	if err != nil {
		return err
	}
	logger.LogInfo(ctx, common.LogText("realtime streaming consume quota success, quota: %d", quota))
	return nil
}

func PostWssConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelName string,
	usage *dto.RealtimeUsage, extraContent []*common.Message) {

	var tieredResult *billingexpr.TieredResult
	tieredOk, tieredQuota, tieredRes := TryTieredSettle(relayInfo, billingexpr.TokenParams{
		P:   float64(usage.InputTokens),
		C:   float64(usage.OutputTokens),
		Len: float64(usage.InputTokens),
	})
	if tieredOk {
		tieredResult = tieredRes
	}

	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	textInputTokens := usage.InputTokenDetails.TextTokens
	textOutTokens := usage.OutputTokenDetails.TextTokens

	audioInputTokens := usage.InputTokenDetails.AudioTokens
	audioOutTokens := usage.OutputTokenDetails.AudioTokens

	tokenName := ctx.GetString("token_name")
	completionRatio := decimal.NewFromFloat(ratio_setting.GetCompletionRatio(modelName))
	audioRatio := decimal.NewFromFloat(ratio_setting.GetAudioRatio(relayInfo.OriginModelName))
	audioCompletionRatio := decimal.NewFromFloat(ratio_setting.GetAudioCompletionRatio(modelName))

	modelRatio := relayInfo.PriceData.ModelRatio
	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	modelPrice := relayInfo.PriceData.ModelPrice
	usePrice := relayInfo.PriceData.UsePrice

	quotaInfo := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  textInputTokens,
			AudioTokens: audioInputTokens,
		},
		OutputDetails: TokenDetails{
			TextTokens:  textOutTokens,
			AudioTokens: audioOutTokens,
		},
		ModelName:  modelName,
		UsePrice:   usePrice,
		ModelRatio: modelRatio,
		GroupRatio: groupRatio,
	}

	quota, clamp, preGroupQuota := calculateAudioQuotaWithBasis(quotaInfo)
	noteQuotaClamp(relayInfo, clamp)
	if tieredOk {
		quota = tieredQuota
		// The expression supersedes the audio estimate; its pre-group amount is
		// the procurement base.
		if tieredResult != nil {
			preGroupQuota = decimal.NewFromFloat(tieredResult.ActualQuotaBeforeGroup)
		}
	}

	totalTokens := usage.TotalTokens
	var logContent []*common.Message
	if !usePrice {
		logContent = append(logContent, common.NewMessage("Model ratio {{model_ratio}}, completion ratio {{completion_ratio}}, audio ratio {{audio_ratio}}, audio completion ratio {{audio_completion_ratio}}, group ratio {{group_ratio}}", map[string]any{
			"model_ratio":            fmt.Sprintf("%.2f", modelRatio),
			"completion_ratio":       fmt.Sprintf("%.2f", completionRatio.InexactFloat64()),
			"audio_ratio":            fmt.Sprintf("%.2f", audioRatio.InexactFloat64()),
			"audio_completion_ratio": fmt.Sprintf("%.2f", audioCompletionRatio.InexactFloat64()),
			"group_ratio":            fmt.Sprintf("%.2f", groupRatio),
		}))
	} else {
		logContent = append(logContent, common.NewMessage("Model price {{model_price}}, group ratio {{group_ratio}}", map[string]any{
			"model_price": fmt.Sprintf("%.2f", modelPrice),
			"group_ratio": fmt.Sprintf("%.2f", groupRatio),
		}))
	}

	// record all the consume log even if quota is 0
	if totalTokens == 0 {
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		quota = 0
		preGroupQuota = decimal.Zero
		logContent = append(logContent, common.NewMessage("Possibly an upstream timeout"))
		logger.LogError(ctx, common.LogText("total tokens is 0, cannot consume quota, userId %d, channelId %d, tokenId %d, model %s， pre-consumed quota %d",
			relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, modelName, relayInfo.FinalPreConsumedQuota))
	} else {
		model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, quota)
	}
	// Recorded after the branches above: a request with no billable usage has no
	// purchase either, so the basis must agree with the final charged quota
	// instead of describing an amount that was then zeroed.
	relayInfo.ProcurementBasis = types.NewProcurementBasis(preGroupQuota, groupRatio)

	if err := SettleBilling(ctx, relayInfo, quota); err != nil {
		logger.LogError(ctx, common.LogText("error settling billing: %s", err.Error()))
	}

	logModel := modelName
	logContent = append(logContent, extraContent...)
	other := GenerateWssOtherInfo(ctx, relayInfo, usage, modelRatio, groupRatio,
		completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), modelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	if tieredResult != nil {
		InjectTieredBillingInfo(other, relayInfo, tieredResult)
	}
	attachQuotaSaturation(ctx, relayInfo, other)
	recordRelayHarnessSettlement(ctx, relayInfo, harnessSettlementUsage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		CacheReadTokens:  usage.InputTokenDetails.CachedTokens,
		CacheWriteTokens: usage.InputTokenDetails.CacheCreationTokensTotal(),
	}, quota)
	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		ModelName:        logModel,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeSeconds:   int(useTimeSeconds),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
	})
}

func CalcOpenRouterCacheCreateTokens(usage dto.Usage, priceData types.PriceData) int {
	if priceData.CacheCreationRatio == 1 {
		return 0
	}
	quotaPrice := priceData.ModelRatio / common.QuotaPerUnit
	promptCacheCreatePrice := quotaPrice * priceData.CacheCreationRatio
	promptCacheReadPrice := quotaPrice * priceData.CacheRatio
	completionPrice := quotaPrice * priceData.CompletionRatio

	cost, _ := usage.Cost.(float64)
	totalPromptTokens := float64(usage.PromptTokens)
	completionTokens := float64(usage.CompletionTokens)
	promptCacheReadTokens := float64(usage.PromptTokensDetails.CachedTokens)

	value := (cost -
		totalPromptTokens*quotaPrice +
		promptCacheReadTokens*(quotaPrice-promptCacheReadPrice) -
		completionTokens*completionPrice) /
		(promptCacheCreatePrice - quotaPrice)
	quota, clamp := common.QuotaRoundChecked(value)
	if clamp != nil {
		return -1
	}
	return quota
}

func PostAudioConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, extraContent []*common.Message) {
	if usage == nil {
		usage = &dto.Usage{PromptTokens: relayInfo.GetEstimatePromptTokens(), TotalTokens: relayInfo.GetEstimatePromptTokens()}
	}

	var tieredUsedVars map[string]bool
	if snap := relayInfo.TieredBillingSnapshot; snap != nil {
		tieredUsedVars = billingexpr.UsedVarsByHash(snap.ExprString, snap.ExprHash)
	}
	var tieredResult *billingexpr.TieredResult
	tieredOk, tieredQuota, tieredRes := TryTieredSettle(relayInfo, BuildTieredTokenParams(usage, false, tieredUsedVars))
	if tieredOk {
		tieredResult = tieredRes
	}
	fixedPriceBilling := tieredOk && isFixedPriceSettlement(relayInfo, tieredRes)

	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	textInputTokens := usage.PromptTokensDetails.TextTokens
	textOutTokens := usage.CompletionTokenDetails.TextTokens

	audioInputTokens := usage.PromptTokensDetails.AudioTokens
	audioOutTokens := usage.CompletionTokenDetails.AudioTokens

	tokenName := ctx.GetString("token_name")
	billingModelName := relayInfo.GetBillingModelName()
	completionRatio := decimal.NewFromFloat(ratio_setting.GetCompletionRatio(billingModelName))
	audioRatio := decimal.NewFromFloat(ratio_setting.GetAudioRatio(billingModelName))
	audioCompletionRatio := decimal.NewFromFloat(ratio_setting.GetAudioCompletionRatio(billingModelName))

	modelRatio := relayInfo.PriceData.ModelRatio
	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	modelPrice := relayInfo.PriceData.ModelPrice
	usePrice := relayInfo.PriceData.UsePrice

	quotaInfo := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  textInputTokens,
			AudioTokens: audioInputTokens,
		},
		OutputDetails: TokenDetails{
			TextTokens:  textOutTokens,
			AudioTokens: audioOutTokens,
		},
		ModelName:  billingModelName,
		UsePrice:   usePrice,
		ModelRatio: modelRatio,
		GroupRatio: groupRatio,
	}

	quota, clamp, preGroupQuota := calculateAudioQuotaWithBasis(quotaInfo)
	noteQuotaClamp(relayInfo, clamp)
	if tieredOk {
		quota = tieredQuota
		// The expression supersedes the audio estimate entirely; its pre-group
		// amount is the procurement base. A fixed request price still has a real
		// purchase behind it, and ActualQuotaBeforeGroup already excludes the
		// customer group ratio, so no special case is needed here.
		if tieredResult != nil {
			preGroupQuota = decimal.NewFromFloat(tieredResult.ActualQuotaBeforeGroup)
		}
	}
	relayInfo.ProcurementBasis = types.NewProcurementBasis(preGroupQuota, groupRatio)

	totalTokens := usage.TotalTokens
	var logContent []*common.Message
	if !usePrice {
		logContent = append(logContent, common.NewMessage("Model ratio {{model_ratio}}, completion ratio {{completion_ratio}}, audio ratio {{audio_ratio}}, audio completion ratio {{audio_completion_ratio}}, group ratio {{group_ratio}}", map[string]any{
			"model_ratio":            fmt.Sprintf("%.2f", modelRatio),
			"completion_ratio":       fmt.Sprintf("%.2f", completionRatio.InexactFloat64()),
			"audio_ratio":            fmt.Sprintf("%.2f", audioRatio.InexactFloat64()),
			"audio_completion_ratio": fmt.Sprintf("%.2f", audioCompletionRatio.InexactFloat64()),
			"group_ratio":            fmt.Sprintf("%.2f", groupRatio),
		}))
	} else {
		logContent = append(logContent, common.NewMessage("Model price {{model_price}}, group ratio {{group_ratio}}", map[string]any{
			"model_price": fmt.Sprintf("%.2f", modelPrice),
			"group_ratio": fmt.Sprintf("%.2f", groupRatio),
		}))
	}

	// record all the consume log even if quota is 0
	if totalTokens == 0 && !fixedPriceBilling {
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		quota = 0
		logContent = append(logContent, common.NewMessage("Possibly an upstream timeout"))
		logger.LogError(ctx, common.LogText("total tokens is 0, cannot consume quota, userId %d, channelId %d, tokenId %d, model %s， pre-consumed quota %d",
			relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, billingModelName, relayInfo.FinalPreConsumedQuota))
	} else {
		model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, quota)
	}

	if err := SettleBilling(ctx, relayInfo, quota); err != nil {
		logger.LogError(ctx, common.LogText("error settling billing: %s", err.Error()))
	}

	logModel := billingModelName
	logContent = append(logContent, extraContent...)
	other := GenerateAudioOtherInfo(ctx, relayInfo, usage, modelRatio, groupRatio,
		completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), modelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	if tieredResult != nil {
		InjectTieredBillingInfo(other, relayInfo, tieredResult)
	}
	attachQuotaSaturation(ctx, relayInfo, other)
	recordRelayHarnessSettlement(ctx, relayInfo, harnessSettlementUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		CacheReadTokens:  usage.PromptTokensDetails.CachedTokens,
		CacheWriteTokens: usage.PromptTokensDetails.CacheCreationTokensTotal(),
	}, quota)
	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		ModelName:        logModel,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeSeconds:   int(useTimeSeconds),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
	})
	relayInfo.PerformanceOutputTokens = int64(usage.CompletionTokens)
}

func PreConsumeTokenQuota(relayInfo *relaycommon.RelayInfo, quota int) error {
	if quota < 0 {
		return errors.New("quota cannot be negative")
	}
	if relayInfo.IsPlayground {
		return nil
	}
	// 原子预扣：检查与扣减在同一操作中完成，并发请求不可能同时通过检查后超扣。
	reserved, err := model.TryReserveTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, quota, relayInfo.TokenUnlimited)
	if err != nil {
		return err
	}
	if !reserved {
		remainQuota := 0
		if token, tokenErr := model.GetTokenByKey(relayInfo.TokenKey, false); tokenErr == nil && token != nil {
			remainQuota = token.RemainQuota
		}
		return fmt.Errorf("token quota is not enough, token remain quota: %s, need quota: %s", logger.FormatQuota(remainQuota), logger.FormatQuota(quota))
	}
	return nil
}

type postConsumeQuotaResult struct {
	FundingApplied bool
	TokenApplied   bool
}

func PostConsumeQuota(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int, sendEmail bool) error {
	_, err := postConsumeQuotaWithResult(relayInfo, quota, preConsumedQuota, sendEmail)
	return err
}

func postConsumeQuotaWithResult(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int, sendEmail bool) (result postConsumeQuotaResult, err error) {

	// 1) Consume from wallet quota OR subscription item
	if relayInfo != nil && relayInfo.BillingSource == BillingSourceSubscription {
		if relayInfo.SubscriptionId == 0 {
			return result, errors.New("subscription id is missing")
		}
		delta := int64(quota)
		if delta != 0 {
			if err := model.PostConsumeUserSubscriptionDelta(relayInfo.SubscriptionId, delta); err != nil {
				return result, err
			}
			relayInfo.SubscriptionPostDelta += delta
		}
	} else {
		// Wallet
		if quota > 0 {
			err = model.DecreaseUserQuota(relayInfo.UserId, quota, false)
		} else {
			err = model.IncreaseUserQuota(relayInfo.UserId, -quota, false)
		}
		if err != nil {
			return result, err
		}
	}
	result.FundingApplied = true

	if !relayInfo.IsPlayground {
		if quota > 0 {
			err = model.DecreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
		} else {
			err = model.IncreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, -quota)
		}
		if err != nil {
			return result, err
		}
		result.TokenApplied = true
	}

	if sendEmail {
		if (quota + preConsumedQuota) != 0 {
			checkAndSendQuotaNotify(relayInfo, quota, preConsumedQuota)
		}
	}

	return result, nil
}

func checkAndSendQuotaNotify(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int) {
	gopool.Go(func() {
		userSetting := relayInfo.UserSetting
		threshold := common.QuotaRemindThreshold
		if userSetting.QuotaWarningThreshold != 0 {
			threshold = int(userSetting.QuotaWarningThreshold)
		}

		//noMoreQuota := userCache.Quota-(quota+preConsumedQuota) <= 0
		quotaTooLow := false
		consumeQuota := quota + preConsumedQuota
		if relayInfo.UserQuota-consumeQuota < threshold {
			quotaTooLow = true
		}
		if quotaTooLow {
			lang := userSetting.Language
			prompt := i18n.Translate(lang, i18n.MsgNotifyQuotaLowTitle)
			content, values := quotaNotifyContent(lang, userSetting.NotifyType, prompt, logger.FormatQuota(relayInfo.UserQuota))
			err := NotifyUser(relayInfo.UserId, relayInfo.UserEmail, relayInfo.UserSetting, dto.NewNotify(dto.NotifyTypeQuotaExceed, prompt, content, values))
			if err != nil {
				common.SysError(common.LogText("failed to send quota notify to user %d: %s", relayInfo.UserId, err.Error()))
			}
		}
	})
}

func checkAndSendSubscriptionQuotaNotify(relayInfo *relaycommon.RelayInfo) {
	gopool.Go(func() {
		if relayInfo == nil {
			return
		}
		if relayInfo.SubscriptionId == 0 || relayInfo.SubscriptionAmountTotal <= 0 {
			return
		}

		userSetting := relayInfo.UserSetting
		threshold := common.QuotaRemindThreshold
		if userSetting.QuotaWarningThreshold != 0 {
			threshold = int(userSetting.QuotaWarningThreshold)
		}

		usedAfter := relayInfo.SubscriptionAmountUsedAfterPreConsume + relayInfo.SubscriptionPostDelta
		remaining := relayInfo.SubscriptionAmountTotal - usedAfter
		if remaining >= int64(threshold) {
			return
		}

		lang := userSetting.Language
		prompt := i18n.Translate(lang, i18n.MsgNotifySubscriptionQuotaLowTitle)
		content, values := quotaNotifyContent(lang, userSetting.NotifyType, prompt, logger.FormatQuota(int(remaining)))
		if err := NotifyUser(relayInfo.UserId, relayInfo.UserEmail, relayInfo.UserSetting, dto.NewNotify(dto.NotifyTypeQuotaExceed, prompt, content, values)); err != nil {
			common.SysError(common.LogText("failed to send subscription quota notify to user %d: %s", relayInfo.UserId, err.Error()))
		}
	})
}

// quotaNotifyContent renders a low-quota notice in the user's saved language.
// Bark and Gotify get plain text; email and webhook get HTML with the top-up
// link. values stay in the notify payload for webhook receivers.
func quotaNotifyContent(lang string, notifyType string, prompt string, remaining string) (string, []any) {
	switch notifyType {
	case dto.NotifyTypeBark:
		// Bark推送使用简短文本，不支持HTML
		return i18n.Translate(lang, i18n.MsgNotifyQuotaLowBark, map[string]any{"Prompt": prompt, "Quota": remaining}), []any{prompt, remaining}
	case dto.NotifyTypeGotify:
		return i18n.Translate(lang, i18n.MsgNotifyQuotaLowGotify, map[string]any{"Prompt": prompt, "Quota": remaining}), []any{prompt, remaining}
	default:
		topUpLink := PaymentReturnURL("/wallet")
		return i18n.Translate(lang, i18n.MsgNotifyQuotaLowEmail, map[string]any{"Prompt": prompt, "Quota": remaining, "Link": topUpLink}), []any{prompt, remaining, topUpLink, topUpLink}
	}
}

// ---------------------------------------------------------------------------
// Harness procurement settlement ledger
// ---------------------------------------------------------------------------

// harnessSettlementAmountPrecision keeps ledger money at a fixed 10 decimal
// places, matching the precision reconciliation is expected to tolerate.
const harnessSettlementAmountPrecision = 10

// harnessSettlementUsage is the token breakdown recorded on a ledger row.
type harnessSettlementUsage struct {
	PromptTokens     int
	CompletionTokens int
	CacheReadTokens  int
	CacheWriteTokens int
}

// harnessSettlementCorrelation carries the platform-side identifiers and the
// usage provenance for the append-only ledger events. It is kept separate from
// the snapshot row because the snapshot table is deliberately free of
// platform-only columns, while the event stream needs them to be queryable by
// Harness request id.
type harnessSettlementCorrelation struct {
	HarnessRequestId string
	HarnessRunId     string
	UsageUnit        string
	UsageSource      string
	IsEstimated      bool
}

// harnessUsageUnitToken is the default billing basis. The platform treats token
// as the implicit unit, so it is used whenever no more specific unit applies.
const harnessUsageUnitToken = "token"

// harnessUsageSourceUpstream marks usage that came from the upstream response
// rather than a local estimate.
const harnessUsageSourceUpstream = "upstream"

// harnessUsageSourceEstimated marks usage the gateway had to estimate.
const harnessUsageSourceEstimated = "estimated"

// harnessSettlementCorrelationFromContext reads the correlation ids the request
// middleware stored. A non-Harness caller has none, and the events are still
// written with empty ids so the cost stream stays complete.
func harnessSettlementCorrelationFromContext(ctx *gin.Context) harnessSettlementCorrelation {
	correlation := harnessSettlementCorrelation{
		UsageUnit:   harnessUsageUnitToken,
		UsageSource: harnessUsageSourceUpstream,
	}
	if ctx == nil {
		return correlation
	}
	correlation.HarnessRequestId = common.GetContextKeyString(ctx, constant.ContextKeyHarnessRequestId)
	correlation.HarnessRunId = common.GetContextKeyString(ctx, constant.ContextKeyHarnessRunId)
	return correlation
}

// harnessSettlementMoney is the resolved procurement cost of one request.
type harnessSettlementMoney struct {
	PricingGroup   string
	GroupRatio     float64
	TaxRatePercent float64
	BaseAmount     string
	TotalAmount    string
	Currency       string
	// PriceVersion identifies the pricing snapshot the amount was derived from,
	// so the platform can tell a price change from a usage change.
	PriceVersion string
	// CostCalculationVersion identifies the algorithm that produced the amounts,
	// separately from the price configuration PriceVersion names. It lets
	// reconciliation tell a legacy row from a corrected one.
	CostCalculationVersion string
}

// harnessSettlementCurrency is the ledger currency. The procurement ledger is
// always denominated in USD and is never converted to the site display
// currency: the agreed contract fixes cost.currency to USD, so any conversion
// here would silently mislabel the amounts whenever the site switch from USD.
// Display-currency conversion belongs to the catalog/dashboard path only.
const harnessSettlementCurrency = "USD"

// ResolveTaskProcurementPricing resolves the procurement pricing parameters an
// async task must freeze at submit time, so settlement hours later books the
// price that applied when the task was accepted. A zero ratio and zero tax mean
// "no procurement discount", exactly as the settlement path reads an unresolved
// group.
func ResolveTaskProcurementPricing(channelId int, modelName string) (pricingGroup, priceVersion string, groupRatio, taxRatePercent float64) {
	pricingGroup, groupRatio, taxRatePercent = harnessSettlementPricing(channelId, modelName)
	return pricingGroup, harnessProcurementPriceVersion(), groupRatio, taxRatePercent
}

// harnessSettlementPricing resolves the procurement pricing group of a
// channel+model, that group's ratio and the channel's supplier tax. It mirrors
// setIntelligentRoutingResponseHeaders in controller/relay.go, which is the
// source of truth for the per-request X-Route-* headers. It never errors: an
// unresolved group yields zeros and the caller treats the request as having no
// procurement discount.
func harnessSettlementPricing(channelId int, modelName string) (pricingGroup string, groupRatio, taxRatePercent float64) {
	pricingGroup, valid, err := model.GetPreferredPricingGroupForChannelModel(channelId, modelName)
	if err != nil {
		common.SysLog(common.LogText("harness settlement: resolving pricing group failed (channel=%d model=%s): %s", channelId, modelName, err.Error()))
		return "", 0, 0
	}
	if !valid {
		return "", 0, 0
	}
	groupRatio = ratio_setting.GetGroupRatio(pricingGroup)
	// The selected channel can be a partial stub for the first attempt, so read
	// the cached channel row for the supplier tax, exactly like the controller.
	channel, err := model.CacheGetChannel(channelId)
	if err != nil {
		common.SysLog(common.LogText("harness settlement: reading supplier tax failed (channel=%d): %s", channelId, err.Error()))
		return pricingGroup, groupRatio, 0
	}
	taxRatePercent = channel.GetOtherSettings().SupplierTaxRatePercent
	if math.IsNaN(taxRatePercent) || math.IsInf(taxRatePercent, 0) || taxRatePercent < 0 {
		common.SysLog(common.LogText("harness settlement: ignoring invalid supplier tax %v (channel=%d)", taxRatePercent, channelId))
		taxRatePercent = 0
	}
	return pricingGroup, groupRatio, taxRatePercent
}

// harnessCostCalculationVersion records which algorithm produced base_amount.
// It is deliberately separate from PriceVersion: PriceVersion identifies the
// price configuration the amount was derived from, while this identifies the
// calculation itself, so reconciliation can tell an old row (whose base still
// carried the customer group ratio) from a repaired one.
const (
	// harnessCostCalculationLegacy marks a row written before the customer-side
	// group ratio was lifted out of the procurement base.
	harnessCostCalculationLegacy = ""
	// harnessCostCalculationPreGroup marks a row whose base_amount excludes the
	// customer-side group ratio.
	harnessCostCalculationPreGroup = "pregroup-v1"
	// harnessCostCalculationFallback marks a row that should have used the
	// pre-group amount but had none, so it fell back to quota/QuotaPerUnit.
	harnessCostCalculationFallback = "pregroup-v1-fallback"
)

// harnessSettlementMoneyForQuota converts a final quota into ledger amounts.
//
// base is the amount before the customer-side group ratio. The preferred source
// is basis, which the billing paths fill with the exact decimal they computed
// before applying that ratio. Falling back to quota/QuotaPerUnit is only for
// requests whose path did not fill a basis: it re-imports the caller's ratio and
// loses the pre-rounding amount, so such a row is tagged as a fallback instead
// of silently claiming the corrected semantics.
//
// A resolved pricing group applies its ratio and the channel's supplier tax, so
// total is the final procurement cost the platform books directly, matching the
// contract sample (0.011500 * 0.8 * 1.065 rounds to 0.009800).
//
// When no group resolved the request has no procurement discount, so the total
// tracks the base instead of collapsing to zero — "no procurement group" must
// never read as "free". This function only describes money; it never changes a
// charge.
func harnessSettlementMoneyForQuota(basis *types.ProcurementBasis, quota int, channelId int, modelName string) harnessSettlementMoney {
	money := harnessSettlementMoney{
		BaseAmount:             "0",
		TotalAmount:            "0",
		Currency:               harnessSettlementCurrency,
		CostCalculationVersion: harnessCostCalculationPreGroup,
	}
	if quota < 0 {
		common.SysLog(common.LogText("harness settlement: ignoring negative quota %d (channel=%d model=%s)", quota, channelId, modelName))
		return money
	}
	quotaPerUnit := common.QuotaPerUnit
	if math.IsNaN(quotaPerUnit) || math.IsInf(quotaPerUnit, 0) || quotaPerUnit <= 0 {
		common.SysLog(common.LogText("harness settlement: invalid quota per unit %v, ledger amounts stay zero", quotaPerUnit))
		return money
	}

	var base decimal.Decimal
	preGroup, hasPreGroup := basis.PreGroupQuotaDecimal()
	if hasPreGroup {
		base = preGroup.Div(decimal.NewFromFloat(quotaPerUnit))
	} else {
		base = decimal.NewFromInt(int64(quota)).Div(decimal.NewFromFloat(quotaPerUnit))
		money.CostCalculationVersion = harnessCostCalculationFallback
		// Warn whenever the fallback can actually be wrong. Only a unit ratio makes
		// the two amounts coincide, so that single case stays quiet to avoid noise
		// on every request of a default deployment; a zero ratio must be reported,
		// because there the charged quota carries no amount at all and the ledger
		// silently loses the whole cost.
		if basis == nil || basis.CustomerGroupRatio != 1 {
			ratio := 0.0
			if basis != nil {
				ratio = basis.CustomerGroupRatio
			}
			common.SysLog(common.LogText("harness settlement: no procurement basis (customer group ratio %v, channel=%d model=%s) - base_amount may include the customer multiplier or lose the cost entirely", ratio, channelId, modelName))
		}
	}

	pricingGroup, groupRatio, taxRatePercent := harnessSettlementPricing(channelId, modelName)
	return harnessSettlementMoneyFromBase(money, base, pricingGroup, groupRatio, taxRatePercent, harnessProcurementPriceVersion(), channelId, modelName)
}

// harnessSettlementMoneyWithFrozenPricing derives the ledger amounts of an async
// task from the procurement parameters frozen at submit time. Reading live
// configuration here would let an edit made while the task runs retroactively
// rewrite the cost of a request that was already accepted.
func harnessSettlementMoneyWithFrozenPricing(basis *types.ProcurementBasis, quota int, channelId int, modelName string, frozen *model.TaskProcurementContext) harnessSettlementMoney {
	money := harnessSettlementMoney{
		BaseAmount:             "0",
		TotalAmount:            "0",
		Currency:               harnessSettlementCurrency,
		CostCalculationVersion: harnessCostCalculationPreGroup,
	}
	if frozen == nil {
		return harnessSettlementMoneyForQuota(basis, quota, channelId, modelName)
	}
	quotaPerUnit := frozen.QuotaPerUnit
	if math.IsNaN(quotaPerUnit) || math.IsInf(quotaPerUnit, 0) || quotaPerUnit <= 0 {
		// A task frozen without a usable conversion rate cannot be priced from its
		// context; fall back rather than book zero, which would read as free.
		common.SysLog(common.LogText("harness settlement: frozen quota per unit %v is unusable (task channel=%d model=%s), falling back", quotaPerUnit, channelId, modelName))
		return harnessSettlementMoneyForQuota(basis, quota, channelId, modelName)
	}

	base := decimal.Zero
	if preGroup, ok := basis.PreGroupQuotaDecimal(); ok {
		base = preGroup.Div(decimal.NewFromFloat(quotaPerUnit))
	} else if quota <= 0 {
		// Nothing was charged and no pre-group amount was frozen, so the task
		// produced no purchase. Stay at zero instead of falling back, which would
		// divide a zero quota and report a fabricated basis version.
		return money
	} else {
		base = decimal.NewFromInt(int64(quota)).Div(decimal.NewFromFloat(quotaPerUnit))
		money.CostCalculationVersion = harnessCostCalculationFallback
		if ratio := frozen.CustomerGroupRatio; ratio != 0 && ratio != 1 {
			common.SysLog(common.LogText("harness settlement: task has no frozen pre-group amount with customer group ratio %v (channel=%d model=%s) - base_amount may include the customer multiplier", ratio, channelId, modelName))
		}
	}

	return harnessSettlementMoneyFromBase(money, base, frozen.PricingGroup, frozen.ProcurementGroupRatio, frozen.TaxRatePercent, frozen.PriceVersion, channelId, modelName)
}

// harnessSettlementMoneyFromBase applies the procurement discount and supplier
// tax to a base amount. Both callers above resolve base differently -- live
// pricing versus a frozen context -- but must compose the money identically.
func harnessSettlementMoneyFromBase(money harnessSettlementMoney, base decimal.Decimal, pricingGroup string, groupRatio, taxRatePercent float64, priceVersion string, channelId int, modelName string) harnessSettlementMoney {
	total := base
	switch {
	case pricingGroup == "":
		// No opt-in procurement group: no discount and no tax. Keep the ratio at
		// zero so the row shows that no procurement group applied.
		groupRatio, taxRatePercent = 0, 0
	case math.IsNaN(groupRatio) || math.IsInf(groupRatio, 0) || groupRatio < 0:
		common.SysLog(common.LogText("harness settlement: ignoring invalid group ratio %v (group=%s), no discount applied", groupRatio, pricingGroup))
		groupRatio, taxRatePercent = 0, 0
	case math.IsNaN(taxRatePercent) || math.IsInf(taxRatePercent, 0) || taxRatePercent < 0:
		common.SysLog(common.LogText("harness settlement: ignoring invalid supplier tax %v (group=%s), tax not applied", taxRatePercent, pricingGroup))
		taxRatePercent = 0
	}
	if pricingGroup != "" && groupRatio >= 0 && !math.IsNaN(groupRatio) && !math.IsInf(groupRatio, 0) {
		total = base.Mul(decimal.NewFromFloat(groupRatio)).
			Mul(decimal.NewFromFloat(1 + taxRatePercent/100))
	}
	if total.IsNegative() {
		common.SysLog(common.LogText("harness settlement: clamping negative total to zero (channel=%d model=%s)", channelId, modelName))
		total = decimal.Zero
	}

	money.PricingGroup = pricingGroup
	money.GroupRatio = groupRatio
	money.TaxRatePercent = taxRatePercent
	money.BaseAmount = base.Round(harnessSettlementAmountPrecision).String()
	money.TotalAmount = total.Round(harnessSettlementAmountPrecision).String()
	money.PriceVersion = priceVersion
	return money
}

// harnessProcurementPriceVersion identifies the pricing snapshot the ledger
// amounts were computed from, so reconciliation can tell a price change apart
// from a usage change. It is derived from the in-memory pricing cache the amount
// was actually computed against, and never issues an extra query on the billing
// path.
func harnessProcurementPriceVersion() string {
	pricing := model.GetPricing()
	if len(pricing) == 0 {
		return ""
	}
	encoded, err := common.Marshal(pricing)
	if err != nil {
		return ""
	}
	return "pw-" + common.ETagFor("harness-procurement-price:v1", string(encoded))
}

// recordHarnessSettlement upserts one procurement ledger row and appends the
// matching append-only ledger events. It never returns an error and never
// panics: a settlement write must never break, slow or fail a relay, so a
// failure only reaches the server log and the request continues.
//
// The snapshot row keeps answering "what is this request worth right now" while
// the events answer "what changed, in order" for the platform's incremental
// feed. Both are written from this one choke point, which is why every billing
// path (synchronous relay, streaming, async task, refund, violation fee) is
// covered without touching each call site.
func recordHarnessSettlement(settlement *model.HarnessSettlement, money harnessSettlementMoney, correlation harnessSettlementCorrelation) {
	if settlement == nil {
		return
	}
	if settlement.RequestId == "" {
		common.SysLog(common.LogText("harness settlement skipped: empty request id (task=%s model=%s)", settlement.TaskId, settlement.ModelName))
		return
	}
	settlement.PricingGroup = money.PricingGroup
	settlement.GroupRatio = money.GroupRatio
	settlement.TaxRatePercent = money.TaxRatePercent
	settlement.BaseAmount = money.BaseAmount
	settlement.TotalAmount = money.TotalAmount
	settlement.Currency = money.Currency
	settlement.CostCalculationVersion = money.CostCalculationVersion
	// Persist the platform correlation on the snapshot. An async task settles long
	// after the request context is gone, so this is the only place the owning
	// request's identity survives to that point.
	if correlation.HarnessRequestId != "" {
		settlement.HarnessRequestId = correlation.HarnessRequestId
		settlement.HarnessRunId = correlation.HarnessRunId
	}
	if settlement.Status != model.HarnessSettlementStatusPending && settlement.SettledAt == 0 {
		settlement.SettledAt = common.GetTimestamp()
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			common.SysLog(common.LogText("harness settlement panicked (request=%s task=%s): %v", settlement.RequestId, settlement.TaskId, recovered))
		}
	}()
	// The snapshot is written for every relayed request because the operator needs
	// the procurement cost regardless of who called us. Only a platform-originated
	// request additionally emits ledger events, and the events are attempted first
	// so a snapshot never advances past an unseen event.
	if settlement.HarnessRequestId == "" {
		if err := model.UpsertHarnessSettlement(settlement); err != nil {
			common.SysLog(common.LogText("harness settlement write failed (request=%s task=%s status=%s): %s", settlement.RequestId, settlement.TaskId, settlement.Status, err.Error()))
		}
		return
	}
	appendHarnessSettlementEvents(settlement, money, harnessSettlementCorrelation{
		HarnessRequestId: settlement.HarnessRequestId,
		HarnessRunId:     settlement.HarnessRunId,
		UsageUnit:        correlation.UsageUnit,
		UsageSource:      correlation.UsageSource,
		IsEstimated:      correlation.IsEstimated,
	})
}

// appendHarnessSettlementEvents derives the append-only event rows from a
// settlement snapshot and inserts them together with the snapshot itself inside
// one transaction.
//
// Only Harness-originated requests produce events. A settlement is still written
// for every relayed request because the operator needs the cost, but the ledger
// is the platform's own view of its traffic: admitting internal sk- requests
// would let the platform read other tenants' usage and procurement cost. The
// correlation id is the discriminator, and an empty one means "not from
// Harness".
//
// The usage and cost streams each keep their own version chain, so a usage
// correction never points at a cost event and vice versa.
//
// The cost event always carries the cumulative net amount (never a delta): a
// dropped event then cannot corrupt the total, because the latest event alone
// states what the request is worth.
func appendHarnessSettlementEvents(settlement *model.HarnessSettlement, money harnessSettlementMoney, correlation harnessSettlementCorrelation) {
	if settlement == nil || settlement.RequestId == "" {
		return
	}
	if correlation.HarnessRequestId == "" {
		// Not platform traffic: the snapshot row alone records the cost and no
		// ledger event is emitted, so the platform's feed stays scoped to its own
		// requests.
		return
	}

	observedAtMs := time.Now().UnixMilli()
	usageSupersedes, usageVersion := previousHarnessLedgerEvent(model.HarnessLedgerStreamUsage, settlement.RequestId)
	costSupersedes, costVersion := previousHarnessLedgerEvent(model.HarnessLedgerStreamCost, settlement.RequestId)

	usageEvent := &model.HarnessLedgerEvent{
		Stream:            model.HarnessLedgerStreamUsage,
		EventType:         harnessUsageEventTypeForStatus(settlement.Status, usageSupersedes != nil),
		HarnessRequestId:  correlation.HarnessRequestId,
		GatewayRequestId:  settlement.RequestId,
		RunId:             correlation.HarnessRunId,
		Status:            settlement.Status,
		Currency:          money.Currency,
		UsageUnit:         correlation.UsageUnit,
		PromptTokens:      settlement.PromptTokens,
		CompletionTokens:  settlement.CompletionTokens,
		CacheReadTokens:   settlement.CacheReadTokens,
		CacheWriteTokens:  settlement.CacheWriteTokens,
		UsageSource:       correlation.UsageSource,
		IsEstimated:       correlation.IsEstimated,
		BillingBasis:      correlation.UsageUnit,
		PricingVersion:    money.PriceVersion,
		RecordVersion:     usageVersion,
		SupersedesEventId: usageSupersedes,
		ObservedAtMs:      observedAtMs,
	}

	costEvent := &model.HarnessLedgerEvent{
		Stream:                 model.HarnessLedgerStreamCost,
		EventType:              harnessCostEventTypeForStatus(settlement.Status),
		HarnessRequestId:       correlation.HarnessRequestId,
		GatewayRequestId:       settlement.RequestId,
		RunId:                  correlation.HarnessRunId,
		Status:                 settlement.Status,
		Currency:               money.Currency,
		UsageUnit:              correlation.UsageUnit,
		PromptTokens:           settlement.PromptTokens,
		CompletionTokens:       settlement.CompletionTokens,
		CacheReadTokens:        settlement.CacheReadTokens,
		CacheWriteTokens:       settlement.CacheWriteTokens,
		UsageSource:            correlation.UsageSource,
		IsEstimated:            correlation.IsEstimated,
		BaseAmount:             money.BaseAmount,
		TotalAmount:            money.TotalAmount,
		GroupRatio:             money.GroupRatio,
		TaxRatePercent:         money.TaxRatePercent,
		BillingBasis:           correlation.UsageUnit,
		PricingVersion:         money.PriceVersion,
		CostCalculationVersion: money.CostCalculationVersion,
		RecordVersion:          costVersion,
		SupersedesEventId:      costSupersedes,
		ObservedAtMs:           observedAtMs,
	}
	persistHarnessLedgerEvents(settlement, []*model.HarnessLedgerEvent{usageEvent, costEvent})
}

// harnessLedgerWriteAttempts bounds the retry of the ledger transaction. One
// retry covers the transient failure worth retrying (a lost connection, a
// deadlock) without hammering a database that is genuinely unavailable.
const harnessLedgerWriteAttempts = 2

// persistHarnessLedgerEvents commits the ledger events and the settlement
// snapshot in one transaction, retrying the whole transaction once.
//
// The single transaction is what makes the retry meaningful. Committing the
// events first and the snapshot afterwards could leave the events durable and the
// snapshot missing; re-inserting those events then collides with the event_id
// unique index, so the "retry" fails before it ever reaches the snapshot. Here a
// failed attempt rolls back both, so the second attempt starts clean.
//
// A non-retryable error (a validation failure) is not retried: repeating it cannot
// succeed and only doubles the write latency on the billing path. A failure never
// propagates, because a ledger write must not fail a relay that already charged
// the user; it is logged so an operator can reconcile by hand.
//
// The failure logs separate two states an operator must treat differently:
//   - state=rolled back: the attempt is known to have written nothing, so the rows
//     are definitely missing and the write can simply be replayed.
//   - state=uncertain: the commit outcome is unknown, so the rows may already be
//     present. Verify by request id before repairing, because a blind replay of an
//     already-committed request could double-book it.
func persistHarnessLedgerEvents(settlement *model.HarnessSettlement, events []*model.HarnessLedgerEvent) {
	var lastErr error
	for attempt := range harnessLedgerWriteAttempts {
		lastErr = model.AppendHarnessLedgerEventsWithSettlement(events, settlement)
		if lastErr == nil {
			return
		}
		if !isRetryableHarnessLedgerError(lastErr) {
			common.SysError(common.LogText("harness ledger write failed with a non-retryable error (request=%s task=%s status=%s, state=rolled back): %v - verify by request id before issuing a correction", settlement.RequestId, settlement.TaskId, settlement.Status, lastErr))
			return
		}
		if attempt+1 < harnessLedgerWriteAttempts {
			common.SysLog(common.LogText("harness ledger write retrying (request=%s task=%s attempt=%d): %v", settlement.RequestId, settlement.TaskId, attempt+1, lastErr))
		}
	}
	// Every attempt reported an error, but a commit may have succeeded with only its
	// acknowledgement lost. Report the state as unknown rather than absent: an
	// operator who assumes absence could book the request twice.
	common.SysError(common.LogText("harness ledger write failed after retries (request=%s task=%s status=%s, state=uncertain): %v - the transaction may have committed; verify by request id before any repair", settlement.RequestId, settlement.TaskId, settlement.Status, lastErr))
}

// isRetryableHarnessLedgerError reports whether repeating the transaction could
// succeed. A validation failure is deterministic, so retrying it only wastes time
// on the billing path; everything else is treated as transient, which is the
// safer default because the alternative silently drops a ledger row.
//
// A duplicate event_id needs no special case: because the events and the snapshot
// now share one transaction, a duplicate can only appear when an earlier attempt
// committed and its acknowledgement was lost. That attempt left both rows
// durable, which is exactly the desired end state, so the retry failing on the
// duplicate is harmless and the failure is only logged.
func isRetryableHarnessLedgerError(err error) bool {
	if err == nil {
		return false
	}
	return !errors.Is(err, model.ErrHarnessLedgerInvalid) && !errors.Is(err, model.ErrHarnessSettlementInvalid)
}

// previousHarnessLedgerEvent returns the id and version of the most recent event
// in one stream for this gateway request, so a correction points at what it
// supersedes and advances that stream's record version. Version chains are kept
// per stream: a usage correction must never reference a cost event.
func previousHarnessLedgerEvent(stream, gatewayRequestId string) (*string, int) {
	if gatewayRequestId == "" || stream == "" {
		return nil, 1
	}
	var last model.HarnessLedgerEvent
	err := model.DB.Where("gateway_request_id = ? AND stream = ?", gatewayRequestId, stream).
		Order("id DESC").Limit(1).Find(&last).Error
	if err != nil || last.EventId == "" {
		return nil, 1
	}
	return &last.EventId, last.RecordVersion + 1
}

// harnessUsageEventTypeForStatus maps a settlement status to the usage event
// type the platform expects.
//
// A correction is only a correction when it supersedes something: the contract
// defines usage_correction as a new event that points at the earlier one through
// supersedes_event_id. A request can reach the refunded status as its very first
// ledger state (a task that is already terminal at submission, or a refund of a
// zero reservation), and labelling that first event a correction would produce an
// event that claims a predecessor it does not have. hasPrevious distinguishes the
// two cases; the amount semantics are identical, only the label differs.
func harnessUsageEventTypeForStatus(status string, hasPrevious bool) string {
	if status == model.HarnessSettlementStatusRefunded && hasPrevious {
		return model.HarnessLedgerEventTypeUsageCorrection
	}
	return model.HarnessLedgerEventTypeUsage
}

// harnessCostEventTypeForStatus maps a settlement status to the cost event type.
func harnessCostEventTypeForStatus(status string) string {
	switch status {
	case model.HarnessSettlementStatusRefunded:
		return model.HarnessLedgerEventTypeRefund
	case model.HarnessSettlementStatusPending:
		return model.HarnessLedgerEventTypeCost
	case model.HarnessSettlementStatusFailed:
		return model.HarnessLedgerEventTypeStatusChange
	default:
		return model.HarnessLedgerEventTypeCost
	}
}

// recordRelayHarnessSettlement records the settled cost of one synchronous
// relay. Call it only once the final quota is known. The deferred recovery makes
// the "never break a relay" guarantee cover the pricing lookups and the decimal
// math too, not just the database write.
func recordRelayHarnessSettlement(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage harnessSettlementUsage, quota int) {
	if relayInfo == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			common.SysLog(common.LogText("harness settlement panicked while pricing a relay: %v", recovered))
		}
	}()
	requestId := ""
	if ctx != nil {
		requestId = ctx.GetString(common.RequestIdKey)
	}
	if requestId == "" {
		requestId = relayInfo.RequestId
	}
	settlement := &model.HarnessSettlement{
		RequestId:        requestId,
		TokenId:          relayInfo.TokenId,
		UserId:           relayInfo.UserId,
		Status:           model.HarnessSettlementStatusSettled,
		ModelName:        relayInfo.GetBillingModelName(),
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		CacheReadTokens:  usage.CacheReadTokens,
		CacheWriteTokens: usage.CacheWriteTokens,
	}
	// The pricing group is resolved against the client-facing model name, which
	// is what the channel's abilities and the response headers are keyed by. The
	// procurement base comes from the basis the billing path filled, so this
	// caller's group ratio never reaches the ledger.
	recordHarnessSettlement(settlement, harnessSettlementMoneyForQuota(relayInfo.ProcurementBasis, quota, relayInfo.ChannelId, relayInfo.OriginModelName), harnessSettlementCorrelationFromContext(ctx))
}

// recordTaskHarnessSettlement moves the ledger row of an async task to its final
// billing state. The row is looked up by task id so a row written at submission
// keeps its request id and creation time; when no row exists one is created.
// It never returns an error: task billing continues regardless.
func recordTaskHarnessSettlement(task *model.Task, status string, quota int) {
	if task == nil || task.TaskID == "" {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			common.SysLog(common.LogText("harness settlement panicked while settling a task: %v", recovered))
		}
	}()
	settlement, err := model.GetHarnessSettlementByTaskId(task.TaskID)
	if err != nil {
		common.SysLog(common.LogText("harness settlement: loading task settlement failed (task=%s): %s", task.TaskID, err.Error()))
		return
	}
	if settlement == nil {
		settlement = &model.HarnessSettlement{
			RequestId: "task:" + task.TaskID,
			TaskId:    task.TaskID,
			TokenId:   task.PrivateData.TokenId,
			UserId:    task.UserId,
			ModelName: taskModelName(task),
			ChannelId: task.ChannelId,
			CreatedAt: task.SubmitTime,
		}
		if task.PrivateData.Execution != nil && task.PrivateData.Execution.RequestID != "" {
			settlement.RequestId = task.PrivateData.Execution.RequestID
		}
	}
	settlement.Status = status
	// The correlation comes from two places, and only one of them survives a
	// process restart.
	//
	// The durable source is the settlement row itself: the submit path wrote
	// harness_request_id on it, polling loads that row by task id, and the
	// upsert keeps a non-empty correlation on re-settle. The row read above
	// therefore already carries the request id.
	//
	// The task snapshot is only a fallback for the same process: the submit path
	// assigns Execution.HarnessRequestID in memory after the task row was
	// inserted, and no production path re-persists private_data for a normal
	// (non-background) task, so tasks.private_data does not carry the ids. That
	// assignment is not what makes attribution work, and a settlement row that
	// went missing leaves the correlation empty here, which suppresses the events
	// for that task.
	correlation := harnessSettlementCorrelation{UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream}
	if settlement.HarnessRequestId != "" {
		correlation.HarnessRequestId = settlement.HarnessRequestId
		correlation.HarnessRunId = settlement.HarnessRunId
	} else if task.PrivateData.Execution != nil {
		correlation.HarnessRequestId = task.PrivateData.Execution.HarnessRequestID
		correlation.HarnessRunId = task.PrivateData.Execution.HarnessRunID
	}
	recordHarnessSettlement(settlement, harnessSettlementMoneyForTask(task, status, quota), correlation)
}

// harnessSettlementMoneyForTask derives the ledger amounts of an async task.
//
// A task submitted with a frozen procurement context settles against that
// context only, so editing a procurement group ratio or a channel tax rate while
// the task runs cannot rewrite its cost. A task without one (submitted before
// the context existed, Version == 0) keeps the previous behaviour of reading the
// live configuration, which is the only safe reading of its zero values.
func harnessSettlementMoneyForTask(task *model.Task, status string, quota int) harnessSettlementMoney {
	var frozen *model.TaskProcurementContext
	if task.PrivateData.BillingContext != nil {
		frozen = task.PrivateData.BillingContext.Procurement
	}
	// A refund means nothing is owed, so there is no purchase to price either.
	if status == model.HarnessSettlementStatusRefunded {
		return harnessSettlementMoneyForQuota(nil, 0, task.ChannelId, taskModelName(task))
	}
	if frozen != nil && frozen.Version > 0 {
		var basis *types.ProcurementBasis
		if preGroup, ok := frozenPrefGroupQuota(frozen); ok {
			basis = types.NewProcurementBasis(preGroup, frozen.CustomerGroupRatio)
		}
		return harnessSettlementMoneyWithFrozenPricing(basis, quota, task.ChannelId, taskModelName(task), frozen)
	}
	return harnessSettlementMoneyForQuota(nil, quota, task.ChannelId, taskModelName(task))
}

// frozenPrefGroupQuota parses the pre-group amount frozen at submit time. ok is
// false for a usage-priced task, which learns its quantity only at completion.
func frozenPrefGroupQuota(frozen *model.TaskProcurementContext) (decimal.Decimal, bool) {
	if frozen == nil || frozen.PreGroupQuota == "" {
		return decimal.Zero, false
	}
	value, err := decimal.NewFromString(frozen.PreGroupQuota)
	if err != nil {
		return decimal.Zero, false
	}
	return value, true
}
