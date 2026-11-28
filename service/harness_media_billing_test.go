package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mediaFactIn returns the stored fact for a dimension/name pair. The internal
// canonical list is keyed by both fields, so the dimension is part of the lookup.
func mediaFactIn(t *testing.T, facts *relaycommon.MediaFacts, dimension, name string) relaycommon.MediaFact {
	t.Helper()
	require.NotNil(t, facts)
	for _, fact := range facts.Facts {
		if fact.Dimension == dimension && fact.Name == name {
			return fact
		}
	}
	require.FailNowf(t, "missing media fact", "%s/%s not found", dimension, name)
	return relaycommon.MediaFact{}
}

// TestMergeTaskCompletionMediaFacts pins the completion-side fact ingestion: the
// plugin's usage keys become frozen facts with the source each quantity actually
// has, and a key the completion did not report leaves the stored fact untouched.
func TestMergeTaskCompletionMediaFacts(t *testing.T) {
	t.Run("maps completion usage facts", func(t *testing.T) {
		requested := "5"
		task := &model.Task{PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				MediaLedger: true,
				MediaFacts:  ledgerMediaFactsJSON(t, &requested, nil, ""),
			},
		}}

		MergeTaskCompletionMediaFacts(task, &relaycommon.TaskInfo{UsageFacts: map[string]any{
			"output_seconds":    8.5,
			"input_seconds":     3.25,
			"reference_seconds": 2,
			"image_count":       1,
		}})

		facts, err := relaycommon.ParseMediaFacts(task.PrivateData.BillingContext.MediaFacts)
		require.NoError(t, err)
		require.NotNil(t, facts)

		output := mediaFactIn(t, facts, relaycommon.MediaDimensionVideo, relaycommon.MediaFactNameOutputSeconds)
		require.NotNil(t, output.Quantity)
		assert.Equal(t, "8.5", *output.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceUpstream, output.Source)
		assert.Equal(t, relaycommon.MediaUnitVideoSecond, output.Unit)

		input := mediaFactIn(t, facts, relaycommon.MediaDimensionVideo, relaycommon.MediaFactNameInputSeconds)
		require.NotNil(t, input.Quantity)
		assert.Equal(t, "3.25", *input.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceUpstream, input.Source)

		reference := mediaFactIn(t, facts, relaycommon.MediaDimensionVideo, relaycommon.MediaFactNameReferenceSeconds)
		require.NotNil(t, reference.Quantity)
		assert.Equal(t, "2", *reference.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceRequest, reference.Source)

		// The submission request value stays the request fact; a completion
		// output_seconds wins over any legacy seconds key.
		requestedFact := mediaFactIn(t, facts, relaycommon.MediaDimensionVideo, relaycommon.MediaFactNameRequestedSeconds)
		require.NotNil(t, requestedFact.Quantity)
		assert.Equal(t, "5", *requestedFact.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceRequest, requestedFact.Source)

		count := mediaFactIn(t, facts, relaycommon.MediaDimensionImage, relaycommon.MediaFactNameImageCount)
		require.NotNil(t, count.Quantity)
		assert.Equal(t, "1", *count.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceObserved, count.Source)
		assert.Equal(t, relaycommon.MediaUnitImage, count.Unit)
	})

	t.Run("seconds-only maps to measured output", func(t *testing.T) {
		requested := "5"
		task := &model.Task{PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				MediaLedger: true,
				MediaFacts:  ledgerMediaFactsJSON(t, &requested, nil, ""),
			},
		}}
		MergeTaskCompletionMediaFacts(task, &relaycommon.TaskInfo{UsageFacts: map[string]any{
			"seconds": 10,
		}})

		facts, err := relaycommon.ParseMediaFacts(task.PrivateData.BillingContext.MediaFacts)
		require.NoError(t, err)
		require.NotNil(t, facts)
		output := mediaFactIn(t, facts, relaycommon.MediaDimensionVideo, relaycommon.MediaFactNameOutputSeconds)
		require.NotNil(t, output.Quantity)
		assert.Equal(t, "10", *output.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceUpstream, output.Source, "completion seconds is an upstream measurement")
		requestedFact := mediaFactIn(t, facts, relaycommon.MediaDimensionVideo, relaycommon.MediaFactNameRequestedSeconds)
		require.NotNil(t, requestedFact.Quantity)
		assert.Equal(t, "5", *requestedFact.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceRequest, requestedFact.Source)
	})

	t.Run("maps requested_seconds key", func(t *testing.T) {
		requested := "5"
		task := &model.Task{PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				MediaLedger: true,
				MediaFacts:  ledgerMediaFactsJSON(t, &requested, nil, ""),
			},
		}}

		MergeTaskCompletionMediaFacts(task, &relaycommon.TaskInfo{UsageFacts: map[string]any{
			"requested_seconds": 7,
		}})

		facts, err := relaycommon.ParseMediaFacts(task.PrivateData.BillingContext.MediaFacts)
		require.NoError(t, err)
		require.NotNil(t, facts)
		requestedFact := mediaFactIn(t, facts, relaycommon.MediaDimensionVideo, relaycommon.MediaFactNameRequestedSeconds)
		require.NotNil(t, requestedFact.Quantity)
		assert.Equal(t, "7", *requestedFact.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceRequest, requestedFact.Source)
	})

	t.Run("missing keys leave unavailable facts untouched", func(t *testing.T) {
		requested := "5"
		task := &model.Task{PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				MediaLedger: true,
				MediaFacts:  ledgerMediaFactsJSON(t, &requested, nil, ""),
			},
		}}
		published := task.PrivateData.BillingContext.MediaFacts

		// None of these keys carries a usable measurement: an absent fact key, a
		// nil value and an unparseable string must all be skipped.
		MergeTaskCompletionMediaFacts(task, &relaycommon.TaskInfo{UsageFacts: map[string]any{
			"unrelated_metric": 9,
			"image_count":      nil,
			"output_seconds":   "not-a-number",
		}})

		assert.Equal(t, published, task.PrivateData.BillingContext.MediaFacts,
			"a completion that reports nothing must not rewrite the stored facts")
		facts, err := relaycommon.ParseMediaFacts(task.PrivateData.BillingContext.MediaFacts)
		require.NoError(t, err)
		require.NotNil(t, facts)
		output := mediaFactIn(t, facts, relaycommon.MediaDimensionVideo, relaycommon.MediaFactNameOutputSeconds)
		assert.Nil(t, output.Quantity, "unknown output stays unknown, never a fabricated zero")
		assert.Equal(t, relaycommon.MediaFactSourceUnavailable, output.Source)
	})

	t.Run("switch off or missing billing context is a no-op", func(t *testing.T) {
		requested := "5"
		published := ledgerMediaFactsJSON(t, &requested, nil, "")
		task := &model.Task{PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				MediaLedger: false,
				MediaFacts:  published,
			},
		}}
		MergeTaskCompletionMediaFacts(task, &relaycommon.TaskInfo{UsageFacts: map[string]any{
			"output_seconds": 8.5,
		}})
		assert.Equal(t, published, task.PrivateData.BillingContext.MediaFacts)

		withoutContext := &model.Task{}
		MergeTaskCompletionMediaFacts(withoutContext, &relaycommon.TaskInfo{UsageFacts: map[string]any{
			"output_seconds": 8.5,
		}})
		assert.Nil(t, withoutContext.PrivateData.BillingContext)

		// Nil inputs must not panic either.
		MergeTaskCompletionMediaFacts(nil, &relaycommon.TaskInfo{})
		MergeTaskCompletionMediaFacts(task, nil)
	})
}

