package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// HarnessPrincipalContextKey holds the verified Harness principal.
	HarnessPrincipalContextKey = "harness_principal"

	harnessJWTCacheTTL         = 10 * time.Minute
	harnessJWTFetchTimeout     = 10 * time.Second
	harnessJWKSMaxResponseSize = 1 << 20
	// harnessJWKSForcedRefreshInterval bounds how long a kid that was absent from
	// a fresh snapshot keeps being refused without another fetch. Without it,
	// anyone could force one outbound JWKS call per inbound request by sending
	// random kids.
	harnessJWKSForcedRefreshInterval = 30 * time.Second
	// harnessJWKSUnknownKidLimit bounds the refused-kid set per snapshot.
	harnessJWKSUnknownKidLimit = 256
)

var errUnknownHarnessJWKSKid = errors.New("unknown harness jwks kid")

// HarnessPrincipal is the verified Harness caller mapped onto an internal
// service token.
type HarnessPrincipal struct {
	Subject string
	Scopes  []string
	JTI     string
	TokenID int
}

func GetHarnessPrincipal(c *gin.Context) (HarnessPrincipal, bool) {
	value, ok := c.Get(HarnessPrincipalContextKey)
	if !ok {
		return HarnessPrincipal{}, false
	}
	principal, ok := value.(HarnessPrincipal)
	return principal, ok
}

// HarnessJWT converts a Harness-issued RS256 JWT into the internal service
// token it maps to, so the existing TokenAuth middleware keeps enforcing every
// token rule (IP allowlist, user status, group ratio, quota, resource pools).
// Requests that are not Harness JWTs, or deployments that have no JWKS URL
// configured, pass through untouched.
func HarnessJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		credential, ok := harnessJWTCredential(c.Request.Header.Get("Authorization"))
		if !ok {
			c.Next()
			return
		}
		config := setting.GetHarnessAuthConfig()
		if !config.Enabled() {
			c.Next()
			return
		}

		parsed, err := harnessJWTParser(config).Parse(credential, func(token *jwt.Token) (any, error) {
			kid, _ := token.Header["kid"].(string)
			return harnessJWKSPublicKey(c.Request.Context(), config.JWKSURL, kid)
		})
		if err != nil || !parsed.Valid {
			abortWithHarnessUnauthorized(c)
			return
		}

		claims, ok := parsed.Claims.(jwt.MapClaims)
		if !ok {
			abortWithHarnessUnauthorized(c)
			return
		}
		subject, _ := claims.GetSubject()
		subject = strings.TrimSpace(subject)
		reference, mapped := config.Subjects[subject]
		if subject == "" || !mapped {
			abortWithHarnessUnauthorized(c)
			return
		}
		token, err := harnessMappedToken(reference)
		if err != nil || token == nil || token.Status != common.TokenStatusEnabled {
			abortWithHarnessUnauthorized(c)
			return
		}

		jti, _ := claims["jti"].(string)
		principal := HarnessPrincipal{
			Subject: subject,
			Scopes:  harnessScopes(claims),
			JTI:     jti,
			TokenID: token.Id,
		}
		// TokenAuth splits the credential on "-", which would corrupt a
		// base64url JWT, so the JWT never reaches it: only the mapped internal
		// token key is forwarded.
		c.Request.Header.Set("Authorization", "Bearer sk-"+token.Key)
		c.Set(HarnessPrincipalContextKey, principal)
		c.Next()
	}
}

// HarnessChatScope is the scope a Harness principal needs to call a model
// (A1/A2/A4/A9). It is named by the contract's scope table, not invented here:
// gateway.chat authorises model calls, gateway.catalog.read the tier catalog,
// gateway.usage.read / gateway.cost.read the ledgers, and gateway.health.read the
// health probe. The catalog and ledger routes already enforce their own scope, so
// this one covers exactly the model-call surface.
const HarnessChatScope = "gateway.chat"

// RequireHarnessScope rejects a verified Harness principal that lacks the
// scope. Requests that were not identified as Harness callers pass through, so
// the middleware is safe on shared routes.
func RequireHarnessScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := GetHarnessPrincipal(c)
		if !ok {
			c.Next()
			return
		}
		if !slices.Contains(principal.Scopes, scope) {
			abortWithOpenAiMessage(c, http.StatusForbidden,
				fmt.Sprintf("harness principal is missing required scope %q", scope),
				types.ErrorCodeAccessDenied)
			return
		}
		c.Next()
	}
}

