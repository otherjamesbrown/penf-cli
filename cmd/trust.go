// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	watchlistv1 "github.com/otherjamesbrown/penf-cli/api/proto/watchlist/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Trust command flags.
var (
	trustTenant  string
	trustOutput  string
	trustLevel   int32
	trustDomains []string
)

// Seniority command flags.
var (
	seniorityTenant string
	seniorityOutput string
	seniorityTier   int32
)

// TrustCommandDeps holds the dependencies for trust commands.
type TrustCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultTrustDeps returns the default dependencies for production use.
func DefaultTrustDeps() *TrustCommandDeps {
	return &TrustCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// SeniorityCommandDeps holds the dependencies for seniority commands.
type SeniorityCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultSeniorityDeps returns the default dependencies for production use.
func DefaultSeniorityDeps() *SeniorityCommandDeps {
	return &SeniorityCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewTrustCommand creates the root trust command with all subcommands.
func NewTrustCommand(deps *TrustCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultTrustDeps()
	}

	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Manage person trust levels and domains",
		Long: `Manage trust levels and domains for people in Penfold.

Trust levels (0-5) indicate how much weight to give to a person's statements:
  0 - No trust (default)
  1 - Low trust
  2 - Medium trust
  3 - High trust
  4 - Very high trust
  5 - Maximum trust

Trust domains are optional categories like:
  - technical-risk
  - timeline
  - security
  - architecture

Examples:
  # Set trust level for a person
  penf trust set 123 --level 4

  # Set trust level with specific domains
  penf trust set 123 --level 4 --domains technical-risk,timeline

  # Clear trust for a person
  penf trust clear 123

Related Commands:
  penf seniority             Set organizational seniority (1-7)
  penf relationship entity   Manage the person entity itself
  penf briefing              Uses trust levels for assertion priority ranking`,
	}

	// Add persistent flags.
	cmd.PersistentFlags().StringVarP(&trustTenant, "tenant", "t", "", "Tenant ID (overrides config)")
	cmd.PersistentFlags().StringVarP(&trustOutput, "output", "o", "", "Output format: text, json, yaml")

	// Add subcommands.
	cmd.AddCommand(newTrustSetCommand(deps))
	cmd.AddCommand(newTrustClearCommand(deps))

	return cmd
}

