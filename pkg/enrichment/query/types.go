package query

import "time"

// Person represents a resolved person entity.
type Person struct {
	ID            int64     `json:"id"`
	CanonicalName string    `json:"canonical_name"`
	PrimaryEmail  string    `json:"primary_email"`
	AccountType   string    `json:"account_type"` // person, bot, distribution, role_account
	Title         string    `json:"title,omitempty"`
	Department    string    `json:"department,omitempty"`
	IsInternal    bool      `json:"is_internal"`
	Aliases       []string  `json:"aliases,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PeopleFilters provides filter options for people queries.
type PeopleFilters struct {
	AccountType string // person, bot, distribution, role_account
	IsInternal  *bool  // Filter by internal/external
	HasRecent   *bool  // Has communications in last 30 days
	TeamID      *int64 // Filter by team membership
	TenantID    string // Filter by tenant
}

// Thread represents an email thread.
type Thread struct {
	ID            string    `json:"id"`
	Subject       string    `json:"subject"`
	MessageCount  int       `json:"message_count"`
	ParticipantCount int    `json:"participant_count"`
	Participants  []Person  `json:"participants,omitempty"`
	ProjectID     *int64    `json:"project_id,omitempty"`
	ProjectName   string    `json:"project_name,omitempty"`
	FirstMessageAt time.Time `json:"first_message_at"`
	LastMessageAt  time.Time `json:"last_message_at"`
	HasActions    bool      `json:"has_actions"`
	HasDecisions  bool      `json:"has_decisions"`
}

// Source represents a content source (email, document, etc.).
type Source struct {
	ID            int64                  `json:"id"`
	TenantID      string                 `json:"tenant_id"`
	ContentType   string                 `json:"content_type"`
	ContentSubtype string               `json:"content_subtype"`
	Subject       string                 `json:"subject,omitempty"`
	FromName      string                 `json:"from_name,omitempty"`
	FromEmail     string                 `json:"from_email,omitempty"`
	ReceivedAt    time.Time              `json:"received_at"`
	ThreadID      string                 `json:"thread_id,omitempty"`
	ProjectID     *int64                 `json:"project_id,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Summary       string                 `json:"summary,omitempty"`
	AIProcessed   bool                   `json:"ai_processed"`
}

