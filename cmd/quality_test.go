// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/config"
)

// =============================================================================
// Quality Command Structure Tests
// =============================================================================

// TestNewQualityCommand verifies the quality command group is registered.
func TestNewQualityCommand(t *testing.T) {
	cfg := mockConfig()
	deps := createQualityTestDeps(cfg)
	cmd := NewQualityCommand(deps)

	if cmd == nil {
		t.Fatal("NewQualityCommand returned nil")
	}

	if cmd.Use != "quality" {
		t.Errorf("expected Use to be 'quality', got %q", cmd.Use)
	}

	// Check aliases
	expectedAliases := []string{"qual", "q"}
	for _, expected := range expectedAliases {
		found := false
		for _, alias := range cmd.Aliases {
			if alias == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected alias %q not found", expected)
		}
	}

	// Check that help text exists and mentions the purpose
	if cmd.Short == "" {
		t.Error("quality command should have Short description")
	}
	if cmd.Long == "" {
		t.Error("quality command should have Long description")
	}
}

// TestQualitySubcommands verifies all required subcommands exist.
func TestQualitySubcommands(t *testing.T) {
	cfg := mockConfig()
	deps := createQualityTestDeps(cfg)
	cmd := NewQualityCommand(deps)

	subcommands := cmd.Commands()
	expectedSubcmds := []string{"summary", "entities", "extractions"}

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

// =============================================================================
// Quality Summary Tests
// =============================================================================

// TestQualitySummaryCommand verifies the summary subcommand structure.
func TestQualitySummaryCommand(t *testing.T) {
	cfg := mockConfig()
	deps := createQualityTestDeps(cfg)
	cmd := NewQualityCommand(deps)

	summaryCmd, _, err := cmd.Find([]string{"summary"})
	if err != nil {
		t.Fatalf("failed to find summary command: %v", err)
	}

	if summaryCmd.Use != "summary" {
		t.Errorf("expected Use to be 'summary', got %q", summaryCmd.Use)
	}

	// Verify help text includes context for AI agents
	if summaryCmd.Short == "" {
		t.Error("summary command should have Short description")
	}
	if summaryCmd.Long == "" {
		t.Error("summary command should have Long description for AI agents")
	}
	if summaryCmd.Example == "" {
		t.Error("summary command should have Example usage")
	}
}

// TestQualitySummaryOutput_Text verifies text output with severity counts.
func TestQualitySummaryOutput_Text(t *testing.T) {
	summary := &QualitySummary{
		HighCount:   5,
		MediumCount: 12,
		LowCount:    3,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputQualitySummary(config.OutputFormatText, summary)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputQualitySummary text failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify severity counts are displayed
	if !strings.Contains(output, "5") {
		t.Error("output should contain high severity count '5'")
	}
	if !strings.Contains(output, "12") {
		t.Error("output should contain medium severity count '12'")
	}
	if !strings.Contains(output, "3") {
		t.Error("output should contain low severity count '3'")
	}

	// Verify severity labels are present
	expectedLabels := []string{"HIGH", "MEDIUM", "LOW"}
	for _, label := range expectedLabels {
		if !strings.Contains(output, label) {
			t.Errorf("output should contain severity label %q", label)
		}
	}
}

// TestQualitySummaryOutput_ColoredSeverity verifies severity-colored output.
func TestQualitySummaryOutput_ColoredSeverity(t *testing.T) {
	summary := &QualitySummary{
		HighCount:   5,
		MediumCount: 12,
		LowCount:    3,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputQualitySummary(config.OutputFormatText, summary)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputQualitySummary failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify ANSI color codes are present
	// HIGH should be red (31m)
	if !strings.Contains(output, "\033[31m") {
		t.Error("output should contain red color code for HIGH severity")
	}
	// MEDIUM should be yellow (33m)
	if !strings.Contains(output, "\033[33m") {
		t.Error("output should contain yellow color code for MEDIUM severity")
	}
	// LOW should be blue (34m)
	if !strings.Contains(output, "\033[34m") {
		t.Error("output should contain blue color code for LOW severity")
	}
}

// TestQualitySummaryOutput_JSON verifies JSON output format.
func TestQualitySummaryOutput_JSON(t *testing.T) {
	summary := &QualitySummary{
		HighCount:   5,
		MediumCount: 12,
		LowCount:    3,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputQualitySummary(config.OutputFormatJSON, summary)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputQualitySummary JSON failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid JSON
	var decoded QualitySummary
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if decoded.HighCount != summary.HighCount {
		t.Errorf("HighCount = %d, want %d", decoded.HighCount, summary.HighCount)
	}
	if decoded.MediumCount != summary.MediumCount {
		t.Errorf("MediumCount = %d, want %d", decoded.MediumCount, summary.MediumCount)
	}
	if decoded.LowCount != summary.LowCount {
		t.Errorf("LowCount = %d, want %d", decoded.LowCount, summary.LowCount)
	}
}

// TestQualitySummaryOutput_YAML verifies YAML output format.
func TestQualitySummaryOutput_YAML(t *testing.T) {
	summary := &QualitySummary{
		HighCount:   5,
		MediumCount: 12,
		LowCount:    3,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputQualitySummary(config.OutputFormatYAML, summary)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputQualitySummary YAML failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid YAML
	var decoded QualitySummary
	if err := yaml.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid YAML: %v", err)
	}
}

// =============================================================================
// Entity Quality Tests
// =============================================================================

// TestQualityEntitiesCommand verifies the entities subcommand structure.
func TestQualityEntitiesCommand(t *testing.T) {
	cfg := mockConfig()
	deps := createQualityTestDeps(cfg)
	cmd := NewQualityCommand(deps)

	entitiesCmd, _, err := cmd.Find([]string{"entities"})
	if err != nil {
		t.Fatalf("failed to find entities command: %v", err)
	}

	if entitiesCmd.Use != "entities" {
		t.Errorf("expected Use to be 'entities', got %q", entitiesCmd.Use)
	}

	// Verify limit flag exists
	limitFlag := entitiesCmd.Flags().Lookup("limit")
	if limitFlag == nil {
		t.Error("entities command should have --limit flag")
	}
}

// TestQualityEntitiesOutput_Text verifies entity list with issues.
func TestQualityEntitiesOutput_Text(t *testing.T) {
	entities := []EntityQualityItem{
		{
			EntityID:     1,
			EntityName:   "John Doe",
			PrimaryEmail: "john@example.com",
			Confidence:   0.65,
			Issues: []QualityIssue{
				{
					Severity:         "HIGH",
					Category:         "entity",
					Description:      "Low confidence score",
					SuggestedCommand: "penf relationship entity show 1",
				},
			},
		},
		{
			EntityID:     2,
			EntityName:   "Jane Smith",
			PrimaryEmail: "jane@example.com",
			Confidence:   0.85,
			Issues: []QualityIssue{
				{
					Severity:         "MEDIUM",
					Category:         "entity",
					Description:      "Multiple aliases need review",
					SuggestedCommand: "penf relationship entity show 2",
				},
			},
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputEntityQuality(config.OutputFormatText, entities)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputEntityQuality text failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify entity names are displayed
	if !strings.Contains(output, "John Doe") {
		t.Error("output should contain entity name 'John Doe'")
	}
	if !strings.Contains(output, "Jane Smith") {
		t.Error("output should contain entity name 'Jane Smith'")
	}

	// Verify suggested commands are displayed
	if !strings.Contains(output, "penf relationship entity show") {
		t.Error("output should contain suggested command")
	}

	// Verify severity labels with colors
	if !strings.Contains(output, "HIGH") {
		t.Error("output should contain HIGH severity")
	}
	if !strings.Contains(output, "MEDIUM") {
		t.Error("output should contain MEDIUM severity")
	}
}

// TestQualityEntitiesOutput_SortedBySeverity verifies entities are sorted by severity.
func TestQualityEntitiesOutput_SortedBySeverity(t *testing.T) {
	entities := []EntityQualityItem{
		{
			EntityID:   1,
			EntityName: "Low Priority",
			Issues: []QualityIssue{
				{Severity: "LOW", Description: "Minor issue"},
			},
		},
		{
			EntityID:   2,
			EntityName: "High Priority",
			Issues: []QualityIssue{
				{Severity: "HIGH", Description: "Critical issue"},
			},
		},
		{
			EntityID:   3,
			EntityName: "Medium Priority",
			Issues: []QualityIssue{
				{Severity: "MEDIUM", Description: "Moderate issue"},
			},
		},
	}

	// After sorting, HIGH should come before MEDIUM, MEDIUM before LOW
	sorted := sortEntitiesBySeverity(entities)

	if len(sorted) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(sorted))
	}

	// Verify HIGH is first
	if sorted[0].Issues[0].Severity != "HIGH" {
		t.Errorf("first entity should have HIGH severity, got %s", sorted[0].Issues[0].Severity)
	}

	// Verify MEDIUM is second
	if sorted[1].Issues[0].Severity != "MEDIUM" {
		t.Errorf("second entity should have MEDIUM severity, got %s", sorted[1].Issues[0].Severity)
	}

	// Verify LOW is third
	if sorted[2].Issues[0].Severity != "LOW" {
		t.Errorf("third entity should have LOW severity, got %s", sorted[2].Issues[0].Severity)
	}
}

// TestQualityEntitiesOutput_JSON verifies JSON output format.
func TestQualityEntitiesOutput_JSON(t *testing.T) {
	entities := []EntityQualityItem{
		{
			EntityID:     1,
			EntityName:   "Test Entity",
			PrimaryEmail: "test@example.com",
			Confidence:   0.75,
			Issues: []QualityIssue{
				{
					Severity:    "HIGH",
					Category:    "entity",
					Description: "Test issue",
				},
			},
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputEntityQuality(config.OutputFormatJSON, entities)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputEntityQuality JSON failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid JSON
	var decoded []EntityQualityItem
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(decoded) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(decoded))
	}

	if decoded[0].EntityID != entities[0].EntityID {
		t.Errorf("EntityID = %d, want %d", decoded[0].EntityID, entities[0].EntityID)
	}
}

// TestQualityEntitiesOutput_Empty verifies handling of no issues.
func TestQualityEntitiesOutput_Empty(t *testing.T) {
	var entities []EntityQualityItem

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputEntityQuality(config.OutputFormatText, entities)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputEntityQuality empty failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should indicate no issues found
	if !strings.Contains(output, "No entity quality issues") && !strings.Contains(output, "no issues") {
		t.Error("output should indicate no issues found")
	}
}

// =============================================================================
// Extraction Quality Tests
// =============================================================================

// TestQualityExtractionsCommand verifies the extractions subcommand structure.
func TestQualityExtractionsCommand(t *testing.T) {
	cfg := mockConfig()
	deps := createQualityTestDeps(cfg)
	cmd := NewQualityCommand(deps)

	extractionsCmd, _, err := cmd.Find([]string{"extractions"})
	if err != nil {
		t.Fatalf("failed to find extractions command: %v", err)
	}

	if extractionsCmd.Use != "extractions" {
		t.Errorf("expected Use to be 'extractions', got %q", extractionsCmd.Use)
	}

	// Verify limit flag exists
	limitFlag := extractionsCmd.Flags().Lookup("limit")
	if limitFlag == nil {
		t.Error("extractions command should have --limit flag")
	}
}

// TestQualityExtractionsOutput_Text verifies content items with extraction scores.
func TestQualityExtractionsOutput_Text(t *testing.T) {
	extractions := []ExtractionQualityItem{
		{
			ContentItemID:    101,
			ContentType:      "email",
			Subject:          "Project Update",
			ExtractionScore:  0.45,
			Issues: []QualityIssue{
				{
					Severity:         "HIGH",
					Category:         "extraction",
					Description:      "Low extraction quality",
					SuggestedCommand: "penf content show 101",
				},
			},
		},
		{
			ContentItemID:    102,
			ContentType:      "meeting",
			Subject:          "Weekly Sync",
			ExtractionScore:  0.72,
			Issues: []QualityIssue{
				{
					Severity:         "MEDIUM",
					Category:         "extraction",
					Description:      "Incomplete entity extraction",
					SuggestedCommand: "penf content show 102",
				},
			},
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputExtractionQuality(config.OutputFormatText, extractions)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputExtractionQuality text failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify content subjects are displayed
	if !strings.Contains(output, "Project Update") {
		t.Error("output should contain subject 'Project Update'")
	}
	if !strings.Contains(output, "Weekly Sync") {
		t.Error("output should contain subject 'Weekly Sync'")
	}

	// Verify content types are displayed
	if !strings.Contains(output, "email") {
		t.Error("output should contain content type 'email'")
	}
	if !strings.Contains(output, "meeting") {
		t.Error("output should contain content type 'meeting'")
	}

	// Verify suggested commands
	if !strings.Contains(output, "penf content show") {
		t.Error("output should contain suggested command")
	}
}

// TestQualityExtractionsOutput_RankedByScore verifies items are ranked by extraction score.
func TestQualityExtractionsOutput_RankedByScore(t *testing.T) {
	extractions := []ExtractionQualityItem{
		{ContentItemID: 1, Subject: "Good Score", ExtractionScore: 0.85},
		{ContentItemID: 2, Subject: "Bad Score", ExtractionScore: 0.35},
		{ContentItemID: 3, Subject: "Medium Score", ExtractionScore: 0.60},
	}

	// After ranking, lowest scores should come first (most problematic)
	ranked := rankExtractionsByScore(extractions)

	if len(ranked) != 3 {
		t.Fatalf("expected 3 items, got %d", len(ranked))
	}

	// Verify lowest score is first
	if ranked[0].ExtractionScore >= ranked[1].ExtractionScore {
		t.Errorf("items not ranked correctly: first score %.2f >= second score %.2f",
			ranked[0].ExtractionScore, ranked[1].ExtractionScore)
	}

	if ranked[1].ExtractionScore >= ranked[2].ExtractionScore {
		t.Errorf("items not ranked correctly: second score %.2f >= third score %.2f",
			ranked[1].ExtractionScore, ranked[2].ExtractionScore)
	}
}

// TestQualityExtractionsOutput_JSON verifies JSON output format.
func TestQualityExtractionsOutput_JSON(t *testing.T) {
	extractions := []ExtractionQualityItem{
		{
			ContentItemID:   101,
			ContentType:     "email",
			Subject:         "Test Subject",
			ExtractionScore: 0.65,
			Issues: []QualityIssue{
				{Severity: "MEDIUM", Description: "Test issue"},
			},
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputExtractionQuality(config.OutputFormatJSON, extractions)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputExtractionQuality JSON failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid JSON
	var decoded []ExtractionQualityItem
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(decoded) != 1 {
		t.Fatalf("expected 1 item, got %d", len(decoded))
	}

	if decoded[0].ContentItemID != extractions[0].ContentItemID {
		t.Errorf("ContentItemID = %d, want %d", decoded[0].ContentItemID, extractions[0].ContentItemID)
	}
}

// =============================================================================
// Test Helper Functions
// =============================================================================

// createQualityTestDeps creates test dependencies for quality commands.
func createQualityTestDeps(cfg *config.CLIConfig) *QualityCommandDeps {
	return &QualityCommandDeps{
		Config:       cfg,
		OutputFormat: cfg.OutputFormat,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
	}
}
