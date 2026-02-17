package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestPipelineStageCommand(t *testing.T) {
	deps := DefaultPipelineDeps()
	cmd := newPipelineStageCmd(deps)

	if cmd == nil {
		t.Fatal("newPipelineStageCmd returned nil")
	}

	if cmd.Use != "stage" {
		t.Errorf("Expected Use='stage', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short help text")
	}
}

func TestPipelineStageListCommand(t *testing.T) {
	deps := DefaultPipelineDeps()
	cmd := newPipelineStageListCmd(deps)

	if cmd == nil {
		t.Fatal("newPipelineStageListCmd returned nil")
	}

	if cmd.Use != "list" {
		t.Errorf("Expected Use='list', got '%s'", cmd.Use)
	}

	// Check output flag
	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("Expected --output flag to exist")
	}
}

func TestPipelineStageSetCommand(t *testing.T) {
	deps := DefaultPipelineDeps()
	cmd := newPipelineStageSetCmd(deps)

	if cmd == nil {
		t.Fatal("newPipelineStageSetCmd returned nil")
	}

	// Verify stage arg is required
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("Expected error when stage arg is missing")
	}

	err = cmd.Args(cmd, []string{"triage"})
	if err != nil {
		t.Errorf("Expected no error with stage arg, got: %v", err)
	}

	// Check flags
	for _, flag := range []string{"model", "timeout", "heartbeat", "reason"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("Expected --%s flag to exist", flag)
		}
	}
}

func TestPipelineStageResetCommand(t *testing.T) {
	deps := DefaultPipelineDeps()
	cmd := newPipelineStageResetCmd(deps)

	if cmd == nil {
		t.Fatal("newPipelineStageResetCmd returned nil")
	}

	// Verify stage arg is required
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("Expected error when stage arg is missing")
	}

	err = cmd.Args(cmd, []string{"triage"})
	if err != nil {
		t.Errorf("Expected no error with stage arg, got: %v", err)
	}

	// Check reason flag
	if cmd.Flags().Lookup("reason") == nil {
		t.Error("Expected --reason flag to exist")
	}
}

func TestIsValidPipelineStage(t *testing.T) {
	validStages := []string{"triage", "extract_entities", "extract_assertions", "deep_analyze", "embedding"}
	for _, stage := range validStages {
		if !isValidPipelineStage(stage) {
			t.Errorf("Expected %s to be valid", stage)
		}
	}

	invalidStages := []string{"invalid", "parse", "ingest", ""}
	for _, stage := range invalidStages {
		if isValidPipelineStage(stage) {
			t.Errorf("Expected %s to be invalid", stage)
		}
	}
}

func TestPipelineCommandHasStageCommand(t *testing.T) {
	deps := DefaultPipelineDeps()
	cmd := NewPipelineCommand(deps)

	if cmd == nil {
		t.Fatal("NewPipelineCommand returned nil")
	}

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "stage" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected 'stage' subcommand in pipeline command")
	}
}

func TestPipelineStageSubcommands(t *testing.T) {
	deps := DefaultPipelineDeps()
	cmd := newPipelineStageCmd(deps)

	expectedCommands := []string{"list", "set", "reset"}
	subcommands := cmd.Commands()

	for _, expected := range expectedCommands {
		found := false
		for _, sub := range subcommands {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected subcommand '%s' not found in stage command", expected)
		}
	}
}

func TestPipelineStageAllCommandsHaveHelp(t *testing.T) {
	deps := DefaultPipelineDeps()

	commands := []*cobra.Command{
		newPipelineStageCmd(deps),
		newPipelineStageListCmd(deps),
		newPipelineStageSetCmd(deps),
		newPipelineStageResetCmd(deps),
	}

	for _, cmd := range commands {
		if cmd.Short == "" {
			t.Errorf("Command '%s' has no Short help text", cmd.Use)
		}
		if cmd.Long == "" {
			t.Errorf("Command '%s' has no Long help text", cmd.Use)
		}
	}
}
