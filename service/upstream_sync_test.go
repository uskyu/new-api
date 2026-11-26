package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareUpstreamSyncTest(t *testing.T) {
	t.Helper()
	model.InitColumnNames()
	require.NoError(t, model.DB.AutoMigrate(
		&model.UpstreamSyncLog{},
		&model.Option{},
		&model.Ability{},
		&model.Model{},
		&model.Vendor{},
		&model.SystemTask{},
		&model.SystemTaskLock{},
	))
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMapRWMutex.Unlock()

	cleanup := func() {
		for _, table := range []string{
			"upstream_sync_logs",
			"options",
			"abilities",
			"channels",
			"system_tasks",
			"system_task_locks",
		} {
			model.DB.Exec("DELETE FROM " + table)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
}

func upstreamSyncMockServer(t *testing.T, status int, payload any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		if payload == nil {
			_, _ = writer.Write([]byte(`{}`))
			return
		}
		body, err := common.Marshal(payload)
		require.NoError(t, err)
		_, _ = writer.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func createUpstreamSyncChannel(t *testing.T, id int, name, group, models, baseURL string, ignored []string) *model.Channel {
	t.Helper()
	otherSettings, err := common.Marshal(dto.ChannelOtherSettings{
		UpstreamModelUpdateIgnoredModels: ignored,
	})
	require.NoError(t, err)
	channel := &model.Channel{
		Id:            id,
		Name:          name,
		Key:           "upstream-sync-test-key",
		Status:        common.ChannelStatusEnabled,
		Group:         group,
		Models:        models,
		BaseURL:       common.GetPointer(baseURL),
		OtherSettings: string(otherSettings),
	}
	require.NoError(t, model.DB.Create(channel).Error)
	return channel
}

func loadUpstreamSyncChannel(t *testing.T, id int) *model.Channel {
	t.Helper()
	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, "id = ?", id).Error)
	return &channel
}

func latestUpstreamSyncLog(t *testing.T) *model.UpstreamSyncLog {
	t.Helper()
	log, err := model.GetLatestUpstreamSyncLog()
	require.NoError(t, err)
	require.NotNil(t, log)
	return log
}

func pricingPayload(models []map[string]any, usableGroups ...string) map[string]any {
	usable := make(map[string]any, len(usableGroups))
	for _, group := range usableGroups {
		usable[group] = ""
	}
	return map[string]any{
		"success":         true,
		"pricing_version": "test-version",
		"usable_group":    usable,
		"data":            models,
	}
}

func TestUpstreamSyncAddsAndRemovesModels(t *testing.T) {
	prepareUpstreamSyncTest(t)
	server := upstreamSyncMockServer(t, http.StatusOK, pricingPayload([]map[string]any{
		{
			"model_name":       "keep-model",
			"quota_type":       1,
			"model_price":      1.5,
			"model_ratio":      0,
			"completion_ratio": 0,
			"enable_groups":    []string{"group-a"},
			"billing_mode":     "",
		},
		{
			"model_name":       "new-model",
			"quota_type":       0,
			"model_ratio":      2,
			"model_price":      0,
			"completion_ratio": 4,
			"enable_groups":    []string{"group-a"},
		},
	}, "group-a"))

	createUpstreamSyncChannel(t, 1, "sync-add-remove", "group-a", "old-model,keep-model", server.URL, nil)

	summary, err := RunUpstreamSync(context.Background(), "manual", nil)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, model.UpstreamSyncLogStatusSuccess, summary.Status)
	assert.Equal(t, 1, summary.ChannelsChanged)
	assert.Contains(t, summary.AddedModels, "new-model")
	assert.Contains(t, summary.RemovedModels, "old-model")

	channel := loadUpstreamSyncChannel(t, 1)
	assert.Equal(t, "keep-model,new-model", channel.Models)
	settings := channel.GetOtherSettings()
	assert.NotZero(t, settings.UpstreamModelUpdateLastCheckTime)
	assert.Equal(t, []string{"new-model"}, settings.UpstreamModelUpdateLastDetectedModels)
	assert.Equal(t, []string{"old-model"}, settings.UpstreamModelUpdateLastRemovedModels)

	var abilityCount int64
	require.NoError(t, model.DB.Model(&model.Ability{}).Where("channel_id = ?", 1).Count(&abilityCount).Error)
	assert.EqualValues(t, 2, abilityCount)

	log := latestUpstreamSyncLog(t)
	assert.Equal(t, model.UpstreamSyncLogStatusSuccess, log.Status)
	assert.Equal(t, "manual", log.Trigger)
	response := log.ToResponse()
	assert.Contains(t, response.AddedModels, "new-model")
	assert.Contains(t, response.RemovedModels, "old-model")
	assert.NotEmpty(t, response.PriceChanges)

	snapshot, err := model.GetModelPricingSnapshot([]string{"keep-model", "new-model"})
	require.NoError(t, err)
	byName := make(map[string]model.ModelPricingEntry)
	for _, entry := range snapshot.Entries {
		byName[entry.ModelName] = entry
	}
	assert.EqualValues(t, 1.5, byName["keep-model"].Configured["ModelPrice"])
	assert.Equal(t, "ratio", byName["keep-model"].Configured["billing_setting.billing_mode"])
	assert.NotContains(t, byName["keep-model"].Configured, "billing_setting.billing_expr")
	assert.EqualValues(t, 2, byName["new-model"].Configured["ModelRatio"])
	assert.EqualValues(t, 4, byName["new-model"].Configured["CompletionRatio"])
	assert.Equal(t, "ratio", byName["new-model"].Configured["billing_setting.billing_mode"])
}

