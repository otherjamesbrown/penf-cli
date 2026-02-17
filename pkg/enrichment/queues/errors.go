package queues

import "errors"

// Queue errors.
var (
	ErrUnknownMessageType = errors.New("unknown message type")
	ErrQueueEmpty         = errors.New("queue is empty")
	ErrMessageNotFound    = errors.New("message not found")
	ErrQueueClosed        = errors.New("queue is closed")
	ErrInvalidMessage     = errors.New("invalid message")
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

// ErrorCategory categorizes processing errors for retry decisions.
type ErrorCategory string

const (
	// ErrorCategoryTransient indicates a temporary error that should be retried.
	ErrorCategoryTransient ErrorCategory = "transient"
	// ErrorCategoryPermanent indicates an error that will not be resolved by retry.
	ErrorCategoryPermanent ErrorCategory = "permanent"
	// ErrorCategoryPartial indicates some processors succeeded, some failed.
	ErrorCategoryPartial ErrorCategory = "partial"
	// ErrorCategoryDependency indicates an external dependency failure.
	ErrorCategoryDependency ErrorCategory = "dependency"
)

// ProcessingError wraps errors with category information.
type ProcessingError struct {
	Category ErrorCategory
	Code     string
	Message  string
	Err      error
}

func (e *ProcessingError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *ProcessingError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error should trigger a retry.
func (e *ProcessingError) IsRetryable() bool {
	return e.Category == ErrorCategoryTransient || e.Category == ErrorCategoryDependency
}

// NewTransientError creates a new transient error.
func NewTransientError(code, message string, err error) *ProcessingError {
	return &ProcessingError{
		Category: ErrorCategoryTransient,
		Code:     code,
		Message:  message,
		Err:      err,
	}
}

// NewPermanentError creates a new permanent error.
func NewPermanentError(code, message string, err error) *ProcessingError {
	return &ProcessingError{
		Category: ErrorCategoryPermanent,
		Code:     code,
		Message:  message,
		Err:      err,
	}
}

// NewPartialError creates a new partial error.
func NewPartialError(code, message string, err error) *ProcessingError {
	return &ProcessingError{
		Category: ErrorCategoryPartial,
		Code:     code,
		Message:  message,
		Err:      err,
	}
}

// NewDependencyError creates a new dependency error.
func NewDependencyError(code, message string, err error) *ProcessingError {
	return &ProcessingError{
		Category: ErrorCategoryDependency,
		Code:     code,
		Message:  message,
		Err:      err,
	}
}

// Common error codes.
const (
	ErrorCodeTimeout          = "TIMEOUT"
	ErrorCodeRateLimited      = "RATE_LIMITED"
	ErrorCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrorCodeInvalidInput     = "INVALID_INPUT"
	ErrorCodeParseError       = "PARSE_ERROR"
	ErrorCodeDatabaseError    = "DATABASE_ERROR"
	ErrorCodeExternalAPI      = "EXTERNAL_API_ERROR"
	ErrorCodeLLMError         = "LLM_ERROR"
)
