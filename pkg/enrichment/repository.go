package enrichment

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

// Repository provides database operations for content enrichment.
type Repository struct {
	pool   *pgxpool.Pool
	logger logging.Logger
}

// NewRepository creates a new enrichment repository.
func NewRepository(pool *pgxpool.Pool, logger logging.Logger) *Repository {
	return &Repository{
		pool:   pool,
		logger: logger.With(logging.F("component", "enrichment_repository")),
	}
}

// Create inserts a new enrichment record.
func (r *Repository) Create(ctx context.Context, e *Enrichment) error {
	participantsJSON, err := json.Marshal(e.Participants)
	if err != nil {
		return fmt.Errorf("failed to marshal participants: %w", err)
	}

	resolvedJSON, err := json.Marshal(e.ResolvedParticipants)
	if err != nil {
		return fmt.Errorf("failed to marshal resolved_participants: %w", err)
	}

	linksJSON, err := json.Marshal(e.ExtractedLinks)
	if err != nil {
		return fmt.Errorf("failed to marshal extracted_links: %w", err)
	}

	extractedDataJSON, err := json.Marshal(e.ExtractedData)
	if err != nil {
		return fmt.Errorf("failed to marshal extracted_data: %w", err)
	}

	query := `
		INSERT INTO content_enrichment (
			source_id, tenant_id,
			content_type, content_subtype, processing_profile,
			classification_confidence, classification_reason,
			source_system,
			status, current_stage, error_message, retry_count,
			participants, resolved_participants, extracted_links,
			thread_id, project_id, extracted_data,
			ai_processed, ai_skip_reason, ai_processed_at,
			created_at, updated_at, completed_at
		) VALUES (
			$1, $2,
			$3, $4, $5,
			$6, $7,
			$8,
			$9, $10, $11, $12,
			$13, $14, $15,
			$16, $17, $18,
			$19, $20, $21,
			NOW(), NOW(), $22
		)
		ON CONFLICT (source_id) DO NOTHING
		RETURNING id, created_at, updated_at
	`

	err = r.pool.QueryRow(ctx, query,
		e.SourceID,
		e.TenantID,
		e.Classification.ContentType,
		e.Classification.Subtype,
		e.Classification.Profile,
		e.Classification.Confidence,
		e.Classification.Reason,
		e.SourceSystem,
		e.Status,
		e.CurrentStage,
		nullIfEmpty(e.ErrorMessage),
		e.RetryCount,
		participantsJSON,
		resolvedJSON,
		linksJSON,
		nullIfEmpty(e.ThreadID),
		e.ProjectID,
		extractedDataJSON,
		e.AIProcessed,
		nullIfEmpty(e.AISkipReason),
		e.AIProcessedAt,
		e.CompletedAt,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)

	// Handle ON CONFLICT case - query existing row
	if err == pgx.ErrNoRows {
		err = r.pool.QueryRow(ctx, `
			SELECT id, created_at, updated_at
			FROM content_enrichment
			WHERE source_id = $1
		`, e.SourceID).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
		if err != nil {
			r.logger.Error("Failed to query existing enrichment after conflict",
				logging.Err(err),
				logging.F("source_id", e.SourceID),
			)
			return fmt.Errorf("failed to query existing enrichment: %w", err)
		}
		r.logger.Debug("Enrichment already exists, retrieved existing record",
			logging.F("source_id", e.SourceID),
			logging.F("enrichment_id", e.ID),
		)
		return nil
	}

	if err != nil {
		r.logger.Error("Failed to create enrichment",
			logging.Err(err),
			logging.F("source_id", e.SourceID),
			logging.F("tenant_id", e.TenantID))
		return fmt.Errorf("failed to create enrichment: %w", err)
	}

	r.logger.Debug("Enrichment created",
		logging.F("id", e.ID),
		logging.F("source_id", e.SourceID),
		logging.F("content_type", string(e.Classification.ContentType)),
		logging.F("subtype", string(e.Classification.Subtype)))

	return nil
}

// GetBySourceID retrieves an enrichment by source ID.
func (r *Repository) GetBySourceID(ctx context.Context, sourceID int64) (*Enrichment, error) {
	query := `
		SELECT
			id, source_id, tenant_id,
			content_type, content_subtype, processing_profile,
			classification_confidence, classification_reason,
			status, current_stage, error_message, retry_count,
			participants, resolved_participants, extracted_links,
			thread_id, project_id, extracted_data,
			ai_processed, ai_skip_reason, ai_processed_at,
			created_at, updated_at, completed_at
		FROM content_enrichment
		WHERE source_id = $1
	`

	return r.scanEnrichment(ctx, query, sourceID)
}

