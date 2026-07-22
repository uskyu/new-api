package model

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyAgentUser struct {
	Id       int    `gorm:"primaryKey"`
	Username string `gorm:"type:varchar(64);uniqueIndex"`
}

func (legacyAgentUser) TableName() string {
	return "users"
}

func setupAgentBackendTest(t *testing.T) {
	t.Helper()
	previousDB := DB
	previousLogDB := LOG_DB
	previousType := common.MainDatabaseType()
	previousAgentEnabled := common.AgentEnabled
	previousAgentInitialized := common.AgentInitialized
	previousDefaultRate := common.AgentDefaultRebateRate
	common.OptionMapRWMutex.Lock()
	previousOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"AgentEnabled":           "true",
		"AgentInitialized":       "true",
		"AgentDefaultRebateRate": "0",
	}
	common.OptionMapRWMutex.Unlock()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.AgentEnabled = true
	common.AgentInitialized = true
	common.AgentDefaultRebateRate = 0
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &Redemption{}, &Log{}))
	require.NoError(t, MigrateAgentSchema())

	t.Cleanup(func() {
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
		DB = previousDB
		LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousType)
		common.AgentEnabled = previousAgentEnabled
		common.AgentInitialized = previousAgentInitialized
		common.AgentDefaultRebateRate = previousDefaultRate
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
	})
}

