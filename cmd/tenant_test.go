// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// mockConfig creates a mock configuration for testing.
func mockConfig() *config.CLIConfig {
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

// Mock data for tenant tests
func getMockTenants() []*client.Tenant {
	return []*client.Tenant{
		{
			Slug:        "tenant-default-001",
			Name:        "Default Tenant",
			Description: "The default development tenant",
			IsActive:    true,
			CreatedAt:   time.Now().Add(-90 * 24 * time.Hour),
		},
		{
			Slug:        "tenant-acme-002",
			Name:        "Acme Corp",
			Description: "Acme Corporation tenant",
			IsActive:    true,
			CreatedAt:   time.Now().Add(-60 * 24 * time.Hour),
		},
		{
			Slug:        "tenant-demo-003",
			Name:        "Demo Tenant",
			Description: "For demonstration purposes",
			IsActive:    true,
			CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
		},
	}
}

func getMockTenant(tenantRef string) *client.Tenant {
	knownTenants := map[string]*client.Tenant{
		"tenant-default-001": {
			Slug:        "tenant-default-001",
			Name:        "Default Tenant",
			Description: "The default development tenant",
			IsActive:    true,
			CreatedAt:   time.Now().Add(-90 * 24 * time.Hour),
		},
		"tenant-acme-002": {
			Slug:        "tenant-acme-002",
			Name:        "Acme Corp",
			Description: "Acme Corporation tenant",
			IsActive:    true,
			CreatedAt:   time.Now().Add(-60 * 24 * time.Hour),
		},
		"tenant-demo-003": {
			Slug:        "tenant-demo-003",
			Name:        "Demo Tenant",
			Description: "For demonstration purposes",
			IsActive:    true,
			CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
		},
		"tenant-valid-123": {
			Slug:        "tenant-valid-123",
			Name:        "Valid Test Tenant",
			IsActive:    true,
			CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
		},
		"new-tenant-id": {
			Slug:        "new-tenant-id",
			Name:        "New Tenant",
			IsActive:    true,
			CreatedAt:   time.Now(),
		},
		"tenant-acme-resolved": {
			Slug:        "tenant-acme-resolved",
			Name:        "Acme Resolved",
			IsActive:    true,
			CreatedAt:   time.Now().Add(-45 * 24 * time.Hour),
		},
	}

	if tenant, found := knownTenants[tenantRef]; found {
		return tenant
	}

	return &client.Tenant{
		Slug:     tenantRef,
		Name:     "Unknown",
		IsActive: false,
	}
}

func isValidMockTenant(tenantRef string) bool {
	validTenants := map[string]bool{
		"tenant-default-001":   true,
		"tenant-acme-002":      true,
		"tenant-demo-003":      true,
		"tenant-valid-123":     true,
		"new-tenant-id":        true,
		"tenant-acme-resolved": true,
	}
	return validTenants[tenantRef]
}

// createTestDeps creates test dependencies with mock implementations.
func createTestDeps(cfg *config.CLIConfig) *TenantCommandDeps {
	return &TenantCommandDeps{
		Config:       cfg,
		OutputFormat: cfg.OutputFormat,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		SaveConfig: func(c *config.CLIConfig) error {
			// Also update the reference.
			*cfg = *c
			return nil
		},
		InitTenantClient: func(c *config.CLIConfig) (*client.TenantClient, error) {
			// Return a dummy client - the actual methods are mocked via function injection
			return &client.TenantClient{}, nil
		},
		// Mock tenant client methods
		ListTenants: func(ctx context.Context, c *client.TenantClient, req *client.ListTenantsRequest) ([]*client.Tenant, int64, error) {
			tenants := getMockTenants()
			return tenants, int64(len(tenants)), nil
		},
		GetTenant: func(ctx context.Context, c *client.TenantClient, id string, slug string) (*client.Tenant, error) {
			tenantRef := slug
			if tenantRef == "" {
				tenantRef = id
			}
			return getMockTenant(tenantRef), nil
		},
		SetCurrentTenant: func(ctx context.Context, c *client.TenantClient, tenantRef string) (*client.Tenant, bool, string, error) {
			if isValidMockTenant(tenantRef) {
				return getMockTenant(tenantRef), true, "", nil
			}
			return nil, false, "tenant not found or access denied", nil
		},
	}
}

// createTestDepsWithSavedConfig creates test deps that track saved config.
func createTestDepsWithSavedConfig(cfg *config.CLIConfig, savedConfig **config.CLIConfig) *TenantCommandDeps {
	return &TenantCommandDeps{
		Config:       cfg,
		OutputFormat: cfg.OutputFormat,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		SaveConfig: func(c *config.CLIConfig) error {
			cfgCopy := *c
			*savedConfig = &cfgCopy
			return nil
		},
		InitTenantClient: func(c *config.CLIConfig) (*client.TenantClient, error) {
			// Return a dummy client - the actual methods are mocked via function injection
			return &client.TenantClient{}, nil
		},
		// Mock tenant client methods
		ListTenants: func(ctx context.Context, c *client.TenantClient, req *client.ListTenantsRequest) ([]*client.Tenant, int64, error) {
			tenants := getMockTenants()
			return tenants, int64(len(tenants)), nil
		},
		GetTenant: func(ctx context.Context, c *client.TenantClient, id string, slug string) (*client.Tenant, error) {
			tenantRef := slug
			if tenantRef == "" {
				tenantRef = id
			}
			return getMockTenant(tenantRef), nil
		},
		SetCurrentTenant: func(ctx context.Context, c *client.TenantClient, tenantRef string) (*client.Tenant, bool, string, error) {
			if isValidMockTenant(tenantRef) {
				return getMockTenant(tenantRef), true, "", nil
			}
			return nil, false, "tenant not found or access denied", nil
		},
	}
}

func TestNewTenantCommand(t *testing.T) {
	deps := createTestDeps(mockConfig())
	cmd := NewTenantCommand(deps)

	if cmd == nil {
		t.Fatal("NewTenantCommand returned nil")
	}

	if cmd.Use != "tenant" {
		t.Errorf("expected Use to be 'tenant', got %q", cmd.Use)
	}

	// Check subcommands exist.
	subcommands := cmd.Commands()
	expectedSubcmds := []string{"list", "switch", "current", "show"}

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

func TestNewTenantCommand_WithNilDeps(t *testing.T) {
	cmd := NewTenantCommand(nil)

	if cmd == nil {
		t.Fatal("NewTenantCommand with nil deps returned nil")
	}
}

func TestGetCurrentTenantID_FromConfig(t *testing.T) {
	cfg := mockConfig()
	cfg.TenantID = "tenant-from-config"

	// Ensure env var is not set.
	os.Unsetenv("PENF_TENANT_ID")

	result := getCurrentTenantID(cfg)
	if result != "tenant-from-config" {
		t.Errorf("expected 'tenant-from-config', got %q", result)
	}
}

func TestGetCurrentTenantID_FromEnv(t *testing.T) {
	cfg := mockConfig()
	cfg.TenantID = "tenant-from-config"

	// Set env var.
	os.Setenv("PENF_TENANT_ID", "tenant-from-env")
	defer os.Unsetenv("PENF_TENANT_ID")

	result := getCurrentTenantID(cfg)
	if result != "tenant-from-env" {
		t.Errorf("expected 'tenant-from-env', got %q", result)
	}
}

func TestGetCurrentTenantID_EmptyConfig(t *testing.T) {
	cfg := mockConfig()
	cfg.TenantID = ""

	os.Unsetenv("PENF_TENANT_ID")

	result := getCurrentTenantID(cfg)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestResolveTenantAlias_ExistingAlias(t *testing.T) {
	cfg := mockConfig()

	result := resolveTenantAlias(cfg, "work")
	if result != "tenant-acme-002" {
		t.Errorf("expected 'tenant-acme-002', got %q", result)
	}
}

func TestResolveTenantAlias_NotAnAlias(t *testing.T) {
	cfg := mockConfig()

	result := resolveTenantAlias(cfg, "tenant-unknown-999")
	if result != "tenant-unknown-999" {
		t.Errorf("expected 'tenant-unknown-999', got %q", result)
	}
}

func TestResolveTenantAlias_NilAliases(t *testing.T) {
	cfg := mockConfig()
	cfg.TenantAliases = nil

	result := resolveTenantAlias(cfg, "work")
	if result != "work" {
		t.Errorf("expected 'work', got %q", result)
	}
}

func TestFindTenantAlias_ExistingID(t *testing.T) {
	cfg := mockConfig()

	result := findTenantAlias(cfg, "tenant-acme-002")
	if result != "work" {
		t.Errorf("expected 'work', got %q", result)
	}
}

func TestFindTenantAlias_NotFound(t *testing.T) {
	cfg := mockConfig()

	result := findTenantAlias(cfg, "tenant-unknown-999")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFindTenantAlias_NilAliases(t *testing.T) {
	cfg := mockConfig()
	cfg.TenantAliases = nil

	result := findTenantAlias(cfg, "tenant-acme-002")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestValidateTenantAccess_ValidID(t *testing.T) {
	deps := createTestDeps(mockConfig())
	ctx := context.Background()

	err := validateTenantAccess(ctx, deps, "tenant-valid-123")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateTenantAccess_EmptyID(t *testing.T) {
	deps := createTestDeps(mockConfig())
	ctx := context.Background()

	err := validateTenantAccess(ctx, deps, "")
	if err == nil {
		t.Error("expected error for empty tenant ID, got nil")
	}
}

func TestValidateTenantAccess_InvalidChars(t *testing.T) {
	deps := createTestDeps(mockConfig())
	ctx := context.Background()

	err := validateTenantAccess(ctx, deps, "tenant with spaces")
	if err == nil {
		t.Error("expected error for tenant ID with spaces, got nil")
	}
}

func TestGetMockTenantList(t *testing.T) {
	tenants := getMockTenantList("tenant-default-001")

	if len(tenants) == 0 {
		t.Fatal("expected non-empty tenant list")
	}

	// Check that current tenant is marked.
	var currentFound bool
	for _, tenant := range tenants {
		if tenant.ID == "tenant-default-001" && tenant.IsCurrent {
			currentFound = true
			break
		}
	}
	if !currentFound {
		t.Error("expected tenant-default-001 to be marked as current")
	}
}

func TestGetMockTenantInfo_KnownTenant(t *testing.T) {
	info := getMockTenantInfo("tenant-default-001")

	if info.ID != "tenant-default-001" {
		t.Errorf("expected ID 'tenant-default-001', got %q", info.ID)
	}
	if info.Name == "" {
		t.Error("expected non-empty name")
	}
	if info.Status != "active" {
		t.Errorf("expected status 'active', got %q", info.Status)
	}
}

func TestGetMockTenantInfo_UnknownTenant(t *testing.T) {
	info := getMockTenantInfo("tenant-unknown-999")

	if info.ID != "tenant-unknown-999" {
		t.Errorf("expected ID 'tenant-unknown-999', got %q", info.ID)
	}
	if info.Status != "unknown" {
		t.Errorf("expected status 'unknown', got %q", info.Status)
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q",
				tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestTenantInfo_JSONOutput(t *testing.T) {
	info := TenantInfo{
		ID:          "tenant-test-001",
		Name:        "Test Tenant",
		Description: "A test tenant",
		Status:      "active",
		Role:        "admin",
		IsCurrent:   true,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal TenantInfo: %v", err)
	}

	var decoded TenantInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TenantInfo: %v", err)
	}

	if decoded.ID != info.ID {
		t.Errorf("expected ID %q, got %q", info.ID, decoded.ID)
	}
	if decoded.Name != info.Name {
		t.Errorf("expected Name %q, got %q", info.Name, decoded.Name)
	}
	if decoded.IsCurrent != info.IsCurrent {
		t.Errorf("expected IsCurrent %v, got %v", info.IsCurrent, decoded.IsCurrent)
	}
}

func TestTenantInfo_YAMLOutput(t *testing.T) {
	info := TenantInfo{
		ID:          "tenant-test-001",
		Name:        "Test Tenant",
		Description: "A test tenant",
		Status:      "active",
		Role:        "admin",
		IsCurrent:   true,
	}

	data, err := yaml.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal TenantInfo: %v", err)
	}

	var decoded TenantInfo
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TenantInfo: %v", err)
	}

	if decoded.ID != info.ID {
		t.Errorf("expected ID %q, got %q", info.ID, decoded.ID)
	}
	if decoded.Name != info.Name {
		t.Errorf("expected Name %q, got %q", info.Name, decoded.Name)
	}
}

func TestTenantListResponse_JSONOutput(t *testing.T) {
	response := TenantListResponse{
		Tenants: []TenantInfo{
			{ID: "tenant-1", Name: "Tenant 1", Status: "active"},
			{ID: "tenant-2", Name: "Tenant 2", Status: "active"},
		},
		CurrentID:  "tenant-1",
		TotalCount: 2,
		FetchedAt:  time.Now(),
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal TenantListResponse: %v", err)
	}

	var decoded TenantListResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TenantListResponse: %v", err)
	}

	if len(decoded.Tenants) != 2 {
		t.Errorf("expected 2 tenants, got %d", len(decoded.Tenants))
	}
	if decoded.CurrentID != "tenant-1" {
		t.Errorf("expected CurrentID 'tenant-1', got %q", decoded.CurrentID)
	}
	if decoded.TotalCount != 2 {
		t.Errorf("expected TotalCount 2, got %d", decoded.TotalCount)
	}
}

func TestOutputTenantList_Text(t *testing.T) {
	response := TenantListResponse{
		Tenants: []TenantInfo{
			{ID: "tenant-1", Name: "Tenant 1", Status: "active", Role: "admin", IsCurrent: true},
			{ID: "tenant-2", Name: "Tenant 2", Status: "active", Role: "member"},
		},
		CurrentID:  "tenant-1",
		TotalCount: 2,
		FetchedAt:  time.Now(),
	}

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputTenantList(config.OutputFormatText, response)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputTenantList failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check that output contains expected content.
	if !strings.Contains(output, "tenant-1") {
		t.Error("output should contain tenant-1")
	}
	if !strings.Contains(output, "Tenant 1") {
		t.Error("output should contain Tenant 1")
	}
	if !strings.Contains(output, "*") {
		t.Error("output should contain current marker (*)")
	}
}

func TestOutputTenantList_EmptyList(t *testing.T) {
	response := TenantListResponse{
		Tenants:    []TenantInfo{},
		TotalCount: 0,
		FetchedAt:  time.Now(),
	}

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputTenantList(config.OutputFormatText, response)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputTenantList failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No tenants found") {
		t.Error("output should indicate no tenants found")
	}
}

func TestOutputTenantInfo_Text(t *testing.T) {
	info := TenantInfo{
		ID:          "tenant-test-001",
		Name:        "Test Tenant",
		Description: "A test tenant for testing",
		Status:      "active",
		Role:        "admin",
		IsCurrent:   true,
		CreatedAt:   time.Now().AddDate(-1, 0, 0),
	}

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputTenantDetail(config.OutputFormatText, info)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputTenantDetail failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check that output contains expected content.
	if !strings.Contains(output, "tenant-test-001") {
		t.Error("output should contain tenant ID")
	}
	if !strings.Contains(output, "Test Tenant") {
		t.Error("output should contain tenant name")
	}
	if !strings.Contains(output, "admin") {
		t.Error("output should contain role")
	}
	if !strings.Contains(output, "active") {
		t.Error("output should contain status")
	}
}

func TestAddTenantMetadata(t *testing.T) {
	ctx := context.Background()

	// Add tenant metadata.
	ctxWithTenant := AddTenantMetadata(ctx, "tenant-test-123")

	// The context should be different (has metadata).
	if ctxWithTenant == ctx {
		t.Error("expected different context with metadata")
	}
}

func TestAddTenantMetadata_EmptyID(t *testing.T) {
	ctx := context.Background()

	// Add empty tenant metadata should return same context.
	ctxWithTenant := AddTenantMetadata(ctx, "")

	if ctxWithTenant != ctx {
		t.Error("expected same context for empty tenant ID")
	}
}

func TestRunTenantList(t *testing.T) {
	cfg := mockConfig()
	deps := createTestDeps(cfg)

	// Capture stdout.
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runTenantList(context.Background(), deps, false, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runTenantList failed: %v", err)
	}
}

func TestRunTenantCurrent_WithTenant(t *testing.T) {
	cfg := mockConfig()
	cfg.TenantID = "tenant-current-test"
	deps := createTestDeps(cfg)

	os.Unsetenv("PENF_TENANT_ID")

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runTenantCurrent(deps, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runTenantCurrent failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "tenant-current-test") {
		t.Error("output should contain current tenant ID")
	}
}

func TestRunTenantCurrent_NoTenant(t *testing.T) {
	cfg := mockConfig()
	cfg.TenantID = ""
	deps := createTestDeps(cfg)

	os.Unsetenv("PENF_TENANT_ID")

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runTenantCurrent(deps, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runTenantCurrent failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No tenant configured") {
		t.Error("output should indicate no tenant configured")
	}
}

func TestRunTenantSwitch(t *testing.T) {
	cfg := mockConfig()
	cfg.TenantID = "old-tenant"

	var savedConfig *config.CLIConfig
	deps := createTestDepsWithSavedConfig(cfg, &savedConfig)

	os.Unsetenv("PENF_TENANT_ID")

	// Capture stdout.
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runTenantSwitch(context.Background(), deps, "new-tenant-id", true, false)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runTenantSwitch failed: %v", err)
	}

	if savedConfig == nil {
		t.Fatal("expected config to be saved")
	}

	if savedConfig.TenantID != "new-tenant-id" {
		t.Errorf("expected TenantID 'new-tenant-id', got %q", savedConfig.TenantID)
	}
}

func TestRunTenantSwitch_WithAlias(t *testing.T) {
	cfg := mockConfig()
	cfg.TenantID = "old-tenant"
	cfg.TenantAliases = map[string]string{
		"work": "tenant-acme-resolved",
	}

	var savedConfig *config.CLIConfig
	deps := createTestDepsWithSavedConfig(cfg, &savedConfig)

	os.Unsetenv("PENF_TENANT_ID")

	// Capture stdout.
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runTenantSwitch(context.Background(), deps, "work", true, false)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runTenantSwitch failed: %v", err)
	}

	if savedConfig == nil {
		t.Fatal("expected config to be saved")
	}

	// Should resolve the alias to the actual tenant ID.
	if savedConfig.TenantID != "tenant-acme-resolved" {
		t.Errorf("expected TenantID 'tenant-acme-resolved', got %q", savedConfig.TenantID)
	}
}

func TestRunTenantSwitch_InvalidID(t *testing.T) {
	cfg := mockConfig()
	deps := createTestDeps(cfg)

	os.Unsetenv("PENF_TENANT_ID")

	// Capture stdout/stderr.
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runTenantSwitch(context.Background(), deps, "", true, false)

	w.Close()
	os.Stdout = oldStdout

	if err == nil {
		t.Error("expected error for empty tenant ID")
	}
}

func TestRunTenantSwitch_NoValidation(t *testing.T) {
	cfg := mockConfig()
	cfg.TenantID = "old-tenant"

	var savedConfig *config.CLIConfig
	deps := createTestDepsWithSavedConfig(cfg, &savedConfig)

	os.Unsetenv("PENF_TENANT_ID")

	// Capture stdout.
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	// Switch with no validation - should work even with invalid format.
	err := runTenantSwitch(context.Background(), deps, "any-tenant", false, false)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runTenantSwitch with no validation failed: %v", err)
	}
}

