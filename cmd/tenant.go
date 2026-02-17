// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/metadata"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// getInsecureFlag retrieves the --insecure flag from the command's root.
// This is needed because tenant commands load their own config, which doesn't
// include flag overrides from PersistentPreRunE.
func getInsecureFlag(cmd *cobra.Command) bool {
	// Walk up to root command to find the persistent flag.
	root := cmd.Root()
	if root == nil {
		return false
	}
	insecure, _ := root.PersistentFlags().GetBool("insecure")
	return insecure
}

// TenantInfo represents detailed information about a tenant.
type TenantInfo struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	Status      string    `json:"status" yaml:"status"`
	Role        string    `json:"role,omitempty" yaml:"role,omitempty"`
	IsCurrent   bool      `json:"is_current" yaml:"is_current"`
}

// TenantListResponse represents the response from listing tenants.
type TenantListResponse struct {
	Tenants    []TenantInfo `json:"tenants" yaml:"tenants"`
	CurrentID  string       `json:"current_tenant_id,omitempty" yaml:"current_tenant_id,omitempty"`
	TotalCount int          `json:"total_count" yaml:"total_count"`
	FetchedAt  time.Time    `json:"fetched_at" yaml:"fetched_at"`
}

// TenantCommandDeps holds the dependencies for tenant commands.
// This allows for easier testing by injecting mock implementations.
type TenantCommandDeps struct {
	Config           *config.CLIConfig
	TenantClient     *client.TenantClient
	OutputFormat     config.OutputFormat
	LoadConfig       func() (*config.CLIConfig, error)
	SaveConfig       func(*config.CLIConfig) error
	InitTenantClient func(*config.CLIConfig) (*client.TenantClient, error)
	// Tenant client method overrides for testing
	ListTenants      func(ctx context.Context, c *client.TenantClient, req *client.ListTenantsRequest) ([]*client.Tenant, int64, error)
	GetTenant        func(ctx context.Context, c *client.TenantClient, id string, slug string) (*client.Tenant, error)
	SetCurrentTenant func(ctx context.Context, c *client.TenantClient, tenantRef string) (*client.Tenant, bool, string, error)
}

// DefaultDeps returns the default dependencies for production use.
func DefaultDeps() *TenantCommandDeps {
	return &TenantCommandDeps{
		LoadConfig: config.LoadConfig,
		SaveConfig: config.SaveConfig,
		InitTenantClient: func(cfg *config.CLIConfig) (*client.TenantClient, error) {
			opts := &client.ClientOptions{
				Insecure:       cfg.Insecure,
				Debug:          cfg.Debug,
				ConnectTimeout: cfg.Timeout,
			}

			// Load TLS config if not in insecure mode.
			if !cfg.Insecure {
				tlsConfig, err := client.LoadClientTLSConfig(&cfg.TLS)
				if err != nil {
					return nil, fmt.Errorf("failed to load TLS config: %w", err)
				}
				opts.TLSConfig = tlsConfig
			}

			tenantClient := client.NewTenantClient(cfg.ServerAddress, opts)
			ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
			defer cancel()

			if err := tenantClient.Connect(ctx); err != nil {
				return nil, fmt.Errorf("connecting to tenant service: %w", err)
			}
			return tenantClient, nil
		},
		// Default implementations call the actual client methods
		ListTenants: func(ctx context.Context, c *client.TenantClient, req *client.ListTenantsRequest) ([]*client.Tenant, int64, error) {
			return c.ListTenants(ctx, req)
		},
		GetTenant: func(ctx context.Context, c *client.TenantClient, id string, slug string) (*client.Tenant, error) {
			return c.GetTenant(ctx, id, slug)
		},
		SetCurrentTenant: func(ctx context.Context, c *client.TenantClient, tenantRef string) (*client.Tenant, bool, string, error) {
			return c.SetCurrentTenant(ctx, tenantRef)
		},
	}
}

