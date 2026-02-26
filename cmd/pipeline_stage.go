// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

// Known pipeline stages for validation (canonical names matching backend).
var knownPipelineStages = []string{
	"triage",
	"segment",
	"extract_ner",
	"extract_semantic",
	"extract_assertions",
	"analyze",
	"summary",
	"embed",
}

func newPipelineStageCmd(deps *PipelineCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stage",
		Short: "View and manage per-stage pipeline configuration",
		Long: `View and manage per-stage pipeline configuration (model, timeout, LLM params).

Each pipeline stage can have its own model, timeout, and LLM parameter settings.
This command provides a unified view, making it easy to see and adjust how each
stage is configured.

Commands:
  list   - Show all stages with model + timeout configuration
  set    - Update model and/or timeout for a stage
  reset  - Reset a stage to default configuration

Examples:
  # Show all stage configurations
  penf pipeline stage list

  # Set model and timeout for triage
  penf pipeline stage set triage --model qwen2.5:7b --timeout 60s

  # Reset triage to defaults
  penf pipeline stage reset triage`,
	}

	cmd.AddCommand(newPipelineStageListCmd(deps))
	cmd.AddCommand(newPipelineStageSetCmd(deps))
	cmd.AddCommand(newPipelineStageResetCmd(deps))

	// Default action is list
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPipelineStageList(cmd.Context(), deps, "text")
	}

	return cmd
}

func newPipelineStageListCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show all stages with model + timeout configuration",
		Long: `Show per-stage pipeline configuration including model, timeout, and heartbeat
for each processing stage.

Examples:
  # Show all stage configurations
  penf pipeline stage list

  # Output as JSON
  penf pipeline stage list -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineStageList(cmd.Context(), deps, outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func newPipelineStageSetCmd(deps *PipelineCommandDeps) *cobra.Command {
	var model string
	var timeout string
	var heartbeat string
	var reason string
	var pipeline string
	var temperature float32
	var maxTokens int32
	var maxRetries int32

	cmd := &cobra.Command{
		Use:   "set <stage>",
		Short: "Set model, timeout, or LLM parameters for a pipeline stage",
		Long: `Update the configuration for a specific pipeline stage.

Stages: triage, segment, extract_ner, extract_semantic, extract_assertions, analyze, summary, embed

When setting --timeout, the heartbeat defaults to timeout/4 unless explicitly
specified with --heartbeat.

LLM parameters (--temperature, --max-tokens, --max-retries) are stored per-pipeline
in pipeline_definitions. Use --pipeline to target a specific pipeline (default: standard).

Examples:
  # Set model for triage
  penf pipeline stage set triage --model qwen2.5:7b

  # Set timeout for extract_ner
  penf pipeline stage set extract_ner --timeout 60s

  # Set LLM parameters for analyze
  penf pipeline stage set analyze --temperature 0.3 --max-tokens 4096

  # Set model and timeout together
  penf pipeline stage set triage --model qwen2.5:7b --timeout 60s`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stage := args[0]
			if !isValidPipelineStage(stage) {
				return fmt.Errorf("invalid stage: %s (valid stages: %s)", stage, strings.Join(knownPipelineStages, ", "))
			}
			hasTemperature := cmd.Flags().Changed("temperature")
			hasMaxTokens := cmd.Flags().Changed("max-tokens")
			hasMaxRetries := cmd.Flags().Changed("max-retries")
			if model == "" && timeout == "" && heartbeat == "" && !hasTemperature && !hasMaxTokens && !hasMaxRetries {
				return fmt.Errorf("at least one of --model, --timeout, --heartbeat, --temperature, --max-tokens, or --max-retries must be specified")
			}
			return runPipelineStageSet(cmd.Context(), deps, stage, model, timeout, heartbeat, reason, pipeline,
				temperature, hasTemperature, maxTokens, hasMaxTokens, maxRetries, hasMaxRetries)
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "Model ID (e.g., qwen2.5:7b, gemini-2.0-flash)")
	cmd.Flags().StringVar(&timeout, "timeout", "", "StartToClose timeout (e.g., 60s, 5m)")
	cmd.Flags().StringVar(&heartbeat, "heartbeat", "", "Heartbeat timeout (e.g., 15s, defaults to timeout/4)")
	cmd.Flags().StringVar(&reason, "reason", "Updated via CLI", "Reason for the change")
	cmd.Flags().StringVar(&pipeline, "pipeline", "standard", "Pipeline name for LLM param updates")
	cmd.Flags().Float32Var(&temperature, "temperature", 0, "LLM temperature (0.0-2.0)")
	cmd.Flags().Int32Var(&maxTokens, "max-tokens", 0, "Max output tokens")
	cmd.Flags().Int32Var(&maxRetries, "max-retries", 0, "Max retry attempts on LLM failure")

	return cmd
}

func newPipelineStageResetCmd(deps *PipelineCommandDeps) *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "reset <stage>",
		Short: "Reset a pipeline stage to default configuration",
		Long: `Reset the model and timeout configuration for a stage back to defaults.

