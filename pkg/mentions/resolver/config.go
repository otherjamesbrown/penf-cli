package resolver

import (
	"time"
)

// HeartbeatFunc is a callback invoked between resolver stages to signal liveness.
// The string argument describes the current stage for diagnostics.
type HeartbeatFunc func(stage string)

// Config holds configuration for the resolver.
type Config struct {
	// LLM provider configuration
	LLM LLMConfig `json:"llm"`

	// Resolution thresholds
	Thresholds ThresholdConfig `json:"thresholds"`

	// Batch processing
	MaxMentionsPerBatch int `json:"max_mentions_per_batch"`

	// Tracing
	TraceLevel TraceLevel `json:"trace_level"`

	// Heartbeat is called between pipeline stages to signal liveness.
	// When running inside a Temporal activity, set this to activity.RecordHeartbeat.
	Heartbeat HeartbeatFunc `json:"-"`
}

// LLMConfig configures the LLM provider.
type LLMConfig struct {
	// Provider selection
	Provider string `json:"provider"` // "mlx", "claude", "openai"
	Model    string `json:"model"`    // "mistral-7b-instruct-v0.2", "claude-3-sonnet"

	// Connection
	BaseURL string        `json:"base_url"` // "http://localhost:11434"
	Timeout time.Duration `json:"timeout"`  // 30s default

	// Performance
	MaxRetries int `json:"max_retries"` // 2 default

	// Escalation (future)
	EscalateOnLowConfidence bool    `json:"escalate_on_low_confidence"`
	EscalationThreshold     float32 `json:"escalation_threshold"` // 0.7
	EscalationProvider      string  `json:"escalation_provider"`  // "claude"
}

// ThresholdConfig configures resolution thresholds.
type ThresholdConfig struct {
	// Auto-resolution threshold (confidence >= this value)
	AutoResolve float32 `json:"auto_resolve"` // 0.8

	// Verification threshold (confidence >= this skips verification)
	Verification float32 `json:"verification"` // 0.9

	// Suggestion threshold (confidence >= this for candidate suggestions)
	Suggest float32 `json:"suggest"` // 0.7
}

// DefaultConfig returns the default resolver configuration.
func DefaultConfig() Config {
	return Config{
		LLM: LLMConfig{
			Provider:                "mlx",
			Model:                   "mistral-7b-instruct-v0.2",
			BaseURL:                 "http://localhost:11434",
			Timeout:                 120 * time.Second,
			MaxRetries:              2,
			EscalateOnLowConfidence: false,
			EscalationThreshold:     0.7,
			EscalationProvider:      "claude",
		},
		Thresholds: ThresholdConfig{
			AutoResolve:  0.8,
			Verification: 0.9,
			Suggest:      0.7,
		},
		MaxMentionsPerBatch: 50,
		TraceLevel:          TraceLevelStandard,
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.LLM.Provider == "" {
		c.LLM.Provider = "mlx"
	}
	if c.LLM.Model == "" {
		c.LLM.Model = "mistral-7b-instruct-v0.2"
	}
	if c.LLM.BaseURL == "" {
		c.LLM.BaseURL = "http://localhost:11434"
	}
	if c.LLM.Timeout == 0 {
		c.LLM.Timeout = 120 * time.Second
	}
	if c.LLM.MaxRetries == 0 {
		c.LLM.MaxRetries = 2
	}
	if c.Thresholds.AutoResolve == 0 {
		c.Thresholds.AutoResolve = 0.8
	}
	if c.Thresholds.Verification == 0 {
		c.Thresholds.Verification = 0.9
	}
	if c.Thresholds.Suggest == 0 {
		c.Thresholds.Suggest = 0.7
	}
	if c.MaxMentionsPerBatch == 0 {
		c.MaxMentionsPerBatch = 50
	}
	if c.TraceLevel == "" {
		c.TraceLevel = TraceLevelStandard
	}
	return nil
}
