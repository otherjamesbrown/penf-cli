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

// GatewayHealthStatus represents the health status from the gateway.
type GatewayHealthStatus struct {
	Status    string                 `json:"status"`
	Services  []GatewayServiceHealth `json:"services"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version,omitempty"`
	Uptime    string                 `json:"uptime,omitempty"`
}

// GatewayServiceHealth represents the health status of a single service.
type GatewayServiceHealth struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	Message      string `json:"message,omitempty"`
	LatencyMs    int64  `json:"latency_ms"`
	CircuitState string `json:"circuit_state"`
	Critical     bool   `json:"critical"`
	Error        string `json:"error,omitempty"`
}

var (
	healthGatewayTimeout time.Duration
	healthGatewayJSON    bool
	healthGatewayURL     string
)

// NewHealthGatewayCommand creates the health gateway command.
func NewHealthGatewayCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Check health of all services via gateway",
		Long: `Check the health status of all services by querying the gateway's aggregated health endpoint.

This command fetches the gateway's /health endpoint which aggregates health status from:
  - Database (PostgreSQL)
  - ML Services (embeddings, LLM) - if configured
  - Worker health endpoint - if configured

The gateway must be running and accessible. By default, uses the gateway HTTP port
derived from the configured server address.

Environment variables:
  - GATEWAY_HEALTH_URL: Override the health endpoint URL (default: http://<server>:8080/health)`,
		RunE: runHealthGateway,
	}

	cmd.Flags().DurationVar(&healthGatewayTimeout, "timeout", 10*time.Second, "Timeout for health check")
	cmd.Flags().BoolVar(&healthGatewayJSON, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&healthGatewayURL, "url", "", "Override gateway health URL")

	return cmd
}

func runHealthGateway(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), healthGatewayTimeout)
	defer cancel()

	// Determine the health URL.
	healthURL := healthGatewayURL
	if healthURL == "" {
		healthURL = os.Getenv("GATEWAY_HEALTH_URL")
	}
	if healthURL == "" {
		// Try to derive from server address in config.
		// The server address is host:grpcPort, we need host:httpPort (8080).
		serverAddr := os.Getenv("PENF_SERVER_ADDRESS")
		if serverAddr == "" {
			// Fall back to config file default.
			serverAddr = "dev02.brown.chat:50051"
		}
		// Extract host from server address.
		host := serverAddr
		if idx := strings.LastIndex(serverAddr, ":"); idx != -1 {
			host = serverAddr[:idx]
		}
		healthURL = fmt.Sprintf("http://%s:8080/health", host)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var status GatewayHealthStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	// Output.
	if healthGatewayJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	return outputHealthGatewayHuman(status, healthURL, latency)
}

func outputHealthGatewayHuman(status GatewayHealthStatus, url string, latency time.Duration) error {
	// Overall status with color.
	statusColor := "\033[32m" // Green
	switch status.Status {
	case "unhealthy":
		statusColor = "\033[31m" // Red
	case "degraded":
		statusColor = "\033[33m" // Yellow
	}
	fmt.Printf("Gateway Health: %s%s\033[0m\n", statusColor, strings.ToUpper(status.Status))
	fmt.Printf("URL: %s\n", url)
	fmt.Printf("Response Time: %s\n", latency.Round(time.Millisecond))
	if status.Version != "" {
		fmt.Printf("Version: %s\n", status.Version)
	}
	if status.Uptime != "" {
		fmt.Printf("Uptime: %s\n", status.Uptime)
	}
	fmt.Printf("Timestamp: %s\n\n", status.Timestamp.Format(time.RFC3339))

	if len(status.Services) == 0 {
		fmt.Println("No backend services registered.")
		return nil
	}

	fmt.Println("SERVICE      STATUS     LATENCY    CIRCUIT    DETAILS")
	fmt.Println("-------      ------     -------    -------    -------")

	for _, svc := range status.Services {
		statusStr := svc.Status
		switch svc.Status {
		case "healthy":
			statusStr = "\033[32mhealthy\033[0m"
		case "degraded":
			statusStr = "\033[33mdegraded\033[0m"
		default:
			statusStr = "\033[31m" + svc.Status + "\033[0m"
		}

		latencyStr := "-"
		if svc.LatencyMs > 0 {
			latencyStr = fmt.Sprintf("%dms", svc.LatencyMs)
		}

		circuitStr := svc.CircuitState
		if circuitStr == "" {
			circuitStr = "-"
		}

		details := svc.Message
		if svc.Error != "" {
			details = svc.Error
		}

		critical := ""
		if svc.Critical {
			critical = " [critical]"
		}

		fmt.Printf("%-12s %-18s %-10s %-10s %s%s\n", svc.Name, statusStr, latencyStr, circuitStr, details, critical)
	}

	return nil
}
