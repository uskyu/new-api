package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpsertPerfMetricMinuteAccumulates(t *testing.T) {
	DB.Exec("DELETE FROM perf_metric_minutes")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM perf_metric_minutes")
	})

	minute := int64(1700001180)
	err := UpsertPerfMetricMinute(&PerfMetricMinute{
		ModelName: "gpt-4", Group: "default", MinuteTs: minute,
		RequestCount: 1, SuccessCount: 1, TotalLatencyMs: 100, OutputTokens: 10, GenerationMs: 1000,
	})
	require.NoError(t, err)
	err = UpsertPerfMetricMinute(&PerfMetricMinute{
		ModelName: "gpt-4", Group: "default", MinuteTs: minute,
		RequestCount: 2, SuccessCount: 1, TotalLatencyMs: 500, OutputTokens: 20, GenerationMs: 2000,
	})
	require.NoError(t, err)

	var row PerfMetricMinute
	require.NoError(t, DB.Where("model_name = ? AND "+commonGroupCol+" = ? AND minute_ts = ?", "gpt-4", "default", minute).First(&row).Error)
	require.Equal(t, int64(3), row.RequestCount)
	require.Equal(t, int64(2), row.SuccessCount)
	require.Equal(t, int64(600), row.TotalLatencyMs)
	require.Equal(t, int64(30), row.OutputTokens)
	require.Equal(t, int64(3000), row.GenerationMs)

	// same model+group in another minute is a distinct row
	require.NoError(t, UpsertPerfMetricMinute(&PerfMetricMinute{
		ModelName: "gpt-4", Group: "default", MinuteTs: minute + 60, RequestCount: 1,
	}))
	var count int64
	require.NoError(t, DB.Model(&PerfMetricMinute{}).Count(&count).Error)
	require.Equal(t, int64(2), count)

	// zero-count upserts must not create rows
	require.NoError(t, UpsertPerfMetricMinute(&PerfMetricMinute{
		ModelName: "gpt-4", Group: "default", MinuteTs: minute + 120,
	}))
	require.NoError(t, DB.Model(&PerfMetricMinute{}).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestGetPerfMetricMinutesDoesNotMixPerfMetricsTable(t *testing.T) {
	DB.Exec("DELETE FROM perf_metric_minutes")
	DB.Exec("DELETE FROM perf_metrics")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM perf_metric_minutes")
		DB.Exec("DELETE FROM perf_metrics")
	})

	rows := []PerfMetricMinute{
		{ModelName: "gpt-4", Group: "default", MinuteTs: 1700001180, RequestCount: 2},
		{ModelName: "gpt-4", Group: "team-a", MinuteTs: 1700001180, RequestCount: 3},
		{ModelName: "gpt-4", Group: "default", MinuteTs: 1700001240, RequestCount: 5},
		{ModelName: "claude-3", Group: "default", MinuteTs: 1700001180, RequestCount: 7},
	}
	require.NoError(t, DB.Create(&rows).Error)
	// same model/group in the coarse long-term table must never leak in
	require.NoError(t, DB.Create(&PerfMetric{
		ModelName: "gpt-4", Group: "default", BucketTs: 1700001180, RequestCount: 999,
	}).Error)

	got, err := GetPerfMetricMinutes(1700001140, 1700001300, nil)
	require.NoError(t, err)
	require.Len(t, got, 4)
	total := int64(0)
	for _, row := range got {
		total += row.RequestCount
	}
	require.Equal(t, int64(17), total)

	// group filter applies only to the minute table
	got, err = GetPerfMetricMinutes(1700001140, 1700001300, []string{"team-a"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int64(3), got[0].RequestCount)

	// empty groups slice means no rows
	got, err = GetPerfMetricMinutes(1700001140, 1700001300, []string{})
	require.NoError(t, err)
	require.Len(t, got, 0)
}

func TestDeletePerfMetricMinutesBefore(t *testing.T) {
	DB.Exec("DELETE FROM perf_metric_minutes")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM perf_metric_minutes")
	})

	rows := []PerfMetricMinute{
		{ModelName: "gpt-4", Group: "default", MinuteTs: 1700001000, RequestCount: 1},
		{ModelName: "gpt-4", Group: "default", MinuteTs: 1700001060, RequestCount: 1},
		{ModelName: "gpt-4", Group: "default", MinuteTs: 1700001120, RequestCount: 1},
	}
	require.NoError(t, DB.Create(&rows).Error)

	require.NoError(t, DeletePerfMetricMinutesBefore(1700001060))
	var remaining []PerfMetricMinute
	require.NoError(t, DB.Order("minute_ts ASC").Find(&remaining).Error)
	require.Len(t, remaining, 2)
	require.Equal(t, int64(1700001060), remaining[0].MinuteTs)
	require.Equal(t, int64(1700001120), remaining[1].MinuteTs)

	// non-positive cutoff is a no-op
	require.NoError(t, DeletePerfMetricMinutesBefore(0))
	require.NoError(t, DB.Order("minute_ts ASC").Find(&remaining).Error)
	require.Len(t, remaining, 2)
}
