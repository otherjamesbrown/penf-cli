// Package contentid provides unique content identifier generation and validation.
//
// ID Format: <type:2>-<base62_ts:4><base62_rand:4> (11 chars total including dash)
//
// Content Types:
//   - em = email
//   - mt = meeting
//   - dc = document
//   - tr = transcript
//   - at = attachment
//
// The timestamp component uses seconds since epoch modulo 62^4 (~171 days cycle).
// The random component provides 14M+ combinations to ensure uniqueness.
package contentid

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"
)

// Content type constants
const (
	TypeEmail      = "em"
	TypeMeeting    = "mt"
	TypeDocument   = "dc"
	TypeTranscript = "tr"
	TypeAttachment = "at"
)

// base62 alphabet: 0-9, a-z, A-Z
const base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// base62Max is 62^4 = 14,776,336 (used for timestamp wrapping)
const base62Max = 62 * 62 * 62 * 62

// validTypes maps type prefixes to their names for validation
var validTypes = map[string]bool{
	TypeEmail:      true,
	TypeMeeting:    true,
	TypeDocument:   true,
	TypeTranscript: true,
	TypeAttachment: true,
}

// Errors
var (
	ErrInvalidFormat = errors.New("invalid content ID format")
	ErrInvalidType   = errors.New("invalid content type")
)

// ContentID represents a parsed content identifier.
type ContentID struct {
	Type      string // Content type prefix (em, mt, dc, tr, at)
	Timestamp string // Base62 encoded timestamp (4 chars)
	Random    string // Base62 encoded random component (4 chars)
	Raw       string // Original ID string
}

// String returns the string representation of the ContentID.
func (c ContentID) String() string {
	return c.Raw
}

// New generates a new content ID for the given content type.
// Panics if contentType is not a valid type constant.
func New(contentType string) string {
	if !validTypes[contentType] {
		panic(fmt.Sprintf("contentid: invalid content type: %q", contentType))
	}

	// Use UnixNano for higher resolution timestamp to reduce collision chance
	// within the same second. The modulo still gives ~171 day cycle.
	ts := encodeBase62(uint64(time.Now().UnixNano()/1000) % base62Max)
	rnd := randomBase62(4)

	return fmt.Sprintf("%s-%s%s", contentType, ts, rnd)
}

// Parse validates and parses a content ID string.
// Returns an error if the ID format is invalid or the type is unknown.
func Parse(id string) (ContentID, error) {
	if len(id) != 11 {
		return ContentID{}, fmt.Errorf("%w: expected 11 characters, got %d", ErrInvalidFormat, len(id))
	}

	if id[2] != '-' {
		return ContentID{}, fmt.Errorf("%w: missing dash at position 2", ErrInvalidFormat)
	}

	prefix := id[:2]
	if !validTypes[prefix] {
		return ContentID{}, fmt.Errorf("%w: unknown type %q", ErrInvalidType, prefix)
	}

	suffix := id[3:]
	if !isValidBase62(suffix) {
		return ContentID{}, fmt.Errorf("%w: suffix contains invalid characters", ErrInvalidFormat)
	}

	return ContentID{
		Type:      prefix,
		Timestamp: suffix[:4],
		Random:    suffix[4:],
		Raw:       id,
	}, nil
}

// IsValid checks if a string is a valid content ID.
// This is a convenience function for quick validation without parsing.
func IsValid(id string) bool {
	_, err := Parse(id)
	return err == nil
}

// TypeFromID extracts the content type from an ID string.
// Returns empty string if the ID is invalid.
func TypeFromID(id string) string {
	if len(id) < 3 || id[2] != '-' {
		return ""
	}
	prefix := id[:2]
	if !validTypes[prefix] {
		return ""
	}
	// Also validate the rest of the ID
	if len(id) != 11 || !isValidBase62(id[3:]) {
		return ""
	}
	return prefix
}

// ValidTypes returns a slice of all valid content type prefixes.
func ValidTypes() []string {
	return []string{TypeEmail, TypeMeeting, TypeDocument, TypeTranscript, TypeAttachment}
}

// IsValidType checks if the given string is a valid content type prefix.
func IsValidType(typ string) bool {
	return validTypes[typ]
}

// encodeBase62 encodes a number as a 4-character base62 string.
func encodeBase62(n uint64) string {
	result := make([]byte, 4)
	for i := 3; i >= 0; i-- {
		result[i] = base62Alphabet[n%62]
		n /= 62
	}
	return string(result)
}

// randomBase62 generates a random base62 string of the specified length.
// Uses rejection sampling to eliminate modulo bias.
func randomBase62(length int) string {
	result := make([]byte, length)

	// 256 / 62 = 4 with remainder 8, so values 0-247 map evenly to 0-61
	// Reject values 248-255 to eliminate bias
	const maxUnbiased = 248

	for i := 0; i < length; {
		var b [1]byte
		if _, err := rand.Read(b[:]); err != nil {
			// Fallback - should never happen with crypto/rand
			result[i] = base62Alphabet[0]
			i++
			continue
		}

		if b[0] < maxUnbiased {
			result[i] = base62Alphabet[b[0]%62]
			i++
		}
		// Reject and retry if b[0] >= 248
	}

	return string(result)
}

// isValidBase62 checks if a string contains only base62 characters.
func isValidBase62(s string) bool {
	for _, c := range s {
		//nolint:staticcheck // QF1001: current form is more readable than De Morgan's law version
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}
