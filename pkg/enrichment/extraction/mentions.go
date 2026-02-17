package extraction

import (
	"context"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
	"github.com/otherjamesbrown/penf-cli/pkg/enrichment/processors"
	"github.com/otherjamesbrown/penf-cli/pkg/mentions"
)

// MentionExtractor extracts mentions from AI-processed content and stores them.
type MentionExtractor struct {
	repo mentions.Repository
}

// NewMentionExtractor creates a new mention extractor.
func NewMentionExtractor(repo mentions.Repository) *MentionExtractor {
	return &MentionExtractor{
		repo: repo,
	}
}

// Name returns the processor name.
func (e *MentionExtractor) Name() string {
	return "MentionExtractor"
}

// Stage returns the pipeline stage.
func (e *MentionExtractor) Stage() processors.Stage {
	return processors.StagePostProcessing
}

// ShouldProcess returns true if post-processing should run for this enrichment.
func (e *MentionExtractor) ShouldProcess(enrich *enrichment.Enrichment) bool {
	// Only process if AI processing was done
	return enrich.AIProcessed
}

// Process extracts mentions from the enrichment and stores them.
func (e *MentionExtractor) Process(ctx context.Context, pctx *processors.ProcessorContext) error {
	enrich := pctx.Enrichment

	// Skip if no extracted data
	if enrich.ExtractedData == nil {
		return nil
	}

	// Extract mentions from various sources
	mentionInputs := e.extractMentions(enrich)

	if len(mentionInputs) == 0 {
		return nil
	}

	// Batch create mentions
	_, err := e.repo.BatchCreateMentions(ctx, mentionInputs)
	if err != nil {
		return fmt.Errorf("failed to create mentions: %w", err)
	}

	return nil
}

// extractMentions finds all mentions in the extracted data.
func (e *MentionExtractor) extractMentions(enrich *enrichment.Enrichment) []mentions.MentionInput {
	var inputs []mentions.MentionInput

	// Extract from extraction output if available
	if extractionData, ok := enrich.ExtractedData["extraction"]; ok {
		if output, ok := extractionData.(*ExtractionOutput); ok {
			inputs = append(inputs, e.extractFromExtractionOutput(enrich, output)...)
		} else if outputMap, ok := extractionData.(map[string]interface{}); ok {
			inputs = append(inputs, e.extractFromExtractionMap(enrich, outputMap)...)
		}
	}

	// Extract person mentions from resolved participants
	for _, p := range enrich.ResolvedParticipants {
		if p.Name != "" && p.PersonID == nil {
			// Unresolved participant - create mention
			inputs = append(inputs, mentions.MentionInput{
				ContentID:        enrich.SourceID,
				EntityType:       mentions.EntityTypePerson,
				MentionedText:    p.Name,
				ContextSnippet:   fmt.Sprintf("Email participant (%s)", p.Role),
				ProjectContextID: enrich.ProjectID,
			})
		}
	}

	return e.deduplicateMentions(inputs)
}

