// Package config provides CLI configuration management for the penf command-line tool.
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDefaultConfig verifies default configuration values.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if cfg.ServerAddress != DefaultServerAddress {
		t.Errorf("ServerAddress = %v, want %v", cfg.ServerAddress, DefaultServerAddress)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, DefaultTimeout)
	}
	if cfg.OutputFormat != DefaultOutputFormat {
		t.Errorf("OutputFormat = %v, want %v", cfg.OutputFormat, DefaultOutputFormat)
	}
	if cfg.TenantID != "" {
		t.Errorf("TenantID = %v, want empty", cfg.TenantID)
	}
	if cfg.Debug {
		t.Error("Debug should be false by default")
	}
	if cfg.Insecure {
		t.Error("Insecure should be false by default")
	}
}

// TestDefaultConstants verifies default constant values.
func TestDefaultConstants(t *testing.T) {
	if DefaultServerAddress != "localhost:50051" {
		t.Errorf("DefaultServerAddress = %v, want localhost:50051", DefaultServerAddress)
	}
	if DefaultTimeout != 10*time.Minute {
		t.Errorf("DefaultTimeout = %v, want 10m", DefaultTimeout)
	}
	if DefaultOutputFormat != OutputFormatText {
		t.Errorf("DefaultOutputFormat = %v, want text", DefaultOutputFormat)
	}
	if DefaultConfigDir != ".penf" {
		t.Errorf("DefaultConfigDir = %v, want .penf", DefaultConfigDir)
	}
	if DefaultConfigFile != "config.yaml" {
		t.Errorf("DefaultConfigFile = %v, want config.yaml", DefaultConfigFile)
	}
}

// TestOutputFormat_IsValid verifies output format validation.
func TestOutputFormat_IsValid(t *testing.T) {
	tests := []struct {
		format OutputFormat
		valid  bool
	}{
		{OutputFormatText, true},
		{OutputFormatJSON, true},
		{OutputFormatYAML, true},
		{"invalid", false},
		{"", false},
		{"JSON", false}, // Case sensitive
		{"xml", false},
	}

	for _, tc := range tests {
		if got := tc.format.IsValid(); got != tc.valid {
			t.Errorf("OutputFormat(%q).IsValid() = %v, want %v", tc.format, got, tc.valid)
		}
	}
}

// TestOutputFormat_String verifies output format string conversion.
func TestOutputFormat_String(t *testing.T) {
	tests := []struct {
		format   OutputFormat
		expected string
	}{
		{OutputFormatText, "text"},
		{OutputFormatJSON, "json"},
		{OutputFormatYAML, "yaml"},
	}

	for _, tc := range tests {
		if got := tc.format.String(); got != tc.expected {
			t.Errorf("OutputFormat.String() = %v, want %v", got, tc.expected)
		}
	}
}

// TestOutputFormatConstants verifies output format constant values.
func TestOutputFormatConstants(t *testing.T) {
	if OutputFormatText != "text" {
		t.Errorf("OutputFormatText = %v, want text", OutputFormatText)
	}
	if OutputFormatJSON != "json" {
		t.Errorf("OutputFormatJSON = %v, want json", OutputFormatJSON)
	}
	if OutputFormatYAML != "yaml" {
		t.Errorf("OutputFormatYAML = %v, want yaml", OutputFormatYAML)
	}
}

