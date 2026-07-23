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

func TestSupportCannotEnableOrEditRedemptionCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name string
		url  string
		body string
	}{
		{name: "enable", url: "/api/redemption/?status_only=true", body: `{"id":1,"status":1}`},
		{name: "edit", url: "/api/redemption/", body: `{"id":1,"name":"changed"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodPut, test.url, strings.NewReader(test.body))
			context.Request.Header.Set("Content-Type", "application/json")
			context.Set("role", common.RoleSupportUser)

			UpdateRedemption(context)

			assert.Contains(t, recorder.Body.String(), `"success":false`)
			assert.Contains(t, recorder.Body.String(), "permission denied")
		})
	}
}
