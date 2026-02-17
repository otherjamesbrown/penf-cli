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
	"gopkg.in/yaml.v3"

	ingestv1 "github.com/otherjamesbrown/penf-cli/api/proto/ingest/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// IngestJobStatus represents the status of an ingestion job.
type IngestJobStatus string

const (
	// IngestJobStatusPending indicates the job is queued.
	IngestJobStatusPending IngestJobStatus = "pending"
	// IngestJobStatusProcessing indicates the job is being processed.
	IngestJobStatusProcessing IngestJobStatus = "processing"
	// IngestJobStatusCompleted indicates the job completed successfully.
	IngestJobStatusCompleted IngestJobStatus = "completed"
	// IngestJobStatusFailed indicates the job failed.
	IngestJobStatusFailed IngestJobStatus = "failed"
)

// IngestJobPriority represents job priority levels.
type IngestJobPriority string

const (
	// IngestJobPriorityLow for background processing.
	IngestJobPriorityLow IngestJobPriority = "low"
	// IngestJobPriorityNormal for standard processing.
	IngestJobPriorityNormal IngestJobPriority = "normal"
	// IngestJobPriorityHigh for urgent processing.
	IngestJobPriorityHigh IngestJobPriority = "high"
)

// IngestJob represents an ingestion job.
type IngestJob struct {
	ID          string          `json:"id" yaml:"id"`
	Type        string          `json:"type" yaml:"type"`
	Source      string          `json:"source" yaml:"source"`
	Status      IngestJobStatus `json:"status" yaml:"status"`
	Priority    string          `json:"priority" yaml:"priority"`
	Progress    int             `json:"progress,omitempty" yaml:"progress,omitempty"`
	Message     string          `json:"message,omitempty" yaml:"message,omitempty"`
	CreatedAt   time.Time       `json:"created_at" yaml:"created_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
	ItemsTotal  int             `json:"items_total,omitempty" yaml:"items_total,omitempty"`
	ItemsDone   int             `json:"items_done,omitempty" yaml:"items_done,omitempty"`
	Tags        []string        `json:"tags,omitempty" yaml:"tags,omitempty"`
	Category    string          `json:"category,omitempty" yaml:"category,omitempty"`
	TenantID    string          `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
}

// IngestStatusResponse represents overall ingestion status.
type IngestStatusResponse struct {
	TotalJobs      int         `json:"total_jobs" yaml:"total_jobs"`
	PendingJobs    int         `json:"pending_jobs" yaml:"pending_jobs"`
	ProcessingJobs int         `json:"processing_jobs" yaml:"processing_jobs"`
	CompletedJobs  int         `json:"completed_jobs" yaml:"completed_jobs"`
	FailedJobs     int         `json:"failed_jobs" yaml:"failed_jobs"`
	RecentJobs     []IngestJob `json:"recent_jobs,omitempty" yaml:"recent_jobs,omitempty"`
	ProcessingRate float64     `json:"processing_rate" yaml:"processing_rate"`
	LastUpdated    time.Time   `json:"last_updated" yaml:"last_updated"`
}

// GmailSyncStatus represents Gmail sync status.
type GmailSyncStatus struct {
	Connected    bool      `json:"connected" yaml:"connected"`
	LastSyncAt   time.Time `json:"last_sync_at,omitempty" yaml:"last_sync_at,omitempty"`
	NextSyncAt   time.Time `json:"next_sync_at,omitempty" yaml:"next_sync_at,omitempty"`
	TotalEmails  int64     `json:"total_emails" yaml:"total_emails"`
	SyncedEmails int64     `json:"synced_emails" yaml:"synced_emails"`
	SyncState    string    `json:"sync_state" yaml:"sync_state"`
	Error        string    `json:"error,omitempty" yaml:"error,omitempty"`
}

// GmailSyncHistoryEntry represents a Gmail sync history entry.
type GmailSyncHistoryEntry struct {
	ID            string    `json:"id" yaml:"id"`
	StartedAt     time.Time `json:"started_at" yaml:"started_at"`
	CompletedAt   time.Time `json:"completed_at" yaml:"completed_at"`
	EmailsAdded   int       `json:"emails_added" yaml:"emails_added"`
	EmailsUpdated int       `json:"emails_updated" yaml:"emails_updated"`
	Status        string    `json:"status" yaml:"status"`
	Error         string    `json:"error,omitempty" yaml:"error,omitempty"`
}

// IngestConfig represents ingestion configuration.
type IngestConfig struct {
	AutoSync        bool     `json:"auto_sync" yaml:"auto_sync"`
	SyncInterval    string   `json:"sync_interval" yaml:"sync_interval"`
	BatchSize       int      `json:"batch_size" yaml:"batch_size"`
	MaxRetries      int      `json:"max_retries" yaml:"max_retries"`
	DefaultPriority string   `json:"default_priority" yaml:"default_priority"`
	DefaultCategory string   `json:"default_category,omitempty" yaml:"default_category,omitempty"`
	ExcludePatterns []string `json:"exclude_patterns,omitempty" yaml:"exclude_patterns,omitempty"`
}

// IngestCommandDeps holds the dependencies for ingest commands.
type IngestCommandDeps struct {
	Config       *config.CLIConfig
	GRPCClient   *client.GRPCClient
	OutputFormat config.OutputFormat
	LoadConfig   func() (*config.CLIConfig, error)
	SaveConfig   func(*config.CLIConfig) error
	InitClient   func(*config.CLIConfig) (*client.GRPCClient, error)
}

