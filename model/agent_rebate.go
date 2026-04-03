package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"

	"gorm.io/gorm"
)

const (
	AgentStatusEnabled  = 1
	AgentStatusDisabled = 0
	AgentLevelPrimary   = 1
	AgentLevelSecondary = 2

	AgentPromoLinkEnabled  = 1
	AgentPromoLinkDisabled = 0

	AgentRebateRecordSettled  = "settled"
	AgentRebateRecordCanceled = "canceled"
	AgentRebateRecordRolled   = "rolled_back"

	AgentRebateSourceEPay   = "epay"
	AgentRebateSourceManual = "manual"

	AgentAdjustmentTypeIncrease = "increase"
	AgentAdjustmentTypeDecrease = "decrease"

	AgentRateSourceGroup  = "group"
	AgentRateSourceCustom = "custom"

	AgentUpgradeRequestPending  = "pending"
	AgentUpgradeRequestApproved = "approved"
	AgentUpgradeRequestRejected = "rejected"

	DefaultAgentRebateGroupName = "default"
)

// AgentRebateGroup stores default rebate rates for different agent groups.
// RebateRate is stored in basis points, e.g. 1500 = 15.00%.
type AgentRebateGroup struct {
	Id         int    `json:"id"`
	Name       string `json:"name" gorm:"type:varchar(64);uniqueIndex;not null"`
	RebateRate int    `json:"rebate_rate" gorm:"type:int;not null;default:0"`
	Status     int    `json:"status" gorm:"type:int;not null;default:1;index"`
	IsDefault  bool   `json:"is_default" gorm:"not null;default:false"`
	Remark     string `json:"remark" gorm:"type:varchar(255);default:''"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt  int64  `json:"updated_at" gorm:"bigint"`
}

func (g *AgentRebateGroup) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	g.CreatedAt = now
	g.UpdatedAt = now
	return nil
}

func (g *AgentRebateGroup) BeforeUpdate(tx *gorm.DB) error {
	g.UpdatedAt = common.GetTimestamp()
	return nil
}

// AgentProfile marks a user as an agent and stores agent-specific settings.
// CustomRate is stored in basis points and overrides the group's default rate when > 0.
type AgentProfile struct {
	Id                  int    `json:"id"`
	UserId              int    `json:"user_id" gorm:"uniqueIndex;not null"`
	Status              int    `json:"status" gorm:"type:int;not null;default:1;index"`
	RebateGroupId       int    `json:"rebate_group_id" gorm:"type:int;not null;default:0;index"`
	CustomRate          int    `json:"custom_rate" gorm:"type:int;not null;default:0"`
	RebateBalanceAmount int64  `json:"rebate_balance_amount" gorm:"type:bigint;not null;default:0"`
	RebateTotalAmount   int64  `json:"rebate_total_amount" gorm:"type:bigint;not null;default:0"`
	Remark              string `json:"remark" gorm:"type:varchar(255);default:''"`
	CreatedAt           int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt           int64  `json:"updated_at" gorm:"bigint"`
}

func (p *AgentProfile) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *AgentProfile) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

// AgentPromoLink tracks channel-level attribution for agent promotion links.
type AgentPromoLink struct {
	Id          int    `json:"id"`
	AgentUserId int    `json:"agent_user_id" gorm:"not null;index"`
	Name        string `json:"name" gorm:"type:varchar(64);not null"`
	Code        string `json:"code" gorm:"type:varchar(32);uniqueIndex;not null"`
	Status      int    `json:"status" gorm:"type:int;not null;default:1;index"`
	LandingPage string `json:"landing_page" gorm:"type:varchar(255);default:''"`
	Remark      string `json:"remark" gorm:"type:varchar(255);default:''"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
}

func (p *AgentPromoLink) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *AgentPromoLink) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

// AgentRebateRecord is the immutable rebate ledger for each successful topup.
// PayAmount and RebateAmount are stored in the smallest currency unit to avoid floating-point drift.
type AgentRebateRecord struct {
	Id            int    `json:"id"`
	TopUpId       int    `json:"topup_id" gorm:"not null;uniqueIndex"`
	TradeNo       string `json:"trade_no" gorm:"type:varchar(255);not null;index"`
	SourceType    string `json:"source_type" gorm:"type:varchar(32);not null;index"`
	InviteeUserId int    `json:"invitee_user_id" gorm:"not null;index"`
	AgentUserId   int    `json:"agent_user_id" gorm:"not null;index"`
	PromoLinkId   int    `json:"promo_link_id" gorm:"type:int;not null;default:0;index"`
	PayAmount     int64  `json:"pay_amount" gorm:"type:bigint;not null;default:0"`
	RebateRate    int    `json:"rebate_rate" gorm:"type:int;not null;default:0"`
	RebateAmount  int64  `json:"rebate_amount" gorm:"type:bigint;not null;default:0"`
	Status        string `json:"status" gorm:"type:varchar(32);not null;default:'settled';index"`
	Remark        string `json:"remark" gorm:"type:varchar(255);default:''"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;index"`
	SettledAt     int64  `json:"settled_at" gorm:"bigint"`
}

type AgentRebateAdjustment struct {
	Id             int    `json:"id"`
	AgentUserId    int    `json:"agent_user_id" gorm:"not null;index"`
	OperatorUserId int    `json:"operator_user_id" gorm:"not null;index"`
	DeltaAmount    int64  `json:"delta_amount" gorm:"type:bigint;not null"`
	BalanceBefore  int64  `json:"balance_before" gorm:"type:bigint;not null;default:0"`
	BalanceAfter   int64  `json:"balance_after" gorm:"type:bigint;not null;default:0"`
	ChangeType     string `json:"change_type" gorm:"type:varchar(32);not null;index"`
	Reason         string `json:"reason" gorm:"type:varchar(255);default:''"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index"`
}

type AgentRelationship struct {
	Id                int   `json:"id"`
	ParentAgentUserId int   `json:"parent_agent_user_id" gorm:"not null;index"`
	ChildAgentUserId  int   `json:"child_agent_user_id" gorm:"not null;uniqueIndex"`
	Status            int   `json:"status" gorm:"type:int;not null;default:1;index"`
	CreatedAt         int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt         int64 `json:"updated_at" gorm:"bigint"`
}

func (r *AgentRelationship) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *AgentRelationship) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

type AgentUpgradeRequest struct {
	Id                 int    `json:"id"`
	SponsorAgentUserId int    `json:"sponsor_agent_user_id" gorm:"not null;index"`
	TargetUserId       int    `json:"target_user_id" gorm:"not null;index"`
	TargetRate         int    `json:"target_rate" gorm:"type:int;not null;default:0"`
	Status             string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	Remark             string `json:"remark" gorm:"type:varchar(255);default:''"`
	ReviewerUserId     int    `json:"reviewer_user_id" gorm:"type:int;not null;default:0;index"`
	ReviewedAt         int64  `json:"reviewed_at" gorm:"bigint"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint"`
}

