// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	contentv1 "github.com/otherjamesbrown/penf-cli/api/proto/content/v1"
	conversationv1 "github.com/otherjamesbrown/penf-cli/api/proto/conversation/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Conversation command flags.
var (
	conversationLimit  int32
	conversationOffset int32
	conversationOutput string
)

// ConversationCommandDeps holds the dependencies for conversation commands.
type ConversationCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultConversationDeps returns the default dependencies for production use.
func DefaultConversationDeps() *ConversationCommandDeps {
	return &ConversationCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewConversationCommand creates the conversation command group.
func NewConversationCommand(deps *ConversationCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultConversationDeps()
	}

	cmd := &cobra.Command{
		Use:   "conversation",
		Short: "Query conversations",
		Long: `Query conversations and their items.

Conversations group related content by topic, tracking participants and temporal sequence.

This command provides two main operations:

  list      List conversations with pagination
  show      Show detailed conversation view with items and participants

Examples:
  penf conversation list --limit 10
  penf conversation show <conversation-id>
  penf conversation list -o json`,
	}

	cmd.AddCommand(newConversationListCommand(deps))
	cmd.AddCommand(newConversationShowCommand(deps))
	cmd.AddCommand(newConversationStatusCommand(deps))
	cmd.AddCommand(newConversationMergeCommand(deps))
	cmd.AddCommand(newConversationSplitCommand(deps))
	cmd.AddCommand(newConversationUnlinkCommand(deps))
	cmd.AddCommand(newConversationAuditCommand(deps))

	return cmd
}

// newConversationListCommand creates the 'conversation list' subcommand.
func newConversationListCommand(deps *ConversationCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List conversations with pagination",
		Long: `List conversations with pagination.

Conversations are ordered by last seen date (most recent first).

Flags:
  --limit             Maximum results (default 20)
  --offset            Pagination offset
  -o, --output        Output format: text, json, yaml

Examples:
  penf conversation list --limit 10
  penf conversation list --offset 20 --limit 10
  penf conversation list -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConversationList(cmd.Context(), deps)
		},
	}

	cmd.Flags().Int32Var(&conversationLimit, "limit", 20, "Maximum results")
	cmd.Flags().Int32Var(&conversationOffset, "offset", 0, "Pagination offset")
	cmd.Flags().StringVarP(&conversationOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newConversationShowCommand creates the 'conversation show' subcommand.
func newConversationShowCommand(deps *ConversationCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <conversation-id>",
		Short: "Show conversation with items and participants",
		Long: `Show a detailed view of a conversation including items and participants.

Items and participants are displayed with relevant metadata.

Flags:
  -o, --output        Output format: text, json, yaml

Examples:
  penf conversation show <conversation-id>
  penf conversation show <conversation-id> -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conversationID := args[0]
			return runConversationShow(cmd.Context(), deps, conversationID)
		},
	}

	cmd.Flags().StringVarP(&conversationOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// ==================== gRPC Connection ====================

// connectConversationToGateway creates a gRPC connection to the gateway service.
func connectConversationToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if cfg.TLS.Enabled {
		tlsConfig, err := client.LoadClientTLSConfig(&cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("loading TLS config: %w", err)
		}
		if tlsConfig != nil {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, cfg.ServerAddress, opts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to gateway at %s: %w", cfg.ServerAddress, err)
	}

	return conn, nil
}

// getTenantIDForConversations returns the tenant ID from env or config.
func getTenantIDForConversations(deps *ConversationCommandDeps) string {
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant
	}
	if deps.Config != nil && deps.Config.TenantID != "" {
		return deps.Config.TenantID
	}
	// Default tenant.
	return "00000001-0000-0000-0000-000000000001"
}

// ==================== Command Execution Functions ====================

// runConversationList executes the conversation list command.
func runConversationList(ctx context.Context, deps *ConversationCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectConversationToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := conversationv1.NewConversationServiceClient(conn)
	tenantID := getTenantIDForConversations(deps)

	// Build request
	req := &conversationv1.ListConversationsRequest{
		TenantId: tenantID,
		Limit:    conversationLimit,
		Offset:   conversationOffset,
	}

	// Execute request
	resp, err := client.ListConversations(ctx, req)
	if err != nil {
		return fmt.Errorf("listing conversations: %w", err)
	}

	// Output results
	return outputConversationList(resp)
}

