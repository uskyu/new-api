package perfmetrics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

var hotBuckets sync.Map

// seriesSchema is a stable client cache/schema marker. Do not change it when
// hiding fields or making response-only privacy hardening changes.
const seriesSchema = "dbcd0a3c01b55203"

var initOnce sync.Once

func Init() {
	initOnce.Do(func() {
		go flushLoop()
		go minuteFlushLoop()
	})
}

func RecordRelaySample(info *relaycommon.RelayInfo, success bool, outputTokens int64) {
	if info == nil {
		return
	}
	now := time.Now()
	hasTtft := info.IsStream && info.HasSendResponse()
	ttftMs := int64(0)
	if hasTtft {
		ttftMs = info.FirstResponseTime.Sub(info.StartTime).Milliseconds()
	}
	latencyMs := now.Sub(info.StartTime).Milliseconds()
	generationMs := latencyMs
	if hasTtft {
		generationMs = now.Sub(info.FirstResponseTime).Milliseconds()
	}
	if generationMs <= 0 {
		generationMs = latencyMs
	}
	Record(Sample{
		Model:        info.OriginModelName,
		Group:        info.UsingGroup,
		LatencyMs:    latencyMs,
		TtftMs:       ttftMs,
		HasTtft:      hasTtft,
		Success:      success,
		OutputTokens: outputTokens,
		GenerationMs: generationMs,
	})
}

func Record(sample Sample) {
	setting := perf_metrics_setting.GetSetting()
	if !setting.Enabled || sample.Model == "" {
		return
	}
	if sample.Group == "" {
		sample.Group = "default"
	}
	if sample.LatencyMs < 0 {
		sample.LatencyMs = 0
	}

	// A single timestamp anchors the configurable bucket, the minute bucket
	// and the derived latency fields, so they never straddle a boundary.
	now := time.Now()
	key := bucketKey{
		model:    sample.Model,
		group:    sample.Group,
		bucketTs: bucketStart(now.Unix()),
	}
	actual, _ := hotBuckets.LoadOrStore(key, &atomicBucket{})
	actual.(*atomicBucket).add(sample)
	recordMinute(sample, now)
	recordRedis(key, sample)
}

func Query(params QueryParams) (QueryResult, error) {
	if params.Hours <= 0 {
		params.Hours = 24
	}
	if params.Hours > 24*30 {
		params.Hours = 24 * 30
	}
	endTs := time.Now().Unix()
	startTs := endTs - int64(params.Hours)*3600

	merged := map[bucketKey]counters{}
	rows, err := model.GetPerfMetrics(params.Model, params.Group, startTs, endTs)
	if err != nil {
		return QueryResult{}, err
	}
	for _, row := range rows {
		mergeCounters(merged, bucketKey{
			model:    row.ModelName,
			group:    row.Group,
			bucketTs: row.BucketTs,
		}, counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			ttftSumMs:      row.TtftSumMs,
			ttftCount:      row.TtftCount,
			outputTokens:   row.OutputTokens,
			generationMs:   row.GenerationMs,
		})
	}

	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.model != params.Model || k.bucketTs < startTs || k.bucketTs > endTs {
			return true
		}
		if params.Group != "" && k.group != params.Group {
			return true
		}
		mergeCounters(merged, k, value.(*atomicBucket).snapshot())
		return true
	})

	return buildQueryResult(params.Model, merged), nil
}

// QuerySummaryAll returns the summary without a series by default; passing
// true keeps the historical hourly series.
func QuerySummaryAll(hours int, groups []string, includeSeries ...bool) (SummaryAllResult, error) {
	seriesMode := ""
	if len(includeSeries) > 0 && includeSeries[0] {
		seriesMode = "hour"
	}
	return QuerySummaryAllWithSeries(hours, groups, seriesMode)
}

