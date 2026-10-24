package service

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// Harness ledger read-side limits agreed with the platform. Exceeding them is a
// client error, never a silently truncated answer: batchGet reports truncation
// explicitly, and the list endpoints reject an oversized page size.
const (
	HarnessUsageEventDefaultLimit = 50
	HarnessUsageEventMaxLimit     = 200
	HarnessBatchGetMaxRequestIds  = 100
	HarnessBatchGetMaxEvents      = 500
)

// HarnessReadError carries the contract error envelope fields for the read APIs
// so every handler answers with the same shape.
type HarnessReadError struct {
	StatusCode int
	Code       string
	Message    string
	Retryable  bool
	Details    map[string]any
}

func (e *HarnessReadError) Error() string { return e.Message }

func newHarnessReadError(status int, code, message string) *HarnessReadError {
	return &HarnessReadError{StatusCode: status, Code: code, Message: message}
}

// Contract error codes for the /internal read APIs.
const (
	HarnessErrCodeUnauthorized    = "GATEWAY_UNAUTHORIZED"
	HarnessErrCodeRejected        = "GATEWAY_REJECTED"
	HarnessErrCodeCursorInvalid   = "CURSOR_INVALID"
	HarnessErrCodeWindowTooLarge  = "WINDOW_TOO_LARGE"
	HarnessErrCodeRequestInvalid  = "REQUEST_INVALID"
	HarnessErrCodeRequestNotFound = "REQUEST_NOT_FOUND"
	HarnessErrCodeUpstream        = "GATEWAY_UPSTREAM"
	// HarnessErrCodeEventLimitExceeded reports one request whose own events
	// cannot fit in a single batchGet page, so retrying cannot complete it.
	HarnessErrCodeEventLimitExceeded = "EVENT_LIMIT_EXCEEDED"
)

// HarnessUsageEvent is the B1 event payload. It is the wire shape agreed with
// the platform: field names are fixed by the contract, not by our internal
// model.
type HarnessUsageEvent struct {
	EventId           string                   `json:"event_id"`
	EventType         string                   `json:"event_type"`
	RequestId         string                   `json:"request_id"`
	GatewayRequestId  string                   `json:"gateway_request_id"`
	RunId             string                   `json:"run_id"`
	UsageUnit         string                   `json:"usage_unit"`
	Usage             HarnessUsageEventAmounts `json:"usage"`
	UsageSource       string                   `json:"usage_source"`
	IsEstimated       bool                     `json:"is_estimated"`
	ObservedAt        string                   `json:"observed_at"`
	RecordVersion     int                      `json:"record_version"`
	SupersedesEventId *string                  `json:"supersedes_event_id"`
}

// HarnessUsageEventAmounts is the normalized usage block.
type HarnessUsageEventAmounts struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens"`
	CacheWriteTokens int `json:"cache_write_tokens"`
}

// HarnessCostBilling mirrors the existing settlement billing block so B3 reuses
// the same field names the platform already consumes.
type HarnessCostBilling struct {
	BaseAmount     string  `json:"base_amount"`
	GroupRatio     float64 `json:"group_ratio"`
	TaxRatePercent float64 `json:"tax_rate_percent"`
	TotalAmount    string  `json:"total_amount"`
}

// HarnessCostRecord is the B3 event payload.
type HarnessCostRecord struct {
	EventId                 string             `json:"event_id"`
	EventType               string             `json:"event_type"`
	CostId                  string             `json:"cost_id"`
	RequestId               string             `json:"request_id"`
	GatewayRequestId        string             `json:"gateway_request_id"`
	Status                  string             `json:"status"`
	Currency                string             `json:"currency"`
	Billing                 HarnessCostBilling `json:"billing"`
	BillingBasis            string             `json:"billing_basis"`
	ProcurementPriceVersion string             `json:"procurement_price_version"`
	RecordVersion           int                `json:"record_version"`
	SupersedesEventId       *string            `json:"supersedes_event_id"`
	ObservedAt              string             `json:"observed_at"`
}

// formatHarnessObservedAt renders the millisecond RFC3339 UTC timestamp the
// contract requires. time.RFC3339 alone drops the fractional part, so the layout
// is spelled out.
func formatHarnessObservedAt(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format("2006-01-02T15:04:05.000Z")
}