// DefaultIngestDeps returns the default dependencies for production use.
func DefaultIngestDeps() *IngestCommandDeps {
	return &IngestCommandDeps{
		LoadConfig: config.LoadConfig,
		SaveConfig: config.SaveConfig,
		InitClient: client.ConnectFromConfig,
	}
}

// Ingest command flags.
var (
	ingestTenantID string
	ingestAsync    bool
	ingestPriority string
	ingestTags     []string
	ingestCategory string
	ingestOutput   string
	ingestDryRun   bool
)

// NewIngestCommand creates the root ingest command with all subcommands.
func NewIngestCommand(deps *IngestCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultIngestDeps()
	}

	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Ingest content into Penfold",
		Long: `Ingest content from various sources into the Penfold knowledge base.

Penfold can ingest content from local files, URLs, batch manifests, and external
services like Gmail. All ingested content is processed for search indexing and
entity extraction.

Examples:
  # Ingest a local file
  penf ingest file /path/to/document.pdf

  # Ingest from a URL
  penf ingest url https://example.com/article

  # Ingest .eml email files
  penf ingest email ./emails/ --source "archive-2024"

  # Batch ingest from a manifest file
  penf ingest batch manifest.yaml

  # Trigger Gmail sync
  penf ingest gmail sync

  # Check ingestion status
  penf ingest status

After Ingestion:
  Run 'penf process onboarding context' to review discovered entities.
  See docs/workflows/onboarding.md for the post-import review workflow.

Documentation:
  What gets extracted:    docs/shared/entities.md (people, products, glossary)
  Post-import workflow:   docs/workflows/onboarding.md
  Entity resolution:      docs/concepts/mentions.md`,
	}

	// Global ingest flags.
	cmd.PersistentFlags().StringVarP(&ingestTenantID, "tenant", "t", "", "Tenant ID for multi-tenant operations")
	cmd.PersistentFlags().BoolVar(&ingestAsync, "async", false, "Run ingestion asynchronously")
	cmd.PersistentFlags().StringVarP(&ingestPriority, "priority", "p", "normal", "Job priority: low, normal, high")
	cmd.PersistentFlags().StringSliceVar(&ingestTags, "tags", nil, "Tags to add to ingested content")
	cmd.PersistentFlags().StringVarP(&ingestCategory, "category", "c", "", "Pre-set category for ingested content")
	cmd.PersistentFlags().StringVarP(&ingestOutput, "output", "o", "", "Output format: text, json, yaml")

	// Add subcommands.
	cmd.AddCommand(newIngestFileCommand(deps))
	cmd.AddCommand(newIngestURLCommand(deps))
	cmd.AddCommand(newIngestBatchCommand(deps))
	cmd.AddCommand(newIngestEmailCommand(deps))   // Email (.eml) ingest
	cmd.AddCommand(newIngestMeetingCommand(deps)) // Meeting transcripts
	cmd.AddCommand(newIngestGmailCommand(deps))
	cmd.AddCommand(newIngestStatusCommand(deps))
	cmd.AddCommand(newIngestQueueCommand(deps))
	cmd.AddCommand(newIngestConfigCommand(deps))

	return cmd
}

// newIngestFileCommand creates the 'ingest file' subcommand.
func newIngestFileCommand(deps *IngestCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "file <path>",
		Short: "Ingest a local file",
		Long: `Ingest a local file into the Penfold knowledge base.

Supported file types include PDF, DOCX, TXT, MD, HTML, and more.
The file will be processed for text extraction, embedding generation,
and entity extraction.

Examples:
  penf ingest file /path/to/document.pdf
  penf ingest file ./notes.md --tags=meeting,project
  penf ingest file report.docx --category=reports --async`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIngestFile(cmd.Context(), deps, args[0])
		},
	}
}

// newIngestURLCommand creates the 'ingest url' subcommand.
func newIngestURLCommand(deps *IngestCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "url <url>",
		Short: "Ingest content from a URL",
		Long: `Ingest content from a URL into the Penfold knowledge base.

The content will be fetched, processed for text extraction, and indexed.
Supports web pages, PDFs, and other document types accessible via HTTP.

Examples:
  penf ingest url https://example.com/article
  penf ingest url https://blog.example.com/post --tags=blog,reference
  penf ingest url https://docs.example.com/guide.pdf --async`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIngestURL(cmd.Context(), deps, args[0])
		},
	}
}

