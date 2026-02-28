// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	reviewv1 "github.com/otherjamesbrown/penf-cli/api/proto/review/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// ReviewPriority defines the priority levels for review items.
type ReviewPriority string

const (
	// ReviewPriorityHigh is for urgent items requiring immediate attention.
	ReviewPriorityHigh ReviewPriority = "high"
	// ReviewPriorityMedium is for standard priority items.
	ReviewPriorityMedium ReviewPriority = "medium"
	// ReviewPriorityLow is for items that can be addressed later.
	ReviewPriorityLow ReviewPriority = "low"
)

// ReviewItemStatus defines the status of a review item.
type ReviewItemStatus string

const (
	// ReviewItemStatusPending indicates the item is awaiting review.
	ReviewItemStatusPending ReviewItemStatus = "pending"
	// ReviewItemStatusAccepted indicates the item has been accepted.
	ReviewItemStatusAccepted ReviewItemStatus = "accepted"
	// ReviewItemStatusRejected indicates the item has been rejected.
	ReviewItemStatusRejected ReviewItemStatus = "rejected"
	// ReviewItemStatusDeferred indicates the item has been deferred.
	ReviewItemStatusDeferred ReviewItemStatus = "deferred"
)

// ReviewSessionStatus defines the status of a review session.
type ReviewSessionStatus string

const (
	// ReviewSessionStatusActive indicates an active review session.
	ReviewSessionStatusActive ReviewSessionStatus = "active"
	// ReviewSessionStatusPaused indicates a paused review session.
	ReviewSessionStatusPaused ReviewSessionStatus = "paused"
	// ReviewSessionStatusEnded indicates an ended review session.
	ReviewSessionStatusEnded ReviewSessionStatus = "ended"
)

// ReviewItem represents a single item in the review queue.
type ReviewItem struct {
	ID          string           `json:"id" yaml:"id"`
	Title       string           `json:"title" yaml:"title"`
	ContentType string           `json:"content_type" yaml:"content_type"`
	Source      string           `json:"source" yaml:"source"`
	Priority    ReviewPriority   `json:"priority" yaml:"priority"`
	Status      ReviewItemStatus `json:"status" yaml:"status"`
	Summary     string           `json:"summary,omitempty" yaml:"summary,omitempty"`
	CreatedAt   time.Time        `json:"created_at" yaml:"created_at"`
	DeferredTo  *time.Time       `json:"deferred_to,omitempty" yaml:"deferred_to,omitempty"`
	Reason      string           `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// ReviewSession represents an active review session.
type ReviewSession struct {
	ID            string              `json:"id" yaml:"id"`
	Status        ReviewSessionStatus `json:"status" yaml:"status"`
	StartedAt     time.Time           `json:"started_at" yaml:"started_at"`
	PausedAt      *time.Time          `json:"paused_at,omitempty" yaml:"paused_at,omitempty"`
	EndedAt       *time.Time          `json:"ended_at,omitempty" yaml:"ended_at,omitempty"`
	TotalReviewed int                 `json:"total_reviewed" yaml:"total_reviewed"`
	Accepted      int                 `json:"accepted" yaml:"accepted"`
	Rejected      int                 `json:"rejected" yaml:"rejected"`
	Deferred      int                 `json:"deferred" yaml:"deferred"`
}

// ReviewAction represents an action taken during a review session.
type ReviewAction struct {
	ID        string           `json:"id" yaml:"id"`
	ItemID    string           `json:"item_id" yaml:"item_id"`
	Action    string           `json:"action" yaml:"action"`
	OldStatus ReviewItemStatus `json:"old_status" yaml:"old_status"`
	NewStatus ReviewItemStatus `json:"new_status" yaml:"new_status"`
	Reason    string           `json:"reason,omitempty" yaml:"reason,omitempty"`
	Timestamp time.Time        `json:"timestamp" yaml:"timestamp"`
	Undone    bool             `json:"undone" yaml:"undone"`
}

// ReviewAutoRule represents an automation rule for review.
type ReviewAutoRule struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	Criteria    string `json:"criteria,omitempty" yaml:"criteria,omitempty"`
	Action      string `json:"action" yaml:"action"`
}

// ReviewQueueResponse contains the review queue and metadata.
type ReviewQueueResponse struct {
	Items      []ReviewItem `json:"items" yaml:"items"`
	TotalCount int          `json:"total_count" yaml:"total_count"`
	FetchedAt  time.Time    `json:"fetched_at" yaml:"fetched_at"`
}

// ReviewCommandDeps holds the dependencies for review commands.
type ReviewCommandDeps struct {
	Config       *config.CLIConfig
	GRPCClient   *client.GRPCClient
	OutputFormat config.OutputFormat
	LoadConfig   func() (*config.CLIConfig, error)
	InitClient   func(*config.CLIConfig) (*client.GRPCClient, error)
}

// DefaultReviewDeps returns the default dependencies for production use.
func DefaultReviewDeps() *ReviewCommandDeps {
	return &ReviewCommandDeps{
		LoadConfig: config.LoadConfig,
		InitClient: client.ConnectFromConfig,
	}
}

// Review command flags.
var (
	reviewPriority        string
	reviewCountOnly       bool
	reviewReason          string
	reviewUntil           string
	reviewOutput          string
	reviewDate            string
	reviewIncludeProcessed bool
	reviewFrom            string
	reviewTo              string
)

// NewReviewCommand creates the root review command with all subcommands.
func NewReviewCommand(deps *ReviewCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultReviewDeps()
	}

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Manage review sessions and queue",
		Long: `Manage review sessions and the review queue in Penfold.

The review system allows you to process incoming content items, make decisions
on their relevance, and track your review progress over time.

Session Commands:
  start    Start a new review session
  pause    Pause the current session
  resume   Resume a paused session
  end      End the session and show summary

Queue Commands:
  queue    List pending review items

Item Commands:
  accept   Accept a review item
  reject   Reject a review item
  defer    Defer a review item
  show     Show item details

History Commands:
  undo     Undo the last action
  redo     Redo the last undone action
  history  Show action history

Automation Commands:
  auto     Manage automation rules

For AI-Powered Review:
  Use 'penf process' commands for intelligent batch processing instead of
  individual review commands. This provides full context for better decisions.

Review workflows are documented in Context Palace knowledge shards.`,
	}

	// Add session subcommands.
	cmd.AddCommand(newReviewStartCommand(deps))
	cmd.AddCommand(newReviewPauseCommand(deps))
	cmd.AddCommand(newReviewResumeCommand(deps))
	cmd.AddCommand(newReviewEndCommand(deps))

	// Add queue subcommand.
	cmd.AddCommand(newReviewQueueCommand(deps))

	// Add item subcommands.
	cmd.AddCommand(newReviewAcceptCommand(deps))
	cmd.AddCommand(newReviewRejectCommand(deps))
	cmd.AddCommand(newReviewDeferCommand(deps))
	cmd.AddCommand(newReviewShowCommand(deps))

	// Add undo/redo/history subcommands.
	cmd.AddCommand(newReviewUndoCommand(deps))
	cmd.AddCommand(newReviewRedoCommand(deps))
	cmd.AddCommand(newReviewHistoryCommand(deps))

	// Add auto subcommand.
	cmd.AddCommand(newReviewAutoCommand(deps))

	// Add questions subcommand for AI review queue.
	cmd.AddCommand(newReviewQuestionsCommand(deps))

	// Add daily, session, stats subcommands.
	cmd.AddCommand(newReviewDailyCommand(deps))
	cmd.AddCommand(newReviewSessionCommand(deps))
	cmd.AddCommand(newReviewStatsCommand(deps))

	return cmd
}

// newReviewStartCommand creates the 'review start' subcommand.
func newReviewStartCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start a new review session",
		Long: `Start a new review session.

A review session tracks your progress through the review queue and provides
statistics when you end the session. Only one session can be active at a time.

Example:
  penf review start`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewStart(cmd.Context(), deps)
		},
	}
}

// newReviewPauseCommand creates the 'review pause' subcommand.
func newReviewPauseCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "pause",
		Short: "Pause the current review session",
		Long: `Pause the current review session.

Your progress is saved and you can resume later with 'penf review resume'.

Example:
  penf review pause`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewPause(cmd.Context(), deps)
		},
	}
}

// newReviewResumeCommand creates the 'review resume' subcommand.
func newReviewResumeCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "resume",
		Short: "Resume a paused review session",
		Long: `Resume a previously paused review session.

Example:
  penf review resume`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewResume(cmd.Context(), deps)
		},
	}
}

// newReviewEndCommand creates the 'review end' subcommand.
func newReviewEndCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "end",
		Short: "End the current review session",
		Long: `End the current review session and display a summary.

The summary includes the number of items reviewed, accepted, rejected, and deferred.

Example:
  penf review end`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewEnd(cmd.Context(), deps)
		},
	}
}

// newReviewQueueCommand creates the 'review queue' subcommand.
func newReviewQueueCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "List pending review items",
		Long: `List items pending review.

Use --priority to filter by priority level (high, medium, low).
Use --count to show only the count of pending items.

Examples:
  penf review queue
  penf review queue --priority high
  penf review queue --count`,
		Aliases: []string{"q", "list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewQueue(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&reviewPriority, "priority", "p", "", "Filter by priority: high, medium, low")
	cmd.Flags().BoolVarP(&reviewCountOnly, "count", "c", false, "Show count only")
	cmd.Flags().StringVarP(&reviewOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newReviewAcceptCommand creates the 'review accept' subcommand.
func newReviewAcceptCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "accept <id>",
		Short: "Accept a review item",
		Long: `Accept a review item, marking it as relevant and processed.

Example:
  penf review accept item-123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewAccept(cmd.Context(), deps, args[0])
		},
	}
}

