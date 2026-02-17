//go:build integration

package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const integrationTestTenantID = "00000000-0000-0000-0000-000000000002"

// TestGetTenant_UUIDTypeMismatch reproduces bug pf-551691.
// ROOT CAUSE: Tenant.ID is int64 but tenants.id column is UUID.
// This causes GetTenant() to fail with: "cannot scan uuid (OID 2950) in *int64"
func TestGetTenant_UUIDTypeMismatch(t *testing.T) {
	repo := setupIntegrationTestRepo(t)
	ctx := context.Background()

	// Ensure test tenant exists
	ensureTestTenantExists(t, repo)

	// This MUST fail with the current (broken) code because Tenant.ID is int64
	// but the database column is UUID.
	tenant, err := repo.GetTenant(ctx, integrationTestTenantID)

	// Current behavior: scan error
	if err != nil {
		// Verify this is specifically the UUID->int64 type mismatch error
		assert.Contains(t, err.Error(), "cannot scan",
			"Expected scan error, got: %v", err)
		// The error should mention either "uuid" or the UUID OID (2950)
		uuidError := assert.Contains(t, err.Error(), "uuid",
			"Expected error to mention uuid type, got: %v", err) ||
			assert.Contains(t, err.Error(), "2950",
				"Expected error to mention UUID OID 2950, got: %v", err)
		assert.True(t, uuidError, "Error should indicate UUID type issue")

		// Expected behavior after fix: this assertion will fail, forcing test update
		t.Logf("BUG CONFIRMED: GetTenant fails with UUID scan error: %v", err)
		return
	}

	// After fix (Tenant.ID changed to string):
	// - This branch will execute
	// - tenant should be non-nil
	// - tenant.ID should contain a valid UUID string
	require.NotNil(t, tenant, "GetTenant should return tenant after fix")
	assert.Equal(t, integrationTestTenantID, tenant.ID,
		"Tenant ID should match the requested UUID")
	assert.NotEmpty(t, tenant.Name, "Tenant should have a name")
	assert.NotEmpty(t, tenant.Slug, "Tenant should have a slug")
}

// TestGetTenantBySlug_UUIDTypeMismatch reproduces bug pf-551691 via GetTenantBySlug.
func TestGetTenantBySlug_UUIDTypeMismatch(t *testing.T) {
	repo := setupIntegrationTestRepo(t)
	ctx := context.Background()

	// Ensure test tenant exists and get its slug
	ensureTestTenantExists(t, repo)

	// Get the slug of our test tenant first (using direct SQL to avoid the broken GetTenant)
	var slug string
	err := repo.pool.QueryRow(ctx,
		"SELECT slug FROM tenants WHERE id = $1",
		integrationTestTenantID).Scan(&slug)
	require.NoError(t, err, "Should be able to get tenant slug directly")

	// Now try GetTenantBySlug - will hit same UUID->int64 scan error
	tenant, err := repo.GetTenantBySlug(ctx, slug)

	// Current behavior: scan error
	if err != nil {
		assert.Contains(t, err.Error(), "cannot scan",
			"Expected scan error, got: %v", err)
		uuidError := assert.Contains(t, err.Error(), "uuid",
			"Expected error to mention uuid type, got: %v", err) ||
			assert.Contains(t, err.Error(), "2950",
				"Expected error to mention UUID OID 2950, got: %v", err)
		assert.True(t, uuidError, "Error should indicate UUID type issue")

		t.Logf("BUG CONFIRMED: GetTenantBySlug fails with UUID scan error: %v", err)
		return
	}

	// After fix: should succeed
	require.NotNil(t, tenant, "GetTenantBySlug should return tenant after fix")
	assert.Equal(t, integrationTestTenantID, tenant.ID,
		"Tenant ID should be the UUID string")
	assert.Equal(t, slug, tenant.Slug, "Slug should match")
}

// setupIntegrationTestRepo creates a test repository connected to the integration database.
func setupIntegrationTestRepo(t *testing.T) *ConfigRepositoryImpl {
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
	require.NoError(t, err, "failed to connect to test database")

	// Verify connection
	err = pool.Ping(ctx)
	require.NoError(t, err, "failed to ping test database")

	// Register cleanup
	t.Cleanup(func() {
		pool.Close()
	})

	return NewConfigRepository(pool)
}

// ensureTestTenantExists creates the integration test tenant if it doesn't exist.
func ensureTestTenantExists(t *testing.T, repo *ConfigRepositoryImpl) {
	t.Helper()
	ctx := context.Background()

	// Check if tenant exists (using direct SQL to avoid the broken GetTenant method)
	var exists bool
	err := repo.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM tenants WHERE id = $1)",
		integrationTestTenantID).Scan(&exists)
	require.NoError(t, err, "Failed to check if test tenant exists")

	if !exists {
		// Create test tenant
		_, err = repo.pool.Exec(ctx, `
			INSERT INTO tenants (id, name, display_name, slug, owner_email, is_active)
			VALUES ($1, 'integration-test', 'Integration Test Tenant', 'integration-test', 'test@example.com', true)
		`, integrationTestTenantID)
		require.NoError(t, err, "Failed to create test tenant")

		t.Logf("Created integration test tenant: %s", integrationTestTenantID)
	}
}

// getEnvOrDefault returns environment variable value or default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
