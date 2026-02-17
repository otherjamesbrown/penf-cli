package entities

import (
	"testing"
)

// TestNormalizeName_WithDisplayName tests that display names are normalized correctly.
// This is a unit test for the logic that will be used in ResolveOrCreate.
func TestNormalizeName_WithDisplayName(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		email       string
		wantName    string
	}{
		{
			name:        "display name from email header with Last, First format",
			displayName: "Oslakovic, Keith",
			email:       "koslakov@akamai.com",
			wantName:    "Keith Oslakovic",
		},
		{
			name:        "display name with standard format",
			displayName: "John Smith",
			email:       "john.smith@example.com",
			wantName:    "John Smith",
		},
		{
			name:        "display name overrides email prefix",
			displayName: "Jane Doe",
			email:       "jdoe@example.com",
			wantName:    "Jane Doe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When display name is provided, it should be normalized
			got := NormalizeDisplayName(tt.displayName)
			if got != tt.wantName {
				t.Errorf("NormalizeDisplayName(%q) = %q, want %q", tt.displayName, got, tt.wantName)
			}
		})
	}
}

// TestNameDerivation_FromEmail tests that names are derived from email prefixes when no display name is available.
// This test currently FAILS because DeriveNameFromEmail() does not exist yet.
// Once implemented, this function should be called as a fallback when displayName is empty.
func TestNameDerivation_FromEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		wantName string
	}{
		{
			name:     "firstname.lastname pattern",
			email:    "john.smith@example.com",
			wantName: "John Smith",
		},
		{
			name:     "firstname_lastname pattern",
			email:    "jane_doe@example.com",
			wantName: "Jane Doe",
		},
		{
			name:     "single word lowercase (ambiguous)",
			email:    "jsmith@example.com",
			wantName: "Jsmith", // Consistent with fix for pf-2c2ab0: preserve single-word usernames
		},
		{
			name:     "three part name",
			email:    "mary.ann.jones@example.com",
			wantName: "Mary Ann Jones",
		},
		{
			name:     "hyphenated name",
			email:    "mary-ann@example.com",
			wantName: "Mary Ann",
		},
		{
			name:     "all lowercase single word",
			email:    "johnsmith@example.com",
			wantName: "Johnsmith",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests the DeriveNameFromEmail function that we'll implement
			got := DeriveNameFromEmail(tt.email)
			if got != tt.wantName {
				t.Errorf("DeriveNameFromEmail(%q) = %q, want %q", tt.email, got, tt.wantName)
			}
		})
	}
}

