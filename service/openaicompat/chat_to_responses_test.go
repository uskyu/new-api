package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionsRequestToResponsesRequestPreservesPenalties(t *testing.T) {
	tests := []struct {
		name      string
		frequency *float64
		presence  *float64
	}{
		{name: "positive", frequency: float64Ptr(1.25), presence: float64Ptr(0.75)},
		{name: "negative", frequency: float64Ptr(-1.5), presence: float64Ptr(-0.5)},
		{name: "explicit zero", frequency: float64Ptr(0), presence: float64Ptr(0)},
		{name: "nil", frequency: nil, presence: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{
				Model:            "gpt-test",
				FrequencyPenalty: tt.frequency,
				PresencePenalty:  tt.presence,
			})
			require.NoError(t, err)

			if tt.frequency == nil {
				require.Nil(t, got.FrequencyPenalty)
			} else {
				require.NotNil(t, got.FrequencyPenalty)
				require.Equal(t, *tt.frequency, *got.FrequencyPenalty)
			}
			if tt.presence == nil {
				require.Nil(t, got.PresencePenalty)
			} else {
				require.NotNil(t, got.PresencePenalty)
				require.Equal(t, *tt.presence, *got.PresencePenalty)
			}
		})
	}
}

func float64Ptr(value float64) *float64 {
	return &value
}
