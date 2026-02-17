package query

import (
	"strings"
	"testing"
	"time"
)

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("NewParser() returned nil")
	}
	if len(p.dateLayouts) == 0 {
		t.Error("NewParser() dateLayouts is empty")
	}
}

func TestParser_Parse_EmptyQuery(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"tabs", "\t\t"},
		{"newlines", "\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.Parse(tt.input)
			if err == nil {
				t.Error("Parse() expected error for empty query, got nil")
			}
			if pe, ok := err.(*ParseError); !ok {
				t.Errorf("Parse() expected *ParseError, got %T", err)
			} else if !strings.Contains(pe.Message, "empty") {
				t.Errorf("Parse() error message = %q, want to contain 'empty'", pe.Message)
			}
		})
	}
}

func TestParser_Parse_SimpleText(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name      string
		input     string
		wantQuery string
	}{
		{"single_word", "hello", "hello"},
		{"multiple_words", "hello world", "hello world"},
		{"mixed_case", "Hello World", "Hello World"},
		{"with_numbers", "project 2024", "project 2024"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if result.TextQuery != tt.wantQuery {
				t.Errorf("TextQuery = %q, want %q", result.TextQuery, tt.wantQuery)
			}
			if result.OriginalQuery != tt.input {
				t.Errorf("OriginalQuery = %q, want %q", result.OriginalQuery, tt.input)
			}
		})
	}
}

func TestParser_Parse_QuotedPhrases(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name         string
		input        string
		wantContains []string
	}{
		{"simple_quoted", `"exact match"`, []string{"exact match"}},
		{"quoted_with_text", `hello "exact match" world`, []string{"hello", "exact match", "world"}},
		{"multiple_quoted", `"first phrase" "second phrase"`, []string{"first phrase", "second phrase"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(result.TextQuery, want) {
					t.Errorf("TextQuery = %q, want to contain %q", result.TextQuery, want)
				}
			}
		})
	}
}

func TestParser_Parse_UnclosedQuote(t *testing.T) {
	p := NewParser()

	_, err := p.Parse(`"unclosed quote`)
	if err == nil {
		t.Error("Parse() expected error for unclosed quote, got nil")
	}
	if pe, ok := err.(*ParseError); !ok {
		t.Errorf("Parse() expected *ParseError, got %T", err)
	} else if !strings.Contains(pe.Message, "unclosed") {
		t.Errorf("Parse() error message = %q, want to contain 'unclosed'", pe.Message)
	}
}

func TestParser_Parse_FilterFrom(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name       string
		input      string
		wantField  string
		wantValue  string
		wantNegate bool
	}{
		{"from_filter", "from:john", "from", "john", false},
		{"from_with_domain", "from:john@example.com", "from", "john@example.com", false},
		{"sender_alias", "sender:jane", "from", "jane", false},
		{"negated_from", "-from:john", "from", "john", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if len(result.Filters) != 1 {
				t.Fatalf("Filters count = %d, want 1", len(result.Filters))
			}
			filter := result.Filters[0]
			if filter.Field != tt.wantField {
				t.Errorf("Filter.Field = %q, want %q", filter.Field, tt.wantField)
			}
			if filter.Value != tt.wantValue {
				t.Errorf("Filter.Value = %q, want %q", filter.Value, tt.wantValue)
			}
			if filter.Negated != tt.wantNegate {
				t.Errorf("Filter.Negated = %v, want %v", filter.Negated, tt.wantNegate)
			}
		})
	}
}

func TestParser_Parse_FilterTo(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name      string
		input     string
		wantField string
		wantValue string
	}{
		{"to_filter", "to:jane", "to", "jane"},
		{"recipient_alias", "recipient:bob", "to", "bob"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if len(result.Filters) != 1 {
				t.Fatalf("Filters count = %d, want 1", len(result.Filters))
			}
			filter := result.Filters[0]
			if filter.Field != tt.wantField {
				t.Errorf("Filter.Field = %q, want %q", filter.Field, tt.wantField)
			}
			if filter.Value != tt.wantValue {
				t.Errorf("Filter.Value = %q, want %q", filter.Value, tt.wantValue)
			}
		})
	}
}

func TestParser_Parse_FilterType(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name      string
		input     string
		wantValue string
		wantTypes []string
	}{
		{"email_type", "type:email", "email", []string{"email"}},
		{"meeting_type", "type:meeting", "meeting", []string{"meeting"}},
		{"document_type", "type:document", "document", []string{"document"}},
		{"uppercase_type", "type:EMAIL", "email", []string{"email"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if len(result.Filters) != 1 {
				t.Fatalf("Filters count = %d, want 1", len(result.Filters))
			}
			filter := result.Filters[0]
			if filter.Value != tt.wantValue {
				t.Errorf("Filter.Value = %q, want %q", filter.Value, tt.wantValue)
			}
			if len(result.ContentTypes) != len(tt.wantTypes) {
				t.Errorf("ContentTypes = %v, want %v", result.ContentTypes, tt.wantTypes)
			}
		})
	}
}

