package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeleteOldUsageLogsOnlyDeletesExpiredUsageLogs(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)

	cutoff := int64(1700000000)
	logs := []*Log{
		{UserId: 1, CreatedAt: cutoff - 300, Type: LogTypeConsume, Content: "old consume 1"},
		{UserId: 1, CreatedAt: cutoff - 200, Type: LogTypeConsume, Content: "old consume 2"},
		{UserId: 1, CreatedAt: cutoff - 100, Type: LogTypeError, Content: "old error"},
		{UserId: 1, CreatedAt: cutoff + 100, Type: LogTypeConsume, Content: "new consume"},
		{UserId: 1, CreatedAt: cutoff + 200, Type: LogTypeError, Content: "new error"},
		{UserId: 1, CreatedAt: cutoff - 100, Type: LogTypeTopup, Content: "old topup"},
		{UserId: 1, CreatedAt: cutoff - 100, Type: LogTypeManage, Content: "old manage"},
		{UserId: 1, CreatedAt: cutoff - 100, Type: LogTypeSystem, Content: "old system"},
		{UserId: 1, CreatedAt: cutoff - 100, Type: LogTypeRefund, Content: "old refund"},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	result, err := DeleteOldUsageLogs(context.Background(), cutoff, 2)
	require.NoError(t, err)
	require.Equal(t, int64(3), result.DeletedCount)
	require.Equal(t, cutoff, result.Cutoff)
	require.Equal(t, 2, result.BatchSize)
	require.Equal(t, 2, result.Batches)

	var remaining []Log
	require.NoError(t, LOG_DB.Order("created_at asc, id asc").Find(&remaining).Error)
	require.Len(t, remaining, 6)
	remainingContents := make([]string, 0, len(remaining))
	for _, log := range remaining {
		remainingContents = append(remainingContents, log.Content)
	}
	require.ElementsMatch(t, []string{
		"new consume",
		"new error",
		"old topup",
		"old manage",
		"old system",
		"old refund",
	}, remainingContents)
}
