package model

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupRiskControlTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&RiskIPRecord{}))
	riskIPWriteState = sync.Map{}
	t.Cleanup(func() {
		DB.Exec("DELETE FROM risk_ip_records")
		DB.Exec("DELETE FROM users")
		riskIPWriteState = sync.Map{}
	})
}

func TestNormalizeRiskIP(t *testing.T) {
	require.Equal(t, "192.0.2.1", NormalizeRiskIP(" 192.0.2.1 "))
	require.Equal(t, "2001:db8::1", NormalizeRiskIP("2001:0db8:0:0:0:0:0:1"))
	require.Empty(t, NormalizeRiskIP("not-an-ip"))
}

func TestRecordRiskIPAggregatesSameIdentity(t *testing.T) {
	setupRiskControlTest(t)
	user := User{Username: "risk-aggregate", Password: "test-password", AffCode: "risk-aggregate-aff"}
	require.NoError(t, DB.Create(&user).Error)

	RecordRiskIP(user.Id, 7, RiskIPSourceToken, "192.0.2.10")
	riskIPWriteState = sync.Map{}
	RecordRiskIP(user.Id, 7, RiskIPSourceToken, "192.0.2.10")

	var records []RiskIPRecord
	require.NoError(t, DB.Find(&records).Error)
	require.Len(t, records, 1)
	require.EqualValues(t, 2, records[0].EventCount)
}

