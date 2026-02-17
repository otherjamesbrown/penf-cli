// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// DebugInfo contains diagnostic information about the CLI and system.
type DebugInfo struct {
	CLI        CLIInfo        `json:"cli" yaml:"cli"`
	Config     ConfigInfo     `json:"config" yaml:"config"`
	Connection ConnectionInfo `json:"connection" yaml:"connection"`
	System     SystemInfo     `json:"system" yaml:"system"`
	Timestamp  time.Time      `json:"timestamp" yaml:"timestamp"`
}

// CLIInfo contains CLI version information.
type CLIInfo struct {
	Version   string `json:"version" yaml:"version"`
	Commit    string `json:"commit" yaml:"commit"`
	BuildTime string `json:"build_time" yaml:"build_time"`
	GoVersion string `json:"go_version" yaml:"go_version"`
}

// ConfigInfo contains configuration information.
type ConfigInfo struct {
	ConfigPath     string `json:"config_path" yaml:"config_path"`
	ConfigExists   bool   `json:"config_exists" yaml:"config_exists"`
	ServerAddress  string `json:"server_address" yaml:"server_address"`
	TenantID       string `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	TenantSource   string `json:"tenant_source,omitempty" yaml:"tenant_source,omitempty"`
	OutputFormat   string `json:"output_format" yaml:"output_format"`
	Timeout        string `json:"timeout" yaml:"timeout"`
	Debug          bool   `json:"debug" yaml:"debug"`
	Insecure       bool   `json:"insecure" yaml:"insecure"`
	CredentialPath string `json:"credential_path" yaml:"credential_path"`
	HasCredentials bool   `json:"has_credentials" yaml:"has_credentials"`
}

// ConnectionInfo contains connection test results.
type ConnectionInfo struct {
	Status        string  `json:"status" yaml:"status"`
	ServerAddress string  `json:"server_address" yaml:"server_address"`
	LatencyMs     float64 `json:"latency_ms,omitempty" yaml:"latency_ms,omitempty"`
	Error         string  `json:"error,omitempty" yaml:"error,omitempty"`
}

// SystemInfo contains system environment information.
type SystemInfo struct {
	OS           string            `json:"os" yaml:"os"`
	Arch         string            `json:"arch" yaml:"arch"`
	NumCPU       int               `json:"num_cpu" yaml:"num_cpu"`
	HomeDir      string            `json:"home_dir" yaml:"home_dir"`
	WorkDir      string            `json:"work_dir" yaml:"work_dir"`
	EnvVars      map[string]string `json:"env_vars" yaml:"env_vars"`
	GoMaxProcs   int               `json:"go_max_procs" yaml:"go_max_procs"`
	NumGoroutine int               `json:"num_goroutine" yaml:"num_goroutine"`
}

// DebugCommandDeps holds the dependencies for debug commands.
type DebugCommandDeps struct {
	Config     *config.CLIConfig
	GRPCClient *client.GRPCClient
	LoadConfig func() (*config.CLIConfig, error)
	InitClient func(*config.CLIConfig) (*client.GRPCClient, error)
	Version    string
	Commit     string
	BuildTime  string
}

// DefaultDebugDeps returns the default dependencies for production use.
func DefaultDebugDeps() *DebugCommandDeps {
	return &DebugCommandDeps{
		LoadConfig: config.LoadConfig,
		InitClient: client.ConnectFromConfig,
		Version:   "dev",
		Commit:    "unknown",
		BuildTime: "unknown",
	}
}

// Debug command flags.
var (
	debugOutput    string
	debugTestConn  bool
	debugShowEnv   bool
	debugShowPaths bool
)

// NewDebugCommand creates the debug command with subcommands.
func NewDebugCommand(deps *DebugCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultDebugDeps()
	}

	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Debug and diagnostic utilities",
		Long: `Debug and diagnostic utilities for troubleshooting the penf CLI.

Provides information about configuration, connection status, system environment,
and other diagnostic data useful for debugging issues.

Commands:
  info   - Show comprehensive debug information
  config - Show configuration details
  env    - Show environment variables
  ping   - Test connection to the server

Examples:
  # Show all debug information
  penf debug info

  # Show configuration
  penf debug config

  # Test server connection
  penf debug ping

  # Show relevant environment variables
  penf debug env`,
	}

	// Add subcommands.
	cmd.AddCommand(newDebugInfoCommand(deps))
	cmd.AddCommand(newDebugConfigCommand(deps))
	cmd.AddCommand(newDebugEnvCommand(deps))
	cmd.AddCommand(newDebugPingCommand(deps))

	return cmd
}

// newDebugInfoCommand creates the 'debug info' subcommand.
func newDebugInfoCommand(deps *DebugCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show comprehensive debug information",
		Long: `Show comprehensive debug information including CLI version, configuration,
connection status, and system environment.

This is useful for troubleshooting issues or providing diagnostic information
when reporting bugs.

Examples:
  penf debug info
  penf debug info --output=json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDebugInfo(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&debugOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newDebugConfigCommand creates the 'debug config' subcommand.
func newDebugConfigCommand(deps *DebugCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show configuration details",
		Long: `Show detailed configuration information including file paths,
loaded values, and their sources (file, environment, defaults).

Examples:
  penf debug config
  penf debug config --output=json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDebugConfig(deps)
		},
	}

	cmd.Flags().StringVarP(&debugOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newDebugEnvCommand creates the 'debug env' subcommand.
func newDebugEnvCommand(deps *DebugCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "env",
		Short: "Show relevant environment variables",
		Long: `Show environment variables relevant to penf configuration.

Displays PENF_* environment variables and their values, useful for
debugging configuration issues.

Examples:
  penf debug env`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDebugEnv()
		},
	}
}

// newDebugPingCommand creates the 'debug ping' subcommand.
func newDebugPingCommand(deps *DebugCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Test connection to the server",
		Long: `Test the connection to the Penfold API Gateway.

Attempts to establish a connection and measure latency.

Examples:
  penf debug ping`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDebugPing(cmd.Context(), deps)
		},
	}
}

// runDebugInfo executes the debug info command.
func runDebugInfo(ctx context.Context, deps *DebugCommandDeps) error {
	cfg, _ := deps.LoadConfig() // Ignore errors, show what we can.

	// Gather debug information.
	info := DebugInfo{
		CLI:        getCLIInfo(deps),
		Config:     getConfigInfo(cfg),
		Connection: getConnectionInfo(ctx, deps, cfg),
		System:     getSystemInfo(),
		Timestamp:  time.Now(),
	}

	// Output based on format.
	format := config.OutputFormatText
	if debugOutput != "" {
		format = config.OutputFormat(debugOutput)
	}

	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(info)
	default:
		return outputDebugInfoText(info)
	}
}

// runDebugConfig shows configuration details.
func runDebugConfig(deps *DebugCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		fmt.Println()
	}

	info := getConfigInfo(cfg)

	format := config.OutputFormatText
	if debugOutput != "" {
		format = config.OutputFormat(debugOutput)
	}

	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(info)
	default:
		return outputConfigInfoText(info)
	}
}

// runDebugEnv shows environment variables.
func runDebugEnv() error {
	fmt.Println("Penfold Environment Variables:")
	fmt.Println()

	envVars := []string{
		"PENF_SERVER_ADDRESS",
		"PENF_TIMEOUT",
		"PENF_OUTPUT_FORMAT",
		"PENF_TENANT_ID",
		"PENF_DEBUG",
		"PENF_INSECURE",
		"PENF_CONFIG_DIR",
		"PENF_API_KEY",
		"PENF_TOKEN",
	}

	found := false
	for _, key := range envVars {
		value := os.Getenv(key)
		if value != "" {
			found = true
			// Mask sensitive values.
			if key == "PENF_API_KEY" || key == "PENF_TOKEN" {
				value = maskValue(value)
			}
			fmt.Printf("  %s=%s\n", key, value)
		} else {
			fmt.Printf("  %s=(not set)\n", key)
		}
	}

	if !found {
		fmt.Println("  (no PENF_* environment variables are set)")
	}

	return nil
}

// runDebugPing tests the connection.
func runDebugPing(ctx context.Context, deps *DebugCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	fmt.Printf("Pinging %s...\n", cfg.ServerAddress)

	// Time the connection attempt.
	start := time.Now()

	grpcClient, err := deps.InitClient(cfg)
	latency := time.Since(start)

	if err != nil {
		fmt.Printf("\n\033[31mConnection failed\033[0m\n")
		fmt.Printf("  Error: %v\n", err)
		fmt.Printf("  Latency: %v\n", latency)
		return nil // Don't return error, just report status.
	}
	defer grpcClient.Close()

	// Perform health check.
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := grpcClient.HealthCheck(healthCtx); err != nil {
		fmt.Printf("\n\033[33mConnection established but health check failed\033[0m\n")
		fmt.Printf("  State: %s\n", grpcClient.ConnectionState())
		fmt.Printf("  Error: %v\n", err)
		fmt.Printf("  Latency: %v\n", latency)
		return nil
	}

	fmt.Printf("\n\033[32mConnection successful\033[0m\n")
	fmt.Printf("  State: %s\n", grpcClient.ConnectionState())
	fmt.Printf("  Latency: %v\n", latency)

	return nil
}

// getCLIInfo returns CLI version information.
func getCLIInfo(deps *DebugCommandDeps) CLIInfo {
	return CLIInfo{
		Version:   deps.Version,
		Commit:    deps.Commit,
		BuildTime: deps.BuildTime,
		GoVersion: runtime.Version(),
	}
}

// getConfigInfo returns configuration information.
func getConfigInfo(cfg *config.CLIConfig) ConfigInfo {
	configPath, _ := config.ConfigPath()
	_, configExists := os.Stat(configPath)

	credPath := ""
	hasCredentials := false
	if configDir, err := config.ConfigDir(); err == nil {
		credPath = configDir + "/credentials.yaml"
		if _, err := os.Stat(credPath); err == nil {
			hasCredentials = true
		}
	}

	info := ConfigInfo{
		ConfigPath:     configPath,
		ConfigExists:   configExists == nil,
		CredentialPath: credPath,
		HasCredentials: hasCredentials,
	}

	if cfg != nil {
		info.ServerAddress = cfg.ServerAddress
		info.TenantID = cfg.TenantID
		info.OutputFormat = string(cfg.OutputFormat)
		info.Timeout = cfg.Timeout.String()
		info.Debug = cfg.Debug
		info.Insecure = cfg.Insecure

		// Determine tenant source.
		if os.Getenv("PENF_TENANT_ID") != "" {
			info.TenantSource = "environment"
		} else if cfg.TenantID != "" {
			info.TenantSource = "config"
		}
	}

	return info
}

// getConnectionInfo returns connection test results.
func getConnectionInfo(ctx context.Context, deps *DebugCommandDeps, cfg *config.CLIConfig) ConnectionInfo {
	if cfg == nil {
		return ConnectionInfo{
			Status: "unknown",
			Error:  "configuration not loaded",
		}
	}

	info := ConnectionInfo{
		ServerAddress: cfg.ServerAddress,
	}

	// Try to connect.
	start := time.Now()
	grpcClient, err := deps.InitClient(cfg)
	latency := time.Since(start)

	if err != nil {
		info.Status = "failed"
		info.Error = err.Error()
		info.LatencyMs = float64(latency.Milliseconds())
		return info
	}
	defer grpcClient.Close()

	info.Status = grpcClient.ConnectionState()
	info.LatencyMs = float64(latency.Milliseconds())

	return info
}

// getSystemInfo returns system environment information.
func getSystemInfo() SystemInfo {
	homeDir, _ := os.UserHomeDir()
	workDir, _ := os.Getwd()

	// Collect relevant environment variables.
	envVars := make(map[string]string)
	relevantEnvs := []string{
		"PENF_SERVER_ADDRESS",
		"PENF_TENANT_ID",
		"PENF_DEBUG",
		"PENF_INSECURE",
		"PENF_CONFIG_DIR",
	}
	for _, key := range relevantEnvs {
		if value := os.Getenv(key); value != "" {
			envVars[key] = value
		}
	}

	return SystemInfo{
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		NumCPU:       runtime.NumCPU(),
		HomeDir:      homeDir,
		WorkDir:      workDir,
		EnvVars:      envVars,
		GoMaxProcs:   runtime.GOMAXPROCS(0),
		NumGoroutine: runtime.NumGoroutine(),
	}
}

// outputDebugInfoText outputs debug info in text format.
func outputDebugInfoText(info DebugInfo) error {
	fmt.Println("=== Penfold CLI Debug Information ===")
	fmt.Printf("Generated: %s\n\n", info.Timestamp.Format(time.RFC3339))

	// CLI Info.
	fmt.Println("[CLI]")
	fmt.Printf("  Version:    %s\n", info.CLI.Version)
	fmt.Printf("  Commit:     %s\n", info.CLI.Commit)
	fmt.Printf("  Build Time: %s\n", info.CLI.BuildTime)
	fmt.Printf("  Go Version: %s\n", info.CLI.GoVersion)
	fmt.Println()

	// Config Info.
	fmt.Println("[Configuration]")
	fmt.Printf("  Config Path:   %s", info.Config.ConfigPath)
	if info.Config.ConfigExists {
		fmt.Println(" (exists)")
	} else {
		fmt.Println(" (not found)")
	}
	fmt.Printf("  Server:        %s\n", info.Config.ServerAddress)
	fmt.Printf("  Timeout:       %s\n", info.Config.Timeout)
	fmt.Printf("  Output Format: %s\n", info.Config.OutputFormat)
	if info.Config.TenantID != "" {
		fmt.Printf("  Tenant ID:     %s (from %s)\n", info.Config.TenantID, info.Config.TenantSource)
	}
	fmt.Printf("  Debug Mode:    %t\n", info.Config.Debug)
	fmt.Printf("  Insecure:      %t\n", info.Config.Insecure)
	fmt.Printf("  Credentials:   %s", info.Config.CredentialPath)
	if info.Config.HasCredentials {
		fmt.Println(" (found)")
	} else {
		fmt.Println(" (not found)")
	}
	fmt.Println()

	// Connection Info.
	fmt.Println("[Connection]")
	fmt.Printf("  Server:  %s\n", info.Connection.ServerAddress)
	statusColor := "\033[32m"
	if info.Connection.Status != "ready" {
		statusColor = "\033[31m"
	}
	fmt.Printf("  Status:  %s%s\033[0m\n", statusColor, info.Connection.Status)
	if info.Connection.LatencyMs > 0 {
		fmt.Printf("  Latency: %.0fms\n", info.Connection.LatencyMs)
	}
	if info.Connection.Error != "" {
		fmt.Printf("  Error:   %s\n", info.Connection.Error)
	}
	fmt.Println()

	// System Info.
	fmt.Println("[System]")
	fmt.Printf("  OS/Arch:    %s/%s\n", info.System.OS, info.System.Arch)
	fmt.Printf("  CPUs:       %d\n", info.System.NumCPU)
	fmt.Printf("  GOMAXPROCS: %d\n", info.System.GoMaxProcs)
	fmt.Printf("  Goroutines: %d\n", info.System.NumGoroutine)
	fmt.Printf("  Home Dir:   %s\n", info.System.HomeDir)
	fmt.Printf("  Work Dir:   %s\n", info.System.WorkDir)
	if len(info.System.EnvVars) > 0 {
		fmt.Println("  Environment:")
		for k, v := range info.System.EnvVars {
			fmt.Printf("    %s=%s\n", k, v)
		}
	}

	return nil
}

// outputConfigInfoText outputs config info in text format.
func outputConfigInfoText(info ConfigInfo) error {
	fmt.Println("Configuration Details:")
	fmt.Println()

	fmt.Printf("  Config File:     %s", info.ConfigPath)
	if info.ConfigExists {
		fmt.Println(" (exists)")
	} else {
		fmt.Println(" (not found - using defaults)")
	}

	fmt.Printf("  Credentials:     %s", info.CredentialPath)
	if info.HasCredentials {
		fmt.Println(" (found)")
	} else {
		fmt.Println(" (not found)")
	}
	fmt.Println()

	fmt.Println("  Active Settings:")
	fmt.Printf("    server_address: %s\n", info.ServerAddress)
	fmt.Printf("    timeout:        %s\n", info.Timeout)
	fmt.Printf("    output_format:  %s\n", info.OutputFormat)
	if info.TenantID != "" {
		fmt.Printf("    tenant_id:      %s (source: %s)\n", info.TenantID, info.TenantSource)
	}
	fmt.Printf("    debug:          %t\n", info.Debug)
	fmt.Printf("    insecure:       %t\n", info.Insecure)

	return nil
}

// maskValue masks a sensitive value.
func maskValue(value string) string {
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "****" + value[len(value)-4:]
}