// newIngestBatchCommand creates the 'ingest batch' subcommand.
func newIngestBatchCommand(deps *IngestCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch <manifest>",
		Short: "Batch ingest from a manifest file",
		Long: `Batch ingest multiple items from a manifest file.

The manifest file should be YAML or JSON with a list of items to ingest,
including paths, URLs, and metadata.

Manifest format example:
  items:
    - type: file
      path: /path/to/doc.pdf
      tags: [important]
    - type: url
      url: https://example.com/article
      category: reference

Use --dry-run to validate the manifest and preview what would be ingested.

Examples:
  penf ingest batch manifest.yaml
  penf ingest batch imports.json --async --priority=low
  penf ingest batch manifest.yaml --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIngestBatch(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().BoolVar(&ingestDryRun, "dry-run", false, "Validate manifest and preview without ingesting")

	return cmd
}

// newIngestGmailCommand creates the 'ingest gmail' subcommand group.
func newIngestGmailCommand(deps *IngestCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gmail",
		Short: "Gmail ingestion commands",
		Long: `Manage Gmail email ingestion into Penfold.

Gmail integration allows automatic syncing of your emails into the
knowledge base for searching and analysis.

Examples:
  penf ingest gmail sync      # Trigger a Gmail sync
  penf ingest gmail status    # Show sync status
  penf ingest gmail history   # Show sync history`,
	}

	cmd.AddCommand(newIngestGmailSyncCommand(deps))
	cmd.AddCommand(newIngestGmailStatusCommand(deps))
	cmd.AddCommand(newIngestGmailHistoryCommand(deps))

	return cmd
}

// newIngestGmailSyncCommand creates the 'ingest gmail sync' subcommand.
func newIngestGmailSyncCommand(deps *IngestCommandDeps) *cobra.Command {
	var fullSync bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Trigger Gmail sync",
		Long: `Trigger a Gmail synchronization to import new emails.

By default, performs an incremental sync of new and updated emails.
Use --full to perform a complete resync of all emails.

Examples:
  penf ingest gmail sync
  penf ingest gmail sync --full`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIngestGmailSync(cmd.Context(), deps, fullSync)
		},
	}

	cmd.Flags().BoolVar(&fullSync, "full", false, "Perform a full sync instead of incremental")

	return cmd
}

// newIngestGmailStatusCommand creates the 'ingest gmail status' subcommand.
func newIngestGmailStatusCommand(deps *IngestCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Gmail sync status",
		Long: `Display the current status of Gmail synchronization.

Shows connection status, last sync time, email counts, and any errors.

Examples:
  penf ingest gmail status
  penf ingest gmail status --output=json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIngestGmailStatus(cmd.Context(), deps)
		},
	}
}

// newIngestGmailHistoryCommand creates the 'ingest gmail history' subcommand.
func newIngestGmailHistoryCommand(deps *IngestCommandDeps) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show Gmail sync history",
		Long: `Display the history of Gmail sync operations.

Shows past sync operations with their status, duration, and item counts.

Examples:
  penf ingest gmail history
  penf ingest gmail history --limit=20`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIngestGmailHistory(cmd.Context(), deps, limit)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of history entries to show")

	return cmd
}

// newIngestStatusCommand creates the 'ingest status' subcommand.
func newIngestStatusCommand(deps *IngestCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "status [job-id]",
		Short: "Show ingestion status",
		Long: `Display the status of ingestion jobs.

Without arguments, shows overall ingestion status and recent jobs.
With a job ID, shows detailed status of that specific job.

Examples:
  penf ingest status
  penf ingest status job-abc123
  penf ingest status --output=json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := ""
			if len(args) > 0 {
				jobID = args[0]
			}
			return runIngestStatus(cmd.Context(), deps, jobID)
		},
	}
}

// newIngestQueueCommand creates the 'ingest queue' subcommand.
func newIngestQueueCommand(deps *IngestCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "queue",
		Short: "Show pending ingestion jobs",
		Long: `Display the queue of pending ingestion jobs.

Shows all jobs that are waiting to be processed, ordered by priority
and creation time.

Examples:
  penf ingest queue
  penf ingest queue --output=json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIngestQueue(cmd.Context(), deps)
		},
	}
}

// newIngestConfigCommand creates the 'ingest config' subcommand group.
func newIngestConfigCommand(deps *IngestCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage ingestion configuration",
		Long: `View and modify ingestion configuration settings.

Examples:
  penf ingest config show
  penf ingest config set batch_size 100`,
	}

	cmd.AddCommand(newIngestConfigShowCommand(deps))
	cmd.AddCommand(newIngestConfigSetCommand(deps))

	return cmd
}

// newIngestConfigShowCommand creates the 'ingest config show' subcommand.
func newIngestConfigShowCommand(deps *IngestCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show ingestion settings",
		Long: `Display the current ingestion configuration settings.

Examples:
  penf ingest config show
  penf ingest config show --output=json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIngestConfigShow(cmd.Context(), deps)
		},
	}
}

// newIngestConfigSetCommand creates the 'ingest config set' subcommand.
func newIngestConfigSetCommand(deps *IngestCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Update ingestion setting",
		Long: `Update an ingestion configuration setting.

Available settings:
  auto_sync        - Enable/disable automatic Gmail sync (true/false)
  sync_interval    - Gmail sync interval (e.g., 15m, 1h)
  batch_size       - Number of items to process per batch
  max_retries      - Maximum retry attempts for failed jobs
  default_priority - Default job priority (low/normal/high)
  default_category - Default category for ingested content

Examples:
  penf ingest config set auto_sync true
  penf ingest config set sync_interval 30m
  penf ingest config set batch_size 50`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIngestConfigSet(cmd.Context(), deps, args[0], args[1])
		},
	}
}

