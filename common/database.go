package common

type DatabaseType string

const (
	DatabaseTypeMySQL      DatabaseType = "mysql"
	DatabaseTypeSQLite     DatabaseType = "sqlite"
	DatabaseTypePostgreSQL DatabaseType = "postgres"
	DatabaseTypeClickHouse DatabaseType = "clickhouse"
)

var mainDatabaseType = DatabaseTypeSQLite
var logDatabaseType = DatabaseTypeSQLite

// Legacy boolean flags kept for backward compatibility with older migrations.
var UsingSQLite = false
var UsingPostgreSQL = false
var LogSqlType = DatabaseTypeSQLite
var UsingMySQL = false
var UsingClickHouse = false

var SQLitePath = "one-api.db?_busy_timeout=30000"

func MainDatabaseType() DatabaseType {
	if UsingMySQL {
		return DatabaseTypeMySQL
	}
	if UsingPostgreSQL {
		return DatabaseTypePostgreSQL
	}
	if UsingSQLite {
		return DatabaseTypeSQLite
	}
	return mainDatabaseType
}

func LogDatabaseType() DatabaseType {
	return logDatabaseType
}

func SetMainDatabaseType(databaseType DatabaseType) {
	mainDatabaseType = databaseType
	UsingMySQL = databaseType == DatabaseTypeMySQL
	UsingPostgreSQL = databaseType == DatabaseTypePostgreSQL
	UsingSQLite = databaseType == DatabaseTypeSQLite
}

func SetLogDatabaseType(databaseType DatabaseType) {
	logDatabaseType = databaseType
	LogSqlType = databaseType
}

func SetDatabaseTypes(mainType DatabaseType, logType DatabaseType) {
	SetMainDatabaseType(mainType)
	SetLogDatabaseType(logType)
}

func UsingMainDatabase(databaseType DatabaseType) bool {
	return MainDatabaseType() == databaseType
}

func UsingLogDatabase(databaseType DatabaseType) bool {
	return LogDatabaseType() == databaseType
}
