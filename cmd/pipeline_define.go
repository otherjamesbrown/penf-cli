// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

func newPipelineDefineCmd(deps *PipelineCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "define",
		Short: "Manage pipeline definitions",
		Long: `Manage pipeline definitions — named pipelines with ordered stage lists.

Each pipeline definition specifies which stages run, in what order, with
per-stage configuration (model override, skip-when-low, timeout, etc.).

Commands:
  list    - List all defined pipelines
  show    - Show stages for a specific pipeline
  set     - Update stage configuration within a pipeline
  create  - Create a new pipeline (optionally clone from existing)

Examples:
  # List all pipeline definitions
  penf pipeline define list

  # Show stages for the standard pipeline
  penf pipeline define show standard

  # Override model for a stage
  penf pipeline define set standard extract_assertions --model gemini-2.5-pro

  # Disable a stage
  penf pipeline define set standard extract_assertions --enabled=false

  # Clone a pipeline
  penf pipeline define create transcript --from standard`,
	}

	cmd.AddCommand(newPipelineDefineListCmd(deps))
	cmd.AddCommand(newPipelineDefineShowCmd(deps))
	cmd.AddCommand(newPipelineDefineSetCmd(deps))
	cmd.AddCommand(newPipelineDefineCreateCmd(deps))

	// Default action is list
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPipelineDefineList(cmd.Context(), deps, "text")
	}

	return cmd
}

func newPipelineDefineListCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all pipeline definitions",
		Long: `List all defined pipelines with stage counts.

Examples:
  penf pipeline define list
  penf pipeline define list -o json`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineDefineList(cmd.Context(), deps, outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func newPipelineDefineShowCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "show <pipeline-name>",
		Short: "Show stages for a pipeline",
		Long: `Show the ordered stages for a specific pipeline definition, including
per-stage configuration (model overrides, skip-when-low, timeouts).

Examples:
  penf pipeline define show standard
  penf pipeline define show transcript -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineDefineShow(cmd.Context(), deps, args[0], outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func newPipelineDefineSetCmd(deps *PipelineCommandDeps) *cobra.Command {
	var (
		model       string
		enabled     string
		skipWhenLow string
		promptVer   int32
		timeout     int32
	)

	cmd := &cobra.Command{
		Use:   "set <pipeline-name> <stage>",
		Short: "Update stage configuration in a pipeline",
		Long: `Update per-stage configuration within a pipeline definition.

Flags:
  --model         Override model ID (empty string to clear)
  --enabled       Enable/disable the stage (true/false)
  --skip-when-low Skip stage when triage importance is LOW (true/false)
  --prompt        Override prompt version (0 to clear)
  --timeout       Timeout in seconds

Examples:
  penf pipeline define set standard extract_assertions --model gemini-2.5-pro
  penf pipeline define set standard extract_assertions --enabled=false
  penf pipeline define set standard triage --skip-when-low=false
  penf pipeline define set standard analyze --timeout 180`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineDefineSet(cmd.Context(), deps, args[0], args[1], model, enabled, skipWhenLow, promptVer, timeout, cmd)
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "Model ID override (empty to clear)")
	cmd.Flags().StringVar(&enabled, "enabled", "", "Enable/disable stage (true/false)")
	cmd.Flags().StringVar(&skipWhenLow, "skip-when-low", "", "Skip when triage importance is LOW (true/false)")
	cmd.Flags().Int32Var(&promptVer, "prompt", 0, "Prompt version override (0 to clear)")
	cmd.Flags().Int32Var(&timeout, "timeout", 0, "Timeout in seconds")

	return cmd
}

func newPipelineDefineCreateCmd(deps *PipelineCommandDeps) *cobra.Command {
	var fromPipeline string

	cmd := &cobra.Command{
		Use:   "create <pipeline-name>",
		Short: "Create a new pipeline definition",
		Long: `Create a new pipeline definition, optionally cloning stages from an existing pipeline.

Examples:
  penf pipeline define create transcript --from standard
  penf pipeline define create custom`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineDefineCreate(cmd.Context(), deps, args[0], fromPipeline)
		},
	}

	cmd.Flags().StringVar(&fromPipeline, "from", "", "Clone stages from this pipeline")

	return cmd
}

// =============================================================================
// Run Functions
// =============================================================================

func runPipelineDefineList(ctx context.Context, deps *PipelineCommandDeps, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.TenantID
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.ListPipelineDefinitions(ctx, &pipelinev1.ListPipelineDefinitionsRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return fmt.Errorf("listing pipeline definitions: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Pipelines)
	}

	return outputDefineListHuman(resp.Pipelines)
}

func runPipelineDefineShow(ctx context.Context, deps *PipelineCommandDeps, pipeline string, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.TenantID
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.GetPipelineDefinition(ctx, &pipelinev1.GetPipelineDefinitionRequest{
		TenantId: tenantID,
		Pipeline: pipeline,
	})
	if err != nil {
		return fmt.Errorf("getting pipeline definition: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Definition)
	}

	return outputDefineShowHuman(resp.Definition)
}

func runPipelineDefineSet(ctx context.Context, deps *PipelineCommandDeps, pipeline string, stage string, model string, enabled string, skipWhenLow string, promptVer int32, timeout int32, cmd *cobra.Command) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.TenantID
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	// Build request with only changed fields
	req := &pipelinev1.UpdatePipelineStageConfigRequest{
		TenantId: tenantID,
		Pipeline: pipeline,
		Stage:    stage,
	}

	hasChanges := false

	if cmd.Flags().Changed("model") {
		req.ModelOverride = &model
		hasChanges = true
	}
	if cmd.Flags().Changed("enabled") {
		val := enabled == "true"
		req.Enabled = &val
		hasChanges = true
	}
	if cmd.Flags().Changed("skip-when-low") {
		val := skipWhenLow == "true"
		req.SkipWhenLow = &val
		hasChanges = true
	}
	if cmd.Flags().Changed("prompt") {
		req.PromptOverride = &promptVer
		hasChanges = true
	}
	if cmd.Flags().Changed("timeout") {
		req.TimeoutSeconds = &timeout
		hasChanges = true
	}

	if !hasChanges {
		return fmt.Errorf("no configuration changes specified — use flags like --model, --enabled, --skip-when-low, --prompt, --timeout")
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.UpdatePipelineStageConfig(ctx, req)
	if err != nil {
		return fmt.Errorf("updating stage config: %w", err)
	}

	fmt.Printf("Updated %s/%s: %s\n", pipeline, stage, resp.Message)
	if resp.Stage != nil {
		outputStageDefinition(resp.Stage)
	}

	return nil
}

func runPipelineDefineCreate(ctx context.Context, deps *PipelineCommandDeps, pipeline string, fromPipeline string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.TenantID
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.CreatePipelineDefinition(ctx, &pipelinev1.CreatePipelineDefinitionRequest{
		TenantId:     tenantID,
		Pipeline:     pipeline,
		FromPipeline: fromPipeline,
	})
	if err != nil {
		return fmt.Errorf("creating pipeline definition: %w", err)
	}

	fmt.Printf("%s\n\n", resp.Message)
	if resp.Definition != nil {
		return outputDefineShowHuman(resp.Definition)
	}

	return nil
}

// =============================================================================
// Output Functions
// =============================================================================

func outputDefineListHuman(pipelines []*pipelinev1.PipelineDefinition) error {
	if len(pipelines) == 0 {
		fmt.Println("No pipeline definitions found.")
		return nil
	}

	fmt.Printf("Pipeline Definitions (%d):\n\n", len(pipelines))
	fmt.Println("  PIPELINE          STAGES  SKIP-LOW  MODELS")
	fmt.Println("  --------          ------  --------  ------")

	for _, p := range pipelines {
		skipLowCount := 0
		models := map[string]bool{}
		for _, s := range p.Stages {
			if s.SkipWhenLow {
				skipLowCount++
			}
			if s.ModelOverride != "" {
				models[s.ModelOverride] = true
			}
		}

		modelList := make([]string, 0, len(models))
		for m := range models {
			modelList = append(modelList, m)
		}
		modelStr := "-"
		if len(modelList) > 0 {
			modelStr = strings.Join(modelList, ", ")
		}

		fmt.Printf("  %-17s %3d     %3d       %s\n",
			p.Pipeline,
			len(p.Stages),
			skipLowCount,
			modelStr)
	}

	fmt.Println()
	return nil
}

func outputDefineShowHuman(def *pipelinev1.PipelineDefinition) error {
	if def == nil {
		fmt.Println("Pipeline definition not found.")
		return nil
	}

	fmt.Printf("Pipeline: %s (%d stages)\n\n", def.Pipeline, len(def.Stages))
	fmt.Println("  ORDER  STAGE                 ENABLED  SKIP-LOW  MODEL              TIMEOUT")
	fmt.Println("  -----  -----                 -------  --------  -----              -------")

	for _, s := range def.Stages {
		enabledStr := "\033[32mtrue\033[0m "
		if !s.Enabled {
			enabledStr = "\033[90mfalse\033[0m"
		}

		skipLowStr := "false"
		if s.SkipWhenLow {
			skipLowStr = "true "
		}

		modelStr := "-"
		if s.ModelOverride != "" {
			modelStr = s.ModelOverride
		}

		timeoutStr := "-"
		if s.TimeoutSeconds > 0 {
			timeoutStr = fmt.Sprintf("%ds", s.TimeoutSeconds)
		}

		fmt.Printf("  %-5d  %-20s  %s    %-5s     %-18s %s\n",
			s.StageOrder,
			s.Stage,
			enabledStr,
			skipLowStr,
			modelStr,
			timeoutStr)
	}

	fmt.Println()
	return nil
}

func outputStageDefinition(s *pipelinev1.PipelineStageDefinition) {
	fmt.Printf("  Stage:        %s\n", s.Stage)
	fmt.Printf("  Order:        %d\n", s.StageOrder)
	fmt.Printf("  Enabled:      %t\n", s.Enabled)
	fmt.Printf("  Skip-when-low: %t\n", s.SkipWhenLow)
	if s.ModelOverride != "" {
		fmt.Printf("  Model:        %s\n", s.ModelOverride)
	}
	if s.PromptOverride > 0 {
		fmt.Printf("  Prompt:       v%d\n", s.PromptOverride)
	}
	if s.TimeoutSeconds > 0 {
		fmt.Printf("  Timeout:      %ds\n", s.TimeoutSeconds)
	}
	fmt.Println()
}
