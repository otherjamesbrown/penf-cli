// Package mentions provides unified mention resolution for all entity types.
package mentions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements the Repository interface using PostgreSQL.
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository.
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// CreateMention creates a new mention record.
func (r *PostgresRepository) CreateMention(ctx context.Context, input MentionInput) (*ContentMention, error) {
	query := `
		INSERT INTO content_mentions (
			tenant_id, content_id, entity_type, mentioned_text,
			position, context_snippet, project_context_id, status, candidates
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, 'pending', '[]'::jsonb
		)
		RETURNING id, created_at
	`

	tenantID := getTenantFromContext(ctx)

	var id int64
	var createdAt time.Time

	err := r.db.QueryRow(ctx, query,
		tenantID,
		input.ContentID,
		string(input.EntityType),
		input.MentionedText,
		input.Position,
		nullableString(input.ContextSnippet),
		input.ProjectContextID,
	).Scan(&id, &createdAt)

	if err != nil {
		return nil, fmt.Errorf("creating mention: %w", err)
	}

	return &ContentMention{
		ID:               id,
		TenantID:         tenantID,
		ContentID:        input.ContentID,
		EntityType:       input.EntityType,
		MentionedText:    input.MentionedText,
		Position:         input.Position,
		ContextSnippet:   input.ContextSnippet,
		ProjectContextID: input.ProjectContextID,
		Status:           MentionStatusPending,
		Candidates:       []Candidate{},
		CreatedAt:        createdAt,
	}, nil
}

// GetMention retrieves a mention by ID.
func (r *PostgresRepository) GetMention(ctx context.Context, id int64) (*ContentMention, error) {
	query := `
		SELECT id, tenant_id, content_id, entity_type, mentioned_text,
			position, context_snippet, resolved_entity_id, resolution_confidence,
			resolution_source, resolved_expansion, candidates, status,
			resolved_at, resolved_by, project_context_id, created_at
		FROM content_mentions
		WHERE id = $1
	`

	row := r.db.QueryRow(ctx, query, id)
	return scanMention(row)
}

// ListMentions lists mentions based on filter criteria.
func (r *PostgresRepository) ListMentions(ctx context.Context, filter MentionFilter) ([]ContentMention, error) {
	query := `
		SELECT id, tenant_id, content_id, entity_type, mentioned_text,
			position, context_snippet, resolved_entity_id, resolution_confidence,
			resolution_source, resolved_expansion, candidates, status,
			resolved_at, resolved_by, project_context_id, created_at
		FROM content_mentions
		WHERE tenant_id = $1
	`
	args := []interface{}{filter.TenantID}
	argNum := 2

	if filter.ContentID != nil {
		query += fmt.Sprintf(" AND content_id = $%d", argNum)
		args = append(args, *filter.ContentID)
		argNum++
	}

	if filter.EntityType != nil {
		query += fmt.Sprintf(" AND entity_type = $%d", argNum)
		args = append(args, string(*filter.EntityType))
		argNum++
	}

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, string(*filter.Status))
		argNum++
	}

	if filter.ProjectID != nil {
		query += fmt.Sprintf(" AND project_context_id = $%d", argNum)
		args = append(args, *filter.ProjectID)
		argNum++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing mentions: %w", err)
	}
	defer rows.Close()

	var mentions []ContentMention
	for rows.Next() {
		m, err := scanMentionRow(rows)
		if err != nil {
			return nil, err
		}
		mentions = append(mentions, *m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating mentions: %w", err)
	}

	return mentions, nil
}

// UpdateMentionResolution updates a mention with resolution information.
func (r *PostgresRepository) UpdateMentionResolution(ctx context.Context, id int64, resolution ResolutionInput) error {
	query := `
		UPDATE content_mentions
		SET resolved_entity_id = $2,
			resolution_source = $3,
			resolution_confidence = 1.0,
			status = $4,
			resolved_at = NOW(),
			resolved_by = $5
		WHERE id = $1
	`

	status := MentionStatusUserResolved
	if resolution.Source == ResolutionSourceExactMatch ||
		resolution.Source == ResolutionSourceAlias ||
		resolution.Source == ResolutionSourceProjectContext ||
		resolution.Source == ResolutionSourcePriorLink {
		status = MentionStatusAutoResolved
	}

	_, err := r.db.Exec(ctx, query,
		id,
		resolution.EntityID,
		string(resolution.Source),
		string(status),
		nullableString(resolution.ResolvedBy),
	)
	if err != nil {
		return fmt.Errorf("updating mention resolution: %w", err)
	}

	return nil
}

