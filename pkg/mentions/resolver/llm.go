package resolver

import (
	"context"
)

// LLMProvider defines the interface for LLM resolution providers.
type LLMProvider interface {
	// Name returns the provider identifier (e.g., "mlx-mistral-7b", "claude-sonnet").
	Name() string

	// Complete sends a completion request and returns the raw response.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// CompleteStructured sends a request expecting JSON output and parses it.
	CompleteStructured(ctx context.Context, req CompletionRequest, target interface{}) error

	// IsAvailable checks if the provider is currently available.
	IsAvailable(ctx context.Context) bool

	// Close releases provider resources.
	Close() error
}

// CompletionRequest represents a request to the LLM.
type CompletionRequest struct {
	// Prompt is the full prompt text to send to the LLM.
	Prompt string `json:"prompt"`

	// SystemPrompt is an optional system-level instruction.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// JSONMode enables structured JSON output.
	JSONMode bool `json:"json_mode"`

	// MaxTokens limits response length (0 = provider default).
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness (0.0-1.0, 0 = provider default).
	Temperature float32 `json:"temperature,omitempty"`

	// Metadata for tracing/logging.
	TraceID  string `json:"trace_id,omitempty"`
	StageNum int    `json:"stage_num,omitempty"`
}

// CompletionResponse represents a response from the LLM.
type CompletionResponse struct {
	// Content is the raw text response from the LLM.
	Content string `json:"content"`

	// TokensUsed tracks token consumption.
	TokensUsed TokenUsage `json:"tokens_used"`

	// LatencyMs is the response time in milliseconds.
	LatencyMs int `json:"latency_ms"`

	// Model is the actual model used (may differ from requested).
	Model string `json:"model"`

	// FinishReason indicates why the model stopped generating.
	// "stop" = natural end, "length" = hit max_tokens limit.
	FinishReason string `json:"finish_reason,omitempty"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	Prompt     int `json:"prompt"`
	Completion int `json:"completion"`
	Total      int `json:"total"`
}

// ProviderRegistry manages registered LLM providers.
type ProviderRegistry struct {
	providers map[string]LLMProvider
	primary   string
	fallback  string
}

// NewProviderRegistry creates a new provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]LLMProvider),
	}
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(name string, provider LLMProvider) {
	r.providers[name] = provider
}

// SetPrimary sets the primary provider.
func (r *ProviderRegistry) SetPrimary(name string) {
	r.primary = name
}

// SetFallback sets the fallback provider.
func (r *ProviderRegistry) SetFallback(name string) {
	r.fallback = name
}

// Get returns a provider by name.
func (r *ProviderRegistry) Get(name string) (LLMProvider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// Primary returns the primary provider.
func (r *ProviderRegistry) Primary() (LLMProvider, bool) {
	return r.Get(r.primary)
}

// Fallback returns the fallback provider.
func (r *ProviderRegistry) Fallback() (LLMProvider, bool) {
	return r.Get(r.fallback)
}

// Close closes all registered providers.
func (r *ProviderRegistry) Close() error {
	var lastErr error
	for _, p := range r.providers {
		if err := p.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
