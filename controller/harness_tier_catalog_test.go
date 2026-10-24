package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The A3/B4 contract is consumed by an external platform, so these tests pin the
// wire shapes and the revalidation behaviour rather than internal helpers.

// harnessTierCatalogTestModelNames are deliberately NOT tier ids: a tier id is
// public while the model name that backs it is internal, which is what lets the
// test prove the response never leaks a real model name.
const (
	harnessTierCatalogTestEconomyModel    = "internal-economy-upstream"
	harnessTierCatalogTestEnterpriseModel = "internal-enterprise-upstream"
)

// setupHarnessTierCatalogTestDB installs a fresh SQLite database with one enabled
// OpenAI-type channel serving the given model names, so model.GetPricing() emits
// exactly those models with the endpoint types that channel type implies.
func setupHarnessTierCatalogTestDB(t *testing.T, modelNames ...string) {
	t.Helper()
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldOptionMap := common.OptionMap

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "-"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	common.MemoryCacheEnabled = false
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}, &model.Option{}))
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
		model.InvalidatePricingCache()
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	if len(modelNames) == 0 {
		model.InvalidatePricingCache()
		return
	}
	channel := model.Channel{
		Type:   constant.ChannelTypeOpenAI,
		Key:    "harness-tier-catalog-key",
		Status: common.ChannelStatusEnabled,
		Name:   "harness-tier-catalog",
		Models: strings.Join(modelNames, ","),
	}
	require.NoError(t, db.Create(&channel).Error)
	for _, modelName := range modelNames {
		require.NoError(t, db.Create(&model.Ability{
			Group: "default", Model: modelName, ChannelId: channel.Id, Enabled: true,
		}).Error)
	}
	model.InvalidatePricingCache()
}

// configureHarnessTierCatalogTiers publishes the tier config through the option
// map, which is how an administrator configures it in production.
func configureHarnessTierCatalogTiers(t *testing.T, tiers []setting.HarnessCatalogTier) {
	t.Helper()
	payload, err := common.Marshal(setting.HarnessCatalogConfig{Tiers: tiers, HarnessCatalogVersion: 1})
	require.NoError(t, err)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[setting.HarnessCatalogOptionKey] = string(payload)
	common.OptionMapRWMutex.Unlock()
}

// harnessTierCatalogTestRouter serves the handler through a throwaway gin engine.
// A real engine is required rather than calling the handler directly: gin defers
// writing the status line until the engine's ServeHTTP runs, so a directly
// invoked handler would report 200 even for c.Status(304).
func harnessTierCatalogTestRouter(authenticated bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		if authenticated {
			c.Set(middleware.HarnessPrincipalContextKey, middleware.HarnessPrincipal{
				Subject: "harness-gateway",
				Scopes:  []string{"gateway.catalog.read"},
				JTI:     "jti-catalog",
				TokenID: 1,
			})
		}
		c.Next()
	})
	engine.GET("/internal/v1/gateway/tier-catalog", GetHarnessTierCatalog)
	engine.GET("/internal/v1/gateway/health", GetHarnessHealth)
	return engine
}

// requestHarnessTierCatalog runs the handler as a verified Harness principal.
func requestHarnessTierCatalog(t *testing.T, ifNoneMatch string, authenticated bool) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/internal/v1/gateway/tier-catalog", nil)
	if ifNoneMatch != "" {
		request.Header.Set("If-None-Match", ifNoneMatch)
	}
	recorder := httptest.NewRecorder()
	harnessTierCatalogTestRouter(authenticated).ServeHTTP(recorder, request)
	return recorder
}

// requestHarnessHealth serves B4 through the same throwaway engine.
func requestHarnessHealth(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	harnessTierCatalogTestRouter(true).ServeHTTP(recorder,
		httptest.NewRequest(http.MethodGet, "/internal/v1/gateway/health", nil))
	return recorder
}