// newReviewRejectCommand creates the 'review reject' subcommand.
func newReviewRejectCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reject <id>",
		Short: "Reject a review item",
		Long: `Reject a review item, marking it as not relevant.

Use --reason to provide context for the rejection.

Examples:
  penf review reject item-123
  penf review reject item-123 --reason "Duplicate content"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewReject(cmd.Context(), deps, args[0], reviewReason)
		},
	}

	cmd.Flags().StringVarP(&reviewReason, "reason", "r", "", "Reason for rejection")

	return cmd
}

// newReviewDeferCommand creates the 'review defer' subcommand.
func newReviewDeferCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "defer <id>",
		Short: "Defer a review item",
		Long: `Defer a review item for later processing.

Use --until to specify when the item should reappear in the queue.

Examples:
  penf review defer item-123
  penf review defer item-123 --until "tomorrow"
  penf review defer item-123 --until "2024-12-31"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewDefer(cmd.Context(), deps, args[0], reviewUntil)
		},
	}

	cmd.Flags().StringVarP(&reviewUntil, "until", "u", "", "Defer until date (YYYY-MM-DD or relative: tomorrow, nextweek)")

	return cmd
}

// newReviewShowCommand creates the 'review show' subcommand.
func newReviewShowCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show review item details",
		Long: `Show detailed information about a review item.

Example:
  penf review show item-123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewShow(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVarP(&reviewOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newReviewUndoCommand creates the 'review undo' subcommand.
func newReviewUndoCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "undo [item-id]",
		Short: "Undo the last review action on an item",
		Long: `Undo the last review action on a specific item.

Restores the item to its previous state before the last accept/reject/defer.

Example:
  penf review undo item-123`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var itemID string
			if len(args) > 0 {
				itemID = args[0]
			}
			return runReviewUndo(cmd.Context(), deps, itemID)
		},
	}
}

// newReviewRedoCommand creates the 'review redo' subcommand.
func newReviewRedoCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "redo",
		Short: "Redo the last undone action",
		Long: `Redo the last undone review action.

Reapplies an action that was previously undone.

Example:
  penf review redo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewRedo(cmd.Context(), deps)
		},
	}
}

