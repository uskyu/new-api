package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAgentOptionsRestoreRuntimeStateAfterInitialization(t *testing.T) {
	previousDB := DB
	previousEnabled := common.AgentEnabled
	previousInitialized := common.AgentInitialized
	previousRate := common.AgentDefaultRebateRate
	common.OptionMapRWMutex.RLock()
	previousOptions := common.OptionMap
	common.OptionMapRWMutex.RUnlock()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, db.AutoMigrate(&Option{}))
	require.NoError(t, db.Create([]Option{
		{Key: "AgentEnabled", Value: "true"},
		{Key: "AgentInitialized", Value: "true"},
		{Key: "AgentDefaultRebateRate", Value: "1750"},
	}).Error)
	common.AgentEnabled = false
	common.AgentInitialized = false
	common.AgentDefaultRebateRate = 0
	t.Cleanup(func() {
		DB = previousDB
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

	InitOptionMap()
	assert.True(t, common.AgentEnabled)
	assert.True(t, common.AgentInitialized)
	assert.Equal(t, 1750, common.AgentDefaultRebateRate)
	assert.Equal(t, "true", common.OptionMap["AgentEnabled"])
	assert.Equal(t, "true", common.OptionMap["AgentInitialized"])
	assert.Equal(t, "1750", common.OptionMap["AgentDefaultRebateRate"])

	require.NoError(t, UpdateOption("AgentEnabled", "false"))
	require.NoError(t, UpdateOption("AgentInitialized", "false"))
	require.NoError(t, UpdateOption("AgentDefaultRebateRate", "900"))
	assert.False(t, common.AgentEnabled)
	assert.False(t, common.AgentInitialized)
	assert.Equal(t, 900, common.AgentDefaultRebateRate)
}
