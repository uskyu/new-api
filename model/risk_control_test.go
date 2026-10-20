package model

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
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

// createRiskUsers 批量创建测试用户，补齐必填字段（密码 / 角色），返回带 id 的副本。
func createRiskUsers(t *testing.T, users []User) []User {
	t.Helper()
	for index := range users {
		if users[index].Password == "" {
			users[index].Password = "test-password"
		}
		if users[index].Role == 0 {
			users[index].Role = common.RoleCommonUser
		}
		require.NoError(t, DB.Create(&users[index]).Error)
	}
	return users
}

func riskSharedIPIPs(rows []*RiskSharedIP) []string {
	ips := make([]string, 0, len(rows))
	for _, row := range rows {
		ips = append(ips, row.IP)
	}
	return ips
}

func riskStatusSkipReason(t *testing.T, skipped []RiskStatusSkip, userId int) string {
	t.Helper()
	for _, item := range skipped {
		if item.UserId == userId {
			return item.Reason
		}
	}
	t.Fatalf("expected user %d in skipped list, got %+v", userId, skipped)
	return ""
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
		{Username: "risk-shared-a", Password: "test-password", AffCode: "risk-shared-a-aff", Email: "risk-shared-a@example.com"},
		{Username: "risk-shared-b", Password: "test-password", AffCode: "risk-shared-b-aff"},
	}
	for index := range users {
		require.NoError(t, DB.Create(&users[index]).Error)
		require.NoError(t, DB.Create(&RiskIPRecord{UserId: users[index].Id, TokenId: index + 1, IP: "198.51.100.8", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 200 + int64(index), EventCount: 3}).Error)
	}

	items, total, err := ListRiskSharedIPs(RiskIPSourceToken, "ip", "198.51.100", 2, 0, 10, "last_seen_at", "desc", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.EqualValues(t, 2, items[0].UserCount)
	require.EqualValues(t, 6, items[0].EventCount)
	require.Len(t, items[0].Users, 2)
	byUser := make(map[int]*RiskUserSummary, len(items[0].Users))
	for _, summary := range items[0].Users {
		byUser[summary.UserId] = summary
	}
	require.Equal(t, "risk-shared-a@example.com", byUser[users[0].Id].Email)
	require.Equal(t, common.UserStatusEnabled, byUser[users[0].Id].Status)
	require.Empty(t, byUser[users[1].Id].Email)

	RecordRiskIP(users[0].Id, 0, RiskIPSourceLogin, "198.51.100.8")
	items, total, err = ListRiskSharedIPs("", "ip", "", 2, 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "all", items[0].Source)
	require.EqualValues(t, 2, items[0].UserCount)
	require.Len(t, items[0].Users, 2)

	items, total, err = ListRiskSharedIPs(RiskIPSourceToken, "username", "risk-shared-a", 2, 0, 10, "user_count", "asc", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.EqualValues(t, 2, items[0].UserCount)

	items, total, err = ListRiskSharedIPs(RiskIPSourceToken, "user_id", strconv.Itoa(users[1].Id), 2, 0, 10, "ip", "asc", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
}

func TestListRiskSharedIPsSortingAndStablePaging(t *testing.T) {
	setupRiskControlTest(t)
	users := createRiskUsers(t, []User{
		{Username: "risk-sort-a", Email: "risk-sort-a@example.com", AffCode: "risk-sort-a-aff", Quota: 5000},
		{Username: "risk-sort-b", AffCode: "risk-sort-b-aff", Status: common.UserStatusDisabled},
		{Username: "risk-sort-c", AffCode: "risk-sort-c-aff"},
		{Username: "risk-sort-d", AffCode: "risk-sort-d-aff"},
	})
	records := []RiskIPRecord{
		// 10.0.0.0: 2 users / 5 events / last_seen 50
		{UserId: users[0].Id, TokenId: 1, IP: "10.0.0.0", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 50, EventCount: 2},
		{UserId: users[1].Id, TokenId: 1, IP: "10.0.0.0", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 40, EventCount: 3},
		// 10.0.0.1: 2 users / 30 events / last_seen 100
		{UserId: users[0].Id, TokenId: 2, IP: "10.0.0.1", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 100, EventCount: 15},
		{UserId: users[1].Id, TokenId: 2, IP: "10.0.0.1", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 90, EventCount: 15},
		// 10.0.0.2: 3 users / 20 events / last_seen 200
		{UserId: users[0].Id, TokenId: 3, IP: "10.0.0.2", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 200, EventCount: 7},
		{UserId: users[1].Id, TokenId: 3, IP: "10.0.0.2", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 150, EventCount: 7},
		{UserId: users[2].Id, TokenId: 3, IP: "10.0.0.2", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 150, EventCount: 6},
		// 10.0.0.3: 4 users / 10 events / last_seen 300
		{UserId: users[0].Id, TokenId: 4, IP: "10.0.0.3", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 300, EventCount: 2},
		{UserId: users[1].Id, TokenId: 4, IP: "10.0.0.3", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 250, EventCount: 2},
		{UserId: users[2].Id, TokenId: 4, IP: "10.0.0.3", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 250, EventCount: 3},
		{UserId: users[3].Id, TokenId: 4, IP: "10.0.0.3", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 250, EventCount: 3},
	}
	require.NoError(t, DB.Create(&records).Error)

	list := func(sortBy string, order string, startIdx int, limit int) ([]*RiskSharedIP, int64) {
		items, total, err := ListRiskSharedIPs(RiskIPSourceToken, "ip", "", 2, startIdx, limit, sortBy, order, "")
		require.NoError(t, err)
		return items, total
	}

	// 默认排序：last_seen_at desc。
	items, total := list("", "", 0, 10)
	require.EqualValues(t, 4, total)
	require.Equal(t, []string{"10.0.0.3", "10.0.0.2", "10.0.0.1", "10.0.0.0"}, riskSharedIPIPs(items))

	// user_count desc，数量相同（.0/.1 均为 2）时按稳定次序 ip ASC。
	items, _ = list("user_count", "desc", 0, 10)
	require.Equal(t, []string{"10.0.0.3", "10.0.0.2", "10.0.0.0", "10.0.0.1"}, riskSharedIPIPs(items))
	require.EqualValues(t, 4, items[0].UserCount)
	require.EqualValues(t, 3, items[1].UserCount)

	items, _ = list("user_count", "asc", 0, 10)
	require.Equal(t, []string{"10.0.0.0", "10.0.0.1", "10.0.0.2", "10.0.0.3"}, riskSharedIPIPs(items))

	items, _ = list("event_count", "desc", 0, 10)
	require.Equal(t, []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.0"}, riskSharedIPIPs(items))
	require.EqualValues(t, 30, items[0].EventCount)

	items, _ = list("event_count", "asc", 0, 10)
	require.Equal(t, []string{"10.0.0.0", "10.0.0.3", "10.0.0.2", "10.0.0.1"}, riskSharedIPIPs(items))

	items, _ = list("last_seen_at", "asc", 0, 10)
	require.Equal(t, []string{"10.0.0.0", "10.0.0.1", "10.0.0.2", "10.0.0.3"}, riskSharedIPIPs(items))

	items, _ = list("ip", "asc", 0, 10)
	require.Equal(t, []string{"10.0.0.0", "10.0.0.1", "10.0.0.2", "10.0.0.3"}, riskSharedIPIPs(items))
	items, _ = list("ip", "desc", 0, 10)
	require.Equal(t, []string{"10.0.0.3", "10.0.0.2", "10.0.0.1", "10.0.0.0"}, riskSharedIPIPs(items))

	// 非法参数回退默认值：last_seen_at desc。
	items, _ = list("bogus", "sideways", 0, 10)
	require.Equal(t, []string{"10.0.0.3", "10.0.0.2", "10.0.0.1", "10.0.0.0"}, riskSharedIPIPs(items))

	// 稳定翻页：同等 user_count 的组按 ip ASC，两页拼接后无重复/丢失。
	firstPage, _ := list("user_count", "desc", 0, 2)
	secondPage, _ := list("user_count", "desc", 2, 2)
	require.Equal(t, []string{"10.0.0.3", "10.0.0.2"}, riskSharedIPIPs(firstPage))
	require.Equal(t, []string{"10.0.0.0", "10.0.0.1"}, riskSharedIPIPs(secondPage))

	// min_users 过滤仍然生效。
	items, total = list("", "", 0, 10)
	require.EqualValues(t, 4, total)
	_, filteredTotal, err := ListRiskSharedIPs(RiskIPSourceToken, "ip", "", 3, 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 2, filteredTotal)

	// email / status / quota 字段填充。
	items, _ = list("ip", "asc", 0, 10)
	require.Len(t, items[1].Users, 2)
	byUser := make(map[int]*RiskUserSummary, len(items[1].Users))
	for _, summary := range items[1].Users {
		byUser[summary.UserId] = summary
	}
	require.Equal(t, "risk-sort-a@example.com", byUser[users[0].Id].Email)
	require.Equal(t, common.UserStatusEnabled, byUser[users[0].Id].Status)
	require.Equal(t, 5000, byUser[users[0].Id].Quota)
	require.Empty(t, byUser[users[1].Id].Email)
	require.Equal(t, common.UserStatusDisabled, byUser[users[1].Id].Status)
	require.Equal(t, 0, byUser[users[1].Id].Quota)
}

func TestRiskUserKeywordMatchCondition(t *testing.T) {
	// 单字符数字关键词：只按用户 ID 精确匹配（LIKE 沿用既有最短 2 字符限制）。
	condition, args, err := riskUserKeywordMatchCondition("users.", "5")
	require.NoError(t, err)
	require.Contains(t, condition, "users.id = ?")
	require.NotContains(t, condition, "LIKE")
	require.Len(t, args, 1)

	// 多位数字关键词：ID 精确匹配 + username/display_name LIKE 同时保留。
	condition, args, err = riskUserKeywordMatchCondition("users.", "123")
	require.NoError(t, err)
	require.Contains(t, condition, "users.id = ?")
	require.Contains(t, condition, "LIKE")
	require.Len(t, args, 3)

	// 非数字关键词：沿用 LIKE 模糊搜索，长度不足时报错。
	_, _, err = riskUserKeywordMatchCondition("users.", "a")
	require.Error(t, err)
}

func TestListRiskInvitersGlobalAggregationSortAndFields(t *testing.T) {
	setupRiskControlTest(t)
	inviters := createRiskUsers(t, []User{
		{Username: "risk-inviter-a", DisplayName: "Risk Inviter A", Email: "inviter-a@example.com", AffCode: "risk-inviter-a-aff", Quota: 7000},
		{Username: "risk-inviter-b", Email: "inviter-b@example.com", AffCode: "risk-inviter-b-aff", Status: common.UserStatusDisabled},
		{Username: "risk-inviter-c", AffCode: "risk-inviter-c-aff"},
	})
	createRiskUsers(t, []User{
		{Username: "risk-ia-1", Email: "ia-1@example.com", AffCode: "risk-ia-1-aff", InviterId: inviters[0].Id},
		{Username: "risk-ia-2", Email: "ia-2@example.com", AffCode: "risk-ia-2-aff", InviterId: inviters[0].Id, Status: common.UserStatusDisabled},
		{Username: "risk-ia-3", AffCode: "risk-ia-3-aff", InviterId: inviters[0].Id},
		{Username: "risk-ib-1", AffCode: "risk-ib-1-aff", InviterId: inviters[1].Id},
		{Username: "risk-ic-1", AffCode: "risk-ic-1-aff", InviterId: inviters[2].Id},
		{Username: "risk-ic-2", AffCode: "risk-ic-2-aff", InviterId: inviters[2].Id},
	})

	list := func(keyword string, startIdx int, limit int, sortBy string, order string) ([]*RiskInviter, int64) {
		items, total, err := ListRiskInviters(keyword, startIdx, limit, sortBy, order, "")
		require.NoError(t, err)
		return items, total
	}

	// direct_invite_count desc：真实总数在数据库侧聚合，而不是仅当前页统计。
	items, total := list("", 0, 10, "", "")
	require.EqualValues(t, 3, total)
	require.Len(t, items, 3)
	require.Equal(t, []int{inviters[0].Id, inviters[2].Id, inviters[1].Id}, []int{items[0].InviterId, items[1].InviterId, items[2].InviterId})
	require.EqualValues(t, 3, items[0].DirectInviteCount)
	require.EqualValues(t, 2, items[1].DirectInviteCount)
	require.EqualValues(t, 1, items[2].DirectInviteCount)
	require.NotNil(t, items[0].MatchedInvitees)
	require.Empty(t, items[0].MatchedInvitees)

	// 邀请人 email / status / quota。
	require.Equal(t, "inviter-a@example.com", items[0].Email)
	require.Equal(t, common.UserStatusEnabled, items[0].Status)
	require.Equal(t, 7000, items[0].Quota)
	require.Equal(t, "inviter-b@example.com", items[2].Email)
	require.Equal(t, common.UserStatusDisabled, items[2].Status)
	require.Equal(t, 0, items[2].Quota)

	// 被邀请人 email / status。
	require.Len(t, items[0].Invitees, 3)
	var disabledInvitee *RiskUserSummary
	for _, summary := range items[0].Invitees {
		if summary.Status == common.UserStatusDisabled {
			disabledInvitee = summary
		}
	}
	require.NotNil(t, disabledInvitee)
	require.Equal(t, "ia-2@example.com", disabledInvitee.Email)

	// direct_invite_count asc。
	items, _ = list("", 0, 10, "direct_invite_count", "asc")
	require.Equal(t, []int{inviters[1].Id, inviters[2].Id, inviters[0].Id}, []int{items[0].InviterId, items[1].InviterId, items[2].InviterId})

	// inviter_id desc / asc。
	items, _ = list("", 0, 10, "inviter_id", "desc")
	require.Equal(t, []int{inviters[2].Id, inviters[1].Id, inviters[0].Id}, []int{items[0].InviterId, items[1].InviterId, items[2].InviterId})
	items, _ = list("", 0, 10, "inviter_id", "asc")
	require.Equal(t, []int{inviters[0].Id, inviters[1].Id, inviters[2].Id}, []int{items[0].InviterId, items[1].InviterId, items[2].InviterId})

	// 分页正确且 total 为匹配总数。
	firstPage, total := list("", 0, 2, "", "")
	require.EqualValues(t, 3, total)
	require.Len(t, firstPage, 2)
	secondPage, total := list("", 2, 2, "", "")
	require.EqualValues(t, 3, total)
	require.Len(t, secondPage, 1)
	require.Equal(t, inviters[1].Id, secondPage[0].InviterId)

	// keyword 过滤邀请人自身字段。
	filtered, filteredTotal := list("risk-inviter-b", 0, 10, "", "")
	require.EqualValues(t, 1, filteredTotal)
	require.Len(t, filtered, 1)
	require.Equal(t, inviters[1].Id, filtered[0].InviterId)
	require.EqualValues(t, 1, filtered[0].DirectInviteCount)

	// 非法排序参数回退 direct_invite_count desc。
	items, _ = list("", 0, 10, "bogus", "bogus")
	require.Equal(t, inviters[0].Id, items[0].InviterId)
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

	items, total, err := ListRiskInviters("risk-inviter", 0, 10, "direct_invite_count", "desc", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.EqualValues(t, 101, items[0].DirectInviteCount)
	require.Len(t, items[0].Invitees, 100)
}

func TestListRiskInvitersKeywordMatchesInviterOrInvitee(t *testing.T) {
	setupRiskControlTest(t)
	inviterAlpha := createRiskUsers(t, []User{
		{Username: "risk-kw-alpha", DisplayName: "Alpha Inviter", AffCode: "risk-kw-alpha-aff"},
	})[0]
	inviterBeta := createRiskUsers(t, []User{
		{Username: "risk-kw-beta", AffCode: "risk-kw-beta-aff"},
	})[0]
	inviterGamma := createRiskUsers(t, []User{
		{Username: "risk-kw-gamma", AffCode: "risk-kw-gamma-aff"},
	})[0]
	inviterBulk := createRiskUsers(t, []User{
		{Username: "risk-kw-bulk", AffCode: "risk-kw-bulk-aff"},
	})[0]
	inviteeTwo := createRiskUsers(t, []User{
		{Username: "risk-kw-invitee-two", Email: "kw-two@example.com", AffCode: "risk-kw-invitee-two-aff", InviterId: inviterAlpha.Id, Status: common.UserStatusDisabled},
	})[0]
	createRiskUsers(t, []User{
		{Username: "risk-kw-invitee-one", Email: "kw-one@example.com", AffCode: "risk-kw-invitee-one-aff", InviterId: inviterAlpha.Id},
		{Username: "risk-kw-invitee-three", AffCode: "risk-kw-invitee-three-aff", InviterId: inviterBeta.Id},
		{Username: "risk-kw-invitee-four", AffCode: "risk-kw-invitee-four-aff", InviterId: inviterGamma.Id},
	})
	bulkInvitees := make([]User, 25)
	for index := range bulkInvitees {
		bulkInvitees[index] = User{
			Username:  fmt.Sprintf("risk-kw-bulk-%02d", index),
			AffCode:   fmt.Sprintf("risk-kw-bulk-%02d-aff", index),
			InviterId: inviterBulk.Id,
		}
	}
	bulkInvitees = createRiskUsers(t, bulkInvitees)

	findInviter := func(items []*RiskInviter, inviterId int) *RiskInviter {
		for _, item := range items {
			if item.InviterId == inviterId {
				return item
			}
		}
		return nil
	}

	// 按被邀请人用户名搜索：返回邀请人行，matched_invitees 命中对应被邀请人。
	items, total, err := ListRiskInviters("risk-kw-invitee-two", 0, 10, "direct_invite_count", "desc", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviterAlpha.Id, items[0].InviterId)
	require.Len(t, items[0].MatchedInvitees, 1)
	require.Equal(t, inviteeTwo.Id, items[0].MatchedInvitees[0].UserId)
	require.Equal(t, "kw-two@example.com", items[0].MatchedInvitees[0].Email)
	require.Equal(t, common.UserStatusDisabled, items[0].MatchedInvitees[0].Status)

	// 按被邀请人用户 ID 搜索：数字关键词按用户 ID 精确命中邀请人。
	items, total, err = ListRiskInviters(strconv.Itoa(inviteeTwo.Id), 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviterAlpha.Id, items[0].InviterId)
	require.Len(t, items[0].MatchedInvitees, 1)
	require.Equal(t, inviteeTwo.Id, items[0].MatchedInvitees[0].UserId)

	// 按邀请人 ID 搜索：命中邀请人自身，matched_invitees 为空数组。
	items, total, err = ListRiskInviters(strconv.Itoa(inviterAlpha.Id), 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviterAlpha.Id, items[0].InviterId)
	require.NotNil(t, items[0].MatchedInvitees)
	require.Empty(t, items[0].MatchedInvitees)

	// 按邀请人用户名搜索：命中邀请人自身。
	items, total, err = ListRiskInviters("risk-kw-alpha", 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviterAlpha.Id, items[0].InviterId)
	require.Empty(t, items[0].MatchedInvitees)

	// 一个关键词命中多个邀请人的被邀请人：total 为匹配邀请人数。
	items, total, err = ListRiskInviters("risk-kw-invitee", 0, 10, "direct_invite_count", "desc", "")
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, items, 3)
	alphaRow := findInviter(items, inviterAlpha.Id)
	require.NotNil(t, alphaRow)
	require.Len(t, alphaRow.MatchedInvitees, 2)
	require.Equal(t, inviterBeta.Id, findInviter(items, inviterBeta.Id).InviterId)
	require.Len(t, findInviter(items, inviterBeta.Id).MatchedInvitees, 1)
	require.Len(t, findInviter(items, inviterGamma.Id).MatchedInvitees, 1)

	// 关键词无命中返回空列表。
	items, total, err = ListRiskInviters("risk-kw-no-such-user", 0, 10, "", "", "")
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, items)

	// matched_invitees 每个邀请人最多 20 个。
	items, total, err = ListRiskInviters("risk-kw-bulk", 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviterBulk.Id, items[0].InviterId)
	require.Len(t, items[0].MatchedInvitees, 20)
	require.Equal(t, bulkInvitees[24].Id, items[0].MatchedInvitees[0].UserId)

	// 关键词为空时 matched_invitees 为空数组（不是 nil）。
	items, total, err = ListRiskInviters("", 0, 10, "direct_invite_count", "desc", "")
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	for _, item := range items {
		require.NotNil(t, item.MatchedInvitees)
		require.Empty(t, item.MatchedInvitees)
	}
}

func TestListRiskInviteesPagination(t *testing.T) {
	setupRiskControlTest(t)
	inviter := createRiskUsers(t, []User{
		{Username: "risk-invitees-inviter", AffCode: "risk-invitees-inviter-aff"},
	})[0]
	invitees := createRiskUsers(t, []User{
		{Username: "risk-invitee-page-1", Email: "page-1@example.com", AffCode: "risk-invitee-page-1-aff", InviterId: inviter.Id},
		{Username: "risk-invitee-page-2", Email: "page-2@example.com", AffCode: "risk-invitee-page-2-aff", InviterId: inviter.Id},
		{Username: "risk-invitee-page-3", AffCode: "risk-invitee-page-3-aff", InviterId: inviter.Id, Status: common.UserStatusDisabled},
		{Username: "risk-invitee-page-4", AffCode: "risk-invitee-page-4-aff", InviterId: inviter.Id},
		{Username: "risk-invitee-page-5", AffCode: "risk-invitee-page-5-aff", InviterId: inviter.Id},
	})
	// 另一个邀请人的被邀请人不能混入。
	otherInviter := createRiskUsers(t, []User{{Username: "risk-invitees-other", AffCode: "risk-invitees-other-aff"}})[0]
	createRiskUsers(t, []User{{Username: "risk-invitee-other-1", AffCode: "risk-invitee-other-aff", InviterId: otherInviter.Id}})

	firstPage, total, err := ListRiskInvitees(inviter.Id, 0, 2)
	require.NoError(t, err)
	require.EqualValues(t, 5, total)
	require.Len(t, firstPage, 2)
	require.Equal(t, invitees[4].Id, firstPage[0].UserId)
	require.Equal(t, invitees[3].Id, firstPage[1].UserId)
	require.Empty(t, firstPage[1].Email)
	require.Equal(t, common.RoleCommonUser, firstPage[1].Role)
	require.Equal(t, inviter.Id, firstPage[1].InviterId)

	secondPage, total, err := ListRiskInvitees(inviter.Id, 2, 2)
	require.NoError(t, err)
	require.EqualValues(t, 5, total)
	require.Len(t, secondPage, 2)
	require.Equal(t, invitees[2].Id, secondPage[0].UserId)
	require.Equal(t, common.UserStatusDisabled, secondPage[0].Status)
	require.Equal(t, invitees[1].Id, secondPage[1].UserId)
	require.Equal(t, "page-2@example.com", secondPage[1].Email)

	thirdPage, total, err := ListRiskInvitees(inviter.Id, 4, 2)
	require.NoError(t, err)
	require.EqualValues(t, 5, total)
	require.Len(t, thirdPage, 1)
	require.Equal(t, invitees[0].Id, thirdPage[0].UserId)

	emptyPage, total, err := ListRiskInvitees(inviter.Id, 6, 2)
	require.NoError(t, err)
	require.EqualValues(t, 5, total)
	require.Empty(t, emptyPage)

	// 非法 / 不存在的邀请人返回空列表而不是错误。
	emptyPage, total, err = ListRiskInvitees(0, 0, 10)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, emptyPage)
	emptyPage, total, err = ListRiskInvitees(-3, 0, 10)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, emptyPage)
	emptyPage, total, err = ListRiskInvitees(inviter.Id+100000, 0, 10)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, emptyPage)
}