// costIdForEvent derives the cost record id of an event.
//
// The id is derived from gateway_request_id, not from event_id: one gateway
// request owns exactly one cost record, and a correction is a NEW event that
// supersedes the previous one. Deriving the id from the event would rename the
// record on every correction, so the platform could not recognize the versions
// as the same cost. A row without a gateway request id falls back to the event
// id derivation to keep the field non-empty.
func costIdForEvent(gatewayRequestId, eventId string) string {
	if gatewayRequestId = strings.TrimSpace(gatewayRequestId); gatewayRequestId != "" {
		return "cost_" + gatewayRequestId
	}
	trimmed, ok := strings.CutPrefix(eventId, model.HarnessLedgerStreamCost+"_")
	if !ok {
		return "cost_" + eventId
	}
	return "cost_" + trimmed
}

func usageEventFromLedgerEvent(e model.HarnessLedgerEvent) HarnessUsageEvent {
	return HarnessUsageEvent{
		EventId:          e.EventId,
		EventType:        e.EventType,
		RequestId:        e.HarnessRequestId,
		GatewayRequestId: e.GatewayRequestId,
		RunId:            e.RunId,
		UsageUnit:        e.UsageUnit,
		Usage: HarnessUsageEventAmounts{
			InputTokens:      e.PromptTokens,
			OutputTokens:     e.CompletionTokens,
			CacheReadTokens:  e.CacheReadTokens,
			CacheWriteTokens: e.CacheWriteTokens,
		},
		UsageSource:       e.UsageSource,
		IsEstimated:       e.IsEstimated,
		ObservedAt:        formatHarnessObservedAt(e.ObservedAtMs),
		RecordVersion:     e.RecordVersion,
		SupersedesEventId: e.SupersedesEventId,
	}
}

func costRecordFromLedgerEvent(e model.HarnessLedgerEvent) HarnessCostRecord {
	return HarnessCostRecord{
		EventId:          e.EventId,
		EventType:        e.EventType,
		CostId:           costIdForEvent(e.GatewayRequestId, e.EventId),
		RequestId:        e.HarnessRequestId,
		GatewayRequestId: e.GatewayRequestId,
		Status:           e.Status,
		Currency:         e.Currency,
		Billing: HarnessCostBilling{
			BaseAmount:     e.BaseAmount,
			GroupRatio:     e.GroupRatio,
			TaxRatePercent: e.TaxRatePercent,
			TotalAmount:    e.TotalAmount,
		},
		BillingBasis:            e.BillingBasis,
		ProcurementPriceVersion: e.PricingVersion,
		RecordVersion:           e.RecordVersion,
		SupersedesEventId:       e.SupersedesEventId,
		ObservedAt:              formatHarnessObservedAt(e.ObservedAtMs),
	}
}

// ParseHarnessLedgerCursor validates an inbound cursor against the expected
// stream so a caller cannot continue the usage feed with a cost cursor.
func ParseHarnessLedgerCursor(raw, wantStream string) (int64, *HarnessReadError) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	stream, id, err := model.DecodeHarnessLedgerCursor(raw)
	if err != nil {
		return 0, newHarnessReadError(http.StatusBadRequest, HarnessErrCodeCursorInvalid, "cursor is not valid")
	}
	if stream != wantStream {
		return 0, newHarnessReadError(http.StatusBadRequest, HarnessErrCodeCursorInvalid, "cursor belongs to another stream")
	}
	return id, nil
}

// ParseHarnessLedgerLimit applies the contract default and maximum. A limit
// above the maximum is clamped rather than rejected, because the platform may
// legitimately raise it later and clamping still returns a correct page.
func ParseHarnessLedgerLimit(raw string) (int, *HarnessReadError) {
	if strings.TrimSpace(raw) == "" {
		return HarnessUsageEventDefaultLimit, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, newHarnessReadError(http.StatusBadRequest, HarnessErrCodeRequestInvalid, "limit must be a positive integer")
	}
	return min(value, HarnessUsageEventMaxLimit), nil
}

