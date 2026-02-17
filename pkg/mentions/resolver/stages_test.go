package resolver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	aiv1 "github.com/otherjamesbrown/penf-cli/api/proto/ai/v1"
	"github.com/otherjamesbrown/penf-cli/pkg/mentions"
)

// TestExecuteStage3_EntityIDAsNameString reproduces bugs pf-90b5ea and pf-15e435:
// When the LLM returns entity_id as a name string (e.g., "John Smith") instead of an integer ID,
// FlexInt64 parsing fails with "cannot parse "John Smith" as int64".
//
// Root cause: Even with JSON mode enabled in AIProvider, Gemini Flash sometimes returns
// entity_id as a name string instead of the numeric ID from the candidate list.
// This happens because the LLM sees both entity_id and entity_name in the prompt and
// occasionally uses the name value for both fields.
//
// Fix: parseStage3WithFallback (lines 392-503 in stages.go) catches FlexInt64 errors
// and attempts to map name strings back to numeric IDs using the candidate list.
//
// TEST STATUS: This test PASSES because the fallback is implemented.
// If the fallback code is removed, this test will FAIL, protecting against regression.
func TestExecuteStage3_EntityIDAsNameString(t *testing.T) {
	// Setup: Create mock AI client that returns entity_id as a name string
	mockClient := new(MockAIClient)
	cfg := LLMConfig{
		Model:      "gemini-2.0-flash",
		MaxRetries: 0,
	}
	provider := NewAIProvider(mockClient, cfg)

	// Mock response: entity_id is "John Smith" instead of 123
	// This is what Gemini Flash returns when not using JSON mode
	invalidJSON := `{
		"resolutions": [
			{
				"mention_text": "John",
				"mention_position": 0,
				"decision": "resolve",
				"resolved_to": {
					"entity_type": "person",
					"entity_id": "John Smith",
					"entity_name": "John Smith"
				},
				"confidence": 0.95,
				"reasoning": "Direct name match"
			}
		]
	}`

	mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
		Return(&aiv1.SummaryResponse{
			Summary:   invalidJSON,
			ModelUsed: "gemini-2.0-flash",
		}, nil)

	// Create executor with the AI provider
	executor := NewStageExecutor(provider, DefaultConfig())

	// Prepare test data
	understanding := &Stage1Understanding{
		Mentions: []MentionUnderstanding{
			{
				Text:       "John",
				EntityType: mentions.EntityTypePerson,
				Position:   0,
			},
		},
	}

	relationships := &Stage2CrossMention{
		ContentID:            123,
		UnifiedUnderstanding: "Discussion about John",
		MentionRelationships: []MentionRelationship{},
		ResolutionHints:      []string{},
	}

	candidates := map[string]*CandidateSet{
		"John": {
			MentionText: "John",
			Candidates: []CandidateWithHints{
				{
					EntityID:        123,
					EntityName:      "John Smith",
					EntityType:      mentions.EntityTypePerson,
					ConfidenceHints: map[string]interface{}{"match_type": "exact name match"},
				},
			},
		},
	}

	// Execute Stage 3
	result, err := executor.ExecuteStage3(context.Background(), understanding, relationships, candidates, "trace_test")

	// VERIFICATION: After fix, name strings should be mapped to IDs
	require.NoError(t, err, "Should handle entity_id as name string gracefully")
	require.NotNil(t, result, "Result should be valid")
	require.Len(t, result.Resolutions, 1)

	resolution := result.Resolutions[0]
	require.Equal(t, "John", resolution.MentionText)
	require.Equal(t, DecisionTypeResolve, resolution.Decision)
	require.NotNil(t, resolution.ResolvedTo)
	require.Equal(t, int64(123), resolution.ResolvedTo.EntityID.Int64(), "Name 'John Smith' should be mapped to ID 123")
	require.Equal(t, "John Smith", resolution.ResolvedTo.EntityName)

	t.Logf("SUCCESS: Name string 'John Smith' was mapped to entity ID 123")

	mockClient.AssertExpectations(t)
}

