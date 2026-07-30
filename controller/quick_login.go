package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type quickLoginExchangeRequest struct {
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
}

func StartQuickLogin(c *gin.Context) {
	setAuthNoStore(c)
	var input service.QuickLoginStartInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeQuickLoginError(c, service.ErrQuickLoginInvalidRequest)
		return
	}
	code, expiresAt, err := service.StartQuickLogin(input)
	if err != nil {
		writeQuickLoginError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"code":              code,
		"authorization_url": "/quick-login?code=" + code,
		"expires_at":        expiresAt,
	})
}

func GetQuickLoginAuthorization(c *gin.Context) {
	setAuthNoStore(c)
	authorization, err := service.GetQuickLoginAuthorization(strings.TrimSpace(c.Query("code")))
	if err != nil {
		writeQuickLoginError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"client_name":  authorization.ClientName,
		"redirect_uri": authorization.RedirectURI,
		"cancel_url":   authorization.CancelURL,
		"expires_at":   authorization.ExpiresAt,
	})
}

func ApproveQuickLogin(c *gin.Context) {
	setAuthNoStore(c)
	identity, ok := requireBrowserSession(c)
	if !ok {
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeQuickLoginError(c, service.ErrQuickLoginInvalidRequest)
		return
	}
	callbackURL, err := service.ApproveQuickLogin(strings.TrimSpace(request.Code), identity.UserID)
	if err != nil {
		writeQuickLoginError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"callback_url": callbackURL})
}

func ExchangeQuickLogin(c *gin.Context) {
	setAuthNoStore(c)
	var request quickLoginExchangeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeQuickLoginError(c, service.ErrQuickLoginInvalidRequest)
		return
	}
	token, err := service.ExchangeQuickLogin(strings.TrimSpace(request.Code), strings.TrimSpace(request.CodeVerifier))
	if err != nil {
		writeQuickLoginError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"token":      "sk-" + token.GetFullKey(),
		"token_type": "Bearer",
		"name":       token.Name,
		"group":      token.Group,
	})
}

func writeQuickLoginError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	code := "QUICK_LOGIN_INVALID"
	switch {
	case errors.Is(err, service.ErrQuickLoginInvalidRedirect):
		code = "QUICK_LOGIN_REDIRECT_INVALID"
	case errors.Is(err, service.ErrQuickLoginInvalidVerifier):
		code = "QUICK_LOGIN_VERIFIER_INVALID"
	case errors.Is(err, service.ErrQuickLoginNotApproved):
		status, code = http.StatusConflict, "QUICK_LOGIN_NOT_APPROVED"
	case errors.Is(err, service.ErrQuickLoginTokenLimit):
		status, code = http.StatusConflict, "QUICK_LOGIN_TOKEN_LIMIT"
	case errors.Is(err, service.ErrQuickLoginUserUnavailable):
		status, code = http.StatusForbidden, "QUICK_LOGIN_USER_UNAVAILABLE"
	case errors.Is(err, model.ErrAuthFlowExpired):
		code = "QUICK_LOGIN_EXPIRED"
	case errors.Is(err, model.ErrAuthFlowConsumed):
		status, code = http.StatusConflict, "QUICK_LOGIN_ALREADY_USED"
	case errors.Is(err, model.ErrAuthFlowApproved):
		status, code = http.StatusConflict, "QUICK_LOGIN_ALREADY_APPROVED"
	case errors.Is(err, model.ErrAuthFlowInvalid), errors.Is(err, service.ErrQuickLoginInvalidRequest):
		code = "QUICK_LOGIN_INVALID"
	default:
		common.SysError("quick login failed: " + err.Error())
		status, code = http.StatusInternalServerError, "QUICK_LOGIN_INTERNAL_ERROR"
	}
	c.JSON(status, gin.H{"success": false, "code": code, "message": http.StatusText(status)})
}
