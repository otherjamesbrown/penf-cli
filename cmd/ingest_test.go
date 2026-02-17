// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// createIngestTestDeps creates test dependencies with mock implementations.
func createIngestTestDeps(cfg *config.CLIConfig) *IngestCommandDeps {
	return &IngestCommandDeps{
		Config:       cfg,
		OutputFormat: cfg.OutputFormat,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		SaveConfig: func(c *config.CLIConfig) error {
			return nil
		},
		InitClient: func(c *config.CLIConfig) (*client.GRPCClient, error) {
			return nil, nil
		},
	}
}

func TestNewIngestCommand(t *testing.T) {
	deps := createIngestTestDeps(mockConfig())
	cmd := NewIngestCommand(deps)

	if cmd == nil {
		t.Fatal("NewIngestCommand returned nil")
	}

	if cmd.Use != "ingest" {
		t.Errorf("expected Use to be 'ingest', got %q", cmd.Use)
	}

	// Check subcommands exist.
	subcommands := cmd.Commands()
	expectedSubcmds := []string{"file", "url", "batch", "gmail", "status", "queue", "config"}

	for _, expected := range expectedSubcmds {
		found := false
		for _, sub := range subcommands {
			if sub.Use == expected || strings.HasPrefix(sub.Use, expected+" ") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q not found", expected)
		}
	}
}

func TestNewIngestCommand_WithNilDeps(t *testing.T) {
	cmd := NewIngestCommand(nil)

	if cmd == nil {
		t.Fatal("NewIngestCommand with nil deps returned nil")
	}
}

func TestNewIngestCommand_PersistentFlags(t *testing.T) {
	deps := createIngestTestDeps(mockConfig())
	cmd := NewIngestCommand(deps)

	// Check persistent flags exist.
	flags := []string{"tenant", "async", "priority", "tags", "category", "output"}
	for _, flag := range flags {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("expected persistent flag %q to exist", flag)
		}
	}
}

func TestIngestGmailSubcommands(t *testing.T) {
	deps := createIngestTestDeps(mockConfig())
	cmd := NewIngestCommand(deps)

	// Find the gmail command.
	gmailCmd, _, err := cmd.Find([]string{"gmail"})
	if err != nil {
		t.Fatalf("failed to find gmail command: %v", err)
	}

	// Check gmail subcommands exist.
	subcommands := gmailCmd.Commands()
	expectedSubcmds := []string{"sync", "status", "history"}

	for _, expected := range expectedSubcmds {
		found := false
		for _, sub := range subcommands {
			if sub.Use == expected || strings.HasPrefix(sub.Use, expected+" ") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected gmail subcommand %q not found", expected)
		}
	}
}

func TestIngestConfigSubcommands(t *testing.T) {
	deps := createIngestTestDeps(mockConfig())
	cmd := NewIngestCommand(deps)

	// Find the config command.
	configCmd, _, err := cmd.Find([]string{"config"})
	if err != nil {
		t.Fatalf("failed to find config command: %v", err)
	}

	// Check config subcommands exist.
	subcommands := configCmd.Commands()
	expectedSubcmds := []string{"show", "set"}

	for _, expected := range expectedSubcmds {
		found := false
		for _, sub := range subcommands {
			if sub.Use == expected || strings.HasPrefix(sub.Use, expected+" ") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected config subcommand %q not found", expected)
		}
	}
}

func TestValidateIngestPriority(t *testing.T) {
	tests := []struct {
		priority string
		valid    bool
	}{
		{"low", true},
		{"normal", true},
		{"high", true},
		{"invalid", false},
		{"", false},
		{"Low", false}, // Case sensitive.
		{"HIGH", false},
	}

	for _, tt := range tests {
		result := validateIngestPriority(tt.priority)
		if result != tt.valid {
			t.Errorf("validateIngestPriority(%q) = %v, want %v", tt.priority, result, tt.valid)
		}
	}
}

func TestGetValidConfigKeys(t *testing.T) {
	keys := getValidConfigKeys()

	expectedKeys := []string{
		"auto_sync",
		"sync_interval",
		"batch_size",
		"max_retries",
		"default_priority",
		"default_category",
	}

	if len(keys) != len(expectedKeys) {
		t.Errorf("expected %d config keys, got %d", len(expectedKeys), len(keys))
	}

	for _, expected := range expectedKeys {
		found := false
		for _, key := range keys {
			if key == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected config key %q not found", expected)
		}
	}
}

