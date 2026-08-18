package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetAgentSelfLeaderboardUsesCommonErrorWrapper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/api/agent/self/leaderboard", nil)
	ctx.Set("id", 0)

	GetAgentSelfLeaderboard(ctx)

	require.Equal(t, 200, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)
	require.Contains(t, recorder.Body.String(), "invalid leaderboard query")
}
