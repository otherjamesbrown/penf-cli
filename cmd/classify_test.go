// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"

	contentv1 "github.com/otherjamesbrown/penf-cli/api/proto/content/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// TestClassifyCommand tests the classify command structure.
func TestClassifyCommand(t *testing.T) {
	cmd := NewClassifyCommand(nil)

	if cmd == nil {
		t.Fatal("NewClassifyCommand returned nil")
	}

	if cmd.Use != "classify" {
		t.Errorf("Use = %q, want %q", cmd.Use, "classify")
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	if cmd.Long == "" {
		t.Error("Long description is empty")
	}
}

// TestClassifyCommandSubcommands tests that classify has the expected subcommands.
func TestClassifyCommandSubcommands(t *testing.T) {
	cmd := NewClassifyCommand(nil)

	expectedSubcommands := []string{"run", "stats", "rules"}
	foundSubcommands := make(map[string]bool)

	for _, subCmd := range cmd.Commands() {
		foundSubcommands[subCmd.Name()] = true
	}

	for _, expected := range expectedSubcommands {
		if !foundSubcommands[expected] {
			t.Errorf("Missing subcommand %q", expected)
		}
	}

	if len(foundSubcommands) != len(expectedSubcommands) {
		t.Errorf("Expected %d subcommands, found %d", len(expectedSubcommands), len(foundSubcommands))
	}
}

// TestClassifyRunCommand tests the 'classify run' subcommand.
func TestClassifyRunCommand(t *testing.T) {
	cmd := NewClassifyCommand(nil)

	runCmd := findSubcommand(cmd, "run")
	if runCmd == nil {
		t.Fatal("run subcommand not found")
	}

	if runCmd.Use != "run [id]" {
		t.Errorf("Use = %q, want %q", runCmd.Use, "run [id]")
	}

	// Check that the command accepts 0 or 1 arguments (id is optional)
	if runCmd.Args != nil {
		// If Args is set, verify it allows 0 or 1 args
		// We'll test this by checking the error for different arg counts
		err0 := runCmd.Args(runCmd, []string{})
		err1 := runCmd.Args(runCmd, []string{"em-abc123"})
		err2 := runCmd.Args(runCmd, []string{"em-abc123", "extra"})

		if err0 != nil {
			t.Errorf("Args validation failed for 0 args: %v", err0)
		}
		if err1 != nil {
			t.Errorf("Args validation failed for 1 arg: %v", err1)
		}
		if err2 == nil {
			t.Error("Args validation should fail for 2+ args")
		}
	}
}

// TestClassifyRunFlags tests the flags on 'classify run' subcommand.
func TestClassifyRunFlags(t *testing.T) {
	cmd := NewClassifyCommand(nil)

	runCmd := findSubcommand(cmd, "run")
	if runCmd == nil {
		t.Fatal("run subcommand not found")
	}

	expectedFlags := []string{"all", "dry-run", "output"}

	for _, flagName := range expectedFlags {
		flag := runCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Missing flag --%s", flagName)
		}
	}

	// Check that --all is a bool flag
	allFlag := runCmd.Flags().Lookup("all")
	if allFlag != nil && allFlag.Value.Type() != "bool" {
		t.Errorf("Flag --all should be bool, got %s", allFlag.Value.Type())
	}

	// Check that --dry-run is a bool flag
	dryRunFlag := runCmd.Flags().Lookup("dry-run")
	if dryRunFlag != nil && dryRunFlag.Value.Type() != "bool" {
		t.Errorf("Flag --dry-run should be bool, got %s", dryRunFlag.Value.Type())
	}

	// Check that --output is a string flag
	outputFlag := runCmd.Flags().Lookup("output")
	if outputFlag != nil && outputFlag.Value.Type() != "string" {
		t.Errorf("Flag --output should be string, got %s", outputFlag.Value.Type())
	}
}

