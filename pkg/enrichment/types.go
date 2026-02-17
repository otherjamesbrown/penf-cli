// Package enrichment provides the content enrichment pipeline for Penfold.
// It processes ingested content through classification, entity resolution,
// type-specific extraction, and AI processing stages.
package enrichment

import (
	"encoding/json"
	"time"
)

// ContentType represents the primary content type classification.
type ContentType string

const (
	ContentTypeEmail      ContentType = "email"
	ContentTypeCalendar   ContentType = "calendar"
	ContentTypeDocument   ContentType = "document"
	ContentTypeAttachment ContentType = "attachment"
)

// ContentSubtype represents specific subtypes within a content type.
// Format: "type" or "type/detail" (e.g., "thread", "notification/jira", "invite")
type ContentSubtype string

// Email subtypes
const (
	SubtypeEmailThread        ContentSubtype = "thread"
	SubtypeEmailForward       ContentSubtype = "forward"
	SubtypeEmailStandalone    ContentSubtype = "standalone"
	SubtypeNotificationJira   ContentSubtype = "notification/jira"
	SubtypeNotificationGoogle ContentSubtype = "notification/google"
	SubtypeNotificationSlack  ContentSubtype = "notification/slack"
	SubtypeNotificationOther  ContentSubtype = "notification/other"
)

// Calendar subtypes
const (
	SubtypeCalendarInvite       ContentSubtype = "invite"
	SubtypeCalendarCancellation ContentSubtype = "cancellation"
	SubtypeCalendarUpdate       ContentSubtype = "update"
	SubtypeCalendarResponse     ContentSubtype = "response"
)

// Document subtypes
const (
	SubtypeDocumentGoogleDoc ContentSubtype = "google_doc"
	SubtypeDocumentPDF       ContentSubtype = "pdf"
	SubtypeDocumentOffice    ContentSubtype = "office"
)

// Attachment subtypes
const (
	SubtypeAttachmentDocument    ContentSubtype = "document"
	SubtypeAttachmentSpreadsheet ContentSubtype = "spreadsheet"
	SubtypeAttachmentImage       ContentSubtype = "image"
	SubtypeAttachmentOther       ContentSubtype = "other"
)

// ProcessingProfile determines what AI processing is applied.
type ProcessingProfile string

const (
	ProfileFullAI        ProcessingProfile = "full_ai"
	ProfileFullAIChunked ProcessingProfile = "full_ai_chunked"
	ProfileMetadataOnly  ProcessingProfile = "metadata_only"
	ProfileStateTracking ProcessingProfile = "state_tracking"
	ProfileStructureOnly ProcessingProfile = "structure_only"
	ProfileOCRIfText     ProcessingProfile = "ocr_if_text"
)

// SourceSystem represents the originating system for the content.
type SourceSystem string

const (
	SourceSystemJira            SourceSystem = "jira"
	SourceSystemAha             SourceSystem = "aha"
	SourceSystemGoogleDocs      SourceSystem = "google_docs"
	SourceSystemWebex           SourceSystem = "webex"
	SourceSystemSmartsheet      SourceSystem = "smartsheet"
	SourceSystemOutlookCalendar SourceSystem = "outlook_calendar"
	SourceSystemAutoReply       SourceSystem = "auto_reply"
	SourceSystemHumanEmail      SourceSystem = "human_email"
	SourceSystemUnknown         SourceSystem = "unknown"
)

// EnrichmentStatus represents the processing state.
type EnrichmentStatus string

const (
	StatusPending      EnrichmentStatus = "pending"
	StatusClassifying  EnrichmentStatus = "classifying"
	StatusEnriching    EnrichmentStatus = "enriching"
	StatusExtracting   EnrichmentStatus = "extracting"
	StatusAIProcessing EnrichmentStatus = "ai_processing"
	StatusCompleted    EnrichmentStatus = "completed"
	StatusFailed       EnrichmentStatus = "failed"
	StatusSkipped      EnrichmentStatus = "skipped"
)

// Classification holds the result of content classification (Stage 1).
type Classification struct {
	ContentType  ContentType       `json:"content_type"`
	Subtype      ContentSubtype    `json:"subtype"`
	Profile      ProcessingProfile `json:"processing_profile"`
	Confidence   float32           `json:"confidence"`
	Reason       string            `json:"reason,omitempty"`
	DetectedVia  string            `json:"detected_via,omitempty"` // Which heuristic matched
	RulesPriority int              `json:"rules_priority,omitempty"`
}

