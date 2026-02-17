package enrichment

import (
	"testing"
)

// TestRepository_Create_Duplicate_BUG_pf_6c7c8f reproduces bug pf-6c7c8f part 2 (pf-ce213a):
// repository.go Create method uses plain INSERT without ON CONFLICT.
// On reprocess (when enrichment record already exists), UNIQUE constraint violation occurs.
// The error is returned but treated as non-blocking by the workflow.
//
// This test requires a real database connection and SHOULD FAIL with a constraint violation error.
// To run: go test ./pkg/enrichment/... -run TestRepository_Create_Duplicate_BUG_pf_6c7c8f -v
//
// NOTE: This is an integration test that requires DATABASE_URL to be set.
// It will be skipped if DB is not available.
func TestRepository_Create_Duplicate_BUG_pf_6c7c8f(t *testing.T) {
	t.Skip("REPRODUCTION TEST: This test requires a real database. " +
		"Run manually with DATABASE_URL set to reproduce Bug 2 (pf-ce213a). " +
		"Expected failure: UNIQUE constraint violation on second Create() call for same source_id.")

	// To run this test manually:
	// 1. Set DATABASE_URL environment variable
	// 2. Uncomment the code below
	// 3. Run: go test ./pkg/enrichment/... -run TestRepository_Create_Duplicate_BUG_pf_6c7c8f -v
	//
	// Expected behavior:
	// - First Create() should succeed
	// - Second Create() with same source_id should fail with UNIQUE constraint violation
	// - After fix: Second Create() should return nil error and populate the enrichment from existing row

	/*
		// Get database connection from environment
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			t.Skip("DATABASE_URL not set, skipping integration test")
		}

		pool, err := pgxpool.New(context.Background(), dbURL)
		require.NoError(t, err, "Failed to connect to database")
		defer pool.Close()

		logger := logging.NewNopLogger()
		repo := NewRepository(pool, logger)

		// Create first enrichment record
		enrichment1 := &Enrichment{
			SourceID: 999999, // Use a high ID to avoid conflicts
			TenantID: "test-tenant-bug-pf-6c7c8f",
			Classification: Classification{
				ContentType: ContentTypeEmail,
				Subtype:     SubtypeEmailThread,
				Profile:     ProfileStandard,
				Confidence:  0.95,
			},
			SourceSystem:  "test",
			Status:        StatusPending,
			CurrentStage:  "test",
			Participants:  nil,
			ResolvedParticipants: nil,
			ExtractedLinks: nil,
			ExtractedData: make(map[string]interface{}),
		}

		err = repo.Create(context.Background(), enrichment1)
		require.NoError(t, err, "First Create should succeed")
		require.NotZero(t, enrichment1.ID, "Enrichment ID should be set after Create")

		// Try to create duplicate enrichment record (same source_id)
		enrichment2 := &Enrichment{
			SourceID: 999999, // Same source_id
			TenantID: "test-tenant-bug-pf-6c7c8f",
			Classification: Classification{
				ContentType: ContentTypeEmail,
				Subtype:     SubtypeEmailThread,
				Profile:     ProfileStandard,
				Confidence:  0.95,
			},
			SourceSystem:  "test",
			Status:        StatusPending,
			CurrentStage:  "test",
			Participants:  nil,
			ResolvedParticipants: nil,
			ExtractedLinks: nil,
			ExtractedData: make(map[string]interface{}),
		}

		// BUG: This will fail with UNIQUE constraint violation
		// EXPECTED (AFTER FIX): Should handle ON CONFLICT and return nil error
		err = repo.Create(context.Background(), enrichment2)
		require.NoError(t, err, "BUG: Second Create with same source_id should not error after fix (ON CONFLICT DO NOTHING)")
		require.NotZero(t, enrichment2.ID, "Enrichment ID should be populated from existing row")

		// Cleanup
		_, _ = pool.Exec(context.Background(), "DELETE FROM content_enrichment WHERE source_id = 999999")
	*/
}

// TestRepository_Create_Description documents the expected behavior after Bug 2 fix.
func TestRepository_Create_Description(t *testing.T) {
	// This is a documentation test that describes the expected behavior.
	// It doesn't actually run code but serves as specification.

	t.Log("Bug 2 (pf-ce213a): CreateEnrichmentRecord upsert")
	t.Log("")
	t.Log("Current behavior:")
	t.Log("  - repository.go Create() uses plain INSERT")
	t.Log("  - On duplicate source_id, returns UNIQUE constraint violation error")
	t.Log("  - Workflow treats error as non-blocking")
	t.Log("  - ThreadGrouper then tries to UPDATE content_enrichment")
	t.Log("  - UPDATE affects 0 rows (enrichment doesn't exist)")
	t.Log("  - Threading data is lost")
	t.Log("")
	t.Log("Expected behavior after fix:")
	t.Log("  - repository.go Create() uses INSERT ... ON CONFLICT (source_id) DO NOTHING")
	t.Log("  - On duplicate source_id:")
	t.Log("    1. INSERT is skipped (no error)")
	t.Log("    2. Query existing row by source_id")
	t.Log("    3. Populate enrichment struct from existing row")
	t.Log("    4. Return nil error")
	t.Log("  - Workflow continues normally")
	t.Log("  - ThreadGrouper UPDATE succeeds (enrichment exists)")
	t.Log("  - Threading data is preserved")
	t.Log("")
	t.Log("Reproduction:")
	t.Log("  1. Process email (creates enrichment record)")
	t.Log("  2. Reprocess same email (tries to create duplicate)")
	t.Log("  3. Without fix: constraint violation, threading lost")
	t.Log("  4. With fix: no error, threading preserved")
}