// TestMergeTaskCompletionMediaFactsKeepsPublishedFactsOnEmptyCompletion proves
// the append-only contract across the ingest and ledger steps: an empty completion
// fact list neither rewrites the task's frozen facts nor removes the facts already
// published on B1 when a later settle appends a correction.
func TestMergeTaskCompletionMediaFactsKeepsPublishedFactsOnEmptyCompletion(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	const gatewayRequestID = "gw-media-empty-completion"
	task := ledgerUsageTestTask("task-media-empty-completion", gatewayRequestID,
		"hrq_media_empty_completion", "run_media_empty_completion", "async-video-model", 4500)
	requested := "5"
	task.PrivateData.BillingContext.MediaLedger = true
	task.PrivateData.BillingContext.MediaFacts = ledgerMediaFactsJSON(t, &requested, nil, "")
	published := task.PrivateData.BillingContext.MediaFacts

	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)
	usages := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
	require.Len(t, usages, 1)
	require.NotEmpty(t, usages[0].MediaFacts)

	MergeTaskCompletionMediaFacts(task, &relaycommon.TaskInfo{UsageFacts: map[string]any{}})
	assert.Equal(t, published, task.PrivateData.BillingContext.MediaFacts,
		"an empty completion must not clear the frozen facts")

	// Force a B1 correction through a learned token total; the correction must
	// inherit the facts published by the pending event.
	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusSettled, task.Quota, 9000)
	usages = harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
	require.Len(t, usages, 2, "learning the total is one usage correction")
	assert.Equal(t, model.HarnessLedgerEventTypeUsageCorrection, usages[1].EventType)
	assert.Equal(t, published, usages[1].MediaFacts, "the correction must not erase published facts")
	wire := usageEventFromLedgerEvent(usages[1])
	require.NotNil(t, wire.MediaUsage)
	output := harnessMediaFactByName(t, wire.MediaUsage, relaycommon.MediaFactNameOutputSeconds)
	assert.Nil(t, output.Quantity)
	assert.Equal(t, relaycommon.MediaFactSourceUnavailable, output.Source)

	costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 2)
	assert.Equal(t, costs[0].BaseAmount, costs[1].BaseAmount, "facts must not reprice the task")
}

