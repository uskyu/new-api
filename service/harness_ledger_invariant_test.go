package service

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedHarnessLedgerChannel creates a channel whose ability exposes the pricing
// group for the model and registers that group as a procurement group with the
// given ratio, which is what harnessSettlementPricing resolves through.
func seedHarnessLedgerChannel(t *testing.T, modelName, group string, groupRatio, supplierTaxPercent float64) int {
	t.Helper()

	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))

	originalRatioJSON := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatioJSON)) })
	ratios := ratio_setting.GetGroupRatioCopy()
	ratios[group] = groupRatio
	encoded, err := common.Marshal(ratios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(string(encoded)))

	originalRouting := setting.GetIntelligentRoutingConfig()
	t.Cleanup(func() {
		// The validator rejects a nil global channel order, and an unconfigured
		// instance starts that way, so restore an empty non-nil list instead.
		restored := originalRouting
		if restored.GlobalChannelOrder == nil {
			restored.GlobalChannelOrder = []int{}
		}
		require.NoError(t, setting.UpdateIntelligentRoutingConfig(restored))
	})

	channel := &model.Channel{
		Name:          "ledger-tax-channel",
		Type:          1,
		Status:        common.ChannelStatusEnabled,
		Models:        modelName,
		Group:         group,
		OtherSettings: `{"supplier_tax_rate_percent":` + formatTestFloat(supplierTaxPercent) + `}`,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: channel.Id,
		Enabled:   true,
	}).Error)
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM channels WHERE id = ?", channel.Id)
		model.DB.Exec("DELETE FROM abilities WHERE channel_id = ?", channel.Id)
	})

	// The routing config validator requires a non-nil global channel order, so the
	// fixture registers its own channel there rather than leaving the list empty.
	routing := originalRouting
	routing.GlobalChannelOrder = []int{channel.Id}
	routing.PricingGroupOrder = append(append([]string{}, originalRouting.PricingGroupOrder...), group)
	require.NoError(t, setting.UpdateIntelligentRoutingConfig(routing))
	return channel.Id
}

// formatTestFloat renders a float for the JSON settings literal without an
// exponent or a trailing zero, which keeps the fixture readable.
func formatTestFloat(value float64) string {
	encoded, err := common.Marshal(value)
	if err != nil {
		return "0"
	}
	return string(encoded)
}

// normalizedDecimal trims insignificant trailing zeros so a fixed-precision
// decimal string can be compared against a readable expectation.
func normalizedDecimal(t *testing.T, value string) string {
	t.Helper()
	trimmed := strings.TrimRight(value, "0")
	trimmed = strings.TrimSuffix(trimmed, ".")
	if trimmed == "" {
		return "0"
	}
	return trimmed
}

// The tests here pin the three ledger invariants that a plain "the API returns
// data" test cannot catch: whose requests may enter the feed, that usage and cost
// corrections stay in their own stream, and that the cost amount follows the
// contract sample.

// TestLedgerSkipsRequestsThatDidNotComeFromHarness is the tenant-isolation
// guarantee. A settlement row is written for every relayed request because the
// operator needs the cost, but only platform-originated requests may appear in
// the ledger feed; otherwise Harness could read internal sk- users' usage and
// procurement cost.
func TestLedgerSkipsRequestsThatDidNotComeFromHarness(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	// No correlation ids: this is an internal sk- caller.
	internalSettlement := &model.HarnessSettlement{
		RequestId:    "gw-internal-sk",
		Status:       model.HarnessSettlementStatusSettled,
		ModelName:    "economy",
		PromptTokens: 5,
	}
	recordHarnessSettlement(internalSettlement,
		harnessSettlementMoneyForQuota(nil, 500_000, 0, "economy"),
		harnessSettlementCorrelation{UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream})

	var leaked []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("gateway_request_id = ?", "gw-internal-sk").Find(&leaked).Error)
	assert.Empty(t, leaked, "an internal request must not enter the platform ledger feed")

	// The snapshot still records the cost for the operator.
	var snapshots []model.HarnessSettlement
	require.NoError(t, model.DB.Where("request_id = ?", "gw-internal-sk").Find(&snapshots).Error)
	require.Len(t, snapshots, 1, "the operator still needs the cost snapshot")
	assert.Empty(t, snapshots[0].HarnessRequestId)

	// A platform request with an equivalent shape still produces both events.
	platformSettlement := &model.HarnessSettlement{
		RequestId:    "gw-platform",
		Status:       model.HarnessSettlementStatusSettled,
		ModelName:    "economy",
		PromptTokens: 5,
	}
	recordHarnessSettlement(platformSettlement,
		harnessSettlementMoneyForQuota(nil, 500_000, 0, "economy"),
		harnessSettlementCorrelation{HarnessRequestId: "hrq_platform", HarnessRunId: "run_platform", UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream})

	var platformEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("gateway_request_id = ?", "gw-platform").Order("id ASC").Find(&platformEvents).Error)
	require.Len(t, platformEvents, 2, "a platform request emits one usage and one cost event")
}

