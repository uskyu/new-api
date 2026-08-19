package model

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	AgentLeaderboardSortRangeNewUser    = "range_new_user_count"
	AgentLeaderboardSortRangeTopup      = "range_topup_amount"
	AgentLeaderboardSortSecondTopupRate = "range_second_topup_rate"
	AgentLeaderboardSortThirdTopupRate  = "range_third_topup_rate"
	AgentLeaderboardSortFourthTopupRate = "range_fourth_topup_rate"

	AgentLeaderboardSortDayNewUser           = "day_new_user_count"
	AgentLeaderboardSortMonthNewUser         = "month_new_user_count"
	AgentLeaderboardSortDayTopup             = "day_topup_amount"
	AgentLeaderboardSortMonthSecondTopupRate = "month_second_topup_rate"
	AgentLeaderboardSortMonthThirdTopupRate  = "month_third_topup_rate"
	AgentLeaderboardSortMonthFourthTopupRate = "month_fourth_topup_rate"
)

var agentLeaderboardSorts = []string{
	AgentLeaderboardSortRangeNewUser,
	AgentLeaderboardSortRangeTopup,
	AgentLeaderboardSortSecondTopupRate,
	AgentLeaderboardSortThirdTopupRate,
	AgentLeaderboardSortFourthTopupRate,
}

type AgentLeaderboardEntry struct {
	Rank                 int     `json:"rank"`
	RowKey               string  `json:"row_key"`
	AgentLabel           string  `json:"agent_label"`
	IsSelf               bool    `json:"is_self"`
	RangeNewUserCount    int64   `json:"range_new_user_count"`
	RangeNewUserRank     int     `json:"range_new_user_rank"`
	RangeTopupAmount     int64   `json:"range_topup_amount"`
	RangeTopupAmountRank int     `json:"range_topup_amount_rank"`
	RangeSecondTopupRate float64 `json:"range_second_topup_rate"`
	RangeSecondTopupRank int     `json:"range_second_topup_rank"`
	RangeThirdTopupRate  float64 `json:"range_third_topup_rate"`
	RangeThirdTopupRank  int     `json:"range_third_topup_rank"`
	RangeFourthTopupRate float64 `json:"range_fourth_topup_rate"`
	RangeFourthTopupRank int     `json:"range_fourth_topup_rank"`

	// Deprecated aliases kept for older clients while they migrate to range_* fields.
	DayNewUserCount      int64   `json:"day_new_user_count,omitempty"`
	DayNewUserRank       int     `json:"day_new_user_rank,omitempty"`
	MonthNewUserCount    int64   `json:"month_new_user_count,omitempty"`
	MonthNewUserRank     int     `json:"month_new_user_rank,omitempty"`
	DayTopupAmount       int64   `json:"day_topup_amount,omitempty"`
	DayTopupRank         int     `json:"day_topup_rank,omitempty"`
	MonthRepurchaseRate  float64 `json:"month_repurchase_rate,omitempty"`
	MonthSecondTopupRate float64 `json:"month_second_topup_rate,omitempty"`
	MonthSecondTopupRank int     `json:"month_second_topup_rank,omitempty"`
	MonthThirdTopupRate  float64 `json:"month_third_topup_rate,omitempty"`
	MonthThirdTopupRank  int     `json:"month_third_topup_rank,omitempty"`
	MonthFourthTopupRate float64 `json:"month_fourth_topup_rate,omitempty"`
	MonthFourthTopupRank int     `json:"month_fourth_topup_rank,omitempty"`
}

type AgentLeaderboardResult struct {
	GeneratedAt      int64                   `json:"generated_at"`
	StartDate        string                  `json:"start_date"`
	EndDate          string                  `json:"end_date"`
	EndExclusiveDate string                  `json:"end_exclusive_date,omitempty"`
	DayStart         int64                   `json:"day_start,omitempty"`
	MonthStart       int64                   `json:"month_start,omitempty"`
	Items            []AgentLeaderboardEntry `json:"items"`
	Self             *AgentLeaderboardEntry  `json:"self"`
	Total            int64                   `json:"total"`
	Page             int                     `json:"page"`
	PageSize         int                     `json:"page_size"`
	SortBy           string                  `json:"sort_by"`
}

