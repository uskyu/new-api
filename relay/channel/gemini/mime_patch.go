package gemini

import (
	"fmt"

	"github.com/MoeForge/moego/ojson"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// patchGeminiInlineDataMimeJSON patches only the MIME field of each corrected
// inlineData part. It never remarshals GeminiChatResponse: that would drop
// unknown upstream fields such as modelVersion, responseId and grounding
// metadata. The camelCase and snake_case spellings are both handled because the
// upstream may use either.
func patchGeminiInlineDataMimeJSON(raw []byte, corrections []dto.GeminiInlineDataMimeCorrection) []byte {
	for _, correction := range corrections {
		for _, path := range []string{
			fmt.Sprintf("candidates.%d.content.parts.%d.inlineData.mimeType",
				correction.CandidateIndex, correction.PartIndex),
			fmt.Sprintf("candidates.%d.content.parts.%d.inline_data.mime_type",
				correction.CandidateIndex, correction.PartIndex),
		} {
			if !ojson.GetBytes(raw, path).Exists() {
				continue
			}
			patched, err := ojson.SetBytes(raw, path, correction.MimeType)
			if err != nil {
				return raw
			}
			raw = patched
		}
	}
	return raw
}
