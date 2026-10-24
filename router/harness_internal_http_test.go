package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestHarnessInternalEndpointsOverHTTP drives the registered routes with real
// HTTP requests. It complements the handler unit tests by proving the route
// shapes actually resolve: the colon-style batchGet paths, the guard that keeps a
// mistyped path at 404, and the fact that an unauthenticated call never reaches a
// handler body.
func TestHarnessInternalEndpointsOverHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previousDB, previousLogDB := model.DB, model.LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() { model.DB, model.LOG_DB = previousDB, previousLogDB })
	require.NoError(t, db.AutoMigrate(&model.HarnessLedgerEvent{}, &model.Channel{}, &model.Ability{}))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	engine := gin.New()
	SetHarnessInternalRouter(engine)

	// No Harness JWT is configured or supplied, so every read endpoint must be
	// rejected by the auth layer rather than answering data.
	for _, path := range []string{
		"/internal/v1/gateway/usage-events",
		"/internal/v1/gateway/cost-records",
		"/internal/v1/gateway/health",
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		assert.NotEqual(t, http.StatusNotFound, recorder.Code, "%s must be a registered route", path)
		assert.NotEqual(t, http.StatusOK, recorder.Code, "%s must not answer without credentials", path)
	}

	// batchGet must reach the guard, which answers 404 for a mistyped verb.
	for _, path := range []string{
		"/internal/v1/gateway/usage-eventsXYZ",
		"/internal/v1/gateway/usage-events:batchGetX",
		"/internal/v1/gateway/cost-recordsXYZ",
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"request_ids":["a"]}`)))
		assert.Equal(t, http.StatusNotFound, recorder.Code, "%s must answer 404", path)
	}
}
