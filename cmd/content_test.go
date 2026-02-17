// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	contentv1 "github.com/otherjamesbrown/penf-cli/api/proto/content/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

// captureStdout captures stdout during function execution
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// TestOutputContentTextJSON tests JSON output formatting for content text.
func TestOutputContentTextJSON(t *testing.T) {
	resp := &contentv1.GetContentTextResponse{
		ContentId:   "mt-abc123",
		ContentType: "meeting",
		Text:        "This is the meeting transcript content.",
		CreatedAt:   timestamppb.New(time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC)),
		Metadata: map[string]string{
			"subject": "Q1 Planning Meeting",
			"host":    "john@example.com",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal content text to JSON: %v", err)
	}

	var decoded contentv1.GetContentTextResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded.ContentId != resp.ContentId {
		t.Errorf("ContentId = %v, want %v", decoded.ContentId, resp.ContentId)
	}
	if decoded.ContentType != resp.ContentType {
		t.Errorf("ContentType = %v, want %v", decoded.ContentType, resp.ContentType)
	}
	if decoded.Text != resp.Text {
		t.Errorf("Text = %v, want %v", decoded.Text, resp.Text)
	}
}

// TestOutputContentTextYAML tests YAML output formatting for content text.
func TestOutputContentTextYAML(t *testing.T) {
	resp := &contentv1.GetContentTextResponse{
		ContentId:   "mt-abc123",
		ContentType: "meeting",
		Text:        "Test content",
		CreatedAt:   timestamppb.New(time.Now()),
	}

	data, err := yaml.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal content text to YAML: %v", err)
	}

	var decoded map[string]interface{}
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	if decoded["contentid"] != resp.ContentId {
		t.Errorf("ContentId = %v, want %v", decoded["contentid"], resp.ContentId)
	}
}

// TestOutputAvailableInsightsJSON tests JSON output for available insights.
func TestOutputAvailableInsightsJSON(t *testing.T) {
	resp := &contentv1.ListAvailableInsightsResponse{
		ContentId:   "mt-abc123",
		ContentType: "meeting",
		Available:   []string{"summary", "actions", "decisions", "risks"},
		Extracted:   []string{"summary", "actions"},
		Pending:     []string{"decisions", "risks"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal available insights to JSON: %v", err)
	}

	var decoded contentv1.ListAvailableInsightsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded.ContentId != resp.ContentId {
		t.Errorf("ContentId = %v, want %v", decoded.ContentId, resp.ContentId)
	}
	if len(decoded.Available) != 4 {
		t.Errorf("Available count = %d, want 4", len(decoded.Available))
	}
	if len(decoded.Extracted) != 2 {
		t.Errorf("Extracted count = %d, want 2", len(decoded.Extracted))
	}
	if len(decoded.Pending) != 2 {
		t.Errorf("Pending count = %d, want 2", len(decoded.Pending))
	}
}

// TestOutputInsightsJSON tests JSON output for insights.
func TestOutputInsightsJSON(t *testing.T) {
	// Create test insight data
	insightData, err := structpb.NewStruct(map[string]interface{}{
		"summary": "Team discussed Q1 planning and identified three key priorities.",
		"key_points": []interface{}{
			"Budget approval needed by Feb 1",
			"Hiring plan for 5 positions",
			"Marketing campaign timeline",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create insight data: %v", err)
	}

	resp := &contentv1.GetInsightsResponse{
		ContentId: "mt-abc123",
		Insights: []*contentv1.Insight{
			{
				Type:         "summary",
				Data:         insightData,
				ExtractedAt:  timestamppb.New(time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)),
				ModelVersion: "gpt-4-turbo",
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal insights to JSON: %v", err)
	}

	var decoded contentv1.GetInsightsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded.ContentId != resp.ContentId {
		t.Errorf("ContentId = %v, want %v", decoded.ContentId, resp.ContentId)
	}
	if len(decoded.Insights) != 1 {
		t.Errorf("Insights count = %d, want 1", len(decoded.Insights))
	}
	if decoded.Insights[0].Type != "summary" {
		t.Errorf("Insight type = %v, want summary", decoded.Insights[0].Type)
	}
	if decoded.Insights[0].ModelVersion != "gpt-4-turbo" {
		t.Errorf("ModelVersion = %v, want gpt-4-turbo", decoded.Insights[0].ModelVersion)
	}
}

// TestOutputContentText tests the outputContentText function routing.
func TestOutputContentText(t *testing.T) {
	resp := &contentv1.GetContentTextResponse{
		ContentId:   "mt-abc123",
		ContentType: "meeting",
		Text:        "Test content",
		CreatedAt:   timestamppb.New(time.Now()),
	}

	tests := []struct {
		name   string
		format config.OutputFormat
	}{
		{"json format", config.OutputFormatJSON},
		{"yaml format", config.OutputFormatYAML},
		{"text format", config.OutputFormatText},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// The function should not panic with any format
			err := outputContentText(tc.format, resp)
			if err != nil {
				t.Fatalf("outputContentText failed: %v", err)
			}
		})
	}
}

// TestOutputAvailableInsights tests the outputAvailableInsights function routing.
func TestOutputAvailableInsights(t *testing.T) {
	resp := &contentv1.ListAvailableInsightsResponse{
		ContentId:   "mt-abc123",
		ContentType: "meeting",
		Available:   []string{"summary", "actions"},
		Extracted:   []string{"summary"},
		Pending:     []string{"actions"},
	}

	tests := []struct {
		name   string
		format config.OutputFormat
	}{
		{"json format", config.OutputFormatJSON},
		{"yaml format", config.OutputFormatYAML},
		{"text format", config.OutputFormatText},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := outputAvailableInsights(tc.format, resp)
			if err != nil {
				t.Fatalf("outputAvailableInsights failed: %v", err)
			}
		})
	}
}

