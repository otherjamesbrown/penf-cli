// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/penf-cli/config"
	"github.com/otherjamesbrown/penf-cli/pkg/db"
)

// AICoordinatorHealthStatus represents the health status from the AI coordinator.
type AICoordinatorHealthStatus struct {
	Status    string                        `json:"status"`
	Checks    map[string]AICoordinatorCheck `json:"checks"`
	Timestamp time.Time                     `json:"timestamp"`
}

// AICoordinatorCheck represents a single health check from the AI coordinator.
type AICoordinatorCheck struct {
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
}

// PreflightResult holds the result of a preflight health check.
type PreflightResult struct {
	Passed   bool             `json:"passed"`
	ExitCode int              `json:"exit_code"`
	Message  string           `json:"message"`
	Failures []string         `json:"failures,omitempty"`
	Warnings []string         `json:"warnings,omitempty"`
	Checks   []PreflightCheck `json:"checks"`
}

// PreflightCheck represents the status of a single preflight check.
type PreflightCheck struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	LatencyMs    int64  `json:"latency_ms,omitempty"`
	CircuitState string `json:"circuit_state,omitempty"`
	Critical     bool   `json:"critical,omitempty"`
	Error        string `json:"error,omitempty"`
}

var (
	preflightTimeout        time.Duration
	preflightJSON           bool
	preflightGatewayURL     string
	preflightCoordinatorURL string
)

// NewHealthPreflightCommand creates the health preflight command.
func NewHealthPreflightCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preflight",
		Short: "Run preflight health checks before ingestion",
		Long: `Run comprehensive preflight health checks to ensure the Penfold system is ready
for ingestion operations. This prevents wasting API credits on a broken system.

Checks performed:
  1. Gateway reachability and health
  2. Critical service health (database must be healthy)
  3. Circuit breaker states (all must be closed)
  4. AI coordinator health
  5. Database migrations (schema must be current)
  6. Pipeline definitions (all pipelines must have at least one enabled stage)

Exit codes:
  0 - All critical services healthy and circuit breakers closed
  1 - At least one critical service unhealthy or circuit breaker open

Environment variables:
  - GATEWAY_HEALTH_URL: Override gateway health endpoint (default: derived from server address)
  - AI_COORDINATOR_HEALTH_URL: Override AI coordinator health endpoint (default: http://<server>:8090/health)`,
		RunE: runHealthPreflight,
	}

	cmd.Flags().DurationVar(&preflightTimeout, "timeout", 5*time.Second, "Timeout for health checks")
	cmd.Flags().BoolVar(&preflightJSON, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&preflightGatewayURL, "gateway-url", "", "Override gateway health URL")
	cmd.Flags().StringVar(&preflightCoordinatorURL, "coordinator-url", "", "Override AI coordinator health URL")

	return cmd
}

func runHealthPreflight(cmd *cobra.Command, args []string) error {
	// Determine health URLs.
	gatewayURL := preflightGatewayURL
	if gatewayURL == "" {
		gatewayURL = os.Getenv("GATEWAY_HEALTH_URL")
	}

	coordinatorURL := preflightCoordinatorURL
	if coordinatorURL == "" {
		coordinatorURL = os.Getenv("AI_COORDINATOR_HEALTH_URL")
	}

	// Derive URLs if not explicitly provided.
	if gatewayURL == "" || coordinatorURL == "" {
		serverAddr := os.Getenv("PENF_SERVER_ADDRESS")
		if serverAddr == "" {
			serverAddr = "dev02.brown.chat:50051"
		}
		derivedGW, derivedAI := deriveHealthURLs(serverAddr)
		if gatewayURL == "" {
			gatewayURL = derivedGW
		}
		if coordinatorURL == "" {
			coordinatorURL = derivedAI
		}
	}

	// Run preflight check.
	result := runPreflightCheck(gatewayURL, coordinatorURL, preflightTimeout)

	// Output results.
	if preflightJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return fmt.Errorf("encode JSON: %w", err)
		}
	} else {
		outputPreflightHuman(result)
	}

	// Exit with appropriate code.
	if result.ExitCode != 0 {
		os.Exit(result.ExitCode)
	}

	return nil
}

// deriveHealthURLs derives gateway and AI coordinator health URLs from server address.
func deriveHealthURLs(serverAddr string) (gatewayURL, coordinatorURL string) {
	// Extract host from server address.
	host := serverAddr
	if idx := strings.LastIndex(serverAddr, ":"); idx != -1 {
		host = serverAddr[:idx]
	}

	gatewayURL = fmt.Sprintf("http://%s:8080/health", host)
	coordinatorURL = fmt.Sprintf("http://%s:8090/health", host)
	return
}

