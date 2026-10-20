package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// AffiliateSetting 邀请奖励后台调控配置。
//
// 默认值（两个开关 false、两个比例 100）代表「与旧行为完全一致」：
// 被邀请人注册即拿奖励、邀请人的邀请额度立即可全额划转。
type AffiliateSetting struct {
	// 被邀请人奖励是否需其本人充值后才发放（关闭=注册即发，默认关闭）
	InviteeRewardRequireRecharge bool `json:"invitee_reward_require_recharge"`
	// 邀请人可提现邀请额度是否需被邀请人充值后解锁（关闭=立即可提，默认关闭）
	InviterWithdrawRequireInviteeRecharge bool `json:"inviter_withdraw_require_invitee_recharge"`
	// 未充值（未解锁）时可划转比例 0-100
	UnrechargedWithdrawRatio int `json:"unrecharged_withdraw_ratio"`
	// 已充值（已解锁）时可划转比例 0-100
	RechargedWithdrawRatio int `json:"recharged_withdraw_ratio"`
}

// 默认配置：全部关闭 + 100%，等价于功能上线前的行为。
var affiliateSetting = AffiliateSetting{
	InviteeRewardRequireRecharge:          false,
	InviterWithdrawRequireInviteeRecharge: false,
	UnrechargedWithdrawRatio:              100,
	RechargedWithdrawRatio:                100,
}

func init() {
	// 注册到全局配置管理器，ExportAllConfigs 会自动生成 affiliate_setting.* 配置项
	config.GlobalConfig.Register("affiliate_setting", &affiliateSetting)
}

// GetAffiliateSetting 获取邀请奖励配置
func GetAffiliateSetting() *AffiliateSetting {
	return &affiliateSetting
}

// clampRatio 将比例限制在 0..100
func clampRatio(ratio int) int {
	if ratio < 0 {
		return 0
	}
	if ratio > 100 {
		return 100
	}
	return ratio
}

// UnlockedRatio 返回「被邀请人已充值（已解锁）」时可划转比例。
// 当 InviterWithdrawRequireInviteeRecharge 关闭时强制 100%，等价旧行为。
func (s *AffiliateSetting) UnlockedRatio() int {
	if s == nil || !s.InviterWithdrawRequireInviteeRecharge {
		return 100
	}
	return clampRatio(s.RechargedWithdrawRatio)
}

// LockedRatio 返回「被邀请人未充值（未解锁）」时可划转比例。
// 当 InviterWithdrawRequireInviteeRecharge 关闭时强制 100%，等价旧行为。
func (s *AffiliateSetting) LockedRatio() int {
	if s == nil || !s.InviterWithdrawRequireInviteeRecharge {
		return 100
	}
	return clampRatio(s.UnrechargedWithdrawRatio)
}
