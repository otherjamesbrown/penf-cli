// Package learning provides correction tracking and learning capabilities for mention resolution.
package learning

import (
	"context"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/mentions/audit"
	"github.com/otherjamesbrown/penf-cli/pkg/mentions/resolver"
)

// Correction represents a human correction to a resolution decision.
type Correction struct {
	DecisionID        int64     `json:"decision_id"`
	TraceID           string    `json:"trace_id"`
	MentionText       string    `json:"mention_text"`
	OriginalChoice    string    `json:"original_choice"`
	CorrectedChoice   string    `json:"corrected_choice"`
	CorrectedEntityID *int64    `json:"corrected_entity_id,omitempty"`
	Notes             string    `json:"notes,omitempty"`
	CorrectedBy       string    `json:"corrected_by,omitempty"`
	CorrectedAt       time.Time `json:"corrected_at"`
	// Feature context for learning
	ContentType    string  `json:"content_type,omitempty"`
	Confidence     float32 `json:"confidence,omitempty"`
	ContextSnippet string  `json:"context_snippet,omitempty"`
}

// CorrectionRecorder handles recording and analyzing corrections.
type CorrectionRecorder struct {
	auditRepo audit.Repository
}

// NewCorrectionRecorder creates a new correction recorder.
func NewCorrectionRecorder(auditRepo audit.Repository) *CorrectionRecorder {
	return &CorrectionRecorder{
		auditRepo: auditRepo,
	}
}

// RecordCorrection records a human correction to a resolution decision.
func (r *CorrectionRecorder) RecordCorrection(ctx context.Context, correction Correction) error {
	// Get the original decision
	decision, err := r.auditRepo.GetDecision(ctx, correction.DecisionID)
	if err != nil {
		return err
	}

	// Mark as incorrect
	wasCorrect := false
	decision.WasCorrect = &wasCorrect
	decision.CorrectionNotes = correction.Notes

	return r.auditRepo.UpdateDecision(ctx, decision)
}

// CorrectionAnalysis provides analysis of corrections.
type CorrectionAnalysis struct {
	TotalCorrections   int                 `json:"total_corrections"`
	ByDecisionType     map[string]int      `json:"by_decision_type"`
	ByContentType      map[string]int      `json:"by_content_type"`
	CommonPatterns     []CorrectionPattern `json:"common_patterns"`
	LowConfidenceRate  float32             `json:"low_confidence_correction_rate"`
	HighConfidenceRate float32             `json:"high_confidence_correction_rate"`
}

// CorrectionPattern represents a common pattern in corrections.
type CorrectionPattern struct {
	MentionText     string `json:"mention_text"`
	Count           int    `json:"count"`
	TypicalError    string `json:"typical_error,omitempty"`
	SuggestedAction string `json:"suggested_action,omitempty"`
}

// AnalyzeCorrections analyzes correction patterns.
func (r *CorrectionRecorder) AnalyzeCorrections(ctx context.Context, tenantID string, daysSince int) (*CorrectionAnalysis, error) {
	since := time.Now().AddDate(0, 0, -daysSince)

	filter := audit.TraceFilter{
		TenantID: tenantID,
		Since:    &since,
		Limit:    1000,
	}

	corrections, err := r.auditRepo.GetCorrections(ctx, filter)
	if err != nil {
		return nil, err
	}

	analysis := &CorrectionAnalysis{
		TotalCorrections: len(corrections),
		ByDecisionType:   make(map[string]int),
		ByContentType:    make(map[string]int),
		CommonPatterns:   make([]CorrectionPattern, 0),
	}

	// Group by patterns
	mentionCounts := make(map[string]int)
	lowConfCount := 0
	highConfCount := 0

	for _, c := range corrections {
		analysis.ByDecisionType[string(c.DecisionType)]++
		mentionCounts[c.MentionedText]++

		if c.Confidence < 0.5 {
			lowConfCount++
		} else if c.Confidence > 0.8 {
			highConfCount++
		}
	}

	// Find common patterns (mentions corrected multiple times)
	for text, count := range mentionCounts {
		if count >= 2 {
			analysis.CommonPatterns = append(analysis.CommonPatterns, CorrectionPattern{
				MentionText: text,
				Count:       count,
			})
		}
	}

	// Calculate rates
	if analysis.TotalCorrections > 0 {
		analysis.LowConfidenceRate = float32(lowConfCount) / float32(analysis.TotalCorrections)
		analysis.HighConfidenceRate = float32(highConfCount) / float32(analysis.TotalCorrections)
	}

	return analysis, nil
}

// GetCorrectionsByType groups corrections by decision type.
func (r *CorrectionRecorder) GetCorrectionsByType(ctx context.Context, tenantID string, daysSince int) (map[resolver.DecisionType][]audit.Decision, error) {
	since := time.Now().AddDate(0, 0, -daysSince)

	filter := audit.TraceFilter{
		TenantID: tenantID,
		Since:    &since,
		Limit:    500,
	}

	corrections, err := r.auditRepo.GetCorrections(ctx, filter)
	if err != nil {
		return nil, err
	}

	result := make(map[resolver.DecisionType][]audit.Decision)
	for _, c := range corrections {
		result[c.DecisionType] = append(result[c.DecisionType], c)
	}

	return result, nil
}

// PatternSuggestion suggests pattern updates based on analysis.
type PatternSuggestion struct {
	MentionText       string `json:"mention_text"`
	SuggestedEntityID int64  `json:"suggested_entity_id"`
	SuggestedName     string `json:"suggested_name,omitempty"`
	Reason            string `json:"reason"`
	SupportingData    int    `json:"supporting_data"` // Number of corrections
	ConfidenceLevel   string `json:"confidence_level"` // high, medium, low
}

// AnalyzeForPatternUpdates analyzes corrections and suggests pattern updates.
func (r *CorrectionRecorder) AnalyzeForPatternUpdates(ctx context.Context, tenantID string, daysSince int) ([]PatternSuggestion, error) {
	since := time.Now().AddDate(0, 0, -daysSince)

	filter := audit.TraceFilter{
		TenantID: tenantID,
		Since:    &since,
		Limit:    500,
	}

	corrections, err := r.auditRepo.GetCorrections(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Group corrections by mention text
	type correctionGroup struct {
		entityCounts map[string]int
		totalCount   int
	}
	groups := make(map[string]*correctionGroup)

	for _, c := range corrections {
		key := c.MentionedText
		if groups[key] == nil {
			groups[key] = &correctionGroup{
				entityCounts: make(map[string]int),
			}
		}
		groups[key].entityCounts[c.ChosenOption]++
		groups[key].totalCount++
	}

	var suggestions []PatternSuggestion

	for mentionText, group := range groups {
		if group.totalCount < 2 {
			continue
		}

		// Find most common correction target
		var maxEntity string
		var maxCount int
		for entity, count := range group.entityCounts {
			if count > maxCount {
				maxEntity = entity
				maxCount = count
			}
		}

		// Calculate agreement rate
		agreement := float32(maxCount) / float32(group.totalCount)
		if agreement < 0.6 {
			continue
		}

		confidence := "low"
		if agreement >= 0.9 && group.totalCount >= 3 {
			confidence = "high"
		} else if agreement >= 0.75 && group.totalCount >= 2 {
			confidence = "medium"
		}

		suggestions = append(suggestions, PatternSuggestion{
			MentionText:     mentionText,
			SuggestedName:   maxEntity,
			Reason:          "Consistent human corrections",
			SupportingData:  maxCount,
			ConfidenceLevel: confidence,
		})
	}

	return suggestions, nil
}
