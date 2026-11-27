package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestResponseOpenAI2GeminiReportsCachedContentTokens(t *testing.T) {
	content := "ok"
	openAIResponse := &dto.OpenAITextResponse{
		Id: "chatcmpl-cache-test",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.Usage{
			PromptTokens:     1000,
			CompletionTokens: 5,
			TotalTokens:      1005,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens: 800,
			},
		},
	}

	geminiResponse := ResponseOpenAI2Gemini(openAIResponse, nil)

	require.NotNil(t, geminiResponse)
	require.Equal(t, 800, geminiResponse.UsageMetadata.CachedContentTokenCount)
	require.Equal(t, 1000, geminiResponse.UsageMetadata.PromptTokenCount)
	require.Equal(t, 5, geminiResponse.UsageMetadata.CandidatesTokenCount)
}

func TestStreamResponseOpenAI2GeminiReportsCachedContentTokens(t *testing.T) {
	content := "ok"
	openAIResponse := &dto.ChatCompletionsStreamResponse{
		Id: "chatcmpl-cache-stream-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Content: &content,
				},
			},
		},
		Usage: &dto.Usage{
			PromptTokens:     1000,
			CompletionTokens: 5,
			TotalTokens:      1005,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens: 800,
			},
		},
	}

	geminiResponse := StreamResponseOpenAI2Gemini(openAIResponse, &relaycommon.RelayInfo{})

	require.NotNil(t, geminiResponse)
	require.Equal(t, 800, geminiResponse.UsageMetadata.CachedContentTokenCount)
	require.Equal(t, 1000, geminiResponse.UsageMetadata.PromptTokenCount)
	require.Equal(t, 5, geminiResponse.UsageMetadata.CandidatesTokenCount)
	require.Equal(t, 1005, geminiResponse.UsageMetadata.TotalTokenCount)
}

func TestGeminiConvertersIgnoreNonPositiveCachedTokens(t *testing.T) {
	tests := []struct {
		name        string
		usage       dto.Usage
		streamUsage *dto.Usage
		want        int
	}{
		{
			name:        "zero cached tokens",
			usage:       dto.Usage{PromptTokens: 10},
			streamUsage: &dto.Usage{PromptTokens: 10},
			want:        0,
		},
		{
			name: "negative cached tokens",
			usage: dto.Usage{
				PromptTokens: 10,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: -5,
				},
			},
			streamUsage: &dto.Usage{
				PromptTokens: 10,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: -5,
				},
			},
			want: 0,
		},
		{
			name:        "no usage",
			usage:       dto.Usage{},
			streamUsage: nil,
			want:        0,
		},
	}

	content := "ok"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openAIResponse := &dto.OpenAITextResponse{
				Choices: []dto.OpenAITextResponseChoice{
					{
						Message:      dto.Message{Role: "assistant", Content: content},
						FinishReason: "stop",
					},
				},
				Usage: tt.usage,
			}
			geminiResponse := ResponseOpenAI2Gemini(openAIResponse, nil)
			require.NotNil(t, geminiResponse)
			require.Equal(t, tt.want, geminiResponse.UsageMetadata.CachedContentTokenCount)

			streamResponse := &dto.ChatCompletionsStreamResponse{
				Choices: []dto.ChatCompletionsStreamResponseChoice{
					{
						Index: 0,
						Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
							Content: &content,
						},
					},
				},
				Usage: tt.streamUsage,
			}
			streamGeminiResponse := StreamResponseOpenAI2Gemini(streamResponse, &relaycommon.RelayInfo{})
			require.NotNil(t, streamGeminiResponse)
			require.Equal(t, tt.want, streamGeminiResponse.UsageMetadata.CachedContentTokenCount)
		})
	}
}

func TestGeminiCachedContentTokenCountFallsBackToInputTokensDetails(t *testing.T) {
	require.Equal(t, 0, geminiCachedContentTokenCount(nil))
	require.Equal(t, 500, geminiCachedContentTokenCount(&dto.Usage{
		InputTokensDetails: &dto.InputTokenDetails{CachedTokens: 500},
	}))
	require.Equal(t, 800, geminiCachedContentTokenCount(&dto.Usage{
		PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 800},
		InputTokensDetails:  &dto.InputTokenDetails{CachedTokens: 500},
	}))
}
