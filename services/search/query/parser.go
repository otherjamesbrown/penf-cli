// Package query provides search query parsing capabilities.
// It supports both natural language queries and structured filter syntax.
package query

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// SortOrder defines the ordering of search results.
type SortOrder int

const (
	// SortRelevance sorts by relevance score (default).
	SortRelevance SortOrder = iota
	// SortDateDesc sorts by date, newest first.
	SortDateDesc
	// SortDateAsc sorts by date, oldest first.
	SortDateAsc
)

// String returns the string representation of a SortOrder.
func (s SortOrder) String() string {
	switch s {
	case SortRelevance:
		return "relevance"
	case SortDateDesc:
		return "date_desc"
	case SortDateAsc:
		return "date_asc"
	default:
		return "relevance"
	}
}

// FilterOperator defines the type of filter operation.
type FilterOperator int

const (
	// OpEquals matches exact values.
	OpEquals FilterOperator = iota
	// OpContains matches partial values.
	OpContains
	// OpAfter matches dates after the specified value.
	OpAfter
	// OpBefore matches dates before the specified value.
	OpBefore
	// OpIn matches values within a set.
	OpIn
)

// String returns the string representation of a FilterOperator.
func (o FilterOperator) String() string {
	switch o {
	case OpEquals:
		return "equals"
	case OpContains:
		return "contains"
	case OpAfter:
		return "after"
	case OpBefore:
		return "before"
	case OpIn:
		return "in"
	default:
		return "equals"
	}
}

// Filter represents a single filter condition.
type Filter struct {
	// Field is the field to filter on (e.g., "type", "from", "to").
	Field string
	// Operator is the filter operation.
	Operator FilterOperator
	// Value is the filter value.
	Value string
	// Values is used for IN operations with multiple values.
	Values []string
	// Negated indicates if this filter should be inverted (NOT).
	Negated bool
}

// BooleanOp represents a boolean operator.
type BooleanOp int

const (
	// BoolNone indicates no boolean operator.
	BoolNone BooleanOp = iota
	// BoolAnd indicates AND operation.
	BoolAnd
	// BoolOr indicates OR operation.
	BoolOr
	// BoolNot indicates NOT operation.
	BoolNot
)

// QueryClause represents a clause in the query, potentially with boolean operators.
type QueryClause struct {
	// Text is the text portion of this clause.
	Text string
	// Filters are the filters in this clause.
	Filters []Filter
	// BooleanOp is the operator connecting this clause to the next.
	BooleanOp BooleanOp
	// IsQuoted indicates if the text was quoted (exact match).
	IsQuoted bool
}

// ParsedQuery represents a fully parsed search query.
type ParsedQuery struct {
	// TextQuery is the text portion for BM25/vector search.
	TextQuery string
	// Clauses are the parsed query clauses with boolean operators.
	Clauses []QueryClause
	// Filters are all extracted filters.
	Filters []Filter
	// Sort is the requested sort order.
	Sort SortOrder
	// Limit is the maximum number of results (0 means use default).
	Limit int
	// Offset is the pagination offset.
	Offset int
	// DateFrom is the start of the date range filter.
	DateFrom *time.Time
	// DateTo is the end of the date range filter.
	DateTo *time.Time
	// ContentTypes are the requested content types.
	ContentTypes []string
	// OriginalQuery is the original input query.
	OriginalQuery string
}

// ParseError represents an error during query parsing.
type ParseError struct {
	// Message is the error message.
	Message string
	// Position is the position in the query where the error occurred.
	Position int
	// Context is the surrounding text for context.
	Context string
}

func (e *ParseError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("parse error at position %d: %s (near '%s')", e.Position, e.Message, e.Context)
	}
	return fmt.Sprintf("parse error at position %d: %s", e.Position, e.Message)
}

// Parser parses search queries into structured ParsedQuery objects.
type Parser struct {
	// dateLayouts are the date formats we accept.
	dateLayouts []string
}