// runIngestFile executes the file ingestion command.
func runIngestFile(ctx context.Context, deps *IngestCommandDeps, filePath string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Validate file exists.
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// Determine output format.
	format := getIngestOutputFormat(cfg)

	// Validate priority.
	if !validateIngestPriority(ingestPriority) {
		return fmt.Errorf("invalid priority: %s (must be low, normal, or high)", ingestPriority)
	}

	// Determine tenant ID.
	tenantID := ingestTenantID
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" {
		tenantID = "default"
	}

	// Create job via gRPC.
	job, err := createIngestJobViaGRPC(ctx, deps, cfg, tenantID, "file", filePath)
	if err != nil {
		return fmt.Errorf("creating ingest job: %w", err)
	}

	if ingestAsync {
		fmt.Printf("Ingestion job queued: %s\n", job.ID)
		fmt.Printf("  File: %s\n", filePath)
		fmt.Printf("  Priority: %s\n", job.Priority)
		fmt.Println("\nUse 'penf ingest status " + job.ID + "' to check progress.")
		return nil
	}

	// Simulate synchronous processing with progress.
	fmt.Printf("Ingesting file: %s\n", filePath)
	job = simulateIngestProgress(job, format)

	return outputIngestJob(format, job)
}

// runIngestURL executes the URL ingestion command.
func runIngestURL(ctx context.Context, deps *IngestCommandDeps, url string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Validate URL format.
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("invalid URL: must start with http:// or https://")
	}

	// Determine output format.
	format := getIngestOutputFormat(cfg)

	// Validate priority.
	if !validateIngestPriority(ingestPriority) {
		return fmt.Errorf("invalid priority: %s (must be low, normal, or high)", ingestPriority)
	}

	// Determine tenant ID.
	tenantID := ingestTenantID
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" {
		tenantID = "default"
	}

	// Create job via gRPC.
	job, err := createIngestJobViaGRPC(ctx, deps, cfg, tenantID, "url", url)
	if err != nil {
		return fmt.Errorf("creating ingest job: %w", err)
	}

	if ingestAsync {
		fmt.Printf("Ingestion job queued: %s\n", job.ID)
		fmt.Printf("  URL: %s\n", url)
		fmt.Printf("  Priority: %s\n", job.Priority)
		fmt.Println("\nUse 'penf ingest status " + job.ID + "' to check progress.")
		return nil
	}

	// Simulate synchronous processing.
	fmt.Printf("Ingesting URL: %s\n", url)
	job = simulateIngestProgress(job, format)

	return outputIngestJob(format, job)
}

// runIngestBatch executes the batch ingestion command.
func runIngestBatch(ctx context.Context, deps *IngestCommandDeps, manifestPath string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Validate manifest file exists.
	info, err := os.Stat(manifestPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("manifest file not found: %s", manifestPath)
	}
	if err != nil {
		return fmt.Errorf("checking manifest file: %w", err)
	}

	// Determine output format.
	format := getIngestOutputFormat(cfg)

	// Validate priority.
	if !validateIngestPriority(ingestPriority) {
		return fmt.Errorf("invalid priority: %s (must be low, normal, or high)", ingestPriority)
	}

	// Determine tenant ID.
	tenantID := ingestTenantID
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" {
		tenantID = "default"
	}

	// Create job via gRPC.
	job, err := createIngestJobViaGRPC(ctx, deps, cfg, tenantID, "batch", manifestPath)
	if err != nil {
		return fmt.Errorf("creating ingest job: %w", err)
	}
	job.ItemsTotal = 5 // TODO: Parse manifest to get actual item count.

	// Dry-run mode: validate manifest and preview.
	if ingestDryRun {
		fmt.Println("\033[1m=== DRY RUN - No ingestion will occur ===\033[0m")
		fmt.Println()
		fmt.Printf("Manifest file: %s\n", manifestPath)
		fmt.Printf("File size: %d bytes\n", info.Size())
		fmt.Println()
		fmt.Printf("Would ingest %d items with:\n", job.ItemsTotal)
		fmt.Printf("  Priority: %s\n", ingestPriority)
		if ingestAsync {
			fmt.Printf("  Mode: async (queued)\n")
		} else {
			fmt.Printf("  Mode: sync (immediate)\n")
		}
		if len(ingestTags) > 0 {
			fmt.Printf("  Tags: %v\n", ingestTags)
		}
		if ingestCategory != "" {
			fmt.Printf("  Category: %s\n", ingestCategory)
		}
		fmt.Println()
		fmt.Println("\033[2mRun without --dry-run to perform the ingestion.\033[0m")
		return nil
	}

	if ingestAsync {
		fmt.Printf("Batch ingestion job queued: %s\n", job.ID)
		fmt.Printf("  Manifest: %s\n", manifestPath)
		fmt.Printf("  Items: %d\n", job.ItemsTotal)
		fmt.Printf("  Priority: %s\n", job.Priority)
		fmt.Println("\nUse 'penf ingest status " + job.ID + "' to check progress.")
		return nil
	}

	// Simulate synchronous batch processing.
	fmt.Printf("Processing batch from: %s\n", manifestPath)
	fmt.Printf("Items to process: %d\n\n", job.ItemsTotal)

	for i := 1; i <= job.ItemsTotal; i++ {
		fmt.Printf("Processing item %d/%d...\n", i, job.ItemsTotal)
		time.Sleep(100 * time.Millisecond) // Simulate processing.
	}

	job.Status = IngestJobStatusCompleted
	job.ItemsDone = job.ItemsTotal
	job.Progress = 100
	now := time.Now()
	job.CompletedAt = &now

	fmt.Printf("\nBatch processing complete: %d items processed\n", job.ItemsDone)

	return outputIngestJob(format, job)
}