func TestParser_Parse_DateFilters(t *testing.T) {
	p := NewParser()

	// Test specific date
	result, err := p.Parse("after:2024-01-01")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if result.DateFrom == nil {
		t.Error("DateFrom is nil, expected date")
	} else {
		expected := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		if !result.DateFrom.Equal(expected) {
			t.Errorf("DateFrom = %v, want %v", result.DateFrom, expected)
		}
	}

	// Test before date
	result, err = p.Parse("before:2024-12-31")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if result.DateTo == nil {
		t.Error("DateTo is nil, expected date")
	}

	// Test date range
	result, err = p.Parse("after:2024-01-01 before:2024-12-31")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if result.DateFrom == nil {
		t.Error("DateFrom is nil, expected date")
	}
	if result.DateTo == nil {
		t.Error("DateTo is nil, expected date")
	}
}

func TestParser_Parse_RelativeDates(t *testing.T) {
	p := NewParser()
	now := time.Now()

	tests := []struct {
		name  string
		input string
	}{
		{"today", "after:today"},
		{"yesterday", "after:yesterday"},
		{"this_week", "after:thisweek"},
		{"last_week", "after:lastweek"},
		{"this_month", "after:thismonth"},
		{"last_month", "after:lastmonth"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if result.DateFrom == nil {
				t.Error("DateFrom is nil, expected date")
			}
			// Verify date is in reasonable range
			if result.DateFrom.After(now) {
				t.Errorf("DateFrom = %v is after now", result.DateFrom)
			}
		})
	}
}

func TestParser_Parse_InvalidDate(t *testing.T) {
	p := NewParser()

	_, err := p.Parse("after:not-a-date")
	if err == nil {
		t.Error("Parse() expected error for invalid date, got nil")
	}
	if pe, ok := err.(*ParseError); !ok {
		t.Errorf("Parse() expected *ParseError, got %T", err)
	} else if !strings.Contains(pe.Message, "invalid date") {
		t.Errorf("Parse() error message = %q, want to contain 'invalid date'", pe.Message)
	}
}

func TestParser_Parse_InFilter(t *testing.T) {
	p := NewParser()

	result, err := p.Parse("in:inbox")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(result.Filters) != 1 {
		t.Fatalf("Filters count = %d, want 1", len(result.Filters))
	}
	filter := result.Filters[0]
	if filter.Field != "source" {
		t.Errorf("Filter.Field = %q, want %q", filter.Field, "source")
	}
	if filter.Operator != OpIn {
		t.Errorf("Filter.Operator = %v, want %v", filter.Operator, OpIn)
	}
}

func TestParser_Parse_BooleanOperators(t *testing.T) {
	p := NewParser()

	// AND operator - both terms should be present in result
	result, err := p.Parse("project AND budget")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !strings.Contains(result.TextQuery, "project") || !strings.Contains(result.TextQuery, "budget") {
		t.Errorf("TextQuery = %q, want to contain 'project' and 'budget'", result.TextQuery)
	}

	// OR operator - both terms should be present
	result, err = p.Parse("email OR message")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !strings.Contains(result.TextQuery, "email") || !strings.Contains(result.TextQuery, "message") {
		t.Errorf("TextQuery = %q, want to contain 'email' and 'message'", result.TextQuery)
	}

	// NOT operator - should negate the term
	result, err = p.Parse("budget NOT draft")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	// The negated term should be marked
	if !strings.Contains(result.TextQuery, "budget") {
		t.Errorf("TextQuery = %q, want to contain 'budget'", result.TextQuery)
	}
	if !strings.Contains(result.TextQuery, "-draft") {
		t.Errorf("TextQuery = %q, want to contain '-draft' for negation", result.TextQuery)
	}
}

func TestParser_Parse_SortOrder(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name     string
		input    string
		wantSort SortOrder
	}{
		{"sort_date", "test sort:date", SortDateDesc},
		{"sort_date_desc", "test sort:date_desc", SortDateDesc},
		{"sort_newest", "test sort:newest", SortDateDesc},
		{"sort_date_asc", "test sort:date_asc", SortDateAsc},
		{"sort_oldest", "test sort:oldest", SortDateAsc},
		{"sort_relevance", "test sort:relevance", SortRelevance},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if result.Sort != tt.wantSort {
				t.Errorf("Sort = %v, want %v", result.Sort, tt.wantSort)
			}
		})
	}
}