// TestLedgerCorrectionChainsStayInTheirOwnStream guards the supersedes link: a
// usage correction must point at the previous usage event, never at a cost event,
// because the platform follows each stream independently.
func TestLedgerCorrectionChainsStayInTheirOwnStream(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	correlation := harnessSettlementCorrelation{
		HarnessRequestId: "hrq_chain",
		HarnessRunId:     "run_chain",
		UsageUnit:        harnessUsageUnitToken,
		UsageSource:      harnessUsageSourceUpstream,
	}

	settled := &model.HarnessSettlement{
		RequestId:        "gw-chain",
		Status:           model.HarnessSettlementStatusSettled,
		ModelName:        "economy",
		PromptTokens:     100,
		CompletionTokens: 20,
	}
	recordHarnessSettlement(settled, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	// A refund that also corrects the measured usage appends one event in each
	// stream. Both must move for that to happen: with per-stream change detection a
	// status-only transition appends to the cost stream alone.
	refunded := &model.HarnessSettlement{
		RequestId:        "gw-chain",
		Status:           model.HarnessSettlementStatusRefunded,
		ModelName:        "economy",
		PromptTokens:     100,
		CompletionTokens: 20,
		CacheReadTokens:  9,
	}
	recordHarnessSettlement(refunded, harnessSettlementMoneyForQuota(nil, 0, 0, "economy"), correlation)

	var usageEvents, costEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("gateway_request_id = ? AND stream = ?", "gw-chain", model.HarnessLedgerStreamUsage).Order("id ASC").Find(&usageEvents).Error)
	require.NoError(t, model.DB.Where("gateway_request_id = ? AND stream = ?", "gw-chain", model.HarnessLedgerStreamCost).Order("id ASC").Find(&costEvents).Error)
	require.Len(t, usageEvents, 2)
	require.Len(t, costEvents, 2)

	require.NotNil(t, usageEvents[1].SupersedesEventId)
	assert.Equal(t, usageEvents[0].EventId, *usageEvents[1].SupersedesEventId,
		"a usage correction must supersede the previous usage event")
	assert.Contains(t, *usageEvents[1].SupersedesEventId, model.HarnessLedgerStreamUsage+"_",
		"the superseded id must belong to the usage stream")
	assert.Equal(t, 2, usageEvents[1].RecordVersion)

	require.NotNil(t, costEvents[1].SupersedesEventId)
	assert.Equal(t, costEvents[0].EventId, *costEvents[1].SupersedesEventId,
		"a cost correction must supersede the previous cost event")
	assert.Contains(t, *costEvents[1].SupersedesEventId, model.HarnessLedgerStreamCost+"_",
		"the superseded id must belong to the cost stream")
	assert.Equal(t, 2, costEvents[1].RecordVersion)

	// The two chains advance independently: both settle at version 2 here, and a
	// third cost event must not be pushed to version 3 by a usage-only change.
	assert.Equal(t, usageEvents[1].RecordVersion, costEvents[1].RecordVersion)
}

// TestLedgerTaskUsesFrozenProcurementPricing pins historical stability: an async
// task settles hours after submission, so a procurement ratio or tax-rate edit
// made in between must not rewrite the cost that was accepted at submit time.
// The frozen context is the only source the settlement path reads.
func TestLedgerTaskUsesFrozenProcurementPricing(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	channelId := seedHarnessLedgerChannel(t, "economy", "frozen-task-group", 0.8, 10)

	// The task was accepted when the procurement ratio was 0.8 with 10% tax.
	task := &model.Task{
		TaskID:    "task-frozen-pricing",
		ChannelId: channelId,
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				OriginModelName: "economy",
				ModelRatio:      1,
				Procurement: &model.TaskProcurementContext{
					Version:               1,
					PreGroupQuota:         "1000000",
					PricingGroup:          "frozen-task-group",
					ProcurementGroupRatio: 0.8,
					TaxRatePercent:        10,
					QuotaPerUnit:          500_000,
				},
			},
		},
	}

	// Live configuration now says something completely different.
	changedRatio := ratio_setting.GetGroupRatioCopy()
	changedRatio["frozen-task-group"] = 0.01
	encoded, err := common.Marshal(changedRatio)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(string(encoded)))

	money := harnessSettlementMoneyForTask(task, model.HarnessSettlementStatusSettled, 1_000_000)

	assert.Equal(t, "2", money.BaseAmount)
	assert.Equal(t, "1.76", money.TotalAmount,
		"the frozen 0.8 ratio and 10%% tax must win over the live 0.01 ratio")
	assert.Equal(t, 0.8, money.GroupRatio, "the frozen ratio is reported, not the live one")
	assert.Equal(t, 10.0, money.TaxRatePercent)
}

// TestLedgerTaskWithoutFrozenContextKeepsLegacyPricing covers the compatibility
// rule: a task submitted before the frozen context existed (Version 0) must keep
// reading live configuration instead of treating its zero values as real prices,
// which would silently price it as free.
func TestLedgerTaskWithoutFrozenContextKeepsLegacyPricing(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	channelId := seedHarnessLedgerChannel(t, "economy", "legacy-task-group", 0.8, 0)

	task := &model.Task{
		TaskID:    "task-legacy-pricing",
		ChannelId: channelId,
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{OriginModelName: "economy"},
		},
	}

	money := harnessSettlementMoneyForTask(task, model.HarnessSettlementStatusSettled, 1_000_000)

	assert.Equal(t, "2", money.BaseAmount)
	assert.Equal(t, 0.8, money.GroupRatio,
		"a task without a frozen context still resolves the live procurement group")
	assert.Equal(t, harnessCostCalculationFallback, money.CostCalculationVersion,
		"and is tagged as a fallback rather than as corrected")
}

// TestLedgerTaskRefundKeepsFrozenPurchase documents the refund rule: a customer
// refund does not prove the supplier reversed the charge. Vendors do invoice for
// tasks that fail after generation or time out, so zeroing the amount here would
// under-report exactly the anomalies reconciliation exists to find. The frozen
// purchase is kept and the refund is expressed by the event type and status.
func TestLedgerTaskRefundKeepsFrozenPurchase(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	channelId := seedHarnessLedgerChannel(t, "economy", "refund-task-group", 0.8, 10)

	task := &model.Task{
		TaskID:    "task-refunded",
		ChannelId: channelId,
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				OriginModelName: "economy",
				Procurement: &model.TaskProcurementContext{
					Version:               1,
					PreGroupQuota:         "1000000",
					PricingGroup:          "refund-task-group",
					ProcurementGroupRatio: 0.8,
					TaxRatePercent:        10,
					QuotaPerUnit:          500_000,
				},
			},
		},
	}

	money := harnessSettlementMoneyForTask(task, model.HarnessSettlementStatusRefunded, 0)

	// The purchase the task already incurred: 1,000,000 / 500,000 = 2 base,
	// times 0.8 procurement ratio, times 1.10 tax.
	assert.Equal(t, "2", money.BaseAmount,
		"a customer refund must not erase the purchase the task incurred")
	assert.Equal(t, "1.76", money.TotalAmount)
}

// TestLedgerTaskRefundWithoutFrozenAmountStaysZero covers the one case where a
// refunded task legitimately reports nothing: a usage-priced task refunded before
// its quantity was ever learned has no purchase amount to state.
func TestLedgerTaskRefundWithoutFrozenAmountStaysZero(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	channelId := seedHarnessLedgerChannel(t, "economy", "refund-unknown-group", 0.8, 10)

	task := &model.Task{
		TaskID:    "task-refunded-unknown",
		ChannelId: channelId,
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				OriginModelName: "economy",
				Procurement: &model.TaskProcurementContext{
					Version:               1,
					PricingGroup:          "refund-unknown-group",
					ProcurementGroupRatio: 0.8,
					TaxRatePercent:        10,
					QuotaPerUnit:          500_000,
				},
			},
		},
	}

	money := harnessSettlementMoneyForTask(task, model.HarnessSettlementStatusRefunded, 0)

	assert.Equal(t, "0", money.BaseAmount,
		"a task whose quantity was never learned has no purchase amount to report")
	assert.Equal(t, "0", money.TotalAmount)
}

