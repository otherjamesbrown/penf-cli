package classification

import (
	"testing"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
)

// TestClassifySourceSystem verifies all source_system classification rules.
// This test covers all 11 acceptance criteria from the Wave 2 spec.
func TestClassifySourceSystem(t *testing.T) {
	tests := []struct {
		name           string
		fromAddress    string
		subject        string
		messageID      string
		headers        map[string]string
		wantSourceSystem enrichment.SourceSystem
		wantDetectedVia  string
		acceptanceCriteria string
	}{
		// AC1: JIRA detection - from_address containing "jira@"
		{
			name:           "jira from address with jira@",
			fromAddress:    "jira@company.atlassian.net",
			subject:        "Issue Updated",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemJira,
			wantDetectedVia:  "jira",
			acceptanceCriteria: "AC1: from_address contains jira@",
		},
		// AC1: JIRA detection - message_id containing "@Atlassian.JIRA"
		{
			name:           "jira message_id with @Atlassian.JIRA",
			fromAddress:    "notifications@company.com",
			subject:        "Issue Updated",
			messageID:      "<12345@Atlassian.JIRA>",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemJira,
			wantDetectedVia:  "jira",
			acceptanceCriteria: "AC1: message_id contains @Atlassian.JIRA",
		},
		// AC1: JIRA detection - subject prefix "[TRACK-JIRA]"
		{
			name:           "jira subject prefix [TRACK-JIRA]",
			fromAddress:    "notifications@company.com",
			subject:        "[TRACK-JIRA] New issue assigned",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemJira,
			wantDetectedVia:  "jira",
			acceptanceCriteria: "AC1: subject starts with [TRACK-JIRA]",
		},

		// AC2: AHA detection - from_address matching *@*.mailer.aha.io
		{
			name:           "aha from address *@*.mailer.aha.io",
			fromAddress:    "noreply@product.mailer.aha.io",
			subject:        "Feature request updated",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemAha,
			wantDetectedVia:  "aha",
			acceptanceCriteria: "AC2: from matches *@*.mailer.aha.io",
		},
		// AC2: AHA detection - subject prefix "[AHA]"
		{
			name:           "aha subject prefix [AHA]",
			fromAddress:    "product@company.com",
			subject:        "[AHA] New feature request",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemAha,
			wantDetectedVia:  "aha",
			acceptanceCriteria: "AC2: subject starts with [AHA]",
		},

		// AC3: Google Docs detection - from matches *-noreply@docs.google.com
		{
			name:           "google docs from *-noreply@docs.google.com",
			fromAddress:    "comments-noreply@docs.google.com",
			subject:        "New comment on document",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemGoogleDocs,
			wantDetectedVia:  "google_docs",
			acceptanceCriteria: "AC3: from matches *-noreply@docs.google.com",
		},
		// AC3: Google Docs detection - from = drive-shares-dm-noreply@google.com
		{
			name:           "google docs from drive-shares-dm-noreply@google.com",
			fromAddress:    "drive-shares-dm-noreply@google.com",
			subject:        "Document shared with you",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemGoogleDocs,
			wantDetectedVia:  "google_docs",
			acceptanceCriteria: "AC3: from = drive-shares-dm-noreply@google.com",
		},

		// AC4: Webex detection - from = messenger@webex.com
		{
			name:           "webex from messenger@webex.com",
			fromAddress:    "messenger@webex.com",
			subject:        "New message from team",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemWebex,
			wantDetectedVia:  "webex",
			acceptanceCriteria: "AC4: from = messenger@webex.com",
		},

		// AC5: Smartsheet detection - from matching *@*.smartsheet.com
		{
			name:           "smartsheet from *@*.smartsheet.com",
			fromAddress:    "notifications@app.smartsheet.com",
			subject:        "Sheet updated",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemSmartsheet,
			wantDetectedVia:  "smartsheet",
			acceptanceCriteria: "AC5: from matches *@*.smartsheet.com",
		},

		// AC6: Auto-reply detection - subject starting with "Automatic reply:"
		{
			name:           "auto-reply subject Automatic reply:",
			fromAddress:    "user@company.com",
			subject:        "Automatic reply: Out of office",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemAutoReply,
			wantDetectedVia:  "auto_reply",
			acceptanceCriteria: "AC6: subject starts with 'Automatic reply:'",
		},

		// AC7: Calendar cancellation - subject starting with "Canceled:" or "Cancelled:"
		{
			name:           "calendar cancelled with Canceled:",
			fromAddress:    "user@company.com",
			subject:        "Canceled: Team Meeting",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemOutlookCalendar,
			wantDetectedVia:  "calendar_cancel",
			acceptanceCriteria: "AC7: subject starts with 'Canceled:'",
		},
		{
			name:           "calendar cancelled with Cancelled:",
			fromAddress:    "user@company.com",
			subject:        "Cancelled: Project Review",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemOutlookCalendar,
			wantDetectedVia:  "calendar_cancel",
			acceptanceCriteria: "AC7: subject starts with 'Cancelled:'",
		},
		// AC7: Calendar invite - has calendar attachment or Content-Type contains text/calendar
		{
			name:           "calendar invite with text/calendar content-type",
			fromAddress:    "user@company.com",
			subject:        "Team Meeting",
			messageID:      "",
			headers:        map[string]string{"Content-Type": "text/calendar; charset=utf-8"},
			wantSourceSystem: enrichment.SourceSystemOutlookCalendar,
			wantDetectedVia:  "calendar_invite",
			acceptanceCriteria: "AC7: Content-Type contains text/calendar",
		},

		// AC8: Human email fallback - no patterns match
		{
			name:           "human email no patterns match",
			fromAddress:    "colleague@company.com",
			subject:        "Project update",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemHumanEmail,
			wantDetectedVia:  "default",
			acceptanceCriteria: "AC8: no patterns match → human_email",
		},

		// AC9: OR logic within rules - any single condition within a rule is sufficient
		{
			name:           "jira OR logic - only from_address matches",
			fromAddress:    "jira@company.com",
			subject:        "Normal subject",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemJira,
			wantDetectedVia:  "jira",
			acceptanceCriteria: "AC9: OR logic - from_address matches jira@ (other conditions don't)",
		},
		{
			name:           "jira OR logic - only message_id matches",
			fromAddress:    "normal@company.com",
			subject:        "Normal subject",
			messageID:      "<abc@Atlassian.JIRA>",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemJira,
			wantDetectedVia:  "jira",
			acceptanceCriteria: "AC9: OR logic - message_id matches @Atlassian.JIRA (other conditions don't)",
		},
		{
			name:           "jira OR logic - only subject matches",
			fromAddress:    "normal@company.com",
			subject:        "[TRACK-JIRA] Issue created",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemJira,
			wantDetectedVia:  "jira",
			acceptanceCriteria: "AC9: OR logic - subject matches [TRACK-JIRA] (other conditions don't)",
		},

		// AC10: Priority ordering - JIRA notification with "Canceled:" subject → jira (higher priority)
		{
			name:           "priority ordering jira wins over calendar",
			fromAddress:    "jira@company.atlassian.net",
			subject:        "Canceled: Sprint Planning",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemJira,
			wantDetectedVia:  "jira",
			acceptanceCriteria: "AC10: JIRA (priority 1) wins over calendar_cancel (priority 7)",
		},

		// AC11: Independence from content_subtype - JIRA reply should get source_system=jira
		// regardless of content_subtype
		{
			name:           "jira independent of content_subtype",
			fromAddress:    "jira@company.atlassian.net",
			subject:        "Re: [PROJ-123] Issue Updated", // Thread reply subject
			messageID:      "<msg@Atlassian.JIRA>",
			headers:        map[string]string{"In-Reply-To": "<previous@company.com>"}, // Would make it a thread
			wantSourceSystem: enrichment.SourceSystemJira,
			wantDetectedVia:  "jira",
			acceptanceCriteria: "AC11: JIRA classification independent of thread/reply indicators",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifySourceSystem(tt.fromAddress, tt.subject, tt.messageID, tt.headers)

			if got != tt.wantSourceSystem {
				t.Errorf("ClassifySourceSystem() = %v, want %v\nAcceptance Criteria: %s",
					got, tt.wantSourceSystem, tt.acceptanceCriteria)
			}

			// Note: wantDetectedVia is included for future verification once the classifier
			// implementation supports returning detection metadata
		})
	}
}

