package query

import "context"

// ThreadsQuery provides queries for email threads.
type ThreadsQuery interface {
	// GetThread retrieves a thread by ID.
	GetThread(ctx context.Context, id string) (*Thread, error)

	// GetThreadsForPerson retrieves threads involving a person.
	GetThreadsForPerson(ctx context.Context, personID int64, timeRange TimeRange) ([]*Thread, error)

	// GetThreadsForProject retrieves threads related to a project.
	GetThreadsForProject(ctx context.Context, projectID int64, timeRange TimeRange) ([]*Thread, error)

	// GetThreadMessages retrieves all messages in a thread.
	GetThreadMessages(ctx context.Context, threadID string) ([]*Source, error)
}

// ThreadsQueryImpl implements ThreadsQuery.
type ThreadsQueryImpl struct {
	repo Repository
}

// NewThreadsQueryImpl creates a new threads query implementation.
func NewThreadsQueryImpl(repo Repository) *ThreadsQueryImpl {
	return &ThreadsQueryImpl{repo: repo}
}

// GetThread retrieves a thread by ID.
func (q *ThreadsQueryImpl) GetThread(ctx context.Context, id string) (*Thread, error) {
	return q.repo.GetThread(ctx, id)
}

// GetThreadsForPerson retrieves threads involving a person.
func (q *ThreadsQueryImpl) GetThreadsForPerson(ctx context.Context, personID int64, timeRange TimeRange) ([]*Thread, error) {
	return q.repo.GetThreadsForPerson(ctx, personID, timeRange, DefaultPagination())
}

// GetThreadsForProject retrieves threads related to a project.
func (q *ThreadsQueryImpl) GetThreadsForProject(ctx context.Context, projectID int64, timeRange TimeRange) ([]*Thread, error) {
	return q.repo.GetThreadsForProject(ctx, projectID, timeRange, DefaultPagination())
}

// GetThreadMessages retrieves all messages in a thread.
func (q *ThreadsQueryImpl) GetThreadMessages(ctx context.Context, threadID string) ([]*Source, error) {
	return q.repo.GetThreadMessages(ctx, threadID, Pagination{Limit: 100, Offset: 0})
}
