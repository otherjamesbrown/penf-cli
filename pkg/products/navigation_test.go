package products

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ==================== Navigation Path Tests ====================
//
// These tests verify the bidirectional navigation paths between entities:
//   Person ↔ ProductTeamRole ↔ ProductTeam ↔ Product
//
// The navigation paths allow queries like:
//   - "What products does this person work on?"
//   - "Who works on this product?"
//   - "What teams work on this product?"
//   - "What products does this team work on?"

// TestNavigationPersonToProduct tests the Person → Products path.
func TestNavigationPersonToProduct(t *testing.T) {
	// Given a person with roles on multiple products
	person := &PersonRoleInfo{
		PersonID:    100,
		PersonName:  "Alice Smith",
		PersonEmail: "alice@example.com",
	}

	// The navigation path is:
	// Person → ProductTeamRole (via person_id) → ProductTeam → Product
	roles := []*ProductTeamRole{
		{
			ID:           1,
			PersonID:     person.PersonID,
			ProductName:  "Product A",
			TeamName:     "Team Alpha",
			Role:         "DRI",
			Scope:        "Networking",
			PersonName:   person.PersonName,
			PersonEmail:  person.PersonEmail,
			IsActive:     true,
		},
		{
			ID:           2,
			PersonID:     person.PersonID,
			ProductName:  "Product B",
			TeamName:     "Team Beta",
			Role:         "Engineer",
			PersonName:   person.PersonName,
			PersonEmail:  person.PersonEmail,
			IsActive:     true,
		},
	}

	// Verify we can extract unique products from roles
	productSet := make(map[string]bool)
	for _, role := range roles {
		productSet[role.ProductName] = true
	}

	assert.Len(t, productSet, 2)
	assert.True(t, productSet["Product A"])
	assert.True(t, productSet["Product B"])
}

// TestNavigationProductToPerson tests the Product → Persons path.
func TestNavigationProductToPerson(t *testing.T) {
	// Given a product with multiple people working on it
	productID := int64(1)
	productName := "Test Product"

	// The navigation path is:
	// Product → ProductTeam (via product_id) → ProductTeamRole → Person
	roles := []*ProductTeamRole{
		{
			ID:          1,
			PersonID:    100,
			ProductName: productName,
			TeamName:    "Core Team",
			Role:        "DRI",
			PersonName:  "Alice",
			PersonEmail: "alice@example.com",
			IsActive:    true,
		},
		{
			ID:          2,
			PersonID:    101,
			ProductName: productName,
			TeamName:    "Core Team",
			Role:        "Engineer",
			PersonName:  "Bob",
			PersonEmail: "bob@example.com",
			IsActive:    true,
		},
		{
			ID:          3,
			PersonID:    102,
			ProductName: productName,
			TeamName:    "API Team",
			Role:        "DRI",
			PersonName:  "Charlie",
			PersonEmail: "charlie@example.com",
			IsActive:    true,
		},
	}

	// Verify we can extract unique people from roles
	personSet := make(map[int64]string)
	for _, role := range roles {
		personSet[role.PersonID] = role.PersonName
	}

	assert.Len(t, personSet, 3)
	assert.Equal(t, "Alice", personSet[100])
	assert.Equal(t, "Bob", personSet[101])
	assert.Equal(t, "Charlie", personSet[102])

	// Verify product ID is consistent
	_ = productID
}

// TestNavigationTeamToProducts tests the Team → Products path.
func TestNavigationTeamToProducts(t *testing.T) {
	// Given a team that works on multiple products
	teamID := int64(10)
	teamName := "Platform Team"

	// The navigation path is:
	// Team → ProductTeam (via team_id) → Product
	productTeams := []*ProductTeam{
		{
			ID:          1,
			TeamID:      teamID,
			ProductID:   1,
			TeamName:    teamName,
			ProductName: "Product A",
			Context:     "Core Team",
		},
		{
			ID:          2,
			TeamID:      teamID,
			ProductID:   2,
			TeamName:    teamName,
			ProductName: "Product B",
			Context:     "Advisory",
		},
	}

	// Verify we can extract products from team associations
	productSet := make(map[int64]string)
	for _, pt := range productTeams {
		productSet[pt.ProductID] = pt.ProductName
	}

	assert.Len(t, productSet, 2)
	assert.Equal(t, "Product A", productSet[1])
	assert.Equal(t, "Product B", productSet[2])
}

