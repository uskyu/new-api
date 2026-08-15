package perfmetrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// init ensures the dialect-specific group column is set for group-filtered
// queries; production initializes it via model.InitDB -> InitCol.
func init() {
	model.InitCol()
}

func TestBuildHourlySummarySeriesFillsEmptyBucketsAndWeightsMetrics(t *testing.T) {
	endTs := int64(1700001234)
	hourEnd := endTs - endTs%3600
	values := map[int64]counters{}
	mergeCounterValue(values, hourEnd-3600, counters{
		requestCount:   1,
		successCount:   1,
		totalLatencyMs: 100,
		outputTokens:   100,
		generationMs:   1000,
	})
	mergeCounterValue(values, hourEnd, counters{
		requestCount:   1,
		successCount:   0,
		totalLatencyMs: 300,
		outputTokens:   300,
		generationMs:   3000,
	})

	series := buildHourlySummarySeries(values, endTs)
	if len(series) != 24 {
		t.Fatalf("expected 24 hourly points, got %d", len(series))
	}
	if series[0].Ts != hourEnd-23*3600 || series[23].Ts != hourEnd {
		t.Fatalf("unexpected timestamps: first=%d last=%d", series[0].Ts, series[23].Ts)
	}
	if series[0].RequestCount != 0 || series[22].RequestCount != 1 {
		t.Fatalf("expected empty leading bucket and one request in previous hour: %+v %+v", series[0], series[22])
	}
	last := series[23]
	if last.RequestCount != 1 || last.SuccessCount != 0 || last.AvgLatencyMs != 300 || last.SuccessRate != 0 || last.AvgTps != 100 {
		t.Fatalf("unexpected last bucket: %+v", last)
	}
}

func TestMergeCounterValueCombinesUnderlyingBuckets(t *testing.T) {
	values := map[int64]counters{}
	mergeCounterValue(values, int64(3600), counters{requestCount: 2, successCount: 2, totalLatencyMs: 200})
	mergeCounterValue(values, int64(3600), counters{requestCount: 3, successCount: 2, totalLatencyMs: 600})

	value := values[3600]
	if value.requestCount != 5 || value.successCount != 4 || value.totalLatencyMs != 800 {
		t.Fatalf("underlying buckets were not combined: %+v", value)
	}
}

func TestBuildMinuteSummarySeriesFillsEmptyBucketsAndWeightsMetrics(t *testing.T) {
	endTs := int64(1700001234)
	minuteEnd := endTs - endTs%60
	values := map[int64]counters{}
	// two underlying minute buckets merged into the previous minute
	mergeCounterValue(values, minuteEnd-60, counters{
		requestCount:   1,
		successCount:   1,
		totalLatencyMs: 100,
		outputTokens:   100,
		generationMs:   1000,
	})
	mergeCounterValue(values, minuteEnd-60, counters{
		requestCount:   2,
		successCount:   1,
		totalLatencyMs: 500,
		outputTokens:   200,
		generationMs:   1000,
	})
	// current minute: one failed request
	mergeCounterValue(values, minuteEnd, counters{
		requestCount:   1,
		successCount:   0,
		totalLatencyMs: 300,
		outputTokens:   300,
		generationMs:   3000,
	})

	series := buildMinuteSummarySeries(values, endTs)
	if len(series) != 30 {
		t.Fatalf("expected 30 minute points, got %d", len(series))
	}
	if series[0].Ts != minuteEnd-29*60 || series[29].Ts != minuteEnd {
		t.Fatalf("unexpected timestamps: first=%d last=%d", series[0].Ts, series[29].Ts)
	}
	for i := 1; i < len(series); i++ {
		if series[i].Ts != series[i-1].Ts+60 {
			t.Fatalf("series not ascending at index %d: %d -> %d", i, series[i-1].Ts, series[i].Ts)
		}
	}
	if series[0].RequestCount != 0 || series[28].RequestCount != 3 {
		t.Fatalf("expected empty leading minute and 3 requests in previous minute: %+v %+v", series[0], series[28])
	}
	prev := series[28]
	if prev.SuccessCount != 2 || prev.AvgLatencyMs != 200 || prev.SuccessRate != 66.67 || prev.AvgTps != 150 {
		t.Fatalf("unexpected weighted previous minute bucket: %+v", prev)
	}
	last := series[29]
	if last.RequestCount != 1 || last.SuccessCount != 0 || last.AvgLatencyMs != 300 || last.SuccessRate != 0 || last.AvgTps != 100 {
		t.Fatalf("unexpected last bucket: %+v", last)
	}
}

