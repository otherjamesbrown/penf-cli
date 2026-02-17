package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// EnrichmentMetrics holds all Prometheus metrics for the enrichment pipeline.
type EnrichmentMetrics struct {
	// Queue metrics
	QueueItemsTotal   *prometheus.CounterVec
	QueueDepth        *prometheus.GaugeVec
	QueueWaitSeconds  *prometheus.HistogramVec
	DLQItemsTotal     *prometheus.CounterVec

	// Processing metrics
	ItemsProcessedTotal    *prometheus.CounterVec
	ProcessingSeconds      *prometheus.HistogramVec
	ClassificationsTotal   *prometheus.CounterVec

	// Entity resolution metrics
	EntityResolutionsTotal  *prometheus.CounterVec
	EntityConfidence        *prometheus.HistogramVec
	EntitiesPendingReview   *prometheus.GaugeVec

	// AI processing metrics
	AIOperationsTotal       *prometheus.CounterVec
	AILatencySeconds        *prometheus.HistogramVec
	AITokensTotal           *prometheus.CounterVec
	ExtractionParseSuccess  *prometheus.GaugeVec
}

// DefaultEnrichmentMetrics creates metrics with default configurations.
func DefaultEnrichmentMetrics() *EnrichmentMetrics {
	return NewEnrichmentMetrics(prometheus.DefaultRegisterer)
}

// NewEnrichmentMetrics creates a new set of enrichment metrics.
func NewEnrichmentMetrics(reg prometheus.Registerer) *EnrichmentMetrics {
	factory := promauto.With(reg)

	return &EnrichmentMetrics{
		// Queue metrics
		QueueItemsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "enrichment_queue_items_total",
				Help: "Total items entering each queue",
			},
			[]string{"queue", "tenant_id", "priority"},
		),
		QueueDepth: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "enrichment_queue_depth",
				Help: "Current queue depth",
			},
			[]string{"queue", "tenant_id", "priority"},
		),
		QueueWaitSeconds: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "enrichment_queue_wait_seconds",
				Help:    "Time spent in queue before pickup",
				Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 300, 600, 1800},
			},
			[]string{"queue", "tenant_id", "priority"},
		),
		DLQItemsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "enrichment_dlq_items_total",
				Help: "Total items added to dead letter queue",
			},
			[]string{"queue", "tenant_id", "error_type"},
		),

		// Processing metrics
		ItemsProcessedTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "enrichment_items_processed_total",
				Help: "Total items processed per stage",
			},
			[]string{"stage", "processor", "status", "tenant_id"},
		),
		ProcessingSeconds: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "enrichment_processing_seconds",
				Help:    "Processing latency per stage",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10},
			},
			[]string{"stage", "processor", "tenant_id"},
		),
		ClassificationsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "enrichment_classifications_total",
				Help: "Total classifications by type",
			},
			[]string{"content_type", "content_subtype", "processing_profile", "tenant_id"},
		),

		// Entity resolution metrics
		EntityResolutionsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "enrichment_entity_resolutions_total",
				Help: "Total entity resolutions",
			},
			[]string{"entity_type", "action", "tenant_id"},
		),
		EntityConfidence: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "enrichment_entity_confidence",
				Help:    "Entity resolution confidence scores",
				Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 0.95, 1.0},
			},
			[]string{"entity_type", "tenant_id"},
		),
		EntitiesPendingReview: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "enrichment_entities_pending_review",
				Help: "Entities pending review",
			},
			[]string{"entity_type", "tenant_id"},
		),

		// AI processing metrics
		AIOperationsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "enrichment_ai_operations_total",
				Help: "Total AI operations",
			},
			[]string{"operation", "model", "status", "tenant_id"},
		),
		AILatencySeconds: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "enrichment_ai_latency_seconds",
				Help:    "AI operation latency",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 15, 30, 60},
			},
			[]string{"operation", "model", "tenant_id"},
		),
		AITokensTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "enrichment_ai_tokens_total",
				Help: "Total tokens processed",
			},
			[]string{"direction", "model", "tenant_id"},
		),
		ExtractionParseSuccess: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "enrichment_extraction_parse_success_rate",
				Help: "Rolling extraction parse success rate",
			},
			[]string{"template_id", "tenant_id"},
		),
	}
}

// RecordQueueEnqueue records an item entering a queue.
func (m *EnrichmentMetrics) RecordQueueEnqueue(queue, tenantID, priority string) {
	m.QueueItemsTotal.WithLabelValues(queue, tenantID, priority).Inc()
}

// RecordQueueDepth sets the current queue depth.
func (m *EnrichmentMetrics) RecordQueueDepth(queue, tenantID, priority string, depth float64) {
	m.QueueDepth.WithLabelValues(queue, tenantID, priority).Set(depth)
}

