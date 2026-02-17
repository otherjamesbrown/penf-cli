// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

// TestPipelineReprocessHasTimeoutFlag verifies the reprocess command has --timeout flag.
func TestPipelineReprocessHasTimeoutFlag(t *testing.T) {
	deps := &PipelineCommandDeps{
		LoadConfig: func() (*config.CLIConfig, error) {
			return &config.CLIConfig{}, nil
		},
	}

	cmd := newPipelineReprocessCmd(deps)

	// Check timeout flag exists
	timeoutFlag := cmd.Flags().Lookup("timeout")
	if timeoutFlag == nil {
		t.Fatal("reprocess command missing --timeout flag")
	}

	// Check it's an int32 type (seconds)
	if timeoutFlag.Value.Type() != "int32" {
		t.Errorf("--timeout flag has type %s, expected int32", timeoutFlag.Value.Type())
	}
}

// TestPipelineReprocessHasModelFlag verifies the reprocess command has --model flag.
func TestPipelineReprocessHasModelFlag(t *testing.T) {
	deps := &PipelineCommandDeps{
		LoadConfig: func() (*config.CLIConfig, error) {
			return &config.CLIConfig{}, nil
		},
	}

	cmd := newPipelineReprocessCmd(deps)

	// Check model flag exists
	modelFlag := cmd.Flags().Lookup("model")
	if modelFlag == nil {
		t.Fatal("reprocess command missing --model flag")
	}

	// Check it's a string type
	if modelFlag.Value.Type() != "string" {
		t.Errorf("--model flag has type %s, expected string", modelFlag.Value.Type())
	}
}

// TestPipelineDiffSubcommandExists verifies the diff subcommand exists.
func TestPipelineDiffSubcommandExists(t *testing.T) {
	cmd := NewPipelineCommand(nil)

	// Find diff subcommand
	var diffCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if subCmd.Name() == "diff" {
			diffCmd = subCmd
			break
		}
	}

	if diffCmd == nil {
		t.Fatal("pipeline command missing 'diff' subcommand")
	}

	// Verify it accepts exactly 1 arg (source-id)
	// Cobra's Args validation happens at runtime, but we can check the Use string
	if !strings.Contains(diffCmd.Use, "<source-id>") && !strings.Contains(diffCmd.Use, "source-id") {
		t.Errorf("diff subcommand Use string does not mention source-id: %q", diffCmd.Use)
	}
}

// TestPipelineDiffHasRunFlags verifies the diff subcommand has --run-a and --run-b flags.
func TestPipelineDiffHasRunFlags(t *testing.T) {
	cmd := NewPipelineCommand(nil)

	// Find diff subcommand
	var diffCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if subCmd.Name() == "diff" {
			diffCmd = subCmd
			break
		}
	}

	if diffCmd == nil {
		t.Fatal("pipeline command missing 'diff' subcommand")
	}

	// Check --run-a flag
	runAFlag := diffCmd.Flags().Lookup("run-a")
	if runAFlag == nil {
		t.Error("diff subcommand missing --run-a flag")
	} else {
		// Should be int64 type for run IDs
		if runAFlag.Value.Type() != "int64" {
			t.Errorf("--run-a flag has type %s, expected int64", runAFlag.Value.Type())
		}
	}

	// Check --run-b flag
	runBFlag := diffCmd.Flags().Lookup("run-b")
	if runBFlag == nil {
		t.Error("diff subcommand missing --run-b flag")
	} else {
		// Should be int64 type for run IDs
		if runBFlag.Value.Type() != "int64" {
			t.Errorf("--run-b flag has type %s, expected int64", runBFlag.Value.Type())
		}
	}
}

// TestPipelineDiffHelpText verifies the diff command help mentions key concepts.
func TestPipelineDiffHelpText(t *testing.T) {
	cmd := NewPipelineCommand(nil)

	// Find diff subcommand
	var diffCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if subCmd.Name() == "diff" {
			diffCmd = subCmd
			break
		}
	}

	if diffCmd == nil {
		t.Fatal("pipeline command missing 'diff' subcommand")
	}

	// Check that Long help text mentions comparisons
	longHelp := diffCmd.Long
	if longHelp == "" {
		t.Error("diff command has no Long help text")
	}

	// Should mention comparing or differences
	if !strings.Contains(strings.ToLower(longHelp), "compar") && !strings.Contains(strings.ToLower(longHelp), "differ") {
		t.Errorf("diff command help should mention comparing or differences, got: %q", longHelp)
	}
}

