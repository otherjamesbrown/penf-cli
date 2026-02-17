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

// serviceConfig defines the build and deploy configuration for a service.
type serviceConfig struct {
	Name       string
	GOOS       string
	GOARCH     string
	BuildDir   string
	BinaryName string
	Host       string
	HostEnvVar string
	BinaryPath string
	NomadJob   string
	NomadName  string
}

var services = map[string]serviceConfig{
	"gateway": {
		Name:       "gateway",
		GOOS:       "linux",
		GOARCH:     "amd64",
		BuildDir:   "services/gateway",
		BinaryName: "gateway-linux",
		Host:       "dev02",
		HostEnvVar: "GATEWAY_HOST",
		BinaryPath: "/opt/penfold/bin/penfold-gateway",
		NomadJob:   "deploy/nomad/gateway.nomad.hcl",
		NomadName:  "penfold-gateway",
	},
	"worker": {
		Name:       "worker",
		GOOS:       "darwin",
		GOARCH:     "arm64",
		BuildDir:   "services/worker",
		BinaryName: "worker-darwin-arm64",
		Host:       "dev01",
		HostEnvVar: "WORKER_HOST",
		BinaryPath: "/opt/penfold/bin/penfold-worker",
		NomadJob:   "deploy/nomad/worker.nomad.hcl",
		NomadName:  "penfold-worker",
	},
	"ai": {
		Name:       "ai-coordinator",
		GOOS:       "linux",
		GOARCH:     "amd64",
		BuildDir:   "services/ai",
		BinaryName: "ai-coordinator-linux",
		Host:       "dev02",
		HostEnvVar: "AI_HOST",
		BinaryPath: "/opt/penfold/bin/penfold-ai-coordinator",
		NomadJob:   "deploy/nomad/ai-coordinator.nomad.hcl",
		NomadName:  "penfold-ai-coordinator",
	},
}

var (
	deployStatus       bool
	historyLast        int
	historyServiceName string

	// record subcommand flags
	recordCommit         string
	recordPreviousCommit string
	recordDeployedBy     string
	recordVersion        string
	recordChanges        string
	recordShardIDs       string
	recordNotify         bool
)