func TestParser_Parse_ComplexQuery(t *testing.T) {
	p := NewParser()

	// Complex query with multiple filters and text
	input := `from:john type:email after:2024-01-01 "budget proposal" sort:date`
	result, err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should have filters for from, type, after, sort
	if len(result.Filters) < 4 {
		t.Errorf("Filters count = %d, want at least 4", len(result.Filters))
	}

	// Should have text query
	if !strings.Contains(result.TextQuery, "budget proposal") {
		t.Errorf("TextQuery = %q, want to contain 'budget proposal'", result.TextQuery)
	}

	// Should have content type
	if len(result.ContentTypes) == 0 {
		t.Error("ContentTypes is empty, expected at least 'email'")
	}

	// Should have sort order
	if result.Sort != SortDateDesc {
		t.Errorf("Sort = %v, want %v", result.Sort, SortDateDesc)
	}
}

func TestParser_Parse_MixedFiltersAndText(t *testing.T) {
	p := NewParser()

	result, err := p.Parse("project meeting from:alice type:email notes")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Text should contain both text portions
	if !strings.Contains(result.TextQuery, "project") {
		t.Errorf("TextQuery = %q, want to contain 'project'", result.TextQuery)
	}
	if !strings.Contains(result.TextQuery, "meeting") {
		t.Errorf("TextQuery = %q, want to contain 'meeting'", result.TextQuery)
	}
	if !strings.Contains(result.TextQuery, "notes") {
		t.Errorf("TextQuery = %q, want to contain 'notes'", result.TextQuery)
	}

	// Should have filters
	if len(result.Filters) != 2 {
		t.Errorf("Filters count = %d, want 2", len(result.Filters))
	}
}

func TestParser_ParseNaturalLanguage_EmailsFrom(t *testing.T) {
	p := NewParser()

	result, err := p.ParseNaturalLanguage("emails from John about project")
	if err != nil {
		t.Fatalf("ParseNaturalLanguage() error = %v", err)
	}

	// Should extract "from" filter
	hasFromFilter := false
	for _, f := range result.Filters {
		if f.Field == "from" && strings.Contains(strings.ToLower(f.Value), "john") {
			hasFromFilter = true
			break
		}
	}
	if !hasFromFilter {
		t.Error("expected 'from' filter for John, not found")
	}

	// Should have email content type
	hasEmailType := false
	for _, ct := range result.ContentTypes {
		if ct == "email" {
			hasEmailType = true
			break
		}
	}
	if !hasEmailType {
		t.Error("expected 'email' content type, not found")
	}

	// Should include "project" in text query
	if !strings.Contains(result.TextQuery, "project") {
		t.Errorf("TextQuery = %q, want to contain 'project'", result.TextQuery)
	}
}

func TestParser_ParseNaturalLanguage_TemporalReferences(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name       string
		input      string
		expectFrom bool
		expectTo   bool
	}{
		{"last_week", "meetings last week", true, false},
		{"last_month", "emails last month", true, false},
		{"this_week", "documents this week", true, false},
		{"yesterday", "notes from yesterday", true, true},
		{"today", "messages today", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.ParseNaturalLanguage(tt.input)
			if err != nil {
				t.Fatalf("ParseNaturalLanguage() error = %v", err)
			}
			if tt.expectFrom && result.DateFrom == nil {
				t.Error("DateFrom is nil, expected date")
			}
			if tt.expectTo && result.DateTo == nil {
				t.Error("DateTo is nil, expected date")
			}
		})
	}
}

func TestParser_ParseNaturalLanguage_ContentTypes(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name     string
		input    string
		wantType string
	}{
		{"emails", "find my emails", "email"},
		{"meetings", "show me meetings", "meeting"},
		{"documents", "search documents", "document"},
		{"chat", "slack messages", "chat"},
		{"notes", "my notes about", "note"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.ParseNaturalLanguage(tt.input)
			if err != nil {
				t.Fatalf("ParseNaturalLanguage() error = %v", err)
			}
			hasType := false
			for _, ct := range result.ContentTypes {
				if ct == tt.wantType {
					hasType = true
					break
				}
			}
			if !hasType {
				t.Errorf("ContentTypes = %v, want to include %q", result.ContentTypes, tt.wantType)
			}
		})
	}
}

func TestParser_ParseNaturalLanguage_SortHints(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name     string
		input    string
		wantSort SortOrder
	}{
		{"newest", "newest emails about project", SortDateDesc},
		{"most_recent", "most recent documents", SortDateDesc},
		{"latest", "latest meeting notes", SortDateDesc},
		{"oldest", "oldest emails", SortDateAsc},
		{"earliest", "earliest messages", SortDateAsc},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.ParseNaturalLanguage(tt.input)
			if err != nil {
				t.Fatalf("ParseNaturalLanguage() error = %v", err)
			}
			if result.Sort != tt.wantSort {
				t.Errorf("Sort = %v, want %v", result.Sort, tt.wantSort)
			}
		})
	}
}

