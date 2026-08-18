package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestParseAgentLeaderboardRangeStrictAndCalendarBounded(t *testing.T) {
	now := time.Date(2025, 3, 20, 15, 0, 0, 0, time.Local)
	start, end, startDate, endDate, err := parseAgentLeaderboardRange(now, "2025-02-15", "2025-03-14")
	require.NoError(t, err)
	require.Equal(t, "2025-02-15", startDate)
	require.Equal(t, "2025-03-14", endDate)
	require.Equal(t, time.Date(2025, 2, 15, 0, 0, 0, 0, time.Local), start)
	require.Equal(t, time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local), end)

	for _, dates := range [][2]string{{"2025-02-15", "2025-03-15"}, {"2025-02-30", "2025-03-01"}, {"2025/02/15", "2025-03-01"}} {
		_, _, _, _, err = parseAgentLeaderboardRange(now, dates[0], dates[1])
		require.Error(t, err, dates)
	}
}

func TestParseAgentLeaderboardRangeClampsMonthEnd(t *testing.T) {
	now := time.Date(2025, 4, 30, 12, 0, 0, 0, time.Local)
	for _, tc := range []struct {
		start, allowedEnd, rejectedEnd string
	}{
		{"2025-01-31", "2025-02-28", "2025-03-01"},
		{"2024-01-31", "2024-02-29", "2024-03-01"},
		{"2025-03-31", "2025-04-30", "2025-05-01"},
	} {
		_, _, _, _, err := parseAgentLeaderboardRange(now, tc.start, tc.allowedEnd)
		require.NoError(t, err, tc)
		_, _, _, _, err = parseAgentLeaderboardRange(now, tc.start, tc.rejectedEnd)
		require.Error(t, err, tc)
	}
}

func TestParseAgentLeaderboardRangeUsesLocalDayAndRejectsFuture(t *testing.T) {
	now := time.Date(2025, 3, 20, 0, 1, 0, 0, time.Local)
	_, _, startDate, endDate, err := parseAgentLeaderboardRange(now)
	require.NoError(t, err)
	require.Equal(t, "2025-03-20", startDate)
	require.Equal(t, "2025-03-20", endDate)
	_, _, _, _, err = parseAgentLeaderboardRange(now, "2025-03-21", "2025-03-21")
	require.Error(t, err)
}

func TestAgentLeaderboardRanksAreDenseAndTieOrderStable(t *testing.T) {
	entries := []agentLeaderboardComputedEntry{
		{AgentUserId: 20, Entry: AgentLeaderboardEntry{RangeNewUserCount: 3}},
		{AgentUserId: 10, Entry: AgentLeaderboardEntry{RangeNewUserCount: 3}},
		{AgentUserId: 30, Entry: AgentLeaderboardEntry{RangeNewUserCount: 1}},
	}
	assignAgentLeaderboardDenseRanks(entries, AgentLeaderboardSortNewUserCount)
	sortAgentLeaderboardEntries(entries, AgentLeaderboardSortNewUserCount)
	require.Equal(t, []int{10, 20, 30}, []int{entries[0].AgentUserId, entries[1].AgentUserId, entries[2].AgentUserId})
	require.Equal(t, []int{1, 1, 2}, []int{entries[0].Entry.RangeNewUserRank, entries[1].Entry.RangeNewUserRank, entries[2].Entry.RangeNewUserRank})
}