// NewParser creates a new Parser instance.
func NewParser() *Parser {
	return &Parser{
		dateLayouts: []string{
			"2006-01-02",
			"2006/01/02",
			"01-02-2006",
			"01/02/2006",
			"Jan 2, 2006",
			"January 2, 2006",
			"2 Jan 2006",
			"2006-01-02T15:04:05Z07:00",
		},
	}
}

// Parse parses the input query string and returns a ParsedQuery.
func (p *Parser) Parse(input string) (*ParsedQuery, error) {
	if strings.TrimSpace(input) == "" {
		return nil, &ParseError{
			Message:  "empty query",
			Position: 0,
		}
	}

	result := &ParsedQuery{
		OriginalQuery: input,
		Sort:          SortRelevance,
	}

	// Tokenize the input
	tokens, err := p.tokenize(input)
	if err != nil {
		return nil, err
	}

	// Parse tokens into clauses and filters
	err = p.parseTokens(tokens, result)
	if err != nil {
		return nil, err
	}

	// Build the text query from non-filter tokens
	result.TextQuery = p.buildTextQuery(result.Clauses)

	// Extract date filters into convenience fields
	p.extractDateFilters(result)

	// Extract content type filters
	p.extractContentTypes(result)

	return result, nil
}

// token represents a parsed token from the input.
type token struct {
	value    string
	position int
	isQuoted bool
	isFilter bool
	key      string
	negated  bool
}

// tokenize breaks the input into tokens.
func (p *Parser) tokenize(input string) ([]token, error) {
	var tokens []token
	pos := 0
	runes := []rune(input)
	n := len(runes)

	for pos < n {
		// Skip whitespace
		for pos < n && unicode.IsSpace(runes[pos]) {
			pos++
		}
		if pos >= n {
			break
		}

		startPos := pos

		// Check for quoted string
		if runes[pos] == '"' {
			pos++
			var sb strings.Builder
			for pos < n && runes[pos] != '"' {
				if runes[pos] == '\\' && pos+1 < n {
					pos++
					sb.WriteRune(runes[pos])
				} else {
					sb.WriteRune(runes[pos])
				}
				pos++
			}
			if pos >= n {
				return nil, &ParseError{
					Message:  "unclosed quoted string",
					Position: startPos,
					Context:  string(runes[startPos:min(startPos+20, n)]),
				}
			}
			pos++ // skip closing quote
			tokens = append(tokens, token{
				value:    sb.String(),
				position: startPos,
				isQuoted: true,
			})
			continue
		}

		// Check for negation prefix
		negated := false
		if runes[pos] == '-' && pos+1 < n && !unicode.IsSpace(runes[pos+1]) {
			negated = true
			pos++
			startPos = pos
		}

		// Read until whitespace or special character
		var sb strings.Builder
		for pos < n && !unicode.IsSpace(runes[pos]) && runes[pos] != '"' {
			sb.WriteRune(runes[pos])
			pos++
		}

		word := sb.String()
		if word == "" {
			continue
		}

		// Check if this is a filter (contains colon)
		if colonIdx := strings.Index(word, ":"); colonIdx > 0 {
			key := word[:colonIdx]
			value := word[colonIdx+1:]

			// Handle quoted value after colon
			if value == "" && pos < n && runes[pos] == '"' {
				pos++
				var valueSb strings.Builder
				for pos < n && runes[pos] != '"' {
					if runes[pos] == '\\' && pos+1 < n {
						pos++
						valueSb.WriteRune(runes[pos])
					} else {
						valueSb.WriteRune(runes[pos])
					}
					pos++
				}
				if pos >= n {
					return nil, &ParseError{
						Message:  "unclosed quoted string in filter value",
						Position: startPos,
						Context:  key + ":",
					}
				}
				pos++ // skip closing quote
				value = valueSb.String()
			}

			tokens = append(tokens, token{
				value:    value,
				position: startPos,
				isFilter: true,
				key:      strings.ToLower(key),
				negated:  negated,
			})
		} else {
			// Check for boolean operators
			upperWord := strings.ToUpper(word)
			if upperWord == "AND" || upperWord == "OR" || upperWord == "NOT" {
				tokens = append(tokens, token{
					value:    upperWord,
					position: startPos,
				})
			} else {
				tokens = append(tokens, token{
					value:    word,
					position: startPos,
					negated:  negated,
				})
			}
		}
	}

	return tokens, nil
}