func TestBatchUpdateRiskUserStatusFiltersAndSkips(t *testing.T) {
	setupRiskControlTest(t)
	created := createRiskUsers(t, []User{
		{Username: "risk-batch-root", AffCode: "risk-batch-root-aff", Role: common.RoleRootUser},
		{Username: "risk-batch-admin", AffCode: "risk-batch-admin-aff", Role: common.RoleAdminUser},
		{Username: "risk-batch-common-a", AffCode: "risk-batch-common-a-aff"},
		{Username: "risk-batch-common-b", AffCode: "risk-batch-common-b-aff"},
		{Username: "risk-batch-disabled", AffCode: "risk-batch-disabled-aff", Status: common.UserStatusDisabled},
	})
	rootUser, adminUser, commonA, commonB, disabledUser := created[0], created[1], created[2], created[3], created[4]
	missingId := rootUser.Id + 1000000

	reloadStatus := func(userId int) int {
		var user User
		require.NoError(t, DB.Select("id, status").First(&user, userId).Error)
		return user.Status
	}

	// admin 操作：common 用户成功；同级 admin / root / 不存在用户被跳过。
	updated, skipped, err := BatchUpdateRiskUserStatus(common.RoleAdminUser, "disable", []int{
		commonA.Id, adminUser.Id, rootUser.Id, missingId, commonB.Id,
	})
	require.NoError(t, err)
	require.Equal(t, 2, updated)
	require.Len(t, skipped, 3)
	require.Equal(t, RiskStatusSkipInsufficientPrivilege, riskStatusSkipReason(t, skipped, adminUser.Id))
	require.Equal(t, RiskStatusSkipRootUser, riskStatusSkipReason(t, skipped, rootUser.Id))
	require.Equal(t, RiskStatusSkipNotFound, riskStatusSkipReason(t, skipped, missingId))
	require.Equal(t, common.UserStatusDisabled, reloadStatus(commonA.Id))
	require.Equal(t, common.UserStatusDisabled, reloadStatus(commonB.Id))
	require.Equal(t, common.UserStatusEnabled, reloadStatus(adminUser.Id))
	require.Equal(t, common.UserStatusEnabled, reloadStatus(rootUser.Id))

	// 已封禁账号幂等跳过。
	updated, skipped, err = BatchUpdateRiskUserStatus(common.RoleAdminUser, "disable", []int{commonA.Id, commonB.Id})
	require.NoError(t, err)
	require.Zero(t, updated)
	require.Len(t, skipped, 2)
	require.Equal(t, RiskStatusSkipAlreadyDisabled, riskStatusSkipReason(t, skipped, commonA.Id))
	require.Equal(t, RiskStatusSkipAlreadyDisabled, riskStatusSkipReason(t, skipped, commonB.Id))

	// enable 解禁；再次 enable 幂等跳过。
	updated, skipped, err = BatchUpdateRiskUserStatus(common.RoleAdminUser, "enable", []int{commonA.Id, commonB.Id, disabledUser.Id})
	require.NoError(t, err)
	require.Equal(t, 3, updated)
	require.Empty(t, skipped)
	require.Equal(t, common.UserStatusEnabled, reloadStatus(commonA.Id))
	require.Equal(t, common.UserStatusEnabled, reloadStatus(commonB.Id))
	require.Equal(t, common.UserStatusEnabled, reloadStatus(disabledUser.Id))

	updated, skipped, err = BatchUpdateRiskUserStatus(common.RoleAdminUser, "enable", []int{commonA.Id})
	require.NoError(t, err)
	require.Zero(t, updated)
	require.Len(t, skipped, 1)
	require.Equal(t, RiskStatusSkipAlreadyEnabled, riskStatusSkipReason(t, skipped, commonA.Id))

	// root 可封禁 admin；root 自身永远跳过。
	updated, skipped, err = BatchUpdateRiskUserStatus(common.RoleRootUser, "disable", []int{adminUser.Id, rootUser.Id})
	require.NoError(t, err)
	require.Equal(t, 1, updated)
	require.Len(t, skipped, 1)
	require.Equal(t, RiskStatusSkipRootUser, riskStatusSkipReason(t, skipped, rootUser.Id))
	require.Equal(t, common.UserStatusDisabled, reloadStatus(adminUser.Id))
	require.Equal(t, common.UserStatusEnabled, reloadStatus(rootUser.Id))

	// 重复 id 在模型层去重。
	updated, skipped, err = BatchUpdateRiskUserStatus(common.RoleRootUser, "enable", []int{adminUser.Id, adminUser.Id})
	require.NoError(t, err)
	require.Equal(t, 1, updated)
	require.Empty(t, skipped)
	require.Equal(t, common.UserStatusEnabled, reloadStatus(adminUser.Id))

	// 非法 action 返回错误（controller 会先拦截）。
	updated, skipped, err = BatchUpdateRiskUserStatus(common.RoleRootUser, "delete", []int{commonA.Id})
	require.Error(t, err)
	require.Zero(t, updated)
	require.Empty(t, skipped)
	require.Equal(t, common.UserStatusEnabled, reloadStatus(commonA.Id))

	// 空输入不产生更新。
	updated, skipped, err = BatchUpdateRiskUserStatus(common.RoleRootUser, "disable", nil)
	require.NoError(t, err)
	require.Zero(t, updated)
	require.Empty(t, skipped)
}