func TestRunTenantInfo_CurrentTenant(t *testing.T) {
	cfg := mockConfig()
	cfg.TenantID = "tenant-default-001"
	deps := createTestDeps(cfg)

	os.Unsetenv("PENF_TENANT_ID")

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call with empty string to use current tenant.
	err := runTenantShow(context.Background(), deps, "", false, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runTenantShow failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "tenant-default-001") {
		t.Error("output should contain current tenant ID")
	}
}

func TestRunTenantInfo_SpecificTenant(t *testing.T) {
	cfg := mockConfig()
	deps := createTestDeps(cfg)

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runTenantShow(context.Background(), deps, "tenant-acme-002", false, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runTenantShow failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "tenant-acme-002") {
		t.Error("output should contain requested tenant ID")
	}
}

func TestRunTenantInfo_NoCurrentTenant(t *testing.T) {
	cfg := mockConfig()
	cfg.TenantID = ""
	deps := createTestDeps(cfg)

	os.Unsetenv("PENF_TENANT_ID")

	err := runTenantShow(context.Background(), deps, "", false, nil)
	if err == nil {
		t.Error("expected error when no current tenant and no argument")
	}
}

func TestTenantCommand_ListAliases(t *testing.T) {
	deps := createTestDeps(mockConfig())
	cmd := NewTenantCommand(deps)

	// Find the list command.
	listCmd, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("failed to find list command: %v", err)
	}

	aliases := listCmd.Aliases
	if len(aliases) == 0 {
		t.Error("list command should have aliases")
	}

	hasAlias := false
	for _, a := range aliases {
		if a == "ls" {
			hasAlias = true
			break
		}
	}
	if !hasAlias {
		t.Error("list command should have 'ls' alias")
	}
}

