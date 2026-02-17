// Package events provides event publishing for the email ingest pipeline.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/otherjamesbrown/penf-cli/pkg/ingest/eml"
	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

// Redis channel for manual email events
const (
	ChannelManualEmailIngested = "events.manual_email.ingested"
	ChannelIngestJobProgress   = "events.ingest_job.progress"
	ChannelIngestJobCompleted  = "events.ingest_job.completed"
	ChannelAttachmentIngested  = "events.attachment.ingested"
)

// BaseEvent contains common fields for all events.
type BaseEvent struct {
	EventType     string    `json:"event_type"`
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID *string   `json:"correlation_id,omitempty"`
	Source        string    `json:"source"`
	Version       string    `json:"version"`
}

// NewBaseEvent creates a BaseEvent with sensible defaults.
func NewBaseEvent(eventType string) BaseEvent {
	return BaseEvent{
		EventType: eventType,
		Timestamp: time.Now().UTC(),
		Source:    "penfold",
		Version:   "1.0",
	}
}

// ManualEmailIngestedEvent is published when an email is successfully ingested.
type ManualEmailIngestedEvent struct {
	BaseEvent

	// Identifiers
	SourceID  int64  `json:"source_id"`
	TenantID  string `json:"tenant_id"`
	MessageID string `json:"message_id"`
	JobID     string `json:"job_id"`

	// Email metadata
	FromEmail      string    `json:"from_email"`
	FromName       *string   `json:"from_name,omitempty"`
	ToEmails       []string  `json:"to_emails"`
	CcEmails       []string  `json:"cc_emails"`
	Subject        *string   `json:"subject,omitempty"`
	EmailDate      time.Time `json:"email_date"`
	DateIsFallback bool      `json:"date_is_fallback"`

	// Threading
	InReplyTo *string `json:"in_reply_to,omitempty"`
	ThreadID  *string `json:"thread_id,omitempty"`

	// Content info
	HasAttachments  bool   `json:"has_attachments"`
	AttachmentCount int    `json:"attachment_count"`
	ContentHash     string `json:"content_hash"`

	// Source tracking
	SourceTag        string   `json:"source_tag"`
	OriginalFilePath string   `json:"original_file_path"`
	Labels           []string `json:"labels"`
}

// IngestJobProgressEvent is published periodically during batch processing.
type IngestJobProgressEvent struct {
	BaseEvent

	JobID    string `json:"job_id"`
	TenantID string `json:"tenant_id"`

	TotalFiles     int `json:"total_files"`
	ProcessedCount int `json:"processed_count"`
	ImportedCount  int `json:"imported_count"`
	SkippedCount   int `json:"skipped_count"`
	FailedCount    int `json:"failed_count"`

	CurrentFile               *string  `json:"current_file,omitempty"`
	ElapsedSeconds            float64  `json:"elapsed_seconds"`
	EstimatedRemainingSeconds *float64 `json:"estimated_remaining_seconds,omitempty"`
	Status                    string   `json:"status"`
}

// IngestJobCompletedEvent is published when a batch ingest job finishes.
type IngestJobCompletedEvent struct {
	BaseEvent

	JobID     string `json:"job_id"`
	TenantID  string `json:"tenant_id"`
	SourceTag string `json:"source_tag"`

	TotalFiles    int `json:"total_files"`
	ImportedCount int `json:"imported_count"`
	SkippedCount  int `json:"skipped_count"`
	FailedCount   int `json:"failed_count"`

	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
	DurationSeconds float64   `json:"duration_seconds"`

	Success     bool   `json:"success"`
	FinalStatus string `json:"final_status"`
}

// Publisher publishes ingest events to Redis.
type Publisher struct {
	client *redis.Client
	logger logging.Logger
}

// PublisherConfig holds Redis connection configuration.
type PublisherConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// NewPublisher creates a new event publisher.
func NewPublisher(client *redis.Client, logger logging.Logger) *Publisher {
	return &Publisher{
		client: client,
		logger: logger.With(logging.F("component", "event_publisher")),
	}
}

// NewPublisherFromConfig creates a publisher with a new Redis connection.
func NewPublisherFromConfig(cfg PublisherConfig, logger logging.Logger) (*Publisher, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return NewPublisher(client, logger), nil
}

// PublishManualEmailIngested publishes an event for a successfully ingested email.
func (p *Publisher) PublishManualEmailIngested(ctx context.Context, params EmailIngestedParams) error {
	event := ManualEmailIngestedEvent{
		BaseEvent:        NewBaseEvent("manual_email.ingested"),
		SourceID:         params.SourceID,
		TenantID:         params.TenantID,
		MessageID:        params.Email.MessageID,
		JobID:            params.JobID,
		FromEmail:        params.Email.From.Email,
		ToEmails:         params.Email.ToAddresses(),
		CcEmails:         params.Email.CcAddresses(),
		EmailDate:        params.Email.Date,
		DateIsFallback:   params.Email.DateFallback,
		HasAttachments:   params.Email.HasAttachments(),
		AttachmentCount:  params.Email.AttachmentCount(),
		ContentHash:      params.Email.ContentHash,
		SourceTag:        params.SourceTag,
		OriginalFilePath: params.Email.FilePath,
		Labels:           params.Labels,
	}

	// Set optional fields
	if params.Email.From.Name != "" {
		event.FromName = &params.Email.From.Name
	}
	if params.Email.Subject != "" {
		event.Subject = &params.Email.Subject
	}
	if params.Email.InReplyTo != "" {
		event.InReplyTo = &params.Email.InReplyTo
	}

	return p.publish(ctx, ChannelManualEmailIngested, event)
}