// TestExecuteStage3_EntityIDAsInteger verifies that valid integer entity_id works correctly.
// This test should PASS even with the current (unfixed) code.
func TestExecuteStage3_EntityIDAsInteger(t *testing.T) {
	// Setup: Create mock AI client that returns entity_id as a proper integer
	mockClient := new(MockAIClient)
	cfg := LLMConfig{
		Model:      "gemini-2.0-flash",
		MaxRetries: 0,
	}
	provider := NewAIProvider(mockClient, cfg)

	// Mock response: entity_id is 123 (correct format)
	validJSON := `{
		"resolutions": [
			{
				"mention_text": "John",
				"mention_position": 0,
				"decision": "resolve",
				"resolved_to": {
					"entity_type": "person",
					"entity_id": 123,
					"entity_name": "John Smith"
				},
				"confidence": 0.95,
				"reasoning": "Direct name match"
			}
		]
	}`

	mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
		Return(&aiv1.SummaryResponse{
			Summary:   validJSON,
			ModelUsed: "gemini-2.0-flash",
		}, nil)

	// Create executor with the AI provider
	executor := NewStageExecutor(provider, DefaultConfig())

	// Prepare test data
	understanding := &Stage1Understanding{
		Mentions: []MentionUnderstanding{
			{
				Text:       "John",
				EntityType: mentions.EntityTypePerson,
				Position:   0,
			},
		},
	}

	relationships := &Stage2CrossMention{
		ContentID:            123,
		UnifiedUnderstanding: "Discussion about John",
		MentionRelationships: []MentionRelationship{},
		ResolutionHints:      []string{},
	}

	candidates := map[string]*CandidateSet{
		"John": {
			MentionText: "John",
			Candidates: []CandidateWithHints{
				{
					EntityID:        123,
					EntityName:      "John Smith",
					EntityType:      mentions.EntityTypePerson,
					ConfidenceHints: map[string]interface{}{"match_type": "exact name match"},
				},
			},
		},
	}

	// Execute Stage 3
	result, err := executor.ExecuteStage3(context.Background(), understanding, relationships, candidates, "trace_test")

	// EXPECTED: Test should PASS with proper integer entity_id
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Resolutions, 1)

	resolution := result.Resolutions[0]
	require.Equal(t, "John", resolution.MentionText)
	require.Equal(t, DecisionTypeResolve, resolution.Decision)
	require.NotNil(t, resolution.ResolvedTo)
	require.Equal(t, int64(123), resolution.ResolvedTo.EntityID.Int64())
	require.Equal(t, "John Smith", resolution.ResolvedTo.EntityName)

	mockClient.AssertExpectations(t)
}

// TestExecuteStage3_EntityIDAsNumericString verifies that numeric strings work (FlexInt64 feature).
// This test should PASS with the current code because FlexInt64 handles numeric strings.
func TestExecuteStage3_EntityIDAsNumericString(t *testing.T) {
	// Setup: Create mock AI client that returns entity_id as a numeric string
	mockClient := new(MockAIClient)
	cfg := LLMConfig{
		Model:      "gemini-2.0-flash",
		MaxRetries: 0,
	}
	provider := NewAIProvider(mockClient, cfg)

	// Mock response: entity_id is "123" (quoted number - FlexInt64 should handle this)
	validJSON := `{
		"resolutions": [
			{
				"mention_text": "John",
				"mention_position": 0,
				"decision": "resolve",
				"resolved_to": {
					"entity_type": "person",
					"entity_id": "123",
					"entity_name": "John Smith"
				},
				"confidence": 0.95,
				"reasoning": "Direct name match"
			}
		]
	}`

	mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
		Return(&aiv1.SummaryResponse{
			Summary:   validJSON,
			ModelUsed: "gemini-2.0-flash",
		}, nil)

	// Create executor with the AI provider
	executor := NewStageExecutor(provider, DefaultConfig())

	// Prepare test data
	understanding := &Stage1Understanding{
		Mentions: []MentionUnderstanding{
			{
				Text:       "John",
				EntityType: mentions.EntityTypePerson,
				Position:   0,
			},
		},
	}

	relationships := &Stage2CrossMention{
		ContentID:            123,
		UnifiedUnderstanding: "Discussion about John",
		MentionRelationships: []MentionRelationship{},
		ResolutionHints:      []string{},
	}

	candidates := map[string]*CandidateSet{
		"John": {
			MentionText: "John",
			Candidates: []CandidateWithHints{
				{
					EntityID:        123,
					EntityName:      "John Smith",
					EntityType:      mentions.EntityTypePerson,
					ConfidenceHints: map[string]interface{}{"match_type": "exact name match"},
				},
			},
		},
	}

	// Execute Stage 3
	result, err := executor.ExecuteStage3(context.Background(), understanding, relationships, candidates, "trace_test")

	// EXPECTED: Test should PASS because FlexInt64 handles numeric strings
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Resolutions, 1)

	resolution := result.Resolutions[0]
	require.Equal(t, int64(123), resolution.ResolvedTo.EntityID.Int64())

	mockClient.AssertExpectations(t)
}

