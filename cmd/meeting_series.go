package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	ingestv1 "github.com/otherjamesbrown/penf-cli/api/proto/ingest/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// MeetingSeriesCommandDeps holds dependencies for series commands.
type MeetingSeriesCommandDeps struct {
	Config       *config.CLIConfig
	GRPCClient   *client.GRPCClient
	OutputFormat config.OutputFormat
	LoadConfig   func() (*config.CLIConfig, error)
	InitClient   func(*config.CLIConfig) (*client.GRPCClient, error)
}

// DefaultMeetingSeriesDeps returns default dependencies for production use.
func DefaultMeetingSeriesDeps() *MeetingSeriesCommandDeps {
	return &MeetingSeriesCommandDeps{
		LoadConfig: config.LoadConfig,
		InitClient: client.ConnectFromConfig,
	}
}

var (
	seriesOutputFormat string
)

// newMeetingSeriesCommand creates the 'meeting series' subcommand.
func newMeetingSeriesCommand(deps *MeetingSeriesCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultMeetingSeriesDeps()
	}

	cmd := &cobra.Command{
		Use:     "series",
		Aliases: []string{"s"},
		Short:   "Manage meeting series",
		Long: `Manage meeting series for grouping related recurring meetings.

Meeting series allow you to organize related meetings together, making it easier
to track discussions over time and identify patterns.

Commands:
  list   - List all series
  create - Create a new series
  show   - Show series details with meetings
  delete - Delete a series (orphans meetings)

Examples:
  # List all series
  penf meeting series list

  # Create a new series
  penf meeting series create "Weekly Standup"

  # Show series details
  penf meeting series show ms-abc123

  # Delete a series
  penf meeting series delete ms-abc123`,
	}

	cmd.AddCommand(newSeriesListCommand(deps))
	cmd.AddCommand(newSeriesCreateCommand(deps))
	cmd.AddCommand(newSeriesShowCommand(deps))
	cmd.AddCommand(newSeriesDeleteCommand(deps))

	return cmd
}

// newSeriesListCommand creates the 'meeting series list' subcommand.
func newSeriesListCommand(deps *MeetingSeriesCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all meeting series",
		Long: `List all meeting series with their meeting counts.

Shows all series organized by name, along with the number of meetings
associated with each series.

Examples:
  # List all series
  penf meeting series list

  # Output as JSON
  penf meeting series list --output=json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSeriesList(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&seriesOutputFormat, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newSeriesCreateCommand creates the 'meeting series create' subcommand.
func newSeriesCreateCommand(deps *MeetingSeriesCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new meeting series",
		Long: `Create a new meeting series with the specified name.

The series name must be unique. After creation, you can assign meetings
to this series using the meeting assign command.

Examples:
  # Create a simple series
  penf meeting series create "Weekly Standup"

  # Create with description
  penf meeting series create "Sprint Planning"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSeriesCreate(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVarP(&seriesOutputFormat, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newSeriesShowCommand creates the 'meeting series show' subcommand.
func newSeriesShowCommand(deps *MeetingSeriesCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show series details with meetings",
		Long: `Show detailed information about a series including all meetings.

Displays the series metadata and lists all meetings that are part of
this series, ordered by date.

Examples:
  # Show series details
  penf meeting series show ms-abc123

  # Output as JSON
  penf meeting series show ms-abc123 --output=json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSeriesShow(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVarP(&seriesOutputFormat, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newSeriesDeleteCommand creates the 'meeting series delete' subcommand.
func newSeriesDeleteCommand(deps *MeetingSeriesCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a series and orphan its meetings",
		Long: `Delete a meeting series and remove the series association from all meetings.

This operation:
  - Deletes the series record
  - Sets series_id to NULL for all associated meetings
  - Preserves the meeting records (they are not deleted)

Examples:
  # Delete a series
  penf meeting series delete ms-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSeriesDelete(cmd.Context(), deps, args[0])
		},
	}

	return cmd
}

// runSeriesList executes the series list command.
func runSeriesList(ctx context.Context, deps *MeetingSeriesCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format
	outputFormat := cfg.OutputFormat
	if seriesOutputFormat != "" {
		outputFormat = config.OutputFormat(seriesOutputFormat)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", seriesOutputFormat)
		}
	}

	// Connect to gateway via gRPC
	conn, err := connectToGateway(cfg)
	if err != nil {
		return fmt.Errorf("connecting to gateway: %w", err)
	}
	defer conn.Close()

	// Get ingest service client
	ingestClient := ingestv1.NewIngestServiceClient(conn)

	// Call ListSeries
	resp, err := ingestClient.ListSeries(ctx, &ingestv1.ListSeriesRequest{})
	if err != nil {
		return fmt.Errorf("listing series: %w", err)
	}

	return outputSeriesList(outputFormat, resp.Series)
}

// runSeriesCreate executes the series create command.
func runSeriesCreate(ctx context.Context, deps *MeetingSeriesCommandDeps, name string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format
	outputFormat := cfg.OutputFormat
	if seriesOutputFormat != "" {
		outputFormat = config.OutputFormat(seriesOutputFormat)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", seriesOutputFormat)
		}
	}

	// Connect to gateway via gRPC
	conn, err := connectToGateway(cfg)
	if err != nil {
		return fmt.Errorf("connecting to gateway: %w", err)
	}
	defer conn.Close()

	// Get ingest service client
	ingestClient := ingestv1.NewIngestServiceClient(conn)

	// Call CreateSeries
	resp, err := ingestClient.CreateSeries(ctx, &ingestv1.CreateSeriesRequest{
		Name: name,
	})
	if err != nil {
		return fmt.Errorf("creating series: %w", err)
	}

	return outputSeriesDetails(outputFormat, resp.Series, nil, resp.Created)
}

