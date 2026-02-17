// Package resolver provides LLM-driven mention resolution using a multi-stage pipeline.
package resolver

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/mentions"
)

// FlexInt64 is an int64 that can unmarshal from both JSON numbers and strings.
// Small LLMs frequently return numeric IDs as quoted strings (e.g., "123" instead of 123).
type FlexInt64 int64

func (f *FlexInt64) UnmarshalJSON(data []byte) error {
	// Try number first
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexInt64(n)
		return nil
	}
	// Try string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("FlexInt64: cannot parse %q as int64: %w", s, err)
		}
		*f = FlexInt64(n)
		return nil
	}
	return fmt.Errorf("FlexInt64: cannot unmarshal %s", string(data))
}

func (f FlexInt64) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(f))
}

func (f FlexInt64) Int64() int64 {
	return int64(f)
}

// TraceLevel controls the amount of detail stored in traces.
type TraceLevel string

const (
	TraceLevelMinimal  TraceLevel = "minimal"  // Outcome only
	TraceLevelStandard TraceLevel = "standard" // Stages + decisions + reasoning
	TraceLevelFull     TraceLevel = "full"     // + Complete LLM prompts/responses
	TraceLevelDebug    TraceLevel = "debug"    // + Intermediate data structures
)

// TraceStatus represents the status of a resolution trace.
type TraceStatus string

const (
	TraceStatusInProgress TraceStatus = "in_progress"
	TraceStatusCompleted  TraceStatus = "completed"
	TraceStatusFailed     TraceStatus = "failed"
)

// StageStatus represents the status of a trace stage.
type StageStatus string

const (
	StageStatusInProgress StageStatus = "in_progress"
	StageStatusCompleted  StageStatus = "completed"
	StageStatusFailed     StageStatus = "failed"
	StageStatusSkipped    StageStatus = "skipped"
)

// DecisionType represents the type of resolution decision.
type DecisionType string

const (
	DecisionTypeResolve          DecisionType = "resolve"
	DecisionTypeQueueReview      DecisionType = "queue_review"
	DecisionTypeSuggestNewEntity DecisionType = "suggest_new_entity"
	DecisionTypeSkipVerification DecisionType = "skip_verification"
)

// StageName represents the name of a resolution stage.
type StageName string

const (
	StageNameUnderstanding StageName = "understanding"
	StageNameCrossMention  StageName = "cross_mention"
	StageNameMatching      StageName = "matching"
	StageNameVerification  StageName = "verification"
)

// ResolutionBatch represents a batch of mentions from the same content.
type ResolutionBatch struct {
	ContentID   int64            `json:"content_id"`
	ContentType string           `json:"content_type"` // email, meeting, document
	ContentText string           `json:"content_text"`
	Mentions    []MentionInput   `json:"mentions"`
	ProjectID   *int64           `json:"project_id,omitempty"`
	Metadata    *ContentMetadata `json:"metadata,omitempty"`
}

// MentionInput represents input for mention extraction/resolution.
type MentionInput struct {
	Text           string `json:"text"`
	Position       int    `json:"position,omitempty"`
	ContextSnippet string `json:"context_snippet,omitempty"`
}

// ContentMetadata provides additional context about the content.
type ContentMetadata struct {
	Subject      string    `json:"subject,omitempty"`
	Date         time.Time `json:"date,omitempty"`
	Participants []string  `json:"participants,omitempty"`
}

// ProjectContext provides project-specific context.
type ProjectContext struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members,omitempty"`
}

// Stage1Understanding is the output of Stage 1: Extraction + Understanding.
type Stage1Understanding struct {
	Mentions []MentionUnderstanding `json:"mentions"`
}

// MentionUnderstanding represents the LLM's understanding of a mention.
type MentionUnderstanding struct {
	Text               string              `json:"text"`
	EntityType         mentions.EntityType `json:"entity_type"`
	Position           int                 `json:"position"`
	ContextSnippet     string              `json:"context_snippet"`
	Understanding      string              `json:"understanding"`
	TranscriptionFlags *TranscriptionFlags `json:"transcription_flags,omitempty"`
}

// TranscriptionFlags indicates potential transcription issues.
type TranscriptionFlags struct {
	LikelyError        bool     `json:"likely_error"`
	PhoneticVariants   []string `json:"phonetic_variants,omitempty"`
	ProbableCorrection string   `json:"probable_correction,omitempty"`
	Confidence         float32  `json:"confidence,omitempty"`
}

// Stage2CrossMention is the output of Stage 2: Cross-Mention Reasoning.
type Stage2CrossMention struct {
	ContentID            int64                 `json:"content_id"`
	UnifiedUnderstanding string                `json:"unified_understanding"`
	MentionRelationships []MentionRelationship `json:"mention_relationships,omitempty"`
	ResolutionHints      []string              `json:"resolution_hints,omitempty"`
}