// TestLedgerUsageProvenanceClassifiesRealSources pins how the ledger decides where
// a usage figure came from. The three states are not interchangeable: a locally
// counted request is an estimate even though it produced a full usage object, and
// a request that obtained nothing must say so rather than report a measured zero.
func TestLedgerUsageProvenanceClassifiesRealSources(t *testing.T) {
	for _, tc := range []struct {
		name           string
		localCount     bool
		hasMeasurement bool
		wantSource     string
		wantEstimated  bool
	}{
		{
			name:           "upstream reported the tokens",
			hasMeasurement: true,
			wantSource:     harnessUsageSourceUpstream,
			wantEstimated:  false,
		},
		{
			name:           "gateway counted the tokens itself",
			localCount:     true,
			hasMeasurement: true,
			wantSource:     harnessUsageSourceEstimated,
			wantEstimated:  true,
		},
		{
			name:           "no usage obtained at all",
			hasMeasurement: false,
			wantSource:     harnessUsageSourceUnavailable,
			wantEstimated:  false,
		},
		{
			name:           "local counting without a measurement is still unavailable",
			localCount:     true,
			hasMeasurement: false,
			wantSource:     harnessUsageSourceUnavailable,
			wantEstimated:  false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, estimated := harnessUsageProvenance(tc.localCount, tc.hasMeasurement)
			assert.Equal(t, tc.wantSource, source)
			assert.Equal(t, tc.wantEstimated, estimated)
		})
	}
}

// TestLedgerTotalOnlyUsagePublishesNullSplit covers the total-only case: an
// upstream that reports a total but no input/output split must not be published as
// a pair of zeros, because zero is a measurement and the split was never measured.
func TestLedgerTotalOnlyUsagePublishesNullSplit(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	correlation := harnessSettlementCorrelation{HarnessRequestId: "hrq_total_only", UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream}
	settlement := &model.HarnessSettlement{
		RequestId:        "gw-total-only",
		Status:           model.HarnessSettlementStatusSettled,
		ModelName:        "economy",
		TotalTokens:      1000,
		TotalTokensKnown: true,
		SplitUnknown:     true,
	}
	recordHarnessSettlement(settlement, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	require.NoError(t, model.DB.Model(&model.HarnessLedgerEvent{}).
		Where("gateway_request_id = ?", "gw-total-only").
		Update("observed_at_ms", 1).Error)

	items, _, _, err := ListHarnessUsageEvents(0, 50, 0, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	usage := items[0].Usage
	assert.False(t, usage.SplitKnown, "the split was never measured")
	assert.Nil(t, usage.InputTokens, "an unknown input count must not be published as zero")
	assert.Nil(t, usage.OutputTokens, "an unknown output count must not be published as zero")
	require.NotNil(t, usage.TotalTokens, "the reported total must survive")
	assert.Equal(t, 1000, *usage.TotalTokens)
}

// TestLedgerRepeatedSettleAppendsNothing keeps the ledger from growing versions
// that describe no change: a settle repeating the same status and the same amount
// is not a new fact.
func TestLedgerRepeatedSettleAppendsNothing(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	correlation := harnessSettlementCorrelation{HarnessRequestId: "hrq_repeat", UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream}
	first := &model.HarnessSettlement{RequestId: "gw-repeat", Status: model.HarnessSettlementStatusSettled, ModelName: "economy"}
	recordHarnessSettlement(first, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	repeat := &model.HarnessSettlement{RequestId: "gw-repeat", Status: model.HarnessSettlementStatusSettled, ModelName: "economy"}
	recordHarnessSettlement(repeat, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	var costEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ? AND gateway_request_id = ?",
		model.HarnessLedgerStreamCost, "gw-repeat").Find(&costEvents).Error)
	assert.Len(t, costEvents, 1, "a no-change settle must not append another version")
}

// TestLedgerCostEventTypesFollowTheChain pins the event-type rules on the real
// append path: the first event is a cost, a change of amount is a correction, and
// a status change with the same amount is a status_change.
func TestLedgerCostEventTypesFollowTheChain(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	correlation := harnessSettlementCorrelation{HarnessRequestId: "hrq_chain", UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream}

	first := &model.HarnessSettlement{RequestId: "gw-chain", Status: model.HarnessSettlementStatusPending, ModelName: "economy"}
	recordHarnessSettlement(first, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	// Same amount, different status: a status change, not a cost change.
	statusOnly := &model.HarnessSettlement{RequestId: "gw-chain", Status: model.HarnessSettlementStatusSettled, ModelName: "economy"}
	recordHarnessSettlement(statusOnly, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	// Different amount: a correction.
	corrected := &model.HarnessSettlement{RequestId: "gw-chain", Status: model.HarnessSettlementStatusSettled, ModelName: "economy"}
	recordHarnessSettlement(corrected, harnessSettlementMoneyForQuota(nil, 2_000_000, 0, "economy"), correlation)

	var events []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ? AND gateway_request_id = ?",
		model.HarnessLedgerStreamCost, "gw-chain").Order("id ASC").Find(&events).Error)
	require.Len(t, events, 3)

	assert.Equal(t, model.HarnessLedgerEventTypeCost, events[0].EventType, "the first event is never a correction")
	assert.Nil(t, events[0].SupersedesEventId)
	assert.Equal(t, 1, events[0].RecordVersion)

	assert.Equal(t, model.HarnessLedgerEventTypeStatusChange, events[1].EventType,
		"an unchanged amount with a new status is a status change")
	assert.Equal(t, 2, events[1].RecordVersion)
	require.NotNil(t, events[1].SupersedesEventId)
	assert.Equal(t, events[0].EventId, *events[1].SupersedesEventId)

	assert.Equal(t, model.HarnessLedgerEventTypeCostCorrection, events[2].EventType,
		"a changed amount is a correction")
	assert.Equal(t, 3, events[2].RecordVersion)
	require.NotNil(t, events[2].SupersedesEventId)
	assert.Equal(t, events[1].EventId, *events[2].SupersedesEventId)
	assert.Equal(t, "4", events[2].BaseAmount, "each version states the full amount, never a delta")
}

// TestLedgerCostEventTypesCoverTheRiskyBranches pins the two transitions the
// happy-path chain does not reach: a refund arriving as the very first cost event
// (which must be a plain cost, not a refund with nothing to reverse), and a
// failure that also changes the amount (which is a correction, because the amount
// changed, not merely a status change).
func TestLedgerCostEventTypesCoverTheRiskyBranches(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	correlation := harnessSettlementCorrelation{HarnessRequestId: "hrq_branches", UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream}

	// Case 1: a refund as the first event has nothing to reverse, so it is simply
	// the first statement of what the request is worth.
	first := &model.HarnessSettlement{RequestId: "gw-first-refund", Status: model.HarnessSettlementStatusRefunded, ModelName: "economy"}
	recordHarnessSettlement(first, harnessSettlementMoneyForQuota(nil, 0, 0, "economy"), correlation)

	var firstEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ? AND gateway_request_id = ?",
		model.HarnessLedgerStreamCost, "gw-first-refund").Find(&firstEvents).Error)
	require.Len(t, firstEvents, 1)
	assert.Equal(t, model.HarnessLedgerEventTypeCost, firstEvents[0].EventType,
		"a first event is never a refund claim about a prior state")
	assert.Nil(t, firstEvents[0].SupersedesEventId)
	assert.Equal(t, 1, firstEvents[0].RecordVersion)

	// Case 2: pending, then failed with a different amount. The amount moved, so
	// the platform is told the cost was corrected rather than merely re-stated.
	start := &model.HarnessSettlement{RequestId: "gw-failed", Status: model.HarnessSettlementStatusPending, ModelName: "economy"}
	recordHarnessSettlement(start, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	failed := &model.HarnessSettlement{RequestId: "gw-failed", Status: model.HarnessSettlementStatusFailed, ModelName: "economy"}
	recordHarnessSettlement(failed, harnessSettlementMoneyForQuota(nil, 500_000, 0, "economy"), correlation)

	var failedEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ? AND gateway_request_id = ?",
		model.HarnessLedgerStreamCost, "gw-failed").Order("id ASC").Find(&failedEvents).Error)
	require.Len(t, failedEvents, 2)
	assert.Equal(t, model.HarnessLedgerEventTypeCostCorrection, failedEvents[1].EventType,
		"a failure that changes the amount corrects the cost")
	assert.Equal(t, 2, failedEvents[1].RecordVersion)
	require.NotNil(t, failedEvents[1].SupersedesEventId)
	assert.Equal(t, failedEvents[0].EventId, *failedEvents[1].SupersedesEventId)
}