func TestBuildMinuteSummarySeriesEndsAtCurrentMinute(t *testing.T) {
	endTs := int64(1700001180) // exactly on a minute boundary
	series := buildMinuteSummarySeries(nil, endTs)
	if len(series) != 30 {
		t.Fatalf("expected 30 minute points, got %d", len(series))
	}
	if series[29].Ts != endTs || series[0].Ts != endTs-29*60 {
		t.Fatalf("expected last slot at endTs=%d and first at %d, got first=%d last=%d",
			endTs, endTs-29*60, series[0].Ts, series[29].Ts)
	}

	// endTs a few seconds after a minute boundary must still end at the
	// boundary (the current minute).
	series2 := buildMinuteSummarySeries(nil, endTs+7)
	if series2[29].Ts != endTs || series2[0].Ts != endTs-29*60 {
		t.Fatalf("expected slots ending at %d, got first=%d last=%d", endTs, series2[0].Ts, series2[29].Ts)
	}
}

func TestMinuteSeriesPointsConstant(t *testing.T) {
	if minuteSeriesPoints != 30 {
		t.Fatalf("minute series must stay at 30 points, got %d", minuteSeriesPoints)
	}
}

func resetHotState() {
	hotBuckets.Range(func(key, value any) bool {
		hotBuckets.Delete(key)
		return true
	})
	minuteHotBuckets.Range(func(key, value any) bool {
		minuteHotBuckets.Delete(key)
		return true
	})
}

func openMinuteTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:perf-minute-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %s", err)
	}
	if err := db.AutoMigrate(&model.PerfMetricMinute{}, &model.PerfMetric{}); err != nil {
		t.Fatalf("failed to migrate test db: %s", err)
	}
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = oldDB
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
}

func countMinuteRows(t *testing.T) int {
	t.Helper()
	var count int64
	if err := model.DB.Model(&model.PerfMetricMinute{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count minute rows: %s", err)
	}
	return int(count)
}

// TestRecordWritesBothConfigurableAndMinuteBuckets verifies the minute chain
// is independent of perf_metrics_setting.bucket_time: with the default hour
// setting the configurable hot bucket is hour-aligned while the minute hot
// bucket is always minute-aligned.
func TestRecordWritesBothConfigurableAndMinuteBuckets(t *testing.T) {
	resetHotState()
	t.Cleanup(resetHotState)
	Record(Sample{
		Model: "gpt-4", Group: "default", LatencyMs: 120,
		Success: true, OutputTokens: 12, GenerationMs: 1000,
	})

	configurableAligned := false
	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.model != "gpt-4" {
			return true
		}
		configurableAligned = true
		if k.bucketTs%3600 != 0 {
			t.Errorf("configurable bucket ts %d is not hour-aligned under default setting", k.bucketTs)
		}
		return true
	})
	if !configurableAligned {
		t.Fatal("Record did not write the configurable hot bucket")
	}

	minuteAligned := false
	minuteHotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.model != "gpt-4" {
			return true
		}
		minuteAligned = true
		if k.bucketTs%60 != 0 {
			t.Errorf("minute bucket ts %d is not minute-aligned", k.bucketTs)
		}
		return true
	})
	if !minuteAligned {
		t.Fatal("Record did not write the independent minute hot bucket")
	}
}

