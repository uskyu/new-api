package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"

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
	CreatedAt        int64   `json:"created_at" gorm:"bigint;index;default:0"`
	SettledAt        int64   `json:"settled_at" gorm:"bigint;default:0"`
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
	if row == nil {
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
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "request_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"task_id",
			"token_id",
			"user_id",
			"status",
			"model_name",
			"channel_id",
			"pricing_group",
			"group_ratio",
			"tax_rate_percent",
			"prompt_tokens",
			"completion_tokens",
			"cache_read_tokens",
			"cache_write_tokens",
			"base_amount",
			"total_amount",
			"currency",
			"settled_at",
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
