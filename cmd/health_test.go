// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/client"
)

// TestHealthOutputJSON tests JSON output formatting for health status.
func TestHealthOutputJSON(t *testing.T) {
	status := &client.SystemStatus{
		Healthy:   true,
		Message:   "All systems operational",
		Timestamp: time.Now(),
		Services: []client.ServiceHealth{
			{
				Name:          "gateway",
				Healthy:       true,
				Status:        "running",
				Message:       "Ready",
				LatencyMs:     1.5,
				Version:       "1.0.0",
				UptimeSeconds: 3600,
			},
		},
		Database: &client.DatabaseStatus{
			Healthy:                true,
			Type:                   "postgresql",
			ConnectionStatus:       "connected",
			ActiveConnections:      5,
			MaxConnections:         100,
			VectorExtensionEnabled: true,
			ContentCount:           1000,
			EntityCount:            250,
			LatencyMs:              0.5,
		},
		Queues: &client.QueueStatus{
			Healthy:         true,
			Type:            "redis",
			TotalPending:    10,
			ProcessingRate:  50.0,
			DeadLetterCount: 0,
			QueueDepths: map[string]int64{
				"ingestion": 5,
				"embedding": 5,
			},
		},
		Version: &client.VersionInfo{
			Version:   "1.0.0",
			Commit:    "abc123",
			BuildTime: "2024-01-01T00:00:00Z",
			GoVersion: "go1.24.0",
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Failed to marshal status to JSON: %v", err)
	}

	var decoded client.SystemStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded.Healthy != status.Healthy {
		t.Errorf("Healthy = %v, want %v", decoded.Healthy, status.Healthy)
	}
	if decoded.Message != status.Message {
		t.Errorf("Message = %v, want %v", decoded.Message, status.Message)
	}
	if len(decoded.Services) != 1 {
		t.Errorf("Services count = %d, want 1", len(decoded.Services))
	}
	if decoded.Database == nil {
		t.Error("Database should not be nil")
	}
	if decoded.Queues == nil {
		t.Error("Queues should not be nil")
	}
	if decoded.Version == nil {
		t.Error("Version should not be nil")
	}
}

// TestHealthOutputYAML tests YAML output formatting for health status.
func TestHealthOutputYAML(t *testing.T) {
	status := &client.SystemStatus{
		Healthy:   true,
		Message:   "All systems operational",
		Timestamp: time.Now(),
		Services: []client.ServiceHealth{
			{
				Name:    "gateway",
				Healthy: true,
				Status:  "running",
			},
		},
	}

	data, err := yaml.Marshal(status)
	if err != nil {
		t.Fatalf("Failed to marshal status to YAML: %v", err)
	}

	var decoded client.SystemStatus
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	if decoded.Healthy != status.Healthy {
		t.Errorf("Healthy = %v, want %v", decoded.Healthy, status.Healthy)
	}
}

// TestServiceHealth tests the ServiceHealth struct serialization.
func TestServiceHealth(t *testing.T) {
	tests := []struct {
		name    string
		health  client.ServiceHealth
		healthy bool
	}{
		{
			name: "healthy service",
			health: client.ServiceHealth{
				Name:          "test-service",
				Healthy:       true,
				Status:        "running",
				Message:       "Service is healthy",
				LatencyMs:     1.0,
				Version:       "1.0.0",
				UptimeSeconds: 3600,
			},
			healthy: true,
		},
		{
			name: "unhealthy service",
			health: client.ServiceHealth{
				Name:      "failing-service",
				Healthy:   false,
				Status:    "error",
				Message:   "Connection refused",
				LatencyMs: 0,
			},
			healthy: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.health)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			var decoded client.ServiceHealth
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if decoded.Healthy != tc.healthy {
				t.Errorf("Healthy = %v, want %v", decoded.Healthy, tc.healthy)
			}
			if decoded.Name != tc.health.Name {
				t.Errorf("Name = %v, want %v", decoded.Name, tc.health.Name)
			}
		})
	}
}

