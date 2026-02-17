// Package observability provides event schemas, metrics, and tracing for the enrichment pipeline.
package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Event channels for Redis pub/sub
const (
	ChannelStageCompleted  = "events.enrichment.stage_completed"
	ChannelClassified      = "events.enrichment.classified"
	ChannelEntityResolved  = "events.enrichment.entity_resolved"
	ChannelAIProcessed     = "events.enrichment.ai_processed"
	ChannelError           = "events.enrichment.error"
	ChannelQueueMetrics    = "events.enrichment.queue_metrics"
)

// EnrichmentStageEvent is emitted after each pipeline stage completes.
type EnrichmentStageEvent struct {
	EventID       string    `json:"event_id"`
	TenantID      string    `json:"tenant_id"`
	SourceID      int64     `json:"source_id"`
	BatchID       string    `json:"batch_id,omitempty"`
	TraceID       string    `json:"trace_id,omitempty"`
	Stage         string    `json:"stage"`
	Status        string    `json:"status"`
	DurationMs    int64     `json:"duration_ms"`
	ProcessorName string    `json:"processor_name"`
	Outputs       []string  `json:"outputs,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

// StageStatus values
const (
	StageStatusCompleted = "completed"
	StageStatusFailed    = "failed"
	StageStatusSkipped   = "skipped"
)

// Stage names
const (
	StageClassification   = "classification"
	StageCommonEnrichment = "common_enrichment"
	StageTypeSpecific     = "type_specific"
	StageAIRouting        = "ai_routing"
	StageAIProcessing     = "ai_processing"
)

// NewEnrichmentStageEvent creates a new stage event with a generated ID.
func NewEnrichmentStageEvent(tenantID string, sourceID int64, stage, status, processor string, durationMs int64) *EnrichmentStageEvent {
	return &EnrichmentStageEvent{
		EventID:       uuid.New().String(),
		TenantID:      tenantID,
		SourceID:      sourceID,
		Stage:         stage,
		Status:        status,
		DurationMs:    durationMs,
		ProcessorName: processor,
		Timestamp:     time.Now(),
	}
}

// ClassificationEvent is emitted when content is classified.
type ClassificationEvent struct {
	EventID           string    `json:"event_id"`
	TenantID          string    `json:"tenant_id"`
	SourceID          int64     `json:"source_id"`
	TraceID           string    `json:"trace_id,omitempty"`
	ContentType       string    `json:"content_type"`
	ContentSubtype    string    `json:"content_subtype"`
	ProcessingProfile string    `json:"processing_profile"`
	DetectionMethod   string    `json:"detection_method"`
	Confidence        float64   `json:"confidence"`
	Timestamp         time.Time `json:"timestamp"`
}

// NewClassificationEvent creates a new classification event.
func NewClassificationEvent(tenantID string, sourceID int64, contentType, subtype, profile, method string, confidence float64) *ClassificationEvent {
	return &ClassificationEvent{
		EventID:           uuid.New().String(),
		TenantID:          tenantID,
		SourceID:          sourceID,
		ContentType:       contentType,
		ContentSubtype:    subtype,
		ProcessingProfile: profile,
		DetectionMethod:   method,
		Confidence:        confidence,
		Timestamp:         time.Now(),
	}
}

// EntityResolutionEvent is emitted when an entity is resolved.
type EntityResolutionEvent struct {
	EventID     string    `json:"event_id"`
	TenantID    string    `json:"tenant_id"`
	SourceID    int64     `json:"source_id"`
	TraceID     string    `json:"trace_id,omitempty"`
	EntityType  string    `json:"entity_type"`
	Action      string    `json:"action"`
	EntityID    int64     `json:"entity_id"`
	InputValue  string    `json:"input_value"`
	Confidence  float64   `json:"confidence"`
	NeedsReview bool      `json:"needs_review"`
	Timestamp   time.Time `json:"timestamp"`
}

// Entity resolution actions
const (
	EntityActionResolved        = "resolved"
	EntityActionCreated         = "created"
	EntityActionFlaggedDuplicate = "flagged_duplicate"
)

// Entity types
const (
	EntityTypePerson  = "person"
	EntityTypeTeam    = "team"
	EntityTypeProject = "project"
)

// NewEntityResolutionEvent creates a new entity resolution event.
func NewEntityResolutionEvent(tenantID string, sourceID int64, entityType, action string, entityID int64, inputValue string, confidence float64, needsReview bool) *EntityResolutionEvent {
	return &EntityResolutionEvent{
		EventID:     uuid.New().String(),
		TenantID:    tenantID,
		SourceID:    sourceID,
		EntityType:  entityType,
		Action:      action,
		EntityID:    entityID,
		InputValue:  inputValue,
		Confidence:  confidence,
		NeedsReview: needsReview,
		Timestamp:   time.Now(),
	}
}

// AIProcessingEvent is emitted after AI processing completes.
type AIProcessingEvent struct {
	EventID         string    `json:"event_id"`
	TenantID        string    `json:"tenant_id"`
	SourceID        int64     `json:"source_id"`
	TraceID         string    `json:"trace_id,omitempty"`
	ExtractionRunID int64     `json:"extraction_run_id"`
	Operation       string    `json:"operation"`
	Model           string    `json:"model"`
	TemplateID      int64     `json:"template_id,omitempty"`
	TemplateVersion string    `json:"template_version,omitempty"`
	InputTokens     int       `json:"input_tokens"`
	OutputTokens    int       `json:"output_tokens"`
	LatencyMs       int64     `json:"latency_ms"`
	ParseSuccess    bool      `json:"parse_success"`
	Timestamp       time.Time `json:"timestamp"`
}

// AI operations
const (
	AIOperationEmbed     = "embed"
	AIOperationSummarize = "summarize"
	AIOperationExtract   = "extract"
)

// NewAIProcessingEvent creates a new AI processing event.
func NewAIProcessingEvent(tenantID string, sourceID int64, runID int64, operation, model string, inputTokens, outputTokens int, latencyMs int64, parseSuccess bool) *AIProcessingEvent {
	return &AIProcessingEvent{
		EventID:         uuid.New().String(),
		TenantID:        tenantID,
		SourceID:        sourceID,
		ExtractionRunID: runID,
		Operation:       operation,
		Model:           model,
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		LatencyMs:       latencyMs,
		ParseSuccess:    parseSuccess,
		Timestamp:       time.Now(),
	}
}

// EnrichmentErrorEvent is emitted when an error occurs.
type EnrichmentErrorEvent struct {
	EventID       string            `json:"event_id"`
	TenantID      string            `json:"tenant_id"`
	SourceID      int64             `json:"source_id"`
	BatchID       string            `json:"batch_id,omitempty"`
	TraceID       string            `json:"trace_id,omitempty"`
	Stage         string            `json:"stage"`
	ProcessorName string            `json:"processor_name"`
	ErrorType     string            `json:"error_type"`
	ErrorMessage  string            `json:"error_message"`
	Retryable     bool              `json:"retryable"`
	RetryCount    int               `json:"retry_count"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Timestamp     time.Time         `json:"timestamp"`
}

