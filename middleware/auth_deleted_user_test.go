package middleware

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTokenAuthRejectsTokenOfSoftDeletedUser(t *testing.T) {
	dir := t.TempDir()
	previousPath := common.SQLitePath
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousSQLite, previousMySQL, previousPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	previousMaster := common.IsMasterNode
	previousRedis := common.RedisEnabled
	t.Cleanup(func() {
		if model.DB != nil {
			if sqlDB, err := model.DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SQLitePath = previousPath
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = previousSQLite, previousMySQL, previousPostgres
		common.IsMasterNode = previousMaster
		common.RedisEnabled = previousRedis
	})

	t.Setenv("SQL_DSN", "local")
	common.SQLitePath = filepath.ToSlash(filepath.Join(dir, "auth.db"))
	common.IsMasterNode = true
	common.RedisEnabled = false
	require.NoError(t, model.InitDB())
	model.LOG_DB = model.DB

	user := model.User{
		Username: "deleted-api-key-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, model.DB.Create(&user).Error)
	token := model.Token{
		UserId:         user.Id,
		Key:            "deletedtoken",
		Status:         common.TokenStatusEnabled,
		UnlimitedQuota: true,
		ExpiredTime:    -1,
	}
	require.NoError(t, model.DB.Create(&token).Error)
	require.NoError(t, model.DB.Delete(&user).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	context.Request.Header.Set("Authorization", "Bearer sk-deletedtoken")

	TokenAuth()(context)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}
