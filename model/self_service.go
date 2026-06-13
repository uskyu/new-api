package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	SelfServiceRefundStatusSuccess = "success"
	SelfServiceRefundStatusSkipped = "skipped"
	SelfServiceRefundStatusFailed  = "failed"

	SelfServiceAttemptStatusSuccess = "success"
	SelfServiceAttemptStatusFailed  = "failed"

	SelfServiceUpgradeStatusSuccess = "success"
	SelfServiceUpgradeStatusFailed  = "failed"
)

type SelfServiceRefundHistory struct {
	Id            int    `json:"id"`
	UserId        int    `json:"user_id" gorm:"index:idx_self_refund_user_time,priority:1"`
	Username      string `json:"username" gorm:"type:varchar(64);default:''"`
	LogId         int    `json:"log_id" gorm:"uniqueIndex"`
	ModelName     string `json:"model_name" gorm:"index;default:''"`
	OriginalQuota int    `json:"original_quota" gorm:"default:0"`
	RefundedQuota int    `json:"refunded_quota" gorm:"default:0"`
	RequestId     string `json:"request_id" gorm:"type:varchar(64);index;default:''"`
	Status        string `json:"status" gorm:"type:varchar(32);index;default:'success'"`
	Message       string `json:"message" gorm:"type:varchar(255);default:''"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;index:idx_self_refund_user_time,priority:2"`
}

type SelfServiceClaimAttempt struct {
	Id                int    `json:"id"`
	UserId            int    `json:"user_id" gorm:"index:idx_self_attempt_user_time,priority:1"`
	Username          string `json:"username" gorm:"type:varchar(64);default:''"`
	ScannedCount      int    `json:"scanned_count" gorm:"default:0"`
	CandidateCount    int    `json:"candidate_count" gorm:"default:0"`
	NewCandidateCount int    `json:"new_candidate_count" gorm:"default:0"`
	RefundedCount     int    `json:"refunded_count" gorm:"default:0"`
	RefundedQuota     int    `json:"refunded_quota" gorm:"default:0"`
	Status            string `json:"status" gorm:"type:varchar(32);index;default:'success'"`
	Message           string `json:"message" gorm:"type:varchar(255);default:''"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index:idx_self_attempt_user_time,priority:2"`
}

type SelfServiceUpgradeRule struct {
	Id             int    `json:"id"`
	ThresholdQuota int    `json:"threshold_quota" gorm:"index"`
	TargetGroup    string `json:"target_group" gorm:"type:varchar(64);uniqueIndex"`
	Description    string `json:"description" gorm:"type:varchar(255);default:''"`
	Enabled        bool   `json:"enabled" gorm:"index;default:true"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint"`
}

type SelfServiceUpgradeHistory struct {
	Id         int    `json:"id"`
	UserId     int    `json:"user_id" gorm:"index:idx_self_upgrade_user_time,priority:1"`
	Username   string `json:"username" gorm:"type:varchar(64);default:''"`
	FromGroup  string `json:"from_group" gorm:"type:varchar(64);default:''"`
	ToGroup    string `json:"to_group" gorm:"type:varchar(64);default:''"`
	TotalQuota int    `json:"total_quota" gorm:"default:0"`
	RuleId     int    `json:"rule_id" gorm:"index;default:0"`
	Status     string `json:"status" gorm:"type:varchar(32);index;default:'success'"`
	Message    string `json:"message" gorm:"type:varchar(255);default:''"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint;index:idx_self_upgrade_user_time,priority:2"`
}

func (h *SelfServiceRefundHistory) BeforeCreate(tx *gorm.DB) error {
	if h.CreatedAt == 0 {
		h.CreatedAt = common.GetTimestamp()
	}
	return nil
}

func (a *SelfServiceClaimAttempt) BeforeCreate(tx *gorm.DB) error {
	if a.CreatedAt == 0 {
		a.CreatedAt = common.GetTimestamp()
	}
	return nil
}

func (r *SelfServiceUpgradeRule) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	if r.UpdatedAt == 0 {
		r.UpdatedAt = now
	}
	return nil
}

func (r *SelfServiceUpgradeRule) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

func (h *SelfServiceUpgradeHistory) BeforeCreate(tx *gorm.DB) error {
	if h.CreatedAt == 0 {
		h.CreatedAt = common.GetTimestamp()
	}
	return nil
}

func GetSelfServiceRefundHistory(logId int) (*SelfServiceRefundHistory, error) {
	var history SelfServiceRefundHistory
	err := DB.Where("log_id = ?", logId).First(&history).Error
	return &history, err
}

func ListSelfServiceRefundHistories(startIdx int, num int, userId int, status string) ([]*SelfServiceRefundHistory, int64, error) {
	var histories []*SelfServiceRefundHistory
	var total int64
	tx := DB.Model(&SelfServiceRefundHistory{})
	if userId > 0 {
		tx = tx.Where("user_id = ?", userId)
	}
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("id desc").Limit(num).Offset(startIdx).Find(&histories).Error
	return histories, total, err
}

func ListSelfServiceClaimAttempts(startIdx int, num int, userId int, status string) ([]*SelfServiceClaimAttempt, int64, error) {
	var attempts []*SelfServiceClaimAttempt
	var total int64
	tx := DB.Model(&SelfServiceClaimAttempt{})
	if userId > 0 {
		tx = tx.Where("user_id = ?", userId)
	}
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("id desc").Limit(num).Offset(startIdx).Find(&attempts).Error
	return attempts, total, err
}