// TestFlushMinuteBucketsPersistsWithoutDoubleCounting drains both a completed
// and the current minute into the DB, then verifies a second flush adds
// nothing and no hot bucket keeps nonzero counts.
func TestFlushMinuteBucketsPersistsWithoutDoubleCounting(t *testing.T) {
	resetHotState()
	t.Cleanup(resetHotState)
	openMinuteTestDB(t)

	now := time.Now()
	past := time.Unix(minuteBucketTs(now.Unix())-120, 0)
	recordMinute(Sample{Model: "gpt-4", Group: "default", LatencyMs: 100, Success: true, OutputTokens: 10, GenerationMs: 1000}, past)
	recordMinute(Sample{Model: "gpt-4", Group: "default", LatencyMs: 300, Success: false, OutputTokens: 30, GenerationMs: 3000}, now)

	flushMinuteBuckets()
	if rows := countMinuteRows(t); rows != 2 {
		t.Fatalf("expected 2 persisted minutes after flush, got %d", rows)
	}
	flushMinuteBuckets()
	if rows := countMinuteRows(t); rows != 2 {
		t.Fatalf("second flush must not create rows, got %d", rows)
	}

	var pastRow, currentRow model.PerfMetricMinute
	if err := model.DB.Where("model_name = ? AND minute_ts = ?", "gpt-4", past.Unix()).First(&pastRow).Error; err != nil {
		t.Fatalf("past minute row missing: %s", err)
	}
	if pastRow.RequestCount != 1 || pastRow.TotalLatencyMs != 100 {
		t.Fatalf("unexpected past minute row: %+v", pastRow)
	}
	if err := model.DB.Where("model_name = ? AND minute_ts = ?", "gpt-4", minuteBucketTs(now.Unix())).First(&currentRow).Error; err != nil {
		t.Fatalf("current minute row missing: %s", err)
	}
	if currentRow.RequestCount != 1 || currentRow.TotalLatencyMs != 300 {
		t.Fatalf("unexpected current minute row: %+v", currentRow)
	}

	// a zeroed completed bucket must be dropped, not kept forever
	pastKey := bucketKey{model: "gpt-4", group: "default", bucketTs: minuteBucketTs(now.Unix()) - 300}
	minuteHotBuckets.Store(pastKey, &atomicBucket{})
	flushMinuteBuckets()
	if _, ok := minuteHotBuckets.Load(pastKey); ok {
		t.Error("zeroed completed hot bucket must be dropped after flush")
	}

	minuteHotBuckets.Range(func(key, value any) bool {
		if value.(*atomicBucket).snapshot().requestCount != 0 {
			t.Errorf("hot bucket %+v kept nonzero counts after flush", key.(bucketKey))
		}
		return true
	})

	// dedicated model-list path must report the drained requests exactly once
	models, err := buildMinuteSummaryModels(nil, now.Unix())
	if err != nil {
		t.Fatalf("buildMinuteSummaryModels: %s", err)
	}
	if len(models) != 1 || models[0].ModelName != "gpt-4" || models[0].RequestCount != 2 {
		t.Fatalf("model list after flush must report each drained request once: %+v", models)
	}
	seriesTotal := int64(0)
	for _, point := range models[0].Series {
		seriesTotal += point.RequestCount
	}
	if seriesTotal != 2 {
		t.Fatalf("series must not double count flushed batches, got %d", seriesTotal)
	}
}

// TestMinuteSummaryMergesDbAndHotWithoutDoubleCount verifies the dedicated
// model-list path aggregates the minute table plus the local not-yet-drained
// hot buckets exactly once: the 30-point series total and the summary fields
// equal the sum of both sources, never doubled.
func TestMinuteSummaryMergesDbAndHotWithoutDoubleCount(t *testing.T) {
	resetHotState()
	t.Cleanup(resetHotState)
	openMinuteTestDB(t)

	now := time.Now()
	endTs := now.Unix()
	past := time.Unix(minuteBucketTs(endTs)-60, 0)
	// two requests in the completed minute, one in the current minute
	recordMinute(Sample{Model: "gpt-4", Group: "default", LatencyMs: 100, Success: true, OutputTokens: 10, GenerationMs: 1000}, past)
	recordMinute(Sample{Model: "gpt-4", Group: "default", LatencyMs: 500, Success: true, OutputTokens: 50, GenerationMs: 5000}, past)
	recordMinute(Sample{Model: "gpt-4", Group: "default", LatencyMs: 300, Success: false, OutputTokens: 30, GenerationMs: 3000}, now)
	flushMinuteBuckets()
	// a sample arriving after the flush stays hot until the next drain
	recordMinute(Sample{Model: "gpt-4", Group: "default", LatencyMs: 200, Success: true, OutputTokens: 20, GenerationMs: 2000}, now)

	models, err := buildMinuteSummaryModels(nil, endTs)
	if err != nil {
		t.Fatalf("buildMinuteSummaryModels: %s", err)
	}
	if len(models) != 1 || models[0].ModelName != "gpt-4" {
		t.Fatalf("expected exactly gpt-4, got %+v", models)
	}
	series := models[0].Series
	if len(series) != minuteSeriesPoints {
		t.Fatalf("expected %d points, got %d", minuteSeriesPoints, len(series))
	}
	if series[len(series)-1].Ts != minuteBucketTs(endTs) {
		t.Fatalf("series must end at the current minute, got %d", series[len(series)-1].Ts)
	}

	totalRequests := int64(0)
	totalLatency := int64(0)
	for _, point := range series {
		totalRequests += point.RequestCount
		totalLatency += point.AvgLatencyMs * point.RequestCount
	}
	if totalRequests != 4 {
		t.Fatalf("expected 4 total requests (2 persisted + 2 hot), got %d", totalRequests)
	}
	if totalLatency != 100+500+300+200 {
		t.Fatalf("expected total latency 1100, got %d", totalLatency)
	}
	if models[0].RequestCount != 4 || models[0].AvgLatencyMs != 275 {
		t.Fatalf("summary fields must match the 30-minute window, got %d requests / %d ms",
			models[0].RequestCount, models[0].AvgLatencyMs)
	}
	if models[0].Series[28].RequestCount != 2 || models[0].Series[29].RequestCount != 2 {
		t.Fatalf("expected 2 persisted + 2 hot split across the last two slots: %+v", models[0].Series[28:])
	}
}

