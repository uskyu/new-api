package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newLedgerTxTestEvent builds one minimal usage event for the transaction tests.
func newLedgerTxTestEvent(requestId string) *HarnessLedgerEvent {
	return &HarnessLedgerEvent{
		Stream:           HarnessLedgerStreamUsage,
		EventType:        HarnessLedgerEventTypeUsage,
		HarnessRequestId: "hrq_" + requestId,
		GatewayRequestId: requestId,
		Status:           HarnessLedgerStatusSettled,
		UsageUnit:        "token",
		BillingBasis:     "token",
		Currency:         "USD",
		RecordVersion:    1,
	}
}

// newLedgerTxTestSettlement builds the matching snapshot row.
func newLedgerTxTestSettlement(requestId string) *HarnessSettlement {
	return &HarnessSettlement{
		RequestId:        requestId,
		Status:           HarnessSettlementStatusSettled,
		ModelName:        "economy",
		Currency:         "USD",
		BaseAmount:       "0.01",
		TotalAmount:      "0.01",
		HarnessRequestId: "hrq_" + requestId,
	}
}

// countLedgerRows reports how many events and snapshots exist for one gateway
// request, which is what the rollback assertions read.
func countLedgerRows(t *testing.T, requestId string) (int64, int64) {
	t.Helper()
	var events, snapshots int64
	require.NoError(t, DB.Model(&HarnessLedgerEvent{}).Where("gateway_request_id = ?", requestId).Count(&events).Error)
	require.NoError(t, DB.Model(&HarnessSettlement{}).Where("request_id = ?", requestId).Count(&snapshots).Error)
	return events, snapshots
}

// failSnapshotUpsert installs a callback that makes the settlement write fail
// while leaving event inserts untouched, which is exactly the partial-failure
// shape a separate-transaction implementation could not recover from.
func failSnapshotUpsert(t *testing.T) func() {
	t.Helper()
	require.NoError(t, DB.Callback().Create().Before("gorm:create").Register("ledger:fail-snapshot", func(tx *gorm.DB) {
		if tx.Statement.Table == "harness_settlements" {
			tx.AddError(errors.New("snapshot write unavailable"))
		}
	}))
	return func() { DB.Callback().Create().Remove("ledger:fail-snapshot") }
}

// TestLedgerTransactionRollsBackEventsWhenSnapshotFails is the fault-injection
// guarantee: the events and the snapshot must commit together. If the snapshot
// write fails, the events must not survive, otherwise the platform's feed would
// report a request the operator's ledger never recorded and a retry would collide
// with the already-inserted event ids.
func TestLedgerTransactionRollsBackEventsWhenSnapshotFails(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&HarnessSettlement{}, &HarnessLedgerEvent{}))
	requestId := "gw-tx-rollback"
	t.Cleanup(func() {
		DB.Exec("DELETE FROM harness_ledger_events WHERE gateway_request_id = ?", requestId)
		DB.Exec("DELETE FROM harness_settlements WHERE request_id = ?", requestId)
	})

	removeFailure := failSnapshotUpsert(t)
	err := AppendHarnessLedgerEventsWithSettlement(
		[]*HarnessLedgerEvent{newLedgerTxTestEvent(requestId)},
		newLedgerTxTestSettlement(requestId),
	)
	removeFailure()

	require.Error(t, err, "the snapshot failure must surface")
	events, snapshots := countLedgerRows(t, requestId)
	assert.Zero(t, events, "events must be rolled back when the snapshot write fails")
	assert.Zero(t, snapshots, "the snapshot itself must not persist")

	// The retry now starts from a clean slate, which is the whole point of sharing
	// one transaction.
	require.NoError(t, AppendHarnessLedgerEventsWithSettlement(
		[]*HarnessLedgerEvent{newLedgerTxTestEvent(requestId)},
		newLedgerTxTestSettlement(requestId),
	))
	events, snapshots = countLedgerRows(t, requestId)
	assert.Equal(t, int64(1), events)
	assert.Equal(t, int64(1), snapshots)
}

// TestLedgerTransactionWritesBothRowsOnSuccess is the control case: the happy
// path must leave exactly one of each, so the rollback test above cannot pass for
// the wrong reason (both writes silently skipped).
func TestLedgerTransactionWritesBothRowsOnSuccess(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&HarnessSettlement{}, &HarnessLedgerEvent{}))
	requestId := "gw-tx-success"
	t.Cleanup(func() {
		DB.Exec("DELETE FROM harness_ledger_events WHERE gateway_request_id = ?", requestId)
		DB.Exec("DELETE FROM harness_settlements WHERE request_id = ?", requestId)
	})

	require.NoError(t, AppendHarnessLedgerEventsWithSettlement(
		[]*HarnessLedgerEvent{newLedgerTxTestEvent(requestId)},
		newLedgerTxTestSettlement(requestId),
	))

	events, snapshots := countLedgerRows(t, requestId)
	assert.Equal(t, int64(1), events)
	assert.Equal(t, int64(1), snapshots)

	var event HarnessLedgerEvent
	require.NoError(t, DB.Where("gateway_request_id = ?", requestId).First(&event).Error)
	assert.NotEmpty(t, event.EventId, "the transaction must still apply identifier defaults")
	assert.Equal(t, 1, event.RecordVersion)

	var snapshot HarnessSettlement
	require.NoError(t, DB.Where("request_id = ?", requestId).First(&snapshot).Error)
	assert.NotZero(t, snapshot.CreatedAt, "the transaction must still apply the created_at default")
}

