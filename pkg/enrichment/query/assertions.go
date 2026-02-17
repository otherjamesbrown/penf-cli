package query

import "context"

// AssertionsQuery provides queries for extracted assertions.
type AssertionsQuery interface {
	// GetAssertions retrieves assertions with filters.
	GetAssertions(ctx context.Context, filters AssertionFilters) ([]*Assertion, error)

	// GetOpenActions retrieves open actions, optionally filtered by assignee.
	GetOpenActions(ctx context.Context, assigneeID *int64) ([]*Assertion, error)

	// GetRecentDecisions retrieves recent decisions for a project.
	GetRecentDecisions(ctx context.Context, projectID *int64, days int) ([]*Assertion, error)

	// GetRisks retrieves risks, optionally filtered by project.
	GetRisks(ctx context.Context, projectID *int64, severities []string) ([]*Assertion, error)

	// GetCommitments retrieves commitments, optionally filtered by committer.
	GetCommitments(ctx context.Context, committerID *int64) ([]*Assertion, error)

	// GetQuestions retrieves unanswered questions.
	GetQuestions(ctx context.Context, projectID *int64, unansweredOnly bool) ([]*Assertion, error)
}

// AssertionsQueryImpl implements AssertionsQuery.
type AssertionsQueryImpl struct {
	repo Repository
}

// NewAssertionsQueryImpl creates a new assertions query implementation.
func NewAssertionsQueryImpl(repo Repository) *AssertionsQueryImpl {
	return &AssertionsQueryImpl{repo: repo}
}

// GetAssertions retrieves assertions with filters.
func (q *AssertionsQueryImpl) GetAssertions(ctx context.Context, filters AssertionFilters) ([]*Assertion, error) {
	return q.repo.GetAssertions(ctx, filters, DefaultPagination())
}

// GetOpenActions retrieves open actions.
func (q *AssertionsQueryImpl) GetOpenActions(ctx context.Context, assigneeID *int64) ([]*Assertion, error) {
	return q.repo.GetOpenActions(ctx, assigneeID)
}

// GetRecentDecisions retrieves recent decisions.
func (q *AssertionsQueryImpl) GetRecentDecisions(ctx context.Context, projectID *int64, days int) ([]*Assertion, error) {
	return q.repo.GetRecentDecisions(ctx, projectID, days)
}

// GetRisks retrieves risks with optional filters.
func (q *AssertionsQueryImpl) GetRisks(ctx context.Context, projectID *int64, severities []string) ([]*Assertion, error) {
	filters := AssertionFilters{
		Type:      "risk",
		ProjectID: projectID,
	}
	// Note: severity filtering would be done in the repo layer
	return q.repo.GetAssertions(ctx, filters, DefaultPagination())
}

// GetCommitments retrieves commitments.
func (q *AssertionsQueryImpl) GetCommitments(ctx context.Context, committerID *int64) ([]*Assertion, error) {
	filters := AssertionFilters{
		Type:    "commitment",
		OwnerID: committerID,
	}
	return q.repo.GetAssertions(ctx, filters, DefaultPagination())
}

// GetQuestions retrieves questions.
func (q *AssertionsQueryImpl) GetQuestions(ctx context.Context, projectID *int64, unansweredOnly bool) ([]*Assertion, error) {
	filters := AssertionFilters{
		Type:      "question",
		ProjectID: projectID,
	}
	if unansweredOnly {
		filters.Status = "unanswered"
	}
	return q.repo.GetAssertions(ctx, filters, DefaultPagination())
}
