package model

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// users / risk_ip_records 的 varchar 列长度上限（以 MySQL 建表为准）。
// SQLite 不校验 varchar 长度，超长值只会在 MySQL 严格模式下报 1406，因此测试侧主动防呆。
const (
	riskTestUserUsernameMaxLen    = 191
	riskTestUserDisplayNameMaxLen = 191
	riskTestUserEmailMaxLen       = 191
	riskTestUserAffCodeMaxLen     = 32
	riskTestIPMaxLen              = 64
	riskTestIPSourceMaxLen        = 16
)

func riskTestAssertStringLimit(t *testing.T, field string, value string, maxLen int) {
	t.Helper()
	require.LessOrEqualf(t, len(value), maxLen, "%s 长度 %d 超过数据库列上限 %d: %q", field, len(value), maxLen, value)
}

// riskTestAssertUserColumnLimits 在创建用户前校验有长度约束的列，避免只在 MySQL 暴露。
func riskTestAssertUserColumnLimits(t *testing.T, user User) {
	t.Helper()
	riskTestAssertStringLimit(t, "users.username", user.Username, riskTestUserUsernameMaxLen)
	riskTestAssertStringLimit(t, "users.display_name", user.DisplayName, riskTestUserDisplayNameMaxLen)
	riskTestAssertStringLimit(t, "users.email", user.Email, riskTestUserEmailMaxLen)
	riskTestAssertStringLimit(t, "users.aff_code", user.AffCode, riskTestUserAffCodeMaxLen)
}

// riskTestAssertRiskIPRecordColumnLimits 在创建 IP 记录前校验有长度约束的列。
func riskTestAssertRiskIPRecordColumnLimits(t *testing.T, record RiskIPRecord) {
	t.Helper()
	riskTestAssertStringLimit(t, "risk_ip_records.ip", record.IP, riskTestIPMaxLen)
	riskTestAssertStringLimit(t, "risk_ip_records.source", record.Source, riskTestIPSourceMaxLen)
}

// riskTestAssertDBSnapshotColumnLimits 在用例清理阶段扫描本用例写入的数据，
// 兜底覆盖所有未走 createRiskUsers 的直接 DB.Create 调用点。
// 这里用 assert 而非 require，保证即使校验失败也能继续执行清理，不污染后续用例。
func riskTestAssertDBSnapshotColumnLimits(t *testing.T) {
	t.Helper()
	var users []*User
	if err := DB.Unscoped().Select("id, username, display_name, email, aff_code").Find(&users).Error; err != nil {
		assert.NoError(t, err, "扫描 users 校验列长度失败")
		return
	}
	for _, user := range users {
		assert.LessOrEqualf(t, len(user.Username), riskTestUserUsernameMaxLen, "users.username 超长: %q", user.Username)
		assert.LessOrEqualf(t, len(user.DisplayName), riskTestUserDisplayNameMaxLen, "users.display_name 超长: %q", user.DisplayName)
		assert.LessOrEqualf(t, len(user.Email), riskTestUserEmailMaxLen, "users.email 超长: %q", user.Email)
		assert.LessOrEqualf(t, len(user.AffCode), riskTestUserAffCodeMaxLen, "users.aff_code 超长: %q", user.AffCode)
	}
	var records []*RiskIPRecord
	if err := DB.Select("id, ip, source").Find(&records).Error; err != nil {
		assert.NoError(t, err, "扫描 risk_ip_records 校验列长度失败")
		return
	}
	for _, record := range records {
		assert.LessOrEqualf(t, len(record.IP), riskTestIPMaxLen, "risk_ip_records.ip 超长: %q", record.IP)
		assert.LessOrEqualf(t, len(record.Source), riskTestIPSourceMaxLen, "risk_ip_records.source 超长: %q", record.Source)
	}
}

func setupRiskControlTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&RiskIPRecord{}, &Checkin{}))
	riskIPWriteState = sync.Map{}
	t.Cleanup(func() {
		riskTestAssertDBSnapshotColumnLimits(t)
		DB.Exec("DELETE FROM risk_ip_records")
		DB.Exec("DELETE FROM checkins")
		DB.Exec("DELETE FROM top_ups")
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
		riskTestAssertUserColumnLimits(t, users[index])
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

// defaultListRiskSharedIPs 封装未使用新增筛选参数（active_24h 等）的既有调用点。
func defaultListRiskSharedIPs(source string, searchType string, keyword string, minUsers int, startIdx int, limit int, sortBy string, order string, banState string) ([]*RiskSharedIP, int64, error) {
	return ListRiskSharedIPs(source, searchType, keyword, minUsers, startIdx, limit, sortBy, order, banState, false)
}

// defaultListRiskInviters 封装未使用搜索类型/筛选参数的既有调用点（三期参数全默认）。
func defaultListRiskInviters(keyword string, startIdx int, limit int, sortBy string, order string, banState string) ([]*RiskInviter, int64, error) {
	return ListRiskInviters(keyword, startIdx, limit, sortBy, order, banState, "any", false, 0, "", riskInviterDefaultRecentInvitesWindow, 0, 0)
}

// listRiskInvitersPhase2Defaults 为二期参数调用点补齐三期新增筛选的默认值（不筛选）。
func listRiskInvitersPhase2Defaults(keyword string, startIdx int, limit int, sortBy string, order string, banState string, searchType string, suspiciousOnly bool, minInvites int, inviterStatus string) ([]*RiskInviter, int64, error) {
	return ListRiskInviters(keyword, startIdx, limit, sortBy, order, banState, searchType, suspiciousOnly, minInvites, inviterStatus, riskInviterDefaultRecentInvitesWindow, 0, 0)
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
	riskTestAssertUserColumnLimits(t, user)
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
		riskTestAssertUserColumnLimits(t, users[index])
		require.NoError(t, DB.Create(&users[index]).Error)
		require.NoError(t, DB.Create(&RiskIPRecord{UserId: users[index].Id, TokenId: index + 1, IP: "198.51.100.8", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 200 + int64(index), EventCount: 3}).Error)
	}

	items, total, err := defaultListRiskSharedIPs(RiskIPSourceToken, "ip", "198.51.100", 2, 0, 10, "last_seen_at", "desc", "")
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
	items, total, err = defaultListRiskSharedIPs("", "ip", "", 2, 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "all", items[0].Source)
	require.EqualValues(t, 2, items[0].UserCount)
	require.Len(t, items[0].Users, 2)

	items, total, err = defaultListRiskSharedIPs(RiskIPSourceToken, "username", "risk-shared-a", 2, 0, 10, "user_count", "asc", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.EqualValues(t, 2, items[0].UserCount)

	items, total, err = defaultListRiskSharedIPs(RiskIPSourceToken, "user_id", strconv.Itoa(users[1].Id), 2, 0, 10, "ip", "asc", "")
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
		items, total, err := defaultListRiskSharedIPs(RiskIPSourceToken, "ip", "", 2, startIdx, limit, sortBy, order, "")
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
	_, filteredTotal, err := defaultListRiskSharedIPs(RiskIPSourceToken, "ip", "", 3, 0, 10, "", "", "")
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

	// 多位数字关键词：ID 精确匹配 + username/display_name/email LIKE 同时保留。
	condition, args, err = riskUserKeywordMatchCondition("users.", "123")
	require.NoError(t, err)
	require.Contains(t, condition, "users.id = ?")
	require.Contains(t, condition, "users.username LIKE")
	require.Contains(t, condition, "users.display_name LIKE")
	require.Contains(t, condition, "users.email LIKE")
	require.Len(t, args, 4)

	// 非数字关键词：id 不参与，username/display_name/email 模糊搜索。
	condition, args, err = riskUserKeywordMatchCondition("users.", "alice@")
	require.NoError(t, err)
	require.NotContains(t, condition, "users.id = ?")
	require.Contains(t, condition, "users.email LIKE")
	require.Len(t, args, 3)

	// 非数字关键词长度不足时报错。
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
		items, total, err := defaultListRiskInviters(keyword, startIdx, limit, sortBy, order, "")
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
	riskTestAssertUserColumnLimits(t, inviter)
	require.NoError(t, DB.Create(&inviter).Error)
	invitees := make([]User, 101)
	for index := range invitees {
		invitees[index] = User{Username: fmt.Sprintf("risk-invitee-%03d", index), Password: "test-password", AffCode: fmt.Sprintf("risk-invitee-aff-%03d", index), InviterId: inviter.Id}
		riskTestAssertUserColumnLimits(t, invitees[index])
	}
	require.NoError(t, DB.CreateInBatches(&invitees, 50).Error)

	items, total, err := defaultListRiskInviters("risk-inviter", 0, 10, "direct_invite_count", "desc", "")
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
	items, total, err := defaultListRiskInviters("risk-kw-invitee-two", 0, 10, "direct_invite_count", "desc", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviterAlpha.Id, items[0].InviterId)
	require.Len(t, items[0].MatchedInvitees, 1)
	require.Equal(t, inviteeTwo.Id, items[0].MatchedInvitees[0].UserId)
	require.Equal(t, "kw-two@example.com", items[0].MatchedInvitees[0].Email)
	require.Equal(t, common.UserStatusDisabled, items[0].MatchedInvitees[0].Status)

	// 按被邀请人用户 ID 搜索：数字关键词按用户 ID 精确命中邀请人。
	items, total, err = defaultListRiskInviters(strconv.Itoa(inviteeTwo.Id), 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviterAlpha.Id, items[0].InviterId)
	require.Len(t, items[0].MatchedInvitees, 1)
	require.Equal(t, inviteeTwo.Id, items[0].MatchedInvitees[0].UserId)

	// 按邀请人 ID 搜索：命中邀请人自身，matched_invitees 为空数组。
	items, total, err = defaultListRiskInviters(strconv.Itoa(inviterAlpha.Id), 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviterAlpha.Id, items[0].InviterId)
	require.NotNil(t, items[0].MatchedInvitees)
	require.Empty(t, items[0].MatchedInvitees)

	// 按邀请人用户名搜索：命中邀请人自身。
	items, total, err = defaultListRiskInviters("risk-kw-alpha", 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviterAlpha.Id, items[0].InviterId)
	require.Empty(t, items[0].MatchedInvitees)

	// 一个关键词命中多个邀请人的被邀请人：total 为匹配邀请人数。
	items, total, err = defaultListRiskInviters("risk-kw-invitee", 0, 10, "direct_invite_count", "desc", "")
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
	items, total, err = defaultListRiskInviters("risk-kw-no-such-user", 0, 10, "", "", "")
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, items)

	// matched_invitees 每个邀请人最多 20 个。
	items, total, err = defaultListRiskInviters("risk-kw-bulk", 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviterBulk.Id, items[0].InviterId)
	require.Len(t, items[0].MatchedInvitees, 20)
	require.Equal(t, bulkInvitees[24].Id, items[0].MatchedInvitees[0].UserId)

	// 关键词为空时 matched_invitees 为空数组（不是 nil）。
	items, total, err = defaultListRiskInviters("", 0, 10, "direct_invite_count", "desc", "")
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
		items, total, err := defaultListRiskSharedIPs(RiskIPSourceToken, "ip", "", 2, 0, 10, "ip", "asc", banState)
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
		items, total, err := defaultListRiskInviters("", 0, 10, "inviter_id", "asc", banState)
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
	items, total, err := defaultListRiskInviters("risk-ban-invitee-mixed", 0, 10, "inviter_id", "asc", RiskBanStateHasBanned)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, []string{"risk-ban-inviter-mixed"}, usernames(items))

	// 关键词命中但无已封禁账号时，筛选后为空。
	items, total, err = defaultListRiskInviters("risk-ban-invitee-clean", 0, 10, "inviter_id", "asc", RiskBanStateHasBanned)
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

// ---------------------------------------------------------------------------
// 风控排序扩充（阶段 A）：shared IP 的 first_seen_at 排序
// ---------------------------------------------------------------------------

func TestListRiskSharedIPsSortsByFirstSeenAt(t *testing.T) {
	setupRiskControlTest(t)
	users := createRiskUsers(t, []User{
		{Username: "risk-firstseen-a", AffCode: "risk-firstseen-a-aff"},
		{Username: "risk-firstseen-b", AffCode: "risk-firstseen-b-aff"},
		{Username: "risk-firstseen-c", AffCode: "risk-firstseen-c-aff"},
	})
	userA, userB, userC := users[0].Id, users[1].Id, users[2].Id
	records := []RiskIPRecord{
		// 198.51.100.10：一组内最早 first_seen_at=10（另有条目 70），last_seen 400。
		{UserId: userA, TokenId: 1, IP: "198.51.100.10", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 400, EventCount: 1},
		{UserId: userB, TokenId: 1, IP: "198.51.100.10", Source: RiskIPSourceToken, FirstSeenAt: 70, LastSeenAt: 350, EventCount: 1},
		// 198.51.100.20：最早 20；last_seen 最高（500），用于确认没有回退到 last_seen_at 排序。
		{UserId: userA, TokenId: 1, IP: "198.51.100.20", Source: RiskIPSourceToken, FirstSeenAt: 20, LastSeenAt: 500, EventCount: 1},
		{UserId: userB, TokenId: 1, IP: "198.51.100.20", Source: RiskIPSourceToken, FirstSeenAt: 60, LastSeenAt: 450, EventCount: 1},
		// 198.51.100.25：最早 20，与 .20 相同；last_seen 最低（100），验证稳定次序按 ip ASC。
		{UserId: userB, TokenId: 1, IP: "198.51.100.25", Source: RiskIPSourceToken, FirstSeenAt: 20, LastSeenAt: 100, EventCount: 1},
		{UserId: userC, TokenId: 1, IP: "198.51.100.25", Source: RiskIPSourceToken, FirstSeenAt: 50, LastSeenAt: 90, EventCount: 1},
		// 198.51.100.30：最早 30。
		{UserId: userB, TokenId: 1, IP: "198.51.100.30", Source: RiskIPSourceToken, FirstSeenAt: 30, LastSeenAt: 200, EventCount: 1},
		{UserId: userC, TokenId: 1, IP: "198.51.100.30", Source: RiskIPSourceToken, FirstSeenAt: 80, LastSeenAt: 190, EventCount: 1},
		// 198.51.100.40：最早 40。
		{UserId: userC, TokenId: 1, IP: "198.51.100.40", Source: RiskIPSourceToken, FirstSeenAt: 40, LastSeenAt: 300, EventCount: 1},
		{UserId: userA, TokenId: 1, IP: "198.51.100.40", Source: RiskIPSourceToken, FirstSeenAt: 70, LastSeenAt: 290, EventCount: 1},
	}
	require.NoError(t, DB.Create(&records).Error)

	list := func(order string, startIdx int, limit int) ([]string, int64) {
		items, total, err := defaultListRiskSharedIPs(RiskIPSourceToken, "ip", "", 2, startIdx, limit, "first_seen_at", order, "")
		require.NoError(t, err)
		return riskSharedIPIPs(items), total
	}

	// MIN(first_seen_at) asc：组内取最早值，而不是任一单条记录的值。
	ascending := []string{"198.51.100.10", "198.51.100.20", "198.51.100.25", "198.51.100.30", "198.51.100.40"}
	// MIN(first_seen_at) desc：.20 / .25 同为 20，稳定次序 ip ASC 仍生效。
	descending := []string{"198.51.100.40", "198.51.100.30", "198.51.100.20", "198.51.100.25", "198.51.100.10"}

	ips, total := list("asc", 0, 10)
	require.EqualValues(t, 5, total)
	require.Equal(t, ascending, ips)

	ips, total = list("desc", 0, 10)
	require.EqualValues(t, 5, total)
	require.Equal(t, descending, ips)

	// order 非法时回退 desc（与 normalizeRiskOrder 的默认一致）。
	ips, _ = list("sideways", 0, 10)
	require.Equal(t, descending, ips)

	// 稳定翻页：并列值 20 的 .20 / .25 正好跨页，两页拼接后无重复、无丢失。
	firstPage, total := list("asc", 0, 2)
	require.EqualValues(t, 5, total)
	require.Equal(t, []string{"198.51.100.10", "198.51.100.20"}, firstPage)
	secondPage, total := list("asc", 2, 2)
	require.EqualValues(t, 5, total)
	require.Equal(t, []string{"198.51.100.25", "198.51.100.30"}, secondPage)
	thirdPage, total := list("asc", 4, 2)
	require.EqualValues(t, 5, total)
	require.Equal(t, []string{"198.51.100.40"}, thirdPage)

	paged := append(append(append([]string{}, firstPage...), secondPage...), thirdPage...)
	assert.Equal(t, ascending, paged)
	assert.Len(t, paged, len(ascending), "两页拼接后行数应与总行数一致")
	require.Equal(t, len(paged), len(uniqueRiskStrings(paged)), "翻页拼接不应出现重复行")
}

// uniqueRiskStrings 返回去重后的字符串切片，用于校验翻页无重复。
func uniqueRiskStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// ---------------------------------------------------------------------------
// 风控排序扩充（阶段 A）：邀请关系的 created_at / aff_quota 排序与稳定翻页
// ---------------------------------------------------------------------------

func TestListRiskInvitersSortsByCreatedAtAndAffQuota(t *testing.T) {
	setupRiskControlTest(t)
	// created_at：C 与 D 同为 200（验证 users.id ASC 稳定次序）；aff_quota：A 与 D 同为 100。
	inviters := createRiskUsers(t, []User{
		{Username: "risk-order-a", AffCode: "risk-order-a-aff", CreatedAt: 300, AffQuota: 100},
		{Username: "risk-order-b", AffCode: "risk-order-b-aff", CreatedAt: 100, AffQuota: 500},
		{Username: "risk-order-c", AffCode: "risk-order-c-aff", CreatedAt: 200, AffQuota: 300},
		{Username: "risk-order-d", AffCode: "risk-order-d-aff", CreatedAt: 200, AffQuota: 100},
		{Username: "risk-order-e", AffCode: "risk-order-e-aff", CreatedAt: 400, AffQuota: 200},
	})
	// 每个邀请人至少一个被邀请人，才会出现在邀请关系聚合结果中。
	createRiskUsers(t, []User{
		{Username: "risk-order-invitee-a1", AffCode: "risk-order-invitee-a1-aff", InviterId: inviters[0].Id},
		{Username: "risk-order-invitee-b1", AffCode: "risk-order-invitee-b1-aff", InviterId: inviters[1].Id},
		{Username: "risk-order-invitee-c1", AffCode: "risk-order-invitee-c1-aff", InviterId: inviters[2].Id},
		{Username: "risk-order-invitee-d1", AffCode: "risk-order-invitee-d1-aff", InviterId: inviters[3].Id},
		{Username: "risk-order-invitee-e1", AffCode: "risk-order-invitee-e1-aff", InviterId: inviters[4].Id},
	})
	require.EqualValues(t, 300, inviters[0].CreatedAt)
	require.EqualValues(t, 100, inviters[0].AffQuota)

	list := func(sortBy string, order string, startIdx int, limit int) ([]int, int64) {
		items, total, err := defaultListRiskInviters("", startIdx, limit, sortBy, order, "")
		require.NoError(t, err)
		ids := make([]int, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.InviterId)
		}
		return ids, total
	}

	idsAt := func(indexes ...int) []int {
		ids := make([]int, 0, len(indexes))
		for _, index := range indexes {
			ids = append(ids, inviters[index].Id)
		}
		return ids
	}

	// created_at asc / desc：C(200) 与 D(200) 并列时按 users.id ASC。
	ids, total := list("created_at", "asc", 0, 10)
	require.EqualValues(t, 5, total)
	require.Equal(t, idsAt(1, 2, 3, 0, 4), ids)

	ids, total = list("created_at", "desc", 0, 10)
	require.EqualValues(t, 5, total)
	require.Equal(t, idsAt(4, 0, 2, 3, 1), ids)

	// aff_quota asc / desc：A(100) 与 D(100) 并列时按 users.id ASC。
	ids, total = list("aff_quota", "asc", 0, 10)
	require.EqualValues(t, 5, total)
	require.Equal(t, idsAt(0, 3, 4, 2, 1), ids)

	ids, total = list("aff_quota", "desc", 0, 10)
	require.EqualValues(t, 5, total)
	require.Equal(t, idsAt(1, 2, 4, 0, 3), ids)

	// order 非法时回退 desc。
	ids, _ = list("created_at", "sideways", 0, 10)
	require.Equal(t, idsAt(4, 0, 2, 3, 1), ids)
	ids, _ = list("aff_quota", "sideways", 0, 10)
	require.Equal(t, idsAt(1, 2, 4, 0, 3), ids)

	// 稳定翻页：并列值跨页时两页拼接无重复、无丢失。
	fullOrder := idsAt(4, 0, 2, 3, 1)
	firstPage, total := list("created_at", "desc", 0, 2)
	require.EqualValues(t, 5, total)
	secondPage, total := list("created_at", "desc", 2, 2)
	require.EqualValues(t, 5, total)
	thirdPage, total := list("created_at", "desc", 4, 2)
	require.EqualValues(t, 5, total)
	require.Equal(t, idsAt(4, 0), firstPage)
	require.Equal(t, idsAt(2, 3), secondPage)
	require.Equal(t, idsAt(1), thirdPage)

	paged := append(append(append([]int{}, firstPage...), secondPage...), thirdPage...)
	assert.Equal(t, fullOrder, paged, "两页拼接应等于完整排序结果")

	seen := make(map[int]struct{}, len(paged))
	for _, id := range paged {
		_, duplicated := seen[id]
		assert.Falsef(t, duplicated, "翻页拼接出现重复邀请人 %d", id)
		seen[id] = struct{}{}
	}
	assert.Len(t, seen, len(fullOrder), "翻页拼接不应丢行")
}

// ---------------------------------------------------------------------------
// 第二阶段：邀请人被邀请人分组接口（A）
// ---------------------------------------------------------------------------

func findRiskInviteeGroup(groups []*RiskInviteeGroup, key string) *RiskInviteeGroup {
	for _, group := range groups {
		if group.Key == key {
			return group
		}
	}
	return nil
}