// parseHarnessLedgerTimeBound parses an RFC3339 bound into epoch milliseconds.
// An empty bound is unbounded.
func parseHarnessLedgerTimeBound(raw, field string) (int64, *HarnessReadError) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		failure := newHarnessReadError(http.StatusBadRequest, HarnessErrCodeRequestInvalid, field+" must be an RFC3339 timestamp")
		failure.Details = map[string]any{"field": field}
		return 0, failure
	}
	return parsed.UTC().UnixMilli(), nil
}

// harnessLedgerWindowLimitMs caps the from/to span so one call cannot scan the
// whole ledger.
const harnessLedgerWindowLimitMs = int64(24 * time.Hour / time.Millisecond)

// ParseHarnessLedgerWindow parses and bounds the from/to pair.
func ParseHarnessLedgerWindow(fromRaw, toRaw string) (int64, int64, *HarnessReadError) {
	fromMs, failure := parseHarnessLedgerTimeBound(fromRaw, "from")
	if failure != nil {
		return 0, 0, failure
	}
	toMs, failure := parseHarnessLedgerTimeBound(toRaw, "to")
	if failure != nil {
		return 0, 0, failure
	}
	if fromMs > 0 && toMs > 0 {
		if toMs <= fromMs {
			return 0, 0, newHarnessReadError(http.StatusBadRequest, HarnessErrCodeRequestInvalid, "to must be later than from")
		}
		if toMs-fromMs > harnessLedgerWindowLimitMs {
			return 0, 0, newHarnessReadError(http.StatusBadRequest, HarnessErrCodeWindowTooLarge, "time window is too large")
		}
	}
	return fromMs, toMs, nil
}

// ListHarnessUsageEvents reads one page of the usage stream.
func ListHarnessUsageEvents(afterId int64, limit int, fromMs, toMs int64) ([]HarnessUsageEvent, string, bool, error) {
	events, hasMore, err := model.QueryHarnessLedgerEvents(model.HarnessLedgerStreamUsage, afterId, limit, fromMs, toMs)
	if err != nil {
		return nil, "", false, err
	}
	items := make([]HarnessUsageEvent, 0, len(events))
	for _, event := range events {
		items = append(items, usageEventFromLedgerEvent(event))
	}
	nextCursor := ""
	if hasMore && len(events) > 0 {
		nextCursor = model.EncodeHarnessLedgerCursor(model.HarnessLedgerStreamUsage, events[len(events)-1].Id)
	}
	return items, nextCursor, hasMore, nil
}

// ListHarnessCostRecords reads one page of the cost stream.
func ListHarnessCostRecords(afterId int64, limit int, fromMs, toMs int64) ([]HarnessCostRecord, string, bool, error) {
	events, hasMore, err := model.QueryHarnessLedgerEvents(model.HarnessLedgerStreamCost, afterId, limit, fromMs, toMs)
	if err != nil {
		return nil, "", false, err
	}
	items := make([]HarnessCostRecord, 0, len(events))
	for _, event := range events {
		items = append(items, costRecordFromLedgerEvent(event))
	}
	nextCursor := ""
	if hasMore && len(events) > 0 {
		nextCursor = model.EncodeHarnessLedgerCursor(model.HarnessLedgerStreamCost, events[len(events)-1].Id)
	}
	return items, nextCursor, hasMore, nil
}

// ValidateHarnessBatchRequestIds enforces the per-call id budget and character
// set. Duplicates are removed but the original order is preserved so the
// response order follows the request.
func ValidateHarnessBatchRequestIds(ids []string) ([]string, *HarnessReadError) {
	if len(ids) == 0 {
		return nil, newHarnessReadError(http.StatusBadRequest, HarnessErrCodeRequestInvalid, "request_ids must not be empty")
	}
	if len(ids) > HarnessBatchGetMaxRequestIds {
		failure := newHarnessReadError(http.StatusBadRequest, HarnessErrCodeRequestInvalid,
			"request_ids exceeds the maximum of "+strconv.Itoa(HarnessBatchGetMaxRequestIds))
		failure.Details = map[string]any{"field": "request_ids", "max": HarnessBatchGetMaxRequestIds}
		return nil, failure
	}
	seen := make(map[string]struct{}, len(ids))
	ordered := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, newHarnessReadError(http.StatusBadRequest, HarnessErrCodeRequestInvalid, "request_ids must not contain an empty value")
		}
		if len(id) > 64 {
			failure := newHarnessReadError(http.StatusBadRequest, HarnessErrCodeRequestInvalid, "request id is too long")
			failure.Details = map[string]any{"field": "request_ids", "max_length": 64}
			return nil, failure
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		ordered = append(ordered, id)
	}
	return ordered, nil
}

