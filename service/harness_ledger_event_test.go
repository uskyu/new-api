package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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
	assert.Empty(t, usageEvents[0].MediaFacts)
	wire := usageEventFromLedgerEvent(usageEvents[0])
	assert.Nil(t, wire.MediaUsage, "the switch-off wire must not grow a media block")

	var costEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("stream = ?", model.HarnessLedgerStreamCost).Find(&costEvents).Error)
	require.Len(t, costEvents, 1, "one cost event per settlement")
	assert.Equal(t, "USD", costEvents[0].Currency)
	assert.Equal(t, "2", costEvents[0].BaseAmount)
	assert.Equal(t, "2", costEvents[0].TotalAmount)
	assert.Equal(t, model.HarnessLedgerStatusSettled, costEvents[0].Status)
	assert.Equal(t, "token", costEvents[0].BillingBasis)
}

// ledgerUsageTestTask builds the task shape the polling settle path hands to
// recordTaskHarnessSettlement: the submit path stored the platform correlation
// and a frozen procurement context on private data, and the settlement row is
// created from those fields on the first settle.
func ledgerUsageTestTask(taskID, requestID, harnessRequestID, harnessRunID, modelName string, quota int) *model.Task {
	return &model.Task{
		TaskID:     taskID,
		UserId:     9,
		ChannelId:  0,
		Quota:      quota,
		SubmitTime: 1730000000,
		Properties: model.Properties{OriginModelName: modelName},
		PrivateData: model.TaskPrivateData{
			TokenId: 7,
			Execution: &model.TaskExecutionSnapshot{
				RequestID:        requestID,
				HarnessRequestID: harnessRequestID,
				HarnessRunID:     harnessRunID,
			},
			BillingContext: &model.TaskBillingContext{
				OriginModelName: modelName,
				Procurement: &model.TaskProcurementContext{
					Version:               1,
					PricingGroup:          "default",
					ProcurementGroupRatio: 1,
					QuotaPerUnit:          500_000,
					PriceVersion:          "test",
				},
			},
		},
	}
}

// assertUnavailableTaskUsage pins the "not measured" shape. Both the stored
// event and the B1 wire projection must say unavailable with no token figures:
// a task without tokens is never a measured zero and never has a known split.
func assertUnavailableTaskUsage(t *testing.T, event model.HarnessLedgerEvent) {
	t.Helper()
	assert.Equal(t, harnessUsageSourceUnavailable, event.UsageSource)
	assert.True(t, event.SplitUnknown)
	assert.False(t, event.TotalTokensKnown)
	wire := usageEventFromLedgerEvent(event)
	assert.False(t, wire.Usage.SplitKnown)
	assert.Nil(t, wire.Usage.TotalTokens)
	assert.Nil(t, wire.Usage.InputTokens)
	assert.Nil(t, wire.Usage.OutputTokens)
	assert.Nil(t, wire.Usage.CacheReadTokens)
	assert.Nil(t, wire.Usage.CacheWriteTokens)
}