// TestClassifyStatsCommand tests the 'classify stats' subcommand.
func TestClassifyStatsCommand(t *testing.T) {
	cmd := NewClassifyCommand(nil)

	statsCmd := findSubcommand(cmd, "stats")
	if statsCmd == nil {
		t.Fatal("stats subcommand not found")
	}

	if statsCmd.Use != "stats" {
		t.Errorf("Use = %q, want %q", statsCmd.Use, "stats")
	}

	if statsCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if statsCmd.Long == "" {
		t.Error("Long description is empty")
	}

	// Verify it has no required args
	if statsCmd.Args != nil {
		err := statsCmd.Args(statsCmd, []string{})
		if err != nil {
			t.Errorf("stats should accept 0 args, got error: %v", err)
		}
	}
}

// TestClassifyStatsFlags tests the flags on 'classify stats' subcommand.
func TestClassifyStatsFlags(t *testing.T) {
	cmd := NewClassifyCommand(nil)

	statsCmd := findSubcommand(cmd, "stats")
	if statsCmd == nil {
		t.Fatal("stats subcommand not found")
	}

	// Check for output flag
	outputFlag := statsCmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("Missing flag --output")
	}

	if outputFlag != nil && outputFlag.Value.Type() != "string" {
		t.Errorf("Flag --output should be string, got %s", outputFlag.Value.Type())
	}
}

// TestClassifyRulesCommand tests the 'classify rules' subcommand.
func TestClassifyRulesCommand(t *testing.T) {
	cmd := NewClassifyCommand(nil)

	rulesCmd := findSubcommand(cmd, "rules")
	if rulesCmd == nil {
		t.Fatal("rules subcommand not found")
	}

	if rulesCmd.Use != "rules" {
		t.Errorf("Use = %q, want %q", rulesCmd.Use, "rules")
	}

	if rulesCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if rulesCmd.Long == "" {
		t.Error("Long description is empty")
	}

	// Verify it has no required args
	if rulesCmd.Args != nil {
		err := rulesCmd.Args(rulesCmd, []string{})
		if err != nil {
			t.Errorf("rules should accept 0 args, got error: %v", err)
		}
	}
}

// TestClassifyRulesFlags tests the flags on 'classify rules' subcommand.
func TestClassifyRulesFlags(t *testing.T) {
	cmd := NewClassifyCommand(nil)

	rulesCmd := findSubcommand(cmd, "rules")
	if rulesCmd == nil {
		t.Fatal("rules subcommand not found")
	}

	// Check for output flag
	outputFlag := rulesCmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("Missing flag --output")
	}

	if outputFlag != nil && outputFlag.Value.Type() != "string" {
		t.Errorf("Flag --output should be string, got %s", outputFlag.Value.Type())
	}
}

// TestClassifyCommandHelpText tests that help text follows AI-first guidelines.
func TestClassifyCommandHelpText(t *testing.T) {
	cmd := NewClassifyCommand(nil)

	// Root classify command should have examples
	if !containsExample(cmd.Example) {
		t.Error("classify command should have Example text for AI guidance")
	}

	// Check subcommands have proper help
	for _, subCmd := range cmd.Commands() {
		if subCmd.Short == "" {
			t.Errorf("Subcommand %q missing Short description", subCmd.Name())
		}

		if subCmd.Long == "" {
			t.Errorf("Subcommand %q missing Long description", subCmd.Name())
		}
	}
}

// TestClassifyOutputFormats tests that commands support multiple output formats.
func TestClassifyOutputFormats(t *testing.T) {
	cmd := NewClassifyCommand(nil)

	// Both stats and rules should support output formats
	testCases := []struct {
		subcommand string
	}{
		{"stats"},
		{"rules"},
		{"run"},
	}

	for _, tc := range testCases {
		subCmd := findSubcommand(cmd, tc.subcommand)
		if subCmd == nil {
			t.Fatalf("Subcommand %q not found", tc.subcommand)
		}

		outputFlag := subCmd.Flags().Lookup("output")
		if outputFlag == nil {
			t.Errorf("Subcommand %q missing --output flag", tc.subcommand)
		}
	}
}

