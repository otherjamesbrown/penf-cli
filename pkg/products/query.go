// Package products provides product management functionality.
package products

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

// QueryType represents the type of product query.
type QueryType string

const (
	// QueryTypeSearch searches for products by name/keywords.
	QueryTypeSearch QueryType = "search"
	// QueryTypeRole finds a person with a specific role on a product.
	QueryTypeRole QueryType = "role"
	// QueryTypeTeam finds teams associated with a product.
	QueryTypeTeam QueryType = "team"
	// QueryTypeTimeline retrieves timeline events for a product.
	QueryTypeTimeline QueryType = "timeline"
	// QueryTypeHierarchy retrieves product hierarchy information.
	QueryTypeHierarchy QueryType = "hierarchy"
)

// ParsedQuery represents a parsed natural language query.
type ParsedQuery struct {
	Type        QueryType
	ProductName string   // Product to query about
	Role        string   // Role to find (e.g., "DRI", "Manager")
	Scope       string   // Optional scope (e.g., "Networking", "Database")
	Keywords    []string // Keywords for search
	RawQuery    string   // Original query
}

// QueryResult represents the result of a product query.
type QueryResult struct {
	Type      QueryType
	Query     *ParsedQuery
	Products  []*Product        // For search results
	Persons   []*PersonRoleInfo // For role queries
	Teams     []*ProductTeam    // For team queries
	Events    []*ProductEvent   // For timeline queries
	Hierarchy *ProductWithHierarchy
	Message   string // Human-readable summary
}

// PersonRoleInfo represents a person with their role context.
type PersonRoleInfo struct {
	PersonID    int64
	PersonName  string
	PersonEmail string
	Role        string
	Scope       string
	TeamName    string
	TeamContext string
	ProductName string
}

// QueryService provides natural language query capabilities for products.
type QueryService struct {
	repo   *Repository
	pool   *pgxpool.Pool
	logger logging.Logger
}

// NewQueryService creates a new product query service.
func NewQueryService(pool *pgxpool.Pool, logger logging.Logger) *QueryService {
	return &QueryService{
		repo:   NewRepository(pool, logger),
		pool:   pool,
		logger: logger.With(logging.F("component", "product_query")),
	}
}

// rolePatterns matches common role query patterns.
var rolePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)who\s+is\s+(?:the\s+)?(\w+(?:\s+\w+)?)\s+(?:for|on|of)\s+(?:the\s+)?(.+?)(?:\s+(?:on|for)\s+(.+))?$`),
	regexp.MustCompile(`(?i)(?:find|show|get)\s+(?:me\s+)?(?:the\s+)?(\w+(?:\s+\w+)?)\s+(?:for|on|of)\s+(?:the\s+)?(.+?)(?:\s+(?:on|for)\s+(.+))?$`),
	regexp.MustCompile(`(?i)(\w+(?:\s+\w+)?)\s+(?:for|on|of)\s+(?:the\s+)?(.+)$`),
}

// teamPatterns matches team query patterns.
var teamPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:which|what)\s+teams?\s+(?:work|are)\s+on\s+(?:the\s+)?(.+)$`),
	regexp.MustCompile(`(?i)(?:show|list|get)\s+(?:me\s+)?teams?\s+(?:for|on)\s+(?:the\s+)?(.+)$`),
	regexp.MustCompile(`(?i)teams?\s+(?:for|on)\s+(?:the\s+)?(.+)$`),
}

// timelinePatterns matches timeline query patterns.
var timelinePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:show|get|display)\s+(?:me\s+)?(?:the\s+)?timeline\s+(?:for|of)\s+(?:the\s+)?(.+)$`),
	regexp.MustCompile(`(?i)(?:show|get|display)\s+(?:me\s+)?(?:the\s+)?(.+?)\s+timeline$`),
	regexp.MustCompile(`(?i)(?:what|show)\s+(?:has\s+)?happened\s+(?:with|on|to)\s+(?:the\s+)?(.+)$`),
	regexp.MustCompile(`(?i)events?\s+(?:for|on)\s+(?:the\s+)?(.+)$`),
	regexp.MustCompile(`(?i)(?:show|get|display)\s+(?:me\s+)?(?:the\s+)?(?:external\s+)?events?\s+affecting\s+(?:the\s+)?(.+)$`),
}

// hierarchyPatterns matches hierarchy query patterns.
var hierarchyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:show|get|display)\s+(?:me\s+)?(?:the\s+)?(?:structure|hierarchy)\s+(?:of|for)\s+(?:the\s+)?(.+)$`),
	regexp.MustCompile(`(?i)(?:what|show)\s+(?:are\s+)?(?:the\s+)?(?:sub-?products?|features?|children)\s+(?:of|for|under)\s+(?:the\s+)?(.+)$`),
}