// GetByID retrieves an enrichment by ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*Enrichment, error) {
	query := `
		SELECT
			id, source_id, tenant_id,
			content_type, content_subtype, processing_profile,
			classification_confidence, classification_reason,
			status, current_stage, error_message, retry_count,
			participants, resolved_participants, extracted_links,
			thread_id, project_id, extracted_data,
			ai_processed, ai_skip_reason, ai_processed_at,
			created_at, updated_at, completed_at
		FROM content_enrichment
		WHERE id = $1
	`

	return r.scanEnrichment(ctx, query, id)
}

// Update updates an existing enrichment record.
func (r *Repository) Update(ctx context.Context, e *Enrichment) error {
	participantsJSON, err := json.Marshal(e.Participants)
	if err != nil {
		return fmt.Errorf("failed to marshal participants: %w", err)
	}

	resolvedJSON, err := json.Marshal(e.ResolvedParticipants)
	if err != nil {
		return fmt.Errorf("failed to marshal resolved_participants: %w", err)
	}

	linksJSON, err := json.Marshal(e.ExtractedLinks)
	if err != nil {
		return fmt.Errorf("failed to marshal extracted_links: %w", err)
	}

	extractedDataJSON, err := json.Marshal(e.ExtractedData)
	if err != nil {
		return fmt.Errorf("failed to marshal extracted_data: %w", err)
	}

	query := `
		UPDATE content_enrichment SET
			content_type = $2,
			content_subtype = $3,
			processing_profile = $4,
			classification_confidence = $5,
			classification_reason = $6,
			status = $7,
			current_stage = $8,
			error_message = $9,
			retry_count = $10,
			participants = $11,
			resolved_participants = $12,
			extracted_links = $13,
			thread_id = $14,
			project_id = $15,
			extracted_data = $16,
			ai_processed = $17,
			ai_skip_reason = $18,
			ai_processed_at = $19,
			completed_at = $20,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err = r.pool.QueryRow(ctx, query,
		e.ID,
		e.Classification.ContentType,
		e.Classification.Subtype,
		e.Classification.Profile,
		e.Classification.Confidence,
		e.Classification.Reason,
		e.Status,
		e.CurrentStage,
		nullIfEmpty(e.ErrorMessage),
		e.RetryCount,
		participantsJSON,
		resolvedJSON,
		linksJSON,
		nullIfEmpty(e.ThreadID),
		e.ProjectID,
		extractedDataJSON,
		e.AIProcessed,
		nullIfEmpty(e.AISkipReason),
		e.AIProcessedAt,
		e.CompletedAt,
	).Scan(&e.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update enrichment: %w", err)
	}

	return nil
}

// UpdateStatus updates only the status fields of an enrichment.
func (r *Repository) UpdateStatus(ctx context.Context, id int64, status EnrichmentStatus, stage string, errMsg string) error {
	query := `
		UPDATE content_enrichment SET
			status = $2,
			current_stage = $3,
			error_message = $4,
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id, status, stage, nullIfEmpty(errMsg))
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("enrichment not found: %d", id)
	}

	return nil
}

// MarkCompleted marks an enrichment as completed.
func (r *Repository) MarkCompleted(ctx context.Context, id int64) error {
	query := `
		UPDATE content_enrichment SET
			status = 'completed',
			current_stage = NULL,
			completed_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark completed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("enrichment not found: %d", id)
	}

	return nil
}

// MarkFailed marks an enrichment as failed with an error message.
func (r *Repository) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	query := `
		UPDATE content_enrichment SET
			status = 'failed',
			error_message = $2,
			retry_count = retry_count + 1,
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id, errMsg)
	if err != nil {
		return fmt.Errorf("failed to mark failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("enrichment not found: %d", id)
	}

	return nil
}

