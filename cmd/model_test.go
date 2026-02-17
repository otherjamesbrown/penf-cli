// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestModelCatalogEntryJSON tests JSON output formatting for model catalog entries.
func TestModelCatalogEntryJSON(t *testing.T) {
	entry := ModelCatalogEntry{
		ID:              "mlx-community/Phi-3.5-mini-instruct-4bit",
		Name:            "Phi-3.5 Mini (4-bit)",
		Size:            "~2.5GB",
		Type:            "mlx",
		Downloaded:      true,
		ExpectedLatency: "1-4s",
		MemoryRequired:  "4GB+",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry to JSON: %v", err)
	}

	var decoded ModelCatalogEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded.ID != entry.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, entry.ID)
	}
	if decoded.Name != entry.Name {
		t.Errorf("Name = %v, want %v", decoded.Name, entry.Name)
	}
	if decoded.Downloaded != entry.Downloaded {
		t.Errorf("Downloaded = %v, want %v", decoded.Downloaded, entry.Downloaded)
	}
}

// TestModelCatalogEntryYAML tests YAML output formatting for model catalog entries.
func TestModelCatalogEntryYAML(t *testing.T) {
	entry := ModelCatalogEntry{
		ID:         "mlx-community/Phi-3.5-mini-instruct-4bit",
		Name:       "Phi-3.5 Mini (4-bit)",
		Type:       "mlx",
		Downloaded: false,
	}

	data, err := yaml.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry to YAML: %v", err)
	}

	var decoded ModelCatalogEntry
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	if decoded.ID != entry.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, entry.ID)
	}
}

// TestModelServerStatusJSON tests JSON output formatting for server status.
func TestModelServerStatusJSON(t *testing.T) {
	status := ModelServerStatus{
		PID:     12345,
		Model:   "mlx-community/Phi-3.5-mini-instruct-4bit",
		Port:    8080,
		CPUPct:  25.5,
		MemPct:  10.2,
		Healthy: true,
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Failed to marshal status to JSON: %v", err)
	}

	var decoded ModelServerStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded.PID != status.PID {
		t.Errorf("PID = %v, want %v", decoded.PID, status.PID)
	}
	if decoded.Port != status.Port {
		t.Errorf("Port = %v, want %v", decoded.Port, status.Port)
	}
	if decoded.Healthy != status.Healthy {
		t.Errorf("Healthy = %v, want %v", decoded.Healthy, status.Healthy)
	}
}

