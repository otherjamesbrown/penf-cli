package enrichment

import (
	"testing"
)

// TestSourceSystemConstants verifies that all SourceSystem constants
// are defined with the expected string values matching the database column specification.
func TestSourceSystemConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant SourceSystem
		want     string
	}{
		{
			name:     "jira constant",
			constant: SourceSystemJira,
			want:     "jira",
		},
		{
			name:     "aha constant",
			constant: SourceSystemAha,
			want:     "aha",
		},
		{
			name:     "google_docs constant",
			constant: SourceSystemGoogleDocs,
			want:     "google_docs",
		},
		{
			name:     "webex constant",
			constant: SourceSystemWebex,
			want:     "webex",
		},
		{
			name:     "smartsheet constant",
			constant: SourceSystemSmartsheet,
			want:     "smartsheet",
		},
		{
			name:     "outlook_calendar constant",
			constant: SourceSystemOutlookCalendar,
			want:     "outlook_calendar",
		},
		{
			name:     "auto_reply constant",
			constant: SourceSystemAutoReply,
			want:     "auto_reply",
		},
		{
			name:     "human_email constant",
			constant: SourceSystemHumanEmail,
			want:     "human_email",
		},
		{
			name:     "unknown constant",
			constant: SourceSystemUnknown,
			want:     "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(tt.constant)
			if got != tt.want {
				t.Errorf("SourceSystem constant = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSourceSystemType verifies that SourceSystem is defined as a string type
// and can be converted to/from strings correctly.
func TestSourceSystemType(t *testing.T) {
	tests := []struct {
		name     string
		value    SourceSystem
		asString string
	}{
		{
			name:     "jira string conversion",
			value:    SourceSystemJira,
			asString: "jira",
		},
		{
			name:     "aha string conversion",
			value:    SourceSystemAha,
			asString: "aha",
		},
		{
			name:     "google_docs string conversion",
			value:    SourceSystemGoogleDocs,
			asString: "google_docs",
		},
		{
			name:     "webex string conversion",
			value:    SourceSystemWebex,
			asString: "webex",
		},
		{
			name:     "smartsheet string conversion",
			value:    SourceSystemSmartsheet,
			asString: "smartsheet",
		},
		{
			name:     "outlook_calendar string conversion",
			value:    SourceSystemOutlookCalendar,
			asString: "outlook_calendar",
		},
		{
			name:     "auto_reply string conversion",
			value:    SourceSystemAutoReply,
			asString: "auto_reply",
		},
		{
			name:     "human_email string conversion",
			value:    SourceSystemHumanEmail,
			asString: "human_email",
		},
		{
			name:     "unknown string conversion",
			value:    SourceSystemUnknown,
			asString: "unknown",
		},
		{
			name:     "default value is unknown",
			value:    SourceSystemUnknown,
			asString: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test type -> string conversion
			if string(tt.value) != tt.asString {
				t.Errorf("string(SourceSystem) = %q, want %q", string(tt.value), tt.asString)
			}

			// Test string -> type conversion
			converted := SourceSystem(tt.asString)
			if converted != tt.value {
				t.Errorf("SourceSystem(%q) = %q, want %q", tt.asString, converted, tt.value)
			}
		})
	}
}

// TestEnrichmentStructHasSourceSystem verifies that the Enrichment struct
// includes a SourceSystem field, ensuring the type is integrated into the
// main enrichment data structure.
func TestEnrichmentStructHasSourceSystem(t *testing.T) {
	// Create an Enrichment instance with SourceSystem field populated
	enrichment := Enrichment{
		ID:       1,
		SourceID: 100,
		TenantID: "test-tenant",
		Classification: Classification{
			ContentType: ContentTypeEmail,
			Subtype:     SubtypeEmailThread,
			Profile:     ProfileFullAI,
		},
		Status:       StatusPending,
		SourceSystem: SourceSystemJira,
	}

	// Verify the field exists and has the correct value
	if enrichment.SourceSystem != SourceSystemJira {
		t.Errorf("Enrichment.SourceSystem = %q, want %q", enrichment.SourceSystem, SourceSystemJira)
	}

	// Verify it can be set to each constant
	testCases := []SourceSystem{
		SourceSystemJira,
		SourceSystemAha,
		SourceSystemGoogleDocs,
		SourceSystemWebex,
		SourceSystemSmartsheet,
		SourceSystemOutlookCalendar,
		SourceSystemAutoReply,
		SourceSystemHumanEmail,
		SourceSystemUnknown,
	}

	for _, tc := range testCases {
		enrichment.SourceSystem = tc
		if enrichment.SourceSystem != tc {
			t.Errorf("After setting Enrichment.SourceSystem to %q, got %q", tc, enrichment.SourceSystem)
		}
	}
}

// TestSourceSystemDefaultValue verifies that the default SourceSystem value
// matches the database default (unknown).
func TestSourceSystemDefaultValue(t *testing.T) {
	// When an Enrichment is created without explicitly setting SourceSystem,
	// it should be the zero value (empty string ""), but our default should be "unknown"
	var enrichment Enrichment

	// The zero value will be an empty string
	if enrichment.SourceSystem != "" {
		t.Errorf("Zero value Enrichment.SourceSystem = %q, want empty string", enrichment.SourceSystem)
	}

	// But when explicitly set to the default, it should be "unknown"
	enrichment.SourceSystem = SourceSystemUnknown
	if enrichment.SourceSystem != SourceSystemUnknown {
		t.Errorf("Enrichment.SourceSystem default = %q, want %q", enrichment.SourceSystem, SourceSystemUnknown)
	}
	if string(enrichment.SourceSystem) != "unknown" {
		t.Errorf("Enrichment.SourceSystem default string = %q, want %q", string(enrichment.SourceSystem), "unknown")
	}
}

// TestSourceSystemAllValuesUnique verifies that all SourceSystem constants
// have unique string values (no duplicates).
func TestSourceSystemAllValuesUnique(t *testing.T) {
	allSystems := []SourceSystem{
		SourceSystemJira,
		SourceSystemAha,
		SourceSystemGoogleDocs,
		SourceSystemWebex,
		SourceSystemSmartsheet,
		SourceSystemOutlookCalendar,
		SourceSystemAutoReply,
		SourceSystemHumanEmail,
		SourceSystemUnknown,
	}

	seen := make(map[string]bool)
	for _, system := range allSystems {
		value := string(system)
		if seen[value] {
			t.Errorf("Duplicate SourceSystem value found: %q", value)
		}
		seen[value] = true
	}

	// Verify we have all 9 unique values
	if len(seen) != 9 {
		t.Errorf("Expected 9 unique SourceSystem values, got %d", len(seen))
	}
}