func createAgentBackendTestUser(t *testing.T, username string) *User {
	t.Helper()
	user := &User{
		Username:    username,
		Password:    "hashed-password",
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "aff-" + username,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func TestResolveRegistrationAttributionSupportsAgentAndLegacyLinks(t *testing.T) {
	setupAgentBackendTest(t)
	agent := createAgentBackendTestUser(t, "registration-agent")
	legacyInviter := createAgentBackendTestUser(t, "registration-legacy")
	require.NoError(t, DB.Create(&AgentProfile{UserId: agent.Id, Status: AgentStatusEnabled}).Error)
	promoLink := &AgentPromoLink{
		AgentUserId: agent.Id,
		Name:        "campaign",
		Code:        "AGENTLINK",
		Status:      AgentPromoLinkEnabled,
	}
	require.NoError(t, DB.Create(promoLink).Error)

	inviterId, promoLinkId, err := ResolveRegistrationAttribution(promoLink.Code)
	require.NoError(t, err)
	assert.Equal(t, agent.Id, inviterId)
	assert.Equal(t, promoLink.Id, promoLinkId)

	inviterId, promoLinkId, err = ResolveRegistrationAttribution(legacyInviter.AffCode)
	require.NoError(t, err)
	assert.Equal(t, legacyInviter.Id, inviterId)
	assert.Zero(t, promoLinkId)

	require.NoError(t, DB.Model(&AgentProfile{}).Where("user_id = ?", agent.Id).Update("status", AgentStatusDisabled).Error)
	inviterId, promoLinkId, err = ResolveRegistrationAttribution(promoLink.Code)
	require.NoError(t, err)
	assert.Zero(t, inviterId)
	assert.Zero(t, promoLinkId)
}

func TestUpsertAgentProfileCreatesPromoLinkWithoutInvitationCodeCollision(t *testing.T) {
	setupAgentBackendTest(t)
	operator := createAgentBackendTestUser(t, "profile-operator")
	agent := createAgentBackendTestUser(t, "profile-agent")
	group := &AgentRebateGroup{Name: "profile-group", RebateRate: 1000, Status: AgentStatusEnabled}
	require.NoError(t, DB.Create(group).Error)

	profile, err := UpsertAgentProfile(operator.Id, &AgentProfile{
		UserId:        agent.Id,
		Status:        AgentStatusEnabled,
		RebateGroupId: group.Id,
	})
	require.NoError(t, err)
	assert.Equal(t, agent.Id, profile.UserId)

	var promoLink AgentPromoLink
	require.NoError(t, DB.Where("agent_user_id = ?", agent.Id).First(&promoLink).Error)
	assert.NotEmpty(t, promoLink.Code)
	assert.NotEqual(t, agent.AffCode, promoLink.Code)

	_, err = UpsertAgentPromoLink(operator.Id, &AgentPromoLink{
		AgentUserId: agent.Id,
		Name:        "conflicting",
		Code:        operator.AffCode,
		Status:      AgentPromoLinkEnabled,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invitation code")
}

func TestChangeAgentParentRejectsCyclesAndOnlyChangesFutureRebates(t *testing.T) {
	setupAgentBackendTest(t)
	operator := createAgentBackendTestUser(t, "agent-operator")
	oldParent := createAgentBackendTestUser(t, "old-parent")
	child := createAgentBackendTestUser(t, "child-agent")
	descendant := createAgentBackendTestUser(t, "descendant-agent")
	newParent := createAgentBackendTestUser(t, "new-parent")
	group := &AgentRebateGroup{Name: "agent-test-group", RebateRate: 1000, Status: AgentStatusEnabled}
	require.NoError(t, DB.Create(group).Error)
	for _, user := range []*User{oldParent, child, descendant, newParent} {
		require.NoError(t, DB.Create(&AgentProfile{UserId: user.Id, Status: AgentStatusEnabled, RebateGroupId: group.Id}).Error)
	}
	oldLink := &AgentPromoLink{AgentUserId: oldParent.Id, Name: "old", Code: "OLDPARENT", Status: AgentPromoLinkEnabled}
	newLink := &AgentPromoLink{AgentUserId: newParent.Id, Name: "new", Code: "NEWPARENT", Status: AgentPromoLinkEnabled}
	descendantLink := &AgentPromoLink{AgentUserId: descendant.Id, Name: "descendant", Code: "DESCENDANT", Status: AgentPromoLinkEnabled}
	require.NoError(t, DB.Create(oldLink).Error)
	require.NoError(t, DB.Create(newLink).Error)
	require.NoError(t, DB.Create(descendantLink).Error)
	require.NoError(t, DB.Model(child).Updates(map[string]interface{}{"inviter_id": oldParent.Id, "promo_link_id": oldLink.Id}).Error)
	require.NoError(t, DB.Create(&AgentRelationship{ParentAgentUserId: oldParent.Id, ChildAgentUserId: child.Id, Status: AgentStatusEnabled}).Error)
	require.NoError(t, DB.Create(&AgentRelationship{ParentAgentUserId: child.Id, ChildAgentUserId: descendant.Id, Status: AgentStatusEnabled}).Error)

	before := &TopUp{UserId: child.Id, Money: 100, TradeNo: "before-parent-change", Status: common.TopUpStatusSuccess}
	require.NoError(t, DB.Create(before).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return SettleAgentRebateTx(tx, before, AgentRebateSourceEPay)
	}))

	err := ChangeAgentDownlineUser(operator.Id, descendant.Id, child.Id, descendantLink.Id, "cycle attempt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")

	require.NoError(t, ChangeAgentDownlineUser(operator.Id, newParent.Id, child.Id, newLink.Id, "approved parent change"))
	var updatedChild User
	require.NoError(t, DB.Select("id", "inviter_id", "promo_link_id").First(&updatedChild, child.Id).Error)
	assert.Equal(t, newParent.Id, updatedChild.InviterId)
	assert.Equal(t, newLink.Id, updatedChild.PromoLinkId)
	var relation AgentRelationship
	require.NoError(t, DB.Where("child_agent_user_id = ?", child.Id).First(&relation).Error)
	assert.Equal(t, newParent.Id, relation.ParentAgentUserId)

	after := &TopUp{UserId: child.Id, Money: 100, TradeNo: "after-parent-change", Status: common.TopUpStatusSuccess}
	require.NoError(t, DB.Create(after).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return SettleAgentRebateTx(tx, after, AgentRebateSourceEPay)
	}))
	var records []AgentRebateRecord
	require.NoError(t, DB.Order("id asc").Find(&records).Error)
	require.Len(t, records, 2)
	assert.Equal(t, oldParent.Id, records[0].AgentUserId)
	assert.Equal(t, oldLink.Id, records[0].PromoLinkId)
	assert.Equal(t, newParent.Id, records[1].AgentUserId)
	assert.Equal(t, newLink.Id, records[1].PromoLinkId)
}

