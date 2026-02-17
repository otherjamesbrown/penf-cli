// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	contentv1 "github.com/otherjamesbrown/penf-cli/api/proto/content/v1"
	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
	"github.com/otherjamesbrown/penf-cli/contextpalace"
)

// PipelineCommandDeps holds the dependencies for pipeline commands.
type PipelineCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultPipelineDeps returns the default dependencies for production use.
func DefaultPipelineDeps() *PipelineCommandDeps {
	return &PipelineCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewPipelineCommand creates the pipeline command.
func NewPipelineCommand(deps interface{}) *cobra.Command {
	// Accept either PipelineCommandDeps or nil
	var pipelineDeps *PipelineCommandDeps
	if d, ok := deps.(*PipelineCommandDeps); ok && d != nil {
		pipelineDeps = d
	} else {
		pipelineDeps = DefaultPipelineDeps()
	}

	cmd := &cobra.Command{
		Use:     "pipeline",
		Aliases: []string{"pipe", "pl"},
		Short:   "View pipeline status and job tracking",
		Long: `View the status of the Penfold processing pipeline.

The pipeline processes ingested content through several stages:
  1. Ingest: Raw content stored in sources table (pending)
  2. Process: Worker extracts text, generates embeddings
  3. Enrich: AI generates summaries, extracts assertions
  4. Index: Content becomes searchable

Commands:
  status  - Show overall pipeline statistics
  job     - Show details for a specific job
  jobs    - List recent ingest jobs

Documentation:
  Pipeline concepts:   docs/concepts/pipeline.md
  System vision:       docs/shared/vision.md

Related Commands:
  penf reprocess   Shortcut for 'penf pipeline reprocess'
  penf workflow    Monitor individual workflow runs (Temporal)
  penf content     Browse and manage processed content items
  penf logs        View service logs (gateway, worker, AI)`,
	}

	cmd.AddCommand(newPipelineStatusCmd(pipelineDeps))
	cmd.AddCommand(newPipelineJobCmd(pipelineDeps))
	cmd.AddCommand(newPipelineJobsCmd(pipelineDeps))
	cmd.AddCommand(newPipelineReprocessCmd(pipelineDeps))
	cmd.AddCommand(newPipelineKickCmd(pipelineDeps))
	cmd.AddCommand(newPipelineRetryCmd(pipelineDeps))
	cmd.AddCommand(newPipelineWorkersCmd(pipelineDeps))
	cmd.AddCommand(newPipelineLogsCmd(pipelineDeps))
	cmd.AddCommand(newPipelineQueueCmd(pipelineDeps))
	cmd.AddCommand(newPipelineHealthCmd(pipelineDeps))
	cmd.AddCommand(newPipelineDeletedCmd(pipelineDeps))
	cmd.AddCommand(newPipelineUndeleteCmd(pipelineDeps))
	cmd.AddCommand(newPipelineDescribeCmd(pipelineDeps))
	cmd.AddCommand(newPipelinePromptCmd(pipelineDeps))
	cmd.AddCommand(newPipelineHistoryCmd(pipelineDeps))
	cmd.AddCommand(newPipelineConfigCmd(pipelineDeps))
	cmd.AddCommand(newPipelineErrorsCmd(pipelineDeps))
	cmd.AddCommand(newPipelineInspectCmd(pipelineDeps))
	cmd.AddCommand(newPipelineDiffCmd(pipelineDeps))
	cmd.AddCommand(newPipelineStageCmd(pipelineDeps))

	return cmd
}

func newPipelineStatusCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string
	var sinceLastSession bool
	var since string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show pipeline statistics",
		Long: `Show overall pipeline statistics including:
  - Sources by processing status (pending, processing, completed, failed)
  - Embeddings count and recent rate
  - Attachments by tier (auto_process, auto_skip, pending_review)
  - Recent ingest jobs

Flags:
  --since-last-session: Show stats since the last closed session
  --since: Show stats since a specific timestamp (e.g., "2h", "yesterday", ISO timestamp)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate mutual exclusivity
			if sinceLastSession && since != "" {
				return fmt.Errorf("cannot specify both --since-last-session and --since flags")
			}
			return runPipelineStatus(cmd.Context(), deps, outputFormat, sinceLastSession, since)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")
	cmd.Flags().BoolVar(&sinceLastSession, "since-last-session", false, "Show stats since the last closed session")
	cmd.Flags().StringVar(&since, "since", "", "Show stats since this time (e.g., '2h', 'yesterday', ISO timestamp)")
	return cmd
}

func newPipelineJobCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "job <job-id>",
		Short: "Show details for a specific ingest job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineJob(cmd.Context(), deps, args[0], outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")
	return cmd
}

func newPipelineJobsCmd(deps *PipelineCommandDeps) *cobra.Command {
	var limit int
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "List recent ingest jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineJobs(cmd.Context(), deps, limit, outputFormat)
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 10, "Maximum number of jobs to show")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")
	return cmd
}

// filterJobsSince filters jobs created at or after the given timestamp.
func filterJobsSince(jobs []*pipelinev1.JobSummary, sinceTime time.Time) []*pipelinev1.JobSummary {
	var filtered []*pipelinev1.JobSummary
	for _, job := range jobs {
		if job.CreatedAt != nil {
			jobTime := job.CreatedAt.AsTime()
			if jobTime.Equal(sinceTime) || jobTime.After(sinceTime) {
				filtered = append(filtered, job)
			}
		}
	}
	return filtered
}

// parseTimeFilter parses a time filter string (relative or absolute).
func parseTimeFilter(filter string) (time.Time, error) {
	// Try parsing as duration (e.g., "2h", "30m")
	if duration, err := time.ParseDuration(filter); err == nil {
		return time.Now().Add(-duration), nil
	}

	// Try parsing as ISO timestamp
	if t, err := time.Parse(time.RFC3339, filter); err == nil {
		return t, nil
	}

	// Try parsing common formats
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, filter); err == nil {
			return t, nil
		}
	}

	// Handle relative keywords
	now := time.Now()
	switch strings.ToLower(filter) {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		return time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location()), nil
	}

	return time.Time{}, fmt.Errorf("invalid time filter: %s (use duration like '2h', ISO timestamp, or 'yesterday')", filter)
}

// connectPipelineToGateway creates a gRPC connection to the gateway service.
func connectPipelineToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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

// Command execution functions

func runPipelineStatus(ctx context.Context, deps *PipelineCommandDeps, outputFormat string, sinceLastSession bool, since string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Resolve timestamp filter
	var sinceTime *time.Time
	var sinceSource string
	var sessionID string
	var sessionTitle string

	if sinceLastSession {
		// Query Context-Palace for last closed session
		if cfg.ContextPalace == nil || !cfg.ContextPalace.IsConfigured() {
			return fmt.Errorf("Context-Palace not configured (required for --since-last-session)")
		}

		cpClient, err := contextpalace.NewClient(cfg.ContextPalace)
		if err != nil {
			return fmt.Errorf("connecting to Context-Palace: %w", err)
		}
		defer cpClient.Close()

		sessions, err := cpClient.ListShards(ctx, contextpalace.ListShardsOptions{
			Type:   "session",
			Status: "closed",
			Limit:  1,
		})
		if err != nil {
			return fmt.Errorf("querying last closed session: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println("No closed sessions found. Showing full pipeline stats.")
		} else {
			session := sessions[0]
			if session.ClosedAt != nil {
				sinceTime = session.ClosedAt
				sinceSource = "last_session"
				sessionID = session.ID
				sessionTitle = session.Title
			}
		}
	} else if since != "" {
		// Parse manual timestamp
		parsedTime, err := parseTimeFilter(since)
		if err != nil {
			return fmt.Errorf("parsing --since value: %w", err)
		}
		sinceTime = &parsedTime
		sinceSource = "manual"
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.GetStats(ctx, &pipelinev1.GetStatsRequest{})
	if err != nil {
		return fmt.Errorf("getting pipeline stats: %w", err)
	}

	if outputFormat == "json" {
		return outputPipelineStatsJSON(resp.Stats, sinceTime, sinceSource, sessionID)
	}
	return outputPipelineStatsHuman(resp.Stats, sinceTime, sinceSource, sessionID, sessionTitle)
}

func runPipelineJob(ctx context.Context, deps *PipelineCommandDeps, jobID string, outputFormat string) error {
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

	resp, err := client.GetJob(ctx, &pipelinev1.GetJobRequest{JobId: jobID})
	if err != nil {
		return fmt.Errorf("getting job: %w", err)
	}

	if outputFormat == "json" {
		return outputPipelineJobJSON(resp.Job, resp.Sources)
	}
	return outputPipelineJobHuman(resp.Job, resp.Sources)
}

func runPipelineJobs(ctx context.Context, deps *PipelineCommandDeps, limit int, outputFormat string) error {
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

	resp, err := client.ListJobs(ctx, &pipelinev1.ListJobsRequest{Limit: int32(limit)})
	if err != nil {
		return fmt.Errorf("listing jobs: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Jobs)
	}
	return outputPipelineJobsHuman(resp.Jobs)
}

// Output functions

func outputPipelineStatsJSON(stats *pipelinev1.PipelineStats, sinceTime *time.Time, sinceSource, sessionID string) error {
	output := map[string]interface{}{
		"stats": stats,
	}

	if sinceTime != nil {
		output["since"] = sinceTime.Format(time.RFC3339)
		output["since_source"] = sinceSource
		if sessionID != "" {
			output["session_id"] = sessionID
		}

		// Filter recent jobs by timestamp
		filteredJobs := filterJobsSince(stats.RecentJobs, *sinceTime)
		output["jobs_since"] = filteredJobs
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputPipelineStatsHuman(stats *pipelinev1.PipelineStats, sinceTime *time.Time, sinceSource, sessionID, sessionTitle string) error {
	// Header with since info
	if sinceTime != nil {
		fmt.Printf("Pipeline Status (since %s)\n", sinceTime.Format("2006-01-02 15:04:05"))
		if sessionID != "" {
			fmt.Printf("Session: %s \"%s\"\n", sessionID, sessionTitle)
		}
	} else {
		fmt.Println("Pipeline Status")
	}
	fmt.Println("=" + fmt.Sprintf("%49s", "="))
	if stats.Timestamp != nil {
		fmt.Printf("  Timestamp: %s\n\n", stats.Timestamp.AsTime().Format(time.RFC3339))
	}

	// Sources
	fmt.Println("Sources")
	fmt.Println("-" + fmt.Sprintf("%49s", "-"))
	fmt.Printf("  Total: %d\n", stats.SourcesTotal)
	if len(stats.SourcesByStatus) > 0 {
		fmt.Println("  By Status:")
		for _, sc := range stats.SourcesByStatus {
			color := "\033[33m" // Yellow for pending
			if sc.Status == "completed" {
				color = "\033[32m" // Green
			} else if sc.Status == "failed" {
				color = "\033[31m" // Red
			}
			fmt.Printf("    %s%-12s\033[0m %d\n", color, sc.Status, sc.Count)
		}
	}
	if len(stats.SourcesByFailureCategory) > 0 {
		fmt.Println("  By Failure Category:")
		for _, sc := range stats.SourcesByFailureCategory {
			fmt.Printf("    %-20s %d\n", sc.Status, sc.Count)
		}
	}
	fmt.Println()

	// Embeddings
	fmt.Println("Embeddings")
	fmt.Println("-" + fmt.Sprintf("%49s", "-"))
	fmt.Printf("  Total: %d\n", stats.EmbeddingsTotal)
	fmt.Printf("  Last Hour: %d\n", stats.EmbeddingsRecent)

	// Calculate coverage
	if stats.SourcesTotal > 0 {
		coverage := float64(stats.EmbeddingsTotal) / float64(stats.SourcesTotal) * 100
		color := "\033[31m" // Red if low
		if coverage >= 90 {
			color = "\033[32m" // Green
		} else if coverage >= 50 {
			color = "\033[33m" // Yellow
		}
		fmt.Printf("  Coverage: %s%.1f%%\033[0m\n", color, coverage)
	}
	fmt.Println()

	// Attachments
	fmt.Println("Attachments")
	fmt.Println("-" + fmt.Sprintf("%49s", "-"))
	fmt.Printf("  Total: %d\n", stats.AttachmentsTotal)
	if len(stats.AttachmentsByTier) > 0 {
		fmt.Println("  By Tier:")
		for _, sc := range stats.AttachmentsByTier {
			fmt.Printf("    %-16s %d\n", sc.Status, sc.Count)
		}
	}
	fmt.Println()

	// Jobs
	fmt.Println("Ingest Jobs")
	fmt.Println("-" + fmt.Sprintf("%49s", "-"))
	fmt.Printf("  Total: %d\n", stats.JobsTotal)
	if len(stats.JobsByStatus) > 0 {
		fmt.Println("  By Status:")
		for _, sc := range stats.JobsByStatus {
			fmt.Printf("    %-16s %d\n", sc.Status, sc.Count)
		}
	}
	fmt.Println()

	// Recent jobs
	if len(stats.RecentJobs) > 0 {
		fmt.Println("Recent Jobs")
		fmt.Println("-" + fmt.Sprintf("%49s", "-"))
		fmt.Println("  ID                                    STATUS       FILES   IMPORTED")
		for _, job := range stats.RecentJobs {
			statusColor := "\033[32m"
			if job.Status == "failed" {
				statusColor = "\033[31m"
			} else if job.Status == "in_progress" {
				statusColor = "\033[33m"
			}
			fmt.Printf("  %s  %s%-12s\033[0m %5d   %8d\n",
				job.Id, statusColor, job.Status, job.TotalFiles, job.ImportedCount)
		}
	}

	// Jobs since filter
	if sinceTime != nil {
		filteredJobs := filterJobsSince(stats.RecentJobs, *sinceTime)
		fmt.Println()
		fmt.Printf("Jobs since %s\n", sinceTime.Format("2006-01-02 15:04:05"))
		fmt.Println("-" + fmt.Sprintf("%49s", "-"))
		if len(filteredJobs) == 0 {
			fmt.Println("  No jobs since this time")
		} else {
			fmt.Println("  ID                                    STATUS       FILES   IMPORTED")
			for _, job := range filteredJobs {
				statusColor := "\033[32m"
				if job.Status == "failed" {
					statusColor = "\033[31m"
				} else if job.Status == "in_progress" {
					statusColor = "\033[33m"
				}
				fmt.Printf("  %s  %s%-12s\033[0m %5d   %8d\n",
					job.Id, statusColor, job.Status, job.TotalFiles, job.ImportedCount)
			}
		}
	}

	return nil
}

func outputPipelineJobJSON(job *pipelinev1.JobDetails, sources *pipelinev1.SourceStats) error {
	output := map[string]interface{}{
		"job":     job,
		"sources": sources,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputPipelineJobHuman(job *pipelinev1.JobDetails, sources *pipelinev1.SourceStats) error {
	summary := job.Summary
	if summary == nil {
		return fmt.Errorf("job summary is nil")
	}

	fmt.Printf("Job: %s\n", summary.Id)
	fmt.Println("=" + fmt.Sprintf("%49s", "="))
	fmt.Printf("  Status:      %s\n", summary.Status)
	fmt.Printf("  Source Tag:  %s\n", summary.SourceTag)
	if summary.CreatedAt != nil {
		fmt.Printf("  Created:     %s\n", summary.CreatedAt.AsTime().Format(time.RFC3339))
	}
	if summary.CompletedAt != nil {
		completedAt := summary.CompletedAt.AsTime()
		fmt.Printf("  Completed:   %s\n", completedAt.Format(time.RFC3339))
		if summary.CreatedAt != nil {
			duration := completedAt.Sub(summary.CreatedAt.AsTime())
			fmt.Printf("  Duration:    %s\n", duration.Round(time.Millisecond))
		}
	}
	fmt.Println()

	fmt.Println("Ingest Results")
	fmt.Println("-" + fmt.Sprintf("%49s", "-"))
	fmt.Printf("  Total Files:   %d\n", summary.TotalFiles)
	fmt.Printf("  Imported:      \033[32m%d\033[0m\n", summary.ImportedCount)
	fmt.Printf("  Skipped:       \033[33m%d\033[0m (duplicates)\n", summary.SkippedCount)
	fmt.Printf("  Failed:        \033[31m%d\033[0m\n", summary.FailedCount)
	fmt.Println()

	if sources != nil {
		fmt.Println("Source Processing")
		fmt.Println("-" + fmt.Sprintf("%49s", "-"))
		fmt.Printf("  Total Sources: %d\n", sources.Total)
		for _, sc := range sources.ByStatus {
			color := "\033[33m"
			if sc.Status == "completed" {
				color = "\033[32m"
			} else if sc.Status == "failed" {
				color = "\033[31m"
			}
			fmt.Printf("  %s%-12s\033[0m %d\n", color, sc.Status, sc.Count)
		}
	}

	return nil
}

func outputPipelineJobsHuman(jobs []*pipelinev1.JobSummary) error {
	if len(jobs) == 0 {
		fmt.Println("No ingest jobs found.")
		return nil
	}

	fmt.Println("Recent Ingest Jobs")
	fmt.Println("=" + fmt.Sprintf("%79s", "="))
	fmt.Println("ID                                    STATUS         TAG              FILES  IMPORTED  FAILED")
	fmt.Println("-" + fmt.Sprintf("%79s", "-"))

	for _, job := range jobs {
		statusColor := "\033[32m"
		if job.Status == "failed" {
			statusColor = "\033[31m"
		} else if job.Status == "in_progress" || job.Status == "pending" {
			statusColor = "\033[33m"
		}

		tag := job.SourceTag
		if len(tag) > 16 {
			tag = tag[:13] + "..."
		}

		fmt.Printf("%s  %s%-12s\033[0m  %-16s %5d  %8d  %6d\n",
			job.Id, statusColor, job.Status, tag, job.TotalFiles, job.ImportedCount, job.FailedCount)
	}

	return nil
}

func newPipelineReprocessCmd(deps *PipelineCommandDeps) *cobra.Command {
	var stage string
	var confirm bool
	var outputFormat string
	var reason string
	var dryRun bool
	var all bool
	var sourceTag string
	var timeout int32
	var model string

	cmd := &cobra.Command{
		Use:   "reprocess <content-id>",
		Short: "Trigger reprocessing of content",
		Long: `Trigger reprocessing of an already-processed content item.

This is useful for:
  - Re-running processing after model updates
  - Fixing content that failed during processing
  - Updating embeddings or summaries with new models

Stages that can be reprocessed:
  - embeddings: Regenerate vector embeddings
  - entities: Re-extract entities and structured data
  - keywords: Re-extract keywords
  - summary: Regenerate AI summaries

Processing Overrides:
  --timeout    Override timeout for this reprocessing run (seconds)
  --model      Override model ID for this reprocessing run

Examples:
  # Reprocess all stages for a content item
  penf pipeline reprocess content-123

  # Reprocess only embeddings
  penf pipeline reprocess content-123 --stage=embeddings

  # Reprocess with a reason
  penf pipeline reprocess content-123 --reason="Updated to new model"

  # Reprocess with custom model and timeout
  penf pipeline reprocess content-123 --model gemini-2.0-pro --timeout 300

  # Dry-run to see impact
  penf pipeline reprocess --stage triage --dry-run

  # Dry-run for specific source tag with overrides
  penf pipeline reprocess --stage triage --dry-run --source-tag gmail-import --timeout 120`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contentID := ""
			if len(args) > 0 {
				contentID = args[0]
			}
			return runPipelineReprocess(cmd.Context(), deps, contentID, stage, reason, outputFormat, dryRun, all, sourceTag, timeout, model)
		},
	}

	cmd.Flags().StringVar(&stage, "stage", "", "Specific stage to reprocess: embeddings, entities, keywords, summary")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Required for bulk operations (future: --source, --all flags)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")
	cmd.Flags().StringVar(&reason, "reason", "Manual reprocess via CLI", "Reason for reprocessing (for audit trail)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Calculate impact without executing")
	cmd.Flags().BoolVar(&all, "all", false, "Reprocess all sources (for bulk operations)")
	cmd.Flags().StringVar(&sourceTag, "source-tag", "", "Filter by source tag")
	cmd.Flags().Int32Var(&timeout, "timeout", 0, "Timeout override in seconds (0 = use default)")
	cmd.Flags().StringVar(&model, "model", "", "Model ID override (empty = use default)")

	return cmd
}

func runPipelineReprocess(ctx context.Context, deps *PipelineCommandDeps, contentID string, stage string, reason string, outputFormat string, dryRun bool, all bool, sourceTag string, timeout int32, model string) error {
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

	// If dry-run, call ReprocessDryRun
	if dryRun {
		if stage == "" {
			return fmt.Errorf("--stage is required for dry-run")
		}

		// Show overrides that would be applied
		if model != "" || timeout > 0 {
			fmt.Println("Overrides:")
			if model != "" {
				fmt.Printf("  Model: %s\n", model)
			}
			if timeout > 0 {
				fmt.Printf("  Timeout: %d seconds\n", timeout)
			}
			fmt.Println()
		}

		pipelineClient := pipelinev1.NewPipelineServiceClient(conn)

		req := &pipelinev1.ReprocessDryRunRequest{
			Stage:     stage,
			SourceTag: sourceTag,
		}

		resp, err := pipelineClient.ReprocessDryRun(ctx, req)
		if err != nil {
			return fmt.Errorf("running dry-run: %w", err)
		}

		if outputFormat == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp)
		}

		return outputReprocessDryRunHuman(resp, stage)
	}

	// Handle --all flag for bulk reprocessing
	if all {
		if contentID != "" {
			return fmt.Errorf("cannot specify both content-id and --all flag")
		}

		return runBulkReprocess(ctx, conn, cfg, sourceTag, stage, reason, outputFormat, timeout, model)
	}

	// Otherwise, call ReprocessContent for single item
	if contentID == "" {
		return fmt.Errorf("content-id is required (use --all to reprocess all content, or --dry-run to see impact)")
	}

	// Create content processor client directly from connection
	contentClient := contentv1.NewContentProcessorServiceClient(conn)

	// Build request
	req := &contentv1.ReprocessContentRequest{
		ContentId: contentID,
		Reason:    reason,
	}

	// Add specific stage if provided
	if stage != "" {
		stageEnum, err := parseProcessingStage(stage)
		if err != nil {
			return err
		}
		req.StagesToReprocess = []contentv1.ProcessingStage{stageEnum}
	}

	// Add processing options if overrides are provided
	if model != "" || timeout > 0 {
		req.Options = &contentv1.ProcessingOptions{}
		if model != "" {
			req.Options.ModelId = &model
		}
		if timeout > 0 {
			req.Options.TimeoutSeconds = &timeout
		}
	}

	// Call ReprocessContent RPC
	resp, err := contentClient.ReprocessContent(ctx, req)
	if err != nil {
		return fmt.Errorf("reprocessing content: %w", err)
	}

	// Output results
	if outputFormat == "json" {
		return outputReprocessJSON(resp)
	}
	return outputReprocessHuman(resp)
}

// runBulkReprocess reprocesses all content items matching filters.
func runBulkReprocess(ctx context.Context, conn *grpc.ClientConn, cfg *config.CLIConfig, sourceTag string, stage string, reason string, outputFormat string, timeout int32, model string) error {
	contentClient := contentv1.NewContentProcessorServiceClient(conn)

	// Use tenant ID from config
	tenantID := cfg.TenantID
	if tenantID == "" {
		return fmt.Errorf("tenant_id not configured")
	}

	// Build filter for listing content
	listReq := &contentv1.ListContentItemsRequest{
		TenantId: tenantID,
		PageSize: 100, // Process in batches of 100
	}

	// Optional source type filter from source tag
	if sourceTag != "" {
		listReq.SourceType = &sourceTag
	}

	fmt.Printf("Querying content items for tenant %s", tenantID)
	if sourceTag != "" {
		fmt.Printf(" (source_tag: %s)", sourceTag)
	}
	fmt.Println("...")

	// Get all content items
	var allContentIDs []string
	var pageToken string
	for {
		listReq.PageToken = pageToken
		resp, err := contentClient.ListContentItems(ctx, listReq)
		if err != nil {
			return fmt.Errorf("listing content items: %w", err)
		}

		for _, item := range resp.Items {
			allContentIDs = append(allContentIDs, item.Id)
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	if len(allContentIDs) == 0 {
		fmt.Println("No content items found to reprocess.")
		return nil
	}

	fmt.Printf("Found %d content items to reprocess.\n", len(allContentIDs))

	// Build stage filter if provided
	var stagesToReprocess []contentv1.ProcessingStage
	if stage != "" {
		stageEnum, err := parseProcessingStage(stage)
		if err != nil {
			return err
		}
		stagesToReprocess = []contentv1.ProcessingStage{stageEnum}
	}

	// Reprocess each content item
	successCount := 0
	failCount := 0
	var jobIDs []string

	for i, contentID := range allContentIDs {
		req := &contentv1.ReprocessContentRequest{
			ContentId:         contentID,
			Reason:            reason,
			StagesToReprocess: stagesToReprocess,
		}

		// Add processing options if overrides are provided
		if model != "" || timeout > 0 {
			req.Options = &contentv1.ProcessingOptions{}
			if model != "" {
				req.Options.ModelId = &model
			}
			if timeout > 0 {
				req.Options.TimeoutSeconds = &timeout
			}
		}

		resp, err := contentClient.ReprocessContent(ctx, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to reprocess %s: %v\n", contentID, err)
			failCount++
			continue
		}

		successCount++
		if resp.JobId != "" {
			jobIDs = append(jobIDs, resp.JobId)
		}

		// Show progress every 10 items
		if (i+1)%10 == 0 || i == len(allContentIDs)-1 {
			fmt.Printf("Progress: %d/%d reprocessed (%d succeeded, %d failed)\n", i+1, len(allContentIDs), successCount, failCount)
		}
	}

	fmt.Println()
	fmt.Printf("Bulk reprocess complete: %d succeeded, %d failed\n", successCount, failCount)

	if outputFormat == "json" {
		result := map[string]interface{}{
			"total_count":   len(allContentIDs),
			"success_count": successCount,
			"fail_count":    failCount,
			"job_ids":       jobIDs,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if len(jobIDs) > 0 {
		fmt.Printf("\nJob IDs (first 10): %v\n", jobIDs[:min(10, len(jobIDs))])
		if len(jobIDs) > 10 {
			fmt.Printf("... and %d more\n", len(jobIDs)-10)
		}
	}

	return nil
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func parseProcessingStage(stage string) (contentv1.ProcessingStage, error) {
	switch stage {
	case "embeddings", "embed":
		return contentv1.ProcessingStage_PROCESSING_STAGE_EMBED, nil
	case "entities", "extract":
		return contentv1.ProcessingStage_PROCESSING_STAGE_EXTRACT, nil
	case "summary", "summarize":
		return contentv1.ProcessingStage_PROCESSING_STAGE_SUMMARIZE, nil
	case "keywords":
		// Note: Keywords might be part of EXTRACT stage - adjust if needed
		return contentv1.ProcessingStage_PROCESSING_STAGE_EXTRACT, nil
	default:
		return contentv1.ProcessingStage_PROCESSING_STAGE_UNSPECIFIED, fmt.Errorf("invalid stage: %s (must be: embeddings, entities, keywords, summary)", stage)
	}
}

func outputReprocessJSON(resp *contentv1.ReprocessContentResponse) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func outputReprocessHuman(resp *contentv1.ReprocessContentResponse) error {
	fmt.Printf("Reprocessing started for content: %s\n", resp.ContentId)
	fmt.Printf("Job ID: %s\n", resp.JobId)
	if resp.PreviousJobId != "" {
		fmt.Printf("Previous Job ID: %s\n", resp.PreviousJobId)
	}

	if resp.Status != nil {
		fmt.Printf("\nStatus: %s\n", resp.Status.State.String())
		if resp.Status.CurrentStage != nil {
			fmt.Printf("Current Stage: %s\n", resp.Status.CurrentStage.String())
		}
		if resp.Status.ProgressPercent > 0 {
			fmt.Printf("Progress: %d%%\n", resp.Status.ProgressPercent)
		}
	}

	fmt.Println("\nUse 'penf pipeline job <job-id>' to check progress")
	return nil
}

func newPipelineKickCmd(deps *PipelineCommandDeps) *cobra.Command {
	var tenant string
	var limit int
	var source string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "kick",
		Short: "Trigger processing of pending items",
		Long: `Trigger processing of pending pipeline items.

This command starts processing for items that are in pending state,
useful for manually triggering a processing batch.

Examples:
  # Kick all pending items
  penf pipeline kick

  # Kick with limit
  penf pipeline kick --limit=100

  # Kick specific source
  penf pipeline kick --source=gmail-import-2024

  # Kick for specific tenant
  penf pipeline kick --tenant=tenant-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineKick(cmd.Context(), deps, tenant, limit, source, outputFormat)
		},
	}

	cmd.Flags().StringVar(&tenant, "tenant", "", "Filter by tenant ID")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "Maximum number of items to queue (0 = no limit)")
	cmd.Flags().StringVar(&source, "source", "", "Filter by source tag")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineKick(ctx context.Context, deps *PipelineCommandDeps, tenant string, limit int, source string, outputFormat string) error {
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

	req := &pipelinev1.KickProcessingRequest{
		TenantId:  tenant,
		Limit:     int32(limit),
		SourceTag: source,
	}

	resp, err := client.KickProcessing(ctx, req)
	if err != nil {
		return fmt.Errorf("kicking processing: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	fmt.Printf("Queued %d items for processing\n", resp.QueuedCount)
	if resp.Message != "" {
		fmt.Printf("%s\n", resp.Message)
	}
	return nil
}

func newPipelineRetryCmd(deps *PipelineCommandDeps) *cobra.Command {
	var stage string
	var tenant string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "retry [job-id]",
		Short: "Retry failed processing jobs",
		Long: `Retry failed pipeline items.

If a job ID is provided, retries only that specific job.
Otherwise, retries all failed items matching the filters.

Examples:
  # Retry all failed items
  penf pipeline retry

  # Retry specific job
  penf pipeline retry job-abc123

  # Retry failed embeddings
  penf pipeline retry --stage=embedding

  # Retry for specific tenant
  penf pipeline retry --tenant=tenant-123`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := ""
			if len(args) > 0 {
				jobID = args[0]
			}
			return runPipelineRetry(cmd.Context(), deps, jobID, stage, tenant, outputFormat)
		},
	}

	cmd.Flags().StringVar(&stage, "stage", "", "Filter by pipeline stage (embedding, attachment)")
	cmd.Flags().StringVar(&tenant, "tenant", "", "Filter by tenant ID")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineRetry(ctx context.Context, deps *PipelineCommandDeps, jobID string, stage string, tenant string, outputFormat string) error {
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

	req := &pipelinev1.RetryFailedRequest{
		TenantId: tenant,
		JobId:    jobID,
		Stage:    stage,
	}

	resp, err := client.RetryFailed(ctx, req)
	if err != nil {
		return fmt.Errorf("retrying failed items: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	fmt.Printf("Retried %d failed items\n", resp.RetriedCount)
	if resp.Message != "" {
		fmt.Printf("%s\n", resp.Message)
	}
	return nil
}

func newPipelineWorkersCmd(deps *PipelineCommandDeps) *cobra.Command {
	var tenant string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "workers",
		Short: "Show worker status",
		Long: `Show status of pipeline workers.

Displays running workflow instances that are processing pipeline items,
providing insight into current processing activity.

Examples:
  # Show all workers
  penf pipeline workers

  # Show workers for specific tenant
  penf pipeline workers --tenant=tenant-123

  # Output as JSON
  penf pipeline workers -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineWorkers(cmd.Context(), deps, tenant, outputFormat)
		},
	}

	cmd.Flags().StringVar(&tenant, "tenant", "", "Filter by tenant ID")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineWorkers(ctx context.Context, deps *PipelineCommandDeps, tenant string, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Initialize gRPC client
	grpcClient, err := client.ConnectFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("initializing client: %w", err)
	}
	defer grpcClient.Close()

	// Use ListWorkflows to get running workers
	filter := client.ListWorkflowsFilter{
		Status:   "Running",
		PageSize: 100, // Get more workers
	}

	rpcCtx, rpcCancel := context.WithTimeout(ctx, 30*time.Second)
	defer rpcCancel()
	result, err := grpcClient.ListWorkflows(rpcCtx, filter)
	if err != nil {
		return fmt.Errorf("listing workflows: %w", err)
	}

	// Note: tenant filtering would require GetWorkflowStatus for each workflow
	// to access SearchAttributes. For now, show all running workflows.
	workers := result.Workflows

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"workers":      workers,
			"total_count":  len(workers),
			"running_only": true,
		})
	}

	// Human-readable output
	if len(workers) == 0 {
		fmt.Println("No running workers found.")
		return nil
	}

	fmt.Printf("Pipeline Workers (%d running):\n\n", len(workers))
	fmt.Println("  WORKFLOW ID                           TYPE                      STATUS      STARTED")
	fmt.Println("  -----------                           ----                      ------      -------")

	for _, wf := range workers {
		workflowType := wf.WorkflowType
		if len(workflowType) > 24 {
			workflowType = workflowType[:21] + "..."
		}

		startTime := wf.StartTime.Format("15:04:05")
		if time.Since(wf.StartTime) > 24*time.Hour {
			startTime = wf.StartTime.Format("Jan 02 15:04")
		}

		fmt.Printf("  %-37s %-25s \033[34m%-11s\033[0m %s\n",
			wf.WorkflowID, workflowType, wf.Status, startTime)
	}

	fmt.Println()
	return nil
}

func newPipelineLogsCmd(deps *PipelineCommandDeps) *cobra.Command {
	var contentID string
	var tail bool
	var since string
	var level string
	var service string
	var outputFormat string
	var limit int

	cmd := &cobra.Command{
		Use:   "logs [job-id]",
		Short: "View processing logs for jobs and content",
		Long: `View processing logs for ingest jobs and content items.

Filter logs by job ID (trace_id), content ID, service, level, or time range.
Use --tail to stream logs in real-time.

Examples:
  # Logs for ingest job
  penf pipeline logs job-abc123

  # Logs for content item
  penf pipeline logs --content content-456

  # Stream live logs for a job
  penf pipeline logs job-abc123 --tail

  # Logs from last hour
  penf pipeline logs job-abc123 --since 1h

  # Filter by level
  penf pipeline logs job-abc123 --level error

  # Filter by service
  penf pipeline logs job-abc123 --service worker`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := ""
			if len(args) > 0 {
				jobID = args[0]
			}
			if jobID == "" && contentID == "" {
				return fmt.Errorf("must provide either job-id or --content flag")
			}
			if jobID != "" && contentID != "" {
				return fmt.Errorf("cannot specify both job-id and --content flag")
			}
			return runPipelineLogs(cmd.Context(), deps, jobID, contentID, tail, since, level, service, outputFormat, limit)
		},
	}

	cmd.Flags().StringVar(&contentID, "content", "", "Filter by content ID")
	cmd.Flags().BoolVar(&tail, "tail", false, "Stream logs in real-time")
	cmd.Flags().StringVar(&since, "since", "15m", "Show logs since this time ago (e.g., 5m, 1h, 24h)")
	cmd.Flags().StringVar(&level, "level", "", "Filter by log level (debug, info, warn, error)")
	cmd.Flags().StringVar(&service, "service", "", "Filter by service name")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")
	cmd.Flags().IntVarP(&limit, "limit", "n", 100, "Maximum number of log entries (not used with --tail)")

	return cmd
}

func runPipelineLogs(ctx context.Context, deps *PipelineCommandDeps, jobID string, contentID string, tail bool, since string, level string, service string, outputFormat string, limit int) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Parse time duration
	var sinceTime time.Time
	if since != "" {
		duration, err := time.ParseDuration(since)
		if err != nil {
			return fmt.Errorf("invalid --since duration: %w", err)
		}
		sinceTime = time.Now().Add(-duration)
	}

	// Validate log level if provided
	if level != "" {
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !validLevels[level] {
			return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", level)
		}
	}

	// Initialize gRPC client
	grpcClient, err := client.ConnectFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("initializing client: %w", err)
	}
	defer grpcClient.Close()

	// Build filter - use job ID or content ID as trace_id
	traceID := jobID
	if contentID != "" {
		traceID = contentID
	}

	filter := client.LogFilter{
		Service: service,
		Level:   level,
		Since:   sinceTime,
		TraceID: traceID,
	}

	// Stream logs if --tail is specified
	if tail {
		fmt.Printf("Following logs for %s (press Ctrl+C to stop)...\n\n", traceID)

		// For tail mode, start from now
		filter.Since = time.Now()

		// StreamLogs uses context for cancellation, no timeout needed
		err := grpcClient.StreamLogs(ctx, filter, 1000, func(entry client.LogEntry) {
			if outputFormat == "json" {
				enc := json.NewEncoder(os.Stdout)
				_ = enc.Encode(entry)
			} else {
				outputPipelineLogEntry(entry)
			}
		})

		if err != nil && ctx.Err() == nil {
			return fmt.Errorf("streaming logs: %w", err)
		}

		fmt.Println("\nStopped following logs.")
		return nil
	}

	// List logs with RPC timeout
	rpcCtx, rpcCancel := context.WithTimeout(ctx, 30*time.Second)
	defer rpcCancel()
	resp, err := grpcClient.ListLogs(rpcCtx, filter, limit, 0, false)
	if err != nil {
		return fmt.Errorf("fetching logs: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	return outputPipelineLogsText(resp, traceID)
}

func outputPipelineLogEntry(entry client.LogEntry) {
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
	levelColor := getLogLevelColorForPipeline(entry.Level)
	levelStr := fmt.Sprintf("%-5s", strings.ToUpper(entry.Level))
	serviceStr := fmt.Sprintf("%-10s", entry.Service)

	fmt.Printf("%s  %s%s\033[0m  \033[36m%s\033[0m  %s\n",
		timestamp, levelColor, levelStr, serviceStr, entry.Message)
}

func outputPipelineLogsText(resp *client.LogsResponse, traceID string) error {
	if len(resp.Entries) == 0 {
		fmt.Printf("No log entries found for %s.\n", traceID)
		return nil
	}

	fmt.Printf("Logs for %s:\n\n", traceID)
	fmt.Println("TIMESTAMP           LEVEL   SERVICE     MESSAGE")
	fmt.Println("-------------------------------------------------------------------")

	for _, entry := range resp.Entries {
		outputPipelineLogEntry(entry)
	}

	if resp.Truncated {
		fmt.Printf("\n(Showing %d entries, use --limit to see more)\n", len(resp.Entries))
	}

	return nil
}

func getLogLevelColorForPipeline(level string) string {
	switch strings.ToLower(level) {
	case "debug":
		return "\033[90m" // Gray
	case "info":
		return "\033[32m" // Green
	case "warn":
		return "\033[33m" // Yellow
	case "error":
		return "\033[31m" // Red
	default:
		return ""
	}
}

func newPipelineQueueCmd(deps *PipelineCommandDeps) *cobra.Command {
	var stage string
	var stuck bool
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Show pipeline queue status",
		Long: `Show processing queue depths and rates.

Displays pending items, processing items, processing rates, and worker counts
for each pipeline stage (embeddings, entities, keywords, etc.).

Examples:
  # Show all queues
  penf pipeline queue

  # Show specific stage
  penf pipeline queue --stage embeddings

  # Show stuck items (older than 5 minutes)
  penf pipeline queue --stuck

  # Output as JSON
  penf pipeline queue -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineQueue(cmd.Context(), deps, stage, stuck, outputFormat)
		},
	}

	cmd.Flags().StringVar(&stage, "stage", "", "Filter by specific stage (embeddings, entities, keywords)")
	cmd.Flags().BoolVar(&stuck, "stuck", false, "Show only items stuck > 5 minutes")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineQueue(ctx context.Context, deps *PipelineCommandDeps, stage string, stuck bool, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Initialize gRPC client
	grpcClient, err := client.ConnectFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("initializing client: %w", err)
	}
	defer grpcClient.Close()

	// Get queue status
	rpcCtx, rpcCancel := context.WithTimeout(ctx, 30*time.Second)
	defer rpcCancel()
	resp, err := grpcClient.GetQueueStatus(rpcCtx, stage)
	if err != nil {
		return fmt.Errorf("getting queue status: %w", err)
	}

	// Filter stuck items if requested
	queues := resp.Queues
	if stuck {
		var stuckQueues []*pipelinev1.QueueStats
		for _, q := range queues {
			if q.OldestItemAgeSeconds > 300 { // 5 minutes
				stuckQueues = append(stuckQueues, q)
			}
		}
		queues = stuckQueues
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"queues": queues,
		})
	}

	return outputPipelineQueueText(queues, stuck)
}

