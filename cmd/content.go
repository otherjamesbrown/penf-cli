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
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	contentv1 "github.com/otherjamesbrown/penf-cli/api/proto/content/v1"
	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Content command flags
var (
	contentOutput     string
	contentStatus     string
	contentSource     string
	contentTenant     string
	contentLimit      int32
	contentProcessing bool
	contentConfirm    bool
	contentBefore     string
	contentReason     string
	contentFull       bool
)

// ContentCommandDeps holds the dependencies for content commands.
type ContentCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultContentDeps returns the default dependencies for production use.
func DefaultContentDeps() *ContentCommandDeps {
	return &ContentCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewContentCommand creates the root content command with all subcommands.
func NewContentCommand(deps *ContentCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultContentDeps()
	}

	cmd := &cobra.Command{
		Use:   "content",
		Short: "Manage content items in the processing pipeline",
		Long: `Manage content items in the processing pipeline.

Content items represent units of content from various sources (email, documents,
meetings, Slack) that have been ingested and processed through the AI pipeline.

Each content item goes through multiple processing stages:
  - FETCH: Fetching raw content from the source system
  - PARSE: Parsing and normalizing content structure
  - EMBED: Generating vector embeddings for semantic search
  - SUMMARIZE: Generating AI summaries
  - EXTRACT: Extracting entities, assertions, and structured data

Processing states:
  - PENDING: Queued for processing
  - IN_PROGRESS: Currently being processed
  - COMPLETED: Processing completed successfully
  - FAILED: Processing failed with an error
  - CANCELLED: Processing was cancelled

JSON Output (for AI processing):
  penf content list -o json

  Returns:
  {
    "items": [
      {
        "id": "content-123",
        "source_type": "email",
        "state": "COMPLETED",
        "metadata": {"subject": "Project update", "from": "user@example.com"}
      }
    ],
    "total_count": 42
  }

Related Commands:
  penf reprocess   Reprocess content items with updated models or settings
  penf pipeline    View overall pipeline status and processing statistics`,
		Aliases: []string{"contents"},
	}

	// Add persistent flags
	cmd.PersistentFlags().StringVarP(&contentOutput, "output", "o", "", "Output format: text, json, yaml")
	cmd.PersistentFlags().Int32VarP(&contentLimit, "limit", "l", 50, "Maximum number of results")

	// Add subcommands
	cmd.AddCommand(newContentListCommand(deps))
	cmd.AddCommand(newContentShowCommand(deps))
	cmd.AddCommand(newContentUpdateCommand(deps))
	cmd.AddCommand(newContentDeleteCommand(deps))
	cmd.AddCommand(newContentPurgeCommand(deps))
	cmd.AddCommand(newContentStatsCommand(deps))
	cmd.AddCommand(newContentTraceCommand(deps))
	cmd.AddCommand(newContentTextCommand(deps))
	cmd.AddCommand(newContentInsightsCommand(deps))
	cmd.AddCommand(newContentAssertionsCommand(deps))
	cmd.AddCommand(newContentClearErrorCommand(deps))

	return cmd
}

// newContentListCommand creates the 'content list' subcommand.
func newContentListCommand(deps *ContentCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List content items",
		Long: `List content items with optional filtering.

Filter by processing state, source type, or tenant. Results are paginated.

Examples:
  # List all content items
  penf content list

  # Filter by status
  penf content list --status pending
  penf content list --status complete

  # Filter by source type
  penf content list --source email
  penf content list --source document

  # Combine filters
  penf content list --status complete --source email --limit 100

  # Output as JSON
  penf content list -o json`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentList(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&contentStatus, "status", "", "Filter by processing state: pending, processing, complete, failed, cancelled, rejected, skipped")
	cmd.Flags().StringVar(&contentSource, "source", "", "Filter by source type: email, document, meeting, slack")
	cmd.Flags().StringVar(&contentTenant, "tenant", "", "Filter by tenant ID (defaults to config tenant)")

	return cmd
}