// TestOutputInsights tests the outputInsights function routing.
func TestOutputInsights(t *testing.T) {
	insightData, err := structpb.NewStruct(map[string]interface{}{
		"summary": "Test summary",
	})
	if err != nil {
		t.Fatalf("Failed to create insight data: %v", err)
	}

	resp := &contentv1.GetInsightsResponse{
		ContentId: "mt-abc123",
		Insights: []*contentv1.Insight{
			{
				Type:         "summary",
				Data:         insightData,
				ExtractedAt:  timestamppb.New(time.Now()),
				ModelVersion: "gpt-4",
			},
		},
	}

	tests := []struct {
		name   string
		format config.OutputFormat
	}{
		{"json format", config.OutputFormatJSON},
		{"yaml format", config.OutputFormatYAML},
		{"text format", config.OutputFormatText},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := outputInsights(tc.format, resp)
			if err != nil {
				t.Fatalf("outputInsights failed: %v", err)
			}
		})
	}
}

// TestDisplayInsightData tests the displayInsightData helper function.
func TestDisplayInsightData(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
	}{
		{
			name: "simple string values",
			data: map[string]interface{}{
				"summary": "Test summary",
				"status":  "completed",
			},
		},
		{
			name: "nested arrays",
			data: map[string]interface{}{
				"items": []interface{}{
					"item1",
					"item2",
					"item3",
				},
			},
		},
		{
			name: "nested objects",
			data: map[string]interface{}{
				"metadata": map[string]interface{}{
					"author": "John Doe",
					"date":   "2026-01-15",
				},
			},
		},
		{
			name: "mixed types",
			data: map[string]interface{}{
				"title":  "Test",
				"count":  42,
				"active": true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// The function should not panic with any input
			displayInsightData(tc.data, "  ")
		})
	}
}

