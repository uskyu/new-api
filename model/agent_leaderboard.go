package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const AgentLeaderboardLimit = 20

type AgentLeaderboardEntry struct {
	Rank                 int    `json:"rank"`
	AgentLabel           string `json:"agent_label"`
	IsSelf               bool   `json:"is_self"`
	DayNewUserCount      int64  `json:"day_new_user_count"`
	MonthNewUserCount    int64  `json:"month_new_user_count"`
	MonthTopupAmount     int64  `json:"month_topup_amount"`
	DownlineBalanceQuota int64  `json:"downline_balance_quota"`
}

type AgentLeaderboardResult struct {
	GeneratedAt int64                   `json:"generated_at"`
	DayStart    int64                   `json:"day_start"`
	MonthStart  int64                   `json:"month_start"`
	Items       []AgentLeaderboardEntry `json:"items"`
	Self        *AgentLeaderboardEntry  `json:"self"`
}

type agentLeaderboardRow struct {
	AgentUserId          int
	Username             string
	DayNewUserCount      int64
	MonthNewUserCount    int64
	MonthTopupAmount     int64
	DownlineBalanceQuota int64
}

func GetAgentLeaderboard(requesterUserId int) (*AgentLeaderboardResult, error) {
	return getAgentLeaderboardAt(requesterUserId, time.Now())
}

func getAgentLeaderboardAt(requesterUserId int, now time.Time) (*AgentLeaderboardResult, error) {
	if requesterUserId <= 0 {
		return nil, errors.New("invalid user id")
	}
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
			COALESCE(month_topups.topup_amount, 0) AS month_topup_amount,
			COALESCE(downline_balances.balance_quota, 0) AS downline_balance_quota`).
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
		) AS month_topups ON month_topups.agent_user_id = ap.user_id`, AgentRebateRecordSettled, monthStart.Unix(), end).
		Joins(`LEFT JOIN (
			SELECT du.inviter_id AS agent_user_id, COALESCE(SUM(du.quota), 0) AS balance_quota
			FROM users AS du
			LEFT JOIN agent_profiles AS dap ON dap.user_id = du.id
			WHERE du.inviter_id > 0 AND du.deleted_at IS NULL AND du.role = ? AND dap.id IS NULL
			GROUP BY du.inviter_id
		) AS downline_balances ON downline_balances.agent_user_id = ap.user_id`, common.RoleCommonUser).
		Where("ap.status = ?", AgentStatusEnabled).
		Order("month_new_user_count DESC").
		Order("month_topup_amount DESC").
		Order("day_new_user_count DESC").
		Order("ap.user_id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := &AgentLeaderboardResult{
		GeneratedAt: now.Unix(),
		DayStart:    dayStart.Unix(),
		MonthStart:  monthStart.Unix(),
		Items:       make([]AgentLeaderboardEntry, 0, AgentLeaderboardLimit),
	}
	for index, row := range rows {
		entry := AgentLeaderboardEntry{
			Rank:                 index + 1,
			AgentLabel:           maskAgentLeaderboardName(row.Username),
			IsSelf:               row.AgentUserId == requesterUserId,
			DayNewUserCount:      row.DayNewUserCount,
			MonthNewUserCount:    row.MonthNewUserCount,
			MonthTopupAmount:     row.MonthTopupAmount,
			DownlineBalanceQuota: row.DownlineBalanceQuota,
		}
		if index < AgentLeaderboardLimit {
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