// TestDatabaseStatus tests the DatabaseStatus struct serialization.
func TestDatabaseStatus(t *testing.T) {
	status := &client.DatabaseStatus{
		Healthy:                true,
		Type:                   "postgresql",
		ConnectionStatus:       "connected",
		ActiveConnections:      10,
		MaxConnections:         100,
		VectorExtensionEnabled: true,
		ContentCount:           5000,
		EntityCount:            1200,
		LatencyMs:              0.8,
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded client.DatabaseStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Type != "postgresql" {
		t.Errorf("Type = %v, want postgresql", decoded.Type)
	}
	if decoded.ActiveConnections != 10 {
		t.Errorf("ActiveConnections = %v, want 10", decoded.ActiveConnections)
	}
	if !decoded.VectorExtensionEnabled {
		t.Error("VectorExtensionEnabled should be true")
	}
}

// TestQueueStatus tests the QueueStatus struct serialization.
func TestQueueStatus(t *testing.T) {
	status := &client.QueueStatus{
		Healthy:         true,
		Type:            "redis",
		TotalPending:    25,
		ProcessingRate:  100.5,
		DeadLetterCount: 3,
		QueueDepths: map[string]int64{
			"ingestion":  10,
			"embedding":  8,
			"extraction": 7,
		},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded client.QueueStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.TotalPending != 25 {
		t.Errorf("TotalPending = %v, want 25", decoded.TotalPending)
	}
	if decoded.DeadLetterCount != 3 {
		t.Errorf("DeadLetterCount = %v, want 3", decoded.DeadLetterCount)
	}
	if len(decoded.QueueDepths) != 3 {
		t.Errorf("QueueDepths count = %v, want 3", len(decoded.QueueDepths))
	}
}

// TestVersionInfo tests the VersionInfo struct serialization.
func TestVersionInfo(t *testing.T) {
	info := &client.VersionInfo{
		Version:   "2.1.0",
		Commit:    "deadbeef",
		BuildTime: "2024-06-15T12:00:00Z",
		GoVersion: "go1.24.0",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded client.VersionInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Version != "2.1.0" {
		t.Errorf("Version = %v, want 2.1.0", decoded.Version)
	}
	if decoded.Commit != "deadbeef" {
		t.Errorf("Commit = %v, want deadbeef", decoded.Commit)
	}
}

// NOTE: The following tests are disabled because they reference functions
// defined in main.go which are not accessible from the cmd package.
// To enable, move to main_test.go or export the helper functions.

/*
// TestBoolToStatus tests the boolToStatus helper function.
func TestBoolToStatus(t *testing.T) {
	tests := []struct {
		healthy  bool
		expected string
	}{
		{true, "HEALTHY"},
		{false, "UNHEALTHY"},
	}

	for _, tc := range tests {
		result := boolToStatus(tc.healthy)
		if result != tc.expected {
			t.Errorf("boolToStatus(%v) = %v, want %v", tc.healthy, result, tc.expected)
		}
	}
}

// TestBoolToEnabled tests the boolToEnabled helper function.
func TestBoolToEnabled(t *testing.T) {
	tests := []struct {
		enabled  bool
		expected string
	}{
		{true, "enabled"},
		{false, "disabled"},
	}

	for _, tc := range tests {
		result := boolToEnabled(tc.enabled)
		if result != tc.expected {
			t.Errorf("boolToEnabled(%v) = %v, want %v", tc.enabled, result, tc.expected)
		}
	}
}

// TestValueOrDefault tests the valueOrDefault helper function.
func TestValueOrDefault(t *testing.T) {
	tests := []struct {
		value        string
		defaultValue string
		expected     string
	}{
		{"hello", "default", "hello"},
		{"", "default", "default"},
		{"value", "", "value"},
		{"", "", ""},
	}

	for _, tc := range tests {
		result := valueOrDefault(tc.value, tc.defaultValue)
		if result != tc.expected {
			t.Errorf("valueOrDefault(%q, %q) = %q, want %q",
				tc.value, tc.defaultValue, result, tc.expected)
		}
	}
}

// TestStatusWithColor tests the statusWithColor helper function.
func TestStatusWithColor(t *testing.T) {
	t.Run("healthy with status", func(t *testing.T) {
		result := statusWithColor(true, "running")
		if !strings.Contains(result, "running") {
			t.Errorf("expected result to contain 'running', got %q", result)
		}
		if !strings.Contains(result, "\033[32m") {
			t.Errorf("expected green color code, got %q", result)
		}
	})

	t.Run("healthy without status", func(t *testing.T) {
		result := statusWithColor(true, "")
		if !strings.Contains(result, "healthy") {
			t.Errorf("expected result to contain 'healthy', got %q", result)
		}
	})

	t.Run("unhealthy with status", func(t *testing.T) {
		result := statusWithColor(false, "error")
		if !strings.Contains(result, "error") {
			t.Errorf("expected result to contain 'error', got %q", result)
		}
		if !strings.Contains(result, "\033[31m") {
			t.Errorf("expected red color code, got %q", result)
		}
	})

	t.Run("unhealthy without status", func(t *testing.T) {
		result := statusWithColor(false, "")
		if !strings.Contains(result, "unhealthy") {
			t.Errorf("expected result to contain 'unhealthy', got %q", result)
		}
	})
}

// TestGetMockStatus tests the getMockStatus function.
func TestGetMockStatus(t *testing.T) {
	t.Run("verbose false", func(t *testing.T) {
		status := getMockStatus(false)
		if status == nil {
			t.Fatal("getMockStatus returned nil")
		}
		if !status.Healthy {
			t.Error("expected healthy status")
		}
		if len(status.Services) == 0 {
			t.Error("expected at least one service")
		}
		if status.Database == nil {
			t.Error("expected database status")
		}
		if status.Queues == nil {
			t.Error("expected queue status")
		}
		if status.Version == nil {
			t.Error("expected version info")
		}
	})

	t.Run("verbose true", func(t *testing.T) {
		status := getMockStatus(true)
		if status == nil {
			t.Fatal("getMockStatus returned nil")
		}
		// Verbose mode should also return full status
		if !status.Healthy {
			t.Error("expected healthy status")
		}
	})
}

// TestSystemStatusTimestamp tests that timestamps are properly set.
func TestSystemStatusTimestamp(t *testing.T) {
	before := time.Now()
	status := getMockStatus(false)
	after := time.Now()

	if status.Timestamp.Before(before) || status.Timestamp.After(after) {
		t.Errorf("Timestamp %v is not between %v and %v",
			status.Timestamp, before, after)
	}
}

// TestOutputStatus tests the outputStatus function routing.
func TestOutputStatus(t *testing.T) {
	status := &client.SystemStatus{
		Healthy:   true,
		Message:   "Test status",
		Timestamp: time.Now(),
		Services: []client.ServiceHealth{
			{Name: "test", Healthy: true, Status: "running"},
		},
	}

	// We need to set up the global cfg for this test
	// Save original and restore after test
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	tests := []struct {
		name   string
		format config.OutputFormat
	}{
		{"text format", config.OutputFormatText},
		{"json format", config.OutputFormatJSON},
		{"yaml format", config.OutputFormatYAML},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg = &config.CLIConfig{
				OutputFormat: tc.format,
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := outputStatus(status)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			if err != nil {
				t.Fatalf("outputStatus failed: %v", err)
			}

			// Verify output is not empty
			if len(output) == 0 {
				t.Error("expected non-empty output")
			}
		})
	}
}

// TestOutputHealthHuman tests human-readable health output.
func TestOutputHealthHuman(t *testing.T) {
	status := &client.SystemStatus{
		Healthy:   true,
		Message:   "All systems operational",
		Timestamp: time.Now(),
		Services: []client.ServiceHealth{
			{
				Name:      "gateway",
				Healthy:   true,
				Status:    "running",
				LatencyMs: 1.5,
				Version:   "1.0.0",
			},
			{
				Name:      "orchestrator",
				Healthy:   true,
				Status:    "running",
				LatencyMs: 2.0,
				Version:   "1.0.0",
			},
		},
		Database: &client.DatabaseStatus{
			Healthy:                true,
			Type:                   "postgresql",
			ConnectionStatus:       "connected",
			ActiveConnections:      5,
			MaxConnections:         100,
			VectorExtensionEnabled: true,
			ContentCount:           1000,
			EntityCount:            250,
			LatencyMs:              0.5,
		},
		Queues: &client.QueueStatus{
			Healthy:         true,
			Type:            "redis",
			TotalPending:    10,
			ProcessingRate:  50.0,
			DeadLetterCount: 0,
			QueueDepths: map[string]int64{
				"ingestion": 5,
				"embedding": 5,
			},
		},
		Version: &client.VersionInfo{
			Version:   "1.0.0",
			Commit:    "abc123",
			BuildTime: "2024-01-01T00:00:00Z",
			GoVersion: "go1.24.0",
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputHealthHuman(status)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("outputHealthHuman failed: %v", err)
	}

	// Verify key elements are present
	expectedStrings := []string{
		"System Status:",
		"HEALTHY",
		"Services:",
		"gateway",
		"Database:",
		"postgresql",
		"Queues:",
		"redis",
		"Version:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q", expected)
		}
	}
}

// TestOutputHealthHuman_Unhealthy tests human-readable output for unhealthy status.
func TestOutputHealthHuman_Unhealthy(t *testing.T) {
	status := &client.SystemStatus{
		Healthy:   false,
		Message:   "Database connection failed",
		Timestamp: time.Now(),
		Services: []client.ServiceHealth{
			{
				Name:    "gateway",
				Healthy: true,
				Status:  "running",
			},
		},
		Database: &client.DatabaseStatus{
			Healthy:          false,
			Type:             "postgresql",
			ConnectionStatus: "disconnected",
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputHealthHuman(status)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("outputHealthHuman failed: %v", err)
	}

	// Should contain UNHEALTHY
	if !strings.Contains(output, "UNHEALTHY") {
		t.Error("expected output to contain 'UNHEALTHY'")
	}

	// Should contain red color code for unhealthy
	if !strings.Contains(output, "\033[31m") {
		t.Error("expected red color code in output")
	}
}

// TestOutputHealthHuman_NilSections tests handling of nil optional sections.
func TestOutputHealthHuman_NilSections(t *testing.T) {
	status := &client.SystemStatus{
		Healthy:   true,
		Message:   "Minimal status",
		Timestamp: time.Now(),
		Services: []client.ServiceHealth{
			{Name: "test", Healthy: true},
		},
		// Database, Queues, and Version are nil
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputHealthHuman(status)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("outputHealthHuman failed with nil sections: %v", err)
	}
}

// TestOutputHealthHuman_QueueDeadLetter tests dead letter queue display.
func TestOutputHealthHuman_QueueDeadLetter(t *testing.T) {
	status := &client.SystemStatus{
		Healthy:   true,
		Message:   "Status with dead letters",
		Timestamp: time.Now(),
		Services:  []client.ServiceHealth{},
		Queues: &client.QueueStatus{
			Healthy:         true,
			Type:            "redis",
			DeadLetterCount: 5, // Non-zero dead letters
			QueueDepths:     map[string]int64{},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputHealthHuman(status)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("outputHealthHuman failed: %v", err)
	}

	// Should show dead letter count with warning color
	if !strings.Contains(output, "Dead Letter:") {
		t.Error("expected output to contain 'Dead Letter:'")
	}
	if !strings.Contains(output, "5") {
		t.Error("expected output to contain dead letter count '5'")
	}
}

// TestGetTenantID tests the getTenantID function.
func TestGetTenantID(t *testing.T) {
	// Save original cfg and env
	originalCfg := cfg
	originalEnv := os.Getenv("PENF_TENANT_ID")
	defer func() {
		cfg = originalCfg
		if originalEnv != "" {
			os.Setenv("PENF_TENANT_ID", originalEnv)
		} else {
			os.Unsetenv("PENF_TENANT_ID")
		}
	}()

	t.Run("from environment", func(t *testing.T) {
		os.Setenv("PENF_TENANT_ID", "env-tenant")
		cfg = &config.CLIConfig{TenantID: "config-tenant"}

		result := getTenantID()
		if result != "env-tenant" {
			t.Errorf("getTenantID() = %q, want 'env-tenant'", result)
		}
	})

	t.Run("from config when env not set", func(t *testing.T) {
		os.Unsetenv("PENF_TENANT_ID")
		cfg = &config.CLIConfig{TenantID: "config-tenant"}

		result := getTenantID()
		if result != "config-tenant" {
			t.Errorf("getTenantID() = %q, want 'config-tenant'", result)
		}
	})

	t.Run("empty when nothing set", func(t *testing.T) {
		os.Unsetenv("PENF_TENANT_ID")
		cfg = &config.CLIConfig{}

		result := getTenantID()
		if result != "" {
			t.Errorf("getTenantID() = %q, want empty string", result)
		}
	})

	t.Run("empty when cfg is nil", func(t *testing.T) {
		os.Unsetenv("PENF_TENANT_ID")
		cfg = nil

		result := getTenantID()
		if result != "" {
			t.Errorf("getTenantID() = %q, want empty string", result)
		}
	})
}

// TestHealthCommandContext tests context handling in health command.
func TestHealthCommandContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// The context should be cancellable
	select {
	case <-ctx.Done():
		t.Error("context should not be done yet")
	default:
		// Expected - context is still valid
	}

	cancel()

	select {
	case <-ctx.Done():
		// Expected - context is now cancelled
	default:
		t.Error("context should be cancelled")
	}
}
*/
