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

func newPipelineAuditCmd(deps *PipelineCommandDeps) *cobra.Command {
	var (
		outputFormat string
		limit        int32
		pipeline     string
	)

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit pipeline completeness",
		Long: `Check completed content items for missing expected pipeline stages.

Compares pipeline_run records against pipeline definitions to find items
where stages should have run but didn't record a pipeline_run.

Examples:
  # Audit all completed items (default limit: 20)
  penf pipeline audit

  # Audit specific pipeline
  penf pipeline audit --pipeline standard

  # Show more results
  penf pipeline audit --limit 50

  # Output as JSON
  penf pipeline audit -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineAudit(cmd.Context(), deps, outputFormat, limit, pipeline)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")
	cmd.Flags().Int32Var(&limit, "limit", 20, "Maximum number of items to return")
	cmd.Flags().StringVar(&pipeline, "pipeline", "", "Filter by pipeline name")

	return cmd
}

func runPipelineAudit(ctx context.Context, deps *PipelineCommandDeps, outputFormat string, limit int32, pipeline string) error {
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

	resp, err := client.AuditPipelineCompleteness(ctx, &pipelinev1.AuditPipelineCompletenessRequest{
		TenantId: tenantID,
		Limit:    limit,
		Pipeline: pipeline,
	})
	if err != nil {
		return fmt.Errorf("auditing pipeline completeness: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	return outputAuditHuman(resp)
}

func outputAuditHuman(resp *pipelinev1.AuditPipelineCompletenessResponse) error {
	if len(resp.Items) == 0 {
		fmt.Println("All completed items have expected pipeline_run records.")
		return nil
	}

	fmt.Printf("Pipeline Audit — %d items with missing stages (of %d total):\n\n",
		len(resp.Items), resp.TotalCount)

	for _, item := range resp.Items {
		fmt.Printf("  %s (source %d) — pipeline: %s\n",
			item.ContentId, item.SourceId, item.Pipeline)

		if len(item.MissingStages) > 0 {
			fmt.Printf("    Missing:   \033[31m%s\033[0m\n", strings.Join(item.MissingStages, ", "))
		}
		if len(item.CompletedStages) > 0 {
			fmt.Printf("    Completed: \033[32m%s\033[0m\n", strings.Join(item.CompletedStages, ", "))
		}
		fmt.Println()
	}

	return nil
}