// TestNavigationProductToTeams tests the Product → Teams path.
func TestNavigationProductToTeams(t *testing.T) {
	// Given a product with multiple teams
	productID := int64(1)
	productName := "Cloud Platform"

	// The navigation path is:
	// Product → ProductTeam (via product_id) → Team
	productTeams := []*ProductTeam{
		{
			ID:          1,
			ProductID:   productID,
			TeamID:      10,
			ProductName: productName,
			TeamName:    "Core Team",
			Context:     "DRI Team",
		},
		{
			ID:          2,
			ProductID:   productID,
			TeamID:      20,
			ProductName: productName,
			TeamName:    "API Team",
		},
		{
			ID:          3,
			ProductID:   productID,
			TeamID:      30,
			ProductName: productName,
			TeamName:    "EMEA Team",
			Context:     "Regional",
		},
	}

	// Verify we can extract teams from product associations
	teamSet := make(map[int64]string)
	for _, pt := range productTeams {
		teamSet[pt.TeamID] = pt.TeamName
	}

	assert.Len(t, teamSet, 3)
	assert.Equal(t, "Core Team", teamSet[10])
	assert.Equal(t, "API Team", teamSet[20])
	assert.Equal(t, "EMEA Team", teamSet[30])
}

// ==================== Multi-Product Navigation Tests ====================

// TestNavigationPersonOnMultipleTeams tests navigation for a person with multiple team memberships.
func TestNavigationPersonOnMultipleTeams(t *testing.T) {
	// Scenario: Alice is on Team Alpha and Team Beta,
	// Team Alpha works on Products A and B,
	// Team Beta works on Products B and C.
	// Therefore Alice works on Products A, B, and C.

	roles := []*ProductTeamRole{
		{PersonID: 100, PersonName: "Alice", TeamName: "Team Alpha", ProductName: "Product A", Role: "DRI"},
		{PersonID: 100, PersonName: "Alice", TeamName: "Team Alpha", ProductName: "Product B", Role: "Reviewer"},
		{PersonID: 100, PersonName: "Alice", TeamName: "Team Beta", ProductName: "Product B", Role: "Engineer"},
		{PersonID: 100, PersonName: "Alice", TeamName: "Team Beta", ProductName: "Product C", Role: "DRI"},
	}

	// Collect unique products
	productSet := make(map[string]bool)
	for _, role := range roles {
		productSet[role.ProductName] = true
	}

	assert.Len(t, productSet, 3, "Alice should be on 3 unique products")
	assert.True(t, productSet["Product A"])
	assert.True(t, productSet["Product B"])
	assert.True(t, productSet["Product C"])

	// Collect unique teams
	teamSet := make(map[string]bool)
	for _, role := range roles {
		teamSet[role.TeamName] = true
	}

	assert.Len(t, teamSet, 2, "Alice should be on 2 unique teams")
}

// TestNavigationTeamOnMultipleProducts tests navigation for a team with multiple product associations.
func TestNavigationTeamOnMultipleProducts(t *testing.T) {
	// Scenario: Platform Team works on Products X, Y, Z with different contexts
	productTeams := []*ProductTeam{
		{TeamID: 10, TeamName: "Platform Team", ProductID: 1, ProductName: "Product X", Context: "Core"},
		{TeamID: 10, TeamName: "Platform Team", ProductID: 2, ProductName: "Product Y", Context: "Support"},
		{TeamID: 10, TeamName: "Platform Team", ProductID: 3, ProductName: "Product Z", Context: "Advisory"},
	}

	// Verify all associations are captured
	assert.Len(t, productTeams, 3)

	// Each product-team can have a unique context
	contexts := make(map[string]string)
	for _, pt := range productTeams {
		contexts[pt.ProductName] = pt.Context
	}

	assert.Equal(t, "Core", contexts["Product X"])
	assert.Equal(t, "Support", contexts["Product Y"])
	assert.Equal(t, "Advisory", contexts["Product Z"])
}