func TestChangeAgentParentRejectsRateConflict(t *testing.T) {
	setupAgentBackendTest(t)
	operator := createAgentBackendTestUser(t, "rate-operator")
	oldParent := createAgentBackendTestUser(t, "rate-old-parent")
	child := createAgentBackendTestUser(t, "rate-child")
	lowRateParent := createAgentBackendTestUser(t, "rate-low-parent")
	highGroup := &AgentRebateGroup{Name: "high-rate", RebateRate: 2000, Status: AgentStatusEnabled}
	lowGroup := &AgentRebateGroup{Name: "low-rate", RebateRate: 1000, Status: AgentStatusEnabled}
	require.NoError(t, DB.Create(highGroup).Error)
	require.NoError(t, DB.Create(lowGroup).Error)
	require.NoError(t, DB.Create(&AgentProfile{UserId: oldParent.Id, Status: AgentStatusEnabled, RebateGroupId: highGroup.Id}).Error)
	require.NoError(t, DB.Create(&AgentProfile{UserId: child.Id, Status: AgentStatusEnabled, RebateGroupId: highGroup.Id}).Error)
	require.NoError(t, DB.Create(&AgentProfile{UserId: lowRateParent.Id, Status: AgentStatusEnabled, RebateGroupId: lowGroup.Id}).Error)
	require.NoError(t, DB.Model(child).Update("inviter_id", oldParent.Id).Error)
	require.NoError(t, DB.Create(&AgentRelationship{ParentAgentUserId: oldParent.Id, ChildAgentUserId: child.Id, Status: AgentStatusEnabled}).Error)

	err := ChangeAgentDownlineUser(operator.Id, lowRateParent.Id, child.Id, 0, "invalid rate")
	var conflict *AgentRateConflictError
	require.ErrorAs(t, err, &conflict)
	require.Len(t, conflict.Conflicts, 1)
	assert.Equal(t, child.Id, conflict.Conflicts[0].AgentUserId)

	var unchanged AgentRelationship
	require.NoError(t, DB.Where("child_agent_user_id = ?", child.Id).First(&unchanged).Error)
	assert.Equal(t, oldParent.Id, unchanged.ParentAgentUserId)
}

