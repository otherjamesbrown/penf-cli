// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"testing"
	"time"

	"github.com/otherjamesbrown/penf-cli/config"
)

// projectMockConfig creates a test configuration for project tests.
func projectMockConfig() *config.CLIConfig {
	return &config.CLIConfig{
		ServerAddress: "localhost:50051",
		TenantID:      "00000001-0000-0000-0000-000000000001",
		OutputFormat:  config.OutputFormatText,
		Timeout:       30 * time.Second,
	}
}

// createProjectTestDeps creates test dependencies with mock implementations.
func createProjectTestDeps(cfg *config.CLIConfig) *ProjectCommandDeps {
	return &ProjectCommandDeps{
		Config: cfg,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
	}
}

// ==================== Command Structure Tests ====================

func TestNewProjectCommand(t *testing.T) {
	deps := createProjectTestDeps(projectMockConfig())
	cmd := NewProjectCommand(deps)

	if cmd == nil {
		t.Fatal("NewProjectCommand returned nil")
	}

	if cmd.Use != "project" {
		t.Errorf("expected Use to be 'project', got %q", cmd.Use)
	}

	// Check aliases.
	expectedAliases := []string{"proj", "projects"}
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

func TestNewProjectCommand_WithNilDeps(t *testing.T) {
	cmd := NewProjectCommand(nil)

	if cmd == nil {
		t.Fatal("NewProjectCommand with nil deps returned nil")
	}
}

func TestNewProjectCommand_Subcommands(t *testing.T) {
	deps := createProjectTestDeps(projectMockConfig())
	cmd := NewProjectCommand(deps)

	// Check subcommands exist.
	subcommands := []string{"list", "add", "show", "delete"}
	for _, sub := range subcommands {
		found := false
		for _, c := range cmd.Commands() {
			if c.Name() == sub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q to exist", sub)
		}
	}
}

// ==================== Project List Command Tests ====================

func TestProjectListCommand_Structure(t *testing.T) {
	deps := createProjectTestDeps(projectMockConfig())
	cmd := NewProjectCommand(deps)

	listCmd, _, _ := cmd.Find([]string{"list"})
	if listCmd == nil {
		t.Fatal("list subcommand not found")
	}

	// Check flags.
	flags := []string{"name", "keyword"}
	for _, flag := range flags {
		if listCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %q to exist on list command", flag)
		}
	}

	// Check alias.
	if len(listCmd.Aliases) == 0 || listCmd.Aliases[0] != "ls" {
		t.Error("expected list command to have 'ls' alias")
	}
}

// ==================== Project Add Command Tests ====================

func TestProjectAddCommand_Structure(t *testing.T) {
	deps := createProjectTestDeps(projectMockConfig())
	cmd := NewProjectCommand(deps)

	addCmd, _, _ := cmd.Find([]string{"add"})
	if addCmd == nil {
		t.Fatal("add subcommand not found")
	}

	// Check flags.
	flags := []string{"description", "keywords"}
	for _, flag := range flags {
		if addCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %q to exist on add command", flag)
		}
	}

	// Check requires exactly 1 argument.
	if addCmd.Args == nil {
		t.Error("expected add command to have args validation")
	}
}

// ==================== Project Show Command Tests ====================

func TestProjectShowCommand_Structure(t *testing.T) {
	deps := createProjectTestDeps(projectMockConfig())
	cmd := NewProjectCommand(deps)

	showCmd, _, _ := cmd.Find([]string{"show"})
	if showCmd == nil {
		t.Fatal("show subcommand not found")
	}

	// Check requires exactly 1 argument.
	if showCmd.Args == nil {
		t.Error("expected show command to have args validation")
	}

	// Check "info" alias still works for backward compatibility.
	infoCmd, _, _ := cmd.Find([]string{"info"})
	if infoCmd == nil {
		t.Fatal("info alias not found (backward compatibility)")
	}

	// Check "get" alias works.
	getCmd, _, _ := cmd.Find([]string{"get"})
	if getCmd == nil {
		t.Fatal("get alias not found")
	}
}

// ==================== Project Delete Command Tests ====================