// RecordQueueWait records the time an item spent in the queue.
func (m *EnrichmentMetrics) RecordQueueWait(queue, tenantID, priority string, seconds float64) {
	m.QueueWaitSeconds.WithLabelValues(queue, tenantID, priority).Observe(seconds)
}

// RecordDLQItem records an item added to the dead letter queue.
func (m *EnrichmentMetrics) RecordDLQItem(queue, tenantID, errorType string) {
	m.DLQItemsTotal.WithLabelValues(queue, tenantID, errorType).Inc()
}

// RecordProcessed records a processed item.
func (m *EnrichmentMetrics) RecordProcessed(stage, processor, status, tenantID string) {
	m.ItemsProcessedTotal.WithLabelValues(stage, processor, status, tenantID).Inc()
}

// RecordProcessingLatency records processing latency.
func (m *EnrichmentMetrics) RecordProcessingLatency(stage, processor, tenantID string, seconds float64) {
	m.ProcessingSeconds.WithLabelValues(stage, processor, tenantID).Observe(seconds)
}

// RecordClassification records a content classification.
func (m *EnrichmentMetrics) RecordClassification(contentType, subtype, profile, tenantID string) {
	m.ClassificationsTotal.WithLabelValues(contentType, subtype, profile, tenantID).Inc()
}

// RecordEntityResolution records an entity resolution.
func (m *EnrichmentMetrics) RecordEntityResolution(entityType, action, tenantID string) {
	m.EntityResolutionsTotal.WithLabelValues(entityType, action, tenantID).Inc()
}

// RecordEntityConfidence records an entity confidence score.
func (m *EnrichmentMetrics) RecordEntityConfidence(entityType, tenantID string, confidence float64) {
	m.EntityConfidence.WithLabelValues(entityType, tenantID).Observe(confidence)
}

// SetEntitiesPendingReview sets the count of entities pending review.
func (m *EnrichmentMetrics) SetEntitiesPendingReview(entityType, tenantID string, count float64) {
	m.EntitiesPendingReview.WithLabelValues(entityType, tenantID).Set(count)
}

// RecordAIOperation records an AI operation.
func (m *EnrichmentMetrics) RecordAIOperation(operation, model, status, tenantID string) {
	m.AIOperationsTotal.WithLabelValues(operation, model, status, tenantID).Inc()
}

// RecordAILatency records AI operation latency.
func (m *EnrichmentMetrics) RecordAILatency(operation, model, tenantID string, seconds float64) {
	m.AILatencySeconds.WithLabelValues(operation, model, tenantID).Observe(seconds)
}

// RecordAITokens records token usage.
func (m *EnrichmentMetrics) RecordAITokens(direction, model, tenantID string, count float64) {
	m.AITokensTotal.WithLabelValues(direction, model, tenantID).Add(count)
}

// SetExtractionParseSuccessRate sets the parse success rate for a template.
func (m *EnrichmentMetrics) SetExtractionParseSuccessRate(templateID, tenantID string, rate float64) {
	m.ExtractionParseSuccess.WithLabelValues(templateID, tenantID).Set(rate)
}

// MetricsRecorder provides a convenient interface for recording metrics during processing.
type MetricsRecorder struct {
	metrics  *EnrichmentMetrics
	tenantID string
}

// NewMetricsRecorder creates a new metrics recorder for a tenant.
func NewMetricsRecorder(metrics *EnrichmentMetrics, tenantID string) *MetricsRecorder {
	return &MetricsRecorder{
		metrics:  metrics,
		tenantID: tenantID,
	}
}

// RecordStageCompletion records stage completion metrics.
func (r *MetricsRecorder) RecordStageCompletion(stage, processor, status string, durationSeconds float64) {
	r.metrics.RecordProcessed(stage, processor, status, r.tenantID)
	r.metrics.RecordProcessingLatency(stage, processor, r.tenantID, durationSeconds)
}

// RecordClassification records a classification.
func (r *MetricsRecorder) RecordClassification(contentType, subtype, profile string) {
	r.metrics.RecordClassification(contentType, subtype, profile, r.tenantID)
}

// RecordEntityResolution records an entity resolution.
func (r *MetricsRecorder) RecordEntityResolution(entityType, action string, confidence float64) {
	r.metrics.RecordEntityResolution(entityType, action, r.tenantID)
	r.metrics.RecordEntityConfidence(entityType, r.tenantID, confidence)
}

// RecordAICompletion records AI operation completion.
func (r *MetricsRecorder) RecordAICompletion(operation, model, status string, latencySeconds float64, inputTokens, outputTokens int) {
	r.metrics.RecordAIOperation(operation, model, status, r.tenantID)
	r.metrics.RecordAILatency(operation, model, r.tenantID, latencySeconds)
	r.metrics.RecordAITokens("input", model, r.tenantID, float64(inputTokens))
	r.metrics.RecordAITokens("output", model, r.tenantID, float64(outputTokens))
}
