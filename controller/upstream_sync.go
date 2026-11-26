package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

type upstreamSyncConfigResponse struct {
	Enabled         bool   `json:"enabled"`
	IntervalMinutes int    `json:"interval_minutes"`
	LastRunAt       int64  `json:"last_run_at"`
	LastStatus      string `json:"last_status"`
	ActiveTaskID    string `json:"active_task_id"`
}

func buildUpstreamSyncConfigResponse() (upstreamSyncConfigResponse, error) {
	response := upstreamSyncConfigResponse{
		Enabled:         setting.GetUpstreamSyncEnabled(),
		IntervalMinutes: setting.GetUpstreamSyncIntervalMinutes(),
	}
	latest, err := model.GetLatestUpstreamSyncLog()
	if err != nil {
		return response, err
	}
	if latest != nil {
		response.LastRunAt = latest.FinishedAt
		if response.LastRunAt == 0 {
			response.LastRunAt = latest.StartedAt
		}
		response.LastStatus = latest.Status
	}
	active, err := model.GetActiveSystemTask(model.SystemTaskTypeUpstreamSync)
	if err != nil {
		return response, err
	}
	if active != nil {
		response.ActiveTaskID = active.TaskID
	}
	return response, nil
}

func GetUpstreamSyncConfig(c *gin.Context) {
	response, err := buildUpstreamSyncConfigResponse()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    response,
	})
}

func UpdateUpstreamSyncConfig(c *gin.Context) {
	var request struct {
		Enabled         *bool `json:"enabled"`
		IntervalMinutes *int  `json:"interval_minutes"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid parameters"})
		return
	}

	values := make(map[string]string, 2)
	if request.Enabled != nil {
		values[setting.UpstreamSyncEnabledOptionKey] = strconv.FormatBool(*request.Enabled)
	}
	if request.IntervalMinutes != nil {
		if *request.IntervalMinutes < setting.MinUpstreamSyncIntervalMinutes ||
			*request.IntervalMinutes > setting.MaxUpstreamSyncIntervalMinutes {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "interval_minutes must be between 1 and 1440",
			})
			return
		}
		values[setting.UpstreamSyncIntervalMinutesOptionKey] = strconv.Itoa(*request.IntervalMinutes)
	}
	if len(values) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "no upstream sync settings provided"})
		return
	}
	if err := model.UpdateOptionsBulk(values); err != nil {
		common.ApiError(c, err)
		return
	}

	response, err := buildUpstreamSyncConfigResponse()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    response,
	})
}

func ListUpstreamSyncLogs(c *gin.Context) {
	var query struct {
		Page      int    `form:"page" binding:"omitempty,gte=1"`
		PageSize  int    `form:"page_size" binding:"omitempty,gte=1,lte=100"`
		Status    string `form:"status" binding:"omitempty,oneof=success failed partial"`
		StartTime int64  `form:"start_time" binding:"omitempty,gte=0"`
		EndTime   int64  `form:"end_time" binding:"omitempty,gte=0"`
		Keyword   string `form:"q" binding:"max=200"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid upstream sync log filters"})
		return
	}
	if query.EndTime > 0 && query.StartTime > 0 && query.EndTime < query.StartTime {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "end_time must not be before start_time"})
		return
	}
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	logs, total, err := model.ListUpstreamSyncLogs(query.Page, query.PageSize, model.UpstreamSyncLogFilter{
		Status:    query.Status,
		Keyword:   strings.TrimSpace(query.Keyword),
		StartTime: query.StartTime,
		EndTime:   query.EndTime,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	responses := make([]model.UpstreamSyncLogResponse, 0, len(logs))
	for _, log := range logs {
		responses = append(responses, log.ToResponse())
	}
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "",
		"data":      responses,
		"total":     total,
		"page":      query.Page,
		"page_size": query.PageSize,
	})
}

func RunUpstreamSyncTask(c *gin.Context) {
	task, created, err := service.EnqueueSystemTask(model.SystemTaskTypeUpstreamSync, upstreamSyncTaskPayload{Manual: true})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"task_id": task.TaskID,
			"created": created,
		},
	})
}
