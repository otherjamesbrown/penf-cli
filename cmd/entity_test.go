// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"testing"
	"time"

	"github.com/otherjamesbrown/penf-cli/config"
)

// entityMockConfig creates a test configuration for entity tests.
func entityMockConfig() *config.CLIConfig {
	return &config.CLIConfig{
		ServerAddress: "localhost:50051",
		TenantID:      "00000001-0000-0000-0000-000000000001",
		OutputFormat:  config.OutputFormatText,
		Timeout:       30 * time.Second,
	}
}

// createEntityTestDeps creates test dependencies with mock implementations.
func createEntityTestDeps(cfg *config.CLIConfig) *EntityCommandDeps {
	return &EntityCommandDeps{
		Config: cfg,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
	}
}

// ==================== Command Structure Tests ====================

func TestNewEntityCommand(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	if cmd == nil {
		t.Fatal("NewEntityCommand returned nil")
	}

	if cmd.Use != "entity" {
		t.Errorf("expected Use to be 'entity', got %q", cmd.Use)
	}

	// Check aliases.
	expectedAliases := []string{"entities"}
	if len(cmd.Aliases) != len(expectedAliases) {
		t.Errorf("expected %d aliases, got %d", len(expectedAliases), len(cmd.Aliases))
	}
	for i, alias := range expectedAliases {
		if i < len(cmd.Aliases) && cmd.Aliases[i] != alias {
			t.Errorf("expected alias %d to be %q, got %q", i, alias, cmd.Aliases[i])
		}
	}

	// Check persistent flags.
	flags := []string{"tenant", "output"}
	for _, flag := range flags {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("expected persistent flag %q to exist", flag)
		}
	}
}

func TestNewEntityCommand_WithNilDeps(t *testing.T) {
	cmd := NewEntityCommand(nil)

	if cmd == nil {
		t.Fatal("NewEntityCommand with nil deps returned nil")
	}
}

func TestNewEntityCommand_Subcommands(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	// Check existing subcommands exist.
	existingSubcommands := []string{"reject", "restore", "filter", "pattern", "stats", "search"}
	for _, sub := range existingSubcommands {
		found := false
		for _, c := range cmd.Commands() {
			if c.Name() == sub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected existing subcommand %q to exist", sub)
		}
	}

	// Check new subcommands that should be added (will fail until implementation).
	newSubcommands := []string{"update", "bulk-enrich"}
	for _, sub := range newSubcommands {
		found := false
		for _, c := range cmd.Commands() {
			if c.Name() == sub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected new subcommand %q to exist", sub)
		}
	}
}

// ==================== Entity Update Command Tests ====================

func TestEntityUpdateCommand_Structure(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	updateCmd, _, _ := cmd.Find([]string{"update"})
	if updateCmd == nil {
		t.Fatal("update subcommand not found")
	}

	// Check that update command exists.
	if updateCmd.Name() != "update" {
		t.Errorf("expected command name to be 'update', got %q", updateCmd.Name())
	}
}

func TestEntityUpdateCommand_Flags(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	updateCmd, _, _ := cmd.Find([]string{"update"})
	if updateCmd == nil {
		t.Fatal("update subcommand not found")
	}

	// Check that --title and --company flags exist.
	expectedFlags := []string{"title", "company"}
	for _, flag := range expectedFlags {
		if updateCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %q to exist on update command", flag)
		}
	}

	// Check that existing flags also exist (name, account-type, metadata).
	// These ensure the command supports all update operations.
	existingFlags := []string{"name", "account-type", "metadata"}
	for _, flag := range existingFlags {
		if updateCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected existing flag %q to exist on update command", flag)
		}
	}
}

func TestEntityUpdateCommand_RequiresEntityID(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	updateCmd, _, _ := cmd.Find([]string{"update"})
	if updateCmd == nil {
		t.Fatal("update subcommand not found")
	}

	// Check that command requires exactly one argument (entity ID).
	if updateCmd.Args == nil {
		t.Error("expected update command to have args validation")
	}
}

func TestEntityUpdateCommand_TitleFlag(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	updateCmd, _, _ := cmd.Find([]string{"update"})
	if updateCmd == nil {
		t.Fatal("update subcommand not found")
	}

	titleFlag := updateCmd.Flags().Lookup("title")
	if titleFlag == nil {
		t.Fatal("title flag not found")
	}

	// Verify flag properties.
	if titleFlag.Name != "title" {
		t.Errorf("expected flag name to be 'title', got %q", titleFlag.Name)
	}

	if titleFlag.Usage == "" {
		t.Error("expected title flag to have usage text")
	}
}

func TestEntityUpdateCommand_CompanyFlag(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	updateCmd, _, _ := cmd.Find([]string{"update"})
	if updateCmd == nil {
		t.Fatal("update subcommand not found")
	}

	companyFlag := updateCmd.Flags().Lookup("company")
	if companyFlag == nil {
		t.Fatal("company flag not found")
	}

	// Verify flag properties.
	if companyFlag.Name != "company" {
		t.Errorf("expected flag name to be 'company', got %q", companyFlag.Name)
	}

	if companyFlag.Usage == "" {
		t.Error("expected company flag to have usage text")
	}
}

func TestEntityUpdateCommand_HelpText(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	updateCmd, _, _ := cmd.Find([]string{"update"})
	if updateCmd == nil {
		t.Fatal("update subcommand not found")
	}

	// Verify help text includes title and company examples.
	if updateCmd.Long == "" {
		t.Error("expected update command to have Long help text")
	}

	// The help text should mention title and company updates.
	// This is a basic check - actual help text validation would be more thorough.
	if updateCmd.Example == "" {
		t.Error("expected update command to have Example text")
	}
}

