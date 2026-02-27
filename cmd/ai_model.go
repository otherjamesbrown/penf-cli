// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	aiv1 "github.com/otherjamesbrown/penf-cli/api/proto/ai/v1"
	"github.com/otherjamesbrown/penf-cli/client"
)

// NewAIModelCommand creates the 'ai model' subcommand tree for managing
// pipeline stage→model configuration.
func NewAIModelCommand(deps *AICommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Manage pipeline stage model configuration",
		Long: `View and manage which AI models are used for each pipeline stage.

The pipeline has several stages (triage, extract_semantic, extract_ner,
extract_assertions, deep_analyze, embedding) each configured to use a
specific model. Configuration can come from DB overrides, environment
variables, or system defaults.

Commands:
  list       Show current stage→model mapping
  set        Set model for a stage or default
  reset      Remove DB override, revert to env/default
  available  List models available for assignment
  test       Run a quick inference test for a stage

Examples:
  penf ai model list
  penf ai model set triage llama3.2
  penf ai model set default qwen3:8b
  penf ai model available
  penf ai model test triage
  penf ai model reset triage`,
	}

	cmd.AddCommand(newAIModelListCommand(deps))
	cmd.AddCommand(newAIModelSetCommand(deps))
	cmd.AddCommand(newAIModelResetCommand(deps))
	cmd.AddCommand(newAIModelAvailableCommand(deps))
	cmd.AddCommand(newAIModelTestCommand(deps))

	return cmd
}

// newAIModelListCommand creates the 'ai model list' subcommand.
func newAIModelListCommand(deps *AICommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show current stage→model mapping",
		Long: `Display the model configured for each pipeline stage.

Shows the model name, config source (db override, env var, or default),
and backend provider for each stage.

Examples:
  penf ai model list
  penf ai model list -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAIModelList(cmd.Context(), deps)
		},
	}
}

// newAIModelSetCommand creates the 'ai model set' subcommand.
func newAIModelSetCommand(deps *AICommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "set <stage|default|default-embedding> <model>",
		Short: "Set model for a stage or default",
		Long: `Set the model for a specific pipeline stage or the global default.

Creates a DB override that takes precedence over environment variables.
Changes take effect on the next pipeline run without service restart.

Valid stages: triage, extract_semantic, extract_ner, extract_assertions,
              deep_analyze, embedding
Special keys: default (global LLM default), default-embedding

Examples:
  penf ai model set triage llama3.2
  penf ai model set deep_analyze gemini-2.5-pro
  penf ai model set default qwen3:8b
  penf ai model set default-embedding mxbai-embed-large`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAIModelSet(cmd.Context(), deps, args[0], args[1])
		},
	}
}

// newAIModelResetCommand creates the 'ai model reset' subcommand.
func newAIModelResetCommand(deps *AICommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "reset <stage|default|default-embedding>",
		Short: "Remove DB override, revert to env/default",
		Long: `Remove the database override for a pipeline stage, reverting to the
environment variable or system default.

Examples:
  penf ai model reset triage
  penf ai model reset default`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAIModelReset(cmd.Context(), deps, args[0])
		},
	}
}

// newAIModelAvailableCommand creates the 'ai model available' subcommand.
func newAIModelAvailableCommand(deps *AICommandDeps) *cobra.Command {
	var backendFilter string

	cmd := &cobra.Command{
		Use:   "available",
		Short: "List models available for assignment",
		Long: `List models installed on Ollama plus known remote models (Gemini, etc.).

Shows which models are currently in use by pipeline stages.

Examples:
  penf ai model available
  penf ai model available --backend ollama
  penf ai model available -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAIModelAvailable(cmd.Context(), deps, backendFilter)
		},
	}

	cmd.Flags().StringVar(&backendFilter, "backend", "", "Filter by backend: ollama, gemini, openai, anthropic")

	return cmd
}

// newAIModelTestCommand creates the 'ai model test' subcommand.
func newAIModelTestCommand(deps *AICommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "test <stage>",
		Short: "Run a quick inference test for a stage",
		Long: `Run a quick inference test using the model configured for a pipeline stage.

Sends a small test prompt and reports latency, status, and a preview of the output.

Examples:
  penf ai model test triage
  penf ai model test deep_analyze`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAIModelTest(cmd.Context(), deps, args[0])
		},
	}
}

// ==================== Command Execution Functions ====================

func runAIModelList(ctx context.Context, deps *AICommandDeps) error {
	aiClient, err := connectAIModel(deps)
	if err != nil {
		return err
	}
	defer aiClient.Close()

	resp, err := aiClient.GetStageConfig(ctx, "", "")
	if err != nil {
		return fmt.Errorf("getting stage config: %w", err)
	}

	switch aiOutput {
	case "json":
		return outputJSONIndent(resp)
	case "yaml":
		return outputYAMLDoc(resp)
	default:
		return outputAIModelListText(resp)
	}
}

func runAIModelSet(ctx context.Context, deps *AICommandDeps, key, model string) error {
	aiClient, err := connectAIModel(deps)
	if err != nil {
		return err
	}
	defer aiClient.Close()

	resp, err := aiClient.SetStageConfig(ctx, "", key, model)
	if err != nil {
		return fmt.Errorf("setting stage config: %w", err)
	}

	switch aiOutput {
	case "json":
		return outputJSONIndent(resp)
	default:
		cfg := resp.GetConfig()
		prev := resp.GetPreviousModel()
		fmt.Printf("\033[32mUpdated %s:\033[0m %s → %s\n", cfg.GetStage(), prev, cfg.GetModel())
		fmt.Printf("  Source:  %s\n", cfg.GetSource())
		fmt.Printf("  Backend: %s\n", cfg.GetBackend())
		return nil
	}
}

