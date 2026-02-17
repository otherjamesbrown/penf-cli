//go:build integration

package entities

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

const (
	// IntegrationTestTenantID is the tenant ID for integration tests.
	IntegrationTestTenantID = "00000000-0000-0000-0000-000000000002"
)

// TestResolveOrCreate_StaleAccountType_Integration is an integration test that reproduces
// bug pf-276070: ResolveOrCreate returns existing entities without updating stale account_type.
//
// This test:
// 1. Creates a Person entity with account_type='person' and email='prb-facilitator@test.com'
// 2. Calls ResolveOrCreate with the same email
// 3. Expects the returned entity to have account_type='role' (updated)
// 4. Currently FAILS because ResolveOrCreate returns the stale account_type='person'
//
// After the bug is fixed, this test should PASS.
func TestResolveOrCreate_StaleAccountType_Integration(t *testing.T) {
	ctx := context.Background()

	// Setup database connection
	pool := setupTestDB(t)
	tenantID := IntegrationTestTenantID
	logger := logging.MustGlobal()

	// Create repository and resolver
	repo := NewRepository(pool, logger)
	resolver := NewResolver(repo, WithResolverLogger(logger))

	// Test cases with emails that should be detected as non-person types
	testCases := []struct {
		name                string
		email               string
		initialAccountType  AccountType
		expectedAccountType AccountType
	}{
		{
			name:                "prb-facilitator should be role",
			email:               "prb-facilitator@test-integration.com",
			initialAccountType:  AccountTypePerson,
			expectedAccountType: AccountTypeRole,
		},
		{
			name:                "updates at mailer.aha.io should be external_service",
			email:               "updates@mailer.aha.io",
			initialAccountType:  AccountTypePerson,
			expectedAccountType: AccountTypeExternalService,
		},
		{
			name:                "gsd-jira should be bot",
			email:               "gsd-automation@test-integration.com",
			initialAccountType:  AccountTypePerson,
			expectedAccountType: AccountTypeBot,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up any existing person with this email
			cleanupPerson(t, ctx, pool, tenantID, tc.email)
			defer cleanupPerson(t, ctx, pool, tenantID, tc.email)

			// Step 1: Create a person with STALE account_type (simulate old data)
			stalePerson := &Person{
				TenantID:      tenantID,
				CanonicalName: "Test Person",
				PrimaryEmail:  tc.email,
				AccountType:   tc.initialAccountType, // STALE - should be updated
				IsInternal:    false,
				Confidence:    0.6,
				AutoCreated:   true,
				NeedsReview:   true,
			}

			if err := repo.CreatePerson(ctx, stalePerson); err != nil {
				t.Fatalf("Failed to create stale person: %v", err)
			}
			t.Logf("Created person ID %d with STALE account_type=%q", stalePerson.ID, stalePerson.AccountType)

			// Verify DetectAccountType returns the correct type
			detectedType := DetectAccountType(tc.email, "")
			if detectedType != tc.expectedAccountType {
				t.Fatalf("DetectAccountType(%q) = %q, want %q (test setup issue)",
					tc.email, detectedType, tc.expectedAccountType)
			}
			t.Logf("DetectAccountType correctly identifies email as %q", detectedType)

			// Step 2: Call ResolveOrCreate - this should detect and fix the stale account_type
			result, err := resolver.ResolveOrCreate(ctx, tenantID, tc.email, "")
			if err != nil {
				t.Fatalf("ResolveOrCreate failed: %v", err)
			}

			// Step 3: Assert the returned account_type matches current DetectAccountType logic
			if result.Person.AccountType != tc.expectedAccountType {
				t.Errorf("FAIL: ResolveOrCreate returned account_type=%q, want %q",
					result.Person.AccountType, tc.expectedAccountType)
				t.Errorf("BUG REPRODUCED: Stale account_type was NOT updated")
				t.Logf("Person ID: %d", result.Person.ID)
				t.Logf("Expected: account_type should be updated from %q to %q",
					tc.initialAccountType, tc.expectedAccountType)
			} else {
				t.Logf("PASS: account_type correctly updated to %q", result.Person.AccountType)
			}

			// Step 4: Verify the database was actually updated (not just the in-memory object)
			dbPerson, err := repo.GetPersonByEmail(ctx, tenantID, tc.email)
			if err != nil {
				t.Fatalf("Failed to fetch person from DB: %v", err)
			}
			if dbPerson.AccountType != tc.expectedAccountType {
				t.Errorf("FAIL: Database still has account_type=%q, want %q",
					dbPerson.AccountType, tc.expectedAccountType)
				t.Errorf("BUG: Changes were not persisted to database")
			}
		})
	}
}

