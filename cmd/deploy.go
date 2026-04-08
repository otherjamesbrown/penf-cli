package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/penf-cli/config"
	"github.com/otherjamesbrown/penf-cli/contextpalace"
)

var (
	deployStatus bool
	historyLast  int

	// record subcommand flags
	recordCommit         string
	recordPreviousCommit string
	recordDeployedBy     string
	recordVersion        string
	recordChanges        string
	recordShardIDs       string
	recordNotify         bool
)

// penfoldRepoDir returns the path to the penfold backend repo.
// Uses PENFOLD_REPO env var, falling back to ~/github/otherjamesbrown/penfold.
func penfoldRepoDir() string {
	if d := os.Getenv("PENFOLD_REPO"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "github", "otherjamesbrown", "penfold")
}

// NewDeployCommand creates the deploy command.
func NewDeployCommand() *cobra.Command {
	deployCmd := &cobra.Command{
		Use:   "deploy [gateway|worker|ai|mcp|all]",
		Short: "Build, upload, and deploy services",
		Long: `Build, upload, and deploy Penfold services.

Delegates to the canonical deploy scripts in the penfold repo
(penfold/scripts/deploy.sh). Each service is cross-compiled, uploaded,
and restarted using the host's native process manager.

Examples:
  penf deploy gateway      Build and deploy gateway to dev02 (systemd)
  penf deploy worker       Build and deploy worker to dev01 (launchd)
  penf deploy ai           Build and deploy AI coordinator to dev02 (systemd)
  penf deploy mcp          Build and deploy MCP server to dev02 (systemd)
  penf deploy all          Deploy all services in order
  penf deploy --status     Show service status
  penf deploy bridge       Deploy penfold-bridge (TypeScript/Node.js) to dev01

Subcommands:
  penf deploy history      Show deployment history
  penf deploy record       Record a deployment in deploy_history

Environment:
  PENFOLD_REPO     Path to penfold repo (default: ~/github/otherjamesbrown/penfold)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if deployStatus {
				return runDeployScript("status")
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			return runDeployScript(args[0])
		},
	}

	deployCmd.Flags().BoolVar(&deployStatus, "status", false, "Show service status for all services")

	// Add history subcommand.
	historyCmd := &cobra.Command{
		Use:   "history [service]",
		Short: "Show deployment history",
		Long: `Display deployment history from the deploy_history table.

Examples:
  penf deploy history                Show all deployments
  penf deploy history gateway        Show gateway deployments only
  penf deploy history --last 5       Show last 5 deployments

Environment:
  PENFOLD_DB_URL   Database connection string (overrides config)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := ""
			if len(args) > 0 {
				svc = args[0]
			}
			return runDeployHistory(svc)
		},
	}
	historyCmd.Flags().IntVar(&historyLast, "last", 0, "Limit to last N deployments")
	deployCmd.AddCommand(historyCmd)

	// Add record subcommand.
	recordCmd := &cobra.Command{
		Use:   "record <service>",
		Short: "Record a deployment in deploy_history",
		Long: `Record a deployment in the deploy_history table and optionally send
a Context-Palace notification.

Examples:
  penf deploy record penfold-gateway --commit abc123
  penf deploy record penfold-worker --commit abc123 --previous-commit def456
  penf deploy record penfold-gateway --commit abc123 --notify --shard-ids pf-xxx,pf-yyy

Environment:
  PENFOLD_DB_URL   Database connection string (overrides config)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeployRecord(args[0])
		},
	}
	recordCmd.Flags().StringVar(&recordCommit, "commit", "", "New commit hash (required)")
	recordCmd.Flags().StringVar(&recordPreviousCommit, "previous-commit", "", "Previous commit hash")
	recordCmd.Flags().StringVar(&recordDeployedBy, "deployed-by", "", "Who deployed (default: from Context-Palace config)")
	recordCmd.Flags().StringVar(&recordVersion, "version", "", "Version tag (default: git describe)")
	recordCmd.Flags().StringVar(&recordChanges, "changes", "", "Changelog (default: git log between commits)")
	recordCmd.Flags().StringVar(&recordShardIDs, "shard-ids", "", "Comma-separated Context-Palace shard IDs")
	recordCmd.Flags().BoolVar(&recordNotify, "notify", true, "Send Context-Palace notification")
	_ = recordCmd.MarkFlagRequired("commit")
	deployCmd.AddCommand(recordCmd)

	return deployCmd
}

// runDeployScript delegates to penfold/scripts/deploy.sh with the given arguments.
func runDeployScript(args ...string) error {
	repoDir := penfoldRepoDir()
	scriptPath := filepath.Join(repoDir, "scripts", "deploy.sh")

	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("deploy script not found at %s\n  Set PENFOLD_REPO to the penfold repo path", scriptPath)
	}

	cmd := exec.Command(scriptPath, args...)
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// --- history and record subcommands (CLI-specific DB features) ---

type deployHistoryEntry struct {
	ID              int
	ServiceName     string
	Commit          string
	PreviousCommit  sql.NullString
	Version         sql.NullString
	DeployedAt      time.Time
	DeployedBy      sql.NullString
	Changes         sql.NullString
	NomadJobVersion sql.NullInt32
	ShardIDs        []string
}

func resolveDeployDBURL() (string, error) {
	if v := os.Getenv("PENFOLD_DB_URL"); v != "" {
		return v, nil
	}
	cfg, err := config.LoadConfig()
	if err == nil && cfg.Database != nil && cfg.Database.IsConfigured() {
		return cfg.Database.ConnectionString(), nil
	}
	return "", fmt.Errorf("PENFOLD_DB_URL not set and no database configured in ~/.penf/config.yaml")
}

func runDeployRecord(serviceName string) error {
	dbURL, err := resolveDeployDBURL()
	if err != nil {
		return err
	}

	version := recordVersion
	if version == "" {
		verCmd := exec.Command("git", "describe", "--tags", "--always")
		if out, err := verCmd.Output(); err == nil {
			version = strings.TrimSpace(string(out))
		}
	}

	changes := recordChanges
	if changes == "" && recordPreviousCommit != "" && recordCommit != "" {
		logCmd := exec.Command("git", "log", "--oneline", recordPreviousCommit+".."+recordCommit)
		if out, err := logCmd.Output(); err == nil {
			changes = strings.TrimSpace(string(out))
		}
	}
	if changes == "" {
		changes = "Deploy " + recordCommit
	}

	deployedBy := recordDeployedBy
	if deployedBy == "" {
		if cfg, err := config.LoadConfig(); err == nil && cfg.ContextPalace != nil {
			deployedBy = cfg.ContextPalace.GetAgent()
		}
	}
	if deployedBy == "" {
		deployedBy = "agent-mycroft"
	}

	var shardIDs []string
	if recordShardIDs != "" {
		for _, id := range strings.Split(recordShardIDs, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				shardIDs = append(shardIDs, id)
			}
		}
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	query := `
		INSERT INTO deploy_history (service_name, commit, previous_commit, version, deployed_by, changes, shard_ids)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	var prevCommit *string
	if recordPreviousCommit != "" {
		prevCommit = &recordPreviousCommit
	}

	var versionPtr *string
	if version != "" {
		versionPtr = &version
	}

	_, err = db.ExecContext(ctx, query,
		serviceName, recordCommit, prevCommit, versionPtr, deployedBy, changes, shardIDs,
	)
	if err != nil {
		return fmt.Errorf("failed to insert deploy record: %w", err)
	}

	prev := recordPreviousCommit
	if prev == "" {
		prev = "unknown"
	}
	fmt.Printf("Recorded: %s %s -> %s\n", serviceName, prev, recordCommit)

	if recordNotify {
		if err := sendDeployNotification(ctx, serviceName, recordCommit, prev, version, changes); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to send deploy notification: %v\n", err)
		}
	}

	return nil
}

