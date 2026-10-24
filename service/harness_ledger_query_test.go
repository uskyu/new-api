package service

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The batchGet read contract around the correction/status filters and the event
// cap. Each case seeds complete ledger rows directly and reads them back through
// the same service functions the HTTP handlers call, so no settlement writer is
// involved.

// appendHarnessQueryEvent inserts one complete ledger row.
func appendHarnessQueryEvent(t *testing.T, event *model.HarnessLedgerEvent) *model.HarnessLedgerEvent {
	t.Helper()
	require.NoError(t, model.DB.Create(event).Error)
	return event
}

// seedHarnessQueryEvents inserts count rows of one kind in ascending observation
// order starting at startMs.
func seedHarnessQueryEvents(t *testing.T, stream, eventType, harnessRequestId, gatewayRequestId string, count int, startMs int64) {
	t.Helper()
	events := make([]*model.HarnessLedgerEvent, 0, count)
	for i := range count {
		events = append(events, &model.HarnessLedgerEvent{
			Stream:           stream,
			EventId:          fmt.Sprintf("%s_%013d", stream, startMs+int64(i)),
			EventType:        eventType,
			HarnessRequestId: harnessRequestId,
			GatewayRequestId: gatewayRequestId,
			Status:           model.HarnessLedgerStatusSettled,
			ObservedAtMs:     startMs + int64(i),
		})
	}
	require.NoError(t, model.DB.CreateInBatches(events, 100).Error)
}

// TestHarnessBatchGetIncludeCorrections pins include_corrections: the field
// defaults to true, and false narrows the answer to plain usage events. A
// request left with no event by the filter is reported as missing rather than
// found with an empty list, because the narrowed ledger holds no usage for it.
func TestHarnessBatchGetIncludeCorrections(t *testing.T) {
	ledgerTestDB(t)

	base := int64(1_700_000_000_000)
	appendHarnessQueryEvent(t, &model.HarnessLedgerEvent{
		Stream:           model.HarnessLedgerStreamUsage,
		EventId:          "ue_corrections_usage",
		EventType:        model.HarnessLedgerEventTypeUsage,
		HarnessRequestId: "hrq_corrections",
		GatewayRequestId: "gw_corrections",
		Status:           model.HarnessLedgerStatusSettled,
		ObservedAtMs:     base,
	})
	appendHarnessQueryEvent(t, &model.HarnessLedgerEvent{
		Stream:           model.HarnessLedgerStreamUsage,
		EventId:          "ue_corrections_fix",
		EventType:        model.HarnessLedgerEventTypeUsageCorrection,
		HarnessRequestId: "hrq_corrections",
		GatewayRequestId: "gw_corrections",
		Status:           model.HarnessLedgerStatusAdjusted,
		ObservedAtMs:     base + 1,
	})
	appendHarnessQueryEvent(t, &model.HarnessLedgerEvent{
		Stream:           model.HarnessLedgerStreamUsage,
		EventId:          "ue_correction_only",
		EventType:        model.HarnessLedgerEventTypeUsageCorrection,
		HarnessRequestId: "hrq_correction_only",
		GatewayRequestId: "gw_correction_only",
		Status:           model.HarnessLedgerStatusAdjusted,
		ObservedAtMs:     base + 2,
	})

	ids := []string{"hrq_corrections", "hrq_correction_only"}

	// An absent field keeps the documented default of true.
	items, missing, truncated, err := BatchGetHarnessUsageEvents(ids)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.True(t, items[0].Found)
	assert.Len(t, items[0].Events, 2)
	assert.Equal(t, model.HarnessLedgerEventTypeUsageCorrection, items[0].Events[1].EventType)
	assert.True(t, items[1].Found)
	assert.Empty(t, missing)
	assert.False(t, truncated)

	includeCorrections := true
	items, _, _, err = BatchGetHarnessUsageEvents(ids, HarnessUsageBatchOptions{IncludeCorrections: &includeCorrections})
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Len(t, items[0].Events, 2)

	includeCorrections = false
	items, missing, truncated, err = BatchGetHarnessUsageEvents(ids, HarnessUsageBatchOptions{IncludeCorrections: &includeCorrections})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Len(t, items[0].Events, 1)
	assert.Equal(t, model.HarnessLedgerEventTypeUsage, items[0].Events[0].EventType,
		"only plain usage events survive include_corrections=false")
	assert.False(t, items[1].Found, "a request holding only corrections is missing once they are filtered out")
	assert.Equal(t, []string{"hrq_correction_only"}, missing)
	assert.False(t, truncated)
}

