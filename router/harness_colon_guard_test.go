package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// The colon form makes gin treat ":batchGet" as a wildcard, so a mistyped
// sibling path would otherwise answer 200 from the batchGet handler. This pins
// the guard that keeps unknown paths at 404.
func TestHarnessColonBatchGetGuardRejectsMistypedPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetHarnessInternalRouter(engine)

	for _, path := range []string{
		"/internal/v1/gateway/usage-eventsXYZ",
		"/internal/v1/gateway/usage-events:batchGetX",
		"/internal/v1/gateway/cost-recordsXYZ",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, nil)
		engine.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusNotFound, recorder.Code, "path %s must not reach a batchGet handler", path)
	}
}
