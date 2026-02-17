// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestPreflightAllHealthy tests the preflight check when all services are healthy.
func TestPreflightAllHealthy(t *testing.T) {
	// Create mock gateway server.
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := GatewayHealthStatus{
			Status: "healthy",
			Services: []GatewayServiceHealth{
				{Name: "database", Status: "healthy", LatencyMs: 1, CircuitState: "closed", Critical: true},
				{Name: "temporal", Status: "healthy", LatencyMs: 2, CircuitState: "closed", Critical: false},
				{Name: "embeddings", Status: "healthy", LatencyMs: 3, CircuitState: "closed", Critical: false},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer gatewayServer.Close()

	// Create mock AI coordinator server.
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := AICoordinatorHealthStatus{
			Status: "healthy",
			Checks: map[string]AICoordinatorCheck{
				"gemini_llm":      {Status: "healthy", DurationMs: 100},
				"grpc_server":     {Status: "healthy", DurationMs: 0},
				"mlx_embeddings":  {Status: "healthy", DurationMs: 5},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer aiServer.Close()

	// Run preflight check. The server URLs already include the full path.
	result := runPreflightCheck(gatewayServer.URL, aiServer.URL, 5*time.Second)

	// Verify success.
	if !result.Passed {
		t.Errorf("Expected preflight to pass, but it failed: %s", result.Message)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	if len(result.Failures) > 0 {
		t.Errorf("Expected no failures, got %d: %v", len(result.Failures), result.Failures)
	}
}

// TestPreflightCriticalServiceUnhealthy tests when a critical service is unhealthy.
func TestPreflightCriticalServiceUnhealthy(t *testing.T) {
	// Create mock gateway server with unhealthy database.
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := GatewayHealthStatus{
			Status: "unhealthy",
			Services: []GatewayServiceHealth{
				{Name: "database", Status: "unhealthy", LatencyMs: 0, CircuitState: "closed", Critical: true, Error: "connection refused"},
				{Name: "temporal", Status: "healthy", LatencyMs: 2, CircuitState: "closed", Critical: false},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer gatewayServer.Close()

	// Create healthy AI coordinator.
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := AICoordinatorHealthStatus{
			Status: "healthy",
			Checks: map[string]AICoordinatorCheck{
				"gemini_llm": {Status: "healthy", DurationMs: 100},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer aiServer.Close()

	// Run preflight check.
	result := runPreflightCheck(gatewayServer.URL, aiServer.URL, 5*time.Second)

	// Verify failure.
	if result.Passed {
		t.Error("Expected preflight to fail due to critical service unhealthy")
	}
	if result.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", result.ExitCode)
	}
	if len(result.Failures) == 0 {
		t.Error("Expected at least one failure")
	}
	// Check that database failure is reported.
	found := false
	for _, failure := range result.Failures {
		if strings.Contains(failure, "database") && strings.Contains(failure, "unhealthy") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected database failure in results: %v", result.Failures)
	}
}

// TestPreflightCircuitBreakerOpen tests when a circuit breaker is open on a critical service.
func TestPreflightCircuitBreakerOpen(t *testing.T) {
	// Create mock gateway server with open circuit breaker.
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := GatewayHealthStatus{
			Status: "degraded",
			Services: []GatewayServiceHealth{
				{Name: "database", Status: "healthy", LatencyMs: 1, CircuitState: "open", Critical: true},
				{Name: "temporal", Status: "healthy", LatencyMs: 2, CircuitState: "closed", Critical: false},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer gatewayServer.Close()

	// Create healthy AI coordinator.
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := AICoordinatorHealthStatus{
			Status: "healthy",
			Checks: map[string]AICoordinatorCheck{
				"gemini_llm": {Status: "healthy", DurationMs: 100},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer aiServer.Close()

	// Run preflight check.
	result := runPreflightCheck(gatewayServer.URL, aiServer.URL, 5*time.Second)

	// Verify failure due to open circuit.
	if result.Passed {
		t.Error("Expected preflight to fail due to open circuit breaker")
	}
	if result.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", result.ExitCode)
	}
	// Check that circuit breaker failure is reported.
	found := false
	for _, failure := range result.Failures {
		if strings.Contains(failure, "circuit") && strings.Contains(failure, "open") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected circuit breaker failure in results: %v", result.Failures)
	}
}

// TestPreflightNonCriticalUnhealthy tests when a non-critical service is unhealthy.
func TestPreflightNonCriticalUnhealthy(t *testing.T) {
	// Create mock gateway server with unhealthy non-critical service.
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := GatewayHealthStatus{
			Status: "degraded",
			Services: []GatewayServiceHealth{
				{Name: "database", Status: "healthy", LatencyMs: 1, CircuitState: "closed", Critical: true},
				{Name: "embeddings", Status: "unhealthy", LatencyMs: 0, CircuitState: "closed", Critical: false, Error: "timeout"},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer gatewayServer.Close()

	// Create healthy AI coordinator.
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := AICoordinatorHealthStatus{
			Status: "healthy",
			Checks: map[string]AICoordinatorCheck{
				"gemini_llm": {Status: "healthy", DurationMs: 100},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer aiServer.Close()

	// Run preflight check.
	result := runPreflightCheck(gatewayServer.URL, aiServer.URL, 5*time.Second)

	// Verify success (non-critical failures don't block).
	if !result.Passed {
		t.Errorf("Expected preflight to pass with warnings, but it failed: %s", result.Message)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	// Should have warnings.
	if len(result.Warnings) == 0 {
		t.Error("Expected warnings for non-critical service failure")
	}
	// Check that embeddings warning is present.
	found := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "embeddings") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected embeddings warning in results: %v", result.Warnings)
	}
}

// TestPreflightGatewayUnreachable tests when the gateway is unreachable.
func TestPreflightGatewayUnreachable(t *testing.T) {
	// Use an invalid URL that will fail to connect.
	gatewayURL := "http://localhost:1" // Port 1 is typically unreachable.
	aiURL := "http://localhost:1"

	// Run preflight check.
	result := runPreflightCheck(gatewayURL, aiURL, 1*time.Second)

	// Verify failure.
	if result.Passed {
		t.Error("Expected preflight to fail when gateway is unreachable")
	}
	if result.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", result.ExitCode)
	}
	if len(result.Failures) == 0 {
		t.Error("Expected at least one failure")
	}
}

// TestPreflightAICoordinatorUnreachable tests when AI coordinator is unreachable.
func TestPreflightAICoordinatorUnreachable(t *testing.T) {
	// Create healthy gateway server.
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := GatewayHealthStatus{
			Status: "healthy",
			Services: []GatewayServiceHealth{
				{Name: "database", Status: "healthy", LatencyMs: 1, CircuitState: "closed", Critical: true},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer gatewayServer.Close()

	// Use unreachable AI coordinator.
	aiURL := "http://localhost:1"

	// Run preflight check.
	result := runPreflightCheck(gatewayServer.URL, aiURL, 1*time.Second)

	// Verify failure.
	if result.Passed {
		t.Error("Expected preflight to fail when AI coordinator is unreachable")
	}
	if result.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", result.ExitCode)
	}
	// Check for AI coordinator failure.
	found := false
	for _, failure := range result.Failures {
		if strings.Contains(strings.ToLower(failure), "coordinator") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected AI coordinator failure in results: %v", result.Failures)
	}
}

// TestPreflightJSONOutput tests JSON output format.
func TestPreflightJSONOutput(t *testing.T) {
	// Create mock gateway server.
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := GatewayHealthStatus{
			Status: "healthy",
			Services: []GatewayServiceHealth{
				{Name: "database", Status: "healthy", LatencyMs: 1, CircuitState: "closed", Critical: true},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer gatewayServer.Close()

	// Create mock AI coordinator server.
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := AICoordinatorHealthStatus{
			Status: "healthy",
			Checks: map[string]AICoordinatorCheck{
				"gemini_llm": {Status: "healthy", DurationMs: 100},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer aiServer.Close()

	// Run preflight check.
	result := runPreflightCheck(gatewayServer.URL, aiServer.URL, 5*time.Second)

	// Convert result to JSON and verify it can be marshaled.
	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result to JSON: %v", err)
	}

	// Verify JSON is valid and can be unmarshaled.
	var decoded PreflightResult
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify key fields.
	if decoded.Passed != result.Passed {
		t.Errorf("Decoded Passed = %v, want %v", decoded.Passed, result.Passed)
	}
	if decoded.ExitCode != result.ExitCode {
		t.Errorf("Decoded ExitCode = %v, want %v", decoded.ExitCode, result.ExitCode)
	}
}

// TestPreflightHalfOpenCircuit tests when a circuit breaker is half-open (warning).
func TestPreflightHalfOpenCircuit(t *testing.T) {
	// Create mock gateway server with half-open circuit on non-critical service.
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := GatewayHealthStatus{
			Status: "degraded",
			Services: []GatewayServiceHealth{
				{Name: "database", Status: "healthy", LatencyMs: 1, CircuitState: "closed", Critical: true},
				{Name: "embeddings", Status: "degraded", LatencyMs: 500, CircuitState: "half-open", Critical: false},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer gatewayServer.Close()

	// Create healthy AI coordinator.
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := AICoordinatorHealthStatus{
			Status: "healthy",
			Checks: map[string]AICoordinatorCheck{
				"gemini_llm": {Status: "healthy", DurationMs: 100},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer aiServer.Close()

	// Run preflight check.
	result := runPreflightCheck(gatewayServer.URL, aiServer.URL, 5*time.Second)

	// Verify success with warnings.
	if !result.Passed {
		t.Errorf("Expected preflight to pass with warnings, got failure: %s", result.Message)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	// Should have warnings for half-open circuit.
	if len(result.Warnings) == 0 {
		t.Error("Expected warnings for half-open circuit breaker")
	}
}

// TestPreflightTimeout tests that the check respects timeout.
func TestPreflightTimeout(t *testing.T) {
	// Create a slow gateway server.
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Sleep longer than timeout.
		status := GatewayHealthStatus{
			Status: "healthy",
			Services: []GatewayServiceHealth{
				{Name: "database", Status: "healthy", LatencyMs: 1, CircuitState: "closed", Critical: true},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer gatewayServer.Close()

	aiURL := "http://localhost:8090" // Won't be reached due to timeout.

	// Run preflight check with short timeout.
	result := runPreflightCheck(gatewayServer.URL, aiURL, 500*time.Millisecond)

	// Verify failure due to timeout.
	if result.Passed {
		t.Error("Expected preflight to fail due to timeout")
	}
	if result.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", result.ExitCode)
	}
}

// TestPreflightOutputHuman tests human-readable output.
func TestPreflightOutputHuman(t *testing.T) {
	// Create a result with mixed statuses.
	result := PreflightResult{
		Passed:   false,
		ExitCode: 1,
		Message:  "Preflight check failed",
		Failures: []string{
			"Gateway: database unhealthy (circuit: open) [critical]",
		},
		Warnings: []string{
			"Gateway: embeddings degraded (circuit: half-open)",
		},
		Checks: []PreflightCheck{
			{Name: "Gateway", Status: "healthy", LatencyMs: 100},
			{Name: "AI Coordinator", Status: "healthy", LatencyMs: 200},
			{Name: "database", Status: "unhealthy", CircuitState: "open", Critical: true},
			{Name: "embeddings", Status: "degraded", CircuitState: "half-open", Critical: false},
		},
	}

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputPreflightHuman(result)

	w.Close()
	os.Stdout = oldStdout

	// Read captured output.
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify key elements are present.
	expectedStrings := []string{
		"Preflight Check:",
		"FAIL",
		"Gateway",
		"AI Coordinator",
		"database",
		"unhealthy",
		"[critical]",
		"embeddings",
		"degraded",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", expected, output)
		}
	}
}

// TestDeriveURLs tests URL derivation from config.
func TestDeriveURLs(t *testing.T) {
	tests := []struct {
		name           string
		serverAddr     string
		expectedGW     string
		expectedAI     string
	}{
		{
			name:       "standard server address",
			serverAddr: "dev02.brown.chat:50051",
			expectedGW: "http://dev02.brown.chat:8080/health",
			expectedAI: "http://dev02.brown.chat:8090/health",
		},
		{
			name:       "localhost",
			serverAddr: "localhost:50051",
			expectedGW: "http://localhost:8080/health",
			expectedAI: "http://localhost:8090/health",
		},
		{
			name:       "IP address",
			serverAddr: "10.0.10.2:50051",
			expectedGW: "http://10.0.10.2:8080/health",
			expectedAI: "http://10.0.10.2:8090/health",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gwURL, aiURL := deriveHealthURLs(tc.serverAddr)
			if gwURL != tc.expectedGW {
				t.Errorf("Gateway URL = %q, want %q", gwURL, tc.expectedGW)
			}
			if aiURL != tc.expectedAI {
				t.Errorf("AI Coordinator URL = %q, want %q", aiURL, tc.expectedAI)
			}
		})
	}
}

// MockPreflightResult creates a sample result for testing output functions.
func mockPreflightResult() PreflightResult {
	return PreflightResult{
		Passed:   true,
		ExitCode: 0,
		Message:  "All critical services healthy",
		Failures: []string{},
		Warnings: []string{"Gateway: embeddings degraded (circuit: half-open)"},
		Checks: []PreflightCheck{
			{Name: "Gateway", Status: "healthy", LatencyMs: 100},
			{Name: "AI Coordinator", Status: "healthy", LatencyMs: 150},
			{Name: "database", Status: "healthy", CircuitState: "closed", Critical: true},
			{Name: "temporal", Status: "healthy", CircuitState: "closed", Critical: false},
			{Name: "embeddings", Status: "degraded", CircuitState: "half-open", Critical: false},
		},
	}
}

// TestPreflightResultJSON tests JSON serialization of preflight result.
func TestPreflightResultJSON(t *testing.T) {
	result := mockPreflightResult()

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	// Verify JSON contains expected keys.
	jsonStr := string(jsonData)
	expectedKeys := []string{
		`"passed"`,
		`"exit_code"`,
		`"message"`,
		`"checks"`,
		`"warnings"`,
	}

	for _, key := range expectedKeys {
		if !strings.Contains(jsonStr, key) {
			t.Errorf("JSON missing expected key %s.\nJSON:\n%s", key, jsonStr)
		}
	}
}

// PreflightTestHelper validates a preflight check result matches expected criteria.
func validatePreflightResult(t *testing.T, result PreflightResult, wantPassed bool, wantExitCode int) {
	t.Helper()
	if result.Passed != wantPassed {
		t.Errorf("Passed = %v, want %v. Message: %s", result.Passed, wantPassed, result.Message)
	}
	if result.ExitCode != wantExitCode {
		t.Errorf("ExitCode = %d, want %d. Message: %s", result.ExitCode, wantExitCode, result.Message)
	}
}

// TestPreflightMultipleCriticalFailures tests handling of multiple critical failures.
func TestPreflightMultipleCriticalFailures(t *testing.T) {
	// Create mock gateway with multiple critical failures.
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := GatewayHealthStatus{
			Status: "unhealthy",
			Services: []GatewayServiceHealth{
				{Name: "database", Status: "unhealthy", LatencyMs: 0, CircuitState: "open", Critical: true, Error: "connection failed"},
			},
			Timestamp: time.Now(),
		}
		json.NewEncoder(w).Encode(status)
	}))
	defer gatewayServer.Close()

	// AI coordinator also fails.
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, "Service unavailable")
	}))
	defer aiServer.Close()

	// Run preflight check.
	result := runPreflightCheck(gatewayServer.URL, aiServer.URL, 5*time.Second)

	// Verify failure with multiple issues.
	validatePreflightResult(t, result, false, 1)

	// Should have multiple failures reported.
	if len(result.Failures) < 2 {
		t.Errorf("Expected at least 2 failures, got %d: %v", len(result.Failures), result.Failures)
	}
}