// PublishJobProgress publishes a progress update for a batch ingest job.
func (p *Publisher) PublishJobProgress(ctx context.Context, params JobProgressParams) error {
	event := IngestJobProgressEvent{
		BaseEvent:                 NewBaseEvent("ingest_job.progress"),
		JobID:                     params.JobID,
		TenantID:                  params.TenantID,
		TotalFiles:                params.TotalFiles,
		ProcessedCount:            params.ProcessedCount,
		ImportedCount:             params.ImportedCount,
		SkippedCount:              params.SkippedCount,
		FailedCount:               params.FailedCount,
		CurrentFile:               params.CurrentFile,
		ElapsedSeconds:            params.ElapsedSeconds,
		EstimatedRemainingSeconds: params.EstimatedRemainingSeconds,
		Status:                    params.Status,
	}

	return p.publish(ctx, ChannelIngestJobProgress, event)
}

// PublishJobCompleted publishes a completion event for a batch ingest job.
func (p *Publisher) PublishJobCompleted(ctx context.Context, params JobCompletedParams) error {
	event := IngestJobCompletedEvent{
		BaseEvent:       NewBaseEvent("ingest_job.completed"),
		JobID:           params.JobID,
		TenantID:        params.TenantID,
		SourceTag:       params.SourceTag,
		TotalFiles:      params.TotalFiles,
		ImportedCount:   params.ImportedCount,
		SkippedCount:    params.SkippedCount,
		FailedCount:     params.FailedCount,
		StartedAt:       params.StartedAt,
		CompletedAt:     params.CompletedAt,
		DurationSeconds: params.CompletedAt.Sub(params.StartedAt).Seconds(),
		Success:         params.Success,
		FinalStatus:     params.FinalStatus,
	}

	return p.publish(ctx, ChannelIngestJobCompleted, event)
}

// PublishAttachmentIngested publishes an event for a successfully stored attachment.
func (p *Publisher) PublishAttachmentIngested(ctx context.Context, params AttachmentIngestedParams) error {
	event := AttachmentIngestedEvent{
		BaseEvent:       NewBaseEvent("attachment.ingested"),
		SourceID:        params.SourceID,
		ParentSourceID:  params.ParentSourceID,
		TenantID:        params.TenantID,
		Filename:        params.Filename,
		MimeType:        params.MimeType,
		SizeBytes:       params.SizeBytes,
		IsEmbeddedEmail: params.IsEmbeddedEmail,
	}

	return p.publish(ctx, ChannelAttachmentIngested, event)
}

// publish serializes and publishes an event to Redis.
func (p *Publisher) publish(ctx context.Context, channel string, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := p.client.Publish(ctx, channel, data).Err(); err != nil {
		p.logger.Error("Failed to publish event",
			logging.Err(err),
			logging.F("channel", channel))
		return fmt.Errorf("failed to publish to %s: %w", channel, err)
	}

	p.logger.Debug("Event published",
		logging.F("channel", channel),
		logging.F("payload_size", len(data)))

	return nil
}

// Close closes the Redis connection.
func (p *Publisher) Close() error {
	return p.client.Close()
}

// EmailIngestedParams contains parameters for publishing an email ingested event.
type EmailIngestedParams struct {
	SourceID  int64
	TenantID  string
	JobID     string
	Email     *eml.ParsedEmail
	SourceTag string
	Labels    []string
}

// JobProgressParams contains parameters for publishing job progress.
type JobProgressParams struct {
	JobID                     string
	TenantID                  string
	TotalFiles                int
	ProcessedCount            int
	ImportedCount             int
	SkippedCount              int
	FailedCount               int
	CurrentFile               *string
	ElapsedSeconds            float64
	EstimatedRemainingSeconds *float64
	Status                    string
}

// JobCompletedParams contains parameters for publishing job completion.
type JobCompletedParams struct {
	JobID       string
	TenantID    string
	SourceTag   string
	TotalFiles  int
	ImportedCount int
	SkippedCount  int
	FailedCount   int
	StartedAt   time.Time
	CompletedAt time.Time
	Success     bool
	FinalStatus string
}

// AttachmentIngestedEvent is published when an attachment is stored as a source.
type AttachmentIngestedEvent struct {
	BaseEvent

	// Identifiers
	SourceID       int64  `json:"source_id"`        // Attachment source ID
	ParentSourceID int64  `json:"parent_source_id"` // Parent email source ID
	TenantID       string `json:"tenant_id"`

	// Attachment metadata
	Filename  string `json:"filename"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`

	// Special flags
	IsEmbeddedEmail bool `json:"is_embedded_email"`
}

// AttachmentIngestedParams contains parameters for publishing an attachment event.
type AttachmentIngestedParams struct {
	SourceID        int64
	ParentSourceID  int64
	TenantID        string
	Filename        string
	MimeType        string
	SizeBytes       int64
	IsEmbeddedEmail bool
}
