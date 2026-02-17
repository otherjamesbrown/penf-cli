package contentid

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantPrefix  string
		wantLen     int
	}{
		{"email", TypeEmail, "em-", 11},
		{"meeting", TypeMeeting, "mt-", 11},
		{"document", TypeDocument, "dc-", 11},
		{"transcript", TypeTranscript, "tr-", 11},
		{"attachment", TypeAttachment, "at-", 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := New(tt.contentType)

			if !strings.HasPrefix(id, tt.wantPrefix) {
				t.Errorf("New(%q) = %q, want prefix %q", tt.contentType, id, tt.wantPrefix)
			}

			if len(id) != tt.wantLen {
				t.Errorf("New(%q) length = %d, want %d", tt.contentType, len(id), tt.wantLen)
			}

			// Verify base62 characters only after prefix
			suffix := id[3:]
			if !isBase62(suffix) {
				t.Errorf("New(%q) suffix %q contains non-base62 characters", tt.contentType, suffix)
			}
		})
	}
}

func TestNewUnknownType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("New with unknown type should panic")
		}
	}()
	New("unknown")
}

func TestNewUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 10000; i++ {
		id := New(TypeEmail)
		if seen[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		wantType string
		wantErr  bool
	}{
		{"valid email", "em-1234abCD", TypeEmail, false},
		{"valid meeting", "mt-ABCD1234", TypeMeeting, false},
		{"valid document", "dc-00000000", TypeDocument, false},
		{"valid transcript", "tr-zzzzzzzz", TypeTranscript, false},
		{"valid attachment", "at-AaZz09bB", TypeAttachment, false},
		{"too short", "em-12345", "", true},
		{"too long", "em-123456789", "", true},
		{"missing dash", "em12345678", "", true},
		{"invalid prefix", "xx-12345678", "", true},
		{"invalid chars in suffix", "em-1234!@#$", "", true},
		{"empty string", "", "", true},
		{"only prefix", "em-", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cid, err := Parse(tt.id)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse(%q) expected error, got nil", tt.id)
				}
				return
			}

			if err != nil {
				t.Errorf("Parse(%q) unexpected error: %v", tt.id, err)
				return
			}

			if cid.Type != tt.wantType {
				t.Errorf("Parse(%q).Type = %q, want %q", tt.id, cid.Type, tt.wantType)
			}

			if cid.Raw != tt.id {
				t.Errorf("Parse(%q).Raw = %q, want %q", tt.id, cid.Raw, tt.id)
			}
		})
	}
}

