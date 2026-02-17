// Package main provides the penf CLI entry point.
// penf is the command-line interface for the Penfold personal information system.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/cmd"
	"github.com/otherjamesbrown/penf-cli/config"
	"github.com/otherjamesbrown/penf-cli/contextpalace"
	"github.com/otherjamesbrown/penf-cli/pkg/buildinfo"
)

// Global flags and state.
var (
	cfgFile      string
	serverAddr   string
	timeout      time.Duration
	outputFormat string
	debug        bool
	insecure     bool

	// cfg holds the loaded configuration.
	cfg *config.CLIConfig

	// grpcClient is the shared gRPC client.
	grpcClient *client.GRPCClient

	// Command logging state.
	cmdStartTime  time.Time
	cmdOutputBuf  *bytes.Buffer
	outputCapture *outputTee
)

// outputTee captures output while still writing to the original destination.
type outputTee struct {
	writer io.Writer
	buffer *bytes.Buffer
}

func (t *outputTee) Write(p []byte) (n int, err error) {
	t.buffer.Write(p)
	return t.writer.Write(p)
}

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "penf",
	Short: "Penfold CLI - Personal information system interface",
	Long: `penf is the command-line interface for the Penfold personal information system.

Penfold aggregates and correlates information from communication channels
(email, Slack, documents, meetings) into a queryable institutional memory.

DESIGNED FOR AI ASSISTANTS:
  This CLI is optimized for AI assistants (like Claude Code). Commands support
  --output json for structured data. Run 'penf <command> --help' to discover
  subcommands, flags, and examples.

COMMON WORKFLOWS:
  Query knowledge:  penf search "topic"  |  penf ai query "question"
  Project briefing: penf briefing "project name"
  Check system:     penf health -e  →  penf pipeline status
  Ingest content:   penf ingest email ./dir  →  penf process onboarding context
  Review entities:  penf review start  →  penf review queue  →  penf review accept <id>
  Manage people:    penf relationship entity list  →  penf trust set  →  penf seniority set

DISCOVERY:
  penf <command> --help       Subcommands, flags, and examples for any command
  penf health -e              System health with pipeline statistics
  penf pipeline status        Processing pipeline overview
  penf debug info             Full diagnostic information`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Record start time for command logging.
		cmdStartTime = time.Now()

		// Set up output capture for command logging.
		cmdOutputBuf = &bytes.Buffer{}
		outputCapture = &outputTee{writer: os.Stdout, buffer: cmdOutputBuf}

		// Skip initialization for commands that don't need it.
		if cmd.Name() == "version" || cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}

		// Load configuration.
		var err error
		cfg, err = config.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading configuration: %w", err)
		}

		// Override with command-line flags.
		if serverAddr != "" {
			cfg.ServerAddress = serverAddr
		}
		if timeout != 0 {
			cfg.Timeout = timeout
		}
		if outputFormat != "" {
			cfg.OutputFormat = config.OutputFormat(outputFormat)
		}
		if debug {
			cfg.Debug = true
		}
		if insecure {
			cfg.Insecure = true
		}

		// Set output capture on the command if Context-Palace is configured.
		if cfg.ContextPalace != nil && cfg.ContextPalace.IsConfigured() {
			cmd.SetOut(outputCapture)
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		// Clean up gRPC client if initialized.
		if grpcClient != nil {
			return grpcClient.Close()
		}
		return nil
	},
}

// Version command flags.
var (
	versionAll        bool
	versionOutputJSON bool
	versionChangelog  bool
)

// versionCmd prints version information.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long: `Print the version, commit hash, and build time of the penf CLI.

Use --all to query all service versions.
Use --changelog to show commits since the last tag.
Use --output json for machine-readable output.

