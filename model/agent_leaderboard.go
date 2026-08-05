package model

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	AgentLeaderboardSortDayNewUser           = "day_new_user_count"
	AgentLeaderboardSortMonthNewUser         = "month_new_user_count"
	AgentLeaderboardSortDayTopup             = "day_topup_amount"
	AgentLeaderboardSortMonthSecondTopupRate = "month_second_topup_rate"
	AgentLeaderboardSortMonthThirdTopupRate  = "month_third_topup_rate"
	AgentLeaderboardSortMonthFourthTopupRate = "month_fourth_topup_rate"
)

var agentLeaderboardSorts = []string{
	AgentLeaderboardSortDayNewUser,
	AgentLeaderboardSortMonthNewUser,
	AgentLeaderboardSortDayTopup,
	AgentLeaderboardSortMonthSecondTopupRate,
	AgentLeaderboardSortMonthThirdTopupRate,
	AgentLeaderboardSortMonthFourthTopupRate,
}

type AgentLeaderboardEntry struct {
	Rank                 int     `json:"rank"`
	AgentLabel           string  `json:"agent_label"`
	IsSelf               bool    `json:"is_self"`
	DayNewUserCount      int64   `json:"day_new_user_count"`
	DayNewUserRank       int     `json:"day_new_user_rank"`
	MonthNewUserCount    int64   `json:"month_new_user_count"`
	MonthNewUserRank     int     `json:"month_new_user_rank"`
	DayTopupAmount       int64   `json:"day_topup_amount"`
	DayTopupRank         int     `json:"day_topup_rank"`
	MonthRepurchaseRate  float64 `json:"month_repurchase_rate"`
	MonthSecondTopupRate float64 `json:"month_second_topup_rate"`
	MonthSecondTopupRank int     `json:"month_second_topup_rank"`
	MonthThirdTopupRate  float64 `json:"month_third_topup_rate"`
	MonthThirdTopupRank  int     `json:"month_third_topup_rank"`
	MonthFourthTopupRate float64 `json:"month_fourth_topup_rate"`
	MonthFourthTopupRank int     `json:"month_fourth_topup_rank"`
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
	SortBy      string                  `json:"sort_by"`
}

type agentLeaderboardRow struct {
	AgentUserId               int
	Username                  string
	DayNewUserCount           int64
	MonthNewUserCount         int64
	DayTopupAmount            int64
	MonthTopupUserCount       int64
	MonthSecondTopupUserCount int64
	MonthThirdTopupUserCount  int64
	MonthFourthTopupUserCount int64
}

type agentLeaderboardComputedEntry struct {
	AgentUserId int
	Entry       AgentLeaderboardEntry
}

func GetAgentLeaderboard(requesterUserId int, pageInfo *common.PageInfo, sortBy string) (*AgentLeaderboardResult, error) {
	return getAgentLeaderboardAt(requesterUserId, pageInfo, sortBy, time.Now())
}