// TestHarnessTaskMediaBilling pins how an async task's B3 quantity is derived
// from its frozen billing context and media facts for each pricing group.
func TestHarnessTaskMediaBilling(t *testing.T) {
	t.Run("plan per-call", func(t *testing.T) {
		snapshot := &billingexpr.BillingSnapshot{
			ExprString:  "fixed_price * 1",
			ExprHash:    "plan-hash",
			ExprVersion: 4,
		}
		got := harnessTaskMediaBilling(&model.TaskBillingContext{
			MediaLedger:    true,
			PerCallBilling: true,
			TieredSnapshot: snapshot,
		})

		require.NotNil(t, got)
		assert.Equal(t, "per_call", got.Basis)
		assert.Equal(t, "1", got.Quantity)
		assert.Equal(t, "request", got.QuantitySource)
		assert.Equal(t, snapshot.ExprString, got.Expr)
		assert.Equal(t, snapshot.ExprHash, got.ExprHash)
		assert.Equal(t, snapshot.ExprVersion, got.ExprVersion)
		require.Len(t, got.Components, 1)
		assert.Equal(t, HarnessBillingComponent{Basis: "per_call", Quantity: "1", Source: "request"}, got.Components[0])
	})

	t.Run("expression with empty facts keeps basis and expression identity", func(t *testing.T) {
		got := harnessTaskMediaBilling(&model.TaskBillingContext{
			MediaLedger: true,
			TieredSnapshot: &billingexpr.BillingSnapshot{
				ExprString:  "output_seconds * 0.04",
				ExprHash:    "empty-facts-hash",
				ExprVersion: 1,
			},
		})

		require.NotNil(t, got)
		assert.Equal(t, "video_second", got.Basis)
		assert.Empty(t, got.Quantity, "an unknown output must not be replaced by a request estimate")
		assert.Equal(t, "empty-facts-hash", got.ExprHash)
		assert.Equal(t, 1, got.ExprVersion)
	})

	t.Run("plan expression marker without frozen per-call flag", func(t *testing.T) {
		snapshot := &billingexpr.BillingSnapshot{
			ExprString:  `tier("per_video", 0.52)`,
			ExprHash:    "plan-marker-hash",
			ExprVersion: 1,
		}
		got := harnessTaskMediaBilling(&model.TaskBillingContext{
			MediaLedger:    true,
			PerCallBilling: false,
			TieredSnapshot: snapshot,
			MediaFacts:     `{"version":1,"facts":[]}`,
		})

		require.NotNil(t, got)
		assert.Equal(t, "per_call", got.Basis)
		assert.Equal(t, "1", got.Quantity)
		assert.Equal(t, "request", got.QuantitySource)
		assert.Equal(t, "plan-marker-hash", got.ExprHash)
	})

	t.Run("C group per-call without snapshot", func(t *testing.T) {
		var facts relaycommon.MediaFacts
		require.NoError(t, facts.SetQuantity(
			relaycommon.MediaFactNameRequestedSeconds,
			relaycommon.MediaDimensionVideo,
			relaycommon.MediaUnitVideoSecond,
			relaycommon.MediaFactSourceRequest,
			"10",
		))
		got := harnessTaskMediaBilling(&model.TaskBillingContext{
			MediaLedger:    true,
			PerCallBilling: true,
			MediaFacts:     facts.CanonicalJSON(),
		})

		require.NotNil(t, got)
		assert.Equal(t, "video_second", got.Basis)
		assert.Equal(t, "10", got.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceRequest, got.QuantitySource)
		require.Len(t, got.Components, 1)
		assert.Equal(t, "video_second", got.Components[0].Basis)
		assert.Equal(t, "10", got.Components[0].Quantity)
	})

	t.Run("A group measured output", func(t *testing.T) {
		snapshot := &billingexpr.BillingSnapshot{ExprString: "output_seconds * 0.04", ExprHash: "a-hash", ExprVersion: 2}

		t.Run("output unavailable leaves quantity absent", func(t *testing.T) {
			var facts relaycommon.MediaFacts
			require.NoError(t, facts.SetUnavailable(
				relaycommon.MediaFactNameOutputSeconds,
				relaycommon.MediaDimensionVideo,
				relaycommon.MediaUnitVideoSecond,
			))
			got := harnessTaskMediaBilling(&model.TaskBillingContext{
				MediaLedger:    true,
				TieredSnapshot: snapshot,
				MediaFacts:     facts.CanonicalJSON(),
			})

			require.NotNil(t, got)
			assert.Equal(t, "video_second", got.Basis)
			assert.Empty(t, got.Quantity, "an unknown output has no billable quantity")
			assert.Empty(t, got.QuantitySource)
			assert.Empty(t, got.Components)
			assert.Equal(t, snapshot.ExprString, got.Expr)
		})

		t.Run("upstream output supplies quantity", func(t *testing.T) {
			var facts relaycommon.MediaFacts
			require.NoError(t, facts.SetQuantity(
				relaycommon.MediaFactNameOutputSeconds,
				relaycommon.MediaDimensionVideo,
				relaycommon.MediaUnitVideoSecond,
				relaycommon.MediaFactSourceUpstream,
				"8.5",
			))
			got := harnessTaskMediaBilling(&model.TaskBillingContext{
				MediaLedger:    true,
				TieredSnapshot: snapshot,
				MediaFacts:     facts.CanonicalJSON(),
			})

			require.NotNil(t, got)
			assert.Equal(t, "video_second", got.Basis)
			assert.Equal(t, "8.5", got.Quantity)
			assert.Equal(t, relaycommon.MediaFactSourceUpstream, got.QuantitySource)
			require.Len(t, got.Components, 1)
			assert.Equal(t, "video_second", got.Components[0].Basis)
			assert.Equal(t, "8.5", got.Components[0].Quantity)
			assert.Equal(t, relaycommon.MediaFactSourceUpstream, got.Components[0].Source)
		})

		t.Run("input duration adds a component", func(t *testing.T) {
			var facts relaycommon.MediaFacts
			require.NoError(t, facts.SetQuantity(
				relaycommon.MediaFactNameOutputSeconds,
				relaycommon.MediaDimensionVideo,
				relaycommon.MediaUnitVideoSecond,
				relaycommon.MediaFactSourceUpstream,
				"8.5",
			))
			require.NoError(t, facts.SetQuantity(
				relaycommon.MediaFactNameInputSeconds,
				relaycommon.MediaDimensionVideo,
				relaycommon.MediaUnitVideoSecond,
				relaycommon.MediaFactSourceUpstream,
				"3",
			))
			got := harnessTaskMediaBilling(&model.TaskBillingContext{
				MediaLedger:    true,
				TieredSnapshot: snapshot,
				MediaFacts:     facts.CanonicalJSON(),
			})

			require.NotNil(t, got)
			assert.Equal(t, "8.5", got.Quantity)
			require.Len(t, got.Components, 2)
			assert.Equal(t, "8.5", got.Components[0].Quantity)
			assert.Equal(t, "3", got.Components[1].Quantity)
		})
	})

	t.Run("B group requested plus reference", func(t *testing.T) {
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
		got := harnessTaskMediaBilling(&model.TaskBillingContext{
			MediaLedger:    true,
			MediaFacts:     facts.CanonicalJSON(),
			TieredSnapshot: &billingexpr.BillingSnapshot{ExprString: "requested_seconds * 0.5 + reference_seconds * 0.1", ExprHash: "b-hash", ExprVersion: 1},
		})

		require.NotNil(t, got)
		assert.Equal(t, "video_second", got.Basis)
		assert.Equal(t, "12", got.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceRequest, got.QuantitySource)
		require.Len(t, got.Components, 2)
		assert.Equal(t, "video_second", got.Components[0].Basis)
		assert.Equal(t, "10", got.Components[0].Quantity)
		assert.Equal(t, "reference_video_second", got.Components[1].Basis)
		assert.Equal(t, "2", got.Components[1].Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceRequest, got.Components[1].Source)
	})

	t.Run("D group request seconds", func(t *testing.T) {
		var facts relaycommon.MediaFacts
		require.NoError(t, facts.SetQuantity(
			relaycommon.MediaFactNameRequestedSeconds,
			relaycommon.MediaDimensionVideo,
			relaycommon.MediaUnitVideoSecond,
			relaycommon.MediaFactSourceEstimated,
			"5",
		))
		got := harnessTaskMediaBilling(&model.TaskBillingContext{
			MediaLedger:    true,
			MediaFacts:     facts.CanonicalJSON(),
			TieredSnapshot: &billingexpr.BillingSnapshot{ExprString: "seconds * 0.05", ExprHash: "d-hash"},
		})

		require.NotNil(t, got)
		assert.Equal(t, "video_second", got.Basis)
		assert.Equal(t, "5", got.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceEstimated, got.QuantitySource)
		require.Len(t, got.Components, 1)
		assert.Equal(t, relaycommon.MediaFactSourceEstimated, got.Components[0].Source)
	})

	t.Run("nil or switch off", func(t *testing.T) {
		assert.Nil(t, harnessTaskMediaBilling(nil))
		assert.Nil(t, harnessTaskMediaBilling(&model.TaskBillingContext{MediaLedger: false}))
	})
}