type agentLeaderboardRow struct {
	AgentUserId                                                             int
	Username                                                                string
	NewUsers, TopupAmount, TopupUsers, SecondUsers, ThirdUsers, FourthUsers int64
}
type agentLeaderboardComputedEntry struct {
	AgentUserId int
	Entry       AgentLeaderboardEntry
}

func GetAgentLeaderboard(requesterUserId int, pageInfo *common.PageInfo, sortBy string, dates ...string) (*AgentLeaderboardResult, error) {
	if len(dates) >= 2 && strings.TrimSpace(dates[0]) == "" && strings.TrimSpace(dates[1]) == "" {
		return getLegacyAgentLeaderboardAt(requesterUserId, pageInfo, sortBy, time.Now().In(time.Local))
	}
	return getAgentLeaderboardAt(requesterUserId, pageInfo, sortBy, time.Now().In(time.Local), dates...)
}

func parseAgentLeaderboardRange(now time.Time, dates ...string) (time.Time, time.Time, string, string, error) {
	location := time.Local
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	start, end := today, today
	if len(dates) > 2 {
		return time.Time{}, time.Time{}, "", "", errors.New("at most start_date and end_date are allowed")
	}
	if len(dates) > 0 && strings.TrimSpace(dates[0]) != "" {
		var err error
		start, err = time.ParseInLocation("2006-01-02", dates[0], location)
		if err != nil || start.Format("2006-01-02") != dates[0] {
			return time.Time{}, time.Time{}, "", "", errors.New("start_date must be YYYY-MM-DD")
		}
	}
	if len(dates) > 1 && strings.TrimSpace(dates[1]) != "" {
		var err error
		end, err = time.ParseInLocation("2006-01-02", dates[1], location)
		if err != nil || end.Format("2006-01-02") != dates[1] {
			return time.Time{}, time.Time{}, "", "", errors.New("end_date must be YYYY-MM-DD")
		}
	} else {
		end = start
	}
	if start.After(end) {
		return time.Time{}, time.Time{}, "", "", errors.New("start_date must not be after end_date")
	}
	if end.After(today) {
		return time.Time{}, time.Time{}, "", "", errors.New("date range cannot include the future")
	}
	endExclusive := end.AddDate(0, 0, 1)
	nextMonth := time.Date(start.Year(), start.Month()+1, 1, 0, 0, 0, 0, location)
	daysInNextMonth := time.Date(nextMonth.Year(), nextMonth.Month()+1, 0, 0, 0, 0, 0, location).Day()
	boundaryDay := start.Day()
	if boundaryDay > daysInNextMonth {
		boundaryDay = daysInNextMonth
	}
	maxEndExclusive := time.Date(nextMonth.Year(), nextMonth.Month(), boundaryDay, 0, 0, 0, 0, location)
	if start.Day() > daysInNextMonth {
		maxEndExclusive = maxEndExclusive.AddDate(0, 0, 1)
	}
	if endExclusive.After(maxEndExclusive) {
		return time.Time{}, time.Time{}, "", "", errors.New("date range cannot exceed one calendar month")
	}
	return start, endExclusive, start.Format("2006-01-02"), end.Format("2006-01-02"), nil
}

func getAgentLeaderboardAt(requesterUserId int, pageInfo *common.PageInfo, sortBy string, now time.Time, dates ...string) (*AgentLeaderboardResult, error) {
	if len(dates) == 0 {
		return getLegacyAgentLeaderboardAt(requesterUserId, pageInfo, sortBy, now)
	}
	return getAgentLeaderboardRangeAt(requesterUserId, pageInfo, sortBy, now, dates...)
}

