package enrichment

import (
	"testing"
)

// TestClassifyContentSubtype_Calendar tests detection of calendar invite/response/update/cancellation
// subtypes based on Content-Type header and subject prefixes.
func TestClassifyContentSubtype_Calendar(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		subject     string
		fromAddress string
		want        ContentSubtype
	}{
		{
			name:        "calendar invite by Content-Type",
			contentType: "text/calendar",
			subject:     "Team Standup",
			fromAddress: "organizer@company.com",
			want:        SubtypeCalendarInvite,
		},
		{
			name:        "calendar invite by Content-Type with method=REQUEST",
			contentType: "text/calendar; method=REQUEST",
			subject:     "Weekly Review",
			fromAddress: "manager@company.com",
			want:        SubtypeCalendarInvite,
		},
		{
			name:        "calendar cancellation by subject prefix",
			contentType: "text/plain",
			subject:     "Canceled: Team Standup",
			fromAddress: "organizer@company.com",
			want:        SubtypeCalendarCancellation,
		},
		{
			name:        "calendar cancellation by subject prefix case insensitive",
			contentType: "text/plain",
			subject:     "CANCELED: All Hands Meeting",
			fromAddress: "hr@company.com",
			want:        SubtypeCalendarCancellation,
		},
		{
			name:        "calendar response accepted",
			contentType: "text/plain",
			subject:     "Accepted: Project Planning",
			fromAddress: "attendee@company.com",
			want:        SubtypeCalendarResponse,
		},
		{
			name:        "calendar response declined",
			contentType: "text/plain",
			subject:     "Declined: Engineering Sync",
			fromAddress: "attendee@company.com",
			want:        SubtypeCalendarResponse,
		},
		{
			name:        "calendar response tentative",
			contentType: "text/plain",
			subject:     "Tentative: 1:1 with Manager",
			fromAddress: "attendee@company.com",
			want:        SubtypeCalendarResponse,
		},
		{
			name:        "calendar update by subject prefix",
			contentType: "text/plain",
			subject:     "Updated: Sprint Planning",
			fromAddress: "organizer@company.com",
			want:        SubtypeCalendarUpdate,
		},
		{
			name:        "calendar update with location change",
			contentType: "text/plain",
			subject:     "Updated invitation: Design Review",
			fromAddress: "organizer@company.com",
			want:        SubtypeCalendarUpdate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				"Content-Type": tt.contentType,
			}
			got := ClassifyContentSubtype(headers, tt.fromAddress, tt.subject, nil)
			if got != tt.want {
				t.Errorf("ClassifyContentSubtype() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestClassifyContentSubtype_Notifications tests detection of notification emails
// by from-address domain patterns.
func TestClassifyContentSubtype_Notifications(t *testing.T) {
	tests := []struct {
		name        string
		fromAddress string
		subject     string
		want        ContentSubtype
	}{
		{
			name:        "Aha notification",
			fromAddress: "updates@mailer.aha.io",
			subject:     "Feature request updated",
			want:        SubtypeNotificationOther,
		},
		{
			name:        "Google Docs notification",
			fromAddress: "comments-noreply@docs.google.com",
			subject:     "John Doe commented on Design Doc",
			want:        SubtypeNotificationGoogle,
		},
		{
			name:        "Google Drive notification",
			fromAddress: "drive-shares-noreply@google.com",
			subject:     "Jane shared Q4 Planning with you",
			want:        SubtypeNotificationGoogle,
		},
		{
			name:        "Google Calendar notification",
			fromAddress: "calendar-notification@google.com",
			subject:     "Reminder: Meeting in 15 minutes",
			want:        SubtypeNotificationGoogle,
		},
		{
			name:        "Jira notification",
			fromAddress: "gsd-jira@akamai.com",
			subject:     "PROJ-123 was updated",
			want:        SubtypeNotificationJira,
		},
		{
			name:        "Jira Cloud notification",
			fromAddress: "notifications@atlassian.net",
			subject:     "TEAM-456 New issue assigned to you",
			want:        SubtypeNotificationJira,
		},
		{
			name:        "Slack notification",
			fromAddress: "notifications@slack.com",
			subject:     "New message in #engineering",
			want:        SubtypeNotificationSlack,
		},
		{
			name:        "generic noreply notification",
			fromAddress: "noreply@service.com",
			subject:     "Your request has been processed",
			want:        SubtypeNotificationOther,
		},
		{
			name:        "generic no-reply notification",
			fromAddress: "no-reply@platform.io",
			subject:     "Build completed",
			want:        SubtypeNotificationOther,
		},
		{
			name:        "GitHub notification",
			fromAddress: "notifications@github.com",
			subject:     "PR #42 merged",
			want:        SubtypeNotificationOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				"Content-Type": "text/plain",
			}
			got := ClassifyContentSubtype(headers, tt.fromAddress, tt.subject, nil)
			if got != tt.want {
				t.Errorf("ClassifyContentSubtype() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestClassifyContentSubtype_EmailThreading tests detection of thread/forward/standalone
// email subtypes based on In-Reply-To header and subject prefixes.
func TestClassifyContentSubtype_EmailThreading(t *testing.T) {
	tests := []struct {
		name        string
		inReplyTo   string
		subject     string
		fromAddress string
		want        ContentSubtype
	}{
		{
			name:        "email thread with In-Reply-To header",
			inReplyTo:   "<original-message-id@domain.com>",
			subject:     "Re: Project proposal",
			fromAddress: "colleague@company.com",
			want:        SubtypeEmailThread,
		},
		{
			name:        "email thread with References header implies In-Reply-To",
			inReplyTo:   "<msg1@domain.com> <msg2@domain.com>",
			subject:     "Re: Design review feedback",
			fromAddress: "designer@company.com",
			want:        SubtypeEmailThread,
		},
		{
			name:        "forwarded email by subject prefix Fwd:",
			inReplyTo:   "",
			subject:     "Fwd: Important announcement",
			fromAddress: "manager@company.com",
			want:        SubtypeEmailForward,
		},
		{
			name:        "forwarded email by subject prefix FW:",
			inReplyTo:   "",
			subject:     "FW: Customer feedback",
			fromAddress: "sales@company.com",
			want:        SubtypeEmailForward,
		},
		{
			name:        "forwarded email case insensitive",
			inReplyTo:   "",
			subject:     "fwd: Meeting notes",
			fromAddress: "team@company.com",
			want:        SubtypeEmailForward,
		},
		{
			name:        "standalone email no threading",
			inReplyTo:   "",
			subject:     "New project kickoff",
			fromAddress: "pm@company.com",
			want:        SubtypeEmailStandalone,
		},
		{
			name:        "standalone email with empty subject",
			inReplyTo:   "",
			subject:     "",
			fromAddress: "user@company.com",
			want:        SubtypeEmailStandalone,
		},
		{
			name:        "thread takes precedence over forward prefix",
			inReplyTo:   "<msg@domain.com>",
			subject:     "Fwd: Re: Discussion",
			fromAddress: "user@company.com",
			want:        SubtypeEmailThread,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				"Content-Type": "text/plain",
			}
			if tt.inReplyTo != "" {
				headers["In-Reply-To"] = tt.inReplyTo
			}
			got := ClassifyContentSubtype(headers, tt.fromAddress, tt.subject, nil)
			if got != tt.want {
				t.Errorf("ClassifyContentSubtype() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestClassifyContentSubtype_TenantPatterns tests that custom tenant patterns
// override or augment default classification behavior.
func TestClassifyContentSubtype_TenantPatterns(t *testing.T) {
	tests := []struct {
		name           string
		fromAddress    string
		subject        string
		tenantPatterns []SubtypePattern
		want           ContentSubtype
	}{
		{
			name:        "custom tenant pattern identifies custom notification",
			fromAddress: "jira-acme@company.com",
			subject:     "ACME-123 updated",
			tenantPatterns: []SubtypePattern{
				{
					Pattern:     "jira-acme@*",
					PatternType: "notification/jira",
					Priority:    10,
				},
			},
			want: SubtypeNotificationJira,
		},
		{
			name:        "custom tenant pattern for internal tool",
			fromAddress: "buildbot@company.com",
			subject:     "Build #456 succeeded",
			tenantPatterns: []SubtypePattern{
				{
					Pattern:     "buildbot@*",
					PatternType: "notification/other",
					Priority:    10,
				},
			},
			want: SubtypeNotificationOther,
		},
		{
			name:        "tenant pattern with higher priority takes precedence",
			fromAddress: "noreply@service.com",
			subject:     "Test",
			tenantPatterns: []SubtypePattern{
				{
					Pattern:     "noreply@service.com",
					PatternType: "notification/other",
					Priority:    5,
				},
			},
			want: SubtypeNotificationOther,
		},
		{
			name:        "fallback to default when no tenant pattern matches",
			fromAddress: "user@company.com",
			subject:     "Regular email",
			tenantPatterns: []SubtypePattern{
				{
					Pattern:     "bot@*",
					PatternType: "notification/other",
					Priority:    10,
				},
			},
			want: SubtypeEmailStandalone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				"Content-Type": "text/plain",
			}
			got := ClassifyContentSubtype(headers, tt.fromAddress, tt.subject, tt.tenantPatterns)
			if got != tt.want {
				t.Errorf("ClassifyContentSubtype() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestClassifyContentSubtype_EdgeCases tests edge cases like empty headers, nil inputs,
// and malformed data.
func TestClassifyContentSubtype_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		fromAddress string
		subject     string
		want        ContentSubtype
	}{
		{
			name:        "empty headers defaults to standalone",
			headers:     map[string]string{},
			fromAddress: "user@company.com",
			subject:     "Test",
			want:        SubtypeEmailStandalone,
		},
		{
			name:        "nil headers defaults to standalone",
			headers:     nil,
			fromAddress: "user@company.com",
			subject:     "Test",
			want:        SubtypeEmailStandalone,
		},
		{
			name: "empty from address defaults to standalone",
			headers: map[string]string{
				"Content-Type": "text/plain",
			},
			fromAddress: "",
			subject:     "Test",
			want:        SubtypeEmailStandalone,
		},
		{
			name: "empty subject defaults to standalone",
			headers: map[string]string{
				"Content-Type": "text/plain",
			},
			fromAddress: "user@company.com",
			subject:     "",
			want:        SubtypeEmailStandalone,
		},
		{
			name: "whitespace-only subject defaults to standalone",
			headers: map[string]string{
				"Content-Type": "text/plain",
			},
			fromAddress: "user@company.com",
			subject:     "   ",
			want:        SubtypeEmailStandalone,
		},
		{
			name: "malformed Content-Type ignored",
			headers: map[string]string{
				"Content-Type": "invalid/malformed/type",
			},
			fromAddress: "user@company.com",
			subject:     "Test",
			want:        SubtypeEmailStandalone,
		},
		{
			name: "Content-Type with charset parameter",
			headers: map[string]string{
				"Content-Type": "text/calendar; charset=UTF-8; method=REQUEST",
			},
			fromAddress: "organizer@company.com",
			subject:     "Meeting Invite",
			want:        SubtypeCalendarInvite,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyContentSubtype(tt.headers, tt.fromAddress, tt.subject, nil)
			if got != tt.want {
				t.Errorf("ClassifyContentSubtype() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestClassifyContentSubtype_PriorityOrder tests that classification happens in the
// correct priority order: calendar > notification > thread/forward > standalone.
func TestClassifyContentSubtype_PriorityOrder(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		fromAddress string
		subject     string
		inReplyTo   string
		want        ContentSubtype
		reason      string
	}{
		{
			name:        "calendar takes precedence over notification",
			contentType: "text/calendar",
			fromAddress: "noreply@company.com",
			subject:     "Meeting Invite",
			inReplyTo:   "",
			want:        SubtypeCalendarInvite,
			reason:      "Calendar Content-Type should override notification detection",
		},
		{
			name:        "calendar cancellation takes precedence over thread",
			contentType: "text/plain",
			fromAddress: "organizer@company.com",
			subject:     "Canceled: Re: Team Standup",
			inReplyTo:   "<msg@domain.com>",
			want:        SubtypeCalendarCancellation,
			reason:      "Calendar subject prefix should override thread detection",
		},
		{
			name:        "notification takes precedence over thread",
			contentType: "text/plain",
			fromAddress: "notifications@slack.com",
			subject:     "Re: Your message",
			inReplyTo:   "<msg@domain.com>",
			want:        SubtypeNotificationSlack,
			reason:      "Notification from-address should override thread detection",
		},
		{
			name:        "thread takes precedence over forward subject",
			contentType: "text/plain",
			fromAddress: "user@company.com",
			subject:     "Fwd: Re: Discussion",
			inReplyTo:   "<msg@domain.com>",
			want:        SubtypeEmailThread,
			reason:      "In-Reply-To should override Fwd: subject prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				"Content-Type": tt.contentType,
			}
			if tt.inReplyTo != "" {
				headers["In-Reply-To"] = tt.inReplyTo
			}
			got := ClassifyContentSubtype(headers, tt.fromAddress, tt.subject, nil)
			if got != tt.want {
				t.Errorf("ClassifyContentSubtype() = %q, want %q (reason: %s)", got, tt.want, tt.reason)
			}
		})
	}
}
