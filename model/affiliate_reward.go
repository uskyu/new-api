package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm"
)

// AffiliateReward 记录「一位邀请人 × 一位被邀请人」的邀请奖励台账。
// 该表是增量记账：功能上线后新注册的邀请关系才写记录；
// 上线前的历史邀请额度没有台账，一律按 100% 可划转（legacy）处理，不做数据回填。
type AffiliateReward struct {
	Id                     int   `json:"id"`
	InviterId              int   `json:"inviter_id" gorm:"index"`
	InviteeId              int   `json:"invitee_id" gorm:"uniqueIndex"`
	InviterQuota           int   `json:"inviter_quota"`          // 注册当时 QuotaForInviter
	InviteeQuota           int   `json:"invitee_quota"`          // 注册当时 QuotaForInvitee
	InviteeRewardGranted   bool  `json:"invitee_reward_granted"` // 被邀请人奖励是否已发放
	InviteeRewardGrantedAt int64 `json:"invitee_reward_granted_at"`
	InviterWithdrawn       int   `json:"inviter_withdrawn"`        // 该记录已划转额度
	UnlockedAt             int64 `json:"unlocked_at" gorm:"index"` // 被邀请人首次充值时间，0=未解锁
	CreatedAt              int64 `json:"created_at" gorm:"autoCreateTime"`
}

// AffiliateRule 对外返回的当前生效规则快照。
type AffiliateRule struct {
	InviteeRewardRequireRecharge          bool `json:"invitee_reward_require_recharge"`
	InviterWithdrawRequireInviteeRecharge bool `json:"inviter_withdraw_require_invitee_recharge"`
	UnrechargedWithdrawRatio              int  `json:"unrecharged_withdraw_ratio"`
	RechargedWithdrawRatio                int  `json:"recharged_withdraw_ratio"`
}

// AffiliateWithdrawable 邀请额度的可划转/锁定明细。
type AffiliateWithdrawable struct {
	Withdrawable         int           // 当前可划转
	Locked               int           // 待解锁（按未充值比例折算后可划转的那部分之外的锁定额）
	UnlockedRemaining    int           // 已解锁池剩余
	LockedRemaining      int           // 未解锁池按比例可划转的剩余
	LegacyRemaining      int           // 无台账的历史额度（100% 可划转）
	LockedTotal          int           // 未解锁记录的原始额度合计（用于展示）
	PendingInviteeReward int           // 因要求充值而尚未发放的被邀请人奖励合计
	Rule                 AffiliateRule // 当前生效规则
}

// createAffiliateRewardTx 为一次邀请关系创建台账记录。
// 若该被邀请人已有台账（异常重复注册）则跳过，避免破坏 invitee_id 唯一约束。
func createAffiliateRewardTx(tx *gorm.DB, inviterId int, inviteeId int) error {
	if inviterId == 0 {
		return errors.New("inviterId 为空！")
	}
	if inviteeId == 0 {
		return errors.New("inviteeId 为空！")
	}

	var exists int64
	if err := tx.Model(&AffiliateReward{}).Where("invitee_id = ?", inviteeId).Count(&exists).Error; err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}

	reward := &AffiliateReward{
		InviterId:    inviterId,
		InviteeId:    inviteeId,
		InviterQuota: common.QuotaForInviter,
		InviteeQuota: common.QuotaForInvitee,
	}
	if common.QuotaForInvitee > 0 && !operation_setting.GetAffiliateSetting().InviteeRewardRequireRecharge {
		reward.InviteeRewardGranted = true
		reward.InviteeRewardGrantedAt = common.GetTimestamp()
	}
	return tx.Create(reward).Error
}

// currentAffiliateRule 返回当前规则快照（原始配置值，不做门控折算）。
func currentAffiliateRule() AffiliateRule {
	setting := operation_setting.GetAffiliateSetting()
	return AffiliateRule{
		InviteeRewardRequireRecharge:          setting.InviteeRewardRequireRecharge,
		InviterWithdrawRequireInviteeRecharge: setting.InviterWithdrawRequireInviteeRecharge,
		UnrechargedWithdrawRatio:              setting.UnrechargedWithdrawRatio,
		RechargedWithdrawRatio:                setting.RechargedWithdrawRatio,
	}
}