func TestListRiskSharedIPsReturnsAssociatedUsers(t *testing.T) {
	setupRiskControlTest(t)
	users := []User{
		{Username: "risk-shared-a", Password: "test-password", AffCode: "risk-shared-a-aff"},
		{Username: "risk-shared-b", Password: "test-password", AffCode: "risk-shared-b-aff"},
	}
	for index := range users {
		require.NoError(t, DB.Create(&users[index]).Error)
		require.NoError(t, DB.Create(&RiskIPRecord{UserId: users[index].Id, TokenId: index + 1, IP: "198.51.100.8", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 200 + int64(index), EventCount: 3}).Error)
	}

	items, total, err := ListRiskSharedIPs(RiskIPSourceToken, "ip", "198.51.100", 2, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.EqualValues(t, 2, items[0].UserCount)
	require.EqualValues(t, 6, items[0].EventCount)
	require.Len(t, items[0].Users, 2)

	RecordRiskIP(users[0].Id, 0, RiskIPSourceLogin, "198.51.100.8")
	items, total, err = ListRiskSharedIPs("", "ip", "", 2, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "all", items[0].Source)
	require.EqualValues(t, 2, items[0].UserCount)
	require.Len(t, items[0].Users, 2)

	items, total, err = ListRiskSharedIPs(RiskIPSourceToken, "username", "risk-shared-a", 2, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.EqualValues(t, 2, items[0].UserCount)

	items, total, err = ListRiskSharedIPs(RiskIPSourceToken, "user_id", strconv.Itoa(users[1].Id), 2, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
}

func TestListRiskInvitersUsesActualInviteCount(t *testing.T) {
	setupRiskControlTest(t)
	inviter := User{Username: "risk-inviter", Password: "test-password", AffCode: "risk-inviter-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitees := make([]User, 101)
	for index := range invitees {
		invitees[index] = User{Username: fmt.Sprintf("risk-invitee-%03d", index), Password: "test-password", AffCode: fmt.Sprintf("risk-invitee-aff-%03d", index), InviterId: inviter.Id}
	}
	require.NoError(t, DB.CreateInBatches(&invitees, 50).Error)

	items, total, err := ListRiskInviters("risk-inviter", "all", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.EqualValues(t, 101, items[0].DirectInviteCount)
	require.Len(t, items[0].Invitees, 100)
}

func TestListRiskInvitersSortedByLatestInviteeCreatedAt(t *testing.T) {
	setupRiskControlTest(t)
	inviters := []User{
		{Username: "risk-sort-inviter-a", Password: "test-password", AffCode: "risk-sort-inviter-a-aff"},
		{Username: "risk-sort-inviter-b", Password: "test-password", AffCode: "risk-sort-inviter-b-aff"},
		{Username: "risk-sort-inviter-c", Password: "test-password", AffCode: "risk-sort-inviter-c-aff"},
	}
	for index := range inviters {
		require.NoError(t, DB.Create(&inviters[index]).Error)
	}
	invitees := []User{
		{Username: "risk-sort-a-1", Password: "test-password", AffCode: "risk-sort-a-1-aff", InviterId: inviters[0].Id, CreatedAt: 3000},
		{Username: "risk-sort-b-1", Password: "test-password", AffCode: "risk-sort-b-1-aff", InviterId: inviters[1].Id, CreatedAt: 1000},
		{Username: "risk-sort-b-2", Password: "test-password", AffCode: "risk-sort-b-2-aff", InviterId: inviters[1].Id, CreatedAt: 5000},
		{Username: "risk-sort-c-1", Password: "test-password", AffCode: "risk-sort-c-1-aff", InviterId: inviters[2].Id, CreatedAt: 2000},
	}
	for index := range invitees {
		require.NoError(t, DB.Create(&invitees[index]).Error)
	}

	items, total, err := ListRiskInviters("", "all", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, items, 3)
	// inviter b has the latest invitee (5000), then a (3000), then c (2000)
	require.Equal(t, inviters[1].Id, items[0].InviterId)
	require.EqualValues(t, 5000, items[0].LatestInviteAt)
	require.Equal(t, inviters[0].Id, items[1].InviterId)
	require.EqualValues(t, 3000, items[1].LatestInviteAt)
	require.Equal(t, inviters[2].Id, items[2].InviterId)
	require.EqualValues(t, 2000, items[2].LatestInviteAt)
}

func TestListRiskInvitersStableSecondarySort(t *testing.T) {
	setupRiskControlTest(t)
	// every inviter has a single invitee with the same CreatedAt, so the order
	// must fall back to the stable secondary sort: inviter id descending
	ids := make([]int, 0, 3)
	for index := 0; index < 3; index++ {
		inviter := User{Username: fmt.Sprintf("risk-stable-inviter-%d", index), Password: "test-password", AffCode: fmt.Sprintf("risk-stable-inviter-%d-aff", index)}
		require.NoError(t, DB.Create(&inviter).Error)
		ids = append(ids, inviter.Id)
		invitee := User{Username: fmt.Sprintf("risk-stable-invitee-%d", index), Password: "test-password", AffCode: fmt.Sprintf("risk-stable-invitee-%d-aff", index), InviterId: inviter.Id, CreatedAt: 9000}
		require.NoError(t, DB.Create(&invitee).Error)
	}

	items, total, err := ListRiskInviters("", "all", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Equal(t, ids[2], items[0].InviterId)
	require.Equal(t, ids[1], items[1].InviterId)
	require.Equal(t, ids[0], items[2].InviterId)
	// pagination must not duplicate or skip inviters thanks to the tie-break
	page1, _, err := ListRiskInviters("", "all", 0, 2)
	require.NoError(t, err)
	page2, _, err := ListRiskInviters("", "all", 2, 2)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	require.Len(t, page2, 1)
	require.Equal(t, ids[2], page1[0].InviterId)
	require.Equal(t, ids[1], page1[1].InviterId)
	require.Equal(t, ids[0], page2[0].InviterId)
}

func TestListRiskInvitersRiskStatusFilter(t *testing.T) {
	setupRiskControlTest(t)
	suspiciousInviter := User{Username: "risk-status-suspicious", Password: "test-password", AffCode: "risk-status-suspicious-aff"}
	normalInviter := User{Username: "risk-status-normal", Password: "test-password", AffCode: "risk-status-normal-aff"}
	require.NoError(t, DB.Create(&suspiciousInviter).Error)
	require.NoError(t, DB.Create(&normalInviter).Error)
	users := []User{
		{Username: "risk-status-sus-1", Password: "test-password", AffCode: "risk-status-sus-1-aff", InviterId: suspiciousInviter.Id},
		{Username: "risk-status-sus-2", Password: "test-password", AffCode: "risk-status-sus-2-aff", InviterId: suspiciousInviter.Id},
		{Username: "risk-status-norm-1", Password: "test-password", AffCode: "risk-status-norm-1-aff", InviterId: normalInviter.Id},
	}
	for index := range users {
		require.NoError(t, DB.Create(&users[index]).Error)
	}
	// sus-1 and sus-2 share an IP; the normal inviter's group shares nothing
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: users[0].Id, TokenId: 1, IP: "203.0.113.21", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 100, EventCount: 1}).Error)
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: users[1].Id, TokenId: 2, IP: "203.0.113.21", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 100, EventCount: 1}).Error)

	items, total, err := ListRiskInviters("", "review", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, suspiciousInviter.Id, items[0].InviterId)
	require.True(t, items[0].Suspicious)
	require.EqualValues(t, 1, items[0].SharedIPCount)

	items, total, err = ListRiskInviters("", "normal", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, normalInviter.Id, items[0].InviterId)
	require.False(t, items[0].Suspicious)
	require.EqualValues(t, 0, items[0].SharedIPCount)

	items, total, err = ListRiskInviters("", "all", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, items, 2)

	// unknown values degrade safely to "all"
	items, total, err = ListRiskInviters("", "bogus-status", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, items, 2)
}