// NewTenantCommand creates the root tenant command with all subcommands.
func NewTenantCommand(deps *TenantCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultDeps()
	}

	cmd := &cobra.Command{
		Use:   "tenant",
		Short: "Manage tenant context for multi-tenant operations",
		Long: `Manage tenant context for multi-tenant operations in Penfold.

Penfold supports multi-tenancy — each tenant has isolated content, entities,
and configuration. The current tenant is stored in ~/.penf/config.yaml and
can be overridden with the PENF_TENANT_ID environment variable or --tenant flag.

Commands:
  list     List all accessible tenants
  current  Show the active tenant
  switch   Change the active tenant
  show     Display tenant details

Examples:
  penf tenant current              Show active tenant
  penf tenant list                 List all tenants
  penf tenant switch my-tenant     Switch to a different tenant
  penf tenant show my-tenant       View tenant details

Most commands accept --tenant to override the active tenant for a single operation.`,
	}

	// Add subcommands
	cmd.AddCommand(newTenantListCommand(deps))
	cmd.AddCommand(newTenantSwitchCommand(deps))
	cmd.AddCommand(newTenantCurrentCommand(deps))
	cmd.AddCommand(newTenantShowCommand(deps))

	return cmd
}

// getOutputFormat reads the --output flag value from the root command's persistent flags.
func getOutputFormat(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	if f := cmd.Root().PersistentFlags().Lookup("output"); f != nil {
		return f.Value.String()
	}
	return ""
}

// applyOutputFormat applies the root --output flag to a config loaded by tenant commands.
func applyOutputFormat(cfg *config.CLIConfig, cmd *cobra.Command) {
	if v := getOutputFormat(cmd); v != "" {
		cfg.OutputFormat = config.OutputFormat(v)
	}
}

// newTenantListCommand creates the 'tenant list' subcommand.
func newTenantListCommand(deps *TenantCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List accessible tenants",
		Long: `List all tenants that the current user has access to.

Displays tenant ID, name, status, and indicates the currently active tenant.
Use --output to change the output format (text, json, yaml).`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTenantList(cmd.Context(), deps, getInsecureFlag(cmd), cmd)
		},
	}
}

// newTenantSwitchCommand creates the 'tenant switch' subcommand.
func newTenantSwitchCommand(deps *TenantCommandDeps) *cobra.Command {
	var noValidate bool

	cmd := &cobra.Command{
		Use:   "switch <tenant-id-or-alias>",
		Short: "Switch to a different tenant",
		Long: `Switch the active tenant context to a different tenant.

You can specify the tenant by its ID or alias (if configured).
The switch is validated against accessible tenants unless --no-validate is used.

Example:
  penf tenant switch acme-corp
  penf tenant switch tenant-123-456
  penf tenant switch work  # using alias`,
		Aliases: []string{"use", "sw"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTenantSwitch(cmd.Context(), deps, args[0], !noValidate, getInsecureFlag(cmd))
		},
	}

	cmd.Flags().BoolVar(&noValidate, "no-validate", false, "Skip tenant access validation")

	return cmd
}

// newTenantCurrentCommand creates the 'tenant current' subcommand.
func newTenantCurrentCommand(deps *TenantCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show current tenant",
		Long: `Show the currently active tenant context.

Displays the tenant ID from the configuration file or environment variable.
The environment variable PENF_TENANT_ID takes precedence over the config file.`,
		Aliases: []string{"whoami"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTenantCurrent(deps, cmd)
		},
	}
}

// newTenantShowCommand creates the 'tenant show' subcommand.
func newTenantShowCommand(deps *TenantCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show [tenant-id-or-alias]",
		Short: "Show tenant details",
		Long: `Show detailed information about a tenant.

If no tenant is specified, shows information about the current tenant.
You can specify the tenant by its ID or alias (if configured).

Example:
  penf tenant show
  penf tenant show acme-corp
  penf tenant show tenant-123-456`,
		Aliases: []string{"info"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tenantID := ""
			if len(args) > 0 {
				tenantID = args[0]
			}
			return runTenantShow(cmd.Context(), deps, tenantID, getInsecureFlag(cmd), cmd)
		},
	}
}

