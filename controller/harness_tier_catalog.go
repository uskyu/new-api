package controller

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

// harnessCatalogReadScope is the contract scope that grants catalog access.
const harnessCatalogReadScope = "gateway.catalog.read"

// harnessTierCatalogETagNamespace scopes the catalog ETag, so the digest can
// never collide with an ETag minted for another resource.
const harnessTierCatalogETagNamespace = "harness-tier-catalog:v1"

// harnessTierCatalogCapability* are the only capability names the contract
// allows. They are an enum, not free text.
const (
	harnessTierCatalogCapabilityChat   = "chat"
	harnessTierCatalogCapabilityStream = "stream"
	harnessTierCatalogCapabilityTools  = "tools"
)

// harnessTierCatalogResponse is the A3 body. The contract fixes this literal
// shape, so it is written directly and never wrapped in common.ApiSuccess: the
// platform reads "data" as an array of model objects, which the dashboard
// envelope ({success,message,data}) would replace with an object.
type harnessTierCatalogResponse struct {
	Object         string                   `json:"object"`
	Data           []harnessTierCatalogItem `json:"data"`
	CatalogVersion string                   `json:"catalog_version"`
	ETag           string                   `json:"etag"`
}

// harnessTierCatalogItem is one advertised resource pool. Each pool lists the
// real model ids it can route, so the platform can address a model directly and
// pick the pool it wants; channels, upstream vendors and procurement prices stay
// internal.
//
// Kind and Billing repeat the operator's pool labels so the platform can group
// the offering without parsing pool ids. They are the pool's own labels rather
// than something derived from its models: a video pool is advertised while it is
// still empty, so the platform can integrate against the final shape before the
// channels exist.
//
// Models and Capabilities are always serialised as arrays, never null: the
// platform iterates them directly, so an absent value would break its parser.
type harnessTierCatalogItem struct {
	ID           string   `json:"id"`
	Object       string   `json:"object"`
	DisplayName  string   `json:"display_name"`
	Kind         string   `json:"kind"`
	Billing      string   `json:"billing,omitempty"`
	Models       []string `json:"models"`
	Capabilities []string `json:"capabilities"`
}

// harnessTierCatalogCapabilitySet answers which of the three enum capabilities a
// downstream endpoint type implies.
//
// The mapping is derived from this gateway's own request DTOs rather than from
// provider marketing data, so it stays true to what the gateway actually relays:
//   - chat: the endpoint family serves conversational completions
//     (/v1/chat/completions, /v1/messages, :generateContent, /v1/responses,
//     /v1/alpha/search). Embeddings, image generation, rerank, and video
//     endpoints are excluded because they produce no assistant turn.
//   - stream: the relay path for that endpoint writes server-sent events and the
//     request DTO carries a stream flag (OpenAI chat, Responses, Anthropic
//     messages, Gemini generateContent, alpha search). Compact, embeddings,
//     image generation, rerank, and video are one-shot and are not streamed.
//   - tools: the request DTO for that endpoint carries a tools/tool_choice
//     passthrough (OpenAI chat, Responses, Anthropic messages, Gemini), so a
//     tool-bearing request can actually be forwarded. Alpha search has no tools
//     field, so it does not advertise tools.
//
// An endpoint type absent from this table (including an empty
// SupportedEndpointTypes, i.e. "unknown") contributes nothing: the catalog must
// never claim a capability the gateway cannot verify.
type harnessTierCatalogCapabilitySet struct {
	Chat   bool
	Stream bool
	Tools  bool
}

var harnessTierCatalogCapabilitiesByEndpoint = map[constant.EndpointType]harnessTierCatalogCapabilitySet{
	constant.EndpointTypeOpenAI:            {Chat: true, Stream: true, Tools: true},
	constant.EndpointTypeAnthropic:         {Chat: true, Stream: true, Tools: true},
	constant.EndpointTypeGemini:            {Chat: true, Stream: true, Tools: true},
	constant.EndpointTypeOpenAIResponse:    {Chat: true, Stream: true, Tools: true},
	constant.EndpointTypeOpenAIAlphaSearch: {Chat: true, Stream: true},
	// Compaction rewrites a conversation into a compact form and returns no
	// streamed tokens, so it advertises chat only.
	constant.EndpointTypeOpenAIResponseCompact: {Chat: true},
}

