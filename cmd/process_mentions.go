// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	mentionsv1 "github.com/otherjamesbrown/penf-cli/api/proto/mentions/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Mention process command flags.
var (
	mentionProcessOutput     string
	mentionProcessLimit      int
	mentionProcessStatus     string
	mentionProcessDryRun     bool
	mentionIncludeCandidates bool
	mentionPatternsLimit     int
	mentionEntityType        string
	mentionContentID         int64
	mentionProjectID         int64
	mentionOffset            int
	mentionSearchQuery       string
	mentionResolveConfidence float64
	mentionResolvePattern    bool
	mentionDismissReason     string
)

// newProcessMentionsCommand creates the 'process mentions' subcommand group.
func newProcessMentionsCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mentions",
		Short: "Process content mentions intelligently",
		Long: `Process mentions found in content that need resolution to entities.

This workflow allows Claude to:
1. Get all pending mentions with their context and candidates
2. Access existing patterns for automatic resolution
3. Batch resolve multiple mentions efficiently
4. Create new patterns for recurring mentions

Mention review workflow details are in Context Palace knowledge shards.`,
		Aliases: []string{"mention"},
	}

	cmd.AddCommand(newMentionsContextCommand(deps))
	cmd.AddCommand(newMentionsBatchResolveCommand(deps))
	cmd.AddCommand(newMentionsListCommand(deps))
	cmd.AddCommand(newMentionsResolveCommand(deps))
	cmd.AddCommand(newMentionsDismissCommand(deps))
	cmd.AddCommand(newMentionsPatternsCommand(deps))
	cmd.AddCommand(newMentionsStatsCommand(deps))

	return cmd
}

// newMentionsContextCommand creates the 'process mentions context' subcommand.
func newMentionsContextCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Get full context for mention processing",
		Long: `Get complete context needed for intelligent mention processing.

Returns:
- All pending mentions with context snippets
- Candidate entities for each mention
- Existing patterns (for auto-resolution)
- Queue statistics
- Workflow guidance (available actions, decision criteria)

This single command provides everything Claude needs to:
1. Match mentions to existing patterns (auto-resolve)
2. Find high-confidence candidates
3. Only ask the user about truly ambiguous items
4. Create new patterns for recurring mentions

Examples:
  penf process mentions context
  penf process mentions context --output json
  penf process mentions context --status pending --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMentionsContext(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&mentionProcessOutput, "output", "o", "json", "Output format: json, text")
	cmd.Flags().IntVar(&mentionProcessLimit, "limit", 100, "Maximum mentions to return")
	cmd.Flags().StringVar(&mentionProcessStatus, "status", "pending", "Filter by status: pending, resolved, all")
	cmd.Flags().BoolVar(&mentionIncludeCandidates, "include-candidates", true, "Include candidate entities for each mention")
	cmd.Flags().IntVar(&mentionPatternsLimit, "patterns-limit", 500, "Maximum patterns to return")

	return cmd
}

// newMentionsBatchResolveCommand creates the 'process mentions batch-resolve' subcommand.
func newMentionsBatchResolveCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch-resolve <json>",
		Short: "Batch resolve mentions to entities",
		Long: `Batch resolve multiple mentions in a single operation.

Accepts JSON with resolutions, patterns, and dismissals:
{
  "resolutions": [
    {"mention_id": 123, "entity_id": 456, "entity_type": "ENTITY_TYPE_PERSON", "create_pattern": true},
    {"mention_id": 789, "entity_id": 101, "entity_type": "ENTITY_TYPE_TERM"}
  ],
  "new_patterns": [
    {"mention_text": "JB", "entity_id": 456, "entity_type": "ENTITY_TYPE_PERSON"}
  ],
  "dismissals": [
    {"mention_id": 202, "reason": "Not a named entity"}
  ]
}

Entity types: ENTITY_TYPE_PERSON, ENTITY_TYPE_TERM, ENTITY_TYPE_PRODUCT, ENTITY_TYPE_COMPANY, ENTITY_TYPE_PROJECT