// Parse parses a natural language query about products.
func (s *QueryService) Parse(query string) *ParsedQuery {
	query = strings.TrimSpace(query)

	// Try more specific patterns first (team, timeline, hierarchy)
	// before the more general role patterns

	// Try team patterns
	for _, pattern := range teamPatterns {
		if matches := pattern.FindStringSubmatch(query); len(matches) >= 2 {
			return &ParsedQuery{
				Type:        QueryTypeTeam,
				ProductName: strings.TrimSpace(matches[1]),
				RawQuery:    query,
			}
		}
	}

	// Try timeline patterns
	for _, pattern := range timelinePatterns {
		if matches := pattern.FindStringSubmatch(query); len(matches) >= 2 {
			return &ParsedQuery{
				Type:        QueryTypeTimeline,
				ProductName: strings.TrimSpace(matches[1]),
				RawQuery:    query,
			}
		}
	}

	// Try hierarchy patterns
	for _, pattern := range hierarchyPatterns {
		if matches := pattern.FindStringSubmatch(query); len(matches) >= 2 {
			return &ParsedQuery{
				Type:        QueryTypeHierarchy,
				ProductName: strings.TrimSpace(matches[1]),
				RawQuery:    query,
			}
		}
	}

	// Try role patterns (broader, so checked after specific patterns)
	for _, pattern := range rolePatterns {
		if matches := pattern.FindStringSubmatch(query); len(matches) >= 3 {
			parsed := &ParsedQuery{
				Type:     QueryTypeRole,
				Role:     normalizeRole(matches[1]),
				RawQuery: query,
			}
			// matches[2] could be "product" or "scope on product"
			productPart := matches[2]
			if len(matches) > 3 && matches[3] != "" {
				parsed.Scope = strings.TrimSpace(matches[2])
				parsed.ProductName = strings.TrimSpace(matches[3])
			} else {
				// Check if productPart contains "on" or "for" indicating scope
				if scopeMatch := regexp.MustCompile(`(?i)^(.+?)\s+on\s+(.+)$`).FindStringSubmatch(productPart); len(scopeMatch) == 3 {
					parsed.Scope = strings.TrimSpace(scopeMatch[1])
					parsed.ProductName = strings.TrimSpace(scopeMatch[2])
				} else {
					parsed.ProductName = strings.TrimSpace(productPart)
				}
			}
			return parsed
		}
	}

	// Default to keyword search
	return &ParsedQuery{
		Type:     QueryTypeSearch,
		Keywords: tokenize(query),
		RawQuery: query,
	}
}

// normalizeRole normalizes role names to match database values.
func normalizeRole(role string) string {
	role = strings.TrimSpace(strings.ToLower(role))

	// Map common variations to canonical names
	roleMap := map[string]string{
		"dri":               "DRI",
		"directly responsible individual": "DRI",
		"manager":           "Manager",
		"lead":              "Lead",
		"tech lead":         "Tech Lead",
		"engineering lead":  "Engineering Lead",
		"em":                "Engineering Manager",
		"engineering manager": "Engineering Manager",
		"pm":                "Product Manager",
		"product manager":   "Product Manager",
		"contributor":       "Contributor",
	}

	if canonical, ok := roleMap[role]; ok {
		return canonical
	}

	// Title case if not found
	return strings.Title(role)
}

// tokenize splits a query into keywords.
func tokenize(query string) []string {
	// Remove common stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"for": true, "on": true, "of": true, "to": true, "in": true,
		"with": true, "and": true, "or": true, "what": true, "who": true,
		"where": true, "which": true, "show": true, "me": true, "find": true,
		"get": true, "list": true, "about": true,
	}

	words := strings.Fields(strings.ToLower(query))
	var keywords []string
	for _, word := range words {
		word = strings.Trim(word, ".,!?\"'")
		if len(word) > 1 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}
	return keywords
}