// TestNewClassifyCommandWithDeps tests dependency injection pattern.
func TestNewClassifyCommandWithDeps(t *testing.T) {
	// Test with nil deps (should use defaults)
	cmd1 := NewClassifyCommand(nil)
	if cmd1 == nil {
		t.Error("NewClassifyCommand(nil) should use default deps")
	}

	// Test with custom deps
	customDeps := &ClassifyCommandDeps{
		LoadConfig: func() (*config.CLIConfig, error) {
			return &config.CLIConfig{
				ServerAddress: "test:50051",
			}, nil
		},
	}

	cmd2 := NewClassifyCommand(customDeps)
	if cmd2 == nil {
		t.Error("NewClassifyCommand with custom deps should work")
	}
}

// Helper functions

// findSubcommand finds a subcommand by name.
func findSubcommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, subCmd := range cmd.Commands() {
		if subCmd.Name() == name {
			return subCmd
		}
	}
	return nil
}

// containsExample checks if a string looks like it contains example usage.
func containsExample(s string) bool {
	// Simple heuristic: should contain "penf classify" to be a valid example
	return len(s) > 0 && (contains(s, "penf classify") || contains(s, "#"))
}

// ========================================
// ACCEPTANCE TESTS FOR CLASSIFY CLI WIRING (pf-11f81b)
// ========================================
// These tests verify the desired behavior after implementation.
// They SHOULD FAIL now because:
// 1. ClassifyCommandDeps doesn't have InitClient, GRPCClient, or mock function fields yet
// 2. runClassify() and runClassifyStats() return stub errors instead of calling RPCs

// TestClassifyRunSingleItem tests classify run with a single content item.
// After implementation: should call ReprocessContent RPC and return JSON with content_id, source_system, job_id.
func TestClassifyRunSingleItem(t *testing.T) {
	cfg := &config.CLIConfig{
		ServerAddress: "localhost:50051",
		Timeout:       30 * time.Second,
		OutputFormat:  config.OutputFormatJSON,
		TenantID:      "tenant-test-001",
	}

	deps := &ClassifyCommandDeps{
		Config: cfg,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		InitClient: func(c *config.CLIConfig) (*client.GRPCClient, error) {
			return nil, nil
		},
		// Mock function that will be called instead of real RPC
		ReprocessContentFn: func(ctx context.Context, contentID, reason string) (*contentv1.ReprocessContentResponse, error) {
			return &contentv1.ReprocessContentResponse{
				ContentId: contentID,
				JobId:     "job-abc123",
			}, nil
		},
		GetContentItemFn: func(ctx context.Context, contentID string, includeEmbedding bool) (*contentv1.ContentItem, error) {
			return &contentv1.ContentItem{
				Id: contentID,
				Metadata: map[string]string{
					"source_system": "human_email",
				},
			}, nil
		},
	}

	// Reset global flags
	oldOutput := classifyOutput
	oldDryRun := classifyDryRun
	oldAll := classifyAll
	classifyOutput = "json"
	classifyDryRun = false
	classifyAll = false
	defer func() {
		classifyOutput = oldOutput
		classifyDryRun = oldDryRun
		classifyAll = oldAll
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runClassify(ctx, deps, "em-test123")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// After implementation, this should succeed
	if err != nil {
		t.Errorf("runClassify should succeed, got error: %v", err)
	}

	// Verify JSON output contains expected fields
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output should be valid JSON: %v", err)
	}

	if result["content_id"] != "em-test123" {
		t.Errorf("output should contain content_id=em-test123, got: %v", result["content_id"])
	}
	if result["source_system"] != "human_email" {
		t.Errorf("output should contain source_system=human_email, got: %v", result["source_system"])
	}
	if result["job_id"] != "job-abc123" {
		t.Errorf("output should contain job_id=job-abc123, got: %v", result["job_id"])
	}
}

