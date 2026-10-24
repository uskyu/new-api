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
	if info.UsePrice {
		modelPrice := decimal.NewFromFloat(info.ModelPrice)
		quotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		groupRatio := decimal.NewFromFloat(info.GroupRatio)

		quota := modelPrice.Mul(quotaPerUnit).Mul(groupRatio)
		return common.QuotaFromDecimalChecked(quota)
	}

	completionRatio := decimal.NewFromFloat(ratio_setting.GetCompletionRatio(info.ModelName))
	audioRatio := decimal.NewFromFloat(ratio_setting.GetAudioRatio(info.ModelName))
	audioCompletionRatio := decimal.NewFromFloat(ratio_setting.GetAudioCompletionRatio(info.ModelName))

	groupRatio := decimal.NewFromFloat(info.GroupRatio)
	modelRatio := decimal.NewFromFloat(info.ModelRatio)
	ratio := groupRatio.Mul(modelRatio)

	inputTextTokens := decimal.NewFromInt(int64(info.InputDetails.TextTokens))
	outputTextTokens := decimal.NewFromInt(int64(info.OutputDetails.TextTokens))
	inputAudioTokens := decimal.NewFromInt(int64(info.InputDetails.AudioTokens))
	outputAudioTokens := decimal.NewFromInt(int64(info.OutputDetails.AudioTokens))

	quota := decimal.Zero
	quota = quota.Add(inputTextTokens)
	quota = quota.Add(outputTextTokens.Mul(completionRatio))
	quota = quota.Add(inputAudioTokens.Mul(audioRatio))
	quota = quota.Add(outputAudioTokens.Mul(audioRatio).Mul(audioCompletionRatio))

	quota = quota.Mul(ratio)

	// If ratio is not zero and quota is less than or equal to zero, set quota to 1
	if !ratio.IsZero() && quota.LessThanOrEqual(decimal.Zero) {
		quota = decimal.NewFromInt(1)
	}

	return common.QuotaFromDecimalChecked(quota)
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

	quota, clamp := calculateAudioQuota(quotaInfo)
	noteQuotaClamp(relayInfo, clamp)

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

	quota, clamp := calculateAudioQuota(quotaInfo)
	noteQuotaClamp(relayInfo, clamp)
	if tieredOk {
		quota = tieredQuota
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
		logContent = append(logContent, common.NewMessage("Possibly an upstream timeout"))
		logger.LogError(ctx, common.LogText("total tokens is 0, cannot consume quota, userId %d, channelId %d, tokenId %d, model %s， pre-consumed quota %d",
			relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, modelName, relayInfo.FinalPreConsumedQuota))
	} else {
		model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, quota)
	}

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

	quota, clamp := calculateAudioQuota(quotaInfo)
	noteQuotaClamp(relayInfo, clamp)
	if tieredOk {
		quota = tieredQuota
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
}

// harnessSettlementCurrency is the ledger currency. The procurement ledger is
// always denominated in USD and is never converted to the site display
// currency: the agreed contract fixes cost.currency to USD, so any conversion
// here would silently mislabel the amounts whenever the site switch from USD.
// Display-currency conversion belongs to the catalog/dashboard path only.
const harnessSettlementCurrency = "USD"

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

// harnessSettlementMoneyForQuota converts a final quota into ledger amounts.
// base is quota / QuotaPerUnit, i.e. the canonical USD procurement base. A
// resolved pricing group applies its ratio; the channel's supplier tax is
// recorded for the caller but never folded into the amount, because the agreed
// contract forwards the tax to the platform and leaves it out of the cost.
// When no group resolved the request has no procurement discount, so the total
// tracks the base instead of collapsing to zero — "no procurement group" must
// never read as "free". This function only describes money; it never changes a
// charge.
func harnessSettlementMoneyForQuota(quota int, channelId int, modelName string) harnessSettlementMoney {
	money := harnessSettlementMoney{BaseAmount: "0", TotalAmount: "0", Currency: harnessSettlementCurrency}
	if quota < 0 {
		common.SysLog(common.LogText("harness settlement: ignoring negative quota %d (channel=%d model=%s)", quota, channelId, modelName))
		return money
	}
	quotaPerUnit := common.QuotaPerUnit
	if math.IsNaN(quotaPerUnit) || math.IsInf(quotaPerUnit, 0) || quotaPerUnit <= 0 {
		common.SysLog(common.LogText("harness settlement: invalid quota per unit %v, ledger amounts stay zero", quotaPerUnit))
		return money
	}

	base := decimal.NewFromInt(int64(quota)).Div(decimal.NewFromFloat(quotaPerUnit))
	pricingGroup, groupRatio, taxRatePercent := harnessSettlementPricing(channelId, modelName)
	total := base
	switch {
	case pricingGroup == "":
		// No opt-in procurement group: no discount. Keep the ratio at zero so
		// the row shows that no procurement group applied.
		groupRatio, taxRatePercent = 0, 0
	case math.IsNaN(groupRatio) || math.IsInf(groupRatio, 0) || groupRatio < 0:
		common.SysLog(common.LogText("harness settlement: ignoring invalid group ratio %v (group=%s), no discount applied", groupRatio, pricingGroup))
		groupRatio, taxRatePercent = 0, 0
	default:
		total = base.Mul(decimal.NewFromFloat(groupRatio))
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
	money.PriceVersion = harnessProcurementPriceVersion()
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

// recordHarnessSettlement upserts one procurement ledger row. It never returns
// an error and never panics: a settlement write must never break, slow or fail a
// relay, so a failure only reaches the server log and the request continues.
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
	if settlement.Status != model.HarnessSettlementStatusPending && settlement.SettledAt == 0 {
		settlement.SettledAt = common.GetTimestamp()
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			common.SysLog(common.LogText("harness settlement panicked (request=%s task=%s): %v", settlement.RequestId, settlement.TaskId, recovered))
		}
	}()
	if err := model.UpsertHarnessSettlement(settlement); err != nil {
		common.SysLog(common.LogText("harness settlement write failed (request=%s task=%s status=%s): %s", settlement.RequestId, settlement.TaskId, settlement.Status, err.Error()))
	}
	appendHarnessSettlementEvents(settlement, money, correlation)
}