// TestClassifySourceSystem_CaseInsensitivity verifies that classification is case-insensitive.
func TestClassifySourceSystem_CaseInsensitivity(t *testing.T) {
	tests := []struct {
		name           string
		fromAddress    string
		subject        string
		wantSourceSystem enrichment.SourceSystem
	}{
		{
			name:           "JIRA uppercase from",
			fromAddress:    "JIRA@COMPANY.COM",
			subject:        "Issue",
			wantSourceSystem: enrichment.SourceSystemJira,
		},
		{
			name:           "jira lowercase from",
			fromAddress:    "jira@company.com",
			subject:        "Issue",
			wantSourceSystem: enrichment.SourceSystemJira,
		},
		{
			name:           "automatic reply mixed case",
			fromAddress:    "user@company.com",
			subject:        "AUTOMATIC REPLY: OOO",
			wantSourceSystem: enrichment.SourceSystemAutoReply,
		},
		{
			name:           "canceled mixed case",
			fromAddress:    "user@company.com",
			subject:        "CANCELED: Meeting",
			wantSourceSystem: enrichment.SourceSystemOutlookCalendar,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifySourceSystem(tt.fromAddress, tt.subject, "", map[string]string{})

			if got != tt.wantSourceSystem {
				t.Errorf("ClassifySourceSystem() = %v, want %v", got, tt.wantSourceSystem)
			}
		})
	}
}

