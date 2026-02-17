// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestNewConversationCommand tests that the conversation command is created correctly.
func TestNewConversationCommand(t *testing.T) {
	deps := DefaultConversationDeps()
	cmd := NewConversationCommand(deps)

	if cmd == nil {
		t.Fatal("NewConversationCommand returned nil")
	}

	if cmd.Use != "conversation" {
		t.Errorf("Use = %v, want 'conversation'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Verify subcommands are registered by name
	found := map[string]bool{"list": false, "show": false}
	for _, sub := range cmd.Commands() {
		found[sub.Name()] = true
	}
	if !found["list"] {
		t.Error("list subcommand should be registered")
	}
	if !found["show"] {
		t.Error("show subcommand should be registered")
	}
}

// TestNewConversationListCommand tests the conversation list command structure.
func TestNewConversationListCommand(t *testing.T) {
	deps := DefaultConversationDeps()
	cmd := newConversationListCommand(deps)

	if cmd == nil {
		t.Fatal("newConversationListCommand returned nil")
	}

	if cmd.Use != "list" {
		t.Errorf("Use = %v, want 'list'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Check that flags are registered
	limitFlag := cmd.Flags().Lookup("limit")
	if limitFlag == nil {
		t.Error("--limit flag should be registered")
	}
	if limitFlag.DefValue != "20" {
		t.Errorf("--limit default = %v, want '20'", limitFlag.DefValue)
	}

	offsetFlag := cmd.Flags().Lookup("offset")
	if offsetFlag == nil {
		t.Error("--offset flag should be registered")
	}
	if offsetFlag.DefValue != "0" {
		t.Errorf("--offset default = %v, want '0'", offsetFlag.DefValue)
	}

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("--output flag should be registered")
	}

	// Verify shorthand for output flag
	if cmd.Flags().ShorthandLookup("o") == nil {
		t.Error("-o shorthand should be registered for output flag")
	}

	// Test that command accepts no positional arguments
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("Command should accept zero arguments: %v", err)
	}

	if err := cmd.Args(cmd, []string{"extra"}); err == nil {
		t.Error("Command should not accept positional arguments")
	}
}

// TestNewConversationShowCommand tests the conversation show command structure.
func TestNewConversationShowCommand(t *testing.T) {
	deps := DefaultConversationDeps()
	cmd := newConversationShowCommand(deps)

	if cmd == nil {
		t.Fatal("newConversationShowCommand returned nil")
	}

	if cmd.Use != "show <conversation-id>" {
		t.Errorf("Use = %v, want 'show <conversation-id>'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Check that output flag is registered
	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("--output flag should be registered")
	}

	// Verify shorthand for output flag
	if cmd.Flags().ShorthandLookup("o") == nil {
		t.Error("-o shorthand should be registered for output flag")
	}

	// Test that command requires exactly one argument
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("Command should require an argument")
	}

	if err := cmd.Args(cmd, []string{"123"}); err != nil {
		t.Errorf("Command should accept one argument: %v", err)
	}

	if err := cmd.Args(cmd, []string{"123", "extra"}); err == nil {
		t.Error("Command should not accept two arguments")
	}
}

// TestConversationListCommandFlags tests that all expected flags have correct types and defaults.
func TestConversationListCommandFlags(t *testing.T) {
	deps := DefaultConversationDeps()
	cmd := newConversationListCommand(deps)

	tests := []struct {
		name         string
		flagType     string
		defaultValue string
	}{
		{"limit", "int32", "20"},
		{"offset", "int32", "0"},
		{"output", "string", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tc.name)
			if flag == nil {
				t.Fatalf("--%s flag should be registered", tc.name)
			}

			if flag.Value.Type() != tc.flagType {
				t.Errorf("--%s type = %v, want %v", tc.name, flag.Value.Type(), tc.flagType)
			}

			if flag.DefValue != tc.defaultValue {
				t.Errorf("--%s default = %v, want %v", tc.name, flag.DefValue, tc.defaultValue)
			}
		})
	}
}

// TestConversationShowCommandFlags tests that the show command has correct flags.
func TestConversationShowCommandFlags(t *testing.T) {
	deps := DefaultConversationDeps()
	cmd := newConversationShowCommand(deps)

	flag := cmd.Flags().Lookup("output")
	if flag == nil {
		t.Fatal("--output flag should be registered")
	}

	if flag.Value.Type() != "string" {
		t.Errorf("--output type = %v, want 'string'", flag.Value.Type())
	}

	if flag.DefValue != "" {
		t.Errorf("--output default = %v, want ''", flag.DefValue)
	}
}

// TestConversationDepsInterface tests that ConversationCommandDeps has the expected structure.
func TestConversationDepsInterface(t *testing.T) {
	deps := DefaultConversationDeps()

	if deps == nil {
		t.Fatal("DefaultConversationDeps returned nil")
	}

	if deps.LoadConfig == nil {
		t.Error("LoadConfig function should be set in default deps")
	}

	if deps.Config != nil {
		t.Error("Config should be nil until command execution")
	}
}

// TestConversationListCommandHelp tests that help text is accessible.
func TestConversationListCommandHelp(t *testing.T) {
	deps := DefaultConversationDeps()
	cmd := newConversationListCommand(deps)

	if cmd.Long == "" {
		t.Error("Long description should not be empty for list command")
	}
}

// TestConversationShowCommandHelp tests that help text is accessible.
func TestConversationShowCommandHelp(t *testing.T) {
	deps := DefaultConversationDeps()
	cmd := newConversationShowCommand(deps)

	if cmd.Long == "" {
		t.Error("Long description should not be empty for show command")
	}
}

// TestConversationCommandHasRunE tests that commands have RunE functions defined.
func TestConversationCommandHasRunE(t *testing.T) {
	deps := DefaultConversationDeps()

	tests := []struct {
		name    string
		cmdFunc func(*ConversationCommandDeps) *cobra.Command
	}{
		{"list", newConversationListCommand},
		{"show", newConversationShowCommand},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmdFunc(deps)
			if cmd.RunE == nil {
				t.Errorf("%s command should have RunE function defined", tc.name)
			}
		})
	}
}
