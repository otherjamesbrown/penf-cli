package observability

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewEnrichmentStageEvent(t *testing.T) {
	event := NewEnrichmentStageEvent("tenant-1", 123, StageClassification, StageStatusCompleted, "ContentClassifier", 45)

	if event.EventID == "" {
		t.Error("EventID should be generated")
	}
	if event.TenantID != "tenant-1" {
		t.Errorf("TenantID = %s, want tenant-1", event.TenantID)
	}
	if event.SourceID != 123 {
		t.Errorf("SourceID = %d, want 123", event.SourceID)
	}
	if event.Stage != StageClassification {
		t.Errorf("Stage = %s, want %s", event.Stage, StageClassification)
	}
	if event.Status != StageStatusCompleted {
		t.Errorf("Status = %s, want %s", event.Status, StageStatusCompleted)
	}
	if event.DurationMs != 45 {
		t.Errorf("DurationMs = %d, want 45", event.DurationMs)
	}
	if event.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestNewClassificationEvent(t *testing.T) {
	event := NewClassificationEvent("tenant-1", 123, "email", "thread", "full_ai", "header_match", 0.95)

	if event.EventID == "" {
		t.Error("EventID should be generated")
	}
	if event.ContentType != "email" {
		t.Errorf("ContentType = %s, want email", event.ContentType)
	}
	if event.ContentSubtype != "thread" {
		t.Errorf("ContentSubtype = %s, want thread", event.ContentSubtype)
	}
	if event.ProcessingProfile != "full_ai" {
		t.Errorf("ProcessingProfile = %s, want full_ai", event.ProcessingProfile)
	}
	if event.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", event.Confidence)
	}
}

func TestNewEntityResolutionEvent(t *testing.T) {
	event := NewEntityResolutionEvent("tenant-1", 123, EntityTypePerson, EntityActionResolved, 456, "john@example.com", 0.9, false)

	if event.EntityType != EntityTypePerson {
		t.Errorf("EntityType = %s, want %s", event.EntityType, EntityTypePerson)
	}
	if event.Action != EntityActionResolved {
		t.Errorf("Action = %s, want %s", event.Action, EntityActionResolved)
	}
	if event.EntityID != 456 {
		t.Errorf("EntityID = %d, want 456", event.EntityID)
	}
	if event.NeedsReview {
		t.Error("NeedsReview should be false")
	}
}

func TestNewAIProcessingEvent(t *testing.T) {
	event := NewAIProcessingEvent("tenant-1", 123, 789, AIOperationExtract, "gpt-4", 1000, 500, 1200, true)

	if event.ExtractionRunID != 789 {
		t.Errorf("ExtractionRunID = %d, want 789", event.ExtractionRunID)
	}
	if event.Operation != AIOperationExtract {
		t.Errorf("Operation = %s, want %s", event.Operation, AIOperationExtract)
	}
	if event.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", event.InputTokens)
	}
	if event.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", event.OutputTokens)
	}
	if !event.ParseSuccess {
		t.Error("ParseSuccess should be true")
	}
}

func TestNewEnrichmentErrorEvent(t *testing.T) {
	event := NewEnrichmentErrorEvent("tenant-1", 123, StageAIProcessing, "LLMExtractor", ErrorTypeTimeout, "request timed out", true, 2)

	if event.Stage != StageAIProcessing {
		t.Errorf("Stage = %s, want %s", event.Stage, StageAIProcessing)
	}
	if event.ErrorType != ErrorTypeTimeout {
		t.Errorf("ErrorType = %s, want %s", event.ErrorType, ErrorTypeTimeout)
	}
	if !event.Retryable {
		t.Error("Retryable should be true")
	}
	if event.RetryCount != 2 {
		t.Errorf("RetryCount = %d, want 2", event.RetryCount)
	}
}

func TestEventMarshalling(t *testing.T) {
	event := NewEnrichmentStageEvent("tenant-1", 123, StageClassification, StageStatusCompleted, "ContentClassifier", 45)
	event.BatchID = "batch-1"
	event.TraceID = "trace-abc"
	event.Outputs = []string{"classification", "profile"}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded EnrichmentStageEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.EventID != event.EventID {
		t.Errorf("Decoded EventID mismatch")
	}
	if decoded.BatchID != "batch-1" {
		t.Errorf("Decoded BatchID = %s, want batch-1", decoded.BatchID)
	}
	if len(decoded.Outputs) != 2 {
		t.Errorf("Decoded Outputs length = %d, want 2", len(decoded.Outputs))
	}
}

