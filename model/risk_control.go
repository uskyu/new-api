package model

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RiskIPSourceLogin = "login"
	RiskIPSourceToken = "token"
	riskIPWriteWindow = int64(300)
)

// 风控列表的封禁状态筛选。作用于每条记录「关联账号」这一列：
//   - 空字符串：不筛选；
//   - RiskBanStateHasBanned：关联账号中至少有一个已封禁；
//   - RiskBanStateAllBanned：关联账号全部已封禁。
//
// 口径与页面展示一致：只统计未删除（deleted_at IS NULL）用户。
const (
	RiskBanStateHasBanned = "has_banned"
	RiskBanStateAllBanned = "all_banned"
)

// normalizeRiskBanState 把未知值归零为「不筛选」，避免非法参数改变结果集。
func normalizeRiskBanState(banState string) string {
	switch strings.TrimSpace(banState) {
	case RiskBanStateHasBanned:
		return RiskBanStateHasBanned
	case RiskBanStateAllBanned:
		return RiskBanStateAllBanned
	default:
		return ""
	}
}

// RiskIPRecord is a compact, queryable IP history. It deliberately aggregates
// repeated requests so the risk-control feature does not grow like request logs.
type RiskIPRecord struct {
	Id          int    `json:"id"`
	UserId      int    `json:"user_id" gorm:"uniqueIndex:uidx_risk_ip_user_token_source;index"`
	TokenId     int    `json:"token_id" gorm:"uniqueIndex:uidx_risk_ip_user_token_source;index"`
	IP          string `json:"ip" gorm:"type:varchar(64);uniqueIndex:uidx_risk_ip_user_token_source;index"`
	Source      string `json:"source" gorm:"type:varchar(16);uniqueIndex:uidx_risk_ip_user_token_source;index"`
	FirstSeenAt int64  `json:"first_seen_at" gorm:"bigint;index"`
	LastSeenAt  int64  `json:"last_seen_at" gorm:"bigint;index"`
	EventCount  int64  `json:"event_count" gorm:"default:0"`
}

type RiskSharedIP struct {
	IP         string             `json:"ip"`
	Source     string             `json:"source"`
	UserCount  int64              `json:"user_count"`
	EventCount int64              `json:"event_count"`
	LastSeenAt int64              `json:"last_seen_at"`
	Users      []*RiskUserSummary `json:"users" gorm:"-"`
}

type RiskUserSummary struct {
	UserId      int    `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`  // users.email，可能为空字符串
	Status      int    `json:"status"` // 1=正常 2=已封禁
	Quota       int    `json:"quota"`  // users.quota，原始额度值，前端按显示设置换算
	Group       string `json:"group"`
	Role        int    `json:"role"`
	InviterId   int    `json:"inviter_id"`
	TokenId     int    `json:"token_id"`
	Source      string `json:"source"`
	FirstSeenAt int64  `json:"first_seen_at"`
	LastSeenAt  int64  `json:"last_seen_at"`
	EventCount  int64  `json:"event_count"`
}

type RiskInviter struct {
	InviterId         int                `json:"inviter_id"`
	Username          string             `json:"username"`
	DisplayName       string             `json:"display_name"`
	Email             string             `json:"email"`
	Status            int                `json:"status"`
	Quota             int                `json:"quota"` // users.quota，原始额度值
	Group             string             `json:"group"`
	DirectInviteCount int64              `json:"direct_invite_count"`
	SharedIPCount     int64              `json:"shared_ip_count"`
	Suspicious        bool               `json:"suspicious"`
	Invitees          []*RiskUserSummary `json:"invitees" gorm:"-"`
	// MatchedInvitees 是 keyword 命中的被邀请人摘要（每个邀请人最多 20 个）。
	// 关键词为空或无命中时为空数组（不会是 nil）。
	MatchedInvitees []*RiskUserSummary `json:"matched_invitees" gorm:"-"`
}

type RiskOverview struct {
	TrackedRecords   int64 `json:"tracked_records"`
	TrackedUsers     int64 `json:"tracked_users"`
	SharedIPGroups   int64 `json:"shared_ip_groups"`
	InviterCount     int64 `json:"inviter_count"`
	ActiveRecords24h int64 `json:"active_records_24h"`
}

var riskIPWriteState sync.Map

func NormalizeRiskIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	ip := net.ParseIP(raw)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func riskIPStateKey(userId int, tokenId int, source string, ip string) string {
	return source + ":" + strconv.Itoa(userId) + ":" + strconv.Itoa(tokenId) + ":" + ip
}

func shouldWriteRiskIP(userId int, tokenId int, source string, ip string, now int64) bool {
	key := riskIPStateKey(userId, tokenId, source, ip)
	if value, ok := riskIPWriteState.Load(key); ok && value.(int64) > now {
		return false
	}
	riskIPWriteState.Store(key, now+riskIPWriteWindow)
	return true
}

func RecordRiskIP(userId int, tokenId int, source string, rawIP string) {
	if userId <= 0 || (source != RiskIPSourceLogin && source != RiskIPSourceToken) {
		return
	}
	ip := NormalizeRiskIP(rawIP)
	if ip == "" {
		return
	}
	now := common.GetTimestamp()
	if !shouldWriteRiskIP(userId, tokenId, source, ip, now) {
		return
	}
	record := &RiskIPRecord{
		UserId: userId, TokenId: tokenId, IP: ip, Source: source,
		FirstSeenAt: now, LastSeenAt: now, EventCount: 1,
	}
	err := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "token_id"}, {Name: "ip"}, {Name: "source"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_seen_at": now,
			"event_count":  gorm.Expr("event_count + ?", 1),
		}),
	}).Create(record).Error
	if err != nil {
		common.SysLog("failed to record risk IP: " + err.Error())
	}
}

func RecordRiskIPAsync(userId int, tokenId int, source string, ip string) {
	gopool.Go(func() {
		RecordRiskIP(userId, tokenId, source, ip)
	})
}

// riskSharedIPOrderClause 将排序白名单参数转换为 SQL 排序片段：
// 主排序字段按 order 方向，随后强制追加稳定次序 ip ASC, source ASC，
// 保证同等主排序值的行在翻页时不会重复或丢失。
// 非法 sortBy/order 回退为 last_seen_at desc。
func riskSharedIPOrderClause(sortBy string, order string) string {
	direction := "desc"
	if order == "asc" {
		direction = "asc"
	}
	switch sortBy {
	case "user_count":
		return "COUNT(DISTINCT user_id) " + direction + ", ip ASC, source ASC"
	case "event_count":
		return "SUM(event_count) " + direction + ", ip ASC, source ASC"
	case "first_seen_at":
		return "MIN(first_seen_at) " + direction + ", ip ASC, source ASC"
	case "ip":
		return "ip " + direction + ", source ASC"
	default: // last_seen_at
		return "MAX(last_seen_at) " + direction + ", ip ASC, source ASC"
	}
}

// riskSharedIPBanHaving 生成「关联账号封禁状态」筛选的 HAVING 片段。
// 只统计未删除用户；同一用户可能有多条 IP 记录，因此用 COUNT(DISTINCT ...) 口径。
// 返回空 having 表示不筛选。CASE WHEN + COUNT(DISTINCT) + 子查询在
// SQLite/MySQL/PostgreSQL 上语义一致。
func riskSharedIPBanHaving(banState string) (string, []interface{}) {
	bannedCount := "COUNT(DISTINCT CASE WHEN user_id IN (" +
		"SELECT id FROM users WHERE status = ? AND deleted_at IS NULL) THEN user_id END)"
	switch banState {
	case RiskBanStateHasBanned:
		return bannedCount + " > 0", []interface{}{common.UserStatusDisabled}
	case RiskBanStateAllBanned:
		return bannedCount + " = COUNT(DISTINCT user_id)", []interface{}{common.UserStatusDisabled}
	default:
		return "", nil
	}
}

