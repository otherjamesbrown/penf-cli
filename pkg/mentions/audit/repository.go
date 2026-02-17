package audit

import (
	"context"

	"github.com/otherjamesbrown/penf-cli/pkg/mentions/resolver"
)

// Repository defines the data access interface for audit traces.
type Repository interface {
	// Trace operations
	CreateTrace(ctx context.Context, trace *Trace) error
	GetTrace(ctx context.Context, id string) (*Trace, error)
	UpdateTrace(ctx context.Context, trace *Trace) error
	ListTraces(ctx context.Context, filter TraceFilter) ([]TraceSummary, error)
	DeleteTrace(ctx context.Context, id string) error

	// Stage operations
	CreateStage(ctx context.Context, stage *Stage) error
	GetStage(ctx context.Context, id int64) (*Stage, error)
	UpdateStage(ctx context.Context, stage *Stage) error
	GetStagesForTrace(ctx context.Context, traceID string) ([]Stage, error)

	// LLM call operations
	CreateLLMCall(ctx context.Context, call *LLMCall) error
	GetLLMCallsForTrace(ctx context.Context, traceID string) ([]LLMCall, error)
	GetLLMCallsForStage(ctx context.Context, stageID int64) ([]LLMCall, error)

	// Decision operations
	CreateDecision(ctx context.Context, decision *Decision) error
	GetDecision(ctx context.Context, id int64) (*Decision, error)
	UpdateDecision(ctx context.Context, decision *Decision) error
	GetDecisionsForTrace(ctx context.Context, traceID string) ([]Decision, error)
	GetDecisionsForMention(ctx context.Context, mentionID int64) ([]Decision, error)
	GetCorrections(ctx context.Context, filter TraceFilter) ([]Decision, error)

	// Aggregate operations
	GetTraceDetail(ctx context.Context, traceID string, includeFullLLMCalls bool) (*TraceDetail, error)
	GetCorrectionStats(ctx context.Context, tenantID string, daysSince int) (*CorrectionStats, error)

	// Cleanup
	DeleteOldTraces(ctx context.Context, olderThanDays int) (int, error)
	DeleteOldDecisions(ctx context.Context, olderThanDays int) (int, error)
}

// Tracer implements the resolver.Tracer interface for recording traces.
type Tracer struct {
	repo       Repository
	traceLevel resolver.TraceLevel
}

// NewTracer creates a new tracer.
func NewTracer(repo Repository, level resolver.TraceLevel) *Tracer {
	return &Tracer{
		repo:       repo,
		traceLevel: level,
	}
}
