package events

import (
	"testing"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/ingest/eml"
)

func TestBaseEvent(t *testing.T) {
	event := NewBaseEvent("test.event")

	if event.EventType != "test.event" {
		t.Errorf("unexpected event type: %s", event.EventType)
	}
	if event.Source != "penfold" {
		t.Errorf("unexpected source: %s", event.Source)
	}
	if event.Version != "1.0" {
		t.Errorf("unexpected version: %s", event.Version)
	}
	if event.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestManualEmailIngestedEvent(t *testing.T) {
	event := ManualEmailIngestedEvent{
		BaseEvent: NewBaseEvent("manual_email.ingested"),
		SourceID:  42,
		TenantID:  "tenant-123",
		MessageID: "<test@example.com>",
		JobID:     "job-456",
		FromEmail: "sender@example.com",
		ToEmails:  []string{"recipient@example.com"},
		CcEmails:  []string{},
		EmailDate: time.Now(),
		ContentHash: "abc123",
		SourceTag:   "test-source",
	}

	if event.SourceID != 42 {
		t.Errorf("unexpected source id: %d", event.SourceID)
	}
	if event.EventType != "manual_email.ingested" {
		t.Errorf("unexpected event type: %s", event.EventType)
	}
}

func TestIngestJobProgressEvent(t *testing.T) {
	currentFile := "/path/to/file.eml"
	remaining := 60.0

	event := IngestJobProgressEvent{
		BaseEvent:                 NewBaseEvent("ingest_job.progress"),
		JobID:                     "job-123",
		TenantID:                  "tenant-123",
		TotalFiles:                100,
		ProcessedCount:            50,
		ImportedCount:             45,
		SkippedCount:              3,
		FailedCount:               2,
		CurrentFile:               &currentFile,
		ElapsedSeconds:            30.5,
		EstimatedRemainingSeconds: &remaining,
		Status:                    "running",
	}

	if event.ProcessedCount != 50 {
		t.Errorf("unexpected processed count: %d", event.ProcessedCount)
	}
	if *event.CurrentFile != currentFile {
		t.Errorf("unexpected current file: %s", *event.CurrentFile)
	}
}

func TestIngestJobCompletedEvent(t *testing.T) {
	startTime := time.Now().Add(-time.Hour)
	endTime := time.Now()

	event := IngestJobCompletedEvent{
		BaseEvent:       NewBaseEvent("ingest_job.completed"),
		JobID:           "job-123",
		TenantID:        "tenant-123",
		SourceTag:       "archive-2024",
		TotalFiles:      100,
		ImportedCount:   95,
		SkippedCount:    3,
		FailedCount:     2,
		StartedAt:       startTime,
		CompletedAt:     endTime,
		DurationSeconds: endTime.Sub(startTime).Seconds(),
		Success:         true,
		FinalStatus:     "completed",
	}

	if !event.Success {
		t.Error("expected success to be true")
	}
	if event.DurationSeconds < 3600 {
		t.Errorf("unexpected duration: %f", event.DurationSeconds)
	}
}

func TestEmailIngestedParams(t *testing.T) {
	email := &eml.ParsedEmail{
		MessageID:   "<test@example.com>",
		ContentHash: "abc123",
		From:        eml.Address{Email: "from@example.com", Name: "Sender"},
		To:          []eml.Address{{Email: "to@example.com", Name: "Recipient"}},
		Subject:     "Test Subject",
		Date:        time.Now(),
		FilePath:    "/path/to/test.eml",
	}

	params := EmailIngestedParams{
		SourceID:  1,
		TenantID:  "tenant-123",
		JobID:     "job-456",
		Email:     email,
		SourceTag: "test",
		Labels:    []string{"label1", "label2"},
	}

	if params.Email.MessageID != "<test@example.com>" {
		t.Errorf("unexpected message id: %s", params.Email.MessageID)
	}
	if len(params.Labels) != 2 {
		t.Errorf("unexpected labels count: %d", len(params.Labels))
	}
}

func TestJobProgressParams(t *testing.T) {
	file := "/current/file.eml"
	remaining := 30.0

	params := JobProgressParams{
		JobID:                     "job-123",
		TenantID:                  "tenant-123",
		TotalFiles:                100,
		ProcessedCount:            75,
		ImportedCount:             70,
		SkippedCount:              3,
		FailedCount:               2,
		CurrentFile:               &file,
		ElapsedSeconds:            60.0,
		EstimatedRemainingSeconds: &remaining,
		Status:                    "running",
	}

	if params.ProcessedCount != 75 {
		t.Errorf("unexpected processed count: %d", params.ProcessedCount)
	}
	if *params.CurrentFile != file {
		t.Errorf("unexpected current file: %s", *params.CurrentFile)
	}
}

func TestJobCompletedParams(t *testing.T) {
	now := time.Now()

	params := JobCompletedParams{
		JobID:         "job-123",
		TenantID:      "tenant-123",
		SourceTag:     "archive",
		TotalFiles:    100,
		ImportedCount: 98,
		SkippedCount:  1,
		FailedCount:   1,
		StartedAt:     now.Add(-time.Hour),
		CompletedAt:   now,
		Success:       true,
		FinalStatus:   "completed",
	}

	if !params.Success {
		t.Error("expected success to be true")
	}
	if params.TotalFiles != 100 {
		t.Errorf("unexpected total files: %d", params.TotalFiles)
	}
}

func TestChannelConstants(t *testing.T) {
	if ChannelManualEmailIngested != "events.manual_email.ingested" {
		t.Errorf("unexpected channel: %s", ChannelManualEmailIngested)
	}
	if ChannelIngestJobProgress != "events.ingest_job.progress" {
		t.Errorf("unexpected channel: %s", ChannelIngestJobProgress)
	}
	if ChannelIngestJobCompleted != "events.ingest_job.completed" {
		t.Errorf("unexpected channel: %s", ChannelIngestJobCompleted)
	}
}