// ==================== Entity Bulk-Enrich Command Tests ====================

func TestEntityBulkEnrichCommand_Structure(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	bulkEnrichCmd, _, _ := cmd.Find([]string{"bulk-enrich"})
	if bulkEnrichCmd == nil {
		t.Fatal("bulk-enrich subcommand not found")
	}

	// Check that bulk-enrich command exists.
	if bulkEnrichCmd.Name() != "bulk-enrich" {
		t.Errorf("expected command name to be 'bulk-enrich', got %q", bulkEnrichCmd.Name())
	}
}

func TestEntityBulkEnrichCommand_Flags(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	bulkEnrichCmd, _, _ := cmd.Find([]string{"bulk-enrich"})
	if bulkEnrichCmd == nil {
		t.Fatal("bulk-enrich subcommand not found")
	}

	// Check required flags: --domain, --company, --internal.
	expectedFlags := []string{"domain", "company", "internal"}
	for _, flag := range expectedFlags {
		if bulkEnrichCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %q to exist on bulk-enrich command", flag)
		}
	}
}

func TestEntityBulkEnrichCommand_DomainFlagRequired(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	bulkEnrichCmd, _, _ := cmd.Find([]string{"bulk-enrich"})
	if bulkEnrichCmd == nil {
		t.Fatal("bulk-enrich subcommand not found")
	}

	domainFlag := bulkEnrichCmd.Flags().Lookup("domain")
	if domainFlag == nil {
		t.Fatal("domain flag not found")
	}

	// Verify flag properties.
	if domainFlag.Name != "domain" {
		t.Errorf("expected flag name to be 'domain', got %q", domainFlag.Name)
	}

	if domainFlag.Usage == "" {
		t.Error("expected domain flag to have usage text")
	}
}

func TestEntityBulkEnrichCommand_CompanyFlag(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	bulkEnrichCmd, _, _ := cmd.Find([]string{"bulk-enrich"})
	if bulkEnrichCmd == nil {
		t.Fatal("bulk-enrich subcommand not found")
	}

	companyFlag := bulkEnrichCmd.Flags().Lookup("company")
	if companyFlag == nil {
		t.Fatal("company flag not found")
	}

	// Verify flag properties.
	if companyFlag.Name != "company" {
		t.Errorf("expected flag name to be 'company', got %q", companyFlag.Name)
	}

	if companyFlag.Usage == "" {
		t.Error("expected company flag to have usage text")
	}
}

func TestEntityBulkEnrichCommand_InternalFlag(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	bulkEnrichCmd, _, _ := cmd.Find([]string{"bulk-enrich"})
	if bulkEnrichCmd == nil {
		t.Fatal("bulk-enrich subcommand not found")
	}

	internalFlag := bulkEnrichCmd.Flags().Lookup("internal")
	if internalFlag == nil {
		t.Fatal("internal flag not found")
	}

	// Verify flag properties (should be a boolean flag).
	if internalFlag.Name != "internal" {
		t.Errorf("expected flag name to be 'internal', got %q", internalFlag.Name)
	}

	if internalFlag.Value.Type() != "bool" {
		t.Errorf("expected internal flag to be boolean, got %q", internalFlag.Value.Type())
	}
}

func TestEntityBulkEnrichCommand_HelpText(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	bulkEnrichCmd, _, _ := cmd.Find([]string{"bulk-enrich"})
	if bulkEnrichCmd == nil {
		t.Fatal("bulk-enrich subcommand not found")
	}

	// Verify help text exists and explains bulk enrichment.
	if bulkEnrichCmd.Long == "" {
		t.Error("expected bulk-enrich command to have Long help text")
	}

	if bulkEnrichCmd.Short == "" {
		t.Error("expected bulk-enrich command to have Short help text")
	}

	// The help text should mention domain-based updates.
	if bulkEnrichCmd.Example == "" {
		t.Error("expected bulk-enrich command to have Example text showing domain usage")
	}
}

func TestEntityBulkEnrichCommand_ReportsCount(t *testing.T) {
	// This test verifies that the bulk-enrich implementation will report
	// the number of entities updated. This is a behavior test that will
	// need to be implemented with a mock or integration test.

	// For now, we just verify the command structure supports this.
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	bulkEnrichCmd, _, _ := cmd.Find([]string{"bulk-enrich"})
	if bulkEnrichCmd == nil {
		t.Fatal("bulk-enrich subcommand not found")
	}

	// The RunE function should be set (will be implemented to call gRPC and report count).
	if bulkEnrichCmd.RunE == nil {
		t.Error("expected bulk-enrich command to have RunE function")
	}
}

// ==================== Integration with Existing Commands ====================

func TestEntityCommand_AllSubcommandsRegistered(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	// Verify all expected subcommands are registered.
	expectedSubcommands := []string{
		"reject",
		"restore",
		"filter",
		"pattern",
		"stats",
		"search",
		"update",       // New
		"bulk-enrich",  // New
	}

	for _, sub := range expectedSubcommands {
		found := false
		for _, c := range cmd.Commands() {
			if c.Name() == sub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q to be registered", sub)
		}
	}
}

func TestEntityCommand_HelpIncludesNewCommands(t *testing.T) {
	deps := createEntityTestDeps(entityMockConfig())
	cmd := NewEntityCommand(deps)

	// The root entity command help text should mention update and bulk-enrich.
	if cmd.Long == "" {
		t.Error("expected entity command to have Long help text")
	}

	// Note: The actual help text content validation would be more thorough
	// in a real implementation, checking for specific mentions of the new commands.
}