// TestGetHarnessTierCatalogWireContract pins the literal A3 body: exact field
// names, tier-only data, and none of the internal data the contract forbids.
func TestGetHarnessTierCatalogWireContract(t *testing.T) {
	setupHarnessTierCatalogTestDB(t, harnessTierCatalogTestEconomyModel, harnessTierCatalogTestEnterpriseModel)
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Models: []string{harnessTierCatalogTestEconomyModel}},
		{ID: "enterprise", DisplayName: "企业档", Models: []string{harnessTierCatalogTestEnterpriseModel}},
	})

	recorder := requestHarnessTierCatalog(t, "", true)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	// The contract requires a bare object, never the dashboard envelope.
	assert.NotContains(t, body, "success")
	assert.NotContains(t, body, "message")
	assert.Equal(t, "list", body["object"])

	items, ok := body["data"].([]any)
	require.True(t, ok, "data must be an array, got %T", body["data"])
	require.Len(t, items, 2)

	first, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "economy", first["id"])
	assert.Equal(t, "model", first["object"])
	assert.Equal(t, "经济档", first["display_name"])
	assert.Equal(t, []any{"chat", "stream", "tools"}, first["capabilities"])
	assert.Len(t, first, 4, "a tier must expose exactly id/object/display_name/capabilities")

	// catalog_version is a decimal string, never a float.
	version, ok := body["catalog_version"].(string)
	require.True(t, ok, "catalog_version must be a string, got %T", body["catalog_version"])
	parsedVersion, err := strconv.ParseInt(version, 10, 64)
	require.NoError(t, err)
	assert.Positive(t, parsedVersion)

	etag, ok := body["etag"].(string)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(etag, `W/"`), "etag must be weak, got %q", etag)
	assert.Equal(t, etag, recorder.Header().Get("ETag"))

	// Nothing internal may reach the platform.
	serialized := recorder.Body.String()
	for _, forbidden := range []string{
		harnessTierCatalogTestEconomyModel,
		harnessTierCatalogTestEnterpriseModel,
		"upstream_models",
		"pricing",
		"channel",
		"harness-tier-catalog-key",
	} {
		assert.NotContains(t, serialized, forbidden)
	}
}

// TestGetHarnessTierCatalogRevalidation covers the three caching requirements:
// the validator headers are present, a matching If-None-Match answers 304 without
// a body, and a changed catalog answers a fresh 200.
func TestGetHarnessTierCatalogRevalidation(t *testing.T) {
	setupHarnessTierCatalogTestDB(t, harnessTierCatalogTestEconomyModel, harnessTierCatalogTestEnterpriseModel)
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Models: []string{harnessTierCatalogTestEconomyModel}},
		{ID: "enterprise", DisplayName: "企业档", Models: []string{harnessTierCatalogTestEnterpriseModel}},
	})

	first := requestHarnessTierCatalog(t, "", true)
	require.Equal(t, http.StatusOK, first.Code)
	etag := first.Header().Get("ETag")
	require.NotEmpty(t, etag)
	// no-store would forbid the client from keeping the body, which makes 304
	// unreachable; the contract requires revalidating no-cache.
	assert.Equal(t, "no-cache", first.Header().Get("Cache-Control"))
	assert.Equal(t, "Authorization, Accept-Encoding", first.Header().Get("Vary"))

	notModified := requestHarnessTierCatalog(t, etag, true)
	require.Equal(t, http.StatusNotModified, notModified.Code)
	assert.Empty(t, notModified.Body.String(), "a 304 must carry no body")
	assert.Equal(t, etag, notModified.Header().Get("ETag"))

	// A non-matching validator still gets the full representation.
	stale := requestHarnessTierCatalog(t, `W/"stale-validator"`, true)
	require.Equal(t, http.StatusOK, stale.Code)
	assert.NotEmpty(t, stale.Body.String())

	// Making a second tier unavailable changes the content, so the same
	// validator must no longer match.
	require.NoError(t, model.DB.Where("model = ?", harnessTierCatalogTestEnterpriseModel).Delete(&model.Ability{}).Error)
	model.InvalidatePricingCache()

	changed := requestHarnessTierCatalog(t, etag, true)
	require.Equal(t, http.StatusOK, changed.Code, "a changed catalog must not answer 304")
	assert.NotEqual(t, etag, changed.Header().Get("ETag"))

	var body map[string]any
	require.NoError(t, common.Unmarshal(changed.Body.Bytes(), &body))
	items, _ := body["data"].([]any)
	require.Len(t, items, 1, "an unavailable tier must not be advertised")
	assert.Equal(t, "economy", items[0].(map[string]any)["id"])
}

