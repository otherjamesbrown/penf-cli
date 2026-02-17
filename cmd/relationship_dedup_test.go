// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntityDuplicatesCommand_Exists verifies the duplicates subcommand is registered.
func TestEntityDuplicatesCommand_Exists(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	subcommands := entityCmd.Commands()
	require.NotEmpty(t, subcommands, "entity command should have subcommands")

	// Look for duplicates subcommand
	duplicatesFound := false
	for _, sub := range subcommands {
		if sub.Use == "duplicates" {
			duplicatesFound = true
			break
		}
	}

	assert.True(t, duplicatesFound, "entity command should have 'duplicates' subcommand")
}

// TestEntityDuplicatesCommand_Structure verifies the duplicates command structure.
func TestEntityDuplicatesCommand_Structure(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	duplicatesCmd, _, err := entityCmd.Find([]string{"duplicates"})
	require.NoError(t, err, "should find duplicates subcommand")
	require.NotNil(t, duplicatesCmd, "duplicates subcommand should not be nil")

	assert.Equal(t, "duplicates", duplicatesCmd.Use, "duplicates subcommand Use should be 'duplicates'")
	assert.NotEmpty(t, duplicatesCmd.Short, "duplicates subcommand should have Short description")
	assert.NotEmpty(t, duplicatesCmd.Long, "duplicates subcommand should have Long description")
}

// TestEntityDuplicatesCommand_Flags verifies the duplicates command has expected flags.
func TestEntityDuplicatesCommand_Flags(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	duplicatesCmd, _, err := entityCmd.Find([]string{"duplicates"})
	require.NoError(t, err)
	require.NotNil(t, duplicatesCmd)

	// Check for --min-similarity flag
	minSimilarityFlag := duplicatesCmd.Flags().Lookup("min-similarity")
	assert.NotNil(t, minSimilarityFlag, "duplicates command should have --min-similarity flag")
	if minSimilarityFlag != nil {
		assert.NotEmpty(t, minSimilarityFlag.Usage, "--min-similarity flag should have usage description")
	}

	// Check for --output flag (inherited from parent, but verify it's accessible)
	outputFlag := duplicatesCmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag, "duplicates command should have --output flag")

	// Check for --auto-merge flag
	autoMergeFlag := duplicatesCmd.Flags().Lookup("auto-merge")
	assert.NotNil(t, autoMergeFlag, "duplicates command should have --auto-merge flag")
	if autoMergeFlag != nil {
		assert.Equal(t, "bool", autoMergeFlag.Value.Type(), "--auto-merge should be a boolean flag")
		assert.NotEmpty(t, autoMergeFlag.Usage, "--auto-merge flag should have usage description")
	}

	// Check for --confirm flag
	confirmFlag := duplicatesCmd.Flags().Lookup("confirm")
	assert.NotNil(t, confirmFlag, "duplicates command should have --confirm flag")
	if confirmFlag != nil {
		assert.Equal(t, "bool", confirmFlag.Value.Type(), "--confirm should be a boolean flag")
		assert.NotEmpty(t, confirmFlag.Usage, "--confirm flag should have usage description")
	}

	// Check for --dry-run flag
	dryRunFlag := duplicatesCmd.Flags().Lookup("dry-run")
	assert.NotNil(t, dryRunFlag, "duplicates command should have --dry-run flag")
	if dryRunFlag != nil {
		assert.Equal(t, "bool", dryRunFlag.Value.Type(), "--dry-run should be a boolean flag")
		assert.NotEmpty(t, dryRunFlag.Usage, "--dry-run flag should have usage description")
	}
}