// TestNewContentTextCommand tests the content text command creation.
func TestNewContentTextCommand(t *testing.T) {
	deps := DefaultContentDeps()
	cmd := newContentTextCommand(deps)

	if cmd == nil {
		t.Fatal("newContentTextCommand returned nil")
	}

	if cmd.Use != "text <content-id>" {
		t.Errorf("Use = %v, want 'text <content-id>'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Test that command requires exactly one argument
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("Command should require an argument")
	}

	if err := cmd.Args(cmd, []string{"mt-abc123"}); err != nil {
		t.Errorf("Command should accept one argument: %v", err)
	}

	if err := cmd.Args(cmd, []string{"mt-abc123", "extra"}); err == nil {
		t.Error("Command should not accept two arguments")
	}
}

// TestNewContentInsightsCommand tests the content insights command creation.
func TestNewContentInsightsCommand(t *testing.T) {
	deps := DefaultContentDeps()
	cmd := newContentInsightsCommand(deps)

	if cmd == nil {
		t.Fatal("newContentInsightsCommand returned nil")
	}

	if cmd.Use != "insights <content-id>" {
		t.Errorf("Use = %v, want 'insights <content-id>'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Check that flags are registered
	typeFlag := cmd.Flags().Lookup("type")
	if typeFlag == nil {
		t.Error("--type flag should be registered")
	}

	allFlag := cmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Error("--all flag should be registered")
	}

	refreshFlag := cmd.Flags().Lookup("refresh")
	if refreshFlag == nil {
		t.Error("--refresh flag should be registered")
	}

	// Test that command requires exactly one argument
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("Command should require an argument")
	}

	if err := cmd.Args(cmd, []string{"mt-abc123"}); err != nil {
		t.Errorf("Command should accept one argument: %v", err)
	}
}

// TestContentCommandIntegration tests that text and insights commands are registered.
func TestContentCommandIntegration(t *testing.T) {
	deps := DefaultContentDeps()
	rootCmd := NewContentCommand(deps)

	if rootCmd == nil {
		t.Fatal("NewContentCommand returned nil")
	}

	// Check that text command is registered
	textCmd := findCommand(rootCmd, "text")
	if textCmd == nil {
		t.Error("text command should be registered")
	}

	// Check that insights command is registered
	insightsCmd := findCommand(rootCmd, "insights")
	if insightsCmd == nil {
		t.Error("insights command should be registered")
	}
}

// Helper function to find a command by name
func findCommand(parent interface{ Commands() []*cobra.Command }, name string) interface{} {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

// TestSortMergedEvents tests the sortMergedEvents function.
func TestSortMergedEvents(t *testing.T) {
	tests := []struct {
		name     string
		input    []mergedEvent
		expected []string // timestamps as strings for easy comparison
	}{
		{
			name: "already sorted",
			input: []mergedEvent{
				{timestamp: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), source: "pipeline"},
				{timestamp: time.Date(2026, 1, 1, 10, 1, 0, 0, time.UTC), source: "langfuse"},
				{timestamp: time.Date(2026, 1, 1, 10, 2, 0, 0, time.UTC), source: "pipeline"},
			},
			expected: []string{"10:00", "10:01", "10:02"},
		},
		{
			name: "reverse order",
			input: []mergedEvent{
				{timestamp: time.Date(2026, 1, 1, 10, 2, 0, 0, time.UTC), source: "pipeline"},
				{timestamp: time.Date(2026, 1, 1, 10, 1, 0, 0, time.UTC), source: "langfuse"},
				{timestamp: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), source: "pipeline"},
			},
			expected: []string{"10:00", "10:01", "10:02"},
		},
		{
			name: "mixed order",
			input: []mergedEvent{
				{timestamp: time.Date(2026, 1, 1, 10, 1, 0, 0, time.UTC), source: "pipeline"},
				{timestamp: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), source: "langfuse"},
				{timestamp: time.Date(2026, 1, 1, 10, 3, 0, 0, time.UTC), source: "pipeline"},
				{timestamp: time.Date(2026, 1, 1, 10, 2, 0, 0, time.UTC), source: "langfuse"},
			},
			expected: []string{"10:00", "10:01", "10:02", "10:03"},
		},
		{
			name:     "empty slice",
			input:    []mergedEvent{},
			expected: []string{},
		},
		{
			name: "single element",
			input: []mergedEvent{
				{timestamp: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), source: "pipeline"},
			},
			expected: []string{"10:00"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sortMergedEvents(tc.input)

			if len(tc.input) != len(tc.expected) {
				t.Fatalf("Length mismatch: got %d, want %d", len(tc.input), len(tc.expected))
			}

			for i, event := range tc.input {
				got := event.timestamp.Format("15:04")
				if got != tc.expected[i] {
					t.Errorf("Position %d: got %s, want %s", i, got, tc.expected[i])
				}
			}
		})
	}
}