func TestAgentRebateRateAndBalanceBounds(t *testing.T) {
	setupAgentBackendTest(t)
	operator := createAgentBackendTestUser(t, "bounds-operator")
	agent := createAgentBackendTestUser(t, "bounds-agent")

	_, err := UpsertAgentRebateGroup(operator.Id, &AgentRebateGroup{
		Name:       "invalid-rate",
		RebateRate: AgentMaxRebateRate + 1,
		Status:     AgentStatusEnabled,
	})
	require.Error(t, err)

	group := &AgentRebateGroup{Name: "valid-rate", RebateRate: AgentMaxRebateRate, Status: AgentStatusEnabled}
	require.NoError(t, DB.Create(group).Error)
	require.NoError(t, DB.Create(&AgentProfile{
		UserId:              agent.Id,
		Status:              AgentStatusEnabled,
		RebateGroupId:       group.Id,
		RebateBalanceAmount: math.MaxInt64,
	}).Error)

	_, err = AdjustAgentRebateBalance(agent.Id, operator.Id, 1, "overflow attempt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "supported range")
}

func TestSuccessfulTopUpsSettleAgentRebateAcrossProviders(t *testing.T) {
	testCases := []struct {
		name       string
		provider   string
		sourceType string
		amount     int64
		complete   func(*TopUp) error
	}{
		{
			name:       "epay",
			provider:   PaymentProviderEpay,
			sourceType: AgentRebateSourceEPay,
			amount:     100,
			complete: func(topUp *TopUp) error {
				_, err := RechargeEpay(topUp.TradeNo, "alipay")
				return err
			},
		},
		{
			name:       "stripe",
			provider:   PaymentProviderStripe,
			sourceType: AgentRebateSourceStripe,
			amount:     100,
			complete: func(topUp *TopUp) error {
				return Recharge(topUp.TradeNo, "cus_agent_rebate", "127.0.0.1")
			},
		},
		{
			name:       "creem",
			provider:   PaymentProviderCreem,
			sourceType: AgentRebateSourceCreem,
			amount:     10000,
			complete: func(topUp *TopUp) error {
				return RechargeCreem(topUp.TradeNo, "", "", "127.0.0.1")
			},
		},
		{
			name:       "waffo",
			provider:   PaymentProviderWaffo,
			sourceType: AgentRebateSourceWaffo,
			amount:     100,
			complete: func(topUp *TopUp) error {
				return RechargeWaffo(topUp.TradeNo, "127.0.0.1")
			},
		},
		{
			name:       "waffo pancake",
			provider:   PaymentProviderWaffoPancake,
			sourceType: AgentRebateSourceWaffoPancake,
			amount:     100,
			complete: func(topUp *TopUp) error {
				return RechargeWaffoPancake(topUp.TradeNo)
			},
		},
		{
			name:       "manual completion",
			provider:   PaymentProviderEpay,
			sourceType: AgentRebateSourceManual,
			amount:     100,
			complete: func(topUp *TopUp) error {
				return ManualCompleteTopUp(topUp.TradeNo, "127.0.0.1")
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setupAgentBackendTest(t)
			previousQuotaPerUnit := common.QuotaPerUnit
			common.QuotaPerUnit = 100
			t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })

			agent := createAgentBackendTestUser(t, "topup-agent")
			invitee := createAgentBackendTestUser(t, "topup-invitee")
			group := &AgentRebateGroup{Name: "topup-group", RebateRate: 1000, Status: AgentStatusEnabled}
			require.NoError(t, DB.Create(group).Error)
			require.NoError(t, DB.Create(&AgentProfile{UserId: agent.Id, Status: AgentStatusEnabled, RebateGroupId: group.Id}).Error)
			require.NoError(t, DB.Model(invitee).Update("inviter_id", agent.Id).Error)

			topUp := &TopUp{
				UserId:          invitee.Id,
				Amount:          testCase.amount,
				Money:           100,
				TradeNo:         "topup-" + strings.ReplaceAll(testCase.name, " ", "-"),
				PaymentMethod:   testCase.provider,
				PaymentProvider: testCase.provider,
				CreateTime:      common.GetTimestamp(),
				Status:          common.TopUpStatusPending,
			}
			require.NoError(t, DB.Create(topUp).Error)

			require.NoError(t, testCase.complete(topUp))
			require.NoError(t, testCase.complete(topUp))

			var record AgentRebateRecord
			require.NoError(t, DB.Where("top_up_id = ?", topUp.Id).First(&record).Error)
			assert.Equal(t, testCase.sourceType, record.SourceType)
			assert.Equal(t, int64(1000), record.RebateAmount)

			var recordCount int64
			require.NoError(t, DB.Model(&AgentRebateRecord{}).Where("top_up_id = ?", topUp.Id).Count(&recordCount).Error)
			assert.Equal(t, int64(1), recordCount)

			profile, err := GetAgentProfileByUserId(agent.Id)
			require.NoError(t, err)
			assert.Equal(t, int64(1000), profile.RebateBalanceAmount)
			assert.Equal(t, int64(1000), profile.RebateTotalAmount)
		})
	}
}