// newReviewHistoryCommand creates the 'review history' subcommand.
func newReviewHistoryCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show review action history",
		Long: `Show the history of review actions in the current session.

Example:
  penf review history`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewHistory(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&reviewOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newReviewAutoCommand creates the 'review auto' subcommand.
func newReviewAutoCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auto",
		Short: "Manage automation rules",
		Long: `Manage automation rules for review processing.

Automation rules can automatically accept, reject, or defer items based on
configurable criteria.

Subcommands:
  status   Show current automation rules and their status
  enable   Enable an automation rule
  disable  Disable an automation rule

Examples:
  penf review auto status
  penf review auto enable auto-archive-spam
  penf review auto disable auto-accept-known`,
	}

	cmd.AddCommand(newReviewAutoStatusCommand(deps))
	cmd.AddCommand(newReviewAutoEnableCommand(deps))
	cmd.AddCommand(newReviewAutoDisableCommand(deps))

	return cmd
}

// newReviewAutoStatusCommand creates the 'review auto status' subcommand.
func newReviewAutoStatusCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show automation rules status",
		Long: `Show all automation rules and their current status.

Example:
  penf review auto status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewAutoStatus(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&reviewOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newReviewAutoEnableCommand creates the 'review auto enable' subcommand.
func newReviewAutoEnableCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <rule>",
		Short: "Enable an automation rule",
		Long: `Enable an automation rule by name or ID.

Example:
  penf review auto enable auto-archive-spam`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewAutoEnable(cmd.Context(), deps, args[0])
		},
	}
}

// newReviewAutoDisableCommand creates the 'review auto disable' subcommand.
func newReviewAutoDisableCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <rule>",
		Short: "Disable an automation rule",
		Long: `Disable an automation rule by name or ID.