// MentionRelationship describes a relationship between two mentions.
type MentionRelationship struct {
	FromMention  string `json:"from_mention"`
	ToMention    string `json:"to_mention"`
	Relationship string `json:"relationship"`
	Inference    string `json:"inference,omitempty"`
}

// Stage3Matching is the output of Stage 3: Entity Matching.
type Stage3Matching struct {
	Resolutions          []Resolution          `json:"resolutions"`
	NewEntitiesSuggested []NewEntitySuggestion `json:"new_entities_suggested,omitempty"`
}

// Resolution represents a resolution decision from the LLM.
type Resolution struct {
	MentionText       string                 `json:"mention_text"`
	MentionPosition   int                    `json:"mention_position"`
	Decision          DecisionType           `json:"decision"`
	ResolvedTo        *ResolvedEntity        `json:"resolved_to,omitempty"`
	Confidence        float32                `json:"confidence"`
	Reasoning         string                 `json:"reasoning"`
	Factors           map[string]interface{} `json:"factors,omitempty"`
	Alternatives      []AlternativeEntity    `json:"alternatives_considered,omitempty"`
	IsTranscription   bool                   `json:"is_transcription_error,omitempty"`
	LinkedEntity      *mentions.LinkedEntityRef `json:"linked_entity,omitempty"`
}

// ResolvedEntity represents the entity a mention was resolved to.
type ResolvedEntity struct {
	EntityType mentions.EntityType `json:"entity_type"`
	EntityID   FlexInt64           `json:"entity_id"`
	EntityName string              `json:"entity_name"`
	Term       string              `json:"term,omitempty"`       // For term type
	Expansion  string              `json:"expansion,omitempty"`  // For term type
}

// AlternativeEntity represents an alternative that was considered.
type AlternativeEntity struct {
	EntityID        FlexInt64 `json:"entity_id"`
	EntityName      string    `json:"entity_name"`
	Confidence      float32   `json:"confidence"`
	RejectionReason string    `json:"rejection_reason"`
}

// NewEntitySuggestion suggests creating a new entity.
type NewEntitySuggestion struct {
	MentionText   string              `json:"mention_text"`
	SuggestedType mentions.EntityType `json:"suggested_type"`
	SuggestedName string              `json:"suggested_name"`
	Reasoning     string              `json:"reasoning"`
	Confidence    float32             `json:"confidence"`
}

// Stage4Verification is the output of Stage 4: Verification.
type Stage4Verification struct {
	MentionText        string `json:"mention_text"`
	OriginalConfidence float32 `json:"original_confidence"`
	VerificationResult string  `json:"verification_result"` // confirmed, adjusted, rejected
	AdjustedConfidence float32 `json:"adjusted_confidence"`
	VerificationNotes  string  `json:"verification_notes"`
}

// CandidateSet represents candidates gathered for matching.
type CandidateSet struct {
	MentionText string               `json:"mention_text"`
	Candidates  []CandidateWithHints `json:"candidates"`
}

// CandidateWithHints extends a candidate with matching hints.
type CandidateWithHints struct {
	EntityID       int64                  `json:"entity_id"`
	EntityType     mentions.EntityType    `json:"entity_type"`
	EntityName     string                 `json:"entity_name"`
	ConfidenceHints map[string]interface{} `json:"confidence_hints"`
}

// BatchResult contains the results of processing a batch.
type BatchResult struct {
	ContentID         int64          `json:"content_id"`
	TraceID           string         `json:"trace_id"`
	Resolutions       []Resolution   `json:"resolutions"`
	NewEntities       []NewEntitySuggestion `json:"new_entities,omitempty"`
	AutoResolved      int            `json:"auto_resolved"`
	QueuedForReview   int            `json:"queued_for_review"`
	ProcessingTimeMs  int            `json:"processing_time_ms"`
	Error             string         `json:"error,omitempty"`
}

// LLMError represents an error from the LLM provider.
type LLMError struct {
	Code    LLMErrorCode `json:"code"`
	Message string       `json:"message"`
	Details interface{}  `json:"details,omitempty"`
}

func (e *LLMError) Error() string {
	return e.Message
}

// LLMErrorCode identifies the type of LLM error.
type LLMErrorCode string

const (
	ErrTimeout        LLMErrorCode = "timeout"
	ErrUnavailable    LLMErrorCode = "unavailable"
	ErrRateLimit      LLMErrorCode = "rate_limit"
	ErrParseFailure   LLMErrorCode = "parse_failure"
	ErrInvalidSchema  LLMErrorCode = "invalid_schema"
	ErrContentTooLong LLMErrorCode = "content_too_long"
	ErrTokenLimit     LLMErrorCode = "token_limit"
)
