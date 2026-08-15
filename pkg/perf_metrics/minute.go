package perfmetrics

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

// minuteHotBuckets holds per-minute counters that have not been persisted to
// perf_metric_minutes yet. Buckets are always 60s wide regardless of the
// configurable perf_metrics_setting.bucket_time; every flush drains them into
// the shared table, so multi-instance deployments aggregate via the DB.
var minuteHotBuckets sync.Map

// minuteStoreMu serializes the handoff between the minute hot buckets and the
// perf_metric_minutes table. flushMinuteBuckets holds the write lock across
// drain + upsert + failure refill + delete; minute summaries hold the read
// lock across their DB read + hot snapshot. recordMinute stays lock-free
// (atomic adds), which is safe because a flush cannot interleave with a
// summary's DB+hot reads, so a drained batch is always seen exactly once:
// entirely in hot before the flush, entirely in the DB after it.
var minuteStoreMu sync.RWMutex

func minuteBucketTs(ts int64) int64 {
	return ts - ts%60
}

func recordMinute(sample Sample, now time.Time) {
	key := bucketKey{
		model:    sample.Model,
		group:    sample.Group,
		bucketTs: minuteBucketTs(now.Unix()),
	}
	actual, _ := minuteHotBuckets.LoadOrStore(key, &atomicBucket{})
	actual.(*atomicBucket).add(sample)
}

// minuteFlushLoop persists minute buckets once a minute, independent of the
// configurable flush interval, so completed minutes are visible across
// instances promptly. The first tick waits for the next minute boundary.
func minuteFlushLoop() {
	time.Sleep(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)))
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		setting := perf_metrics_setting.GetSetting()
		if setting.Enabled {
			flushMinuteBuckets()
		}
		cleanupExpiredMinuteMetrics()
	}
}

func flushMinuteBuckets() {
	currentMinute := minuteBucketTs(time.Now().Unix())
	// Hold the write lock for the entire drain + upsert + refill + delete
	// phase so a concurrent minute summary either sees the batch fully in
	// hot (pre-flush) or fully in the DB (post-flush), never split or twice.
	minuteStoreMu.Lock()
	defer minuteStoreMu.Unlock()
	minuteHotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs > currentMinute {
			return true
		}
		bucket := value.(*atomicBucket)
		drained := bucket.drain()
		if drained.requestCount == 0 {
			// Completed buckets never receive new samples (recordMinute only
			// writes the current minute), so zeroed ones can be dropped.
			if k.bucketTs < currentMinute {
				minuteHotBuckets.Delete(key)
			}
			return true
		}
		err := model.UpsertPerfMetricMinute(&model.PerfMetricMinute{
			ModelName:      k.model,
			Group:          k.group,
			MinuteTs:       k.bucketTs,
			RequestCount:   drained.requestCount,
			SuccessCount:   drained.successCount,
			TotalLatencyMs: drained.totalLatencyMs,
			TtftSumMs:      drained.ttftSumMs,
			TtftCount:      drained.ttftCount,
			OutputTokens:   drained.outputTokens,
			GenerationMs:   drained.generationMs,
		})
		if err != nil {
			bucket.addCounters(drained)
			common.SysError(fmt.Sprintf("failed to flush perf metric minute model=%s group=%s minute=%d: %s", k.model, k.group, k.bucketTs, err.Error()))
			return true
		}
		// The current minute stays hot so later samples accumulate there and
		// are picked up by the next drain; completed minutes can be dropped.
		if k.bucketTs < currentMinute {
			minuteHotBuckets.Delete(key)
		}
		return true
	})
}

func cleanupExpiredMinuteMetrics() {
	// 2h retention covers the 30-minute series with slack for slow queries;
	// long-term retention semantics of perf_metrics are untouched.
	cutoff := minuteBucketTs(time.Now().Unix()) - 120*60
	if err := model.DeletePerfMetricMinutesBefore(cutoff); err != nil {
		common.SysError("failed to cleanup expired perf metric minutes: " + err.Error())
	}
}

// buildMinuteSummaryModels builds the series=minute model list entirely from
// the dedicated recent-30-minute store (perf_metric_minutes plus the local
// not-yet-drained minute hot buckets). Models that only exist in the minute
// table therefore still appear, independent of the long-term perf_metrics
// table. One pass produces the per-model 30-point series and the 30-minute
// summary; models without any request in the window are omitted. The whole
// DB read + hot snapshot happens under the read lock, so a concurrent flush
// cannot split a drained batch between the two sources.
func buildMinuteSummaryModels(groups []string, endTs int64) ([]ModelSummary, error) {
	minuteEnd := minuteBucketTs(endTs)
	startTs := minuteEnd - (minuteSeriesPoints-1)*60

	minuteStoreMu.RLock()
	defer minuteStoreMu.RUnlock()

	rows, err := model.GetPerfMetricMinutes(startTs, minuteEnd, groups)
	if err != nil {
		return nil, err
	}
	seriesTotals := map[string]map[int64]counters{}
	totals := map[string]counters{}
	mergeRow := func(modelName string, ts int64, value counters) {
		if value.requestCount == 0 {
			return
		}
		if seriesTotals[modelName] == nil {
			seriesTotals[modelName] = map[int64]counters{}
		}
		mergeCounterValue(seriesTotals[modelName], ts, value)
		mergeCounterValue(totals, modelName, value)
	}
	for _, row := range rows {
		mergeRow(row.ModelName, row.MinuteTs, counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			outputTokens:   row.OutputTokens,
			generationMs:   row.GenerationMs,
		})
	}
	allowedGroups := allowedGroupSet(groups)
	minuteHotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs < startTs || k.bucketTs > minuteEnd {
			return true
		}
		if allowedGroups != nil {
			if _, ok := allowedGroups[k.group]; !ok {
				return true
			}
		}
		mergeRow(k.model, k.bucketTs, value.(*atomicBucket).snapshot())
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
			Series:       buildMinuteSummarySeries(seriesTotals[name], endTs),
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].RequestCount > models[j].RequestCount })
	return models, nil
}