Example:
  penf review auto disable auto-accept-known`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewAutoDisable(cmd.Context(), deps, args[0])
		},
	}
}

// runReviewStart executes the review start command.
func runReviewStart(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	resp, err := client.StartSession(ctx, &reviewv1.StartSessionRequest{})
	if err != nil {
		return fmt.Errorf("starting session: %w", err)
	}

	session := protoSessionToLocal(resp.Session)

	fmt.Println("Review session started.")
	fmt.Printf("  Session ID: %s\n", session.ID)
	fmt.Printf("  Started at: %s\n", session.StartedAt.Format(time.RFC3339))
	if resp.PreviousSessionEnded {
		fmt.Println("  (Previous session was automatically ended)")
	}
	fmt.Println("\nUse 'penf review queue' to see pending items.")

	return nil
}

// runReviewPause executes the review pause command.
func runReviewPause(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	resp, err := client.PauseSession(ctx, &reviewv1.PauseSessionRequest{})
	if err != nil {
		return fmt.Errorf("pausing session: %w", err)
	}

	session := protoSessionToLocal(resp.Session)

	fmt.Println("Review session paused.")
	fmt.Printf("  Session ID: %s\n", session.ID)
	fmt.Printf("  Items reviewed: %d\n", session.TotalReviewed)
	fmt.Println("\nUse 'penf review resume' to continue.")

	return nil
}

// runReviewResume executes the review resume command.
func runReviewResume(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	resp, err := client.ResumeSession(ctx, &reviewv1.ResumeSessionRequest{})
	if err != nil {
		return fmt.Errorf("resuming session: %w", err)
	}

	session := protoSessionToLocal(resp.Session)

	fmt.Println("Review session resumed.")
	fmt.Printf("  Session ID: %s\n", session.ID)
	fmt.Printf("  Items reviewed so far: %d\n", session.TotalReviewed)
	fmt.Println("\nUse 'penf review queue' to see pending items.")

	return nil
}

// runReviewEnd executes the review end command.
func runReviewEnd(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	resp, err := client.EndSession(ctx, &reviewv1.EndSessionRequest{})
	if err != nil {
		return fmt.Errorf("ending session: %w", err)
	}

	session := protoSessionToLocal(resp.Session)

	duration := time.Duration(resp.Session.ActiveDurationSeconds) * time.Second

	fmt.Println("Review session ended.")
	fmt.Println()
	fmt.Println("Session Summary:")
	fmt.Printf("  Session ID:     %s\n", session.ID)
	fmt.Printf("  Duration:       %s\n", formatReviewDuration(duration))
	fmt.Printf("  Total reviewed: %d\n", session.TotalReviewed)
	fmt.Println()
	fmt.Println("Decisions:")
	fmt.Printf("  Accepted: \033[32m%d\033[0m\n", session.Accepted)
	fmt.Printf("  Rejected: \033[31m%d\033[0m\n", session.Rejected)
	fmt.Printf("  Deferred: \033[33m%d\033[0m\n", session.Deferred)

	return nil
}

// runReviewQueue executes the review queue command.
func runReviewQueue(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Validate priority filter if provided.
	if reviewPriority != "" {
		priority := ReviewPriority(reviewPriority)
		if priority != ReviewPriorityHigh && priority != ReviewPriorityMedium && priority != ReviewPriorityLow {
			return fmt.Errorf("invalid priority: %s (must be high, medium, or low)", reviewPriority)
		}
	}

	// Get output format.
	outputFormat := cfg.OutputFormat
	if reviewOutput != "" {
		outputFormat = config.OutputFormat(reviewOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s (must be text, json, or yaml)", reviewOutput)
		}
	}

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())
	tenantID := getTenantID()

	req := &reviewv1.ListReviewItemsRequest{
		TenantId: &tenantID,
		Statuses: []reviewv1.ReviewStatus{reviewv1.ReviewStatus_REVIEW_STATUS_PENDING},
		PageSize: 50,
	}

	// Add priority filter if specified
	if reviewPriority != "" {
		var protoPriority reviewv1.Priority
		switch ReviewPriority(reviewPriority) {
		case ReviewPriorityHigh:
			protoPriority = reviewv1.Priority_PRIORITY_HIGH
		case ReviewPriorityMedium:
			protoPriority = reviewv1.Priority_PRIORITY_MEDIUM
		case ReviewPriorityLow:
			protoPriority = reviewv1.Priority_PRIORITY_LOW
		}
		req.Priorities = []reviewv1.Priority{protoPriority}
	}

	rpcCtx, rpcCancel := context.WithTimeout(ctx, 30*time.Second)
	defer rpcCancel()
	resp, err := client.ListReviewItems(rpcCtx, req)
	if err != nil {
		return fmt.Errorf("listing review items: %w", err)
	}

	// Convert proto items to local items
	items := make([]ReviewItem, len(resp.Items))
	for i, item := range resp.Items {
		items[i] = protoItemToLocal(item)
	}

	totalCount := len(items)
	if resp.TotalCount != nil {
		totalCount = int(*resp.TotalCount)
	}

	response := ReviewQueueResponse{
		Items:      items,
		TotalCount: totalCount,
		FetchedAt:  time.Now(),
	}

	if reviewCountOnly {
		return outputReviewCount(outputFormat, response)
	}

	return outputReviewQueue(outputFormat, response)
}

// runReviewAccept executes the review accept command.
func runReviewAccept(ctx context.Context, deps *ReviewCommandDeps, itemID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	// Validate itemID is a valid ID
	if _, err := strconv.ParseInt(itemID, 10, 64); err != nil {
		return fmt.Errorf("invalid item ID: %s", itemID)
	}

	resp, err := client.ApproveItem(ctx, &reviewv1.ApproveItemRequest{
		Id:   itemID,
		Note: "Accepted via CLI",
	})
	if err != nil {
		return fmt.Errorf("accepting item: %w", err)
	}

	item := protoItemToLocal(resp.Item)

	fmt.Printf("Item accepted: %s\n", itemID)
	fmt.Printf("  Title: %s\n", item.Title)

	return nil
}

// runReviewReject executes the review reject command.
func runReviewReject(ctx context.Context, deps *ReviewCommandDeps, itemID string, reason string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	// Validate itemID is a valid ID
	if _, err := strconv.ParseInt(itemID, 10, 64); err != nil {
		return fmt.Errorf("invalid item ID: %s", itemID)
	}

	resp, err := client.RejectItem(ctx, &reviewv1.RejectItemRequest{
		Id:     itemID,
		Reason: reason,
	})
	if err != nil {
		return fmt.Errorf("rejecting item: %w", err)
	}

	item := protoItemToLocal(resp.Item)

	fmt.Printf("Item rejected: %s\n", itemID)
	fmt.Printf("  Title: %s\n", item.Title)
	if reason != "" {
		fmt.Printf("  Reason: %s\n", reason)
	}

	return nil
}

// runReviewDefer executes the review defer command.
func runReviewDefer(ctx context.Context, deps *ReviewCommandDeps, itemID string, until string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Parse the until date if provided.
	var deferredTo *time.Time
	if until != "" {
		t, err := parseDeferDate(until)
		if err != nil {
			return fmt.Errorf("invalid --until date: %w", err)
		}
		deferredTo = &t
	}

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	// Validate itemID is a valid ID
	if _, err := strconv.ParseInt(itemID, 10, 64); err != nil {
		return fmt.Errorf("invalid item ID: %s", itemID)
	}

	// Note: The proto doesn't have a DeferItem RPC, so we use RejectItem with a deferral reason
	// A proper implementation would add a DeferItem RPC to the proto
	resp, err := client.RejectItem(ctx, &reviewv1.RejectItemRequest{
		Id:     itemID,
		Reason: "Deferred via CLI",
	})
	if err != nil {
		return fmt.Errorf("deferring item: %w", err)
	}

	item := protoItemToLocal(resp.Item)

	fmt.Printf("Item deferred: %s\n", itemID)
	fmt.Printf("  Title: %s\n", item.Title)
	if deferredTo != nil {
		fmt.Printf("  Deferred until: %s\n", deferredTo.Format("2006-01-02"))
	}

	return nil
}

// runReviewShow executes the review show command.
func runReviewShow(ctx context.Context, deps *ReviewCommandDeps, itemID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Get output format.
	outputFormat := cfg.OutputFormat
	if reviewOutput != "" {
		outputFormat = config.OutputFormat(reviewOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s (must be text, json, or yaml)", reviewOutput)
		}
	}

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	resp, err := client.GetReviewItem(ctx, &reviewv1.GetReviewItemRequest{
		Id: itemID,
	})
	if err != nil {
		return fmt.Errorf("getting review item: %w", err)
	}

	item := protoItemToLocal(resp.Item)

	return outputReviewItem(outputFormat, &item)
}

// runReviewUndo executes the review undo command.
func runReviewUndo(ctx context.Context, deps *ReviewCommandDeps, itemID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	if itemID == "" {
		fmt.Println("Please specify an item ID to undo.")
		fmt.Println("Usage: penf review undo <item-id>")
		return nil
	}

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	resp, err := client.UndoAction(ctx, &reviewv1.UndoActionRequest{
		Id: itemID,
	})
	if err != nil {
		return fmt.Errorf("undoing action: %w", err)
	}

	if resp.UndoneAction == nil {
		fmt.Println("Nothing to undo for this item.")
		return nil
	}

	fmt.Printf("Undone: %s on item %s\n", resp.UndoneAction.ActionType.String(), itemID)
	fmt.Printf("  Status reverted from %s to %s\n",
		resp.UndoneAction.NewStatus.String(),
		resp.UndoneAction.PreviousStatus.String())

	if resp.CanUndoMore {
		fmt.Println("  (More actions available to undo)")
	}

	return nil
}

// runReviewRedo executes the review redo command.
func runReviewRedo(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// TODO: No redo RPC exists in the review service yet.
	// The backend would need to track undone actions and provide a RedoAction RPC.
	// For now, inform the user that redo is not yet implemented.
	fmt.Println("Redo functionality is not yet implemented in the review service.")
	fmt.Println("To re-apply an action, use the original command (accept/reject/defer) again.")

	return nil
}

// runReviewHistory executes the review history command.
func runReviewHistory(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Get output format.
	outputFormat := cfg.OutputFormat
	if reviewOutput != "" {
		outputFormat = config.OutputFormat(reviewOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s (must be text, json, or yaml)", reviewOutput)
		}
	}

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	resp, err := client.GetSessionHistory(ctx, &reviewv1.GetSessionHistoryRequest{
		Limit: 10,
	})
	if err != nil {
		return fmt.Errorf("getting session history: %w", err)
	}

	// Convert sessions to actions for backward compatibility with output
	var actions []ReviewAction
	for _, sess := range resp.Sessions {
		localSession := protoSessionToLocal(sess)
		action := ReviewAction{
			ID:        localSession.ID,
			ItemID:    localSession.ID,
			Action:    string(localSession.Status),
			Timestamp: localSession.StartedAt,
		}
		actions = append(actions, action)
	}

	return outputReviewHistory(outputFormat, actions)
}

// runReviewAutoStatus executes the review auto status command.
func runReviewAutoStatus(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Get output format.
	outputFormat := cfg.OutputFormat
	if reviewOutput != "" {
		outputFormat = config.OutputFormat(reviewOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s (must be text, json, or yaml)", reviewOutput)
		}
	}

	// TODO: No automation rules backend exists yet.
	// When implemented, this should call a ListAutoRules RPC.
	// For now, return an empty list.
	var rules []ReviewAutoRule

	return outputReviewAutoRules(outputFormat, rules)
}

// runReviewAutoEnable executes the review auto enable command.
func runReviewAutoEnable(ctx context.Context, deps *ReviewCommandDeps, ruleName string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// TODO: No automation rules backend exists yet.
	// When implemented, this should call an EnableAutoRule RPC.
	_ = ruleName // Suppress unused variable warning
	fmt.Println("Automation rules are not yet implemented in the review service.")
	fmt.Println("This feature will be available in a future release.")

	return nil
}

// runReviewAutoDisable executes the review auto disable command.
func runReviewAutoDisable(ctx context.Context, deps *ReviewCommandDeps, ruleName string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// TODO: No automation rules backend exists yet.
	// When implemented, this should call a DisableAutoRule RPC.
	_ = ruleName // Suppress unused variable warning
	fmt.Println("Automation rules are not yet implemented in the review service.")
	fmt.Println("This feature will be available in a future release.")

	return nil
}

// protoSessionToLocal converts a proto ReviewSession to the local ReviewSession type.
func protoSessionToLocal(s *reviewv1.ReviewSession) *ReviewSession {
	if s == nil {
		return nil
	}

	session := &ReviewSession{
		ID:            s.Id,
		TotalReviewed: int(s.TotalReviewed),
		Accepted:      int(s.ApprovedCount),
		Rejected:      int(s.RejectedCount),
		Deferred:      int(s.DeferredCount),
	}

	switch s.Status {
	case reviewv1.SessionStatus_SESSION_STATUS_ACTIVE:
		session.Status = ReviewSessionStatusActive
	case reviewv1.SessionStatus_SESSION_STATUS_PAUSED:
		session.Status = ReviewSessionStatusPaused
	case reviewv1.SessionStatus_SESSION_STATUS_ENDED:
		session.Status = ReviewSessionStatusEnded
	}

	if s.StartedAt != nil {
		session.StartedAt = s.StartedAt.AsTime()
	}
	if s.PausedAt != nil {
		pausedAt := s.PausedAt.AsTime()
		session.PausedAt = &pausedAt
	}
	if s.EndedAt != nil {
		endedAt := s.EndedAt.AsTime()
		session.EndedAt = &endedAt
	}

	return session
}

// protoItemToLocal converts a proto ReviewItem to the local ReviewItem type.
func protoItemToLocal(item *reviewv1.ReviewItem) ReviewItem {
	result := ReviewItem{
		ID:          item.Id,
		Title:       item.ContentSummary,
		ContentType: item.ContentType,
		Source:      item.Source,
		Summary:     item.ContentSummary,
		Status:      ReviewItemStatusPending,
	}

	switch item.Priority {
	case reviewv1.Priority_PRIORITY_HIGH, reviewv1.Priority_PRIORITY_URGENT:
		result.Priority = ReviewPriorityHigh
	case reviewv1.Priority_PRIORITY_MEDIUM:
		result.Priority = ReviewPriorityMedium
	case reviewv1.Priority_PRIORITY_LOW:
		result.Priority = ReviewPriorityLow
	}

	switch item.Status {
	case reviewv1.ReviewStatus_REVIEW_STATUS_PENDING:
		result.Status = ReviewItemStatusPending
	case reviewv1.ReviewStatus_REVIEW_STATUS_APPROVED:
		result.Status = ReviewItemStatusAccepted
	case reviewv1.ReviewStatus_REVIEW_STATUS_REJECTED:
		result.Status = ReviewItemStatusRejected
	case reviewv1.ReviewStatus_REVIEW_STATUS_DEFERRED:
		result.Status = ReviewItemStatusDeferred
	}

	if item.CreatedAt != nil {
		result.CreatedAt = item.CreatedAt.AsTime()
	}

	return result
}

// Output formatting functions.

// outputReviewCount outputs just the count of review items.
func outputReviewCount(format config.OutputFormat, response ReviewQueueResponse) error {
	switch format {
	case config.OutputFormatJSON:
		output := map[string]interface{}{
			"count":      response.TotalCount,
			"fetched_at": response.FetchedAt,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	case config.OutputFormatYAML:
		output := map[string]interface{}{
			"count":      response.TotalCount,
			"fetched_at": response.FetchedAt,
		}
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(output)
	default:
		fmt.Printf("Pending review items: %d\n", response.TotalCount)
		return nil
	}
}

// outputReviewQueue outputs the review queue.
func outputReviewQueue(format config.OutputFormat, response ReviewQueueResponse) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(response)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(response)
	default:
		return outputReviewQueueText(response)
	}
}

// outputReviewQueueText outputs the review queue in human-readable format.
func outputReviewQueueText(response ReviewQueueResponse) error {
	if len(response.Items) == 0 {
		fmt.Println("No items pending review.")
		return nil
	}

	fmt.Printf("Review Queue (%d items):\n\n", response.TotalCount)
	fmt.Println("  PRIORITY   ID          TITLE                                TYPE      SOURCE    AGE")
	fmt.Println("  --------   --          -----                                ----      ------    ---")

	for _, item := range response.Items {
		priorityColor := getReviewPriorityColor(item.Priority)
		age := formatRelativeTime(item.CreatedAt)

		fmt.Printf("  %s%-8s\033[0m   %-10s  %-35s  %-8s  %-8s  %s\n",
			priorityColor,
			item.Priority,
			truncateString(item.ID, 10),
			truncateString(item.Title, 35),
			item.ContentType,
			item.Source,
			age)
	}

	fmt.Println()
	return nil
}

// outputReviewItem outputs a single review item.
func outputReviewItem(format config.OutputFormat, item *ReviewItem) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(item)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(item)
	default:
		return outputReviewItemText(item)
	}
}

// outputReviewItemText outputs a review item in human-readable format.
func outputReviewItemText(item *ReviewItem) error {
	priorityColor := getReviewPriorityColor(item.Priority)
	statusColor := getReviewStatusColor(item.Status)

	fmt.Println("Review Item Details:")
	fmt.Println()
	fmt.Printf("  ID:           %s\n", item.ID)
	fmt.Printf("  Title:        %s\n", item.Title)
	fmt.Printf("  Content Type: %s\n", item.ContentType)
	fmt.Printf("  Source:       %s\n", item.Source)
	fmt.Printf("  Priority:     %s%s\033[0m\n", priorityColor, item.Priority)
	fmt.Printf("  Status:       %s%s\033[0m\n", statusColor, item.Status)
	fmt.Printf("  Created:      %s (%s)\n", item.CreatedAt.Format(time.RFC3339), formatRelativeTime(item.CreatedAt))

	if item.Summary != "" {
		fmt.Printf("  Summary:      %s\n", item.Summary)
	}

	if item.DeferredTo != nil {
		fmt.Printf("  Deferred To:  %s\n", item.DeferredTo.Format("2006-01-02"))
	}

	if item.Reason != "" {
		fmt.Printf("  Reason:       %s\n", item.Reason)
	}

	return nil
}

// outputReviewHistory outputs the action history.
func outputReviewHistory(format config.OutputFormat, actions []ReviewAction) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(actions)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(actions)
	default:
		return outputReviewHistoryText(actions)
	}
}

// outputReviewHistoryText outputs action history in human-readable format.
func outputReviewHistoryText(actions []ReviewAction) error {
	if len(actions) == 0 {
		fmt.Println("No actions in history.")
		return nil
	}

	fmt.Printf("Review Action History (%d actions):\n\n", len(actions))
	fmt.Println("  TIME          ACTION    ITEM        STATUS CHANGE          UNDONE")
	fmt.Println("  ----          ------    ----        -------------          ------")

	for _, action := range actions {
		timeStr := formatRelativeTime(action.Timestamp)
		undoneStr := " "
		if action.Undone {
			undoneStr = "Y"
		}

		fmt.Printf("  %-12s  %-8s  %-10s  %-8s -> %-8s  %s\n",
			timeStr,
			action.Action,
			truncateString(action.ItemID, 10),
			action.OldStatus,
			action.NewStatus,
			undoneStr)
	}

	fmt.Println()
	return nil
}

// outputReviewAutoRules outputs automation rules.
func outputReviewAutoRules(format config.OutputFormat, rules []ReviewAutoRule) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rules)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(rules)
	default:
		return outputReviewAutoRulesText(rules)
	}
}

// outputReviewAutoRulesText outputs automation rules in human-readable format.
func outputReviewAutoRulesText(rules []ReviewAutoRule) error {
	if len(rules) == 0 {
		fmt.Println("No automation rules configured.")
		return nil
	}

	fmt.Printf("Automation Rules (%d rules):\n\n", len(rules))
	fmt.Println("  STATUS    NAME                    ACTION    DESCRIPTION")
	fmt.Println("  ------    ----                    ------    -----------")

	for _, rule := range rules {
		statusStr := "\033[31mdisabled\033[0m"
		if rule.Enabled {
			statusStr = "\033[32menabled \033[0m"
		}

		fmt.Printf("  %s  %-22s  %-8s  %s\n",
			statusStr,
			truncateString(rule.Name, 22),
			rule.Action,
			truncateString(rule.Description, 40))
	}

	fmt.Println()
	return nil
}

// Helper functions.

// getReviewPriorityColor returns the ANSI color for a review priority level.
func getReviewPriorityColor(priority ReviewPriority) string {
	switch priority {
	case ReviewPriorityHigh:
		return "\033[31m" // Red
	case ReviewPriorityMedium:
		return "\033[33m" // Yellow
	case ReviewPriorityLow:
		return "\033[32m" // Green
	default:
		return ""
	}
}

// getReviewStatusColor returns the ANSI color for a review item status.
func getReviewStatusColor(status ReviewItemStatus) string {
	switch status {
	case ReviewItemStatusPending:
		return "\033[33m" // Yellow
	case ReviewItemStatusAccepted:
		return "\033[32m" // Green
	case ReviewItemStatusRejected:
		return "\033[31m" // Red
	case ReviewItemStatusDeferred:
		return "\033[36m" // Cyan
	default:
		return ""
	}
}

// parseDeferDate parses a defer date string.
func parseDeferDate(dateStr string) (time.Time, error) {
	lower := strings.ToLower(dateStr)
	now := time.Now()

	switch lower {
	case "tomorrow":
		tomorrow := now.AddDate(0, 0, 1)
		return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 9, 0, 0, 0, now.Location()), nil
	case "nextweek", "next_week":
		nextWeek := now.AddDate(0, 0, 7)
		return time.Date(nextWeek.Year(), nextWeek.Month(), nextWeek.Day(), 9, 0, 0, 0, now.Location()), nil
	case "nextmonth", "next_month":
		nextMonth := now.AddDate(0, 1, 0)
		return time.Date(nextMonth.Year(), nextMonth.Month(), nextMonth.Day(), 9, 0, 0, 0, now.Location()), nil
	}

	// Try standard date formats.
	dateLayouts := []string{
		"2006-01-02",
		"2006/01/02",
		"01-02-2006",
		"01/02/2006",
		"Jan 2, 2006",
		"January 2, 2006",
	}

	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// formatReviewDuration formats a duration in a human-readable way for review sessions.
func formatReviewDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs > 0 {
			return fmt.Sprintf("%d min %d sec", mins, secs)
		}
		return fmt.Sprintf("%d min", mins)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins > 0 {
		return fmt.Sprintf("%d hr %d min", hours, mins)
	}
	return fmt.Sprintf("%d hr", hours)
}

// ValidateReviewPriority validates a review priority string.
func ValidateReviewPriority(priority string) bool {
	switch ReviewPriority(priority) {
	case ReviewPriorityHigh, ReviewPriorityMedium, ReviewPriorityLow:
		return true
	default:
		return false
	}
}

// ValidateReviewItemStatus validates a review item status string.
func ValidateReviewItemStatus(status string) bool {
	switch ReviewItemStatus(status) {
	case ReviewItemStatusPending, ReviewItemStatusAccepted, ReviewItemStatusRejected, ReviewItemStatusDeferred:
		return true
	default:
		return false
	}
}

// newReviewDailyCommand creates the 'review daily' subcommand.
func newReviewDailyCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daily",
		Short: "Get today's review items grouped by category",
		Long: `Get today's review items grouped by category.

