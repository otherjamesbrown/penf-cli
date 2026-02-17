package query

import "context"

// ProjectsQuery provides queries for project data.
type ProjectsQuery interface {
	// GetProject retrieves a project by ID.
	GetProject(ctx context.Context, id int64) (*Project, error)

	// GetProjectActivity retrieves activity for a project.
	GetProjectActivity(ctx context.Context, id int64, timeRange TimeRange) ([]*ActivityItem, error)
}

// ProjectsQueryImpl implements ProjectsQuery.
type ProjectsQueryImpl struct {
	repo Repository
}

// NewProjectsQueryImpl creates a new projects query implementation.
func NewProjectsQueryImpl(repo Repository) *ProjectsQueryImpl {
	return &ProjectsQueryImpl{repo: repo}
}

// GetProject retrieves a project by ID.
func (q *ProjectsQueryImpl) GetProject(ctx context.Context, id int64) (*Project, error) {
	return q.repo.GetProject(ctx, id)
}

// GetProjectActivity retrieves activity for a project.
func (q *ProjectsQueryImpl) GetProjectActivity(ctx context.Context, id int64, timeRange TimeRange) ([]*ActivityItem, error) {
	return q.repo.GetProjectActivity(ctx, id, timeRange, DefaultPagination())
}
