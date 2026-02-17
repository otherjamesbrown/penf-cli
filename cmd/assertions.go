// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	assertionsv1 "github.com/otherjamesbrown/penf-cli/api/proto/assertions/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Assertion command flags.
var (
	assertionType      string
	assertionSince     string
	assertionUntil     string
	assertionAttrTo    string
	assertionProjectID int64
	assertionShowSource bool
	assertionLimit     int32
	assertionOffset    int32
	assertionOutput    string
	assertionGroupBy   string
)

// AssertionsCommandDeps holds the dependencies for assertion commands.
type AssertionsCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultAssertionsDeps returns the default dependencies for production use.
func DefaultAssertionsDeps() *AssertionsCommandDeps {
	return &AssertionsCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewAssertionsCommand creates the assertions command group.
func NewAssertionsCommand(deps *AssertionsCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultAssertionsDeps()
	}

	cmd := &cobra.Command{
		Use:   "assertions",
		Short: "Query assertions across all content",
		Long: `Query assertions (decisions, risks, commitments, action items, etc.) across all content.

Assertions are extracted claims from emails, Slack messages, meetings, and documents.
This command provides three main operations:

  list      List assertions with filters (type, date, person, project)
  search    Search assertions by keyword
  summary   Get aggregate statistics

Examples:
  penf assertions list --type action_item
  penf assertions list --since 7d --attributed-to "James Brown"
  penf assertions search "CLIC" --type decision
  penf assertions summary --since 30d --group-by type`,
	}

	cmd.AddCommand(newAssertionsListCommand(deps))
	cmd.AddCommand(newAssertionsSearchCommand(deps))
	cmd.AddCommand(newAssertionsSummaryCommand(deps))

	return cmd
}