func TestEpayRechargeRollsBackWhenAgentSettlementFails(t *testing.T) {
	setupAgentBackendTest(t)
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })

	agent := createAgentBackendTestUser(t, "rollback-agent")
	invitee := createAgentBackendTestUser(t, "rollback-invitee")
	group := &AgentRebateGroup{Name: "rollback-group", RebateRate: 1000, Status: AgentStatusEnabled}
	require.NoError(t, DB.Create(group).Error)
	require.NoError(t, DB.Create(&AgentProfile{UserId: agent.Id, Status: AgentStatusEnabled, RebateGroupId: group.Id}).Error)
	require.NoError(t, DB.Model(invitee).Update("inviter_id", agent.Id).Error)
	require.NoError(t, DB.Model(group).UpdateColumn("rebate_rate", AgentMaxRebateRate+1).Error)

	topUp := &TopUp{
		UserId:          invitee.Id,
		Amount:          100,
		Money:           100,
		TradeNo:         "topup-agent-rollback",
		PaymentMethod:   "alipay",
		PaymentProvider: PaymentProviderEpay,
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, DB.Create(topUp).Error)

	_, err := RechargeEpay(topUp.TradeNo, "alipay")
	require.Error(t, err)

	var savedTopUp TopUp
	require.NoError(t, DB.First(&savedTopUp, topUp.Id).Error)
	assert.Equal(t, common.TopUpStatusPending, savedTopUp.Status)
	var savedInvitee User
	require.NoError(t, DB.First(&savedInvitee, invitee.Id).Error)
	assert.Zero(t, savedInvitee.Quota)
	var recordCount int64
	require.NoError(t, DB.Model(&AgentRebateRecord{}).Count(&recordCount).Error)
	assert.Zero(t, recordCount)
}

func TestEpayRechargeReconcilesMissingRebateWithoutCreditingQuotaAgain(t *testing.T) {
	setupAgentBackendTest(t)
	agent := createAgentBackendTestUser(t, "reconcile-agent")
	invitee := createAgentBackendTestUser(t, "reconcile-invitee")
	group := &AgentRebateGroup{Name: "reconcile-group", RebateRate: 1000, Status: AgentStatusEnabled}
	require.NoError(t, DB.Create(group).Error)
	require.NoError(t, DB.Create(&AgentProfile{UserId: agent.Id, Status: AgentStatusEnabled, RebateGroupId: group.Id}).Error)
	require.NoError(t, DB.Model(invitee).Updates(map[string]interface{}{
		"inviter_id": agent.Id,
		"quota":      500,
	}).Error)

	topUp := &TopUp{
		UserId:          invitee.Id,
		Amount:          100,
		Money:           100,
		TradeNo:         "topup-agent-reconcile",
		PaymentMethod:   "alipay",
		PaymentProvider: PaymentProviderEpay,
		CreateTime:      common.GetTimestamp(),
		CompleteTime:    common.GetTimestamp(),
		Status:          common.TopUpStatusSuccess,
	}
	require.NoError(t, DB.Create(topUp).Error)

	result, err := RechargeEpay(topUp.TradeNo, "alipay")
	require.NoError(t, err)
	assert.False(t, result.Completed)
	assert.Zero(t, result.QuotaAdded)

	var savedInvitee User
	require.NoError(t, DB.Select("quota").First(&savedInvitee, invitee.Id).Error)
	assert.Equal(t, 500, savedInvitee.Quota)
	var record AgentRebateRecord
	require.NoError(t, DB.Where("top_up_id = ?", topUp.Id).First(&record).Error)
	assert.Equal(t, agent.Id, record.AgentUserId)
	assert.Equal(t, int64(1000), record.RebateAmount)
}

func TestAgentBalanceAdjustmentDoesNotInflateEarnedRebate(t *testing.T) {
	setupAgentBackendTest(t)
	operator := createAgentBackendTestUser(t, "adjustment-operator")
	agent := createAgentBackendTestUser(t, "adjustment-agent")
	require.NoError(t, DB.Create(&AgentProfile{
		UserId:              agent.Id,
		Status:              AgentStatusEnabled,
		RebateBalanceAmount: 100,
		RebateTotalAmount:   500,
	}).Error)

	adjustment, err := AdjustAgentRebateBalance(agent.Id, operator.Id, 25, "manual payable balance")
	require.NoError(t, err)
	assert.Equal(t, int64(125), adjustment.BalanceAfter)

	var profile AgentProfile
	require.NoError(t, DB.Where("user_id = ?", agent.Id).First(&profile).Error)
	assert.Equal(t, int64(125), profile.RebateBalanceAmount)
	assert.Equal(t, int64(500), profile.RebateTotalAmount)
}

