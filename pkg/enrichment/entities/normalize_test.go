package entities

import (
	"testing"
)

func TestNormalizeDisplayName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Eskelsen, Rick", "Rick Eskelsen"},
		{"Brown, James", "James Brown"},
		{"  James  Brown  ", "James Brown"},
		{`"John Doe"`, "John Doe"},
		{"'Jane Smith'", "Jane Smith"},
		{"john doe", "John Doe"},
		{"JAMES BROWN", "James Brown"},
		{"Smith, Dr. John", "Dr. John Smith"},
		{"", ""},
		{"OneWord", "Oneword"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeDisplayName(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeDisplayName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		email string
		want  string
	}{
		{"user@example.com", "example.com"},
		{"USER@EXAMPLE.COM", "example.com"},
		{"john.doe@company.co.uk", "company.co.uk"},
		{"invalid", ""},
		{"@nodomain", ""},
		{"noat", ""},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			got := ExtractDomain(tt.email)
			if got != tt.want {
				t.Errorf("ExtractDomain(%q) = %q, want %q", tt.email, got, tt.want)
			}
		})
	}
}

func TestDetectAccountType(t *testing.T) {
	tests := []struct {
		email       string
		displayName string
		want        AccountType
	}{
		// Bot patterns
		{"noreply@company.com", "", AccountTypeBot},
		{"no-reply@company.com", "", AccountTypeBot},
		{"jira@company.com", "", AccountTypeBot},
		{"jenkins@company.com", "", AccountTypeBot},
		{"automation@company.com", "", AccountTypeBot},

		// Distribution lists
		{"team-engineering@company.com", "", AccountTypeDistribution},
		{"all-staff@company.com", "", AccountTypeDistribution},
		{"group-sales@company.com", "", AccountTypeDistribution},
		{"dl-ttmtc-SteerCo@akamai.com", "", AccountTypeDistribution},

		// Role accounts
		{"support@company.com", "", AccountTypeRole},
		{"sales@company.com", "", AccountTypeRole},
		{"hr@company.com", "", AccountTypeRole},
		{"facilitator@company.com", "", AccountTypeRole},
		{"prb-facilitator@akamai.com", "", AccountTypeRole},

		// External services
		{"comments-noreply@docs.google.com", "", AccountTypeExternalService},
		{"notification@slack.com", "", AccountTypeExternalService},
		{"updates@mailer.aha.io", "", AccountTypeExternalService},

		// Service accounts (bots with specific prefixes)
		{"gsd-jira@akamai.com", "", AccountTypeBot},

		// Regular person
		{"john.doe@company.com", "John Doe", AccountTypePerson},
		{"jdoe@company.com", "Jane Doe", AccountTypePerson},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			got := DetectAccountType(tt.email, tt.displayName)
			if got != tt.want {
				t.Errorf("DetectAccountType(%q, %q) = %q, want %q", tt.email, tt.displayName, got, tt.want)
			}
		})
	}
}

func TestNameSimilarity(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		min  float64 // minimum expected similarity
		max  float64 // maximum expected similarity (0 if no upper bound)
	}{
		// Exact match
		{"Rick Eskelsen", "Rick Eskelsen", 1.0, 0.0},

		// Same after normalization
		{"Eskelsen, Rick", "Rick Eskelsen", 1.0, 0.0},

		// Subset
		{"Rick", "Rick Eskelsen", 0.85, 0.0},
		{"Eskelsen", "Rick Eskelsen", 0.85, 0.0},

		// Contains
		{"Rick E", "Rick Eskelsen", 0.8, 0.0},

		// Different
		{"John Doe", "Jane Smith", 0.0, 0.0},

		// Empty
		{"", "Rick", 0.0, 0.0},
		{"Rick", "", 0.0, 0.0},

		// False positive prevention: same first name, different last name
		// These should score LOW (< 0.5) to prevent false duplicates
		{"Patrick Brisbane", "Patrick Bussmann", 0.0, 0.5},
		{"James Brown", "James DeMent", 0.0, 0.5},
		{"Sean Butler", "Sean Li", 0.0, 0.5},

		// True duplicates: same first + last name
		// These should score HIGH (> 0.9) to detect real duplicates
		{"John Smith", "Smith, John", 0.9, 0.0},
		{"Jane Doe", "Jane Doe", 1.0, 0.0},

		// Close variant (typo/spelling): should still score high
		{"Jon Smith", "John Smith", 0.8, 0.0},

		// BUG pf-96c91a: Single-character substring matches should NOT score 0.9
		// The contains() check at line 227 returns 0.9 for ANY substring match,
		// even single characters like "K" in "Mike" or "a" in "Sarah"
		{"K", "Mike", 0.0, 0.3},           // Single char should score very low
		{"a", "Sarah", 0.0, 0.3},          // Single char should score very low
		{"M", "Mike Johnson", 0.0, 0.3},   // Single char should score very low
		{"e", "Mike", 0.0, 0.3},           // Single char at end should score very low

		// Short substring matches should score lower than full matches
		{"Mi", "Mike", 0.0, 0.7},          // 2-char substring should be moderate, not 0.9
		{"ike", "Mike", 0.0, 0.7},         // 3-char substring should be moderate, not 0.9
		{"Mik", "Mike Johnson", 0.0, 0.7}, // Short substring should be moderate, not 0.9

		// First-name-only mentions (common in emails: "Thanks, Mike")
		// Without minimum length validation, "Mike" matches "Mike Johnson" at 0.85
		// but could also fuzzy-match other Mikes without email context
		{"Mike", "Mike Johnson", 0.85, 0.0},    // This is intended behavior (partial match)
		{"Mike", "Mike Smith", 0.85, 0.0},      // This is intended behavior (partial match)
		// The bug is that "K" also gets 0.9 via contains(), skipping proper similarity scoring
	}

	for _, tt := range tests {
		t.Run(tt.a+" vs "+tt.b, func(t *testing.T) {
			got := NameSimilarity(tt.a, tt.b)
			if got < tt.min {
				t.Errorf("NameSimilarity(%q, %q) = %v, want >= %v", tt.a, tt.b, got, tt.min)
			}
			if tt.max > 0 && got > tt.max {
				t.Errorf("NameSimilarity(%q, %q) = %v, want <= %v", tt.a, tt.b, got, tt.max)
			}
		})
	}
}

