package model

import (
	"fmt"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type TestDBDialect string

const (
	TestDBDialectSQLite   TestDBDialect = "sqlite"
	TestDBDialectMySQL    TestDBDialect = "mysql"
	TestDBDialectPostgres TestDBDialect = "postgres"
)

func setupAgentTestDB(t *testing.T, dialect TestDBDialect) *gorm.DB {
	t.Helper()

	prevSQLite := common.UsingSQLite
	prevMySQL := common.UsingMySQL
	prevPostgres := common.UsingPostgreSQL
	t.Cleanup(func() {
		common.UsingSQLite = prevSQLite
		common.UsingMySQL = prevMySQL
		common.UsingPostgreSQL = prevPostgres
	})

	db := openTestDB(t, dialect)
	DB = db
	LOG_DB = db
	ensureAgentTestTables(t)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = nil
		LOG_DB = nil
	})
	return db
}

func openTestDB(t *testing.T, dialect TestDBDialect) *gorm.DB {
	t.Helper()
	var (
		db  *gorm.DB
		err error
	)
	switch dialect {
	case TestDBDialectSQLite:
		common.UsingSQLite = true
		common.UsingMySQL = false
		common.UsingPostgreSQL = false
		db, err = gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	case TestDBDialectMySQL:
		common.UsingSQLite = false
		common.UsingMySQL = true
		common.UsingPostgreSQL = false
		dsn := os.Getenv("TEST_MYSQL_DSN")
		if dsn == "" {
			t.Fatalf("TEST_MYSQL_DSN is not set")
		}
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case TestDBDialectPostgres:
		common.UsingSQLite = false
		common.UsingMySQL = false
		common.UsingPostgreSQL = true
		t.Fatalf("PostgreSQL test dialect is not yet implemented")
	default:
		t.Fatalf("unsupported test dialect %s", dialect)
	}
	require.NoError(t, err)
	return db
}
