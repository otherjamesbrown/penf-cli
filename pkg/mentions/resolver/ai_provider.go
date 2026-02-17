// Package resolver provides LLM-driven mention resolution using a multi-stage pipeline.
package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	aiv1 "github.com/otherjamesbrown/penf-cli/api/proto/ai/v1"
)

// AIClient defines the interface for the AI service client.
// This interface is satisfied by pkg/ai.Client.
type AIClient interface {
	// GenerateSummary generates a summary or completion for the given content.
	GenerateSummary(ctx context.Context, req *aiv1.SummaryRequest) (*aiv1.SummaryResponse, error)

	// HealthCheck verifies the AI service is healthy.
	HealthCheck(ctx context.Context) error

	// Close closes the client connection.
	Close() error
}

// AIProvider implements LLMProvider using the centralized AI service via AIClient.
// This is the preferred provider for production use as it routes through the
// AI Coordinator service which handles model selection, retries, and observability.
type AIProvider struct {
	aiClient AIClient
	config   LLMConfig
	name     string
}

// NewAIProvider creates a new AI service-based provider.
// The aiClient should be a connected instance of pkg/ai.Client.
func NewAIProvider(aiClient AIClient, cfg LLMConfig) *AIProvider {
	name := "ai-service"
	if cfg.Model != "" {
		name = fmt.Sprintf("ai-%s", cfg.Model)
	}
	return &AIProvider{
		aiClient: aiClient,
		config:   cfg,
		name:     name,
	}
}

// Name returns the provider identifier.
func (p *AIProvider) Name() string {
	return p.name
}

// Complete sends a completion request and returns the raw response.
func (p *AIProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()

	// Build the content for the AI service
	// Combine system prompt and user prompt if both exist
	var content string
	if req.SystemPrompt != "" {
		content = fmt.Sprintf("System: %s\n\nUser: %s", req.SystemPrompt, req.Prompt)
	} else {
		content = req.Prompt
	}

	// Create summary request with verbose style (raw completion)
	// The AI service uses GenerateSummary as a general-purpose text generation endpoint
	summaryReq := &aiv1.SummaryRequest{
		Content: content,
		Style:   aiv1.SummaryStyle_SUMMARY_STYLE_DETAILED, // Use detailed for full context preservation
	}

	// Set max length if specified
	if req.MaxTokens > 0 {
		maxLength := int32(req.MaxTokens)
		summaryReq.MaxLength = &maxLength
	}

	// Set model if specified in config
	if p.config.Model != "" {
		summaryReq.Model = &p.config.Model
	}

	// Pass through JSON mode
	if req.JSONMode {
		jm := true
		summaryReq.JsonMode = &jm
	}

	// Execute request
	resp, err := p.aiClient.GenerateSummary(ctx, summaryReq)
	if err != nil {
		// Map errors to LLMError types
		if ctx.Err() == context.DeadlineExceeded {
			return nil, &LLMError{Code: ErrTimeout, Message: "request timeout"}
		}
		if ctx.Err() == context.Canceled {
			return nil, &LLMError{Code: ErrUnavailable, Message: "request canceled"}
		}
		return nil, &LLMError{Code: ErrUnavailable, Message: fmt.Sprintf("AI service error: %v", err)}
	}

	latency := time.Since(start)

	// Build token usage from response if available
	var tokenUsage TokenUsage
	if resp.InputTokens != nil {
		tokenUsage.Prompt = int(*resp.InputTokens)
	}
	if resp.OutputTokens != nil {
		tokenUsage.Completion = int(*resp.OutputTokens)
	}
	tokenUsage.Total = tokenUsage.Prompt + tokenUsage.Completion

	return &CompletionResponse{
		Content:      resp.GetSummary(),
		FinishReason: resp.GetFinishReason(),
		LatencyMs:    int(latency.Milliseconds()),
		Model:        resp.GetModelUsed(),
		TokensUsed:   tokenUsage,
	}, nil
}

// CompleteStructured sends a request expecting JSON output and parses it.
func (p *AIProvider) CompleteStructured(ctx context.Context, req CompletionRequest, target interface{}) error {
	// Enhance prompt to request JSON output if not already specified
	if !strings.Contains(req.Prompt, "JSON") && !strings.Contains(req.Prompt, "json") {
		req.Prompt = req.Prompt + "\n\nRespond with valid JSON only."
	}

	var lastErr error
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		resp, err := p.Complete(ctx, req)
		if err != nil {
			lastErr = err
			// Don't retry on context cancellation
			if ctx.Err() != nil {
				return err
			}
			continue
		}

		// Clean up the response - sometimes LLMs wrap JSON in markdown
		content := strings.TrimSpace(resp.Content)
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)

		// Try to parse the JSON response
		if err := json.Unmarshal([]byte(content), target); err != nil {
			lastErr = &LLMError{
				Code:    ErrParseFailure,
				Message: fmt.Sprintf("parse JSON: %v", err),
				Details: resp.Content,
			}
			// Retry with a stronger hint about the format
			if attempt < p.config.MaxRetries {
				req.Prompt = fmt.Sprintf("%s\n\nIMPORTANT: Respond with valid JSON only. No markdown, no explanations.", req.Prompt)
			}
			continue
		}

		return nil
	}

	return lastErr
}

// IsAvailable checks if the provider is currently available.
func (p *AIProvider) IsAvailable(ctx context.Context) bool {
	err := p.aiClient.HealthCheck(ctx)
	return err == nil
}

// Close releases provider resources.
func (p *AIProvider) Close() error {
	return p.aiClient.Close()
}
