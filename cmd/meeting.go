package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	ingestv1 "github.com/otherjamesbrown/penf-cli/api/proto/ingest/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Meeting command flags
var (
	meetingOutputFormat         string
	meetingSeriesFilter         string
	meetingLimit                int32
	meetingParticipant          string
	meetingExcludeParticipant   string
	meetingSince                string
	meetingHasChanges           bool
	meetingProjects             string
	meetingRecapSeries          string
	meetingRecapSourceID        string
	meetingRecapMostRecent      bool
)

// MeetingCommandDeps holds dependencies for meeting commands.
type MeetingCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultMeetingDeps returns default dependencies for production use.
func DefaultMeetingDeps() *MeetingCommandDeps {
	return &MeetingCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewMeetingCommand creates the root meeting command with all subcommands.
func NewMeetingCommand(deps *MeetingCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultMeetingDeps()
	}

	cmd := &cobra.Command{
		Use:   "meeting",
		Short: "Manage meetings",
		Long: `Manage meeting records including listing, filtering, and viewing details.

Meetings can be organized into series for recurring meetings.

Examples:
  # List all meetings
  penf meeting list

  # Filter meetings by series
  penf meeting list --series "TER Weekly"

  # Output as JSON
  penf meeting list -o json`,
		Aliases: []string{"meetings"},
	}

	// Add subcommands
	cmd.AddCommand(newMeetingListCommand(deps))
	cmd.AddCommand(newMeetingSeriesCommand(DefaultMeetingSeriesDeps()))
	cmd.AddCommand(newMeetingSetSeriesCommand(DefaultMeetingSeriesDeps()))
	cmd.AddCommand(newMeetingUnsetSeriesCommand(DefaultMeetingSeriesDeps()))
	cmd.AddCommand(newMeetingUpdateCommand(DefaultMeetingSeriesDeps()))
	cmd.AddCommand(newMeetingRecapCommand(deps))

	return cmd
}

// newMeetingListCommand creates the 'meeting list' subcommand.
func newMeetingListCommand(deps *MeetingCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List meetings",
		Long: `List meetings with optional filtering by series.

Displays meetings in reverse chronological order (most recent first).

Examples:
  # List all meetings
  penf meeting list

  # Filter by series name
  penf meeting list --series "TER Weekly"

  # Limit results
  penf meeting list --limit 10

  # Filter by participant
  penf meeting list --participant me --since yesterday

  # Exclude meetings you were in
  penf meeting list --not-participant me --has-changes

  # Output as JSON
  penf meeting list -o json`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMeetingList(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&meetingSeriesFilter, "series", "s", "", "Filter by series name")
	cmd.Flags().Int32VarP(&meetingLimit, "limit", "l", 50, "Maximum number of results")
	cmd.Flags().StringVarP(&meetingOutputFormat, "output", "o", "", "Output format: text, json, yaml")
	cmd.Flags().StringVar(&meetingParticipant, "participant", "", "Filter by participant email (use 'me' for configured user)")
	cmd.Flags().StringVar(&meetingExcludeParticipant, "not-participant", "", "Exclude meetings with this participant")
	cmd.Flags().StringVar(&meetingSince, "since", "", "Filter meetings since time (e.g., 'yesterday', '2026-01-01', RFC3339)")
	cmd.Flags().BoolVar(&meetingHasChanges, "has-changes", false, "Only include meetings with assertion changes")
	cmd.Flags().StringVar(&meetingProjects, "projects", "", "Filter by project status (e.g., 'active')")

	return cmd
}

// runMeetingList executes the meeting list command.
func runMeetingList(ctx context.Context, deps *MeetingCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format
	outputFormat := cfg.OutputFormat
	if meetingOutputFormat != "" {
		outputFormat = config.OutputFormat(meetingOutputFormat)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", meetingOutputFormat)
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

	// Build request with filters
	req := &ingestv1.ListMeetingsRequest{
		SeriesName: meetingSeriesFilter,
		Limit:      meetingLimit,
		HasChanges: meetingHasChanges,
	}

	// Parse participant filter (handle "me" alias)
	if meetingParticipant != "" {
		// TODO: Add user email to config for "me" alias support
		// For now, just use the value as-is
		req.ParticipantEmail = meetingParticipant
	}

	// Parse exclude participant filter
	if meetingExcludeParticipant != "" {
		// TODO: Add user email to config for "me" alias support
		// For now, just use the value as-is
		req.ExcludeParticipantEmail = meetingExcludeParticipant
	}

	// Parse since filter
	if meetingSince != "" {
		sinceTime, err := parseMeetingTimeFilter(meetingSince)
		if err != nil {
			return fmt.Errorf("invalid since filter: %w", err)
		}
		req.Since = sinceTime
	}

	// Parse project filter (stub for now - would need project ID resolution)
	// TODO: Implement project status -> ID mapping

	// Call ListMeetings
	resp, err := ingestClient.ListMeetings(ctx, req)
	if err != nil {
		return fmt.Errorf("listing meetings: %w", err)
	}

	return outputMeetingList(outputFormat, resp.Meetings)
}

// outputMeetingList formats and outputs the meeting list.
func outputMeetingList(format config.OutputFormat, meetings []*ingestv1.MeetingInfo) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"meetings": meetings,
			"count":    len(meetings),
		})
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(map[string]interface{}{
			"meetings": meetings,
			"count":    len(meetings),
		})
	default:
		return outputMeetingListText(meetings)
	}
}

