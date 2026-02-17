package products

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pferrors "github.com/otherjamesbrown/penf-cli/pkg/errors"
	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

// Common errors - these are aliases to centralized errors for backward compatibility.
var (
	// ErrNotFound is returned when a product is not found.
	// Deprecated: Use pferrors.ErrNotFound and pferrors.IsNotFound() instead.
	ErrNotFound = pferrors.ErrNotFound

	// ErrAliasConflict is returned when an alias already exists for another product.
	// Deprecated: Use pferrors.ErrConflict and pferrors.IsConflict() instead.
	ErrAliasConflict = pferrors.ErrConflict
)

// Repository provides database operations for products.
type Repository struct {
	pool   *pgxpool.Pool
	logger logging.Logger
}

// NewRepository creates a new product repository.
func NewRepository(pool *pgxpool.Pool, logger logging.Logger) *Repository {
	return &Repository{
		pool:   pool,
		logger: logger.With(logging.F("component", "product_repository")),
	}
}

// Pool returns the underlying database pool.
func (r *Repository) Pool() *pgxpool.Pool {
	return r.pool
}

// ==================== Product CRUD ====================

// CreateProduct creates a new product.
func (r *Repository) CreateProduct(ctx context.Context, p *Product) error {
	query := `
		INSERT INTO products (
			tenant_id, name, description, parent_id,
			product_type, status, keywords,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			NOW(), NOW()
		)
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		p.TenantID,
		p.Name,
		p.Description,
		p.ParentID,
		p.ProductType,
		p.Status,
		p.Keywords,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create product: %w", err)
	}

	r.logger.Debug("Product created",
		logging.F("id", p.ID),
		logging.F("name", p.Name))

	return nil
}

// GetProductByID retrieves a product by ID.
func (r *Repository) GetProductByID(ctx context.Context, id int64) (*Product, error) {
	query := `
		SELECT
			id, tenant_id, name, description, parent_id,
			product_type, status, keywords,
			created_at, updated_at
		FROM products
		WHERE id = $1
	`
	return r.scanProduct(ctx, query, id)
}

// GetProductByName retrieves a product by name.
func (r *Repository) GetProductByName(ctx context.Context, tenantID, name string) (*Product, error) {
	query := `
		SELECT
			id, tenant_id, name, description, parent_id,
			product_type, status, keywords,
			created_at, updated_at
		FROM products
		WHERE tenant_id = $1 AND LOWER(name) = LOWER($2)
	`
	return r.scanProduct(ctx, query, tenantID, name)
}

// GetProductByAlias retrieves a product by alias.
func (r *Repository) GetProductByAlias(ctx context.Context, alias string) (*Product, error) {
	query := `
		SELECT
			p.id, p.tenant_id, p.name, p.description, p.parent_id,
			p.product_type, p.status, p.keywords,
			p.created_at, p.updated_at
		FROM products p
		JOIN product_aliases a ON a.product_id = p.id
		WHERE LOWER(a.alias) = LOWER($1)
	`
	return r.scanProduct(ctx, query, alias)
}

// ResolveProduct resolves a product by ID, name, or alias.
func (r *Repository) ResolveProduct(ctx context.Context, tenantID, nameOrAlias string) (*Product, error) {
	// Try by numeric ID first
	var id int64
	if _, err := fmt.Sscanf(nameOrAlias, "%d", &id); err == nil {
		p, err := r.GetProductByID(ctx, id)
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	// Try by name
	p, err := r.GetProductByName(ctx, tenantID, nameOrAlias)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	// Try by alias
	return r.GetProductByAlias(ctx, nameOrAlias)
}

// UpdateProduct updates an existing product.
func (r *Repository) UpdateProduct(ctx context.Context, p *Product) error {
	query := `
		UPDATE products SET
			name = $2,
			description = $3,
			parent_id = $4,
			product_type = $5,
			status = $6,
			keywords = $7,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		p.ID,
		p.Name,
		p.Description,
		p.ParentID,
		p.ProductType,
		p.Status,
		p.Keywords,
	).Scan(&p.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to update product: %w", err)
	}

	return nil
}

