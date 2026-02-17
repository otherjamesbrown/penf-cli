package handlers

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
	"github.com/otherjamesbrown/penf-cli/pkg/enrichment/processors"
)

// ThreadData represents thread membership information for a source.
type ThreadData struct {
	MessageID       string    `json:"message_id"`
	InReplyTo       string    `json:"in_reply_to,omitempty"`
	References      []string  `json:"references,omitempty"`
	NormalizedSubject string  `json:"normalized_subject"`
	IsReply         bool      `json:"is_reply"`
	IsForward       bool      `json:"is_forward"`
	ThreadRoot      string    `json:"thread_root,omitempty"` // Detected root message ID
	MessageDate     time.Time `json:"message_date"`
}

// ThreadGrouper groups emails into threads based on headers.
// It runs in Stage 2 (Common Enrichment) for email content.
type ThreadGrouper struct {
	// fallbackToSubject enables subject-based matching when headers are missing
	fallbackToSubject bool
	// subjectTimeWindow is how far back to look for subject matches (hours)
	subjectTimeWindow int
}

// NewThreadGrouper creates a new ThreadGrouper processor.
func NewThreadGrouper(opts ...ThreadGrouperOption) *ThreadGrouper {
	t := &ThreadGrouper{
		fallbackToSubject: true,
		subjectTimeWindow: 168, // 7 days
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// ThreadGrouperOption configures the ThreadGrouper.
type ThreadGrouperOption func(*ThreadGrouper)

// WithSubjectFallback enables/disables subject-based matching.
func WithSubjectFallback(enabled bool) ThreadGrouperOption {
	return func(t *ThreadGrouper) {
		t.fallbackToSubject = enabled
	}
}

// WithSubjectTimeWindow sets the time window for subject matching.
func WithSubjectTimeWindow(hours int) ThreadGrouperOption {
	return func(t *ThreadGrouper) {
		t.subjectTimeWindow = hours
	}
}

// Name returns the processor name.
func (t *ThreadGrouper) Name() string {
	return "ThreadGrouper"
}

// Stage returns the pipeline stage.
func (t *ThreadGrouper) Stage() processors.Stage {
	return processors.StageCommonEnrichment
}

// CanProcess returns true for email content types.
func (t *ThreadGrouper) CanProcess(classification *enrichment.Classification) bool {
	return classification.ContentType == enrichment.ContentTypeEmail
}

// Process extracts thread information from the source.
func (t *ThreadGrouper) Process(ctx context.Context, pctx *processors.ProcessorContext) error {
	data := t.extractThreadData(pctx.Source)

	if pctx.Enrichment.ExtractedData == nil {
		pctx.Enrichment.ExtractedData = make(map[string]interface{})
	}
	pctx.Enrichment.ExtractedData["thread"] = data

	// Also set ThreadID on enrichment if we can determine it
	if data.ThreadRoot != "" {
		pctx.Enrichment.ThreadID = data.ThreadRoot
	} else if data.MessageID != "" {
		// This message could be a thread root
		pctx.Enrichment.ThreadID = data.MessageID
	}

	return nil
}

// extractThreadData extracts thread-related data from the source.
func (t *ThreadGrouper) extractThreadData(source *processors.Source) *ThreadData {
	data := &ThreadData{
		MessageDate: time.Now(),
	}

	// Extract Message-ID
	data.MessageID = t.getMetadataString(source, "message_id", "message-id", "Message-ID")
	if data.MessageID == "" {
		data.MessageID = source.ExternalID
	}

	// Extract In-Reply-To
	data.InReplyTo = t.getMetadataString(source, "in_reply_to", "in-reply-to", "In-Reply-To")

	// Extract References (can be string or array)
	data.References = t.getReferences(source)

	// Extract and normalize subject
	subject := t.getMetadataString(source, "subject", "Subject")
	data.NormalizedSubject = t.normalizeSubject(subject)
	data.IsReply = t.isReply(subject)
	data.IsForward = t.isForward(subject)

	// Get message date
	if date, ok := source.Metadata["date"].(time.Time); ok {
		data.MessageDate = date
	} else if date, ok := source.Metadata["received_at"].(time.Time); ok {
		data.MessageDate = date
	}

	// Determine thread root
	data.ThreadRoot = t.determineThreadRoot(data)

	return data
}

// getReferences extracts the References header as a slice.
func (t *ThreadGrouper) getReferences(source *processors.Source) []string {
	if source.Metadata == nil {
		return nil
	}

	// Try as string array
	if refs, ok := source.Metadata["references"].([]string); ok {
		return refs
	}

	// Try as interface array
	if refs, ok := source.Metadata["references"].([]interface{}); ok {
		var result []string
		for _, r := range refs {
			if s, ok := r.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	// Try as single string (space or comma separated, or JSON-encoded array)
	if refs, ok := source.Metadata["references"].(string); ok && refs != "" {
		// If it looks like JSON, try parsing it
		if len(refs) > 0 && refs[0] == '[' {
			var jsonArray []string
			if err := json.Unmarshal([]byte(refs), &jsonArray); err == nil {
				return jsonArray
			}
		}
		return t.parseReferences(refs)
	}

	// Check headers
	if headers, ok := source.Metadata["headers"].(map[string]interface{}); ok {
		if refs, ok := headers["References"].(string); ok {
			return t.parseReferences(refs)
		}
		if refs, ok := headers["references"].(string); ok {
			return t.parseReferences(refs)
		}
	}

	return nil
}

// messageIDPattern matches Message-ID format
var messageIDPattern = regexp.MustCompile(`<[^>]+>`)

// parseReferences parses a References header string into individual message IDs.
func (t *ThreadGrouper) parseReferences(refs string) []string {
	// References are typically space-separated Message-IDs in angle brackets
	matches := messageIDPattern.FindAllString(refs, -1)
	if len(matches) > 0 {
		return matches
	}

	// Fallback: split by whitespace and filter
	var result []string
	for _, part := range strings.Fields(refs) {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// subjectPrefixPattern matches common email subject prefixes
var subjectPrefixPattern = regexp.MustCompile(`(?i)^(Re:\s*|Fwd?:\s*|FW:\s*)+`)

// normalizeSubject removes reply/forward prefixes and normalizes a subject.
func (t *ThreadGrouper) normalizeSubject(subject string) string {
	// Trim leading/trailing whitespace first
	normalized := strings.TrimSpace(subject)
	// Remove Re:/Fwd:/FW: prefixes (may need multiple passes)
	for {
		stripped := subjectPrefixPattern.ReplaceAllString(normalized, "")
		stripped = strings.TrimSpace(stripped)
		if stripped == normalized {
			break
		}
		normalized = stripped
	}
	// Normalize whitespace
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}

// isReply checks if subject indicates a reply.
func (t *ThreadGrouper) isReply(subject string) bool {
	lowerSubject := strings.ToLower(strings.TrimSpace(subject))
	return strings.HasPrefix(lowerSubject, "re:")
}

// isForward checks if subject indicates a forward.
func (t *ThreadGrouper) isForward(subject string) bool {
	lowerSubject := strings.ToLower(strings.TrimSpace(subject))
	return strings.HasPrefix(lowerSubject, "fwd:") ||
		strings.HasPrefix(lowerSubject, "fw:")
}

// determineThreadRoot determines the root message ID for threading.
func (t *ThreadGrouper) determineThreadRoot(data *ThreadData) string {
	// Priority 1: First message in References chain
	if len(data.References) > 0 {
		return data.References[0]
	}

	// Priority 2: In-Reply-To header
	if data.InReplyTo != "" {
		return data.InReplyTo
	}

	// Priority 3: This message is potentially a root (no thread indicators)
	if !data.IsReply && !data.IsForward {
		return data.MessageID
	}

	// If it looks like a reply but has no headers, leave empty
	// (the pipeline may use subject matching later)
	return ""
}

// getMetadataString extracts a string from source metadata.
func (t *ThreadGrouper) getMetadataString(source *processors.Source, keys ...string) string {
	if source.Metadata == nil {
		return ""
	}

	for _, key := range keys {
		if val, ok := source.Metadata[key].(string); ok && val != "" {
			return val
		}
	}

	// Also check nested headers
	if headers, ok := source.Metadata["headers"].(map[string]interface{}); ok {
		for _, key := range keys {
			if val, ok := headers[key].(string); ok && val != "" {
				return val
			}
		}
	}

	return ""
}

// Verify interface compliance
var _ processors.CommonEnrichmentProcessor = (*ThreadGrouper)(nil)
var _ processors.Processor = (*ThreadGrouper)(nil)