// TestClassifySourceSystem_EdgeCases tests edge cases and boundary conditions.
func TestClassifySourceSystem_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		fromAddress    string
		subject        string
		messageID      string
		headers        map[string]string
		wantSourceSystem enrichment.SourceSystem
		description    string
	}{
		{
			name:           "empty inputs",
			fromAddress:    "",
			subject:        "",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemHumanEmail,
			description:    "All empty should default to human_email",
		},
		{
			name:           "nil headers",
			fromAddress:    "user@company.com",
			subject:        "Test",
			messageID:      "",
			headers:        nil,
			wantSourceSystem: enrichment.SourceSystemHumanEmail,
			description:    "Nil headers should be handled gracefully",
		},
		{
			name:           "whitespace subject prefix",
			fromAddress:    "user@company.com",
			subject:        "  Automatic reply: OOO",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemAutoReply,
			description:    "Leading whitespace should be trimmed before matching",
		},
		{
			name:           "partial jira match in from",
			fromAddress:    "jirasupport@company.com",
			subject:        "Test",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemJira,
			description:    "jira@ should match as substring (contains)",
		},
		{
			name:           "jira in middle of message_id",
			fromAddress:    "user@company.com",
			subject:        "Test",
			messageID:      "<prefix-abc@Atlassian.JIRA-suffix>",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemJira,
			description:    "@Atlassian.JIRA should match anywhere in message_id",
		},
		{
			name:           "calendar attachment metadata",
			fromAddress:    "user@company.com",
			subject:        "Meeting Invitation",
			messageID:      "",
			headers:        map[string]string{"X-MS-Has-Attach": "yes"},
			wantSourceSystem: enrichment.SourceSystemHumanEmail,
			description:    "Without Content-Type text/calendar, not classified as calendar (note: attachment detection may be future enhancement)",
		},
		{
			name:           "multiple domain parts smartsheet",
			fromAddress:    "noreply@us.app.smartsheet.com",
			subject:        "Update",
			messageID:      "",
			headers:        map[string]string{},
			wantSourceSystem: enrichment.SourceSystemSmartsheet,
			description:    "Should match *@*.smartsheet.com pattern with multiple subdomains",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifySourceSystem(tt.fromAddress, tt.subject, tt.messageID, tt.headers)

			if got != tt.wantSourceSystem {
				t.Errorf("ClassifySourceSystem() = %v, want %v\nDescription: %s",
					got, tt.wantSourceSystem, tt.description)
			}
		})
	}
}
