package handlers

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
	"github.com/otherjamesbrown/penf-cli/pkg/enrichment/processors"
)

// JiraChangeType represents types of changes in Jira notifications.
type JiraChangeType string

const (
	JiraChangeCreated         JiraChangeType = "created"
	JiraChangeStatusChanged   JiraChangeType = "status_changed"
	JiraChangeAssigned        JiraChangeType = "assigned"
	JiraChangeCommented       JiraChangeType = "commented"
	JiraChangePriorityChanged JiraChangeType = "priority_changed"
	JiraChangeLabelAdded      JiraChangeType = "label_added"
	JiraChangeLabelRemoved    JiraChangeType = "label_removed"
	JiraChangeEpicLinked      JiraChangeType = "epic_linked"
	JiraChangeResolved        JiraChangeType = "resolved"
	JiraChangeReopened        JiraChangeType = "reopened"
	JiraChangeOther           JiraChangeType = "other"
)

// JiraTicketData represents extracted Jira ticket information.
type JiraTicketData struct {
	TicketKey    string         `json:"ticket_key"`
	ProjectKey   string         `json:"project_key"`
	Summary      string         `json:"summary,omitempty"`
	Status       string         `json:"status,omitempty"`
	Priority     string         `json:"priority,omitempty"`
	Assignee     string         `json:"assignee,omitempty"`
	AssigneeEmail string        `json:"assignee_email,omitempty"`
	Reporter     string         `json:"reporter,omitempty"`
	ReporterEmail string        `json:"reporter_email,omitempty"`
	ChangeType   JiraChangeType `json:"change_type"`
	ChangedBy    string         `json:"changed_by,omitempty"`
	ChangedByEmail string       `json:"changed_by_email,omitempty"`
	FieldName    string         `json:"field_name,omitempty"`
	FromValue    string         `json:"from_value,omitempty"`
	ToValue      string         `json:"to_value,omitempty"`
	Comment      string         `json:"comment,omitempty"`
	ChangedAt    time.Time      `json:"changed_at"`
}

// JiraExtractor extracts Jira ticket information from notification emails.
// It runs in Stage 3 (Type-Specific) for notification/jira content.
type JiraExtractor struct{}

// NewJiraExtractor creates a new JiraExtractor processor.
func NewJiraExtractor() *JiraExtractor {
	return &JiraExtractor{}
}

// Name returns the processor name.
func (j *JiraExtractor) Name() string {
	return "JiraExtractor"
}

// Stage returns the pipeline stage.
func (j *JiraExtractor) Stage() processors.Stage {
	return processors.StageTypeSpecific
}

// Subtypes returns the content subtypes this processor handles.
func (j *JiraExtractor) Subtypes() []enrichment.ContentSubtype {
	return []enrichment.ContentSubtype{enrichment.SubtypeNotificationJira}
}

// Process implements the Processor interface.
func (j *JiraExtractor) Process(ctx context.Context, pctx *processors.ProcessorContext) error {
	return j.Extract(ctx, pctx)
}

// Extract extracts Jira ticket information from the notification.
func (j *JiraExtractor) Extract(ctx context.Context, pctx *processors.ProcessorContext) error {
	data, err := j.extractTicketData(pctx.Source)
	if err != nil {
		return err
	}

	if data != nil {
		if pctx.Enrichment.ExtractedData == nil {
			pctx.Enrichment.ExtractedData = make(map[string]interface{})
		}
		pctx.Enrichment.ExtractedData["jira"] = data
	}

	return nil
}

// extractTicketData extracts Jira ticket data from the source.
func (j *JiraExtractor) extractTicketData(source *processors.Source) (*JiraTicketData, error) {
	data := &JiraTicketData{
		ChangedAt: time.Now(),
	}

	// Extract ticket key from subject
	subject := j.getMetadataString(source, "subject")
	ticketKey := j.extractTicketKey(subject)
	if ticketKey == "" {
		// Try extracting from body
		bodyText := j.getMetadataString(source, "body_text", "body")
		ticketKey = j.extractTicketKey(bodyText)
	}

	if ticketKey == "" {
		return nil, nil // No ticket found
	}

	data.TicketKey = ticketKey
	data.ProjectKey = j.extractProjectKey(ticketKey)

	// Extract summary from subject (after ticket key)
	data.Summary = j.extractSummary(subject, ticketKey)

	// Determine change type from subject
	data.ChangeType = j.detectChangeType(subject)

	// Extract details from body
	bodyText := j.getMetadataString(source, "body_text", "body")
	j.extractDetailsFromBody(bodyText, data)

	// Get timestamp from email
	if ts, ok := source.Metadata["date"].(time.Time); ok {
		data.ChangedAt = ts
	} else if ts, ok := source.Metadata["received_at"].(time.Time); ok {
		data.ChangedAt = ts
	}

	// Get changed_by from sender
	from := j.getMetadataString(source, "from")
	data.ChangedByEmail = j.extractEmail(from)
	data.ChangedBy = j.extractName(from)

	return data, nil
}

// ticketKeyPattern matches Jira ticket keys like OUT-123, TRACK-JIRA-456
var ticketKeyPattern = regexp.MustCompile(`\b([A-Z]{2,10}-\d+)\b`)