// runConversationShow executes the conversation show command.
func runConversationShow(ctx context.Context, deps *ConversationCommandDeps, conversationID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectConversationToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := conversationv1.NewConversationServiceClient(conn)
	tenantID := getTenantIDForConversations(deps)

	// Build request
	req := &conversationv1.ShowConversationRequest{
		TenantId:       tenantID,
		ConversationId: conversationID,
	}

	// Execute request
	resp, err := client.ShowConversation(ctx, req)
	if err != nil {
		return fmt.Errorf("showing conversation: %w", err)
	}

	// Output results
	return outputConversationDetail(resp)
}

// ==================== Output Functions ====================

// outputConversationList formats and displays the conversation list response.
func outputConversationList(resp *conversationv1.ListConversationsResponse) error {
	switch conversationOutput {
	case "json":
		return outputJSON(resp)
	case "yaml":
		return outputYAML(resp)
	default:
		return outputConversationListText(resp)
	}
}

// outputConversationListText displays conversations as a formatted table.
func outputConversationListText(resp *conversationv1.ListConversationsResponse) error {
	if len(resp.Conversations) == 0 {
		fmt.Println("No conversations found.")
		return nil
	}

	// Header
	fmt.Printf("%-38s %-50s %-8s %-13s %-20s\n",
		"ID", "TOPIC", "ITEMS", "PARTICIPANTS", "LAST SEEN")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────")

	// Rows
	for _, c := range resp.Conversations {
		topic := truncateThreadString(c.Topic, 50)
		lastSeen := formatThreadTimestamp(c.LastSeen)

		fmt.Printf("%-38s %-50s %-8d %-13d %-20s\n",
			c.Id,
			topic,
			c.ItemCount,
			c.ParticipantCount,
			lastSeen)
	}

	// Footer
	fmt.Printf("\nShowing %d conversations (Total: %d)\n", len(resp.Conversations), resp.TotalCount)

	return nil
}

// outputConversationDetail formats and displays the conversation detail response.
func outputConversationDetail(resp *conversationv1.ShowConversationResponse) error {
	switch conversationOutput {
	case "json":
		return outputJSON(resp)
	case "yaml":
		return outputYAML(resp)
	default:
		return outputConversationDetailText(resp)
	}
}