func TestIsInternalDomain(t *testing.T) {
	internalDomains := []string{"company.com", "corp.company.com"}

	tests := []struct {
		email string
		want  bool
	}{
		{"user@company.com", true},
		{"user@corp.company.com", true},
		{"user@sub.company.com", true},
		{"user@external.com", false},
		{"user@company.org", false},
		{"user@companycom", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			got := IsInternalDomain(tt.email, internalDomains)
			if got != tt.want {
				t.Errorf("IsInternalDomain(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{"user@example.com", true},
		{"user.name@example.com", true},
		{"user+tag@example.com", true},
		{"user@sub.example.com", true},
		{"invalid", false},
		{"@example.com", false},
		{"user@", false},
		{"user@nodot", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			got := IsValidEmail(tt.email)
			if got != tt.want {
				t.Errorf("IsValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "adc", 1},
		{"abc", "abcd", 1},
		{"kitten", "sitting", 3},
	}

	for _, tt := range tests {
		t.Run(tt.a+" vs "+tt.b, func(t *testing.T) {
			got := levenshteinDistance(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestDetectAccountTypeWithPatterns tests that DetectAccountType can use additional patterns
// beyond the hardcoded defaults. This verifies custom tenant-specific patterns work correctly.
func TestDetectAccountTypeWithPatterns(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		displayName string
		patterns    *AccountTypePatterns
		want        AccountType
	}{
		{
			name:        "custom bot pattern identifies custom bot",
			email:       "custom-bot-alerts@company.com",
			displayName: "",
			patterns: &AccountTypePatterns{
				BotPatterns: []string{"custom-bot-"},
			},
			want: AccountTypeBot,
		},
		{
			name:        "custom distribution pattern identifies custom distribution list",
			email:       "dept-all-engineering@company.com",
			displayName: "",
			patterns: &AccountTypePatterns{
				DistributionPatterns: []string{"dept-all-"},
			},
			want: AccountTypeDistribution,
		},
		{
			name:        "custom role pattern identifies custom role account",
			email:       "facilities@company.com",
			displayName: "",
			patterns: &AccountTypePatterns{
				RolePatterns: []string{"facilities"},
			},
			want: AccountTypeRole,
		},
		{
			name:        "custom external service domain identifies external service",
			email:       "bot@custom-service.example.com",
			displayName: "",
			patterns: &AccountTypePatterns{
				ExternalDomains: []string{"custom-service.example.com"},
			},
			want: AccountTypeExternalService,
		},
		{
			name:        "hardcoded patterns still work with custom patterns",
			email:       "noreply@company.com",
			displayName: "",
			patterns: &AccountTypePatterns{
				BotPatterns: []string{"custom-bot-"},
			},
			want: AccountTypeBot,
		},
		{
			name:        "empty custom patterns falls back to defaults only",
			email:       "jira@company.com",
			displayName: "",
			patterns:    &AccountTypePatterns{},
			want:        AccountTypeBot,
		},
		{
			name:        "nil patterns uses defaults",
			email:       "support@company.com",
			displayName: "",
			patterns:    nil,
			want:        AccountTypeRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectAccountTypeWithPatterns(tt.email, tt.displayName, tt.patterns)
			if got != tt.want {
				t.Errorf("DetectAccountTypeWithPatterns(%q, %q, patterns) = %q, want %q",
					tt.email, tt.displayName, got, tt.want)
			}
		})
	}
}

// TestDetectAccountTypeWithPatternsFromTenantConfig tests that patterns from TenantConfig
// are correctly applied to account type detection.
func TestDetectAccountTypeWithPatternsFromTenantConfig(t *testing.T) {
	// Simulate TenantConfig with custom patterns
	tenantBotPatterns := []string{"acme-bot-", "widget-automation-"}
	tenantDistPatterns := []string{"all-acme-", "acme-team-"}
	tenantRolePatterns := []string{"acme-support", "acme-helpdesk"}

	tests := []struct {
		name        string
		email       string
		displayName string
		want        AccountType
	}{
		{
			name:        "tenant-specific bot pattern",
			email:       "acme-bot-scheduler@company.com",
			displayName: "",
			want:        AccountTypeBot,
		},
		{
			name:        "tenant-specific distribution pattern",
			email:       "all-acme-engineers@company.com",
			displayName: "",
			want:        AccountTypeDistribution,
		},
		{
			name:        "tenant-specific role pattern",
			email:       "acme-support@company.com",
			displayName: "",
			want:        AccountTypeRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := &AccountTypePatterns{
				BotPatterns:          tenantBotPatterns,
				DistributionPatterns: tenantDistPatterns,
				RolePatterns:         tenantRolePatterns,
			}
			got := DetectAccountTypeWithPatterns(tt.email, tt.displayName, patterns)
			if got != tt.want {
				t.Errorf("DetectAccountTypeWithPatterns(%q, %q, tenantPatterns) = %q, want %q",
					tt.email, tt.displayName, got, tt.want)
			}
		})
	}
}

// TestDeriveNameFromEmail_CorporateEmailPrefixes is a REPRODUCTION TEST for bug pf-2c2ab0.
// It tests that DeriveNameFromEmail handles single-word corporate email prefixes correctly.
//
// BUG: The current implementation incorrectly splits single-word email prefixes like "uzeeshan",
// "knaidu", "ppandya" into "U Zeeshan", "K Naidu", "P Pandya" by treating the first character
// as an initial. This is incorrect for corporate emails where the entire prefix is a username
// (often lowercase firstname+lastname concatenated without separator).
//
// ROOT CAUSE: Lines 416-425 in normalize.go split any single-word prefix ≤8 chars at position 1,
// treating first char as initial. This heuristic fails for corporate usernames.
//
// EVIDENCE: Source 3168 (em-4xp9yUfl) "FW: Tiktok FY26 discounts"
// - "uzeeshan@akamai.com" → derived "U Zeeshan" (should stay "Uzeeshan" or not split)
// - "knaidu@akamai.com" → derived "K Naidu" (should stay "Knaidu" or not split)
//
// EXPECTED: This test should FAIL until the heuristic is fixed to avoid splitting
// lowercase single-word prefixes that don't follow clear initial+lastname patterns.
func TestDeriveNameFromEmail_CorporateEmailPrefixes(t *testing.T) {
	tests := []struct {
		name         string
		email        string
		wantName     string // What we SHOULD get
		currentlyGot string // What we INCORRECTLY get now (documents the bug)
	}{
		{
			name:         "uzeeshan - single-word corporate username",
			email:        "uzeeshan@akamai.com",
			wantName:     "Uzeeshan",     // Should title-case as-is
			currentlyGot: "U Zeeshan",    // BUG: incorrectly splits at position 1
		},
		{
			name:         "knaidu - single-word corporate username",
			email:        "knaidu@akamai.com",
			wantName:     "Knaidu",       // Should title-case as-is
			currentlyGot: "K Naidu",      // BUG: incorrectly splits at position 1
		},
		{
			name:         "ppandya - single-word corporate username",
			email:        "ppandya@akamai.com",
			wantName:     "Ppandya",      // Should title-case as-is
			currentlyGot: "P Pandya",     // BUG: incorrectly splits at position 1
		},
		{
			name:         "jsmith - ambiguous single-word username",
			email:        "jsmith@example.com",
			wantName:     "Jsmith",       // Treat as single word (consistent with knaidu, uzeeshan)
			currentlyGot: "J Smith",      // OLD behavior: split at position 1
		},
		{
			name:         "john.smith - clear separator pattern",
			email:        "john.smith@example.com",
			wantName:     "John Smith",   // Should split on separator
			currentlyGot: "John Smith",   // This case works correctly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveNameFromEmail(tt.email)

			// Document the current buggy behavior
			if got == tt.currentlyGot && tt.currentlyGot != tt.wantName {
				t.Logf("BUG pf-2c2ab0: DeriveNameFromEmail(%q) = %q (incorrect)", tt.email, got)
				t.Logf("EXPECTED: %q", tt.wantName)
				// Fail the test to confirm it reproduces the bug
				t.Errorf("DeriveNameFromEmail(%q) = %q, want %q (bug reproduced)", tt.email, got, tt.wantName)
			} else if got == tt.wantName {
				// This case works correctly
				t.Logf("DeriveNameFromEmail(%q) = %q (correct)", tt.email, got)
			} else {
				// Unexpected result
				t.Errorf("DeriveNameFromEmail(%q) = %q, expected current buggy behavior %q or correct %q",
					tt.email, got, tt.currentlyGot, tt.wantName)
			}
		})
	}
}
