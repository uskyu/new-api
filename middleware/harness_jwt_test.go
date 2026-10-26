package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func setupHarnessJWTTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	model.DB = db
	model.LOG_DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
	})
	return db
}

// resetHarnessJWKSCache isolates tests: the JWKS cache is process-wide and
// keyed by URL, while httptest servers reuse ephemeral ports between runs.
func resetHarnessJWKSCache() {
	harnessJWKSCacheMu.Lock()
	harnessJWKSCache = make(map[string]*harnessJWKSKeySet)
	harnessJWKSCacheMu.Unlock()
}

func setupHarnessJWTOptions(t *testing.T, jwksURL string, subjectMap string) {
	t.Helper()
	resetHarnessJWKSCache()
	t.Cleanup(resetHarnessJWKSCache)
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{
		setting.HarnessJWKSURLOptionKey:         jwksURL,
		setting.HarnessJWTIssuerOptionKey:       "harness",
		setting.HarnessJWTAudienceOptionKey:     "new-api",
		setting.HarnessSubjectTokenMapOptionKey: subjectMap,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

type harnessTestSigner struct {
	key *rsa.PrivateKey
	kid string
}

func newHarnessTestSigner(t *testing.T, kid string) harnessTestSigner {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return harnessTestSigner{key: key, kid: kid}
}

func (signer harnessTestSigner) token(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	return signer.tokenWithKid(t, signer.kid, claims)
}

func (signer harnessTestSigner) tokenWithKid(t *testing.T, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(signer.key)
	require.NoError(t, err)
	return signed
}

func (signer harnessTestSigner) jwks() string {
	exponent := big.NewInt(int64(signer.key.PublicKey.E)).Bytes()
	return fmt.Sprintf(
		`{"keys":[{"kid":%q,"kty":"RSA","alg":"RS256","use":"sig","n":%q,"e":%q}]}`,
		signer.kid,
		base64.RawURLEncoding.EncodeToString(signer.key.PublicKey.N.Bytes()),
		base64.RawURLEncoding.EncodeToString(exponent),
	)
}

type harnessTestJWKS struct {
	server  *httptest.Server
	mu      sync.Mutex
	body    string
	fetches int
}

func newHarnessTestJWKS(t *testing.T, body string) *harnessTestJWKS {
	t.Helper()
	fixture := &harnessTestJWKS{body: body}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fixture.mu.Lock()
		fixture.fetches++
		current := fixture.body
		fixture.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(current))
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (fixture *harnessTestJWKS) setBody(body string) {
	fixture.mu.Lock()
	fixture.body = body
	fixture.mu.Unlock()
}

func (fixture *harnessTestJWKS) fetchCount() int {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return fixture.fetches
}

func harnessTestClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":   "harness",
		"aud":   []string{"new-api"},
		"sub":   "harness-gateway",
		"scope": "chat.write models.read",
		"jti":   "jti-test-1",
		"exp":   time.Now().Add(10 * time.Minute).Unix(),
	}
}

func createHarnessServiceToken(t *testing.T, db *gorm.DB, name string, status int) *model.Token {
	t.Helper()
	token := &model.Token{
		UserId: 1, Key: "harness-key-" + name, Name: name, Status: status,
		CreatedTime: time.Now().Unix(),
	}
	require.NoError(t, db.Create(token).Error)
	return token
}

func runHarnessJWT(t *testing.T, authorization string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	if authorization != "" {
		c.Request.Header.Set("Authorization", authorization)
	}
	HarnessJWT()(c)
	return c, recorder
}

func TestHarnessJWTMapsVerifiedTokenOntoServiceToken(t *testing.T) {
	db := setupHarnessJWTTestDB(t)
	serviceToken := createHarnessServiceToken(t, db, "harness-gateway", common.TokenStatusEnabled)
	signer := newHarnessTestSigner(t, "kid-1")
	jwks := newHarnessTestJWKS(t, signer.jwks())
	setupHarnessJWTOptions(t, jwks.server.URL, fmt.Sprintf(`{"harness-gateway": %d}`, serviceToken.Id))

	c, recorder := runHarnessJWT(t, "Bearer "+signer.token(t, harnessTestClaims()))

	assert.False(t, c.IsAborted())
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "Bearer sk-"+serviceToken.Key, c.Request.Header.Get("Authorization"))
	principal, ok := GetHarnessPrincipal(c)
	require.True(t, ok)
	assert.Equal(t, "harness-gateway", principal.Subject)
	assert.Equal(t, []string{"chat.write", "models.read"}, principal.Scopes)
	assert.Equal(t, "jti-test-1", principal.JTI)
	assert.Equal(t, serviceToken.Id, principal.TokenID)
	assert.Equal(t, 1, jwks.fetchCount())
}

