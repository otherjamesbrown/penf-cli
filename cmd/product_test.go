// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/otherjamesbrown/penf-cli/config"
	"github.com/otherjamesbrown/penf-cli/pkg/products"
)

// productMockConfig creates a test configuration for product tests.
func productMockConfig() *config.CLIConfig {
	return &config.CLIConfig{
		ServerAddress: "localhost:50051",
		TenantID:      "00000001-0000-0000-0000-000000000001",
		OutputFormat:  config.OutputFormatText,
		Timeout:       30 * time.Second,
	}
}

// createProductTestDeps creates test dependencies with mock implementations.
func createProductTestDeps(cfg *config.CLIConfig) *ProductCommandDeps {
	return &ProductCommandDeps{
		Config: cfg,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		InitPool: func(c *config.CLIConfig) (*pgxpool.Pool, error) {
			// Return nil pool for unit tests (won't execute actual commands)
			return nil, nil
		},
	}
}

// ==================== Command Structure Tests ====================

func TestNewProductCommand(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	if cmd == nil {
		t.Fatal("NewProductCommand returned nil")
	}

	if cmd.Use != "product" {
		t.Errorf("expected Use to be 'product', got %q", cmd.Use)
	}

	// Check aliases.
	expectedAliases := []string{"prod", "products"}
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

func TestNewProductCommand_WithNilDeps(t *testing.T) {
	cmd := NewProductCommand(nil)

	if cmd == nil {
		t.Fatal("NewProductCommand with nil deps returned nil")
	}
}

func TestNewProductCommand_Subcommands(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	// Check subcommands exist.
	subcommands := []string{"list", "add", "show", "hierarchy", "alias", "team", "timeline", "event", "query"}
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

// ==================== Product List Command Tests ====================

func TestProductListCommand_Structure(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	listCmd, _, _ := cmd.Find([]string{"list"})
	if listCmd == nil {
		t.Fatal("list subcommand not found")
	}

	// Check flags.
	flags := []string{"parent", "type", "status", "all"}
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

// ==================== Product Add Command Tests ====================

func TestProductAddCommand_Structure(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	addCmd, _, _ := cmd.Find([]string{"add"})
	if addCmd == nil {
		t.Fatal("add subcommand not found")
	}

	// Check flags.
	flags := []string{"parent", "type", "status", "description", "keywords"}
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

// ==================== Product Show Command Tests ====================

func TestProductShowCommand_Structure(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

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
}

// ==================== Product Hierarchy Command Tests ====================

func TestProductHierarchyCommand_Structure(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	hierarchyCmd, _, _ := cmd.Find([]string{"hierarchy"})
	if hierarchyCmd == nil {
		t.Fatal("hierarchy subcommand not found")
	}

	// Check alias.
	if len(hierarchyCmd.Aliases) == 0 || hierarchyCmd.Aliases[0] != "tree" {
		t.Error("expected hierarchy command to have 'tree' alias")
	}

	// Check requires exactly 1 argument.
	if hierarchyCmd.Args == nil {
		t.Error("expected hierarchy command to have args validation")
	}
}

// ==================== Product Alias Command Tests ====================

func TestProductAliasCommand_Structure(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	aliasCmd, _, _ := cmd.Find([]string{"alias"})
	if aliasCmd == nil {
		t.Fatal("alias subcommand not found")
	}

	// Check subcommands.
	subcommands := []string{"add", "remove", "list"}
	for _, sub := range subcommands {
		found := false
		for _, c := range aliasCmd.Commands() {
			if c.Name() == sub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected alias subcommand %q to exist", sub)
		}
	}
}

// ==================== Product Team Command Tests ====================

func TestProductTeamCommand_Structure(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	teamCmd, _, _ := cmd.Find([]string{"team"})
	if teamCmd == nil {
		t.Fatal("team subcommand not found")
	}

	// Check subcommands (team command itself acts as list).
	subcommands := []string{"add", "remove", "role", "people"}
	for _, sub := range subcommands {
		found := false
		for _, c := range teamCmd.Commands() {
			if c.Name() == sub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected team subcommand %q to exist", sub)
		}
	}
}

func TestProductTeamRoleCommand_Structure(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	roleCmd, _, _ := cmd.Find([]string{"team", "role"})
	if roleCmd == nil {
		t.Fatal("team role subcommand not found")
	}

	// Check role subcommands (role uses "end" instead of "remove").
	subcommands := []string{"add", "end", "list", "find"}
	for _, sub := range subcommands {
		found := false
		for _, c := range roleCmd.Commands() {
			if c.Name() == sub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected role subcommand %q to exist", sub)
		}
	}
}

// ==================== Product Timeline Command Tests ====================

func TestProductTimelineCommand_Structure(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	timelineCmd, _, _ := cmd.Find([]string{"timeline"})
	if timelineCmd == nil {
		t.Fatal("timeline subcommand not found")
	}

	// Check flags.
	flags := []string{"type", "visibility", "since", "until", "limit"}
	for _, flag := range flags {
		if timelineCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %q to exist on timeline command", flag)
		}
	}
}

// ==================== Product Event Command Tests ====================

func TestProductEventCommand_Structure(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	eventCmd, _, _ := cmd.Find([]string{"event"})
	if eventCmd == nil {
		t.Fatal("event subcommand not found")
	}

	// Check subcommands.
	subcommands := []string{"add", "show", "delete", "link", "context"}
	for _, sub := range subcommands {
		found := false
		for _, c := range eventCmd.Commands() {
			if c.Name() == sub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected event subcommand %q to exist", sub)
		}
	}
}

func TestProductEventAddCommand_Structure(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	addCmd, _, _ := cmd.Find([]string{"event", "add"})
	if addCmd == nil {
		t.Fatal("event add subcommand not found")
	}

	// Check flags.
	flags := []string{"type", "visibility", "title", "description", "occurred", "recorded-by"}
	for _, flag := range flags {
		if addCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %q to exist on event add command", flag)
		}
	}
}

func TestProductEventContextCommand_Structure(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	contextCmd, _, _ := cmd.Find([]string{"event", "context"})
	if contextCmd == nil {
		t.Fatal("event context subcommand not found")
	}

	// Check flags.
	if contextCmd.Flags().Lookup("window") == nil {
		t.Error("expected flag 'window' to exist on event context command")
	}
}

// ==================== Product Query Command Tests ====================

func TestProductQueryCommand_Structure(t *testing.T) {
	deps := createProductTestDeps(productMockConfig())
	cmd := NewProductCommand(deps)

	queryCmd, _, _ := cmd.Find([]string{"query"})
	if queryCmd == nil {
		t.Fatal("query subcommand not found")
	}

	// Check flags.
	if queryCmd.Flags().Lookup("output") == nil {
		t.Error("expected flag 'output' to exist on query command")
	}
}

// ==================== Type Validation Tests ====================

func TestProductType_Valid(t *testing.T) {
	validTypes := []products.ProductType{
		products.ProductTypeProduct,
		products.ProductTypeSubProduct,
		products.ProductTypeFeature,
	}

	for _, pt := range validTypes {
		if pt == "" {
			t.Errorf("expected product type %v to be non-empty", pt)
		}
	}
}

func TestProductStatus_Valid(t *testing.T) {
	validStatuses := []products.ProductStatus{
		products.ProductStatusActive,
		products.ProductStatusBeta,
		products.ProductStatusSunset,
		products.ProductStatusDeprecated,
	}

	for _, ps := range validStatuses {
		if ps == "" {
			t.Errorf("expected product status %v to be non-empty", ps)
		}
	}
}

// ==================== Color Helper Tests ====================

func TestGetProductTypeColor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"product", "\033[35m"},     // Magenta
		{"sub_product", "\033[36m"}, // Cyan
		{"feature", "\033[34m"},     // Blue
		{"unknown", ""},
	}

	for _, tt := range tests {
		result := getProductTypeColor(tt.input)
		if result != tt.expected {
			t.Errorf("getProductTypeColor(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetProductStatusColor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"active", "\033[32m"},     // Green
		{"beta", "\033[33m"},       // Yellow
		{"sunset", "\033[31m"},     // Red
		{"deprecated", "\033[90m"}, // Gray
		{"unknown", ""},
	}

	for _, tt := range tests {
		result := getProductStatusColor(tt.input)
		if result != tt.expected {
			t.Errorf("getProductStatusColor(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// ==================== Output Format Tests ====================

func TestGetProductOutputFormat(t *testing.T) {
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
			oldFormat := productOutput
			defer func() { productOutput = oldFormat }()

			productOutput = tt.formatFlag

			cfg := &config.CLIConfig{OutputFormat: tt.configFormat}
			result := getProductOutputFormat(cfg)

			if result != tt.expectedResult {
				t.Errorf("getProductOutputFormat() = %q, want %q", result, tt.expectedResult)
			}
		})
	}
}

func TestGetProductOutputFormat_NilConfig(t *testing.T) {
	// Save and restore global flag.
	oldFormat := productOutput
	defer func() { productOutput = oldFormat }()

	productOutput = ""

	result := getProductOutputFormat(nil)
	if result != config.OutputFormatText {
		t.Errorf("getProductOutputFormat(nil) = %q, want %q", result, config.OutputFormatText)
	}
}

// ==================== Tenant ID Resolution Tests ====================

func TestGetTenantIDForProduct(t *testing.T) {
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
			oldTenant := productTenant
			defer func() { productTenant = oldTenant }()

			productTenant = tt.tenantFlag

			// Set env var if needed.
			if tt.envTenant != "" {
				t.Setenv("PENF_TENANT_ID", tt.envTenant)
			}

			deps := &ProductCommandDeps{
				Config: &config.CLIConfig{TenantID: tt.cfgTenant},
			}

			result := getTenantIDForProduct(deps)
			if result != tt.expected {
				t.Errorf("getTenantIDForProduct() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ==================== Event Type Constants Tests ====================

func TestEventTypeConstants(t *testing.T) {
	expectedTypes := map[products.EventType]string{
		products.EventTypeDecision:   "decision",
		products.EventTypeMilestone:  "milestone",
		products.EventTypeRisk:       "risk",
		products.EventTypeRelease:    "release",
		products.EventTypeCompetitor: "competitor",
		products.EventTypeOrgChange:  "org_change",
		products.EventTypeMarket:     "market",
		products.EventTypeNote:       "note",
	}

	for et, expected := range expectedTypes {
		if string(et) != expected {
			t.Errorf("EventType constant %v = %q, want %q", et, string(et), expected)
		}
	}
}

func TestEventVisibilityConstants(t *testing.T) {
	expectedVisibilities := map[products.EventVisibility]string{
		products.EventVisibilityInternal: "internal",
		products.EventVisibilityExternal: "external",
	}

	for ev, expected := range expectedVisibilities {
		if string(ev) != expected {
			t.Errorf("EventVisibility constant %v = %q, want %q", ev, string(ev), expected)
		}
	}
}