func TestUpstreamSyncRequestFailureLeavesEverythingUnchanged(t *testing.T) {
	prepareUpstreamSyncTest(t)
	server := upstreamSyncMockServer(t, http.StatusInternalServerError, nil)
	createUpstreamSyncChannel(t, 1, "sync-http-failure", "group-a", "model-a,model-b", server.URL, nil)

	summary, err := RunUpstreamSync(context.Background(), "scheduled", nil)
	require.Error(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, model.UpstreamSyncLogStatusFailed, summary.Status)
	assert.NotEmpty(t, summary.Errors)

	channel := loadUpstreamSyncChannel(t, 1)
	assert.Equal(t, "model-a,model-b", channel.Models)
	settings := channel.GetOtherSettings()
	assert.Zero(t, settings.UpstreamModelUpdateLastCheckTime)

	log := latestUpstreamSyncLog(t)
	assert.Equal(t, model.UpstreamSyncLogStatusFailed, log.Status)
	assert.Equal(t, "scheduled", log.Trigger)

	var optionCount int64
	require.NoError(t, model.DB.Model(&model.Option{}).Where("key LIKE ?", "ModelRatio").Count(&optionCount).Error)
	assert.Zero(t, optionCount, "a failed response must not write pricing options")
}

func TestUpstreamSyncEmptyDataLeavesEverythingUnchanged(t *testing.T) {
	prepareUpstreamSyncTest(t)
	server := upstreamSyncMockServer(t, http.StatusOK, pricingPayload([]map[string]any{}))
	createUpstreamSyncChannel(t, 1, "sync-empty-data", "group-a", "model-a,model-b", server.URL, nil)

	summary, err := RunUpstreamSync(context.Background(), "manual", nil)
	require.Error(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, model.UpstreamSyncLogStatusFailed, summary.Status)

	channel := loadUpstreamSyncChannel(t, 1)
	assert.Equal(t, "model-a,model-b", channel.Models)
	assert.Zero(t, channel.GetOtherSettings().UpstreamModelUpdateLastCheckTime)

	log := latestUpstreamSyncLog(t)
	assert.Equal(t, model.UpstreamSyncLogStatusFailed, log.Status)
	assert.Contains(t, log.Message, "no models")
}