// newAssertionsListCommand creates the 'assertions list' subcommand.
func newAssertionsListCommand(deps *AssertionsCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List assertions with optional filters",
		Long: `List assertions with optional filters for type, date range, attribution, and project.

Filters:
  --type              Filter by assertion type (decision, risk, commitment, action, etc.)
  --since             Filter by content date >= since (e.g., "7d", "2024-01-01")
  --until             Filter by content date <= until
  --attributed-to     Filter by person name (fuzzy match)
  --project-id        Filter by project ID
  --show-source       Include source context (content subject, date, from)
  --limit             Maximum results (default 50, max 500)
  --offset            Pagination offset

Examples:
  penf assertions list --type action_item
  penf assertions list --since 7d --limit 20
  penf assertions list --attributed-to "James Brown" --show-source
  penf assertions list --project-id 123 --type decision
  penf assertions list --since 2024-01-01 --until 2024-01-31 -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssertionsList(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&assertionType, "type", "", "Filter by assertion type")
	cmd.Flags().StringVar(&assertionSince, "since", "", "Filter by date >= since (e.g., 7d, 2024-01-01)")
	cmd.Flags().StringVar(&assertionUntil, "until", "", "Filter by date <= until")
	cmd.Flags().StringVar(&assertionAttrTo, "attributed-to", "", "Filter by person name (fuzzy match)")
	cmd.Flags().Int64Var(&assertionProjectID, "project-id", 0, "Filter by project ID")
	cmd.Flags().BoolVar(&assertionShowSource, "show-source", false, "Include source context")
	cmd.Flags().Int32Var(&assertionLimit, "limit", 50, "Maximum results (max 500)")
	cmd.Flags().Int32Var(&assertionOffset, "offset", 0, "Pagination offset")
	cmd.Flags().StringVarP(&assertionOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newAssertionsSearchCommand creates the 'assertions search' subcommand.
func newAssertionsSearchCommand(deps *AssertionsCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search assertions by keyword",
		Long: `Search assertions by keyword in description and source quote.

The search is case-insensitive and matches partial strings.

Examples:
  penf assertions search "CLIC"
  penf assertions search "budget" --type decision
  penf assertions search "migration" --since 30d
  penf assertions search "deadline" --show-source -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssertionsSearch(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVar(&assertionType, "type", "", "Filter by assertion type")
	cmd.Flags().StringVar(&assertionSince, "since", "", "Filter by date >= since")
	cmd.Flags().StringVar(&assertionUntil, "until", "", "Filter by date <= until")
	cmd.Flags().BoolVar(&assertionShowSource, "show-source", false, "Include source context")
	cmd.Flags().Int32Var(&assertionLimit, "limit", 50, "Maximum results")
	cmd.Flags().Int32Var(&assertionOffset, "offset", 0, "Pagination offset")
	cmd.Flags().StringVarP(&assertionOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newAssertionsSummaryCommand creates the 'assertions summary' subcommand.
func newAssertionsSummaryCommand(deps *AssertionsCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Get aggregate assertion statistics",
		Long: `Get aggregate statistics for assertions, grouped by type, project, or person.

Shows total counts and week-over-week trends (last 7 days vs. previous 7 days).

Group By Options:
  type      Group by assertion type (default)
  project   Group by project
  person    Group by attributed person

Examples:
  penf assertions summary --since 30d
  penf assertions summary --since 90d --group-by project
  penf assertions summary --group-by person -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssertionsSummary(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&assertionSince, "since", "", "Stats since date (e.g., 30d, 2024-01-01)")
	cmd.Flags().StringVar(&assertionUntil, "until", "", "Stats until date")
	cmd.Flags().StringVar(&assertionGroupBy, "group-by", "type", "Group by: type, project, person")
	cmd.Flags().StringVarP(&assertionOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// ==================== gRPC Connection ====================

// connectAssertionsToGateway creates a gRPC connection to the gateway service.
func connectAssertionsToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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

// getTenantIDForAssertions returns the tenant ID from env or config.
func getTenantIDForAssertions(deps *AssertionsCommandDeps) string {
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant
	}
	if deps.Config != nil && deps.Config.TenantID != "" {
		return deps.Config.TenantID
	}
	// Default tenant.
	return "00000001-0000-0000-0000-000000000001"
}

// parseDateOrDuration parses a duration string like "7d", "24h", "30d" into time.Time.
func parseDateOrDuration(durationStr string) (*time.Time, error) {
	if durationStr == "" {
		return nil, nil
	}

	// Try parsing as ISO date first
	if t, err := time.Parse("2006-01-02", durationStr); err == nil {
		return &t, nil
	}

	// Try parsing as duration (e.g., "7d", "24h")
	var duration time.Duration
	var err error

	if strings.HasSuffix(durationStr, "d") {
		days := strings.TrimSuffix(durationStr, "d")
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err != nil {
			return nil, fmt.Errorf("invalid duration format: %s", durationStr)
		}
		duration = time.Duration(d) * 24 * time.Hour
	} else {
		duration, err = time.ParseDuration(durationStr)
		if err != nil {
			return nil, fmt.Errorf("invalid duration format: %s", durationStr)
		}
	}

	t := time.Now().Add(-duration)
	return &t, nil
}

// ==================== Command Execution Functions ====================

// runAssertionsList executes the assertions list command.
func runAssertionsList(ctx context.Context, deps *AssertionsCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectAssertionsToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := assertionsv1.NewAssertionsServiceClient(conn)
	tenantID := getTenantIDForAssertions(deps)

	// Build request
	req := &assertionsv1.ListAssertionsRequest{
		TenantId:   tenantID,
		Limit:      assertionLimit,
		Offset:     assertionOffset,
		ShowSource: assertionShowSource,
	}

	if assertionType != "" {
		req.AssertionType = &assertionType
	}

	if assertionSince != "" {
		since, err := parseDateOrDuration(assertionSince)
		if err != nil {
			return fmt.Errorf("parsing --since: %w", err)
		}
		req.Since = timestamppb.New(*since)
	}

	if assertionUntil != "" {
		until, err := parseDateOrDuration(assertionUntil)
		if err != nil {
			return fmt.Errorf("parsing --until: %w", err)
		}
		req.Until = timestamppb.New(*until)
	}

	if assertionAttrTo != "" {
		req.AttributedTo = &assertionAttrTo
	}

	if assertionProjectID > 0 {
		req.ProjectId = &assertionProjectID
	}

	// Execute request
	resp, err := client.ListAssertions(ctx, req)
	if err != nil {
		return fmt.Errorf("listing assertions: %w", err)
	}

	// Output results
	return outputAssertionsList(cfg, resp)
}

// runAssertionsSearch executes the assertions search command.
func runAssertionsSearch(ctx context.Context, deps *AssertionsCommandDeps, query string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectAssertionsToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := assertionsv1.NewAssertionsServiceClient(conn)
	tenantID := getTenantIDForAssertions(deps)

	// Build request
	req := &assertionsv1.SearchAssertionsRequest{
		TenantId:   tenantID,
		Query:      query,
		Limit:      assertionLimit,
		Offset:     assertionOffset,
		ShowSource: assertionShowSource,
	}

	if assertionType != "" {
		req.AssertionType = &assertionType
	}

	if assertionSince != "" {
		since, err := parseDateOrDuration(assertionSince)
		if err != nil {
			return fmt.Errorf("parsing --since: %w", err)
		}
		req.Since = timestamppb.New(*since)
	}

	if assertionUntil != "" {
		until, err := parseDateOrDuration(assertionUntil)
		if err != nil {
			return fmt.Errorf("parsing --until: %w", err)
		}
		req.Until = timestamppb.New(*until)
	}

	// Execute request
	resp, err := client.SearchAssertions(ctx, req)
	if err != nil {
		return fmt.Errorf("searching assertions: %w", err)
	}

	// Output results
	return outputAssertionsSearch(cfg, query, resp)
}

// runAssertionsSummary executes the assertions summary command.
func runAssertionsSummary(ctx context.Context, deps *AssertionsCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectAssertionsToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := assertionsv1.NewAssertionsServiceClient(conn)
	tenantID := getTenantIDForAssertions(deps)

	// Build request
	req := &assertionsv1.GetAssertionSummaryRequest{
		TenantId: tenantID,
		GroupBy:  assertionGroupBy,
	}

	if assertionSince != "" {
		since, err := parseDateOrDuration(assertionSince)
		if err != nil {
			return fmt.Errorf("parsing --since: %w", err)
		}
		req.Since = timestamppb.New(*since)
	}

	if assertionUntil != "" {
		until, err := parseDateOrDuration(assertionUntil)
		if err != nil {
			return fmt.Errorf("parsing --until: %w", err)
		}
		req.Until = timestamppb.New(*until)
	}

	// Execute request
	resp, err := client.GetAssertionSummary(ctx, req)
	if err != nil {
		return fmt.Errorf("getting assertion summary: %w", err)
	}

	// Output results
	return outputAssertionsSummary(cfg, resp)
}

// ==================== Output Functions ====================

// getAssertionsOutputFormat returns the output format from flag or config.
func getAssertionsOutputFormat(cfg *config.CLIConfig) config.OutputFormat {
	if assertionOutput != "" {
		return config.OutputFormat(assertionOutput)
	}
	if cfg != nil {
		return cfg.OutputFormat
	}
	return config.OutputFormatText
}

// outputAssertionsList outputs list results.
func outputAssertionsList(cfg *config.CLIConfig, resp *assertionsv1.ListAssertionsResponse) error {
	format := getAssertionsOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputJSON(resp)
	case config.OutputFormatYAML:
		return outputYAML(resp)
	default:
		return outputAssertionsListText(resp)
	}
}

// outputAssertionsSearch outputs search results.
func outputAssertionsSearch(cfg *config.CLIConfig, query string, resp *assertionsv1.SearchAssertionsResponse) error {
	format := getAssertionsOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputJSON(resp)
	case config.OutputFormatYAML:
		return outputYAML(resp)
	default:
		return outputAssertionsSearchText(query, resp)
	}
}

// outputAssertionsSummary outputs summary results.
func outputAssertionsSummary(cfg *config.CLIConfig, resp *assertionsv1.AssertionSummaryResponse) error {
	format := getAssertionsOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputJSON(resp)
	case config.OutputFormatYAML:
		return outputYAML(resp)
	default:
		return outputAssertionsSummaryText(resp)
	}
}

// outputAssertionsListText outputs list in human-readable format.
func outputAssertionsListText(resp *assertionsv1.ListAssertionsResponse) error {
	if len(resp.Assertions) == 0 {
		fmt.Println("No assertions found.")
		return nil
	}

	fmt.Printf("Assertions (showing %d of %d total)\n\n", len(resp.Assertions), resp.TotalCount)

	for _, a := range resp.Assertions {
		fmt.Printf("ID %d: [%s] %s\n", a.Id, strings.ToUpper(a.AssertionType), a.Description)

		if a.SourceQuote != nil && *a.SourceQuote != "" {
			fmt.Printf("  Quote: %s\n", truncate(*a.SourceQuote, 80))
		}

		if len(a.AttributedTo) > 0 {
			var names []string
			for _, attr := range a.AttributedTo {
				names = append(names, fmt.Sprintf("%s (%s)", attr.Name, attr.Role))
			}
			fmt.Printf("  Attributed: %s\n", strings.Join(names, ", "))
		}

		if a.Source != nil {
			fmt.Printf("  Source: %s", a.Source.SourceType)
			if a.Source.Subject != nil {
				fmt.Printf(" - %s", truncate(*a.Source.Subject, 60))
			}
			if a.Source.From != nil {
				fmt.Printf(" (from: %s)", *a.Source.From)
			}
			if a.Source.Date != nil {
				fmt.Printf(" [%s]", a.Source.Date.AsTime().Format("2006-01-02"))
			}
			fmt.Println()
		}

		fmt.Println()
	}

	return nil
}

// outputAssertionsSearchText outputs search in human-readable format.
func outputAssertionsSearchText(query string, resp *assertionsv1.SearchAssertionsResponse) error {
	if len(resp.Assertions) == 0 {
		fmt.Printf("No assertions found matching \"%s\".\n", query)
		return nil
	}

	fmt.Printf("Search results for \"%s\" (showing %d of %d total)\n\n", query, len(resp.Assertions), resp.TotalCount)

	for _, a := range resp.Assertions {
		fmt.Printf("ID %d: [%s] %s\n", a.Id, strings.ToUpper(a.AssertionType), a.Description)

		if a.SourceQuote != nil && *a.SourceQuote != "" {
			fmt.Printf("  Quote: %s\n", truncate(*a.SourceQuote, 80))
		}

		if len(a.AttributedTo) > 0 {
			var names []string
			for _, attr := range a.AttributedTo {
				names = append(names, fmt.Sprintf("%s (%s)", attr.Name, attr.Role))
			}
			fmt.Printf("  Attributed: %s\n", strings.Join(names, ", "))
		}

		if a.Source != nil {
			fmt.Printf("  Source: %s", a.Source.SourceType)
			if a.Source.Subject != nil {
				fmt.Printf(" - %s", truncate(*a.Source.Subject, 60))
			}
			fmt.Println()
		}

		fmt.Println()
	}

	return nil
}

// outputAssertionsSummaryText outputs summary in human-readable format.
func outputAssertionsSummaryText(resp *assertionsv1.AssertionSummaryResponse) error {
	if len(resp.Entries) == 0 {
		fmt.Println("No assertion statistics available.")
		return nil
	}

	fmt.Printf("Assertion Summary (Total: %d)\n\n", resp.TotalAssertions)
	fmt.Printf("%-30s %8s %12s %12s %10s\n", "KEY", "TOTAL", "THIS WEEK", "LAST WEEK", "TREND")
	fmt.Printf("%-30s %8s %12s %12s %10s\n", "---", "-----", "---------", "---------", "-----")

	for _, entry := range resp.Entries {
		trend := ""
		if entry.LastWeek > 0 {
			change := ((float64(entry.ThisWeek) - float64(entry.LastWeek)) / float64(entry.LastWeek)) * 100
			if change > 0 {
				trend = fmt.Sprintf("+%.0f%%", change)
			} else if change < 0 {
				trend = fmt.Sprintf("%.0f%%", change)
			} else {
				trend = "="
			}
		} else if entry.ThisWeek > 0 {
			trend = "NEW"
		}

		fmt.Printf("%-30s %8d %12d %12d %10s\n",
			truncate(entry.Key, 30),
			entry.Count,
			entry.ThisWeek,
			entry.LastWeek,
			trend)
	}

	fmt.Println()
	return nil
}

// outputJSON outputs data as JSON.
func outputJSON(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// outputYAML outputs data as YAML.
func outputYAML(data any) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(data)
}

// truncate is defined in audit.go (same package)
