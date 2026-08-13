package codex

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestStripsUnsupportedPenaltiesForNonCompact(t *testing.T) {
	frequencyPenalty := 1.25
	presencePenalty := -0.75
	temperature := 0.5
	maxOutputTokens := uint(128)
	request := dto.OpenAIResponsesRequest{
		Model:            "gpt-5.4",
		FrequencyPenalty: &frequencyPenalty,
		PresencePenalty:  &presencePenalty,
		Temperature:      &temperature,
		MaxOutputTokens:  &maxOutputTokens,
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}, request)
	require.NoError(t, err)

	got, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.Nil(t, got.FrequencyPenalty)
	require.Nil(t, got.PresencePenalty)
	require.Nil(t, got.Temperature)
	require.Nil(t, got.MaxOutputTokens)
}

func TestConvertOpenAIResponsesRequestKeepsPenaltiesForCompact(t *testing.T) {
	frequencyPenalty := 0.0
	presencePenalty := -0.75
	temperature := 0.5
	maxOutputTokens := uint(128)
	request := dto.OpenAIResponsesRequest{
		Model:            "gpt-5.4",
		FrequencyPenalty: &frequencyPenalty,
		PresencePenalty:  &presencePenalty,
		Temperature:      &temperature,
		MaxOutputTokens:  &maxOutputTokens,
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponsesCompact,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}, request)
	require.NoError(t, err)

	got, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.NotNil(t, got.FrequencyPenalty)
	require.Equal(t, 0.0, *got.FrequencyPenalty)
	require.NotNil(t, got.PresencePenalty)
	require.Equal(t, -0.75, *got.PresencePenalty)
	require.Equal(t, &temperature, got.Temperature)
	require.Equal(t, &maxOutputTokens, got.MaxOutputTokens)
}
