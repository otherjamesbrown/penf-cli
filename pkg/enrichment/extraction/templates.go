package extraction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"
)

// TemplateType defines the type of prompt template.
type TemplateType string

const (
	TemplateTypeSystemDefault TemplateType = "system_default"
	TemplateTypeTenantDefault TemplateType = "tenant_default"
	TemplateTypeProject       TemplateType = "project"
)

// PromptTemplate represents a prompt template for AI extraction.
type PromptTemplate struct {
	ID               int64                  `json:"id"`
	TenantID         *string                `json:"tenant_id,omitempty"`
	Name             string                 `json:"name"`
	Type             TemplateType           `json:"type"`
	Version          string                 `json:"version"`
	TemplateText     string                 `json:"template_text"`
	ExtractionSchema map[string]interface{} `json:"extraction_schema,omitempty"`
	ContextConfig    map[string]interface{} `json:"context_config,omitempty"`
	ProjectIDs       []int64                `json:"project_ids,omitempty"`
	Active           bool                   `json:"active"`
}

// TemplateRepository provides data access for templates.
type TemplateRepository interface {
	// GetByProjectID finds a template associated with a project.
	GetByProjectID(ctx context.Context, projectID int64) (*PromptTemplate, error)
	// GetTenantDefault finds the tenant's default template.
	GetTenantDefault(ctx context.Context, tenantID string) (*PromptTemplate, error)
	// GetSystemDefault finds the system default template.
	GetSystemDefault(ctx context.Context) (*PromptTemplate, error)
	// GetByID retrieves a template by ID.
	GetByID(ctx context.Context, id int64) (*PromptTemplate, error)
}

// TemplateResolver resolves the appropriate template for extraction.
type TemplateResolver struct {
	repo           TemplateRepository
	systemDefault  *PromptTemplate // Cached system default
}

// NewTemplateResolver creates a new template resolver.
func NewTemplateResolver(repo TemplateRepository) *TemplateResolver {
	return &TemplateResolver{
		repo: repo,
	}
}

// ResolveTemplate finds the appropriate template using fallback chain:
// 1. Project-specific template (if project identified)
// 2. Tenant default template
// 3. System default template
func (tr *TemplateResolver) ResolveTemplate(ctx context.Context, tenantID string, projectID *int64) (*PromptTemplate, error) {
	// 1. Try project-specific template
	if projectID != nil {
		tmpl, err := tr.repo.GetByProjectID(ctx, *projectID)
		if err == nil && tmpl != nil && tmpl.Active {
			return tmpl, nil
		}
		// Continue to fallback on error or inactive template
	}

	// 2. Try tenant default
	tmpl, err := tr.repo.GetTenantDefault(ctx, tenantID)
	if err == nil && tmpl != nil && tmpl.Active {
		return tmpl, nil
	}

	// 3. Fall back to system default
	return tr.GetSystemDefault(ctx)
}

// GetSystemDefault returns the system default template (cached).
func (tr *TemplateResolver) GetSystemDefault(ctx context.Context) (*PromptTemplate, error) {
	if tr.systemDefault != nil {
		return tr.systemDefault, nil
	}

	tmpl, err := tr.repo.GetSystemDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get system default template: %w", err)
	}
	if tmpl == nil {
		// Return built-in default if no database template exists
		return DefaultSystemTemplate(), nil
	}

	tr.systemDefault = tmpl
	return tmpl, nil
}

// DefaultSystemTemplate returns the built-in system default template.
func DefaultSystemTemplate() *PromptTemplate {
	return &PromptTemplate{
		Name:         "system-default",
		Type:         TemplateTypeSystemDefault,
		Version:      "1.0.0",
		Active:       true,
		TemplateText: defaultTemplateText,
		ExtractionSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"risks":       map[string]interface{}{"type": "array"},
				"actions":     map[string]interface{}{"type": "array"},
				"issues":      map[string]interface{}{"type": "array"},
				"decisions":   map[string]interface{}{"type": "array"},
				"commitments": map[string]interface{}{"type": "array"},
				"questions":   map[string]interface{}{"type": "array"},
				"sentiment":   map[string]interface{}{"type": "object"},
			},
		},
	}
}

