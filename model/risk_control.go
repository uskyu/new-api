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

func ListRiskInviters(keyword string, startIdx int, limit int) ([]*RiskInviter, int64, error) {
	if limit <= 0 {
		limit = common.ItemsPerPage
	}
	var inviterIds []int
	if err := DB.Model(&User{}).Where("inviter_id > 0").Distinct("inviter_id").Pluck("inviter_id", &inviterIds).Error; err != nil {
		return nil, 0, err
	}
	if len(inviterIds) == 0 {
		return []*RiskInviter{}, 0, nil
	}
	tx := DB.Model(&User{}).Where("id IN ?", inviterIds)
	if keyword != "" {
		pattern, err := sanitizeLikePattern("%" + keyword + "%")
		if err != nil {
			return nil, 0, err
		}
		tx = tx.Where("username LIKE ? ESCAPE '!' OR display_name LIKE ? ESCAPE '!'", pattern, pattern)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var inviters []*User
	if err := tx.Select("id, username, display_name, " + commonGroupCol).Order("id desc").Offset(startIdx).Limit(limit).Find(&inviters).Error; err != nil {
		return nil, 0, err
	}
	result := make([]*RiskInviter, 0, len(inviters))
	for _, inviter := range inviters {
		var directInviteCount int64
		if err := DB.Model(&User{}).Where("inviter_id = ?", inviter.Id).Count(&directInviteCount).Error; err != nil {
			return nil, 0, err
		}
		var invitees []*User
		if err := DB.Select("id, username, display_name, "+commonGroupCol+", role, inviter_id").Where("inviter_id = ?", inviter.Id).Order("id desc").Limit(100).Find(&invitees).Error; err != nil {
			return nil, 0, err
		}
		ids := make([]int, 0, len(invitees)+1)
		ids = append(ids, inviter.Id)
		for _, invitee := range invitees {
			ids = append(ids, invitee.Id)
		}
		sharedIPs, err := countSharedIPsForUsers(ids)
		if err != nil {
			return nil, 0, err
		}
		summaries := make([]*RiskUserSummary, 0, len(invitees))
		for _, invitee := range invitees {
			summaries = append(summaries, &RiskUserSummary{UserId: invitee.Id, Username: invitee.Username, DisplayName: invitee.DisplayName, Group: invitee.Group, Role: invitee.Role, InviterId: invitee.InviterId})
		}
		result = append(result, &RiskInviter{InviterId: inviter.Id, Username: inviter.Username, DisplayName: inviter.DisplayName, Group: inviter.Group, DirectInviteCount: directInviteCount, SharedIPCount: sharedIPs, Suspicious: sharedIPs > 0, Invitees: summaries})
	}
	return result, total, nil
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
