package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitializeAgentModuleIsIdempotent(t *testing.T) {
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousType := common.MainDatabaseType()
	previousEnabled := common.AgentEnabled
	previousInitialized := common.AgentInitialized
	previousRate := common.AgentDefaultRebateRate
	common.OptionMapRWMutex.RLock()
	previousOptions := common.OptionMap
	common.OptionMapRWMutex.RUnlock()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Option{}))
	model.InitOptionMap()
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousType)
		common.AgentEnabled = previousEnabled
		common.AgentInitialized = previousInitialized
		common.AgentDefaultRebateRate = previousRate
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	status, err := InitializeAgentModule(1200)
	require.NoError(t, err)
	assert.True(t, status.MigrationReady)
	assert.True(t, status.Enabled)
	assert.True(t, status.Initialized)
	assert.Equal(t, 1200, status.DefaultRate)

	status, err = InitializeAgentModule(1200)
	require.NoError(t, err)
	assert.True(t, status.Initialized)
	var count int64
	require.NoError(t, db.Model(&model.AgentRebateGroup{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
