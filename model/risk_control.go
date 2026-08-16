package model

import (
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
	Group             string             `json:"group"`
	DirectInviteCount int64              `json:"direct_invite_count"`
	SharedIPCount     int64              `json:"shared_ip_count"`
	Suspicious        bool               `json:"suspicious"`
	LatestInviteAt    int64              `json:"latest_invite_at"`
	Invitees          []*RiskUserSummary `json:"invitees" gorm:"-"`
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

func ListRiskSharedIPs(source string, searchType string, keyword string, minUsers int, startIdx int, limit int) ([]*RiskSharedIP, int64, error) {
	if minUsers < 2 {
		minUsers = 2
	}
	if limit <= 0 {
		limit = common.ItemsPerPage
	}

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
	var total int64
	if err := DB.Table("(?) AS risk_ip_groups", grouped).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []*RiskSharedIP
	err := applyFilters(DB.Model(&RiskIPRecord{})).Select(selectColumns+", COUNT(DISTINCT user_id) AS user_count, SUM(event_count) AS event_count, MAX(last_seen_at) AS last_seen_at").
		Group(groupColumns).Having("COUNT(DISTINCT user_id) >= ?", minUsers).
		Order("last_seen_at desc").Offset(startIdx).Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	for _, row := range rows {
		row.Users, err = listRiskIPUsers(row.IP, row.Source)
		if err != nil {
			return nil, 0, err
		}
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
		if err := DB.Select("id, username, display_name, "+commonGroupCol+", role, inviter_id").Where("id IN ?", ids).Find(&list).Error; err != nil {
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
			summary = &RiskUserSummary{
				UserId: user.Id, Username: user.Username, DisplayName: user.DisplayName,
				Group: user.Group, Role: user.Role, InviterId: user.InviterId,
				TokenId: record.TokenId, Source: record.Source, FirstSeenAt: record.FirstSeenAt,
				LastSeenAt: record.LastSeenAt, EventCount: record.EventCount,
			}
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

type riskInviterRow struct {
	InviterId         int
	Username          string
	DisplayName       string
	Group             string
	DirectInviteCount int64
	LatestInviteAt    int64
	SharedIPCount     int64
}

func chunkInts(ids []int, size int) [][]int {
	if size <= 0 {
		size = 500
	}
	var chunks [][]int
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[start:end])
	}
	return chunks
}

// riskInviteeScan holds the invitation graph for a set of inviters.
type riskInviteeScan struct {
	memberGroups   map[int][]int // user id -> group (inviter) ids the user belongs to
	inviteeCount   map[int]int64
	latestInviteAt map[int]int64
}

// loadRiskInviteeMembers loads ALL invitees of the given inviters; each
// invitee joins their inviter's group and each inviter joins their own.
func loadRiskInviteeMembers(inviterIds []int) (*riskInviteeScan, error) {
	scan := &riskInviteeScan{
		memberGroups:   make(map[int][]int, len(inviterIds)*2),
		inviteeCount:   make(map[int]int64, len(inviterIds)),
		latestInviteAt: make(map[int]int64, len(inviterIds)),
	}
	type inviteeRow struct {
		Id        int
		InviterId int
		CreatedAt int64
	}
	for _, chunk := range chunkInts(inviterIds, 500) {
		var invitees []inviteeRow
		if err := DB.Model(&User{}).Select("id, inviter_id, created_at").Where("inviter_id IN ?", chunk).Find(&invitees).Error; err != nil {
			return nil, err
		}
		for _, row := range invitees {
			scan.memberGroups[row.Id] = append(scan.memberGroups[row.Id], row.InviterId)
			scan.inviteeCount[row.InviterId]++
			if row.CreatedAt > scan.latestInviteAt[row.InviterId] {
				scan.latestInviteAt[row.InviterId] = row.CreatedAt
			}
		}
	}
	for _, inviterId := range inviterIds {
		scan.memberGroups[inviterId] = append(scan.memberGroups[inviterId], inviterId)
	}
	return scan, nil
}

// riskSharedIPCounts counts, per inviter, the IPs with records from at least
// two distinct group members.
func riskSharedIPCounts(scan *riskInviteeScan) (map[int]int64, error) {
	memberIds := make([]int, 0, len(scan.memberGroups))
	for memberId := range scan.memberGroups {
		memberIds = append(memberIds, memberId)
	}
	type ipRecordRow struct {
		UserId int
		IP     string
	}
	groupIPUsers := make(map[int]map[string]map[int]struct{}, len(scan.memberGroups))
	for _, chunk := range chunkInts(memberIds, 500) {
		var records []ipRecordRow
		if err := DB.Model(&RiskIPRecord{}).Distinct("user_id", "ip").Where("user_id IN ? AND ip <> ''", chunk).Scan(&records).Error; err != nil {
			return nil, err
		}
		for _, record := range records {
			for _, groupId := range scan.memberGroups[record.UserId] {
				byIP := groupIPUsers[groupId]
				if byIP == nil {
					byIP = make(map[string]map[int]struct{})
					groupIPUsers[groupId] = byIP
				}
				users := byIP[record.IP]
				if users == nil {
					users = make(map[int]struct{})
					byIP[record.IP] = users
				}
				users[record.UserId] = struct{}{}
			}
		}
	}
	sharedIPCount := make(map[int]int64, len(scan.memberGroups))
	for groupId, byIP := range groupIPUsers {
		for _, users := range byIP {
			if len(users) > 1 {
				sharedIPCount[groupId]++
			}
		}
	}
	return sharedIPCount, nil
}

// buildRiskInviters loads the displayed invitees (up to 100 per inviter) and
// assembles the API result for the given rows.
func buildRiskInviters(rows []riskInviterRow) ([]*RiskInviter, error) {
	result := make([]*RiskInviter, 0, len(rows))
	for _, row := range rows {
		var invitees []*User
		if err := DB.Select("id, username, display_name, "+commonGroupCol+", role, inviter_id").Where("inviter_id = ?", row.InviterId).Order("id desc").Limit(100).Find(&invitees).Error; err != nil {
			return nil, err
		}
		summaries := make([]*RiskUserSummary, 0, len(invitees))
		for _, invitee := range invitees {
			summaries = append(summaries, &RiskUserSummary{UserId: invitee.Id, Username: invitee.Username, DisplayName: invitee.DisplayName, Group: invitee.Group, Role: invitee.Role, InviterId: invitee.InviterId})
		}
		result = append(result, &RiskInviter{
			InviterId: row.InviterId, Username: row.Username, DisplayName: row.DisplayName, Group: row.Group,
			DirectInviteCount: row.DirectInviteCount, SharedIPCount: row.SharedIPCount,
			Suspicious: row.SharedIPCount > 0, LatestInviteAt: row.LatestInviteAt, Invitees: summaries,
		})
	}
	return result, nil
}

// ListRiskInviters lists users who invited others, ordered by the CreatedAt of
// their most recently invited user (desc) with a stable inviter id (desc)
// tie-break. riskStatus "all" paginates in the database and only evaluates
// shared-IP risk for the current page; "review"/"normal" evaluate all
// candidates first because filtering must run before pagination. Shared IP
// counts always cover the inviter plus ALL invitees; displayed invitees stay
// capped at 100. Any riskStatus other than review/normal behaves as "all".
func ListRiskInviters(keyword string, riskStatus string, startIdx int, limit int) ([]*RiskInviter, int64, error) {
	if limit <= 0 {
		limit = common.ItemsPerPage
	}
	if riskStatus != "review" && riskStatus != "normal" {
		riskStatus = "all"
	}
	if startIdx < 0 {
		startIdx = 0
	}
	inviterSub := DB.Model(&User{}).Where("inviter_id > 0").Distinct("inviter_id")

	if riskStatus == "all" {
		statsSub := DB.Model(&User{}).
			Select("inviter_id, COUNT(*) AS direct_invite_count, MAX(created_at) AS latest_invite_at").
			Where("inviter_id IN (?)", inviterSub).Group("inviter_id")
		pageTx := DB.Model(&User{}).
			Joins("LEFT JOIN (?) AS inv_stats ON inv_stats.inviter_id = users.id", statsSub).
			Where("users.id IN (?)", inviterSub)
		if keyword != "" {
			pattern, err := sanitizeLikePattern("%" + keyword + "%")
			if err != nil {
				return nil, 0, err
			}
			pageTx = pageTx.Where("users.username LIKE ? ESCAPE '!' OR users.display_name LIKE ? ESCAPE '!'", pattern, pattern)
		}
		var total int64
		if err := pageTx.Count(&total).Error; err != nil {
			return nil, 0, err
		}
		var rows []riskInviterRow
		if err := pageTx.Select("users.id AS inviter_id, users.username, users.display_name, users." + commonGroupCol +
			", COALESCE(inv_stats.direct_invite_count, 0) AS direct_invite_count, COALESCE(inv_stats.latest_invite_at, 0) AS latest_invite_at").
			Order("inv_stats.latest_invite_at DESC").Order("users.id DESC").
			Offset(startIdx).Limit(limit).Scan(&rows).Error; err != nil {
			return nil, 0, err
		}
		if len(rows) == 0 {
			return []*RiskInviter{}, total, nil
		}
		pageIds := make([]int, 0, len(rows))
		for _, row := range rows {
			pageIds = append(pageIds, row.InviterId)
		}
		scan, err := loadRiskInviteeMembers(pageIds)
		if err != nil {
			return nil, 0, err
		}
		sharedIPCount, err := riskSharedIPCounts(scan)
		if err != nil {
			return nil, 0, err
		}
		for index := range rows {
			rows[index].SharedIPCount = sharedIPCount[rows[index].InviterId]
		}
		result, err := buildRiskInviters(rows)
		if err != nil {
			return nil, 0, err
		}
		return result, total, nil
	}

	// review/normal: risk must be known for every candidate before pagination
	kw := DB.Model(&User{}).Select("id").Where("id IN (?)", inviterSub)
	if keyword != "" {
		pattern, err := sanitizeLikePattern("%" + keyword + "%")
		if err != nil {
			return nil, 0, err
		}
		kw = kw.Where("username LIKE ? ESCAPE '!' OR display_name LIKE ? ESCAPE '!'", pattern, pattern)
	}
	var inviterIds []int
	if err := kw.Pluck("id", &inviterIds).Error; err != nil {
		return nil, 0, err
	}
	if len(inviterIds) == 0 {
		return []*RiskInviter{}, 0, nil
	}
	scan, err := loadRiskInviteeMembers(inviterIds)
	if err != nil {
		return nil, 0, err
	}
	sharedIPCount, err := riskSharedIPCounts(scan)
	if err != nil {
		return nil, 0, err
	}
	rows := make([]riskInviterRow, 0, len(inviterIds))
	for _, inviterId := range inviterIds {
		rows = append(rows, riskInviterRow{
			InviterId: inviterId, DirectInviteCount: scan.inviteeCount[inviterId],
			LatestInviteAt: scan.latestInviteAt[inviterId], SharedIPCount: sharedIPCount[inviterId],
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].LatestInviteAt != rows[j].LatestInviteAt {
			return rows[i].LatestInviteAt > rows[j].LatestInviteAt
		}
		return rows[i].InviterId > rows[j].InviterId
	})
	kept := make([]riskInviterRow, 0, len(rows))
	for _, row := range rows {
		if (riskStatus == "review" && row.SharedIPCount > 0) || (riskStatus == "normal" && row.SharedIPCount == 0) {
			kept = append(kept, row)
		}
	}
	rows = kept
	total := int64(len(rows))
	if startIdx >= len(rows) {
		return []*RiskInviter{}, total, nil
	}
	pageLen := limit
	if pageLen > len(rows)-startIdx {
		pageLen = len(rows) - startIdx
	}
	pageRows := rows[startIdx : startIdx+pageLen]
	pageIds := make([]int, 0, len(pageRows))
	for _, row := range pageRows {
		pageIds = append(pageIds, row.InviterId)
	}
	if len(pageIds) > 0 {
		var inviterUsers []*User
		if err := DB.Select("id, username, display_name, "+commonGroupCol).Where("id IN ?", pageIds).Find(&inviterUsers).Error; err != nil {
			return nil, 0, err
		}
		byID := make(map[int]*User, len(inviterUsers))
		for _, user := range inviterUsers {
			byID[user.Id] = user
		}
		for index := range pageRows {
			if user := byID[pageRows[index].InviterId]; user != nil {
				pageRows[index].Username = user.Username
				pageRows[index].DisplayName = user.DisplayName
				pageRows[index].Group = user.Group
			}
		}
	}
	result, err := buildRiskInviters(pageRows)
	if err != nil {
		return nil, 0, err
	}
	return result, total, nil
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
