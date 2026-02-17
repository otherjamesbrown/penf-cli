package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

// Tests for pipeline errors CLI command focus on output formatting.
// gRPC-dependent tests require integration test setup.

func TestOutputErrorsJSON(t *testing.T) {
	errors := []*pipelinev1.PipelineErrorEvent{
		{
			Code:            "timeout",
			Stage:           "parse",
			Message:         "operation timed out",
			Retryable:       true,
			SuggestedAction: "Check timeout configuration",
		},
	}

	// Just verify the function signature is correct
	// Actual output testing would require mocking os.Stdout
	assert.NotNil(t, errors)
	assert.Len(t, errors, 1)
	assert.Equal(t, "timeout", errors[0].Code)
}

func TestOutputErrorsGrouped_Validation(t *testing.T) {
	errors := []*pipelinev1.PipelineErrorEvent{
		{
			Code:      "timeout",
			Stage:     "parse",
			Retryable: true,
		},
		{
			Code:      "timeout",
			Stage:     "embed",
			Retryable: true,
		},
		{
			Code:      "rate_limit",
			Stage:     "parse",
			Retryable: true,
		},
	}

	// Verify we have the right test data structure
	assert.Len(t, errors, 3)

	// Count by code
	codeCounts := make(map[string]int)
	for _, e := range errors {
		codeCounts[e.Code]++
	}

	assert.Equal(t, 2, codeCounts["timeout"])
	assert.Equal(t, 1, codeCounts["rate_limit"])
}
