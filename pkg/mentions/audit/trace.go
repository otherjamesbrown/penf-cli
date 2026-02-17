package audit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/mentions/resolver"
)

// StartTrace implements resolver.Tracer.
func (t *Tracer) StartTrace(
	tenantID string,
	contentID int64,
	contentType, contentSummary string,
	model string,
	level resolver.TraceLevel,
	config map[string]interface{},
) (string, error) {
	traceID := generateTraceID()

	trace := &Trace{
		ID:             traceID,
		TenantID:       tenantID,
		ContentID:      contentID,
		ContentType:    contentType,
		ContentSummary: contentSummary,
		StartedAt:      time.Now(),
		Status:         resolver.TraceStatusInProgress,
		ModelUsed:      model,
		TraceLevel:     level,
		ConfigSnapshot: config,
		CreatedAt:      time.Now(),
	}

	if err := t.repo.CreateTrace(context.Background(), trace); err != nil {
		return "", err
	}

	return traceID, nil
}

// RecordStageStart implements resolver.Tracer.
func (t *Tracer) RecordStageStart(
	traceID string,
	stageNum int,
	stageName resolver.StageName,
	inputSummary string,
	inputData interface{},
) (int64, error) {
	stage := &Stage{
		TraceID:      traceID,
		StageNumber:  stageNum,
		StageName:    stageName,
		StartedAt:    time.Now(),
		InputSummary: inputSummary,
		Status:       resolver.StageStatusInProgress,
		CreatedAt:    time.Now(),
	}

	// Only store input data at full/debug levels
	if t.traceLevel == resolver.TraceLevelFull || t.traceLevel == resolver.TraceLevelDebug {
		stage.InputData = inputData
	}

	if err := t.repo.CreateStage(context.Background(), stage); err != nil {
		return 0, err
	}

	return stage.ID, nil
}

// RecordStageComplete implements resolver.Tracer.
func (t *Tracer) RecordStageComplete(stageID int64, outputSummary string, outputData interface{}) error {
	stage, err := t.repo.GetStage(context.Background(), stageID)
	if err != nil {
		return err
	}

	now := time.Now()
	stage.CompletedAt = &now
	stage.DurationMs = int(now.Sub(stage.StartedAt).Milliseconds())
	stage.OutputSummary = outputSummary
	stage.Status = resolver.StageStatusCompleted

	// Only store output data at full/debug levels
	if t.traceLevel == resolver.TraceLevelFull || t.traceLevel == resolver.TraceLevelDebug {
		stage.OutputData = outputData
	}

	return t.repo.UpdateStage(context.Background(), stage)
}

// RecordStageFailed implements resolver.Tracer.
func (t *Tracer) RecordStageFailed(stageID int64, errorMsg string) error {
	stage, err := t.repo.GetStage(context.Background(), stageID)
	if err != nil {
		return err
	}

	now := time.Now()
	stage.CompletedAt = &now
	stage.DurationMs = int(now.Sub(stage.StartedAt).Milliseconds())
	stage.Status = resolver.StageStatusFailed
	stage.ErrorMessage = errorMsg

	return t.repo.UpdateStage(context.Background(), stage)
}

// RecordStageSkipped implements resolver.Tracer.
func (t *Tracer) RecordStageSkipped(stageID int64, reason string) error {
	stage, err := t.repo.GetStage(context.Background(), stageID)
	if err != nil {
		return err
	}

	now := time.Now()
	stage.CompletedAt = &now
	stage.Status = resolver.StageStatusSkipped
	stage.Skipped = true
	stage.SkipReason = reason

	return t.repo.UpdateStage(context.Background(), stage)
}