// newContentShowCommand creates the 'content show' subcommand.
func newContentShowCommand(deps *ContentCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <content-id>",
		Short: "Show details of a content item",
		Long: `Show detailed information about a specific content item.

Displays content metadata, processing state, timestamps, and optionally
detailed processing status with stage completion information.

Examples:
  # Show content item details
  penf content show content-123

  # Show with detailed processing status
  penf content show content-123 --processing

  # Output as JSON
  penf content show content-123 -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentShow(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().BoolVar(&contentProcessing, "processing", false, "Show detailed processing status")
	cmd.Flags().BoolVar(&contentFull, "full", false, "Show full body text without truncation")

	return cmd
}

// Command execution functions

func runContentList(ctx context.Context, deps *ContentCommandDeps) error {
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
	req := &contentv1.ListContentItemsRequest{
		PageSize: contentLimit,
	}

	// Apply filters
	if contentSource != "" {
		req.SourceType = &contentSource
	}

	if contentStatus != "" {
		state, err := parseProcessingState(contentStatus)
		if err != nil {
			return err
		}
		req.State = &state
	}

	// Get tenant ID
	tenantID := contentTenant
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via --tenant flag or 'penf config set tenant_id <id>'")
	}
	req.TenantId = tenantID

	// Make request
	resp, err := client.ListContentItems(ctx, req)
	if err != nil {
		return fmt.Errorf("listing content items: %w", err)
	}

	// Output results
	format := cfg.OutputFormat
	if contentOutput != "" {
		format = config.OutputFormat(contentOutput)
	}

	return outputContentList(format, resp)
}

func runContentShow(ctx context.Context, deps *ContentCommandDeps, contentID string) error {
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

	// Get content item
	getReq := &contentv1.GetContentItemRequest{
		ContentId:        contentID,
		IncludeEmbedding: false,
	}
	item, err := client.GetContentItem(ctx, getReq)
	if err != nil {
		return fmt.Errorf("getting content item: %w", err)
	}

	// Get processing status if requested
	var status *contentv1.ProcessingStatus
	if contentProcessing {
		statusReq := &contentv1.GetProcessingStatusRequest{
			ContentId: contentID,
		}
		status, err = client.GetProcessingStatus(ctx, statusReq)
		if err != nil {
			// Don't fail if processing status unavailable
			fmt.Fprintf(os.Stderr, "Warning: could not get processing status: %v\n", err)
		}
	}

	// Output results
	format := cfg.OutputFormat
	if contentOutput != "" {
		format = config.OutputFormat(contentOutput)
	}

	return outputContentItem(format, item, status)
}

// Output functions

func outputContentList(format config.OutputFormat, resp *contentv1.ListContentItemsResponse) error {
	switch format {
	case config.OutputFormatJSON:
		return outputContentJSON(resp)
	case config.OutputFormatYAML:
		return outputContentYAML(resp)
	default:
		return outputContentListText(resp)
	}
}

func outputContentListText(resp *contentv1.ListContentItemsResponse) error {
	if len(resp.Items) == 0 {
		fmt.Println("No content items found.")
		return nil
	}

	totalCount := int64(len(resp.Items))
	if resp.TotalCount != nil {
		totalCount = *resp.TotalCount
	}

	fmt.Printf("Content Items (%d total):\n\n", totalCount)
	fmt.Println("  ID                    TYPE        SUBJECT/TITLE                      SOURCE     STATE       CREATED")
	fmt.Println("  --                    ----        -------------                      ------     -----       -------")

	for _, item := range resp.Items {
		// Get subject/title from metadata
		subject := getSubjectFromMetadata(item.Metadata)
		if subject == "" {
			subject = "-"
		}

		// Format timestamps
		createdStr := "-"
		if item.CreatedAt != nil {
			createdStr = item.CreatedAt.AsTime().Format("2006-01-02")
		}

		// Format state
		stateStr := formatProcessingState(item.State)

		fmt.Printf("  %-21s %-11s %-34s %-10s %-11s %s\n",
			truncate(item.Id, 21),
			truncate(item.SourceType, 11),
			truncate(subject, 34),
			truncate(item.SourceId, 10),
			stateStr,
			createdStr)
	}

	fmt.Println()

	if resp.NextPageToken != "" {
		fmt.Printf("More results available. Use --page-token=%s to fetch next page.\n", resp.NextPageToken)
	}

	return nil
}

func outputContentItem(format config.OutputFormat, item *contentv1.ContentItem, status *contentv1.ProcessingStatus) error {
	switch format {
	case config.OutputFormatJSON:
		data := map[string]interface{}{
			"item": item,
		}
		if status != nil {
			data["processing_status"] = status
		}
		return outputContentJSON(data)
	case config.OutputFormatYAML:
		data := map[string]interface{}{
			"item": item,
		}
		if status != nil {
			data["processing_status"] = status
		}
		return outputContentYAML(data)
	default:
		return outputContentItemText(item, status, contentFull)
	}
}

func outputContentItemText(item *contentv1.ContentItem, status *contentv1.ProcessingStatus, fullBody bool) error {
	fmt.Println("Content Item Details:")
	fmt.Println()
	fmt.Printf("  \033[1mID:\033[0m           %s\n", item.Id)
	fmt.Printf("  \033[1mSource Type:\033[0m  %s\n", item.SourceType)
	fmt.Printf("  \033[1mSource ID:\033[0m    %s\n", item.SourceId)
	fmt.Printf("  \033[1mTenant ID:\033[0m    %s\n", item.TenantId)
	fmt.Printf("  \033[1mState:\033[0m        %s\n", formatProcessingState(item.State))

	// Show Langfuse trace ID if available
	if item.LangfuseTraceId != nil && *item.LangfuseTraceId != "" {
		fmt.Printf("  \033[1mTrace ID:\033[0m     %s\n", *item.LangfuseTraceId)
		fmt.Printf("  \033[1mLangfuse:\033[0m     http://dev02.brown.chat:3000/project/penfold-ai/traces/%s\n", *item.LangfuseTraceId)
	}

	// Show failure info for rejected/failed items
	if item.FailureCategory != nil && *item.FailureCategory != "" {
		fmt.Printf("  \033[1m\033[31mError Code:\033[0m   %s\n", *item.FailureCategory)
	}
	if item.FailureReason != nil && *item.FailureReason != "" {
		fmt.Printf("  \033[1mMessage:\033[0m      %s\n", *item.FailureReason)
	}
	// Show suggested action for known error codes
	if item.FailureCategory != nil && *item.FailureCategory != "" {
		// Import the errors package to get suggested actions
		// For now, we'll add a generic hint
		fmt.Printf("  \033[1mHint:\033[0m         Use 'penf pipeline errors --code %s' to see similar errors\n", *item.FailureCategory)
	}
	fmt.Println()

	// For email content, show email-specific fields first
	if item.SourceType == "email" && len(item.Metadata) > 0 {
		fmt.Println("  \033[1mEmail:\033[0m")
		if subject, ok := item.Metadata["subject"]; ok {
			fmt.Printf("    Subject:     %s\n", subject)
		}
		if from, ok := item.Metadata["from"]; ok {
			fmt.Printf("    From:        %s\n", from)
		}
		if to, ok := item.Metadata["to"]; ok {
			fmt.Printf("    To:          %s\n", to)
		}
		if cc, ok := item.Metadata["cc"]; ok {
			fmt.Printf("    CC:          %s\n", cc)
		}
		if bcc, ok := item.Metadata["bcc"]; ok {
			fmt.Printf("    BCC:         %s\n", bcc)
		}
		fmt.Println()
	}

	// Metadata (show remaining fields not already displayed)
	if len(item.Metadata) > 0 {
		// For email, skip the fields we already showed
		emailFields := map[string]bool{
			"subject": true,
			"from":    true,
			"to":      true,
			"cc":      true,
			"bcc":     true,
		}

		hasOtherMetadata := false
		if item.SourceType == "email" {
			for key := range item.Metadata {
				if !emailFields[key] {
					hasOtherMetadata = true
					break
				}
			}
		} else {
			hasOtherMetadata = len(item.Metadata) > 0
		}

		if hasOtherMetadata {
			fmt.Println("  \033[1mMetadata:\033[0m")
			for key, value := range item.Metadata {
				if item.SourceType == "email" && emailFields[key] {
					continue
				}
				fmt.Printf("    %-12s %s\n", key+":", truncate(value, 60))
			}
			fmt.Println()
		}
	}

	// Content hash
	fmt.Printf("  \033[1mContent Hash:\033[0m %s\n", item.ContentHash)
	fmt.Println()

	// Body text from raw_content
	if item.RawContent != "" {
		fmt.Printf("  \033[1mBody:\033[0m\n")
		bodyText := item.RawContent
		const maxBodyLength = 1000

		if !fullBody && len(bodyText) > maxBodyLength {
			bodyText = truncateContentBody(bodyText, maxBodyLength)
			fmt.Printf("    %s\n", bodyText)
			fmt.Printf("    \033[90m[truncated at %d chars, use --full to see complete body]\033[0m\n", maxBodyLength)
		} else {
			fmt.Printf("    %s\n", bodyText)
		}
		fmt.Println()
	}

	// Summary if available
	if item.Summary != nil && *item.Summary != "" {
		fmt.Printf("  \033[1mSummary:\033[0m\n")
		fmt.Printf("    %s\n", *item.Summary)
		fmt.Println()
	}

	// Timestamps
	if item.CreatedAt != nil {
		fmt.Printf("  \033[1mCreated:\033[0m      %s\n", item.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}
	if item.UpdatedAt != nil {
		fmt.Printf("  \033[1mUpdated:\033[0m      %s\n", item.UpdatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}
	if item.ProcessedAt != nil {
		fmt.Printf("  \033[1mProcessed:\033[0m    %s\n", item.ProcessedAt.AsTime().Format("2006-01-02 15:04:05"))
	}

	// Processing status if available
	if status != nil {
		fmt.Println()
		fmt.Println("  \033[1mProcessing Status:\033[0m")
		fmt.Printf("    Job ID:       %s\n", status.JobId)
		fmt.Printf("    State:        %s\n", formatProcessingState(status.State))
		fmt.Printf("    Progress:     %d%%\n", status.ProgressPercent)

		if status.CurrentStage != nil {
			fmt.Printf("    Current Stage: %s\n", formatProcessingStage(*status.CurrentStage))
		}

		// Show completed stages with checkmarks
		if len(status.StagesCompleted) > 0 {
			fmt.Println()
			fmt.Println("    \033[1mStages:\033[0m")
			allStages := []contentv1.ProcessingStage{
				contentv1.ProcessingStage_PROCESSING_STAGE_FETCH,
				contentv1.ProcessingStage_PROCESSING_STAGE_PARSE,
				contentv1.ProcessingStage_PROCESSING_STAGE_EMBED,
				contentv1.ProcessingStage_PROCESSING_STAGE_SUMMARIZE,
				contentv1.ProcessingStage_PROCESSING_STAGE_EXTRACT,
			}

			for _, stage := range allStages {
				completed := stageCompleted(stage, status.StagesCompleted)
				checkmark := " "
				if completed {
					checkmark = "\033[32m✓\033[0m"
				}
				fmt.Printf("      %s %s\n", checkmark, formatProcessingStage(stage))
			}
		}

		// Show errors if any
		if len(status.Errors) > 0 {
			fmt.Println()
			fmt.Println("    \033[1mErrors:\033[0m")
			for _, err := range status.Errors {
				fmt.Printf("      \033[31m[%s]\033[0m %s: %s\n",
					formatProcessingStage(err.Stage),
					err.Code,
					err.Message)
				if err.RetryCount > 0 {
					fmt.Printf("        (retries: %d)\n", err.RetryCount)
				}
			}
		}

		// Duration
		if status.DurationMs != nil {
			duration := time.Duration(*status.DurationMs) * time.Millisecond
			fmt.Printf("\n    Duration:     %s\n", duration)
		}
	}

	fmt.Println()
	return nil
}

func outputContentJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputContentYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

// Helper functions

func parseProcessingState(state string) (contentv1.ProcessingState, error) {
	state = strings.ToLower(state)
	switch state {
	case "pending":
		return contentv1.ProcessingState_PROCESSING_STATE_PENDING, nil
	case "processing", "in_progress":
		return contentv1.ProcessingState_PROCESSING_STATE_IN_PROGRESS, nil
	case "complete", "completed":
		return contentv1.ProcessingState_PROCESSING_STATE_COMPLETED, nil
	case "failed":
		return contentv1.ProcessingState_PROCESSING_STATE_FAILED, nil
	case "cancelled":
		return contentv1.ProcessingState_PROCESSING_STATE_CANCELLED, nil
	case "rejected":
		return contentv1.ProcessingState_PROCESSING_STATE_REJECTED, nil
	case "skipped":
		return contentv1.ProcessingState_PROCESSING_STATE_SKIPPED, nil
	default:
		return contentv1.ProcessingState_PROCESSING_STATE_UNSPECIFIED,
			fmt.Errorf("invalid processing state: %s (must be: pending, processing, complete, failed, cancelled, rejected, skipped)", state)
	}
}

func formatProcessingState(state contentv1.ProcessingState) string {
	switch state {
	case contentv1.ProcessingState_PROCESSING_STATE_PENDING:
		return "\033[33mPENDING\033[0m"
	case contentv1.ProcessingState_PROCESSING_STATE_IN_PROGRESS:
		return "\033[36mIN_PROGRESS\033[0m"
	case contentv1.ProcessingState_PROCESSING_STATE_COMPLETED:
		return "\033[32mCOMPLETED\033[0m"
	case contentv1.ProcessingState_PROCESSING_STATE_FAILED:
		return "\033[31mFAILED\033[0m"
	case contentv1.ProcessingState_PROCESSING_STATE_CANCELLED:
		return "\033[90mCANCELLED\033[0m"
	case contentv1.ProcessingState_PROCESSING_STATE_REJECTED:
		return "\033[33mREJECTED\033[0m"
	case contentv1.ProcessingState_PROCESSING_STATE_SKIPPED:
		return "\033[90mSKIPPED\033[0m"
	default:
		return "UNSPECIFIED"
	}
}

func formatProcessingStage(stage contentv1.ProcessingStage) string {
	switch stage {
	case contentv1.ProcessingStage_PROCESSING_STAGE_FETCH:
		return "FETCH"
	case contentv1.ProcessingStage_PROCESSING_STAGE_PARSE:
		return "PARSE"
	case contentv1.ProcessingStage_PROCESSING_STAGE_EMBED:
		return "EMBED"
	case contentv1.ProcessingStage_PROCESSING_STAGE_SUMMARIZE:
		return "SUMMARIZE"
	case contentv1.ProcessingStage_PROCESSING_STAGE_EXTRACT:
		return "EXTRACT"
	case contentv1.ProcessingStage_PROCESSING_STAGE_COMPLETE:
		return "COMPLETE"
	default:
		return "UNSPECIFIED"
	}
}

func getSubjectFromMetadata(metadata map[string]string) string {
	// Try common metadata keys for subject/title
	keys := []string{"subject", "title", "name", "summary"}
	for _, key := range keys {
		if val, ok := metadata[key]; ok && val != "" {
			return val
		}
	}
	return ""
}

func stageCompleted(stage contentv1.ProcessingStage, completed []contentv1.ProcessingStage) bool {
	for _, s := range completed {
		if s == stage {
			return true
		}
	}
	return false
}

func truncateContentBody(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// newContentDeleteCommand creates the 'content delete' subcommand.
func newContentDeleteCommand(deps *ContentCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [content-id]",
		Short: "Delete content items",
		Long: `Delete content items individually or in bulk.

Delete a single content item by ID, or bulk delete items matching filters.

Examples:
  # Delete a single content item
  penf content delete content-123

  # Bulk delete by filters (requires --confirm)
  penf content delete --source email --status failed --confirm

  # Delete items created before a date
  penf content delete --before 2026-01-01 --confirm

  # Combine filters
  penf content delete --source document --status pending --confirm`,
		Aliases: []string{"rm", "remove"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				// Single content item delete
				return runContentDeleteSingle(cmd.Context(), deps, args[0])
			} else if len(args) == 0 {
				// Bulk delete
				return runContentDeleteBulk(cmd.Context(), deps)
			}
			return fmt.Errorf("specify a single content ID or use filters for bulk delete")
		},
	}

	cmd.Flags().StringVar(&contentStatus, "status", "", "Filter by processing state for bulk delete: pending, processing, complete, failed")
	cmd.Flags().StringVar(&contentSource, "source", "", "Filter by source type for bulk delete: email, document, meeting, slack")
	cmd.Flags().StringVar(&contentTenant, "tenant", "", "Filter by tenant ID (defaults to config tenant)")
	cmd.Flags().StringVar(&contentBefore, "before", "", "Filter by items created before this date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&contentConfirm, "confirm", false, "Confirm bulk delete operation (required for bulk delete)")

	return cmd
}

// newContentStatsCommand creates the 'content stats' subcommand.
func newContentStatsCommand(deps *ContentCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show content statistics",
		Long: `Show statistics about content items.

Displays total counts, breakdowns by source type and processing state,
storage usage, and processing completion metrics.

Examples:
  # Show content statistics
  penf content stats

  # Output as JSON
  penf content stats -o json`,
		Aliases: []string{"statistics"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentStats(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&contentTenant, "tenant", "", "Tenant ID (defaults to config tenant)")

	return cmd
}

func runContentDeleteSingle(ctx context.Context, deps *ContentCommandDeps, contentID string) error {
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

	// Delete the content item
	resp, err := client.DeleteContentItem(ctx, &contentv1.DeleteContentItemRequest{
		ContentId: contentID,
	})
	if err != nil {
		return fmt.Errorf("deleting content item: %w", err)
	}

	if resp.Success {
		fmt.Printf("Successfully deleted content item: %s\n", resp.ContentId)
	} else {
		fmt.Printf("Failed to delete content item: %s\n", resp.ContentId)
	}

	return nil
}

func runContentDeleteBulk(ctx context.Context, deps *ContentCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Require confirmation for bulk delete
	if !contentConfirm {
		return fmt.Errorf("bulk delete requires --confirm flag")
	}

	// Get tenant ID
	tenantID := contentTenant
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via --tenant flag or 'penf config set tenant_id <id>'")
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := contentv1.NewContentProcessorServiceClient(conn)

	// Build request
	req := &contentv1.DeleteContentItemsRequest{
		TenantId: tenantID,
		Confirm:  true,
	}

	// Apply filters
	if contentSource != "" {
		req.SourceType = &contentSource
	}

	if contentStatus != "" {
		state, err := parseProcessingState(contentStatus)
		if err != nil {
			return err
		}
		req.State = &state
	}

	if contentBefore != "" {
		beforeTime, err := time.Parse("2006-01-02", contentBefore)
		if err != nil {
			return fmt.Errorf("invalid before date format (use YYYY-MM-DD): %w", err)
		}
		before := beforeTime
		req.Before = timestampProto(before)
	}

	// Execute bulk delete
	resp, err := client.DeleteContentItems(ctx, req)
	if err != nil {
		return fmt.Errorf("bulk deleting content items: %w", err)
	}

	fmt.Printf("Successfully deleted %d content items.\n", resp.DeletedCount)

	if len(resp.DeletedIds) > 0 && len(resp.DeletedIds) <= 20 {
		fmt.Println("\nDeleted IDs:")
		for _, id := range resp.DeletedIds {
			fmt.Printf("  - %s\n", id)
		}
	}

	return nil
}

// newContentPurgeCommand creates the 'content purge' subcommand.
func newContentPurgeCommand(deps *ContentCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purge [content-id]",
		Short: "Permanently delete (purge) content items",
		Long: `Permanently delete (purge) soft-deleted content items.

Purge performs a hard delete of content items that have already been soft-deleted.
This operation is irreversible and removes all associated data.

Examples:
  # Purge a single content item
  penf content purge content-123 --reason "re-ingest needed" --confirm

  # Bulk purge by filters (requires --confirm)
  penf content purge --source email --reason "cleanup" --confirm

  # Limit bulk purge to 50 items
  penf content purge --source email --reason "cleanup" --confirm --limit 50`,
		Aliases: []string{"hard-delete"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				// Single content item purge
				return runContentPurgeSingle(cmd.Context(), deps, args[0])
			} else if len(args) == 0 {
				// Bulk purge
				return runContentPurgeBulk(cmd.Context(), deps)
			}
			return fmt.Errorf("specify a single content ID or use filters for bulk purge")
		},
	}

	cmd.Flags().StringVar(&contentSource, "source", "", "Filter by source type for bulk purge: email, document, meeting, slack")
	cmd.Flags().StringVar(&contentTenant, "tenant", "", "Filter by tenant ID (defaults to config tenant)")
	cmd.Flags().StringVar(&contentReason, "reason", "", "Reason for purging (required)")
	cmd.Flags().BoolVar(&contentConfirm, "confirm", false, "Confirm purge operation (required)")
	cmd.Flags().Int32VarP(&contentLimit, "limit", "l", 100, "Maximum number of items to purge in bulk mode")

	return cmd
}

func runContentPurgeSingle(ctx context.Context, deps *ContentCommandDeps, contentID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Validate required flags
	if contentReason == "" {
		return fmt.Errorf("--reason flag is required")
	}
	if !contentConfirm {
		return fmt.Errorf("--confirm flag is required")
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := contentv1.NewContentProcessorServiceClient(conn)

	// Purge the content item
	resp, err := client.PurgeContentItem(ctx, &contentv1.PurgeContentItemRequest{
		ContentId: contentID,
		Reason:    contentReason,
		Confirm:   true,
	})
	if err != nil {
		return fmt.Errorf("purging content item: %w", err)
	}

	if resp.Success {
		fmt.Printf("Successfully purged content item: %s\n", resp.ContentId)
		if resp.Message != "" {
			fmt.Printf("Message: %s\n", resp.Message)
		}
	} else {
		fmt.Printf("Failed to purge content item: %s\n", resp.ContentId)
		if resp.Message != "" {
			fmt.Printf("Message: %s\n", resp.Message)
		}
	}

	return nil
}

func runContentPurgeBulk(ctx context.Context, deps *ContentCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Validate required flags
	if contentReason == "" {
		return fmt.Errorf("--reason flag is required")
	}
	if !contentConfirm {
		return fmt.Errorf("--confirm flag is required")
	}

	// Get tenant ID
	tenantID := contentTenant
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via --tenant flag or 'penf config set tenant_id <id>'")
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := contentv1.NewContentProcessorServiceClient(conn)

	// Build request
	req := &contentv1.PurgeContentItemsRequest{
		TenantId: tenantID,
		Reason:   contentReason,
		Confirm:  true,
		Limit:    contentLimit,
	}

	// Apply filters
	if contentSource != "" {
		req.SourceType = &contentSource
	}

	// Execute bulk purge
	resp, err := client.PurgeContentItems(ctx, req)
	if err != nil {
		return fmt.Errorf("bulk purging content items: %w", err)
	}

	fmt.Printf("Successfully purged %d content items.\n", resp.PurgedCount)
	if resp.Message != "" {
		fmt.Printf("Message: %s\n", resp.Message)
	}

	if len(resp.ContentIds) > 0 && len(resp.ContentIds) <= 20 {
		fmt.Println("\nPurged IDs:")
		for _, id := range resp.ContentIds {
			fmt.Printf("  - %s\n", id)
		}
	} else if len(resp.ContentIds) > 20 {
		fmt.Printf("\n%d content IDs purged (too many to display)\n", len(resp.ContentIds))
	}

	return nil
}

func runContentStats(ctx context.Context, deps *ContentCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Get tenant ID
	tenantID := contentTenant
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via --tenant flag or 'penf config set tenant_id <id>'")
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := contentv1.NewContentProcessorServiceClient(conn)

	// Get stats
	stats, err := client.GetContentStats(ctx, &contentv1.GetContentStatsRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return fmt.Errorf("getting content stats: %w", err)
	}

	// Output results
	format := cfg.OutputFormat
	if contentOutput != "" {
		format = config.OutputFormat(contentOutput)
	}

	return outputContentStats(format, stats)
}

func outputContentStats(format config.OutputFormat, stats *contentv1.ContentStats) error {
	switch format {
	case config.OutputFormatJSON:
		return outputContentJSON(stats)
	case config.OutputFormatYAML:
		return outputContentYAML(stats)
	default:
		return outputContentStatsText(stats)
	}
}

func outputContentStatsText(stats *contentv1.ContentStats) error {
	fmt.Println("Content Statistics:")
	fmt.Println()
	fmt.Printf("  \033[1mTenant ID:\033[0m     %s\n", stats.TenantId)
	fmt.Printf("  \033[1mTotal Items:\033[0m   %d\n", stats.TotalCount)
	fmt.Println()

	// Count by source type
	if len(stats.CountByType) > 0 {
		fmt.Println("  \033[1mBy Source Type:\033[0m")
		for sourceType, count := range stats.CountByType {
			fmt.Printf("    %-12s %d\n", sourceType+":", count)
		}
		fmt.Println()
	}

	// Count by processing state
	if len(stats.CountByState) > 0 {
		fmt.Println("  \033[1mBy Processing State:\033[0m")
		for state, count := range stats.CountByState {
			fmt.Printf("    %-12s %d\n", state+":", count)
		}
		fmt.Println()
	}

	// Processing metrics
	fmt.Println("  \033[1mProcessing Metrics:\033[0m")
	fmt.Printf("    Embedded:    %d\n", stats.EmbeddedCount)
	fmt.Printf("    Summarized:  %d\n", stats.SummarizedCount)
	fmt.Printf("    Extracted:   %d\n", stats.ExtractedCount)
	fmt.Println()

	// Storage
	fmt.Printf("  \033[1mTotal Storage:\033[0m %s\n", formatBytes(stats.TotalStorageBytes))
	fmt.Println()

	return nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func timestampProto(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

// newContentTraceCommand creates the 'content trace' subcommand.
func newContentTraceCommand(deps *ContentCommandDeps) *cobra.Command {
	var verbose bool
	var outputFormat string
	var traceSource string
	var traceEnv string

	cmd := &cobra.Command{
		Use:   "trace <content-id>",
		Short: "Show processing timeline for content",
		Long: `Show the full processing timeline for a content item.

Displays all processing events from ingestion through completion,
including timestamps, stages, and durations.

By default, shows both pipeline events and Langfuse AI traces merged chronologically.
Use --source to filter to a specific trace source.

Examples:
  # Show all traces (pipeline + Langfuse)
  penf content trace em-gFo2YZi3

  # Show only pipeline events
  penf content trace em-gFo2YZi3 --source pipeline

  # Show only Langfuse AI traces
  penf content trace em-gFo2YZi3 --source langfuse

  # Filter Langfuse by environment
  penf content trace em-gFo2YZi3 --source langfuse --env production

  # Show with verbose details
  penf content trace em-gFo2YZi3 --verbose

  # Output as JSON
  penf content trace em-gFo2YZi3 -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentTrace(cmd.Context(), deps, args[0], verbose, outputFormat, traceSource, traceEnv)
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Include detailed payloads and extra information")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")
	cmd.Flags().StringVar(&traceSource, "source", "all", "Trace source: pipeline, langfuse, all")
	cmd.Flags().StringVar(&traceEnv, "env", "", "Langfuse environment filter")

	return cmd
}