Examples:
  penf version                      Show CLI version only
  penf version --all                Show all service versions
  penf version --changelog          Show commits since last tag
  penf version --changelog --output json  Output changelog as JSON
  penf version --all --output json  Output as JSON`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Always get local CLI version first.
		info := buildinfo.Get("penf-cli")

		// If --changelog is set, show commits since last tag.
		if versionChangelog {
			// Get the last tag.
			tagCmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
			tagOut, err := tagCmd.Output()
			lastTag := strings.TrimSpace(string(tagOut))
			if err != nil || lastTag == "" {
				lastTag = "" // No tags, show all commits
			}

			// Get commits since last tag (or all if no tag).
			var logCmd *exec.Cmd
			if lastTag != "" {
				logCmd = exec.Command("git", "log", "--oneline", lastTag+"..HEAD")
			} else {
				logCmd = exec.Command("git", "log", "--oneline")
			}

			logOut, err := logCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get git log: %w", err)
			}

			changelog := strings.TrimSpace(string(logOut))

			// Handle --output-json mode.
			if versionOutputJSON {
				type commit struct {
					Hash    string `json:"hash"`
					Message string `json:"message"`
				}
				commits := []commit{}
				if changelog != "" {
					lines := strings.Split(changelog, "\n")
					for _, line := range lines {
						fields := strings.SplitN(line, " ", 2)
						if len(fields) == 2 {
							commits = append(commits, commit{
								Hash:    fields[0],
								Message: fields[1],
							})
						}
					}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(commits)
			}

			// Text output.
			out := cmd.OutOrStdout()
			if changelog == "" {
				fmt.Fprintln(out, "No commits since last tag.")
			} else {
				fmt.Fprintln(out, changelog)
			}
			return nil
		}

		if !versionAll {
			// Just print local version.
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "penf version %s\n", info.Version)
			fmt.Fprintf(out, "  commit:     %s\n", info.Commit)
			fmt.Fprintf(out, "  built:      %s\n", info.BuildTime)
			return nil
		}

		// Query all services.
		services := []struct {
			Name string
			URL  string
		}{
			{"penfold-gateway", "http://dev02.brown.chat:8080"},
			{"penfold-worker", "http://dev01.brown.chat:8085"},
			{"penfold-ai-coordinator", "http://dev02.brown.chat:8090"},
		}

		type result struct {
			Info buildinfo.Info
			Err  error
		}

		results := []result{{Info: info}} // CLI first

		httpClient := &http.Client{Timeout: 5 * time.Second}
		for _, svc := range services {
			resp, err := httpClient.Get(svc.URL + "/version")
			if err != nil {
				results = append(results, result{
					Info: buildinfo.Info{ServiceName: svc.Name, Version: "unreachable"},
					Err:  err,
				})
				continue
			}
			defer resp.Body.Close()

			var svcInfo buildinfo.Info
			if err := json.NewDecoder(resp.Body).Decode(&svcInfo); err != nil {
				results = append(results, result{
					Info: buildinfo.Info{ServiceName: svc.Name, Version: "error"},
					Err:  err,
				})
				continue
			}
			results = append(results, result{Info: svcInfo})
		}

		if versionOutputJSON {
			infos := make([]buildinfo.Info, len(results))
			for i, r := range results {
				infos[i] = r.Info
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(infos)
		}

		// Table format.
		fmt.Printf("%-25s %-12s %-10s %s\n", "SERVICE", "VERSION", "COMMIT", "BUILT")
		for _, r := range results {
			version := r.Info.Version
			commit := r.Info.Commit
			built := r.Info.BuildTime
			if r.Err != nil {
				version = "unreachable"
				commit = "-"
				built = "-"
			}
			if len(commit) > 10 {
				commit = commit[:10]
			}
			if len(built) > 20 {
				built = built[:20]
			}
			fmt.Printf("%-25s %-12s %-10s %s\n", r.Info.ServiceName, version, commit, built)
		}

		return nil
	},
}

// statusCmd checks the connection status to the API Gateway.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check connection status to the API Gateway",
	Long: `Check the connection status to the Penfold API Gateway.

This is a lightweight connectivity check — it verifies the gRPC connection
is established and responsive. For comprehensive system health including
database, queues, pipeline stats, and ML services, use 'penf health' instead.

Examples:
  penf status                Quick connection check
  penf health                Full system health check
  penf health -e             Health with pipeline statistics
  penf health -e -f          Health with functional inference tests`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize client.
		if err := initClient(); err != nil {
			return err
		}

		// Create context with timeout.
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()

		// Perform health check.
		if err := grpcClient.HealthCheck(ctx); err != nil {
			fmt.Printf("Connection status: UNHEALTHY\n")
			fmt.Printf("  Server:  %s\n", cfg.ServerAddress)
			fmt.Printf("  State:   %s\n", grpcClient.ConnectionState())
			fmt.Printf("  Error:   %s\n", err)
			return nil // Don't return error, just report status.
		}

		fmt.Printf("Connection status: HEALTHY\n")
		fmt.Printf("  Server:  %s\n", cfg.ServerAddress)
		fmt.Printf("  State:   %s\n", grpcClient.ConnectionState())
		return nil
	},
}

// configCmd manages CLI configuration.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long:  `View and modify the penf CLI configuration settings.`,
}

// configShowCmd displays current configuration.
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current CLI configuration values.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config (uses PersistentPreRunE, so cfg is already loaded).
		if cfg == nil {
			var err error
			cfg, err = config.LoadConfig()
			if err != nil {
				return fmt.Errorf("loading configuration: %w", err)
			}
		}

		configPath, _ := config.ConfigPath()

		fmt.Println("Current configuration:")
		fmt.Printf("  Config file:    %s\n", configPath)
		fmt.Printf("  Server address: %s\n", cfg.ServerAddress)
		fmt.Printf("  Timeout:        %s\n", cfg.Timeout)
		fmt.Printf("  Output format:  %s\n", cfg.OutputFormat)
		fmt.Printf("  Tenant ID:      %s\n", valueOrDefault(cfg.TenantID, "(not set)"))
		fmt.Printf("  Debug:          %t\n", cfg.Debug)
		fmt.Printf("  Insecure:       %t\n", cfg.Insecure)

		return nil
	},
}

// configInitCmd initializes configuration.
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	Long:  `Create a new configuration file with default values if one doesn't exist.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, err := config.ConfigPath()
		if err != nil {
			return fmt.Errorf("getting config path: %w", err)
		}

		// Check if config already exists.
		if _, err := os.Stat(configPath); err == nil {
			fmt.Printf("Configuration file already exists: %s\n", configPath)
			fmt.Println("Use 'penf config show' to view current settings.")
			return nil
		}

		// Create default config.
		defaultCfg := config.DefaultConfig()
		if err := config.SaveConfig(defaultCfg); err != nil {
			return fmt.Errorf("saving configuration: %w", err)
		}

		fmt.Printf("Created configuration file: %s\n", configPath)
		fmt.Println("\nDefault settings:")
		fmt.Printf("  Server address: %s\n", defaultCfg.ServerAddress)
		fmt.Printf("  Timeout:        %s\n", defaultCfg.Timeout)
		fmt.Printf("  Output format:  %s\n", defaultCfg.OutputFormat)

		return nil
	},
}

