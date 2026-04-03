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
	status := &AgentBootstrapStatus{
		Enabled:     common.AgentEnabled,
		Initialized: common.AgentInitialized,
		DefaultRate: common.AgentDefaultRebateRate,
	}
	status.MissingResources = detectAgentMigrationGaps(model.DB)
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
	if defaultRate < 0 {
		return nil, errors.New("default rebate rate must be >= 0")
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
	if err := model.UpdateOption("AgentDefaultRebateRate", strconv.Itoa(defaultRate)); err != nil {
		return nil, err
	}
	if err := model.UpdateOption("AgentInitialized", "true"); err != nil {
		return nil, err
	}
	if err := model.UpdateOption("AgentEnabled", "true"); err != nil {
		return nil, err
	}
	return GetAgentBootstrapStatus()
}

func detectAgentMigrationGaps(db *gorm.DB) []string {
	missing := make([]string, 0, 8)
	if db == nil {
		return append(missing, "database")
	}
	if !db.Migrator().HasTable(&model.AgentRebateGroup{}) {
		missing = append(missing, "agent_rebate_groups")
	}
	if !db.Migrator().HasTable(&model.AgentProfile{}) {
		missing = append(missing, "agent_profiles")
	}
	if !db.Migrator().HasTable(&model.AgentPromoLink{}) {
		missing = append(missing, "agent_promo_links")
	}
	if !db.Migrator().HasTable(&model.AgentRebateRecord{}) {
		missing = append(missing, "agent_rebate_records")
	}
	if !db.Migrator().HasTable(&model.AgentRebateAdjustment{}) {
		missing = append(missing, "agent_rebate_adjustments")
	}
	if !db.Migrator().HasTable(&model.AgentRelationship{}) {
		missing = append(missing, "agent_relationships")
	}
	if !db.Migrator().HasTable(&model.AgentUpgradeRequest{}) {
		missing = append(missing, "agent_upgrade_requests")
	}
	if !db.Migrator().HasColumn(&model.User{}, "promo_link_id") {
		missing = append(missing, "users.promo_link_id")
	}
	return missing
}
