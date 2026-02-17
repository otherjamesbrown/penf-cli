package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"

	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

func newPipelineErrorsCmd(deps *PipelineCommandDeps) *cobra.Command {
	var (
		since      string
		code       string
		sourceID   string
		retryable  *bool
		outputFmt  string
		limit      int
		groupBy    string
	)

	cmd := &cobra.Command{
		Use:   "errors",
		Short: "View and filter pipeline errors",
		Long: `View pipeline errors with filtering by time, code, source, and retryability.

Displays structured error codes with retryable flags, suggested actions, and error counts.

Examples:
  # Show all errors from the last 24 hours
  penf pipeline errors --since 24h

  # Filter by specific error code
  penf pipeline errors --code TIMEOUT --since 24h

  # Filter by source
  penf pipeline errors --source em-abc123

  # Show only retryable errors
  penf pipeline errors --retryable --since 24h

  # Group errors by code
  penf pipeline errors --since 24h --group-by code

  # Output as JSON
  penf pipeline errors --since 24h -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineErrors(cmd.Context(), deps, since, code, sourceID, retryable, outputFmt, limit, groupBy)
		},
	}

	cmd.Flags().StringVar(&since, "since", "24h", "Show errors since this time (duration like '24h', '2d', or ISO timestamp)")
	cmd.Flags().StringVar(&code, "code", "", "Filter by error code (e.g., TIMEOUT, RATE_LIMIT)")
	cmd.Flags().StringVar(&sourceID, "source", "", "Filter by source ID")
	retryable = cmd.Flags().Bool("retryable", false, "Show only retryable errors")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "text", "Output format: text, json")
	cmd.Flags().IntVarP(&limit, "limit", "l", 100, "Maximum number of errors to show")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "Group results by: code, stage")

	return cmd
}

func runPipelineErrors(ctx context.Context, deps *PipelineCommandDeps, since string, code string, sourceID string, retryable *bool, outputFmt string, limit int, groupBy string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	// Build request
	req := &pipelinev1.GetPipelineErrorsRequest{
		Limit:     int32(limit),
		ErrorCode: code,
		SourceId:  sourceID,
	}

	// Parse since time
	if since != "" {
		sinceTime, err := parseTimeFilter(since)
		if err != nil {
			return fmt.Errorf("parsing --since value: %w", err)
		}
		req.Since = timestamppb.New(sinceTime)
	}

	// Fetch errors
	resp, err := client.GetPipelineErrors(ctx, req)
	if err != nil {
		return fmt.Errorf("getting pipeline errors: %w", err)
	}

	// Filter by retryable if specified
	var filteredErrors []*pipelinev1.PipelineErrorEvent
	if retryable != nil && *retryable {
		for _, e := range resp.Errors {
			if e.Retryable {
				filteredErrors = append(filteredErrors, e)
			}
		}
	} else {
		filteredErrors = resp.Errors
	}

	// Output results
	if outputFmt == "json" {
		return outputErrorsJSON(filteredErrors, resp.TotalCount)
	}

	if groupBy != "" {
		return outputErrorsGrouped(filteredErrors, groupBy)
	}

	return outputErrorsText(filteredErrors, resp.TotalCount, since)
}

func outputErrorsJSON(errors []*pipelinev1.PipelineErrorEvent, totalCount int64) error {
	output := map[string]interface{}{
		"errors":      errors,
		"count":       len(errors),
		"total_count": totalCount,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputErrorsText(errors []*pipelinev1.PipelineErrorEvent, totalCount int64, since string) error {
	if len(errors) == 0 {
		fmt.Printf("No pipeline errors found (since %s)\n", since)
		return nil
	}

	fmt.Printf("Pipeline Errors (since %s):\n", since)
	fmt.Printf("Total: %d errors\n\n", totalCount)

	fmt.Println("TIME      CODE                   STAGE         RETRYABLE  COUNT  LATEST ERROR")
	fmt.Println("----      ----                   -----         ---------  -----  ------------")

	for _, e := range errors {
		timestamp := "-"
		if e.OccurredAt != nil {
			timestamp = e.OccurredAt.AsTime().Format("15:04:05")
		}

		retryableStr := "no"
		retryableColor := "\033[33m" // Yellow
		if e.Retryable {
			retryableStr = "yes"
			retryableColor = "\033[32m" // Green
		}

		// Truncate message
		message := e.Message
		if len(message) > 60 {
			message = message[:57] + "..."
		}

		fmt.Printf("%s  %-21s  %-12s  %s%-9s\033[0m  %-5s  %s\n",
			timestamp,
			truncate(e.Code, 21),
			truncate(e.Stage, 12),
			retryableColor,
			retryableStr,
			"1", // Count placeholder - we'd need to aggregate to get real count
			message,
		)
	}

	fmt.Println()
	fmt.Println("Use 'penf content trace <source-id>' for detailed error timeline")

	return nil
}

func outputErrorsGrouped(errors []*pipelinev1.PipelineErrorEvent, groupBy string) error {
	if len(errors) == 0 {
		fmt.Println("No errors to group")
		return nil
	}

	// Group errors
	groups := make(map[string][]*pipelinev1.PipelineErrorEvent)
	for _, e := range errors {
		var key string
		switch groupBy {
		case "code":
			key = e.Code
		case "stage":
			key = e.Stage
		default:
			return fmt.Errorf("invalid group-by value: %s (must be: code, stage)", groupBy)
		}

		groups[key] = append(groups[key], e)
	}

	fmt.Printf("Pipeline Errors (grouped by %s):\n\n", groupBy)

	// Output groups
	for key, groupErrors := range groups {
		count := len(groupErrors)
		retryable := groupErrors[0].Retryable
		suggestedAction := groupErrors[0].SuggestedAction

		retryableStr := "no"
		if retryable {
			retryableStr = "yes"
		}

		fmt.Printf("\033[1m%s\033[0m (count: %d, retryable: %s)\n", key, count, retryableStr)
		if suggestedAction != "" {
			fmt.Printf("  Suggested action: %s\n", suggestedAction)
		}

		// Show sample errors (first 3)
		sampleSize := 3
		if len(groupErrors) < sampleSize {
			sampleSize = len(groupErrors)
		}

		for i := 0; i < sampleSize; i++ {
			e := groupErrors[i]
			timestamp := "-"
			if e.OccurredAt != nil {
				timestamp = e.OccurredAt.AsTime().Format("2006-01-02 15:04:05")
			}

			message := e.Message
			if len(message) > 80 {
				message = message[:77] + "..."
			}

			fmt.Printf("  [%s] %s: %s\n", timestamp, e.Stage, message)
		}

		if len(groupErrors) > sampleSize {
			fmt.Printf("  ... and %d more\n", len(groupErrors)-sampleSize)
		}

		fmt.Println()
	}

	return nil
}