func ListRiskSharedIPs(source string, searchType string, keyword string, minUsers int, startIdx int, limit int, sortBy string, order string, banState string) ([]*RiskSharedIP, int64, error) {
	if minUsers < 2 {
		minUsers = 2
	}
	if limit <= 0 {
		limit = common.ItemsPerPage
	}
	banState = normalizeRiskBanState(banState)

	var matchingUserIds []int
	var matchingIPs []string
	if searchType == "username" && keyword != "" {
		pattern, err := sanitizeLikePattern("%" + keyword + "%")
		if err != nil {
			return nil, 0, err
		}
		if err := DB.Model(&User{}).
			Where("username LIKE ? ESCAPE '!' OR display_name LIKE ? ESCAPE '!'", pattern, pattern).
			Pluck("id", &matchingUserIds).Error; err != nil {
			return nil, 0, err
		}
		if len(matchingUserIds) == 0 {
			return []*RiskSharedIP{}, 0, nil
		}
	} else if searchType == "user_id" && keyword != "" {
		id, err := strconv.Atoi(strings.TrimSpace(keyword))
		if err != nil || id <= 0 {
			return []*RiskSharedIP{}, 0, nil
		}
		matchingUserIds = append(matchingUserIds, id)
	}
	if len(matchingUserIds) > 0 {
		if err := DB.Model(&RiskIPRecord{}).
			Where("user_id IN ? AND ip <> ''", matchingUserIds).
			Distinct("ip").
			Pluck("ip", &matchingIPs).Error; err != nil {
			return nil, 0, err
		}
		if len(matchingIPs) == 0 {
			return []*RiskSharedIP{}, 0, nil
		}
	}

	pattern := ""
	if searchType != "username" && searchType != "user_id" && keyword != "" {
		var err error
		pattern, err = sanitizeLikePattern("%" + keyword + "%")
		if err != nil {
			return nil, 0, err
		}
	}
	applyFilters := func(tx *gorm.DB) *gorm.DB {
		tx = tx.Where("ip <> ''")
		if source == RiskIPSourceLogin || source == RiskIPSourceToken {
			tx = tx.Where("source = ?", source)
		}
		if len(matchingIPs) > 0 {
			tx = tx.Where("ip IN ?", matchingIPs)
		}
		if pattern != "" {
			tx = tx.Where("ip LIKE ? ESCAPE '!'", pattern)
		}
		return tx
	}
	groupColumns := "ip"
	selectColumns := "ip, 'all' AS source"
	if source == RiskIPSourceLogin || source == RiskIPSourceToken {
		groupColumns = "ip, source"
		selectColumns = "ip, source"
	}
	grouped := applyFilters(DB.Model(&RiskIPRecord{})).Select(groupColumns).Group(groupColumns).Having("COUNT(DISTINCT user_id) >= ?", minUsers)
	if having, args := riskSharedIPBanHaving(banState); having != "" {
		grouped = grouped.Having(having, args...)
	}
	var total int64
	if err := DB.Table("(?) AS risk_ip_groups", grouped).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []*RiskSharedIP
	query := applyFilters(DB.Model(&RiskIPRecord{})).Select(selectColumns+", COUNT(DISTINCT user_id) AS user_count, SUM(event_count) AS event_count, MAX(last_seen_at) AS last_seen_at").
		Group(groupColumns).Having("COUNT(DISTINCT user_id) >= ?", minUsers)
	if having, args := riskSharedIPBanHaving(banState); having != "" {
		query = query.Having(having, args...)
	}
	err := query.Order(riskSharedIPOrderClause(sortBy, order)).Offset(startIdx).Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	for _, row := range rows {
		users, err := listRiskIPUsers(row.IP, row.Source)
		if err != nil {
			return nil, 0, err
		}
		row.Users = users
	}
	return rows, total, nil
}

func listRiskIPUsers(ip string, source string) ([]*RiskUserSummary, error) {
	var records []*RiskIPRecord
	tx := DB.Where("ip = ?", ip)
	if source == RiskIPSourceLogin || source == RiskIPSourceToken {
		tx = tx.Where("source = ?", source)
	}
	if err := tx.Order("last_seen_at desc").Find(&records).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.UserId)
	}
	users := make(map[int]*User)
	if len(ids) > 0 {
		var list []*User
		if err := DB.Select("id, username, display_name, "+commonGroupCol+", role, inviter_id, email, status, quota").Where("id IN ?", ids).Find(&list).Error; err != nil {
			return nil, err
		}
		for _, user := range list {
			users[user.Id] = user
		}
	}
	result := make([]*RiskUserSummary, 0, len(records))
	byUser := make(map[int]*RiskUserSummary)
	for _, record := range records {
		user := users[record.UserId]
		if user == nil {
			continue
		}
		summary := byUser[user.Id]
		if summary == nil {
			summary = riskUserSummaryFromUser(user)
			summary.TokenId = record.TokenId
			summary.Source = record.Source
			summary.FirstSeenAt = record.FirstSeenAt
			summary.LastSeenAt = record.LastSeenAt
			summary.EventCount = record.EventCount
			byUser[user.Id] = summary
			result = append(result, summary)
			continue
		}
		if record.FirstSeenAt < summary.FirstSeenAt {
			summary.FirstSeenAt = record.FirstSeenAt
		}
		if record.LastSeenAt > summary.LastSeenAt {
			summary.LastSeenAt = record.LastSeenAt
		}
		summary.EventCount += record.EventCount
	}
	return result, nil
}

