package products

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

// Team-related errors.
var (
	ErrTeamAssociationExists = errors.New("team is already associated with this product")
	ErrRoleExists            = errors.New("role already exists for this person on this product-team")
)

// ==================== ProductTeam Operations ====================

// AssociateTeam associates a team with a product.
func (r *Repository) AssociateTeam(ctx context.Context, pt *ProductTeam) error {
	query := `
		INSERT INTO product_teams (
			tenant_id, product_id, team_id, context,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			NOW(), NOW()
		)
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		pt.TenantID,
		pt.ProductID,
		pt.TeamID,
		nullIfEmpty(pt.Context),
	).Scan(&pt.ID, &pt.CreatedAt, &pt.UpdatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "idx_product_teams_unique") {
			return ErrTeamAssociationExists
		}
		return fmt.Errorf("failed to associate team: %w", err)
	}

	r.logger.Debug("Team associated with product",
		logging.F("id", pt.ID),
		logging.F("product_id", pt.ProductID),
		logging.F("team_id", pt.TeamID),
	)

	return nil
}

// DissociateTeam removes a team association from a product.
func (r *Repository) DissociateTeam(ctx context.Context, productID, teamID int64, context string) error {
	query := `
		DELETE FROM product_teams
		WHERE product_id = $1 AND team_id = $2 AND COALESCE(context, '') = COALESCE($3, '')
	`

	result, err := r.pool.Exec(ctx, query, productID, teamID, nullIfEmpty(context))
	if err != nil {
		return fmt.Errorf("failed to dissociate team: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetProductTeam retrieves a product-team association by ID.
func (r *Repository) GetProductTeam(ctx context.Context, id int64) (*ProductTeam, error) {
	query := `
		SELECT
			pt.id, pt.tenant_id, pt.product_id, pt.team_id, pt.context,
			pt.created_at, pt.updated_at,
			p.name as product_name, t.name as team_name
		FROM product_teams pt
		JOIN products p ON pt.product_id = p.id
		JOIN teams t ON pt.team_id = t.id
		WHERE pt.id = $1
	`

	pt := &ProductTeam{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&pt.ID, &pt.TenantID, &pt.ProductID, &pt.TeamID, &pt.Context,
		&pt.CreatedAt, &pt.UpdatedAt,
		&pt.ProductName, &pt.TeamName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get product team: %w", err)
	}
	return pt, nil
}

// GetTeamsForProduct retrieves all teams associated with a product.
func (r *Repository) GetTeamsForProduct(ctx context.Context, productID int64) ([]*ProductTeam, error) {
	query := `
		SELECT
			pt.id, pt.tenant_id, pt.product_id, pt.team_id, pt.context,
			pt.created_at, pt.updated_at,
			p.name as product_name, t.name as team_name
		FROM product_teams pt
		JOIN products p ON pt.product_id = p.id
		JOIN teams t ON pt.team_id = t.id
		WHERE pt.product_id = $1
		ORDER BY pt.context NULLS LAST, t.name
	`

	rows, err := r.pool.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams for product: %w", err)
	}
	defer rows.Close()

	return r.scanProductTeams(rows)
}

// GetProductsForTeam retrieves all products a team is associated with.
func (r *Repository) GetProductsForTeam(ctx context.Context, teamID int64) ([]*ProductTeam, error) {
	query := `
		SELECT
			pt.id, pt.tenant_id, pt.product_id, pt.team_id, pt.context,
			pt.created_at, pt.updated_at,
			p.name as product_name, t.name as team_name
		FROM product_teams pt
		JOIN products p ON pt.product_id = p.id
		JOIN teams t ON pt.team_id = t.id
		WHERE pt.team_id = $1
		ORDER BY p.name, pt.context NULLS LAST
	`

	rows, err := r.pool.Query(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get products for team: %w", err)
	}
	defer rows.Close()

	return r.scanProductTeams(rows)
}

// ==================== ProductTeamRole Operations ====================

// AddRole adds a role assignment.
func (r *Repository) AddRole(ctx context.Context, role *ProductTeamRole) error {
	query := `
		INSERT INTO product_team_roles (
			tenant_id, product_team_id, person_id,
			role, scope, is_active, started_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7,
			NOW(), NOW()
		)
		RETURNING id, created_at, updated_at
	`

	if role.StartedAt.IsZero() {
		role.StartedAt = time.Now()
	}
	role.IsActive = true

	err := r.pool.QueryRow(ctx, query,
		role.TenantID,
		role.ProductTeamID,
		role.PersonID,
		role.Role,
		nullIfEmpty(role.Scope),
		role.IsActive,
		role.StartedAt,
	).Scan(&role.ID, &role.CreatedAt, &role.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to add role: %w", err)
	}

	r.logger.Debug("Role added",
		logging.F("id", role.ID),
		logging.F("role", role.Role),
		logging.F("person_id", role.PersonID),
	)

	return nil
}

// UpdateRole updates an existing role.
func (r *Repository) UpdateRole(ctx context.Context, role *ProductTeamRole) error {
	query := `
		UPDATE product_team_roles SET
			role = $2,
			scope = $3,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		role.ID,
		role.Role,
		nullIfEmpty(role.Scope),
	).Scan(&role.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to update role: %w", err)
	}

	return nil
}

