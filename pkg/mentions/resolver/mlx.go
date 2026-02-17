package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MLXProvider implements LLMProvider for the MLX sidecar with Ollama-compatible API.
type MLXProvider struct {
	config     LLMConfig
	httpClient *http.Client
	name       string
}

// NewMLXProvider creates a new MLX provider.
func NewMLXProvider(config LLMConfig) *MLXProvider {
	return &MLXProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		name: fmt.Sprintf("mlx-%s", config.Model),
	}
}

// Name returns the provider identifier.
func (p *MLXProvider) Name() string {
	return p.name
}

// ollamaRequest is the request format for Ollama-compatible API.
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Format string `json:"format,omitempty"` // "json" for JSON mode
	Stream bool   `json:"stream"`
}

// ollamaResponse is the response format from Ollama-compatible API.
type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Complete sends a completion request and returns the raw response.
func (p *MLXProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()

	// Build the prompt with optional system prompt
	fullPrompt := req.Prompt
	if req.SystemPrompt != "" {
		fullPrompt = fmt.Sprintf("%s\n\n%s", req.SystemPrompt, req.Prompt)
	}

	// Create Ollama request
	ollamaReq := ollamaRequest{
		Model:  p.config.Model,
		Prompt: fullPrompt,
		Stream: false,
	}
	if req.JSONMode {
		ollamaReq.Format = "json"
	}

	// Marshal request
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, &LLMError{Code: ErrParseFailure, Message: fmt.Sprintf("marshal request: %v", err)}
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/api/generate", p.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, &LLMError{Code: ErrUnavailable, Message: fmt.Sprintf("create request: %v", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, &LLMError{Code: ErrTimeout, Message: "request timeout"}
		}
		return nil, &LLMError{Code: ErrUnavailable, Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &LLMError{Code: ErrParseFailure, Message: fmt.Sprintf("read response: %v", err)}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &LLMError{
			Code:    ErrUnavailable,
			Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
		}
	}

	// Parse response
	var ollamaResp ollamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, &LLMError{Code: ErrParseFailure, Message: fmt.Sprintf("parse response: %v", err)}
	}

	latency := time.Since(start)
	return &CompletionResponse{
		Content:   ollamaResp.Response,
		LatencyMs: int(latency.Milliseconds()),
		Model:     p.config.Model,
		TokensUsed: TokenUsage{
			// Ollama doesn't return token counts, estimate roughly
			Prompt:     len(fullPrompt) / 4,
			Completion: len(ollamaResp.Response) / 4,
			Total:      (len(fullPrompt) + len(ollamaResp.Response)) / 4,
		},
	}, nil
}

// CompleteStructured sends a request expecting JSON output and parses it.
func (p *MLXProvider) CompleteStructured(ctx context.Context, req CompletionRequest, target interface{}) error {
	req.JSONMode = true

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

		// Try to parse the JSON response
		if err := json.Unmarshal([]byte(resp.Content), target); err != nil {
			lastErr = &LLMError{
				Code:    ErrParseFailure,
				Message: fmt.Sprintf("parse JSON: %v", err),
				Details: resp.Content,
			}
			// Retry with a hint about the format
			if attempt < p.config.MaxRetries {
				req.Prompt = fmt.Sprintf("%s\n\nIMPORTANT: Respond with valid JSON only.", req.Prompt)
			}
			continue
		}

		return nil
	}

	return lastErr
}

// IsAvailable checks if the provider is currently available.
func (p *MLXProvider) IsAvailable(ctx context.Context) bool {
	url := fmt.Sprintf("%s/api/tags", p.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Close releases provider resources.
func (p *MLXProvider) Close() error {
	p.httpClient.CloseIdleConnections()
	return nil
}