func TestTenantCommand_SwitchAliases(t *testing.T) {
	deps := createTestDeps(mockConfig())
	cmd := NewTenantCommand(deps)

	// Find the switch command.
	switchCmd, _, err := cmd.Find([]string{"switch"})
	if err != nil {
		t.Fatalf("failed to find switch command: %v", err)
	}

	aliases := switchCmd.Aliases
	expectedAliases := []string{"use", "sw"}

	for _, expected := range expectedAliases {
		found := false
		for _, a := range aliases {
			if a == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("switch command should have %q alias", expected)
		}
	}
}

func TestTenantCommand_CurrentAliases(t *testing.T) {
	deps := createTestDeps(mockConfig())
	cmd := NewTenantCommand(deps)

	// Find the current command.
	currentCmd, _, err := cmd.Find([]string{"current"})
	if err != nil {
		t.Fatalf("failed to find current command: %v", err)
	}

	hasWhoami := false
	for _, a := range currentCmd.Aliases {
		if a == "whoami" {
			hasWhoami = true
			break
		}
	}
	if !hasWhoami {
		t.Error("current command should have 'whoami' alias")
	}
}

// Mock helper functions for tenant tests.

// getMockTenantList returns a list of mock tenants, marking the current one.
func getMockTenantList(currentID string) []TenantInfo {
	tenants := []TenantInfo{
		{
			ID:          "tenant-default-001",
			Name:        "Default Tenant",
			Description: "The default development tenant",
			CreatedAt:   time.Now().Add(-90 * 24 * time.Hour),
			Status:      "active",
			Role:        "admin",
			IsCurrent:   false,
		},
		{
			ID:          "tenant-acme-002",
			Name:        "Acme Corp",
			Description: "Acme Corporation tenant",
			CreatedAt:   time.Now().Add(-60 * 24 * time.Hour),
			Status:      "active",
			Role:        "member",
			IsCurrent:   false,
		},
		{
			ID:          "tenant-demo-003",
			Name:        "Demo Tenant",
			Description: "For demonstration purposes",
			CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
			Status:      "active",
			Role:        "viewer",
			IsCurrent:   false,
		},
	}

	// Mark the current tenant.
	for i := range tenants {
		if tenants[i].ID == currentID {
			tenants[i].IsCurrent = true
			break
		}
	}

	return tenants
}