// TestExecuteStage3_EntityIDAsUnmatchedNameString verifies graceful handling when entity_id
// is a name string that doesn't match any candidate in the list.
func TestExecuteStage3_EntityIDAsUnmatchedNameString(t *testing.T) {
	// Setup: Create mock AI client that returns entity_id as an unmatched name string
	mockClient := new(MockAIClient)
	cfg := LLMConfig{
		Model:      "gemini-2.0-flash",
		MaxRetries: 0,
	}
	provider := NewAIProvider(mockClient, cfg)

	// Mock response: entity_id is "Jane Doe" which is NOT in the candidate list
	unmatchedJSON := `{
		"resolutions": [
			{
				"mention_text": "Jane",
				"mention_position": 0,
				"decision": "resolve",
				"resolved_to": {
					"entity_type": "person",
					"entity_id": "Jane Doe",
					"entity_name": "Jane Doe"
				},
				"confidence": 0.95,
				"reasoning": "Direct name match"
			}
		]
	}`

	mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
		Return(&aiv1.SummaryResponse{
			Summary:   unmatchedJSON,
			ModelUsed: "gemini-2.0-flash",
		}, nil)

	// Create executor with the AI provider
	executor := NewStageExecutor(provider, DefaultConfig())

	// Prepare test data - only has "John Smith" as candidate, not "Jane Doe"
	understanding := &Stage1Understanding{
		Mentions: []MentionUnderstanding{
			{
				Text:       "Jane",
				EntityType: mentions.EntityTypePerson,
				Position:   0,
			},
		},
	}

	relationships := &Stage2CrossMention{
		ContentID:            123,
		UnifiedUnderstanding: "Discussion about Jane",
		MentionRelationships: []MentionRelationship{},
		ResolutionHints:      []string{},
	}

	candidates := map[string]*CandidateSet{
		"Jane": {
			MentionText: "Jane",
			Candidates: []CandidateWithHints{
				{
					EntityID:        123,
					EntityName:      "John Smith", // Different name
					EntityType:      mentions.EntityTypePerson,
					ConfidenceHints: map[string]interface{}{"match_type": "partial"},
				},
			},
		},
	}

	// Execute Stage 3
	result, err := executor.ExecuteStage3(context.Background(), understanding, relationships, candidates, "trace_test")

	// VERIFICATION: Should fail gracefully since "Jane Doe" is not in candidate list
	// The fallback mapping can't fix this, so it should return the original FlexInt64 error
	require.Error(t, err, "Should error when name string doesn't match any candidate")
	require.Nil(t, result)
	require.Contains(t, err.Error(), "FlexInt64", "Error should mention FlexInt64 parsing issue")

	t.Logf("EXPECTED: Got FlexInt64 error for unmatched name: %v", err)

	mockClient.AssertExpectations(t)
}

