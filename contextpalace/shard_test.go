package contextpalace

import (
	"testing"
	"time"
)

// TestShardStructure tests the Shard struct.
func TestShardStructure(t *testing.T) {
	now := time.Now()
	shard := &Shard{
		ID:        "pf-test123",
		Project:   "penfold",
		Title:     "Test Shard",
		Content:   "Test content",
		Type:      "task",
		Status:    "open",
		Owner:     "agent-mycroft",
		Priority:  2,
		Creator:   "agent-mycroft",
		Labels:    []string{"test", "example"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if shard.ID != "pf-test123" {
		t.Errorf("expected ID pf-test123, got %s", shard.ID)
	}

	if len(shard.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(shard.Labels))
	}

	if shard.ClosedAt != nil {
		t.Error("expected ClosedAt to be nil")
	}
}

// TestShardOptions tests the functional options pattern.
func TestShardOptions(t *testing.T) {
	opts := &shardOptions{
		priority: 2, // Default
	}

	// Apply options
	WithOwner("agent-test")(opts)
	WithPriority(1)(opts)
	WithLabels("label1", "label2")(opts)

	if opts.owner != "agent-test" {
		t.Errorf("expected owner agent-test, got %s", opts.owner)
	}

	if opts.priority != 1 {
		t.Errorf("expected priority 1, got %d", opts.priority)
	}

	if len(opts.labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(opts.labels))
	}
}

// TestUpdateOptions tests the update options pattern.
func TestUpdateOptions(t *testing.T) {
	opts := &updateOptions{}

	// Apply options
	status := "in_progress"
	title := "Updated Title"
	content := "Updated Content"
	priority := 1
	owner := "new-owner"
	expiresAt := time.Now().Add(24 * time.Hour)

	WithStatus(status)(opts)
	WithTitle(title)(opts)
	WithContent(content)(opts)
	WithUpdatePriority(priority)(opts)
	WithUpdateOwner(owner)(opts)
	WithExpiresAt(expiresAt)(opts)

	if opts.status == nil || *opts.status != status {
		t.Error("status not set correctly")
	}

	if opts.title == nil || *opts.title != title {
		t.Error("title not set correctly")
	}

	if opts.content == nil || *opts.content != content {
		t.Error("content not set correctly")
	}

	if opts.priority == nil || *opts.priority != priority {
		t.Error("priority not set correctly")
	}

	if opts.owner == nil || *opts.owner != owner {
		t.Error("owner not set correctly")
	}

	if opts.expiresAt == nil {
		t.Error("expiresAt not set")
	}
}

// TestListShardsOptions tests the ListShardsOptions struct.
func TestListShardsOptions(t *testing.T) {
	opts := ListShardsOptions{
		Type:     "task",
		Status:   "open",
		Owner:    "agent-mycroft",
		Creator:  "agent-mycroft",
		Labels:   []string{"urgent", "bug"},
		ParentID: "pf-parent",
		Limit:    50,
	}

	if opts.Type != "task" {
		t.Errorf("expected type task, got %s", opts.Type)
	}

	if opts.Limit != 50 {
		t.Errorf("expected limit 50, got %d", opts.Limit)
	}

	if len(opts.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(opts.Labels))
	}
}

// Integration tests would require a database connection.
// These are unit tests for the structures and options only.
func TestNullIfEmptyHelper(t *testing.T) {
	result := nullIfEmpty("")
	if result != nil {
		t.Error("expected nil for empty string")
	}

	result = nullIfEmpty("test")
	if result != "test" {
		t.Errorf("expected 'test', got %v", result)
	}
}

// TestTruncateHelper tests the truncate helper function.
func TestTruncateHelper(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello"},
		{"", 5, ""},
		{"test", 4, "test"},
		{"testing", 4, "test"},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q",
				tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

// Benchmark for option application
func BenchmarkShardOptions(b *testing.B) {
	for i := 0; i < b.N; i++ {
		opts := &shardOptions{priority: 2}
		WithOwner("agent-test")(opts)
		WithPriority(1)(opts)
		WithLabels("label1", "label2")(opts)
	}
}
