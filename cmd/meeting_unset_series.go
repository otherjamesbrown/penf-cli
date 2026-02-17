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
	unsetSeriesOutputFormat string
)

// newMeetingUnsetSeriesCommand creates the 'meeting unset-series' command.
func newMeetingUnsetSeriesCommand(deps *MeetingSeriesCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultMeetingSeriesDeps()
	}

	cmd := &cobra.Command{
		Use:   "unset-series <meeting-id>",
		Short: "Remove a meeting from its series",
		Long: `Remove a meeting from its series, if it belongs to one.

This command removes the series association from a meeting. The meeting record
is preserved, and the series itself is not deleted (even if this was the last
meeting in the series).

Examples:
  # Remove meeting from its series
  penf meeting unset-series mt-abc123

  # Output as JSON
  penf meeting unset-series mt-abc123 --output=json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMeetingUnsetSeries(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVarP(&unsetSeriesOutputFormat, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// runMeetingUnsetSeries executes the unset-series command.
func runMeetingUnsetSeries(ctx context.Context, deps *MeetingSeriesCommandDeps, meetingID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format
	outputFormat := cfg.OutputFormat
	if unsetSeriesOutputFormat != "" {
		outputFormat = config.OutputFormat(unsetSeriesOutputFormat)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", unsetSeriesOutputFormat)
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

	// Call UnsetMeetingSeries
	resp, err := ingestClient.UnsetMeetingSeries(ctx, &ingestv1.UnsetMeetingSeriesRequest{
		MeetingId: meetingID,
	})
	if err != nil {
		return fmt.Errorf("unsetting meeting series: %w", err)
	}

	return outputUnsetSeriesResult(outputFormat, meetingID, resp)
}

// outputUnsetSeriesResult formats and outputs the unset-series result.
func outputUnsetSeriesResult(format config.OutputFormat, meetingID string, resp *ingestv1.UnsetMeetingSeriesResponse) error {
	switch format {
	case config.OutputFormatJSON:
		return outputUnsetSeriesResultJSON(meetingID, resp)
	case config.OutputFormatYAML:
		return outputUnsetSeriesResultYAML(meetingID, resp)
	default:
		return outputUnsetSeriesResultText(meetingID, resp)
	}
}

// outputUnsetSeriesResultText formats result for terminal display.
func outputUnsetSeriesResultText(meetingID string, resp *ingestv1.UnsetMeetingSeriesResponse) error {
	if !resp.Updated {
		fmt.Printf("Meeting not found or already has no series: %s\n", meetingID)
		return nil
	}

	fmt.Printf("\033[32mMeeting removed from series\033[0m\n\n")
	fmt.Printf("  Meeting: %s\n", meetingID)
	fmt.Println()
	return nil
}

// outputUnsetSeriesResultJSON formats result as JSON.
func outputUnsetSeriesResultJSON(meetingID string, resp *ingestv1.UnsetMeetingSeriesResponse) error {
	result := map[string]interface{}{
		"meeting_id": meetingID,
		"updated":    resp.Updated,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// outputUnsetSeriesResultYAML formats result as YAML.
func outputUnsetSeriesResultYAML(meetingID string, resp *ingestv1.UnsetMeetingSeriesResponse) error {
	result := map[string]interface{}{
		"meeting_id": meetingID,
		"updated":    resp.Updated,
	}

	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(result)
}