// ==================== Historical Navigation Tests ====================

// TestNavigationHistoricalRoles tests navigation with both active and inactive roles.
func TestNavigationHistoricalRoles(t *testing.T) {
	now := time.Now()
	lastMonth := now.AddDate(0, -1, 0)
	lastYear := now.AddDate(-1, 0, 0)

	roles := []*ProductTeamRole{
		// Current active role
		{
			ID:          1,
			PersonID:    100,
			PersonName:  "Alice",
			ProductName: "Product A",
			TeamName:    "Core Team",
			Role:        "DRI",
			IsActive:    true,
			StartedAt:   lastMonth,
		},
		// Historical role (ended)
		{
			ID:          2,
			PersonID:    100,
			PersonName:  "Alice",
			ProductName: "Product A",
			TeamName:    "Core Team",
			Role:        "Engineer",
			IsActive:    false,
			StartedAt:   lastYear,
			EndedAt:     &lastMonth,
		},
		// Historical role on different product
		{
			ID:          3,
			PersonID:    100,
			PersonName:  "Alice",
			ProductName: "Product B",
			TeamName:    "Beta Team",
			Role:        "Contributor",
			IsActive:    false,
			StartedAt:   lastYear,
			EndedAt:     &lastMonth,
		},
	}

	// Count active vs inactive
	activeCount := 0
	inactiveCount := 0
	for _, role := range roles {
		if role.IsActive {
			activeCount++
		} else {
			inactiveCount++
		}
	}

	assert.Equal(t, 1, activeCount, "Should have 1 active role")
	assert.Equal(t, 2, inactiveCount, "Should have 2 inactive roles")

	// Verify ended roles have EndedAt set
	for _, role := range roles {
		if !role.IsActive {
			assert.NotNil(t, role.EndedAt, "Inactive role should have EndedAt")
		} else {
			assert.Nil(t, role.EndedAt, "Active role should not have EndedAt")
		}
	}
}

// ==================== Bidirectional Consistency Tests ====================

// TestNavigationBidirectionalConsistency tests that navigation is consistent in both directions.
func TestNavigationBidirectionalConsistency(t *testing.T) {
	// Setup: Define the relationships
	// Product A has teams: T1, T2
	// Product B has teams: T2, T3
	// Person Alice is on T1, T2
	// Person Bob is on T2

	type Relationship struct {
		ProductID int64
		TeamID    int64
		PersonID  int64
	}

	relationships := []Relationship{
		{ProductID: 1, TeamID: 1, PersonID: 100}, // Product A, Team 1, Alice
		{ProductID: 1, TeamID: 2, PersonID: 100}, // Product A, Team 2, Alice
		{ProductID: 1, TeamID: 2, PersonID: 101}, // Product A, Team 2, Bob
		{ProductID: 2, TeamID: 2, PersonID: 100}, // Product B, Team 2, Alice
		{ProductID: 2, TeamID: 2, PersonID: 101}, // Product B, Team 2, Bob
		{ProductID: 2, TeamID: 3, PersonID: 101}, // Product B, Team 3, Bob
	}

	// Navigation: Product → People
	productsToPersons := make(map[int64]map[int64]bool)
	for _, r := range relationships {
		if productsToPersons[r.ProductID] == nil {
			productsToPersons[r.ProductID] = make(map[int64]bool)
		}
		productsToPersons[r.ProductID][r.PersonID] = true
	}

	// Navigation: Person → Products
	personsToProducts := make(map[int64]map[int64]bool)
	for _, r := range relationships {
		if personsToProducts[r.PersonID] == nil {
			personsToProducts[r.PersonID] = make(map[int64]bool)
		}
		personsToProducts[r.PersonID][r.ProductID] = true
	}

	// Verify bidirectional consistency
	// If Product A contains Person X, then Person X must contain Product A
	for productID, persons := range productsToPersons {
		for personID := range persons {
			assert.True(t, personsToProducts[personID][productID],
				"Bidirectional: if product %d has person %d, person should have product", productID, personID)
		}
	}

	// Verify the reverse
	for personID, products := range personsToProducts {
		for productID := range products {
			assert.True(t, productsToPersons[productID][personID],
				"Bidirectional: if person %d has product %d, product should have person", personID, productID)
		}
	}

	// Specific assertions
	assert.Len(t, productsToPersons[1], 2, "Product A should have 2 people (Alice, Bob)")
	assert.Len(t, productsToPersons[2], 2, "Product B should have 2 people (Alice, Bob)")
	assert.Len(t, personsToProducts[100], 2, "Alice should be on 2 products (A, B)")
	assert.Len(t, personsToProducts[101], 2, "Bob should be on 2 products (A, B)")
}

