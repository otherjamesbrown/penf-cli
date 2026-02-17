// Package classification provides content classification for the enrichment pipeline.
package classification

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
	"github.com/otherjamesbrown/penf-cli/pkg/enrichment/processors"
)

// ContentClassifier implements Stage 1: Content Classification.
// It determines content_type, content_subtype, and processing_profile
// based on headers, content patterns, and heuristics.
type ContentClassifier struct {
	// Configuration for classification rules
	internalDomains []string
}

// NewContentClassifier creates a new content classifier.
func NewContentClassifier(opts ...ClassifierOption) *ContentClassifier {
	c := &ContentClassifier{
		internalDomains: []string{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ClassifierOption configures the classifier.
type ClassifierOption func(*ContentClassifier)

// WithInternalDomains sets the list of internal email domains.
func WithInternalDomains(domains []string) ClassifierOption {
	return func(c *ContentClassifier) {
		c.internalDomains = domains
	}
}

// Name returns the processor name.
func (c *ContentClassifier) Name() string {
	return "ContentClassifier"
}

// Stage returns the pipeline stage.
func (c *ContentClassifier) Stage() processors.Stage {
	return processors.StageClassification
}

// Process implements the Processor interface.
func (c *ContentClassifier) Process(ctx context.Context, pctx *processors.ProcessorContext) error {
	classification, err := c.Classify(ctx, pctx.Source)
	if err != nil {
		return err
	}
	pctx.Enrichment.Classification = *classification
	return nil
}

// Classify determines the content type, subtype, and processing profile.
func (c *ContentClassifier) Classify(ctx context.Context, source *processors.Source) (*enrichment.Classification, error) {
	// Extract metadata for classification
	metadata := c.extractMetadata(source)

	// Apply classification rules in priority order
	for _, rule := range classificationRules {
		if rule.matches(metadata) {
			return &enrichment.Classification{
				ContentType: rule.contentType,
				Subtype:     rule.subtype,
				Profile:     rule.profile,
				Confidence:  1.0,
				Reason:      rule.reason,
				DetectedVia: rule.name,
				RulesPriority: rule.priority,
			}, nil
		}
	}

	// Default fallback: standalone email with full AI processing
	return &enrichment.Classification{
		ContentType: enrichment.ContentTypeEmail,
		Subtype:     enrichment.SubtypeEmailStandalone,
		Profile:     enrichment.ProfileFullAI,
		Confidence:  1.0,
		Reason:      "Default classification for unmatched content",
		DetectedVia: "default",
		RulesPriority: 999,
	}, nil
}

// classificationMetadata holds extracted data used for classification.
type classificationMetadata struct {
	// Headers
	from            string
	subject         string
	contentType     string
	autoSubmitted   string
	precedence      string
	xAutoResponse   string
	inReplyTo       string
	references      string
	xMSExchangeAuth string

	// Derived
	hasCalendarAttachment bool
	hasInReplyTo          bool
	hasReferences         bool
}

// extractMetadata extracts classification-relevant data from the source.
func (c *ContentClassifier) extractMetadata(source *processors.Source) *classificationMetadata {
	meta := &classificationMetadata{}

	// Try to extract from source metadata
	if source.Metadata != nil {
		// Headers are often stored in metadata
		if headers, ok := source.Metadata["headers"].(map[string]interface{}); ok {
			meta.from = getStringFromMap(headers, "from", "From")
			meta.subject = getStringFromMap(headers, "subject", "Subject")
			meta.contentType = getStringFromMap(headers, "content-type", "Content-Type")
			meta.autoSubmitted = getStringFromMap(headers, "auto-submitted", "Auto-Submitted")
			meta.precedence = getStringFromMap(headers, "precedence", "Precedence")
			meta.xAutoResponse = getStringFromMap(headers, "x-auto-response-suppress", "X-Auto-Response-Suppress")
			meta.inReplyTo = getStringFromMap(headers, "in-reply-to", "In-Reply-To")
			meta.references = getStringFromMap(headers, "references", "References")
			meta.xMSExchangeAuth = getStringFromMap(headers, "x-ms-exchange-organization-authas", "X-MS-Exchange-Organization-AuthAs")
		}

		// Direct metadata fields
		if from, ok := source.Metadata["from"].(string); ok && meta.from == "" {
			meta.from = from
		}
		if subject, ok := source.Metadata["subject"].(string); ok && meta.subject == "" {
			meta.subject = subject
		}
		if inReplyTo, ok := source.Metadata["in_reply_to"].(string); ok && meta.inReplyTo == "" {
			meta.inReplyTo = inReplyTo
		}
		if refs, ok := source.Metadata["references"].([]interface{}); ok && len(refs) > 0 {
			meta.hasReferences = true
		}
		if refs, ok := source.Metadata["references"].(string); ok && refs != "" {
			meta.references = refs
			meta.hasReferences = true
		}

		// Check for attachments
		if attachments, ok := source.Metadata["attachments"].([]interface{}); ok {
			for _, att := range attachments {
				if attMap, ok := att.(map[string]interface{}); ok {
					if mimeType, ok := attMap["mime_type"].(string); ok {
						if strings.Contains(mimeType, "calendar") || strings.Contains(mimeType, "ics") {
							meta.hasCalendarAttachment = true
							break
						}
					}
					if filename, ok := attMap["filename"].(string); ok {
						if strings.HasSuffix(strings.ToLower(filename), ".ics") {
							meta.hasCalendarAttachment = true
							break
						}
					}
				}
			}
		}
	}

	// Also check content type
	if strings.Contains(source.ContentType, "calendar") {
		meta.hasCalendarAttachment = true
	}

	// Derive boolean fields
	meta.hasInReplyTo = meta.inReplyTo != ""
	if meta.references != "" {
		meta.hasReferences = true
	}

	return meta
}

// classificationRule defines a classification rule.
type classificationRule struct {
	name        string
	priority    int
	contentType enrichment.ContentType
	subtype     enrichment.ContentSubtype
	profile     enrichment.ProcessingProfile
	reason      string
	matches     func(*classificationMetadata) bool
}

// classificationRules defines rules in priority order (lower number = higher priority).
var classificationRules = []classificationRule{
	// Priority 1: Calendar content
	{
		name:        "calendar_cancellation",
		priority:    1,
		contentType: enrichment.ContentTypeCalendar,
		subtype:     enrichment.SubtypeCalendarCancellation,
		profile:     enrichment.ProfileStateTracking,
		reason:      "Subject indicates meeting cancellation",
		matches: func(m *classificationMetadata) bool {
			return m.hasCalendarAttachment && subjectStartsWithAny(m.subject, "Canceled:", "Cancelled:")
		},
	},
	{
		name:        "calendar_response",
		priority:    1,
		contentType: enrichment.ContentTypeCalendar,
		subtype:     enrichment.SubtypeCalendarResponse,
		profile:     enrichment.ProfileStateTracking,
		reason:      "Subject indicates meeting response",
		matches: func(m *classificationMetadata) bool {
			return subjectStartsWithAny(m.subject, "Accepted:", "Declined:", "Tentative:")
		},
	},
	{
		name:        "calendar_update",
		priority:    1,
		contentType: enrichment.ContentTypeCalendar,
		subtype:     enrichment.SubtypeCalendarUpdate,
		profile:     enrichment.ProfileStateTracking,
		reason:      "Subject indicates meeting update with calendar attachment",
		matches: func(m *classificationMetadata) bool {
			return m.hasCalendarAttachment && subjectStartsWithAny(m.subject, "Updated:")
		},
	},
	{
		name:        "calendar_invite",
		priority:    1,
		contentType: enrichment.ContentTypeCalendar,
		subtype:     enrichment.SubtypeCalendarInvite,
		profile:     enrichment.ProfileStateTracking,
		reason:      "Calendar attachment detected",
		matches: func(m *classificationMetadata) bool {
			return m.hasCalendarAttachment || strings.Contains(m.contentType, "text/calendar")
		},
	},

	// Priority 2: Jira notifications
	{
		name:        "notification_jira",
		priority:    2,
		contentType: enrichment.ContentTypeEmail,
		subtype:     enrichment.SubtypeNotificationJira,
		profile:     enrichment.ProfileMetadataOnly,
		reason:      "Jira notification detected via sender and auto-submitted header",
		matches: func(m *classificationMetadata) bool {
			fromLower := strings.ToLower(m.from)
			hasJiraSender := strings.Contains(fromLower, "jira")
			hasAutoHeader := m.autoSubmitted != "" || strings.ToLower(m.precedence) == "bulk"
			return hasJiraSender && hasAutoHeader
		},
	},

	// Priority 3: Google notifications
	{
		name:        "notification_google",
		priority:    3,
		contentType: enrichment.ContentTypeEmail,
		subtype:     enrichment.SubtypeNotificationGoogle,
		profile:     enrichment.ProfileMetadataOnly,
		reason:      "Google notification detected via sender",
		matches: func(m *classificationMetadata) bool {
			fromLower := strings.ToLower(m.from)
			return strings.Contains(fromLower, "-noreply@docs.google.com") ||
				strings.Contains(fromLower, "@google.com") && strings.Contains(fromLower, "noreply")
		},
	},

	// Priority 4: Slack notifications
	{
		name:        "notification_slack",
		priority:    4,
		contentType: enrichment.ContentTypeEmail,
		subtype:     enrichment.SubtypeNotificationSlack,
		profile:     enrichment.ProfileMetadataOnly,
		reason:      "Slack notification detected via sender",
		matches: func(m *classificationMetadata) bool {
			fromLower := strings.ToLower(m.from)
			return strings.Contains(fromLower, "slack") || strings.Contains(fromLower, "@slack.com")
		},
	},

	// Priority 5: Other automated notifications
	{
		name:        "notification_other",
		priority:    5,
		contentType: enrichment.ContentTypeEmail,
		subtype:     enrichment.SubtypeNotificationOther,
		profile:     enrichment.ProfileMetadataOnly,
		reason:      "Auto-generated content detected via headers",
		matches: func(m *classificationMetadata) bool {
			return m.autoSubmitted != "" || strings.ToLower(m.precedence) == "bulk" ||
				m.xAutoResponse != ""
		},
	},

	// Priority 6: Forwards
	{
		name:        "email_forward",
		priority:    6,
		contentType: enrichment.ContentTypeEmail,
		subtype:     enrichment.SubtypeEmailForward,
		profile:     enrichment.ProfileFullAI,
		reason:      "Subject indicates forwarded message",
		matches: func(m *classificationMetadata) bool {
			return subjectStartsWithAny(m.subject, "FW:", "Fwd:", "FWD:")
		},
	},

	// Priority 7: Thread replies
	{
		name:        "email_thread",
		priority:    7,
		contentType: enrichment.ContentTypeEmail,
		subtype:     enrichment.SubtypeEmailThread,
		profile:     enrichment.ProfileFullAI,
		reason:      "Thread headers present (In-Reply-To or References)",
		matches: func(m *classificationMetadata) bool {
			return m.hasInReplyTo || m.hasReferences
		},
	},
}

// Helper functions

func subjectStartsWithAny(subject string, prefixes ...string) bool {
	subjectLower := strings.ToLower(strings.TrimSpace(subject))
	for _, prefix := range prefixes {
		if strings.HasPrefix(subjectLower, strings.ToLower(prefix)) {
			return true
		}
	}
	return false
}

func getStringFromMap(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			switch v := val.(type) {
			case string:
				return v
			case []string:
				if len(v) > 0 {
					return v[0]
				}
			case []interface{}:
				if len(v) > 0 {
					if s, ok := v[0].(string); ok {
						return s
					}
				}
			}
		}
	}
	return ""
}

// JiraTicketPattern matches Jira ticket IDs in format [PROJECT-123]
var JiraTicketPattern = regexp.MustCompile(`\[([A-Z]{2,10}-\d+)\]`)

// ExtractJiraTicket extracts a Jira ticket ID from the subject.
func ExtractJiraTicket(subject string) string {
	matches := JiraTicketPattern.FindStringSubmatch(subject)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// Verify interface compliance
var _ processors.ClassificationProcessor = (*ContentClassifier)(nil)
var _ processors.Processor = (*ContentClassifier)(nil)

// Ensure json is used (for metadata parsing)
var _ = json.Marshal
