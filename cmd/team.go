// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"

	teamsv1 "github.com/otherjamesbrown/penf-cli/api/proto/teams/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Team command flags.
var (
	teamTenant      string
	teamOutput      string
	teamDescription string
	teamMemberRole  string
)

// TeamCommandDeps holds the dependencies for team commands.
type TeamCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultTeamDeps returns the default dependencies for production use.
func DefaultTeamDeps() *TeamCommandDeps {
	return &TeamCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewTeamCommand creates the root team command with all subcommands.
func NewTeamCommand(deps *TeamCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultTeamDeps()
	}

	cmd := &cobra.Command{
		Use:   "team",
		Short: "Manage teams for organizing people",
		Long: `Manage teams in Penfold for grouping people and assigning them to projects or products.

Teams represent organizational units like engineering teams, cross-functional squads,
or functional groups. Teams can be associated with projects and products to track
who is working on what.

Use Cases:
  - Define organizational structure (Engineering, Product, Design teams)
  - Group people by function or project
  - Assign teams to products with specific contexts (Core Team, DRI Team, etc.)
  - Track team membership over time

Team vs Project:
  - Teams are persistent organizational units (e.g., "Platform Team", "API Squad")
  - Projects are temporary initiatives (e.g., "Q1 Migration", "Product Launch")
  - Use 'penf project' to manage projects

Examples:
  # List all teams
  penf team list

  # Create a new team
  penf team create "Platform Team" --description "Platform infrastructure and services"

  # Show team details and members
  penf team show "Platform Team"

  # Add a member to a team
  penf team add-member "Platform Team" --email john@example.com --role lead

  # Output as JSON for programmatic use
  penf team list --output json

Related Commands:
  penf product team    Associate teams with products
  penf project         Manage projects (different from teams)`,
		Aliases: []string{"teams"},
	}

	// Add persistent flags.
	cmd.PersistentFlags().StringVarP(&teamTenant, "tenant", "t", "", "Tenant ID (overrides config)")
	cmd.PersistentFlags().StringVarP(&teamOutput, "output", "o", "", "Output format: text, json, yaml")

	// Add subcommands.
	cmd.AddCommand(newTeamListCommand(deps))
	cmd.AddCommand(newTeamCreateCommand(deps))
	cmd.AddCommand(newTeamShowCommand(deps))
	cmd.AddCommand(newTeamDeleteCommand(deps))
	cmd.AddCommand(newTeamAddMemberCommand(deps))
	cmd.AddCommand(newTeamRemoveMemberCommand(deps))
	cmd.AddCommand(newTeamMembersCommand(deps))

	return cmd
}

// newTeamListCommand creates the 'team list' subcommand.
func newTeamListCommand(deps *TeamCommandDeps) *cobra.Command {
	var nameSearch string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all teams",
		Long: `List all teams in the system.

Shows team names and descriptions.
Use --output json for programmatic access to full team details.

Filtering Options:
  --name    Search by name (partial match)

Examples:
  # List all teams (table format)
  penf team list

  # Search by name
  penf team list --name "platform"

  # List as JSON for programmatic use
  penf team list --output json

  # List as YAML
  penf team list --output yaml`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamList(cmd.Context(), deps, nameSearch)
		},
	}

	cmd.Flags().StringVar(&nameSearch, "name", "", "Search by name (partial match)")

	return cmd
}

