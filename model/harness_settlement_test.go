package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openHarnessSettlementDB opens a database for one supported dialect and
// migrates only the settlement table on it. SQLite always runs; MySQL and
// PostgreSQL run when the suite provides TEST_MYSQL_DSN / TEST_POSTGRES_DSN.
func openHarnessSettlementDB(t *testing.T, dialect string) *gorm.DB {
	t.Helper()
	var dialector gorm.Dialector
	switch dialect {
	case "sqlite":
		dialector = sqlite.Open(filepath.Join(t.TempDir(), "harness_settlement.db"))
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

	require.NoError(t, db.AutoMigrate(&HarnessSettlement{}))
	// A shared external test database may hold rows from an earlier run.
	require.NoError(t, db.Exec("DELETE FROM harness_settlements").Error)
	t.Cleanup(func() {
		db.Exec("DELETE FROM harness_settlements")
		_ = db.Migrator().DropTable("harness_settlements")
	})
	// Re-running the same migration must be a no-op; startup runs it every boot.
	require.NoError(t, db.AutoMigrate(&HarnessSettlement{}))
	return db
}

// useHarnessSettlementDB points the package-level DB at a dialect fixture so the
// exported persistence helpers run against that dialect.
func useHarnessSettlementDB(t *testing.T, dialect string) {
	t.Helper()
	db := openHarnessSettlementDB(t, dialect)
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })
}

func TestHarnessSettlementPersistenceAcrossDialects(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			useHarnessSettlementDB(t, dialect)

			first := &HarnessSettlement{
				RequestId:        "req-settlement-1",
				TokenId:          7,
				UserId:           11,
				Status:           HarnessSettlementStatusPending,
				ModelName:        "deepseek-v4.1-flash",
				ChannelId:        3,
				PricingGroup:     "harness",
				GroupRatio:       0.8,
				TaxRatePercent:   6,
				PromptTokens:     100,
				CompletionTokens: 40,
				CreatedAt:        1000,
			}
			require.NoError(t, UpsertHarnessSettlement(first))

			settled := &HarnessSettlement{
				RequestId:        "req-settlement-1",
				TaskId:           "task-settlement-1",
				TokenId:          7,
				UserId:           11,
				Status:           HarnessSettlementStatusSettled,
				ModelName:        "deepseek-v4.1-flash",
				ChannelId:        4,
				PricingGroup:     "harness",
				GroupRatio:       0.9,
				TaxRatePercent:   6,
				PromptTokens:     120,
				CompletionTokens: 60,
				CacheReadTokens:  10,
				CacheWriteTokens: 5,
				// The split was not observed and a total was. SplitUnknown is stored
				// inverted precisely so this combination survives GORM: a
				// default:true flag would let the insert drop a genuine false and
				// silently turn "unknown" into "known".
				TotalTokens:      999,
				TotalTokensKnown: true,
				SplitUnknown:     true,
				BaseAmount:       "0.0012345678",
				TotalAmount:      "0.0010470000",
				Currency:         "USD",
				// A fresh zero timestamp must not replace the original created_at.
				CreatedAt: 0,
				SettledAt: 2000,
			}
			require.NoError(t, UpsertHarnessSettlement(settled))

			var rows []HarnessSettlement
			require.NoError(t, DB.Order("id").Find(&rows).Error)
			require.Len(t, rows, 1, "repeated upserts must keep exactly one row per request id")

			row := rows[0]
			assert.Equal(t, HarnessSettlementStatusSettled, row.Status)
			assert.Equal(t, "task-settlement-1", row.TaskId)
			assert.Equal(t, 4, row.ChannelId)
			assert.Equal(t, 120, row.PromptTokens)
			assert.Equal(t, 60, row.CompletionTokens)
			assert.Equal(t, 10, row.CacheReadTokens)
			assert.Equal(t, 5, row.CacheWriteTokens)
			assert.Equal(t, 999, row.TotalTokens)
			assert.True(t, row.TotalTokensKnown)
			assert.True(t, row.SplitUnknown,
				"the inverted flag must round-trip as set, on every dialect")
			assert.Equal(t, "0.0012345678", row.BaseAmount)
			assert.Equal(t, "0.0010470000", row.TotalAmount)
			assert.Equal(t, "USD", row.Currency)
			assert.Equal(t, 0.9, row.GroupRatio)
			assert.Equal(t, int64(1000), row.CreatedAt, "created_at must come from the original insert")
			assert.Equal(t, int64(2000), row.SettledAt)

			// A later transition to refunded keeps the row and its created_at.
			refunded := &HarnessSettlement{
				RequestId:   "req-settlement-1",
				TaskId:      "task-settlement-1",
				TokenId:     7,
				Status:      HarnessSettlementStatusRefunded,
				TotalAmount: "0.0000000000",
				SettledAt:   3000,
			}
			require.NoError(t, UpsertHarnessSettlement(refunded))

			require.NoError(t, DB.Where("request_id = ?", "req-settlement-1").Find(&rows).Error)
			require.Len(t, rows, 1)
			assert.Equal(t, HarnessSettlementStatusRefunded, rows[0].Status)
			assert.Equal(t, "0.0000000000", rows[0].TotalAmount)
			assert.Equal(t, int64(1000), rows[0].CreatedAt)
			assert.Equal(t, int64(3000), rows[0].SettledAt)

			// Token-scoped reads must not reveal another token's request.
			found, err := GetHarnessSettlementForToken("req-settlement-1", 7)
			require.NoError(t, err)
			require.NotNil(t, found)
			assert.Equal(t, "task-settlement-1", found.TaskId)

			otherToken, err := GetHarnessSettlementForToken("req-settlement-1", 8)
			require.NoError(t, err)
			assert.Nil(t, otherToken)

			unknown, err := GetHarnessSettlementForToken("req-unknown", 7)
			require.NoError(t, err)
			assert.Nil(t, unknown)

			// Task polling resolves the row created at submit time.
			byTask, err := GetHarnessSettlementByTaskId("task-settlement-1")
			require.NoError(t, err)
			require.NotNil(t, byTask)
			assert.Equal(t, "req-settlement-1", byTask.RequestId)
			assert.Equal(t, HarnessSettlementStatusRefunded, byTask.Status)

			missingTask, err := GetHarnessSettlementByTaskId("task-unknown")
			require.NoError(t, err)
			assert.Nil(t, missingTask)

			// The request_id unique index is the conflict target of the upsert.
			assert.Error(t, DB.Create(&HarnessSettlement{RequestId: "req-settlement-1"}).Error)
		})
	}
}