// TestGetHarnessTierCatalogVersionAdvancesOnlyOnContentChange pins the monotonic
// version: repeated reads of identical content reuse it, while a real content
// change advances it.
func TestGetHarnessTierCatalogVersionAdvancesOnlyOnContentChange(t *testing.T) {
	setupHarnessTierCatalogTestDB(t, harnessTierCatalogTestEconomyModel, harnessTierCatalogTestEnterpriseModel)
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Models: []string{harnessTierCatalogTestEconomyModel}},
		{ID: "enterprise", DisplayName: "企业档", Models: []string{harnessTierCatalogTestEnterpriseModel}},
	})

	catalogVersion := func(recorder *httptest.ResponseRecorder) int64 {
		t.Helper()
		var body map[string]any
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		raw, ok := body["catalog_version"].(string)
		require.True(t, ok)
		version, err := strconv.ParseInt(raw, 10, 64)
		require.NoError(t, err)
		return version
	}

	baseline := catalogVersion(requestHarnessTierCatalog(t, "", true))
	assert.Equal(t, baseline, catalogVersion(requestHarnessTierCatalog(t, "", true)),
		"identical content must reuse the version")
	assert.Equal(t, baseline, catalogVersion(requestHarnessTierCatalog(t, "", true)),
		"repeated reads must never advance the version")

	// Change only the display name: content changed, so the version advances.
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档（新）", Models: []string{harnessTierCatalogTestEconomyModel}},
		{ID: "enterprise", DisplayName: "企业档", Models: []string{harnessTierCatalogTestEnterpriseModel}},
	})

	advanced := catalogVersion(requestHarnessTierCatalog(t, "", true))
	assert.Greater(t, advanced, baseline, "a content change must advance the version")
	assert.Equal(t, advanced, catalogVersion(requestHarnessTierCatalog(t, "", true)),
		"the advanced version must then stay stable")
}

// TestGetHarnessTierCatalogRejectsUnverifiedCaller pins the 403 contract. The
// tier list is commercial information, so an unauthenticated caller learns
// nothing about it.
func TestGetHarnessTierCatalogRejectsUnverifiedCaller(t *testing.T) {
	setupHarnessTierCatalogTestDB(t, harnessTierCatalogTestEconomyModel)
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Models: []string{harnessTierCatalogTestEconomyModel}},
	})

	recorder := requestHarnessTierCatalog(t, "", false)
	require.Equal(t, http.StatusForbidden, recorder.Code)

	var body harnessTierCatalogErrorEnvelope
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "GATEWAY_UNAUTHORIZED", body.Error.Code)
	assert.NotEmpty(t, body.Error.Message)
	assert.False(t, body.Error.Retryable)
	assert.NotContains(t, recorder.Body.String(), "economy")
}

// TestHarnessTierCatalogCapabilitiesMapping pins the endpoint-type to capability
// mapping, including the "unknown means empty" rule: the catalog must never claim
// a capability the gateway cannot verify.
func TestHarnessTierCatalogCapabilitiesMapping(t *testing.T) {
	tests := []struct {
		name          string
		endpointTypes []constant.EndpointType
		expected      []string
	}{
		{
			name:          "no endpoint types yields no capabilities",
			endpointTypes: nil,
			expected:      []string{},
		},
		{
			name:          "unmapped endpoint yields no capabilities",
			endpointTypes: []constant.EndpointType{constant.EndpointTypeJinaRerank},
			expected:      []string{},
		},
		{
			name:          "embeddings and image generation are not conversational",
			endpointTypes: []constant.EndpointType{constant.EndpointTypeEmbeddings, constant.EndpointTypeImageGeneration},
			expected:      []string{},
		},
		{
			name:          "openai chat supports chat stream and tools",
			endpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI},
			expected:      []string{"chat", "stream", "tools"},
		},
		{
			name:          "anthropic messages supports chat stream and tools",
			endpointTypes: []constant.EndpointType{constant.EndpointTypeAnthropic},
			expected:      []string{"chat", "stream", "tools"},
		},
		{
			name:          "gemini generateContent supports chat stream and tools",
			endpointTypes: []constant.EndpointType{constant.EndpointTypeGemini},
			expected:      []string{"chat", "stream", "tools"},
		},
		{
			name:          "responses supports chat stream and tools",
			endpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAIResponse},
			expected:      []string{"chat", "stream", "tools"},
		},
		{
			name:          "alpha search streams but carries no tools",
			endpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAIAlphaSearch},
			expected:      []string{"chat", "stream"},
		},
		{
			name:          "compaction is a one-shot chat transformation",
			endpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAIResponseCompact},
			expected:      []string{"chat"},
		},
		{
			name:          "duplicate endpoint types collapse and keep enum order",
			endpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAIResponseCompact, constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAI},
			expected:      []string{"chat", "stream", "tools"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, harnessTierCatalogCapabilities(test.endpointTypes))
		})
	}
}

