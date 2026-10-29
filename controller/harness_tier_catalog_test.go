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
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The A3/B4 contract is consumed by an external platform, so these tests pin the
// wire shapes and the revalidation behaviour rather than internal helpers.

const (
	harnessTierCatalogTestTextModelA = "test-text-model-a"
	harnessTierCatalogTestTextModelB = "test-text-model-b"
	harnessTierCatalogTestVideoModel = "test-video-model"
)

// setupHarnessTierCatalogTestDB installs a fresh SQLite database. When
// channelsForPool is given, it creates one enabled OpenAI-type channel per pool,
// listing that pool's model names, described by the pool's Intelligent Routing
// channel order. A pool absent from the map keeps no channels, which is how the
// video pools look until they are wired up.
func setupHarnessTierCatalogTestDB(t *testing.T, channelsForPool map[string][]string) {
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

	poolIDs := make([]string, 0, len(channelsForPool))
	for poolID := range channelsForPool {
		poolIDs = append(poolIDs, poolID)
	}
	// Deterministic channel creation keeps channel ids stable across runs.
	sortStrings(poolIDs)

	channelOrder := make(map[string][]int, len(channelsForPool))
	for _, poolID := range poolIDs {
		modelNames := channelsForPool[poolID]
		channel := model.Channel{
			Type:   constant.ChannelTypeOpenAI,
			Key:    "harness-tier-catalog-key-" + poolID,
			Status: common.ChannelStatusEnabled,
			Name:   "harness-tier-catalog-" + poolID,
			Models: strings.Join(modelNames, ","),
		}
		require.NoError(t, db.Create(&channel).Error)
		for _, modelName := range modelNames {
			require.NoError(t, db.Create(&model.Ability{
				Group: "default", Model: modelName, ChannelId: channel.Id, Enabled: true,
			}).Error)
		}
		channelOrder[poolID] = []int{channel.Id}
	}
	if len(channelOrder) > 0 {
		payload, marshalErr := common.Marshal(struct {
			GlobalChannelOrder []int            `json:"global_channel_order"`
			Groups             []map[string]any `json:"groups"`
		}{
			GlobalChannelOrder: []int{},
			Groups:             poolGroupsFromOrder(channelOrder, poolIDs),
		})
		require.NoError(t, marshalErr)
		common.OptionMapRWMutex.Lock()
		common.OptionMap["IntelligentRouting"] = string(payload)
		common.OptionMapRWMutex.Unlock()
		require.NoError(t, setting.ValidateIntelligentRoutingJSON(string(payload)))
		require.NoError(t, setting.UpdateIntelligentRoutingConfigJSON(string(payload)))
	}
	model.InvalidatePricingCache()
}

// poolGroupsFromOrder renders Intelligent Routing groups from the test's pool to
// channel mapping.
func poolGroupsFromOrder(channelOrder map[string][]int, poolIDs []string) []map[string]any {
	groups := make([]map[string]any, 0, len(poolIDs))
	for _, poolID := range poolIDs {
		groups = append(groups, map[string]any{
			"id":            poolID,
			"name":          poolID,
			"channel_order": channelOrder[poolID],
		})
	}
	return groups
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// configureHarnessTierCatalogTiers publishes the pool config through the option
// map, which is how an administrator configures it in production.
func configureHarnessTierCatalogTiers(t *testing.T, tiers []setting.HarnessCatalogTier) {
	t.Helper()
	payload, err := common.Marshal(setting.HarnessCatalogConfig{Tiers: tiers, HarnessCatalogVersion: 1})
	require.NoError(t, err)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[setting.HarnessCatalogOptionKey] = string(payload)
	common.OptionMapRWMutex.Unlock()
}

// priceHarnessTierCatalogModels gives every named model a configured ratio, so
// the catalog treats it as billable and actually advertises it. A model with no
// price is deliberately not advertised, which is a separate test.
func priceHarnessTierCatalogModels(t *testing.T, modelNames ...string) {
	t.Helper()
	payload := make(map[string]float64, len(modelNames))
	for _, name := range modelNames {
		payload[name] = 1.5
	}
	encoded, err := common.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
	})
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
				Scopes:  []string{gatewayCatalogReadScopeForTest},
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

// gatewayCatalogReadScopeForTest mirrors the constant the handler checks, so the
// test fails loudly if the scope name ever drifts.
const gatewayCatalogReadScopeForTest = "gateway.catalog.read"

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

func requestHarnessHealth(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	harnessTierCatalogTestRouter(true).ServeHTTP(recorder,
		httptest.NewRequest(http.MethodGet, "/internal/v1/gateway/health", nil))
	return recorder
}

// catalogItems decodes the A3 body's data array into per-pool maps.
func catalogItems(t *testing.T, recorder *httptest.ResponseRecorder) (map[string]any, []map[string]any) {
	t.Helper()
	var body map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	raw, ok := body["data"].([]any)
	require.True(t, ok, "data must be an array, got %T", body["data"])
	items := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		item, ok := entry.(map[string]any)
		require.True(t, ok)
		items = append(items, item)
	}
	return body, items
}

