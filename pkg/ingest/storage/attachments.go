package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/otherjamesbrown/penf-cli/pkg/ingest/types"
	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

// SourceSystemAttachment identifies attachments stored as sources.
const SourceSystemAttachment = "attachment"

// AttachmentSource represents data needed to create an attachment source.
type AttachmentSource struct {
	TenantID        string
	ParentSourceID  int64
	Filename        string
	MimeType        string
	SizeBytes       int64
	ContentHash     string
	Content         []byte // Raw attachment content
	SourceTimestamp time.Time
}

// AttachmentLink represents the link between parent and child sources.
type AttachmentLink struct {
	ParentSourceID int64
	ChildSourceID  *int64 // NULL if skipped

	// Metadata
	Filename    string
	MimeType    string
	SizeBytes   int64
	ContentHash string

	// Position
	Position  int
	ContentID string
	IsInline  bool

	// Classification
	ProcessingTier  types.ProcessingTier
	TierReason      string
	ProcessingSteps []types.ProcessingStep

	// Flags
	IsEmbeddedEmail bool
}

// CreatedAttachment contains the result of creating an attachment.
type CreatedAttachment struct {
	LinkID   int64
	SourceID *int64 // NULL if skipped
}

// StoredAttachment represents a full attachment record from the database.
type StoredAttachment struct {
	ID              int64
	ParentSourceID  int64
	ChildSourceID   *int64
	Filename        string
	MimeType        string
	SizeBytes       int64
	ContentHash     string
	Position        int
	ContentID       string
	IsInline        bool
	ProcessingTier  types.ProcessingTier
	TierReason      string
	ProcessingSteps []types.ProcessingStep
	IsEmbeddedEmail bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// CreateAttachmentWithSource creates both an attachment source and the link in a transaction.
// Use this for attachments that should be processed (auto_process, manual_process).
func (r *Repository) CreateAttachmentWithSource(ctx context.Context, att *AttachmentSource, link *AttachmentLink) (*CreatedAttachment, error) {
	// Use default tenant if not specified or not a valid UUID
	tenantID := att.TenantID
	if tenantID == "" || tenantID == "default" {
		tenantID = DefaultTenantID
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create the attachment as a source
	sourceQuery := `
		INSERT INTO sources (
			tenant_id, source_system, external_id, content_hash,
			raw_content, content_type, content_size,
			ingestion_metadata, processing_status, source_timestamp,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10,
			NOW(), NOW()
		)
		RETURNING id
	`

	// Generate external_id from parent + position
	externalID := fmt.Sprintf("attachment:%d:%d", link.ParentSourceID, link.Position)

	metadata := map[string]interface{}{
		"parent_source_id": link.ParentSourceID,
		"filename":         att.Filename,
		"position":         link.Position,
		"is_inline":        link.IsInline,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// For binary content (images, PDFs, etc.), base64 encode it.
	// Text files can be stored directly.
	var contentToStore string
	if att.MimeType != "" && !strings.HasPrefix(att.MimeType, "text/") {
		// Binary content - base64 encode it
		contentToStore = "base64:" + base64.StdEncoding.EncodeToString(att.Content)
	} else {
		// Text content - store directly
		contentToStore = string(att.Content)
	}

	var sourceID int64
	err = tx.QueryRow(ctx, sourceQuery,
		tenantID,
		SourceSystemAttachment,
		externalID,
		att.ContentHash,
		contentToStore,
		att.MimeType,
		att.SizeBytes,
		metadataJSON,
		ProcessingStatusPending,
		att.SourceTimestamp,
	).Scan(&sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to create attachment source: %w", err)
	}

	// Create the link
	link.ChildSourceID = &sourceID
	linkID, err := r.createAttachmentLinkTx(ctx, tx, link)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Debug("Attachment source and link created",
		logging.F("source_id", sourceID),
		logging.F("link_id", linkID),
		logging.F("parent_id", link.ParentSourceID),
		logging.F("filename", att.Filename))

	return &CreatedAttachment{
		LinkID:   linkID,
		SourceID: &sourceID,
	}, nil
}

// CreateAttachmentLinkOnly creates just the link record without storing content.
// Use this for attachments that should be skipped (auto_skip, manual_skip).
func (r *Repository) CreateAttachmentLinkOnly(ctx context.Context, link *AttachmentLink) (*CreatedAttachment, error) {
	linkID, err := r.createAttachmentLinkTx(ctx, r.pool, link)
	if err != nil {
		return nil, err
	}

	r.logger.Debug("Attachment link created (skipped)",
		logging.F("link_id", linkID),
		logging.F("parent_id", link.ParentSourceID),
		logging.F("filename", link.Filename),
		logging.F("tier", string(link.ProcessingTier)))

	return &CreatedAttachment{
		LinkID:   linkID,
		SourceID: nil,
	}, nil
}

// createAttachmentLinkTx creates the link record using the provided transaction/pool.
func (r *Repository) createAttachmentLinkTx(ctx context.Context, db interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, link *AttachmentLink) (int64, error) {
	stepsJSON, err := json.Marshal(link.ProcessingSteps)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal processing steps: %w", err)
	}

	query := `
		INSERT INTO source_attachments (
			parent_source_id, child_source_id,
			filename, mime_type, size_bytes, content_hash,
			position, content_id, is_inline,
			processing_tier, tier_reason, processing_steps,
			is_embedded_email,
			created_at, updated_at
		) VALUES (
			$1, $2,
			$3, $4, $5, $6,
			$7, $8, $9,
			$10, $11, $12,
			$13,
			NOW(), NOW()
		)
		RETURNING id
	`

	var linkID int64
	err = db.QueryRow(ctx, query,
		link.ParentSourceID,
		link.ChildSourceID,
		link.Filename,
		link.MimeType,
		link.SizeBytes,
		link.ContentHash,
		link.Position,
		link.ContentID,
		link.IsInline,
		link.ProcessingTier,
		link.TierReason,
		stepsJSON,
		link.IsEmbeddedEmail,
	).Scan(&linkID)

	if err != nil {
		return 0, fmt.Errorf("failed to create attachment link: %w", err)
	}

	return linkID, nil
}

// GetAttachmentsForSource retrieves all attachments for a parent source.
func (r *Repository) GetAttachmentsForSource(ctx context.Context, parentSourceID int64) ([]*StoredAttachment, error) {
	query := `
		SELECT
			id, parent_source_id, child_source_id,
			filename, mime_type, size_bytes, content_hash,
			position, content_id, is_inline,
			processing_tier, tier_reason, processing_steps,
			is_embedded_email,
			created_at, updated_at
		FROM source_attachments
		WHERE parent_source_id = $1
		ORDER BY position ASC
	`

	rows, err := r.pool.Query(ctx, query, parentSourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}
	defer rows.Close()

	var atts []*StoredAttachment
	for rows.Next() {
		att := &StoredAttachment{}
		var stepsJSON []byte
		err := rows.Scan(
			&att.ID,
			&att.ParentSourceID,
			&att.ChildSourceID,
			&att.Filename,
			&att.MimeType,
			&att.SizeBytes,
			&att.ContentHash,
			&att.Position,
			&att.ContentID,
			&att.IsInline,
			&att.ProcessingTier,
			&att.TierReason,
			&stepsJSON,
			&att.IsEmbeddedEmail,
			&att.CreatedAt,
			&att.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}

		if err := json.Unmarshal(stepsJSON, &att.ProcessingSteps); err != nil {
			att.ProcessingSteps = []types.ProcessingStep{}
		}

		atts = append(atts, att)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating attachments: %w", err)
	}

	return atts, nil
}

// GetParentSourceID retrieves the parent source ID for an attachment source.
func (r *Repository) GetParentSourceID(ctx context.Context, attachmentSourceID int64) (int64, error) {
	query := `
		SELECT parent_source_id
		FROM source_attachments
		WHERE child_source_id = $1
		LIMIT 1
	`

	var parentID int64
	err := r.pool.QueryRow(ctx, query, attachmentSourceID).Scan(&parentID)
	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("attachment source not found: %d", attachmentSourceID)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get parent source: %w", err)
	}

	return parentID, nil
}

// UpdateAttachmentTier updates the processing tier for manual triage.
func (r *Repository) UpdateAttachmentTier(ctx context.Context, linkID int64, tier types.ProcessingTier, reason string) error {
	// Get current steps
	var stepsJSON []byte
	getQuery := `SELECT processing_steps FROM source_attachments WHERE id = $1`
	err := r.pool.QueryRow(ctx, getQuery, linkID).Scan(&stepsJSON)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("attachment link not found: %d", linkID)
	}
	if err != nil {
		return fmt.Errorf("failed to get attachment: %w", err)
	}

	var steps []types.ProcessingStep
	if err := json.Unmarshal(stepsJSON, &steps); err != nil {
		steps = []types.ProcessingStep{}
	}

	// Add manual triage step
	steps = append(steps, types.NewProcessingStep("manual_triage", tier, reason, 1.0))

	newStepsJSON, err := json.Marshal(steps)
	if err != nil {
		return fmt.Errorf("failed to marshal steps: %w", err)
	}

	// Update
	updateQuery := `
		UPDATE source_attachments
		SET processing_tier = $2, tier_reason = $3, processing_steps = $4, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, updateQuery, linkID, tier, reason, newStepsJSON)
	if err != nil {
		return fmt.Errorf("failed to update tier: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("attachment link not found: %d", linkID)
	}

	r.logger.Debug("Attachment tier updated",
		logging.F("link_id", linkID),
		logging.F("tier", string(tier)),
		logging.F("reason", reason))

	return nil
}

// GetPendingReviewAttachments retrieves attachments needing manual review.
func (r *Repository) GetPendingReviewAttachments(ctx context.Context, tenantID string, limit int) ([]*StoredAttachment, error) {
	query := `
		SELECT
			sa.id, sa.parent_source_id, sa.child_source_id,
			sa.filename, sa.mime_type, sa.size_bytes, sa.content_hash,
			sa.position, sa.content_id, sa.is_inline,
			sa.processing_tier, sa.tier_reason, sa.processing_steps,
			sa.is_embedded_email,
			sa.created_at, sa.updated_at
		FROM source_attachments sa
		JOIN sources s ON sa.parent_source_id = s.id
		WHERE s.tenant_id = $1 AND sa.processing_tier = 'pending_review'
		ORDER BY sa.created_at ASC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending attachments: %w", err)
	}
	defer rows.Close()

	var atts []*StoredAttachment
	for rows.Next() {
		att := &StoredAttachment{}
		var stepsJSON []byte
		err := rows.Scan(
			&att.ID,
			&att.ParentSourceID,
			&att.ChildSourceID,
			&att.Filename,
			&att.MimeType,
			&att.SizeBytes,
			&att.ContentHash,
			&att.Position,
			&att.ContentID,
			&att.IsInline,
			&att.ProcessingTier,
			&att.TierReason,
			&stepsJSON,
			&att.IsEmbeddedEmail,
			&att.CreatedAt,
			&att.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}

		if err := json.Unmarshal(stepsJSON, &att.ProcessingSteps); err != nil {
			att.ProcessingSteps = []types.ProcessingStep{}
		}

		atts = append(atts, att)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating attachments: %w", err)
	}

	return atts, nil
}

// CheckAttachmentDuplicate checks if an attachment with the same content hash exists.
// Returns (exists, existingChildSourceID, error).
func (r *Repository) CheckAttachmentDuplicate(ctx context.Context, contentHash string) (bool, *int64, error) {
	query := `
		SELECT child_source_id
		FROM source_attachments
		WHERE content_hash = $1 AND child_source_id IS NOT NULL
		LIMIT 1
	`

	var sourceID *int64
	err := r.pool.QueryRow(ctx, query, contentHash).Scan(&sourceID)
	if err == pgx.ErrNoRows {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, fmt.Errorf("failed to check attachment duplicate: %w", err)
	}

	return true, sourceID, nil
}
