package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// VLLMProvider implements LLMProvider for vLLM-MLX servers using the OpenAI-compatible API.
//
// Deprecated: VLLMProvider makes direct HTTP calls to the LLM service.
// Use NewAIProvider with an AIClient instead, which routes through the
// centralized AI Coordinator service for better observability, model
// routing, and retry handling.
type VLLMProvider struct {
	config     LLMConfig
	httpClient *http.Client
	name       string
}

// NewVLLMProvider creates a new vLLM provider.
//
// Deprecated: Use NewAIProvider instead. NewVLLMProvider makes direct HTTP
// calls to the LLM service. NewAIProvider routes through the AI Coordinator
// service for better observability and retry handling.
func NewVLLMProvider(config LLMConfig) *VLLMProvider {
	return &VLLMProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		name: fmt.Sprintf("vllm-%s", config.Model),
	}
}

// Name returns the provider identifier.
func (p *VLLMProvider) Name() string {
	return p.name
}

// chatMessage represents a message in the chat conversation.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the OpenAI-compatible chat completion request.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float32       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// chatChoice represents a completion choice.
type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// chatUsage represents token usage.
type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// chatResponse is the OpenAI-compatible chat completion response.
type chatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

// Complete sends a completion request and returns the raw response.
func (p *VLLMProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()

	// Build messages for chat completion
	var messages []chatMessage

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		messages = append(messages, chatMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Add user prompt
	messages = append(messages, chatMessage{
		Role:    "user",
		Content: req.Prompt,
	})

	// Build request
	chatReq := chatRequest{
		Model:    p.config.Model,
		Messages: messages,
	}

	if req.Temperature > 0 {
		chatReq.Temperature = req.Temperature
	} else {
		chatReq.Temperature = 0.1 // Low temperature for structured extraction
	}

	if req.MaxTokens > 0 {
		chatReq.MaxTokens = req.MaxTokens
	} else {
		chatReq.MaxTokens = 4096
	}

	// Marshal request
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, &LLMError{Code: ErrParseFailure, Message: fmt.Sprintf("marshal request: %v", err)}
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v1/chat/completions", p.config.BaseURL)
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
	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, &LLMError{Code: ErrParseFailure, Message: fmt.Sprintf("parse response: %v", err)}
	}

	if len(chatResp.Choices) == 0 {
		return nil, &LLMError{Code: ErrParseFailure, Message: "no choices in response"}
	}

	latency := time.Since(start)
	return &CompletionResponse{
		Content:      chatResp.Choices[0].Message.Content,
		FinishReason: chatResp.Choices[0].FinishReason,
		LatencyMs:    int(latency.Milliseconds()),
		Model:        chatResp.Model,
		TokensUsed: TokenUsage{
			Prompt:     chatResp.Usage.PromptTokens,
			Completion: chatResp.Usage.CompletionTokens,
			Total:      chatResp.Usage.TotalTokens,
		},
	}, nil
}

// CompleteStructured sends a request expecting JSON output and parses it.
func (p *VLLMProvider) CompleteStructured(ctx context.Context, req CompletionRequest, target interface{}) error {
	// Enhance prompt to request JSON output
	if !strings.Contains(req.Prompt, "JSON") && !strings.Contains(req.Prompt, "json") {
		req.Prompt = req.Prompt + "\n\nRespond with valid JSON only."
	}

	var lastErr error
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		resp, err := p.Complete(ctx, req)
		if err != nil {
			lastErr = err
			// Don't retry on context cancellation or timeouts
			if ctx.Err() != nil {
				return err
			}
			if llmErr, ok := err.(*LLMError); ok && llmErr.Code == ErrTimeout {
				return err // retrying a saturated server won't help
			}
			continue
		}

		// Check for token limit truncation before attempting JSON parse
		if resp.FinishReason == "length" {
			return &LLMError{
				Code:    ErrTokenLimit,
				Message: fmt.Sprintf("response truncated: hit max_tokens limit (%d completion tokens used)", resp.TokensUsed.Completion),
				Details: resp.Content,
			}
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
func (p *VLLMProvider) IsAvailable(ctx context.Context) bool {
	url := fmt.Sprintf("%s/health", p.config.BaseURL)
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
func (p *VLLMProvider) Close() error {
	p.httpClient.CloseIdleConnections()
	return nil
}
