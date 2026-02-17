// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/config"
	"github.com/otherjamesbrown/penf-cli/pkg/db"
)

// Database command flags
var (
	dbDryRun      bool
	dbTarget      string
	dbOutput      string
	dbMigrationDir string
)

// DbCommandDeps holds the dependencies for database commands.
type DbCommandDeps struct {
	Config       *config.CLIConfig
	LoadConfig   func() (*config.CLIConfig, error)
	ConnectToDB  func(context.Context, *config.CLIConfig) (*pgxpool.Pool, error)
}

// DefaultDbDeps returns the default dependencies for production use.
func DefaultDbDeps() *DbCommandDeps {
	return &DbCommandDeps{
		LoadConfig:  config.LoadConfig,
		ConnectToDB: connectToDatabase,
	}
}

// NewDbCommand creates the root db command with all subcommands.
func NewDbCommand() *cobra.Command {
	deps := DefaultDbDeps()

	cmd := &cobra.Command{
		Use:   "db",
		Short: "Database management commands",
		Long: `Database management commands for Penfold.

Manage database schema migrations and view migration status.

The db command connects directly to the PostgreSQL database to run migrations
and check status. It requires DATABASE_URL or DB_* environment variables to be set.

Migration files are SQL files in the migrations directory, named with numeric
prefixes (e.g., 001_create_users.sql, 002_add_indexes.sql). Migrations are
applied in alphabetical order and tracked in the schema_migrations table.

Examples:
  # Show migration status
  penf db status

  # Apply all pending migrations
  penf db migrate

  # Preview migrations without applying
  penf db migrate --dry-run

  # Apply migrations up to a specific version
  penf db migrate --target 040`,
		Aliases: []string{"database", "migrations"},
	}

	// Add persistent flags
	cmd.PersistentFlags().StringVarP(&dbMigrationDir, "migrations", "m", "migrations", "Path to migrations directory")

	// Add subcommands
	cmd.AddCommand(newDbMigrateCommand(deps))
	cmd.AddCommand(newDbStatusCommand(deps))

	return cmd
}

// newDbMigrateCommand creates the 'db migrate' subcommand.
func newDbMigrateCommand(deps *DbCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Apply database migrations",
		Long: `Apply pending database migrations.

Shows pending migrations before applying them. Migrations are executed in
alphabetical order based on their filename prefix. Each migration is run
in a transaction, and the migration is recorded in schema_migrations table.

If a migration fails, the transaction is rolled back and no further migrations
are attempted.

Flags:
  --dry-run      Show what would be applied without executing migrations
  --target       Apply migrations up to and including this version (e.g., 040)
  --migrations   Path to migrations directory (default: migrations)

Examples:
  # Apply all pending migrations
  penf db migrate

  # Preview migrations without applying
  penf db migrate --dry-run

  # Apply migrations up to version 040
  penf db migrate --target 040

  # Use a custom migrations directory
  penf db migrate --migrations ./db/migrations`,
		Example: `  penf db migrate
  penf db migrate --dry-run
  penf db migrate --target 040`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDbMigrate(cmd.Context(), deps)
		},
	}

	cmd.Flags().BoolVar(&dbDryRun, "dry-run", false, "Show what would be applied without executing")
	cmd.Flags().StringVarP(&dbTarget, "target", "t", "", "Target version to migrate to (e.g., 040)")

	return cmd
}