// runPreflightCheck performs the actual preflight health checks.
func runPreflightCheck(gatewayURL, coordinatorURL string, timeout time.Duration) PreflightResult {
	result := PreflightResult{
		Passed:   true,
		ExitCode: 0,
		Message:  "All critical services healthy",
		Failures: []string{},
		Warnings: []string{},
		Checks:   []PreflightCheck{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Check 1: Gateway health.
	gatewayStatus, gatewayLatency, gatewayErr := checkGatewayHealth(ctx, gatewayURL)
	if gatewayErr != nil {
		result.Passed = false
		result.ExitCode = 1
		result.Message = "Preflight check failed"
		result.Failures = append(result.Failures, fmt.Sprintf("Gateway unreachable: %v", gatewayErr))
		result.Checks = append(result.Checks, PreflightCheck{
			Name:   "Gateway",
			Status: "unreachable",
			Error:  gatewayErr.Error(),
		})
	} else {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:      "Gateway",
			Status:    gatewayStatus.Status,
			LatencyMs: gatewayLatency.Milliseconds(),
		})

		// Check gateway services.
		for _, svc := range gatewayStatus.Services {
			check := PreflightCheck{
				Name:         svc.Name,
				Status:       svc.Status,
				CircuitState: svc.CircuitState,
				Critical:     svc.Critical,
				Error:        svc.Error,
			}
			result.Checks = append(result.Checks, check)

			// Critical service check.
			if svc.Critical && svc.Status != "healthy" {
				result.Passed = false
				result.ExitCode = 1
				result.Message = "Preflight check failed"
				msg := fmt.Sprintf("Gateway: %s %s (circuit: %s) [critical]", svc.Name, svc.Status, svc.CircuitState)
				if svc.Error != "" {
					msg += fmt.Sprintf(" — %s", svc.Error)
				}
				result.Failures = append(result.Failures, msg)
			}

			// Circuit breaker check (critical services only).
			if svc.Critical && svc.CircuitState != "closed" && svc.CircuitState != "" {
				result.Passed = false
				result.ExitCode = 1
				result.Message = "Preflight check failed"
				result.Failures = append(result.Failures, fmt.Sprintf("Gateway: %s circuit breaker %s [critical]", svc.Name, svc.CircuitState))
			}

			// Non-critical warnings.
			if !svc.Critical {
				if svc.Status != "healthy" {
					warning := fmt.Sprintf("Gateway: %s %s", svc.Name, svc.Status)
					if svc.Error != "" {
						warning += fmt.Sprintf(" — %s", svc.Error)
					}
					result.Warnings = append(result.Warnings, warning)
				}
				if svc.CircuitState != "closed" && svc.CircuitState != "" {
					result.Warnings = append(result.Warnings, fmt.Sprintf("Gateway: %s (circuit: %s)", svc.Name, svc.CircuitState))
				}
			}
		}
	}

	// Check 2: AI Coordinator health.
	aiStatus, aiLatency, aiErr := checkAICoordinatorHealth(ctx, coordinatorURL)
	if aiErr != nil {
		result.Passed = false
		result.ExitCode = 1
		result.Message = "Preflight check failed"
		result.Failures = append(result.Failures, fmt.Sprintf("AI Coordinator unreachable: %v [critical]", aiErr))
		result.Checks = append(result.Checks, PreflightCheck{
			Name:     "AI Coordinator",
			Status:   "unreachable",
			Critical: true,
			Error:    aiErr.Error(),
		})
	} else {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:      "AI Coordinator",
			Status:    aiStatus.Status,
			LatencyMs: aiLatency.Milliseconds(),
			Critical:  true,
		})

		// AI coordinator is critical for enrichment.
		if aiStatus.Status != "healthy" {
			result.Passed = false
			result.ExitCode = 1
			result.Message = "Preflight check failed"
			result.Failures = append(result.Failures, "AI Coordinator unhealthy [critical]")
		}
	}

	// Check 3: Database migrations.
	checkMigrations(&result, timeout)

	// Check 4: Pipeline definitions.
	checkPipelineDefinitions(&result, timeout)

	return result
}