func TestHarnessSettlementRejectsRowsWithoutRequestId(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&HarnessSettlement{}))
	t.Cleanup(func() { DB.Exec("DELETE FROM harness_settlements") })

	// An empty request id would collide with every other id-less row in the
	// unique index, so the upsert refuses it instead of merging ledgers.
	assert.ErrorIs(t, UpsertHarnessSettlement(nil), ErrHarnessSettlementInvalid)
	assert.ErrorIs(t, UpsertHarnessSettlement(&HarnessSettlement{Status: HarnessSettlementStatusPending}), ErrHarnessSettlementInvalid)

	require.NoError(t, UpsertHarnessSettlement(&HarnessSettlement{
		RequestId: "req-valid-id",
		Status:    HarnessSettlementStatusPending,
	}))

	var stored HarnessSettlement
	require.NoError(t, DB.Where("request_id = ?", "req-valid-id").First(&stored).Error)
	assert.NotZero(t, stored.CreatedAt, "the upsert stamps created_at when the caller leaves it zero")
}

// TestKeepFirstNonEmptyRendersPerDialect pins the correlation-id guard used by the
// settlement upsert. MySQL and PostgreSQL reference the proposed row differently,
// and a wrong reference is not a subtle bug: MySQL answers "Unknown column" and
// every settlement write fails.
func TestKeepFirstNonEmptyRendersPerDialect(t *testing.T) {
	assert.Equal(t,
		"CASE WHEN excluded.harness_request_id <> '' THEN excluded.harness_request_id ELSE harness_request_id END",
		keepFirstNonEmpty("harness_request_id", "excluded"),
		"PostgreSQL and SQLite reference the proposed row as excluded")
	assert.Equal(t,
		"CASE WHEN VALUES(harness_run_id) <> '' THEN VALUES(harness_run_id) ELSE harness_run_id END",
		keepFirstNonEmpty("harness_run_id", "VALUES"),
		"MySQL references the proposed row through VALUES(col)")
}

// TestListHarnessSettlementsFiltersAndPages pins the operator list query: the
// filters must actually narrow the result and the total must describe the filtered
// set rather than the whole table, because the page and its total are read
// together.
func TestListHarnessSettlementsFiltersAndPages(t *testing.T) {
	useHarnessSettlementDB(t, "sqlite")

	seed := func(id string, status string, createdAt int64, modelName string) {
		require.NoError(t, DB.Create(&HarnessSettlement{
			RequestId:   id,
			Status:      status,
			ModelName:   modelName,
			BaseAmount:  "1",
			TotalAmount: "1",
			Currency:    "USD",
			CreatedAt:   createdAt,
			// The unique index on harness_request_id admits one empty value, so each
			// row carries its own to keep them distinct.
			HarnessRequestId: "hrq_" + id,
		}).Error)
	}
	seed("gw-a", HarnessSettlementStatusSettled, 1000, "economy")
	seed("gw-b", HarnessSettlementStatusPending, 2000, "enterprise")
	seed("gw-c", HarnessSettlementStatusSettled, 3000, "economy")
	seed("other-x", HarnessSettlementStatusRefunded, 4000, "economy")

	all, total, err := ListHarnessSettlements(HarnessSettlementListQuery{})
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	assert.Len(t, all, 4)
	assert.Equal(t, "other-x", all[0].RequestId, "newest first")

	// Request id prefix narrows the result.
	byPrefix, total, err := ListHarnessSettlements(HarnessSettlementListQuery{RequestId: "gw-"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, byPrefix, 3)

	// Status filter narrows the result.
	byStatus, total, err := ListHarnessSettlements(HarnessSettlementListQuery{
		Statuses: []string{HarnessSettlementStatusSettled},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, byStatus, 2)

	// Time range is inclusive of the start and exclusive of the end.
	byTime, total, err := ListHarnessSettlements(HarnessSettlementListQuery{StartTime: 2000, EndTime: 4000})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total, "created_at in [2000, 4000)")
	assert.Len(t, byTime, 2)

	// Paging keeps the filtered total while returning one page.
	page2, total, err := ListHarnessSettlements(HarnessSettlementListQuery{Page: 2, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(4), total, "the total describes the filtered set, not the page")
	assert.Len(t, page2, 2)

	// A search term containing a LIKE wildcard must not widen the match.
	wildcard, total, err := ListHarnessSettlements(HarnessSettlementListQuery{RequestId: "g%"})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total, "a literal % must not match as a wildcard")
	assert.Empty(t, wildcard)
}
