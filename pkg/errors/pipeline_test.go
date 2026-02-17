package errors

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestClassifyError_Nil(t *testing.T) {
	result := ClassifyError(nil, "test-stage")
	if result != nil {
		t.Errorf("Expected nil for nil error, got %v", result)
	}
}

func TestClassifyError_DeadlineExceeded(t *testing.T) {
	err := context.DeadlineExceeded
	result := ClassifyError(err, "test-stage")

	if result == nil {
		t.Fatal("Expected non-nil PipelineError")
	}
	if result.Code != ErrTimeout {
		t.Errorf("Expected ErrTimeout, got %s", result.Code)
	}
	if result.Stage != "test-stage" {
		t.Errorf("Expected stage 'test-stage', got %s", result.Stage)
	}
	if result.Message != "operation timed out" {
		t.Errorf("Expected 'operation timed out', got %s", result.Message)
	}
	if result.Cause != err {
		t.Errorf("Expected cause to be original error")
	}
}

func TestClassifyError_Canceled(t *testing.T) {
	err := context.Canceled
	result := ClassifyError(err, "test-stage")

	if result == nil {
		t.Fatal("Expected non-nil PipelineError")
	}
	if result.Code != ErrContextCancelled {
		t.Errorf("Expected ErrContextCancelled, got %s", result.Code)
	}
	if result.Message != "operation cancelled" {
		t.Errorf("Expected 'operation cancelled', got %s", result.Message)
	}
}

func TestClassifyError_RateLimit(t *testing.T) {
	tests := []struct {
		name     string
		errorMsg string
	}{
		{"rate limit exact", "rate limit exceeded"},
		{"429 status", "HTTP 429 error"},
		{"too many requests", "too many requests"},
		{"quota exceeded", "quota exceeded for this resource"},
		{"resource exhausted", "resource_exhausted error from gRPC"},
		{"Rate Limit uppercase", "Rate Limit Error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errorMsg)
			result := ClassifyError(err, "test-stage")

			if result == nil {
				t.Fatal("Expected non-nil PipelineError")
			}
			if result.Code != ErrRateLimit {
				t.Errorf("Expected ErrRateLimit for '%s', got %s", tt.errorMsg, result.Code)
			}
			if result.Message != tt.errorMsg {
				t.Errorf("Expected message '%s', got %s", tt.errorMsg, result.Message)
			}
		})
	}
}

func TestClassifyError_ModelUnavailable(t *testing.T) {
	tests := []struct {
		name     string
		errorMsg string
	}{
		{"connection refused", "connection refused"},
		{"unavailable", "service unavailable"},
		{"503 status", "HTTP 503 error"},
		{"service unavailable exact", "Service Unavailable"},
		{"no such host", "dial tcp: lookup example.com: no such host"},
		{"Unavailable uppercase", "Model Unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errorMsg)
			result := ClassifyError(err, "test-stage")

			if result == nil {
				t.Fatal("Expected non-nil PipelineError")
			}
			if result.Code != ErrModelUnavailable {
				t.Errorf("Expected ErrModelUnavailable for '%s', got %s", tt.errorMsg, result.Code)
			}
			if result.Message != tt.errorMsg {
				t.Errorf("Expected message '%s', got %s", tt.errorMsg, result.Message)
			}
		})
	}
}

func TestClassifyError_Unknown(t *testing.T) {
	err := errors.New("some random error")
	result := ClassifyError(err, "test-stage")

	if result == nil {
		t.Fatal("Expected non-nil PipelineError")
	}
	if result.Code != ErrProcessingError {
		t.Errorf("Expected ErrProcessingError for unrecognized error, got %s", result.Code)
	}
	if result.Message != "some random error" {
		t.Errorf("Expected message 'some random error', got %s", result.Message)
	}
}

func TestPipelineError_Error_WithTimeout(t *testing.T) {
	pe := &PipelineError{
		Code:     ErrTimeout,
		Stage:    "Stage 4 DeepAnalyze",
		Duration: 120 * time.Second,
		Timeout:  120 * time.Second,
	}

	expected := "timeout: Stage 4 DeepAnalyze timed out after 2m0s (limit: 2m0s)"
	if pe.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, pe.Error())
	}
}

func TestPipelineError_Error_WithStage(t *testing.T) {
	pe := &PipelineError{
		Code:    ErrRateLimit,
		Stage:   "Stage 1 Triage",
		Message: "quota exceeded",
	}

	expected := "rate_limit: Stage 1 Triage: quota exceeded"
	if pe.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, pe.Error())
	}
}