// parseTokens processes tokens into clauses and filters.
func (p *Parser) parseTokens(tokens []token, result *ParsedQuery) error {
	var currentClause QueryClause
	var pendingOp BooleanOp = BoolNone

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		// Handle boolean operators
		if !tok.isFilter && !tok.isQuoted {
			switch tok.value {
			case "AND":
				pendingOp = BoolAnd
				continue
			case "OR":
				pendingOp = BoolOr
				continue
			case "NOT":
				// NOT modifies the next token
				if i+1 < len(tokens) {
					tokens[i+1].negated = true
				}
				continue
			}
		}

		if tok.isFilter {
			filter, err := p.parseFilter(tok)
			if err != nil {
				return err
			}
			result.Filters = append(result.Filters, filter)
			currentClause.Filters = append(currentClause.Filters, filter)
		} else {
			// Add text to current clause
			if currentClause.Text != "" {
				currentClause.Text += " "
			}
			if tok.negated {
				currentClause.Text += "-"
			}
			currentClause.Text += tok.value
			currentClause.IsQuoted = tok.isQuoted
		}

		// If we have a pending operator and text/filter, finalize clause
		if pendingOp != BoolNone && (currentClause.Text != "" || len(currentClause.Filters) > 0) {
			currentClause.BooleanOp = pendingOp
			result.Clauses = append(result.Clauses, currentClause)
			currentClause = QueryClause{}
			pendingOp = BoolNone
		}
	}

	// Add final clause
	if currentClause.Text != "" || len(currentClause.Filters) > 0 {
		result.Clauses = append(result.Clauses, currentClause)
	}

	return nil
}

// parseFilter creates a Filter from a filter token.
func (p *Parser) parseFilter(tok token) (Filter, error) {
	filter := Filter{
		Field:   tok.key,
		Value:   tok.value,
		Negated: tok.negated,
	}

	// Determine operator based on key
	switch tok.key {
	case "after":
		filter.Operator = OpAfter
		filter.Field = "date"
		if _, err := p.parseDate(tok.value); err != nil {
			return Filter{}, &ParseError{
				Message:  fmt.Sprintf("invalid date format for 'after': %s", tok.value),
				Position: tok.position,
				Context:  "after:" + tok.value,
			}
		}
	case "before":
		filter.Operator = OpBefore
		filter.Field = "date"
		if _, err := p.parseDate(tok.value); err != nil {
			return Filter{}, &ParseError{
				Message:  fmt.Sprintf("invalid date format for 'before': %s", tok.value),
				Position: tok.position,
				Context:  "before:" + tok.value,
			}
		}
	case "type":
		filter.Operator = OpEquals
		filter.Value = strings.ToLower(tok.value)
	case "from", "to", "sender", "recipient":
		filter.Operator = OpEquals
		// Normalize field names
		if tok.key == "sender" {
			filter.Field = "from"
		}
		if tok.key == "recipient" {
			filter.Field = "to"
		}
	case "in":
		filter.Operator = OpIn
		filter.Field = "source"
	case "sort":
		filter.Operator = OpEquals
		filter.Field = "sort"
		// Parse sort value
		switch strings.ToLower(tok.value) {
		case "date", "date_desc", "newest":
			filter.Value = "date_desc"
		case "date_asc", "oldest":
			filter.Value = "date_asc"
		case "relevance", "score":
			filter.Value = "relevance"
		}
	case "limit":
		filter.Operator = OpEquals
	case "offset":
		filter.Operator = OpEquals
	default:
		// Unknown filter key, treat as contains
		filter.Operator = OpContains
	}

	return filter, nil
}

