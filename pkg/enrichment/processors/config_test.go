package processors

import (
	"testing"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Check classification stage
	if cfg.Stages.Classification.Processor != "ContentClassifier" {
		t.Errorf("Classification.Processor = %q, want %q", cfg.Stages.Classification.Processor, "ContentClassifier")
	}

	// Check common enrichment has processors
	if len(cfg.Stages.CommonEnrichment.Processors) == 0 {
		t.Error("CommonEnrichment.Processors is empty")
	}

	// Check type specific processors exist
	if len(cfg.Stages.TypeSpecific) == 0 {
		t.Error("TypeSpecific config is empty")
	}

	// Check AI routing profiles
	if len(cfg.Stages.AIRouting) == 0 {
		t.Error("AIRouting config is empty")
	}
}

func TestConfig_GetTypeSpecificProcessor(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		subtype  enrichment.ContentSubtype
		wantProc string
	}{
		{enrichment.SubtypeNotificationJira, "JiraExtractor"},
		{enrichment.SubtypeNotificationGoogle, "GoogleNotificationExtractor"},
		// Calendar subtypes use "calendar/invite" format in config
		{enrichment.ContentSubtype("calendar/invite"), "CalendarExtractor"},
		{enrichment.ContentSubtype("email/forward"), "ForwardExtractor"},
		{enrichment.ContentSubtype("email/thread"), "ThreadContextBuilder"},
		{enrichment.ContentSubtype("email/standalone"), ""},
		{enrichment.ContentSubtype("unknown"), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.subtype), func(t *testing.T) {
			got := cfg.GetTypeSpecificProcessor(tt.subtype)
			if got != tt.wantProc {
				t.Errorf("GetTypeSpecificProcessor(%q) = %q, want %q", tt.subtype, got, tt.wantProc)
			}
		})
	}
}

func TestConfig_ShouldSkipAI(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		profile  enrichment.ProcessingProfile
		wantSkip bool
	}{
		{enrichment.ProfileFullAI, false},
		{enrichment.ProfileFullAIChunked, false},
		{enrichment.ProfileMetadataOnly, true},
		{enrichment.ProfileStateTracking, true},
		{enrichment.ProfileStructureOnly, true},
		{enrichment.ProfileOCRIfText, false},
		{enrichment.ProcessingProfile("unknown"), false}, // Default to processing
	}

	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			got := cfg.ShouldSkipAI(tt.profile)
			if got != tt.wantSkip {
				t.Errorf("ShouldSkipAI(%q) = %v, want %v", tt.profile, got, tt.wantSkip)
			}
		})
	}
}

func TestConfig_GetAISkipReason(t *testing.T) {
	cfg := DefaultConfig()

	// metadata_only should have a reason
	reason := cfg.GetAISkipReason(enrichment.ProfileMetadataOnly)
	if reason == "" {
		t.Error("GetAISkipReason(metadata_only) returned empty string, want reason")
	}

	// full_ai should not have a skip reason
	reason = cfg.GetAISkipReason(enrichment.ProfileFullAI)
	if reason != "" {
		t.Errorf("GetAISkipReason(full_ai) = %q, want empty string", reason)
	}
}

func TestParseConfig(t *testing.T) {
	yamlConfig := `
stages:
  classification:
    processor: TestClassifier
    input: source
    output: type, subtype, profile
  common_enrichment:
    processors:
      - Processor1
      - Processor2
  type_specific:
    "notification/test":
      processor: TestExtractor
      outputs: [test_output]
  ai_routing:
    "test_profile":
      skip_ai: true
      reason: "Test reason"
`

	cfg, err := ParseConfig([]byte(yamlConfig))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if cfg.Stages.Classification.Processor != "TestClassifier" {
		t.Errorf("Classification.Processor = %q, want %q", cfg.Stages.Classification.Processor, "TestClassifier")
	}

	if len(cfg.Stages.CommonEnrichment.Processors) != 2 {
		t.Errorf("CommonEnrichment.Processors has %d items, want 2", len(cfg.Stages.CommonEnrichment.Processors))
	}

	if typeCfg, ok := cfg.Stages.TypeSpecific["notification/test"]; !ok {
		t.Error("TypeSpecific[notification/test] not found")
	} else if typeCfg.Processor != "TestExtractor" {
		t.Errorf("TypeSpecific[notification/test].Processor = %q, want %q", typeCfg.Processor, "TestExtractor")
	}

	if aiCfg, ok := cfg.Stages.AIRouting["test_profile"]; !ok {
		t.Error("AIRouting[test_profile] not found")
	} else {
		if !aiCfg.SkipAI {
			t.Error("AIRouting[test_profile].SkipAI = false, want true")
		}
		if aiCfg.Reason != "Test reason" {
			t.Errorf("AIRouting[test_profile].Reason = %q, want %q", aiCfg.Reason, "Test reason")
		}
	}
}

func TestParseConfig_Invalid(t *testing.T) {
	_, err := ParseConfig([]byte("invalid: yaml: ["))
	if err == nil {
		t.Error("ParseConfig() should return error for invalid YAML")
	}
}
