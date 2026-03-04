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

	contentv1 "github.com/otherjamesbrown/penf-cli/api/proto/content/v1"
	projectv1 "github.com/otherjamesbrown/penf-cli/api/proto/project/v1"
	topicv1 "github.com/otherjamesbrown/penf-cli/api/proto/topic/v1"
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
	cmd.AddCommand(newProjectThemesCommand(deps))
	cmd.AddCommand(newProjectContentCommand(deps))
	cmd.AddCommand(newProjectStatsCommand(deps))
	cmd.AddCommand(newProjectUnattributedCommand(deps))

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
			descStr = truncateString(p.Description, 30)
		}
		fmt.Printf("  %-6d  %-30s %-30s %s\n",
			p.Id,
			truncateString(p.Name, 30),
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

	if project.Timeline != "" {
		var tl map[string]interface{}
		if json.Unmarshal([]byte(project.Timeline), &tl) == nil {
			fmt.Println()
			fmt.Println("  \033[1mTimeline:\033[0m")
			if phase, ok := tl["current_phase"].(string); ok {
				fmt.Printf("    Phase: %s\n", phase)
			}
			if milestones, ok := tl["milestones"].([]interface{}); ok {
				for _, m := range milestones {
					if ms, ok := m.(map[string]interface{}); ok {
						fmt.Printf("    %s — %s\n", ms["date"], ms["label"])
					}
				}
			}
		}
	}

	if project.Metadata != "" {
		var md map[string]interface{}
		if json.Unmarshal([]byte(project.Metadata), &md) == nil && len(md) > 0 {
			fmt.Println()
			fmt.Println("  \033[1mMetadata:\033[0m")
			for k, v := range md {
				fmt.Printf("    %s: %v\n", k, v)
			}
		}
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

// newProjectThemesCommand creates the 'project themes' subcommand.
func newProjectThemesCommand(deps *ProjectCommandDeps) *cobra.Command {
	var limit int32

	cmd := &cobra.Command{
		Use:   "themes <name-or-id>",
		Short: "List topics scoped to a project",
		Long: `Show topics (themes) associated with a project.

Topics linked to a project via project_id are shown with their
running context and status.

Examples:
  penf project themes "API Migration"
  penf project themes 42
  penf project themes MTC --limit 100`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectThemes(cmd.Context(), deps, args[0], limit)
		},
	}

	cmd.Flags().Int32VarP(&limit, "limit", "l", 50, "Maximum number of topics to return")

	return cmd
}

func runProjectThemes(ctx context.Context, deps *ProjectCommandDeps, identifier string, limit int32) error {
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

	// Resolve project to get its ID
	projClient := projectv1.NewProjectServiceClient(conn)
	tenantID := getTenantIDForProject(deps)

	projResp, err := projClient.GetProject(ctx, &projectv1.GetProjectRequest{
		TenantId:   tenantID,
		Identifier: identifier,
	})
	if err != nil {
		return fmt.Errorf("project not found: %s", identifier)
	}

	project := projResp.Project

	// Delegate topic listing to topic.go via exported helper
	topicDeps := &TopicCommandDeps{LoadConfig: deps.LoadConfig}
	topics, err := ListTopicsByProject(ctx, topicDeps, project.Id, limit)
	if err != nil {
		return err
	}

	format := getProjectOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputProjectJSON(topics)
	case config.OutputFormatYAML:
		return outputProjectYAML(topics)
	default:
		return outputProjectThemesText(project.Name, topics)
	}
}

func outputProjectThemesText(projectName string, topics []*topicv1.Topic) error {
	if len(topics) == 0 {
		fmt.Printf("No topics linked to project \"%s\".\n", projectName)
		return nil
	}

	fmt.Printf("Topics for \"%s\" (%d):\n\n", projectName, len(topics))
	fmt.Println("  ID    NAME                 STATUS    CONTEXT")
	fmt.Println("  --    ----                 ------    -------")

	for _, t := range topics {
		status := t.Status
		if status == "" {
			status = "active"
		}
		fmt.Printf("  %-5d %-20s %-9s %s\n",
			t.Id,
			truncateString(t.Name, 20),
			status,
			truncateString(t.RunningContext, 50))
	}

	fmt.Println()
	return nil
}

// ==================== Project Observability Commands ====================

// newProjectContentCommand creates 'project content <project>' — list attributed content.
func newProjectContentCommand(deps *ProjectCommandDeps) *cobra.Command {
	var since string
	var until string
	var pageSize int32

	cmd := &cobra.Command{
		Use:   "content <project>",
		Short: "List content attributed to a project",
		Long: `List content items (emails, meetings, documents) attributed to a project.

Shows title, date, attribution source (channel_mapping/keyword/llm), and confidence.

Examples:
  penf project content MTC
  penf project content MTC --since 2026-01-01
  penf project content MTC -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectContent(cmd.Context(), deps, args[0], since, until, pageSize)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Show content created after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&until, "until", "", "Show content created before this date (YYYY-MM-DD)")
	cmd.Flags().Int32Var(&pageSize, "limit", 50, "Maximum number of results")

	return cmd
}

// newProjectStatsCommand creates 'project stats <project>' — attribution breakdown.
func newProjectStatsCommand(deps *ProjectCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats <project>",
		Short: "Show attribution statistics for a project",
		Long: `Show attribution statistics for a project.

Displays total attributed content and assertions, broken down by attribution source
(channel_mapping, keyword, participant, llm).

Examples:
  penf project stats MTC
  penf project stats MTC -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectStats(cmd.Context(), deps, args[0])
		},
	}

	return cmd
}