// newDbStatusCommand creates the 'db status' subcommand.
func newDbStatusCommand(deps *DbCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show database migration status",
		Long: `Show the current state of database migrations.

Displays three categories of migrations:
  - Applied: migrations that have been applied and have corresponding files
  - Pending: migrations with files that have not been applied yet
  - Drift: migrations that were applied but no longer have corresponding files

The status command helps identify which migrations need to be applied and
detect any drift between the filesystem and the database.

Flags:
  --output       Output format: text, json, yaml (default: text)
  --format       Alias for --output
  --migrations   Path to migrations directory (default: migrations)

Examples:
  # Show migration status
  penf db status

  # Output as JSON for programmatic use
  penf db status --output json

  # Check status with custom migrations directory
  penf db status --migrations ./db/migrations`,
		Example: `  penf db status
  penf db status --output json
  penf db status --format yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDbStatus(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&dbOutput, "output", "o", "", "Output format: text, json, yaml")
	cmd.Flags().StringVarP(&dbOutput, "format", "f", "", "Output format: text, json, yaml (alias for --output)")

	return cmd
}

// runDbMigrate executes the db migrate command.
func runDbMigrate(ctx context.Context, deps *DbCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Connect to database
	pool, err := deps.ConnectToDB(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	// Get pending migrations first
	pending, err := db.GetPendingMigrations(ctx, pool, dbMigrationDir)
	if err != nil {
		return fmt.Errorf("getting pending migrations: %w", err)
	}

	if len(pending) == 0 {
		fmt.Println("No pending migrations.")
		return nil
	}

	// Show pending migrations
	fmt.Printf("Pending migrations (%d):\n", len(pending))
	for _, m := range pending {
		fmt.Printf("  %s - %s\n", m.Version, m.Name)
	}
	fmt.Println()

	if dbDryRun {
		fmt.Println("Dry run mode: no migrations applied.")
		return nil
	}

	// Confirm before applying
	fmt.Print("Apply these migrations? (y/N): ")
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(response) != "y" {
		fmt.Println("Migration cancelled.")
		return nil
	}

	// Apply migrations
	var result *db.MigrationResult
	if dbTarget != "" {
		fmt.Printf("Applying migrations up to version %s...\n", dbTarget)
		result, err = db.RunMigrationsToTarget(ctx, pool, dbMigrationDir, dbTarget)
	} else {
		fmt.Println("Applying all pending migrations...")
		result, err = db.RunMigrations(ctx, pool, dbMigrationDir)
	}

	if err != nil {
		fmt.Printf("\n\033[31mMigration failed:\033[0m %v\n", err)
		if len(result.Applied) > 0 {
			fmt.Printf("\nSuccessfully applied before failure:\n")
			for _, v := range result.Applied {
				fmt.Printf("  \033[32m✓\033[0m %s\n", v)
			}
		}
		return err
	}

	// Show results
	fmt.Println()
	if len(result.Applied) > 0 {
		fmt.Printf("\033[32mSuccessfully applied %d migration(s):\033[0m\n", len(result.Applied))
		for _, v := range result.Applied {
			fmt.Printf("  \033[32m✓\033[0m %s\n", v)
		}
	}
	if len(result.Skipped) > 0 {
		fmt.Printf("\nSkipped %d migration(s) (already applied):\n", len(result.Skipped))
		for _, v := range result.Skipped {
			fmt.Printf("  - %s\n", v)
		}
	}

	fmt.Println()
	fmt.Println("\033[32mMigrations completed successfully.\033[0m")
	return nil
}

// runDbStatus executes the db status command.
func runDbStatus(ctx context.Context, deps *DbCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Connect to database
	pool, err := deps.ConnectToDB(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	// Get migration status
	status, err := db.GetMigrationStatus(ctx, pool, dbMigrationDir)
	if err != nil {
		return fmt.Errorf("getting migration status: %w", err)
	}

	// Determine output format
	format := cfg.OutputFormat
	if dbOutput != "" {
		format = config.OutputFormat(dbOutput)
	}

	return outputMigrationStatus(format, status)
}

// outputMigrationStatus formats and outputs migration status.
func outputMigrationStatus(format config.OutputFormat, status *db.MigrationStatus) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(status)
	default:
		return outputMigrationStatusText(status)
	}
}

// outputMigrationStatusText formats migration status for terminal display.
func outputMigrationStatusText(status *db.MigrationStatus) error {
	// Applied migrations
	if len(status.Applied) > 0 {
		fmt.Printf("\033[32mApplied Migrations (%d):\033[0m\n", len(status.Applied))
		fmt.Println("  VERSION                    NAME                              APPLIED")
		fmt.Println("  -------                    ----                              -------")
		for _, m := range status.Applied {
			appliedAt := "-"
			if m.AppliedAt != nil {
				appliedAt = m.AppliedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("  %-26s %-33s %s\n",
				truncateDbString(m.Version, 26),
				truncateDbString(m.Name, 33),
				appliedAt)
		}
		fmt.Println()
	}

	// Pending migrations
	if len(status.Pending) > 0 {
		fmt.Printf("\033[33mPending Migrations (%d):\033[0m\n", len(status.Pending))
		fmt.Println("  VERSION                    NAME")
		fmt.Println("  -------                    ----")
		for _, m := range status.Pending {
			fmt.Printf("  %-26s %s\n",
				truncateDbString(m.Version, 26),
				m.Name)
		}
		fmt.Println()
	}

	// Drift (migrations applied but files missing)
	if len(status.Drift) > 0 {
		fmt.Printf("\033[31mDrift (%d) - applied but file missing:\033[0m\n", len(status.Drift))
		fmt.Println("  VERSION                    NAME                              APPLIED")
		fmt.Println("  -------                    ----                              -------")
		for _, m := range status.Drift {
			appliedAt := "-"
			if m.AppliedAt != nil {
				appliedAt = m.AppliedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("  %-26s %-33s %s\n",
				truncateDbString(m.Version, 26),
				truncateDbString(m.Name, 33),
				appliedAt)
		}
		fmt.Println()
	}

	// Summary
	if len(status.Applied) == 0 && len(status.Pending) == 0 && len(status.Drift) == 0 {
		fmt.Println("No migrations found.")
		return nil
	}

	fmt.Printf("Summary: %d applied, %d pending",
		len(status.Applied), len(status.Pending))
	if len(status.Drift) > 0 {
		fmt.Printf(", \033[31m%d drift\033[0m", len(status.Drift))
	}
	fmt.Println()

	return nil
}

// truncateDbString truncates a string to maxLen, adding "..." if truncated.
func truncateDbString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