// TestRecordTaskHarnessSettlementUsageSource covers the async task path, whose
// only usage fact is the total the provider reported at completion, and often
// not even that.
//
// A task with no token usage must be published as unavailable, not as a measured
// zero: the usage stream keeps the pending event and the settle appends no
// forced closing event, because a status transition that measured nothing new is
// a cost-side fact. A task that later reports a total appends exactly one usage
// correction, and a usage-less re-settle neither invents a new usage event nor
// clears the total it already recorded.
func TestRecordTaskHarnessSettlementUsageSource(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	// assertZeroUsageTask runs the shared no-token flow: pending, settle,
	// re-settle. Each call uses its own gateway request id, so the event counts
	// asserted here are independent per scenario.
	assertZeroUsageTask := func(t *testing.T, task *model.Task) {
		t.Helper()
		gatewayRequestID := task.PrivateData.Execution.RequestID

		recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)

		usages := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
		require.Len(t, usages, 1, "the pending submit is the task's only usage fact")
		assert.Equal(t, model.HarnessSettlementStatusPending, usages[0].Status)
		assert.Equal(t, model.HarnessLedgerEventTypeUsage, usages[0].EventType)
		assertUnavailableTaskUsage(t, usages[0])

		costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
		require.Len(t, costs, 1)
		assert.Equal(t, model.HarnessSettlementStatusPending, costs[0].Status)
		assert.Equal(t, model.HarnessLedgerEventTypeCost, costs[0].EventType)

		recordTaskHarnessSettlement(task, model.HarnessSettlementStatusSettled, task.Quota)

		usages = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
		require.Len(t, usages, 1, "a settle without usage says nothing new about consumption")
		assertUnavailableTaskUsage(t, usages[0])

		costs = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
		require.Len(t, costs, 2, "the status transition is a cost-side fact")
		assert.Equal(t, model.HarnessLedgerEventTypeStatusChange, costs[1].EventType)
		assert.Equal(t, model.HarnessSettlementStatusSettled, costs[1].Status)
		assert.Equal(t, costs[0].BaseAmount, costs[1].BaseAmount, "settling does not reprice the task")
		assert.Equal(t, costs[0].TotalAmount, costs[1].TotalAmount, "settling does not reprice the task")

		recordTaskHarnessSettlement(task, model.HarnessSettlementStatusSettled, task.Quota)
		assert.Len(t, harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage), 1,
			"a repeated settle appends no usage event")
		assert.Len(t, harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost), 2,
			"a repeated settle appends no cost event")
	}

	t.Run("per-image task without tokens", func(t *testing.T) {
		task := ledgerUsageTestTask("task-banana-no-usage", "gw-banana-no-usage",
			"hrq_banana_no_usage", "run_banana_no_usage", "Nano Banana 2.1 1K", 4500)
		assertZeroUsageTask(t, task)
	})

	t.Run("PAI per-call task without tokens", func(t *testing.T) {
		task := ledgerUsageTestTask("task-pai-no-usage", "gw-pai-no-usage",
			"hrq_pai_no_usage", "run_pai_no_usage", "seedance2-fast-720p-once-1", 4500)
		assertZeroUsageTask(t, task)
	})

	t.Run("async task that later reports a total", func(t *testing.T) {
		const gatewayRequestID = "gw-total-usage"
		task := ledgerUsageTestTask("task-total-usage", gatewayRequestID,
			"hrq_total_usage", "run_total_usage", "async-total-model", 5000)

		recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)
		usages := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
		require.Len(t, usages, 1)
		assertUnavailableTaskUsage(t, usages[0])
		costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
		require.Len(t, costs, 1)

		recordTaskHarnessSettlement(task, model.HarnessSettlementStatusSettled, task.Quota, 5000)

		usages = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
		require.Len(t, usages, 2, "learning the total is exactly one usage correction")
		correction := usages[1]
		assert.Equal(t, model.HarnessLedgerEventTypeUsageCorrection, correction.EventType)
		assert.Equal(t, 2, correction.RecordVersion)
		require.NotNil(t, correction.SupersedesEventId)
		assert.Equal(t, usages[0].EventId, *correction.SupersedesEventId)
		assert.Equal(t, harnessUsageSourceUpstream, correction.UsageSource)
		assert.True(t, correction.TotalTokensKnown)
		assert.Equal(t, 5000, correction.TotalTokens)
		assert.True(t, correction.SplitUnknown, "a total still says nothing about the input/output split")
		wire := usageEventFromLedgerEvent(correction)
		assert.False(t, wire.Usage.SplitKnown)
		require.NotNil(t, wire.Usage.TotalTokens)
		assert.Equal(t, 5000, *wire.Usage.TotalTokens)
		assert.Nil(t, wire.Usage.InputTokens)
		assert.Nil(t, wire.Usage.OutputTokens)
		assert.Nil(t, wire.Usage.CacheReadTokens)
		assert.Nil(t, wire.Usage.CacheWriteTokens)

		costs = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
		require.Len(t, costs, 2)
		assert.Equal(t, model.HarnessLedgerEventTypeStatusChange, costs[1].EventType)
		assert.Equal(t, model.HarnessSettlementStatusSettled, costs[1].Status)
		assert.Equal(t, costs[0].BaseAmount, costs[1].BaseAmount, "settling does not reprice the task")
		assert.Equal(t, costs[0].TotalAmount, costs[1].TotalAmount, "settling does not reprice the task")

		// A usage-less re-settle must neither clear the learned total nor append
		// another event on either stream.
		recordTaskHarnessSettlement(task, model.HarnessSettlementStatusSettled, task.Quota)

		usages = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
		require.Len(t, usages, 2, "a usage-less re-settle appends no usage event")
		assert.True(t, usages[1].TotalTokensKnown)
		assert.Equal(t, 5000, usages[1].TotalTokens, "the learned total survives a usage-less settle")

		costs = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
		require.Len(t, costs, 2, "a usage-less re-settle appends no cost event")
		assert.Equal(t, costs[0].BaseAmount, costs[1].BaseAmount)
	})
}

