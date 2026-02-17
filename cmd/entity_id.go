// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseEntityID accepts both raw numeric IDs ("123") and prefixed string IDs
// ("ent-person-123", "ent-org-456") and returns the int64 ID.
//
// Supported formats:
//   - "123" -> 123
//   - "ent-person-123" -> 123
//   - "ent-org-456" -> 456
//   - "ent-project-789" -> 789
//
// Returns an error if the input is not a valid ID format.
func ParseEntityID(input string) (int64, error) {
	if input == "" {
		return 0, fmt.Errorf("entity ID cannot be empty")
	}

	// Check if input has the "ent-" prefix
	if strings.HasPrefix(input, "ent-") {
		// Split into parts: ["ent", "person", "123"]
		parts := strings.Split(input, "-")
		if len(parts) != 3 {
			return 0, fmt.Errorf("invalid entity ID format: %s (expected format: ent-type-id)", input)
		}

		// The third part should be the numeric ID
		idStr := parts[2]
		if idStr == "" {
			return 0, fmt.Errorf("invalid entity ID format: %s (missing numeric ID)", input)
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid entity ID format: %s (non-numeric ID: %s)", input, idStr)
		}

		return id, nil
	}

	// Try parsing as raw numeric ID
	id, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid entity ID: %s (must be numeric or format ent-type-id)", input)
	}

	return id, nil
}

// FormatEntityID formats an int64 ID into the prefixed string format "ent-{type}-{id}".
//
// Examples:
//   - FormatEntityID(123, "person") -> "ent-person-123"
//   - FormatEntityID(456, "org") -> "ent-org-456"
//   - FormatEntityID(789, "") -> "789" (empty type returns raw numeric string)
func FormatEntityID(id int64, entityType string) string {
	if entityType == "" {
		return fmt.Sprintf("%d", id)
	}
	return fmt.Sprintf("ent-%s-%d", entityType, id)
}