// runTenantList executes the tenant list command.
func runTenantList(ctx context.Context, deps *TenantCommandDeps, insecureFlag bool, cmd *cobra.Command) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	if insecureFlag {
		cfg.Insecure = true
	}
	applyOutputFormat(cfg, cmd)
	deps.Config = cfg

	// Get current tenant ID (from env or config).
	currentTenantID := getCurrentTenantID(cfg)

	// Create tenant client using injected dependency.
	tenantClient, err := deps.InitTenantClient(cfg)
	if err != nil {
		return err
	}
	defer tenantClient.Close()

	var tenantList []*client.Tenant
	var totalCount int64
	if deps.ListTenants != nil {
		tenantList, totalCount, err = deps.ListTenants(ctx, tenantClient, &client.ListTenantsRequest{
			Limit: 100,
		})
	} else {
		tenantList, totalCount, err = tenantClient.ListTenants(ctx, &client.ListTenantsRequest{
			Limit: 100,
		})
	}
	if err != nil {
		return fmt.Errorf("listing tenants: %w", err)
	}

	// Convert to TenantInfo slice.
	tenants := make([]TenantInfo, len(tenantList))
	for i, t := range tenantList {
		status := "inactive"
		if t.IsActive {
			status = "active"
		}
		tenants[i] = TenantInfo{
			ID:          t.Slug,
			Name:        t.Name,
			Description: t.Description,
			CreatedAt:   t.CreatedAt,
			Status:      status,
			IsCurrent:   t.Slug == currentTenantID,
		}
	}

	response := TenantListResponse{
		Tenants:    tenants,
		CurrentID:  currentTenantID,
		TotalCount: int(totalCount),
		FetchedAt:  time.Now(),
	}

	return outputTenantList(cfg.OutputFormat, response)
}

// runTenantSwitch executes the tenant switch command.
func runTenantSwitch(ctx context.Context, deps *TenantCommandDeps, tenantRef string, validate bool, insecureFlag bool) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	// Override with command-line flag if set.
	if insecureFlag {
		cfg.Insecure = true
	}
	deps.Config = cfg

	// Resolve alias to tenant ID if applicable.
	tenantID := resolveTenantAlias(cfg, tenantRef)

	// Validate tenant access if requested.
	if validate {
		if err := validateTenantAccess(ctx, deps, tenantID); err != nil {
			return fmt.Errorf("tenant validation failed: %w", err)
		}
	}

	// Check if env var is set (warn user).
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		fmt.Fprintf(os.Stderr, "Warning: PENF_TENANT_ID environment variable is set to %q.\n", envTenant)
		fmt.Fprintf(os.Stderr, "The environment variable takes precedence over the config file.\n")
		fmt.Fprintf(os.Stderr, "Unset it to use the config file setting.\n\n")
	}

	// Update configuration.
	cfg.TenantID = tenantID
	if err := deps.SaveConfig(cfg); err != nil {
		return fmt.Errorf("saving configuration: %w", err)
	}

	fmt.Printf("Switched to tenant: %s\n", tenantID)

	// Show alias if it was used.
	if tenantRef != tenantID {
		fmt.Printf("  (alias: %s)\n", tenantRef)
	}

	return nil
}

