package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Harness settlement status values. A row starts as pending and only reaches a
// final state once the underlying request or async task settles its billing.
// pending means "not final yet" and is still a successful read; refunded means
// the request was rolled back to zero (or its remaining net) after charging.
const (
	HarnessSettlementStatusPending  = "pending"
	HarnessSettlementStatusSettled  = "settled"
	HarnessSettlementStatusRefunded = "refunded"
	HarnessSettlementStatusFailed   = "failed"
)

// ErrHarnessSettlementInvalid reports a ledger row that cannot be persisted
// without corrupting another request's entry.
var ErrHarnessSettlementInvalid = errors.New("invalid harness settlement")

// HarnessSettlement records the settled cost of one relayed request for the
// Harness procurement ledger. It is written once per request and updated when
// an async task reaches its final billing state. Money is stored as a decimal
// string so reconciliation never loses precision to float rounding.
type HarnessSettlement struct {
	Id               int     `json:"id" gorm:"primaryKey;autoIncrement"`
	RequestId        string  `json:"request_id" gorm:"type:varchar(64);uniqueIndex:idx_harness_settlement_request_id;default:''"`
	TaskId           string  `json:"task_id" gorm:"type:varchar(191);index:idx_harness_settlement_task_id;default:''"`
	TokenId          int     `json:"token_id" gorm:"index:idx_harness_settlement_token_id;default:0"`
	UserId           int     `json:"user_id" gorm:"index;default:0"`
	Status           string  `json:"status" gorm:"type:varchar(20);index;default:'pending'"`
	ModelName        string  `json:"model_name" gorm:"type:varchar(191);default:''"`
	ChannelId        int     `json:"channel_id" gorm:"default:0"`
	PricingGroup     string  `json:"pricing_group" gorm:"type:varchar(64);default:''"`
	GroupRatio       float64 `json:"group_ratio" gorm:"default:0"`
	TaxRatePercent   float64 `json:"tax_rate_percent" gorm:"default:0"`
	PromptTokens     int     `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int     `json:"completion_tokens" gorm:"default:0"`
	CacheReadTokens  int     `json:"cache_read_tokens" gorm:"default:0"`
	CacheWriteTokens int     `json:"cache_write_tokens" gorm:"default:0"`
	BaseAmount       string  `json:"base_amount" gorm:"type:varchar(64);default:''"`
	TotalAmount      string  `json:"total_amount" gorm:"type:varchar(64);default:''"`
	Currency         string  `json:"currency" gorm:"type:varchar(16);default:''"`
	// HarnessRequestId and HarnessRunId record the platform correlation ids on the
	// snapshot so an async task, which settles long after the request context is
	// gone, can still be attributed. An empty HarnessRequestId means the request
	// did not come from the platform, and such a request must not reach the
	// platform's ledger feed.
	HarnessRequestId string `json:"harness_request_id" gorm:"type:varchar(64);index:idx_harness_settlement_harness_request_id;default:''"`
	HarnessRunId     string `json:"harness_run_id" gorm:"type:varchar(64);default:''"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index;default:0"`
	SettledAt        int64  `json:"settled_at" gorm:"bigint;default:0"`
}

func (HarnessSettlement) TableName() string {
	return "harness_settlements"
}

// UpsertHarnessSettlement inserts or updates by request_id. Idempotent: calling
// it twice with the same non-empty request_id must not create a second row.
//
// request_id is the conflict target and only mutable columns are rewritten, so
// the row keeps the created_at recorded by its first insert. SettledAt is
// stored as provided; the caller stamps it when the request reaches a final
// billing state.
func UpsertHarnessSettlement(row *HarnessSettlement) error {
	return UpsertHarnessSettlementTx(DB, row)
}

