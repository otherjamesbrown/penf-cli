// Package entities provides entity resolution for people, teams, and projects.
package entities

import (
	"time"
)

// AccountType represents the type of account.
type AccountType string

const (
	AccountTypePerson          AccountType = "person"
	AccountTypeRole            AccountType = "role"
	AccountTypeDistribution    AccountType = "distribution"
	AccountTypeBot             AccountType = "bot"
	AccountTypeExternalService AccountType = "external_service"
	AccountTypeTeam            AccountType = "team"
	AccountTypeService         AccountType = "service"
)

// AliasType represents the type of alias.
type AliasType string

const (
	AliasTypeEmail       AliasType = "email"
	AliasTypeSlackID     AliasType = "slack_id"
	AliasTypeName        AliasType = "name"
	AliasTypeDisplayName AliasType = "display_name"
)

// Person represents a canonical person record.
type Person struct {
	ID            int64       `json:"id,omitempty"`
	TenantID      string      `json:"tenant_id"`
	CanonicalName string      `json:"canonical_name"`
	PrimaryEmail  string      `json:"primary_email"`
	Title         string      `json:"title,omitempty"`
	Department    string      `json:"department,omitempty"`
	Company       string      `json:"company,omitempty"`
	IsInternal    bool        `json:"is_internal"`
	AccountType   AccountType `json:"account_type"`

	// Review status
	Confidence  float32    `json:"confidence"`
	NeedsReview bool       `json:"needs_review"`
	AutoCreated bool       `json:"auto_created"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy  string     `json:"reviewed_by,omitempty"`

	// Rejection status (soft delete)
	RejectedAt     *time.Time `json:"rejected_at,omitempty"`
	RejectedReason string     `json:"rejected_reason,omitempty"`
	RejectedBy     string     `json:"rejected_by,omitempty"`

	// Duplicate tracking
	PotentialDuplicates []int64 `json:"potential_duplicates,omitempty"`

	// Loaded relationships
	Aliases []PersonAlias `json:"aliases,omitempty"`

	// Message counts
	SentCount     int `db:"sent_count" json:"sent_count"`
	ReceivedCount int `db:"received_count" json:"received_count"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PersonAlias represents an alias that resolves to a person.
type PersonAlias struct {
	ID           int64     `json:"id,omitempty"`
	PersonID     int64     `json:"person_id"`
	AliasType    AliasType `json:"alias_type"`
	AliasValue   string    `json:"alias_value"`
	Confidence   float32   `json:"confidence"`
	Source       string    `json:"source"`
	DiscoveredAt time.Time `json:"discovered_at"`
}

// Team represents a team of people.
type Team struct {
	ID          int64  `json:"id,omitempty"`
	TenantID    string `json:"tenant_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Loaded relationships
	Members []TeamMember `json:"members,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TeamMember represents membership in a team.
type TeamMember struct {
	ID       int64     `json:"id,omitempty"`
	TeamID   int64     `json:"team_id"`
	PersonID int64     `json:"person_id"`
	Role     string    `json:"role,omitempty"`
	JoinedAt time.Time `json:"joined_at"`

	// Loaded relationship
	Person *Person `json:"person,omitempty"`
}

// Project represents a project with associated people and Jira integration.
type Project struct {
	ID           int64    `json:"id,omitempty"`
	TenantID     string   `json:"tenant_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Keywords     []string `json:"keywords,omitempty"`
	JiraProjects []string `json:"jira_projects,omitempty"`

	// Loaded relationships
	Members []ProjectMember `json:"members,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProjectMember represents membership in a project.
type ProjectMember struct {
	ID        int64     `json:"id,omitempty"`
	ProjectID int64     `json:"project_id"`
	PersonID  *int64    `json:"person_id,omitempty"`
	TeamID    *int64    `json:"team_id,omitempty"`
	Role      string    `json:"role,omitempty"`
	AddedAt   time.Time `json:"added_at"`

	// Loaded relationships
	Person *Person `json:"person,omitempty"`
	Team   *Team   `json:"team,omitempty"`
}

// ResolvedEntity represents the result of entity resolution.
type ResolvedEntity struct {
	PersonID   int64   `json:"person_id"`
	Confidence float32 `json:"confidence"`
	Source     string  `json:"source"` // exact_match, alias, inferred
	IsNew      bool    `json:"is_new"` // True if person was just created
}

// ResolutionResult contains the full result of resolution.
type ResolutionResult struct {
	Person     *Person `json:"person"`
	Confidence float32 `json:"confidence"`
	Source     string  `json:"source"`
	IsNew      bool    `json:"is_new"`
}

// IsBot returns true if this is a bot account.
func (p *Person) IsBot() bool {
	return p.AccountType == AccountTypeBot || p.AccountType == AccountTypeExternalService
}

// IsDistributionList returns true if this is a distribution list.
func (p *Person) IsDistributionList() bool {
	return p.AccountType == AccountTypeDistribution
}

// HasEmail returns true if the person has the given email as primary or alias.
func (p *Person) HasEmail(email string) bool {
	if p.PrimaryEmail == email {
		return true
	}
	for _, alias := range p.Aliases {
		if alias.AliasType == AliasTypeEmail && alias.AliasValue == email {
			return true
		}
	}
	return false
}

// IsRejected returns true if this entity has been soft-deleted.
func (p *Person) IsRejected() bool {
	return p.RejectedAt != nil
}

// EntityFilterRule represents a pattern-based rule for rejecting entity creation.
type EntityFilterRule struct {
	ID           int64     `json:"id,omitempty"`
	TenantID     string    `json:"tenant_id"`
	EmailPattern string    `json:"email_pattern,omitempty"`
	NamePattern  string    `json:"name_pattern,omitempty"`
	EntityType   string    `json:"entity_type,omitempty"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
	CreatedBy    string    `json:"created_by,omitempty"`
}

// EntityStats provides statistics about entities in the system.
type EntityStats struct {
	TotalPeople     int64                     `json:"total_people"`
	TotalRejected   int64                     `json:"total_rejected"`
	ByAccountType   map[AccountType]int64     `json:"by_account_type"`
	ByConfidence    map[string]int64          `json:"by_confidence"` // high (0.8+), medium (0.5-0.8), low (<0.5)
	NeedingReview   int64                     `json:"needing_review"`
	AutoCreated     int64                     `json:"auto_created"`
	Internal        int64                     `json:"internal"`
	External        int64                     `json:"external"`
}

// AccountTypePatterns contains configurable patterns for account type detection.
// These patterns are merged with hardcoded defaults, not replacing them.
type AccountTypePatterns struct {
	BotPatterns          []string `json:"bot_patterns,omitempty"`
	DistributionPatterns []string `json:"distribution_patterns,omitempty"`
	RolePatterns         []string `json:"role_patterns,omitempty"`
	ExternalDomains      []string `json:"external_domains,omitempty"`
}
