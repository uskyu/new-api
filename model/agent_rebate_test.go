package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func ensureAgentTestTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&User{}, &TopUp{}, &AgentRebateGroup{}, &AgentProfile{}, &AgentPromoLink{}, &AgentRebateRecord{}))
	require.NoError(t, DB.AutoMigrate(&AgentRebateAdjustment{}, &AgentRelationship{}, &AgentUpgradeRequest{}))
	t.Cleanup(func() {
		session := DB.Session(&gorm.Session{AllowGlobalUpdate: true})
		_ = session.Delete(&AgentUpgradeRequest{}).Error
		_ = session.Delete(&AgentRelationship{}).Error
		_ = session.Delete(&AgentRebateAdjustment{}).Error
		_ = session.Delete(&AgentRebateRecord{}).Error
		_ = session.Delete(&AgentPromoLink{}).Error
		_ = session.Delete(&AgentProfile{}).Error
		_ = session.Delete(&AgentRebateGroup{}).Error
		_ = session.Delete(&TopUp{}).Error
		_ = session.Delete(&User{}).Error
	})
}

func createAgentTestUser(t *testing.T, username string, affCode string) *User {
	t.Helper()
	user := &User{
		Username:    username,
		Password:    "hashed-password",
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     affCode,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func TestResolveRegistrationAttribution(t *testing.T) {
	ensureAgentTestTables(t)
	agent := createAgentTestUser(t, "agent_user", "AFF1")
	promoLink := &AgentPromoLink{
		AgentUserId: agent.Id,
		Name:        "wechat",
		Code:        "PROMO1",
		Status:      AgentPromoLinkEnabled,
	}
	require.NoError(t, DB.Create(promoLink).Error)

	inviterId, promoLinkId, err := ResolveRegistrationAttribution("PROMO1")
	require.NoError(t, err)
	require.Equal(t, agent.Id, inviterId)
	require.Equal(t, promoLink.Id, promoLinkId)

	inviterId, promoLinkId, err = ResolveRegistrationAttribution("AFF1")
	require.NoError(t, err)
	require.Equal(t, agent.Id, inviterId)
	require.Zero(t, promoLinkId)

	inviterId, promoLinkId, err = ResolveRegistrationAttribution("UNKNOWN")
	require.NoError(t, err)
	require.Zero(t, inviterId)
	require.Zero(t, promoLinkId)
}

func TestSettleAgentRebateTx(t *testing.T) {
	ensureAgentTestTables(t)
	common.AgentEnabled = true
	common.AgentInitialized = true
	common.AgentDefaultRebateRate = 0
	t.Cleanup(func() {
		common.AgentEnabled = false
		common.AgentInitialized = false
		common.AgentDefaultRebateRate = 0
	})

	agent := createAgentTestUser(t, "agent_settle", "AFF2")
	invitee := createAgentTestUser(t, "invitee_settle", "AFF3")
	group := &AgentRebateGroup{
		Name:       "group-a",
		RebateRate: 1500,
		Status:     AgentStatusEnabled,
	}
	require.NoError(t, DB.Create(group).Error)
	promoLink := &AgentPromoLink{
		AgentUserId: agent.Id,
		Name:        "banner",
		Code:        "PROMO2",
		Status:      AgentPromoLinkEnabled,
	}
	require.NoError(t, DB.Create(promoLink).Error)
	require.NoError(t, DB.Model(invitee).Updates(map[string]interface{}{
		"inviter_id":    agent.Id,
		"promo_link_id": promoLink.Id,
	}).Error)
	profile := &AgentProfile{
		UserId:        agent.Id,
		Status:        AgentStatusEnabled,
		RebateGroupId: group.Id,
	}
	require.NoError(t, DB.Create(profile).Error)
	topup := &TopUp{
		UserId:        invitee.Id,
		Amount:        100,
		Money:         100,
		TradeNo:       "trade_agent_settle",
		PaymentMethod: "epay",
		CreateTime:    common.GetTimestamp(),
		Status:        common.TopUpStatusSuccess,
	}
	require.NoError(t, DB.Create(topup).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return SettleAgentRebateTx(tx, topup, AgentRebateSourceEPay)
	}))
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return SettleAgentRebateTx(tx, topup, AgentRebateSourceEPay)
	}))

	updatedProfile, err := GetAgentProfileByUserId(agent.Id)
	require.NoError(t, err)
	require.Equal(t, int64(1500), updatedProfile.RebateBalanceAmount)
	require.Equal(t, int64(1500), updatedProfile.RebateTotalAmount)

	var records []AgentRebateRecord
	require.NoError(t, DB.Find(&records).Error)
	require.Len(t, records, 1)
	require.Equal(t, topup.Id, records[0].TopUpId)
	require.Equal(t, invitee.Id, records[0].InviteeUserId)
	require.Equal(t, agent.Id, records[0].AgentUserId)
	require.Equal(t, promoLink.Id, records[0].PromoLinkId)
	require.Equal(t, int64(10000), records[0].PayAmount)
	require.Equal(t, 1500, records[0].RebateRate)
	require.Equal(t, int64(1500), records[0].RebateAmount)
}