func TestProjectDeleteCommand_Structure(t *testing.T) {
	deps := createProjectTestDeps(projectMockConfig())
	cmd := NewProjectCommand(deps)

	deleteCmd, _, _ := cmd.Find([]string{"delete"})
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	// Check requires exactly 1 argument.
	if deleteCmd.Args == nil {
		t.Error("expected delete command to have args validation")
	}

	// Check --force flag.
	forceFlag := deleteCmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("expected --force flag to exist on delete command")
	}

	// Check "rm" alias works.
	rmCmd, _, _ := cmd.Find([]string{"rm"})
	if rmCmd == nil {
		t.Fatal("rm alias not found")
	}

	// Check "remove" alias works.
	removeCmd, _, _ := cmd.Find([]string{"remove"})
	if removeCmd == nil {
		t.Fatal("remove alias not found")
	}
}

// ==================== Output Format Tests ====================

func TestGetProjectOutputFormat(t *testing.T) {
	tests := []struct {
		name           string
		formatFlag     string
		configFormat   config.OutputFormat
		expectedResult config.OutputFormat
	}{
		{
			name:           "flag takes precedence",
			formatFlag:     "json",
			configFormat:   config.OutputFormatText,
			expectedResult: config.OutputFormatJSON,
		},
		{
			name:           "config used when flag empty",
			formatFlag:     "",
			configFormat:   config.OutputFormatYAML,
			expectedResult: config.OutputFormatYAML,
		},
		{
			name:           "config format returned when both empty",
			formatFlag:     "",
			configFormat:   "",
			expectedResult: "", // Returns empty config format, not default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore global flag.
			oldFormat := projectOutput
			defer func() { projectOutput = oldFormat }()

			projectOutput = tt.formatFlag

			cfg := &config.CLIConfig{OutputFormat: tt.configFormat}
			result := getProjectOutputFormat(cfg)

			if result != tt.expectedResult {
				t.Errorf("getProjectOutputFormat() = %q, want %q", result, tt.expectedResult)
			}
		})
	}
}

func TestGetProjectOutputFormat_NilConfig(t *testing.T) {
	// Save and restore global flag.
	oldFormat := projectOutput
	defer func() { projectOutput = oldFormat }()

	projectOutput = ""

	result := getProjectOutputFormat(nil)
	if result != config.OutputFormatText {
		t.Errorf("getProjectOutputFormat(nil) = %q, want %q", result, config.OutputFormatText)
	}
}

// ==================== Tenant ID Resolution Tests ====================

func TestGetTenantIDForProject(t *testing.T) {
	tests := []struct {
		name       string
		tenantFlag string
		envTenant  string
		cfgTenant  string
		expected   string
	}{
		{
			name:       "flag takes precedence",
			tenantFlag: "flag-tenant",
			envTenant:  "env-tenant",
			cfgTenant:  "cfg-tenant",
			expected:   "flag-tenant",
		},
		{
			name:       "env used when flag empty",
			tenantFlag: "",
			envTenant:  "env-tenant",
			cfgTenant:  "cfg-tenant",
			expected:   "env-tenant",
		},
		{
			name:       "config used when flag and env empty",
			tenantFlag: "",
			envTenant:  "",
			cfgTenant:  "cfg-tenant",
			expected:   "cfg-tenant",
		},
		{
			name:       "default when all empty",
			tenantFlag: "",
			envTenant:  "",
			cfgTenant:  "",
			expected:   "00000001-0000-0000-0000-000000000001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore global flag.
			oldTenant := projectTenant
			defer func() { projectTenant = oldTenant }()

			projectTenant = tt.tenantFlag

			// Set env var if needed.
			if tt.envTenant != "" {
				t.Setenv("PENF_TENANT_ID", tt.envTenant)
			}

			deps := &ProjectCommandDeps{
				Config: &config.CLIConfig{TenantID: tt.cfgTenant},
			}

			result := getTenantIDForProject(deps)
			if result != tt.expected {
				t.Errorf("getTenantIDForProject() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ==================== String Utility Tests ====================

func TestProjectTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"hi", 5, "hi"},
		{"abc", 3, "abc"},
		{"abcdef", 3, "abc"}, // maxLen <= 3 case
		{"", 10, ""},
	}

	for _, tt := range tests {
		result := projectTruncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("projectTruncateString(%q, %d) = %q, want %q",
				tt.input, tt.maxLen, result, tt.expected)
		}
	}
}
