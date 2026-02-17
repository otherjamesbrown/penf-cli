// Package storage provides database operations for email ingest.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

// SourceSystem identifies the origin system for ingested content.
const (
	SourceSystemManualEML = "manual_eml"
	SourceSystemGmail     = "gmail"
)

// DefaultTenantID is the UUID for the default tenant (single-tenant mode).
const DefaultTenantID = "00000001-0000-0000-0000-000000000001"

// IngestErrorType identifies the type of error during ingest.
type IngestErrorType string

const (
	ErrorTypeParse      IngestErrorType = "parse_error"
	ErrorTypeEncoding   IngestErrorType = "encoding_error"
	ErrorTypeIO         IngestErrorType = "io_error"
	ErrorTypeValidation IngestErrorType = "validation_error"
	ErrorTypeStorage    IngestErrorType = "storage_error"
	ErrorTypeUnexpected IngestErrorType = "unexpected_error"
)

// ProcessingStatus represents the current state of source processing.
const (
	ProcessingStatusPending    = "pending"
	ProcessingStatusProcessing = "processing"
	ProcessingStatusCompleted  = "completed"
	ProcessingStatusFailed     = "failed"
)

// IngestJobStatus represents the state of an ingest batch job.
type IngestJobStatus string

const (
	IngestJobStatusPending           IngestJobStatus = "pending"
	IngestJobStatusInProgress        IngestJobStatus = "in_progress"
	IngestJobStatusCompleted         IngestJobStatus = "completed"
	IngestJobStatusCompletedErrors   IngestJobStatus = "completed_with_errors"
	IngestJobStatusFailed            IngestJobStatus = "failed"
	IngestJobStatusCancelled         IngestJobStatus = "cancelled"
)

// EmailSource represents the data needed to create a source record.
type EmailSource struct {
	TenantID          string
	SourceSystem      string
	ExternalID        string // Message-ID
	ContentHash       string
	RawContent        string
	ContentType       string
	ContentSize       int32
	Metadata          map[string]interface{}
	SourceTimestamp   time.Time
	ParticipantEmails []string // From, To, Cc, Bcc email addresses
	ContentID         string   // Optional: short human-readable ID for unified tracing (format: <type>-<base62>)
}

// CreatedSource contains the result of creating a source.
type CreatedSource struct {
	ID        int64
	CreatedAt time.Time
	ContentID string // The content_id that was stored (echoed back for confirmation)
}