This removes any per-stage overrides, reverting to the system default values.

Examples:
  # Reset triage to defaults
  penf pipeline stage reset triage

  # Reset with reason
  penf pipeline stage reset triage --reason "Reverting to defaults after testing"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stage := args[0]
			if !isValidPipelineStage(stage) {
				return fmt.Errorf("invalid stage: %s (valid stages: %s)", stage, strings.Join(knownPipelineStages, ", "))
			}
			return runPipelineStageReset(cmd.Context(), deps, stage, reason)
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "Reset to defaults via CLI", "Reason for the reset")

	return cmd
}

func isValidPipelineStage(stage string) bool {
	for _, s := range knownPipelineStages {
		if s == stage {
			return true
		}
	}
	return false
}

func runPipelineStageList(ctx context.Context, deps *PipelineCommandDeps, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.GetStageConfig(ctx, &pipelinev1.GetStageConfigRequest{
		TenantId: cfg.TenantID,
	})
	if err != nil {
		return fmt.Errorf("getting stage config: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Stages)
	}

	return outputStageConfigListHuman(resp.Stages)
}

func runPipelineStageSet(ctx context.Context, deps *PipelineCommandDeps, stage string, model string, timeout string, heartbeat string, reason string, pipeline string,
	temperature float32, hasTemperature bool, maxTokens int32, hasMaxTokens bool, maxRetries int32, hasMaxRetries bool) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	pipelineClient := pipelinev1.NewPipelineServiceClient(conn)

	updatedBy := os.Getenv("USER")
	if updatedBy == "" {
		updatedBy = "cli"
	}

	// Update timeout if specified
	if timeout != "" {
		// Validate duration
		if _, err := time.ParseDuration(timeout); err != nil {
			return fmt.Errorf("invalid timeout duration '%s': %v", timeout, err)
		}

		stcKey := fmt.Sprintf("timeout.stage.%s.start_to_close", stage)
		_, err := pipelineClient.UpdateTimeoutConfig(ctx, &pipelinev1.UpdateTimeoutConfigRequest{
			Key:       stcKey,
			Value:     timeout,
			UpdatedBy: updatedBy,
			Reason:    reason,
		})
		if err != nil {
			return fmt.Errorf("updating timeout for %s: %w", stage, err)
		}
		fmt.Printf("Updated %s start_to_close timeout: %s\n", stage, timeout)

		// Set heartbeat: explicit value, or default to timeout/4
		hbValue := heartbeat
		if hbValue == "" {
			dur, _ := time.ParseDuration(timeout)
			hbValue = (dur / 4).String()
		}
		if _, err := time.ParseDuration(hbValue); err != nil {
			return fmt.Errorf("invalid heartbeat duration '%s': %v", hbValue, err)
		}

		hbKey := fmt.Sprintf("timeout.stage.%s.heartbeat", stage)
		_, err = pipelineClient.UpdateTimeoutConfig(ctx, &pipelinev1.UpdateTimeoutConfigRequest{
			Key:       hbKey,
			Value:     hbValue,
			UpdatedBy: updatedBy,
			Reason:    reason,
		})
		if err != nil {
			return fmt.Errorf("updating heartbeat for %s: %w", stage, err)
		}
		fmt.Printf("Updated %s heartbeat timeout: %s\n", stage, hbValue)
	} else if heartbeat != "" {
		// Only heartbeat specified (no timeout)
		if _, err := time.ParseDuration(heartbeat); err != nil {
			return fmt.Errorf("invalid heartbeat duration '%s': %v", heartbeat, err)
		}

		hbKey := fmt.Sprintf("timeout.stage.%s.heartbeat", stage)
		_, err := pipelineClient.UpdateTimeoutConfig(ctx, &pipelinev1.UpdateTimeoutConfigRequest{
			Key:       hbKey,
			Value:     heartbeat,
			UpdatedBy: updatedBy,
			Reason:    reason,
		})
		if err != nil {
			return fmt.Errorf("updating heartbeat for %s: %w", stage, err)
		}
		fmt.Printf("Updated %s heartbeat timeout: %s\n", stage, heartbeat)
	}

	// Update model if specified
	if model != "" {
		modelKey := fmt.Sprintf("model.stage.%s", stage)
		_, err := pipelineClient.UpdateTimeoutConfig(ctx, &pipelinev1.UpdateTimeoutConfigRequest{
			Key:       modelKey,
			Value:     model,
			UpdatedBy: updatedBy,
			Reason:    reason,
		})
		if err != nil {
			return fmt.Errorf("updating model for %s: %w", stage, err)
		}
		fmt.Printf("Updated %s model: %s\n", stage, model)
	}

	// Update LLM parameters via UpdatePipelineStageConfig
	if hasTemperature || hasMaxTokens || hasMaxRetries {
		tenantID := cfg.TenantID
		if tenantID == "" {
			return fmt.Errorf("tenant ID required for LLM params: set via 'penf config set tenant_id <id>'")
		}

		req := &pipelinev1.UpdatePipelineStageConfigRequest{
			TenantId: tenantID,
			Pipeline: pipeline,
			Stage:    stage,
		}
		if hasTemperature {
			req.Temperature = &temperature
		}
		if hasMaxTokens {
			req.MaxTokens = &maxTokens
		}
		if hasMaxRetries {
			req.MaxRetries = &maxRetries
		}

		resp, err := pipelineClient.UpdatePipelineStageConfig(ctx, req)
		if err != nil {
			return fmt.Errorf("updating LLM params for %s: %w", stage, err)
		}

		if hasTemperature {
			fmt.Printf("Updated %s temperature: %.2f\n", stage, temperature)
		}
		if hasMaxTokens {
			fmt.Printf("Updated %s max_tokens: %d\n", stage, maxTokens)
		}
		if hasMaxRetries {
			fmt.Printf("Updated %s max_retries: %d\n", stage, maxRetries)
		}
		_ = resp
	}

	fmt.Println("\nConfiguration changes take effect for new workflow activities.")
	return nil
}