// checkMigrations checks for pending database migrations.
// Pending migrations are a critical failure — the schema must be current before ingestion.
func checkMigrations(result *PreflightResult, timeout time.Duration) {
	cfg, err := config.LoadConfig()
	if err != nil {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:   "Migrations",
			Status: "unknown",
			Error:  "could not load config",
		})
		result.Warnings = append(result.Warnings, "Migrations: could not load config to check")
		return
	}

	if cfg.Database == nil || !cfg.Database.IsConfigured() {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:   "Migrations",
			Status: "skipped",
			Error:  "no database config",
		})
		result.Warnings = append(result.Warnings, "Migrations: no database config — add 'database' section to ~/.penf/config.yaml")
		return
	}

	migDir := ""
	if cfg.Database != nil {
		migDir = cfg.Database.GetMigrationsDir()
	}
	if migDir == "" {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:   "Migrations",
			Status: "skipped",
			Error:  "no migrations_dir configured",
		})
		result.Warnings = append(result.Warnings, "Migrations: no migrations_dir in database config")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pool, err := connectToDatabase(ctx, cfg)
	if err != nil {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:     "Migrations",
			Status:   "unreachable",
			Critical: true,
			Error:    err.Error(),
		})
		result.Passed = false
		result.ExitCode = 1
		result.Message = "Preflight check failed"
		result.Failures = append(result.Failures, fmt.Sprintf("Migrations: DB unreachable — %v [critical]", err))
		return
	}
	defer pool.Close()

	pending, err := db.GetPendingMigrations(ctx, pool, migDir)
	if err != nil {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:   "Migrations",
			Status: "error",
			Error:  err.Error(),
		})
		result.Warnings = append(result.Warnings, fmt.Sprintf("Migrations: check failed — %v", err))
		return
	}

	if len(pending) == 0 {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:     "Migrations",
			Status:   "healthy",
			Critical: true,
		})
		return
	}

	// Pending migrations = critical failure
	names := make([]string, len(pending))
	for i, m := range pending {
		names[i] = m.Version
	}
	result.Passed = false
	result.ExitCode = 1
	result.Message = "Preflight check failed"
	result.Checks = append(result.Checks, PreflightCheck{
		Name:     "Migrations",
		Status:   "pending",
		Critical: true,
		Error:    fmt.Sprintf("%d pending: %s", len(pending), strings.Join(names, ", ")),
	})
	result.Failures = append(result.Failures,
		fmt.Sprintf("Migrations: %d pending — run 'penf db migrate' or 'penf deploy gateway' [critical]", len(pending)))
}

// pipelineDefinitionSummary holds the per-pipeline/tenant summary from pipeline_definitions.
type pipelineDefinitionSummary struct {
	TenantID      string
	Pipeline      string
	TotalStages   int
	EnabledStages int
}

// validatePipelineDefinitions returns pass/fail details for each pipeline/tenant pair.
// Extracted for unit testing.
func validatePipelineDefinitions(defs []pipelineDefinitionSummary) (passes []pipelineDefinitionSummary, failures []pipelineDefinitionSummary) {
	for _, d := range defs {
		if d.EnabledStages == 0 {
			failures = append(failures, d)
		} else {
			passes = append(passes, d)
		}
	}
	return
}

// checkPipelineDefinitions verifies all pipeline definitions have at least one enabled stage.
// Skipped with a warning if no database config is available.
func checkPipelineDefinitions(result *PreflightResult, timeout time.Duration) {
	cfg, err := config.LoadConfig()
	if err != nil {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:   "Pipeline Definitions",
			Status: "unknown",
			Error:  "could not load config",
		})
		result.Warnings = append(result.Warnings, "Pipeline Definitions: could not load config to check")
		return
	}

	if cfg.Database == nil || !cfg.Database.IsConfigured() {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:   "Pipeline Definitions",
			Status: "skipped",
			Error:  "no database config",
		})
		result.Warnings = append(result.Warnings, "Pipeline Definitions: no database config — add 'database' section to ~/.penf/config.yaml")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pool, err := connectToDatabase(ctx, cfg)
	if err != nil {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:     "Pipeline Definitions",
			Status:   "unreachable",
			Critical: true,
			Error:    err.Error(),
		})
		result.Passed = false
		result.ExitCode = 1
		result.Message = "Preflight check failed"
		result.Failures = append(result.Failures, fmt.Sprintf("Pipeline Definitions: DB unreachable — %v [critical]", err))
		return
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, `
		SELECT tenant_id, pipeline,
		       COUNT(*) AS total_stages,
		       COUNT(*) FILTER (WHERE enabled) AS enabled_stages
		FROM pipeline_definitions
		GROUP BY tenant_id, pipeline
		ORDER BY tenant_id, pipeline
	`)
	if err != nil {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:   "Pipeline Definitions",
			Status: "error",
			Error:  err.Error(),
		})
		result.Warnings = append(result.Warnings, fmt.Sprintf("Pipeline Definitions: query failed — %v", err))
		return
	}
	defer rows.Close()

	var defs []pipelineDefinitionSummary
	for rows.Next() {
		var d pipelineDefinitionSummary
		if err := rows.Scan(&d.TenantID, &d.Pipeline, &d.TotalStages, &d.EnabledStages); err != nil {
			result.Checks = append(result.Checks, PreflightCheck{
				Name:   "Pipeline Definitions",
				Status: "error",
				Error:  err.Error(),
			})
			result.Warnings = append(result.Warnings, fmt.Sprintf("Pipeline Definitions: scan failed — %v", err))
			return
		}
		defs = append(defs, d)
	}
	if err := rows.Err(); err != nil {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:   "Pipeline Definitions",
			Status: "error",
			Error:  err.Error(),
		})
		result.Warnings = append(result.Warnings, fmt.Sprintf("Pipeline Definitions: iteration failed — %v", err))
		return
	}

	if len(defs) == 0 {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:   "Pipeline Definitions",
			Status: "skipped",
			Error:  "no pipeline definitions found",
		})
		result.Warnings = append(result.Warnings, "Pipeline Definitions: no pipeline definitions found in database")
		return
	}

	passes, failures := validatePipelineDefinitions(defs)

	for _, d := range passes {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:     fmt.Sprintf("Pipeline %s/%s", d.TenantID, d.Pipeline),
			Status:   "healthy",
			Critical: true,
		})
	}
	for _, d := range failures {
		result.Checks = append(result.Checks, PreflightCheck{
			Name:     fmt.Sprintf("Pipeline %s/%s", d.TenantID, d.Pipeline),
			Status:   "unhealthy",
			Critical: true,
			Error:    fmt.Sprintf("0 enabled stages (total: %d)", d.TotalStages),
		})
		result.Passed = false
		result.ExitCode = 1
		result.Message = "Preflight check failed"
		result.Failures = append(result.Failures,
			fmt.Sprintf("Pipeline %s/%s: no enabled stages (total: %d) [critical]", d.TenantID, d.Pipeline, d.TotalStages))
	}
}

