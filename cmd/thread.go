// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	threadsv1 "github.com/otherjamesbrown/penf-cli/api/proto/threads/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Thread command flags.
var (
	threadLimit  int32
	threadOffset int32
	threadOutput string
)

// ThreadCommandDeps holds the dependencies for thread commands.
type ThreadCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultThreadDeps returns the default dependencies for production use.
func DefaultThreadDeps() *ThreadCommandDeps {
	return &ThreadCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewThreadCommand creates the thread command group.
func NewThreadCommand(deps *ThreadCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultThreadDeps()
	}

	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Query email threads",
		Long: `Query email threads and messages.

Threads group related email messages by conversation, tracking participants,
subjects, and temporal sequence.

This command provides two main operations:

  list      List threads with pagination
  show      Show detailed thread view with all messages

Examples:
  penf thread list --limit 10
  penf thread show 42
  penf thread list -o json`,
	}

	cmd.AddCommand(newThreadListCommand(deps))
	cmd.AddCommand(newThreadShowCommand(deps))

	return cmd
}

// newThreadListCommand creates the 'thread list' subcommand.
func newThreadListCommand(deps *ThreadCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List threads with pagination",
		Long: `List email threads with pagination.

Threads are ordered by last message date (most recent first).

Flags:
  --limit             Maximum results (default 20)
  --offset            Pagination offset
  -o, --output        Output format: text, json, yaml

Examples:
  penf thread list --limit 10
  penf thread list --offset 20 --limit 10
  penf thread list -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThreadList(cmd.Context(), deps)
		},
	}

	cmd.Flags().Int32Var(&threadLimit, "limit", 20, "Maximum results")
	cmd.Flags().Int32Var(&threadOffset, "offset", 0, "Pagination offset")
	cmd.Flags().StringVarP(&threadOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newThreadShowCommand creates the 'thread show' subcommand.
func newThreadShowCommand(deps *ThreadCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <thread-id>",
		Short: "Show detailed thread view with all messages",
		Long: `Show a detailed view of a thread including all messages.

Messages are displayed in chronological order showing position, sender,
subject, date, and body preview.

Flags:
  -o, --output        Output format: text, json, yaml

Examples:
  penf thread show 42
  penf thread show 42 -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			threadID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid thread ID %q: %w", args[0], err)
			}
			return runThreadShow(cmd.Context(), deps, threadID)
		},
	}

	cmd.Flags().StringVarP(&threadOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// ==================== gRPC Connection ====================

// connectThreadsToGateway creates a gRPC connection to the gateway service.
func connectThreadsToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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

// getTenantIDForThreads returns the tenant ID from env or config.
func getTenantIDForThreads(deps *ThreadCommandDeps) string {
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

// runThreadList executes the thread list command.
func runThreadList(ctx context.Context, deps *ThreadCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectThreadsToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := threadsv1.NewThreadsServiceClient(conn)
	tenantID := getTenantIDForThreads(deps)

	// Build request
	req := &threadsv1.ListThreadsRequest{
		TenantId: tenantID,
		Limit:    threadLimit,
		Offset:   threadOffset,
	}

	// Execute request
	resp, err := client.ListThreads(ctx, req)
	if err != nil {
		return fmt.Errorf("listing threads: %w", err)
	}

	// Output results
	return outputThreadList(resp)
}

// runThreadShow executes the thread show command.
func runThreadShow(ctx context.Context, deps *ThreadCommandDeps, threadID int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectThreadsToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := threadsv1.NewThreadsServiceClient(conn)
	tenantID := getTenantIDForThreads(deps)

	// Build request
	req := &threadsv1.GetThreadRequest{
		TenantId: tenantID,
		ThreadId: threadID,
	}

	// Execute request
	resp, err := client.GetThread(ctx, req)
	if err != nil {
		return fmt.Errorf("getting thread: %w", err)
	}

	// Output results
	return outputThreadDetail(resp)
}

// ==================== Output Functions ====================

// outputThreadList formats and displays the thread list response.
func outputThreadList(resp *threadsv1.ListThreadsResponse) error {
	switch threadOutput {
	case "json":
		return outputJSON(resp)
	case "yaml":
		return outputYAML(resp)
	default:
		return outputThreadListText(resp)
	}
}

// outputThreadListText displays threads as a formatted table.
func outputThreadListText(resp *threadsv1.ListThreadsResponse) error {
	if len(resp.Threads) == 0 {
		fmt.Println("No threads found.")
		return nil
	}

	// Header
	fmt.Printf("%-8s %-50s %-8s %-20s %-20s\n",
		"ID", "Subject", "Messages", "First", "Last")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────────────────────────────────────────")

	// Rows
	for _, t := range resp.Threads {
		subject := truncateThreadString(t.Subject, 50)
		firstDate := formatThreadTimestamp(t.FirstMessageAt)
		lastDate := formatThreadTimestamp(t.LastMessageAt)

		fmt.Printf("%-8d %-50s %-8d %-20s %-20s\n",
			t.Id,
			subject,
			t.MessageCount,
			firstDate,
			lastDate)
	}

	// Footer
	fmt.Printf("\nShowing %d threads (Total: %d)\n", len(resp.Threads), resp.TotalCount)

	return nil
}

// outputThreadDetail formats and displays the thread detail response.
func outputThreadDetail(resp *threadsv1.GetThreadResponse) error {
	switch threadOutput {
	case "json":
		return outputJSON(resp)
	case "yaml":
		return outputYAML(resp)
	default:
		return outputThreadDetailText(resp)
	}
}

// outputThreadDetailText displays thread details as formatted text.
func outputThreadDetailText(resp *threadsv1.GetThreadResponse) error {
	// Thread header
	fmt.Printf("Thread ID:       %d\n", resp.Id)
	fmt.Printf("Subject:         %s\n", resp.Subject)
	fmt.Printf("Message Count:   %d\n", resp.MessageCount)
	fmt.Printf("First Message:   %s\n", formatThreadTimestamp(resp.FirstMessageAt))
	fmt.Printf("Last Message:    %s\n", formatThreadTimestamp(resp.LastMessageAt))
	fmt.Printf("Participants:    %v\n", resp.ParticipantIds)

	if resp.Summary != nil && *resp.Summary != "" {
		fmt.Printf("Summary:         %s\n", *resp.Summary)
	}

	// Messages
	if len(resp.Messages) > 0 {
		fmt.Println("\nMessages:")
		fmt.Println("────────────────────────────────────────────────────────────────────────────────────────")

		for _, msg := range resp.Messages {
			fmt.Printf("\n[%d] From: %s <%s>\n", msg.PositionInThread, msg.FromName, msg.FromEmail)
			fmt.Printf("    Date: %s\n", formatThreadTimestamp(msg.MessageDate))
			fmt.Printf("    Subject: %s\n", msg.Subject)
			if msg.IsReply {
				fmt.Printf("    Type: Reply\n")
			}
			if msg.BodyPreview != "" {
				preview := truncateThreadString(msg.BodyPreview, 200)
				fmt.Printf("    Preview: %s\n", preview)
			}
		}
	}

	fmt.Println()
	return nil
}

// truncateThreadString truncates a string to maxLen and adds ellipsis if needed.
func truncateThreadString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatThreadTimestamp formats a protobuf timestamp for display.
func formatThreadTimestamp(ts interface{}) string {
	if ts == nil {
		return "N/A"
	}

	// Handle *timestamppb.Timestamp
	type timestampInterface interface {
		AsTime() time.Time
	}

	if tsi, ok := ts.(timestampInterface); ok {
		t := tsi.AsTime()
		return t.Format("2006-01-02 15:04")
	}

	return "N/A"
}
