package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	// TracerName is the name of the tracer for enrichment operations.
	TracerName = "enrichment"
)

// Span attribute keys
const (
	AttrTenantID      = "tenant_id"
	AttrSourceID      = "source_id"
	AttrBatchID       = "batch_id"
	AttrThreadID      = "thread_id"
	AttrContentType   = "content_type"
	AttrSubtype       = "content_subtype"
	AttrProfile       = "processing_profile"
	AttrStage         = "stage"
	AttrProcessor     = "processor"
	AttrDurationMs    = "duration_ms"
	AttrEntityType    = "entity_type"
	AttrEntityID      = "entity_id"
	AttrModel         = "model"
	AttrTemplateID    = "template_id"
	AttrInputTokens   = "input_tokens"
	AttrOutputTokens  = "output_tokens"
	AttrErrorType     = "error_type"
	AttrRetryable     = "retryable"
)

// Span names
const (
	SpanProcessSource      = "enrichment.process_source"
	SpanStageClassify      = "enrichment.stage.classification"
	SpanStageCommonEnrich  = "enrichment.stage.common_enrichment"
	SpanStageTypeSpecific  = "enrichment.stage.type_specific"
	SpanStageAIRouting     = "enrichment.stage.ai_routing"
	SpanStageAIProcessing  = "enrichment.stage.ai_processing"
	SpanEntityResolution   = "enrichment.entity_resolution"
	SpanLLMCall            = "enrichment.llm_call"
	SpanParseOutput        = "enrichment.parse_output"
)

// Tracer provides distributed tracing for enrichment operations.
type Tracer struct {
	tracer trace.Tracer
}

// NewTracer creates a new enrichment tracer.
func NewTracer() *Tracer {
	return &Tracer{
		tracer: otel.Tracer(TracerName),
	}
}

// StartSourceSpan starts a root span for processing a source.
func (t *Tracer) StartSourceSpan(ctx context.Context, tenantID string, sourceID int64, batchID string) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, SpanProcessSource,
		trace.WithAttributes(
			attribute.String(AttrTenantID, tenantID),
			attribute.Int64(AttrSourceID, sourceID),
		),
	)
	if batchID != "" {
		span.SetAttributes(attribute.String(AttrBatchID, batchID))
	}
	return ctx, span
}

// StartStageSpan starts a span for a pipeline stage.
func (t *Tracer) StartStageSpan(ctx context.Context, stage string) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("enrichment.stage.%s", stage)
	return t.tracer.Start(ctx, spanName,
		trace.WithAttributes(
			attribute.String(AttrStage, stage),
		),
	)
}

// StartProcessorSpan starts a span for a processor within a stage.
func (t *Tracer) StartProcessorSpan(ctx context.Context, processor string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, fmt.Sprintf("enrichment.processor.%s", processor),
		trace.WithAttributes(
			attribute.String(AttrProcessor, processor),
		),
	)
}

// StartEntityResolutionSpan starts a span for entity resolution.
func (t *Tracer) StartEntityResolutionSpan(ctx context.Context, entityType string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, SpanEntityResolution,
		trace.WithAttributes(
			attribute.String(AttrEntityType, entityType),
		),
	)
}

// StartLLMSpan starts a span for an LLM call.
func (t *Tracer) StartLLMSpan(ctx context.Context, model string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, SpanLLMCall,
		trace.WithAttributes(
			attribute.String(AttrModel, model),
		),
	)
}

// SpanHelper provides convenient methods for working with the current span.
type SpanHelper struct {
	span trace.Span
}

// NewSpanHelper creates a new span helper for the given span.
func NewSpanHelper(span trace.Span) *SpanHelper {
	return &SpanHelper{span: span}
}

// SetSourceInfo sets source-related attributes on the span.
func (h *SpanHelper) SetSourceInfo(tenantID string, sourceID int64, threadID string) {
	h.span.SetAttributes(
		attribute.String(AttrTenantID, tenantID),
		attribute.Int64(AttrSourceID, sourceID),
	)
	if threadID != "" {
		h.span.SetAttributes(attribute.String(AttrThreadID, threadID))
	}
}

// SetClassification sets classification attributes on the span.
func (h *SpanHelper) SetClassification(contentType, subtype, profile string) {
	h.span.SetAttributes(
		attribute.String(AttrContentType, contentType),
		attribute.String(AttrSubtype, subtype),
		attribute.String(AttrProfile, profile),
	)
}

// SetDuration sets the duration attribute.
func (h *SpanHelper) SetDuration(durationMs int64) {
	h.span.SetAttributes(attribute.Int64(AttrDurationMs, durationMs))
}

// SetProcessor sets the processor attribute.
func (h *SpanHelper) SetProcessor(processor string) {
	h.span.SetAttributes(attribute.String(AttrProcessor, processor))
}

// SetEntityResolution sets entity resolution attributes.
func (h *SpanHelper) SetEntityResolution(entityType string, entityID int64) {
	h.span.SetAttributes(
		attribute.String(AttrEntityType, entityType),
		attribute.Int64(AttrEntityID, entityID),
	)
}

// SetLLMResult sets LLM result attributes.
func (h *SpanHelper) SetLLMResult(inputTokens, outputTokens int, latencyMs int64) {
	h.span.SetAttributes(
		attribute.Int(AttrInputTokens, inputTokens),
		attribute.Int(AttrOutputTokens, outputTokens),
		attribute.Int64(AttrDurationMs, latencyMs),
	)
}

// SetTemplate sets template attributes.
func (h *SpanHelper) SetTemplate(templateID int64, version string) {
	h.span.SetAttributes(attribute.Int64(AttrTemplateID, templateID))
	if version != "" {
		h.span.SetAttributes(attribute.String("template_version", version))
	}
}

// SetError records an error on the span.
func (h *SpanHelper) SetError(err error, errorType string, retryable bool) {
	h.span.SetStatus(codes.Error, err.Error())
	h.span.SetAttributes(
		attribute.String(AttrErrorType, errorType),
		attribute.Bool(AttrRetryable, retryable),
	)
	h.span.RecordError(err)
}

// SetSuccess marks the span as successful.
func (h *SpanHelper) SetSuccess() {
	h.span.SetStatus(codes.Ok, "")
}

// AddEvent adds an event to the span.
func (h *SpanHelper) AddEvent(name string, attrs ...attribute.KeyValue) {
	h.span.AddEvent(name, trace.WithAttributes(attrs...))
}

// GetTraceID returns the trace ID from the context.
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// GetSpanID returns the span ID from the context.
func GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasSpanID() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// InjectTraceContext extracts trace context for propagation (e.g., to queue messages).
func InjectTraceContext(ctx context.Context) map[string]string {
	headers := make(map[string]string)
	traceID := GetTraceID(ctx)
	spanID := GetSpanID(ctx)
	if traceID != "" {
		headers["trace_id"] = traceID
	}
	if spanID != "" {
		headers["span_id"] = spanID
	}
	return headers
}
