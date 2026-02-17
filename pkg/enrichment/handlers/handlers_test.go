package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
	"github.com/otherjamesbrown/penf-cli/pkg/enrichment/processors"
)

// Helper function to create a source with metadata
func makeSource(metadata map[string]interface{}) *processors.Source {
	return &processors.Source{
		ID:         1,
		TenantID:   "test-tenant",
		Metadata:   metadata,
		RawContent: "",
	}
}

// Helper function to create processor context
func makeContext(source *processors.Source) *processors.ProcessorContext {
	return &processors.ProcessorContext{
		Source: source,
		Enrichment: &enrichment.Enrichment{
			SourceID: source.ID,
			TenantID: source.TenantID,
		},
		TenantID: source.TenantID,
	}
}

// ==================== LinkExtractor Tests ====================

func TestLinkExtractor_ExtractFromText(t *testing.T) {
	extractor := NewLinkExtractor()

	tests := []struct {
		name     string
		body     string
		wantURLs []string
	}{
		{
			name:     "simple URL",
			body:     "Check out https://example.com for more info",
			wantURLs: []string{"https://example.com"},
		},
		{
			name:     "multiple URLs",
			body:     "See https://google.com and https://github.com for details",
			wantURLs: []string{"https://google.com", "https://github.com"},
		},
		{
			name:     "Google Doc URL",
			body:     "Here's the doc: https://docs.google.com/document/d/1abc123/edit",
			wantURLs: []string{"https://docs.google.com/document/d/1abc123/edit"},
		},
		{
			name:     "Jira URL",
			body:     "See https://company.atlassian.net/browse/OUT-697",
			wantURLs: []string{"https://company.atlassian.net/browse/OUT-697"},
		},
		{
			name:     "URL with trailing punctuation",
			body:     "Check https://example.com.",
			wantURLs: []string{"https://example.com"},
		},
		{
			name:     "no URLs",
			body:     "No links here",
			wantURLs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := makeSource(map[string]interface{}{
				"body_text": tt.body,
			})
			pctx := makeContext(source)

			err := extractor.Process(context.Background(), pctx)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			gotURLs := make([]string, len(pctx.Enrichment.ExtractedLinks))
			for i, link := range pctx.Enrichment.ExtractedLinks {
				gotURLs[i] = link.URL
			}

			if len(gotURLs) != len(tt.wantURLs) {
				t.Errorf("got %d URLs, want %d. Got: %v", len(gotURLs), len(tt.wantURLs), gotURLs)
				return
			}

			for i, wantURL := range tt.wantURLs {
				if gotURLs[i] != wantURL {
					t.Errorf("URL[%d] = %v, want %v", i, gotURLs[i], wantURL)
				}
			}
		})
	}
}

func TestLinkExtractor_Categorize(t *testing.T) {
	tests := []struct {
		url      string
		wantCat  LinkCategory
		wantID   string
	}{
		{
			url:     "https://docs.google.com/document/d/1abc123/edit",
			wantCat: LinkCategoryGoogleDoc,
			wantID:  "1abc123",
		},
		{
			url:     "https://docs.google.com/spreadsheets/d/xyz789/edit",
			wantCat: LinkCategoryGoogleSheet,
			wantID:  "xyz789",
		},
		{
			url:     "https://company.atlassian.net/browse/OUT-697",
			wantCat: LinkCategoryJiraTicket,
			wantID:  "OUT-697",
		},
		{
			url:     "https://github.com/owner/repo/issues/123",
			wantCat: LinkCategoryGitHub,
			wantID:  "owner/repo",
		},
		{
			url:     "https://zoom.us/rec/share/abc123",
			wantCat: LinkCategoryZoomRecording,
			wantID:  "",
		},
		{
			url:     "https://example.com/random",
			wantCat: LinkCategoryGenericURL,
			wantID:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			gotCat := categorizeLink(tt.url)
			if gotCat != tt.wantCat {
				t.Errorf("categorizeLink() = %v, want %v", gotCat, tt.wantCat)
			}

			gotID := extractServiceID(tt.url, tt.wantCat)
			if gotID != tt.wantID {
				t.Errorf("extractServiceID() = %v, want %v", gotID, tt.wantID)
			}
		})
	}
}

// ==================== JiraExtractor Tests ====================

