// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"

	watchlistv1 "github.com/otherjamesbrown/penf-cli/api/proto/watchlist/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Watch command flags.
var (
	watchTenant      string
	watchOutput      string
	watchUserID      string
	watchProjectID   int64
	watchAssertionID int64
	watchNotes       string
)

// WatchCommandDeps holds the dependencies for watch commands.
type WatchCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultWatchDeps returns the default dependencies for production use.
func DefaultWatchDeps() *WatchCommandDeps {
	return &WatchCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewWatchCommand creates the root watch command with all subcommands.
func NewWatchCommand(deps *WatchCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultWatchDeps()
	}

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Manage watch list items",
		Long: `Manage watch list items for tracking important assertions and projects.

Watch list allows you to mark specific assertions or projects for closer monitoring.
Items on your watch list are prioritized in briefings and notifications.

Use Cases:
  - Track critical risks or decisions
  - Monitor important projects
  - Get notified of changes to watched items
  - Prioritize content in briefings

Examples:
  # List all watched items
  penf watch list

  # Watch an assertion
  penf watch add --assertion 123 --notes "Critical timeline risk"

  # Watch a project
  penf watch add --project 456 --notes "MTC migration tracking"

  # Update notes on a watched item
  penf watch annotate 789 --notes "Updated: risk mitigated"

  # Remove a watched item
  penf watch remove 789

  # Output as JSON for programmatic use
  penf watch list --output json

Related Commands:
  penf project      Manage projects
  penf search       Search for assertions`,
		Aliases: []string{"watchlist"},
	}

	// Add persistent flags.
	cmd.PersistentFlags().StringVarP(&watchTenant, "tenant", "t", "", "Tenant ID (overrides config)")
	cmd.PersistentFlags().StringVarP(&watchOutput, "output", "o", "", "Output format: text, json, yaml")

	// Add subcommands.
	cmd.AddCommand(newWatchListCommand(deps))
	cmd.AddCommand(newWatchAddCommand(deps))
	cmd.AddCommand(newWatchRemoveCommand(deps))
	cmd.AddCommand(newWatchAnnotateCommand(deps))

	return cmd
}

// newWatchListCommand creates the 'watch list' subcommand.
func newWatchListCommand(deps *WatchCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List watched items",
		Long: `List all items on your watch list.

Shows assertions and projects you are actively monitoring.
Use --project to filter by a specific project.

Examples:
  # List all watched items
  penf watch list

  # Filter by project
  penf watch list --project 5

  # List as JSON for programmatic use
  penf watch list --output json

  # List as YAML
  penf watch list --output yaml`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatchList(cmd.Context(), deps)
		},
	}

	cmd.Flags().Int64Var(&watchProjectID, "project", 0, "Filter by project ID")
	cmd.Flags().StringVar(&watchUserID, "user", "default", "User ID (default: 'default')")

	return cmd
}

// newWatchAddCommand creates the 'watch add' subcommand.
func newWatchAddCommand(deps *WatchCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an item to your watch list",
		Long: `Add an assertion or project to your watch list for closer monitoring.

You must specify exactly one of --assertion or --project.
Optionally add notes explaining why you're watching this item.

Examples:
  # Watch an assertion
  penf watch add --assertion 123 --notes "Critical timeline risk"

  # Watch a project
  penf watch add --project 456 --notes "MTC migration tracking"

  # Watch without notes
  penf watch add --assertion 789`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatchAdd(cmd.Context(), deps)
		},
	}

	cmd.Flags().Int64Var(&watchAssertionID, "assertion", 0, "Assertion root ID to watch")
	cmd.Flags().Int64Var(&watchProjectID, "project", 0, "Project ID to watch")
	cmd.Flags().StringVar(&watchNotes, "notes", "", "Notes explaining why you're watching this")
	cmd.Flags().StringVar(&watchUserID, "user", "default", "User ID (default: 'default')")

	return cmd
}

// newWatchRemoveCommand creates the 'watch remove' subcommand.
func newWatchRemoveCommand(deps *WatchCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove an item from your watch list",
		Long: `Remove an item from your watch list by its watch item ID.

Use 'watch list' to find the ID of items you want to remove.

Examples:
  # Remove watch item by ID
  penf watch remove 789`,
		Aliases: []string{"rm", "delete"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatchRemove(cmd.Context(), deps, args[0])
		},
	}
}