// parseDate attempts to parse a date string in various formats.
func (p *Parser) parseDate(s string) (time.Time, error) {
	// Try relative dates first
	s = strings.ToLower(strings.TrimSpace(s))
	now := time.Now()

	switch s {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		return time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location()), nil
	case "thisweek", "this_week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startOfWeek := now.AddDate(0, 0, -weekday+1)
		return time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, now.Location()), nil
	case "thismonth", "this_month":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), nil
	case "lastweek", "last_week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startOfLastWeek := now.AddDate(0, 0, -weekday-6)
		return time.Date(startOfLastWeek.Year(), startOfLastWeek.Month(), startOfLastWeek.Day(), 0, 0, 0, 0, now.Location()), nil
	case "lastmonth", "last_month":
		lastMonth := now.AddDate(0, -1, 0)
		return time.Date(lastMonth.Year(), lastMonth.Month(), 1, 0, 0, 0, 0, now.Location()), nil
	}

	// Try each date layout
	for _, layout := range p.dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", s)
}

// buildTextQuery constructs the text query from clauses.
func (p *Parser) buildTextQuery(clauses []QueryClause) string {
	var parts []string

	for _, clause := range clauses {
		if clause.Text != "" {
			text := clause.Text
			if clause.IsQuoted {
				text = `"` + text + `"`
			}
			parts = append(parts, text)
		}
	}

	return strings.Join(parts, " ")
}

// extractDateFilters pulls date filters into convenience fields.
func (p *Parser) extractDateFilters(result *ParsedQuery) {
	for _, filter := range result.Filters {
		if filter.Field == "date" {
			date, err := p.parseDate(filter.Value)
			if err != nil {
				continue
			}
			switch filter.Operator {
			case OpAfter:
				result.DateFrom = &date
			case OpBefore:
				result.DateTo = &date
			}
		}
		if filter.Field == "sort" {
			switch filter.Value {
			case "date_desc":
				result.Sort = SortDateDesc
			case "date_asc":
				result.Sort = SortDateAsc
			case "relevance":
				result.Sort = SortRelevance
			}
		}
	}
}

// extractContentTypes pulls type filters into convenience field.
func (p *Parser) extractContentTypes(result *ParsedQuery) {
	for _, filter := range result.Filters {
		if filter.Field == "type" && !filter.Negated {
			result.ContentTypes = append(result.ContentTypes, filter.Value)
		}
	}
}

// ParseNaturalLanguage attempts to interpret natural language queries.
// It extracts entities, dates, and intent from conversational queries.
func (p *Parser) ParseNaturalLanguage(input string) (*ParsedQuery, error) {
	// First, try standard parsing
	result, err := p.Parse(input)
	if err != nil {
		return nil, err
	}

	// Apply natural language patterns to extract additional filters
	p.applyNaturalLanguagePatterns(input, result)

	return result, nil
}

