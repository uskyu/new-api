package model

type agentSchemaCheckMigration struct{}

func (agentSchemaCheckMigration) Name() string {
	return "agent_schema_check"
}

func (agentSchemaCheckMigration) Run() error {
	_, err := CheckAgentSchemaReady()
	return err
}

func AgentMigrations() []Migration {
	return []Migration{
		agentSchemaCheckMigration{},
	}
}

func CheckAgentSchemaReady() ([]string, error) {
	missing := make([]string, 0, 12)
	if DB == nil {
		return append(missing, "database"), nil
	}
	if !DB.Migrator().HasTable(&AgentRebateGroup{}) {
		missing = append(missing, "agent_rebate_groups")
	}
	if !DB.Migrator().HasTable(&AgentProfile{}) {
		missing = append(missing, "agent_profiles")
	}
	if !DB.Migrator().HasTable(&AgentPromoLink{}) {
		missing = append(missing, "agent_promo_links")
	}
	if !DB.Migrator().HasTable(&AgentRebateRecord{}) {
		missing = append(missing, "agent_rebate_records")
	}
	if !DB.Migrator().HasTable(&AgentRebateAdjustment{}) {
		missing = append(missing, "agent_rebate_adjustments")
	}
	if !DB.Migrator().HasTable(&AgentRelationship{}) {
		missing = append(missing, "agent_relationships")
	}
	if !DB.Migrator().HasTable(&AgentUpgradeRequest{}) {
		missing = append(missing, "agent_upgrade_requests")
	}
	if !DB.Migrator().HasTable(&AgentWithdrawAccount{}) {
		missing = append(missing, "agent_withdraw_accounts")
	}
	if !DB.Migrator().HasTable(&AgentWithdrawRequest{}) {
		missing = append(missing, "agent_withdraw_requests")
	}
	if !DB.Migrator().HasTable(&AgentBalanceLedger{}) {
		missing = append(missing, "agent_balance_ledgers")
	}
	if !DB.Migrator().HasColumn(&User{}, "promo_link_id") {
		missing = append(missing, "users.promo_link_id")
	}
	if !DB.Migrator().HasColumn(&AgentProfile{}, "rebate_frozen_amount") {
		missing = append(missing, "agent_profiles.rebate_frozen_amount")
	}
	return missing, nil
}
