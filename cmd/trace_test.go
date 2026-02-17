package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/otherjamesbrown/penf-cli/config"
)

func TestContentTypeFullName(t *testing.T) {
	tests := []struct {
		prefix   string
		expected string
	}{
		{"em", "email"},
		{"mt", "meeting"},
		{"dc", "document"},
		{"tr", "transcript"},
		{"at", "attachment"},
		{"xx", "xx"}, // Unknown type returns as-is.
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			result := contentTypeFullName(tt.prefix)
			if result != tt.expected {
				t.Errorf("contentTypeFullName(%q) = %q, want %q", tt.prefix, result, tt.expected)
			}
		})
	}
}

func TestBuildLangfuseURL(t *testing.T) {
	tests := []struct {
		name      string
		host      string
		contentID string
		wantURL   string
	}{
		{
			name:      "basic document",
			host:      "dev02.brown.chat:3000",
			contentID: "dc-9x3kp7mn",
			wantURL:   "https://dev02.brown.chat:3000/traces?filter=penfold.content_id%3Ddc-9x3kp7mn",
		},
		{
			name:      "email content",
			host:      "langfuse.example.com",
			contentID: "em-abc12345",
			wantURL:   "https://langfuse.example.com/traces?filter=penfold.content_id%3Dem-abc12345",
		},
		{
			name:      "meeting content",
			host:      "localhost:3000",
			contentID: "mt-XYZ98765",
			wantURL:   "https://localhost:3000/traces?filter=penfold.content_id%3Dmt-XYZ98765",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildLangfuseURL(tt.host, tt.contentID)
			if result != tt.wantURL {
				t.Errorf("buildLangfuseURL(%q, %q) = %q, want %q",
					tt.host, tt.contentID, result, tt.wantURL)
			}
		})
	}
}

func TestOutputTraceResultsText(t *testing.T) {
	output := TraceOutput{
		ContentID:   "dc-9x3kp7mn",
		ContentType: "document",
		LangfuseURL: "https://dev02.brown.chat:3000/traces?filter=penfold.content_id%3Ddc-9x3kp7mn",
	}

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputTraceResultsText(output)
	if err != nil {
		t.Fatalf("outputTraceResultsText() error = %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	result := buf.String()

	// Check expected content.
	expectedLines := []string{
		"Content: dc-9x3kp7mn",
		"Type:    document",
		"Langfuse Traces:",
		"https://dev02.brown.chat:3000/traces?filter=penfold.content_id%3Ddc-9x3kp7mn",
	}

	for _, line := range expectedLines {
		if !strings.Contains(result, line) {
			t.Errorf("output missing expected line: %q\nGot: %s", line, result)
		}
	}
}

func TestOutputTraceResultsJSON(t *testing.T) {
	output := TraceOutput{
		ContentID:   "em-abc12345",
		ContentType: "email",
		LangfuseURL: "https://dev02.brown.chat:3000/traces?filter=penfold.content_id%3Dem-abc12345",
	}

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputTraceResults(config.OutputFormatJSON, output)
	if err != nil {
		t.Fatalf("outputTraceResults() error = %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	// Parse JSON output.
	var result TraceOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, buf.String())
	}

	// Verify fields.
	if result.ContentID != output.ContentID {
		t.Errorf("content_id = %q, want %q", result.ContentID, output.ContentID)
	}
	if result.ContentType != output.ContentType {
		t.Errorf("type = %q, want %q", result.ContentType, output.ContentType)
	}
	if result.LangfuseURL != output.LangfuseURL {
		t.Errorf("langfuse_url = %q, want %q", result.LangfuseURL, output.LangfuseURL)
	}
}

func TestNewTraceCommand(t *testing.T) {
	deps := DefaultTraceDeps()
	cmd := NewTraceCommand(deps)

	// Check command configuration.
	if cmd.Use != "trace <content_id>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "trace <content_id>")
	}

	// Check flags.
	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("missing --output flag")
	}
	if outputFlag.Shorthand != "o" {
		t.Errorf("output flag shorthand = %q, want %q", outputFlag.Shorthand, "o")
	}
}

func TestRunTraceValidation(t *testing.T) {
	deps := &TraceCommandDeps{
		LoadConfig: func() (*config.CLIConfig, error) {
			return config.DefaultConfig(), nil
		},
		LangfuseHost: "langfuse.test:3000",
	}

	cmd := NewTraceCommand(deps)

	tests := []struct {
		name      string
		contentID string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid document ID",
			contentID: "dc-0000aaaa",
			wantErr:   false,
		},
		{
			name:      "valid email ID",
			contentID: "em-1234abcd",
			wantErr:   false,
		},
		{
			name:      "invalid format - too short",
			contentID: "dc-abc",
			wantErr:   true,
			errMsg:    "invalid content ID",
		},
		{
			name:      "invalid format - no dash",
			contentID: "dcabc12345",
			wantErr:   true,
			errMsg:    "invalid content ID",
		},
		{
			name:      "invalid type",
			contentID: "xx-12345678",
			wantErr:   true,
			errMsg:    "invalid content ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset output flag for each test.
			traceOutput = ""

			// Capture stdout to avoid polluting test output.
			oldStdout := os.Stdout
			_, w, _ := os.Pipe()
			os.Stdout = w

			err := runTrace(cmd, deps, tt.contentID)

			w.Close()
			os.Stdout = oldStdout

			if tt.wantErr {
				if err == nil {
					t.Errorf("runTrace(%q) expected error, got nil", tt.contentID)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("runTrace(%q) error = %v, want error containing %q", tt.contentID, err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("runTrace(%q) unexpected error: %v", tt.contentID, err)
				}
			}
		})
	}
}