func TestIngestJob_JSONSerialization(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-1 * time.Minute)
	completedAt := now

	job := IngestJob{
		ID:          "ingest-test-001",
		Type:        "file",
		Source:      "/path/to/file.pdf",
		Status:      IngestJobStatusCompleted,
		Priority:    "normal",
		Progress:    100,
		Message:     "Complete",
		CreatedAt:   now.Add(-2 * time.Minute),
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		ItemsTotal:  10,
		ItemsDone:   10,
		Tags:        []string{"test", "document"},
		Category:    "reports",
		TenantID:    "tenant-001",
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("failed to marshal IngestJob: %v", err)
	}

	var decoded IngestJob
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal IngestJob: %v", err)
	}

	if decoded.ID != job.ID {
		t.Errorf("expected ID %q, got %q", job.ID, decoded.ID)
	}
	if decoded.Type != job.Type {
		t.Errorf("expected Type %q, got %q", job.Type, decoded.Type)
	}
	if decoded.Status != job.Status {
		t.Errorf("expected Status %q, got %q", job.Status, decoded.Status)
	}
	if decoded.Progress != job.Progress {
		t.Errorf("expected Progress %d, got %d", job.Progress, decoded.Progress)
	}
	if len(decoded.Tags) != len(job.Tags) {
		t.Errorf("expected %d tags, got %d", len(job.Tags), len(decoded.Tags))
	}
}

func TestIngestJob_YAMLSerialization(t *testing.T) {
	job := IngestJob{
		ID:       "ingest-test-001",
		Type:     "url",
		Source:   "https://example.com/article",
		Status:   IngestJobStatusProcessing,
		Priority: "high",
		Progress: 50,
	}

	data, err := yaml.Marshal(job)
	if err != nil {
		t.Fatalf("failed to marshal IngestJob: %v", err)
	}

	var decoded IngestJob
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal IngestJob: %v", err)
	}

	if decoded.ID != job.ID {
		t.Errorf("expected ID %q, got %q", job.ID, decoded.ID)
	}
	if decoded.Status != job.Status {
		t.Errorf("expected Status %q, got %q", job.Status, decoded.Status)
	}
}

func TestIngestStatusResponse_JSONSerialization(t *testing.T) {
	status := IngestStatusResponse{
		TotalJobs:      100,
		PendingJobs:    5,
		ProcessingJobs: 2,
		CompletedJobs:  90,
		FailedJobs:     3,
		ProcessingRate: 15.5,
		LastUpdated:    time.Now(),
		RecentJobs: []IngestJob{
			{ID: "job-1", Type: "file", Status: IngestJobStatusCompleted},
			{ID: "job-2", Type: "url", Status: IngestJobStatusProcessing},
		},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal IngestStatusResponse: %v", err)
	}

	var decoded IngestStatusResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal IngestStatusResponse: %v", err)
	}

	if decoded.TotalJobs != status.TotalJobs {
		t.Errorf("expected TotalJobs %d, got %d", status.TotalJobs, decoded.TotalJobs)
	}
	if decoded.PendingJobs != status.PendingJobs {
		t.Errorf("expected PendingJobs %d, got %d", status.PendingJobs, decoded.PendingJobs)
	}
	if len(decoded.RecentJobs) != len(status.RecentJobs) {
		t.Errorf("expected %d recent jobs, got %d", len(status.RecentJobs), len(decoded.RecentJobs))
	}
}

func TestGmailSyncStatus_JSONSerialization(t *testing.T) {
	status := GmailSyncStatus{
		Connected:    true,
		LastSyncAt:   time.Now().Add(-15 * time.Minute),
		NextSyncAt:   time.Now().Add(15 * time.Minute),
		TotalEmails:  5000,
		SyncedEmails: 4500,
		SyncState:    "idle",
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal GmailSyncStatus: %v", err)
	}

	var decoded GmailSyncStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal GmailSyncStatus: %v", err)
	}

	if decoded.Connected != status.Connected {
		t.Errorf("expected Connected %v, got %v", status.Connected, decoded.Connected)
	}
	if decoded.TotalEmails != status.TotalEmails {
		t.Errorf("expected TotalEmails %d, got %d", status.TotalEmails, decoded.TotalEmails)
	}
}

