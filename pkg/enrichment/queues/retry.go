package queues

import (
	"time"
)

// RetryPolicy defines retry behavior for failed messages.
type RetryPolicy struct {
	MaxRetries      int           `yaml:"max_retries"`
	InitialBackoff  time.Duration `yaml:"initial_backoff"`
	MaxBackoff      time.Duration `yaml:"max_backoff"`
	BackoffFactor   float64       `yaml:"backoff_factor"`
	RetryableErrors []string      `yaml:"retryable_errors"`
}

// DefaultRetryPolicy returns the default retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     5 * time.Minute,
		BackoffFactor:  2.0,
		RetryableErrors: []string{
			ErrorCodeTimeout,
			ErrorCodeRateLimited,
			ErrorCodeServiceUnavailable,
		},
	}
}

// CalculateBackoff calculates the backoff duration for a given retry attempt.
func (p RetryPolicy) CalculateBackoff(retryCount int) time.Duration {
	if retryCount <= 0 {
		return p.InitialBackoff
	}

	backoff := p.InitialBackoff
	for i := 0; i < retryCount; i++ {
		backoff = time.Duration(float64(backoff) * p.BackoffFactor)
		if backoff > p.MaxBackoff {
			return p.MaxBackoff
		}
	}
	return backoff
}

// ShouldRetry determines if an error should trigger a retry.
func (p RetryPolicy) ShouldRetry(err error, retryCount int) bool {
	if retryCount >= p.MaxRetries {
		return false
	}

	// Check if it's a ProcessingError
	if procErr, ok := err.(*ProcessingError); ok {
		// Check category
		if procErr.IsRetryable() {
			return true
		}

		// Check error code
		for _, code := range p.RetryableErrors {
			if procErr.Code == code {
				return true
			}
		}
	}

	return false
}

// RetryDecision represents the decision about whether to retry.
type RetryDecision struct {
	ShouldRetry    bool
	BackoffDuration time.Duration
	Reason         string
}

// DecideRetry makes a retry decision based on the error and retry count.
func (p RetryPolicy) DecideRetry(err error, retryCount int) RetryDecision {
	if retryCount >= p.MaxRetries {
		return RetryDecision{
			ShouldRetry: false,
			Reason:      "max retries exceeded",
		}
	}

	if procErr, ok := err.(*ProcessingError); ok {
		if !procErr.IsRetryable() {
			return RetryDecision{
				ShouldRetry: false,
				Reason:      "permanent error: " + procErr.Code,
			}
		}
	}

	return RetryDecision{
		ShouldRetry:    true,
		BackoffDuration: p.CalculateBackoff(retryCount),
		Reason:         "retryable error",
	}
}

// RetryableErrorCodes returns the list of error codes that trigger retries.
var RetryableErrorCodes = []string{
	ErrorCodeTimeout,
	ErrorCodeRateLimited,
	ErrorCodeServiceUnavailable,
	ErrorCodeDatabaseError,
	ErrorCodeExternalAPI,
}

// IsRetryableCode checks if an error code should trigger a retry.
func IsRetryableCode(code string) bool {
	for _, c := range RetryableErrorCodes {
		if c == code {
			return true
		}
	}
	return false
}
