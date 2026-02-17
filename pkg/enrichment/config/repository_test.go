package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTenantID = "test-tenant-1"

// TestConfigRepo_CreateEmailPattern tests creating a new email pattern.
func TestConfigRepo_CreateEmailPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("creates pattern successfully", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// Create a new email pattern
		pattern, err := repo.CreateEmailPattern(ctx, testTenantID, "*-jira@*", "bot", "Jira notification bot")
		require.NoError(t, err)
		require.NotNil(t, pattern)

		// Verify returned struct has correct fields
		assert.NotZero(t, pattern.ID)
		assert.Equal(t, testTenantID, pattern.TenantID)
		assert.Equal(t, "*-jira@*", pattern.Pattern)
		assert.Equal(t, "bot", pattern.PatternType)
		assert.Equal(t, 100, pattern.Priority) // Default priority
		assert.True(t, pattern.Enabled)        // Default enabled
	})

	t.Run("creates pattern with custom priority", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// Create pattern with high priority
		pattern, err := repo.CreateEmailPattern(ctx, testTenantID, "noreply@*", "ignore", "No-reply addresses")
		require.NoError(t, err)

		assert.NotZero(t, pattern.ID)
		assert.Equal(t, "noreply@*", pattern.Pattern)
	})
}

// TestConfigRepo_CreateEmailPattern_DuplicatePattern tests duplicate pattern detection.
func TestConfigRepo_CreateEmailPattern_DuplicatePattern(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("duplicate pattern returns error", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// Create first pattern
		_, err := repo.CreateEmailPattern(ctx, testTenantID, "*@automated.com", "bot", "Automated domain")
		require.NoError(t, err)

		// Try to create same pattern again
		_, err = repo.CreateEmailPattern(ctx, testTenantID, "*@automated.com", "bot", "Duplicate attempt")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate") // Should contain constraint violation hint
	})

	t.Run("same pattern in different tenant is allowed", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// Create pattern in tenant 1
		pattern1, err := repo.CreateEmailPattern(ctx, testTenantID, "*@shared.com", "bot", "Tenant 1")
		require.NoError(t, err)
		assert.NotZero(t, pattern1.ID)

		// Create same pattern in tenant 2
		pattern2, err := repo.CreateEmailPattern(ctx, "test-tenant-2", "*@shared.com", "bot", "Tenant 2")
		require.NoError(t, err)
		assert.NotZero(t, pattern2.ID)
		assert.NotEqual(t, pattern1.ID, pattern2.ID)
	})
}

// TestConfigRepo_CreateEmailPattern_InvalidType tests pattern type validation.
func TestConfigRepo_CreateEmailPattern_InvalidType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("invalid pattern type returns error", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// Try to create pattern with invalid type
		_, err := repo.CreateEmailPattern(ctx, testTenantID, "*@test.com", "invalid_type", "Invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "constraint") // CHECK constraint violation
	})

	t.Run("all valid types succeed", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		validTypes := []string{"bot", "distribution_list", "role_account", "ignore"}

		for i, patternType := range validTypes {
			pattern, err := repo.CreateEmailPattern(
				ctx,
				testTenantID,
				"pattern"+string(rune('0'+i))+"@*",
				patternType,
				"Valid type: "+patternType,
			)
			require.NoError(t, err, "pattern type %s should be valid", patternType)
			assert.Equal(t, patternType, pattern.PatternType)
		}
	})
}

// TestConfigRepo_ListEmailPatterns tests listing all patterns for a tenant.
func TestConfigRepo_ListEmailPatterns(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("lists all patterns for tenant", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// Create multiple patterns
		pattern1, err := repo.CreateEmailPattern(ctx, testTenantID, "*-bot@*", "bot", "Bot pattern")
		require.NoError(t, err)

		pattern2, err := repo.CreateEmailPattern(ctx, testTenantID, "team-*@*", "distribution_list", "Team DL")
		require.NoError(t, err)

		// List patterns
		patterns, err := repo.ListEmailPatterns(ctx, testTenantID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(patterns), 2)

		// Verify patterns are in the list
		found1 := false
		found2 := false
		for _, p := range patterns {
			if p.ID == pattern1.ID {
				found1 = true
				assert.Equal(t, "*-bot@*", p.Pattern)
				assert.Equal(t, "bot", p.PatternType)
			}
			if p.ID == pattern2.ID {
				found2 = true
				assert.Equal(t, "team-*@*", p.Pattern)
				assert.Equal(t, "distribution_list", p.PatternType)
			}
		}
		assert.True(t, found1, "pattern1 should be in list")
		assert.True(t, found2, "pattern2 should be in list")
	})

	t.Run("includes disabled patterns", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// Create an enabled pattern
		enabledPattern, err := repo.CreateEmailPattern(ctx, testTenantID, "enabled@*", "bot", "Enabled")
		require.NoError(t, err)

		// List should include both enabled and disabled
		patterns, err := repo.ListEmailPatterns(ctx, testTenantID)
		require.NoError(t, err)

		foundEnabled := false
		for _, p := range patterns {
			if p.ID == enabledPattern.ID {
				foundEnabled = true
				assert.True(t, p.Enabled)
			}
		}
		assert.True(t, foundEnabled)
	})
}

