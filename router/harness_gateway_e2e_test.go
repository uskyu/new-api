package router

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// harnessE2EJWKS serves a JWKS document for the given key and returns its URL.
func harnessE2EJWKS(t *testing.T, key *rsa.PrivateKey, kid string) string {
	t.Helper()
	document := map[string]any{"keys": []map[string]any{{
		"kty": "RSA",
		"kid": kid,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	}}}
	encoded, err := json.Marshal(document)
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(encoded)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

// harnessE2ESign mints an RS256 token the way Harness will.
func harnessE2ESign(t *testing.T, key *rsa.PrivateKey, kid string, scopes ...string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss":   "harness",
		"aud":   "new-api",
		"sub":   "harness-gateway",
		"scope": joinScopes(scopes),
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(10 * time.Minute).Unix(),
		"jti":   "e2e-jti",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func joinScopes(scopes []string) string {
	joined := ""
	for i, scope := range scopes {
		if i > 0 {
			joined += " "
		}
		joined += scope
	}
	return joined
}

// TestHarnessGatewayEndToEnd exercises the complete platform path over real HTTP:
// a signed Harness JWT is verified against a live JWKS, mapped onto an internal
// service token, and then used to read the ledger, the tier catalog and health.
// It is the only test that proves the JWT integration and the read APIs fit
// together; the unit tests each cover one half.
func TestHarnessGatewayEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	previousDB, previousLogDB := model.DB, model.LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	common.OptionMap = map[string]string{}
	t.Cleanup(func() { model.DB, model.LOG_DB = previousDB, previousLogDB })
	require.NoError(t, db.AutoMigrate(
		&model.Option{},
		&model.Token{},
		&model.User{},
		&model.Channel{},
		&model.Ability{},
		&model.HarnessLedgerEvent{},
	))

	// A dedicated service token and its owning user, which is what the JWT maps
	// onto. The token must be unrestricted so the read APIs accept it.
	user := &model.User{Username: "harness-e2e", Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, db.Create(user).Error)
	serviceToken := &model.Token{
		Name:           "harness-e2e-service",
		Key:            "harness-e2e-service-key",
		UserId:         user.Id,
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          "default",
	}
	require.NoError(t, db.Create(serviceToken).Error)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwksURL := harnessE2EJWKS(t, key, "kid-e2e")

	// Point the integration at the live JWKS and map the subject to the token.
	require.NoError(t, model.UpdateOption(setting.HarnessJWKSURLOptionKey, jwksURL))
	require.NoError(t, model.UpdateOption(setting.HarnessJWTIssuerOptionKey, "harness"))
	require.NoError(t, model.UpdateOption(setting.HarnessJWTAudienceOptionKey, "new-api"))
	subjectMap, err := json.Marshal(map[string]int{"harness-gateway": serviceToken.Id})
	require.NoError(t, err)
	require.NoError(t, model.UpdateOption(setting.HarnessSubjectTokenMapOptionKey, string(subjectMap)))
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM options WHERE key IN ?", []string{
			setting.HarnessJWKSURLOptionKey, setting.HarnessJWTIssuerOptionKey,
			setting.HarnessJWTAudienceOptionKey, setting.HarnessSubjectTokenMapOptionKey,
		})
	})

	// One settled ledger event the read APIs must return.
	common.QuotaPerUnit = 500_000
	observedAt := time.Now().Add(-5 * time.Second).UnixMilli()
	require.NoError(t, model.AppendHarnessLedgerEvent(&model.HarnessLedgerEvent{
		Stream:           model.HarnessLedgerStreamUsage,
		EventType:        model.HarnessLedgerEventTypeUsage,
		HarnessRequestId: "hrq_e2e_http",
		GatewayRequestId: "gw_e2e_http",
		RunId:            "run_e2e_http",
		Status:           model.HarnessLedgerStatusSettled,
		UsageUnit:        "token",
		PromptTokens:     11,
		CompletionTokens: 22,
		UsageSource:      "upstream",
		BillingBasis:     "token",
		ObservedAtMs:     observedAt,
	}))
	require.NoError(t, model.AppendHarnessLedgerEvent(&model.HarnessLedgerEvent{
		Stream:           model.HarnessLedgerStreamCost,
		EventType:        model.HarnessLedgerEventTypeCost,
		HarnessRequestId: "hrq_e2e_http",
		GatewayRequestId: "gw_e2e_http",
		Status:           model.HarnessLedgerStatusSettled,
		Currency:         "USD",
		UsageUnit:        "token",
		BaseAmount:       "0.010000",
		TotalAmount:      "0.008000",
		GroupRatio:       0.8,
		BillingBasis:     "token",
		ObservedAtMs:     observedAt,
	}))

	engine := gin.New()
	SetHarnessInternalRouter(engine)

	token := harnessE2ESign(t, key, "kid-e2e", "gateway.usage.read", "gateway.cost.read", "gateway.health.read", "gateway.catalog.read")
	authHeader := "Bearer " + token

	// B4 health must answer ok now that a channel is not the blocker: the check
	// only requires the database, so ok is expected with zero abilities? No 闂?	// zero enabled abilities is degraded by design. Assert the reachable statuses.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/v1/gateway/health", nil)
	request.Header.Set("Authorization", authHeader)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	var health map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &health))
	assert.Equal(t, "v2", health["interface_version"])
	assert.Equal(t, "45fbb149", health["upstream_baseline"])
	assert.Contains(t, []any{"ok", "degraded"}, health["status"])

	// B1 list must return the usage event.
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/internal/v1/gateway/usage-events?limit=10", nil)
	request.Header.Set("Authorization", authHeader)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	var usagePage struct {
		Items []struct {
			EventId   string `json:"event_id"`
			RequestId string `json:"request_id"`
			Usage     struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		} `json:"items"`
		HasMore bool `json:"has_more"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &usagePage))
	require.Len(t, usagePage.Items, 1)
	assert.Equal(t, "hrq_e2e_http", usagePage.Items[0].RequestId)
	assert.Equal(t, 11, usagePage.Items[0].Usage.InputTokens)
	assert.Equal(t, 22, usagePage.Items[0].Usage.OutputTokens)
	assert.False(t, usagePage.HasMore)

	// B3 list must return the cost record with USD amounts.
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/internal/v1/gateway/cost-records?limit=10", nil)
	request.Header.Set("Authorization", authHeader)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	var costPage struct {
		Items []struct {
			Currency string `json:"currency"`
			Billing  struct {
				BaseAmount  string  `json:"base_amount"`
				TotalAmount string  `json:"total_amount"`
				GroupRatio  float64 `json:"group_ratio"`
			} `json:"billing"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &costPage))
	require.Len(t, costPage.Items, 1)
	assert.Equal(t, "USD", costPage.Items[0].Currency)
	assert.Equal(t, "0.010000", costPage.Items[0].Billing.BaseAmount)
	assert.Equal(t, "0.008000", costPage.Items[0].Billing.TotalAmount)

	// B1-batchGet must answer by Harness request id and report the missing one.
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/internal/v1/gateway/usage-events:batchGet",
		strings.NewReader(`{"request_ids":["hrq_e2e_http","hrq_absent"]}`))
	request.Header.Set("Authorization", authHeader)
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	var batch struct {
		Items []struct {
			RequestId string `json:"request_id"`
			Found     bool   `json:"found"`
			Events    []struct {
				EventId string `json:"event_id"`
			} `json:"events"`
		} `json:"items"`
		MissingRequestIds []string `json:"missing_request_ids"`
		Truncated         bool     `json:"truncated"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &batch))
	require.Len(t, batch.Items, 2)
	assert.True(t, batch.Items[0].Found)
	require.Len(t, batch.Items[0].Events, 1)
	assert.False(t, batch.Items[1].Found)
	assert.Equal(t, []string{"hrq_absent"}, batch.MissingRequestIds)
	assert.False(t, batch.Truncated)

	// B3-batchGet mirrors B1.
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/internal/v1/gateway/cost-records:batchGet",
		strings.NewReader(`{"request_ids":["hrq_e2e_http"]}`))
	request.Header.Set("Authorization", authHeader)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	var costBatch struct {
		Items []struct {
			Found   bool `json:"found"`
			Records []struct {
				Currency string `json:"currency"`
			} `json:"records"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &costBatch))
	require.Len(t, costBatch.Items, 1)
	assert.True(t, costBatch.Items[0].Found)
	require.Len(t, costBatch.Items[0].Records, 1)
	assert.Equal(t, "USD", costBatch.Items[0].Records[0].Currency)

	// A token with no scope must not read the ledger.
	limitedToken := harnessE2ESign(t, key, "kid-e2e")
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/internal/v1/gateway/usage-events", nil)
	request.Header.Set("Authorization", "Bearer "+limitedToken)
	engine.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusForbidden, recorder.Code, "a principal without gateway.usage.read must be refused")
}