// TestExecuteStage3_MultipleEntityIDsAsNameStrings reproduces bug pf-15e435:
// When multiple resolutions return entity_id as name strings, the fallback must handle
// all of them, including alternatives_considered arrays.
//
// This test verifies that the fallback mapping works for:
// 1. Primary resolved_to.entity_id fields
// 2. alternatives_considered[].entity_id fields
// 3. Multiple resolutions in the same response
func TestExecuteStage3_MultipleEntityIDsAsNameStrings(t *testing.T) {
	// Setup: Create mock AI client that returns multiple entity_ids as name strings
	mockClient := new(MockAIClient)
	cfg := LLMConfig{
		Model:      "gemini-2.0-flash",
		MaxRetries: 0,
	}
	provider := NewAIProvider(mockClient, cfg)

	// Mock response: Multiple resolutions with entity_id as name strings
	// Also includes alternatives_considered with name strings
	multipleNamesJSON := `{
		"resolutions": [
			{
				"mention_text": "John",
				"mention_position": 0,
				"decision": "resolve",
				"resolved_to": {
					"entity_type": "person",
					"entity_id": "John Smith",
					"entity_name": "John Smith"
				},
				"confidence": 0.95,
				"reasoning": "Direct name match",
				"alternatives_considered": [
					{
						"entity_id": "John Doe",
						"entity_name": "John Doe",
						"confidence": 0.3,
						"rejection_reason": "Less likely match"
					}
				]
			},
			{
				"mention_text": "Alice",
				"mention_position": 20,
				"decision": "resolve",
				"resolved_to": {
					"entity_type": "person",
					"entity_id": "Alice Johnson",
					"entity_name": "Alice Johnson"
				},
				"confidence": 0.88,
				"reasoning": "Clear name match"
			}
		]
	}`

	mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
		Return(&aiv1.SummaryResponse{
			Summary:   multipleNamesJSON,
			ModelUsed: "gemini-2.0-flash",
		}, nil)

	// Create executor with the AI provider
	executor := NewStageExecutor(provider, DefaultConfig())

	// Prepare test data with multiple mentions
	understanding := &Stage1Understanding{
		Mentions: []MentionUnderstanding{
			{
				Text:       "John",
				EntityType: mentions.EntityTypePerson,
				Position:   0,
			},
			{
				Text:       "Alice",
				EntityType: mentions.EntityTypePerson,
				Position:   20,
			},
		},
	}

	relationships := &Stage2CrossMention{
		ContentID:            123,
		UnifiedUnderstanding: "Discussion about John and Alice",
		MentionRelationships: []MentionRelationship{},
		ResolutionHints:      []string{},
	}

	candidates := map[string]*CandidateSet{
		"John": {
			MentionText: "John",
			Candidates: []CandidateWithHints{
				{
					EntityID:        100,
					EntityName:      "John Smith",
					EntityType:      mentions.EntityTypePerson,
					ConfidenceHints: map[string]interface{}{"match_type": "exact"},
				},
				{
					EntityID:        101,
					EntityName:      "John Doe",
					EntityType:      mentions.EntityTypePerson,
					ConfidenceHints: map[string]interface{}{"match_type": "partial"},
				},
			},
		},
		"Alice": {
			MentionText: "Alice",
			Candidates: []CandidateWithHints{
				{
					EntityID:        200,
					EntityName:      "Alice Johnson",
					EntityType:      mentions.EntityTypePerson,
					ConfidenceHints: map[string]interface{}{"match_type": "exact"},
				},
			},
		},
	}

	// Execute Stage 3
	result, err := executor.ExecuteStage3(context.Background(), understanding, relationships, candidates, "trace_test")

	// VERIFICATION: All name strings should be mapped to IDs
	require.NoError(t, err, "Should handle multiple entity_id name strings gracefully")
	require.NotNil(t, result, "Result should be valid")
	require.Len(t, result.Resolutions, 2)

	// Check first resolution (John)
	johnRes := result.Resolutions[0]
	require.Equal(t, "John", johnRes.MentionText)
	require.Equal(t, DecisionTypeResolve, johnRes.Decision)
	require.NotNil(t, johnRes.ResolvedTo)
	require.Equal(t, int64(100), johnRes.ResolvedTo.EntityID.Int64(), "Name 'John Smith' should be mapped to ID 100")
	require.Equal(t, "John Smith", johnRes.ResolvedTo.EntityName)

	// Check alternatives for first resolution
	require.Len(t, johnRes.Alternatives, 1)
	require.Equal(t, int64(101), johnRes.Alternatives[0].EntityID.Int64(), "Alternative 'John Doe' should be mapped to ID 101")
	require.Equal(t, "John Doe", johnRes.Alternatives[0].EntityName)

	// Check second resolution (Alice)
	aliceRes := result.Resolutions[1]
	require.Equal(t, "Alice", aliceRes.MentionText)
	require.Equal(t, DecisionTypeResolve, aliceRes.Decision)
	require.NotNil(t, aliceRes.ResolvedTo)
	require.Equal(t, int64(200), aliceRes.ResolvedTo.EntityID.Int64(), "Name 'Alice Johnson' should be mapped to ID 200")
	require.Equal(t, "Alice Johnson", aliceRes.ResolvedTo.EntityName)

	t.Logf("SUCCESS: Mapped 3 name strings to IDs: John Smith->100, John Doe->101, Alice Johnson->200")

	mockClient.AssertExpectations(t)
}

