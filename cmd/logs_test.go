// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// mockLogsConfig creates a mock configuration for logs command testing.
func mockLogsConfig() *config.CLIConfig {
	return &config.CLIConfig{
		ServerAddress: "localhost:50051",
		Timeout:       30 * time.Second,
		OutputFormat:  config.OutputFormatText,
		TenantID:      "tenant-test-001",
		Debug:         false,
		Insecure:      true,
	}
}

// createLogsTestDeps creates test dependencies for logs commands.
func createLogsTestDeps(cfg *config.CLIConfig) *LogsCommandDeps {
	return &LogsCommandDeps{
		Config:       cfg,
		OutputFormat: cfg.OutputFormat,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		InitClient: func(c *config.CLIConfig) (*client.GRPCClient, error) {
			return nil, nil
		},
	}
}

func TestNewLogsCommand(t *testing.T) {
	deps := createLogsTestDeps(mockLogsConfig())
	cmd := NewLogsCommand(deps)

	assert.NotNil(t, cmd)
	assert.Equal(t, "logs", cmd.Use)
	assert.Contains(t, cmd.Short, "logs")
}

func TestNewLogsCommand_WithNilDeps(t *testing.T) {
	cmd := NewLogsCommand(nil)
	assert.NotNil(t, cmd)
	assert.Equal(t, "logs", cmd.Use)
}

func TestLogsCommand_Flags(t *testing.T) {
	deps := createLogsTestDeps(mockLogsConfig())
	cmd := NewLogsCommand(deps)

	// Check flags.
	flags := []string{"service", "level", "since", "until", "contains", "limit", "follow", "output", "no-color"}
	for _, flagName := range flags {
		flag := cmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "logs command missing flag: %s", flagName)
	}
}