// NewDeployCommand creates the deploy command.
func NewDeployCommand() *cobra.Command {
	deployCmd := &cobra.Command{
		Use:   "deploy [gateway|worker|ai|all]",
		Short: "Build, upload, and deploy services via Nomad",
		Long: `Build, upload, and deploy Penfold services using Nomad.

Each service is cross-compiled, uploaded via SCP, and deployed using
'nomad job run'. Nomad handles health checks, canary promotion, and
auto-revert on failure.

Examples:
  penf deploy gateway      Build and deploy gateway to dev02
  penf deploy worker       Build and deploy worker to dev01
  penf deploy ai           Build and deploy AI coordinator to dev02
  penf deploy all          Deploy all services in order
  penf deploy --status     Show Nomad job status for all services

Subcommands:
  penf deploy history      Show deployment history

Environment:
  NOMAD_ADDR       Nomad server address (default: http://dev02.brown.chat:4646)
  GATEWAY_HOST     Gateway host (default: dev02)
  WORKER_HOST      Worker host (default: dev01)
  AI_HOST          AI coordinator host (default: dev02)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if deployStatus {
				return runDeployStatus()
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			target := args[0]
			if target == "all" {
				return runDeployAll()
			}
			svc, ok := services[target]
			if !ok {
				return fmt.Errorf("unknown service: %s (valid: gateway, worker, ai, all)", target)
			}
			return runDeploy(svc)
		},
	}

	deployCmd.Flags().BoolVar(&deployStatus, "status", false, "Show Nomad job status for all services")

	// Add history subcommand.
	historyCmd := &cobra.Command{
		Use:   "history [service]",
		Short: "Show deployment history",
		Long: `Display deployment history from the deploy_history table.

Optionally filter by service name and limit results.

Examples:
  penf deploy history                Show all deployments
  penf deploy history gateway        Show gateway deployments only
  penf deploy history --last 5       Show last 5 deployments
  penf deploy history gateway --last 10  Show last 10 gateway deployments

Environment:
  PENFOLD_DB_URL   Database connection string (required)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				historyServiceName = args[0]
			}
			return runDeployHistory()
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

This replaces the shell-based record_deploy() function in deploy scripts,
providing proper parameterized queries and unified notification logic.

Examples:
  penf deploy record penfold-gateway --commit abc123
  penf deploy record penfold-worker --commit abc123 --previous-commit def456
  penf deploy record penfold-gateway --commit abc123 --notify --shard-ids pf-xxx,pf-yyy

Environment:
  PENFOLD_DB_URL   Database connection string (required)`,
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

func nomadAddr() string {
	if addr := os.Getenv("NOMAD_ADDR"); addr != "" {
		return addr
	}
	return "http://dev02.brown.chat:4646"
}

func projectRoot() (string, error) {
	// Walk up from the executable or current directory to find go.mod.
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("cannot find project root (no go.mod found)")
		}
		dir = parent
	}
}

func hostForService(svc serviceConfig) string {
	if h := os.Getenv(svc.HostEnvVar); h != "" {
		return h
	}
	return svc.Host
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCmdEnv(env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return cmd.Run()
}

func buildLDFlags() (string, error) {
	// Get version from git
	verCmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	verOut, err := verCmd.Output()
	ver := "dev"
	if err == nil {
		ver = strings.TrimSpace(string(verOut))
	}

	// Get commit from git
	cmtCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmtOut, err := cmtCmd.Output()
	cmt := "unknown"
	if err == nil {
		cmt = strings.TrimSpace(string(cmtOut))
	}

	// Get build time
	bt := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	// Build ldflags string targeting pkg/buildinfo
	ldflags := fmt.Sprintf("-X github.com/otherjamesbrown/penf-cli/pkg/buildinfo.Version=%s -X github.com/otherjamesbrown/penf-cli/pkg/buildinfo.Commit=%s -X github.com/otherjamesbrown/penf-cli/pkg/buildinfo.BuildTime=%s",
		ver, cmt, bt)

	return ldflags, nil
}

func runDeploy(svc serviceConfig) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}

	host := hostForService(svc)

	fmt.Printf("=== Deploying %s ===\n\n", svc.Name)

	// 1. Build
	fmt.Printf("[1/3] Building %s (%s/%s)...\n", svc.Name, svc.GOOS, svc.GOARCH)
	buildDir := filepath.Join(root, svc.BuildDir)
	buildOutput := filepath.Join(buildDir, svc.BinaryName)

	ldflags, err := buildLDFlags()
	if err != nil {
		return fmt.Errorf("failed to generate ldflags: %w", err)
	}

	buildCmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", buildOutput, ".")
	buildCmd.Dir = buildDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Env = append(os.Environ(),
		"GOOS="+svc.GOOS,
		"GOARCH="+svc.GOARCH,
	)
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fi, err := os.Stat(buildOutput)
	if err != nil {
		return fmt.Errorf("build output not found: %w", err)
	}
	fmt.Printf("  Built %s (%.1f MB)\n\n", svc.BinaryName, float64(fi.Size())/(1024*1024))

	// 2. Upload
	fmt.Printf("[2/3] Uploading to %s:%s...\n", host, svc.BinaryPath)
	if err := runCmd("scp", buildOutput, fmt.Sprintf("%s:%s.new", host, svc.BinaryPath)); err != nil {
		return fmt.Errorf("scp failed: %w", err)
	}
	if err := runCmd("ssh", host, fmt.Sprintf("chmod +x %s.new && mv %s.new %s", svc.BinaryPath, svc.BinaryPath, svc.BinaryPath)); err != nil {
		return fmt.Errorf("binary swap failed: %w", err)
	}
	fmt.Printf("  Uploaded\n\n")

	// 3. Nomad job run
	fmt.Printf("[3/3] Submitting Nomad job: %s...\n", svc.NomadName)
	jobFile := filepath.Join(root, svc.NomadJob)
	if err := runCmdEnv([]string{"NOMAD_ADDR=" + nomadAddr()}, "nomad", "job", "run", jobFile); err != nil {
		return fmt.Errorf("nomad job run failed: %w", err)
	}

	// Wait for healthy
	fmt.Printf("  Waiting for %s to be healthy...\n", svc.NomadName)
	if err := waitForNomadHealthy(svc.NomadName, 60); err != nil {
		return err
	}

	fmt.Printf("\n=== %s deployed successfully ===\n", svc.Name)
	return nil
}

func runDeployAll() error {
	// Deploy in dependency order: gateway -> worker -> ai
	order := []string{"gateway", "worker", "ai"}
	for _, name := range order {
		svc := services[name]
		if err := runDeploy(svc); err != nil {
			return fmt.Errorf("deploy %s failed: %w", name, err)
		}
		fmt.Println()
	}
	fmt.Println("=== All services deployed ===")
	return nil
}

func runDeployStatus() error {
	addr := nomadAddr()
	fmt.Printf("Nomad: %s\n\n", addr)
	fmt.Printf("%-25s %s\n", "JOB", "STATUS")
	fmt.Printf("%-25s %s\n", "---", "------")

	for _, name := range []string{"gateway", "worker", "ai"} {
		svc := services[name]
		status := getNomadJobStatus(svc.NomadName)
		fmt.Printf("%-25s %s\n", svc.NomadName, status)
	}
	return nil
}

func getNomadJobStatus(jobName string) string {
	cmd := exec.Command("nomad", "job", "status", "-short", jobName)
	cmd.Env = append(os.Environ(), "NOMAD_ADDR="+nomadAddr())
	out, err := cmd.Output()
	if err != nil {
		return "not found"
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Status") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return parts[len(parts)-1]
			}
		}
	}
	return "unknown"
}

func waitForNomadHealthy(jobName string, timeoutSecs int) error {
	for i := 0; i < timeoutSecs; i++ {
		status := getNomadJobStatus(jobName)
		if status == "running" {
			fmt.Printf("  %s is running\n", jobName)
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("%s failed to become healthy within %ds", jobName, timeoutSecs)
}

// deployHistoryEntry represents a row from the deploy_history table.
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

func runDeployRecord(serviceName string) error {
	// Get database URL from environment.
	dbURL := os.Getenv("PENFOLD_DB_URL")
	if dbURL == "" {
		return fmt.Errorf("PENFOLD_DB_URL environment variable not set")
	}

	// Resolve version: flag > git describe.
	version := recordVersion
	if version == "" {
		verCmd := exec.Command("git", "describe", "--tags", "--always")
		if out, err := verCmd.Output(); err == nil {
			version = strings.TrimSpace(string(out))
		}
	}

	// Resolve changes: flag > git log between commits.
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

	// Resolve deployed-by: flag > Context-Palace config > default.
	deployedBy := recordDeployedBy
	if deployedBy == "" {
		if cfg, err := config.LoadConfig(); err == nil && cfg.ContextPalace != nil {
			deployedBy = cfg.ContextPalace.GetAgent()
		}
	}
	if deployedBy == "" {
		deployedBy = "agent-mycroft"
	}

	// Parse shard IDs.
	var shardIDs []string
	if recordShardIDs != "" {
		for _, id := range strings.Split(recordShardIDs, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				shardIDs = append(shardIDs, id)
			}
		}
	}

	// Insert into deploy_history.
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
		serviceName,
		recordCommit,
		prevCommit,
		versionPtr,
		deployedBy,
		changes,
		shardIDs,
	)
	if err != nil {
		return fmt.Errorf("failed to insert deploy record: %w", err)
	}

	prev := recordPreviousCommit
	if prev == "" {
		prev = "unknown"
	}
	fmt.Printf("Recorded: %s %s -> %s\n", serviceName, prev, recordCommit)

	// Send Context-Palace notification if requested.
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

func runDeployHistory() error {
	// Get database URL from environment.
	dbURL := os.Getenv("PENFOLD_DB_URL")
	if dbURL == "" {
		return fmt.Errorf("PENFOLD_DB_URL environment variable not set")
	}

	// Connect to database.
	ctx := context.Background()
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Build query.
	query := `
		SELECT id, service_name, commit, previous_commit, version,
		       deployed_at, deployed_by, changes, nomad_job_version, shard_ids
		FROM deploy_history`

	var args []interface{}
	var whereClauses []string

	if historyServiceName != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("service_name = $%d", len(args)+1))
		args = append(args, historyServiceName)
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	query += " ORDER BY deployed_at DESC"

	if historyLast > 0 {
		query += fmt.Sprintf(" LIMIT %d", historyLast)
	}

	// Execute query.
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to query deploy history: %w", err)
	}
	defer rows.Close()

	// Collect results.
	var entries []deployHistoryEntry
	for rows.Next() {
		var entry deployHistoryEntry
		err := rows.Scan(
			&entry.ID,
			&entry.ServiceName,
			&entry.Commit,
			&entry.PreviousCommit,
			&entry.Version,
			&entry.DeployedAt,
			&entry.DeployedBy,
			&entry.Changes,
			&entry.NomadJobVersion,
			&entry.ShardIDs,
		)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	// Display results.
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