// TestQuerySummaryAllVariadicCompatibility keeps the historical signature:
// no argument means no series, true means the hourly series, false means none.
func TestQuerySummaryAllVariadicCompatibility(t *testing.T) {
	resetHotState()
	t.Cleanup(resetHotState)
	openMinuteTestDB(t)

	endTs := time.Now().Unix()
	hourTs := endTs - endTs%3600
	if err := model.DB.Create(&model.PerfMetric{
		ModelName: "gpt-4", Group: "default", BucketTs: hourTs,
		RequestCount: 2, SuccessCount: 2, TotalLatencyMs: 200,
	}).Error; err != nil {
		t.Fatalf("failed to insert perf_metrics row: %s", err)
	}
	if err := model.DB.Create(&model.PerfMetricMinute{
		ModelName: "gpt-4", Group: "default", MinuteTs: minuteBucketTs(endTs),
		RequestCount: 1,
	}).Error; err != nil {
		t.Fatalf("failed to insert minute row: %s", err)
	}

	result, err := QuerySummaryAll(24, nil)
	if err != nil {
		t.Fatalf("QuerySummaryAll(): %s", err)
	}
	if len(result.Models) != 1 || len(result.Models[0].Series) != 0 {
		t.Fatalf("plain QuerySummaryAll must omit the series: %+v", result.Models)
	}

	result, err = QuerySummaryAll(24, nil, true)
	if err != nil {
		t.Fatalf("QuerySummaryAll(true): %s", err)
	}
	if len(result.Models[0].Series) != 24 || result.Models[0].Series[23].RequestCount != 2 {
		t.Fatalf("QuerySummaryAll(true) must return the hourly series: %+v", result.Models[0])
	}

	result, err = QuerySummaryAll(24, nil, false)
	if err != nil {
		t.Fatalf("QuerySummaryAll(false): %s", err)
	}
	if len(result.Models[0].Series) != 0 {
		t.Fatalf("QuerySummaryAll(false) must omit the series: %+v", result.Models[0])
	}

	result, err = QuerySummaryAllWithSeries(1, nil, "minute")
	if err != nil {
		t.Fatalf("QuerySummaryAllWithSeries(minute): %s", err)
	}
	if len(result.Models[0].Series) != minuteSeriesPoints {
		t.Fatalf("minute series must have %d points, got %d", minuteSeriesPoints, len(result.Models[0].Series))
	}
	if result.Models[0].Series[minuteSeriesPoints-1].RequestCount != 1 {
		t.Fatalf("current minute slot must carry the minute row: %+v", result.Models[0].Series[minuteSeriesPoints-1])
	}
}

func TestCleanupExpiredMinuteMetrics(t *testing.T) {
	resetHotState()
	t.Cleanup(resetHotState)
	openMinuteTestDB(t)

	now := time.Now().Unix()
	old := minuteBucketTs(now) - 3*3600
	recent := minuteBucketTs(now) - 60
	if err := model.DB.Create(&model.PerfMetricMinute{ModelName: "gpt-4", Group: "default", MinuteTs: old, RequestCount: 1}).Error; err != nil {
		t.Fatalf("failed to insert old minute row: %s", err)
	}
	if err := model.DB.Create(&model.PerfMetricMinute{ModelName: "gpt-4", Group: "default", MinuteTs: recent, RequestCount: 1}).Error; err != nil {
		t.Fatalf("failed to insert recent minute row: %s", err)
	}
	if err := model.DB.Create(&model.PerfMetric{ModelName: "gpt-4", Group: "default", BucketTs: old, RequestCount: 1}).Error; err != nil {
		t.Fatalf("failed to insert perf_metrics row: %s", err)
	}

	cleanupExpiredMinuteMetrics()
	if rows := countMinuteRows(t); rows != 1 {
		t.Fatalf("expected only the recent minute row to survive, got %d", rows)
	}
	var longTermCount int64
	if err := model.DB.Model(&model.PerfMetric{}).Count(&longTermCount).Error; err != nil {
		t.Fatalf("failed to count perf_metrics rows: %s", err)
	}
	if longTermCount != 1 {
		t.Fatalf("long-term perf_metrics row must be untouched, got %d", longTermCount)
	}
}

