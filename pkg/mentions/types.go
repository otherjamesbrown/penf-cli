// Package mentions provides unified mention resolution for all entity types.
// It handles extracting and resolving mentions of persons, terms, products,
// companies, and projects from content with context-aware ranking.
package mentions

import (
	"time"
)

// EntityType represents the type of entity being mentioned.
type EntityType string

const (
	EntityTypePerson  EntityType = "person"
	EntityTypeTerm    EntityType = "term"
	EntityTypeProduct EntityType = "product"
	EntityTypeCompany EntityType = "company"
	EntityTypeProject EntityType = "project"
)

// MentionStatus represents the resolution status of a mention.
type MentionStatus string

const (
	MentionStatusPending      MentionStatus = "pending"
	MentionStatusAutoResolved MentionStatus = "auto_resolved"
	MentionStatusUserResolved MentionStatus = "user_resolved"
	MentionStatusDismissed    MentionStatus = "dismissed"
)

// ResolutionSource indicates how a resolution was determined.
type ResolutionSource string

const (
	ResolutionSourceExactMatch      ResolutionSource = "exact_match"
	ResolutionSourceAlias           ResolutionSource = "alias"
	ResolutionSourceFuzzy           ResolutionSource = "fuzzy"
	ResolutionSourceProjectContext  ResolutionSource = "project_context"
	ResolutionSourcePriorLink       ResolutionSource = "prior_link"
	ResolutionSourceUserConfirmed   ResolutionSource = "user_confirmed"
)