// EndRole ends a role assignment (sets is_active=false and ended_at).
func (r *Repository) EndRole(ctx context.Context, roleID int64) error {
	query := `
		UPDATE product_team_roles SET
			is_active = FALSE,
			ended_at = NOW(),
			updated_at = NOW()
		WHERE id = $1 AND is_active = TRUE
	`

	result, err := r.pool.Exec(ctx, query, roleID)
	if err != nil {
		return fmt.Errorf("failed to end role: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetRole retrieves a role by ID.
func (r *Repository) GetRole(ctx context.Context, id int64) (*ProductTeamRole, error) {
	query := `
		SELECT
			ptr.id, ptr.tenant_id, ptr.product_team_id, ptr.person_id,
			ptr.role, ptr.scope, ptr.is_active, ptr.started_at, ptr.ended_at,
			ptr.created_at, ptr.updated_at,
			p.name as product_name, t.name as team_name, pt.context as team_context,
			pe.canonical_name as person_name, pe.primary_email as person_email
		FROM product_team_roles ptr
		JOIN product_teams pt ON ptr.product_team_id = pt.id
		JOIN products p ON pt.product_id = p.id
		JOIN teams t ON pt.team_id = t.id
		JOIN people pe ON ptr.person_id = pe.id
		WHERE ptr.id = $1
	`

	role := &ProductTeamRole{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&role.ID, &role.TenantID, &role.ProductTeamID, &role.PersonID,
		&role.Role, &role.Scope, &role.IsActive, &role.StartedAt, &role.EndedAt,
		&role.CreatedAt, &role.UpdatedAt,
		&role.ProductName, &role.TeamName, &role.TeamContext,
		&role.PersonName, &role.PersonEmail,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}
	return role, nil
}

// GetRolesForProductTeam retrieves all roles for a product-team association.
func (r *Repository) GetRolesForProductTeam(ctx context.Context, productTeamID int64, activeOnly bool) ([]*ProductTeamRole, error) {
	query := `
		SELECT
			ptr.id, ptr.tenant_id, ptr.product_team_id, ptr.person_id,
			ptr.role, ptr.scope, ptr.is_active, ptr.started_at, ptr.ended_at,
			ptr.created_at, ptr.updated_at,
			p.name as product_name, t.name as team_name, pt.context as team_context,
			pe.canonical_name as person_name, pe.primary_email as person_email
		FROM product_team_roles ptr
		JOIN product_teams pt ON ptr.product_team_id = pt.id
		JOIN products p ON pt.product_id = p.id
		JOIN teams t ON pt.team_id = t.id
		JOIN people pe ON ptr.person_id = pe.id
		WHERE ptr.product_team_id = $1
	`
	if activeOnly {
		query += " AND ptr.is_active = TRUE"
	}
	query += " ORDER BY ptr.role, ptr.scope NULLS LAST, pe.canonical_name"

	rows, err := r.pool.Query(ctx, query, productTeamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for product team: %w", err)
	}
	defer rows.Close()

	return r.scanRoles(rows)
}

// GetRolesForPerson retrieves all roles for a person.
func (r *Repository) GetRolesForPerson(ctx context.Context, personID int64, activeOnly bool) ([]*ProductTeamRole, error) {
	query := `
		SELECT
			ptr.id, ptr.tenant_id, ptr.product_team_id, ptr.person_id,
			ptr.role, ptr.scope, ptr.is_active, ptr.started_at, ptr.ended_at,
			ptr.created_at, ptr.updated_at,
			p.name as product_name, t.name as team_name, pt.context as team_context,
			pe.canonical_name as person_name, pe.primary_email as person_email
		FROM product_team_roles ptr
		JOIN product_teams pt ON ptr.product_team_id = pt.id
		JOIN products p ON pt.product_id = p.id
		JOIN teams t ON pt.team_id = t.id
		JOIN people pe ON ptr.person_id = pe.id
		WHERE ptr.person_id = $1
	`
	if activeOnly {
		query += " AND ptr.is_active = TRUE"
	}
	query += " ORDER BY p.name, t.name, ptr.role"

	rows, err := r.pool.Query(ctx, query, personID)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for person: %w", err)
	}
	defer rows.Close()

	return r.scanRoles(rows)
}

// ==================== Role Query Operations ====================

// FindByRole finds people with a specific role, optionally scoped to product/team/scope.
// This is the "who is the DRI for networking on MTC" query.
func (r *Repository) FindByRole(ctx context.Context, q RoleQuery) ([]*ProductTeamRole, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("ptr.tenant_id = $%d", argIdx))
	args = append(args, q.TenantID)
	argIdx++

	if q.Role != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(ptr.role) = LOWER($%d)", argIdx))
		args = append(args, q.Role)
		argIdx++
	}

	if q.ProductName != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(p.name) = LOWER($%d)", argIdx))
		args = append(args, q.ProductName)
		argIdx++
	}

	if q.TeamName != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(t.name) = LOWER($%d)", argIdx))
		args = append(args, q.TeamName)
		argIdx++
	}

	if q.Scope != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(ptr.scope) LIKE '%%' || LOWER($%d) || '%%'", argIdx))
		args = append(args, q.Scope)
		argIdx++
	}

	if q.PersonName != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(pe.canonical_name) LIKE '%%' || LOWER($%d) || '%%'", argIdx))
		args = append(args, q.PersonName)
		argIdx++
	}

	if q.ActiveOnly {
		conditions = append(conditions, "ptr.is_active = TRUE")
	}

	query := fmt.Sprintf(`
		SELECT
			ptr.id, ptr.tenant_id, ptr.product_team_id, ptr.person_id,
			ptr.role, ptr.scope, ptr.is_active, ptr.started_at, ptr.ended_at,
			ptr.created_at, ptr.updated_at,
			p.name as product_name, t.name as team_name, pt.context as team_context,
			pe.canonical_name as person_name, pe.primary_email as person_email
		FROM product_team_roles ptr
		JOIN product_teams pt ON ptr.product_team_id = pt.id
		JOIN products p ON pt.product_id = p.id
		JOIN teams t ON pt.team_id = t.id
		JOIN people pe ON ptr.person_id = pe.id
		WHERE %s
		ORDER BY p.name, t.name, ptr.role, pe.canonical_name
	`, strings.Join(conditions, " AND "))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to find by role: %w", err)
	}
	defer rows.Close()

	return r.scanRoles(rows)
}