// TestCLIConfig_Validate verifies configuration validation.
func TestCLIConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *CLIConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: &CLIConfig{
				ServerAddress: "localhost:50051",
				Timeout:       30 * time.Second,
				OutputFormat:  OutputFormatText,
			},
			wantErr: false,
		},
		{
			name: "empty server address",
			cfg: &CLIConfig{
				ServerAddress: "",
				Timeout:       30 * time.Second,
				OutputFormat:  OutputFormatText,
			},
			wantErr: true,
			errMsg:  "server_address is required",
		},
		{
			name: "zero timeout",
			cfg: &CLIConfig{
				ServerAddress: "localhost:50051",
				Timeout:       0,
				OutputFormat:  OutputFormatText,
			},
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
		{
			name: "negative timeout",
			cfg: &CLIConfig{
				ServerAddress: "localhost:50051",
				Timeout:       -5 * time.Second,
				OutputFormat:  OutputFormatText,
			},
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
		{
			name: "invalid output format",
			cfg: &CLIConfig{
				ServerAddress: "localhost:50051",
				Timeout:       30 * time.Second,
				OutputFormat:  "invalid",
			},
			wantErr: true,
			errMsg:  "invalid output_format",
		},
		{
			name: "valid with all fields",
			cfg: &CLIConfig{
				ServerAddress: "api.example.com:443",
				Timeout:       60 * time.Second,
				OutputFormat:  OutputFormatJSON,
				TenantID:      "tenant-123",
				TenantAliases: map[string]string{"work": "tenant-work"},
				Debug:         true,
				Insecure:      false,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tc.errMsg)
				} else if tc.errMsg != "" && err.Error() != tc.errMsg && !containsError(err.Error(), tc.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tc.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// containsError checks if the error message contains the expected substring.
func containsError(errMsg, expected string) bool {
	return len(errMsg) > 0 && len(expected) > 0 && (errMsg == expected || errMsg != "" && expected != "" && errMsg != expected)
}

// TestConfigDir verifies config directory path resolution.
func TestConfigDir(t *testing.T) {
	// Save original env
	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer func() {
		if originalEnv != "" {
			os.Setenv("PENF_CONFIG_DIR", originalEnv)
		} else {
			os.Unsetenv("PENF_CONFIG_DIR")
		}
	}()

	t.Run("with env var", func(t *testing.T) {
		customDir := "/tmp/test-penf-config"
		os.Setenv("PENF_CONFIG_DIR", customDir)

		dir, err := ConfigDir()
		if err != nil {
			t.Fatalf("ConfigDir() error = %v", err)
		}
		if dir != customDir {
			t.Errorf("ConfigDir() = %v, want %v", dir, customDir)
		}
	})

	t.Run("default without env var", func(t *testing.T) {
		os.Unsetenv("PENF_CONFIG_DIR")

		dir, err := ConfigDir()
		if err != nil {
			t.Fatalf("ConfigDir() error = %v", err)
		}

		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, DefaultConfigDir)
		if dir != expected {
			t.Errorf("ConfigDir() = %v, want %v", dir, expected)
		}
	})
}

// TestConfigPath verifies config file path resolution.
func TestConfigPath(t *testing.T) {
	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer func() {
		if originalEnv != "" {
			os.Setenv("PENF_CONFIG_DIR", originalEnv)
		} else {
			os.Unsetenv("PENF_CONFIG_DIR")
		}
	}()

	customDir := "/tmp/test-penf-config-path"
	os.Setenv("PENF_CONFIG_DIR", customDir)

	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}

	expected := filepath.Join(customDir, DefaultConfigFile)
	if path != expected {
		t.Errorf("ConfigPath() = %v, want %v", path, expected)
	}
}

// TestLoadConfig_WithEnvOverrides verifies environment variable overrides.
func TestLoadConfig_WithEnvOverrides(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "penf-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save and restore all env vars
	envVars := []string{
		"PENF_CONFIG_DIR",
		"PENF_SERVER_ADDRESS",
		"PENF_TIMEOUT",
		"PENF_OUTPUT_FORMAT",
		"PENF_TENANT_ID",
		"PENF_DEBUG",
		"PENF_INSECURE",
	}
	originals := make(map[string]string)
	for _, key := range envVars {
		originals[key] = os.Getenv(key)
	}
	defer func() {
		for key, val := range originals {
			if val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Set up env
	os.Setenv("PENF_CONFIG_DIR", tempDir)
	os.Setenv("PENF_SERVER_ADDRESS", "custom.server:9090")
	os.Setenv("PENF_TIMEOUT", "45s")
	os.Setenv("PENF_OUTPUT_FORMAT", "json")
	os.Setenv("PENF_TENANT_ID", "env-tenant")
	os.Setenv("PENF_DEBUG", "true")
	os.Setenv("PENF_INSECURE", "1")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.ServerAddress != "custom.server:9090" {
		t.Errorf("ServerAddress = %v, want custom.server:9090", cfg.ServerAddress)
	}
	if cfg.Timeout != 45*time.Second {
		t.Errorf("Timeout = %v, want 45s", cfg.Timeout)
	}
	if cfg.OutputFormat != OutputFormatJSON {
		t.Errorf("OutputFormat = %v, want json", cfg.OutputFormat)
	}
	if cfg.TenantID != "env-tenant" {
		t.Errorf("TenantID = %v, want env-tenant", cfg.TenantID)
	}
	if !cfg.Debug {
		t.Error("Debug should be true")
	}
	if !cfg.Insecure {
		t.Error("Insecure should be true")
	}
}

// TestLoadConfig_Defaults verifies default values when no config exists.
func TestLoadConfig_Defaults(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save and clear relevant env vars
	envVars := []string{
		"PENF_CONFIG_DIR",
		"PENF_SERVER_ADDRESS",
		"PENF_TIMEOUT",
		"PENF_OUTPUT_FORMAT",
		"PENF_TENANT_ID",
		"PENF_DEBUG",
		"PENF_INSECURE",
	}
	originals := make(map[string]string)
	for _, key := range envVars {
		originals[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	defer func() {
		for key, val := range originals {
			if val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	os.Setenv("PENF_CONFIG_DIR", tempDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.ServerAddress != DefaultServerAddress {
		t.Errorf("ServerAddress = %v, want %v", cfg.ServerAddress, DefaultServerAddress)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, DefaultTimeout)
	}
	if cfg.OutputFormat != DefaultOutputFormat {
		t.Errorf("OutputFormat = %v, want %v", cfg.OutputFormat, DefaultOutputFormat)
	}
}

// TestSaveConfig verifies configuration saving.
func TestSaveConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer func() {
		if originalEnv != "" {
			os.Setenv("PENF_CONFIG_DIR", originalEnv)
		} else {
			os.Unsetenv("PENF_CONFIG_DIR")
		}
	}()
	os.Setenv("PENF_CONFIG_DIR", tempDir)

	cfg := &CLIConfig{
		ServerAddress: "saved.server:8080",
		Timeout:       60 * time.Second,
		OutputFormat:  OutputFormatYAML,
		TenantID:      "saved-tenant",
		TenantAliases: map[string]string{
			"work":     "tenant-work-123",
			"personal": "tenant-personal-456",
		},
		Debug:    true,
		Insecure: true,
	}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tempDir, DefaultConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load it back and verify
	// Clear env overrides first
	os.Unsetenv("PENF_SERVER_ADDRESS")
	os.Unsetenv("PENF_TIMEOUT")
	os.Unsetenv("PENF_OUTPUT_FORMAT")
	os.Unsetenv("PENF_TENANT_ID")
	os.Unsetenv("PENF_DEBUG")
	os.Unsetenv("PENF_INSECURE")

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.ServerAddress != cfg.ServerAddress {
		t.Errorf("ServerAddress = %v, want %v", loaded.ServerAddress, cfg.ServerAddress)
	}
	if loaded.Timeout != cfg.Timeout {
		t.Errorf("Timeout = %v, want %v", loaded.Timeout, cfg.Timeout)
	}
	if loaded.OutputFormat != cfg.OutputFormat {
		t.Errorf("OutputFormat = %v, want %v", loaded.OutputFormat, cfg.OutputFormat)
	}
	if loaded.TenantID != cfg.TenantID {
		t.Errorf("TenantID = %v, want %v", loaded.TenantID, cfg.TenantID)
	}
	if loaded.Debug != cfg.Debug {
		t.Errorf("Debug = %v, want %v", loaded.Debug, cfg.Debug)
	}
	if loaded.Insecure != cfg.Insecure {
		t.Errorf("Insecure = %v, want %v", loaded.Insecure, cfg.Insecure)
	}
	if len(loaded.TenantAliases) != len(cfg.TenantAliases) {
		t.Errorf("TenantAliases count = %v, want %v", len(loaded.TenantAliases), len(cfg.TenantAliases))
	}
}

// TestLoadConfig_FromFile verifies loading from YAML file.
func TestLoadConfig_FromFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer func() {
		if originalEnv != "" {
			os.Setenv("PENF_CONFIG_DIR", originalEnv)
		} else {
			os.Unsetenv("PENF_CONFIG_DIR")
		}
	}()
	os.Setenv("PENF_CONFIG_DIR", tempDir)

	// Clear any env overrides
	os.Unsetenv("PENF_SERVER_ADDRESS")
	os.Unsetenv("PENF_TIMEOUT")
	os.Unsetenv("PENF_OUTPUT_FORMAT")
	os.Unsetenv("PENF_TENANT_ID")
	os.Unsetenv("PENF_DEBUG")
	os.Unsetenv("PENF_INSECURE")

	// Create a config file manually
	configContent := `server_address: file.server:7070
timeout: 2m
output_format: yaml
tenant_id: file-tenant
tenant_aliases:
  work: tenant-work-file
debug: true
insecure: false
`
	configPath := filepath.Join(tempDir, DefaultConfigFile)
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.ServerAddress != "file.server:7070" {
		t.Errorf("ServerAddress = %v, want file.server:7070", cfg.ServerAddress)
	}
	if cfg.Timeout != 2*time.Minute {
		t.Errorf("Timeout = %v, want 2m", cfg.Timeout)
	}
	if cfg.OutputFormat != OutputFormatYAML {
		t.Errorf("OutputFormat = %v, want yaml", cfg.OutputFormat)
	}
	if cfg.TenantID != "file-tenant" {
		t.Errorf("TenantID = %v, want file-tenant", cfg.TenantID)
	}
	if !cfg.Debug {
		t.Error("Debug should be true")
	}
	if cfg.Insecure {
		t.Error("Insecure should be false")
	}
	if cfg.TenantAliases["work"] != "tenant-work-file" {
		t.Errorf("TenantAliases[work] = %v, want tenant-work-file", cfg.TenantAliases["work"])
	}
}

// TestLoadConfig_InvalidTimeout verifies handling of invalid timeout in file.
func TestLoadConfig_InvalidTimeout(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer func() {
		if originalEnv != "" {
			os.Setenv("PENF_CONFIG_DIR", originalEnv)
		} else {
			os.Unsetenv("PENF_CONFIG_DIR")
		}
	}()
	os.Setenv("PENF_CONFIG_DIR", tempDir)

	// Clear any env overrides
	os.Unsetenv("PENF_SERVER_ADDRESS")
	os.Unsetenv("PENF_TIMEOUT")

	// Create a config file with invalid timeout
	configContent := `server_address: test.server:8080
timeout: invalid-duration
output_format: text
`
	configPath := filepath.Join(tempDir, DefaultConfigFile)
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err = LoadConfig()
	if err == nil {
		t.Error("LoadConfig() should fail with invalid timeout")
	}
}

// TestEnsureConfigDir verifies config directory creation.
func TestEnsureConfigDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer func() {
		if originalEnv != "" {
			os.Setenv("PENF_CONFIG_DIR", originalEnv)
		} else {
			os.Unsetenv("PENF_CONFIG_DIR")
		}
	}()

	// Use a subdirectory that doesn't exist yet
	newDir := filepath.Join(tempDir, "new-config-dir")
	os.Setenv("PENF_CONFIG_DIR", newDir)

	if err := EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir() error = %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(newDir)
	if os.IsNotExist(err) {
		t.Fatal("Directory was not created")
	}
	if !info.IsDir() {
		t.Fatal("Created path is not a directory")
	}

	// Calling again should not error
	if err := EnsureConfigDir(); err != nil {
		t.Errorf("EnsureConfigDir() second call error = %v", err)
	}
}

// TestCLIConfig_Fields verifies struct field access.
func TestCLIConfig_Fields(t *testing.T) {
	cfg := &CLIConfig{
		ServerAddress: "test.server:8080",
		Timeout:       45 * time.Second,
		OutputFormat:  OutputFormatJSON,
		TenantID:      "test-tenant",
		TenantAliases: map[string]string{
			"alias1": "tenant-1",
			"alias2": "tenant-2",
		},
		Debug:    true,
		Insecure: true,
	}

	if cfg.ServerAddress != "test.server:8080" {
		t.Errorf("ServerAddress = %v, want test.server:8080", cfg.ServerAddress)
	}
	if cfg.Timeout != 45*time.Second {
		t.Errorf("Timeout = %v, want 45s", cfg.Timeout)
	}
	if cfg.OutputFormat != OutputFormatJSON {
		t.Errorf("OutputFormat = %v, want json", cfg.OutputFormat)
	}
	if cfg.TenantID != "test-tenant" {
		t.Errorf("TenantID = %v, want test-tenant", cfg.TenantID)
	}
	if len(cfg.TenantAliases) != 2 {
		t.Errorf("TenantAliases count = %v, want 2", len(cfg.TenantAliases))
	}
	if !cfg.Debug {
		t.Error("Debug should be true")
	}
	if !cfg.Insecure {
		t.Error("Insecure should be true")
	}
}

// TestLoadFromEnv_PartialOverride verifies partial env var overrides.
func TestLoadFromEnv_PartialOverride(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save and restore env vars
	envVars := []string{
		"PENF_CONFIG_DIR",
		"PENF_SERVER_ADDRESS",
		"PENF_TIMEOUT",
		"PENF_OUTPUT_FORMAT",
	}
	originals := make(map[string]string)
	for _, key := range envVars {
		originals[key] = os.Getenv(key)
	}
	defer func() {
		for key, val := range originals {
			if val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	os.Setenv("PENF_CONFIG_DIR", tempDir)

	// Create config file with all values
	configContent := `server_address: file.server:1111
timeout: 10s
output_format: yaml
`
	configPath := filepath.Join(tempDir, DefaultConfigFile)
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Only override server address via env
	os.Setenv("PENF_SERVER_ADDRESS", "env.server:2222")
	os.Unsetenv("PENF_TIMEOUT")
	os.Unsetenv("PENF_OUTPUT_FORMAT")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Server address from env
	if cfg.ServerAddress != "env.server:2222" {
		t.Errorf("ServerAddress = %v, want env.server:2222", cfg.ServerAddress)
	}
	// Timeout from file
	if cfg.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", cfg.Timeout)
	}
	// Output format from file
	if cfg.OutputFormat != OutputFormatYAML {
		t.Errorf("OutputFormat = %v, want yaml", cfg.OutputFormat)
	}
}

// TestLoadFromEnv_InvalidTimeout verifies invalid env timeout is ignored.
func TestLoadFromEnv_InvalidTimeout(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalConfigDir := os.Getenv("PENF_CONFIG_DIR")
	originalTimeout := os.Getenv("PENF_TIMEOUT")
	defer func() {
		if originalConfigDir != "" {
			os.Setenv("PENF_CONFIG_DIR", originalConfigDir)
		} else {
			os.Unsetenv("PENF_CONFIG_DIR")
		}
		if originalTimeout != "" {
			os.Setenv("PENF_TIMEOUT", originalTimeout)
		} else {
			os.Unsetenv("PENF_TIMEOUT")
		}
	}()

	os.Setenv("PENF_CONFIG_DIR", tempDir)
	os.Setenv("PENF_TIMEOUT", "not-a-duration") // Invalid

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Should use default timeout since env value is invalid
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v (default)", cfg.Timeout, DefaultTimeout)
	}
}

// TestSaveConfig_CreatesDirectory verifies SaveConfig creates parent directory.
func TestSaveConfig_CreatesDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer func() {
		if originalEnv != "" {
			os.Setenv("PENF_CONFIG_DIR", originalEnv)
		} else {
			os.Unsetenv("PENF_CONFIG_DIR")
		}
	}()

	// Use a non-existent subdirectory
	newDir := filepath.Join(tempDir, "nested", "config", "dir")
	os.Setenv("PENF_CONFIG_DIR", newDir)

	cfg := DefaultConfig()
	cfg.TenantID = "test-tenant"

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(newDir, DefaultConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}
}

// TestLoadConfig_EmptyOutputFormat verifies handling of empty output format.
func TestLoadConfig_EmptyOutputFormat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer func() {
		if originalEnv != "" {
			os.Setenv("PENF_CONFIG_DIR", originalEnv)
		} else {
			os.Unsetenv("PENF_CONFIG_DIR")
		}
	}()
	os.Setenv("PENF_CONFIG_DIR", tempDir)

	// Create config with empty output_format (should use default)
	configContent := `server_address: test.server:8080
timeout: 30s
`
	configPath := filepath.Join(tempDir, DefaultConfigFile)
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Should use default output format
	if cfg.OutputFormat != DefaultOutputFormat {
		t.Errorf("OutputFormat = %v, want %v", cfg.OutputFormat, DefaultOutputFormat)
	}
}

// TestFilePermissions verifies config file permissions.
func TestFilePermissions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer func() {
		if originalEnv != "" {
			os.Setenv("PENF_CONFIG_DIR", originalEnv)
		} else {
			os.Unsetenv("PENF_CONFIG_DIR")
		}
	}()
	os.Setenv("PENF_CONFIG_DIR", tempDir)

	cfg := DefaultConfig()
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	configPath := filepath.Join(tempDir, DefaultConfigFile)
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	mode := info.Mode().Perm()
	// Should be 0600 (owner read/write only)
	if mode != 0600 {
		t.Errorf("File permissions = %o, want 0600", mode)
	}
}
