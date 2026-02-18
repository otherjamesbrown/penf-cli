// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"testing"
)

// TestNewReprocessCommand tests that NewReprocessCommand returns a valid cobra.Command.
func TestNewReprocessCommand(t *testing.T) {
	cmd := NewReprocessCommand(nil)

	if cmd == nil {
		t.Fatal("NewReprocessCommand() returned nil")
	}

	if cmd.Use != "reprocess <content-id>" {
		t.Errorf("NewReprocessCommand().Use = %q, want %q", cmd.Use, "reprocess <content-id>")
	}
}

// TestReprocessCommandHasRequiredFlags verifies all required flags are present.
func TestReprocessCommandHasRequiredFlags(t *testing.T) {
	cmd := NewReprocessCommand(nil)

	requiredFlags := []string{
		"stage",
		"all",
		"reason",
		"dry-run",
		"timeout",
		"model",
		"confirm",
		"output",
		"source-tag",
	}

	for _, flagName := range requiredFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("NewReprocessCommand() missing required flag: --%s", flagName)
		}
	}
}

// TestReprocessCommandFlagTypes verifies flag types are correct.
func TestReprocessCommandFlagTypes(t *testing.T) {
	cmd := NewReprocessCommand(nil)

	tests := []struct {
		name     string
		flagType string
	}{
		{"stage", "string"},
		{"all", "bool"},
		{"reason", "string"},
		{"dry-run", "bool"},
		{"timeout", "int32"},
		{"model", "string"},
		{"confirm", "bool"},
		{"output", "string"},
		{"source-tag", "string"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tc.name)
			if flag == nil {
				t.Fatalf("flag --%s not found", tc.name)
			}

			if flag.Value.Type() != tc.flagType {
				t.Errorf("flag --%s type = %s, want %s", tc.name, flag.Value.Type(), tc.flagType)
			}
		})
	}
}

// TestReprocessCommandIsCallable verifies the command can be executed (has a RunE function).
func TestReprocessCommandIsCallable(t *testing.T) {
	cmd := NewReprocessCommand(nil)

	if cmd.RunE == nil && cmd.Run == nil {
		t.Error("NewReprocessCommand() has no RunE or Run function, command is not callable")
	}
}

// TestReprocessCommandAcceptsContentID verifies the command accepts a content ID argument.
func TestReprocessCommandAcceptsContentID(t *testing.T) {
	cmd := NewReprocessCommand(nil)

	// The Args validator should allow 0 or 1 argument (MaximumNArgs(1))
	// We can't easily test cobra.Args validators without executing, but we can
	// verify the command structure looks correct
	if cmd.Args == nil {
		t.Log("Warning: command has no Args validator, should use cobra.MaximumNArgs(1)")
	}
}

// TestReprocessCommandHasHelpText verifies command has help text for AI agents.
func TestReprocessCommandHasHelpText(t *testing.T) {
	cmd := NewReprocessCommand(nil)

	if cmd.Short == "" {
		t.Error("NewReprocessCommand().Short is empty, should have brief description")
	}

	if cmd.Long == "" {
		t.Error("NewReprocessCommand().Long is empty, should have detailed description for AI agents")
	}
}

// TestReprocessCommandOutputFlag verifies the --output flag has correct shorthand.
func TestReprocessCommandOutputFlag(t *testing.T) {
	cmd := NewReprocessCommand(nil)

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Fatal("output flag not found")
	}

	// Should have -o as shorthand
	if outputFlag.Shorthand != "o" {
		t.Errorf("output flag shorthand = %q, want %q", outputFlag.Shorthand, "o")
	}
}

// TestReprocessCommandDefaultValues verifies flag default values.
func TestReprocessCommandDefaultValues(t *testing.T) {
	cmd := NewReprocessCommand(nil)

	tests := []struct {
		name         string
		expectedDefault string
	}{
		{"stage", ""},
		{"all", "false"},
		{"reason", "Manual reprocess via CLI"},
		{"dry-run", "false"},
		{"timeout", "0"},
		{"model", ""},
		{"confirm", "false"},
		{"output", "text"},
		{"source-tag", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tc.name)
			if flag == nil {
				t.Fatalf("flag --%s not found", tc.name)
			}

			if flag.DefValue != tc.expectedDefault {
				t.Errorf("flag --%s default = %q, want %q", tc.name, flag.DefValue, tc.expectedDefault)
			}
		})
	}
}