// setupTestDB creates a connection to the test database.
func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	host := getEnvOrDefault("PENFOLD_DB_HOST", "dev02.brown.chat")
	port := getEnvOrDefault("PENFOLD_DB_PORT", "5432")
	user := getEnvOrDefault("PENFOLD_DB_USER", "penfold")
	dbName := getEnvOrDefault("PENFOLD_DB_NAME", "penfold")

	// Build connection string with SSL cert auth
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}
	sslCert := filepath.Join(homeDir, ".postgresql", "postgresql.crt")
	sslKey := filepath.Join(homeDir, ".postgresql", "postgresql.key")
	sslRootCert := filepath.Join(homeDir, ".postgresql", "root.crt")

	// Check if SSL certs exist
	if _, err := os.Stat(sslCert); os.IsNotExist(err) {
		t.Skip("SSL certs not found in ~/.postgresql/ - skipping integration test")
	}

	connStr := fmt.Sprintf(
		"postgres://%s@%s:%s/%s?sslmode=verify-full&sslcert=%s&sslkey=%s&sslrootcert=%s",
		user, host, port, dbName, sslCert, sslKey, sslRootCert,
	)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping test database: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

// getEnvOrDefault returns the environment variable value or a default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestResolveOrCreate_CanonicalNameUpdate_Integration is an integration test that reproduces
// bug pf-38e4a9: ResolveOrCreate does not update canonical_name when a better display name
// becomes available.
//
// ROOT CAUSE: When finding an existing entity by exact email match (lines ~119-152 in resolver.go),
// the code only adds the display name as an alias but never updates canonical_name. When an entity
// is first created with canonical_name=email (e.g., "mponec@akamai.com") and later resolved again
// with a display name (e.g., "Miroslav Ponec"), the canonical_name stays as the email address.
//
// This test:
// 1. Creates a Person entity with canonical_name = email address (no display name available)
// 2. Calls ResolveOrCreate with the same email AND a display name
// 3. Expects the returned entity to have canonical_name updated to the display name
// 4. Currently FAILS because ResolveOrCreate never updates canonical_name for existing entities
//
// After the bug is fixed, this test should PASS.
func TestResolveOrCreate_CanonicalNameUpdate_Integration(t *testing.T) {
	ctx := context.Background()

	// Setup database connection
	pool := setupTestDB(t)
	tenantID := IntegrationTestTenantID
	logger := logging.MustGlobal()

	// Create repository and resolver
	repo := NewRepository(pool, logger)
	resolver := NewResolver(repo, WithResolverLogger(logger))

	testCases := []struct {
		name                 string
		email                string
		initialCanonicalName string // Email address used as canonical name
		displayName          string // Better display name provided later
		expectedCanonicalName string // Should be updated to display name
	}{
		{
			name:                 "email canonical name updated to proper display name",
			email:                "mponec@test-integration.com",
			initialCanonicalName: "mponec@test-integration.com", // Email used as name initially
			displayName:          "Miroslav Ponec",
			expectedCanonicalName: "Miroslav Ponec",
		},
		{
			name:                 "email prefix canonical name updated to full name",
			email:                "jsmith@test-integration.com",
			initialCanonicalName: "jsmith@test-integration.com",
			displayName:          "John Smith",
			expectedCanonicalName: "John Smith",
		},
		{
			name:                 "email canonical name with domain updated to clean name",
			email:                "contact@test-integration.com",
			initialCanonicalName: "contact@test-integration.com",
			displayName:          "Jane Contact",
			expectedCanonicalName: "Jane Contact",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up any existing person with this email
			cleanupPerson(t, ctx, pool, tenantID, tc.email)
			defer cleanupPerson(t, ctx, pool, tenantID, tc.email)

			// Step 1: Create a person with canonical_name = email address
			// This simulates an entity created before a display name was available
			initialPerson := &Person{
				TenantID:      tenantID,
				CanonicalName: tc.initialCanonicalName, // Using email as canonical name
				PrimaryEmail:  tc.email,
				AccountType:   AccountTypePerson,
				IsInternal:    false,
				Confidence:    0.6,
				AutoCreated:   true,
				NeedsReview:   true,
			}

			if err := repo.CreatePerson(ctx, initialPerson); err != nil {
				t.Fatalf("Failed to create initial person: %v", err)
			}
			t.Logf("Created person ID %d with canonical_name=%q (email address)",
				initialPerson.ID, initialPerson.CanonicalName)

			// Verify initial state: canonical_name looks like an email (contains @)
			if !containsAt(initialPerson.CanonicalName) {
				t.Fatalf("Test setup issue: canonical_name %q doesn't look like an email",
					initialPerson.CanonicalName)
			}

			// Step 2: Call ResolveOrCreate with a display name
			// This simulates processing a new email from the same person that includes a display name
			result, err := resolver.ResolveOrCreate(ctx, tenantID, tc.email, tc.displayName)
			if err != nil {
				t.Fatalf("ResolveOrCreate failed: %v", err)
			}

			// Step 3: Assert that canonical_name was updated to the display name
			if result.Person.CanonicalName != tc.expectedCanonicalName {
				t.Errorf("FAIL: ResolveOrCreate returned canonical_name=%q, want %q",
					result.Person.CanonicalName, tc.expectedCanonicalName)
				t.Errorf("BUG REPRODUCED: canonical_name was NOT updated from email to display name")
				t.Logf("Person ID: %d", result.Person.ID)
				t.Logf("Email: %s", tc.email)
				t.Logf("Display name provided: %s", tc.displayName)
				t.Logf("Expected: canonical_name should be updated from %q to %q",
					tc.initialCanonicalName, tc.expectedCanonicalName)
			} else {
				t.Logf("PASS: canonical_name correctly updated to %q", result.Person.CanonicalName)
			}

			// Step 4: Verify the database was actually updated (not just the in-memory object)
			dbPerson, err := repo.GetPersonByEmail(ctx, tenantID, tc.email)
			if err != nil {
				t.Fatalf("Failed to fetch person from DB: %v", err)
			}
			if dbPerson.CanonicalName != tc.expectedCanonicalName {
				t.Errorf("FAIL: Database still has canonical_name=%q, want %q",
					dbPerson.CanonicalName, tc.expectedCanonicalName)
				t.Errorf("BUG: Changes were not persisted to database")
			}
		})
	}
}

