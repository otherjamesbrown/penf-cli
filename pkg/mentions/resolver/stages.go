package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"text/template"
)

// StageExecutor executes resolution stages.
type StageExecutor struct {
	provider LLMProvider
	config   Config
	prompts  *PromptTemplates
}

// NewStageExecutor creates a new stage executor.
func NewStageExecutor(provider LLMProvider, config Config) *StageExecutor {
	return &StageExecutor{
		provider: provider,
		config:   config,
		prompts:  DefaultPromptTemplates(),
	}
}

// ExecuteStage1 extracts and understands mentions from content.
func (e *StageExecutor) ExecuteStage1(ctx context.Context, batch ResolutionBatch, traceID string) (*Stage1Understanding, error) {
	// Build prompt
	promptData := struct {
		ContentText string
		ContentType string
		Date        string
		Metadata    *ContentMetadata
	}{
		ContentText: batch.ContentText,
		ContentType: batch.ContentType,
		Metadata:    batch.Metadata,
	}
	if batch.Metadata != nil && !batch.Metadata.Date.IsZero() {
		promptData.Date = batch.Metadata.Date.Format("2006-01-02")
	}

	prompt, err := e.renderTemplate("understanding", promptData)
	if err != nil {
		return nil, fmt.Errorf("render stage 1 prompt: %w", err)
	}

	var result Stage1Understanding
	req := CompletionRequest{
		SystemPrompt: understandingSystemPrompt,
		Prompt:       prompt,
		JSONMode:     true,
		TraceID:      traceID,
		StageNum:     1,
	}

	if err := e.provider.CompleteStructured(ctx, req, &result); err != nil {
		return nil, fmt.Errorf("stage 1 completion: %w", err)
	}

	return &result, nil
}

// ExecuteStage2 performs cross-mention reasoning.
func (e *StageExecutor) ExecuteStage2(ctx context.Context, understanding *Stage1Understanding, batch ResolutionBatch, traceID string) (*Stage2CrossMention, error) {
	promptData := struct {
		ContentID   int64
		Mentions    []MentionUnderstanding
		FullContent string
	}{
		ContentID:   batch.ContentID,
		Mentions:    understanding.Mentions,
		FullContent: batch.ContentText,
	}

	prompt, err := e.renderTemplate("cross_mention", promptData)
	if err != nil {
		return nil, fmt.Errorf("render stage 2 prompt: %w", err)
	}

	var result Stage2CrossMention
	req := CompletionRequest{
		SystemPrompt: crossMentionSystemPrompt,
		Prompt:       prompt,
		JSONMode:     true,
		TraceID:      traceID,
		StageNum:     2,
	}

	if err := e.provider.CompleteStructured(ctx, req, &result); err != nil {
		return nil, fmt.Errorf("stage 2 completion: %w", err)
	}

	result.ContentID = batch.ContentID
	return &result, nil
}

// ExecuteStage3 matches mentions to candidates.
func (e *StageExecutor) ExecuteStage3(
	ctx context.Context,
	understanding *Stage1Understanding,
	relationships *Stage2CrossMention,
	candidates map[string]*CandidateSet,
	traceID string,
) (*Stage3Matching, error) {
	promptData := struct {
		Understanding *Stage1Understanding
		Relationships *Stage2CrossMention
		Candidates    map[string]*CandidateSet
	}{
		Understanding: understanding,
		Relationships: relationships,
		Candidates:    candidates,
	}

	prompt, err := e.renderTemplate("matching", promptData)
	if err != nil {
		return nil, fmt.Errorf("render stage 3 prompt: %w", err)
	}

	req := CompletionRequest{
		SystemPrompt: matchingSystemPrompt,
		Prompt:       prompt,
		JSONMode:     true,
		TraceID:      traceID,
		StageNum:     3,
	}

	// Get raw completion response
	resp, err := e.provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("stage 3 completion: %w", err)
	}

	// Parse with fallback for name-to-ID mapping
	result, err := e.parseStage3WithFallback(resp.Content, candidates)
	if err != nil {
		return nil, fmt.Errorf("parse stage 3 result: %w", err)
	}

	return result, nil
}

