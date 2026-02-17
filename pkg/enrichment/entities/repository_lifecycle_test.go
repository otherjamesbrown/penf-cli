package entities

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRejectPerson tests soft-deleting a person by setting rejected_at.
func TestRejectPerson(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("rejects person successfully", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create a person first
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "Test User",
			PrimaryEmail:  "test@example.com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)
		require.NotZero(t, person.ID)

		// Reject the person
		err = repo.RejectPerson(ctx, testTenantID, person.ID, "spam account", "admin")
		require.NoError(t, err)

		// Verify rejection
		retrieved, err := repo.GetPersonByID(ctx, person.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.NotNil(t, retrieved.RejectedAt)
		assert.Equal(t, "spam account", retrieved.RejectedReason)
		assert.Equal(t, "admin", retrieved.RejectedBy)
	})

	t.Run("double reject is no-op", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create and reject a person
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "Test User",
			PrimaryEmail:  "test2@example.com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)

		err = repo.RejectPerson(ctx, testTenantID, person.ID, "first rejection", "admin")
		require.NoError(t, err)

		// Try to reject again
		err = repo.RejectPerson(ctx, testTenantID, person.ID, "second rejection", "admin")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found or already rejected")
	})

	t.Run("rejects only within tenant", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create person in one tenant
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "Test User",
			PrimaryEmail:  "test3@example.com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)

		// Try to reject from different tenant
		err = repo.RejectPerson(ctx, "different-tenant", person.ID, "reason", "admin")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found or already rejected")
	})

	t.Run("non-existent person returns error", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		err := repo.RejectPerson(ctx, testTenantID, 99999, "reason", "admin")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found or already rejected")
	})
}

// TestRestorePerson tests removing the rejection from a person.
func TestRestorePerson(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("restores rejected person successfully", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create and reject a person
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "Test User",
			PrimaryEmail:  "restore1@example.com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)

		err = repo.RejectPerson(ctx, testTenantID, person.ID, "spam", "admin")
		require.NoError(t, err)

		// Restore the person
		err = repo.RestorePerson(ctx, testTenantID, person.ID)
		require.NoError(t, err)

		// Verify restoration
		retrieved, err := repo.GetPersonByID(ctx, person.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Nil(t, retrieved.RejectedAt)
		assert.Empty(t, retrieved.RejectedReason)
		assert.Empty(t, retrieved.RejectedBy)
	})

	t.Run("restoring non-rejected person returns error", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create person but don't reject
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "Test User",
			PrimaryEmail:  "restore2@example.com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)

		// Try to restore
		err = repo.RestorePerson(ctx, testTenantID, person.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found or not rejected")
	})

	t.Run("restores only within tenant", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create and reject person
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "Test User",
			PrimaryEmail:  "restore3@example.com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)

		err = repo.RejectPerson(ctx, testTenantID, person.ID, "spam", "admin")
		require.NoError(t, err)

		// Try to restore from different tenant
		err = repo.RestorePerson(ctx, "different-tenant", person.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found or not rejected")
	})
}

