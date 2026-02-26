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
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Project command flags.
var (
	projectTenant      string
	projectOutput      string
	projectDescription string
	projectKeywords    []string
)

// ProjectCommandDeps holds the dependencies for project commands.
// All commands use gRPC via the gateway - CLI must NEVER access the database directly.
type ProjectCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultProjectDeps returns the default dependencies for production use.
func DefaultProjectDeps() *ProjectCommandDeps {
	return &ProjectCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewProjectCommand creates the root project command with all subcommands.
func NewProjectCommand(deps *ProjectCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultProjectDeps()
	}

	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects for content tagging and organization",
		Long: `Manage projects in Penfold for content organization and tagging.

Projects represent organizational initiatives like migration projects, product launches,
or cross-team efforts. Content (emails, meetings, documents) can be automatically
tagged to projects based on keywords or Jira project keys.

Use Cases:
  - Track discussions across email/Slack/meetings for a specific project
  - Group related content for easier search and reporting
  - Link projects to products for portfolio management

Project vs Product:
  - Projects are temporary initiatives (e.g., "MTC Migration", "Q3 Launch")
  - Products are persistent entities (e.g., "LKE", "Managed Databases")
  - Projects may involve multiple products
  - Use 'penf product' to manage products

Examples:
  # List all projects
  penf project list

  # Add a new project with keywords for auto-tagging
  penf project add "MTC" --description "TikTok Migration" --keywords "tiktok,mtc,migration"

  # Show project details
  penf project show "MTC"

  # Output as JSON for programmatic use
  penf project list --output json

Related Commands:
  penf product       Manage products (different from projects)
  penf search        Search content including project-tagged items
  penf glossary      Manage acronyms (projects may have acronyms)`,
		Aliases: []string{"proj", "projects"},
	}

	// Add persistent flags.
	cmd.PersistentFlags().StringVarP(&projectTenant, "tenant", "t", "", "Tenant ID (overrides config)")
	cmd.PersistentFlags().StringVarP(&projectOutput, "output", "o", "", "Output format: text, json, yaml")

	// Add subcommands.
	cmd.AddCommand(newProjectListCommand(deps))
	cmd.AddCommand(newProjectAddCommand(deps))
	cmd.AddCommand(newProjectShowCommand(deps))
	cmd.AddCommand(newProjectDeleteCommand(deps))
	cmd.AddCommand(newProjectUpdateCommand(deps))

	return cmd
}

// newProjectListCommand creates the 'project list' subcommand.
func newProjectListCommand(deps *ProjectCommandDeps) *cobra.Command {
	var nameSearch string
	var keyword string
	var statusFilter string
	var sortBy string
	var limit int32
	var alwaysInclude []string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		Long: `List all projects in the system.

Shows project names, descriptions, and keyword counts.
Use --output json for programmatic access to full project details.

Filtering Options:
  --name            Search by name (partial match)
  --keyword         Filter by keyword (projects containing this keyword)
  --status          Filter by status ("active", "archived")
  --sort            Sort by: "name", "activity", "created" (default: "name")
  --limit           Maximum number of results (default: 100)
  --always-include  Always include these project names (can be repeated)

Examples:
  # List all projects (table format)
  penf project list

  # Search by name
  penf project list --name "migration"

  # Filter by keyword
  penf project list --keyword "tiktok"

  # List active projects sorted by activity
  penf project list --status active --sort activity --limit 3

  # Always include a specific project even if filtered
  penf project list --status active --always-include "MTC 2026"

  # List as JSON for programmatic use
  penf project list --output json

  # List as YAML
  penf project list --output yaml`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectList(cmd.Context(), deps, nameSearch, keyword, statusFilter, sortBy, limit, alwaysInclude)
		},
	}

	cmd.Flags().StringVar(&nameSearch, "name", "", "Search by name (partial match)")
	cmd.Flags().StringVar(&keyword, "keyword", "", "Filter by keyword")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (active, archived)")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort by: name, activity, created")
	cmd.Flags().Int32Var(&limit, "limit", 0, "Maximum number of results")
	cmd.Flags().StringSliceVar(&alwaysInclude, "always-include", nil, "Always include these project names (can be repeated)")

	return cmd
}

