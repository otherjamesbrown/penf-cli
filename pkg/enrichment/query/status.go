package query

import "context"

// StatusQuery provides queries for enrichment status and statistics.
type StatusQuery interface {
	// GetEnrichmentStatus retrieves the enrichment status for a source.
	GetEnrichmentStatus(ctx context.Context, sourceID int64) (*EnrichmentStatus, error)

	// GetEnrichmentStats retrieves aggregated enrichment statistics.
	GetEnrichmentStats(ctx context.Context, tenantID string, timeRange TimeRange) (*EnrichmentStats, error)
}

// StatusQueryImpl implements StatusQuery.
type StatusQueryImpl struct {
	repo Repository
}

// NewStatusQueryImpl creates a new status query implementation.
func NewStatusQueryImpl(repo Repository) *StatusQueryImpl {
	return &StatusQueryImpl{repo: repo}
}

// GetEnrichmentStatus retrieves the enrichment status for a source.
func (q *StatusQueryImpl) GetEnrichmentStatus(ctx context.Context, sourceID int64) (*EnrichmentStatus, error) {
	return q.repo.GetEnrichmentStatus(ctx, sourceID)
}

// GetEnrichmentStats retrieves aggregated enrichment statistics.
func (q *StatusQueryImpl) GetEnrichmentStats(ctx context.Context, tenantID string, timeRange TimeRange) (*EnrichmentStats, error) {
	return q.repo.GetEnrichmentStats(ctx, tenantID, timeRange)
}