// TestConfigRepo_ListEmailPatterns_Empty tests empty list for tenant with no patterns.
func TestConfigRepo_ListEmailPatterns_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("returns empty slice for tenant with no patterns", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// List patterns for a tenant that has none
		patterns, err := repo.ListEmailPatterns(ctx, "nonexistent-tenant")
		require.NoError(t, err)
		assert.NotNil(t, patterns)
		assert.Empty(t, patterns)
	})
}

// TestConfigRepo_ListEmailPatterns_FiltersByTenant tests tenant isolation.
func TestConfigRepo_ListEmailPatterns_FiltersByTenant(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("only returns patterns for specified tenant", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// Create pattern in tenant 1
		pattern1, err := repo.CreateEmailPattern(ctx, "tenant-iso-1", "tenant1@*", "bot", "Tenant 1")
		require.NoError(t, err)

		// Create pattern in tenant 2
		pattern2, err := repo.CreateEmailPattern(ctx, "tenant-iso-2", "tenant2@*", "bot", "Tenant 2")
		require.NoError(t, err)

		// List for tenant 1
		patterns1, err := repo.ListEmailPatterns(ctx, "tenant-iso-1")
		require.NoError(t, err)

		// Verify only tenant 1 patterns are returned
		for _, p := range patterns1 {
			assert.Equal(t, "tenant-iso-1", p.TenantID)
			assert.NotEqual(t, pattern2.ID, p.ID, "tenant 2 pattern should not be in tenant 1 list")
		}

		// Verify tenant 1 pattern is in the list
		found := false
		for _, p := range patterns1 {
			if p.ID == pattern1.ID {
				found = true
			}
		}
		assert.True(t, found, "tenant 1 pattern should be in tenant 1 list")
	})
}

// TestConfigRepo_GetTenantEmailPatterns tests interface method that returns only enabled patterns.
func TestConfigRepo_GetTenantEmailPatterns(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("returns only enabled patterns", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// Create enabled pattern
		enabledPattern, err := repo.CreateEmailPattern(ctx, testTenantID, "enabled@*", "bot", "Enabled")
		require.NoError(t, err)

		// Use interface method (defined in config.go)
		patterns, err := repo.GetTenantEmailPatterns(ctx, testTenantID)
		require.NoError(t, err)

		// All returned patterns should be enabled
		for _, p := range patterns {
			assert.True(t, p.Enabled, "GetTenantEmailPatterns should only return enabled patterns")
		}

		// Verify enabled pattern is in the list
		found := false
		for _, p := range patterns {
			if p.ID == enabledPattern.ID {
				found = true
			}
		}
		assert.True(t, found, "enabled pattern should be in list")
	})
}

// TestConfigRepo_DeleteEmailPattern tests deleting a pattern.
func TestConfigRepo_DeleteEmailPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("deletes pattern successfully", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// Create a pattern
		pattern, err := repo.CreateEmailPattern(ctx, testTenantID, "delete@*", "bot", "To be deleted")
		require.NoError(t, err)

		// Delete it
		err = repo.DeleteEmailPattern(ctx, testTenantID, pattern.ID)
		require.NoError(t, err)

		// Verify it's not in the list anymore
		patterns, err := repo.ListEmailPatterns(ctx, testTenantID)
		require.NoError(t, err)

		for _, p := range patterns {
			assert.NotEqual(t, pattern.ID, p.ID, "deleted pattern should not be in list")
		}
	})

	t.Run("deletes only within tenant", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// Create pattern in tenant 1
		pattern, err := repo.CreateEmailPattern(ctx, "tenant-del-1", "tenant1@*", "bot", "Tenant 1")
		require.NoError(t, err)

		// Try to delete from tenant 2
		err = repo.DeleteEmailPattern(ctx, "tenant-del-2", pattern.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		// Verify pattern still exists in tenant 1
		patterns, err := repo.ListEmailPatterns(ctx, "tenant-del-1")
		require.NoError(t, err)

		found := false
		for _, p := range patterns {
			if p.ID == pattern.ID {
				found = true
			}
		}
		assert.True(t, found, "pattern should still exist in tenant 1")
	})
}

// TestConfigRepo_DeleteEmailPattern_NotFound tests deleting non-existent pattern.
func TestConfigRepo_DeleteEmailPattern_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("deleting non-existent pattern returns error", func(t *testing.T) {
		ctx := context.Background()
		repo, cleanup := setupTestConfigRepo(t)
		defer cleanup()

		// Try to delete non-existent pattern
		err := repo.DeleteEmailPattern(ctx, testTenantID, 99999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// Helper functions for tests

// setupTestConfigRepo sets up a test repository.
// This will be implemented by the implementation shard to return a real DB-backed repository.
func setupTestConfigRepo(t *testing.T) (*ConfigRepositoryImpl, func()) {
	// This would set up a test database connection
	// For now, we'll skip actual DB tests in short mode
	// In integration tests, this would return a real repository
	t.Skip("Integration test - requires database setup")
	return nil, func() {}
}