// riskInviterAggregateRow 是邀请人聚合分页查询的扫描结果。
type riskInviterAggregateRow struct {
	InviterId         int
	Username          string
	DisplayName       string
	Email             string
	Status            int
	Quota             int
	Group             string
	DirectInviteCount int64
}

// riskInviterOrderClause 将排序白名单参数转换为 SQL 排序片段，并在主排序后
// 强制追加 inviter_id (users.id) ASC 作为稳定次序。非法参数回退为
// direct_invite_count desc。
func riskInviterOrderClause(sortBy string, order string) string {
	direction := "desc"
	if order == "asc" {
		direction = "asc"
	}
	switch sortBy {
	case "inviter_id":
		return "users.id " + direction + ", users.id ASC"
	case "direct_invite_count":
		return "inviter_stats.direct_invite_count " + direction + ", users.id ASC"
	case "created_at":
		return "users.created_at " + direction + ", users.id ASC"
	case "aff_quota":
		return "users.aff_quota " + direction + ", users.id ASC"
	default:
		return "inviter_stats.direct_invite_count desc, users.id ASC"
	}
}

// riskUserSummaryFromUser 将用户记录转换为风控用户摘要（含 email/status/quota）。
func riskUserSummaryFromUser(user *User) *RiskUserSummary {
	return &RiskUserSummary{
		UserId: user.Id, Username: user.Username, DisplayName: user.DisplayName,
		Email: user.Email, Status: user.Status, Quota: user.Quota,
		Group: user.Group, Role: user.Role, InviterId: user.InviterId,
	}
}

// riskUserKeywordMatchCondition 构建单个用户记录的 keyword 匹配条件与参数
// （id / username / display_name）。数字关键词始终按用户 ID 精确匹配，并在
// 关键词长度 >= 2 时继续对 username / display_name 做 LIKE 匹配；非数字关键词
// 沿用原有 LIKE 模糊搜索规则（最短 2 个字符）。prefix 为列名前缀（如 "users."）。
func riskUserKeywordMatchCondition(prefix string, keyword string) (string, []interface{}, error) {
	keyword = strings.TrimSpace(keyword)
	numericId := 0
	if parsedId, err := strconv.Atoi(keyword); err == nil && parsedId > 0 {
		numericId = parsedId
	}
	conditions := make([]string, 0, 3)
	args := make([]interface{}, 0, 3)
	if numericId > 0 {
		conditions = append(conditions, prefix+"id = ?")
		args = append(args, numericId)
	}
	// 单字符数字关键词只做 ID 精确匹配：LIKE 模糊搜索沿用既有最小长度限制。
	if numericId == 0 || len(keyword) >= 2 {
		pattern, err := sanitizeLikePattern("%" + keyword + "%")
		if err != nil {
			return "", nil, err
		}
		conditions = append(conditions,
			prefix+"username LIKE ? ESCAPE '!'",
			prefix+"display_name LIKE ? ESCAPE '!'",
		)
		args = append(args, pattern, pattern)
	}
	return "(" + strings.Join(conditions, " OR ") + ")", args, nil
}

// riskInviterKeywordCondition 构建邀请关系 keyword 过滤条件：关键词同时匹配
// 邀请人自身（users.id / username / display_name）或其任一被邀请人；只有被邀请人
// 命中时也会返回对应邀请人行。返回的 SQL 已用外层括号分组，可直接作为 AND 条件。
func riskInviterKeywordCondition(keyword string) (string, []interface{}, error) {
	inviterCondition, inviterArgs, err := riskUserKeywordMatchCondition("users.", keyword)
	if err != nil {
		return "", nil, err
	}
	inviteeCondition, inviteeArgs, err := riskUserKeywordMatchCondition("", keyword)
	if err != nil {
		return "", nil, err
	}
	// 命中任一被邀请人时取其 inviter_id 匹配邀请人；软删除用户不参与。
	inviteeSubquery := "SELECT DISTINCT inviter_id FROM users WHERE inviter_id > 0 AND deleted_at IS NULL AND " + inviteeCondition
	condition := "(" + inviterCondition + " OR inviter_stats.inviter_id IN (" + inviteeSubquery + "))"
	args := make([]interface{}, 0, len(inviterArgs)+len(inviteeArgs))
	args = append(args, inviterArgs...)
	args = append(args, inviteeArgs...)
	return condition, args, nil
}

