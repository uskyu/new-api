package model

import (
	"fmt"
	"net"
	"sort"
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

// 邀请关系列表的搜索类型：any（默认，匹配用户字段） / ip（匹配 IP 精确或前缀）。
const (
	riskInviterSearchAny = "any"
	riskInviterSearchIP  = "ip"
)

// 邀请人自身状态筛选：仅支持 disabled，其余值不筛选。
const riskInviterStatusDisabled = "disabled"

// 邀请关系高危筛选（E）：最近邀请窗口白名单（秒）与默认值。
const riskInviterDefaultRecentInvitesWindow = 86400

var riskInviterRecentInvitesWindows = map[int]struct{}{
	3600:   {},
	21600:  {},
	86400:  {},
	604800: {},
}

func normalizeRiskInviterRecentInvitesWindow(window int) int {
	if _, ok := riskInviterRecentInvitesWindows[window]; ok {
		return window
	}
	return riskInviterDefaultRecentInvitesWindow
}

// riskInviterSharedIPCountExpr 计算「邀请人本人或其任一被邀请人与其他未删除账号
// 共享的去重 IP 数」的 SQL 派生列（契约 phase3 E.2）。口径为邀请人 + **全部**被邀请人，
// 不再局限当前页/前 100 个被邀请人。
//
// 该表达式同时用于：
//   - 行内 RiskInviter.SharedIPCount 的取值；
//   - min_shared_ips / suspicious_only 的 SQL 过滤（先过滤再分页，保证 total 正确）；
//   - ORDER BY（通过别名 risk_shared_ip_count）。
//
// 性能说明：这是一个相关子查询，无法在 WHERE/ORDER BY 里引用 SELECT 别名，
// 因此 WHERE 处会重复表达式；risk_ip_records.ip / user_id、users.id / inviter_id
// 均有索引，但本表达式仍会按邀请人行逐行求值。契约要求本期不加索引，若线上邀请人
// 规模大导致该列表变慢，需要单独立项评估（如落冗余列/加索引）。
const riskInviterSharedIPCountExpr = "(SELECT COUNT(DISTINCT a.ip) FROM risk_ip_records a " +
	"JOIN risk_ip_records b ON a.ip = b.ip AND a.user_id <> b.user_id " +
	"JOIN users ax ON ax.id = a.user_id AND ax.deleted_at IS NULL " +
	"JOIN users bx ON bx.id = b.user_id AND bx.deleted_at IS NULL " +
	"WHERE a.ip <> '' AND (ax.id = users.id OR ax.inviter_id = users.id))"

// riskInviterRecentInviteCountExpr 计算「最近窗口内新增被邀请人数」的 SQL 相关子查询
// （唯一参数为窗口起点 cutoff）。按契约 E.1：它既用于 min_recent_invites 过滤，
// 也作为响应字段 recent_invite_count 返回；默认查询（不过滤）同样需要 SELECT 该列。
const riskInviterRecentInviteCountExpr = "(SELECT COUNT(*) FROM users recent_inv WHERE recent_inv.inviter_id = users.id " +
	"AND recent_inv.deleted_at IS NULL AND recent_inv.created_at >= ?)"

func normalizeRiskInviterSearchType(raw string) string {
	if strings.TrimSpace(raw) == riskInviterSearchIP {
		return riskInviterSearchIP
	}
	return riskInviterSearchAny
}

func normalizeRiskInviterStatus(raw string) string {
	if strings.TrimSpace(raw) == riskInviterStatusDisabled {
		return riskInviterStatusDisabled
	}
	return ""
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
	IP              string             `json:"ip"`
	Source          string             `json:"source"`
	Subnet          string             `json:"subnet"`            // 该 IP 所在网段（IPv4 /24、IPv6 /64），无法解析为空串
	SubnetUserCount int64              `json:"subnet_user_count"` // 当前页同网段去重账号数（含本行；按页统计，翻页可能变化）
	UserCount       int64              `json:"user_count"`
	EventCount      int64              `json:"event_count"`
	LastSeenAt      int64              `json:"last_seen_at"`
	Users           []*RiskUserSummary `json:"users" gorm:"-"`
}

type RiskUserSummary struct {
	UserId      int    `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`      // users.email，可能为空字符串
	Status      int    `json:"status"`     // 1=正常 2=已封禁
	Quota       int    `json:"quota"`      // users.quota，原始额度值，前端按显示设置换算
	IP          string `json:"ip"`         // 该用户最近一次出现的 IP，无记录时为空串
	CreatedAt   int64  `json:"created_at"` // users.created_at
	// H：活跃度（僵尸号识别）
	UsedQuota    int   `json:"used_quota"`    // users.used_quota 历史消耗
	RequestCount int   `json:"request_count"` // users.request_count 历史请求数
	LastLoginAt  int64 `json:"last_login_at"` // users.last_login_at
	CheckinCount int64 `json:"checkin_count"` // checkins 表记录数
	// I：充值统计（top_ups.status = success，口径为到账额度）
	RechargeQuota int64  `json:"recharge_quota"` // SUM(top_ups.amount)
	RechargeCount int64  `json:"recharge_count"` // COUNT(*)
	Group         string `json:"group"`
	Role          int    `json:"role"`
	InviterId     int    `json:"inviter_id"`
	TokenId       int    `json:"token_id"`
	Source        string `json:"source"`
	FirstSeenAt   int64  `json:"first_seen_at"`
	LastSeenAt    int64  `json:"last_seen_at"`
	EventCount    int64  `json:"event_count"`
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
	RecentInviteCount int64              `json:"recent_invite_count"` // 最近窗口内新增被邀请人数（E，响应字段）
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

// ListRiskSharedIPs 返回同 IP 用户组列表。
// search_type=username|email 时按用户字段匹配（均包含邮箱模糊匹配）；
// search_type=user_id 时按用户 ID 精确匹配；其余按 IP 模糊匹配。
// active24h=true 时只保留 MAX(last_seen_at) >= now-86400 的 IP 组。
func ListRiskSharedIPs(source string, searchType string, keyword string, minUsers int, startIdx int, limit int, sortBy string, order string, banState string, active24h bool) ([]*RiskSharedIP, int64, error) {
	if minUsers < 2 {
		minUsers = 2
	}
	if limit <= 0 {
		limit = common.ItemsPerPage
	}
	banState = normalizeRiskBanState(banState)
	searchType = strings.TrimSpace(searchType)
	keyword = strings.TrimSpace(keyword)

	var matchingUserIds []int
	var matchingIPs []string
	ipPattern := ""
	// userSearchApplied 表示本次关键词走「用户维度」搜索（username/email/user_id）。
	// 该维度命中 0 个用户时必须立即返回空列表，不能退化为「不筛选」而返回全量 IP 组。
	userSearchApplied := false
	if keyword != "" {
		switch searchType {
		case "username":
			userSearchApplied = true
			pattern, err := sanitizeLikePattern("%" + keyword + "%")
			if err != nil {
				return nil, 0, err
			}
			if err := DB.Model(&User{}).
				Where("username LIKE ? ESCAPE '!' OR display_name LIKE ? ESCAPE '!' OR email LIKE ? ESCAPE '!'", pattern, pattern, pattern).
				Pluck("id", &matchingUserIds).Error; err != nil {
				return nil, 0, err
			}
		case "email":
			userSearchApplied = true
			pattern, err := sanitizeLikePattern("%" + keyword + "%")
			if err != nil {
				return nil, 0, err
			}
			if err := DB.Model(&User{}).
				Where("email LIKE ? ESCAPE '!'", pattern).
				Pluck("id", &matchingUserIds).Error; err != nil {
				return nil, 0, err
			}
		case "user_id":
			userSearchApplied = true
			id, err := strconv.Atoi(keyword)
			if err != nil || id <= 0 {
				return []*RiskSharedIP{}, 0, nil
			}
			matchingUserIds = append(matchingUserIds, id)
		default:
			pattern, err := sanitizeLikePattern("%" + keyword + "%")
			if err != nil {
				return nil, 0, err
			}
			ipPattern = pattern
		}
	}
	if userSearchApplied {
		if len(matchingUserIds) == 0 {
			return []*RiskSharedIP{}, 0, nil
		}
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

	applyFilters := func(tx *gorm.DB) *gorm.DB {
		tx = tx.Where("ip <> ''")
		if source == RiskIPSourceLogin || source == RiskIPSourceToken {
			tx = tx.Where("source = ?", source)
		}
		if len(matchingIPs) > 0 {
			tx = tx.Where("ip IN ?", matchingIPs)
		}
		if ipPattern != "" {
			tx = tx.Where("ip LIKE ? ESCAPE '!'", ipPattern)
		}
		return tx
	}
	groupColumns := "ip"
	selectColumns := "ip, 'all' AS source"
	if source == RiskIPSourceLogin || source == RiskIPSourceToken {
		groupColumns = "ip, source"
		selectColumns = "ip, source"
	}
	// 24 小时活跃阈值只取一次，保证 count 与分页查询使用同一时间基准。
	activeCutoff := time.Now().Unix() - 86400
	applyHaving := func(tx *gorm.DB) *gorm.DB {
		tx = tx.Having("COUNT(DISTINCT user_id) >= ?", minUsers)
		if having, args := riskSharedIPBanHaving(banState); having != "" {
			tx = tx.Having(having, args...)
		}
		if active24h {
			tx = tx.Having("MAX(last_seen_at) >= ?", activeCutoff)
		}
		return tx
	}
	grouped := applyHaving(applyFilters(DB.Model(&RiskIPRecord{})).Select(groupColumns).Group(groupColumns))
	var total int64
	if err := DB.Table("(?) AS risk_ip_groups", grouped).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []*RiskSharedIP
	query := applyHaving(applyFilters(DB.Model(&RiskIPRecord{})).Select(selectColumns + ", COUNT(DISTINCT user_id) AS user_count, SUM(event_count) AS event_count, MAX(last_seen_at) AS last_seen_at").
		Group(groupColumns))
	err := query.Order(riskSharedIPOrderClause(sortBy, order)).Offset(startIdx).Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	// 整页只做一次 checkins / top_ups 批量聚合（H/I），避免逐行/逐用户查询。
	pageSummaries := make([]*RiskUserSummary, 0)
	for _, row := range rows {
		users, err := listRiskIPUsers(row.IP, row.Source)
		if err != nil {
			return nil, 0, err
		}
		row.Users = users
		pageSummaries = append(pageSummaries, users...)
	}
	if err := hydrateRiskUserSummaries(pageSummaries); err != nil {
		return nil, 0, err
	}
	if err := attachRiskSharedIPSubnets(rows); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// riskIPSubnet 返回 IP 所在网段：IPv4 取前 3 段（/24），IPv6 取前 4 段（/64）。
// 解析失败返回空字符串。网段计算必须在 Go 里完成，保证 SQLite/MySQL/PostgreSQL 一致。
func riskIPSubnet(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return ""
	}
	if ip4 := ip.To4(); ip4 != nil {
		return fmt.Sprintf("%d.%d.%d.0/24", ip4[0], ip4[1], ip4[2])
	}
	ip16 := ip.To16()
	if ip16 == nil {
		return ""
	}
	// IPv6 /64：保留前 8 字节（4 个 hextet），其余清零后按标准压缩格式输出。
	masked := make(net.IP, net.IPv6len)
	copy(masked, ip16[:8])
	return masked.String() + "/64"
}

// attachRiskSharedIPSubnets 为当前页的同 IP 行补充网段信息。
// 只为当前页行计算，且用一次带 IN 的聚合查询完成（禁止逐行查询）：
// 统计范围为「当前页这些 IP」下按 (ip, user_id) 去重后的账号，即同一网段
// 在当前页内的去重账号数（含本行账号）。
//
// 注意口径：该值按页统计，不含不在当前页的同网段 IP 组；因此同一网段的行
// 被分页拆开时，不同页看到的 subnet_user_count 可能不同。前端只把它当作
// 「疑似」提示（subnet_user_count > user_count 时展示），不作为精确总数使用。
func attachRiskSharedIPSubnets(rows []*RiskSharedIP) error {
	if len(rows) == 0 {
		return nil
	}
	pageIPs := make([]string, 0, len(rows))
	ipSubnet := make(map[string]string, len(rows))
	for _, row := range rows {
		subnet := riskIPSubnet(row.IP)
		row.Subnet = subnet
		if subnet == "" {
			continue
		}
		if _, ok := ipSubnet[row.IP]; !ok {
			ipSubnet[row.IP] = subnet
			pageIPs = append(pageIPs, row.IP)
		}
	}
	if len(pageIPs) == 0 {
		return nil
	}
	type riskIPUserPair struct {
		IP     string
		UserId int
	}
	var pairs []*riskIPUserPair
	if err := DB.Model(&RiskIPRecord{}).
		Select("ip, user_id").
		Where("ip IN ? AND ip <> ''", pageIPs).
		Group("ip, user_id").
		Scan(&pairs).Error; err != nil {
		return err
	}
	subnetUsers := make(map[string]map[int]struct{})
	for _, pair := range pairs {
		subnet := ipSubnet[pair.IP]
		if subnet == "" {
			continue
		}
		users := subnetUsers[subnet]
		if users == nil {
			users = make(map[int]struct{})
			subnetUsers[subnet] = users
		}
		users[pair.UserId] = struct{}{}
	}
	for _, row := range rows {
		if row.Subnet == "" {
			row.SubnetUserCount = 0
			continue
		}
		row.SubnetUserCount = int64(len(subnetUsers[row.Subnet]))
	}
	return nil
}

// listLatestRiskIPRecordsByUserIds 返回每个用户 last_seen_at 最大的那条 IP 记录。
// 同一 last_seen_at 有多条记录时按 ip 字典序取最小，保证结果稳定。
// 无任何 IP 记录的用户不在返回 map 中。按 500 个 id 分块，避免 SQLite 变量上限。
func listLatestRiskIPRecordsByUserIds(userIds []int) (map[int]*RiskIPRecord, error) {
	latest := make(map[int]*RiskIPRecord, len(userIds))
	const chunkSize = 500
	for start := 0; start < len(userIds); start += chunkSize {
		end := start + chunkSize
		if end > len(userIds) {
			end = len(userIds)
		}
		var records []*RiskIPRecord
		if err := DB.Select("user_id, ip, first_seen_at, last_seen_at").
			Where("user_id IN ? AND ip <> ''", userIds[start:end]).
			Where("last_seen_at = (SELECT MAX(latest.last_seen_at) FROM risk_ip_records latest WHERE latest.user_id = risk_ip_records.user_id)").
			Order("user_id ASC, last_seen_at DESC, ip ASC").
			Find(&records).Error; err != nil {
			return nil, err
		}
		for _, record := range records {
			if _, ok := latest[record.UserId]; ok {
				continue
			}
			latest[record.UserId] = record
		}
	}
	return latest, nil
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
		if err := DB.Select("id, username, display_name, "+commonGroupCol+", role, inviter_id, email, status, quota, created_at, used_quota, request_count, last_login_at").Where("id IN ?", ids).Find(&list).Error; err != nil {
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
			summary.IP = record.IP
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
	// RecentInviteCount 来自 SQL 派生列 recent_invite_count（契约 phase3 E.1）。
	RecentInviteCount int64 `gorm:"column:recent_invite_count"`
	// SharedIPCount 来自 SQL 派生列 risk_shared_ip_count（契约 phase3 E.2）；
	// 显式 gorm column 保证 Scan 能按别名映射到该字段。
	SharedIPCount int64 `gorm:"column:risk_shared_ip_count"`
}

// riskInviterOrderClause 将排序白名单参数转换为 SQL 排序片段，并在主排序后
// 强制追加 inviter_id (users.id) ASC 作为稳定次序。非法参数回退为
// direct_invite_count desc。shared_ip_count 使用 SQL 派生列别名排序。
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
	case "shared_ip_count":
		return "risk_shared_ip_count " + direction + ", users.id ASC"
	default:
		return "inviter_stats.direct_invite_count desc, users.id ASC"
	}
}

// riskUserSummaryFromUser 将用户记录转换为风控用户摘要
// （含 email/status/quota/created_at 与 H 的活跃度基础字段）。
func riskUserSummaryFromUser(user *User) *RiskUserSummary {
	return &RiskUserSummary{
		UserId: user.Id, Username: user.Username, DisplayName: user.DisplayName,
		Email: user.Email, Status: user.Status, Quota: user.Quota,
		CreatedAt: user.CreatedAt,
		UsedQuota: user.UsedQuota, RequestCount: user.RequestCount, LastLoginAt: user.LastLoginAt,
		Group: user.Group, Role: user.Role, InviterId: user.InviterId,
	}
}

// riskUserActivityStats 是单个用户的 checkins / top_ups 聚合结果。
type riskUserActivityStats struct {
	CheckinCount  int64
	RechargeQuota int64
	RechargeCount int64
}

// loadRiskUserActivityStats 批量加载一批用户的签到次数与充值统计：
//   - checkins：SELECT user_id, COUNT(*) ... WHERE user_id IN (...) GROUP BY user_id
//   - top_ups： SELECT user_id, SUM(amount), COUNT(*) ... WHERE user_id IN (...) AND status='success' GROUP BY user_id
//
// 每张表只发一条查询（禁止 N+1）。调用方传入的都是一页数据（≤ 2000 个 id，
// ListRiskInviters 单页最多约 10000 个被邀请人 id），一次 IN 在三个数据库的参数上限内。
func loadRiskUserActivityStats(userIds []int) (map[int]*riskUserActivityStats, error) {
	stats := make(map[int]*riskUserActivityStats, len(userIds))
	ids := dedupeRiskUserIds(userIds)
	if len(ids) == 0 {
		return stats, nil
	}
	ensureStats := func(userId int) *riskUserActivityStats {
		item := stats[userId]
		if item == nil {
			item = &riskUserActivityStats{}
			stats[userId] = item
		}
		return item
	}

	var checkinRows []*struct {
		UserId int
		Count  int64
	}
	if err := DB.Model(&Checkin{}).
		Select("user_id, COUNT(*) AS count").
		Where("user_id IN ?", ids).
		Group("user_id").
		Scan(&checkinRows).Error; err != nil {
		return nil, err
	}
	for _, row := range checkinRows {
		ensureStats(row.UserId).CheckinCount = row.Count
	}

	var rechargeRows []*struct {
		UserId int
		Quota  int64
		Count  int64
	}
	if err := DB.Model(&TopUp{}).
		Select("user_id, SUM(amount) AS quota, COUNT(*) AS count").
		Where("user_id IN ? AND status = ?", ids, common.TopUpStatusSuccess).
		Group("user_id").
		Scan(&rechargeRows).Error; err != nil {
		return nil, err
	}
	for _, row := range rechargeRows {
		item := ensureStats(row.UserId)
		item.RechargeQuota = row.Quota
		item.RechargeCount = row.Count
	}
	return stats, nil
}

// hydrateRiskUserSummaries 为一页内的用户摘要批量补齐 checkins / top_ups 统计。
func hydrateRiskUserSummaries(summaries []*RiskUserSummary) error {
	if len(summaries) == 0 {
		return nil
	}
	userIds := make([]int, 0, len(summaries))
	for _, summary := range summaries {
		if summary != nil {
			userIds = append(userIds, summary.UserId)
		}
	}
	stats, err := loadRiskUserActivityStats(userIds)
	if err != nil {
		return err
	}
	for _, summary := range summaries {
		if summary == nil {
			continue
		}
		if item := stats[summary.UserId]; item != nil {
			summary.CheckinCount = item.CheckinCount
			summary.RechargeQuota = item.RechargeQuota
			summary.RechargeCount = item.RechargeCount
		}
	}
	return nil
}

// applyRiskUserActivityStats 把已加载的聚合结果写入单个用户摘要。
func applyRiskUserActivityStats(summary *RiskUserSummary, stats *riskUserActivityStats) {
	if summary == nil || stats == nil {
		return
	}
	summary.CheckinCount = stats.CheckinCount
	summary.RechargeQuota = stats.RechargeQuota
	summary.RechargeCount = stats.RechargeCount
}

// escapeRiskLikeLiteral 转义 LIKE 模式中的特殊字符（ESCAPE '!'），
// 用于把用户输入的 IP 关键词当作字面前缀匹配。
func escapeRiskLikeLiteral(input string) string {
	input = strings.ReplaceAll(input, "!", "!!")
	input = strings.ReplaceAll(input, "%", "!%")
	input = strings.ReplaceAll(input, "_", "!_")
	return input
}

// riskIPKeywordCondition 构造 RiskIPRecord.ip 的「精确 / 前缀」匹配条件。
// 可解析为合法 IP 时同时做精确匹配；前缀匹配使用字面前缀，避免用户输入被当作通配符。
func riskIPKeywordCondition(column string, keyword string) (string, []interface{}) {
	keyword = strings.TrimSpace(keyword)
	prefixPattern := escapeRiskLikeLiteral(keyword) + "%"
	if exact := NormalizeRiskIP(keyword); exact != "" {
		return "(" + column + " = ? OR " + column + " LIKE ? ESCAPE '!')", []interface{}{exact, prefixPattern}
	}
	return "(" + column + " LIKE ? ESCAPE '!')", []interface{}{prefixPattern}
}

// riskUserKeywordMatchCondition 构建单个用户记录的 keyword 匹配条件与参数
// （id / username / display_name / email）。数字关键词始终按用户 ID 精确匹配，
// 并在关键词长度 >= 2 时继续对 username / display_name / email 做 LIKE 匹配；
// 非数字关键词沿用原有 LIKE 模糊搜索规则（最短 2 个字符）。
// prefix 为列名前缀（如 "users."）。
func riskUserKeywordMatchCondition(prefix string, keyword string) (string, []interface{}, error) {
	keyword = strings.TrimSpace(keyword)
	numericId := 0
	if parsedId, err := strconv.Atoi(keyword); err == nil && parsedId > 0 {
		numericId = parsedId
	}
	conditions := make([]string, 0, 4)
	args := make([]interface{}, 0, 4)
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
			prefix+"email LIKE ? ESCAPE '!'",
		)
		args = append(args, pattern, pattern, pattern)
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

// riskInviterIPKeywordCondition 构建邀请关系的 IP 关键词过滤条件：
// 命中「邀请人本人或其任一被邀请人用过该 IP」的邀请人。IP 采用精确/前缀匹配。
func riskInviterIPKeywordCondition(keyword string) (string, []interface{}) {
	ipCondition, ipArgs := riskIPKeywordCondition("r.ip", keyword)
	condition := "EXISTS (SELECT 1 FROM risk_ip_records r JOIN users ip_users ON ip_users.id = r.user_id " +
		"WHERE ip_users.deleted_at IS NULL AND (ip_users.id = users.id OR ip_users.inviter_id = users.id) AND " + ipCondition + ")"
	return condition, ipArgs
}

// listRiskMatchedInvitees 返回指定邀请人被 keyword 命中的前 20 个被邀请人（id DESC）。
// searchType=ip 时按「该被邀请人用过该 IP」匹配，否则按用户字段（id/用户名/昵称/邮箱）匹配。
// 关键词为空时返回空数组（不为 nil）。
func listRiskMatchedInvitees(inviterId int, keyword string, searchType string) ([]*RiskUserSummary, error) {
	summaries := make([]*RiskUserSummary, 0)
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return summaries, nil
	}
	var condition string
	var args []interface{}
	if normalizeRiskInviterSearchType(searchType) == "ip" {
		ipCondition, ipArgs := riskIPKeywordCondition("r.ip", keyword)
		condition = "EXISTS (SELECT 1 FROM risk_ip_records r WHERE r.user_id = users.id AND " + ipCondition + ")"
		args = ipArgs
	} else {
		var err error
		condition, args, err = riskUserKeywordMatchCondition("", keyword)
		if err != nil {
			return nil, err
		}
	}
	var users []*User
	if err := DB.Select("id, username, display_name, "+commonGroupCol+", role, inviter_id, email, status, quota, created_at, used_quota, request_count, last_login_at").
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
// matched_invitees（keyword 命中的被邀请人，最多 20 个）、recent_invite_count（最近窗口内
// 新增邀请人数，默认查询也会返回）、shared_ip_count 与用户活跃度/充值统计
// （H/I，整页各一次批量聚合，禁止 N+1）。
//
// searchType=ip 时 keyword 按 IP 精确/前缀匹配「邀请人本人或其被邀请人用过的 IP」；
// 其余（默认 any）沿用用户字段匹配。新增筛选均在 SQL 侧完成，保证 total 与分页正确：
//   - suspiciousOnly：shared_ip_count > 0（与行内口径完全一致，使用 SQL 派生列）；
//   - minInvites >= 2：direct_invite_count >= minInvites；
//   - inviterStatus=disabled：邀请人自身 status=2；
//   - minRecentInvites >= 2：最近 recentInvitesWindow 秒内新增被邀请人 >= 阈值；
//   - minSharedIPs >= 1：shared_ip_count >= minSharedIPs。
//
// 行内 RiskInviter.SharedIPCount / Suspicious 与上述筛选/排序统一使用
// riskInviterSharedIPCountExpr 的 SQL 派生值（口径 = 邀请人本人 + 全部被邀请人），
// 不再使用基于当前页前 100 个被邀请人的 Go 侧统计，消除「筛选命中但行内
// suspicious=false」的历史不一致。
//
// 实现只用 GORM 子查询 + JOIN，兼容 SQLite/MySQL/PostgreSQL。
func ListRiskInviters(keyword string, startIdx int, limit int, sortBy string, order string, banState string, searchType string, suspiciousOnly bool, minInvites int, inviterStatus string, recentInvitesWindow int, minRecentInvites int, minSharedIPs int) ([]*RiskInviter, int64, error) {
	if limit <= 0 {
		limit = common.ItemsPerPage
	}
	keyword = strings.TrimSpace(keyword)
	banState = normalizeRiskBanState(banState)
	searchType = normalizeRiskInviterSearchType(searchType)
	inviterStatus = normalizeRiskInviterStatus(inviterStatus)
	recentInvitesWindow = normalizeRiskInviterRecentInvitesWindow(recentInvitesWindow)
	if minInvites <= 1 {
		minInvites = 0
	}
	if minRecentInvites <= 1 {
		minRecentInvites = 0
	}
	if minSharedIPs <= 0 {
		minSharedIPs = 0
	}
	var keywordCondition string
	var keywordArgs []interface{}
	if keyword != "" {
		if searchType == riskInviterSearchIP {
			keywordCondition, keywordArgs = riskInviterIPKeywordCondition(keyword)
		} else {
			var err error
			keywordCondition, keywordArgs, err = riskInviterKeywordCondition(keyword)
			if err != nil {
				return nil, 0, err
			}
		}
	}
	banCondition, banArgs := riskInviterBanCondition(banState)
	// 最近邀请窗口的时间阈值只取一次，保证 count 与分页查询使用同一时间基准。
	recentInvitesCutoff := time.Now().Unix() - int64(recentInvitesWindow)
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
		if minInvites >= 2 {
			tx = tx.Where("inviter_stats.direct_invite_count >= ?", minInvites)
		}
		// 「需复核」与 min_shared_ips 都直接复用 SQL 派生列，保证过滤、排序与行内值一致。
		if suspiciousOnly {
			tx = tx.Where(riskInviterSharedIPCountExpr + " > 0")
		}
		if minSharedIPs >= 1 {
			tx = tx.Where(riskInviterSharedIPCountExpr+" >= ?", minSharedIPs)
		}
		if minRecentInvites >= 2 {
			tx = tx.Where(riskInviterRecentInviteCountExpr+" >= ?", recentInvitesCutoff, minRecentInvites)
		}
		if inviterStatus == riskInviterStatusDisabled {
			tx = tx.Where("users.status = ?", common.UserStatusDisabled)
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
	// recent_invite_count 是响应字段（E.1），默认查询（不传 min_recent_invites）也必须计算；
	// 唯一占位符为窗口起点 recentInvitesCutoff。
	err := buildQuery().
		Select("users.id AS inviter_id, users.username, users.display_name, users.email, users.status, users.quota, users."+commonGroupCol+", inviter_stats.direct_invite_count, "+riskInviterSharedIPCountExpr+" AS risk_shared_ip_count, "+riskInviterRecentInviteCountExpr+" AS recent_invite_count", recentInvitesCutoff).
		Order(riskInviterOrderClause(sortBy, order)).
		Offset(startIdx).Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	// 整页只做一次 checkins / top_ups 批量聚合（H/I），当前页的被邀请人摘要与
	// matched_invitees 摘要共用同一次查询结果。
	pageSummaries := make([]*RiskUserSummary, 0)
	result := make([]*RiskInviter, 0, len(rows))
	for _, row := range rows {
		var invitees []*User
		if err := DB.Select("id, username, display_name, "+commonGroupCol+", role, inviter_id, email, status, quota, created_at, used_quota, request_count, last_login_at").
			Where("inviter_id = ?", row.InviterId).Order("id desc").Limit(100).Find(&invitees).Error; err != nil {
			return nil, 0, err
		}
		summaries := make([]*RiskUserSummary, 0, len(invitees))
		for _, invitee := range invitees {
			summaries = append(summaries, riskUserSummaryFromUser(invitee))
		}
		matchedInvitees, err := listRiskMatchedInvitees(row.InviterId, keyword, searchType)
		if err != nil {
			return nil, 0, err
		}
		pageSummaries = append(pageSummaries, summaries...)
		pageSummaries = append(pageSummaries, matchedInvitees...)
		result = append(result, &RiskInviter{
			InviterId: row.InviterId, Username: row.Username, DisplayName: row.DisplayName,
			Email: row.Email, Status: row.Status, Quota: row.Quota, Group: row.Group,
			DirectInviteCount: row.DirectInviteCount,
			RecentInviteCount: row.RecentInviteCount,
			SharedIPCount:     row.SharedIPCount,
			Suspicious:        row.SharedIPCount > 0,
			Invitees:          summaries,
			MatchedInvitees:   matchedInvitees,
		})
	}
	if err := hydrateRiskUserSummaries(pageSummaries); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

// ListRiskInvitees 分页返回指定邀请人的直接被邀请用户（按 users.id DESC）。
// 每行补 created_at 与该用户最近一次出现的 IP / last_seen_at（无记录时为空/0）。
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
	if err := DB.Select("id, username, display_name, "+commonGroupCol+", role, inviter_id, email, status, quota, created_at, used_quota, request_count, last_login_at").
		Where("inviter_id = ?", inviterId).
		Order("id desc").Offset(startIdx).Limit(limit).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}
	ids := make([]int, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.Id)
	}
	latestRecords, err := listLatestRiskIPRecordsByUserIds(ids)
	if err != nil {
		return nil, 0, err
	}
	items := make([]*RiskUserSummary, 0, len(users))
	for _, user := range users {
		summary := riskUserSummaryFromUser(user)
		if record := latestRecords[user.Id]; record != nil {
			summary.IP = record.IP
			summary.LastSeenAt = record.LastSeenAt
		}
		items = append(items, summary)
	}
	// 整页一次批量聚合 checkins / top_ups（H/I）。
	if err := hydrateRiskUserSummaries(items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// 邀请人被邀请人分组接口（GET /inviters/:id/invitee-groups）的常量与结构。
const (
	RiskInviteeGroupByIP     = "ip"
	RiskInviteeGroupByTime   = "time"
	RiskInviteeGroupBySubnet = "subnet"

	RiskInviteeGroupKindIP     = "ip"
	RiskInviteeGroupKindTime   = "time"
	RiskInviteeGroupKindSubnet = "subnet"
	RiskInviteeGroupKindNoIP   = "no_ip"

	// riskInviteeGroupMaxLoadedUsers 是参与分组的被邀请人上限。
	riskInviteeGroupMaxLoadedUsers = 2000
	// riskInviteeGroupDefaultGroupUsers 是每组返回成员摘要上限的默认值与上限。
	riskInviteeGroupDefaultGroupUsers = 50
	riskInviteeGroupMaxGroupUsers     = 50
	// riskInviteeGroupDefaultLimit 是返回分组数上限的默认值与上限。
	riskInviteeGroupDefaultLimit = 200
	riskInviteeGroupMaxLimit     = 200
)

// riskInviteeGroupTimeWindows 是 group_by=time 允许的分桶秒数白名单。
var riskInviteeGroupTimeWindows = map[int64]struct{}{
	300:   {},
	1800:  {},
	3600:  {},
	21600: {},
	86400: {},
}

// RiskInviteeGroup 是邀请人名下被邀请人的一个分组。
type RiskInviteeGroup struct {
	Key         string `json:"key"`
	Kind        string `json:"kind"`         // ip | time | subnet | no_ip
	Label       string `json:"label"`        // ip → IP；subnet → 网段；time / no_ip → ""
	BucketStart int64  `json:"bucket_start"` // 仅 kind=time
	UserCount   int64  `json:"user_count"`
	BannedCount int64  `json:"banned_count"`
	// H：活跃度分组计数（Go 侧按成员摘要累加）
	ZombieCount      int64              `json:"zombie_count"`       // request_count==0 && used_quota==0 && checkin_count==0
	CheckinOnlyCount int64              `json:"checkin_only_count"` // request_count==0 && checkin_count>0
	ActiveCount      int64              `json:"active_count"`       // request_count>0
	FirstSeenAt      int64              `json:"first_seen_at"`      // 组内成员最早出现；无 IP 记录为 0
	LastSeenAt       int64              `json:"last_seen_at"`       // 组内成员最近出现；无 IP 记录为 0
	UserIds          []int              `json:"user_ids"`           // 组内完整 user_id（上限 2000）
	UserIdsTruncated bool               `json:"user_ids_truncated"`
	Users            []*RiskUserSummary `json:"users"` // 最多 group_users 个成员摘要
}

// RiskInviteeGroupsResult 是 GET /inviters/:id/invitee-groups 的响应 data 结构。
type RiskInviteeGroupsResult struct {
	InviterId       int                 `json:"inviter_id"`
	GroupBy         string              `json:"group_by"`
	TimeWindow      int64               `json:"time_window"`
	TotalUsers      int64               `json:"total_users"`
	LoadedUsers     int                 `json:"loaded_users"`
	Truncated       bool                `json:"truncated"`
	GroupCount      int                 `json:"group_count"`
	GroupsTruncated bool                `json:"groups_truncated"`
	Groups          []*RiskInviteeGroup `json:"groups"`
}

func normalizeRiskInviteeGroupBy(raw string) string {
	switch strings.TrimSpace(raw) {
	case RiskInviteeGroupByTime:
		return RiskInviteeGroupByTime
	case RiskInviteeGroupBySubnet:
		return RiskInviteeGroupBySubnet
	default:
		return RiskInviteeGroupByIP
	}
}

func normalizeRiskInviteeGroupTimeWindow(window int64) int64 {
	if _, ok := riskInviteeGroupTimeWindows[window]; ok {
		return window
	}
	return 3600
}

func normalizeRiskInviteeGroupUsers(groupUsers int) int {
	if groupUsers <= 0 {
		return riskInviteeGroupDefaultGroupUsers
	}
	if groupUsers > riskInviteeGroupMaxGroupUsers {
		return riskInviteeGroupMaxGroupUsers
	}
	return groupUsers
}

func normalizeRiskInviteeGroupLimit(limitGroups int) int {
	if limitGroups <= 0 {
		return riskInviteeGroupDefaultLimit
	}
	if limitGroups > riskInviteeGroupMaxLimit {
		return riskInviteeGroupMaxLimit
	}
	return limitGroups
}

// ListRiskInviteeGroups 按指定维度对邀请人名下的被邀请人分组。
//
// 分组、时间分桶与网段计算全部在 Go 里完成，SQL 只负责取数：
//   - group_by=ip：按用户「最近一次出现的 IP」归组，一个用户只计一次；无记录归入 no_ip；
//   - group_by=time：按 users.created_at / time_window 分桶（bucket_start 在 Go 里算）；
//   - group_by=subnet：按最近一次 IP 的网段（IPv4 /24、IPv6 /64）归组，解析失败归入 no_ip。
//
// 排序统一为 user_count DESC、key ASC，no_ip 组强制最后；分组数超过 limit_groups 时截断。
// 参与分组的用户上限 2000，超过时 truncated=true。inviterId <= 0 返回空分组不报错。
func ListRiskInviteeGroups(inviterId int, groupBy string, timeWindow int64, groupUsers int, limitGroups int) (*RiskInviteeGroupsResult, error) {
	groupBy = normalizeRiskInviteeGroupBy(groupBy)
	timeWindow = normalizeRiskInviteeGroupTimeWindow(timeWindow)
	groupUsers = normalizeRiskInviteeGroupUsers(groupUsers)
	limitGroups = normalizeRiskInviteeGroupLimit(limitGroups)
	result := &RiskInviteeGroupsResult{
		InviterId:  inviterId,
		GroupBy:    groupBy,
		TimeWindow: timeWindow,
		Groups:     make([]*RiskInviteeGroup, 0),
	}
	if inviterId <= 0 {
		return result, nil
	}
	var totalUsers int64
	if err := DB.Model(&User{}).Where("inviter_id = ?", inviterId).Count(&totalUsers).Error; err != nil {
		return nil, err
	}
	result.TotalUsers = totalUsers
	if totalUsers == 0 {
		return result, nil
	}
	var users []*User
	if err := DB.Select("id, username, display_name, "+commonGroupCol+", role, inviter_id, email, status, quota, created_at, used_quota, request_count, last_login_at").
		Where("inviter_id = ?", inviterId).
		Order("id ASC").
		Limit(riskInviteeGroupMaxLoadedUsers).
		Find(&users).Error; err != nil {
		return nil, err
	}
	result.LoadedUsers = len(users)
	result.Truncated = int64(len(users)) < totalUsers

	userIds := make([]int, 0, len(users))
	for _, user := range users {
		userIds = append(userIds, user.Id)
	}
	latestRecords, err := listLatestRiskIPRecordsByUserIds(userIds)
	if err != nil {
		return nil, err
	}
	// 整批（≤2000 人）一次批量聚合 checkins / top_ups（H/I）。
	userStats, err := loadRiskUserActivityStats(userIds)
	if err != nil {
		return nil, err
	}

	type inviteeGroupBuilder struct {
		group        *RiskInviteeGroup
		memberIds    []int
		members      []*RiskUserSummary
		firstSeenSet bool
		lastSeenSet  bool
	}
	builders := make(map[string]*inviteeGroupBuilder)
	for _, user := range users {
		record := latestRecords[user.Id]
		var key, kind, label string
		var bucketStart int64
		switch groupBy {
		case RiskInviteeGroupByTime:
			bucketStart = (user.CreatedAt / timeWindow) * timeWindow
			key = "time:" + strconv.FormatInt(bucketStart, 10)
			kind = RiskInviteeGroupKindTime
		case RiskInviteeGroupBySubnet:
			subnet := ""
			if record != nil {
				subnet = riskIPSubnet(record.IP)
			}
			if subnet == "" {
				key, kind, label = "no_ip", RiskInviteeGroupKindNoIP, ""
			} else {
				key, kind, label = "subnet:"+subnet, RiskInviteeGroupKindSubnet, subnet
			}
		default: // ip
			if record == nil || record.IP == "" {
				key, kind, label = "no_ip", RiskInviteeGroupKindNoIP, ""
			} else {
				key, kind, label = "ip:"+record.IP, RiskInviteeGroupKindIP, record.IP
			}
		}
		builder := builders[key]
		if builder == nil {
			builder = &inviteeGroupBuilder{group: &RiskInviteeGroup{
				Key: key, Kind: kind, Label: label, BucketStart: bucketStart,
				UserIds: make([]int, 0), Users: make([]*RiskUserSummary, 0),
			}}
			builders[key] = builder
		}
		builder.group.UserCount++
		if user.Status == common.UserStatusDisabled {
			builder.group.BannedCount++
		}
		builder.memberIds = append(builder.memberIds, user.Id)
		summary := riskUserSummaryFromUser(user)
		applyRiskUserActivityStats(summary, userStats[user.Id])
		if record != nil {
			summary.IP = record.IP
			summary.LastSeenAt = record.LastSeenAt
		}
		builder.members = append(builder.members, summary)
		// H：组级活跃度计数（判据与前端标签一致）。
		switch {
		case summary.RequestCount > 0:
			builder.group.ActiveCount++
		case summary.CheckinCount > 0:
			builder.group.CheckinOnlyCount++
		case summary.UsedQuota == 0:
			builder.group.ZombieCount++
		}
		// 组级 first/last seen 仅对 IP 相关维度有含义；time / no_ip 固定为 0。
		if record != nil && (kind == RiskInviteeGroupKindIP || kind == RiskInviteeGroupKindSubnet) {
			if !builder.firstSeenSet || record.FirstSeenAt < builder.group.FirstSeenAt {
				builder.group.FirstSeenAt = record.FirstSeenAt
				builder.firstSeenSet = true
			}
			if !builder.lastSeenSet || record.LastSeenAt > builder.group.LastSeenAt {
				builder.group.LastSeenAt = record.LastSeenAt
				builder.lastSeenSet = true
			}
		}
	}

	groups := make([]*RiskInviteeGroup, 0, len(builders))
	for _, builder := range builders {
		group := builder.group
		group.UserIds = append(group.UserIds, builder.memberIds...)
		group.UserIdsTruncated = len(builder.memberIds) > riskInviteeGroupMaxLoadedUsers
		for index, summary := range builder.members {
			if index >= groupUsers {
				break
			}
			group.Users = append(group.Users, summary)
		}
		groups = append(groups, group)
	}
	// user_count DESC、key ASC；no_ip 组强制最后。
	sort.SliceStable(groups, func(i int, j int) bool {
		iNoIP := groups[i].Kind == RiskInviteeGroupKindNoIP
		jNoIP := groups[j].Kind == RiskInviteeGroupKindNoIP
		if iNoIP != jNoIP {
			return !iNoIP
		}
		if groups[i].UserCount != groups[j].UserCount {
			return groups[i].UserCount > groups[j].UserCount
		}
		return groups[i].Key < groups[j].Key
	})
	if len(groups) > limitGroups {
		groups = groups[:limitGroups]
		result.GroupsTruncated = true
	}
	result.GroupCount = len(groups)
	result.Groups = groups
	return result, nil
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