// harnessTierCatalogCapabilities maps a pricing entry's supported endpoint types
// to the contract enum. The result is de-duplicated and emitted in the fixed
// order chat, stream, tools; no endpoint type maps to anything, the result is an
// empty slice (never nil, never an invented capability).
func harnessTierCatalogCapabilities(endpointTypes []constant.EndpointType) []string {
	capabilities := make([]string, 0, 3)
	union := harnessTierCatalogCapabilitySet{}
	for _, endpointType := range endpointTypes {
		supported := harnessTierCatalogCapabilitiesByEndpoint[endpointType]
		union.Chat = union.Chat || supported.Chat
		union.Stream = union.Stream || supported.Stream
		union.Tools = union.Tools || supported.Tools
	}
	if union.Chat {
		capabilities = append(capabilities, harnessTierCatalogCapabilityChat)
	}
	if union.Stream {
		capabilities = append(capabilities, harnessTierCatalogCapabilityStream)
	}
	if union.Tools {
		capabilities = append(capabilities, harnessTierCatalogCapabilityTools)
	}
	return capabilities
}

// buildHarnessTierCatalog renders every configured pool, each with the real
// model ids it can route.
//
// Every pool is advertised, including one with no channels yet: the platform has
// to see the whole offering (text pools plus both video billing modes) before the
// channels exist, otherwise it cannot build its model picker against the final
// shape. A pool's model ids come from its channels, never from a stored copy, so
// the platform's model list follows channel changes automatically and the catalog
// can never advertise a model the pool cannot route.
func buildHarnessTierCatalog(pricingByModel map[string]model.Pricing) []harnessTierCatalogItem {
	config := setting.GetHarnessCatalogConfig()
	items := make([]harnessTierCatalogItem, 0, len(config.Tiers))
	for _, tier := range config.Tiers {
		// ResourcePoolModels returns a non-nil empty slice for a pool that has no
		// channels, which serialises as [] rather than null.
		models := model.ResourcePoolModels(tier.ID)
		// Capabilities are the union over the pool's routable models; a model the
		// gateway cannot price contributes nothing, matching what A1-A9 can serve.
		capabilities := make([]string, 0, 3)
		for _, modelName := range models {
			pricing, ok := pricingByModel[modelName]
			if !ok {
				continue
			}
			capabilities = harnessTierCatalogMergeCapabilities(
				capabilities,
				harnessTierCatalogCapabilities(pricing.SupportedEndpointTypes),
			)
		}
		items = append(items, harnessTierCatalogItem{
			ID:           tier.ID,
			Object:       "model",
			DisplayName:  tier.DisplayName,
			Kind:         tier.Kind,
			Billing:      tier.Billing,
			Models:       models,
			Capabilities: capabilities,
		})
	}
	return items
}

// harnessTierCatalogMergeCapabilities unions two enum slices while keeping the
// fixed chat, stream, tools order, so the fingerprint cannot depend on the order
// tiers happen to list their backing models in.
func harnessTierCatalogMergeCapabilities(into, add []string) []string {
	present := make(map[string]struct{}, len(into)+len(add))
	for _, capability := range into {
		present[capability] = struct{}{}
	}
	for _, capability := range add {
		present[capability] = struct{}{}
	}
	merged := make([]string, 0, 3)
	for _, capability := range []string{
		harnessTierCatalogCapabilityChat,
		harnessTierCatalogCapabilityStream,
		harnessTierCatalogCapabilityTools,
	} {
		if _, ok := present[capability]; ok {
			merged = append(merged, capability)
		}
	}
	return merged
}

// harnessTierCatalogPricingByModel indexes the priced models once per request.
func harnessTierCatalogPricingByModel() map[string]model.Pricing {
	pricings := model.GetPricing()
	byModel := make(map[string]model.Pricing, len(pricings))
	for _, pricing := range pricings {
		byModel[pricing.ModelName] = pricing
	}
	return byModel
}

