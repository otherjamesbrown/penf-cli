// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"

	aiv1 "github.com/otherjamesbrown/penf-cli/api/proto/ai/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// ModelProfile represents a configured LLM model profile.
type ModelProfile struct {
	Name            string `json:"name" yaml:"name"`
	Type            string `json:"type" yaml:"type"`
	Model           string `json:"model" yaml:"model"`
	Endpoint        string `json:"endpoint" yaml:"endpoint"`
	APIKeyFile      string `json:"api_key_file,omitempty" yaml:"api_key_file,omitempty"`
	ExpectedLatency string `json:"expected_latency,omitempty" yaml:"expected_latency,omitempty"`
	DownloadSize    string `json:"download_size,omitempty" yaml:"download_size,omitempty"`
	Notes           string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// ModelServerStatus represents the status of a running model server.
type ModelServerStatus struct {
	PID       int       `json:"pid" yaml:"pid"`
	Model     string    `json:"model" yaml:"model"`
	Port      int       `json:"port" yaml:"port"`
	CPUPct    float64   `json:"cpu_pct" yaml:"cpu_pct"`
	MemPct    float64   `json:"mem_pct" yaml:"mem_pct"`
	StartedAt time.Time `json:"started_at" yaml:"started_at"`
	Healthy   bool      `json:"healthy" yaml:"healthy"`
}

// ModelCatalogEntry represents a model in the catalog.
type ModelCatalogEntry struct {
	ID              string `json:"id" yaml:"id"`
	Name            string `json:"name" yaml:"name"`
	Size            string `json:"size" yaml:"size"`
	Type            string `json:"type" yaml:"type"` // mlx, gguf, etc.
	Downloaded      bool   `json:"downloaded" yaml:"downloaded"`
	ExpectedLatency string `json:"expected_latency,omitempty" yaml:"expected_latency,omitempty"`
	MemoryRequired  string `json:"memory_required,omitempty" yaml:"memory_required,omitempty"`
}

// ModelConfig holds the model configuration from llm-models.yaml.
type ModelConfig struct {
	DefaultProfile string                  `yaml:"default_profile"`
	Profiles       map[string]ModelProfile `yaml:"profiles"`
}

// Model command flags.
var (
	modelOutput string
	modelPort   int
)

// Default model catalog (built-in known models).
var defaultModelCatalog = []ModelCatalogEntry{
	{
		ID:              "mlx-community/Qwen2.5-7B-Instruct-4bit",
		Name:            "Qwen 2.5 7B (4-bit)",
		Size:            "~4GB",
		Type:            "mlx",
		ExpectedLatency: "3-8s",
		MemoryRequired:  "6GB+",
	},
	{
		ID:              "mlx-community/Phi-3.5-mini-instruct-4bit",
		Name:            "Phi-3.5 Mini (4-bit)",
		Size:            "~2.5GB",
		Type:            "mlx",
		ExpectedLatency: "1-4s",
		MemoryRequired:  "4GB+",
	},
	{
		ID:              "mlx-community/Llama-3.2-3B-Instruct-4bit",
		Name:            "Llama 3.2 3B (4-bit)",
		Size:            "~2GB",
		Type:            "mlx",
		ExpectedLatency: "1-3s",
		MemoryRequired:  "3GB+",
	},
	{
		ID:              "mlx-community/gemma-2-9b-it-4bit",
		Name:            "Gemma 2 9B (4-bit)",
		Size:            "~6GB",
		Type:            "mlx",
		ExpectedLatency: "3-8s",
		MemoryRequired:  "8GB+",
	},
}

// ModelCommandDeps holds the dependencies for model commands.
type ModelCommandDeps struct {
	Config      *config.CLIConfig
	LoadConfig  func() (*config.CLIConfig, error)
	ConfigDir   string
	SidecarDir  string
	GatewayAddr string // Gateway gRPC address for AI service
}

// DefaultModelDeps returns the default dependencies for production use.
func DefaultModelDeps() *ModelCommandDeps {
	homeDir, _ := os.UserHomeDir()
	return &ModelCommandDeps{
		LoadConfig: config.LoadConfig,
		ConfigDir:  filepath.Join(homeDir, ".config", "penfold"),
		SidecarDir: filepath.Join(homeDir, "github", "otherjamesbrown", "penfold", "penfold-go-pipeline", "sidecar"),
	}
}

// NewModelCommand creates the root model command with all subcommands.
func NewModelCommand(deps *ModelCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultModelDeps()
	}

	cmd := &cobra.Command{
		Use:   "model",
		Short: "Manage AI models (local + remote)",
		Long: `Manage AI models for Penfold - both local MLX models and remote API models.

The model command provides tools to manage the AI models used for
entity extraction, mention resolution, embeddings, summarization, and
other AI-powered features.

Model Types:
  - Local models: MLX models running on your machine (Ollama, MLX sidecar)
  - Remote models: Cloud API models (Gemini, OpenAI, Anthropic)

Commands:
  list       List downloaded local models
  registry   List all registered models (local + remote) from AI service
  add        Register a new remote model
  enable     Enable a registered model
  disable    Disable a registered model
  rules      Show model routing configuration
  status     Show running local model servers
  serve      Start a local model server
  stop       Stop local model server(s)

Examples:
  # List local downloaded models
  penf model list

  # Show all registered models (local + remote)
  penf model registry

  # Show only remote models
  penf model registry --remote

  # Register a new remote model
  penf model add gemini gemini-2.0-flash

  # Enable/disable a model
  penf model enable <model-id>
  penf model disable <model-id>

  # Show routing rules
  penf model rules

  # Start a local model server
  penf model serve phi --port 8080

Model configuration details are in Context Palace knowledge shards.`,
		Aliases: []string{"models", "llm"},
	}

	// Persistent flags.
	cmd.PersistentFlags().StringVarP(&modelOutput, "output", "o", "", "Output format: text, json, yaml")

	// Add subcommands - local model management.
	cmd.AddCommand(newModelListCommand(deps))
	cmd.AddCommand(newModelStatusCommand(deps))
	cmd.AddCommand(newModelServeCommand(deps))
	cmd.AddCommand(newModelStopCommand(deps))
	cmd.AddCommand(newModelSwitchCommand(deps))
	cmd.AddCommand(newModelInfoCommand(deps))
	cmd.AddCommand(newModelBenchCommand(deps))
	cmd.AddCommand(newModelDownloadCommand(deps))

	// Add subcommands - registry management (local + remote).
	cmd.AddCommand(newModelRegistryCommand(deps))
	cmd.AddCommand(newModelAddCommand(deps))
	cmd.AddCommand(newModelEnableCommand(deps))
	cmd.AddCommand(newModelDisableCommand(deps))
	cmd.AddCommand(newModelRulesCommand(deps))

	return cmd
}