// ledgerMediaFactsJSON builds the canonical fact list used by the media tests:
// requested_seconds from the request plus output_seconds either unavailable or
// measured. A nil output means the gateway learned nothing about it.
func ledgerMediaFactsJSON(t *testing.T, requested *string, output *string, outputSource string) string {
	t.Helper()
	var facts relaycommon.MediaFacts
	if requested != nil {
		require.NoError(t, facts.SetQuantity(
			relaycommon.MediaFactNameRequestedSeconds,
			relaycommon.MediaDimensionVideo,
			relaycommon.MediaUnitVideoSecond,
			relaycommon.MediaFactSourceRequest,
			*requested,
		))
	}
	if output == nil {
		require.NoError(t, facts.SetUnavailable(
			relaycommon.MediaFactNameOutputSeconds,
			relaycommon.MediaDimensionVideo,
			relaycommon.MediaUnitVideoSecond,
		))
	} else {
		require.NoError(t, facts.SetQuantity(
			relaycommon.MediaFactNameOutputSeconds,
			relaycommon.MediaDimensionVideo,
			relaycommon.MediaUnitVideoSecond,
			outputSource,
			*output,
		))
	}
	return facts.CanonicalJSON()
}

func harnessMediaFactByName(t *testing.T, usage *HarnessMediaUsage, name string) HarnessMediaFact {
	t.Helper()
	require.NotNil(t, usage)
	for _, fact := range usage.Facts {
		if fact.Name == name {
			return fact
		}
	}
	require.FailNowf(t, "missing media fact", "fact %q not found", name)
	return HarnessMediaFact{}
}