// DeleteProduct deletes a product by ID.
func (r *Repository) DeleteProduct(ctx context.Context, id int64) error {
	result, err := r.pool.Exec(ctx, "DELETE FROM products WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListProducts lists products with optional filtering.
func (r *Repository) ListProducts(ctx context.Context, filter ProductFilter) ([]*Product, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", argIdx))
	args = append(args, filter.TenantID)
	argIdx++

	if filter.ParentID != nil {
		conditions = append(conditions, fmt.Sprintf("parent_id = $%d", argIdx))
		args = append(args, *filter.ParentID)
		argIdx++
	}

	if filter.ProductType != nil {
		conditions = append(conditions, fmt.Sprintf("product_type = $%d", argIdx))
		args = append(args, *filter.ProductType)
		argIdx++
	}

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}

	if filter.NameSearch != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(name) LIKE '%%' || LOWER($%d) || '%%'", argIdx))
		args = append(args, filter.NameSearch)
		argIdx++
	}

	query := fmt.Sprintf(`
		SELECT
			id, tenant_id, name, description, parent_id,
			product_type, status, keywords,
			created_at, updated_at
		FROM products
		WHERE %s
		ORDER BY name
	`, strings.Join(conditions, " AND "))

	limit := 100
	if filter.Limit > 0 && filter.Limit <= 1000 {
		limit = filter.Limit
	} else if filter.Limit > 1000 {
		limit = 1000
	}
	query += fmt.Sprintf(" LIMIT %d", limit)
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}
	defer rows.Close()

	return r.scanProducts(rows)
}

// ListTopLevelProducts lists all top-level products (no parent).
func (r *Repository) ListTopLevelProducts(ctx context.Context, tenantID string) ([]*Product, error) {
	query := `
		SELECT
			id, tenant_id, name, description, parent_id,
			product_type, status, keywords,
			created_at, updated_at
		FROM products
		WHERE tenant_id = $1 AND parent_id IS NULL
		ORDER BY name
		LIMIT 1000
	`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list top-level products: %w", err)
	}
	defer rows.Close()

	return r.scanProducts(rows)
}

// ListChildren lists child products of a given product.
func (r *Repository) ListChildren(ctx context.Context, parentID int64) ([]*Product, error) {
	query := `
		SELECT
			id, tenant_id, name, description, parent_id,
			product_type, status, keywords,
			created_at, updated_at
		FROM products
		WHERE parent_id = $1
		ORDER BY name
		LIMIT 1000
	`

	rows, err := r.pool.Query(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list children: %w", err)
	}
	defer rows.Close()

	return r.scanProducts(rows)
}

// ==================== Hierarchy Operations ====================

// GetHierarchy retrieves the full hierarchy tree starting from a product.
func (r *Repository) GetHierarchy(ctx context.Context, productID int64) ([]*ProductWithHierarchy, error) {
	query := `
		WITH RECURSIVE product_tree AS (
			SELECT
				id, tenant_id, name, description, parent_id,
				product_type, status, keywords,
				created_at, updated_at,
				0 as depth,
				name::text as path
			FROM products
			WHERE id = $1

			UNION ALL

			SELECT
				p.id, p.tenant_id, p.name, p.description, p.parent_id,
				p.product_type, p.status, p.keywords,
				p.created_at, p.updated_at,
				pt.depth + 1,
				pt.path || ' > ' || p.name
			FROM products p
			JOIN product_tree pt ON p.parent_id = pt.id
		)
		SELECT * FROM product_tree ORDER BY depth, name
	`

	rows, err := r.pool.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get hierarchy: %w", err)
	}
	defer rows.Close()

	var results []*ProductWithHierarchy
	for rows.Next() {
		p := &Product{}
		h := &ProductWithHierarchy{Product: p}
		err := rows.Scan(
			&p.ID, &p.TenantID, &p.Name, &p.Description, &p.ParentID,
			&p.ProductType, &p.Status, &p.Keywords,
			&p.CreatedAt, &p.UpdatedAt,
			&h.Depth, &h.Path,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan hierarchy row: %w", err)
		}
		results = append(results, h)
	}

	return results, rows.Err()
}

