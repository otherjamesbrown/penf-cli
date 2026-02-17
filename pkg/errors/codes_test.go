package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCodeRegistry_Completeness(t *testing.T) {
	// All error codes should be registered
	allCodes := []ErrorCode{
		ErrTimeout,
		ErrTimeoutHeartbeat,
		ErrRateLimit,
		ErrRateLimitEmbedding,
		ErrModelUnavailable,
		ErrContextCancelled,
		ErrParseError,
		ErrEmptyContent,
		ErrContentTooLarge,
		ErrStageDependencyFailed,
		ErrDuplicateContent,
		ErrEntityResolutionFailed,
		ErrEmbeddingDimensionMismatch,
		ErrProcessingError,
	}

	for _, code := range allCodes {
		t.Run(string(code), func(t *testing.T) {
			info, ok := ErrorCodeRegistry[code]
			assert.True(t, ok, "ErrorCode %s should be in registry", code)
			assert.Equal(t, code, info.Code, "Registry entry should have matching code")
			assert.NotEmpty(t, info.Description, "Description should not be empty")
			assert.NotEmpty(t, info.SuggestedAction, "SuggestedAction should not be empty")
		})
	}
}

func TestIsRetryable_ErrorCode(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected bool
	}{
		{ErrTimeout, true},
		{ErrTimeoutHeartbeat, true},
		{ErrRateLimit, true},
		{ErrRateLimitEmbedding, true},
		{ErrModelUnavailable, true},
		{ErrContextCancelled, false},
		{ErrParseError, false},
		{ErrEmptyContent, false},
		{ErrContentTooLarge, false},
		{ErrStageDependencyFailed, false},
		{ErrDuplicateContent, false},
		{ErrEntityResolutionFailed, false},
		{ErrEmbeddingDimensionMismatch, false},
		{ErrProcessingError, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			assert.Equal(t, tt.expected, IsRetryable(tt.code),
				"IsRetryable(%s) should be %v", tt.code, tt.expected)
		})
	}
}

func TestGetSuggestedAction(t *testing.T) {
	// All registered codes should have a suggested action
	for code := range ErrorCodeRegistry {
		action := GetSuggestedAction(code)
		assert.NotEmpty(t, action, "Code %s should have a suggested action", code)
		// Action should be actionable (either a command, a clear instruction, or an explanation)
		assert.True(t, len(action) > 10, "Action for %s should be meaningful (>10 chars)", code)
	}

	// Unknown code should return default
	action := GetSuggestedAction("unknown_code")
	assert.Contains(t, action, "logs", "Unknown codes should suggest checking logs")
}

func TestGetDescription(t *testing.T) {
	// All registered codes should have a description
	for code := range ErrorCodeRegistry {
		desc := GetDescription(code)
		assert.NotEmpty(t, desc, "Code %s should have a description", code)
	}

	// Unknown code should return default
	desc := GetDescription("unknown_code")
	assert.Equal(t, "Unknown error", desc)
}

func TestErrorCodeRegistry_AllCodesUnique(t *testing.T) {
	// Ensure all error code strings are unique (no duplicates)
	seen := make(map[ErrorCode]bool)
	for code := range ErrorCodeRegistry {
		assert.False(t, seen[code], "Error code %s should be unique", code)
		seen[code] = true
	}
}

func TestErrorCodeRegistry_ActionsAreConcrete(t *testing.T) {
	// All suggested actions should be concrete commands or clear instructions
	for code, info := range ErrorCodeRegistry {
		action := info.SuggestedAction

		// Should not be vague
		assert.NotContains(t, action, "might", "Action for %s should be concrete, not vague", code)
		assert.NotContains(t, action, "maybe", "Action for %s should be concrete, not vague", code)

		// Should be meaningful (not too short)
		assert.True(t, len(action) > 15, "Action for %s should be meaningful (>15 chars): %s", code, action)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