func getLegacyAgentLeaderboardAt(requesterUserId int, pageInfo *common.PageInfo, sortBy string, now time.Time) (*AgentLeaderboardResult, error) {
	if requesterUserId <= 0 || pageInfo == nil || DB == nil {
		return nil, errors.New("invalid leaderboard query")
	}
	page, pageSize := normalizeAgentLeaderboardPageInfo(pageInfo)
	sortBy = normalizeLegacyAgentLeaderboardSort(sortBy)
	var profile AgentProfile
	if err := DB.Where("user_id = ? AND status = ?", requesterUserId, AgentStatusEnabled).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("current user is not an enabled agent")
		}
		return nil, err
	}
	location := now.Location()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	end := now.Unix() + 1
	type legacyRow struct {
		AgentUserId                                                          int
		Username                                                             string
		DayNewUserCount, MonthNewUserCount, DayTopupAmount                   int64
		MonthTopupUsers, MonthSecondUsers, MonthThirdUsers, MonthFourthUsers int64
	}
	rows := make([]legacyRow, 0)
	err := DB.Table("agent_profiles AS ap").Select(`ap.user_id AS agent_user_id, u.username,
		COALESCE(du.new_users, 0) AS day_new_user_count, COALESCE(mu.new_users, 0) AS month_new_user_count,
		COALESCE(dt.topup_amount, 0) AS day_topup_amount, COALESCE(mt.topup_users, 0) AS month_topup_users,
		COALESCE(mt.second_users, 0) AS month_second_users, COALESCE(mt.third_users, 0) AS month_third_users,
		COALESCE(mt.fourth_users, 0) AS month_fourth_users`).
		Joins("JOIN users AS u ON u.id = ap.user_id AND u.deleted_at IS NULL").
		Joins(`LEFT JOIN (SELECT inviter_id, COUNT(*) AS new_users FROM users WHERE inviter_id > 0 AND deleted_at IS NULL AND created_at >= ? AND created_at < ? GROUP BY inviter_id) du ON du.inviter_id=ap.user_id`, dayStart.Unix(), end).
		Joins(`LEFT JOIN (SELECT inviter_id, COUNT(*) AS new_users FROM users WHERE inviter_id > 0 AND deleted_at IS NULL AND created_at >= ? AND created_at < ? GROUP BY inviter_id) mu ON mu.inviter_id=ap.user_id`, monthStart.Unix(), end).
		Joins(`LEFT JOIN (SELECT agent_user_id, SUM(pay_amount) AS topup_amount FROM agent_rebate_records WHERE status=? AND source_type=? AND settled_at >= ? AND settled_at < ? GROUP BY agent_user_id) dt ON dt.agent_user_id=ap.user_id`, AgentRebateRecordSettled, AgentRebateSourceEPay, dayStart.Unix(), end).
		Joins(`LEFT JOIN (SELECT agent_user_id, COUNT(*) AS topup_users, SUM(CASE WHEN topup_count>=2 THEN 1 ELSE 0 END) AS second_users, SUM(CASE WHEN topup_count>=3 THEN 1 ELSE 0 END) AS third_users, SUM(CASE WHEN topup_count>=4 THEN 1 ELSE 0 END) AS fourth_users FROM (SELECT agent_user_id, invitee_user_id, COUNT(*) AS topup_count FROM agent_rebate_records WHERE status=? AND source_type=? AND settled_at >= ? AND settled_at < ? GROUP BY agent_user_id, invitee_user_id) x GROUP BY agent_user_id) mt ON mt.agent_user_id=ap.user_id`, AgentRebateRecordSettled, AgentRebateSourceEPay, monthStart.Unix(), end).
		Where("ap.status = ?", AgentStatusEnabled).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	entries := make([]agentLeaderboardComputedEntry, 0, len(rows))
	for _, row := range rows {
		rate := analyticsRatio(float64(row.MonthSecondUsers), float64(row.MonthTopupUsers))
		entries = append(entries, agentLeaderboardComputedEntry{row.AgentUserId, AgentLeaderboardEntry{AgentLabel: maskAgentLeaderboardName(row.Username), IsSelf: row.AgentUserId == requesterUserId, DayNewUserCount: row.DayNewUserCount, MonthNewUserCount: row.MonthNewUserCount, DayTopupAmount: row.DayTopupAmount, MonthRepurchaseRate: rate, MonthSecondTopupRate: rate, MonthThirdTopupRate: analyticsRatio(float64(row.MonthThirdUsers), float64(row.MonthTopupUsers)), MonthFourthTopupRate: analyticsRatio(float64(row.MonthFourthUsers), float64(row.MonthTopupUsers))}})
	}
	legacySorts := []string{AgentLeaderboardSortDayNewUser, AgentLeaderboardSortMonthNewUser, AgentLeaderboardSortDayTopup, AgentLeaderboardSortMonthSecondTopupRate, AgentLeaderboardSortMonthThirdTopupRate, AgentLeaderboardSortMonthFourthTopupRate}
	for _, metric := range legacySorts {
		assignLegacyAgentLeaderboardRanks(entries, metric)
	}
	sortLegacyAgentLeaderboardEntries(entries, sortBy)
	result := &AgentLeaderboardResult{GeneratedAt: now.Unix(), DayStart: dayStart.Unix(), MonthStart: monthStart.Unix(), Items: make([]AgentLeaderboardEntry, 0, pageSize), Total: int64(len(entries)), Page: page, PageSize: pageSize, SortBy: sortBy}
	for i := range entries {
		entries[i].Entry.Rank = i + 1
		if i >= (page-1)*pageSize && i < page*pageSize {
			result.Items = append(result.Items, entries[i].Entry)
		}
		if entries[i].Entry.IsSelf {
			self := entries[i].Entry
			result.Self = &self
		}
	}
	return result, nil
}

