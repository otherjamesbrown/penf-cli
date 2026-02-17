package resolver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	aiv1 "github.com/otherjamesbrown/penf-cli/api/proto/ai/v1"
)

// MockAIClient implements AIClient for testing.
type MockAIClient struct {
	mock.Mock
}

func (m *MockAIClient) GenerateSummary(ctx context.Context, req *aiv1.SummaryRequest) (*aiv1.SummaryResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aiv1.SummaryResponse), args.Error(1)
}

func (m *MockAIClient) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockAIClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// TestNewAIProvider tests the constructor
func TestNewAIProvider(t *testing.T) {
	t.Run("default name", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{}

		provider := NewAIProvider(mockClient, cfg)

		assert.NotNil(t, provider)
		assert.Equal(t, "ai-service", provider.Name())
	})

	t.Run("name with model", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{
			Model: "mistral-7b",
		}

		provider := NewAIProvider(mockClient, cfg)

		assert.NotNil(t, provider)
		assert.Equal(t, "ai-mistral-7b", provider.Name())
	})
}

// TestAIProvider_Complete tests the Complete method
func TestAIProvider_Complete(t *testing.T) {
	t.Run("successful completion", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{
			Model: "test-model",
		}
		provider := NewAIProvider(mockClient, cfg)

		inputTokens := int32(10)
		outputTokens := int32(20)
		mockClient.On("GenerateSummary", mock.Anything, mock.MatchedBy(func(req *aiv1.SummaryRequest) bool {
			return req.Content == "Hello world" &&
				req.Style == aiv1.SummaryStyle_SUMMARY_STYLE_DETAILED &&
				*req.Model == "test-model"
		})).Return(&aiv1.SummaryResponse{
			Summary:      "Response text",
			ModelUsed:    "test-model",
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
		}, nil)

		resp, err := provider.Complete(context.Background(), CompletionRequest{
			Prompt: "Hello world",
		})

		require.NoError(t, err)
		assert.Equal(t, "Response text", resp.Content)
		assert.Equal(t, "test-model", resp.Model)
		assert.Equal(t, 10, resp.TokensUsed.Prompt)
		assert.Equal(t, 20, resp.TokensUsed.Completion)
		assert.Equal(t, 30, resp.TokensUsed.Total)
		mockClient.AssertExpectations(t)
	})

	t.Run("with system prompt", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{}
		provider := NewAIProvider(mockClient, cfg)

		mockClient.On("GenerateSummary", mock.Anything, mock.MatchedBy(func(req *aiv1.SummaryRequest) bool {
			return req.Content == "System: Be helpful\n\nUser: Hello" &&
				req.Style == aiv1.SummaryStyle_SUMMARY_STYLE_DETAILED
		})).Return(&aiv1.SummaryResponse{
			Summary:   "Hi there!",
			ModelUsed: "default-model",
		}, nil)

		resp, err := provider.Complete(context.Background(), CompletionRequest{
			Prompt:       "Hello",
			SystemPrompt: "Be helpful",
		})

		require.NoError(t, err)
		assert.Equal(t, "Hi there!", resp.Content)
		mockClient.AssertExpectations(t)
	})

	t.Run("with max tokens", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{}
		provider := NewAIProvider(mockClient, cfg)

		mockClient.On("GenerateSummary", mock.Anything, mock.MatchedBy(func(req *aiv1.SummaryRequest) bool {
			return req.MaxLength != nil && *req.MaxLength == 100
		})).Return(&aiv1.SummaryResponse{
			Summary:   "Short response",
			ModelUsed: "model",
		}, nil)

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Prompt:    "Hello",
			MaxTokens: 100,
		})

		require.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("AI service error", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{}
		provider := NewAIProvider(mockClient, cfg)

		mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
			Return(nil, errors.New("service unavailable"))

		resp, err := provider.Complete(context.Background(), CompletionRequest{
			Prompt: "Hello",
		})

		require.Error(t, err)
		assert.Nil(t, resp)

		llmErr, ok := err.(*LLMError)
		require.True(t, ok)
		assert.Equal(t, ErrUnavailable, llmErr.Code)
		assert.Contains(t, llmErr.Message, "service unavailable")
		mockClient.AssertExpectations(t)
	})

	t.Run("context timeout", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{}
		provider := NewAIProvider(mockClient, cfg)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		time.Sleep(5 * time.Millisecond) // Let context expire

		mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
			Return(nil, context.DeadlineExceeded)

		resp, err := provider.Complete(ctx, CompletionRequest{
			Prompt: "Hello",
		})

		require.Error(t, err)
		assert.Nil(t, resp)

		llmErr, ok := err.(*LLMError)
		require.True(t, ok)
		assert.Equal(t, ErrTimeout, llmErr.Code)
		mockClient.AssertExpectations(t)
	})
}

