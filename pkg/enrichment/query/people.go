package query

import "context"

// PeopleQuery provides queries for person entities.
type PeopleQuery interface {
	// GetPerson retrieves a person by ID.
	GetPerson(ctx context.Context, id int64) (*Person, error)

	// SearchPeople searches for people matching the query.
	SearchPeople(ctx context.Context, query string, filters PeopleFilters) ([]*Person, error)

	// GetPersonCommunications retrieves communications for a person.
	GetPersonCommunications(ctx context.Context, personID int64, timeRange TimeRange) ([]*Source, error)
}

// PeopleQueryImpl implements PeopleQuery.
type PeopleQueryImpl struct {
	repo Repository
}

// NewPeopleQueryImpl creates a new people query implementation.
func NewPeopleQueryImpl(repo Repository) *PeopleQueryImpl {
	return &PeopleQueryImpl{repo: repo}
}

// GetPerson retrieves a person by ID.
func (q *PeopleQueryImpl) GetPerson(ctx context.Context, id int64) (*Person, error) {
	return q.repo.GetPerson(ctx, id)
}

// SearchPeople searches for people matching the query.
func (q *PeopleQueryImpl) SearchPeople(ctx context.Context, query string, filters PeopleFilters) ([]*Person, error) {
	return q.repo.SearchPeople(ctx, query, filters, DefaultPagination())
}

// GetPersonCommunications retrieves communications for a person.
func (q *PeopleQueryImpl) GetPersonCommunications(ctx context.Context, personID int64, timeRange TimeRange) ([]*Source, error) {
	return q.repo.GetPersonCommunications(ctx, personID, timeRange, DefaultPagination())
}
