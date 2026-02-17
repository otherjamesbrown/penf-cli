package buildinfo

import (
	"encoding/json"
	"runtime"
	"testing"
)

func TestGet_ReturnsCorrectDefaults(t *testing.T) {
	info := Get("test-svc")

	if info.ServiceName != "test-svc" {
		t.Errorf("expected ServiceName='test-svc', got %q", info.ServiceName)
	}
	if info.Version != "dev" {
		t.Errorf("expected Version='dev', got %q", info.Version)
	}
	if info.Commit != "unknown" {
		t.Errorf("expected Commit='unknown', got %q", info.Commit)
	}
	if info.BuildTime != "unknown" {
		t.Errorf("expected BuildTime='unknown', got %q", info.BuildTime)
	}
	if info.GoVersion != runtime.Version() {
		t.Errorf("expected GoVersion=%q, got %q", runtime.Version(), info.GoVersion)
	}
}

func TestGet_ServiceName(t *testing.T) {
	tests := []string{
		"gateway",
		"worker",
		"ai-coordinator",
		"mlx-services",
	}

	for _, serviceName := range tests {
		t.Run(serviceName, func(t *testing.T) {
			info := Get(serviceName)
			if info.ServiceName != serviceName {
				t.Errorf("expected ServiceName=%q, got %q", serviceName, info.ServiceName)
			}
		})
	}
}

func TestString_DefaultFormat(t *testing.T) {
	result := String()
	expected := "dev (unknown, unknown)"

	if result != expected {
		t.Errorf("expected String()=%q, got %q", expected, result)
	}
}

func TestString_CustomValues(t *testing.T) {
	// Save original values
	origVersion := Version
	origCommit := Commit
	origBuildTime := BuildTime

	// Restore after test
	defer func() {
		Version = origVersion
		Commit = origCommit
		BuildTime = origBuildTime
	}()

	// Set custom values
	Version = "v1.2.3"
	Commit = "abc123d"
	BuildTime = "2026-02-07T10:30:00Z"

	result := String()
	expected := "v1.2.3 (abc123d, 2026-02-07T10:30:00Z)"

	if result != expected {
		t.Errorf("expected String()=%q, got %q", expected, result)
	}
}

func TestInfo_JSONSerialization(t *testing.T) {
	info := Info{
		ServiceName: "test-service",
		Version:     "v1.0.0",
		Commit:      "abcd1234",
		BuildTime:   "2026-01-01T00:00:00Z",
		GoVersion:   "go1.24.0",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal Info: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	expectedKeys := map[string]string{
		"service_name": "test-service",
		"version":      "v1.0.0",
		"commit":       "abcd1234",
		"build_time":   "2026-01-01T00:00:00Z",
		"go_version":   "go1.24.0",
	}

	for key, expectedValue := range expectedKeys {
		value, ok := decoded[key]
		if !ok {
			t.Errorf("missing key %q in JSON output", key)
			continue
		}
		if value != expectedValue {
			t.Errorf("key %q: expected %q, got %v", key, expectedValue, value)
		}
	}

	// Verify no extra keys
	if len(decoded) != len(expectedKeys) {
		t.Errorf("expected %d keys in JSON, got %d", len(expectedKeys), len(decoded))
	}
}
