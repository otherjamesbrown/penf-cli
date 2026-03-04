package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	graphpb "github.com/otherjamesbrown/penf-cli/api/proto/connectors/v1/graphpb"
)

var graphSyncType string

// newSourceGraphCommand creates the `penf source graph` subcommand group.
func newSourceGraphCommand(deps *SourceCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Manage Microsoft Graph (Outlook, Teams) integration",
		Long: `Connect Penfold to your Microsoft 365 account via the Graph API.

Syncs your Outlook email, Teams messages, meeting transcripts, and org directory
into Penfold's knowledge base. Uses delegated auth — reads your data only.

Getting started:
  penf source graph auth      Sign in with your Microsoft account (one-time)
  penf source graph status    Check connection and sync health
  penf source graph sync      Trigger a manual sync`,
	}

	cmd.AddCommand(newSourceGraphStatusCommand(deps))
	cmd.AddCommand(newSourceGraphSyncCommand(deps))
	cmd.AddCommand(newSourceGraphChannelsCommand(deps))
	cmd.AddCommand(newSourceGraphAuthCommand(deps))

	return cmd
}

// newSourceGraphStatusCommand creates `penf source graph status`.
func newSourceGraphStatusCommand(deps *SourceCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Microsoft Graph connection and sync status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraphStatus(cmd.Context(), deps)
		},
	}
}

// newSourceGraphSyncCommand creates `penf source graph sync`.
func newSourceGraphSyncCommand(deps *SourceCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Trigger a manual Microsoft Graph sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraphSync(cmd.Context(), deps)
		},
	}
	cmd.Flags().StringVar(&graphSyncType, "type", "all", "Sync type: email, teams, transcripts, org, all")
	return cmd
}

// newSourceGraphChannelsCommand creates `penf source graph channels`.
func newSourceGraphChannelsCommand(deps *SourceCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "channels",
		Short: "List Microsoft Teams channels available for sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraphChannels(cmd.Context(), deps)
		},
	}
}

