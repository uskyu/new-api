package perfmetrics

import "testing"

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
