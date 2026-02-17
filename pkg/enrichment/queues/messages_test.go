package queues

import (
	"encoding/json"
	"testing"
	"time"
)

func TestIngestMessage_Interface(t *testing.T) {
	msg := &IngestMessage{
		SourceID:   123,
		TenantID:   "test-tenant",
		SourceType: "email",
		Priority:   PriorityNormal,
		IngestedAt: time.Now(),
		BatchID:    "batch-1",
	}

	// Test interface methods
	if msg.GetSourceID() != 123 {
		t.Errorf("GetSourceID() = %d, want 123", msg.GetSourceID())
	}
	if msg.GetTenantID() != "test-tenant" {
		t.Errorf("GetTenantID() = %s, want test-tenant", msg.GetTenantID())
	}
	if msg.GetPriority() != PriorityNormal {
		t.Errorf("GetPriority() = %d, want %d", msg.GetPriority(), PriorityNormal)
	}
	if msg.GetMessageType() != MessageTypeIngest {
		t.Errorf("GetMessageType() = %s, want %s", msg.GetMessageType(), MessageTypeIngest)
	}
	if msg.GetBatchID() != "batch-1" {
		t.Errorf("GetBatchID() = %s, want batch-1", msg.GetBatchID())
	}
}

func TestEnrichmentMessage_Interface(t *testing.T) {
	msg := &EnrichmentMessage{
		SourceID:          456,
		TenantID:          "test-tenant",
		ContentType:       "email",
		ContentSubtype:    "thread",
		ProcessingProfile: "full_ai",
		ThreadID:          "thread-1",
		IsLatestInThread:  true,
		Priority:          PriorityHigh,
	}

	if msg.GetSourceID() != 456 {
		t.Errorf("GetSourceID() = %d, want 456", msg.GetSourceID())
	}
	if msg.GetMessageType() != MessageTypeEnrichment {
		t.Errorf("GetMessageType() = %s, want %s", msg.GetMessageType(), MessageTypeEnrichment)
	}
}

func TestAIMessage_Interface(t *testing.T) {
	projectID := int64(789)
	msg := &AIMessage{
		SourceID:          100,
		TenantID:          "test-tenant",
		ContentType:       "email",
		ProcessingProfile: "full_ai",
		ProjectID:         &projectID,
		EnrichmentID:      200,
		Priority:          PriorityLow,
	}

	if msg.GetSourceID() != 100 {
		t.Errorf("GetSourceID() = %d, want 100", msg.GetSourceID())
	}
	if msg.GetMessageType() != MessageTypeAI {
		t.Errorf("GetMessageType() = %s, want %s", msg.GetMessageType(), MessageTypeAI)
	}
}

func TestQueuedMessage_ParseMessage(t *testing.T) {
	// Create an ingest message
	ingestMsg := &IngestMessage{
		SourceID:   123,
		TenantID:   "test",
		SourceType: "email",
		Priority:   PriorityNormal,
	}

	msgBytes, _ := json.Marshal(ingestMsg)
	qm := &QueuedMessage{
		ID:          "msg-1",
		Message:     msgBytes,
		MessageType: MessageTypeIngest,
		Priority:    PriorityNormal,
		EnqueuedAt:  time.Now(),
	}

	parsed, err := qm.ParseMessage()
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}

	im, ok := parsed.(*IngestMessage)
	if !ok {
		t.Fatal("ParseMessage() did not return *IngestMessage")
	}

	if im.SourceID != 123 {
		t.Errorf("Parsed SourceID = %d, want 123", im.SourceID)
	}
}

func TestQueuedMessage_ParseMessage_UnknownType(t *testing.T) {
	qm := &QueuedMessage{
		ID:          "msg-1",
		Message:     []byte("{}"),
		MessageType: MessageType("unknown"),
	}

	_, err := qm.ParseMessage()
	if err != ErrUnknownMessageType {
		t.Errorf("ParseMessage() error = %v, want %v", err, ErrUnknownMessageType)
	}
}

func TestDefaultQueueConfigs(t *testing.T) {
	configs := DefaultQueueConfigs()

	expected := []string{"enrichment:ingest", "enrichment:process", "enrichment:ai"}
	for _, name := range expected {
		if _, ok := configs[name]; !ok {
			t.Errorf("DefaultQueueConfigs() missing %s", name)
		}
	}

	// Check AI queue has longer visibility timeout
	aiConfig := configs["enrichment:ai"]
	if aiConfig.VisibilityTimeout < configs["enrichment:ingest"].VisibilityTimeout {
		t.Error("AI queue should have longer visibility timeout than ingest queue")
	}
}