func TestAdjustAgentRebateBalance(t *testing.T) {
	ensureAgentTestTables(t)
	agent := createAgentTestUser(t, "agent_adjust", "AFF4")
	operator := createAgentTestUser(t, "admin_adjust", "AFF5")
	profile := &AgentProfile{
		UserId:              agent.Id,
		Status:              AgentStatusEnabled,
		RebateBalanceAmount: 500,
		RebateTotalAmount:   800,
	}
	require.NoError(t, DB.Create(profile).Error)

	adjustment, err := AdjustAgentRebateBalance(agent.Id, operator.Id, 250, "manual bonus")
	require.NoError(t, err)
	require.Equal(t, int64(500), adjustment.BalanceBefore)
	require.Equal(t, int64(750), adjustment.BalanceAfter)
	require.Equal(t, AgentAdjustmentTypeIncrease, adjustment.ChangeType)

	adjustment, err = AdjustAgentRebateBalance(agent.Id, operator.Id, -100, "manual deduction")
	require.NoError(t, err)
	require.Equal(t, AgentAdjustmentTypeDecrease, adjustment.ChangeType)

	updatedProfile, err := GetAgentProfileByUserId(agent.Id)
	require.NoError(t, err)
	require.Equal(t, int64(650), updatedProfile.RebateBalanceAmount)
	require.Equal(t, int64(1050), updatedProfile.RebateTotalAmount)

	_, err = AdjustAgentRebateBalance(agent.Id, operator.Id, -1000, "overflow")
	require.Error(t, err)

	var adjustments []AgentRebateAdjustment
	require.NoError(t, DB.Order("id asc").Find(&adjustments).Error)
	require.Len(t, adjustments, 2)
}

func TestUpsertAndDeleteAgentPromoLink(t *testing.T) {
	ensureAgentTestTables(t)
	agent := createAgentTestUser(t, "agent_promo", "AFF6")
	operator := createAgentTestUser(t, "admin_promo", "AFF7")
	require.NoError(t, DB.Create(&AgentProfile{UserId: agent.Id, Status: AgentStatusEnabled}).Error)

	promoLink, err := UpsertAgentPromoLink(operator.Id, &AgentPromoLink{
		AgentUserId: agent.Id,
		Name:        "homepage",
		Status:      AgentPromoLinkEnabled,
		LandingPage: "/",
	})
	require.NoError(t, err)
	require.NotZero(t, promoLink.Id)
	require.NotEmpty(t, promoLink.Code)

	promoLink, err = UpsertAgentPromoLink(operator.Id, &AgentPromoLink{
		Id:          promoLink.Id,
		AgentUserId: agent.Id,
		Name:        "homepage-updated",
		Code:        promoLink.Code,
		Status:      AgentPromoLinkDisabled,
		LandingPage: "/landing",
	})
	require.NoError(t, err)
	require.Equal(t, "homepage-updated", promoLink.Name)
	require.Equal(t, AgentPromoLinkDisabled, promoLink.Status)

	links, total, err := GetAgentPromoLinks(&common.PageInfo{Page: 1, PageSize: 10}, agent.Id, "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, links, 1)
	require.Equal(t, agent.Id, links[0].AgentUserId)

	require.NoError(t, DeleteAgentPromoLink(promoLink.Id, operator.Id))
	links, total, err = GetAgentPromoLinks(&common.PageInfo{Page: 1, PageSize: 10}, agent.Id, "")
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, links)
}