func TestGmailSyncHistoryEntry_JSONSerialization(t *testing.T) {
	entry := GmailSyncHistoryEntry{
		ID:            "sync-001",
		StartedAt:     time.Now().Add(-5 * time.Minute),
		CompletedAt:   time.Now(),
		EmailsAdded:   10,
		EmailsUpdated: 5,
		Status:        "completed",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal GmailSyncHistoryEntry: %v", err)
	}

	var decoded GmailSyncHistoryEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal GmailSyncHistoryEntry: %v", err)
	}

	if decoded.ID != entry.ID {
		t.Errorf("expected ID %q, got %q", entry.ID, decoded.ID)
	}
	if decoded.EmailsAdded != entry.EmailsAdded {
		t.Errorf("expected EmailsAdded %d, got %d", entry.EmailsAdded, decoded.EmailsAdded)
	}
}

func TestIngestConfig_JSONSerialization(t *testing.T) {
	cfg := IngestConfig{
		AutoSync:        true,
		SyncInterval:    "30m",
		BatchSize:       50,
		MaxRetries:      3,
		DefaultPriority: "normal",
		DefaultCategory: "documents",
		ExcludePatterns: []string{"*.tmp", "*.bak"},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal IngestConfig: %v", err)
	}

	var decoded IngestConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal IngestConfig: %v", err)
	}

	if decoded.AutoSync != cfg.AutoSync {
		t.Errorf("expected AutoSync %v, got %v", cfg.AutoSync, decoded.AutoSync)
	}
	if decoded.BatchSize != cfg.BatchSize {
		t.Errorf("expected BatchSize %d, got %d", cfg.BatchSize, decoded.BatchSize)
	}
	if len(decoded.ExcludePatterns) != len(cfg.ExcludePatterns) {
		t.Errorf("expected %d exclude patterns, got %d", len(cfg.ExcludePatterns), len(decoded.ExcludePatterns))
	}
}

func TestCreateMockIngestJob(t *testing.T) {
	// Reset global flags.
	ingestPriority = "high"
	ingestTags = []string{"test", "mock"}
	ingestCategory = "testing"
	ingestTenantID = "tenant-test"

	job := createMockIngestJob("file", "/path/to/file.pdf")

	if job.Type != "file" {
		t.Errorf("expected Type 'file', got %q", job.Type)
	}
	if job.Source != "/path/to/file.pdf" {
		t.Errorf("expected Source '/path/to/file.pdf', got %q", job.Source)
	}
	if job.Priority != "high" {
		t.Errorf("expected Priority 'high', got %q", job.Priority)
	}
	if len(job.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(job.Tags))
	}
	if job.Category != "testing" {
		t.Errorf("expected Category 'testing', got %q", job.Category)
	}
	if job.TenantID != "tenant-test" {
		t.Errorf("expected TenantID 'tenant-test', got %q", job.TenantID)
	}
	if job.Status != IngestJobStatusPending {
		t.Errorf("expected Status 'pending', got %q", job.Status)
	}
	if job.ID == "" {
		t.Error("expected non-empty ID")
	}
	if !strings.HasPrefix(job.ID, "ingest-file-") {
		t.Errorf("expected ID to start with 'ingest-file-', got %q", job.ID)
	}

	// Reset flags.
	ingestPriority = "normal"
	ingestTags = nil
	ingestCategory = ""
	ingestTenantID = ""
}

func TestGetMockIngestJob(t *testing.T) {
	job := getMockIngestJob("test-job-123")

	if job.ID != "test-job-123" {
		t.Errorf("expected ID 'test-job-123', got %q", job.ID)
	}
	if job.Type == "" {
		t.Error("expected non-empty Type")
	}
	if job.Status == "" {
		t.Error("expected non-empty Status")
	}
}

func TestGetMockIngestStatus(t *testing.T) {
	status := getMockIngestStatus()

	if status.TotalJobs == 0 {
		t.Error("expected non-zero TotalJobs")
	}
	if status.CompletedJobs == 0 {
		t.Error("expected non-zero CompletedJobs")
	}
	if len(status.RecentJobs) == 0 {
		t.Error("expected non-empty RecentJobs")
	}
}

func TestGetMockIngestQueue(t *testing.T) {
	jobs := getMockIngestQueue()

	if len(jobs) == 0 {
		t.Error("expected non-empty queue")
	}

	for _, job := range jobs {
		if job.Status != IngestJobStatusPending {
			t.Errorf("expected all jobs to be pending, got %q for job %s", job.Status, job.ID)
		}
	}
}

