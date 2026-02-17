package query

import (
	"testing"
	"time"
)

func TestTimeRange(t *testing.T) {
	tr := NewTimeRange(24 * time.Hour)

	if tr.Start.After(tr.End) {
		t.Error("Start should be before End")
	}

	duration := tr.End.Sub(tr.Start)
	// Allow 1 second tolerance for test execution time
	if duration < 23*time.Hour || duration > 25*time.Hour {
		t.Errorf("Duration = %v, expected ~24h", duration)
	}
}

func TestLast7Days(t *testing.T) {
	tr := Last7Days()

	expected := 7 * 24 * time.Hour
	actual := tr.End.Sub(tr.Start)

	if actual < expected-time.Hour || actual > expected+time.Hour {
		t.Errorf("Last7Days duration = %v, expected ~%v", actual, expected)
	}
}

func TestLast30Days(t *testing.T) {
	tr := Last30Days()

	expected := 30 * 24 * time.Hour
	actual := tr.End.Sub(tr.Start)

	if actual < expected-time.Hour || actual > expected+time.Hour {
		t.Errorf("Last30Days duration = %v, expected ~%v", actual, expected)
	}
}

func TestDefaultPagination(t *testing.T) {
	p := DefaultPagination()

	if p.Limit != 50 {
		t.Errorf("Default limit = %d, want 50", p.Limit)
	}
	if p.Offset != 0 {
		t.Errorf("Default offset = %d, want 0", p.Offset)
	}
}

func TestAssertionFilters(t *testing.T) {
	projectID := int64(123)
	isCurrent := true

	filters := AssertionFilters{
		Type:      "action",
		Status:    "open",
		ProjectID: &projectID,
		IsCurrent: &isCurrent,
		TenantID:  "tenant-1",
	}

	if filters.Type != "action" {
		t.Errorf("Type = %s, want action", filters.Type)
	}
	if *filters.ProjectID != 123 {
		t.Errorf("ProjectID = %d, want 123", *filters.ProjectID)
	}
	if !*filters.IsCurrent {
		t.Error("IsCurrent should be true")
	}
}

func TestPeopleFilters(t *testing.T) {
	isInternal := true
	hasRecent := false

	filters := PeopleFilters{
		AccountType: "person",
		IsInternal:  &isInternal,
		HasRecent:   &hasRecent,
		TenantID:    "tenant-1",
	}

	if filters.AccountType != "person" {
		t.Errorf("AccountType = %s, want person", filters.AccountType)
	}
	if !*filters.IsInternal {
		t.Error("IsInternal should be true")
	}
	if *filters.HasRecent {
		t.Error("HasRecent should be false")
	}
}

func TestEnrichmentStatus(t *testing.T) {
	completedAt := time.Now()
	status := EnrichmentStatus{
		SourceID:          123,
		ContentType:       "email",
		ContentSubtype:    "thread",
		ProcessingProfile: "full_ai",
		Stages: []StageStatus{
			{
				Name:        "classification",
				Status:      "completed",
				DurationMs:  5,
				Processor:   "ContentClassifier",
				CompletedAt: &completedAt,
			},
			{
				Name:       "ai_processing",
				Status:     "completed",
				DurationMs: 1200,
				Processor:  "LLMExtractor",
				Outputs:    []string{"embedding", "summary", "assertions"},
			},
		},
		TotalDurationMs: 1205,
	}

	if status.SourceID != 123 {
		t.Errorf("SourceID = %d, want 123", status.SourceID)
	}
	if len(status.Stages) != 2 {
		t.Errorf("Stages count = %d, want 2", len(status.Stages))
	}
	if status.Stages[0].Name != "classification" {
		t.Errorf("First stage = %s, want classification", status.Stages[0].Name)
	}
}

func TestEnrichmentStats(t *testing.T) {
	stats := EnrichmentStats{
		TenantID:        "tenant-1",
		TimeRange:       Last7Days(),
		TotalProcessed:  1000,
		TotalFailed:     10,
		TotalSkipped:    50,
		AvgProcessingMs: 150.5,
		QueueDepths: QueueDepths{
			Ingest:     25,
			Enrichment: 150,
			AI:         80,
			DLQ:        3,
		},
		ClassificationStats: map[string]int64{
			"email":    800,
			"calendar": 150,
			"document": 50,
		},
		AIStats: AIStats{
			TotalOperations:  900,
			TotalTokens:      450000,
			AvgLatencyMs:     1200,
			ParseSuccessRate: 0.95,
		},
	}

	if stats.TotalProcessed != 1000 {
		t.Errorf("TotalProcessed = %d, want 1000", stats.TotalProcessed)
	}
	if stats.QueueDepths.Enrichment != 150 {
		t.Errorf("Enrichment queue depth = %d, want 150", stats.QueueDepths.Enrichment)
	}
	if stats.AIStats.ParseSuccessRate != 0.95 {
		t.Errorf("ParseSuccessRate = %f, want 0.95", stats.AIStats.ParseSuccessRate)
	}
}

func TestThread(t *testing.T) {
	thread := Thread{
		ID:              "thread-123",
		Subject:         "Project Update",
		MessageCount:    5,
		ParticipantCount: 3,
		HasActions:      true,
		HasDecisions:    false,
	}

	if thread.ID != "thread-123" {
		t.Errorf("ID = %s, want thread-123", thread.ID)
	}
	if !thread.HasActions {
		t.Error("HasActions should be true")
	}
	if thread.HasDecisions {
		t.Error("HasDecisions should be false")
	}
}

func TestJiraTicket(t *testing.T) {
	ticket := JiraTicket{
		Key:            "OUT-697",
		Summary:        "Implement feature X",
		Status:         "In Progress",
		StatusCategory: "in_progress",
		Type:           "story",
		Priority:       "medium",
		ProjectKey:     "OUT",
		ReferenceCount: 15,
	}

	if ticket.Key != "OUT-697" {
		t.Errorf("Key = %s, want OUT-697", ticket.Key)
	}
	if ticket.StatusCategory != "in_progress" {
		t.Errorf("StatusCategory = %s, want in_progress", ticket.StatusCategory)
	}
}

func TestActivityItem(t *testing.T) {
	sourceID := int64(123)
	item := ActivityItem{
		Type:        "decision",
		Description: "Decided to use approach A",
		SourceID:    &sourceID,
		ActorName:   "John Doe",
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"rationale": "Better performance",
		},
	}

	if item.Type != "decision" {
		t.Errorf("Type = %s, want decision", item.Type)
	}
	if *item.SourceID != 123 {
		t.Errorf("SourceID = %d, want 123", *item.SourceID)
	}
	if item.Metadata["rationale"] != "Better performance" {
		t.Error("Metadata rationale mismatch")
	}
}
