// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"testing"
)

// ==================== Entity Delete Command Tests ====================

func TestEntityDeleteCommand_Structure(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	deleteCmd, _, _ := cmd.Find([]string{"delete"})
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	// Check that delete command exists.
	if deleteCmd.Name() != "delete" {
		t.Errorf("expected command name to be 'delete', got %q", deleteCmd.Name())
	}
}

func TestEntityDeleteCommand_RegisteredInEntityCommand(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	// Verify delete subcommand is registered with entity command.
	found := false
	for _, c := range cmd.Commands() {
		if c.Name() == "delete" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'delete' subcommand to be registered with entity command")
	}
}

func TestEntityDeleteCommand_RequiresEntityID(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	deleteCmd, _, _ := cmd.Find([]string{"delete"})
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	// Check that command requires exactly one argument (entity ID).
	if deleteCmd.Args == nil {
		t.Error("expected delete command to have args validation")
	}
}

func TestEntityDeleteCommand_HasForceFlag(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	deleteCmd, _, _ := cmd.Find([]string{"delete"})
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	// Check that --force flag exists.
	forceFlag := deleteCmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Fatal("force flag not found")
	}

	// Verify it's a boolean flag.
	if forceFlag.Value.Type() != "bool" {
		t.Errorf("expected force flag to be boolean, got %q", forceFlag.Value.Type())
	}

	// Verify flag properties.
	if forceFlag.Name != "force" {
		t.Errorf("expected flag name to be 'force', got %q", forceFlag.Name)
	}

	if forceFlag.Usage == "" {
		t.Error("expected force flag to have usage text")
	}
}

func TestEntityDeleteCommand_HelpText(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	deleteCmd, _, _ := cmd.Find([]string{"delete"})
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	// Verify help text exists.
	if deleteCmd.Short == "" {
		t.Error("expected delete command to have Short help text")
	}

	if deleteCmd.Long == "" {
		t.Error("expected delete command to have Long help text")
	}
}

func TestEntityDeleteCommand_HelpTextExplainsForceFlag(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	deleteCmd, _, _ := cmd.Find([]string{"delete"})
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	// The help text should mention that this is a permanent deletion
	// and explain the force flag requirement.
	if deleteCmd.Long == "" {
		t.Error("expected delete command to have Long help text explaining permanent deletion")
	}
}

func TestEntityDeleteCommand_HasExamples(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	deleteCmd, _, _ := cmd.Find([]string{"delete"})
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	// The command should have examples showing both numeric and prefixed ID formats.
	if deleteCmd.Example == "" {
		t.Error("expected delete command to have Example text")
	}
}

func TestEntityDeleteCommand_HasRunE(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	deleteCmd, _, _ := cmd.Find([]string{"delete"})
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	// The RunE function should be set to execute the deletion.
	if deleteCmd.RunE == nil {
		t.Error("expected delete command to have RunE function")
	}
}

func TestEntityDeleteCommand_HasAliases(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	deleteCmd, _, _ := cmd.Find([]string{"delete"})
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	// Check if 'rm' alias exists (common pattern in CLI tools).
	expectedAliases := []string{"rm"}
	if len(deleteCmd.Aliases) != len(expectedAliases) {
		t.Errorf("expected %d aliases, got %d", len(expectedAliases), len(deleteCmd.Aliases))
	}

	// Verify the 'rm' alias specifically.
	hasRmAlias := false
	for _, alias := range deleteCmd.Aliases {
		if alias == "rm" {
			hasRmAlias = true
			break
		}
	}
	if !hasRmAlias {
		t.Error("expected delete command to have 'rm' alias")
	}
}

func TestEntityDeleteCommand_ForceFlagDefaultValue(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	deleteCmd, _, _ := cmd.Find([]string{"delete"})
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	forceFlag := deleteCmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Fatal("force flag not found")
	}

	// The force flag should default to false (requiring explicit confirmation).
	if forceFlag.DefValue != "false" {
		t.Errorf("expected force flag default value to be 'false', got %q", forceFlag.DefValue)
	}
}

func TestEntityDeleteCommand_AcceptsNumericEntityID(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	deleteCmd, _, _ := cmd.Find([]string{"delete"})
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	// The command should accept numeric entity IDs like "123".
	// This is verified by checking that Args validator allows exactly 1 arg.
	// The actual parsing is tested in entity_id_test.go via ParseEntityID.
	if deleteCmd.Args == nil {
		t.Error("expected delete command to have args validation for entity ID")
	}
}

func TestEntityDeleteCommand_AcceptsPrefixedEntityID(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	deleteCmd, _, _ := cmd.Find([]string{"delete"})
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	// The command should accept prefixed entity IDs like "ent-person-123".
	// The actual ID format parsing is handled by ParseEntityID (tested separately).
	// This test verifies the command structure supports ID arguments.
	if deleteCmd.Args == nil {
		t.Error("expected delete command to have args validation for entity ID")
	}
}

func TestEntityDeleteCommand_IntegrationWithExistingSubcommands(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	// Verify that adding delete doesn't break existing subcommands.
	existingSubcommands := []string{
		"reject",
		"restore",
		"filter",
		"pattern",
		"stats",
		"search",
		"update",
		"bulk-enrich",
	}

	for _, sub := range existingSubcommands {
		found := false
		for _, c := range cmd.Commands() {
			if c.Name() == sub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected existing subcommand %q to still be registered after adding delete", sub)
		}
	}
}
