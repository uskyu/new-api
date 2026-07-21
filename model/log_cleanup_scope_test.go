package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogCleanupOnlyDeletesOldConsumeAndErrorLogs(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)

	cutoff := int64(1_700_000_000)
	logs := []Log{
		{CreatedAt: cutoff - 300, Type: LogTypeConsume, Content: "old consume"},
		{CreatedAt: cutoff - 200, Type: LogTypeError, Content: "old error"},
		{CreatedAt: cutoff + 100, Type: LogTypeConsume, Content: "new consume"},
		{CreatedAt: cutoff + 200, Type: LogTypeError, Content: "new error"},
		{CreatedAt: cutoff - 100, Type: LogTypeTopup, Content: "old topup"},
		{CreatedAt: cutoff - 100, Type: LogTypeManage, Content: "old manage"},
		{CreatedAt: cutoff - 100, Type: LogTypeSystem, Content: "old system"},
		{CreatedAt: cutoff - 100, Type: LogTypeRefund, Content: "old refund"},
		{CreatedAt: cutoff - 100, Type: LogTypeLogin, Content: "old login"},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	total, err := CountOldLog(context.Background(), cutoff)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)

	deleted, err := DeleteOldLogBatch(context.Background(), cutoff, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
	deleted, err = DeleteOldLogBatch(context.Background(), cutoff, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
	deleted, err = DeleteOldLogBatch(context.Background(), cutoff, 10)
	require.NoError(t, err)
	assert.Zero(t, deleted)

	var remaining []Log
	require.NoError(t, LOG_DB.Order("created_at asc, id asc").Find(&remaining).Error)
	contents := make([]string, 0, len(remaining))
	for _, entry := range remaining {
		contents = append(contents, entry.Content)
	}
	assert.ElementsMatch(t, []string{
		"new consume",
		"new error",
		"old topup",
		"old manage",
		"old system",
		"old refund",
		"old login",
	}, contents)
}
