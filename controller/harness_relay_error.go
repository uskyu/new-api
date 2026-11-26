package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

const (
	// harnessAttemptsContextKey holds the per-request trail of upstream channel
	// attempts. It is set only for verified Harness callers.
	harnessAttemptsContextKey = "harness_upstream_attempts"

	// harnessAttemptMessageLimit bounds the per-attempt text echoed to the
	// platform. Diagnostics need a cause, not a full upstream body; keeping the
	// excerpt short also keeps the response small when many candidates failed.
	harnessAttemptMessageLimit = 320

	harnessCodeUpstreamUnavailable     = "upstream_unavailable"
	harnessCodeUpstreamAuthError       = "upstream_auth_error"
	harnessCodeUpstreamRateLimited     = "upstream_rate_limited"
	harnessCodeUpstreamInvalidResponse = "upstream_invalid_response"
)

// harnessUpstreamAttempt is one failed candidate in a Harness relay call.
// It never carries the channel key: only the id, the HTTP status the upstream
// answered with, the upstream/relay error code, and a masked short message.
type harnessUpstreamAttempt struct {
	ChannelId int    `json:"channel_id"`
	Status    int    `json:"status"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
}

// harnessModelName returns the model the caller asked for. The relay writes it
// into "original_model" when a channel is prepared, which is exactly the state a
// failed attempt leaves behind.
func harnessModelName(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if name := c.GetString("original_model"); name != "" {
		return name
	}
	return c.GetString("resolved_task_model")
}

// harnessRetryLimit returns the retry-loop bound for this request. Ordinary
// callers keep the configured RetryTimes. A Harness caller routed through a
// resource pool gets at least one attempt per enabled candidate channel for the
// model, capped by service.HarnessMaxRetryLimit. Selection itself still comes
// from CacheGetRandomSatisfiedChannel and its configured order.
func harnessRetryLimit(c *gin.Context, modelName string) int {
	configuredRetries := common.RetryTimes
	if _, harness := middleware.GetHarnessPrincipal(c); !harness {
		return configuredRetries
	}
	poolID := common.GetContextKeyString(c, setting.IntelligentRoutingContextKey)
	if poolID == "" {
		return configuredRetries
	}
	candidateCount := model.CountResourcePoolModelChannels(poolID, modelName)
	return service.HarnessRetryBudget(configuredRetries, candidateCount)
}

// recordHarnessUpstreamAttempt appends one failed candidate for a Harness
// caller. Non-Harness requests never allocate the trail.
func recordHarnessUpstreamAttempt(c *gin.Context, channelID int, status int, code string, message string) {
	if c == nil {
		return
	}
	if _, harness := middleware.GetHarnessPrincipal(c); !harness {
		return
	}
	attempt := harnessUpstreamAttempt{
		ChannelId: channelID,
		Status:    status,
		Code:      strings.TrimSpace(code),
		Message:   harnessAttemptMessage(c, message),
	}
	existing, _ := c.Get(harnessAttemptsContextKey)
	attempts, _ := existing.([]harnessUpstreamAttempt)
	c.Set(harnessAttemptsContextKey, append(attempts, attempt))
}

func harnessUpstreamAttempts(c *gin.Context) []harnessUpstreamAttempt {
	if c == nil {
		return nil
	}
	value, exists := c.Get(harnessAttemptsContextKey)
	if !exists {
		return nil
	}
	attempts, _ := value.([]harnessUpstreamAttempt)
	return attempts
}

// harnessAttemptMessage masks upstream URLs and credentials, collapses
// whitespace, and truncates the text so an upstream HTML page cannot bloat the
// platform response.
func harnessAttemptMessage(c *gin.Context, message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	message = common.MaskSensitiveInfo(message, common.UpstreamAddresses(c)...)
	message = strings.Join(strings.Fields(message), " ")
	runes := []rune(message)
	if len(runes) > harnessAttemptMessageLimit {
		return string(runes[:harnessAttemptMessageLimit]) + "..."
	}
	return message
}

// harnessUpstreamErrorCode maps the last failed candidate to the contract code.
// Status wins over the upstream body code because the same provider error name
// can appear with different meanings; an explicit 401/403/429 is unambiguous.
func harnessUpstreamErrorCode(attempts []harnessUpstreamAttempt) (string, bool) {
	if len(attempts) == 0 {
		return harnessCodeUpstreamUnavailable, true
	}
	last := attempts[len(attempts)-1]
	switch last.Status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return harnessCodeUpstreamAuthError, false
	case http.StatusTooManyRequests:
		return harnessCodeUpstreamRateLimited, true
	}
	switch types.ErrorCode(last.Code) {
	case types.ErrorCodeBadResponseBody,
		types.ErrorCodeBadResponse,
		types.ErrorCodeEmptyResponse,
		types.ErrorCodeReadResponseBodyFailed:
		return harnessCodeUpstreamInvalidResponse, true
	}
	switch last.Code {
	case "plugin_submit_response_invalid", "read_response_body_failed", "copy_response_body_failed", "parse_response_failed":
		return harnessCodeUpstreamInvalidResponse, true
	}
	return harnessCodeUpstreamUnavailable, true
}

// buildHarnessUpstreamErrorBody renders the all-candidates-failed answer. The
// final error's own code can be a local "no more candidates" marker, so the
// last real attempt supplies the message in that case and always supplies the
// classification.
func buildHarnessUpstreamErrorBody(c *gin.Context, requestID string, modelName string, routeGroup string, attempts []harnessUpstreamAttempt, finalCode string, finalMessage string) middleware.HarnessErrorBody {
	code, retryable := harnessUpstreamErrorCode(attempts)
	if finalMessage == "" || finalCode == string(types.ErrorCodeGetChannelFailed) || finalCode == "get_channel_failed" {
		if len(attempts) > 0 && attempts[len(attempts)-1].Message != "" {
			finalMessage = attempts[len(attempts)-1].Message
		}
	}
	finalMessage = harnessAttemptMessage(c, finalMessage)
	if finalMessage == "" {
		finalMessage = code
	}
	return middleware.HarnessErrorBody{
		Message:   finalMessage,
		Type:      "new_api_error",
		Code:      code,
		Retryable: retryable,
		RequestId: requestID,
		Details: map[string]any{
			"model":       modelName,
			"route_group": routeGroup,
			"attempts":    attempts,
			"request_id":  requestID,
		},
	}
}

// writeHarnessRelayError renders the structured body on an HTTP relay response.
// All upstream outcomes are a gateway-side 503: the caller's request is valid,
// and the code plus retryable flag says whether retrying can help.
func writeHarnessRelayError(c *gin.Context, relayFormat types.RelayFormat, body middleware.HarnessErrorBody) {
	status := http.StatusServiceUnavailable
	if relayFormat == types.RelayFormatClaude {
		// Claude clients keep their top-level type marker; the error object gains
		// the structured Harness fields.
		c.JSON(status, gin.H{"type": "error", "error": body})
		return
	}
	c.JSON(status, gin.H{"error": body})
}

// writeHarnessRelayStreamError renders the structured body inside an already
// committed event stream, mirroring writeRelayStreamError.
func writeHarnessRelayStreamError(c *gin.Context, format types.RelayFormat, body middleware.HarnessErrorBody, apiError *types.NewAPIError) {
	status, _ := common.GetContextKeyType[*relaycommon.StreamStatus](c, constant.ContextKeyResponseStreamStatus)
	if status == nil {
		status = relaycommon.NewStreamStatus()
		common.SetContextKey(c, constant.ContextKeyResponseStreamStatus, status)
	}
	if apiError != nil {
		status.MarkFailed(string(apiError.GetErrorCode()), string(apiError.GetErrorType()), apiError.StatusCode)
	}
	if format == types.RelayFormatClaude {
		_ = helper.ClaudeData(c, dto.ClaudeResponse{Type: "error", Error: body})
		return
	}
	_ = helper.ObjectData(c, gin.H{"error": body})
	helper.Done(c)
}

// writeHarnessTaskSubmissionError renders the structured body for an async task
// submission (for example /v1/videos), whose errors are presented by
// respondTaskSubmissionError rather than Relay's defer.
func writeHarnessTaskSubmissionError(c *gin.Context, taskErr *taskdto.TaskError, attempts []harnessUpstreamAttempt) {
	if taskErr == nil {
		return
	}
	requestID := middleware.HarnessRequestID(c)
	routeGroup := middleware.HarnessRouteGroupID(c, common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
	body := buildHarnessUpstreamErrorBody(
		c, requestID, harnessModelName(c), routeGroup, attempts,
		taskErr.Code, taskErr.Message,
	)
	c.JSON(http.StatusServiceUnavailable, middleware.HarnessErrorEnvelope{Error: body})
}