func (r *AgentUpgradeRequest) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *AgentUpgradeRequest) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

func (a *AgentRebateAdjustment) BeforeCreate(tx *gorm.DB) error {
	if a.CreatedAt == 0 {
		a.CreatedAt = common.GetTimestamp()
	}
	return nil
}

type AgentProfileView struct {
	Id                  int    `json:"id"`
	UserId              int    `json:"user_id"`
	Username            string `json:"username"`
	DisplayName         string `json:"display_name"`
	AgentLevel          int    `json:"agent_level"`
	ParentAgentUserId   int    `json:"parent_agent_user_id"`
	ParentAgentUsername string `json:"parent_agent_username"`
	Status              int    `json:"status"`
	RebateGroupId       int    `json:"rebate_group_id"`
	RebateGroupName     string `json:"rebate_group_name"`
	CustomRate          int    `json:"custom_rate"`
	EffectiveRate       int    `json:"effective_rate"`
	EffectiveRateSource string `json:"effective_rate_source"`
	ParentMaxRate       int    `json:"parent_max_rate"`
	RebateBalanceAmount int64  `json:"rebate_balance_amount"`
	RebateTotalAmount   int64  `json:"rebate_total_amount"`
	Remark              string `json:"remark"`
	CreatedAt           int64  `json:"created_at"`
	UpdatedAt           int64  `json:"updated_at"`
}

type AgentSelfSummary struct {
	AgentEnabled          bool              `json:"agent_enabled"`
	AgentInitialized      bool              `json:"agent_initialized"`
	IsAgent               bool              `json:"is_agent"`
	Profile               *AgentProfileView `json:"profile,omitempty"`
	RecentRebateCount     int64             `json:"recent_rebate_count"`
	RecentRebateAmount    int64             `json:"recent_rebate_amount"`
	RecentAdjustmentCount int64             `json:"recent_adjustment_count"`
}