// computeAffiliateWithdrawable 按契约口径计算可划转额度。
//
//	records = AffiliateReward where inviter_id = userId
//	unlockedRemaining = Σ max(0, InviterQuota - InviterWithdrawn)   for unlocked_at > 0
//	lockedLimit       = floor(Σ InviterQuota for unlocked_at = 0 * LockedRatio / 100)
//	lockedWithdrawn   = Σ InviterWithdrawn                          for unlocked_at = 0
//	lockedRemaining   = max(0, lockedLimit - lockedWithdrawn)
//	ledgerRemaining   = Σ max(0, InviterQuota - InviterWithdrawn)    for all records
//	legacyRemaining   = max(0, user.AffQuota - ledgerRemaining)
//	withdrawable      = legacyRemaining + unlockedRemaining + lockedRemaining
//	locked            = max(0, user.AffQuota - withdrawable)
func computeAffiliateWithdrawable(affQuota int, records []AffiliateReward) *AffiliateWithdrawable {
	lockedRatio := operation_setting.GetAffiliateSetting().LockedRatio()

	unlockedRemaining := 0
	lockedQuotaTotal := 0
	lockedWithdrawn := 0
	ledgerRemaining := 0
	pendingInviteeReward := 0

	for i := range records {
		record := records[i]
		remaining := record.InviterQuota - record.InviterWithdrawn
		if remaining < 0 {
			remaining = 0
		}
		ledgerRemaining += remaining

		if record.UnlockedAt > 0 {
			unlockedRemaining += remaining
		} else {
			lockedQuotaTotal += record.InviterQuota
			lockedWithdrawn += record.InviterWithdrawn
		}

		if !record.InviteeRewardGranted && record.InviteeQuota > 0 {
			pendingInviteeReward += record.InviteeQuota
		}
	}

	// floor(lockedQuotaTotal * lockedRatio / 100)，非负整数除法即为向下取整。
	lockedLimit := lockedQuotaTotal * lockedRatio / 100
	lockedRemaining := lockedLimit - lockedWithdrawn
	if lockedRemaining < 0 {
		lockedRemaining = 0
	}

	legacyRemaining := affQuota - ledgerRemaining
	if legacyRemaining < 0 {
		legacyRemaining = 0
	}

	withdrawable := legacyRemaining + unlockedRemaining + lockedRemaining
	// 异常数据（台账合计大于 aff_quota）时不允许划转超额，保证 AffQuota 不会变负。
	if withdrawable > affQuota {
		withdrawable = affQuota
	}
	if withdrawable < 0 {
		withdrawable = 0
	}

	locked := affQuota - withdrawable
	if locked < 0 {
		locked = 0
	}

	return &AffiliateWithdrawable{
		Withdrawable:         withdrawable,
		Locked:               locked,
		UnlockedRemaining:    unlockedRemaining,
		LockedRemaining:      lockedRemaining,
		LegacyRemaining:      legacyRemaining,
		LockedTotal:          lockedQuotaTotal,
		PendingInviteeReward: pendingInviteeReward,
		Rule:                 currentAffiliateRule(),
	}
}

// GetAffiliateWithdrawable 返回某位邀请人当前的邀请额度可划转明细。
func GetAffiliateWithdrawable(userId int) (*AffiliateWithdrawable, error) {
	if userId == 0 {
		return nil, errors.New("user id 为空！")
	}
	user, err := GetUserById(userId, false)
	if err != nil {
		return nil, err
	}
	records := make([]AffiliateReward, 0)
	if err := DB.Where("inviter_id = ?", userId).Order("id asc").Find(&records).Error; err != nil {
		return nil, err
	}
	return computeAffiliateWithdrawable(user.AffQuota, records), nil
}

// allocateAffiliateWithdrawal 在事务内按「先已解锁池、再未解锁池」扣减台账。
// legacy 额度不落台账，因此扣不完的剩余部分不回写。
func allocateAffiliateWithdrawal(tx *gorm.DB, records []AffiliateReward, quota int, lockedBudget int) error {
	remaining := quota

	apply := func(record *AffiliateReward, limit int) error {
		if remaining <= 0 || limit <= 0 {
			return nil
		}
		available := record.InviterQuota - record.InviterWithdrawn
		if available > limit {
			available = limit
		}
		if available <= 0 {
			return nil
		}
		deduct := available
		if deduct > remaining {
			deduct = remaining
		}
		record.InviterWithdrawn += deduct
		remaining -= deduct
		return tx.Model(&AffiliateReward{}).
			Where("id = ?", record.Id).
			Update("inviter_withdrawn", record.InviterWithdrawn).Error
	}

	// 1) 已解锁池：按 id 升序逐条扣，单条不超其未划转额度
	for i := range records {
		if remaining <= 0 {
			break
		}
		if records[i].UnlockedAt == 0 {
			continue
		}
		if err := apply(&records[i], records[i].InviterQuota); err != nil {
			return err
		}
	}

	// 2) 未解锁池：按 id 升序逐条扣，整体不超按比例折算出的 lockedRemaining 预算
	budget := lockedBudget
	for i := range records {
		if remaining <= 0 || budget <= 0 {
			break
		}
		if records[i].UnlockedAt > 0 {
			continue
		}
		before := records[i].InviterWithdrawn
		if err := apply(&records[i], budget); err != nil {
			return err
		}
		budget -= records[i].InviterWithdrawn - before
	}

	return nil
}

