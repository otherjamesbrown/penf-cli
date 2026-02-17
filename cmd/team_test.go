// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/otherjamesbrown/penf-cli/config"
)

// mockTeamConfig creates a mock configuration for team testing.
func mockTeamConfig() *config.CLIConfig {
	return &config.CLIConfig{
		ServerAddress: "localhost:50051",
		Timeout:       30 * time.Second,
		OutputFormat:  config.OutputFormatText,
		TenantID:      "tenant-test-001",
		TenantAliases: map[string]string{
			"work":     "tenant-acme-002",
			"personal": "tenant-default-001",
		},
		Debug:    false,
		Insecure: true,
	}
}

// createTeamTestDeps creates test dependencies for team commands.
func createTeamTestDeps(cfg *config.CLIConfig) *TeamCommandDeps {
	return &TeamCommandDeps{
		Config: cfg,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
	}
}

// TestGetTenantIDForTeam tests the tenant resolution priority.
func TestGetTenantIDForTeam(t *testing.T) {
	// Save original environment.
	originalEnv := os.Getenv("PENF_TENANT_ID")
	defer func() {
		if originalEnv != "" {
			os.Setenv("PENF_TENANT_ID", originalEnv)
		} else {
			os.Unsetenv("PENF_TENANT_ID")
		}
		// Reset global flag.
		teamTenant = ""
	}()

	t.Run("uses flag when provided", func(t *testing.T) {
		// Reset environment and flag.
		os.Unsetenv("PENF_TENANT_ID")
		teamTenant = "tenant-from-flag"

		cfg := mockTeamConfig()
		cfg.TenantID = "tenant-from-config"
		deps := createTeamTestDeps(cfg)

		tenantID, err := getTenantIDForTeam(deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tenantID != "tenant-from-flag" {
			t.Errorf("expected 'tenant-from-flag', got %q", tenantID)
		}

		// Clean up.
		teamTenant = ""
	})

	t.Run("uses env var when no flag", func(t *testing.T) {
		// Reset flag and set environment.
		teamTenant = ""
		os.Setenv("PENF_TENANT_ID", "tenant-from-env")

		cfg := mockTeamConfig()
		cfg.TenantID = "tenant-from-config"
		deps := createTeamTestDeps(cfg)

		tenantID, err := getTenantIDForTeam(deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tenantID != "tenant-from-env" {
			t.Errorf("expected 'tenant-from-env', got %q", tenantID)
		}

		// Clean up.
		os.Unsetenv("PENF_TENANT_ID")
	})

	t.Run("uses config when no flag or env", func(t *testing.T) {
		// Reset flag and environment.
		teamTenant = ""
		os.Unsetenv("PENF_TENANT_ID")

		cfg := mockTeamConfig()
		cfg.TenantID = "tenant-from-config"
		deps := createTeamTestDeps(cfg)

		tenantID, err := getTenantIDForTeam(deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tenantID != "tenant-from-config" {
			t.Errorf("expected 'tenant-from-config', got %q", tenantID)
		}
	})

	t.Run("returns error when no tenant configured", func(t *testing.T) {
		// Reset everything.
		teamTenant = ""
		os.Unsetenv("PENF_TENANT_ID")

		cfg := mockTeamConfig()
		cfg.TenantID = ""
		deps := createTeamTestDeps(cfg)

		tenantID, err := getTenantIDForTeam(deps)
		if err == nil {
			t.Error("expected error when no tenant configured")
		}

		if tenantID != "" {
			t.Errorf("expected empty tenant ID, got %q", tenantID)
		}

		// Verify error message is helpful.
		if err != nil && !strings.Contains(err.Error(), "tenant ID required") {
			t.Errorf("expected helpful error message, got: %v", err)
		}

		// Verify it does NOT return a hardcoded UUID.
		if err == nil && strings.Contains(tenantID, "-") && len(tenantID) == 36 {
			t.Error("getTenantIDForTeam should not return a hardcoded UUID when no tenant is configured")
		}
	})
}

// TestNewTeamCommand tests command structure and subcommands.
func TestNewTeamCommand(t *testing.T) {
	cfg := mockTeamConfig()
	deps := createTeamTestDeps(cfg)
	cmd := NewTeamCommand(deps)

	if cmd == nil {
		t.Fatal("NewTeamCommand returned nil")
	}

	if cmd.Use != "team" {
		t.Errorf("expected Use to be 'team', got %q", cmd.Use)
	}

	// Check aliases.
	expectedAliases := []string{"teams"}
	for _, expected := range expectedAliases {
		found := false
		for _, alias := range cmd.Aliases {
			if alias == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected alias %q not found", expected)
		}
	}

	// Check subcommands exist.
	subcommands := cmd.Commands()
	expectedSubcmds := []string{"list", "create", "show", "delete", "add-member", "remove-member", "members"}

	for _, expected := range expectedSubcmds {
		found := false
		for _, sub := range subcommands {
			if sub.Use == expected || strings.HasPrefix(sub.Use, expected+" ") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q not found", expected)
		}
	}
}

// TestNewTeamCommand_WithNilDeps tests that nil deps doesn't crash.
func TestNewTeamCommand_WithNilDeps(t *testing.T) {
	cmd := NewTeamCommand(nil)

	if cmd == nil {
		t.Fatal("NewTeamCommand with nil deps returned nil")
	}
}

// TestNewTeamCommand_Aliases tests command aliases.
func TestNewTeamCommand_Aliases(t *testing.T) {
	cfg := mockTeamConfig()
	deps := createTeamTestDeps(cfg)
	cmd := NewTeamCommand(deps)

	// Check list command has 'ls' alias.
	listCmd, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("failed to find list command: %v", err)
	}

	hasLsAlias := false
	for _, alias := range listCmd.Aliases {
		if alias == "ls" {
			hasLsAlias = true
			break
		}
	}
	if !hasLsAlias {
		t.Error("list command should have 'ls' alias")
	}

	// Check show command has aliases.
	showCmd, _, err := cmd.Find([]string{"show"})
	if err != nil {
		t.Fatalf("failed to find show command: %v", err)
	}

	expectedShowAliases := []string{"info", "get"}
	for _, expected := range expectedShowAliases {
		found := false
		for _, alias := range showCmd.Aliases {
			if alias == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("show command should have %q alias", expected)
		}
	}

	// Check delete command has aliases.
	deleteCmd, _, err := cmd.Find([]string{"delete"})
	if err != nil {
		t.Fatalf("failed to find delete command: %v", err)
	}

	expectedDeleteAliases := []string{"rm", "remove"}
	for _, expected := range expectedDeleteAliases {
		found := false
		for _, alias := range deleteCmd.Aliases {
			if alias == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("delete command should have %q alias", expected)
		}
	}

	// Check remove-member command has alias.
	removeMemberCmd, _, err := cmd.Find([]string{"remove-member"})
	if err != nil {
		t.Fatalf("failed to find remove-member command: %v", err)
	}

	hasRmMemberAlias := false
	for _, alias := range removeMemberCmd.Aliases {
		if alias == "rm-member" {
			hasRmMemberAlias = true
			break
		}
	}
	if !hasRmMemberAlias {
		t.Error("remove-member command should have 'rm-member' alias")
	}
}

// TestTeamList_RequiresTenant tests that team list fails gracefully without tenant.
func TestTeamList_RequiresTenant(t *testing.T) {
	// Reset environment and flag.
	originalEnv := os.Getenv("PENF_TENANT_ID")
	defer func() {
		if originalEnv != "" {
			os.Setenv("PENF_TENANT_ID", originalEnv)
		} else {
			os.Unsetenv("PENF_TENANT_ID")
		}
		teamTenant = ""
	}()

	teamTenant = ""
	os.Unsetenv("PENF_TENANT_ID")

	cfg := mockTeamConfig()
	cfg.TenantID = ""
	deps := createTeamTestDeps(cfg)

	// Use getTenantIDForTeam directly to test tenant resolution without gateway connection.
	tenantID, err := getTenantIDForTeam(deps)

	if err == nil {
		t.Error("expected error when no tenant ID configured")
	}

	if tenantID != "" {
		t.Errorf("expected empty tenant ID, got %q", tenantID)
	}

	// Verify error message is helpful.
	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "tenant ID required") {
			t.Errorf("error message should mention tenant requirement, got: %v", err)
		}

		// Error message should guide the user.
		if !strings.Contains(errMsg, "flag") || !strings.Contains(errMsg, "env") || !strings.Contains(errMsg, "config") {
			t.Errorf("error message should suggest how to provide tenant ID (flag, env, config), got: %v", err)
		}
	}
}