// TestRunLogs requires a running backend - move to integration tests.
// TODO: Move to tests/integration/cli_logs_test.go
func TestRunLogs(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunLogs_JSONOutput(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunLogs_YAMLOutput(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunLogs_ServiceFilter(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunLogs_LevelFilter(t *testing.T) {
	t.Skip("Requires running backend - move to integration tests")
}

func TestRunLogs_InvalidLevel(t *testing.T) {
	cfg := mockLogsConfig()
	deps := createLogsTestDeps(cfg)

	oldLevel := logsLevel
	logsLevel = "invalid_level"
	defer func() {
		logsLevel = oldLevel
	}()

	ctx := context.Background()
	err := runLogs(ctx, deps)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level")
}

func TestRunLogs_InvalidSinceDuration(t *testing.T) {
	cfg := mockLogsConfig()
	deps := createLogsTestDeps(cfg)

	oldSince := logsSince
	logsSince = "invalid"
	defer func() {
		logsSince = oldSince
	}()

	ctx := context.Background()
	err := runLogs(ctx, deps)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --since duration")
}

func TestRunLogs_InvalidUntilDuration(t *testing.T) {
	cfg := mockLogsConfig()
	deps := createLogsTestDeps(cfg)

	oldSince := logsSince
	oldUntil := logsUntil
	logsSince = ""
	logsUntil = "invalid"
	defer func() {
		logsSince = oldSince
		logsUntil = oldUntil
	}()

	ctx := context.Background()
	err := runLogs(ctx, deps)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --until duration")
}

func TestRunLogs_InvalidOutputFormat(t *testing.T) {
	cfg := mockLogsConfig()
	deps := createLogsTestDeps(cfg)

	oldOutput := logsOutput
	logsOutput = "invalid"
	defer func() {
		logsOutput = oldOutput
	}()

	ctx := context.Background()
	err := runLogs(ctx, deps)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid output format")
}

func TestGetMockLogs(t *testing.T) {
	query := LogQuery{
		Limit: 100,
	}

	entries := getMockLogs(query)
	assert.True(t, len(entries) > 0)
}

func TestGetMockLogs_ServiceFilter(t *testing.T) {
	query := LogQuery{
		Service: "gateway",
		Limit:   100,
	}

	entries := getMockLogs(query)

	for _, entry := range entries {
		assert.Equal(t, "gateway", entry.Service)
	}
}

func TestGetMockLogs_LevelFilter(t *testing.T) {
	query := LogQuery{
		Level: "error",
		Limit: 100,
	}

	entries := getMockLogs(query)

	// Should only include entries at or above error level.
	for _, entry := range entries {
		assert.Equal(t, LogLevelError, entry.Level)
	}
}

func TestGetMockLogs_ContainsFilter(t *testing.T) {
	query := LogQuery{
		Contains: "connection",
		Limit:    100,
	}

	entries := getMockLogs(query)

	for _, entry := range entries {
		// Filter is case-insensitive, so verify lowercase match
		assert.Contains(t, strings.ToLower(entry.Message), "connection")
	}
}

func TestGetMockLogs_Limit(t *testing.T) {
	query := LogQuery{
		Limit: 2,
	}

	entries := getMockLogs(query)
	assert.LessOrEqual(t, len(entries), 2)
}

func TestLogLevelMatches(t *testing.T) {
	tests := []struct {
		entryLevel LogLevel
		minLevel   LogLevel
		expected   bool
	}{
		{LogLevelDebug, LogLevelDebug, true},
		{LogLevelInfo, LogLevelDebug, true},
		{LogLevelWarn, LogLevelDebug, true},
		{LogLevelError, LogLevelDebug, true},
		{LogLevelDebug, LogLevelInfo, false},
		{LogLevelInfo, LogLevelInfo, true},
		{LogLevelWarn, LogLevelInfo, true},
		{LogLevelError, LogLevelInfo, true},
		{LogLevelDebug, LogLevelWarn, false},
		{LogLevelInfo, LogLevelWarn, false},
		{LogLevelWarn, LogLevelWarn, true},
		{LogLevelError, LogLevelWarn, true},
		{LogLevelDebug, LogLevelError, false},
		{LogLevelInfo, LogLevelError, false},
		{LogLevelWarn, LogLevelError, false},
		{LogLevelError, LogLevelError, true},
	}

	for _, tc := range tests {
		t.Run(string(tc.entryLevel)+"_"+string(tc.minLevel), func(t *testing.T) {
			result := logLevelMatches(tc.entryLevel, tc.minLevel)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestLogLevel_Constants(t *testing.T) {
	assert.Equal(t, LogLevel("debug"), LogLevelDebug)
	assert.Equal(t, LogLevel("info"), LogLevelInfo)
	assert.Equal(t, LogLevel("warn"), LogLevelWarn)
	assert.Equal(t, LogLevel("error"), LogLevelError)
}

func TestLogEntry_JSONSerialization(t *testing.T) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Service:   "gateway",
		Message:   "Test message",
		Fields: map[string]string{
			"key1": "value1",
		},
		TraceID: "trace-123",
	}

	data, err := json.Marshal(entry)
	require.NoError(t, err)

	var decoded LogEntry
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, entry.Level, decoded.Level)
	assert.Equal(t, entry.Service, decoded.Service)
	assert.Equal(t, entry.Message, decoded.Message)
	assert.Equal(t, entry.TraceID, decoded.TraceID)
}

func TestLogsResponse_JSONSerialization(t *testing.T) {
	response := LogsResponse{
		Entries: []LogEntry{
			{Level: LogLevelInfo, Service: "gateway", Message: "Test 1"},
			{Level: LogLevelError, Service: "ai_service", Message: "Test 2"},
		},
		TotalCount: 2,
		Truncated:  false,
		Query: LogQuery{
			Service: "gateway",
			Limit:   100,
		},
		FetchedAt: time.Now(),
	}

	data, err := json.Marshal(response)
	require.NoError(t, err)

	var decoded LogsResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Entries, 2)
	assert.Equal(t, 2, decoded.TotalCount)
	assert.False(t, decoded.Truncated)
}

func TestOutputLogs_EmptyList(t *testing.T) {
	response := LogsResponse{
		Entries:    []LogEntry{},
		TotalCount: 0,
		Truncated:  false,
		FetchedAt:  time.Now(),
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputLogs(config.OutputFormatText, response)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "No log entries found")
}

func TestOutputLogs_Truncated(t *testing.T) {
	response := LogsResponse{
		Entries: []LogEntry{
			{Level: LogLevelInfo, Service: "gateway", Message: "Test"},
		},
		TotalCount: 1,
		Truncated:  true,
		FetchedAt:  time.Now(),
	}

	// Reset global flag.
	oldNoColor := logsNoColor
	logsNoColor = true
	defer func() {
		logsNoColor = oldNoColor
	}()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputLogs(config.OutputFormatText, response)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "use --limit")
}

func TestOutputLogEntry_NoColor(t *testing.T) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Service:   "gateway",
		Message:   "Test message",
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputLogEntry(entry, true)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should not contain ANSI escape codes.
	assert.NotContains(t, output, "\033[")
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "gateway")
	assert.Contains(t, output, "Test message")
}

func TestOutputLogEntry_WithFields(t *testing.T) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Service:   "gateway",
		Message:   "Test message",
		Fields: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputLogEntry(entry, true)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, "key1=value1")
}