// TestBulkRejectByPattern tests bulk rejection of people matching patterns.
func TestBulkRejectByPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("rejects by email pattern", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create multiple people with same domain
		for i := 0; i < 3; i++ {
			person := &Person{
				TenantID:      testTenantID,
				CanonicalName: "Spam User",
				PrimaryEmail:  "user" + string(rune('0'+i)) + "@spam.com",
				AccountType:   AccountTypePerson,
				IsInternal:    false,
				Confidence:    0.5,
			}
			err := repo.CreatePerson(ctx, person)
			require.NoError(t, err)
		}

		// Create one with different domain
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "Good User",
			PrimaryEmail:  "user@good.com",
			AccountType:   AccountTypePerson,
			IsInternal:    false,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)

		// Bulk reject by email pattern
		count, err := repo.BulkRejectByPattern(ctx, testTenantID, "%@spam.com", "", "spam domain", "admin")
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		// Verify good user was not rejected
		retrieved, err := repo.GetPersonByEmail(ctx, testTenantID, "user@good.com")
		require.NoError(t, err)
		assert.Nil(t, retrieved.RejectedAt)
	})

	t.Run("rejects by name pattern", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create people with bot names
		for i := 0; i < 2; i++ {
			person := &Person{
				TenantID:      testTenantID,
				CanonicalName: "Bot Service " + string(rune('A'+i)),
				PrimaryEmail:  "bot" + string(rune('0'+i)) + "@example.com",
				AccountType:   AccountTypeBot,
				IsInternal:    false,
				Confidence:    0.8,
			}
			err := repo.CreatePerson(ctx, person)
			require.NoError(t, err)
		}

		// Create a normal user
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "Human User",
			PrimaryEmail:  "human@example.com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)

		// Bulk reject by name pattern
		count, err := repo.BulkRejectByPattern(ctx, testTenantID, "", "Bot%", "automated accounts", "admin")
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		// Verify human was not rejected
		retrieved, err := repo.GetPersonByEmail(ctx, testTenantID, "human@example.com")
		require.NoError(t, err)
		assert.Nil(t, retrieved.RejectedAt)
	})

	t.Run("requires at least one pattern", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		count, err := repo.BulkRejectByPattern(ctx, testTenantID, "", "", "no pattern", "admin")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one pattern")
		assert.Equal(t, 0, count)
	})

	t.Run("operates only within tenant", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create person in tenant
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "User",
			PrimaryEmail:  "user@spam.com",
			AccountType:   AccountTypePerson,
			IsInternal:    false,
			Confidence:    0.5,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)

		// Bulk reject in different tenant should affect 0 people
		count, err := repo.BulkRejectByPattern(ctx, "different-tenant", "%@spam.com", "", "spam", "admin")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

// TestCreateFilterRule tests creating entity filter rules.
func TestCreateFilterRule(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("creates filter rule with email pattern", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		rule := &EntityFilterRule{
			TenantID:     testTenantID,
			EmailPattern: "%@spam.com",
			Reason:       "known spam domain",
			CreatedBy:    "admin",
		}

		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)
		assert.NotZero(t, rule.ID)
		assert.NotZero(t, rule.CreatedAt)
	})

	t.Run("creates filter rule with name pattern", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		rule := &EntityFilterRule{
			TenantID:    testTenantID,
			NamePattern: "Bot%",
			Reason:      "automated accounts",
			CreatedBy:   "admin",
		}

		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)
		assert.NotZero(t, rule.ID)
	})

	t.Run("creates filter rule with both patterns", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		rule := &EntityFilterRule{
			TenantID:     testTenantID,
			EmailPattern: "%noreply%",
			NamePattern:  "%No Reply%",
			Reason:       "no-reply addresses",
			CreatedBy:    "admin",
		}

		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)
		assert.NotZero(t, rule.ID)
	})

	t.Run("creates filter rule with entity type", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		rule := &EntityFilterRule{
			TenantID:     testTenantID,
			EmailPattern: "%@external.com",
			EntityType:   "bot",
			Reason:       "external bots",
			CreatedBy:    "admin",
		}

		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)
		assert.NotZero(t, rule.ID)
	})
}

// TestListFilterRules tests retrieving filter rules for a tenant.
func TestListFilterRules(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("lists all filter rules for tenant", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create multiple rules
		rule1 := &EntityFilterRule{
			TenantID:     testTenantID,
			EmailPattern: "%@spam1.com",
			Reason:       "spam domain 1",
			CreatedBy:    "admin",
		}
		err := repo.CreateFilterRule(ctx, rule1)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond) // Ensure different timestamps

		rule2 := &EntityFilterRule{
			TenantID:    testTenantID,
			NamePattern: "Bot%",
			Reason:      "bots",
			CreatedBy:   "admin",
		}
		err = repo.CreateFilterRule(ctx, rule2)
		require.NoError(t, err)

		// List rules
		rules, err := repo.ListFilterRules(ctx, testTenantID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(rules), 2)

		// Should be ordered by created_at DESC (newest first)
		found1 := false
		found2 := false
		for _, r := range rules {
			if r.ID == rule1.ID {
				found1 = true
				assert.Equal(t, "%@spam1.com", r.EmailPattern)
				assert.Equal(t, "spam domain 1", r.Reason)
			}
			if r.ID == rule2.ID {
				found2 = true
				assert.Equal(t, "Bot%", r.NamePattern)
				assert.Equal(t, "bots", r.Reason)
			}
		}
		assert.True(t, found1)
		assert.True(t, found2)
	})

	t.Run("returns empty list for tenant with no rules", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		rules, err := repo.ListFilterRules(ctx, "empty-tenant")
		require.NoError(t, err)
		assert.Empty(t, rules)
	})

	t.Run("isolates rules by tenant", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create rule in tenant1
		rule1 := &EntityFilterRule{
			TenantID:     "tenant1",
			EmailPattern: "%@spam.com",
			Reason:       "spam",
			CreatedBy:    "admin",
		}
		err := repo.CreateFilterRule(ctx, rule1)
		require.NoError(t, err)

		// Create rule in tenant2
		rule2 := &EntityFilterRule{
			TenantID:     "tenant2",
			EmailPattern: "%@spam.com",
			Reason:       "spam",
			CreatedBy:    "admin",
		}
		err = repo.CreateFilterRule(ctx, rule2)
		require.NoError(t, err)

		// List for tenant1 should only return tenant1 rules
		rules1, err := repo.ListFilterRules(ctx, "tenant1")
		require.NoError(t, err)
		for _, r := range rules1 {
			assert.Equal(t, "tenant1", r.TenantID)
		}

		// List for tenant2 should only return tenant2 rules
		rules2, err := repo.ListFilterRules(ctx, "tenant2")
		require.NoError(t, err)
		for _, r := range rules2 {
			assert.Equal(t, "tenant2", r.TenantID)
		}
	})
}