// getMockTenantInfo returns mock tenant information for a given ID.
func getMockTenantInfo(id string) TenantInfo {
	knownTenants := map[string]TenantInfo{
		"tenant-default-001": {
			ID:          "tenant-default-001",
			Name:        "Default Tenant",
			Description: "The default development tenant",
			CreatedAt:   time.Now().Add(-90 * 24 * time.Hour),
			Status:      "active",
			Role:        "admin",
			IsCurrent:   true,
		},
		"tenant-acme-002": {
			ID:          "tenant-acme-002",
			Name:        "Acme Corp",
			Description: "Acme Corporation tenant",
			CreatedAt:   time.Now().Add(-60 * 24 * time.Hour),
			Status:      "active",
			Role:        "member",
			IsCurrent:   false,
		},
		"tenant-demo-003": {
			ID:          "tenant-demo-003",
			Name:        "Demo Tenant",
			Description: "For demonstration purposes",
			CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
			Status:      "active",
			Role:        "viewer",
			IsCurrent:   false,
		},
	}

	if info, found := knownTenants[id]; found {
		return info
	}

	// Return unknown tenant info for unrecognized IDs.
	return TenantInfo{
		ID:          id,
		Name:        "Unknown",
		Description: "",
		Status:      "unknown",
		Role:        "",
		IsCurrent:   false,
	}
}