func TestOutputLogEntry_WithManyFields(t *testing.T) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Service:   "gateway",
		Message:   "Test message",
		Fields: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputLogEntry(entry, true)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// With more than 3 fields, should be on separate lines.
	assert.Contains(t, output, "key1=value1")
}

func TestGetLogLevelColor(t *testing.T) {
	tests := []struct {
		level    LogLevel
		noColor  bool
		expected string
	}{
		{LogLevelDebug, false, "\033[90m"},
		{LogLevelInfo, false, "\033[32m"},
		{LogLevelWarn, false, "\033[33m"},
		{LogLevelError, false, "\033[31m"},
		{LogLevel("unknown"), false, ""},
		{LogLevelInfo, true, ""},
	}

	for _, tc := range tests {
		t.Run(string(tc.level), func(t *testing.T) {
			color := getLogLevelColor(tc.level, tc.noColor)
			assert.Equal(t, tc.expected, color)
		})
	}
}

func TestDefaultLogsDeps(t *testing.T) {
	deps := DefaultLogsDeps()

	assert.NotNil(t, deps)
	assert.NotNil(t, deps.LoadConfig)
	assert.NotNil(t, deps.InitClient)
}

// =============================================================================
// Mock Helper Functions
// =============================================================================

// getMockLogs returns mock log entries filtered by the query.
func getMockLogs(query LogQuery) []LogEntry {
	// Create sample log entries
	allEntries := []LogEntry{
		{
			Timestamp: time.Now().Add(-5 * time.Minute),
			Level:     LogLevelInfo,
			Service:   "gateway",
			Message:   "Connection established successfully",
			Fields:    map[string]string{"client_ip": "192.168.1.1"},
		},
		{
			Timestamp: time.Now().Add(-4 * time.Minute),
			Level:     LogLevelDebug,
			Service:   "gateway",
			Message:   "Processing request",
			Fields:    map[string]string{"method": "GET", "path": "/api/v1/health"},
		},
		{
			Timestamp: time.Now().Add(-3 * time.Minute),
			Level:     LogLevelError,
			Service:   "ai_service",
			Message:   "Failed to generate embedding",
			Fields:    map[string]string{"error": "timeout"},
		},
		{
			Timestamp: time.Now().Add(-2 * time.Minute),
			Level:     LogLevelWarn,
			Service:   "worker",
			Message:   "Slow query detected",
			Fields:    map[string]string{"duration": "5.2s"},
		},
		{
			Timestamp: time.Now().Add(-1 * time.Minute),
			Level:     LogLevelError,
			Service:   "gateway",
			Message:   "Connection timeout",
			Fields:    map[string]string{"client_ip": "192.168.1.2"},
		},
	}

	// Apply filters
	var filtered []LogEntry
	for _, entry := range allEntries {
		// Service filter
		if query.Service != "" && entry.Service != query.Service {
			continue
		}

		// Level filter
		if query.Level != "" {
			minLevel := LogLevel(query.Level)
			if !logLevelMatches(entry.Level, minLevel) {
				continue
			}
		}

		// Contains filter (case-insensitive)
		if query.Contains != "" {
			if !strings.Contains(strings.ToLower(entry.Message), strings.ToLower(query.Contains)) {
				continue
			}
		}

		filtered = append(filtered, entry)
	}

	// Apply limit
	if query.Limit > 0 && len(filtered) > query.Limit {
		filtered = filtered[:query.Limit]
	}

	return filtered
}