// TestHarnessBatchGetIncludeStatusEvents pins include_status_events: false hides
// only the pure status transition. A refund and a cost correction change the
// amount the platform owes, so both must still be answered.
func TestHarnessBatchGetIncludeStatusEvents(t *testing.T) {
	ledgerTestDB(t)

	base := int64(1_700_000_100_000)
	for i, eventType := range []string{
		model.HarnessLedgerEventTypeCost,
		model.HarnessLedgerEventTypeStatusChange,
		model.HarnessLedgerEventTypeRefund,
		model.HarnessLedgerEventTypeCostCorrection,
	} {
		appendHarnessQueryEvent(t, &model.HarnessLedgerEvent{
			Stream:           model.HarnessLedgerStreamCost,
			EventId:          fmt.Sprintf("ce_status_%d", i),
			EventType:        eventType,
			HarnessRequestId: "hrq_status",
			GatewayRequestId: "gw_status",
			Status:           model.HarnessLedgerStatusSettled,
			BaseAmount:       "1",
			TotalAmount:      "1",
			Currency:         "USD",
			ObservedAtMs:     base + int64(i),
		})
	}

	ids := []string{"hrq_status"}

	items, missing, _, err := BatchGetHarnessCostRecords(ids)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Len(t, items[0].Records, 4, "an absent field keeps every event")
	assert.Empty(t, missing)

	includeStatusEvents := false
	items, missing, truncated, err := BatchGetHarnessCostRecords(ids, HarnessCostBatchOptions{IncludeStatusEvents: &includeStatusEvents})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.True(t, items[0].Found)
	assert.Empty(t, missing)
	assert.False(t, truncated)

	gotTypes := make([]string, 0, len(items[0].Records))
	for _, record := range items[0].Records {
		gotTypes = append(gotTypes, record.EventType)
	}
	assert.Equal(t, []string{
		model.HarnessLedgerEventTypeCost,
		model.HarnessLedgerEventTypeRefund,
		model.HarnessLedgerEventTypeCostCorrection,
	}, gotTypes, "refund and cost_correction carry cost changes and must survive the status filter")
}

// TestHarnessCostIdIsStableAcrossCorrections pins the cost_id derivation: the id
// identifies one gateway request's cost record, so a correction is another
// version of the same record and keeps the same id.
func TestHarnessCostIdIsStableAcrossCorrections(t *testing.T) {
	ledgerTestDB(t)

	base := int64(1_700_000_200_000)
	appendHarnessQueryEvent(t, &model.HarnessLedgerEvent{
		Stream:           model.HarnessLedgerStreamCost,
		EventId:          "ce_costid_first",
		EventType:        model.HarnessLedgerEventTypeCost,
		HarnessRequestId: "hrq_costid",
		GatewayRequestId: "gw_costid",
		Status:           model.HarnessLedgerStatusSettled,
		ObservedAtMs:     base,
	})
	appendHarnessQueryEvent(t, &model.HarnessLedgerEvent{
		Stream:           model.HarnessLedgerStreamCost,
		EventId:          "ce_costid_fix",
		EventType:        model.HarnessLedgerEventTypeCostCorrection,
		HarnessRequestId: "hrq_costid",
		GatewayRequestId: "gw_costid",
		Status:           model.HarnessLedgerStatusAdjusted,
		ObservedAtMs:     base + 1,
	})

	items, _, _, err := BatchGetHarnessCostRecords([]string{"hrq_costid"})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].Records, 2)
	assert.Equal(t, "cost_gw_costid", items[0].Records[0].CostId)
	assert.Equal(t, items[0].Records[0].CostId, items[0].Records[1].CostId,
		"a correction must not rename the cost record")
	assert.NotEqual(t, items[0].Records[0].EventId, items[0].Records[1].EventId)

	// Without a gateway request id the fallback keeps the field non-empty.
	appendHarnessQueryEvent(t, &model.HarnessLedgerEvent{
		Stream:           model.HarnessLedgerStreamCost,
		EventId:          "ce_costid_orphan",
		EventType:        model.HarnessLedgerEventTypeCost,
		HarnessRequestId: "hrq_costid_orphan",
		Status:           model.HarnessLedgerStatusSettled,
		ObservedAtMs:     base + 2,
	})
	items, _, _, err = BatchGetHarnessCostRecords([]string{"hrq_costid_orphan"})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].Records, 1)
	assert.Equal(t, "cost_costid_orphan", items[0].Records[0].CostId)
}

