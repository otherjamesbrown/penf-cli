// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// mockDebugConfig creates a mock configuration for debug command testing.
func mockDebugConfig() *config.CLIConfig {
	return &config.CLIConfig{
		ServerAddress: "localhost:50051",
		Timeout:       30 * time.Second,
		OutputFormat:  config.OutputFormatText,
		TenantID:      "tenant-test-001",
		Debug:         false,
		Insecure:      true,
	}
}

// createDebugTestDeps creates test dependencies for debug commands.
func createDebugTestDeps(cfg *config.CLIConfig) *DebugCommandDeps {
	return &DebugCommandDeps{
		Config: cfg,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		InitClient: func(c *config.CLIConfig) (*client.GRPCClient, error) {
			// Return a client that will fail health checks for testing.
			opts := client.DefaultOptions()
			opts.Insecure = true
			return client.NewGRPCClient(c.ServerAddress, opts), nil
		},
		Version:   "1.0.0-test",
		Commit:    "abc123",
		BuildTime: "2024-01-15T10:00:00Z",
	}
}

func TestNewDebugCommand(t *testing.T) {
	deps := createDebugTestDeps(mockDebugConfig())
	cmd := NewDebugCommand(deps)

	assert.NotNil(t, cmd)
	assert.Equal(t, "debug", cmd.Use)
	assert.Contains(t, cmd.Short, "Debug")

	// Check subcommands exist.
	subcommands := cmd.Commands()
	expectedSubcmds := []string{"info", "config", "env", "ping"}

	for _, expected := range expectedSubcmds {
		found := false
		for _, sub := range subcommands {
			if sub.Use == expected || strings.HasPrefix(sub.Use, expected+" ") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected subcommand %q not found", expected)
	}
}

func TestNewDebugCommand_WithNilDeps(t *testing.T) {
	cmd := NewDebugCommand(nil)
	assert.NotNil(t, cmd)
	assert.Equal(t, "debug", cmd.Use)
}

func TestDebugCommand_InfoSubcommand(t *testing.T) {
	deps := createDebugTestDeps(mockDebugConfig())
	cmd := NewDebugCommand(deps)

	infoCmd, _, err := cmd.Find([]string{"info"})
	require.NoError(t, err)
	require.NotNil(t, infoCmd)

	assert.Equal(t, "info", infoCmd.Use)

	// Check flags.
	flag := infoCmd.Flags().Lookup("output")
	assert.NotNil(t, flag)
}

func TestDebugCommand_ConfigSubcommand(t *testing.T) {
	deps := createDebugTestDeps(mockDebugConfig())
	cmd := NewDebugCommand(deps)

	configCmd, _, err := cmd.Find([]string{"config"})
	require.NoError(t, err)
	require.NotNil(t, configCmd)

	assert.Equal(t, "config", configCmd.Use)

	// Check flags.
	flag := configCmd.Flags().Lookup("output")
	assert.NotNil(t, flag)
}

func TestDebugCommand_EnvSubcommand(t *testing.T) {
	deps := createDebugTestDeps(mockDebugConfig())
	cmd := NewDebugCommand(deps)

	envCmd, _, err := cmd.Find([]string{"env"})
	require.NoError(t, err)
	require.NotNil(t, envCmd)

	assert.Equal(t, "env", envCmd.Use)
}

func TestDebugCommand_PingSubcommand(t *testing.T) {
	deps := createDebugTestDeps(mockDebugConfig())
	cmd := NewDebugCommand(deps)

	pingCmd, _, err := cmd.Find([]string{"ping"})
	require.NoError(t, err)
	require.NotNil(t, pingCmd)

	assert.Equal(t, "ping", pingCmd.Use)
}

func TestRunDebugInfo(t *testing.T) {
	cfg := mockDebugConfig()
	deps := createDebugTestDeps(cfg)

	// Reset global flags.
	oldOutput := debugOutput
	debugOutput = ""
	defer func() {
		debugOutput = oldOutput
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runDebugInfo(ctx, deps)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Penfold CLI Debug Information")
	assert.Contains(t, output, "CLI")
	assert.Contains(t, output, "Configuration")
	assert.Contains(t, output, "System")
}

func TestRunDebugInfo_JSONOutput(t *testing.T) {
	cfg := mockDebugConfig()
	deps := createDebugTestDeps(cfg)

	oldOutput := debugOutput
	debugOutput = "json"
	defer func() {
		debugOutput = oldOutput
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runDebugInfo(ctx, deps)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)

	// Verify valid JSON.
	var debugInfo DebugInfo
	err = json.Unmarshal([]byte(output), &debugInfo)
	assert.NoError(t, err)
	assert.Equal(t, "1.0.0-test", debugInfo.CLI.Version)
}

func TestRunDebugInfo_YAMLOutput(t *testing.T) {
	cfg := mockDebugConfig()
	deps := createDebugTestDeps(cfg)

	oldOutput := debugOutput
	debugOutput = "yaml"
	defer func() {
		debugOutput = oldOutput
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runDebugInfo(ctx, deps)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)

	// Verify valid YAML.
	var debugInfo DebugInfo
	err = yaml.Unmarshal([]byte(output), &debugInfo)
	assert.NoError(t, err)
}

func TestRunDebugConfig(t *testing.T) {
	cfg := mockDebugConfig()
	deps := createDebugTestDeps(cfg)

	oldOutput := debugOutput
	debugOutput = ""
	defer func() {
		debugOutput = oldOutput
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runDebugConfig(deps)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Configuration Details")
	assert.Contains(t, output, "server_address")
}

func TestRunDebugConfig_JSONOutput(t *testing.T) {
	cfg := mockDebugConfig()
	deps := createDebugTestDeps(cfg)

	oldOutput := debugOutput
	debugOutput = "json"
	defer func() {
		debugOutput = oldOutput
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runDebugConfig(deps)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)

	// Verify valid JSON.
	var configInfo ConfigInfo
	err = json.Unmarshal([]byte(output), &configInfo)
	assert.NoError(t, err)
}

func TestRunDebugEnv(t *testing.T) {
	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runDebugEnv()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Penfold Environment Variables")
	assert.Contains(t, output, "PENF_SERVER_ADDRESS")
}

func TestRunDebugEnv_WithEnvVarSet(t *testing.T) {
	// Set an env var.
	oldEnv := os.Getenv("PENF_DEBUG")
	os.Setenv("PENF_DEBUG", "true")
	defer os.Setenv("PENF_DEBUG", oldEnv)

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runDebugEnv()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "PENF_DEBUG=true")
}

func TestGetCLIInfo(t *testing.T) {
	deps := createDebugTestDeps(mockDebugConfig())

	info := getCLIInfo(deps)

	assert.Equal(t, "1.0.0-test", info.Version)
	assert.Equal(t, "abc123", info.Commit)
	assert.Equal(t, "2024-01-15T10:00:00Z", info.BuildTime)
	assert.Contains(t, info.GoVersion, "go")
}

func TestGetConfigInfo(t *testing.T) {
	cfg := mockDebugConfig()

	info := getConfigInfo(cfg)

	assert.Equal(t, "localhost:50051", info.ServerAddress)
	assert.Equal(t, "30s", info.Timeout)
	assert.Equal(t, "text", info.OutputFormat)
	assert.True(t, info.Insecure)
}

func TestGetConfigInfo_NilConfig(t *testing.T) {
	info := getConfigInfo(nil)

	// Should still return a valid struct with defaults/empty values.
	assert.NotEmpty(t, info.ConfigPath)
}

func TestGetConfigInfo_WithTenantFromEnv(t *testing.T) {
	oldEnv := os.Getenv("PENF_TENANT_ID")
	os.Setenv("PENF_TENANT_ID", "env-tenant-123")
	defer os.Setenv("PENF_TENANT_ID", oldEnv)

	cfg := mockDebugConfig()
	cfg.TenantID = "env-tenant-123"

	info := getConfigInfo(cfg)

	assert.Equal(t, "env-tenant-123", info.TenantID)
	assert.Equal(t, "environment", info.TenantSource)
}

func TestGetConfigInfo_WithTenantFromConfig(t *testing.T) {
	// Clear env var.
	oldEnv := os.Getenv("PENF_TENANT_ID")
	os.Unsetenv("PENF_TENANT_ID")
	defer os.Setenv("PENF_TENANT_ID", oldEnv)

	cfg := mockDebugConfig()
	cfg.TenantID = "config-tenant-123"

	info := getConfigInfo(cfg)

	assert.Equal(t, "config-tenant-123", info.TenantID)
	assert.Equal(t, "config", info.TenantSource)
}

func TestGetSystemInfo(t *testing.T) {
	info := getSystemInfo()

	assert.Equal(t, runtime.GOOS, info.OS)
	assert.Equal(t, runtime.GOARCH, info.Arch)
	assert.True(t, info.NumCPU > 0)
	assert.True(t, info.GoMaxProcs > 0)
	assert.True(t, info.NumGoroutine > 0)
	assert.NotEmpty(t, info.HomeDir)
	assert.NotEmpty(t, info.WorkDir)
}

func TestGetConnectionInfo_NilConfig(t *testing.T) {
	deps := createDebugTestDeps(nil)
	ctx := context.Background()

	info := getConnectionInfo(ctx, deps, nil)

	assert.Equal(t, "unknown", info.Status)
	assert.Contains(t, info.Error, "configuration not loaded")
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"short", "****"},
		{"12345678", "****"},
		{"123456789", "1234****6789"},
		{"long-api-key-value", "long****alue"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := maskValue(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestDebugInfo_JSONSerialization(t *testing.T) {
	info := DebugInfo{
		CLI: CLIInfo{
			Version:   "1.0.0",
			Commit:    "abc123",
			BuildTime: "2024-01-15",
			GoVersion: "go1.21",
		},
		Config: ConfigInfo{
			ServerAddress: "localhost:50051",
			Timeout:       "30s",
		},
		Connection: ConnectionInfo{
			Status:        "ready",
			ServerAddress: "localhost:50051",
			LatencyMs:     10.5,
		},
		System: SystemInfo{
			OS:     "linux",
			Arch:   "amd64",
			NumCPU: 8,
		},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded DebugInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, info.CLI.Version, decoded.CLI.Version)
	assert.Equal(t, info.Config.ServerAddress, decoded.Config.ServerAddress)
	assert.Equal(t, info.Connection.Status, decoded.Connection.Status)
	assert.Equal(t, info.System.OS, decoded.System.OS)
}

func TestCLIInfo_JSONSerialization(t *testing.T) {
	info := CLIInfo{
		Version:   "1.0.0",
		Commit:    "abc123",
		BuildTime: "2024-01-15T10:00:00Z",
		GoVersion: "go1.21.0",
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded CLIInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, info.Version, decoded.Version)
	assert.Equal(t, info.Commit, decoded.Commit)
	assert.Equal(t, info.BuildTime, decoded.BuildTime)
	assert.Equal(t, info.GoVersion, decoded.GoVersion)
}

func TestConfigInfo_JSONSerialization(t *testing.T) {
	info := ConfigInfo{
		ConfigPath:     "/home/user/.penf/config.yaml",
		ConfigExists:   true,
		ServerAddress:  "localhost:50051",
		TenantID:       "tenant-123",
		TenantSource:   "config",
		OutputFormat:   "text",
		Timeout:        "30s",
		Debug:          false,
		Insecure:       true,
		CredentialPath: "/home/user/.penf/credentials.yaml",
		HasCredentials: true,
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded ConfigInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, info.ConfigPath, decoded.ConfigPath)
	assert.Equal(t, info.ConfigExists, decoded.ConfigExists)
	assert.Equal(t, info.ServerAddress, decoded.ServerAddress)
	assert.Equal(t, info.TenantID, decoded.TenantID)
}

func TestConnectionInfo_JSONSerialization(t *testing.T) {
	info := ConnectionInfo{
		Status:        "ready",
		ServerAddress: "localhost:50051",
		LatencyMs:     15.5,
		Error:         "",
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded ConnectionInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, info.Status, decoded.Status)
	assert.Equal(t, info.ServerAddress, decoded.ServerAddress)
	assert.Equal(t, info.LatencyMs, decoded.LatencyMs)
}

func TestSystemInfo_JSONSerialization(t *testing.T) {
	info := SystemInfo{
		OS:           "darwin",
		Arch:         "arm64",
		NumCPU:       10,
		HomeDir:      "/Users/test",
		WorkDir:      "/Users/test/projects",
		EnvVars:      map[string]string{"PENF_DEBUG": "true"},
		GoMaxProcs:   10,
		NumGoroutine: 5,
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded SystemInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, info.OS, decoded.OS)
	assert.Equal(t, info.Arch, decoded.Arch)
	assert.Equal(t, info.NumCPU, decoded.NumCPU)
	assert.Equal(t, info.HomeDir, decoded.HomeDir)
	assert.Equal(t, "true", decoded.EnvVars["PENF_DEBUG"])
}

func TestOutputDebugInfoText(t *testing.T) {
	info := DebugInfo{
		CLI: CLIInfo{
			Version:   "1.0.0",
			Commit:    "abc123",
			BuildTime: "2024-01-15",
			GoVersion: "go1.21",
		},
		Config: ConfigInfo{
			ConfigPath:    "/home/user/.penf/config.yaml",
			ConfigExists:  true,
			ServerAddress: "localhost:50051",
			Timeout:       "30s",
			OutputFormat:  "text",
		},
		Connection: ConnectionInfo{
			Status:        "ready",
			ServerAddress: "localhost:50051",
			LatencyMs:     10.5,
		},
		System: SystemInfo{
			OS:           "linux",
			Arch:         "amd64",
			NumCPU:       8,
			HomeDir:      "/home/user",
			WorkDir:      "/home/user/work",
			GoMaxProcs:   8,
			NumGoroutine: 5,
		},
		Timestamp: time.Now(),
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputDebugInfoText(info)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "[CLI]")
	assert.Contains(t, output, "1.0.0")
	assert.Contains(t, output, "[Configuration]")
	assert.Contains(t, output, "[Connection]")
	assert.Contains(t, output, "[System]")
}

func TestOutputConfigInfoText(t *testing.T) {
	info := ConfigInfo{
		ConfigPath:     "/home/user/.penf/config.yaml",
		ConfigExists:   true,
		CredentialPath: "/home/user/.penf/credentials.yaml",
		HasCredentials: true,
		ServerAddress:  "localhost:50051",
		Timeout:        "30s",
		OutputFormat:   "text",
		TenantID:       "tenant-123",
		TenantSource:   "config",
		Debug:          false,
		Insecure:       true,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputConfigInfoText(info)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Configuration Details")
	assert.Contains(t, output, "Config File:")
	assert.Contains(t, output, "server_address:")
	assert.Contains(t, output, "tenant_id:")
}

func TestDefaultDebugDeps(t *testing.T) {
	deps := DefaultDebugDeps()

	assert.NotNil(t, deps)
	assert.NotNil(t, deps.LoadConfig)
	assert.NotNil(t, deps.InitClient)
	assert.Equal(t, "dev", deps.Version)
	assert.Equal(t, "unknown", deps.Commit)
	assert.Equal(t, "unknown", deps.BuildTime)
}
