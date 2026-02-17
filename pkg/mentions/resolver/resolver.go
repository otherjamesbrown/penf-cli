// Package resolver provides LLM-driven mention resolution using a multi-stage pipeline.
//
// The resolution process follows 4 stages:
//   1. Extraction + Understanding - Extract mentions and understand context
//   2. Cross-Mention Reasoning - Identify relationships between mentions
//   3. Entity Matching - Match to database candidates with confidence
//   4. Verification - Challenge uncertain resolutions
//
// Resolution decisions are made by an LLM (local MLX or Claude API).
// All stages are traced for audit and debugging.
package resolver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/mentions"
)

// Resolver orchestrates the multi-stage LLM resolution pipeline.
type Resolver struct {
	config    Config
	provider  LLMProvider
	stages    *StageExecutor
	gatherer  *CandidateGatherer
	mentRepo  mentions.Repository
	tracer    Tracer
}

// Tracer records traces for audit.
type Tracer interface {
	StartTrace(tenantID string, contentID int64, contentType, contentSummary string, model string, level TraceLevel, config map[string]interface{}) (string, error)
	RecordStageStart(traceID string, stageNum int, stageName StageName, inputSummary string, inputData interface{}) (int64, error)
	RecordStageComplete(stageID int64, outputSummary string, outputData interface{}) error
	RecordStageFailed(stageID int64, errorMsg string) error
	RecordStageSkipped(stageID int64, reason string) error
	RecordLLMCall(traceID string, stageID int64, model, promptTemplate, promptText string, promptTokens int, responseText string, responseTokens int, parsedOutput interface{}, parseErrors []string, latencyMs, attemptNumber int, isFallback bool, fallbackReason string) error
	RecordDecision(traceID string, stageID int64, decisionType DecisionType, mentionID *int64, mentionText, chosenOption string, alternatives interface{}, confidence float32, reasoning string, factors interface{}) error
	CompleteTrace(traceID string, mentionsFound, autoResolved, queuedForReview, newEntitiesSuggested int) error
	FailTrace(traceID string, errorMsg string) error
}

// NewResolver creates a new resolver with the given dependencies.
func NewResolver(
	config Config,
	provider LLMProvider,
	lookup mentions.EntityLookup,
	mentRepo mentions.Repository,
	tracer Tracer,
) *Resolver {
	if err := config.Validate(); err != nil {
		// Use defaults
		config = DefaultConfig()
	}

	return &Resolver{
		config:   config,
		provider: provider,
		stages:   NewStageExecutor(provider, config),
		gatherer: NewCandidateGatherer(lookup, mentRepo, config),
		mentRepo: mentRepo,
		tracer:   tracer,
	}
}

// ProcessBatch processes a batch of mentions from the same content.
func (r *Resolver) ProcessBatch(ctx context.Context, tenantID string, batch ResolutionBatch) (*BatchResult, error) {
	start := time.Now()

	// Generate trace ID
	traceID, err := r.startTrace(tenantID, batch)
	if err != nil {
		return nil, fmt.Errorf("start trace: %w", err)
	}

	result := &BatchResult{
		ContentID: batch.ContentID,
		TraceID:   traceID,
	}

	// Stage 1: Extraction + Understanding
	r.heartbeat("stage 1: extraction + understanding")
	understanding, err := r.executeStage1(ctx, batch, traceID)
	if err != nil {
		r.failTrace(traceID, err.Error())
		result.Error = err.Error()
		return result, err
	}

	if len(understanding.Mentions) == 0 {
		// No mentions found, complete trace
		r.completeTrace(traceID, 0, 0, 0, 0)
		result.ProcessingTimeMs = int(time.Since(start).Milliseconds())
		return result, nil
	}

	// Stage 2: Cross-Mention Reasoning
	r.heartbeat("stage 2: cross-mention reasoning")
	relationships, err := r.executeStage2(ctx, understanding, batch, traceID)
	if err != nil {
		r.failTrace(traceID, err.Error())
		result.Error = err.Error()
		return result, err
	}

	// Gather candidates (code-based)
	r.heartbeat("gathering candidates")
	candidates, err := r.gatherer.GatherCandidates(ctx, tenantID, understanding, relationships, batch.ProjectID)
	if err != nil {
		r.failTrace(traceID, err.Error())
		result.Error = err.Error()
		return result, err
	}

	// Stage 3: Entity Matching
	r.heartbeat("stage 3: entity matching")
	matching, err := r.executeStage3(ctx, understanding, relationships, candidates, traceID)
	if err != nil {
		r.failTrace(traceID, err.Error())
		result.Error = err.Error()
		return result, err
	}

	// Stage 4: Verification (for uncertain resolutions)
	r.heartbeat("stage 4: verification")
	verifiedResolutions := r.executeStage4(ctx, matching.Resolutions, batch, traceID)

	// Apply resolutions
	result.Resolutions = verifiedResolutions
	result.NewEntities = matching.NewEntitiesSuggested

	for _, res := range verifiedResolutions {
		if res.Decision == DecisionTypeResolve && res.Confidence >= r.config.Thresholds.AutoResolve {
			result.AutoResolved++
		} else if res.Decision == DecisionTypeQueueReview || (res.Decision == DecisionTypeResolve && res.Confidence < r.config.Thresholds.AutoResolve) {
			result.QueuedForReview++
		}
	}

	// Complete trace
	r.completeTrace(traceID, len(understanding.Mentions), result.AutoResolved, result.QueuedForReview, len(matching.NewEntitiesSuggested))

	result.ProcessingTimeMs = int(time.Since(start).Milliseconds())
	return result, nil
}