// conversationStatusCommand creates the 'conversation status' subcommand.
func conversationStatusCommand(deps *ConversationCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <conversation-id>",
		Short: "Show processing status for all items in a conversation",
		Long: `Show aggregated processing status for a conversation.

Displays a summary of processing states across all content items in the
conversation, plus a per-item breakdown with stage details.

Examples:
  penf conversation status conv-thread-63
  penf conversation status conv-thread-63 -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConversationStatus(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVarP(&conversationOutput, "output", "o", "", "Output format: text, json")

	return cmd
}

// newConversationStatusCommand wraps conversationStatusCommand for registration.
func newConversationStatusCommand(deps *ConversationCommandDeps) *cobra.Command {
	return conversationStatusCommand(deps)
}

// runConversationStatus executes the conversation status command.
func runConversationStatus(ctx context.Context, deps *ConversationCommandDeps, conversationID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectConversationToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := conversationv1.NewConversationServiceClient(conn)

	resp, err := client.GetConversationProcessingStatus(ctx, &conversationv1.GetConversationProcessingStatusRequest{
		ConversationId: conversationID,
	})
	if err != nil {
		return fmt.Errorf("getting conversation processing status: %w", err)
	}

	switch conversationOutput {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	default:
		return outputConversationStatusText(resp)
	}
}

// outputConversationStatusText displays conversation processing status as formatted text.
func outputConversationStatusText(resp *conversationv1.ConversationProcessingStatus) error {
	fmt.Printf("Conversation: %s\n", resp.ConversationId)
	fmt.Printf("Topic:        %s\n", resp.Topic)
	fmt.Println()

	// Summary counts
	fmt.Printf("  Total: %d   Completed: %d   Processing: %d   Failed: %d   Pending: %d\n",
		resp.TotalItems, resp.Completed, resp.Processing, resp.Failed, resp.Pending)

	// Token totals
	if resp.TotalInputTokens != nil || resp.TotalOutputTokens != nil {
		inTok := int32(0)
		outTok := int32(0)
		if resp.TotalInputTokens != nil {
			inTok = *resp.TotalInputTokens
		}
		if resp.TotalOutputTokens != nil {
			outTok = *resp.TotalOutputTokens
		}
		fmt.Printf("  Tokens: %s in / %s out\n", formatNumber(int64(inTok)), formatNumber(int64(outTok)))
	}

	// Per-item table
	if len(resp.Items) > 0 {
		fmt.Println()
		fmt.Println("  CONTENT ID                 SOURCE  STATE        CONTRIBUTION    STAGES")
		fmt.Println("  ----------                 ------  -----        ------------    ------")

		for _, item := range resp.Items {
			state := formatProcessingState(item.State)
			contrib := item.ContentContribution
			if contrib == "" {
				contrib = "-"
			}

			// Summarize stages
			stagesStr := summarizeStages(item.Stages)

			fmt.Printf("  %-26s %-7d %-12s %-15s %s\n",
				truncate(item.ContentId, 26),
				item.SourceId,
				state,
				truncate(contrib, 15),
				stagesStr)
		}
	}

	fmt.Println()
	return nil
}

// summarizeStages creates a compact stage summary like "5/9 done, 3 skipped, 1 failed".
func summarizeStages(stages []*contentv1.StageResult) string {
	if len(stages) == 0 {
		return "-"
	}

	var done, skipped, failed, running int
	var totalDuration time.Duration
	for _, sr := range stages {
		switch sr.Status {
		case contentv1.StageStatus_STAGE_STATUS_COMPLETED:
			done++
		case contentv1.StageStatus_STAGE_STATUS_SKIPPED:
			skipped++
		case contentv1.StageStatus_STAGE_STATUS_FAILED:
			failed++
		case contentv1.StageStatus_STAGE_STATUS_RUNNING:
			running++
		}
		if sr.DurationMs != nil {
			totalDuration += time.Duration(*sr.DurationMs) * time.Millisecond
		}
	}

	parts := []string{}
	parts = append(parts, fmt.Sprintf("%d/%d done", done, len(stages)))
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skip", skipped))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d fail", failed))
	}
	if running > 0 {
		parts = append(parts, fmt.Sprintf("%d run", running))
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}

	if totalDuration > 0 {
		if totalDuration < time.Second {
			result += fmt.Sprintf(" (%dms)", totalDuration.Milliseconds())
		} else {
			result += fmt.Sprintf(" (%.1fs)", totalDuration.Seconds())
		}
	}

	return result
}

// ==================== Merge / Split / Unlink Commands ====================

// newConversationMergeCommand creates the 'conversation merge' subcommand.
func newConversationMergeCommand(deps *ConversationCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge <source-id> <target-id>",
		Short: "Merge two conversations (source into target)",
		Long: `Merge two conversations by moving all items from source into target.

The source conversation is deleted after merge. Duplicate items (already in
target) are skipped. The target summary is regenerated.

Examples:
  penf conversation merge conv-decb00c8 conv-34b2785a
  penf conversation merge conv-abc conv-def -o json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConversationMerge(cmd.Context(), deps, args[0], args[1])
		},
	}

	cmd.Flags().StringVarP(&conversationOutput, "output", "o", "", "Output format: text, json")

	return cmd
}

