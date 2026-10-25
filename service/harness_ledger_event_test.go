package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ledgerTestDB migrates the ledger tables into the shared test database. The
// shared service TestMain does not include them, so each ledger test sets them up
// itself and clears its own rows.
func ledgerTestDB(t *testing.T) {
	t.Helper()
	// The pricing lookup the settlement uses joins channels/abilities and reads the
	// model/vendor metadata, so those tables have to exist even for a test that
	// only asserts on the ledger rows.
	require.NoError(t, model.DB.AutoMigrate(
		&model.HarnessSettlement{},
		&model.HarnessLedgerEvent{},
		&model.Channel{},
		&model.Ability{},
		&model.Model{},
		&model.Vendor{},
		&model.Option{},
	))
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM harness_settlements")
		model.DB.Exec("DELETE FROM harness_ledger_events")
	})
}

// TestRecordHarnessSettlementAppendsBothStreams is the end-to-end guarantee for
// B1/B3: one settled request must leave exactly one usage event and one cost
// event, both carrying the platform's correlation ids.
func TestRecordHarnessSettlementAppendsBothStreams(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	settlement := &model.HarnessSettlement{
		RequestId:        "gw-request-1",
		TokenId:          7,
		UserId:           9,
		Status:           model.HarnessSettlementStatusSettled,
		ModelName:        "economy",
		ChannelId:        0,
		PromptTokens:     128,
		CompletionTokens: 512,
	}
	correlation := harnessSettlementCorrelation{
		HarnessRequestId: "hrq_01H8X",
		HarnessRunId:     "run_01H8Z",
		UsageUnit:        harnessUsageUnitToken,
		UsageSource:      harnessUsageSourceUpstream,
	}
	recordHarnessSettlement(settlement, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	var usageEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ?", model.HarnessLedgerStreamUsage).Find(&usageEvents).Error)
	require.Len(t, usageEvents, 1, "one usage event per settlement")
	assert.Equal(t, "hrq_01H8X", usageEvents[0].HarnessRequestId)
	assert.Equal(t, "run_01H8Z", usageEvents[0].RunId)
	assert.Equal(t, "gw-request-1", usageEvents[0].GatewayRequestId)
	assert.Equal(t, 128, usageEvents[0].PromptTokens)
	assert.Equal(t, 512, usageEvents[0].CompletionTokens)
	assert.Equal(t, harnessUsageSourceUpstream, usageEvents[0].UsageSource)
	assert.Equal(t, 1, usageEvents[0].RecordVersion)
	assert.Nil(t, usageEvents[0].SupersedesEventId)

	var costEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ?", model.HarnessLedgerStreamCost).Find(&costEvents).Error)
	require.Len(t, costEvents, 1, "one cost event per settlement")
	assert.Equal(t, "USD", costEvents[0].Currency)
	assert.Equal(t, "2", costEvents[0].BaseAmount)
	assert.Equal(t, "2", costEvents[0].TotalAmount)
	assert.Equal(t, model.HarnessLedgerStatusSettled, costEvents[0].Status)
	assert.Equal(t, "token", costEvents[0].BillingBasis)
}

// TestRecordHarnessSettlementRefundSupersedes covers the correction chain: a
// refund must append a new cost event that points at the settled one and carries
// the cumulative net amount of zero, never a delta.
func TestRecordHarnessSettlementRefundSupersedes(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	correlation := harnessSettlementCorrelation{HarnessRequestId: "hrq_refund", UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream}
	settled := &model.HarnessSettlement{
		RequestId: "gw-request-refund",
		Status:    model.HarnessSettlementStatusSettled,
		ModelName: "economy",
	}
	recordHarnessSettlement(settled, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	refunded := &model.HarnessSettlement{
		RequestId: "gw-request-refund",
		Status:    model.HarnessSettlementStatusRefunded,
		ModelName: "economy",
	}
	recordHarnessSettlement(refunded, harnessSettlementMoneyForQuota(nil, 0, 0, "economy"), correlation)

	var costEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ?", model.HarnessLedgerStreamCost).Order("id ASC").Find(&costEvents).Error)
	require.Len(t, costEvents, 2, "settled and refund are separate events")

	first, second := costEvents[0], costEvents[1]
	assert.Equal(t, 1, first.RecordVersion)
	assert.Nil(t, first.SupersedesEventId)

	assert.Equal(t, 2, second.RecordVersion, "a correction advances the record version")
	require.NotNil(t, second.SupersedesEventId)
	assert.Equal(t, first.EventId, *second.SupersedesEventId, "the refund must point at the event it supersedes")
	assert.Equal(t, model.HarnessLedgerEventTypeRefund, second.EventType)
	assert.Equal(t, "0", second.TotalAmount, "a refund records the cumulative net amount, not a delta")

	// The snapshot table keeps only the latest state, which is exactly why the
	// event stream exists.
	var snapshots []model.HarnessSettlement
	require.NoError(t, model.DB.Where("request_id = ?", "gw-request-refund").Find(&snapshots).Error)
	require.Len(t, snapshots, 1)
	assert.Equal(t, model.HarnessSettlementStatusRefunded, snapshots[0].Status)
}

// TestRecordHarnessSettlementFirstEventIsNotACorrection covers the request that
// reaches "refunded" as its very first ledger state: a task already terminal at
// submission, or a refund of a zero reservation.
//
// The contract defines usage_correction as a new event that points at the earlier
// one through supersedes_event_id, so a first event claiming that label would
// reference a predecessor that never existed. The amount semantics are the same;
// only the label must reflect that nothing is being corrected.
func TestRecordHarnessSettlementFirstEventIsNotACorrection(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	correlation := harnessSettlementCorrelation{HarnessRequestId: "hrq_first_refund", UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream}

	// No prior event for this request: this refund is its first appearance.
	refunded := &model.HarnessSettlement{
		RequestId: "gw-request-first-refund",
		Status:    model.HarnessSettlementStatusRefunded,
		ModelName: "economy",
	}
	recordHarnessSettlement(refunded, harnessSettlementMoneyForQuota(nil, 0, 0, "economy"), correlation)

	var usageEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ? AND gateway_request_id = ?",
		model.HarnessLedgerStreamUsage, "gw-request-first-refund").Find(&usageEvents).Error)
	require.Len(t, usageEvents, 1)

	first := usageEvents[0]
	assert.Nil(t, first.SupersedesEventId, "the first event has nothing to supersede")
	assert.Equal(t, 1, first.RecordVersion)
	assert.Equal(t, model.HarnessLedgerEventTypeUsage, first.EventType,
		"a first event must not claim to correct an earlier one")

	// The same status on a request that DOES have history stays a correction.
	settled := &model.HarnessSettlement{
		RequestId: "gw-request-later-refund",
		Status:    model.HarnessSettlementStatusSettled,
		ModelName: "economy",
	}
	recordHarnessSettlement(settled, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)
	laterRefund := &model.HarnessSettlement{
		RequestId: "gw-request-later-refund",
		Status:    model.HarnessSettlementStatusRefunded,
		ModelName: "economy",
	}
	recordHarnessSettlement(laterRefund, harnessSettlementMoneyForQuota(nil, 0, 0, "economy"), correlation)

	var laterEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ? AND gateway_request_id = ?",
		model.HarnessLedgerStreamUsage, "gw-request-later-refund").Order("id ASC").Find(&laterEvents).Error)
	require.Len(t, laterEvents, 2)
	assert.Equal(t, model.HarnessLedgerEventTypeUsageCorrection, laterEvents[1].EventType,
		"a refund with history is a correction and points at the earlier event")
	require.NotNil(t, laterEvents[1].SupersedesEventId)
	assert.Equal(t, laterEvents[0].EventId, *laterEvents[1].SupersedesEventId)
}

