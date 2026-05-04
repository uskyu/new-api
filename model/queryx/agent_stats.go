package queryx

import "gorm.io/gorm"

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
	return `LEFT JOIN (
			SELECT promo_link_id,
			COUNT(id) AS topup_count,
			COALESCE(SUM(pay_amount), 0) AS topup_amount
			FROM agent_rebate_records
			WHERE promo_link_id > 0 AND status = ?
			GROUP BY promo_link_id
		) AS topup_stats ON topup_stats.promo_link_id = apl.id`
}

func agentPromoLinkRebateJoin() string {
	return `LEFT JOIN (
			SELECT promo_link_id,
			COALESCE(SUM(rebate_amount), 0) AS rebate_amount
			FROM (
				SELECT promo_link_id, rebate_amount
				FROM agent_rebate_records
				WHERE promo_link_id > 0 AND status = ?
				UNION ALL
				SELECT promo_link_id, rebate_amount
				FROM agent_redemption_rebate_records
				WHERE promo_link_id > 0 AND status = ?
			) AS rebate_union
			GROUP BY promo_link_id
		) AS rebate_stats ON rebate_stats.promo_link_id = apl.id`
}

func BuildAgentPromoLinkStatsQuery(db *gorm.DB, agentUserId int, rebateStatus string) *gorm.DB {
	tx := db.Table("agent_promo_links AS apl").
		Select(agentPromoLinkSelectFields()).
		Joins(agentPromoLinkUserJoin()).
		Joins(agentPromoLinkTopupJoin(), rebateStatus).
		Joins(agentPromoLinkRebateJoin(), rebateStatus, rebateStatus)
	if agentUserId > 0 {
		tx = tx.Where("apl.agent_user_id = ?", agentUserId)
	}
	return tx
}