func runContentTrace(ctx context.Context, deps *ContentCommandDeps, contentID string, verbose bool, outputFormat string, source string, env string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Use connectToGateway for proper TLS handling (same as other content commands)
	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Apply output format
	format := cfg.OutputFormat
	if outputFormat != "" {
		format = config.OutputFormat(outputFormat)
	}

	// Validate source parameter
	source = strings.ToLower(source)
	if source != "pipeline" && source != "langfuse" && source != "all" {
		return fmt.Errorf("invalid source: %s (must be: pipeline, langfuse, all)", source)
	}

	// Fetch traces based on source parameter
	var pipelineResp *pipelinev1.GetContentTraceResponse
	var langfuseResp *contentv1.GetContentTraceResponse
	var pipelineErr, langfuseErr error

	// Fetch pipeline trace if requested
	if source == "pipeline" || source == "all" {
		pipelineClient := pipelinev1.NewPipelineServiceClient(conn)
		pipelineResp, pipelineErr = pipelineClient.GetContentTrace(ctx, &pipelinev1.GetContentTraceRequest{
			ContentId: contentID,
			Verbose:   verbose,
		})
		if pipelineErr != nil && source == "pipeline" {
			return fmt.Errorf("getting pipeline trace: %w", pipelineErr)
		}
		if pipelineErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: pipeline trace unavailable: %v\n", pipelineErr)
		}
	}

	// Fetch Langfuse trace if requested
	if source == "langfuse" || source == "all" {
		contentClient := contentv1.NewContentProcessorServiceClient(conn)
		req := &contentv1.GetContentTraceRequest{
			ContentId: contentID,
		}
		if env != "" {
			req.Environment = &env
		}
		langfuseResp, langfuseErr = contentClient.GetContentTrace(ctx, req)
		if langfuseErr != nil && source == "langfuse" {
			return fmt.Errorf("getting Langfuse trace: %w", langfuseErr)
		}
		if langfuseErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Langfuse trace unavailable: %v\n", langfuseErr)
		}
	}

	// Handle case where both sources failed
	if (pipelineErr != nil || pipelineResp == nil) && (langfuseErr != nil || langfuseResp == nil) {
		return fmt.Errorf("no trace data available for content: %s", contentID)
	}

	// Output based on format and source combination
	if format == config.OutputFormatJSON {
		result := map[string]interface{}{
			"content_id": contentID,
		}
		if pipelineResp != nil {
			result["pipeline"] = pipelineResp
		}
		if langfuseResp != nil {
			result["langfuse"] = langfuseResp
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	// Text output
	if source == "all" && pipelineResp != nil && langfuseResp != nil {
		return outputContentTraceMerged(pipelineResp, langfuseResp, verbose)
	} else if source == "langfuse" || (source == "all" && pipelineResp == nil && langfuseResp != nil) {
		return outputLangfuseTraceText(langfuseResp, verbose)
	} else if pipelineResp != nil {
		return outputContentTraceText(pipelineResp, verbose)
	}

	return fmt.Errorf("no trace data to display")
}

func outputContentTraceText(resp *pipelinev1.GetContentTraceResponse, verbose bool) error {
	if len(resp.Events) == 0 {
		fmt.Printf("No trace events found for content: %s\n", resp.ContentId)
		return nil
	}

	fmt.Printf("Content Trace: %s\n", resp.ContentId)
	fmt.Println()
	fmt.Println("Pipeline Events:")
	fmt.Println("───────────────")

	for _, event := range resp.Events {
		timestamp := event.Timestamp.AsTime().Format("15:04:05")

		// Format stage with color
		stageColor := "\033[36m" // Cyan
		stage := strings.ToUpper(event.Stage)

		// Color based on action
		if event.Action == "failed" {
			stageColor = "\033[31m" // Red
		} else if event.Action == "completed" || event.Action == "complete" {
			stageColor = "\033[32m" // Green
		}

		// Format duration if available
		durationStr := ""
		if event.DurationMs > 0 {
			duration := time.Duration(event.DurationMs) * time.Millisecond
			if duration < time.Second {
				durationStr = fmt.Sprintf(" (%dms)", event.DurationMs)
			} else {
				durationStr = fmt.Sprintf(" (%.1fs)", duration.Seconds())
			}
		}

		fmt.Printf("%s [%s%-12s\033[0m] %s%s\n",
			timestamp,
			stageColor,
			stage,
			event.Message,
			durationStr)

		// Show details in verbose mode
		if verbose && len(event.Details) > 0 {
			for key, value := range event.Details {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}
	}

	fmt.Println()
	return nil
}

func outputLangfuseTraceText(resp *contentv1.GetContentTraceResponse, verbose bool) error {
	if len(resp.Traces) == 0 {
		fmt.Printf("No Langfuse traces found for content: %s\n", resp.ContentId)
		return nil
	}

	fmt.Printf("Content Trace: %s\n", resp.ContentId)
	fmt.Println()
	fmt.Println("Langfuse AI Traces:")
	fmt.Println("──────────────────")

	for _, trace := range resp.Traces {
		fmt.Printf("\nTrace: %s (%s)\n", trace.Name, trace.Status)

		if len(trace.Observations) == 0 {
			fmt.Println("  No observations recorded")
			continue
		}

		for _, obs := range trace.Observations {
			timestamp := obs.StartTime.AsTime().Format("15:04:05")

			// Format observation type with color
			obsColor := "\033[36m" // Cyan
			if obs.Status == "ERROR" {
				obsColor = "\033[31m" // Red
			} else if obs.Status == "COMPLETED" {
				obsColor = "\033[32m" // Green
			}

			// Format duration if available
			durationStr := ""
			if obs.EndTime != nil {
				duration := obs.EndTime.AsTime().Sub(obs.StartTime.AsTime())
				if duration < time.Second {
					durationStr = fmt.Sprintf("  %dms", duration.Milliseconds())
				} else {
					durationStr = fmt.Sprintf("  %.1fs", duration.Seconds())
				}
			}

			// Format model info
			modelStr := ""
			if obs.Model != nil && *obs.Model != "" {
				modelStr = fmt.Sprintf("  %s", *obs.Model)
			}

			// Format token counts
			tokenStr := ""
			if obs.TotalTokens != nil && *obs.TotalTokens > 0 {
				tokenStr = fmt.Sprintf("  %s tokens", formatNumber(int64(*obs.TotalTokens)))
			}

			fmt.Printf("%s  [%s%-11s\033[0m]  %s%s%s%s\n",
				timestamp,
				obsColor,
				obs.Type,
				obs.Name,
				modelStr,
				tokenStr,
				durationStr)

			// Show error in verbose mode
			if verbose && obs.Error != nil && *obs.Error != "" {
				fmt.Printf("    Error: %s\n", *obs.Error)
			}

			// Show token breakdown in verbose mode
			if verbose && obs.InputTokens != nil && obs.OutputTokens != nil {
				fmt.Printf("    Tokens: %d in, %d out\n", *obs.InputTokens, *obs.OutputTokens)
			}
		}
	}

	if resp.LangfuseUrl != "" {
		fmt.Printf("\nLangfuse: %s\n", resp.LangfuseUrl)
	}

	fmt.Println()
	return nil
}

// mergedEvent represents a single event from either pipeline or Langfuse traces.
type mergedEvent struct {
	timestamp time.Time
	source    string // "pipeline" or "langfuse"
	stage     string
	message   string
	duration  *time.Duration
	color     string
	details   map[string]string
	error     *string
}

func outputContentTraceMerged(pipelineResp *pipelinev1.GetContentTraceResponse, langfuseResp *contentv1.GetContentTraceResponse, verbose bool) error {
	fmt.Printf("Content: %s\n", pipelineResp.ContentId)
	fmt.Println()
	fmt.Println("Processing Timeline:")
	fmt.Println("───────────────────")

	var events []mergedEvent

	// Add pipeline events
	for _, event := range pipelineResp.Events {
		stage := strings.ToUpper(event.Stage)
		color := "\033[36m" // Cyan

		if event.Action == "failed" {
			color = "\033[31m" // Red
		} else if event.Action == "completed" || event.Action == "complete" {
			color = "\033[32m" // Green
		}

		var duration *time.Duration
		if event.DurationMs > 0 {
			d := time.Duration(event.DurationMs) * time.Millisecond
			duration = &d
		}

		events = append(events, mergedEvent{
			timestamp: event.Timestamp.AsTime(),
			source:    "pipeline",
			stage:     stage,
			message:   event.Message,
			duration:  duration,
			color:     color,
			details:   event.Details,
		})
	}

	// Add Langfuse observations
	for _, trace := range langfuseResp.Traces {
		for _, obs := range trace.Observations {
			color := "\033[36m" // Cyan
			if obs.Status == "ERROR" {
				color = "\033[31m" // Red
			} else if obs.Status == "COMPLETED" {
				color = "\033[32m" // Green
			}

			message := obs.Name
			if obs.Model != nil && *obs.Model != "" {
				message += fmt.Sprintf("  %s", *obs.Model)
			}
			if obs.TotalTokens != nil && *obs.TotalTokens > 0 {
				message += fmt.Sprintf("  %s tokens", formatNumber(int64(*obs.TotalTokens)))
			}

			var duration *time.Duration
			if obs.EndTime != nil {
				d := obs.EndTime.AsTime().Sub(obs.StartTime.AsTime())
				duration = &d
			}

			events = append(events, mergedEvent{
				timestamp: obs.StartTime.AsTime(),
				source:    "langfuse",
				stage:     obs.Type,
				message:   message,
				duration:  duration,
				color:     color,
				error:     obs.Error,
			})
		}
	}

	// Sort events by timestamp
	sortMergedEvents(events)

	// Display merged timeline
	for _, event := range events {
		timestamp := event.timestamp.Format("15:04:05")
		durationStr := ""
		if event.duration != nil {
			if *event.duration < time.Second {
				durationStr = fmt.Sprintf("  %dms", event.duration.Milliseconds())
			} else {
				durationStr = fmt.Sprintf("  %.1fs", event.duration.Seconds())
			}
		}

		fmt.Printf("%s  [%-8s]  %s%-12s\033[0m  %s%s\n",
			timestamp,
			event.source,
			event.color,
			event.stage,
			event.message,
			durationStr)

		// Show details in verbose mode
		if verbose && len(event.details) > 0 {
			for key, value := range event.details {
				fmt.Printf("    %s: %s\n", key, value)
			}
		}

		// Show error in verbose mode
		if verbose && event.error != nil && *event.error != "" {
			fmt.Printf("    Error: %s\n", *event.error)
		}
	}

	if langfuseResp.LangfuseUrl != "" {
		fmt.Printf("\nLangfuse: %s\n", langfuseResp.LangfuseUrl)
	}

	fmt.Println()
	return nil
}

// sortMergedEvents sorts merged events by timestamp in place.
func sortMergedEvents(events []mergedEvent) {
	// Simple insertion sort - good enough for small event lists
	for i := 1; i < len(events); i++ {
		key := events[i]
		j := i - 1
		for j >= 0 && events[j].timestamp.After(key.timestamp) {
			events[j+1] = events[j]
			j--
		}
		events[j+1] = key
	}
}

// formatNumber formats a number with thousands separators.
func formatNumber(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result string
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

// newContentTextCommand creates the 'content text' subcommand.
func newContentTextCommand(deps *ContentCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "text <content-id>",
		Short: "Display raw text content",
		Long: `Display the raw text content of a content item.

Shows the full text content with a header containing content type and created date.

Examples:
  # Display text content
  penf content text mt-abc123

  # Output as JSON
  penf content text mt-abc123 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentText(cmd.Context(), deps, args[0])
		},
	}

	return cmd
}

// newContentInsightsCommand creates the 'content insights' subcommand.
func newContentInsightsCommand(deps *ContentCommandDeps) *cobra.Command {
	var insightTypes string
	var allInsights bool
	var refresh bool

	cmd := &cobra.Command{
		Use:   "insights <content-id>",
		Short: "Display content insights",
		Long: `Display extracted insights for a content item.

Without flags, shows available insight types and their status.
Use --type to retrieve specific insights, or --all for all extracted insights.

Examples:
  # Show available insights
  penf content insights mt-abc123

  # Get a specific insight type
  penf content insights mt-abc123 --type summary

  # Get multiple insight types
  penf content insights mt-abc123 --type actions,decisions

  # Get all extracted insights
  penf content insights mt-abc123 --all

  # Force refresh (triggers re-extraction)
  penf content insights mt-abc123 --type summary --refresh

  # Output as JSON
  penf content insights mt-abc123 --type summary --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentInsights(cmd.Context(), deps, args[0], insightTypes, allInsights, refresh)
		},
	}

	cmd.Flags().StringVar(&insightTypes, "type", "", "Comma-separated insight types to retrieve")
	cmd.Flags().BoolVar(&allInsights, "all", false, "Get all available insights")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Force re-extraction of insights")

	return cmd
}

// runContentText executes the content text command.
func runContentText(ctx context.Context, deps *ContentCommandDeps, contentID string) error {
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

	// Get content text
	resp, err := client.GetContentText(ctx, &contentv1.GetContentTextRequest{
		ContentId: contentID,
	})
	if err != nil {
		return fmt.Errorf("getting content text: %w", err)
	}

	// Output results
	format := cfg.OutputFormat
	if contentOutput != "" {
		format = config.OutputFormat(contentOutput)
	}

	return outputContentText(format, resp)
}

// runContentInsights executes the content insights command.
func runContentInsights(ctx context.Context, deps *ContentCommandDeps, contentID string, insightTypes string, allInsights bool, refresh bool) error {
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

	// If no type specified and not --all, show available insights (discovery mode)
	if insightTypes == "" && !allInsights {
		resp, err := client.ListAvailableInsights(ctx, &contentv1.ListAvailableInsightsRequest{
			ContentId: contentID,
		})
		if err != nil {
			return fmt.Errorf("listing available insights: %w", err)
		}

		format := cfg.OutputFormat
		if contentOutput != "" {
			format = config.OutputFormat(contentOutput)
		}

		return outputAvailableInsights(format, resp)
	}

	// Otherwise, get specific insights
	req := &contentv1.GetInsightsRequest{
		ContentId: contentID,
		Refresh:   refresh,
	}

	// Parse types if specified
	if insightTypes != "" {
		types := strings.Split(insightTypes, ",")
		for i := range types {
			types[i] = strings.TrimSpace(types[i])
		}
		req.Types = types
	}

	resp, err := client.GetInsights(ctx, req)
	if err != nil {
		return fmt.Errorf("getting insights: %w", err)
	}

	// Output results
	format := cfg.OutputFormat
	if contentOutput != "" {
		format = config.OutputFormat(contentOutput)
	}

	return outputInsights(format, resp)
}

// outputContentText outputs content text in the specified format.
func outputContentText(format config.OutputFormat, resp *contentv1.GetContentTextResponse) error {
	switch format {
	case config.OutputFormatJSON:
		return outputContentJSON(resp)
	case config.OutputFormatYAML:
		return outputContentYAML(resp)
	default:
		return outputContentTextText(resp)
	}
}

// outputContentTextText outputs content text in text format.
func outputContentTextText(resp *contentv1.GetContentTextResponse) error {
	fmt.Printf("Content: %s (%s)\n", resp.ContentId, resp.ContentType)
	fmt.Printf("Created: %s\n\n", resp.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))

	// Show metadata if available
	if len(resp.Metadata) > 0 {
		for key, value := range resp.Metadata {
			fmt.Printf("%s: %s\n", strings.Title(key), value)
		}
		fmt.Println()
	}

	// Output the text content
	fmt.Println(resp.Text)
	fmt.Println()

	return nil
}