// harnessTierCatalogErrorEnvelope is the contract error body for A3. It is
// declared here rather than reusing another endpoint's envelope so this wire
// shape stays exactly the three agreed fields.
type harnessTierCatalogErrorEnvelope struct {
	Error harnessTierCatalogErrorBody `json:"error"`
}

type harnessTierCatalogErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// writeHarnessTierCatalogError renders the contract error body.
func writeHarnessTierCatalogError(c *gin.Context, status int, code, message string) {
	c.JSON(status, harnessTierCatalogErrorEnvelope{Error: harnessTierCatalogErrorBody{
		Code:      code,
		Message:   message,
		Retryable: false,
	}})
}

// GetHarnessTierCatalog serves A3.
//
// Only a verified Harness principal gets an answer: the tier catalog is a
// platform-only view, and an unauthenticated caller must not learn which tiers an
// operator sells. The body is a literal contract shape, and the response is
// revalidatable rather than uncacheable: the weak ETag plus Cache-Control
// no-cache (deliberately NOT no-store, which would forbid the client from
// reusing the body and make every conditional request a full 200) lets the
// platform long-poll with If-None-Match and receive an empty 304.
func GetHarnessTierCatalog(c *gin.Context) {
	principal, ok := middleware.GetHarnessPrincipal(c)
	if !ok {
		writeHarnessTierCatalogError(c, http.StatusForbidden,
			"GATEWAY_UNAUTHORIZED", "a verified Harness credential is required")
		return
	}
	// The contract defines a dedicated read scope for the catalog, so a credential
	// granted only ledger access must not learn which tiers an operator sells.
	if !slices.Contains(principal.Scopes, harnessCatalogReadScope) {
		writeHarnessTierCatalogError(c, http.StatusForbidden,
			"GATEWAY_UNAUTHORIZED", "the credential lacks the "+harnessCatalogReadScope+" scope")
		return
	}

	items := buildHarnessTierCatalog(harnessTierCatalogPricingByModel())

	// The fingerprint covers the rendered tiers only, never the version: a
	// version inside its own fingerprint would advance on every read.
	content, err := common.Marshal(items)
	if err != nil {
		writeHarnessTierCatalogError(c, http.StatusInternalServerError,
			"GATEWAY_UPSTREAM", "the gateway could not render its catalog")
		return
	}
	etag := common.ETagFor(harnessTierCatalogETagNamespace, string(content))

	config, advanced := setting.AdvanceHarnessCatalogVersion(etag)
	if advanced {
		// Persisting keeps the monotonic version across restarts. A failure is
		// logged but not fatal: the in-memory version is already correct for
		// this process, and refusing to serve a valid catalog would be worse.
		persistHarnessTierCatalogVersion(config)
	}

	response := harnessTierCatalogResponse{
		Object:         "list",
		Data:           items,
		CatalogVersion: strconv.FormatInt(config.HarnessCatalogVersion, 10),
		ETag:           etag,
	}
	body, err := common.Marshal(response)
	if err != nil {
		writeHarnessTierCatalogError(c, http.StatusInternalServerError,
			"GATEWAY_UPSTREAM", "the gateway could not render its catalog")
		return
	}

	c.Header("ETag", etag)
	c.Header("Cache-Control", "no-cache")
	c.Header("Vary", "Authorization, Accept-Encoding")

	if common.ETagMatches(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// persistHarnessTierCatalogVersion stores the version and fingerprint in the
// catalog option row. It runs outside the catalog lock so the option write path
// (which takes its own lock and may itself read options) cannot deadlock.
func persistHarnessTierCatalogVersion(config setting.HarnessCatalogConfig) {
	if model.DB == nil {
		// Without a database there is nowhere to persist the version; the
		// in-memory counter still serves this process correctly.
		return
	}
	value, err := setting.HarnessCatalogConfigJSON(config)
	if err != nil {
		common.SysError(common.LogText("failed to encode HarnessCatalog option: %s", err.Error()))
		return
	}
	if err := model.UpdateOption(setting.HarnessCatalogOptionKey, value); err != nil {
		common.SysError(common.LogText("failed to persist HarnessCatalog version: %s", err.Error()))
	}
}