// listRiskMatchedInvitees 返回指定邀请人被 keyword 命中的前 20 个被邀请人（id DESC）。
// 关键词为空时返回空数组（不为 nil）。
func listRiskMatchedInvitees(inviterId int, keyword string) ([]*RiskUserSummary, error) {
	summaries := make([]*RiskUserSummary, 0)
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return summaries, nil
	}
	condition, args, err := riskUserKeywordMatchCondition("", keyword)
	if err != nil {
		return nil, err
	}
	var users []*User
	if err := DB.Select("id, username, display_name, "+commonGroupCol+", role, inviter_id, email, status, quota").
		Where("inviter_id = ?", inviterId).
		Where(condition, args...).
		Order("id desc").Limit(20).
		Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		summaries = append(summaries, riskUserSummaryFromUser(user))
	}
	return summaries, nil
}

// riskInviterBanCondition 生成「被邀请账号封禁状态」筛选条件（作用于邀请人行）。
// 只统计未删除的被邀请人；返回空条件表示不筛选。
func riskInviterBanCondition(banState string) (string, []interface{}) {
	switch banState {
	case RiskBanStateHasBanned:
		return "EXISTS (SELECT 1 FROM users invitees WHERE invitees.inviter_id = users.id AND invitees.deleted_at IS NULL AND invitees.status = ?)",
			[]interface{}{common.UserStatusDisabled}
	case RiskBanStateAllBanned:
		return "EXISTS (SELECT 1 FROM users invitees WHERE invitees.inviter_id = users.id AND invitees.deleted_at IS NULL AND invitees.status = ?) " +
				"AND NOT EXISTS (SELECT 1 FROM users invitees WHERE invitees.inviter_id = users.id AND invitees.deleted_at IS NULL AND invitees.status <> ?)",
			[]interface{}{common.UserStatusDisabled, common.UserStatusDisabled}
	default:
		return "", nil
	}
}

// ListRiskInviters 聚合所有邀请人的直接邀请人数并分页返回。
// 聚合（COUNT(*) GROUP BY inviter_id）与分页都在数据库侧完成，total 与
// direct_invite_count 都是真实总数而非当前页统计；当前页再补前 100 个被邀请人、
// matched_invitees（keyword 命中的被邀请人，最多 20 个）与 shared_ip_count。
// 实现只用 GORM 子查询 + JOIN，兼容 SQLite/MySQL/PostgreSQL。
func ListRiskInviters(keyword string, startIdx int, limit int, sortBy string, order string, banState string) ([]*RiskInviter, int64, error) {
	if limit <= 0 {
		limit = common.ItemsPerPage
	}
	keyword = strings.TrimSpace(keyword)
	banState = normalizeRiskBanState(banState)
	var keywordCondition string
	var keywordArgs []interface{}
	if keyword != "" {
		var err error
		keywordCondition, keywordArgs, err = riskInviterKeywordCondition(keyword)
		if err != nil {
			return nil, 0, err
		}
	}
	banCondition, banArgs := riskInviterBanCondition(banState)
	buildQuery := func() *gorm.DB {
		inviterStats := DB.Model(&User{}).
			Select("inviter_id, COUNT(*) AS direct_invite_count").
			Where("inviter_id > 0").
			Group("inviter_id")
		tx := DB.Table("(?) AS inviter_stats", inviterStats).
			Joins("JOIN users ON users.id = inviter_stats.inviter_id").
			Where("users.deleted_at IS NULL")
		if keywordCondition != "" {
			tx = tx.Where(keywordCondition, keywordArgs...)
		}
		if banCondition != "" {
			tx = tx.Where(banCondition, banArgs...)
		}
		return tx
	}
	var total int64
	if err := buildQuery().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*RiskInviter{}, 0, nil
	}
	var rows []*riskInviterAggregateRow
	err := buildQuery().
		Select("users.id AS inviter_id, users.username, users.display_name, users.email, users.status, users.quota, users." + commonGroupCol + ", inviter_stats.direct_invite_count").
		Order(riskInviterOrderClause(sortBy, order)).
		Offset(startIdx).Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	result := make([]*RiskInviter, 0, len(rows))
	for _, row := range rows {
		var invitees []*User
		if err := DB.Select("id, username, display_name, "+commonGroupCol+", role, inviter_id, email, status, quota").
			Where("inviter_id = ?", row.InviterId).Order("id desc").Limit(100).Find(&invitees).Error; err != nil {
			return nil, 0, err
		}
		ids := make([]int, 0, len(invitees)+1)
		ids = append(ids, row.InviterId)
		summaries := make([]*RiskUserSummary, 0, len(invitees))
		for _, invitee := range invitees {
			ids = append(ids, invitee.Id)
			summaries = append(summaries, riskUserSummaryFromUser(invitee))
		}
		sharedIPs, err := countSharedIPsForUsers(ids)
		if err != nil {
			return nil, 0, err
		}
		matchedInvitees, err := listRiskMatchedInvitees(row.InviterId, keyword)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, &RiskInviter{
			InviterId: row.InviterId, Username: row.Username, DisplayName: row.DisplayName,
			Email: row.Email, Status: row.Status, Quota: row.Quota, Group: row.Group,
			DirectInviteCount: row.DirectInviteCount,
			SharedIPCount:     sharedIPs,
			Suspicious:        sharedIPs > 0,
			Invitees:          summaries,
			MatchedInvitees:   matchedInvitees,
		})
	}
	return result, total, nil
}

