// Package audit provides resolution trace recording and auditing capabilities.
package audit

import (
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/mentions/resolver"
)

// Trace represents a top-level resolution trace.
type Trace struct {
	ID            string                  `json:"id"` // trace_abc123
	TenantID      string                  `json:"tenant_id"`
	ContentID     int64                   `json:"content_id"`
	ContentType   string                  `json:"content_type,omitempty"`
	ContentSummary string                 `json:"content_summary,omitempty"`
	StartedAt     time.Time               `json:"started_at"`
	CompletedAt   *time.Time              `json:"completed_at,omitempty"`
	DurationMs    int                     `json:"duration_ms,omitempty"`
	MentionsFound int                     `json:"mentions_found,omitempty"`
	AutoResolved  int                     `json:"auto_resolved,omitempty"`
	QueuedForReview int                   `json:"queued_for_review,omitempty"`
	NewEntitiesSuggested int              `json:"new_entities_suggested,omitempty"`
	Status        resolver.TraceStatus    `json:"status"`
	ErrorMessage  string                  `json:"error_message,omitempty"`
	ModelUsed     string                  `json:"model_used,omitempty"`
	TraceLevel    resolver.TraceLevel     `json:"trace_level"`
	ConfigSnapshot map[string]interface{} `json:"config_snapshot,omitempty"`
	CreatedAt     time.Time               `json:"created_at"`
}

// Stage represents an individual stage within a trace.
type Stage struct {
	ID            int64                `json:"id"`
	TraceID       string               `json:"trace_id"`
	StageNumber   int                  `json:"stage_number"` // 1-4
	StageName     resolver.StageName   `json:"stage_name"`
	StartedAt     time.Time            `json:"started_at"`
	CompletedAt   *time.Time           `json:"completed_at,omitempty"`
	DurationMs    int                  `json:"duration_ms,omitempty"`
	InputSummary  string               `json:"input_summary,omitempty"`
	InputData     interface{}          `json:"input_data,omitempty"`
	OutputSummary string               `json:"output_summary,omitempty"`
	OutputData    interface{}          `json:"output_data,omitempty"`
	Status        resolver.StageStatus `json:"status"`
	Skipped       bool                 `json:"skipped"`
	SkipReason    string               `json:"skip_reason,omitempty"`
	ErrorMessage  string               `json:"error_message,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
}

// LLMCall represents an LLM request/response log entry.
type LLMCall struct {
	ID              int64       `json:"id"`
	TraceID         string      `json:"trace_id"`
	StageID         *int64      `json:"stage_id,omitempty"`
	Model           string      `json:"model"`
	PromptTemplate  string      `json:"prompt_template,omitempty"`
	PromptText      string      `json:"prompt_text,omitempty"`
	PromptTokens    int         `json:"prompt_tokens,omitempty"`
	ResponseText    string      `json:"response_text,omitempty"`
	ResponseTokens  int         `json:"response_tokens,omitempty"`
	ParsedOutput    interface{} `json:"parsed_output,omitempty"`
	ParseErrors     []string    `json:"parse_errors,omitempty"`
	LatencyMs       int         `json:"latency_ms,omitempty"`
	AttemptNumber   int         `json:"attempt_number"`
	IsFallback      bool        `json:"is_fallback"`
	FallbackReason  string      `json:"fallback_reason,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
}

// Decision represents an individual resolution decision.
type Decision struct {
	ID              int64                  `json:"id"`
	TraceID         string                 `json:"trace_id"`
	StageID         *int64                 `json:"stage_id,omitempty"`
	DecisionType    resolver.DecisionType  `json:"decision_type"`
	MentionID       *int64                 `json:"mention_id,omitempty"`
	MentionedText   string                 `json:"mentioned_text,omitempty"`
	ChosenOption    string                 `json:"chosen_option,omitempty"`
	Alternatives    interface{}            `json:"alternatives,omitempty"`
	Confidence      float32                `json:"confidence,omitempty"`
	Reasoning       string                 `json:"reasoning,omitempty"`
	Factors         map[string]interface{} `json:"factors,omitempty"`
	WasCorrect      *bool                  `json:"was_correct,omitempty"`
	CorrectionNotes string                 `json:"correction_notes,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
}

// TraceFilter specifies criteria for listing traces.
type TraceFilter struct {
	TenantID       string               `json:"tenant_id"`
	ContentID      *int64               `json:"content_id,omitempty"`
	ContentType    string               `json:"content_type,omitempty"`
	Status         *resolver.TraceStatus `json:"status,omitempty"`
	ModelUsed      string               `json:"model_used,omitempty"`
	HadCorrections bool                 `json:"had_corrections,omitempty"`
	Since          *time.Time           `json:"since,omitempty"`
	Until          *time.Time           `json:"until,omitempty"`
	Limit          int                  `json:"limit,omitempty"`
	Offset         int                  `json:"offset,omitempty"`
}

// TraceSummary provides a summary view of a trace.
type TraceSummary struct {
	ID              string    `json:"id"`
	ContentID       int64     `json:"content_id"`
	ContentType     string    `json:"content_type,omitempty"`
	ContentSummary  string    `json:"content_summary,omitempty"`
	Status          string    `json:"status"`
	MentionsFound   int       `json:"mentions_found"`
	AutoResolved    int       `json:"auto_resolved"`
	QueuedForReview int       `json:"queued_for_review"`
	ModelUsed       string    `json:"model_used,omitempty"`
	DurationMs      int       `json:"duration_ms,omitempty"`
	StartedAt       time.Time `json:"started_at"`
}

// TraceDetail provides full detail view of a trace.
type TraceDetail struct {
	Trace     Trace      `json:"trace"`
	Stages    []Stage    `json:"stages"`
	Decisions []Decision `json:"decisions"`
	LLMCalls  []LLMCall  `json:"llm_calls,omitempty"` // Only for full/debug level
}

// CorrectionStats provides statistics about corrections.
type CorrectionStats struct {
	TotalCorrections      int            `json:"total_corrections"`
	ByPattern             map[string]int `json:"by_pattern"` // e.g., "phonetic_mismatch": 5
	MostCorrectedType     string         `json:"most_corrected_type"`
	MostCorrectedTypeCount int           `json:"most_corrected_type_count"`
}