// configSetCmd sets a configuration value.
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value in the config file.

Available keys:
  server_address  - API Gateway server address (host:port)
  timeout         - Request timeout (e.g., 30s, 1m)
  output_format   - Default output format (text, json, yaml)
  tenant_id       - Default tenant ID
  install_path    - Path for penf binary updates (supports ~)
  debug           - Enable debug mode (true/false)
  insecure        - Disable TLS verification (true/false)

Examples:
  penf config set server_address localhost:50051
  penf config set timeout 1m
  penf config set output_format json
  penf config set tenant_id my-tenant-123
  penf config set install_path ~/bin/penf`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		// Load current config.
		currentCfg, err := config.LoadConfig()
		if err != nil {
			// If config doesn't exist, start with defaults.
			currentCfg = config.DefaultConfig()
		}

		// Set the value.
		switch key {
		case "server_address":
			currentCfg.ServerAddress = value
		case "timeout":
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("invalid timeout value: %w", err)
			}
			currentCfg.Timeout = duration
		case "output_format":
			format := config.OutputFormat(value)
			if !format.IsValid() {
				return fmt.Errorf("invalid output format: %s (must be text, json, or yaml)", value)
			}
			currentCfg.OutputFormat = format
		case "tenant_id":
			currentCfg.TenantID = value
		case "install_path":
			// Validate the path is expandable.
			expanded, err := config.ExpandPath(value)
			if err != nil {
				return fmt.Errorf("invalid install path: %w", err)
			}
			// Store the original value (with ~) for readability.
			currentCfg.InstallPath = value
			fmt.Printf("  (expands to: %s)\n", expanded)
		case "debug":
			if value == "true" || value == "1" {
				currentCfg.Debug = true
			} else if value == "false" || value == "0" {
				currentCfg.Debug = false
			} else {
				return fmt.Errorf("invalid debug value: %s (must be true or false)", value)
			}
		case "insecure":
			if value == "true" || value == "1" {
				currentCfg.Insecure = true
			} else if value == "false" || value == "0" {
				currentCfg.Insecure = false
			} else {
				return fmt.Errorf("invalid insecure value: %s (must be true or false)", value)
			}
		default:
			return fmt.Errorf("unknown configuration key: %s", key)
		}

		// Save the config.
		if err := config.SaveConfig(currentCfg); err != nil {
			return fmt.Errorf("saving configuration: %w", err)
		}

		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

// completionCmd generates shell completion scripts.
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for penf.

To load completions:

Bash:
  $ source <(penf completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ penf completion bash > /etc/bash_completion.d/penf
  # macOS:
  $ penf completion bash > $(brew --prefix)/etc/bash_completion.d/penf

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. Execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ penf completion zsh > "${fpath[1]}/_penf"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ penf completion fish | source

  # To load completions for each session, execute once:
  $ penf completion fish > ~/.config/fish/completions/penf.fish

PowerShell:
  PS> penf completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> penf completion powershell > penf.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

// Health command flags.
var (
	healthWatch         bool
	healthWatchInterval time.Duration
	healthExtended      bool
	healthFunctional    bool
)

// healthCmd checks system health status.
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check system health status",
	Long: `Check the health status of the Penfold system.

Displays the status of all services, database connections, and queue depths.

Flags:
  --extended, -e   Include pipeline statistics (sources, embeddings, jobs by status)
  --functional, -f Run functional inference tests (actual embedding/LLM calls)
  --watch, -w      Continuously monitor health status
  --json           Output as JSON for machine processing

Examples:
  penf health                # Basic health check
  penf health -e             # Include pipeline stats
  penf health -e -f          # Full check with inference tests
  penf health -w             # Watch mode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize client.
		if err := initClient(); err != nil {
			return err
		}

		ctx := cmd.Context()

		if healthWatch {
			return runHealthWatch(ctx)
		}

		return runHealthOnce(ctx)
	},
}

// ExtendedHealthStatus combines system status with pipeline stats and functional tests.
type ExtendedHealthStatus struct {
	*client.SystemStatus
	Pipeline    *PipelineStats    `json:"pipeline,omitempty"`
	Functional  *FunctionalTests  `json:"functional,omitempty"`
	WorkerIdle  *WorkerIdleStatus `json:"worker_idle,omitempty"`
}

// PipelineStats holds pipeline statistics from GetStats.
type PipelineStats struct {
	SourcesTotal      int64            `json:"sources_total"`
	SourcesByStatus   map[string]int64 `json:"sources_by_status"`
	EmbeddingsTotal   int64            `json:"embeddings_total"`
	EmbeddingsRecent  int64            `json:"embeddings_recent"`
	JobsTotal         int64            `json:"jobs_total"`
	JobsByStatus      map[string]int64 `json:"jobs_by_status"`
}

// FunctionalTests holds results of functional inference tests.
type FunctionalTests struct {
	Embeddings *FunctionalTestResult `json:"embeddings,omitempty"`
	LLM        *FunctionalTestResult `json:"llm,omitempty"`
}