// TestHarnessBatchGetRejectsSingleRequestOverLimit pins the one unrecoverable
// case: a request whose own events alone reach the page cap cannot be completed
// by retrying with the same ids, so the call fails instead of answering a
// partial page that looks complete.
func TestHarnessBatchGetRejectsSingleRequestOverLimit(t *testing.T) {
	ledgerTestDB(t)

	seedHarnessQueryEvents(t, model.HarnessLedgerStreamUsage, model.HarnessLedgerEventTypeUsage,
		"hrq_oversize", "gw_oversize", HarnessBatchGetMaxEvents+1, 1_700_000_300_000)

	items, missing, truncated, err := BatchGetHarnessUsageEvents([]string{"hrq_oversize"})
	require.Error(t, err)
	var failure *HarnessReadError
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, http.StatusBadRequest, failure.StatusCode)
	assert.Equal(t, HarnessErrCodeEventLimitExceeded, failure.Code)
	assert.Equal(t, "hrq_oversize", failure.Details["request_id"])
	assert.Equal(t, HarnessBatchGetMaxEvents, failure.Details["max_events"])
	assert.Nil(t, items)
	assert.Nil(t, missing)
	assert.False(t, truncated, "an error must not also claim a usable page")
}

// TestHarnessBatchGetTruncatesMultipleRequestsWithoutError keeps the aggregate
// case non-fatal: the cap is shared by several requests, so asking for the ids in
// smaller batches returns everything.
func TestHarnessBatchGetTruncatesMultipleRequestsWithoutError(t *testing.T) {
	ledgerTestDB(t)

	seedHarnessQueryEvents(t, model.HarnessLedgerStreamUsage, model.HarnessLedgerEventTypeUsage,
		"hrq_split_a", "gw_split_a", 300, 1_700_000_400_000)
	seedHarnessQueryEvents(t, model.HarnessLedgerStreamUsage, model.HarnessLedgerEventTypeUsage,
		"hrq_split_b", "gw_split_b", 300, 1_700_000_500_000)

	items, missing, truncated, err := BatchGetHarnessUsageEvents([]string{"hrq_split_a", "hrq_split_b"})
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Empty(t, missing)
	assert.True(t, truncated, "the shared cap still truncates the page")
	assert.Len(t, items[0].Events, 300)
	assert.Len(t, items[1].Events, 200)

	// Splitting the ids is the documented recovery and returns every event.
	for _, id := range []string{"hrq_split_a", "hrq_split_b"} {
		single, _, singleTruncated, singleErr := BatchGetHarnessUsageEvents([]string{id})
		require.NoError(t, singleErr)
		require.Len(t, single, 1)
		assert.Len(t, single[0].Events, 300)
		assert.False(t, singleTruncated)
	}
}

// TestHarnessBatchGetAcceptsRequestAtExactlyTheCap keeps the limit check exact: a
// request holding exactly the cap is complete, so it must not be reported as
// exceeding it even though its group fills the shared page.
func TestHarnessBatchGetAcceptsRequestAtExactlyTheCap(t *testing.T) {
	ledgerTestDB(t)

	seedHarnessQueryEvents(t, model.HarnessLedgerStreamUsage, model.HarnessLedgerEventTypeUsage,
		"hrq_exact_cap", "gw_exact_cap", HarnessBatchGetMaxEvents, 1_700_000_700_000)
	seedHarnessQueryEvents(t, model.HarnessLedgerStreamUsage, model.HarnessLedgerEventTypeUsage,
		"hrq_cap_spill", "gw_cap_spill", 3, 1_700_000_800_000)

	items, _, truncated, err := BatchGetHarnessUsageEvents([]string{"hrq_exact_cap", "hrq_cap_spill"})
	require.NoError(t, err, "a request at exactly the cap is complete, not over it")
	assert.Len(t, items[0].Events, HarnessBatchGetMaxEvents)
	assert.True(t, truncated, "the shared page is still truncated")
}

// TestHarnessBatchGetCostAppliesEventCapToCostStream makes sure the cost stream
// enforces the same single-request rule and the same filtered-total behaviour.
func TestHarnessBatchGetCostAppliesEventCapToCostStream(t *testing.T) {
	ledgerTestDB(t)

	seedHarnessQueryEvents(t, model.HarnessLedgerStreamCost, model.HarnessLedgerEventTypeCost,
		"hrq_cost_oversize", "gw_cost_oversize", HarnessBatchGetMaxEvents+1, 1_700_000_600_000)

	_, _, _, err := BatchGetHarnessCostRecords([]string{"hrq_cost_oversize"})
	var failure *HarnessReadError
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, HarnessErrCodeEventLimitExceeded, failure.Code)

	// Filtering status events cannot hide the condition: every row here is a
	// cost event, so the same request still exceeds the cap.
	includeStatusEvents := false
	_, _, _, err = BatchGetHarnessCostRecords([]string{"hrq_cost_oversize"},
		HarnessCostBatchOptions{IncludeStatusEvents: &includeStatusEvents})
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, HarnessErrCodeEventLimitExceeded, failure.Code)
	assert.Equal(t, "hrq_cost_oversize", failure.Details["request_id"])
}