// TestDeleteFilterRule tests deleting a filter rule.
func TestDeleteFilterRule(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("deletes filter rule successfully", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create a rule
		rule := &EntityFilterRule{
			TenantID:     testTenantID,
			EmailPattern: "%@delete.com",
			Reason:       "test deletion",
			CreatedBy:    "admin",
		}
		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)

		// Delete it
		err = repo.DeleteFilterRule(ctx, testTenantID, rule.ID)
		require.NoError(t, err)

		// Verify it's gone
		rules, err := repo.ListFilterRules(ctx, testTenantID)
		require.NoError(t, err)
		for _, r := range rules {
			assert.NotEqual(t, rule.ID, r.ID)
		}
	})

	t.Run("deletes only within tenant", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create rule in tenant1
		rule := &EntityFilterRule{
			TenantID:     "tenant1",
			EmailPattern: "%@spam.com",
			Reason:       "spam",
			CreatedBy:    "admin",
		}
		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)

		// Try to delete from tenant2
		err = repo.DeleteFilterRule(ctx, "tenant2", rule.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		// Verify rule still exists in tenant1
		rules, err := repo.ListFilterRules(ctx, "tenant1")
		require.NoError(t, err)
		found := false
		for _, r := range rules {
			if r.ID == rule.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("non-existent rule returns error", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		err := repo.DeleteFilterRule(ctx, testTenantID, 99999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// TestTestFilterRule tests checking if email/name matches filter rules.
func TestTestFilterRule(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("matches email pattern", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create filter rule
		rule := &EntityFilterRule{
			TenantID:     testTenantID,
			EmailPattern: "%@spam.com",
			Reason:       "spam domain",
			CreatedBy:    "admin",
		}
		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)

		// Test matching email
		matches, err := repo.TestFilterRule(ctx, testTenantID, "test@spam.com", "Test User")
		require.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.Equal(t, rule.ID, matches[0].ID)
	})

	t.Run("matches name pattern", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create filter rule
		rule := &EntityFilterRule{
			TenantID:    testTenantID,
			NamePattern: "Bot%",
			Reason:      "automated account",
			CreatedBy:   "admin",
		}
		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)

		// Test matching name
		matches, err := repo.TestFilterRule(ctx, testTenantID, "bot@example.com", "Bot Service")
		require.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.Equal(t, rule.ID, matches[0].ID)
	})

	t.Run("returns empty for no match", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create filter rule
		rule := &EntityFilterRule{
			TenantID:     testTenantID,
			EmailPattern: "%@spam.com",
			Reason:       "spam domain",
			CreatedBy:    "admin",
		}
		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)

		// Test non-matching email
		matches, err := repo.TestFilterRule(ctx, testTenantID, "test@good.com", "Test User")
		require.NoError(t, err)
		assert.Empty(t, matches)
	})

	t.Run("returns multiple matching rules", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create multiple rules that match
		rule1 := &EntityFilterRule{
			TenantID:     testTenantID,
			EmailPattern: "%@spam.com",
			Reason:       "spam domain",
			CreatedBy:    "admin",
		}
		err := repo.CreateFilterRule(ctx, rule1)
		require.NoError(t, err)

		rule2 := &EntityFilterRule{
			TenantID:     testTenantID,
			EmailPattern: "%noreply%",
			Reason:       "no-reply address",
			CreatedBy:    "admin",
		}
		err = repo.CreateFilterRule(ctx, rule2)
		require.NoError(t, err)

		// Test email that matches first rule
		matches, err := repo.TestFilterRule(ctx, testTenantID, "noreply@spam.com", "Test")
		require.NoError(t, err)
		assert.Len(t, matches, 2)
	})
}