func sendDeployNotification(ctx context.Context, serviceName, commit, previousCommit, version, changes string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	cpClient, err := contextpalace.NewClient(cfg.ContextPalace)
	if err != nil {
		return fmt.Errorf("creating context-palace client: %w", err)
	}
	defer cpClient.Close()

	subject := fmt.Sprintf("Deploy: %s %s", serviceName, commit)
	body := fmt.Sprintf("Deployed %s %s (was %s)\n\nVersion: %s\n\nChanges:\n%s",
		serviceName, commit, previousCommit, version, changes)

	_, err = cpClient.SendMessage(ctx, []string{"agent-penfold"}, subject, body, &contextpalace.SendMessageOptions{
		Kind: "deploy",
	})
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}

	fmt.Println("Notification sent to agent-penfold")
	return nil
}

func runDeployHistory(serviceName string) error {
	dbURL, err := resolveDeployDBURL()
	if err != nil {
		return err
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	query := `
		SELECT id, service_name, commit, previous_commit, version,
		       deployed_at, deployed_by, changes, nomad_job_version, shard_ids
		FROM deploy_history`

	var args []interface{}
	if serviceName != "" {
		query += " WHERE service_name = $1"
		args = append(args, serviceName)
	}

	query += " ORDER BY deployed_at DESC"

	if historyLast > 0 {
		query += fmt.Sprintf(" LIMIT %d", historyLast)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to query deploy history: %w", err)
	}
	defer rows.Close()

	var entries []deployHistoryEntry
	for rows.Next() {
		var entry deployHistoryEntry
		err := rows.Scan(
			&entry.ID, &entry.ServiceName, &entry.Commit, &entry.PreviousCommit,
			&entry.Version, &entry.DeployedAt, &entry.DeployedBy, &entry.Changes,
			&entry.NomadJobVersion, &entry.ShardIDs,
		)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No deployment history found.")
		return nil
	}

	fmt.Printf("%-20s %-25s %-10s %-50s %s\n", "DEPLOYED AT", "SERVICE", "COMMIT", "CHANGES", "BY")
	fmt.Printf("%-20s %-25s %-10s %-50s %s\n", "-----------", "-------", "------", "-------", "--")

	for _, entry := range entries {
		deployedAt := entry.DeployedAt.Format("2006-01-02 15:04")
		commit := entry.Commit
		if len(commit) > 10 {
			commit = commit[:10]
		}
		changes := ""
		if entry.Changes.Valid {
			changes = entry.Changes.String
			if len(changes) > 50 {
				changes = changes[:47] + "..."
			}
		}
		deployedBy := "-"
		if entry.DeployedBy.Valid && entry.DeployedBy.String != "" {
			deployedBy = entry.DeployedBy.String
		}

		fmt.Printf("%-20s %-25s %-10s %-50s %s\n",
			deployedAt, entry.ServiceName, commit, changes, deployedBy)
	}

	return nil
}