func TestParser_Parse_QuotedFilterValue(t *testing.T) {
	p := NewParser()

	result, err := p.Parse(`from:"John Smith"`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(result.Filters) != 1 {
		t.Fatalf("Filters count = %d, want 1", len(result.Filters))
	}
	if result.Filters[0].Value != "John Smith" {
		t.Errorf("Filter.Value = %q, want %q", result.Filters[0].Value, "John Smith")
	}
}

func TestParser_Parse_MultipleFilters(t *testing.T) {
	p := NewParser()

	result, err := p.Parse("from:alice to:bob type:email")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(result.Filters) != 3 {
		t.Errorf("Filters count = %d, want 3", len(result.Filters))
	}

	// Verify each filter
	filtersByField := make(map[string]Filter)
	for _, f := range result.Filters {
		filtersByField[f.Field] = f
	}

	if f, ok := filtersByField["from"]; !ok {
		t.Error("missing 'from' filter")
	} else if f.Value != "alice" {
		t.Errorf("from filter value = %q, want 'alice'", f.Value)
	}

	if f, ok := filtersByField["to"]; !ok {
		t.Error("missing 'to' filter")
	} else if f.Value != "bob" {
		t.Errorf("to filter value = %q, want 'bob'", f.Value)
	}

	if f, ok := filtersByField["type"]; !ok {
		t.Error("missing 'type' filter")
	} else if f.Value != "email" {
		t.Errorf("type filter value = %q, want 'email'", f.Value)
	}
}

func TestSortOrder_String(t *testing.T) {
	tests := []struct {
		sort SortOrder
		want string
	}{
		{SortRelevance, "relevance"},
		{SortDateDesc, "date_desc"},
		{SortDateAsc, "date_asc"},
		{SortOrder(99), "relevance"}, // unknown defaults to relevance
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.sort.String(); got != tt.want {
				t.Errorf("SortOrder.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterOperator_String(t *testing.T) {
	tests := []struct {
		op   FilterOperator
		want string
	}{
		{OpEquals, "equals"},
		{OpContains, "contains"},
		{OpAfter, "after"},
		{OpBefore, "before"},
		{OpIn, "in"},
		{FilterOperator(99), "equals"}, // unknown defaults to equals
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.op.String(); got != tt.want {
				t.Errorf("FilterOperator.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *ParseError
		wantMsg string
	}{
		{
			name: "with_context",
			err: &ParseError{
				Message:  "test error",
				Position: 5,
				Context:  "hello",
			},
			wantMsg: "parse error at position 5: test error (near 'hello')",
		},
		{
			name: "without_context",
			err: &ParseError{
				Message:  "test error",
				Position: 10,
			},
			wantMsg: "parse error at position 10: test error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("ParseError.Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestParser_Parse_EscapedQuotes(t *testing.T) {
	p := NewParser()

	result, err := p.Parse(`"he said \"hello\""`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !strings.Contains(result.TextQuery, `he said "hello"`) {
		t.Errorf("TextQuery = %q, want to contain escaped quote content", result.TextQuery)
	}
}

func TestParser_Parse_NegatedFilter(t *testing.T) {
	p := NewParser()

	result, err := p.Parse("-type:email project")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(result.Filters) != 1 {
		t.Fatalf("Filters count = %d, want 1", len(result.Filters))
	}

	filter := result.Filters[0]
	if !filter.Negated {
		t.Error("Filter.Negated = false, want true")
	}
	if filter.Field != "type" {
		t.Errorf("Filter.Field = %q, want 'type'", filter.Field)
	}
}

func TestParser_Parse_DefaultSort(t *testing.T) {
	p := NewParser()

	result, err := p.Parse("simple query")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if result.Sort != SortRelevance {
		t.Errorf("Sort = %v, want %v (default)", result.Sort, SortRelevance)
	}
}

func TestParser_Parse_DateFormats(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name  string
		input string
	}{
		{"iso", "after:2024-01-15"},
		{"slash", "after:2024/01/15"},
		{"us_slash", "after:01/15/2024"},
		{"us_dash", "after:01-15-2024"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if result.DateFrom == nil {
				t.Error("DateFrom is nil, expected date")
			}
		})
	}
}

// Benchmark tests
func BenchmarkParser_Parse_Simple(b *testing.B) {
	p := NewParser()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.Parse("simple query")
	}
}

func BenchmarkParser_Parse_Complex(b *testing.B) {
	p := NewParser()
	input := `from:john type:email after:2024-01-01 "budget proposal" sort:date`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.Parse(input)
	}
}

func BenchmarkParser_ParseNaturalLanguage(b *testing.B) {
	p := NewParser()
	input := "emails from John about project budget last week"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.ParseNaturalLanguage(input)
	}
}