// TestClassifyRunBatchAll tests classify run --all with multiple items.
// After implementation: should call ListContentItems + ReprocessContent for each, return processed count.
func TestClassifyRunBatchAll(t *testing.T) {
	cfg := &config.CLIConfig{
		ServerAddress: "localhost:50051",
		Timeout:       30 * time.Second,
		OutputFormat:  config.OutputFormatJSON,
		TenantID:      "tenant-test-001",
	}

	processedItems := []string{}

	deps := &ClassifyCommandDeps{
		Config: cfg,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		InitClient: func(c *config.CLIConfig) (*client.GRPCClient, error) {
			return nil, nil
		},
		// Mock function to list content items
		ListContentItemsFn: func(ctx context.Context, req *contentv1.ListContentItemsRequest) (*contentv1.ListContentItemsResponse, error) {
			return &contentv1.ListContentItemsResponse{
				Items: []*contentv1.ContentItem{
					{Id: "em-001", Metadata: map[string]string{"source_system": "unknown"}},
					{Id: "em-002", Metadata: map[string]string{"source_system": "unknown"}},
					{Id: "em-003", Metadata: map[string]string{"source_system": "unknown"}},
				},
			}, nil
		},
		// Mock function to reprocess each item
		ReprocessContentFn: func(ctx context.Context, contentID, reason string) (*contentv1.ReprocessContentResponse, error) {
			processedItems = append(processedItems, contentID)
			return &contentv1.ReprocessContentResponse{
				ContentId: contentID,
				JobId:     "job-" + contentID,
			}, nil
		},
	}

	// Reset global flags
	oldOutput := classifyOutput
	oldDryRun := classifyDryRun
	oldAll := classifyAll
	classifyOutput = "json"
	classifyDryRun = false
	classifyAll = true
	defer func() {
		classifyOutput = oldOutput
		classifyDryRun = oldDryRun
		classifyAll = oldAll
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runClassify(ctx, deps, "") // Empty contentID means batch mode

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// After implementation, this should succeed
	if err != nil {
		t.Errorf("runClassify --all should succeed, got error: %v", err)
	}

	// Verify JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output should be valid JSON: %v", err)
	}

	// Check processed count
	processed, ok := result["processed"].(float64)
	if !ok || int(processed) != 3 {
		t.Errorf("output should contain processed=3, got: %v", result["processed"])
	}

	// Verify all items were processed
	if len(processedItems) != 3 {
		t.Errorf("should have processed 3 items, got: %d", len(processedItems))
	}
}

// TestClassifyRunDryRun tests classify run --dry-run for a single item.
// After implementation: should call GetContentItem, classify locally with ClassifySourceSystem(), return result without persisting.
func TestClassifyRunDryRun(t *testing.T) {
	cfg := &config.CLIConfig{
		ServerAddress: "localhost:50051",
		Timeout:       30 * time.Second,
		OutputFormat:  config.OutputFormatJSON,
		TenantID:      "tenant-test-001",
	}

	deps := &ClassifyCommandDeps{
		Config: cfg,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		InitClient: func(c *config.CLIConfig) (*client.GRPCClient, error) {
			return nil, nil
		},
		// Mock function to get content item metadata
		GetContentItemFn: func(ctx context.Context, contentID string, includeEmbedding bool) (*contentv1.ContentItem, error) {
			return &contentv1.ContentItem{
				Id: contentID,
				Metadata: map[string]string{
					"source_system": "unknown",
					"from":          "jira@example.atlassian.net",
					"subject":       "[PROJ-123] New issue created",
				},
			}, nil
		},
		// ReprocessContentFn should NOT be called in dry-run mode
		ReprocessContentFn: func(ctx context.Context, contentID, reason string) (*contentv1.ReprocessContentResponse, error) {
			t.Error("ReprocessContentFn should NOT be called in dry-run mode")
			return nil, fmt.Errorf("should not be called")
		},
	}

	// Reset global flags
	oldOutput := classifyOutput
	oldDryRun := classifyDryRun
	oldAll := classifyAll
	classifyOutput = "json"
	classifyDryRun = true
	classifyAll = false
	defer func() {
		classifyOutput = oldOutput
		classifyDryRun = oldDryRun
		classifyAll = oldAll
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runClassify(ctx, deps, "em-test123")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// After implementation, this should succeed
	if err != nil {
		t.Errorf("runClassify --dry-run should succeed, got error: %v", err)
	}

	// Verify JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output should be valid JSON: %v", err)
	}

	// Should show the classification result (likely "jira" based on mock data)
	if result["content_id"] != "em-test123" {
		t.Errorf("output should contain content_id=em-test123, got: %v", result["content_id"])
	}

	// Should indicate it was a dry-run
	dryRun, ok := result["dry_run"].(bool)
	if !ok || !dryRun {
		t.Errorf("output should contain dry_run=true, got: %v", result["dry_run"])
	}
}