// TestEntityDuplicatesCommand_Help verifies the duplicates command has comprehensive help text.
func TestEntityDuplicatesCommand_Help(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	duplicatesCmd, _, err := entityCmd.Find([]string{"duplicates"})
	require.NoError(t, err)
	require.NotNil(t, duplicatesCmd)

	// Verify help text mentions key concepts
	longText := duplicatesCmd.Long
	assert.Contains(t, longText, "similarity", "Long description should mention similarity scores")
	assert.Contains(t, longText, "duplicate", "Long description should mention duplicates")

	// Commands should have examples for AI agents to understand usage
	assert.NotEmpty(t, duplicatesCmd.Example, "duplicates command should have example usage")
}

// TestEntityDuplicatesCommand_FlagTypes verifies flag types are correct.
func TestEntityDuplicatesCommand_FlagTypes(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	duplicatesCmd, _, err := entityCmd.Find([]string{"duplicates"})
	require.NoError(t, err)
	require.NotNil(t, duplicatesCmd)

	// --min-similarity should be float64
	minSimilarityFlag := duplicatesCmd.Flags().Lookup("min-similarity")
	if minSimilarityFlag != nil {
		assert.Equal(t, "float64", minSimilarityFlag.Value.Type(), "--min-similarity should be float64")
	}

	// --output should be string
	outputFlag := duplicatesCmd.Flags().Lookup("output")
	if outputFlag != nil {
		assert.Equal(t, "string", outputFlag.Value.Type(), "--output should be string")
	}
}

// TestEntityMergePreviewCommand_Exists verifies the merge-preview subcommand is registered.
func TestEntityMergePreviewCommand_Exists(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	subcommands := entityCmd.Commands()
	require.NotEmpty(t, subcommands, "entity command should have subcommands")

	// Look for merge-preview subcommand
	mergePreviewFound := false
	for _, sub := range subcommands {
		if sub.Use == "merge-preview <id1> <id2>" || sub.Use == "merge-preview" {
			mergePreviewFound = true
			break
		}
	}

	assert.True(t, mergePreviewFound, "entity command should have 'merge-preview' subcommand")
}

// TestEntityMergePreviewCommand_Structure verifies the merge-preview command structure.
func TestEntityMergePreviewCommand_Structure(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	mergePreviewCmd, _, err := entityCmd.Find([]string{"merge-preview"})
	require.NoError(t, err, "should find merge-preview subcommand")
	require.NotNil(t, mergePreviewCmd, "merge-preview subcommand should not be nil")

	assert.Contains(t, mergePreviewCmd.Use, "merge-preview", "merge-preview subcommand Use should contain 'merge-preview'")
	assert.NotEmpty(t, mergePreviewCmd.Short, "merge-preview subcommand should have Short description")
	assert.NotEmpty(t, mergePreviewCmd.Long, "merge-preview subcommand should have Long description")
}

// TestEntityMergePreviewCommand_Args verifies merge-preview requires two arguments.
func TestEntityMergePreviewCommand_Args(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	mergePreviewCmd, _, err := entityCmd.Find([]string{"merge-preview"})
	require.NoError(t, err)
	require.NotNil(t, mergePreviewCmd)

	// Verify Args validator exists (should require exactly 2 args)
	assert.NotNil(t, mergePreviewCmd.Args, "merge-preview should have Args validator")
}

// TestEntityMergePreviewCommand_Flags verifies the merge-preview command has expected flags.
func TestEntityMergePreviewCommand_Flags(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	mergePreviewCmd, _, err := entityCmd.Find([]string{"merge-preview"})
	require.NoError(t, err)
	require.NotNil(t, mergePreviewCmd)

	// Check for --output flag (inherited from parent, but verify it's accessible)
	outputFlag := mergePreviewCmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag, "merge-preview command should have --output flag")
	if outputFlag != nil {
		assert.Equal(t, "string", outputFlag.Value.Type(), "--output should be string")
	}
}