Shows all pending review items organized by category (communications, tasks, etc.).
Optionally filter by date or include already processed items.

Examples:
  penf review daily
  penf review daily --date 2026-02-05
  penf review daily --include-processed
  penf review daily -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewDaily(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&reviewDate, "date", "d", "", "Date in YYYY-MM-DD format (defaults to today)")
	cmd.Flags().BoolVarP(&reviewIncludeProcessed, "include-processed", "i", false, "Include already processed items")
	cmd.Flags().StringVarP(&reviewOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newReviewSessionCommand creates the 'review session' subcommand.
func newReviewSessionCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Show current review session status",
		Long: `Show current review session status.

Displays the current active or paused review session with statistics on
items reviewed, accepted, rejected, and deferred. Shows session duration.

Examples:
  penf review session
  penf review session -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewSession(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&reviewOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newReviewStatsCommand creates the 'review stats' subcommand.
func newReviewStatsCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show review statistics and metrics",
		Long: `Show review statistics and metrics.

Provides insights into review queue health, processing velocity, and
breakdowns by priority, content type, source, and category.

Examples:
  penf review stats
  penf review stats --from 2026-01-01 --to 2026-01-31
  penf review stats -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReviewStats(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&reviewFrom, "from", "f", "", "Start date in YYYY-MM-DD format")
	cmd.Flags().StringVarP(&reviewTo, "to", "t", "", "End date in YYYY-MM-DD format")
	cmd.Flags().StringVarP(&reviewOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// runReviewDaily executes the review daily command.
func runReviewDaily(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Get output format.
	outputFormat := cfg.OutputFormat
	if reviewOutput != "" {
		outputFormat = config.OutputFormat(reviewOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s (must be text, json, or yaml)", reviewOutput)
		}
	}

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	req := &reviewv1.GetDailyReviewRequest{
		IncludeProcessed: reviewIncludeProcessed,
	}

	if reviewDate != "" {
		req.Date = &reviewDate
	}

	resp, err := client.GetDailyReview(ctx, req)
	if err != nil {
		return fmt.Errorf("getting daily review: %w", err)
	}

	return outputDailyReview(outputFormat, resp.Review)
}

// runReviewSession executes the review session command.
func runReviewSession(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Get output format.
	outputFormat := cfg.OutputFormat
	if reviewOutput != "" {
		outputFormat = config.OutputFormat(reviewOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s (must be text, json, or yaml)", reviewOutput)
		}
	}

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	resp, err := client.GetCurrentSession(ctx, &reviewv1.GetCurrentSessionRequest{})
	if err != nil {
		return fmt.Errorf("getting current session: %w", err)
	}

	if !resp.HasSession {
		fmt.Println("No active review session.")
		fmt.Println("Use 'penf review start' to begin a new session.")
		return nil
	}

	return outputCurrentSession(outputFormat, resp.Session)
}

// runReviewStats executes the review stats command.
func runReviewStats(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Get output format.
	outputFormat := cfg.OutputFormat
	if reviewOutput != "" {
		outputFormat = config.OutputFormat(reviewOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s (must be text, json, or yaml)", reviewOutput)
		}
	}

	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	client := reviewv1.NewReviewServiceClient(grpcClient.GetConnection())

	req := &reviewv1.GetReviewStatsRequest{}

	// Parse date filters if provided.
	if reviewFrom != "" {
		t, err := time.Parse("2006-01-02", reviewFrom)
		if err != nil {
			return fmt.Errorf("invalid --from date: %w", err)
		}
		req.FromTime = timestamppb.New(t)
	}

	if reviewTo != "" {
		t, err := time.Parse("2006-01-02", reviewTo)
		if err != nil {
			return fmt.Errorf("invalid --to date: %w", err)
		}
		req.ToTime = timestamppb.New(t)
	}

	resp, err := client.GetReviewStats(ctx, req)
	if err != nil {
		return fmt.Errorf("getting review stats: %w", err)
	}

	return outputReviewStats(outputFormat, resp)
}

// outputDailyReview outputs the daily review.
func outputDailyReview(format config.OutputFormat, review *reviewv1.DailyReview) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(review)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(review)
	default:
		return outputDailyReviewText(review)
	}
}

