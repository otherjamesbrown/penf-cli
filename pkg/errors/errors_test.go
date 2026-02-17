package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct match", ErrNotFound, true},
		{"wrapped once", fmt.Errorf("get user: %w", ErrNotFound), true},
		{"wrapped twice", fmt.Errorf("service: %w", fmt.Errorf("repo: %w", ErrNotFound)), true},
		{"different error", ErrConflict, false},
		{"nil error", nil, false},
		{"unrelated error", errors.New("something else"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConflict(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct match", ErrConflict, true},
		{"wrapped", fmt.Errorf("create: %w", ErrConflict), true},
		{"different error", ErrNotFound, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConflict(tt.err); got != tt.want {
				t.Errorf("IsConflict() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct match", ErrValidation, true},
		{"wrapped", fmt.Errorf("input: %w", ErrValidation), true},
		{"different error", ErrNotFound, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidation(tt.err); got != tt.want {
				t.Errorf("IsValidation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUnauthorized(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct match", ErrUnauthorized, true},
		{"wrapped", fmt.Errorf("auth: %w", ErrUnauthorized), true},
		{"different error", ErrForbidden, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnauthorized(tt.err); got != tt.want {
				t.Errorf("IsUnauthorized() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsForbidden(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct match", ErrForbidden, true},
		{"wrapped", fmt.Errorf("access: %w", ErrForbidden), true},
		{"different error", ErrUnauthorized, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsForbidden(tt.err); got != tt.want {
				t.Errorf("IsForbidden() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAlreadyExists(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct match", ErrAlreadyExists, true},
		{"wrapped", fmt.Errorf("insert: %w", ErrAlreadyExists), true},
		{"different error", ErrConflict, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAlreadyExists(tt.err); got != tt.want {
				t.Errorf("IsAlreadyExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInvalidState(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct match", ErrInvalidState, true},
		{"wrapped", fmt.Errorf("transition: %w", ErrInvalidState), true},
		{"different error", ErrValidation, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInvalidState(tt.err); got != tt.want {
				t.Errorf("IsInvalidState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorsAreDistinct(t *testing.T) {
	// Ensure all sentinel errors are distinct
	allErrors := []error{
		ErrNotFound,
		ErrConflict,
		ErrValidation,
		ErrUnauthorized,
		ErrForbidden,
		ErrAlreadyExists,
		ErrInvalidState,
	}

	for i, e1 := range allErrors {
		for j, e2 := range allErrors {
			if i != j && errors.Is(e1, e2) {
				t.Errorf("errors should be distinct: %v and %v", e1, e2)
			}
		}
	}
}