func TestGetMockGmailStatus(t *testing.T) {
	status := getMockGmailStatus()

	if !status.Connected {
		t.Error("expected Connected to be true")
	}
	if status.TotalEmails == 0 {
		t.Error("expected non-zero TotalEmails")
	}
	if status.SyncState == "" {
		t.Error("expected non-empty SyncState")
	}
}

func TestGetMockGmailHistory(t *testing.T) {
	history := getMockGmailHistory(2)

	if len(history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history))
	}

	// Test with larger limit.
	fullHistory := getMockGmailHistory(100)
	if len(fullHistory) == 0 {
		t.Error("expected non-empty history")
	}
}

func TestGetMockIngestConfig(t *testing.T) {
	cfg := getMockIngestConfig()

	if cfg.SyncInterval == "" {
		t.Error("expected non-empty SyncInterval")
	}
	if cfg.BatchSize == 0 {
		t.Error("expected non-zero BatchSize")
	}
	if cfg.MaxRetries == 0 {
		t.Error("expected non-zero MaxRetries")
	}
}

func TestTruncateIngestString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncateIngestString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateIngestString(%q, %d) = %q, want %q",
				tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestIngestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		contains string
	}{
		{500 * time.Millisecond, "ms"},
		{5 * time.Second, "s"},
		{90 * time.Second, "m"},
		{5 * time.Minute, "m"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("formatDuration(%v) = %q, expected to contain %q",
				tt.duration, result, tt.contains)
		}
	}
}

func TestGetJobStatusColor(t *testing.T) {
	tests := []struct {
		status   IngestJobStatus
		expected string
	}{
		{IngestJobStatusCompleted, "\033[32m"},  // Green.
		{IngestJobStatusProcessing, "\033[34m"}, // Blue.
		{IngestJobStatusPending, "\033[33m"},    // Yellow.
		{IngestJobStatusFailed, "\033[31m"},     // Red.
		{IngestJobStatus("unknown"), ""},
	}

	for _, tt := range tests {
		result := getJobStatusColor(tt.status)
		if result != tt.expected {
			t.Errorf("getJobStatusColor(%q) = %q, expected %q", tt.status, result, tt.expected)
		}
	}
}

func TestIngestGetPriorityColor(t *testing.T) {
	tests := []struct {
		priority string
		expected string
	}{
		{"high", "\033[31m"},   // Red.
		{"normal", "\033[33m"}, // Yellow.
		{"low", "\033[32m"},    // Green.
		{"unknown", ""},
	}

	for _, tt := range tests {
		result := getPriorityColor(tt.priority)
		if result != tt.expected {
			t.Errorf("getPriorityColor(%q) = %q, expected %q", tt.priority, result, tt.expected)
		}
	}
}

func TestOutputIngestJob_JSON(t *testing.T) {
	job := IngestJob{
		ID:       "test-001",
		Type:     "file",
		Source:   "/test/file.pdf",
		Status:   IngestJobStatusCompleted,
		Priority: "normal",
		Progress: 100,
	}

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputIngestJob(config.OutputFormatJSON, job)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputIngestJob failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid JSON.
	var decoded IngestJob
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}

	if decoded.ID != "test-001" {
		t.Errorf("expected ID 'test-001', got %q", decoded.ID)
	}
}

func TestOutputIngestJob_YAML(t *testing.T) {
	job := IngestJob{
		ID:       "test-001",
		Type:     "url",
		Source:   "https://example.com",
		Status:   IngestJobStatusProcessing,
		Priority: "high",
		Progress: 50,
	}

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputIngestJob(config.OutputFormatYAML, job)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputIngestJob failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid YAML.
	var decoded IngestJob
	if err := yaml.Unmarshal([]byte(output), &decoded); err != nil {
		t.Errorf("output is not valid YAML: %v", err)
	}

	if decoded.ID != "test-001" {
		t.Errorf("expected ID 'test-001', got %q", decoded.ID)
	}
}

func TestOutputIngestJob_Text(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-1 * time.Minute)
	job := IngestJob{
		ID:        "test-001",
		Type:      "file",
		Source:    "/test/file.pdf",
		Status:    IngestJobStatusProcessing,
		Priority:  "high",
		Progress:  75,
		Message:   "Processing...",
		CreatedAt: now.Add(-2 * time.Minute),
		StartedAt: &startedAt,
		Tags:      []string{"test", "doc"},
		Category:  "documents",
	}

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputIngestJob(config.OutputFormatText, job)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputIngestJob failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check expected content.
	if !strings.Contains(output, "test-001") {
		t.Error("output should contain job ID")
	}
	if !strings.Contains(output, "file") {
		t.Error("output should contain job type")
	}
	if !strings.Contains(output, "75%") {
		t.Error("output should contain progress")
	}
	if !strings.Contains(output, "test, doc") {
		t.Error("output should contain tags")
	}
	if !strings.Contains(output, "documents") {
		t.Error("output should contain category")
	}
}