// newTeamCreateCommand creates the 'team create' subcommand.
func newTeamCreateCommand(deps *TeamCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new team",
		Long: `Create a new team in the system.

Teams represent organizational units like engineering teams, product teams,
or cross-functional squads. After creating a team, use 'team add-member' to
add people to the team.

Examples:
  # Create a simple team
  penf team create "Platform Team"

  # Create with description
  penf team create "Platform Team" --description "Infrastructure and platform services"

  # Create a product team
  penf team create "LKE Core Team" --description "Core LKE engineering team"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamCreate(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVar(&teamDescription, "description", "", "Team description")

	return cmd
}

// newTeamShowCommand creates the 'team show' subcommand.
func newTeamShowCommand(deps *TeamCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show team details and members",
		Long: `Show detailed information about a specific team including all members.

Displays team properties and the list of people in the team with their roles.
Use --output json for full structured data.

The identifier can be:
  - Team name (case-sensitive match)
  - Team ID (numeric)

Examples:
  penf team show "Platform Team"
  penf team show "Platform Team" --output json`,
		Aliases: []string{"info", "get"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamShow(cmd.Context(), deps, args[0])
		},
	}
}

// newTeamDeleteCommand creates the 'team delete' subcommand.
func newTeamDeleteCommand(deps *TeamCommandDeps) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <identifier>",
		Short: "Delete a team",
		Long: `Delete a team from the system.

WARNING: This will remove the team and all member associations.
Product-team associations will also be removed.

The identifier can be:
  - Team ID (numeric)
  - Team name (case-sensitive)

By default, prompts for confirmation. Use --force to skip the prompt.

Examples:
  # Delete by ID
  penf team delete 5

  # Delete by name
  penf team delete "Platform Team"

  # Force delete without confirmation
  penf team delete "Platform Team" --force`,
		Aliases: []string{"rm", "remove"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamDelete(cmd.Context(), deps, args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

// newTeamAddMemberCommand creates the 'team add-member' subcommand.
func newTeamAddMemberCommand(deps *TeamCommandDeps) *cobra.Command {
	var email string

	cmd := &cobra.Command{
		Use:   "add-member <team>",
		Short: "Add a person to a team",
		Long: `Add a person to a team with an optional role.

The person must already exist in the system. Use 'penf entity seed' to add people first.

Roles are flexible strings like:
  - member (default)
  - lead
  - manager
  - contributor

Examples:
  # Add a member with default role
  penf team add-member "Platform Team" --email john@example.com

  # Add a lead
  penf team add-member "Platform Team" --email jane@example.com --role lead

  # Add a manager
  penf team add-member "Product Team" --email pm@example.com --role manager`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamAddMember(cmd.Context(), deps, args[0], email)
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Person's email address (required)")
	cmd.Flags().StringVar(&teamRole, "role", "member", "Role in the team")
	cmd.MarkFlagRequired("email")

	return cmd
}

// newTeamRemoveMemberCommand creates the 'team remove-member' subcommand.
func newTeamRemoveMemberCommand(deps *TeamCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove-member <member-id>",
		Short: "Remove a member from a team",
		Long: `Remove a member from a team by member ID.

Use 'team show' or 'team members' to find the member ID.

Examples:
  # Remove member by ID
  penf team remove-member 123`,
		Aliases: []string{"rm-member"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamRemoveMember(cmd.Context(), deps, args[0])
		},
	}
}

// newTeamMembersCommand creates the 'team members' subcommand.
func newTeamMembersCommand(deps *TeamCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "members <team>",
		Short: "List members of a team",
		Long: `List all members of a team with their roles.

The identifier can be:
  - Team name (case-sensitive)
  - Team ID (numeric)

Examples:
  penf team members "Platform Team"
  penf team members "Platform Team" --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamMembers(cmd.Context(), deps, args[0])
		},
	}
}

// ==================== gRPC Connection ====================

func connectTeamToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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