Use --dry-run to preview changes without executing them.

Example:
  penf process mentions batch-resolve '{"resolutions":[{"mention_id":24,"entity_id":5,"entity_type":"ENTITY_TYPE_PERSON"}]}'
  penf process mentions batch-resolve --dry-run '{"resolutions":[...]}'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMentionsBatchResolve(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().BoolVar(&mentionProcessDryRun, "dry-run", false, "Preview changes without executing them")

	return cmd
}

// runMentionsContext executes the context command.
func runMentionsContext(ctx context.Context, deps *ProcessCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToMentionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := mentionsv1.NewMentionsServiceClient(conn)

	// Build request
	req := &mentionsv1.GetMentionContextRequest{
		Limit:             int32(mentionProcessLimit),
		IncludeCandidates: mentionIncludeCandidates,
		PatternsLimit:     int32(mentionPatternsLimit),
	}

	// Set status filter
	switch mentionProcessStatus {
	case "pending":
		req.Status = mentionsv1.MentionStatus_MENTION_STATUS_PENDING
	case "resolved":
		req.Status = mentionsv1.MentionStatus_MENTION_STATUS_USER_RESOLVED
	case "all":
		req.Status = mentionsv1.MentionStatus_MENTION_STATUS_UNSPECIFIED
	default:
		req.Status = mentionsv1.MentionStatus_MENTION_STATUS_PENDING
	}

	resp, err := client.GetMentionContext(ctx, req)
	if err != nil {
		return fmt.Errorf("getting mention context: %w", err)
	}

	// Output
	return outputMentionContextResponse(mentionProcessOutput, resp)
}

// runMentionsBatchResolve executes the batch-resolve command.
func runMentionsBatchResolve(ctx context.Context, deps *ProcessCommandDeps, jsonInput string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Parse input JSON
	var req mentionsv1.BatchResolveMentionsRequest
	if err := json.Unmarshal([]byte(jsonInput), &req); err != nil {
		return fmt.Errorf("parsing JSON input: %w", err)
	}

	// Dry-run mode: preview changes without executing
	if mentionProcessDryRun {
		fmt.Println("\033[1m=== DRY RUN - No changes will be made ===\033[0m")
		fmt.Println()

		if len(req.Resolutions) > 0 {
			fmt.Printf("Would resolve %d mentions:\n", len(req.Resolutions))
			for _, r := range req.Resolutions {
				pattern := ""
				if r.CreatePattern {
					pattern = " (+ pattern)"
				}
				fmt.Printf("  \033[32m#%d → %s:%d%s\033[0m\n", r.MentionId, r.EntityType.String(), r.EntityId, pattern)
			}
			fmt.Println()
		}

		if len(req.NewPatterns) > 0 {
			fmt.Printf("Would create %d patterns:\n", len(req.NewPatterns))
			for _, p := range req.NewPatterns {
				fmt.Printf("  \033[34m\"%s\" → %s:%d\033[0m\n", p.MentionText, p.EntityType.String(), p.EntityId)
			}
			fmt.Println()
		}

		if len(req.Dismissals) > 0 {
			fmt.Printf("Would dismiss %d mentions:\n", len(req.Dismissals))
			for _, d := range req.Dismissals {
				fmt.Printf("  \033[33m#%d:\033[0m %s\n", d.MentionId, d.Reason)
			}
			fmt.Println()
		}

		fmt.Printf("Summary: %d resolutions, %d patterns, %d dismissals\n",
			len(req.Resolutions), len(req.NewPatterns), len(req.Dismissals))
		fmt.Println("\n\033[2mRun without --dry-run to apply these changes.\033[0m")
		return nil
	}

	conn, err := connectToMentionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := mentionsv1.NewMentionsServiceClient(conn)

	resp, err := client.BatchResolveMentions(ctx, &req)
	if err != nil {
		return fmt.Errorf("batch resolving mentions: %w", err)
	}

	// Output results
	fmt.Println()
	fmt.Printf("Batch complete: %d resolved, %d patterns, %d dismissed",
		resp.Resolved, resp.PatternsCreated, resp.Dismissed)
	if len(resp.Errors) > 0 {
		fmt.Printf(", %d errors\n", len(resp.Errors))
		for _, e := range resp.Errors {
			fmt.Printf("  \033[31mError:\033[0m %s\n", e)
		}
	} else {
		fmt.Println()
	}

	return nil
}

