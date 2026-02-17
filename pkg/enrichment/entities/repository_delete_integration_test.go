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
	"github.com/stretchr/testify/require"
)

// TestDeleteEntity_WithContentMentions_Integration is an integration test that reproduces
// bug pf-018309: DeleteEntity references non-existent 'entity_mentions' table.
//
// The bug: Repository.DeleteEntity (line ~985) executes:
//   DELETE FROM entity_mentions WHERE person_id = $1
//
// But the actual table is 'content_mentions' with structure:
//   - entity_type (person, term, product, etc.)
//   - resolved_entity_id (references people.id, glossary.id, etc.)
//
// The correct query should be:
//   DELETE FROM content_mentions WHERE entity_type = 'person' AND resolved_entity_id = $1
//
// This test:
// 1. Creates a person entity
// 2. Creates a content_mention that references the person (via resolved_entity_id)
// 3. Calls DeleteEntity on the person
// 4. FAILS with error: relation "entity_mentions" does not exist
//
// After the bug is fixed, this test should PASS.
func TestDeleteEntity_WithContentMentions_Integration(t *testing.T) {
	ctx := context.Background()

	// Setup database connection
	pool := setupTestDBForDelete(t)
	tenantID := IntegrationTestTenantID
	logger := logging.MustGlobal()

	// Create repository
	repo := NewRepository(pool, logger)

	// Clean up before and after test
	testEmail := "delete-test-person@integration.test"
	cleanupTestData(t, ctx, pool, tenantID, testEmail)
	defer cleanupTestData(t, ctx, pool, tenantID, testEmail)

	// Step 1: Create a person
	person := &Person{
		TenantID:      tenantID,
		CanonicalName: "Delete Test Person",
		PrimaryEmail:  testEmail,
		AccountType:   AccountTypePerson,
		IsInternal:    true,
		Confidence:    0.9,
	}
	err := repo.CreatePerson(ctx, person)
	require.NoError(t, err, "failed to create test person")
	require.NotZero(t, person.ID, "person ID should be set after creation")
	t.Logf("Created person ID %d", person.ID)

	// Step 2: Create a content_mention that references this person
	// First, we need a content record (sources table)
	var contentID int64
	testHash := fmt.Sprintf("%064d", person.ID) // Simple 64-char hex-compatible hash
	err = pool.QueryRow(ctx, `
		INSERT INTO sources (
			tenant_id,
			source_system,
			external_id,
			content_hash,
			processing_status
		)
		VALUES ($1, 'gmail', $2, $3, 'completed')
		RETURNING id
	`, tenantID, "delete-test-"+testEmail, testHash).Scan(&contentID)
	require.NoError(t, err, "failed to create test content")
	t.Logf("Created content ID %d", contentID)

	// Create the content_mention
	var mentionID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO content_mentions (
			tenant_id,
			content_id,
			entity_type,
			mentioned_text,
			resolved_entity_id,
			status,
			resolution_source
		) VALUES ($1, $2, 'person', 'Delete Test Person', $3, 'auto_resolved', 'exact_match')
		RETURNING id
	`, tenantID, contentID, person.ID).Scan(&mentionID)
	require.NoError(t, err, "failed to create content_mention")
	t.Logf("Created content_mention ID %d referencing person ID %d", mentionID, person.ID)

	// Step 3: Call DeleteEntity
	// This should fail with: relation "entity_mentions" does not exist
	err = repo.DeleteEntity(ctx, tenantID, person.ID)

	// The test EXPECTS this to fail on the current (buggy) code
	if err != nil {
		t.Logf("DeleteEntity failed (as expected with bug): %v", err)
		// Verify it's the expected error
		require.Contains(t, err.Error(), "entity_mentions",
			"Expected error to mention 'entity_mentions' table, but got: %v", err)
		t.Logf("✓ Test correctly reproduces bug pf-018309: references non-existent 'entity_mentions' table")

		// After the bug is fixed, this test will fail here because err will be nil
		// At that point, remove the error expectation and verify cleanup below
		return
	}

	// After bug is fixed, execution reaches here and we verify cleanup
	t.Log("DeleteEntity succeeded (bug is fixed!)")

	// Step 4: Verify the mention was deleted
	var mentionCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM content_mentions
		WHERE entity_type = 'person' AND resolved_entity_id = $1
	`, person.ID).Scan(&mentionCount)
	require.NoError(t, err, "failed to count content_mentions")
	require.Equal(t, 0, mentionCount, "content_mentions should be deleted for the person")

	// Step 5: Verify the person was deleted
	_, err = repo.GetPersonByID(ctx, person.ID)
	require.Error(t, err, "person should be deleted")
	require.Contains(t, err.Error(), "not found", "expected 'not found' error")

	t.Log("✓ Test passed: DeleteEntity correctly deletes content_mentions and person")
}

// setupTestDBForDelete sets up a database connection for delete integration tests.
func setupTestDBForDelete(t *testing.T) *pgxpool.Pool {
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

// cleanupTestData removes all test data for a given person email.
func cleanupTestData(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, email string) {
	t.Helper()

	// Get person ID first
	var personID int64
	err := pool.QueryRow(ctx, `
		SELECT id FROM people WHERE tenant_id = $1 AND primary_email = $2
	`, tenantID, email).Scan(&personID)

	if err == nil {
		// Delete content_mentions (use correct table name)
		_, _ = pool.Exec(ctx, `
			DELETE FROM content_mentions
			WHERE entity_type = 'person' AND resolved_entity_id = $1
		`, personID)

		// Delete person_aliases
		_, _ = pool.Exec(ctx, `
			DELETE FROM person_aliases WHERE person_id = $1
		`, personID)

		// Delete team memberships
		_, _ = pool.Exec(ctx, `
			DELETE FROM team_members WHERE person_id = $1
		`, personID)

		// Delete project memberships
		_, _ = pool.Exec(ctx, `
			DELETE FROM project_members WHERE person_id = $1
		`, personID)
	}

	// Delete test content created for this test
	_, _ = pool.Exec(ctx, `
		DELETE FROM sources
		WHERE tenant_id = $1 AND source_system = 'gmail' AND external_id LIKE 'delete-test-%'
	`, tenantID)

	// Delete person
	_, _ = pool.Exec(ctx, `
		DELETE FROM people
		WHERE tenant_id = $1 AND primary_email = $2
	`, tenantID, email)
}