func TestUpsertAgentProfileAutoCreatesDefaultPromoLink(t *testing.T) {
	ensureAgentTestTables(t)
	operator := createAgentTestUser(t, "admin_auto_link", "AFF11")
	agent := createAgentTestUser(t, "agent_auto_link", "AFF12")

	profile, err := UpsertAgentProfile(operator.Id, &AgentProfile{
		UserId: agent.Id,
		Status: AgentStatusEnabled,
		Remark: "auto link",
	})
	require.NoError(t, err)
	require.NotNil(t, profile)

	links, total, err := GetAgentPromoLinks(&common.PageInfo{Page: 1, PageSize: 10}, agent.Id, "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, links, 1)
	require.Equal(t, "default", links[0].Name)
}

func TestUpsertAndDeleteAgentRebateGroup(t *testing.T) {
	ensureAgentTestTables(t)
	operator := createAgentTestUser(t, "admin_group", "AFF8")

	group, err := UpsertAgentRebateGroup(operator.Id, &AgentRebateGroup{
		Name:       "vip-agent",
		RebateRate: 1800,
		Status:     AgentStatusEnabled,
		Remark:     "vip",
	})
	require.NoError(t, err)
	require.NotZero(t, group.Id)

	group, err = UpsertAgentRebateGroup(operator.Id, &AgentRebateGroup{
		Id:         group.Id,
		Name:       "vip-agent-updated",
		RebateRate: 2200,
		Status:     AgentStatusEnabled,
	})
	require.NoError(t, err)
	require.Equal(t, 2200, group.RebateRate)

	groups, err := GetAllAgentRebateGroups()
	require.NoError(t, err)
	require.Len(t, groups, 1)

	require.NoError(t, DeleteAgentRebateGroup(group.Id, operator.Id))
	groups, err = GetAllAgentRebateGroups()
	require.NoError(t, err)
	require.Empty(t, groups)
}

func TestAgentPromoLinkStatsAndDownlines(t *testing.T) {
	ensureAgentTestTables(t)
	common.AgentEnabled = true
	common.AgentInitialized = true
	t.Cleanup(func() {
		common.AgentEnabled = false
		common.AgentInitialized = false
	})
	agent := createAgentTestUser(t, "agent_stats", "AFF9")
	invitee := createAgentTestUser(t, "invitee_stats", "AFF10")
	require.NoError(t, DB.Create(&AgentProfile{UserId: agent.Id, Status: AgentStatusEnabled, CustomRate: 1200}).Error)
	promoLink, err := UpsertAgentPromoLink(agent.Id, &AgentPromoLink{
		AgentUserId: agent.Id,
		Name:        "landing-home",
		Code:        "PROMOSTAT",
		Status:      AgentPromoLinkEnabled,
		LandingPage: "/",
	})
	require.NoError(t, err)
	require.NoError(t, DB.Model(invitee).Updates(map[string]interface{}{
		"inviter_id":    agent.Id,
		"promo_link_id": promoLink.Id,
	}).Error)
	topup := &TopUp{
		UserId:       invitee.Id,
		Amount:       88,
		Money:        88,
		TradeNo:      "trade_stats_1",
		CreateTime:   common.GetTimestamp(),
		CompleteTime: common.GetTimestamp(),
		Status:       common.TopUpStatusSuccess,
	}
	require.NoError(t, DB.Create(topup).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return SettleAgentRebateTx(tx, topup, AgentRebateSourceEPay)
	}))

	stats, err := GetAgentPromoLinkStats(agent.Id)
	require.NoError(t, err)
	var targetStat *AgentPromoLinkStat
	for _, stat := range stats {
		if stat.PromoLinkId == promoLink.Id {
			targetStat = stat
			break
		}
	}
	require.NotNil(t, targetStat)
	require.Equal(t, int64(1), targetStat.InviteeCount)
	require.Equal(t, int64(1), targetStat.TopupCount)
	require.Equal(t, int64(8800), targetStat.TopupAmount)
	require.Equal(t, int64(1056), targetStat.RebateAmount)

	downlines, total, err := GetAgentDownlineUsers(&common.PageInfo{Page: 1, PageSize: 10}, agent.Id, "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, downlines, 1)
	require.Equal(t, invitee.Id, downlines[0].UserId)
	require.Equal(t, int64(8800), downlines[0].TopupAmount)
	require.Equal(t, int64(1056), downlines[0].RebateAmount)
}

func TestAgentUpgradeRequestAndRateConflict(t *testing.T) {
	ensureAgentTestTables(t)
	operator := createAgentTestUser(t, "admin_phase2", "AFF13")
	parent := createAgentTestUser(t, "parent_agent", "AFF14")
	child := createAgentTestUser(t, "child_agent", "AFF15")
	group := &AgentRebateGroup{Name: "parent-group", RebateRate: 3000, Status: AgentStatusEnabled}
	require.NoError(t, DB.Create(group).Error)
	_, err := UpsertAgentProfile(operator.Id, &AgentProfile{UserId: parent.Id, Status: AgentStatusEnabled, RebateGroupId: group.Id})
	require.NoError(t, err)
	require.NoError(t, DB.Model(child).Update("inviter_id", parent.Id).Error)

	request, err := CreateAgentUpgradeRequest(parent.Id, child.Id, 2500, "upgrade child")
	require.NoError(t, err)
	require.Equal(t, AgentUpgradeRequestPending, request.Status)

	approved, err := ReviewAgentUpgradeRequest(request.Id, operator.Id, true, 2500, "approved")
	require.NoError(t, err)
	require.Equal(t, AgentUpgradeRequestApproved, approved.Status)

	childProfile, err := GetAgentProfileByUserId(child.Id)
	require.NoError(t, err)
	require.Equal(t, 2500, childProfile.CustomRate)

	relation, err := getAgentRelationshipByChildTx(DB, child.Id)
	require.NoError(t, err)
	require.Equal(t, parent.Id, relation.ParentAgentUserId)

	_, err = UpsertAgentRebateGroup(operator.Id, &AgentRebateGroup{Id: group.Id, Name: group.Name, RebateRate: 2000, Status: AgentStatusEnabled})
	require.Error(t, err)
	var conflictErr *AgentRateConflictError
	require.ErrorAs(t, err, &conflictErr)
	require.NotEmpty(t, conflictErr.Conflicts)
}