// newTrustSetCommand creates the 'trust set' subcommand.
func newTrustSetCommand(deps *TrustCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <person_id>",
		Short: "Set trust level and domains for a person",
		Long: `Set trust level and optional domains for a person.

Trust level must be 0-5. Domains are optional comma-separated categories.

Examples:
  # Set trust level
  penf trust set 123 --level 4

  # Set trust level with domains
  penf trust set 123 --level 4 --domains technical-risk,timeline`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTrustSet(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().Int32Var(&trustLevel, "level", 0, "Trust level (0-5, required)")
	cmd.Flags().StringSliceVar(&trustDomains, "domains", nil, "Trust domains (comma-separated)")
	cmd.MarkFlagRequired("level")

	return cmd
}

// newTrustClearCommand creates the 'trust clear' subcommand.
func newTrustClearCommand(deps *TrustCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "clear <person_id>",
		Short: "Clear trust for a person",
		Long: `Clear trust for a person by setting trust level to 0 and clearing all domains.

Example:
  penf trust clear 123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTrustClear(cmd.Context(), deps, args[0])
		},
	}
}

// NewSeniorityCommand creates the root seniority command with all subcommands.
func NewSeniorityCommand(deps *SeniorityCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultSeniorityDeps()
	}

	cmd := &cobra.Command{
		Use:   "seniority",
		Short: "Manage person seniority tiers",
		Long: `Manage seniority tiers for people in Penfold.

Seniority tiers (1-7) indicate organizational level:
  1 - IC (Individual Contributor)
  2 - Senior IC
  3 - Staff IC
  4 - Manager
  5 - Senior Manager
  6 - Director
  7 - VP/Executive

Examples:
  # Set seniority tier for a person
  penf seniority set 123 --tier 5

  # Clear seniority for a person
  penf seniority clear 123

Related Commands:
  penf trust                 Set trust levels and domains (0-5)
  penf relationship entity   Manage the person entity itself
  penf escalations           Detect seniority escalations in content`,
	}

	// Add persistent flags.
	cmd.PersistentFlags().StringVarP(&seniorityTenant, "tenant", "t", "", "Tenant ID (overrides config)")
	cmd.PersistentFlags().StringVarP(&seniorityOutput, "output", "o", "", "Output format: text, json, yaml")

	// Add subcommands.
	cmd.AddCommand(newSenioritySetCommand(deps))
	cmd.AddCommand(newSeniorityClearCommand(deps))

	return cmd
}

// newSenioritySetCommand creates the 'seniority set' subcommand.
func newSenioritySetCommand(deps *SeniorityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <person_id>",
		Short: "Set seniority tier for a person",
		Long: `Set seniority tier for a person.

Seniority tier must be 1-7.

Example:
  penf seniority set 123 --tier 5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSenioritySet(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().Int32Var(&seniorityTier, "tier", 0, "Seniority tier (1-7, required)")
	cmd.MarkFlagRequired("tier")

	return cmd
}

// newSeniorityClearCommand creates the 'seniority clear' subcommand.
func newSeniorityClearCommand(deps *SeniorityCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "clear <person_id>",
		Short: "Clear seniority for a person",
		Long: `Clear seniority for a person by setting tier to 0.

Example:
  penf seniority clear 123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSeniorityClear(cmd.Context(), deps, args[0])
		},
	}
}

// ==================== gRPC Connection ====================

// connectTrustToGateway creates a gRPC connection to the gateway service.
func connectTrustToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

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
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, cfg.ServerAddress, opts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to gateway at %s: %w", cfg.ServerAddress, err)
	}

	return conn, nil
}

// getTenantIDForTrust returns the tenant ID from flag, env, or config.
func getTenantIDForTrust() string {
	if trustTenant != "" {
		return trustTenant
	}
	if seniorityTenant != "" {
		return seniorityTenant
	}
	// Default tenant.
	return "00000001-0000-0000-0000-000000000001"
}

// ==================== Command Execution Functions ====================

// runTrustSet executes the trust set command via gRPC.
func runTrustSet(ctx context.Context, deps *TrustCommandDeps, personIDStr string) error {
	// Validate trust level.
	if trustLevel < 0 || trustLevel > 5 {
		return fmt.Errorf("trust level must be 0-5, got: %d", trustLevel)
	}

	// Parse person ID.
	personID, err := strconv.ParseInt(personIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid person ID: %s", personIDStr)
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectTrustToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := watchlistv1.NewWatchListServiceClient(conn)
	tenantID := getTenantIDForTrust()

	// Clean up domains.
	var cleanDomains []string
	for _, d := range trustDomains {
		d = strings.TrimSpace(d)
		if d != "" {
			cleanDomains = append(cleanDomains, d)
		}
	}

	resp, err := client.SetTrust(ctx, &watchlistv1.SetTrustRequest{
		TenantId:     tenantID,
		PersonId:     personID,
		TrustLevel:   trustLevel,
		TrustDomains: cleanDomains,
	})
	if err != nil {
		return fmt.Errorf("setting trust: %w", err)
	}

	fmt.Printf("\033[32mSet trust:\033[0m %s (ID: %d)\n", resp.Person.Name, resp.Person.Id)
	fmt.Printf("  Trust level: %d\n", resp.Person.TrustLevel)
	if len(resp.Person.TrustDomains) > 0 {
		fmt.Printf("  Domains: %s\n", strings.Join(resp.Person.TrustDomains, ", "))
	}

	// Reset flags for next call.
	trustLevel = 0
	trustDomains = nil

	return nil
}

// runTrustClear executes the trust clear command via gRPC.
func runTrustClear(ctx context.Context, deps *TrustCommandDeps, personIDStr string) error {
	// Parse person ID.
	personID, err := strconv.ParseInt(personIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid person ID: %s", personIDStr)
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectTrustToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := watchlistv1.NewWatchListServiceClient(conn)
	tenantID := getTenantIDForTrust()

	resp, err := client.SetTrust(ctx, &watchlistv1.SetTrustRequest{
		TenantId:     tenantID,
		PersonId:     personID,
		TrustLevel:   0,
		TrustDomains: nil,
	})
	if err != nil {
		return fmt.Errorf("clearing trust: %w", err)
	}

	fmt.Printf("\033[32mCleared trust:\033[0m %s (ID: %d)\n", resp.Person.Name, resp.Person.Id)
	return nil
}

// runSenioritySet executes the seniority set command via gRPC.
func runSenioritySet(ctx context.Context, deps *SeniorityCommandDeps, personIDStr string) error {
	// Validate seniority tier.
	if seniorityTier < 1 || seniorityTier > 7 {
		return fmt.Errorf("seniority tier must be 1-7, got: %d", seniorityTier)
	}

	// Parse person ID.
	personID, err := strconv.ParseInt(personIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid person ID: %s", personIDStr)
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectTrustToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := watchlistv1.NewWatchListServiceClient(conn)
	tenantID := getTenantIDForTrust()

	resp, err := client.SetSeniority(ctx, &watchlistv1.SetSeniorityRequest{
		TenantId:      tenantID,
		PersonId:      personID,
		SeniorityTier: seniorityTier,
	})
	if err != nil {
		return fmt.Errorf("setting seniority: %w", err)
	}

	fmt.Printf("\033[32mSet seniority:\033[0m %s (ID: %d)\n", resp.Person.Name, resp.Person.Id)
	fmt.Printf("  Seniority tier: %d\n", resp.Person.SeniorityTier)
	if resp.Person.Title != "" {
		fmt.Printf("  Title: %s\n", resp.Person.Title)
	}

	// Reset flags for next call.
	seniorityTier = 0

	return nil
}

// runSeniorityClear executes the seniority clear command via gRPC.
func runSeniorityClear(ctx context.Context, deps *SeniorityCommandDeps, personIDStr string) error {
	// Parse person ID.
	personID, err := strconv.ParseInt(personIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid person ID: %s", personIDStr)
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectTrustToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := watchlistv1.NewWatchListServiceClient(conn)
	tenantID := getTenantIDForTrust()

	resp, err := client.SetSeniority(ctx, &watchlistv1.SetSeniorityRequest{
		TenantId:      tenantID,
		PersonId:      personID,
		SeniorityTier: 0,
	})
	if err != nil {
		return fmt.Errorf("clearing seniority: %w", err)
	}

	fmt.Printf("\033[32mCleared seniority:\033[0m %s (ID: %d)\n", resp.Person.Name, resp.Person.Id)
	return nil
}
