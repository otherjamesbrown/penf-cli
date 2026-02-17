package errors

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrorCode represents a classified pipeline error.
type ErrorCode string

const (
	ErrTimeout                    ErrorCode = "timeout"
	ErrTimeoutHeartbeat           ErrorCode = "timeout_heartbeat"
	ErrRateLimit                  ErrorCode = "rate_limit"
	ErrRateLimitEmbedding         ErrorCode = "rate_limit_embedding"
	ErrModelUnavailable           ErrorCode = "model_unavailable"
	ErrContextCancelled           ErrorCode = "context_cancelled"
	ErrParseError                 ErrorCode = "parse_error"
	ErrEmptyContent               ErrorCode = "empty_content"
	ErrContentTooLarge            ErrorCode = "content_too_large"
	ErrStageDependencyFailed      ErrorCode = "stage_dependency_failed"
	ErrDuplicateContent           ErrorCode = "duplicate_content"
	ErrEntityResolutionFailed     ErrorCode = "entity_resolution_failed"
	ErrEmbeddingDimensionMismatch ErrorCode = "embedding_dimension_mismatch"
	ErrProcessingError            ErrorCode = "processing_error"
)

// PipelineError is a structured error for pipeline failures.
type PipelineError struct {
	Code     ErrorCode
	Stage    string
	Message  string
	Duration time.Duration
	Timeout  time.Duration
	Cause    error
}

func (e *PipelineError) Error() string {
	if e.Timeout > 0 && e.Duration > 0 {
		return fmt.Sprintf("%s: %s timed out after %s (limit: %s)", e.Code, e.Stage, e.Duration.Truncate(time.Second), e.Timeout.Truncate(time.Second))
	}
	if e.Stage != "" {
		return fmt.Sprintf("%s: %s: %s", e.Code, e.Stage, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *PipelineError) Unwrap() error {
	return e.Cause
}

// ClassifyError inspects an error and returns a *PipelineError with the appropriate code.
// If the error doesn't match any known pattern, it returns a PipelineError with ErrProcessingError.
func ClassifyError(err error, stage string) *PipelineError {
	if err == nil {
		return nil
	}

	pe := &PipelineError{
		Stage: stage,
		Cause: err,
	}

	// Check for context deadline exceeded (timeout)
	if errors.Is(err, context.DeadlineExceeded) {
		pe.Code = ErrTimeout
		pe.Message = "operation timed out"
		return pe
	}

	// Check for context cancelled
	if errors.Is(err, context.Canceled) {
		pe.Code = ErrContextCancelled
		pe.Message = "operation cancelled"
		return pe
	}

	// Check error message patterns
	msg := err.Error()
	lower := strings.ToLower(msg)

	// Heartbeat timeout patterns
	if strings.Contains(lower, "heartbeat") && strings.Contains(lower, "timeout") {
		pe.Code = ErrTimeoutHeartbeat
		pe.Message = msg
		return pe
	}

	// Empty content patterns
	if strings.Contains(lower, "empty content") || strings.Contains(lower, "content is empty") || strings.Contains(lower, "no content") {
		pe.Code = ErrEmptyContent
		pe.Message = msg
		return pe
	}

	// Content too large patterns
	if strings.Contains(lower, "too large") || strings.Contains(lower, "exceeds maximum") || strings.Contains(lower, "content size") {
		pe.Code = ErrContentTooLarge
		pe.Message = msg
		return pe
	}

	// Duplicate content patterns
	if strings.Contains(lower, "duplicate") || strings.Contains(lower, "already exists") {
		pe.Code = ErrDuplicateContent
		pe.Message = msg
		return pe
	}

	// Entity resolution patterns
	if strings.Contains(lower, "entity resolution") || strings.Contains(lower, "entity lookup") {
		pe.Code = ErrEntityResolutionFailed
		pe.Message = msg
		return pe
	}

	// Embedding dimension mismatch patterns
	if strings.Contains(lower, "dimension mismatch") || strings.Contains(lower, "embedding dimension") {
		pe.Code = ErrEmbeddingDimensionMismatch
		pe.Message = msg
		return pe
	}

	// Stage dependency patterns
	if strings.Contains(lower, "upstream") || strings.Contains(lower, "dependency failed") || strings.Contains(lower, "prerequisite") {
		pe.Code = ErrStageDependencyFailed
		pe.Message = msg
		return pe
	}

	// Embedding rate limit patterns (more specific than general rate limit)
	if strings.Contains(lower, "embedding") && (strings.Contains(lower, "rate limit") || strings.Contains(lower, "quota")) {
		pe.Code = ErrRateLimitEmbedding
		pe.Message = msg
		return pe
	}

	// General rate limit patterns
	if strings.Contains(lower, "rate limit") || strings.Contains(lower, "429") || strings.Contains(lower, "too many requests") || strings.Contains(lower, "quota exceeded") || strings.Contains(lower, "resource_exhausted") {
		pe.Code = ErrRateLimit
		pe.Message = msg
		return pe
	}

	// Model unavailable patterns
	if strings.Contains(lower, "connection refused") || strings.Contains(lower, "unavailable") || strings.Contains(lower, "503") || strings.Contains(lower, "service unavailable") || strings.Contains(lower, "no such host") {
		pe.Code = ErrModelUnavailable
		pe.Message = msg
		return pe
	}

	// Default to processing error
	pe.Code = ErrProcessingError
	pe.Message = msg
	return pe
}

// IsTimeout returns true if the error is a timeout error.
func IsTimeout(err error) bool {
	var pe *PipelineError
	if errors.As(err, &pe) {
		return pe.Code == ErrTimeout
	}
	return false
}

// IsErrorRetryable returns true if the error is likely transient and worth retrying.
// This function checks the error code using the ErrorCodeRegistry.
func IsErrorRetryable(err error) bool {
	var pe *PipelineError
	if errors.As(err, &pe) {
		if info, ok := ErrorCodeRegistry[pe.Code]; ok {
			return info.Retryable
		}
		// Default to non-retryable for unknown codes
		return false
	}
	return false
}
