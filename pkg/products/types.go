// Package products provides types and repository for product management.
// Products represent business products with hierarchy, team associations, and timeline events.
package products

import (
	"time"

	"github.com/google/uuid"
)

// ProductType represents the type/level of a product in the hierarchy.
type ProductType string

const (
	ProductTypeProduct    ProductType = "product"
	ProductTypeSubProduct ProductType = "sub_product"
	ProductTypeFeature    ProductType = "feature"
)

// ProductStatus represents the lifecycle status of a product.
type ProductStatus string

const (
	ProductStatusActive     ProductStatus = "active"
	ProductStatusBeta       ProductStatus = "beta"
	ProductStatusSunset     ProductStatus = "sunset"
	ProductStatusDeprecated ProductStatus = "deprecated"
)

// Product represents a business product with hierarchy support.
type Product struct {
	ID          int64         `json:"id,omitempty"`
	TenantID    string        `json:"tenant_id"`
	Name        string        `json:"name"`
	Description *string       `json:"description,omitempty"`
	ParentID    *int64        `json:"parent_id,omitempty"`
	ProductType ProductType   `json:"product_type"`
	Status      ProductStatus `json:"status"`
	Keywords    []string      `json:"keywords,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`

	// Loaded relationships
	Parent   *Product        `json:"parent,omitempty"`
	Children []*Product      `json:"children,omitempty"`
	Aliases  []*ProductAlias `json:"aliases,omitempty"`
}

// ProductAlias represents an alternative name for a product.
type ProductAlias struct {
	ID        int64     `json:"id,omitempty"`
	ProductID int64     `json:"product_id"`
	Alias     string    `json:"alias"`
	CreatedAt time.Time `json:"created_at"`
}

// ProductWithHierarchy represents a product with its full hierarchy path.
type ProductWithHierarchy struct {
	*Product
	Depth int    `json:"depth"`
	Path  string `json:"path"` // e.g., "LKE > Managed Databases > PostgreSQL"
}

// ProductFilter contains filtering options for product queries.
type ProductFilter struct {
	TenantID    string
	ParentID    *int64
	ProductType *ProductType
	Status      *ProductStatus
	NameSearch  string
	Limit       int
	Offset      int
}

// ==================== Product Team Types ====================

// ProductTeam represents an association between a product and a team.
type ProductTeam struct {
	ID        int64     `json:"id,omitempty"`
	TenantID  string    `json:"tenant_id"`
	ProductID int64     `json:"product_id"`
	TeamID    int64     `json:"team_id"`
	Context   string    `json:"context,omitempty"` // e.g., "Core Team", "DRI Team"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Loaded relationships - populated by joins
	ProductName string `json:"product_name,omitempty"`
	TeamName    string `json:"team_name,omitempty"`
}

// ProductTeamRole represents a scoped role assignment.
// A person has a role in the context of a product through a team.
type ProductTeamRole struct {
	ID            int64      `json:"id,omitempty"`
	TenantID      string     `json:"tenant_id"`
	ProductTeamID int64      `json:"product_team_id"`
	PersonID      int64      `json:"person_id"`
	Role          string     `json:"role"`            // e.g., "DRI", "Manager", "Lead"
	Scope         string     `json:"scope,omitempty"` // e.g., "Networking", "Database"
	IsActive      bool       `json:"is_active"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	// Loaded relationships - populated by joins
	ProductName string `json:"product_name,omitempty"`
	TeamName    string `json:"team_name,omitempty"`
	TeamContext string `json:"team_context,omitempty"`
	PersonName  string `json:"person_name,omitempty"`
	PersonEmail string `json:"person_email,omitempty"`
}

// RoleQuery represents a query for finding roles.
type RoleQuery struct {
	TenantID    string
	ProductName string
	TeamName    string
	Role        string
	Scope       string
	PersonName  string
	ActiveOnly  bool
}

// ==================== Product Event Types ====================

// EventType represents the type of timeline event.
type EventType string

const (
	EventTypeDecision   EventType = "decision"
	EventTypeMilestone  EventType = "milestone"
	EventTypeRisk       EventType = "risk"
	EventTypeRelease    EventType = "release"
	EventTypeCompetitor EventType = "competitor"
	EventTypeOrgChange  EventType = "org_change"
	EventTypeMarket     EventType = "market"
	EventTypeNote       EventType = "note"
)

// EventVisibility indicates whether an event is internal or external.
type EventVisibility string

const (
	EventVisibilityInternal EventVisibility = "internal"
	EventVisibilityExternal EventVisibility = "external"
)

// EventSourceType indicates how an event was created.
type EventSourceType string

const (
	EventSourceManual  EventSourceType = "manual"
	EventSourceDerived EventSourceType = "derived"
)

// ProductEvent represents a timeline entry for a product.
type ProductEvent struct {
	ID          int64           `json:"id,omitempty"`
	EventUUID   uuid.UUID       `json:"event_uuid"`
	TenantID    string          `json:"tenant_id"`
	ProductID   int64           `json:"product_id"`
	EventType   EventType       `json:"event_type"`
	Visibility  EventVisibility `json:"visibility"`
	SourceType  EventSourceType `json:"source_type"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	OccurredAt  time.Time       `json:"occurred_at"`
	RecordedBy  string          `json:"recorded_by,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`

	// Loaded relationships
	ProductName string              `json:"product_name,omitempty"`
	Links       []*ProductEventLink `json:"links,omitempty"`
}

// ProductEventLink links an event to other entities (meetings, emails, etc.).
type ProductEventLink struct {
	ID               int64     `json:"id,omitempty"`
	EventID          int64     `json:"event_id"`
	LinkedEntityType string    `json:"linked_entity_type"` // meeting, email, document, source
	LinkedEntityID   int64     `json:"linked_entity_id"`
	LinkType         string    `json:"link_type"` // source, reference, follow_up
	CreatedAt        time.Time `json:"created_at"`
}

// EventFilter contains filtering options for event queries.
type EventFilter struct {
	TenantID   string
	ProductID  *int64
	EventTypes []EventType
	Visibility *EventVisibility
	Since      *time.Time
	Until      *time.Time
	Limit      int
	Offset     int
}

// ContextWindow represents events around a specific point in time.
type ContextWindow struct {
	CenterEvent  *ProductEvent   `json:"center_event,omitempty"`
	CenterTime   time.Time       `json:"center_time"`
	EventsBefore []*ProductEvent `json:"events_before"`
	EventsAfter  []*ProductEvent `json:"events_after"`
	WindowStart  time.Time       `json:"window_start"`
	WindowEnd    time.Time       `json:"window_end"`
}
