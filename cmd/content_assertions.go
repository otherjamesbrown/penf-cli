package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	contentv1 "github.com/otherjamesbrown/penf-cli/api/proto/content/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

// newContentAssertionsCommand creates the 'content assertions' subcommand.
func newContentAssertionsCommand(deps *ContentCommandDeps) *cobra.Command {
	var assertionType string

	cmd := &cobra.Command{
		Use:   "assertions <content-id>",
		Short: "Show extracted assertions for content",
		Long: `Show extracted assertions for a content item.

Assertions are extracted claims, commitments, decisions, risks, and action items
identified from the content using AI analysis.

Examples:
  # Show all assertions for a content item
  penf content assertions em-abc123

  # Filter by assertion type
  penf content assertions em-abc123 --type risk
  penf content assertions em-abc123 --type action_item
  penf content assertions em-abc123 --type decision

  # Output as JSON
  penf content assertions em-abc123 -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentAssertions(cmd.Context(), deps, args[0], assertionType)
		},
	}

	cmd.Flags().StringVar(&assertionType, "type", "", "Filter by assertion type (e.g., risk, action_item, decision, commitment)")

	return cmd
}

// runContentAssertions executes the content assertions command.
func runContentAssertions(ctx context.Context, deps *ContentCommandDeps, contentID string, assertionType string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := contentv1.NewContentProcessorServiceClient(conn)

	// Build request
	req := &contentv1.GetAssertionsRequest{
		ContentId: contentID,
	}

	if assertionType != "" {
		req.AssertionType = &assertionType
	}

	// Get assertions
	resp, err := client.GetAssertions(ctx, req)
	if err != nil {
		return fmt.Errorf("getting assertions: %w", err)
	}

	// Output results
	format := cfg.OutputFormat
	if contentOutput != "" {
		format = config.OutputFormat(contentOutput)
	}

	return outputAssertions(format, resp)
}

// outputAssertions outputs assertions in the specified format.
func outputAssertions(format config.OutputFormat, resp *contentv1.GetAssertionsResponse) error {
	switch format {
	case config.OutputFormatJSON:
		return outputContentJSON(resp)
	case config.OutputFormatYAML:
		return outputContentYAML(resp)
	default:
		return outputAssertionsText(resp)
	}
}

// outputAssertionsText outputs assertions in text format.
func outputAssertionsText(resp *contentv1.GetAssertionsResponse) error {
	if len(resp.Assertions) == 0 {
		fmt.Printf("No assertions found for content: %s\n", resp.ContentId)
		return nil
	}

	fmt.Printf("Assertions for content: %s\n", resp.ContentId)
	fmt.Printf("Total: %d\n\n", resp.TotalCount)

	fmt.Println("ID      TYPE              CONFIDENCE  MODEL                 DESCRIPTION")
	fmt.Println("--      ----              ----------  -----                 -----------")

	for _, assertion := range resp.Assertions {
		// Format confidence as percentage
		confidence := fmt.Sprintf("%.2f", assertion.Confidence)

		// Get model (or show "-" if not set)
		model := "-"
		if assertion.ExtractionModel != nil && *assertion.ExtractionModel != "" {
			model = truncate(*assertion.ExtractionModel, 20)
		}

		// Format description (truncate if too long)
		description := truncate(assertion.Description, 60)

		fmt.Printf("%-7d %-17s %-11s %-21s %s\n",
			assertion.Id,
			truncate(assertion.AssertionType, 17),
			confidence,
			model,
			description)
	}

	fmt.Println()
	return nil
}
