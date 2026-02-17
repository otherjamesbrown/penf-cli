package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/otherjamesbrown/penf-cli/config"
)

func TestNewTrustCommand(t *testing.T) {
	cmd := NewTrustCommand(nil)

	require.NotNil(t, cmd)
	assert.Equal(t, "trust", cmd.Use)

	// Verify subcommands exist
	subcommands := cmd.Commands()
	require.Len(t, subcommands, 2)

	var hasSet, hasClear bool
	for _, subcmd := range subcommands {
		if subcmd.Use == "set <person_id>" {
			hasSet = true
		}
		if subcmd.Use == "clear <person_id>" {
			hasClear = true
		}
	}
	assert.True(t, hasSet, "trust command should have 'set' subcommand")
	assert.True(t, hasClear, "trust command should have 'clear' subcommand")
}

func TestTrustSet_ValidatesLevel(t *testing.T) {
	tests := []struct {
		name      string
		level     int32
		wantError bool
	}{
		{
			name:      "level 0 is valid",
			level:     0,
			wantError: false,
		},
		{
			name:      "level 5 is valid",
			level:     5,
			wantError: false,
		},
		{
			name:      "level 3 is valid",
			level:     3,
			wantError: false,
		},
		{
			name:      "level -1 is invalid",
			level:     -1,
			wantError: true,
		},
		{
			name:      "level 6 is invalid",
			level:     6,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the global flag
			trustLevel = tt.level

			// Create mock deps that will fail at config load (we just want to test validation)
			deps := &TrustCommandDeps{
				LoadConfig: func() (*config.CLIConfig, error) {
					// Return a minimal config to get past config loading
					return &config.CLIConfig{
						ServerAddress: "localhost:50051",
					}, nil
				},
			}

			err := runTrustSet(nil, deps, "123")

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "trust level must be 0-5")
			} else {
				// We expect a different error (connection failure) since we're not testing the full flow
				// The important thing is we don't get the validation error
				if err != nil {
					assert.NotContains(t, err.Error(), "trust level must be 0-5")
				}
			}

			// Reset global flag
			trustLevel = 0
		})
	}
}

func TestNewSeniorityCommand(t *testing.T) {
	cmd := NewSeniorityCommand(nil)

	require.NotNil(t, cmd)
	assert.Equal(t, "seniority", cmd.Use)

	// Verify subcommands exist
	subcommands := cmd.Commands()
	require.Len(t, subcommands, 2)

	var hasSet, hasClear bool
	for _, subcmd := range subcommands {
		if subcmd.Use == "set <person_id>" {
			hasSet = true
		}
		if subcmd.Use == "clear <person_id>" {
			hasClear = true
		}
	}
	assert.True(t, hasSet, "seniority command should have 'set' subcommand")
	assert.True(t, hasClear, "seniority command should have 'clear' subcommand")
}

func TestSenioritySet_ValidatesTier(t *testing.T) {
	tests := []struct {
		name      string
		tier      int32
		wantError bool
	}{
		{
			name:      "tier 1 is valid",
			tier:      1,
			wantError: false,
		},
		{
			name:      "tier 7 is valid",
			tier:      7,
			wantError: false,
		},
		{
			name:      "tier 4 is valid",
			tier:      4,
			wantError: false,
		},
		{
			name:      "tier 0 is invalid",
			tier:      0,
			wantError: true,
		},
		{
			name:      "tier 8 is invalid",
			tier:      8,
			wantError: true,
		},
		{
			name:      "tier -1 is invalid",
			tier:      -1,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the global flag
			seniorityTier = tt.tier

			// Create mock deps that will fail at config load (we just want to test validation)
			deps := &SeniorityCommandDeps{
				LoadConfig: func() (*config.CLIConfig, error) {
					// Return a minimal config to get past config loading
					return &config.CLIConfig{
						ServerAddress: "localhost:50051",
					}, nil
				},
			}

			err := runSenioritySet(nil, deps, "123")

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "seniority tier must be 1-7")
			} else {
				// We expect a different error (connection failure) since we're not testing the full flow
				// The important thing is we don't get the validation error
				if err != nil {
					assert.NotContains(t, err.Error(), "seniority tier must be 1-7")
				}
			}

			// Reset global flag
			seniorityTier = 0
		})
	}
}
