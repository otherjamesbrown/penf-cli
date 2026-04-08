// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	smv1 "github.com/otherjamesbrown/penf-cli/api/proto/source_mappings/v1"
	projectv1 "github.com/otherjamesbrown/penf-cli/api/proto/project/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

var (
	sourceTenant     string
	sourceOutput     string
	sourceProject    string
	sourceType       string
	sourceMatchType  string
	sourceConfidence float64
	sourceNotes      string
)

// SourceCommandDeps holds dependencies for source commands.
type SourceCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultSourceDeps returns production dependencies.
func DefaultSourceDeps() *SourceCommandDeps {
	return &SourceCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewSourceCommand creates the root source command with subcommands.
func NewSourceCommand(deps *SourceCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultSourceDeps()
	}

	cmd := &cobra.Command{
		Use:   "source",
		Short: "Manage source-to-project mappings for attribution",
		Long: `Manage mappings between content sources and projects.

Source mappings tell the attribution pipeline which project to assign content to
based on where it came from — a Teams channel, Jira project, meeting series, etc.

This is the highest-confidence attribution signal: content from a tagged source is
automatically attributed to the mapped project without LLM inference.

Source Types:
  channel          Slack or Teams channel name
  teams_channel    Microsoft Teams channel
  slack_channel    Slack channel
  meeting_series   Meeting series (matched by subject pattern)
  email_list       Email distribution list address
  jira_project     Jira project key (e.g. MTC, PROJ)

Match Types:
  exact      Source identifier must match exactly (default)
  prefix     Source identifier must start with the value
  contains   Source identifier must contain the value
  regex      Source identifier matched as a regular expression

Examples:
  # Tag a Teams channel to a project
  penf source tag "#mtc-general" --project "ManagedTrafficController" --type teams_channel

  # Tag a Jira project key
  penf source tag "MTC" --project "ManagedTrafficController" --type jira_project

  # List all source mappings
  penf source list

  # List mappings for a specific project
  penf source list --project "ManagedTrafficController"

  # Remove a mapping by ID
  penf source remove 42`,
	}

	cmd.PersistentFlags().StringVar(&sourceTenant, "tenant", "", "Tenant ID (overrides config)")
	cmd.PersistentFlags().StringVarP(&sourceOutput, "output", "o", "", "Output format: text, json")

	cmd.AddCommand(newSourceTagCommand(deps))
	cmd.AddCommand(newSourceListCommand(deps))
	cmd.AddCommand(newSourceRemoveCommand(deps))
	cmd.AddCommand(newSourceGraphCommand(deps))

	return cmd
}

