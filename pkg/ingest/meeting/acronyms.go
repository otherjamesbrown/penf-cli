package meeting

import (
	"regexp"
	"strings"
	"unicode"
)

// AcronymDetector detects potential acronyms in text.
type AcronymDetector struct {
	// KnownTerms contains terms to skip (already defined in glossary)
	KnownTerms map[string]bool

	// CommonWords contains common words/abbreviations to ignore
	CommonWords map[string]bool

	// MinLength is the minimum length of an acronym (default: 2)
	MinLength int

	// MaxLength is the maximum length of an acronym (default: 10)
	MaxLength int
}

// DetectedAcronym represents a potential acronym found in text.
type DetectedAcronym struct {
	Term    string // The acronym itself
	Context string // Surrounding text for context
	Count   int    // Number of occurrences
}

// NewAcronymDetector creates a new acronym detector with default settings.
func NewAcronymDetector() *AcronymDetector {
	return &AcronymDetector{
		KnownTerms:  make(map[string]bool),
		CommonWords: defaultCommonWords(),
		MinLength:   2,
		MaxLength:   10,
	}
}

// defaultCommonWords returns common abbreviations and words to ignore.
func defaultCommonWords() map[string]bool {
	// Common words, abbreviations, and single letters that aren't acronyms
	words := []string{
		// Single and double letters
		"I", "A", "AM", "PM", "OK", "NO", "SO", "IT", "IS", "AS", "AT", "BE",
		"BY", "DO", "GO", "IF", "IN", "OF", "ON", "OR", "TO", "UP", "WE", "HE",
		"ME", "US",

		// Common abbreviations
		"ID", "VS", "ETC", "AKA", "FYI", "ASAP", "BTW", "RE", "FW", "CC", "BCC",

		// Days and months
		"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN",
		"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC",

		// Common technical terms
		"API", "URL", "HTTP", "HTTPS", "HTML", "CSS", "JSON", "XML", "SQL", "UI", "UX",
		"CPU", "GPU", "RAM", "SSD", "HDD", "USB", "PDF", "CSV", "IP", "TCP", "UDP",
		"DNS", "SSH", "SSL", "TLS", "VPN", "LAN", "WAN", "WIFI", "IOT",

		// Time zones
		"UTC", "GMT", "EST", "PST", "CST", "MST",

		// Miscellaneous
		"USA", "UK", "EU", "US", "CEO", "CTO", "CFO", "COO", "VP", "SVP", "EVP",
		"HR", "IT", "PR", "QA", "AI", "ML", "BI", "PM", "PO",
	}

	m := make(map[string]bool)
	for _, w := range words {
		m[strings.ToUpper(w)] = true
	}
	return m
}

// SetKnownTerms sets the terms to skip (from glossary).
func (d *AcronymDetector) SetKnownTerms(terms []string) {
	d.KnownTerms = make(map[string]bool)
	for _, t := range terms {
		d.KnownTerms[strings.ToUpper(t)] = true
	}
}

// AddKnownTerm adds a single known term.
func (d *AcronymDetector) AddKnownTerm(term string) {
	d.KnownTerms[strings.ToUpper(term)] = true
}

// Detect finds potential acronyms in the given text.
func (d *AcronymDetector) Detect(text string) []DetectedAcronym {
	// Pattern: 2+ uppercase letters, optionally with numbers
	// Includes patterns like DBaaS, PostgreSQL (mixed case with capitals)
	pattern := regexp.MustCompile(`\b([A-Z][A-Za-z0-9]*[A-Z][A-Za-z0-9]*|[A-Z]{2,}[a-z]*)\b`)

	matches := pattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}

	// Track acronyms and their contexts
	acronymContexts := make(map[string][]string)
	acronymCounts := make(map[string]int)

	for _, match := range matches {
		start, end := match[0], match[1]
		word := text[start:end]

		// Normalize to uppercase for comparison
		upperWord := strings.ToUpper(word)

		// Skip if too short or too long
		if len(upperWord) < d.MinLength || len(upperWord) > d.MaxLength {
			continue
		}

		// Skip common words
		if d.CommonWords[upperWord] {
			continue
		}

		// Skip known terms
		if d.KnownTerms[upperWord] {
			continue
		}

		// Skip if it looks like a regular word (mostly lowercase with initial cap)
		if isRegularWord(word) {
			continue
		}

		// Extract context (surrounding text)
		contextStart := maxInt(0, start-50)
		contextEnd := minInt(len(text), end+50)
		context := strings.TrimSpace(text[contextStart:contextEnd])

		// Clean up context
		context = cleanContext(context)

		acronymCounts[upperWord]++
		if len(acronymContexts[upperWord]) < 3 {
			acronymContexts[upperWord] = append(acronymContexts[upperWord], context)
		}
	}

	// Build results
	var results []DetectedAcronym
	for term, count := range acronymCounts {
		// Pick the best context (first one for now)
		context := ""
		if len(acronymContexts[term]) > 0 {
			context = acronymContexts[term][0]
		}

		results = append(results, DetectedAcronym{
			Term:    term,
			Context: context,
			Count:   count,
		})
	}

	return results
}

// isRegularWord checks if a word looks like a regular capitalized word
// (e.g., "Hello" vs "TER" or "DBaaS")
func isRegularWord(word string) bool {
	if len(word) < 2 {
		return false
	}

	// Count uppercase and lowercase letters
	var upper, lower int
	for _, r := range word {
		if unicode.IsUpper(r) {
			upper++
		} else if unicode.IsLower(r) {
			lower++
		}
	}

	// If mostly lowercase with just first letter uppercase, it's a regular word
	if upper == 1 && unicode.IsUpper(rune(word[0])) && lower > 0 {
		return true
	}

	// If entirely lowercase except first letter, it's a regular word
	if upper == 1 && lower == len(word)-1 {
		return true
	}

	return false
}

// cleanContext cleans up context text for display.
func cleanContext(context string) string {
	// Remove excessive whitespace
	context = strings.Join(strings.Fields(context), " ")

	// Truncate if too long
	if len(context) > 150 {
		context = context[:147] + "..."
	}

	return context
}

// DetectInTranscript detects acronyms in a meeting transcript.
// It returns only unique acronyms that appear at least minOccurrences times.
func (d *AcronymDetector) DetectInTranscript(transcript *TranscriptResult, minOccurrences int) []DetectedAcronym {
	if transcript == nil || transcript.FullText == "" {
		return nil
	}

	acronyms := d.Detect(transcript.FullText)

	// Filter by minimum occurrences
	var filtered []DetectedAcronym
	for _, a := range acronyms {
		if a.Count >= minOccurrences {
			filtered = append(filtered, a)
		}
	}

	return filtered
}

// Helper functions
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
