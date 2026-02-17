package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	ingestv1 "github.com/otherjamesbrown/penf-cli/api/proto/ingest/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

var (
	updateMeetingTitle       string
	updateMeetingDate        string
	updateMeetingSeries      string
	updateMeetingDescription string
	updateMeetingOutput      string
)

// newMeetingUpdateCommand creates the 'meeting update' command.
func newMeetingUpdateCommand(deps *MeetingSeriesCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultMeetingSeriesDeps()
	}

	cmd := &cobra.Command{
		Use:   "update <meeting-id>",
		Short: "Update meeting metadata",
		Long: `Update metadata for a meeting.

Update title, date, series, or description for an existing meeting. At least one
flag must be provided. If a series is specified and doesn't exist, it will be
automatically created.

Examples:
  # Update title only
  penf meeting update mt-abc123 --title "New Meeting Title"

  # Update date (YYYY-MM-DD or RFC3339)
  penf meeting update mt-abc123 --date "2026-02-15"

  # Assign to series (creates if not exists)
  penf meeting update mt-abc123 --series "Weekly Standup"

  # Update description
  penf meeting update mt-abc123 --description "Discussed project roadmap"

  # Update multiple fields
  penf meeting update mt-abc123 --title "Q1 Planning" --series "Quarterly" --date "2026-03-01"

  # Output as JSON
  penf meeting update mt-abc123 --title "New Title" --output=json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMeetingUpdate(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVar(&updateMeetingTitle, "title", "", "New title for the meeting")
	cmd.Flags().StringVar(&updateMeetingDate, "date", "", "New date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&updateMeetingSeries, "series", "", "Series name (creates if not exists)")
	cmd.Flags().StringVar(&updateMeetingDescription, "description", "", "Description/notes")
	cmd.Flags().StringVarP(&updateMeetingOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// runMeetingUpdate executes the update command.
func runMeetingUpdate(ctx context.Context, deps *MeetingSeriesCommandDeps, meetingID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Require at least one flag
	if updateMeetingTitle == "" && updateMeetingDate == "" && updateMeetingSeries == "" && updateMeetingDescription == "" {
		return fmt.Errorf("at least one flag required: --title, --date, --series, or --description")
	}

	// Determine output format
	outputFormat := cfg.OutputFormat
	if updateMeetingOutput != "" {
		outputFormat = config.OutputFormat(updateMeetingOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", updateMeetingOutput)
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

	// Build request
	req := &ingestv1.UpdateMeetingRequest{
		MeetingId: meetingID,
	}

	if updateMeetingTitle != "" {
		req.Title = &updateMeetingTitle
	}

	if updateMeetingDate != "" {
		req.Date = &updateMeetingDate
	}

	if updateMeetingSeries != "" {
		req.SeriesName = &updateMeetingSeries
	}

	if updateMeetingDescription != "" {
		req.Description = &updateMeetingDescription
	}

	// Call UpdateMeeting
	resp, err := ingestClient.UpdateMeeting(ctx, req)
	if err != nil {
		return fmt.Errorf("updating meeting: %w", err)
	}

	return outputUpdateMeetingResult(outputFormat, resp)
}

// outputUpdateMeetingResult formats and outputs the update result.
func outputUpdateMeetingResult(format config.OutputFormat, resp *ingestv1.UpdateMeetingResponse) error {
	switch format {
	case config.OutputFormatJSON:
		return outputUpdateMeetingResultJSON(resp)
	case config.OutputFormatYAML:
		return outputUpdateMeetingResultYAML(resp)
	default:
		return outputUpdateMeetingResultText(resp)
	}
}

// outputUpdateMeetingResultText formats result for terminal display.
func outputUpdateMeetingResultText(resp *ingestv1.UpdateMeetingResponse) error {
	fmt.Printf("\033[32mMeeting updated\033[0m\n\n")

	if resp.Meeting != nil {
		fmt.Printf("  ID:       %s\n", resp.Meeting.Id)
		fmt.Printf("  Title:    %s\n", resp.Meeting.Title)
		fmt.Printf("  Date:     %s\n", resp.Meeting.Date)
		fmt.Printf("  Platform: %s\n", resp.Meeting.Platform)

		if resp.SeriesId != "" {
			fmt.Printf("  Series:   %s\n", resp.SeriesId)
		}
	}

	if resp.SeriesCreated {
		fmt.Printf("\n  \033[33mNote: Series was auto-created\033[0m\n")
	}

	fmt.Println()
	return nil
}

// outputUpdateMeetingResultJSON formats result as JSON.
func outputUpdateMeetingResultJSON(resp *ingestv1.UpdateMeetingResponse) error {
	result := map[string]interface{}{
		"meeting":        resp.Meeting,
		"series_id":      resp.SeriesId,
		"series_created": resp.SeriesCreated,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// outputUpdateMeetingResultYAML formats result as YAML.
func outputUpdateMeetingResultYAML(resp *ingestv1.UpdateMeetingResponse) error {
	result := map[string]interface{}{
		"meeting":        resp.Meeting,
		"series_id":      resp.SeriesId,
		"series_created": resp.SeriesCreated,
	}

	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(result)
}
