package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type AgentLeaderboardEntry struct {
	Rank                int     `json:"rank"`
	AgentLabel          string  `json:"agent_label"`
	IsSelf              bool    `json:"is_self"`
	DayNewUserCount     int64   `json:"day_new_user_count"`
	MonthNewUserCount   int64   `json:"month_new_user_count"`
	DayTopupAmount      int64   `json:"day_topup_amount"`
	MonthRepurchaseRate float64 `json:"month_repurchase_rate"`
}

type AgentLeaderboardResult struct {
	GeneratedAt int64                   `json:"generated_at"`
	DayStart    int64                   `json:"day_start"`
	MonthStart  int64                   `json:"month_start"`
	Items       []AgentLeaderboardEntry `json:"items"`
	Self        *AgentLeaderboardEntry  `json:"self"`
	Total       int64                   `json:"total"`
	Page        int                     `json:"page"`
	PageSize    int                     `json:"page_size"`
}

type agentLeaderboardRow struct {
	AgentUserId               int
	Username                  string
	DayNewUserCount           int64
	MonthNewUserCount         int64
	DayTopupAmount            int64
	MonthTopupUserCount       int64
	MonthRepeatTopupUserCount int64
}

func GetAgentLeaderboard(requesterUserId int, pageInfo *common.PageInfo) (*AgentLeaderboardResult, error) {
	return getAgentLeaderboardAt(requesterUserId, pageInfo, time.Now())
}

func getAgentLeaderboardAt(requesterUserId int, pageInfo *common.PageInfo, now time.Time) (*AgentLeaderboardResult, error) {
	if requesterUserId <= 0 || pageInfo == nil {
		return nil, errors.New("invalid leaderboard query")
	}
	page, pageSize := normalizeAgentPageInfo(pageInfo)
	var requesterProfile AgentProfile
	if err := DB.Where("user_id = ? AND status = ?", requesterUserId, AgentStatusEnabled).First(&requesterProfile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("current user is not an enabled agent")
		}
		return nil, err
	}

	location := now.Location()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	end := now.Unix() + 1
	rows := make([]agentLeaderboardRow, 0)
	err := DB.Table("agent_profiles AS ap").
		Select(`ap.user_id AS agent_user_id,
			u.username,
			COALESCE(day_users.new_user_count, 0) AS day_new_user_count,
			COALESCE(month_users.new_user_count, 0) AS month_new_user_count,
			COALESCE(day_topups.topup_amount, 0) AS day_topup_amount,
			COALESCE(month_repurchase.topup_user_count, 0) AS month_topup_user_count,
			COALESCE(month_repurchase.repeat_topup_user_count, 0) AS month_repeat_topup_user_count`).
		Joins("JOIN users AS u ON u.id = ap.user_id AND u.deleted_at IS NULL").
		Joins(`LEFT JOIN (
			SELECT inviter_id AS agent_user_id, COUNT(*) AS new_user_count
			FROM users
			WHERE inviter_id > 0 AND deleted_at IS NULL AND created_at >= ? AND created_at < ?
			GROUP BY inviter_id
		) AS day_users ON day_users.agent_user_id = ap.user_id`, dayStart.Unix(), end).
		Joins(`LEFT JOIN (
			SELECT inviter_id AS agent_user_id, COUNT(*) AS new_user_count
			FROM users
			WHERE inviter_id > 0 AND deleted_at IS NULL AND created_at >= ? AND created_at < ?
			GROUP BY inviter_id
		) AS month_users ON month_users.agent_user_id = ap.user_id`, monthStart.Unix(), end).
		Joins(`LEFT JOIN (
			SELECT agent_user_id, COALESCE(SUM(pay_amount), 0) AS topup_amount
			FROM agent_rebate_records
			WHERE status = ? AND settled_at >= ? AND settled_at < ?
			GROUP BY agent_user_id
		) AS day_topups ON day_topups.agent_user_id = ap.user_id`, AgentRebateRecordSettled, dayStart.Unix(), end).
		Joins(`LEFT JOIN (
			SELECT agent_user_id,
				COUNT(*) AS topup_user_count,
				COALESCE(SUM(CASE WHEN topup_count >= 2 THEN 1 ELSE 0 END), 0) AS repeat_topup_user_count
			FROM (
				SELECT agent_user_id, invitee_user_id, COUNT(id) AS topup_count
				FROM agent_rebate_records
				WHERE status = ? AND settled_at >= ? AND settled_at < ?
				GROUP BY agent_user_id, invitee_user_id
			) AS month_user_topups
			GROUP BY agent_user_id
		) AS month_repurchase ON month_repurchase.agent_user_id = ap.user_id`, AgentRebateRecordSettled, monthStart.Unix(), end).
		Where("ap.status = ?", AgentStatusEnabled).
		Order("month_new_user_count DESC").
		Order("day_new_user_count DESC").
		Order("day_topup_amount DESC").
		Order("month_repeat_topup_user_count DESC").
		Order("ap.user_id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := &AgentLeaderboardResult{
		GeneratedAt: now.Unix(),
		DayStart:    dayStart.Unix(),
		MonthStart:  monthStart.Unix(),
		Items:       make([]AgentLeaderboardEntry, 0, pageSize),
		Total:       int64(len(rows)),
		Page:        page,
		PageSize:    pageSize,
	}
	pageStart := (page - 1) * pageSize
	pageEnd := pageStart + pageSize
	for index, row := range rows {
		entry := AgentLeaderboardEntry{
			Rank:                index + 1,
			AgentLabel:          maskAgentLeaderboardName(row.Username),
			IsSelf:              row.AgentUserId == requesterUserId,
			DayNewUserCount:     row.DayNewUserCount,
			MonthNewUserCount:   row.MonthNewUserCount,
			DayTopupAmount:      row.DayTopupAmount,
			MonthRepurchaseRate: analyticsRatio(float64(row.MonthRepeatTopupUserCount), float64(row.MonthTopupUserCount)),
		}
		if index >= pageStart && index < pageEnd {
			result.Items = append(result.Items, entry)
		}
		if entry.IsSelf {
			self := entry
			result.Self = &self
		}
	}
	return result, nil
}

func maskAgentLeaderboardName(username string) string {
	name := []rune(strings.TrimSpace(username))
	switch len(name) {
	case 0:
		return "***"
	case 1:
		return string(name[0]) + "*"
	case 2:
		return string(name[0]) + "*"
	case 3:
		return string(name[0]) + "*" + string(name[2])
	default:
		return string(name[:2]) + "***" + string(name[len(name)-2:])
	}
}
