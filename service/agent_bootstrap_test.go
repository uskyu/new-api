package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAgentBootstrapTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.AgentEnabled = false
	common.AgentInitialized = false
	common.AgentDefaultRebateRate = 0

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Option{},
		&model.AgentRebateGroup{},
		&model.AgentProfile{},
		&model.AgentPromoLink{},
		&model.AgentRebateRecord{},
		&model.AgentRebateAdjustment{},
		&model.AgentRelationship{},
		&model.AgentUpgradeRequest{},
	))
	model.InitOptionMap()

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestGetAgentBootstrapStatusReady(t *testing.T) {
	setupAgentBootstrapTestDB(t)

	status, err := GetAgentBootstrapStatus()
	require.NoError(t, err)
	require.True(t, status.MigrationReady)
	require.Empty(t, status.MissingResources)
	require.False(t, status.Enabled)
	require.False(t, status.Initialized)
}

func TestInitializeAgentModule(t *testing.T) {
	setupAgentBootstrapTestDB(t)

	status, err := InitializeAgentModule(1200)
	require.NoError(t, err)
	require.True(t, status.MigrationReady)
	require.True(t, status.Enabled)
	require.True(t, status.Initialized)
	require.Equal(t, 1200, status.DefaultRate)
	require.NotZero(t, status.DefaultGroupId)

	group, err := model.GetDefaultAgentRebateGroup()
	require.NoError(t, err)
	require.Equal(t, 1200, group.RebateRate)
	require.True(t, group.IsDefault)

	require.Equal(t, "true", common.OptionMap["AgentEnabled"])
	require.Equal(t, "true", common.OptionMap["AgentInitialized"])
	require.Equal(t, "1200", common.OptionMap["AgentDefaultRebateRate"])

	status, err = InitializeAgentModule(1200)
	require.NoError(t, err)
	require.True(t, status.Initialized)

	var count int64
	require.NoError(t, model.DB.Model(&model.AgentRebateGroup{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}