func normalizeLegacyAgentLeaderboardSort(s string) string {
	s = strings.TrimSpace(s)
	switch s {
	case AgentLeaderboardSortMonthNewUser, AgentLeaderboardSortDayTopup, AgentLeaderboardSortMonthSecondTopupRate, AgentLeaderboardSortMonthThirdTopupRate, AgentLeaderboardSortMonthFourthTopupRate:
		return s
	default:
		return AgentLeaderboardSortDayNewUser
	}
}
func legacyAgentLeaderboardMetricValue(e AgentLeaderboardEntry, s string) float64 {
	switch s {
	case AgentLeaderboardSortMonthNewUser:
		return float64(e.MonthNewUserCount)
	case AgentLeaderboardSortDayTopup:
		return float64(e.DayTopupAmount)
	case AgentLeaderboardSortMonthSecondTopupRate:
		return e.MonthSecondTopupRate
	case AgentLeaderboardSortMonthThirdTopupRate:
		return e.MonthThirdTopupRate
	case AgentLeaderboardSortMonthFourthTopupRate:
		return e.MonthFourthTopupRate
	default:
		return float64(e.DayNewUserCount)
	}
}
func setLegacyAgentLeaderboardRank(e *AgentLeaderboardEntry, s string, rank int) {
	switch s {
	case AgentLeaderboardSortMonthNewUser:
		e.MonthNewUserRank = rank
	case AgentLeaderboardSortDayTopup:
		e.DayTopupRank = rank
	case AgentLeaderboardSortMonthSecondTopupRate:
		e.MonthSecondTopupRank = rank
	case AgentLeaderboardSortMonthThirdTopupRate:
		e.MonthThirdTopupRank = rank
	case AgentLeaderboardSortMonthFourthTopupRate:
		e.MonthFourthTopupRank = rank
	default:
		e.DayNewUserRank = rank
	}
}
func assignLegacyAgentLeaderboardRanks(entries []agentLeaderboardComputedEntry, s string) {
	idx := make([]int, len(entries))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool {
		a, b := legacyAgentLeaderboardMetricValue(entries[idx[i]].Entry, s), legacyAgentLeaderboardMetricValue(entries[idx[j]].Entry, s)
		if a != b {
			return a > b
		}
		return entries[idx[i]].AgentUserId < entries[idx[j]].AgentUserId
	})
	rank := 0
	last := 0.0
	for pos, i := range idx {
		value := legacyAgentLeaderboardMetricValue(entries[i].Entry, s)
		if pos == 0 || value != last {
			rank++
			last = value
		}
		setLegacyAgentLeaderboardRank(&entries[i].Entry, s, rank)
	}
}
func sortLegacyAgentLeaderboardEntries(entries []agentLeaderboardComputedEntry, s string) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := legacyAgentLeaderboardMetricValue(entries[i].Entry, s), legacyAgentLeaderboardMetricValue(entries[j].Entry, s)
		if a != b {
			return a > b
		}
		return entries[i].AgentUserId < entries[j].AgentUserId
	})
}