// RecordLLMCall implements resolver.Tracer.
func (t *Tracer) RecordLLMCall(
	traceID string,
	stageID int64,
	model, promptTemplate, promptText string,
	promptTokens int,
	responseText string,
	responseTokens int,
	parsedOutput interface{},
	parseErrors []string,
	latencyMs, attemptNumber int,
	isFallback bool,
	fallbackReason string,
) error {
	// Only record LLM calls at full/debug levels
	if t.traceLevel != resolver.TraceLevelFull && t.traceLevel != resolver.TraceLevelDebug {
		return nil
	}

	var stageIDPtr *int64
	if stageID > 0 {
		stageIDPtr = &stageID
	}

	call := &LLMCall{
		TraceID:        traceID,
		StageID:        stageIDPtr,
		Model:          model,
		PromptTemplate: promptTemplate,
		PromptText:     promptText,
		PromptTokens:   promptTokens,
		ResponseText:   responseText,
		ResponseTokens: responseTokens,
		ParsedOutput:   parsedOutput,
		ParseErrors:    parseErrors,
		LatencyMs:      latencyMs,
		AttemptNumber:  attemptNumber,
		IsFallback:     isFallback,
		FallbackReason: fallbackReason,
		CreatedAt:      time.Now(),
	}

	return t.repo.CreateLLMCall(context.Background(), call)
}

// RecordDecision implements resolver.Tracer.
func (t *Tracer) RecordDecision(
	traceID string,
	stageID int64,
	decisionType resolver.DecisionType,
	mentionID *int64,
	mentionText, chosenOption string,
	alternatives interface{},
	confidence float32,
	reasoning string,
	factors interface{},
) error {
	// Record decisions at all levels except minimal
	if t.traceLevel == resolver.TraceLevelMinimal {
		return nil
	}

	var stageIDPtr *int64
	if stageID > 0 {
		stageIDPtr = &stageID
	}

	// Convert factors to map
	var factorsMap map[string]interface{}
	if factors != nil {
		if fm, ok := factors.(map[string]interface{}); ok {
			factorsMap = fm
		} else {
			// Try JSON round-trip
			data, _ := json.Marshal(factors)
			json.Unmarshal(data, &factorsMap)
		}
	}

	decision := &Decision{
		TraceID:       traceID,
		StageID:       stageIDPtr,
		DecisionType:  decisionType,
		MentionID:     mentionID,
		MentionedText: mentionText,
		ChosenOption:  chosenOption,
		Alternatives:  alternatives,
		Confidence:    confidence,
		Reasoning:     reasoning,
		Factors:       factorsMap,
		CreatedAt:     time.Now(),
	}

	return t.repo.CreateDecision(context.Background(), decision)
}

// CompleteTrace implements resolver.Tracer.
func (t *Tracer) CompleteTrace(traceID string, mentionsFound, autoResolved, queuedForReview, newEntitiesSuggested int) error {
	trace, err := t.repo.GetTrace(context.Background(), traceID)
	if err != nil {
		return err
	}

	now := time.Now()
	trace.CompletedAt = &now
	trace.DurationMs = int(now.Sub(trace.StartedAt).Milliseconds())
	trace.MentionsFound = mentionsFound
	trace.AutoResolved = autoResolved
	trace.QueuedForReview = queuedForReview
	trace.NewEntitiesSuggested = newEntitiesSuggested
	trace.Status = resolver.TraceStatusCompleted

	return t.repo.UpdateTrace(context.Background(), trace)
}

// FailTrace implements resolver.Tracer.
func (t *Tracer) FailTrace(traceID string, errorMsg string) error {
	trace, err := t.repo.GetTrace(context.Background(), traceID)
	if err != nil {
		return err
	}

	now := time.Now()
	trace.CompletedAt = &now
	trace.DurationMs = int(now.Sub(trace.StartedAt).Milliseconds())
	trace.Status = resolver.TraceStatusFailed
	trace.ErrorMessage = errorMsg

	return t.repo.UpdateTrace(context.Background(), trace)
}

// generateTraceID generates a unique trace ID.
func generateTraceID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "trace_" + hex.EncodeToString(b)
}

// GetTraceLevel returns the current trace level.
func (t *Tracer) GetTraceLevel() resolver.TraceLevel {
	return t.traceLevel
}

// SetTraceLevel updates the trace level.
func (t *Tracer) SetTraceLevel(level resolver.TraceLevel) {
	t.traceLevel = level
}
