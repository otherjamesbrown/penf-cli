package entities

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

// TestIncrementSentCount verifies that the IncrementSentCount method exists and increments
// the sent_count column on a person entity.
//
// This test verifies requirement pf-0f8878: message count tracking on person entities.
//
// Expected behavior:
//   - IncrementSentCount(personID) increments sent_count by 1
//   - sent_count starts at 0 for new entities
//   - Multiple calls accumulate the count
func TestIncrementSentCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test")
	}

	ctx := context.Background()

	// Connect to test database
	pool, err := pgxpool.New(ctx, getTestDatabaseURL())
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer pool.Close()

	logger := logging.MustGlobal()
	repo := NewRepository(pool, logger)

	// Create a test person
	person := &Person{
		TenantID:      "test-tenant",
		CanonicalName: "Test Person",
		PrimaryEmail:  "test@example.com",
		IsInternal:    false,
		AccountType:   AccountTypePerson,
		Confidence:    0.8,
		NeedsReview:   false,
		AutoCreated:   true,
	}

	if err := repo.CreatePerson(ctx, person); err != nil {
		t.Fatalf("Failed to create test person: %v", err)
	}
	defer cleanupPerson(t, ctx, pool, person.ID)

	// Verify sent_count starts at 0
	fetched, err := repo.GetPersonByID(ctx, person.ID)
	if err != nil {
		t.Fatalf("Failed to fetch person: %v", err)
	}
	if fetched.SentCount != 0 {
		t.Errorf("Expected initial sent_count=0, got %d", fetched.SentCount)
	}

	// Test: Increment sent count once
	if err := repo.IncrementSentCount(ctx, person.ID); err != nil {
		t.Fatalf("IncrementSentCount failed: %v", err)
	}

	// Verify sent_count is now 1
	fetched, err = repo.GetPersonByID(ctx, person.ID)
	if err != nil {
		t.Fatalf("Failed to fetch person after increment: %v", err)
	}
	if fetched.SentCount != 1 {
		t.Errorf("Expected sent_count=1 after first increment, got %d", fetched.SentCount)
	}

	// Test: Increment sent count again
	if err := repo.IncrementSentCount(ctx, person.ID); err != nil {
		t.Fatalf("IncrementSentCount failed on second call: %v", err)
	}

	// Verify sent_count is now 2
	fetched, err = repo.GetPersonByID(ctx, person.ID)
	if err != nil {
		t.Fatalf("Failed to fetch person after second increment: %v", err)
	}
	if fetched.SentCount != 2 {
		t.Errorf("Expected sent_count=2 after second increment, got %d", fetched.SentCount)
	}

	// Test: Increment sent count a third time
	if err := repo.IncrementSentCount(ctx, person.ID); err != nil {
		t.Fatalf("IncrementSentCount failed on third call: %v", err)
	}

	// Verify sent_count is now 3
	fetched, err = repo.GetPersonByID(ctx, person.ID)
	if err != nil {
		t.Fatalf("Failed to fetch person after third increment: %v", err)
	}
	if fetched.SentCount != 3 {
		t.Errorf("Expected sent_count=3 after third increment, got %d", fetched.SentCount)
	}
}

