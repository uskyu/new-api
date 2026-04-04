package dbx

import "github.com/QuantumNous/new-api/common"

type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectMySQL    Dialect = "mysql"
	DialectPostgres Dialect = "postgres"
	DialectUnknown  Dialect = "unknown"
)

func DetectDialect() Dialect {
	switch {
	case common.UsingMySQL:
		return DialectMySQL
	case common.UsingPostgreSQL:
		return DialectPostgres
	case common.UsingSQLite:
		return DialectSQLite
	default:
		return DialectUnknown
	}
}

func (d Dialect) String() string {
	return string(d)
}

func IsMySQL() bool {
	return DetectDialect() == DialectMySQL
}

func IsPostgres() bool {
	return DetectDialect() == DialectPostgres
}

func IsSQLite() bool {
	return DetectDialect() == DialectSQLite
}