// newSourceGraphAuthCommand creates `penf source graph auth`.
func newSourceGraphAuthCommand(deps *SourceCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "Sign in to Microsoft 365 (device code flow)",
		Long: `Authenticate with your Microsoft 365 account using device code flow.

You will be shown a short code and a URL. Visit the URL in any browser,
enter the code, and sign in with your work account. Penfold stores the
token securely and uses it for all future syncs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraphAuth(cmd.Context(), deps)
		},
	}
}

// ==================== Runners ====================

func runGraphStatus(ctx context.Context, deps *SourceCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := graphpb.NewGraphConnectorServiceClient(conn)
	resp, err := client.GetGraphStatus(ctx, &graphpb.GetGraphStatusRequest{
		TenantId: getTenantID(),
	})
	if err != nil {
		return fmt.Errorf("getting graph status: %w", err)
	}

	if sourceOutput == "json" {
		return json.NewEncoder(os.Stdout).Encode(graphStatusToMap(resp))
	}

	// Auth status line
	authIcon := "✓"
	if resp.AuthStatus != "connected" {
		authIcon = "✗"
	}
	fmt.Printf("Auth:   %s %s\n", authIcon, resp.AuthStatus)
	if resp.SyncError != "" {
		fmt.Printf("Error:  %s\n", resp.SyncError)
	}
	fmt.Println()

	// Sync stats table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tSTATUS\tLAST SYNC\tITEMS")
	printSyncStats(w, "email", resp.EmailStats)
	printSyncStats(w, "teams", resp.TeamsStats)
	printSyncStats(w, "org", resp.OrgStats)
	return w.Flush()
}

func printSyncStats(w *tabwriter.Writer, name string, s *graphpb.SyncStats) {
	if s == nil {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, "—", "—", "—")
		return
	}
	lastSync := "never"
	if s.LastSyncAt != nil {
		lastSync = s.LastSyncAt.AsTime().Local().Format(time.DateTime)
	}
	fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", name, s.Status, lastSync, s.ItemsSynced)
}

func runGraphSync(ctx context.Context, deps *SourceCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := graphpb.NewGraphConnectorServiceClient(conn)
	resp, err := client.TriggerGraphSync(ctx, &graphpb.TriggerGraphSyncRequest{
		TenantId: getTenantID(),
		SyncType: graphSyncType,
	})
	if err != nil {
		return fmt.Errorf("triggering sync: %w", err)
	}

	if sourceOutput == "json" {
		return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
			"message":      resp.Message,
			"workflow_ids": resp.WorkflowIds,
		})
	}

	fmt.Println(resp.Message)
	if len(resp.WorkflowIds) > 0 {
		fmt.Printf("Workflows: %v\n", resp.WorkflowIds)
	}
	fmt.Println("Use 'penf source graph status' to monitor progress.")
	return nil
}

func runGraphChannels(ctx context.Context, deps *SourceCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := graphpb.NewGraphConnectorServiceClient(conn)
	resp, err := client.ListGraphChannels(ctx, &graphpb.ListGraphChannelsRequest{
		TenantId: getTenantID(),
	})
	if err != nil {
		return fmt.Errorf("listing channels: %w", err)
	}

	if sourceOutput == "json" {
		out := make([]map[string]interface{}, 0, len(resp.Channels))
		for _, c := range resp.Channels {
			out = append(out, map[string]interface{}{
				"team_name":      c.TeamName,
				"team_id":        c.TeamId,
				"channel_name":   c.ChannelName,
				"channel_id":     c.ChannelId,
				"mapped_project": c.MappedProject,
			})
		}
		return json.NewEncoder(os.Stdout).Encode(out)
	}

	if len(resp.Channels) == 0 {
		fmt.Println("No Teams channels found. Ensure Teams sync is configured and auth is complete.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TEAM\tCHANNEL\tCHANNEL ID\tMAPPED PROJECT")
	for _, c := range resp.Channels {
		project := c.MappedProject
		if project == "" {
			project = "—"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.TeamName, c.ChannelName, c.ChannelId, project)
	}
	return w.Flush()
}

func runGraphAuth(ctx context.Context, deps *SourceCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := graphpb.NewGraphConnectorServiceClient(conn)
	resp, err := client.InitiateGraphAuth(ctx, &graphpb.InitiateGraphAuthRequest{
		TenantId: getTenantID(),
	})
	if err != nil {
		return fmt.Errorf("initiating auth: %w", err)
	}

	fmt.Println(resp.Message)
	fmt.Printf("\n  URL:  %s\n", resp.VerificationUrl)
	fmt.Printf("  Code: %s\n\n", resp.UserCode)
	if resp.ExpiresIn > 0 {
		fmt.Printf("Code expires in %d minutes.\n", resp.ExpiresIn/60)
	}
	fmt.Println("After signing in, run 'penf source graph status' to confirm connection.")
	return nil
}

// ==================== Helpers ====================


func graphStatusToMap(r *graphpb.GetGraphStatusResponse) map[string]interface{} {
	out := map[string]interface{}{
		"auth_status": r.AuthStatus,
		"sync_status": r.SyncStatus,
		"sync_error":  r.SyncError,
		"enabled":     r.Enabled,
	}
	if r.LastSyncAt != nil {
		out["last_sync_at"] = r.LastSyncAt.AsTime()
	}
	if r.EmailStats != nil {
		out["email"] = syncStatsToMap(r.EmailStats)
	}
	if r.TeamsStats != nil {
		out["teams"] = syncStatsToMap(r.TeamsStats)
	}
	if r.OrgStats != nil {
		out["org"] = syncStatsToMap(r.OrgStats)
	}
	return out
}

func syncStatsToMap(s *graphpb.SyncStats) map[string]interface{} {
	m := map[string]interface{}{
		"status":       s.Status,
		"items_synced": s.ItemsSynced,
	}
	if s.LastSyncAt != nil {
		m["last_sync_at"] = s.LastSyncAt.AsTime()
	}
	return m
}
