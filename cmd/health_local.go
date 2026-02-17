// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// LocalHealthStatus represents the health status of local services.
type LocalHealthStatus struct {
	Overall   string                        `json:"overall"`
	Timestamp time.Time                     `json:"timestamp"`
	Services  map[string]LocalServiceStatus `json:"services"`
}

// LocalServiceStatus represents the status of a single local service.
type LocalServiceStatus struct {
	Status  string `json:"status"`
	URL     string `json:"url"`
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
	Details string `json:"details,omitempty"`
}

var (
	healthLocalTimeout time.Duration
	healthLocalJSON    bool
)

// NewHealthLocalCommand creates the health local command.
func NewHealthLocalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Check health of local ML services",
		Long: `Check the health status of local ML services running on dev01.

This command directly probes the local services without going through the gateway.
Useful for verifying the ML infrastructure is running before starting work.

Services checked:
  - Ollama Embeddings (localhost:11434)
  - Worker health endpoint (localhost:8085)

Environment variables for custom URLs:
  - AI_SERVICE_URL: Embeddings service URL (default: http://localhost:11434)
  - WORKER_HEALTH_URL: Worker health URL (default: http://localhost:8085)`,
		RunE: runHealthLocal,
	}

	cmd.Flags().DurationVar(&healthLocalTimeout, "timeout", 5*time.Second, "Timeout for health checks")
	cmd.Flags().BoolVar(&healthLocalJSON, "json", false, "Output as JSON")

	return cmd
}

func runHealthLocal(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), healthLocalTimeout*3)
	defer cancel()

	// Get service URLs from environment or use defaults
	embeddingsURL := os.Getenv("AI_SERVICE_URL")
	if embeddingsURL == "" {
		embeddingsURL = "http://localhost:11434"
	}

	workerURL := os.Getenv("WORKER_HEALTH_URL")
	if workerURL == "" {
		workerURL = "http://localhost:8085"
	}

	status := LocalHealthStatus{
		Timestamp: time.Now(),
		Services:  make(map[string]LocalServiceStatus),
	}

	allHealthy := true

	// Check embeddings service (Ollama)
	embStatus := checkEmbeddings(ctx, embeddingsURL)
	status.Services["embeddings"] = embStatus
	if embStatus.Status != "healthy" {
		allHealthy = false
	}

	// Check worker health endpoint
	workerStatus := checkWorker(ctx, workerURL)
	status.Services["worker"] = workerStatus
	if workerStatus.Status != "healthy" {
		allHealthy = false
	}

	if allHealthy {
		status.Overall = "healthy"
	} else {
		status.Overall = "unhealthy"
	}

	// Output
	if healthLocalJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	return outputHealthLocalHuman(status)
}

func checkEmbeddings(ctx context.Context, baseURL string) LocalServiceStatus {
	url := baseURL + "/"
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return LocalServiceStatus{
			Status: "error",
			URL:    baseURL,
			Error:  fmt.Sprintf("create request: %v", err),
		}
	}

	resp, err := http.DefaultClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		return LocalServiceStatus{
			Status: "unhealthy",
			URL:    baseURL,
			Error:  err.Error(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return LocalServiceStatus{
			Status:  "unhealthy",
			URL:     baseURL,
			Latency: latency.Round(time.Millisecond).String(),
			Error:   fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
	}

	// Parse response to get model info
	body, _ := io.ReadAll(resp.Body)
	var healthResp struct {
		Status      string `json:"status"`
		Model       string `json:"model"`
		ModelLoaded bool   `json:"model_loaded"`
	}
	if json.Unmarshal(body, &healthResp) == nil && healthResp.Model != "" {
		return LocalServiceStatus{
			Status:  "healthy",
			URL:     baseURL,
			Latency: latency.Round(time.Millisecond).String(),
			Details: fmt.Sprintf("model=%s", healthResp.Model),
		}
	}

	return LocalServiceStatus{
		Status:  "healthy",
		URL:     baseURL,
		Latency: latency.Round(time.Millisecond).String(),
	}
}

func checkWorker(ctx context.Context, baseURL string) LocalServiceStatus {
	url := baseURL + "/health"
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return LocalServiceStatus{
			Status: "error",
			URL:    baseURL,
			Error:  fmt.Sprintf("create request: %v", err),
		}
	}

	resp, err := http.DefaultClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		return LocalServiceStatus{
			Status: "unhealthy",
			URL:    baseURL,
			Error:  err.Error(),
		}
	}
	defer resp.Body.Close()

	// Parse response
	body, _ := io.ReadAll(resp.Body)
	var healthResp struct {
		Status string                 `json:"status"`
		Checks map[string]interface{} `json:"checks"`
	}
	json.Unmarshal(body, &healthResp)

	if resp.StatusCode == http.StatusServiceUnavailable {
		return LocalServiceStatus{
			Status:  "unhealthy",
			URL:     baseURL,
			Latency: latency.Round(time.Millisecond).String(),
			Error:   fmt.Sprintf("status=%s", healthResp.Status),
			Details: fmt.Sprintf("checks=%d", len(healthResp.Checks)),
		}
	}

	if resp.StatusCode != http.StatusOK {
		return LocalServiceStatus{
			Status:  "unhealthy",
			URL:     baseURL,
			Latency: latency.Round(time.Millisecond).String(),
			Error:   fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
	}

	return LocalServiceStatus{
		Status:  "healthy",
		URL:     baseURL,
		Latency: latency.Round(time.Millisecond).String(),
		Details: fmt.Sprintf("checks=%d", len(healthResp.Checks)),
	}
}

func outputHealthLocalHuman(status LocalHealthStatus) error {
	// Overall status with color
	statusColor := "\033[32m" // Green
	if status.Overall != "healthy" {
		statusColor = "\033[31m" // Red
	}
	fmt.Printf("Local Services: %s%s\033[0m\n", statusColor, status.Overall)
	fmt.Printf("Timestamp: %s\n\n", status.Timestamp.Format(time.RFC3339))

	fmt.Println("SERVICE      STATUS     LATENCY    URL                      DETAILS")
	fmt.Println("-------      ------     -------    ---                      -------")

	for name, svc := range status.Services {
		statusStr := svc.Status
		if svc.Status == "healthy" {
			statusStr = "\033[32mhealthy\033[0m"
		} else {
			statusStr = "\033[31m" + svc.Status + "\033[0m"
		}

		latency := svc.Latency
		if latency == "" {
			latency = "-"
		}

		details := svc.Details
		if svc.Error != "" {
			details = svc.Error
		}

		fmt.Printf("%-12s %-18s %-10s %-24s %s\n", name, statusStr, latency, svc.URL, details)
	}

	return nil
}
