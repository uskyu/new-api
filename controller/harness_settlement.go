package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// harnessSettlementResponse is the billing state of one relayed request as
// Harness reconciles it. Amounts are the decimal strings stored in the
// database passed through untouched: re-parsing them as float64 would silently
// lose precision on a ledger value.
type harnessSettlementResponse struct {
	RequestId    string                   `json:"request_id"`
	Status       string                   `json:"status"`
	TaskId       string                   `json:"task_id,omitempty"`
	Model        string                   `json:"model"`
	ChannelId    int                      `json:"channel_id"`
	PricingGroup string                   `json:"pricing_group,omitempty"`
	Usage        harnessSettlementUsage   `json:"usage"`
	Billing      harnessSettlementBilling `json:"billing"`
	CreatedAt    int64                    `json:"created_at"`
	SettledAt    int64                    `json:"settled_at,omitempty"`
}

type harnessSettlementUsage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
}

type harnessSettlementBilling struct {
	BaseAmount     string  `json:"base_amount"`
	GroupRatio     float64 `json:"group_ratio"`
	TaxRatePercent float64 `json:"tax_rate_percent"`
	TotalAmount    string  `json:"total_amount"`
	Currency       string  `json:"currency"`
}

// GetHarnessSettlement returns the settlement of one relayed request owned by
// the calling service token. A settlement owned by another token answers with
// the same 404 as one that does not exist, so the endpoint cannot be used to
// probe request ids across tokens.
//
// A "pending" status is a successful response, not an error: Harness polls it
// with backoff until the request reaches a final billing state.
func GetHarnessSettlement(c *gin.Context) {
	token, err := model.GetTokenById(c.GetInt("token_id"))
	if err != nil {
		common.ApiErrorStatus(c, http.StatusUnauthorized, common.NewMessage("Invalid service token"))
		return
	}
	if token.ModelLimitsEnabled {
		common.ApiErrorStatus(c, http.StatusForbidden, common.NewMessage("Harness settlement query requires a full-model service token"))
		return
	}
	requestId := strings.TrimSpace(c.Param("request_id"))
	if requestId == "" {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Request ID is required"))
		return
	}
	settlement, err := model.GetHarnessSettlementForToken(requestId, token.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if settlement == nil {
		common.ApiErrorStatus(c, http.StatusNotFound, common.NewMessage("Settlement not found"))
		return
	}
	common.ApiSuccess(c, harnessSettlementResponse{
		RequestId:    settlement.RequestId,
		Status:       settlement.Status,
		TaskId:       settlement.TaskId,
		Model:        settlement.ModelName,
		ChannelId:    settlement.ChannelId,
		PricingGroup: settlement.PricingGroup,
		Usage: harnessSettlementUsage{
			InputTokens:      settlement.PromptTokens,
			OutputTokens:     settlement.CompletionTokens,
			CacheReadTokens:  settlement.CacheReadTokens,
			CacheWriteTokens: settlement.CacheWriteTokens,
		},
		Billing: harnessSettlementBilling{
			BaseAmount:     settlement.BaseAmount,
			GroupRatio:     settlement.GroupRatio,
			TaxRatePercent: settlement.TaxRatePercent,
			TotalAmount:    settlement.TotalAmount,
			Currency:       settlement.Currency,
		},
		CreatedAt: settlement.CreatedAt,
		SettledAt: settlement.SettledAt,
	})
}
