// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	entityv1 "github.com/otherjamesbrown/penf-cli/api/proto/entity/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Entity command flags.
var (
	entityTenant      string
	entityOutput      string
	entityReason      string
	entityRejectedBy  string
	entityEmailPattern string
	entityNamePattern  string
	entityEntityType   string
	entityCreatedBy    string
	entityField        string
	entityLimit        int
	// Pattern command flags
	entityPatternType  string
	entityPatternNotes string
	entityPatternValue string
	// Update command flags
	entityName        string
	entityAccountType string
	entityTitle       string
	entityCompany     string
	entityMetadata    map[string]string
	// Bulk-enrich command flags
	entityDomain     string
	entityIsInternal bool
	// Delete command flags
	entityForce bool
)

// EntityCommandDeps holds the dependencies for entity commands.
type EntityCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultEntityDeps returns the default dependencies for production use.
func DefaultEntityDeps() *EntityCommandDeps {
	return &EntityCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewEntityCommand creates the root entity command with all subcommands.
func NewEntityCommand(deps *EntityCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultEntityDeps()
	}

	cmd := &cobra.Command{
		Use:   "entity",
		Short: "Manage entity lifecycle (reject, restore, filter, stats, search)",
		Long: `Manage entity lifecycle in Penfold.

Entity lifecycle management allows you to:
  - Reject (soft-delete) entities that are spam, duplicates, or unwanted
  - Restore previously rejected entities
  - Create filter rules to automatically block entity creation by pattern
  - View statistics about entities in the system
  - Search entities by name or email

Examples:
  # Reject an entity
  penf entity reject 123 --reason "spam account"

  # Restore a rejected entity
  penf entity restore 123

  # Bulk reject by pattern
  penf entity reject --email-pattern "%@aha.io" --reason "Block Aha domain"

  # Create filter rule
  penf entity filter add --email-pattern "%noreply%" --reason "Block noreply addresses"

  # View entity statistics
  penf entity stats

  # Search entities
  penf entity search "john"`,
		Aliases: []string{"entities"},
	}

	// Add persistent flags.
	cmd.PersistentFlags().StringVarP(&entityTenant, "tenant", "t", "", "Tenant ID (overrides config)")
	cmd.PersistentFlags().StringVarP(&entityOutput, "output", "o", "", "Output format: text, json, yaml")

	// Add subcommands.
	cmd.AddCommand(newEntityRejectCommand(deps))
	cmd.AddCommand(newEntityRestoreCommand(deps))
	cmd.AddCommand(newEntityManagementDeleteCommand(deps))
	cmd.AddCommand(newEntityFilterCommand(deps))
	cmd.AddCommand(newEntityPatternCommand(deps))
	cmd.AddCommand(newEntityStatsCommand(deps))
	cmd.AddCommand(newEntitySearchCommand(deps))
	cmd.AddCommand(newEntityManagementUpdateCommand(deps))
	cmd.AddCommand(newEntityManagementBulkEnrichCommand(deps))

	return cmd
}

// newEntityRejectCommand creates the 'entity reject' subcommand.
func newEntityRejectCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reject [entity-id]",
		Short: "Reject (soft-delete) an entity",
		Long: `Reject an entity by ID or bulk reject by pattern.

Single entity rejection:
  penf entity reject <id> --reason "spam account"

Bulk rejection by pattern:
  penf entity reject --email-pattern "%@aha.io" --reason "Block Aha domain"
  penf entity reject --name-pattern "Bot%" --reason "Block bot accounts"

Examples:
  # Reject specific entity (accepts both numeric and prefixed format)
  penf entity reject 123 --reason "duplicate account"
  penf entity reject ent-person-123 --reason "duplicate account"

  # Bulk reject by email pattern
  penf entity reject --email-pattern "%noreply%" --reason "Block noreply addresses"

  # Bulk reject by name pattern
  penf entity reject --name-pattern "%Automated%" --reason "Block automated accounts"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				// Single entity rejection
				id, err := ParseEntityID(args[0])
				if err != nil {
					return fmt.Errorf("invalid entity ID: %s", args[0])
				}
				return runEntityReject(cmd.Context(), deps, id)
			}
			// Bulk rejection by pattern
			if entityEmailPattern == "" && entityNamePattern == "" {
				return fmt.Errorf("either entity ID or pattern (--email-pattern/--name-pattern) is required")
			}
			return runEntityBulkReject(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&entityReason, "reason", "", "Reason for rejection (required)")
	cmd.Flags().StringVar(&entityRejectedBy, "rejected-by", "", "User or service performing rejection")
	cmd.Flags().StringVar(&entityEmailPattern, "email-pattern", "", "Email pattern for bulk rejection (SQL LIKE format)")
	cmd.Flags().StringVar(&entityNamePattern, "name-pattern", "", "Name pattern for bulk rejection (SQL LIKE format)")
	cmd.MarkFlagRequired("reason")

	return cmd
}

// newEntityRestoreCommand creates the 'entity restore' subcommand.
func newEntityRestoreCommand(deps *EntityCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "restore <entity-id>",
		Short: "Restore a rejected entity",
		Long: `Restore a previously rejected entity.

