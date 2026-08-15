package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PerfMetricMinute stores one-minute relay performance buckets for the
// recent-30-minute model square series. Rows are short-lived: the minute
// flush loop deletes anything older than two hours, independently of the
// long-term perf_metrics retention.
type PerfMetricMinute struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	ModelName      string `json:"model_name" gorm:"size:128;uniqueIndex:idx_perf_minute_model_group_ts,priority:1"`
	Group          string `json:"group" gorm:"column:group;size:64;uniqueIndex:idx_perf_minute_model_group_ts,priority:2"`
	MinuteTs       int64  `json:"minute_ts" gorm:"uniqueIndex:idx_perf_minute_model_group_ts,priority:3;index:idx_perf_minute_ts"`
	RequestCount   int64  `json:"-" gorm:"default:0"`
	SuccessCount   int64  `json:"-" gorm:"default:0"`
	TotalLatencyMs int64  `json:"-" gorm:"default:0"`
	TtftSumMs      int64  `json:"-" gorm:"default:0"`
	TtftCount      int64  `json:"-" gorm:"default:0"`
	OutputTokens   int64  `json:"-" gorm:"default:0"`
	GenerationMs   int64  `json:"-" gorm:"default:0"`
}

func (PerfMetricMinute) TableName() string {
	return "perf_metric_minutes"
}

func UpsertPerfMetricMinute(metric *PerfMetricMinute) error {
	if metric == nil || metric.RequestCount == 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "model_name"},
			{Name: "group"},
			{Name: "minute_ts"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"request_count":    gorm.Expr("perf_metric_minutes.request_count + ?", metric.RequestCount),
			"success_count":    gorm.Expr("perf_metric_minutes.success_count + ?", metric.SuccessCount),
			"total_latency_ms": gorm.Expr("perf_metric_minutes.total_latency_ms + ?", metric.TotalLatencyMs),
			"ttft_sum_ms":      gorm.Expr("perf_metric_minutes.ttft_sum_ms + ?", metric.TtftSumMs),
			"ttft_count":       gorm.Expr("perf_metric_minutes.ttft_count + ?", metric.TtftCount),
			"output_tokens":    gorm.Expr("perf_metric_minutes.output_tokens + ?", metric.OutputTokens),
			"generation_ms":    gorm.Expr("perf_metric_minutes.generation_ms + ?", metric.GenerationMs),
		}),
	}).Create(metric).Error
}

func GetPerfMetricMinutes(startTs int64, endTs int64, groups []string) ([]PerfMetricMinute, error) {
	var metrics []PerfMetricMinute
	query := DB.Model(&PerfMetricMinute{}).
		Where("minute_ts >= ? AND minute_ts <= ?", startTs, endTs)
	if groups != nil {
		if len(groups) == 0 {
			return metrics, nil
		}
		query = query.Where(commonGroupCol+" IN ?", groups)
	}
	err := query.Order("minute_ts ASC, model_name ASC").Find(&metrics).Error
	return metrics, err
}

func DeletePerfMetricMinutesBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return DB.Where("minute_ts < ?", cutoffTs).Delete(&PerfMetricMinute{}).Error
}