func getTenantIDForTeam(deps *TeamCommandDeps) (string, error) {
	if teamTenant != "" {
		return teamTenant, nil
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

func runTeamList(ctx context.Context, deps *TeamCommandDeps, nameSearch string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectTeamToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := teamsv1.NewTeamsServiceClient(conn)
	tenantID, err := getTenantIDForTeam(deps)
	if err != nil {
		return err
	}

	filter := &teamsv1.TeamFilter{
		TenantId:   tenantID,
		NameSearch: nameSearch,
	}

	resp, err := client.ListTeams(ctx, &teamsv1.ListTeamsRequest{Filter: filter})
	if err != nil {
		return fmt.Errorf("listing teams: %w", err)
	}

	return outputTeamsProto(cfg, resp.Teams)
}

func runTeamCreate(ctx context.Context, deps *TeamCommandDeps, name string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectTeamToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := teamsv1.NewTeamsServiceClient(conn)
	tenantID, err := getTenantIDForTeam(deps)
	if err != nil {
		return err
	}

	input := &teamsv1.TeamInput{
		Name:        name,
		Description: teamDescription,
	}

	resp, err := client.CreateTeam(ctx, &teamsv1.CreateTeamRequest{
		TenantId: tenantID,
		Input:    input,
	})
	if err != nil {
		return fmt.Errorf("creating team: %w", err)
	}

	fmt.Printf("\033[32mCreated team:\033[0m %s (ID: %d)\n", name, resp.Team.Id)
	if teamDescription != "" {
		fmt.Printf("  Description: %s\n", teamDescription)
	}

	teamDescription = ""

	return nil
}

func runTeamShow(ctx context.Context, deps *TeamCommandDeps, identifier string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectTeamToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := teamsv1.NewTeamsServiceClient(conn)
	tenantID, err := getTenantIDForTeam(deps)
	if err != nil {
		return err
	}

	resp, err := client.GetTeam(ctx, &teamsv1.GetTeamRequest{
		TenantId:   tenantID,
		Identifier: identifier,
	})
	if err != nil {
		return fmt.Errorf("team not found: %s", identifier)
	}

	// Get members for display
	membersResp, err := client.ListTeamMembers(ctx, &teamsv1.ListTeamMembersRequest{
		TenantId:       tenantID,
		TeamIdentifier: identifier,
	})
	if err != nil {
		return fmt.Errorf("failed to get members: %w", err)
	}

	return outputTeamDetailProto(cfg, resp.Team, membersResp.Members)
}

func runTeamDelete(ctx context.Context, deps *TeamCommandDeps, identifier string, force bool) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectTeamToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := teamsv1.NewTeamsServiceClient(conn)
	tenantID, err := getTenantIDForTeam(deps)
	if err != nil {
		return err
	}

	// First, resolve the identifier to get team details
	teamResp, err := client.GetTeam(ctx, &teamsv1.GetTeamRequest{
		TenantId:   tenantID,
		Identifier: identifier,
	})
	if err != nil {
		return fmt.Errorf("team not found: %s", identifier)
	}

	team := teamResp.Team

	// Prompt for confirmation unless --force is used
	if !force {
		fmt.Printf("Delete team \"%s\" (ID: %d)? This will remove all member associations. [y/N] ", team.Name, team.Id)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Delete the team by ID
	_, err = client.DeleteTeam(ctx, &teamsv1.DeleteTeamRequest{
		Id: team.Id,
	})
	if err != nil {
		return fmt.Errorf("deleting team: %w", err)
	}

	fmt.Printf("\033[32mDeleted team:\033[0m %s (ID: %d)\n", team.Name, team.Id)
	return nil
}

func runTeamAddMember(ctx context.Context, deps *TeamCommandDeps, teamName, email string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectTeamToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := teamsv1.NewTeamsServiceClient(conn)
	tenantID, err := getTenantIDForTeam(deps)
	if err != nil {
		return err
	}

	resp, err := client.AddTeamMember(ctx, &teamsv1.AddTeamMemberRequest{
		TenantId:         tenantID,
		TeamIdentifier:   teamName,
		PersonIdentifier: email,
		Role:             teamRole,
	})
	if err != nil {
		return fmt.Errorf("adding team member: %w", err)
	}

	fmt.Printf("\033[32mAdded member:\033[0m %s (%s) to %s as %s (ID: %d)\n",
		resp.Member.PersonName, email, teamName, resp.Member.Role, resp.Member.Id)

	teamRole = "member"

	return nil
}

func runTeamRemoveMember(ctx context.Context, deps *TeamCommandDeps, memberIDStr string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectTeamToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := teamsv1.NewTeamsServiceClient(conn)
	tenantID, err := getTenantIDForTeam(deps)
	if err != nil {
		return err
	}

	var memberID int64
	if _, err := fmt.Sscanf(memberIDStr, "%d", &memberID); err != nil {
		return fmt.Errorf("invalid member ID: %s", memberIDStr)
	}

	_, err = client.RemoveTeamMember(ctx, &teamsv1.RemoveTeamMemberRequest{
		TenantId: tenantID,
		MemberId: memberID,
	})
	if err != nil {
		return fmt.Errorf("removing team member: %w", err)
	}

	fmt.Printf("\033[32mRemoved member:\033[0m ID %d\n", memberID)
	return nil
}

func runTeamMembers(ctx context.Context, deps *TeamCommandDeps, identifier string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectTeamToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := teamsv1.NewTeamsServiceClient(conn)
	tenantID, err := getTenantIDForTeam(deps)
	if err != nil {
		return err
	}

	resp, err := client.ListTeamMembers(ctx, &teamsv1.ListTeamMembersRequest{
		TenantId:       tenantID,
		TeamIdentifier: identifier,
	})
	if err != nil {
		return fmt.Errorf("listing team members: %w", err)
	}

	return outputTeamMembersProto(cfg, identifier, resp.Members)
}

// ==================== Output Functions ====================

func getTeamOutputFormat(cfg *config.CLIConfig) config.OutputFormat {
	if teamOutput != "" {
		return config.OutputFormat(teamOutput)
	}
	if cfg != nil {
		return cfg.OutputFormat
	}
	return config.OutputFormatText
}

func outputTeamsProto(cfg *config.CLIConfig, teams []*teamsv1.Team) error {
	format := getTeamOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputTeamJSON(teams)
	case config.OutputFormatYAML:
		return outputTeamYAML(teams)
	default:
		return outputTeamsTableProto(teams)
	}
}

func outputTeamsTableProto(teams []*teamsv1.Team) error {
	if len(teams) == 0 {
		fmt.Println("No teams found.")
		return nil
	}

	fmt.Printf("Teams (%d):\n\n", len(teams))
	fmt.Println("  ID      NAME                           DESCRIPTION")
	fmt.Println("  --      ----                           -----------")

	for _, t := range teams {
		descStr := "-"
		if t.Description != "" {
			descStr = teamTruncateString(t.Description, 50)
		}
		fmt.Printf("  %-6d  %-30s %s\n",
			t.Id,
			teamTruncateString(t.Name, 30),
			descStr)
	}

	fmt.Println()
	return nil
}

func outputTeamDetailProto(cfg *config.CLIConfig, team *teamsv1.Team, members []*teamsv1.TeamMember) error {
	format := getTeamOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		data := map[string]interface{}{
			"team":    team,
			"members": members,
		}
		return outputTeamJSON(data)
	case config.OutputFormatYAML:
		data := map[string]interface{}{
			"team":    team,
			"members": members,
		}
		return outputTeamYAML(data)
	default:
		return outputTeamDetailTextProto(team, members)
	}
}