// checkGatewayHealth checks the gateway health endpoint.
func checkGatewayHealth(ctx context.Context, url string) (*GatewayHealthStatus, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		return nil, latency, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, latency, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, latency, fmt.Errorf("read response: %w", err)
	}

	var status GatewayHealthStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, latency, fmt.Errorf("parse response: %w", err)
	}

	return &status, latency, nil
}

// checkAICoordinatorHealth checks the AI coordinator health endpoint.
func checkAICoordinatorHealth(ctx context.Context, url string) (*AICoordinatorHealthStatus, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		return nil, latency, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, latency, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, latency, fmt.Errorf("read response: %w", err)
	}

	var status AICoordinatorHealthStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, latency, fmt.Errorf("parse response: %w", err)
	}

	return &status, latency, nil
}

// outputPreflightHuman outputs preflight results in human-readable format.
func outputPreflightHuman(result PreflightResult) {
	// Overall status.
	statusStr := "PASS"
	statusColor := "\033[32m" // Green
	if !result.Passed {
		statusStr = "FAIL"
		statusColor = "\033[31m" // Red
	}
	fmt.Printf("Preflight Check: %s%s\033[0m\n", statusColor, statusStr)

	// Service checks.
	for _, check := range result.Checks {
		// Format status with color.
		status := check.Status
		switch check.Status {
		case "healthy":
			status = "\033[32mhealthy\033[0m"
		case "degraded":
			status = "\033[33mdegraded\033[0m"
		case "unhealthy", "unreachable":
			status = "\033[31m" + check.Status + "\033[0m"
		}

		// Format latency.
		latencyStr := ""
		if check.LatencyMs > 0 {
			latencyStr = fmt.Sprintf(" (%dms)", check.LatencyMs)
		}

		// Format circuit state.
		circuitStr := ""
		if check.CircuitState != "" {
			circuitStr = fmt.Sprintf(" (circuit: %s)", check.CircuitState)
		}

		// Critical marker.
		criticalStr := ""
		if check.Critical {
			criticalStr = " [critical]"
		}

		// Error message.
		errorStr := ""
		if check.Error != "" && check.Status == "unreachable" {
			errorStr = fmt.Sprintf(" — %s", check.Error)
		}

		fmt.Printf("  %-18s %s%s%s%s%s\n", check.Name+":", status, latencyStr, circuitStr, criticalStr, errorStr)
	}

	// Warnings.
	if len(result.Warnings) > 0 {
		fmt.Println()
		for _, warning := range result.Warnings {
			fmt.Printf("\033[33m⚠\033[0m  %s\n", warning)
		}
	}

	// Failures.
	if len(result.Failures) > 0 {
		fmt.Println()
		for _, failure := range result.Failures {
			fmt.Printf("\033[31m✗\033[0m  %s\n", failure)
		}
	}
}