// DismissMention marks a mention as dismissed.
func (r *PostgresRepository) DismissMention(ctx context.Context, id int64, dismissal DismissalInput) error {
	query := `
		UPDATE content_mentions
		SET status = 'dismissed',
			resolved_at = NOW(),
			resolved_by = $2
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, id, nullableString(dismissal.DismissedBy))
	if err != nil {
		return fmt.Errorf("dismissing mention: %w", err)
	}

	return nil
}

// GetPattern retrieves a pattern by text and optional project scope.
func (r *PostgresRepository) GetPattern(ctx context.Context, tenantID string, entityType EntityType, text string, projectID *int64) (*MentionPattern, error) {
	query := `
		SELECT id, tenant_id, entity_type, pattern_text, resolved_entity_id,
			resolved_expansion, project_id, is_permanent, times_seen,
			times_linked, last_seen_at, last_linked_at, first_content_id, created_at
		FROM mention_patterns
		WHERE tenant_id = $1 AND entity_type = $2 AND LOWER(pattern_text) = LOWER($3)
	`
	args := []interface{}{tenantID, string(entityType), text}

	if projectID != nil {
		query += " AND project_id = $4"
		args = append(args, *projectID)
	} else {
		query += " AND project_id IS NULL"
	}

	row := r.db.QueryRow(ctx, query, args...)
	return scanPattern(row)
}

// GetPatternsByText retrieves all patterns matching text (any project scope).
func (r *PostgresRepository) GetPatternsByText(ctx context.Context, tenantID string, entityType EntityType, text string) ([]MentionPattern, error) {
	query := `
		SELECT id, tenant_id, entity_type, pattern_text, resolved_entity_id,
			resolved_expansion, project_id, is_permanent, times_seen,
			times_linked, last_seen_at, last_linked_at, first_content_id, created_at
		FROM mention_patterns
		WHERE tenant_id = $1 AND entity_type = $2 AND LOWER(pattern_text) = LOWER($3)
		ORDER BY times_linked DESC, project_id NULLS LAST
	`

	rows, err := r.db.Query(ctx, query, tenantID, string(entityType), text)
	if err != nil {
		return nil, fmt.Errorf("getting patterns by text: %w", err)
	}
	defer rows.Close()

	var patterns []MentionPattern
	for rows.Next() {
		p, err := scanPatternRow(rows)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, *p)
	}

	return patterns, nil
}

// ListPatterns lists all patterns with optional filters.
func (r *PostgresRepository) ListPatterns(ctx context.Context, filter PatternFilter) ([]MentionPattern, error) {
	query := `
		SELECT id, tenant_id, entity_type, pattern_text, resolved_entity_id,
			resolved_expansion, project_id, is_permanent, times_seen,
			times_linked, last_seen_at, last_linked_at, first_content_id, created_at
		FROM mention_patterns
		WHERE tenant_id = $1
	`
	args := []interface{}{filter.TenantID}
	argNum := 2

	if filter.EntityType != nil {
		query += fmt.Sprintf(" AND entity_type = $%d", argNum)
		args = append(args, string(*filter.EntityType))
		argNum++
	}

	if filter.ProjectID != nil {
		query += fmt.Sprintf(" AND project_id = $%d", argNum)
		args = append(args, *filter.ProjectID)
		argNum++
	}

	query += " ORDER BY times_linked DESC, times_seen DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing patterns: %w", err)
	}
	defer rows.Close()

	var patterns []MentionPattern
	for rows.Next() {
		p, err := scanPatternRow(rows)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, *p)
	}

	return patterns, nil
}

// CreateOrUpdatePattern creates or updates a pattern.
func (r *PostgresRepository) CreateOrUpdatePattern(ctx context.Context, pattern *MentionPattern) error {
	query := `
		INSERT INTO mention_patterns (
			tenant_id, entity_type, pattern_text, resolved_entity_id,
			resolved_expansion, project_id, is_permanent, times_seen,
			times_linked, last_seen_at, first_content_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (tenant_id, entity_type, pattern_text, project_id)
		DO UPDATE SET
			resolved_entity_id = EXCLUDED.resolved_entity_id,
			resolved_expansion = EXCLUDED.resolved_expansion,
			is_permanent = EXCLUDED.is_permanent,
			times_seen = mention_patterns.times_seen + 1,
			last_seen_at = NOW()
		RETURNING id
	`

	err := r.db.QueryRow(ctx, query,
		pattern.TenantID,
		string(pattern.EntityType),
		pattern.PatternText,
		pattern.ResolvedEntityID,
		nullableString(pattern.ResolvedExpansion),
		pattern.ProjectID,
		pattern.IsPermanent,
		pattern.TimesSeen,
		pattern.TimesLinked,
		pattern.LastSeenAt,
		pattern.FirstContentID,
	).Scan(&pattern.ID)

	if err != nil {
		return fmt.Errorf("creating/updating pattern: %w", err)
	}

	return nil
}

// IncrementPatternSeen increments the times_seen counter.
func (r *PostgresRepository) IncrementPatternSeen(ctx context.Context, id int64) error {
	query := `
		UPDATE mention_patterns
		SET times_seen = times_seen + 1, last_seen_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("incrementing pattern seen: %w", err)
	}

	return nil
}