// newWatchAnnotateCommand creates the 'watch annotate' subcommand.
func newWatchAnnotateCommand(deps *WatchCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "annotate <id>",
		Short: "Update notes on a watched item",
		Long: `Update the notes on a watched item to explain why you're monitoring it.

Examples:
  # Update notes
  penf watch annotate 789 --notes "Risk now mitigated, monitoring for updates"

  # Clear notes
  penf watch annotate 789 --notes ""`,
		Aliases: []string{"update", "note"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatchAnnotate(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVar(&watchNotes, "notes", "", "Notes for this watch item (required)")
	cmd.MarkFlagRequired("notes")

	return cmd
}

// ==================== gRPC Connection ====================

// connectWatchToGateway creates a gRPC connection to the gateway service.
func connectWatchToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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

// getTenantIDForWatch returns the tenant ID from flag, env, or config.
func getTenantIDForWatch(deps *WatchCommandDeps) string {
	if watchTenant != "" {
		return watchTenant
	}
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

// runWatchList executes the watch list command via gRPC.
func runWatchList(ctx context.Context, deps *WatchCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectWatchToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := watchlistv1.NewWatchListServiceClient(conn)
	tenantID := getTenantIDForWatch(deps)

	req := &watchlistv1.ListWatchItemsRequest{
		TenantId: tenantID,
		UserId:   watchUserID,
	}

	// Add optional project filter
	if watchProjectID > 0 {
		req.ProjectId = &watchProjectID
	}

	resp, err := client.ListWatchItems(ctx, req)
	if err != nil {
		return fmt.Errorf("listing watch items: %w", err)
	}

	return outputWatchItemsProto(cfg, resp.Items)
}

// runWatchAdd executes the watch add command via gRPC.
func runWatchAdd(ctx context.Context, deps *WatchCommandDeps) error {
	// Validate that exactly one target is specified
	if watchAssertionID == 0 && watchProjectID == 0 {
		return fmt.Errorf("must specify either --assertion or --project")
	}
	if watchAssertionID != 0 && watchProjectID != 0 {
		return fmt.Errorf("cannot specify both --assertion and --project")
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectWatchToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := watchlistv1.NewWatchListServiceClient(conn)
	tenantID := getTenantIDForWatch(deps)

	req := &watchlistv1.AddWatchItemRequest{
		TenantId: tenantID,
		UserId:   watchUserID,
		Notes:    watchNotes,
	}

	if watchAssertionID != 0 {
		req.AssertionRootId = &watchAssertionID
	}
	if watchProjectID != 0 {
		req.ProjectId = &watchProjectID
	}

	resp, err := client.AddWatchItem(ctx, req)
	if err != nil {
		return fmt.Errorf("adding watch item: %w", err)
	}

	fmt.Printf("\033[32mAdded watch item:\033[0m ID %d\n", resp.Item.Id)
	if resp.Item.AssertionDescription != "" {
		fmt.Printf("  Assertion: %s\n", resp.Item.AssertionDescription)
	}
	if resp.Item.ProjectName != "" {
		fmt.Printf("  Project: %s\n", resp.Item.ProjectName)
	}
	if watchNotes != "" {
		fmt.Printf("  Notes: %s\n", watchNotes)
	}

	// Reset flags for next call
	watchAssertionID = 0
	watchProjectID = 0
	watchNotes = ""

	return nil
}

// runWatchRemove executes the watch remove command via gRPC.
func runWatchRemove(ctx context.Context, deps *WatchCommandDeps, idStr string) error {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid watch item ID: %s", idStr)
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectWatchToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := watchlistv1.NewWatchListServiceClient(conn)
	tenantID := getTenantIDForWatch(deps)

	_, err = client.RemoveWatchItem(ctx, &watchlistv1.RemoveWatchItemRequest{
		TenantId: tenantID,
		Id:       id,
	})
	if err != nil {
		return fmt.Errorf("removing watch item: %w", err)
	}

	fmt.Printf("\033[32mRemoved watch item:\033[0m ID %d\n", id)
	return nil
}

// runWatchAnnotate executes the watch annotate command via gRPC.
func runWatchAnnotate(ctx context.Context, deps *WatchCommandDeps, idStr string) error {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid watch item ID: %s", idStr)
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectWatchToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := watchlistv1.NewWatchListServiceClient(conn)
	tenantID := getTenantIDForWatch(deps)

	resp, err := client.UpdateWatchItem(ctx, &watchlistv1.UpdateWatchItemRequest{
		TenantId: tenantID,
		Id:       id,
		Notes:    watchNotes,
	})
	if err != nil {
		return fmt.Errorf("updating watch item: %w", err)
	}

	fmt.Printf("\033[32mUpdated watch item:\033[0m ID %d\n", resp.Item.Id)
	fmt.Printf("  Notes: %s\n", watchNotes)

	// Reset flag for next call
	watchNotes = ""

	return nil
}

// ==================== Output Functions ====================

// getWatchOutputFormat returns the output format from flag or config.
func getWatchOutputFormat(cfg *config.CLIConfig) config.OutputFormat {
	if watchOutput != "" {
		return config.OutputFormat(watchOutput)
	}
	if cfg != nil {
		return cfg.OutputFormat
	}
	return config.OutputFormatText
}

// outputWatchItemsProto outputs a list of watch items from proto messages.
func outputWatchItemsProto(cfg *config.CLIConfig, items []*watchlistv1.WatchItem) error {
	format := getWatchOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputWatchJSON(items)
	case config.OutputFormatYAML:
		return outputWatchYAML(items)
	default:
		return outputWatchItemsTableProto(items)
	}
}

// outputWatchItemsTableProto outputs watch items in table format.
func outputWatchItemsTableProto(items []*watchlistv1.WatchItem) error {
	if len(items) == 0 {
		fmt.Println("No watched items found.")
		return nil
	}

	fmt.Printf("Watch List (%d items):\n\n", len(items))
	fmt.Println("  ID      TYPE         DESCRIPTION                           NOTES")
	fmt.Println("  --      ----         -----------                           -----")

	for _, item := range items {
		itemType := "Project"
		description := item.ProjectName
		if item.AssertionRootId != nil {
			itemType = "Assertion"
			description = item.AssertionDescription
		}

		notesStr := "-"
		if item.Notes != "" {
			notesStr = watchTruncateString(item.Notes, 40)
		}

		fmt.Printf("  %-6d  %-12s %-37s %s\n",
			item.Id,
			itemType,
			watchTruncateString(description, 37),
			notesStr)
	}

	fmt.Println()
	return nil
}

// Helper output functions.

func outputWatchJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputWatchYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

// watchTruncateString truncates a string to maxLen, adding "..." if truncated.
func watchTruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