// connectToMentionsGateway creates a gRPC connection to the gateway.
func connectToMentionsGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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

// outputMentionContextResponse outputs the context in the specified format.
func outputMentionContextResponse(format string, resp *mentionsv1.GetMentionContextResponse) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	case "text":
		return outputMentionContextText(resp)
	default:
		// Default to JSON for AI consumption
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}
}

// outputMentionContextText outputs context in human-readable format.
func outputMentionContextText(resp *mentionsv1.GetMentionContextResponse) error {
	fmt.Printf("Mention Resolution Context\n")
	fmt.Printf("==========================\n\n")

	if resp.Stats != nil {
		fmt.Printf("Pending Mentions: %d\n", resp.Stats.TotalPending)
		fmt.Printf("Resolved Today: %d\n", resp.Stats.ResolvedToday)
		fmt.Printf("Patterns: %d\n\n", resp.Stats.PatternsCount)
	}

	if len(resp.Mentions) > 0 {
		fmt.Println("Mentions:")
		for _, m := range resp.Mentions {
			fmt.Printf("  #%-4d [%s] \"%s\"\n", m.Id, m.Status.String(), m.MentionedText)
			if m.ContextSnippet != "" {
				fmt.Printf("         Context: \"%s\"\n", truncateMentionString(m.ContextSnippet, 60))
			}
			if len(m.Candidates) > 0 {
				fmt.Printf("         Candidates: ")
				for i, c := range m.Candidates {
					if i > 0 {
						fmt.Print(", ")
					}
					fmt.Printf("%s (%.0f%%)", c.EntityName, c.Score*100)
					if i >= 2 {
						fmt.Printf(" +%d more", len(m.Candidates)-3)
						break
					}
				}
				fmt.Println()
			}
		}
		fmt.Println()
	}

	if len(resp.Patterns) > 0 {
		fmt.Printf("Top Patterns (%d total):\n", len(resp.Patterns))
		shown := 0
		for _, p := range resp.Patterns {
			if shown >= 10 {
				break
			}
			fmt.Printf("  \"%s\" → %s:%d (used %dx)\n", p.PatternText, p.EntityType.String(), p.ResolvedEntityId, p.TimesLinked)
			shown++
		}
		fmt.Println()
	}

	if resp.Workflow != nil {
		fmt.Println("Batch Command:")
		fmt.Printf("  %s\n", resp.Workflow.BatchResolveCommand)
	}

	return nil
}

// truncateMentionString truncates a string to max length for mention output.
func truncateMentionString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// =============================================================================
// List Command
// =============================================================================

