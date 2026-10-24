package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// The /internal/v1/gateway read APIs answer the platform's reconciliation
// callers. Their envelopes are deliberately NOT common.ApiSuccess: the contract
// fixes the literal shapes ({items,next_cursor,has_more} and {error:{...}}), so
// these handlers bypass the dashboard wrapper.

// harnessReadErrorEnvelope is the contract error body.
type harnessReadErrorEnvelope struct {
	Error harnessReadErrorBody `json:"error"`
}

type harnessReadErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	RequestId string         `json:"request_id,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

// writeHarnessReadError renders one contract error. The request id comes from
// the middleware so a caller can correlate the failure with our logs.
func writeHarnessReadError(c *gin.Context, failure *service.HarnessReadError) {
	body := harnessReadErrorEnvelope{Error: harnessReadErrorBody{
		Code:      failure.Code,
		Message:   failure.Message,
		Retryable: failure.Retryable,
		RequestId: c.GetString(common.RequestIdKey),
		Details:   failure.Details,
	}}
	status := failure.StatusCode
	if status == 0 {
		status = http.StatusBadRequest
	}
	c.JSON(status, body)
}

// writeHarnessInternalError maps an unexpected failure to GATEWAY_UPSTREAM with
// a retryable flag, never leaking database details to the caller.
func writeHarnessInternalError(c *gin.Context) {
	writeHarnessReadError(c, &service.HarnessReadError{
		StatusCode: http.StatusInternalServerError,
		Code:       service.HarnessErrCodeUpstream,
		Message:    "the gateway could not read its ledger",
		Retryable:  true,
	})
}

// requireHarnessScope rejects a caller that is not a verified Harness principal
// or lacks the scope. Non-Harness callers never reach these routes because the
// router requires the JWT middleware, but the check keeps the guarantee local.
func requireHarnessScope(c *gin.Context, scope string) bool {
	principal, ok := middleware.GetHarnessPrincipal(c)
	if !ok {
		writeHarnessReadError(c, &service.HarnessReadError{
			StatusCode: http.StatusUnauthorized,
			Code:       service.HarnessErrCodeUnauthorized,
			Message:    "a verified Harness credential is required",
		})
		return false
	}
	// An empty scope list is treated as "no scopes granted" rather than "all
	// scopes", so a misconfigured token fails closed.
	for _, granted := range principal.Scopes {
		if granted == scope {
			return true
		}
	}
	writeHarnessReadError(c, &service.HarnessReadError{
		StatusCode: http.StatusForbidden,
		Code:       service.HarnessErrCodeUnauthorized,
		Message:    "the credential lacks the required scope",
		Details:    map[string]any{"required_scope": scope},
	})
	return false
}

// harnessUsageEventsResponse is the B1 list body.
type harnessUsageEventsResponse struct {
	Items      []service.HarnessUsageEvent `json:"items"`
	NextCursor *string                     `json:"next_cursor"`
	HasMore    bool                        `json:"has_more"`
}

// GetHarnessUsageEvents serves B1: the incremental usage-event feed.
func GetHarnessUsageEvents(c *gin.Context) {
	if !requireHarnessScope(c, "gateway.usage.read") {
		return
	}
	afterId, failure := service.ParseHarnessLedgerCursor(c.Query("cursor"), model.HarnessLedgerStreamUsage)
	if failure != nil {
		writeHarnessReadError(c, failure)
		return
	}
	limit, failure := service.ParseHarnessLedgerLimit(c.Query("limit"))
	if failure != nil {
		writeHarnessReadError(c, failure)
		return
	}
	fromMs, toMs, failure := service.ParseHarnessLedgerWindow(c.Query("from"), c.Query("to"))
	if failure != nil {
		writeHarnessReadError(c, failure)
		return
	}
	items, nextCursor, hasMore, err := service.ListHarnessUsageEvents(afterId, limit, fromMs, toMs)
	if err != nil {
		writeHarnessInternalError(c)
		return
	}
	response := harnessUsageEventsResponse{Items: items, HasMore: hasMore}
	if nextCursor != "" {
		response.NextCursor = &nextCursor
	}
	c.JSON(http.StatusOK, response)
}

// harnessCostRecordsResponse is the B3 list body.
type harnessCostRecordsResponse struct {
	Items      []service.HarnessCostRecord `json:"items"`
	NextCursor *string                     `json:"next_cursor"`
	HasMore    bool                        `json:"has_more"`
}

// GetHarnessCostRecords serves B3: the incremental cost-event feed.
func GetHarnessCostRecords(c *gin.Context) {
	if !requireHarnessScope(c, "gateway.cost.read") {
		return
	}
	afterId, failure := service.ParseHarnessLedgerCursor(c.Query("cursor"), model.HarnessLedgerStreamCost)
	if failure != nil {
		writeHarnessReadError(c, failure)
		return
	}
	limit, failure := service.ParseHarnessLedgerLimit(c.Query("limit"))
	if failure != nil {
		writeHarnessReadError(c, failure)
		return
	}
	fromMs, toMs, failure := service.ParseHarnessLedgerWindow(c.Query("from"), c.Query("to"))
	if failure != nil {
		writeHarnessReadError(c, failure)
		return
	}
	items, nextCursor, hasMore, err := service.ListHarnessCostRecords(afterId, limit, fromMs, toMs)
	if err != nil {
		writeHarnessInternalError(c)
		return
	}
	response := harnessCostRecordsResponse{Items: items, HasMore: hasMore}
	if nextCursor != "" {
		response.NextCursor = &nextCursor
	}
	c.JSON(http.StatusOK, response)
}

// harnessUsageBatchGetRequest is the B1-batchGet body.
type harnessUsageBatchGetRequest struct {
	RequestIds []string `json:"request_ids"`
}

// harnessUsageBatchGetResponse is the B1-batchGet answer.
type harnessUsageBatchGetResponse struct {
	Items             []service.HarnessUsageBatchItem `json:"items"`
	MissingRequestIds []string                        `json:"missing_request_ids"`
	Truncated         bool                            `json:"truncated"`
}

// BatchGetHarnessUsageEvents serves B1-batchGet.
func BatchGetHarnessUsageEvents(c *gin.Context) {
	if !requireHarnessScope(c, "gateway.usage.read") {
		return
	}
	var request harnessUsageBatchGetRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		writeHarnessReadError(c, &service.HarnessReadError{
			StatusCode: http.StatusBadRequest,
			Code:       service.HarnessErrCodeRequestInvalid,
			Message:    "request body is not valid JSON",
		})
		return
	}
	ids, failure := service.ValidateHarnessBatchRequestIds(request.RequestIds)
	if failure != nil {
		writeHarnessReadError(c, failure)
		return
	}
	items, missing, truncated, err := service.BatchGetHarnessUsageEvents(ids)
	if err != nil {
		writeHarnessInternalError(c)
		return
	}
	if missing == nil {
		missing = []string{}
	}
	c.JSON(http.StatusOK, harnessUsageBatchGetResponse{Items: items, MissingRequestIds: missing, Truncated: truncated})
}

// harnessCostBatchGetRequest is the B3-batchGet body.
type harnessCostBatchGetRequest struct {
	RequestIds []string `json:"request_ids"`
}

// harnessCostBatchGetResponse is the B3-batchGet answer.
type harnessCostBatchGetResponse struct {
	Items             []service.HarnessCostBatchItem `json:"items"`
	MissingRequestIds []string                       `json:"missing_request_ids"`
	Truncated         bool                           `json:"truncated"`
}

// BatchGetHarnessCostRecords serves B3-batchGet.
func BatchGetHarnessCostRecords(c *gin.Context) {
	if !requireHarnessScope(c, "gateway.cost.read") {
		return
	}
	var request harnessCostBatchGetRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		writeHarnessReadError(c, &service.HarnessReadError{
			StatusCode: http.StatusBadRequest,
			Code:       service.HarnessErrCodeRequestInvalid,
			Message:    "request body is not valid JSON",
		})
		return
	}
	ids, failure := service.ValidateHarnessBatchRequestIds(request.RequestIds)
	if failure != nil {
		writeHarnessReadError(c, failure)
		return
	}
	items, missing, truncated, err := service.BatchGetHarnessCostRecords(ids)
	if err != nil {
		writeHarnessInternalError(c)
		return
	}
	if missing == nil {
		missing = []string{}
	}
	c.JSON(http.StatusOK, harnessCostBatchGetResponse{Items: items, MissingRequestIds: missing, Truncated: truncated})
}
