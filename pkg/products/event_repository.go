package products

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

// ==================== ProductEvent Operations ====================

// CreateEvent creates a new product event.
func (r *Repository) CreateEvent(ctx context.Context, e *ProductEvent) error {
	if e.EventUUID == uuid.Nil {
		e.EventUUID = uuid.New()
	}

	metadataJSON, err := json.Marshal(e.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO product_events (
			event_uuid, tenant_id, product_id,
			event_type, visibility, source_type,
			title, description, occurred_at, recorded_by,
			metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6,
			$7, $8, $9, $10,
			$11, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at
	`

	err = r.pool.QueryRow(ctx, query,
		e.EventUUID,
		e.TenantID,
		e.ProductID,
		e.EventType,
		e.Visibility,
		e.SourceType,
		e.Title,
		nullIfEmpty(e.Description),
		e.OccurredAt,
		nullIfEmpty(e.RecordedBy),
		metadataJSON,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	r.logger.Debug("Event created",
		logging.F("id", e.ID),
		logging.F("type", string(e.EventType)),
		logging.F("title", e.Title),
	)

	return nil
}

// GetEvent retrieves an event by ID.
func (r *Repository) GetEvent(ctx context.Context, id int64) (*ProductEvent, error) {
	query := `
		SELECT
			e.id, e.event_uuid, e.tenant_id, e.product_id,
			e.event_type, e.visibility, e.source_type,
			e.title, e.description, e.occurred_at, e.recorded_by,
			e.metadata, e.created_at, e.updated_at,
			p.name as product_name
		FROM product_events e
		JOIN products p ON e.product_id = p.id
		WHERE e.id = $1
	`
	return r.scanEvent(ctx, query, id)
}

// GetEventByUUID retrieves an event by UUID.
func (r *Repository) GetEventByUUID(ctx context.Context, eventUUID uuid.UUID) (*ProductEvent, error) {
	query := `
		SELECT
			e.id, e.event_uuid, e.tenant_id, e.product_id,
			e.event_type, e.visibility, e.source_type,
			e.title, e.description, e.occurred_at, e.recorded_by,
			e.metadata, e.created_at, e.updated_at,
			p.name as product_name
		FROM product_events e
		JOIN products p ON e.product_id = p.id
		WHERE e.event_uuid = $1
	`
	return r.scanEvent(ctx, query, eventUUID)
}

// UpdateEvent updates an existing event.
func (r *Repository) UpdateEvent(ctx context.Context, e *ProductEvent) error {
	metadataJSON, err := json.Marshal(e.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE product_events SET
			event_type = $2,
			visibility = $3,
			source_type = $4,
			title = $5,
			description = $6,
			occurred_at = $7,
			recorded_by = $8,
			metadata = $9,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err = r.pool.QueryRow(ctx, query,
		e.ID,
		e.EventType,
		e.Visibility,
		e.SourceType,
		e.Title,
		nullIfEmpty(e.Description),
		e.OccurredAt,
		nullIfEmpty(e.RecordedBy),
		metadataJSON,
	).Scan(&e.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to update event: %w", err)
	}

	return nil
}

// DeleteEvent deletes an event by ID.
func (r *Repository) DeleteEvent(ctx context.Context, id int64) error {
	result, err := r.pool.Exec(ctx, "DELETE FROM product_events WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ==================== Event Query Operations ====================

// ListEvents lists events with optional filtering.
func (r *Repository) ListEvents(ctx context.Context, filter EventFilter) ([]*ProductEvent, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("e.tenant_id = $%d", argIdx))
	args = append(args, filter.TenantID)
	argIdx++

	if filter.ProductID != nil {
		conditions = append(conditions, fmt.Sprintf("e.product_id = $%d", argIdx))
		args = append(args, *filter.ProductID)
		argIdx++
	}

	if len(filter.EventTypes) > 0 {
		placeholders := make([]string, len(filter.EventTypes))
		for i, et := range filter.EventTypes {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, et)
			argIdx++
		}
		conditions = append(conditions, fmt.Sprintf("e.event_type IN (%s)", strings.Join(placeholders, ", ")))
	}

	if filter.Visibility != nil {
		conditions = append(conditions, fmt.Sprintf("e.visibility = $%d", argIdx))
		args = append(args, *filter.Visibility)
		argIdx++
	}

	if filter.Since != nil {
		conditions = append(conditions, fmt.Sprintf("e.occurred_at >= $%d", argIdx))
		args = append(args, *filter.Since)
		argIdx++
	}

	if filter.Until != nil {
		conditions = append(conditions, fmt.Sprintf("e.occurred_at <= $%d", argIdx))
		args = append(args, *filter.Until)
		argIdx++
	}

	query := fmt.Sprintf(`
		SELECT
			e.id, e.event_uuid, e.tenant_id, e.product_id,
			e.event_type, e.visibility, e.source_type,
			e.title, e.description, e.occurred_at, e.recorded_by,
			e.metadata, e.created_at, e.updated_at,
			p.name as product_name
		FROM product_events e
		JOIN products p ON e.product_id = p.id
		WHERE %s
		ORDER BY e.occurred_at DESC
	`, strings.Join(conditions, " AND "))

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}
	defer rows.Close()

	return r.scanEvents(rows)
}

// GetProductTimeline retrieves all events for a product in chronological order.
func (r *Repository) GetProductTimeline(ctx context.Context, productID int64, limit int) ([]*ProductEvent, error) {
	query := `
		SELECT
			e.id, e.event_uuid, e.tenant_id, e.product_id,
			e.event_type, e.visibility, e.source_type,
			e.title, e.description, e.occurred_at, e.recorded_by,
			e.metadata, e.created_at, e.updated_at,
			p.name as product_name
		FROM product_events e
		JOIN products p ON e.product_id = p.id
		WHERE e.product_id = $1
		ORDER BY e.occurred_at DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := r.pool.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get timeline: %w", err)
	}
	defer rows.Close()

	return r.scanEvents(rows)
}

// GetExternalEvents retrieves external events (competitor, market) for a product.
func (r *Repository) GetExternalEvents(ctx context.Context, productID int64, limit int) ([]*ProductEvent, error) {
	query := `
		SELECT
			e.id, e.event_uuid, e.tenant_id, e.product_id,
			e.event_type, e.visibility, e.source_type,
			e.title, e.description, e.occurred_at, e.recorded_by,
			e.metadata, e.created_at, e.updated_at,
			p.name as product_name
		FROM product_events e
		JOIN products p ON e.product_id = p.id
		WHERE e.product_id = $1 AND e.visibility = 'external'
		ORDER BY e.occurred_at DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := r.pool.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get external events: %w", err)
	}
	defer rows.Close()

	return r.scanEvents(rows)
}

// GetContextWindow retrieves events in a time window around a specific point.
// This is used for AI context anchoring - "what was happening when we made decision X".
func (r *Repository) GetContextWindow(ctx context.Context, productID int64, centerTime time.Time, windowDays int) (*ContextWindow, error) {
	windowDuration := time.Duration(windowDays) * 24 * time.Hour
	windowStart := centerTime.Add(-windowDuration)
	windowEnd := centerTime.Add(windowDuration)

	query := `
		SELECT
			e.id, e.event_uuid, e.tenant_id, e.product_id,
			e.event_type, e.visibility, e.source_type,
			e.title, e.description, e.occurred_at, e.recorded_by,
			e.metadata, e.created_at, e.updated_at,
			p.name as product_name
		FROM product_events e
		JOIN products p ON e.product_id = p.id
		WHERE e.product_id = $1
		  AND e.occurred_at BETWEEN $2 AND $3
		ORDER BY e.occurred_at
	`

	rows, err := r.pool.Query(ctx, query, productID, windowStart, windowEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get context window: %w", err)
	}
	defer rows.Close()

	events, err := r.scanEvents(rows)
	if err != nil {
		return nil, err
	}

	// Split into before/after
	window := &ContextWindow{
		CenterTime:   centerTime,
		WindowStart:  windowStart,
		WindowEnd:    windowEnd,
		EventsBefore: make([]*ProductEvent, 0),
		EventsAfter:  make([]*ProductEvent, 0),
	}

	for _, e := range events {
		if e.OccurredAt.Before(centerTime) {
			window.EventsBefore = append(window.EventsBefore, e)
		} else if e.OccurredAt.After(centerTime) {
			window.EventsAfter = append(window.EventsAfter, e)
		} else {
			window.CenterEvent = e
		}
	}

	return window, nil
}

// ==================== Event Link Operations ====================

// LinkEventToSource links an event to a source entity (meeting, email, etc.).
func (r *Repository) LinkEventToSource(ctx context.Context, link *ProductEventLink) error {
	query := `
		INSERT INTO product_event_links (
			event_id, linked_entity_type, linked_entity_id, link_type, created_at
		) VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, created_at
	`

	err := r.pool.QueryRow(ctx, query,
		link.EventID,
		link.LinkedEntityType,
		link.LinkedEntityID,
		link.LinkType,
	).Scan(&link.ID, &link.CreatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "product_event_links_unique") {
			return errors.New("link already exists")
		}
		return fmt.Errorf("failed to link event: %w", err)
	}

	return nil
}

// UnlinkEventFromSource removes a link from an event.
func (r *Repository) UnlinkEventFromSource(ctx context.Context, eventID int64, entityType string, entityID int64) error {
	result, err := r.pool.Exec(ctx,
		"DELETE FROM product_event_links WHERE event_id = $1 AND linked_entity_type = $2 AND linked_entity_id = $3",
		eventID, entityType, entityID,
	)
	if err != nil {
		return fmt.Errorf("failed to unlink event: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetEventLinks retrieves all links for an event.
func (r *Repository) GetEventLinks(ctx context.Context, eventID int64) ([]*ProductEventLink, error) {
	query := `
		SELECT id, event_id, linked_entity_type, linked_entity_id, link_type, created_at
		FROM product_event_links
		WHERE event_id = $1
		ORDER BY link_type, linked_entity_type
	`

	rows, err := r.pool.Query(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get event links: %w", err)
	}
	defer rows.Close()

	var links []*ProductEventLink
	for rows.Next() {
		link := &ProductEventLink{}
		err := rows.Scan(
			&link.ID, &link.EventID, &link.LinkedEntityType,
			&link.LinkedEntityID, &link.LinkType, &link.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan link: %w", err)
		}
		links = append(links, link)
	}

	return links, rows.Err()
}

// GetEventsLinkedToSource retrieves all events linked to a specific source.
func (r *Repository) GetEventsLinkedToSource(ctx context.Context, entityType string, entityID int64) ([]*ProductEvent, error) {
	query := `
		SELECT
			e.id, e.event_uuid, e.tenant_id, e.product_id,
			e.event_type, e.visibility, e.source_type,
			e.title, e.description, e.occurred_at, e.recorded_by,
			e.metadata, e.created_at, e.updated_at,
			p.name as product_name
		FROM product_events e
		JOIN products p ON e.product_id = p.id
		JOIN product_event_links l ON e.id = l.event_id
		WHERE l.linked_entity_type = $1 AND l.linked_entity_id = $2
		ORDER BY e.occurred_at DESC
	`

	rows, err := r.pool.Query(ctx, query, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get linked events: %w", err)
	}
	defer rows.Close()

	return r.scanEvents(rows)
}

// ==================== Helper Functions ====================

func (r *Repository) scanEvent(ctx context.Context, query string, args ...any) (*ProductEvent, error) {
	e := &ProductEvent{}
	var metadataJSON []byte
	var description, recordedBy *string

	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&e.ID, &e.EventUUID, &e.TenantID, &e.ProductID,
		&e.EventType, &e.Visibility, &e.SourceType,
		&e.Title, &description, &e.OccurredAt, &recordedBy,
		&metadataJSON, &e.CreatedAt, &e.UpdatedAt,
		&e.ProductName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to scan event: %w", err)
	}

	if description != nil {
		e.Description = *description
	}
	if recordedBy != nil {
		e.RecordedBy = *recordedBy
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &e.Metadata); err != nil {
			r.logger.Warn("Failed to unmarshal event metadata",
				logging.Err(err),
				logging.F("event_id", e.ID),
			)
		}
	}

	return e, nil
}

func (r *Repository) scanEvents(rows pgx.Rows) ([]*ProductEvent, error) {
	var events []*ProductEvent
	for rows.Next() {
		e := &ProductEvent{}
		var metadataJSON []byte
		var description, recordedBy *string

		err := rows.Scan(
			&e.ID, &e.EventUUID, &e.TenantID, &e.ProductID,
			&e.EventType, &e.Visibility, &e.SourceType,
			&e.Title, &description, &e.OccurredAt, &recordedBy,
			&metadataJSON, &e.CreatedAt, &e.UpdatedAt,
			&e.ProductName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if description != nil {
			e.Description = *description
		}
		if recordedBy != nil {
			e.RecordedBy = *recordedBy
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &e.Metadata); err != nil {
				r.logger.Warn("Failed to unmarshal event metadata",
					logging.Err(err),
					logging.F("event_id", e.ID),
				)
			}
		}

		events = append(events, e)
	}
	return events, rows.Err()
}