// TestRecordTaskHarnessSettlementMediaFactsVersionChain pins the P1-A task
// chain: the submit facts are published on pending B1, a settle with unchanged
// facts adds no usage event, and a completion that learns the actual value
// appends exactly one correction without repricing the task.
func TestRecordTaskHarnessSettlementMediaFactsVersionChain(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	const gatewayRequestID = "gw-media-facts"
	task := ledgerUsageTestTask("task-media-facts", gatewayRequestID,
		"hrq_media_facts", "run_media_facts", "seedance2.5-flash-480p-once-2", 4500)
	requested := "5"
	task.PrivateData.BillingContext.MediaLedger = true
	task.PrivateData.BillingContext.MediaFacts = ledgerMediaFactsJSON(t, &requested, nil, "")

	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)
	usages := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
	require.Len(t, usages, 1)
	wire := usageEventFromLedgerEvent(usages[0])
	require.NotNil(t, wire.MediaUsage)
	assert.Equal(t, relaycommon.MediaFactsVersion, wire.MediaUsage.Version)
	output := harnessMediaFactByName(t, wire.MediaUsage, relaycommon.MediaFactNameOutputSeconds)
	assert.Nil(t, output.Quantity, "unknown output must be null, never zero")
	assert.Equal(t, relaycommon.MediaFactSourceUnavailable, output.Source)
	requestedFact := harnessMediaFactByName(t, wire.MediaUsage, relaycommon.MediaFactNameRequestedSeconds)
	requestedQuantity, ok := requestedFact.Quantity.(string)
	require.True(t, ok, "video seconds stay a decimal string")
	assert.Equal(t, "5", requestedQuantity)
	assert.Equal(t, relaycommon.MediaFactSourceRequest, requestedFact.Source)

	encoded, err := common.Marshal(wire)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"quantity":"5"`, "video seconds are a JSON string")
	assert.Contains(t, string(encoded), `"quantity":null`, "unknown stays JSON null")

	costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 1)

	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusSettled, task.Quota)
	usages = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
	require.Len(t, usages, 1, "the same facts on settle are not a new usage fact")
	costs = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 2, "the status transition is a cost-side fact")
	assert.Equal(t, costs[0].BaseAmount, costs[1].BaseAmount, "media facts must not reprice the task")
	assert.Equal(t, costs[0].TotalAmount, costs[1].TotalAmount, "media facts must not reprice the task")

	actual := "5"
	task.PrivateData.BillingContext.MediaFacts = ledgerMediaFactsJSON(t, &requested, &actual, relaycommon.MediaFactSourceUpstream)
	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusSettled, task.Quota)

	usages = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
	require.Len(t, usages, 2, "learning the output is exactly one usage correction")
	correction := usages[1]
	assert.Equal(t, model.HarnessLedgerEventTypeUsageCorrection, correction.EventType)
	assert.Equal(t, 2, correction.RecordVersion)
	require.NotNil(t, correction.SupersedesEventId)
	assert.Equal(t, usages[0].EventId, *correction.SupersedesEventId)
	wire = usageEventFromLedgerEvent(correction)
	output = harnessMediaFactByName(t, wire.MediaUsage, relaycommon.MediaFactNameOutputSeconds)
	outputQuantity, ok := output.Quantity.(string)
	require.True(t, ok, "video seconds stay a decimal string")
	assert.Equal(t, "5", outputQuantity)
	assert.Equal(t, relaycommon.MediaFactSourceUpstream, output.Source)

	costs = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 3, "the newly known billed quantity is a cost-side fact")
	quantityEvent := costs[2]
	assert.Equal(t, model.HarnessLedgerEventTypeCostCorrection, quantityEvent.EventType,
		"a billed-quantity change is a cost correction even when the amount is unchanged")
	assert.Equal(t, "video_second", quantityEvent.BillingBasis)
	assert.Equal(t, "5", quantityEvent.BillingQuantity)
	assert.Equal(t, relaycommon.MediaFactSourceUpstream, quantityEvent.BillingQuantitySource)
	assert.NotEmpty(t, quantityEvent.BillingComponents)
	assert.Equal(t, costs[1].BaseAmount, quantityEvent.BaseAmount, "the quantity correction must not reprice the task")
	assert.Equal(t, costs[1].TotalAmount, quantityEvent.TotalAmount, "the quantity correction must not reprice the task")
}

// TestRecordTaskHarnessSettlementIgnoresMediaFactsWhenSwitchOff pins the
// gate: facts present on the task row but MediaLedger false must leave the B1
// wire and the version chain exactly as they were before P1-A.
func TestRecordTaskHarnessSettlementIgnoresMediaFactsWhenSwitchOff(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	const gatewayRequestID = "gw-media-off"
	task := ledgerUsageTestTask("task-media-off", gatewayRequestID,
		"hrq_media_off", "run_media_off", "seedance2.5-flash-480p-once-2", 4500)
	requested := "5"
	task.PrivateData.BillingContext.MediaFacts = ledgerMediaFactsJSON(t, &requested, nil, "")
	// MediaLedger stays false: the submit path did not capture the switch.

	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)
	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusSettled, task.Quota)

	usages := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
	require.Len(t, usages, 1, "the switch-off path emits exactly the pre-existing B1 events")
	assert.Empty(t, usages[0].MediaFacts)
	wire := usageEventFromLedgerEvent(usages[0])
	assert.Nil(t, wire.MediaUsage)
	encoded, err := common.Marshal(wire)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "media_usage", "the switch-off wire must stay byte-shape compatible")

	costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 2)
	assert.Empty(t, costs[0].MediaFacts)
	assert.Empty(t, costs[1].MediaFacts)
	for _, cost := range costs {
		assert.Equal(t, "token", cost.BillingBasis)
		assert.Empty(t, cost.BillingQuantity)
		assert.Empty(t, cost.BillingQuantitySource)
		assert.Empty(t, cost.BillingComponents)
		assert.Empty(t, cost.BillingExprHash)
		assert.Zero(t, cost.BillingExprVersion)
		encodedCost, err := common.Marshal(costRecordFromLedgerEvent(cost))
		require.NoError(t, err)
		assert.NotContains(t, string(encodedCost), "billing_quantity")
		assert.NotContains(t, string(encodedCost), "billing_components")
		assert.NotContains(t, string(encodedCost), "cost_calculation_version")
	}
}

// TestRecordTaskHarnessSettlementKeepsMediaFactsOnTokenCorrection covers the
// correction path where a token change triggers a new B1 version but the settle
// carries no fact list. The new version must inherit the facts, not erase them.
func TestRecordTaskHarnessSettlementKeepsMediaFactsOnTokenCorrection(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	const gatewayRequestID = "gw-media-token-correction"
	task := ledgerUsageTestTask("task-media-token-correction", gatewayRequestID,
		"hrq_media_token_correction", "run_media_token_correction", "async-video-model", 4500)
	requested := "5"
	task.PrivateData.BillingContext.MediaLedger = true
	task.PrivateData.BillingContext.MediaFacts = ledgerMediaFactsJSON(t, &requested, nil, "")

	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)
	usages := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
	require.Len(t, usages, 1)
	publishedFacts := usages[0].MediaFacts
	require.NotEmpty(t, publishedFacts)

	task.PrivateData.BillingContext.MediaFacts = ""
	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusSettled, task.Quota, 5000)

	usages = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
	require.Len(t, usages, 2, "learning the total is a usage correction")
	correction := usages[1]
	assert.Equal(t, 2, correction.RecordVersion)
	assert.Equal(t, publishedFacts, correction.MediaFacts, "the correction must inherit the published facts")
	assert.True(t, correction.TotalTokensKnown)
	assert.NotNil(t, usageEventFromLedgerEvent(correction).MediaUsage)

	costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 2)
	assert.Equal(t, costs[0].BaseAmount, costs[1].BaseAmount, "a token correction must not reprice the task")
}

// TestRecordHarnessSettlementPublishesImageMediaFacts covers the synchronous
// image path: the B1 event carries the observed count, while the cost event
// keeps the existing token basis and amount.
func TestRecordHarnessSettlementPublishesImageMediaFacts(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	var facts relaycommon.MediaFacts
	require.NoError(t, facts.SetQuantity(
		relaycommon.MediaFactNameImageCount,
		relaycommon.MediaDimensionImage,
		relaycommon.MediaUnitImage,
		relaycommon.MediaFactSourceObserved,
		"1",
	))
	settlement := &model.HarnessSettlement{
		RequestId: "gw-image-media",
		Status:    model.HarnessSettlementStatusSettled,
		ModelName: "gpt-image-2.6-flare",
	}
	correlation := harnessSettlementCorrelation{
		HarnessRequestId: "hrq_image_media",
		UsageUnit:        harnessUsageUnitToken,
		UsageSource:      harnessUsageSourceUpstream,
		MediaFacts:       facts.CanonicalJSON(),
	}
	recordHarnessSettlement(settlement, harnessSettlementMoneyForQuota(nil, 9_000, 0, "gpt-image-2.6-flare"), correlation)

	usages := harnessLedgerStreamCount(t, "gw-image-media", model.HarnessLedgerStreamUsage)
	require.Len(t, usages, 1)
	wire := usageEventFromLedgerEvent(usages[0])
	require.NotNil(t, wire.MediaUsage)
	count := harnessMediaFactByName(t, wire.MediaUsage, relaycommon.MediaFactNameImageCount)
	assert.Equal(t, int64(1), count.Quantity, "image counts are integers on the wire")
	assert.Equal(t, relaycommon.MediaFactSourceObserved, count.Source)

	encoded, err := common.Marshal(wire)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"quantity":1`, "image count is a JSON integer")

	costs := harnessLedgerStreamCount(t, "gw-image-media", model.HarnessLedgerStreamCost)
	require.Len(t, costs, 1)
	assert.Equal(t, "token", costs[0].BillingBasis)
	assert.Empty(t, costs[0].MediaFacts)
}

