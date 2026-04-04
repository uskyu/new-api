package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testMigration struct {
	name string
	run  func() error
}

func (m testMigration) Name() string {
	return m.name
}

func (m testMigration) Run() error {
	if m.run == nil {
		return nil
	}
	return m.run()
}

func TestCheckAgentSchemaReadyWithNilDB(t *testing.T) {
	prevDB := DB
	DB = nil
	t.Cleanup(func() {
		DB = prevDB
	})

	missing, err := CheckAgentSchemaReady()
	require.NoError(t, err)
	require.Equal(t, []string{"database"}, missing)
}

func TestCheckAgentSchemaReady(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)

	missing, err := CheckAgentSchemaReady()
	require.NoError(t, err)
	require.Empty(t, missing)
}

func TestMigrationRunnerRunAll(t *testing.T) {
	order := make([]string, 0, 2)
	runner := NewMigrationRunner()
	err := runner.RunAll(
		testMigration{name: "first", run: func() error {
			order = append(order, "first")
			return nil
		}},
		testMigration{name: "second", run: func() error {
			order = append(order, "second")
			return nil
		}},
	)
	require.NoError(t, err)
	require.Equal(t, []string{"first", "second"}, order)
}
