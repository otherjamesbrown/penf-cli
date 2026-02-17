package products

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==================== Parse Query Tests ====================

func TestParseRoleQuery(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		expectedType QueryType
		expectedRole string
		expectedProduct string
		expectedScope string
	}{
		{
			name:        "who is DRI",
			query:       "who is the DRI for MTC",
			expectedType: QueryTypeRole,
			expectedRole: "DRI",
			expectedProduct: "MTC",
		},
		{
			name:        "who is DRI lowercase",
			query:       "who is dri for mtc",
			expectedType: QueryTypeRole,
			expectedRole: "DRI",
			expectedProduct: "mtc",
		},
		{
			name:        "who is DRI with scope",
			query:       "who is the DRI for networking on MTC",
			expectedType: QueryTypeRole,
			expectedRole: "DRI",
			expectedProduct: "MTC",
			expectedScope: "networking",
		},
		{
			name:        "find manager",
			query:       "find the manager for Cloud Platform",
			expectedType: QueryTypeRole,
			expectedRole: "Manager",
			expectedProduct: "Cloud Platform",
		},
		{
			name:        "show tech lead",
			query:       "show me the tech lead on API Gateway",
			expectedType: QueryTypeRole,
			expectedRole: "Tech Lead",
			expectedProduct: "API Gateway",
		},
	}

	svc := &QueryService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := svc.Parse(tt.query)
			assert.Equal(t, tt.expectedType, parsed.Type, "query type mismatch")
			assert.Equal(t, tt.expectedRole, parsed.Role, "role mismatch")
			assert.Equal(t, tt.expectedProduct, parsed.ProductName, "product name mismatch")
			if tt.expectedScope != "" {
				assert.Equal(t, tt.expectedScope, parsed.Scope, "scope mismatch")
			}
		})
	}
}

func TestParseTeamQuery(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		expectedType    QueryType
		expectedProduct string
	}{
		{
			name:            "which teams work on",
			query:           "which teams work on LKE Enterprise",
			expectedType:    QueryTypeTeam,
			expectedProduct: "LKE Enterprise",
		},
		{
			name:            "what teams are on",
			query:           "what teams are on Cloud Platform",
			expectedType:    QueryTypeTeam,
			expectedProduct: "Cloud Platform",
		},
		{
			name:            "show teams",
			query:           "show teams for MTC",
			expectedType:    QueryTypeTeam,
			expectedProduct: "MTC",
		},
		{
			name:            "list teams",
			query:           "list teams on API Gateway",
			expectedType:    QueryTypeTeam,
			expectedProduct: "API Gateway",
		},
	}

	svc := &QueryService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := svc.Parse(tt.query)
			assert.Equal(t, tt.expectedType, parsed.Type)
			assert.Equal(t, tt.expectedProduct, parsed.ProductName)
		})
	}
}

func TestParseTimelineQuery(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		expectedType    QueryType
		expectedProduct string
	}{
		{
			name:            "show timeline",
			query:           "show timeline for MTC",
			expectedType:    QueryTypeTimeline,
			expectedProduct: "MTC",
		},
		{
			name:            "get timeline",
			query:           "get the timeline of Cloud Platform",
			expectedType:    QueryTypeTimeline,
			expectedProduct: "Cloud Platform",
		},
		{
			name:            "what happened",
			query:           "what happened with LKE Enterprise",
			expectedType:    QueryTypeTimeline,
			expectedProduct: "LKE Enterprise",
		},
		{
			name:            "events for",
			query:           "events for API Gateway",
			expectedType:    QueryTypeTimeline,
			expectedProduct: "API Gateway",
		},
	}

	svc := &QueryService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := svc.Parse(tt.query)
			assert.Equal(t, tt.expectedType, parsed.Type)
			assert.Equal(t, tt.expectedProduct, parsed.ProductName)
		})
	}
}

func TestParseHierarchyQuery(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		expectedType    QueryType
		expectedProduct string
	}{
		{
			name:            "show hierarchy",
			query:           "show hierarchy of MTC",
			expectedType:    QueryTypeHierarchy,
			expectedProduct: "MTC",
		},
		{
			name:            "get structure",
			query:           "get the structure of Cloud Platform",
			expectedType:    QueryTypeHierarchy,
			expectedProduct: "Cloud Platform",
		},
		{
			name:            "show sub-products",
			query:           "what are the sub-products of LKE",
			expectedType:    QueryTypeHierarchy,
			expectedProduct: "LKE",
		},
		{
			name:            "show features",
			query:           "show the features of API Gateway",
			expectedType:    QueryTypeHierarchy,
			expectedProduct: "API Gateway",
		},
	}

	svc := &QueryService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := svc.Parse(tt.query)
			assert.Equal(t, tt.expectedType, parsed.Type)
			assert.Equal(t, tt.expectedProduct, parsed.ProductName)
		})
	}
}