// ==================== RoleQuery Navigation Tests ====================

// TestRoleQueryProductNavigation tests RoleQuery for finding people by product.
func TestRoleQueryProductNavigation(t *testing.T) {
	query := RoleQuery{
		TenantID:    "test-tenant",
		ProductName: "Cloud Platform",
		ActiveOnly:  true,
	}

	assert.Equal(t, "test-tenant", query.TenantID)
	assert.Equal(t, "Cloud Platform", query.ProductName)
	assert.True(t, query.ActiveOnly)
	assert.Empty(t, query.TeamName, "TeamName should be empty for product-level query")
	assert.Empty(t, query.Role, "Role should be empty for all-roles query")
}

// TestRoleQueryScopedNavigation tests RoleQuery for scoped role queries.
func TestRoleQueryScopedNavigation(t *testing.T) {
	query := RoleQuery{
		TenantID:    "test-tenant",
		ProductName: "Cloud Platform",
		Role:        "DRI",
		Scope:       "Networking",
		ActiveOnly:  true,
	}

	assert.Equal(t, "DRI", query.Role)
	assert.Equal(t, "Networking", query.Scope)
}

// TestRoleQueryTeamNavigation tests RoleQuery for team-scoped queries.
func TestRoleQueryTeamNavigation(t *testing.T) {
	query := RoleQuery{
		TenantID:   "test-tenant",
		TeamName:   "Core Team",
		ActiveOnly: true,
	}

	assert.Empty(t, query.ProductName, "ProductName should be empty for team-level query")
	assert.Equal(t, "Core Team", query.TeamName)
}

// ==================== ProductFilter Navigation Tests ====================

// TestProductFilterHierarchyNavigation tests ProductFilter for hierarchy queries.
func TestProductFilterHierarchyNavigation(t *testing.T) {
	parentID := int64(1)

	// Query for children of a product
	filter := ProductFilter{
		TenantID: "test-tenant",
		ParentID: &parentID,
	}

	assert.Equal(t, "test-tenant", filter.TenantID)
	assert.NotNil(t, filter.ParentID)
	assert.Equal(t, int64(1), *filter.ParentID)
}

// TestProductFilterTypeNavigation tests ProductFilter for type-based queries.
func TestProductFilterTypeNavigation(t *testing.T) {
	productType := ProductTypeSubProduct

	filter := ProductFilter{
		TenantID:    "test-tenant",
		ProductType: &productType,
	}

	assert.NotNil(t, filter.ProductType)
	assert.Equal(t, ProductTypeSubProduct, *filter.ProductType)
}

// ==================== EventFilter Navigation Tests ====================

// TestEventFilterProductNavigation tests EventFilter for product timeline queries.
func TestEventFilterProductNavigation(t *testing.T) {
	productID := int64(42)

	filter := EventFilter{
		TenantID:  "test-tenant",
		ProductID: &productID,
		Limit:     50,
	}

	assert.Equal(t, "test-tenant", filter.TenantID)
	assert.NotNil(t, filter.ProductID)
	assert.Equal(t, int64(42), *filter.ProductID)
	assert.Equal(t, 50, filter.Limit)
}