// outputAvailableInsights outputs available insights in the specified format.
func outputAvailableInsights(format config.OutputFormat, resp *contentv1.ListAvailableInsightsResponse) error {
	switch format {
	case config.OutputFormatJSON:
		return outputContentJSON(resp)
	case config.OutputFormatYAML:
		return outputContentYAML(resp)
	default:
		return outputAvailableInsightsText(resp)
	}
}

// outputAvailableInsightsText outputs available insights in text format.
func outputAvailableInsightsText(resp *contentv1.ListAvailableInsightsResponse) error {
	fmt.Printf("Content: %s (%s)\n", resp.ContentId, resp.ContentType)
	fmt.Println()

	if len(resp.Available) == 0 {
		fmt.Println("No insights available for this content type.")
		return nil
	}

	fmt.Println("Available Insights:")
	fmt.Println("  TYPE          STATUS")
	fmt.Println("  ----          ------")

	// Create a map for quick status lookup
	extractedMap := make(map[string]bool)
	for _, t := range resp.Extracted {
		extractedMap[t] = true
	}

	pendingMap := make(map[string]bool)
	for _, t := range resp.Pending {
		pendingMap[t] = true
	}

	// Display all available types with their status
	for _, insightType := range resp.Available {
		status := "available"
		if extractedMap[insightType] {
			status = "\033[32mextracted\033[0m"
		} else if pendingMap[insightType] {
			status = "\033[33mpending\033[0m"
		}

		fmt.Printf("  %-13s %s\n", insightType, status)
	}

	fmt.Println()
	return nil
}