func getAgentLeaderboardAt(requesterUserId int, pageInfo *common.PageInfo, sortBy string, now time.Time) (*AgentLeaderboardResult, error) {
	if requesterUserId <= 0 || pageInfo == nil {
		return nil, errors.New("invalid leaderboard query")
	}
	page, pageSize := normalizeAgentPageInfo(pageInfo)
	sortBy = normalizeAgentLeaderboardSort(sortBy)
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
			COALESCE(month_repurchase.second_topup_user_count, 0) AS month_second_topup_user_count,
			COALESCE(month_repurchase.third_topup_user_count, 0) AS month_third_topup_user_count,
			COALESCE(month_repurchase.fourth_topup_user_count, 0) AS month_fourth_topup_user_count`).
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
			WHERE status = ? AND source_type = ? AND settled_at >= ? AND settled_at < ?
			GROUP BY agent_user_id
		) AS day_topups ON day_topups.agent_user_id = ap.user_id`, AgentRebateRecordSettled, AgentRebateSourceEPay, dayStart.Unix(), end).
		Joins(`LEFT JOIN (
			SELECT agent_user_id,
				COUNT(*) AS topup_user_count,
				COALESCE(SUM(CASE WHEN topup_count >= 2 THEN 1 ELSE 0 END), 0) AS second_topup_user_count,
				COALESCE(SUM(CASE WHEN topup_count >= 3 THEN 1 ELSE 0 END), 0) AS third_topup_user_count,
				COALESCE(SUM(CASE WHEN topup_count >= 4 THEN 1 ELSE 0 END), 0) AS fourth_topup_user_count
			FROM (
				SELECT agent_user_id, invitee_user_id, COUNT(id) AS topup_count
				FROM agent_rebate_records
				WHERE status = ? AND source_type = ? AND settled_at >= ? AND settled_at < ?
				GROUP BY agent_user_id, invitee_user_id
			) AS month_user_topups
			GROUP BY agent_user_id
		) AS month_repurchase ON month_repurchase.agent_user_id = ap.user_id`, AgentRebateRecordSettled, AgentRebateSourceEPay, monthStart.Unix(), end).
		Where("ap.status = ?", AgentStatusEnabled).
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
		SortBy:      sortBy,
	}
	computedEntries := make([]agentLeaderboardComputedEntry, 0, len(rows))
	for _, row := range rows {
		secondTopupRate := analyticsRatio(float64(row.MonthSecondTopupUserCount), float64(row.MonthTopupUserCount))
		computedEntries = append(computedEntries, agentLeaderboardComputedEntry{
			AgentUserId: row.AgentUserId,
			Entry: AgentLeaderboardEntry{
				AgentLabel:           maskAgentLeaderboardName(row.Username),
				IsSelf:               row.AgentUserId == requesterUserId,
				DayNewUserCount:      row.DayNewUserCount,
				MonthNewUserCount:    row.MonthNewUserCount,
				DayTopupAmount:       row.DayTopupAmount,
				MonthRepurchaseRate:  secondTopupRate,
				MonthSecondTopupRate: secondTopupRate,
				MonthThirdTopupRate:  analyticsRatio(float64(row.MonthThirdTopupUserCount), float64(row.MonthTopupUserCount)),
				MonthFourthTopupRate: analyticsRatio(float64(row.MonthFourthTopupUserCount), float64(row.MonthTopupUserCount)),
			},
		})
	}
	for _, metric := range agentLeaderboardSorts {
		assignAgentLeaderboardDenseRanks(computedEntries, metric)
	}
	sortAgentLeaderboardEntries(computedEntries, sortBy)

	pageStart := (page - 1) * pageSize
	pageEnd := pageStart + pageSize
	for index := range computedEntries {
		entry := computedEntries[index].Entry
		entry.Rank = index + 1
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

func normalizeAgentLeaderboardSort(sortBy string) string {
	sortBy = strings.TrimSpace(sortBy)
	for _, allowed := range agentLeaderboardSorts {
		if sortBy == allowed {
			return sortBy
		}
	}
	return AgentLeaderboardSortDayNewUser
}

func agentLeaderboardMetricValue(entry AgentLeaderboardEntry, sortBy string) float64 {
	switch sortBy {
	case AgentLeaderboardSortMonthNewUser:
		return float64(entry.MonthNewUserCount)
	case AgentLeaderboardSortDayTopup:
		return float64(entry.DayTopupAmount)
	case AgentLeaderboardSortMonthSecondTopupRate:
		return entry.MonthSecondTopupRate
	case AgentLeaderboardSortMonthThirdTopupRate:
		return entry.MonthThirdTopupRate
	case AgentLeaderboardSortMonthFourthTopupRate:
		return entry.MonthFourthTopupRate
	default:
		return float64(entry.DayNewUserCount)
	}
}

func setAgentLeaderboardMetricRank(entry *AgentLeaderboardEntry, sortBy string, rank int) {
	switch sortBy {
	case AgentLeaderboardSortMonthNewUser:
		entry.MonthNewUserRank = rank
	case AgentLeaderboardSortDayTopup:
		entry.DayTopupRank = rank
	case AgentLeaderboardSortMonthSecondTopupRate:
		entry.MonthSecondTopupRank = rank
	case AgentLeaderboardSortMonthThirdTopupRate:
		entry.MonthThirdTopupRank = rank
	case AgentLeaderboardSortMonthFourthTopupRate:
		entry.MonthFourthTopupRank = rank
	default:
		entry.DayNewUserRank = rank
	}
}

func sortAgentLeaderboardEntries(entries []agentLeaderboardComputedEntry, sortBy string) {
	sort.SliceStable(entries, func(i, j int) bool {
		left := agentLeaderboardMetricValue(entries[i].Entry, sortBy)
		right := agentLeaderboardMetricValue(entries[j].Entry, sortBy)
		if left != right {
			return left > right
		}
		return entries[i].AgentUserId < entries[j].AgentUserId
	})
}

func assignAgentLeaderboardDenseRanks(entries []agentLeaderboardComputedEntry, sortBy string) {
	indices := make([]int, len(entries))
	for index := range entries {
		indices[index] = index
	}
	sort.SliceStable(indices, func(i, j int) bool {
		leftIndex := indices[i]
		rightIndex := indices[j]
		left := agentLeaderboardMetricValue(entries[leftIndex].Entry, sortBy)
		right := agentLeaderboardMetricValue(entries[rightIndex].Entry, sortBy)
		if left != right {
			return left > right
		}
		return entries[leftIndex].AgentUserId < entries[rightIndex].AgentUserId
	})
	rank := 0
	lastValue := 0.0
	for position, entryIndex := range indices {
		value := agentLeaderboardMetricValue(entries[entryIndex].Entry, sortBy)
		if position == 0 || value != lastValue {
			rank++
			lastValue = value
		}
		setAgentLeaderboardMetricRank(&entries[entryIndex].Entry, sortBy, rank)
	}
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
