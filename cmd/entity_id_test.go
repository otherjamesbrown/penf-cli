package cmd

import (
	"testing"
)

// TestParseEntityID verifies that the ParseEntityID function correctly handles
// both raw int64 IDs and prefixed string IDs ("ent-person-123", "ent-org-456").
func TestParseEntityID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantID    int64
		wantError bool
	}{
		{
			name:      "raw numeric ID",
			input:     "123",
			wantID:    123,
			wantError: false,
		},
		{
			name:      "prefixed person ID",
			input:     "ent-person-123",
			wantID:    123,
			wantError: false,
		},
		{
			name:      "prefixed org ID",
			input:     "ent-org-456",
			wantID:    456,
			wantError: false,
		},
		{
			name:      "prefixed project ID",
			input:     "ent-project-789",
			wantID:    789,
			wantError: false,
		},
		{
			name:      "prefixed topic ID",
			input:     "ent-topic-1000",
			wantID:    1000,
			wantError: false,
		},
		{
			name:      "invalid non-numeric input",
			input:     "invalid",
			wantID:    0,
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			wantID:    0,
			wantError: true,
		},
		{
			name:      "prefixed with non-numeric suffix",
			input:     "ent-person-abc",
			wantID:    0,
			wantError: true,
		},
		{
			name:      "malformed prefix missing ID",
			input:     "ent-person-",
			wantID:    0,
			wantError: true,
		},
		{
			name:      "malformed prefix only two parts",
			input:     "ent-person",
			wantID:    0,
			wantError: true,
		},
		{
			name:      "negative numeric ID",
			input:     "-123",
			wantID:    -123,
			wantError: false,
		},
		{
			name:      "zero ID",
			input:     "0",
			wantID:    0,
			wantError: false,
		},
		{
			name:      "prefixed with zero ID",
			input:     "ent-person-0",
			wantID:    0,
			wantError: false,
		},
		{
			name:      "large ID value",
			input:     "9223372036854775807",
			wantID:    9223372036854775807,
			wantError: false,
		},
		{
			name:      "prefixed with large ID value",
			input:     "ent-person-9223372036854775807",
			wantID:    9223372036854775807,
			wantError: false,
		},
		{
			name:      "invalid prefix format with extra dashes",
			input:     "ent-person-123-extra",
			wantID:    0,
			wantError: true,
		},
		{
			name:      "whitespace only",
			input:     "   ",
			wantID:    0,
			wantError: true,
		},
		{
			name:      "numeric with whitespace",
			input:     " 123 ",
			wantID:    0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEntityID(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ParseEntityID(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
				return
			}
			if !tt.wantError && got != tt.wantID {
				t.Errorf("ParseEntityID(%q) = %v, want %v", tt.input, got, tt.wantID)
			}
		})
	}
}

// TestFormatEntityID verifies that the FormatEntityID function correctly creates
// prefixed string IDs in the format "ent-{type}-{id}".
func TestFormatEntityID(t *testing.T) {
	tests := []struct {
		name       string
		id         int64
		entityType string
		want       string
	}{
		{
			name:       "person entity",
			id:         123,
			entityType: "person",
			want:       "ent-person-123",
		},
		{
			name:       "org entity",
			id:         456,
			entityType: "org",
			want:       "ent-org-456",
		},
		{
			name:       "project entity",
			id:         789,
			entityType: "project",
			want:       "ent-project-789",
		},
		{
			name:       "topic entity",
			id:         1000,
			entityType: "topic",
			want:       "ent-topic-1000",
		},
		{
			name:       "location entity",
			id:         2000,
			entityType: "location",
			want:       "ent-location-2000",
		},
		{
			name:       "zero ID",
			id:         0,
			entityType: "person",
			want:       "ent-person-0",
		},
		{
			name:       "negative ID",
			id:         -123,
			entityType: "person",
			want:       "ent-person--123",
		},
		{
			name:       "large ID value",
			id:         9223372036854775807,
			entityType: "person",
			want:       "ent-person-9223372036854775807",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatEntityID(tt.id, tt.entityType)
			if got != tt.want {
				t.Errorf("FormatEntityID(%v, %q) = %q, want %q", tt.id, tt.entityType, got, tt.want)
			}
		})
	}
}

// TestParseAndFormatRoundTrip verifies that parsing and formatting are inverses.
func TestParseAndFormatRoundTrip(t *testing.T) {
	tests := []struct {
		id         int64
		entityType string
	}{
		{123, "person"},
		{456, "org"},
		{789, "project"},
		{0, "topic"},
		{9223372036854775807, "location"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			// Format then parse
			formatted := FormatEntityID(tt.id, tt.entityType)
			parsed, err := ParseEntityID(formatted)
			if err != nil {
				t.Errorf("ParseEntityID(FormatEntityID(%v, %q)) failed: %v", tt.id, tt.entityType, err)
				return
			}
			if parsed != tt.id {
				t.Errorf("Round trip failed: started with %v, got %v", tt.id, parsed)
			}

			// Also verify parsing raw numeric string
			rawNumeric := FormatEntityID(tt.id, "")
			parsedRaw, err := ParseEntityID(rawNumeric)
			if err != nil && tt.id >= 0 {
				// Negative IDs format as "-123" which may be handled differently
				t.Errorf("ParseEntityID(raw %q) failed: %v", rawNumeric, err)
				return
			}
			if err == nil && parsedRaw != tt.id {
				t.Errorf("Raw round trip failed: started with %v, got %v", tt.id, parsedRaw)
			}
		})
	}
}