// TestFormatNumber tests the formatNumber function.
func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{99, "99"},
		{100, "100"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{1234567890, "1,234,567,890"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			got := formatNumber(tc.input)
			if got != tc.expected {
				t.Errorf("formatNumber(%d) = %s, want %s", tc.input, got, tc.expected)
			}
		})
	}
}

// TestNewContentTraceCommand tests the content trace command creation.
func TestNewContentTraceCommand(t *testing.T) {
	deps := DefaultContentDeps()
	cmd := newContentTraceCommand(deps)

	if cmd == nil {
		t.Fatal("newContentTraceCommand returned nil")
	}

	if cmd.Use != "trace <content-id>" {
		t.Errorf("Use = %v, want 'trace <content-id>'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Check that flags are registered
	sourceFlag := cmd.Flags().Lookup("source")
	if sourceFlag == nil {
		t.Error("--source flag should be registered")
	}
	if sourceFlag.DefValue != "all" {
		t.Errorf("--source default = %v, want 'all'", sourceFlag.DefValue)
	}

	envFlag := cmd.Flags().Lookup("env")
	if envFlag == nil {
		t.Error("--env flag should be registered")
	}

	verboseFlag := cmd.Flags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("--verbose flag should be registered")
	}

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("--output flag should be registered")
	}

	// Test that command requires exactly one argument
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("Command should require an argument")
	}

	if err := cmd.Args(cmd, []string{"em-abc123"}); err != nil {
		t.Errorf("Command should accept one argument: %v", err)
	}

	if err := cmd.Args(cmd, []string{"em-abc123", "extra"}); err == nil {
		t.Error("Command should not accept two arguments")
	}
}

// TestOutputContentItemText_EmailFields tests that email-specific fields are displayed.
// Tests for requirement pf-0e4eb5: enhance content show to display full email details.
func TestOutputContentItemText_EmailFields(t *testing.T) {
	item := &contentv1.ContentItem{
		Id:         "em-test123",
		SourceType: "email",
		SourceId:   "gmail-msg-456",
		TenantId:   "tenant-1",
		State:      contentv1.ProcessingState_PROCESSING_STATE_COMPLETED,
		RawContent: "This is the body of the email. It contains important information about the project timeline and budget considerations.",
		Metadata: map[string]string{
			"subject": "Q1 Project Update",
			"from":    "alice@example.com",
			"to":      "team@example.com",
			"cc":      "manager@example.com",
			"bcc":     "archive@example.com",
		},
		ContentHash: "abc123def456",
		CreatedAt:   timestamppb.New(time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC)),
		UpdatedAt:   timestamppb.New(time.Date(2026, 1, 15, 14, 35, 0, 0, time.UTC)),
	}

	// Capture output
	output := captureStdout(func() {
		err := outputContentItemText(item, nil, false)
		if err != nil {
			t.Errorf("outputContentItemText failed: %v", err)
		}
	})

	// After implementation, output should include email-specific fields in a dedicated section
	// Currently it only shows them in generic Metadata section
	requiredStrings := []string{
		"Q1 Project Update",     // Subject should be displayed
		"alice@example.com",     // From should be displayed
		"team@example.com",      // To should be displayed
		"manager@example.com",   // CC should be displayed
		"archive@example.com",   // BCC should be displayed
		"This is the body",      // Body text from raw_content should be displayed
	}

	// Check if output has email-specific formatting (not just generic metadata)
	// The feature should add a dedicated "Email:" section or similar
	// For now, we check that body text is shown (it's currently NOT shown)
	if !strings.Contains(output, "This is the body") {
		t.Error("Output should include body text from raw_content field")
		t.Logf("Output was:\n%s", output)
	}

	// Verify all email fields are present
	for _, required := range requiredStrings {
		if !strings.Contains(output, required) {
			t.Errorf("Output should contain '%s'", required)
		}
	}
}