// runTenantCurrent executes the tenant current command.
func runTenantCurrent(deps *TenantCommandDeps, cmd *cobra.Command) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	applyOutputFormat(cfg, cmd)
	deps.Config = cfg

	currentTenantID := getCurrentTenantID(cfg)

	if currentTenantID == "" {
		fmt.Println("No tenant configured.")
		fmt.Println("\nUse 'penf tenant switch <tenant-id>' to set a tenant.")
		fmt.Println("Or set the PENF_TENANT_ID environment variable.")
		return nil
	}

	// Determine the source.
	source := "config file"
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		source = "environment variable (PENF_TENANT_ID)"
	}

	switch cfg.OutputFormat {
	case config.OutputFormatJSON:
		output := map[string]string{
			"tenant_id": currentTenantID,
			"source":    source,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	case config.OutputFormatYAML:
		output := map[string]string{
			"tenant_id": currentTenantID,
			"source":    source,
		}
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(output)
	default:
		fmt.Printf("Current tenant: %s\n", currentTenantID)
		fmt.Printf("  Source: %s\n", source)

		// Show alias if there is one.
		if alias := findTenantAlias(cfg, currentTenantID); alias != "" {
			fmt.Printf("  Alias: %s\n", alias)
		}
	}

	return nil
}

// runTenantShow executes the tenant info command.
func runTenantShow(ctx context.Context, deps *TenantCommandDeps, tenantRef string, insecureFlag bool, cmd *cobra.Command) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	if insecureFlag {
		cfg.Insecure = true
	}
	applyOutputFormat(cfg, cmd)
	deps.Config = cfg

	// Use current tenant if none specified.
	if tenantRef == "" {
		tenantRef = getCurrentTenantID(cfg)
		if tenantRef == "" {
			return fmt.Errorf("no tenant specified and no current tenant configured")
		}
	}

	// Resolve alias to tenant ID if applicable.
	tenantID := resolveTenantAlias(cfg, tenantRef)

	// Create tenant client using injected dependency.
	tenantClient, err := deps.InitTenantClient(cfg)
	if err != nil {
		return err
	}
	defer tenantClient.Close()

	var tenant *client.Tenant
	if deps.GetTenant != nil {
		tenant, err = deps.GetTenant(ctx, tenantClient, "", tenantID)
	} else {
		tenant, err = tenantClient.GetTenant(ctx, "", tenantID)
	}
	if err != nil {
		return fmt.Errorf("getting tenant info: %w", err)
	}

	status := "inactive"
	if tenant != nil && tenant.IsActive {
		status = "active"
	}

	info := TenantInfo{
		ID:          tenant.Slug,
		Name:        tenant.Name,
		Description: tenant.Description,
		CreatedAt:   tenant.CreatedAt,
		Status:      status,
		IsCurrent:   tenant.Slug == getCurrentTenantID(cfg),
	}

	return outputTenantDetail(cfg.OutputFormat, info)
}

// getCurrentTenantID returns the current tenant ID from env or config.
// Environment variable takes precedence.
func getCurrentTenantID(cfg *config.CLIConfig) string {
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant
	}
	return cfg.TenantID
}

// resolveTenantAlias resolves a tenant reference to its actual ID.
// If the reference is an alias in the config, returns the mapped ID.
// Otherwise returns the reference as-is.
func resolveTenantAlias(cfg *config.CLIConfig, ref string) string {
	if cfg.TenantAliases != nil {
		if id, ok := cfg.TenantAliases[ref]; ok {
			return id
		}
	}
	return ref
}

// findTenantAlias finds an alias for a given tenant ID.
// Returns empty string if no alias exists.
func findTenantAlias(cfg *config.CLIConfig, tenantID string) string {
	if cfg.TenantAliases == nil {
		return ""
	}
	for alias, id := range cfg.TenantAliases {
		if id == tenantID {
			return alias
		}
	}
	return ""
}