// TestAIProvider_CompleteStructured tests the CompleteStructured method
func TestAIProvider_CompleteStructured(t *testing.T) {
	t.Run("successful JSON parsing", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{MaxRetries: 2}
		provider := NewAIProvider(mockClient, cfg)

		mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
			Return(&aiv1.SummaryResponse{
				Summary:   `{"name": "John", "age": 30}`,
				ModelUsed: "model",
			}, nil)

		var result struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		err := provider.CompleteStructured(context.Background(), CompletionRequest{
			Prompt: "Return JSON",
		}, &result)

		require.NoError(t, err)
		assert.Equal(t, "John", result.Name)
		assert.Equal(t, 30, result.Age)
		mockClient.AssertExpectations(t)
	})

	t.Run("JSON wrapped in markdown", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{MaxRetries: 2}
		provider := NewAIProvider(mockClient, cfg)

		mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
			Return(&aiv1.SummaryResponse{
				Summary:   "```json\n{\"value\": 42}\n```",
				ModelUsed: "model",
			}, nil)

		var result struct {
			Value int `json:"value"`
		}

		err := provider.CompleteStructured(context.Background(), CompletionRequest{
			Prompt: "Return JSON",
		}, &result)

		require.NoError(t, err)
		assert.Equal(t, 42, result.Value)
		mockClient.AssertExpectations(t)
	})

	t.Run("adds JSON hint if not present", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{MaxRetries: 0}
		provider := NewAIProvider(mockClient, cfg)

		mockClient.On("GenerateSummary", mock.Anything, mock.MatchedBy(func(req *aiv1.SummaryRequest) bool {
			return req.Content == "Give me data\n\nRespond with valid JSON only."
		})).Return(&aiv1.SummaryResponse{
			Summary:   `{"data": true}`,
			ModelUsed: "model",
		}, nil)

		var result struct {
			Data bool `json:"data"`
		}

		err := provider.CompleteStructured(context.Background(), CompletionRequest{
			Prompt: "Give me data",
		}, &result)

		require.NoError(t, err)
		assert.True(t, result.Data)
		mockClient.AssertExpectations(t)
	})

	t.Run("retries on parse failure", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{MaxRetries: 1}
		provider := NewAIProvider(mockClient, cfg)

		// First call returns invalid JSON
		mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
			Return(&aiv1.SummaryResponse{
				Summary:   "not valid json",
				ModelUsed: "model",
			}, nil).Once()

		// Second call returns valid JSON
		mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
			Return(&aiv1.SummaryResponse{
				Summary:   `{"success": true}`,
				ModelUsed: "model",
			}, nil).Once()

		var result struct {
			Success bool `json:"success"`
		}

		err := provider.CompleteStructured(context.Background(), CompletionRequest{
			Prompt: "Return JSON",
		}, &result)

		require.NoError(t, err)
		assert.True(t, result.Success)
		mockClient.AssertExpectations(t)
	})

	t.Run("returns error after max retries", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{MaxRetries: 1}
		provider := NewAIProvider(mockClient, cfg)

		// Both calls return invalid JSON
		mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
			Return(&aiv1.SummaryResponse{
				Summary:   "not valid json",
				ModelUsed: "model",
			}, nil)

		var result struct {
			Success bool `json:"success"`
		}

		err := provider.CompleteStructured(context.Background(), CompletionRequest{
			Prompt: "Return JSON",
		}, &result)

		require.Error(t, err)
		llmErr, ok := err.(*LLMError)
		require.True(t, ok)
		assert.Equal(t, ErrParseFailure, llmErr.Code)
		mockClient.AssertExpectations(t)
	})
}

// TestAIProvider_IsAvailable tests the IsAvailable method
func TestAIProvider_IsAvailable(t *testing.T) {
	t.Run("available when health check passes", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{}
		provider := NewAIProvider(mockClient, cfg)

		mockClient.On("HealthCheck", mock.Anything).Return(nil)

		available := provider.IsAvailable(context.Background())

		assert.True(t, available)
		mockClient.AssertExpectations(t)
	})

	t.Run("unavailable when health check fails", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{}
		provider := NewAIProvider(mockClient, cfg)

		mockClient.On("HealthCheck", mock.Anything).Return(errors.New("unhealthy"))

		available := provider.IsAvailable(context.Background())

		assert.False(t, available)
		mockClient.AssertExpectations(t)
	})
}

// TestAIProvider_Close tests the Close method
func TestAIProvider_Close(t *testing.T) {
	t.Run("delegates to client", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{}
		provider := NewAIProvider(mockClient, cfg)

		mockClient.On("Close").Return(nil)

		err := provider.Close()

		require.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("propagates errors", func(t *testing.T) {
		mockClient := new(MockAIClient)
		cfg := LLMConfig{}
		provider := NewAIProvider(mockClient, cfg)

		mockClient.On("Close").Return(errors.New("close failed"))

		err := provider.Close()

		require.Error(t, err)
		assert.Equal(t, "close failed", err.Error())
		mockClient.AssertExpectations(t)
	})
}

// TestAIProvider_ImplementsLLMProvider ensures AIProvider implements LLMProvider
func TestAIProvider_ImplementsLLMProvider(t *testing.T) {
	mockClient := new(MockAIClient)
	cfg := LLMConfig{}
	provider := NewAIProvider(mockClient, cfg)

	// This is a compile-time check - if AIProvider doesn't implement LLMProvider,
	// this will fail to compile
	var _ LLMProvider = provider
}
