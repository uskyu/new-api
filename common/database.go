package common

type DatabaseType string

const (
	DatabaseTypeMySQL      DatabaseType = "mysql"
	DatabaseTypeSQLite     DatabaseType = "sqlite"
	DatabaseTypePostgreSQL DatabaseType = "postgres"
)

// Legacy boolean flags — kept for backward compatibility.
var UsingSQLite = false
var UsingPostgreSQL = false
var LogSqlType = DatabaseTypeSQLite // Default to SQLite for logging SQL queries
var UsingMySQL = false
var UsingClickHouse = false

var SQLitePath = "one-api.db?_busy_timeout=30000"

// UsingMainDatabase reports whether the main (business) database is the given type.
// Prefer this over the legacy boolean flags; it is safe to call before InitDB.
func UsingMainDatabase(databaseType DatabaseType) bool {
	return UsingSQLite && databaseType == DatabaseTypeSQLite ||
		UsingPostgreSQL && databaseType == DatabaseTypePostgreSQL ||
		UsingMySQL && databaseType == DatabaseTypeMySQL
}
