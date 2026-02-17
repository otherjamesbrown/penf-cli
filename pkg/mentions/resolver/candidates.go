package resolver

import (
	"context"
	"sort"
	"strings"

	"github.com/otherjamesbrown/penf-cli/pkg/mentions"
)

// CandidateGatherer gathers candidate entities for mention matching.
type CandidateGatherer struct {
	lookup   mentions.EntityLookup
	mentRepo mentions.Repository
	config   Config
}

// NewCandidateGatherer creates a new candidate gatherer.
func NewCandidateGatherer(lookup mentions.EntityLookup, mentRepo mentions.Repository, config Config) *CandidateGatherer {
	return &CandidateGatherer{
		lookup:   lookup,
		mentRepo: mentRepo,
		config:   config,
	}
}

// GatherCandidates collects candidates for all mentions based on understanding.
func (g *CandidateGatherer) GatherCandidates(
	ctx context.Context,
	tenantID string,
	understanding *Stage1Understanding,
	relationships *Stage2CrossMention,
	projectID *int64,
) (map[string]*CandidateSet, error) {
	results := make(map[string]*CandidateSet)

	for _, mention := range understanding.Mentions {
		candidates, err := g.gatherForMention(ctx, tenantID, mention, projectID)
		if err != nil {
			return nil, err
		}

		results[mention.Text] = &CandidateSet{
			MentionText: mention.Text,
			Candidates:  candidates,
		}
	}

	// Apply cross-mention hints to boost candidates
	g.applyCrossMentionHints(results, relationships)

	return results, nil
}

// gatherForMention collects candidates for a single mention.
func (g *CandidateGatherer) gatherForMention(
	ctx context.Context,
	tenantID string,
	mention MentionUnderstanding,
	projectID *int64,
) ([]CandidateWithHints, error) {
	var candidates []CandidateWithHints

	// If EntityLookup is nil, return empty candidates (no matching possible)
	if g.lookup == nil {
		return candidates, nil
	}

	// Look up candidates based on entity type
	var baseCandidates []mentions.Candidate
	var err error

	switch mention.EntityType {
	case mentions.EntityTypePerson:
		baseCandidates, err = g.lookup.LookupPerson(ctx, tenantID, mention.Text)
	case mentions.EntityTypeTerm:
		baseCandidates, err = g.lookup.LookupTerm(ctx, tenantID, mention.Text)
	case mentions.EntityTypeProduct:
		baseCandidates, err = g.lookup.LookupProduct(ctx, tenantID, mention.Text)
	case mentions.EntityTypeCompany:
		baseCandidates, err = g.lookup.LookupCompany(ctx, tenantID, mention.Text)
	case mentions.EntityTypeProject:
		baseCandidates, err = g.lookup.LookupProject(ctx, tenantID, mention.Text)
	}

	if err != nil {
		return nil, err
	}

	// Convert to CandidateWithHints and gather additional hints
	for _, c := range baseCandidates {
		hints := make(map[string]interface{})
		hints["fuzzy_match"] = c.Confidence
		hints["prior_links"] = c.PriorLinks

		// Check for project membership/affinity
		if projectID != nil {
			affinity, err := g.mentRepo.GetAffinity(ctx, tenantID, mention.EntityType, c.EntityID, *projectID)
			if err == nil && affinity != nil {
				hints["project_member"] = affinity.IsMember
				hints["affinity_score"] = affinity.AffinityScore
				hints["project_mention_count"] = affinity.MentionCount
			}
		}

		// Check for patterns
		patterns, err := g.mentRepo.GetPatternsByText(ctx, tenantID, mention.EntityType, mention.Text)
		if err == nil && len(patterns) > 0 {
			for _, p := range patterns {
				if p.ResolvedEntityID != nil && *p.ResolvedEntityID == c.EntityID {
					hints["pattern_match"] = true
					hints["pattern_times_linked"] = p.TimesLinked
					hints["pattern_is_permanent"] = p.IsPermanent
					break
				}
			}
		}

		candidates = append(candidates, CandidateWithHints{
			EntityID:        c.EntityID,
			EntityType:      mention.EntityType,
			EntityName:      c.EntityName,
			ConfidenceHints: hints,
		})
	}

	// Also check for phonetic variants if transcription error suspected
	if mention.TranscriptionFlags != nil && mention.TranscriptionFlags.LikelyError {
		for _, variant := range mention.TranscriptionFlags.PhoneticVariants {
			variantCandidates, err := g.lookupByType(ctx, tenantID, mention.EntityType, variant)
			if err != nil {
				continue
			}
			for _, vc := range variantCandidates {
				// Don't duplicate
				found := false
				for _, existing := range candidates {
					if existing.EntityID == vc.EntityID {
						found = true
						break
					}
				}
				if !found {
					hints := make(map[string]interface{})
					hints["phonetic_variant"] = variant
					hints["fuzzy_match"] = vc.Confidence
					candidates = append(candidates, CandidateWithHints{
						EntityID:        vc.EntityID,
						EntityType:      mention.EntityType,
						EntityName:      vc.EntityName,
						ConfidenceHints: hints,
					})
				}
			}
		}
	}

	// Sort by confidence hints
	sort.Slice(candidates, func(i, j int) bool {
		scoreI := g.scoreCandidate(candidates[i])
		scoreJ := g.scoreCandidate(candidates[j])
		return scoreI > scoreJ
	})

	return candidates, nil
}

