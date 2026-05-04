package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

type Migration interface {
	Name() string
	Run() error
}

type MigrationRunner struct{}

func NewMigrationRunner() *MigrationRunner {
	return &MigrationRunner{}
}

func (r *MigrationRunner) RunAll(migrations ...Migration) error {
	for _, migration := range migrations {
		if migration == nil {
			continue
		}
		if err := migration.Run(); err != nil {
			return fmt.Errorf("run migration %s: %w", migration.Name(), err)
		}
	}
	return nil
}

type namedMigration struct {
	name string
	run  func() error
}

func (m namedMigration) Name() string {
	return m.name
}

func (m namedMigration) Run() error {
	if m.run == nil {
		return nil
	}
	return m.run()
}

func CoreSchemaModels() []interface{} {
	return []interface{}{
		&Channel{},
		&Token{},
		&User{},
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
		&PasskeyCredential{},
		&Option{},
		&Redemption{},
		&Ability{},
		&Log{},
		&Midjourney{},
		&TopUp{},
		&QuotaData{},
		&Task{},
		&ImageTask{},
		&ImagePromptFavorite{},
		&EcommerceWorkflow{},
		&EcommerceWorkflowSegment{},
		&Model{},
		&Vendor{},
		&PrefillGroup{},
		&Setup{},
		&TwoFA{},
		&TwoFABackupCode{},
		&Checkin{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&SubscriptionPreConsumeRecord{},
		&CustomOAuthProvider{},
		&UserOAuthBinding{},
	}
}

func runCompatMigrations(migrations ...Migration) error {
	return NewMigrationRunner().RunAll(migrations...)
}

func preSchemaCompatMigrations() []Migration {
	return []Migration{
		namedMigration{
			name: "subscription_plan_price_amount",
			run: func() error {
				migrateSubscriptionPlanPriceAmount()
				return nil
			},
		},
		namedMigration{
			name: "token_model_limits_to_text",
			run:  migrateTokenModelLimitsToText,
		},
	}
}

func postSchemaCompatMigrations() []Migration {
	return []Migration{
		namedMigration{
			name: "subscription_plan_schema",
			run: func() error {
				if common.UsingSQLite {
					return ensureSubscriptionPlanTableSQLite()
				}
				return DB.AutoMigrate(&SubscriptionPlan{})
			},
		},
	}
}
