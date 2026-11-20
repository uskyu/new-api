package common

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLitePathUsesDriverEffectiveConcurrencyParams(t *testing.T) {
	parsed, err := url.Parse(SQLitePath)
	require.NoError(t, err)
	query := parsed.Query()

	// glebarez/sqlite (modernc-based) only understands `_pragma`; the legacy
	// `_busy_timeout=` spelling is silently ignored, so assert we do not use it.
	assert.Empty(t, query.Get("_busy_timeout"))
	assert.Equal(t, []string{
		"busy_timeout(30000)",
		"journal_mode(WAL)",
	}, query["_pragma"])
	assert.Equal(t, "immediate", query.Get("_txlock"))
}

func sqliteTestDSN(t *testing.T) string {
	t.Helper()
	idx := strings.Index(SQLitePath, "?")
	require.NotEqual(t, -1, idx, "SQLitePath must carry concurrency parameters")
	return filepath.ToSlash(filepath.Join(t.TempDir(), "concurrency.db")) + SQLitePath[idx:]
}

func TestSQLiteConcurrencyParamsTakeEffect(t *testing.T) {
	db, err := sql.Open("sqlite", sqliteTestDSN(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)

	var journalMode string
	require.NoError(t, db.QueryRow("PRAGMA journal_mode").Scan(&journalMode))
	assert.Equal(t, "wal", strings.ToLower(journalMode))

	var busyTimeout int
	require.NoError(t, db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout))
	assert.Equal(t, 30000, busyTimeout)
}

func TestSQLiteTxLockImmediateHoldsWriteLockAtBegin(t *testing.T) {
	dsn := sqliteTestDSN(t)
	holder, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = holder.Close() })
	holder.SetMaxOpenConns(1)
	_, err = holder.Exec("CREATE TABLE txlock_probe (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)

	// With _txlock=immediate the write lock is taken by BEGIN itself, before
	// any statement runs inside the transaction.
	tx, err := holder.Begin()
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	// Queue for only 1ms so a deferred BEGIN (which would leave writers free
	// until the first write) is observably different from an immediate one.
	contender, err := sql.Open("sqlite", dsn+"&_pragma=busy_timeout(1)")
	require.NoError(t, err)
	t.Cleanup(func() { _ = contender.Close() })
	contender.SetMaxOpenConns(1)

	_, err = contender.Exec("INSERT INTO txlock_probe DEFAULT VALUES")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locked")
}