func TestResolveRiskUserIDsBySharedIPAndInviter(t *testing.T) {
	setupRiskControlTest(t)
	inviter := createRiskUsers(t, []User{
		{Username: "risk-resolve-inviter", AffCode: "risk-resolve-inviter-aff"},
	})[0]
	users := createRiskUsers(t, []User{
		{Username: "risk-resolve-a", AffCode: "risk-resolve-a-aff", InviterId: inviter.Id},
		{Username: "risk-resolve-b", AffCode: "risk-resolve-b-aff", InviterId: inviter.Id},
		{Username: "risk-resolve-c", AffCode: "risk-resolve-c-aff", InviterId: inviter.Id},
		{Username: "risk-resolve-d", AffCode: "risk-resolve-d-aff"},
	})
	records := []RiskIPRecord{
		{UserId: users[0].Id, TokenId: 1, IP: "203.0.113.50", Source: RiskIPSourceLogin, FirstSeenAt: 1, LastSeenAt: 1, EventCount: 1},
		{UserId: users[0].Id, TokenId: 2, IP: "203.0.113.50", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 1, EventCount: 1},
		{UserId: users[1].Id, TokenId: 2, IP: "203.0.113.50", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 1, EventCount: 1},
		{UserId: users[2].Id, TokenId: 1, IP: "203.0.113.50", Source: RiskIPSourceLogin, FirstSeenAt: 1, LastSeenAt: 1, EventCount: 1},
		{UserId: users[3].Id, TokenId: 1, IP: "203.0.113.51", Source: RiskIPSourceLogin, FirstSeenAt: 1, LastSeenAt: 1, EventCount: 1},
	}
	require.NoError(t, DB.Create(&records).Error)

	allSources, err := ResolveRiskUserIDsBySharedIP("203.0.113.50", "")
	require.NoError(t, err)
	require.Equal(t, []int{users[0].Id, users[1].Id, users[2].Id}, allSources)

	loginUsers, err := ResolveRiskUserIDsBySharedIP("203.0.113.50", RiskIPSourceLogin)
	require.NoError(t, err)
	require.Equal(t, []int{users[0].Id, users[2].Id}, loginUsers)

	tokenUsers, err := ResolveRiskUserIDsBySharedIP("203.0.113.50", RiskIPSourceToken)
	require.NoError(t, err)
	require.Equal(t, []int{users[0].Id, users[1].Id}, tokenUsers)

	emptyIds, err := ResolveRiskUserIDsBySharedIP("not-an-ip", "")
	require.NoError(t, err)
	require.Empty(t, emptyIds)

	emptyIds, err = ResolveRiskUserIDsBySharedIP("203.0.113.99", "")
	require.NoError(t, err)
	require.Empty(t, emptyIds)

	invitedIds, err := ResolveRiskUserIDsByInviter(inviter.Id)
	require.NoError(t, err)
	require.Equal(t, []int{users[0].Id, users[1].Id, users[2].Id}, invitedIds)

	emptyIds, err = ResolveRiskUserIDsByInviter(0)
	require.NoError(t, err)
	require.Empty(t, emptyIds)

	emptyIds, err = ResolveRiskUserIDsByInviter(inviter.Id + 1000000)
	require.NoError(t, err)
	require.Empty(t, emptyIds)
}

