package service

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupQuickLoginTest(t *testing.T) *model.User {
	t.Helper()
	previousDB := model.DB
	previousSecret := common.SessionSecret
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.AuthFlow{}))
	model.DB = db
	common.SessionSecret = "quick-login-test-session-secret"
	t.Cleanup(func() {
		model.DB = previousDB
		common.SessionSecret = previousSecret
	})

	user := &model.User{
		Username: "quick-login-user",
		Password: "unused-password-hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "quick-login-user-aff",
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func quickLoginPKCE(t *testing.T) (string, string) {
	t.Helper()
	verifier := strings.Repeat("a", 43)
	digest := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(digest[:])
}

func startQuickLoginTestFlow(t *testing.T) (string, string) {
	t.Helper()
	verifier, challenge := quickLoginPKCE(t)
	code, _, err := StartQuickLogin(QuickLoginStartInput{
		ClientName:          "Desktop App",
		RedirectURI:         "http://127.0.0.1:18765/callback?source=test&code=stale&error=stale",
		State:               "random-client-state",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	return code, verifier
}

func TestQuickLoginIssuesUserTokenOnce(t *testing.T) {
	user := setupQuickLoginTest(t)
	code, verifier := startQuickLoginTestFlow(t)

	authorization, err := GetQuickLoginAuthorization(code)
	require.NoError(t, err)
	assert.Equal(t, "Desktop App", authorization.ClientName)
	cancelURL, err := url.Parse(authorization.CancelURL)
	require.NoError(t, err)
	assert.Equal(t, "access_denied", cancelURL.Query().Get("error"))
	assert.Equal(t, "random-client-state", cancelURL.Query().Get("state"))
	assert.Equal(t, "test", cancelURL.Query().Get("source"))
	assert.Empty(t, cancelURL.Query().Get("code"))

	callbackURL, err := ApproveQuickLogin(code, user.Id)
	require.NoError(t, err)
	callback, err := url.Parse(callbackURL)
	require.NoError(t, err)
	assert.Equal(t, code, callback.Query().Get("code"))
	assert.Equal(t, "random-client-state", callback.Query().Get("state"))
	assert.Empty(t, callback.Query().Get("error"))

	_, err = ExchangeQuickLogin(code, strings.Repeat("b", 43))
	require.ErrorIs(t, err, ErrQuickLoginInvalidVerifier)

	token, err := ExchangeQuickLogin(code, verifier)
	require.NoError(t, err)
	assert.Equal(t, user.Id, token.UserId)
	assert.Equal(t, "Quick login: Desktop App", token.Name)
	assert.Equal(t, "default", token.Group)
	assert.True(t, token.UnlimitedQuota)
	assert.Equal(t, int64(-1), token.ExpiredTime)

	var storedToken model.Token
	require.NoError(t, model.DB.Where("key = ?", token.Key).First(&storedToken).Error)
	assert.Equal(t, token.Id, storedToken.Id)

	_, err = ExchangeQuickLogin(code, verifier)
	require.ErrorIs(t, err, model.ErrAuthFlowConsumed)
}

func TestQuickLoginRejectsNonLoopbackRedirect(t *testing.T) {
	setupQuickLoginTest(t)
	_, challenge := quickLoginPKCE(t)
	_, _, err := StartQuickLogin(QuickLoginStartInput{
		ClientName:          "Desktop App",
		RedirectURI:         "https://example.com/callback",
		State:               "random-client-state",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.ErrorIs(t, err, ErrQuickLoginInvalidRedirect)
}

func TestQuickLoginCannotBeApprovedByAnotherUser(t *testing.T) {
	firstUser := setupQuickLoginTest(t)
	secondUser := &model.User{
		Username: "second-quick-login-user",
		Password: "unused-password-hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "second-quick-login-user-aff",
	}
	require.NoError(t, model.DB.Create(secondUser).Error)
	code, _ := startQuickLoginTestFlow(t)

	_, err := ApproveQuickLogin(code, firstUser.Id)
	require.NoError(t, err)
	_, err = ApproveQuickLogin(code, secondUser.Id)
	require.True(t, errors.Is(err, model.ErrAuthFlowApproved))
}

func TestQuickLoginDoesNotConsumeFlowWhenUserIsDisabled(t *testing.T) {
	user := setupQuickLoginTest(t)
	code, verifier := startQuickLoginTestFlow(t)
	_, err := ApproveQuickLogin(code, user.Id)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("status", common.UserStatusDisabled).Error)

	_, err = ExchangeQuickLogin(code, verifier)
	require.ErrorIs(t, err, ErrQuickLoginUserUnavailable)

	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("status", common.UserStatusEnabled).Error)
	_, err = ExchangeQuickLogin(code, verifier)
	require.NoError(t, err)
}
