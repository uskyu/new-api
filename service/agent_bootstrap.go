package service

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type AgentBootstrapStatus struct {
	Enabled          bool     `json:"enabled"`
	Initialized      bool     `json:"initialized"`
	MigrationReady   bool     `json:"migration_ready"`
	MissingResources []string `json:"missing_resources,omitempty"`
	DefaultGroupId   int      `json:"default_group_id"`
	DefaultRate      int      `json:"default_rate"`
}

func GetAgentBootstrapStatus() (*AgentBootstrapStatus, error) {
	if model.DB == nil {
		return nil, errors.New("database not initialized")
	}
	model.RefreshAgentRuntimeOptions()
	status := &AgentBootstrapStatus{
		Enabled:     common.AgentEnabled,
		Initialized: common.AgentInitialized,
		DefaultRate: common.AgentDefaultRebateRate,
	}
	missingResources, err := model.CheckAgentSchemaReady()
	if err != nil {
		return nil, err
	}
	status.MissingResources = missingResources
	status.MigrationReady = len(status.MissingResources) == 0
	if group, err := model.GetDefaultAgentRebateGroup(); err == nil {
		status.DefaultGroupId = group.Id
		if status.DefaultRate == 0 {
			status.DefaultRate = group.RebateRate
		}
	}
	return status, nil
}

func InitializeAgentModule(defaultRate int) (*AgentBootstrapStatus, error) {
	if model.DB == nil {
		return nil, errors.New("database not initialized")
	}
	if defaultRate < 0 || defaultRate > model.AgentMaxRebateRate {
		return nil, errors.New("default rebate rate must be between 0 and 10000")
	}
	if err := model.MigrateAgentSchema(); err != nil {
		return nil, err
	}
	status, err := GetAgentBootstrapStatus()
	if err != nil {
		return nil, err
	}
	if !status.MigrationReady {
		return nil, errors.New("agent migration is not ready")
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		_, err := model.CreateDefaultAgentRebateGroupTx(tx, defaultRate)
		return err
	}); err != nil {
		return nil, err
	}
	if err := model.UpdateOptionsBulk(map[string]string{
		"AgentDefaultRebateRate": strconv.Itoa(defaultRate),
		"AgentInitialized":       "true",
		"AgentEnabled":           "true",
	}); err != nil {
		return nil, err
	}
	return GetAgentBootstrapStatus()
}