func TestInvalidateUsersCacheAndTokensWithoutRedis(t *testing.T) {
	setupRiskControlTest(t)
	users := createRiskUsers(t, []User{
		{Username: "risk-cache-a", AffCode: "risk-cache-a-aff"},
		{Username: "risk-cache-b", AffCode: "risk-cache-b-aff"},
	})
	require.False(t, common.RedisEnabled)
	// Redis 未启用时为空操作，不应 panic，也不影响数据库。
	require.NotPanics(t, func() {
		InvalidateUsersCacheAndTokens([]int{users[0].Id, users[1].Id})
		InvalidateUsersCacheAndTokens(nil)
	})
	var count int64
	require.NoError(t, DB.Model(&User{}).Where("id IN ?", []int{users[0].Id, users[1].Id}).Count(&count).Error)
	require.EqualValues(t, 2, count)
}

func TestListRiskSharedIPsFiltersByBanState(t *testing.T) {
	setupRiskControlTest(t)
	users := createRiskUsers(t, []User{
		{Username: "risk-ban-normal", AffCode: "risk-ban-normal-aff"},
		{Username: "risk-ban-disabled", AffCode: "risk-ban-disabled-aff", Status: common.UserStatusDisabled},
		{Username: "risk-ban-disabled-two", AffCode: "risk-ban-disabled-two-aff", Status: common.UserStatusDisabled},
		{Username: "risk-ban-normal-two", AffCode: "risk-ban-normal-two-aff"},
	})
	records := []RiskIPRecord{
		// 203.0.113.10：普通 + 已封禁 → 命中 has_banned，不命中 all_banned。
		{UserId: users[0].Id, TokenId: 1, IP: "203.0.113.10", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 10, EventCount: 1},
		{UserId: users[1].Id, TokenId: 1, IP: "203.0.113.10", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 10, EventCount: 1},
		// 203.0.113.11：两个已封禁 → 同时命中 has_banned 与 all_banned。
		{UserId: users[1].Id, TokenId: 2, IP: "203.0.113.11", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 20, EventCount: 1},
		{UserId: users[2].Id, TokenId: 2, IP: "203.0.113.11", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 20, EventCount: 1},
		// 203.0.113.12：全部正常 → 两个筛选都不命中。
		{UserId: users[0].Id, TokenId: 3, IP: "203.0.113.12", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 30, EventCount: 1},
		{UserId: users[3].Id, TokenId: 3, IP: "203.0.113.12", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 30, EventCount: 1},
	}
	require.NoError(t, DB.Create(&records).Error)

	list := func(banState string) ([]string, int64) {
		items, total, err := ListRiskSharedIPs(RiskIPSourceToken, "ip", "", 2, 0, 10, "ip", "asc", banState)
		require.NoError(t, err)
		return riskSharedIPIPs(items), total
	}

	// 不筛选：三组都在。
	ips, total := list("")
	require.Equal(t, []string{"203.0.113.10", "203.0.113.11", "203.0.113.12"}, ips)
	require.EqualValues(t, 3, total)

	// 非法值等同不筛选。
	ips, total = list("bogus")
	require.Equal(t, []string{"203.0.113.10", "203.0.113.11", "203.0.113.12"}, ips)
	require.EqualValues(t, 3, total)

	// has_banned：关联账号中存在已封禁。
	ips, total = list(RiskBanStateHasBanned)
	require.Equal(t, []string{"203.0.113.10", "203.0.113.11"}, ips)
	require.EqualValues(t, 2, total)

	// all_banned：关联账号全部已封禁。
	ips, total = list(RiskBanStateAllBanned)
	require.Equal(t, []string{"203.0.113.11"}, ips)
	require.EqualValues(t, 1, total)

	// 软删除用户不计入封禁统计：删掉 203.0.113.11 中的普通账号后，该组不再全部封禁。
	require.NoError(t, DB.Delete(&User{}, users[1].Id).Error)
	ips, total = list(RiskBanStateAllBanned)
	require.Empty(t, ips)
	require.EqualValues(t, 0, total)
}

