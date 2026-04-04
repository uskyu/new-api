package queryx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/common/dbx"
	"gorm.io/gorm"
)

func agentDownlineSelectFields() string {
	return fmt.Sprintf(`u.id AS user_id, u.username, u.display_name, u.inviter_id, u.promo_link_id,
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
	return fmt.Sprintf(`LEFT JOIN (
			SELECT user_id,
			COUNT(id) AS topup_count,
			%s AS topup_amount,
			MAX(complete_time) AS latest_topup_time
			FROM top_ups
			WHERE status = ?
			GROUP BY user_id
		) AS topup_stats ON topup_stats.user_id = u.id`, dbx.SumMoneyCentsExpr("money"))
}

func agentDownlineRebateJoin() string {
	return `LEFT JOIN (
			SELECT invitee_user_id,
			COALESCE(SUM(rebate_amount), 0) AS rebate_amount,
			MAX(settled_at) AS latest_rebate_time
			FROM agent_rebate_records
			WHERE status = ?
			GROUP BY invitee_user_id
		) AS rebate_stats ON rebate_stats.invitee_user_id = u.id`
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

func BuildAgentDownlineUsersQuery(db *gorm.DB, agentUserId int, keyword string, childAgentStatus int, rebateStatus string) *gorm.DB {
	tx := db.Table("users AS u").
		Select(agentDownlineSelectFields()).
		Joins("LEFT JOIN agent_promo_links AS apl ON apl.id = u.promo_link_id").
		Joins("LEFT JOIN agent_profiles AS child_profile ON child_profile.user_id = u.id AND child_profile.status = ?", childAgentStatus).
		Joins(agentDownlineTopupJoin(), common.TopUpStatusSuccess).
		Joins(agentDownlineRebateJoin(), rebateStatus).
		Where("u.inviter_id = ? AND u.deleted_at IS NULL", agentUserId)
	tx = applyDownlineKeywordFilter(tx, keyword)
	return tx
}
