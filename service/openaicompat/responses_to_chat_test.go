package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesResponseToChatPreservesTextAndToolCalls(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		ID:        "resp_1",
		CreatedAt: 123,
		Model:     "gpt-test",
		Output: []dto.ResponsesOutput{
			{
				Type: "message",
				Role: "assistant",
				Content: []dto.ResponsesOutputContent{
					{Type: "output_text", Text: "I will check that."},
				},
			},
			{
				Type:      "function_call",
				ID:        "fc_1",
				CallId:    "call_1",
				Name:      "lookup",
				Arguments: []byte(`{"query":"status"}`),
			},
		},
		Usage: &dto.Usage{
			InputTokens:  10,
			OutputTokens: 5,
		},
	}

	got, usage, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl-test")
	require.NoError(t, err)
	require.Equal(t, 10, usage.PromptTokens)
	require.Equal(t, 5, usage.CompletionTokens)
	require.Equal(t, 15, usage.TotalTokens)
	require.Len(t, got.Choices, 1)
	require.Equal(t, "tool_calls", got.Choices[0].FinishReason)
	require.Equal(t, "I will check that.", got.Choices[0].Message.StringContent())

	toolCalls := got.Choices[0].Message.ParseToolCalls()
	require.Len(t, toolCalls, 1)
	require.Equal(t, "call_1", toolCalls[0].ID)
	require.Equal(t, "lookup", toolCalls[0].Function.Name)
	require.Equal(t, `{"query":"status"}`, toolCalls[0].Function.Arguments)
}

func TestResponsesResponseToChatMapsIncompleteStatus(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		Status:            []byte(`"incomplete"`),
		IncompleteDetails: &dto.IncompleteDetails{Reason: "content_filter"},
		Output: []dto.ResponsesOutput{
			{
				Type: "message",
				Role: "assistant",
				Content: []dto.ResponsesOutputContent{
					{Type: "output_text", Text: "partial"},
				},
			},
		},
	}

	got, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl-test")
	require.NoError(t, err)
	require.Equal(t, "content_filter", got.Choices[0].FinishReason)
}

func TestResponsesResponseToChatPreservesReasoningText(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		Output: []dto.ResponsesOutput{
			{
				Type: "reasoning",
				Content: []dto.ResponsesOutputContent{
					{Text: "hidden reasoning"},
				},
			},
		},
	}

	got, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl-test")
	require.NoError(t, err)
	require.NotNil(t, got.Choices[0].Message.ReasoningContent)
	require.Equal(t, "hidden reasoning", *got.Choices[0].Message.ReasoningContent)
}