// TestHarnessSyncMediaBilling pins the synchronous image explanation: only the
// observed image count is a commercial quantity, and an empty fact list carries
// no B3 media block at all.
func TestHarnessSyncMediaBilling(t *testing.T) {
	t.Run("observed image count", func(t *testing.T) {
		info := &relaycommon.RelayInfo{MediaLedgerEnabled: true}
		require.NoError(t, info.MediaFacts.SetQuantity(
			relaycommon.MediaFactNameImageCount,
			relaycommon.MediaDimensionImage,
			relaycommon.MediaUnitImage,
			relaycommon.MediaFactSourceObserved,
			"1",
		))

		got := harnessSyncMediaBilling(info)
		require.NotNil(t, got)
		assert.Equal(t, "image", got.Basis)
		assert.Equal(t, "1", got.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceObserved, got.QuantitySource)
		require.Len(t, got.Components, 1)
		assert.Equal(t, HarnessBillingComponent{Basis: "image", Quantity: "1", Source: relaycommon.MediaFactSourceObserved}, got.Components[0])
	})

	t.Run("image mode empty facts falls back to requested n", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			MediaLedgerEnabled: true,
			RelayMode:          relayconstant.RelayModeImagesGenerations,
			ImageRequestCount:  3,
		}
		got := harnessSyncMediaBilling(info)
		require.NotNil(t, got)
		assert.Equal(t, "image", got.Basis)
		assert.Equal(t, "3", got.Quantity)
		assert.Equal(t, relaycommon.MediaFactSourceRequest, got.QuantitySource)
	})

	t.Run("empty facts", func(t *testing.T) {
		assert.Nil(t, harnessSyncMediaBilling(nil))
		assert.Nil(t, harnessSyncMediaBilling(&relaycommon.RelayInfo{MediaLedgerEnabled: true}))
	})
}

