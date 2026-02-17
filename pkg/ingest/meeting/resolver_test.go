package meeting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeName_StripPronouns(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Sara Weisman (she/her)", "Sara Weisman"},
		{"Mark Van Horn (he/him)", "Mark Van Horn"},
		{"Alex Johnson (they/them)", "Alex Johnson"},
		{"Pat Smith (she/they)", "Pat Smith"},
		{"James Brown", "James Brown"},
		{"  John Doe  ", "John Doe"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := NormalizeName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNormalizeName_HandlesEdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"   ", ""},
		{"(she/her)", ""},
		{"Name (with) parens", "Name (with) parens"}, // Non-pronoun parens preserved
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := NormalizeName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMatchPerson_ExactCanonicalMatch(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "James Brown", Aliases: nil},
		{ID: 2, CanonicalName: "Sara Weisman", Aliases: []string{"Sara"}},
		{ID: 3, CanonicalName: "Hrishikesh Varma", Aliases: []string{"Rishi", "Rishi Varma"}},
	}

	resolver := NewParticipantResolver(people)

	// Exact match
	match := resolver.Match("James Brown")
	assert.NotNil(t, match)
	assert.Equal(t, int64(1), match.PersonID)
	assert.Equal(t, MatchTypeExact, match.MatchType)
	assert.Equal(t, 1.0, match.Confidence)
}

func TestMatchPerson_AliasMatch(t *testing.T) {
	people := []Person{
		{ID: 2, CanonicalName: "Sara Weisman", Aliases: []string{"Sara"}},
		{ID: 3, CanonicalName: "Hrishikesh Varma", Aliases: []string{"Rishi", "Rishi Varma"}},
	}

	resolver := NewParticipantResolver(people)

	// Alias match - "Rishi Varma" matches Hrishikesh Varma
	match := resolver.Match("Rishi Varma")
	assert.NotNil(t, match)
	assert.Equal(t, int64(3), match.PersonID)
	assert.Equal(t, MatchTypeAlias, match.MatchType)
	assert.Equal(t, 1.0, match.Confidence)

	// Alias match - "Sara" matches Sara Weisman
	match = resolver.Match("Sara")
	assert.NotNil(t, match)
	assert.Equal(t, int64(2), match.PersonID)
	assert.Equal(t, MatchTypeAlias, match.MatchType)
}

func TestMatchPerson_NormalizedMatch(t *testing.T) {
	people := []Person{
		{ID: 2, CanonicalName: "Sara Weisman", Aliases: []string{"Sara"}},
	}

	resolver := NewParticipantResolver(people)

	// Match with pronouns stripped
	match := resolver.Match("Sara Weisman (she/her)")
	assert.NotNil(t, match)
	assert.Equal(t, int64(2), match.PersonID)
	assert.Equal(t, MatchTypeExact, match.MatchType)
}

func TestMatchPerson_NoMatch(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "James Brown", Aliases: nil},
	}

	resolver := NewParticipantResolver(people)

	match := resolver.Match("Unknown Person")
	assert.Nil(t, match)
}

func TestMatchPerson_CaseInsensitive(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "James Brown", Aliases: nil},
		{ID: 2, CanonicalName: "Sara Weisman", Aliases: []string{"sara"}},
	}

	resolver := NewParticipantResolver(people)

	// Case insensitive canonical match
	match := resolver.Match("james brown")
	assert.NotNil(t, match)
	assert.Equal(t, int64(1), match.PersonID)

	// Case insensitive alias match
	match = resolver.Match("SARA")
	assert.NotNil(t, match)
	assert.Equal(t, int64(2), match.PersonID)
}

func TestResolveParticipants_BatchResolve(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "James Brown", Aliases: nil},
		{ID: 2, CanonicalName: "Sara Weisman", Aliases: []string{"Sara"}},
		{ID: 3, CanonicalName: "Hrishikesh Varma", Aliases: []string{"Rishi", "Rishi Varma"}},
	}

	resolver := NewParticipantResolver(people)

	participants := []string{
		"Sara Weisman (she/her)",
		"Rishi Varma",
		"Unknown Person",
		"James Brown",
	}

	results := resolver.ResolveAll(participants)

	assert.Len(t, results, 4)

	// Sara matched
	assert.Equal(t, "Sara Weisman (she/her)", results[0].DisplayName)
	assert.NotNil(t, results[0].Match)
	assert.Equal(t, int64(2), results[0].Match.PersonID)

	// Rishi matched via alias
	assert.Equal(t, "Rishi Varma", results[1].DisplayName)
	assert.NotNil(t, results[1].Match)
	assert.Equal(t, int64(3), results[1].Match.PersonID)

	// Unknown not matched
	assert.Equal(t, "Unknown Person", results[2].DisplayName)
	assert.Nil(t, results[2].Match)

	// James matched
	assert.Equal(t, "James Brown", results[3].DisplayName)
	assert.NotNil(t, results[3].Match)
	assert.Equal(t, int64(1), results[3].Match.PersonID)
}

func TestResolveParticipants_Stats(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "James Brown", Aliases: nil},
		{ID: 2, CanonicalName: "Sara Weisman", Aliases: []string{"Sara"}},
	}

	resolver := NewParticipantResolver(people)

	participants := []string{
		"Sara Weisman (she/her)",
		"Unknown Person 1",
		"Unknown Person 2",
		"James Brown",
	}

	results := resolver.ResolveAll(participants)
	stats := results.Stats()

	assert.Equal(t, 4, stats.Total)
	assert.Equal(t, 2, stats.Matched)
	assert.Equal(t, 2, stats.Unmatched)
	assert.Equal(t, 0.5, stats.MatchRate)
}
