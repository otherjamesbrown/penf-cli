// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	productv1 "github.com/otherjamesbrown/penf-cli/api/proto/product/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Team command flags.
var (
	teamContext string
	teamScope   string
	teamRole    string
	showAll     bool // include inactive
)

// addProductTeamCommands adds the team subcommand group to the product command.
func addProductTeamCommands(productCmd *cobra.Command, deps *ProductCommandDeps) {
	teamCmd := &cobra.Command{
		Use:   "team <product>",
		Short: "List teams associated with a product",
		Long: `List all teams associated with a product.

Shows team associations including context (e.g., "EMEA", "core").

Examples:
  # List teams for a product
  penf product team "My Product"

  # List teams with a specific context
  penf product team "My Product" --context core`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductTeamList(cmd.Context(), deps, args[0])
		},
	}

	teamCmd.Flags().StringVar(&teamContext, "context", "", "Filter by context")

	// Subcommands for team management.
	teamCmd.AddCommand(newProductTeamAddCommand(deps))
	teamCmd.AddCommand(newProductTeamRemoveCommand(deps))
	teamCmd.AddCommand(newProductTeamRoleCommand(deps))
	teamCmd.AddCommand(newProductTeamPeopleCommand(deps))

	productCmd.AddCommand(teamCmd)
}

// newProductTeamAddCommand creates the 'product team add' subcommand.
func newProductTeamAddCommand(deps *ProductCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <product> <team>",
		Short: "Associate a team with a product",
		Long: `Associate a team with a product.

The team must already exist in the system. Use --context to specify
the team's relationship to the product (e.g., "EMEA", "core", "API").

Examples:
  # Associate a team with a product
  penf product team add "My Product" "Engineering Team"

  # Associate with a context
  penf product team add "My Product" "Engineering Team" --context "EMEA"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductTeamAdd(cmd.Context(), deps, args[0], args[1])
		},
	}

	cmd.Flags().StringVar(&teamContext, "context", "", "Context for the team association (e.g., EMEA, core)")

	return cmd
}

// newProductTeamRemoveCommand creates the 'product team remove' subcommand.
func newProductTeamRemoveCommand(deps *ProductCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <product> <team>",
		Short: "Remove a team from a product",
		Long: `Remove a team association from a product.

If the team has multiple contexts, specify --context to remove a specific one.

Examples:
  penf product team remove "My Product" "Engineering Team"
  penf product team remove "My Product" "Engineering Team" --context "EMEA"`,
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductTeamRemove(cmd.Context(), deps, args[0], args[1])
		},
	}

	cmd.Flags().StringVar(&teamContext, "context", "", "Context to remove (if team has multiple)")

	return cmd
}

// newProductTeamRoleCommand creates the 'product team role' subcommand group.
func newProductTeamRoleCommand(deps *ProductCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "role",
		Short: "Manage roles for product-team associations",
		Long: `Manage scoped roles for product-team associations.

Roles define who does what for a product within a team context.
Common roles: DRI, TL, PM, Engineer, Reviewer.
Scopes can further narrow the role (e.g., "networking", "security").`,
	}

	cmd.AddCommand(newProductTeamRoleListCommand(deps))
	cmd.AddCommand(newProductTeamRoleAddCommand(deps))
	cmd.AddCommand(newProductTeamRoleEndCommand(deps))
	cmd.AddCommand(newProductTeamRoleFindCommand(deps))

	return cmd
}

// newProductTeamRoleListCommand creates the 'product team role list' subcommand.
func newProductTeamRoleListCommand(deps *ProductCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <product> <team>",
		Short: "List roles for a product-team",
		Long: `List all roles assigned to people for a product-team association.

Examples:
  penf product team role list "My Product" "Engineering Team"
  penf product team role list "My Product" "Engineering Team" --all  # include inactive`,
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductTeamRoleList(cmd.Context(), deps, args[0], args[1])
		},
	}

	cmd.Flags().BoolVar(&showAll, "all", false, "Include inactive roles")
	cmd.Flags().StringVar(&teamContext, "context", "", "Filter by team context")

	return cmd
}

