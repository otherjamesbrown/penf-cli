// Package mentions provides unified mention resolution for all entity types.
package mentions

import (
	"context"
)

// Repository defines the data access interface for mentions.
type Repository interface {
	// Mentions CRUD
	CreateMention(ctx context.Context, input MentionInput) (*ContentMention, error)
	GetMention(ctx context.Context, id int64) (*ContentMention, error)
	ListMentions(ctx context.Context, filter MentionFilter) ([]ContentMention, error)
	UpdateMentionResolution(ctx context.Context, id int64, resolution ResolutionInput) error
	DismissMention(ctx context.Context, id int64, dismissal DismissalInput) error

	// Pattern operations
	GetPattern(ctx context.Context, tenantID string, entityType EntityType, text string, projectID *int64) (*MentionPattern, error)
	GetPatternsByText(ctx context.Context, tenantID string, entityType EntityType, text string) ([]MentionPattern, error)
	CreateOrUpdatePattern(ctx context.Context, pattern *MentionPattern) error
	IncrementPatternSeen(ctx context.Context, id int64) error
	IncrementPatternLinked(ctx context.Context, id int64, entityID int64) error

	// Affinity operations
	GetAffinity(ctx context.Context, tenantID string, entityType EntityType, entityID, projectID int64) (*EntityProjectAffinity, error)
	GetAffinitiesForProject(ctx context.Context, tenantID string, projectID int64, entityType EntityType) ([]EntityProjectAffinity, error)
	GetAffinitiesForEntity(ctx context.Context, tenantID string, entityType EntityType, entityID int64) ([]EntityProjectAffinity, error)
	UpsertAffinity(ctx context.Context, affinity *EntityProjectAffinity) error
	IncrementAffinityMentionCount(ctx context.Context, tenantID string, entityType EntityType, entityID, projectID int64) error

	// Statistics
	GetMentionStats(ctx context.Context, tenantID string) (*MentionStats, error)
	GetPendingCount(ctx context.Context, tenantID string, entityType *EntityType) (int, error)

	// Batch operations
	BatchCreateMentions(ctx context.Context, inputs []MentionInput) ([]ContentMention, error)
	BatchResolveMentions(ctx context.Context, resolutions []ResolutionInput) (*BatchResolutionResult, error)
}

// Resolver defines the interface for mention resolution.
type Resolver interface {
	// Resolve attempts to resolve a mention to an entity.
	Resolve(ctx context.Context, input MentionInput) (*ResolutionResult, error)

	// ResolveAll resolves multiple mentions, typically from the same content.
	ResolveAll(ctx context.Context, inputs []MentionInput) ([]ResolutionResult, error)
}

// EntityLookup defines the interface for looking up entities by type.
type EntityLookup interface {
	// LookupPerson finds person candidates by name.
	LookupPerson(ctx context.Context, tenantID, name string) ([]Candidate, error)

	// LookupTerm finds term candidates by text.
	LookupTerm(ctx context.Context, tenantID, text string) ([]Candidate, error)

	// LookupProduct finds product candidates by text.
	LookupProduct(ctx context.Context, tenantID, text string) ([]Candidate, error)

	// LookupCompany finds company candidates by text.
	LookupCompany(ctx context.Context, tenantID, text string) ([]Candidate, error)

	// LookupProject finds project candidates by text.
	LookupProject(ctx context.Context, tenantID, text string) ([]Candidate, error)

	// GetEntityName returns the display name for an entity.
	GetEntityName(ctx context.Context, entityType EntityType, entityID int64) (string, error)
}
