// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

// newPipelineDescribeCmd creates the pipeline describe command.
func newPipelineDescribeCmd(deps *PipelineCommandDeps) *cobra.Command {
	var stage string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Show pipeline stage registry",
		Long: `Show pipeline stage information.

Without --stage, displays all stages in dependency order.
With --stage, displays detailed information for a specific stage.

Examples:
  # Show all stages
  penf pipeline describe

  # Show specific stage
  penf pipeline describe --stage triage

  # Output as JSON
  penf pipeline describe -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineDescribe(cmd.Context(), deps, stage, outputFormat)
		},
	}

	cmd.Flags().StringVar(&stage, "stage", "", "Show detail for specific stage")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineDescribe(ctx context.Context, deps *PipelineCommandDeps, stage string, outputFormat string) error {
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

	resp, err := client.DescribePipeline(ctx, &pipelinev1.DescribePipelineRequest{
		Stage: stage,
	})
	if err != nil {
		return fmt.Errorf("describing pipeline: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	return outputPipelineDescribeHuman(resp.Stages, stage)
}

func outputPipelineDescribeHuman(stages []*pipelinev1.StageInfo, filterStage string) error {
	if len(stages) == 0 {
		fmt.Println("No pipeline stages found.")
		return nil
	}

	// If single stage detail view
	if filterStage != "" && len(stages) == 1 {
		s := stages[0]
		fmt.Printf("Stage: %s\n", s.Stage)
		fmt.Printf("Display Name: %s\n", s.DisplayName)
		fmt.Printf("Type: %s", s.StageType)
		if s.ModelDependent {
			fmt.Printf(" (model-dependent)")
		}
		fmt.Println()
		fmt.Printf("Description: %s\n", s.Description)

		if s.HasPrompt {
			fmt.Printf("Has Prompt: Yes (active version: %d)\n", s.ActivePromptVersion)
		} else {
			fmt.Println("Has Prompt: No")
		}

		if s.ActiveModelId != "" {
			fmt.Printf("Active Model: %s\n", s.ActiveModelId)
		}

		if len(s.DependsOn) > 0 {
			fmt.Printf("Depends On: %s\n", strings.Join(s.DependsOn, ", "))
		} else {
			fmt.Println("Depends On: -")
		}

		if len(s.Downstream) > 0 {
			fmt.Printf("Downstream: %s\n", strings.Join(s.Downstream, ", "))
		} else {
			fmt.Println("Downstream: -")
		}

		return nil
	}

	// All stages table view
	fmt.Println("Pipeline Stages")
	fmt.Println("===============")
	fmt.Println("  STAGE            TYPE         MODEL  PROMPT  DEPENDS ON           DOWNSTREAM")
	fmt.Println("  -----            ----         -----  ------  ----------           ----------")

	for _, s := range stages {
		modelStatus := "No"
		if s.ActiveModelId != "" {
			modelStatus = "Yes"
		}

		promptStatus := "No"
		if s.HasPrompt {
			promptStatus = fmt.Sprintf("v%d", s.ActivePromptVersion)
		}

		dependsOn := "-"
		if len(s.DependsOn) > 0 {
			dependsOn = strings.Join(s.DependsOn, ", ")
			if len(dependsOn) > 20 {
				dependsOn = dependsOn[:17] + "..."
			}
		}

		downstream := "-"
		if len(s.Downstream) > 0 {
			downstream = strings.Join(s.Downstream, ", ")
			if len(downstream) > 20 {
				downstream = downstream[:17] + "..."
			}
		}

		fmt.Printf("  %-16s %-12s %-6s %-7s %-20s %s\n",
			s.Stage, s.StageType, modelStatus, promptStatus, dependsOn, downstream)
	}

	return nil
}

// newPipelinePromptCmd creates the pipeline prompt command group.
func newPipelinePromptCmd(deps *PipelineCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Manage pipeline prompt templates",
		Long: `Manage versioned prompt templates for pipeline stages.

Subcommands:
  show      - Show active or specific prompt version
  history   - List all versions for a stage
  diff      - Compare two prompt versions
  update    - Create new prompt version
  rollback  - Activate previous prompt version
  export    - Export prompts as JSON`,
	}

	cmd.AddCommand(newPipelinePromptShowCmd(deps))
	cmd.AddCommand(newPipelinePromptHistoryCmd(deps))
	cmd.AddCommand(newPipelinePromptDiffCmd(deps))
	cmd.AddCommand(newPipelinePromptUpdateCmd(deps))
	cmd.AddCommand(newPipelinePromptRollbackCmd(deps))
	cmd.AddCommand(newPipelinePromptExportCmd(deps))

	return cmd
}

func newPipelinePromptShowCmd(deps *PipelineCommandDeps) *cobra.Command {
	var version int
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "show <stage>",
		Short: "Show prompt template for a stage",
		Long: `Show the active prompt template or a specific version.

Examples:
  # Show active prompt
  penf pipeline prompt show triage

  # Show specific version
  penf pipeline prompt show triage --version 3

  # Output as JSON
  penf pipeline prompt show triage -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelinePromptShow(cmd.Context(), deps, args[0], version, outputFormat)
		},
	}

	cmd.Flags().IntVar(&version, "version", 0, "Specific version to show (0 = active)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelinePromptShow(ctx context.Context, deps *PipelineCommandDeps, stage string, version int, outputFormat string) error {
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

	resp, err := client.GetPrompt(ctx, &pipelinev1.GetPromptRequest{
		Stage:   stage,
		Version: int32(version),
	})
	if err != nil {
		return fmt.Errorf("getting prompt: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Prompt)
	}

	return outputPromptHuman(resp.Prompt)
}

func outputPromptHuman(prompt *pipelinev1.PromptTemplate) error {
	fmt.Printf("Stage: %s (Version %d)\n", prompt.Stage, prompt.Version)
	if prompt.IsActive {
		fmt.Println("Status: \033[32mActive\033[0m")
	} else {
		fmt.Println("Status: Inactive")
	}
	fmt.Printf("Created By: %s\n", prompt.CreatedBy)
	if prompt.CreatedAt != nil {
		fmt.Printf("Created At: %s\n", prompt.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}
	if prompt.Description != "" {
		fmt.Printf("Description: %s\n", prompt.Description)
	}
	fmt.Println()
	fmt.Println("Content:")
	fmt.Println("--------")
	fmt.Println(prompt.Content)
	return nil
}

func newPipelinePromptHistoryCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "history <stage>",
		Short: "List all prompt versions for a stage",
		Long: `List all prompt versions for a stage in reverse chronological order.

Examples:
  # List all versions
  penf pipeline prompt history triage

  # Output as JSON
  penf pipeline prompt history triage -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelinePromptHistory(cmd.Context(), deps, args[0], outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelinePromptHistory(ctx context.Context, deps *PipelineCommandDeps, stage string, outputFormat string) error {
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

	resp, err := client.ListPromptVersions(ctx, &pipelinev1.ListPromptVersionsRequest{
		Stage: stage,
	})
	if err != nil {
		return fmt.Errorf("listing prompt versions: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Versions)
	}

	return outputPromptHistoryHuman(resp.Versions, stage)
}

func outputPromptHistoryHuman(versions []*pipelinev1.PromptTemplate, stage string) error {
	if len(versions) == 0 {
		fmt.Printf("No prompt versions found for stage: %s\n", stage)
		return nil
	}

	fmt.Printf("Prompt History for Stage: %s\n\n", stage)
	fmt.Println("VERSION  STATUS    CREATED AT           CREATED BY       DESCRIPTION")
	fmt.Println("-------  ------    ----------           ----------       -----------")

	for _, v := range versions {
		status := "Inactive"
		if v.IsActive {
			status = "\033[32mActive\033[0m  "
		}

		createdAt := "-"
		if v.CreatedAt != nil {
			createdAt = v.CreatedAt.AsTime().Format("2006-01-02 15:04:05")
		}

		createdBy := v.CreatedBy
		if len(createdBy) > 16 {
			createdBy = createdBy[:13] + "..."
		}

		description := v.Description
		if len(description) > 40 {
			description = description[:37] + "..."
		}

		fmt.Printf("%-7d  %s  %-19s  %-16s %s\n",
			v.Version, status, createdAt, createdBy, description)
	}

	return nil
}

func newPipelinePromptDiffCmd(deps *PipelineCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <stage> <version1> <version2>",
		Short: "Show differences between two prompt versions",
		Long: `Compare two prompt versions and show their differences.

Examples:
  # Compare versions 1 and 2
  penf pipeline prompt diff triage 1 2

  # Compare version 1 with current active
  penf pipeline prompt diff triage 1 0`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			var v1, v2 int
			if _, err := fmt.Sscanf(args[1], "%d", &v1); err != nil {
				return fmt.Errorf("invalid version1: %s", args[1])
			}
			if _, err := fmt.Sscanf(args[2], "%d", &v2); err != nil {
				return fmt.Errorf("invalid version2: %s", args[2])
			}
			return runPipelinePromptDiff(cmd.Context(), deps, args[0], v1, v2)
		},
	}

	return cmd
}

func runPipelinePromptDiff(ctx context.Context, deps *PipelineCommandDeps, stage string, v1 int, v2 int) error {
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

	// Fetch both versions
	resp1, err := client.GetPrompt(ctx, &pipelinev1.GetPromptRequest{
		Stage:   stage,
		Version: int32(v1),
	})
	if err != nil {
		return fmt.Errorf("getting version %d: %w", v1, err)
	}

	resp2, err := client.GetPrompt(ctx, &pipelinev1.GetPromptRequest{
		Stage:   stage,
		Version: int32(v2),
	})
	if err != nil {
		return fmt.Errorf("getting version %d: %w", v2, err)
	}

	return outputPromptDiff(resp1.Prompt, resp2.Prompt)
}

func outputPromptDiff(p1 *pipelinev1.PromptTemplate, p2 *pipelinev1.PromptTemplate) error {
	fmt.Printf("Comparing: Version %d vs Version %d\n\n", p1.Version, p2.Version)

	// Split content into lines
	lines1 := strings.Split(p1.Content, "\n")
	lines2 := strings.Split(p2.Content, "\n")

	// Simple line-by-line diff
	maxLen := len(lines1)
	if len(lines2) > maxLen {
		maxLen = len(lines2)
	}

	hasChanges := false
	for i := 0; i < maxLen; i++ {
		line1 := ""
		if i < len(lines1) {
			line1 = lines1[i]
		}

		line2 := ""
		if i < len(lines2) {
			line2 = lines2[i]
		}

		if line1 != line2 {
			hasChanges = true
			if line1 != "" && line2 == "" {
				// Line removed
				fmt.Printf("\033[31m- %s\033[0m\n", line1)
			} else if line1 == "" && line2 != "" {
				// Line added
				fmt.Printf("\033[32m+ %s\033[0m\n", line2)
			} else {
				// Line changed
				fmt.Printf("\033[31m- %s\033[0m\n", line1)
				fmt.Printf("\033[32m+ %s\033[0m\n", line2)
			}
		} else {
			// Line unchanged
			fmt.Printf("  %s\n", line1)
		}
	}

	if !hasChanges {
		fmt.Println("No differences found.")
	}

	return nil
}

func newPipelinePromptUpdateCmd(deps *PipelineCommandDeps) *cobra.Command {
	var contentPath string
	var description string
	var author string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "update <stage>",
		Short: "Create new prompt version",
		Long: `Create a new prompt version and set it as active.

The content can be provided via file path or stdin (-).

Examples:
  # Update from file
  penf pipeline prompt update triage --content prompt.txt --description "Improved clarity"

  # Update from stdin
  cat prompt.txt | penf pipeline prompt update triage --content - --description "New prompt"

  # With author
  penf pipeline prompt update triage --content prompt.txt --description "Fix" --author "dev@example.com"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelinePromptUpdate(cmd.Context(), deps, args[0], contentPath, description, author, outputFormat)
		},
	}

	cmd.Flags().StringVar(&contentPath, "content", "", "File path or - for stdin (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description of changes")
	cmd.Flags().StringVar(&author, "author", "", "Author of changes")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	_ = cmd.MarkFlagRequired("content")

	return cmd
}