This removes the rejection flag and makes the entity active again.

Examples:
  penf entity restore 123
  penf entity restore ent-person-123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := ParseEntityID(args[0])
			if err != nil {
				return fmt.Errorf("invalid entity ID: %s", args[0])
			}
			return runEntityRestore(cmd.Context(), deps, id)
		},
	}
}

// newEntityManagementDeleteCommand creates the 'entity delete' subcommand.
func newEntityManagementDeleteCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <entity-id>",
		Short: "Permanently delete an entity",
		Long: `Permanently delete an entity and all related records.

WARNING: This is a permanent deletion that cannot be undone. The entity and all
related data will be removed from the database.

For soft deletion, use 'penf entity reject' instead.

The --force flag skips the confirmation prompt for scripting purposes.`,
		Example: `  # Delete with confirmation prompt (accepts both numeric and prefixed format)
  penf entity delete 123
  penf entity delete ent-person-123

  # Delete without confirmation (for scripting)
  penf entity delete 123 --force`,
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := ParseEntityID(args[0])
			if err != nil {
				return fmt.Errorf("invalid entity ID: %s", args[0])
			}
			return runEntityManagementDelete(cmd.Context(), deps, id)
		},
	}

	cmd.Flags().BoolVar(&entityForce, "force", false, "Skip confirmation prompt")

	return cmd
}

// newEntityFilterCommand creates the 'entity filter' subcommand group.
func newEntityFilterCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filter",
		Short: "Manage entity filter rules",
		Long: `Manage filter rules that automatically block entity creation during pipeline processing.

Filter rules use SQL LIKE patterns to match email addresses or names.
When a new entity would be created during pipeline processing, it's checked against
all active filter rules. If a match is found, the entity creation is blocked.

Examples:
  # Add a filter rule
  penf entity filter add --email-pattern "%@aha.io" --reason "Block Aha domain"

  # List all filter rules
  penf entity filter list

  # Test if email/name matches any rules
  penf entity filter test --email "noreply@example.com" --name "Bot User"

  # Remove a filter rule
  penf entity filter remove 5`,
	}

	cmd.AddCommand(newEntityFilterAddCommand(deps))
	cmd.AddCommand(newEntityFilterListCommand(deps))
	cmd.AddCommand(newEntityFilterTestCommand(deps))
	cmd.AddCommand(newEntityFilterRemoveCommand(deps))

	return cmd
}

// newEntityPatternCommand creates the 'entity pattern' subcommand group.
func newEntityPatternCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pattern",
		Short: "Manage tenant email patterns",
		Long: `Manage tenant-specific email patterns for entity classification.

Email patterns help identify entity types during pipeline processing:
  - bot: Automated bot accounts (e.g., "bot@", "-bot-")
  - distribution_list: Group distribution lists (e.g., "team@", "all@")
  - role_account: Role-based accounts (e.g., "noreply@", "support@")
  - ignore: Emails to ignore during entity creation

Patterns are simple substring matches (case-insensitive).

Examples:
  # Add a bot pattern
  penf entity pattern add --pattern "bot@" --type bot --notes "Bot accounts"

  # List all patterns
  penf entity pattern list

  # Remove a pattern
  penf entity pattern remove 5`,
	}

	cmd.AddCommand(newEntityPatternAddCommand(deps))
	cmd.AddCommand(newEntityPatternListCommand(deps))
	cmd.AddCommand(newEntityPatternRemoveCommand(deps))

	return cmd
}

