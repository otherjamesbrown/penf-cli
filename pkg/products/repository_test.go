package products

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== Product Type Constants ====================

func TestProductTypeConstants(t *testing.T) {
	assert.Equal(t, ProductType("product"), ProductTypeProduct)
	assert.Equal(t, ProductType("sub_product"), ProductTypeSubProduct)
	assert.Equal(t, ProductType("feature"), ProductTypeFeature)
}

func TestProductStatusConstants(t *testing.T) {
	assert.Equal(t, ProductStatus("active"), ProductStatusActive)
	assert.Equal(t, ProductStatus("beta"), ProductStatusBeta)
	assert.Equal(t, ProductStatus("sunset"), ProductStatusSunset)
	assert.Equal(t, ProductStatus("deprecated"), ProductStatusDeprecated)
}

// ==================== Product Struct Tests ====================

func TestProductStructure(t *testing.T) {
	now := time.Now()
	parentID := int64(1)
	desc := "Test description"

	product := &Product{
		ID:          42,
		TenantID:    "tenant-123",
		Name:        "Test Product",
		Description: &desc,
		ParentID:    &parentID,
		ProductType: ProductTypeProduct,
		Status:      ProductStatusActive,
		Keywords:    []string{"test", "example"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	assert.Equal(t, int64(42), product.ID)
	assert.Equal(t, "tenant-123", product.TenantID)
	assert.Equal(t, "Test Product", product.Name)
	require.NotNil(t, product.Description)
	assert.Equal(t, "Test description", *product.Description)
	require.NotNil(t, product.ParentID)
	assert.Equal(t, int64(1), *product.ParentID)
	assert.Equal(t, ProductTypeProduct, product.ProductType)
	assert.Equal(t, ProductStatusActive, product.Status)
	assert.Len(t, product.Keywords, 2)
	assert.Contains(t, product.Keywords, "test")
}

func TestProductWithNilFields(t *testing.T) {
	product := &Product{
		ID:          1,
		TenantID:    "tenant-123",
		Name:        "Minimal Product",
		ProductType: ProductTypeFeature,
		Status:      ProductStatusBeta,
	}

	assert.Nil(t, product.Description)
	assert.Nil(t, product.ParentID)
	assert.Nil(t, product.Keywords)
	assert.Nil(t, product.Parent)
	assert.Nil(t, product.Children)
	assert.Nil(t, product.Aliases)
}

func TestProductRelationships(t *testing.T) {
	parent := &Product{ID: 1, Name: "Parent"}
	child1 := &Product{ID: 2, Name: "Child1"}
	child2 := &Product{ID: 3, Name: "Child2"}
	alias := &ProductAlias{ID: 1, ProductID: 1, Alias: "alias1"}

	parent.Children = []*Product{child1, child2}
	parent.Aliases = []*ProductAlias{alias}
	child1.Parent = parent

	assert.Len(t, parent.Children, 2)
	assert.Len(t, parent.Aliases, 1)
	assert.Equal(t, "Parent", child1.Parent.Name)
}

// ==================== ProductAlias Tests ====================

func TestProductAliasStructure(t *testing.T) {
	now := time.Now()
	alias := &ProductAlias{
		ID:        1,
		ProductID: 42,
		Alias:     "MyAlias",
		CreatedAt: now,
	}

	assert.Equal(t, int64(1), alias.ID)
	assert.Equal(t, int64(42), alias.ProductID)
	assert.Equal(t, "MyAlias", alias.Alias)
	assert.Equal(t, now, alias.CreatedAt)
}

// ==================== ProductWithHierarchy Tests ====================

func TestProductWithHierarchyStructure(t *testing.T) {
	product := &Product{
		ID:          1,
		Name:        "Child Product",
		ProductType: ProductTypeSubProduct,
	}

	hierarchy := &ProductWithHierarchy{
		Product: product,
		Depth:   2,
		Path:    "Parent > Child > Child Product",
	}

	assert.Equal(t, "Child Product", hierarchy.Name)
	assert.Equal(t, 2, hierarchy.Depth)
	assert.Equal(t, "Parent > Child > Child Product", hierarchy.Path)
}

func TestProductWithHierarchyRootLevel(t *testing.T) {
	product := &Product{
		ID:          1,
		Name:        "Root Product",
		ProductType: ProductTypeProduct,
	}

	hierarchy := &ProductWithHierarchy{
		Product: product,
		Depth:   0,
		Path:    "Root Product",
	}

	assert.Equal(t, 0, hierarchy.Depth)
	assert.Equal(t, "Root Product", hierarchy.Path)
}

// ==================== ProductFilter Tests ====================

func TestProductFilterDefaults(t *testing.T) {
	filter := ProductFilter{}

	assert.Empty(t, filter.TenantID)
	assert.Nil(t, filter.ParentID)
	assert.Nil(t, filter.ProductType)
	assert.Nil(t, filter.Status)
	assert.Empty(t, filter.NameSearch)
	assert.Equal(t, 0, filter.Limit)
	assert.Equal(t, 0, filter.Offset)
}

func TestProductFilterWithAllFields(t *testing.T) {
	parentID := int64(10)
	prodType := ProductTypeSubProduct
	status := ProductStatusActive

	filter := ProductFilter{
		TenantID:    "tenant-123",
		ParentID:    &parentID,
		ProductType: &prodType,
		Status:      &status,
		NameSearch:  "search term",
		Limit:       50,
		Offset:      100,
	}

	assert.Equal(t, "tenant-123", filter.TenantID)
	require.NotNil(t, filter.ParentID)
	assert.Equal(t, int64(10), *filter.ParentID)
	require.NotNil(t, filter.ProductType)
	assert.Equal(t, ProductTypeSubProduct, *filter.ProductType)
	require.NotNil(t, filter.Status)
	assert.Equal(t, ProductStatusActive, *filter.Status)
	assert.Equal(t, "search term", filter.NameSearch)
	assert.Equal(t, 50, filter.Limit)
	assert.Equal(t, 100, filter.Offset)
}

// ==================== Error Constants Tests ====================

func TestErrorConstants(t *testing.T) {
	// Error constants now use centralized domain errors from pkg/errors
	assert.Equal(t, "not found", ErrNotFound.Error())
	assert.Equal(t, "conflict", ErrAliasConflict.Error())
}

// ==================== Helper Function Tests ====================

func TestNullIfEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *string
	}{
		{"empty string", "", nil},
		{"non-empty string", "hello", strPtr("hello")},
		{"whitespace only", "  ", strPtr("  ")}, // whitespace is not empty
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := nullIfEmpty(tc.input)
			if tc.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tc.expected, *result)
			}
		})
	}
}

// strPtr is a helper to create string pointers for tests.
func strPtr(s string) *string {
	return &s
}

// int64Ptr is a helper to create int64 pointers for tests.
func int64Ptr(i int64) *int64 {
	return &i
}