// TestRecordTaskHarnessSettlementPlanPerCallBilling pins the plan/fixed-price
// case: one accepted call is exactly one billed unit, explained by the frozen
// expression snapshot rather than by any request-second figure.
func TestRecordTaskHarnessSettlementPlanPerCallBilling(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	const gatewayRequestID = "gw-plan-per-call"
	task := ledgerUsageTestTask("task-plan-per-call", gatewayRequestID,
		"hrq_plan_per_call", "run_plan_per_call", "seedance2-fast-720p-once-1", 4500)
	requested := "5"
	bc := task.PrivateData.BillingContext
	bc.MediaLedger = true
	bc.MediaFacts = ledgerMediaFactsJSON(t, &requested, nil, "")
	bc.PerCallBilling = true
	bc.TieredSnapshot = &billingexpr.BillingSnapshot{
		ExprString:  `fixed(0.02)`,
		ExprHash:    "hash-plan-per-call",
		ExprVersion: 3,
	}

	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)

	costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 1)
	wire := costRecordFromLedgerEvent(costs[0])
	assert.Equal(t, "per_call", wire.BillingBasis)
	require.NotNil(t, wire.BillingQuantity)
	assert.Equal(t, "1", *wire.BillingQuantity)
	assert.Equal(t, relaycommon.MediaFactSourceRequest, wire.BillingQuantitySource)
	require.Len(t, wire.BillingComponents, 1)
	assert.Equal(t, HarnessBillingComponent{
		Basis: "per_call", Quantity: "1", Source: relaycommon.MediaFactSourceRequest,
	}, wire.BillingComponents[0])
	assert.Equal(t, "hash-plan-per-call", wire.BillingExprHash)
	assert.Equal(t, 3, wire.BillingExprVersion)
	assert.NotEmpty(t, wire.CostCalculationVersion, "media evidence exposes the calculation version")
}