// newModelListCommand creates the 'model list' subcommand.
func newModelListCommand(deps *ModelCommandDeps) *cobra.Command {
	var showAll bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available models",
		Long: `List available LLM models.

Shows both downloaded models and models available for download.
By default, only shows downloaded models. Use --all to show all available models.

Examples:
  penf model list
  penf model list --all
  penf model list -o json`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelList(cmd.Context(), deps, showAll)
		},
	}

	cmd.Flags().BoolVar(&showAll, "all", false, "Show all available models (including not downloaded)")

	return cmd
}

// newModelStatusCommand creates the 'model status' subcommand.
func newModelStatusCommand(deps *ModelCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show running model servers",
		Long: `Display the status of running MLX model servers.

Shows all running mlx_lm.server processes with their model, port,
CPU/memory usage, and health status.

Examples:
  penf model status
  penf model status -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelStatus(cmd.Context(), deps)
		},
	}
}

// newModelServeCommand creates the 'model serve' subcommand.
func newModelServeCommand(deps *ModelCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve <model>",
		Short: "Start a model server",
		Long: `Start an MLX model server.

Starts a new mlx_lm.server process serving the specified model.
The model can be specified by:
  - Profile name (e.g., "phi", "qwen-7b")
  - Full model ID (e.g., "mlx-community/Phi-3.5-mini-instruct-4bit")

Examples:
  penf model serve phi
  penf model serve phi --port 8080
  penf model serve mlx-community/Qwen2.5-7B-Instruct-4bit --port 8081`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelServe(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().IntVar(&modelPort, "port", 8080, "Port to serve on")

	return cmd
}

// newModelStopCommand creates the 'model stop' subcommand.
func newModelStopCommand(deps *ModelCommandDeps) *cobra.Command {
	var stopAll bool

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop model server(s)",
		Long: `Stop running MLX model server(s).

Stops the server running on the specified port, or all servers with --all.

Examples:
  penf model stop --port 8080
  penf model stop --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelStop(cmd.Context(), deps, stopAll)
		},
	}

	cmd.Flags().IntVar(&modelPort, "port", 8080, "Port of server to stop")
	cmd.Flags().BoolVar(&stopAll, "all", false, "Stop all model servers")

	return cmd
}

// newModelSwitchCommand creates the 'model switch' subcommand.
func newModelSwitchCommand(deps *ModelCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "switch <model>",
		Short: "Switch model on a running server",
		Long: `Switch the model running on a server.

Stops the current server and starts a new one with the specified model.
The port defaults to 8080 unless specified.

Examples:
  penf model switch phi
  penf model switch qwen-7b --port 8081`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelSwitch(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().IntVar(&modelPort, "port", 8080, "Port of server to switch")

	return cmd
}

// newModelInfoCommand creates the 'model info' subcommand.
func newModelInfoCommand(deps *ModelCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "info <model>",
		Short: "Show model details",
		Long: `Display detailed information about a model.

Shows the model's size, type, expected latency, memory requirements,
and download status.

Examples:
  penf model info phi
  penf model info mlx-community/Phi-3.5-mini-instruct-4bit`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelInfo(cmd.Context(), deps, args[0])
		},
	}
}

// newModelBenchCommand creates the 'model bench' subcommand.
func newModelBenchCommand(deps *ModelCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Benchmark model server",
		Long: `Run a quick benchmark against a model server.

Sends test requests to measure response latency.

Examples:
  penf model bench
  penf model bench --port 8081`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelBench(cmd.Context(), deps)
		},
	}

	cmd.Flags().IntVar(&modelPort, "port", 8080, "Port of server to benchmark")

	return cmd
}

// newModelDownloadCommand creates the 'model download' subcommand.
func newModelDownloadCommand(deps *ModelCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "download <model>",
		Short: "Download a model from HuggingFace",
		Long: `Download an MLX model from HuggingFace.

Downloads the specified model to the local HuggingFace cache.
The model can be specified by:
  - Short name (e.g., "phi", "qwen-7b", "llama")
  - Full model ID (e.g., "mlx-community/Mistral-7B-Instruct-v0.3-4bit")

After download, the model will appear in 'penf model list' and can be
served with 'penf model serve'.

Note: Downloads use the huggingface-cli tool via the sidecar Python
environment. Large models (7B+) may take several minutes to download.

Examples:
  # Download by short name
  penf model download phi
  penf model download qwen-7b

  # Download by full HuggingFace ID
  penf model download mlx-community/Mistral-7B-Instruct-v0.3-4bit

  # Download a specific MLX model
  penf model download mlx-community/Llama-3.2-1B-Instruct-4bit`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelDownload(cmd.Context(), deps, args[0])
		},
	}
}

// ==================== Registry Commands (Local + Remote) ====================

// newModelRegistryCommand creates the 'model registry' subcommand.
func newModelRegistryCommand(deps *ModelCommandDeps) *cobra.Command {
	var providerFilter string
	var showLocal bool
	var showRemote bool
	var showEnabled bool
	var showDisabled bool

	cmd := &cobra.Command{
		Use:   "registry",
		Short: "List all registered models (local + remote)",
		Long: `Shows all models registered in the AI service registry.

This includes:
  - Local models (Ollama, MLX) auto-discovered from running services
  - Remote models (Gemini, OpenAI) configured in the database

The registry is managed by the AI service and determines which models
are available for different AI tasks (embedding, summarization, etc.).

Use 'penf model add' to register new remote models, and
'penf model enable/disable' to control which models are active.

Examples:
  penf model registry
  penf model registry --provider gemini
  penf model registry --local
  penf model registry --remote
  penf model registry --enabled
  penf model registry -o json`,
		Aliases: []string{"reg"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelRegistry(cmd.Context(), deps, providerFilter, showLocal, showRemote, showEnabled, showDisabled)
		},
	}

	cmd.Flags().StringVar(&providerFilter, "provider", "", "Filter by provider (ollama, gemini, openai, anthropic)")
	cmd.Flags().BoolVar(&showLocal, "local", false, "Show only local models")
	cmd.Flags().BoolVar(&showRemote, "remote", false, "Show only remote models")
	cmd.Flags().BoolVar(&showEnabled, "enabled", false, "Show only enabled models")
	cmd.Flags().BoolVar(&showDisabled, "disabled", false, "Show only disabled models")

	return cmd
}