// TestRecordTaskHarnessSettlementRefundedOmitsMediaBilling pins the refund rule:
// a refunded cost event may carry facts on B1, but it must not claim a per-call
// or video quantity the platform never bought.
func TestRecordTaskHarnessSettlementRefundedOmitsMediaBilling(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	const gatewayRequestID = "gw-media-refund"
	task := ledgerUsageTestTask("task-media-refund", gatewayRequestID,
		"hrq_media_refund", "run_media_refund", "seedance2.5-flash-480p-once-2", 4500)
	requested := "5"
	task.PrivateData.BillingContext.MediaLedger = true
	task.PrivateData.BillingContext.MediaFacts = ledgerMediaFactsJSON(t, &requested, nil, "")
	task.PrivateData.BillingContext.PerCallBilling = true
	task.PrivateData.BillingContext.TieredSnapshot = &billingexpr.BillingSnapshot{
		ExprString:  `tier("per_video", 0.52)`,
		ExprHash:    "refund-plan-hash",
		ExprVersion: 1,
	}

	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)
	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusRefunded, 0)

	costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 2)
	refunded := costs[1]
	assert.Equal(t, model.HarnessLedgerEventTypeRefund, refunded.EventType)
	assert.Equal(t, "token", refunded.BillingBasis, "a refund falls back to the legacy basis")
	assert.Empty(t, refunded.BillingQuantity)
	assert.Empty(t, refunded.BillingQuantitySource)
	assert.Empty(t, refunded.BillingComponents)
	assert.Empty(t, refunded.BillingExprHash)
	assert.Empty(t, refunded.BillingExprVersion)

	wire := costRecordFromLedgerEvent(refunded)
	assert.Equal(t, "token", wire.BillingBasis)
	assert.Nil(t, wire.BillingQuantity)
	assert.Empty(t, wire.BillingComponents)
	assert.Empty(t, wire.CostCalculationVersion)
	assert.Empty(t, wire.BillingExprHash)
	assert.Zero(t, wire.BillingExprVersion)
}

