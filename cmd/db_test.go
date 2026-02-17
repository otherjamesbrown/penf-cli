// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDbCommand tests the parent db command structure.
func TestDbCommand(t *testing.T) {
	cmd := NewDbCommand()

	assert.NotNil(t, cmd, "NewDbCommand() should not return nil")
	assert.Equal(t, "db", cmd.Use, "db command Use should be 'db'")
	assert.NotEmpty(t, cmd.Short, "db command should have Short description")
	assert.NotEmpty(t, cmd.Long, "db command should have Long description")
}

// TestDbCommand_HasSubcommands verifies the db command has migrate and status subcommands.
func TestDbCommand_HasSubcommands(t *testing.T) {
	cmd := NewDbCommand()

	subcommands := cmd.Commands()
	require.NotEmpty(t, subcommands, "db command should have subcommands")

	// Look for migrate subcommand
	migrateFound := false
	statusFound := false

	for _, sub := range subcommands {
		switch sub.Use {
		case "migrate":
			migrateFound = true
		case "status":
			statusFound = true
		}
	}

	assert.True(t, migrateFound, "db command should have 'migrate' subcommand")
	assert.True(t, statusFound, "db command should have 'status' subcommand")
}

// TestDbMigrateCommand_Help verifies the migrate subcommand has expected flags.
func TestDbMigrateCommand_Help(t *testing.T) {
	cmd := NewDbCommand()

	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	require.NoError(t, err, "should find migrate subcommand")
	require.NotNil(t, migrateCmd, "migrate subcommand should not be nil")

	assert.Equal(t, "migrate", migrateCmd.Use, "migrate subcommand Use should be 'migrate'")
	assert.NotEmpty(t, migrateCmd.Short, "migrate subcommand should have Short description")
	assert.NotEmpty(t, migrateCmd.Long, "migrate subcommand should have Long description")

	// Check for --dry-run flag
	dryRunFlag := migrateCmd.Flags().Lookup("dry-run")
	assert.NotNil(t, dryRunFlag, "migrate command should have --dry-run flag")
	assert.Equal(t, "bool", dryRunFlag.Value.Type(), "--dry-run should be a boolean flag")

	// Check for --target flag
	targetFlag := migrateCmd.Flags().Lookup("target")
	assert.NotNil(t, targetFlag, "migrate command should have --target flag")
	assert.Equal(t, "string", targetFlag.Value.Type(), "--target should be a string flag")
}

// TestDbMigrateCommand_FlagDescriptions verifies flag help text is present.
func TestDbMigrateCommand_FlagDescriptions(t *testing.T) {
	cmd := NewDbCommand()

	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	require.NoError(t, err)
	require.NotNil(t, migrateCmd)

	dryRunFlag := migrateCmd.Flags().Lookup("dry-run")
	require.NotNil(t, dryRunFlag)
	assert.NotEmpty(t, dryRunFlag.Usage, "--dry-run flag should have usage description")

	targetFlag := migrateCmd.Flags().Lookup("target")
	require.NotNil(t, targetFlag)
	assert.NotEmpty(t, targetFlag.Usage, "--target flag should have usage description")
}

// TestDbStatusCommand_Help verifies the status subcommand structure.
func TestDbStatusCommand_Help(t *testing.T) {
	cmd := NewDbCommand()

	statusCmd, _, err := cmd.Find([]string{"status"})
	require.NoError(t, err, "should find status subcommand")
	require.NotNil(t, statusCmd, "status subcommand should not be nil")

	assert.Equal(t, "status", statusCmd.Use, "status subcommand Use should be 'status'")
	assert.NotEmpty(t, statusCmd.Short, "status subcommand should have Short description")
	assert.NotEmpty(t, statusCmd.Long, "status subcommand should have Long description")
}

// TestDbStatusCommand_OutputFlag verifies the status command has output format flag.
func TestDbStatusCommand_OutputFlag(t *testing.T) {
	cmd := NewDbCommand()

	statusCmd, _, err := cmd.Find([]string{"status"})
	require.NoError(t, err)
	require.NotNil(t, statusCmd)

	// Check for --output or --format flag (common CLI pattern)
	outputFlag := statusCmd.Flags().Lookup("output")
	formatFlag := statusCmd.Flags().Lookup("format")

	// At least one output format flag should exist
	hasOutputFlag := outputFlag != nil || formatFlag != nil
	assert.True(t, hasOutputFlag, "status command should have --output or --format flag")
}

// TestDbMigrateCommand_Examples verifies migrate command has examples in help text.
func TestDbMigrateCommand_Examples(t *testing.T) {
	cmd := NewDbCommand()

	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	require.NoError(t, err)
	require.NotNil(t, migrateCmd)

	// Commands should have examples for AI agents to understand usage
	assert.NotEmpty(t, migrateCmd.Example, "migrate command should have example usage")
}

// TestDbStatusCommand_Examples verifies status command has examples in help text.
func TestDbStatusCommand_Examples(t *testing.T) {
	cmd := NewDbCommand()

	statusCmd, _, err := cmd.Find([]string{"status"})
	require.NoError(t, err)
	require.NotNil(t, statusCmd)

	// Commands should have examples for AI agents to understand usage
	assert.NotEmpty(t, statusCmd.Example, "status command should have example usage")
}
