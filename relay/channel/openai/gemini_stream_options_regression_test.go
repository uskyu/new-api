package openai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newGeminiChatRequest() *dto.GeminiChatRequest {
	return &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role:  "user",
				Parts: []dto.GeminiPart{{Text: "hello"}},
			},
		},
	}
}

// TestConvertGeminiRequestRequestsStreamUsage verifies that Gemini requests
// converted to OpenAI chat completions ask for upstream usage on streaming
// requests, while non-streaming requests do not.
func TestConvertGeminiRequestRequestsStreamUsage(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	tests := []struct {
		name        string
		channelType int
		stream      bool
		wantUsage   bool
	}{
		{
			name:        "openai streaming requests include usage",
			channelType: constant.ChannelTypeOpenAI,
			stream:      true,
			wantUsage:   true,
		},
		{
			name:        "openai non-streaming omits stream options",
			channelType: constant.ChannelTypeOpenAI,
			stream:      false,
			wantUsage:   false,
		},
		{
			name:        "non-openai channel keeps clearing stream options",
			channelType: constant.ChannelTypeGemini,
			stream:      true,
			wantUsage:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

			info := &relaycommon.RelayInfo{
				IsStream: test.stream,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:          test.channelType,
					UpstreamModelName:    "gpt-4o-mini",
					SupportStreamOptions: true,
				},
			}

			converted, err := (&Adaptor{}).ConvertGeminiRequest(c, info, newGeminiChatRequest())
			require.NoError(t, err)
			openaiRequest, ok := converted.(*dto.GeneralOpenAIRequest)
			require.True(t, ok)

			payload, err := common.Marshal(openaiRequest)
			require.NoError(t, err)
			if test.wantUsage {
				require.NotNil(t, openaiRequest.StreamOptions)
				require.True(t, openaiRequest.StreamOptions.IncludeUsage)
				require.Contains(t, string(payload), `"include_usage":true`)
			} else {
				require.Nil(t, openaiRequest.StreamOptions)
				require.NotContains(t, string(payload), "include_usage")
			}
		})
	}
}