// FunctionalTestResult holds a single functional test result.
type FunctionalTestResult struct {
	Healthy   bool    `json:"healthy"`
	LatencyMs float64 `json:"latency_ms"`
	Message   string  `json:"message,omitempty"`
	Error     string  `json:"error,omitempty"`
}

// WorkerIdleStatus tracks if the worker is idle while items are pending.
type WorkerIdleStatus struct {
	IsIdle       bool  `json:"is_idle"`
	PendingCount int64 `json:"pending_count"`
	Message      string `json:"message"`
}

// runHealthOnce performs a single health check and outputs results.
func runHealthOnce(ctx context.Context) error {
	// Create context with timeout.
	checkCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	status, err := grpcClient.GetStatus(checkCtx, false)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	// If no extended or functional flags, just output basic status.
	if !healthExtended && !healthFunctional {
		return outputStatus(status)
	}

	// Build extended status.
	extStatus := &ExtendedHealthStatus{
		SystemStatus: status,
	}

	// Fetch pipeline stats if extended.
	if healthExtended {
		pipelineStats, workerIdle, err := fetchPipelineStats(checkCtx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get pipeline stats: %v\n", err)
		} else {
			extStatus.Pipeline = pipelineStats
			extStatus.WorkerIdle = workerIdle
		}
	}

	// Run functional tests if requested.
	if healthFunctional {
		extStatus.Functional = runFunctionalTests(checkCtx)
	}

	return outputExtendedStatus(extStatus)
}

// fetchPipelineStats fetches pipeline statistics via gRPC.
func fetchPipelineStats(ctx context.Context) (*PipelineStats, *WorkerIdleStatus, error) {
	resp, err := grpcClient.GetStats(ctx, cfg.TenantID)
	if err != nil {
		return nil, nil, err
	}

	stats := resp.GetStats()
	if stats == nil {
		return nil, nil, fmt.Errorf("empty stats response")
	}

	pStats := &PipelineStats{
		SourcesTotal:     stats.GetSourcesTotal(),
		SourcesByStatus:  make(map[string]int64),
		EmbeddingsTotal:  stats.GetEmbeddingsTotal(),
		EmbeddingsRecent: stats.GetEmbeddingsRecent(),
		JobsTotal:        stats.GetJobsTotal(),
		JobsByStatus:     make(map[string]int64),
	}

	for _, sc := range stats.GetSourcesByStatus() {
		pStats.SourcesByStatus[sc.GetStatus()] = sc.GetCount()
	}

	for _, jc := range stats.GetJobsByStatus() {
		pStats.JobsByStatus[jc.GetStatus()] = jc.GetCount()
	}

	// Check for worker idle condition.
	var workerIdle *WorkerIdleStatus
	pendingCount := pStats.SourcesByStatus["pending"]
	// If there are pending items but no recent embeddings, worker may be idle.
	if pendingCount > 0 && pStats.EmbeddingsRecent == 0 {
		workerIdle = &WorkerIdleStatus{
			IsIdle:       true,
			PendingCount: pendingCount,
			Message:      fmt.Sprintf("Worker may be idle: %d pending items but no embeddings in last hour", pendingCount),
		}
	}

	return pStats, workerIdle, nil
}

// runFunctionalTests executes actual inference calls to verify ML services.
func runFunctionalTests(ctx context.Context) *FunctionalTests {
	tests := &FunctionalTests{}

	// Get service URLs from config or environment.
	embeddingsURL := os.Getenv("GATEWAY_EMBEDDINGS_URL")
	if embeddingsURL == "" {
		embeddingsURL = "http://dev01.brown.chat:8081"
	}

	llmURL := os.Getenv("GATEWAY_LLM_URL")
	if llmURL == "" {
		llmURL = "http://dev01.brown.chat:8080"
	}

	// Test embeddings.
	tests.Embeddings = testEmbeddings(ctx, embeddingsURL)

	// Test LLM.
	tests.LLM = testLLM(ctx, llmURL)

	return tests
}

// testEmbeddings sends a test embedding request.
func testEmbeddings(ctx context.Context, baseURL string) *FunctionalTestResult {
	url := baseURL + "/v1/embeddings"
	payload := `{"input": "test"}`

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(payload))
	if err != nil {
		return &FunctionalTestResult{Healthy: false, Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return &FunctionalTestResult{Healthy: false, LatencyMs: float64(latency.Milliseconds()), Error: err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return &FunctionalTestResult{
			Healthy:   false,
			LatencyMs: float64(latency.Milliseconds()),
			Error:     fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
		}
	}

	// Parse response to get embedding dimensions.
	var embResp struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &embResp) == nil && len(embResp.Data) > 0 {
		dims := len(embResp.Data[0].Embedding)
		return &FunctionalTestResult{
			Healthy:   true,
			LatencyMs: float64(latency.Milliseconds()),
			Message:   fmt.Sprintf("%d dimensions", dims),
		}
	}

	return &FunctionalTestResult{Healthy: true, LatencyMs: float64(latency.Milliseconds())}
}