func runPipelinePromptUpdate(ctx context.Context, deps *PipelineCommandDeps, stage string, contentPath string, description string, author string, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Read content
	var content string
	if contentPath == "-" {
		// Read from stdin
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		content = strings.Join(lines, "\n")
	} else {
		// Read from file
		data, err := os.ReadFile(contentPath)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", contentPath, err)
		}
		content = string(data)
	}

	if content == "" {
		return fmt.Errorf("content is empty")
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.UpdatePrompt(ctx, &pipelinev1.UpdatePromptRequest{
		Stage:       stage,
		Content:     content,
		Description: description,
		CreatedBy:   author,
	})
	if err != nil {
		return fmt.Errorf("updating prompt: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	fmt.Printf("✓ %s\n", resp.Message)
	fmt.Printf("Created version %d for stage: %s\n", resp.Prompt.Version, stage)
	return nil
}

func newPipelinePromptRollbackCmd(deps *PipelineCommandDeps) *cobra.Command {
	var version int
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "rollback <stage>",
		Short: "Activate a previous prompt version",
		Long: `Activate a previous prompt version as the active version.

Examples:
  # Rollback to version 2
  penf pipeline prompt rollback triage --version 2

  # Output as JSON
  penf pipeline prompt rollback triage --version 2 -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if version == 0 {
				return fmt.Errorf("--version is required")
			}
			return runPipelinePromptRollback(cmd.Context(), deps, args[0], version, outputFormat)
		},
	}

	cmd.Flags().IntVar(&version, "version", 0, "Version to activate (required)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	_ = cmd.MarkFlagRequired("version")

	return cmd
}

func runPipelinePromptRollback(ctx context.Context, deps *PipelineCommandDeps, stage string, version int, outputFormat string) error {
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

	resp, err := client.RollbackPrompt(ctx, &pipelinev1.RollbackPromptRequest{
		Stage:   stage,
		Version: int32(version),
	})
	if err != nil {
		return fmt.Errorf("rolling back prompt: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	fmt.Printf("✓ %s\n", resp.Message)
	fmt.Printf("Version %d is now active for stage: %s\n", version, stage)
	return nil
}

func newPipelinePromptExportCmd(deps *PipelineCommandDeps) *cobra.Command {
	var stage string
	var format string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export prompt templates as JSON",
		Long: `Export prompt templates in portable JSON format.

Without --stage, exports all stages. With --stage, exports only that stage.

Examples:
  # Export all prompts
  penf pipeline prompt export

  # Export specific stage
  penf pipeline prompt export --stage triage

  # Export and save to file
  penf pipeline prompt export > prompts.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelinePromptExport(cmd.Context(), deps, stage, format)
		},
	}

	cmd.Flags().StringVar(&stage, "stage", "", "Export specific stage (empty = all)")
	cmd.Flags().StringVar(&format, "format", "json", "Export format (currently only json)")

	return cmd
}

