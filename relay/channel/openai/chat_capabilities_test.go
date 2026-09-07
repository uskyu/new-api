package openai

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOpenAIChatModelCapabilities(t *testing.T) {
	for _, tc := range []struct {
		model, effort                   string
		sampling, completion, developer bool
	}{
		{"gpt-5.1", "", true, true, true}, {"gpt-5.2", "none", true, true, true},
		{"gpt-5.4-2026-03-05", "", true, true, true}, {"gpt-5.4-high", "none", false, true, true},
		{"gpt-5.4", "high", false, true, true}, {"gpt-5.4-pro", "none", false, true, true},
		{"gpt-5.2-codex", "", false, true, true}, {"gpt-5.2-chat-latest", "", false, true, true},
		{"gpt-5", "", false, true, true}, {"gpt-6-astra", "none", false, true, true},
		{"gpt-6-astra-2026-09-01", "", false, true, true}, {"gpt-6-astra-high", "", false, true, true},
		{"gpt-50", "", true, false, false}, {"gpt-5custom", "", true, false, false},
		{"gpt-7", "", true, false, false}, {"qwen-max", "", true, false, false},
		{"gpt-6-astra-custom", "", true, false, false},
	} {
		t.Run(tc.model+tc.effort, func(t *testing.T) {
			req := &dto.GeneralOpenAIRequest{Model: tc.model, ReasoningEffort: tc.effort, MaxTokens: common.GetPointer(uint(100)), Temperature: common.GetPointer(0.0), TopP: common.GetPointer(0.0), LogProbs: common.GetPointer(false), TopLogProbs: common.GetPointer(0), Messages: []dto.Message{{Role: "system", Content: "first"}, {Role: "system", Content: "second"}}}
			info := &relaycommon.RelayInfo{OriginModelName: "public-alias", ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI, UpstreamModelName: tc.model}}
			_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, req)
			require.NoError(t, err)
			data, err := common.Marshal(req)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, common.Unmarshal(data, &payload))
			for _, key := range []string{"temperature", "top_p", "logprobs", "top_logprobs"} {
				_, ok := payload[key]
				require.Equal(t, tc.sampling, ok, key)
			}
			require.Equal(t, tc.completion, req.MaxCompletionTokens != nil)
			require.Equal(t, tc.developer, req.Messages[0].Role == "developer")
			require.Equal(t, "system", req.Messages[1].Role)
		})
	}
}

func TestOpenAIChatTokenPriority(t *testing.T) {
	for _, tc := range []struct {
		max, completion uint
		keepMax         bool
		want            uint
	}{{100, 200, true, 200}, {100, 0, false, 100}, {0, 0, true, 0}} {
		req := &dto.GeneralOpenAIRequest{Model: "gpt-6-astra", MaxTokens: common.GetPointer(tc.max), MaxCompletionTokens: common.GetPointer(tc.completion)}
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeAzure, UpstreamModelName: req.Model}}
		_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, req)
		require.NoError(t, err)
		require.Equal(t, tc.keepMax, req.MaxTokens != nil)
		require.Equal(t, tc.want, *req.MaxCompletionTokens)
	}
}