// Error types
const (
	ErrorTypeParseError     = "parse_error"
	ErrorTypeTimeout        = "timeout"
	ErrorTypeQuotaExceeded  = "quota_exceeded"
	ErrorTypeValidation     = "validation"
	ErrorTypeInternal       = "internal"
	ErrorTypeExternal       = "external_service"
	ErrorTypeRateLimit      = "rate_limit"
)

// NewEnrichmentErrorEvent creates a new error event.
func NewEnrichmentErrorEvent(tenantID string, sourceID int64, stage, processor, errorType, errorMsg string, retryable bool, retryCount int) *EnrichmentErrorEvent {
	return &EnrichmentErrorEvent{
		EventID:       uuid.New().String(),
		TenantID:      tenantID,
		SourceID:      sourceID,
		Stage:         stage,
		ProcessorName: processor,
		ErrorType:     errorType,
		ErrorMessage:  errorMsg,
		Retryable:     retryable,
		RetryCount:    retryCount,
		Timestamp:     time.Now(),
	}
}

// QueueMetricsEvent provides periodic queue status updates.
type QueueMetricsEvent struct {
	EventID       string    `json:"event_id"`
	Queue         string    `json:"queue"`
	TenantID      string    `json:"tenant_id,omitempty"`
	Depth         int64     `json:"depth"`
	Processing    int64     `json:"processing"`
	DLQDepth      int64     `json:"dlq_depth"`
	OldestItemAge int64     `json:"oldest_item_age_seconds"`
	Timestamp     time.Time `json:"timestamp"`
}

// EventPublisher publishes events to Redis channels.
type EventPublisher interface {
	Publish(ctx context.Context, channel string, event interface{}) error
	Close() error
}

// RedisEventPublisher publishes events to Redis.
type RedisEventPublisher struct {
	publish func(ctx context.Context, channel string, message interface{}) error
}

// NewRedisEventPublisher creates a publisher using a Redis publish function.
func NewRedisEventPublisher(publishFn func(ctx context.Context, channel string, message interface{}) error) *RedisEventPublisher {
	return &RedisEventPublisher{publish: publishFn}
}

// Publish publishes an event to a Redis channel.
func (p *RedisEventPublisher) Publish(ctx context.Context, channel string, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	return p.publish(ctx, channel, data)
}

// Close is a no-op for Redis publisher.
func (p *RedisEventPublisher) Close() error {
	return nil
}

// NoOpEventPublisher discards all events (for testing or disabled observability).
type NoOpEventPublisher struct{}

// Publish does nothing.
func (p *NoOpEventPublisher) Publish(ctx context.Context, channel string, event interface{}) error {
	return nil
}

// Close does nothing.
func (p *NoOpEventPublisher) Close() error {
	return nil
}

// EventEmitter provides a convenient interface for emitting enrichment events.
type EventEmitter struct {
	publisher EventPublisher
}

// NewEventEmitter creates a new event emitter.
func NewEventEmitter(publisher EventPublisher) *EventEmitter {
	return &EventEmitter{publisher: publisher}
}

// EmitStageCompleted emits a stage completion event.
func (e *EventEmitter) EmitStageCompleted(ctx context.Context, event *EnrichmentStageEvent) error {
	return e.publisher.Publish(ctx, ChannelStageCompleted, event)
}

// EmitClassified emits a classification event.
func (e *EventEmitter) EmitClassified(ctx context.Context, event *ClassificationEvent) error {
	return e.publisher.Publish(ctx, ChannelClassified, event)
}

// EmitEntityResolved emits an entity resolution event.
func (e *EventEmitter) EmitEntityResolved(ctx context.Context, event *EntityResolutionEvent) error {
	return e.publisher.Publish(ctx, ChannelEntityResolved, event)
}

// EmitAIProcessed emits an AI processing event.
func (e *EventEmitter) EmitAIProcessed(ctx context.Context, event *AIProcessingEvent) error {
	return e.publisher.Publish(ctx, ChannelAIProcessed, event)
}

// EmitError emits an error event.
func (e *EventEmitter) EmitError(ctx context.Context, event *EnrichmentErrorEvent) error {
	return e.publisher.Publish(ctx, ChannelError, event)
}

// Close closes the underlying publisher.
func (e *EventEmitter) Close() error {
	return e.publisher.Close()
}