// extractTicketKey extracts the Jira ticket key from text.
func (j *JiraExtractor) extractTicketKey(text string) string {
	// First try to find in brackets like [OUT-697]
	bracketPattern := regexp.MustCompile(`\[([A-Z]{2,10}-\d+)\]`)
	if matches := bracketPattern.FindStringSubmatch(text); len(matches) >= 2 {
		return matches[1]
	}

	// Then try general pattern
	if matches := ticketKeyPattern.FindStringSubmatch(text); len(matches) >= 2 {
		return matches[1]
	}

	return ""
}

// extractProjectKey extracts the project key from a ticket key.
func (j *JiraExtractor) extractProjectKey(ticketKey string) string {
	parts := strings.Split(ticketKey, "-")
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

// extractSummary extracts the ticket summary from the subject.
func (j *JiraExtractor) extractSummary(subject, ticketKey string) string {
	// Remove ticket key in brackets
	summary := regexp.MustCompile(`\[`+regexp.QuoteMeta(ticketKey)+`\]\s*`).ReplaceAllString(subject, "")
	// Remove common prefixes
	summary = regexp.MustCompile(`(?i)^(Updates?\s+for\s+|Re:\s*|Fwd?:\s*)`).ReplaceAllString(summary, "")
	return strings.TrimSpace(summary)
}

// detectChangeType determines the type of change from the subject.
func (j *JiraExtractor) detectChangeType(subject string) JiraChangeType {
	subjectLower := strings.ToLower(subject)

	switch {
	case strings.Contains(subjectLower, "created"):
		return JiraChangeCreated
	case strings.Contains(subjectLower, "assigned") || strings.Contains(subjectLower, "assignee"):
		return JiraChangeAssigned
	case strings.Contains(subjectLower, "commented") || strings.Contains(subjectLower, "comment"):
		return JiraChangeCommented
	case strings.Contains(subjectLower, "resolved"):
		return JiraChangeResolved
	case strings.Contains(subjectLower, "reopened"):
		return JiraChangeReopened
	case strings.Contains(subjectLower, "status"):
		return JiraChangeStatusChanged
	case strings.Contains(subjectLower, "priority"):
		return JiraChangePriorityChanged
	case strings.Contains(subjectLower, "updated") || strings.Contains(subjectLower, "update"):
		return JiraChangeOther
	default:
		return JiraChangeOther
	}
}

// extractDetailsFromBody extracts additional details from the email body.
func (j *JiraExtractor) extractDetailsFromBody(body string, data *JiraTicketData) {
	if body == "" {
		return
	}

	// Extract status change pattern: "Status: Open → In Progress"
	statusPattern := regexp.MustCompile(`(?i)Status:\s*(\S+)\s*(?:→|->|to)\s*(\S+)`)
	if matches := statusPattern.FindStringSubmatch(body); len(matches) >= 3 {
		data.FieldName = "status"
		data.FromValue = matches[1]
		data.ToValue = matches[2]
		data.Status = matches[2]
		if data.ChangeType == JiraChangeOther {
			data.ChangeType = JiraChangeStatusChanged
		}
	}

	// Extract priority pattern: "Priority: High"
	priorityPattern := regexp.MustCompile(`(?i)Priority:\s*(\S+)`)
	if matches := priorityPattern.FindStringSubmatch(body); len(matches) >= 2 {
		data.Priority = matches[1]
	}

	// Extract assignee pattern: "Assignee: John Smith"
	assigneePattern := regexp.MustCompile(`(?i)Assignee:\s*([^\n]+)`)
	if matches := assigneePattern.FindStringSubmatch(body); len(matches) >= 2 {
		data.Assignee = strings.TrimSpace(matches[1])
	}

	// Extract reporter pattern: "Reporter: Jane Doe"
	reporterPattern := regexp.MustCompile(`(?i)Reporter:\s*([^\n]+)`)
	if matches := reporterPattern.FindStringSubmatch(body); len(matches) >= 2 {
		data.Reporter = strings.TrimSpace(matches[1])
	}

	// Extract comment (usually after "commented:" or in quotes)
	commentPattern := regexp.MustCompile(`(?i)(?:commented?:?\s*|added a comment[:\s]+)(.+?)(?:\n\n|$)`)
	if matches := commentPattern.FindStringSubmatch(body); len(matches) >= 2 {
		data.Comment = strings.TrimSpace(matches[1])
	}
}

// getMetadataString extracts a string from source metadata.
func (j *JiraExtractor) getMetadataString(source *processors.Source, keys ...string) string {
	if source.Metadata == nil {
		return ""
	}

	for _, key := range keys {
		if val, ok := source.Metadata[key].(string); ok && val != "" {
			return val
		}
	}

	// Also check nested headers
	if headers, ok := source.Metadata["headers"].(map[string]interface{}); ok {
		for _, key := range keys {
			if val, ok := headers[key].(string); ok && val != "" {
				return val
			}
		}
	}

	return ""
}

// emailPattern matches email addresses.
var emailPattern = regexp.MustCompile(`([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)

// extractEmail extracts an email address from a "Name <email>" string.
func (j *JiraExtractor) extractEmail(from string) string {
	if matches := emailPattern.FindStringSubmatch(from); len(matches) >= 2 {
		return strings.ToLower(matches[1])
	}
	return ""
}

// extractName extracts the display name from a "Name <email>" string.
func (j *JiraExtractor) extractName(from string) string {
	// Remove <email> part
	name := regexp.MustCompile(`\s*<[^>]+>\s*`).ReplaceAllString(from, "")
	name = strings.Trim(name, `"' `)
	return name
}

// Verify interface compliance
var _ processors.TypeSpecificProcessor = (*JiraExtractor)(nil)
var _ processors.Processor = (*JiraExtractor)(nil)
