package queryx

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/common/dbx"
	"gorm.io/gorm"
)

func agentPromoLinkSelectFields() string {
	return `apl.id AS promo_link_id, apl.agent_user_id, apl.name, apl.code, apl.status, apl.landing_page,
COALESCE(user_stats.invitee_count, 0) AS invitee_count,
COALESCE(topup_stats.topup_count, 0) AS topup_count,
COALESCE(topup_stats.topup_amount, 0) AS topup_amount,
COALESCE(rebate_stats.rebate_amount, 0) AS rebate_amount,
COALESCE(user_stats.last_invitee_id, 0) AS last_invitee_id,
COALESCE(user_stats.last_invitee_name, '') AS last_invitee_name`
}

func agentPromoLinkUserJoin() string {
	return `LEFT JOIN (
			SELECT u.promo_link_id,
			COUNT(*) AS invitee_count,
			MAX(u.id) AS last_invitee_id,
			MAX(u.username) AS last_invitee_name
			FROM users AS u
			WHERE u.promo_link_id > 0 AND u.deleted_at IS NULL
			GROUP BY u.promo_link_id
		) AS user_stats ON user_stats.promo_link_id = apl.id`
}

func agentPromoLinkTopupJoin() string {
	return fmt.Sprintf(`LEFT JOIN (
			SELECT u.promo_link_id,
			COUNT(t.id) AS topup_count,
			%s AS topup_amount
			FROM users AS u
			LEFT JOIN top_ups AS t ON t.user_id = u.id AND t.status = ?
			WHERE u.promo_link_id > 0 AND u.deleted_at IS NULL
			GROUP BY u.promo_link_id
		) AS topup_stats ON topup_stats.promo_link_id = apl.id`, dbx.SumMoneyCentsExpr("t.money"))
}

func agentPromoLinkRebateJoin() string {
	return `LEFT JOIN (
			SELECT promo_link_id,
			COALESCE(SUM(rebate_amount), 0) AS rebate_amount
			FROM agent_rebate_records
			WHERE promo_link_id > 0 AND status = ?
			GROUP BY promo_link_id
		) AS rebate_stats ON rebate_stats.promo_link_id = apl.id`
}

func BuildAgentPromoLinkStatsQuery(db *gorm.DB, agentUserId int, rebateStatus string) *gorm.DB {
	tx := db.Table("agent_promo_links AS apl").
		Select(agentPromoLinkSelectFields()).
		Joins(agentPromoLinkUserJoin()).
		Joins(agentPromoLinkTopupJoin(), common.TopUpStatusSuccess).
		Joins(agentPromoLinkRebateJoin(), rebateStatus)
	if agentUserId > 0 {
		tx = tx.Where("apl.agent_user_id = ?", agentUserId)
	}
	return tx
}