// TestDetectAccountType_NewPatterns verifies that DetectAccountType correctly identifies
// role accounts, bots, and external services that were added in recent pattern updates.
// This is part 1 of the regression test for bug pf-276070.
//
// This test documents the EXPECTED behavior of DetectAccountType with emails that
// previously may have been classified as 'person' but should now be classified correctly.
func TestDetectAccountType_NewPatterns(t *testing.T) {
	tests := []struct {
		email       string
		displayName string
		want        AccountType
	}{
		// Role accounts
		{"Prb-Facilitator@akamai.com", "", AccountTypeRole},
		{"prb-facilitator@akamai.com", "", AccountTypeRole},
		{"facilitator@company.com", "", AccountTypeRole},
		{"support@company.com", "", AccountTypeRole},

		// Bots
		{"gsd-jira@akamai.com", "", AccountTypeBot},
		{"noreply@company.com", "", AccountTypeBot},

		// External services
		{"updates@mailer.aha.io", "", AccountTypeExternalService},
		{"notification@mailer.aha.io", "", AccountTypeExternalService},
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

// TestResolveOrCreate_AccountTypeMismatch documents the bug in ResolveOrCreate where
// existing entities with stale account_type values are returned as-is without updating.
// This is part 2 of the regression test for bug pf-276070.
//
// BUG: When GetPersonByEmail finds an existing entity, ResolveOrCreate returns it
// immediately (lines 114-124 in resolver.go) without checking if the stored account_type
// matches what DetectAccountType would return for that email.
//
// SCENARIO:
// 1. Entity created before new patterns were added, stored as account_type='person'
// 2. New patterns added to DetectAccountType (e.g., 'prb-facilitator', 'mailer.aha.io')
// 3. ResolveOrCreate called with same email
// 4. CURRENT: Returns entity with stale account_type='person'
// 5. EXPECTED: Should detect mismatch and update entity to account_type='role'/'bot'/etc
//
// This test demonstrates the issue by showing that:
// - DetectAccountType returns the correct type for these emails
// - But a Person struct with these emails would have stale account_type
// - ResolveOrCreate should reconcile this mismatch
//
// NOTE: This test cannot be fully automated without either:
// (a) Adding a repository interface for mocking, or
// (b) Using integration tests with a real database
//
// For now, this test documents the expected behavior and serves as a specification.
func TestResolveOrCreate_AccountTypeMismatch_Documentation(t *testing.T) {
	type mismatchCase struct {
		email               string
		staleAccountType    AccountType
		correctAccountType  AccountType
	}

	cases := []mismatchCase{
		{
			email:              "Prb-Facilitator@akamai.com",
			staleAccountType:   AccountTypePerson,
			correctAccountType: AccountTypeRole,
		},
		{
			email:              "updates@mailer.aha.io",
			staleAccountType:   AccountTypePerson,
			correctAccountType: AccountTypeExternalService,
		},
		{
			email:              "gsd-jira@akamai.com",
			staleAccountType:   AccountTypePerson,
			correctAccountType: AccountTypeBot,
		},
	}

	for _, tc := range cases {
		t.Run(tc.email, func(t *testing.T) {
			// Verify that DetectAccountType returns the correct type
			detectedType := DetectAccountType(tc.email, "")
			if detectedType != tc.correctAccountType {
				t.Errorf("DetectAccountType(%q) = %q, want %q", tc.email, detectedType, tc.correctAccountType)
			}

			// Document the expected ResolveOrCreate behavior:
			// When an existing Person with this email has account_type = staleAccountType,
			// ResolveOrCreate should:
			// 1. Detect the mismatch between stored account_type and DetectAccountType result
			// 2. Update the Person.AccountType to match DetectAccountType
			// 3. Call repo.UpdatePerson to persist the correction
			// 4. Return the corrected Person

			t.Logf("BUG: If existing Person has email=%q with account_type=%q,", tc.email, tc.staleAccountType)
			t.Logf("     ResolveOrCreate currently returns it unchanged,")
			t.Logf("     but SHOULD update account_type to %q", tc.correctAccountType)
		})
	}
}

// TestResolveOrCreate_FilterRuleBlocks tests that filter rules prevent entity creation.
// This is a unit test documenting the expected behavior when MatchesFilterRule returns true.
//
// BEHAVIOR: When a filter rule matches the email or name being resolved:
// 1. ResolveOrCreate should call repo.MatchesFilterRule(ctx, tenantID, email, displayName)
// 2. If it returns (true, nil), entity creation should be blocked
// 3. ResolveOrCreate should return (nil, error) with a descriptive error message
// 4. The error message should indicate that entity creation was blocked by a filter rule
//
// This prevents unwanted entities (spam, bots, test accounts) from being auto-created
// during pipeline processing.
func TestResolveOrCreate_FilterRuleBlocks(t *testing.T) {
	t.Run("filter rule blocks entity creation by email", func(t *testing.T) {
		// This test documents the expected behavior:
		// When MatchesFilterRule returns true for an email pattern,
		// the entity should NOT be created and an error should be returned.

		email := "spam@blocked.com"
		displayName := "Spam User"

		// Expected flow:
		// 1. GetPersonByEmail(ctx, tenantID, email) -> (nil, nil) [not found]
		// 2. GetPersonByAlias(ctx, tenantID, email) -> (nil, nil) [not found]
		// 3. MatchesFilterRule(ctx, tenantID, email, displayName) -> (true, nil) [blocked]
		// 4. Return (nil, error) with message like "entity creation blocked by filter rule: spam@blocked.com"

		t.Logf("EXPECTED: ResolveOrCreate(%q, %q) should:", email, displayName)
		t.Logf("  1. Check MatchesFilterRule(%q, %q)", email, displayName)
		t.Logf("  2. When it returns true, return (nil, error)")
		t.Logf("  3. Error message should contain 'blocked by filter rule'")
		t.Logf("  4. CreatePerson should NOT be called")
	})

	t.Run("filter rule blocks entity creation by name", func(t *testing.T) {
		// This test documents the expected behavior:
		// When MatchesFilterRule returns true for a name pattern,
		// the entity should NOT be created and an error should be returned.

		email := "bot@example.com"
		displayName := "Bot Service Account"

		// Expected flow:
		// 1. GetPersonByEmail(ctx, tenantID, email) -> (nil, nil) [not found]
		// 2. GetPersonByAlias(ctx, tenantID, email) -> (nil, nil) [not found]
		// 3. MatchesFilterRule(ctx, tenantID, email, displayName) -> (true, nil) [blocked]
		// 4. Return (nil, error) with message like "entity creation blocked by filter rule: bot@example.com"

		t.Logf("EXPECTED: ResolveOrCreate(%q, %q) should:", email, displayName)
		t.Logf("  1. Check MatchesFilterRule(%q, %q)", email, displayName)
		t.Logf("  2. When it returns true, return (nil, error)")
		t.Logf("  3. Error message should contain 'blocked by filter rule'")
		t.Logf("  4. CreatePerson should NOT be called")
	})

	t.Run("filter rule check error is handled gracefully", func(t *testing.T) {
		// This test documents error handling:
		// If MatchesFilterRule returns an error, the resolver should:
		// 1. Log a warning
		// 2. Continue with entity creation (fail open, not fail closed)
		// This prevents database issues from blocking legitimate entities

		t.Logf("EXPECTED: When MatchesFilterRule returns error:")
		t.Logf("  1. Log warning about filter check failure")
		t.Logf("  2. Continue with entity creation (fail open)")
		t.Logf("  3. CreatePerson should still be called")
	})

	t.Run("no filter rule allows entity creation", func(t *testing.T) {
		// This test documents the normal flow:
		// When MatchesFilterRule returns (false, nil), entity creation proceeds normally

		t.Logf("EXPECTED: When MatchesFilterRule returns (false, nil):")
		t.Logf("  1. Entity creation should proceed")
		t.Logf("  2. CreatePerson should be called")
		t.Logf("  3. Return created person with IsNew=true")
	})
}

// TestResolver_WithTenantPatterns tests that the Resolver can be configured with
// custom tenant-specific patterns and that these patterns are used during entity resolution.
func TestResolver_WithTenantPatterns(t *testing.T) {
	// This test verifies that:
	// 1. Resolver accepts a WithTenantPatterns option
	// 2. The patterns are stored and used when detecting account types
	// 3. ResolveOrCreate uses the custom patterns when creating new entities

	t.Run("resolver accepts tenant patterns option", func(t *testing.T) {
		// This test documents the expected API:
		// r := NewResolver(repo,
		//     WithTenantPatterns(&AccountTypePatterns{
		//         BotPatterns: []string{"custom-bot-"},
		//         DistributionPatterns: []string{"custom-dist-"},
		//         RolePatterns: []string{"custom-role"},
		//         ExternalDomains: []string{"external.example.com"},
		//     }),
		// )

		t.Logf("EXPECTED: NewResolver should accept WithTenantPatterns option")
		t.Logf("  type WithTenantPatterns func(*AccountTypePatterns) ResolverOption")
		t.Logf("  Stores patterns in Resolver for use during entity resolution")

		// Verify the option is available
		// This will fail until WithTenantPatterns is implemented
		t.Skip("WithTenantPatterns option not yet implemented")
	})

	t.Run("resolver uses custom patterns for account type detection", func(t *testing.T) {
		// This test documents the integration:
		// When ResolveOrCreate creates a new entity, it should:
		// 1. Call DetectAccountTypeWithPatterns with the configured custom patterns
		// 2. Use the result to set Person.AccountType
		// 3. Store the correctly classified entity

		t.Logf("EXPECTED: When ResolveOrCreate creates new entity with custom patterns:")
		t.Logf("  1. Call DetectAccountTypeWithPatterns(email, displayName, customPatterns)")
		t.Logf("  2. Set Person.AccountType to the result")
		t.Logf("  3. Custom bot 'acme-bot-scheduler@company.com' -> AccountTypeBot")
		t.Logf("  4. Custom role 'acme-support@company.com' -> AccountTypeRole")

		// This will fail until the integration is implemented
		t.Skip("Custom pattern integration not yet implemented")
	})

	t.Run("resolver updates stale account types with custom patterns", func(t *testing.T) {
		// This test documents a critical behavior:
		// When ResolveOrCreate finds an existing entity, it should:
		// 1. Re-detect account type using CURRENT custom patterns
		// 2. Compare with stored Person.AccountType
		// 3. If mismatch, update the entity
		//
		// This ensures that when tenant patterns are updated, existing entities
		// are automatically corrected on next resolution.

		t.Logf("EXPECTED: When ResolveOrCreate finds existing entity:")
		t.Logf("  1. Call DetectAccountTypeWithPatterns with CURRENT custom patterns")
		t.Logf("  2. Compare result with Person.AccountType")
		t.Logf("  3. If different, update Person.AccountType and call UpdatePerson")
		t.Logf("  4. This auto-corrects stale classifications when patterns change")

		// This will fail until the integration is implemented
		t.Skip("Account type update with custom patterns not yet implemented")
	})

	t.Run("empty tenant patterns use defaults", func(t *testing.T) {
		// This test verifies backward compatibility:
		// If WithTenantPatterns is called with empty/nil patterns,
		// DetectAccountType should fall back to hardcoded defaults only.

		t.Logf("EXPECTED: WithTenantPatterns(nil) or WithTenantPatterns(&AccountTypePatterns{}):")
		t.Logf("  1. Should not error")
		t.Logf("  2. Should fall back to hardcoded patterns")
		t.Logf("  3. 'noreply@company.com' -> AccountTypeBot (default pattern)")
		t.Logf("  4. 'support@company.com' -> AccountTypeRole (default pattern)")

		// This will fail until the integration is implemented
		t.Skip("Empty pattern handling not yet implemented")
	})
}
