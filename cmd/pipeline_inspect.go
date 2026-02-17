// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

// newPipelineInspectCmd creates the pipeline inspect command.
func newPipelineInspectCmd(deps *PipelineCommandDeps) *cobra.Command {
	var stage string
	var showInput bool
	var showOutput bool
	var showParsed bool
	var diff bool
	var noTruncate bool
	var limit int
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "inspect <source-id>",
		Short: "Inspect pipeline stage IO data",
		Long: `Inspect input, output, and parsed data for pipeline stages.

Without IO flags (--show-input, --show-output, --show-parsed), displays an
overview table of all pipeline runs for the source.

With IO flags, displays detailed IO data for each run.

With --diff, compares the two most recent runs of the filtered stage.

Examples:
  # Show overview for source
  penf pipeline inspect 42

  # Show input/output for specific stage
  penf pipeline inspect 42 --stage triage --show-input --show-output

  # Show parsed data only
  penf pipeline inspect 42 --stage extract_ner --show-parsed

  # Compare two most recent runs
  penf pipeline inspect 42 --stage triage --diff

  # Show full data without truncation
  penf pipeline inspect 42 --stage triage --show-input --no-truncate`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid source ID: %s", args[0])
			}
			return runPipelineInspect(cmd.Context(), deps, sourceID, stage, showInput, showOutput, showParsed, diff, noTruncate, limit, outputFormat)
		},
	}

	cmd.Flags().StringVar(&stage, "stage", "", "Filter by specific stage")
	cmd.Flags().BoolVar(&showInput, "show-input", false, "Show input/prompt data")
	cmd.Flags().BoolVar(&showOutput, "show-output", false, "Show output/response data")
	cmd.Flags().BoolVar(&showParsed, "show-parsed", false, "Show parsed/structured data")
	cmd.Flags().BoolVar(&diff, "diff", false, "Compare two most recent runs")
	cmd.Flags().BoolVar(&noTruncate, "no-truncate", false, "Show full data without truncation")
	cmd.Flags().IntVarP(&limit, "limit", "l", 3, "Maximum number of runs per stage")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineInspect(ctx context.Context, deps *PipelineCommandDeps, sourceID int64, stage string, showInput bool, showOutput bool, showParsed bool, diff bool, noTruncate bool, limit int, outputFormat string) error {
	cfg := deps.Config
	if cfg == nil {
		var err error
		cfg, err = deps.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		deps.Config = cfg
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	// Handle --diff flag
	if diff {
		if stage == "" {
			return fmt.Errorf("--diff requires --stage flag")
		}
		return runPipelineInspectDiff(ctx, client, sourceID, stage, outputFormat)
	}

	// Fetch inspect data
	resp, err := client.InspectStage(ctx, &pipelinev1.InspectStageRequest{
		SourceId: sourceID,
		Stage:    stage,
		Limit:    int32(limit),
	})
	if err != nil {
		return fmt.Errorf("inspecting stage: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	// Determine if we're showing IO data
	showIOData := showInput || showOutput || showParsed

	if showIOData {
		return outputPipelineInspectIOHuman(resp.Runs, sourceID, showInput, showOutput, showParsed, noTruncate)
	}

	return outputPipelineInspectOverviewHuman(resp.Runs, sourceID)
}

func runPipelineInspectDiff(ctx context.Context, client pipelinev1.PipelineServiceClient, sourceID int64, stage string, outputFormat string) error {
	// First, get the most recent runs for this stage
	resp, err := client.InspectStage(ctx, &pipelinev1.InspectStageRequest{
		SourceId: sourceID,
		Stage:    stage,
		Limit:    2,
	})
	if err != nil {
		return fmt.Errorf("fetching runs for diff: %w", err)
	}

	if len(resp.Runs) < 2 {
		return fmt.Errorf("need at least 2 runs to compare (found %d)", len(resp.Runs))
	}

	// Get the two most recent runs
	runA := resp.Runs[0]
	runB := resp.Runs[1]

	// Call DiffStageRuns
	diffResp, err := client.DiffStageRuns(ctx, &pipelinev1.DiffStageRunsRequest{
		RunIdA: runA.Id,
		RunIdB: runB.Id,
	})
	if err != nil {
		return fmt.Errorf("comparing runs: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(diffResp)
	}

	return outputPipelineInspectDiffHuman(diffResp, runA, runB)
}

func outputPipelineInspectOverviewHuman(runs []*pipelinev1.PipelineRunDetail, sourceID int64) error {
	if len(runs) == 0 {
		fmt.Printf("No pipeline runs found for source %d.\n", sourceID)
		return nil
	}

	fmt.Printf("Source %d - Pipeline Runs\n", sourceID)
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println("STAGE             STATUS      DURATION  MODEL           VERSION  HAS IO")
	fmt.Println(strings.Repeat("-", 80))

	// Extract triage information to check for content gating
	var triageContribution, triageReason string
	for _, run := range runs {
		if run.Stage == "triage" && run.Io != nil && run.Io.ParsedData != "" {
			var triageData map[string]interface{}
			if err := json.Unmarshal([]byte(run.Io.ParsedData), &triageData); err == nil {
				if contrib, ok := triageData["content_contribution"].(string); ok {
					triageContribution = contrib
				}
				if reason, ok := triageData["contribution_reason"].(string); ok {
					triageReason = reason
				}
			}
			break
		}
	}

	for _, run := range runs {
		model := run.ModelId
		if model == "" {
			model = "-"
		} else if len(model) > 15 {
			model = model[:12] + "..."
		}

		version := "-"
		if run.PromptVersion > 0 {
			version = fmt.Sprintf("v%d", run.PromptVersion)
		}

		statusColor := "\033[32m" // Green
		if run.Status == "failed" {
			statusColor = "\033[31m" // Red
		} else if run.Status == "superseded" {
			statusColor = "\033[33m" // Yellow
		}

		duration := formatDurationMs(int(run.DurationMs))

		hasIO := "no"
		if run.Io != nil && (run.Io.InputData != "" || run.Io.OutputData != "" || run.Io.ParsedData != "") {
			hasIO = "yes"
		}

		fmt.Printf("%-17s %s%-11s\033[0m %-9s %-15s %-8s %s\n",
			run.Stage, statusColor, run.Status, duration, model, version, hasIO)
	}

	// Show skipped stages if content contribution gating occurred
	if triageContribution == "NONE" || triageContribution == "LOW" {
		fmt.Println()
		fmt.Println("\033[36mSkipped Stages (Content Gating)\033[0m") // Cyan color
		fmt.Println(strings.Repeat("-", 80))

		// Determine which stages were skipped based on what ran
		stagesRan := make(map[string]bool)
		for _, run := range runs {
			stagesRan[run.Stage] = true
		}

		// List of stages that would normally run after triage
		expectedStages := []string{"extract_ner", "extract_assertions", "analyze", "embeddings"}

		for _, stage := range expectedStages {
			if !stagesRan[stage] {
				fmt.Printf("\033[36mStage: %-13s — SKIPPED (content_contribution: %s, reason: %s)\033[0m\n",
					stage, triageContribution, triageReason)
			}
		}
	}

	return nil
}

func outputPipelineInspectIOHuman(runs []*pipelinev1.PipelineRunDetail, sourceID int64, showInput bool, showOutput bool, showParsed bool, noTruncate bool) error {
	if len(runs) == 0 {
		fmt.Printf("No pipeline runs with IO data found for source %d.\n", sourceID)
		return nil
	}

	// Extract triage information to show skip context
	var triageContribution, triageReason string
	for _, run := range runs {
		if run.Stage == "triage" && run.Io != nil && run.Io.ParsedData != "" {
			var triageData map[string]interface{}
			if err := json.Unmarshal([]byte(run.Io.ParsedData), &triageData); err == nil {
				if contrib, ok := triageData["content_contribution"].(string); ok {
					triageContribution = contrib
				}
				if reason, ok := triageData["contribution_reason"].(string); ok {
					triageReason = reason
				}
			}
			break
		}
	}

	for i, run := range runs {
		if i > 0 {
			fmt.Println()
			fmt.Println(strings.Repeat("=", 80))
			fmt.Println()
		}

		model := run.ModelId
		if model == "" {
			model = "-"
		}

		version := ""
		if run.PromptVersion > 0 {
			version = fmt.Sprintf(" v%d", run.PromptVersion)
		}

		duration := formatDurationMs(int(run.DurationMs))

		fmt.Printf("Stage: %s (run #%d)\n", run.Stage, run.Id)
		fmt.Printf("Status: %s | Duration: %s | Model: %s%s\n", run.Status, duration, model, version)

		// Show content contribution info for triage stage
		if run.Stage == "triage" && triageContribution != "" {
			fmt.Printf("Content Contribution: %s (%s)\n", triageContribution, triageReason)
		}

		fmt.Println()

		if run.Io == nil {
			fmt.Println("(No IO data available)")
			continue
		}

		if showInput && run.Io.InputData != "" {
			fmt.Println(strings.Repeat("-", 40))
			fmt.Println("Input")
			fmt.Println(strings.Repeat("-", 40))
			fmt.Println(truncateInspectData(run.Io.InputData, noTruncate))
			fmt.Println()
		}

		if showOutput && run.Io.OutputData != "" {
			fmt.Println(strings.Repeat("-", 40))
			fmt.Println("Output")
			fmt.Println(strings.Repeat("-", 40))
			fmt.Println(truncateInspectData(run.Io.OutputData, noTruncate))
			fmt.Println()
		}

		if showParsed && run.Io.ParsedData != "" {
			fmt.Println(strings.Repeat("-", 40))
			fmt.Println("Parsed")
			fmt.Println(strings.Repeat("-", 40))
			fmt.Println(truncateInspectData(run.Io.ParsedData, noTruncate))
			fmt.Println()
		}
	}

	return nil
}

func outputPipelineInspectDiffHuman(diffResp *pipelinev1.DiffStageRunsResponse, runA *pipelinev1.PipelineRunDetail, runB *pipelinev1.PipelineRunDetail) error {
	fmt.Printf("Diff: Stage %s\n", diffResp.Stage)
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	fmt.Printf("Run A (ID: %d)\n", runA.Id)
	fmt.Printf("  Status:    %s\n", diffResp.RunAStatus)
	if diffResp.RunATime != nil {
		fmt.Printf("  Timestamp: %s\n", diffResp.RunATime.AsTime().Format("2006-01-02 15:04:05"))
	}
	fmt.Println()

	fmt.Printf("Run B (ID: %d)\n", runB.Id)
	fmt.Printf("  Status:    %s\n", diffResp.RunBStatus)
	if diffResp.RunBTime != nil {
		fmt.Printf("  Timestamp: %s\n", diffResp.RunBTime.AsTime().Format("2006-01-02 15:04:05"))
	}
	fmt.Println()

	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("Diff Summary")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println(diffResp.DiffSummary)
	fmt.Println()

	if diffResp.DiffJson != "" {
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println("Structured Diff (JSON)")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println(diffResp.DiffJson)
	}

	return nil
}

// truncateInspectData truncates data for display unless noTruncate is true.
func truncateInspectData(data string, noTruncate bool) string {
	const maxLen = 10000
	if noTruncate || len(data) <= maxLen {
		return data
	}
	return data[:maxLen] + fmt.Sprintf("\n\n... (truncated at %d chars, use --no-truncate for full data)", maxLen)
}
