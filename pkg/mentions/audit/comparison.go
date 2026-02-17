// Package audit provides resolution trace recording, auditing, and model comparison capabilities.
package audit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/mentions/resolver"
)

// Comparison represents a model comparison run.
type Comparison struct {
	ID             string    `json:"id"` // comp_xyz789 format
	TenantID       string    `json:"tenant_id"`
	ContentID      int64     `json:"content_id"`
	ContentType    string    `json:"content_type,omitempty"`
	ContentSummary string    `json:"content_summary,omitempty"`
	Models         []string  `json:"models"`
	TraceIDs       []string  `json:"trace_ids"`
	InitiatedBy    string    `json:"initiated_by,omitempty"`
	Purpose        string    `json:"purpose,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`

	// Summary stats
	TotalDecisions     int `json:"total_decisions,omitempty"`
	UnanimousDecisions int `json:"unanimous_decisions,omitempty"`
	DivergentDecisions int `json:"divergent_decisions,omitempty"`

	// Analysis
	DivergenceSummary map[string]interface{} `json:"divergence_summary,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// ComparisonDecision represents a per-mention decision comparison across models.
type ComparisonDecision struct {
	ID           int64  `json:"id"`
	ComparisonID string `json:"comparison_id"`

	// What mention
	MentionedText string `json:"mentioned_text"`
	MentionIndex  *int   `json:"mention_index,omitempty"`

	// Decisions by each model
	ModelDecisions []ModelDecision `json:"model_decisions"`

	// Analysis
	IsUnanimous      bool    `json:"is_unanimous"`
	DivergenceType   string  `json:"divergence_type,omitempty"`
	ConfidenceSpread float32 `json:"confidence_spread,omitempty"`

	// Ground truth
	GroundTruthEntityID *int64   `json:"ground_truth_entity_id,omitempty"`
	ModelsCorrect       []string `json:"models_correct,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// ModelDecision represents a single model's decision for a mention.
type ModelDecision struct {
	Model       string  `json:"model"`
	EntityID    *int64  `json:"entity_id,omitempty"`
	EntityName  string  `json:"entity_name,omitempty"`
	Confidence  float32 `json:"confidence"`
	Reasoning   string  `json:"reasoning,omitempty"`
	SuggestNew  bool    `json:"suggest_new,omitempty"`
	NewEntityType string `json:"new_entity_type,omitempty"`
}

// ComparisonFilter specifies criteria for listing comparisons.
type ComparisonFilter struct {
	TenantID    string     `json:"tenant_id"`
	ContentID   *int64     `json:"content_id,omitempty"`
	ContentType string     `json:"content_type,omitempty"`
	Purpose     string     `json:"purpose,omitempty"`
	Since       *time.Time `json:"since,omitempty"`
	Limit       int        `json:"limit,omitempty"`
	Offset      int        `json:"offset,omitempty"`
}

// ComparisonSummary provides a summary view of a comparison.
type ComparisonSummary struct {
	ID                 string    `json:"id"`
	ContentID          int64     `json:"content_id"`
	ContentType        string    `json:"content_type,omitempty"`
	ContentSummary     string    `json:"content_summary,omitempty"`
	Models             []string  `json:"models"`
	TotalDecisions     int       `json:"total_decisions"`
	UnanimousDecisions int       `json:"unanimous_decisions"`
	DivergentDecisions int       `json:"divergent_decisions"`
	StartedAt          time.Time `json:"started_at"`
}

// ComparisonDetail provides full detail view of a comparison.
type ComparisonDetail struct {
	Comparison Comparison           `json:"comparison"`
	Decisions  []ComparisonDecision `json:"decisions"`
	Traces     []TraceSummary       `json:"traces,omitempty"` // Optional linked traces
}

// ModelStats aggregates statistics for a specific model.
type ModelStats struct {
	Model              string  `json:"model"`
	TotalComparisons   int     `json:"total_comparisons"`
	TotalDecisions     int     `json:"total_decisions"`
	CorrectDecisions   int     `json:"correct_decisions"`
	Accuracy           float32 `json:"accuracy,omitempty"`
	AverageConfidence  float32 `json:"average_confidence"`
	AverageLatencyMs   int     `json:"average_latency_ms"`
	UnanimousAgreement int     `json:"unanimous_agreement"` // Times it agreed with all others
}

// ComparisonRunner orchestrates model comparisons.
type ComparisonRunner struct {
	repo      ComparisonRepository
	resolvers map[string]*resolver.Resolver // model -> resolver
	tracer    *Tracer
}

// ComparisonRepository defines the data access interface for comparisons.
type ComparisonRepository interface {
	// Comparison operations
	CreateComparison(ctx context.Context, comp *Comparison) error
	GetComparison(ctx context.Context, id string) (*Comparison, error)
	UpdateComparison(ctx context.Context, comp *Comparison) error
	ListComparisons(ctx context.Context, filter ComparisonFilter) ([]ComparisonSummary, error)
	DeleteComparison(ctx context.Context, id string) error

	// Decision operations
	CreateComparisonDecision(ctx context.Context, decision *ComparisonDecision) error
	GetComparisonDecisions(ctx context.Context, comparisonID string) ([]ComparisonDecision, error)
	GetDivergentDecisions(ctx context.Context, comparisonID string) ([]ComparisonDecision, error)

	// Aggregate operations
	GetComparisonDetail(ctx context.Context, comparisonID string) (*ComparisonDetail, error)
	GetModelStats(ctx context.Context, tenantID string, daysSince int) ([]ModelStats, error)

	// Cleanup
	DeleteOldComparisons(ctx context.Context, olderThanDays int) (int, error)
}

// NewComparisonRunner creates a new comparison runner.
func NewComparisonRunner(repo ComparisonRepository, tracer *Tracer) *ComparisonRunner {
	return &ComparisonRunner{
		repo:      repo,
		resolvers: make(map[string]*resolver.Resolver),
		tracer:    tracer,
	}
}

// RegisterResolver registers a resolver for a specific model.
func (r *ComparisonRunner) RegisterResolver(model string, res *resolver.Resolver) {
	r.resolvers[model] = res
}

// modelResult holds the result from a single model's resolution.
type modelResult struct {
	model       string
	traceID     string
	resolutions []resolver.Resolution
	err         error
}

// ErrProviderUnavailable indicates a model provider is not registered.
var ErrProviderUnavailable = fmt.Errorf("provider unavailable")

// RunComparison runs the same content through multiple models and compares results.
func (r *ComparisonRunner) RunComparison(
	ctx context.Context,
	tenantID string,
	contentID int64,
	contentType, contentSummary string,
	content string,
	models []string,
	initiatedBy, purpose string,
) (*ComparisonDetail, error) {
	compID := generateComparisonID()

	comp := &Comparison{
		ID:             compID,
		TenantID:       tenantID,
		ContentID:      contentID,
		ContentType:    contentType,
		ContentSummary: contentSummary,
		Models:         models,
		TraceIDs:       make([]string, 0, len(models)),
		InitiatedBy:    initiatedBy,
		Purpose:        purpose,
		StartedAt:      time.Now(),
		CreatedAt:      time.Now(),
	}

	if err := r.repo.CreateComparison(ctx, comp); err != nil {
		return nil, err
	}

	results := make([]modelResult, len(models))
	for i, model := range models {
		res, ok := r.resolvers[model]
		if !ok {
			results[i] = modelResult{model: model, err: ErrProviderUnavailable}
			continue
		}

		// Run resolution with this model
		batchResult, err := res.ProcessContent(ctx, tenantID, contentID, contentType, content, nil, nil)
		if err != nil {
			results[i] = modelResult{model: model, err: err}
			continue
		}

		results[i] = modelResult{
			model:       model,
			traceID:     batchResult.TraceID,
			resolutions: batchResult.Resolutions,
			err:         nil,
		}

		if batchResult.TraceID != "" {
			comp.TraceIDs = append(comp.TraceIDs, batchResult.TraceID)
		}
	}

	// Compare results and create decision records
	decisions, unanimous, divergent := r.compareResults(models, results)
	comp.TotalDecisions = len(decisions)
	comp.UnanimousDecisions = unanimous
	comp.DivergentDecisions = divergent

	// Create divergence summary
	comp.DivergenceSummary = r.createDivergenceSummary(decisions)

	// Save decisions
	for i := range decisions {
		decisions[i].ComparisonID = compID
		if err := r.repo.CreateComparisonDecision(ctx, &decisions[i]); err != nil {
			// Log but continue
			continue
		}
	}

	// Mark comparison complete
	now := time.Now()
	comp.CompletedAt = &now
	if err := r.repo.UpdateComparison(ctx, comp); err != nil {
		return nil, err
	}

	return &ComparisonDetail{
		Comparison: *comp,
		Decisions:  decisions,
	}, nil
}

// compareResults compares resolution results across models.
func (r *ComparisonRunner) compareResults(models []string, results []modelResult) ([]ComparisonDecision, int, int) {
	// Collect all unique mentions across all models
	mentionMap := make(map[string][]ModelDecision) // mentionText -> decisions

	for i, res := range results {
		if res.err != nil {
			continue
		}
		for _, resolution := range res.resolutions {
			md := ModelDecision{
				Model:      models[i],
				Confidence: resolution.Confidence,
				Reasoning:  resolution.Reasoning,
				SuggestNew: resolution.Decision == resolver.DecisionTypeSuggestNewEntity,
			}
			if resolution.ResolvedTo != nil {
				eid := resolution.ResolvedTo.EntityID.Int64()
				md.EntityID = &eid
				md.EntityName = resolution.ResolvedTo.EntityName
			}
			mentionMap[resolution.MentionText] = append(mentionMap[resolution.MentionText], md)
		}
	}

	var decisions []ComparisonDecision
	unanimous, divergent := 0, 0

	for mentionText, modelDecisions := range mentionMap {
		decision := ComparisonDecision{
			MentionedText:  mentionText,
			ModelDecisions: modelDecisions,
			CreatedAt:      time.Now(),
		}

		// Analyze unanimity
		decision.IsUnanimous = r.isUnanimous(modelDecisions)
		if decision.IsUnanimous {
			unanimous++
		} else {
			divergent++
			decision.DivergenceType = r.classifyDivergence(modelDecisions)
		}

		// Calculate confidence spread
		decision.ConfidenceSpread = r.calculateConfidenceSpread(modelDecisions)

		decisions = append(decisions, decision)
	}

	return decisions, unanimous, divergent
}

// isUnanimous checks if all models made the same decision.
func (r *ComparisonRunner) isUnanimous(decisions []ModelDecision) bool {
	if len(decisions) < 2 {
		return true
	}

	first := decisions[0]
	for _, d := range decisions[1:] {
		// Both suggest new entity of same type
		if first.SuggestNew && d.SuggestNew {
			if first.NewEntityType != d.NewEntityType {
				return false
			}
			continue
		}
		// Both resolved to same entity
		if first.EntityID != nil && d.EntityID != nil {
			if *first.EntityID != *d.EntityID {
				return false
			}
			continue
		}
		// One suggests new, other resolved - not unanimous
		if first.SuggestNew != d.SuggestNew {
			return false
		}
	}

	return true
}

// classifyDivergence determines the type of divergence.
func (r *ComparisonRunner) classifyDivergence(decisions []ModelDecision) string {
	hasNew := false
	hasExisting := false
	entityIDs := make(map[int64]bool)

	for _, d := range decisions {
		if d.SuggestNew {
			hasNew = true
		} else if d.EntityID != nil {
			hasExisting = true
			entityIDs[*d.EntityID] = true
		}
	}

	if hasNew && hasExisting {
		return "new_vs_existing"
	}
	if len(entityIDs) > 1 {
		return "different_entity"
	}
	// Check if it's just a confidence gap
	spread := r.calculateConfidenceSpread(decisions)
	if spread > 0.3 {
		return "confidence_gap"
	}
	return "unknown"
}

// calculateConfidenceSpread calculates the spread between max and min confidence.
func (r *ComparisonRunner) calculateConfidenceSpread(decisions []ModelDecision) float32 {
	if len(decisions) < 2 {
		return 0
	}

	minConf := decisions[0].Confidence
	maxConf := decisions[0].Confidence

	for _, d := range decisions[1:] {
		if d.Confidence < minConf {
			minConf = d.Confidence
		}
		if d.Confidence > maxConf {
			maxConf = d.Confidence
		}
	}

	return maxConf - minConf
}

// createDivergenceSummary creates a summary of divergences.
func (r *ComparisonRunner) createDivergenceSummary(decisions []ComparisonDecision) map[string]interface{} {
	summary := make(map[string]interface{})

	divergenceTypes := make(map[string]int)
	var divergentMentions []string

	for _, d := range decisions {
		if !d.IsUnanimous {
			divergenceTypes[d.DivergenceType]++
			if len(divergentMentions) < 5 {
				divergentMentions = append(divergentMentions, d.MentionedText)
			}
		}
	}

	summary["types"] = divergenceTypes
	summary["sample_mentions"] = divergentMentions

	return summary
}

// generateComparisonID generates a unique comparison ID.
func generateComparisonID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "comp_" + hex.EncodeToString(b)
}