// TestRecordTaskHarnessSettlementTANoUsageZeroSettlement is the deliberate
// detection test for the TA risk: a successful task with no upstream usage may
// settle at zero. It proves the ledger exposes the anomaly (settled + zero amount
// + no billing quantity) without changing the charge or claiming a fix.
func TestRecordTaskHarnessSettlementTANoUsageZeroSettlement(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	const gatewayRequestID = "gw-ta-zero-settlement"
	task := ledgerUsageTestTask("task-ta-zero-settlement", gatewayRequestID,
		"hrq_ta_zero_settlement", "run_ta_zero_settlement", "ta-video-model", 4500)
	requested := "10"
	task.PrivateData.BillingContext.MediaLedger = true
	task.PrivateData.BillingContext.MediaFacts = ledgerMediaFactsJSON(t, &requested, nil, "")
	task.PrivateData.BillingContext.TieredSnapshot = &billingexpr.BillingSnapshot{
		ExprString:  "output_seconds * 0.04",
		ExprHash:    "ta-zero-hash",
		ExprVersion: 1,
	}

	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)
	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusSettled, 0)

	usages := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
	require.Len(t, usages, 1, "no usage fact changed, so B1 keeps its single pending event")
	wireUsage := usageEventFromLedgerEvent(usages[0])
	require.NotNil(t, wireUsage.MediaUsage)
	output := harnessMediaFactByName(t, wireUsage.MediaUsage, relaycommon.MediaFactNameOutputSeconds)
	assert.Nil(t, output.Quantity, "TA no-usage must be null, never a measured zero")
	assert.Equal(t, relaycommon.MediaFactSourceUnavailable, output.Source)

	costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 2, "the zero settlement is a cost-side fact")
	wireCost := costRecordFromLedgerEvent(costs[1])
	assert.Equal(t, model.HarnessSettlementStatusSettled, wireCost.Status)
	assert.Equal(t, "0", wireCost.Billing.TotalAmount)
	assert.Equal(t, "video_second", wireCost.BillingBasis)
	assert.Nil(t, wireCost.BillingQuantity, "the zero amount has no defensible quantity")
	assert.Empty(t, wireCost.BillingComponents)
	assert.Equal(t, "ta-zero-hash", wireCost.BillingExprHash)
	assert.Equal(t, 1, wireCost.BillingExprVersion)
	// The anomaly is identified, not fixed: the function must not re-price the
	// task or fabricate a request-second charge for the zero-settled case.
}

