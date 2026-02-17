package query

import "context"

// JiraQuery provides queries for Jira ticket data.
type JiraQuery interface {
	// GetTicket retrieves a Jira ticket by key.
	GetTicket(ctx context.Context, key string) (*JiraTicket, error)

	// GetTicketReferences retrieves emails referencing a ticket.
	GetTicketReferences(ctx context.Context, key string) ([]*Source, error)

	// GetTicketHistory retrieves change history for a ticket.
	GetTicketHistory(ctx context.Context, key string) ([]*TicketChange, error)
}

// JiraQueryImpl implements JiraQuery.
type JiraQueryImpl struct {
	repo Repository
}

// NewJiraQueryImpl creates a new Jira query implementation.
func NewJiraQueryImpl(repo Repository) *JiraQueryImpl {
	return &JiraQueryImpl{repo: repo}
}

// GetTicket retrieves a Jira ticket by key.
func (q *JiraQueryImpl) GetTicket(ctx context.Context, key string) (*JiraTicket, error) {
	return q.repo.GetJiraTicket(ctx, key)
}

// GetTicketReferences retrieves emails referencing a ticket.
func (q *JiraQueryImpl) GetTicketReferences(ctx context.Context, key string) ([]*Source, error) {
	return q.repo.GetTicketReferences(ctx, key, DefaultPagination())
}

// GetTicketHistory retrieves change history for a ticket.
func (q *JiraQueryImpl) GetTicketHistory(ctx context.Context, key string) ([]*TicketChange, error) {
	return q.repo.GetTicketHistory(ctx, key, DefaultPagination())
}