// TestBuildHarnessTierCatalogUnionsBackingModelCapabilities pins the tier-level
// aggregation and the availability rule: a tier is advertised only when one of its
// backing models is in the pricing table, and its capabilities are the union of the
// reachable models' capabilities in a stable order.
func TestBuildHarnessTierCatalogUnionsBackingModelCapabilities(t *testing.T) {
	setupHarnessTierCatalogTestDB(t)
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Models: []string{"embedding-only", "chat-model"}},
		{ID: "enterprise", DisplayName: "企业档", Models: []string{"absent-model"}},
	})

	items := buildHarnessTierCatalog(map[string]model.Pricing{
		"embedding-only": {ModelName: "embedding-only", SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeEmbeddings}},
		"chat-model":     {ModelName: "chat-model", SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI}},
	})

	require.Len(t, items, 1, "a tier with no priced backing model must be omitted")
	assert.Equal(t, "economy", items[0].ID)
	assert.Equal(t, []string{"chat", "stream", "tools"}, items[0].Capabilities)

	// A tier backed only by a known-but-non-conversational model reports an empty
	// capability list rather than an invented one.
	items = buildHarnessTierCatalog(map[string]model.Pricing{
		"embedding-only": {ModelName: "embedding-only", SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeEmbeddings}},
	})
	require.Len(t, items, 1)
	assert.Equal(t, []string{}, items[0].Capabilities)
}

// TestGetHarnessHealthStatusBranches pins B4's three states and the exact body.
func TestGetHarnessHealthStatusBranches(t *testing.T) {
	healthBody := func(t *testing.T, recorder *httptest.ResponseRecorder) harnessHealthResponse {
		t.Helper()
		var body harnessHealthResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		return body
	}
	requestHealth := func() *httptest.ResponseRecorder {
		return requestHarnessHealth(t)
	}

	timePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`)

	t.Run("ok when the database answers and a channel is enabled", func(t *testing.T) {
		setupHarnessTierCatalogTestDB(t, harnessTierCatalogTestEconomyModel)
		recorder := requestHealth()
		require.Equal(t, http.StatusOK, recorder.Code)
		body := healthBody(t, recorder)
		assert.Equal(t, "ok", body.Status)
		assert.Equal(t, "v2", body.InterfaceVersion)
		assert.Equal(t, "45fbb149", body.UpstreamBaseline)
		assert.Regexp(t, timePattern, body.Time, "time must be UTC RFC3339 with millisecond precision")
	})

	t.Run("degraded when no channel is enabled", func(t *testing.T) {
		setupHarnessTierCatalogTestDB(t)
		recorder := requestHealth()
		require.Equal(t, http.StatusOK, recorder.Code, "degraded is still a served response")
		body := healthBody(t, recorder)
		assert.Equal(t, "degraded", body.Status)
		assert.Equal(t, "v2", body.InterfaceVersion)
		assert.Equal(t, "45fbb149", body.UpstreamBaseline)
		assert.Regexp(t, timePattern, body.Time)
	})

	t.Run("down when the database is unreachable", func(t *testing.T) {
		oldDB := model.DB
		model.DB = nil
		t.Cleanup(func() { model.DB = oldDB })

		recorder := requestHealth()
		require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
		body := healthBody(t, recorder)
		assert.Equal(t, "down", body.Status)
		assert.Equal(t, "v2", body.InterfaceVersion)
		assert.Equal(t, "45fbb149", body.UpstreamBaseline)
		assert.Regexp(t, timePattern, body.Time)
	})
}
