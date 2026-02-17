package entities

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigration003_PeopleTableColumns tests that migration 003 defines
// the people table with column names that match repository.go expectations.
//
// Bug pf-a7f451: Migration 003 creates people table with column 'title',
// but repository.go references 'job_title' in 11 places. This test reproduces
// the bug by verifying that migration 003 defines 'job_title' (not 'title').
//
// Expected to FAIL until migration 044 renames title -> job_title.
func TestMigration003_PeopleTableColumns(t *testing.T) {
	// Find the migrations directory relative to this test file
	projectRoot := findProjectRoot(t)
	migrationPath := filepath.Join(projectRoot, "migrations", "003_entity_resolution.sql")

	// Read migration file
	content, err := os.ReadFile(migrationPath)
	require.NoError(t, err, "Failed to read migration 003")

	migrationSQL := string(content)

	// Extract the people table definition
	peopleTableSQL := extractPeopleTableDefinition(t, migrationSQL)

	// Test: Migration should define 'job_title' column (not 'title')
	// This test FAILS because migration 003 currently defines 'title'
	t.Run("people table should define job_title column", func(t *testing.T) {
		// Check for job_title column definition
		hasJobTitle := containsColumnDefinition(peopleTableSQL, "job_title")

		// Check for incorrect 'title' column definition
		hasTitle := containsColumnDefinition(peopleTableSQL, "title")

		// Assert: Should have job_title, NOT title
		assert.True(t, hasJobTitle,
			"Migration 003 should define 'job_title' column in people table (repository.go expects 'job_title')")
		assert.False(t, hasTitle,
			"Migration 003 should NOT define 'title' column (repository.go expects 'job_title', not 'title')")

		// Additional diagnostic info
		if !hasJobTitle && hasTitle {
			t.Logf("REPRODUCES BUG pf-a7f451: Migration defines 'title' but repository.go uses 'job_title'")
			t.Logf("See repository.go lines: 39, 92, 110, 128, 153, 179, 209, 239, 965")
		}
	})
}

// extractPeopleTableDefinition extracts the CREATE TABLE people block from migration SQL.
func extractPeopleTableDefinition(t *testing.T, migrationSQL string) string {
	t.Helper()

	// Find CREATE TABLE people ... up to the closing );
	// Pattern: CREATE TABLE ... people ( ... );
	pattern := regexp.MustCompile(`(?s)CREATE TABLE[^;]*people\s*\((.*?)\);`)
	matches := pattern.FindStringSubmatch(migrationSQL)

	require.NotEmpty(t, matches, "Failed to find CREATE TABLE people in migration 003")
	require.Len(t, matches, 2, "Unexpected regex match count")

	return matches[1] // Return the table definition (between parentheses)
}

// containsColumnDefinition checks if SQL contains a column definition.
// Matches patterns like: "column_name TYPE" or "column_name TYPE,"
func containsColumnDefinition(tableSQL, columnName string) bool {
	// Match column definition patterns:
	// - "title VARCHAR" or "job_title VARCHAR"
	// - Column name must be at start of line or after comma/whitespace
	// - Followed by a SQL type keyword

	// Pattern: column name followed by SQL type (VARCHAR, TEXT, BOOLEAN, etc.)
	pattern := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(columnName) + `\s+\w+`)

	return pattern.MatchString(tableSQL)
}

// findProjectRoot walks up from the test file to find the project root.
// Project root is identified by the presence of go.work or migrations directory.
func findProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from the directory of this test file
	dir, err := os.Getwd()
	require.NoError(t, err, "Failed to get working directory")

	// Walk up until we find go.work (project root) or migrations directory
	for {
		// Check for migrations directory (most reliable indicator)
		migrationsPath := filepath.Join(dir, "migrations")
		if stat, err := os.Stat(migrationsPath); err == nil && stat.IsDir() {
			return dir
		}

		// Check for go.work file (project root for workspace)
		goWorkPath := filepath.Join(dir, "go.work")
		if _, err := os.Stat(goWorkPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding project root
			t.Fatal("Failed to find project root (no go.work or migrations/ found)")
		}
		dir = parent
	}
}


// TestRepositorySQL_JobTitleReferences verifies that repository.go references
// 'job_title' in SQL queries (not 'title'). This proves the repository expects
// the column to be named 'job_title'.
//
// This test PASSES (confirms repository uses 'job_title').
func TestRepositorySQL_JobTitleReferences(t *testing.T) {
	projectRoot := findProjectRoot(t)
	repositoryPath := filepath.Join(projectRoot, "pkg", "enrichment", "entities", "repository.go")

	content, err := os.ReadFile(repositoryPath)
	require.NoError(t, err, "Failed to read repository.go")

	repoCode := string(content)

	t.Run("repository.go uses job_title in SQL", func(t *testing.T) {
		// Count occurrences of 'job_title' in SQL contexts
		jobTitleCount := strings.Count(repoCode, "job_title")

		// Per bug report: job_title appears in 11 places (lines 39, 92, 110, 128, 153, 179, 209, 239, 965, 1410, 1427, 1445, 1480)
		assert.Greater(t, jobTitleCount, 10,
			"repository.go should reference 'job_title' at least 11 times")

		t.Logf("Found %d references to 'job_title' in repository.go", jobTitleCount)
	})

	t.Run("repository.go should not use bare 'title' column", func(t *testing.T) {
		// Check for SQL patterns that reference a 'title' column
		// We need to distinguish 'title' (column) from 'job_title' (column)

		// Look for patterns like: "title," "title as" "title =" but NOT "job_title"
		titlePattern := regexp.MustCompile(`\btitle\s*(,|as|=|\s+\w+)`)
		jobTitlePattern := regexp.MustCompile(`\bjob_title\b`)

		// Find all title matches
		titleMatches := titlePattern.FindAllString(repoCode, -1)

		// Filter out matches that are actually 'job_title'
		var bareTitle []string
		for _, match := range titleMatches {
			if !jobTitlePattern.MatchString(match) {
				bareTitle = append(bareTitle, match)
			}
		}

		// There might be some 'title' references in comments or struct fields,
		// but SQL queries should use 'job_title'
		t.Logf("Found %d potential bare 'title' references (may include comments/struct fields)", len(bareTitle))

		// The key assertion: SQL should use 'job_title', not 'title'
		// This test is informational - the main bug is in migration 003
	})
}
