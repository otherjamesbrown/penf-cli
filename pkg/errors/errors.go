// Package errors provides common domain error types for the penfold application.
//
// This package defines sentinel errors for common domain conditions like "not found"
// or "conflict" that can be used across all packages. Using typed errors enables
// consistent error handling patterns with errors.Is() checks.
//
// Usage:
//
//	import pferrors "github.com/penfold/pkg/errors"
//
//	// Return a domain error
//	return nil, pferrors.ErrNotFound
//
//	// Check for domain errors
//	if pferrors.IsNotFound(err) {
//	    // handle not found case
//	}
package errors

import "errors"

// Domain errors - common sentinel errors for domain conditions.
var (
	// ErrNotFound indicates the requested resource was not found.
	ErrNotFound = errors.New("not found")

	// ErrConflict indicates a conflict with existing data (e.g., duplicate key).
	ErrConflict = errors.New("conflict")

	// ErrValidation indicates invalid input or validation failure.
	ErrValidation = errors.New("validation error")

	// ErrUnauthorized indicates the request lacks valid authentication.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden indicates the authenticated user lacks permission.
	ErrForbidden = errors.New("forbidden")

	// ErrAlreadyExists indicates the resource already exists.
	ErrAlreadyExists = errors.New("already exists")

	// ErrInvalidState indicates the operation is not valid for the current state.
	ErrInvalidState = errors.New("invalid state")
)

// IsNotFound reports whether any error in err's chain is ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsConflict reports whether any error in err's chain is ErrConflict.
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}

// IsValidation reports whether any error in err's chain is ErrValidation.
func IsValidation(err error) bool {
	return errors.Is(err, ErrValidation)
}

// IsUnauthorized reports whether any error in err's chain is ErrUnauthorized.
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsForbidden reports whether any error in err's chain is ErrForbidden.
func IsForbidden(err error) bool {
	return errors.Is(err, ErrForbidden)
}

// IsAlreadyExists reports whether any error in err's chain is ErrAlreadyExists.
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsInvalidState reports whether any error in err's chain is ErrInvalidState.
func IsInvalidState(err error) bool {
	return errors.Is(err, ErrInvalidState)
}