// newMentionsListCommand creates the 'process mentions list' subcommand.
func newMentionsListCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List mentions with filters",
		Long: `List mentions filtered by status, entity type, content ID, or project.

Examples:
  penf process mentions list
  penf process mentions list --status pending --limit 20
  penf process mentions list --entity-type ENTITY_TYPE_PERSON
  penf process mentions list --content-id 123 --include-candidates
  penf process mentions list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMentionsList(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&mentionProcessStatus, "status", "", "Filter by status: pending, resolved, dismissed, all")
	cmd.Flags().StringVar(&mentionEntityType, "entity-type", "", "Filter by entity type: ENTITY_TYPE_PERSON, ENTITY_TYPE_TERM, etc.")
	cmd.Flags().Int64Var(&mentionContentID, "content-id", 0, "Filter by content ID")
	cmd.Flags().Int64Var(&mentionProjectID, "project-id", 0, "Filter by project ID")
	cmd.Flags().IntVar(&mentionProcessLimit, "limit", 50, "Maximum mentions to return")
	cmd.Flags().IntVar(&mentionOffset, "offset", 0, "Number of mentions to skip")
	cmd.Flags().BoolVar(&mentionIncludeCandidates, "include-candidates", false, "Include candidate entities for each mention")
	cmd.Flags().StringVarP(&mentionProcessOutput, "output", "o", "text", "Output format: json, text")

	return cmd
}

// runMentionsList executes the list command.
func runMentionsList(ctx context.Context, deps *ProcessCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToMentionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := mentionsv1.NewMentionsServiceClient(conn)

	// Build request
	req := &mentionsv1.ListMentionsRequest{
		Limit:             int32(mentionProcessLimit),
		Offset:            int32(mentionOffset),
		IncludeCandidates: mentionIncludeCandidates,
	}

	if mentionContentID > 0 {
		req.ContentId = mentionContentID
	}

	if mentionProjectID > 0 {
		req.ProjectId = mentionProjectID
	}

	// Parse status filter
	if mentionProcessStatus != "" {
		switch strings.ToLower(mentionProcessStatus) {
		case "pending":
			req.Status = mentionsv1.MentionStatus_MENTION_STATUS_PENDING
		case "resolved":
			req.Status = mentionsv1.MentionStatus_MENTION_STATUS_USER_RESOLVED
		case "dismissed":
			req.Status = mentionsv1.MentionStatus_MENTION_STATUS_DISMISSED
		case "all":
			req.Status = mentionsv1.MentionStatus_MENTION_STATUS_UNSPECIFIED
		default:
			return fmt.Errorf("invalid status: %s (use: pending, resolved, dismissed, all)", mentionProcessStatus)
		}
	}

	// Parse entity type filter
	if mentionEntityType != "" {
		entityType, ok := mentionsv1.EntityType_value[mentionEntityType]
		if !ok {
			return fmt.Errorf("invalid entity-type: %s", mentionEntityType)
		}
		req.EntityType = mentionsv1.EntityType(entityType)
	}

	resp, err := client.ListMentions(ctx, req)
	if err != nil {
		return fmt.Errorf("listing mentions: %w", err)
	}

	// Output
	return outputListMentionsResponse(mentionProcessOutput, resp)
}

// outputListMentionsResponse outputs the list response in the specified format.
func outputListMentionsResponse(format string, resp *mentionsv1.ListMentionsResponse) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	case "text":
		return outputListMentionsText(resp)
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}
}

// outputListMentionsText outputs the list response in human-readable format.
func outputListMentionsText(resp *mentionsv1.ListMentionsResponse) error {
	fmt.Printf("Mentions (%d total)\n", resp.TotalCount)
	fmt.Printf("==================\n\n")

	if len(resp.Mentions) == 0 {
		fmt.Println("No mentions found.")
		return nil
	}

	for _, m := range resp.Mentions {
		fmt.Printf("#%-6d [%-20s] \"%s\"\n", m.Id, m.Status.String(), m.MentionedText)
		fmt.Printf("        Type: %s\n", m.EntityType.String())
		if m.ContextSnippet != "" {
			fmt.Printf("        Context: \"%s\"\n", truncateMentionString(m.ContextSnippet, 60))
		}
		if m.ResolvedEntityId > 0 {
			fmt.Printf("        Resolved: %s (ID=%d, confidence=%.2f)\n", m.ResolvedEntityName, m.ResolvedEntityId, m.ResolutionConfidence)
		}
		if len(m.Candidates) > 0 {
			fmt.Printf("        Candidates: ")
			for i, c := range m.Candidates {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Printf("%s (%.0f%%)", c.EntityName, c.Score*100)
				if i >= 2 {
					fmt.Printf(" +%d more", len(m.Candidates)-3)
					break
				}
			}
			fmt.Println()
		}
		fmt.Println()
	}

	return nil
}

// =============================================================================
// Resolve Command
// =============================================================================

// newMentionsResolveCommand creates the 'process mentions resolve' subcommand.
func newMentionsResolveCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve <mention-id> <entity-id>",
		Short: "Resolve a single mention to an entity",
		Long: `Resolve a mention to a specific entity.

