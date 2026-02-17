package classification

import (
	"context"
	"testing"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
	"github.com/otherjamesbrown/penf-cli/pkg/enrichment/processors"
)

func TestContentClassifier_Classify(t *testing.T) {
	classifier := NewContentClassifier()

	tests := []struct {
		name            string
		source          *processors.Source
		wantType        enrichment.ContentType
		wantSubtype     enrichment.ContentSubtype
		wantProfile     enrichment.ProcessingProfile
		wantDetectedVia string
	}{
		{
			name: "calendar cancellation",
			source: &processors.Source{
				ID:       1,
				TenantID: "test-tenant",
				Metadata: map[string]interface{}{
					"subject": "Canceled: Weekly Standup",
					"attachments": []interface{}{
						map[string]interface{}{
							"filename":  "invite.ics",
							"mime_type": "text/calendar",
						},
					},
				},
			},
			wantType:        enrichment.ContentTypeCalendar,
			wantSubtype:     enrichment.SubtypeCalendarCancellation,
			wantProfile:     enrichment.ProfileStateTracking,
			wantDetectedVia: "calendar_cancellation",
		},
		{
			name: "calendar response accepted",
			source: &processors.Source{
				ID:       2,
				TenantID: "test-tenant",
				Metadata: map[string]interface{}{
					"subject": "Accepted: Team Meeting",
				},
			},
			wantType:        enrichment.ContentTypeCalendar,
			wantSubtype:     enrichment.SubtypeCalendarResponse,
			wantProfile:     enrichment.ProfileStateTracking,
			wantDetectedVia: "calendar_response",
		},
		{
			name: "calendar invite",
			source: &processors.Source{
				ID:          3,
				TenantID:    "test-tenant",
				ContentType: "text/calendar",
				Metadata: map[string]interface{}{
					"subject": "Meeting: Project Review",
				},
			},
			wantType:        enrichment.ContentTypeCalendar,
			wantSubtype:     enrichment.SubtypeCalendarInvite,
			wantProfile:     enrichment.ProfileStateTracking,
			wantDetectedVia: "calendar_invite",
		},
		{
			name: "jira notification",
			source: &processors.Source{
				ID:       4,
				TenantID: "test-tenant",
				Metadata: map[string]interface{}{
					"from":    "jira@company.atlassian.net",
					"subject": "[PROJ-123] Issue Updated",
					"headers": map[string]interface{}{
						"Auto-Submitted": "auto-generated",
					},
				},
			},
			wantType:        enrichment.ContentTypeEmail,
			wantSubtype:     enrichment.SubtypeNotificationJira,
			wantProfile:     enrichment.ProfileMetadataOnly,
			wantDetectedVia: "notification_jira",
		},
		{
			name: "google doc notification",
			source: &processors.Source{
				ID:       5,
				TenantID: "test-tenant",
				Metadata: map[string]interface{}{
					"from":    "comments-noreply@docs.google.com",
					"subject": "New comment on 'Project Plan'",
				},
			},
			wantType:        enrichment.ContentTypeEmail,
			wantSubtype:     enrichment.SubtypeNotificationGoogle,
			wantProfile:     enrichment.ProfileMetadataOnly,
			wantDetectedVia: "notification_google",
		},
		{
			name: "slack notification",
			source: &processors.Source{
				ID:       6,
				TenantID: "test-tenant",
				Metadata: map[string]interface{}{
					"from":    "no-reply@slack.com",
					"subject": "New message in #general",
				},
			},
			wantType:        enrichment.ContentTypeEmail,
			wantSubtype:     enrichment.SubtypeNotificationSlack,
			wantProfile:     enrichment.ProfileMetadataOnly,
			wantDetectedVia: "notification_slack",
		},
		{
			name: "other automated notification",
			source: &processors.Source{
				ID:       7,
				TenantID: "test-tenant",
				Metadata: map[string]interface{}{
					"from":    "noreply@service.com",
					"subject": "Your weekly report",
					"headers": map[string]interface{}{
						"Precedence": "bulk",
					},
				},
			},
			wantType:        enrichment.ContentTypeEmail,
			wantSubtype:     enrichment.SubtypeNotificationOther,
			wantProfile:     enrichment.ProfileMetadataOnly,
			wantDetectedVia: "notification_other",
		},
		{
			name: "forwarded email",
			source: &processors.Source{
				ID:       8,
				TenantID: "test-tenant",
				Metadata: map[string]interface{}{
					"from":    "colleague@company.com",
					"subject": "FW: Important Information",
				},
			},
			wantType:        enrichment.ContentTypeEmail,
			wantSubtype:     enrichment.SubtypeEmailForward,
			wantProfile:     enrichment.ProfileFullAI,
			wantDetectedVia: "email_forward",
		},
		{
			name: "email thread reply",
			source: &processors.Source{
				ID:       9,
				TenantID: "test-tenant",
				Metadata: map[string]interface{}{
					"from":        "colleague@company.com",
					"subject":     "Re: Project Discussion",
					"in_reply_to": "<message-id@company.com>",
				},
			},
			wantType:        enrichment.ContentTypeEmail,
			wantSubtype:     enrichment.SubtypeEmailThread,
			wantProfile:     enrichment.ProfileFullAI,
			wantDetectedVia: "email_thread",
		},
		{
			name: "standalone email - default",
			source: &processors.Source{
				ID:       10,
				TenantID: "test-tenant",
				Metadata: map[string]interface{}{
					"from":    "colleague@company.com",
					"subject": "New Project Proposal",
				},
			},
			wantType:        enrichment.ContentTypeEmail,
			wantSubtype:     enrichment.SubtypeEmailStandalone,
			wantProfile:     enrichment.ProfileFullAI,
			wantDetectedVia: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification, err := classifier.Classify(context.Background(), tt.source)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}

			if classification.ContentType != tt.wantType {
				t.Errorf("ContentType = %v, want %v", classification.ContentType, tt.wantType)
			}
			if classification.Subtype != tt.wantSubtype {
				t.Errorf("Subtype = %v, want %v", classification.Subtype, tt.wantSubtype)
			}
			if classification.Profile != tt.wantProfile {
				t.Errorf("Profile = %v, want %v", classification.Profile, tt.wantProfile)
			}
			if classification.DetectedVia != tt.wantDetectedVia {
				t.Errorf("DetectedVia = %v, want %v", classification.DetectedVia, tt.wantDetectedVia)
			}
		})
	}
}