// GetPeopleOnProduct retrieves all people working on a product (across all teams).
func (r *Repository) GetPeopleOnProduct(ctx context.Context, productID int64, activeOnly bool) ([]*ProductTeamRole, error) {
	query := `
		SELECT
			ptr.id, ptr.tenant_id, ptr.product_team_id, ptr.person_id,
			ptr.role, ptr.scope, ptr.is_active, ptr.started_at, ptr.ended_at,
			ptr.created_at, ptr.updated_at,
			p.name as product_name, t.name as team_name, pt.context as team_context,
			pe.canonical_name as person_name, pe.primary_email as person_email
		FROM product_team_roles ptr
		JOIN product_teams pt ON ptr.product_team_id = pt.id
		JOIN products p ON pt.product_id = p.id
		JOIN teams t ON pt.team_id = t.id
		JOIN people pe ON ptr.person_id = pe.id
		WHERE pt.product_id = $1
	`
	if activeOnly {
		query += " AND ptr.is_active = TRUE"
	}
	query += " ORDER BY t.name, ptr.role, pe.canonical_name"

	rows, err := r.pool.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get people on product: %w", err)
	}
	defer rows.Close()

	return r.scanRoles(rows)
}

// GetPeopleOnProductByCountry retrieves people on a product filtered by country.
// This is the "who is on MTC in Poland" query.
func (r *Repository) GetPeopleOnProductByCountry(ctx context.Context, productID int64, country string, activeOnly bool) ([]*ProductTeamRole, error) {
	query := `
		SELECT
			ptr.id, ptr.tenant_id, ptr.product_team_id, ptr.person_id,
			ptr.role, ptr.scope, ptr.is_active, ptr.started_at, ptr.ended_at,
			ptr.created_at, ptr.updated_at,
			p.name as product_name, t.name as team_name, pt.context as team_context,
			pe.canonical_name as person_name, pe.primary_email as person_email
		FROM product_team_roles ptr
		JOIN product_teams pt ON ptr.product_team_id = pt.id
		JOIN products p ON pt.product_id = p.id
		JOIN teams t ON pt.team_id = t.id
		JOIN people pe ON ptr.person_id = pe.id
		WHERE pt.product_id = $1 AND LOWER(pe.country) = LOWER($2)
	`
	if activeOnly {
		query += " AND ptr.is_active = TRUE"
	}
	query += " ORDER BY t.name, ptr.role, pe.canonical_name"

	rows, err := r.pool.Query(ctx, query, productID, country)
	if err != nil {
		return nil, fmt.Errorf("failed to get people by country: %w", err)
	}
	defer rows.Close()

	return r.scanRoles(rows)
}

