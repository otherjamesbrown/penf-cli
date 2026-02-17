package errors

// ErrorCodeInfo contains metadata about an error code.
type ErrorCodeInfo struct {
	Code            ErrorCode
	Retryable       bool
	Description     string
	SuggestedAction string
}

// ErrorCodeRegistry maps error codes to their metadata.
var ErrorCodeRegistry = map[ErrorCode]ErrorCodeInfo{
	ErrTimeout: {
		Code:            ErrTimeout,
		Retryable:       true,
		Description:     "Operation exceeded time limit",
		SuggestedAction: "Check timeout configuration: penf pipeline config --key timeout",
	},
	ErrTimeoutHeartbeat: {
		Code:            ErrTimeoutHeartbeat,
		Retryable:       true,
		Description:     "Activity heartbeat timeout (worker may be stuck or overloaded)",
		SuggestedAction: "Check worker health: penf pipeline health, penf pipeline queue",
	},
	ErrRateLimit: {
		Code:            ErrRateLimit,
		Retryable:       true,
		Description:     "API rate limit exceeded",
		SuggestedAction: "Wait and retry automatically, or check quota limits with AI provider",
	},
	ErrRateLimitEmbedding: {
		Code:            ErrRateLimitEmbedding,
		Retryable:       true,
		Description:     "Embedding API rate limit exceeded",
		SuggestedAction: "Reduce batch size or wait for rate limit reset",
	},
	ErrModelUnavailable: {
		Code:            ErrModelUnavailable,
		Retryable:       true,
		Description:     "AI model or service unavailable",
		SuggestedAction: "Check service health: penf pipeline health, or verify model deployment",
	},
	ErrContextCancelled: {
		Code:            ErrContextCancelled,
		Retryable:       false,
		Description:     "Operation cancelled by user or system",
		SuggestedAction: "Check if cancellation was intentional, or investigate upstream cancellation",
	},
	ErrParseError: {
		Code:            ErrParseError,
		Retryable:       false,
		Description:     "Content parsing failed (malformed structure)",
		SuggestedAction: "Inspect raw content: penf content text <content-id>",
	},
	ErrEmptyContent: {
		Code:            ErrEmptyContent,
		Retryable:       false,
		Description:     "Content is empty or missing",
		SuggestedAction: "Verify source data: penf content show <content-id>",
	},
	ErrContentTooLarge: {
		Code:            ErrContentTooLarge,
		Retryable:       false,
		Description:     "Content exceeds maximum size limit",
		SuggestedAction: "Content may need to be split or summarized before processing",
	},
	ErrStageDependencyFailed: {
		Code:            ErrStageDependencyFailed,
		Retryable:       false,
		Description:     "Upstream pipeline stage failed",
		SuggestedAction: "Fix upstream stage error first: penf content trace <content-id>",
	},
	ErrDuplicateContent: {
		Code:            ErrDuplicateContent,
		Retryable:       false,
		Description:     "Content already exists (duplicate hash)",
		SuggestedAction: "This is expected for duplicate content; no action needed",
	},
	ErrEntityResolutionFailed: {
		Code:            ErrEntityResolutionFailed,
		Retryable:       false,
		Description:     "Entity resolution failed (database or matching error)",
		SuggestedAction: "Check entity database: penf glossary list",
	},
	ErrEmbeddingDimensionMismatch: {
		Code:            ErrEmbeddingDimensionMismatch,
		Retryable:       false,
		Description:     "Embedding dimension mismatch (model or config change)",
		SuggestedAction: "Verify embedding model configuration and reprocess if needed",
	},
	ErrProcessingError: {
		Code:            ErrProcessingError,
		Retryable:       false,
		Description:     "Unclassified processing error",
		SuggestedAction: "Check logs: penf pipeline logs <job-id>, or penf content trace <content-id>",
	},
}

// IsRetryable returns true if the given error code represents a transient, retryable error.
func IsRetryable(code ErrorCode) bool {
	if info, ok := ErrorCodeRegistry[code]; ok {
		return info.Retryable
	}
	return false
}

// GetSuggestedAction returns the suggested action for the given error code.
func GetSuggestedAction(code ErrorCode) string {
	if info, ok := ErrorCodeRegistry[code]; ok {
		return info.SuggestedAction
	}
	return "Check logs for more details: penf pipeline logs <job-id>"
}

// GetDescription returns the human-readable description for the given error code.
func GetDescription(code ErrorCode) string {
	if info, ok := ErrorCodeRegistry[code]; ok {
		return info.Description
	}
	return "Unknown error"
}
