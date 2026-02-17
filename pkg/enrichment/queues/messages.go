// Package queues provides queue infrastructure for the enrichment pipeline.
package queues

import (
	"encoding/json"
	"time"
)

// Priority levels for queue messages.
type Priority int

const (
	PriorityLow    Priority = 0 // Backfill, re-enrichment
	PriorityNormal Priority = 1 // Batch ingest
	PriorityHigh   Priority = 2 // Real-time sync
)

// MessageType identifies the type of queue message.
type MessageType string

const (
	MessageTypeIngest     MessageType = "ingest"
	MessageTypeEnrichment MessageType = "enrichment"
	MessageTypeAI         MessageType = "ai"
)

// Message is the base interface for all queue messages.
type Message interface {
	// GetSourceID returns the source ID being processed.
	GetSourceID() int64
	// GetTenantID returns the tenant ID.
	GetTenantID() string
	// GetPriority returns the message priority.
	GetPriority() Priority
	// GetMessageType returns the message type.
	GetMessageType() MessageType
	// GetBatchID returns the batch ID if part of a batch.
	GetBatchID() string
}

// IngestMessage triggers enrichment after source is stored.
type IngestMessage struct {
	SourceID   int64       `json:"source_id"`
	TenantID   string      `json:"tenant_id"`
	SourceType string      `json:"source_type"` // email, calendar, document
	Priority   Priority    `json:"priority"`
	IngestedAt time.Time   `json:"ingested_at"`
	BatchID    string      `json:"batch_id,omitempty"`
	Metadata   interface{} `json:"metadata,omitempty"` // Source-specific metadata
}

func (m *IngestMessage) GetSourceID() int64        { return m.SourceID }
func (m *IngestMessage) GetTenantID() string       { return m.TenantID }
func (m *IngestMessage) GetPriority() Priority     { return m.Priority }
func (m *IngestMessage) GetMessageType() MessageType { return MessageTypeIngest }
func (m *IngestMessage) GetBatchID() string        { return m.BatchID }

// EnrichmentMessage triggers AI processing after enrichment.
type EnrichmentMessage struct {
	SourceID          int64     `json:"source_id"`
	TenantID          string    `json:"tenant_id"`
	ContentType       string    `json:"content_type"`
	ContentSubtype    string    `json:"content_subtype"`
	ProcessingProfile string    `json:"processing_profile"`
	ThreadID          string    `json:"thread_id,omitempty"`
	IsLatestInThread  bool      `json:"is_latest_in_thread"`
	EnrichedAt        time.Time `json:"enriched_at"`
	Priority          Priority  `json:"priority"`
	BatchID           string    `json:"batch_id,omitempty"`
}

func (m *EnrichmentMessage) GetSourceID() int64        { return m.SourceID }
func (m *EnrichmentMessage) GetTenantID() string       { return m.TenantID }
func (m *EnrichmentMessage) GetPriority() Priority     { return m.Priority }
func (m *EnrichmentMessage) GetMessageType() MessageType { return MessageTypeEnrichment }
func (m *EnrichmentMessage) GetBatchID() string        { return m.BatchID }

// AIMessage triggers AI extraction and embedding.
type AIMessage struct {
	SourceID          int64     `json:"source_id"`
	TenantID          string    `json:"tenant_id"`
	ContentType       string    `json:"content_type"`
	ContentSubtype    string    `json:"content_subtype"`
	ProcessingProfile string    `json:"processing_profile"`
	ThreadID          string    `json:"thread_id,omitempty"`
	IsLatestInThread  bool      `json:"is_latest_in_thread"`
	ProjectID         *int64    `json:"project_id,omitempty"`
	EnrichmentID      int64     `json:"enrichment_id"`
	QueuedAt          time.Time `json:"queued_at"`
	Priority          Priority  `json:"priority"`
	BatchID           string    `json:"batch_id,omitempty"`
}

func (m *AIMessage) GetSourceID() int64        { return m.SourceID }
func (m *AIMessage) GetTenantID() string       { return m.TenantID }
func (m *AIMessage) GetPriority() Priority     { return m.Priority }
func (m *AIMessage) GetMessageType() MessageType { return MessageTypeAI }
func (m *AIMessage) GetBatchID() string        { return m.BatchID }

// QueuedMessage wraps a message with queue metadata.
type QueuedMessage struct {
	ID           string          `json:"id"`
	Message      json.RawMessage `json:"message"`
	MessageType  MessageType     `json:"message_type"`
	Priority     Priority        `json:"priority"`
	RetryCount   int             `json:"retry_count"`
	EnqueuedAt   time.Time       `json:"enqueued_at"`
	VisibleAfter time.Time       `json:"visible_after,omitempty"` // For delayed visibility
}

// ParseMessage parses the raw message based on message type.
func (qm *QueuedMessage) ParseMessage() (Message, error) {
	switch qm.MessageType {
	case MessageTypeIngest:
		var msg IngestMessage
		if err := json.Unmarshal(qm.Message, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	case MessageTypeEnrichment:
		var msg EnrichmentMessage
		if err := json.Unmarshal(qm.Message, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	case MessageTypeAI:
		var msg AIMessage
		if err := json.Unmarshal(qm.Message, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	default:
		return nil, ErrUnknownMessageType
	}
}

// Queue defines the interface for a message queue.
type Queue interface {
	// Name returns the queue name.
	Name() string

	// Enqueue adds a message to the queue.
	Enqueue(msg Message) error

	// EnqueueBatch adds multiple messages to the queue.
	EnqueueBatch(msgs []Message) error

	// Dequeue retrieves messages from the queue.
	// Returns up to maxMessages, blocks for timeout.
	Dequeue(maxMessages int, timeout time.Duration) ([]*QueuedMessage, error)

	// Ack acknowledges successful processing of a message.
	Ack(messageID string) error

	// Nack indicates processing failure, message will be retried.
	Nack(messageID string) error

	// MoveToDeadLetter moves a message to the dead letter queue.
	MoveToDeadLetter(messageID string, reason string) error

	// Depth returns the current queue depth.
	Depth() (int64, error)

	// Close closes the queue connection.
	Close() error
}

// QueueConfig configures queue behavior.
type QueueConfig struct {
	Name              string        `yaml:"name"`
	VisibilityTimeout time.Duration `yaml:"visibility_timeout"`
	MaxRetries        int           `yaml:"max_retries"`
	RetentionPeriod   time.Duration `yaml:"retention_period"`
}

// DefaultQueueConfigs returns default configurations for each queue type.
func DefaultQueueConfigs() map[string]QueueConfig {
	return map[string]QueueConfig{
		"enrichment:ingest": {
			Name:              "enrichment:ingest",
			VisibilityTimeout: 60 * time.Second,
			MaxRetries:        3,
			RetentionPeriod:   24 * time.Hour,
		},
		"enrichment:process": {
			Name:              "enrichment:process",
			VisibilityTimeout: 120 * time.Second,
			MaxRetries:        3,
			RetentionPeriod:   24 * time.Hour,
		},
		"enrichment:ai": {
			Name:              "enrichment:ai",
			VisibilityTimeout: 300 * time.Second, // AI calls can be slow
			MaxRetries:        3,
			RetentionPeriod:   24 * time.Hour,
		},
	}
}

// Verify interface compliance
var _ Message = (*IngestMessage)(nil)
var _ Message = (*EnrichmentMessage)(nil)
var _ Message = (*AIMessage)(nil)