func TestPipelineError_Error_NoStage(t *testing.T) {
	pe := &PipelineError{
		Code:    ErrProcessingError,
		Message: "something went wrong",
	}

	expected := "processing_error: something went wrong"
	if pe.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, pe.Error())
	}
}

func TestPipelineError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	pe := &PipelineError{
		Code:  ErrProcessingError,
		Cause: originalErr,
	}

	unwrapped := pe.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Expected unwrapped error to be original error")
	}
}

func TestIsTimeout(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "timeout error",
			err:      &PipelineError{Code: ErrTimeout},
			expected: true,
		},
		{
			name:     "rate limit error",
			err:      &PipelineError{Code: ErrRateLimit},
			expected: false,
		},
		{
			name:     "processing error",
			err:      &PipelineError{Code: ErrProcessingError},
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTimeout(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsErrorRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "timeout error",
			err:      &PipelineError{Code: ErrTimeout},
			expected: true,
		},
		{
			name:     "rate limit error",
			err:      &PipelineError{Code: ErrRateLimit},
			expected: true,
		},
		{
			name:     "model unavailable error",
			err:      &PipelineError{Code: ErrModelUnavailable},
			expected: true,
		},
		{
			name:     "processing error",
			err:      &PipelineError{Code: ErrProcessingError},
			expected: false,
		},
		{
			name:     "parse error",
			err:      &PipelineError{Code: ErrParseError},
			expected: false,
		},
		{
			name:     "context cancelled error",
			err:      &PipelineError{Code: ErrContextCancelled},
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsErrorRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestClassifyError_EmptyContent(t *testing.T) {
	tests := []struct {
		name     string
		errorMsg string
	}{
		{"empty content", "empty content in response"},
		{"content is empty", "content is empty after parsing"},
		{"no content", "no content found in document"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errorMsg)
			result := ClassifyError(err, "test-stage")

			if result == nil {
				t.Fatal("Expected non-nil PipelineError")
			}
			if result.Code != ErrEmptyContent {
				t.Errorf("Expected ErrEmptyContent for '%s', got %s", tt.errorMsg, result.Code)
			}
			if result.Message != tt.errorMsg {
				t.Errorf("Expected message '%s', got %s", tt.errorMsg, result.Message)
			}
		})
	}
}

func TestPipelineError_Error_WithDurationAndTimeout(t *testing.T) {
	pe := &PipelineError{
		Code:     ErrTimeout,
		Stage:    "Stage 2 Extract",
		Message:  "operation timed out",
		Duration: 45 * time.Second,
		Timeout:  30 * time.Second,
	}

	expected := "timeout: Stage 2 Extract timed out after 45s (limit: 30s)"
	if pe.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, pe.Error())
	}

	// When only Duration is set (no Timeout), should fall through to stage+message format
	peNoTimeout := &PipelineError{
		Code:     ErrTimeout,
		Stage:    "Stage 2 Extract",
		Message:  "operation timed out",
		Duration: 45 * time.Second,
	}

	expectedNoTimeout := "timeout: Stage 2 Extract: operation timed out"
	if peNoTimeout.Error() != expectedNoTimeout {
		t.Errorf("Expected '%s', got '%s'", expectedNoTimeout, peNoTimeout.Error())
	}

	// When only Timeout is set (no Duration), should fall through to stage+message format
	peNoDuration := &PipelineError{
		Code:    ErrTimeout,
		Stage:   "Stage 2 Extract",
		Message: "operation timed out",
		Timeout: 30 * time.Second,
	}

	expectedNoDuration := "timeout: Stage 2 Extract: operation timed out"
	if peNoDuration.Error() != expectedNoDuration {
		t.Errorf("Expected '%s', got '%s'", expectedNoDuration, peNoDuration.Error())
	}
}

func TestClassifyError_WrappedErrors(t *testing.T) {
	// Test that context.DeadlineExceeded works even when wrapped
	wrappedErr := fmt.Errorf("wrapped: %w", context.DeadlineExceeded)
	result := ClassifyError(wrappedErr, "test-stage")

	if result == nil {
		t.Fatal("Expected non-nil PipelineError")
	}
	if result.Code != ErrTimeout {
		t.Errorf("Expected ErrTimeout for wrapped DeadlineExceeded, got %s", result.Code)
	}
}
