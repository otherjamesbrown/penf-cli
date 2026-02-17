// Package attachments provides attachment extraction and classification for email ingest.
package attachments

import (
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/ingest/types"
)

// Re-export ProcessingTier constants for convenience
const (
	TierAutoProcess   = types.TierAutoProcess
	TierAutoSkip      = types.TierAutoSkip
	TierPendingReview = types.TierPendingReview
	TierManualProcess = types.TierManualProcess
	TierManualSkip    = types.TierManualSkip
)

// ProcessingTier is an alias for types.ProcessingTier
type ProcessingTier = types.ProcessingTier

// ProcessingStep is an alias for types.ProcessingStep
type ProcessingStep = types.ProcessingStep

// NewProcessingStep creates a new processing step record.
var NewProcessingStep = types.NewProcessingStep

// Attachment represents an email attachment with its metadata and optional content.
type Attachment struct {
	// Identity
	Filename    string `json:"filename"`
	MimeType    string `json:"mime_type"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentHash string `json:"content_hash,omitempty"` // SHA-256

	// Position in email
	Position  int    `json:"position"`            // 0-indexed order in email
	ContentID string `json:"content_id,omitempty"` // MIME Content-ID for inline refs
	IsInline  bool   `json:"is_inline"`           // Referenced in HTML body

	// Content (may be nil if not loaded)
	Content []byte `json:"-"`

	// Special flags
	IsEmbeddedEmail bool `json:"is_embedded_email,omitempty"` // .eml/.msg file
}

// Classification represents the result of classifying an attachment.
type Classification struct {
	Tier       ProcessingTier `json:"tier"`
	Reason     string         `json:"reason"`      // Human-readable explanation
	Confidence float64        `json:"confidence"`  // 0.0 to 1.0
	Step       string         `json:"step"`        // Which classifier step produced this
}

// SourceAttachment represents a link between parent email and child attachment sources.
type SourceAttachment struct {
	ID             int64  `json:"id"`
	ParentSourceID int64  `json:"parent_source_id"`
	ChildSourceID  *int64 `json:"child_source_id,omitempty"` // NULL if skipped

	// Attachment metadata
	Filename    string `json:"filename"`
	MimeType    string `json:"mime_type,omitempty"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentHash string `json:"content_hash,omitempty"`

	// Position
	Position  int    `json:"position"`
	ContentID string `json:"content_id,omitempty"`
	IsInline  bool   `json:"is_inline"`

	// Classification
	ProcessingTier ProcessingTier   `json:"processing_tier"`
	TierReason     string           `json:"tier_reason,omitempty"`
	ProcessingSteps []ProcessingStep `json:"processing_steps,omitempty"`

	// Flags
	IsEmbeddedEmail bool `json:"is_embedded_email"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AttachmentResult contains the result of processing a single attachment.
type AttachmentResult struct {
	Attachment     *Attachment       `json:"attachment"`
	Classification *Classification   `json:"classification"`
	SourceID       *int64            `json:"source_id,omitempty"` // If stored as source
	LinkID         int64             `json:"link_id"`             // source_attachments.id
	Error          error             `json:"error,omitempty"`
}

// ExtractionResult contains all attachments extracted from an email.
type ExtractionResult struct {
	Attachments []*AttachmentResult `json:"attachments"`
	TotalCount  int                 `json:"total_count"`
	Processed   int                 `json:"processed"`   // auto_process + manual_process
	Skipped     int                 `json:"skipped"`     // auto_skip + manual_skip
	Pending     int                 `json:"pending"`     // pending_review
	Errors      int                 `json:"errors"`
}

// IsDefinitive returns true if this classification is definitive (not pending).
func (c *Classification) IsDefinitive() bool {
	return c.Tier != TierPendingReview
}