func runAIModelReset(ctx context.Context, deps *AICommandDeps, key string) error {
	aiClient, err := connectAIModel(deps)
	if err != nil {
		return err
	}
	defer aiClient.Close()

	resp, err := aiClient.ResetStageConfig(ctx, "", key)
	if err != nil {
		return fmt.Errorf("resetting stage config: %w", err)
	}

	switch aiOutput {
	case "json":
		return outputJSONIndent(resp)
	default:
		cfg := resp.GetConfig()
		fmt.Printf("\033[32mReset %s:\033[0m now using %s (%s)\n", key, cfg.GetModel(), cfg.GetSource())
		return nil
	}
}

func runAIModelAvailable(ctx context.Context, deps *AICommandDeps, backend string) error {
	aiClient, err := connectAIModel(deps)
	if err != nil {
		return err
	}
	defer aiClient.Close()

	resp, err := aiClient.ListAvailableModels(ctx, "", backend)
	if err != nil {
		return fmt.Errorf("listing available models: %w", err)
	}

	switch aiOutput {
	case "json":
		return outputJSONIndent(resp)
	case "yaml":
		return outputYAMLDoc(resp)
	default:
		return outputAIModelAvailableText(resp)
	}
}

func runAIModelTest(ctx context.Context, deps *AICommandDeps, stage string) error {
	aiClient, err := connectAIModel(deps)
	if err != nil {
		return err
	}
	defer aiClient.Close()

	fmt.Printf("Testing %s stage...\n", stage)

	resp, err := aiClient.TestStage(ctx, "", stage)
	if err != nil {
		return fmt.Errorf("testing stage: %w", err)
	}

	switch aiOutput {
	case "json":
		return outputJSONIndent(resp)
	default:
		return outputAIModelTestText(resp)
	}
}

// ==================== Output Functions ====================

func outputAIModelListText(resp *aiv1.GetStageConfigResponse) error {
	// Stage table.
	fmt.Printf("%-22s %-28s %-10s %s\n", "STAGE", "MODEL", "SOURCE", "BACKEND")
	fmt.Println(strings.Repeat("─", 80))

	for _, s := range resp.GetStages() {
		source := formatSource(s.GetSource(), s.GetEnvVar())
		fmt.Printf("%-22s %-28s %-10s %s\n", s.GetStage(), s.GetModel(), source, s.GetBackend())
	}

	// Defaults.
	fmt.Println()
	fmt.Println("Defaults:")
	if dl := resp.GetDefaultLlm(); dl != nil && dl.GetModel() != "" {
		source := formatSource(dl.GetSource(), dl.GetEnvVar())
		fmt.Printf("  LLM:       %-28s %s\n", dl.GetModel(), source)
	}
	if de := resp.GetDefaultEmbedding(); de != nil && de.GetModel() != "" {
		source := formatSource(de.GetSource(), de.GetEnvVar())
		fmt.Printf("  Embedding: %-28s %s\n", de.GetModel(), source)
	}

	return nil
}

func outputAIModelAvailableText(resp *aiv1.ListAvailableModelsResponse) error {
	fmt.Printf("%-35s %-12s %-10s %s\n", "MODEL", "BACKEND", "SIZE", "USED BY")
	fmt.Println(strings.Repeat("─", 80))

	for _, m := range resp.GetModels() {
		size := m.GetSize()
		if size == "" {
			size = "-"
		}
		usedBy := "-"
		if m.GetInUse() && len(m.GetUsedByStages()) > 0 {
			usedBy = strings.Join(m.GetUsedByStages(), ", ")
		}
		fmt.Printf("%-35s %-12s %-10s %s\n", m.GetName(), m.GetBackend(), size, usedBy)
	}

	return nil
}

func outputAIModelTestText(resp *aiv1.TestStageResponse) error {
	if resp.GetSuccess() {
		fmt.Printf("  \033[32mStatus:  OK\033[0m\n")
	} else {
		fmt.Printf("  \033[31mStatus:  FAILED\033[0m\n")
	}
	fmt.Printf("  Model:   %s\n", resp.GetModel())
	fmt.Printf("  Backend: %s\n", resp.GetBackend())
	fmt.Printf("  Latency: %.1fms\n", resp.GetLatencyMs())

	if resp.GetError() != "" {
		fmt.Printf("  Error:   %s\n", resp.GetError())
	}
	if resp.GetOutputPreview() != "" {
		preview := resp.GetOutputPreview()
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		fmt.Printf("  Output:  %s\n", preview)
	}

	return nil
}

// ==================== Helper Functions ====================

// connectAIModel creates an AI client connected to the gateway.
func connectAIModel(deps *AICommandDeps) (*client.AIClient, error) {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("loading configuration: %w", err)
	}

	opts := client.DefaultOptions()
	opts.ConnectTimeout = cfg.Timeout
	opts.Insecure = cfg.Insecure

	aiClient := client.NewAIClient(cfg.ServerAddress, opts)

	if err := aiClient.Connect(context.Background()); err != nil {
		return nil, fmt.Errorf("connecting to gateway at %s: %w\n\nEnsure the Gateway service is running.", cfg.ServerAddress, err)
	}

	return aiClient, nil
}

func formatSource(source, envVar string) string {
	switch source {
	case "db":
		return "db"
	case "env":
		if envVar != "" {
			return fmt.Sprintf("env (%s)", envVar)
		}
		return "env"
	default:
		return source
	}
}

func outputJSONIndent(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputYAMLDoc(v interface{}) error {
	// JSON round-trip for proto compatibility.
	jdata, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var obj interface{}
	if err := json.Unmarshal(jdata, &obj); err != nil {
		return err
	}
	data, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}
