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

func newPipelineRoutingCmd(deps *PipelineCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routing",
		Short: "Inspect and manage pipeline routing",
		Long: `Inspect and manage pipeline routing rules.

Routing rules map (content_type, content_subtype) to pipeline name(s).
Items with no matching route skip all pipeline stages.

Commands:
  list  - List all routing rules
  test  - Show which pipeline(s) a content item would enter

Examples:
  # List all routing rules
  penf pipeline routing list

  # Test routing for a content item
  penf pipeline routing test em-nGRm5RLf`,
	}

	cmd.AddCommand(newPipelineRoutingListCmd(deps))
	cmd.AddCommand(newPipelineRoutingTestCmd(deps))

	// Default action is list
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPipelineRoutingList(cmd.Context(), deps, "text")
	}

	return cmd
}

func newPipelineRoutingListCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all pipeline routing rules",
		Long: `List all pipeline routing rules showing content type/subtype to pipeline mapping.

Examples:
  # List all routing rules
  penf pipeline routing list

  # Output as JSON
  penf pipeline routing list -o json`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineRoutingList(cmd.Context(), deps, outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func newPipelineRoutingTestCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "test <content-id>",
		Short: "Test pipeline routing for a content item",
		Long: `Show which pipeline(s) a content item would enter based on its
content type and subtype. Items with no matching route skip all stages.

Examples:
  # Test routing for a content item
  penf pipeline routing test em-nGRm5RLf

  # Output as JSON
  penf pipeline routing test em-nGRm5RLf -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineRoutingTest(cmd.Context(), deps, args[0], outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineRoutingList(ctx context.Context, deps *PipelineCommandDeps, outputFormat string) error {
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

	resp, err := client.ListPipelineRoutes(ctx, &pipelinev1.ListPipelineRoutesRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return fmt.Errorf("listing pipeline routes: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Routes)
	}

	return outputRoutingListHuman(resp.Routes)
}

func runPipelineRoutingTest(ctx context.Context, deps *PipelineCommandDeps, contentID string, outputFormat string) error {
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

	resp, err := client.TestPipelineRoute(ctx, &pipelinev1.TestPipelineRouteRequest{
		TenantId:  tenantID,
		ContentId: contentID,
	})
	if err != nil {
		return fmt.Errorf("testing pipeline route: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	return outputRoutingTestHuman(resp)
}

func outputRoutingListHuman(routes []*pipelinev1.PipelineRoute) error {
	if len(routes) == 0 {
		fmt.Println("No pipeline routing rules found.")
		return nil
	}

	fmt.Printf("Pipeline Routing Rules (%d):\n\n", len(routes))
	fmt.Println("  ID  TYPE      SUBTYPE       PIPELINE        ACTIVE")
	fmt.Println("  --  ----      -------       --------        ------")

	for _, route := range routes {
		subtype := route.ContentSubtype
		if subtype == "" {
			subtype = "*"
		}
		activeStr := "\033[32mtrue\033[0m"
		if !route.Active {
			activeStr = "\033[90mfalse\033[0m"
		}

		fmt.Printf("  %-3d %-9s %-13s %-15s %s\n",
			route.Id,
			route.ContentType,
			subtype,
			route.Pipeline,
			activeStr)
	}

	fmt.Println()
	return nil
}

func outputRoutingTestHuman(resp *pipelinev1.TestPipelineRouteResponse) error {
	fmt.Println("Routing Result:")
	fmt.Printf("  Content Type:    %s\n", strings.ToUpper(resp.ContentType))
	fmt.Printf("  Content Subtype: %s\n", strings.ToUpper(resp.ContentSubtype))
	fmt.Println()

	if len(resp.MatchedRoutes) == 0 {
		fmt.Println("  No matching routes \u2014 all pipeline stages skipped")
	} else {
		fmt.Println("  Matched Routes:")
		for _, route := range resp.MatchedRoutes {
			stages := strings.Join(route.Stages, " \u2192 ")
			fmt.Printf("    %s (%s)\n", route.Pipeline, stages)
		}
	}

	fmt.Println()
	return nil
}