// TestPostTextConsumeQuotaRecordsUnavailableUsage drives the real text settle path
// when the upstream returned no usage: the ledger must say unavailable rather than
// report a measured zero, because the gateway never observed the consumption.
func TestPostTextConsumeQuotaRecordsUnavailableUsage(t *testing.T) {
	ledgerTestDB(t)
	const userID, channelID = 9103, 9103
	require.NoError(t, model.DB.Create(&model.User{Id: userID, Username: "no_usage_user", AffCode: "nousg1", Quota: 10_000_000, Status: common.UserStatusEnabled}).Error)
	seedChannel(t, channelID)

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyHarnessRequestId, "hrq_no_usage")
	common.SetContextKey(ctx, constant.ContextKeyHarnessRunId, "run_no_usage")
	// A request id is required for the settlement row to be written at all.
	common.SetContextKey(ctx, common.RequestIdKey, "gw-no-usage")

	info := &relaycommon.RelayInfo{
		UserId:          userID,
		OriginModelName: "no-usage-ledger-model",
		UsingGroup:      "default",
		StartTime:       time.Now(),
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: channelID},
		PriceData: hosttypes.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			GroupRatioInfo:  hosttypes.GroupRatioInfo{GroupRatio: 1},
		},
	}

	// An empty usage object is what a provider that reported nothing looks like.
	PostTextConsumeQuota(ctx, info, &dto.Usage{}, nil)

	var rows []model.HarnessSettlement
	require.NoError(t, model.DB.Where("request_id = ?", "gw-no-usage").Find(&rows).Error)
	require.Len(t, rows, 1, "the settlement row is written regardless of usage")

	require.NoError(t, model.DB.Model(&model.HarnessLedgerEvent{}).
		Where("gateway_request_id = ?", "gw-no-usage").Update("observed_at_ms", 1).Error)
	var events []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ? AND gateway_request_id = ?",
		model.HarnessLedgerStreamUsage, "gw-no-usage").Find(&events).Error)
	require.Len(t, events, 1, "the usage stream still records the event")

	assert.Equal(t, harnessUsageSourceUnavailable, events[0].UsageSource,
		"an empty upstream usage block reported nothing, so the ledger must say unavailable")
	assert.False(t, events[0].IsEstimated)
}

// TestLedgerSkippedRequestStillRecordsSnapshot keeps the operator's own view intact:
// a non-platform request emits no events but still gets a settlement row, which is
// what lets the gateway report its procurement cost without exposing it to the
// platform.
func TestLedgerSkippedRequestStillRecordsSnapshot(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	settlement := &model.HarnessSettlement{
		RequestId: "gw-internal-only",
		Status:    model.HarnessSettlementStatusSettled,
		ModelName: "economy",
	}
	recordHarnessSettlement(settlement, harnessSettlementMoneyForQuota(nil, 500_000, 0, "economy"),
		harnessSettlementCorrelation{UsageUnit: harnessUsageUnitToken})

	var events []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("gateway_request_id = ?", "gw-internal-only").Find(&events).Error)
	assert.Empty(t, events, "a non-platform request must not enter the platform feed")

	var rows []model.HarnessSettlement
	require.NoError(t, model.DB.Where("request_id = ?", "gw-internal-only").Find(&rows).Error)
	require.Len(t, rows, 1, "the operator still needs the cost")
	// The amount follows the charged quota, which the minimum-charge rule lifts to
	// one quota; the point of the case is that the row exists at all.
	assert.NotEmpty(t, rows[0].BaseAmount)
}