func TestExtractJiraTicket(t *testing.T) {
	tests := []struct {
		subject string
		want    string
	}{
		{"[PROJ-123] Issue Updated", "PROJ-123"},
		{"[ABC-1] Simple ticket", "ABC-1"},
		{"[LONGPROJ-99999] Big number", "LONGPROJ-99999"},
		{"No ticket here", ""},
		{"Multiple [PROJ-1] tickets [PROJ-2]", "PROJ-1"}, // First match
		{"[P-1] Too short prefix", ""}, // Minimum 2 chars
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			got := ExtractJiraTicket(tt.subject)
			if got != tt.want {
				t.Errorf("ExtractJiraTicket(%q) = %q, want %q", tt.subject, got, tt.want)
			}
		})
	}
}

func TestContentClassifier_Name(t *testing.T) {
	c := NewContentClassifier()
	if name := c.Name(); name != "ContentClassifier" {
		t.Errorf("Name() = %v, want ContentClassifier", name)
	}
}

func TestContentClassifier_Stage(t *testing.T) {
	c := NewContentClassifier()
	if stage := c.Stage(); stage != processors.StageClassification {
		t.Errorf("Stage() = %v, want %v", stage, processors.StageClassification)
	}
}

func TestSubjectStartsWithAny(t *testing.T) {
	tests := []struct {
		subject  string
		prefixes []string
		want     bool
	}{
		{"Canceled: Meeting", []string{"Canceled:", "Cancelled:"}, true},
		{"Cancelled: Meeting", []string{"Canceled:", "Cancelled:"}, true},
		{"canceled: Meeting", []string{"Canceled:", "Cancelled:"}, true}, // Case insensitive
		{"FW: Message", []string{"FW:", "Fwd:"}, true},
		{"Fwd: Message", []string{"FW:", "Fwd:"}, true},
		{"  FW: Message", []string{"FW:", "Fwd:"}, true}, // Leading whitespace
		{"RE: Reply", []string{"FW:", "Fwd:"}, false},
		{"", []string{"FW:", "Fwd:"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			got := subjectStartsWithAny(tt.subject, tt.prefixes...)
			if got != tt.want {
				t.Errorf("subjectStartsWithAny(%q, %v) = %v, want %v", tt.subject, tt.prefixes, got, tt.want)
			}
		})
	}
}