// Assertion represents an extracted assertion (action, decision, risk, etc.).
type Assertion struct {
	ID              int64      `json:"id"`
	TenantID        string     `json:"tenant_id"`
	SourceID        int64      `json:"source_id"`
	ThreadID        *int64     `json:"thread_id,omitempty"`
	Type            string     `json:"assertion_type"` // risk, action, issue, decision, commitment, question
	Description     string     `json:"description"`
	SourceQuote     string     `json:"source_quote,omitempty"`
	Confidence      float64    `json:"confidence"`
	Status          string     `json:"status,omitempty"`          // For actions: open, in_progress, completed, cancelled
	Severity        string     `json:"severity,omitempty"`        // For risks/issues: low, medium, high, critical
	OwnerID         *int64     `json:"owner_id,omitempty"`
	OwnerName       string     `json:"owner_name,omitempty"`
	AssigneeID      *int64     `json:"assignee_id,omitempty"`
	AssigneeName    string     `json:"assignee_name,omitempty"`
	ProjectID       *int64     `json:"project_id,omitempty"`
	ProjectName     string     `json:"project_name,omitempty"`
	DueDate         *time.Time `json:"due_date,omitempty"`
	Rationale       string     `json:"rationale,omitempty"`       // For decisions
	IsCurrent       bool       `json:"is_current"`                // Not superseded
	SupersededBy    *int64     `json:"superseded_by,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// AssertionFilters provides filter options for assertion queries.
type AssertionFilters struct {
	Type        string    // risk, action, issue, decision, commitment, question
	Status      string    // open, in_progress, completed, cancelled
	AssigneeID  *int64
	OwnerID     *int64
	ProjectID   *int64
	TicketID    *int64
	DateRange   TimeRange
	IsCurrent   *bool     // Filter to current (not superseded)
	TenantID    string
}

// JiraTicket represents a Jira ticket reference.
type JiraTicket struct {
	ID             int64     `json:"id"`
	TenantID       string    `json:"tenant_id"`
	Key            string    `json:"key"`
	Summary        string    `json:"summary"`
	Status         string    `json:"status"`
	StatusCategory string    `json:"status_category"` // todo, in_progress, done
	Type           string    `json:"type"`            // bug, story, task, epic
	Priority       string    `json:"priority,omitempty"`
	AssigneeID     *int64    `json:"assignee_id,omitempty"`
	AssigneeName   string    `json:"assignee_name,omitempty"`
	ReporterID     *int64    `json:"reporter_id,omitempty"`
	ReporterName   string    `json:"reporter_name,omitempty"`
	ProjectKey     string    `json:"project_key"`
	PenfoldProjectID *int64  `json:"penfold_project_id,omitempty"`
	FirstSeenAt    time.Time `json:"first_seen_at"`
	LastUpdatedAt  time.Time `json:"last_updated_at"`
	ReferenceCount int       `json:"reference_count"` // Number of emails mentioning this ticket
}

// TicketChange represents a change to a Jira ticket.
type TicketChange struct {
	ID          int64     `json:"id"`
	TicketID    int64     `json:"ticket_id"`
	ChangeType  string    `json:"change_type"` // status_changed, assigned, commented, etc.
	FieldName   string    `json:"field_name,omitempty"`
	OldValue    string    `json:"old_value,omitempty"`
	NewValue    string    `json:"new_value,omitempty"`
	ChangedBy   string    `json:"changed_by,omitempty"`
	ChangedAt   time.Time `json:"changed_at"`
	SourceID    *int64    `json:"source_id,omitempty"` // Email that reported this change
}

// Project represents a Penfold project.
type Project struct {
	ID           int64     `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Status       string    `json:"status"` // active, completed, on_hold, cancelled
	Keywords     []string  `json:"keywords,omitempty"`
	JiraProjects []string  `json:"jira_projects,omitempty"`
	MemberCount  int       `json:"member_count"`
	ThreadCount  int       `json:"thread_count"`
	ActionCount  int       `json:"action_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ActivityItem represents a project activity item.
type ActivityItem struct {
	Type        string    `json:"type"` // email, decision, action, ticket_update
	Description string    `json:"description"`
	SourceID    *int64    `json:"source_id,omitempty"`
	ActorName   string    `json:"actor_name,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// EnrichmentStatus represents the enrichment status of a source.
type EnrichmentStatus struct {
	SourceID         int64     `json:"source_id"`
	ContentType      string    `json:"content_type"`
	ContentSubtype   string    `json:"content_subtype"`
	ProcessingProfile string   `json:"processing_profile"`
	Stages           []StageStatus `json:"stages"`
	TotalDurationMs  int64     `json:"total_duration_ms"`
	Errors           []string  `json:"errors,omitempty"`
	EnrichedAt       *time.Time `json:"enriched_at,omitempty"`
}

// StageStatus represents the status of a pipeline stage.
type StageStatus struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"` // completed, failed, skipped, pending
	DurationMs  int64     `json:"duration_ms"`
	Processor   string    `json:"processor"`
	Outputs     []string  `json:"outputs,omitempty"`
	Error       string    `json:"error,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// EnrichmentStats provides aggregated enrichment statistics.
type EnrichmentStats struct {
	TenantID           string        `json:"tenant_id"`
	TimeRange          TimeRange     `json:"time_range"`
	TotalProcessed     int64         `json:"total_processed"`
	TotalFailed        int64         `json:"total_failed"`
	TotalSkipped       int64         `json:"total_skipped"`
	AvgProcessingMs    float64       `json:"avg_processing_ms"`
	QueueDepths        QueueDepths   `json:"queue_depths"`
	ClassificationStats map[string]int64 `json:"classification_stats"` // by content_type
	AIStats            AIStats       `json:"ai_stats"`
}

// QueueDepths shows current queue depths.
type QueueDepths struct {
	Ingest     int64 `json:"ingest"`
	Enrichment int64 `json:"enrichment"`
	AI         int64 `json:"ai"`
	DLQ        int64 `json:"dlq"`
}

// AIStats shows AI processing statistics.
type AIStats struct {
	TotalOperations int64   `json:"total_operations"`
	TotalTokens     int64   `json:"total_tokens"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	ParseSuccessRate float64 `json:"parse_success_rate"`
}
