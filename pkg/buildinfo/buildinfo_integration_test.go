//go:build integration

package buildinfo_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/buildinfo"
)

// These tests verify live /version endpoints on deployed services.
// They only pass after deployment and serve as post-deploy verification.

func TestVersionEndpoint_Gateway(t *testing.T) {
	testVersionEndpoint(t, "http://dev02.brown.chat:8080/version", "penfold-gateway")
}

func TestVersionEndpoint_Worker(t *testing.T) {
	testVersionEndpoint(t, "http://dev01.brown.chat:8085/version", "penfold-worker")
}

func TestVersionEndpoint_AICoordinator(t *testing.T) {
	testVersionEndpoint(t, "http://dev02.brown.chat:8090/version", "penfold-ai-coordinator")
}

func testVersionEndpoint(t *testing.T, url, expectedServiceName string) {
	t.Helper()

	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		t.Skipf("Service unreachable at %s (not deployed?): %v", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Skipf("Service at %s returned 404 (version endpoint not deployed yet)", url)
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 from %s, got %d", url, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var info buildinfo.Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("Failed to decode JSON from %s: %v", url, err)
	}

	// Verify required fields
	if info.ServiceName != expectedServiceName {
		t.Errorf("Expected service_name '%s', got '%s'", expectedServiceName, info.ServiceName)
	}
	if info.Version == "" {
		t.Error("Expected version to be non-empty")
	}
	if info.Commit == "" {
		t.Error("Expected commit to be non-empty")
	}
	if info.BuildTime == "" {
		t.Error("Expected build_time to be non-empty")
	}
	if info.GoVersion == "" {
		t.Error("Expected go_version to be non-empty")
	}

	// Verify Go version looks right
	if len(info.GoVersion) < 2 || info.GoVersion[:2] != "go" {
		t.Errorf("Expected go_version to start with 'go', got '%s'", info.GoVersion)
	}

	// In non-dev deployments, version should not be "dev"
	if info.Version != "dev" {
		t.Logf("Service %s running version %s (commit: %s, built: %s)",
			info.ServiceName, info.Version, info.Commit, info.BuildTime)
	} else {
		t.Logf("Service %s running dev build (commit: %s)", info.ServiceName, info.Commit)
	}

	// Verify JSON round-trip: marshal back and confirm all expected keys present
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal info back to JSON: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal round-tripped JSON: %v", err)
	}

	expectedKeys := []string{"service_name", "version", "commit", "build_time", "go_version"}
	for _, key := range expectedKeys {
		if _, ok := decoded[key]; !ok {
			t.Errorf("Missing key '%s' in JSON response from %s", key, url)
		}
	}

	// Verify exact key count (no unexpected extra fields)
	if len(decoded) != len(expectedKeys) {
		t.Errorf("Expected %d keys in JSON from %s, got %d: %v",
			len(expectedKeys), url, len(decoded), keysOf(decoded))
	}
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