// IngestJob tracks a batch ingest operation.
// Matches the database schema: ingest_jobs table
type IngestJob struct {
	ID             string
	TenantID       string // UUID format
	Status         IngestJobStatus
	SourceTag      string
	ContentType    string // e.g., "email"
	TotalFiles     int
	ProcessedCount int
	ImportedCount  int
	SkippedCount   int
	FailedCount    int
	FileManifest   []string // JSON array of file paths
	ProcessedFiles []string // JSON array of processed file paths
	Options        map[string]interface{}
	StartedAt      *time.Time
	CompletedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IngestError records an error for a specific file during ingest.
// Matches the database schema: ingest_errors table
type IngestError struct {
	ID           string // UUID
	JobID        string // UUID
	FilePath     string
	ErrorType    IngestErrorType
	ErrorMsg     string
	ErrorDetails map[string]interface{}
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Repository provides database operations for email ingest.
type Repository struct {
	pool   *pgxpool.Pool
	logger logging.Logger
}

// NewRepository creates a new ingest repository.
func NewRepository(pool *pgxpool.Pool, logger logging.Logger) *Repository {
	return &Repository{
		pool:   pool,
		logger: logger.With(logging.F("component", "ingest_repository")),
	}
}

// CreateSource inserts a new source record and returns the created ID.
func (r *Repository) CreateSource(ctx context.Context, source *EmailSource) (*CreatedSource, error) {
	// Use default tenant if not specified or not a valid UUID
	tenantID := source.TenantID
	if tenantID == "" || tenantID == "default" {
		tenantID = DefaultTenantID
	}

	metadataJSON, err := json.Marshal(source.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Handle nullable content_id (empty string becomes NULL)
	var contentID interface{}
	if source.ContentID != "" {
		contentID = source.ContentID
	}

	query := `
		INSERT INTO sources (
			tenant_id, source_system, external_id, content_hash,
			raw_content, content_type, content_size,
			ingestion_metadata, processing_status, source_timestamp,
			participant_emails, content_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10,
			$11, $12, NOW(), NOW()
		)
		RETURNING id, created_at, content_id
	`

	var result CreatedSource
	var returnedContentID *string
	err = r.pool.QueryRow(ctx, query,
		tenantID,
		source.SourceSystem,
		source.ExternalID,
		source.ContentHash,
		source.RawContent,
		source.ContentType,
		source.ContentSize,
		metadataJSON,
		ProcessingStatusPending,
		source.SourceTimestamp,
		source.ParticipantEmails,
		contentID,
	).Scan(&result.ID, &result.CreatedAt, &returnedContentID)

	if err != nil {
		r.logger.Error("Failed to create source",
			logging.Err(err),
			logging.F("tenant_id", tenantID),
			logging.F("external_id", source.ExternalID))
		return nil, fmt.Errorf("failed to create source: %w", err)
	}

	// Convert nullable content_id back to string
	if returnedContentID != nil {
		result.ContentID = *returnedContentID
	}

	r.logger.Debug("Source created",
		logging.F("id", result.ID),
		logging.F("tenant_id", tenantID),
		logging.F("external_id", source.ExternalID),
		logging.F("content_id", result.ContentID))

	return &result, nil
}

// ExistsByExternalID checks if a source with the given external ID exists.
func (r *Repository) ExistsByExternalID(ctx context.Context, tenantID, externalID string) (bool, int64, error) {
	// Use default tenant if not specified or not a valid UUID
	if tenantID == "" || tenantID == "default" {
		tenantID = DefaultTenantID
	}

	query := `
		SELECT id FROM sources
		WHERE tenant_id = $1 AND external_id = $2 AND deleted_at IS NULL
		LIMIT 1
	`

	var id int64
	err := r.pool.QueryRow(ctx, query, tenantID, externalID).Scan(&id)

	if err == pgx.ErrNoRows {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("failed to check existence by external_id: %w", err)
	}

	return true, id, nil
}

// ExistsByContentHash checks if a source with the given content hash exists.
func (r *Repository) ExistsByContentHash(ctx context.Context, tenantID, contentHash string) (bool, int64, error) {
	// Use default tenant if not specified or not a valid UUID
	if tenantID == "" || tenantID == "default" {
		tenantID = DefaultTenantID
	}

	query := `
		SELECT id FROM sources
		WHERE tenant_id = $1 AND content_hash = $2 AND deleted_at IS NULL
		LIMIT 1
	`

	var id int64
	err := r.pool.QueryRow(ctx, query, tenantID, contentHash).Scan(&id)

	if err == pgx.ErrNoRows {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("failed to check existence by content_hash: %w", err)
	}

	return true, id, nil
}

// CheckDuplicate checks if an email is a duplicate by message ID or content hash.
// Returns (isDuplicate, existingID, duplicateReason, error)
func (r *Repository) CheckDuplicate(ctx context.Context, tenantID, messageID, contentHash string) (bool, int64, string, error) {
	// First check by message ID (exact duplicate)
	exists, id, err := r.ExistsByExternalID(ctx, tenantID, messageID)
	if err != nil {
		return false, 0, "", err
	}
	if exists {
		return true, id, "message_id", nil
	}

	// Then check by content hash (content duplicate)
	exists, id, err = r.ExistsByContentHash(ctx, tenantID, contentHash)
	if err != nil {
		return false, 0, "", err
	}
	if exists {
		return true, id, "content_hash", nil
	}

	return false, 0, "", nil
}

// CreateJob creates a new ingest job record.
func (r *Repository) CreateJob(ctx context.Context, job *IngestJob) error {
	// Use default tenant if not specified or not a valid UUID
	tenantID := job.TenantID
	if tenantID == "" || tenantID == "default" {
		tenantID = DefaultTenantID
	}

	manifestJSON, err := json.Marshal(job.FileManifest)
	if err != nil {
		return fmt.Errorf("failed to marshal file manifest: %w", err)
	}

	processedJSON, err := json.Marshal(job.ProcessedFiles)
	if err != nil {
		return fmt.Errorf("failed to marshal processed files: %w", err)
	}

	optionsJSON, err := json.Marshal(job.Options)
	if err != nil {
		return fmt.Errorf("failed to marshal options: %w", err)
	}

	contentType := job.ContentType
	if contentType == "" {
		contentType = "email"
	}

	query := `
		INSERT INTO ingest_jobs (
			id, tenant_id, source_tag, content_type, status,
			total_files, processed_count, imported_count, skipped_count, failed_count,
			file_manifest, processed_files, options,
			started_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13,
			NOW(), NOW(), NOW()
		)
	`

	_, err = r.pool.Exec(ctx, query,
		job.ID,
		tenantID,
		job.SourceTag,
		contentType,
		job.Status,
		job.TotalFiles,
		job.ProcessedCount,
		job.ImportedCount,
		job.SkippedCount,
		job.FailedCount,
		manifestJSON,
		processedJSON,
		optionsJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	r.logger.Debug("Ingest job created",
		logging.F("job_id", job.ID),
		logging.F("tenant_id", tenantID))

	return nil
}

// GetJob retrieves an ingest job by ID.
func (r *Repository) GetJob(ctx context.Context, jobID string) (*IngestJob, error) {
	query := `
		SELECT
			id, tenant_id, source_tag, content_type, status,
			total_files, processed_count, imported_count, skipped_count, failed_count,
			file_manifest, processed_files, options,
			started_at, completed_at, created_at, updated_at
		FROM ingest_jobs
		WHERE id = $1
	`

	job := &IngestJob{}
	var manifestJSON, processedJSON, optionsJSON []byte
	err := r.pool.QueryRow(ctx, query, jobID).Scan(
		&job.ID,
		&job.TenantID,
		&job.SourceTag,
		&job.ContentType,
		&job.Status,
		&job.TotalFiles,
		&job.ProcessedCount,
		&job.ImportedCount,
		&job.SkippedCount,
		&job.FailedCount,
		&manifestJSON,
		&processedJSON,
		&optionsJSON,
		&job.StartedAt,
		&job.CompletedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	if err := json.Unmarshal(manifestJSON, &job.FileManifest); err != nil {
		job.FileManifest = []string{}
	}
	if err := json.Unmarshal(processedJSON, &job.ProcessedFiles); err != nil {
		job.ProcessedFiles = []string{}
	}
	if err := json.Unmarshal(optionsJSON, &job.Options); err != nil {
		job.Options = map[string]interface{}{}
	}

	return job, nil
}

// UpdateJobProgress updates the progress of an ingest job.
func (r *Repository) UpdateJobProgress(ctx context.Context, jobID string, processed, imported, skipped, failed int, processedFiles []string) error {
	processedJSON, err := json.Marshal(processedFiles)
	if err != nil {
		return fmt.Errorf("failed to marshal processed files: %w", err)
	}

	query := `
		UPDATE ingest_jobs
		SET processed_count = $2, imported_count = $3, skipped_count = $4, failed_count = $5,
		    processed_files = $6, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, jobID, processed, imported, skipped, failed, processedJSON)
	if err != nil {
		return fmt.Errorf("failed to update job progress: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("job not found: %s", jobID)
	}

	return nil
}

// CompleteJob marks an ingest job as completed.
func (r *Repository) CompleteJob(ctx context.Context, jobID string, status IngestJobStatus) error {
	query := `
		UPDATE ingest_jobs
		SET status = $2, completed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, jobID, status)
	if err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("job not found: %s", jobID)
	}

	r.logger.Debug("Job completed",
		logging.F("job_id", jobID),
		logging.F("status", string(status)))

	return nil
}

// RecordError records an error that occurred during ingest.
func (r *Repository) RecordError(ctx context.Context, jobID, filePath string, errorType IngestErrorType, errorMsg string, details map[string]interface{}) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	query := `
		INSERT INTO ingest_errors (job_id, file_path, error_type, error_message, error_details, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
	`

	_, err = r.pool.Exec(ctx, query, jobID, filePath, errorType, errorMsg, detailsJSON)
	if err != nil {
		return fmt.Errorf("failed to record error: %w", err)
	}

	return nil
}

// GetJobErrors retrieves all errors for a job.
func (r *Repository) GetJobErrors(ctx context.Context, jobID string) ([]*IngestError, error) {
	query := `
		SELECT id, job_id, file_path, error_type, error_message, error_details, created_at, updated_at
		FROM ingest_errors
		WHERE job_id = $1
		ORDER BY created_at ASC
		LIMIT 1000
	`

	rows, err := r.pool.Query(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job errors: %w", err)
	}
	defer rows.Close()

	var errors []*IngestError
	for rows.Next() {
		e := &IngestError{}
		var detailsJSON []byte
		if err := rows.Scan(&e.ID, &e.JobID, &e.FilePath, &e.ErrorType, &e.ErrorMsg, &detailsJSON, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan error: %w", err)
		}
		if err := json.Unmarshal(detailsJSON, &e.ErrorDetails); err != nil {
			e.ErrorDetails = map[string]interface{}{}
		}
		errors = append(errors, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating errors: %w", err)
	}

	return errors, nil
}

// SourceRecord represents a source retrieved from the database.
type SourceRecord struct {
	ID               int64
	TenantID         string
	RawContent       string
	ContentType      string
	ContentHash      string
	ProcessingStatus string
}

// GetSource retrieves a source by ID for processing.
func (r *Repository) GetSource(ctx context.Context, tenantID string, sourceID int64) (*SourceRecord, error) {
	// Use default tenant if not specified or not a valid UUID
	if tenantID == "" || tenantID == "default" {
		tenantID = DefaultTenantID
	}

	query := `
		SELECT id, tenant_id, raw_content, content_type, content_hash, processing_status
		FROM sources
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`

	source := &SourceRecord{}
	err := r.pool.QueryRow(ctx, query, sourceID, tenantID).Scan(
		&source.ID,
		&source.TenantID,
		&source.RawContent,
		&source.ContentType,
		&source.ContentHash,
		&source.ProcessingStatus,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("source not found: %d", sourceID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get source %d: %w", sourceID, err)
	}

	return source, nil
}

// UpdateSourceStatus updates the processing status of a source.
func (r *Repository) UpdateSourceStatus(ctx context.Context, tenantID string, sourceID int64, status string) error {
	// Use default tenant if not specified or not a valid UUID
	if tenantID == "" || tenantID == "default" {
		tenantID = DefaultTenantID
	}

	query := `
		UPDATE sources
		SET processing_status = $3, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, sourceID, tenantID, status)
	if err != nil {
		return fmt.Errorf("failed to update source %d status: %w", sourceID, err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("source not found: %d", sourceID)
	}

	r.logger.Debug("Source status updated",
		logging.F("source_id", sourceID),
		logging.F("status", status))

	return nil
}

// UpdateSourceStatusWithFailure updates the processing status and failure info of a source.
// If triage metadata fields are provided, they are persisted to the ingestion_metadata JSONB column.
func (r *Repository) UpdateSourceStatusWithFailure(ctx context.Context, tenantID string, sourceID int64, status, failureCategory, failureReason string, triageMetadata ...map[string]interface{}) error {
	// Use default tenant if not specified or not a valid UUID
	if tenantID == "" || tenantID == "default" {
		tenantID = DefaultTenantID
	}

	// Build the query based on whether triage metadata is provided
	var query string
	var args []interface{}

	if len(triageMetadata) > 0 && triageMetadata[0] != nil && len(triageMetadata[0]) > 0 {
		// Update with triage metadata merged into ingestion_metadata JSONB
		metadataJSON, err := json.Marshal(triageMetadata[0])
		if err != nil {
			return fmt.Errorf("failed to marshal triage metadata: %w", err)
		}

		query = `
			UPDATE sources
			SET processing_status = $3,
			    failure_category = $4,
			    failure_reason = $5,
			    ingestion_metadata = COALESCE(ingestion_metadata, '{}'::jsonb) || $6::jsonb,
			    updated_at = NOW()
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`
		args = []interface{}{sourceID, tenantID, status, failureCategory, failureReason, metadataJSON}
	} else {
		// Original behavior: only update status and failure fields
		query = `
			UPDATE sources
			SET processing_status = $3,
			    failure_category = $4,
			    failure_reason = $5,
			    updated_at = NOW()
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`
		args = []interface{}{sourceID, tenantID, status, failureCategory, failureReason}
	}

	result, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update source %d status: %w", sourceID, err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("source not found: %d", sourceID)
	}

	r.logger.Debug("Source status updated with failure info",
		logging.F("source_id", sourceID),
		logging.F("status", status),
		logging.F("failure_category", failureCategory),
		logging.F("has_triage_metadata", len(triageMetadata) > 0))

	return nil
}

// GetRemainingFilesForJob returns files that haven't been processed yet for a resumed job.
// This requires the caller to provide the full file list and we filter out already processed files.
func (r *Repository) GetRemainingFilesForJob(ctx context.Context, jobID string, allFiles []string) ([]string, error) {
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	// If no files have been processed, return all
	if len(job.ProcessedFiles) == 0 {
		return allFiles, nil
	}

	// Build a set of processed files
	processed := make(map[string]bool)
	for _, f := range job.ProcessedFiles {
		processed[f] = true
	}

	// Filter out already processed files
	var remaining []string
	for _, f := range allFiles {
		if !processed[f] {
			remaining = append(remaining, f)
		}
	}

	return remaining, nil
}