Examples:
  penf process mentions resolve 123 456 --entity-type ENTITY_TYPE_PERSON
  penf process mentions resolve 123 456 --entity-type ENTITY_TYPE_PERSON --create-pattern
  penf process mentions resolve 789 101 --entity-type ENTITY_TYPE_TERM --confidence 0.95`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMentionsResolve(cmd.Context(), deps, args[0], args[1])
		},
	}

	cmd.Flags().StringVar(&mentionEntityType, "entity-type", "", "Entity type (required): ENTITY_TYPE_PERSON, ENTITY_TYPE_TERM, etc.")
	cmd.Flags().BoolVar(&mentionResolvePattern, "create-pattern", false, "Create a pattern for future auto-resolution")
	cmd.Flags().Float64Var(&mentionResolveConfidence, "confidence", 0.9, "Resolution confidence (0.0-1.0)")

	cmd.MarkFlagRequired("entity-type")

	return cmd
}

// runMentionsResolve executes the resolve command.
func runMentionsResolve(ctx context.Context, deps *ProcessCommandDeps, mentionIDStr, entityIDStr string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Parse IDs
	var mentionID, entityID int64
	if _, err := fmt.Sscanf(mentionIDStr, "%d", &mentionID); err != nil {
		return fmt.Errorf("invalid mention-id: %s", mentionIDStr)
	}
	if _, err := fmt.Sscanf(entityIDStr, "%d", &entityID); err != nil {
		return fmt.Errorf("invalid entity-id: %s", entityIDStr)
	}

	// Parse entity type
	if mentionEntityType == "" {
		return fmt.Errorf("--entity-type is required")
	}
	entityTypeVal, ok := mentionsv1.EntityType_value[mentionEntityType]
	if !ok {
		return fmt.Errorf("invalid entity-type: %s", mentionEntityType)
	}

	conn, err := connectToMentionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := mentionsv1.NewMentionsServiceClient(conn)

	req := &mentionsv1.ResolveMentionRequest{
		MentionId:     mentionID,
		EntityId:      entityID,
		EntityType:    mentionsv1.EntityType(entityTypeVal),
		Confidence:    float32(mentionResolveConfidence),
		CreatePattern: mentionResolvePattern,
	}

	resp, err := client.ResolveMention(ctx, req)
	if err != nil {
		return fmt.Errorf("resolving mention: %w", err)
	}

	// Output result
	if resp.Resolved {
		fmt.Printf("✓ Mention #%d resolved to entity %d\n", mentionID, entityID)
		if resp.PatternCreated {
			fmt.Println("✓ Pattern created for future auto-resolution")
		}
	} else {
		fmt.Printf("✗ Failed to resolve mention #%d\n", mentionID)
	}

	return nil
}

// =============================================================================
// Dismiss Command
// =============================================================================

// newMentionsDismissCommand creates the 'process mentions dismiss' subcommand.
func newMentionsDismissCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dismiss <mention-id>",
		Short: "Dismiss a mention (mark as not a valid entity)",
		Long: `Dismiss a mention, indicating it is not a valid entity reference.

Examples:
  penf process mentions dismiss 123 --reason "Common word, not a name"
  penf process mentions dismiss 456 --reason "Informal phrase"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMentionsDismiss(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVar(&mentionDismissReason, "reason", "", "Reason for dismissal (required)")
	cmd.MarkFlagRequired("reason")

	return cmd
}