// outputDailyReviewText outputs the daily review in human-readable format.
func outputDailyReviewText(review *reviewv1.DailyReview) error {
	if review == nil {
		fmt.Println("No review data available.")
		return nil
	}

	fmt.Printf("Daily Review for %s\n\n", review.Date)
	fmt.Printf("Total Pending: %d\n", review.TotalPending)
	fmt.Printf("High Priority: %d\n", review.HighPriorityCount)
	fmt.Printf("Processed Today: %d\n\n", review.ProcessedToday)

	if len(review.Categories) == 0 {
		fmt.Println("No items to review.")
		return nil
	}

	for _, category := range review.Categories {
		fmt.Printf("  %s (%d items):\n", category.DisplayName, category.ItemCount)
		if len(category.Items) == 0 {
			fmt.Println("    (no items)")
		} else {
			for _, item := range category.Items {
				priorityColor := ""
				switch item.Priority {
				case reviewv1.Priority_PRIORITY_HIGH, reviewv1.Priority_PRIORITY_URGENT:
					priorityColor = "\033[31m"
				case reviewv1.Priority_PRIORITY_MEDIUM:
					priorityColor = "\033[33m"
				case reviewv1.Priority_PRIORITY_LOW:
					priorityColor = "\033[32m"
				}

				fmt.Printf("    %s[%s]\033[0m %s - %s\n",
					priorityColor,
					item.Priority.String(),
					truncateString(item.Id, 10),
					truncateString(item.ContentSummary, 60))
			}
		}
		fmt.Println()
	}

	return nil
}