// applyNaturalLanguagePatterns extracts filters from natural language patterns.
func (p *Parser) applyNaturalLanguagePatterns(input string, result *ParsedQuery) {
	lowerInput := strings.ToLower(input)

	// Pattern: "emails from <person>"
	fromPattern := regexp.MustCompile(`(?i)(?:emails?|messages?|mail)\s+from\s+([A-Za-z]+(?:\s+[A-Za-z]+)?)`)
	if matches := fromPattern.FindStringSubmatch(input); len(matches) > 1 {
		result.Filters = append(result.Filters, Filter{
			Field:    "from",
			Operator: OpContains,
			Value:    strings.TrimSpace(matches[1]),
		})
		// Also ensure email type if mentioned
		if strings.Contains(lowerInput, "email") {
			hasEmailType := false
			for _, ct := range result.ContentTypes {
				if ct == "email" {
					hasEmailType = true
					break
				}
			}
			if !hasEmailType {
				result.ContentTypes = append(result.ContentTypes, "email")
				result.Filters = append(result.Filters, Filter{
					Field:    "type",
					Operator: OpEquals,
					Value:    "email",
				})
			}
		}
	}

	// Pattern: "to <person>"
	toPattern := regexp.MustCompile(`(?i)(?:emails?|messages?|mail)?\s*to\s+([A-Za-z]+(?:\s+[A-Za-z]+)?)`)
	if matches := toPattern.FindStringSubmatch(input); len(matches) > 1 {
		// Avoid matching "to" in common phrases like "related to"
		if !strings.Contains(lowerInput, "related to") {
			result.Filters = append(result.Filters, Filter{
				Field:    "to",
				Operator: OpContains,
				Value:    strings.TrimSpace(matches[1]),
			})
		}
	}

	// Pattern: "about <topic>"
	aboutPattern := regexp.MustCompile(`(?i)about\s+([^,\.\n]+)`)
	if matches := aboutPattern.FindStringSubmatch(input); len(matches) > 1 {
		// Add to text query for semantic matching
		topic := strings.TrimSpace(matches[1])
		if !strings.Contains(result.TextQuery, topic) {
			if result.TextQuery != "" {
				result.TextQuery += " "
			}
			result.TextQuery += topic
		}
	}

	// Pattern: temporal references
	if strings.Contains(lowerInput, "last week") {
		date, _ := p.parseDate("lastweek")
		result.DateFrom = &date
	}
	if strings.Contains(lowerInput, "last month") {
		date, _ := p.parseDate("lastmonth")
		result.DateFrom = &date
	}
	if strings.Contains(lowerInput, "this week") {
		date, _ := p.parseDate("thisweek")
		result.DateFrom = &date
	}
	if strings.Contains(lowerInput, "this month") {
		date, _ := p.parseDate("thismonth")
		result.DateFrom = &date
	}
	if strings.Contains(lowerInput, "yesterday") {
		date, _ := p.parseDate("yesterday")
		result.DateFrom = &date
		endDate := date.Add(24 * time.Hour)
		result.DateTo = &endDate
	}
	if strings.Contains(lowerInput, "today") {
		date, _ := p.parseDate("today")
		result.DateFrom = &date
	}

	// Pattern: content types
	typePatterns := map[string][]string{
		"email":    {"emails?", "mail"},
		"meeting":  {"meetings?", "calendar", "appointment"},
		"document": {"documents?", "files?", "docs?"},
		"chat":     {"chats?", "messages?", "slack", "im"},
		"note":     {"notes?", "memo"},
	}

	for contentType, patterns := range typePatterns {
		for _, pattern := range patterns {
			re := regexp.MustCompile(`(?i)\b` + pattern + `\b`)
			if re.MatchString(input) {
				hasType := false
				for _, ct := range result.ContentTypes {
					if ct == contentType {
						hasType = true
						break
					}
				}
				if !hasType {
					result.ContentTypes = append(result.ContentTypes, contentType)
					result.Filters = append(result.Filters, Filter{
						Field:    "type",
						Operator: OpEquals,
						Value:    contentType,
					})
				}
				break
			}
		}
	}

	// Pattern: "newest" or "most recent" implies date sort
	if strings.Contains(lowerInput, "newest") || strings.Contains(lowerInput, "most recent") || strings.Contains(lowerInput, "latest") {
		result.Sort = SortDateDesc
	}

	// Pattern: "oldest" implies ascending date sort
	if strings.Contains(lowerInput, "oldest") || strings.Contains(lowerInput, "earliest") {
		result.Sort = SortDateAsc
	}
}

// Helper function for Go versions before 1.21
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