// Execute executes a parsed query and returns results.
func (s *QueryService) Execute(ctx context.Context, tenantID string, query *ParsedQuery) (*QueryResult, error) {
	result := &QueryResult{
		Type:  query.Type,
		Query: query,
	}

	switch query.Type {
	case QueryTypeRole:
		return s.executeRoleQuery(ctx, tenantID, query, result)
	case QueryTypeTeam:
		return s.executeTeamQuery(ctx, tenantID, query, result)
	case QueryTypeTimeline:
		return s.executeTimelineQuery(ctx, tenantID, query, result)
	case QueryTypeHierarchy:
		return s.executeHierarchyQuery(ctx, tenantID, query, result)
	case QueryTypeSearch:
		return s.executeSearchQuery(ctx, tenantID, query, result)
	default:
		return nil, fmt.Errorf("unknown query type: %s", query.Type)
	}
}

// executeRoleQuery finds people with a specific role on a product.
func (s *QueryService) executeRoleQuery(ctx context.Context, tenantID string, query *ParsedQuery, result *QueryResult) (*QueryResult, error) {
	// First, resolve the product
	product, err := s.repo.ResolveProduct(ctx, tenantID, query.ProductName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve product: %w", err)
	}
	if product == nil {
		result.Message = fmt.Sprintf("Product '%s' not found", query.ProductName)
		return result, nil
	}

	// Find people with the specified role
	roleQuery := RoleQuery{
		TenantID:    tenantID,
		ProductName: product.Name,
		Role:        query.Role,
		Scope:       query.Scope,
		ActiveOnly:  true,
	}

	roles, err := s.repo.FindByRole(ctx, roleQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to find roles: %w", err)
	}

	// Convert to PersonRoleInfo
	var persons []*PersonRoleInfo
	for _, role := range roles {
		persons = append(persons, &PersonRoleInfo{
			PersonID:    role.PersonID,
			PersonName:  role.PersonName,
			PersonEmail: role.PersonEmail,
			Role:        role.Role,
			Scope:       role.Scope,
			TeamName:    role.TeamName,
			TeamContext: role.TeamContext,
			ProductName: product.Name,
		})
	}
	result.Persons = persons

	// Build message
	if len(persons) == 0 {
		if query.Scope != "" {
			result.Message = fmt.Sprintf("No %s found for %s on %s", query.Role, query.Scope, product.Name)
		} else {
			result.Message = fmt.Sprintf("No %s found for %s", query.Role, product.Name)
		}
	} else if len(persons) == 1 {
		p := persons[0]
		if query.Scope != "" {
			result.Message = fmt.Sprintf("%s (%s) is the %s for %s on %s",
				p.PersonName, p.PersonEmail, p.Role, query.Scope, product.Name)
		} else {
			result.Message = fmt.Sprintf("%s (%s) is the %s for %s",
				p.PersonName, p.PersonEmail, p.Role, product.Name)
		}
	} else {
		var names []string
		for _, p := range persons {
			names = append(names, fmt.Sprintf("%s (%s)", p.PersonName, p.PersonEmail))
		}
		if query.Scope != "" {
			result.Message = fmt.Sprintf("Found %d people with %s role for %s on %s: %s",
				len(persons), query.Role, query.Scope, product.Name, strings.Join(names, ", "))
		} else {
			result.Message = fmt.Sprintf("Found %d people with %s role on %s: %s",
				len(persons), query.Role, product.Name, strings.Join(names, ", "))
		}
	}

	return result, nil
}

// executeTeamQuery finds teams associated with a product.
func (s *QueryService) executeTeamQuery(ctx context.Context, tenantID string, query *ParsedQuery, result *QueryResult) (*QueryResult, error) {
	// Resolve the product
	product, err := s.repo.ResolveProduct(ctx, tenantID, query.ProductName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve product: %w", err)
	}
	if product == nil {
		result.Message = fmt.Sprintf("Product '%s' not found", query.ProductName)
		return result, nil
	}

	// Get teams for this product
	teams, err := s.repo.GetTeamsForProduct(ctx, product.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get product teams: %w", err)
	}

	result.Teams = teams

	// Build message
	if len(teams) == 0 {
		result.Message = fmt.Sprintf("No teams found for %s", product.Name)
	} else {
		var teamDescs []string
		for _, t := range teams {
			if t.Context != "" {
				teamDescs = append(teamDescs, fmt.Sprintf("%s (%s)", t.TeamName, t.Context))
			} else {
				teamDescs = append(teamDescs, t.TeamName)
			}
		}
		result.Message = fmt.Sprintf("Teams working on %s: %s", product.Name, strings.Join(teamDescs, ", "))
	}

	return result, nil
}