// TestLedgerRepeatedSettleStillRefreshesSnapshot pins the other half of the dedup
// rule: no new event is appended, but the snapshot must still be refreshed. The
// append path is the only writer of that row for platform traffic, so returning
// early without the upsert would freeze the row at its first values and silently
// drop later corrections.
func TestLedgerRepeatedSettleStillRefreshesSnapshot(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	correlation := harnessSettlementCorrelation{HarnessRequestId: "hrq_refresh", UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream}
	first := &model.HarnessSettlement{
		RequestId: "gw-refresh", Status: model.HarnessSettlementStatusSettled,
		ModelName: "economy", PromptTokens: 10, CompletionTokens: 5,
	}
	recordHarnessSettlement(first, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	// Same status and same amount, but the task later learned its real total.
	refreshed := &model.HarnessSettlement{
		RequestId: "gw-refresh", Status: model.HarnessSettlementStatusSettled,
		ModelName: "economy", TotalTokens: 42, TotalTokensKnown: true, SplitUnknown: true,
	}
	recordHarnessSettlement(refreshed, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	var events []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ? AND gateway_request_id = ?",
		model.HarnessLedgerStreamCost, "gw-refresh").Find(&events).Error)
	assert.Len(t, events, 1, "a no-change settle appends no event")

	var rows []model.HarnessSettlement
	require.NoError(t, model.DB.Where("request_id = ?", "gw-refresh").Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, 42, rows[0].TotalTokens,
		"the snapshot must still take the newly learned usage")
	assert.True(t, rows[0].TotalTokensKnown)
	assert.True(t, rows[0].SplitUnknown)
}

// TestTextQuotaSummaryCapturesPreGroupBasis is the end-to-end form of the
// acceptance invariant: for one usage and one set of model prices, only the
// caller's group ratio varies, and the captured pre-group amount must stay fixed
// while the charged quota scales with the ratio.
//
// This exercises the real billing formula rather than the ledger helper, so a
// future reordering of the arithmetic that reintroduces the ratio into the
// captured amount fails here.
func TestTextQuotaSummaryCapturesPreGroupBasis(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	usage := &dto.Usage{PromptTokens: 400, CompletionTokens: 100, TotalTokens: 500}

	var preGroupQuota string
	var chargedQuota int
	for index, groupRatio := range []float64{1, 2, 0.5} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
		ctx.Request.URL.Path = "/v1/chat/completions"

		info := &relaycommon.RelayInfo{
			OriginModelName: "basis-invariant-model",
			RelayFormat:     types.RelayFormatOpenAI,
			PriceData: hosttypes.PriceData{
				ModelRatio:      1,
				CompletionRatio: 2,
				GroupRatioInfo:  hosttypes.GroupRatioInfo{GroupRatio: groupRatio},
			},
		}

		summary := calculateTextQuotaSummary(ctx, info, usage)

		if index == 0 {
			preGroupQuota = summary.PreGroupQuota.String()
			chargedQuota = summary.Quota
			require.Equal(t, 600, chargedQuota,
				"400 prompt + 100 completion x2 at model ratio 1 is 600 quota at group ratio 1")
			continue
		}
		assert.Equal(t, preGroupQuota, summary.PreGroupQuota.String(),
			"customer group ratio %v must not change the captured pre-group amount", groupRatio)
		assert.Equal(t, int(float64(chargedQuota)*groupRatio), summary.Quota,
			"the customer is still charged the pre-group amount times their ratio")
	}
}

// TestTextQuotaSummaryExcludesToolSurchargeFromBasis pins the decision that a
// platform-set tool surcharge is customer pricing, not a purchase: the customer
// keeps paying it, but the procurement base must not include it.
func TestTextQuotaSummaryExcludesToolSurchargeFromBasis(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	operation_setting.SetToolPriceForTest("web_search", 10)
	t.Cleanup(func() { operation_setting.DeleteToolPriceForTest("web_search") })

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName: "basis-tool-model",
		RelayFormat:     types.RelayFormatOpenAI,
		PriceData: hosttypes.PriceData{
			ModelRatio:     1,
			GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1},
		},
		ResponsesUsageInfo: &relaycommon.ResponsesUsageInfo{
			BuiltInTools: map[string]*relaycommon.BuildInToolInfo{
				"web_search": {ToolName: "web_search", CallCount: 1},
			},
		},
	}

	summary := calculateTextQuotaSummary(ctx, info, &dto.Usage{PromptTokens: 100, TotalTokens: 100})

	// The surcharge is 10/1000 * 500000 = 5000 quota, and the customer pays it.
	require.False(t, summary.ToolCallSurchargeQuota.IsZero(),
		"the tool surcharge must still be charged to the customer")
	assert.Equal(t, common.QuotaFromDecimal(summary.PreGroupQuota)+common.QuotaFromDecimal(summary.ToolCallSurchargeQuota), summary.Quota,
		"the customer charge is the pre-group amount plus the surcharge")
	assert.Equal(t, "100", summary.PreGroupQuota.String(),
		"the procurement basis must not include the platform tool surcharge")
}

// TestTextQuotaSummaryFloorChargeKeepsRealBasis covers the minimum-charge rule:
// a request whose real amount rounds to zero is still charged one quota so it is
// not silently free. That artificial unit is a charging rule, but billable usage
// did occur, so the real pre-rounding amount must survive in the procurement
// basis -- dropping it would discard exactly the precision the basis exists to
// preserve.
func TestTextQuotaSummaryFloorChargeKeepsRealBasis(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName: "floor-model",
		RelayFormat:     types.RelayFormatOpenAI,
		PriceData: hosttypes.PriceData{
			// A tiny per-token price so a one-token request rounds to zero before
			// the floor applies.
			ModelRatio:     0.000001,
			GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1},
		},
	}

	summary := calculateTextQuotaSummary(ctx, info, &dto.Usage{PromptTokens: 1, TotalTokens: 1})

	assert.Equal(t, 1, summary.Quota, "a non-free model still charges at least one quota")
	assert.Equal(t, "0.000001", summary.PreGroupQuota.String(),
		"the real pre-rounding amount survives; only the artificial floor is not a purchase")
}

// TestTextQuotaSummaryNoUsageBooksNoPurchase keeps the other half of that rule:
// with no billable usage at all there is no purchase, so the basis is zero
// rather than whatever arithmetic happened to produce.
func TestTextQuotaSummaryNoUsageBooksNoPurchase(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName: "no-usage-model",
		RelayFormat:     types.RelayFormatOpenAI,
		PriceData: hosttypes.PriceData{
			ModelRatio:     1,
			GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1},
		},
	}

	summary := calculateTextQuotaSummary(ctx, info, &dto.Usage{})

	assert.Equal(t, 0, summary.Quota)
	assert.True(t, summary.PreGroupQuota.IsZero(),
		"nothing was billable, so nothing was purchased, got %s", summary.PreGroupQuota)
}