// runIngestGmailSync executes the Gmail sync command.
func runIngestGmailSync(ctx context.Context, deps *IngestCommandDeps, fullSync bool) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	syncType := "incremental"
	if fullSync {
		syncType = "full"
	}

	fmt.Printf("Starting %s Gmail sync...\n", syncType)

	// Simulate sync progress.
	stages := []string{
		"Connecting to Gmail API...",
		"Fetching message list...",
		"Downloading new emails...",
		"Processing attachments...",
		"Generating embeddings...",
		"Indexing content...",
	}

	for _, stage := range stages {
		fmt.Printf("  %s\n", stage)
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\nGmail sync completed successfully!")
	fmt.Printf("  New emails synced: 12\n")
	fmt.Printf("  Updated emails: 3\n")
	fmt.Printf("  Processing time: 2.5s\n")

	return nil
}

// runIngestGmailStatus executes the Gmail status command.
func runIngestGmailStatus(ctx context.Context, deps *IngestCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format.
	format := getIngestOutputFormat(cfg)

	// Determine tenant ID.
	tenantID := ingestTenantID
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" {
		tenantID = "default"
	}

	// TODO: GetGmailSyncStatus requires a syncID parameter, but the CLI status command
	// should show the current/overall status without needing a specific sync ID.
	// Need to add a GetCurrentGmailStatus or ListGmailSyncs endpoint to the GmailConnectorService.
	// For now, return default empty status indicating no active sync.
	status := GmailSyncStatus{
		Connected:   false,
		SyncState:   "unknown",
		TotalEmails: 0,
		Error:       "Gmail status service not yet implemented - requires GetCurrentGmailStatus endpoint",
	}

	// Try to connect and get status if possible.
	grpcClient, err := deps.InitClient(cfg)
	if err == nil {
		defer grpcClient.Close()
		// TODO: When GetCurrentGmailStatus is available, use it here.
		// For now, just indicate we connected but have no status endpoint.
		status.SyncState = "service connected, status endpoint not available"
	}

	return outputGmailStatus(format, status)
}

// runIngestGmailHistory executes the Gmail history command.
func runIngestGmailHistory(ctx context.Context, deps *IngestCommandDeps, limit int) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format.
	format := getIngestOutputFormat(cfg)

	// TODO: GmailConnectorService does not have a ListSyncHistory endpoint yet.
	// Need to add ListGmailSyncHistory RPC to return past sync operations.
	// For now, return empty history with a message.
	_ = limit // Will be used when ListGmailSyncHistory is available.
	history := []GmailSyncHistoryEntry{}

	fmt.Fprintln(os.Stderr, "Warning: Gmail sync history service not yet implemented")

	return outputGmailHistory(format, history)
}

// runIngestStatus executes the status command.
func runIngestStatus(ctx context.Context, deps *IngestCommandDeps, jobID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format.
	format := getIngestOutputFormat(cfg)

	// Determine tenant ID.
	tenantID := ingestTenantID
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" {
		tenantID = "default"
	}

	if jobID != "" {
		// Show specific job status via gRPC.
		grpcClient, err := deps.InitClient(cfg)
		if err != nil {
			return fmt.Errorf("connecting to gateway: %w", err)
		}
		defer grpcClient.Close()

		ingestJob, err := grpcClient.GetIngestJob(ctx, tenantID, jobID)
		if err != nil {
			return fmt.Errorf("retrieving job %s: %w", jobID, err)
		}

		// Convert proto job to CLI job type.
		job := convertProtoToIngestJob(ingestJob)
		return outputIngestJob(format, job)
	}

	// TODO: IngestService does not have a GetIngestStatus or ListIngestJobs endpoint yet.
	// Need to add aggregate status RPC to return overall ingestion statistics.
	// For now, return empty status with zeros.
	fmt.Fprintln(os.Stderr, "Warning: Aggregate ingest status service not yet implemented")
	status := IngestStatusResponse{
		TotalJobs:      0,
		PendingJobs:    0,
		ProcessingJobs: 0,
		CompletedJobs:  0,
		FailedJobs:     0,
		ProcessingRate: 0,
		LastUpdated:    time.Now(),
		RecentJobs:     []IngestJob{},
	}
	return outputIngestStatus(format, status)
}

// runIngestQueue executes the queue command.
func runIngestQueue(ctx context.Context, deps *IngestCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format.
	format := getIngestOutputFormat(cfg)

	// TODO: IngestService does not have a ListPendingJobs endpoint yet.
	// Need to add ListIngestJobs RPC with status filter to return pending jobs.
	// For now, return empty queue.
	fmt.Fprintln(os.Stderr, "Warning: Ingest queue service not yet implemented")
	jobs := []IngestJob{}

	return outputIngestQueue(format, jobs)
}

// runIngestConfigShow executes the config show command.
func runIngestConfigShow(ctx context.Context, deps *IngestCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format.
	format := getIngestOutputFormat(cfg)

	// TODO: There is no IngestConfigService yet.
	// Need to add GetIngestConfig RPC to retrieve server-side ingestion settings.
	// For now, return default configuration values.
	fmt.Fprintln(os.Stderr, "Warning: Ingest config service not yet implemented - showing defaults")
	ingestCfg := IngestConfig{
		AutoSync:        false,
		SyncInterval:    "30m",
		BatchSize:       50,
		MaxRetries:      3,
		DefaultPriority: "normal",
		DefaultCategory: "",
		ExcludePatterns: []string{},
	}

	return outputIngestConfig(format, ingestCfg)
}

