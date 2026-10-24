package model

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Harness ledger streams. "ue" carries usage events (B1) and "ce" carries cost
// events (B3); each stream owns its own cursor and visibility waterline.
const (
	HarnessLedgerStreamUsage = "ue"
	HarnessLedgerStreamCost  = "ce"
)

// Harness ledger event types. A correction or refund is always a new event that
// supersedes the previous one; historical rows are never rewritten.
const (
	HarnessLedgerEventTypeUsage           = "usage"
	HarnessLedgerEventTypeUsageCorrection = "usage_correction"
	HarnessLedgerEventTypeCost            = "cost"
	HarnessLedgerEventTypeStatusChange    = "status_change"
	HarnessLedgerEventTypeRefund          = "refund"
	HarnessLedgerEventTypeCostCorrection  = "cost_correction"
)

// Harness ledger event status values. "adjusted" marks a correction row that
// supersedes an earlier amount without fully refunding it.
const (
	HarnessLedgerStatusPending  = "pending"
	HarnessLedgerStatusSettled  = "settled"
	HarnessLedgerStatusRefunded = "refunded"
	HarnessLedgerStatusAdjusted = "adjusted"
)

// harnessLedgerVisibilityLagMs is the delay before an appended event becomes
// readable. An event's auto-increment id is allocated before its transaction
// commits, so a reader that only follows ids can step over an id that is still
// uncommitted and lose it forever. Waiting until observed_at_ms is well in the
// past keeps the id cursor safe without coordination with writers.
const harnessLedgerVisibilityLagMs = int64(2000)

var (
	// ErrHarnessLedgerInvalid reports an event that cannot be appended as-is.
	ErrHarnessLedgerInvalid = errors.New("invalid harness ledger event")
	// ErrHarnessLedgerCursorInvalid reports a malformed incremental cursor; the
	// API layer maps it to CURSOR_INVALID.
	ErrHarnessLedgerCursorInvalid = errors.New("invalid harness ledger cursor")
)