func getAgentLeaderboardRangeAt(requesterUserId int, pageInfo *common.PageInfo, sortBy string, now time.Time, dates ...string) (*AgentLeaderboardResult, error) {
	if requesterUserId <= 0 || pageInfo == nil || DB == nil {
		return nil, errors.New("invalid leaderboard query")
	}
	start, endExclusive, startDate, endDate, err := parseAgentLeaderboardRange(now, dates...)
	if err != nil {
		return nil, err
	}
	page, pageSize := normalizeAgentLeaderboardPageInfo(pageInfo)
	sortBy = normalizeAgentLeaderboardSort(sortBy)
	var requesterProfile AgentProfile
	if err := DB.Where("user_id = ? AND status = ?", requesterUserId, AgentStatusEnabled).First(&requesterProfile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("current user is not an enabled agent")
		}
		return nil, err
	}
	sources := []string{AgentRebateSourceEPay}
	rows := make([]agentLeaderboardRow, 0)
	err = DB.Table("agent_profiles AS ap").Select(`ap.user_id AS agent_user_id, u.username,
		COALESCE(nu.new_user_count, 0) AS new_users, COALESCE(tu.topup_amount, 0) AS topup_amount,
		COALESCE(r.topup_users, 0) AS topup_users, COALESCE(r.second_users, 0) AS second_users,
		COALESCE(r.third_users, 0) AS third_users, COALESCE(r.fourth_users, 0) AS fourth_users`).
		Joins("JOIN users AS u ON u.id = ap.user_id AND u.deleted_at IS NULL").
		Joins(`LEFT JOIN (SELECT inviter_id AS agent_user_id, COUNT(*) AS new_user_count FROM users WHERE inviter_id > 0 AND deleted_at IS NULL AND created_at >= ? AND created_at < ? GROUP BY inviter_id) nu ON nu.agent_user_id=ap.user_id`, start.Unix(), endExclusive.Unix()).
		Joins(`LEFT JOIN (SELECT agent_user_id, SUM(pay_amount) AS topup_amount FROM agent_rebate_records WHERE status=? AND source_type IN ? AND settled_at >= ? AND settled_at < ? GROUP BY agent_user_id) tu ON tu.agent_user_id=ap.user_id`, AgentRebateRecordSettled, sources, start.Unix(), endExclusive.Unix()).
		Joins(`LEFT JOIN (SELECT agent_user_id, COUNT(*) AS topup_users, SUM(CASE WHEN topup_count>=2 THEN 1 ELSE 0 END) AS second_users, SUM(CASE WHEN topup_count>=3 THEN 1 ELSE 0 END) AS third_users, SUM(CASE WHEN topup_count>=4 THEN 1 ELSE 0 END) AS fourth_users FROM (SELECT agent_user_id, invitee_user_id, COUNT(*) AS topup_count FROM agent_rebate_records WHERE status=? AND source_type IN ? AND settled_at >= ? AND settled_at < ? GROUP BY agent_user_id, invitee_user_id) x GROUP BY agent_user_id) r ON r.agent_user_id=ap.user_id`, AgentRebateRecordSettled, sources, start.Unix(), endExclusive.Unix()).
		Where("ap.status = ?", AgentStatusEnabled).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	computed := make([]agentLeaderboardComputedEntry, 0, len(rows))
	for _, row := range rows {
		denom := float64(row.TopupUsers)
		computed = append(computed, agentLeaderboardComputedEntry{row.AgentUserId, AgentLeaderboardEntry{RowKey: agentLeaderboardRowKey(row.AgentUserId), AgentLabel: maskAgentLeaderboardName(row.Username), IsSelf: row.AgentUserId == requesterUserId, RangeNewUserCount: row.NewUsers, RangeTopupAmount: row.TopupAmount, RangeSecondTopupRate: analyticsRatio(float64(row.SecondUsers), denom), RangeThirdTopupRate: analyticsRatio(float64(row.ThirdUsers), denom), RangeFourthTopupRate: analyticsRatio(float64(row.FourthUsers), denom)}})
	}
	result := &AgentLeaderboardResult{GeneratedAt: now.Unix(), StartDate: startDate, EndDate: endDate, EndExclusiveDate: endExclusive.Format("2006-01-02"), Items: make([]AgentLeaderboardEntry, 0, pageSize), Total: int64(len(computed)), Page: page, PageSize: pageSize, SortBy: sortBy}
	for _, metric := range agentLeaderboardSorts {
		assignAgentLeaderboardDenseRanks(computed, metric)
	}
	sortAgentLeaderboardEntries(computed, sortBy)
	for i := range computed {
		computed[i].Entry.Rank = i + 1
		if i >= (page-1)*pageSize && i < page*pageSize {
			result.Items = append(result.Items, computed[i].Entry)
		}
		if computed[i].Entry.IsSelf {
			self := computed[i].Entry
			result.Self = &self
		}
	}
	return result, nil
}