// TestClassifyStats tests classify stats command.
// After implementation: should call GetContentStats RPC, return total + breakdown by source_system.
func TestClassifyStats(t *testing.T) {
	cfg := &config.CLIConfig{
		ServerAddress: "localhost:50051",
		Timeout:       30 * time.Second,
		OutputFormat:  config.OutputFormatJSON,
		TenantID:      "tenant-test-001",
	}

	// This will cause a compile error because these fields don't exist yet
	deps := &ClassifyCommandDeps{
		Config: cfg,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		InitClient: func(c *config.CLIConfig) (*client.GRPCClient, error) {
			return nil, nil
		},
		// Mock function to get classification stats
		GetContentStatsFn: func(ctx context.Context, tenantID string) (*contentv1.ContentStats, error) {
			return &contentv1.ContentStats{
				TenantId:   tenantID,
				TotalCount: 150,
				CountByType: map[string]int64{
					"human_email":      100,
					"jira":             25,
					"google_docs":      10,
					"webex":            5,
					"outlook_calendar": 5,
					"unknown":          5,
				},
			}, nil
		},
	}

	// Reset global flags
	oldOutput := classifyOutput
	classifyOutput = "json"
	defer func() {
		classifyOutput = oldOutput
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runClassifyStats(ctx, deps)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// After implementation, this should succeed
	if err != nil {
		t.Errorf("runClassifyStats should succeed, got error: %v", err)
	}

	// Verify JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output should be valid JSON: %v", err)
	}

	// Check total count
	total, ok := result["total"].(float64)
	if !ok || int(total) != 150 {
		t.Errorf("output should contain total=150, got: %v", result["total"])
	}

	// Check breakdown exists
	breakdown, ok := result["breakdown"].(map[string]interface{})
	if !ok {
		t.Errorf("output should contain breakdown map, got: %v", result["breakdown"])
	}

	// Verify breakdown has expected source systems
	humanEmail, ok := breakdown["human_email"].(float64)
	if !ok || int(humanEmail) != 100 {
		t.Errorf("breakdown should contain human_email=100, got: %v", breakdown["human_email"])
	}
}

// Mock types that will need to be defined in classify.go after implementation.
// These are documented here for reference.

// ReprocessContentResult represents the result of reprocessing a content item.
// type ReprocessContentResult struct {
// 	ContentID    string
// 	SourceSystem string
// 	JobID        string
// }

// ContentItemSummary represents a summary of a content item for listing.
// type ContentItemSummary struct {
// 	ID           string
// 	SourceSystem string
// }

// ContentListFilter represents filters for listing content items.
// type ContentListFilter struct {
// 	SourceSystem string
// 	Limit        int
// }

// ContentItemDetails represents full details of a content item.
// type ContentItemDetails struct {
// 	ID           string
// 	SourceSystem string
// 	Metadata     map[string]string
// }

// ContentStatsResult represents classification statistics.
// type ContentStatsResult struct {
// 	Total     int
// 	Breakdown map[string]int
// }
