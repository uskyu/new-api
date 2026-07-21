package model

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model/queryx"
	"github.com/shopspring/decimal"

	"gorm.io/gorm"
)

const (
	AgentStatusEnabled  = 1
	AgentStatusDisabled = 0
	AgentLevelPrimary   = 1
	AgentLevelSecondary = 2
	AgentMaxRebateRate  = 10000

	AgentPromoLinkEnabled  = 1
	AgentPromoLinkDisabled = 0

	AgentRebateRecordSettled  = "settled"
	AgentRebateRecordCanceled = "canceled"
	AgentRebateRecordRolled   = "rolled_back"

	AgentRebateSourceEPay       = "epay"
	AgentRebateSourceManual     = "manual"
	AgentRebateSourceRedemption = "redemption"

	AgentAdjustmentTypeIncrease   = "increase"
	AgentAdjustmentTypeDecrease   = "decrease"
	AgentLedgerTypeRebateIncome   = "rebate_income"
	AgentLedgerTypeAdminAdjust    = "admin_adjust"
	AgentLedgerTypeWithdrawFreeze = "withdraw_freeze"
	AgentLedgerTypeWithdrawPaid   = "withdraw_paid"

	AgentRateSourceGroup  = "group"
	AgentRateSourceCustom = "custom"

	AgentUpgradeRequestPending  = "pending"
	AgentUpgradeRequestApproved = "approved"
	AgentUpgradeRequestRejected = "rejected"

	AgentWithdrawStatusPending  = "pending"
	AgentWithdrawStatusExported = "exported"
	AgentWithdrawStatusPaid     = "paid"

	AgentWithdrawChannelAlipay = "alipay"

	DefaultAgentRebateGroupName = "default"
)

func validateAgentRebateRate(rate int) error {
	if rate < 0 || rate > AgentMaxRebateRate {
		return fmt.Errorf("rebate rate must be between 0 and %d", AgentMaxRebateRate)
	}
	return nil
}