// extractFromExtractionOutput extracts mentions from typed extraction output.
func (e *MentionExtractor) extractFromExtractionOutput(enrich *enrichment.Enrichment, output *ExtractionOutput) []mentions.MentionInput {
	var inputs []mentions.MentionInput

	// Extract person mentions from assertions
	for _, r := range output.Risks {
		if r.Owner != "" {
			inputs = append(inputs, mentions.MentionInput{
				ContentID:        enrich.SourceID,
				EntityType:       mentions.EntityTypePerson,
				MentionedText:    r.Owner,
				ContextSnippet:   truncate(r.Description, 100),
				ProjectContextID: enrich.ProjectID,
			})
		}
	}

	for _, a := range output.Actions {
		if a.Assignee != "" {
			inputs = append(inputs, mentions.MentionInput{
				ContentID:        enrich.SourceID,
				EntityType:       mentions.EntityTypePerson,
				MentionedText:    a.Assignee,
				ContextSnippet:   truncate(a.Description, 100),
				ProjectContextID: enrich.ProjectID,
			})
		}
	}

	for _, i := range output.Issues {
		if i.Owner != "" {
			inputs = append(inputs, mentions.MentionInput{
				ContentID:        enrich.SourceID,
				EntityType:       mentions.EntityTypePerson,
				MentionedText:    i.Owner,
				ContextSnippet:   truncate(i.Description, 100),
				ProjectContextID: enrich.ProjectID,
			})
		}
	}

	for _, d := range output.Decisions {
		if d.DecisionMaker != "" {
			inputs = append(inputs, mentions.MentionInput{
				ContentID:        enrich.SourceID,
				EntityType:       mentions.EntityTypePerson,
				MentionedText:    d.DecisionMaker,
				ContextSnippet:   truncate(d.Description, 100),
				ProjectContextID: enrich.ProjectID,
			})
		}
	}

	for _, c := range output.Commitments {
		if c.Committer != "" {
			inputs = append(inputs, mentions.MentionInput{
				ContentID:        enrich.SourceID,
				EntityType:       mentions.EntityTypePerson,
				MentionedText:    c.Committer,
				ContextSnippet:   truncate(c.Description, 100),
				ProjectContextID: enrich.ProjectID,
			})
		}
		if c.CommittedTo != "" {
			inputs = append(inputs, mentions.MentionInput{
				ContentID:        enrich.SourceID,
				EntityType:       mentions.EntityTypePerson,
				MentionedText:    c.CommittedTo,
				ContextSnippet:   truncate(c.Description, 100),
				ProjectContextID: enrich.ProjectID,
			})
		}
	}

	for _, q := range output.Questions {
		if q.Asker != "" {
			inputs = append(inputs, mentions.MentionInput{
				ContentID:        enrich.SourceID,
				EntityType:       mentions.EntityTypePerson,
				MentionedText:    q.Asker,
				ContextSnippet:   truncate(q.Question, 100),
				ProjectContextID: enrich.ProjectID,
			})
		}
		if q.DirectedTo != "" {
			inputs = append(inputs, mentions.MentionInput{
				ContentID:        enrich.SourceID,
				EntityType:       mentions.EntityTypePerson,
				MentionedText:    q.DirectedTo,
				ContextSnippet:   truncate(q.Question, 100),
				ProjectContextID: enrich.ProjectID,
			})
		}
	}

	// Extract key entities if present
	if output.Sentiment.Overall != "" {
		// No direct entity extraction from sentiment currently
	}

	return inputs
}

// extractFromExtractionMap extracts mentions from untyped extraction map.
func (e *MentionExtractor) extractFromExtractionMap(enrich *enrichment.Enrichment, output map[string]interface{}) []mentions.MentionInput {
	var inputs []mentions.MentionInput

	// Helper to extract string field
	getString := func(m map[string]interface{}, key string) string {
		if v, ok := m[key].(string); ok {
			return v
		}
		return ""
	}

	// Helper to process an array of maps
	processArray := func(key string, personFields ...string) {
		if arr, ok := output[key].([]interface{}); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					description := getString(m, "description")
					if description == "" {
						description = getString(m, "question")
					}
					for _, field := range personFields {
						if name := getString(m, field); name != "" {
							inputs = append(inputs, mentions.MentionInput{
								ContentID:        enrich.SourceID,
								EntityType:       mentions.EntityTypePerson,
								MentionedText:    name,
								ContextSnippet:   truncate(description, 100),
								ProjectContextID: enrich.ProjectID,
							})
						}
					}
				}
			}
		}
	}

	processArray("risks", "owner")
	processArray("actions", "assignee")
	processArray("issues", "owner")
	processArray("decisions", "decision_maker", "decisionMaker")
	processArray("commitments", "committer", "committed_to", "committedTo")
	processArray("questions", "asker", "directed_to", "directedTo")

	return inputs
}

// deduplicateMentions removes duplicate mentions for the same content/text combo.
func (e *MentionExtractor) deduplicateMentions(inputs []mentions.MentionInput) []mentions.MentionInput {
	seen := make(map[string]bool)
	var result []mentions.MentionInput

	for _, input := range inputs {
		key := fmt.Sprintf("%d:%s:%s", input.ContentID, input.EntityType, strings.ToLower(input.MentionedText))
		if !seen[key] {
			seen[key] = true
			result = append(result, input)
		}
	}

	return result
}

// truncate shortens a string to the specified length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Verify interface compliance
var _ processors.PostProcessor = (*MentionExtractor)(nil)
var _ processors.Processor = (*MentionExtractor)(nil)
