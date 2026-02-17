package meeting

import (
	"regexp"
	"strings"
)

// MentionMatchType indicates how a mention was matched.
type MentionMatchType string

const (
	MentionMatchCanonical MentionMatchType = "canonical"
	MentionMatchAlias     MentionMatchType = "alias"
)

// Mention represents a person mentioned in text.
type Mention struct {
	PersonID      int64
	CanonicalName string
	MatchedText   string           // The actual text that matched
	MatchType     MentionMatchType // How it was matched
	Count         int              // Number of times mentioned
	Context       string           // Surrounding text snippet
}

// MentionExtractor extracts mentions of people from text.
type MentionExtractor struct {
	people []Person
	// Compiled patterns for each person
	patterns []personPattern
}

type personPattern struct {
	person    *Person
	canonical *regexp.Regexp
	aliases   []*regexp.Regexp
}

// NewMentionExtractor creates a new mention extractor for the given people.
func NewMentionExtractor(people []Person) *MentionExtractor {
	e := &MentionExtractor{
		people:   people,
		patterns: make([]personPattern, len(people)),
	}

	for i := range people {
		p := &people[i]
		pattern := personPattern{person: p}

		// Compile canonical name pattern (word boundary, case insensitive)
		pattern.canonical = compileNamePattern(p.CanonicalName)

		// Compile alias patterns
		pattern.aliases = make([]*regexp.Regexp, len(p.Aliases))
		for j, alias := range p.Aliases {
			pattern.aliases[j] = compileNamePattern(alias)
		}

		e.patterns[i] = pattern
	}

	return e
}

// compileNamePattern creates a case-insensitive word-boundary regex for a name.
func compileNamePattern(name string) *regexp.Regexp {
	// Escape special regex characters in the name
	escaped := regexp.QuoteMeta(name)
	// Create pattern with word boundaries
	pattern := `(?i)\b` + escaped + `\b`
	return regexp.MustCompile(pattern)
}

// Extract finds all mentions of known people in the text.
func (e *MentionExtractor) Extract(text string) []Mention {
	return e.ExtractExcluding(text, nil)
}

// ExtractExcluding finds mentions excluding specified person IDs (e.g., attendees).
func (e *MentionExtractor) ExtractExcluding(text string, excludeIDs map[int64]bool) []Mention {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	var mentions []Mention

	for _, pattern := range e.patterns {
		// Skip excluded people
		if excludeIDs != nil && excludeIDs[pattern.person.ID] {
			continue
		}

		// Try canonical name first
		matches := pattern.canonical.FindAllStringIndex(text, -1)
		if len(matches) > 0 {
			matchedText := text[matches[0][0]:matches[0][1]]
			mentions = append(mentions, Mention{
				PersonID:      pattern.person.ID,
				CanonicalName: pattern.person.CanonicalName,
				MatchedText:   matchedText,
				MatchType:     MentionMatchCanonical,
				Count:         len(matches),
				Context:       extractContext(text, matches[0][0], matches[0][1]),
			})
			continue
		}

		// Try aliases
		for _, aliasPattern := range pattern.aliases {
			matches := aliasPattern.FindAllStringIndex(text, -1)
			if len(matches) > 0 {
				matchedText := text[matches[0][0]:matches[0][1]]
				mentions = append(mentions, Mention{
					PersonID:      pattern.person.ID,
					CanonicalName: pattern.person.CanonicalName,
					MatchedText:   matchedText,
					MatchType:     MentionMatchAlias,
					Count:         len(matches),
					Context:       extractContext(text, matches[0][0], matches[0][1]),
				})
				break // Found a match via this alias, don't check other aliases
			}
		}
	}

	return mentions
}

// extractContext returns a snippet of text around the match position.
func extractContext(text string, start, end int) string {
	const contextRadius = 50

	contextStart := start - contextRadius
	if contextStart < 0 {
		contextStart = 0
	}

	contextEnd := end + contextRadius
	if contextEnd > len(text) {
		contextEnd = len(text)
	}

	// Try to start/end at word boundaries
	for contextStart > 0 && text[contextStart] != ' ' && text[contextStart] != '\n' {
		contextStart--
	}
	for contextEnd < len(text) && text[contextEnd] != ' ' && text[contextEnd] != '\n' {
		contextEnd++
	}

	context := strings.TrimSpace(text[contextStart:contextEnd])

	// Add ellipsis if truncated
	if contextStart > 0 {
		context = "..." + context
	}
	if contextEnd < len(text) {
		context = context + "..."
	}

	return context
}