// SetHeartbeat sets a heartbeat callback for the next ProcessBatch call.
// This allows callers (e.g., Temporal activities) to inject liveness signals
// without the resolver depending on Temporal directly.
func (r *Resolver) SetHeartbeat(fn HeartbeatFunc) {
	r.config.Heartbeat = fn
}

// heartbeat signals liveness if a heartbeat function is configured.
func (r *Resolver) heartbeat(stage string) {
	if r.config.Heartbeat != nil {
		r.config.Heartbeat(stage)
	}
}

// startTrace initializes a new trace.
func (r *Resolver) startTrace(tenantID string, batch ResolutionBatch) (string, error) {
	if r.tracer == nil {
		return generateTraceID(), nil
	}

	summary := ""
	if batch.Metadata != nil && batch.Metadata.Subject != "" {
		summary = batch.Metadata.Subject
	}

	configSnapshot := map[string]interface{}{
		"auto_resolve_threshold":  r.config.Thresholds.AutoResolve,
		"verification_threshold":  r.config.Thresholds.Verification,
		"suggest_threshold":       r.config.Thresholds.Suggest,
		"max_mentions_per_batch":  r.config.MaxMentionsPerBatch,
	}

	return r.tracer.StartTrace(
		tenantID,
		batch.ContentID,
		batch.ContentType,
		summary,
		r.provider.Name(),
		r.config.TraceLevel,
		configSnapshot,
	)
}

// executeStage1 runs Stage 1: Extraction + Understanding.
func (r *Resolver) executeStage1(ctx context.Context, batch ResolutionBatch, traceID string) (*Stage1Understanding, error) {
	var stageID int64
	if r.tracer != nil {
		var err error
		stageID, err = r.tracer.RecordStageStart(traceID, 1, StageNameUnderstanding,
			fmt.Sprintf("Content: %d chars", len(batch.ContentText)), batch)
		if err != nil {
			return nil, err
		}
	}

	understanding, err := r.stages.ExecuteStage1(ctx, batch, traceID)
	if err != nil {
		if r.tracer != nil && stageID > 0 {
			r.tracer.RecordStageFailed(stageID, err.Error())
		}
		return nil, err
	}

	if r.tracer != nil && stageID > 0 {
		r.tracer.RecordStageComplete(stageID,
			fmt.Sprintf("Found %d mentions", len(understanding.Mentions)), understanding)
	}

	return understanding, nil
}

// executeStage2 runs Stage 2: Cross-Mention Reasoning.
func (r *Resolver) executeStage2(ctx context.Context, understanding *Stage1Understanding, batch ResolutionBatch, traceID string) (*Stage2CrossMention, error) {
	var stageID int64
	if r.tracer != nil {
		var err error
		stageID, err = r.tracer.RecordStageStart(traceID, 2, StageNameCrossMention,
			fmt.Sprintf("%d mentions", len(understanding.Mentions)), understanding)
		if err != nil {
			return nil, err
		}
	}

	relationships, err := r.stages.ExecuteStage2(ctx, understanding, batch, traceID)
	if err != nil {
		if r.tracer != nil && stageID > 0 {
			r.tracer.RecordStageFailed(stageID, err.Error())
		}
		return nil, err
	}

	if r.tracer != nil && stageID > 0 {
		r.tracer.RecordStageComplete(stageID,
			fmt.Sprintf("%d relationships found", len(relationships.MentionRelationships)), relationships)
	}

	return relationships, nil
}