// TestResolveOrCreate_CanonicalNameNoOverwrite_Integration verifies that we do NOT overwrite
// a good canonical_name (one that doesn't look like an email) with a new display name.
//
// This is an edge case test to ensure the fix for pf-38e4a9 doesn't break existing good names.
func TestResolveOrCreate_CanonicalNameNoOverwrite_Integration(t *testing.T) {
	ctx := context.Background()

	// Setup database connection
	pool := setupTestDB(t)
	tenantID := IntegrationTestTenantID
	logger := logging.MustGlobal()

	// Create repository and resolver
	repo := NewRepository(pool, logger)
	resolver := NewResolver(repo, WithResolverLogger(logger))

	testCases := []struct {
		name                  string
		email                 string
		existingCanonicalName string // Already a good name (not an email)
		newDisplayName        string // Different display name provided
		expectedCanonicalName string // Should NOT change
	}{
		{
			name:                  "good canonical name not overwritten by different display name",
			email:                 "mponec@test-integration.com",
			existingCanonicalName: "Miroslav Ponec", // Already a good name
			newDisplayName:        "M. Ponec",        // Different variation
			expectedCanonicalName: "Miroslav Ponec",  // Keep original
		},
		{
			name:                  "canonical name not overwritten when empty display name",
			email:                 "jsmith@test-integration.com",
			existingCanonicalName: "John Smith",
			newDisplayName:        "", // Empty display name
			expectedCanonicalName: "John Smith",
		},
		{
			name:                  "canonical name not overwritten by same value",
			email:                 "jane@test-integration.com",
			existingCanonicalName: "Jane Contact",
			newDisplayName:        "Jane Contact", // Same value
			expectedCanonicalName: "Jane Contact",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up any existing person with this email
			cleanupPerson(t, ctx, pool, tenantID, tc.email)
			defer cleanupPerson(t, ctx, pool, tenantID, tc.email)

			// Step 1: Create a person with a GOOD canonical_name (not an email)
			initialPerson := &Person{
				TenantID:      tenantID,
				CanonicalName: tc.existingCanonicalName, // Already a proper name
				PrimaryEmail:  tc.email,
				AccountType:   AccountTypePerson,
				IsInternal:    false,
				Confidence:    0.8,
				AutoCreated:   true,
				NeedsReview:   false,
			}

			if err := repo.CreatePerson(ctx, initialPerson); err != nil {
				t.Fatalf("Failed to create initial person: %v", err)
			}
			t.Logf("Created person ID %d with canonical_name=%q (good name)",
				initialPerson.ID, initialPerson.CanonicalName)

			// Step 2: Call ResolveOrCreate with a different display name
			result, err := resolver.ResolveOrCreate(ctx, tenantID, tc.email, tc.newDisplayName)
			if err != nil {
				t.Fatalf("ResolveOrCreate failed: %v", err)
			}

			// Step 3: Assert that canonical_name was NOT changed
			if result.Person.CanonicalName != tc.expectedCanonicalName {
				t.Errorf("FAIL: canonical_name changed from %q to %q, should stay unchanged",
					tc.existingCanonicalName, result.Person.CanonicalName)
				t.Errorf("We should NOT overwrite good canonical names with new display names")
			} else {
				t.Logf("PASS: canonical_name correctly preserved as %q", result.Person.CanonicalName)
			}

			// Step 4: Verify the database preserved the original name
			dbPerson, err := repo.GetPersonByEmail(ctx, tenantID, tc.email)
			if err != nil {
				t.Fatalf("Failed to fetch person from DB: %v", err)
			}
			if dbPerson.CanonicalName != tc.expectedCanonicalName {
				t.Errorf("FAIL: Database canonical_name changed to %q, want %q",
					dbPerson.CanonicalName, tc.expectedCanonicalName)
			}
		})
	}
}

// containsAt returns true if the string contains an @ symbol (simple email check).
func containsAt(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '@' {
			return true
		}
	}
	return false
}

// cleanupPerson removes a person by email from the database.
func cleanupPerson(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, email string) {
	t.Helper()

	// Delete person_aliases first (foreign key constraint)
	_, _ = pool.Exec(ctx, `
		DELETE FROM person_aliases
		WHERE person_id IN (SELECT id FROM people WHERE tenant_id = $1 AND primary_email = $2)
	`, tenantID, email)

	// Delete person
	_, _ = pool.Exec(ctx, `
		DELETE FROM people
		WHERE tenant_id = $1 AND primary_email = $2
	`, tenantID, email)
}