// newProjectAddCommand creates the 'project add' subcommand.
func newProjectAddCommand(deps *ProjectCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new project",
		Long: `Add a new project to the system.

Creates a project with the specified name. Add keywords to enable
automatic content tagging - any email, meeting, or document mentioning
these keywords will be associated with this project.

Keywords are case-insensitive. Common patterns:
  - Project acronyms: "MTC", "DBaaS", "K8s"
  - Full project names: "TikTok Migration"
  - Jira project keys: "PROJ-" prefix patterns

Examples:
  # Add a simple project
  penf project add "MTC"

  # Add with description
  penf project add "MTC" --description "TikTok Migration to our platform"

  # Add with keywords for auto-tagging
  penf project add "MTC" --description "TikTok Migration" --keywords "tiktok,mtc,migration,tikcloud"

  # Multiple keywords can also be specified separately
  penf project add "MTC" --keywords "tiktok" --keywords "mtc" --keywords "migration"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectAdd(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVar(&projectDescription, "description", "", "Project description")
	cmd.Flags().StringSliceVar(&projectKeywords, "keywords", nil, "Keywords for auto-tagging (comma-separated)")

	return cmd
}

// newProjectShowCommand creates the 'project show' subcommand.
func newProjectShowCommand(deps *ProjectCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show project details",
		Long: `Show detailed information about a specific project.

Displays all project properties including keywords and Jira integrations.
Use --output json for full structured data.

The identifier can be:
  - Project name (case-insensitive)
  - Project ID (numeric)

Examples:
  penf project show "MTC"
  penf project show "MTC" --output json`,
		Aliases: []string{"info", "get"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectShow(cmd.Context(), deps, args[0])
		},
	}
}

// newProjectDeleteCommand creates the 'project delete' subcommand.
func newProjectDeleteCommand(deps *ProjectCommandDeps) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <identifier>",
		Short: "Delete a project",
		Long: `Delete a project from the system.

This removes the project and its keyword associations. Content previously
tagged with this project will no longer be associated with it.

The identifier can be:
  - Project ID (numeric)
  - Project name (case-insensitive)

By default, prompts for confirmation. Use --force to skip the prompt.

Examples:
  # Delete by ID
  penf project delete 2

  # Delete by name
  penf project delete "MTC"

  # Force delete without confirmation
  penf project delete "MTC" --force`,
		Aliases: []string{"rm", "remove"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectDelete(cmd.Context(), deps, args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

// newProjectUpdateCommand creates the 'project update' subcommand.
func newProjectUpdateCommand(deps *ProjectCommandDeps) *cobra.Command {
	var updateName string
	var updateDescription string
	var updateKeywords []string

	cmd := &cobra.Command{
		Use:   "update <name-or-id>",
		Short: "Update a project",
		Long: `Update an existing project's name, description, or keywords.

Only specified flags are changed — omitted fields are preserved.

Examples:
  # Update description only
  penf project update MTC --description "Major TikTok migration project"

  # Update keywords
  penf project update MTC --keywords mtc,tiktok,migration

  # Update name
  penf project update MTC --name "MTC Phase 2"

  # Update multiple fields
  penf project update MTC --description "Updated desc" --keywords new,kw`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nameSet := cmd.Flags().Changed("name")
			descSet := cmd.Flags().Changed("description")
			kwSet := cmd.Flags().Changed("keywords")
			return runProjectUpdate(cmd.Context(), deps, args[0], updateName, updateDescription, updateKeywords, nameSet, descSet, kwSet)
		},
	}

	cmd.Flags().StringVar(&updateName, "name", "", "New project name")
	cmd.Flags().StringVarP(&updateDescription, "description", "d", "", "New description")
	cmd.Flags().StringSliceVarP(&updateKeywords, "keywords", "k", nil, "New keywords (comma-separated)")

	return cmd
}

// ==================== gRPC Connection ====================

// connectProjectToGateway creates a gRPC connection to the gateway service.
func connectProjectToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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

// getTenantIDForProject returns the tenant ID from flag, env, or config.
func getTenantIDForProject(deps *ProjectCommandDeps) string {
	if projectTenant != "" {
		return projectTenant
	}
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant
	}
	if deps.Config != nil && deps.Config.TenantID != "" {
		return deps.Config.TenantID
	}
	// Default tenant.
	return "00000001-0000-0000-0000-000000000001"
}

// ==================== Command Execution Functions ====================

// runProjectList executes the project list command via gRPC.
func runProjectList(ctx context.Context, deps *ProjectCommandDeps, nameSearch, keyword, statusFilter, sortBy string, limit int32, alwaysInclude []string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProjectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := projectv1.NewProjectServiceClient(conn)
	tenantID := getTenantIDForProject(deps)

	filter := &projectv1.ProjectFilter{
		TenantId:          tenantID,
		NameSearch:        nameSearch,
		Keyword:           keyword,
		Status:            statusFilter,
		SortBy:            sortBy,
		Limit:             limit,
		AlwaysIncludeNames: alwaysInclude,
	}

	resp, err := client.ListProjects(ctx, &projectv1.ListProjectsRequest{Filter: filter})
	if err != nil {
		return fmt.Errorf("listing projects: %w", err)
	}

	return outputProjectsProto(cfg, resp.Projects)
}

// runProjectAdd executes the project add command via gRPC.
func runProjectAdd(ctx context.Context, deps *ProjectCommandDeps, name string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProjectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := projectv1.NewProjectServiceClient(conn)
	tenantID := getTenantIDForProject(deps)

	// Clean up keywords.
	var cleanKeywords []string
	for _, kw := range projectKeywords {
		kw = strings.TrimSpace(kw)
		if kw != "" {
			cleanKeywords = append(cleanKeywords, kw)
		}
	}

	input := &projectv1.ProjectInput{
		Name:        name,
		Description: projectDescription,
		Keywords:    cleanKeywords,
	}

	resp, err := client.CreateProject(ctx, &projectv1.CreateProjectRequest{
		TenantId: tenantID,
		Input:    input,
	})
	if err != nil {
		return fmt.Errorf("creating project: %w", err)
	}

	fmt.Printf("\033[32mCreated project:\033[0m %s (ID: %d)\n", name, resp.Project.Id)
	if len(cleanKeywords) > 0 {
		fmt.Printf("  Keywords: %s\n", strings.Join(cleanKeywords, ", "))
	}

	// Reset flags for next call.
	projectDescription = ""
	projectKeywords = nil

	return nil
}

// runProjectShow executes the project show command via gRPC.
func runProjectShow(ctx context.Context, deps *ProjectCommandDeps, identifier string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProjectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := projectv1.NewProjectServiceClient(conn)
	tenantID := getTenantIDForProject(deps)

	resp, err := client.GetProject(ctx, &projectv1.GetProjectRequest{
		TenantId:   tenantID,
		Identifier: identifier,
	})
	if err != nil {
		return fmt.Errorf("project not found: %s", identifier)
	}

	return outputProjectDetailProto(cfg, resp.Project)
}

// runProjectDelete executes the project delete command via gRPC.
func runProjectDelete(ctx context.Context, deps *ProjectCommandDeps, identifier string, force bool) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProjectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := projectv1.NewProjectServiceClient(conn)
	tenantID := getTenantIDForProject(deps)

	// First, resolve the identifier to get project details
	projectResp, err := client.GetProject(ctx, &projectv1.GetProjectRequest{
		TenantId:   tenantID,
		Identifier: identifier,
	})
	if err != nil {
		return fmt.Errorf("project not found: %s", identifier)
	}

	project := projectResp.Project

	// Prompt for confirmation unless --force is used
	if !force {
		fmt.Printf("Delete project \"%s\" (ID: %d)? [y/N] ", project.Name, project.Id)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Delete the project by ID
	_, err = client.DeleteProject(ctx, &projectv1.DeleteProjectRequest{
		Id: project.Id,
	})
	if err != nil {
		return fmt.Errorf("deleting project: %w", err)
	}

	fmt.Printf("\033[32mDeleted project:\033[0m %s (ID: %d)\n", project.Name, project.Id)
	return nil
}

// runProjectUpdate executes the project update command via gRPC.
func runProjectUpdate(ctx context.Context, deps *ProjectCommandDeps, identifier, name, description string, keywords []string, nameSet, descSet, kwSet bool) error {
	if !nameSet && !descSet && !kwSet {
		return fmt.Errorf("at least one of --name, --description, or --keywords is required")
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProjectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := projectv1.NewProjectServiceClient(conn)
	tenantID := getTenantIDForProject(deps)

	// Resolve identifier to get current project
	projectResp, err := client.GetProject(ctx, &projectv1.GetProjectRequest{
		TenantId:   tenantID,
		Identifier: identifier,
	})
	if err != nil {
		return fmt.Errorf("project not found: %s", identifier)
	}

	current := projectResp.Project

	// Build input preserving unchanged fields
	input := &projectv1.ProjectInput{
		Name:        current.Name,
		Description: current.Description,
		Keywords:    current.Keywords,
	}
	if nameSet {
		input.Name = name
	}
	if descSet {
		input.Description = description
	}
	if kwSet {
		var cleanKeywords []string
		for _, kw := range keywords {
			kw = strings.TrimSpace(kw)
			if kw != "" {
				cleanKeywords = append(cleanKeywords, kw)
			}
		}
		input.Keywords = cleanKeywords
	}

	resp, err := client.UpdateProject(ctx, &projectv1.UpdateProjectRequest{
		Id:    current.Id,
		Input: input,
	})
	if err != nil {
		return fmt.Errorf("updating project: %w", err)
	}

	p := resp.Project
	fmt.Printf("\033[32mUpdated project:\033[0m %s (ID: %d)\n", p.Name, p.Id)
	if descSet {
		fmt.Printf("  Description: %s\n", p.Description)
	}
	if kwSet {
		fmt.Printf("  Keywords:    %s\n", strings.Join(p.Keywords, ", "))
	}

	return nil
}

// ==================== Output Functions ====================

// getProjectOutputFormat returns the output format from flag or config.
func getProjectOutputFormat(cfg *config.CLIConfig) config.OutputFormat {
	if projectOutput != "" {
		return config.OutputFormat(projectOutput)
	}
	if cfg != nil {
		return cfg.OutputFormat
	}
	return config.OutputFormatText
}

// outputProjectsProto outputs a list of projects from proto messages.
func outputProjectsProto(cfg *config.CLIConfig, projects []*projectv1.Project) error {
	format := getProjectOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputProjectJSON(projects)
	case config.OutputFormatYAML:
		return outputProjectYAML(projects)
	default:
		return outputProjectsTableProto(projects)
	}
}

// outputProjectsTableProto outputs projects in table format.
func outputProjectsTableProto(projects []*projectv1.Project) error {
	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return nil
	}

	fmt.Printf("Projects (%d):\n\n", len(projects))
	fmt.Println("  ID      NAME                           DESCRIPTION                    KEYWORDS")
	fmt.Println("  --      ----                           -----------                    --------")

	for _, p := range projects {
		keywordStr := "-"
		if len(p.Keywords) > 0 {
			keywordStr = fmt.Sprintf("%d keywords", len(p.Keywords))
		}
		descStr := "-"
		if p.Description != "" {
			descStr = projectTruncateString(p.Description, 30)
		}
		fmt.Printf("  %-6d  %-30s %-30s %s\n",
			p.Id,
			projectTruncateString(p.Name, 30),
			descStr,
			keywordStr)
	}

	fmt.Println()
	return nil
}

// outputProjectDetailProto outputs detailed project information from proto message.
func outputProjectDetailProto(cfg *config.CLIConfig, project *projectv1.Project) error {
	format := getProjectOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputProjectJSON(project)
	case config.OutputFormatYAML:
		return outputProjectYAML(project)
	default:
		return outputProjectDetailTextProto(project)
	}
}

// outputProjectDetailTextProto outputs project info in human-readable format.
func outputProjectDetailTextProto(project *projectv1.Project) error {
	fmt.Println("Project Details:")
	fmt.Println()
	fmt.Printf("  \033[1mID:\033[0m          %d\n", project.Id)
	fmt.Printf("  \033[1mName:\033[0m        %s\n", project.Name)

	if project.Description != "" {
		fmt.Printf("  \033[1mDescription:\033[0m %s\n", project.Description)
	}

	fmt.Println()

	if len(project.Keywords) > 0 {
		fmt.Printf("  \033[1mKeywords:\033[0m    %s\n", strings.Join(project.Keywords, ", "))
	} else {
		fmt.Printf("  \033[1mKeywords:\033[0m    (none - add with 'penf project add' or update project)\n")
	}

	if len(project.JiraProjects) > 0 {
		fmt.Printf("  \033[1mJira:\033[0m        %s\n", strings.Join(project.JiraProjects, ", "))
	}

	fmt.Println()
	if project.CreatedAt != nil {
		fmt.Printf("  \033[1mCreated:\033[0m     %s\n", project.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}
	if project.UpdatedAt != nil {
		fmt.Printf("  \033[1mUpdated:\033[0m     %s\n", project.UpdatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}

	return nil
}

// Helper output functions.

func outputProjectJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputProjectYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

// projectTruncateString truncates a string to maxLen, adding "..." if truncated.
func projectTruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