// TestPostAudioConsumeQuotaRecordsProcurementBasis drives the real audio settle
// path, which is a separate entry point from the text one: it used to call the
// wrapper that discards the pre-group amount, so its ledger row silently fell
// back to the charged quota and re-imported the customer group ratio.
func TestPostAudioConsumeQuotaRecordsProcurementBasis(t *testing.T) {
	truncate(t)
	// The expected amounts are expressed in quota units, so the conversion rate
	// must be pinned rather than inherited from whatever ran before.
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000
	const userID, channelID = 9101, 9101
	// seedUser writes one row per call with an empty aff_code, so calling it twice
	// against the same truncated database violates that unique index. Creating the
	// row here with its own name keeps this case independent of test order.
	require.NoError(t, model.DB.Create(&model.User{
		Id: userID, Username: "audio_basis_user", AffCode: "audio01", Quota: 10_000_000, Status: common.UserStatusEnabled,
	}).Error)
	seedChannel(t, channelID)

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/audio/speech", nil)

	// Group ratio 3 must scale the charge but never the procurement basis.
	info := &relaycommon.RelayInfo{
		UserId:          userID,
		OriginModelName: "audio-basis-model",
		UsingGroup:      "default",
		StartTime:       time.Now(),
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: channelID},
		PriceData: hosttypes.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			GroupRatioInfo:  hosttypes.GroupRatioInfo{GroupRatio: 3},
		},
	}

	// The audio settle path reads the detailed input breakdown, not PromptTokens.
	usage := &dto.Usage{
		PromptTokensDetails: dto.InputTokenDetails{TextTokens: 100},
		TotalTokens:         100,
	}
	PostAudioConsumeQuota(ctx, info, usage, nil)

	require.NotNil(t, info.ProcurementBasis, "the audio path must record a procurement basis")
	preGroup, ok := info.ProcurementBasis.PreGroupQuotaDecimal()
	require.True(t, ok)

	// 100 input text tokens at model ratio 1 is 100 quota before the ratio.
	assert.Equal(t, "100", preGroup.String(),
		"the basis is the amount before the customer group ratio")
	assert.Equal(t, 3.0, info.ProcurementBasis.CustomerGroupRatio)
	assert.Equal(t, 300, getLastLog(t).Quota,
		"the customer is still charged the pre-group amount times the group ratio")
}

// TestPostWssConsumeQuotaZeroUsageRecordsZeroBasis keeps the basis consistent with
// the final settlement state: a request with no billable usage is settled at zero,
// so the basis must be zero too instead of describing an amount that was zeroed.
func TestPostWssConsumeQuotaZeroUsageRecordsZeroBasis(t *testing.T) {
	truncate(t)
	const userID, channelID = 9102, 9102
	require.NoError(t, model.DB.Create(&model.User{Id: userID, Username: "wss_zero_user", AffCode: "wss001", Quota: 10_000_000, Status: common.UserStatusEnabled}).Error)
	seedChannel(t, channelID)

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/v1/realtime", nil)

	info := &relaycommon.RelayInfo{
		UserId:          userID,
		OriginModelName: "wss-zero-model",
		UsingGroup:      "default",
		StartTime:       time.Now(),
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: channelID},
		PriceData: hosttypes.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			GroupRatioInfo:  hosttypes.GroupRatioInfo{GroupRatio: 3},
		},
	}

	PostWssConsumeQuota(ctx, info, "wss-zero-model", &dto.RealtimeUsage{}, nil)

	require.NotNil(t, info.ProcurementBasis)
	preGroup, ok := info.ProcurementBasis.PreGroupQuotaDecimal()
	require.True(t, ok)
	assert.True(t, preGroup.IsZero(),
		"no usage means no purchase, got %s", preGroup)
}

// TestLedgerTaskFrozenZeroModelRatioIsReal pins the boundary between "frozen
// zero" and "missing". Models such as glm-4-flash are legitimately configured at
// model ratio 0, so a frozen zero is a real price. Presence is decided by the
// version tag: treating a zero as missing would silently re-price a deliberately
// free model from live configuration and break the freeze guarantee.
func TestLedgerTaskFrozenZeroModelRatioIsReal(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	channelId := seedHarnessLedgerChannel(t, "economy", "zero-ratio-group", 0.8, 0)

	task := &model.Task{
		TaskID:    "task-zero-ratio",
		ChannelId: channelId,
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				OriginModelName: "glm-4-flash",
				ModelRatio:      0,
				Procurement: &model.TaskProcurementContext{
					Version:               1,
					PricingGroup:          "zero-ratio-group",
					ProcurementGroupRatio: 0.8,
					QuotaPerUnit:          500_000,
				},
			},
		},
	}

	// The frozen zero must win over a live ratio that says otherwise.
	changed := ratio_setting.GetModelRatioCopy()
	changed["glm-4-flash"] = 5
	encoded, err := common.Marshal(changed)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))

	assert.Equal(t, 0.0, frozenTaskModelRatio(task, 5),
		"a frozen zero model ratio is a real price, not a missing value")
}

