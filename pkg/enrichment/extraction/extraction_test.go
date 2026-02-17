package extraction

import (
	"testing"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
)

func TestGetContextTier(t *testing.T) {
	tests := []struct {
		profile enrichment.ProcessingProfile
		want    ContextTier
	}{
		{enrichment.ProfileFullAI, ContextTierFull},
		{enrichment.ProfileFullAIChunked, ContextTierMinimal},
		{enrichment.ProfileMetadataOnly, ContextTierNone},
		{enrichment.ProfileStateTracking, ContextTierNone},
		{enrichment.ProfileStructureOnly, ContextTierNone},
		{enrichment.ProfileOCRIfText, ContextTierStandard},
	}

	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			got := GetContextTier(tt.profile)
			if got != tt.want {
				t.Errorf("GetContextTier(%s) = %s, want %s", tt.profile, got, tt.want)
			}
		})
	}
}

func TestDefaultContextBuilderConfig(t *testing.T) {
	config := DefaultContextBuilderConfig()

	if config.MaxParticipants <= 0 {
		t.Error("MaxParticipants should be positive")
	}
	if config.MaxPriorMessages <= 0 {
		t.Error("MaxPriorMessages should be positive")
	}
	if config.FullTokenBudget <= 0 {
		t.Error("FullTokenBudget should be positive")
	}
	if config.FullTokenBudget <= config.StandardTokenBudget {
		t.Error("FullTokenBudget should be > StandardTokenBudget")
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		text   string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"hello world", 5, "hello..."},
		{"hello beautiful world", 15, "hello..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := truncateText(tt.text, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateText(%q, %d) = %q, want %q", tt.text, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFormatContextForPrompt(t *testing.T) {
	// Test with nil context
	result := FormatContextForPrompt(nil)
	if result != "" {
		t.Error("FormatContextForPrompt(nil) should return empty string")
	}

	// Test with participants
	ctx := &ExtractionContext{
		Participants: []ParticipantContext{
			{Name: "John Doe", Email: "john@example.com", Title: "Engineer"},
			{Name: "Jane Smith", Email: "jane@example.com", Department: "Sales"},
		},
	}

	result = FormatContextForPrompt(ctx)
	if result == "" {
		t.Error("FormatContextForPrompt should include participants")
	}
	if !contains(result, "John Doe") {
		t.Error("FormatContextForPrompt should contain participant name")
	}
	if !contains(result, "john@example.com") {
		t.Error("FormatContextForPrompt should contain participant email")
	}
}

func TestFormatContextForPrompt_WithProject(t *testing.T) {
	ctx := &ExtractionContext{
		Project: &ProjectContext{
			Name:        "Test Project",
			Description: "A test project",
		},
	}

	result := FormatContextForPrompt(ctx)
	if !contains(result, "PROJECT:") {
		t.Error("FormatContextForPrompt should include PROJECT header")
	}
	if !contains(result, "Test Project") {
		t.Error("FormatContextForPrompt should contain project name")
	}
}

func TestDefaultSystemTemplate(t *testing.T) {
	tmpl := DefaultSystemTemplate()

	if tmpl.Name != "system-default" {
		t.Errorf("Name = %s, want system-default", tmpl.Name)
	}
	if tmpl.Type != TemplateTypeSystemDefault {
		t.Errorf("Type = %s, want system_default", tmpl.Type)
	}
	if !tmpl.Active {
		t.Error("Template should be active")
	}
	if tmpl.TemplateText == "" {
		t.Error("TemplateText should not be empty")
	}
	if tmpl.ExtractionSchema == nil {
		t.Error("ExtractionSchema should not be nil")
	}
}

func TestRenderPrompt(t *testing.T) {
	tmpl := DefaultSystemTemplate()

	data := PromptData{
		Context: "PARTICIPANTS:\n- John Doe <john@example.com>",
		Content: "This is the email body",
	}

	result, err := RenderPrompt(tmpl, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}

	if !contains(result, "John Doe") {
		t.Error("RenderPrompt should include context")
	}
	if !contains(result, "This is the email body") {
		t.Error("RenderPrompt should include content")
	}
	if !contains(result, "INSTRUCTIONS:") {
		t.Error("RenderPrompt should include instructions from template")
	}
}

func TestParseExtractionOutput(t *testing.T) {
	rawJSON := `{
		"risks": [{"description": "Test risk", "severity": "high"}],
		"actions": [{"description": "Test action", "assignee": "John"}],
		"issues": [],
		"decisions": [],
		"commitments": [],
		"questions": [],
		"sentiment": {"overall": "positive", "urgency": "medium"}
	}`

	output, err := ParseExtractionOutput(rawJSON)
	if err != nil {
		t.Fatalf("ParseExtractionOutput() error = %v", err)
	}

	if len(output.Risks) != 1 {
		t.Errorf("Risks count = %d, want 1", len(output.Risks))
	}
	if output.Risks[0].Description != "Test risk" {
		t.Errorf("Risk description = %s, want 'Test risk'", output.Risks[0].Description)
	}
	if len(output.Actions) != 1 {
		t.Errorf("Actions count = %d, want 1", len(output.Actions))
	}
	if output.Sentiment.Overall != "positive" {
		t.Errorf("Sentiment overall = %s, want 'positive'", output.Sentiment.Overall)
	}
}

func TestParseExtractionOutput_Invalid(t *testing.T) {
	_, err := ParseExtractionOutput("invalid json")
	if err == nil {
		t.Error("ParseExtractionOutput should return error for invalid JSON")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