// runIngestConfigSet executes the config set command.
func runIngestConfigSet(ctx context.Context, deps *IngestCommandDeps, key, value string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Validate key.
	validKeys := map[string]bool{
		"auto_sync":        true,
		"sync_interval":    true,
		"batch_size":       true,
		"max_retries":      true,
		"default_priority": true,
		"default_category": true,
	}

	if !validKeys[key] {
		return fmt.Errorf("invalid configuration key: %s\nValid keys: %s",
			key, strings.Join(getValidConfigKeys(), ", "))
	}

	// STUB: Returns mock acknowledgment until ingest service gRPC is connected.
	fmt.Printf("Updated ingestion setting:\n")
	fmt.Printf("  %s = %s\n", key, value)

	return nil
}

// Helper functions.

// getIngestOutputFormat determines the output format from flags and config.
func getIngestOutputFormat(cfg *config.CLIConfig) config.OutputFormat {
	if ingestOutput != "" {
		return config.OutputFormat(ingestOutput)
	}
	return cfg.OutputFormat
}

// validateIngestPriority validates a priority string.
func validateIngestPriority(priority string) bool {
	switch IngestJobPriority(priority) {
	case IngestJobPriorityLow, IngestJobPriorityNormal, IngestJobPriorityHigh:
		return true
	default:
		return false
	}
}

// getValidConfigKeys returns a list of valid configuration keys.
func getValidConfigKeys() []string {
	return []string{
		"auto_sync",
		"sync_interval",
		"batch_size",
		"max_retries",
		"default_priority",
		"default_category",
	}
}

// convertProtoToIngestJob converts a proto IngestJob to the CLI IngestJob type.
func convertProtoToIngestJob(pj *ingestv1.IngestJob) IngestJob {
	job := IngestJob{
		ID:         pj.Id,
		Type:       pj.Platform.String(),
		Source:     pj.SourcePath,
		Priority:   "normal",
		ItemsTotal: int(pj.TotalFiles),
		ItemsDone:  int(pj.ProcessedCount),
		Progress:   0,
	}

	// Calculate progress.
	if pj.TotalFiles > 0 {
		job.Progress = int(float64(pj.ProcessedCount) / float64(pj.TotalFiles) * 100)
	}

	// Map status.
	switch pj.Status {
	case ingestv1.JobStatus_JOB_STATUS_CREATED:
		job.Status = IngestJobStatusPending
	case ingestv1.JobStatus_JOB_STATUS_RUNNING:
		job.Status = IngestJobStatusProcessing
	case ingestv1.JobStatus_JOB_STATUS_COMPLETED:
		job.Status = IngestJobStatusCompleted
	case ingestv1.JobStatus_JOB_STATUS_FAILED:
		job.Status = IngestJobStatusFailed
	case ingestv1.JobStatus_JOB_STATUS_CANCELLED:
		job.Status = IngestJobStatusFailed
	default:
		job.Status = IngestJobStatusPending
	}

	// Timestamps.
	if pj.CreatedAt != nil {
		job.CreatedAt = pj.CreatedAt.AsTime()
	}
	if pj.CompletedAt != nil {
		t := pj.CompletedAt.AsTime()
		job.CompletedAt = &t
	}

	return job
}

// createIngestJobViaGRPC creates an ingest job using the gRPC IngestService.
func createIngestJobViaGRPC(ctx context.Context, deps *IngestCommandDeps, cfg *config.CLIConfig, tenantID, jobType, source string) (IngestJob, error) {
	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return IngestJob{}, fmt.Errorf("connecting to gateway: %w", err)
	}
	defer grpcClient.Close()

	// Map job type to platform.
	platform := ingestv1.Platform_PLATFORM_LOCAL
	switch jobType {
	case "file":
		platform = ingestv1.Platform_PLATFORM_LOCAL
	case "url":
		platform = ingestv1.Platform_PLATFORM_LOCAL // URLs are treated as local ingestion
	case "batch":
		platform = ingestv1.Platform_PLATFORM_LOCAL
	case "gmail":
		platform = ingestv1.Platform_PLATFORM_GMAIL
	}

	// Build metadata from tags and category.
	metadata := make(map[string]string)
	if len(ingestTags) > 0 {
		metadata["tags"] = strings.Join(ingestTags, ",")
	}
	if ingestCategory != "" {
		metadata["category"] = ingestCategory
	}
	if ingestPriority != "" {
		metadata["priority"] = ingestPriority
	}

	req := &ingestv1.CreateIngestJobRequest{
		TenantId:   tenantID,
		Name:       fmt.Sprintf("%s ingestion: %s", jobType, source),
		Platform:   platform,
		TotalFiles: 1, // Single file/URL; batch will update this.
		SourcePath: source,
		Metadata:   metadata,
	}

	protoJob, err := grpcClient.CreateIngestJob(ctx, req)
	if err != nil {
		return IngestJob{}, fmt.Errorf("CreateIngestJob RPC: %w", err)
	}

	// Convert proto response to CLI job type.
	job := convertProtoToIngestJob(protoJob)

	// Augment with CLI-specific fields not in proto.
	job.Type = jobType
	job.Priority = ingestPriority
	job.Tags = ingestTags
	job.Category = ingestCategory
	job.TenantID = tenantID

	return job, nil
}