// runMentionsDismiss executes the dismiss command.
func runMentionsDismiss(ctx context.Context, deps *ProcessCommandDeps, mentionIDStr string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Parse ID
	var mentionID int64
	if _, err := fmt.Sscanf(mentionIDStr, "%d", &mentionID); err != nil {
		return fmt.Errorf("invalid mention-id: %s", mentionIDStr)
	}

	if mentionDismissReason == "" {
		return fmt.Errorf("--reason is required")
	}

	conn, err := connectToMentionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := mentionsv1.NewMentionsServiceClient(conn)

	req := &mentionsv1.DismissMentionRequest{
		MentionId: mentionID,
		Reason:    mentionDismissReason,
	}

	resp, err := client.DismissMention(ctx, req)
	if err != nil {
		return fmt.Errorf("dismissing mention: %w", err)
	}

	// Output result
	if resp.Dismissed {
		fmt.Printf("✓ Mention #%d dismissed: %s\n", mentionID, mentionDismissReason)
	} else {
		fmt.Printf("✗ Failed to dismiss mention #%d\n", mentionID)
	}

	return nil
}

// =============================================================================
// Patterns Command
// =============================================================================

// newMentionsPatternsCommand creates the 'process mentions patterns' subcommand.
func newMentionsPatternsCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patterns",
		Short: "List resolution patterns",
		Long: `List existing resolution patterns used for auto-resolution.

Examples:
  penf process mentions patterns
  penf process mentions patterns --entity-type ENTITY_TYPE_PERSON
  penf process mentions patterns --project-id 123
  penf process mentions patterns --search "JB"
  penf process mentions patterns --limit 100 --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMentionsPatterns(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&mentionEntityType, "entity-type", "", "Filter by entity type")
	cmd.Flags().Int64Var(&mentionProjectID, "project-id", 0, "Filter by project ID")
	cmd.Flags().StringVar(&mentionSearchQuery, "search", "", "Search pattern text")
	cmd.Flags().IntVar(&mentionProcessLimit, "limit", 50, "Maximum patterns to return")
	cmd.Flags().IntVar(&mentionOffset, "offset", 0, "Number of patterns to skip")
	cmd.Flags().StringVarP(&mentionProcessOutput, "output", "o", "text", "Output format: json, text")

	return cmd
}

// runMentionsPatterns executes the patterns command.
func runMentionsPatterns(ctx context.Context, deps *ProcessCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToMentionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := mentionsv1.NewMentionsServiceClient(conn)

	// Build request
	req := &mentionsv1.ListPatternsRequest{
		Limit:  int32(mentionProcessLimit),
		Offset: int32(mentionOffset),
	}

	if mentionProjectID > 0 {
		req.ProjectId = mentionProjectID
	}

	if mentionSearchQuery != "" {
		req.Search = mentionSearchQuery
	}

	// Parse entity type filter
	if mentionEntityType != "" {
		entityType, ok := mentionsv1.EntityType_value[mentionEntityType]
		if !ok {
			return fmt.Errorf("invalid entity-type: %s", mentionEntityType)
		}
		req.EntityType = mentionsv1.EntityType(entityType)
	}

	resp, err := client.ListPatterns(ctx, req)
	if err != nil {
		return fmt.Errorf("listing patterns: %w", err)
	}

	// Output
	return outputListPatternsResponse(mentionProcessOutput, resp)
}

// outputListPatternsResponse outputs the patterns response in the specified format.
func outputListPatternsResponse(format string, resp *mentionsv1.ListPatternsResponse) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	case "text":
		return outputListPatternsText(resp)
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}
}

// outputListPatternsText outputs the patterns response in human-readable format.
func outputListPatternsText(resp *mentionsv1.ListPatternsResponse) error {
	fmt.Printf("Resolution Patterns (%d total)\n", resp.TotalCount)
	fmt.Printf("============================\n\n")

	if len(resp.Patterns) == 0 {
		fmt.Println("No patterns found.")
		return nil
	}

	for _, p := range resp.Patterns {
		scope := "permanent"
		if p.ProjectId > 0 {
			scope = fmt.Sprintf("project:%d", p.ProjectId)
		}
		fmt.Printf("#%-6d \"%s\" → %s:%d (%s)\n", p.Id, p.PatternText, p.EntityType.String(), p.ResolvedEntityId, p.ResolvedEntityName)
		fmt.Printf("        Scope: %s\n", scope)
		fmt.Printf("        Usage: seen=%d, linked=%d\n", p.TimesSeen, p.TimesLinked)
		if p.Source != "" {
			fmt.Printf("        Source: %s\n", p.Source)
		}
		fmt.Println()
	}

	return nil
}

// =============================================================================
// Stats Command
// =============================================================================

// newMentionsStatsCommand creates the 'process mentions stats' subcommand.
func newMentionsStatsCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Get mention statistics",
		Long: `Get statistics about mentions (pending, resolved, dismissed counts, etc.).