// newProductTeamRoleAddCommand creates the 'product team role add' subcommand.
func newProductTeamRoleAddCommand(deps *ProductCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <product> <team> <person-email> <role>",
		Short: "Add a role assignment",
		Long: `Add a role for a person on a product-team association.

Common roles: DRI, TL, PM, Engineer, Reviewer, Approver.
Use --scope to narrow the role (e.g., "networking", "security", "API").

Examples:
  # Make someone the DRI for a product
  penf product team role add "My Product" "Engineering" "john@example.com" "DRI"

  # Add a scoped role
  penf product team role add "My Product" "Engineering" "jane@example.com" "DRI" --scope "networking"`,
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductTeamRoleAdd(cmd.Context(), deps, args[0], args[1], args[2], args[3])
		},
	}

	cmd.Flags().StringVar(&teamContext, "context", "", "Team context (if team has multiple)")
	cmd.Flags().StringVar(&teamScope, "scope", "", "Role scope (e.g., networking, security)")

	return cmd
}

// newProductTeamRoleEndCommand creates the 'product team role end' subcommand.
func newProductTeamRoleEndCommand(deps *ProductCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "end <role-id>",
		Short: "End a role assignment",
		Long: `End a role assignment (mark as inactive).

The role history is preserved for audit purposes.

Example:
  penf product team role end 123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductTeamRoleEnd(cmd.Context(), deps, args[0])
		},
	}
}

// newProductTeamRoleFindCommand creates the 'product team role find' subcommand.
func newProductTeamRoleFindCommand(deps *ProductCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find",
		Short: "Find people by role",
		Long: `Find people by role across products and teams.

This answers questions like "who is the DRI for networking on Product X?"

Examples:
  # Find all DRIs
  penf product team role find --role DRI

  # Find DRI for a specific product
  penf product team role find --role DRI --product "My Product"

  # Find networking DRI on a product
  penf product team role find --role DRI --product "My Product" --scope networking`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductTeamRoleFind(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&teamRole, "role", "", "Role to search for (e.g., DRI, TL)")
	cmd.Flags().StringVar(&productParent, "product", "", "Filter by product name")
	cmd.Flags().StringVar(&teamScope, "scope", "", "Filter by scope")
	cmd.Flags().BoolVar(&showAll, "all", false, "Include inactive roles")

	return cmd
}

// newProductTeamPeopleCommand creates the 'product team people' subcommand.
func newProductTeamPeopleCommand(deps *ProductCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "people <product>",
		Short: "List all people working on a product",
		Long: `List all people working on a product across all teams.

Shows everyone with a role on the product, organized by team.

Examples:
  penf product team people "My Product"
  penf product team people "My Product" --all  # include inactive`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductTeamPeople(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().BoolVar(&showAll, "all", false, "Include inactive roles")

	return cmd
}

// ==================== Command Execution Functions ====================

// runProductTeamList lists teams for a product.
func runProductTeamList(ctx context.Context, deps *ProductCommandDeps, productName string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	resp, err := client.ListProductTeams(ctx, &productv1.ListProductTeamsRequest{
		TenantId:          tenantID,
		ProductIdentifier: productName,
	})
	if err != nil {
		return fmt.Errorf("listing teams: %w", err)
	}

	teams := resp.Teams

	// Filter by context if specified (client-side filtering).
	if teamContext != "" {
		var filtered []*productv1.ProductTeam
		for _, t := range teams {
			if strings.EqualFold(t.Context, teamContext) {
				filtered = append(filtered, t)
			}
		}
		teams = filtered
	}

	return outputProductTeamsProto(deps.Config, productName, teams)
}

// runProductTeamAdd associates a team with a product.
func runProductTeamAdd(ctx context.Context, deps *ProductCommandDeps, productName, teamName string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	resp, err := client.AddProductTeam(ctx, &productv1.AddProductTeamRequest{
		TenantId:          tenantID,
		ProductIdentifier: productName,
		TeamIdentifier:    teamName,
		Context:           teamContext,
	})
	if err != nil {
		return fmt.Errorf("associating team: %w", err)
	}

	contextStr := ""
	if resp.ProductTeam.Context != "" {
		contextStr = fmt.Sprintf(" (context: %s)", resp.ProductTeam.Context)
	}
	fmt.Printf("\033[32mAssociated team:\033[0m %s -> %s%s\n", resp.ProductTeam.TeamName, resp.ProductTeam.ProductName, contextStr)
	return nil
}

