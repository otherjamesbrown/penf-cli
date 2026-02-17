package classification

import (
	"strings"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
)

// ClassifySourceSystem determines which system generated a piece of content.
// It uses rule-based classification with priority ordering and OR logic within each rule.
//
// Priority order:
// 1. JIRA (highest priority)
// 2. AHA
// 3. Google Docs
// 4. Webex
// 5. Smartsheet
// 6. Auto-reply
// 7. Calendar cancellation
// 8. Calendar invite
// 99. Default (human_email)
func ClassifySourceSystem(fromAddress, subject, messageID string, headers map[string]string) enrichment.SourceSystem {
	// Normalize inputs - case-insensitive matching, trim whitespace
	from := strings.ToLower(strings.TrimSpace(fromAddress))
	subj := strings.ToLower(strings.TrimSpace(subject))
	msgID := strings.ToLower(strings.TrimSpace(messageID))

	// Extract Content-Type header if present
	contentType := ""
	if headers != nil {
		if ct, ok := headers["Content-Type"]; ok {
			contentType = strings.ToLower(ct)
		}
		// Try case-insensitive lookup
		if contentType == "" {
			for k, v := range headers {
				if strings.ToLower(k) == "content-type" {
					contentType = strings.ToLower(v)
					break
				}
			}
		}
	}

	// Priority 1: JIRA
	// Match conditions (OR): from contains "jira" OR subject starts with "[TRACK-JIRA]" OR message_id contains "@Atlassian.JIRA"
	if strings.Contains(from, "jira") ||
		strings.HasPrefix(subj, "[track-jira]") ||
		strings.Contains(msgID, "@atlassian.jira") {
		return enrichment.SourceSystemJira
	}

	// Priority 2: AHA
	// Match conditions (OR): from matches *@*.mailer.aha.io OR subject starts with "[AHA]"
	if matchesPattern(from, "*@*.mailer.aha.io") ||
		strings.HasPrefix(subj, "[aha]") {
		return enrichment.SourceSystemAha
	}

	// Priority 3: Google Docs
	// Match conditions (OR): from matches *-noreply@docs.google.com OR from = drive-shares-dm-noreply@google.com
	if matchesPattern(from, "*-noreply@docs.google.com") ||
		from == "drive-shares-dm-noreply@google.com" {
		return enrichment.SourceSystemGoogleDocs
	}

	// Priority 4: Webex
	// Match condition: from = messenger@webex.com
	if from == "messenger@webex.com" {
		return enrichment.SourceSystemWebex
	}

	// Priority 5: Smartsheet
	// Match condition: from matches *@*.smartsheet.com
	if matchesPattern(from, "*@*.smartsheet.com") {
		return enrichment.SourceSystemSmartsheet
	}

	// Priority 6: Auto-reply
	// Match condition: subject starts with "Automatic reply:"
	if strings.HasPrefix(subj, "automatic reply:") {
		return enrichment.SourceSystemAutoReply
	}

	// Priority 7: Calendar cancellation
	// Match condition: subject starts with "Canceled:" or "Cancelled:"
	if strings.HasPrefix(subj, "canceled:") || strings.HasPrefix(subj, "cancelled:") {
		return enrichment.SourceSystemOutlookCalendar
	}

	// Priority 8: Calendar invite
	// Match condition: Content-Type header contains text/calendar
	if strings.Contains(contentType, "text/calendar") {
		return enrichment.SourceSystemOutlookCalendar
	}

	// Priority 99: Default - human email
	return enrichment.SourceSystemHumanEmail
}

// matchesPattern checks if an email address matches a wildcard pattern.
// Supports patterns like "*@domain.com", "prefix@*", "*@*.domain.com".
// This is a helper that works with the patterns used in source_system classification.
func matchesPattern(email, pattern string) bool {
	email = strings.ToLower(email)
	pattern = strings.ToLower(pattern)

	// No wildcard - exact match
	if !strings.Contains(pattern, "*") {
		return email == pattern
	}

	// Split pattern by wildcard
	parts := strings.Split(pattern, "*")

	// Pattern like "*@*.domain.com" has 3 parts: ["", "@", ".domain.com"]
	// We need to check that the email contains each non-empty part in order
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}

		// For the first non-empty part, it must be a prefix
		if i == 0 {
			if !strings.HasPrefix(email, part) {
				return false
			}
			pos = len(part)
			continue
		}

		// For the last non-empty part, it must be a suffix
		if i == len(parts)-1 {
			if !strings.HasSuffix(email, part) {
				return false
			}
			continue
		}

		// For middle parts, must contain the part after the current position
		idx := strings.Index(email[pos:], part)
		if idx == -1 {
			return false
		}
		pos += idx + len(part)
	}

	return true
}