// UpsertHarnessSettlementTx upserts one settlement row on the given connection,
// so the caller can commit it atomically with the ledger events it belongs to.
// Passing the transaction in is what makes a retry meaningful: a failure rolls
// back the events too, so the second attempt starts from a clean slate instead of
// colliding with already-inserted event ids.
func UpsertHarnessSettlementTx(tx *gorm.DB, row *HarnessSettlement) error {
	if row == nil {
		return ErrHarnessSettlementInvalid
	}
	if tx == nil {
		return ErrHarnessSettlementInvalid
	}
	if row.RequestId == "" {
		// The unique request_id index admits exactly one empty value across the
		// whole table, so an empty id would silently overwrite another ledger
		// entry instead of identifying this request.
		return ErrHarnessSettlementInvalid
	}
	if row.CreatedAt == 0 {
		// created_at carries an explicit default:0 tag, which disables GORM's
		// automatic timestamp so a zero value would stay zero forever.
		row.CreatedAt = common.GetTimestamp()
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "request_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"task_id":            gorm.Expr("excluded.task_id"),
			"token_id":           gorm.Expr("excluded.token_id"),
			"user_id":            gorm.Expr("excluded.user_id"),
			"status":             gorm.Expr("excluded.status"),
			"model_name":         gorm.Expr("excluded.model_name"),
			"channel_id":         gorm.Expr("excluded.channel_id"),
			"pricing_group":      gorm.Expr("excluded.pricing_group"),
			"group_ratio":        gorm.Expr("excluded.group_ratio"),
			"tax_rate_percent":   gorm.Expr("excluded.tax_rate_percent"),
			"prompt_tokens":      gorm.Expr("excluded.prompt_tokens"),
			"completion_tokens":  gorm.Expr("excluded.completion_tokens"),
			"cache_read_tokens":  gorm.Expr("excluded.cache_read_tokens"),
			"cache_write_tokens": gorm.Expr("excluded.cache_write_tokens"),
			"base_amount":        gorm.Expr("excluded.base_amount"),
			"total_amount":       gorm.Expr("excluded.total_amount"),
			"currency":           gorm.Expr("excluded.currency"),
			// The platform correlation is written once and then never cleared. A
			// re-settle can arrive with an empty correlation -- an async task settling
			// from a path that lost the request context, or a later refund -- and a
			// plain assignment would wipe the fact that this row belongs to the
			// platform ledger, silently detaching it from the feed it was reported on.
			// Keeping the first non-empty value is safe because a request id maps to
			// exactly one platform request for its whole life.
			"harness_request_id": gorm.Expr(
				"CASE WHEN excluded.harness_request_id <> '' THEN excluded.harness_request_id ELSE harness_request_id END"),
			"harness_run_id": gorm.Expr(
				"CASE WHEN excluded.harness_run_id <> '' THEN excluded.harness_run_id ELSE harness_run_id END"),
			"settled_at": gorm.Expr("excluded.settled_at"),
		}),
	}).Create(row).Error
}

// GetHarnessSettlementForToken loads one settlement scoped to the owning token.
// Returns (nil, nil) when no row matches so the caller can answer 404 without
// leaking whether another token's request exists.
func GetHarnessSettlementForToken(requestId string, tokenId int) (*HarnessSettlement, error) {
	if requestId == "" {
		return nil, nil
	}
	var settlement HarnessSettlement
	err := DB.Where("request_id = ? AND token_id = ?", requestId, tokenId).First(&settlement).Error
	exists, err := RecordExist(err)
	if err != nil || !exists {
		return nil, err
	}
	return &settlement, nil
}

// GetHarnessSettlementByTaskId finds the settlement created for a task. When a
// task somehow owns several rows the earliest one wins, so polling callers
// always update the row created at submit time.
func GetHarnessSettlementByTaskId(taskId string) (*HarnessSettlement, error) {
	if taskId == "" {
		return nil, nil
	}
	var settlement HarnessSettlement
	err := DB.Where("task_id = ?", taskId).First(&settlement).Error
	exists, err := RecordExist(err)
	if err != nil || !exists {
		return nil, err
	}
	return &settlement, nil
}
