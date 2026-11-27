package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// harnessInternalContractBody mirrors the internal gateway error envelope. The
// fields are spelled out rather than decoded into the implementation struct so a
// dropped or renamed JSON field fails this test.
type harnessInternalContractBody struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Retryable *bool  `json:"retryable"`
		RequestId string `json:"request_id"`
	} `json:"error"`
}

func decodeHarnessInternalContractBody(t *testing.T, recorder *httptest.ResponseRecorder) harnessInternalContractBody {
	t.Helper()
	var body harnessInternalContractBody
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

// assertHarnessInternalRequestId checks that the contract body carries a request
// id and that the same value is exposed through the gateway's correlation header.
func assertHarnessInternalRequestId(t *testing.T, recorder *httptest.ResponseRecorder, body harnessInternalContractBody) {
	t.Helper()
	require.NotEmpty(t, body.Error.RequestId, "internal contract errors must carry request_id")
	assert.Equal(t, body.Error.RequestId, recorder.Header().Get(middleware.NewAPIRequestIdHeader))
}

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
	// main.go mounts RequestId() globally; the test engine mirrors that so the
	// contract body and the correlation header are proven to agree end to end.
	engine.Use(middleware.RequestId())
	SetHarnessInternalRouter(engine)

	// No Harness JWT is configured or supplied, so every read endpoint must be
	// rejected by the auth layer rather than answering data. The rejection is the
	// contract envelope: the platform branches on code/retryable/request_id.
	for _, path := range []string{
		"/internal/v1/gateway/usage-events",
		"/internal/v1/gateway/cost-records",
		"/internal/v1/gateway/health",
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusUnauthorized, recorder.Code, "%s must not answer without credentials", path)

		body := decodeHarnessInternalContractBody(t, recorder)
		assert.Equal(t, "GATEWAY_UNAUTHORIZED", body.Error.Code, path)
		assert.NotEmpty(t, body.Error.Message, path)
		require.NotNil(t, body.Error.Retryable, path)
		assert.False(t, *body.Error.Retryable, "%s: a refused credential is not retryable", path)
		assertHarnessInternalRequestId(t, recorder, body)
	}

	// The batchGet routes run the verb guard before auth. A correct verb with no
	// credentials must still answer the same 401 contract body.
	for _, path := range []string{
		"/internal/v1/gateway/usage-events:batchGet",
		"/internal/v1/gateway/cost-records:batchGet",
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"request_ids":["a"]}`)))
		require.Equal(t, http.StatusUnauthorized, recorder.Code, "%s must not answer without credentials", path)

		body := decodeHarnessInternalContractBody(t, recorder)
		assert.Equal(t, "GATEWAY_UNAUTHORIZED", body.Error.Code, path)
		require.NotNil(t, body.Error.Retryable, path)
		assert.False(t, *body.Error.Retryable, path)
		assertHarnessInternalRequestId(t, recorder, body)
	}

	// batchGet must reach the guard, which answers 404 for a mistyped verb.
	for _, path := range []string{
		"/internal/v1/gateway/usage-eventsXYZ",
		"/internal/v1/gateway/usage-events:batchGetX",
		"/internal/v1/gateway/cost-recordsXYZ",
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"request_ids":["a"]}`)))
		require.Equal(t, http.StatusNotFound, recorder.Code, "%s must answer 404", path)

		body := decodeHarnessInternalContractBody(t, recorder)
		assert.Equal(t, "REQUEST_NOT_FOUND", body.Error.Code, path)
		assert.NotEmpty(t, body.Error.Message, path)
		require.NotNil(t, body.Error.Retryable, path)
		assert.False(t, *body.Error.Retryable, "%s: an unknown operation is not retryable", path)
		assertHarnessInternalRequestId(t, recorder, body)
	}
}
