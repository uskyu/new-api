package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openHarnessLedgerDB opens one supported dialect and migrates only the ledger
// table on it, twice, because startup reruns AutoMigrate on every boot. SQLite
// always runs; MySQL and PostgreSQL run when TEST_MYSQL_DSN / TEST_POSTGRES_DSN
// are provided.
func openHarnessLedgerDB(t *testing.T, dialect string) *gorm.DB {
	t.Helper()
	var dialector gorm.Dialector
	switch dialect {
	case "sqlite":
		dialector = sqlite.Open(filepath.Join(t.TempDir(), "harness_ledger.db"))
	case "mysql":
		dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
		if dsn == "" {
			t.Skip("TEST_MYSQL_DSN is not configured")
		}
		dialector = mysql.Open(dsn)
	case "postgres":
		dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
		if dsn == "" {
			t.Skip("TEST_POSTGRES_DSN is not configured")
		}
		dialector = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
	default:
		t.Fatalf("unknown dialect %q", dialect)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	// Create the table before cleaning it: a shared external test database may
	// hold rows from an earlier run.
	require.NoError(t, db.AutoMigrate(&HarnessLedgerEvent{}))
	require.NoError(t, db.Exec("DELETE FROM harness_ledger_events").Error)
	t.Cleanup(func() {
		db.Exec("DELETE FROM harness_ledger_events")
		_ = db.Migrator().DropTable("harness_ledger_events")
	})

	// Re-running the same migration must be a no-op; startup runs it every boot.
	require.NoError(t, db.AutoMigrate(&HarnessLedgerEvent{}))
	return db
}

// useHarnessLedgerDB points the package-level DB at a dialect fixture so the
// exported helpers run against that dialect.
func useHarnessLedgerDB(t *testing.T, dialect string) {
	t.Helper()
	db := openHarnessLedgerDB(t, dialect)
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })
}

func harnessLedgerDialects() []string {
	return []string{"sqlite", "mysql", "postgres"}
}

// appendVisibleHarnessLedgerEvent appends an event already behind the
// visibility waterline, so cursor queries can see it right away.
func appendVisibleHarnessLedgerEvent(t *testing.T, stream, requestId, eventType string, observedAtMs int64) *HarnessLedgerEvent {
	t.Helper()
	event := &HarnessLedgerEvent{
		Stream:           stream,
		EventType:        eventType,
		HarnessRequestId: requestId,
		Status:           HarnessLedgerStatusSettled,
		ObservedAtMs:     time.Now().UnixMilli() - 10000,
	}
	if observedAtMs != 0 {
		event.ObservedAtMs = observedAtMs
	}
	require.NoError(t, AppendHarnessLedgerEvent(event))
	return event
}

func TestHarnessLedgerMigrateIsIdempotent(t *testing.T) {
	for _, dialect := range harnessLedgerDialects() {
		t.Run(dialect, func(t *testing.T) {
			db := openHarnessLedgerDB(t, dialect)
			// openHarnessLedgerDB already migrated twice; a third run must still
			// be a no-op instead of rewriting columns on MySQL/PostgreSQL.
			require.NoError(t, db.AutoMigrate(&HarnessLedgerEvent{}))
			assert.True(t, db.Migrator().HasTable("harness_ledger_events"))
			assert.True(t, db.Migrator().HasIndex("harness_ledger_events", "idx_harness_ledger_event_id"))
			assert.True(t, db.Migrator().HasIndex("harness_ledger_events", "idx_harness_ledger_stream_id"))
			assert.True(t, db.Migrator().HasIndex("harness_ledger_events", "idx_harness_ledger_batch"))
			assert.True(t, db.Migrator().HasIndex("harness_ledger_events", "idx_harness_ledger_observed_at_ms"))
			assert.True(t, db.Migrator().HasIndex("harness_ledger_events", "idx_harness_ledger_harness_request_id"))
		})
	}
}

