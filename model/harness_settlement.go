package model

import (
	"errors"
	"strings"

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
	// TotalTokensKnown records whether the upstream reported a total.
	TotalTokens      int  `json:"total_tokens" gorm:"default:0"`
	TotalTokensKnown bool `json:"total_tokens_known" gorm:"default:false"`
	// SplitUnknown records that the prompt/completion split was NOT observed. It is
	// stored inverted so the "known split" case needs no explicit write; a
	// `default:true` flag would let GORM drop a genuine false on insert.
	SplitUnknown bool   `json:"split_unknown" gorm:"default:false"`
	BaseAmount   string `json:"base_amount" gorm:"type:varchar(64);default:''"`
	TotalAmount  string `json:"total_amount" gorm:"type:varchar(64);default:''"`
	Currency     string `json:"currency" gorm:"type:varchar(16);default:''"`
	// CostCalculationVersion identifies the algorithm that produced the amounts,
	// independent of any price-configuration version. Empty marks a legacy row
	// whose base_amount still carried the customer-side group ratio.
	//
	// json:"-" keeps it internal so the external settlement response schema does
	// not grow an audit-only field without agreement.
	CostCalculationVersion string `json:"-" gorm:"type:varchar(32);default:''"`
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
	// The updated columns are named rather than written as raw expressions so GORM
	// renders the new-row reference per dialect. A literal `excluded.col` is
	// PostgreSQL-only spelling; MySQL answers "Unknown column 'excluded.col'", so
	// writing the expressions by hand silently breaks every MySQL deployment.
	//
	// harness_request_id and harness_run_id are excluded from the column list
	// because they need a keep-first-non-empty rule instead of a plain overwrite.
	updates := clause.AssignmentColumns([]string{
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
		"total_tokens",
		"total_tokens_known",
		"split_unknown",
		"base_amount",
		"total_amount",
		"currency",
		"cost_calculation_version",
		"settled_at",
	})
	// The platform correlation is written once and then never cleared. A re-settle
	// can arrive with an empty correlation -- an async task settling from a path
	// that lost the request context, or a later refund -- and a plain assignment
	// would wipe the fact that this row belongs to the platform ledger, silently
	// detaching it from the feed it was reported on. Keeping the first non-empty
	// value is safe because a request id maps to exactly one platform request for
	// its whole life.
	newRow := "excluded"
	if tx.Dialector.Name() == "mysql" {
		newRow = "VALUES"
	}
	updates = append(updates,
		clause.Assignment{Column: clause.Column{Name: "harness_request_id"},
			Value: gorm.Expr(keepFirstNonEmpty("harness_request_id", newRow))},
		clause.Assignment{Column: clause.Column{Name: "harness_run_id"},
			Value: gorm.Expr(keepFirstNonEmpty("harness_run_id", newRow))},
	)
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "request_id"}},
		DoUpdates: updates,
	}).Create(row).Error
}

// keepFirstNonEmpty builds the dialect-appropriate expression that keeps an
// existing non-empty correlation id. newRow names the dialect's reference to the
// row proposed for insert: PostgreSQL spells it `excluded`, MySQL `VALUES`.
func keepFirstNonEmpty(column, newRow string) string {
	incoming := newRow + "." + column
	if newRow == "VALUES" {
		incoming = "VALUES(" + column + ")"
	}
	return "CASE WHEN " + incoming + " <> '' THEN " + incoming + " ELSE " + column + " END"
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

// HarnessSettlementListQuery filters the operator-facing settlement list. Every
// field is optional; a zero value means that filter is not applied. The list is
// a read-only operator view of what the gateway booked, so it lives outside the
// platform contract and carries no cursor or visibility rules.
type HarnessSettlementListQuery struct {
	// RequestId matches the gateway request id by prefix.
	RequestId string
	// HarnessRequestId matches the platform request id by prefix.
	HarnessRequestId string
	// ModelName matches the client-facing model name by prefix.
	ModelName string
	// Statuses restricts the result to these settlement states.
	Statuses []string
	// StartTime / EndTime bound created_at in unix seconds, inclusive start and
	// exclusive end. Zero means unbounded.
	StartTime int64
	EndTime   int64
	Page      int
	PageSize  int
}

// ListHarnessSettlements returns one page of settlement rows and the total count
// matching the filter, newest first.
//
// Prefix matching is used for the free-text fields because an operator typically
// pastes the leading part of an id from a log line; an exact match would require
// the whole value. The like patterns escape their own wildcards so a value
// containing % or _ cannot widen the search.
func ListHarnessSettlements(query HarnessSettlementListQuery) ([]HarnessSettlement, int64, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	build := func() *gorm.DB {
		tx := DB.Model(&HarnessSettlement{})
		if requestId := strings.TrimSpace(query.RequestId); requestId != "" {
			tx = tx.Where("request_id LIKE ?", escapeLikePrefix(requestId))
		}
		if harnessRequestId := strings.TrimSpace(query.HarnessRequestId); harnessRequestId != "" {
			tx = tx.Where("harness_request_id LIKE ?", escapeLikePrefix(harnessRequestId))
		}
		if modelName := strings.TrimSpace(query.ModelName); modelName != "" {
			tx = tx.Where("model_name LIKE ?", escapeLikePrefix(modelName))
		}
		if len(query.Statuses) > 0 {
			tx = tx.Where("status IN ?", query.Statuses)
		}
		if query.StartTime > 0 {
			tx = tx.Where("created_at >= ?", query.StartTime)
		}
		if query.EndTime > 0 {
			tx = tx.Where("created_at < ?", query.EndTime)
		}
		return tx
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []HarnessSettlement
	err := build().Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// escapeLikePrefix turns a user-supplied search term into a LIKE pattern that
// matches it as a prefix, with the term's own wildcards neutralized.
func escapeLikePrefix(term string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(term) + "%"
}