// TestPipelineDiffArgsValidation verifies the diff command accepts exactly one argument.
func TestPipelineDiffArgsValidation(t *testing.T) {
	cmd := NewPipelineCommand(nil)

	// Find diff subcommand
	var diffCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if subCmd.Name() == "diff" {
			diffCmd = subCmd
			break
		}
	}

	if diffCmd == nil {
		t.Fatal("pipeline command missing 'diff' subcommand")
	}

	// The Args field should be set to cobra.ExactArgs(1)
	// We can't directly check this, but we can verify it's set
	if diffCmd.Args == nil {
		t.Error("diff command has no Args validation set")
	}

	// Try to validate with wrong number of args
	// If Args is set to ExactArgs(1), this should fail
	if diffCmd.Args != nil {
		// Test with 0 args
		err := diffCmd.Args(diffCmd, []string{})
		if err == nil {
			t.Error("diff command should reject 0 arguments")
		}

		// Test with 2 args
		err = diffCmd.Args(diffCmd, []string{"123", "456"})
		if err == nil {
			t.Error("diff command should reject 2 arguments")
		}

		// Test with 1 arg (should succeed)
		err = diffCmd.Args(diffCmd, []string{"123"})
		if err != nil {
			t.Errorf("diff command should accept 1 argument, got error: %v", err)
		}
	}
}

// TestPipelineReprocessDryRunOutputMentionsOverrides verifies dry-run output format.
// This test checks that the dry-run output function signature includes parameters
// for showing override information.
func TestPipelineReprocessDryRunOutputFormat(t *testing.T) {
	// This test verifies that outputReprocessDryRunHuman exists and can be called
	// The actual implementation will show override info when model/timeout are provided

	// Create a mock response
	resp := &pipelinev1.ReprocessDryRunResponse{
		SourceCount:              42,
		AffectedStages:           []string{"triage", "extract"},
		EstimatedDurationSeconds: 120,
		Message:                  "Test message",
	}

	// Call the output function - should not panic
	err := outputReprocessDryRunHuman(resp, "triage")
	if err != nil {
		t.Errorf("outputReprocessDryRunHuman failed: %v", err)
	}

	// Note: The actual test of showing model/timeout in output will require
	// capturing stdout and verifying the content. This structural test ensures
	// the function exists and can be called with the expected parameters.
}

// TestPipelineReprocessFlagsDefaults verifies default values for override flags.
func TestPipelineReprocessFlagsDefaults(t *testing.T) {
	deps := &PipelineCommandDeps{
		LoadConfig: func() (*config.CLIConfig, error) {
			return &config.CLIConfig{}, nil
		},
	}

	cmd := newPipelineReprocessCmd(deps)

	// Check --timeout default
	timeoutFlag := cmd.Flags().Lookup("timeout")
	if timeoutFlag != nil {
		defaultValue := timeoutFlag.DefValue
		// Default should be empty (optional override)
		if defaultValue != "0" && defaultValue != "" {
			t.Errorf("--timeout default should be 0 or empty, got %s", defaultValue)
		}
	}

	// Check --model default
	modelFlag := cmd.Flags().Lookup("model")
	if modelFlag != nil {
		defaultValue := modelFlag.DefValue
		// Default should be empty (no override)
		if defaultValue != "" {
			t.Errorf("--model default should be empty, got %s", defaultValue)
		}
	}
}

// TestPipelineDiffCommandShortHelp verifies the diff command has a short description.
func TestPipelineDiffCommandShortHelp(t *testing.T) {
	cmd := NewPipelineCommand(nil)

	// Find diff subcommand
	var diffCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if subCmd.Name() == "diff" {
			diffCmd = subCmd
			break
		}
	}

	if diffCmd == nil {
		t.Fatal("pipeline command missing 'diff' subcommand")
	}

	// Check Short help text exists
	if diffCmd.Short == "" {
		t.Error("diff command has no Short help text")
	}

	// Should be concise (under 80 chars is good practice)
	if len(diffCmd.Short) > 80 {
		t.Errorf("diff command Short help is too long (%d chars): %q", len(diffCmd.Short), diffCmd.Short)
	}
}
