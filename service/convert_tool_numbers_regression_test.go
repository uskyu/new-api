package service

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponseOpenAI2ClaudePreservesToolArgumentNumbers(t *testing.T) {
	arguments := `{"id":9007199254740993,"ratio":1.5,"nested":{"big":12345678901234567890}}`
	toolCalls, err := common.Marshal([]dto.ToolCallRequest{
		{
			ID:   "call_big_numbers",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "big_numbers",
				Arguments: arguments,
			},
		},
	})
	require.NoError(t, err)

	openAIResponse := &dto.OpenAITextResponse{
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message:      dto.Message{ToolCalls: toolCalls},
				FinishReason: "tool_calls",
			},
		},
	}

	claudeResponse := ResponseOpenAI2Claude(openAIResponse, nil)
	require.NotNil(t, claudeResponse)
	require.Len(t, claudeResponse.Content, 1)
	require.Equal(t, "tool_use", claudeResponse.Content[0].Type)
	require.Equal(t, "big_numbers", claudeResponse.Content[0].Name)

	encoded, err := common.Marshal(claudeResponse.Content[0].Input)
	require.NoError(t, err)

	var parsed map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(encoded, &parsed))
	require.Equal(t, "9007199254740993", string(parsed["id"]))
	require.Equal(t, "1.5", string(parsed["ratio"]))
	require.NotContains(t, string(encoded), "e+15")

	var nested map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(parsed["nested"], &nested))
	require.Equal(t, "12345678901234567890", string(nested["big"]))
}

func TestResponseOpenAI2ClaudeFallsBackToRawArgumentsOnInvalidJSON(t *testing.T) {
	arguments := `{"id":`
	toolCalls, err := common.Marshal([]dto.ToolCallRequest{
		{
			ID:   "call_invalid",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "invalid",
				Arguments: arguments,
			},
		},
	})
	require.NoError(t, err)

	openAIResponse := &dto.OpenAITextResponse{
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message:      dto.Message{ToolCalls: toolCalls},
				FinishReason: "tool_calls",
			},
		},
	}

	claudeResponse := ResponseOpenAI2Claude(openAIResponse, nil)
	require.NotNil(t, claudeResponse)
	require.Len(t, claudeResponse.Content, 1)
	require.Equal(t, arguments, claudeResponse.Content[0].Input)
}