// appendHarnessSettlementEvents derives the append-only event rows from a
// settlement snapshot. A correction or refund is a new event that supersedes the
// previous one; nothing is ever overwritten, so the platform can follow the
// stream with a single id cursor.
//
// The cost event always carries the cumulative net amount (never a delta): a
// dropped event then cannot corrupt the total, because the latest event alone
// states what the request is worth.
func appendHarnessSettlementEvents(settlement *model.HarnessSettlement, money harnessSettlementMoney, correlation harnessSettlementCorrelation) {
	if settlement == nil || settlement.RequestId == "" {
		return
	}
	observedAtMs := time.Now().UnixMilli()
	supersedes, recordVersion := previousHarnessLedgerEvent(settlement.RequestId)

	usageEvent := &model.HarnessLedgerEvent{
		Stream:            model.HarnessLedgerStreamUsage,
		EventType:         harnessUsageEventTypeForStatus(settlement.Status),
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
		RecordVersion:     recordVersion,
		SupersedesEventId: supersedes,
		ObservedAtMs:      observedAtMs,
	}
	if err := model.AppendHarnessLedgerEvent(usageEvent); err != nil {
		common.SysLog(common.LogText("harness usage event append failed (request=%s): %s", settlement.RequestId, err.Error()))
	}

	costEvent := &model.HarnessLedgerEvent{
		Stream:            model.HarnessLedgerStreamCost,
		EventType:         harnessCostEventTypeForStatus(settlement.Status),
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
		BaseAmount:        money.BaseAmount,
		TotalAmount:       money.TotalAmount,
		GroupRatio:        money.GroupRatio,
		TaxRatePercent:    money.TaxRatePercent,
		BillingBasis:      correlation.UsageUnit,
		PricingVersion:    money.PriceVersion,
		RecordVersion:     recordVersion,
		SupersedesEventId: supersedes,
		ObservedAtMs:      observedAtMs,
	}
	if err := model.AppendHarnessLedgerEvent(costEvent); err != nil {
		common.SysLog(common.LogText("harness cost event append failed (request=%s): %s", settlement.RequestId, err.Error()))
	}
}

// previousHarnessLedgerEvent returns the id and version of the most recent cost
// event recorded for this gateway request, so a correction can point at what it
// supersedes and advance the record version. A request's first event yields a
// nil predecessor and version 1.
func previousHarnessLedgerEvent(gatewayRequestId string) (*string, int) {
	if gatewayRequestId == "" {
		return nil, 1
	}
	var last model.HarnessLedgerEvent
	err := model.DB.Where("gateway_request_id = ? AND stream = ?", gatewayRequestId, model.HarnessLedgerStreamCost).
		Order("id DESC").Limit(1).Find(&last).Error
	if err != nil || last.EventId == "" {
		return nil, 1
	}
	return &last.EventId, last.RecordVersion + 1
}

// harnessUsageEventTypeForStatus maps a settlement status to the usage event
// type the platform expects.
func harnessUsageEventTypeForStatus(status string) string {
	if status == model.HarnessSettlementStatusRefunded {
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
	// is what the channel's abilities and the response headers are keyed by.
	recordHarnessSettlement(settlement, harnessSettlementMoneyForQuota(quota, relayInfo.ChannelId, relayInfo.OriginModelName), harnessSettlementCorrelationFromContext(ctx))
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
	recordHarnessSettlement(settlement, harnessSettlementMoneyForQuota(quota, task.ChannelId, taskModelName(task)), harnessSettlementCorrelation{UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream})
}
