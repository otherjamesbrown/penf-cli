package buildinfo_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/otherjamesbrown/penf-cli/pkg/buildinfo"
)

func TestHandler(t *testing.T) {
	// Create test server with the handler
	handler := buildinfo.Handler("test-service")
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()

	// Execute request
	handler(rec, req)

	// Verify status code
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// Verify content type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Verify JSON structure
	var info buildinfo.Info
	if err := json.NewDecoder(rec.Body).Decode(&info); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Verify service name
	if info.ServiceName != "test-service" {
		t.Errorf("Expected service_name 'test-service', got '%s'", info.ServiceName)
	}

	// Verify required fields are present
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

	// Verify Go version starts with "go" (e.g., "go1.24.0")
	if len(info.GoVersion) < 2 || info.GoVersion[:2] != "go" {
		t.Errorf("Expected go_version to start with 'go', got '%s'", info.GoVersion)
	}
}

func TestHandler_MultipleServices(t *testing.T) {
	services := []string{
		"penfold-gateway",
		"penfold-worker",
		"penfold-ai-coordinator",
	}

	for _, serviceName := range services {
		t.Run(serviceName, func(t *testing.T) {
			handler := buildinfo.Handler(serviceName)
			req := httptest.NewRequest(http.MethodGet, "/version", nil)
			rec := httptest.NewRecorder()

			handler(rec, req)

			var info buildinfo.Info
			if err := json.NewDecoder(rec.Body).Decode(&info); err != nil {
				t.Fatalf("Failed to decode JSON for %s: %v", serviceName, err)
			}

			if info.ServiceName != serviceName {
				t.Errorf("Expected service_name '%s', got '%s'", serviceName, info.ServiceName)
			}
		})
	}
}