func TestGetAgentLeaderboardFiltersSourcesAndKeepsGlobalSelf(t *testing.T) {
	setupAgentBackendTest(t)
	now := time.Date(2025, 3, 20, 12, 0, 0, 0, time.Local)
	agentA := createAgentBackendTestUser(t, "agent-alpha")
	agentB := createAgentBackendTestUser(t, "agent-bravo")
	inviteeA := createAgentBackendTestUser(t, "invitee-alpha")
	inviteeB := createAgentBackendTestUser(t, "invitee-bravo")
	for _, agent := range []*User{agentA, agentB} {
		require.NoError(t, DB.Create(&AgentProfile{UserId: agent.Id, Status: AgentStatusEnabled}).Error)
	}
	inside := time.Date(2025, 3, 10, 12, 0, 0, 0, time.Local).Unix()
	require.NoError(t, DB.Model(inviteeA).Updates(map[string]any{"inviter_id": agentA.Id, "created_at": inside}).Error)
	require.NoError(t, DB.Model(inviteeB).Updates(map[string]any{"inviter_id": agentB.Id, "created_at": inside}).Error)
	records := []AgentRebateRecord{
		{TopUpId: 1, TradeNo: "epay-1", AgentUserId: agentA.Id, InviteeUserId: inviteeA.Id, SourceType: AgentRebateSourceEPay, PayAmount: 50, Status: AgentRebateRecordSettled, SettledAt: inside},
		{TopUpId: 2, TradeNo: "stripe-2", AgentUserId: agentA.Id, InviteeUserId: inviteeA.Id, SourceType: AgentRebateSourceStripe, PayAmount: 100, Status: AgentRebateRecordSettled, SettledAt: inside},
		{TopUpId: 3, TradeNo: "creem-3", AgentUserId: agentA.Id, InviteeUserId: inviteeA.Id, SourceType: AgentRebateSourceCreem, PayAmount: 200, Status: AgentRebateRecordSettled, SettledAt: inside},
		{TopUpId: 4, TradeNo: "waffo-4", AgentUserId: agentA.Id, InviteeUserId: inviteeA.Id, SourceType: AgentRebateSourceWaffo, PayAmount: 300, Status: AgentRebateRecordSettled, SettledAt: inside},
		{TopUpId: 5, TradeNo: "waffo-pancake-5", AgentUserId: agentA.Id, InviteeUserId: inviteeA.Id, SourceType: AgentRebateSourceWaffoPancake, PayAmount: 400, Status: AgentRebateRecordSettled, SettledAt: inside},
		{TopUpId: 6, TradeNo: "manual", AgentUserId: agentA.Id, InviteeUserId: inviteeA.Id, SourceType: AgentRebateSourceManual, PayAmount: 9000, Status: AgentRebateRecordSettled, SettledAt: inside},
		{TopUpId: 7, TradeNo: "redemption", AgentUserId: agentA.Id, InviteeUserId: inviteeA.Id, SourceType: AgentRebateSourceRedemption, PayAmount: 9000, Status: AgentRebateRecordSettled, SettledAt: inside},
		{TopUpId: 8, TradeNo: "canceled", AgentUserId: agentA.Id, InviteeUserId: inviteeA.Id, SourceType: AgentRebateSourceEPay, PayAmount: 8000, Status: AgentRebateRecordCanceled, SettledAt: inside},
		{TopUpId: 9, TradeNo: "outside", AgentUserId: agentA.Id, InviteeUserId: inviteeA.Id, SourceType: AgentRebateSourceEPay, PayAmount: 7000, Status: AgentRebateRecordSettled, SettledAt: time.Date(2025, 3, 21, 0, 0, 0, 0, time.Local).Unix()},
		{TopUpId: 10, TradeNo: "agent-b", AgentUserId: agentB.Id, InviteeUserId: inviteeB.Id, SourceType: AgentRebateSourceWaffo, PayAmount: 50, Status: AgentRebateRecordSettled, SettledAt: inside},
	}

	require.NoError(t, DB.Create(&records).Error)

	result, err := getAgentLeaderboardAt(agentB.Id, &common.PageInfo{Page: 1, PageSize: 1}, AgentLeaderboardSortTopupAmount, now, "2025-03-01", "2025-03-20")
	require.NoError(t, err)
	require.Equal(t, int64(2), result.Total)
	require.Len(t, result.Items, 1)
	require.Equal(t, int64(1050), result.Items[0].RangeTopupAmount)
	require.Len(t, result.Items[0].RowKey, 24)
	require.Equal(t, agentLeaderboardRowKey(agentA.Id), result.Items[0].RowKey)
	require.Equal(t, float64(1), result.Items[0].RangeSecondTopupRate)
	require.NotNil(t, result.Self)
	require.True(t, result.Self.IsSelf)
	require.Equal(t, int64(50), result.Self.RangeTopupAmount)
	require.Equal(t, 2, result.Self.Rank)
}

func TestAgentLeaderboardEntryDoesNotExposeID(t *testing.T) {
	entry := AgentLeaderboardEntry{AgentLabel: maskAgentLeaderboardName("alice@example.com"), IsSelf: true}
	require.NotContains(t, entry.AgentLabel, "alice@example.com")
	payload, err := common.Marshal(entry)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "user_id")
}
