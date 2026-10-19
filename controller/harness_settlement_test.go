package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestGetHarnessSettlementScopesByTokenAndStatus pins the endpoint contract the
// Harness ledger depends on: a settlement belonging to another token is
// indistinguishable from a missing one, and an unsettled request is a 200 with
// status "pending" rather than an error, because Harness polls it with backoff.
func TestGetHarnessSettlementScopesByTokenAndStatus(t *testing.T) {
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open("file:harness-settlement-endpoint?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.HarnessSettlement{}))

	owner := model.Token{Name: "owner", Key: "owner-key", UserId: 1, Status: 1}
	other := model.Token{Name: "other", Key: "other-key", UserId: 1, Status: 1}
	require.NoError(t, db.Create(&owner).Error)
	require.NoError(t, db.Create(&other).Error)

	// Two rows: one settled for the owner, one still pending.
	require.NoError(t, model.UpsertHarnessSettlement(&model.HarnessSettlement{
		RequestId:        "req-settled",
		TokenId:          owner.Id,
		Status:           model.HarnessSettlementStatusSettled,
		ModelName:        "deepseek-v4.1-flash",
		ChannelId:        7,
		PricingGroup:     "default",
		GroupRatio:       1,
		BaseAmount:       "0.000613",
		TotalAmount:      "0.000613",
		Currency:         "CNY",
		PromptTokens:     37,
		CacheReadTokens:  128,
		CompletionTokens: 4,
	}))
	require.NoError(t, model.UpsertHarnessSettlement(&model.HarnessSettlement{
		RequestId: "req-pending",
		TokenId:   owner.Id,
		Status:    model.HarnessSettlementStatusPending,
		ModelName: "deepseek-v4.1-flash",
	}))

	gin.SetMode(gin.TestMode)
	query := func(tokenId int, requestId string) (*httptest.ResponseRecorder, map[string]any) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/harness/settlements/"+requestId, nil)
		c.Params = gin.Params{{Key: "request_id", Value: requestId}}
		c.Set("token_id", tokenId)
		GetHarnessSettlement(c)
		var body map[string]any
		_ = common.Unmarshal(recorder.Body.Bytes(), &body)
		return recorder, body
	}

	// Owner reads the settled row and gets the stored amounts verbatim.
	recorder, body := query(owner.Id, "req-settled")
	require.Equal(t, http.StatusOK, recorder.Code)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "expected a data object, got %v", body)
	assert.Equal(t, "settled", data["status"])
	billing, ok := data["billing"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "0.000613", billing["base_amount"], "decimal strings must pass through untouched")
	assert.Equal(t, "0.000613", billing["total_amount"])
	assert.Equal(t, "CNY", billing["currency"])

	// A pending request is still a successful response.
	recorder, body = query(owner.Id, "req-pending")
	require.Equal(t, http.StatusOK, recorder.Code, "pending must not be an error status")
	data, _ = body["data"].(map[string]any)
	require.NotNil(t, data)
	assert.Equal(t, "pending", data["status"])

	// Another token sees exactly the same 404 as for an unknown id.
	recorderOther, _ := query(other.Id, "req-settled")
	recorderMissing, _ := query(owner.Id, "req-does-not-exist")
	assert.Equal(t, http.StatusNotFound, recorderOther.Code)
	assert.Equal(t, http.StatusNotFound, recorderMissing.Code)
	assert.Equal(t, recorderMissing.Body.String(), recorderOther.Body.String(),
		"another token's settlement must be indistinguishable from a missing one")
}