func normalizeAgentLeaderboardSort(s string) string {
	s = strings.TrimSpace(s)
	for _, v := range agentLeaderboardSorts {
		if s == v {
			return v
		}
	}
	switch s {
	case AgentLeaderboardSortDayNewUser:
		return AgentLeaderboardSortRangeNewUser
	case AgentLeaderboardSortMonthNewUser:
		return AgentLeaderboardSortRangeNewUser
	case AgentLeaderboardSortDayTopup:
		return AgentLeaderboardSortRangeTopup
	case AgentLeaderboardSortMonthSecondTopupRate:
		return AgentLeaderboardSortSecondTopupRate
	case AgentLeaderboardSortMonthThirdTopupRate:
		return AgentLeaderboardSortThirdTopupRate
	case AgentLeaderboardSortMonthFourthTopupRate:
		return AgentLeaderboardSortFourthTopupRate
	default:
		return AgentLeaderboardSortRangeNewUser
	}
}
func normalizeAgentLeaderboardPageInfo(p *common.PageInfo) (int, int) {
	page, size := p.Page, p.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = common.ItemsPerPage
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
func agentLeaderboardMetricValue(e AgentLeaderboardEntry, s string) float64 {
	switch s {
	case AgentLeaderboardSortRangeTopup:
		return float64(e.RangeTopupAmount)
	case AgentLeaderboardSortSecondTopupRate:
		return e.RangeSecondTopupRate
	case AgentLeaderboardSortThirdTopupRate:
		return e.RangeThirdTopupRate
	case AgentLeaderboardSortFourthTopupRate:
		return e.RangeFourthTopupRate
	default:
		return float64(e.RangeNewUserCount)
	}
}
func setAgentLeaderboardMetricRank(e *AgentLeaderboardEntry, s string, r int) {
	switch s {
	case AgentLeaderboardSortRangeTopup:
		e.RangeTopupAmountRank = r
	case AgentLeaderboardSortSecondTopupRate:
		e.RangeSecondTopupRank = r
	case AgentLeaderboardSortThirdTopupRate:
		e.RangeThirdTopupRank = r
	case AgentLeaderboardSortFourthTopupRate:
		e.RangeFourthTopupRank = r
	default:
		e.RangeNewUserRank = r
	}
}
func sortAgentLeaderboardEntries(es []agentLeaderboardComputedEntry, s string) {
	sort.SliceStable(es, func(i, j int) bool {
		a, b := agentLeaderboardMetricValue(es[i].Entry, s), agentLeaderboardMetricValue(es[j].Entry, s)
		if a != b {
			return a > b
		}
		return es[i].AgentUserId < es[j].AgentUserId
	})
}
func assignAgentLeaderboardDenseRanks(es []agentLeaderboardComputedEntry, s string) {
	idx := make([]int, len(es))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool {
		a, b := agentLeaderboardMetricValue(es[idx[i]].Entry, s), agentLeaderboardMetricValue(es[idx[j]].Entry, s)
		if a != b {
			return a > b
		}
		return es[idx[i]].AgentUserId < es[idx[j]].AgentUserId
	})
	rank := 0
	last := 0.0
	for pos, i := range idx {
		v := agentLeaderboardMetricValue(es[i].Entry, s)
		if pos == 0 || v != last {
			rank++
			last = v
		}
		setAgentLeaderboardMetricRank(&es[i].Entry, s, rank)
	}
}
func agentLeaderboardRowKey(userId int) string {
	digest := common.GenerateHMACWithKey([]byte("agent-leaderboard-row-v1:"+common.SessionSecret), strconv.Itoa(userId))
	return digest[:24]
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