// ExecuteStage4 verifies uncertain resolutions.
func (e *StageExecutor) ExecuteStage4(
	ctx context.Context,
	resolution Resolution,
	batch ResolutionBatch,
	traceID string,
) (*Stage4Verification, error) {
	promptData := struct {
		Resolution  Resolution
		FullContent string
		Challenge   string
	}{
		Resolution:  resolution,
		FullContent: batch.ContentText,
		Challenge:   fmt.Sprintf("You resolved '%s' to %s. Verify this is correct by looking for contradictory evidence.", resolution.MentionText, resolution.ResolvedTo.EntityName),
	}

	prompt, err := e.renderTemplate("verification", promptData)
	if err != nil {
		return nil, fmt.Errorf("render stage 4 prompt: %w", err)
	}

	var result Stage4Verification
	req := CompletionRequest{
		SystemPrompt: verificationSystemPrompt,
		Prompt:       prompt,
		JSONMode:     true,
		TraceID:      traceID,
		StageNum:     4,
	}

	if err := e.provider.CompleteStructured(ctx, req, &result); err != nil {
		return nil, fmt.Errorf("stage 4 completion: %w", err)
	}

	return &result, nil
}

// renderTemplate renders a prompt template.
func (e *StageExecutor) renderTemplate(name string, data interface{}) (string, error) {
	tmpl, ok := e.prompts.Templates[name]
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// PromptTemplates holds the prompt templates for each stage.
type PromptTemplates struct {
	Templates map[string]*template.Template
}

// DefaultPromptTemplates returns the default prompt templates.
func DefaultPromptTemplates() *PromptTemplates {
	templates := make(map[string]*template.Template)

	templates["understanding"] = template.Must(template.New("understanding").Parse(understandingPromptTemplate))
	templates["cross_mention"] = template.Must(template.New("cross_mention").Parse(crossMentionPromptTemplate))
	templates["matching"] = template.Must(template.New("matching").Parse(matchingPromptTemplate))
	templates["verification"] = template.Must(template.New("verification").Parse(verificationPromptTemplate))

	return &PromptTemplates{Templates: templates}
}

// System prompts
const understandingSystemPrompt = `You are an expert at extracting and understanding entity mentions from business content.
Your task is to identify mentions of persons, terms/acronyms, products, companies, and projects.
For each mention, provide your understanding of what the mention refers to based on context.
Flag any likely transcription errors (from speech-to-text) and suggest phonetic variants.
Output valid JSON only.`

const crossMentionSystemPrompt = `You are an expert at reasoning across multiple mentions in the same content.
Your task is to identify relationships between mentions and build a unified understanding.
Look for patterns like: same person mentioned by different names, terms that relate to products,
people who work on mentioned projects, transcription errors that match known terms.
Output valid JSON only.`

const matchingSystemPrompt = `You are an expert at matching entity mentions to database candidates.
Given the understanding from previous stages and candidate entities from the database,
make resolution decisions with confidence scores and clear reasoning.
For each mention, decide: resolve (match to candidate), queue_review (uncertain), or suggest_new_entity.
Include detailed reasoning and consider all alternatives.
Output valid JSON only.`

const verificationSystemPrompt = `You are an expert at verifying entity resolution decisions.
Your task is to challenge uncertain resolutions and look for contradictory evidence.
Verify consistency across the content and adjust confidence as needed.
Output valid JSON only.`

// Prompt templates
const understandingPromptTemplate = `Analyze the following content and identify all entity mentions.

Content Type: {{.ContentType}}
{{if .Date}}Date: {{.Date}}{{end}}
{{if .Metadata}}{{if .Metadata.Subject}}Subject: {{.Metadata.Subject}}{{end}}
{{if .Metadata.Participants}}Participants: {{range $i, $p := .Metadata.Participants}}{{if $i}}, {{end}}{{$p}}{{end}}{{end}}{{end}}

Content:
"""
{{.ContentText}}
"""

For each mention, provide:
{
  "mentions": [
    {
      "text": "exact text mentioned",
      "entity_type": "person|term|product|company|project",
      "position": character_offset,
      "context_snippet": "surrounding text for context",
      "understanding": "what you understand about this mention",
      "transcription_flags": {
        "likely_error": true|false,
        "phonetic_variants": ["variant1", "variant2"],
        "probable_correction": "corrected text",
        "confidence": 0.0-1.0
      }
    }
  ]
}`

const crossMentionPromptTemplate = `Analyze relationships between mentions from the same content.

Content ID: {{.ContentID}}

Mentions extracted:
{{range .Mentions}}
- "{{.Text}}" ({{.EntityType}}): {{.Understanding}}
{{end}}

Full Content:
"""
{{.FullContent}}
"""

Identify:
1. A unified understanding of what the content discusses
2. Relationships between mentions
3. Resolution hints based on relationships

Output:
{
  "content_id": {{.ContentID}},
  "unified_understanding": "summary of what the content discusses",
  "mention_relationships": [
    {
      "from_mention": "mention text",
      "to_mention": "related mention text",
      "relationship": "relationship type",
      "inference": "what this relationship implies for resolution"
    }
  ],
  "resolution_hints": ["hint1", "hint2"]
}`

const matchingPromptTemplate = `Match mentions to candidate entities.

Understanding:
{{range .Understanding.Mentions}}
- "{{.Text}}" ({{.EntityType}}): {{.Understanding}}
{{end}}

{{if .Relationships}}
Relationships:
{{range .Relationships.MentionRelationships}}
- {{.FromMention}} → {{.ToMention}}: {{.Relationship}}
{{end}}

Resolution Hints: {{range .Relationships.ResolutionHints}}
- {{.}}
{{end}}
{{end}}

Candidates:
{{range $text, $set := .Candidates}}
"{{$text}}":
{{range .Candidates}}
  - ID: {{.EntityID}}, Name: "{{.EntityName}}", Hints: {{.ConfidenceHints}}
{{end}}
{{end}}

IMPORTANT: When matching to a candidate, you MUST use the numeric ID value shown above.
For entity_id, return the INTEGER ID from the candidate list (e.g., 123), NOT the entity name (e.g., "John Smith").

For each mention, provide:
{
  "resolutions": [
    {
      "mention_text": "text",
      "mention_position": position,
      "decision": "resolve|queue_review|suggest_new_entity",
      "resolved_to": {"entity_type": "type", "entity_id": id, "entity_name": "name"},
      "confidence": 0.0-1.0,
      "reasoning": "detailed reasoning",
      "factors": {"factor_name": value},
      "alternatives_considered": [{"entity_id": id, "entity_name": "name", "confidence": 0.0, "rejection_reason": "why"}],
      "is_transcription_error": true|false
    }
  ],
  "new_entities_suggested": [
    {
      "mention_text": "text",
      "suggested_type": "entity_type",
      "suggested_name": "canonical name",
      "reasoning": "why this should be a new entity",
      "confidence": 0.0-1.0
    }
  ]
}`

const verificationPromptTemplate = `Verify the following resolution decision.

Resolution:
- Mention: "{{.Resolution.MentionText}}"
- Resolved to: {{.Resolution.ResolvedTo.EntityName}} ({{.Resolution.ResolvedTo.EntityType}})
- Confidence: {{.Resolution.Confidence}}
- Reasoning: {{.Resolution.Reasoning}}

Challenge: {{.Challenge}}

Full Content:
"""
{{.FullContent}}
"""

Verify by:
1. Looking for contradictory evidence
2. Checking consistency with other mentions
3. Evaluating if the reasoning is sound

Output:
{
  "mention_text": "{{.Resolution.MentionText}}",
  "original_confidence": {{.Resolution.Confidence}},
  "verification_result": "confirmed|adjusted|rejected",
  "adjusted_confidence": 0.0-1.0,
  "verification_notes": "detailed notes on verification"
}`

// parseStage3WithFallback attempts to parse Stage3Matching from JSON.
// If FlexInt64 parsing fails due to name strings, it retries with name-to-ID mapping.
func (e *StageExecutor) parseStage3WithFallback(content string, candidates map[string]*CandidateSet) (*Stage3Matching, error) {
	// Clean up the response - sometimes LLMs wrap JSON in markdown
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result Stage3Matching
	err := json.Unmarshal([]byte(content), &result)
	if err == nil {
		// Success on first try
		return &result, nil
	}

	// Check if it's a FlexInt64 parsing error
	if !strings.Contains(err.Error(), "FlexInt64") {
		return nil, err
	}

	// FlexInt64 error - likely entity_id is a name string
	// Try to fix the JSON by mapping names to IDs
	fixedContent, fixed := e.fixEntityIDNamesInJSON(content, candidates)
	if !fixed {
		return nil, fmt.Errorf("FlexInt64 error and fallback failed: %w", err)
	}

	// Retry parsing with fixed content
	if err := json.Unmarshal([]byte(fixedContent), &result); err != nil {
		return nil, fmt.Errorf("parse fixed JSON: %w", err)
	}

	return &result, nil
}

// fixEntityIDNamesInJSON finds entity_id fields with name strings and replaces them with numeric IDs.
// Returns the fixed JSON and whether any fixes were applied.
func (e *StageExecutor) fixEntityIDNamesInJSON(content string, candidates map[string]*CandidateSet) (string, bool) {
	// Build a name-to-ID mapping from candidates
	nameToID := make(map[string]int64)
	for _, candidateSet := range candidates {
		for _, candidate := range candidateSet.Candidates {
			nameToID[candidate.EntityName] = candidate.EntityID
		}
	}

	// Parse as generic JSON to manipulate
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return content, false
	}

	fixed := false

	// Fix resolutions
	if resolutions, ok := raw["resolutions"].([]interface{}); ok {
		for _, res := range resolutions {
			if resMap, ok := res.(map[string]interface{}); ok {
				if resolvedTo, ok := resMap["resolved_to"].(map[string]interface{}); ok {
					if entityIDVal, ok := resolvedTo["entity_id"]; ok {
						// Check if entity_id is a string (and not a numeric string)
						if entityIDStr, ok := entityIDVal.(string); ok {
							// Try to parse as int - if it fails, it's a name string
							if _, err := strconv.ParseInt(entityIDStr, 10, 64); err != nil {
								// It's a name string - look up the ID
								if id, found := nameToID[entityIDStr]; found {
									resolvedTo["entity_id"] = id
									fixed = true
									slog.Warn("LLM returned entity_id as name string, mapped to numeric ID",
										"entity_name", entityIDStr,
										"entity_id", id)
								}
							}
						}
					}
				}

				// Also fix alternatives_considered
				if alternatives, ok := resMap["alternatives_considered"].([]interface{}); ok {
					for _, alt := range alternatives {
						if altMap, ok := alt.(map[string]interface{}); ok {
							if entityIDVal, ok := altMap["entity_id"]; ok {
								if entityIDStr, ok := entityIDVal.(string); ok {
									if _, err := strconv.ParseInt(entityIDStr, 10, 64); err != nil {
										if id, found := nameToID[entityIDStr]; found {
											altMap["entity_id"] = id
											fixed = true
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if !fixed {
		return content, false
	}

	// Re-marshal the fixed JSON
	fixedBytes, err := json.Marshal(raw)
	if err != nil {
		return content, false
	}

	return string(fixedBytes), true
}