func TestUpstreamSyncMissingAndEmptyGroupsRemoveModelsAndWarn(t *testing.T) {
	prepareUpstreamSyncTest(t)
	server := upstreamSyncMockServer(t, http.StatusOK, pricingPayload([]map[string]any{
		{
			"model_name":       "unused-model",
			"quota_type":       0,
			"model_ratio":      1,
			"completion_ratio": 1,
			"enable_groups":    []string{"unused-group"},
		},
	}, "empty-group", "unused-group"))

	createUpstreamSyncChannel(t, 1, "sync-missing-group", "missing-group", "model-a,model-b,model-c", server.URL, []string{"model-b"})
	createUpstreamSyncChannel(t, 2, "sync-empty-group", "empty-group", "empty-model-a,empty-model-b", server.URL, nil)

	summary, err := RunUpstreamSync(context.Background(), "manual", nil)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, model.UpstreamSyncLogStatusSuccess, summary.Status)
	assert.Equal(t, []string{"empty-model-a", "empty-model-b", "model-a", "model-c"}, summary.RemovedModels)

	channelOne := loadUpstreamSyncChannel(t, 1)
	assert.Equal(t, "model-b", channelOne.Models, "ignored model stays even when its group is missing")
	assert.Equal(t, []string{"model-a", "model-c"}, channelOne.GetOtherSettings().UpstreamModelUpdateLastRemovedModels)

	channelTwo := loadUpstreamSyncChannel(t, 2)
	assert.Empty(t, channelTwo.Models)

	log := latestUpstreamSyncLog(t)
	response := log.ToResponse()
	var messages []string
	for _, issue := range response.Errors {
		messages = append(messages, issue.Message)
	}
	assert.Contains(t, messages, "group_missing: upstream pricing response does not contain this group")
	assert.Contains(t, messages, "group_empty: upstream exposes the group but has no models in it")
}

func TestUpstreamSyncDoesNotRemoveIgnoredModels(t *testing.T) {
	prepareUpstreamSyncTest(t)
	server := upstreamSyncMockServer(t, http.StatusOK, pricingPayload([]map[string]any{
		{
			"model_name":       "kept-model",
			"quota_type":       0,
			"model_ratio":      1.25,
			"completion_ratio": 2,
			"enable_groups":    []string{"group-a"},
		},
	}, "group-a"))

	createUpstreamSyncChannel(t, 1, "sync-ignored", "group-a", "kept-model,ignored-model", server.URL, []string{"ignored-model"})

	summary, err := RunUpstreamSync(context.Background(), "manual", nil)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Empty(t, summary.RemovedModels)

	channel := loadUpstreamSyncChannel(t, 1)
	assert.Equal(t, "kept-model,ignored-model", channel.Models)
	assert.Empty(t, channel.GetOtherSettings().UpstreamModelUpdateLastRemovedModels)
}