func TestHarnessJWTResolvesSubjectByTokenName(t *testing.T) {
	db := setupHarnessJWTTestDB(t)
	serviceToken := createHarnessServiceToken(t, db, "harness gateway", common.TokenStatusEnabled)
	signer := newHarnessTestSigner(t, "kid-name")
	jwks := newHarnessTestJWKS(t, signer.jwks())
	setupHarnessJWTOptions(t, jwks.server.URL, `{"harness-gateway": "harness gateway"}`)

	c, _ := runHarnessJWT(t, "Bearer "+signer.token(t, harnessTestClaims()))

	assert.False(t, c.IsAborted())
	assert.Equal(t, "Bearer sk-"+serviceToken.Key, c.Request.Header.Get("Authorization"))
	principal, ok := GetHarnessPrincipal(c)
	require.True(t, ok)
	assert.Equal(t, serviceToken.Id, principal.TokenID)
}

func TestHarnessJWTRejectsInvalidTokens(t *testing.T) {
	db := setupHarnessJWTTestDB(t)
	serviceToken := createHarnessServiceToken(t, db, "harness-gateway", common.TokenStatusEnabled)
	signer := newHarnessTestSigner(t, "kid-reject")
	otherSigner := newHarnessTestSigner(t, "kid-reject")
	jwks := newHarnessTestJWKS(t, signer.jwks())
	setupHarnessJWTOptions(t, jwks.server.URL, fmt.Sprintf(`{"harness-gateway": %d}`, serviceToken.Id))

	hmacSigner := jwt.NewWithClaims(jwt.SigningMethodHS256, harnessTestClaims())
	hmacSigner.Header["kid"] = signer.kid
	hmacToken, err := hmacSigner.SignedString([]byte("harness-test-hmac-secret"))
	require.NoError(t, err)

	for _, testCase := range []struct {
		name          string
		authorization string
	}{
		{
			name: "expired",
			authorization: "Bearer " + signer.token(t, func() jwt.MapClaims {
				claims := harnessTestClaims()
				claims["exp"] = time.Now().Add(-30 * time.Minute).Unix()
				return claims
			}()),
		},
		{
			name: "wrong signature",
			authorization: "Bearer " + otherSigner.token(t, func() jwt.MapClaims {
				claims := harnessTestClaims()
				claims["sub"] = "harness-gateway"
				return claims
			}()),
		},
		{
			name: "wrong audience",
			authorization: "Bearer " + signer.token(t, func() jwt.MapClaims {
				claims := harnessTestClaims()
				claims["aud"] = []string{"other-service"}
				return claims
			}()),
		},
		{
			name: "wrong issuer",
			authorization: "Bearer " + signer.token(t, func() jwt.MapClaims {
				claims := harnessTestClaims()
				claims["iss"] = "not-harness"
				return claims
			}()),
		},
		{
			name:          "hs256",
			authorization: "Bearer " + hmacToken,
		},
		{
			name: "unmapped subject",
			authorization: "Bearer " + signer.token(t, func() jwt.MapClaims {
				claims := harnessTestClaims()
				claims["sub"] = "unknown-gateway"
				return claims
			}()),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			c, recorder := runHarnessJWT(t, testCase.authorization)
			assert.True(t, c.IsAborted())
			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
			// The code is part of the contract: a client branches on it to tell a
			// refused platform credential from an unrelated gateway error.
			assertHarnessUnauthorizedBody(t, recorder.Body.Bytes())
			assert.NotEqual(t, "Bearer sk-"+serviceToken.Key, c.Request.Header.Get("Authorization"))
			_, ok := GetHarnessPrincipal(c)
			assert.False(t, ok)
		})
	}
}

func TestHarnessJWTRejectsDisabledServiceToken(t *testing.T) {
	db := setupHarnessJWTTestDB(t)
	serviceToken := createHarnessServiceToken(t, db, "harness-disabled", common.TokenStatusDisabled)
	signer := newHarnessTestSigner(t, "kid-disabled")
	jwks := newHarnessTestJWKS(t, signer.jwks())
	setupHarnessJWTOptions(t, jwks.server.URL, fmt.Sprintf(`{"harness-gateway": %d}`, serviceToken.Id))

	c, recorder := runHarnessJWT(t, "Bearer "+signer.token(t, harnessTestClaims()))

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	_, ok := GetHarnessPrincipal(c)
	assert.False(t, ok)
}

