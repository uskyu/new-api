package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGeneralOpenAIRequestPreserveExplicitZeroValues(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-4.1",
		"stream":false,
		"max_tokens":0,
		"max_completion_tokens":0,
		"top_p":0,
		"top_k":0,
		"n":0,
		"frequency_penalty":0,
		"presence_penalty":0,
		"seed":0,
		"logprobs":false,
		"top_logprobs":0,
		"dimensions":0,
		"return_images":false,
		"return_related_questions":false
	}`)

	var req GeneralOpenAIRequest
	err := common.Unmarshal(raw, &req)
	require.NoError(t, err)

	encoded, err := common.Marshal(req)
	require.NoError(t, err)

	require.True(t, gjson.GetBytes(encoded, "stream").Exists())
	require.True(t, gjson.GetBytes(encoded, "max_tokens").Exists())
	require.True(t, gjson.GetBytes(encoded, "max_completion_tokens").Exists())
	require.True(t, gjson.GetBytes(encoded, "top_p").Exists())
	require.True(t, gjson.GetBytes(encoded, "top_k").Exists())
	require.True(t, gjson.GetBytes(encoded, "n").Exists())
	require.True(t, gjson.GetBytes(encoded, "frequency_penalty").Exists())
	require.True(t, gjson.GetBytes(encoded, "presence_penalty").Exists())
	require.True(t, gjson.GetBytes(encoded, "seed").Exists())
	require.True(t, gjson.GetBytes(encoded, "logprobs").Exists())
	require.True(t, gjson.GetBytes(encoded, "top_logprobs").Exists())
	require.True(t, gjson.GetBytes(encoded, "dimensions").Exists())
	require.True(t, gjson.GetBytes(encoded, "return_images").Exists())
	require.True(t, gjson.GetBytes(encoded, "return_related_questions").Exists())
}

func TestOpenAIResponsesRequestPreserveExplicitZeroValues(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-4.1",
		"max_output_tokens":0,
		"max_tool_calls":0,
		"stream":false,
		"top_p":0,
		"frequency_penalty":0,
		"presence_penalty":0,
		"service_tier":""
	}`)

	var req OpenAIResponsesRequest
	err := common.Unmarshal(raw, &req)
	require.NoError(t, err)

	encoded, err := common.Marshal(req)
	require.NoError(t, err)

	require.True(t, gjson.GetBytes(encoded, "max_output_tokens").Exists())
	require.True(t, gjson.GetBytes(encoded, "max_tool_calls").Exists())
	require.True(t, gjson.GetBytes(encoded, "stream").Exists())
	require.True(t, gjson.GetBytes(encoded, "top_p").Exists())
	require.True(t, gjson.GetBytes(encoded, "frequency_penalty").Exists())
	require.True(t, gjson.GetBytes(encoded, "presence_penalty").Exists())
	require.True(t, gjson.GetBytes(encoded, "service_tier").Exists())
	require.Empty(t, gjson.GetBytes(encoded, "service_tier").String())
}

func TestCodexResponsesFieldsRoundTrip(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-5.1-codex",
		"client_metadata":{"session_id":"abc"},
		"reasoning":{"effort":"high","mode":"auto","context":{"turn":2}}
	}`)

	var req OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	encoded, err := common.Marshal(req)
	require.NoError(t, err)
	require.Equal(t, "abc", gjson.GetBytes(encoded, "client_metadata.session_id").String())
	require.Equal(t, "auto", gjson.GetBytes(encoded, "reasoning.mode").String())
	require.Equal(t, int64(2), gjson.GetBytes(encoded, "reasoning.context.turn").Int())
}

func TestOpenAIResponsesCompactionRequestFieldsRoundTrip(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-5.1-codex",
		"tools":[],
		"parallel_tool_calls":false,
		"reasoning":{"mode":"auto"},
		"service_tier":"priority",
		"prompt_cache_key":"cache-key",
		"text":{"format":{"type":"text"}}
	}`)

	var req OpenAIResponsesCompactionRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	encoded, err := common.Marshal(req)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(encoded, "parallel_tool_calls").Exists())
	require.False(t, gjson.GetBytes(encoded, "parallel_tool_calls").Bool())
	require.Equal(t, "priority", gjson.GetBytes(encoded, "service_tier").String())
	require.Equal(t, "cache-key", gjson.GetBytes(encoded, "prompt_cache_key").String())
	require.Equal(t, "auto", gjson.GetBytes(encoded, "reasoning.mode").String())
	require.Equal(t, "text", gjson.GetBytes(encoded, "text.format.type").String())
}

func TestOpenAIResponsesCompactionRequestPreservesExplicitEmptyServiceTier(t *testing.T) {
	var req OpenAIResponsesCompactionRequest
	require.NoError(t, common.Unmarshal([]byte(`{"model":"gpt-5.1-codex","service_tier":""}`), &req))
	require.NotNil(t, req.ServiceTier)

	encoded, err := common.Marshal(req)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(encoded, "service_tier").Exists())
	require.Empty(t, gjson.GetBytes(encoded, "service_tier").String())

	var absent OpenAIResponsesCompactionRequest
	require.NoError(t, common.Unmarshal([]byte(`{"model":"gpt-5.1-codex"}`), &absent))
	require.Nil(t, absent.ServiceTier)
}

func TestGeneralOpenAIRequestGetSystemRoleName(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  string
	}{
		{name: "o1 uses developer", model: "o1", want: "developer"},
		{name: "o3 family uses developer", model: "o3-mini-high", want: "developer"},
		{name: "o4 family uses developer", model: "o4-mini", want: "developer"},
		{name: "o1 mini stays system", model: "o1-mini", want: "system"},
		{name: "o1 preview stays system", model: "o1-preview", want: "system"},
		{name: "gpt 5 uses developer", model: "gpt-5", want: "developer"},
		{name: "omni is not o series", model: "omni-moderation-latest", want: "system"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := GeneralOpenAIRequest{Model: tt.model}

			require.Equal(t, tt.want, req.GetSystemRoleName())
		})
	}
}
