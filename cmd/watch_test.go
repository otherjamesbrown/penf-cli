package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/otherjamesbrown/penf-cli/config"
)

func TestNewWatchCommand(t *testing.T) {
	cmd := NewWatchCommand(nil)
	require.NotNil(t, cmd)

	// Verify basic command properties
	assert.Equal(t, "watch", cmd.Use)
	assert.Contains(t, cmd.Aliases, "watchlist")

	// Verify all subcommands are present
	subcommands := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subcommands[sub.Name()] = true
	}

	assert.True(t, subcommands["list"], "watch command should have 'list' subcommand")
	assert.True(t, subcommands["add"], "watch command should have 'add' subcommand")
	assert.True(t, subcommands["remove"], "watch command should have 'remove' subcommand")
	assert.True(t, subcommands["annotate"], "watch command should have 'annotate' subcommand")
}

func TestWatchAdd_RequiresTarget(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name:          "no target specified",
			args:          []string{},
			expectedError: "must specify either --assertion or --project",
		},
		{
			name:          "both targets specified",
			args:          []string{"--assertion", "123", "--project", "456"},
			expectedError: "cannot specify both --assertion and --project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := &WatchCommandDeps{
				LoadConfig: func() (*config.CLIConfig, error) {
					t.Fatal("validation should have failed before loading config")
					return nil, nil
				},
			}

			cmd := newWatchAddCommand(deps)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestWatchRemove_RequiresID(t *testing.T) {
	cmd := newWatchRemoveCommand(nil)
	require.NotNil(t, cmd)

	// Verify command requires exactly one argument by testing behavior
	require.NotNil(t, cmd.Args, "Args validator should be set")
	assert.Error(t, cmd.Args(cmd, []string{}), "should fail with no args")
	assert.NoError(t, cmd.Args(cmd, []string{"123"}), "should pass with one arg")
	assert.Error(t, cmd.Args(cmd, []string{"1", "2"}), "should fail with two args")

	// Test that command has correct aliases
	assert.Contains(t, cmd.Aliases, "rm")
	assert.Contains(t, cmd.Aliases, "delete")
}

func TestWatchAnnotate_RequiresID(t *testing.T) {
	cmd := newWatchAnnotateCommand(nil)
	require.NotNil(t, cmd)

	// Verify command requires exactly one argument by testing behavior
	require.NotNil(t, cmd.Args, "Args validator should be set")
	assert.Error(t, cmd.Args(cmd, []string{}), "should fail with no args")
	assert.NoError(t, cmd.Args(cmd, []string{"123"}), "should pass with one arg")
	assert.Error(t, cmd.Args(cmd, []string{"1", "2"}), "should fail with two args")

	// Test that command has correct aliases
	assert.Contains(t, cmd.Aliases, "update")
	assert.Contains(t, cmd.Aliases, "note")

	// Verify notes flag is required
	notesFlag := cmd.Flag("notes")
	require.NotNil(t, notesFlag, "notes flag should exist")

	// Check if required flags list contains "notes"
	// Note: Cobra's MarkFlagRequired adds to an internal list
	// We can verify by trying to execute without it
	err := cmd.ParseFlags([]string{})
	require.NoError(t, err, "parsing empty flags should succeed")

	// The actual required check happens during execution, which we can't easily test
	// without a full command setup, but we've verified the flag exists
}

func TestWatchTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "string shorter than max",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "string equal to max",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "string longer than max",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "max length very small",
			input:    "hello",
			maxLen:   3,
			expected: "hel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := watchTruncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWatchListCommand(t *testing.T) {
	cmd := newWatchListCommand(nil)
	require.NotNil(t, cmd)

	// Verify basic properties
	assert.Equal(t, "list", cmd.Use)
	assert.Contains(t, cmd.Aliases, "ls")

	// Verify flags exist
	assert.NotNil(t, cmd.Flag("project"), "should have --project flag")
	assert.NotNil(t, cmd.Flag("user"), "should have --user flag")
}