// runProductTeamRemove removes a team from a product.
func runProductTeamRemove(ctx context.Context, deps *ProductCommandDeps, productName, teamName string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	// First, list teams to find the product-team ID.
	listResp, err := client.ListProductTeams(ctx, &productv1.ListProductTeamsRequest{
		TenantId:          tenantID,
		ProductIdentifier: productName,
	})
	if err != nil {
		return fmt.Errorf("listing teams: %w", err)
	}

	// Find the matching team.
	var productTeamID int64
	var foundTeamName string
	for _, pt := range listResp.Teams {
		if strings.EqualFold(pt.TeamName, teamName) {
			// If context specified, must match.
			if teamContext != "" && !strings.EqualFold(pt.Context, teamContext) {
				continue
			}
			productTeamID = pt.Id
			foundTeamName = pt.TeamName
			break
		}
	}

	if productTeamID == 0 {
		return fmt.Errorf("team '%s' is not associated with '%s'", teamName, productName)
	}

	// Remove the association.
	_, err = client.RemoveProductTeam(ctx, &productv1.RemoveProductTeamRequest{
		TenantId:      tenantID,
		ProductTeamId: productTeamID,
	})
	if err != nil {
		return fmt.Errorf("removing team: %w", err)
	}

	fmt.Printf("\033[32mRemoved team:\033[0m %s from %s\n", foundTeamName, productName)
	return nil
}

// runProductTeamRoleList lists roles for a product-team.
func runProductTeamRoleList(ctx context.Context, deps *ProductCommandDeps, productName, teamName string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	// First, list teams to find the product-team ID.
	listResp, err := client.ListProductTeams(ctx, &productv1.ListProductTeamsRequest{
		TenantId:          tenantID,
		ProductIdentifier: productName,
	})
	if err != nil {
		return fmt.Errorf("listing teams: %w", err)
	}

	// Find the matching team.
	var productTeamID int64
	for _, pt := range listResp.Teams {
		if strings.EqualFold(pt.TeamName, teamName) {
			// If context specified, must match.
			if teamContext != "" && !strings.EqualFold(pt.Context, teamContext) {
				continue
			}
			productTeamID = pt.Id
			break
		}
	}

	if productTeamID == 0 {
		return fmt.Errorf("team '%s' is not associated with '%s'", teamName, productName)
	}

	// Get roles.
	activeOnly := !showAll
	rolesResp, err := client.ListProductTeamRoles(ctx, &productv1.ListProductTeamRolesRequest{
		TenantId:      tenantID,
		ProductTeamId: productTeamID,
		ActiveOnly:    activeOnly,
	})
	if err != nil {
		return fmt.Errorf("getting roles: %w", err)
	}

	return outputProductTeamRolesProto(deps.Config, productName, teamName, rolesResp.Roles)
}