// IncrementPatternLinked increments the times_linked counter and updates entity.
func (r *PostgresRepository) IncrementPatternLinked(ctx context.Context, id int64, entityID int64) error {
	query := `
		UPDATE mention_patterns
		SET times_linked = times_linked + 1,
			last_linked_at = NOW(),
			resolved_entity_id = $2
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, id, entityID)
	if err != nil {
		return fmt.Errorf("incrementing pattern linked: %w", err)
	}

	return nil
}

// GetAffinity retrieves an affinity record.
func (r *PostgresRepository) GetAffinity(ctx context.Context, tenantID string, entityType EntityType, entityID, projectID int64) (*EntityProjectAffinity, error) {
	query := `
		SELECT id, tenant_id, entity_type, entity_id, project_id,
			mention_count, last_mentioned_at, is_member, role,
			affinity_score, created_at, updated_at
		FROM entity_project_affinity
		WHERE tenant_id = $1 AND entity_type = $2 AND entity_id = $3 AND project_id = $4
	`

	row := r.db.QueryRow(ctx, query, tenantID, string(entityType), entityID, projectID)
	return scanAffinity(row)
}

// GetAffinitiesForProject retrieves all affinities for a project.
func (r *PostgresRepository) GetAffinitiesForProject(ctx context.Context, tenantID string, projectID int64, entityType EntityType) ([]EntityProjectAffinity, error) {
	query := `
		SELECT id, tenant_id, entity_type, entity_id, project_id,
			mention_count, last_mentioned_at, is_member, role,
			affinity_score, created_at, updated_at
		FROM entity_project_affinity
		WHERE tenant_id = $1 AND project_id = $2 AND entity_type = $3
		ORDER BY affinity_score DESC
	`

	rows, err := r.db.Query(ctx, query, tenantID, projectID, string(entityType))
	if err != nil {
		return nil, fmt.Errorf("getting affinities for project: %w", err)
	}
	defer rows.Close()

	var affinities []EntityProjectAffinity
	for rows.Next() {
		a, err := scanAffinityRow(rows)
		if err != nil {
			return nil, err
		}
		affinities = append(affinities, *a)
	}

	return affinities, nil
}

// GetAffinitiesForEntity retrieves all affinities for an entity.
func (r *PostgresRepository) GetAffinitiesForEntity(ctx context.Context, tenantID string, entityType EntityType, entityID int64) ([]EntityProjectAffinity, error) {
	query := `
		SELECT id, tenant_id, entity_type, entity_id, project_id,
			mention_count, last_mentioned_at, is_member, role,
			affinity_score, created_at, updated_at
		FROM entity_project_affinity
		WHERE tenant_id = $1 AND entity_type = $2 AND entity_id = $3
		ORDER BY affinity_score DESC
	`

	rows, err := r.db.Query(ctx, query, tenantID, string(entityType), entityID)
	if err != nil {
		return nil, fmt.Errorf("getting affinities for entity: %w", err)
	}
	defer rows.Close()

	var affinities []EntityProjectAffinity
	for rows.Next() {
		a, err := scanAffinityRow(rows)
		if err != nil {
			return nil, err
		}
		affinities = append(affinities, *a)
	}

	return affinities, nil
}

// UpsertAffinity creates or updates an affinity record.
func (r *PostgresRepository) UpsertAffinity(ctx context.Context, affinity *EntityProjectAffinity) error {
	query := `
		INSERT INTO entity_project_affinity (
			tenant_id, entity_type, entity_id, project_id,
			mention_count, last_mentioned_at, is_member, role, affinity_score
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, entity_type, entity_id, project_id)
		DO UPDATE SET
			mention_count = EXCLUDED.mention_count,
			last_mentioned_at = EXCLUDED.last_mentioned_at,
			is_member = EXCLUDED.is_member,
			role = EXCLUDED.role,
			affinity_score = EXCLUDED.affinity_score,
			updated_at = NOW()
		RETURNING id
	`

	err := r.db.QueryRow(ctx, query,
		affinity.TenantID,
		string(affinity.EntityType),
		affinity.EntityID,
		affinity.ProjectID,
		affinity.MentionCount,
		affinity.LastMentionedAt,
		affinity.IsMember,
		nullableString(affinity.Role),
		affinity.AffinityScore,
	).Scan(&affinity.ID)

	if err != nil {
		return fmt.Errorf("upserting affinity: %w", err)
	}

	return nil
}

// IncrementAffinityMentionCount increments the mention count and updates score.
func (r *PostgresRepository) IncrementAffinityMentionCount(ctx context.Context, tenantID string, entityType EntityType, entityID, projectID int64) error {
	// First try to update existing
	query := `
		UPDATE entity_project_affinity
		SET mention_count = mention_count + 1,
			last_mentioned_at = NOW(),
			affinity_score = LEAST(1.0, affinity_score + 0.01)
		WHERE tenant_id = $1 AND entity_type = $2 AND entity_id = $3 AND project_id = $4
	`

	result, err := r.db.Exec(ctx, query, tenantID, string(entityType), entityID, projectID)
	if err != nil {
		return fmt.Errorf("incrementing affinity mention count: %w", err)
	}

	if result.RowsAffected() == 0 {
		// Create new record
		insertQuery := `
			INSERT INTO entity_project_affinity (
				tenant_id, entity_type, entity_id, project_id,
				mention_count, last_mentioned_at, affinity_score
			) VALUES ($1, $2, $3, $4, 1, NOW(), 0.5)
		`
		_, err = r.db.Exec(ctx, insertQuery, tenantID, string(entityType), entityID, projectID)
		if err != nil {
			return fmt.Errorf("creating affinity record: %w", err)
		}
	}

	return nil
}

// GetMentionStats returns statistics about pending mentions.
func (r *PostgresRepository) GetMentionStats(ctx context.Context, tenantID string) (*MentionStats, error) {
	stats := &MentionStats{
		ByType: make(map[string]int),
	}

	// Total pending
	query := `SELECT COUNT(*) FROM content_mentions WHERE tenant_id = $1 AND status = 'pending'`
	err := r.db.QueryRow(ctx, query, tenantID).Scan(&stats.TotalPending)
	if err != nil {
		return nil, fmt.Errorf("getting total pending: %w", err)
	}

	// By type
	typeQuery := `
		SELECT entity_type, COUNT(*) as count
		FROM content_mentions
		WHERE tenant_id = $1 AND status = 'pending'
		GROUP BY entity_type
	`
	rows, err := r.db.Query(ctx, typeQuery, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting counts by type: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entityType string
		var count int
		if err := rows.Scan(&entityType, &count); err != nil {
			return nil, err
		}
		stats.ByType[entityType] = count
	}

	// Resolved today
	todayQuery := `
		SELECT COUNT(*) FROM content_mentions
		WHERE tenant_id = $1 AND resolved_at >= CURRENT_DATE
	`
	err = r.db.QueryRow(ctx, todayQuery, tenantID).Scan(&stats.ResolvedToday)
	if err != nil {
		return nil, fmt.Errorf("getting resolved today: %w", err)
	}

	return stats, nil
}

// GetPendingCount returns the count of pending mentions.
func (r *PostgresRepository) GetPendingCount(ctx context.Context, tenantID string, entityType *EntityType) (int, error) {
	query := `SELECT COUNT(*) FROM content_mentions WHERE tenant_id = $1 AND status = 'pending'`
	args := []interface{}{tenantID}

	if entityType != nil {
		query += " AND entity_type = $2"
		args = append(args, string(*entityType))
	}

	var count int
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("getting pending count: %w", err)
	}

	return count, nil
}

// BatchCreateMentions creates multiple mentions using PostgreSQL's COPY protocol
// for optimal bulk insert performance (single round-trip instead of N round-trips).
func (r *PostgresRepository) BatchCreateMentions(ctx context.Context, inputs []MentionInput) ([]ContentMention, error) {
	if len(inputs) == 0 {
		return []ContentMention{}, nil
	}

	tenantID := getTenantFromContext(ctx)
	now := time.Now()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Pre-allocate IDs from the sequence in a single query
	// This allows us to use CopyFrom while still knowing the IDs
	var startID int64
	err = tx.QueryRow(ctx,
		"SELECT setval('content_mentions_id_seq', nextval('content_mentions_id_seq') + $1 - 1)",
		len(inputs),
	).Scan(&startID)
	if err != nil {
		return nil, fmt.Errorf("allocating IDs: %w", err)
	}
	// startID is now the last ID in our allocated range
	// So the first ID we use is startID - len(inputs) + 1
	firstID := startID - int64(len(inputs)) + 1

	// Use CopyFrom for bulk insert (single round-trip)
	columns := []string{
		"id", "tenant_id", "content_id", "entity_type", "mentioned_text",
		"position", "context_snippet", "project_context_id", "status", "candidates", "created_at",
	}

	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"content_mentions"},
		columns,
		pgx.CopyFromSlice(len(inputs), func(i int) ([]any, error) {
			input := inputs[i]
			return []any{
				firstID + int64(i),           // id
				tenantID,                     // tenant_id
				input.ContentID,              // content_id
				string(input.EntityType),     // entity_type
				input.MentionedText,          // mentioned_text
				input.Position,               // position (can be nil)
				nullableString(input.ContextSnippet), // context_snippet
				input.ProjectContextID,       // project_context_id (can be nil)
				string(MentionStatusPending), // status
				[]byte("[]"),                 // candidates (empty JSONB array)
				now,                          // created_at
			}, nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("bulk inserting mentions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	// Build result slice with known IDs
	mentions := make([]ContentMention, len(inputs))
	for i, input := range inputs {
		mentions[i] = ContentMention{
			ID:               firstID + int64(i),
			TenantID:         tenantID,
			ContentID:        input.ContentID,
			EntityType:       input.EntityType,
			MentionedText:    input.MentionedText,
			Position:         input.Position,
			ContextSnippet:   input.ContextSnippet,
			ProjectContextID: input.ProjectContextID,
			Status:           MentionStatusPending,
			Candidates:       []Candidate{},
			CreatedAt:        now,
		}
	}

	return mentions, nil
}

// BatchResolveMentions resolves multiple mentions in a single transaction.
func (r *PostgresRepository) BatchResolveMentions(ctx context.Context, resolutions []ResolutionInput) (*BatchResolutionResult, error) {
	result := &BatchResolutionResult{}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, res := range resolutions {
		query := `
			UPDATE content_mentions
			SET resolved_entity_id = $2,
				resolution_source = $3,
				resolution_confidence = 1.0,
				status = 'user_resolved',
				resolved_at = NOW(),
				resolved_by = $4
			WHERE id = $1
		`

		_, err := tx.Exec(ctx, query,
			res.MentionID,
			res.EntityID,
			string(res.Source),
			nullableString(res.ResolvedBy),
		)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("resolve %d: %v", res.MentionID, err))
		} else {
			result.Resolved++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return result, nil
}

// PatternFilter specifies criteria for listing patterns.
type PatternFilter struct {
	TenantID   string      `json:"tenant_id"`
	EntityType *EntityType `json:"entity_type,omitempty"`
	ProjectID  *int64      `json:"project_id,omitempty"`
	Search     string      `json:"search,omitempty"`
	Limit      int         `json:"limit,omitempty"`
	Offset     int         `json:"offset,omitempty"`
}

// Helper functions

func scanMention(row pgx.Row) (*ContentMention, error) {
	var m ContentMention
	var position *int
	var contextSnippet, resolutionSource, resolvedExpansion, resolvedBy *string
	var resolvedEntityID, projectContextID *int64
	var resolutionConfidence *float32
	var resolvedAt *time.Time
	var candidatesJSON []byte
	var entityType, status string

	err := row.Scan(
		&m.ID,
		&m.TenantID,
		&m.ContentID,
		&entityType,
		&m.MentionedText,
		&position,
		&contextSnippet,
		&resolvedEntityID,
		&resolutionConfidence,
		&resolutionSource,
		&resolvedExpansion,
		&candidatesJSON,
		&status,
		&resolvedAt,
		&resolvedBy,
		&projectContextID,
		&m.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning mention: %w", err)
	}

	m.EntityType = EntityType(entityType)
	m.Status = MentionStatus(status)
	m.Position = position
	m.ResolvedEntityID = resolvedEntityID
	m.ResolutionConfidence = resolutionConfidence
	m.ProjectContextID = projectContextID
	m.ResolvedAt = resolvedAt

	if contextSnippet != nil {
		m.ContextSnippet = *contextSnippet
	}
	if resolutionSource != nil {
		m.ResolutionSource = ResolutionSource(*resolutionSource)
	}
	if resolvedExpansion != nil {
		m.ResolvedExpansion = *resolvedExpansion
	}
	if resolvedBy != nil {
		m.ResolvedBy = *resolvedBy
	}

	if len(candidatesJSON) > 0 {
		if err := json.Unmarshal(candidatesJSON, &m.Candidates); err != nil {
			return nil, fmt.Errorf("parsing candidates: %w", err)
		}
	}

	return &m, nil
}

func scanMentionRow(rows pgx.Rows) (*ContentMention, error) {
	var m ContentMention
	var position *int
	var contextSnippet, resolutionSource, resolvedExpansion, resolvedBy *string
	var resolvedEntityID, projectContextID *int64
	var resolutionConfidence *float32
	var resolvedAt *time.Time
	var candidatesJSON []byte
	var entityType, status string

	err := rows.Scan(
		&m.ID,
		&m.TenantID,
		&m.ContentID,
		&entityType,
		&m.MentionedText,
		&position,
		&contextSnippet,
		&resolvedEntityID,
		&resolutionConfidence,
		&resolutionSource,
		&resolvedExpansion,
		&candidatesJSON,
		&status,
		&resolvedAt,
		&resolvedBy,
		&projectContextID,
		&m.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning mention row: %w", err)
	}

	m.EntityType = EntityType(entityType)
	m.Status = MentionStatus(status)
	m.Position = position
	m.ResolvedEntityID = resolvedEntityID
	m.ResolutionConfidence = resolutionConfidence
	m.ProjectContextID = projectContextID
	m.ResolvedAt = resolvedAt

	if contextSnippet != nil {
		m.ContextSnippet = *contextSnippet
	}
	if resolutionSource != nil {
		m.ResolutionSource = ResolutionSource(*resolutionSource)
	}
	if resolvedExpansion != nil {
		m.ResolvedExpansion = *resolvedExpansion
	}
	if resolvedBy != nil {
		m.ResolvedBy = *resolvedBy
	}

	if len(candidatesJSON) > 0 {
		if err := json.Unmarshal(candidatesJSON, &m.Candidates); err != nil {
			return nil, fmt.Errorf("parsing candidates: %w", err)
		}
	}

	return &m, nil
}

func scanPattern(row pgx.Row) (*MentionPattern, error) {
	var p MentionPattern
	var resolvedExpansion *string
	var resolvedEntityID, projectID, firstContentID *int64
	var lastLinkedAt *time.Time
	var entityType string

	err := row.Scan(
		&p.ID,
		&p.TenantID,
		&entityType,
		&p.PatternText,
		&resolvedEntityID,
		&resolvedExpansion,
		&projectID,
		&p.IsPermanent,
		&p.TimesSeen,
		&p.TimesLinked,
		&p.LastSeenAt,
		&lastLinkedAt,
		&firstContentID,
		&p.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning pattern: %w", err)
	}

	p.EntityType = EntityType(entityType)
	p.ResolvedEntityID = resolvedEntityID
	p.ProjectID = projectID
	p.FirstContentID = firstContentID
	p.LastLinkedAt = lastLinkedAt

	if resolvedExpansion != nil {
		p.ResolvedExpansion = *resolvedExpansion
	}

	return &p, nil
}

func scanPatternRow(rows pgx.Rows) (*MentionPattern, error) {
	var p MentionPattern
	var resolvedExpansion *string
	var resolvedEntityID, projectID, firstContentID *int64
	var lastLinkedAt *time.Time
	var entityType string

	err := rows.Scan(
		&p.ID,
		&p.TenantID,
		&entityType,
		&p.PatternText,
		&resolvedEntityID,
		&resolvedExpansion,
		&projectID,
		&p.IsPermanent,
		&p.TimesSeen,
		&p.TimesLinked,
		&p.LastSeenAt,
		&lastLinkedAt,
		&firstContentID,
		&p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning pattern row: %w", err)
	}

	p.EntityType = EntityType(entityType)
	p.ResolvedEntityID = resolvedEntityID
	p.ProjectID = projectID
	p.FirstContentID = firstContentID
	p.LastLinkedAt = lastLinkedAt

	if resolvedExpansion != nil {
		p.ResolvedExpansion = *resolvedExpansion
	}

	return &p, nil
}

func scanAffinity(row pgx.Row) (*EntityProjectAffinity, error) {
	var a EntityProjectAffinity
	var lastMentionedAt *time.Time
	var role *string
	var entityType string

	err := row.Scan(
		&a.ID,
		&a.TenantID,
		&entityType,
		&a.EntityID,
		&a.ProjectID,
		&a.MentionCount,
		&lastMentionedAt,
		&a.IsMember,
		&role,
		&a.AffinityScore,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning affinity: %w", err)
	}

	a.EntityType = EntityType(entityType)
	a.LastMentionedAt = lastMentionedAt
	if role != nil {
		a.Role = *role
	}

	return &a, nil
}

func scanAffinityRow(rows pgx.Rows) (*EntityProjectAffinity, error) {
	var a EntityProjectAffinity
	var lastMentionedAt *time.Time
	var role *string
	var entityType string

	err := rows.Scan(
		&a.ID,
		&a.TenantID,
		&entityType,
		&a.EntityID,
		&a.ProjectID,
		&a.MentionCount,
		&lastMentionedAt,
		&a.IsMember,
		&role,
		&a.AffinityScore,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning affinity row: %w", err)
	}

	a.EntityType = EntityType(entityType)
	a.LastMentionedAt = lastMentionedAt
	if role != nil {
		a.Role = *role
	}

	return &a, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func getTenantFromContext(ctx context.Context) string {
	// STUB: Returns hardcoded tenant until multi-tenant context propagation is implemented.
	return "00000001-0000-0000-0000-000000000001"
}
