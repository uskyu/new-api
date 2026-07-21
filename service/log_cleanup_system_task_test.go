package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogCleanupSystemTaskPersistsProgressAndPreservesAuditLogs(t *testing.T) {
	truncate(t)
	require.NoError(t, model.LOG_DB.Exec("DELETE FROM logs").Error)

	cutoff := common.GetTimestamp() - 24*60*60
	logs := []model.Log{
		{CreatedAt: cutoff - 30, Type: model.LogTypeConsume, Content: "old consume"},
		{CreatedAt: cutoff - 20, Type: model.LogTypeError, Content: "old error"},
		{CreatedAt: cutoff - 10, Type: model.LogTypeTopup, Content: "old topup"},
		{CreatedAt: cutoff - 10, Type: model.LogTypeManage, Content: "old manage"},
	}
	require.NoError(t, model.LOG_DB.Create(&logs).Error)

	task, err := model.CreateSystemTask(model.SystemTaskTypeLogCleanup, LogCleanupPayload{
		TargetTimestamp: cutoff,
		BatchSize:       1,
	}, LogCleanupState{})
	require.NoError(t, err)

	const runnerID = "cleanup-test-runner"
	claimedTask, claimed, err := model.ClaimSystemTask(task.ID, task.Type, runnerID, common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	runLogCleanupTask(context.Background(), claimedTask, runnerID)

	finished, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, finished)
	assert.Equal(t, model.SystemTaskStatusSucceeded, finished.Status)
	assert.Nil(t, finished.ActiveKey)

	state := LogCleanupState{}
	require.NoError(t, finished.DecodeState(&state))
	assert.EqualValues(t, 2, state.Total)
	assert.EqualValues(t, 2, state.Processed)
	assert.Zero(t, state.Remaining)
	assert.Equal(t, 100, state.Progress)

	result := LogCleanupResult{}
	require.NoError(t, common.UnmarshalJsonStr(finished.Result, &result))
	assert.EqualValues(t, 2, result.DeletedCount)

	var remaining []model.Log
	require.NoError(t, model.LOG_DB.Order("id asc").Find(&remaining).Error)
	require.Len(t, remaining, 2)
	assert.Equal(t, "old topup", remaining[0].Content)
	assert.Equal(t, "old manage", remaining[1].Content)
}

func TestStartLogCleanupTaskRejectsNonPastCutoff(t *testing.T) {
	truncate(t)

	_, err := StartLogCleanupTask(common.GetTimestamp() + 60)
	require.ErrorContains(t, err, "must be in the past")
}
