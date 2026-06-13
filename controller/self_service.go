package controller

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const selfServiceConfigOptionKey = "SelfServiceConfig"

type SelfServiceConfig struct {
	Enabled            bool   `json:"enabled"`
	LookbackHours      int    `json:"lookback_hours"`
	MaxRefundsPerClaim int    `json:"max_refunds_per_claim"`
	DailyRefundLimit   int    `json:"daily_refund_limit"`
	DisplayLimit       int    `json:"display_limit"`
	RefundPercent      int    `json:"refund_percent"`
	ExcludeModels      string `json:"exclude_models"`
	ScanLimit          int    `json:"scan_limit"`
}

type selfServiceRulePayload struct {
	Id             int    `json:"id"`
	ThresholdQuota int    `json:"threshold_quota"`
	TargetGroup    string `json:"target_group"`
	Description    string `json:"description"`
	Enabled        bool   `json:"enabled"`
}

type selfServiceRulesUpdateRequest struct {
	Rules []selfServiceRulePayload `json:"rules"`
}

type selfServiceIssueRecord struct {
	LogId         int    `json:"log_id"`
	CreatedAt     int64  `json:"created_at"`
	ModelName     string `json:"model_name"`
	Quota         int    `json:"quota"`
	RefundedQuota int    `json:"refunded_quota"`
	RequestId     string `json:"request_id"`
	Status        string `json:"status"`
}

type selfServiceUpgradeOffer struct {
	RuleId         int    `json:"rule_id"`
	TargetGroup    string `json:"target_group"`
	Description    string `json:"description"`
	ThresholdQuota int    `json:"threshold_quota"`
}

type SelfServiceSelfInfo struct {
	Enabled      bool                            `json:"enabled"`
	UserId       int                             `json:"user_id"`
	Username     string                          `json:"username"`
	CurrentGroup string                          `json:"current_group"`
	Quota        int                             `json:"quota"`
	UsedQuota    int                             `json:"used_quota"`
	TotalQuota   int                             `json:"total_quota"`
	TotalRefund  int64                           `json:"total_refunded_quota"`
	Config       SelfServiceConfig               `json:"config"`
	UpgradeOffer *selfServiceUpgradeOffer        `json:"upgrade_offer"`
	UpgradeRules []*model.SelfServiceUpgradeRule `json:"upgrade_rules"`
}

type selfServiceCheckResponse struct {
	SelfServiceSelfInfo
	ScannedCount      int                      `json:"scanned_count"`
	CandidateCount    int                      `json:"candidate_count"`
	NewCandidateCount int                      `json:"new_candidate_count"`
	RefundedCount     int                      `json:"refunded_count"`
	RefundedQuota     int                      `json:"refunded_quota"`
	RemainingDaily    int                      `json:"remaining_daily"`
	Records           []selfServiceIssueRecord `json:"records"`
	Message           string                   `json:"message"`
}

func defaultSelfServiceConfig() SelfServiceConfig {
	return SelfServiceConfig{
		Enabled:            true,
		LookbackHours:      24,
		MaxRefundsPerClaim: 10,
		DailyRefundLimit:   10,
		DisplayLimit:       10,
		RefundPercent:      100,
		ExcludeModels:      "gpt-image,dall-e",
		ScanLimit:          200,
	}
}

func normalizeSelfServiceConfig(config SelfServiceConfig) SelfServiceConfig {
	defaults := defaultSelfServiceConfig()
	if config.LookbackHours <= 0 {
		config.LookbackHours = defaults.LookbackHours
	}
	if config.MaxRefundsPerClaim <= 0 {
		config.MaxRefundsPerClaim = defaults.MaxRefundsPerClaim
	}
	if config.DailyRefundLimit <= 0 {
		config.DailyRefundLimit = defaults.DailyRefundLimit
	}
	if config.DisplayLimit <= 0 {
		config.DisplayLimit = defaults.DisplayLimit
	}
	if config.RefundPercent <= 0 {
		config.RefundPercent = defaults.RefundPercent
	}
	if config.RefundPercent > 100 {
		config.RefundPercent = 100
	}
	if strings.TrimSpace(config.ExcludeModels) == "" {
		config.ExcludeModels = defaults.ExcludeModels
	}
	if config.ScanLimit <= 0 {
		config.ScanLimit = defaults.ScanLimit
	}
	if config.ScanLimit > 1000 {
		config.ScanLimit = 1000
	}
	return config
}