type affiliateGrant struct {
	userId int
	quota  int
}

// settleAffiliateRewardsInTx 在事务内解锁邀请奖励，返回本次实际发放的被邀请人奖励。
// CAS 条件（WHERE unlocked_at = 0 / invitee_reward_granted = false）保证并发与重复调用幂等。
func settleAffiliateRewardsInTx(tx *gorm.DB, userId int) ([]affiliateGrant, error) {
	grants := make([]affiliateGrant, 0)
	if userId == 0 {
		return grants, errors.New("user id 为空！")
	}

	// 先锁用户行、再锁台账行：与 TransferAffQuotaToQuota（同样先锁用户再改台账）
	// 保持一致的加锁顺序，避免两个事务交叉加锁造成死锁。
	lockedUser := &User{}
	if err := lockForUpdate(tx).Where("id = ?", userId).First(lockedUser).Error; err != nil {
		return grants, err
	}

	rewards := make([]AffiliateReward, 0)
	if err := lockForUpdate(tx).
		Where("invitee_id = ? AND unlocked_at = 0", userId).
		Order("id asc").
		Find(&rewards).Error; err != nil {
		return grants, err
	}
	if len(rewards) == 0 {
		return grants, nil
	}

	now := common.GetTimestamp()
	inviteeRewardRequireRecharge := operation_setting.GetAffiliateSetting().InviteeRewardRequireRecharge

	for i := range rewards {
		reward := &rewards[i]
		unlock := tx.Model(&AffiliateReward{}).
			Where("id = ? AND unlocked_at = 0", reward.Id).
			Updates(map[string]interface{}{"unlocked_at": now})
		if unlock.Error != nil {
			return grants, unlock.Error
		}
		if unlock.RowsAffected == 0 {
			// 已被并发调用解锁，解锁与奖励发放由那次调用负责
			continue
		}
		if reward.InviteeRewardGranted || reward.InviteeQuota <= 0 || !inviteeRewardRequireRecharge {
			continue
		}
		grant := tx.Model(&AffiliateReward{}).
			Where("id = ? AND invitee_reward_granted = ?", reward.Id, false).
			Updates(map[string]interface{}{
				"invitee_reward_granted":    true,
				"invitee_reward_granted_at": now,
			})
		if grant.Error != nil {
			return grants, grant.Error
		}
		if grant.RowsAffected == 0 {
			continue
		}
		if err := tx.Model(&User{}).
			Where("id = ?", userId).
			Update("quota", gorm.Expr("quota + ?", reward.InviteeQuota)).Error; err != nil {
			return grants, err
		}
		grants = append(grants, affiliateGrant{userId: userId, quota: reward.InviteeQuota})
	}

	return grants, nil
}

// settleAffiliateRewardsTx 事务版本，供已在事务内的调用点使用。
func settleAffiliateRewardsTx(tx *gorm.DB, userId int) error {
	_, err := settleAffiliateRewardsInTx(tx, userId)
	return err
}

// SettleAffiliateRewards 在任何一次「真实充值成功」后调用，解锁该用户的邀请奖励。
// 幂等：重复调用不重复解锁、不重复发放被邀请人奖励。失败只记录日志，不影响充值主流程。
func SettleAffiliateRewards(userId int) error {
	if userId == 0 {
		return errors.New("user id 为空！")
	}

	var grants []affiliateGrant
	err := DB.Transaction(func(tx *gorm.DB) error {
		var txErr error
		grants, txErr = settleAffiliateRewardsInTx(tx, userId)
		return txErr
	})
	if err != nil {
		return err
	}

	// 日志在事务提交后写，避免独立日志库在事务内写造成死锁/回滚残留。
	for _, grant := range grants {
		RecordLog(grant.userId, LogTypeSystem, fmt.Sprintf("邀请奖励解锁：被邀请人充值后发放 %s", logger.LogQuota(grant.quota)))
	}
	return nil
}

// settleAffiliateRewardsAfterTopUp 供充值链路使用：失败只记录日志，绝不影响充值结果。
func settleAffiliateRewardsAfterTopUp(userId int) {
	if userId == 0 {
		return
	}
	if err := SettleAffiliateRewards(userId); err != nil {
		common.SysLog("failed to settle affiliate rewards: " + err.Error())
	}
}