// newModelAddCommand creates the 'model add' subcommand.
func newModelAddCommand(deps *ModelCommandDeps) *cobra.Command {
	var modelType string
	var capabilities []string
	var endpoint string
	var priority int

	cmd := &cobra.Command{
		Use:   "add <provider> <model-name>",
		Short: "Register a remote model",
		Long: `Register a new remote model in the AI service registry.

Supported providers:
  - gemini    Google's Gemini models (gemini-2.0-flash, gemini-2.5-pro, etc.)
  - openai    OpenAI models (gpt-4o, gpt-4o-mini, text-embedding-3-small, etc.)
  - anthropic Anthropic models (claude-3-5-sonnet, etc.)

The model will be registered and enabled by default. Use --type to specify
whether this is an LLM or embedding model.

Examples:
  # Register a Gemini model
  penf model add gemini gemini-2.0-flash

  # Register an OpenAI embedding model
  penf model add openai text-embedding-3-small --type embedding

  # Register with custom capabilities
  penf model add openai gpt-4o --capabilities chat,summarization,extraction

  # Register with custom priority (higher = preferred)
  penf model add gemini gemini-2.5-pro --priority 10`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelAdd(cmd.Context(), deps, args[0], args[1], modelType, capabilities, endpoint, priority)
		},
	}

	cmd.Flags().StringVar(&modelType, "type", "llm", "Model type: llm, embedding, classifier")
	cmd.Flags().StringSliceVar(&capabilities, "capabilities", nil, "Capabilities (comma-separated): chat, summarization, extraction, classification, embedding")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Custom API endpoint (optional)")
	cmd.Flags().IntVar(&priority, "priority", 0, "Model priority for selection (higher = preferred)")

	return cmd
}

// newModelEnableCommand creates the 'model enable' subcommand.
func newModelEnableCommand(deps *ModelCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <model-id>",
		Short: "Enable a registered model",
		Long: `Enable a model in the AI service registry.

Enabled models are available for routing and will be considered
when selecting models for AI tasks. Use the model ID from
'penf model registry'.

Examples:
  penf model enable abc123
  penf model enable gemini-2.0-flash`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelEnable(cmd.Context(), deps, args[0], true)
		},
	}
}

// newModelDisableCommand creates the 'model disable' subcommand.
func newModelDisableCommand(deps *ModelCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <model-id>",
		Short: "Disable a registered model",
		Long: `Disable a model in the AI service registry.

Disabled models remain in the registry but are not selected for
AI tasks. This is useful for temporarily removing a model from
rotation without deleting its configuration.

Examples:
  penf model disable abc123
  penf model disable gemini-2.0-flash`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelEnable(cmd.Context(), deps, args[0], false)
		},
	}
}

