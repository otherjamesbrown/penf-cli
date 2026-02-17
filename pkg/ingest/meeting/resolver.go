package meeting

import (
	"regexp"
	"strings"
)

// MatchType indicates how a participant was matched to a person.
type MatchType string

const (
	MatchTypeExact MatchType = "exact"
	MatchTypeAlias MatchType = "alias"
	MatchTypeFuzzy MatchType = "fuzzy"
)

// Person represents a person from the people table.
type Person struct {
	ID            int64
	CanonicalName string
	Aliases       []string
}

// PersonMatch represents a successful match between a participant name and a person.
type PersonMatch struct {
	PersonID      int64
	CanonicalName string
	MatchType     MatchType
	Confidence    float64
}

// ParticipantResult represents the resolution result for a single participant.
type ParticipantResult struct {
	DisplayName string
	Match       *PersonMatch
}

// ParticipantResults is a slice of resolution results with helper methods.
type ParticipantResults []ParticipantResult

// Stats returns statistics about the resolution results.
func (r ParticipantResults) Stats() ResolveStats {
	matched := 0
	for _, result := range r {
		if result.Match != nil {
			matched++
		}
	}
	total := len(r)
	matchRate := 0.0
	if total > 0 {
		matchRate = float64(matched) / float64(total)
	}
	return ResolveStats{
		Total:     total,
		Matched:   matched,
		Unmatched: total - matched,
		MatchRate: matchRate,
	}
}

// ResolveStats contains statistics about participant resolution.
type ResolveStats struct {
	Total     int
	Matched   int
	Unmatched int
	MatchRate float64
}

// ParticipantResolver matches participant names to people.
type ParticipantResolver struct {
	// Index by normalized lowercase name for fast lookup
	canonicalIndex map[string]*Person
	aliasIndex     map[string]*Person
}

// Pronoun patterns to strip from names
var pronounPatterns = regexp.MustCompile(`\s*\((?:she|he|they)(?:/(?:her|him|them|they|she|he))*\)\s*$`)

// NormalizeName cleans up a participant name by stripping pronouns and trimming whitespace.
func NormalizeName(name string) string {
	// Strip pronouns like (she/her), (he/him), (they/them), (she/they)
	name = pronounPatterns.ReplaceAllString(name, "")
	// Trim whitespace
	name = strings.TrimSpace(name)
	return name
}

// NewParticipantResolver creates a new resolver with the given people.
func NewParticipantResolver(people []Person) *ParticipantResolver {
	r := &ParticipantResolver{
		canonicalIndex: make(map[string]*Person),
		aliasIndex:     make(map[string]*Person),
	}

	for i := range people {
		p := &people[i]
		// Index by normalized lowercase canonical name
		key := strings.ToLower(NormalizeName(p.CanonicalName))
		r.canonicalIndex[key] = p

		// Index all aliases
		for _, alias := range p.Aliases {
			aliasKey := strings.ToLower(NormalizeName(alias))
			r.aliasIndex[aliasKey] = p
		}
	}

	return r
}

// Match attempts to match a participant name to a person.
// Returns nil if no match found.
func (r *ParticipantResolver) Match(participantName string) *PersonMatch {
	// Normalize the input name
	normalized := NormalizeName(participantName)
	key := strings.ToLower(normalized)

	// Try exact canonical match first
	if person, ok := r.canonicalIndex[key]; ok {
		return &PersonMatch{
			PersonID:      person.ID,
			CanonicalName: person.CanonicalName,
			MatchType:     MatchTypeExact,
			Confidence:    1.0,
		}
	}

	// Try alias match
	if person, ok := r.aliasIndex[key]; ok {
		return &PersonMatch{
			PersonID:      person.ID,
			CanonicalName: person.CanonicalName,
			MatchType:     MatchTypeAlias,
			Confidence:    1.0,
		}
	}

	// No match found
	return nil
}

// ResolveAll resolves a list of participant names.
func (r *ParticipantResolver) ResolveAll(participants []string) ParticipantResults {
	results := make(ParticipantResults, len(participants))
	for i, name := range participants {
		results[i] = ParticipantResult{
			DisplayName: name,
			Match:       r.Match(name),
		}
	}
	return results
}