// lookupByType performs lookup based on entity type.
func (g *CandidateGatherer) lookupByType(
	ctx context.Context,
	tenantID string,
	entityType mentions.EntityType,
	text string,
) ([]mentions.Candidate, error) {
	switch entityType {
	case mentions.EntityTypePerson:
		return g.lookup.LookupPerson(ctx, tenantID, text)
	case mentions.EntityTypeTerm:
		return g.lookup.LookupTerm(ctx, tenantID, text)
	case mentions.EntityTypeProduct:
		return g.lookup.LookupProduct(ctx, tenantID, text)
	case mentions.EntityTypeCompany:
		return g.lookup.LookupCompany(ctx, tenantID, text)
	case mentions.EntityTypeProject:
		return g.lookup.LookupProject(ctx, tenantID, text)
	default:
		return nil, nil
	}
}

// scoreCandidate computes a preliminary score for sorting.
func (g *CandidateGatherer) scoreCandidate(c CandidateWithHints) float64 {
	score := 0.0

	if fuzzy, ok := c.ConfidenceHints["fuzzy_match"].(float32); ok {
		score += float64(fuzzy)
	}
	if member, ok := c.ConfidenceHints["project_member"].(bool); ok && member {
		score += 0.3
	}
	if affinity, ok := c.ConfidenceHints["affinity_score"].(float32); ok {
		score += float64(affinity) * 0.2
	}
	if priorLinks, ok := c.ConfidenceHints["prior_links"].(int); ok {
		score += float64(priorLinks) * 0.05
	}
	if patternMatch, ok := c.ConfidenceHints["pattern_match"].(bool); ok && patternMatch {
		score += 0.2
	}

	return score
}

// applyCrossMentionHints applies hints from cross-mention reasoning.
func (g *CandidateGatherer) applyCrossMentionHints(
	results map[string]*CandidateSet,
	relationships *Stage2CrossMention,
) {
	if relationships == nil {
		return
	}

	for _, rel := range relationships.MentionRelationships {
		// If two mentions are related, boost candidates that share context
		fromSet, fromOK := results[rel.FromMention]
		toSet, toOK := results[rel.ToMention]

		if !fromOK || !toOK {
			continue
		}

		// Apply relationship inference
		for i := range fromSet.Candidates {
			fromSet.Candidates[i].ConfidenceHints["cross_mention_relationship"] = rel.Relationship
			fromSet.Candidates[i].ConfidenceHints["cross_mention_inference"] = rel.Inference
		}

		// If relationship is "transcription_of", link candidates
		if strings.Contains(rel.Relationship, "transcription") {
			for _, toCandidate := range toSet.Candidates {
				// Add to_candidate to from_set if not present
				found := false
				for _, fc := range fromSet.Candidates {
					if fc.EntityID == toCandidate.EntityID {
						found = true
						fc.ConfidenceHints["transcription_linked_to"] = rel.ToMention
						break
					}
				}
				if !found {
					newCandidate := CandidateWithHints{
						EntityID:        toCandidate.EntityID,
						EntityType:      toCandidate.EntityType,
						EntityName:      toCandidate.EntityName,
						ConfidenceHints: make(map[string]interface{}),
					}
					newCandidate.ConfidenceHints["transcription_linked_to"] = rel.ToMention
					newCandidate.ConfidenceHints["fuzzy_match"] = 0.8 // High confidence for transcription link
					fromSet.Candidates = append(fromSet.Candidates, newCandidate)
				}
			}
		}
	}
}