// ContentMention represents an entity mention extracted from content.
type ContentMention struct {
	ID        int64  `json:"id"`
	TenantID  string `json:"tenant_id"`
	ContentID int64  `json:"content_id"`

	// What was mentioned
	EntityType     EntityType `json:"entity_type"`
	MentionedText  string     `json:"mentioned_text"`
	Position       *int       `json:"position,omitempty"`
	ContextSnippet string     `json:"context_snippet,omitempty"`

	// Resolution
	ResolvedEntityID     *int64           `json:"resolved_entity_id,omitempty"`
	ResolutionConfidence *float32         `json:"resolution_confidence,omitempty"`
	ResolutionSource     ResolutionSource `json:"resolution_source,omitempty"`

	// For terms: expansion used
	ResolvedExpansion string `json:"resolved_expansion,omitempty"`

	// Candidates at extraction time
	Candidates []Candidate `json:"candidates,omitempty"`

	// Review status
	Status     MentionStatus `json:"status"`
	ResolvedAt *time.Time    `json:"resolved_at,omitempty"`
	ResolvedBy string        `json:"resolved_by,omitempty"`

	// Project context
	ProjectContextID *int64 `json:"project_context_id,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
}

// Candidate represents a potential resolution for a mention.
type Candidate struct {
	EntityID   int64    `json:"entity_id"`
	EntityName string   `json:"entity_name"`
	Confidence float32  `json:"confidence"`
	Reasons    []string `json:"reasons,omitempty"`

	// Prior link information
	PriorLinks int `json:"prior_links,omitempty"`

	// Project context
	ProjectRole string `json:"project_role,omitempty"`

	// For terms: linked entity info
	LinkedEntity *LinkedEntityRef `json:"linked_entity,omitempty"`

	// For fuzzy matches: similarity info
	TranscriptionLikelihood float32 `json:"transcription_likelihood,omitempty"`
}

// LinkedEntityRef represents a reference to a linked canonical entity.
type LinkedEntityRef struct {
	Type string `json:"type"` // product, project, company
	ID   int64  `json:"id"`
	Name string `json:"name,omitempty"`
}

// MentionPattern represents a resolution pattern for suggestions.
type MentionPattern struct {
	ID       int64  `json:"id"`
	TenantID string `json:"tenant_id"`

	// Pattern definition
	EntityType  EntityType `json:"entity_type"`
	PatternText string     `json:"pattern_text"`

	// Resolution target
	ResolvedEntityID  *int64 `json:"resolved_entity_id,omitempty"`
	ResolvedExpansion string `json:"resolved_expansion,omitempty"`

	// Scope
	ProjectID   *int64 `json:"project_id,omitempty"`
	IsPermanent bool   `json:"is_permanent"`

	// Usage tracking
	TimesSeen    int        `json:"times_seen"`
	TimesLinked  int        `json:"times_linked"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	LastLinkedAt *time.Time `json:"last_linked_at,omitempty"`

	// Source tracking
	FirstContentID *int64 `json:"first_content_id,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
}

// EntityProjectAffinity tracks entity relevance within a project context.
type EntityProjectAffinity struct {
	ID       int64  `json:"id"`
	TenantID string `json:"tenant_id"`

	// Entity reference
	EntityType EntityType `json:"entity_type"`
	EntityID   int64      `json:"entity_id"`
	ProjectID  int64      `json:"project_id"`

	// Affinity metrics
	MentionCount    int        `json:"mention_count"`
	LastMentionedAt *time.Time `json:"last_mentioned_at,omitempty"`
	IsMember        bool       `json:"is_member"`
	Role            string     `json:"role,omitempty"`

	// Computed score
	AffinityScore float32 `json:"affinity_score"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ResolutionResult contains the result of resolving a mention.
type ResolutionResult struct {
	MentionedText string     `json:"mentioned_text"`
	EntityType    EntityType `json:"entity_type"`
	ContentID     int64      `json:"content_id"`
	ProjectID     *int64     `json:"project_id,omitempty"`

	// Resolution outcome
	Candidates       []Candidate `json:"candidates"`
	AutoResolved     bool        `json:"auto_resolved"`
	ResolvedEntityID *int64      `json:"resolved_entity_id,omitempty"`

	// For terms
	ResolvedExpansion string           `json:"resolved_expansion,omitempty"`
	LinkedEntity      *LinkedEntityRef `json:"linked_entity,omitempty"`
}

// MentionInput is used for creating a new mention.
type MentionInput struct {
	ContentID        int64      `json:"content_id"`
	EntityType       EntityType `json:"entity_type"`
	MentionedText    string     `json:"mentioned_text"`
	Position         *int       `json:"position,omitempty"`
	ContextSnippet   string     `json:"context_snippet,omitempty"`
	ProjectContextID *int64     `json:"project_context_id,omitempty"`
}

// ResolutionInput is used when resolving a mention.
type ResolutionInput struct {
	MentionID        int64            `json:"mention_id"`
	EntityID         int64            `json:"entity_id"`
	Source           ResolutionSource `json:"source"`
	MakePermanent    bool             `json:"make_permanent"`
	TranscriptError  bool             `json:"transcription_error"`
	ResolvedBy       string           `json:"resolved_by,omitempty"`
}

// DismissalInput is used when dismissing a mention.
type DismissalInput struct {
	MentionID  int64  `json:"mention_id"`
	Reason     string `json:"reason"`
	DismissedBy string `json:"dismissed_by,omitempty"`
}

// BatchResolutionRequest is used for batch resolution via CLI.
type BatchResolutionRequest struct {
	Resolutions []ResolutionInput `json:"resolutions"`
	Dismissals  []DismissalInput  `json:"dismissals"`
}

// BatchResolutionResult contains the result of batch operations.
type BatchResolutionResult struct {
	Resolved  int      `json:"resolved"`
	Dismissed int      `json:"dismissed"`
	Errors    []string `json:"errors,omitempty"`
}

// MentionFilter specifies criteria for listing mentions.
type MentionFilter struct {
	TenantID   string        `json:"tenant_id"`
	ContentID  *int64        `json:"content_id,omitempty"`
	EntityType *EntityType   `json:"entity_type,omitempty"`
	Status     *MentionStatus `json:"status,omitempty"`
	ProjectID  *int64        `json:"project_id,omitempty"`
	Limit      int           `json:"limit,omitempty"`
	Offset     int           `json:"offset,omitempty"`
}

// MentionContext provides full context for Claude-native processing.
type MentionContext struct {
	Mentions []ContentMention `json:"mentions"`
	Stats    MentionStats     `json:"stats"`
	Workflow WorkflowConfig   `json:"workflow"`
}

// MentionStats provides statistics about pending mentions.
type MentionStats struct {
	TotalPending    int            `json:"total_pending"`
	ByType          map[string]int `json:"by_type"`
	AutoResolvable  int            `json:"auto_resolvable"`
	ResolvedToday   int            `json:"resolved_today"`
}

// WorkflowConfig provides thresholds for resolution workflow.
type WorkflowConfig struct {
	AutoResolveThreshold    float32 `json:"auto_resolve_threshold"`
	SuggestThreshold        float32 `json:"suggest_threshold"`
	PriorLinkBoostThreshold int     `json:"prior_link_boost_threshold"`
}

// DefaultWorkflowConfig returns default workflow configuration.
func DefaultWorkflowConfig() WorkflowConfig {
	return WorkflowConfig{
		AutoResolveThreshold:    0.9,
		SuggestThreshold:        0.7,
		PriorLinkBoostThreshold: 5,
	}
}

// ConfidenceFactors defines the scoring factors for resolution.
type ConfidenceFactors struct {
	ExactCanonicalMatch float32 // 1.0
	ExactAliasMatch     float32 // 0.9
	FuzzyMatchMultiplier float32 // similarity * 0.8
	ProjectMemberBoost  float32 // +0.2
	HighAffinityBoost   float32 // +0.15 (for 10+ mentions)
	PriorLinkBoost      float32 // +0.05 per link (max +0.2)
	SameProjectBoost    float32 // +0.1
	TranscriptionPenalty float32 // -0.1 per edit distance
}

// DefaultConfidenceFactors returns default confidence scoring factors.
func DefaultConfidenceFactors() ConfidenceFactors {
	return ConfidenceFactors{
		ExactCanonicalMatch:  1.0,
		ExactAliasMatch:      0.9,
		FuzzyMatchMultiplier: 0.8,
		ProjectMemberBoost:   0.2,
		HighAffinityBoost:    0.15,
		PriorLinkBoost:       0.05,
		SameProjectBoost:     0.1,
		TranscriptionPenalty: 0.1,
	}
}
