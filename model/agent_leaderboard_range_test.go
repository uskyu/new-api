package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestParseAgentLeaderboardRangeUsesLocalDayAndStrictDates(t *testing.T) {
	now := time.Date(2026, time.August, 19, 15, 0, 0, 0, time.Local)
	start, end, startDate, endDate, err := parseAgentLeaderboardRange(now, "2026-07-31", "2026-08-30")
	require.Error(t, err)
	require.Zero(t, start)
	require.Zero(t, end)
	require.Empty(t, startDate)
	require.Empty(t, endDate)

	start, end, startDate, endDate, err = parseAgentLeaderboardRange(now, "2026-07-31", "2026-08-30")
	require.Error(t, err)
	require.Zero(t, start)
	require.Zero(t, end)
	require.Empty(t, startDate)
	require.Empty(t, endDate)

	_, _, _, _, err = parseAgentLeaderboardRange(now, "2026/08/01", "2026-08-02")
	require.Error(t, err)
	_, _, _, _, err = parseAgentLeaderboardRange(now, "2026-08-02", "2026-08-01")
	require.Error(t, err)
	_, _, _, _, err = parseAgentLeaderboardRange(now, "2026-08-20", "2026-08-20")
	require.Error(t, err)
}

func TestParseAgentLeaderboardRangeAllowsOneCalendarMonthAndClampsMonthEnd(t *testing.T) {
	now := time.Date(2026, time.August, 19, 15, 0, 0, 0, time.Local)
	start, end, startDate, endDate, err := parseAgentLeaderboardRange(now, "2026-07-31", "2026-08-30")
	require.Error(t, err)
	require.Zero(t, start)
	require.Zero(t, end)
	require.Empty(t, startDate)
	require.Empty(t, endDate)

	start, end, startDate, endDate, err = parseAgentLeaderboardRange(now, "2026-07-20", "2026-08-19")
	require.NoError(t, err)
	require.Equal(t, "2026-07-20", startDate)
	require.Equal(t, "2026-08-19", endDate)
	require.Equal(t, time.Date(2026, time.July, 20, 0, 0, 0, 0, time.Local), start)
	require.Equal(t, time.Date(2026, time.August, 20, 0, 0, 0, 0, time.Local), end)
}

func TestParseAgentLeaderboardRangeDefaultsToToday(t *testing.T) {
	now := time.Date(2026, time.August, 19, 15, 0, 0, 0, time.Local)
	start, end, startDate, endDate, err := parseAgentLeaderboardRange(now, "", "")
	require.NoError(t, err)
	require.Equal(t, "2026-08-19", startDate)
	require.Equal(t, "2026-08-19", endDate)
	require.Equal(t, time.Date(2026, time.August, 19, 0, 0, 0, 0, time.Local), start)
	require.Equal(t, time.Date(2026, time.August, 20, 0, 0, 0, 0, time.Local), end)
}

func TestGetAgentLeaderboardRangeFiltersRecordsAndComputesMetrics(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)
	now := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.Local)
	agent := createAgentTestUser(t, "range_agent", "RANGE_AGENT")
	otherAgent := createAgentTestUser(t, "range_other_agent", "RANGE_OTHER")
	require.NoError(t, DB.Create(&AgentProfile{UserId: agent.Id, Status: AgentStatusEnabled}).Error)
	require.NoError(t, DB.Create(&AgentProfile{UserId: otherAgent.Id, Status: AgentStatusEnabled}).Error)
	invitee := createAgentTestUser(t, "range_invitee", "RANGE_INVITEE")
	inside := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.Local).Unix()
	outside := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.Local).Unix()
	require.NoError(t, DB.Model(invitee).Updates(map[string]interface{}{
		"inviter_id": agent.Id,
		"created_at": inside,
	}).Error)
	require.NoError(t, DB.Create([]AgentRebateRecord{
		{TopUpId: 1001, TradeNo: "range-epay-1", SourceType: AgentRebateSourceEPay, InviteeUserId: invitee.Id, AgentUserId: agent.Id, PayAmount: 100, Status: AgentRebateRecordSettled, SettledAt: inside},
		{TopUpId: 1002, TradeNo: "range-epay-2", SourceType: AgentRebateSourceEPay, InviteeUserId: invitee.Id, AgentUserId: agent.Id, PayAmount: 200, Status: AgentRebateRecordSettled, SettledAt: inside + 60},
		{TopUpId: 1003, TradeNo: "range-manual", SourceType: AgentRebateSourceManual, InviteeUserId: invitee.Id, AgentUserId: agent.Id, PayAmount: 9000, Status: AgentRebateRecordSettled, SettledAt: inside},
		{TopUpId: 1004, TradeNo: "range-outside", SourceType: AgentRebateSourceEPay, InviteeUserId: invitee.Id, AgentUserId: agent.Id, PayAmount: 7000, Status: AgentRebateRecordSettled, SettledAt: outside},
	}).Error)

	result, err := getAgentLeaderboardAt(agent.Id, &common.PageInfo{Page: 1, PageSize: 10}, AgentLeaderboardSortRangeTopup, now, "2026-08-01", "2026-08-03")
	require.NoError(t, err)
	require.Equal(t, "2026-08-01", result.StartDate)
	require.Equal(t, "2026-08-03", result.EndDate)
	require.Equal(t, int64(2), result.Total)
	require.NotNil(t, result.Self)
	require.Equal(t, int64(300), result.Self.RangeTopupAmount)
	require.InDelta(t, 1.0, result.Self.RangeSecondTopupRate, 0.0001)
	require.Zero(t, result.Self.RangeThirdTopupRate)
}
