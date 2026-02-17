package meeting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAcronymDetector_Detect(t *testing.T) {
	detector := NewAcronymDetector()

	tests := []struct {
		name     string
		text     string
		expected []string // expected terms found
	}{
		{
			name:     "simple acronym",
			text:     "We discussed the TER meeting today",
			expected: []string{"TER"},
		},
		{
			name:     "multiple acronyms",
			text:     "The TER and DBaaS teams met to discuss SLAs",
			expected: []string{"TER", "DBAAS", "SLAS"},
		},
		{
			name:     "mixed case acronym",
			text:     "DBaaS is the database team",
			expected: []string{"DBAAS"},
		},
		{
			name:     "skip common words",
			text:     "I am OK with this API",
			expected: []string{}, // API is in common words
		},
		{
			name:     "regular words should be skipped",
			text:     "Hello World from John",
			expected: []string{},
		},
		{
			name:     "minimum length enforced",
			text:     "A B C should be skipped but ABC should not",
			expected: []string{"ABC"},
		},
		{
			name:     "acronym with numbers",
			text:     "The S3 bucket and EC2 instance",
			expected: []string{"EC2"},
		},
		{
			name:     "multiple occurrences counted",
			text:     "TER is great. I love TER. TER is the best.",
			expected: []string{"TER"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := detector.Detect(tc.text)

			// Extract just the terms for comparison
			var foundTerms []string
			for _, r := range results {
				foundTerms = append(foundTerms, r.Term)
			}

			// Check that expected terms are found
			for _, exp := range tc.expected {
				assert.Contains(t, foundTerms, exp, "expected to find %s", exp)
			}

			// Check that we don't have extra unexpected terms
			assert.Len(t, foundTerms, len(tc.expected), "unexpected number of terms found")
		})
	}
}

func TestAcronymDetector_SetKnownTerms(t *testing.T) {
	detector := NewAcronymDetector()
	detector.SetKnownTerms([]string{"TER", "DBaaS"})

	results := detector.Detect("TER and DBaaS and MTC teams")

	assert.Len(t, results, 1)
	assert.Equal(t, "MTC", results[0].Term)
}

func TestAcronymDetector_AddKnownTerm(t *testing.T) {
	detector := NewAcronymDetector()
	detector.AddKnownTerm("TER")

	results := detector.Detect("TER and MTC teams")

	assert.Len(t, results, 1)
	assert.Equal(t, "MTC", results[0].Term)
}

func TestAcronymDetector_Context(t *testing.T) {
	detector := NewAcronymDetector()

	text := "The quick brown fox jumps over the lazy TER dog"
	results := detector.Detect(text)

	assert.Len(t, results, 1)
	assert.Equal(t, "TER", results[0].Term)
	assert.Contains(t, results[0].Context, "TER")
	assert.Contains(t, results[0].Context, "lazy")
}

func TestAcronymDetector_Count(t *testing.T) {
	detector := NewAcronymDetector()

	text := "TER is great. TER is the best. TER forever."
	results := detector.Detect(text)

	assert.Len(t, results, 1)
	assert.Equal(t, "TER", results[0].Term)
	assert.Equal(t, 3, results[0].Count)
}

func TestAcronymDetector_DetectInTranscript(t *testing.T) {
	detector := NewAcronymDetector()

	transcript := &TranscriptResult{
		FullText: "TER meeting today. MTC is mentioned once.",
	}

	// With min occurrences = 1
	results := detector.DetectInTranscript(transcript, 1)
	assert.Len(t, results, 2)

	// With min occurrences = 2 (only TER appears once)
	transcript.FullText = "TER TER MTC today"
	results = detector.DetectInTranscript(transcript, 2)
	assert.Len(t, results, 1)
	assert.Equal(t, "TER", results[0].Term)
}

func TestAcronymDetector_NilTranscript(t *testing.T) {
	detector := NewAcronymDetector()

	results := detector.DetectInTranscript(nil, 1)
	assert.Nil(t, results)

	results = detector.DetectInTranscript(&TranscriptResult{}, 1)
	assert.Nil(t, results)
}

func TestIsRegularWord(t *testing.T) {
	tests := []struct {
		word     string
		expected bool
	}{
		{"Hello", true},
		{"World", true},
		{"TER", false},
		{"DBaaS", false},
		{"PostgreSQL", false},
		{"AWS", false},
		{"hi", false}, // all lowercase
		{"HI", false}, // all uppercase (acronym)
	}

	for _, tc := range tests {
		t.Run(tc.word, func(t *testing.T) {
			assert.Equal(t, tc.expected, isRegularWord(tc.word))
		})
	}
}

func TestCleanContext(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes extra whitespace",
			input:    "hello   world    test",
			expected: "hello world test",
		},
		{
			name:     "truncates long context",
			input:    "this is a very long context that should be truncated because it exceeds the maximum length allowed for context which is one hundred and fifty characters total",
			expected: "this is a very long context that should be truncated because it exceeds the maximum length allowed for context which is one hundred and fifty chara...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := cleanContext(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