// ListRiskInvitees 分页返回指定邀请人的直接被邀请用户（按 users.id DESC）。
// inviterId 非法或不存在时返回空列表而非错误，便于前端直接展示空态。
func ListRiskInvitees(inviterId int, startIdx int, limit int) ([]*RiskUserSummary, int64, error) {
	if inviterId <= 0 {
		return []*RiskUserSummary{}, 0, nil
	}
	if limit <= 0 {
		limit = common.ItemsPerPage
	}
	var total int64
	if err := DB.Model(&User{}).Where("inviter_id = ?", inviterId).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*RiskUserSummary{}, 0, nil
	}
	var users []*User
	if err := DB.Select("id, username, display_name, "+commonGroupCol+", role, inviter_id, email, status, quota").
		Where("inviter_id = ?", inviterId).
		Order("id desc").Offset(startIdx).Limit(limit).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*RiskUserSummary, 0, len(users))
	for _, user := range users {
		items = append(items, riskUserSummaryFromUser(user))
	}
	return items, total, nil
}

func countSharedIPsForUsers(userIds []int) (int64, error) {
	if len(userIds) < 2 {
		return 0, nil
	}
	var count int64
	err := DB.Model(&RiskIPRecord{}).Where("user_id IN ? AND ip <> ''", userIds).
		Select("COUNT(DISTINCT ip)").Where("ip IN (?)", DB.Model(&RiskIPRecord{}).Select("ip").Where("user_id IN ? AND ip <> ''", userIds).Group("ip").Having("COUNT(DISTINCT user_id) > 1")).Scan(&count).Error
	return count, err
}

func GetRiskOverview() (*RiskOverview, error) {
	result := &RiskOverview{}
	if err := DB.Model(&RiskIPRecord{}).Count(&result.TrackedRecords).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&RiskIPRecord{}).Distinct("user_id").Count(&result.TrackedUsers).Error; err != nil {
		return nil, err
	}
	var groups []struct{ IP string }
	if err := DB.Model(&RiskIPRecord{}).Select("ip").Where("ip <> ''").Group("ip").Having("COUNT(DISTINCT user_id) >= 2").Find(&groups).Error; err != nil {
		return nil, err
	}
	result.SharedIPGroups = int64(len(groups))
	if err := DB.Model(&User{}).Where("inviter_id > 0").Distinct("inviter_id").Count(&result.InviterCount).Error; err != nil {
		return nil, err
	}
	cutoff := time.Now().Unix() - 86400
	return result, DB.Model(&RiskIPRecord{}).Where("last_seen_at >= ?", cutoff).Count(&result.ActiveRecords24h).Error
}

func ListRiskIPRecordsByUser(userId int) ([]*RiskIPRecord, error) {
	var records []*RiskIPRecord
	err := DB.Where("user_id = ?", userId).Order("last_seen_at desc").Limit(200).Find(&records).Error
	return records, err
}