// TestMinuteSummaryReturnsModelsOnlyInMinuteTable verifies series=minute
// surfaces models whose data exists only in perf_metric_minutes, with no
// perf_metrics row backing them, while a model that exists only in the
// long-term hour table must not leak into the minute list.
func TestMinuteSummaryReturnsModelsOnlyInMinuteTable(t *testing.T) {
	resetHotState()
	t.Cleanup(resetHotState)
	openMinuteTestDB(t)

	endTs := time.Now().Unix()
	currentMinute := minuteBucketTs(endTs)
	rows := []model.PerfMetricMinute{
		{ModelName: "only-minute-model", Group: "default", MinuteTs: currentMinute - 60,
			RequestCount: 2, SuccessCount: 2, TotalLatencyMs: 200, OutputTokens: 20, GenerationMs: 2000},
		{ModelName: "only-minute-model", Group: "default", MinuteTs: currentMinute,
			RequestCount: 1, SuccessCount: 0, TotalLatencyMs: 300, OutputTokens: 30, GenerationMs: 3000},
		{ModelName: "busy-model", Group: "default", MinuteTs: currentMinute - 60,
			RequestCount: 5, SuccessCount: 5, TotalLatencyMs: 1000, OutputTokens: 100, GenerationMs: 5000},
	}
	if err := model.DB.Create(&rows).Error; err != nil {
		t.Fatalf("failed to insert minute rows: %s", err)
	}
	// decoy: a long-term row inside the hour for a model with zero minute data
	if err := model.DB.Create(&model.PerfMetric{
		ModelName: "hourly-only-model", Group: "default", BucketTs: endTs - endTs%3600,
		RequestCount: 999, SuccessCount: 999, TotalLatencyMs: 999000,
	}).Error; err != nil {
		t.Fatalf("failed to insert perf_metrics decoy: %s", err)
	}

	result, err := QuerySummaryAllWithSeries(1, nil, "minute")
	if err != nil {
		t.Fatalf("QuerySummaryAllWithSeries(minute): %s", err)
	}
	if len(result.Models) != 2 {
		t.Fatalf("expected the two minute-table models, got %+v", result.Models)
	}
	// sorted by request count desc: busy-model (5) before only-minute-model (3)
	if result.Models[0].ModelName != "busy-model" || result.Models[1].ModelName != "only-minute-model" {
		t.Fatalf("unexpected ordering or models: %+v", result.Models)
	}
	only := result.Models[1]
	if len(only.Series) != minuteSeriesPoints {
		t.Fatalf("expected %d points, got %d", minuteSeriesPoints, len(only.Series))
	}
	if only.RequestCount != 3 || only.AvgLatencyMs != 166 {
		t.Fatalf("expected 3 requests / 166ms avg for only-minute-model, got %d / %d",
			only.RequestCount, only.AvgLatencyMs)
	}
	if only.SuccessRate != 66.67 {
		t.Fatalf("expected success rate 66.67, got %v", only.SuccessRate)
	}
	if only.Series[minuteSeriesPoints-2].RequestCount != 2 || only.Series[minuteSeriesPoints-1].RequestCount != 1 {
		t.Fatalf("series slots must carry the minute rows: %+v", only.Series[minuteSeriesPoints-2:])
	}
}

