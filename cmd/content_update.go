package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	ingestv1 "github.com/otherjamesbrown/penf-cli/api/proto/ingest/v1"
)

var (
	updateContentTitle    string
	updateContentTags     string
	updateContentCategory string
)

// newContentUpdateCommand creates the 'content update' command.
func newContentUpdateCommand(deps *ContentCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultContentDeps()
	}

	cmd := &cobra.Command{
		Use:   "update <content-id>",
		Short: "Update content metadata",
		Long: `Update metadata for a content item.

Update title, tags, or category for an existing content item. At least one
flag must be provided. Tags are comma-separated and replace existing tags.

Examples:
  # Update title only
  penf content update em-gFo2YZi3 --title "New Title"

  # Update tags (replaces existing)
  penf content update em-gFo2YZi3 --tags "work,project,urgent"

  # Update category
  penf content update em-gFo2YZi3 --category "Documentation"

  # Update multiple fields
  penf content update em-gFo2YZi3 --title "New Title" --tags "work" --category "Project"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentUpdate(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVar(&updateContentTitle, "title", "", "New title")
	cmd.Flags().StringVar(&updateContentTags, "tags", "", "Comma-separated tags (replaces existing)")
	cmd.Flags().StringVar(&updateContentCategory, "category", "", "Category name")

	return cmd
}

// runContentUpdate executes the update command.
func runContentUpdate(ctx context.Context, deps *ContentCommandDeps, contentID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Require at least one flag
	if updateContentTitle == "" && updateContentTags == "" && updateContentCategory == "" {
		return fmt.Errorf("at least one flag required: --title, --tags, or --category")
	}

	// Connect to gateway via gRPC
	conn, err := connectToGateway(cfg)
	if err != nil {
		return fmt.Errorf("connecting to gateway: %w", err)
	}
	defer conn.Close()

	// Get ingest service client
	ingestClient := ingestv1.NewIngestServiceClient(conn)

	// Build request
	req := &ingestv1.UpdateContentRequest{
		ContentId: contentID,
	}

	if updateContentTitle != "" {
		req.Title = &updateContentTitle
	}

	if updateContentTags != "" {
		// Parse comma-separated tags
		tags := strings.Split(updateContentTags, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
		req.Tags = tags
	}

	if updateContentCategory != "" {
		req.Category = &updateContentCategory
	}

	// Call UpdateContent
	resp, err := ingestClient.UpdateContent(ctx, req)
	if err != nil {
		return fmt.Errorf("updating content: %w", err)
	}

	// Output result
	fmt.Printf("\033[32mContent updated\033[0m\n\n")
	fmt.Printf("  Content ID:  %s\n", resp.ContentId)
	fmt.Printf("  Title:       %s\n", resp.Title)
	if len(resp.Tags) > 0 {
		fmt.Printf("  Tags:        %s\n", strings.Join(resp.Tags, ", "))
	}
	if resp.Category != "" {
		fmt.Printf("  Category:    %s\n", resp.Category)
	}
	fmt.Println()

	return nil
}
