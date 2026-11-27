package dto

import (
	"encoding/base64"
	"testing"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiChatResponseUsageMetadataPresence(t *testing.T) {
	var missing GeminiChatResponse
	require.NoError(t, kitutil.Unmarshal([]byte(`{"candidates":[]}`), &missing))
	assert.False(t, missing.HasUsageMetadata)
	assert.Nil(t, missing.GetUsageMetadata())

	var empty GeminiChatResponse
	require.NoError(t, kitutil.Unmarshal([]byte(`{"candidates":[],"usageMetadata":{}}`), &empty))
	assert.True(t, empty.HasUsageMetadata)
	require.NotNil(t, empty.GetUsageMetadata())
	assert.False(t, HasGeminiUsageMetadataTokens(empty.GetUsageMetadata()))

	var populated GeminiChatResponse
	require.NoError(t, kitutil.Unmarshal([]byte(`{"candidates":[],"usageMetadata":{"promptTokenCount":3}}`), &populated))
	assert.True(t, populated.HasUsageMetadata)
	require.NotNil(t, populated.GetUsageMetadata())
	assert.True(t, HasGeminiUsageMetadataTokens(populated.GetUsageMetadata()))
}

func TestGeminiChatResponseMarshalKeepsUsageMetadataField(t *testing.T) {
	data, err := kitutil.Marshal(GeminiChatResponse{})
	require.NoError(t, err)
	assert.Contains(t, string(data), `"usageMetadata"`)
}

func TestGeminiInlineDataMimeTypeIsCorrectedFromBytes(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d}
	webp := append([]byte("RIFF"), append([]byte{0x00, 0x00, 0x00, 0x00}, []byte("WEBP")...)...)

	response := GeminiChatResponse{Candidates: []GeminiChatCandidate{{
		Content: GeminiChatContent{Parts: []GeminiPart{{
			InlineData: &GeminiInlineData{MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(jpeg)},
		}, {
			InlineData: &GeminiInlineData{MimeType: "", Data: base64.StdEncoding.EncodeToString(png)},
		}, {
			InlineData: &GeminiInlineData{MimeType: "application/octet-stream", Data: base64.StdEncoding.EncodeToString(webp)},
		}, {
			InlineData: &GeminiInlineData{MimeType: "video/mp4", Data: base64.StdEncoding.EncodeToString(jpeg)},
		}}},
	}}}

	corrections := response.NormalizeInlineDataMimeTypes()
	require.Len(t, corrections, 3)
	assert.Equal(t, "image/jpeg", response.Candidates[0].Content.Parts[0].InlineData.MimeType,
		"declared image/png must be corrected to the actual JPEG bytes")
	assert.Equal(t, "image/png", response.Candidates[0].Content.Parts[1].InlineData.MimeType,
		"an empty declaration must be filled from the actual PNG bytes")
	assert.Equal(t, "image/webp", response.Candidates[0].Content.Parts[2].InlineData.MimeType,
		"a generic octet-stream declaration must be corrected from the actual WebP bytes")
	assert.Equal(t, "video/mp4", response.Candidates[0].Content.Parts[3].InlineData.MimeType,
		"an explicit non-image declaration must not be re-labelled as an image")
}

func TestGeminiInlineDataMimeTypeKeepsMatchingDeclaration(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d}
	response := GeminiChatResponse{Candidates: []GeminiChatCandidate{{
		Content: GeminiChatContent{Parts: []GeminiPart{{
			InlineData: &GeminiInlineData{MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(png)},
		}}},
	}}}
	assert.Empty(t, response.NormalizeInlineDataMimeTypes())
	assert.Equal(t, "image/png", response.Candidates[0].Content.Parts[0].InlineData.MimeType)
}
