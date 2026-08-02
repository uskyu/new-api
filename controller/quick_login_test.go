package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestQuickLoginAPIContract(t *testing.T) {
	previousDB := model.DB
	previousSecret := common.SessionSecret
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.AuthFlow{}))
	model.DB = db
	common.SessionSecret = "quick-login-controller-test-secret"
	t.Cleanup(func() {
		model.DB = previousDB
		common.SessionSecret = previousSecret
	})

	user := model.User{
		Username: "quick-login-api-user",
		Password: "unused-password-hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "quick-login-api-user-aff",
	}
	require.NoError(t, db.Create(&user).Error)

	verifier := strings.Repeat("a", 43)
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/quick-login/start", StartQuickLogin)
	router.POST("/api/quick-login/exchange", ExchangeQuickLogin)

	startBody, err := common.Marshal(service.QuickLoginStartInput{
		ClientName:          "Desktop App",
		RedirectURI:         "http://127.0.0.1:18765/callback",
		State:               "desktop-state",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	startResponse := performQuickLoginJSONRequest(t, router, "/api/quick-login/start", startBody)
	assert.Equal(t, http.StatusOK, startResponse.Code)

	var started struct {
		Success bool `json:"success"`
		Data    struct {
			Code             string `json:"code"`
			AuthorizationURL string `json:"authorization_url"`
			ExpiresAt        int64  `json:"expires_at"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(startResponse.Body.Bytes(), &started))
	require.True(t, started.Success)
	require.NotEmpty(t, started.Data.Code)
	assert.Equal(t, "/quick-login?code="+started.Data.Code, started.Data.AuthorizationURL)
	assert.Positive(t, started.Data.ExpiresAt)

	_, err = service.ApproveQuickLogin(started.Data.Code, user.Id)
	require.NoError(t, err)
	exchangeBody, err := common.Marshal(quickLoginExchangeRequest{
		Code:         started.Data.Code,
		CodeVerifier: verifier,
	})
	require.NoError(t, err)
	exchangeResponse := performQuickLoginJSONRequest(t, router, "/api/quick-login/exchange", exchangeBody)
	assert.Equal(t, http.StatusOK, exchangeResponse.Code)

	var exchanged struct {
		Success bool `json:"success"`
		Data    struct {
			Token     string `json:"token"`
			TokenType string `json:"token_type"`
			Name      string `json:"name"`
			Group     string `json:"group"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(exchangeResponse.Body.Bytes(), &exchanged))
	require.True(t, exchanged.Success)
	assert.True(t, strings.HasPrefix(exchanged.Data.Token, "sk-"))
	assert.Equal(t, "Bearer", exchanged.Data.TokenType)
	assert.Equal(t, "Quick login: Desktop App", exchanged.Data.Name)
	assert.Equal(t, "default", exchanged.Data.Group)
}

func performQuickLoginJSONRequest(t *testing.T, router http.Handler, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