func TestListRiskInvitersFiltersByBanState(t *testing.T) {
	setupRiskControlTest(t)
	inviters := createRiskUsers(t, []User{
		{Username: "risk-ban-inviter-mixed", AffCode: "risk-ban-inviter-mixed-aff"},
		{Username: "risk-ban-inviter-all", AffCode: "risk-ban-inviter-all-aff"},
		{Username: "risk-ban-inviter-clean", AffCode: "risk-ban-inviter-clean-aff"},
	})
	invitees := createRiskUsers(t, []User{
		// 混合：1 个已封禁 + 1 个正常。
		{Username: "risk-ban-invitee-mixed-a", AffCode: "risk-ban-invitee-mixed-a-aff", InviterId: inviters[0].Id},
		{Username: "risk-ban-invitee-mixed-b", AffCode: "risk-ban-invitee-mixed-b-aff", InviterId: inviters[0].Id},
		// 全部封禁。
		{Username: "risk-ban-invitee-all-a", AffCode: "risk-ban-invitee-all-a-aff", InviterId: inviters[1].Id, Status: common.UserStatusDisabled},
		{Username: "risk-ban-invitee-all-b", AffCode: "risk-ban-invitee-all-b-aff", InviterId: inviters[1].Id, Status: common.UserStatusDisabled},
		// 全部正常。
		{Username: "risk-ban-invitee-clean-a", AffCode: "risk-ban-invitee-clean-a-aff", InviterId: inviters[2].Id},
		{Username: "risk-ban-invitee-clean-b", AffCode: "risk-ban-invitee-clean-b-aff", InviterId: inviters[2].Id},
	})
	require.NotZero(t, invitees[0].Id)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", invitees[0].Id).Update("status", common.UserStatusDisabled).Error)

	usernames := func(rows []*RiskInviter) []string {
		result := make([]string, 0, len(rows))
		for _, row := range rows {
			result = append(result, row.Username)
		}
		return result
	}
	list := func(banState string) ([]string, int64) {
		items, total, err := ListRiskInviters("", 0, 10, "inviter_id", "asc", banState)
		require.NoError(t, err)
		return usernames(items), total
	}

	ips, total := list("")
	require.Equal(t, []string{"risk-ban-inviter-mixed", "risk-ban-inviter-all", "risk-ban-inviter-clean"}, ips)
	require.EqualValues(t, 3, total)

	ips, total = list(RiskBanStateHasBanned)
	require.Equal(t, []string{"risk-ban-inviter-mixed", "risk-ban-inviter-all"}, ips)
	require.EqualValues(t, 2, total)

	ips, total = list(RiskBanStateAllBanned)
	require.Equal(t, []string{"risk-ban-inviter-all"}, ips)
	require.EqualValues(t, 1, total)

	// 与关键词搜索叠加：命中被邀请人且存在已封禁。
	items, total, err := ListRiskInviters("risk-ban-invitee-mixed", 0, 10, "inviter_id", "asc", RiskBanStateHasBanned)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, []string{"risk-ban-inviter-mixed"}, usernames(items))

	// 关键词命中但无已封禁账号时，筛选后为空。
	items, total, err = ListRiskInviters("risk-ban-invitee-clean", 0, 10, "inviter_id", "asc", RiskBanStateHasBanned)
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.Empty(t, items)

	// 软删除被邀请人不参与封禁统计：删掉唯一正常被邀请人后，mixed 变为全部封禁。
	require.NoError(t, DB.Delete(&User{}, invitees[1].Id).Error)
	ips, total = list(RiskBanStateAllBanned)
	require.Equal(t, []string{"risk-ban-inviter-mixed", "risk-ban-inviter-all"}, ips)
	require.EqualValues(t, 2, total)
}

func TestNormalizeRiskBanState(t *testing.T) {
	require.Equal(t, "", normalizeRiskBanState(""))
	require.Equal(t, RiskBanStateHasBanned, normalizeRiskBanState(" has_banned "))
	require.Equal(t, RiskBanStateAllBanned, normalizeRiskBanState("all_banned"))
	require.Equal(t, "", normalizeRiskBanState("disabled"))
	require.Equal(t, "", normalizeRiskBanState("ALL_BANNED"))
}
