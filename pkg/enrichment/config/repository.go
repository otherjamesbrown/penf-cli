package config

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConfigRepositoryImpl implements the ConfigRepository interface.
type ConfigRepositoryImpl struct {
	pool *pgxpool.Pool
}

// NewConfigRepository creates a new config repository.
func NewConfigRepository(pool *pgxpool.Pool) *ConfigRepositoryImpl {
	return &ConfigRepositoryImpl{
		pool: pool,
	}
}

// ==================== ConfigRepository Interface Methods ====================

// GetTenant retrieves a tenant by ID.
func (r *ConfigRepositoryImpl) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	query := `
		SELECT id, name, slug, is_active, created_at
		FROM tenants
		WHERE id = $1
	`

	t := &Tenant{}
	err := r.pool.QueryRow(ctx, query, tenantID).Scan(
		&t.ID, &t.Name, &t.Slug, &t.IsActive, &t.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	return t, nil
}

// GetTenantBySlug retrieves a tenant by slug.
func (r *ConfigRepositoryImpl) GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error) {
	query := `
		SELECT id, name, slug, is_active, created_at
		FROM tenants
		WHERE slug = $1
	`

	t := &Tenant{}
	err := r.pool.QueryRow(ctx, query, slug).Scan(
		&t.ID, &t.Name, &t.Slug, &t.IsActive, &t.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant by slug: %w", err)
	}

	return t, nil
}

// GetTenantDomains retrieves domains for a tenant.
func (r *ConfigRepositoryImpl) GetTenantDomains(ctx context.Context, tenantID string) ([]TenantDomain, error) {
	query := `
		SELECT id, tenant_id, domain, domain_type, COALESCE(notes, '')
		FROM tenant_domains
		WHERE tenant_id = $1
		ORDER BY domain_type, domain
		LIMIT 1000
	`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant domains: %w", err)
	}
	defer rows.Close()

	var domains []TenantDomain
	for rows.Next() {
		var d TenantDomain
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Domain, &d.DomainType, &d.Notes); err != nil {
			return nil, fmt.Errorf("failed to scan domain: %w", err)
		}
		domains = append(domains, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating domains: %w", err)
	}

	return domains, nil
}

// GetTenantEmailPatterns retrieves email patterns for a tenant (enabled only).
func (r *ConfigRepositoryImpl) GetTenantEmailPatterns(ctx context.Context, tenantID string) ([]TenantEmailPattern, error) {
	query := `
		SELECT id, tenant_id, pattern, pattern_type, priority, enabled
		FROM tenant_email_patterns
		WHERE tenant_id = $1 AND enabled = true
		ORDER BY priority ASC, id ASC
		LIMIT 1000
	`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant email patterns: %w", err)
	}
	defer rows.Close()

	var patterns []TenantEmailPattern
	for rows.Next() {
		var p TenantEmailPattern
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Pattern, &p.PatternType, &p.Priority, &p.Enabled); err != nil {
			return nil, fmt.Errorf("failed to scan email pattern: %w", err)
		}
		patterns = append(patterns, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating email patterns: %w", err)
	}

	return patterns, nil
}

// GetTenantProcessingRules retrieves processing rules for a tenant.
func (r *ConfigRepositoryImpl) GetTenantProcessingRules(ctx context.Context, tenantID string) ([]TenantProcessingRule, error) {
	query := `
		SELECT id, tenant_id, rule_name, priority, match_conditions, classification_override, enabled
		FROM tenant_processing_rules
		WHERE tenant_id = $1
		ORDER BY priority ASC, id ASC
		LIMIT 1000
	`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant processing rules: %w", err)
	}
	defer rows.Close()

	var rules []TenantProcessingRule
	for rows.Next() {
		var rule TenantProcessingRule
		if err := rows.Scan(
			&rule.ID,
			&rule.TenantID,
			&rule.RuleName,
			&rule.Priority,
			&rule.MatchConditions,
			&rule.ClassificationOverride,
			&rule.Enabled,
		); err != nil {
			return nil, fmt.Errorf("failed to scan processing rule: %w", err)
		}
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating processing rules: %w", err)
	}

	return rules, nil
}

