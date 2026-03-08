package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/otherjamesbrown/penf-cli/pkg/db"
)

// serviceConfig defines the build and deploy configuration for a service.
type serviceConfig struct {
	Name           string
	GOOS           string
	GOARCH         string
	BuildDir       string
	BinaryName     string
	Host           string
	HostEnvVar     string
	BinaryPath     string
	ProcessManager string // "launchd" or "systemd"
	ServiceLabel   string // launchd label or systemd unit name
}

var services = map[string]serviceConfig{
	"gateway": {
		Name:           "gateway",
		GOOS:           "linux",
		GOARCH:         "amd64",
		BuildDir:       "services/gateway",
		BinaryName:     "gateway-linux",
		Host:           "dev02",
		HostEnvVar:     "GATEWAY_HOST",
		BinaryPath:     "/opt/penfold/bin/penfold-gateway",
		ProcessManager: "systemd",
		ServiceLabel:   "penfold-gateway",
	},
	"worker": {
		Name:           "worker",
		GOOS:           "darwin",
		GOARCH:         "arm64",
		BuildDir:       "services/worker",
		BinaryName:     "worker-darwin-arm64",
		Host:           "dev01",
		HostEnvVar:     "WORKER_HOST",
		BinaryPath:     "/opt/penfold/bin/penfold-worker",
		ProcessManager: "launchd",
		ServiceLabel:   "system/com.penfold.worker",
	},
	"ai": {
		Name:           "ai-coordinator",
		GOOS:           "linux",
		GOARCH:         "amd64",
		BuildDir:       "services/ai",
		BinaryName:     "ai-coordinator-linux",
		Host:           "dev02",
		HostEnvVar:     "AI_HOST",
		BinaryPath:     "/opt/penfold/bin/penfold-ai-coordinator",
		ProcessManager: "systemd",
		ServiceLabel:   "penfold-ai-coordinator",
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

	// bridge subcommand flags
	bridgeStatus bool
)

// NewDeployCommand creates the deploy command.
func NewDeployCommand() *cobra.Command {
	deployCmd := &cobra.Command{
		Use:   "deploy [gateway|worker|ai|all]",
		Short: "Build, upload, and deploy services",
		Long: `Build, upload, and deploy Penfold services.

Each service is cross-compiled, uploaded via SCP, and restarted using
the host's native process manager (launchd on macOS, systemd on Linux).

Examples:
  penf deploy gateway      Build and deploy gateway to dev02 (systemd)
  penf deploy worker       Build and deploy worker to dev01 (launchd)
  penf deploy ai           Build and deploy AI coordinator to dev02 (systemd)
  penf deploy all          Deploy all services in order
  penf deploy --status     Show service status
  penf deploy bridge       Deploy penfold-bridge (TypeScript/Node.js) to dev01

Subcommands:
  penf deploy history      Show deployment history
  penf deploy bridge       Deploy the WhatsApp bridge service

Environment:
  GATEWAY_HOST     Gateway host (default: dev02)
  WORKER_HOST      Worker host (default: dev01)
  AI_HOST          AI coordinator host (default: dev02)
  BRIDGE_HOST      Bridge host (default: dev01)`,
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

	deployCmd.Flags().BoolVar(&deployStatus, "status", false, "Show service status for all services")

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

	// Bridge subcommand — TypeScript/Node.js, separate deploy logic.
	bridgeCmd := &cobra.Command{
		Use:   "bridge",
		Short: "Deploy penfold-bridge (TypeScript/Node.js WhatsApp bridge)",
		Long: `Build and deploy the penfold-bridge service to dev01.

The bridge is a TypeScript/Node.js service and is deployed differently from Go services:
  1. npm run build     — compile TypeScript to dist/
  2. rsync            — upload dist/, package.json, package-lock.json, proto/ to dev01
  3. npm ci           — install production deps on dev01
  4. launchctl        — restart com.penfold.bridge

Examples:
  penf deploy bridge           Build and deploy bridge to dev01
  penf deploy bridge --status  Show bridge service status

Environment:
  BRIDGE_HOST      Bridge host (default: dev01)
  BRIDGE_REPO      Local path to penfold-bridge repo (default: ~/github/otherjamesbrown/penfold-bridge)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if bridgeStatus {
				return runDeployBridgeStatus()
			}
			return runDeployBridge()
		},
	}
	bridgeCmd.Flags().BoolVar(&bridgeStatus, "status", false, "Show bridge service status")
	deployCmd.AddCommand(bridgeCmd)

	return deployCmd
}

// restartService restarts a service using the host's native process manager.
func restartService(host string, svc serviceConfig) error {
	var shellCmd string
	switch svc.ProcessManager {
	case "launchd":
		shellCmd = fmt.Sprintf("sudo launchctl kickstart -k %s", svc.ServiceLabel)
	case "systemd":
		shellCmd = fmt.Sprintf("sudo systemctl restart %s", svc.ServiceLabel)
	default:
		return fmt.Errorf("unknown process manager: %s", svc.ProcessManager)
	}
	return runRemoteCmd(host, shellCmd)
}

// getServiceStatus checks service status using the host's native process manager.
func getServiceStatus(host string, svc serviceConfig) string {
	var shellCmd string
	switch svc.ProcessManager {
	case "launchd":
		shellCmd = fmt.Sprintf("sudo launchctl print %s 2>/dev/null | grep '^\\tstate' | awk '{print $NF}'", svc.ServiceLabel)
	case "systemd":
		shellCmd = fmt.Sprintf("systemctl is-active %s 2>/dev/null", svc.ServiceLabel)
	default:
		return "unknown"
	}
	out, err := runRemoteCmdOutput(host, shellCmd)
	if err != nil {
		return "not running"
	}
	return strings.TrimSpace(string(out))
}

// serviceHTTPPort returns the HTTP port for a service's health/version endpoints.
func serviceHTTPPort(svc serviceConfig) (int, error) {
	switch svc.Name {
	case "gateway":
		return 8080, nil
	case "worker":
		return 8085, nil
	case "ai-coordinator":
		return 8090, nil
	default:
		return 0, fmt.Errorf("unknown service: %s", svc.Name)
	}
}

// waitForServiceHealthy polls the service's health endpoint until it responds,
// then verifies the running binary matches the expected commit.
func waitForServiceHealthy(host string, svc serviceConfig, expectedCommit string, timeoutSecs int) error {
	port, err := serviceHTTPPort(svc)
	if err != nil {
		return err
	}
	healthURL := fmt.Sprintf("http://%s:%d/health", host, port)
	versionURL := fmt.Sprintf("http://%s:%d/version", host, port)

	// Wait for healthy
	for i := 0; i < timeoutSecs; i++ {
		cmd := exec.Command("curl", "-sf", "-o", "/dev/null", "-w", "%{http_code}", healthURL)
		out, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(out)) == "200" {
			fmt.Printf("  %s is healthy\n", svc.Name)

			// Verify the running commit matches what we just deployed
			if expectedCommit != "" && expectedCommit != "unknown" {
				vcmd := exec.Command("curl", "-sf", versionURL)
				vout, verr := vcmd.Output()
				if verr == nil {
					// Parse {"commit":"abc1234",...} from version endpoint
					var versionInfo struct {
						Commit string `json:"commit"`
					}
					if jsonErr := json.Unmarshal(vout, &versionInfo); jsonErr == nil {
						if versionInfo.Commit == expectedCommit {
							fmt.Printf("  Verified: running commit %s\n", expectedCommit)
						} else {
							return fmt.Errorf("version mismatch: expected commit %s, running %s", expectedCommit, versionInfo.Commit)
						}
					}
				}
			}
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("%s failed to become healthy within %ds", svc.Name, timeoutSecs)
}

// backupBinary creates a .prev backup of the current binary on the target host.
func backupBinary(host, binaryPath string) error {
	return runRemoteCmd(host, fmt.Sprintf("[ -f %s ] && cp %s %s.prev || true", binaryPath, binaryPath, binaryPath))
}

// rollbackBinary restores the .prev backup and restarts the service.
func rollbackBinary(host string, svc serviceConfig) error {
	fmt.Printf("  Rolling back %s...\n", svc.Name)
	if err := runRemoteCmd(host, fmt.Sprintf("[ -f %s.prev ] && mv %s.prev %s", svc.BinaryPath, svc.BinaryPath, svc.BinaryPath)); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}
	return restartService(host, svc)
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

// isLocalHost returns true if the given host refers to this machine.
func isLocalHost(host string) bool {
	hostname, err := os.Hostname()
	if err != nil {
		return false
	}
	// Match "dev01" against "dev01.brown.chat" etc.
	return host == hostname || strings.HasPrefix(hostname, host+".")
}

// runRemoteCmd runs a shell command on the given host via SSH, or locally if the host is this machine.
func runRemoteCmd(host, shellCmd string) error {
	if isLocalHost(host) {
		return runCmd("bash", "-c", shellCmd)
	}
	return runCmd("ssh", host, shellCmd)
}

// runRemoteCmdOutput runs a shell command on the given host and returns its output.
func runRemoteCmdOutput(host, shellCmd string) ([]byte, error) {
	var cmd *exec.Cmd
	if isLocalHost(host) {
		cmd = exec.Command("bash", "-c", shellCmd)
	} else {
		cmd = exec.Command("ssh", host, shellCmd)
	}
	return cmd.Output()
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// buildLDFlags returns ldflags for the penfold server binaries and the embedded commit hash.
func buildLDFlags() (ldflags string, commit string, err error) {
	// Get version from git
	verCmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	verOut, verErr := verCmd.Output()
	ver := "dev"
	if verErr == nil {
		ver = strings.TrimSpace(string(verOut))
	}

	// Get commit from git
	cmtCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmtOut, cmtErr := cmtCmd.Output()
	cmt := "unknown"
	if cmtErr == nil {
		cmt = strings.TrimSpace(string(cmtOut))
	}

	// Get build time
	bt := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	// Target the penfold server buildinfo package (not penf-cli)
	flags := fmt.Sprintf("-X github.com/otherjamesbrown/penfold/pkg/buildinfo.Version=%s -X github.com/otherjamesbrown/penfold/pkg/buildinfo.Commit=%s -X github.com/otherjamesbrown/penfold/pkg/buildinfo.BuildTime=%s",
		ver, cmt, bt)

	return flags, cmt, nil
}

// runPreDeployMigrations applies pending database migrations before deploying.
// Uses the database config from ~/.penf/config.yaml and the migrations directory
// from the penfold repo (resolved via config or the project root).
func runPreDeployMigrations(projectRoot string) error {
	fmt.Println("[0/4] Checking database migrations...")

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.Database == nil || !cfg.Database.IsConfigured() {
		fmt.Println("  ⚠ No database config — skipping migration check")
		fmt.Println("  Add 'database' section to ~/.penf/config.yaml to enable")
		fmt.Println()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := connectToDatabase(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	// Resolve migrations directory
	migDir := ""
	if cfg.Database != nil {
		migDir = cfg.Database.GetMigrationsDir()
	}
	if migDir == "" {
		migDir = filepath.Join(projectRoot, "migrations")
	}

	pending, err := db.GetPendingMigrations(ctx, pool, migDir)
	if err != nil {
		return fmt.Errorf("checking migrations: %w", err)
	}

	if len(pending) == 0 {
		fmt.Println("  ✓ All migrations applied")
		fmt.Println()
		return nil
	}

	fmt.Printf("  Applying %d pending migration(s)...\n", len(pending))
	for _, m := range pending {
		fmt.Printf("    %s - %s\n", m.Version, m.Name)
	}

	result, err := db.RunMigrations(ctx, pool, migDir)
	if err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	fmt.Printf("  ✓ Applied %d migration(s)\n\n", len(result.Applied))
	return nil
}

// runPostDeployPreflight runs a full preflight check after deploy to verify the system is ready.
// This is informational — deploy already succeeded, but we want to confirm everything is clean
// before handing off to workflows like /ingest.run.
func runPostDeployPreflight(host string) {
	fmt.Println("Running post-deploy preflight check...")

	// Derive health URLs from the deploy host.
	gatewayURL := fmt.Sprintf("http://%s:8080/health", host)
	coordinatorURL := fmt.Sprintf("http://%s:8090/health", host)

	result := runPreflightCheck(gatewayURL, coordinatorURL, 10*time.Second)
	outputPreflightHuman(result)
	fmt.Println()

	if !result.Passed {
		fmt.Println("\033[33m⚠ Post-deploy preflight has failures — check before running workflows\033[0m")
	} else {
		fmt.Println("\033[32m✓ System ready for workflows\033[0m")
	}
	fmt.Println()
}

func runDeploy(svc serviceConfig) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}

	host := hostForService(svc)

	fmt.Printf("=== Deploying %s (%s) ===\n\n", svc.Name, svc.ProcessManager)

	// Pre-step: run pending migrations for gateway deploys
	if svc.Name == "gateway" {
		if err := runPreDeployMigrations(root); err != nil {
			return fmt.Errorf("pre-deploy migrations failed: %w", err)
		}
	}

	// 1. Build
	fmt.Printf("[1/4] Building %s (%s/%s)...\n", svc.Name, svc.GOOS, svc.GOARCH)
	buildDir := filepath.Join(root, svc.BuildDir)
	buildOutput := filepath.Join(buildDir, svc.BinaryName)

	ldflags, commit, err := buildLDFlags()
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

	// 2. Backup + Upload
	fmt.Printf("[2/4] Backing up and uploading to %s:%s...\n", host, svc.BinaryPath)
	if err := backupBinary(host, svc.BinaryPath); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	if isLocalHost(host) {
		if err := runCmd("cp", buildOutput, svc.BinaryPath+".new"); err != nil {
			return fmt.Errorf("local copy failed: %w", err)
		}
	} else {
		if err := runCmd("scp", buildOutput, fmt.Sprintf("%s:%s.new", host, svc.BinaryPath)); err != nil {
			return fmt.Errorf("scp failed: %w", err)
		}
	}
	if err := runRemoteCmd(host, fmt.Sprintf("chmod +x %s.new && mv %s.new %s", svc.BinaryPath, svc.BinaryPath, svc.BinaryPath)); err != nil {
		return fmt.Errorf("binary swap failed: %w", err)
	}
	fmt.Printf("  Uploaded\n\n")

	// 3. Restart service
	fmt.Printf("[3/4] Restarting %s via %s...\n", svc.Name, svc.ProcessManager)
	if err := restartService(host, svc); err != nil {
		return fmt.Errorf("restart failed: %w", err)
	}

	// 4. Verify health + version
	fmt.Printf("[4/4] Waiting for %s to be healthy (commit %s)...\n", svc.Name, commit)
	if err := waitForServiceHealthy(host, svc, commit, 30); err != nil {
		fmt.Printf("  Health check failed, rolling back...\n")
		if rbErr := rollbackBinary(host, svc); rbErr != nil {
			return fmt.Errorf("health check failed (%w) and rollback also failed (%v)", err, rbErr)
		}
		return fmt.Errorf("health check failed, rolled back: %w", err)
	}

	fmt.Printf("\n=== %s deployed successfully ===\n\n", svc.Name)

	// Post-deploy: run full preflight to verify system is ready
	if svc.Name == "gateway" {
		runPostDeployPreflight(host)
	}

	// Fire-and-forget activity log
	if cfg, err := config.LoadConfig(); err == nil {
		logActivity(cfg, fmt.Sprintf("deploy: %s (%s)", svc.Name, commit))
	}

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
	fmt.Printf("%-25s %-10s %-8s %s\n", "SERVICE", "HOST", "MANAGER", "STATUS")
	fmt.Printf("%-25s %-10s %-8s %s\n", "-------", "----", "-------", "------")

	for _, name := range []string{"gateway", "worker", "ai"} {
		svc := services[name]
		host := hostForService(svc)
		status := getServiceStatus(host, svc)
		fmt.Printf("%-25s %-10s %-8s %s\n", svc.Name, host, svc.ProcessManager, status)
	}

	// Bridge status (launchd on dev01, queried via health endpoint)
	bridgeHost := os.Getenv("BRIDGE_HOST")
	if bridgeHost == "" {
		bridgeHost = "dev01"
	}
	bridgeHealthOut, err := remoteOutput(bridgeHost, "curl -sf http://localhost:3456/health 2>/dev/null | python3 -c \"import sys,json; d=json.load(sys.stdin); print('healthy' if d.get('healthy') else 'unhealthy')\" 2>/dev/null")
	bridgeStatusStr := "unknown"
	if err == nil && len(bridgeHealthOut) > 0 {
		bridgeStatusStr = strings.TrimSpace(string(bridgeHealthOut))
	}
	fmt.Printf("%-25s %-10s %-8s %s\n", "penfold-bridge", bridgeHost, "launchd", bridgeStatusStr)

	return nil
}

// runDeployBridge builds and deploys the penfold-bridge TypeScript service.
// Deploy steps differ from Go services: build with npm, rsync dist/, restart launchd.
func runDeployBridge() error {
	host := os.Getenv("BRIDGE_HOST")
	if host == "" {
		host = "dev01"
	}

	repoDir := os.Getenv("BRIDGE_REPO")
	if repoDir == "" {
		home, _ := os.UserHomeDir()
		repoDir = filepath.Join(home, "github", "otherjamesbrown", "penfold-bridge")
	}

	const (
		remoteDir    = "/opt/penfold/bridge"
		launchdLabel = "com.penfold.bridge"
		healthURL    = "http://localhost:3456/health"
	)

	fmt.Printf("=== Deploying penfold-bridge (Node.js/launchd) ===\n\n")

	// 1. Build TypeScript
	fmt.Printf("[1/4] Building TypeScript in %s...\n", repoDir)
	buildCmd := exec.Command("npm", "run", "build")
	buildCmd.Dir = repoDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("npm build failed: %w", err)
	}
	fmt.Printf("  Build complete\n\n")

	// 2. Upload: dist/, proto/, package.json, package-lock.json
	fmt.Printf("[2/4] Uploading to %s:%s...\n", host, remoteDir)
	if err := runRemoteCmd(host, fmt.Sprintf("mkdir -p %s", remoteDir)); err != nil {
		return fmt.Errorf("remote mkdir failed: %w", err)
	}
	rsyncArgs := []string{
		"-avz", "--delete",
		filepath.Join(repoDir, "dist") + "/",
		filepath.Join(repoDir, "proto") + "/",
		filepath.Join(repoDir, "package.json"),
		fmt.Sprintf("%s:%s/", host, remoteDir),
	}
	if isLocalHost(host) {
		// Local copy: replace remote "host:path" destination with plain local path
		localArgs := append(rsyncArgs[:len(rsyncArgs)-1:len(rsyncArgs)-1], remoteDir+"/")
		if err := runCmd("rsync", localArgs...); err != nil {
			return fmt.Errorf("rsync failed: %w", err)
		}
		// Also copy package-lock.json if it exists
		if err := runCmd("cp", filepath.Join(repoDir, "package-lock.json"), remoteDir+"/"); err == nil {
			_ = runCmd("cp", "-r", filepath.Join(repoDir, "proto"), remoteDir+"/")
		}
	} else {
		if err := runCmd("rsync", append([]string{"-e", "ssh"}, rsyncArgs...)...); err != nil {
			return fmt.Errorf("rsync failed: %w", err)
		}
		if err := runCmd("scp", filepath.Join(repoDir, "package-lock.json"), fmt.Sprintf("%s:%s/", host, remoteDir)); err != nil {
			return fmt.Errorf("scp package-lock.json failed: %w", err)
		}
		if err := runCmd("rsync", "-avz", "-e", "ssh", filepath.Join(repoDir, "proto")+"/", fmt.Sprintf("%s:%s/proto/", host, remoteDir)); err != nil {
			return fmt.Errorf("rsync proto failed: %w", err)
		}
	}
	fmt.Printf("  Uploaded\n\n")

	// Install production deps on remote
	fmt.Printf("  Installing production deps on %s...\n", host)
	if err := runRemoteCmd(host, fmt.Sprintf("cd %s && npm ci --omit=dev 2>&1", remoteDir)); err != nil {
		return fmt.Errorf("npm ci failed: %w", err)
	}

	// 3. Restart service via launchd
	fmt.Printf("[3/4] Restarting %s via launchd on %s...\n", launchdLabel, host)
	// Stop if running, then start (kickstart -k kills and restarts)
	if err := runRemoteCmd(host, fmt.Sprintf("sudo launchctl kickstart -k system/%s 2>/dev/null || sudo launchctl load /Library/LaunchDaemons/%s.plist", launchdLabel, launchdLabel)); err != nil {
		return fmt.Errorf("launchctl restart failed: %w", err)
	}

	// 4. Verify health
	fmt.Printf("[4/4] Waiting for bridge health endpoint (%s)...\n", healthURL)
	for i := 0; i < 15; i++ {
		time.Sleep(2 * time.Second)
		out, err := remoteOutput(host, fmt.Sprintf("curl -sf %s 2>/dev/null | head -c 200", healthURL))
		if err == nil && len(out) > 0 {
			fmt.Printf("  Health OK: %s\n", strings.TrimSpace(string(out)))
			fmt.Printf("\n=== penfold-bridge deployed successfully ===\n")
			return nil
		}
	}
	return fmt.Errorf("bridge health check timed out — check logs: ssh %s 'tail -50 /var/log/penfold-bridge.log'", host)
}

func runDeployBridgeStatus() error {
	host := os.Getenv("BRIDGE_HOST")
	if host == "" {
		host = "dev01"
	}
	const launchdLabel = "com.penfold.bridge"
	statusCmd := fmt.Sprintf("launchctl print system/%s 2>/dev/null | grep -E 'state|pid' | head -5", launchdLabel)
	out, err := remoteOutput(host, statusCmd)
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		fmt.Printf("penfold-bridge: not loaded (or launchctl unavailable on %s)\n", host)
		return nil
	}
	fmt.Printf("penfold-bridge on %s:\n%s\n", host, string(out))

	// Also show health endpoint
	health, _ := remoteOutput(host, "curl -sf http://localhost:3456/health 2>/dev/null")
	if len(health) > 0 {
		fmt.Printf("health: %s\n", strings.TrimSpace(string(health)))
	}
	return nil
}

// remoteOutput runs a command on host and returns stdout (supports local host).
func remoteOutput(host, shellCmd string) ([]byte, error) {
	if isLocalHost(host) {
		return exec.Command("sh", "-c", shellCmd).Output()
	}
	return exec.Command("ssh", host, shellCmd).Output()
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
	NomadJobVersion sql.NullInt32 // legacy column, kept for backward compatibility
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