// testLLM sends a test chat completion request.
func testLLM(ctx context.Context, baseURL string) *FunctionalTestResult {
	url := baseURL + "/v1/chat/completions"
	payload := `{"messages": [{"role": "user", "content": "Say OK"}], "max_tokens": 5}`

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(payload))
	if err != nil {
		return &FunctionalTestResult{Healthy: false, Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return &FunctionalTestResult{Healthy: false, LatencyMs: float64(latency.Milliseconds()), Error: err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return &FunctionalTestResult{
			Healthy:   false,
			LatencyMs: float64(latency.Milliseconds()),
			Error:     fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
		}
	}

	// Parse response to get completion.
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(body, &chatResp) == nil && len(chatResp.Choices) > 0 {
		content := chatResp.Choices[0].Message.Content
		if len(content) > 20 {
			content = content[:20] + "..."
		}
		return &FunctionalTestResult{
			Healthy:   true,
			LatencyMs: float64(latency.Milliseconds()),
			Message:   fmt.Sprintf("response: %q", content),
		}
	}

	return &FunctionalTestResult{Healthy: true, LatencyMs: float64(latency.Milliseconds())}
}

// outputExtendedStatus outputs the extended health status.
func outputExtendedStatus(status *ExtendedHealthStatus) error {
	format := cfg.OutputFormat
	if outputFormat != "" {
		format = config.OutputFormat(outputFormat)
	}

	switch format {
	case config.OutputFormatJSON:
		return outputJSON(status)
	case config.OutputFormatYAML:
		return outputYAML(status)
	default:
		return outputExtendedHealthHuman(status)
	}
}

// outputExtendedHealthHuman outputs extended health status in human-readable format.
func outputExtendedHealthHuman(status *ExtendedHealthStatus) error {
	// First output the base health info.
	if err := outputHealthHuman(status.SystemStatus); err != nil {
		return err
	}

	// Pipeline stats.
	if status.Pipeline != nil {
		p := status.Pipeline
		fmt.Println("Pipeline:")
		fmt.Printf("  Sources: %d total\n", p.SourcesTotal)
		if len(p.SourcesByStatus) > 0 {
			fmt.Print("    ")
			parts := make([]string, 0, len(p.SourcesByStatus))
			for status, count := range p.SourcesByStatus {
				color := "\033[0m"
				if status == "completed" {
					color = "\033[32m"
				} else if status == "failed" || status == "rejected" {
					color = "\033[31m"
				} else if status == "pending" {
					color = "\033[33m"
				}
				parts = append(parts, fmt.Sprintf("%s%s: %d\033[0m", color, status, count))
			}
			fmt.Println(strings.Join(parts, ", "))
		}
		fmt.Printf("  Embeddings: %d total, %d in last hour\n", p.EmbeddingsTotal, p.EmbeddingsRecent)
		fmt.Printf("  Jobs: %d total\n", p.JobsTotal)
		if len(p.JobsByStatus) > 0 {
			fmt.Print("    ")
			parts := make([]string, 0, len(p.JobsByStatus))
			for status, count := range p.JobsByStatus {
				color := "\033[0m"
				if status == "completed" {
					color = "\033[32m"
				} else if status == "failed" {
					color = "\033[31m"
				} else if status == "pending" || status == "in_progress" {
					color = "\033[33m"
				}
				parts = append(parts, fmt.Sprintf("%s%s: %d\033[0m", color, status, count))
			}
			fmt.Println(strings.Join(parts, ", "))
		}
		fmt.Println()
	}

	// Worker idle warning.
	if status.WorkerIdle != nil && status.WorkerIdle.IsIdle {
		fmt.Printf("\033[33m⚠ %s\033[0m\n", status.WorkerIdle.Message)
		fmt.Println("  Run 'penf pipeline kick' to start processing.")
		fmt.Println()
	}

	// Functional tests.
	if status.Functional != nil {
		fmt.Println("Functional Tests:")
		f := status.Functional
		if f.Embeddings != nil {
			statusStr := "\033[32m✓\033[0m"
			if !f.Embeddings.Healthy {
				statusStr = "\033[31m✗\033[0m"
			}
			detail := f.Embeddings.Message
			if f.Embeddings.Error != "" {
				detail = f.Embeddings.Error
			}
			fmt.Printf("  %s Embeddings: %.0fms %s\n", statusStr, f.Embeddings.LatencyMs, detail)
		}
		if f.LLM != nil {
			statusStr := "\033[32m✓\033[0m"
			if !f.LLM.Healthy {
				statusStr = "\033[31m✗\033[0m"
			}
			detail := f.LLM.Message
			if f.LLM.Error != "" {
				detail = f.LLM.Error
			}
			fmt.Printf("  %s LLM: %.0fms %s\n", statusStr, f.LLM.LatencyMs, detail)
		}
		fmt.Println()
	}

	return nil
}

// runHealthWatch performs continuous health monitoring.
func runHealthWatch(ctx context.Context) error {
	ticker := time.NewTicker(healthWatchInterval)
	defer ticker.Stop()

	// Initial check.
	if err := runHealthOnce(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nStopped watching.")
			return nil
		case <-ticker.C:
			if outputFormat != "json" && outputFormat != "yaml" {
				// Clear screen for human-readable output.
				fmt.Print("\033[H\033[2J")
			}
			if err := runHealthOnce(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		}
	}
}

// outputStatus outputs the system status in the configured format.
func outputStatus(status *client.SystemStatus) error {
	format := cfg.OutputFormat
	if outputFormat != "" {
		format = config.OutputFormat(outputFormat)
	}

	switch format {
	case config.OutputFormatJSON:
		return outputJSON(status)
	case config.OutputFormatYAML:
		return outputYAML(status)
	default:
		return outputHealthHuman(status)
	}
}

// outputJSON outputs data as JSON.
func outputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// outputYAML outputs data as YAML.
func outputYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

// outputHealthHuman outputs health status in human-readable format.
func outputHealthHuman(status *client.SystemStatus) error {
	// Overall status with color.
	statusColor := "\033[32m" // Green
	if !status.Healthy {
		statusColor = "\033[31m" // Red
	}
	fmt.Printf("System Status: %s%s\033[0m\n", statusColor, boolToStatus(status.Healthy))
	fmt.Printf("Message: %s\n", status.Message)
	fmt.Printf("Timestamp: %s\n\n", status.Timestamp.Format(time.RFC3339))

	// Services.
	fmt.Println("Services:")
	fmt.Println("  NAME             STATUS     LATENCY    VERSION")
	fmt.Println("  ----             ------     -------    -------")
	for _, svc := range status.Services {
		statusStr := statusWithColor(svc.Healthy, svc.Status)
		latencyStr := "-"
		if svc.LatencyMs > 0 {
			latencyStr = fmt.Sprintf("%.1fms", svc.LatencyMs)
		}
		fmt.Printf("  %-16s %-10s %-10s %s\n", svc.Name, statusStr, latencyStr, svc.Version)
	}
	fmt.Println()

	// Database.
	if status.Database != nil {
		db := status.Database
		dbStatus := statusWithColor(db.Healthy, db.ConnectionStatus)
		fmt.Println("Database:")
		fmt.Printf("  Type: %s\n", db.Type)
		fmt.Printf("  Status: %s\n", dbStatus)
		fmt.Printf("  Connections: %d/%d\n", db.ActiveConnections, db.MaxConnections)
		fmt.Printf("  Vector Extension: %s\n", boolToEnabled(db.VectorExtensionEnabled))
		fmt.Printf("  Content Items: %d\n", db.ContentCount)
		fmt.Printf("  Entities: %d\n", db.EntityCount)
		fmt.Printf("  Latency: %.1fms\n", db.LatencyMs)
		fmt.Println()
	}

	// Queues.
	if status.Queues != nil {
		q := status.Queues
		queueStatus := statusWithColor(q.Healthy, "healthy")
		fmt.Println("Queues:")
		fmt.Printf("  Type: %s\n", q.Type)
		fmt.Printf("  Status: %s\n", queueStatus)
		fmt.Printf("  Total Pending: %d\n", q.TotalPending)
		fmt.Printf("  Processing Rate: %.1f/min\n", q.ProcessingRate)
		if q.DeadLetterCount > 0 {
			fmt.Printf("  Dead Letter: \033[33m%d\033[0m\n", q.DeadLetterCount)
		}
		if len(q.QueueDepths) > 0 {
			fmt.Println("  Queue Depths:")
			for name, depth := range q.QueueDepths {
				fmt.Printf("    %s: %d\n", name, depth)
			}
		}
		fmt.Println()
	}

	// Version info.
	if status.Version != nil {
		v := status.Version
		fmt.Println("Version:")
		fmt.Printf("  Version: %s\n", v.Version)
		fmt.Printf("  Commit: %s\n", v.Commit)
		fmt.Printf("  Build Time: %s\n", v.BuildTime)
		fmt.Printf("  Go Version: %s\n", v.GoVersion)
	}

	return nil
}

// boolToStatus converts a boolean to a status string.
func boolToStatus(healthy bool) string {
	if healthy {
		return "HEALTHY"
	}
	return "UNHEALTHY"
}

// boolToEnabled converts a boolean to an enabled/disabled string.
func boolToEnabled(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

// statusWithColor returns a colored status string.
func statusWithColor(healthy bool, status string) string {
	if healthy {
		if status == "" {
			return "\033[32mhealthy\033[0m"
		}
		return fmt.Sprintf("\033[32m%s\033[0m", status)
	}
	if status == "" {
		return "\033[31munhealthy\033[0m"
	}
	return fmt.Sprintf("\033[31m%s\033[0m", status)
}

// initClient initializes the gRPC client if not already initialized.
func initClient() error {
	if grpcClient != nil {
		return nil
	}

	// Override tenant ID from environment if set.
	if envTenant := getTenantID(); envTenant != "" {
		cfg.TenantID = envTenant
	}

	c, err := client.ConnectFromConfig(cfg)
	if err != nil {
		return err
	}
	grpcClient = c
	return nil
}

// getTenantID returns the current tenant ID from environment or config.
func getTenantID() string {
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant
	}
	if cfg != nil {
		return cfg.TenantID
	}
	return ""
}

// valueOrDefault returns the value if non-empty, otherwise the default.
func valueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func init() {
	// Global flags.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.penf/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", "", "API Gateway server address (host:port)")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 0, "request timeout (e.g., 30s, 1m)")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output", "", "output format: text, json, yaml")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "disable TLS verification")

	// Health command flags.
	healthCmd.Flags().BoolVarP(&healthWatch, "watch", "w", false, "Continuously monitor health status")
	healthCmd.Flags().DurationVar(&healthWatchInterval, "interval", 5*time.Second, "Watch interval (default 5s)")
	healthCmd.Flags().BoolVarP(&healthExtended, "extended", "e", false, "Include pipeline stats and database counts")
	healthCmd.Flags().BoolVarP(&healthFunctional, "functional", "f", false, "Run functional inference tests (embeddings, LLM)")

	// Health subcommands.
	healthCmd.AddCommand(cmd.NewHealthLocalCommand())
	healthCmd.AddCommand(cmd.NewHealthGatewayCommand())
	healthCmd.AddCommand(cmd.NewHealthPreflightCommand())

	// Add command groups for organized help output.
	rootCmd.AddGroup(
		&cobra.Group{ID: "query", Title: "Querying:"},
		&cobra.Group{ID: "content", Title: "Content & Pipeline:"},
		&cobra.Group{ID: "entities", Title: "Entities:"},
		&cobra.Group{ID: "people", Title: "People Attributes:"},
		&cobra.Group{ID: "review", Title: "Review & Triage:"},
		&cobra.Group{ID: "meetings", Title: "Meetings:"},
		&cobra.Group{ID: "ops", Title: "Operations:"},
		&cobra.Group{ID: "setup", Title: "Setup:"},
	)

	// Querying
	searchCmd := cmd.NewSearchCommand(nil)
	searchCmd.GroupID = "query"
	rootCmd.AddCommand(searchCmd)

	aiCmd := cmd.NewAICommand(nil)
	aiCmd.GroupID = "query"
	rootCmd.AddCommand(aiCmd)

	briefingCmd := cmd.NewBriefingCommand(nil)
	briefingCmd.GroupID = "query"
	rootCmd.AddCommand(briefingCmd)

	assertionsCmd := cmd.NewAssertionsCommand(nil)
	assertionsCmd.GroupID = "query"
	rootCmd.AddCommand(assertionsCmd)

	threadCmd := cmd.NewThreadCommand(nil)
	threadCmd.GroupID = "query"
	rootCmd.AddCommand(threadCmd)

	conversationCmd := cmd.NewConversationCommand(nil)
	conversationCmd.GroupID = "query"
	rootCmd.AddCommand(conversationCmd)

	// Content & Pipeline
	contentCmd := cmd.NewContentCommand(nil)
	contentCmd.GroupID = "content"
	rootCmd.AddCommand(contentCmd)

	pipelineCmd := cmd.NewPipelineCommand(nil)
	pipelineCmd.GroupID = "content"
	rootCmd.AddCommand(pipelineCmd)

	reprocessCmd := cmd.NewReprocessCommand(nil)
	reprocessCmd.GroupID = "content"
	rootCmd.AddCommand(reprocessCmd)

	ingestCmd := cmd.NewIngestCommand(nil)
	ingestCmd.GroupID = "content"
	rootCmd.AddCommand(ingestCmd)

	workflowCmd := cmd.NewWorkflowCommand(nil)
	workflowCmd.GroupID = "content"
	rootCmd.AddCommand(workflowCmd)

	classifyCmd := cmd.NewClassifyCommand(nil)
	classifyCmd.GroupID = "content"
	rootCmd.AddCommand(classifyCmd)

	// Entities
	relationshipCmd := cmd.NewRelationshipCommand(nil)
	relationshipCmd.GroupID = "entities"
	rootCmd.AddCommand(relationshipCmd)

	productCmd := cmd.NewProductCommand(nil)
	productCmd.GroupID = "entities"
	rootCmd.AddCommand(productCmd)

	projectCmd := cmd.NewProjectCommand(nil)
	projectCmd.GroupID = "entities"
	rootCmd.AddCommand(projectCmd)

	teamCmd := cmd.NewTeamCommand(nil)
	teamCmd.GroupID = "entities"
	rootCmd.AddCommand(teamCmd)

	glossaryCmd := cmd.NewGlossaryCommand(nil)
	glossaryCmd.GroupID = "entities"
	rootCmd.AddCommand(glossaryCmd)

	entityCmd := cmd.NewEntityCommand(nil)
	entityCmd.GroupID = "entities"
	rootCmd.AddCommand(entityCmd)

	// People Attributes
	trustCmd := cmd.NewTrustCommand(nil)
	trustCmd.GroupID = "people"
	rootCmd.AddCommand(trustCmd)

	seniorityCmd := cmd.NewSeniorityCommand(nil)
	seniorityCmd.GroupID = "people"
	rootCmd.AddCommand(seniorityCmd)

	// Review & Triage
	reviewCmd := cmd.NewReviewCommand(nil)
	reviewCmd.GroupID = "review"
	rootCmd.AddCommand(reviewCmd)

	processCmd := cmd.NewProcessCommand(nil)
	processCmd.GroupID = "review"
	rootCmd.AddCommand(processCmd)

	watchCmd := cmd.NewWatchCommand(nil)
	watchCmd.GroupID = "review"
	rootCmd.AddCommand(watchCmd)

	escalationsCmd := cmd.NewEscalationsCommand(nil)
	escalationsCmd.GroupID = "review"
	rootCmd.AddCommand(escalationsCmd)

	auditCmd := cmd.NewAuditCommand(nil)
	auditCmd.GroupID = "review"
	rootCmd.AddCommand(auditCmd)

	// Meetings
	meetingCmd := cmd.NewMeetingCommand(nil)
	meetingCmd.GroupID = "meetings"
	rootCmd.AddCommand(meetingCmd)

	// Operations
	healthCmd.GroupID = "ops"
	rootCmd.AddCommand(healthCmd)

	statusCmd.GroupID = "ops"
	rootCmd.AddCommand(statusCmd)

	dbCmd := cmd.NewDbCommand()
	dbCmd.GroupID = "ops"
	rootCmd.AddCommand(dbCmd)

	logsCmd := cmd.NewLogsCommand(nil)
	logsCmd.GroupID = "ops"
	rootCmd.AddCommand(logsCmd)

	deployCmd := cmd.NewDeployCommand()
	deployCmd.GroupID = "ops"
	rootCmd.AddCommand(deployCmd)

	modelCmd := cmd.NewModelCommand(nil)
	modelCmd.GroupID = "ops"
	rootCmd.AddCommand(modelCmd)

	traceCmd := cmd.NewTraceCommand(nil)
	traceCmd.GroupID = "ops"
	rootCmd.AddCommand(traceCmd)

	debugCmd := cmd.NewDebugCommand(nil)
	debugCmd.GroupID = "ops"
	rootCmd.AddCommand(debugCmd)

	qualityCmd := cmd.NewQualityCommand(nil)
	qualityCmd.GroupID = "ops"
	rootCmd.AddCommand(qualityCmd)

	// Setup
	configCmd.GroupID = "setup"
	rootCmd.AddCommand(configCmd)

	cmd.AuthCmd.GroupID = "setup"
	rootCmd.AddCommand(cmd.AuthCmd)

	certCmd := cmd.NewCertCommand()
	certCmd.GroupID = "setup"
	rootCmd.AddCommand(certCmd)

	tenantCmd := cmd.NewTenantCommand(nil)
	tenantCmd.GroupID = "setup"
	rootCmd.AddCommand(tenantCmd)

	initCmd := cmd.NewInitCommand()
	initCmd.GroupID = "setup"
	rootCmd.AddCommand(initCmd)

	updateCmd := cmd.NewUpdateCommand(buildinfo.Version)
	updateCmd.GroupID = "setup"
	rootCmd.AddCommand(updateCmd)

	feedbackCmd := cmd.NewFeedbackCommand(buildinfo.Version)
	feedbackCmd.GroupID = "setup"
	rootCmd.AddCommand(feedbackCmd)

	completionCmd.GroupID = "setup"
	rootCmd.AddCommand(completionCmd)

	versionCmd.GroupID = "setup"
	versionCmd.Flags().BoolVar(&versionAll, "all", false, "Query all service versions")
	versionCmd.Flags().BoolVar(&versionOutputJSON, "output-json", false, "Output as JSON")
	versionCmd.Flags().BoolVar(&versionChangelog, "changelog", false, "Show commits since last tag")
	rootCmd.AddCommand(versionCmd)

	// Config subcommands.
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configSetCmd)
}