// TestLedgerWarnsWhenProcurementBasisIsMissing pins the operator-visible signal
// for a silent mis-booking. The fallback re-imports the charged quota, which is
// only equivalent when the customer group ratio is exactly 1; in every other case
// the amount is either mis-scaled or, at ratio 0, loses the entire cost. Those
// cases must be reported instead of writing a plausible-looking number.
func TestLedgerWarnsWhenProcurementBasisIsMissing(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	const warningFragment = "no procurement basis"

	for _, tc := range []struct {
		name      string
		basis     *hosttypes.ProcurementBasis
		wantWarn  bool
		reasoning string
	}{
		{
			name:      "no basis at all",
			basis:     nil,
			wantWarn:  true,
			reasoning: "the ratio is unknown, so the fallback cannot be proven equivalent",
		},
		{
			name:      "zero group ratio",
			basis:     &hosttypes.ProcurementBasis{CustomerGroupRatio: 0},
			wantWarn:  true,
			reasoning: "a zero ratio charges nothing, so the fallback loses the whole cost",
		},
		{
			name:      "discounted group ratio",
			basis:     &hosttypes.ProcurementBasis{CustomerGroupRatio: 0.5},
			wantWarn:  true,
			reasoning: "the charged quota is half the pre-group amount, so the fallback under-reports",
		},
		{
			name:      "premium group ratio",
			basis:     &hosttypes.ProcurementBasis{CustomerGroupRatio: 3},
			wantWarn:  true,
			reasoning: "the charged quota is three times the pre-group amount, so the fallback over-reports",
		},
		{
			name:      "unit group ratio is provably equivalent",
			basis:     &hosttypes.ProcurementBasis{CustomerGroupRatio: 1},
			wantWarn:  false,
			reasoning: "with ratio 1 the fallback and the corrected amount are the same number",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var captured strings.Builder
			originalWriter := gin.DefaultWriter
			gin.DefaultWriter = &captured
			t.Cleanup(func() { gin.DefaultWriter = originalWriter })

			money := harnessSettlementMoneyForQuota(tc.basis, 1_000_000, 0, "model-without-pricing-group")

			warned := strings.Contains(captured.String(), warningFragment)
			assert.Equal(t, tc.wantWarn, warned, tc.reasoning)
			assert.Equal(t, harnessCostCalculationFallback, money.CostCalculationVersion,
				"every fallback row is tagged as one, warned or not")
			assert.Equal(t, "2", money.BaseAmount,
				"the fallback still prices the request rather than booking zero")
		})
	}
}

// TestLedgerTotalAmountIncludesSupplierTax pins the cost formula against the
// contract sample: base 0.011500, group ratio 0.8 and 6.5% tax must yield 0.0098,
// not the untaxed 0.0092.
func TestLedgerTotalAmountIncludesSupplierTax(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	// quota 5750 / 500000 = 0.0115 base, matching the contract sample.
	common.QuotaPerUnit = 500_000

	channelId := seedHarnessLedgerChannel(t, "economy", "ledger-tax-group", 0.8, 6.5)

	money := harnessSettlementMoneyForQuota(nil, 5750, channelId, "economy")

	assert.Equal(t, "0.0115", normalizedDecimal(t, money.BaseAmount))
	assert.Equal(t, 0.8, money.GroupRatio)
	assert.Equal(t, 6.5, money.TaxRatePercent)
	assert.Equal(t, "0.009798", normalizedDecimal(t, money.TotalAmount),
		"total_amount must include the supplier tax, matching the contract sample")
}

// harnessLedgerStreamCount returns how many events one stream holds for a request.
func harnessLedgerStreamCount(t *testing.T, gatewayRequestId, stream string) []model.HarnessLedgerEvent {
	t.Helper()
	var events []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("gateway_request_id = ? AND stream = ?", gatewayRequestId, stream).
		Order("id ASC").Find(&events).Error)
	return events
}

