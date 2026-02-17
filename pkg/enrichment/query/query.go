// Package query provides a shared query library for accessing enrichment data.
package query

import (
	"context"
	"time"
)

// TimeRange represents a time range for queries.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// NewTimeRange creates a time range ending now and starting the given duration ago.
func NewTimeRange(duration time.Duration) TimeRange {
	now := time.Now()
	return TimeRange{
		Start: now.Add(-duration),
		End:   now,
	}
}

// Last7Days returns a time range for the last 7 days.
func Last7Days() TimeRange {
	return NewTimeRange(7 * 24 * time.Hour)
}

// Last30Days returns a time range for the last 30 days.
func Last30Days() TimeRange {
	return NewTimeRange(30 * 24 * time.Hour)
}

// Pagination represents pagination parameters.
type Pagination struct {
	Limit  int
	Offset int
}

// DefaultPagination returns default pagination (50 items).
func DefaultPagination() Pagination {
	return Pagination{Limit: 50, Offset: 0}
}

// SortOrder represents sort direction.
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// Queries combines all query interfaces for convenient access.
type Queries struct {
	People     PeopleQuery
	Threads    ThreadsQuery
	Assertions AssertionsQuery
	Jira       JiraQuery
	Projects   ProjectsQuery
	Status     StatusQuery
}

// NewQueries creates a new combined query interface.
func NewQueries(repo Repository) *Queries {
	return &Queries{
		People:     NewPeopleQueryImpl(repo),
		Threads:    NewThreadsQueryImpl(repo),
		Assertions: NewAssertionsQueryImpl(repo),
		Jira:       NewJiraQueryImpl(repo),
		Projects:   NewProjectsQueryImpl(repo),
		Status:     NewStatusQueryImpl(repo),
	}
}

// Repository provides the underlying data access layer.
type Repository interface {
	// Person queries
	GetPerson(ctx context.Context, id int64) (*Person, error)
	SearchPeople(ctx context.Context, query string, filters PeopleFilters, page Pagination) ([]*Person, error)
	GetPersonCommunications(ctx context.Context, personID int64, timeRange TimeRange, page Pagination) ([]*Source, error)

	// Thread queries
	GetThread(ctx context.Context, id string) (*Thread, error)
	GetThreadsForPerson(ctx context.Context, personID int64, timeRange TimeRange, page Pagination) ([]*Thread, error)
	GetThreadsForProject(ctx context.Context, projectID int64, timeRange TimeRange, page Pagination) ([]*Thread, error)
	GetThreadMessages(ctx context.Context, threadID string, page Pagination) ([]*Source, error)

	// Assertion queries
	GetAssertions(ctx context.Context, filters AssertionFilters, page Pagination) ([]*Assertion, error)
	GetOpenActions(ctx context.Context, assigneeID *int64) ([]*Assertion, error)
	GetRecentDecisions(ctx context.Context, projectID *int64, days int) ([]*Assertion, error)

	// Jira queries
	GetJiraTicket(ctx context.Context, key string) (*JiraTicket, error)
	GetTicketReferences(ctx context.Context, key string, page Pagination) ([]*Source, error)
	GetTicketHistory(ctx context.Context, key string, page Pagination) ([]*TicketChange, error)

	// Project queries
	GetProject(ctx context.Context, id int64) (*Project, error)
	GetProjectActivity(ctx context.Context, id int64, timeRange TimeRange, page Pagination) ([]*ActivityItem, error)

	// Status queries
	GetEnrichmentStatus(ctx context.Context, sourceID int64) (*EnrichmentStatus, error)
	GetEnrichmentStats(ctx context.Context, tenantID string, timeRange TimeRange) (*EnrichmentStats, error)
}