func runPipelineStageReset(ctx context.Context, deps *PipelineCommandDeps, stage string, reason string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	pipelineClient := pipelinev1.NewPipelineServiceClient(conn)

	// Get current config to show what's being reset
	resp, err := pipelineClient.GetStageConfig(ctx, &pipelinev1.GetStageConfigRequest{
		Stage:    stage,
		TenantId: cfg.TenantID,
	})
	if err != nil {
		return fmt.Errorf("getting current stage config: %w", err)
	}

	if len(resp.Stages) == 0 {
		return fmt.Errorf("stage %s not found", stage)
	}

	current := resp.Stages[0]
	fmt.Printf("Resetting %s configuration:\n", stage)
	fmt.Printf("  Current model:     %s (source: %s)\n", current.Model, current.ModelSource)
	fmt.Printf("  Current timeout:   %s (source: %s)\n", current.Timeout, current.TimeoutSource)
	fmt.Printf("  Current heartbeat: %s\n", current.Heartbeat)
	if current.Temperature != nil {
		fmt.Printf("  Current temperature: %.2f\n", *current.Temperature)
	}
	if current.MaxTokens != nil {
		fmt.Printf("  Current max_tokens:  %d\n", *current.MaxTokens)
	}
	if current.MaxRetries != nil {
		fmt.Printf("  Current max_retries: %d\n", *current.MaxRetries)
	}

	updatedBy := os.Getenv("USER")
	if updatedBy == "" {
		updatedBy = "cli"
	}

	// Reset timeout keys to defaults
	keysToReset := []string{
		fmt.Sprintf("timeout.stage.%s.start_to_close", stage),
		fmt.Sprintf("timeout.stage.%s.heartbeat", stage),
	}

	// Look up defaults from the current config response
	defaults := map[string]string{
		"triage":             "120s",
		"extract_ner":        "120s",
		"extract_semantic":   "120s",
		"extract_assertions": "120s",
		"analyze":            "600s",
		"embed":              "120s",
	}
	heartbeatDefaults := map[string]string{
		"triage":             "30s",
		"extract_ner":        "30s",
		"extract_semantic":   "30s",
		"extract_assertions": "30s",
		"analyze":            "300s",
		"embed":              "30s",
	}

	defaultValues := []string{
		defaults[stage],
		heartbeatDefaults[stage],
	}

	for i, key := range keysToReset {
		_, err := pipelineClient.UpdateTimeoutConfig(ctx, &pipelinev1.UpdateTimeoutConfigRequest{
			Key:       key,
			Value:     defaultValues[i],
			UpdatedBy: updatedBy,
			Reason:    reason,
		})
		if err != nil {
			return fmt.Errorf("resetting %s: %w", key, err)
		}
	}

	fmt.Printf("\nReset %s timeouts to defaults (timeout: %s, heartbeat: %s)\n",
		stage, defaults[stage], heartbeatDefaults[stage])
	fmt.Println("Configuration changes take effect for new workflow activities.")

	return nil
}

