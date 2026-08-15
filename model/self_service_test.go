package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFindSelfServiceEmptyOutputLogsUsesZeroToFiveRange(t *testing.T) {
	dsn := fmt.Sprintf("file:self-service-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))

	oldDB, oldLogDB := DB, LOG_DB
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})

	rows := []Log{
		{UserId: 7, CreatedAt: 100, Type: LogTypeConsume, ModelName: "negative", Quota: 10, CompletionTokens: -1},
		{UserId: 7, CreatedAt: 101, Type: LogTypeConsume, ModelName: "zero", Quota: 10, CompletionTokens: 0},
		{UserId: 7, CreatedAt: 102, Type: LogTypeConsume, ModelName: "one", Quota: 10, CompletionTokens: 1},
		{UserId: 7, CreatedAt: 103, Type: LogTypeConsume, ModelName: "five", Quota: 10, CompletionTokens: 5},
		{UserId: 7, CreatedAt: 104, Type: LogTypeConsume, ModelName: "six", Quota: 10, CompletionTokens: 6},
		{UserId: 7, CreatedAt: 105, Type: LogTypeConsume, ModelName: "free", Quota: 0, CompletionTokens: 0},
		{UserId: 8, CreatedAt: 106, Type: LogTypeConsume, ModelName: "other-user", Quota: 10, CompletionTokens: 0},
		{UserId: 7, CreatedAt: 107, Type: LogTypeError, ModelName: "error", Quota: 10, CompletionTokens: 0},
		{UserId: 7, CreatedAt: 108, Type: LogTypeConsume, ModelName: "DALL-E-preview", Quota: 10, CompletionTokens: 0},
	}
	require.NoError(t, LOG_DB.Create(&rows).Error)

	logs, total, err := FindSelfServiceEmptyOutputLogs(7, 100, 108, 10, []string{"dall-e"})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, logs, 3)
	completionTokens := make(map[int]bool, len(logs))
	for _, logItem := range logs {
		completionTokens[logItem.CompletionTokens] = true
	}
	require.Equal(t, map[int]bool{0: true, 1: true, 5: true}, completionTokens)

	limited, limitedTotal, err := FindSelfServiceEmptyOutputLogs(7, 100, 108, 2, nil)
	require.NoError(t, err)
	require.Equal(t, 4, limitedTotal)
	require.Len(t, limited, 2)
	for _, logItem := range limited {
		require.Contains(t, []int{0, 1, 5}, logItem.CompletionTokens)
	}
}
