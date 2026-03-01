package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	ingestv1 "github.com/otherjamesbrown/penf-cli/api/proto/ingest/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
	"github.com/otherjamesbrown/penf-cli/pkg/contentid"
	"github.com/otherjamesbrown/penf-cli/pkg/ingest/eml"
)

// Email ingest specific flags
var (
	emailSource      string
	emailLabels      []string
	emailConcurrency int
	emailDryRun      bool
	emailResumeJob   string
)

// emailIngestResult tracks the result of email ingestion.
type emailIngestResult struct {
	JobID         string
	TotalFiles    int
	ImportedCount int
	SkippedCount  int
	FailedCount   int
	StartedAt     time.Time
	CompletedAt   time.Time
	Success       bool
	Errors        []emailFileError
	ContentIDs    []string // Content IDs of successfully ingested emails
}

// emailFileError records an error for a specific file.
type emailFileError struct {
	FilePath string
	Error    string
}

// emailProgress tracks progress of email ingestion.
type emailProgress struct {
	mu             sync.RWMutex
	TotalFiles     int
	ProcessedCount int
	ImportedCount  int
	SkippedCount   int
	FailedCount    int
	CurrentFile    string
	StartedAt      time.Time
}

func newEmailProgress(totalFiles int) *emailProgress {
	return &emailProgress{
		TotalFiles: totalFiles,
		StartedAt:  time.Now(),
	}
}

func (p *emailProgress) snapshot() emailProgressSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	elapsed := time.Since(p.StartedAt).Seconds()
	var estimatedRemaining *float64
	if p.ProcessedCount > 0 {
		remaining := p.TotalFiles - p.ProcessedCount
		rate := elapsed / float64(p.ProcessedCount)
		est := rate * float64(remaining)
		estimatedRemaining = &est
	}

	return emailProgressSnapshot{
		TotalFiles:                p.TotalFiles,
		ProcessedCount:            p.ProcessedCount,
		ImportedCount:             p.ImportedCount,
		SkippedCount:              p.SkippedCount,
		FailedCount:               p.FailedCount,
		EstimatedRemainingSeconds: estimatedRemaining,
	}
}

func (p *emailProgress) recordImported() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ImportedCount++
	p.ProcessedCount++
}

func (p *emailProgress) recordSkipped() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.SkippedCount++
	p.ProcessedCount++
}

func (p *emailProgress) recordFailed() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.FailedCount++
	p.ProcessedCount++
}

func (p *emailProgress) setCurrentFile(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.CurrentFile = path
}

type emailProgressSnapshot struct {
	TotalFiles                int
	ProcessedCount            int
	ImportedCount             int
	SkippedCount              int
	FailedCount               int
	EstimatedRemainingSeconds *float64
}

func (s emailProgressSnapshot) percentComplete() float64 {
	if s.TotalFiles == 0 {
		return 0
	}
	return float64(s.ProcessedCount) / float64(s.TotalFiles) * 100
}