func runPipelinePromptExport(ctx context.Context, deps *PipelineCommandDeps, stage string, format string) error {
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

	resp, err := client.ExportPrompt(ctx, &pipelinev1.ExportPromptRequest{
		Stage: stage,
	})
	if err != nil {
		return fmt.Errorf("exporting prompts: %w", err)
	}

	// Output the JSON directly
	fmt.Println(resp.Json)
	return nil
}

// newPipelineHistoryCmd creates the pipeline history command.
func newPipelineHistoryCmd(deps *PipelineCommandDeps) *cobra.Command {
	var sourceID int64
	var stage string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show pipeline execution history for a source",
		Long: `Show all pipeline runs for a specific source.

Displays the complete execution history including stages, models, prompts,
status, and duration.

Examples:
  # Show history for source
  penf pipeline history --source 42

  # Filter by stage
  penf pipeline history --source 42 --stage triage

  # Output as JSON
  penf pipeline history --source 42 -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sourceID == 0 {
				return fmt.Errorf("--source is required")
			}
			return runPipelineHistory(cmd.Context(), deps, sourceID, stage, outputFormat)
		},
	}

	cmd.Flags().Int64Var(&sourceID, "source", 0, "Source ID (required)")
	cmd.Flags().StringVar(&stage, "stage", "", "Filter by stage")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	_ = cmd.MarkFlagRequired("source")

	return cmd
}

func runPipelineHistory(ctx context.Context, deps *PipelineCommandDeps, sourceID int64, stage string, outputFormat string) error {
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

	resp, err := client.GetSourceHistory(ctx, &pipelinev1.GetSourceHistoryRequest{
		SourceId: sourceID,
		Stage:    stage,
	})
	if err != nil {
		return fmt.Errorf("getting source history: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	return outputPipelineHistoryHuman(resp.Runs, sourceID)
}

func outputPipelineHistoryHuman(runs []*pipelinev1.PipelineRun, sourceID int64) error {
	if len(runs) == 0 {
		fmt.Printf("No pipeline history found for source %d.\n", sourceID)
		return nil
	}

	fmt.Printf("Pipeline History for Source %d\n", sourceID)
	fmt.Println("==============================")
	fmt.Println("RUN   STAGE       MODEL            PROMPT  STATUS      DURATION  TIMESTAMP")
	fmt.Println("---   -----       -----            ------  ------      --------  ---------")

	for _, run := range runs {
		model := run.ModelId
		if model == "" {
			model = "-"
		} else if len(model) > 16 {
			model = model[:13] + "..."
		}

		prompt := "-"
		if run.PromptVersion > 0 {
			prompt = fmt.Sprintf("v%d", run.PromptVersion)
		}

		statusColor := "\033[32m" // Green
		if run.Status == "failed" {
			statusColor = "\033[31m" // Red
		} else if run.Status == "superseded" {
			statusColor = "\033[33m" // Yellow
		}

		duration := fmt.Sprintf("%dms", run.DurationMs)
		if run.DurationMs > 1000 {
			duration = fmt.Sprintf("%.1fs", float64(run.DurationMs)/1000)
		}

		timestamp := "-"
		if run.CreatedAt != nil {
			timestamp = run.CreatedAt.AsTime().Format("15:04:05")
		}

		fmt.Printf("%-5d %-11s %-16s %-7s %s%-11s\033[0m %-9s %s\n",
			run.Id, run.Stage, model, prompt, statusColor, run.Status, duration, timestamp)
	}

	return nil
}