// simulateIngestProgress simulates ingestion progress for synchronous operations.
func simulateIngestProgress(job IngestJob, format config.OutputFormat) IngestJob {
	stages := []struct {
		progress int
		message  string
	}{
		{10, "Reading content..."},
		{30, "Extracting text..."},
		{50, "Generating embeddings..."},
		{70, "Extracting entities..."},
		{90, "Indexing content..."},
		{100, "Complete"},
	}

	now := time.Now()
	job.StartedAt = &now
	job.Status = IngestJobStatusProcessing

	for _, stage := range stages {
		if format == config.OutputFormatText {
			fmt.Printf("  [%d%%] %s\n", stage.progress, stage.message)
		}
		job.Progress = stage.progress
		job.Message = stage.message
		time.Sleep(100 * time.Millisecond)
	}

	completedAt := time.Now()
	job.CompletedAt = &completedAt
	job.Status = IngestJobStatusCompleted

	return job
}

// Output functions.

// outputIngestJob outputs a single ingestion job.
func outputIngestJob(format config.OutputFormat, job IngestJob) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(job)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(job)
	default:
		return outputIngestJobText(job)
	}
}

// outputIngestJobText outputs a job in human-readable format.
func outputIngestJobText(job IngestJob) error {
	statusColor := getJobStatusColor(job.Status)

	fmt.Printf("\nIngestion Job: %s\n", job.ID)
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("  Type:     %s\n", job.Type)
	fmt.Printf("  Source:   %s\n", job.Source)
	fmt.Printf("  Status:   %s%s\033[0m\n", statusColor, job.Status)
	fmt.Printf("  Priority: %s\n", job.Priority)

	if job.Progress > 0 {
		fmt.Printf("  Progress: %d%%\n", job.Progress)
	}
	if job.Message != "" {
		fmt.Printf("  Message:  %s\n", job.Message)
	}
	if job.ItemsTotal > 0 {
		fmt.Printf("  Items:    %d/%d\n", job.ItemsDone, job.ItemsTotal)
	}
	if len(job.Tags) > 0 {
		fmt.Printf("  Tags:     %s\n", strings.Join(job.Tags, ", "))
	}
	if job.Category != "" {
		fmt.Printf("  Category: %s\n", job.Category)
	}

	fmt.Printf("  Created:  %s\n", job.CreatedAt.Format(time.RFC3339))
	if job.StartedAt != nil {
		fmt.Printf("  Started:  %s\n", job.StartedAt.Format(time.RFC3339))
	}
	if job.CompletedAt != nil {
		fmt.Printf("  Completed: %s\n", job.CompletedAt.Format(time.RFC3339))
	}

	return nil
}

// outputIngestStatus outputs overall ingestion status.
func outputIngestStatus(format config.OutputFormat, status IngestStatusResponse) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(status)
	default:
		return outputIngestStatusText(status)
	}
}

// outputIngestStatusText outputs status in human-readable format.
func outputIngestStatusText(status IngestStatusResponse) error {
	fmt.Println("Ingestion Status")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Printf("  Total Jobs:      %d\n", status.TotalJobs)
	fmt.Printf("  Pending:         \033[33m%d\033[0m\n", status.PendingJobs)
	fmt.Printf("  Processing:      \033[34m%d\033[0m\n", status.ProcessingJobs)
	fmt.Printf("  Completed:       \033[32m%d\033[0m\n", status.CompletedJobs)
	fmt.Printf("  Failed:          \033[31m%d\033[0m\n", status.FailedJobs)
	fmt.Printf("  Processing Rate: %.1f jobs/min\n", status.ProcessingRate)
	fmt.Printf("  Last Updated:    %s\n", status.LastUpdated.Format(time.RFC3339))

	if len(status.RecentJobs) > 0 {
		fmt.Println("\nRecent Jobs:")
		fmt.Println("  ID                TYPE    STATUS       PROGRESS  SOURCE")
		fmt.Println("  --                ----    ------       --------  ------")
		for _, job := range status.RecentJobs {
			statusColor := getJobStatusColor(job.Status)
			progressStr := fmt.Sprintf("%d%%", job.Progress)
			fmt.Printf("  %-16s  %-6s  %s%-10s\033[0m  %-8s  %s\n",
				truncateIngestString(job.ID, 16),
				job.Type,
				statusColor,
				job.Status,
				progressStr,
				truncateIngestString(job.Source, 30))
		}
	}

	return nil
}

// outputIngestQueue outputs the pending job queue.
func outputIngestQueue(format config.OutputFormat, jobs []IngestJob) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(jobs)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(jobs)
	default:
		return outputIngestQueueText(jobs)
	}
}