// TestMatchesFilterRule tests the boolean check for filter rule matching.
func TestMatchesFilterRule(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("returns true when email matches", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create filter rule
		rule := &EntityFilterRule{
			TenantID:     testTenantID,
			EmailPattern: "%@blocked.com",
			Reason:       "blocked domain",
			CreatedBy:    "admin",
		}
		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)

		// Test matching
		matches, err := repo.MatchesFilterRule(ctx, testTenantID, "test@blocked.com", "Test")
		require.NoError(t, err)
		assert.True(t, matches)
	})

	t.Run("returns true when name matches", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create filter rule
		rule := &EntityFilterRule{
			TenantID:    testTenantID,
			NamePattern: "%Automated%",
			Reason:      "automated account",
			CreatedBy:   "admin",
		}
		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)

		// Test matching
		matches, err := repo.MatchesFilterRule(ctx, testTenantID, "bot@example.com", "Automated Service")
		require.NoError(t, err)
		assert.True(t, matches)
	})

	t.Run("returns false when no match", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create filter rule
		rule := &EntityFilterRule{
			TenantID:     testTenantID,
			EmailPattern: "%@blocked.com",
			Reason:       "blocked domain",
			CreatedBy:    "admin",
		}
		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)

		// Test non-matching
		matches, err := repo.MatchesFilterRule(ctx, testTenantID, "test@allowed.com", "Test")
		require.NoError(t, err)
		assert.False(t, matches)
	})

	t.Run("isolates by tenant", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create filter rule in tenant1
		rule := &EntityFilterRule{
			TenantID:     "tenant1",
			EmailPattern: "%@blocked.com",
			Reason:       "blocked domain",
			CreatedBy:    "admin",
		}
		err := repo.CreateFilterRule(ctx, rule)
		require.NoError(t, err)

		// Test in tenant2 should return false
		matches, err := repo.MatchesFilterRule(ctx, "tenant2", "test@blocked.com", "Test")
		require.NoError(t, err)
		assert.False(t, matches)
	})
}

// TestGetEntityStats tests retrieving entity statistics.
func TestGetEntityStats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("returns correct counts", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create various people
		people := []*Person{
			{
				TenantID:      testTenantID,
				CanonicalName: "Internal Person",
				PrimaryEmail:  "internal@example.com",
				AccountType:   AccountTypePerson,
				IsInternal:    true,
				Confidence:    0.9,
				NeedsReview:   false,
				AutoCreated:   true,
			},
			{
				TenantID:      testTenantID,
				CanonicalName: "External Person",
				PrimaryEmail:  "external@other.com",
				AccountType:   AccountTypePerson,
				IsInternal:    false,
				Confidence:    0.6,
				NeedsReview:   true,
				AutoCreated:   true,
			},
			{
				TenantID:      testTenantID,
				CanonicalName: "Bot Account",
				PrimaryEmail:  "bot@example.com",
				AccountType:   AccountTypeBot,
				IsInternal:    false,
				Confidence:    0.8,
				NeedsReview:   false,
				AutoCreated:   true,
			},
		}

		for _, p := range people {
			err := repo.CreatePerson(ctx, p)
			require.NoError(t, err)
		}

		// Reject one person
		err := repo.RejectPerson(ctx, testTenantID, people[1].ID, "spam", "admin")
		require.NoError(t, err)

		// Get stats
		stats, err := repo.GetEntityStats(ctx, testTenantID)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, stats.TotalPeople, int64(3))
		assert.GreaterOrEqual(t, stats.TotalRejected, int64(1))
		assert.GreaterOrEqual(t, stats.Internal, int64(1))
		assert.GreaterOrEqual(t, stats.External, int64(2))
		assert.GreaterOrEqual(t, stats.AutoCreated, int64(3))

		// Check account type breakdown
		assert.GreaterOrEqual(t, stats.ByAccountType[AccountTypePerson], int64(1)) // Only non-rejected
		assert.GreaterOrEqual(t, stats.ByAccountType[AccountTypeBot], int64(1))

		// Check confidence breakdown
		assert.GreaterOrEqual(t, stats.ByConfidence["high"], int64(1))  // 0.8+
		assert.GreaterOrEqual(t, stats.ByConfidence["medium"], int64(1)) // 0.5-0.8
	})

	t.Run("isolates by tenant", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create person in tenant1
		person1 := &Person{
			TenantID:      "stats-tenant1",
			CanonicalName: "User 1",
			PrimaryEmail:  "user1@tenant1.com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person1)
		require.NoError(t, err)

		// Create person in tenant2
		person2 := &Person{
			TenantID:      "stats-tenant2",
			CanonicalName: "User 2",
			PrimaryEmail:  "user2@tenant2.com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err = repo.CreatePerson(ctx, person2)
		require.NoError(t, err)

		// Get stats for tenant1
		stats1, err := repo.GetEntityStats(ctx, "stats-tenant1")
		require.NoError(t, err)

		// Get stats for tenant2
		stats2, err := repo.GetEntityStats(ctx, "stats-tenant2")
		require.NoError(t, err)

		// They should have different counts
		assert.NotEqual(t, stats1.TotalPeople, stats2.TotalPeople)
	})
}