// newConversationSplitCommand creates the 'conversation split' subcommand.
func newConversationSplitCommand(deps *ConversationCommandDeps) *cobra.Command {
	var splitItems []string
	var splitTopic string

	cmd := &cobra.Command{
		Use:   "split <conversation-id>",
		Short: "Split items into a new conversation",
		Long: `Extract specified items from a conversation into a new one.

The original conversation keeps its remaining items. Both conversations
get regenerated summaries.

Examples:
  penf conversation split conv-abc --items em-123,em-456
  penf conversation split conv-abc --items em-123 --topic "GPU procurement"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConversationSplit(cmd.Context(), deps, args[0], splitItems, splitTopic)
		},
	}

	cmd.Flags().StringSliceVar(&splitItems, "items", nil, "Content IDs to extract (comma-separated)")
	cmd.Flags().StringVar(&splitTopic, "topic", "", "Topic for new conversation (LLM-generated if empty)")
	cmd.Flags().StringVarP(&conversationOutput, "output", "o", "", "Output format: text, json")
	_ = cmd.MarkFlagRequired("items")

	return cmd
}

// newConversationUnlinkCommand creates the 'conversation unlink' subcommand.
func newConversationUnlinkCommand(deps *ConversationCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlink <conversation-id> <content-id>",
		Short: "Remove a content item from a conversation",
		Long: `Remove a single content item from a conversation.

The conversation summary is regenerated after removal.

Examples:
  penf conversation unlink conv-abc em-123`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConversationUnlink(cmd.Context(), deps, args[0], args[1])
		},
	}

	cmd.Flags().StringVarP(&conversationOutput, "output", "o", "", "Output format: text, json")

	return cmd
}

// runConversationMerge executes the conversation merge command.
func runConversationMerge(ctx context.Context, deps *ConversationCommandDeps, sourceID, targetID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectConversationToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := conversationv1.NewConversationServiceClient(conn)
	tenantID := getTenantIDForConversations(deps)

	resp, err := client.MergeConversations(ctx, &conversationv1.MergeConversationsRequest{
		TenantId:             tenantID,
		SourceConversationId: sourceID,
		TargetConversationId: targetID,
	})
	if err != nil {
		return fmt.Errorf("merging conversations: %w", err)
	}

	switch conversationOutput {
	case "json":
		return outputJSON(resp)
	default:
		fmt.Printf("Merged %s into %s (%d items moved, %d duplicates skipped)\n",
			sourceID, resp.ConversationId, resp.ItemsMoved, resp.DuplicatesSkipped)
		if resp.NewSummary != "" {
			fmt.Printf("Summary: %s\n", resp.NewSummary)
		}
		return nil
	}
}

// runConversationSplit executes the conversation split command.
func runConversationSplit(ctx context.Context, deps *ConversationCommandDeps, conversationID string, items []string, topic string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectConversationToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := conversationv1.NewConversationServiceClient(conn)
	tenantID := getTenantIDForConversations(deps)

	req := &conversationv1.SplitConversationRequest{
		TenantId:       tenantID,
		ConversationId: conversationID,
		ContentIds:     items,
	}
	if topic != "" {
		req.NewTopic = &topic
	}

	resp, err := client.SplitConversation(ctx, req)
	if err != nil {
		return fmt.Errorf("splitting conversation: %w", err)
	}

	switch conversationOutput {
	case "json":
		return outputJSON(resp)
	default:
		fmt.Printf("Split %d items into %s %q\n", resp.ItemsMoved, resp.NewConversationId, resp.NewTopic)
		return nil
	}
}

// runConversationUnlink executes the conversation unlink command.
func runConversationUnlink(ctx context.Context, deps *ConversationCommandDeps, conversationID, contentID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectConversationToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := conversationv1.NewConversationServiceClient(conn)
	tenantID := getTenantIDForConversations(deps)

	resp, err := client.UnlinkItem(ctx, &conversationv1.UnlinkItemRequest{
		TenantId:       tenantID,
		ConversationId: conversationID,
		ContentId:      contentID,
	})
	if err != nil {
		return fmt.Errorf("unlinking item: %w", err)
	}

	switch conversationOutput {
	case "json":
		return outputJSON(resp)
	default:
		fmt.Printf("Removed %s from %s (%d items remaining)\n", contentID, conversationID, resp.RemainingItems)
		return nil
	}
}

// ==================== Audit Command ====================

// newConversationAuditCommand creates the 'conversation audit' subcommand.
func newConversationAuditCommand(deps *ConversationCommandDeps) *cobra.Command {
	var orphansOnly, duplicatesOnly, mergeOnly bool

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit conversation linkages for issues",
		Long: `Run conversation audit to detect orphans, duplicates, and merge candidates.

By default runs all checks. Use flags to run individual checks.

Examples:
  penf conversation audit
  penf conversation audit --orphans
  penf conversation audit --duplicates
  penf conversation audit --merge-candidates`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConversationAudit(cmd.Context(), deps, orphansOnly, duplicatesOnly, mergeOnly)
		},
	}

	cmd.Flags().BoolVar(&orphansOnly, "orphans", false, "Only detect orphaned items")
	cmd.Flags().BoolVar(&duplicatesOnly, "duplicates", false, "Only detect duplicate memberships")
	cmd.Flags().BoolVar(&mergeOnly, "merge-candidates", false, "Only detect merge candidates")
	cmd.Flags().StringVarP(&conversationOutput, "output", "o", "", "Output format: text, json")

	return cmd
}

// runConversationAudit executes the conversation audit command.
func runConversationAudit(ctx context.Context, deps *ConversationCommandDeps, orphansOnly, duplicatesOnly, mergeOnly bool) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectConversationToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := conversationv1.NewConversationServiceClient(conn)
	tenantID := getTenantIDForConversations(deps)

	resp, err := client.RunConversationAudit(ctx, &conversationv1.RunConversationAuditRequest{
		TenantId:       tenantID,
		OrphansOnly:    orphansOnly,
		DuplicatesOnly: duplicatesOnly,
		MergeOnly:      mergeOnly,
	})
	if err != nil {
		return fmt.Errorf("running audit: %w", err)
	}

	switch conversationOutput {
	case "json":
		return outputJSON(resp)
	default:
		return outputConversationAuditText(resp)
	}
}

// outputConversationAuditText displays audit results as formatted text.
func outputConversationAuditText(resp *conversationv1.RunConversationAuditResponse) error {
	fmt.Printf("Conversations audited: %d\n\n", resp.ConversationsAudited)

	if len(resp.Orphans) > 0 {
		fmt.Printf("Orphaned Items (%d):\n", len(resp.Orphans))
		for _, o := range resp.Orphans {
			suggested := ""
			if o.SuggestedConversationId != "" {
				suggested = fmt.Sprintf(" → suggest: %s", o.SuggestedConversationId)
			}
			fmt.Printf("  %s  %s%s\n", o.ContentId, o.Reason, suggested)
		}
		fmt.Println()
	}

	if len(resp.Flagged) > 0 {
		fmt.Printf("Duplicate Memberships (%d):\n", len(resp.Flagged))
		for _, f := range resp.Flagged {
			fmt.Printf("  %s in %s  %s [%s]\n", f.ItemId, f.ConversationId, f.Reason, f.SuggestedAction)
		}
		fmt.Println()
	}

	if len(resp.MergeCandidates) > 0 {
		fmt.Printf("Merge Candidates (%d):\n", len(resp.MergeCandidates))
		for _, m := range resp.MergeCandidates {
			fmt.Printf("  %s (%s) + %s (%s)\n", m.ConversationIdA, truncate(m.TopicA, 30), m.ConversationIdB, truncate(m.TopicB, 30))
			fmt.Printf("    Reason: %s", m.Reason)
			if len(m.SharedItems) > 0 {
				fmt.Printf(" (%d shared items)", len(m.SharedItems))
			}
			fmt.Println()
		}
		fmt.Println()
	}

	if len(resp.Orphans) == 0 && len(resp.Flagged) == 0 && len(resp.MergeCandidates) == 0 {
		fmt.Println("No issues found.")
	}

	return nil
}

// outputConversationDetailText displays conversation details as formatted text.
func outputConversationDetailText(resp *conversationv1.ShowConversationResponse) error {
	// Conversation header
	fmt.Printf("Conversation ID:  %s\n", resp.Id)
	fmt.Printf("Topic:            %s\n", resp.Topic)
	fmt.Printf("Items:            %d\n", resp.ItemCount)
	fmt.Printf("Participants:     %d\n", resp.ParticipantCount)
	fmt.Printf("First Seen:       %s\n", formatThreadTimestamp(resp.FirstSeen))
	fmt.Printf("Last Seen:        %s\n", formatThreadTimestamp(resp.LastSeen))
	if resp.State != nil {
		fmt.Printf("State:            %s\n", *resp.State)
	}
	if resp.StateReason != nil {
		fmt.Printf("State Reason:     %s\n", *resp.StateReason)
	}
	if resp.StateSummary != nil {
		fmt.Printf("Summary:          %s\n", *resp.StateSummary)
	}

	// Items
	if len(resp.Items) > 0 {
		fmt.Println("\nItems:")
		fmt.Println("──────────────────────────────────────────────────────────────────────────────────────────────────────────────────")
		fmt.Printf("%-16s %-20s %-22s %s\n", "CONTENT ID", "DATE", "FROM", "SUBJECT")
		fmt.Println("──────────────────────────────────────────────────────────────────────────────────────────────────────────────────")

		for _, item := range resp.Items {
			date := formatThreadTimestamp(item.ContentDate)
			from := ""
			if item.FromName != nil {
				from = *item.FromName
			}
			subject := ""
			if item.Subject != nil {
				subject = *item.Subject
			}

			fmt.Printf("%-16s %-20s %-22s %s\n",
				item.ContentId,
				date,
				truncateThreadString(from, 22),
				truncateThreadString(subject, 50))
		}
	}

	// Participants
	if len(resp.Participants) > 0 {
		fmt.Println("\nParticipants:")
		fmt.Println("────────────────────────────────────────────────────────────────────────────────────────")
		fmt.Printf("%-30s %-40s\n", "NAME", "ADDRESS")
		fmt.Println("────────────────────────────────────────────────────────────────────────────────────────")

		for _, p := range resp.Participants {
			name := "N/A"
			if p.Name != nil {
				name = *p.Name
			}
			address := "N/A"
			if p.Address != nil {
				address = *p.Address
			}

			fmt.Printf("%-30s %-40s\n",
				truncateThreadString(name, 30),
				truncateThreadString(address, 40))
		}
	}

	fmt.Println()
	return nil
}