func main() {
	// Set up signal handling for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
		if grpcClient != nil {
			_ = grpcClient.Close()
		}
		os.Exit(0)
	}()

	// Execute root command and capture the error for logging.
	cmdErr := rootCmd.ExecuteContext(ctx)

	// Log the command to Context-Palace (called here to capture both success and failure).
	logCommandExecution(os.Args, cmdErr)

	if cmdErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", cmdErr)
		os.Exit(1)
	}
}

// logCommandExecution logs the CLI command to Context-Palace.
// This is best-effort - errors are logged to stderr but don't affect the command result.
func logCommandExecution(args []string, cmdErr error) {
	// Skip if config not loaded or Context-Palace not configured.
	if cfg == nil || cfg.ContextPalace == nil || !cfg.ContextPalace.IsConfigured() {
		return
	}

	// Skip logging for certain commands.
	if len(args) > 1 {
		cmd := args[1]
		if cmd == "version" || cmd == "help" || cmd == "completion" || cmd == "context" {
			return
		}
	}

	// Calculate duration.
	duration := time.Since(cmdStartTime)

	// Build the command entry.
	entry := &contextpalace.CommandEntry{
		Command:     getCommandName(args),
		Args:        getCommandArgs(args),
		FullCommand: strings.Join(args, " "),
		DurationMs:  int(duration.Milliseconds()),
		Success:     cmdErr == nil,
		TenantID:    cfg.TenantID,
	}

	// Capture error message if command failed.
	if cmdErr != nil {
		entry.ErrorMessage = cmdErr.Error()
	}

	// Capture response output.
	if cmdOutputBuf != nil {
		entry.Response = cmdOutputBuf.String()
	}

	// Connect to Context-Palace and log.
	cpClient, err := contextpalace.NewClient(cfg.ContextPalace)
	if err != nil {
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "Warning: failed to connect to Context-Palace: %v\n", err)
		}
		return
	}
	defer cpClient.Close()

	logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cpClient.LogCommand(logCtx, entry); err != nil {
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "Warning: failed to log command to Context-Palace: %v\n", err)
		}
	}
}

// getCommandName extracts the command name from args (e.g., "search" from ["penf", "search", "query"]).
func getCommandName(args []string) string {
	if len(args) < 2 {
		return "penf"
	}
	// Find the first non-flag argument after "penf".
	for i := 1; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "-") {
			return args[i]
		}
	}
	return "penf"
}

// getCommandArgs extracts the arguments after the command name.
func getCommandArgs(args []string) []string {
	if len(args) < 3 {
		return nil
	}
	// Find the command name index and return everything after it.
	for i := 1; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "-") {
			return args[i+1:]
		}
	}
	return nil
}