// newProjectUnattributedCommand creates 'project unattributed' — list unattributed content.
func newProjectUnattributedCommand(deps *ProjectCommandDeps) *cobra.Command {
	var since string
	var limit int32

	cmd := &cobra.Command{
		Use:   "unattributed",
		Short: "List content with no project attribution",
		Long: `List content items that have not been attributed to any project.

Useful for finding content that should be tagged but isn't — helps identify
sources that need 'penf source tag' mappings.

Examples:
  penf project unattributed
  penf project unattributed --since 2026-01-01
  penf project unattributed --limit 20 -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectUnattributed(cmd.Context(), deps, since, limit)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Show content created after this date (YYYY-MM-DD)")
	cmd.Flags().Int32Var(&limit, "limit", 50, "Maximum number of results")

	return cmd
}

// runProjectContent implements 'penf project content'.
func runProjectContent(ctx context.Context, deps *ProjectCommandDeps, identifier, since, until string, pageSize int32) error {
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

	tenantID := getTenantIDForProject(deps)

	// Resolve project name → ID
	projClient := projectv1.NewProjectServiceClient(conn)
	projResp, err := projClient.GetProject(ctx, &projectv1.GetProjectRequest{
		TenantId:   tenantID,
		Identifier: identifier,
	})
	if err != nil {
		return fmt.Errorf("project not found: %s", identifier)
	}
	projectID := projResp.Project.Id

	req := &contentv1.ListProjectContentRequest{
		TenantId:  tenantID,
		ProjectId: projectID,
		PageSize:  pageSize,
	}

	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			return fmt.Errorf("invalid --since date (use YYYY-MM-DD): %w", err)
		}
		req.Since = timestamppb.New(t)
	}
	if until != "" {
		t, err := time.Parse("2006-01-02", until)
		if err != nil {
			return fmt.Errorf("invalid --until date (use YYYY-MM-DD): %w", err)
		}
		req.Until = timestamppb.New(t)
	}

	contentClient := contentv1.NewContentProcessorServiceClient(conn)
	resp, err := contentClient.ListProjectContent(ctx, req)
	if err != nil {
		return fmt.Errorf("listing project content: %w", err)
	}

	format := projectOutput
	if f, _ := cmd_outputFormat(cfg); f != "" {
		format = f
	}

	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"project":     projResp.Project.Name,
			"project_id":  projectID,
			"total_count": resp.TotalCount,
			"items":       resp.Items,
		})
	case "yaml":
		return yaml.NewEncoder(os.Stdout).Encode(map[string]interface{}{
			"project":     projResp.Project.Name,
			"project_id":  projectID,
			"total_count": resp.TotalCount,
			"items":       resp.Items,
		})
	default:
		return outputProjectContentText(projResp.Project.Name, resp)
	}
}

// runProjectStats implements 'penf project stats'.
func runProjectStats(ctx context.Context, deps *ProjectCommandDeps, identifier string) error {
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

	tenantID := getTenantIDForProject(deps)

	projClient := projectv1.NewProjectServiceClient(conn)
	projResp, err := projClient.GetProject(ctx, &projectv1.GetProjectRequest{
		TenantId:   tenantID,
		Identifier: identifier,
	})
	if err != nil {
		return fmt.Errorf("project not found: %s", identifier)
	}
	projectID := projResp.Project.Id

	contentClient := contentv1.NewContentProcessorServiceClient(conn)
	resp, err := contentClient.GetProjectStats(ctx, &contentv1.GetProjectStatsRequest{
		TenantId:  tenantID,
		ProjectId: projectID,
	})
	if err != nil {
		return fmt.Errorf("getting project stats: %w", err)
	}

	format := projectOutput
	if f, _ := cmd_outputFormat(cfg); f != "" {
		format = f
	}

	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"project":                   projResp.Project.Name,
			"project_id":                projectID,
			"total_attributed_sources":  resp.TotalAttributedSources,
			"total_attributed_assertions": resp.TotalAttributedAssertions,
			"breakdown":                 resp.Breakdown,
		})
	case "yaml":
		return yaml.NewEncoder(os.Stdout).Encode(map[string]interface{}{
			"project":                   projResp.Project.Name,
			"total_attributed_sources":  resp.TotalAttributedSources,
			"total_attributed_assertions": resp.TotalAttributedAssertions,
			"breakdown":                 resp.Breakdown,
		})
	default:
		return outputProjectStatsText(projResp.Project.Name, resp)
	}
}

// runProjectUnattributed implements 'penf project unattributed'.
func runProjectUnattributed(ctx context.Context, deps *ProjectCommandDeps, since string, limit int32) error {
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

	tenantID := getTenantIDForProject(deps)

	req := &contentv1.ListUnattributedContentRequest{
		TenantId: tenantID,
		Limit:    limit,
	}

	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			return fmt.Errorf("invalid --since date (use YYYY-MM-DD): %w", err)
		}
		req.Since = timestamppb.New(t)
	}

	contentClient := contentv1.NewContentProcessorServiceClient(conn)
	resp, err := contentClient.ListUnattributedContent(ctx, req)
	if err != nil {
		return fmt.Errorf("listing unattributed content: %w", err)
	}

	format := projectOutput
	if f, _ := cmd_outputFormat(cfg); f != "" {
		format = f
	}

	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"total_count": resp.TotalCount,
			"items":       resp.Items,
		})
	case "yaml":
		return yaml.NewEncoder(os.Stdout).Encode(map[string]interface{}{
			"total_count": resp.TotalCount,
			"items":       resp.Items,
		})
	default:
		return outputProjectUnattributedText(resp)
	}
}

// cmd_outputFormat returns the output format from config (helper to avoid import cycle).
func cmd_outputFormat(cfg *config.CLIConfig) (string, error) {
	if projectOutput != "" {
		return projectOutput, nil
	}
	if cfg != nil {
		return string(cfg.OutputFormat), nil
	}
	return "", nil
}

// ==================== Output Formatters ====================

func outputProjectContentText(projectName string, resp *contentv1.ListProjectContentResponse) error {
	if len(resp.Items) == 0 {
		fmt.Printf("No content attributed to project '%s'.\n", projectName)
		fmt.Println("Tip: run 'penf source tag <identifier> --project <name>' to map a source.")
		return nil
	}

	fmt.Printf("Content attributed to %s (%d items):\n\n", projectName, resp.TotalCount)
	fmt.Printf("  %-8s  %-45s  %-12s  %-16s  %s\n",
		"ID", "TITLE", "DATE", "ATTRIBUTION", "CONF")
	fmt.Printf("  %-8s  %-45s  %-12s  %-16s  %s\n",
		"--", "-----", "----", "-----------", "----")

	for _, item := range resp.Items {
		dateStr := "-"
		if item.CreatedAt != nil {
			dateStr = item.CreatedAt.AsTime().Format("2006-01-02")
		}
		fmt.Printf("  %-8d  %-45s  %-12s  %-16s  %.2f\n",
			item.SourceId,
			truncateString(item.Title, 45),
			dateStr,
			item.AttributionSource,
			item.AttributionConfidence)
	}
	fmt.Println()
	return nil
}

func outputProjectStatsText(projectName string, resp *contentv1.GetProjectStatsResponse) error {
	fmt.Printf("Attribution stats for %s:\n\n", projectName)
	fmt.Printf("  Total attributed content:    %d\n", resp.TotalAttributedSources)
	fmt.Printf("  Total attributed assertions: %d\n\n", resp.TotalAttributedAssertions)

	if len(resp.Breakdown) == 0 {
		fmt.Println("  No attribution data yet.")
		fmt.Println("  Tip: run 'penf pipeline reprocess <id>' on recent content to trigger attribution.")
		return nil
	}

	fmt.Printf("  %-20s  %10s  %10s\n", "ATTRIBUTION SOURCE", "ASSERTIONS", "SOURCES")
	fmt.Printf("  %-20s  %10s  %10s\n", "------------------", "----------", "-------")
	for _, b := range resp.Breakdown {
		fmt.Printf("  %-20s  %10d  %10d\n", b.AttributionSource, b.AssertionCount, b.SourceCount)
	}
	fmt.Println()
	return nil
}

func outputProjectUnattributedText(resp *contentv1.ListUnattributedContentResponse) error {
	if len(resp.Items) == 0 {
		fmt.Println("No unattributed content found.")
		return nil
	}

	fmt.Printf("Unattributed content (%d items):\n\n", resp.TotalCount)
	fmt.Printf("  %-8s  %-45s  %-12s  %-12s  %s\n",
		"ID", "TITLE", "DATE", "TYPE", "ASSERTIONS")
	fmt.Printf("  %-8s  %-45s  %-12s  %-12s  %s\n",
		"--", "-----", "----", "----", "----------")

	for _, item := range resp.Items {
		dateStr := "-"
		if item.CreatedAt != nil {
			dateStr = item.CreatedAt.AsTime().Format("2006-01-02")
		}
		fmt.Printf("  %-8d  %-45s  %-12s  %-12s  %d\n",
			item.SourceId,
			truncateString(item.Title, 45),
			dateStr,
			item.ContentType,
			item.AssertionCount)
	}
	fmt.Println()
	fmt.Println("Tip: use 'penf source tag <identifier> --project <name>' to attribute these.")
	return nil
}