// runSeriesShow executes the series show command.
func runSeriesShow(ctx context.Context, deps *MeetingSeriesCommandDeps, id string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format
	outputFormat := cfg.OutputFormat
	if seriesOutputFormat != "" {
		outputFormat = config.OutputFormat(seriesOutputFormat)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", seriesOutputFormat)
		}
	}

	// Connect to gateway via gRPC
	conn, err := connectToGateway(cfg)
	if err != nil {
		return fmt.Errorf("connecting to gateway: %w", err)
	}
	defer conn.Close()

	// Get ingest service client
	ingestClient := ingestv1.NewIngestServiceClient(conn)

	// Call GetSeries
	resp, err := ingestClient.GetSeries(ctx, &ingestv1.GetSeriesRequest{
		Id: id,
	})
	if err != nil {
		return fmt.Errorf("getting series: %w", err)
	}

	return outputSeriesDetails(outputFormat, resp.Series, resp.Meetings, false)
}

// runSeriesDelete executes the series delete command.
func runSeriesDelete(ctx context.Context, deps *MeetingSeriesCommandDeps, id string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Connect to gateway via gRPC
	conn, err := connectToGateway(cfg)
	if err != nil {
		return fmt.Errorf("connecting to gateway: %w", err)
	}
	defer conn.Close()

	// Get ingest service client
	ingestClient := ingestv1.NewIngestServiceClient(conn)

	// Call DeleteSeries
	resp, err := ingestClient.DeleteSeries(ctx, &ingestv1.DeleteSeriesRequest{
		Id: id,
	})
	if err != nil {
		return fmt.Errorf("deleting series: %w", err)
	}

	if !resp.Deleted {
		fmt.Printf("Series not found: %s\n", id)
		return nil
	}

	fmt.Printf("Series deleted: %s\n", id)
	if resp.OrphanedMeetings > 0 {
		fmt.Printf("Orphaned meetings: %d\n", resp.OrphanedMeetings)
	}

	return nil
}

// outputSeriesList formats and outputs the series list.
func outputSeriesList(format config.OutputFormat, series []*ingestv1.MeetingSeries) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"series": series,
			"count":  len(series),
		})
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(map[string]interface{}{
			"series": series,
			"count":  len(series),
		})
	default:
		return outputSeriesListText(series)
	}
}

// outputSeriesListText formats series list for terminal display.
func outputSeriesListText(series []*ingestv1.MeetingSeries) error {
	if len(series) == 0 {
		fmt.Println("No series found.")
		return nil
	}

	fmt.Printf("Meeting Series (%d):\n\n", len(series))
	fmt.Println("  ID                              NAME                                         CREATED")
	fmt.Println("  --                              ----                                         -------")

	for _, s := range series {
		nameDisplay := s.Name
		if len(nameDisplay) > 44 {
			nameDisplay = nameDisplay[:41] + "..."
		}

		fmt.Printf("  %-30s  %-44s %s\n",
			s.Id, nameDisplay, s.CreatedAt)
	}

	fmt.Println()
	return nil
}

// outputSeriesDetails formats and outputs series details.
func outputSeriesDetails(format config.OutputFormat, series *ingestv1.MeetingSeries, meetings []*ingestv1.MeetingInfo, created bool) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		output := map[string]interface{}{
			"series": series,
		}
		if meetings != nil {
			output["meetings"] = meetings
			output["meeting_count"] = len(meetings)
		}
		if created {
			output["created"] = true
		}
		return enc.Encode(output)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		output := map[string]interface{}{
			"series": series,
		}
		if meetings != nil {
			output["meetings"] = meetings
			output["meeting_count"] = len(meetings)
		}
		if created {
			output["created"] = true
		}
		return enc.Encode(output)
	default:
		return outputSeriesDetailsText(series, meetings, created)
	}
}

// outputSeriesDetailsText formats series details for terminal display.
func outputSeriesDetailsText(series *ingestv1.MeetingSeries, meetings []*ingestv1.MeetingInfo, created bool) error {
	if created {
		fmt.Printf("\033[32mSeries created successfully\033[0m\n\n")
	}

	fmt.Printf("\033[1mSeries: %s\033[0m\n", series.Name)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("  ID:          %s\n", series.Id)
	fmt.Printf("  Name:        %s\n", series.Name)
	if series.Description != "" {
		fmt.Printf("  Description: %s\n", series.Description)
	}
	if series.ProjectId != "" {
		fmt.Printf("  Project:     %s\n", series.ProjectId)
	}
	fmt.Printf("  Created:     %s\n", series.CreatedAt)

	if meetings != nil {
		fmt.Printf("\n  Meetings: %d\n", len(meetings))
		if len(meetings) > 0 {
			fmt.Println()
			for i, m := range meetings {
				fmt.Printf("    %d. [%s] %s (%s)\n", i+1, m.Id, m.Title, m.Date)
				if m.Platform != "" {
					fmt.Printf("       Platform: %s\n", m.Platform)
				}
			}
		}
	}

	fmt.Println()
	return nil
}