// TestGetHarnessTierCatalogWireContract pins the literal A3 body: each pool
// carries its id, labels, real model ids, and capabilities, and nothing about
// channels or pricing leaks.
func TestGetHarnessTierCatalogWireContract(t *testing.T) {
	setupHarnessTierCatalogTestDB(t, map[string][]string{
		"economy":    {harnessTierCatalogTestTextModelA},
		"enterprise": {harnessTierCatalogTestTextModelB},
	})
	priceHarnessTierCatalogModels(t, harnessTierCatalogTestTextModelA, harnessTierCatalogTestTextModelB)
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Kind: setting.HarnessCatalogKindText},
		{ID: "enterprise", DisplayName: "企业档", Kind: setting.HarnessCatalogKindText},
	})

	recorder := requestHarnessTierCatalog(t, "", true)
	require.Equal(t, http.StatusOK, recorder.Code)

	body, items := catalogItems(t, recorder)
	// The contract requires a bare object, never the dashboard envelope.
	assert.NotContains(t, body, "success")
	assert.NotContains(t, body, "message")
	assert.Equal(t, "list", body["object"])

	require.Len(t, items, 2)
	first := items[0]
	assert.Equal(t, "economy", first["id"])
	assert.Equal(t, "model", first["object"])
	assert.Equal(t, "经济档", first["display_name"])
	assert.Equal(t, "text", first["kind"])
	assert.Equal(t, []any{harnessTierCatalogTestTextModelA}, first["models"],
		"a pool must advertise the real model ids it can route")
	assert.Equal(t, []any{"chat", "stream", "tools"}, first["capabilities"])

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
		"upstream_models",
		"pricing",
		"model_mapping",
		"harness-tier-catalog-key",
		"channel_id",
	} {
		assert.NotContains(t, serialized, forbidden)
	}
}

// An empty pool is still advertised, with models and capabilities serialised as
// arrays rather than null: the platform iterates them directly.
func TestGetHarnessTierCatalogAdvertisesEmptyVideoPools(t *testing.T) {
	setupHarnessTierCatalogTestDB(t, map[string][]string{
		"economy": {harnessTierCatalogTestTextModelA},
	})
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Kind: setting.HarnessCatalogKindText},
		{
			ID:          "video-per-call",
			DisplayName: "视频按次",
			Kind:        setting.HarnessCatalogKindVideo,
			Billing:     setting.HarnessCatalogBillingPerCall,
		},
		{
			ID:          "video-per-second",
			DisplayName: "视频按秒",
			Kind:        setting.HarnessCatalogKindVideo,
			Billing:     setting.HarnessCatalogBillingPerSecond,
		},
	})

	recorder := requestHarnessTierCatalog(t, "", true)
	require.Equal(t, http.StatusOK, recorder.Code)

	_, items := catalogItems(t, recorder)
	require.Len(t, items, 3, "a pool with no channels is still advertised")

	byID := make(map[string]map[string]any, len(items))
	for _, item := range items {
		byID[item["id"].(string)] = item
	}

	videoCall := byID["video-per-call"]
	require.NotNil(t, videoCall)
	assert.Equal(t, "video", videoCall["kind"])
	assert.Equal(t, "per_call", videoCall["billing"])

	videoSecond := byID["video-per-second"]
	require.NotNil(t, videoSecond)
	assert.Equal(t, "per_second", videoSecond["billing"])

	serialized := recorder.Body.String()
	assert.Contains(t, serialized, `"models":[]`, "empty models must be an array, not null")
	assert.Contains(t, serialized, `"capabilities":[]`, "empty capabilities must be an array, not null")
	assert.NotContains(t, serialized, `"models":null`)
	assert.NotContains(t, serialized, `"capabilities":null`)
}

// TestGetHarnessTierCatalogRevalidation covers the three caching requirements:
// the validator headers are present, a matching If-None-Match answers 304 without
// a body, and a changed catalog answers a fresh 200.
func TestGetHarnessTierCatalogRevalidation(t *testing.T) {
	setupHarnessTierCatalogTestDB(t, map[string][]string{
		"economy":    {harnessTierCatalogTestTextModelA},
		"enterprise": {harnessTierCatalogTestTextModelB},
	})
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Kind: setting.HarnessCatalogKindText},
		{ID: "enterprise", DisplayName: "企业档", Kind: setting.HarnessCatalogKindText},
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

	// Changing a display name changes the content, so the same validator must no
	// longer match.
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档（新）", Kind: setting.HarnessCatalogKindText},
		{ID: "enterprise", DisplayName: "企业档", Kind: setting.HarnessCatalogKindText},
	})

	changed := requestHarnessTierCatalog(t, etag, true)
	require.Equal(t, http.StatusOK, changed.Code, "a changed catalog must not answer 304")
	assert.NotEqual(t, etag, changed.Header().Get("ETag"))
}