func outputTeamDetailTextProto(team *teamsv1.Team, members []*teamsv1.TeamMember) error {
	fmt.Println("Team Details:")
	fmt.Println()
	fmt.Printf("  \033[1mID:\033[0m          %d\n", team.Id)
	fmt.Printf("  \033[1mName:\033[0m        %s\n", team.Name)

	if team.Description != "" {
		fmt.Printf("  \033[1mDescription:\033[0m %s\n", team.Description)
	}

	fmt.Println()

	if team.CreatedAt != nil {
		fmt.Printf("  \033[1mCreated:\033[0m     %s\n", team.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}
	if team.UpdatedAt != nil {
		fmt.Printf("  \033[1mUpdated:\033[0m     %s\n", team.UpdatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}

	fmt.Println()
	fmt.Printf("  \033[1mMembers:\033[0m     %d\n", len(members))

	if len(members) > 0 {
		fmt.Println()
		fmt.Println("  ID      NAME                           EMAIL                          ROLE          JOINED")
		fmt.Println("  --      ----                           -----                          ----          ------")

		for _, m := range members {
			joinedStr := ""
			if m.JoinedAt != nil {
				joinedStr = m.JoinedAt.AsTime().Format("2006-01-02")
			}
			fmt.Printf("  %-6d  %-30s %-30s %-13s %s\n",
				m.Id,
				teamTruncateString(m.PersonName, 30),
				teamTruncateString(m.PersonEmail, 30),
				m.Role,
				joinedStr)
		}
	}

	fmt.Println()
	return nil
}

func outputTeamMembersProto(cfg *config.CLIConfig, teamName string, members []*teamsv1.TeamMember) error {
	format := getTeamOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputTeamJSON(members)
	case config.OutputFormatYAML:
		return outputTeamYAML(members)
	default:
		return outputTeamMembersTableProto(teamName, members)
	}
}

func outputTeamMembersTableProto(teamName string, members []*teamsv1.TeamMember) error {
	if len(members) == 0 {
		fmt.Printf("No members in team '%s'\n", teamName)
		return nil
	}

	fmt.Printf("Members of '%s' (%d):\n\n", teamName, len(members))
	fmt.Println("  ID      NAME                           EMAIL                          ROLE          JOINED")
	fmt.Println("  --      ----                           -----                          ----          ------")

	for _, m := range members {
		joinedStr := ""
		if m.JoinedAt != nil {
			joinedStr = m.JoinedAt.AsTime().Format("2006-01-02")
		}
		fmt.Printf("  %-6d  %-30s %-30s %-13s %s\n",
			m.Id,
			teamTruncateString(m.PersonName, 30),
			teamTruncateString(m.PersonEmail, 30),
			m.Role,
			joinedStr)
	}

	fmt.Println()
	return nil
}

// Helper output functions.

func outputTeamJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputTeamYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

func teamTruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