// TestResolveModelID tests the model ID resolution function.
func TestResolveModelID(t *testing.T) {
	deps := &ModelCommandDeps{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full ID passthrough",
			input:    "mlx-community/Phi-3.5-mini-instruct-4bit",
			expected: "mlx-community/Phi-3.5-mini-instruct-4bit",
		},
		{
			name:     "short name phi",
			input:    "phi",
			expected: "mlx-community/Phi-3.5-mini-instruct-4bit",
		},
		{
			name:     "short name phi uppercase",
			input:    "PHI",
			expected: "mlx-community/Phi-3.5-mini-instruct-4bit",
		},
		{
			name:     "short name qwen-7b",
			input:    "qwen-7b",
			expected: "mlx-community/Qwen2.5-7B-Instruct-4bit",
		},
		{
			name:     "short name llama",
			input:    "llama",
			expected: "mlx-community/Llama-3.2-3B-Instruct-4bit",
		},
		{
			name:     "short name gemma",
			input:    "gemma",
			expected: "mlx-community/gemma-2-9b-it-4bit",
		},
		{
			name:     "unknown model passthrough",
			input:    "unknown-model",
			expected: "unknown-model",
		},
		{
			name:     "partial match from catalog",
			input:    "Qwen2.5-7B",
			expected: "mlx-community/Qwen2.5-7B-Instruct-4bit",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := resolveModelID(deps, tc.input)
			if result != tc.expected {
				t.Errorf("resolveModelID(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// TestGetDownloadedModels tests the downloaded models detection.
func TestGetDownloadedModels(t *testing.T) {
	// Create a temporary HuggingFace cache directory structure.
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, ".cache", "huggingface", "hub")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	// Create some model directories.
	modelDirs := []string{
		"models--mlx-community--Phi-3.5-mini-instruct-4bit",
		"models--mlx-community--Qwen2.5-7B-Instruct-4bit",
		"other-file.txt", // Should be ignored (not a directory).
	}

	for _, dir := range modelDirs {
		path := filepath.Join(cacheDir, dir)
		if dir == "other-file.txt" {
			// Create a file instead of directory.
			if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
		} else {
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatalf("Failed to create model dir: %v", err)
			}
		}
	}

	// Override home directory detection by using a custom deps.
	deps := &ModelCommandDeps{}

	// Note: getDownloadedModels uses os.UserHomeDir() internally,
	// so we can't easily override it without modifying the function.
	// This test verifies the basic structure parsing works.
	downloaded := getDownloadedModels(deps)

	// Since we can't override the home directory, the real cache may or may not exist.
	// We just verify the function returns a map without error.
	if downloaded == nil {
		t.Error("getDownloadedModels returned nil")
	}
}

// TestDefaultModelCatalog tests that the default catalog is properly populated.
func TestDefaultModelCatalog(t *testing.T) {
	if len(defaultModelCatalog) == 0 {
		t.Error("defaultModelCatalog is empty")
	}

	for _, entry := range defaultModelCatalog {
		if entry.ID == "" {
			t.Error("found entry with empty ID")
		}
		if entry.Name == "" {
			t.Errorf("entry %s has empty Name", entry.ID)
		}
		if entry.Type != "mlx" {
			t.Errorf("entry %s has Type=%q, want 'mlx'", entry.ID, entry.Type)
		}
		if entry.Size == "" {
			t.Errorf("entry %s has empty Size", entry.ID)
		}
	}
}

// TestNewModelCommand tests that all subcommands are registered.
func TestNewModelCommand(t *testing.T) {
	deps := &ModelCommandDeps{
		ConfigDir:  "/tmp/test-config",
		SidecarDir: "/tmp/test-sidecar",
	}

	cmd := NewModelCommand(deps)

	expectedSubcommands := []string{
		// Local model commands
		"list",
		"status",
		"serve",
		"stop",
		"switch",
		"info",
		"bench",
		"download",
		// Registry commands (local + remote)
		"registry",
		"add",
		"enable",
		"disable",
		"rules",
	}

	for _, expected := range expectedSubcommands {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q not found", expected)
		}
	}
}

// TestNewModelDownloadCommand tests the download command setup.
func TestNewModelDownloadCommand(t *testing.T) {
	deps := &ModelCommandDeps{
		ConfigDir:  "/tmp/test-config",
		SidecarDir: "/tmp/test-sidecar",
	}

	cmd := newModelDownloadCommand(deps)

	if cmd.Use != "download <model>" {
		t.Errorf("Use = %q, want 'download <model>'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short description is empty")
	}
	if cmd.Long == "" {
		t.Error("Long description is empty")
	}
}

// TestModelCommandDeps tests the default dependencies.
func TestModelCommandDeps(t *testing.T) {
	deps := DefaultModelDeps()

	if deps == nil {
		t.Fatal("DefaultModelDeps returned nil")
	}

	if deps.ConfigDir == "" {
		t.Error("ConfigDir is empty")
	}

	if deps.SidecarDir == "" {
		t.Error("SidecarDir is empty")
	}

	if deps.LoadConfig == nil {
		t.Error("LoadConfig is nil")
	}
}

// TestNewModelRegistryCommand tests the registry command setup.
func TestNewModelRegistryCommand(t *testing.T) {
	deps := &ModelCommandDeps{
		ConfigDir:  "/tmp/test-config",
		SidecarDir: "/tmp/test-sidecar",
	}

	cmd := newModelRegistryCommand(deps)

	if cmd.Use != "registry" {
		t.Errorf("Use = %q, want 'registry'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short description is empty")
	}
	if cmd.Long == "" {
		t.Error("Long description is empty")
	}

	// Check flags are defined
	flags := []string{"provider", "local", "remote", "enabled", "disabled"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %q not found", flag)
		}
	}
}