// executeTimelineQuery retrieves timeline events for a product.
func (s *QueryService) executeTimelineQuery(ctx context.Context, tenantID string, query *ParsedQuery, result *QueryResult) (*QueryResult, error) {
	// Resolve the product
	product, err := s.repo.ResolveProduct(ctx, tenantID, query.ProductName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve product: %w", err)
	}
	if product == nil {
		result.Message = fmt.Sprintf("Product '%s' not found", query.ProductName)
		return result, nil
	}

	// Get events
	filter := EventFilter{
		TenantID:  tenantID,
		ProductID: &product.ID,
		Limit:     20,
	}

	events, err := s.repo.ListEvents(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	result.Events = events

	// Build message
	if len(events) == 0 {
		result.Message = fmt.Sprintf("No timeline events found for %s", product.Name)
	} else {
		result.Message = fmt.Sprintf("Found %d timeline events for %s", len(events), product.Name)
	}

	return result, nil
}

// executeHierarchyQuery retrieves product hierarchy information.
func (s *QueryService) executeHierarchyQuery(ctx context.Context, tenantID string, query *ParsedQuery, result *QueryResult) (*QueryResult, error) {
	// Resolve the product
	product, err := s.repo.ResolveProduct(ctx, tenantID, query.ProductName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve product: %w", err)
	}
	if product == nil {
		result.Message = fmt.Sprintf("Product '%s' not found", query.ProductName)
		return result, nil
	}

	// Get hierarchy
	hierarchy, err := s.repo.GetHierarchy(ctx, product.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get hierarchy: %w", err)
	}

	if len(hierarchy) == 0 {
		result.Message = fmt.Sprintf("No hierarchy information found for %s", product.Name)
		return result, nil
	}

	// The first item is the product itself
	result.Hierarchy = hierarchy[0]

	// Build message
	var childTypes []string
	subProducts := 0
	features := 0
	for _, h := range hierarchy {
		if h.ID == product.ID {
			continue
		}
		if h.ProductType == "sub_product" {
			subProducts++
		} else if h.ProductType == "feature" {
			features++
		}
	}
	if subProducts > 0 {
		childTypes = append(childTypes, fmt.Sprintf("%d sub-product(s)", subProducts))
	}
	if features > 0 {
		childTypes = append(childTypes, fmt.Sprintf("%d feature(s)", features))
	}

	if len(childTypes) == 0 {
		result.Message = fmt.Sprintf("%s has no sub-products or features", product.Name)
	} else {
		result.Message = fmt.Sprintf("%s has %s", product.Name, strings.Join(childTypes, " and "))
	}

	return result, nil
}

// executeSearchQuery searches for products by keywords.
func (s *QueryService) executeSearchQuery(ctx context.Context, tenantID string, query *ParsedQuery, result *QueryResult) (*QueryResult, error) {
	// Build a search filter using keywords
	filter := ProductFilter{
		TenantID: tenantID,
		Limit:    20,
	}

	// If we have keywords, search in name and keywords
	if len(query.Keywords) > 0 {
		filter.NameSearch = strings.Join(query.Keywords, " ")
	}

	products, err := s.repo.ListProducts(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to search products: %w", err)
	}

	result.Products = products

	// Build message
	if len(products) == 0 {
		result.Message = fmt.Sprintf("No products found matching '%s'", query.RawQuery)
	} else {
		var names []string
		for _, p := range products {
			names = append(names, p.Name)
		}
		result.Message = fmt.Sprintf("Found %d product(s): %s", len(products), strings.Join(names, ", "))
	}

	return result, nil
}

// Query is a convenience method that parses and executes a query.
func (s *QueryService) Query(ctx context.Context, tenantID, queryStr string) (*QueryResult, error) {
	parsed := s.Parse(queryStr)
	return s.Execute(ctx, tenantID, parsed)
}