// newIngestEmailCommand creates the 'ingest email' subcommand.
func newIngestEmailCommand(deps *IngestCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "email <path>",
		Short: "Ingest .eml files into Penfold",
		Long: `Ingest RFC 5322 email files (.eml) into the Penfold knowledge base.

Parses email headers, body content, and attachment metadata. Each email
is stored in the database and triggers AI processing for embeddings,
entity extraction, and search indexing.

Supports single files, directories (recursive), and glob patterns.
Duplicate detection is performed by message-id and content hash.

Examples:
  # Ingest a single email
  penf ingest email message.eml --source "archive"

  # Ingest a directory recursively
  penf ingest email ./emails/ --source "outlook-2024"

  # Ingest with labels and custom concurrency
  penf ingest email ./backup/ --source "backup" --labels "project-a,important" --concurrency 8

  # Preview without importing (dry run)
  penf ingest email ./emails/ --source "test" --dry-run

  # Resume an interrupted job
  penf ingest email ./emails/ --source "backup" --resume job-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIngestEmail(cmd.Context(), deps, args[0])
		},
	}

	// Email-specific flags
	cmd.Flags().StringVarP(&emailSource, "source", "s", "", "Source tag identifier (required)")
	cmd.Flags().StringSliceVarP(&emailLabels, "labels", "l", nil, "Comma-separated labels to apply")
	cmd.Flags().IntVarP(&emailConcurrency, "concurrency", "w", 4, "Number of concurrent workers")
	cmd.Flags().BoolVar(&emailDryRun, "dry-run", false, "Preview import without persisting")
	cmd.Flags().StringVar(&emailResumeJob, "resume", "", "Resume an interrupted job by ID")

	cmd.MarkFlagRequired("source")

	return cmd
}

// runIngestEmail executes the email ingestion command.
func runIngestEmail(ctx context.Context, deps *IngestCommandDeps, path string) error {
	// Load configuration
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Validate path exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("path not found: %s", path)
	}
	if err != nil {
		return fmt.Errorf("accessing path: %w", err)
	}

	// Validate source is provided
	if emailSource == "" {
		return fmt.Errorf("--source flag is required")
	}

	// Determine tenant ID
	tenantID := ingestTenantID
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" {
		tenantID = "default" // Fallback for single-tenant
	}

	// Get output format
	format := getIngestOutputFormat(cfg)

	// Display startup message
	fmt.Printf("Email Ingest: %s\n", path)
	fmt.Printf("  Source:      %s\n", emailSource)
	fmt.Printf("  Tenant:      %s\n", tenantID)
	fmt.Printf("  Concurrency: %d workers\n", emailConcurrency)
	if len(emailLabels) > 0 {
		fmt.Printf("  Labels:      %s\n", strings.Join(emailLabels, ", "))
	}
	if emailDryRun {
		fmt.Printf("  Mode:        DRY RUN (no changes will be made)\n")
	}
	if emailResumeJob != "" {
		fmt.Printf("  Resuming:    %s\n", emailResumeJob)
	}
	if info.IsDir() {
		fmt.Printf("  Path type:   directory (recursive)\n")
	} else {
		fmt.Printf("  Path type:   single file\n")
	}
	fmt.Println()

	// Discover files locally (CLI keeps file discovery)
	files, err := discoverEmailFiles(path)
	if err != nil {
		return fmt.Errorf("discovering files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No .eml files found.")
		return nil
	}

	fmt.Printf("Found %d .eml files\n\n", len(files))

	// Create email parser (CLI keeps parsing)
	parseOpts := eml.DefaultParseOptions()
	parseOpts.IncludeAttachmentContent = false // Only need metadata for gRPC
	parser := eml.NewParser(parseOpts)

	// For dry-run mode, just parse and display without calling gRPC
	if emailDryRun {
		return runEmailDryRun(ctx, parser, files, format)
	}

	// Connect to gateway for gRPC operations
	conn, err := connectIngestToGateway(cfg)
	if err != nil {
		return fmt.Errorf("connecting to gateway: %w", err)
	}
	defer conn.Close()

	client := ingestv1.NewIngestServiceClient(conn)

	// Create or resume job via gRPC
	jobID := emailResumeJob
	if jobID == "" {
		// Create job record via gRPC
		resp, err := client.CreateIngestJob(ctx, &ingestv1.CreateIngestJobRequest{
			TenantId:   tenantID,
			Name:       fmt.Sprintf("Email ingest: %s", emailSource),
			Platform:   ingestv1.Platform_PLATFORM_LOCAL,
			TotalFiles: int64(len(files)),
			SourcePath: path,
			Metadata: map[string]string{
				"source_tag": emailSource,
				"labels":     strings.Join(emailLabels, ","),
			},
		})
		if err != nil {
			return fmt.Errorf("creating ingest job: %w", err)
		}
		// Use the job ID returned by the gateway
		jobID = resp.Job.Id
	}

	// Initialize progress tracking
	progress := newEmailProgress(len(files))
	result := &emailIngestResult{
		JobID:      jobID,
		TotalFiles: len(files),
		StartedAt:  time.Now(),
		Errors:     []emailFileError{},
	}

	// Process files
	if emailConcurrency == 1 {
		processEmailsSequential(ctx, client, parser, tenantID, jobID, files, progress, result, format)
	} else {
		processEmailsParallel(ctx, client, parser, tenantID, jobID, files, progress, result, format)
	}

	result.CompletedAt = time.Now()
	result.ImportedCount = progress.ImportedCount
	result.SkippedCount = progress.SkippedCount
	result.FailedCount = progress.FailedCount
	result.Success = result.FailedCount == 0

	// Complete job via gRPC
	_, err = client.CompleteIngestJob(ctx, &ingestv1.CompleteIngestJobRequest{
		JobId:        jobID,
		Success:      result.Success,
		ErrorMessage: "",
	})
	if err != nil {
		// Log warning but don't fail - files are already ingested
		fmt.Fprintf(os.Stderr, "Warning: failed to complete job: %v\n", err)
	}

	// Display results
	fmt.Println()
	displayEmailResults(result, format)

	// Return error if there were failures
	if result.FailedCount > 0 {
		return fmt.Errorf("%d files failed to import", result.FailedCount)
	}

	return nil
}

// connectIngestToGateway creates a gRPC connection to the gateway service.
func connectIngestToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	// Configure transport credentials based on TLS settings.
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

// discoverEmailFiles finds all .eml files at the given path.
func discoverEmailFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		// Single file
		if strings.HasSuffix(strings.ToLower(path), ".eml") {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return nil, err
			}
			return []string{absPath}, nil
		}
		return nil, fmt.Errorf("file is not an .eml file: %s", path)
	}

	// Directory - walk recursively
	var files []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".eml") {
			absPath, err := filepath.Abs(p)
			if err != nil {
				return err
			}
			files = append(files, absPath)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// runEmailDryRun processes files in dry-run mode (parse only, no gRPC calls).
func runEmailDryRun(ctx context.Context, parser *eml.Parser, files []string, format config.OutputFormat) error {
	progress := newEmailProgress(len(files))
	result := &emailIngestResult{
		JobID:      "dry-run",
		TotalFiles: len(files),
		StartedAt:  time.Now(),
		Errors:     []emailFileError{},
	}

	for _, file := range files {
		if ctx.Err() != nil {
			break
		}

		progress.setCurrentFile(file)

		// Parse the email
		parseResult, err := parser.ParseFile(file)
		if err != nil {
			result.Errors = append(result.Errors, emailFileError{
				FilePath: file,
				Error:    err.Error(),
			})
			progress.recordFailed()
			continue
		}

		email := parseResult.Email
		fmt.Printf("  [DRY RUN] Would import: %s\n", filepath.Base(file))
		fmt.Printf("            Subject: %s\n", truncateIngestString(email.Subject, 60))
		fmt.Printf("            From: %s\n", email.From.Email)
		fmt.Printf("            Date: %s\n", email.Date.Format(time.RFC3339))
		if email.HasAttachments() {
			fmt.Printf("            Attachments: %d\n", email.AttachmentCount())
		}
		fmt.Println()

		progress.recordImported()
	}

	result.CompletedAt = time.Now()
	result.ImportedCount = progress.ImportedCount
	result.SkippedCount = progress.SkippedCount
	result.FailedCount = progress.FailedCount
	result.Success = result.FailedCount == 0

	fmt.Println()
	fmt.Println("=== DRY RUN COMPLETE ===")
	displayEmailResults(result, format)

	return nil
}

// processEmailsSequential processes files one at a time.
func processEmailsSequential(
	ctx context.Context,
	client ingestv1.IngestServiceClient,
	parser *eml.Parser,
	tenantID, jobID string,
	files []string,
	progress *emailProgress,
	result *emailIngestResult,
	format config.OutputFormat,
) {
	for _, file := range files {
		if ctx.Err() != nil {
			return
		}

		progress.setCurrentFile(file)
		outcome := processEmailFile(ctx, client, parser, tenantID, jobID, file)
		recordEmailOutcome(ctx, client, jobID, file, outcome, progress, result)
		displayEmailProgress(progress, format)
	}
}

// processEmailsParallel processes files using a worker pool.
func processEmailsParallel(
	ctx context.Context,
	client ingestv1.IngestServiceClient,
	parser *eml.Parser,
	tenantID, jobID string,
	files []string,
	progress *emailProgress,
	result *emailIngestResult,
	format config.OutputFormat,
) {
	filesCh := make(chan string, len(files))
	resultsCh := make(chan emailFileOutcome, len(files))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < emailConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range filesCh {
				if ctx.Err() != nil {
					resultsCh <- emailFileOutcome{file: file, outcome: emailOutcome{status: "skipped"}}
					continue
				}
				progress.setCurrentFile(file)
				outcome := processEmailFile(ctx, client, parser, tenantID, jobID, file)
				resultsCh <- emailFileOutcome{file: file, outcome: outcome}
			}
		}()
	}

	// Send files to workers
	for _, file := range files {
		filesCh <- file
	}
	close(filesCh)

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// Collect results
	for fo := range resultsCh {
		recordEmailOutcome(ctx, client, jobID, fo.file, fo.outcome, progress, result)
		displayEmailProgress(progress, format)
	}
}

type emailFileOutcome struct {
	file    string
	outcome emailOutcome
}

type emailOutcome struct {
	status    string // "imported", "skipped", "failed"
	contentID string
	sourceID  string
	err       error
}

// processEmailFile processes a single file and returns the outcome.
func processEmailFile(
	ctx context.Context,
	client ingestv1.IngestServiceClient,
	parser *eml.Parser,
	tenantID, jobID, filePath string,
) emailOutcome {
	// Parse the email locally (CLI keeps parsing)
	parseResult, err := parser.ParseFile(filePath)
	if err != nil {
		return emailOutcome{status: "failed", err: fmt.Errorf("parsing: %w", err)}
	}

	email := parseResult.Email

	// Generate content_id before sending to gateway
	cid := contentid.New(contentid.TypeEmail)

	// Convert to proto request and send via gRPC
	req := emailToProtoRequest(email, tenantID, emailSource, emailLabels, jobID, cid)

	resp, err := client.IngestEmail(ctx, req)
	if err != nil {
		return emailOutcome{status: "failed", err: fmt.Errorf("ingesting: %w", err)}
	}

	if resp.WasDuplicate {
		return emailOutcome{status: "skipped", contentID: resp.ContentId, sourceID: resp.ExistingSourceId}
	}

	return emailOutcome{status: "imported", contentID: resp.ContentId, sourceID: resp.SourceId}
}

// emailToProtoRequest converts a parsed email to a gRPC request.
func emailToProtoRequest(email *eml.ParsedEmail, tenantID, sourceTag string, labels []string, jobID string, contentID string) *ingestv1.IngestEmailRequest {
	req := &ingestv1.IngestEmailRequest{
		TenantId:     tenantID,
		MessageId:    email.MessageID,
		ContentHash:  email.ContentHash,
		Subject:      email.Subject,
		BodyPlain:    email.BodyText,
		BodyHtml:     email.BodyHTML,
		SourceSystem: "manual_eml",
		SourceTag:    sourceTag,
		Labels:       labels,
		InReplyTo:    email.InReplyTo,
		References:   email.References,
		JobId:        jobID,
		ContentId:    contentID,
	}

	// From address
	req.From = &ingestv1.EmailAddress{
		Name:    email.From.Name,
		Address: email.From.Email,
	}

	// To addresses
	for _, addr := range email.To {
		req.To = append(req.To, &ingestv1.EmailAddress{
			Name:    addr.Name,
			Address: addr.Email,
		})
	}

	// Cc addresses
	for _, addr := range email.Cc {
		req.Cc = append(req.Cc, &ingestv1.EmailAddress{
			Name:    addr.Name,
			Address: addr.Email,
		})
	}

	// Bcc addresses
	for _, addr := range email.Bcc {
		req.Bcc = append(req.Bcc, &ingestv1.EmailAddress{
			Name:    addr.Name,
			Address: addr.Email,
		})
	}

	// Timestamps
	if !email.Date.IsZero() {
		req.SentAt = timestamppb.New(email.Date)
		req.ReceivedAt = timestamppb.New(email.Date)
	}

	// Attachment metadata (content not included - gateway handles storage)
	for _, att := range email.Attachments {
		req.Attachments = append(req.Attachments, &ingestv1.AttachmentMetadata{
			Filename:  att.Filename,
			MimeType:  att.MimeType,
			SizeBytes: int64(att.Size),
		})
	}

	// Always include To/CC/From headers for pipeline classification (pf-b19e97).
	// The triage activity uses metadata["headers"] for newsletter detection (To==From)
	// and header prepending. The parsed email has structured To/CC/From fields but
	// they weren't flowing into the headers map that downstream expects.
	headers := make(map[string]string)
	if email.From.Email != "" {
		headers["From"] = formatEmailHeader(email.From)
	}
	if len(email.To) > 0 {
		headers["To"] = formatEmailHeaders(email.To)
	}
	if len(email.Cc) > 0 {
		headers["Cc"] = formatEmailHeaders(email.Cc)
	}
	// Merge any explicitly preserved headers (auto-submission, precedence, etc.)
	for k, v := range email.Headers {
		headers[k] = v
	}
	if len(headers) > 0 {
		req.Headers = headers
	}

	return req
}

// formatEmailHeader formats an eml.Address as an RFC 5322 header value.
func formatEmailHeader(addr eml.Address) string {
	if addr.Name != "" {
		return addr.Name + " <" + addr.Email + ">"
	}
	return addr.Email
}

// formatEmailHeaders formats a slice of addresses as a comma-separated header value.
func formatEmailHeaders(addrs []eml.Address) string {
	parts := make([]string, len(addrs))
	for i, addr := range addrs {
		parts[i] = formatEmailHeader(addr)
	}
	return strings.Join(parts, ", ")
}

// recordEmailOutcome updates progress and result based on the processing outcome.
func recordEmailOutcome(
	ctx context.Context,
	client ingestv1.IngestServiceClient,
	jobID, filePath string,
	o emailOutcome,
	progress *emailProgress,
	result *emailIngestResult,
) {
	switch o.status {
	case "imported":
		progress.recordImported()
		if o.contentID != "" {
			result.ContentIDs = append(result.ContentIDs, o.contentID)
		}

	case "skipped":
		progress.recordSkipped()

	case "failed":
		progress.recordFailed()
		result.Errors = append(result.Errors, emailFileError{
			FilePath: filePath,
			Error:    o.err.Error(),
		})

		// Record error via gRPC
		errorType := ingestv1.ErrorType_ERROR_TYPE_UNKNOWN
		errMsg := o.err.Error()
		if strings.Contains(errMsg, "parse") || strings.Contains(errMsg, "Parse") {
			errorType = ingestv1.ErrorType_ERROR_TYPE_PARSE_ERROR
		} else if strings.Contains(errMsg, "validation") {
			errorType = ingestv1.ErrorType_ERROR_TYPE_VALIDATION
		} else if strings.Contains(errMsg, "storage") || strings.Contains(errMsg, "database") {
			errorType = ingestv1.ErrorType_ERROR_TYPE_STORAGE
		}

		_, _ = client.RecordIngestError(ctx, &ingestv1.RecordIngestErrorRequest{
			TenantId:     "", // Will be inferred from job
			JobId:        jobID,
			FilePath:     filePath,
			ErrorType:    errorType,
			ErrorMessage: errMsg,
			IsRetryable:  errorType != ingestv1.ErrorType_ERROR_TYPE_PARSE_ERROR,
		})
	}

	// Update job progress via gRPC (batch updates to reduce overhead)
	snapshot := progress.snapshot()
	if snapshot.ProcessedCount%10 == 0 || snapshot.ProcessedCount == snapshot.TotalFiles {
		_, _ = client.UpdateJobProgress(ctx, &ingestv1.UpdateJobProgressRequest{
			JobId:          jobID,
			ProcessedDelta: 0, // We send absolute counts indirectly via Complete
			FailedDelta:    0,
			SkippedDelta:   0,
		})
	}
}

// displayEmailProgress shows progress updates.
func displayEmailProgress(progress *emailProgress, format config.OutputFormat) {
	if format != config.OutputFormatText {
		return
	}

	snapshot := progress.snapshot()
	if snapshot.ProcessedCount == 0 {
		return
	}

	// Simple progress line
	pct := snapshot.percentComplete()
	remaining := ""
	if snapshot.EstimatedRemainingSeconds != nil {
		remaining = fmt.Sprintf(" ETA: %s", formatDuration(time.Duration(*snapshot.EstimatedRemainingSeconds)*time.Second))
	}

	fmt.Printf("\r  [%3.0f%%] %d/%d files (imported: %d, skipped: %d, failed: %d)%s   ",
		pct,
		snapshot.ProcessedCount,
		snapshot.TotalFiles,
		snapshot.ImportedCount,
		snapshot.SkippedCount,
		snapshot.FailedCount,
		remaining)
}

// displayEmailResults shows the final results.
func displayEmailResults(result *emailIngestResult, format config.OutputFormat) {
	duration := result.CompletedAt.Sub(result.StartedAt)

	switch format {
	case config.OutputFormatJSON:
		outputEmailJSON(result)
	case config.OutputFormatYAML:
		outputEmailYAML(result)
	default:
		outputEmailResultsText(result, duration)
	}
}

// outputEmailResultsText displays results in human-readable format.
func outputEmailResultsText(result *emailIngestResult, duration time.Duration) {
	fmt.Println("Ingest Complete")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("  Job ID:        %s\n", result.JobID)
	fmt.Printf("  Total Files:   %d\n", result.TotalFiles)
	fmt.Printf("  Imported:      \033[32m%d\033[0m\n", result.ImportedCount)
	fmt.Printf("  Skipped:       \033[33m%d\033[0m (duplicates)\n", result.SkippedCount)
	fmt.Printf("  Failed:        \033[31m%d\033[0m\n", result.FailedCount)
	fmt.Printf("  Duration:      %s\n", formatDuration(duration))

	if result.TotalFiles > 0 && duration.Seconds() > 0 {
		rate := float64(result.TotalFiles) / duration.Seconds()
		fmt.Printf("  Rate:          %.1f files/sec\n", rate)
	}

	if result.Success {
		fmt.Printf("\n  Status:        \033[32mSUCCESS\033[0m\n")
	} else {
		fmt.Printf("\n  Status:        \033[31mFAILED\033[0m\n")
	}

	// Display content IDs if any
	if len(result.ContentIDs) > 0 {
		fmt.Println("\nContent IDs:")
		displayCount := len(result.ContentIDs)
		if displayCount > 10 {
			displayCount = 10
		}
		for i := 0; i < displayCount; i++ {
			fmt.Printf("  - %s\n", result.ContentIDs[i])
		}
		if len(result.ContentIDs) > 10 {
			fmt.Printf("  ... and %d more\n", len(result.ContentIDs)-10)
		}
	}

	// Display errors if any
	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for i, e := range result.Errors {
			if i >= 10 {
				fmt.Printf("  ... and %d more errors\n", len(result.Errors)-10)
				break
			}
			fmt.Printf("  - %s: %s\n", truncateIngestString(e.FilePath, 40), e.Error)
		}
	}
}

// emailJSONResult is the JSON output structure for email ingest results.
type emailJSONResult struct {
	JobID       string   `json:"job_id"`
	TotalFiles  int      `json:"total_files"`
	Imported    int      `json:"imported"`
	Skipped     int      `json:"skipped"`
	Failed      int      `json:"failed"`
	Success     bool     `json:"success"`
	StartedAt   string   `json:"started_at"`
	CompletedAt string   `json:"completed_at"`
	ContentIDs  []string `json:"content_ids,omitempty"`
}

// outputEmailJSON outputs result as JSON.
func outputEmailJSON(result *emailIngestResult) {
	output := emailJSONResult{
		JobID:       result.JobID,
		TotalFiles:  result.TotalFiles,
		Imported:    result.ImportedCount,
		Skipped:     result.SkippedCount,
		Failed:      result.FailedCount,
		Success:     result.Success,
		StartedAt:   result.StartedAt.Format(time.RFC3339),
		CompletedAt: result.CompletedAt.Format(time.RFC3339),
		ContentIDs:  result.ContentIDs,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}

// outputEmailYAML outputs result as YAML.
func outputEmailYAML(result *emailIngestResult) {
	output := emailJSONResult{
		JobID:       result.JobID,
		TotalFiles:  result.TotalFiles,
		Imported:    result.ImportedCount,
		Skipped:     result.SkippedCount,
		Failed:      result.FailedCount,
		Success:     result.Success,
		StartedAt:   result.StartedAt.Format(time.RFC3339),
		CompletedAt: result.CompletedAt.Format(time.RFC3339),
		ContentIDs:  result.ContentIDs,
	}
	enc := yaml.NewEncoder(os.Stdout)
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding YAML: %v\n", err)
	}
}
