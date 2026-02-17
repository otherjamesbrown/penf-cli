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

	return result
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