// validateTenantAccess validates that the user has access to the tenant.
func validateTenantAccess(ctx context.Context, deps *TenantCommandDeps, tenantID string) error {
	if tenantID == "" {
		return fmt.Errorf("tenant ID cannot be empty")
	}

	if strings.ContainsAny(tenantID, " \t\n") {
		return fmt.Errorf("tenant ID contains invalid characters")
	}

	cfg := deps.Config
	if cfg == nil {
		var err error
		cfg, err = deps.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading configuration: %w", err)
		}
	}

	// Create tenant client using injected dependency.
	tenantClient, err := deps.InitTenantClient(cfg)
	if err != nil {
		return err
	}
	defer tenantClient.Close()

	var valid bool
	var errMsg string
	if deps.SetCurrentTenant != nil {
		_, valid, errMsg, err = deps.SetCurrentTenant(ctx, tenantClient, tenantID)
	} else {
		_, valid, errMsg, err = tenantClient.SetCurrentTenant(ctx, tenantID)
	}
	if err != nil {
		return fmt.Errorf("validating tenant: %w", err)
	}

	if !valid {
		if errMsg != "" {
			return fmt.Errorf("tenant validation failed: %s", errMsg)
		}
		return fmt.Errorf("tenant %q is not accessible", tenantID)
	}

	return nil
}

// outputTenantList outputs the tenant list in the configured format.
func outputTenantList(format config.OutputFormat, response TenantListResponse) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(response)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(response)
	default:
		return outputTenantListText(response)
	}
}

// outputTenantListText outputs the tenant list in human-readable format.
func outputTenantListText(response TenantListResponse) error {
	if len(response.Tenants) == 0 {
		fmt.Println("No tenants found.")
		return nil
	}

	fmt.Printf("Available tenants (%d):\n\n", response.TotalCount)
	fmt.Println("  CURRENT  ID                      NAME                    STATUS    ROLE")
	fmt.Println("  -------  --                      ----                    ------    ----")

	// Sort by name for consistent output.
	tenants := make([]TenantInfo, len(response.Tenants))
	copy(tenants, response.Tenants)
	sort.Slice(tenants, func(i, j int) bool {
		return tenants[i].Name < tenants[j].Name
	})

	for _, t := range tenants {
		currentMarker := " "
		if t.IsCurrent {
			currentMarker = "*"
		}

		// Truncate long values for table display.
		id := truncateString(t.ID, 22)
		name := truncateString(t.Name, 22)

		statusColor := "\033[32m" // Green for active
		if t.Status != "active" {
			statusColor = "\033[33m" // Yellow for other
		}

		fmt.Printf("  %s        %-22s  %-22s  %s%-8s\033[0m  %s\n",
			currentMarker, id, name, statusColor, t.Status, t.Role)
	}

	fmt.Println()
	return nil
}

// outputTenantDetail outputs tenant info in the configured format.
func outputTenantDetail(format config.OutputFormat, info TenantInfo) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(info)
	default:
		return outputTenantDetailText(info)
	}
}

// outputTenantDetailText outputs tenant info in human-readable format.
func outputTenantDetailText(info TenantInfo) error {
	statusColor := "\033[32m"
	if info.Status != "active" {
		statusColor = "\033[33m"
	}

	fmt.Printf("Tenant Information:\n\n")
	fmt.Printf("  ID:          %s\n", info.ID)
	fmt.Printf("  Name:        %s\n", info.Name)
	if info.Description != "" {
		fmt.Printf("  Description: %s\n", info.Description)
	}
	fmt.Printf("  Status:      %s%s\033[0m\n", statusColor, info.Status)
	if info.Role != "" && info.Role != "unknown" {
		fmt.Printf("  Your Role:   %s\n", info.Role)
	}
	if !info.CreatedAt.IsZero() {
		fmt.Printf("  Created:     %s\n", info.CreatedAt.Format(time.RFC3339))
	}
	if info.IsCurrent {
		fmt.Printf("  Current:     yes (active context)\n")
	}

	return nil
}

// truncateString truncates a string to the given length with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// AddTenantMetadata adds tenant ID to gRPC metadata context.
// This should be called before making gRPC requests.
func AddTenantMetadata(ctx context.Context, tenantID string) context.Context {
	if tenantID == "" {
		return ctx
	}
	md := metadata.Pairs("x-tenant-id", tenantID)
	return metadata.NewOutgoingContext(ctx, md)
}

// GetTenantFromContext extracts tenant ID from incoming gRPC context.
func GetTenantFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get("x-tenant-id")
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