// outputCurrentSession outputs the current session.
func outputCurrentSession(format config.OutputFormat, session *reviewv1.ReviewSession) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(session)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(session)
	default:
		return outputCurrentSessionText(session)
	}
}

// outputCurrentSessionText outputs the current session in human-readable format.
func outputCurrentSessionText(session *reviewv1.ReviewSession) error {
	if session == nil {
		fmt.Println("No session data available.")
		return nil
	}

	fmt.Println("Current Review Session:")
	fmt.Println()
	fmt.Printf("  Session ID:     %s\n", session.Id)
	fmt.Printf("  Status:         %s\n", session.Status.String())
	fmt.Printf("  Started:        %s\n", session.StartedAt.AsTime().Format(time.RFC3339))

	if session.PausedAt != nil {
		fmt.Printf("  Paused:         %s\n", session.PausedAt.AsTime().Format(time.RFC3339))
	}

	if session.EndedAt != nil {
		fmt.Printf("  Ended:          %s\n", session.EndedAt.AsTime().Format(time.RFC3339))
	}

	duration := time.Duration(session.ActiveDurationSeconds) * time.Second
	fmt.Printf("  Duration:       %s\n", formatReviewDuration(duration))
	fmt.Println()
	fmt.Printf("  Total Reviewed: %d\n", session.TotalReviewed)
	fmt.Printf("  Approved:       \033[32m%d\033[0m\n", session.ApprovedCount)
	fmt.Printf("  Rejected:       \033[31m%d\033[0m\n", session.RejectedCount)
	fmt.Printf("  Deferred:       \033[33m%d\033[0m\n", session.DeferredCount)

	return nil
}

