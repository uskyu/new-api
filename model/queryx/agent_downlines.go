package queryx

import (
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/QuantumNous/new-api/common/dbx"
)

func agentDownlineSelectFields() string {
	return fmt.Sprintf(`u.id AS user_id, u.username, u.display_name, u.inviter_id, u.promo_link_id, u.quota AS balance_quota,
COALESCE(apl.name, '') AS promo_link_name,
CASE WHEN child_profile.user_id IS NULL THEN %s ELSE %s END AS is_agent,
COALESCE(topup_stats.topup_count, 0) AS topup_count,
COALESCE(topup_stats.topup_amount, 0) AS topup_amount,
COALESCE(rebate_stats.rebate_amount, 0) AS rebate_amount,
COALESCE(topup_stats.latest_topup_time, 0) AS latest_topup_time,
COALESCE(rebate_stats.latest_rebate_time, 0) AS latest_rebate_time`,
		dbx.BoolFalseLiteral(), dbx.BoolTrueLiteral())
}

func agentDownlineTopupJoin() string {
	return `LEFT JOIN (
			SELECT invitee_user_id,
			agent_user_id,
			COUNT(id) AS topup_count,
			COALESCE(SUM(pay_amount), 0) AS topup_amount,
			MAX(settled_at) AS latest_topup_time
			FROM agent_rebate_records
			WHERE status = ?
			GROUP BY invitee_user_id, agent_user_id
		) AS topup_stats ON topup_stats.invitee_user_id = u.id AND topup_stats.agent_user_id = ?`
}

func agentDownlineRebateJoin() string {
	return `LEFT JOIN (
			SELECT invitee_user_id,
			agent_user_id,
			COALESCE(SUM(rebate_amount), 0) AS rebate_amount,
			MAX(settled_at) AS latest_rebate_time
			FROM (
				SELECT invitee_user_id, agent_user_id, rebate_amount, settled_at
				FROM agent_rebate_records
				WHERE status = ?
				UNION ALL
				SELECT invitee_user_id, agent_user_id, rebate_amount, settled_at
				FROM agent_redemption_rebate_records
				WHERE status = ?
			) AS rebate_union
			GROUP BY invitee_user_id, agent_user_id
		) AS rebate_stats ON rebate_stats.invitee_user_id = u.id AND rebate_stats.agent_user_id = ?`
}

func applyDownlineKeywordFilter(tx *gorm.DB, keyword string) *gorm.DB {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return tx
	}
	like := "%" + keyword + "%"
	if keywordInt, err := strconv.Atoi(keyword); err == nil {
		return tx.Where("u.id = ? OR u.username LIKE ? OR u.display_name LIKE ?", keywordInt, like, like)
	}
	return tx.Where("u.username LIKE ? OR u.display_name LIKE ?", like, like)
}

func BuildAgentDownlineUsersQuery(db *gorm.DB, agentUserId int, keyword string, recentSince int64, childAgentStatus int, rebateStatus string) *gorm.DB {
	tx := db.Table("users AS u").
		Select(agentDownlineSelectFields()).
		Joins("LEFT JOIN agent_promo_links AS apl ON apl.id = u.promo_link_id").
		Joins("LEFT JOIN agent_profiles AS child_profile ON child_profile.user_id = u.id AND child_profile.status = ?", childAgentStatus).
		Joins(agentDownlineTopupJoin(), rebateStatus, agentUserId).
		Joins(agentDownlineRebateJoin(), rebateStatus, rebateStatus, agentUserId).
		Where("u.inviter_id = ? AND u.deleted_at IS NULL", agentUserId)
	if strings.TrimSpace(keyword) == "" && recentSince > 0 {
		tx = tx.Where("u.created_at >= ?", recentSince)
	}
	tx = applyDownlineKeywordFilter(tx, keyword)
	return tx
}
