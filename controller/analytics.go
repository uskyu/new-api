package controller

import (
	"errors"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const maxAdminAnalyticsRange = 90 * 24 * time.Hour

func GetAdminAnalyticsOverview(c *gin.Context) {
	timeRange, err := parseAdminAnalyticsRange(c, time.Now())
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
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

func GetInactiveUserAnalytics(c *gin.Context) {
	days, err := strconv.Atoi(c.DefaultQuery("days", "30"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid inactive days")
		return
	}
	if days <= 0 || days > model.MaxInactiveAnalyticsDays {
		common.ApiErrorMsg(c, "inactive days must be between 1 and 3650")
		return
	}
	accountType := c.DefaultQuery("account_type", model.InactiveAccountTypeAll)
	pageInfo := common.GetPageQuery(c)
	result, err := model.GetInactiveUserAnalytics(pageInfo, model.InactiveUserAnalyticsOptions{
		Days:        days,
		Keyword:     c.Query("keyword"),
		AccountType: accountType,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func parseAdminAnalyticsRange(c *gin.Context, now time.Time) (model.AnalyticsRange, error) {
	rangeName := c.DefaultQuery("range", "7d")
	location := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	end := now.Unix()
	var start int64

	switch rangeName {
	case "today":
		start = today.Unix()
	case "yesterday":
		start = today.AddDate(0, 0, -1).Unix()
		end = today.Unix() - 1
	case "7d":
		start = today.AddDate(0, 0, -6).Unix()
	case "30d":
		start = today.AddDate(0, 0, -29).Unix()
	case "90d":
		start = today.AddDate(0, 0, -89).Unix()
	case "custom":
		var err error
		start, err = strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
		if err != nil || start <= 0 {
			return model.AnalyticsRange{}, errors.New("invalid start_timestamp")
		}
		end, err = strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
		if err != nil || end <= 0 {
			return model.AnalyticsRange{}, errors.New("invalid end_timestamp")
		}
	default:
		return model.AnalyticsRange{}, errors.New("invalid analytics range")
	}

	if end < start {
		return model.AnalyticsRange{}, errors.New("end_timestamp must not be earlier than start_timestamp")
	}
	if time.Duration(end-start)*time.Second > maxAdminAnalyticsRange {
		return model.AnalyticsRange{}, errors.New("analytics range must not exceed 90 days")
	}
	return model.AnalyticsRange{Range: rangeName, Start: start, End: end}, nil
}