// GetAncestors retrieves all ancestors of a product (path to root).
func (r *Repository) GetAncestors(ctx context.Context, productID int64) ([]*Product, error) {
	query := `
		WITH RECURSIVE ancestors AS (
			SELECT
				id, tenant_id, name, description, parent_id,
				product_type, status, keywords,
				created_at, updated_at,
				0 as depth
			FROM products
			WHERE id = $1

			UNION ALL

			SELECT
				p.id, p.tenant_id, p.name, p.description, p.parent_id,
				p.product_type, p.status, p.keywords,
				p.created_at, p.updated_at,
				a.depth + 1
			FROM products p
			JOIN ancestors a ON p.id = a.parent_id
		)
		SELECT
			id, tenant_id, name, description, parent_id,
			product_type, status, keywords,
			created_at, updated_at
		FROM ancestors
		WHERE depth > 0
		ORDER BY depth DESC
	`

	rows, err := r.pool.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ancestors: %w", err)
	}
	defer rows.Close()

	return r.scanProducts(rows)
}

// ==================== Alias Operations ====================

// AddAlias adds an alias for a product.
func (r *Repository) AddAlias(ctx context.Context, productID int64, alias string) error {
	query := `
		INSERT INTO product_aliases (product_id, alias, created_at)
		VALUES ($1, $2, NOW())
	`

	_, err := r.pool.Exec(ctx, query, productID, alias)
	if err != nil {
		// Check for unique constraint violation
		if strings.Contains(err.Error(), "idx_product_aliases_lookup") {
			return ErrAliasConflict
		}
		return fmt.Errorf("failed to add alias: %w", err)
	}

	return nil
}

// RemoveAlias removes an alias from a product.
func (r *Repository) RemoveAlias(ctx context.Context, productID int64, alias string) error {
	result, err := r.pool.Exec(ctx,
		"DELETE FROM product_aliases WHERE product_id = $1 AND LOWER(alias) = LOWER($2)",
		productID, alias,
	)
	if err != nil {
		return fmt.Errorf("failed to remove alias: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetAliases retrieves all aliases for a product.
func (r *Repository) GetAliases(ctx context.Context, productID int64) ([]*ProductAlias, error) {
	query := `
		SELECT id, product_id, alias, created_at
		FROM product_aliases
		WHERE product_id = $1
		ORDER BY alias
		LIMIT 1000
	`

	rows, err := r.pool.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get aliases: %w", err)
	}
	defer rows.Close()

	var aliases []*ProductAlias
	for rows.Next() {
		a := &ProductAlias{}
		err := rows.Scan(&a.ID, &a.ProductID, &a.Alias, &a.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alias: %w", err)
		}
		aliases = append(aliases, a)
	}

	return aliases, rows.Err()
}

// ==================== Helper Functions ====================

func (r *Repository) scanProduct(ctx context.Context, query string, args ...any) (*Product, error) {
	p := &Product{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.ParentID,
		&p.ProductType, &p.Status, &p.Keywords,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to scan product: %w", err)
	}
	return p, nil
}

func (r *Repository) scanProducts(rows pgx.Rows) ([]*Product, error) {
	var products []*Product
	for rows.Next() {
		p := &Product{}
		err := rows.Scan(
			&p.ID, &p.TenantID, &p.Name, &p.Description, &p.ParentID,
			&p.ProductType, &p.Status, &p.Keywords,
			&p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