func TestListRiskInviteeGroupsByIP(t *testing.T) {
	setupRiskControlTest(t)
	inviter := createRiskUsers(t, []User{
		{Username: "risk-group-ip-inviter", AffCode: "risk-group-ip-inviter-aff"},
	})[0]
	invitees := createRiskUsers(t, []User{
		{Username: "risk-group-ip-a", AffCode: "risk-group-ip-a-aff", InviterId: inviter.Id, CreatedAt: 1000},
		{Username: "risk-group-ip-b", AffCode: "risk-group-ip-b-aff", InviterId: inviter.Id, Status: common.UserStatusDisabled, CreatedAt: 2000},
		{Username: "risk-group-ip-c", AffCode: "risk-group-ip-c-aff", InviterId: inviter.Id, CreatedAt: 3000},
		{Username: "risk-group-ip-d", AffCode: "risk-group-ip-d-aff", InviterId: inviter.Id, CreatedAt: 4000},
		{Username: "risk-group-ip-none", AffCode: "risk-group-ip-none-aff", InviterId: inviter.Id, CreatedAt: 5000},
	})
	records := []RiskIPRecord{
		// a：较早的另一 IP，应归入 last_seen 更大的 203.0.113.7。
		{UserId: invitees[0].Id, TokenId: 1, IP: "10.0.0.1", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: invitees[0].Id, TokenId: 2, IP: "203.0.113.7", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 500, EventCount: 1},
		{UserId: invitees[1].Id, TokenId: 1, IP: "203.0.113.7", Source: RiskIPSourceToken, FirstSeenAt: 150, LastSeenAt: 600, EventCount: 1},
		{UserId: invitees[2].Id, TokenId: 1, IP: "203.0.113.7", Source: RiskIPSourceLogin, FirstSeenAt: 200, LastSeenAt: 700, EventCount: 1},
		{UserId: invitees[3].Id, TokenId: 1, IP: "198.51.100.9", Source: RiskIPSourceLogin, FirstSeenAt: 300, LastSeenAt: 800, EventCount: 1},
	}
	require.NoError(t, DB.Create(&records).Error)

	result, err := ListRiskInviteeGroups(inviter.Id, "ip", 3600, 50, 200)
	require.NoError(t, err)
	require.Equal(t, inviter.Id, result.InviterId)
	require.Equal(t, RiskInviteeGroupByIP, result.GroupBy)
	require.EqualValues(t, 3600, result.TimeWindow)
	require.EqualValues(t, 5, result.TotalUsers)
	require.Equal(t, 5, result.LoadedUsers)
	require.False(t, result.Truncated)
	require.Equal(t, 3, result.GroupCount)
	require.False(t, result.GroupsTruncated)
	require.Len(t, result.Groups, 3)

	// 主组：203.0.113.7，3 人（1 个已封禁），最近 IP 归组且只计一次。
	mainGroup := result.Groups[0]
	require.Equal(t, "ip:203.0.113.7", mainGroup.Key)
	require.Equal(t, RiskInviteeGroupKindIP, mainGroup.Kind)
	require.Equal(t, "203.0.113.7", mainGroup.Label)
	require.EqualValues(t, 3, mainGroup.UserCount)
	require.EqualValues(t, 1, mainGroup.BannedCount)
	require.EqualValues(t, 100, mainGroup.FirstSeenAt)
	require.EqualValues(t, 700, mainGroup.LastSeenAt)
	require.ElementsMatch(t, []int{invitees[0].Id, invitees[1].Id, invitees[2].Id}, mainGroup.UserIds)
	require.Len(t, mainGroup.Users, 3)
	mainUsers := make(map[int]*RiskUserSummary, len(mainGroup.Users))
	for _, summary := range mainGroup.Users {
		mainUsers[summary.UserId] = summary
	}
	require.Equal(t, "203.0.113.7", mainUsers[invitees[0].Id].IP)
	require.EqualValues(t, 500, mainUsers[invitees[0].Id].LastSeenAt)
	require.EqualValues(t, 1000, mainUsers[invitees[0].Id].CreatedAt)

	singleGroup := result.Groups[1]
	require.Equal(t, "ip:198.51.100.9", singleGroup.Key)
	require.EqualValues(t, 1, singleGroup.UserCount)
	require.Equal(t, []int{invitees[3].Id}, singleGroup.UserIds)
	require.EqualValues(t, 300, singleGroup.FirstSeenAt)
	require.EqualValues(t, 800, singleGroup.LastSeenAt)

	// no_ip 组永远最后。
	noIPGroup := result.Groups[2]
	require.Equal(t, "no_ip", noIPGroup.Key)
	require.Equal(t, RiskInviteeGroupKindNoIP, noIPGroup.Kind)
	require.Empty(t, noIPGroup.Label)
	require.EqualValues(t, 1, noIPGroup.UserCount)
	require.Zero(t, noIPGroup.FirstSeenAt)
	require.Zero(t, noIPGroup.LastSeenAt)
	require.Equal(t, []int{invitees[4].Id}, noIPGroup.UserIds)
	require.Len(t, noIPGroup.Users, 1)
	require.Empty(t, noIPGroup.Users[0].IP)
	require.Zero(t, noIPGroup.Users[0].LastSeenAt)

	// group_users 限制只影响成员摘要，不影响完整 user_ids；limit_groups 截断。
	limited, err := ListRiskInviteeGroups(inviter.Id, "ip", 0, 2, 1)
	require.NoError(t, err)
	require.EqualValues(t, 3600, limited.TimeWindow, "非法 time_window 回退 3600")
	require.Equal(t, 1, limited.GroupCount)
	require.True(t, limited.GroupsTruncated)
	require.Len(t, limited.Groups, 1)
	require.Len(t, limited.Groups[0].Users, 2)
	require.Len(t, limited.Groups[0].UserIds, 3)

	// 非法 group_by 回退 ip。
	defaulted, err := ListRiskInviteeGroups(inviter.Id, "bogus", 0, 0, 0)
	require.NoError(t, err)
	require.Equal(t, RiskInviteeGroupByIP, defaulted.GroupBy)

	// inviterId 非法：空 groups、total_users=0、不报错。
	for _, invalidId := range []int{0, -7} {
		empty, err := ListRiskInviteeGroups(invalidId, "", 0, 0, 0)
		require.NoError(t, err)
		require.Zero(t, empty.TotalUsers)
		require.Zero(t, empty.LoadedUsers)
		require.Empty(t, empty.Groups)
		require.Equal(t, RiskInviteeGroupByIP, empty.GroupBy)
		require.NotNil(t, empty.Groups)
	}

	// 不存在的邀请人。
	empty, err := ListRiskInviteeGroups(inviter.Id+100000, "ip", 3600, 50, 200)
	require.NoError(t, err)
	require.Zero(t, empty.TotalUsers)
	require.Empty(t, empty.Groups)
}

func TestListRiskInviteeGroupsByTime(t *testing.T) {
	setupRiskControlTest(t)
	inviter := createRiskUsers(t, []User{
		{Username: "risk-group-time-inviter", AffCode: "risk-group-time-inviter-aff"},
	})[0]
	invitees := createRiskUsers(t, []User{
		{Username: "risk-group-time-a", AffCode: "risk-group-time-a-aff", InviterId: inviter.Id, CreatedAt: 100},
		{Username: "risk-group-time-b", AffCode: "risk-group-time-b-aff", InviterId: inviter.Id, CreatedAt: 150},
		{Username: "risk-group-time-c", AffCode: "risk-group-time-c-aff", InviterId: inviter.Id, CreatedAt: 3600},
		{Username: "risk-group-time-d", AffCode: "risk-group-time-d-aff", InviterId: inviter.Id, CreatedAt: 3700},
		{Username: "risk-group-time-e", AffCode: "risk-group-time-e-aff", InviterId: inviter.Id, CreatedAt: 7300},
	})
	// time 维度的组级 first/last seen 固定为 0，即使成员有 IP 记录。
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: invitees[0].Id, TokenId: 1, IP: "203.0.113.60", Source: RiskIPSourceToken, FirstSeenAt: 11, LastSeenAt: 22, EventCount: 1}).Error)

	result, err := ListRiskInviteeGroups(inviter.Id, "time", 3600, 50, 200)
	require.NoError(t, err)
	require.Equal(t, RiskInviteeGroupByTime, result.GroupBy)
	require.EqualValues(t, 3600, result.TimeWindow)
	require.EqualValues(t, 5, result.TotalUsers)
	require.Equal(t, 3, result.GroupCount)
	require.Len(t, result.Groups, 3)

	// 排序 user_count DESC, key ASC：bucket 0（2 人）→ 3600（2 人）→ 7200（1 人）。
	first := result.Groups[0]
	require.Equal(t, "time:0", first.Key)
	require.Equal(t, RiskInviteeGroupKindTime, first.Kind)
	require.Empty(t, first.Label)
	require.Zero(t, first.BucketStart)
	require.EqualValues(t, 2, first.UserCount)
	require.ElementsMatch(t, []int{invitees[0].Id, invitees[1].Id}, first.UserIds)
	require.Zero(t, first.FirstSeenAt)
	require.Zero(t, first.LastSeenAt)
	// 契约 A.3：group_by=time 只要求「组级」时间为 0；成员摘要的 last_seen_at 仍是
	// 该用户最近 IP 记录的真实值（A.5），无记录成员为 0。
	for _, summary := range first.Users {
		if summary.UserId == invitees[0].Id {
			require.Equal(t, "203.0.113.60", summary.IP)
			require.EqualValues(t, 22, summary.LastSeenAt)
			continue
		}
		require.Empty(t, summary.IP)
		require.Zero(t, summary.LastSeenAt)
	}
	require.Equal(t, "time:3600", result.Groups[1].Key)
	require.EqualValues(t, 3600, result.Groups[1].BucketStart)
	require.ElementsMatch(t, []int{invitees[2].Id, invitees[3].Id}, result.Groups[1].UserIds)
	require.Equal(t, "time:7200", result.Groups[2].Key)
	require.EqualValues(t, 7200, result.Groups[2].BucketStart)
	require.Equal(t, []int{invitees[4].Id}, result.Groups[2].UserIds)

	// 合法窗口 300：100/150 落在 0，3700 落在 3600。
	smallWindow, err := ListRiskInviteeGroups(inviter.Id, "time", 300, 50, 200)
	require.NoError(t, err)
	require.EqualValues(t, 300, smallWindow.TimeWindow)
	smallWindowGroup := findRiskInviteeGroup(smallWindow.Groups, "time:3600")
	require.NotNil(t, smallWindowGroup)
	require.ElementsMatch(t, []int{invitees[2].Id, invitees[3].Id}, smallWindowGroup.UserIds)

	// 非法窗口 1234 回退 3600。
	defaulted, err := ListRiskInviteeGroups(inviter.Id, "time", 1234, 50, 200)
	require.NoError(t, err)
	require.EqualValues(t, 3600, defaulted.TimeWindow)
}

func TestListRiskInviteeGroupsBySubnet(t *testing.T) {
	setupRiskControlTest(t)
	inviter := createRiskUsers(t, []User{
		{Username: "risk-group-subnet-inviter", AffCode: "risk-group-subnet-inviter-aff"},
	})[0]
	invitees := createRiskUsers(t, []User{
		{Username: "risk-group-subnet-a", AffCode: "risk-group-subnet-a-aff", InviterId: inviter.Id},
		{Username: "risk-group-subnet-b", AffCode: "risk-group-subnet-b-aff", InviterId: inviter.Id},
		{Username: "risk-group-subnet-c", AffCode: "risk-group-subnet-c-aff", InviterId: inviter.Id},
		{Username: "risk-group-subnet-d", AffCode: "risk-group-subnet-d-aff", InviterId: inviter.Id},
		{Username: "risk-group-subnet-none", AffCode: "risk-group-subnet-none-aff", InviterId: inviter.Id},
		{Username: "risk-group-subnet-invalid", AffCode: "risk-group-subnet-invalid-aff", InviterId: inviter.Id},
	})
	records := []RiskIPRecord{
		{UserId: invitees[0].Id, TokenId: 1, IP: "203.0.113.7", Source: RiskIPSourceToken, FirstSeenAt: 100, LastSeenAt: 500, EventCount: 1},
		{UserId: invitees[1].Id, TokenId: 1, IP: "203.0.113.8", Source: RiskIPSourceToken, FirstSeenAt: 150, LastSeenAt: 600, EventCount: 1},
		{UserId: invitees[2].Id, TokenId: 1, IP: "2001:db8:1:2::1", Source: RiskIPSourceToken, FirstSeenAt: 200, LastSeenAt: 700, EventCount: 1},
		{UserId: invitees[3].Id, TokenId: 1, IP: "2001:db8:1:2:abcd:ef01:2345:6789", Source: RiskIPSourceToken, FirstSeenAt: 250, LastSeenAt: 800, EventCount: 1},
		// 非 IP 文本：归入 no_ip。
		{UserId: invitees[5].Id, TokenId: 1, IP: "not-an-ip", Source: RiskIPSourceToken, FirstSeenAt: 300, LastSeenAt: 900, EventCount: 1},
	}
	require.NoError(t, DB.Create(&records).Error)

	result, err := ListRiskInviteeGroups(inviter.Id, "subnet", 3600, 50, 200)
	require.NoError(t, err)
	require.Equal(t, RiskInviteeGroupBySubnet, result.GroupBy)
	require.EqualValues(t, 6, result.TotalUsers)
	require.Equal(t, 3, result.GroupCount)
	require.Len(t, result.Groups, 3)

	// 两个 2 人网段按 key ASC：2001:... < 203.0...；no_ip 最后。
	ipv6Group := result.Groups[0]
	require.Equal(t, "subnet:2001:db8:1:2::/64", ipv6Group.Key)
	require.Equal(t, RiskInviteeGroupKindSubnet, ipv6Group.Kind)
	require.Equal(t, "2001:db8:1:2::/64", ipv6Group.Label)
	require.EqualValues(t, 2, ipv6Group.UserCount)
	require.EqualValues(t, 200, ipv6Group.FirstSeenAt)
	require.EqualValues(t, 800, ipv6Group.LastSeenAt)
	require.ElementsMatch(t, []int{invitees[2].Id, invitees[3].Id}, ipv6Group.UserIds)

	ipv4Group := result.Groups[1]
	require.Equal(t, "subnet:203.0.113.0/24", ipv4Group.Key)
	require.Equal(t, "203.0.113.0/24", ipv4Group.Label)
	require.EqualValues(t, 2, ipv4Group.UserCount)
	require.EqualValues(t, 100, ipv4Group.FirstSeenAt)
	require.EqualValues(t, 600, ipv4Group.LastSeenAt)
	require.ElementsMatch(t, []int{invitees[0].Id, invitees[1].Id}, ipv4Group.UserIds)

	noIPGroup := result.Groups[2]
	require.Equal(t, "no_ip", noIPGroup.Key)
	require.Equal(t, RiskInviteeGroupKindNoIP, noIPGroup.Kind)
	require.EqualValues(t, 2, noIPGroup.UserCount)
	require.ElementsMatch(t, []int{invitees[4].Id, invitees[5].Id}, noIPGroup.UserIds)
	require.Zero(t, noIPGroup.FirstSeenAt)
	require.Zero(t, noIPGroup.LastSeenAt)
}