// TestLedgerChangeDetectionIsPerStream pins the rule that each feed records only
// its own changes. The two streams answer different questions, so a change in one
// must leave the other untouched instead of appending an event that repeats what
// it already said.
//
// Every case settles the same request twice and asserts the resulting event count
// in each stream, which is the observable behaviour the platform depends on: its
// cursor walk must not see versions that describe no change in that feed.
func TestLedgerChangeDetectionIsPerStream(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	correlation := harnessSettlementCorrelation{HarnessRequestId: "hrq_detect", UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream}

	// taxChannel applies a 10% supplier tax, so total_amount carries the tax but
	// base_amount does not.
	taxChannel := seedHarnessLedgerChannel(t, "economy", "detect-tax-group", 0.8, 10)

	settle := func(requestId string, settlement *model.HarnessSettlement, quota int, channelId int, corr harnessSettlementCorrelation) {
		settlement.RequestId = requestId
		settlement.ModelName = "economy"
		recordHarnessSettlement(settlement, harnessSettlementMoneyForQuota(nil, quota, channelId, "economy"), corr)
	}

	t.Run("base unchanged but total changes", func(t *testing.T) {
		const requestId = "gw-detect-total"
		// The same quota prices the same base on both channels; only the supplier
		// tax differs, so base_amount stays put while total_amount moves. That is
		// exactly the case a base-only comparison would silently swallow.
		untaxedChannel := seedHarnessLedgerChannel(t, "economy", "detect-untaxed-group", 0.8, 0)

		settle(requestId, &model.HarnessSettlement{Status: model.HarnessSettlementStatusSettled},
			1_000_000, untaxedChannel, correlation)
		settle(requestId, &model.HarnessSettlement{Status: model.HarnessSettlementStatusSettled},
			1_000_000, taxChannel, correlation)

		costs := harnessLedgerStreamCount(t, requestId, model.HarnessLedgerStreamCost)
		require.Len(t, costs, 2, "a changed total_amount is a new cost fact")
		assert.Equal(t, costs[0].BaseAmount, costs[1].BaseAmount, "the base did not move")
		assert.NotEqual(t, costs[0].TotalAmount, costs[1].TotalAmount, "the total did")
		assert.Equal(t, 2, costs[1].RecordVersion)
		// A moved total is a correction even though the base is unchanged. Keying
		// the event type on the base alone would publish status_change here and tell
		// the platform the amount it already has is still current.
		assert.Equal(t, model.HarnessLedgerEventTypeCostCorrection, costs[1].EventType,
			"a changed total_amount is a cost correction, not a status change")
		require.NotNil(t, costs[1].SupersedesEventId)
		assert.Equal(t, costs[0].EventId, *costs[1].SupersedesEventId)

		usages := harnessLedgerStreamCount(t, requestId, model.HarnessLedgerStreamUsage)
		assert.Len(t, usages, 1, "the cost change must not append a usage event")
	})

	t.Run("cost unchanged but usage corrected", func(t *testing.T) {
		const requestId = "gw-detect-usage"
		settle(requestId, &model.HarnessSettlement{
			Status:           model.HarnessSettlementStatusSettled,
			PromptTokens:     10,
			CompletionTokens: 5,
		}, 1_000_000, taxChannel, correlation)

		// Same money, corrected token counts.
		settle(requestId, &model.HarnessSettlement{
			Status:           model.HarnessSettlementStatusSettled,
			PromptTokens:     10,
			CompletionTokens: 9,
		}, 1_000_000, taxChannel, correlation)

		usages := harnessLedgerStreamCount(t, requestId, model.HarnessLedgerStreamUsage)
		require.Len(t, usages, 2, "a corrected token count is a new usage fact")
		assert.Equal(t, 2, usages[1].RecordVersion)
		require.NotNil(t, usages[1].SupersedesEventId)
		assert.Equal(t, usages[0].EventId, *usages[1].SupersedesEventId)
		assert.Equal(t, 9, usages[1].CompletionTokens)

		costs := harnessLedgerStreamCount(t, requestId, model.HarnessLedgerStreamCost)
		assert.Len(t, costs, 1, "a usage correction must not append a cost event")
	})

	t.Run("usage unchanged but cost corrected", func(t *testing.T) {
		const requestId = "gw-detect-cost"
		const promptTokens, completionTokens = 12, 6
		settle(requestId, &model.HarnessSettlement{
			Status:           model.HarnessSettlementStatusSettled,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
		}, 1_000_000, taxChannel, correlation)

		// The same usage, but the purchase amount moves.
		settle(requestId, &model.HarnessSettlement{
			Status:           model.HarnessSettlementStatusSettled,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
		}, 2_000_000, taxChannel, correlation)

		costs := harnessLedgerStreamCount(t, requestId, model.HarnessLedgerStreamCost)
		require.Len(t, costs, 2, "a changed purchase amount is a new cost fact")
		assert.Equal(t, 2, costs[1].RecordVersion)

		usages := harnessLedgerStreamCount(t, requestId, model.HarnessLedgerStreamUsage)
		assert.Len(t, usages, 1, "an unchanged usage must not append a usage event")
	})

	t.Run("completely repeated settle", func(t *testing.T) {
		const requestId = "gw-detect-repeat"
		for range 2 {
			settle(requestId, &model.HarnessSettlement{
				Status:           model.HarnessSettlementStatusSettled,
				PromptTokens:     15,
				CompletionTokens: 7,
			}, 1_000_000, taxChannel, correlation)
		}

		assert.Len(t, harnessLedgerStreamCount(t, requestId, model.HarnessLedgerStreamUsage), 1,
			"a repeated settle says nothing new about usage")
		assert.Len(t, harnessLedgerStreamCount(t, requestId, model.HarnessLedgerStreamCost), 1,
			"a repeated settle says nothing new about cost")

		// The snapshot is still refreshed, because this path is its only writer.
		var rows []model.HarnessSettlement
		require.NoError(t, model.DB.Where("request_id = ?", requestId).Find(&rows).Error)
		require.Len(t, rows, 1)
		assert.Equal(t, 15, rows[0].PromptTokens)
	})

	t.Run("both streams change together", func(t *testing.T) {
		const requestId = "gw-detect-both"
		settle(requestId, &model.HarnessSettlement{
			Status:           model.HarnessSettlementStatusSettled,
			PromptTokens:     20,
			CompletionTokens: 8,
		}, 1_000_000, taxChannel, correlation)

		// A refund that also reports the usage it finally measured.
		settle(requestId, &model.HarnessSettlement{
			Status:           model.HarnessSettlementStatusRefunded,
			PromptTokens:     20,
			CompletionTokens: 8,
			CacheReadTokens:  3,
		}, 0, taxChannel, correlation)

		usages := harnessLedgerStreamCount(t, requestId, model.HarnessLedgerStreamUsage)
		require.Len(t, usages, 2)
		costs := harnessLedgerStreamCount(t, requestId, model.HarnessLedgerStreamCost)
		require.Len(t, costs, 2)

		// Each stream advances and links within itself, never across streams.
		assert.Equal(t, 2, usages[1].RecordVersion)
		assert.Equal(t, 2, costs[1].RecordVersion)
		require.NotNil(t, usages[1].SupersedesEventId)
		require.NotNil(t, costs[1].SupersedesEventId)
		assert.Equal(t, usages[0].EventId, *usages[1].SupersedesEventId)
		assert.Equal(t, costs[0].EventId, *costs[1].SupersedesEventId)
		assert.Contains(t, *usages[1].SupersedesEventId, model.HarnessLedgerStreamUsage+"_")
		assert.Contains(t, *costs[1].SupersedesEventId, model.HarnessLedgerStreamCost+"_")
		assert.Equal(t, model.HarnessLedgerEventTypeRefund, costs[1].EventType)
	})
}

// TestLedgerUsageCorrectionTypingForNonRefundSettles pins the event type when a
// usage figure is corrected while the settlement itself stays settled.
//
// The contract defines usage_correction as a new event that supersedes the earlier
// one. A corrected token count is exactly that, regardless of whether the money
// moved: labelling it plain "usage" would tell the platform the earlier figure is
// still current, which is the one thing the feed must never say.
func TestLedgerUsageCorrectionTypingForNonRefundSettles(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	correlation := harnessSettlementCorrelation{
		HarnessRequestId: "hrq_usage_typing",
		HarnessRunId:     "run_usage_typing",
		UsageUnit:        harnessUsageUnitToken,
		UsageSource:      harnessUsageSourceUpstream,
	}

	first := &model.HarnessSettlement{
		RequestId:        "gw-usage-typing",
		Status:           model.HarnessSettlementStatusSettled,
		ModelName:        "economy",
		PromptTokens:     100,
		CompletionTokens: 20,
	}
	recordHarnessSettlement(first, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	// The same money, a corrected completion count, and the settlement stays settled.
	corrected := &model.HarnessSettlement{
		RequestId:        "gw-usage-typing",
		Status:           model.HarnessSettlementStatusSettled,
		ModelName:        "economy",
		PromptTokens:     100,
		CompletionTokens: 25,
	}
	recordHarnessSettlement(corrected, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)

	var usageEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ? AND gateway_request_id = ?",
		model.HarnessLedgerStreamUsage, "gw-usage-typing").Order("id ASC").Find(&usageEvents).Error)
	require.Len(t, usageEvents, 2, "a corrected token count is a new usage fact")

	assert.Equal(t, model.HarnessLedgerEventTypeUsage, usageEvents[0].EventType,
		"the first event states usage rather than correcting anything")
	assert.Equal(t, model.HarnessLedgerEventTypeUsageCorrection, usageEvents[1].EventType,
		"a correction must be labelled a correction even when the status did not change")
	assert.Equal(t, 2, usageEvents[1].RecordVersion)
	require.NotNil(t, usageEvents[1].SupersedesEventId)
	assert.Equal(t, usageEvents[0].EventId, *usageEvents[1].SupersedesEventId)
}
