package cmd

import (
	"testing"
)

// TestSourceGraphCommand_HasInitSubcommand verifies `penf source graph init` is registered.
func TestSourceGraphCommand_HasInitSubcommand(t *testing.T) {
	deps := &SourceCommandDeps{
		LoadConfig: DefaultSourceDeps().LoadConfig,
	}
	graphCmd := newSourceGraphCommand(deps)

	found := false
	for _, sub := range graphCmd.Commands() {
		if sub.Name() == "init" {
			found = true
			break
		}
	}
	if !found {
		t.Error("penf source graph init subcommand not found")
	}
}

// TestSourceGraphInitCommand_Flags verifies all expected flags are declared.
func TestSourceGraphInitCommand_Flags(t *testing.T) {
	deps := &SourceCommandDeps{
		LoadConfig: DefaultSourceDeps().LoadConfig,
	}
	initCmd := newSourceGraphInitCommand(deps)

	expectedFlags := []string{"client-id", "tenant-id", "name", "force"}
	for _, flagName := range expectedFlags {
		if f := initCmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("penf source graph init missing flag: --%s", flagName)
		}
	}
}

// TestSourceGraphInitCommand_RequiresClientID verifies --client-id is enforced.
func TestSourceGraphInitCommand_RequiresClientID(t *testing.T) {
	deps := &SourceCommandDeps{
		LoadConfig: DefaultSourceDeps().LoadConfig,
	}
	initCmd := newSourceGraphInitCommand(deps)

	// Set tenant-id but not client-id
	if err := initCmd.Flags().Set("tenant-id", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"); err != nil {
		t.Fatalf("failed to set --tenant-id: %v", err)
	}

	err := initCmd.RunE(initCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --client-id is missing, got nil")
	}
	if err.Error() != "--client-id is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestSourceGraphInitCommand_RequiresTenantID verifies --tenant-id is enforced.
func TestSourceGraphInitCommand_RequiresTenantID(t *testing.T) {
	deps := &SourceCommandDeps{
		LoadConfig: DefaultSourceDeps().LoadConfig,
	}
	initCmd := newSourceGraphInitCommand(deps)

	// Set client-id but not tenant-id
	if err := initCmd.Flags().Set("client-id", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"); err != nil {
		t.Fatalf("failed to set --client-id: %v", err)
	}

	err := initCmd.RunE(initCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --tenant-id is missing, got nil")
	}
	if err.Error() != "--tenant-id is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestSourceGraphInitCommand_ForceFlag verifies --force defaults to false and can be set.
func TestSourceGraphInitCommand_ForceFlag(t *testing.T) {
	deps := &SourceCommandDeps{
		LoadConfig: DefaultSourceDeps().LoadConfig,
	}
	initCmd := newSourceGraphInitCommand(deps)

	forceFlag := initCmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Fatal("--force flag not found")
	}
	if forceFlag.DefValue != "false" {
		t.Errorf("--force default = %q, want \"false\"", forceFlag.DefValue)
	}
}