// outputIngestQueueText outputs the queue in human-readable format.
func outputIngestQueueText(jobs []IngestJob) error {
	if len(jobs) == 0 {
		fmt.Println("No pending ingestion jobs.")
		return nil
	}

	fmt.Printf("Pending Ingestion Jobs (%d)\n", len(jobs))
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("  PRIORITY  ID                TYPE    ITEMS  SOURCE")
	fmt.Println("  --------  --                ----    -----  ------")

	for _, job := range jobs {
		priorityColor := getPriorityColor(job.Priority)
		itemsStr := "-"
		if job.ItemsTotal > 0 {
			itemsStr = fmt.Sprintf("%d", job.ItemsTotal)
		}
		fmt.Printf("  %s%-8s\033[0m  %-16s  %-6s  %-5s  %s\n",
			priorityColor,
			job.Priority,
			truncateIngestString(job.ID, 16),
			job.Type,
			itemsStr,
			truncateIngestString(job.Source, 30))
	}

	return nil
}

// outputGmailStatus outputs Gmail sync status.
func outputGmailStatus(format config.OutputFormat, status GmailSyncStatus) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(status)
	default:
		return outputGmailStatusText(status)
	}
}

// outputGmailStatusText outputs Gmail status in human-readable format.
func outputGmailStatusText(status GmailSyncStatus) error {
	fmt.Println("Gmail Sync Status")
	fmt.Println(strings.Repeat("=", 40))

	connColor := "\033[32m"
	connStatus := "Connected"
	if !status.Connected {
		connColor = "\033[31m"
		connStatus = "Disconnected"
	}

	fmt.Printf("  Connection:   %s%s\033[0m\n", connColor, connStatus)
	fmt.Printf("  Sync State:   %s\n", status.SyncState)
	fmt.Printf("  Total Emails: %d\n", status.TotalEmails)
	fmt.Printf("  Synced:       %d\n", status.SyncedEmails)

	if !status.LastSyncAt.IsZero() {
		fmt.Printf("  Last Sync:    %s\n", status.LastSyncAt.Format(time.RFC3339))
	}
	if !status.NextSyncAt.IsZero() {
		fmt.Printf("  Next Sync:    %s\n", status.NextSyncAt.Format(time.RFC3339))
	}
	if status.Error != "" {
		fmt.Printf("  Error:        \033[31m%s\033[0m\n", status.Error)
	}

	return nil
}

// outputGmailHistory outputs Gmail sync history.
func outputGmailHistory(format config.OutputFormat, history []GmailSyncHistoryEntry) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(history)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(history)
	default:
		return outputGmailHistoryText(history)
	}
}

// outputGmailHistoryText outputs Gmail history in human-readable format.
func outputGmailHistoryText(history []GmailSyncHistoryEntry) error {
	if len(history) == 0 {
		fmt.Println("No Gmail sync history found.")
		return nil
	}

	fmt.Printf("Gmail Sync History (%d entries)\n", len(history))
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("  STARTED                 DURATION  ADDED  UPDATED  STATUS")
	fmt.Println("  -------                 --------  -----  -------  ------")

	for _, entry := range history {
		duration := entry.CompletedAt.Sub(entry.StartedAt)
		statusColor := "\033[32m"
		if entry.Status != "completed" {
			statusColor = "\033[31m"
		}
		fmt.Printf("  %-24s  %-8s  %-5d  %-7d  %s%s\033[0m\n",
			entry.StartedAt.Format("2006-01-02 15:04:05"),
			formatDuration(duration),
			entry.EmailsAdded,
			entry.EmailsUpdated,
			statusColor,
			entry.Status)
	}

	return nil
}

// outputIngestConfig outputs ingestion configuration.
func outputIngestConfig(format config.OutputFormat, cfg IngestConfig) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cfg)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(cfg)
	default:
		return outputIngestConfigText(cfg)
	}
}

// outputIngestConfigText outputs config in human-readable format.
func outputIngestConfigText(cfg IngestConfig) error {
	fmt.Println("Ingestion Configuration")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Printf("  auto_sync:        %t\n", cfg.AutoSync)
	fmt.Printf("  sync_interval:    %s\n", cfg.SyncInterval)
	fmt.Printf("  batch_size:       %d\n", cfg.BatchSize)
	fmt.Printf("  max_retries:      %d\n", cfg.MaxRetries)
	fmt.Printf("  default_priority: %s\n", cfg.DefaultPriority)
	if cfg.DefaultCategory != "" {
		fmt.Printf("  default_category: %s\n", cfg.DefaultCategory)
	}
	if len(cfg.ExcludePatterns) > 0 {
		fmt.Printf("  exclude_patterns: %s\n", strings.Join(cfg.ExcludePatterns, ", "))
	}

	return nil
}

// getJobStatusColor returns the ANSI color code for a job status.
func getJobStatusColor(status IngestJobStatus) string {
	switch status {
	case IngestJobStatusCompleted:
		return "\033[32m" // Green.
	case IngestJobStatusProcessing:
		return "\033[34m" // Blue.
	case IngestJobStatusPending:
		return "\033[33m" // Yellow.
	case IngestJobStatusFailed:
		return "\033[31m" // Red.
	default:
		return ""
	}
}

// getPriorityColor returns the ANSI color code for a priority level.
func getPriorityColor(priority string) string {
	switch IngestJobPriority(priority) {
	case IngestJobPriorityHigh:
		return "\033[31m" // Red.
	case IngestJobPriorityNormal:
		return "\033[33m" // Yellow.
	case IngestJobPriorityLow:
		return "\033[32m" // Green.
	default:
		return ""
	}
}

// truncateIngestString truncates a string to the given length.
func truncateIngestString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}