// TestOutputContentItemText_BodyTruncation tests that body text is truncated at reasonable length.
// Tests for requirement pf-0e4eb5: body text truncation.
func TestOutputContentItemText_BodyTruncation(t *testing.T) {
	// Create a long body text (>2000 characters)
	longBody := "This is a very long email body. "
	for i := 0; i < 100; i++ {
		longBody += "Lorem ipsum dolor sit amet, consectetur adipiscing elit. "
	}

	item := &contentv1.ContentItem{
		Id:         "em-long123",
		SourceType: "email",
		SourceId:   "gmail-msg-789",
		TenantId:   "tenant-1",
		State:      contentv1.ProcessingState_PROCESSING_STATE_COMPLETED,
		RawContent: longBody,
		Metadata: map[string]string{
			"subject": "Long Email Test",
			"from":    "sender@example.com",
			"to":      "recipient@example.com",
		},
		ContentHash: "xyz789abc123",
		CreatedAt:   timestamppb.New(time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC)),
	}

	// Capture output
	output := captureStdout(func() {
		err := outputContentItemText(item, nil, false)
		if err != nil {
			t.Errorf("outputContentItemText failed: %v", err)
		}
	})

	// The feature should show body text from raw_content
	// Currently it doesn't show raw_content at all
	if !strings.Contains(output, "This is a very long email body") {
		t.Error("Output should include body text from raw_content field")
		t.Logf("Output was:\n%s", output)
	}

	// Body should be truncated - output should be significantly shorter than the full body
	// and should contain truncation indicator like "..." or "[truncated]"
	if len(output) > len(longBody) {
		// If output is longer than the body, it means no truncation occurred
		// (This is actually OK for now since we also show metadata)
		// The key test is that we don't show ALL of the long body
		bodyInOutput := strings.Contains(output, longBody)
		if bodyInOutput {
			t.Error("Output should truncate long body text, not show it in full")
		}
	}

	// Check for truncation indicator (will be added in implementation)
	// This assertion will fail until truncation is implemented
	hasTruncationIndicator := strings.Contains(output, "...") ||
		strings.Contains(output, "[truncated]") ||
		strings.Contains(output, "truncated")
	if !hasTruncationIndicator && len(longBody) > 2000 {
		t.Error("Long body text should show truncation indicator")
	}
}

// TestOutputContentItemText_FullFlag tests that --full flag shows untruncated body.
// Tests for requirement pf-0e4eb5: optional --full flag for untruncated body.
func TestOutputContentItemText_FullFlag(t *testing.T) {
	// Note: This test validates the concept of a full/truncated mode
	// The actual flag handling happens in the command, not in outputContentItemText
	// We'll need to add a parameter to outputContentItemText to control truncation

	longBody := "This is a complete email body that should be shown in full when --full flag is used. "
	for i := 0; i < 50; i++ {
		longBody += "Important details line " + string(rune(i)) + ". "
	}

	item := &contentv1.ContentItem{
		Id:         "em-full123",
		SourceType: "email",
		SourceId:   "gmail-msg-999",
		TenantId:   "tenant-1",
		State:      contentv1.ProcessingState_PROCESSING_STATE_COMPLETED,
		RawContent: longBody,
		Metadata: map[string]string{
			"subject": "Full Body Test",
			"from":    "sender@example.com",
			"to":      "recipient@example.com",
		},
		ContentHash: "full123",
		CreatedAt:   timestamppb.New(time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC)),
	}

	// This test should FAIL until the feature is implemented
	// Current implementation doesn't have a truncation parameter
	// After implementation, we need to add a parameter like:
	// outputContentItemText(item *ContentItem, status *ProcessingStatus, fullBody bool)
	err := outputContentItemText(item, nil, false)
	if err != nil {
		t.Fatalf("outputContentItemText failed: %v", err)
	}

	// TODO: Once implemented, test both modes:
	// 1. outputContentItemText(item, nil, false) - should truncate
	// 2. outputContentItemText(item, nil, true) - should show full body
}