func TestOutputIngestStatus_JSON(t *testing.T) {
	status := getMockIngestStatus()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputIngestStatus(config.OutputFormatJSON, status)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputIngestStatus failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid JSON.
	var decoded IngestStatusResponse
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestOutputIngestStatus_Text(t *testing.T) {
	status := getMockIngestStatus()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputIngestStatus(config.OutputFormatText, status)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputIngestStatus failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check expected content.
	if !strings.Contains(output, "Ingestion Status") {
		t.Error("output should contain title")
	}
	if !strings.Contains(output, "Total Jobs") {
		t.Error("output should contain Total Jobs")
	}
	if !strings.Contains(output, "Pending") {
		t.Error("output should contain Pending")
	}
	if !strings.Contains(output, "Recent Jobs") {
		t.Error("output should contain Recent Jobs section")
	}
}

func TestOutputIngestQueue_JSON(t *testing.T) {
	jobs := getMockIngestQueue()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputIngestQueue(config.OutputFormatJSON, jobs)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputIngestQueue failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid JSON.
	var decoded []IngestJob
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestOutputIngestQueue_TextEmpty(t *testing.T) {
	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputIngestQueue(config.OutputFormatText, []IngestJob{})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputIngestQueue failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No pending") {
		t.Error("output should indicate no pending jobs")
	}
}

func TestOutputIngestQueue_Text(t *testing.T) {
	jobs := getMockIngestQueue()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputIngestQueue(config.OutputFormatText, jobs)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputIngestQueue failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Pending Ingestion Jobs") {
		t.Error("output should contain title")
	}
	if !strings.Contains(output, "PRIORITY") {
		t.Error("output should contain PRIORITY header")
	}
}

func TestOutputGmailStatus_JSON(t *testing.T) {
	status := getMockGmailStatus()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputGmailStatus(config.OutputFormatJSON, status)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputGmailStatus failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid JSON.
	var decoded GmailSyncStatus
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestOutputGmailStatus_Text(t *testing.T) {
	status := getMockGmailStatus()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputGmailStatus(config.OutputFormatText, status)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputGmailStatus failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Gmail Sync Status") {
		t.Error("output should contain title")
	}
	if !strings.Contains(output, "Connected") {
		t.Error("output should contain Connection status")
	}
}

func TestOutputGmailHistory_JSON(t *testing.T) {
	history := getMockGmailHistory(5)

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputGmailHistory(config.OutputFormatJSON, history)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputGmailHistory failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid JSON.
	var decoded []GmailSyncHistoryEntry
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestOutputGmailHistory_TextEmpty(t *testing.T) {
	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputGmailHistory(config.OutputFormatText, []GmailSyncHistoryEntry{})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputGmailHistory failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No Gmail sync history") {
		t.Error("output should indicate no history")
	}
}

func TestOutputGmailHistory_Text(t *testing.T) {
	history := getMockGmailHistory(5)

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputGmailHistory(config.OutputFormatText, history)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputGmailHistory failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Gmail Sync History") {
		t.Error("output should contain title")
	}
	if !strings.Contains(output, "STARTED") {
		t.Error("output should contain STARTED header")
	}
	if !strings.Contains(output, "ADDED") {
		t.Error("output should contain ADDED header")
	}
}

func TestOutputIngestConfig_JSON(t *testing.T) {
	cfg := getMockIngestConfig()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputIngestConfig(config.OutputFormatJSON, cfg)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputIngestConfig failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid JSON.
	var decoded IngestConfig
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestOutputIngestConfig_Text(t *testing.T) {
	cfg := getMockIngestConfig()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputIngestConfig(config.OutputFormatText, cfg)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputIngestConfig failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Ingestion Configuration") {
		t.Error("output should contain title")
	}
	if !strings.Contains(output, "auto_sync") {
		t.Error("output should contain auto_sync setting")
	}
	if !strings.Contains(output, "batch_size") {
		t.Error("output should contain batch_size setting")
	}
}