func TestParseRoundTrip(t *testing.T) {
	types := []string{TypeEmail, TypeMeeting, TypeDocument, TypeTranscript, TypeAttachment}

	for _, contentType := range types {
		t.Run(contentType, func(t *testing.T) {
			id := New(contentType)
			cid, err := Parse(id)

			if err != nil {
				t.Errorf("Parse(New(%q)) unexpected error: %v", contentType, err)
				return
			}

			if cid.Type != contentType {
				t.Errorf("Parse(New(%q)).Type = %q, want %q", contentType, cid.Type, contentType)
			}

			if cid.Raw != id {
				t.Errorf("Parse(New(%q)).Raw = %q, want %q", contentType, cid.Raw, id)
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"valid email", "em-1234abCD", true},
		{"valid meeting", "mt-ABCD1234", true},
		{"valid document", "dc-00000000", true},
		{"valid transcript", "tr-zzzzzzzz", true},
		{"valid attachment", "at-AaZz09bB", true},
		{"too short", "em-12345", false},
		{"too long", "em-123456789", false},
		{"missing dash", "em12345678", false},
		{"invalid prefix", "xx-12345678", false},
		{"invalid chars", "em-1234!@#$", false},
		{"empty string", "", false},
		{"only prefix", "em-", false},
		{"generated id", New(TypeEmail), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValid(tt.id)
			if got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestTypeFromID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"email", "em-1234abCD", TypeEmail},
		{"meeting", "mt-ABCD1234", TypeMeeting},
		{"document", "dc-00000000", TypeDocument},
		{"transcript", "tr-zzzzzzzz", TypeTranscript},
		{"attachment", "at-AaZz09bB", TypeAttachment},
		{"invalid id", "xx-12345678", ""},
		{"too short", "em-", ""},
		{"empty string", "", ""},
		{"malformed", "em12345678", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TypeFromID(tt.id)
			if got != tt.want {
				t.Errorf("TypeFromID(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestContentIDString(t *testing.T) {
	id := "em-1234abCD"
	cid, _ := Parse(id)
	if cid.String() != id {
		t.Errorf("ContentID.String() = %q, want %q", cid.String(), id)
	}
}

func TestBase62Encoding(t *testing.T) {
	// Verify the base62 alphabet is correct
	alphabet := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if len(alphabet) != 62 {
		t.Errorf("base62 alphabet length = %d, want 62", len(alphabet))
	}

	// Generate many IDs and verify they only use base62 chars
	pattern := regexp.MustCompile(`^[0-9a-zA-Z]{8}$`)
	for i := 0; i < 1000; i++ {
		id := New(TypeEmail)
		suffix := id[3:]
		if !pattern.MatchString(suffix) {
			t.Errorf("Generated ID suffix %q doesn't match base62 pattern", suffix)
		}
	}
}

func TestTimestampWrapping(t *testing.T) {
	// The timestamp wraps every 62^4 seconds (~171 days)
	// We can't easily test the wrapping but we can verify
	// the timestamp portion changes over time

	id1 := New(TypeEmail)
	time.Sleep(10 * time.Millisecond) // Small delay
	id2 := New(TypeEmail)

	// IDs should be different (random portion guarantees this)
	if id1 == id2 {
		t.Errorf("Two sequential IDs should be different: %s == %s", id1, id2)
	}
}

func TestTypeConstants(t *testing.T) {
	// Verify type constants are 2 characters
	types := []string{TypeEmail, TypeMeeting, TypeDocument, TypeTranscript, TypeAttachment}
	for _, typ := range types {
		if len(typ) != 2 {
			t.Errorf("Type constant %q should be 2 characters, got %d", typ, len(typ))
		}
	}
}

func TestValidTypes(t *testing.T) {
	// Verify ValidTypes returns all expected types
	types := ValidTypes()
	expected := []string{TypeEmail, TypeMeeting, TypeDocument, TypeTranscript, TypeAttachment}

	if len(types) != len(expected) {
		t.Errorf("ValidTypes() returned %d types, want %d", len(types), len(expected))
	}

	typeSet := make(map[string]bool)
	for _, typ := range types {
		typeSet[typ] = true
	}

	for _, exp := range expected {
		if !typeSet[exp] {
			t.Errorf("ValidTypes() missing type %q", exp)
		}
	}
}

func TestIsValidType(t *testing.T) {
	tests := []struct {
		typ  string
		want bool
	}{
		{TypeEmail, true},
		{TypeMeeting, true},
		{TypeDocument, true},
		{TypeTranscript, true},
		{TypeAttachment, true},
		{"xx", false},
		{"", false},
		{"email", false},
		{"EM", false},
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			got := IsValidType(tt.typ)
			if got != tt.want {
				t.Errorf("IsValidType(%q) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}

// isBase62 checks if a string contains only base62 characters
func isBase62(s string) bool {
	for _, c := range s {
		//nolint:staticcheck // QF1001: current form is more readable than De Morgan's law version
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

// Benchmarks

func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		New(TypeEmail)
	}
}

func BenchmarkParse(b *testing.B) {
	id := "em-1234abCD"
	for i := 0; i < b.N; i++ {
		_, _ = Parse(id)
	}
}

func BenchmarkIsValid(b *testing.B) {
	id := "em-1234abCD"
	for i := 0; i < b.N; i++ {
		_ = IsValid(id)
	}
}

func BenchmarkTypeFromID(b *testing.B) {
	id := "em-1234abCD"
	for i := 0; i < b.N; i++ {
		_ = TypeFromID(id)
	}
}