func outputPipelineQueueText(queues []*pipelinev1.QueueStats, stuckOnly bool) error {
	if len(queues) == 0 {
		if stuckOnly {
			fmt.Println("No stuck items found.")
		} else {
			fmt.Println("No queue data available.")
		}
		return nil
	}

	fmt.Println("Pipeline Queues:")
	fmt.Println()
	fmt.Println("QUEUE        PENDING  PROCESSING  RATE/MIN  OLDEST    WORKERS")
	fmt.Println("-----        -------  ----------  --------  ------    -------")

	var totalPending, totalProcessing int64
	for _, q := range queues {
		// Format oldest item age
		oldest := "-"
		if q.OldestItemAgeSeconds > 0 {
			duration := time.Duration(q.OldestItemAgeSeconds) * time.Second
			if duration < time.Minute {
				oldest = fmt.Sprintf("%ds ago", int(duration.Seconds()))
			} else if duration < time.Hour {
				oldest = fmt.Sprintf("%dm ago", int(duration.Minutes()))
			} else {
				oldest = fmt.Sprintf("%dh ago", int(duration.Hours()))
			}
		}

		// Color code based on status
		nameColor := ""
		if q.PendingCount > 100 {
			nameColor = "\033[33m" // Yellow for high pending
		}
		if q.OldestItemAgeSeconds > 300 { // > 5 minutes
			nameColor = "\033[31m" // Red for stuck
		}

		fmt.Printf("%s%-12s\033[0m %7d  %10d  %8.1f  %-9s %d\n",
			nameColor,
			q.Name,
			q.PendingCount,
			q.ProcessingCount,
			q.RatePerMinute,
			oldest,
			q.WorkerCount)

		totalPending += q.PendingCount
		totalProcessing += q.ProcessingCount
	}

	fmt.Println()
	fmt.Printf("Total: %d pending, %d processing\n", totalPending, totalProcessing)

	return nil
}