// newSourceTagCommand creates the `penf source tag` command.
func newSourceTagCommand(deps *SourceCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag <source-identifier>",
		Short: "Map a source to a project",
		Long: `Create a mapping from a source identifier to a project.

Content from this source will be automatically attributed to the specified project
by the attribution pipeline, without requiring LLM inference.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSourceTag(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVar(&sourceProject, "project", "", "Project name or ID to map to (required)")
	cmd.Flags().StringVar(&sourceType, "type", "channel", "Source type: channel, teams_channel, slack_channel, meeting_series, email_list, jira_project")
	cmd.Flags().StringVar(&sourceMatchType, "match", "exact", "Match type: exact, prefix, contains, regex")
	cmd.Flags().Float64Var(&sourceConfidence, "confidence", 0, "Attribution confidence 0.0–1.0 (default 0.95)")
	cmd.Flags().StringVar(&sourceNotes, "notes", "", "Optional notes about this mapping")

	_ = cmd.MarkFlagRequired("project")

	return cmd
}

// newSourceListCommand creates the `penf source list` command.
func newSourceListCommand(deps *SourceCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List source-to-project mappings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSourceList(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&sourceProject, "project", "", "Filter by project name or ID")
	cmd.Flags().StringVar(&sourceType, "type", "", "Filter by source type")

	return cmd
}

// newSourceRemoveCommand creates the `penf source remove` command.
func newSourceRemoveCommand(deps *SourceCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <mapping-id>",
		Short: "Remove a source-to-project mapping",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSourceRemove(cmd.Context(), deps, args[0])
		},
	}
	return cmd
}

func getTenantIDForSource(deps *SourceCommandDeps) string {
	if sourceTenant != "" {
		return sourceTenant
	}
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant
	}
	if deps.Config != nil && deps.Config.EffectiveTenantID() != "" {
		return deps.Config.EffectiveTenantID()
	}
	return "00000001-0000-0000-0000-000000000001"
}

// ==================== Command Execution ====================

func runSourceTag(ctx context.Context, deps *SourceCommandDeps, identifier string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	projectID, err := resolveProjectID(ctx, conn, deps, sourceProject)
	if err != nil {
		return fmt.Errorf("resolving project %q: %w", sourceProject, err)
	}

	req := &smv1.CreateSourceMappingRequest{
		TenantId:         getTenantIDForSource(deps),
		ProjectId:        projectID,
		SourceType:       sourceType,
		SourceIdentifier: identifier,
		MatchType:        sourceMatchType,
	}
	if sourceConfidence > 0 {
		c := float32(sourceConfidence)
		req.Confidence = &c
	}
	if sourceNotes != "" {
		req.Notes = &sourceNotes
	}

	smClient := smv1.NewSourceMappingServiceClient(conn)
	resp, err := smClient.CreateSourceMapping(ctx, req)
	if err != nil {
		return fmt.Errorf("creating source mapping: %w", err)
	}

	m := resp.Mapping
	if sourceOutput == "json" {
		return json.NewEncoder(os.Stdout).Encode(mappingToMap(m))
	}

	fmt.Printf("Mapped source %q → project %d (type=%s match=%s confidence=%.2f)\n",
		m.SourceIdentifier, m.ProjectId, m.SourceType, m.MatchType, m.Confidence)
	fmt.Printf("ID: %d\n", m.Id)
	return nil
}

func runSourceList(ctx context.Context, deps *SourceCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	tenantID := getTenantIDForSource(deps)
	req := &smv1.ListSourceMappingsRequest{TenantId: tenantID}

	if sourceProject != "" {
		projectID, err := resolveProjectID(ctx, conn, deps, sourceProject)
		if err != nil {
			return fmt.Errorf("resolving project %q: %w", sourceProject, err)
		}
		req.ProjectId = &projectID
	}
	if sourceType != "" {
		req.SourceType = &sourceType
	}

	smClient := smv1.NewSourceMappingServiceClient(conn)
	resp, err := smClient.ListSourceMappings(ctx, req)
	if err != nil {
		return fmt.Errorf("listing source mappings: %w", err)
	}

	if sourceOutput == "json" {
		out := make([]map[string]interface{}, 0, len(resp.Mappings))
		for _, m := range resp.Mappings {
			out = append(out, mappingToMap(m))
		}
		return json.NewEncoder(os.Stdout).Encode(out)
	}

	if len(resp.Mappings) == 0 {
		fmt.Println("No source mappings found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSOURCE IDENTIFIER\tTYPE\tMATCH\tCONFIDENCE\tPROJECT\tENABLED")
	for _, m := range resp.Mappings {
		enabled := "yes"
		if !m.Enabled {
			enabled = "no"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%.2f\t%d\t%s\n",
			m.Id, m.SourceIdentifier, m.SourceType, m.MatchType, m.Confidence, m.ProjectId, enabled)
	}
	return w.Flush()
}

func runSourceRemove(ctx context.Context, deps *SourceCommandDeps, idStr string) error {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid mapping ID %q: must be an integer", idStr)
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	smClient := smv1.NewSourceMappingServiceClient(conn)
	resp, err := smClient.DeleteSourceMapping(ctx, &smv1.DeleteSourceMappingRequest{
		TenantId: getTenantIDForSource(deps),
		Id:       id,
	})
	if err != nil {
		return fmt.Errorf("deleting source mapping %d: %w", id, err)
	}

	if !resp.Success {
		return fmt.Errorf("mapping %d not found or could not be deleted", id)
	}

	fmt.Printf("Removed source mapping %d\n", id)
	return nil
}

// ==================== Helpers ====================

// resolveProjectID looks up a project by name or parses it as a numeric ID.
// conn must already be open; the caller owns its lifecycle.
func resolveProjectID(ctx context.Context, conn *grpc.ClientConn, deps *SourceCommandDeps, nameOrID string) (int64, error) {
	if id, err := strconv.ParseInt(nameOrID, 10, 64); err == nil {
		return id, nil
	}

	pClient := projectv1.NewProjectServiceClient(conn)
	resp, err := pClient.ListProjects(ctx, &projectv1.ListProjectsRequest{
		Filter: &projectv1.ProjectFilter{
			TenantId:   getTenantIDForSource(deps),
			NameSearch: nameOrID,
		},
	})
	if err != nil {
		return 0, fmt.Errorf("looking up project: %w", err)
	}

	for _, p := range resp.Projects {
		if p.Name == nameOrID {
			return p.Id, nil
		}
	}

	return 0, fmt.Errorf("project %q not found", nameOrID)
}

// mappingToMap converts a SourceMapping proto to a JSON-serializable map.
func mappingToMap(m *smv1.SourceMapping) map[string]interface{} {
	out := map[string]interface{}{
		"id":                m.Id,
		"project_id":        m.ProjectId,
		"source_type":       m.SourceType,
		"source_identifier": m.SourceIdentifier,
		"match_type":        m.MatchType,
		"confidence":        m.Confidence,
		"notes":             m.Notes,
		"enabled":           m.Enabled,
	}
	if m.CreatedAt != nil {
		out["created_at"] = m.CreatedAt.AsTime()
	}
	if m.UpdatedAt != nil {
		out["updated_at"] = m.UpdatedAt.AsTime()
	}
	return out
}