func getSelfServiceConfig() SelfServiceConfig {
	config := defaultSelfServiceConfig()
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[selfServiceConfigOptionKey]
	common.OptionMapRWMutex.RUnlock()
	if raw != "" {
		_ = common.UnmarshalJsonStr(raw, &config)
	}
	return normalizeSelfServiceConfig(config)
}

func saveSelfServiceConfig(config SelfServiceConfig) error {
	config = normalizeSelfServiceConfig(config)
	bytes, err := common.Marshal(config)
	if err != nil {
		return err
	}
	return model.UpdateOption(selfServiceConfigOptionKey, string(bytes))
}

func selfServiceExcludedModels(config SelfServiceConfig) []string {
	parts := strings.Split(config.ExcludeModels, ",")
	models := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			models = append(models, part)
		}
	}
	return models
}

func selfServiceBestOffer(currentGroup string, totalQuota int, rules []*model.SelfServiceUpgradeRule) *selfServiceUpgradeOffer {
	eligible := make([]*model.SelfServiceUpgradeRule, 0)
	for _, rule := range rules {
		if !rule.Enabled || rule.ThresholdQuota > totalQuota {
			continue
		}
		if strings.TrimSpace(rule.TargetGroup) == "" || rule.TargetGroup == currentGroup {
			continue
		}
		eligible = append(eligible, rule)
	}
	if len(eligible) == 0 {
		return nil
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].ThresholdQuota == eligible[j].ThresholdQuota {
			return eligible[i].Id > eligible[j].Id
		}
		return eligible[i].ThresholdQuota > eligible[j].ThresholdQuota
	})
	best := eligible[0]
	return &selfServiceUpgradeOffer{
		RuleId:         best.Id,
		TargetGroup:    best.TargetGroup,
		Description:    best.Description,
		ThresholdQuota: best.ThresholdQuota,
	}
}

func buildSelfServiceSelfInfo(user *model.User, config SelfServiceConfig) (SelfServiceSelfInfo, error) {
	rules, err := model.ListSelfServiceUpgradeRules(false)
	if err != nil {
		return SelfServiceSelfInfo{}, err
	}
	totalRefund, err := model.SumUserSelfServiceRefundQuota(user.Id)
	if err != nil {
		return SelfServiceSelfInfo{}, err
	}
	totalQuota := user.Quota + user.UsedQuota
	return SelfServiceSelfInfo{
		Enabled:      config.Enabled,
		UserId:       user.Id,
		Username:     user.Username,
		CurrentGroup: user.Group,
		Quota:        user.Quota,
		UsedQuota:    user.UsedQuota,
		TotalQuota:   totalQuota,
		TotalRefund:  totalRefund,
		Config:       config,
		UpgradeOffer: selfServiceBestOffer(user.Group, totalQuota, rules),
		UpgradeRules: rules,
	}, nil
}

func getSelfServiceCurrentUser(c *gin.Context) (*model.User, error) {
	userId := c.GetInt("id")
	if userId == 0 {
		return nil, errors.New("user not found")
	}
	return model.GetUserById(userId, false)
}

func GetSelfServiceSelf(c *gin.Context) {
	user, err := getSelfServiceCurrentUser(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	config := getSelfServiceConfig()
	info, err := buildSelfServiceSelfInfo(user, config)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, info)
}