// TestHarnessLedgerReadEndToEnd reads the appended events back through the same
// service functions the HTTP handlers use, which is what the platform actually
// consumes.
func TestHarnessLedgerReadEndToEnd(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	settlement := &model.HarnessSettlement{
		RequestId:        "gw-request-e2e",
		Status:           model.HarnessSettlementStatusSettled,
		ModelName:        "economy",
		PromptTokens:     10,
		CompletionTokens: 20,
	}
	correlation := harnessSettlementCorrelation{HarnessRequestId: "hrq_e2e", HarnessRunId: "run_e2e", UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream}
	recordHarnessSettlement(settlement, harnessSettlementMoneyForQuota(nil, 500_000, 0, "economy"), correlation)

	// The cursor query applies a 2s visibility waterline: age the rows the way
	// time would, otherwise the feed legitimately reports nothing yet.
	require.NoError(t, model.DB.Model(&model.HarnessLedgerEvent{}).
		Where("harness_request_id = ?", "hrq_e2e").
		Update("observed_at_ms", 1).Error)

	items, nextCursor, hasMore, err := ListHarnessUsageEvents(0, 50, 0, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "hrq_e2e", items[0].RequestId)
	assert.Equal(t, "gw-request-e2e", items[0].GatewayRequestId)
	assert.Equal(t, "run_e2e", items[0].RunId)
	// The counts are published as optional values so "not measured" stays
	// distinguishable from "measured zero".
	require.NotNil(t, items[0].Usage.InputTokens)
	require.NotNil(t, items[0].Usage.OutputTokens)
	assert.Equal(t, 10, *items[0].Usage.InputTokens)
	assert.Equal(t, 20, *items[0].Usage.OutputTokens)
	assert.True(t, items[0].Usage.SplitKnown)
	assert.Equal(t, 1, items[0].RecordVersion)
	assert.NotEmpty(t, items[0].ObservedAt)
	assert.False(t, hasMore)
	assert.Empty(t, nextCursor)

	costs, _, _, err := ListHarnessCostRecords(0, 50, 0, 0)
	require.NoError(t, err)
	require.Len(t, costs, 1)
	assert.Equal(t, "USD", costs[0].Currency)
	assert.Equal(t, "1", costs[0].Billing.TotalAmount)
	assert.Equal(t, "token", costs[0].BillingBasis)
	assert.NotEmpty(t, costs[0].CostId)

	// batchGet answers by Harness request id, which is what the platform has.
	batchItems, missing, truncated, err := BatchGetHarnessUsageEvents([]string{"hrq_e2e", "hrq_missing"})
	require.NoError(t, err)
	require.Len(t, batchItems, 2)
	assert.True(t, batchItems[0].Found)
	require.Len(t, batchItems[0].Events, 1)
	assert.False(t, batchItems[1].Found)
	assert.Equal(t, []string{"hrq_missing"}, missing)
	assert.False(t, truncated)
}