func TestHarnessLedgerEventIdIsUniqueAndTimeOrdered(t *testing.T) {
	const total = 100000
	seen := make(map[string]struct{}, total)
	for range total {
		id := NewHarnessEventId(HarnessLedgerStreamUsage)
		require.Len(t, id, len("ue_")+26)
		require.True(t, strings.HasPrefix(id, "ue_"))
		_, duplicate := seen[id]
		require.False(t, duplicate, "duplicate event id %q", id)
		seen[id] = struct{}{}
	}

	// The timestamp prefix occupies 48 bits of the 50 leading bits, so ids from
	// different milliseconds sort lexicographically in generation order.
	early := NewHarnessEventId(HarnessLedgerStreamCost)
	time.Sleep(5 * time.Millisecond)
	late := NewHarnessEventId(HarnessLedgerStreamCost)
	assert.Less(t, early, late, "event id ordering must follow the millisecond timestamp")
	assert.True(t, strings.HasPrefix(early, HarnessLedgerStreamCost+"_"))
	// 2026 milliseconds are below 2^45, so the 48-bit timestamp prefix starts
	// with a '0' base32 digit for every id generated in this century.
	assert.Equal(t, byte('0'), early[len(HarnessLedgerStreamCost)+1], "the timestamp prefix must lead with '0'")
}

func TestHarnessLedgerCursorPagination(t *testing.T) {
	for _, dialect := range harnessLedgerDialects() {
		t.Run(dialect, func(t *testing.T) {
			useHarnessLedgerDB(t, dialect)

			base := time.Now().UnixMilli() - 60000
			appended := make([]*HarnessLedgerEvent, 0, 5)
			for i := range 5 {
				appended = append(appended, appendVisibleHarnessLedgerEvent(
					t, HarnessLedgerStreamUsage, "req-page-1", HarnessLedgerEventTypeUsage, base+int64(i),
				))
			}

			page, hasMore, err := QueryHarnessLedgerEvents(HarnessLedgerStreamUsage, 0, 2, 0, 0)
			require.NoError(t, err)
			require.Len(t, page, 2)
			assert.True(t, hasMore)
			assert.Equal(t, appended[0].Id, page[0].Id)
			assert.Equal(t, appended[1].Id, page[1].Id)

			page, hasMore, err = QueryHarnessLedgerEvents(HarnessLedgerStreamUsage, page[1].Id, 2, 0, 0)
			require.NoError(t, err)
			require.Len(t, page, 2)
			assert.True(t, hasMore)
			assert.Equal(t, appended[2].Id, page[0].Id)
			assert.Equal(t, appended[3].Id, page[1].Id)

			// The last page is short and reports no further page.
			page, hasMore, err = QueryHarnessLedgerEvents(HarnessLedgerStreamUsage, page[1].Id, 2, 0, 0)
			require.NoError(t, err)
			require.Len(t, page, 1)
			assert.False(t, hasMore)
			assert.Equal(t, appended[4].Id, page[0].Id)

			// The cursor is per stream: the cost stream has no rows yet.
			costPage, costHasMore, err := QueryHarnessLedgerEvents(HarnessLedgerStreamCost, 0, 2, 0, 0)
			require.NoError(t, err)
			assert.Empty(t, costPage)
			assert.False(t, costHasMore)

			// Rows outside the requested observation window are excluded.
			windowed, _, err := QueryHarnessLedgerEvents(HarnessLedgerStreamUsage, 0, 10, base+int64(1), base+int64(3))
			require.NoError(t, err)
			require.Len(t, windowed, 3)
			assert.Equal(t, appended[1].Id, windowed[0].Id)
			assert.Equal(t, appended[3].Id, windowed[2].Id)
		})
	}
}