// TestRecordTaskHarnessSettlementPerCallRequestSeconds pins the C group: a
// per-call-priced task without an expression bills the seconds frozen in the
// request, and an unavailable output fact must not replace that value.
func TestRecordTaskHarnessSettlementPerCallRequestSeconds(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	const gatewayRequestID = "gw-per-call-request-seconds"
	task := ledgerUsageTestTask("task-per-call-request-seconds", gatewayRequestID,
		"hrq_per_call_request_seconds", "run_per_call_request_seconds", "seedance2-fast-720p-once-1", 4500)
	requested := "10"
	bc := task.PrivateData.BillingContext
	bc.MediaLedger = true
	bc.PerCallBilling = true
	bc.MediaFacts = ledgerMediaFactsJSON(t, &requested, nil, "")

	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)

	costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 1)
	wire := costRecordFromLedgerEvent(costs[0])
	assert.Equal(t, "video_second", wire.BillingBasis)
	require.NotNil(t, wire.BillingQuantity)
	assert.Equal(t, "10", *wire.BillingQuantity)
	assert.Equal(t, relaycommon.MediaFactSourceRequest, wire.BillingQuantitySource)
	require.Len(t, wire.BillingComponents, 1)
	assert.Equal(t, HarnessBillingComponent{
		Basis: "video_second", Quantity: "10", Source: relaycommon.MediaFactSourceRequest,
	}, wire.BillingComponents[0])
}