// TestMinuteSummaryTotalsCoverThirtyMinutesNotHour verifies series=minute
// totals are the 30-minute window even when the caller passes hours=1: data
// older than 30 minutes but inside the hour must be excluded from the model
// list, the summary fields and the series.
func TestMinuteSummaryTotalsCoverThirtyMinutesNotHour(t *testing.T) {
	resetHotState()
	t.Cleanup(resetHotState)
	openMinuteTestDB(t)

	endTs := time.Now().Unix()
	currentMinute := minuteBucketTs(endTs)
	rows := []model.PerfMetricMinute{
		{ModelName: "gpt-4", Group: "default", MinuteTs: currentMinute - 60, RequestCount: 2},
		// 40 minutes ago: inside the hour, outside the 30-minute series window
		{ModelName: "gpt-4", Group: "default", MinuteTs: currentMinute - 40*60, RequestCount: 100},
	}
	if err := model.DB.Create(&rows).Error; err != nil {
		t.Fatalf("failed to insert minute rows: %s", err)
	}

	result, err := QuerySummaryAllWithSeries(1, nil, "minute")
	if err != nil {
		t.Fatalf("QuerySummaryAllWithSeries(minute): %s", err)
	}
	if len(result.Models) != 1 || result.Models[0].ModelName != "gpt-4" {
		t.Fatalf("expected gpt-4 only, got %+v", result.Models)
	}
	m := result.Models[0]
	if m.RequestCount != 2 {
		t.Fatalf("summary must cover 30 minutes, not the 1-hour window; got %d", m.RequestCount)
	}
	seriesTotal := int64(0)
	for _, point := range m.Series {
		if point.Ts < currentMinute-29*60 {
			t.Fatalf("series point %d lies outside the 30-minute window", point.Ts)
		}
		seriesTotal += point.RequestCount
	}
	if seriesTotal != 2 {
		t.Fatalf("series must total 2, got %d", seriesTotal)
	}
}

// TestMinuteSummaryGroupFilter verifies allowed-group filtering applies to
// both DB rows and hot buckets in the minute model-list path.
func TestMinuteSummaryGroupFilter(t *testing.T) {
	resetHotState()
	t.Cleanup(resetHotState)
	openMinuteTestDB(t)

	endTs := time.Now().Unix()
	currentMinute := minuteBucketTs(endTs)
	rows := []model.PerfMetricMinute{
		{ModelName: "vip-model", Group: "vip", MinuteTs: currentMinute - 60, RequestCount: 3},
		{ModelName: "team-model", Group: "team-a", MinuteTs: currentMinute - 60, RequestCount: 4},
	}
	if err := model.DB.Create(&rows).Error; err != nil {
		t.Fatalf("failed to insert minute rows: %s", err)
	}
	// hot buckets for both groups must be filtered the same way
	recordMinute(Sample{Model: "team-model", Group: "team-a", LatencyMs: 100, Success: true}, time.Unix(endTs, 0))
	recordMinute(Sample{Model: "vip-model", Group: "vip", LatencyMs: 200, Success: true}, time.Unix(endTs, 0))

	models, err := buildMinuteSummaryModels([]string{"vip"}, endTs)
	if err != nil {
		t.Fatalf("buildMinuteSummaryModels: %s", err)
	}
	if len(models) != 1 || models[0].ModelName != "vip-model" || models[0].RequestCount != 4 {
		t.Fatalf("expected only vip-model with 4 requests, got %+v", models)
	}

	models, err = buildMinuteSummaryModels(nil, endTs)
	if err != nil {
		t.Fatalf("buildMinuteSummaryModels(nil): %s", err)
	}
	if len(models) != 2 {
		t.Fatalf("nil groups must return every model, got %+v", models)
	}

	models, err = buildMinuteSummaryModels([]string{}, endTs)
	if err != nil {
		t.Fatalf("buildMinuteSummaryModels(empty): %s", err)
	}
	if len(models) != 0 {
		t.Fatalf("empty groups must return no models, got %+v", models)
	}
}

// TestMinuteSummaryConcurrentFlushNeverDoubleCountsOrMisses drives flush and
// summary concurrently: under minuteStoreMu every snapshot must see each
// drained batch exactly once (fully in hot before the flush, fully in the DB
// after it), never doubled and never momentarily missing.
func TestMinuteSummaryConcurrentFlushNeverDoubleCountsOrMisses(t *testing.T) {
	resetHotState()
	t.Cleanup(resetHotState)
	openMinuteTestDB(t)

	endTs := time.Now().Unix()
	for i := 1; i <= 3; i++ {
		past := time.Unix(minuteBucketTs(endTs)-int64(i)*60, 0)
		recordMinute(Sample{Model: "gpt-4", Group: "default", LatencyMs: 100, Success: true}, past)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 300; i++ {
			flushMinuteBuckets()
		}
	}()

	const expected = 3
	for i := 0; i < 300; i++ {
		models, err := buildMinuteSummaryModels(nil, endTs)
		if err != nil {
			t.Fatalf("buildMinuteSummaryModels: %s", err)
		}
		total := int64(0)
		for _, m := range models {
			total += m.RequestCount
		}
		if total != expected {
			t.Fatalf("iteration %d: summary must always see exactly %d requests, got %d",
				i, expected, total)
		}
	}
	<-done
}