// TestNewModelAddCommand tests the add command setup.
func TestNewModelAddCommand(t *testing.T) {
	deps := &ModelCommandDeps{
		ConfigDir:  "/tmp/test-config",
		SidecarDir: "/tmp/test-sidecar",
	}

	cmd := newModelAddCommand(deps)

	if cmd.Use != "add <provider> <model-name>" {
		t.Errorf("Use = %q, want 'add <provider> <model-name>'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	// Check flags are defined
	flags := []string{"type", "capabilities", "endpoint", "priority"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %q not found", flag)
		}
	}
}

// TestNewModelEnableDisableCommands tests enable/disable commands setup.
func TestNewModelEnableDisableCommands(t *testing.T) {
	deps := &ModelCommandDeps{
		ConfigDir:  "/tmp/test-config",
		SidecarDir: "/tmp/test-sidecar",
	}

	enableCmd := newModelEnableCommand(deps)
	if enableCmd.Use != "enable <model-id>" {
		t.Errorf("enable Use = %q, want 'enable <model-id>'", enableCmd.Use)
	}

	disableCmd := newModelDisableCommand(deps)
	if disableCmd.Use != "disable <model-id>" {
		t.Errorf("disable Use = %q, want 'disable <model-id>'", disableCmd.Use)
	}
}

// TestNewModelRulesCommand tests the rules command setup.
func TestNewModelRulesCommand(t *testing.T) {
	deps := &ModelCommandDeps{
		ConfigDir:  "/tmp/test-config",
		SidecarDir: "/tmp/test-sidecar",
	}

	cmd := newModelRulesCommand(deps)

	if cmd.Use != "rules" {
		t.Errorf("Use = %q, want 'rules'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	// Check task flag is defined
	if cmd.Flags().Lookup("task") == nil {
		t.Error("expected flag 'task' not found")
	}
}

// TestRegistryModelEntryJSON tests JSON output formatting for registry model entries.
func TestRegistryModelEntryJSON(t *testing.T) {
	entry := RegistryModelEntry{
		ID:           "abc123",
		Name:         "gemini/gemini-2.0-flash",
		Provider:     "gemini",
		ModelName:    "gemini-2.0-flash",
		Type:         "llm",
		Status:       "ready",
		Capabilities: []string{"chat", "summarization"},
		IsLocal:      false,
		IsEnabled:    true,
		Priority:     5,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry to JSON: %v", err)
	}

	var decoded RegistryModelEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded.ID != entry.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, entry.ID)
	}
	if decoded.Provider != entry.Provider {
		t.Errorf("Provider = %v, want %v", decoded.Provider, entry.Provider)
	}
	if decoded.IsEnabled != entry.IsEnabled {
		t.Errorf("IsEnabled = %v, want %v", decoded.IsEnabled, entry.IsEnabled)
	}
	if len(decoded.Capabilities) != len(entry.Capabilities) {
		t.Errorf("Capabilities length = %v, want %v", len(decoded.Capabilities), len(entry.Capabilities))
	}
}

// TestRegistryModelEntryYAML tests YAML output formatting for registry model entries.
func TestRegistryModelEntryYAML(t *testing.T) {
	entry := RegistryModelEntry{
		ID:           "xyz789",
		Name:         "openai/gpt-4o",
		Provider:     "openai",
		ModelName:    "gpt-4o",
		Type:         "llm",
		Status:       "ready",
		Capabilities: []string{"chat", "extraction"},
		IsLocal:      false,
		IsEnabled:    true,
		Priority:     10,
	}

	data, err := yaml.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry to YAML: %v", err)
	}

	var decoded RegistryModelEntry
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	if decoded.ID != entry.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, entry.ID)
	}
	if decoded.ModelName != entry.ModelName {
		t.Errorf("ModelName = %v, want %v", decoded.ModelName, entry.ModelName)
	}
}

// TestRoutingRuleEntryJSON tests JSON output formatting for routing rule entries.
func TestRoutingRuleEntryJSON(t *testing.T) {
	entry := RoutingRuleEntry{
		ID:               "rule-1",
		Name:             "embedding-routing",
		TaskType:         "embedding",
		PreferredModels:  []string{"model-1", "model-2"},
		FallbackModels:   []string{"model-3"},
		OptimizationMode: "latency",
		IsEnabled:        true,
		Description:      "Route embedding requests",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry to JSON: %v", err)
	}

	var decoded RoutingRuleEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded.ID != entry.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, entry.ID)
	}
	if decoded.TaskType != entry.TaskType {
		t.Errorf("TaskType = %v, want %v", decoded.TaskType, entry.TaskType)
	}
	if len(decoded.PreferredModels) != len(entry.PreferredModels) {
		t.Errorf("PreferredModels length = %v, want %v", len(decoded.PreferredModels), len(entry.PreferredModels))
	}
}