Examples:
  penf process mentions stats
  penf process mentions stats --entity-type ENTITY_TYPE_PERSON
  penf process mentions stats --project-id 123
  penf process mentions stats --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMentionsStats(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&mentionEntityType, "entity-type", "", "Filter by entity type")
	cmd.Flags().Int64Var(&mentionProjectID, "project-id", 0, "Filter by project ID")
	cmd.Flags().StringVarP(&mentionProcessOutput, "output", "o", "text", "Output format: json, text")

	return cmd
}

// runMentionsStats executes the stats command.
func runMentionsStats(ctx context.Context, deps *ProcessCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToMentionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := mentionsv1.NewMentionsServiceClient(conn)

	// Build request
	req := &mentionsv1.GetMentionStatsRequest{}

	if mentionProjectID > 0 {
		req.ProjectId = mentionProjectID
	}

	// Parse entity type filter
	if mentionEntityType != "" {
		entityType, ok := mentionsv1.EntityType_value[mentionEntityType]
		if !ok {
			return fmt.Errorf("invalid entity-type: %s", mentionEntityType)
		}
		req.EntityType = mentionsv1.EntityType(entityType)
	}

	resp, err := client.GetMentionStats(ctx, req)
	if err != nil {
		return fmt.Errorf("getting mention stats: %w", err)
	}

	// Output
	return outputMentionStatsResponse(mentionProcessOutput, resp)
}

// outputMentionStatsResponse outputs the stats response in the specified format.
func outputMentionStatsResponse(format string, resp *mentionsv1.GetMentionStatsResponse) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	case "text":
		return outputMentionStatsText(resp.Stats)
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}
}

// outputMentionStatsText outputs the stats response in human-readable format.
func outputMentionStatsText(stats *mentionsv1.MentionStats) error {
	if stats == nil {
		fmt.Println("No statistics available.")
		return nil
	}

	fmt.Printf("Mention Statistics\n")
	fmt.Printf("==================\n\n")

	fmt.Printf("Pending:       %d\n", stats.TotalPending)
	fmt.Printf("Resolved:      %d\n", stats.TotalResolved)
	fmt.Printf("Dismissed:     %d\n", stats.TotalDismissed)
	fmt.Printf("Patterns:      %d\n", stats.PatternsCount)
	fmt.Println()

	fmt.Printf("Today:\n")
	fmt.Printf("  Resolved:    %d\n", stats.ResolvedToday)
	fmt.Printf("  Auto:        %d\n", stats.AutoResolvedToday)
	fmt.Println()

	if len(stats.ByStatus) > 0 {
		fmt.Printf("By Status:\n")
		for status, count := range stats.ByStatus {
			fmt.Printf("  %-25s %d\n", status, count)
		}
		fmt.Println()
	}

	if len(stats.ByEntityType) > 0 {
		fmt.Printf("By Entity Type:\n")
		for entityType, count := range stats.ByEntityType {
			fmt.Printf("  %-25s %d\n", entityType, count)
		}
		fmt.Println()
	}

	if len(stats.ByContentType) > 0 {
		fmt.Printf("By Content Type:\n")
		for contentType, count := range stats.ByContentType {
			fmt.Printf("  %-25s %d\n", contentType, count)
		}
		fmt.Println()
	}

	return nil
}