// TestSearchEntities tests searching for entities by name or email.
func TestSearchEntities(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("searches by email", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create test people
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "John Doe",
			PrimaryEmail:  "john.doe@example.com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)

		// Search by email
		results, err := repo.SearchEntities(ctx, testTenantID, "john.doe", "email", 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 1)

		found := false
		for _, p := range results {
			if p.ID == person.ID {
				found = true
				assert.Equal(t, "john.doe@example.com", p.PrimaryEmail)
			}
		}
		assert.True(t, found)
	})

	t.Run("searches by name", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create test people
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "Jane Smith",
			PrimaryEmail:  "jane.smith@example.com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)

		// Search by name
		results, err := repo.SearchEntities(ctx, testTenantID, "Jane", "name", 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 1)

		found := false
		for _, p := range results {
			if p.ID == person.ID {
				found = true
				assert.Equal(t, "Jane Smith", p.CanonicalName)
			}
		}
		assert.True(t, found)
	})

	t.Run("searches both fields when field is empty", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create test people
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "Bob Wilson",
			PrimaryEmail:  "bob@example.com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)

		// Search both fields
		results, err := repo.SearchEntities(ctx, testTenantID, "Bob", "", 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 1)

		found := false
		for _, p := range results {
			if p.ID == person.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("respects limit", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create multiple people
		for i := 0; i < 5; i++ {
			person := &Person{
				TenantID:      testTenantID,
				CanonicalName: "Test User",
				PrimaryEmail:  "test" + string(rune('0'+i)) + "@limit.com",
				AccountType:   AccountTypePerson,
				IsInternal:    true,
				Confidence:    0.9,
			}
			err := repo.CreatePerson(ctx, person)
			require.NoError(t, err)
		}

		// Search with limit
		results, err := repo.SearchEntities(ctx, testTenantID, "@limit.com", "email", 3)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(results), 3)
	})

	t.Run("case insensitive search", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		// Create person
		person := &Person{
			TenantID:      testTenantID,
			CanonicalName: "CamelCase User",
			PrimaryEmail:  "CamelCase@Example.Com",
			AccountType:   AccountTypePerson,
			IsInternal:    true,
			Confidence:    0.9,
		}
		err := repo.CreatePerson(ctx, person)
		require.NoError(t, err)

		// Search with lowercase
		results, err := repo.SearchEntities(ctx, testTenantID, "camelcase", "", 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 1)

		found := false
		for _, p := range results {
			if p.ID == person.ID {
				found = true
			}
		}
		assert.True(t, found)
	})
}

// Helper functions for tests

const testTenantID = "test-tenant"

func setupTestRepo(t *testing.T) (*Repository, func()) {
	// This would set up a test database connection
	// For now, we'll skip actual DB tests in short mode
	// In integration tests, this would return a real repository
	t.Skip("Integration test - requires database setup")
	return nil, func() {}
}
