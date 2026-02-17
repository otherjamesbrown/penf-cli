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
	setSeriesOutputFormat string
)

// newMeetingSetSeriesCommand creates the 'meeting set-series' command.
func newMeetingSetSeriesCommand(deps *MeetingSeriesCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultMeetingSeriesDeps()
	}

	cmd := &cobra.Command{
		Use:   "set-series <meeting-id> <series-name>",
		Short: "Assign a meeting to a series",
		Long: `Assign a meeting to a series, creating the series if it doesn't exist.

This command links a meeting to a named series. If the series doesn't exist,
it will be automatically created. Meetings can only belong to one series at a time.

Examples:
  # Assign meeting to a series
  penf meeting set-series mt-abc123 "Weekly Standup"

  # The series will be auto-created if it doesn't exist
  penf meeting set-series mt-xyz789 "Sprint Planning"

  # Output as JSON
  penf meeting set-series mt-abc123 "Weekly Standup" --output=json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMeetingSetSeries(cmd.Context(), deps, args[0], args[1])
		},
	}

	cmd.Flags().StringVarP(&setSeriesOutputFormat, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// runMeetingSetSeries executes the set-series command.
func runMeetingSetSeries(ctx context.Context, deps *MeetingSeriesCommandDeps, meetingID, seriesName string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format
	outputFormat := cfg.OutputFormat
	if setSeriesOutputFormat != "" {
		outputFormat = config.OutputFormat(setSeriesOutputFormat)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", setSeriesOutputFormat)
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

	// Call SetMeetingSeries
	resp, err := ingestClient.SetMeetingSeries(ctx, &ingestv1.SetMeetingSeriesRequest{
		MeetingId:  meetingID,
		SeriesName: seriesName,
	})
	if err != nil {
		return fmt.Errorf("setting meeting series: %w", err)
	}

	return outputSetSeriesResult(outputFormat, meetingID, seriesName, resp)
}

// outputSetSeriesResult formats and outputs the set-series result.
func outputSetSeriesResult(format config.OutputFormat, meetingID, seriesName string, resp *ingestv1.SetMeetingSeriesResponse) error {
	switch format {
	case config.OutputFormatJSON:
		return outputSetSeriesResultJSON(meetingID, seriesName, resp)
	case config.OutputFormatYAML:
		return outputSetSeriesResultYAML(meetingID, seriesName, resp)
	default:
		return outputSetSeriesResultText(meetingID, seriesName, resp)
	}
}

// outputSetSeriesResultText formats result for terminal display.
func outputSetSeriesResultText(meetingID, seriesName string, resp *ingestv1.SetMeetingSeriesResponse) error {
	if !resp.Updated {
		fmt.Printf("Meeting not found: %s\n", meetingID)
		return nil
	}

	fmt.Printf("\033[32mMeeting assigned to series\033[0m\n\n")
	fmt.Printf("  Meeting:  %s\n", meetingID)
	fmt.Printf("  Series:   %s\n", seriesName)
	fmt.Printf("  SeriesID: %s\n", resp.SeriesId)

	if resp.SeriesCreated {
		fmt.Printf("\n  \033[33mNote: Series was auto-created\033[0m\n")
	}

	fmt.Println()
	return nil
}

// outputSetSeriesResultJSON formats result as JSON.
func outputSetSeriesResultJSON(meetingID, seriesName string, resp *ingestv1.SetMeetingSeriesResponse) error {
	result := map[string]interface{}{
		"meeting_id":     meetingID,
		"series_name":    seriesName,
		"series_id":      resp.SeriesId,
		"updated":        resp.Updated,
		"series_created": resp.SeriesCreated,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// outputSetSeriesResultYAML formats result as YAML.
func outputSetSeriesResultYAML(meetingID, seriesName string, resp *ingestv1.SetMeetingSeriesResponse) error {
	result := map[string]interface{}{
		"meeting_id":     meetingID,
		"series_name":    seriesName,
		"series_id":      resp.SeriesId,
		"updated":        resp.Updated,
		"series_created": resp.SeriesCreated,
	}

	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(result)
}