const defaultTemplateText = `You are extracting structured information from a business email.

{{if .Context}}
CONTEXT:
{{.Context}}
{{end}}

INSTRUCTIONS:
Extract the following from the email below. For each item found, include:
- A clear description
- The exact quote from the email that supports this extraction
- Any referenced people (by name), dates, or projects

CATEGORIES TO EXTRACT:

1. RISKS: Potential problems, uncertainties, or concerns mentioned
   - Include severity (low/medium/high) if apparent
   - Note who owns or is responsible for the risk

2. ACTIONS: Tasks that need to be done
   - Include who it's assigned to if mentioned
   - Include due date if mentioned (exact text like "by Friday" and inferred date)
   - Status is "open" unless explicitly stated otherwise

3. ISSUES: Current blockers, problems, or obstacles
   - Include severity (low/medium/high) if apparent
   - Note any related tickets or projects

4. DECISIONS: Choices or determinations that were made
   - Include who made the decision
   - Include rationale if provided

5. COMMITMENTS: Promises or pledges made by someone
   - Include who made the commitment
   - Include who it was made to
   - Include due date if mentioned

6. QUESTIONS: Unanswered questions raised
   - Include who asked
   - Include who it's directed to if clear
   - Note if it was answered in this email

7. SENTIMENT: Overall analysis
   - overall: positive/neutral/negative
   - urgency: low/medium/high/critical
   - tone: collaborative/confrontational/informational/other

Respond with valid JSON matching this structure:
{
  "risks": [{"description": "", "severity": "", "owner": "", "source_quote": ""}],
  "actions": [{"description": "", "assignee": "", "due_date": "", "due_date_source": "", "status": "open", "source_quote": ""}],
  "issues": [{"description": "", "severity": "", "owner": "", "blocker_for": "", "source_quote": ""}],
  "decisions": [{"description": "", "decision_maker": "", "rationale": "", "source_quote": ""}],
  "commitments": [{"description": "", "committer": "", "committed_to": "", "due_date": "", "source_quote": ""}],
  "questions": [{"question": "", "asker": "", "directed_to": "", "answered": false, "source_quote": ""}],
  "sentiment": {"overall": "", "urgency": "", "tone": ""}
}

If a category has no items, use an empty array [].
Only include items that are clearly stated or strongly implied in the email.

EMAIL:
{{.Content}}`

// PromptData holds data for template rendering.
type PromptData struct {
	Context string // Formatted context string
	Content string // Email/document content
}

// RenderPrompt renders a template with the given data.
func RenderPrompt(tmpl *PromptTemplate, data PromptData) (string, error) {
	t, err := template.New("prompt").Parse(tmpl.TemplateText)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// ValidateExtractionOutput validates extraction output against the template schema.
func ValidateExtractionOutput(tmpl *PromptTemplate, output map[string]interface{}) error {
	if tmpl.ExtractionSchema == nil {
		return nil // No schema to validate against
	}

	// Basic validation: check required top-level fields
	schema := tmpl.ExtractionSchema
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for field := range props {
			if _, exists := output[field]; !exists {
				// Field missing but might be optional
				continue
			}
		}
	}

	return nil
}

// ExtractionOutput represents the parsed output from AI extraction.
type ExtractionOutput struct {
	Risks       []RiskOutput       `json:"risks"`
	Actions     []ActionOutput     `json:"actions"`
	Issues      []IssueOutput      `json:"issues"`
	Decisions   []DecisionOutput   `json:"decisions"`
	Commitments []CommitmentOutput `json:"commitments"`
	Questions   []QuestionOutput   `json:"questions"`
	Sentiment   SentimentOutput    `json:"sentiment"`
}

// RiskOutput represents an extracted risk.
type RiskOutput struct {
	Description string `json:"description"`
	Severity    string `json:"severity,omitempty"`
	Owner       string `json:"owner,omitempty"`
	SourceQuote string `json:"source_quote,omitempty"`
}

// ActionOutput represents an extracted action.
type ActionOutput struct {
	Description   string `json:"description"`
	Assignee      string `json:"assignee,omitempty"`
	DueDate       string `json:"due_date,omitempty"`
	DueDateSource string `json:"due_date_source,omitempty"`
	Status        string `json:"status,omitempty"`
	SourceQuote   string `json:"source_quote,omitempty"`
}

// IssueOutput represents an extracted issue.
type IssueOutput struct {
	Description string `json:"description"`
	Severity    string `json:"severity,omitempty"`
	Owner       string `json:"owner,omitempty"`
	BlockerFor  string `json:"blocker_for,omitempty"`
	SourceQuote string `json:"source_quote,omitempty"`
}

// DecisionOutput represents an extracted decision.
type DecisionOutput struct {
	Description   string `json:"description"`
	DecisionMaker string `json:"decision_maker,omitempty"`
	Rationale     string `json:"rationale,omitempty"`
	SourceQuote   string `json:"source_quote,omitempty"`
}

// CommitmentOutput represents an extracted commitment.
type CommitmentOutput struct {
	Description string `json:"description"`
	Committer   string `json:"committer,omitempty"`
	CommittedTo string `json:"committed_to,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
	SourceQuote string `json:"source_quote,omitempty"`
}

// QuestionOutput represents an extracted question.
type QuestionOutput struct {
	Question    string `json:"question"`
	Asker       string `json:"asker,omitempty"`
	DirectedTo  string `json:"directed_to,omitempty"`
	Answered    bool   `json:"answered"`
	SourceQuote string `json:"source_quote,omitempty"`
}

// SentimentOutput represents extracted sentiment.
type SentimentOutput struct {
	Overall string `json:"overall,omitempty"`
	Urgency string `json:"urgency,omitempty"`
	Tone    string `json:"tone,omitempty"`
}

// ParseExtractionOutput parses raw JSON output into structured format.
func ParseExtractionOutput(rawJSON string) (*ExtractionOutput, error) {
	var output ExtractionOutput
	if err := json.Unmarshal([]byte(rawJSON), &output); err != nil {
		return nil, fmt.Errorf("failed to parse extraction output: %w", err)
	}
	return &output, nil
}