func TestAgentWithdrawReceiptRequiresExportAndSettlesFrozenBalance(t *testing.T) {
	setupAgentBackendTest(t)
	agent := createAgentBackendTestUser(t, "withdraw-agent")
	require.NoError(t, DB.Create(&AgentProfile{
		UserId:              agent.Id,
		Status:              AgentStatusEnabled,
		RebateBalanceAmount: 500,
		RebateFrozenAmount:  200,
		RebateTotalAmount:   900,
	}).Error)
	request := &AgentWithdrawRequest{
		AgentUserId:         agent.Id,
		Amount:              200,
		Status:              AgentWithdrawStatusPending,
		AccountNameSnapshot: "Test Agent",
		AccountNoSnapshot:   "account-1",
	}
	require.NoError(t, DB.Create(request).Error)

	content, batchNo, err := ExportAgentWithdrawRequests(AgentWithdrawStatusPending, "", "")
	require.NoError(t, err)
	assert.NotEmpty(t, batchNo)
	assert.Contains(t, string(content), "exported")

	var exported AgentWithdrawRequest
	require.NoError(t, DB.First(&exported, request.Id).Error)
	assert.Equal(t, AgentWithdrawStatusExported, exported.Status)
	assert.Equal(t, batchNo, exported.ExportBatchNo)

	receipt := fmt.Sprintf("request_id,username,email,account_name,account_no,amount,status,external_order_no\n%d,,,,,2.00,exported,BANK-ORDER-1\n", request.Id)
	result, err := ImportAgentWithdrawResults(strings.NewReader(receipt))
	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
	assert.Equal(t, []int{request.Id}, result.RequestIds)

	var profile AgentProfile
	require.NoError(t, DB.Where("user_id = ?", agent.Id).First(&profile).Error)
	assert.Equal(t, int64(500), profile.RebateBalanceAmount)
	assert.Zero(t, profile.RebateFrozenAmount)
	assert.Equal(t, int64(900), profile.RebateTotalAmount)
	require.NoError(t, DB.First(&exported, request.Id).Error)
	assert.Equal(t, AgentWithdrawStatusPaid, exported.Status)
	assert.Equal(t, "BANK-ORDER-1", exported.ExternalOrderNo)

	pending := &AgentWithdrawRequest{
		AgentUserId:         agent.Id,
		Amount:              100,
		Status:              AgentWithdrawStatusPending,
		AccountNameSnapshot: "Test Agent",
		AccountNoSnapshot:   "account-1",
	}
	require.NoError(t, DB.Create(pending).Error)
	pendingReceipt := fmt.Sprintf("request_id,external_order_no\n%d,BANK-ORDER-2\n", pending.Id)
	_, err = ImportAgentWithdrawResults(strings.NewReader(pendingReceipt))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not exported")
}

func TestMigrateAgentSchemaIsAdditiveAndIdempotent(t *testing.T) {
	previousDB := DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&legacyAgentUser{}))
	require.NoError(t, db.Create(&legacyAgentUser{Username: "preserved-user"}).Error)

	require.NoError(t, MigrateAgentSchema())
	require.NoError(t, MigrateAgentSchema())
	assert.True(t, db.Migrator().HasColumn(&User{}, "PromoLinkId"))
	assert.True(t, db.Migrator().HasIndex(&User{}, "PromoLinkId"))
	for _, schemaModel := range AgentSchemaModels() {
		assert.True(t, db.Migrator().HasTable(schemaModel))
	}
	var count int64
	require.NoError(t, db.Table("users").Where("username = ?", "preserved-user").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
