package deepseek

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestMapsDeveloperRoleToSystem(t *testing.T) {
	adaptor := &Adaptor{}
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-chat",
		Messages: []dto.Message{
			{Role: "developer", Content: "follow the rules"},
			{Role: "user", Content: "hello"},
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "deepseek-chat"},
	}

	converted, err := adaptor.ConvertOpenAIRequest(nil, info, request)
	require.NoError(t, err)

	convertedRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Len(t, convertedRequest.Messages, 2)
	require.Equal(t, "system", convertedRequest.Messages[0].Role)
	require.Equal(t, "user", convertedRequest.Messages[1].Role)
}

// TestConvertOpenAIResponsesRequestKeepsDeveloperRole documents that the
// Responses API path is intentionally left unchanged; DeepSeek accepts the
// developer role there.
func TestConvertOpenAIResponsesRequestKeepsDeveloperRole(t *testing.T) {
	adaptor := &Adaptor{}
	input := `[{"role":"developer","content":"follow the rules"}]`
	request := dto.OpenAIResponsesRequest{Model: "deepseek-chat", Input: []byte(input)}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{}, request)

	require.Error(t, err)
	require.Nil(t, converted)
	require.JSONEq(t, input, string(request.Input))
}