func TestListRiskInvitersRiskOverAllInviteesBeyondDisplayLimit(t *testing.T) {
	setupRiskControlTest(t)
	inviter := User{Username: "risk-full-inviter", Password: "test-password", AffCode: "risk-full-inviter-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitees := make([]User, 101)
	for index := range invitees {
		invitees[index] = User{Username: fmt.Sprintf("risk-full-invitee-%03d", index), Password: "test-password", AffCode: fmt.Sprintf("risk-full-invitee-aff-%03d", index), InviterId: inviter.Id}
	}
	require.NoError(t, DB.CreateInBatches(&invitees, 50).Error)
	// invitees[0] has the lowest id and is excluded from the displayed 100
	// (display order is id desc); it still shares an IP with invitees[1], so the
	// risk metrics must be computed over ALL 101 invitees, not the displayed ones
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: invitees[0].Id, TokenId: 1, IP: "203.0.113.42", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 100, EventCount: 1}).Error)
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: invitees[1].Id, TokenId: 2, IP: "203.0.113.42", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 100, EventCount: 1}).Error)

	items, total, err := ListRiskInviters("risk-full-inviter", "all", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.EqualValues(t, 101, items[0].DirectInviteCount)
	require.Len(t, items[0].Invitees, 100)
	require.True(t, items[0].Suspicious)
	require.EqualValues(t, 1, items[0].SharedIPCount)
	for _, summary := range items[0].Invitees {
		require.NotEqual(t, invitees[0].Id, summary.UserId, "invitee with the lowest id must not appear in the displayed 100")
	}

	// the inviter must be flagged by the review filter even though the sharing
	// invitee is not part of the displayed invitees
	items, total, err = ListRiskInviters("risk-full-inviter", "review", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.True(t, items[0].Suspicious)
	require.EqualValues(t, 1, items[0].SharedIPCount)
}

func TestListRiskInvitersAllPathPaginatesWithRisk(t *testing.T) {
	setupRiskControlTest(t)
	// only the inviter with the oldest latest invite time (sorted last) has a
	// shared-IP pair; the "all" path (DB pagination) must still report its risk
	// on the page that contains it
	inviters := []User{
		{Username: "risk-page-a", Password: "test-password", AffCode: "risk-page-a-aff"},
		{Username: "risk-page-b", Password: "test-password", AffCode: "risk-page-b-aff"},
		{Username: "risk-page-c", Password: "test-password", AffCode: "risk-page-c-aff"},
	}
	for index := range inviters {
		require.NoError(t, DB.Create(&inviters[index]).Error)
	}
	invitees := []User{
		{Username: "risk-page-a-1", Password: "test-password", AffCode: "risk-page-a-1-aff", InviterId: inviters[0].Id, CreatedAt: 1000},
		{Username: "risk-page-a-2", Password: "test-password", AffCode: "risk-page-a-2-aff", InviterId: inviters[0].Id, CreatedAt: 1000},
		{Username: "risk-page-b-1", Password: "test-password", AffCode: "risk-page-b-1-aff", InviterId: inviters[1].Id, CreatedAt: 2000},
		{Username: "risk-page-c-1", Password: "test-password", AffCode: "risk-page-c-1-aff", InviterId: inviters[2].Id, CreatedAt: 3000},
	}
	for index := range invitees {
		require.NoError(t, DB.Create(&invitees[index]).Error)
	}
	// a-1 and a-2 share an IP
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: invitees[0].Id, TokenId: 1, IP: "203.0.113.55", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 100, EventCount: 1}).Error)
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: invitees[1].Id, TokenId: 2, IP: "203.0.113.55", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 100, EventCount: 1}).Error)

	// latest invite times: a=1000, b=2000, c=3000 → order c, b, a
	page1, total, err := ListRiskInviters("", "all", 0, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, page1, 2)
	require.Equal(t, inviters[2].Id, page1[0].InviterId)
	require.Equal(t, inviters[1].Id, page1[1].InviterId)
	require.False(t, page1[0].Suspicious)
	require.False(t, page1[1].Suspicious)

	page2, total, err := ListRiskInviters("", "all", 2, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, page2, 1)
	require.Equal(t, inviters[0].Id, page2[0].InviterId)
	require.True(t, page2[0].Suspicious)
	require.EqualValues(t, 1, page2[0].SharedIPCount)
}