func ListSelfServiceUpgradeHistories(startIdx int, num int, userId int) ([]*SelfServiceUpgradeHistory, int64, error) {
	var histories []*SelfServiceUpgradeHistory
	var total int64
	tx := DB.Model(&SelfServiceUpgradeHistory{})
	if userId > 0 {
		tx = tx.Where("user_id = ?", userId)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("id desc").Limit(num).Offset(startIdx).Find(&histories).Error
	return histories, total, err
}

func ListSelfServiceUpgradeRules(includeDisabled bool) ([]*SelfServiceUpgradeRule, error) {
	return ListSelfServiceUpgradeRulesTx(DB, includeDisabled)
}

func ListSelfServiceUpgradeRulesTx(tx *gorm.DB, includeDisabled bool) ([]*SelfServiceUpgradeRule, error) {
	if tx == nil {
		tx = DB
	}
	var rules []*SelfServiceUpgradeRule
	query := tx.Model(&SelfServiceUpgradeRule{})
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}
	err := query.Order("threshold_quota asc, id asc").Find(&rules).Error
	return rules, err
}

func ReplaceSelfServiceUpgradeRules(rules []*SelfServiceUpgradeRule) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&SelfServiceUpgradeRule{}).Where("enabled = ?", true).Update("enabled", false).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		for _, rule := range rules {
			if rule.TargetGroup == "" || rule.ThresholdQuota <= 0 {
				continue
			}
			var existing SelfServiceUpgradeRule
			err := tx.Where("target_group = ?", rule.TargetGroup).First(&existing).Error
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					rule.Enabled = true
					rule.CreatedAt = now
					rule.UpdatedAt = now
					if err := tx.Create(rule).Error; err != nil {
						return err
					}
					continue
				}
				return err
			}
			updates := map[string]interface{}{
				"threshold_quota": rule.ThresholdQuota,
				"description":     rule.Description,
				"enabled":         true,
				"updated_at":      now,
			}
			if err := tx.Model(&SelfServiceUpgradeRule{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func GetSelfServiceStats(todayStart int64) (map[string]int64, error) {
	stats := map[string]int64{}
	var todayRefunds int64
	var todayRefundQuota int64
	var totalRefunds int64
	var totalUpgrades int64

	if err := DB.Model(&SelfServiceRefundHistory{}).
		Where("status = ? AND created_at >= ?", SelfServiceRefundStatusSuccess, todayStart).
		Count(&todayRefunds).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&SelfServiceRefundHistory{}).
		Where("status = ? AND created_at >= ?", SelfServiceRefundStatusSuccess, todayStart).
		Select("COALESCE(SUM(refunded_quota), 0)").Scan(&todayRefundQuota).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&SelfServiceRefundHistory{}).
		Where("status = ?", SelfServiceRefundStatusSuccess).
		Count(&totalRefunds).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&SelfServiceUpgradeHistory{}).
		Where("status = ?", SelfServiceUpgradeStatusSuccess).
		Count(&totalUpgrades).Error; err != nil {
		return nil, err
	}

	stats["today_refunds"] = todayRefunds
	stats["today_refund_quota"] = todayRefundQuota
	stats["total_refunds"] = totalRefunds
	stats["total_upgrades"] = totalUpgrades
	return stats, nil
}

func CountUserSelfServiceRefundsSince(userId int, since int64) (int64, error) {
	var count int64
	err := DB.Model(&SelfServiceRefundHistory{}).
		Where("user_id = ? AND status = ? AND created_at >= ?", userId, SelfServiceRefundStatusSuccess, since).
		Count(&count).Error
	return count, err
}

func SumUserSelfServiceRefundQuota(userId int) (int64, error) {
	var total int64
	err := DB.Model(&SelfServiceRefundHistory{}).
		Where("user_id = ? AND status = ?", userId, SelfServiceRefundStatusSuccess).
		Select("COALESCE(SUM(refunded_quota), 0)").Scan(&total).Error
	return total, err
}

func CountSelfServiceRefundsSinceTx(tx *gorm.DB, userId int, since int64) (int64, error) {
	if tx == nil {
		tx = DB
	}
	var count int64
	err := tx.Model(&SelfServiceRefundHistory{}).
		Where("user_id = ? AND status = ? AND created_at >= ?", userId, SelfServiceRefundStatusSuccess, since).
		Count(&count).Error
	return count, err
}

func FindSelfServiceEmptyOutputLogs(userId int, startTimestamp int64, endTimestamp int64, limit int, excludeModels []string) ([]*Log, int, error) {
	var logs []*Log
	query := LOG_DB.Model(&Log{}).
		Where("user_id = ?", userId).
		Where("type = ?", LogTypeConsume).
		Where("quota > ?", 0).
		Where("completion_tokens = ?", 0).
		Where("created_at >= ? AND created_at <= ?", startTimestamp, endTimestamp)

	for _, pattern := range excludeModels {
		if pattern == "" {
			continue
		}
		query = query.Where("LOWER(model_name) NOT LIKE ?", "%"+pattern+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").Limit(limit).Find(&logs).Error
	return logs, int(total), err
}

func TodayStartTimestamp() int64 {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return start.Unix()
}
