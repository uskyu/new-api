package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	UpstreamSyncLogStatusSuccess = "success"
	UpstreamSyncLogStatusFailed  = "failed"
	UpstreamSyncLogStatusPartial = "partial"

	UpstreamSyncIssueLevelError   = "error"
	UpstreamSyncIssueLevelWarning = "warning"

	upstreamSyncLogDefaultRetention = 10000
)

// UpstreamSyncPriceChange records the pricing fields refreshed from the
// upstream /api/pricing response for one model. Before/After only contain the
// fields that actually changed, keyed by the local pricing option name.
type UpstreamSyncPriceChange struct {
	ModelName string         `json:"model_name"`
	Fields    []string       `json:"fields"`
	Before    map[string]any `json:"before,omitempty"`
	After     map[string]any `json:"after,omitempty"`
}

// UpstreamSyncIssue is one per-base-url, per-channel or per-group problem
// captured during a sync run. Warnings (for example a group that upstream no
// longer exposes) do not by themselves make the run partial.
type UpstreamSyncIssue struct {
	Level       string `json:"level,omitempty"`
	Scope       string `json:"scope,omitempty"`
	BaseURL     string `json:"base_url,omitempty"`
	ChannelID   int    `json:"channel_id,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
	Group       string `json:"group,omitempty"`
	Message     string `json:"message"`
}

// UpstreamSyncLog is one persisted sync run. Structured payloads are stored as
// TEXT JSON so the table works unchanged on SQLite, MySQL and PostgreSQL.
type UpstreamSyncLog struct {
	Id              int64  `json:"id" gorm:"primary_key"`
	StartedAt       int64  `json:"started_at" gorm:"bigint;index"`
	FinishedAt      int64  `json:"finished_at" gorm:"bigint;index"`
	Trigger         string `json:"trigger" gorm:"type:varchar(32);index"`
	Status          string `json:"status" gorm:"type:varchar(32);index"`
	BaseUrl         string `json:"base_url" gorm:"type:varchar(1024)"`
	PricingVersion  string `json:"pricing_version" gorm:"type:varchar(255)"`
	GroupsChecked   int    `json:"groups_checked"`
	ChannelsChanged int    `json:"channels_changed"`
	AddedModels     string `json:"-" gorm:"column:added_models;type:text"`
	RemovedModels   string `json:"-" gorm:"column:removed_models;type:text"`
	PriceChanges    string `json:"-" gorm:"column:price_changes;type:text"`
	Errors          string `json:"-" gorm:"column:errors;type:text"`
	DurationMs      int64  `json:"duration_ms"`
	Message         string `json:"message" gorm:"type:text"`
}

func (UpstreamSyncLog) TableName() string {
	return "upstream_sync_logs"
}

// UpstreamSyncLogResponse is the API shape of a log row: TEXT columns are
// decoded back to real JSON arrays/objects and all timestamps stay Unix
// seconds so the console date pickers need no extra parsing.
type UpstreamSyncLogResponse struct {
	Id              int64                     `json:"id"`
	StartedAt       int64                     `json:"started_at"`
	FinishedAt      int64                     `json:"finished_at"`
	Trigger         string                    `json:"trigger"`
	Status          string                    `json:"status"`
	BaseURL         string                    `json:"base_url"`
	PricingVersion  string                    `json:"pricing_version"`
	GroupsChecked   int                       `json:"groups_checked"`
	ChannelsChanged int                       `json:"channels_changed"`
	AddedModels     []string                  `json:"added_models"`
	RemovedModels   []string                  `json:"removed_models"`
	PriceChanges    []UpstreamSyncPriceChange `json:"price_changes"`
	Errors          []UpstreamSyncIssue       `json:"errors"`
	DurationMs      int64                     `json:"duration_ms"`
	Message         string                    `json:"message"`
}

type UpstreamSyncLogFilter struct {
	Status    string
	Keyword   string
	StartTime int64
	EndTime   int64
}

func CreateUpstreamSyncLog(log *UpstreamSyncLog) error {
	if log == nil {
		return nil
	}
	if log.StartedAt == 0 {
		log.StartedAt = common.GetTimestamp()
	}
	if log.FinishedAt == 0 {
		log.FinishedAt = log.StartedAt
	}
	return DB.Create(log).Error
}

func GetLatestUpstreamSyncLog() (*UpstreamSyncLog, error) {
	var log UpstreamSyncLog
	err := DB.Order("id desc").First(&log).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &log, nil
}

func ListUpstreamSyncLogs(page, pageSize int, filter UpstreamSyncLogFilter) ([]*UpstreamSyncLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := DB.Model(&UpstreamSyncLog{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.StartTime > 0 {
		query = query.Where("started_at >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("started_at <= ?", filter.EndTime)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		pattern := "%" + escapeUpstreamSyncLike(strings.ToLower(keyword)) + "%"
		query = query.Where(
			"(LOWER(base_url) LIKE ? ESCAPE '!' OR LOWER(message) LIKE ? ESCAPE '!' OR LOWER(added_models) LIKE ? ESCAPE '!' OR LOWER(removed_models) LIKE ? ESCAPE '!' OR LOWER(price_changes) LIKE ? ESCAPE '!' OR LOWER(errors) LIKE ? ESCAPE '!')",
			pattern, pattern, pattern, pattern, pattern, pattern,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []*UpstreamSyncLog
	err := query.Order("id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error
	return logs, total, err
}

// DeleteUpstreamSyncLogsBefore removes finished runs older than the given
// timestamp. It is safe on all supported databases because it uses a simple
// indexed comparison and no dialect-specific syntax.
func DeleteUpstreamSyncLogsBefore(before int64) (int64, error) {
	result := DB.Where("finished_at > 0 AND finished_at < ?", before).Delete(&UpstreamSyncLog{})
	return result.RowsAffected, result.Error
}

// TrimUpstreamSyncLogs keeps only the newest maxRows rows. Callers use this as
// a bounded retention policy so the table cannot grow without limit.
func TrimUpstreamSyncLogs(maxRows int) (int64, error) {
	if maxRows <= 0 {
		maxRows = upstreamSyncLogDefaultRetention
	}
	var ids []int64
	if err := DB.Model(&UpstreamSyncLog{}).
		Order("id desc").
		Offset(maxRows).
		Limit(1000).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := DB.Where("id IN ?", ids).Delete(&UpstreamSyncLog{})
	return result.RowsAffected, result.Error
}

func (log *UpstreamSyncLog) ToResponse() UpstreamSyncLogResponse {
	response := UpstreamSyncLogResponse{
		Id:              log.Id,
		StartedAt:       log.StartedAt,
		FinishedAt:      log.FinishedAt,
		Trigger:         log.Trigger,
		Status:          log.Status,
		BaseURL:         log.BaseUrl,
		PricingVersion:  log.PricingVersion,
		GroupsChecked:   log.GroupsChecked,
		ChannelsChanged: log.ChannelsChanged,
		AddedModels:     []string{},
		RemovedModels:   []string{},
		PriceChanges:    []UpstreamSyncPriceChange{},
		Errors:          []UpstreamSyncIssue{},
		DurationMs:      log.DurationMs,
		Message:         log.Message,
	}
	if log.AddedModels != "" {
		_ = common.UnmarshalJsonStr(log.AddedModels, &response.AddedModels)
	}
	if log.RemovedModels != "" {
		_ = common.UnmarshalJsonStr(log.RemovedModels, &response.RemovedModels)
	}
	if log.PriceChanges != "" {
		_ = common.UnmarshalJsonStr(log.PriceChanges, &response.PriceChanges)
	}
	if log.Errors != "" {
		_ = common.UnmarshalJsonStr(log.Errors, &response.Errors)
	}
	if response.AddedModels == nil {
		response.AddedModels = []string{}
	}
	if response.RemovedModels == nil {
		response.RemovedModels = []string{}
	}
	if response.PriceChanges == nil {
		response.PriceChanges = []UpstreamSyncPriceChange{}
	}
	if response.Errors == nil {
		response.Errors = []UpstreamSyncIssue{}
	}
	return response
}

func escapeUpstreamSyncLike(value string) string {
	value = strings.NewReplacer(
		"!", "!!",
		"%", "!%",
		"_", "!_",
	).Replace(value)
	return value
}