func TestUpstreamSyncWritesPricesAndNeverGroupRatio(t *testing.T) {
	prepareUpstreamSyncTest(t)
	server := upstreamSyncMockServer(t, http.StatusOK, pricingPayload([]map[string]any{
		{
			"model_name":       "price-model",
			"quota_type":       0,
			"model_ratio":      2.5,
			"completion_ratio": 3,
			"cache_ratio":      0.2,
			"enable_groups":    []string{"group-price"},
		},
		{
			"model_name":       "tiered-model",
			"quota_type":       0,
			"model_ratio":      37.5,
			"model_price":      0,
			"completion_ratio": 1,
			"enable_groups":    []string{"group-price"},
			"billing_mode":     "tiered_expr",
			"billing_expr":     "tier(\"base\", p * 2)",
		},
	}, "group-price"))

	createUpstreamSyncChannel(t, 1, "sync-prices", "group-price", "price-model,tiered-model", server.URL, nil)

	snapshot, err := model.GetModelPricingSnapshot([]string{"price-model"})
	require.NoError(t, err)
	require.Len(t, snapshot.Entries, 1)
	oldExpression := "tier(\"base\", p * 9)"
	require.NoError(t, model.UpdateModelPricing([]model.ModelPricingChange{{
		ModelName:       "price-model",
		ExpectedVersion: snapshot.Entries[0].Version,
		Pricing: model.PricingValues{
			"billing_setting.billing_mode": "tiered_expr",
			"billing_setting.billing_expr": oldExpression,
		},
	}}))

	groupRatioBefore := ratio_setting.GetGroupRatioCopy()

	summary, err := RunUpstreamSync(context.Background(), "manual", nil)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, model.UpstreamSyncLogStatusSuccess, summary.Status)
	assert.NotEmpty(t, summary.PriceChanges)

	after, err := model.GetModelPricingSnapshot([]string{"price-model", "tiered-model"})
	require.NoError(t, err)
	byName := make(map[string]model.ModelPricingEntry)
	for _, entry := range after.Entries {
		byName[entry.ModelName] = entry
	}

	assert.EqualValues(t, 2.5, byName["price-model"].Configured["ModelRatio"])
	assert.EqualValues(t, 3, byName["price-model"].Configured["CompletionRatio"])
	assert.EqualValues(t, 0.2, byName["price-model"].Configured["CacheRatio"])
	assert.Equal(t, "ratio", byName["price-model"].Configured["billing_setting.billing_mode"])
	assert.NotContains(t, byName["price-model"].Configured, "billing_setting.billing_expr")

	assert.Equal(t, "tiered_expr", byName["tiered-model"].Configured["billing_setting.billing_mode"])
	assert.Equal(t, "tier(\"base\", p * 2)", byName["tiered-model"].Configured["billing_setting.billing_expr"])
	assert.NotContains(t, byName["tiered-model"].Configured, "ModelRatio")
	assert.NotContains(t, byName["tiered-model"].Configured, "ModelPrice")

	groupRatioAfter := ratio_setting.GetGroupRatioCopy()
	assert.Equal(t, groupRatioBefore, groupRatioAfter, "upstream group_ratio must never overwrite local group ratios")

	assert.EqualValues(t, 2.5, ratio_setting.GetModelRatioCopy()["price-model"])
}

func TestUpstreamSyncPriceBatchFallsBackToPerModel(t *testing.T) {
	prepareUpstreamSyncTest(t)
	server := upstreamSyncMockServer(t, http.StatusOK, pricingPayload([]map[string]any{
		{
			"model_name":       "valid-price",
			"quota_type":       0,
			"model_ratio":      1.5,
			"completion_ratio": 2,
			"enable_groups":    []string{"group-price"},
		},
		{
			"model_name":    "invalid-price",
			"quota_type":    0,
			"model_ratio":   1,
			"enable_groups": []string{"group-price"},
			"billing_mode":  "tiered_expr",
			"billing_expr":  "tier(",
		},
	}, "group-price"))

	createUpstreamSyncChannel(t, 1, "sync-prices-fallback", "group-price", "valid-price,invalid-price", server.URL, nil)

	summary, err := RunUpstreamSync(context.Background(), "manual", nil)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, model.UpstreamSyncLogStatusPartial, summary.Status)
	assert.EqualValues(t, 1.5, ratio_setting.GetModelRatioCopy()["valid-price"])

	var validApplied bool
	for _, change := range summary.PriceChanges {
		if change.ModelName == "valid-price" {
			validApplied = true
			break
		}
	}
	assert.True(t, validApplied, "valid model price must be applied even when another model's expression is invalid")

	var invalidIssue bool
	for _, issue := range summary.Errors {
		if strings.Contains(issue.Message, "invalid-price") {
			invalidIssue = true
			break
		}
	}
	assert.True(t, invalidIssue, "invalid model expression must be reported in the sync log")
}

func sortedUnion(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
