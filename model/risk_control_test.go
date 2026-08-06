package model

import (
	"fmt"
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

	items, total, err := ListRiskSharedIPs(RiskIPSourceToken, "198.51.100", 2, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.EqualValues(t, 2, items[0].UserCount)
	require.EqualValues(t, 6, items[0].EventCount)
	require.Len(t, items[0].Users, 2)

	RecordRiskIP(users[0].Id, 0, RiskIPSourceLogin, "198.51.100.8")
	items, total, err = ListRiskSharedIPs("", "", 2, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "all", items[0].Source)
	require.EqualValues(t, 2, items[0].UserCount)
	require.Len(t, items[0].Users, 2)
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

	items, total, err := ListRiskInviters("risk-inviter", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.EqualValues(t, 101, items[0].DirectInviteCount)
	require.Len(t, items[0].Invitees, 100)
}