// TestOutputContentItemText_NonEmailContent tests that non-email content still works.
// Tests backward compatibility for requirement pf-0e4eb5.
func TestOutputContentItemText_NonEmailContent(t *testing.T) {
	item := &contentv1.ContentItem{
		Id:         "mt-meeting123",
		SourceType: "meeting",
		SourceId:   "zoom-meeting-456",
		TenantId:   "tenant-1",
		State:      contentv1.ProcessingState_PROCESSING_STATE_COMPLETED,
		RawContent: "Meeting transcript text goes here.",
		Metadata: map[string]string{
			"title": "Weekly Standup",
			"host":  "alice@example.com",
		},
		ContentHash: "meeting123",
		CreatedAt:   timestamppb.New(time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC)),
	}

	// This test should continue to pass
	// Non-email content should still display properly after enhancement
	err := outputContentItemText(item, nil, false)
	if err != nil {
		t.Fatalf("outputContentItemText failed: %v", err)
	}

	// Expected: Should display metadata and content appropriately for meeting type
}

// TestOutputContentItemText_MissingEmailFields tests handling of missing optional fields.
// Tests robustness for requirement pf-0e4eb5.
func TestOutputContentItemText_MissingEmailFields(t *testing.T) {
	item := &contentv1.ContentItem{
		Id:         "em-minimal123",
		SourceType: "email",
		SourceId:   "gmail-msg-111",
		TenantId:   "tenant-1",
		State:      contentv1.ProcessingState_PROCESSING_STATE_COMPLETED,
		RawContent: "Minimal email body.",
		Metadata: map[string]string{
			"subject": "Minimal Email",
			"from":    "sender@example.com",
			// Note: no "to", "cc", or "bcc" fields
		},
		ContentHash: "minimal123",
		CreatedAt:   timestamppb.New(time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC)),
	}

	// This test should FAIL until the feature is implemented
	// After implementation, should gracefully handle missing CC/BCC fields
	// Should still display subject, from, and whatever fields are present
	err := outputContentItemText(item, nil, false)
	if err != nil {
		t.Fatalf("outputContentItemText failed: %v", err)
	}

	// Expected: Should display available fields, skip missing ones (no errors)
}

// TestNewContentShowCommand_FullFlag tests that --full flag is registered.
// Tests for requirement pf-0e4eb5: --full flag.
func TestNewContentShowCommand_FullFlag(t *testing.T) {
	deps := DefaultContentDeps()
	cmd := newContentShowCommand(deps)

	if cmd == nil {
		t.Fatal("newContentShowCommand returned nil")
	}

	// This test should FAIL until the --full flag is added
	fullFlag := cmd.Flags().Lookup("full")
	if fullFlag == nil {
		t.Error("--full flag should be registered for showing untruncated body")
		return
	}

	// Expected: --full flag should exist as a boolean flag
	// Default value should be false (truncate by default)
	if fullFlag.DefValue != "false" {
		t.Errorf("--full flag default = %v, want 'false'", fullFlag.DefValue)
	}

	// Verify flag type is boolean
	if fullFlag.Value.Type() != "bool" {
		t.Errorf("--full flag type = %v, want 'bool'", fullFlag.Value.Type())
	}
}