// outputReviewStats outputs the review statistics.
func outputReviewStats(format config.OutputFormat, stats *reviewv1.GetReviewStatsResponse) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(stats)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(stats)
	default:
		return outputReviewStatsText(stats)
	}
}

// outputReviewStatsText outputs review statistics in human-readable format.
func outputReviewStatsText(stats *reviewv1.GetReviewStatsResponse) error {
	if stats == nil {
		fmt.Println("No statistics available.")
		return nil
	}

	fmt.Println("Review Statistics:")
	fmt.Println()
	fmt.Println("Overall Counts:")
	fmt.Printf("  Pending:   %d\n", stats.PendingCount)
	fmt.Printf("  Approved:  \033[32m%d\033[0m\n", stats.ApprovedCount)
	fmt.Printf("  Rejected:  \033[31m%d\033[0m\n", stats.RejectedCount)
	fmt.Printf("  Deferred:  \033[33m%d\033[0m\n", stats.DeferredCount)
	fmt.Println()

	if stats.AvgReviewTimeSeconds > 0 {
		avgDuration := time.Duration(stats.AvgReviewTimeSeconds) * time.Second
		fmt.Printf("Average Review Time: %s\n\n", formatReviewDuration(avgDuration))
	}

	if len(stats.ByPriority) > 0 {
		fmt.Println("By Priority:")
		for priority, count := range stats.ByPriority {
			fmt.Printf("  %-8s: %d\n", priority, count)
		}
		fmt.Println()
	}

	if len(stats.ByContentType) > 0 {
		fmt.Println("By Content Type:")
		for contentType, count := range stats.ByContentType {
			fmt.Printf("  %-12s: %d\n", contentType, count)
		}
		fmt.Println()
	}

	if len(stats.BySource) > 0 {
		fmt.Println("By Source:")
		for source, count := range stats.BySource {
			fmt.Printf("  %-12s: %d\n", source, count)
		}
		fmt.Println()
	}

	if len(stats.ByCategory) > 0 {
		fmt.Println("By Category:")
		for category, count := range stats.ByCategory {
			fmt.Printf("  %-20s: %d\n", category, count)
		}
		fmt.Println()
	}

	if len(stats.DailyCounts) > 0 {
		fmt.Println("Daily Activity:")
		fmt.Println("  DATE         CREATED  REVIEWED  APPROVED  REJECTED")
		fmt.Println("  ----------   -------  --------  --------  --------")
		for _, daily := range stats.DailyCounts {
			fmt.Printf("  %-10s   %-7d  %-8d  %-8d  %-8d\n",
				daily.Date,
				daily.Created,
				daily.Reviewed,
				daily.Approved,
				daily.Rejected)
		}
		fmt.Println()
	}

	return nil
}
