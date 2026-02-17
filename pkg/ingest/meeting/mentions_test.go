package meeting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractMentions_CanonicalName(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "Adam Weingarten", Aliases: []string{"AdamW"}},
		{ID: 2, CanonicalName: "Sara Weisman", Aliases: []string{"Sara"}},
	}

	text := "We need to talk to Adam Weingarten about the timeline."

	extractor := NewMentionExtractor(people)
	mentions := extractor.Extract(text)

	assert.Len(t, mentions, 1)
	assert.Equal(t, int64(1), mentions[0].PersonID)
	assert.Equal(t, "Adam Weingarten", mentions[0].MatchedText)
	assert.Equal(t, MentionMatchCanonical, mentions[0].MatchType)
	assert.Equal(t, 1, mentions[0].Count)
}

func TestExtractMentions_Alias(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "Hrishikesh Varma", Aliases: []string{"Rishi", "Rishi Varma"}},
	}

	text := "Rishi mentioned that the deadline is next week. I'll follow up with Rishi tomorrow."

	extractor := NewMentionExtractor(people)
	mentions := extractor.Extract(text)

	assert.Len(t, mentions, 1)
	assert.Equal(t, int64(1), mentions[0].PersonID)
	assert.Equal(t, "Rishi", mentions[0].MatchedText)
	assert.Equal(t, MentionMatchAlias, mentions[0].MatchType)
	assert.Equal(t, 2, mentions[0].Count) // Mentioned twice
}

func TestExtractMentions_MultiplePeople(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "Adam Weingarten", Aliases: []string{"AdamW", "Weingarten"}},
		{ID: 2, CanonicalName: "Sara Weisman", Aliases: []string{"Sara"}},
		{ID: 3, CanonicalName: "James Brown", Aliases: []string{"James"}},
	}

	text := "Sara said we should check with Weingarten. James agreed."

	extractor := NewMentionExtractor(people)
	mentions := extractor.Extract(text)

	assert.Len(t, mentions, 3)

	// Find each person's mention
	var sara, adam, james *Mention
	for i := range mentions {
		switch mentions[i].PersonID {
		case 1:
			adam = &mentions[i]
		case 2:
			sara = &mentions[i]
		case 3:
			james = &mentions[i]
		}
	}

	assert.NotNil(t, sara)
	assert.Equal(t, "Sara", sara.MatchedText)

	assert.NotNil(t, adam)
	assert.Equal(t, "Weingarten", adam.MatchedText)

	assert.NotNil(t, james)
	assert.Equal(t, "James", james.MatchedText)
}

func TestExtractMentions_CaseInsensitive(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "Adam Weingarten", Aliases: []string{"AdamW"}},
	}

	text := "ADAM WEINGARTEN will present. Then adam weingarten will answer questions."

	extractor := NewMentionExtractor(people)
	mentions := extractor.Extract(text)

	assert.Len(t, mentions, 1)
	assert.Equal(t, 2, mentions[0].Count)
}

func TestExtractMentions_WordBoundaries(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "Adam", Aliases: nil},
	}

	// "Adam" should match but "Adams" or "Adamant" should not
	text := "Adam joined. The Adams family. Adamant refusal."

	extractor := NewMentionExtractor(people)
	mentions := extractor.Extract(text)

	assert.Len(t, mentions, 1)
	assert.Equal(t, 1, mentions[0].Count) // Only "Adam" matches
}

func TestExtractMentions_ExcludeAttendees(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "Sara Weisman", Aliases: []string{"Sara"}},
		{ID: 2, CanonicalName: "Adam Weingarten", Aliases: []string{"Adam"}},
	}

	text := "Sara mentioned that we need to talk to Adam about the project."

	// Sara is an attendee, Adam is not
	attendeeIDs := map[int64]bool{1: true}

	extractor := NewMentionExtractor(people)
	mentions := extractor.ExtractExcluding(text, attendeeIDs)

	// Only Adam should be in mentions (Sara is excluded as attendee)
	assert.Len(t, mentions, 1)
	assert.Equal(t, int64(2), mentions[0].PersonID)
	assert.Equal(t, "Adam", mentions[0].MatchedText)
}

func TestExtractMentions_Context(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "Adam Weingarten", Aliases: nil},
	}

	text := "The project deadline is approaching. Adam Weingarten will handle the presentation. Everyone agreed."

	extractor := NewMentionExtractor(people)
	mentions := extractor.Extract(text)

	assert.Len(t, mentions, 1)
	// Context should include surrounding text
	assert.Contains(t, mentions[0].Context, "Adam Weingarten")
	assert.Contains(t, mentions[0].Context, "presentation")
}

func TestExtractMentions_NoMatches(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "Adam Weingarten", Aliases: nil},
	}

	text := "The project is on track. No blockers."

	extractor := NewMentionExtractor(people)
	mentions := extractor.Extract(text)

	assert.Len(t, mentions, 0)
}

func TestExtractMentions_EmptyText(t *testing.T) {
	people := []Person{
		{ID: 1, CanonicalName: "Adam Weingarten", Aliases: nil},
	}

	extractor := NewMentionExtractor(people)

	assert.Len(t, extractor.Extract(""), 0)
	assert.Len(t, extractor.Extract("   "), 0)
}