func TestListRiskInvitersSharedIPBetweenInviterAndInvitee(t *testing.T) {
	setupRiskControlTest(t)
	inviter := User{Username: "risk-self-share", Password: "test-password", AffCode: "risk-self-share-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{Username: "risk-self-share-1", Password: "test-password", AffCode: "risk-self-share-1-aff", InviterId: inviter.Id}
	require.NoError(t, DB.Create(&invitee).Error)
	// the inviter and their single invitee share one IP; another IP used by
	// the inviter alone must not count
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: inviter.Id, TokenId: 1, IP: "203.0.113.66", Source: RiskIPSourceLogin, FirstSeenAt: 100, LastSeenAt: 100, EventCount: 1}).Error)
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: invitee.Id, TokenId: 2, IP: "203.0.113.66", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 100, EventCount: 1}).Error)
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: inviter.Id, TokenId: 3, IP: "203.0.113.67", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 100, EventCount: 1}).Error)

	items, total, err := ListRiskInviters("risk-self-share", "all", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.EqualValues(t, 1, items[0].SharedIPCount)
	require.True(t, items[0].Suspicious)

	items, total, err = ListRiskInviters("", "review", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviter.Id, items[0].InviterId)
	require.EqualValues(t, 1, items[0].SharedIPCount)
}

func TestListRiskInvitersFilterOutOfRangePage(t *testing.T) {
	setupRiskControlTest(t)
	inviter := User{Username: "risk-oob", Password: "test-password", AffCode: "risk-oob-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitees := []User{
		{Username: "risk-oob-1", Password: "test-password", AffCode: "risk-oob-1-aff", InviterId: inviter.Id},
		{Username: "risk-oob-2", Password: "test-password", AffCode: "risk-oob-2-aff", InviterId: inviter.Id},
	}
	for index := range invitees {
		require.NoError(t, DB.Create(&invitees[index]).Error)
	}
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: invitees[0].Id, TokenId: 1, IP: "203.0.113.77", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 100, EventCount: 1}).Error)
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: invitees[1].Id, TokenId: 2, IP: "203.0.113.77", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 100, EventCount: 1}).Error)

	// review: one suspicious inviter; out-of-range pages stay empty with the
	// correct total
	items, total, err := ListRiskInviters("", "review", 10, 5)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Empty(t, items)

	// normal: zero matching inviters on any page, total 0
	items, total, err = ListRiskInviters("", "normal", 0, 5)
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.Empty(t, items)
	items, total, err = ListRiskInviters("", "normal", 10, 5)
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.Empty(t, items)

	// all path: out-of-range offset keeps the real total
	items, total, err = ListRiskInviters("", "all", 10, 5)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Empty(t, items)
}