// TestRecordTaskHarnessSettlementOutputSecondsBilling pins the A/TA group: the
// billed quantity is the measured output only, so an unknown output leaves the
// quantity absent and a later upstream measurement appends a new cost event
// without repricing the task.
func TestRecordTaskHarnessSettlementOutputSecondsBilling(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	const gatewayRequestID = "gw-output-seconds"
	task := ledgerUsageTestTask("task-output-seconds", gatewayRequestID,
		"hrq_output_seconds", "run_output_seconds", "seedance-output-seconds", 4500)
	requested := "10"
	bc := task.PrivateData.BillingContext
	bc.MediaLedger = true
	bc.MediaFacts = ledgerMediaFactsJSON(t, &requested, nil, "")
	bc.TieredSnapshot = &billingexpr.BillingSnapshot{
		ExprString:  `u("output_seconds") * 0.1`,
		ExprHash:    "hash-output-seconds",
		ExprVersion: 4,
	}

	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)

	costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 1)
	wire := costRecordFromLedgerEvent(costs[0])
	assert.Equal(t, "video_second", wire.BillingBasis)
	assert.Nil(t, wire.BillingQuantity, "an unknown output must not borrow the request estimate")
	assert.Equal(t, "hash-output-seconds", wire.BillingExprHash)
	assert.Equal(t, 4, wire.BillingExprVersion)

	actual := "8.5"
	bc.MediaFacts = ledgerMediaFactsJSON(t, &requested, &actual, relaycommon.MediaFactSourceUpstream)
	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusSettled, task.Quota)

	costs = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 2, "learning the output seconds is a cost-side fact")
	assert.Equal(t, costs[0].BaseAmount, costs[1].BaseAmount, "the quantity correction must not reprice the task")
	assert.Equal(t, costs[0].TotalAmount, costs[1].TotalAmount, "the quantity correction must not reprice the task")
	wire = costRecordFromLedgerEvent(costs[1])
	assert.Equal(t, "video_second", wire.BillingBasis)
	require.NotNil(t, wire.BillingQuantity)
	assert.Equal(t, "8.5", *wire.BillingQuantity)
	assert.Equal(t, relaycommon.MediaFactSourceUpstream, wire.BillingQuantitySource)
	require.Len(t, wire.BillingComponents, 1)
	assert.Equal(t, HarnessBillingComponent{
		Basis: "video_second", Quantity: "8.5", Source: relaycommon.MediaFactSourceUpstream,
	}, wire.BillingComponents[0])
}

// TestRecordTaskHarnessSettlementReferenceSecondsBilling pins the B group: the
// expression bills requested plus reference seconds and the B3 components keep
// both quantities separately auditable.
func TestRecordTaskHarnessSettlementReferenceSecondsBilling(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	const gatewayRequestID = "gw-reference-seconds"
	task := ledgerUsageTestTask("task-reference-seconds", gatewayRequestID,
		"hrq_reference_seconds", "run_reference_seconds", "seedance-reference-seconds", 4500)
	var facts relaycommon.MediaFacts
	require.NoError(t, facts.SetQuantity(
		relaycommon.MediaFactNameRequestedSeconds,
		relaycommon.MediaDimensionVideo,
		relaycommon.MediaUnitVideoSecond,
		relaycommon.MediaFactSourceRequest,
		"10",
	))
	require.NoError(t, facts.SetQuantity(
		relaycommon.MediaFactNameReferenceSeconds,
		relaycommon.MediaDimensionVideo,
		relaycommon.MediaUnitVideoSecond,
		relaycommon.MediaFactSourceRequest,
		"2",
	))
	bc := task.PrivateData.BillingContext
	bc.MediaLedger = true
	bc.MediaFacts = facts.CanonicalJSON()
	bc.TieredSnapshot = &billingexpr.BillingSnapshot{
		ExprString:  `u("requested_seconds") + u("reference_seconds")`,
		ExprHash:    "hash-reference-seconds",
		ExprVersion: 2,
	}

	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)

	costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 1)
	wire := costRecordFromLedgerEvent(costs[0])
	assert.Equal(t, "video_second", wire.BillingBasis)
	require.NotNil(t, wire.BillingQuantity)
	assert.Equal(t, "12", *wire.BillingQuantity)
	assert.Equal(t, relaycommon.MediaFactSourceRequest, wire.BillingQuantitySource)
	assert.Equal(t, []HarnessBillingComponent{
		{Basis: "video_second", Quantity: "10", Source: relaycommon.MediaFactSourceRequest},
		{Basis: "reference_video_second", Quantity: "2", Source: relaycommon.MediaFactSourceRequest},
	}, wire.BillingComponents)
	assert.Equal(t, "hash-reference-seconds", wire.BillingExprHash)
	assert.Equal(t, 2, wire.BillingExprVersion)
}

