// Package contextpalace provides a client for Context-Palace database operations.
package contextpalace

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/lib/pq"

	"github.com/otherjamesbrown/penf-cli/config"
)

// Client provides Context-Palace database operations.
type Client struct {
	db      *sql.DB
	config  *config.ContextPalaceConfig
	project string
	agent   string
}

// CommandEntry represents a CLI command log entry.
type CommandEntry struct {
	ID           int64     `json:"id"`
	Project      string    `json:"project"`
	Agent        string    `json:"agent"`
	Command      string    `json:"command"`
	Args         []string  `json:"args"`
	FullCommand  string    `json:"full_command"`
	DurationMs   int       `json:"duration_ms"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
	Response     string    `json:"response,omitempty"`
	TenantID     string    `json:"tenant_id,omitempty"`
	Hostname     string    `json:"hostname,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// NewClient creates a new Context-Palace client from configuration.
func NewClient(cfg *config.ContextPalaceConfig) (*Client, error) {
	if cfg == nil || !cfg.IsConfigured() {
		return nil, fmt.Errorf("context-palace not configured")
	}

	connStr := cfg.ConnectionString()
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Configure connection pool.
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &Client{
		db:      db,
		config:  cfg,
		project: cfg.GetProject(),
		agent:   cfg.GetAgent(),
	}, nil
}

// Close closes the database connection.
func (c *Client) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// Ping checks the database connection.
func (c *Client) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// LogCommand logs a CLI command execution to Context-Palace.
func (c *Client) LogCommand(ctx context.Context, entry *CommandEntry) error {
	// Use provided values or defaults from config.
	project := entry.Project
	if project == "" {
		project = c.project
	}
	agent := entry.Agent
	if agent == "" {
		agent = c.agent
	}

	// Get hostname if not provided.
	hostname := entry.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	// Truncate response and error message to 500 chars (server does this too, but be safe).
	response := truncate(entry.Response, 500)
	errorMsg := truncate(entry.ErrorMessage, 500)

	query := `SELECT log_cli_command($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err := c.db.ExecContext(ctx, query,
		project,
		agent,
		entry.Command,
		pq.Array(entry.Args),
		entry.FullCommand,
		entry.DurationMs,
		entry.Success,
		nullIfEmpty(errorMsg),
		nullIfEmpty(response),
		nullIfEmpty(entry.TenantID),
		nullIfEmpty(hostname),
	)
	if err != nil {
		return fmt.Errorf("logging command: %w", err)
	}

	return nil
}

// History retrieves recent CLI command history from Context-Palace.
func (c *Client) History(ctx context.Context, agent string, limit int) ([]CommandEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `SELECT * FROM cli_history($1, $2, $3)`

	rows, err := c.db.QueryContext(ctx, query, c.project, nullIfEmpty(agent), limit)
	if err != nil {
		return nil, fmt.Errorf("querying history: %w", err)
	}
	defer rows.Close()

	var entries []CommandEntry
	for rows.Next() {
		var e CommandEntry
		var errorMsg sql.NullString

		// cli_history returns: id, agent, command, full_command, duration_ms, success, error_message, created_at
		err := rows.Scan(
			&e.ID,
			&e.Agent,
			&e.Command,
			&e.FullCommand,
			&e.DurationMs,
			&e.Success,
			&errorMsg,
			&e.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		e.Project = c.project
		e.ErrorMessage = errorMsg.String

		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return entries, nil
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// nullIfEmpty returns nil if s is empty, otherwise returns s.
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// Shard represents a Context-Palace shard.
type Shard struct {
	ID         string     `json:"id"`
	Project    string     `json:"project"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	Type       string     `json:"type"`         // memory, session, backlog, task, etc.
	Status     string     `json:"status"`       // open, in_progress, closed
	Owner      string     `json:"owner"`
	Priority   int        `json:"priority"`     // 0=critical, 1=high, 2=normal, 3=low, 4=backlog
	Creator    string     `json:"creator"`
	ParentID   string     `json:"parent_id,omitempty"`
	Labels     []string   `json:"labels,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ClosedAt   *time.Time `json:"closed_at,omitempty"`
	ClosedBy   string     `json:"closed_by,omitempty"`
	Resolution string     `json:"resolution,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// ShardOption is a functional option for creating shards.
type ShardOption func(*shardOptions)

type shardOptions struct {
	owner    string
	priority int
	labels   []string
}

// WithOwner sets the owner of the shard.
func WithOwner(owner string) ShardOption {
	return func(opts *shardOptions) {
		opts.owner = owner
	}
}

// WithPriority sets the priority of the shard (0-4).
func WithPriority(priority int) ShardOption {
	return func(opts *shardOptions) {
		opts.priority = priority
	}
}

// WithLabels sets the labels for the shard.
func WithLabels(labels ...string) ShardOption {
	return func(opts *shardOptions) {
		opts.labels = labels
	}
}

// CreateShard creates a new shard in Context-Palace.
func (c *Client) CreateShard(ctx context.Context, shardType, title, content string, opts ...ShardOption) (*Shard, error) {
	options := &shardOptions{
		priority: 2, // Default to normal priority
	}
	for _, opt := range opts {
		opt(options)
	}

	query := `SELECT create_shard($1, $2, $3, $4, $5, $6, $7)`
	var shardID string
	err := c.db.QueryRowContext(ctx, query,
		c.project,
		title,
		content,
		shardType,
		c.agent, // creator
		nullIfEmpty(options.owner),
		options.priority,
	).Scan(&shardID)
	if err != nil {
		return nil, fmt.Errorf("creating shard: %w", err)
	}

	// Add labels if provided
	if len(options.labels) > 0 {
		labelQuery := `SELECT add_labels($1, $2)`
		_, err = c.db.ExecContext(ctx, labelQuery, shardID, pq.Array(options.labels))
		if err != nil {
			return nil, fmt.Errorf("adding labels: %w", err)
		}
	}

	// Fetch the created shard
	return c.GetShard(ctx, shardID)
}

// GetShard retrieves a shard by ID.
func (c *Client) GetShard(ctx context.Context, id string) (*Shard, error) {
	query := `
		SELECT s.id, s.project, s.title, s.content, s.type, s.status, s.owner,
		       s.priority, s.creator, s.parent_id, s.created_at, s.updated_at,
		       s.closed_at, s.closed_reason, s.expires_at,
		       COALESCE(array_agg(l.label) FILTER (WHERE l.label IS NOT NULL), '{}') as labels
		FROM shards s
		LEFT JOIN labels l ON s.id = l.shard_id
		WHERE s.id = $1
		GROUP BY s.id
	`

	var shard Shard
	var content, owner, parentID, resolution sql.NullString
	var closedAt, expiresAt sql.NullTime
	var priority sql.NullInt64

	err := c.db.QueryRowContext(ctx, query, id).Scan(
		&shard.ID,
		&shard.Project,
		&shard.Title,
		&content,
		&shard.Type,
		&shard.Status,
		&owner,
		&priority,
		&shard.Creator,
		&parentID,
		&shard.CreatedAt,
		&shard.UpdatedAt,
		&closedAt,
		&resolution,
		&expiresAt,
		pq.Array(&shard.Labels),
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("shard not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying shard: %w", err)
	}

	shard.Content = content.String
	shard.Owner = owner.String
	shard.ParentID = parentID.String
	shard.Resolution = resolution.String
	if priority.Valid {
		shard.Priority = int(priority.Int64)
	}
	if closedAt.Valid {
		shard.ClosedAt = &closedAt.Time
	}
	if expiresAt.Valid {
		shard.ExpiresAt = &expiresAt.Time
	}

	return &shard, nil
}

// ListShardsOptions contains filters for listing shards.
type ListShardsOptions struct {
	Type     string
	Status   string
	Owner    string
	Creator  string
	Labels   []string
	ParentID string
	Limit    int
}

// ListShards retrieves shards matching the filter criteria.
func (c *Client) ListShards(ctx context.Context, opts ListShardsOptions) ([]Shard, error) {
	query := `
		SELECT s.id, s.project, s.title, s.content, s.type, s.status, s.owner,
		       s.priority, s.creator, s.parent_id, s.created_at, s.updated_at,
		       s.closed_at, s.closed_reason, s.expires_at,
		       COALESCE(array_agg(l.label) FILTER (WHERE l.label IS NOT NULL), '{}') as labels
		FROM shards s
		LEFT JOIN labels l ON s.id = l.shard_id
		WHERE s.project = $1
	`

	args := []interface{}{c.project}
	argCount := 1

	if opts.Type != "" {
		argCount++
		query += fmt.Sprintf(" AND s.type = $%d", argCount)
		args = append(args, opts.Type)
	}

	if opts.Status != "" {
		argCount++
		query += fmt.Sprintf(" AND s.status = $%d", argCount)
		args = append(args, opts.Status)
	}

	if opts.Owner != "" {
		argCount++
		query += fmt.Sprintf(" AND s.owner = $%d", argCount)
		args = append(args, opts.Owner)
	}

	if opts.Creator != "" {
		argCount++
		query += fmt.Sprintf(" AND s.creator = $%d", argCount)
		args = append(args, opts.Creator)
	}

	if opts.ParentID != "" {
		argCount++
		query += fmt.Sprintf(" AND s.parent_id = $%d", argCount)
		args = append(args, opts.ParentID)
	}

	query += " GROUP BY s.id"

	// Filter by labels if provided
	if len(opts.Labels) > 0 {
		query += fmt.Sprintf(" HAVING array_agg(l.label) @> $%d", argCount+1)
		args = append(args, pq.Array(opts.Labels))
	}

	query += " ORDER BY s.created_at DESC"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying shards: %w", err)
	}
	defer rows.Close()

	var shards []Shard
	for rows.Next() {
		var shard Shard
		var content, owner, parentID, resolution sql.NullString
		var closedAt, expiresAt sql.NullTime
		var priority sql.NullInt64

		err := rows.Scan(
			&shard.ID,
			&shard.Project,
			&shard.Title,
			&content,
			&shard.Type,
			&shard.Status,
			&owner,
			&priority,
			&shard.Creator,
			&parentID,
			&shard.CreatedAt,
			&shard.UpdatedAt,
			&closedAt,
			&resolution,
			&expiresAt,
			pq.Array(&shard.Labels),
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		shard.Content = content.String
		shard.Owner = owner.String
		shard.ParentID = parentID.String
		shard.Resolution = resolution.String
		if priority.Valid {
			shard.Priority = int(priority.Int64)
		}
		if closedAt.Valid {
			shard.ClosedAt = &closedAt.Time
		}
		if expiresAt.Valid {
			shard.ExpiresAt = &expiresAt.Time
		}

		shards = append(shards, shard)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return shards, nil
}

// UpdateOption is a functional option for updating shards.
type UpdateOption func(*updateOptions)

type updateOptions struct {
	status    *string
	title     *string
	content   *string
	priority  *int
	owner     *string
	expiresAt *time.Time
}

// WithStatus sets the status for update.
func WithStatus(status string) UpdateOption {
	return func(opts *updateOptions) {
		opts.status = &status
	}
}

// WithTitle sets the title for update.
func WithTitle(title string) UpdateOption {
	return func(opts *updateOptions) {
		opts.title = &title
	}
}

// WithContent sets the content for update.
func WithContent(content string) UpdateOption {
	return func(opts *updateOptions) {
		opts.content = &content
	}
}

// WithUpdatePriority sets the priority for update.
func WithUpdatePriority(priority int) UpdateOption {
	return func(opts *updateOptions) {
		opts.priority = &priority
	}
}

// WithUpdateOwner sets the owner for update.
func WithUpdateOwner(owner string) UpdateOption {
	return func(opts *updateOptions) {
		opts.owner = &owner
	}
}

// WithExpiresAt sets the expiration time for update.
func WithExpiresAt(expiresAt time.Time) UpdateOption {
	return func(opts *updateOptions) {
		opts.expiresAt = &expiresAt
	}
}

// UpdateShard updates a shard's fields.
func (c *Client) UpdateShard(ctx context.Context, id string, opts ...UpdateOption) error {
	options := &updateOptions{}
	for _, opt := range opts {
		opt(options)
	}

	updates := []string{}
	args := []interface{}{id}
	argCount := 1

	if options.status != nil {
		argCount++
		updates = append(updates, fmt.Sprintf("status = $%d", argCount))
		args = append(args, *options.status)
	}

	if options.title != nil {
		argCount++
		updates = append(updates, fmt.Sprintf("title = $%d", argCount))
		args = append(args, *options.title)
	}

	if options.content != nil {
		argCount++
		updates = append(updates, fmt.Sprintf("content = $%d", argCount))
		args = append(args, *options.content)
	}

	if options.priority != nil {
		argCount++
		updates = append(updates, fmt.Sprintf("priority = $%d", argCount))
		args = append(args, *options.priority)
	}

	if options.owner != nil {
		argCount++
		updates = append(updates, fmt.Sprintf("owner = $%d", argCount))
		args = append(args, *options.owner)
	}

	if options.expiresAt != nil {
		argCount++
		updates = append(updates, fmt.Sprintf("expires_at = $%d", argCount))
		args = append(args, *options.expiresAt)
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	query := fmt.Sprintf("UPDATE shards SET %s WHERE id = $1",
		fmt.Sprintf("%s", updates[0]))
	for i := 1; i < len(updates); i++ {
		query = fmt.Sprintf("%s, %s", query, updates[i])
	}

	result, err := c.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating shard: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("shard not found: %s", id)
	}

	return nil
}

// CloseShard closes a shard with a resolution.
func (c *Client) CloseShard(ctx context.Context, id, resolution string) error {
	query := `SELECT close_task($1, $2)`
	_, err := c.db.ExecContext(ctx, query, id, resolution)
	if err != nil {
		return fmt.Errorf("closing shard: %w", err)
	}

	return nil
}

// Message represents a Context-Palace message.
type Message struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Creator   string    `json:"creator"`
	Kind      string    `json:"kind,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// InboxSummary represents inbox statistics.
type InboxSummary struct {
	TotalUnread  int64                  `json:"total_unread"`
	ByKind       map[string]int64       `json:"by_kind"`
	UrgentCount  int64                  `json:"urgent_count"`
	OldestUnread *time.Time             `json:"oldest_unread,omitempty"`
}

// SendMessageOptions contains optional parameters for sending messages.
type SendMessageOptions struct {
	CC      []string
	Kind    string
	ReplyTo string
}

// SendMessage sends a message using the send_message() function.
func (c *Client) SendMessage(ctx context.Context, recipients []string, subject, body string, opts *SendMessageOptions) (string, error) {
	if opts == nil {
		opts = &SendMessageOptions{}
	}

	query := `SELECT send_message($1, $2, $3, $4, $5, $6, $7, $8)`

	var messageID string
	err := c.db.QueryRowContext(ctx, query,
		c.project,
		c.agent,
		pq.Array(recipients),
		subject,
		body,
		nullIfEmptyArray(opts.CC),
		nullIfEmpty(opts.Kind),
		nullIfEmpty(opts.ReplyTo),
	).Scan(&messageID)
	if err != nil {
		return "", fmt.Errorf("sending message: %w", err)
	}

	return messageID, nil
}

// GetUnread retrieves unread messages for the current agent.
func (c *Client) GetUnread(ctx context.Context) ([]Message, error) {
	query := `SELECT * FROM unread_for($1, $2)`

	rows, err := c.db.QueryContext(ctx, query, c.project, c.agent)
	if err != nil {
		return nil, fmt.Errorf("querying unread messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var kind sql.NullString
		err := rows.Scan(&m.ID, &m.Title, &m.Creator, &kind, &m.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning message: %w", err)
		}
		m.Kind = kind.String
		messages = append(messages, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating messages: %w", err)
	}

	return messages, nil
}

// GetInboxSummary retrieves inbox statistics for the current agent.
func (c *Client) GetInboxSummary(ctx context.Context) (*InboxSummary, error) {
	query := `SELECT * FROM inbox_summary($1, $2)`

	var summary InboxSummary
	var byKindJSON []byte
	var oldestUnread sql.NullTime

	err := c.db.QueryRowContext(ctx, query, c.project, c.agent).Scan(
		&summary.TotalUnread,
		&byKindJSON,
		&summary.UrgentCount,
		&oldestUnread,
	)
	if err == sql.ErrNoRows {
		return &InboxSummary{ByKind: make(map[string]int64)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying inbox summary: %w", err)
	}

	// Parse by_kind JSON
	summary.ByKind = make(map[string]int64)
	if len(byKindJSON) > 0 {
		// Parse the JSONB as a map
		var byKindRaw map[string]interface{}
		if err := json.Unmarshal(byKindJSON, &byKindRaw); err == nil {
			for k, v := range byKindRaw {
				if n, ok := v.(float64); ok {
					summary.ByKind[k] = int64(n)
				}
			}
		}
	}

	if oldestUnread.Valid {
		summary.OldestUnread = &oldestUnread.Time
	}

	return &summary, nil
}

// MarkRead marks messages as read for the current agent.
func (c *Client) MarkRead(ctx context.Context, shardIDs []string) (int, error) {
	query := `SELECT mark_read($1, $2)`

	var count int
	err := c.db.QueryRowContext(ctx, query, pq.Array(shardIDs), c.agent).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("marking messages read: %w", err)
	}

	return count, nil
}

// nullIfEmptyArray returns nil if the slice is empty or nil.
func nullIfEmptyArray(s []string) interface{} {
	if len(s) == 0 {
		return nil
	}
	return pq.Array(s)
}

// SearchShards performs full-text search on shards.
func (c *Client) SearchShards(ctx context.Context, query string, shardType string) ([]Shard, error) {
	sqlQuery := `
		SELECT s.id, s.project, s.title, s.content, s.type, s.status, s.owner,
		       s.priority, s.creator, s.parent_id, s.created_at, s.updated_at,
		       s.closed_at, s.closed_reason, s.expires_at,
		       COALESCE(array_agg(l.label) FILTER (WHERE l.label IS NOT NULL), '{}') as labels
		FROM shards s
		LEFT JOIN labels l ON s.id = l.shard_id
		WHERE s.project = $1
		  AND s.search_vector @@ plainto_tsquery('english', $2)
	`

	args := []interface{}{c.project, query}

	if shardType != "" {
		sqlQuery += " AND s.type = $3"
		args = append(args, shardType)
	}

	sqlQuery += " GROUP BY s.id ORDER BY ts_rank(s.search_vector, plainto_tsquery('english', $2)) DESC"

	rows, err := c.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("searching shards: %w", err)
	}
	defer rows.Close()

	var shards []Shard
	for rows.Next() {
		var shard Shard
		var content, owner, parentID, resolution sql.NullString
		var closedAt, expiresAt sql.NullTime
		var priority sql.NullInt64

		err := rows.Scan(
			&shard.ID,
			&shard.Project,
			&shard.Title,
			&content,
			&shard.Type,
			&shard.Status,
			&owner,
			&priority,
			&shard.Creator,
			&parentID,
			&shard.CreatedAt,
			&shard.UpdatedAt,
			&closedAt,
			&resolution,
			&expiresAt,
			pq.Array(&shard.Labels),
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		shard.Content = content.String
		shard.Owner = owner.String
		shard.ParentID = parentID.String
		shard.Resolution = resolution.String
		if priority.Valid {
			shard.Priority = int(priority.Int64)
		}
		if closedAt.Valid {
			shard.ClosedAt = &closedAt.Time
		}
		if expiresAt.Valid {
			shard.ExpiresAt = &expiresAt.Time
		}

		shards = append(shards, shard)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return shards, nil
}
