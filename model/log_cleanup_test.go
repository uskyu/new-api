package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeleteOldConsumeLogsOnlyDeletesExpiredConsumeLogs(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)

	cutoff := int64(1700000000)
	logs := []*Log{
		{UserId: 1, CreatedAt: cutoff - 300, Type: LogTypeConsume, Content: "old consume 1"},
		{UserId: 1, CreatedAt: cutoff - 200, Type: LogTypeConsume, Content: "old consume 2"},
		{UserId: 1, CreatedAt: cutoff - 100, Type: LogTypeConsume, Content: "old consume 3"},
		{UserId: 1, CreatedAt: cutoff + 100, Type: LogTypeConsume, Content: "new consume"},
		{UserId: 1, CreatedAt: cutoff - 100, Type: LogTypeManage, Content: "old manage"},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	result, err := DeleteOldConsumeLogs(context.Background(), cutoff, 2)
	require.NoError(t, err)
	require.Equal(t, int64(3), result.DeletedCount)
	require.Equal(t, cutoff, result.Cutoff)
	require.Equal(t, 2, result.BatchSize)
	require.Equal(t, 2, result.Batches)

	var remaining []Log
	require.NoError(t, LOG_DB.Order("created_at asc, id asc").Find(&remaining).Error)
	require.Len(t, remaining, 2)
	require.Equal(t, LogTypeManage, remaining[0].Type)
	require.Equal(t, "old manage", remaining[0].Content)
	require.Equal(t, LogTypeConsume, remaining[1].Type)
	require.Equal(t, "new consume", remaining[1].Content)
}