// ==================== Helper Functions ====================

func (r *Repository) scanProductTeams(rows pgx.Rows) ([]*ProductTeam, error) {
	var teams []*ProductTeam
	for rows.Next() {
		pt := &ProductTeam{}
		err := rows.Scan(
			&pt.ID, &pt.TenantID, &pt.ProductID, &pt.TeamID, &pt.Context,
			&pt.CreatedAt, &pt.UpdatedAt,
			&pt.ProductName, &pt.TeamName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product team: %w", err)
		}
		teams = append(teams, pt)
	}
	return teams, rows.Err()
}

func (r *Repository) scanRoles(rows pgx.Rows) ([]*ProductTeamRole, error) {
	var roles []*ProductTeamRole
	for rows.Next() {
		role := &ProductTeamRole{}
		var scope, teamContext, personEmail *string
		err := rows.Scan(
			&role.ID, &role.TenantID, &role.ProductTeamID, &role.PersonID,
			&role.Role, &scope, &role.IsActive, &role.StartedAt, &role.EndedAt,
			&role.CreatedAt, &role.UpdatedAt,
			&role.ProductName, &role.TeamName, &teamContext,
			&role.PersonName, &personEmail,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		if scope != nil {
			role.Scope = *scope
		}
		if teamContext != nil {
			role.TeamContext = *teamContext
		}
		if personEmail != nil {
			role.PersonEmail = *personEmail
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}
