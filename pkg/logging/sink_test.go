package logging

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockLogWriter is a test implementation of LogWriter.
type mockLogWriter struct {
	mu      sync.Mutex
	batches [][]LogEntry
	err     error
}

func (m *mockLogWriter) WriteBatch(ctx context.Context, entries []LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	// Make a copy of entries to avoid race conditions
	batch := make([]LogEntry, len(entries))
	copy(batch, entries)
	m.batches = append(m.batches, batch)

	return nil
}

func (m *mockLogWriter) GetBatches() [][]LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.batches
}

func (m *mockLogWriter) TotalEntries() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for _, batch := range m.batches {
		total += len(batch)
	}
	return total
}

func TestDBSink_Batching(t *testing.T) {
	writer := &mockLogWriter{}
	sink := NewDBSink(DBSinkConfig{
		Writer:        writer,
		BufferSize:    100,
		BatchSize:     10,
		FlushInterval: 100 * time.Millisecond,
	})
	defer sink.Close()

	// Write 25 entries (should create 2 full batches of 10, leaving 5 buffered)
	for i := 0; i < 25; i++ {
		sink.Write(LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Service:   "test",
			Message:   "test message",
		})
	}

	// Give time for batches to be written
	time.Sleep(50 * time.Millisecond)

	// Should have 2 batches of 10
	batches := writer.GetBatches()
	if len(batches) != 2 {
		t.Errorf("Expected 2 batches, got %d", len(batches))
	}

	for i, batch := range batches {
		if len(batch) != 10 {
			t.Errorf("Batch %d: expected 10 entries, got %d", i, len(batch))
		}
	}

	// Flush should write the remaining 5
	ctx := context.Background()
	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Now should have 3 batches total
	batches = writer.GetBatches()
	if len(batches) != 3 {
		t.Errorf("After flush: expected 3 batches, got %d", len(batches))
	}

	// Verify total entries
	total := writer.TotalEntries()
	if total != 25 {
		t.Errorf("Expected 25 total entries, got %d", total)
	}
}

func TestDBSink_PeriodicFlush(t *testing.T) {
	writer := &mockLogWriter{}
	sink := NewDBSink(DBSinkConfig{
		Writer:        writer,
		BufferSize:    100,
		BatchSize:     100, // Large batch size so we don't trigger batch flush
		FlushInterval: 200 * time.Millisecond,
	})
	defer sink.Close()

	// Write a few entries
	for i := 0; i < 5; i++ {
		sink.Write(LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Service:   "test",
			Message:   "test message",
		})
	}

	// Wait for periodic flush
	time.Sleep(300 * time.Millisecond)

	// Should have been flushed
	batches := writer.GetBatches()
	if len(batches) != 1 {
		t.Errorf("Expected 1 batch from periodic flush, got %d", len(batches))
	}

	if len(batches) > 0 && len(batches[0]) != 5 {
		t.Errorf("Expected 5 entries in batch, got %d", len(batches[0]))
	}
}

func TestDBSink_FullBuffer(t *testing.T) {
	writer := &mockLogWriter{}
	sink := NewDBSink(DBSinkConfig{
		Writer:        writer,
		BufferSize:    10,  // Small buffer
		BatchSize:     100, // Large batch size
		FlushInterval: 10 * time.Second,
	})
	defer sink.Close()

	// Write more than buffer can hold
	for i := 0; i < 20; i++ {
		sink.Write(LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Service:   "test",
			Message:   "test message",
		})
	}

	// Flush and verify some entries were dropped
	ctx := context.Background()
	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	total := writer.TotalEntries()
	if total >= 20 {
		t.Errorf("Expected some entries to be dropped (buffer=10), but got %d entries written", total)
	}

	// Due to async processing, we might get slightly more than buffer size
	// (entries being processed while we're writing), but should be close to buffer size
	if total > 15 {
		t.Errorf("Expected around buffer size (10) entries, got %d", total)
	}
}