// QuerySummaryAllWithSeries accepts a series mode: "hour" keeps the historical
// hourly series, "minute" returns the fixed recent-30-minute one-minute
// series. Any other value (including the empty default) omits the series.
func QuerySummaryAllWithSeries(hours int, groups []string, seriesMode string) (SummaryAllResult, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 24*30 {
		hours = 24 * 30
	}
	if seriesMode != "minute" && seriesMode != "hour" {
		seriesMode = ""
	}
	endTs := time.Now().Unix()

	// series=minute is generated purely from the recent-30-minute minute store
	// (dedicated table + minute hot buckets): models that only exist there must
	// appear, and totals are the 30-minute window, not the hours-long table.
	if seriesMode == "minute" {
		models, err := buildMinuteSummaryModels(groups, endTs)
		if err != nil {
			return SummaryAllResult{}, err
		}
		return SummaryAllResult{Models: models}, nil
	}
	startTs := endTs - int64(hours)*3600
	allowedGroups := allowedGroupSet(groups)

	rows, err := model.GetPerfMetricsSummaryAll(startTs, endTs, groups)
	if err != nil {
		return SummaryAllResult{}, err
	}

	totals := map[string]counters{}
	for _, row := range rows {
		totals[row.ModelName] = counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			outputTokens:   row.OutputTokens,
			generationMs:   row.GenerationMs,
		}
	}

	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs < startTs || k.bucketTs > endTs {
			return true
		}
		if allowedGroups != nil {
			if _, ok := allowedGroups[k.group]; !ok {
				return true
			}
		}
		snap := value.(*atomicBucket).snapshot()
		if snap.requestCount == 0 {
			return true
		}
		cur := totals[k.model]
		cur.requestCount += snap.requestCount
		cur.successCount += snap.successCount
		cur.totalLatencyMs += snap.totalLatencyMs
		cur.outputTokens += snap.outputTokens
		cur.generationMs += snap.generationMs
		totals[k.model] = cur
		return true
	})

	models := make([]ModelSummary, 0, len(totals))
	for name, total := range totals {
		if total.requestCount == 0 {
			continue
		}
		models = append(models, ModelSummary{
			ModelName:    name,
			AvgLatencyMs: avg(total.totalLatencyMs, total.requestCount),
			SuccessRate:  math.Round(successRate(total)*100) / 100,
			AvgTps:       math.Round(avgTps(total)*100) / 100,
			RequestCount: total.requestCount,
		})
	}

	switch seriesMode {
	case "hour":
		if err := attachHourlySeries(models, groups, startTs, endTs, allowedGroups); err != nil {
			return SummaryAllResult{}, err
		}
	}

	sort.Slice(models, func(i, j int) bool { return models[i].RequestCount > models[j].RequestCount })
	return SummaryAllResult{Models: models}, nil
}

func attachHourlySeries(models []ModelSummary, groups []string, startTs int64, endTs int64, allowedGroups map[string]struct{}) error {
	buckets, err := model.GetPerfMetricsSummaryBuckets(startTs, endTs, groups)
	if err != nil {
		return err
	}
	seriesTotals := map[string]map[int64]counters{}
	for _, row := range buckets {
		hourTs := row.BucketTs - row.BucketTs%3600
		if seriesTotals[row.ModelName] == nil {
			seriesTotals[row.ModelName] = map[int64]counters{}
		}
		mergeCounterValue(seriesTotals[row.ModelName], hourTs, counters{
			requestCount: row.RequestCount, successCount: row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs, outputTokens: row.OutputTokens,
			generationMs: row.GenerationMs,
		})
	}
	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs < startTs || k.bucketTs > endTs {
			return true
		}
		if allowedGroups != nil {
			if _, ok := allowedGroups[k.group]; !ok {
				return true
			}
		}
		if seriesTotals[k.model] == nil {
			seriesTotals[k.model] = map[int64]counters{}
		}
		hourTs := k.bucketTs - k.bucketTs%3600
		mergeCounterValue(seriesTotals[k.model], hourTs, value.(*atomicBucket).snapshot())
		return true
	})
	for i := range models {
		models[i].Series = buildHourlySummarySeries(seriesTotals[models[i].ModelName], endTs)
	}
	return nil
}

func buildHourlySummarySeries(values map[int64]counters, endTs int64) []SummaryBucketPoint {
	hourEnd := endTs - endTs%3600
	hourStart := hourEnd - 23*3600
	points := make([]SummaryBucketPoint, 24)
	for i := range points {
		ts := hourStart + int64(i)*3600
		value := values[ts]
		points[i] = SummaryBucketPoint{
			Ts:           ts,
			RequestCount: value.requestCount,
			SuccessCount: value.successCount,
			AvgLatencyMs: avg(value.totalLatencyMs, value.requestCount),
			SuccessRate:  math.Round(successRate(value)*100) / 100,
			AvgTps:       math.Round(avgTps(value)*100) / 100,
		}
	}
	return points
}

// minuteSeriesPoints is the fixed number of one-minute slots returned by the
// recent-30-minute summary series: exactly one slot per minute, oldest first,
// ending at the current minute.
const minuteSeriesPoints = 30

func buildMinuteSummarySeries(values map[int64]counters, endTs int64) []SummaryBucketPoint {
	minuteEnd := endTs - endTs%60
	minuteStart := minuteEnd - (minuteSeriesPoints-1)*60
	points := make([]SummaryBucketPoint, minuteSeriesPoints)
	for i := range points {
		ts := minuteStart + int64(i)*60
		value := values[ts]
		points[i] = SummaryBucketPoint{
			Ts:           ts,
			RequestCount: value.requestCount,
			SuccessCount: value.successCount,
			AvgLatencyMs: avg(value.totalLatencyMs, value.requestCount),
			SuccessRate:  math.Round(successRate(value)*100) / 100,
			AvgTps:       math.Round(avgTps(value)*100) / 100,
		}
	}
	return points
}

func allowedGroupSet(groups []string) map[string]struct{} {
	if groups == nil {
		return nil
	}
	allowed := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		allowed[group] = struct{}{}
	}
	return allowed
}

func bucketStart(ts int64) int64 {
	bucketSeconds := perf_metrics_setting.GetBucketSeconds()
	if bucketSeconds <= 0 {
		bucketSeconds = 3600
	}
	return ts - (ts % bucketSeconds)
}

func mergeCounters(merged map[bucketKey]counters, key bucketKey, value counters) {
	mergeCounterValue(merged, key, value)
}

