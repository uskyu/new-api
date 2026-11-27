package gemini

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newGeminiChatResponse(parts ...dto.GeminiPart) *dto.GeminiChatResponse {
	return &dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Index: 0,
				Content: dto.GeminiChatContent{
					Role:  "model",
					Parts: parts,
				},
			},
		},
	}
}

func TestResponseGeminiChat2OpenAIJoinsTextPartsVerbatim(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	response := newGeminiChatResponse(
		dto.GeminiPart{Text: `{"a":`},
		dto.GeminiPart{Text: `"b\n`},
		dto.GeminiPart{Text: `"c"}`},
	)

	converted := responseGeminiChat2OpenAI(c, response)
	require.Len(t, converted.Choices, 1)
	require.Equal(t, `{"a":"b\n"c"}`, converted.Choices[0].Message.StringContent())
}

func TestResponseGeminiChat2OpenAIPreservesStandaloneNewlinePart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	tests := []struct {
		name  string
		parts []dto.GeminiPart
		want  string
	}{
		{
			name: "newline between text parts",
			parts: []dto.GeminiPart{
				{Text: "first"},
				{Text: "\n"},
				{Text: "second"},
			},
			want: "first\nsecond",
		},
		{
			name:  "only newline",
			parts: []dto.GeminiPart{{Text: "\n"}},
			want:  "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converted := responseGeminiChat2OpenAI(c, newGeminiChatResponse(tt.parts...))
			require.Len(t, converted.Choices, 1)
			require.Equal(t, tt.want, converted.Choices[0].Message.StringContent())
		})
	}
}

func TestResponseGeminiChat2OpenAISeparatesImageMarkdownFromText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	response := newGeminiChatResponse(
		dto.GeminiPart{Text: "before"},
		dto.GeminiPart{InlineData: &dto.GeminiInlineData{
			MimeType: "image/png",
			Data:     "AAAA",
		}},
		dto.GeminiPart{Text: "after"},
	)

	converted := responseGeminiChat2OpenAI(c, response)
	require.Len(t, converted.Choices, 1)
	require.Equal(t,
		"before\n![image](data:image/png;base64,AAAA)\nafter",
		converted.Choices[0].Message.StringContent(),
	)
}

func TestStreamResponseGeminiChat2OpenAIJoinsTextPartsVerbatim(t *testing.T) {
	response := newGeminiChatResponse(
		dto.GeminiPart{Text: `{"a":`},
		dto.GeminiPart{Text: `"b\n`},
		dto.GeminiPart{Text: `"c"}`},
	)

	converted, isStop := streamResponseGeminiChat2OpenAI(response)
	require.False(t, isStop)
	require.Len(t, converted.Choices, 1)
	require.Equal(t, `{"a":"b\n"c"}`, converted.Choices[0].Delta.GetContentString())
}

func TestStreamResponseGeminiChat2OpenAIPreservesNewlinePart(t *testing.T) {
	response := newGeminiChatResponse(
		dto.GeminiPart{Text: "first"},
		dto.GeminiPart{Text: "\n"},
		dto.GeminiPart{Text: "second"},
	)

	converted, isStop := streamResponseGeminiChat2OpenAI(response)
	require.False(t, isStop)
	require.Len(t, converted.Choices, 1)
	require.Equal(t, "first\nsecond", converted.Choices[0].Delta.GetContentString())
}

func TestStreamResponseGeminiChat2OpenAIPreservesLoneNewlinePart(t *testing.T) {
	response := newGeminiChatResponse(dto.GeminiPart{Text: "\n"})

	converted, isStop := streamResponseGeminiChat2OpenAI(response)
	require.False(t, isStop)
	require.Len(t, converted.Choices, 1)
	require.Equal(t, "\n", converted.Choices[0].Delta.GetContentString())
}

func TestStreamResponseGeminiChat2OpenAISeparatesImageMarkdownFromText(t *testing.T) {
	response := newGeminiChatResponse(
		dto.GeminiPart{Text: "before"},
		dto.GeminiPart{InlineData: &dto.GeminiInlineData{
			MimeType: "image/png",
			Data:     "AAAA",
		}},
		dto.GeminiPart{Text: "after"},
	)

	converted, isStop := streamResponseGeminiChat2OpenAI(response)
	require.False(t, isStop)
	require.Len(t, converted.Choices, 1)
	require.Equal(t,
		"before\n![image](data:image/png;base64,AAAA)\nafter",
		converted.Choices[0].Delta.GetContentString(),
	)
}