// TestLedgerSettlementKeepsCorrelationOnResettle guards the upsert column list: an
// async task re-settles an existing row, and losing harness_request_id there would
// silently detach the row from the platform ledger, so a later correction could no
// longer be attributed.
func TestLedgerSettlementKeepsCorrelationOnResettle(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&HarnessSettlement{}))
	requestId := "gw-tx-resettle"
	t.Cleanup(func() { DB.Exec("DELETE FROM harness_settlements WHERE request_id = ?", requestId) })

	first := newLedgerTxTestSettlement(requestId)
	first.Status = HarnessSettlementStatusPending
	require.NoError(t, UpsertHarnessSettlement(first))

	// Re-settle with a status change, as an async task reaching its final state.
	second := newLedgerTxTestSettlement(requestId)
	second.Status = HarnessSettlementStatusRefunded
	second.TotalAmount = "0"
	require.NoError(t, UpsertHarnessSettlement(second))

	var stored HarnessSettlement
	require.NoError(t, DB.Where("request_id = ?", requestId).First(&stored).Error)
	assert.Equal(t, HarnessSettlementStatusRefunded, stored.Status)
	assert.Equal(t, "hrq_"+requestId, stored.HarnessRequestId, "the platform correlation must survive a re-settle")
	assert.NotZero(t, stored.CreatedAt, "the first insert's created_at must be kept")
}

// TestLedgerSettlementNeverClearsCorrelation is the case the plain column
// assignment in the upsert would have broken: a re-settle that carries no
// correlation must not blank the one already recorded. A refund or a task settled
// from a path without the request context would otherwise detach the row from the
// platform ledger it was already reported on.
func TestLedgerSettlementNeverClearsCorrelation(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&HarnessSettlement{}))
	requestId := "gw-tx-clear-correlation"
	t.Cleanup(func() { DB.Exec("DELETE FROM harness_settlements WHERE request_id = ?", requestId) })

	withCorrelation := newLedgerTxTestSettlement(requestId)
	withCorrelation.HarnessRunId = "run_" + requestId
	require.NoError(t, UpsertHarnessSettlement(withCorrelation))

	// A later settle that lost the context: status changes, correlation is empty.
	empty := &HarnessSettlement{
		RequestId:   requestId,
		Status:      HarnessSettlementStatusRefunded,
		Currency:    "USD",
		BaseAmount:  "0.01",
		TotalAmount: "0",
	}
	require.NoError(t, UpsertHarnessSettlement(empty))

	var stored HarnessSettlement
	require.NoError(t, DB.Where("request_id = ?", requestId).First(&stored).Error)
	assert.Equal(t, HarnessSettlementStatusRefunded, stored.Status, "the status must still update")
	assert.Equal(t, "hrq_"+requestId, stored.HarnessRequestId,
		"an empty correlation must not erase the recorded one")
	assert.Equal(t, "run_"+requestId, stored.HarnessRunId,
		"an empty run id must not erase the recorded one")
}

// TestLedgerSettlementUpdatesCorrelationWhenProvided is the counterpart: a later
// settle that does carry a correlation must be able to correct it, so the guard
// above does not freeze a wrong value forever.
func TestLedgerSettlementUpdatesCorrelationWhenProvided(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&HarnessSettlement{}))
	requestId := "gw-tx-update-correlation"
	t.Cleanup(func() { DB.Exec("DELETE FROM harness_settlements WHERE request_id = ?", requestId) })

	require.NoError(t, UpsertHarnessSettlement(newLedgerTxTestSettlement(requestId)))

	corrected := newLedgerTxTestSettlement(requestId)
	corrected.HarnessRequestId = "hrq_corrected"
	corrected.HarnessRunId = "run_corrected"
	require.NoError(t, UpsertHarnessSettlement(corrected))

	var stored HarnessSettlement
	require.NoError(t, DB.Where("request_id = ?", requestId).First(&stored).Error)
	assert.Equal(t, "hrq_corrected", stored.HarnessRequestId)
	assert.Equal(t, "run_corrected", stored.HarnessRunId)
	_ = common.GetTimestamp
}