func TestHarnessJWTRefreshesJWKSForUnknownKid(t *testing.T) {
	db := setupHarnessJWTTestDB(t)
	serviceToken := createHarnessServiceToken(t, db, "harness-gateway", common.TokenStatusEnabled)
	firstSigner := newHarnessTestSigner(t, "kid-old")
	rotatedSigner := newHarnessTestSigner(t, "kid-new")
	jwks := newHarnessTestJWKS(t, firstSigner.jwks())
	setupHarnessJWTOptions(t, jwks.server.URL, fmt.Sprintf(`{"harness-gateway": %d}`, serviceToken.Id))

	c, _ := runHarnessJWT(t, "Bearer "+firstSigner.token(t, harnessTestClaims()))
	require.False(t, c.IsAborted())
	require.Equal(t, 1, jwks.fetchCount())

	jwks.setBody(rotatedSigner.jwks())
	c, recorder := runHarnessJWT(t, "Bearer "+rotatedSigner.token(t, harnessTestClaims()))

	assert.False(t, c.IsAborted())
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "Bearer sk-"+serviceToken.Key, c.Request.Header.Get("Authorization"))
	assert.Equal(t, 2, jwks.fetchCount(), "unknown kid must force a JWKS refresh")
}

func TestHarnessJWTThrottlesRepeatedUnknownKidRefreshes(t *testing.T) {
	db := setupHarnessJWTTestDB(t)
	serviceToken := createHarnessServiceToken(t, db, "harness-gateway", common.TokenStatusEnabled)
	signer := newHarnessTestSigner(t, "kid-known")
	jwks := newHarnessTestJWKS(t, signer.jwks())
	setupHarnessJWTOptions(t, jwks.server.URL, fmt.Sprintf(`{"harness-gateway": %d}`, serviceToken.Id))

	c, _ := runHarnessJWT(t, "Bearer "+signer.token(t, harnessTestClaims()))
	require.False(t, c.IsAborted())
	require.Equal(t, 1, jwks.fetchCount())

	unknownKid := signer.tokenWithKid(t, "kid-never-published", harnessTestClaims())
	for range 3 {
		c, recorder := runHarnessJWT(t, "Bearer "+unknownKid)
		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	}
	assert.Equal(t, 2, jwks.fetchCount(), "repeated unknown kids must not force one fetch per request")
}

func TestHarnessJWTPassesThroughNonJWTRequests(t *testing.T) {
	db := setupHarnessJWTTestDB(t)
	serviceToken := createHarnessServiceToken(t, db, "harness-gateway", common.TokenStatusEnabled)
	signer := newHarnessTestSigner(t, "kid-passthrough")
	jwks := newHarnessTestJWKS(t, signer.jwks())
	setupHarnessJWTOptions(t, jwks.server.URL, fmt.Sprintf(`{"harness-gateway": %d}`, serviceToken.Id))

	for _, authorization := range []string{
		"",
		"Bearer sk-" + serviceToken.Key,
		"Bearer " + serviceToken.Key,
		"Bearer not.a.jwt",
		"Basic dXNlcjpwYXNz",
	} {
		c, recorder := runHarnessJWT(t, authorization)
		assert.False(t, c.IsAborted(), authorization)
		assert.Equal(t, http.StatusOK, recorder.Code, authorization)
		assert.Equal(t, authorization, c.Request.Header.Get("Authorization"), authorization)
		_, ok := GetHarnessPrincipal(c)
		assert.False(t, ok, authorization)
	}
	assert.Equal(t, 0, jwks.fetchCount(), "non-JWT requests must not reach the JWKS endpoint")
}

func TestHarnessJWTPassesThroughWhenDisabled(t *testing.T) {
	signer := newHarnessTestSigner(t, "kid-disabled-feature")
	setupHarnessJWTOptions(t, "", `{"harness-gateway": 1}`)

	c, recorder := runHarnessJWT(t, "Bearer "+signer.token(t, harnessTestClaims()))

	assert.False(t, c.IsAborted())
	assert.Equal(t, http.StatusOK, recorder.Code)
	_, ok := GetHarnessPrincipal(c)
	assert.False(t, ok)
}

func TestRequireHarnessScopeRejectsMissingScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	called := false
	RequireHarnessScope("chat.write")(c)
	if !c.IsAborted() {
		called = true
		c.Next()
	}
	assert.True(t, called, "non-harness requests must pass through")
	assert.Equal(t, http.StatusOK, recorder.Code)

	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(HarnessPrincipalContextKey, HarnessPrincipal{Subject: "harness-gateway", Scopes: []string{"models.read"}})
	RequireHarnessScope("chat.write")(c)
	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, recorder.Code)

	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(HarnessPrincipalContextKey, HarnessPrincipal{Subject: "harness-gateway", Scopes: []string{"chat.write"}})
	RequireHarnessScope("chat.write")(c)
	assert.False(t, c.IsAborted())
}

