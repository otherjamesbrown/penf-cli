package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	contentv1 "github.com/otherjamesbrown/penf-cli/api/proto/content/v1"
)

// newContentClearErrorCommand creates the 'content clear-error' subcommand.
func newContentClearErrorCommand(deps *ContentCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear-error <content-id>",
		Short: "Clear error fields from successfully reprocessed content",
		Long: `Clear error fields from a content item that was successfully reprocessed.

When content initially fails processing (rejected or failed state), Penfold sets
failure_category and failure_reason fields to explain what went wrong. If you
fix the issue and successfully reprocess the content, these error fields may
remain as stale data even though processing succeeded.

This command clears failure_category and failure_reason fields, removing the
stale error information. It's idempotent - running it multiple times on the
same content is safe and won't produce errors.

Use this after successful reprocessing to clean up error fields.

Examples:
  # Clear error fields after successful reprocessing
  penf content clear-error em-abc123

  # View content status before and after
  penf content show em-abc123
  penf content clear-error em-abc123
  penf content show em-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentClearError(cmd.Context(), deps, args[0])
		},
	}

	return cmd
}

// runContentClearError executes the content clear-error command.
func runContentClearError(ctx context.Context, deps *ContentCommandDeps, contentID string) error {
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
	req := &contentv1.ClearErrorRequest{
		ContentId: contentID,
	}

	// Clear error fields
	resp, err := client.ClearError(ctx, req)
	if err != nil {
		return fmt.Errorf("clearing error fields: %w", err)
	}

	// Output success message
	fmt.Printf("Successfully cleared error fields for content: %s\n", resp.ContentId)

	return nil
}
