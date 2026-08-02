package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

const QuickLoginFlowTTL = 5 * time.Minute

var (
	ErrQuickLoginInvalidRequest  = errors.New("quick login request is invalid")
	ErrQuickLoginInvalidRedirect = errors.New("quick login redirect must use a loopback address")
	ErrQuickLoginInvalidVerifier = errors.New("quick login verifier is invalid")
	ErrQuickLoginNotApproved     = errors.New("quick login request has not been approved")
	ErrQuickLoginTokenLimit      = errors.New("quick login token limit reached")
	ErrQuickLoginUserUnavailable = errors.New("quick login user is unavailable")
)

type QuickLoginStartInput struct {
	ClientName          string `json:"client_name"`
	RedirectURI         string `json:"redirect_uri"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
}

type QuickLoginFlowPayload struct {
	ClientName    string `json:"client_name"`
	RedirectURI   string `json:"redirect_uri"`
	State         string `json:"state"`
	CodeChallenge string `json:"code_challenge"`
}

type QuickLoginAuthorization struct {
	ClientName  string
	RedirectURI string
	CancelURL   string
	ExpiresAt   int64
}

func StartQuickLogin(input QuickLoginStartInput) (string, int64, error) {
	payload, err := normalizeQuickLoginInput(input)
	if err != nil {
		return "", 0, err
	}
	payloadBytes, err := common.Marshal(payload)
	if err != nil {
		return "", 0, err
	}
	expiresAt := time.Now().Add(QuickLoginFlowTTL)
	code, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeQuickLogin,
		Payload:   string(payloadBytes),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return "", 0, err
	}
	return code, expiresAt.Unix(), nil
}

func GetQuickLoginAuthorization(code string) (*QuickLoginAuthorization, error) {
	flow, err := model.GetAuthFlow(code, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeQuickLogin})
	if err != nil {
		return nil, err
	}
	payload, err := decodeQuickLoginPayload(flow.Payload)
	if err != nil {
		return nil, err
	}
	cancelURL, err := quickLoginCallbackURL(payload, "", "access_denied")
	if err != nil {
		return nil, err
	}
	return &QuickLoginAuthorization{
		ClientName:  payload.ClientName,
		RedirectURI: payload.RedirectURI,
		CancelURL:   cancelURL,
		ExpiresAt:   flow.ExpiresAt.Unix(),
	}, nil
}

func ApproveQuickLogin(code string, userId int) (string, error) {
	flow, err := model.ApproveAuthFlow(code, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeQuickLogin}, userId)
	if err != nil {
		return "", err
	}
	payload, err := decodeQuickLoginPayload(flow.Payload)
	if err != nil {
		return "", err
	}
	return quickLoginCallbackURL(payload, code, "")
}

func ExchangeQuickLogin(code, verifier string) (*model.Token, error) {
	var issuedToken *model.Token
	_, err := model.ConsumeAuthFlowWithAction(code, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeQuickLogin}, func(tx *gorm.DB, flow *model.AuthFlow) error {
		if flow.UserId <= 0 {
			return ErrQuickLoginNotApproved
		}
		payload, err := decodeQuickLoginPayload(flow.Payload)
		if err != nil {
			return err
		}
		if !verifyQuickLoginPKCE(verifier, payload.CodeChallenge) {
			return ErrQuickLoginInvalidVerifier
		}

		user, err := model.GetUserForQuickLoginTokenIssue(tx, flow.UserId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuickLoginUserUnavailable
			}
			return err
		}
		if user.Status != common.UserStatusEnabled {
			return ErrQuickLoginUserUnavailable
		}
		var tokenCount int64
		if err := tx.Model(&model.Token{}).Where("user_id = ?", user.Id).Count(&tokenCount).Error; err != nil {
			return err
		}
		if tokenCount >= int64(operation_setting.GetMaxUserTokens()) {
			return ErrQuickLoginTokenLimit
		}
		key, err := common.GenerateKey()
		if err != nil {
			return err
		}
		now := common.GetTimestamp()
		issuedToken = &model.Token{
			UserId:         user.Id,
			Key:            key,
			Status:         common.TokenStatusEnabled,
			Name:           "Quick login: " + payload.ClientName,
			CreatedTime:    now,
			AccessedTime:   now,
			ExpiredTime:    -1,
			UnlimitedQuota: true,
			Group:          user.Group,
		}
		return tx.Create(issuedToken).Error
	})
	if err != nil {
		return nil, err
	}
	return issuedToken, nil
}

func normalizeQuickLoginInput(input QuickLoginStartInput) (*QuickLoginFlowPayload, error) {
	clientName := strings.TrimSpace(input.ClientName)
	state := input.State
	challenge := strings.TrimSpace(input.CodeChallenge)
	if clientName == "" || len(clientName) > 32 || strings.TrimSpace(state) == "" || len(state) > 256 || input.CodeChallengeMethod != "S256" {
		return nil, ErrQuickLoginInvalidRequest
	}
	for _, char := range clientName {
		if unicode.IsControl(char) {
			return nil, ErrQuickLoginInvalidRequest
		}
	}
	challengeBytes, err := base64.RawURLEncoding.DecodeString(challenge)
	if err != nil || len(challengeBytes) != sha256.Size {
		return nil, ErrQuickLoginInvalidRequest
	}
	redirectURI, err := validateQuickLoginRedirect(input.RedirectURI)
	if err != nil {
		return nil, err
	}
	return &QuickLoginFlowPayload{
		ClientName:    clientName,
		RedirectURI:   redirectURI,
		State:         state,
		CodeChallenge: challenge,
	}, nil
}

func validateQuickLoginRedirect(raw string) (string, error) {
	redirect, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || redirect.Scheme != "http" || redirect.Host == "" || redirect.User != nil || redirect.Fragment != "" {
		return "", ErrQuickLoginInvalidRedirect
	}
	hostname := strings.ToLower(redirect.Hostname())
	if hostname != "localhost" {
		ip := net.ParseIP(hostname)
		if ip == nil || !ip.IsLoopback() {
			return "", ErrQuickLoginInvalidRedirect
		}
	}
	return redirect.String(), nil
}

func decodeQuickLoginPayload(raw string) (*QuickLoginFlowPayload, error) {
	var payload QuickLoginFlowPayload
	if err := common.UnmarshalJsonStr(raw, &payload); err != nil {
		return nil, ErrQuickLoginInvalidRequest
	}
	return normalizeQuickLoginInput(QuickLoginStartInput{
		ClientName:          payload.ClientName,
		RedirectURI:         payload.RedirectURI,
		State:               payload.State,
		CodeChallenge:       payload.CodeChallenge,
		CodeChallengeMethod: "S256",
	})
}

func quickLoginCallbackURL(payload *QuickLoginFlowPayload, code, callbackError string) (string, error) {
	callback, err := url.Parse(payload.RedirectURI)
	if err != nil {
		return "", ErrQuickLoginInvalidRedirect
	}
	query := callback.Query()
	query.Del("code")
	query.Del("error")
	query.Set("state", payload.State)
	if code != "" {
		query.Set("code", code)
	}
	if callbackError != "" {
		query.Set("error", callbackError)
	}
	callback.RawQuery = query.Encode()
	return callback.String(), nil
}

func verifyQuickLoginPKCE(verifier, expectedChallenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	for _, char := range verifier {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("-._~", char)) {
			return false
		}
	}
	digest := sha256.Sum256([]byte(verifier))
	actualChallenge := base64.RawURLEncoding.EncodeToString(digest[:])
	return hmac.Equal([]byte(actualChallenge), []byte(expectedChallenge))
}