func TestDBSink_Close(t *testing.T) {
	writer := &mockLogWriter{}
	sink := NewDBSink(DBSinkConfig{
		Writer:        writer,
		BufferSize:    100,
		BatchSize:     10,
		FlushInterval: 1 * time.Second,
	})

	// Write some entries
	for i := 0; i < 5; i++ {
		sink.Write(LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Service:   "test",
			Message:   "test message",
		})
	}

	// Close should flush remaining entries
	if err := sink.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify entries were written
	total := writer.TotalEntries()
	if total != 5 {
		t.Errorf("Expected 5 entries after close, got %d", total)
	}

	// Writing after close should be safe (no-op)
	sink.Write(LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Service:   "test",
		Message:   "should be ignored",
	})

	// Total should still be 5
	total = writer.TotalEntries()
	if total != 5 {
		t.Errorf("Expected 5 entries (write after close ignored), got %d", total)
	}
}

func TestDBSink_ConcurrentWrites(t *testing.T) {
	writer := &mockLogWriter{}
	sink := NewDBSink(DBSinkConfig{
		Writer:        writer,
		BufferSize:    500, // Large enough to handle concurrent writes
		BatchSize:     50,
		FlushInterval: 100 * time.Millisecond,
	})
	defer sink.Close()

	// Concurrent writes from multiple goroutines
	const goroutines = 10
	const entriesPerGoroutine = 20
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < entriesPerGoroutine; j++ {
				sink.Write(LogEntry{
					Timestamp: time.Now(),
					Level:     "info",
					Service:   "test",
					Message:   "concurrent write",
				})
				// Small delay to avoid overwhelming the channel
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()

	// Give background goroutine time to process
	time.Sleep(50 * time.Millisecond)

	// Flush to ensure all writes are processed
	ctx := context.Background()
	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Verify all (or most) entries were written
	total := writer.TotalEntries()
	expected := goroutines * entriesPerGoroutine
	if total < expected-5 { // Allow small margin for async processing
		t.Errorf("Expected around %d entries from concurrent writes, got %d (too many dropped)", expected, total)
	}
}

func TestDBSink_FieldsPreserved(t *testing.T) {
	writer := &mockLogWriter{}
	sink := NewDBSink(DBSinkConfig{
		Writer:        writer,
		BufferSize:    10,
		BatchSize:     10,
		FlushInterval: 1 * time.Second,
	})
	defer sink.Close()

	// Write entry with fields
	testFields := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	sink.Write(LogEntry{
		TenantID:  "test-tenant",
		Timestamp: time.Now(),
		Level:     "warn",
		Service:   "test-service",
		Message:   "test message",
		Fields:    testFields,
		TraceID:   "trace-123",
		Caller:    "test.go:42",
	})

	// Give time for entry to be queued
	time.Sleep(10 * time.Millisecond)

	ctx := context.Background()
	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	batches := writer.GetBatches()
	if len(batches) == 0 {
		t.Fatalf("Expected at least 1 batch, got 0")
	}

	totalEntries := 0
	for _, batch := range batches {
		totalEntries += len(batch)
	}
	if totalEntries != 1 {
		t.Fatalf("Expected 1 entry total, got %d", totalEntries)
	}

	// Get the first entry from any batch
	var entry LogEntry
	for _, batch := range batches {
		if len(batch) > 0 {
			entry = batch[0]
			break
		}
	}
	if entry.TenantID != "test-tenant" {
		t.Errorf("TenantID: expected 'test-tenant', got '%s'", entry.TenantID)
	}
	if entry.Level != "warn" {
		t.Errorf("Level: expected 'warn', got '%s'", entry.Level)
	}
	if entry.Service != "test-service" {
		t.Errorf("Service: expected 'test-service', got '%s'", entry.Service)
	}
	if entry.Message != "test message" {
		t.Errorf("Message: expected 'test message', got '%s'", entry.Message)
	}
	if entry.TraceID != "trace-123" {
		t.Errorf("TraceID: expected 'trace-123', got '%s'", entry.TraceID)
	}
	if entry.Caller != "test.go:42" {
		t.Errorf("Caller: expected 'test.go:42', got '%s'", entry.Caller)
	}
	if entry.Fields["key1"] != "value1" {
		t.Errorf("Fields[key1]: expected 'value1', got '%s'", entry.Fields["key1"])
	}
	if entry.Fields["key2"] != "value2" {
		t.Errorf("Fields[key2]: expected 'value2', got '%s'", entry.Fields["key2"])
	}
}
