package gemini

import (
	"testing"

	"github.com/MoeForge/moego/ojson"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
)

func TestPatchGeminiInlineDataMimeJSONPreservesUnknownFieldsAndBothSpellings(t *testing.T) {
	raw := []byte(`{"modelVersion":"gemini-3.8-flash","responseId":"resp-1","candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"abc"}},{"inline_data":{"mime_type":"image/png","data":"def"}}]}}]}`)
	corrections := []dto.GeminiInlineDataMimeCorrection{
		{CandidateIndex: 0, PartIndex: 0, MimeType: "image/jpeg"},
		{CandidateIndex: 0, PartIndex: 1, MimeType: "image/webp"},
	}

	patched := patchGeminiInlineDataMimeJSON(raw, corrections)
	assert.Equal(t, "image/jpeg", ojson.GetBytes(patched, "candidates.0.content.parts.0.inlineData.mimeType").Str)
	assert.Equal(t, "image/webp", ojson.GetBytes(patched, "candidates.0.content.parts.1.inline_data.mime_type").Str)
	assert.Equal(t, "gemini-3.8-flash", ojson.GetBytes(patched, "modelVersion").Str)
	assert.Equal(t, "resp-1", ojson.GetBytes(patched, "responseId").Str)
	assert.Equal(t, "abc", ojson.GetBytes(patched, "candidates.0.content.parts.0.inlineData.data").Str)
}