// TestRecordHarnessSettlementTokenOnlyOmitsMediaBillingFields pins legacy B3 parity:
// a token-only event has no media evidence, so the additive P1-A2 fields must
// not appear on the wire at all.
func TestRecordHarnessSettlementTokenOnlyOmitsMediaBillingFields(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	settlement := &model.HarnessSettlement{
		RequestId:        "gw-token-only-wire",
		Status:           model.HarnessSettlementStatusSettled,
		ModelName:        "economy",
		PromptTokens:     10,
		CompletionTokens: 20,
	}
	correlation := harnessSettlementCorrelation{
		HarnessRequestId: "hrq_token_only_wire",
		UsageUnit:        harnessUsageUnitToken,
		UsageSource:      harnessUsageSourceUpstream,
	}
	recordHarnessSettlement(settlement, harnessSettlementMoneyForQuota(nil, 500_000, 0, "economy"), correlation)

	costs := harnessLedgerStreamCount(t, "gw-token-only-wire", model.HarnessLedgerStreamCost)
	require.Len(t, costs, 1)
	wire := costRecordFromLedgerEvent(costs[0])
	assert.Equal(t, "token", wire.BillingBasis)
	assert.Nil(t, wire.BillingQuantity)
	assert.Empty(t, wire.BillingQuantitySource)
	assert.Empty(t, wire.BillingComponents)
	assert.Empty(t, wire.CostCalculationVersion)
	assert.Empty(t, wire.BillingExprHash)
	assert.Zero(t, wire.BillingExprVersion)

	encoded, err := common.Marshal(wire)
	require.NoError(t, err)
	for _, field := range []string{
		"billing_quantity",
		"billing_quantity_source",
		"billing_components",
		"cost_calculation_version",
		"billing_expr_hash",
		"billing_expr_version",
	} {
		assert.NotContains(t, string(encoded), `"`+field+`"`,
			"the token-only wire must stay byte-shape compatible")
	}
}

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
	// Each version states the full amount for its status rather than a delta. This
	// refund is quota-based and therefore zero; a task refund with a frozen
	// purchase keeps a non-zero amount, so the invariant is "full amount, never a
	// delta" rather than "a refund is always zero".
	assert.Equal(t, "0", second.TotalAmount, "a refund states its full amount, not a delta")

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

	// The same status on a request that DOES have history stays a correction. The
	// refund also reports a cache-read count the first settle did not, because with
	// per-stream change detection a usage event is only appended when the measured
	// usage actually moves; a pure status transition appends none.
	settled := &model.HarnessSettlement{
		RequestId:        "gw-request-later-refund",
		Status:           model.HarnessSettlementStatusSettled,
		ModelName:        "economy",
		PromptTokens:     100,
		CompletionTokens: 20,
	}
	recordHarnessSettlement(settled, harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "economy"), correlation)
	laterRefund := &model.HarnessSettlement{
		RequestId:        "gw-request-later-refund",
		Status:           model.HarnessSettlementStatusRefunded,
		ModelName:        "economy",
		PromptTokens:     100,
		CompletionTokens: 20,
		CacheReadTokens:  7,
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
