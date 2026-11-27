package router

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHarnessP0ModelCallE2E is the local stand-in for the Harness joint
// acceptance run: a signed Harness JWT, a real HTTP engine, a SQLite database,
// a mock OpenAI upstream and the actual relay/ledger chain. It proves the
// pieces that cannot be verified with an ordinary sk- token: 80-character Run
// ID acceptance, B1 run_id echo, 81-character refusal before dispatch, and the
// A3 ETag/304 path on the same engine.
func TestHarnessP0ModelCallE2E(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupRelayRouterTestDB(t)
	oldOptionMap := common.OptionMap
	oldRouting := setting.GetIntelligentRoutingConfig()
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.OptionMap = oldOptionMap
		require.NoError(t, setting.UpdateIntelligentRoutingConfig(oldRouting))
	})

	require.NoError(t, model.DB.AutoMigrate(
		&model.Option{},
		&model.Channel{},
		&model.HarnessLedgerEvent{},
		&model.HarnessSettlement{},
		&model.UserSubscription{},
		&model.Log{},
	))
	model.InitOptionMap()

	var upstreamCalls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-p0-e2e","object":"chat.completion","created":1,"model":"mock-gpt","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":3,"total_tokens":6}}`))
	}))
	t.Cleanup(upstream.Close)

	user := &model.User{Username: "p0-e2e", Status: common.UserStatusEnabled, Group: "default", Quota: 1 << 30}
	require.NoError(t, model.DB.Create(user).Error)
	serviceToken := &model.Token{
		Name: "p0-e2e-service", Key: "p0e2eservicekey", UserId: user.Id,
		Status: common.TokenStatusEnabled, ExpiredTime: -1,
		UnlimitedQuota: true, Group: "default",
	}
	require.NoError(t, model.DB.Create(serviceToken).Error)

	baseURL := upstream.URL
	channel := &model.Channel{
		Type: constant.ChannelTypeOpenAI, Key: "p0-e2e-upstream-key",
		Name: "p0-e2e-mock", BaseURL: &baseURL, Models: "mock-gpt",
		Group: "default", Status: common.ChannelStatusEnabled,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group: "default", Model: "mock-gpt", ChannelId: channel.Id, Enabled: true,
	}).Error)
	require.NoError(t, setting.UpdateIntelligentRoutingConfig(setting.IntelligentRoutingConfig{
		PricingGroupOrder: []string{"default"},
		Groups: []setting.IntelligentRoutingGroup{{
			ID: "chat", Name: "Chat", ChannelOrder: []int{channel.Id},
		}},
	}))
	require.NoError(t, model.UpdateOption("ModelRatio", `{"mock-gpt":1.0}`))
	model.InitChannelCache()
	model.InvalidatePricingCache()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	require.NoError(t, model.UpdateOption(setting.HarnessJWKSURLOptionKey, harnessE2EJWKS(t, key, "kid-p0")))
	require.NoError(t, model.UpdateOption(setting.HarnessJWTIssuerOptionKey, "harness"))
	require.NoError(t, model.UpdateOption(setting.HarnessJWTAudienceOptionKey, "new-api"))
	subjectMap, err := json.Marshal(map[string]int{"harness-gateway": serviceToken.Id})
	require.NoError(t, err)
	require.NoError(t, model.UpdateOption(setting.HarnessSubjectTokenMapOptionKey, string(subjectMap)))

	engine := gin.New()
	engine.Use(middleware.RequestId())
	SetRelayRouter(engine)
	SetHarnessInternalRouter(engine)

	jwtToken := harnessE2ESign(t, key, "kid-p0",
		"gateway.chat", "gateway.usage.read", "gateway.cost.read", "gateway.catalog.read")
	authHeader := "Bearer " + jwtToken
	runID := strings.Repeat("r", 80)
	requestID := "hrq_p0_e2e_model"
	body := `{"model":"mock-gpt","messages":[{"role":"user","content":"Reply with OK"}],"max_tokens":8}`

	callModel := func(run string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", authHeader)
		request.Header.Set("X-Harness-Request-Id", requestID)
		request.Header.Set("X-Harness-Run-Id", run)
		engine.ServeHTTP(recorder, request)
		return recorder
	}

	valid := callModel(runID)
	require.Equal(t, http.StatusOK, valid.Code, "body=%s", valid.Body.String())
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	require.NoError(t, json.Unmarshal(valid.Body.Bytes(), &completion))
	require.NotEmpty(t, completion.Choices)
	assert.Equal(t, "OK", completion.Choices[0].Message.Content)
	assert.NotEmpty(t, valid.Header().Get(middleware.NewAPIRequestIdHeader))
	require.Equal(t, int64(1), upstreamCalls.Load())

	overlong := callModel(runID + "r")
	require.Equal(t, http.StatusBadRequest, overlong.Code, "body=%s", overlong.Body.String())
	assert.Contains(t, overlong.Body.String(), "REQUEST_INVALID")
	assert.Equal(t, int64(1), upstreamCalls.Load(), "an overlong run id must be refused before dispatch")

	// B1 batchGet is deliberately not watermarked, so the just-written event is
	// visible immediately and the run id echo can be checked without a sleep.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/gateway/usage-events:batchGet",
		strings.NewReader(`{"request_ids":["`+requestID+`"]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", authHeader)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, "body=%s", recorder.Body.String())
	var usageBatch struct {
		Items []struct {
			RequestId string `json:"request_id"`
			Found     bool   `json:"found"`
			Events    []struct {
				RequestId string `json:"request_id"`
				RunId     string `json:"run_id"`
				Usage     struct {
					InputTokens  *int `json:"input_tokens"`
					OutputTokens *int `json:"output_tokens"`
					TotalTokens  *int `json:"total_tokens"`
					SplitKnown   bool `json:"split_known"`
				} `json:"usage"`
			} `json:"events"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &usageBatch))
	require.Len(t, usageBatch.Items, 1)
	require.True(t, usageBatch.Items[0].Found)
	require.Len(t, usageBatch.Items[0].Events, 1)
	assert.Equal(t, requestID, usageBatch.Items[0].Events[0].RequestId)
	assert.Equal(t, runID, usageBatch.Items[0].Events[0].RunId, "B1 must echo the full 80-character run id")
	require.NotNil(t, usageBatch.Items[0].Events[0].Usage.TotalTokens)
	assert.Equal(t, 6, *usageBatch.Items[0].Events[0].Usage.TotalTokens)

	// A3 on the same engine: a Harness JWT gets the catalog, a conditional
	// request with the returned ETag gets 304.
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", authHeader)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, "body=%s", recorder.Body.String())
	etag := recorder.Header().Get("ETag")
	require.NotEmpty(t, etag)
	var catalog struct {
		Data []struct {
			ID     string   `json:"id"`
			Models []string `json:"models"`
		} `json:"data"`
		CatalogVersion string `json:"catalog_version"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &catalog))
	assert.NotEmpty(t, catalog.CatalogVersion)
	found := false
	for _, tier := range catalog.Data {
		if tier.ID == "chat" && slices.Contains(tier.Models, "mock-gpt") {
			found = true
		}
	}
	assert.True(t, found, "A3 chat tier must include the routable mock model")

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", authHeader)
	request.Header.Set("If-None-Match", etag)
	engine.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusNotModified, recorder.Code)

	// Keep the test's cleanup from racing a lingering cache refresh.
	time.Sleep(10 * time.Millisecond)
}
