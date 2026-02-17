package entities

import (
	"testing"
)

// TestEntitySimilarity_ExactMatch verifies that identical entities return a high similarity score.
func TestEntitySimilarity_ExactMatch(t *testing.T) {
	// Arrange
	entity1 := &EntityComparisonData{
		Name:      "Rick Eskelsen",
		Email:     "rick.eskelsen@company.com",
		Domain:    "company.com",
		SourceIDs: []string{"src-1", "src-2"},
	}
	entity2 := &EntityComparisonData{
		Name:      "Rick Eskelsen",
		Email:     "rick.eskelsen@company.com",
		Domain:    "company.com",
		SourceIDs: []string{"src-1", "src-2"},
	}

	// Act
	similarity := EntitySimilarity(entity1, entity2)

	// Assert
	if similarity < 0.95 {
		t.Errorf("EntitySimilarity(%+v, %+v) = %v, expected >= 0.95 for exact match", entity1, entity2, similarity)
	}
}

// TestEntitySimilarity_NameOnly verifies that similar names with different emails return a moderate score.
func TestEntitySimilarity_NameOnly(t *testing.T) {
	// Arrange
	entity1 := &EntityComparisonData{
		Name:      "James Brown",
		Email:     "james.brown@company.com",
		Domain:    "company.com",
		SourceIDs: []string{"src-1"},
	}
	entity2 := &EntityComparisonData{
		Name:      "James Brown",
		Email:     "jbrown@other.com",
		Domain:    "other.com",
		SourceIDs: []string{"src-2"},
	}

	// Act
	similarity := EntitySimilarity(entity1, entity2)

	// Assert
	// High name similarity (1.0) but no domain bonus or source bonus
	// Should be primarily driven by name similarity weight
	if similarity < 0.7 || similarity > 0.85 {
		t.Errorf("EntitySimilarity(%+v, %+v) = %v, expected between 0.7 and 0.85 for name-only match", entity1, entity2, similarity)
	}
}

// TestEntitySimilarity_DomainBonus verifies that matching email domains contribute a bonus to similarity.
func TestEntitySimilarity_DomainBonus(t *testing.T) {
	// Arrange
	entity1 := &EntityComparisonData{
		Name:      "John Smith",
		Email:     "john.smith@company.com",
		Domain:    "company.com",
		SourceIDs: []string{"src-1"},
	}
	entity2 := &EntityComparisonData{
		Name:      "Jonathan Smith",
		Email:     "jonathan.smith@company.com",
		Domain:    "company.com",
		SourceIDs: []string{"src-2"},
	}

	// Act
	similarity := EntitySimilarity(entity1, entity2)

	// Assert
	// "John Smith" vs "Jonathan Smith" yields ~0.57 name similarity.
	// With domain bonus (0.6 weight), combined score should be >= 0.70.
	// Also verify domain bonus actually increases the score vs name-only.
	nameOnly := NameSimilarity(entity1.Name, entity2.Name)
	if similarity <= nameOnly {
		t.Errorf("EntitySimilarity with matching domain (%v) should be > name-only score (%v)", similarity, nameOnly)
	}
	if similarity < 0.70 {
		t.Errorf("EntitySimilarity(%+v, %+v) = %v, expected >= 0.70 with domain bonus", entity1, entity2, similarity)
	}
}

// TestEntitySimilarity_SharedSources verifies that shared source IDs contribute a bonus to similarity.
func TestEntitySimilarity_SharedSources(t *testing.T) {
	// Arrange
	entity1 := &EntityComparisonData{
		Name:      "Rick Eskelsen",
		Email:     "rick@company.com",
		Domain:    "company.com",
		SourceIDs: []string{"src-1", "src-2", "src-3"},
	}
	entity2 := &EntityComparisonData{
		Name:      "Rick E",
		Email:     "reskelsen@company.com",
		Domain:    "company.com",
		SourceIDs: []string{"src-2", "src-3", "src-4"},
	}

	// Act
	similarity := EntitySimilarity(entity1, entity2)

	// Assert
	// Similar names + matching domain + shared sources (0.2 bonus)
	// Should include the shared source bonus on top of other signals
	if similarity < 0.85 {
		t.Errorf("EntitySimilarity(%+v, %+v) = %v, expected >= 0.85 with shared sources", entity1, entity2, similarity)
	}
}

// TestEntitySimilarity_DifferentEntities verifies that completely different entities return a low score.
func TestEntitySimilarity_DifferentEntities(t *testing.T) {
	// Arrange
	entity1 := &EntityComparisonData{
		Name:      "Alice Johnson",
		Email:     "alice.johnson@company1.com",
		Domain:    "company1.com",
		SourceIDs: []string{"src-1"},
	}
	entity2 := &EntityComparisonData{
		Name:      "Bob Williams",
		Email:     "bob.williams@company2.com",
		Domain:    "company2.com",
		SourceIDs: []string{"src-2"},
	}

	// Act
	similarity := EntitySimilarity(entity1, entity2)

	// Assert
	if similarity > 0.3 {
		t.Errorf("EntitySimilarity(%+v, %+v) = %v, expected <= 0.3 for different entities", entity1, entity2, similarity)
	}
}

// TestEntitySimilarity_EmptyData verifies that empty or incomplete data is handled gracefully.
func TestEntitySimilarity_EmptyData(t *testing.T) {
	tests := []struct {
		name     string
		entity1  *EntityComparisonData
		entity2  *EntityComparisonData
		expected float64
	}{
		{
			name: "nil entities",
			entity1: nil,
			entity2: nil,
			expected: 0.0,
		},
		{
			name: "empty name",
			entity1: &EntityComparisonData{
				Name:      "",
				Email:     "test@example.com",
				Domain:    "example.com",
				SourceIDs: []string{"src-1"},
			},
			entity2: &EntityComparisonData{
				Name:      "John Doe",
				Email:     "john@example.com",
				Domain:    "example.com",
				SourceIDs: []string{"src-1"},
			},
			expected: 0.0,
		},
		{
			name: "no sources",
			entity1: &EntityComparisonData{
				Name:      "John Doe",
				Email:     "john@example.com",
				Domain:    "example.com",
				SourceIDs: []string{},
			},
			entity2: &EntityComparisonData{
				Name:      "John Doe",
				Email:     "john@example.com",
				Domain:    "example.com",
				SourceIDs: []string{},
			},
			expected: 0.9, // Should still match on name and domain
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			similarity := EntitySimilarity(tt.entity1, tt.entity2)
			if similarity < tt.expected-0.1 || similarity > tt.expected+0.1 {
				t.Errorf("EntitySimilarity() = %v, expected ~%v for %s", similarity, tt.expected, tt.name)
			}
		})
	}
}