// HarnessUsageBatchItem is one request's usage events in a batchGet answer.
type HarnessUsageBatchItem struct {
	RequestId string              `json:"request_id"`
	Found     bool                `json:"found"`
	Events    []HarnessUsageEvent `json:"events"`
}

// HarnessCostBatchItem is one request's cost records in a batchGet answer.
type HarnessCostBatchItem struct {
	RequestId string              `json:"request_id"`
	Found     bool                `json:"found"`
	Records   []HarnessCostRecord `json:"records"`
}

// HarnessUsageBatchOptions narrows a B1 batchGet answer.
//
// include_corrections defaults to true, so nil keeps the full answer and only an
// explicit false removes correction events. The option is variadic so a caller
// that wants the contract default passes nothing.
type HarnessUsageBatchOptions struct {
	IncludeCorrections *bool
}

// HarnessCostBatchOptions narrows a B3 batchGet answer.
//
// include_status_events defaults to true and can only remove the pure status
// transition: a refund and a cost correction change the amount the platform
// owes, so they are answered even when it is false.
type HarnessCostBatchOptions struct {
	IncludeStatusEvents *bool
}

// queryHarnessBatchEvents loads the batchGet rows of one stream, optionally
// excluding one event type.
//
// A filter must run in SQL rather than after the read, because the row limit
// would otherwise be consumed by events the caller asked not to see and the
// answer would silently drop rows the caller did want.
func queryHarnessBatchEvents(stream string, ids []string, limit int, excludeEventType string) ([]model.HarnessLedgerEvent, error) {
	if excludeEventType == "" {
		return model.QueryHarnessLedgerEventsByHarnessRequestIds(stream, ids, limit)
	}
	if stream == "" || len(ids) == 0 || limit <= 0 {
		return []model.HarnessLedgerEvent{}, nil
	}
	var events []model.HarnessLedgerEvent
	err := model.DB.Where("stream = ?", stream).
		Where("harness_request_id IN ?", ids).
		Where("event_type <> ?", excludeEventType).
		// Same ordering as QueryHarnessLedgerEventsByHarnessRequestIds; keep the
		// two in step so a filtered page and an unfiltered one agree.
		Order("observed_at_ms ASC, event_id ASC").
		Limit(limit).
		Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

// limitHarnessBatchEvents applies the per-call event cap and rejects a page that
// a retry cannot repair.
//
// A request whose own events exceed the cap can never be completed by asking
// again with the same ids, so answering it would look complete while dropping
// events. Requests that only exceed the cap in aggregate are not an error:
// splitting the ids returns every event, so truncation stays an explicit flag.
//
// A group that fills the kept page is only a candidate, since it may hold exactly
// the cap and merely sort ahead of the others. At most one group can do that in a
// page of cap rows, so re-reading just that id settles the question exactly
// instead of guessing.
func limitHarnessBatchEvents(stream string, ids []string, events []model.HarnessLedgerEvent, excludeEventType string) ([]model.HarnessLedgerEvent, bool, error) {
	if len(events) <= HarnessBatchGetMaxEvents {
		return events, false, nil
	}
	events = events[:HarnessBatchGetMaxEvents]
	counts := make(map[string]int, len(ids))
	for _, event := range events {
		counts[event.HarnessRequestId]++
	}
	for _, id := range ids {
		if counts[id] < HarnessBatchGetMaxEvents {
			continue
		}
		alone, err := queryHarnessBatchEvents(stream, []string{id}, HarnessBatchGetMaxEvents+1, excludeEventType)
		if err != nil {
			return nil, false, err
		}
		if len(alone) <= HarnessBatchGetMaxEvents {
			continue
		}
		failure := newHarnessReadError(http.StatusBadRequest, HarnessErrCodeEventLimitExceeded,
			"a single request has more events than one batchGet page can return")
		failure.Details = map[string]any{"request_id": id, "max_events": HarnessBatchGetMaxEvents}
		return nil, false, failure
	}
	return events, true, nil
}

// BatchGetHarnessUsageEvents answers usage events for up to
// HarnessBatchGetMaxRequestIds requests. Missing ids are reported per id and do
// not fail the call.
func BatchGetHarnessUsageEvents(ids []string, options ...HarnessUsageBatchOptions) ([]HarnessUsageBatchItem, []string, bool, error) {
	includeCorrections := true
	if len(options) > 0 && options[0].IncludeCorrections != nil {
		includeCorrections = *options[0].IncludeCorrections
	}
	excludeEventType := ""
	if !includeCorrections {
		excludeEventType = model.HarnessLedgerEventTypeUsageCorrection
	}

	events, err := queryHarnessBatchEvents(model.HarnessLedgerStreamUsage, ids, HarnessBatchGetMaxEvents+1, excludeEventType)
	if err != nil {
		return nil, nil, false, err
	}
	events, truncated, err := limitHarnessBatchEvents(model.HarnessLedgerStreamUsage, ids, events, excludeEventType)
	if err != nil {
		return nil, nil, false, err
	}

	grouped := make(map[string][]HarnessUsageEvent, len(ids))
	for _, event := range events {
		grouped[event.HarnessRequestId] = append(grouped[event.HarnessRequestId], usageEventFromLedgerEvent(event))
	}

	items := make([]HarnessUsageBatchItem, 0, len(ids))
	missing := make([]string, 0)
	for _, id := range ids {
		// With include_corrections=false a request whose events were all
		// corrections has nothing left to return, so it is reported as missing
		// rather than found with an empty list: from the caller's narrowed view
		// the ledger holds no usage for it, which is the same answer as for an
		// unknown id.
		eventsForId, found := grouped[id]
		if !found {
			missing = append(missing, id)
		}
		items = append(items, HarnessUsageBatchItem{RequestId: id, Found: found, Events: eventsForId})
	}
	return items, missing, truncated, nil
}

// BatchGetHarnessCostRecords answers cost records for up to
// HarnessBatchGetMaxRequestIds requests.
func BatchGetHarnessCostRecords(ids []string, options ...HarnessCostBatchOptions) ([]HarnessCostBatchItem, []string, bool, error) {
	includeStatusEvents := true
	if len(options) > 0 && options[0].IncludeStatusEvents != nil {
		includeStatusEvents = *options[0].IncludeStatusEvents
	}
	// Only the pure status transition is optional. A refund and a cost
	// correction are cost changes themselves, so they are returned even when
	// status events are excluded. A request left without records is reported
	// as missing, the same as for the usage stream.
	excludeEventType := ""
	if !includeStatusEvents {
		excludeEventType = model.HarnessLedgerEventTypeStatusChange
	}

	events, err := queryHarnessBatchEvents(model.HarnessLedgerStreamCost, ids, HarnessBatchGetMaxEvents+1, excludeEventType)
	if err != nil {
		return nil, nil, false, err
	}
	events, truncated, err := limitHarnessBatchEvents(model.HarnessLedgerStreamCost, ids, events, excludeEventType)
	if err != nil {
		return nil, nil, false, err
	}

	grouped := make(map[string][]HarnessCostRecord, len(ids))
	for _, event := range events {
		grouped[event.HarnessRequestId] = append(grouped[event.HarnessRequestId], costRecordFromLedgerEvent(event))
	}

	items := make([]HarnessCostBatchItem, 0, len(ids))
	missing := make([]string, 0)
	for _, id := range ids {
		recordsForId, found := grouped[id]
		if !found {
			missing = append(missing, id)
		}
		items = append(items, HarnessCostBatchItem{RequestId: id, Found: found, Records: recordsForId})
	}
	return items, missing, truncated, nil
}

// ErrHarnessLedgerUnavailable marks a database failure on the read path so the
// handler can map it to GATEWAY_UPSTREAM.
var ErrHarnessLedgerUnavailable = errors.New("harness ledger is unavailable")
