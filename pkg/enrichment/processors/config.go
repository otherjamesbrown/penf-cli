package processors

import (
	"fmt"
	"os"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
	"gopkg.in/yaml.v3"
)

// Config represents the processor pipeline configuration.
type Config struct {
	Stages StagesConfig `yaml:"stages"`
}

// StagesConfig defines configuration for each pipeline stage.
type StagesConfig struct {
	Classification    ClassificationConfig    `yaml:"classification"`
	CommonEnrichment  CommonEnrichmentConfig  `yaml:"common_enrichment"`
	TypeSpecific      TypeSpecificConfig      `yaml:"type_specific"`
	AIRouting         AIRoutingConfig         `yaml:"ai_routing"`
}

// ClassificationConfig defines the classification stage.
type ClassificationConfig struct {
	Processor string `yaml:"processor"`
	Input     string `yaml:"input"`
	Output    string `yaml:"output"`
}

// CommonEnrichmentConfig defines common enrichment processors.
type CommonEnrichmentConfig struct {
	Processors []string `yaml:"processors"`
}

// TypeSpecificConfig maps subtypes to their processor configurations.
type TypeSpecificConfig map[string]TypeProcessorConfig

// TypeProcessorConfig defines a type-specific processor.
type TypeProcessorConfig struct {
	Processor string   `yaml:"processor"`
	Outputs   []string `yaml:"outputs,omitempty"`
}

// AIRoutingConfig maps processing profiles to their AI configuration.
type AIRoutingConfig map[string]AIProfileConfig

// AIProfileConfig defines AI processing for a profile.
type AIProfileConfig struct {
	SkipAI       bool     `yaml:"skip_ai"`
	Reason       string   `yaml:"reason,omitempty"`
	Preprocessor string   `yaml:"preprocessor,omitempty"`
	Processors   []string `yaml:"processors,omitempty"`
}

// LoadConfig loads processor configuration from a YAML file.
func LoadConfig(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return ParseConfig(data)
}

// ParseConfig parses processor configuration from YAML bytes.
func ParseConfig(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// DefaultConfig returns the default processor configuration.
func DefaultConfig() *Config {
	return &Config{
		Stages: StagesConfig{
			Classification: ClassificationConfig{
				Processor: "ContentClassifier",
				Input:     "source",
				Output:    "content_type, content_subtype, processing_profile",
			},
			CommonEnrichment: CommonEnrichmentConfig{
				Processors: []string{
					"ParticipantExtractor",
					"EntityResolver",
					"InternalExternalClassifier",
					"LinkExtractor",
					"LinkCategorizer",
					"ThreadGrouper",
					"ProjectMatcher",
					"AttachmentExtractor",
				},
			},
			TypeSpecific: TypeSpecificConfig{
				"notification/jira": {
					Processor: "JiraExtractor",
					Outputs:   []string{"jira_tickets", "jira_ticket_changes"},
				},
				"notification/google": {
					Processor: "GoogleNotificationExtractor",
					Outputs:   []string{"link_enrichment"},
				},
				"notification/slack": {
					Processor: "SlackExtractor",
					Outputs:   []string{"slack_references"},
				},
				"calendar/invite": {
					Processor: "CalendarExtractor",
					Outputs:   []string{"meetings", "meeting_attendees", "meeting_events"},
				},
				"calendar/cancellation": {
					Processor: "CalendarExtractor",
					Outputs:   []string{"meetings", "meeting_attendees", "meeting_events"},
				},
				"calendar/update": {
					Processor: "CalendarExtractor",
					Outputs:   []string{"meetings", "meeting_attendees", "meeting_events"},
				},
				"calendar/response": {
					Processor: "CalendarExtractor",
					Outputs:   []string{"meetings", "meeting_attendees", "meeting_events"},
				},
				"email/forward": {
					Processor: "ForwardExtractor",
					Outputs:   []string{"forwarded_from_person_id"},
				},
				"email/thread": {
					Processor: "ThreadContextBuilder",
					Outputs:   []string{"thread_context"},
				},
				"email/standalone": {
					Processor: "", // No type-specific processing
				},
			},
			AIRouting: AIRoutingConfig{
				"metadata_only": {
					SkipAI: true,
					Reason: "Structured extraction only",
				},
				"state_tracking": {
					SkipAI: true,
					Reason: "State machine updates only",
				},
				"full_ai": {
					SkipAI:     false,
					Processors: []string{"ContextBuilder", "TemplateResolver", "LLMExtractor", "Embedder", "Summarizer"},
				},
				"full_ai_chunked": {
					SkipAI:       false,
					Preprocessor: "Chunker",
					Processors:   []string{"ContextBuilder", "TemplateResolver", "LLMExtractor", "Embedder", "Summarizer"},
				},
				"structure_only": {
					SkipAI:     true,
					Processors: []string{"SchemaExtractor"},
					Reason:     "Spreadsheet structure only",
				},
				"ocr_if_text": {
					SkipAI:       false,
					Preprocessor: "OCRProcessor",
					Processors:   []string{"ContextBuilder", "TemplateResolver", "LLMExtractor", "Embedder", "Summarizer"},
				},
			},
		},
	}
}

// GetTypeSpecificProcessor returns the processor name for a content subtype.
func (c *Config) GetTypeSpecificProcessor(subtype enrichment.ContentSubtype) string {
	if cfg, ok := c.Stages.TypeSpecific[string(subtype)]; ok {
		return cfg.Processor
	}
	return ""
}

// GetAIConfig returns the AI configuration for a processing profile.
func (c *Config) GetAIConfig(profile enrichment.ProcessingProfile) *AIProfileConfig {
	if cfg, ok := c.Stages.AIRouting[string(profile)]; ok {
		return &cfg
	}
	return nil
}

// ShouldSkipAI returns true if AI processing should be skipped for the profile.
func (c *Config) ShouldSkipAI(profile enrichment.ProcessingProfile) bool {
	if cfg := c.GetAIConfig(profile); cfg != nil {
		return cfg.SkipAI
	}
	// Default to full AI processing for unknown profiles
	return false
}

// GetAISkipReason returns the reason for skipping AI processing.
func (c *Config) GetAISkipReason(profile enrichment.ProcessingProfile) string {
	if cfg := c.GetAIConfig(profile); cfg != nil && cfg.SkipAI {
		return cfg.Reason
	}
	return ""
}