// TestRequireHarnessChatScopeGatesModelCalls pins the contract's model-call
// permission. gateway.chat authorises A1/A2/A4/A9, so a Harness principal must
// hold it to reach a relay route; a principal holding only the read scopes is
// refused, and a non-Harness caller is untouched.
func TestRequireHarnessChatScopeGatesModelCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newContext := func() (*gin.Context, *httptest.ResponseRecorder) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		return c, recorder
	}

	for _, tc := range []struct {
		name      string
		principal *HarnessPrincipal
		wantCode  int
		wantPass  bool
	}{
		{
			name:      "harness principal with gateway.chat passes",
			principal: &HarnessPrincipal{Subject: "harness-gateway", Scopes: []string{HarnessChatScope}},
			wantCode:  http.StatusOK,
			wantPass:  true,
		},
		{
			name:      "harness principal with gateway.chat among others passes",
			principal: &HarnessPrincipal{Subject: "harness-gateway", Scopes: []string{"gateway.catalog.read", HarnessChatScope}},
			wantCode:  http.StatusOK,
			wantPass:  true,
		},
		{
			name:      "harness principal without any scope is refused",
			principal: &HarnessPrincipal{Subject: "harness-gateway"},
			wantCode:  http.StatusForbidden,
		},
		{
			name:      "harness principal with only read scopes is refused",
			principal: &HarnessPrincipal{Subject: "harness-gateway", Scopes: []string{"gateway.usage.read", "gateway.cost.read", "gateway.catalog.read", "gateway.health.read"}},
			wantCode:  http.StatusForbidden,
		},
		{
			name:      "harness principal with a lookalike scope is refused",
			principal: &HarnessPrincipal{Subject: "harness-gateway", Scopes: []string{"gateway.chat.write", "chat"}},
			wantCode:  http.StatusForbidden,
		},
		{
			name:      "non-harness caller passes through untouched",
			principal: nil,
			wantCode:  http.StatusOK,
			wantPass:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := newContext()
			if tc.principal != nil {
				c.Set(HarnessPrincipalContextKey, *tc.principal)
			}

			reached := false
			handler := RequireHarnessScope(HarnessChatScope)
			handler(c)
			if !c.IsAborted() {
				reached = true
			}

			assert.Equal(t, tc.wantPass, reached, "handler reachability")
			assert.Equal(t, tc.wantCode, recorder.Code)
		})
	}
}

func TestRequestIdCorrelatesHarnessHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set(HarnessRequestIdHeader, "hrq_01J8TESTREQUESTID")
	c.Request.Header.Set(HarnessRunIdHeader, "run_01J8TESTRUNID")

	RequestId()(c)

	assert.Equal(t, "hrq_01J8TESTREQUESTID", GetHarnessRequestId(c))
	assert.Equal(t, "run_01J8TESTRUNID", GetHarnessRunId(c))
	assert.Equal(t, "hrq_01J8TESTREQUESTID", c.Request.Context().Value(HarnessRequestIdContextKey))
	assert.Equal(t, "run_01J8TESTRUNID", c.Request.Context().Value(HarnessRunIdContextKey))
	requestId := c.GetString(common.RequestIdKey)
	assert.NotEmpty(t, requestId)
	assert.Equal(t, requestId, recorder.Header().Get(common.RequestIdKey))
	assert.Equal(t, requestId, recorder.Header().Get(NewAPIRequestIdHeader))
}

func TestRequestIdIgnoresMalformedHarnessHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, testCase := range []struct {
		name      string
		requestId string
		runId     string
	}{
		{name: "space", requestId: "hrq bad", runId: "run bad"},
		{name: "too long", requestId: "hrq_" + strings.Repeat("a", 80), runId: "run_" + strings.Repeat("b", 80)},
		{name: "punctuation", requestId: "hrq_<script>", runId: "run.01J8"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			c.Request.Header.Set(HarnessRequestIdHeader, testCase.requestId)
			c.Request.Header.Set(HarnessRunIdHeader, testCase.runId)

			RequestId()(c)

			assert.Empty(t, GetHarnessRequestId(c))
			assert.Empty(t, GetHarnessRunId(c))
			assert.NotEmpty(t, recorder.Header().Get(NewAPIRequestIdHeader))
		})
	}
}

// assertHarnessUnauthorizedBody checks the wire shape a refused platform credential
// returns. The code matters: the published contract names GATEWAY_UNAUTHORIZED, and
// a client branching on it must not receive an empty code.
func assertHarnessUnauthorizedBody(t *testing.T, body []byte) {
	t.Helper()
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(body, &payload))
	assert.Equal(t, string(harnessUnauthorizedCode), payload.Error.Code,
		"a refused credential answers the contract code, not an empty one")
	assert.NotEmpty(t, payload.Error.Message)
}

// TestHarnessUnauthorizedResponseCarriesContractCode pins the response a withheld
// principal produces. RequireHarnessPrincipal is the fail-closed gate on the
// /internal group, so its rejection is the first thing a caller sees when something
// is wrong with its credential.
func TestHarnessUnauthorizedResponseCarriesContractCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/internal/v1/gateway/health", nil)

	RequireHarnessPrincipal()(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assertHarnessUnauthorizedBody(t, recorder.Body.Bytes())
}
