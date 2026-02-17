package extraction

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
	"github.com/otherjamesbrown/penf-cli/pkg/enrichment/processors"
)

// LLMClient is the interface for LLM API clients.
type LLMClient interface {
	// Complete sends a prompt to the LLM and returns the response.
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}

// CompletionRequest represents a request to the LLM.
type CompletionRequest struct {
	Model       string                 `json:"model"`
	Prompt      string                 `json:"prompt"`
	MaxTokens   int                    `json:"max_tokens"`
	Temperature float64                `json:"temperature"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CompletionResponse represents a response from the LLM.
type CompletionResponse struct {
	Content      string `json:"content"`
	Model        string `json:"model"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	FinishReason string `json:"finish_reason"`
	LatencyMs    int    `json:"latency_ms"`
}

// ExtractionRun records a single extraction for audit purposes.
type ExtractionRun struct {
	ID               int64                  `json:"id,omitempty"`
	TenantID         string                 `json:"tenant_id"`
	SourceID         int64                  `json:"source_id"`
	ThreadID         *string                `json:"thread_id,omitempty"`
	TemplateID       *int64                 `json:"template_id,omitempty"`
	TemplateVersion  string                 `json:"template_version,omitempty"`
	ModelID          string                 `json:"model_id"`
	ModelVersion     string                 `json:"model_version,omitempty"`
	ContextInjected  map[string]interface{} `json:"context_injected,omitempty"`
	FullPrompt       string                 `json:"full_prompt"`
	InputTokens      int                    `json:"input_tokens"`
	OutputTokens     int                    `json:"output_tokens"`
	LatencyMs        int                    `json:"latency_ms"`
	RawResponse      string                 `json:"raw_response"`
	ParsedResponse   map[string]interface{} `json:"parsed_response,omitempty"`
	ParseErrors      []string               `json:"parse_errors,omitempty"`
	Status           string                 `json:"status"`
	ExperimentID     *int64                 `json:"experiment_id,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
}

// ExtractionRepository provides data access for extraction results.
type ExtractionRepository interface {
	// SaveExtractionRun persists an extraction run.
	SaveExtractionRun(ctx context.Context, run *ExtractionRun) error
	// SaveAssertions persists extracted assertions.
	SaveAssertions(ctx context.Context, assertions []Assertion) error
	// SaveSentiment persists content sentiment.
	SaveSentiment(ctx context.Context, sentiment *ContentSentiment) error
}

// Assertion represents an extracted assertion to be persisted.
type Assertion struct {
	TenantID              string     `json:"tenant_id"`
	SourceID              int64      `json:"source_id"`
	ThreadID              *int64     `json:"thread_id,omitempty"`
	ExtractionRunID       *int64     `json:"extraction_run_id,omitempty"`
	Type                  string     `json:"assertion_type"`
	Description           string     `json:"description"`
	SourceQuote           string     `json:"source_quote,omitempty"`
	Confidence            float64    `json:"confidence"`
	OwnerName             string     `json:"owner_name,omitempty"`
	AssigneeName          string     `json:"assignee_name,omitempty"`
	TargetName            string     `json:"target_name,omitempty"`
	DecisionMakerName     string     `json:"decision_maker_name,omitempty"`
	CommitterName         string     `json:"committer_name,omitempty"`
	CommittedToName       string     `json:"committed_to_name,omitempty"`
	ProjectID             *int64     `json:"project_id,omitempty"`
	TicketID              *int64     `json:"ticket_id,omitempty"`
	Severity              string     `json:"severity,omitempty"`
	Status                string     `json:"status,omitempty"`
	DueDate               *time.Time `json:"due_date,omitempty"`
	DueDateSource         string     `json:"due_date_source,omitempty"`
	Rationale             string     `json:"rationale,omitempty"`
	Answered              *bool      `json:"answered,omitempty"`
}

// ContentSentiment represents extracted sentiment to be persisted.
type ContentSentiment struct {
	SourceID            int64    `json:"source_id"`
	ExtractionRunID     *int64   `json:"extraction_run_id,omitempty"`
	OverallSentiment    string   `json:"overall_sentiment"`
	Urgency             string   `json:"urgency,omitempty"`
	ConfidenceInOutcome string   `json:"confidence_in_outcome,omitempty"`
	Tone                string   `json:"tone,omitempty"`
	Topics              []Topic  `json:"topics,omitempty"`
	KeyEntities         []Entity `json:"key_entities,omitempty"`
}

// Topic represents a detected topic.
type Topic struct {
	Topic     string  `json:"topic"`
	Relevance float64 `json:"relevance"`
}

// Entity represents a detected entity.
type Entity struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Context string `json:"context,omitempty"`
}

// LLMExtractorConfig configures the LLM extractor.
type LLMExtractorConfig struct {
	ModelID       string  `yaml:"model_id"`
	ModelVersion  string  `yaml:"model_version"`
	MaxTokens     int     `yaml:"max_tokens"`
	Temperature   float64 `yaml:"temperature"`
	TimeoutMs     int     `yaml:"timeout_ms"`
}

// DefaultLLMExtractorConfig returns default configuration.
func DefaultLLMExtractorConfig() LLMExtractorConfig {
	return LLMExtractorConfig{
		ModelID:      "gpt-4",
		ModelVersion: "gpt-4-turbo-preview",
		MaxTokens:    2048,
		Temperature:  0.1,
		TimeoutMs:    60000,
	}
}

// LLMExtractor performs AI extraction on enriched content.
type LLMExtractor struct {
	config           LLMExtractorConfig
	llmClient        LLMClient
	templateResolver *TemplateResolver
	contextBuilder   *ContextBuilder
	repo             ExtractionRepository
}

// NewLLMExtractor creates a new LLM extractor.
func NewLLMExtractor(
	llmClient LLMClient,
	templateResolver *TemplateResolver,
	contextBuilder *ContextBuilder,
	repo ExtractionRepository,
	opts ...LLMExtractorOption,
) *LLMExtractor {
	e := &LLMExtractor{
		config:           DefaultLLMExtractorConfig(),
		llmClient:        llmClient,
		templateResolver: templateResolver,
		contextBuilder:   contextBuilder,
		repo:             repo,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// LLMExtractorOption configures the LLM extractor.
type LLMExtractorOption func(*LLMExtractor)

// WithLLMConfig sets the LLM configuration.
func WithLLMConfig(config LLMExtractorConfig) LLMExtractorOption {
	return func(e *LLMExtractor) {
		e.config = config
	}
}

// Name returns the processor name.
func (e *LLMExtractor) Name() string {
	return "LLMExtractor"
}

// Stage returns the pipeline stage.
func (e *LLMExtractor) Stage() processors.Stage {
	return processors.StageAIProcessing
}

// ShouldProcess returns true if AI processing should run for this enrichment.
func (e *LLMExtractor) ShouldProcess(enrich *enrichment.Enrichment) bool {
	return !enrich.Classification.Profile.SkipAI()
}

// Process performs AI extraction on the enrichment.
func (e *LLMExtractor) Process(ctx context.Context, pctx *processors.ProcessorContext) error {
	enrich := pctx.Enrichment

	// Skip if profile indicates no AI processing
	if enrich.Classification.Profile.SkipAI() {
		enrich.AIProcessed = false
		enrich.AISkipReason = fmt.Sprintf("profile:%s", enrich.Classification.Profile)
		return nil
	}

	// Resolve template
	template, err := e.templateResolver.ResolveTemplate(ctx, enrich.TenantID, enrich.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to resolve template: %w", err)
	}

	// Build context
	contextTier := GetContextTier(enrich.Classification.Profile)
	extractionCtx, err := e.contextBuilder.Build(ctx, enrich, contextTier)
	if err != nil {
		// Context build failed, continue without context
		extractionCtx = nil
	}

	// Get content from source
	content := e.getContent(pctx.Source)

	// Render prompt
	promptData := PromptData{
		Content: content,
	}
	if extractionCtx != nil {
		promptData.Context = FormatContextForPrompt(extractionCtx)
	}

	prompt, err := RenderPrompt(template, promptData)
	if err != nil {
		return fmt.Errorf("failed to render prompt: %w", err)
	}

	// Call LLM
	startTime := time.Now()
	resp, err := e.llmClient.Complete(ctx, &CompletionRequest{
		Model:       e.config.ModelVersion,
		Prompt:      prompt,
		MaxTokens:   e.config.MaxTokens,
		Temperature: e.config.Temperature,
	})
	latencyMs := int(time.Since(startTime).Milliseconds())

	// Create extraction run for audit
	run := &ExtractionRun{
		TenantID:        enrich.TenantID,
		SourceID:        enrich.SourceID,
		TemplateVersion: template.Version,
		ModelID:         e.config.ModelID,
		ModelVersion:    e.config.ModelVersion,
		FullPrompt:      prompt,
		LatencyMs:       latencyMs,
		Status:          "completed",
		CreatedAt:       time.Now(),
	}
	if template.ID != 0 {
		run.TemplateID = &template.ID
	}
	if enrich.ThreadID != "" {
		run.ThreadID = &enrich.ThreadID
	}
	if extractionCtx != nil {
		run.ContextInjected = map[string]interface{}{
			"tokens_used":  extractionCtx.TokensUsed,
			"participants": len(extractionCtx.Participants),
			"has_project":  extractionCtx.Project != nil,
			"has_thread":   extractionCtx.Thread != nil,
		}
	}

	if err != nil {
		run.Status = "failed"
		run.ParseErrors = []string{err.Error()}
		e.saveExtractionRun(ctx, run)
		return fmt.Errorf("LLM call failed: %w", err)
	}

	run.InputTokens = resp.InputTokens
	run.OutputTokens = resp.OutputTokens
	run.RawResponse = resp.Content

	// Parse response
	output, parseErr := ParseExtractionOutput(resp.Content)
	if parseErr != nil {
		run.ParseErrors = []string{parseErr.Error()}
		run.Status = "partial"
	} else {
		parsed := make(map[string]interface{})
		json.Unmarshal([]byte(resp.Content), &parsed)
		run.ParsedResponse = parsed
	}

	// Save extraction run
	e.saveExtractionRun(ctx, run)

	// Convert and save assertions
	if output != nil && e.repo != nil {
		assertions := e.convertToAssertions(enrich, run, output)
		if len(assertions) > 0 {
			e.repo.SaveAssertions(ctx, assertions)
		}

		sentiment := e.convertToSentiment(enrich.SourceID, run, output)
		if sentiment != nil {
			e.repo.SaveSentiment(ctx, sentiment)
		}
	}

	// Update enrichment
	enrich.AIProcessed = true
	now := time.Now()
	enrich.AIProcessedAt = &now

	if output != nil {
		if enrich.ExtractedData == nil {
			enrich.ExtractedData = make(map[string]interface{})
		}
		enrich.ExtractedData["extraction"] = output
	}

	return nil
}

func (e *LLMExtractor) getContent(source *processors.Source) string {
	if source.Metadata != nil {
		if body, ok := source.Metadata["body_text"].(string); ok && body != "" {
			return body
		}
		if body, ok := source.Metadata["body"].(string); ok && body != "" {
			return body
		}
	}
	return source.RawContent
}

func (e *LLMExtractor) saveExtractionRun(ctx context.Context, run *ExtractionRun) {
	if e.repo != nil {
		e.repo.SaveExtractionRun(ctx, run)
	}
}

func (e *LLMExtractor) convertToAssertions(enrich *enrichment.Enrichment, run *ExtractionRun, output *ExtractionOutput) []Assertion {
	var assertions []Assertion

	// Convert risks
	for _, r := range output.Risks {
		assertions = append(assertions, Assertion{
			TenantID:        enrich.TenantID,
			SourceID:        enrich.SourceID,
			ExtractionRunID: &run.ID,
			Type:            "risk",
			Description:     r.Description,
			SourceQuote:     r.SourceQuote,
			Confidence:      0.8,
			OwnerName:       r.Owner,
			Severity:        r.Severity,
			ProjectID:       enrich.ProjectID,
		})
	}

	// Convert actions
	for _, a := range output.Actions {
		assertions = append(assertions, Assertion{
			TenantID:        enrich.TenantID,
			SourceID:        enrich.SourceID,
			ExtractionRunID: &run.ID,
			Type:            "action",
			Description:     a.Description,
			SourceQuote:     a.SourceQuote,
			Confidence:      0.8,
			AssigneeName:    a.Assignee,
			Status:          a.Status,
			DueDateSource:   a.DueDateSource,
			ProjectID:       enrich.ProjectID,
		})
	}

	// Convert issues
	for _, i := range output.Issues {
		assertions = append(assertions, Assertion{
			TenantID:        enrich.TenantID,
			SourceID:        enrich.SourceID,
			ExtractionRunID: &run.ID,
			Type:            "issue",
			Description:     i.Description,
			SourceQuote:     i.SourceQuote,
			Confidence:      0.8,
			OwnerName:       i.Owner,
			Severity:        i.Severity,
			ProjectID:       enrich.ProjectID,
		})
	}

	// Convert decisions
	for _, d := range output.Decisions {
		assertions = append(assertions, Assertion{
			TenantID:          enrich.TenantID,
			SourceID:          enrich.SourceID,
			ExtractionRunID:   &run.ID,
			Type:              "decision",
			Description:       d.Description,
			SourceQuote:       d.SourceQuote,
			Confidence:        0.8,
			DecisionMakerName: d.DecisionMaker,
			Rationale:         d.Rationale,
			ProjectID:         enrich.ProjectID,
		})
	}

	// Convert commitments
	for _, c := range output.Commitments {
		assertions = append(assertions, Assertion{
			TenantID:        enrich.TenantID,
			SourceID:        enrich.SourceID,
			ExtractionRunID: &run.ID,
			Type:            "commitment",
			Description:     c.Description,
			SourceQuote:     c.SourceQuote,
			Confidence:      0.8,
			CommitterName:   c.Committer,
			CommittedToName: c.CommittedTo,
			ProjectID:       enrich.ProjectID,
		})
	}

	// Convert questions
	for _, q := range output.Questions {
		answered := q.Answered
		assertions = append(assertions, Assertion{
			TenantID:        enrich.TenantID,
			SourceID:        enrich.SourceID,
			ExtractionRunID: &run.ID,
			Type:            "question",
			Description:     q.Question,
			SourceQuote:     q.SourceQuote,
			Confidence:      0.8,
			OwnerName:       q.Asker,
			TargetName:      q.DirectedTo,
			Answered:        &answered,
			ProjectID:       enrich.ProjectID,
		})
	}

	return assertions
}

func (e *LLMExtractor) convertToSentiment(sourceID int64, run *ExtractionRun, output *ExtractionOutput) *ContentSentiment {
	if output.Sentiment.Overall == "" {
		return nil
	}

	return &ContentSentiment{
		SourceID:         sourceID,
		ExtractionRunID:  &run.ID,
		OverallSentiment: output.Sentiment.Overall,
		Urgency:          output.Sentiment.Urgency,
		Tone:             output.Sentiment.Tone,
	}
}

// Verify interface compliance
var _ processors.AIProcessor = (*LLMExtractor)(nil)
var _ processors.Processor = (*LLMExtractor)(nil)
