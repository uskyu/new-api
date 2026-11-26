package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUpstreamSyncControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	model.InitColumnNames()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	previousDB := model.DB
	previousLOGDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db

	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()

	previousEnabled := setting.UpstreamSyncEnabled
	previousInterval := setting.UpstreamSyncIntervalMinutes

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLOGDB
		setting.UpstreamSyncEnabled = previousEnabled
		setting.UpstreamSyncIntervalMinutes = previousInterval
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
		_ = sqlDB.Close()
	})

	require.NoError(t, db.AutoMigrate(
		&model.Option{},
		&model.UpstreamSyncLog{},
		&model.SystemTask{},
		&model.SystemTaskLock{},
	))
	setting.UpstreamSyncEnabled = false
	setting.UpstreamSyncIntervalMinutes = setting.DefaultUpstreamSyncIntervalMinutes
	return db
}

func newUpstreamSyncLog(t *testing.T, db *gorm.DB, startedAt int64, status string) *model.UpstreamSyncLog {
	t.Helper()
	added, err := common.Marshal([]string{"added-model"})
	require.NoError(t, err)
	removed, err := common.Marshal([]string{"removed-model"})
	require.NoError(t, err)
	priceChanges, err := common.Marshal([]model.UpstreamSyncPriceChange{{
		ModelName: "price-model",
		Fields:    []string{"ModelRatio"},
		Before:    map[string]any{"ModelRatio": 1.0},
		After:     map[string]any{"ModelRatio": 2.0},
	}})
	require.NoError(t, err)
	issues, err := common.Marshal([]model.UpstreamSyncIssue{{
		Level:   model.UpstreamSyncIssueLevelError,
		Scope:   "base_url",
		Message: "mock fetch failure",
	}})
	require.NoError(t, err)
	log := &model.UpstreamSyncLog{
		StartedAt:       startedAt,
		FinishedAt:      startedAt + 10,
		Trigger:         "manual",
		Status:          status,
		BaseUrl:         "https://upstream.example",
		PricingVersion:  "pricing-v1",
		GroupsChecked:   2,
		ChannelsChanged: 1,
		AddedModels:     string(added),
		RemovedModels:   string(removed),
		PriceChanges:    string(priceChanges),
		Errors:          string(issues),
		DurationMs:      42,
		Message:         "mock log",
	}
	require.NoError(t, db.Create(log).Error)
	return log
}

func TestUpstreamSyncConfigGetIncludesLastRunAndActiveTask(t *testing.T) {
	db := setupUpstreamSyncControllerTestDB(t)
	setting.UpstreamSyncEnabled = true
	setting.UpstreamSyncIntervalMinutes = 7
	newUpstreamSyncLog(t, db, 1000, model.UpstreamSyncLogStatusPartial)
	active, err := model.CreateSystemTask(model.SystemTaskTypeUpstreamSync, nil, nil)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/upstream-sync/config", nil)
	GetUpstreamSyncConfig(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                       `json:"success"`
		Data    upstreamSyncConfigResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.True(t, response.Data.Enabled)
	assert.Equal(t, 7, response.Data.IntervalMinutes)
	assert.EqualValues(t, 1010, response.Data.LastRunAt, "last_run_at is Unix seconds from finished_at")
	assert.Equal(t, model.UpstreamSyncLogStatusPartial, response.Data.LastStatus)
	assert.Equal(t, active.TaskID, response.Data.ActiveTaskID)
}

func TestUpstreamSyncConfigUpdatePersistsOptionsAndRejectsOutOfRangeInterval(t *testing.T) {
	db := setupUpstreamSyncControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/upstream-sync/config", strings.NewReader(`{"enabled":true,"interval_minutes":9}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	UpdateUpstreamSyncConfig(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                       `json:"success"`
		Data    upstreamSyncConfigResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.True(t, response.Data.Enabled)
	assert.Equal(t, 9, response.Data.IntervalMinutes)
	assert.True(t, setting.UpstreamSyncEnabled)
	assert.Equal(t, 9, setting.UpstreamSyncIntervalMinutes)

	var enabledOption model.Option
	require.NoError(t, db.Where(&model.Option{Key: setting.UpstreamSyncEnabledOptionKey}).First(&enabledOption).Error)
	assert.Equal(t, "true", enabledOption.Value)
	var intervalOption model.Option
	require.NoError(t, db.Where(&model.Option{Key: setting.UpstreamSyncIntervalMinutesOptionKey}).First(&intervalOption).Error)
	assert.Equal(t, "9", intervalOption.Value)

	for _, interval := range []int{0, 1441} {
		recorder = httptest.NewRecorder()
		ctx, _ = gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(
			http.MethodPut,
			"/api/upstream-sync/config",
			strings.NewReader(`{"interval_minutes":`+strconv.Itoa(interval)+`}`),
		)
		UpdateUpstreamSyncConfig(ctx)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Equal(t, 9, setting.UpstreamSyncIntervalMinutes)
	}
}

func TestUpstreamSyncLogsReturnsTypedJSONAndFilters(t *testing.T) {
	db := setupUpstreamSyncControllerTestDB(t)
	newUpstreamSyncLog(t, db, 1000, model.UpstreamSyncLogStatusSuccess)
	newUpstreamSyncLog(t, db, 2000, model.UpstreamSyncLogStatusFailed)
	log := newUpstreamSyncLog(t, db, 3000, model.UpstreamSyncLogStatusFailed)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/upstream-sync/logs?page=1&page_size=1&status=failed&start_time=2500&end_time=3500&q=mock",
		nil,
	)
	ListUpstreamSyncLogs(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success  bool                            `json:"success"`
		Total    int64                           `json:"total"`
		Page     int                             `json:"page"`
		PageSize int                             `json:"page_size"`
		Data     []model.UpstreamSyncLogResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.EqualValues(t, 1, response.Total)
	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 1, response.PageSize)
	require.Len(t, response.Data, 1)
	assert.Equal(t, log.Id, response.Data[0].Id)
	assert.EqualValues(t, 3000, response.Data[0].StartedAt)
	assert.Equal(t, model.UpstreamSyncLogStatusFailed, response.Data[0].Status)
	assert.Equal(t, []string{"added-model"}, response.Data[0].AddedModels)
	assert.Equal(t, []string{"removed-model"}, response.Data[0].RemovedModels)
	require.Len(t, response.Data[0].PriceChanges, 1)
	assert.Equal(t, "price-model", response.Data[0].PriceChanges[0].ModelName)
	require.Len(t, response.Data[0].Errors, 1)

	assert.Contains(t, recorder.Body.String(), `"added_models":["added-model"]`)
	assert.Contains(t, recorder.Body.String(), `"price_changes":[{`)
}

func TestUpstreamSyncRunDeduplicatesActiveTask(t *testing.T) {
	_ = setupUpstreamSyncControllerTestDB(t)
	existing, err := model.CreateSystemTask(model.SystemTaskTypeUpstreamSync, map[string]any{"manual": true}, nil)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/upstream-sync/run", nil)
	RunUpstreamSyncTask(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			TaskID  string `json:"task_id"`
			Created bool   `json:"created"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, existing.TaskID, response.Data.TaskID)
	assert.False(t, response.Data.Created)
}