// runProductTeamRoleAdd adds a role assignment.
func runProductTeamRoleAdd(ctx context.Context, deps *ProductCommandDeps, productName, teamName, personEmail, roleName string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	// First, list teams to find the product-team ID.
	listResp, err := client.ListProductTeams(ctx, &productv1.ListProductTeamsRequest{
		TenantId:          tenantID,
		ProductIdentifier: productName,
	})
	if err != nil {
		return fmt.Errorf("listing teams: %w", err)
	}

	// Find the matching team.
	var productTeamID int64
	for _, pt := range listResp.Teams {
		if strings.EqualFold(pt.TeamName, teamName) {
			// If context specified, must match.
			if teamContext != "" && !strings.EqualFold(pt.Context, teamContext) {
				continue
			}
			productTeamID = pt.Id
			break
		}
	}

	if productTeamID == 0 {
		return fmt.Errorf("team '%s' is not associated with '%s' - add team first", teamName, productName)
	}

	// Add role via gRPC - gateway resolves person by email.
	resp, err := client.AddProductTeamRole(ctx, &productv1.AddProductTeamRoleRequest{
		TenantId:         tenantID,
		ProductTeamId:    productTeamID,
		PersonIdentifier: personEmail,
		Role:             roleName,
		Scope:            teamScope,
	})
	if err != nil {
		return fmt.Errorf("adding role: %w", err)
	}

	scopeStr := ""
	if resp.Role.Scope != "" {
		scopeStr = fmt.Sprintf(" [%s]", resp.Role.Scope)
	}
	fmt.Printf("\033[32mAdded role:\033[0m %s is now %s%s on %s/%s (ID: %d)\n",
		resp.Role.PersonName, resp.Role.Role, scopeStr, resp.Role.ProductName, resp.Role.TeamName, resp.Role.Id)
	return nil
}

// runProductTeamRoleEnd ends a role assignment.
func runProductTeamRoleEnd(ctx context.Context, deps *ProductCommandDeps, roleIDStr string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	var roleID int64
	if _, err := fmt.Sscanf(roleIDStr, "%d", &roleID); err != nil {
		return fmt.Errorf("invalid role ID: %s", roleIDStr)
	}

	// Get role info for display before ending.
	getResp, err := client.GetProductTeamRole(ctx, &productv1.GetProductTeamRoleRequest{
		TenantId: tenantID,
		RoleId:   roleID,
	})
	if err != nil {
		return fmt.Errorf("role not found: %d", roleID)
	}

	// End role.
	_, err = client.EndProductTeamRole(ctx, &productv1.EndProductTeamRoleRequest{
		TenantId: tenantID,
		RoleId:   roleID,
	})
	if err != nil {
		return fmt.Errorf("ending role: %w", err)
	}

	fmt.Printf("\033[32mEnded role:\033[0m %s is no longer %s on %s/%s\n",
		getResp.Role.PersonName, getResp.Role.Role, getResp.Role.ProductName, getResp.Role.TeamName)
	return nil
}

// runProductTeamRoleFind finds people by role.
func runProductTeamRoleFind(ctx context.Context, deps *ProductCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	resp, err := client.FindByRole(ctx, &productv1.FindByRoleRequest{
		TenantId:          tenantID,
		Role:              teamRole,
		ProductIdentifier: productParent, // reusing the --product flag
		Scope:             teamScope,
		ActiveOnly:        !showAll,
	})
	if err != nil {
		return fmt.Errorf("finding roles: %w", err)
	}

	return outputRoleSearchResultsProto(deps.Config, resp.Roles)
}

// runProductTeamPeople lists all people on a product.
func runProductTeamPeople(ctx context.Context, deps *ProductCommandDeps, productName string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	activeOnly := !showAll
	resp, err := client.ListProductPeople(ctx, &productv1.ListProductPeopleRequest{
		TenantId:          tenantID,
		ProductIdentifier: productName,
		ActiveOnly:        activeOnly,
	})
	if err != nil {
		return fmt.Errorf("getting people: %w", err)
	}

	return outputProductPeopleProto(deps.Config, productName, resp.People)
}

// ==================== Output Functions ====================

func outputProductTeamsProto(cfg *config.CLIConfig, productName string, teams []*productv1.ProductTeam) error {
	format := getProductOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputProductJSON(teams)
	case config.OutputFormatYAML:
		return outputProductYAML(teams)
	default:
		return outputProductTeamsTableProto(productName, teams)
	}
}

func outputProductTeamsTableProto(productName string, teams []*productv1.ProductTeam) error {
	if len(teams) == 0 {
		fmt.Printf("No teams associated with '%s'\n", productName)
		return nil
	}

	fmt.Printf("Teams for '%s' (%d):\n\n", productName, len(teams))
	fmt.Println("  ID      TEAM                           CONTEXT")
	fmt.Println("  --      ----                           -------")

	for _, t := range teams {
		context := "-"
		if t.Context != "" {
			context = t.Context
		}
		fmt.Printf("  %-6d  %-30s %s\n", t.Id, truncateString(t.TeamName, 30), context)
	}

	fmt.Println()
	return nil
}