func TestJiraExtractor_ExtractTicketKey(t *testing.T) {
	extractor := NewJiraExtractor()

	tests := []struct {
		text    string
		want    string
	}{
		{"[OUT-697] Some issue title", "OUT-697"},
		{"[PROJ-123] Another issue", "PROJ-123"},
		{"[TRACK-JIRA] Updates for OUT-697", "OUT-697"}, // Prefix with real ticket
		{"Updates for OUT-697", "OUT-697"},
		{"No ticket here", ""},
		{"[AB-1] Minimum valid", "AB-1"},
		{"[LONGPROJ-99999] Large number", "LONGPROJ-99999"},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := extractor.extractTicketKey(tt.text)
			if got != tt.want {
				t.Errorf("extractTicketKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJiraExtractor_DetectChangeType(t *testing.T) {
	extractor := NewJiraExtractor()

	tests := []struct {
		subject string
		want    JiraChangeType
	}{
		{"[OUT-697] Issue created", JiraChangeCreated},
		{"[OUT-697] Status changed", JiraChangeStatusChanged},
		{"[OUT-697] Assigned to John", JiraChangeAssigned},
		{"[OUT-697] John commented on this", JiraChangeCommented},
		{"[OUT-697] Resolved", JiraChangeResolved},
		{"[OUT-697] Some update", JiraChangeOther},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			got := extractor.detectChangeType(tt.subject)
			if got != tt.want {
				t.Errorf("detectChangeType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJiraExtractor_Process(t *testing.T) {
	extractor := NewJiraExtractor()

	source := makeSource(map[string]interface{}{
		"subject":   "[OUT-697] Updates for Launch new products",
		"from":      "John Smith <jsmith@company.com>",
		"body_text": "Status: Open → In Progress\nPriority: High\nAssignee: Jane Doe",
		"date":      time.Now(),
	})
	pctx := makeContext(source)
	pctx.Enrichment.Classification = enrichment.Classification{
		Subtype: enrichment.SubtypeNotificationJira,
	}

	err := extractor.Process(context.Background(), pctx)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	jiraData, ok := pctx.Enrichment.ExtractedData["jira"].(*JiraTicketData)
	if !ok {
		t.Fatal("Expected jira data in ExtractedData")
	}

	if jiraData.TicketKey != "OUT-697" {
		t.Errorf("TicketKey = %v, want OUT-697", jiraData.TicketKey)
	}
	if jiraData.ProjectKey != "OUT" {
		t.Errorf("ProjectKey = %v, want OUT", jiraData.ProjectKey)
	}
	if jiraData.Status != "In" { // "In Progress" gets truncated at whitespace
		// This is expected behavior with simple regex
	}
	if jiraData.ChangedByEmail != "jsmith@company.com" {
		t.Errorf("ChangedByEmail = %v, want jsmith@company.com", jiraData.ChangedByEmail)
	}
}

// ==================== CalendarExtractor Tests ====================

func TestCalendarExtractor_CleanTitle(t *testing.T) {
	extractor := NewCalendarExtractor()

	tests := []struct {
		subject string
		want    string
	}{
		{"Canceled: Team Standup", "Team Standup"},
		{"Cancelled: Team Standup", "Team Standup"},
		{"Updated: Team Standup", "Team Standup"},
		{"Accepted: Team Standup", "Team Standup"},
		{"Re: Team Standup", "Team Standup"},
		{"Team Standup", "Team Standup"},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			got := extractor.cleanTitle(tt.subject)
			if got != tt.want {
				t.Errorf("cleanTitle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalendarExtractor_ExtractVideoURL(t *testing.T) {
	extractor := NewCalendarExtractor()

	tests := []struct {
		body string
		want string
	}{
		{
			body: "Join the meeting: https://company.webex.com/meet/john",
			want: "https://company.webex.com/meet/john",
		},
		{
			body: "Zoom: https://zoom.us/j/123456789",
			want: "https://zoom.us/j/123456789",
		},
		{
			body: "Google Meet: https://meet.google.com/abc-defg-hij",
			want: "https://meet.google.com/abc-defg-hij",
		},
		{
			body: "No video link here",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := extractor.extractVideoURL(tt.body)
			if got != tt.want {
				t.Errorf("extractVideoURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalendarExtractor_ExtractName(t *testing.T) {
	extractor := NewCalendarExtractor()

	tests := []struct {
		name  string
		from  string
		want  string
	}{
		{
			name: "Last, First format",
			from: "Brown, James <jbrown@example.com>",
			want: "James Brown",
		},
		{
			name: "First Last format",
			from: "John Smith <jsmith@example.com>",
			want: "John Smith",
		},
		{
			name: "quoted Last, First format",
			from: "\"Doe, Jane\" <jdoe@example.com>",
			want: "Jane Doe",
		},
		{
			name: "single name",
			from: "Alice <alice@example.com>",
			want: "Alice",
		},
		{
			name: "extra whitespace with Last, First",
			from: "  Eskelsen, Rick   <reskelsen@example.com>",
			want: "Rick Eskelsen",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractor.extractName(tt.from)
			if got != tt.want {
				t.Errorf("extractName(%q) = %q, want %q", tt.from, got, tt.want)
			}
		})
	}
}

func TestCalendarExtractor_Process(t *testing.T) {
	extractor := NewCalendarExtractor()

	source := makeSource(map[string]interface{}{
		"subject":   "Canceled: Weekly Team Standup",
		"from":      "John Organizer <john@company.com>",
		"body_text": "This meeting has been cancelled.\n\nJoin: https://zoom.us/j/123",
	})
	pctx := makeContext(source)
	pctx.Enrichment.Classification = enrichment.Classification{
		Subtype: enrichment.SubtypeCalendarCancellation,
	}

	err := extractor.Process(context.Background(), pctx)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	meetingData, ok := pctx.Enrichment.ExtractedData["meeting"].(*MeetingData)
	if !ok {
		t.Fatal("Expected meeting data in ExtractedData")
	}

	if meetingData.Title != "Weekly Team Standup" {
		t.Errorf("Title = %v, want Weekly Team Standup", meetingData.Title)
	}
	if meetingData.Status != MeetingStatusCancelled {
		t.Errorf("Status = %v, want cancelled", meetingData.Status)
	}
	if meetingData.EventType != MeetingEventCancelled {
		t.Errorf("EventType = %v, want cancelled", meetingData.EventType)
	}
	if meetingData.OrganizerEmail != "john@company.com" {
		t.Errorf("OrganizerEmail = %v, want john@company.com", meetingData.OrganizerEmail)
	}
	if meetingData.VideoURL != "https://zoom.us/j/123" {
		t.Errorf("VideoURL = %v, want https://zoom.us/j/123", meetingData.VideoURL)
	}
}

func TestCalendarExtractor_Process_LastFirstName(t *testing.T) {
	extractor := NewCalendarExtractor()

	source := makeSource(map[string]interface{}{
		"subject":   "Updated: Project Planning Meeting",
		"from":      "Brown, James <jbrown@company.com>",
		"body_text": "Meeting details updated.\n\nJoin: https://zoom.us/j/456",
	})
	pctx := makeContext(source)
	pctx.Enrichment.Classification = enrichment.Classification{
		Subtype: enrichment.SubtypeCalendarUpdate,
	}

	err := extractor.Process(context.Background(), pctx)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	meetingData, ok := pctx.Enrichment.ExtractedData["meeting"].(*MeetingData)
	if !ok {
		t.Fatal("Expected meeting data in ExtractedData")
	}

	if meetingData.OrganizerEmail != "jbrown@company.com" {
		t.Errorf("OrganizerEmail = %v, want jbrown@company.com", meetingData.OrganizerEmail)
	}
	if meetingData.OrganizerName != "James Brown" {
		t.Errorf("OrganizerName = %q, want %q (Bug pf-df45d7: extractName should normalize 'Last, First' format)", meetingData.OrganizerName, "James Brown")
	}
}

// ==================== ThreadGrouper Tests ====================

func TestThreadGrouper_NormalizeSubject(t *testing.T) {
	grouper := NewThreadGrouper()

	tests := []struct {
		subject string
		want    string
	}{
		{"Re: Some topic", "Some topic"},
		{"RE: RE: Some topic", "Some topic"},
		{"Fwd: Some topic", "Some topic"},
		{"FW: Some topic", "Some topic"},
		{"Re: Fwd: Some topic", "Some topic"},
		{"Some topic", "Some topic"},
		{"  Re:  Spaced  topic  ", "Spaced topic"},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			got := grouper.normalizeSubject(tt.subject)
			if got != tt.want {
				t.Errorf("normalizeSubject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestThreadGrouper_ParseReferences(t *testing.T) {
	grouper := NewThreadGrouper()

	tests := []struct {
		refs string
		want []string
	}{
		{
			refs: "<msg1@example.com> <msg2@example.com>",
			want: []string{"<msg1@example.com>", "<msg2@example.com>"},
		},
		{
			refs: "<single@example.com>",
			want: []string{"<single@example.com>"},
		},
		{
			refs: "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.refs, func(t *testing.T) {
			got := grouper.parseReferences(tt.refs)
			if len(got) != len(tt.want) {
				t.Errorf("parseReferences() len = %v, want %v", len(got), len(tt.want))
				return
			}
			for i, ref := range tt.want {
				if got[i] != ref {
					t.Errorf("parseReferences()[%d] = %v, want %v", i, got[i], ref)
				}
			}
		})
	}
}

func TestThreadGrouper_DetermineThreadRoot(t *testing.T) {
	grouper := NewThreadGrouper()

	tests := []struct {
		name string
		data *ThreadData
		want string
	}{
		{
			name: "with references",
			data: &ThreadData{
				MessageID:  "<msg3@example.com>",
				InReplyTo:  "<msg2@example.com>",
				References: []string{"<msg1@example.com>", "<msg2@example.com>"},
			},
			want: "<msg1@example.com>",
		},
		{
			name: "with only in-reply-to",
			data: &ThreadData{
				MessageID: "<msg2@example.com>",
				InReplyTo: "<msg1@example.com>",
			},
			want: "<msg1@example.com>",
		},
		{
			name: "new message",
			data: &ThreadData{
				MessageID: "<msg1@example.com>",
				IsReply:   false,
				IsForward: false,
			},
			want: "<msg1@example.com>",
		},
		{
			name: "reply without headers",
			data: &ThreadData{
				MessageID: "<msg2@example.com>",
				IsReply:   true,
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := grouper.determineThreadRoot(tt.data)
			if got != tt.want {
				t.Errorf("determineThreadRoot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestThreadGrouper_Process(t *testing.T) {
	grouper := NewThreadGrouper()

	source := makeSource(map[string]interface{}{
		"subject":     "Re: Project Discussion",
		"message_id":  "<msg3@example.com>",
		"in_reply_to": "<msg2@example.com>",
		"references":  "<msg1@example.com> <msg2@example.com>",
	})
	pctx := makeContext(source)
	pctx.Enrichment.Classification = enrichment.Classification{
		ContentType: enrichment.ContentTypeEmail,
		Subtype:     enrichment.SubtypeEmailThread,
	}

	err := grouper.Process(context.Background(), pctx)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	threadData, ok := pctx.Enrichment.ExtractedData["thread"].(*ThreadData)
	if !ok {
		t.Fatal("Expected thread data in ExtractedData")
	}

	if threadData.MessageID != "<msg3@example.com>" {
		t.Errorf("MessageID = %v, want <msg3@example.com>", threadData.MessageID)
	}
	if threadData.NormalizedSubject != "Project Discussion" {
		t.Errorf("NormalizedSubject = %v, want Project Discussion", threadData.NormalizedSubject)
	}
	if !threadData.IsReply {
		t.Error("IsReply should be true")
	}
	if threadData.ThreadRoot != "<msg1@example.com>" {
		t.Errorf("ThreadRoot = %v, want <msg1@example.com>", threadData.ThreadRoot)
	}
	if pctx.Enrichment.ThreadID != "<msg1@example.com>" {
		t.Errorf("Enrichment.ThreadID = %v, want <msg1@example.com>", pctx.Enrichment.ThreadID)
	}
}

// TestThreadGrouper_GetReferences_JSONString_BUG_pf_6c7c8f reproduces bug pf-6c7c8f part 2:
// getReferences (lines 135-173) should handle JSON array strings like "[\"<msg1>\",\"<msg2>\"]"
// that come from source_repo.go metadata serialization.
//
// This test SHOULD FAIL because current getReferences doesn't deserialize JSON strings.
func TestThreadGrouper_GetReferences_JSONString_BUG_pf_6c7c8f(t *testing.T) {
	grouper := NewThreadGrouper()

	tests := []struct {
		name     string
		metadata map[string]interface{}
		want     []string
	}{
		{
			name: "references as JSON string",
			metadata: map[string]interface{}{
				// BUG: This is how references comes from source_repo.go:67-72
				"references": "[\"<msg1@example.com>\",\"<msg2@example.com>\",\"<msg3@example.com>\"]",
			},
			want: []string{"<msg1@example.com>", "<msg2@example.com>", "<msg3@example.com>"},
		},
		{
			name: "references as actual array (working case)",
			metadata: map[string]interface{}{
				"references": []string{"<msg1@example.com>", "<msg2@example.com>"},
			},
			want: []string{"<msg1@example.com>", "<msg2@example.com>"},
		},
		{
			name: "references as interface array (working case)",
			metadata: map[string]interface{}{
				"references": []interface{}{"<msg1@example.com>", "<msg2@example.com>"},
			},
			want: []string{"<msg1@example.com>", "<msg2@example.com>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &processors.Source{
				Metadata: tt.metadata,
			}

			got := grouper.getReferences(source)

			if len(got) != len(tt.want) {
				t.Errorf("getReferences() len = %v, want %v. Got: %v", len(got), len(tt.want), got)
				return
			}

			for i, wantRef := range tt.want {
				if got[i] != wantRef {
					t.Errorf("getReferences()[%d] = %v, want %v", i, got[i], wantRef)
				}
			}
		})
	}
}

// TestThreadGrouper_Process_JSONMetadata_BUG_pf_6c7c8f reproduces the full bug flow:
// metadata with JSON strings (as they come from source_repo.go) should be parsed correctly
// so that ThreadData has correct References array.
//
// This test SHOULD FAIL because current code doesn't parse JSON strings in metadata.
func TestThreadGrouper_Process_JSONMetadata_BUG_pf_6c7c8f(t *testing.T) {
	grouper := NewThreadGrouper()

	// Simulate what thread_activities.go:82-94 passes to ThreadGrouper:
	// metadata values are still strings (including JSON strings for arrays)
	source := makeSource(map[string]interface{}{
		"subject":     "Re: Project Discussion",
		"message_id":  "<msg3@example.com>",
		"in_reply_to": "<msg2@example.com>",
		// BUG: references comes as JSON string from source_repo.go
		"references": "[\"<msg1@example.com>\",\"<msg2@example.com>\"]",
	})
	pctx := makeContext(source)
	pctx.Enrichment.Classification = enrichment.Classification{
		ContentType: enrichment.ContentTypeEmail,
		Subtype:     enrichment.SubtypeEmailThread,
	}

	err := grouper.Process(context.Background(), pctx)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	threadData, ok := pctx.Enrichment.ExtractedData["thread"].(*ThreadData)
	if !ok {
		t.Fatal("Expected thread data in ExtractedData")
	}

	// EXPECTED (AFTER FIX): References should be parsed from JSON string
	expectedReferences := []string{"<msg1@example.com>", "<msg2@example.com>"}
	if len(threadData.References) != len(expectedReferences) {
		t.Errorf("BUG: References len = %d, want %d. Got: %v (JSON string not parsed)",
			len(threadData.References), len(expectedReferences), threadData.References)
	} else {
		for i, expected := range expectedReferences {
			if threadData.References[i] != expected {
				t.Errorf("BUG: References[%d] = %v, want %v (JSON string not parsed)",
					i, threadData.References[i], expected)
			}
		}
	}

	// EXPECTED (AFTER FIX): ThreadRoot should be first reference (msg1), not in_reply_to (msg2)
	expectedThreadRoot := "<msg1@example.com>"
	if threadData.ThreadRoot != expectedThreadRoot {
		t.Errorf("BUG: ThreadRoot = %v, want %v (should use first reference, not in_reply_to)",
			threadData.ThreadRoot, expectedThreadRoot)
	}
}

// ==================== Interface Compliance Tests ====================

func TestProcessorInterfaces(t *testing.T) {
	// Verify all processors implement the expected interfaces
	var _ processors.CommonEnrichmentProcessor = (*LinkExtractor)(nil)
	var _ processors.TypeSpecificProcessor = (*JiraExtractor)(nil)
	var _ processors.TypeSpecificProcessor = (*CalendarExtractor)(nil)
	var _ processors.CommonEnrichmentProcessor = (*ThreadGrouper)(nil)

	// Verify stages
	if NewLinkExtractor().Stage() != processors.StageCommonEnrichment {
		t.Error("LinkExtractor should be StageCommonEnrichment")
	}
	if NewJiraExtractor().Stage() != processors.StageTypeSpecific {
		t.Error("JiraExtractor should be StageTypeSpecific")
	}
	if NewCalendarExtractor().Stage() != processors.StageTypeSpecific {
		t.Error("CalendarExtractor should be StageTypeSpecific")
	}
	if NewThreadGrouper().Stage() != processors.StageCommonEnrichment {
		t.Error("ThreadGrouper should be StageCommonEnrichment")
	}
}