func CheckSelfService(c *gin.Context) {
	user, err := getSelfServiceCurrentUser(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	config := getSelfServiceConfig()
	if !config.Enabled {
		common.ApiErrorMsg(c, "Self-service platform is disabled")
		return
	}

	now := common.GetTimestamp()
	start := now - int64(config.LookbackHours)*3600
	queryLimit := int(math.Max(float64(config.ScanLimit), float64(config.DisplayLimit+config.MaxRefundsPerClaim)))
	logs, totalCandidates, err := model.FindSelfServiceEmptyOutputLogs(user.Id, start, now, queryLimit, selfServiceExcludedModels(config))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	todayRefunded, err := model.CountUserSelfServiceRefundsSince(user.Id, model.TodayStartTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	remainingDaily := config.DailyRefundLimit - int(todayRefunded)
	if remainingDaily < 0 {
		remainingDaily = 0
	}
	claimLimit := config.MaxRefundsPerClaim
	if claimLimit > remainingDaily {
		claimLimit = remainingDaily
	}

	freshLogs := make([]*model.Log, 0)
	records := make([]selfServiceIssueRecord, 0, config.DisplayLimit)
	for _, logItem := range logs {
		status := "checked"
		refundedQuota := 0
		if history, err := model.GetSelfServiceRefundHistory(logItem.Id); err == nil && history.Id > 0 {
			status = "fixed"
			refundedQuota = history.RefundedQuota
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			freshLogs = append(freshLogs, logItem)
		} else if err != nil {
			common.ApiError(c, err)
			return
		}
		if len(records) < config.DisplayLimit {
			records = append(records, selfServiceIssueRecord{
				LogId:         logItem.Id,
				CreatedAt:     logItem.CreatedAt,
				ModelName:     logItem.ModelName,
				Quota:         logItem.Quota,
				RefundedQuota: refundedQuota,
				RequestId:     logItem.RequestId,
				Status:        status,
			})
		}
	}

	selected := freshLogs
	if len(selected) > claimLimit {
		selected = selected[:claimLimit]
	}
	selectedLogIds := make(map[int]int, len(selected))
	refundedQuota := 0
	for index, item := range selected {
		quota := int(math.Floor(float64(item.Quota) * float64(config.RefundPercent) / 100))
		if quota > 0 {
			refundedQuota += quota
		}
		selectedLogIds[item.Id] = index
	}

	refundedCount := 0
	if refundedQuota > 0 || len(selected) > 0 {
		err = model.DB.Transaction(func(tx *gorm.DB) error {
			currentDailyCount, err := model.CountSelfServiceRefundsSinceTx(tx, user.Id, model.TodayStartTimestamp())
			if err != nil {
				return err
			}
			if int(currentDailyCount)+len(selected) > config.DailyRefundLimit {
				return errors.New("daily refund limit reached, please refresh and retry")
			}
			if refundedQuota > 0 {
				if err := tx.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", gorm.Expr("quota + ?", refundedQuota)).Error; err != nil {
					return err
				}
			}
			for _, item := range selected {
				itemRefundQuota := int(math.Floor(float64(item.Quota) * float64(config.RefundPercent) / 100))
				history := &model.SelfServiceRefundHistory{
					UserId:        user.Id,
					Username:      user.Username,
					LogId:         item.Id,
					ModelName:     item.ModelName,
					OriginalQuota: item.Quota,
					RefundedQuota: itemRefundQuota,
					RequestId:     item.RequestId,
					Status:        model.SelfServiceRefundStatusSuccess,
					Message:       "self-service",
				}
				if err := tx.Create(history).Error; err != nil {
					return err
				}
				refundedCount++
			}
			return nil
		})
		if err != nil {
			_ = model.DB.Create(&model.SelfServiceClaimAttempt{
				UserId:            user.Id,
				Username:          user.Username,
				ScannedCount:      len(logs),
				CandidateCount:    totalCandidates,
				NewCandidateCount: len(freshLogs),
				Status:            model.SelfServiceAttemptStatusFailed,
				Message:           err.Error(),
			}).Error
			common.ApiError(c, err)
			return
		}
		_ = model.InvalidateUserCache(user.Id)
		for i, record := range records {
			if selectedIndex, ok := selectedLogIds[record.LogId]; ok {
				records[i].Status = "fixed"
				records[i].RefundedQuota = int(math.Floor(float64(selected[selectedIndex].Quota) * float64(config.RefundPercent) / 100))
			}
		}
		model.RecordLog(user.Id, model.LogTypeManage, fmt.Sprintf("自助平台返还空回额度 %s", logger.LogQuota(refundedQuota)))
	}

	attemptMessage := ""
	if totalCandidates == 0 {
		attemptMessage = "no_candidates"
	} else if remainingDaily <= 0 && len(freshLogs) > 0 {
		attemptMessage = "daily_limit_reached"
	} else if len(freshLogs) == 0 {
		attemptMessage = "already_processed"
	}
	_ = model.DB.Create(&model.SelfServiceClaimAttempt{
		UserId:            user.Id,
		Username:          user.Username,
		ScannedCount:      len(logs),
		CandidateCount:    totalCandidates,
		NewCandidateCount: len(freshLogs),
		RefundedCount:     refundedCount,
		RefundedQuota:     refundedQuota,
		Status:            model.SelfServiceAttemptStatusSuccess,
		Message:           attemptMessage,
	}).Error

	user, err = model.GetUserById(user.Id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	info, err := buildSelfServiceSelfInfo(user, config)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	message := "No empty output records found"
	if refundedCount > 0 {
		message = "Empty output records were found and refunded"
	} else if remainingDaily <= 0 && len(freshLogs) > 0 {
		message = "Daily refund limit reached"
	} else if totalCandidates > 0 {
		message = "Empty output records were already checked"
	}
	common.ApiSuccess(c, selfServiceCheckResponse{
		SelfServiceSelfInfo: info,
		ScannedCount:        len(logs),
		CandidateCount:      totalCandidates,
		NewCandidateCount:   len(freshLogs),
		RefundedCount:       refundedCount,
		RefundedQuota:       refundedQuota,
		RemainingDaily:      remainingDaily - refundedCount,
		Records:             records,
		Message:             message,
	})
}

func UpgradeSelfServiceGroup(c *gin.Context) {
	user, err := getSelfServiceCurrentUser(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	config := getSelfServiceConfig()
	if !config.Enabled {
		common.ApiErrorMsg(c, "Self-service platform is disabled")
		return
	}
	rules, err := model.ListSelfServiceUpgradeRules(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	totalQuota := user.Quota + user.UsedQuota
	offer := selfServiceBestOffer(user.Group, totalQuota, rules)
	if offer == nil {
		common.ApiErrorMsg(c, "No eligible upgrade is available")
		return
	}
	fromGroup := user.Group
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var currentUser model.User
		if err := tx.Where("id = ?", user.Id).First(&currentUser).Error; err != nil {
			return err
		}
		if currentUser.Group != fromGroup {
			return errors.New("upgrade state changed, please refresh and retry")
		}
		currentTotalQuota := currentUser.Quota + currentUser.UsedQuota
		currentRules, err := model.ListSelfServiceUpgradeRulesTx(tx, false)
		if err != nil {
			return err
		}
		currentOffer := selfServiceBestOffer(currentUser.Group, currentTotalQuota, currentRules)
		if currentOffer == nil || currentOffer.RuleId != offer.RuleId || currentOffer.TargetGroup != offer.TargetGroup {
			return errors.New("upgrade rule changed, please refresh and retry")
		}
		result := tx.Model(&model.User{}).Where(&model.User{Id: user.Id, Group: fromGroup}).Update("group", currentOffer.TargetGroup)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("upgrade state changed, please refresh and retry")
		}
		if err := tx.Create(&model.SelfServiceUpgradeHistory{
			UserId:     user.Id,
			Username:   user.Username,
			FromGroup:  fromGroup,
			ToGroup:    currentOffer.TargetGroup,
			TotalQuota: currentTotalQuota,
			RuleId:     currentOffer.RuleId,
			Status:     model.SelfServiceUpgradeStatusSuccess,
			Message:    "self-upgrade",
		}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		_ = model.DB.Create(&model.SelfServiceUpgradeHistory{
			UserId:     user.Id,
			Username:   user.Username,
			FromGroup:  fromGroup,
			ToGroup:    offer.TargetGroup,
			TotalQuota: totalQuota,
			RuleId:     offer.RuleId,
			Status:     model.SelfServiceUpgradeStatusFailed,
			Message:    err.Error(),
		}).Error
		common.ApiError(c, err)
		return
	}
	_ = model.InvalidateUserCache(user.Id)
	_ = model.InvalidateUserTokensCache(user.Id)
	session := sessions.Default(c)
	session.Set("group", offer.TargetGroup)
	_ = session.Save()
	model.RecordLog(user.Id, model.LogTypeManage, fmt.Sprintf("自助平台升级分组：%s -> %s", fromGroup, offer.TargetGroup))
	common.ApiSuccess(c, gin.H{"current_group": offer.TargetGroup})
}

func GetSelfServiceAdminConfig(c *gin.Context) {
	config := getSelfServiceConfig()
	rules, err := model.ListSelfServiceUpgradeRules(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	stats, err := model.GetSelfServiceStats(model.TodayStartTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"config": config,
		"rules":  rules,
		"stats":  stats,
	})
}

func UpdateSelfServiceAdminConfig(c *gin.Context) {
	var config SelfServiceConfig
	if err := common.DecodeJson(c.Request.Body, &config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := saveSelfServiceConfig(config); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, getSelfServiceConfig())
}

func UpdateSelfServiceUpgradeRules(c *gin.Context) {
	var req selfServiceRulesUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	groups := ratio_setting.GetGroupRatioCopy()
	rules := make([]*model.SelfServiceUpgradeRule, 0, len(req.Rules))
	for _, payload := range req.Rules {
		targetGroup := strings.TrimSpace(payload.TargetGroup)
		if targetGroup == "" || payload.ThresholdQuota <= 0 {
			continue
		}
		if _, ok := groups[targetGroup]; !ok {
			common.ApiErrorMsg(c, "Invalid target group: "+targetGroup)
			return
		}
		rules = append(rules, &model.SelfServiceUpgradeRule{
			ThresholdQuota: payload.ThresholdQuota,
			TargetGroup:    targetGroup,
			Description:    strings.TrimSpace(payload.Description),
			Enabled:        true,
		})
	}
	if len(rules) > 50 {
		common.ApiErrorMsg(c, "Too many upgrade rules")
		return
	}
	if err := model.ReplaceSelfServiceUpgradeRules(rules); err != nil {
		common.ApiError(c, err)
		return
	}
	updated, err := model.ListSelfServiceUpgradeRules(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, updated)
}

func GetSelfServiceRefundHistories(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId, _ := strconv.Atoi(c.Query("user_id"))
	status := strings.TrimSpace(c.Query("status"))
	items, total, err := model.ListSelfServiceRefundHistories(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), userId, status)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetSelfServiceClaimAttempts(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId, _ := strconv.Atoi(c.Query("user_id"))
	status := strings.TrimSpace(c.Query("status"))
	items, total, err := model.ListSelfServiceClaimAttempts(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), userId, status)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetSelfServiceUpgradeHistories(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId, _ := strconv.Atoi(c.Query("user_id"))
	items, total, err := model.ListSelfServiceUpgradeHistories(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}