func outputProductTeamRolesProto(cfg *config.CLIConfig, productName, teamName string, roles []*productv1.ProductTeamRole) error {
	format := getProductOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputProductJSON(roles)
	case config.OutputFormatYAML:
		return outputProductYAML(roles)
	default:
		return outputProductTeamRolesTableProto(productName, teamName, roles)
	}
}

func outputProductTeamRolesTableProto(productName, teamName string, roles []*productv1.ProductTeamRole) error {
	if len(roles) == 0 {
		fmt.Printf("No roles assigned for '%s' / '%s'\n", productName, teamName)
		return nil
	}

	fmt.Printf("Roles for '%s' / '%s' (%d):\n\n", productName, teamName, len(roles))
	fmt.Println("  ID      PERSON                         ROLE          SCOPE         ACTIVE")
	fmt.Println("  --      ------                         ----          -----         ------")

	for _, r := range roles {
		scope := "-"
		if r.Scope != "" {
			scope = r.Scope
		}
		activeStr := "yes"
		if !r.IsActive {
			activeStr = "\033[90mno\033[0m"
		}
		fmt.Printf("  %-6d  %-30s %-13s %-13s %s\n",
			r.Id,
			truncateString(r.PersonName, 30),
			r.Role,
			truncateString(scope, 13),
			activeStr)
	}

	fmt.Println()
	return nil
}

func outputRoleSearchResultsProto(cfg *config.CLIConfig, roles []*productv1.ProductTeamRole) error {
	format := getProductOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputProductJSON(roles)
	case config.OutputFormatYAML:
		return outputProductYAML(roles)
	default:
		return outputRoleSearchResultsTableProto(roles)
	}
}

func outputRoleSearchResultsTableProto(roles []*productv1.ProductTeamRole) error {
	if len(roles) == 0 {
		fmt.Println("No matching roles found.")
		return nil
	}

	fmt.Printf("Found %d role(s):\n\n", len(roles))
	fmt.Println("  ID      PRODUCT              TEAM                 PERSON                    ROLE          SCOPE")
	fmt.Println("  --      -------              ----                 ------                    ----          -----")

	for _, r := range roles {
		scope := "-"
		if r.Scope != "" {
			scope = r.Scope
		}
		fmt.Printf("  %-6d  %-20s %-20s %-25s %-13s %s\n",
			r.Id,
			truncateString(r.ProductName, 20),
			truncateString(r.TeamName, 20),
			truncateString(r.PersonName, 25),
			r.Role,
			truncateString(scope, 13))
	}

	fmt.Println()
	return nil
}

func outputProductPeopleProto(cfg *config.CLIConfig, productName string, people []*productv1.ProductPersonSummary) error {
	format := getProductOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputProductJSON(people)
	case config.OutputFormatYAML:
		return outputProductYAML(people)
	default:
		return outputProductPeopleTableProto(productName, people)
	}
}

func outputProductPeopleTableProto(productName string, people []*productv1.ProductPersonSummary) error {
	if len(people) == 0 {
		fmt.Printf("No people assigned to '%s'\n", productName)
		return nil
	}

	// Count total roles across all people.
	totalRoles := 0
	for _, p := range people {
		totalRoles += len(p.Roles)
	}

	fmt.Printf("People on '%s' (%d people, %d roles):\n\n", productName, len(people), totalRoles)
	fmt.Println("  TEAM                 PERSON                         ROLE          SCOPE         EMAIL")
	fmt.Println("  ----                 ------                         ----          -----         -----")

	for _, p := range people {
		for _, r := range p.Roles {
			scope := "-"
			if r.Scope != "" {
				scope = r.Scope
			}
			fmt.Printf("  %-20s %-30s %-13s %-13s %s\n",
				truncateString(r.TeamName, 20),
				truncateString(r.PersonName, 30),
				r.Role,
				truncateString(scope, 13),
				r.PersonEmail)
		}
	}

	fmt.Println()
	return nil
}