type AgentPromoLinkView struct {
	Id          int    `json:"id"`
	AgentUserId int    `json:"agent_user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Status      int    `json:"status"`
	LandingPage string `json:"landing_page"`
	Remark      string `json:"remark"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type AgentPromoLinkStat struct {
	PromoLinkId     int    `json:"promo_link_id"`
	AgentUserId     int    `json:"agent_user_id"`
	Name            string `json:"name"`
	Code            string `json:"code"`
	Status          int    `json:"status"`
	InviteeCount    int64  `json:"invitee_count"`
	TopupCount      int64  `json:"topup_count"`
	TopupAmount     int64  `json:"topup_amount"`
	RebateAmount    int64  `json:"rebate_amount"`
	LastInviteeId   int    `json:"last_invitee_id"`
	LastInviteeName string `json:"last_invitee_name"`
	LandingPage     string `json:"landing_page"`
}

type AgentDownlineUserView struct {
	UserId           int    `json:"user_id"`
	Username         string `json:"username"`
	DisplayName      string `json:"display_name"`
	InviterId        int    `json:"inviter_id"`
	PromoLinkId      int    `json:"promo_link_id"`
	PromoLinkName    string `json:"promo_link_name"`
	IsAgent          bool   `json:"is_agent"`
	TopupCount       int64  `json:"topup_count"`
	TopupAmount      int64  `json:"topup_amount"`
	RebateAmount     int64  `json:"rebate_amount"`
	LatestTopupTime  int64  `json:"latest_topup_time"`
	LatestRebateTime int64  `json:"latest_rebate_time"`
}

type AgentAdminOverview struct {
	AgentCount          int64 `json:"agent_count"`
	GroupCount          int64 `json:"group_count"`
	PromoLinkCount      int64 `json:"promo_link_count"`
	DownlineUserCount   int64 `json:"downline_user_count"`
	RebateBalanceAmount int64 `json:"rebate_balance_amount"`
	RebateTotalAmount   int64 `json:"rebate_total_amount"`
}

type AgentUpgradeRequestView struct {
	Id                   int    `json:"id"`
	SponsorAgentUserId   int    `json:"sponsor_agent_user_id"`
	SponsorAgentUsername string `json:"sponsor_agent_username"`
	TargetUserId         int    `json:"target_user_id"`
	TargetUsername       string `json:"target_username"`
	TargetDisplayName    string `json:"target_display_name"`
	TargetRate           int    `json:"target_rate"`
	Status               string `json:"status"`
	Remark               string `json:"remark"`
	ReviewerUserId       int    `json:"reviewer_user_id"`
	ReviewerUsername     string `json:"reviewer_username"`
	ReviewedAt           int64  `json:"reviewed_at"`
	CreatedAt            int64  `json:"created_at"`
}

type AgentRateConflict struct {
	AgentUserId       int    `json:"agent_user_id"`
	AgentUsername     string `json:"agent_username"`
	ParentAgentUserId int    `json:"parent_agent_user_id"`
	ParentAgentName   string `json:"parent_agent_name"`
	AgentRate         int    `json:"agent_rate"`
	ParentAllowedRate int    `json:"parent_allowed_rate"`
	ConflictType      string `json:"conflict_type"`
}

type AgentRateConflictError struct {
	Message   string              `json:"message"`
	Conflicts []AgentRateConflict `json:"conflicts"`
}

func (e *AgentRateConflictError) Error() string {
	if e == nil {
		return "agent rate conflict"
	}
	if e.Message != "" {
		return e.Message
	}
	return "agent rate conflict"
}

func (r *AgentRebateRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	if r.SettledAt == 0 && r.Status == AgentRebateRecordSettled {
		r.SettledAt = now
	}
	return nil
}

func GetAgentProfileByUserId(userId int) (*AgentProfile, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	var profile AgentProfile
	if err := DB.Where("user_id = ?", userId).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func GetDefaultAgentRebateGroup() (*AgentRebateGroup, error) {
	var group AgentRebateGroup
	if err := DB.Where("is_default = ?", true).First(&group).Error; err == nil {
		return &group, nil
	}
	if err := DB.Where("name = ?", DefaultAgentRebateGroupName).First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func CreateDefaultAgentRebateGroupTx(tx *gorm.DB, rebateRate int) (*AgentRebateGroup, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	var group AgentRebateGroup
	err := tx.Where("name = ?", DefaultAgentRebateGroupName).First(&group).Error
	if err == nil {
		return &group, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	group = AgentRebateGroup{
		Name:       DefaultAgentRebateGroupName,
		RebateRate: rebateRate,
		Status:     AgentStatusEnabled,
		IsDefault:  true,
		Remark:     "system bootstrap",
	}
	if err := tx.Create(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func NormalizePromoCode(code string) string {
	return strings.TrimSpace(code)
}

func ensureAgentDefaultPromoLinkTx(tx *gorm.DB, agentUserId int) error {
	if tx == nil || agentUserId <= 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&AgentPromoLink{}).Where("agent_user_id = ?", agentUserId).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	code, err := GenerateUniqueAgentPromoCodeTx(tx)
	if err != nil {
		return err
	}
	defaultLink := AgentPromoLink{
		AgentUserId: agentUserId,
		Name:        "default",
		Code:        code,
		Status:      AgentPromoLinkEnabled,
		LandingPage: "/",
		Remark:      "auto-created",
	}
	return tx.Create(&defaultLink).Error
}

func GenerateUniqueAgentPromoCodeTx(tx *gorm.DB) (string, error) {
	for i := 0; i < 5; i++ {
		code := strings.ToUpper(common.GetRandomString(8))
		var count int64
		if err := tx.Model(&AgentPromoLink{}).Where("code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", errors.New("failed to generate unique promo code")
}

func GetAgentPromoLinkByCode(code string) (*AgentPromoLink, error) {
	code = NormalizePromoCode(code)
	if code == "" {
		return nil, errors.New("promo code is empty")
	}
	var promoLink AgentPromoLink
	if err := DB.Where("code = ?", code).First(&promoLink).Error; err != nil {
		return nil, err
	}
	return &promoLink, nil
}

func ResolveRegistrationAttribution(code string) (inviterId int, promoLinkId int, err error) {
	code = NormalizePromoCode(code)
	if code == "" {
		return 0, 0, nil
	}
	if DB != nil && DB.Migrator().HasTable(&AgentPromoLink{}) {
		promoLink, promoErr := GetAgentPromoLinkByCode(code)
		if promoErr == nil && promoLink.Status == AgentPromoLinkEnabled {
			return promoLink.AgentUserId, promoLink.Id, nil
		}
		if promoErr != nil && !errors.Is(promoErr, gorm.ErrRecordNotFound) {
			return 0, 0, promoErr
		}
	}
	inviterId, err = GetUserIdByAffCode(code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	return inviterId, 0, nil
}

func SettleAgentRebateTx(tx *gorm.DB, topUp *TopUp, sourceType string) error {
	if tx == nil || topUp == nil {
		return errors.New("invalid settlement params")
	}
	if !common.AgentEnabled || !common.AgentInitialized {
		return nil
	}
	if topUp.Id <= 0 || topUp.UserId <= 0 {
		return nil
	}
	var existing AgentRebateRecord
	if err := tx.Where("top_up_id = ?", topUp.Id).First(&existing).Error; err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var invitee User
	if err := tx.Select("id", "inviter_id", "promo_link_id").First(&invitee, topUp.UserId).Error; err != nil {
		return err
	}
	if invitee.InviterId <= 0 || invitee.InviterId == invitee.Id {
		return nil
	}
	var profile AgentProfile
	if err := tx.Where("user_id = ?", invitee.InviterId).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if profile.Status != AgentStatusEnabled {
		return nil
	}
	rebateRate, err := getEffectiveAgentRebateRateTx(tx, &profile)
	if err != nil {
		return err
	}
	if rebateRate <= 0 {
		return nil
	}
	payAmount := convertMoneyToMinorUnit(topUp.Money)
	if payAmount <= 0 {
		return nil
	}
	rebateAmount := decimal.NewFromInt(payAmount).Mul(decimal.NewFromInt(int64(rebateRate))).Div(decimal.NewFromInt(10000)).Round(0).IntPart()
	if rebateAmount <= 0 {
		return nil
	}
	record := AgentRebateRecord{
		TopUpId:       topUp.Id,
		TradeNo:       topUp.TradeNo,
		SourceType:    sourceType,
		InviteeUserId: invitee.Id,
		AgentUserId:   profile.UserId,
		PromoLinkId:   invitee.PromoLinkId,
		PayAmount:     payAmount,
		RebateRate:    rebateRate,
		RebateAmount:  rebateAmount,
		Status:        AgentRebateRecordSettled,
	}
	if err := tx.Create(&record).Error; err != nil {
		return err
	}
	return tx.Model(&AgentProfile{}).Where("id = ?", profile.Id).Updates(map[string]interface{}{
		"rebate_balance_amount": gorm.Expr("rebate_balance_amount + ?", rebateAmount),
		"rebate_total_amount":   gorm.Expr("rebate_total_amount + ?", rebateAmount),
	}).Error
}

func getEffectiveAgentRebateRateTx(tx *gorm.DB, profile *AgentProfile) (int, error) {
	if profile == nil {
		return 0, nil
	}
	if profile.CustomRate > 0 {
		return profile.CustomRate, nil
	}
	if profile.RebateGroupId > 0 {
		var group AgentRebateGroup
		if err := tx.First(&group, profile.RebateGroupId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			return 0, err
		}
		if group.Status == AgentStatusEnabled {
			return group.RebateRate, nil
		}
	}
	if common.AgentDefaultRebateRate > 0 {
		return common.AgentDefaultRebateRate, nil
	}
	var defaultGroup AgentRebateGroup
	if err := tx.Where("is_default = ?", true).First(&defaultGroup).Error; err == nil && defaultGroup.Status == AgentStatusEnabled {
		return defaultGroup.RebateRate, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	return 0, nil
}

func getEffectiveAgentRebateRateDetailsTx(tx *gorm.DB, profile *AgentProfile) (int, string, error) {
	if profile == nil {
		return 0, AgentRateSourceGroup, nil
	}
	if profile.CustomRate > 0 {
		return profile.CustomRate, AgentRateSourceCustom, nil
	}
	rate, err := getEffectiveAgentRebateRateTx(tx, profile)
	return rate, AgentRateSourceGroup, err
}

func getAgentProfileByUserIdTx(tx *gorm.DB, userId int) (*AgentProfile, error) {
	var profile AgentProfile
	if err := tx.Where("user_id = ?", userId).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func getAgentRelationshipByChildTx(tx *gorm.DB, childAgentUserId int) (*AgentRelationship, error) {
	var relation AgentRelationship
	if err := tx.Where("child_agent_user_id = ? AND status = ?", childAgentUserId, AgentStatusEnabled).First(&relation).Error; err != nil {
		return nil, err
	}
	return &relation, nil
}

func getAgentChildRelationshipsTx(tx *gorm.DB, parentAgentUserId int) ([]*AgentRelationship, error) {
	var relations []*AgentRelationship
	err := tx.Where("parent_agent_user_id = ? AND status = ?", parentAgentUserId, AgentStatusEnabled).Find(&relations).Error
	return relations, err
}

func getEffectiveAgentRateForUserTx(tx *gorm.DB, userId int, overrides map[int]int) (int, error) {
	if rate, ok := overrides[userId]; ok {
		return rate, nil
	}
	profile, err := getAgentProfileByUserIdTx(tx, userId)
	if err != nil {
		return 0, err
	}
	return getEffectiveAgentRebateRateTx(tx, profile)
}

func getUserBasicInfoTx(tx *gorm.DB, userId int) (string, error) {
	var user User
	if err := tx.Select("id", "username").First(&user, userId).Error; err != nil {
		return "", err
	}
	return user.Username, nil
}

func checkAgentRateConflictsTx(tx *gorm.DB, overrides map[int]int) ([]AgentRateConflict, error) {
	if len(overrides) == 0 {
		return nil, nil
	}
	userIds := make([]int, 0, len(overrides))
	for userId := range overrides {
		userIds = append(userIds, userId)
	}
	var relations []*AgentRelationship
	if err := tx.Where("status = ? AND (parent_agent_user_id IN ? OR child_agent_user_id IN ?)", AgentStatusEnabled, userIds, userIds).Find(&relations).Error; err != nil {
		return nil, err
	}
	conflicts := make([]AgentRateConflict, 0)
	seen := make(map[string]struct{})
	for _, relation := range relations {
		parentRate, err := getEffectiveAgentRateForUserTx(tx, relation.ParentAgentUserId, overrides)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		childRate, err := getEffectiveAgentRateForUserTx(tx, relation.ChildAgentUserId, overrides)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		if childRate <= parentRate {
			continue
		}
		key := fmt.Sprintf("%d-%d", relation.ParentAgentUserId, relation.ChildAgentUserId)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		childName, _ := getUserBasicInfoTx(tx, relation.ChildAgentUserId)
		parentName, _ := getUserBasicInfoTx(tx, relation.ParentAgentUserId)
		conflicts = append(conflicts, AgentRateConflict{
			AgentUserId:       relation.ChildAgentUserId,
			AgentUsername:     childName,
			ParentAgentUserId: relation.ParentAgentUserId,
			ParentAgentName:   parentName,
			AgentRate:         childRate,
			ParentAllowedRate: parentRate,
			ConflictType:      "child_exceeds_parent",
		})
	}
	return conflicts, nil
}

func buildAgentRateConflictError(conflicts []AgentRateConflict) error {
	if len(conflicts) == 0 {
		return nil
	}
	parts := make([]string, 0, len(conflicts))
	for _, conflict := range conflicts {
		parts = append(parts, fmt.Sprintf("%s(%d) 当前 %.2f%%，上级 %s(%d) 上限 %.2f%%",
			conflict.AgentUsername,
			conflict.AgentUserId,
			float64(conflict.AgentRate)/100,
			conflict.ParentAgentName,
			conflict.ParentAgentUserId,
			float64(conflict.ParentAllowedRate)/100,
		))
	}
	return &AgentRateConflictError{
		Message:   "存在下级代理比例超限，请先调整后再保存: " + strings.Join(parts, "；"),
		Conflicts: conflicts,
	}
}

func buildAgentRateConflictErrorFromOverrides(tx *gorm.DB, overrides map[int]int) error {
	conflicts, err := checkAgentRateConflictsTx(tx, overrides)
	if err != nil {
		return err
	}
	return buildAgentRateConflictError(conflicts)
}

func convertMoneyToMinorUnit(amount float64) int64 {
	return decimal.NewFromFloat(amount).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}

func GetAllAgentRebateGroups() ([]*AgentRebateGroup, error) {
	var groups []*AgentRebateGroup
	err := DB.Order("is_default desc, id asc").Find(&groups).Error
	return groups, err
}

func UpsertAgentRebateGroup(operatorUserId int, group *AgentRebateGroup) (*AgentRebateGroup, error) {
	if group == nil {
		return nil, errors.New("group is nil")
	}
	group.Name = strings.TrimSpace(group.Name)
	group.Remark = strings.TrimSpace(group.Remark)
	if group.Name == "" {
		return nil, errors.New("group name is required")
	}
	if group.RebateRate < 0 {
		return nil, errors.New("rebate rate must be >= 0")
	}
	if group.Status != AgentStatusEnabled && group.Status != AgentStatusDisabled {
		return nil, errors.New("invalid group status")
	}
	var saved AgentRebateGroup
	err := DB.Transaction(func(tx *gorm.DB) error {
		overrides := make(map[int]int)
		var existing AgentRebateGroup
		if err := tx.Where("name = ?", group.Name).First(&existing).Error; err == nil {
			if group.Id == 0 || existing.Id != group.Id {
				return errors.New("group name already exists")
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if group.Id > 0 {
			if err := tx.First(&saved, group.Id).Error; err != nil {
				return err
			}
			if saved.IsDefault && group.Status != AgentStatusEnabled {
				return errors.New("default group cannot be disabled")
			}
			saved.Name = group.Name
			saved.RebateRate = group.RebateRate
			saved.Status = group.Status
			saved.Remark = group.Remark
			var profiles []*AgentProfile
			if err := tx.Where("rebate_group_id = ? AND custom_rate = 0", saved.Id).Find(&profiles).Error; err != nil {
				return err
			}
			for _, profile := range profiles {
				overrides[profile.UserId] = group.RebateRate
			}
			if err := buildAgentRateConflictErrorFromOverrides(tx, overrides); err != nil {
				return err
			}
			return tx.Save(&saved).Error
		}
		saved = AgentRebateGroup{
			Name:       group.Name,
			RebateRate: group.RebateRate,
			Status:     group.Status,
			IsDefault:  false,
			Remark:     group.Remark,
		}
		return tx.Create(&saved).Error
	})
	if err != nil {
		return nil, err
	}
	RecordLog(operatorUserId, LogTypeManage, fmt.Sprintf("更新代理分组，分组ID: %d", saved.Id))
	return &saved, nil
}

func DeleteAgentRebateGroup(id int, operatorUserId int) error {
	if id <= 0 {
		return errors.New("invalid group id")
	}
	var group AgentRebateGroup
	if err := DB.First(&group, id).Error; err != nil {
		return err
	}
	if group.IsDefault {
		return errors.New("default group cannot be deleted")
	}
	var count int64
	if err := DB.Model(&AgentProfile{}).Where("rebate_group_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("group is in use")
	}
	if err := DB.Delete(&group).Error; err != nil {
		return err
	}
	RecordLog(operatorUserId, LogTypeManage, fmt.Sprintf("删除代理分组，分组ID: %d", id))
	return nil
}

func UpsertAgentProfile(operatorUserId int, profile *AgentProfile) (*AgentProfile, error) {
	if profile == nil {
		return nil, errors.New("profile is nil")
	}
	if profile.UserId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if profile.CustomRate < 0 {
		return nil, errors.New("custom rate must be >= 0")
	}
	if profile.RebateGroupId < 0 {
		return nil, errors.New("invalid rebate group")
	}
	var saved *AgentProfile
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Select("id").First(&user, profile.UserId).Error; err != nil {
			return err
		}
		var err error
		saved, err = upsertAgentProfileTx(tx, profile)
		if err != nil {
			return err
		}
		if saved.Status == AgentStatusEnabled {
			if err := ensureAgentDefaultPromoLinkTx(tx, saved.UserId); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	RecordLog(operatorUserId, LogTypeManage, fmt.Sprintf("更新代理资料，用户ID: %d", profile.UserId))
	return saved, nil
}

func AdjustAgentRebateBalance(agentUserId int, operatorUserId int, deltaAmount int64, reason string) (*AgentRebateAdjustment, error) {
	if agentUserId <= 0 || operatorUserId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if deltaAmount == 0 {
		return nil, errors.New("delta amount cannot be zero")
	}
	changeType := AgentAdjustmentTypeIncrease
	if deltaAmount < 0 {
		changeType = AgentAdjustmentTypeDecrease
	}
	adjustment := &AgentRebateAdjustment{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		profile := &AgentProfile{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ?", agentUserId).First(profile).Error; err != nil {
			return err
		}
		balanceBefore := profile.RebateBalanceAmount
		balanceAfter := balanceBefore + deltaAmount
		if balanceAfter < 0 {
			return errors.New("agent rebate balance is insufficient")
		}
		updates := map[string]interface{}{
			"rebate_balance_amount": balanceAfter,
		}
		if deltaAmount > 0 {
			updates["rebate_total_amount"] = gorm.Expr("rebate_total_amount + ?", deltaAmount)
		}
		if err := tx.Model(profile).Updates(updates).Error; err != nil {
			return err
		}
		adjustment.AgentUserId = agentUserId
		adjustment.OperatorUserId = operatorUserId
		adjustment.DeltaAmount = deltaAmount
		adjustment.BalanceBefore = balanceBefore
		adjustment.BalanceAfter = balanceAfter
		adjustment.ChangeType = changeType
		adjustment.Reason = strings.TrimSpace(reason)
		if err := tx.Create(adjustment).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	RecordLog(operatorUserId, LogTypeManage, fmt.Sprintf("调整代理返利余额，代理用户ID: %d，变动金额: %d，原因: %s", agentUserId, deltaAmount, adjustment.Reason))
	return adjustment, nil
}

func GetAgentProfiles(pageInfo *common.PageInfo, keyword string) ([]*AgentProfileView, int64, error) {
	var profiles []*AgentProfileView
	var total int64
	tx := DB.Table("agent_profiles AS ap").
		Select("ap.id, ap.user_id, u.username, u.display_name, COALESCE(ar.parent_agent_user_id, 0) AS parent_agent_user_id, COALESCE(pu.username, '') AS parent_agent_username, ap.status, ap.rebate_group_id, ag.name AS rebate_group_name, ap.custom_rate, ap.rebate_balance_amount, ap.rebate_total_amount, ap.remark, ap.created_at, ap.updated_at").
		Joins("LEFT JOIN users AS u ON u.id = ap.user_id").
		Joins("LEFT JOIN agent_rebate_groups AS ag ON ag.id = ap.rebate_group_id").
		Joins("LEFT JOIN agent_relationships AS ar ON ar.child_agent_user_id = ap.user_id AND ar.status = ?", AgentStatusEnabled).
		Joins("LEFT JOIN users AS pu ON pu.id = ar.parent_agent_user_id")
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		if keywordInt, err := strconv.Atoi(keyword); err == nil {
			tx = tx.Where("ap.user_id = ? OR u.username LIKE ? OR u.display_name LIKE ?", keywordInt, like, like)
		} else {
			tx = tx.Where("u.username LIKE ? OR u.display_name LIKE ?", like, like)
		}
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Order("ap.id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&profiles).Error; err != nil {
		return nil, 0, err
	}
	for _, profile := range profiles {
		profile.AgentLevel = AgentLevelPrimary
		if profile.ParentAgentUserId > 0 {
			profile.AgentLevel = AgentLevelSecondary
		}
		profile.EffectiveRate = profile.CustomRate
		if profile.EffectiveRate == 0 {
			profile.EffectiveRateSource = AgentRateSourceGroup
			groupRate, err := GetAgentGroupRateById(profile.RebateGroupId)
			if err == nil {
				profile.EffectiveRate = groupRate
			} else if common.AgentDefaultRebateRate > 0 {
				profile.EffectiveRate = common.AgentDefaultRebateRate
			}
		} else {
			profile.EffectiveRateSource = AgentRateSourceCustom
		}
		if profile.ParentAgentUserId > 0 {
			parentProfile, err := GetAgentProfileByUserId(profile.ParentAgentUserId)
			if err == nil {
				parentRate, rateErr := getEffectiveAgentRebateRateTx(DB, parentProfile)
				if rateErr == nil {
					profile.ParentMaxRate = parentRate
				}
			}
		}
	}
	return profiles, total, nil
}

func GetAgentGroupRateById(groupId int) (int, error) {
	if groupId <= 0 {
		return 0, errors.New("invalid group id")
	}
	var group AgentRebateGroup
	if err := DB.Select("rebate_rate").First(&group, groupId).Error; err != nil {
		return 0, err
	}
	return group.RebateRate, nil
}

func GetAgentAdjustments(pageInfo *common.PageInfo, agentUserId int) ([]*AgentRebateAdjustment, int64, error) {
	var adjustments []*AgentRebateAdjustment
	var total int64
	tx := DB.Model(&AgentRebateAdjustment{})
	if agentUserId > 0 {
		tx = tx.Where("agent_user_id = ?", agentUserId)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&adjustments).Error; err != nil {
		return nil, 0, err
	}
	return adjustments, total, nil
}

func GetAgentProfileViewByUserId(userId int) (*AgentProfileView, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	profiles, _, err := GetAgentProfiles(&common.PageInfo{Page: 1, PageSize: 1}, strconv.Itoa(userId))
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if profile.UserId == userId {
			return profile, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func GetAgentRebateRecords(pageInfo *common.PageInfo, agentUserId int) ([]*AgentRebateRecord, int64, error) {
	var records []*AgentRebateRecord
	var total int64
	tx := DB.Model(&AgentRebateRecord{})
	if agentUserId > 0 {
		tx = tx.Where("agent_user_id = ?", agentUserId)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func GetAgentSelfSummary(userId int) (*AgentSelfSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	summary := &AgentSelfSummary{
		AgentEnabled:     common.AgentEnabled,
		AgentInitialized: common.AgentInitialized,
	}
	profile, err := GetAgentProfileViewByUserId(userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return summary, nil
		}
		return nil, err
	}
	summary.IsAgent = true
	summary.Profile = profile
	var recentRebateAmount int64
	if err := DB.Model(&AgentRebateRecord{}).Where("agent_user_id = ?", userId).Count(&summary.RecentRebateCount).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&AgentRebateRecord{}).Select("COALESCE(SUM(rebate_amount), 0)").Where("agent_user_id = ?", userId).Scan(&recentRebateAmount).Error; err != nil {
		return nil, err
	}
	summary.RecentRebateAmount = recentRebateAmount
	if err := DB.Model(&AgentRebateAdjustment{}).Where("agent_user_id = ?", userId).Count(&summary.RecentAdjustmentCount).Error; err != nil {
		return nil, err
	}
	return summary, nil
}

func GetAgentAdminOverview() (*AgentAdminOverview, error) {
	overview := &AgentAdminOverview{}
	if err := DB.Model(&AgentProfile{}).Count(&overview.AgentCount).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&AgentRebateGroup{}).Count(&overview.GroupCount).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&AgentPromoLink{}).Count(&overview.PromoLinkCount).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&User{}).Where("inviter_id > 0 AND deleted_at IS NULL").Count(&overview.DownlineUserCount).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&AgentProfile{}).Select("COALESCE(SUM(rebate_balance_amount), 0)").Scan(&overview.RebateBalanceAmount).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&AgentProfile{}).Select("COALESCE(SUM(rebate_total_amount), 0)").Scan(&overview.RebateTotalAmount).Error; err != nil {
		return nil, err
	}
	return overview, nil
}

func CreateAgentUpgradeRequest(sponsorAgentUserId int, targetUserId int, targetRate int, remark string) (*AgentUpgradeRequest, error) {
	if sponsorAgentUserId <= 0 || targetUserId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if sponsorAgentUserId == targetUserId {
		return nil, errors.New("target user cannot be self")
	}
	if targetRate < 0 {
		return nil, errors.New("target rate must be >= 0")
	}
	request := &AgentUpgradeRequest{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		sponsorProfile, err := getAgentProfileByUserIdTx(tx, sponsorAgentUserId)
		if err != nil {
			return err
		}
		if sponsorProfile.Status != AgentStatusEnabled {
			return errors.New("sponsor agent is disabled")
		}
		sponsorRate, err := getEffectiveAgentRebateRateTx(tx, sponsorProfile)
		if err != nil {
			return err
		}
		if targetRate > sponsorRate {
			return errors.New("target rate exceeds sponsor agent rate")
		}
		var targetUser User
		if err := tx.Select("id", "inviter_id").First(&targetUser, targetUserId).Error; err != nil {
			return err
		}
		if targetUser.InviterId != sponsorAgentUserId {
			return errors.New("target user is not a direct downline")
		}
		if _, err := getAgentProfileByUserIdTx(tx, targetUserId); err == nil {
			return errors.New("target user is already an agent")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var pending AgentUpgradeRequest
		if err := tx.Where("sponsor_agent_user_id = ? AND target_user_id = ? AND status = ?", sponsorAgentUserId, targetUserId, AgentUpgradeRequestPending).First(&pending).Error; err == nil {
			return errors.New("upgrade request already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		request.SponsorAgentUserId = sponsorAgentUserId
		request.TargetUserId = targetUserId
		request.TargetRate = targetRate
		request.Status = AgentUpgradeRequestPending
		request.Remark = strings.TrimSpace(remark)
		return tx.Create(request).Error
	})
	if err != nil {
		return nil, err
	}
	RecordLog(sponsorAgentUserId, LogTypeManage, fmt.Sprintf("发起代理开通申请，目标用户ID: %d", targetUserId))
	return request, nil
}

func DirectUpgradeDownlineToAgent(sponsorAgentUserId int, targetUserId int, targetRate int, remark string) (*AgentProfile, error) {
	if sponsorAgentUserId <= 0 || targetUserId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if sponsorAgentUserId == targetUserId {
		return nil, errors.New("target user cannot be self")
	}
	if targetRate < 0 {
		return nil, errors.New("target rate must be >= 0")
	}
	var saved *AgentProfile
	err := DB.Transaction(func(tx *gorm.DB) error {
		sponsorProfile, err := getAgentProfileByUserIdTx(tx, sponsorAgentUserId)
		if err != nil {
			return err
		}
		if sponsorProfile.Status != AgentStatusEnabled {
			return errors.New("sponsor agent is disabled")
		}
		sponsorRate, err := getEffectiveAgentRebateRateTx(tx, sponsorProfile)
		if err != nil {
			return err
		}
		if targetRate <= 0 {
			targetRate = sponsorRate
		}
		if targetRate > sponsorRate {
			return errors.New("target rate exceeds sponsor agent rate")
		}
		var targetUser User
		if err := tx.Select("id", "inviter_id").First(&targetUser, targetUserId).Error; err != nil {
			return err
		}
		if targetUser.InviterId != sponsorAgentUserId {
			return errors.New("target user is not a direct downline")
		}
		if _, err := getAgentProfileByUserIdTx(tx, targetUserId); err == nil {
			return errors.New("target user is already an agent")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		defaultGroup, err := CreateDefaultAgentRebateGroupTx(tx, common.AgentDefaultRebateRate)
		if err != nil {
			return err
		}
		saved, err = upsertAgentProfileTx(tx, &AgentProfile{
			UserId:        targetUserId,
			Status:        AgentStatusEnabled,
			RebateGroupId: defaultGroup.Id,
			CustomRate:    targetRate,
			Remark:        strings.TrimSpace(remark),
		})
		if err != nil {
			return err
		}
		if err := ensureAgentRelationshipTx(tx, sponsorAgentUserId, targetUserId); err != nil {
			return err
		}
		return ensureAgentDefaultPromoLinkTx(tx, targetUserId)
	})
	if err != nil {
		return nil, err
	}
	RecordLog(sponsorAgentUserId, LogTypeManage, fmt.Sprintf("直属下级开通代理成功，目标用户ID: %d", targetUserId))
	return saved, nil
}

func ReviewAgentUpgradeRequest(requestId int, reviewerUserId int, approve bool, targetRate int, remark string) (*AgentUpgradeRequest, error) {
	if requestId <= 0 || reviewerUserId <= 0 {
		return nil, errors.New("invalid request id")
	}
	request := &AgentUpgradeRequest{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(request, requestId).Error; err != nil {
			return err
		}
		if request.Status != AgentUpgradeRequestPending {
			return errors.New("request is already processed")
		}
		request.ReviewerUserId = reviewerUserId
		request.ReviewedAt = common.GetTimestamp()
		request.Remark = strings.TrimSpace(remark)
		if !approve {
			request.Status = AgentUpgradeRequestRejected
			return tx.Save(request).Error
		}
		sponsorProfile, err := getAgentProfileByUserIdTx(tx, request.SponsorAgentUserId)
		if err != nil {
			return err
		}
		sponsorRate, err := getEffectiveAgentRebateRateTx(tx, sponsorProfile)
		if err != nil {
			return err
		}
		if targetRate <= 0 {
			targetRate = request.TargetRate
		}
		if targetRate > sponsorRate {
			return errors.New("target rate exceeds sponsor agent rate")
		}
		request.TargetRate = targetRate
		defaultGroup, err := CreateDefaultAgentRebateGroupTx(tx, common.AgentDefaultRebateRate)
		if err != nil {
			return err
		}
		profilePayload := &AgentProfile{
			UserId:        request.TargetUserId,
			Status:        AgentStatusEnabled,
			RebateGroupId: defaultGroup.Id,
			CustomRate:    targetRate,
			Remark:        "approved by upgrade request",
		}
		createdProfile, err := upsertAgentProfileTx(tx, profilePayload)
		if err != nil {
			return err
		}
		if createdProfile.Status == AgentStatusEnabled {
			if err := ensureAgentRelationshipTx(tx, request.SponsorAgentUserId, request.TargetUserId); err != nil {
				return err
			}
			if err := ensureAgentDefaultPromoLinkTx(tx, request.TargetUserId); err != nil {
				return err
			}
		}
		request.Status = AgentUpgradeRequestApproved
		return tx.Save(request).Error
	})
	if err != nil {
		return nil, err
	}
	return request, nil
}

func ensureAgentRelationshipTx(tx *gorm.DB, parentAgentUserId int, childAgentUserId int) error {
	if parentAgentUserId <= 0 || childAgentUserId <= 0 {
		return errors.New("invalid relationship users")
	}
	if parentAgentUserId == childAgentUserId {
		return errors.New("parent agent cannot equal child agent")
	}
	var relation AgentRelationship
	err := tx.Where("child_agent_user_id = ?", childAgentUserId).First(&relation).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		relation = AgentRelationship{
			ParentAgentUserId: parentAgentUserId,
			ChildAgentUserId:  childAgentUserId,
			Status:            AgentStatusEnabled,
		}
		return tx.Create(&relation).Error
	}
	if relation.ParentAgentUserId != parentAgentUserId {
		return errors.New("child agent already has another parent agent")
	}
	relation.Status = AgentStatusEnabled
	return tx.Save(&relation).Error
}

func upsertAgentProfileTx(tx *gorm.DB, profile *AgentProfile) (*AgentProfile, error) {
	if tx == nil || profile == nil {
		return nil, errors.New("invalid profile")
	}
	groupId := profile.RebateGroupId
	if groupId == 0 {
		defaultGroup, err := CreateDefaultAgentRebateGroupTx(tx, common.AgentDefaultRebateRate)
		if err != nil {
			return nil, err
		}
		groupId = defaultGroup.Id
	}
	var existing AgentProfile
	err := tx.Where("user_id = ?", profile.UserId).First(&existing).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		existing = AgentProfile{
			UserId:        profile.UserId,
			Status:        profile.Status,
			RebateGroupId: groupId,
			CustomRate:    profile.CustomRate,
			Remark:        profile.Remark,
		}
		if err := tx.Create(&existing).Error; err != nil {
			return nil, err
		}
		if existing.Status == AgentStatusEnabled {
			rate, rateErr := getEffectiveAgentRebateRateTx(tx, &existing)
			if rateErr != nil {
				return nil, rateErr
			}
			if err := buildAgentRateConflictErrorFromOverrides(tx, map[int]int{existing.UserId: rate}); err != nil {
				return nil, err
			}
		}
		return &existing, nil
	}
	existing.Status = profile.Status
	existing.RebateGroupId = groupId
	existing.CustomRate = profile.CustomRate
	existing.Remark = profile.Remark
	if existing.Status == AgentStatusEnabled {
		rate, rateErr := getEffectiveAgentRebateRateTx(tx, &existing)
		if rateErr != nil {
			return nil, rateErr
		}
		if err := buildAgentRateConflictErrorFromOverrides(tx, map[int]int{existing.UserId: rate}); err != nil {
			return nil, err
		}
	}
	if err := tx.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func GetAgentUpgradeRequests(pageInfo *common.PageInfo, status string) ([]*AgentUpgradeRequestView, int64, error) {
	var requests []*AgentUpgradeRequestView
	var total int64
	tx := DB.Table("agent_upgrade_requests AS aur").
		Select("aur.id, aur.sponsor_agent_user_id, su.username AS sponsor_agent_username, aur.target_user_id, tu.username AS target_username, tu.display_name AS target_display_name, aur.target_rate, aur.status, aur.remark, aur.reviewer_user_id, COALESCE(ru.username, '') AS reviewer_username, aur.reviewed_at, aur.created_at").
		Joins("LEFT JOIN users AS su ON su.id = aur.sponsor_agent_user_id").
		Joins("LEFT JOIN users AS tu ON tu.id = aur.target_user_id").
		Joins("LEFT JOIN users AS ru ON ru.id = aur.reviewer_user_id")
	status = strings.TrimSpace(status)
	if status != "" {
		tx = tx.Where("aur.status = ?", status)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Order("aur.id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&requests).Error; err != nil {
		return nil, 0, err
	}
	return requests, total, nil
}

func GenerateUniqueAgentPromoCode() (string, error) {
	return GenerateUniqueAgentPromoCodeTx(DB)
}

func GetAgentPromoLinks(pageInfo *common.PageInfo, agentUserId int, keyword string) ([]*AgentPromoLinkView, int64, error) {
	var links []*AgentPromoLinkView
	var total int64
	tx := DB.Table("agent_promo_links AS apl").
		Select("apl.id, apl.agent_user_id, u.username, u.display_name, apl.name, apl.code, apl.status, apl.landing_page, apl.remark, apl.created_at, apl.updated_at").
		Joins("LEFT JOIN users AS u ON u.id = apl.agent_user_id")
	if agentUserId > 0 {
		tx = tx.Where("apl.agent_user_id = ?", agentUserId)
	}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		if keywordInt, err := strconv.Atoi(keyword); err == nil {
			tx = tx.Where("apl.agent_user_id = ? OR apl.code LIKE ? OR apl.name LIKE ? OR u.username LIKE ? OR u.display_name LIKE ?", keywordInt, like, like, like, like)
		} else {
			tx = tx.Where("apl.code LIKE ? OR apl.name LIKE ? OR u.username LIKE ? OR u.display_name LIKE ?", like, like, like, like)
		}
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Order("apl.id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&links).Error; err != nil {
		return nil, 0, err
	}
	return links, total, nil
}

func UpsertAgentPromoLink(operatorUserId int, promoLink *AgentPromoLink) (*AgentPromoLink, error) {
	if promoLink == nil {
		return nil, errors.New("promo link is nil")
	}
	if promoLink.AgentUserId <= 0 {
		return nil, errors.New("invalid agent user id")
	}
	promoLink.Name = strings.TrimSpace(promoLink.Name)
	promoLink.Code = strings.ToUpper(NormalizePromoCode(promoLink.Code))
	promoLink.LandingPage = strings.TrimSpace(promoLink.LandingPage)
	promoLink.Remark = strings.TrimSpace(promoLink.Remark)
	if promoLink.Name == "" {
		return nil, errors.New("promo link name is required")
	}
	if promoLink.Status != AgentPromoLinkEnabled && promoLink.Status != AgentPromoLinkDisabled {
		return nil, errors.New("invalid promo link status")
	}
	if promoLink.Code == "" {
		code, err := GenerateUniqueAgentPromoCode()
		if err != nil {
			return nil, err
		}
		promoLink.Code = code
	}
	var saved AgentPromoLink
	err := DB.Transaction(func(tx *gorm.DB) error {
		var profile AgentProfile
		if err := tx.Where("user_id = ?", promoLink.AgentUserId).First(&profile).Error; err != nil {
			return err
		}
		var existingByCode AgentPromoLink
		if err := tx.Where("code = ?", promoLink.Code).First(&existingByCode).Error; err == nil {
			if promoLink.Id == 0 || existingByCode.Id != promoLink.Id {
				return errors.New("promo code already exists")
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if promoLink.Id > 0 {
			if err := tx.First(&saved, promoLink.Id).Error; err != nil {
				return err
			}
			saved.AgentUserId = promoLink.AgentUserId
			saved.Name = promoLink.Name
			saved.Code = promoLink.Code
			saved.Status = promoLink.Status
			saved.LandingPage = promoLink.LandingPage
			saved.Remark = promoLink.Remark
			return tx.Save(&saved).Error
		}
		saved = AgentPromoLink{
			AgentUserId: promoLink.AgentUserId,
			Name:        promoLink.Name,
			Code:        promoLink.Code,
			Status:      promoLink.Status,
			LandingPage: promoLink.LandingPage,
			Remark:      promoLink.Remark,
		}
		return tx.Create(&saved).Error
	})
	if err != nil {
		return nil, err
	}
	RecordLog(operatorUserId, LogTypeManage, fmt.Sprintf("更新代理推广链接，代理用户ID: %d，链接ID: %d", saved.AgentUserId, saved.Id))
	return &saved, nil
}

func DeleteAgentPromoLink(id int, operatorUserId int) error {
	if id <= 0 {
		return errors.New("invalid promo link id")
	}
	var promoLink AgentPromoLink
	if err := DB.First(&promoLink, id).Error; err != nil {
		return err
	}
	if err := DB.Delete(&promoLink).Error; err != nil {
		return err
	}
	RecordLog(operatorUserId, LogTypeManage, fmt.Sprintf("删除代理推广链接，代理用户ID: %d，链接ID: %d", promoLink.AgentUserId, promoLink.Id))
	return nil
}

func GetAgentPromoLinkStats(agentUserId int) ([]*AgentPromoLinkStat, error) {
	var stats []*AgentPromoLinkStat
	tx := DB.Table("agent_promo_links AS apl").
		Select(`apl.id AS promo_link_id, apl.agent_user_id, apl.name, apl.code, apl.status, apl.landing_page,
COALESCE(user_stats.invitee_count, 0) AS invitee_count,
COALESCE(topup_stats.topup_count, 0) AS topup_count,
COALESCE(topup_stats.topup_amount, 0) AS topup_amount,
COALESCE(rebate_stats.rebate_amount, 0) AS rebate_amount,
COALESCE(user_stats.last_invitee_id, 0) AS last_invitee_id,
COALESCE(user_stats.last_invitee_name, '') AS last_invitee_name`).
		Joins(`LEFT JOIN (
			SELECT u.promo_link_id,
			COUNT(*) AS invitee_count,
			MAX(u.id) AS last_invitee_id,
			MAX(u.username) AS last_invitee_name
			FROM users AS u
			WHERE u.promo_link_id > 0 AND u.deleted_at IS NULL
			GROUP BY u.promo_link_id
		) AS user_stats ON user_stats.promo_link_id = apl.id`).
		Joins(`LEFT JOIN (
			SELECT u.promo_link_id,
			COUNT(t.id) AS topup_count,
			COALESCE(SUM(CAST(ROUND(t.money * 100, 0) AS BIGINT)), 0) AS topup_amount
			FROM users AS u
			LEFT JOIN top_ups AS t ON t.user_id = u.id AND t.status = ?
			WHERE u.promo_link_id > 0 AND u.deleted_at IS NULL
			GROUP BY u.promo_link_id
		) AS topup_stats ON topup_stats.promo_link_id = apl.id`, common.TopUpStatusSuccess).
		Joins(`LEFT JOIN (
			SELECT promo_link_id,
			COALESCE(SUM(rebate_amount), 0) AS rebate_amount
			FROM agent_rebate_records
			WHERE promo_link_id > 0 AND status = ?
			GROUP BY promo_link_id
		) AS rebate_stats ON rebate_stats.promo_link_id = apl.id`, AgentRebateRecordSettled)
	if agentUserId > 0 {
		tx = tx.Where("apl.agent_user_id = ?", agentUserId)
	}
	err := tx.Order("apl.id desc").Scan(&stats).Error
	return stats, err
}

func GetAgentDownlineUsers(pageInfo *common.PageInfo, agentUserId int, keyword string) ([]*AgentDownlineUserView, int64, error) {
	var users []*AgentDownlineUserView
	var total int64
	tx := DB.Table("users AS u").
		Select(`u.id AS user_id, u.username, u.display_name, u.inviter_id, u.promo_link_id,
COALESCE(apl.name, '') AS promo_link_name,
CASE WHEN child_profile.user_id IS NULL THEN false ELSE true END AS is_agent,
COALESCE(topup_stats.topup_count, 0) AS topup_count,
COALESCE(topup_stats.topup_amount, 0) AS topup_amount,
COALESCE(rebate_stats.rebate_amount, 0) AS rebate_amount,
COALESCE(topup_stats.latest_topup_time, 0) AS latest_topup_time,
COALESCE(rebate_stats.latest_rebate_time, 0) AS latest_rebate_time`).
		Joins("LEFT JOIN agent_promo_links AS apl ON apl.id = u.promo_link_id").
		Joins("LEFT JOIN agent_profiles AS child_profile ON child_profile.user_id = u.id AND child_profile.status = ?", AgentStatusEnabled).
		Joins(`LEFT JOIN (
			SELECT user_id,
			COUNT(id) AS topup_count,
			COALESCE(SUM(CAST(ROUND(money * 100, 0) AS BIGINT)), 0) AS topup_amount,
			MAX(complete_time) AS latest_topup_time
			FROM top_ups
			WHERE status = ?
			GROUP BY user_id
		) AS topup_stats ON topup_stats.user_id = u.id`, common.TopUpStatusSuccess).
		Joins(`LEFT JOIN (
			SELECT invitee_user_id,
			COALESCE(SUM(rebate_amount), 0) AS rebate_amount,
			MAX(settled_at) AS latest_rebate_time
			FROM agent_rebate_records
			WHERE status = ?
			GROUP BY invitee_user_id
		) AS rebate_stats ON rebate_stats.invitee_user_id = u.id`, AgentRebateRecordSettled).
		Where("u.inviter_id = ? AND u.deleted_at IS NULL", agentUserId)
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		if keywordInt, err := strconv.Atoi(keyword); err == nil {
			tx = tx.Where("u.id = ? OR u.username LIKE ? OR u.display_name LIKE ?", keywordInt, like, like)
		} else {
			tx = tx.Where("u.username LIKE ? OR u.display_name LIKE ?", like, like)
		}
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Order("u.id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}