// HarnessLedgerEvent is one append-only row of the Harness procurement event
// log. It backs both the usage stream and the cost stream, so a request can be
// read incrementally with a single id cursor. Money stays a decimal string to
// keep reconciliation exact.
type HarnessLedgerEvent struct {
	Id                int64   `json:"id" gorm:"primaryKey;autoIncrement;index:idx_harness_ledger_stream_id,priority:2;index:idx_harness_ledger_batch,priority:3"`
	Stream            string  `json:"stream" gorm:"type:varchar(8);index:idx_harness_ledger_stream_id,priority:1;default:''"`
	EventId           string  `json:"event_id" gorm:"type:varchar(64);uniqueIndex:idx_harness_ledger_event_id;default:''"`
	EventType         string  `json:"event_type" gorm:"type:varchar(32);default:''"`
	HarnessRequestId  string  `json:"harness_request_id" gorm:"type:varchar(64);index:idx_harness_ledger_harness_request_id;index:idx_harness_ledger_batch,priority:1;default:''"`
	GatewayRequestId  string  `json:"gateway_request_id" gorm:"type:varchar(64);index:idx_harness_ledger_gateway_request_id;default:''"`
	RunId             string  `json:"run_id" gorm:"type:varchar(64);default:''"`
	Status            string  `json:"status" gorm:"type:varchar(20);default:''"`
	Currency          string  `json:"currency" gorm:"type:varchar(16);default:''"`
	UsageUnit         string  `json:"usage_unit" gorm:"type:varchar(24);default:''"`
	PromptTokens      int     `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens  int     `json:"completion_tokens" gorm:"default:0"`
	CacheReadTokens   int     `json:"cache_read_tokens" gorm:"default:0"`
	CacheWriteTokens  int     `json:"cache_write_tokens" gorm:"default:0"`
	UsageSource       string  `json:"usage_source" gorm:"type:varchar(16);default:''"`
	IsEstimated       bool    `json:"is_estimated" gorm:"default:false"`
	BaseAmount        string  `json:"base_amount" gorm:"type:varchar(64);default:''"`
	TotalAmount       string  `json:"total_amount" gorm:"type:varchar(64);default:''"`
	GroupRatio        float64 `json:"group_ratio" gorm:"default:0"`
	TaxRatePercent    float64 `json:"tax_rate_percent" gorm:"default:0"`
	BillingBasis      string  `json:"billing_basis" gorm:"type:varchar(24);default:''"`
	PricingVersion    string  `json:"pricing_version" gorm:"type:varchar(64);default:''"`
	RecordVersion     int     `json:"record_version" gorm:"default:1"`
	SupersedesEventId *string `json:"supersedes_event_id" gorm:"type:varchar(64)"`
	ObservedAtMs      int64   `json:"observed_at_ms" gorm:"bigint;index:idx_harness_ledger_observed_at_ms;index:idx_harness_ledger_batch,priority:2;default:0"`
	CreatedAt         int64   `json:"created_at" gorm:"bigint;default:0"`
}

func (HarnessLedgerEvent) TableName() string {
	return "harness_ledger_events"
}

// harnessEventIdAlphabet is Crockford Base32 in ASCII-ascending order, which is
// what makes the encoded id sort lexicographically in timestamp order.
const harnessEventIdAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// harnessEventIdLength is the ULID-compatible length: 10 characters of
// millisecond timestamp plus 16 characters of randomness.
const harnessEventIdLength = 26

// NewHarnessEventId returns "ue_"/"ce_" plus 26 Crockford Base32 characters.
// The first 10 characters encode the current millisecond in 48 of their 50
// bits, so ids sort lexicographically by time; the remaining 16 characters
// (80 bits) come from crypto/rand, which makes ids unique within one
// millisecond without process-wide state.
func NewHarnessEventId(stream string) string {
	var encoded [harnessEventIdLength]byte

	ms := time.Now().UnixMilli()
	for i := 9; i >= 0; i-- {
		encoded[i] = harnessEventIdAlphabet[ms&0x1f]
		ms >>= 5
	}

	var random [10]byte
	// crypto/rand never returns an error or a short read on supported
	// platforms; a failure is unrecoverable for id generation.
	_, _ = rand.Read(random[:])

	// 10 random bytes are 80 bits: exactly 16 base32 characters.
	var accumulator uint32
	var bits uint
	next := 10
	for _, b := range random {
		accumulator = accumulator<<8 | uint32(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			encoded[next] = harnessEventIdAlphabet[(accumulator>>bits)&0x1f]
			next++
		}
	}

	return stream + "_" + string(encoded[:])
}

// AppendHarnessLedgerEvent inserts one event. It only ever INSERTs: corrections
// and refunds are new rows that point at the row they supersede, so history is
// immutable and the id cursor never moves backwards.
//
// Missing identifiers are filled in for the caller: EventId from
// NewHarnessEventId, RecordVersion from 1, ObservedAtMs from the current
// millisecond, and CreatedAt from common.GetTimestamp.
func AppendHarnessLedgerEvent(e *HarnessLedgerEvent) error {
	if e == nil {
		return fmt.Errorf("%w: nil event", ErrHarnessLedgerInvalid)
	}
	switch e.Stream {
	case HarnessLedgerStreamUsage, HarnessLedgerStreamCost:
	default:
		return fmt.Errorf("%w: unknown stream %q", ErrHarnessLedgerInvalid, e.Stream)
	}
	if e.EventId == "" {
		e.EventId = NewHarnessEventId(e.Stream)
	}
	if e.RecordVersion == 0 {
		e.RecordVersion = 1
	}
	if e.ObservedAtMs == 0 {
		e.ObservedAtMs = time.Now().UnixMilli()
	}
	if e.CreatedAt == 0 {
		e.CreatedAt = common.GetTimestamp()
	}
	return DB.Create(e).Error
}

// harnessLedgerCursor is the wire form of an incremental cursor. The stream is
// part of the cursor so a caller cannot mix usage and cost positions.
type harnessLedgerCursor struct {
	Version int    `json:"v"`
	Stream  string `json:"s"`
	Id      int64  `json:"id"`
}

// EncodeHarnessLedgerCursor encodes base64url(JSON) for the given position.
func EncodeHarnessLedgerCursor(stream string, id int64) string {
	payload, err := common.Marshal(harnessLedgerCursor{Version: 1, Stream: stream, Id: id})
	if err != nil {
		// The struct only holds an int, a string and an int64, so marshalling
		// cannot fail; there is no meaningful cursor to return if it ever does.
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

// DecodeHarnessLedgerCursor parses a cursor produced by
// EncodeHarnessLedgerCursor. Anything else is rejected so the caller can answer
// CURSOR_INVALID instead of silently restarting the stream.
func DecodeHarnessLedgerCursor(raw string) (string, int64, error) {
	if raw == "" {
		return "", 0, fmt.Errorf("%w: empty cursor", ErrHarnessLedgerCursorInvalid)
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %v", ErrHarnessLedgerCursorInvalid, err)
	}
	var cursor harnessLedgerCursor
	if err := common.Unmarshal(payload, &cursor); err != nil {
		return "", 0, fmt.Errorf("%w: %v", ErrHarnessLedgerCursorInvalid, err)
	}
	if cursor.Version != 1 {
		return "", 0, fmt.Errorf("%w: unsupported version %d", ErrHarnessLedgerCursorInvalid, cursor.Version)
	}
	switch cursor.Stream {
	case HarnessLedgerStreamUsage, HarnessLedgerStreamCost:
	default:
		return "", 0, fmt.Errorf("%w: unknown stream %q", ErrHarnessLedgerCursorInvalid, cursor.Stream)
	}
	if cursor.Id < 0 {
		return "", 0, fmt.Errorf("%w: negative id %d", ErrHarnessLedgerCursorInvalid, cursor.Id)
	}
	return cursor.Stream, cursor.Id, nil
}

// QueryHarnessLedgerEvents pages one stream by ascending id. Only events older
// than harnessLedgerVisibilityLagMs are returned, so a concurrent writer's
// uncommitted id can never be skipped. fromMs/toMs of 0 mean unbounded.
// limit+1 rows are fetched so hasMore needs no second query.
func QueryHarnessLedgerEvents(stream string, afterId int64, limit int, fromMs, toMs int64) ([]HarnessLedgerEvent, bool, error) {
	if stream == "" || limit <= 0 {
		return nil, false, nil
	}
	query := DB.Where("stream = ?", stream)
	if afterId > 0 {
		query = query.Where("id > ?", afterId)
	}
	if fromMs > 0 {
		query = query.Where("observed_at_ms >= ?", fromMs)
	}
	if toMs > 0 {
		query = query.Where("observed_at_ms <= ?", toMs)
	}
	visibleBefore := time.Now().UnixMilli() - harnessLedgerVisibilityLagMs
	query = query.Where("observed_at_ms <= ?", visibleBefore)

	var events []HarnessLedgerEvent
	if err := query.Order("id ASC").Limit(limit + 1).Find(&events).Error; err != nil {
		return nil, false, err
	}
	if len(events) > limit {
		return events[:limit], true, nil
	}
	return events, false, nil
}

// QueryHarnessLedgerEventsByHarnessRequestIds loads the events of the given
// Harness request ids for batchGet, ordered by observation time and then by
// event id. limit caps the total number of rows.
//
// Unlike the cursor query this is deliberately NOT watermarked: batchGet exists
// to recheck a request right after its response was lost, so withholding a row
// that has already committed would answer "missing" for a request that just
// settled. Any event appended after the read still reaches the caller through
// the incremental stream, which is the authoritative feed; batchGet is a point
// lookup that complements it.
func QueryHarnessLedgerEventsByHarnessRequestIds(stream string, ids []string, limit int) ([]HarnessLedgerEvent, error) {
	if stream == "" || len(ids) == 0 || limit <= 0 {
		return []HarnessLedgerEvent{}, nil
	}
	var events []HarnessLedgerEvent
	err := DB.Where("stream = ?", stream).
		Where("harness_request_id IN ?", ids).
		Order("observed_at_ms ASC, event_id ASC").
		Limit(limit).
		Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}
