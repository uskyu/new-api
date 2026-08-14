package model

import (
	"errors"
	"math/rand"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// Checkin 签到记录
type Checkin struct {
	Id           int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId       int    `json:"user_id" gorm:"not null;uniqueIndex:idx_user_checkin_date"`
	CheckinDate  string `json:"checkin_date" gorm:"type:varchar(10);not null;uniqueIndex:idx_user_checkin_date"` // 格式: YYYY-MM-DD
	QuotaAwarded int    `json:"quota_awarded" gorm:"not null"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
}

// CheckinRecord 用于API返回的签到记录（不包含敏感字段）
type CheckinRecord struct {
	CheckinDate  string `json:"checkin_date"`
	QuotaAwarded int    `json:"quota_awarded"`
}

func (Checkin) TableName() string {
	return "checkins"
}

// GetUserCheckinRecords 获取用户在指定日期范围内的签到记录
func GetUserCheckinRecords(userId int, startDate, endDate string) ([]Checkin, error) {
	var records []Checkin
	err := DB.Where("user_id = ? AND checkin_date >= ? AND checkin_date <= ?",
		userId, startDate, endDate).
		Order("checkin_date DESC").
		Find(&records).Error
	return records, err
}

// HasCheckedInToday 检查用户今天是否已签到
func HasCheckedInToday(userId int) (bool, error) {
	today := time.Now().Format("2006-01-02")
	var count int64
	err := DB.Model(&Checkin{}).
		Where("user_id = ? AND checkin_date = ?", userId, today).
		Count(&count).Error
	return count > 0, err
}

// GetYesterdayCheckinUsage 统计用户昨日成功计费调用次数和消耗额度
func GetYesterdayCheckinUsage(userId int) (calls int64, quota int64, err error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := startOfDay.Add(-24 * time.Hour).Unix()
	end := startOfDay.Unix()
	err = LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ? AND created_at >= ? AND created_at < ?", userId, LogTypeConsume, start, end).
		Count(&calls).Error
	if err != nil {
		return 0, 0, err
	}
	err = LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ? AND created_at >= ? AND created_at < ?", userId, LogTypeConsume, start, end).
		Select("COALESCE(SUM(quota), 0)").Scan(&quota).Error
	return calls, quota, err
}

// SelectCheckinTier 返回用户命中的最高活跃奖励档位
func SelectCheckinTier(tiers []operation_setting.CheckinBonusTier, metric int64) *operation_setting.CheckinBonusTier {
	var selected *operation_setting.CheckinBonusTier
	for i := range tiers {
		tier := &tiers[i]
		if int64(tier.Threshold) > metric {
			continue
		}
		if selected == nil || tier.Threshold > selected.Threshold {
			selected = tier
		}
	}
	return selected
}

// CheckinRewardCandidate describes the highest matched tier for one metric.
type CheckinRewardCandidate struct {
	Metric string
	Tier   *operation_setting.CheckinBonusTier
}

// CheckinRewardDecision contains both metric matches and the single selected reward source.
type CheckinRewardDecision struct {
	RequestCount  CheckinRewardCandidate
	QuotaConsumed CheckinRewardCandidate
	Selected      CheckinRewardCandidate
}

func validCheckinTier(tier *operation_setting.CheckinBonusTier) bool {
	return tier != nil && tier.MinQuota >= 0 && tier.MaxQuota >= tier.MinQuota
}

func selectTierReward(tier *operation_setting.CheckinBonusTier) int {
	if !validCheckinTier(tier) {
		return 0
	}
	if tier.MinQuota == tier.MaxQuota {
		return tier.MinQuota
	}
	return tier.MinQuota + rand.Intn(tier.MaxQuota-tier.MinQuota+1)
}

// EvaluateCheckinReward matches both metrics and selects one candidate.
// Each metric uses only its highest-threshold matched tier. Candidates are
// compared by max_quota; ties prefer request_count for deterministic behavior.
func EvaluateCheckinReward(setting *operation_setting.CheckinSetting, calls, consumedQuota int64) CheckinRewardDecision {
	decision := CheckinRewardDecision{
		RequestCount:  CheckinRewardCandidate{Metric: "request_count", Tier: nil},
		QuotaConsumed: CheckinRewardCandidate{Metric: "quota_consumed", Tier: nil},
	}
	if setting == nil {
		return decision
	}
	requestTier := SelectCheckinTier(setting.RequestCountTiers, calls)
	if validCheckinTier(requestTier) {
		decision.RequestCount.Tier = requestTier
	}
	quotaTier := SelectCheckinTier(setting.QuotaConsumedTiers, consumedQuota)
	if validCheckinTier(quotaTier) {
		decision.QuotaConsumed.Tier = quotaTier
	}

	if decision.RequestCount.Tier != nil {
		decision.Selected = decision.RequestCount
	}
	if decision.QuotaConsumed.Tier != nil &&
		(decision.Selected.Tier == nil || decision.QuotaConsumed.Tier.MaxQuota > decision.Selected.Tier.MaxQuota) {
		decision.Selected = decision.QuotaConsumed
	}
	return decision
}

// SelectCheckinReward evaluates both active metrics and randomizes exactly once
// inside the selected tier. Rewards from the two metrics are never added.
func SelectCheckinReward(setting *operation_setting.CheckinSetting, calls, consumedQuota int64) (int, CheckinRewardDecision) {
	decision := EvaluateCheckinReward(setting, calls, consumedQuota)
	return selectTierReward(decision.Selected.Tier), decision
}

// selectCheckinReward preserves the single-metric helper used by legacy tests.
func selectCheckinReward(setting *operation_setting.CheckinSetting, metric int64) int {
	if setting == nil {
		return 0
	}
	return selectTierReward(SelectCheckinTier(setting.ActiveTiers(), metric))
}

// UserCheckin 执行用户签到
// MySQL 和 PostgreSQL 使用事务保证原子性
// SQLite 不支持嵌套事务，使用顺序操作 + 手动回滚
func UserCheckin(userId int) (*Checkin, error) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		return nil, errors.New("签到功能未启用")
	}

	// 检查今天是否已签到
	hasChecked, err := HasCheckedInToday(userId)
	if err != nil {
		return nil, err
	}
	if hasChecked {
		return nil, errors.New("今日已签到")
	}

	// 计算签到奖励：两套指标同时判定，最终只发放一套奖励。
	// 不再使用基础签到额度（checkin_setting.min_quota/max_quota）。
	quotaAwarded := 0
	if len(setting.RequestCountTiers) > 0 || len(setting.QuotaConsumedTiers) > 0 {
		calls, consumedQuota, usageErr := GetYesterdayCheckinUsage(userId)
		if usageErr != nil {
			return nil, usageErr
		}
		quotaAwarded, _ = SelectCheckinReward(setting, calls, consumedQuota)
	}

	today := time.Now().Format("2006-01-02")
	checkin := &Checkin{
		UserId:       userId,
		CheckinDate:  today,
		QuotaAwarded: quotaAwarded,
		CreatedAt:    time.Now().Unix(),
	}

	// 根据数据库类型选择不同的策略
	if common.UsingSQLite {
		// SQLite 不支持嵌套事务，使用顺序操作 + 手动回滚
		return userCheckinWithoutTransaction(checkin, userId, quotaAwarded)
	}

	// MySQL 和 PostgreSQL 支持事务，使用事务保证原子性
	return userCheckinWithTransaction(checkin, userId, quotaAwarded)
}

// userCheckinWithTransaction 使用事务执行签到（适用于 MySQL 和 PostgreSQL）
func userCheckinWithTransaction(checkin *Checkin, userId int, quotaAwarded int) (*Checkin, error) {
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 步骤1: 创建签到记录
		// 数据库有唯一约束 (user_id, checkin_date)，可以防止并发重复签到
		if err := tx.Create(checkin).Error; err != nil {
			return errors.New("签到失败，请稍后重试")
		}

		// 步骤2: 在事务中增加用户额度
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("quota", gorm.Expr("quota + ?", quotaAwarded)).Error; err != nil {
			return errors.New("签到失败：更新额度出错")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 事务成功后，异步更新缓存
	go func() {
		_ = cacheIncrUserQuota(userId, int64(quotaAwarded))
	}()

	return checkin, nil
}

// userCheckinWithoutTransaction 不使用事务执行签到（适用于 SQLite）
func userCheckinWithoutTransaction(checkin *Checkin, userId int, quotaAwarded int) (*Checkin, error) {
	// 步骤1: 创建签到记录
	// 数据库有唯一约束 (user_id, checkin_date)，可以防止并发重复签到
	if err := DB.Create(checkin).Error; err != nil {
		return nil, errors.New("签到失败，请稍后重试")
	}

	// 步骤2: 增加用户额度
	// 使用 db=true 强制直接写入数据库，不使用批量更新
	if err := IncreaseUserQuota(userId, quotaAwarded, true); err != nil {
		// 如果增加额度失败，需要回滚签到记录
		DB.Delete(checkin)
		return nil, errors.New("签到失败：更新额度出错")
	}

	return checkin, nil
}

// GetUserCheckinStats 获取用户签到统计信息
func GetUserCheckinStats(userId int, month string) (map[string]interface{}, error) {
	// 获取指定月份的所有签到记录
	startDate := month + "-01"
	endDate := month + "-31"

	records, err := GetUserCheckinRecords(userId, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// 转换为不包含敏感字段的记录
	checkinRecords := make([]CheckinRecord, len(records))
	for i, r := range records {
		checkinRecords[i] = CheckinRecord{
			CheckinDate:  r.CheckinDate,
			QuotaAwarded: r.QuotaAwarded,
		}
	}

	// 检查今天是否已签到
	hasCheckedToday, _ := HasCheckedInToday(userId)

	// 获取用户所有时间的签到统计
	var totalCheckins int64
	var totalQuota int64
	DB.Model(&Checkin{}).Where("user_id = ?", userId).Count(&totalCheckins)
	DB.Model(&Checkin{}).Where("user_id = ?", userId).Select("COALESCE(SUM(quota_awarded), 0)").Scan(&totalQuota)

	return map[string]interface{}{
		"total_quota":      totalQuota,      // 所有时间累计获得的额度
		"total_checkins":   totalCheckins,   // 所有时间累计签到次数
		"checkin_count":    len(records),    // 本月签到次数
		"checked_in_today": hasCheckedToday, // 今天是否已签到
		"records":          checkinRecords,  // 本月签到记录详情（不含id和user_id）
	}, nil
}