// outputMeetingListText formats meeting list for terminal display.
func outputMeetingListText(meetings []*ingestv1.MeetingInfo) error {
	if len(meetings) == 0 {
		fmt.Println("No meetings found.")
		return nil
	}

	fmt.Printf("Meetings (%d):\n\n", len(meetings))
	fmt.Println("  ID                 TITLE                                         PLATFORM    DATE")
	fmt.Println("  --                 -----                                         --------    ----")

	for _, m := range meetings {
		titleDisplay := m.Title
		if len(titleDisplay) > 45 {
			titleDisplay = titleDisplay[:42] + "..."
		}

		dateDisplay := m.Date
		if len(dateDisplay) > 10 {
			dateDisplay = dateDisplay[:10]
		}

		platformDisplay := m.Platform
		if len(platformDisplay) > 11 {
			platformDisplay = platformDisplay[:11]
		}

		fmt.Printf("  %-18s %-45s %-11s %s\n",
			m.Id, titleDisplay, platformDisplay, dateDisplay)
	}

	fmt.Println()
	return nil
}

// parseMeetingTimeFilter parses time filters like "yesterday", "last-week", ISO dates, or RFC3339.
func parseMeetingTimeFilter(filter string) (*timestamppb.Timestamp, error) {
	var t time.Time
	var err error

	switch filter {
	case "yesterday":
		t = time.Now().AddDate(0, 0, -1)
	case "last-week":
		t = time.Now().AddDate(0, 0, -7)
	case "last-month":
		t = time.Now().AddDate(0, -1, 0)
	default:
		// Try parsing as ISO date (YYYY-MM-DD)
		t, err = time.Parse("2006-01-02", filter)
		if err != nil {
			// Try parsing as RFC3339
			t, err = time.Parse(time.RFC3339, filter)
			if err != nil {
				return nil, fmt.Errorf("unsupported time format (use 'yesterday', 'YYYY-MM-DD', or RFC3339): %s", filter)
			}
		}
	}

	return timestamppb.New(t), nil
}

// newMeetingRecapCommand creates the 'meeting recap' subcommand.
func newMeetingRecapCommand(deps *MeetingCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recap",
		Short: "Get meeting recap with summary and key points",
		Long: `Get a recap of a meeting including summary, decisions, action items, and risks.

You can either specify a series with --most-recent to get the latest meeting,
or specify a specific meeting with --source-id.

Examples:
  # Get recap of most recent Daily Standup
  penf meeting recap --series "Daily Standup" --most-recent

  # Get recap of specific meeting
  penf meeting recap --source-id cnt-abc123xyz

  # Output as JSON
  penf meeting recap --series "TER Weekly" --most-recent -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMeetingRecap(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&meetingRecapSeries, "series", "", "Filter by series name")
	cmd.Flags().StringVar(&meetingRecapSourceID, "source-id", "", "Get recap for specific meeting by source ID")
	cmd.Flags().BoolVar(&meetingRecapMostRecent, "most-recent", false, "Get most recent meeting in series")
	cmd.Flags().StringVarP(&meetingOutputFormat, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// runMeetingRecap executes the meeting recap command.
func runMeetingRecap(ctx context.Context, deps *MeetingCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Validate flags
	if meetingRecapSourceID == "" && (meetingRecapSeries == "" || !meetingRecapMostRecent) {
		return fmt.Errorf("must provide either --source-id or (--series with --most-recent)")
	}

	// Determine output format
	outputFormat := cfg.OutputFormat
	if meetingOutputFormat != "" {
		outputFormat = config.OutputFormat(meetingOutputFormat)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", meetingOutputFormat)
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

	// Call GetMeetingRecap
	resp, err := ingestClient.GetMeetingRecap(ctx, &ingestv1.GetMeetingRecapRequest{
		SeriesName:  meetingRecapSeries,
		SourceId:    meetingRecapSourceID,
		MostRecent:  meetingRecapMostRecent,
	})
	if err != nil {
		return fmt.Errorf("getting meeting recap: %w", err)
	}

	return outputMeetingRecap(outputFormat, resp.Recap)
}

// outputMeetingRecap formats and outputs the meeting recap.
func outputMeetingRecap(format config.OutputFormat, recap *ingestv1.MeetingRecap) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(recap)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(recap)
	default:
		return outputMeetingRecapText(recap)
	}
}

// outputMeetingRecapText formats meeting recap for terminal display.
func outputMeetingRecapText(recap *ingestv1.MeetingRecap) error {
	if recap == nil || recap.Meeting == nil {
		fmt.Println("No meeting found.")
		return nil
	}

	m := recap.Meeting
	fmt.Printf("Meeting: %s\n", m.Title)
	fmt.Printf("Date:    %s\n", m.Date)
	fmt.Printf("Platform: %s\n", m.Platform)
	fmt.Printf("ID:      %s\n\n", m.Id)

	if recap.Summary != "" {
		fmt.Printf("Summary:\n%s\n\n", recap.Summary)
	}

	if len(recap.KeyDecisions) > 0 {
		fmt.Println("Key Decisions:")
		for _, d := range recap.KeyDecisions {
			fmt.Printf("  - %s\n", d)
		}
		fmt.Println()
	}

	if len(recap.ActionItems) > 0 {
		fmt.Println("Action Items:")
		for _, a := range recap.ActionItems {
			fmt.Printf("  - %s\n", a)
		}
		fmt.Println()
	}

	if len(recap.Risks) > 0 {
		fmt.Println("Risks:")
		for _, r := range recap.Risks {
			fmt.Printf("  - %s\n", r)
		}
		fmt.Println()
	}

	if recap.ParticipantCount > 0 {
		fmt.Printf("Participants: %d\n", recap.ParticipantCount)
	}

	return nil
}