// executeStage3 runs Stage 3: Entity Matching.
func (r *Resolver) executeStage3(
	ctx context.Context,
	understanding *Stage1Understanding,
	relationships *Stage2CrossMention,
	candidates map[string]*CandidateSet,
	traceID string,
) (*Stage3Matching, error) {
	var stageID int64
	if r.tracer != nil {
		totalCandidates := 0
		for _, cs := range candidates {
			totalCandidates += len(cs.Candidates)
		}
		var err error
		stageID, err = r.tracer.RecordStageStart(traceID, 3, StageNameMatching,
			fmt.Sprintf("%d mentions, %d candidates", len(understanding.Mentions), totalCandidates),
			map[string]interface{}{"understanding": understanding, "relationships": relationships, "candidates": candidates})
		if err != nil {
			return nil, err
		}
	}

	matching, err := r.stages.ExecuteStage3(ctx, understanding, relationships, candidates, traceID)
	if err != nil {
		if r.tracer != nil && stageID > 0 {
			r.tracer.RecordStageFailed(stageID, err.Error())
		}
		return nil, err
	}

	// Record decisions
	if r.tracer != nil && stageID > 0 {
		for _, res := range matching.Resolutions {
			r.tracer.RecordDecision(traceID, stageID, res.Decision, nil, res.MentionText,
				func() string {
					if res.ResolvedTo != nil {
						return res.ResolvedTo.EntityName
					}
					return ""
				}(),
				res.Alternatives, res.Confidence, res.Reasoning, res.Factors)
		}

		resolved := 0
		queued := 0
		for _, res := range matching.Resolutions {
			if res.Decision == DecisionTypeResolve {
				resolved++
			} else if res.Decision == DecisionTypeQueueReview {
				queued++
			}
		}
		r.tracer.RecordStageComplete(stageID,
			fmt.Sprintf("%d resolved, %d queued, %d new suggested", resolved, queued, len(matching.NewEntitiesSuggested)),
			matching)
	}

	return matching, nil
}

// executeStage4 runs Stage 4: Verification for uncertain resolutions.
func (r *Resolver) executeStage4(ctx context.Context, resolutions []Resolution, batch ResolutionBatch, traceID string) []Resolution {
	needsVerification := make([]int, 0)
	for i, res := range resolutions {
		if res.Decision == DecisionTypeResolve && res.Confidence < r.config.Thresholds.Verification && res.Confidence >= r.config.Thresholds.AutoResolve {
			needsVerification = append(needsVerification, i)
		}
	}

	if len(needsVerification) == 0 {
		// Skip stage 4
		if r.tracer != nil {
			stageID, err := r.tracer.RecordStageStart(traceID, 4, StageNameVerification,
				"No uncertain resolutions", nil)
			if err == nil && stageID > 0 {
				r.tracer.RecordStageSkipped(stageID, "No resolutions need verification")
			}
		}
		return resolutions
	}

	var stageID int64
	if r.tracer != nil {
		var err error
		stageID, err = r.tracer.RecordStageStart(traceID, 4, StageNameVerification,
			fmt.Sprintf("%d resolutions need verification", len(needsVerification)), needsVerification)
		if err != nil {
			return resolutions
		}
	}

	// Verify each uncertain resolution
	verified := 0
	for _, idx := range needsVerification {
		res := resolutions[idx]
		verification, err := r.stages.ExecuteStage4(ctx, res, batch, traceID)
		if err != nil {
			// On error, keep original confidence
			continue
		}

		// Apply verification result
		verified++
		switch verification.VerificationResult {
		case "confirmed":
			// Keep confidence
		case "adjusted":
			resolutions[idx].Confidence = verification.AdjustedConfidence
		case "rejected":
			resolutions[idx].Decision = DecisionTypeQueueReview
			resolutions[idx].Confidence = verification.AdjustedConfidence
		}
	}

	if r.tracer != nil && stageID > 0 {
		r.tracer.RecordStageComplete(stageID,
			fmt.Sprintf("%d/%d verified", verified, len(needsVerification)), nil)
	}

	return resolutions
}

// completeTrace marks the trace as completed.
func (r *Resolver) completeTrace(traceID string, mentionsFound, autoResolved, queuedForReview, newEntitiesSuggested int) {
	if r.tracer == nil {
		return
	}
	r.tracer.CompleteTrace(traceID, mentionsFound, autoResolved, queuedForReview, newEntitiesSuggested)
}

// failTrace marks the trace as failed.
func (r *Resolver) failTrace(traceID string, errorMsg string) {
	if r.tracer == nil {
		return
	}
	r.tracer.FailTrace(traceID, errorMsg)
}

// generateTraceID generates a unique trace ID.
func generateTraceID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "trace_" + hex.EncodeToString(b)
}

// ProcessContent processes all mentions from a content item.
func (r *Resolver) ProcessContent(ctx context.Context, tenantID string, contentID int64, contentType, contentText string, projectID *int64, metadata *ContentMetadata) (*BatchResult, error) {
	batch := ResolutionBatch{
		ContentID:   contentID,
		ContentType: contentType,
		ContentText: contentText,
		ProjectID:   projectID,
		Metadata:    metadata,
	}

	// Split into batches if content has too many mentions
	// For now, process as single batch (splitting can be added later)
	return r.ProcessBatch(ctx, tenantID, batch)
}

// ResolveAll implements the mentions.Resolver interface for compatibility.
func (r *Resolver) ResolveAll(ctx context.Context, tenantID string, contentID int64, contentText string, inputs []mentions.MentionInput) ([]Resolution, error) {
	batch := ResolutionBatch{
		ContentID:   contentID,
		ContentType: "content",
		ContentText: contentText,
	}

	result, err := r.ProcessBatch(ctx, tenantID, batch)
	if err != nil {
		return nil, err
	}

	return result.Resolutions, nil
}
