package model

import "fmt"

type extensionMigration struct {
	name string
	run  func() error
}

func migrateExtensionDB() error {
	models := extensionSchemaModels()
	if len(models) > 0 {
		if err := DB.AutoMigrate(models...); err != nil {
			return fmt.Errorf("migrate extension schema: %w", err)
		}
	}

	for _, migration := range extensionMigrations() {
		if migration.run == nil {
			continue
		}
		if err := migration.run(); err != nil {
			return fmt.Errorf("run extension migration %s: %w", migration.name, err)
		}
	}
	return nil
}

func extensionSchemaModels() []any {
	return nil
}

func extensionMigrations() []extensionMigration {
	return []extensionMigration{
		{name: "agent_schema_compat", run: MigrateAgentSchema},
	}
}