// The following tests require a running backend and should be moved to integration tests.
// They are skipped here to prevent panics from nil gRPC clients.
// TODO: Move to tests/integration/cli_ingest_test.go

func TestRunIngestStatus_Overall(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunIngestStatus_SpecificJob(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunIngestQueue(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunIngestGmailStatus(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunIngestGmailHistory(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunIngestGmailSync(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunIngestGmailSync_Full(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunIngestConfigShow(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunIngestConfigSet_ValidKey(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunIngestConfigSet_InvalidKey(t *testing.T) {
	cfg := mockConfig()
	deps := createIngestTestDeps(cfg)

	// Capture stdout.
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runIngestConfigSet(context.Background(), deps, "invalid_key", "value")

	w.Close()
	os.Stdout = oldStdout

	if err == nil {
		t.Error("expected error for invalid config key")
	}
	if !strings.Contains(err.Error(), "invalid configuration key") {
		t.Errorf("expected 'invalid configuration key' error, got: %v", err)
	}
}

func TestRunIngestURL_InvalidURL(t *testing.T) {
	cfg := mockConfig()
	deps := createIngestTestDeps(cfg)

	// Reset flags.
	ingestPriority = "normal"
	ingestAsync = false
	ingestOutput = ""

	// Capture stdout.
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runIngestURL(context.Background(), deps, "not-a-valid-url")

	w.Close()
	os.Stdout = oldStdout

	if err == nil {
		t.Error("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("expected 'invalid URL' error, got: %v", err)
	}
}

func TestRunIngestFile_InvalidPriority(t *testing.T) {
	cfg := mockConfig()
	deps := createIngestTestDeps(cfg)

	// Set invalid priority.
	ingestPriority = "invalid"
	ingestAsync = false
	ingestOutput = ""

	// Create a temporary file for testing.
	tmpFile, err := os.CreateTemp("", "test-ingest-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Capture stdout.
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err = runIngestFile(context.Background(), deps, tmpFile.Name())

	w.Close()
	os.Stdout = oldStdout

	if err == nil {
		t.Error("expected error for invalid priority")
	}
	if !strings.Contains(err.Error(), "invalid priority") {
		t.Errorf("expected 'invalid priority' error, got: %v", err)
	}

	// Reset priority.
	ingestPriority = "normal"
}

func TestRunIngestFile_NonExistentFile(t *testing.T) {
	cfg := mockConfig()
	deps := createIngestTestDeps(cfg)

	// Reset flags.
	ingestPriority = "normal"
	ingestAsync = false
	ingestOutput = ""

	// Capture stdout.
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runIngestFile(context.Background(), deps, "/nonexistent/path/to/file.pdf")

	w.Close()
	os.Stdout = oldStdout

	if err == nil {
		t.Error("expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("expected 'file not found' error, got: %v", err)
	}
}

func TestRunIngestBatch_NonExistentManifest(t *testing.T) {
	cfg := mockConfig()
	deps := createIngestTestDeps(cfg)

	// Reset flags.
	ingestPriority = "normal"
	ingestAsync = false
	ingestOutput = ""

	// Capture stdout.
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runIngestBatch(context.Background(), deps, "/nonexistent/manifest.yaml")

	w.Close()
	os.Stdout = oldStdout

	if err == nil {
		t.Error("expected error for non-existent manifest")
	}
	if !strings.Contains(err.Error(), "manifest file not found") {
		t.Errorf("expected 'manifest file not found' error, got: %v", err)
	}
}

func TestDefaultIngestDeps(t *testing.T) {
	deps := DefaultIngestDeps()

	if deps == nil {
		t.Fatal("DefaultIngestDeps returned nil")
	}
	if deps.LoadConfig == nil {
		t.Error("expected LoadConfig to be set")
	}
	if deps.SaveConfig == nil {
		t.Error("expected SaveConfig to be set")
	}
	if deps.InitClient == nil {
		t.Error("expected InitClient to be set")
	}
}

// =============================================================================
// Mock Helper Functions
// =============================================================================

// createMockIngestJob creates a mock ingest job for testing.
func createMockIngestJob(jobType, source string) IngestJob {
	return IngestJob{
		ID:        fmt.Sprintf("ingest-%s-%d", jobType, time.Now().UnixNano()),
		Type:      jobType,
		Source:    source,
		Status:    IngestJobStatusPending,
		Priority:  ingestPriority,
		CreatedAt: time.Now(),
		Tags:      ingestTags,
		Category:  ingestCategory,
		TenantID:  ingestTenantID,
	}
}

// getMockIngestJob returns a mock ingest job with the given ID.
func getMockIngestJob(id string) IngestJob {
	now := time.Now()
	started := now.Add(-5 * time.Minute)
	return IngestJob{
		ID:         id,
		Type:       "file",
		Source:     "/path/to/document.pdf",
		Status:     IngestJobStatusProcessing,
		Priority:   "normal",
		Progress:   50,
		Message:    "Processing page 5 of 10",
		CreatedAt:  now.Add(-10 * time.Minute),
		StartedAt:  &started,
		ItemsTotal: 10,
		ItemsDone:  5,
		Tags:       []string{"test"},
		Category:   "documents",
		TenantID:   "tenant-test-001",
	}
}

// getMockIngestStatus returns mock ingestion status.
func getMockIngestStatus() IngestStatusResponse {
	return IngestStatusResponse{
		TotalJobs:      100,
		PendingJobs:    10,
		ProcessingJobs: 5,
		CompletedJobs:  80,
		FailedJobs:     5,
		RecentJobs: []IngestJob{
			getMockIngestJob("recent-001"),
			getMockIngestJob("recent-002"),
		},
		ProcessingRate: 12.5,
		LastUpdated:    time.Now(),
	}
}

// getMockIngestQueue returns mock pending jobs.
func getMockIngestQueue() []IngestJob {
	return []IngestJob{
		{
			ID:        "queue-001",
			Type:      "file",
			Source:    "/docs/report.pdf",
			Status:    IngestJobStatusPending,
			Priority:  "high",
			CreatedAt: time.Now().Add(-2 * time.Minute),
			Tags:      []string{"urgent"},
			TenantID:  "tenant-test-001",
		},
		{
			ID:        "queue-002",
			Type:      "url",
			Source:    "https://example.com/article",
			Status:    IngestJobStatusPending,
			Priority:  "normal",
			CreatedAt: time.Now().Add(-5 * time.Minute),
			TenantID:  "tenant-test-001",
		},
		{
			ID:        "queue-003",
			Type:      "file",
			Source:    "/docs/manual.docx",
			Status:    IngestJobStatusPending,
			Priority:  "low",
			CreatedAt: time.Now().Add(-10 * time.Minute),
			Category:  "manuals",
			TenantID:  "tenant-test-001",
		},
	}
}

// getMockGmailStatus returns mock Gmail sync status.
func getMockGmailStatus() GmailSyncStatus {
	return GmailSyncStatus{
		Connected:    true,
		LastSyncAt:   time.Now().Add(-30 * time.Minute),
		NextSyncAt:   time.Now().Add(30 * time.Minute),
		TotalEmails:  5000,
		SyncedEmails: 4500,
		SyncState:    "idle",
	}
}

// getMockGmailHistory returns mock Gmail sync history entries.
func getMockGmailHistory(limit int) []GmailSyncHistoryEntry {
	entries := []GmailSyncHistoryEntry{
		{
			ID:            "sync-001",
			StartedAt:     time.Now().Add(-1 * time.Hour),
			CompletedAt:   time.Now().Add(-55 * time.Minute),
			EmailsAdded:   25,
			EmailsUpdated: 5,
			Status:        "completed",
		},
		{
			ID:            "sync-002",
			StartedAt:     time.Now().Add(-2 * time.Hour),
			CompletedAt:   time.Now().Add(-115 * time.Minute),
			EmailsAdded:   50,
			EmailsUpdated: 10,
			Status:        "completed",
		},
		{
			ID:            "sync-003",
			StartedAt:     time.Now().Add(-3 * time.Hour),
			CompletedAt:   time.Now().Add(-175 * time.Minute),
			EmailsAdded:   100,
			EmailsUpdated: 20,
			Status:        "completed",
		},
	}

	if limit > 0 && limit < len(entries) {
		return entries[:limit]
	}
	return entries
}

// getMockIngestConfig returns mock ingestion configuration.
func getMockIngestConfig() IngestConfig {
	return IngestConfig{
		AutoSync:        true,
		SyncInterval:    "1h",
		BatchSize:       50,
		MaxRetries:      3,
		DefaultPriority: "normal",
		DefaultCategory: "general",
		ExcludePatterns: []string{"*.tmp", "*.bak"},
	}
}