// ---------------------------------------------------------------------------
// 第二阶段：同 IP 列表网段聚合（B）
// ---------------------------------------------------------------------------

func TestListRiskSharedIPsSubnetAggregation(t *testing.T) {
	setupRiskControlTest(t)
	users := createRiskUsers(t, []User{
		{Username: "risk-subnet-a", AffCode: "risk-subnet-a-aff"},
		{Username: "risk-subnet-b", AffCode: "risk-subnet-b-aff"},
		{Username: "risk-subnet-c", AffCode: "risk-subnet-c-aff"},
		{Username: "risk-subnet-d", AffCode: "risk-subnet-d-aff"},
		{Username: "risk-subnet-e", AffCode: "risk-subnet-e-aff"},
		{Username: "risk-subnet-singleton", AffCode: "risk-subnet-singleton-aff"},
		{Username: "risk-subnet-g", AffCode: "risk-subnet-g-aff"},
		{Username: "risk-subnet-h", AffCode: "risk-subnet-h-aff"},
	})
	records := []RiskIPRecord{
		{UserId: users[0].Id, TokenId: 1, IP: "203.0.113.7", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: users[1].Id, TokenId: 1, IP: "203.0.113.7", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: users[1].Id, TokenId: 2, IP: "203.0.113.8", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: users[2].Id, TokenId: 1, IP: "203.0.113.8", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: users[3].Id, TokenId: 1, IP: "198.51.100.9", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: users[4].Id, TokenId: 1, IP: "198.51.100.9", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		// 单账号 IP：不进入 min_users=2 的列表，也不应被计入当前页网段统计。
		{UserId: users[5].Id, TokenId: 1, IP: "203.0.113.99", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: users[6].Id, TokenId: 1, IP: "2001:db8:1:2::1", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: users[7].Id, TokenId: 1, IP: "2001:db8:1:2::1", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
	}
	require.NoError(t, DB.Create(&records).Error)

	items, total, err := ListRiskSharedIPs(RiskIPSourceToken, "ip", "", 2, 0, 10, "ip", "asc", "", false)
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	require.Len(t, items, 4)
	require.Equal(t, []string{"198.51.100.9", "2001:db8:1:2::1", "203.0.113.7", "203.0.113.8"}, riskSharedIPIPs(items))

	require.Equal(t, "198.51.100.0/24", items[0].Subnet)
	require.EqualValues(t, 2, items[0].SubnetUserCount)
	require.Equal(t, "2001:db8:1:2::/64", items[1].Subnet)
	require.EqualValues(t, 2, items[1].SubnetUserCount)

	// 同网段两个 IP 组：去重账号数 3（a/b/c），大于本行 user_count 2。
	require.Equal(t, "203.0.113.0/24", items[2].Subnet)
	require.EqualValues(t, 2, items[2].UserCount)
	require.EqualValues(t, 3, items[2].SubnetUserCount)
	require.Equal(t, "203.0.113.0/24", items[3].Subnet)
	require.EqualValues(t, 3, items[3].SubnetUserCount)
}

// ---------------------------------------------------------------------------
// 第二阶段：搜索补强（C.1-C.3）
// ---------------------------------------------------------------------------

func TestListRiskSharedIPsSearchesByEmail(t *testing.T) {
	setupRiskControlTest(t)
	users := createRiskUsers(t, []User{
		{Username: "risk-email-alice", Email: "alice@corp.example.com", AffCode: "risk-email-alice-aff"},
		{Username: "risk-email-bob", Email: "bob@corp.example.com", AffCode: "risk-email-bob-aff"},
		{Username: "risk-email-carol", Email: "carol@other.example.com", AffCode: "risk-email-carol-aff"},
		{Username: "risk-email-dave", Email: "dave@other.example.com", AffCode: "risk-email-dave-aff"},
	})
	records := []RiskIPRecord{
		{UserId: users[0].Id, TokenId: 1, IP: "203.0.113.10", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: users[1].Id, TokenId: 1, IP: "203.0.113.10", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: users[2].Id, TokenId: 1, IP: "203.0.113.11", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: users[3].Id, TokenId: 1, IP: "203.0.113.11", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
	}
	require.NoError(t, DB.Create(&records).Error)

	list := func(searchType string, keyword string) ([]string, int64) {
		items, total, err := ListRiskSharedIPs(RiskIPSourceToken, searchType, keyword, 2, 0, 10, "ip", "asc", "", false)
		require.NoError(t, err)
		return riskSharedIPIPs(items), total
	}

	// search_type=email：只匹配邮箱。
	ips, total := list("email", "alice@")
	require.EqualValues(t, 1, total)
	require.Equal(t, []string{"203.0.113.10"}, ips)
	ips, total = list("email", "corp.example.com")
	require.EqualValues(t, 1, total)
	require.Equal(t, []string{"203.0.113.10"}, ips)
	ips, total = list("email", "nobody@")
	require.EqualValues(t, 0, total)
	require.Empty(t, ips)

	// C.1：username 分支同样命中邮箱。
	ips, total = list("username", "alice@")
	require.EqualValues(t, 1, total)
	require.Equal(t, []string{"203.0.113.10"}, ips)
}

func TestListRiskInvitersSearchesByIP(t *testing.T) {
	setupRiskControlTest(t)
	inviterA := createRiskUsers(t, []User{
		{Username: "risk-ipsearch-inviter-a", AffCode: "risk-ipsearch-inviter-a-aff"},
	})[0]
	inviterB := createRiskUsers(t, []User{
		{Username: "risk-ipsearch-inviter-b", AffCode: "risk-ipsearch-inviter-b-aff"},
	})[0]
	inviterC := createRiskUsers(t, []User{
		{Username: "risk-ipsearch-inviter-c", AffCode: "risk-ipsearch-inviter-c-aff"},
	})[0]
	invitees := createRiskUsers(t, []User{
		{Username: "risk-ipsearch-invitee-a", AffCode: "risk-ipsearch-invitee-a-aff", InviterId: inviterA.Id},
		{Username: "risk-ipsearch-invitee-b", AffCode: "risk-ipsearch-invitee-b-aff", InviterId: inviterB.Id},
		{Username: "risk-ipsearch-invitee-c", AffCode: "risk-ipsearch-invitee-c-aff", InviterId: inviterC.Id},
	})
	records := []RiskIPRecord{
		// A 的被邀请人用过 203.0.113.5。
		{UserId: invitees[0].Id, TokenId: 1, IP: "203.0.113.5", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		// B 的被邀请人用过 198.51.100.5。
		{UserId: invitees[1].Id, TokenId: 1, IP: "198.51.100.5", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		// C 本人用过 203.0.113.7；其被邀请人用别的 IP。
		{UserId: inviterC.Id, TokenId: 1, IP: "203.0.113.7", Source: RiskIPSourceLogin, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: invitees[2].Id, TokenId: 1, IP: "10.0.0.9", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
	}
	require.NoError(t, DB.Create(&records).Error)

	list := func(keyword string, searchType string) ([]*RiskInviter, int64) {
		items, total, err := listRiskInvitersPhase2Defaults(keyword, 0, 10, "inviter_id", "asc", "", searchType, false, 0, "")
		require.NoError(t, err)
		return items, total
	}

	// 前缀匹配：A（被邀请人用过）+ C（本人用过）。
	items, total := list("203.0.113", "ip")
	require.EqualValues(t, 2, total)
	require.Len(t, items, 2)
	require.Equal(t, inviterA.Id, items[0].InviterId)
	require.Equal(t, inviterC.Id, items[1].InviterId)
	require.Len(t, items[0].MatchedInvitees, 1)
	require.Equal(t, invitees[0].Id, items[0].MatchedInvitees[0].UserId)
	require.NotNil(t, items[1].MatchedInvitees)
	require.Empty(t, items[1].MatchedInvitees, "仅邀请人本人命中时 matched_invitees 为空")

	// 精确 IP。
	items, total = list("203.0.113.5", "ip")
	require.EqualValues(t, 1, total)
	require.Equal(t, inviterA.Id, items[0].InviterId)

	// 另一前缀只命中 B。
	items, total = list("198.51.100", "ip")
	require.EqualValues(t, 1, total)
	require.Equal(t, inviterB.Id, items[0].InviterId)

	// 无命中。
	items, total = list("203.0.113.99", "ip")
	require.Zero(t, total)
	require.Empty(t, items)

	// 空关键词不筛选。
	_, total = list("", "ip")
	require.EqualValues(t, 3, total)

	// 非法 search_type 回退 any：按用户字段匹配，IP 关键词不命中。
	items, total = list("203.0.113", "bogus")
	require.Zero(t, total)
	require.Empty(t, items)
}

// ---------------------------------------------------------------------------
// 第二阶段：新增筛选参数（C.4）
// ---------------------------------------------------------------------------

func TestListRiskSharedIPsFiltersActive24h(t *testing.T) {
	setupRiskControlTest(t)
	users := createRiskUsers(t, []User{
		{Username: "risk-active-a", AffCode: "risk-active-a-aff"},
		{Username: "risk-active-b", AffCode: "risk-active-b-aff"},
		{Username: "risk-stale-a", AffCode: "risk-stale-a-aff"},
		{Username: "risk-stale-b", AffCode: "risk-stale-b-aff"},
	})
	now := time.Now().Unix()
	records := []RiskIPRecord{
		{UserId: users[0].Id, TokenId: 1, IP: "203.0.113.30", Source: RiskIPSourceToken, FirstSeenAt: now - 7200, LastSeenAt: now - 60, EventCount: 1},
		{UserId: users[1].Id, TokenId: 1, IP: "203.0.113.30", Source: RiskIPSourceToken, FirstSeenAt: now - 7200, LastSeenAt: now - 60, EventCount: 1},
		{UserId: users[2].Id, TokenId: 1, IP: "203.0.113.31", Source: RiskIPSourceToken, FirstSeenAt: now - 300000, LastSeenAt: now - 200000, EventCount: 1},
		{UserId: users[3].Id, TokenId: 1, IP: "203.0.113.31", Source: RiskIPSourceToken, FirstSeenAt: now - 300000, LastSeenAt: now - 200000, EventCount: 1},
	}
	require.NoError(t, DB.Create(&records).Error)

	activeOnly, total, err := ListRiskSharedIPs(RiskIPSourceToken, "ip", "", 2, 0, 10, "ip", "asc", "", true)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, activeOnly, 1)
	require.Equal(t, "203.0.113.30", activeOnly[0].IP)

	all, total, err := ListRiskSharedIPs(RiskIPSourceToken, "ip", "", 2, 0, 10, "ip", "asc", "", false)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, all, 2)
}

func TestListRiskInvitersFiltersSuspiciousOnly(t *testing.T) {
	setupRiskControlTest(t)
	inviters := createRiskUsers(t, []User{
		{Username: "risk-susp-inviter-clean", AffCode: "risk-susp-inviter-clean-aff"},
		// 回归用例：邀请人本人没有任何 IP 记录，但其被邀请人之间共享 IP。
		{Username: "risk-susp-inviter-invitees", AffCode: "risk-susp-inviter-invitees-aff"},
		// 邀请人本人与正常账号共享 IP。
		{Username: "risk-susp-inviter-self", AffCode: "risk-susp-inviter-self-aff"},
		// 被邀请人与「已删除」账号共享 IP → 不算。
		{Username: "risk-susp-inviter-deleted-peer", AffCode: "risk-susp-delpeer-aff"},
		// 被邀请人与「无关的正常账号」共享 IP → 算。
		{Username: "risk-susp-inviter-external-live", AffCode: "risk-susp-extlive-aff"},
		// 邀请人本人与「已删除」账号共享 IP → 不算。
		{Username: "risk-susp-inviter-self-deleted", AffCode: "risk-susp-selfdel-aff"},
	})
	invitees := createRiskUsers(t, []User{
		{Username: "risk-susp-clean-invitee", AffCode: "risk-susp-clean-invitee-aff", InviterId: inviters[0].Id},
		{Username: "risk-susp-invitees-a", AffCode: "risk-susp-invitees-a-aff", InviterId: inviters[1].Id},
		{Username: "risk-susp-invitees-b", AffCode: "risk-susp-invitees-b-aff", InviterId: inviters[1].Id},
		{Username: "risk-susp-self-invitee", AffCode: "risk-susp-self-invitee-aff", InviterId: inviters[2].Id},
		{Username: "risk-susp-deleted-invitee", AffCode: "risk-susp-deleted-invitee-aff", InviterId: inviters[3].Id},
		{Username: "risk-susp-external-invitee", AffCode: "risk-susp-external-invitee-aff", InviterId: inviters[4].Id},
		{Username: "risk-susp-selfdeleted-invitee", AffCode: "risk-susp-selfdel-inv-aff", InviterId: inviters[5].Id},
	})
	externals := createRiskUsers(t, []User{
		{Username: "risk-susp-external-live", AffCode: "risk-susp-external-live-aff"},
		{Username: "risk-susp-external-deleted", AffCode: "risk-susp-external-deleted-aff"},
	})
	liveExternal, deletedExternal := externals[0], externals[1]
	records := []RiskIPRecord{
		// i1：邀请人本人无记录；两个被邀请人互相共享 203.0.113.7。
		{UserId: invitees[1].Id, TokenId: 1, IP: "203.0.113.7", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: invitees[2].Id, TokenId: 1, IP: "203.0.113.7", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		// i2：邀请人本人与无关正常账号共享。
		{UserId: inviters[2].Id, TokenId: 1, IP: "198.51.100.50", Source: RiskIPSourceLogin, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: liveExternal.Id, TokenId: 1, IP: "198.51.100.50", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		// i3：被邀请人与「已删除」账号共享 → 不应命中。
		{UserId: invitees[4].Id, TokenId: 1, IP: "203.0.113.80", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: deletedExternal.Id, TokenId: 2, IP: "203.0.113.80", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		// i4：被邀请人与无关正常账号共享 → 命中。
		{UserId: invitees[5].Id, TokenId: 1, IP: "203.0.113.90", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: liveExternal.Id, TokenId: 2, IP: "203.0.113.90", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		// i5：邀请人本人与「已删除」账号共享 → 不应命中。
		{UserId: inviters[5].Id, TokenId: 1, IP: "203.0.113.95", Source: RiskIPSourceLogin, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: deletedExternal.Id, TokenId: 3, IP: "203.0.113.95", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
	}
	require.NoError(t, DB.Create(&records).Error)
	// 软删除一个共享对端账号：deleted_at IS NULL 过滤应使其不再造成「需复核」。
	require.NoError(t, DB.Delete(&User{}, deletedExternal.Id).Error)

	list := func(suspiciousOnly bool, startIdx int, limit int) ([]*RiskInviter, int64) {
		items, total, err := listRiskInvitersPhase2Defaults("", startIdx, limit, "inviter_id", "asc", "", "any", suspiciousOnly, 0, "")
		require.NoError(t, err)
		return items, total
	}

	// 不筛选：6 个邀请人。
	items, total := list(false, 0, 10)
	require.EqualValues(t, 6, total)
	require.Len(t, items, 6)

	// suspicious_only：i1（仅被邀请人共享，本次 bug 回归）、i2（本人共享）、i4（被邀请人与
	// 无关正常账号共享）命中；i0 无共享、i3/i5 的共享对端已删除 → 不命中。
	items, total = list(true, 0, 10)
	require.EqualValues(t, 3, total)
	require.Len(t, items, 3)
	require.Equal(t, inviters[1].Id, items[0].InviterId, "邀请人本人无 IP 记录、被邀请人共享时应命中")
	require.Equal(t, inviters[2].Id, items[1].InviterId, "邀请人本人共享时应命中")
	require.Equal(t, inviters[4].Id, items[2].InviterId, "被邀请人与无关正常账号共享时应命中")

	// 关键：筛选在 SQL 侧、分页之前完成。limit=1 逐页取应直接跳过不匹配的邀请人。
	for page, expected := range []int{inviters[1].Id, inviters[2].Id, inviters[4].Id} {
		pageItems, pageTotal := list(true, page, 1)
		require.EqualValues(t, 3, pageTotal, "每页 total 都应是筛选后的真实总数")
		require.Len(t, pageItems, 1)
		require.Equal(t, expected, pageItems[0].InviterId)
	}
	emptyPage, emptyTotal := list(true, 3, 1)
	require.EqualValues(t, 3, emptyTotal)
	require.Empty(t, emptyPage)
}

func TestListRiskInvitersFiltersMinInvites(t *testing.T) {
	setupRiskControlTest(t)
	inviters := createRiskUsers(t, []User{
		{Username: "risk-mininv-one", AffCode: "risk-mininv-one-aff"},
		{Username: "risk-mininv-two", AffCode: "risk-mininv-two-aff"},
		{Username: "risk-mininv-three", AffCode: "risk-mininv-three-aff"},
	})
	inviteeCounts := []int{1, 2, 3}
	for index, count := range inviteeCounts {
		users := make([]User, 0, count)
		for sub := 0; sub < count; sub++ {
			users = append(users, User{
				Username:  fmt.Sprintf("risk-mininv-%d-%d", index, sub),
				AffCode:   fmt.Sprintf("risk-mininv-%d-%d-aff", index, sub),
				InviterId: inviters[index].Id,
			})
		}
		createRiskUsers(t, users)
	}

	list := func(minInvites int) ([]*RiskInviter, int64) {
		items, total, err := listRiskInvitersPhase2Defaults("", 0, 10, "direct_invite_count", "desc", "", "any", false, minInvites, "")
		require.NoError(t, err)
		return items, total
	}

	items, total := list(0)
	require.EqualValues(t, 3, total)
	require.EqualValues(t, 3, items[0].DirectInviteCount)

	// min_invites=2 只保留 2/3 人组。
	items, total = list(2)
	require.EqualValues(t, 2, total)
	require.Len(t, items, 2)
	require.EqualValues(t, 3, items[0].DirectInviteCount)
	require.EqualValues(t, 2, items[1].DirectInviteCount)

	// <=1 视为不筛选。
	for _, noFilter := range []int{-1, 0, 1} {
		_, total = list(noFilter)
		require.EqualValues(t, 3, total)
	}

	_, total = list(10)
	require.Zero(t, total)
}

func TestListRiskInvitersFiltersInviterStatus(t *testing.T) {
	setupRiskControlTest(t)
	inviters := createRiskUsers(t, []User{
		{Username: "risk-status-enabled", AffCode: "risk-status-enabled-aff"},
		{Username: "risk-status-disabled", AffCode: "risk-status-disabled-aff", Status: common.UserStatusDisabled},
	})
	createRiskUsers(t, []User{
		{Username: "risk-status-invitee-enabled", AffCode: "risk-status-invitee-enabled-aff", InviterId: inviters[0].Id},
		{Username: "risk-status-invitee-disabled", AffCode: "risk-status-invitee-disabled-aff", InviterId: inviters[1].Id},
	})

	list := func(inviterStatus string) ([]*RiskInviter, int64) {
		items, total, err := listRiskInvitersPhase2Defaults("", 0, 10, "inviter_id", "asc", "", "any", false, 0, inviterStatus)
		require.NoError(t, err)
		return items, total
	}

	items, total := list("disabled")
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviters[1].Id, items[0].InviterId)

	for _, noFilter := range []string{"", "enabled", "DISABLED", "bogus"} {
		_, total = list(noFilter)
		require.EqualValues(t, 2, total, "非法 inviter_status 不应筛选: %q", noFilter)
	}
}

// ---------------------------------------------------------------------------
// 第二阶段：平铺被邀请人列表补充 created_at / ip / last_seen_at（A.5）
// ---------------------------------------------------------------------------

func TestListRiskInviteesIncludesIPAndCreatedAt(t *testing.T) {
	setupRiskControlTest(t)
	inviter := createRiskUsers(t, []User{
		{Username: "risk-flat-inviter", AffCode: "risk-flat-inviter-aff"},
	})[0]
	invitees := createRiskUsers(t, []User{
		{Username: "risk-flat-with-ip", AffCode: "risk-flat-with-ip-aff", InviterId: inviter.Id, CreatedAt: 1234},
		{Username: "risk-flat-no-ip", AffCode: "risk-flat-no-ip-aff", InviterId: inviter.Id, CreatedAt: 5678},
	})
	require.NoError(t, DB.Create(&RiskIPRecord{UserId: invitees[0].Id, TokenId: 1, IP: "203.0.113.44", Source: RiskIPSourceToken, FirstSeenAt: 10, LastSeenAt: 20, EventCount: 1}).Error)

	items, total, err := ListRiskInvitees(inviter.Id, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, items, 2)
	// 按 user id DESC：无 IP 的账号创建更晚，排第一。
	require.Equal(t, invitees[1].Id, items[0].UserId)
	require.Empty(t, items[0].IP)
	require.Zero(t, items[0].LastSeenAt)
	require.EqualValues(t, 5678, items[0].CreatedAt)
	require.Equal(t, invitees[0].Id, items[1].UserId)
	require.Equal(t, "203.0.113.44", items[1].IP)
	require.EqualValues(t, 20, items[1].LastSeenAt)
	require.EqualValues(t, 1234, items[1].CreatedAt)
}

// ---------------------------------------------------------------------------
// 第二阶段：分组接口上限（group_users 上限 50、加载 2000 截断）
// ---------------------------------------------------------------------------

func TestListRiskInviteeGroupsLimits(t *testing.T) {
	setupRiskControlTest(t)
	inviter := createRiskUsers(t, []User{
		{Username: "risk-grouplimit-inviter", AffCode: "risk-grouplimit-inviter-aff"},
	})[0]
	groupAUsers := make([]User, 51)
	for index := range groupAUsers {
		groupAUsers[index] = User{
			Username:  fmt.Sprintf("risk-grouplimit-a-%02d", index),
			AffCode:   fmt.Sprintf("risk-grouplimit-a-%02d-aff", index),
			InviterId: inviter.Id,
		}
	}
	groupAUsers = createRiskUsers(t, groupAUsers)
	groupBUsers := createRiskUsers(t, []User{
		{Username: "risk-grouplimit-b-1", AffCode: "risk-grouplimit-b-1-aff", InviterId: inviter.Id},
		{Username: "risk-grouplimit-b-2", AffCode: "risk-grouplimit-b-2-aff", InviterId: inviter.Id},
	})
	records := make([]RiskIPRecord, 0, len(groupAUsers)+len(groupBUsers))
	for index, user := range groupAUsers {
		records = append(records, RiskIPRecord{UserId: user.Id, TokenId: index + 1, IP: "203.0.113.70", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1})
	}
	for index, user := range groupBUsers {
		records = append(records, RiskIPRecord{UserId: user.Id, TokenId: index + 1, IP: "203.0.113.71", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1})
	}
	require.NoError(t, DB.Create(&records).Error)

	result, err := ListRiskInviteeGroups(inviter.Id, "ip", 3600, 999, 200)
	require.NoError(t, err)
	require.EqualValues(t, 53, result.TotalUsers)
	require.Len(t, result.Groups, 2)
	require.EqualValues(t, 51, result.Groups[0].UserCount)
	require.Len(t, result.Groups[0].UserIds, 51)
	require.Len(t, result.Groups[0].Users, riskInviteeGroupMaxGroupUsers, "group_users 超过上限按 50 处理")

	limited, err := ListRiskInviteeGroups(inviter.Id, "ip", 3600, 50, 1)
	require.NoError(t, err)
	require.Equal(t, 1, limited.GroupCount)
	require.True(t, limited.GroupsTruncated)
}

func TestListRiskInviteeGroupsTruncatesAt2000Users(t *testing.T) {
	setupRiskControlTest(t)
	inviter := createRiskUsers(t, []User{
		{Username: "risk-grouptop-inviter", AffCode: "risk-grouptop-inviter-aff"},
	})[0]
	total := riskInviteeGroupMaxLoadedUsers + 1
	users := make([]User, total)
	for index := range users {
		users[index] = User{
			Username:  fmt.Sprintf("risk-grouptop-%04d", index),
			AffCode:   fmt.Sprintf("risk-grouptop-%04d-aff", index),
			InviterId: inviter.Id,
		}
		riskTestAssertUserColumnLimits(t, users[index])
	}
	require.NoError(t, DB.CreateInBatches(&users, 200).Error)

	result, err := ListRiskInviteeGroups(inviter.Id, "time", 3600, 50, 200)
	require.NoError(t, err)
	require.EqualValues(t, total, result.TotalUsers)
	require.Equal(t, riskInviteeGroupMaxLoadedUsers, result.LoadedUsers)
	require.True(t, result.Truncated)
	require.Len(t, result.Groups, 1)
	require.EqualValues(t, riskInviteeGroupMaxLoadedUsers, result.Groups[0].UserCount)
	require.Len(t, result.Groups[0].UserIds, riskInviteeGroupMaxLoadedUsers)
	require.False(t, result.Groups[0].UserIdsTruncated)
}

// ---------------------------------------------------------------------------
// 第三阶段 D：include_inviter
// ---------------------------------------------------------------------------

func TestBuildRiskInviterBatchUserIds(t *testing.T) {
	inviteeIds := []int{12, 13, 12, 0, -1}
	// 默认不包含邀请人本人，行为与旧版一致（去重 + 丢弃非法 id）。
	require.Equal(t, []int{12, 13}, BuildRiskInviterBatchUserIds(99, inviteeIds, false))
	// include_inviter=true：邀请人本人放最前。
	require.Equal(t, []int{99, 12, 13}, BuildRiskInviterBatchUserIds(99, inviteeIds, true))
	// 邀请人本人与列表重复时只保留最前一个。
	require.Equal(t, []int{99, 12}, BuildRiskInviterBatchUserIds(99, []int{99, 12}, true))
	// 无被邀请人时仍可只处理邀请人本人。
	require.Equal(t, []int{99}, BuildRiskInviterBatchUserIds(99, nil, true))
	// 非法邀请人 id 被丢弃。
	require.Equal(t, []int{12}, BuildRiskInviterBatchUserIds(0, []int{12}, true))
}

func TestBatchUpdateRiskInviterScopeIncludeInviter(t *testing.T) {
	setupRiskControlTest(t)
	inviters := createRiskUsers(t, []User{
		{Username: "risk-inc-admin-inviter", AffCode: "risk-inc-admin-aff", Role: common.RoleAdminUser},
		{Username: "risk-inc-root-inviter", AffCode: "risk-inc-root-aff", Role: common.RoleRootUser},
		{Username: "risk-inc-common-inviter", AffCode: "risk-inc-common-aff"},
		{Username: "risk-inc-rootop-inviter", AffCode: "risk-inc-rootop-aff"},
	})
	invitees := createRiskUsers(t, []User{
		{Username: "risk-inc-admin-invitee", AffCode: "risk-inc-admin-invitee-aff", InviterId: inviters[0].Id},
		{Username: "risk-inc-root-invitee", AffCode: "risk-inc-root-invitee-aff", InviterId: inviters[1].Id},
		{Username: "risk-inc-common-invitee", AffCode: "risk-inc-common-invitee-aff", InviterId: inviters[2].Id},
		{Username: "risk-inc-rootop-invitee", AffCode: "risk-inc-rootop-invitee-aff", InviterId: inviters[3].Id},
	})
	reloadStatus := func(userId int) int {
		var user User
		require.NoError(t, DB.Select("id, status").First(&user, userId).Error)
		return user.Status
	}
	runInviterScope := func(inviterId int, includeInviter bool, operatorRole int) (int, []RiskStatusSkip) {
		inviteeIds, err := ResolveRiskUserIDsByInviter(inviterId)
		require.NoError(t, err)
		userIds := BuildRiskInviterBatchUserIds(inviterId, inviteeIds, includeInviter)
		updated, skipped, err := BatchUpdateRiskUserStatus(operatorRole, "disable", userIds)
		require.NoError(t, err)
		return updated, skipped
	}

	// admin 操作者 + include_inviter=true：邀请人本人是同级 admin → skipped，被邀请人成功。
	updated, skipped := runInviterScope(inviters[0].Id, true, common.RoleAdminUser)
	require.Equal(t, 1, updated)
	require.Len(t, skipped, 1)
	require.Equal(t, RiskStatusSkipInsufficientPrivilege, riskStatusSkipReason(t, skipped, inviters[0].Id))
	require.Equal(t, common.UserStatusEnabled, reloadStatus(inviters[0].Id))
	require.Equal(t, common.UserStatusDisabled, reloadStatus(invitees[0].Id))

	// include_inviter=true + 邀请人是 root → root_user skipped，被邀请人成功。
	updated, skipped = runInviterScope(inviters[1].Id, true, common.RoleAdminUser)
	require.Equal(t, 1, updated)
	require.Len(t, skipped, 1)
	require.Equal(t, RiskStatusSkipRootUser, riskStatusSkipReason(t, skipped, inviters[1].Id))
	require.Equal(t, common.UserStatusEnabled, reloadStatus(inviters[1].Id))
	require.Equal(t, common.UserStatusDisabled, reloadStatus(invitees[1].Id))

	// include_inviter=false：只处理被邀请人，邀请人本人完全不参与（不在 skipped）。
	updated, skipped = runInviterScope(inviters[2].Id, false, common.RoleAdminUser)
	require.Equal(t, 1, updated)
	require.Empty(t, skipped)
	require.Equal(t, common.UserStatusEnabled, reloadStatus(inviters[2].Id))
	require.Equal(t, common.UserStatusDisabled, reloadStatus(invitees[2].Id))

	// root 操作者 + include_inviter=true：普通邀请人本人也会被禁用。
	updated, skipped = runInviterScope(inviters[3].Id, true, common.RoleRootUser)
	require.Equal(t, 2, updated)
	require.Empty(t, skipped)
	require.Equal(t, common.UserStatusDisabled, reloadStatus(inviters[3].Id))
	require.Equal(t, common.UserStatusDisabled, reloadStatus(invitees[3].Id))
}

// ---------------------------------------------------------------------------
// 第三阶段 E：shared_ip_count 筛选 / 排序 / 最近邀请窗口
// ---------------------------------------------------------------------------

// riskUserSummaryMap 把摘要列表转成 user_id → summary 的 map，便于按用户断言。
func riskUserSummaryMap(summaries []*RiskUserSummary) map[int]*RiskUserSummary {
	result := make(map[int]*RiskUserSummary, len(summaries))
	for _, summary := range summaries {
		if summary != nil {
			result[summary.UserId] = summary
		}
	}
	return result
}

// inviteeIdByName 按用户名查询测试用户 id（测试数据辅助）。
func inviteeIdByName(t *testing.T, username string) int {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("id").Where("username = ?", username).First(&user).Error)
	require.Positive(t, user.Id)
	return user.Id
}

func TestListRiskInvitersFiltersMinSharedIPsAndSharedCountConsistency(t *testing.T) {
	setupRiskControlTest(t)
	inviters := createRiskUsers(t, []User{
		{Username: "risk-sharedcnt-inviter-2", AffCode: "risk-sharedcnt-inv-2-aff"},
		{Username: "risk-sharedcnt-inviter-1", AffCode: "risk-sharedcnt-inv-1-aff"},
		{Username: "risk-sharedcnt-inviter-0", AffCode: "risk-sharedcnt-inv-0-aff"},
	})
	createRiskUsers(t, []User{
		{Username: "risk-sharedcnt-a1", AffCode: "risk-sharedcnt-a1-aff", InviterId: inviters[0].Id},
		{Username: "risk-sharedcnt-a2", AffCode: "risk-sharedcnt-a2-aff", InviterId: inviters[0].Id},
		{Username: "risk-sharedcnt-b1", AffCode: "risk-sharedcnt-b1-aff", InviterId: inviters[1].Id},
		{Username: "risk-sharedcnt-c1", AffCode: "risk-sharedcnt-c1-aff", InviterId: inviters[2].Id},
	})
	// 外部正常账号：分别与两个邀请人的被邀请人共享不同 IP。
	external := createRiskUsers(t, []User{
		{Username: "risk-sharedcnt-external", AffCode: "risk-sharedcnt-ext-aff"},
	})[0]
	records := []RiskIPRecord{
		// inviter-2 的被邀请人共享 2 个 IP。
		{UserId: inviteeIdByName(t, "risk-sharedcnt-a1"), TokenId: 1, IP: "203.0.113.101", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: inviteeIdByName(t, "risk-sharedcnt-a2"), TokenId: 1, IP: "203.0.113.102", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: external.Id, TokenId: 1, IP: "203.0.113.101", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: external.Id, TokenId: 2, IP: "203.0.113.102", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		// inviter-1 的被邀请人共享 1 个 IP。
		{UserId: inviteeIdByName(t, "risk-sharedcnt-b1"), TokenId: 1, IP: "198.51.100.201", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: external.Id, TokenId: 3, IP: "198.51.100.201", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		// inviter-0 的被邀请人独享 IP，不构成共享。
		{UserId: inviteeIdByName(t, "risk-sharedcnt-c1"), TokenId: 1, IP: "10.0.0.99", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
	}
	require.NoError(t, DB.Create(&records).Error)

	list := func(minSharedIPs int) ([]*RiskInviter, int64) {
		items, total, err := ListRiskInviters("", 0, 10, "inviter_id", "asc", "", "any", false, 0, "", riskInviterDefaultRecentInvitesWindow, 0, minSharedIPs)
		require.NoError(t, err)
		return items, total
	}

	// 不筛选：行内 SharedIPCount 就是 SQL 派生值（本人 + 全部被邀请人）。
	items, total := list(0)
	require.EqualValues(t, 3, total)
	require.Len(t, items, 3)
	byId := make(map[int]*RiskInviter, len(items))
	for _, item := range items {
		byId[item.InviterId] = item
	}
	require.EqualValues(t, 2, byId[inviters[0].Id].SharedIPCount)
	require.True(t, byId[inviters[0].Id].Suspicious)
	require.EqualValues(t, 1, byId[inviters[1].Id].SharedIPCount)
	require.True(t, byId[inviters[1].Id].Suspicious)
	require.Zero(t, byId[inviters[2].Id].SharedIPCount)
	require.False(t, byId[inviters[2].Id].Suspicious)

	// min_shared_ips=2 只保留共享 2 个 IP 的邀请人，且行内值与筛选阈值一致。
	items, total = list(2)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviters[0].Id, items[0].InviterId)
	require.GreaterOrEqual(t, items[0].SharedIPCount, int64(2))

	// min_shared_ips=1：两个有共享 IP 的邀请人，均满足行内 >= 阈值。
	items, total = list(1)
	require.EqualValues(t, 2, total)
	require.Len(t, items, 2)
	for _, item := range items {
		require.GreaterOrEqual(t, item.SharedIPCount, int64(1))
	}
	// <=0 不筛选。
	_, total = list(-5)
	require.EqualValues(t, 3, total)

	// suspicious_only 与 min_shared_ips=1 口径完全一致。
	suspiciousItems, suspiciousTotal, err := ListRiskInviters("", 0, 10, "inviter_id", "asc", "", "any", true, 0, "", riskInviterDefaultRecentInvitesWindow, 0, 0)
	require.NoError(t, err)
	require.EqualValues(t, 2, suspiciousTotal)
	require.Len(t, suspiciousItems, 2)
	for _, item := range suspiciousItems {
		require.True(t, item.Suspicious)
		require.Positive(t, item.SharedIPCount)
	}
}

func TestListRiskInvitersSortsBySharedIPCountAndPaging(t *testing.T) {
	setupRiskControlTest(t)
	inviters := createRiskUsers(t, []User{
		{Username: "risk-sortshared-a", AffCode: "risk-sortshared-a-aff"},
		{Username: "risk-sortshared-b", AffCode: "risk-sortshared-b-aff"},
		{Username: "risk-sortshared-c", AffCode: "risk-sortshared-c-aff"},
		{Username: "risk-sortshared-d", AffCode: "risk-sortshared-d-aff"},
	})
	// a：3 个共享 IP；b/c：各 2 个；d：0 个。
	createRiskUsers(t, []User{
		{Username: "risk-sortshared-a1", AffCode: "risk-sortshared-a1-aff", InviterId: inviters[0].Id},
		{Username: "risk-sortshared-a2", AffCode: "risk-sortshared-a2-aff", InviterId: inviters[0].Id},
		{Username: "risk-sortshared-a3", AffCode: "risk-sortshared-a3-aff", InviterId: inviters[0].Id},
		{Username: "risk-sortshared-b1", AffCode: "risk-sortshared-b1-aff", InviterId: inviters[1].Id},
		{Username: "risk-sortshared-b2", AffCode: "risk-sortshared-b2-aff", InviterId: inviters[1].Id},
		{Username: "risk-sortshared-c1", AffCode: "risk-sortshared-c1-aff", InviterId: inviters[2].Id},
		{Username: "risk-sortshared-c2", AffCode: "risk-sortshared-c2-aff", InviterId: inviters[2].Id},
		{Username: "risk-sortshared-d1", AffCode: "risk-sortshared-d1-aff", InviterId: inviters[3].Id},
	})
	external := createRiskUsers(t, []User{
		{Username: "risk-sortshared-external", AffCode: "risk-sortshared-ext-aff"},
	})[0]
	records := make([]RiskIPRecord, 0, 16)
	tokenId := 1
	addShared := func(inviteeUsername string, ip string) {
		inviteeId := inviteeIdByName(t, inviteeUsername)
		records = append(records,
			RiskIPRecord{UserId: inviteeId, TokenId: tokenId, IP: ip, Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
			RiskIPRecord{UserId: external.Id, TokenId: tokenId, IP: ip, Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		)
		tokenId++
	}
	addShared("risk-sortshared-a1", "203.0.113.1")
	addShared("risk-sortshared-a2", "203.0.113.2")
	addShared("risk-sortshared-a3", "203.0.113.3")
	addShared("risk-sortshared-b1", "203.0.113.4")
	addShared("risk-sortshared-b2", "203.0.113.5")
	addShared("risk-sortshared-c1", "203.0.113.6")
	addShared("risk-sortshared-c2", "203.0.113.7")
	require.NoError(t, DB.Create(&records).Error)

	list := func(order string, startIdx int, limit int, minSharedIPs int) ([]int, int64) {
		items, total, err := ListRiskInviters("", startIdx, limit, "shared_ip_count", order, "", "any", false, 0, "", riskInviterDefaultRecentInvitesWindow, 0, minSharedIPs)
		require.NoError(t, err)
		ids := make([]int, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.InviterId)
		}
		return ids, total
	}

	// desc：3,2,2,0；2 并列时按稳定次序 users.id ASC（b 先于 c）。
	ids, total := list("desc", 0, 10, 0)
	require.EqualValues(t, 4, total)
	require.Equal(t, []int{inviters[0].Id, inviters[1].Id, inviters[2].Id, inviters[3].Id}, ids)

	// asc：0,2,2,3；并列仍按 users.id ASC。
	ids, total = list("asc", 0, 10, 0)
	require.EqualValues(t, 4, total)
	require.Equal(t, []int{inviters[3].Id, inviters[1].Id, inviters[2].Id, inviters[0].Id}, ids)

	// 先排序再分页，翻页无重复/丢失。
	page0, total := list("desc", 0, 2, 0)
	require.EqualValues(t, 4, total)
	page1, total := list("desc", 2, 2, 0)
	require.EqualValues(t, 4, total)
	require.Equal(t, []int{inviters[0].Id, inviters[1].Id}, page0)
	require.Equal(t, []int{inviters[2].Id, inviters[3].Id}, page1)

	// 先按 min_shared_ips 过滤再分页：total=3（a/b/c），逐页 limit=1 正确。
	expectedFiltered := []int{inviters[0].Id, inviters[1].Id, inviters[2].Id}
	for page, expected := range expectedFiltered {
		pageIds, pageTotal := list("desc", page, 1, 2)
		require.EqualValues(t, 3, pageTotal)
		require.Equal(t, []int{expected}, pageIds)
	}
	emptyIds, emptyTotal := list("desc", 3, 1, 2)
	require.EqualValues(t, 3, emptyTotal)
	require.Empty(t, emptyIds)
}

func TestListRiskInvitersFiltersRecentInvites(t *testing.T) {
	setupRiskControlTest(t)
	now := time.Now().Unix()
	inviters := createRiskUsers(t, []User{
		{Username: "risk-recent-inviter-a", AffCode: "risk-recent-a-aff"},
		{Username: "risk-recent-inviter-b", AffCode: "risk-recent-b-aff"},
		{Username: "risk-recent-inviter-c", AffCode: "risk-recent-c-aff"},
	})
	// a：86400 窗口内 3 人（100/7200/80000 秒前）+ 窗口外 2 人（200000/300000 秒前）；
	// b：窗口内 1 人 + 窗口外 1 人；c：只有窗口外 1 人。
	createRiskUsers(t, []User{
		{Username: "risk-recent-a1", AffCode: "risk-recent-a1-aff", InviterId: inviters[0].Id, CreatedAt: now - 100},
		{Username: "risk-recent-a2", AffCode: "risk-recent-a2-aff", InviterId: inviters[0].Id, CreatedAt: now - 7200},
		{Username: "risk-recent-a3", AffCode: "risk-recent-a3-aff", InviterId: inviters[0].Id, CreatedAt: now - 80000},
		{Username: "risk-recent-a4", AffCode: "risk-recent-a4-aff", InviterId: inviters[0].Id, CreatedAt: now - 200000},
		{Username: "risk-recent-a5", AffCode: "risk-recent-a5-aff", InviterId: inviters[0].Id, CreatedAt: now - 300000},
		{Username: "risk-recent-b1", AffCode: "risk-recent-b1-aff", InviterId: inviters[1].Id, CreatedAt: now - 7200},
		{Username: "risk-recent-b2", AffCode: "risk-recent-b2-aff", InviterId: inviters[1].Id, CreatedAt: now - 200000},
		{Username: "risk-recent-c1", AffCode: "risk-recent-c1-aff", InviterId: inviters[2].Id, CreatedAt: now - 300000},
	})

	list := func(window int, minRecentInvites int) ([]*RiskInviter, int64) {
		items, total, err := ListRiskInviters("", 0, 10, "inviter_id", "asc", "", "any", false, 0, "", window, minRecentInvites, 0)
		require.NoError(t, err)
		return items, total
	}
	recentByInviter := func(items []*RiskInviter) map[int]int64 {
		result := make(map[int]int64, len(items))
		for _, item := range items {
			result[item.InviterId] = item.RecentInviteCount
		}
		return result
	}

	// 响应字段：默认查询（不筛选、window 非法 → 回退 86400）也必须返回 recent_invite_count。
	items, total := list(0, 0)
	require.EqualValues(t, 3, total)
	require.Len(t, items, 3)
	defaultRecent := recentByInviter(items)
	require.EqualValues(t, 3, defaultRecent[inviters[0].Id], "a 在 86400 窗口内应为 3")
	require.EqualValues(t, 1, defaultRecent[inviters[1].Id], "b 在 86400 窗口内应为 1")
	require.Zero(t, defaultRecent[inviters[2].Id], "c 在 86400 窗口内应为 0")
	require.EqualValues(t, 3, items[0].RecentInviteCount, "响应结构体字段应与子查询一致")

	// window=86400 + 阈值 2：a(3) 命中，b(1) 不中；字段值必须正确。
	items, total = list(86400, 2)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inviters[0].Id, items[0].InviterId)
	require.EqualValues(t, 3, items[0].RecentInviteCount)

	// 阈值 4 > 窗口内 3 人 → 不命中（契约验收语义）。
	items, total = list(86400, 4)
	require.Zero(t, total)
	require.Empty(t, items)

	// window=3600：a 只有 1 个；不筛选时字段也应反映 3600 窗口。
	items, total = list(3600, 0)
	require.EqualValues(t, 3, total)
	windowRecent := recentByInviter(items)
	require.EqualValues(t, 1, windowRecent[inviters[0].Id])
	require.Zero(t, windowRecent[inviters[1].Id])

	// window=3600 + 阈值 2 → 空。
	items, total = list(3600, 2)
	require.Zero(t, total)
	require.Empty(t, items)

	// 非法窗口 1234 回退 86400：过滤与字段值都与 86400 一致。
	items, total = list(1234, 2)
	require.EqualValues(t, 1, total)
	require.EqualValues(t, 3, items[0].RecentInviteCount)

	// window=21600 + 阈值 2：a 窗口内 2 人命中；b 只有 1 人。
	items, total = list(21600, 2)
	require.EqualValues(t, 1, total)
	require.Equal(t, inviters[0].Id, items[0].InviterId)
	require.EqualValues(t, 2, items[0].RecentInviteCount)

	// window=604800 + 阈值 2：a(5)、b(2) 都命中，字段为窗口内真实人数。
	items, total = list(604800, 2)
	require.EqualValues(t, 2, total)
	weekRecent := recentByInviter(items)
	require.EqualValues(t, 5, weekRecent[inviters[0].Id])
	require.EqualValues(t, 2, weekRecent[inviters[1].Id])

	// 阈值 <=1 不筛选。
	_, total = list(3600, 1)
	require.EqualValues(t, 3, total)
}

// TestRiskControlPhase3JSONFieldNames 校验三期新增字段的 JSON key 确实存在：
// 避免「结构体/DB 值都有了但响应里漏字段」这类只能在真机发现的问题。
func TestRiskControlPhase3JSONFieldNames(t *testing.T) {
	inviterJSON, err := common.Marshal(&RiskInviter{
		RecentInviteCount: 7,
		SharedIPCount:     2,
		Suspicious:        true,
		Invitees:          []*RiskUserSummary{},
		MatchedInvitees:   []*RiskUserSummary{},
	})
	require.NoError(t, err)
	var inviterFields map[string]interface{}
	require.NoError(t, common.Unmarshal(inviterJSON, &inviterFields))
	require.EqualValues(t, 7, inviterFields["recent_invite_count"])
	require.EqualValues(t, 2, inviterFields["shared_ip_count"])
	require.Equal(t, true, inviterFields["suspicious"])

	summaryJSON, err := common.Marshal(&RiskUserSummary{
		UsedQuota: 11, RequestCount: 12, LastLoginAt: 13, CheckinCount: 14,
		RechargeQuota: 15, RechargeCount: 16,
	})
	require.NoError(t, err)
	var summaryFields map[string]interface{}
	require.NoError(t, common.Unmarshal(summaryJSON, &summaryFields))
	require.EqualValues(t, 11, summaryFields["used_quota"])
	require.EqualValues(t, 12, summaryFields["request_count"])
	require.EqualValues(t, 13, summaryFields["last_login_at"])
	require.EqualValues(t, 14, summaryFields["checkin_count"])
	require.EqualValues(t, 15, summaryFields["recharge_quota"])
	require.EqualValues(t, 16, summaryFields["recharge_count"])

	groupJSON, err := common.Marshal(&RiskInviteeGroup{
		ZombieCount: 1, CheckinOnlyCount: 2, ActiveCount: 3,
		UserIds: []int{}, Users: []*RiskUserSummary{},
	})
	require.NoError(t, err)
	var groupFields map[string]interface{}
	require.NoError(t, common.Unmarshal(groupJSON, &groupFields))
	require.EqualValues(t, 1, groupFields["zombie_count"])
	require.EqualValues(t, 2, groupFields["checkin_only_count"])
	require.EqualValues(t, 3, groupFields["active_count"])
}

// ---------------------------------------------------------------------------
// 第三阶段 H/I：活跃度字段、分组活跃度计数、充值聚合
// ---------------------------------------------------------------------------

func createRiskCheckins(t *testing.T, userId int, count int) {
	t.Helper()
	for index := 0; index < count; index++ {
		require.NoError(t, DB.Create(&Checkin{
			UserId:       userId,
			CheckinDate:  fmt.Sprintf("2026-02-%02d", index+1),
			QuotaAwarded: 10,
			CreatedAt:    int64(1000 + index),
		}).Error)
	}
}

func createRiskTopUp(t *testing.T, userId int, amount int64, status string, tradeNo string) {
	t.Helper()
	require.NoError(t, DB.Create(&TopUp{
		UserId:     userId,
		Amount:     amount,
		Money:      0,
		TradeNo:    tradeNo,
		CreateTime: 1000,
		Status:     status,
	}).Error)
}

func TestListRiskUserActivityAndRechargeFields(t *testing.T) {
	setupRiskControlTest(t)
	inviter := createRiskUsers(t, []User{
		{Username: "risk-activity-inviter", AffCode: "risk-activity-inviter-aff"},
	})[0]
	users := createRiskUsers(t, []User{
		{Username: "risk-activity-a", AffCode: "risk-activity-a-aff", InviterId: inviter.Id, UsedQuota: 500, RequestCount: 3, LastLoginAt: 111},
		{Username: "risk-activity-b", AffCode: "risk-activity-b-aff", InviterId: inviter.Id},
	})
	// a：2 次签到；b：0 次。
	createRiskCheckins(t, users[0].Id, 2)
	// a：2 笔成功充值（100+50）+ 1 笔未成功；b：0。
	createRiskTopUp(t, users[0].Id, 100, common.TopUpStatusSuccess, "risk-activity-trade-a1")
	createRiskTopUp(t, users[0].Id, 50, common.TopUpStatusSuccess, "risk-activity-trade-a2")
	createRiskTopUp(t, users[0].Id, 999, "pending", "risk-activity-trade-a3")
	// 两个用户共享同一 IP，用于验证同 IP 列表也带 H/I 字段。
	require.NoError(t, DB.Create(&[]RiskIPRecord{
		{UserId: users[0].Id, TokenId: 1, IP: "203.0.113.120", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
		{UserId: users[1].Id, TokenId: 1, IP: "203.0.113.120", Source: RiskIPSourceToken, FirstSeenAt: 1, LastSeenAt: 10, EventCount: 1},
	}).Error)

	assertActivity := func(label string, summary *RiskUserSummary) {
		t.Helper()
		require.NotNil(t, summary, label)
		if summary.UserId == users[0].Id {
			require.Equal(t, 500, summary.UsedQuota, label)
			require.Equal(t, 3, summary.RequestCount, label)
			require.EqualValues(t, 111, summary.LastLoginAt, label)
			require.EqualValues(t, 2, summary.CheckinCount, label)
			require.EqualValues(t, 150, summary.RechargeQuota, label)
			require.EqualValues(t, 2, summary.RechargeCount, label)
			return
		}
		require.Zero(t, summary.UsedQuota, label)
		require.Zero(t, summary.RequestCount, label)
		require.Zero(t, summary.LastLoginAt, label)
		require.Zero(t, summary.CheckinCount, label)
		require.Zero(t, summary.RechargeQuota, label)
		require.Zero(t, summary.RechargeCount, label)
	}

	// 1) ListRiskInvitees（平铺列表）
	invitees, total, err := ListRiskInvitees(inviter.Id, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	byId := riskUserSummaryMap(invitees)
	require.Len(t, byId, 2)
	assertActivity("ListRiskInvitees", byId[users[0].Id])
	assertActivity("ListRiskInvitees", byId[users[1].Id])

	// 2) ListRiskSharedIPs 的 users 列表
	sharedRows, _, err := ListRiskSharedIPs(RiskIPSourceToken, "ip", "", 2, 0, 10, "ip", "asc", "", false)
	require.NoError(t, err)
	require.Len(t, sharedRows, 1)
	require.Len(t, sharedRows[0].Users, 2)
	sharedById := riskUserSummaryMap(sharedRows[0].Users)
	assertActivity("ListRiskSharedIPs", sharedById[users[0].Id])
	assertActivity("ListRiskSharedIPs", sharedById[users[1].Id])

	// 3) ListRiskInviters 的 invitees + matched_invitees
	inviterItems, inviterTotal, err := ListRiskInviters("risk-activity-a", 0, 10, "inviter_id", "asc", "", "any", false, 0, "", riskInviterDefaultRecentInvitesWindow, 0, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, inviterTotal)
	require.Len(t, inviterItems, 1)
	inviterInvitees := riskUserSummaryMap(inviterItems[0].Invitees)
	assertActivity("ListRiskInviters.Invitees", inviterInvitees[users[0].Id])
	require.Len(t, inviterItems[0].MatchedInvitees, 1)
	assertActivity("ListRiskInviters.MatchedInvitees", inviterItems[0].MatchedInvitees[0])

	// 4) ListRiskInviteeGroups 的成员摘要 + H 组级计数（a 活跃、b 僵尸）。
	groupsResult, err := ListRiskInviteeGroups(inviter.Id, "ip", 3600, 50, 200)
	require.NoError(t, err)
	require.Equal(t, 1, groupsResult.GroupCount, "两个用户同 IP，归入同一组")
	allMembers := make([]*RiskUserSummary, 0)
	for _, group := range groupsResult.Groups {
		allMembers = append(allMembers, group.Users...)
	}
	groupById := riskUserSummaryMap(allMembers)
	assertActivity("ListRiskInviteeGroups", groupById[users[0].Id])
	assertActivity("ListRiskInviteeGroups", groupById[users[1].Id])
	var totalZombie, totalCheckinOnly, totalActive int64
	for _, group := range groupsResult.Groups {
		totalZombie += group.ZombieCount
		totalCheckinOnly += group.CheckinOnlyCount
		totalActive += group.ActiveCount
	}
	require.EqualValues(t, 1, totalActive)
	require.EqualValues(t, 1, totalZombie)
	require.Zero(t, totalCheckinOnly)
}

func TestListRiskInviteeGroupsActivityCounters(t *testing.T) {
	setupRiskControlTest(t)
	inviter := createRiskUsers(t, []User{
		{Username: "risk-counters-inviter", AffCode: "risk-counters-inviter-aff"},
	})[0]
	// 同一组（均无 IP 记录，归入 no_ip）内四类成员，判据与前端标签一致。
	users := createRiskUsers(t, []User{
		{Username: "risk-counters-zombie", AffCode: "risk-counters-zombie-aff", InviterId: inviter.Id},
		{Username: "risk-counters-checkin", AffCode: "risk-counters-checkin-aff", InviterId: inviter.Id},
		{Username: "risk-counters-active", AffCode: "risk-counters-active-aff", InviterId: inviter.Id, RequestCount: 5, UsedQuota: 10},
		{Username: "risk-counters-used-only", AffCode: "risk-counters-used-aff", InviterId: inviter.Id, UsedQuota: 100},
	})
	createRiskCheckins(t, users[1].Id, 1)

	result, err := ListRiskInviteeGroups(inviter.Id, "ip", 3600, 50, 200)
	require.NoError(t, err)
	require.Equal(t, 1, result.GroupCount)
	group := result.Groups[0]
	require.Equal(t, "no_ip", group.Key)
	require.EqualValues(t, 4, group.UserCount)
	require.EqualValues(t, 1, group.ZombieCount, "零请求零消耗零签到")
	require.EqualValues(t, 1, group.CheckinOnlyCount, "零请求但有签到")
	require.EqualValues(t, 1, group.ActiveCount, "有请求")
	// 「有消耗但零请求且无签到」不属于三个计数中的任何一个。
	require.EqualValues(t, 3, group.ZombieCount+group.CheckinOnlyCount+group.ActiveCount)
}

func TestListRiskInviteeGroupsActivityCountersExcludeUsedOnly(t *testing.T) {
	setupRiskControlTest(t)
	inviter := createRiskUsers(t, []User{
		{Username: "risk-usedonly-inviter", AffCode: "risk-usedonly-inviter-aff"},
	})[0]
	createRiskUsers(t, []User{
		{Username: "risk-usedonly-user", AffCode: "risk-usedonly-user-aff", InviterId: inviter.Id, UsedQuota: 100},
	})
	result, err := ListRiskInviteeGroups(inviter.Id, "time", 3600, 50, 200)
	require.NoError(t, err)
	require.Equal(t, 1, result.GroupCount)
	require.Zero(t, result.Groups[0].ZombieCount)
	require.Zero(t, result.Groups[0].CheckinOnlyCount)
	require.Zero(t, result.Groups[0].ActiveCount)
}

func TestListRiskInviteesRechargeAggregatesOnlySuccess(t *testing.T) {
	setupRiskControlTest(t)
	inviter := createRiskUsers(t, []User{
		{Username: "risk-recharge-inviter", AffCode: "risk-recharge-inviter-aff"},
	})[0]
	users := createRiskUsers(t, []User{
		{Username: "risk-recharge-a", AffCode: "risk-recharge-a-aff", InviterId: inviter.Id},
		{Username: "risk-recharge-b", AffCode: "risk-recharge-b-aff", InviterId: inviter.Id},
		{Username: "risk-recharge-c", AffCode: "risk-recharge-c-aff", InviterId: inviter.Id},
	})
	// a：两笔 success；另有 pending/failed 不计。
	createRiskTopUp(t, users[0].Id, 200, common.TopUpStatusSuccess, "risk-recharge-a1")
	createRiskTopUp(t, users[0].Id, 300, common.TopUpStatusSuccess, "risk-recharge-a2")
	createRiskTopUp(t, users[0].Id, 999, "pending", "risk-recharge-a3")
	createRiskTopUp(t, users[0].Id, 888, "failed", "risk-recharge-a4")
	// b：一笔 success。
	createRiskTopUp(t, users[1].Id, 50, common.TopUpStatusSuccess, "risk-recharge-b1")
	// c：只有失败记录 → 充值统计为 0。
	createRiskTopUp(t, users[2].Id, 77, "failed", "risk-recharge-c1")

	items, total, err := ListRiskInvitees(inviter.Id, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	byId := riskUserSummaryMap(items)
	require.EqualValues(t, 500, byId[users[0].Id].RechargeQuota)
	require.EqualValues(t, 2, byId[users[0].Id].RechargeCount)
	require.EqualValues(t, 50, byId[users[1].Id].RechargeQuota)
	require.EqualValues(t, 1, byId[users[1].Id].RechargeCount)
	require.Zero(t, byId[users[2].Id].RechargeQuota)
	require.Zero(t, byId[users[2].Id].RechargeCount)
}
