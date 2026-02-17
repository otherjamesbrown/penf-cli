package enrichment

import (
	"strings"
)

// SubtypePattern represents a tenant-specific pattern for content subtype classification.
type SubtypePattern struct {
	Pattern     string
	PatternType string
	Priority    int
}

// ClassifyContentSubtype determines the content subtype based on headers, from address,
// and subject line. It uses both hardcoded default patterns and optional tenant-specific patterns.
//
// Priority order:
// 1. Calendar (Content-Type: text/calendar OR subject prefix: Canceled:, Accepted:, etc.)
// 2. Notification (from-address domain patterns)
// 3. Email threading (In-Reply-To header → thread, Fwd: subject → forward)
// 4. Standalone (default)
func ClassifyContentSubtype(headers map[string]string, fromAddress string, subject string, tenantPatterns []SubtypePattern) ContentSubtype {
	// Normalize inputs
	contentType := ""
	inReplyTo := ""
	if headers != nil {
		if ct, ok := headers["Content-Type"]; ok {
			contentType = strings.ToLower(ct)
		}
		if irt, ok := headers["In-Reply-To"]; ok {
			inReplyTo = strings.TrimSpace(irt)
		}
	}
	fromLower := strings.ToLower(strings.TrimSpace(fromAddress))
	subjectTrimmed := strings.TrimSpace(subject)

	// Priority 1: Calendar detection by Content-Type
	if strings.Contains(contentType, "text/calendar") {
		return SubtypeCalendarInvite
	}

	// Priority 1b: Calendar detection by subject prefix
	if calendarSubtype := classifyCalendarBySubject(subjectTrimmed); calendarSubtype != "" {
		return calendarSubtype
	}

	// Priority 2: Notification detection by from-address patterns
	if notificationSubtype := classifyNotificationByFromAddress(fromLower, tenantPatterns); notificationSubtype != "" {
		return notificationSubtype
	}

	// Priority 3: Email threading detection
	return classifyEmailThreading(inReplyTo, subjectTrimmed)
}

// classifyCalendarBySubject checks subject prefixes for calendar event types.
func classifyCalendarBySubject(subject string) ContentSubtype {
	subjectLower := strings.ToLower(subject)

	// Check for cancellation
	if strings.HasPrefix(subjectLower, "canceled:") {
		return SubtypeCalendarCancellation
	}

	// Check for responses (Accepted/Declined/Tentative)
	if strings.HasPrefix(subjectLower, "accepted:") ||
		strings.HasPrefix(subjectLower, "declined:") ||
		strings.HasPrefix(subjectLower, "tentative:") {
		return SubtypeCalendarResponse
	}

	// Check for updates
	if strings.HasPrefix(subjectLower, "updated:") ||
		strings.HasPrefix(subjectLower, "updated invitation:") {
		return SubtypeCalendarUpdate
	}

	return ""
}

// classifyNotificationByFromAddress checks from-address patterns for notification emails.
func classifyNotificationByFromAddress(fromAddress string, tenantPatterns []SubtypePattern) ContentSubtype {
	// Extract domain from email address
	domain := ""
	if atIndex := strings.LastIndex(fromAddress, "@"); atIndex != -1 && atIndex < len(fromAddress)-1 {
		domain = fromAddress[atIndex+1:]
	}
	localPart := ""
	if atIndex := strings.LastIndex(fromAddress, "@"); atIndex != -1 {
		localPart = fromAddress[:atIndex]
	}

	// Check tenant patterns first (higher priority)
	if tenantPatterns != nil {
		// Sort by priority (higher priority first)
		// For simplicity, check in order - caller should pre-sort if needed
		for _, pattern := range tenantPatterns {
			if matchesPattern(fromAddress, pattern.Pattern) {
				// Map pattern type to ContentSubtype
				switch pattern.PatternType {
				case "notification/jira":
					return SubtypeNotificationJira
				case "notification/google":
					return SubtypeNotificationGoogle
				case "notification/slack":
					return SubtypeNotificationSlack
				case "notification/other":
					return SubtypeNotificationOther
				}
			}
		}
	}

	// Hardcoded default patterns
	// Check for specific notification services

	// Jira patterns
	if strings.Contains(domain, "atlassian.net") ||
		strings.HasPrefix(fromAddress, "gsd-jira@") {
		return SubtypeNotificationJira
	}

	// Slack patterns
	if strings.Contains(domain, "slack.com") {
		return SubtypeNotificationSlack
	}

	// Google patterns (docs, drive, calendar, etc.)
	if strings.Contains(domain, "docs.google.com") ||
		strings.Contains(domain, "google.com") && (strings.Contains(localPart, "noreply") || strings.Contains(localPart, "notification")) {
		return SubtypeNotificationGoogle
	}

	// Aha.io patterns
	if strings.Contains(domain, "mailer.aha.io") {
		return SubtypeNotificationOther
	}

	// Generic no-reply patterns
	if strings.Contains(localPart, "noreply") || strings.Contains(localPart, "no-reply") {
		return SubtypeNotificationOther
	}

	// GitHub patterns
	if strings.Contains(domain, "github.com") {
		return SubtypeNotificationOther
	}

	return ""
}

// classifyEmailThreading determines thread/forward/standalone based on headers and subject.
func classifyEmailThreading(inReplyTo string, subject string) ContentSubtype {
	// Check for thread (In-Reply-To header present)
	if inReplyTo != "" {
		return SubtypeEmailThread
	}

	// Check for forward by subject prefix
	subjectLower := strings.ToLower(subject)
	if strings.HasPrefix(subjectLower, "fwd:") || strings.HasPrefix(subjectLower, "fw:") {
		return SubtypeEmailForward
	}

	// Default to standalone
	return SubtypeEmailStandalone
}

// matchesPattern checks if an email address matches a pattern.
// Supports wildcards (* matches any sequence).
func matchesPattern(email, pattern string) bool {
	email = strings.ToLower(email)
	pattern = strings.ToLower(pattern)

	// Simple wildcard matching
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			// Pattern like "prefix@*" or "*@suffix"
			if parts[0] != "" && !strings.HasPrefix(email, parts[0]) {
				return false
			}
			if parts[1] != "" && !strings.HasSuffix(email, parts[1]) {
				return false
			}
			return true
		}
	}

	// Exact match
	return email == pattern
}