// TestIncrementReceivedCount verifies that the IncrementReceivedCount method exists and increments
// the received_count column on a person entity.
//
// This test verifies requirement pf-0f8878: message count tracking on person entities.
//
// Expected behavior:
//   - IncrementReceivedCount(personID) increments received_count by 1
//   - received_count starts at 0 for new entities
//   - Multiple calls accumulate the count
func TestIncrementReceivedCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test")
	}

	ctx := context.Background()

	// Connect to test database
	pool, err := pgxpool.New(ctx, getTestDatabaseURL())
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer pool.Close()

	logger := logging.MustGlobal()
	repo := NewRepository(pool, logger)

	// Create a test person
	person := &Person{
		TenantID:      "test-tenant",
		CanonicalName: "Test Recipient",
		PrimaryEmail:  "recipient@example.com",
		IsInternal:    false,
		AccountType:   AccountTypePerson,
		Confidence:    0.8,
		NeedsReview:   false,
		AutoCreated:   true,
	}

	if err := repo.CreatePerson(ctx, person); err != nil {
		t.Fatalf("Failed to create test person: %v", err)
	}
	defer cleanupPerson(t, ctx, pool, person.ID)

	// Verify received_count starts at 0
	fetched, err := repo.GetPersonByID(ctx, person.ID)
	if err != nil {
		t.Fatalf("Failed to fetch person: %v", err)
	}
	if fetched.ReceivedCount != 0 {
		t.Errorf("Expected initial received_count=0, got %d", fetched.ReceivedCount)
	}

	// Test: Increment received count once
	if err := repo.IncrementReceivedCount(ctx, person.ID); err != nil {
		t.Fatalf("IncrementReceivedCount failed: %v", err)
	}

	// Verify received_count is now 1
	fetched, err = repo.GetPersonByID(ctx, person.ID)
	if err != nil {
		t.Fatalf("Failed to fetch person after increment: %v", err)
	}
	if fetched.ReceivedCount != 1 {
		t.Errorf("Expected received_count=1 after first increment, got %d", fetched.ReceivedCount)
	}

	// Test: Increment received count again
	if err := repo.IncrementReceivedCount(ctx, person.ID); err != nil {
		t.Fatalf("IncrementReceivedCount failed on second call: %v", err)
	}

	// Verify received_count is now 2
	fetched, err = repo.GetPersonByID(ctx, person.ID)
	if err != nil {
		t.Fatalf("Failed to fetch person after second increment: %v", err)
	}
	if fetched.ReceivedCount != 2 {
		t.Errorf("Expected received_count=2 after second increment, got %d", fetched.ReceivedCount)
	}

	// Test: Increment received count a third time
	if err := repo.IncrementReceivedCount(ctx, person.ID); err != nil {
		t.Fatalf("IncrementReceivedCount failed on third call: %v", err)
	}

	// Verify received_count is now 3
	fetched, err = repo.GetPersonByID(ctx, person.ID)
	if err != nil {
		t.Fatalf("Failed to fetch person after third increment: %v", err)
	}
	if fetched.ReceivedCount != 3 {
		t.Errorf("Expected received_count=3 after third increment, got %d", fetched.ReceivedCount)
	}
}

// TestIncrementMessageCounts_Independent verifies that sent_count and received_count
// are independent and can be incremented separately.
//
// This test verifies requirement pf-0f8878: message count tracking on person entities.
//
// Expected behavior:
//   - Incrementing sent_count does not affect received_count
//   - Incrementing received_count does not affect sent_count
//   - Both counts can be incremented on the same person
func TestIncrementMessageCounts_Independent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test")
	}

	ctx := context.Background()

	// Connect to test database
	pool, err := pgxpool.New(ctx, getTestDatabaseURL())
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer pool.Close()

	logger := logging.MustGlobal()
	repo := NewRepository(pool, logger)

	// Create a test person
	person := &Person{
		TenantID:      "test-tenant",
		CanonicalName: "Test Person Both Counts",
		PrimaryEmail:  "both@example.com",
		IsInternal:    false,
		AccountType:   AccountTypePerson,
		Confidence:    0.8,
		NeedsReview:   false,
		AutoCreated:   true,
	}

	if err := repo.CreatePerson(ctx, person); err != nil {
		t.Fatalf("Failed to create test person: %v", err)
	}
	defer cleanupPerson(t, ctx, pool, person.ID)

	// Increment sent_count twice
	if err := repo.IncrementSentCount(ctx, person.ID); err != nil {
		t.Fatalf("IncrementSentCount failed: %v", err)
	}
	if err := repo.IncrementSentCount(ctx, person.ID); err != nil {
		t.Fatalf("IncrementSentCount failed on second call: %v", err)
	}

	// Increment received_count three times
	if err := repo.IncrementReceivedCount(ctx, person.ID); err != nil {
		t.Fatalf("IncrementReceivedCount failed: %v", err)
	}
	if err := repo.IncrementReceivedCount(ctx, person.ID); err != nil {
		t.Fatalf("IncrementReceivedCount failed on second call: %v", err)
	}
	if err := repo.IncrementReceivedCount(ctx, person.ID); err != nil {
		t.Fatalf("IncrementReceivedCount failed on third call: %v", err)
	}

	// Verify both counts are independent
	fetched, err := repo.GetPersonByID(ctx, person.ID)
	if err != nil {
		t.Fatalf("Failed to fetch person: %v", err)
	}

	if fetched.SentCount != 2 {
		t.Errorf("Expected sent_count=2, got %d", fetched.SentCount)
	}
	if fetched.ReceivedCount != 3 {
		t.Errorf("Expected received_count=3, got %d", fetched.ReceivedCount)
	}
}

// Helper functions for test setup/teardown

func getTestDatabaseURL() string {
	// TODO: Read from environment or test config
	return "postgres://postgres:postgres@localhost:5432/penfold_test?sslmode=disable"
}

func cleanupPerson(t *testing.T, ctx context.Context, pool *pgxpool.Pool, personID int64) {
	_, err := pool.Exec(ctx, "DELETE FROM people WHERE id = $1", personID)
	if err != nil {
		t.Logf("Warning: Failed to cleanup test person %d: %v", personID, err)
	}
}