// newEntityPatternAddCommand creates the 'entity pattern add' subcommand.
func newEntityPatternAddCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new email pattern",
		Long: `Add a new tenant email pattern for entity classification.

Pattern types:
  - bot: Automated bot accounts
  - distribution_list: Group distribution lists
  - role_account: Role-based accounts
  - ignore: Emails to ignore during entity creation

Patterns are case-insensitive substring matches.

Examples:
  # Add a bot pattern
  penf entity pattern add --pattern "bot@" --type bot --notes "Bot accounts"

  # Add a distribution list pattern
  penf entity pattern add --pattern "team@" --type distribution_list --notes "Team lists"

  # Add a role account pattern
  penf entity pattern add --pattern "noreply@" --type role_account --notes "No-reply addresses"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEntityPatternAdd(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&entityPatternValue, "pattern", "", "Pattern string (required)")
	cmd.Flags().StringVar(&entityPatternType, "type", "", "Pattern type: bot, distribution_list, role_account, ignore (required)")
	cmd.Flags().StringVar(&entityPatternNotes, "notes", "", "Optional notes explaining the pattern")
	cmd.MarkFlagRequired("pattern")
	cmd.MarkFlagRequired("type")

	return cmd
}

// newEntityPatternListCommand creates the 'entity pattern list' subcommand.
func newEntityPatternListCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all email patterns",
		Long: `List all tenant email patterns.

Examples:
  penf entity pattern list
  penf entity pattern list --output json`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEntityPatternList(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&entityPatternType, "type", "", "Filter by pattern type (optional)")

	return cmd
}

// newEntityPatternRemoveCommand creates the 'entity pattern remove' subcommand.
func newEntityPatternRemoveCommand(deps *EntityCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <pattern-id>",
		Short: "Remove an email pattern",
		Long: `Remove a tenant email pattern by ID.

Use 'penf entity pattern list' to find pattern IDs.

Examples:
  penf entity pattern remove 5`,
		Aliases: []string{"rm", "delete"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			patternID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pattern ID: %s", args[0])
			}
			return runEntityPatternRemove(cmd.Context(), deps, patternID)
		},
	}
}

// newEntityFilterAddCommand creates the 'entity filter add' subcommand.
func newEntityFilterAddCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new filter rule",
		Long: `Add a new entity filter rule.

At least one pattern (email or name) is required.
Patterns use SQL LIKE format (% for wildcard, _ for single char).

Examples:
  # Block all noreply addresses
  penf entity filter add --email-pattern "%noreply%" --reason "Block noreply addresses"

  # Block specific domain
  penf entity filter add --email-pattern "%@aha.io" --reason "Block Aha domain"

  # Block bot accounts by name
  penf entity filter add --name-pattern "Bot%" --reason "Block bot accounts"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEntityFilterAdd(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&entityEmailPattern, "email-pattern", "", "Email pattern (SQL LIKE format)")
	cmd.Flags().StringVar(&entityNamePattern, "name-pattern", "", "Name pattern (SQL LIKE format)")
	cmd.Flags().StringVar(&entityEntityType, "entity-type", "", "Entity type filter (optional)")
	cmd.Flags().StringVar(&entityReason, "reason", "", "Reason for the rule (required)")
	cmd.Flags().StringVar(&entityCreatedBy, "created-by", "", "User or service creating the rule")
	cmd.MarkFlagRequired("reason")

	return cmd
}

// newEntityFilterListCommand creates the 'entity filter list' subcommand.
func newEntityFilterListCommand(deps *EntityCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all filter rules",
		Long: `List all entity filter rules for the current tenant.

Examples:
  penf entity filter list
  penf entity filter list --output json`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEntityFilterList(cmd.Context(), deps)
		},
	}
}