// ListPending returns pending enrichments for processing.
func (r *Repository) ListPending(ctx context.Context, tenantID string, limit int) ([]*Enrichment, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := `
		SELECT
			id, source_id, tenant_id,
			content_type, content_subtype, processing_profile,
			classification_confidence, classification_reason,
			status, current_stage, error_message, retry_count,
			participants, resolved_participants, extracted_links,
			thread_id, project_id, extracted_data,
			ai_processed, ai_skip_reason, ai_processed_at,
			created_at, updated_at, completed_at
		FROM content_enrichment
		WHERE tenant_id = $1 AND status = 'pending'
		ORDER BY created_at ASC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending: %w", err)
	}
	defer rows.Close()

	var enrichments []*Enrichment
	for rows.Next() {
		e, err := r.scanEnrichmentRow(rows)
		if err != nil {
			return nil, err
		}
		enrichments = append(enrichments, e)
	}

	return enrichments, rows.Err()
}

// ListFailed returns failed enrichments eligible for retry.
func (r *Repository) ListFailed(ctx context.Context, tenantID string, maxRetries int, limit int) ([]*Enrichment, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := `
		SELECT
			id, source_id, tenant_id,
			content_type, content_subtype, processing_profile,
			classification_confidence, classification_reason,
			status, current_stage, error_message, retry_count,
			participants, resolved_participants, extracted_links,
			thread_id, project_id, extracted_data,
			ai_processed, ai_skip_reason, ai_processed_at,
			created_at, updated_at, completed_at
		FROM content_enrichment
		WHERE tenant_id = $1 AND status = 'failed' AND retry_count < $2
		ORDER BY updated_at ASC
		LIMIT $3
	`

	rows, err := r.pool.Query(ctx, query, tenantID, maxRetries, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list failed: %w", err)
	}
	defer rows.Close()

	var enrichments []*Enrichment
	for rows.Next() {
		e, err := r.scanEnrichmentRow(rows)
		if err != nil {
			return nil, err
		}
		enrichments = append(enrichments, e)
	}

	return enrichments, rows.Err()
}

// RecordStage records a processing stage result.
func (r *Repository) RecordStage(ctx context.Context, stage *StageResult) error {
	query := `
		INSERT INTO enrichment_stages (
			enrichment_id, stage_name, processor_name, status,
			input_data, output_data, error_message,
			started_at, completed_at, duration_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`

	err := r.pool.QueryRow(ctx, query,
		stage.EnrichmentID,
		stage.StageName,
		stage.ProcessorName,
		stage.Status,
		stage.InputData,
		stage.OutputData,
		nullIfEmpty(stage.ErrorMessage),
		stage.StartedAt,
		stage.CompletedAt,
		stage.DurationMs,
	).Scan(&stage.ID, &stage.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to record stage: %w", err)
	}

	return nil
}

// GetStages retrieves all stage records for an enrichment.
func (r *Repository) GetStages(ctx context.Context, enrichmentID int64) ([]*StageResult, error) {
	query := `
		SELECT
			id, enrichment_id, stage_name, processor_name, status,
			input_data, output_data, error_message,
			started_at, completed_at, duration_ms, created_at
		FROM enrichment_stages
		WHERE enrichment_id = $1
		ORDER BY created_at ASC
		LIMIT 1000
	`

	rows, err := r.pool.Query(ctx, query, enrichmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stages: %w", err)
	}
	defer rows.Close()

	var stages []*StageResult
	for rows.Next() {
		s := &StageResult{}
		err := rows.Scan(
			&s.ID,
			&s.EnrichmentID,
			&s.StageName,
			&s.ProcessorName,
			&s.Status,
			&s.InputData,
			&s.OutputData,
			&s.ErrorMessage,
			&s.StartedAt,
			&s.CompletedAt,
			&s.DurationMs,
			&s.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stage: %w", err)
		}
		stages = append(stages, s)
	}

	return stages, rows.Err()
}

// Stats returns enrichment statistics for a tenant.
type EnrichmentStats struct {
	TenantID       string         `json:"tenant_id"`
	TotalCount     int            `json:"total_count"`
	PendingCount   int            `json:"pending_count"`
	CompletedCount int            `json:"completed_count"`
	FailedCount    int            `json:"failed_count"`
	ByType         map[string]int `json:"by_type"`
	ByProfile      map[string]int `json:"by_profile"`
}

// GetStats returns enrichment statistics for a tenant.
func (r *Repository) GetStats(ctx context.Context, tenantID string) (*EnrichmentStats, error) {
	stats := &EnrichmentStats{
		TenantID:  tenantID,
		ByType:    make(map[string]int),
		ByProfile: make(map[string]int),
	}

	// Get counts by status
	statusQuery := `
		SELECT status, COUNT(*)
		FROM content_enrichment
		WHERE tenant_id = $1
		GROUP BY status
	`
	rows, err := r.pool.Query(ctx, statusQuery, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get status counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats.TotalCount += count
		switch EnrichmentStatus(status) {
		case StatusPending, StatusClassifying, StatusEnriching, StatusExtracting, StatusAIProcessing:
			stats.PendingCount += count
		case StatusCompleted:
			stats.CompletedCount = count
		case StatusFailed:
			stats.FailedCount = count
		}
	}

	// Get counts by type
	typeQuery := `
		SELECT content_type, COUNT(*)
		FROM content_enrichment
		WHERE tenant_id = $1
		GROUP BY content_type
	`
	rows, err = r.pool.Query(ctx, typeQuery, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get type counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var contentType string
		var count int
		if err := rows.Scan(&contentType, &count); err != nil {
			return nil, err
		}
		stats.ByType[contentType] = count
	}

	// Get counts by profile
	profileQuery := `
		SELECT processing_profile, COUNT(*)
		FROM content_enrichment
		WHERE tenant_id = $1
		GROUP BY processing_profile
	`
	rows, err = r.pool.Query(ctx, profileQuery, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var profile string
		var count int
		if err := rows.Scan(&profile, &count); err != nil {
			return nil, err
		}
		stats.ByProfile[profile] = count
	}

	return stats, nil
}

// scanEnrichment scans a single enrichment from a query.
func (r *Repository) scanEnrichment(ctx context.Context, query string, args ...interface{}) (*Enrichment, error) {
	row := r.pool.QueryRow(ctx, query, args...)
	return r.scanEnrichmentFromRow(row)
}

func (r *Repository) scanEnrichmentFromRow(row pgx.Row) (*Enrichment, error) {
	e := &Enrichment{}
	var participantsJSON, resolvedJSON, linksJSON, extractedDataJSON []byte
	var contentType, subtype, profile string
	var threadID, classificationReason, currentStage, errorMessage, aiSkipReason *string

	err := row.Scan(
		&e.ID,
		&e.SourceID,
		&e.TenantID,
		&contentType,
		&subtype,
		&profile,
		&e.Classification.Confidence,
		&classificationReason,
		&e.Status,
		&currentStage,
		&errorMessage,
		&e.RetryCount,
		&participantsJSON,
		&resolvedJSON,
		&linksJSON,
		&threadID,
		&e.ProjectID,
		&extractedDataJSON,
		&e.AIProcessed,
		&aiSkipReason,
		&e.AIProcessedAt,
		&e.CreatedAt,
		&e.UpdatedAt,
		&e.CompletedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan enrichment: %w", err)
	}

	e.Classification.ContentType = ContentType(contentType)
	e.Classification.Subtype = ContentSubtype(subtype)
	e.Classification.Profile = ProcessingProfile(profile)
	e.Classification.Reason = derefString(classificationReason)
	e.CurrentStage = derefString(currentStage)
	e.ErrorMessage = derefString(errorMessage)
	e.ThreadID = derefString(threadID)
	e.AISkipReason = derefString(aiSkipReason)

	if err := json.Unmarshal(participantsJSON, &e.Participants); err != nil {
		e.Participants = nil
	}
	if err := json.Unmarshal(resolvedJSON, &e.ResolvedParticipants); err != nil {
		e.ResolvedParticipants = nil
	}
	if err := json.Unmarshal(linksJSON, &e.ExtractedLinks); err != nil {
		e.ExtractedLinks = nil
	}
	if err := json.Unmarshal(extractedDataJSON, &e.ExtractedData); err != nil {
		e.ExtractedData = nil
	}

	return e, nil
}

func (r *Repository) scanEnrichmentRow(rows pgx.Rows) (*Enrichment, error) {
	e := &Enrichment{}
	var participantsJSON, resolvedJSON, linksJSON, extractedDataJSON []byte
	var contentType, subtype, profile string
	var threadID, classificationReason, currentStage, errorMessage, aiSkipReason *string

	err := rows.Scan(
		&e.ID,
		&e.SourceID,
		&e.TenantID,
		&contentType,
		&subtype,
		&profile,
		&e.Classification.Confidence,
		&classificationReason,
		&e.Status,
		&currentStage,
		&errorMessage,
		&e.RetryCount,
		&participantsJSON,
		&resolvedJSON,
		&linksJSON,
		&threadID,
		&e.ProjectID,
		&extractedDataJSON,
		&e.AIProcessed,
		&aiSkipReason,
		&e.AIProcessedAt,
		&e.CreatedAt,
		&e.UpdatedAt,
		&e.CompletedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan enrichment: %w", err)
	}

	e.Classification.ContentType = ContentType(contentType)
	e.Classification.Subtype = ContentSubtype(subtype)
	e.Classification.Profile = ProcessingProfile(profile)
	e.Classification.Reason = derefString(classificationReason)
	e.CurrentStage = derefString(currentStage)
	e.ErrorMessage = derefString(errorMessage)
	e.ThreadID = derefString(threadID)
	e.AISkipReason = derefString(aiSkipReason)

	if err := json.Unmarshal(participantsJSON, &e.Participants); err != nil {
		e.Participants = nil
	}
	if err := json.Unmarshal(resolvedJSON, &e.ResolvedParticipants); err != nil {
		e.ResolvedParticipants = nil
	}
	if err := json.Unmarshal(linksJSON, &e.ExtractedLinks); err != nil {
		e.ExtractedLinks = nil
	}
	if err := json.Unmarshal(extractedDataJSON, &e.ExtractedData); err != nil {
		e.ExtractedData = nil
	}

	return e, nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Ensure we have time imported for AIProcessedAt
var _ = time.Now