// RequireHarnessPrincipal rejects every caller that is not a verified Harness
// principal. It is the fail-closed gate for the /internal/v1/gateway group: those
// paths expose the procurement ledger, so they must never be reachable when the
// JWT integration is unconfigured or when a caller presents any other credential.
func RequireHarnessPrincipal() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := GetHarnessPrincipal(c); !ok {
			abortWithOpenAiMessage(c, http.StatusUnauthorized,
				common.TranslateMessage(c, i18n.MsgTokenInvalid))
			return
		}
		c.Next()
	}
}

func abortWithHarnessUnauthorized(c *gin.Context) {
	// One message for every failure mode (signature, expiry, audience, unknown
	// subject, disabled token) so callers cannot probe the difference.
	abortWithOpenAiMessage(c, http.StatusUnauthorized, common.TranslateMessage(c, i18n.MsgTokenInvalid))
}

// harnessJWTCredential reports whether the Authorization header looks like a
// Harness JWT: Bearer plus three dot-separated segments whose first one is the
// base64url of "{".
func harnessJWTCredential(header string) (string, bool) {
	scheme, value, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "eyJ") {
		return "", false
	}
	segments := strings.Split(value, ".")
	if len(segments) != 3 || segments[1] == "" || segments[2] == "" {
		return "", false
	}
	return value, true
}

func harnessJWTParser(config setting.HarnessAuthConfig) *jwt.Parser {
	return jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(config.Issuer),
		jwt.WithAudience(config.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(60*time.Second),
	)
}

func harnessScopes(claims jwt.MapClaims) []string {
	raw, _ := claims["scope"].(string)
	fields := strings.Fields(raw)
	if fields == nil {
		return []string{}
	}
	return fields
}

// harnessMappedToken loads the internal token a subject maps to. An ambiguous
// token name is rejected by the model layer rather than bound to an arbitrary
// credential, because that would silently authorize the wrong quota, group and
// resource pools.
func harnessMappedToken(reference setting.HarnessSubject) (*model.Token, error) {
	if reference.TokenID > 0 {
		return model.GetTokenById(reference.TokenID)
	}
	if reference.TokenName == "" {
		return nil, errors.New("harness subject has no token reference")
	}
	return model.GetUniqueTokenByName(reference.TokenName)
}

type harnessJWKSKeySet struct {
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	// unknownKids records kids this snapshot did not contain, with the time they
	// were observed. A repeat of the same kid is refused instead of refetching,
	// so random kids cannot amplify into outbound requests; a kid seen for the
	// first time still forces an immediate refresh, which is what lets a
	// rotated key be picked up without waiting for the cache TTL.
	unknownKids map[string]time.Time
}

var (
	harnessJWKSCacheMu   sync.RWMutex
	harnessJWKSCache     = make(map[string]*harnessJWKSKeySet)
	harnessJWKSRefreshMu sync.Mutex

	harnessJWKSHTTPClient = &http.Client{Timeout: harnessJWTFetchTimeout}
)

func (set *harnessJWKSKeySet) publicKey(kid string) (*rsa.PublicKey, bool) {
	if key, ok := set.keys[kid]; ok {
		return key, true
	}
	// A single-key JWKS may omit kid; accepting it is still signature-verified.
	if kid == "" && len(set.keys) == 1 {
		for _, key := range set.keys {
			return key, true
		}
	}
	return nil, false
}

func (set *harnessJWKSKeySet) fresh(now time.Time) bool {
	return now.Sub(set.fetchedAt) < harnessJWTCacheTTL
}

// lookupHarnessJWKSKey returns the cached key when the snapshot is fresh, plus
// whether this kid is a recently-refused repeat.
func lookupHarnessJWKSKey(jwksURL string, kid string, now time.Time) (key *rsa.PublicKey, refused bool) {
	harnessJWKSCacheMu.RLock()
	defer harnessJWKSCacheMu.RUnlock()
	set, ok := harnessJWKSCache[jwksURL]
	if !ok || set == nil || !set.fresh(now) {
		return nil, false
	}
	if key, found := set.publicKey(kid); found {
		return key, false
	}
	seenAt, seen := set.unknownKids[kid]
	return nil, seen && now.Sub(seenAt) < harnessJWKSForcedRefreshInterval
}

// rememberUnknownKid records a refused kid on the cached snapshot, pruning stale
// entries and bounding the map so an abusive client cannot grow it without limit.
func rememberUnknownKid(jwksURL string, kid string) {
	harnessJWKSCacheMu.Lock()
	defer harnessJWKSCacheMu.Unlock()
	set, ok := harnessJWKSCache[jwksURL]
	if !ok || set == nil {
		return
	}
	now := time.Now()
	for recordedKid, seenAt := range set.unknownKids {
		if now.Sub(seenAt) >= harnessJWKSForcedRefreshInterval {
			delete(set.unknownKids, recordedKid)
		}
	}
	if len(set.unknownKids) >= harnessJWKSUnknownKidLimit {
		return
	}
	set.unknownKids[kid] = now
}