// newEntityFilterTestCommand creates the 'entity filter test' subcommand.
func newEntityFilterTestCommand(deps *EntityCommandDeps) *cobra.Command {
	var email, name string

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test if an email/name would match filter rules",
		Long: `Test if a given email or name would be blocked by any filter rules.

Examples:
  # Test an email
  penf entity filter test --email "noreply@example.com"

  # Test a name
  penf entity filter test --name "Bot User"

  # Test both
  penf entity filter test --email "bot@example.com" --name "Automated Bot"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEntityFilterTest(cmd.Context(), deps, email, name)
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Email to test")
	cmd.Flags().StringVar(&name, "name", "", "Name to test")

	return cmd
}

// newEntityFilterRemoveCommand creates the 'entity filter remove' subcommand.
func newEntityFilterRemoveCommand(deps *EntityCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <rule-id>",
		Short: "Remove a filter rule",
		Long: `Remove an entity filter rule by ID.

Use 'penf entity filter list' to find rule IDs.

Examples:
  penf entity filter remove 5`,
		Aliases: []string{"rm", "delete"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ruleID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid rule ID: %s", args[0])
			}
			return runEntityFilterRemove(cmd.Context(), deps, ruleID)
		},
	}
}

// newEntityStatsCommand creates the 'entity stats' subcommand.
func newEntityStatsCommand(deps *EntityCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show entity statistics",
		Long: `Show statistics about entities in the system.

Displays breakdown by account type, confidence, and lifecycle status.

Examples:
  penf entity stats
  penf entity stats --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEntityStats(cmd.Context(), deps)
		},
	}
}

// newEntitySearchCommand creates the 'entity search' subcommand.
func newEntitySearchCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search entities by name or email",
		Long: `Search for entities by name or email substring.

By default, searches both name and email fields.
Use --field to limit search to specific field.

Examples:
  # Search both fields
  penf entity search "john"

  # Search only email
  penf entity search "example.com" --field email

  # Search only name
  penf entity search "smith" --field name

  # Limit results
  penf entity search "john" --limit 50`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEntitySearch(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVar(&entityField, "field", "", "Field to search: name, email, or empty for both")
	cmd.Flags().IntVar(&entityLimit, "limit", 100, "Maximum number of results")

	return cmd
}

// ==================== gRPC Connection ====================

func connectEntityToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	// Configure transport credentials based on TLS settings.
	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if cfg.TLS.Enabled {
		tlsConfig, err := client.LoadClientTLSConfig(&cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("loading TLS config: %w", err)
		}
		if tlsConfig != nil {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
	} else {
		// Default to insecure if TLS not explicitly configured.
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, cfg.ServerAddress, opts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to gateway at %s: %w", cfg.ServerAddress, err)
	}

	return conn, nil
}

func getTenantIDForEntity(deps *EntityCommandDeps) (string, error) {
	if entityTenant != "" {
		return entityTenant, nil
	}
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant, nil
	}
	if deps.Config != nil && deps.Config.TenantID != "" {
		return deps.Config.TenantID, nil
	}
	return "", fmt.Errorf("tenant ID required: set --tenant flag, PENF_TENANT_ID env var, or tenant_id in config")
}

// ==================== Command Execution Functions ====================

func runEntityReject(ctx context.Context, deps *EntityCommandDeps, entityID int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	_, err = client.RejectEntity(ctx, &entityv1.RejectEntityRequest{
		TenantId:   tenantID,
		EntityId:   entityID,
		Reason:     entityReason,
		RejectedBy: entityRejectedBy,
	})
	if err != nil {
		return fmt.Errorf("rejecting entity: %w", err)
	}

	fmt.Printf("Rejected entity ID %d: %s\n", entityID, entityReason)
	return nil
}

func runEntityRestore(ctx context.Context, deps *EntityCommandDeps, entityID int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	_, err = client.RestoreEntity(ctx, &entityv1.RestoreEntityRequest{
		TenantId: tenantID,
		EntityId: entityID,
	})
	if err != nil {
		return fmt.Errorf("restoring entity: %w", err)
	}

	fmt.Printf("Restored entity ID %d\n", entityID)
	return nil
}

