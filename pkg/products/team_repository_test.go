package products

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== ProductTeam Struct Tests ====================

func TestProductTeamStructure(t *testing.T) {
	now := time.Now()
	team := &ProductTeam{
		ID:          1,
		TenantID:    "tenant-123",
		ProductID:   10,
		TeamID:      20,
		Context:     "Core Team",
		CreatedAt:   now,
		UpdatedAt:   now,
		ProductName: "Test Product",
		TeamName:    "Engineering",
	}

	assert.Equal(t, int64(1), team.ID)
	assert.Equal(t, "tenant-123", team.TenantID)
	assert.Equal(t, int64(10), team.ProductID)
	assert.Equal(t, int64(20), team.TeamID)
	assert.Equal(t, "Core Team", team.Context)
	assert.Equal(t, "Test Product", team.ProductName)
	assert.Equal(t, "Engineering", team.TeamName)
}

func TestProductTeamWithNoContext(t *testing.T) {
	team := &ProductTeam{
		ID:        1,
		TenantID:  "tenant-123",
		ProductID: 10,
		TeamID:    20,
	}

	assert.Empty(t, team.Context)
	assert.Empty(t, team.ProductName)
	assert.Empty(t, team.TeamName)
}

// ==================== ProductTeamRole Struct Tests ====================

func TestProductTeamRoleStructure(t *testing.T) {
	now := time.Now()
	endedAt := now.Add(24 * time.Hour)

	role := &ProductTeamRole{
		ID:            1,
		TenantID:      "tenant-123",
		ProductTeamID: 5,
		PersonID:      100,
		Role:          "DRI",
		Scope:         "Networking",
		IsActive:      true,
		StartedAt:     now,
		EndedAt:       &endedAt,
		CreatedAt:     now,
		UpdatedAt:     now,
		ProductName:   "Test Product",
		TeamName:      "Engineering",
		TeamContext:   "Core",
		PersonName:    "John Doe",
		PersonEmail:   "john@example.com",
	}

	assert.Equal(t, int64(1), role.ID)
	assert.Equal(t, "tenant-123", role.TenantID)
	assert.Equal(t, int64(5), role.ProductTeamID)
	assert.Equal(t, int64(100), role.PersonID)
	assert.Equal(t, "DRI", role.Role)
	assert.Equal(t, "Networking", role.Scope)
	assert.True(t, role.IsActive)
	require.NotNil(t, role.EndedAt)
	assert.Equal(t, endedAt, *role.EndedAt)
	assert.Equal(t, "Test Product", role.ProductName)
	assert.Equal(t, "Engineering", role.TeamName)
	assert.Equal(t, "Core", role.TeamContext)
	assert.Equal(t, "John Doe", role.PersonName)
	assert.Equal(t, "john@example.com", role.PersonEmail)
}

func TestProductTeamRoleActiveWithNoEndDate(t *testing.T) {
	now := time.Now()
	role := &ProductTeamRole{
		ID:            1,
		TenantID:      "tenant-123",
		ProductTeamID: 5,
		PersonID:      100,
		Role:          "Engineer",
		IsActive:      true,
		StartedAt:     now,
		EndedAt:       nil,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	assert.True(t, role.IsActive)
	assert.Nil(t, role.EndedAt)
}

func TestProductTeamRoleInactiveWithEndDate(t *testing.T) {
	now := time.Now()
	endedAt := now.Add(-24 * time.Hour)
	role := &ProductTeamRole{
		ID:            1,
		TenantID:      "tenant-123",
		ProductTeamID: 5,
		PersonID:      100,
		Role:          "Engineer",
		IsActive:      false,
		StartedAt:     now.Add(-30 * 24 * time.Hour),
		EndedAt:       &endedAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	assert.False(t, role.IsActive)
	require.NotNil(t, role.EndedAt)
	assert.True(t, role.EndedAt.Before(now))
}

func TestProductTeamRoleWithNoScope(t *testing.T) {
	role := &ProductTeamRole{
		ID:            1,
		TenantID:      "tenant-123",
		ProductTeamID: 5,
		PersonID:      100,
		Role:          "TL",
		IsActive:      true,
	}

	assert.Empty(t, role.Scope)
}

// ==================== RoleQuery Tests ====================

func TestRoleQueryDefaults(t *testing.T) {
	query := RoleQuery{}

	assert.Empty(t, query.TenantID)
	assert.Empty(t, query.ProductName)
	assert.Empty(t, query.TeamName)
	assert.Empty(t, query.Role)
	assert.Empty(t, query.Scope)
	assert.Empty(t, query.PersonName)
	assert.False(t, query.ActiveOnly)
}

func TestRoleQueryFullyPopulated(t *testing.T) {
	query := RoleQuery{
		TenantID:    "tenant-123",
		ProductName: "Test Product",
		TeamName:    "Engineering",
		Role:        "DRI",
		Scope:       "Networking",
		PersonName:  "John",
		ActiveOnly:  true,
	}

	assert.Equal(t, "tenant-123", query.TenantID)
	assert.Equal(t, "Test Product", query.ProductName)
	assert.Equal(t, "Engineering", query.TeamName)
	assert.Equal(t, "DRI", query.Role)
	assert.Equal(t, "Networking", query.Scope)
	assert.Equal(t, "John", query.PersonName)
	assert.True(t, query.ActiveOnly)
}

func TestRoleQueryPartialFilters(t *testing.T) {
	// Test filtering by role only
	query1 := RoleQuery{
		TenantID:   "tenant-123",
		Role:       "DRI",
		ActiveOnly: true,
	}
	assert.Equal(t, "DRI", query1.Role)
	assert.Empty(t, query1.ProductName)

	// Test filtering by product only
	query2 := RoleQuery{
		TenantID:    "tenant-123",
		ProductName: "LKE",
		ActiveOnly:  true,
	}
	assert.Equal(t, "LKE", query2.ProductName)
	assert.Empty(t, query2.Role)
}

// ==================== Common Role Types ====================

func TestCommonRoleTypes(t *testing.T) {
	// These are common role types used in the system
	commonRoles := []string{
		"DRI",
		"TL",
		"PM",
		"Engineer",
		"Reviewer",
		"Approver",
		"Manager",
	}

	for _, role := range commonRoles {
		t.Run(role, func(t *testing.T) {
			r := ProductTeamRole{Role: role}
			assert.Equal(t, role, r.Role)
			assert.NotEmpty(t, r.Role)
		})
	}
}

// ==================== Context Values ====================

func TestCommonContextValues(t *testing.T) {
	// These are common context values used for team associations
	contexts := []string{
		"Core Team",
		"EMEA",
		"APAC",
		"Americas",
		"API Team",
		"Platform",
	}

	for _, ctx := range contexts {
		t.Run(ctx, func(t *testing.T) {
			team := ProductTeam{Context: ctx}
			assert.Equal(t, ctx, team.Context)
		})
	}
}

// ==================== Scope Values ====================

func TestCommonScopeValues(t *testing.T) {
	// These are common scope values used for role assignments
	scopes := []string{
		"Networking",
		"Database",
		"Security",
		"Storage",
		"Compute",
		"API",
	}

	for _, scope := range scopes {
		t.Run(scope, func(t *testing.T) {
			role := ProductTeamRole{Scope: scope}
			assert.Equal(t, scope, role.Scope)
		})
	}
}
