package model

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	InactiveAccountTypeAll    = "all"
	InactiveAccountTypeUsers  = "users"
	InactiveAccountTypeAgents = "agents"
	MaxInactiveAnalyticsDays  = 3650
)

type InactiveUserAnalytics struct {
	Days     int                        `json:"days"`
	Cutoff   int64                      `json:"cutoff"`
	Summary  InactiveUserSummary        `json:"summary"`
	Items    []InactiveUserAnalyticsRow `json:"items"`
	Total    int64                      `json:"total"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"page_size"`
}

type InactiveUserSummary struct {
	EligibleUserCount    int64   `json:"eligible_user_count"`
	InactiveUserCount    int64   `json:"inactive_user_count"`
	InactiveBalanceQuota int64   `json:"inactive_balance_quota"`
	NeverLoggedInCount   int64   `json:"never_logged_in_count"`
	NeverLoggedInBalance int64   `json:"never_logged_in_balance"`
	InactiveRatio        float64 `json:"inactive_ratio"`
}

type InactiveUserAnalyticsRow struct {
	Id              int    `json:"id"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	Quota           int64  `json:"quota"`
	UsedQuota       int64  `json:"used_quota"`
	CreatedAt       int64  `json:"created_at"`
	LastLoginAt     int64  `json:"last_login_at"`
	InactiveDays    int    `json:"inactive_days"`
	InviterId       int    `json:"inviter_id"`
	InviterUsername string `json:"inviter_username"`
	IsAgent         bool   `json:"is_agent"`
}

type InactiveUserAnalyticsOptions struct {
	Days        int
	Keyword     string
	AccountType string
}

func GetInactiveUserAnalytics(pageInfo *common.PageInfo, options InactiveUserAnalyticsOptions) (*InactiveUserAnalytics, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	if pageInfo == nil || options.Days <= 0 || options.Days > MaxInactiveAnalyticsDays {
		return nil, errors.New("invalid inactive user query")
	}
	if options.AccountType != InactiveAccountTypeAll && options.AccountType != InactiveAccountTypeUsers && options.AccountType != InactiveAccountTypeAgents {
		return nil, errors.New("invalid account type")
	}

	now := common.GetTimestamp()
	cutoff := now - int64(options.Days)*24*3600
	base := DB.Table("users AS u").
		Joins("LEFT JOIN agent_profiles AS ap ON ap.user_id = u.id").
		Where("u.deleted_at IS NULL").
		Where("u.role = ? AND u.status = ?", common.RoleCommonUser, common.UserStatusEnabled)
	base = applyInactiveAccountType(base, options.AccountType)

	var eligibleCount int64
	if err := base.Session(&gorm.Session{}).Count(&eligibleCount).Error; err != nil {
		return nil, err
	}

	inactive := base.
		Where("u.created_at <= ?", cutoff).
		Where("(COALESCE(u.last_login_at, 0) = 0 OR u.last_login_at <= ?)", cutoff)
	keyword := strings.TrimSpace(options.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		if id, err := strconv.Atoi(keyword); err == nil {
			inactive = inactive.Where("(u.id = ? OR u.username LIKE ? OR u.display_name LIKE ?)", id, like, like)
		} else {
			inactive = inactive.Where("(u.username LIKE ? OR u.display_name LIKE ?)", like, like)
		}
	}

	var summary struct {
		InactiveUserCount    int64 `gorm:"column:inactive_user_count"`
		InactiveBalanceQuota int64 `gorm:"column:inactive_balance_quota"`
		NeverLoggedCount     int64 `gorm:"column:never_logged_count"`
		NeverLoggedBalance   int64 `gorm:"column:never_logged_balance"`
	}
	if err := inactive.Session(&gorm.Session{}).Select(`COUNT(*) AS inactive_user_count,
		COALESCE(SUM(u.quota), 0) AS inactive_balance_quota,
		COALESCE(SUM(CASE WHEN COALESCE(u.last_login_at, 0) = 0 THEN 1 ELSE 0 END), 0) AS never_logged_count,
		COALESCE(SUM(CASE WHEN COALESCE(u.last_login_at, 0) = 0 THEN u.quota ELSE 0 END), 0) AS never_logged_balance`).
		Scan(&summary).Error; err != nil {
		return nil, err
	}

	var items []InactiveUserAnalyticsRow
	if err := inactive.Session(&gorm.Session{}).
		Select(`u.id, u.username, u.display_name, u.quota, u.used_quota, u.created_at,
			COALESCE(u.last_login_at, 0) AS last_login_at, u.inviter_id,
			COALESCE(iu.username, '') AS inviter_username,
			CASE WHEN ap.id IS NULL THEN 0 ELSE 1 END AS is_agent`).
		Joins("LEFT JOIN users AS iu ON iu.id = u.inviter_id").
		Order("COALESCE(u.last_login_at, 0) ASC").
		Order("u.quota DESC").
		Order("u.id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		lastActivityAt := items[i].LastLoginAt
		if lastActivityAt <= 0 {
			lastActivityAt = items[i].CreatedAt
		}
		if lastActivityAt > 0 && lastActivityAt <= now {
			items[i].InactiveDays = int((now - lastActivityAt) / (24 * 3600))
		}
	}

	return &InactiveUserAnalytics{
		Days:   options.Days,
		Cutoff: cutoff,
		Summary: InactiveUserSummary{
			EligibleUserCount:    eligibleCount,
			InactiveUserCount:    summary.InactiveUserCount,
			InactiveBalanceQuota: summary.InactiveBalanceQuota,
			NeverLoggedInCount:   summary.NeverLoggedCount,
			NeverLoggedInBalance: summary.NeverLoggedBalance,
			InactiveRatio:        analyticsRatio(float64(summary.InactiveUserCount), float64(eligibleCount)),
		},
		Items:    items,
		Total:    summary.InactiveUserCount,
		Page:     pageInfo.GetPage(),
		PageSize: pageInfo.GetPageSize(),
	}, nil
}

func applyInactiveAccountType(query *gorm.DB, accountType string) *gorm.DB {
	switch accountType {
	case InactiveAccountTypeUsers:
		return query.Where("ap.id IS NULL")
	case InactiveAccountTypeAgents:
		return query.Where("ap.id IS NOT NULL")
	default:
		return query
	}
}