func newPipelineHealthCmd(deps *PipelineCommandDeps) *cobra.Command {
	var watch bool
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check pipeline health",
		Long: `Perform a comprehensive pipeline health check.

Checks database connectivity, queue status, worker availability,
and model availability. Provides actionable recommendations for issues.

Examples:
  # One-time health check
  penf pipeline health

  # Continuous monitoring (refresh every 5 seconds)
  penf pipeline health --watch

  # Output as JSON
  penf pipeline health -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineHealth(cmd.Context(), deps, watch, outputFormat)
		},
	}

	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Continuously monitor health (refresh every 5s)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineHealth(ctx context.Context, deps *PipelineCommandDeps, watch bool, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Initialize gRPC client
	grpcClient, err := client.ConnectFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("initializing client: %w", err)
	}
	defer grpcClient.Close()

	// Watch mode - refresh periodically
	if watch {
		if outputFormat == "json" {
			return fmt.Errorf("--watch mode not supported with JSON output")
		}

		fmt.Println("Monitoring pipeline health (press Ctrl+C to stop)...")
		fmt.Println()

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Show initial health
		if err := fetchAndDisplayHealth(ctx, grpcClient); err != nil {
			return err
		}

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				// Clear screen and show updated health
				fmt.Print("\033[H\033[2J") // Clear screen
				if err := fetchAndDisplayHealth(ctx, grpcClient); err != nil {
					return err
				}
			}
		}
	}

	// One-time health check
	rpcCtx, rpcCancel := context.WithTimeout(ctx, 30*time.Second)
	defer rpcCancel()
	resp, err := grpcClient.GetPipelineHealth(rpcCtx)
	if err != nil {
		return fmt.Errorf("getting pipeline health: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	return outputPipelineHealthText(resp)
}

func fetchAndDisplayHealth(ctx context.Context, grpcClient *client.GRPCClient) error {
	rpcCtx, rpcCancel := context.WithTimeout(ctx, 30*time.Second)
	defer rpcCancel()
	resp, err := grpcClient.GetPipelineHealth(rpcCtx)
	if err != nil {
		return fmt.Errorf("getting pipeline health: %w", err)
	}

	return outputPipelineHealthText(resp)
}

func outputPipelineHealthText(resp *pipelinev1.GetPipelineHealthResponse) error {
	// Overall status with color
	statusColor := "\033[32m" // Green
	if resp.OverallStatus == "DEGRADED" {
		statusColor = "\033[33m" // Yellow
	} else if resp.OverallStatus == "UNHEALTHY" {
		statusColor = "\033[31m" // Red
	}

	fmt.Printf("Pipeline Health: %s%s\033[0m\n", statusColor, resp.OverallStatus)
	fmt.Println()

	// Individual health checks
	for _, check := range resp.Checks {
		checkmark := "\033[31m✗\033[0m" // Red X
		if check.Healthy {
			checkmark = "\033[32m✓\033[0m" // Green checkmark
		}

		fmt.Printf("%s %s: %s\n", checkmark, check.Name, check.Status)
		if check.Message != "" && check.Message != check.Status {
			fmt.Printf("  %s\n", check.Message)
		}
	}

	// Issues
	if len(resp.Issues) > 0 {
		fmt.Println()
		fmt.Println("\033[1mIssues:\033[0m")
		for _, issue := range resp.Issues {
			fmt.Printf("  \033[33m⚠\033[0m  %s\n", issue)
		}
	}

	fmt.Println()
	return nil
}

func newPipelineDeletedCmd(deps *PipelineCommandDeps) *cobra.Command {
	var limit int
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "deleted",
		Short: "List soft-deleted sources",
		Long: `List sources that have been soft-deleted.

These sources are excluded from normal pipeline operations but can be restored
using the 'undelete' command.

Examples:
  # List deleted sources
  penf pipeline deleted

  # List more sources
  penf pipeline deleted --limit=100

  # Output as JSON
  penf pipeline deleted -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineDeleted(cmd.Context(), deps, limit, outputFormat)
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Maximum number of sources to show")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineDeleted(ctx context.Context, deps *PipelineCommandDeps, limit int, outputFormat string) error {
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

	resp, err := client.ListDeletedSources(ctx, &pipelinev1.ListDeletedSourcesRequest{
		Limit: int32(limit),
	})
	if err != nil {
		return fmt.Errorf("listing deleted sources: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Sources)
	}

	return outputDeletedSourcesHuman(resp.Sources)
}

func outputDeletedSourcesHuman(sources []*pipelinev1.DeletedSource) error {
	if len(sources) == 0 {
		fmt.Println("No deleted sources found.")
		return nil
	}

	fmt.Printf("Deleted Sources (%d):\n\n", len(sources))
	fmt.Println("ID      SOURCE          EXTERNAL_ID                              STATUS       DELETED_AT           DELETED_BY")
	fmt.Println("------  --------------  ---------------------------------------  -----------  -------------------  ----------")

	for _, src := range sources {
		externalID := src.ExternalId
		if len(externalID) > 39 {
			externalID = externalID[:36] + "..."
		}

		deletedAt := "-"
		if src.DeletedAt != nil {
			deletedAt = src.DeletedAt.AsTime().Format("2006-01-02 15:04:05")
		}

		deletedBy := src.DeletedBy
		if deletedBy == "" {
			deletedBy = "-"
		}

		fmt.Printf("%-6d  %-14s  %-39s  %-11s  %-19s  %s\n",
			src.Id, src.SourceSystem, externalID, src.ProcessingStatus, deletedAt, deletedBy)
	}

	fmt.Println()
	fmt.Println("To restore a source: penf pipeline undelete <source-id>")

	return nil
}

func newPipelineUndeleteCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "undelete <source-id>",
		Short: "Restore a soft-deleted source",
		Long: `Restore a soft-deleted source by its ID.

The source will be restored to its previous processing status and will be
included in normal pipeline operations again.

Examples:
  # Restore a deleted source
  penf pipeline undelete 1234

  # Output as JSON
  penf pipeline undelete 1234 -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid source ID: %s", args[0])
			}
			return runPipelineUndelete(cmd.Context(), deps, sourceID, outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineUndelete(ctx context.Context, deps *PipelineCommandDeps, sourceID int64, outputFormat string) error {
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

	resp, err := client.UndeleteSource(ctx, &pipelinev1.UndeleteSourceRequest{
		SourceId: sourceID,
	})
	if err != nil {
		return fmt.Errorf("undeleting source: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	if resp.Success {
		fmt.Printf("✓ %s\n", resp.Message)
		fmt.Println("\nThe source will now appear in pipeline status and can be processed.")
		fmt.Println("To kick processing: penf pipeline kick")
	} else {
		fmt.Printf("✗ Failed: %s\n", resp.Message)
	}

	return nil
}

func outputReprocessDryRunHuman(resp *pipelinev1.ReprocessDryRunResponse, stage string) error {
	fmt.Printf("Reprocess Dry Run: Stage %s\n", stage)
	fmt.Println("================================")

	if len(resp.AffectedStages) > 0 {
		fmt.Println("Downstream stages that would re-run:")
		for _, s := range resp.AffectedStages {
			fmt.Printf("  - %s\n", s)
		}
		fmt.Println()
	}

	fmt.Printf("Affected sources: %d\n", resp.SourceCount)

	if resp.EstimatedDurationSeconds > 0 {
		duration := resp.EstimatedDurationSeconds
		if duration < 60 {
			fmt.Printf("Estimated duration: %d seconds\n", duration)
		} else if duration < 3600 {
			fmt.Printf("Estimated duration: %.1f minutes\n", float64(duration)/60)
		} else {
			fmt.Printf("Estimated duration: %.1f hours\n", float64(duration)/3600)
		}
	}

	if resp.Message != "" {
		fmt.Println()
		fmt.Println(resp.Message)
	}

	fmt.Println()
	fmt.Printf("To execute: penf pipeline reprocess --stage %s --all\n", stage)

	return nil
}

func newPipelineDiffCmd(deps *PipelineCommandDeps) *cobra.Command {
	var runA int64
	var runB int64
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "diff <source-id>",
		Short: "Compare two pipeline runs for a source",
		Long: `Compare pipeline run outputs for a source to identify changes in processing results.

By default, compares the two most recent runs. Use --run-a and --run-b to compare
specific runs by their run IDs.

This is useful for:
  - Verifying reprocessing changes after model updates
  - Debugging processing differences
  - Validating pipeline improvements

Examples:
  # Compare two most recent runs
  penf pipeline diff 123

  # Compare specific runs
  penf pipeline diff 123 --run-a 456 --run-b 789

  # Output as JSON for programmatic analysis
  penf pipeline diff 123 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid source-id: %s (must be a number)", args[0])
			}
			return runPipelineDiff(cmd.Context(), deps, sourceID, runA, runB, outputFormat)
		},
	}

	cmd.Flags().Int64Var(&runA, "run-a", 0, "First run ID to compare (default: second most recent)")
	cmd.Flags().Int64Var(&runB, "run-b", 0, "Second run ID to compare (default: most recent)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineDiff(ctx context.Context, deps *PipelineCommandDeps, sourceID int64, runA int64, runB int64, outputFormat string) error {
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

	pipelineClient := pipelinev1.NewPipelineServiceClient(conn)

	req := &pipelinev1.DiffPipelineRunsRequest{
		SourceId: sourceID,
	}

	// Add optional run IDs if provided
	if runA > 0 {
		req.RunIdA = &runA
	}
	if runB > 0 {
		req.RunIdB = &runB
	}

	resp, err := pipelineClient.DiffPipelineRuns(ctx, req)
	if err != nil {
		return fmt.Errorf("comparing pipeline runs: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	return outputPipelineDiffHuman(resp, sourceID)
}

func outputPipelineDiffHuman(resp *pipelinev1.DiffPipelineRunsResponse, sourceID int64) error {
	fmt.Printf("Pipeline Diff for Source %d\n", sourceID)
	fmt.Println("====================================")

	if len(resp.Diffs) == 0 {
		fmt.Println("No differences found between runs.")
		return nil
	}

	fmt.Printf("Found %d differences:\n\n", len(resp.Diffs))
	fmt.Println("STAGE            FIELD                      OLD VALUE                  NEW VALUE                  CHANGE")
	fmt.Println("-----            -----                      ---------                  ---------                  ------")

	for _, diff := range resp.Diffs {
		// Format change type
		changeType := ""
		switch diff.ChangeType {
		case pipelinev1.ChangeType_CHANGE_TYPE_ADDED:
			changeType = "\033[32mADDED\033[0m"
		case pipelinev1.ChangeType_CHANGE_TYPE_REMOVED:
			changeType = "\033[31mREMOVED\033[0m"
		case pipelinev1.ChangeType_CHANGE_TYPE_MODIFIED:
			changeType = "\033[33mMODIFIED\033[0m"
		default:
			changeType = "UNKNOWN"
		}

		// Truncate long values
		oldValue := diff.OldValue
		if len(oldValue) > 26 {
			oldValue = oldValue[:23] + "..."
		}
		newValue := diff.NewValue
		if len(newValue) > 26 {
			newValue = newValue[:23] + "..."
		}

		stage := diff.Stage
		if len(stage) > 16 {
			stage = stage[:13] + "..."
		}

		field := diff.Field
		if len(field) > 26 {
			field = field[:23] + "..."
		}

		fmt.Printf("%-16s %-26s %-26s %-26s %s\n",
			stage, field, oldValue, newValue, changeType)
	}

	fmt.Println()
	return nil
}