// outputInsights outputs insights in the specified format.
func outputInsights(format config.OutputFormat, resp *contentv1.GetInsightsResponse) error {
	switch format {
	case config.OutputFormatJSON:
		return outputContentJSON(resp)
	case config.OutputFormatYAML:
		return outputContentYAML(resp)
	default:
		return outputInsightsText(resp)
	}
}

// outputInsightsText outputs insights in text format.
func outputInsightsText(resp *contentv1.GetInsightsResponse) error {
	if len(resp.Insights) == 0 {
		fmt.Println("No insights found.")
		return nil
	}

	for i, insight := range resp.Insights {
		if i > 0 {
			fmt.Println()
			fmt.Println("---")
			fmt.Println()
		}

		// Display insight type as header
		fmt.Printf("\033[1m%s:\033[0m\n", strings.Title(strings.ReplaceAll(insight.Type, "_", " ")))

		// Display the insight data
		if insight.Data != nil {
			displayInsightData(insight.Data.AsMap(), "  ")
		}

		// Display metadata
		fmt.Println()
		fmt.Printf("  Extracted: %s\n", insight.ExtractedAt.AsTime().Format("2006-01-02 15:04:05"))
		if insight.ModelVersion != "" {
			fmt.Printf("  Model: %s\n", insight.ModelVersion)
		}
	}

	fmt.Println()
	return nil
}

// displayInsightData recursively displays insight data as formatted text.
func displayInsightData(data map[string]interface{}, indent string) {
	for key, value := range data {
		switch v := value.(type) {
		case string:
			fmt.Printf("%s%s: %s\n", indent, strings.Title(key), v)
		case []interface{}:
			fmt.Printf("%s%s:\n", indent, strings.Title(key))
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					displayInsightData(itemMap, indent+"  ")
				} else {
					fmt.Printf("%s  - %v\n", indent, item)
				}
			}
		case map[string]interface{}:
			fmt.Printf("%s%s:\n", indent, strings.Title(key))
			displayInsightData(v, indent+"  ")
		default:
			fmt.Printf("%s%s: %v\n", indent, strings.Title(key), v)
		}
	}
}
