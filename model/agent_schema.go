package model

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
)

func RefreshAgentRuntimeOptions() {
	common.OptionMapRWMutex.RLock()
	enabledValue, hasEnabled := common.OptionMap["AgentEnabled"]
	initializedValue, hasInitialized := common.OptionMap["AgentInitialized"]
	defaultRateValue, hasDefaultRate := common.OptionMap["AgentDefaultRebateRate"]
	common.OptionMapRWMutex.RUnlock()

	if hasEnabled {
		common.AgentEnabled, _ = strconv.ParseBool(enabledValue)
	}
	if hasInitialized {
		common.AgentInitialized, _ = strconv.ParseBool(initializedValue)
	}
	if hasDefaultRate {
		common.AgentDefaultRebateRate, _ = strconv.Atoi(defaultRateValue)
	}
}

// AgentSchemaModels returns the additive tables owned by the agent module.
func AgentSchemaModels() []any {
	return []any{
		&AgentRebateGroup{},
		&AgentProfile{},
		&AgentPromoLink{},
		&AgentRebateRecord{},
		&AgentRedemptionRebateRecord{},
		&AgentRebateAdjustment{},
		&AgentRelationship{},
		&AgentUpgradeRequest{},
		&AgentWithdrawAccount{},
		&AgentWithdrawRequest{},
		&AgentBalanceLedger{},
	}
}

// MigrateAgentSchema only creates missing tables and columns. Existing agent
// data and historical rebate records are never rewritten or removed.
func MigrateAgentSchema() error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	if err := DB.AutoMigrate(AgentSchemaModels()...); err != nil {
		return err
	}
	if !DB.Migrator().HasColumn(&User{}, "PromoLinkId") {
		if err := DB.Migrator().AddColumn(&User{}, "PromoLinkId"); err != nil {
			return err
		}
	}
	if !DB.Migrator().HasIndex(&User{}, "PromoLinkId") {
		if err := DB.Migrator().CreateIndex(&User{}, "PromoLinkId"); err != nil {
			return err
		}
	}
	return nil
}

func CheckAgentSchemaReady() ([]string, error) {
	missing := make([]string, 0, len(AgentSchemaModels())+2)
	if DB == nil {
		return append(missing, "database"), nil
	}
	resources := []struct {
		name  string
		model any
	}{
		{"agent_rebate_groups", &AgentRebateGroup{}},
		{"agent_profiles", &AgentProfile{}},
		{"agent_promo_links", &AgentPromoLink{}},
		{"agent_rebate_records", &AgentRebateRecord{}},
		{"agent_redemption_rebate_records", &AgentRedemptionRebateRecord{}},
		{"agent_rebate_adjustments", &AgentRebateAdjustment{}},
		{"agent_relationships", &AgentRelationship{}},
		{"agent_upgrade_requests", &AgentUpgradeRequest{}},
		{"agent_withdraw_accounts", &AgentWithdrawAccount{}},
		{"agent_withdraw_requests", &AgentWithdrawRequest{}},
		{"agent_balance_ledgers", &AgentBalanceLedger{}},
	}
	for _, resource := range resources {
		if !DB.Migrator().HasTable(resource.model) {
			missing = append(missing, resource.name)
		}
	}
	if !DB.Migrator().HasColumn(&User{}, "PromoLinkId") {
		missing = append(missing, "users.promo_link_id")
	}
	if DB.Migrator().HasTable(&AgentProfile{}) && !DB.Migrator().HasColumn(&AgentProfile{}, "RebateFrozenAmount") {
		missing = append(missing, "agent_profiles.rebate_frozen_amount")
	}
	return missing, nil
}