func TestHarnessLedgerVisibilityWaterline(t *testing.T) {
	for _, dialect := range harnessLedgerDialects() {
		t.Run(dialect, func(t *testing.T) {
			useHarnessLedgerDB(t, dialect)

			fresh := &HarnessLedgerEvent{
				Stream:           HarnessLedgerStreamUsage,
				EventType:        HarnessLedgerEventTypeUsage,
				HarnessRequestId: "req-fresh-1",
			}
			require.NoError(t, AppendHarnessLedgerEvent(fresh))
			require.NotZero(t, fresh.ObservedAtMs)
			require.NotZero(t, fresh.CreatedAt)
			require.Equal(t, 1, fresh.RecordVersion)
			require.NotEmpty(t, fresh.EventId)

			// A just-committed event is withheld until it is older than the lag,
			// otherwise a concurrent reader could skip an uncommitted id.
			events, hasMore, err := QueryHarnessLedgerEvents(HarnessLedgerStreamUsage, 0, 10, 0, 0)
			require.NoError(t, err)
			assert.Empty(t, events)
			assert.False(t, hasMore)

			// Batch reads are not watermarked: they are point lookups, not cursors.
			byRequest, err := QueryHarnessLedgerEventsByHarnessRequestIds(HarnessLedgerStreamUsage, []string{"req-fresh-1"}, 10)
			require.NoError(t, err)
			require.Len(t, byRequest, 1)
			assert.Equal(t, fresh.EventId, byRequest[0].EventId)

			// Ageing the row past the waterline makes it visible to the cursor.
			require.NoError(t, DB.Model(&HarnessLedgerEvent{}).
				Where("event_id = ?", fresh.EventId).
				Update("observed_at_ms", time.Now().UnixMilli()-5000).Error)

			events, hasMore, err = QueryHarnessLedgerEvents(HarnessLedgerStreamUsage, 0, 10, 0, 0)
			require.NoError(t, err)
			require.Len(t, events, 1)
			assert.False(t, hasMore)
			assert.Equal(t, fresh.EventId, events[0].EventId)
		})
	}
}

func TestHarnessLedgerBatchGet(t *testing.T) {
	for _, dialect := range harnessLedgerDialects() {
		t.Run(dialect, func(t *testing.T) {
			useHarnessLedgerDB(t, dialect)

			base := time.Now().UnixMilli() - 60000
			first := appendVisibleHarnessLedgerEvent(t, HarnessLedgerStreamCost, "req-batch-a", HarnessLedgerEventTypeCost, base)
			second := appendVisibleHarnessLedgerEvent(t, HarnessLedgerStreamCost, "req-batch-b", HarnessLedgerEventTypeCost, base+int64(1))
			third := appendVisibleHarnessLedgerEvent(t, HarnessLedgerStreamCost, "req-batch-a", HarnessLedgerEventTypeCost, base+int64(2))
			// Another stream's events must never leak into this batch.
			appendVisibleHarnessLedgerEvent(t, HarnessLedgerStreamUsage, "req-batch-a", HarnessLedgerEventTypeUsage, base)

			events, err := QueryHarnessLedgerEventsByHarnessRequestIds(
				HarnessLedgerStreamCost, []string{"req-batch-a", "req-batch-b"}, 10,
			)
			require.NoError(t, err)
			require.Len(t, events, 3)
			assert.Equal(t, []string{first.EventId, second.EventId, third.EventId},
				[]string{events[0].EventId, events[1].EventId, events[2].EventId},
				"events must be ordered by observed_at_ms then event_id")

			// limit caps the total row count, not the number of request ids.
			events, err = QueryHarnessLedgerEventsByHarnessRequestIds(
				HarnessLedgerStreamCost, []string{"req-batch-a", "req-batch-b"}, 2,
			)
			require.NoError(t, err)
			require.Len(t, events, 2)
			assert.Equal(t, first.EventId, events[0].EventId)
			assert.Equal(t, second.EventId, events[1].EventId)

			// No ids means no query and no error.
			events, err = QueryHarnessLedgerEventsByHarnessRequestIds(HarnessLedgerStreamCost, nil, 10)
			require.NoError(t, err)
			assert.Empty(t, events)
		})
	}
}

func TestHarnessLedgerCursorRoundTrip(t *testing.T) {
	encoded := EncodeHarnessLedgerCursor(HarnessLedgerStreamCost, 987654321)
	require.NotEmpty(t, encoded)

	stream, id, err := DecodeHarnessLedgerCursor(encoded)
	require.NoError(t, err)
	assert.Equal(t, HarnessLedgerStreamCost, stream)
	assert.Equal(t, int64(987654321), id)

	zero := EncodeHarnessLedgerCursor(HarnessLedgerStreamUsage, 0)
	stream, id, err = DecodeHarnessLedgerCursor(zero)
	require.NoError(t, err)
	assert.Equal(t, HarnessLedgerStreamUsage, stream)
	assert.Equal(t, int64(0), id)

	for _, raw := range []string{
		"",
		"not-base64!!",
		"eyJ2IjoxfQ",                      // {"v":1}, no stream or id
		"eyJ2IjoyLCJzIjoidWUiLCJpZCI6MX0", // version 2, unsupported
	} {
		stream, id, err := DecodeHarnessLedgerCursor(raw)
		assert.ErrorIs(t, err, ErrHarnessLedgerCursorInvalid, "cursor %q must be rejected", raw)
		assert.Empty(t, stream)
		assert.Zero(t, id)
	}
}