// AgentRebateGroup stores default rebate rates for different agent groups.
// RebateRate is stored in basis points, e.g. 1500 = 15.00%.
type AgentRebateGroup struct {
	Id         int    `json:"id"`
	Name       string `json:"name" gorm:"type:varchar(64);uniqueIndex;not null"`
	RebateRate int    `json:"rebate_rate" gorm:"type:int;not null;default:0"`
	Status     int    `json:"status" gorm:"type:int;not null;default:1;index"`
	IsDefault  bool   `json:"is_default" gorm:"not null"`
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
	RebateFrozenAmount  int64  `json:"rebate_frozen_amount" gorm:"type:bigint;not null;default:0"`
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

type AgentRebateRecordView struct {
	Id            int    `json:"id"`
	RecordKey     string `json:"record_key"`
	RecordType    string `json:"record_type"`
	RecordId      int    `json:"record_id"`
	TopUpId       int    `json:"topup_id"`
	TradeNo       string `json:"trade_no"`
	SourceType    string `json:"source_type"`
	InviteeUserId int    `json:"invitee_user_id"`
	AgentUserId   int    `json:"agent_user_id"`
	PromoLinkId   int    `json:"promo_link_id"`
	RedeemQuota   int    `json:"redeem_quota"`
	PayAmount     int64  `json:"pay_amount"`
	RebateRate    int    `json:"rebate_rate"`
	RebateAmount  int64  `json:"rebate_amount"`
	Status        string `json:"status"`
	Remark        string `json:"remark"`
	CreatedAt     int64  `json:"created_at"`
	SettledAt     int64  `json:"settled_at"`
}

// AgentRedemptionRebateRecord stores rebates created by successful redemption-code usage.
// PayAmount and RebateAmount are stored in the smallest currency unit.
type AgentRedemptionRebateRecord struct {
	Id            int    `json:"id"`
	RedemptionId  int    `json:"redemption_id" gorm:"not null;uniqueIndex"`
	ReferenceNo   string `json:"reference_no" gorm:"type:varchar(255);not null;index"`
	SourceType    string `json:"source_type" gorm:"type:varchar(32);not null;index"`
	InviteeUserId int    `json:"invitee_user_id" gorm:"not null;index"`
	AgentUserId   int    `json:"agent_user_id" gorm:"not null;index"`
	PromoLinkId   int    `json:"promo_link_id" gorm:"type:int;not null;default:0;index"`
	RedeemQuota   int    `json:"redeem_quota" gorm:"type:int;not null;default:0"`
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

type AgentWithdrawAccount struct {
	Id          int    `json:"id"`
	AgentUserId int    `json:"agent_user_id" gorm:"not null;uniqueIndex"`
	ChannelType string `json:"channel_type" gorm:"type:varchar(32);not null;default:'alipay'"`
	AccountNo   string `json:"account_no" gorm:"type:varchar(128);not null"`
	AccountName string `json:"account_name" gorm:"type:varchar(64);not null"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
}

func (a *AgentWithdrawAccount) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	a.CreatedAt = now
	a.UpdatedAt = now
	return nil
}

func (a *AgentWithdrawAccount) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = common.GetTimestamp()
	return nil
}

type AgentWithdrawRequest struct {
	Id                  int    `json:"id"`
	AgentUserId         int    `json:"agent_user_id" gorm:"not null;index"`
	Amount              int64  `json:"amount" gorm:"type:bigint;not null;default:0"`
	Status              string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	AccountNoSnapshot   string `json:"account_no_snapshot" gorm:"type:varchar(128);not null"`
	AccountNameSnapshot string `json:"account_name_snapshot" gorm:"type:varchar(64);not null"`
	ExportBatchNo       string `json:"export_batch_no" gorm:"type:varchar(64);default:'';index"`
	ExternalOrderNo     string `json:"external_order_no" gorm:"type:varchar(128);default:'';index"`
	Remark              string `json:"remark" gorm:"type:varchar(255);default:''"`
	CreatedAt           int64  `json:"created_at" gorm:"bigint;index"`
	ProcessedAt         int64  `json:"processed_at" gorm:"bigint"`
}

func (r *AgentWithdrawRequest) BeforeCreate(tx *gorm.DB) error {
	if r.CreatedAt == 0 {
		r.CreatedAt = common.GetTimestamp()
	}
	return nil
}

type AgentBalanceLedger struct {
	Id            int    `json:"id"`
	AgentUserId   int    `json:"agent_user_id" gorm:"not null;index"`
	ChangeType    string `json:"change_type" gorm:"type:varchar(32);not null;index"`
	Amount        int64  `json:"amount" gorm:"type:bigint;not null"`
	BalanceBefore int64  `json:"balance_before" gorm:"type:bigint;not null;default:0"`
	BalanceAfter  int64  `json:"balance_after" gorm:"type:bigint;not null;default:0"`
	FrozenBefore  int64  `json:"frozen_before" gorm:"type:bigint;not null;default:0"`
	FrozenAfter   int64  `json:"frozen_after" gorm:"type:bigint;not null;default:0"`
	ReferenceType string `json:"reference_type" gorm:"type:varchar(32);default:'';index"`
	ReferenceId   int    `json:"reference_id" gorm:"type:int;not null;default:0;index"`
	Remark        string `json:"remark" gorm:"type:varchar(255);default:''"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;index"`
}

func (l *AgentBalanceLedger) BeforeCreate(tx *gorm.DB) error {
	if l.CreatedAt == 0 {
		l.CreatedAt = common.GetTimestamp()
	}
	return nil
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
	RebateFrozenAmount  int64  `json:"rebate_frozen_amount"`
	RebateTotalAmount   int64  `json:"rebate_total_amount"`
	Remark              string `json:"remark"`
	CreatedAt           int64  `json:"created_at"`
	UpdatedAt           int64  `json:"updated_at"`
}

type AgentSelfSummary struct {
	AgentEnabled          bool                  `json:"agent_enabled"`
	AgentInitialized      bool                  `json:"agent_initialized"`
	IsAgent               bool                  `json:"is_agent"`
	Profile               *AgentProfileView     `json:"profile,omitempty"`
	WithdrawAccount       *AgentWithdrawAccount `json:"withdraw_account,omitempty"`
	RecentRebateCount     int64                 `json:"recent_rebate_count"`
	RecentRebateAmount    int64                 `json:"recent_rebate_amount"`
	RecentAdjustmentCount int64                 `json:"recent_adjustment_count"`
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

type AgentTransferDownlineUserView struct {
	UserId        int    `json:"user_id"`
	Username      string `json:"username"`
	DisplayName   string `json:"display_name"`
	InviterId     int    `json:"inviter_id"`
	PromoLinkId   int    `json:"promo_link_id"`
	PromoLinkName string `json:"promo_link_name"`
	IsAgent       bool   `json:"is_agent"`
}

type AgentAdminOverview struct {
	AgentCount          int64 `json:"agent_count"`
	GroupCount          int64 `json:"group_count"`
	PromoLinkCount      int64 `json:"promo_link_count"`
	DownlineUserCount   int64 `json:"downline_user_count"`
	RebateBalanceAmount int64 `json:"rebate_balance_amount"`
	RebateTotalAmount   int64 `json:"rebate_total_amount"`
}

type AgentDailyMetric struct {
	Date                   string `json:"date"`
	AgentUserId            int    `json:"agent_user_id"`
	Username               string `json:"username"`
	DisplayName            string `json:"display_name"`
	NewUserCount           int64  `json:"new_user_count"`
	TopupCount             int64  `json:"topup_count"`
	TopupAmount            int64  `json:"topup_amount"`
	TopupRebateAmount      int64  `json:"topup_rebate_amount"`
	RedemptionRebateAmount int64  `json:"redemption_rebate_amount"`
	TotalRebateAmount      int64  `json:"total_rebate_amount"`
}

type AgentDailyMetricsSummary struct {
	NewUserCount      int64 `json:"new_user_count"`
	TopupCount        int64 `json:"topup_count"`
	TopupAmount       int64 `json:"topup_amount"`
	TotalRebateAmount int64 `json:"total_rebate_amount"`
}

type AgentDailyMetricsResult struct {
	StartDate string                   `json:"start_date"`
	EndDate   string                   `json:"end_date"`
	Summary   AgentDailyMetricsSummary `json:"summary"`
	Items     []*AgentDailyMetric      `json:"items"`
}

type AgentWithdrawRequestView struct {
	Id                  int    `json:"id"`
	AgentUserId         int    `json:"agent_user_id"`
	Username            string `json:"username"`
	DisplayName         string `json:"display_name"`
	Email               string `json:"email"`
	Amount              int64  `json:"amount"`
	Status              string `json:"status"`
	AccountNoSnapshot   string `json:"account_no_snapshot"`
	AccountNameSnapshot string `json:"account_name_snapshot"`
	ExportBatchNo       string `json:"export_batch_no"`
	ExternalOrderNo     string `json:"external_order_no"`
	Remark              string `json:"remark"`
	CreatedAt           int64  `json:"created_at"`
	ProcessedAt         int64  `json:"processed_at"`
}

type AgentWithdrawImportResult struct {
	Processed  int   `json:"processed"`
	RequestIds []int `json:"request_ids"`
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

func (r *AgentRedemptionRebateRecord) BeforeCreate(tx *gorm.DB) error {
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
	if err := validateAgentRebateRate(rebateRate); err != nil {
		return nil, err
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
	RefreshAgentRuntimeOptions()
	if tx == nil || topUp == nil {
		return errors.New("invalid settlement params")
	}
	if !common.AgentEnabled || !common.AgentInitialized {
		return nil
	}
	if topUp.Id <= 0 || topUp.UserId <= 0 {
		return nil
	}
	var invitee User
	if err := lockForUpdate(tx).Select("id", "inviter_id", "promo_link_id").First(&invitee, topUp.UserId).Error; err != nil {
		return err
	}
	if invitee.InviterId <= 0 || invitee.InviterId == invitee.Id {
		return nil
	}
	var profile AgentProfile
	if err := lockForUpdate(tx).Where("user_id = ?", invitee.InviterId).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if profile.Status != AgentStatusEnabled {
		return nil
	}
	var existing AgentRebateRecord
	if err := tx.Where("top_up_id = ?", topUp.Id).First(&existing).Error; err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
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
	balanceBefore := profile.RebateBalanceAmount
	balanceAfter := balanceBefore + rebateAmount
	if err := tx.Model(&AgentProfile{}).Where("id = ?", profile.Id).Updates(map[string]interface{}{
		"rebate_balance_amount": balanceAfter,
		"rebate_total_amount":   profile.RebateTotalAmount + rebateAmount,
	}).Error; err != nil {
		return err
	}
	return createAgentBalanceLedgerTx(tx, &AgentBalanceLedger{
		AgentUserId:   profile.UserId,
		ChangeType:    AgentLedgerTypeRebateIncome,
		Amount:        rebateAmount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		FrozenBefore:  profile.RebateFrozenAmount,
		FrozenAfter:   profile.RebateFrozenAmount,
		ReferenceType: "rebate_record",
		ReferenceId:   record.Id,
		Remark:        sourceType,
	})
}

func SettleAgentRedemptionRebateTx(tx *gorm.DB, redemption *Redemption, inviteeUserId int) error {
	RefreshAgentRuntimeOptions()
	if tx == nil || redemption == nil {
		return errors.New("invalid redemption settlement params")
	}
	if !common.AgentEnabled || !common.AgentInitialized {
		return nil
	}
	if redemption.Id <= 0 || inviteeUserId <= 0 || redemption.Quota <= 0 {
		return nil
	}
	var invitee User
	if err := lockForUpdate(tx).Select("id", "inviter_id", "promo_link_id").First(&invitee, inviteeUserId).Error; err != nil {
		return err
	}
	if invitee.InviterId <= 0 || invitee.InviterId == invitee.Id {
		return nil
	}
	var profile AgentProfile
	if err := lockForUpdate(tx).Where("user_id = ?", invitee.InviterId).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if profile.Status != AgentStatusEnabled {
		return nil
	}
	var existing AgentRedemptionRebateRecord
	if err := tx.Where("redemption_id = ?", redemption.Id).First(&existing).Error; err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	rebateRate, err := getEffectiveAgentRebateRateTx(tx, &profile)
	if err != nil {
		return err
	}
	if rebateRate <= 0 {
		return nil
	}
	payAmount := convertQuotaToMinorUnit(redemption.Quota)
	if payAmount <= 0 {
		return nil
	}
	rebateAmount := decimal.NewFromInt(payAmount).Mul(decimal.NewFromInt(int64(rebateRate))).Div(decimal.NewFromInt(10000)).Round(0).IntPart()
	if rebateAmount <= 0 {
		return nil
	}
	record := AgentRedemptionRebateRecord{
		RedemptionId:  redemption.Id,
		ReferenceNo:   fmt.Sprintf("redemption:%d", redemption.Id),
		SourceType:    AgentRebateSourceRedemption,
		InviteeUserId: invitee.Id,
		AgentUserId:   profile.UserId,
		PromoLinkId:   invitee.PromoLinkId,
		RedeemQuota:   redemption.Quota,
		PayAmount:     payAmount,
		RebateRate:    rebateRate,
		RebateAmount:  rebateAmount,
		Status:        AgentRebateRecordSettled,
	}
	if err := tx.Create(&record).Error; err != nil {
		return err
	}
	balanceBefore := profile.RebateBalanceAmount
	balanceAfter := balanceBefore + rebateAmount
	if err := tx.Model(&AgentProfile{}).Where("id = ?", profile.Id).Updates(map[string]interface{}{
		"rebate_balance_amount": balanceAfter,
		"rebate_total_amount":   profile.RebateTotalAmount + rebateAmount,
	}).Error; err != nil {
		return err
	}
	return createAgentBalanceLedgerTx(tx, &AgentBalanceLedger{
		AgentUserId:   profile.UserId,
		ChangeType:    AgentLedgerTypeRebateIncome,
		Amount:        rebateAmount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		FrozenBefore:  profile.RebateFrozenAmount,
		FrozenAfter:   profile.RebateFrozenAmount,
		ReferenceType: "redemption_rebate_record",
		ReferenceId:   record.Id,
		Remark:        AgentRebateSourceRedemption,
	})
}

func getEffectiveAgentRebateRateTx(tx *gorm.DB, profile *AgentProfile) (int, error) {
	RefreshAgentRuntimeOptions()
	if profile == nil {
		return 0, nil
	}
	if profile.CustomRate > 0 {
		if err := validateAgentRebateRate(profile.CustomRate); err != nil {
			return 0, err
		}
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
			if err := validateAgentRebateRate(group.RebateRate); err != nil {
				return 0, err
			}
			return group.RebateRate, nil
		}
	}
	if common.AgentDefaultRebateRate > 0 {
		if err := validateAgentRebateRate(common.AgentDefaultRebateRate); err != nil {
			return 0, err
		}
		return common.AgentDefaultRebateRate, nil
	}
	var defaultGroup AgentRebateGroup
	if err := tx.Where("is_default = ?", true).First(&defaultGroup).Error; err == nil && defaultGroup.Status == AgentStatusEnabled {
		if err := validateAgentRebateRate(defaultGroup.RebateRate); err != nil {
			return 0, err
		}
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

// lockAgentHierarchyTx serializes the infrequent hierarchy and rate mutations.
// Locking every agent profile prevents two concurrent parent changes from both
// passing cycle validation against an incomplete view of the graph.
func lockAgentHierarchyTx(tx *gorm.DB) error {
	var userIds []int
	return lockForUpdate(tx).
		Model(&AgentProfile{}).
		Order("user_id asc").
		Pluck("user_id", &userIds).Error
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

func createAgentBalanceLedgerTx(tx *gorm.DB, ledger *AgentBalanceLedger) error {
	if tx == nil || ledger == nil {
		return errors.New("invalid balance ledger")
	}
	return tx.Create(ledger).Error
}

func upsertAgentWithdrawAccountTx(tx *gorm.DB, agentUserId int, accountName string, accountNo string) (*AgentWithdrawAccount, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	accountName = strings.TrimSpace(accountName)
	accountNo = strings.TrimSpace(accountNo)
	if accountName == "" || accountNo == "" {
		return nil, errors.New("withdraw account info is incomplete")
	}
	var account AgentWithdrawAccount
	err := tx.Where("agent_user_id = ?", agentUserId).First(&account).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		account = AgentWithdrawAccount{
			AgentUserId: agentUserId,
			ChannelType: AgentWithdrawChannelAlipay,
			AccountNo:   accountNo,
			AccountName: accountName,
		}
		if err := tx.Create(&account).Error; err != nil {
			return nil, err
		}
		return &account, nil
	}
	account.AccountName = accountName
	account.AccountNo = accountNo
	account.ChannelType = AgentWithdrawChannelAlipay
	if err := tx.Save(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func GetAgentWithdrawAccount(agentUserId int) (*AgentWithdrawAccount, error) {
	if agentUserId <= 0 {
		return nil, errors.New("invalid agent user id")
	}
	var account AgentWithdrawAccount
	if err := DB.Where("agent_user_id = ?", agentUserId).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func CreateAgentWithdrawRequest(agentUserId int, accountName string, accountNo string, amount int64, remark string) (*AgentWithdrawRequest, error) {
	if agentUserId <= 0 {
		return nil, errors.New("invalid agent user id")
	}
	if amount <= 0 {
		return nil, errors.New("withdraw amount must be greater than 0")
	}
	request := &AgentWithdrawRequest{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		profile := &AgentProfile{}
		if err := lockForUpdate(tx).Where("user_id = ?", agentUserId).First(profile).Error; err != nil {
			return err
		}
		if profile.Status != AgentStatusEnabled {
			return errors.New("agent is disabled")
		}
		if profile.RebateBalanceAmount < amount {
			return errors.New("agent rebate balance is insufficient")
		}
		account, err := upsertAgentWithdrawAccountTx(tx, agentUserId, accountName, accountNo)
		if err != nil {
			return err
		}
		balanceBefore := profile.RebateBalanceAmount
		frozenBefore := profile.RebateFrozenAmount
		balanceAfter := balanceBefore - amount
		frozenAfter := frozenBefore + amount
		if err := tx.Model(profile).Updates(map[string]interface{}{
			"rebate_balance_amount": balanceAfter,
			"rebate_frozen_amount":  frozenAfter,
		}).Error; err != nil {
			return err
		}
		request.AgentUserId = agentUserId
		request.Amount = amount
		request.Status = AgentWithdrawStatusPending
		request.AccountNoSnapshot = account.AccountNo
		request.AccountNameSnapshot = account.AccountName
		request.Remark = strings.TrimSpace(remark)
		if err := tx.Create(request).Error; err != nil {
			return err
		}
		return createAgentBalanceLedgerTx(tx, &AgentBalanceLedger{
			AgentUserId:   agentUserId,
			ChangeType:    AgentLedgerTypeWithdrawFreeze,
			Amount:        amount,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balanceAfter,
			FrozenBefore:  frozenBefore,
			FrozenAfter:   frozenAfter,
			ReferenceType: "withdraw_request",
			ReferenceId:   request.Id,
			Remark:        "withdraw request created",
		})
	})
	if err != nil {
		return nil, err
	}
	return request, nil
}

func GetAgentWithdrawRequests(pageInfo *common.PageInfo, agentUserId int, status string, startDate string, endDate string) ([]*AgentWithdrawRequestView, int64, error) {
	var requests []*AgentWithdrawRequestView
	var total int64
	tx := DB.Table("agent_withdraw_requests AS awr").
		Select("awr.id, awr.agent_user_id, u.username, u.display_name, u.email, awr.amount, awr.status, awr.account_no_snapshot, awr.account_name_snapshot, awr.export_batch_no, awr.external_order_no, awr.remark, awr.created_at, awr.processed_at").
		Joins("LEFT JOIN users AS u ON u.id = awr.agent_user_id")
	if agentUserId > 0 {
		tx = tx.Where("awr.agent_user_id = ?", agentUserId)
	}
	status = strings.TrimSpace(status)
	if status != "" {
		tx = tx.Where("awr.status = ?", status)
	}
	if startTs, endTs, ok := parseWithdrawDateRange(startDate, endDate); ok {
		tx = tx.Where("awr.created_at >= ? AND awr.created_at <= ?", startTs, endTs)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Order("awr.id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&requests).Error; err != nil {
		return nil, 0, err
	}
	return requests, total, nil
}

func parseWithdrawDateRange(startDate string, endDate string) (int64, int64, bool) {
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)
	if startDate == "" || endDate == "" {
		return 0, 0, false
	}
	startTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return 0, 0, false
	}
	endTime, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return 0, 0, false
	}
	return startTime.Unix(), endTime.Add(24*time.Hour - time.Second).Unix(), true
}

func ExportAgentWithdrawRequests(status string, startDate string, endDate string) ([]byte, string, error) {
	batchNo := fmt.Sprintf("WD-%d", common.GetTimestamp())
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := writer.Write([]string{"request_id", "username", "email", "account_name", "account_no", "amount", "status", "external_order_no"}); err != nil {
		return nil, "", err
	}
	pageInfo := &common.PageInfo{Page: 1, PageSize: 100000}
	requests, _, err := GetAgentWithdrawRequests(pageInfo, 0, status, startDate, endDate)
	if err != nil {
		return nil, "", err
	}
	for _, request := range requests {
		if request.Status == AgentWithdrawStatusPending {
			if err := DB.Model(&AgentWithdrawRequest{}).Where("id = ?", request.Id).Updates(map[string]interface{}{
				"status":          AgentWithdrawStatusExported,
				"export_batch_no": batchNo,
			}).Error; err != nil {
				return nil, "", err
			}
			request.Status = AgentWithdrawStatusExported
			request.ExportBatchNo = batchNo
		}
		if err := writer.Write([]string{
			strconv.Itoa(request.Id),
			request.Username,
			request.Email,
			request.AccountNameSnapshot,
			request.AccountNoSnapshot,
			decimal.NewFromInt(request.Amount).Div(decimal.NewFromInt(100)).StringFixed(2),
			request.Status,
			request.ExternalOrderNo,
		}); err != nil {
			return nil, "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), batchNo, nil
}

func ImportAgentWithdrawResults(reader io.Reader) (*AgentWithdrawImportResult, error) {
	if reader == nil {
		return nil, errors.New("import file is required")
	}
	csvReader := csv.NewReader(reader)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	result := &AgentWithdrawImportResult{Processed: 0, RequestIds: make([]int, 0)}
	if len(records) <= 1 {
		return result, nil
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		for idx, row := range records {
			if idx == 0 {
				continue
			}
			if len(row) < 8 {
				continue
			}
			requestId, parseErr := strconv.Atoi(strings.TrimSpace(row[0]))
			if parseErr != nil || requestId <= 0 {
				continue
			}
			externalOrderNo := strings.TrimSpace(row[7])
			if externalOrderNo == "" {
				continue
			}
			withdrawRequest := &AgentWithdrawRequest{}
			if err := lockForUpdate(tx).First(withdrawRequest, requestId).Error; err != nil {
				return err
			}
			if withdrawRequest.Status == AgentWithdrawStatusPaid {
				continue
			}
			profile := &AgentProfile{}
			if err := lockForUpdate(tx).Where("user_id = ?", withdrawRequest.AgentUserId).First(profile).Error; err != nil {
				return err
			}
			balanceBefore := profile.RebateBalanceAmount
			frozenBefore := profile.RebateFrozenAmount
			frozenAfter := frozenBefore - withdrawRequest.Amount
			if frozenAfter < 0 {
				return errors.New("withdraw frozen amount is insufficient")
			}
			if err := tx.Model(profile).Updates(map[string]interface{}{
				"rebate_frozen_amount": frozenAfter,
			}).Error; err != nil {
				return err
			}
			withdrawRequest.Status = AgentWithdrawStatusPaid
			withdrawRequest.ExternalOrderNo = externalOrderNo
			withdrawRequest.ProcessedAt = common.GetTimestamp()
			if err := tx.Save(withdrawRequest).Error; err != nil {
				return err
			}
			if err := createAgentBalanceLedgerTx(tx, &AgentBalanceLedger{
				AgentUserId:   withdrawRequest.AgentUserId,
				ChangeType:    AgentLedgerTypeWithdrawPaid,
				Amount:        withdrawRequest.Amount,
				BalanceBefore: balanceBefore,
				BalanceAfter:  balanceBefore,
				FrozenBefore:  frozenBefore,
				FrozenAfter:   frozenAfter,
				ReferenceType: "withdraw_request",
				ReferenceId:   withdrawRequest.Id,
				Remark:        externalOrderNo,
			}); err != nil {
				return err
			}
			result.Processed++
			result.RequestIds = append(result.RequestIds, withdrawRequest.Id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func convertMoneyToMinorUnit(amount float64) int64 {
	return decimal.NewFromFloat(amount).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}

func convertQuotaToMinorUnit(quota int) int64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return decimal.NewFromInt(int64(quota)).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Mul(decimal.NewFromInt(100)).
		Round(0).
		IntPart()
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
	if err := validateAgentRebateRate(group.RebateRate); err != nil {
		return nil, err
	}
	if group.Status != AgentStatusEnabled && group.Status != AgentStatusDisabled {
		return nil, errors.New("invalid group status")
	}
	var saved AgentRebateGroup
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockAgentHierarchyTx(tx); err != nil {
			return err
		}
		overrides := make(map[int]int)
		var existing AgentRebateGroup
		if err := lockForUpdate(tx).Where("name = ?", group.Name).First(&existing).Error; err == nil {
			if group.Id == 0 || existing.Id != group.Id {
				return errors.New("group name already exists")
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if group.Id > 0 {
			if err := lockForUpdate(tx).First(&saved, group.Id).Error; err != nil {
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
	if err := validateAgentRebateRate(profile.CustomRate); err != nil {
		return nil, err
	}
	if profile.RebateGroupId < 0 {
		return nil, errors.New("invalid rebate group")
	}
	var saved *AgentProfile
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id").First(&user, profile.UserId).Error; err != nil {
			return err
		}
		if err := lockAgentHierarchyTx(tx); err != nil {
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
		if err := lockForUpdate(tx).Where("user_id = ?", agentUserId).First(profile).Error; err != nil {
			return err
		}
		balanceBefore := profile.RebateBalanceAmount
		if deltaAmount > 0 && balanceBefore > math.MaxInt64-deltaAmount {
			return errors.New("agent rebate balance exceeds supported range")
		}
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
		return createAgentBalanceLedgerTx(tx, &AgentBalanceLedger{
			AgentUserId:   agentUserId,
			ChangeType:    AgentLedgerTypeAdminAdjust,
			Amount:        deltaAmount,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balanceAfter,
			FrozenBefore:  profile.RebateFrozenAmount,
			FrozenAfter:   profile.RebateFrozenAmount,
			ReferenceType: "rebate_adjustment",
			ReferenceId:   adjustment.Id,
			Remark:        adjustment.Reason,
		})
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
		Select("ap.id, ap.user_id, u.username, u.display_name, COALESCE(ar.parent_agent_user_id, 0) AS parent_agent_user_id, COALESCE(pu.username, '') AS parent_agent_username, ap.status, ap.rebate_group_id, ag.name AS rebate_group_name, ap.custom_rate, ap.rebate_balance_amount, ap.rebate_frozen_amount, ap.rebate_total_amount, ap.remark, ap.created_at, ap.updated_at").
		Joins("LEFT JOIN users AS u ON u.id = ap.user_id").
		Joins("LEFT JOIN agent_rebate_groups AS ag ON ag.id = ap.rebate_group_id").
		Joins("LEFT JOIN agent_relationships AS ar ON ar.child_agent_user_id = ap.user_id AND ar.status = ?", AgentStatusEnabled).
		Joins("LEFT JOIN users AS pu ON pu.id = ar.parent_agent_user_id")
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		if keywordInt, err := strconv.Atoi(keyword); err == nil {
			tx = tx.Where("(ap.id = ? OR ap.user_id = ? OR u.username LIKE ? OR u.display_name LIKE ?)", keywordInt, keywordInt, like, like)
		} else {
			tx = tx.Where("(u.username LIKE ? OR u.display_name LIKE ?)", like, like)
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
	var profile AgentProfileView
	err := DB.Table("agent_profiles AS ap").
		Select("ap.id, ap.user_id, u.username, u.display_name, COALESCE(ar.parent_agent_user_id, 0) AS parent_agent_user_id, COALESCE(pu.username, '') AS parent_agent_username, ap.status, ap.rebate_group_id, ag.name AS rebate_group_name, ap.custom_rate, ap.rebate_balance_amount, ap.rebate_frozen_amount, ap.rebate_total_amount, ap.remark, ap.created_at, ap.updated_at").
		Joins("LEFT JOIN users AS u ON u.id = ap.user_id").
		Joins("LEFT JOIN agent_rebate_groups AS ag ON ag.id = ap.rebate_group_id").
		Joins("LEFT JOIN agent_relationships AS ar ON ar.child_agent_user_id = ap.user_id AND ar.status = ?", AgentStatusEnabled).
		Joins("LEFT JOIN users AS pu ON pu.id = ar.parent_agent_user_id").
		Where("ap.user_id = ?", userId).
		Take(&profile).Error
	if err != nil {
		return nil, err
	}
	profile.AgentLevel = AgentLevelPrimary
	if profile.ParentAgentUserId > 0 {
		profile.AgentLevel = AgentLevelSecondary
	}
	profile.EffectiveRate = profile.CustomRate
	if profile.EffectiveRate == 0 {
		profile.EffectiveRateSource = AgentRateSourceGroup
		groupRate, groupErr := GetAgentGroupRateById(profile.RebateGroupId)
		if groupErr == nil {
			profile.EffectiveRate = groupRate
		} else if common.AgentDefaultRebateRate > 0 {
			profile.EffectiveRate = common.AgentDefaultRebateRate
		}
	} else {
		profile.EffectiveRateSource = AgentRateSourceCustom
	}
	if profile.ParentAgentUserId > 0 {
		parentProfile, parentErr := GetAgentProfileByUserId(profile.ParentAgentUserId)
		if parentErr == nil {
			parentRate, rateErr := getEffectiveAgentRebateRateTx(DB, parentProfile)
			if rateErr == nil {
				profile.ParentMaxRate = parentRate
			}
		}
	}
	return &profile, nil
}

func GetAgentRebateRecords(pageInfo *common.PageInfo, agentUserId int) ([]*AgentRebateRecordView, int64, error) {
	var records []*AgentRebateRecordView
	var topupTotal int64
	topupTx := DB.Model(&AgentRebateRecord{})
	if agentUserId > 0 {
		topupTx = topupTx.Where("agent_user_id = ?", agentUserId)
	}
	if err := topupTx.Count(&topupTotal).Error; err != nil {
		return nil, 0, err
	}
	var redemptionTotal int64
	redemptionTx := DB.Model(&AgentRedemptionRebateRecord{})
	if agentUserId > 0 {
		redemptionTx = redemptionTx.Where("agent_user_id = ?", agentUserId)
	}
	if err := redemptionTx.Count(&redemptionTotal).Error; err != nil {
		return nil, 0, err
	}

	params := make([]interface{}, 0, 4)
	topupWhere := ""
	redemptionWhere := ""
	if agentUserId > 0 {
		topupWhere = "WHERE agent_user_id = ?"
		redemptionWhere = "WHERE agent_user_id = ?"
		params = append(params, agentUserId, agentUserId)
	}
	query := fmt.Sprintf(`SELECT *
FROM (
	SELECT id, 'topup' AS record_type, id AS record_id, top_up_id, trade_no, source_type, invitee_user_id, agent_user_id, promo_link_id, 0 AS redeem_quota, pay_amount, rebate_rate, rebate_amount, status, remark, created_at, settled_at
	FROM agent_rebate_records
	%s
	UNION ALL
	SELECT id, 'redemption' AS record_type, id AS record_id, 0 AS top_up_id, reference_no AS trade_no, source_type, invitee_user_id, agent_user_id, promo_link_id, redeem_quota, pay_amount, rebate_rate, rebate_amount, status, remark, created_at, settled_at
	FROM agent_redemption_rebate_records
	%s
) AS rebate_records
ORDER BY settled_at DESC, id DESC
LIMIT ? OFFSET ?`, topupWhere, redemptionWhere)
	params = append(params, pageInfo.GetPageSize(), pageInfo.GetStartIdx())
	if err := DB.Raw(query, params...).Scan(&records).Error; err != nil {
		return nil, 0, err
	}
	for _, record := range records {
		record.RecordKey = fmt.Sprintf("%s:%d", record.RecordType, record.RecordId)
	}
	return records, topupTotal + redemptionTotal, nil
}

func GetAgentSelfSummary(userId int) (*AgentSelfSummary, error) {
	RefreshAgentRuntimeOptions()
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
	if account, err := GetAgentWithdrawAccount(userId); err == nil {
		summary.WithdrawAccount = account
	}
	var topupRebateCount int64
	if err := DB.Model(&AgentRebateRecord{}).Where("agent_user_id = ?", userId).Count(&topupRebateCount).Error; err != nil {
		return nil, err
	}
	var redemptionRebateCount int64
	if err := DB.Model(&AgentRedemptionRebateRecord{}).Where("agent_user_id = ?", userId).Count(&redemptionRebateCount).Error; err != nil {
		return nil, err
	}
	summary.RecentRebateCount = topupRebateCount + redemptionRebateCount
	var topupRebateAmount int64
	if err := DB.Model(&AgentRebateRecord{}).Select("COALESCE(SUM(rebate_amount), 0)").Where("agent_user_id = ?", userId).Scan(&topupRebateAmount).Error; err != nil {
		return nil, err
	}
	var redemptionRebateAmount int64
	if err := DB.Model(&AgentRedemptionRebateRecord{}).Select("COALESCE(SUM(rebate_amount), 0)").Where("agent_user_id = ?", userId).Scan(&redemptionRebateAmount).Error; err != nil {
		return nil, err
	}
	summary.RecentRebateAmount = topupRebateAmount + redemptionRebateAmount
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

func GetAgentDailyMetrics(agentUserId int, startDate string, endDate string) (*AgentDailyMetricsResult, error) {
	startTs, endTs, ok := parseAgentDateRange(startDate, endDate)
	if !ok {
		end := time.Now()
		start := end.AddDate(0, 0, -6)
		startDate = start.Format("2006-01-02")
		endDate = end.Format("2006-01-02")
		startTs, endTs, _ = parseAgentDateRange(startDate, endDate)
	} else {
		startDate = time.Unix(startTs, 0).Local().Format("2006-01-02")
		endDate = time.Unix(endTs-1, 0).Local().Format("2006-01-02")
	}
	metrics := make([]*AgentDailyMetric, 0)
	for dayStart := startTs; dayStart < endTs; dayStart += int64(24 * time.Hour / time.Second) {
		dayEnd := dayStart + int64(24*time.Hour/time.Second)
		day := time.Unix(dayStart, 0).Format("2006-01-02")
		rows, err := getAgentDailyMetricRows(agentUserId, day, dayStart, dayEnd)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, rows...)
	}
	result := &AgentDailyMetricsResult{
		StartDate: startDate,
		EndDate:   endDate,
		Items:     metrics,
	}
	for _, item := range metrics {
		result.Summary.NewUserCount += item.NewUserCount
		result.Summary.TopupCount += item.TopupCount
		result.Summary.TopupAmount += item.TopupAmount
		result.Summary.TotalRebateAmount += item.TotalRebateAmount
	}
	return result, nil
}

func getAgentDailyMetricRows(agentUserId int, day string, startTs int64, endTs int64) ([]*AgentDailyMetric, error) {
	type metricRow struct {
		AgentUserId            int
		Username               string
		DisplayName            string
		NewUserCount           int64
		TopupCount             int64
		TopupAmount            int64
		TopupRebateAmount      int64
		RedemptionRebateAmount int64
	}
	rows := make([]*metricRow, 0)
	tx := DB.Table("agent_profiles AS ap").
		Select(`ap.user_id AS agent_user_id,
u.username,
u.display_name,
COALESCE(new_user_stats.new_user_count, 0) AS new_user_count,
COALESCE(topup_stats.topup_count, 0) AS topup_count,
COALESCE(topup_stats.topup_amount, 0) AS topup_amount,
COALESCE(topup_stats.topup_rebate_amount, 0) AS topup_rebate_amount,
COALESCE(redemption_stats.redemption_rebate_amount, 0) AS redemption_rebate_amount`).
		Joins("LEFT JOIN users AS u ON u.id = ap.user_id").
		Joins(`LEFT JOIN (
			SELECT inviter_id AS agent_user_id, COUNT(*) AS new_user_count
			FROM users
			WHERE inviter_id > 0 AND deleted_at IS NULL AND created_at >= ? AND created_at < ?
			GROUP BY inviter_id
		) AS new_user_stats ON new_user_stats.agent_user_id = ap.user_id`, startTs, endTs).
		Joins(`LEFT JOIN (
			SELECT agent_user_id, COUNT(id) AS topup_count, COALESCE(SUM(pay_amount), 0) AS topup_amount, COALESCE(SUM(rebate_amount), 0) AS topup_rebate_amount
			FROM agent_rebate_records
			WHERE status = ? AND settled_at >= ? AND settled_at < ?
			GROUP BY agent_user_id
		) AS topup_stats ON topup_stats.agent_user_id = ap.user_id`, AgentRebateRecordSettled, startTs, endTs).
		Joins(`LEFT JOIN (
			SELECT agent_user_id, COALESCE(SUM(rebate_amount), 0) AS redemption_rebate_amount
			FROM agent_redemption_rebate_records
			WHERE status = ? AND settled_at >= ? AND settled_at < ?
			GROUP BY agent_user_id
		) AS redemption_stats ON redemption_stats.agent_user_id = ap.user_id`, AgentRebateRecordSettled, startTs, endTs)
	if agentUserId > 0 {
		tx = tx.Where("ap.user_id = ?", agentUserId)
	}
	if err := tx.Order("ap.user_id asc").Scan(&rows).Error; err != nil {
		return nil, err
	}
	metrics := make([]*AgentDailyMetric, 0, len(rows))
	for _, row := range rows {
		totalRebate := row.TopupRebateAmount + row.RedemptionRebateAmount
		if row.NewUserCount == 0 && row.TopupCount == 0 && row.TopupAmount == 0 && totalRebate == 0 {
			continue
		}
		metrics = append(metrics, &AgentDailyMetric{
			Date:                   day,
			AgentUserId:            row.AgentUserId,
			Username:               row.Username,
			DisplayName:            row.DisplayName,
			NewUserCount:           row.NewUserCount,
			TopupCount:             row.TopupCount,
			TopupAmount:            row.TopupAmount,
			TopupRebateAmount:      row.TopupRebateAmount,
			RedemptionRebateAmount: row.RedemptionRebateAmount,
			TotalRebateAmount:      totalRebate,
		})
	}
	return metrics, nil
}

func parseAgentDateRange(startDate string, endDate string) (int64, int64, bool) {
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)
	if startDate == "" || endDate == "" {
		return 0, 0, false
	}
	startTime, err := time.ParseInLocation("2006-01-02", startDate, time.Local)
	if err != nil {
		return 0, 0, false
	}
	endTime, err := time.ParseInLocation("2006-01-02", endDate, time.Local)
	if err != nil || endTime.Before(startTime) {
		return 0, 0, false
	}
	if endTime.Sub(startTime) > 90*24*time.Hour {
		endTime = startTime.AddDate(0, 0, 90)
	}
	return startTime.Unix(), endTime.Add(24 * time.Hour).Unix(), true
}

func CreateAgentUpgradeRequest(sponsorAgentUserId int, targetUserId int, targetRate int, remark string) (*AgentUpgradeRequest, error) {
	if sponsorAgentUserId <= 0 || targetUserId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if sponsorAgentUserId == targetUserId {
		return nil, errors.New("target user cannot be self")
	}
	if err := validateAgentRebateRate(targetRate); err != nil {
		return nil, err
	}
	request := &AgentUpgradeRequest{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var targetUser User
		if err := lockForUpdate(tx).Select("id", "inviter_id").First(&targetUser, targetUserId).Error; err != nil {
			return err
		}
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
		var targetUser User
		if err := lockForUpdate(tx).Select("id", "inviter_id").First(&targetUser, targetUserId).Error; err != nil {
			return err
		}
		if err := lockAgentHierarchyTx(tx); err != nil {
			return err
		}
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
		if err := lockForUpdate(tx).First(request, requestId).Error; err != nil {
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
		var targetUser User
		if err := lockForUpdate(tx).Select("id", "inviter_id").First(&targetUser, request.TargetUserId).Error; err != nil {
			return err
		}
		if targetUser.InviterId != request.SponsorAgentUserId {
			return errors.New("target user is no longer a direct downline")
		}
		if err := lockAgentHierarchyTx(tx); err != nil {
			return err
		}
		if _, err := getAgentProfileByUserIdTx(tx, request.TargetUserId); err == nil {
			return errors.New("target user is already an agent")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
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
		if err := validateAgentRebateRate(targetRate); err != nil {
			return err
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

func TransferAgentDownlineUser(operatorUserId int, sourceAgentUserId int, targetAgentUserId int, downlineUserId int, promoLinkId int, remark string) error {
	RefreshAgentRuntimeOptions()
	if operatorUserId <= 0 || sourceAgentUserId <= 0 || targetAgentUserId <= 0 || downlineUserId <= 0 {
		return errors.New("invalid user id")
	}
	if !common.AgentEnabled || !common.AgentInitialized {
		return errors.New("agent module is not initialized")
	}
	if sourceAgentUserId == targetAgentUserId {
		return errors.New("target agent cannot equal source agent")
	}
	if downlineUserId == sourceAgentUserId || downlineUserId == targetAgentUserId {
		return errors.New("downline user cannot equal agent user")
	}
	remark = strings.TrimSpace(remark)
	targetPromoLinkId := promoLinkId
	err := DB.Transaction(func(tx *gorm.DB) error {
		var downline User
		if err := lockForUpdate(tx).Select("id", "inviter_id", "promo_link_id").First(&downline, downlineUserId).Error; err != nil {
			return err
		}
		if err := lockAgentHierarchyTx(tx); err != nil {
			return err
		}
		sourceProfile, err := getAgentProfileByUserIdTx(tx, sourceAgentUserId)
		if err != nil {
			return err
		}
		if sourceProfile.Status != AgentStatusEnabled {
			return errors.New("source agent is disabled")
		}
		targetProfile, err := getAgentProfileByUserIdTx(tx, targetAgentUserId)
		if err != nil {
			return err
		}
		if targetProfile.Status != AgentStatusEnabled {
			return errors.New("target agent is disabled")
		}
		if downline.InviterId != sourceAgentUserId {
			return errors.New("downline user does not belong to source agent")
		}
		if _, err := getAgentProfileByUserIdTx(tx, downlineUserId); err == nil {
			return errors.New("downline user is an agent")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		linkId, err := resolveTransferPromoLinkIdTx(tx, targetAgentUserId, promoLinkId)
		if err != nil {
			return err
		}
		targetPromoLinkId = linkId
		result := tx.Model(&User{}).Where("id = ? AND inviter_id = ?", downlineUserId, sourceAgentUserId).Updates(map[string]interface{}{
			"inviter_id":    targetAgentUserId,
			"promo_link_id": targetPromoLinkId,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("downline user ownership changed, please refresh and retry")
		}
		return nil
	})
	if err != nil {
		return err
	}
	RecordLog(operatorUserId, LogTypeManage, fmt.Sprintf("transfer agent downline user, source agent user ID: %d, target agent user ID: %d, downline user ID: %d, promo link ID: %d, remark: %s", sourceAgentUserId, targetAgentUserId, downlineUserId, targetPromoLinkId, remark))
	return nil
}

func AssignAgentDownlineUser(operatorUserId int, targetAgentUserId int, downlineUserId int, promoLinkId int, remark string) error {
	RefreshAgentRuntimeOptions()
	if operatorUserId <= 0 || targetAgentUserId <= 0 || downlineUserId <= 0 {
		return errors.New("invalid user id")
	}
	if !common.AgentEnabled || !common.AgentInitialized {
		return errors.New("agent module is not initialized")
	}
	if downlineUserId == targetAgentUserId {
		return errors.New("downline user cannot equal agent user")
	}
	remark = strings.TrimSpace(remark)
	targetPromoLinkId := promoLinkId
	err := DB.Transaction(func(tx *gorm.DB) error {
		var downline User
		if err := lockForUpdate(tx).Select("id", "role", "inviter_id", "promo_link_id").First(&downline, downlineUserId).Error; err != nil {
			return err
		}
		if err := lockAgentHierarchyTx(tx); err != nil {
			return err
		}
		targetProfile, err := getAgentProfileByUserIdTx(tx, targetAgentUserId)
		if err != nil {
			return err
		}
		if targetProfile.Status != AgentStatusEnabled {
			return errors.New("target agent is disabled")
		}
		if downline.Role != common.RoleCommonUser {
			return errors.New("only common users can be assigned as agent downlines")
		}
		if downline.InviterId > 0 {
			return errors.New("user already belongs to an inviter")
		}
		if _, err := getAgentProfileByUserIdTx(tx, downlineUserId); err == nil {
			return errors.New("downline user is an agent")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		linkId, err := resolveTransferPromoLinkIdTx(tx, targetAgentUserId, promoLinkId)
		if err != nil {
			return err
		}
		targetPromoLinkId = linkId
		result := tx.Model(&User{}).Where("id = ? AND inviter_id = ?", downlineUserId, 0).Updates(map[string]interface{}{
			"inviter_id":    targetAgentUserId,
			"promo_link_id": targetPromoLinkId,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("downline user ownership changed, please refresh and retry")
		}
		return nil
	})
	if err != nil {
		return err
	}
	RecordLog(operatorUserId, LogTypeManage, fmt.Sprintf("assign agent downline user, target agent user ID: %d, downline user ID: %d, promo link ID: %d, remark: %s", targetAgentUserId, downlineUserId, targetPromoLinkId, remark))
	return nil
}

func ChangeAgentDownlineUser(operatorUserId int, targetAgentUserId int, downlineUserId int, promoLinkId int, remark string) error {
	RefreshAgentRuntimeOptions()
	if operatorUserId <= 0 || targetAgentUserId <= 0 || downlineUserId <= 0 {
		return errors.New("invalid user id")
	}
	if !common.AgentEnabled || !common.AgentInitialized {
		return errors.New("agent module is not initialized")
	}
	if downlineUserId == targetAgentUserId {
		return errors.New("downline user cannot equal agent user")
	}
	remark = strings.TrimSpace(remark)
	oldAgentUserId := 0
	targetPromoLinkId := promoLinkId
	err := DB.Transaction(func(tx *gorm.DB) error {
		var downline User
		if err := lockForUpdate(tx).Select("id", "role", "inviter_id", "promo_link_id").First(&downline, downlineUserId).Error; err != nil {
			return err
		}
		if err := lockAgentHierarchyTx(tx); err != nil {
			return err
		}
		targetProfile, err := getAgentProfileByUserIdTx(tx, targetAgentUserId)
		if err != nil {
			return err
		}
		if targetProfile.Status != AgentStatusEnabled {
			return errors.New("target agent is disabled")
		}
		childProfile, err := getAgentProfileByUserIdTx(tx, downlineUserId)
		if err == nil {
			if childProfile.Status != AgentStatusEnabled {
				return errors.New("child agent is disabled")
			}
			createsCycle, err := agentRelationshipCreatesCycleTx(tx, targetAgentUserId, downlineUserId)
			if err != nil {
				return err
			}
			if createsCycle {
				return errors.New("agent relationship would create a cycle")
			}
			childRate, err := getEffectiveAgentRebateRateTx(tx, childProfile)
			if err != nil {
				return err
			}
			targetRate, err := getEffectiveAgentRebateRateTx(tx, targetProfile)
			if err != nil {
				return err
			}
			if childRate > targetRate {
				childName, _ := getUserBasicInfoTx(tx, downlineUserId)
				targetName, _ := getUserBasicInfoTx(tx, targetAgentUserId)
				return buildAgentRateConflictError([]AgentRateConflict{{
					AgentUserId:       downlineUserId,
					AgentUsername:     childName,
					ParentAgentUserId: targetAgentUserId,
					ParentAgentName:   targetName,
					AgentRate:         childRate,
					ParentAllowedRate: targetRate,
					ConflictType:      "child_exceeds_parent",
				}})
			}
			var relation AgentRelationship
			err = lockForUpdate(tx).Where("child_agent_user_id = ?", downlineUserId).First(&relation).Error
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				oldAgentUserId = downline.InviterId
				relation = AgentRelationship{
					ParentAgentUserId: targetAgentUserId,
					ChildAgentUserId:  downlineUserId,
					Status:            AgentStatusEnabled,
				}
				if err := tx.Create(&relation).Error; err != nil {
					return err
				}
			} else {
				if relation.ParentAgentUserId == targetAgentUserId && relation.Status == AgentStatusEnabled {
					return errors.New("target agent cannot equal current agent")
				}
				oldAgentUserId = relation.ParentAgentUserId
				relation.ParentAgentUserId = targetAgentUserId
				relation.Status = AgentStatusEnabled
				if err := tx.Save(&relation).Error; err != nil {
					return err
				}
			}
			linkId, err := resolveTransferPromoLinkIdTx(tx, targetAgentUserId, promoLinkId)
			if err != nil {
				return err
			}
			targetPromoLinkId = linkId
			update := tx.Model(&User{}).Where("id = ?", downlineUserId).Updates(map[string]interface{}{
				"inviter_id":    targetAgentUserId,
				"promo_link_id": targetPromoLinkId,
			})
			return update.Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if downline.Role != common.RoleCommonUser {
			return errors.New("only common users can be assigned as agent downlines")
		}
		if downline.InviterId == targetAgentUserId {
			return errors.New("target agent cannot equal current agent")
		}
		oldAgentUserId = downline.InviterId
		linkId, err := resolveTransferPromoLinkIdTx(tx, targetAgentUserId, promoLinkId)
		if err != nil {
			return err
		}
		update := tx.Model(&User{}).Where("id = ?", downlineUserId).Updates(map[string]interface{}{
			"inviter_id":    targetAgentUserId,
			"promo_link_id": linkId,
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 0 {
			return errors.New("downline user ownership changed, please refresh and retry")
		}
		return nil
	})
	if err != nil {
		return err
	}
	RecordLog(operatorUserId, LogTypeManage, fmt.Sprintf("change agent downline user, previous agent user ID: %d, target agent user ID: %d, downline user ID: %d, promo link ID: %d, remark: %s", oldAgentUserId, targetAgentUserId, downlineUserId, targetPromoLinkId, remark))
	return nil
}

func resolveTransferPromoLinkIdTx(tx *gorm.DB, targetAgentUserId int, promoLinkId int) (int, error) {
	if promoLinkId > 0 {
		var promoLink AgentPromoLink
		if err := tx.Where("id = ? AND agent_user_id = ? AND status = ?", promoLinkId, targetAgentUserId, AgentPromoLinkEnabled).First(&promoLink).Error; err != nil {
			return 0, err
		}
		return promoLink.Id, nil
	}
	var promoLink AgentPromoLink
	err := tx.Where("agent_user_id = ? AND status = ?", targetAgentUserId, AgentPromoLinkEnabled).Order("id asc").First(&promoLink).Error
	if err == nil {
		return promoLink.Id, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	code, err := GenerateUniqueAgentPromoCodeTx(tx)
	if err != nil {
		return 0, err
	}
	promoLink = AgentPromoLink{
		AgentUserId: targetAgentUserId,
		Name:        "default",
		Code:        code,
		Status:      AgentPromoLinkEnabled,
		LandingPage: "/",
		Remark:      "auto-created for transfer",
	}
	if err := tx.Create(&promoLink).Error; err != nil {
		return 0, err
	}
	return promoLink.Id, nil
}

func ensureAgentRelationshipTx(tx *gorm.DB, parentAgentUserId int, childAgentUserId int) error {
	if parentAgentUserId <= 0 || childAgentUserId <= 0 {
		return errors.New("invalid relationship users")
	}
	if parentAgentUserId == childAgentUserId {
		return errors.New("parent agent cannot equal child agent")
	}
	createsCycle, err := agentRelationshipCreatesCycleTx(tx, parentAgentUserId, childAgentUserId)
	if err != nil {
		return err
	}
	if createsCycle {
		return errors.New("agent relationship would create a cycle")
	}
	var relation AgentRelationship
	err = tx.Where("child_agent_user_id = ?", childAgentUserId).First(&relation).Error
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

func agentRelationshipCreatesCycleTx(tx *gorm.DB, parentAgentUserId int, childAgentUserId int) (bool, error) {
	current := parentAgentUserId
	visited := make(map[int]struct{})
	for current > 0 {
		if current == childAgentUserId {
			return true, nil
		}
		if _, exists := visited[current]; exists {
			return true, nil
		}
		visited[current] = struct{}{}

		var relation AgentRelationship
		err := tx.Select("parent_agent_user_id").
			Where("child_agent_user_id = ? AND status = ?", current, AgentStatusEnabled).
			First(&relation).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		current = relation.ParentAgentUserId
	}
	return false, nil
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
	err := lockForUpdate(tx).Where("user_id = ?", profile.UserId).First(&existing).Error
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
	stats := make([]*AgentPromoLinkStat, 0)
	tx := queryx.BuildAgentPromoLinkStatsQuery(DB, agentUserId, AgentRebateRecordSettled)
	err := tx.Order("apl.id desc").Scan(&stats).Error
	return stats, err
}

func GetAgentDownlineUsers(pageInfo *common.PageInfo, agentUserId int, keyword string) ([]*AgentDownlineUserView, int64, error) {
	users := make([]*AgentDownlineUserView, 0)
	tx := queryx.BuildAgentDownlineUsersQuery(DB, agentUserId, keyword, AgentStatusEnabled, AgentRebateRecordSettled)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Order("u.id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func GetAgentTransferDownlineUsers(pageInfo *common.PageInfo, agentUserId int, keyword string) ([]*AgentTransferDownlineUserView, int64, error) {
	if agentUserId <= 0 {
		return nil, 0, errors.New("agent_user_id is required")
	}
	rows, total, err := GetAgentDownlineUsers(pageInfo, agentUserId, keyword)
	if err != nil {
		return nil, 0, err
	}
	users := make([]*AgentTransferDownlineUserView, 0, len(rows))
	for _, row := range rows {
		users = append(users, &AgentTransferDownlineUserView{
			UserId:        row.UserId,
			Username:      row.Username,
			DisplayName:   row.DisplayName,
			InviterId:     row.InviterId,
			PromoLinkId:   row.PromoLinkId,
			PromoLinkName: row.PromoLinkName,
			IsAgent:       row.IsAgent,
		})
	}
	return users, total, nil
}