// newModelRulesCommand creates the 'model rules' subcommand.
func newModelRulesCommand(deps *ModelCommandDeps) *cobra.Command {
	var taskTypeFilter string

	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Show model routing rules",
		Long: `Display the AI model routing configuration.

Shows which models are preferred for different task types:
  - embedding:      Vector embedding generation
  - summarization:  Content summarization
  - extraction:     Entity and assertion extraction
  - classification: Content categorization

Each rule specifies preferred models (tried first) and fallback
models (used if preferred models fail).

Examples:
  penf model rules
  penf model rules --task embedding
  penf model rules -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelRules(cmd.Context(), deps, taskTypeFilter)
		},
	}

	cmd.Flags().StringVar(&taskTypeFilter, "task", "", "Filter by task type (embedding, summarization, extraction, classification)")

	return cmd
}

// ==================== Command Execution Functions ====================

// runModelList executes the model list command.
func runModelList(ctx context.Context, deps *ModelCommandDeps, showAll bool) error {
	// Get list of downloaded models from HuggingFace cache.
	downloadedModels := getDownloadedModels(deps)

	// Build catalog with download status.
	catalog := make([]ModelCatalogEntry, len(defaultModelCatalog))
	copy(catalog, defaultModelCatalog)

	for i := range catalog {
		catalog[i].Downloaded = downloadedModels[catalog[i].ID]
	}

	// Filter to downloaded only unless --all.
	if !showAll {
		filtered := make([]ModelCatalogEntry, 0)
		for _, m := range catalog {
			if m.Downloaded {
				filtered = append(filtered, m)
			}
		}
		catalog = filtered
	}

	return outputModelCatalog(deps, catalog, showAll)
}

// runModelStatus executes the model status command.
func runModelStatus(ctx context.Context, deps *ModelCommandDeps) error {
	servers, err := getRunningServers()
	if err != nil {
		return fmt.Errorf("getting running servers: %w", err)
	}

	return outputModelStatus(deps, servers)
}

// runModelServe executes the model serve command.
func runModelServe(ctx context.Context, deps *ModelCommandDeps, model string) error {
	// Resolve model name to full ID.
	modelID := resolveModelID(deps, model)

	// Check if port is already in use.
	servers, _ := getRunningServers()
	for _, s := range servers {
		if s.Port == modelPort {
			return fmt.Errorf("port %d is already in use by model %s (PID %d)", modelPort, s.Model, s.PID)
		}
	}

	fmt.Printf("Starting model server...\n")
	fmt.Printf("  Model: %s\n", modelID)
	fmt.Printf("  Port:  %d\n\n", modelPort)

	// Build the command.
	venvPython := filepath.Join(deps.SidecarDir, ".venv", "bin", "python")
	cmdArgs := []string{
		"-m", "mlx_lm.server",
		"--model", modelID,
		"--port", strconv.Itoa(modelPort),
		"--host", "0.0.0.0",
	}

	// Start the server in the background.
	execCmd := exec.Command(venvPython, cmdArgs...)
	execCmd.Dir = deps.SidecarDir

	// Redirect output to log file.
	logFile := fmt.Sprintf("/tmp/mlx-server-%d.log", modelPort)
	f, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("creating log file: %w", err)
	}
	execCmd.Stdout = f
	execCmd.Stderr = f

	// Start the process.
	if err := execCmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("starting server: %w", err)
	}

	fmt.Printf("Server starting in background (PID: %d)\n", execCmd.Process.Pid)
	fmt.Printf("Log file: %s\n\n", logFile)

	// Wait for server to be ready.
	fmt.Print("Waiting for server to be ready")
	endpoint := fmt.Sprintf("http://localhost:%d/v1/models", modelPort)

	for i := 0; i < 60; i++ {
		resp, err := http.Get(endpoint)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			fmt.Printf("\n\n\033[32mServer is ready!\033[0m\n")
			fmt.Printf("Endpoint: http://localhost:%d/v1/chat/completions\n", modelPort)
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		fmt.Print(".")
		time.Sleep(2 * time.Second)
	}

	fmt.Println()
	return fmt.Errorf("server did not start within 2 minutes - check %s", logFile)
}

// runModelStop executes the model stop command.
func runModelStop(ctx context.Context, deps *ModelCommandDeps, stopAll bool) error {
	servers, err := getRunningServers()
	if err != nil {
		return fmt.Errorf("getting running servers: %w", err)
	}

	if len(servers) == 0 {
		fmt.Println("No model servers are running.")
		return nil
	}

	stopped := 0
	for _, s := range servers {
		if stopAll || s.Port == modelPort {
			process, err := os.FindProcess(s.PID)
			if err != nil {
				fmt.Printf("Warning: could not find process %d: %v\n", s.PID, err)
				continue
			}

			if err := process.Signal(syscall.SIGTERM); err != nil {
				fmt.Printf("Warning: could not stop process %d: %v\n", s.PID, err)
				continue
			}

			fmt.Printf("Stopped server on port %d (PID %d, model: %s)\n", s.Port, s.PID, s.Model)
			stopped++

			if !stopAll {
				break
			}
		}
	}

	if stopped == 0 && !stopAll {
		return fmt.Errorf("no server found on port %d", modelPort)
	}

	return nil
}

// runModelSwitch executes the model switch command.
func runModelSwitch(ctx context.Context, deps *ModelCommandDeps, model string) error {
	// Stop the current server on the port.
	servers, _ := getRunningServers()
	for _, s := range servers {
		if s.Port == modelPort {
			fmt.Printf("Stopping current server (model: %s)...\n", s.Model)
			process, _ := os.FindProcess(s.PID)
			if process != nil {
				_ = process.Signal(syscall.SIGTERM)
				time.Sleep(2 * time.Second)
			}
			break
		}
	}

	// Start the new model.
	return runModelServe(ctx, deps, model)
}

// runModelInfo executes the model info command.
func runModelInfo(ctx context.Context, deps *ModelCommandDeps, model string) error {
	modelID := resolveModelID(deps, model)

	// Find in catalog.
	var entry *ModelCatalogEntry
	for _, m := range defaultModelCatalog {
		if m.ID == modelID || strings.Contains(strings.ToLower(m.ID), strings.ToLower(model)) {
			entryCopy := m
			entry = &entryCopy
			break
		}
	}

	if entry == nil {
		// Create a basic entry for unknown models.
		entry = &ModelCatalogEntry{
			ID:   modelID,
			Name: modelID,
			Type: "mlx",
		}
	}

	// Check if downloaded.
	downloadedModels := getDownloadedModels(deps)
	entry.Downloaded = downloadedModels[entry.ID]

	return outputModelInfo(deps, entry)
}

// runModelBench executes the model bench command.
func runModelBench(ctx context.Context, deps *ModelCommandDeps) error {
	endpoint := fmt.Sprintf("http://localhost:%d/v1/chat/completions", modelPort)

	// First check if server is running.
	modelsEndpoint := fmt.Sprintf("http://localhost:%d/v1/models", modelPort)
	resp, err := http.Get(modelsEndpoint)
	if err != nil {
		return fmt.Errorf("no server running on port %d", modelPort)
	}
	resp.Body.Close()

	fmt.Printf("Benchmarking model server on port %d...\n\n", modelPort)

	// Run benchmark tests.
	tests := []struct {
		name      string
		prompt    string
		maxTokens int
	}{
		{"Simple", "Reply with just: hello", 10},
		{"Medium", "What is 2+2? Answer briefly.", 50},
		{"Complex", "Explain in one sentence why the sky is blue.", 100},
	}

	for _, test := range tests {
		start := time.Now()

		reqBody := fmt.Sprintf(`{"messages":[{"role":"user","content":"%s"}],"max_tokens":%d}`,
			test.prompt, test.maxTokens)

		req, _ := http.NewRequest("POST", endpoint, strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(req)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("  %s: \033[31mFAILED\033[0m (%v)\n", test.name, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != 200 {
			fmt.Printf("  %s: \033[31mFAILED\033[0m (status %d)\n", test.name, resp.StatusCode)
			continue
		}

		color := "\033[32m" // Green
		if elapsed > 5*time.Second {
			color = "\033[33m" // Yellow
		}
		if elapsed > 15*time.Second {
			color = "\033[31m" // Red
		}

		fmt.Printf("  %s: %s%.2fs\033[0m\n", test.name, color, elapsed.Seconds())
	}

	return nil
}

// runModelDownload executes the model download command.
func runModelDownload(ctx context.Context, deps *ModelCommandDeps, model string) error {
	// Resolve model name to full ID.
	modelID := resolveModelID(deps, model)

	// Check if already downloaded.
	downloadedModels := getDownloadedModels(deps)
	if downloadedModels[modelID] {
		fmt.Printf("Model '%s' is already downloaded.\n", modelID)
		fmt.Println("\nUse 'penf model serve' to start a server with this model.")
		return nil
	}

	// Find model info in catalog for display.
	var modelInfo *ModelCatalogEntry
	for _, m := range defaultModelCatalog {
		if m.ID == modelID {
			entryCopy := m
			modelInfo = &entryCopy
			break
		}
	}

	fmt.Println("Downloading Model")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  \033[1mModel ID:\033[0m %s\n", modelID)
	if modelInfo != nil {
		fmt.Printf("  \033[1mName:\033[0m     %s\n", modelInfo.Name)
		fmt.Printf("  \033[1mSize:\033[0m     %s\n", modelInfo.Size)
	} else {
		fmt.Printf("  \033[33mNote: Model not in catalog - downloading from HuggingFace\033[0m\n")
	}
	fmt.Println()

	// Use huggingface-cli from the sidecar venv to download.
	venvBin := filepath.Join(deps.SidecarDir, ".venv", "bin")
	hfCLI := filepath.Join(venvBin, "huggingface-cli")

	// Check if huggingface-cli exists.
	if _, err := os.Stat(hfCLI); os.IsNotExist(err) {
		return fmt.Errorf("huggingface-cli not found at %s\nEnsure the sidecar environment is set up", hfCLI)
	}

	fmt.Println("Starting download... (this may take several minutes for large models)")
	fmt.Println()

	// Build the download command.
	// huggingface-cli download <model-id>
	execCmd := exec.CommandContext(ctx, hfCLI, "download", modelID)
	execCmd.Dir = deps.SidecarDir

	// Stream output to show progress.
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	// Run the download.
	if err := execCmd.Run(); err != nil {
		// Check for common error conditions.
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("download cancelled")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Check for network errors, disk space, etc.
			return fmt.Errorf("download failed (exit code %d): check network connection and disk space", exitErr.ExitCode())
		}
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Println()
	fmt.Printf("\033[32mDownload complete!\033[0m\n")
	fmt.Println()

	// Verify the model was downloaded.
	downloadedModels = getDownloadedModels(deps)
	if !downloadedModels[modelID] {
		fmt.Printf("\033[33mWarning: Model may not be MLX-compatible or download incomplete.\033[0m\n")
		fmt.Println("Check 'penf model list --all' to verify.")
		return nil
	}

	fmt.Println("Model is ready to use:")
	fmt.Printf("  penf model serve %s\n", model)

	return nil
}

// ==================== Registry Command Execution Functions ====================

// runModelRegistry executes the model registry command.
func runModelRegistry(ctx context.Context, deps *ModelCommandDeps, providerFilter string, showLocal, showRemote, showEnabled, showDisabled bool) error {
	cfg, err := loadModelConfig(deps)
	if err != nil {
		return err
	}

	conn, err := connectModelToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := aiv1.NewAICoordinatorServiceClient(conn)

	// Build request with filters.
	req := &aiv1.ListModelsRequest{}

	if providerFilter != "" {
		req.Provider = &providerFilter
	}

	if showLocal && !showRemote {
		isLocal := true
		req.IsLocal = &isLocal
	} else if showRemote && !showLocal {
		isLocal := false
		req.IsLocal = &isLocal
	}

	if showEnabled && !showDisabled {
		isEnabled := true
		req.IsEnabled = &isEnabled
	} else if showDisabled && !showEnabled {
		isEnabled := false
		req.IsEnabled = &isEnabled
	}

	resp, err := client.ListModels(ctx, req)
	if err != nil {
		return fmt.Errorf("listing models from registry: %w\n\nEnsure the Gateway service is running at %s", err, cfg.ServerAddress)
	}

	return outputRegistryModels(deps, resp.Models, resp.TotalCount)
}

// runModelAdd executes the model add command.
func runModelAdd(ctx context.Context, deps *ModelCommandDeps, provider, modelName, modelType string, capabilities []string, endpoint string, priority int) error {
	cfg, err := loadModelConfig(deps)
	if err != nil {
		return err
	}

	conn, err := connectModelToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := aiv1.NewAICoordinatorServiceClient(conn)

	// Map model type string to proto enum.
	var protoType aiv1.ModelType
	switch strings.ToLower(modelType) {
	case "embedding":
		protoType = aiv1.ModelType_MODEL_TYPE_EMBEDDING
	case "classifier", "classification":
		protoType = aiv1.ModelType_MODEL_TYPE_CLASSIFIER
	case "ner":
		protoType = aiv1.ModelType_MODEL_TYPE_NER
	default:
		protoType = aiv1.ModelType_MODEL_TYPE_LLM
	}

	// Set default capabilities based on model type if not provided.
	if len(capabilities) == 0 {
		switch protoType {
		case aiv1.ModelType_MODEL_TYPE_EMBEDDING:
			capabilities = []string{"embedding"}
		case aiv1.ModelType_MODEL_TYPE_LLM:
			capabilities = []string{"chat", "summarization", "extraction"}
		case aiv1.ModelType_MODEL_TYPE_CLASSIFIER:
			capabilities = []string{"classification"}
		}
	}

	// Generate a display name from provider and model name.
	displayName := fmt.Sprintf("%s/%s", provider, modelName)

	req := &aiv1.RegisterModelRequest{
		Name:         displayName,
		Provider:     provider,
		ModelName:    modelName,
		Type:         protoType,
		Capabilities: capabilities,
		IsLocal:      false, // Remote models
		IsEnabled:    true,  // Enabled by default
		Priority:     int32(priority),
	}

	if endpoint != "" {
		req.Endpoint = &endpoint
	}

	resp, err := client.RegisterModel(ctx, req)
	if err != nil {
		return fmt.Errorf("registering model: %w", err)
	}

	fmt.Printf("\033[32mRegistered model:\033[0m %s\n", resp.Model.Name)
	fmt.Printf("  ID:           %s\n", resp.Model.Id)
	fmt.Printf("  Provider:     %s\n", resp.Model.Provider)
	fmt.Printf("  Type:         %s\n", modelTypeToString(resp.Model.Type))
	fmt.Printf("  Capabilities: %s\n", strings.Join(resp.Model.Capabilities, ", "))
	fmt.Printf("  Enabled:      %v\n", resp.Model.IsEnabled)

	return nil
}

// runModelEnable executes the model enable/disable command.
func runModelEnable(ctx context.Context, deps *ModelCommandDeps, modelID string, enable bool) error {
	cfg, err := loadModelConfig(deps)
	if err != nil {
		return err
	}

	conn, err := connectModelToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := aiv1.NewAICoordinatorServiceClient(conn)

	req := &aiv1.UpdateModelRequest{
		ModelId:   modelID,
		IsEnabled: &enable,
	}

	resp, err := client.UpdateModel(ctx, req)
	if err != nil {
		return fmt.Errorf("updating model: %w", err)
	}

	action := "Enabled"
	if !enable {
		action = "Disabled"
	}

	fmt.Printf("\033[32m%s model:\033[0m %s\n", action, resp.Model.Name)
	fmt.Printf("  ID: %s\n", resp.Model.Id)

	return nil
}

// runModelRules executes the model rules command.
func runModelRules(ctx context.Context, deps *ModelCommandDeps, taskTypeFilter string) error {
	cfg, err := loadModelConfig(deps)
	if err != nil {
		return err
	}

	conn, err := connectModelToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := aiv1.NewAICoordinatorServiceClient(conn)

	req := &aiv1.GetRoutingRulesRequest{}
	if taskTypeFilter != "" {
		req.TaskType = &taskTypeFilter
	}

	resp, err := client.GetRoutingRules(ctx, req)
	if err != nil {
		return fmt.Errorf("getting routing rules: %w\n\nEnsure the Gateway service is running at %s", err, cfg.ServerAddress)
	}

	return outputRoutingRules(deps, resp.Rules)
}

// ==================== Helper Functions ====================

// loadModelConfig loads the CLI configuration.
func loadModelConfig(deps *ModelCommandDeps) (*config.CLIConfig, error) {
	if deps.Config != nil {
		return deps.Config, nil
	}

	if deps.LoadConfig == nil {
		deps.LoadConfig = config.LoadConfig
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg
	return cfg, nil
}

// connectModelToGateway creates a gRPC connection to the gateway service.
func connectModelToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if cfg.TLS.Enabled {
		tlsConfig, err := client.LoadClientTLSConfig(&cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("loading TLS config: %w", err)
		}
		if tlsConfig != nil {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, cfg.ServerAddress, opts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to gateway at %s: %w", cfg.ServerAddress, err)
	}

	return conn, nil
}

// modelTypeToString converts a proto ModelType to a readable string.
func modelTypeToString(t aiv1.ModelType) string {
	switch t {
	case aiv1.ModelType_MODEL_TYPE_EMBEDDING:
		return "embedding"
	case aiv1.ModelType_MODEL_TYPE_LLM:
		return "llm"
	case aiv1.ModelType_MODEL_TYPE_CLASSIFIER:
		return "classifier"
	case aiv1.ModelType_MODEL_TYPE_NER:
		return "ner"
	default:
		return "unknown"
	}
}

// modelStatusToString converts a proto ModelStatus to a readable string.
func modelStatusToString(s aiv1.ModelStatus) string {
	switch s {
	case aiv1.ModelStatus_MODEL_STATUS_READY:
		return "ready"
	case aiv1.ModelStatus_MODEL_STATUS_LOADING:
		return "loading"
	case aiv1.ModelStatus_MODEL_STATUS_ERROR:
		return "error"
	case aiv1.ModelStatus_MODEL_STATUS_UNLOADED:
		return "unloaded"
	case aiv1.ModelStatus_MODEL_STATUS_UPDATING:
		return "updating"
	default:
		return "unknown"
	}
}

// optimizationModeToString converts a proto OptimizationMode to a readable string.
func optimizationModeToString(m aiv1.OptimizationMode) string {
	switch m {
	case aiv1.OptimizationMode_OPTIMIZATION_MODE_LATENCY:
		return "latency"
	case aiv1.OptimizationMode_OPTIMIZATION_MODE_QUALITY:
		return "quality"
	case aiv1.OptimizationMode_OPTIMIZATION_MODE_COST:
		return "cost"
	case aiv1.OptimizationMode_OPTIMIZATION_MODE_BALANCED:
		return "balanced"
	default:
		return "default"
	}
}

// getDownloadedModels returns a map of model IDs that are downloaded.
func getDownloadedModels(deps *ModelCommandDeps) map[string]bool {
	downloaded := make(map[string]bool)

	// Check HuggingFace cache directory.
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".cache", "huggingface", "hub")

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return downloaded
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "models--") {
			// Convert directory name to model ID.
			// e.g., "models--mlx-community--Phi-3.5-mini-instruct-4bit"
			// -> "mlx-community/Phi-3.5-mini-instruct-4bit"
			modelID := strings.TrimPrefix(name, "models--")
			modelID = strings.Replace(modelID, "--", "/", 1)
			downloaded[modelID] = true
		}
	}

	return downloaded
}

// getRunningServers returns a list of running MLX model servers.
func getRunningServers() ([]ModelServerStatus, error) {
	var servers []ModelServerStatus

	// Use ps to find mlx_lm.server processes.
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "mlx_lm.server") || strings.Contains(line, "grep") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		pid, _ := strconv.Atoi(fields[1])
		cpuPct, _ := strconv.ParseFloat(fields[2], 64)
		memPct, _ := strconv.ParseFloat(fields[3], 64)

		// Extract model and port from command line.
		model := ""
		port := 8080

		for i, f := range fields {
			if f == "--model" && i+1 < len(fields) {
				model = fields[i+1]
			}
			if f == "--port" && i+1 < len(fields) {
				port, _ = strconv.Atoi(fields[i+1])
			}
		}

		// Check health.
		healthy := false
		endpoint := fmt.Sprintf("http://localhost:%d/v1/models", port)
		resp, err := http.Get(endpoint)
		if err == nil && resp.StatusCode == 200 {
			healthy = true
			resp.Body.Close()
		}

		servers = append(servers, ModelServerStatus{
			PID:     pid,
			Model:   model,
			Port:    port,
			CPUPct:  cpuPct,
			MemPct:  memPct,
			Healthy: healthy,
		})
	}

	return servers, nil
}

// resolveModelID resolves a short model name to a full model ID.
func resolveModelID(deps *ModelCommandDeps, model string) string {
	// If it looks like a full model ID, return as-is.
	if strings.Contains(model, "/") {
		return model
	}

	// Map short names to full IDs.
	shortNames := map[string]string{
		"phi":      "mlx-community/Phi-3.5-mini-instruct-4bit",
		"phi-3.5":  "mlx-community/Phi-3.5-mini-instruct-4bit",
		"qwen-7b": "mlx-community/Qwen2.5-7B-Instruct-4bit",
		"qwen":    "mlx-community/Qwen2.5-7B-Instruct-4bit",
		"llama-3b": "mlx-community/Llama-3.2-3B-Instruct-4bit",
		"llama":    "mlx-community/Llama-3.2-3B-Instruct-4bit",
		"gemma":    "mlx-community/gemma-2-9b-it-4bit",
		"gemma-9b": "mlx-community/gemma-2-9b-it-4bit",
	}

	if fullID, ok := shortNames[strings.ToLower(model)]; ok {
		return fullID
	}

	// Search catalog for partial match.
	modelLower := strings.ToLower(model)
	for _, entry := range defaultModelCatalog {
		if strings.Contains(strings.ToLower(entry.ID), modelLower) {
			return entry.ID
		}
	}

	// Return as-is if not found.
	return model
}

// ==================== Output Functions ====================

// getModelOutputFormat determines the output format from flags and config.
func getModelOutputFormat(deps *ModelCommandDeps) config.OutputFormat {
	if modelOutput != "" {
		return config.OutputFormat(modelOutput)
	}
	if deps.Config != nil {
		return deps.Config.OutputFormat
	}
	return config.OutputFormatText
}

// outputModelCatalog outputs the model catalog.
func outputModelCatalog(deps *ModelCommandDeps, catalog []ModelCatalogEntry, showAll bool) error {
	format := getModelOutputFormat(deps)

	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(catalog)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(catalog)
	default:
		return outputModelCatalogText(catalog, showAll)
	}
}

// outputModelCatalogText outputs the catalog in human-readable format.
func outputModelCatalogText(catalog []ModelCatalogEntry, showAll bool) error {
	if len(catalog) == 0 {
		if showAll {
			fmt.Println("No models in catalog.")
		} else {
			fmt.Println("No downloaded models found.")
			fmt.Println("\nUse 'penf model list --all' to see available models.")
		}
		return nil
	}

	title := "Downloaded Models"
	if showAll {
		title = "Available Models"
	}
	fmt.Printf("%s (%d)\n", title, len(catalog))
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("  %-42s %-12s %-8s %s\n", "MODEL", "SIZE", "LATENCY", "STATUS")
	fmt.Printf("  %-42s %-12s %-8s %s\n", "-----", "----", "-------", "------")

	for _, m := range catalog {
		status := "\033[32mdownloaded\033[0m"
		if !m.Downloaded {
			status = "\033[90mavailable\033[0m"
		}

		name := m.Name
		if len(name) > 40 {
			name = name[:37] + "..."
		}

		fmt.Printf("  %-42s %-12s %-8s %s\n",
			name,
			m.Size,
			m.ExpectedLatency,
			status)
	}

	fmt.Println()
	return nil
}

// outputModelStatus outputs running server status.
func outputModelStatus(deps *ModelCommandDeps, servers []ModelServerStatus) error {
	format := getModelOutputFormat(deps)

	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(servers)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(servers)
	default:
		return outputModelStatusText(servers)
	}
}

// outputModelStatusText outputs server status in human-readable format.
func outputModelStatusText(servers []ModelServerStatus) error {
	if len(servers) == 0 {
		fmt.Println("No model servers are running.")
		fmt.Println("\nUse 'penf model serve <model>' to start a server.")
		return nil
	}

	fmt.Printf("Running Model Servers (%d)\n", len(servers))
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("  %-6s %-40s %-6s %-6s %-6s %s\n", "PID", "MODEL", "PORT", "CPU", "MEM", "HEALTH")
	fmt.Printf("  %-6s %-40s %-6s %-6s %-6s %s\n", "---", "-----", "----", "---", "---", "------")

	for _, s := range servers {
		healthColor := "\033[32m"
		healthStr := "healthy"
		if !s.Healthy {
			healthColor = "\033[31m"
			healthStr = "unhealthy"
		}

		modelName := s.Model
		if len(modelName) > 38 {
			modelName = modelName[:35] + "..."
		}

		fmt.Printf("  %-6d %-40s %-6d %-6.1f %-6.1f %s%s\033[0m\n",
			s.PID,
			modelName,
			s.Port,
			s.CPUPct,
			s.MemPct,
			healthColor,
			healthStr)
	}

	fmt.Println()
	return nil
}

// outputModelInfo outputs model information.
func outputModelInfo(deps *ModelCommandDeps, entry *ModelCatalogEntry) error {
	format := getModelOutputFormat(deps)

	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entry)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(entry)
	default:
		return outputModelInfoText(entry)
	}
}

// outputModelInfoText outputs model info in human-readable format.
func outputModelInfoText(entry *ModelCatalogEntry) error {
	fmt.Println("Model Information")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("  \033[1mID:\033[0m              %s\n", entry.ID)
	fmt.Printf("  \033[1mName:\033[0m            %s\n", entry.Name)
	fmt.Printf("  \033[1mType:\033[0m            %s\n", entry.Type)

	if entry.Size != "" {
		fmt.Printf("  \033[1mSize:\033[0m            %s\n", entry.Size)
	}
	if entry.ExpectedLatency != "" {
		fmt.Printf("  \033[1mLatency:\033[0m         %s\n", entry.ExpectedLatency)
	}
	if entry.MemoryRequired != "" {
		fmt.Printf("  \033[1mMemory Required:\033[0m %s\n", entry.MemoryRequired)
	}

	status := "\033[32mdownloaded\033[0m"
	if !entry.Downloaded {
		status = "\033[33mnot downloaded\033[0m"
	}
	fmt.Printf("  \033[1mStatus:\033[0m          %s\n", status)

	fmt.Println()
	return nil
}

// ==================== Registry Output Functions ====================

// RegistryModelEntry represents a model for JSON/YAML output.
type RegistryModelEntry struct {
	ID           string   `json:"id" yaml:"id"`
	Name         string   `json:"name" yaml:"name"`
	Provider     string   `json:"provider" yaml:"provider"`
	ModelName    string   `json:"model_name" yaml:"model_name"`
	Type         string   `json:"type" yaml:"type"`
	Status       string   `json:"status" yaml:"status"`
	Capabilities []string `json:"capabilities" yaml:"capabilities"`
	IsLocal      bool     `json:"is_local" yaml:"is_local"`
	IsEnabled    bool     `json:"is_enabled" yaml:"is_enabled"`
	Priority     int32    `json:"priority" yaml:"priority"`
}

// RoutingRuleEntry represents a routing rule for JSON/YAML output.
type RoutingRuleEntry struct {
	ID               string   `json:"id" yaml:"id"`
	Name             string   `json:"name" yaml:"name"`
	TaskType         string   `json:"task_type" yaml:"task_type"`
	PreferredModels  []string `json:"preferred_models" yaml:"preferred_models"`
	FallbackModels   []string `json:"fallback_models" yaml:"fallback_models"`
	OptimizationMode string   `json:"optimization_mode" yaml:"optimization_mode"`
	IsEnabled        bool     `json:"is_enabled" yaml:"is_enabled"`
	Description      string   `json:"description,omitempty" yaml:"description,omitempty"`
}

// outputRegistryModels outputs the registry models.
func outputRegistryModels(deps *ModelCommandDeps, models []*aiv1.ModelInfo, totalCount int32) error {
	format := getModelOutputFormat(deps)

	// Convert to output format.
	entries := make([]RegistryModelEntry, len(models))
	for i, m := range models {
		entries[i] = RegistryModelEntry{
			ID:           m.Id,
			Name:         m.Name,
			Provider:     m.Provider,
			ModelName:    m.ModelName,
			Type:         modelTypeToString(m.Type),
			Status:       modelStatusToString(m.Status),
			Capabilities: m.Capabilities,
			IsLocal:      m.IsLocal,
			IsEnabled:    m.IsEnabled,
			Priority:     m.Priority,
		}
	}

	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(entries)
	default:
		return outputRegistryModelsText(entries, totalCount)
	}
}

// outputRegistryModelsText outputs registry models in human-readable format.
func outputRegistryModelsText(models []RegistryModelEntry, totalCount int32) error {
	if len(models) == 0 {
		fmt.Println("No models registered in the AI service.")
		fmt.Println("\nUse 'penf model add <provider> <model-name>' to register a model.")
		return nil
	}

	fmt.Printf("Registered Models (%d)\n", totalCount)
	fmt.Println(strings.Repeat("=", 100))
	fmt.Printf("  %-12s %-32s %-10s %-10s %-8s %s\n", "ID", "NAME", "PROVIDER", "TYPE", "STATUS", "ENABLED")
	fmt.Printf("  %-12s %-32s %-10s %-10s %-8s %s\n", "--", "----", "--------", "----", "------", "-------")

	for _, m := range models {
		// Truncate ID for display.
		id := m.ID
		if len(id) > 10 {
			id = id[:10] + ".."
		}

		// Truncate name for display.
		name := m.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}

		// Format enabled status with color.
		enabledStr := "\033[32myes\033[0m"
		if !m.IsEnabled {
			enabledStr = "\033[31mno\033[0m"
		}

		// Format status with color.
		statusColor := ""
		switch m.Status {
		case "ready":
			statusColor = "\033[32m"
		case "loading", "updating":
			statusColor = "\033[33m"
		case "error", "unloaded":
			statusColor = "\033[31m"
		default:
			statusColor = "\033[90m"
		}

		// Format type indicator.
		typeStr := m.Type
		if m.IsLocal {
			typeStr += " (L)"
		}

		fmt.Printf("  %-12s %-32s %-10s %-10s %s%-8s\033[0m %s\n",
			id,
			name,
			m.Provider,
			typeStr,
			statusColor,
			m.Status,
			enabledStr)
	}

	fmt.Println()
	fmt.Println("(L) = Local model")
	fmt.Println()
	return nil
}

// outputRoutingRules outputs the routing rules.
func outputRoutingRules(deps *ModelCommandDeps, rules []*aiv1.RoutingRule) error {
	format := getModelOutputFormat(deps)

	// Convert to output format.
	entries := make([]RoutingRuleEntry, len(rules))
	for i, r := range rules {
		desc := ""
		if r.Description != nil {
			desc = *r.Description
		}
		entries[i] = RoutingRuleEntry{
			ID:               r.Id,
			Name:             r.Name,
			TaskType:         r.TaskType,
			PreferredModels:  r.PreferredModelIds,
			FallbackModels:   r.FallbackModelIds,
			OptimizationMode: optimizationModeToString(r.OptimizationMode),
			IsEnabled:        r.IsEnabled,
			Description:      desc,
		}
	}

	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(entries)
	default:
		return outputRoutingRulesText(entries)
	}
}

// outputRoutingRulesText outputs routing rules in human-readable format.
func outputRoutingRulesText(rules []RoutingRuleEntry) error {
	if len(rules) == 0 {
		fmt.Println("No routing rules configured.")
		fmt.Println("\nRouting rules are configured in the AI service to determine")
		fmt.Println("which models are used for different task types.")
		return nil
	}

	fmt.Printf("Routing Rules (%d)\n", len(rules))
	fmt.Println(strings.Repeat("=", 80))

	for _, r := range rules {
		enabledStr := "\033[32menabled\033[0m"
		if !r.IsEnabled {
			enabledStr = "\033[31mdisabled\033[0m"
		}

		fmt.Printf("\n  \033[1m%s\033[0m (%s)\n", r.Name, enabledStr)
		fmt.Printf("  Task Type: %s\n", r.TaskType)
		fmt.Printf("  Optimization: %s\n", r.OptimizationMode)

		if len(r.PreferredModels) > 0 {
			fmt.Printf("  Preferred Models:\n")
			for i, m := range r.PreferredModels {
				fmt.Printf("    %d. %s\n", i+1, m)
			}
		}

		if len(r.FallbackModels) > 0 {
			fmt.Printf("  Fallback Models:\n")
			for i, m := range r.FallbackModels {
				fmt.Printf("    %d. %s\n", i+1, m)
			}
		}

		if r.Description != "" {
			fmt.Printf("  Description: %s\n", r.Description)
		}
	}

	fmt.Println()
	return nil
}