// GetTenantIntegrations retrieves integrations for a tenant.
func (r *ConfigRepositoryImpl) GetTenantIntegrations(ctx context.Context, tenantID string) ([]TenantIntegration, error) {
	query := `
		SELECT id, tenant_id, integration_type, name, instance_url, config, credentials_key, enabled, sync_status, last_sync_at
		FROM tenant_integrations
		WHERE tenant_id = $1
		ORDER BY integration_type, name
		LIMIT 1000
	`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant integrations: %w", err)
	}
	defer rows.Close()

	var integrations []TenantIntegration
	for rows.Next() {
		var i TenantIntegration
		if err := rows.Scan(
			&i.ID,
			&i.TenantID,
			&i.IntegrationType,
			&i.Name,
			&i.InstanceURL,
			&i.Config,
			&i.CredentialsKey,
			&i.Enabled,
			&i.SyncStatus,
			&i.LastSyncAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan integration: %w", err)
		}
		integrations = append(integrations, i)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating integrations: %w", err)
	}

	return integrations, nil
}

// ==================== CRUD Methods for Email Patterns ====================

// CreateEmailPattern creates a new email pattern.
func (r *ConfigRepositoryImpl) CreateEmailPattern(ctx context.Context, tenantID string, pattern, patternType, notes string) (*TenantEmailPattern, error) {
	query := `
		INSERT INTO tenant_email_patterns (
			tenant_id, pattern, pattern_type, notes, priority, enabled, created_at
		) VALUES ($1, $2, $3, $4, 100, true, NOW())
		RETURNING id, tenant_id, pattern, pattern_type, priority, enabled
	`

	p := &TenantEmailPattern{}
	err := r.pool.QueryRow(ctx, query, tenantID, pattern, patternType, nullIfEmpty(notes)).Scan(
		&p.ID, &p.TenantID, &p.Pattern, &p.PatternType, &p.Priority, &p.Enabled,
	)

	if err != nil {
		// Check for unique constraint violation
		if isPgUniqueViolation(err) {
			return nil, fmt.Errorf("duplicate pattern for tenant: %w", err)
		}
		// Check for check constraint violation (invalid pattern_type)
		if isPgCheckViolation(err) {
			return nil, fmt.Errorf("invalid pattern_type (must be: bot, distribution_list, role_account, ignore): %w", err)
		}
		return nil, fmt.Errorf("failed to create email pattern: %w", err)
	}

	return p, nil
}

// DeleteEmailPattern deletes an email pattern.
func (r *ConfigRepositoryImpl) DeleteEmailPattern(ctx context.Context, tenantID string, patternID int64) error {
	query := `
		DELETE FROM tenant_email_patterns
		WHERE tenant_id = $1 AND id = $2
	`

	result, err := r.pool.Exec(ctx, query, tenantID, patternID)
	if err != nil {
		return fmt.Errorf("failed to delete email pattern: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("email pattern not found: %d", patternID)
	}

	return nil
}

// ListEmailPatterns returns ALL email patterns for a tenant (enabled + disabled).
func (r *ConfigRepositoryImpl) ListEmailPatterns(ctx context.Context, tenantID string) ([]TenantEmailPattern, error) {
	query := `
		SELECT id, tenant_id, pattern, pattern_type, priority, enabled
		FROM tenant_email_patterns
		WHERE tenant_id = $1
		ORDER BY priority ASC, id ASC
		LIMIT 1000
	`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list email patterns: %w", err)
	}
	defer rows.Close()

	var patterns []TenantEmailPattern
	for rows.Next() {
		var p TenantEmailPattern
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Pattern, &p.PatternType, &p.Priority, &p.Enabled); err != nil {
			return nil, fmt.Errorf("failed to scan email pattern: %w", err)
		}
		patterns = append(patterns, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating email patterns: %w", err)
	}

	return patterns, nil
}

// ==================== Helper Functions ====================

// nullIfEmpty returns nil if string is empty, otherwise returns pointer to string.
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// isPgUniqueViolation checks if error is a PostgreSQL unique constraint violation.
func isPgUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// Check for "duplicate key" in error message (pgx error format)
	errStr := err.Error()
	return contains(errStr, "duplicate key") || contains(errStr, "unique constraint")
}

// isPgCheckViolation checks if error is a PostgreSQL check constraint violation.
func isPgCheckViolation(err error) bool {
	if err == nil {
		return false
	}
	// Check for "check constraint" in error message
	errStr := err.Error()
	return contains(errStr, "check constraint") || contains(errStr, "violates check")
}

// contains checks if a string contains a substring (case-insensitive helper).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && indexSubstring(s, substr) >= 0)
}

// indexSubstring finds the index of a substring in a string.
func indexSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
