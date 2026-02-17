// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewBriefingCommand verifies the briefing command structure.
func TestNewBriefingCommand(t *testing.T) {
	deps := DefaultBriefingDeps()
	cmd := NewBriefingCommand(deps)

	assert.Equal(t, "briefing", cmd.Use[:8], "command name should be briefing")
	assert.NotEmpty(t, cmd.Short, "command should have short description")
	assert.NotEmpty(t, cmd.Long, "command should have long description")

	// Verify flags exist.
	tierFlag := cmd.Flags().Lookup("tier")
	require.NotNil(t, tierFlag, "tier flag should exist")
	assert.Equal(t, "int32", tierFlag.Value.Type(), "tier flag should be int32")

	limitFlag := cmd.Flags().Lookup("limit")
	require.NotNil(t, limitFlag, "limit flag should exist")
	assert.Equal(t, "int32", limitFlag.Value.Type(), "limit flag should be int32")

	outputFlag := cmd.Flags().Lookup("output")
	require.NotNil(t, outputFlag, "output flag should exist")
	assert.Equal(t, "string", outputFlag.Value.Type(), "output flag should be string")

	// Verify shorthand for output flag.
	outputShortFlag := cmd.Flags().ShorthandLookup("o")
	require.NotNil(t, outputShortFlag, "output flag should have shorthand -o")
}

// TestBriefingCommand_RequiresProject verifies that project name is required.
func TestBriefingCommand_RequiresProject(t *testing.T) {
	deps := DefaultBriefingDeps()
	cmd := NewBriefingCommand(deps)

	// Execute without arguments should fail.
	err := cmd.Execute()
	assert.Error(t, err, "briefing command should require project name argument")
}

// TestNewEscalationsCommand verifies the escalations command structure.
func TestNewEscalationsCommand(t *testing.T) {
	deps := DefaultBriefingDeps()
	cmd := NewEscalationsCommand(deps)

	assert.Equal(t, "escalations", cmd.Use, "command name should be escalations")
	assert.NotEmpty(t, cmd.Short, "command should have short description")
	assert.NotEmpty(t, cmd.Long, "command should have long description")

	// Verify flags exist.
	sourceFlag := cmd.Flags().Lookup("source-id")
	require.NotNil(t, sourceFlag, "source-id flag should exist")
	assert.Equal(t, "int64", sourceFlag.Value.Type(), "source-id flag should be int64")

	outputFlag := cmd.Flags().Lookup("output")
	require.NotNil(t, outputFlag, "output flag should exist")
	assert.Equal(t, "string", outputFlag.Value.Type(), "output flag should be string")

	// Verify shorthand for output flag.
	outputShortFlag := cmd.Flags().ShorthandLookup("o")
	require.NotNil(t, outputShortFlag, "output flag should have shorthand -o")
}

// TestEscalationsCommand_RequiresSourceID verifies that --source-id is required.
func TestEscalationsCommand_RequiresSourceID(t *testing.T) {
	deps := DefaultBriefingDeps()
	cmd := NewEscalationsCommand(deps)

	// Execute without --source-id should fail.
	err := cmd.Execute()
	assert.Error(t, err, "escalations command should require --source-id flag")
}

// TestFormatTierName tests the tier name formatting helper function.
func TestFormatTierName(t *testing.T) {
	tests := []struct {
		name     string
		tier     int32
		expected string
	}{
		{
			name:     "Tier 1 - Watched Items",
			tier:     1,
			expected: "Tier 1: Watched Items",
		},
		{
			name:     "Tier 2 - Trusted Source",
			tier:     2,
			expected: "Tier 2: Trusted Source",
		},
		{
			name:     "Tier 3 - Senior Source",
			tier:     3,
			expected: "Tier 3: Senior Source",
		},
		{
			name:     "Tier 4 - Other",
			tier:     4,
			expected: "Tier 4: Other",
		},
		{
			name:     "Unknown Tier",
			tier:     99,
			expected: "Tier 99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTierName(tt.tier)
			assert.Equal(t, tt.expected, result, "tier name should match expected format")
		})
	}
}

// TestGetSeverityColor tests the severity color helper function.
func TestGetSeverityColor(t *testing.T) {
	tests := []struct {
		name     string
		severity string
		expected string
	}{
		{
			name:     "Critical severity - red",
			severity: "critical",
			expected: "\033[31m",
		},
		{
			name:     "High severity - yellow",
			severity: "high",
			expected: "\033[33m",
		},
		{
			name:     "Medium severity - cyan",
			severity: "medium",
			expected: "\033[36m",
		},
		{
			name:     "Low severity - green",
			severity: "low",
			expected: "\033[32m",
		},
		{
			name:     "Unknown severity - no color",
			severity: "unknown",
			expected: "",
		},
		{
			name:     "Case insensitive - CRITICAL",
			severity: "CRITICAL",
			expected: "\033[31m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSeverityColor(tt.severity)
			assert.Equal(t, tt.expected, result, "severity color should match expected ANSI code")
		})
	}
}

// TestBriefingCommandFlags tests flag parsing for briefing command.
func TestBriefingCommandFlags(t *testing.T) {
	deps := DefaultBriefingDeps()
	cmd := NewBriefingCommand(deps)

	// Set flags.
	cmd.SetArgs([]string{"TestProject", "--tier", "2", "--limit", "25", "-o", "json"})

	// Parse flags (don't execute).
	err := cmd.ParseFlags([]string{"TestProject", "--tier", "2", "--limit", "25", "-o", "json"})
	require.NoError(t, err, "flag parsing should succeed")

	// Verify flag values.
	tierFlag := cmd.Flags().Lookup("tier")
	assert.Equal(t, "2", tierFlag.Value.String())

	limitFlag := cmd.Flags().Lookup("limit")
	assert.Equal(t, "25", limitFlag.Value.String())

	outputFlag := cmd.Flags().Lookup("output")
	assert.Equal(t, "json", outputFlag.Value.String())
}

// TestEscalationsCommandFlags tests flag parsing for escalations command.
func TestEscalationsCommandFlags(t *testing.T) {
	deps := DefaultBriefingDeps()
	cmd := NewEscalationsCommand(deps)

	// Set flags.
	err := cmd.ParseFlags([]string{"--source-id", "12345", "-o", "yaml"})
	require.NoError(t, err, "flag parsing should succeed")

	// Verify flag values.
	sourceFlag := cmd.Flags().Lookup("source-id")
	assert.Equal(t, "12345", sourceFlag.Value.String())

	outputFlag := cmd.Flags().Lookup("output")
	assert.Equal(t, "yaml", outputFlag.Value.String())
}