// TestGetHarnessTierCatalogVersionAdvancesOnlyOnContentChange pins the monotonic
// version: repeated reads of identical content reuse it, while a real content
// change advances it.
func TestGetHarnessTierCatalogVersionAdvancesOnlyOnContentChange(t *testing.T) {
	setupHarnessTierCatalogTestDB(t, map[string][]string{
		"economy":    {harnessTierCatalogTestTextModelA},
		"enterprise": {harnessTierCatalogTestTextModelB},
	})
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Kind: setting.HarnessCatalogKindText},
		{ID: "enterprise", DisplayName: "企业档", Kind: setting.HarnessCatalogKindText},
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

	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档（新）", Kind: setting.HarnessCatalogKindText},
		{ID: "enterprise", DisplayName: "企业档", Kind: setting.HarnessCatalogKindText},
	})

	advanced := catalogVersion(requestHarnessTierCatalog(t, "", true))
	assert.Greater(t, advanced, baseline, "a content change must advance the version")
	assert.Equal(t, advanced, catalogVersion(requestHarnessTierCatalog(t, "", true)),
		"the advanced version must then stay stable")
}

// TestGetHarnessTierCatalogRejectsUnverifiedCaller pins the 403 contract. The
// pool list is commercial information, so an unauthenticated caller learns
// nothing about it.
func TestGetHarnessTierCatalogRejectsUnverifiedCaller(t *testing.T) {
	setupHarnessTierCatalogTestDB(t, map[string][]string{
		"economy": {harnessTierCatalogTestTextModelA},
	})
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Kind: setting.HarnessCatalogKindText},
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

// TestBuildHarnessTierCatalogUnionsPoolCapabilities pins the per-pool capability
// aggregation: it is the union over the pool's routable models in a stable order,
// and an empty pool reports empty arrays rather than an invented capability.
func TestBuildHarnessTierCatalogUnionsPoolCapabilities(t *testing.T) {
	setupHarnessTierCatalogTestDB(t, map[string][]string{
		"economy": {harnessTierCatalogTestTextModelA, harnessTierCatalogTestVideoModel},
	})
	priceHarnessTierCatalogModels(t, harnessTierCatalogTestTextModelA, harnessTierCatalogTestVideoModel)
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Kind: setting.HarnessCatalogKindText},
		{ID: "video-per-call", DisplayName: "视频按次", Kind: setting.HarnessCatalogKindVideo, Billing: setting.HarnessCatalogBillingPerCall},
	})

	items := buildHarnessTierCatalog(map[string]model.Pricing{
		harnessTierCatalogTestTextModelA: {
			ModelName:              harnessTierCatalogTestTextModelA,
			SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI},
		},
		harnessTierCatalogTestVideoModel: {
			ModelName:              harnessTierCatalogTestVideoModel,
			SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAIVideo},
		},
	})

	require.Len(t, items, 2, "both pools are advertised")
	assert.Equal(t, "economy", items[0].ID)
	assert.Equal(t, []string{"chat", "stream", "tools"}, items[0].Capabilities,
		"capabilities are unioned over the pool's models")

	// The video pool has no channels yet, so it reports empty arrays.
	assert.Equal(t, "video-per-call", items[1].ID)
	assert.Equal(t, []string{}, items[1].Models)
	assert.Equal(t, []string{}, items[1].Capabilities)
}

// A channel may list a model the gateway has no price for. Advertising it would
// hand the platform a model that fails on the first call with "model price is not
// configured", so the catalog must omit it instead.
func TestBuildHarnessTierCatalogOmitsUnpricedModels(t *testing.T) {
	setupHarnessTierCatalogTestDB(t, map[string][]string{
		"economy": {"priced-model", "unpriced-model"},
	})
	configureHarnessTierCatalogTiers(t, []setting.HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Kind: setting.HarnessCatalogKindText},
	})

	pricing := map[string]model.Pricing{
		"priced-model": {
			ModelName:              "priced-model",
			SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI},
		},
		"unpriced-model": {
			ModelName:              "unpriced-model",
			SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI},
		},
	}

	// With no price configured for either model, nothing may be advertised: an
	// unpriced model is not callable, so listing it would be a false promise.
	items := buildHarnessTierCatalog(pricing)
	require.Len(t, items, 1)
	assert.Equal(t, []string{}, items[0].Models,
		"models without any configured price must not be advertised")

	// Pricing one of them makes exactly that model appear.
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"priced-model": 1.5}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
	})

	items = buildHarnessTierCatalog(pricing)
	require.Len(t, items, 1)
	assert.Equal(t, []string{"priced-model"}, items[0].Models,
		"only the priced model is advertised")
	assert.Equal(t, []string{"chat", "stream", "tools"}, items[0].Capabilities)
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
		setupHarnessTierCatalogTestDB(t, map[string][]string{
			"economy": {harnessTierCatalogTestTextModelA},
		})
		recorder := requestHealth()
		require.Equal(t, http.StatusOK, recorder.Code)
		body := healthBody(t, recorder)
		assert.Equal(t, "ok", body.Status)
		assert.Equal(t, "v2", body.InterfaceVersion)
		assert.Equal(t, "45fbb149", body.UpstreamBaseline)
		assert.Regexp(t, timePattern, body.Time, "time must be UTC RFC3339 with millisecond precision")
	})

	t.Run("degraded when no channel is enabled", func(t *testing.T) {
		setupHarnessTierCatalogTestDB(t, nil)
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