// TestExecuteStage3_WithoutFallback_WouldFail documents bug pf-15e435 regression protection.
//
// This test demonstrates that without the parseStage3WithFallback function (lines 392-503
// in stages.go), the resolver would fail when the LLM returns entity_id as a name string.
//
// REGRESSION PROTECTION: If someone removes or breaks the fallback logic in
// parseStage3WithFallback, this test will start failing, alerting them to the regression.
//
// NOTE: This test currently PASSES because the fallback is working. To see the original
// bug, temporarily comment out the fallback logic in stages.go:392-426 and re-run.
func TestExecuteStage3_WithoutFallback_WouldFail(t *testing.T) {
	mockClient := new(MockAIClient)
	cfg := LLMConfig{
		Model:      "gemini-2.0-flash",
		MaxRetries: 0,
	}
	provider := NewAIProvider(mockClient, cfg)

	// This is what Gemini Flash actually returns without strict schema enforcement:
	// entity_id is "Team Status Report" (a name string) instead of a numeric ID
	buggyJSON := `{
		"resolutions": [
			{
				"mention_text": "Team Status Report",
				"mention_position": 45,
				"decision": "resolve",
				"resolved_to": {
					"entity_type": "product",
					"entity_id": "Team Status Report",
					"entity_name": "Team Status Report"
				},
				"confidence": 0.85,
				"reasoning": "Exact match to project name"
			}
		]
	}`

	mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
		Return(&aiv1.SummaryResponse{
			Summary:   buggyJSON,
			ModelUsed: "gemini-2.0-flash",
		}, nil)

	executor := NewStageExecutor(provider, DefaultConfig())

	understanding := &Stage1Understanding{
		Mentions: []MentionUnderstanding{
			{
				Text:       "Team Status Report",
				EntityType: mentions.EntityTypeProduct,
				Position:   45,
			},
		},
	}

	relationships := &Stage2CrossMention{
		ContentID:            789,
		UnifiedUnderstanding: "Discussion about Team Status Report project",
		MentionRelationships: []MentionRelationship{},
		ResolutionHints:      []string{},
	}

	candidates := map[string]*CandidateSet{
		"Team Status Report": {
			MentionText: "Team Status Report",
			Candidates: []CandidateWithHints{
				{
					EntityID:        456,
					EntityName:      "Team Status Report",
					EntityType:      mentions.EntityTypeProduct,
					ConfidenceHints: map[string]interface{}{"match_type": "exact"},
				},
			},
		},
	}

	// Execute Stage 3
	result, err := executor.ExecuteStage3(context.Background(), understanding, relationships, candidates, "trace_test")

	// WITH FALLBACK: Test passes - name string is mapped to ID 456
	// WITHOUT FALLBACK: Would get error "FlexInt64: cannot parse "Team Status Report" as int64"
	require.NoError(t, err, "Fallback should handle entity_id as name string")
	require.NotNil(t, result)
	require.Len(t, result.Resolutions, 1)
	require.Equal(t, int64(456), result.Resolutions[0].ResolvedTo.EntityID.Int64())

	t.Logf("PASS: Fallback successfully mapped 'Team Status Report' string to ID 456")
	t.Logf("To reproduce original bug pf-15e435: comment out parseStage3WithFallback fallback logic")

	mockClient.AssertExpectations(t)
}

