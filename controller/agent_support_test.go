package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSupportAgentOperationsRejectSelfBenefit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		body    string
		handler gin.HandlerFunc
		message string
	}{
		{
			name:    "balance adjustment",
			body:    `{"agent_user_id":7,"amount":"1.00","reason":"self adjustment"}`,
			handler: AdjustAgentBalance,
			message: "cannot adjust their own agent balance",
		},
		{
			name:    "downline assignment",
			body:    `{"target_agent_user_id":7,"downline_user_id":8,"remark":"self assignment"}`,
			handler: AssignAgentDownlineUser,
			message: "cannot assign downlines to their own agent account",
		},
		{
			name:    "downline transfer",
			body:    `{"source_agent_user_id":9,"target_agent_user_id":7,"downline_user_id":8,"remark":"self transfer"}`,
			handler: TransferAgentDownlineUser,
			message: "cannot transfer downlines to or from their own agent account",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodPost, "/api/agent/test", strings.NewReader(test.body))
			context.Request.Header.Set("Content-Type", "application/json")
			context.Set("id", 7)
			context.Set("role", common.RoleSupportUser)

			test.handler(context)

			assert.Contains(t, recorder.Body.String(), `"success":false`)
			assert.Contains(t, recorder.Body.String(), test.message)
		})
	}
}