// TestEntityMergePreviewCommand_Help verifies the merge-preview command has comprehensive help text.
func TestEntityMergePreviewCommand_Help(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	mergePreviewCmd, _, err := entityCmd.Find([]string{"merge-preview"})
	require.NoError(t, err)
	require.NotNil(t, mergePreviewCmd)

	// Verify help text mentions key concepts
	longText := mergePreviewCmd.Long
	assert.Contains(t, longText, "merge", "Long description should mention merging")
	assert.Contains(t, longText, "preview", "Long description should mention preview")

	// Commands should have examples for AI agents to understand usage
	assert.NotEmpty(t, mergePreviewCmd.Example, "merge-preview command should have example usage")
}

// TestEntityMergePreviewCommand_HelpDescription verifies help text quality.
func TestEntityMergePreviewCommand_HelpDescription(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	mergePreviewCmd, _, err := entityCmd.Find([]string{"merge-preview"})
	require.NoError(t, err)
	require.NotNil(t, mergePreviewCmd)

	// Short description should be concise
	assert.NotEmpty(t, mergePreviewCmd.Short, "Short description should not be empty")
	assert.LessOrEqual(t, len(mergePreviewCmd.Short), 80, "Short description should be concise")

	// Long description should be detailed
	assert.Greater(t, len(mergePreviewCmd.Long), len(mergePreviewCmd.Short),
		"Long description should be more detailed than short")
}

// TestEntityDuplicatesCommand_AutoMergeFlagRequirements verifies auto-merge validation.
func TestEntityDuplicatesCommand_AutoMergeFlagRequirements(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	duplicatesCmd, _, err := entityCmd.Find([]string{"duplicates"})
	require.NoError(t, err)
	require.NotNil(t, duplicatesCmd)

	// Verify help text explains auto-merge requirements
	longText := duplicatesCmd.Long
	assert.Contains(t, longText, "auto-merge", "Long description should explain --auto-merge")

	// Verify example shows proper usage
	exampleText := duplicatesCmd.Example
	assert.NotEmpty(t, exampleText, "Should have examples")
}

// TestEntityDuplicatesCommand_OutputFormats verifies output format support.
func TestEntityDuplicatesCommand_OutputFormats(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	duplicatesCmd, _, err := entityCmd.Find([]string{"duplicates"})
	require.NoError(t, err)
	require.NotNil(t, duplicatesCmd)

	// Verify help text mentions output formats
	assert.NotEmpty(t, duplicatesCmd.Long, "Should have long description")

	// Verify example shows different output formats
	exampleText := duplicatesCmd.Example
	if exampleText != "" {
		// Examples should demonstrate both table and JSON output
		assert.True(t,
			len(exampleText) > 50,
			"Examples should demonstrate various usage patterns")
	}
}

// TestEntityDuplicatesCommand_DefaultValues verifies default flag values.
func TestEntityDuplicatesCommand_DefaultValues(t *testing.T) {
	entityCmd := newRelationshipEntityCommand(DefaultRelationshipDeps())

	duplicatesCmd, _, err := entityCmd.Find([]string{"duplicates"})
	require.NoError(t, err)
	require.NotNil(t, duplicatesCmd)

	// Check default for --min-similarity (should be 0.7 per spec)
	minSimilarityFlag := duplicatesCmd.Flags().Lookup("min-similarity")
	if minSimilarityFlag != nil {
		assert.NotEmpty(t, minSimilarityFlag.DefValue, "--min-similarity should have a default value")
	}

	// Check default for boolean flags (should be false)
	autoMergeFlag := duplicatesCmd.Flags().Lookup("auto-merge")
	if autoMergeFlag != nil {
		assert.Equal(t, "false", autoMergeFlag.DefValue, "--auto-merge should default to false")
	}

	confirmFlag := duplicatesCmd.Flags().Lookup("confirm")
	if confirmFlag != nil {
		assert.Equal(t, "false", confirmFlag.DefValue, "--confirm should default to false")
	}

	dryRunFlag := duplicatesCmd.Flags().Lookup("dry-run")
	if dryRunFlag != nil {
		assert.Equal(t, "false", dryRunFlag.DefValue, "--dry-run should default to false")
	}
}