func TestParseSearchQuery(t *testing.T) {
	tests := []struct {
		name             string
		query            string
		expectedType     QueryType
		expectedKeywords []string
	}{
		{
			name:             "simple search",
			query:            "kubernetes",
			expectedType:     QueryTypeSearch,
			expectedKeywords: []string{"kubernetes"},
		},
		{
			name:             "multi-word search",
			query:            "cloud storage",
			expectedType:     QueryTypeSearch,
			expectedKeywords: []string{"cloud", "storage"},
		},
		{
			name:             "removes stop words",
			query:            "find me the kubernetes",
			expectedType:     QueryTypeSearch,
			expectedKeywords: []string{"kubernetes"},
		},
	}

	svc := &QueryService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := svc.Parse(tt.query)
			assert.Equal(t, tt.expectedType, parsed.Type)
			assert.Equal(t, tt.expectedKeywords, parsed.Keywords)
		})
	}
}

// ==================== Role Normalization Tests ====================

func TestNormalizeRole(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"dri", "DRI"},
		{"DRI", "DRI"},
		{"manager", "Manager"},
		{"Manager", "Manager"},
		{"tech lead", "Tech Lead"},
		{"pm", "Product Manager"},
		{"em", "Engineering Manager"},
		{"engineering manager", "Engineering Manager"},
		{"contributor", "Contributor"},
		{"unknown role", "Unknown Role"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeRole(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ==================== Tokenization Tests ====================

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple words",
			input:    "cloud kubernetes",
			expected: []string{"cloud", "kubernetes"},
		},
		{
			name:     "removes stop words",
			input:    "the cloud and kubernetes",
			expected: []string{"cloud", "kubernetes"},
		},
		{
			name:     "removes punctuation",
			input:    "cloud! kubernetes.",
			expected: []string{"cloud", "kubernetes"},
		},
		{
			name:     "handles quotes",
			input:    `"cloud platform"`,
			expected: []string{"cloud", "platform"},
		},
		{
			name:     "removes short words",
			input:    "a i cloud",
			expected: []string{"cloud"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ==================== QueryType Constants Tests ====================

func TestQueryTypeConstants(t *testing.T) {
	assert.Equal(t, QueryType("search"), QueryTypeSearch)
	assert.Equal(t, QueryType("role"), QueryTypeRole)
	assert.Equal(t, QueryType("team"), QueryTypeTeam)
	assert.Equal(t, QueryType("timeline"), QueryTypeTimeline)
	assert.Equal(t, QueryType("hierarchy"), QueryTypeHierarchy)
}

// ==================== ParsedQuery Structure Tests ====================

func TestParsedQueryStructure(t *testing.T) {
	query := &ParsedQuery{
		Type:        QueryTypeRole,
		ProductName: "MTC",
		Role:        "DRI",
		Scope:       "Networking",
		Keywords:    []string{"cloud"},
		RawQuery:    "who is the DRI for networking on MTC",
	}

	assert.Equal(t, QueryTypeRole, query.Type)
	assert.Equal(t, "MTC", query.ProductName)
	assert.Equal(t, "DRI", query.Role)
	assert.Equal(t, "Networking", query.Scope)
	assert.Contains(t, query.Keywords, "cloud")
	assert.NotEmpty(t, query.RawQuery)
}

// ==================== PersonRoleInfo Structure Tests ====================

func TestPersonRoleInfoStructure(t *testing.T) {
	info := &PersonRoleInfo{
		PersonID:    123,
		PersonName:  "Alice Smith",
		PersonEmail: "alice@example.com",
		Role:        "DRI",
		Scope:       "Networking",
		TeamName:    "Platform Team",
		TeamContext: "Core Team",
		ProductName: "MTC",
	}

	assert.Equal(t, int64(123), info.PersonID)
	assert.Equal(t, "Alice Smith", info.PersonName)
	assert.Equal(t, "alice@example.com", info.PersonEmail)
	assert.Equal(t, "DRI", info.Role)
	assert.Equal(t, "Networking", info.Scope)
	assert.Equal(t, "Platform Team", info.TeamName)
	assert.Equal(t, "Core Team", info.TeamContext)
	assert.Equal(t, "MTC", info.ProductName)
}

// ==================== QueryResult Structure Tests ====================

func TestQueryResultStructure(t *testing.T) {
	result := &QueryResult{
		Type:    QueryTypeRole,
		Query:   &ParsedQuery{Type: QueryTypeRole},
		Message: "Found 1 person",
		Persons: []*PersonRoleInfo{
			{PersonName: "Alice"},
		},
	}

	assert.Equal(t, QueryTypeRole, result.Type)
	assert.NotNil(t, result.Query)
	assert.Equal(t, "Found 1 person", result.Message)
	assert.Len(t, result.Persons, 1)
}
