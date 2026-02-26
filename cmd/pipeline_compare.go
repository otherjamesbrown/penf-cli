// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

func newPipelineCompareCmd(deps *PipelineCommandDeps) *cobra.Command {
	var (
		outputFormat string
		limit        int32
	)

	cmd := &cobra.Command{
		Use:   "compare --stage <stage>",
		Short: "Compare pipeline run statistics by model and prompt version",
		Long: `Compare pipeline run statistics grouped by model ID and prompt version.

Shows aggregate metrics for each model/prompt combination that has been
used for a stage, making it easy to evaluate model changes.

Examples:
  # Compare extract_assertions runs
  penf pipeline compare --stage extract_assertions

  # Compare triage runs
  penf pipeline compare --stage triage

  # Output as JSON
  penf pipeline compare --stage extract_assertions -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			stage, _ := cmd.Flags().GetString("stage")
			if stage == "" {
				return fmt.Errorf("--stage is required")
			}
			return runPipelineCompare(cmd.Context(), deps, stage, outputFormat, limit)
		},
	}

	cmd.Flags().String("stage", "", "Pipeline stage to compare (required)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")
	cmd.Flags().Int32Var(&limit, "limit", 20, "Maximum number of groups to return")

	return cmd
}

func runPipelineCompare(ctx context.Context, deps *PipelineCommandDeps, stage string, outputFormat string, limit int32) error {
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

	resp, err := client.ComparePipelineRuns(ctx, &pipelinev1.ComparePipelineRunsRequest{
		TenantId: tenantID,
		Stage:    stage,
		Limit:    limit,
	})
	if err != nil {
		return fmt.Errorf("comparing pipeline runs: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	return outputCompareHuman(resp)
}

func outputCompareHuman(resp *pipelinev1.ComparePipelineRunsResponse) error {
	if len(resp.Stats) == 0 {
		fmt.Printf("No pipeline runs found for stage '%s'.\n", resp.Stage)
		return nil
	}

	fmt.Printf("Pipeline Run Comparison — stage: %s\n\n", resp.Stage)
	fmt.Println("  MODEL                    PROMPT  RUNS   AVG MS    AVG IN   AVG OUT  OK    FAIL")
	fmt.Println("  -----                    ------  ----   ------    ------   -------  --    ----")

	for _, s := range resp.Stats {
		modelStr := s.ModelId
		if modelStr == "" {
			modelStr = "(none)"
		}

		promptStr := "-"
		if s.PromptVersion > 0 {
			promptStr = fmt.Sprintf("v%d", s.PromptVersion)
		}

		fmt.Printf("  %-24s %-5s   %-5d  %7.0f   %6.0f   %6.0f   %-5d %-5d\n",
			truncateDefineStr(modelStr, 24),
			promptStr,
			s.RunCount,
			s.AvgDurationMs,
			s.AvgInputTokens,
			s.AvgOutputTokens,
			s.SuccessCount,
			s.FailureCount)
	}

	fmt.Println()
	return nil
}

func truncateDefineStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