func TestNoOpEventPublisher(t *testing.T) {
	pub := &NoOpEventPublisher{}

	err := pub.Publish(context.Background(), ChannelStageCompleted, &EnrichmentStageEvent{})
	if err != nil {
		t.Errorf("NoOp Publish returned error: %v", err)
	}

	err = pub.Close()
	if err != nil {
		t.Errorf("NoOp Close returned error: %v", err)
	}
}

func TestEventEmitter(t *testing.T) {
	var published []struct {
		channel string
		event   interface{}
	}

	mockPublisher := NewRedisEventPublisher(func(ctx context.Context, channel string, message interface{}) error {
		published = append(published, struct {
			channel string
			event   interface{}
		}{channel, message})
		return nil
	})

	emitter := NewEventEmitter(mockPublisher)

	ctx := context.Background()

	// Test stage event
	stageEvent := NewEnrichmentStageEvent("tenant-1", 123, StageClassification, StageStatusCompleted, "ContentClassifier", 45)
	if err := emitter.EmitStageCompleted(ctx, stageEvent); err != nil {
		t.Errorf("EmitStageCompleted error: %v", err)
	}

	// Test classification event
	classEvent := NewClassificationEvent("tenant-1", 123, "email", "thread", "full_ai", "rule", 0.9)
	if err := emitter.EmitClassified(ctx, classEvent); err != nil {
		t.Errorf("EmitClassified error: %v", err)
	}

	// Test entity event
	entityEvent := NewEntityResolutionEvent("tenant-1", 123, EntityTypePerson, EntityActionResolved, 456, "test@example.com", 0.9, false)
	if err := emitter.EmitEntityResolved(ctx, entityEvent); err != nil {
		t.Errorf("EmitEntityResolved error: %v", err)
	}

	// Test AI event
	aiEvent := NewAIProcessingEvent("tenant-1", 123, 789, AIOperationExtract, "gpt-4", 100, 50, 500, true)
	if err := emitter.EmitAIProcessed(ctx, aiEvent); err != nil {
		t.Errorf("EmitAIProcessed error: %v", err)
	}

	// Test error event
	errorEvent := NewEnrichmentErrorEvent("tenant-1", 123, StageAIProcessing, "LLMExtractor", ErrorTypeTimeout, "timeout", true, 1)
	if err := emitter.EmitError(ctx, errorEvent); err != nil {
		t.Errorf("EmitError error: %v", err)
	}

	if len(published) != 5 {
		t.Errorf("Expected 5 published events, got %d", len(published))
	}

	// Verify channels
	expectedChannels := []string{
		ChannelStageCompleted,
		ChannelClassified,
		ChannelEntityResolved,
		ChannelAIProcessed,
		ChannelError,
	}
	for i, expected := range expectedChannels {
		if published[i].channel != expected {
			t.Errorf("Event %d channel = %s, want %s", i, published[i].channel, expected)
		}
	}
}

func TestEnrichmentMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewEnrichmentMetrics(reg)

	// Test queue metrics
	metrics.RecordQueueEnqueue("ingest", "tenant-1", "normal")
	metrics.RecordQueueDepth("ingest", "tenant-1", "normal", 10)
	metrics.RecordQueueWait("ingest", "tenant-1", "normal", 0.5)
	metrics.RecordDLQItem("ingest", "tenant-1", ErrorTypeTimeout)

	// Test processing metrics
	metrics.RecordProcessed(StageClassification, "ContentClassifier", "success", "tenant-1")
	metrics.RecordProcessingLatency(StageClassification, "ContentClassifier", "tenant-1", 0.045)
	metrics.RecordClassification("email", "thread", "full_ai", "tenant-1")

	// Test entity metrics
	metrics.RecordEntityResolution(EntityTypePerson, EntityActionResolved, "tenant-1")
	metrics.RecordEntityConfidence(EntityTypePerson, "tenant-1", 0.85)
	metrics.SetEntitiesPendingReview(EntityTypePerson, "tenant-1", 5)

	// Test AI metrics
	metrics.RecordAIOperation(AIOperationExtract, "gpt-4", "success", "tenant-1")
	metrics.RecordAILatency(AIOperationExtract, "gpt-4", "tenant-1", 1.2)
	metrics.RecordAITokens("input", "gpt-4", "tenant-1", 1000)
	metrics.RecordAITokens("output", "gpt-4", "tenant-1", 500)
	metrics.SetExtractionParseSuccessRate("template-1", "tenant-1", 0.95)

	// Verify metrics were registered
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	expectedMetrics := map[string]bool{
		"enrichment_queue_items_total":             false,
		"enrichment_queue_depth":                   false,
		"enrichment_queue_wait_seconds":            false,
		"enrichment_dlq_items_total":               false,
		"enrichment_items_processed_total":         false,
		"enrichment_processing_seconds":            false,
		"enrichment_classifications_total":         false,
		"enrichment_entity_resolutions_total":      false,
		"enrichment_entity_confidence":             false,
		"enrichment_entities_pending_review":       false,
		"enrichment_ai_operations_total":           false,
		"enrichment_ai_latency_seconds":            false,
		"enrichment_ai_tokens_total":               false,
		"enrichment_extraction_parse_success_rate": false,
	}

	for _, fam := range families {
		if _, ok := expectedMetrics[fam.GetName()]; ok {
			expectedMetrics[fam.GetName()] = true
		}
	}

	for name, found := range expectedMetrics {
		if !found {
			t.Errorf("Metric %s not found in registry", name)
		}
	}
}

func TestMetricsRecorder(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewEnrichmentMetrics(reg)
	recorder := NewMetricsRecorder(metrics, "tenant-1")

	recorder.RecordStageCompletion(StageClassification, "ContentClassifier", "success", 0.05)
	recorder.RecordClassification("email", "thread", "full_ai")
	recorder.RecordEntityResolution(EntityTypePerson, EntityActionResolved, 0.9)
	recorder.RecordAICompletion(AIOperationExtract, "gpt-4", "success", 1.5, 1000, 500)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(families) == 0 {
		t.Error("No metrics were recorded")
	}
}

func TestTracer(t *testing.T) {
	tracer := NewTracer()

	ctx := context.Background()

	// Test source span
	ctx, sourceSpan := tracer.StartSourceSpan(ctx, "tenant-1", 123, "batch-1")
	if sourceSpan == nil {
		t.Error("Source span should not be nil")
	}
	sourceSpan.End()

	// Test stage span
	ctx, stageSpan := tracer.StartStageSpan(ctx, StageClassification)
	if stageSpan == nil {
		t.Error("Stage span should not be nil")
	}
	stageSpan.End()

	// Test processor span
	ctx, procSpan := tracer.StartProcessorSpan(ctx, "ContentClassifier")
	if procSpan == nil {
		t.Error("Processor span should not be nil")
	}
	procSpan.End()

	// Test entity span
	ctx, entitySpan := tracer.StartEntityResolutionSpan(ctx, EntityTypePerson)
	if entitySpan == nil {
		t.Error("Entity span should not be nil")
	}
	entitySpan.End()

	// Test LLM span
	_, llmSpan := tracer.StartLLMSpan(ctx, "gpt-4")
	if llmSpan == nil {
		t.Error("LLM span should not be nil")
	}
	llmSpan.End()
}

func TestSpanHelper(t *testing.T) {
	tracer := NewTracer()
	ctx, span := tracer.StartSourceSpan(context.Background(), "tenant-1", 123, "")
	defer span.End()

	helper := NewSpanHelper(span)

	// Test setting various attributes
	helper.SetSourceInfo("tenant-1", 123, "thread-1")
	helper.SetClassification("email", "thread", "full_ai")
	helper.SetDuration(1500)
	helper.SetProcessor("ContentClassifier")
	helper.SetEntityResolution(EntityTypePerson, 456)
	helper.SetLLMResult(1000, 500, 1200)
	helper.SetTemplate(789, "1.0.0")
	helper.SetSuccess()

	// Test trace ID extraction
	traceID := GetTraceID(ctx)
	if traceID == "" {
		t.Log("TraceID is empty (expected with NoOp provider)")
	}

	// Test context injection
	headers := InjectTraceContext(ctx)
	if headers == nil {
		t.Error("InjectTraceContext returned nil")
	}
}