func outputStageConfigListHuman(stages []*pipelinev1.StageConfigEntry) error {
	if len(stages) == 0 {
		fmt.Println("No stage configuration found.")
		return nil
	}

	fmt.Println("Pipeline Stage Configuration")
	fmt.Println("============================")
	fmt.Println()
	fmt.Printf("  %-22s %-20s %-10s %-10s %-6s %-8s %-8s %s\n",
		"STAGE", "MODEL", "TIMEOUT", "HEARTBEAT", "TEMP", "TOKENS", "RETRIES", "SOURCE")
	fmt.Printf("  %-22s %-20s %-10s %-10s %-6s %-8s %-8s %s\n",
		"-----", "-----", "-------", "---------", "----", "------", "-------", "------")

	for _, s := range stages {
		modelDisplay := s.Model
		if modelDisplay == "" {
			modelDisplay = "-"
		}
		if len(modelDisplay) > 20 {
			modelDisplay = modelDisplay[:17] + "..."
		}

		source := s.ModelSource
		if s.TimeoutSource != "" && s.TimeoutSource != s.ModelSource {
			source = fmt.Sprintf("%s/%s", s.ModelSource, s.TimeoutSource)
		}

		tempDisplay := "-"
		if s.Temperature != nil {
			tempDisplay = fmt.Sprintf("%.1f", *s.Temperature)
		}
		tokensDisplay := "-"
		if s.MaxTokens != nil {
			tokensDisplay = fmt.Sprintf("%d", *s.MaxTokens)
		}
		retriesDisplay := "-"
		if s.MaxRetries != nil {
			retriesDisplay = fmt.Sprintf("%d", *s.MaxRetries)
		}

		fmt.Printf("  %-22s %-20s %-10s %-10s %-6s %-8s %-8s %s\n",
			s.Stage, modelDisplay, s.Timeout, s.Heartbeat,
			tempDisplay, tokensDisplay, retriesDisplay, source)
	}

	fmt.Println()
	fmt.Println("To update: penf pipeline stage set <stage> --model <model> --timeout <duration> --temperature <float>")

	return nil
}