// TestRecordTaskHarnessSettlementMediaEndToEnd links the frozen task context,
// completion ingestion and both ledger streams: the completion's measured output
// must reach B1 as an upstream fact and B3 as the media quantity, without
// changing the amount the task was settled for.
func TestRecordTaskHarnessSettlementMediaEndToEnd(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	const gatewayRequestID = "gw-media-a2-e2e"
	task := ledgerUsageTestTask("task-media-a2-e2e", gatewayRequestID,
		"hrq_media_a2_e2e", "run_media_a2_e2e", "video-a2-model", 4500)
	requested := "5"
	task.PrivateData.BillingContext.MediaLedger = true
	task.PrivateData.BillingContext.MediaFacts = ledgerMediaFactsJSON(t, &requested, nil, "")
	task.PrivateData.BillingContext.TieredSnapshot = &billingexpr.BillingSnapshot{
		ExprString:  "output_seconds * 0.04",
		ExprHash:    "expr-a2-hash",
		ExprVersion: 1,
	}

	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusPending, task.Quota)
	pendingUsages := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
	require.Len(t, pendingUsages, 1)
	pendingWire := usageEventFromLedgerEvent(pendingUsages[0])
	require.NotNil(t, pendingWire.MediaUsage)
	pendingOutput := harnessMediaFactByName(t, pendingWire.MediaUsage, relaycommon.MediaFactNameOutputSeconds)
	assert.Nil(t, pendingOutput.Quantity, "the completion had not reported output yet")

	MergeTaskCompletionMediaFacts(task, &relaycommon.TaskInfo{UsageFacts: map[string]any{
		"output_seconds": 8.5,
	}})
	recordTaskHarnessSettlement(task, model.HarnessSettlementStatusSettled, task.Quota)

	usages := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamUsage)
	require.Len(t, usages, 2, "the measured output is exactly one B1 correction")
	settledUsage := usages[1]
	assert.Equal(t, model.HarnessLedgerEventTypeUsageCorrection, settledUsage.EventType)
	assert.Equal(t, model.HarnessSettlementStatusSettled, settledUsage.Status)
	assert.Equal(t, 2, settledUsage.RecordVersion)
	require.NotNil(t, settledUsage.SupersedesEventId)
	assert.Equal(t, pendingUsages[0].EventId, *settledUsage.SupersedesEventId)
	usageWire := usageEventFromLedgerEvent(settledUsage)
	require.NotNil(t, usageWire.MediaUsage)
	output := harnessMediaFactByName(t, usageWire.MediaUsage, relaycommon.MediaFactNameOutputSeconds)
	outputQuantity, ok := output.Quantity.(string)
	require.True(t, ok, "video seconds stay a decimal string on the wire")
	assert.Equal(t, "8.5", outputQuantity)
	assert.Equal(t, relaycommon.MediaFactSourceUpstream, output.Source)

	costs := harnessLedgerStreamCount(t, gatewayRequestID, model.HarnessLedgerStreamCost)
	require.Len(t, costs, 2)
	assert.Equal(t, model.HarnessSettlementStatusPending, costs[0].Status)
	assert.Equal(t, "video_second", costs[0].BillingBasis)
	assert.Empty(t, costs[0].BillingQuantity, "an unknown output has no billable quantity")
	pendingCost := costRecordFromLedgerEvent(costs[0])
	assert.Nil(t, pendingCost.BillingQuantity)

	settledCost := costRecordFromLedgerEvent(costs[1])
	assert.Equal(t, model.HarnessLedgerEventTypeCostCorrection, costs[1].EventType,
		"the settled event learns a billed quantity, so it is a cost correction")
	assert.Equal(t, model.HarnessSettlementStatusSettled, settledCost.Status)
	assert.Equal(t, "video_second", settledCost.BillingBasis)
	require.NotNil(t, settledCost.BillingQuantity)
	assert.Equal(t, "8.5", *settledCost.BillingQuantity)
	assert.Equal(t, relaycommon.MediaFactSourceUpstream, settledCost.BillingQuantitySource)
	assert.Equal(t, "expr-a2-hash", settledCost.BillingExprHash)
	assert.Equal(t, 1, settledCost.BillingExprVersion)
	require.Len(t, settledCost.BillingComponents, 1)
	assert.Equal(t, HarnessBillingComponent{
		Basis:    "video_second",
		Quantity: "8.5",
		Source:   relaycommon.MediaFactSourceUpstream,
	}, settledCost.BillingComponents[0])

	assert.Equal(t, pendingCost.Billing.BaseAmount, settledCost.Billing.BaseAmount,
		"learning the output must not reprice the task")
	assert.Equal(t, pendingCost.Billing.TotalAmount, settledCost.Billing.TotalAmount,
		"learning the output must not reprice the task")
}
