package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupportPermissionUsesStatelessSessionWithoutGrantingAdmin(t *testing.T) {
	setupDashboardAuthMiddlewareTest(t)
	gin.SetMode(gin.TestMode)
	user := &model.User{
		Username: "support-agent-permission", Password: "password-placeholder",
		Role: common.RoleSupportUser, Status: common.UserStatusEnabled, Group: "default",
		AffCode: "support-agent-permission-aff",
	}
	require.NoError(t, model.DB.Create(user).Error)
	now := time.Now().Unix()
	session := &model.UserSession{
		SID: "support-agent-session", UserID: user.Id, Version: 1,
		UserAuthVersion: user.AuthVersion, Status: model.UserSessionStatusActive,
		RefreshHash: "refresh-hash", LoginMethod: "password", LastActiveAt: now, ExpiresAt: now + 3600,
	}
	require.NoError(t, model.CreateUserSession(session))
	identity := service.AuthIdentity{
		UserID: user.Id, SessionID: session.SID,
		UserAuthVersion: session.UserAuthVersion, SessionVersion: session.Version,
	}
	token, _, err := service.IssueAccessToken(identity)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/agent", PermissionAuth(common.PermissionAgentDownlineTransfer), func(c *gin.Context) {
		assert.Equal(t, user.Id, c.GetInt("id"))
		assert.Equal(t, user.AuthVersion, c.GetInt64("auth_version"))
		c.Status(http.StatusNoContent)
	})
	router.GET("/agent/assign", PermissionAuth(common.PermissionAgentDownlineAssign), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/agent/adjust", PermissionAuth(common.PermissionAgentBalanceAdjust), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/admin", AdminAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodGet, "/agent", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusNoContent, response.Code)

	request = httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusForbidden, response.Code)

	for _, path := range []string{"/agent/assign", "/agent/adjust"} {
		request = httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response = httptest.NewRecorder()
		router.ServeHTTP(response, request)
		assert.Equal(t, http.StatusForbidden, response.Code, path)
	}

	_, err = model.BumpUserAuthVersion(user.Id)
	require.NoError(t, err)
	request = httptest.NewRequest(http.MethodGet, "/agent", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusUnauthorized, response.Code)
}
