package gemini

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newEmbeddingTestContext(t *testing.T) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName: "gemini-embedding-001",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-embedding-001",
		},
	}
	info.SetEstimatePromptTokens(3)
	return c, info
}

func newHTTPResponse(body string) *http.Response {
	return &http.Response{
		Body: io.NopCloser(bytes.NewReader([]byte(body))),
	}
}

func TestNativeGeminiEmbeddingHandlerUsesUpstreamUsageMetadata(t *testing.T) {
	c, info := newEmbeddingTestContext(t)

	usage, newAPIError := NativeGeminiEmbeddingHandler(
		c,
		newHTTPResponse(`{"embedding":{"values":[0.1]},"usageMetadata":{"promptTokenCount":7}}`),
		info,
	)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 7, usage.PromptTokens)
	require.Equal(t, 7, usage.TotalTokens)
}

func TestNativeGeminiEmbeddingHandlerBatchUsesUpstreamUsageMetadata(t *testing.T) {
	c, info := newEmbeddingTestContext(t)
	info.IsGeminiBatchEmbedding = true

	usage, newAPIError := NativeGeminiEmbeddingHandler(
		c,
		newHTTPResponse(`{"embeddings":[{"values":[0.1]}],"usageMetadata":{"promptTokenCount":9}}`),
		info,
	)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 9, usage.PromptTokens)
	require.Equal(t, 9, usage.TotalTokens)
}

func TestGeminiEmbeddingHandlerUsesUpstreamUsageMetadata(t *testing.T) {
	c, info := newEmbeddingTestContext(t)

	usage, newAPIError := GeminiEmbeddingHandler(
		c,
		info,
		newHTTPResponse(`{"embeddings":[{"values":[0.1]}],"usageMetadata":{"promptTokenCount":9}}`),
	)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 9, usage.PromptTokens)
	require.Equal(t, 9, usage.TotalTokens)
}

func TestNativeGeminiEmbeddingHandlerFallsBackToEstimateWithoutUsageMetadata(t *testing.T) {
	c, info := newEmbeddingTestContext(t)

	usage, newAPIError := NativeGeminiEmbeddingHandler(
		c,
		newHTTPResponse(`{"embedding":{"values":[0.1]}}`),
		info,
	)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 3, usage.TotalTokens)
}

func TestGeminiEmbeddingHandlerFallsBackToEstimateWithoutUsageMetadata(t *testing.T) {
	c, info := newEmbeddingTestContext(t)

	usage, newAPIError := GeminiEmbeddingHandler(
		c,
		info,
		newHTTPResponse(`{"embeddings":[{"values":[0.1]}]}`),
	)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 3, usage.TotalTokens)
}

func TestGeminiEmbeddingUsageMetadataConvertsToGeminiUsageMetadata(t *testing.T) {
	metadata := &dto.GeminiEmbeddingUsageMetadata{
		PromptTokenCount: 9,
		PromptTokenDetails: []dto.GeminiPromptTokensDetails{
			{Modality: "TEXT", TokenCount: 9},
		},
	}

	upstream := metadata.GeminiUsageMetadata()
	require.NotNil(t, upstream)
	require.Equal(t, 9, upstream.PromptTokenCount)
	require.Equal(t, 9, upstream.TotalTokenCount)
	require.Equal(t, metadata.PromptTokenDetails, upstream.PromptTokensDetails)

	require.Nil(t, (*dto.GeminiEmbeddingUsageMetadata)(nil).GeminiUsageMetadata())
	require.Nil(t, (&dto.GeminiEmbeddingUsageMetadata{PromptTokenCount: 0}).GeminiUsageMetadata())
}