// TestGetTeamOutputFormat tests output format resolution.
func TestGetTeamOutputFormat(t *testing.T) {
	// Save original value.
	originalTeamOutput := teamOutput
	defer func() {
		teamOutput = originalTeamOutput
	}()

	t.Run("uses flag when provided", func(t *testing.T) {
		teamOutput = "json"
		cfg := mockTeamConfig()
		cfg.OutputFormat = config.OutputFormatText

		format := getTeamOutputFormat(cfg)
		if format != config.OutputFormatJSON {
			t.Errorf("expected JSON format, got %q", format)
		}

		teamOutput = ""
	})

	t.Run("uses config when no flag", func(t *testing.T) {
		teamOutput = ""
		cfg := mockTeamConfig()
		cfg.OutputFormat = config.OutputFormatYAML

		format := getTeamOutputFormat(cfg)
		if format != config.OutputFormatYAML {
			t.Errorf("expected YAML format, got %q", format)
		}
	})

	t.Run("defaults to text when no config", func(t *testing.T) {
		teamOutput = ""

		format := getTeamOutputFormat(nil)
		if format != config.OutputFormatText {
			t.Errorf("expected text format, got %q", format)
		}
	})
}

// TestTeamTruncateString tests the string truncation helper.
func TestTeamTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly30chars1234567890abcd", 30, "exactly30chars1234567890abcd"},
		{"this is a very long string that needs truncation", 20, "this is a very lo..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"}, // maxLen <= 3 returns s[:maxLen]
		{"", 5, ""},
		{"test", 2, "te"},
		{"long string here", 10, "long st..."},
	}

	for _, tt := range tests {
		result := teamTruncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("teamTruncateString(%q, %d) = %q, want %q",
				tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

// TestTeamCommandPersistentFlags tests that persistent flags are available.
func TestTeamCommandPersistentFlags(t *testing.T) {
	cfg := mockTeamConfig()
	deps := createTeamTestDeps(cfg)
	cmd := NewTeamCommand(deps)

	// Check that tenant flag exists.
	tenantFlag := cmd.PersistentFlags().Lookup("tenant")
	if tenantFlag == nil {
		t.Error("expected 'tenant' persistent flag to exist")
	}

	// Check shorthand.
	if tenantFlag != nil && tenantFlag.Shorthand != "t" {
		t.Errorf("expected tenant flag shorthand 't', got %q", tenantFlag.Shorthand)
	}

	// Check that output flag exists.
	outputFlag := cmd.PersistentFlags().Lookup("output")
	if outputFlag == nil {
		t.Error("expected 'output' persistent flag to exist")
	}

	// Check shorthand.
	if outputFlag != nil && outputFlag.Shorthand != "o" {
		t.Errorf("expected output flag shorthand 'o', got %q", outputFlag.Shorthand)
	}
}

// TestTeamCreateCommand_Flags tests create command flags.
func TestTeamCreateCommand_Flags(t *testing.T) {
	cfg := mockTeamConfig()
	deps := createTeamTestDeps(cfg)
	cmd := NewTeamCommand(deps)

	createCmd, _, err := cmd.Find([]string{"create"})
	if err != nil {
		t.Fatalf("failed to find create command: %v", err)
	}

	// Check description flag.
	descFlag := createCmd.Flags().Lookup("description")
	if descFlag == nil {
		t.Error("expected 'description' flag on create command")
	}
}

// TestTeamAddMemberCommand_Flags tests add-member command flags.
func TestTeamAddMemberCommand_Flags(t *testing.T) {
	cfg := mockTeamConfig()
	deps := createTeamTestDeps(cfg)
	cmd := NewTeamCommand(deps)

	addMemberCmd, _, err := cmd.Find([]string{"add-member"})
	if err != nil {
		t.Fatalf("failed to find add-member command: %v", err)
	}

	// Check email flag (required).
	emailFlag := addMemberCmd.Flags().Lookup("email")
	if emailFlag == nil {
		t.Error("expected 'email' flag on add-member command")
	}

	// Check role flag.
	roleFlag := addMemberCmd.Flags().Lookup("role")
	if roleFlag == nil {
		t.Error("expected 'role' flag on add-member command")
	}

	// Check default value for role.
	if roleFlag != nil && roleFlag.DefValue != "member" {
		t.Errorf("expected role default value 'member', got %q", roleFlag.DefValue)
	}
}

// TestTeamDeleteCommand_Flags tests delete command flags.
func TestTeamDeleteCommand_Flags(t *testing.T) {
	cfg := mockTeamConfig()
	deps := createTeamTestDeps(cfg)
	cmd := NewTeamCommand(deps)

	deleteCmd, _, err := cmd.Find([]string{"delete"})
	if err != nil {
		t.Fatalf("failed to find delete command: %v", err)
	}

	// Check force flag.
	forceFlag := deleteCmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("expected 'force' flag on delete command")
	}

	// Check shorthand.
	if forceFlag != nil && forceFlag.Shorthand != "f" {
		t.Errorf("expected force flag shorthand 'f', got %q", forceFlag.Shorthand)
	}
}
