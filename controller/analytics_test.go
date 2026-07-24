package controller

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAdminAnalyticsRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, time.July, 24, 15, 30, 0, 0, time.FixedZone("CST", 8*3600))

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/api/admin/analytics/overview?range=7d", nil)
	timeRange, err := parseAdminAnalyticsRange(context, now)
	require.NoError(t, err)
	assert.Equal(t, "7d", timeRange.Range)
	assert.Equal(t, time.Date(2026, time.July, 18, 0, 0, 0, 0, now.Location()).Unix(), timeRange.Start)
	assert.Equal(t, now.Unix(), timeRange.End)

	context, _ = gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/api/admin/analytics/overview?range=custom&start_timestamp=100&end_timestamp=200", nil)
	timeRange, err = parseAdminAnalyticsRange(context, now)
	require.NoError(t, err)
	assert.Equal(t, int64(100), timeRange.Start)
	assert.Equal(t, int64(200), timeRange.End)
}

func TestParseAdminAnalyticsRangeRejectsOversizedCustomRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/api/admin/analytics/overview?range=custom&start_timestamp=1&end_timestamp=8000000", nil)

	_, err := parseAdminAnalyticsRange(context, time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "90 days")
}
