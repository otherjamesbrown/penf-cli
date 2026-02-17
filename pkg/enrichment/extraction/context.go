// Package extraction provides AI extraction capabilities for the enrichment pipeline.
package extraction

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
)

// ExtractionContext holds all context to be injected into extraction prompts.
type ExtractionContext struct {
	Participants     []ParticipantContext `json:"participants,omitempty"`
	Project          *ProjectContext      `json:"project,omitempty"`
	Thread           *ThreadContext       `json:"thread,omitempty"`
	LinkedTickets    []TicketContext      `json:"linked_tickets,omitempty"`
	PriorDecisions   []DecisionContext    `json:"prior_decisions,omitempty"`
	RecentMeetings   []MeetingContext     `json:"recent_meetings,omitempty"`
	TokensUsed       int                  `json:"tokens_used"`
}

// ParticipantContext provides context about a participant.
type ParticipantContext struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Role         string `json:"role,omitempty"`
	Title        string `json:"title,omitempty"`
	Department   string `json:"department,omitempty"`
	IsInternal   bool   `json:"is_internal"`
	Relationship string `json:"relationship,omitempty"` // project owner, stakeholder, etc.
}

// ProjectContext provides context about the matched project.
type ProjectContext struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Goal        string   `json:"goal,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	TicketKeys  []string `json:"ticket_keys,omitempty"` // Linked Jira tickets
}

// ThreadContext provides context about the email thread.
type ThreadContext struct {
	ThreadID      string            `json:"thread_id"`
	Position      int               `json:"position"`       // This message's position (1-indexed)
	TotalMessages int               `json:"total_messages"`
	PriorMessages []MessagePreview  `json:"prior_messages,omitempty"`
	DateRange     string            `json:"date_range,omitempty"`
}

// MessagePreview is a summary of a prior message in a thread.
type MessagePreview struct {
	Position int       `json:"position"`
	From     string    `json:"from"`
	Date     time.Time `json:"date"`
	Preview  string    `json:"preview"` // First N chars of body
}

// TicketContext provides context about a linked Jira ticket.
type TicketContext struct {
	Key           string `json:"key"`
	Summary       string `json:"summary"`
	Status        string `json:"status"`
	StatusCategory string `json:"status_category,omitempty"` // todo, in_progress, done
	Assignee      string `json:"assignee,omitempty"`
	Priority      string `json:"priority,omitempty"`
}

// DecisionContext provides context about a prior decision.
type DecisionContext struct {
	Description   string    `json:"description"`
	DecisionMaker string    `json:"decision_maker,omitempty"`
	Date          time.Time `json:"date"`
	SourceSummary string    `json:"source_summary,omitempty"` // Where decision came from
}

// MeetingContext provides context about a recent meeting.
type MeetingContext struct {
	Title     string    `json:"title"`
	Date      time.Time `json:"date"`
	Organizer string    `json:"organizer,omitempty"`
}

// ContextTier defines the level of context to inject.
type ContextTier string

const (
	ContextTierFull     ContextTier = "full"     // Full context for threads (~500 tokens)
	ContextTierStandard ContextTier = "standard" // Participants + project (~200 tokens)
	ContextTierMinimal  ContextTier = "minimal"  // Project + metadata only (~150 tokens)
	ContextTierNone     ContextTier = "none"     // No context injection
)

// GetContextTier returns the appropriate context tier for a processing profile.
func GetContextTier(profile enrichment.ProcessingProfile) ContextTier {
	switch profile {
	case enrichment.ProfileFullAI:
		return ContextTierFull
	case enrichment.ProfileFullAIChunked:
		return ContextTierMinimal
	case enrichment.ProfileMetadataOnly, enrichment.ProfileStateTracking, enrichment.ProfileStructureOnly:
		return ContextTierNone
	default:
		return ContextTierStandard
	}
}

// ContextBuilderConfig configures the context builder.
type ContextBuilderConfig struct {
	MaxParticipants    int `yaml:"max_participants"`
	MaxPriorMessages   int `yaml:"max_prior_messages"`
	MaxPriorDecisions  int `yaml:"max_prior_decisions"`
	MaxTickets         int `yaml:"max_tickets"`
	MessagePreviewLen  int `yaml:"message_preview_len"`
	FullTokenBudget    int `yaml:"full_token_budget"`
	StandardTokenBudget int `yaml:"standard_token_budget"`
	MinimalTokenBudget int `yaml:"minimal_token_budget"`
}

// DefaultContextBuilderConfig returns the default configuration.
func DefaultContextBuilderConfig() ContextBuilderConfig {
	return ContextBuilderConfig{
		MaxParticipants:     10,
		MaxPriorMessages:    5,
		MaxPriorDecisions:   3,
		MaxTickets:          3,
		MessagePreviewLen:   500,
		FullTokenBudget:     500,
		StandardTokenBudget: 200,
		MinimalTokenBudget:  150,
	}
}

// ContextRepository provides data access for context building.
type ContextRepository interface {
	// GetPerson retrieves a person by ID.
	GetPerson(ctx context.Context, id int64) (*Person, error)
	// GetProject retrieves a project by ID.
	GetProject(ctx context.Context, id int64) (*Project, error)
	// GetThreadMessages retrieves messages in a thread.
	GetThreadMessages(ctx context.Context, threadID string, limit int) ([]ThreadMessage, error)
	// GetProjectTickets retrieves Jira tickets linked to a project.
	GetProjectTickets(ctx context.Context, projectID int64, limit int) ([]Ticket, error)
	// GetProjectDecisions retrieves prior decisions for a project.
	GetProjectDecisions(ctx context.Context, projectID int64, limit int) ([]Decision, error)
	// GetRecentMeetings retrieves recent meetings with participants.
	GetRecentMeetings(ctx context.Context, participantIDs []int64, limit int) ([]Meeting, error)
}

// Person is a simplified person model for context building.
type Person struct {
	ID           int64
	CanonicalName string
	PrimaryEmail string
	Title        string
	Department   string
	IsInternal   bool
}

// Project is a simplified project model for context building.
type Project struct {
	ID          int64
	Name        string
	Description string
	Keywords    []string
	JiraProjects []string
}

// ThreadMessage is a simplified message model for context building.
type ThreadMessage struct {
	SourceID   int64
	MessageID  string
	FromName   string
	FromEmail  string
	Date       time.Time
	BodyText   string
	Position   int
}

// Ticket is a simplified Jira ticket model for context building.
type Ticket struct {
	Key            string
	Summary        string
	Status         string
	StatusCategory string
	AssigneeName   string
	Priority       string
}

// Decision is a simplified decision model for context building.
type Decision struct {
	Description   string
	DecisionMaker string
	Date          time.Time
	SourceSummary string
}

// Meeting is a simplified meeting model for context building.
type Meeting struct {
	Title     string
	StartTime time.Time
	Organizer string
}

// ContextBuilder builds extraction context from enrichment data.
type ContextBuilder struct {
	config ContextBuilderConfig
	repo   ContextRepository
}

// NewContextBuilder creates a new context builder.
func NewContextBuilder(repo ContextRepository, opts ...ContextBuilderOption) *ContextBuilder {
	cb := &ContextBuilder{
		config: DefaultContextBuilderConfig(),
		repo:   repo,
	}
	for _, opt := range opts {
		opt(cb)
	}
	return cb
}

// ContextBuilderOption configures the context builder.
type ContextBuilderOption func(*ContextBuilder)

// WithContextConfig sets the context builder configuration.
func WithContextConfig(config ContextBuilderConfig) ContextBuilderOption {
	return func(cb *ContextBuilder) {
		cb.config = config
	}
}

// Build constructs the extraction context for an enrichment.
func (cb *ContextBuilder) Build(ctx context.Context, e *enrichment.Enrichment, tier ContextTier) (*ExtractionContext, error) {
	if tier == ContextTierNone {
		return nil, nil
	}

	tokenBudget := cb.getTokenBudget(tier)
	ec := &ExtractionContext{}
	tokensUsed := 0

	// Always include participants (high value, relatively low tokens)
	participants, tokens := cb.buildParticipantContext(ctx, e)
	if tokensUsed+tokens <= tokenBudget {
		ec.Participants = participants
		tokensUsed += tokens
	}

	// Include project if identified
	if e.ProjectID != nil && tier != ContextTierMinimal {
		project, tokens := cb.buildProjectContext(ctx, *e.ProjectID)
		if project != nil && tokensUsed+tokens <= tokenBudget {
			ec.Project = project
			tokensUsed += tokens
		}
	}

	// Include thread context for thread messages (full tier only)
	if tier == ContextTierFull && e.ThreadID != "" {
		thread, tokens := cb.buildThreadContext(ctx, e.ThreadID)
		if thread != nil && tokensUsed+tokens <= tokenBudget {
			ec.Thread = thread
			tokensUsed += tokens
		}
	}

	// Include linked tickets if budget allows
	if e.ProjectID != nil && tokensUsed+50 <= tokenBudget {
		tickets, tokens := cb.buildTicketContext(ctx, *e.ProjectID)
		if len(tickets) > 0 && tokensUsed+tokens <= tokenBudget {
			ec.LinkedTickets = tickets
			tokensUsed += tokens
		}
	}

	// Include prior decisions if budget allows
	if e.ProjectID != nil && tier == ContextTierFull && tokensUsed+100 <= tokenBudget {
		decisions, tokens := cb.buildDecisionContext(ctx, *e.ProjectID)
		if len(decisions) > 0 && tokensUsed+tokens <= tokenBudget {
			ec.PriorDecisions = decisions
			tokensUsed += tokens
		}
	}

	ec.TokensUsed = tokensUsed
	return ec, nil
}

func (cb *ContextBuilder) getTokenBudget(tier ContextTier) int {
	switch tier {
	case ContextTierFull:
		return cb.config.FullTokenBudget
	case ContextTierStandard:
		return cb.config.StandardTokenBudget
	case ContextTierMinimal:
		return cb.config.MinimalTokenBudget
	default:
		return 0
	}
}

func (cb *ContextBuilder) buildParticipantContext(ctx context.Context, e *enrichment.Enrichment) ([]ParticipantContext, int) {
	var participants []ParticipantContext

	for i, rp := range e.ResolvedParticipants {
		if i >= cb.config.MaxParticipants {
			break
		}

		pc := ParticipantContext{
			Name:       rp.Name,
			Email:      rp.Email,
			Role:       rp.Role,
			IsInternal: rp.IsInternal != nil && *rp.IsInternal,
		}

		// Get additional person details if resolved
		if rp.PersonID != nil && cb.repo != nil {
			if person, err := cb.repo.GetPerson(ctx, *rp.PersonID); err == nil && person != nil {
				pc.Title = person.Title
				pc.Department = person.Department
				pc.IsInternal = person.IsInternal
				if pc.Name == "" {
					pc.Name = person.CanonicalName
				}
			}
		}

		participants = append(participants, pc)
	}

	// Estimate tokens: ~15 tokens per participant on average
	return participants, len(participants) * 15
}

func (cb *ContextBuilder) buildProjectContext(ctx context.Context, projectID int64) (*ProjectContext, int) {
	if cb.repo == nil {
		return nil, 0
	}

	project, err := cb.repo.GetProject(ctx, projectID)
	if err != nil || project == nil {
		return nil, 0
	}

	pc := &ProjectContext{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		Keywords:    project.Keywords,
	}

	// Estimate tokens: ~30 tokens for basic project context
	return pc, 30
}

func (cb *ContextBuilder) buildThreadContext(ctx context.Context, threadID string) (*ThreadContext, int) {
	if cb.repo == nil {
		return nil, 0
	}

	messages, err := cb.repo.GetThreadMessages(ctx, threadID, cb.config.MaxPriorMessages+1)
	if err != nil || len(messages) == 0 {
		return nil, 0
	}

	tc := &ThreadContext{
		ThreadID:      threadID,
		TotalMessages: len(messages),
		Position:      len(messages), // Current message is latest
	}

	// Build previews for prior messages (exclude latest)
	for i := 0; i < len(messages)-1 && i < cb.config.MaxPriorMessages; i++ {
		msg := messages[i]
		preview := truncateText(msg.BodyText, cb.config.MessagePreviewLen)
		tc.PriorMessages = append(tc.PriorMessages, MessagePreview{
			Position: msg.Position,
			From:     msg.FromName,
			Date:     msg.Date,
			Preview:  preview,
		})
	}

	// Estimate tokens: ~50 tokens per message preview
	return tc, len(tc.PriorMessages) * 50
}

func (cb *ContextBuilder) buildTicketContext(ctx context.Context, projectID int64) ([]TicketContext, int) {
	if cb.repo == nil {
		return nil, 0
	}

	tickets, err := cb.repo.GetProjectTickets(ctx, projectID, cb.config.MaxTickets)
	if err != nil || len(tickets) == 0 {
		return nil, 0
	}

	var result []TicketContext
	for _, t := range tickets {
		result = append(result, TicketContext{
			Key:            t.Key,
			Summary:        t.Summary,
			Status:         t.Status,
			StatusCategory: t.StatusCategory,
			Assignee:       t.AssigneeName,
			Priority:       t.Priority,
		})
	}

	// Estimate tokens: ~20 tokens per ticket
	return result, len(result) * 20
}

func (cb *ContextBuilder) buildDecisionContext(ctx context.Context, projectID int64) ([]DecisionContext, int) {
	if cb.repo == nil {
		return nil, 0
	}

	decisions, err := cb.repo.GetProjectDecisions(ctx, projectID, cb.config.MaxPriorDecisions)
	if err != nil || len(decisions) == 0 {
		return nil, 0
	}

	var result []DecisionContext
	for _, d := range decisions {
		result = append(result, DecisionContext{
			Description:   d.Description,
			DecisionMaker: d.DecisionMaker,
			Date:          d.Date,
			SourceSummary: d.SourceSummary,
		})
	}

	// Estimate tokens: ~25 tokens per decision
	return result, len(result) * 25
}

// truncateText truncates text to maxLen characters, adding ellipsis if needed.
func truncateText(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxLen {
		return text
	}
	// Try to break at word boundary
	truncated := text[:maxLen]
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > 0 && lastSpace > maxLen-50 {
		truncated = truncated[:lastSpace]
	}
	return truncated + "..."
}

// FormatContextForPrompt formats the extraction context for inclusion in a prompt.
func FormatContextForPrompt(ec *ExtractionContext) string {
	if ec == nil {
		return ""
	}

	var parts []string

	// Participants
	if len(ec.Participants) > 0 {
		var pLines []string
		for _, p := range ec.Participants {
			line := fmt.Sprintf("- %s <%s>", p.Name, p.Email)
			if p.Title != "" {
				line += " - " + p.Title
			}
			if p.Department != "" {
				line += " (" + p.Department + ")"
			}
			pLines = append(pLines, line)
		}
		parts = append(parts, "PARTICIPANTS:\n"+strings.Join(pLines, "\n"))
	}

	// Project
	if ec.Project != nil {
		pStr := fmt.Sprintf("PROJECT: %s", ec.Project.Name)
		if ec.Project.Description != "" {
			pStr += "\n  Description: " + ec.Project.Description
		}
		parts = append(parts, pStr)
	}

	// Thread
	if ec.Thread != nil && len(ec.Thread.PriorMessages) > 0 {
		var tLines []string
		tLines = append(tLines, fmt.Sprintf("THREAD CONTEXT (message %d of %d):",
			ec.Thread.Position, ec.Thread.TotalMessages))
		for _, m := range ec.Thread.PriorMessages {
			tLines = append(tLines, fmt.Sprintf("  Msg %d (%s): %s",
				m.Position, m.From, m.Preview))
		}
		parts = append(parts, strings.Join(tLines, "\n"))
	}

	// Linked tickets
	if len(ec.LinkedTickets) > 0 {
		var tLines []string
		tLines = append(tLines, "LINKED TICKETS:")
		for _, t := range ec.LinkedTickets {
			tLines = append(tLines, fmt.Sprintf("  - %s: %s [%s]", t.Key, t.Summary, t.Status))
		}
		parts = append(parts, strings.Join(tLines, "\n"))
	}

	// Prior decisions
	if len(ec.PriorDecisions) > 0 {
		var dLines []string
		dLines = append(dLines, "PRIOR DECISIONS:")
		for _, d := range ec.PriorDecisions {
			line := fmt.Sprintf("  - %s", d.Description)
			if d.DecisionMaker != "" {
				line += fmt.Sprintf(" (by %s on %s)", d.DecisionMaker, d.Date.Format("2006-01-02"))
			}
			dLines = append(dLines, line)
		}
		parts = append(parts, strings.Join(dLines, "\n"))
	}

	return strings.Join(parts, "\n\n")
}
