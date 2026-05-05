package controller

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	analyticsRangeToday     = "today"
	analyticsRangeYesterday = "yesterday"
	analyticsRangeWeek      = "week"
	analyticsRangeMonth     = "month"
	analyticsRangeCustom    = "custom"
)

func parseAdminAnalyticsTimeRange(c *gin.Context) (model.AnalyticsRange, error) {
	rangeName := strings.TrimSpace(c.DefaultQuery("range", analyticsRangeToday))
	if rangeName == "" {
		rangeName = analyticsRangeToday
	}

	now := time.Now()
	location := now.Location()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)

	var startTime, endTime time.Time
	switch rangeName {
	case analyticsRangeToday:
		startTime = todayStart
		endTime = todayStart.AddDate(0, 0, 1).Add(-time.Second)
	case analyticsRangeYesterday:
		startTime = todayStart.AddDate(0, 0, -1)
		endTime = todayStart.Add(-time.Second)
	case analyticsRangeWeek:
		weekday := int(todayStart.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startTime = todayStart.AddDate(0, 0, -(weekday - 1))
		endTime = todayStart.AddDate(0, 0, 1).Add(-time.Second)
	case analyticsRangeMonth:
		startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
		endTime = todayStart.AddDate(0, 0, 1).Add(-time.Second)
	case analyticsRangeCustom:
		start, err := strconv.ParseInt(c.Query("start"), 10, 64)
		if err != nil || start <= 0 {
			return model.AnalyticsRange{}, errInvalidAnalyticsRange
		}
		end, err := strconv.ParseInt(c.Query("end"), 10, 64)
		if err != nil || end <= 0 || end < start {
			return model.AnalyticsRange{}, errInvalidAnalyticsRange
		}
		return model.AnalyticsRange{
			Range: rangeName,
			Start: start,
			End:   end,
		}, nil
	default:
		return model.AnalyticsRange{}, errInvalidAnalyticsRange
	}

	return model.AnalyticsRange{
		Range: rangeName,
		Start: startTime.Unix(),
		End:   endTime.Unix(),
	}, nil
}

var errInvalidAnalyticsRange = analyticsRequestError("invalid analytics range")

type analyticsRequestError string

func (e analyticsRequestError) Error() string {
	return string(e)
}

func GetAdminAnalyticsOverview(c *gin.Context) {
	timeRange, err := parseAdminAnalyticsTimeRange(c)
	if err != nil {
		common.ApiErrorMsg(c, "invalid time range")
		return
	}

	limit := 10
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			common.ApiErrorMsg(c, "invalid ranking limit")
			return
		}
		if parsedLimit > 100 {
			parsedLimit = 100
		}
		limit = parsedLimit
	}

	overview, err := model.GetAdminAnalyticsOverview(model.AnalyticsOverviewOptions{
		Range: timeRange,
		Limit: limit,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, overview)
}