func TestHarnessLedgerCorrectionChain(t *testing.T) {
	for _, dialect := range harnessLedgerDialects() {
		t.Run(dialect, func(t *testing.T) {
			useHarnessLedgerDB(t, dialect)

			base := time.Now().UnixMilli() - 60000
			original := &HarnessLedgerEvent{
				Stream:           HarnessLedgerStreamCost,
				EventType:        HarnessLedgerEventTypeCost,
				HarnessRequestId: "req-correct-1",
				Status:           HarnessLedgerStatusSettled,
				Currency:         "USD",
				BaseAmount:       "0.0100000000",
				TotalAmount:      "0.0106000000",
				BillingBasis:     "tokens",
				ObservedAtMs:     base,
			}
			require.NoError(t, AppendHarnessLedgerEvent(original))
			assert.Equal(t, 1, original.RecordVersion)

			correction := &HarnessLedgerEvent{
				Stream:            HarnessLedgerStreamCost,
				EventType:         HarnessLedgerEventTypeCostCorrection,
				HarnessRequestId:  "req-correct-1",
				Status:            HarnessLedgerStatusAdjusted,
				Currency:          "USD",
				BaseAmount:        "0.0080000000",
				TotalAmount:       "0.0084800000",
				BillingBasis:      "tokens",
				RecordVersion:     2,
				SupersedesEventId: &original.EventId,
				ObservedAtMs:      base + int64(1),
			}
			require.NoError(t, AppendHarnessLedgerEvent(correction))
			require.NotEqual(t, original.EventId, correction.EventId)

			events, err := QueryHarnessLedgerEventsByHarnessRequestIds(HarnessLedgerStreamCost, []string{"req-correct-1"}, 10)
			require.NoError(t, err)
			require.Len(t, events, 2, "the superseded event must stay readable")

			assert.Equal(t, original.EventId, events[0].EventId)
			assert.Equal(t, 1, events[0].RecordVersion)
			assert.Equal(t, "0.0106000000", events[0].TotalAmount)

			assert.Equal(t, correction.EventId, events[1].EventId)
			assert.Equal(t, 2, events[1].RecordVersion)
			require.NotNil(t, events[1].SupersedesEventId)
			assert.Equal(t, original.EventId, *events[1].SupersedesEventId)
			assert.Equal(t, "0.0084800000", events[1].TotalAmount)
			assert.Equal(t, HarnessLedgerStatusAdjusted, events[1].Status)

			// The chain is also reachable through the id cursor in append order.
			cursorPage, hasMore, err := QueryHarnessLedgerEvents(HarnessLedgerStreamCost, original.Id, 10, 0, 0)
			require.NoError(t, err)
			require.Len(t, cursorPage, 1)
			assert.False(t, hasMore)
			assert.Equal(t, correction.EventId, cursorPage[0].EventId)

			// Appending never mutates history: the superseded row is unchanged.
			var stored HarnessLedgerEvent
			require.NoError(t, DB.Where("event_id = ?", original.EventId).First(&stored).Error)
			assert.Equal(t, "0.0106000000", stored.TotalAmount)
			assert.Equal(t, 1, stored.RecordVersion)
		})
	}
}

func TestHarnessLedgerAppendRejectsInvalidEvents(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&HarnessLedgerEvent{}))
	t.Cleanup(func() { DB.Exec("DELETE FROM harness_ledger_events") })

	assert.ErrorIs(t, AppendHarnessLedgerEvent(nil), ErrHarnessLedgerInvalid)
	assert.ErrorIs(t, AppendHarnessLedgerEvent(&HarnessLedgerEvent{EventType: HarnessLedgerEventTypeUsage}), ErrHarnessLedgerInvalid)
	assert.ErrorIs(t, AppendHarnessLedgerEvent(&HarnessLedgerEvent{
		Stream:    "xx",
		EventType: HarnessLedgerEventTypeUsage,
	}), ErrHarnessLedgerInvalid)
}