func mergeCounterValue[K comparable](merged map[K]counters, key K, value counters) {
	if value.requestCount == 0 {
		return
	}
	current := merged[key]
	current.requestCount += value.requestCount
	current.successCount += value.successCount
	current.totalLatencyMs += value.totalLatencyMs
	current.ttftSumMs += value.ttftSumMs
	current.ttftCount += value.ttftCount
	current.outputTokens += value.outputTokens
	current.generationMs += value.generationMs
	merged[key] = current
}

func buildQueryResult(modelName string, merged map[bucketKey]counters) QueryResult {
	groupBuckets := map[string]map[int64]counters{}
	for key, value := range merged {
		if value.requestCount == 0 {
			continue
		}
		if _, ok := groupBuckets[key.group]; !ok {
			groupBuckets[key.group] = map[int64]counters{}
		}
		groupBuckets[key.group][key.bucketTs] = value
	}

	groups := make([]string, 0, len(groupBuckets))
	for group := range groupBuckets {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	results := make([]GroupResult, 0, len(groups))
	for _, group := range groups {
		buckets := groupBuckets[group]
		timestamps := make([]int64, 0, len(buckets))
		for ts := range buckets {
			timestamps = append(timestamps, ts)
		}
		sort.Slice(timestamps, func(i, j int) bool {
			return timestamps[i] < timestamps[j]
		})

		total := counters{}
		series := make([]BucketPoint, 0, len(timestamps))
		for _, ts := range timestamps {
			value := buckets[ts]
			total.requestCount += value.requestCount
			total.successCount += value.successCount
			total.totalLatencyMs += value.totalLatencyMs
			total.ttftSumMs += value.ttftSumMs
			total.ttftCount += value.ttftCount
			total.outputTokens += value.outputTokens
			total.generationMs += value.generationMs
			series = append(series, bucketPoint(ts, value))
		}

		results = append(results, GroupResult{
			Group:        group,
			AvgTtftMs:    avg(total.ttftSumMs, total.ttftCount),
			AvgLatencyMs: avg(total.totalLatencyMs, total.requestCount),
			SuccessRate:  successRate(total),
			AvgTps:       avgTps(total),
			Series:       series,
		})
	}

	return QueryResult{
		ModelName:    modelName,
		SeriesSchema: seriesSchema,
		Groups:       results,
	}
}

func bucketPoint(ts int64, value counters) BucketPoint {
	return BucketPoint{
		Ts:           ts,
		AvgTtftMs:    avg(value.ttftSumMs, value.ttftCount),
		AvgLatencyMs: avg(value.totalLatencyMs, value.requestCount),
		SuccessRate:  successRate(value),
		AvgTps:       avgTps(value),
	}
}

func avg(sum int64, count int64) int64 {
	if count <= 0 {
		return 0
	}
	return sum / count
}

func successRate(value counters) float64 {
	if value.requestCount <= 0 {
		return 0
	}
	return float64(value.successCount) / float64(value.requestCount) * 100
}

func avgTps(value counters) float64 {
	if value.outputTokens <= 0 || value.generationMs <= 0 {
		return 0
	}
	return float64(value.outputTokens) / (float64(value.generationMs) / 1000)
}

func recordRedis(key bucketKey, sample Sample) {
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	redisKey := redisBucketKey(key)
	pipe := common.RDB.TxPipeline()
	pipe.HIncrBy(ctx, redisKey, "req", 1)
	if sample.Success {
		pipe.HIncrBy(ctx, redisKey, "ok", 1)
	}
	if sample.LatencyMs > 0 {
		pipe.HIncrBy(ctx, redisKey, "lat", sample.LatencyMs)
	}
	if sample.HasTtft && sample.TtftMs >= 0 {
		pipe.HIncrBy(ctx, redisKey, "ttft", sample.TtftMs)
		pipe.HIncrBy(ctx, redisKey, "ttft_n", 1)
	}
	if sample.OutputTokens > 0 && sample.GenerationMs > 0 {
		pipe.HIncrBy(ctx, redisKey, "out", sample.OutputTokens)
		pipe.HIncrBy(ctx, redisKey, "gen_ms", sample.GenerationMs)
	}
	pipe.Expire(ctx, redisKey, time.Hour)
	_, _ = pipe.Exec(ctx)
}

func mergeRedisActiveBuckets(merged map[bucketKey]counters, params QueryParams, startTs int64, endTs int64) {
	if !common.RedisEnabled || common.RDB == nil || params.Model == "" || params.Group == "" {
		return
	}
	active := bucketStart(time.Now().Unix())
	if active < startTs || active > endTs {
		return
	}
	key := bucketKey{model: params.Model, group: params.Group, bucketTs: active}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	values, err := common.RDB.HGetAll(ctx, redisBucketKey(key)).Result()
	if err != nil || len(values) == 0 {
		return
	}
	mergeCounters(merged, key, redisCounters(values))
}

func redisBucketKey(key bucketKey) string {
	return fmt.Sprintf("perf:%s:%s:%d", key.model, key.group, key.bucketTs)
}