// harnessJWKSPublicKey returns the JWKS key for kid, refetching immediately when
// the kid is unknown: that forced refresh is what makes key rotation safe.
// Cached keys are served without taking the refresh lock, so only an unknown kid
// (or an expired cache) waits for a fetch, and concurrent unknowns share one.
func harnessJWKSPublicKey(ctx context.Context, jwksURL string, kid string) (*rsa.PublicKey, error) {
	key, refused := lookupHarnessJWKSKey(jwksURL, kid, time.Now())
	if key != nil {
		return key, nil
	}
	if refused {
		return nil, errUnknownHarnessJWKSKid
	}

	harnessJWKSRefreshMu.Lock()
	defer harnessJWKSRefreshMu.Unlock()

	// Another request may have refreshed while this one waited; the refreshed
	// snapshot is authoritative, so a kid it still lacks is really unknown.
	key, refused = lookupHarnessJWKSKey(jwksURL, kid, time.Now())
	if key != nil {
		return key, nil
	}
	if refused {
		return nil, errUnknownHarnessJWKSKid
	}
	staleKey := harnessStaleHarnessJWKSKey(jwksURL, kid)

	// The cache is shared across requests, so a cancelled caller must not abort
	// a refresh the other requests are waiting on.
	fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), harnessJWTFetchTimeout)
	defer cancel()
	fetched, err := fetchHarnessJWKS(fetchCtx, jwksURL)
	if err != nil {
		if staleKey != nil {
			return staleKey, nil
		}
		return nil, err
	}

	now := time.Now()
	harnessJWKSCacheMu.Lock()
	for cachedURL, cachedSet := range harnessJWKSCache {
		if !cachedSet.fresh(now) {
			delete(harnessJWKSCache, cachedURL)
		}
	}
	harnessJWKSCache[jwksURL] = fetched
	harnessJWKSCacheMu.Unlock()

	if key, ok := fetched.publicKey(kid); ok {
		return key, nil
	}
	rememberUnknownKid(jwksURL, kid)
	return nil, errUnknownHarnessJWKSKid
}

// harnessStaleHarnessJWKSKey keeps serving an already-known key when the JWKS
// endpoint is temporarily unreachable.
func harnessStaleHarnessJWKSKey(jwksURL string, kid string) *rsa.PublicKey {
	harnessJWKSCacheMu.RLock()
	defer harnessJWKSCacheMu.RUnlock()
	set, ok := harnessJWKSCache[jwksURL]
	if !ok || set == nil {
		return nil
	}
	key, _ := set.publicKey(kid)
	return key
}

type harnessJWKSResponse struct {
	Keys []harnessJWK `json:"keys"`
}

type harnessJWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func fetchHarnessJWKS(ctx context.Context, jwksURL string) (*harnessJWKSKeySet, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := harnessJWKSHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("harness jwks endpoint returned status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, harnessJWKSMaxResponseSize))
	if err != nil {
		return nil, err
	}
	var document harnessJWKSResponse
	if err := common.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, jwk := range document.Keys {
		if jwk.Kty != "RSA" {
			continue
		}
		key, err := jwk.rsaPublicKey()
		if err != nil {
			continue
		}
		keys[jwk.Kid] = key
	}
	if len(keys) == 0 {
		return nil, errors.New("harness jwks contains no usable RSA key")
	}
	return &harnessJWKSKeySet{keys: keys, fetchedAt: time.Now(), unknownKids: make(map[string]time.Time)}, nil
}

func (jwk harnessJWK) rsaPublicKey() (*rsa.PublicKey, error) {
	modulusBytes, err := decodeJWKSBase64(jwk.N)
	if err != nil {
		return nil, err
	}
	exponentBytes, err := decodeJWKSBase64(jwk.E)
	if err != nil {
		return nil, err
	}
	modulus := new(big.Int).SetBytes(modulusBytes)
	exponent := new(big.Int).SetBytes(exponentBytes)
	if modulus.Sign() <= 0 || exponent.Sign() <= 0 || !exponent.IsInt64() || exponent.Int64() > 1<<31-1 {
		return nil, errors.New("invalid rsa jwk parameters")
	}
	return &rsa.PublicKey{N: modulus, E: int(exponent.Int64())}, nil
}

func decodeJWKSBase64(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(value, "="))
}