// Participant represents a participant in content (from/to/cc).
type Participant struct {
	Email       string `json:"email"`
	Name        string `json:"name,omitempty"`
	Role        string `json:"role"` // from, to, cc, bcc, organizer, attendee
	IsInternal  *bool  `json:"is_internal,omitempty"`
	AccountType string `json:"account_type,omitempty"` // person, role, distribution, bot, external_service
}

// ResolvedParticipant is a participant resolved to a person record.
type ResolvedParticipant struct {
	Participant
	PersonID   *int64  `json:"person_id,omitempty"`
	Confidence float32 `json:"confidence,omitempty"`
	Source     string  `json:"source,omitempty"` // exact_match, alias, inferred
}

// ExtractedLink represents a URL extracted from content.
type ExtractedLink struct {
	URL         string `json:"url"`
	Text        string `json:"text,omitempty"`        // Anchor text if available
	Category    string `json:"category,omitempty"`    // google_doc, jira, github, etc.
	ServiceID   string `json:"service_id,omitempty"`  // Extracted ID (doc ID, ticket ID)
	Context     string `json:"context,omitempty"`     // Surrounding text
	IsInline    bool   `json:"is_inline,omitempty"`   // Inline vs block link
	SourceField string `json:"source_field,omitempty"` // body_text, body_html, header
}

// Enrichment represents the complete enrichment state for a source.
type Enrichment struct {
	ID       int64  `json:"id,omitempty"`
	SourceID int64  `json:"source_id"`
	TenantID string `json:"tenant_id"`

	// Classification (Stage 1)
	Classification Classification `json:"classification"`
	SourceSystem   SourceSystem   `json:"source_system"`

	// Processing status
	Status       EnrichmentStatus `json:"status"`
	CurrentStage string           `json:"current_stage,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
	RetryCount   int              `json:"retry_count"`

	// Common enrichment (Stage 2)
	Participants         []Participant         `json:"participants,omitempty"`
	ResolvedParticipants []ResolvedParticipant `json:"resolved_participants,omitempty"`
	ExtractedLinks       []ExtractedLink       `json:"extracted_links,omitempty"`
	ThreadID             string                `json:"thread_id,omitempty"`
	ProjectID            *int64                `json:"project_id,omitempty"`

	// Type-specific extraction (Stage 3)
	ExtractedData map[string]interface{} `json:"extracted_data,omitempty"`

	// AI processing (Stage 5)
	AIProcessed   bool       `json:"ai_processed"`
	AISkipReason  string     `json:"ai_skip_reason,omitempty"`
	AIProcessedAt *time.Time `json:"ai_processed_at,omitempty"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// StageResult records the result of a single processing stage.
type StageResult struct {
	ID            int64           `json:"id,omitempty"`
	EnrichmentID  int64           `json:"enrichment_id"`
	StageName     string          `json:"stage_name"`
	ProcessorName string          `json:"processor_name"`
	Status        string          `json:"status"` // pending, running, completed, failed, skipped
	InputData     json.RawMessage `json:"input_data,omitempty"`
	OutputData    json.RawMessage `json:"output_data,omitempty"`
	ErrorMessage  string          `json:"error_message,omitempty"`
	StartedAt     *time.Time      `json:"started_at,omitempty"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
	DurationMs    int             `json:"duration_ms,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// SkipAI returns true if this profile should skip AI processing.
func (p ProcessingProfile) SkipAI() bool {
	switch p {
	case ProfileMetadataOnly, ProfileStateTracking, ProfileStructureOnly:
		return true
	default:
		return false
	}
}

// NeedsChunking returns true if content needs to be chunked before AI processing.
func (p ProcessingProfile) NeedsChunking() bool {
	return p == ProfileFullAIChunked
}

// IsNotification returns true if this is a notification subtype.
func (s ContentSubtype) IsNotification() bool {
	switch s {
	case SubtypeNotificationJira, SubtypeNotificationGoogle, SubtypeNotificationSlack, SubtypeNotificationOther:
		return true
	default:
		return false
	}
}

// IsCalendar returns true if this is a calendar subtype.
func (s ContentSubtype) IsCalendar() bool {
	switch s {
	case SubtypeCalendarInvite, SubtypeCalendarCancellation, SubtypeCalendarUpdate, SubtypeCalendarResponse:
		return true
	default:
		return false
	}
}