func runEntityManagementDelete(ctx context.Context, deps *EntityCommandDeps, entityID int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Prompt for confirmation unless --force is set
	if !entityForce {
		fmt.Printf("WARNING: This will permanently delete entity ID %d and all related records.\n", entityID)
		fmt.Print("This action cannot be undone. Continue? (yes/no): ")

		var response string
		fmt.Scanln(&response)

		if response != "yes" && response != "y" {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	resp, err := client.DeleteEntity(ctx, &entityv1.DeleteEntityRequest{
		TenantId: tenantID,
		EntityId: entityID,
	})
	if err != nil {
		return fmt.Errorf("deleting entity: %w", err)
	}

	fmt.Printf("Deleted entity ID %d: %s\n", resp.EntityId, resp.Message)
	return nil
}

func runEntityBulkReject(ctx context.Context, deps *EntityCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	resp, err := client.BulkRejectEntities(ctx, &entityv1.BulkRejectEntitiesRequest{
		TenantId:     tenantID,
		EmailPattern: entityEmailPattern,
		NamePattern:  entityNamePattern,
		Reason:       entityReason,
		RejectedBy:   entityRejectedBy,
	})
	if err != nil {
		return fmt.Errorf("bulk rejecting entities: %w", err)
	}

	fmt.Printf("Rejected %d entities: %s\n", resp.Count, entityReason)
	return nil
}

func runEntityFilterAdd(ctx context.Context, deps *EntityCommandDeps) error {
	if entityEmailPattern == "" && entityNamePattern == "" {
		return fmt.Errorf("at least one pattern (--email-pattern or --name-pattern) is required")
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	resp, err := client.CreateFilterRule(ctx, &entityv1.CreateFilterRuleRequest{
		TenantId:     tenantID,
		EmailPattern: entityEmailPattern,
		NamePattern:  entityNamePattern,
		EntityType:   entityEntityType,
		Reason:       entityReason,
		CreatedBy:    entityCreatedBy,
	})
	if err != nil {
		return fmt.Errorf("creating filter rule: %w", err)
	}

	fmt.Printf("Created filter rule ID %d\n", resp.Rule.Id)
	if entityEmailPattern != "" {
		fmt.Printf("  Email pattern: %s\n", entityEmailPattern)
	}
	if entityNamePattern != "" {
		fmt.Printf("  Name pattern: %s\n", entityNamePattern)
	}
	fmt.Printf("  Reason: %s\n", entityReason)

	return nil
}

func runEntityFilterList(ctx context.Context, deps *EntityCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	resp, err := client.ListFilterRules(ctx, &entityv1.ListFilterRulesRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return fmt.Errorf("listing filter rules: %w", err)
	}

	return outputFilterRules(cfg, resp.Rules)
}

func runEntityFilterTest(ctx context.Context, deps *EntityCommandDeps, email, name string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	resp, err := client.TestFilterRule(ctx, &entityv1.TestFilterRuleRequest{
		TenantId: tenantID,
		Email:    email,
		Name:     name,
	})
	if err != nil {
		return fmt.Errorf("testing filter rules: %w", err)
	}

	if resp.WouldBeBlocked {
		fmt.Printf("BLOCKED: This email/name matches %d filter rule(s)\n\n", len(resp.MatchingRules))
		return outputFilterRules(cfg, resp.MatchingRules)
	}

	fmt.Println("OK: No matching filter rules found")
	return nil
}

func runEntityFilterRemove(ctx context.Context, deps *EntityCommandDeps, ruleID int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	_, err = client.DeleteFilterRule(ctx, &entityv1.DeleteFilterRuleRequest{
		TenantId: tenantID,
		RuleId:   ruleID,
	})
	if err != nil {
		return fmt.Errorf("deleting filter rule: %w", err)
	}

	fmt.Printf("Deleted filter rule ID %d\n", ruleID)
	return nil
}

func runEntityPatternAdd(ctx context.Context, deps *EntityCommandDeps) error {
	if entityPatternValue == "" {
		return fmt.Errorf("--pattern flag is required")
	}
	if entityPatternType == "" {
		return fmt.Errorf("--type flag is required")
	}

	// Validate pattern type
	validTypes := map[string]bool{
		"bot":               true,
		"distribution_list": true,
		"role_account":      true,
		"ignore":            true,
	}
	if !validTypes[entityPatternType] {
		return fmt.Errorf("invalid pattern type: %s (must be one of: bot, distribution_list, role_account, ignore)", entityPatternType)
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	resp, err := client.CreateEmailPattern(ctx, &entityv1.CreateEmailPatternRequest{
		TenantId:    tenantID,
		Pattern:     entityPatternValue,
		PatternType: entityPatternType,
		Notes:       entityPatternNotes,
	})
	if err != nil {
		return fmt.Errorf("creating email pattern: %w", err)
	}

	fmt.Printf("Created email pattern ID %d\n", resp.Pattern.Id)
	fmt.Printf("  Pattern: %s\n", entityPatternValue)
	fmt.Printf("  Type: %s\n", entityPatternType)
	if entityPatternNotes != "" {
		fmt.Printf("  Notes: %s\n", entityPatternNotes)
	}

	return nil
}

func runEntityPatternList(ctx context.Context, deps *EntityCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	resp, err := client.ListEmailPatterns(ctx, &entityv1.ListEmailPatternsRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return fmt.Errorf("listing email patterns: %w", err)
	}

	// Filter by type if specified
	patterns := resp.Patterns
	if entityPatternType != "" {
		filtered := []*entityv1.EmailPattern{}
		for _, p := range patterns {
			if p.PatternType == entityPatternType {
				filtered = append(filtered, p)
			}
		}
		patterns = filtered
	}

	return outputEmailPatterns(cfg, patterns)
}

func runEntityPatternRemove(ctx context.Context, deps *EntityCommandDeps, patternID int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	_, err = client.DeleteEmailPattern(ctx, &entityv1.DeleteEmailPatternRequest{
		TenantId:  tenantID,
		PatternId: patternID,
	})
	if err != nil {
		return fmt.Errorf("deleting email pattern: %w", err)
	}

	fmt.Printf("Deleted email pattern ID %d\n", patternID)
	return nil
}

func runEntityStats(ctx context.Context, deps *EntityCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	resp, err := client.GetEntityStats(ctx, &entityv1.GetEntityStatsRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return fmt.Errorf("getting entity stats: %w", err)
	}

	return outputEntityStats(cfg, resp)
}

func runEntitySearch(ctx context.Context, deps *EntityCommandDeps, query string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	resp, err := client.SearchEntities(ctx, &entityv1.SearchEntitiesRequest{
		TenantId: tenantID,
		Query:    query,
		Field:    entityField,
		Limit:    int32(entityLimit),
	})
	if err != nil {
		return fmt.Errorf("searching entities: %w", err)
	}

	return outputEntitySearchResults(cfg, resp.People)
}

// newEntityManagementUpdateCommand creates the 'entity update' subcommand.
func newEntityManagementUpdateCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <entity-id>",
		Short: "Update entity fields",
		Long: `Update specific fields of an entity.

You can update name, account type, title, company, and metadata.`,
		Example: `  # Update entity title and company (accepts both numeric and prefixed format)
  penf entity update 123 --title "VP Engineering" --company "Akamai"
  penf entity update ent-person-123 --title "VP Engineering" --company "Akamai"

  # Update entity name
  penf entity update 123 --name "John Smith"

  # Update account type
  penf entity update 123 --account-type "person"

  # Update metadata
  penf entity update 123 --metadata "linkedin=https://linkedin.com/in/john"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := ParseEntityID(args[0])
			if err != nil {
				return fmt.Errorf("invalid entity ID: %s", args[0])
			}
			return runEntityManagementUpdate(cmd.Context(), deps, id)
		},
	}

	cmd.Flags().StringVar(&entityName, "name", "", "New entity name")
	cmd.Flags().StringVar(&entityAccountType, "account-type", "", "New account type (person, role, distribution, bot, external_service, team, service)")
	cmd.Flags().StringVar(&entityTitle, "title", "", "New job title")
	cmd.Flags().StringVar(&entityCompany, "company", "", "New company name")
	cmd.Flags().StringToStringVar(&entityMetadata, "metadata", nil, "Metadata key=value pairs")

	return cmd
}

// newEntityManagementBulkEnrichCommand creates the 'entity bulk-enrich' subcommand.
func newEntityManagementBulkEnrichCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk-enrich",
		Short: "Bulk enrich entities by domain",
		Long: `Enrich entities by email domain, setting company and is_internal flag.

This command updates all entities with email addresses in the specified domain.`,
		Example: `  # Set company for all akamai.com entities
  penf entity bulk-enrich --domain akamai.com --company Akamai

  # Mark all internal domain entities as internal
  penf entity bulk-enrich --domain mycompany.com --company "My Company" --internal

  # Set company without marking as internal
  penf entity bulk-enrich --domain example.com --company Example`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEntityManagementBulkEnrich(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&entityDomain, "domain", "", "Email domain to match (required)")
	cmd.Flags().StringVar(&entityCompany, "company", "", "Company name to set")
	cmd.Flags().BoolVar(&entityIsInternal, "internal", false, "Mark entities as internal")
	cmd.MarkFlagRequired("domain")

	return cmd
}

func runEntityManagementUpdate(ctx context.Context, deps *EntityCommandDeps, entityID int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	// At least one field must be specified
	if entityName == "" && entityAccountType == "" && entityTitle == "" && entityCompany == "" && len(entityMetadata) == 0 {
		return fmt.Errorf("at least one field (--name, --account-type, --title, --company, or --metadata) must be specified")
	}

	// Build request with optional fields
	req := &entityv1.UpdateEntityRequest{
		TenantId: tenantID,
		EntityId: entityID,
		Metadata: entityMetadata,
	}

	if entityName != "" {
		req.Name = &entityName
	}
	if entityAccountType != "" {
		req.AccountType = &entityAccountType
	}
	if entityTitle != "" {
		req.Title = &entityTitle
	}
	if entityCompany != "" {
		req.Company = &entityCompany
	}

	resp, err := client.UpdateEntity(ctx, req)
	if err != nil {
		return fmt.Errorf("updating entity: %w", err)
	}

	fmt.Printf("Updated entity ID %d: %s\n", resp.EntityId, resp.Message)
	return nil
}

func runEntityManagementBulkEnrich(ctx context.Context, deps *EntityCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	resp, err := client.BulkEnrichEntities(ctx, &entityv1.BulkEnrichEntitiesRequest{
		TenantId:   tenantID,
		Domain:     entityDomain,
		Company:    entityCompany,
		IsInternal: entityIsInternal,
	})
	if err != nil {
		return fmt.Errorf("bulk enriching entities: %w", err)
	}

	fmt.Printf("Enriched %d entities in domain %s\n", resp.Count, entityDomain)
	if entityCompany != "" {
		fmt.Printf("  Set company: %s\n", entityCompany)
	}
	if entityIsInternal {
		fmt.Printf("  Marked as internal\n")
	}
	return nil
}

// ==================== Output Functions ====================

func getEntityOutputFormat(cfg *config.CLIConfig) config.OutputFormat {
	if entityOutput != "" {
		return config.OutputFormat(entityOutput)
	}
	if cfg != nil {
		return cfg.OutputFormat
	}
	return config.OutputFormatText
}

func outputFilterRules(cfg *config.CLIConfig, rules []*entityv1.FilterRule) error {
	format := getEntityOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputJSON(rules)
	case config.OutputFormatYAML:
		return outputYAML(rules)
	default:
		return outputFilterRulesTable(rules)
	}
}

func outputFilterRulesTable(rules []*entityv1.FilterRule) error {
	if len(rules) == 0 {
		fmt.Println("No filter rules found.")
		return nil
	}

	fmt.Printf("Filter Rules (%d):\n\n", len(rules))
	fmt.Println("  ID      EMAIL PATTERN                 NAME PATTERN                  REASON")
	fmt.Println("  --      -------------                 ------------                  ------")

	for _, rule := range rules {
		emailPat := "-"
		if rule.EmailPattern != "" {
			emailPat = truncate(rule.EmailPattern, 30)
		}
		namePat := "-"
		if rule.NamePattern != "" {
			namePat = truncate(rule.NamePattern, 30)
		}
		fmt.Printf("  %-6d  %-30s %-30s %s\n",
			rule.Id,
			emailPat,
			namePat,
			truncate(rule.Reason, 40))
	}

	fmt.Println()
	return nil
}

func outputEntityStats(cfg *config.CLIConfig, stats *entityv1.GetEntityStatsResponse) error {
	format := getEntityOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputJSON(stats)
	case config.OutputFormatYAML:
		return outputYAML(stats)
	default:
		return outputEntityStatsText(stats)
	}
}

func outputEntityStatsText(stats *entityv1.GetEntityStatsResponse) error {
	fmt.Println("Entity Statistics:")
	fmt.Println()
	fmt.Printf("  Total People:      %d\n", stats.TotalPeople)
	fmt.Printf("  Total Rejected:    %d\n", stats.TotalRejected)
	fmt.Printf("  Needing Review:    %d\n", stats.NeedingReview)
	fmt.Printf("  Auto-Created:      %d\n", stats.AutoCreated)
	fmt.Printf("  Internal:          %d\n", stats.Internal)
	fmt.Printf("  External:          %d\n", stats.External)
	fmt.Println()

	fmt.Println("By Account Type:")
	for accountType, count := range stats.ByAccountType {
		fmt.Printf("  %-20s %d\n", accountType, count)
	}
	fmt.Println()

	fmt.Println("By Confidence:")
	fmt.Printf("  High (0.8+):       %d\n", stats.ByConfidence["high"])
	fmt.Printf("  Medium (0.5-0.8):  %d\n", stats.ByConfidence["medium"])
	fmt.Printf("  Low (<0.5):        %d\n", stats.ByConfidence["low"])
	fmt.Println()

	return nil
}

func outputEntitySearchResults(cfg *config.CLIConfig, people []*entityv1.Person) error {
	format := getEntityOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputJSON(people)
	case config.OutputFormatYAML:
		return outputYAML(people)
	default:
		return outputEntitySearchResultsTable(people)
	}
}

func outputEntitySearchResultsTable(people []*entityv1.Person) error {
	if len(people) == 0 {
		fmt.Println("No entities found.")
		return nil
	}

	fmt.Printf("Search Results (%d):\n\n", len(people))
	fmt.Println("  ID      NAME                           EMAIL                          TITLE")
	fmt.Println("  --      ----                           -----                          -----")

	for _, p := range people {
		title := "-"
		if p.Title != "" {
			title = truncate(p.Title, 30)
		}
		fmt.Printf("  %-6d  %-30s %-30s %s\n",
			p.Id,
			truncate(p.Name, 30),
			truncate(p.Email, 30),
			title)
	}

	fmt.Println()
	return nil
}

func outputEmailPatterns(cfg *config.CLIConfig, patterns []*entityv1.EmailPattern) error {
	format := getEntityOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputJSON(patterns)
	case config.OutputFormatYAML:
		return outputYAML(patterns)
	default:
		return outputEmailPatternsTable(patterns)
	}
}

func outputEmailPatternsTable(patterns []*entityv1.EmailPattern) error {
	if len(patterns) == 0 {
		fmt.Println("No email patterns found.")
		return nil
	}

	fmt.Printf("Email Patterns (%d):\n\n", len(patterns))
	fmt.Println("  ID      PATTERN                        TYPE                 PRIORITY  ENABLED")
	fmt.Println("  --      -------                        ----                 --------  -------")

	for _, p := range patterns {
		enabled := "yes"
		if !p.Enabled {
			enabled = "no"
		}
		fmt.Printf("  %-6d  %-30s %-20s %-9d %s\n",
			p.Id,
			truncate(p.Pattern, 30),
			truncate(p.PatternType, 20),
			p.Priority,
			enabled)
	}

	fmt.Println()
	return nil
}
