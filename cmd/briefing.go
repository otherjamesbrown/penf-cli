// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"

	projectv1 "github.com/otherjamesbrown/penf-cli/api/proto/project/v1"
	watchlistv1 "github.com/otherjamesbrown/penf-cli/api/proto/watchlist/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Briefing command flags.
var (
	briefingTier     int32
	briefingLimit    int32
	briefingOutput   string
	escalationSource int64
)

// BriefingCommandDeps holds the dependencies for briefing commands.
type BriefingCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultBriefingDeps returns the default dependencies for production use.
func DefaultBriefingDeps() *BriefingCommandDeps {
	return &BriefingCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewBriefingCommand creates the briefing command.
func NewBriefingCommand(deps *BriefingCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultBriefingDeps()
	}

	cmd := &cobra.Command{
		Use:   "briefing <project_name>",
		Short: "Show priority-ordered assertions for a project",
		Long: `Show priority-ordered assertions for a project.

Displays assertions grouped by priority tiers:
  Tier 1: Watched items (explicit human attention)
  Tier 2: Trusted source assertions (trust_level >= 3)
  Tier 3: Senior source assertions (seniority_tier >= 5)
  Tier 4: Everything else (by recency)

Within each tier, assertions are ordered by severity (critical > high > medium > low),
then by recency.

Examples:
  penf briefing "MTC 2026"
  penf briefing "MTC 2026" --tier 1
  penf briefing "MTC 2026" --limit 20
  penf briefing "MTC 2026" -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBriefing(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().Int32Var(&briefingTier, "tier", 0, "Filter to specific tier (1-4)")
	cmd.Flags().Int32Var(&briefingLimit, "limit", 50, "Maximum number of assertions")
	cmd.Flags().StringVarP(&briefingOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// NewEscalationsCommand creates the escalations command.
func NewEscalationsCommand(deps *BriefingCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultBriefingDeps()
	}

	cmd := &cobra.Command{
		Use:   "escalations",
		Short: "Show seniority escalations for a processing batch",
		Long: `Show seniority escalations for a processing batch.

An escalation occurs when a more senior person joins a discussion, raising
the maximum seniority tier. This signals that a topic may be getting
elevated attention and could require priority handling.

Seniority tiers: 1 (IC) → 4 (Manager) → 7 (VP/Executive)
When a topic jumps from tier 2 to tier 5, that's an escalation.

Examples:
  penf escalations --source-id 12345
  penf escalations --source-id 12345 -o json

Related Commands:
  penf briefing     Priority-ordered assertions (uses escalation data)
  penf seniority    Set seniority tiers for people
  penf trust        Set trust levels for people`,
		Aliases: []string{"escalation"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if escalationSource == 0 {
				return fmt.Errorf("--source-id is required")
			}
			return runEscalations(cmd.Context(), deps)
		},
	}

	cmd.Flags().Int64Var(&escalationSource, "source-id", 0, "Source ID for escalation detection (required)")
	cmd.MarkFlagRequired("source-id")
	cmd.Flags().StringVarP(&briefingOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// ==================== gRPC Connection ====================

// connectBriefingToGateway creates a gRPC connection to the gateway service.
func connectBriefingToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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

// getTenantIDForBriefing returns the tenant ID from env or config.
func getTenantIDForBriefing(deps *BriefingCommandDeps) string {
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant
	}
	if deps.Config != nil && deps.Config.TenantID != "" {
		return deps.Config.TenantID
	}
	// Default tenant.
	return "00000001-0000-0000-0000-000000000001"
}

// getUserIDForBriefing returns the user ID (default for now).
func getUserIDForBriefing() string {
	if envUser := os.Getenv("PENF_USER_ID"); envUser != "" {
		return envUser
	}
	return "default"
}

// ==================== Command Execution Functions ====================

// runBriefing executes the briefing command.
func runBriefing(ctx context.Context, deps *BriefingCommandDeps, projectName string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectBriefingToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Resolve project name to project ID.
	projectClient := projectv1.NewProjectServiceClient(conn)
	tenantID := getTenantIDForBriefing(deps)

	projectResp, err := projectClient.GetProject(ctx, &projectv1.GetProjectRequest{
		TenantId:   tenantID,
		Identifier: projectName,
	})
	if err != nil {
		return fmt.Errorf("project not found: %s", projectName)
	}

	projectID := projectResp.Project.Id

	// Get briefing assertions.
	watchlistClient := watchlistv1.NewWatchListServiceClient(conn)
	userID := getUserIDForBriefing()

	resp, err := watchlistClient.GetBriefingAssertions(ctx, &watchlistv1.GetBriefingAssertionsRequest{
		TenantId:  tenantID,
		UserId:    userID,
		ProjectId: projectID,
		Limit:     briefingLimit,
	})
	if err != nil {
		return fmt.Errorf("getting briefing assertions: %w", err)
	}

	// Filter by tier if specified.
	assertions := resp.Assertions
	if briefingTier > 0 {
		var filtered []*watchlistv1.BriefingAssertion
		for _, a := range assertions {
			if a.PriorityTier == briefingTier {
				filtered = append(filtered, a)
			}
		}
		assertions = filtered
	}

	return outputBriefing(cfg, projectResp.Project.Name, assertions)
}

// runEscalations executes the escalations command.
func runEscalations(ctx context.Context, deps *BriefingCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectBriefingToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	watchlistClient := watchlistv1.NewWatchListServiceClient(conn)
	tenantID := getTenantIDForBriefing(deps)

	resp, err := watchlistClient.GetSeniorityEscalations(ctx, &watchlistv1.GetSeniorityEscalationsRequest{
		TenantId: tenantID,
		SourceId: escalationSource,
	})
	if err != nil {
		return fmt.Errorf("getting seniority escalations: %w", err)
	}

	return outputEscalations(cfg, escalationSource, resp.Escalations)
}

// ==================== Output Functions ====================

// getBriefingOutputFormat returns the output format from flag or config.
func getBriefingOutputFormat(cfg *config.CLIConfig) config.OutputFormat {
	if briefingOutput != "" {
		return config.OutputFormat(briefingOutput)
	}
	if cfg != nil {
		return cfg.OutputFormat
	}
	return config.OutputFormatText
}

// outputBriefing outputs briefing assertions.
func outputBriefing(cfg *config.CLIConfig, projectName string, assertions []*watchlistv1.BriefingAssertion) error {
	format := getBriefingOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputBriefingJSON(assertions)
	case config.OutputFormatYAML:
		return outputBriefingYAML(assertions)
	default:
		return outputBriefingText(projectName, assertions)
	}
}

// outputBriefingText outputs briefing in human-readable format.
func outputBriefingText(projectName string, assertions []*watchlistv1.BriefingAssertion) error {
	if len(assertions) == 0 {
		fmt.Printf("Project: %s — Priority Briefing\n\n", projectName)
		fmt.Println("No assertions found.")
		return nil
	}

	fmt.Printf("Project: %s — Priority Briefing\n\n", projectName)

	// Group assertions by tier.
	tierGroups := make(map[int32][]*watchlistv1.BriefingAssertion)
	for _, a := range assertions {
		tierGroups[a.PriorityTier] = append(tierGroups[a.PriorityTier], a)
	}

	// Output each tier in order.
	for tier := int32(1); tier <= 4; tier++ {
		items := tierGroups[tier]
		if len(items) == 0 {
			continue
		}

		fmt.Printf("--- %s ---\n", formatTierName(tier))
		for _, a := range items {
			severityColor := getSeverityColor(a.Severity)
			fmt.Printf("  %s[%s]\033[0m %s: %s",
				severityColor,
				strings.ToUpper(a.Severity),
				a.Type,
				a.Description)

			// Add person info if available.
			if a.OwnerName != "" {
				parts := []string{a.OwnerName}
				if a.SeniorityTier > 0 {
					parts = append(parts, fmt.Sprintf("seniority: %d", a.SeniorityTier))
				}
				if a.TrustLevel > 0 {
					parts = append(parts, fmt.Sprintf("trust: %d", a.TrustLevel))
				}
				fmt.Printf(" (%s)", strings.Join(parts, ", "))
			}

			fmt.Println()
		}
		fmt.Println()
	}

	return nil
}

// outputBriefingJSON outputs briefing as JSON.
func outputBriefingJSON(assertions []*watchlistv1.BriefingAssertion) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(assertions)
}

// outputBriefingYAML outputs briefing as YAML.
func outputBriefingYAML(assertions []*watchlistv1.BriefingAssertion) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(assertions)
}

// outputEscalations outputs seniority escalations.
func outputEscalations(cfg *config.CLIConfig, sourceID int64, escalations []*watchlistv1.SeniorityEscalation) error {
	format := getBriefingOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputEscalationsJSON(escalations)
	case config.OutputFormatYAML:
		return outputEscalationsYAML(escalations)
	default:
		return outputEscalationsText(sourceID, escalations)
	}
}

// outputEscalationsText outputs escalations in human-readable format.
func outputEscalationsText(sourceID int64, escalations []*watchlistv1.SeniorityEscalation) error {
	if len(escalations) == 0 {
		fmt.Printf("Seniority Escalations (source: %d)\n\n", sourceID)
		fmt.Println("No escalations detected.")
		return nil
	}

	fmt.Printf("Seniority Escalations (source: %d)\n\n", sourceID)

	for _, e := range escalations {
		fmt.Printf("  Assertion #%d: \"%s\"\n", e.AssertionRootId, e.AssertionDescription)
		fmt.Printf("    Previous max seniority: %d\n", e.PreviousMaxSeniority)
		fmt.Printf("    Current max seniority:  %d\n", e.CurrentMaxSeniority)
		fmt.Println()
	}

	return nil
}

// outputEscalationsJSON outputs escalations as JSON.
func outputEscalationsJSON(escalations []*watchlistv1.SeniorityEscalation) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(escalations)
}

// outputEscalationsYAML outputs escalations as YAML.
func outputEscalationsYAML(escalations []*watchlistv1.SeniorityEscalation) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(escalations)
}

// ==================== Helper Functions ====================

// formatTierName returns the display name for a priority tier.
func formatTierName(tier int32) string {
	switch tier {
	case 1:
		return "Tier 1: Watched Items"
	case 2:
		return "Tier 2: Trusted Source"
	case 3:
		return "Tier 3: Senior Source"
	case 4:
		return "Tier 4: Other"
	default:
		return fmt.Sprintf("Tier %d", tier)
	}
}

// getSeverityColor returns the ANSI color code for a severity level.
func getSeverityColor(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "\033[31m" // Red
	case "high":
		return "\033[33m" // Yellow
	case "medium":
		return "\033[36m" // Cyan
	case "low":
		return "\033[32m" // Green
	default:
		return ""
	}
}
