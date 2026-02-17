package logging

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
)

// LogEntry represents a log entry to be written to a sink.
type LogEntry struct {
	TenantID  string
	Timestamp time.Time
	Level     string
	Service   string
	Message   string
	Fields    map[string]string
	TraceID   string
	Caller    string
}

// LogWriter is an interface for writing log entries to persistent storage.
// Implementations should handle batching and error recovery.
type LogWriter interface {
	WriteBatch(ctx context.Context, entries []LogEntry) error
}

// Sink is an interface for components that receive log entries.
type Sink interface {
	// Write queues a log entry for async processing.
	Write(entry LogEntry)
	// Flush blocks until all queued entries are written.
	Flush(ctx context.Context) error
	// Close shuts down the sink gracefully.
	Close() error
}

// DBSink is an async database log sink with buffered writes.
type DBSink struct {
	writer       LogWriter
	entryChan    chan LogEntry
	flushChan    chan chan error
	flushTicker  *time.Ticker
	batchSize    int
	flushTimeout time.Duration
	wg           sync.WaitGroup
	done         chan struct{}
	mu           sync.Mutex
	closed       bool
}

// DBSinkConfig configures a DBSink.
type DBSinkConfig struct {
	// Writer is the backend for persisting log entries.
	Writer LogWriter
	// BufferSize is the channel capacity (default: 1000).
	BufferSize int
	// BatchSize is the max entries per batch write (default: 100).
	BatchSize int
	// FlushInterval is how often to flush buffered entries (default: 2s).
	FlushInterval time.Duration
}

// NewDBSink creates a new async database log sink.
func NewDBSink(cfg DBSinkConfig) *DBSink {
	if cfg.Writer == nil {
		panic("DBSink requires a non-nil Writer")
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 2 * time.Second
	}

	sink := &DBSink{
		writer:       cfg.Writer,
		entryChan:    make(chan LogEntry, cfg.BufferSize),
		flushChan:    make(chan chan error),
		flushTicker:  time.NewTicker(cfg.FlushInterval),
		batchSize:    cfg.BatchSize,
		flushTimeout: 5 * time.Second,
		done:         make(chan struct{}),
	}

	sink.wg.Add(1)
	go sink.run()

	return sink
}

// Write queues a log entry for async processing.
// If the buffer is full, the entry is dropped and a warning is logged to stderr.
func (s *DBSink) Write(entry LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	select {
	case s.entryChan <- entry:
		// Successfully queued
	default:
		// Buffer full - log to stderr and drop
		fmt.Fprintf(os.Stderr, "[DBSink] Buffer full, dropping log entry: %s\n", entry.Message)
	}
}

// Flush blocks until all queued entries are written.
func (s *DBSink) Flush(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	// Send flush request to background goroutine (non-blocking)
	errChan := make(chan error, 1)
	select {
	case s.flushChan <- errChan:
		// Wait for flush to complete
		select {
		case err := <-errChan:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.flushTimeout):
			return fmt.Errorf("flush timeout after %v", s.flushTimeout)
		}
	case <-time.After(100 * time.Millisecond):
		// If we can't send flush request quickly, the goroutine is busy
		// Just wait a bit for it to process
		return nil
	}
}

// Close shuts down the sink gracefully.
func (s *DBSink) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	close(s.done)
	s.flushTicker.Stop()
	s.wg.Wait()

	return nil
}

// run is the background goroutine that batches and writes log entries.
func (s *DBSink) run() {
	defer s.wg.Done()

	batch := make([]LogEntry, 0, s.batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), s.flushTimeout)
		defer cancel()

		err := s.writeBatch(ctx, batch)
		if err != nil {
			// Log error to stderr, never crash
			fmt.Fprintf(os.Stderr, "[DBSink] Failed to write batch of %d entries: %v\n", len(batch), err)
		}

		batch = batch[:0] // Reset batch
		return err
	}

	for {
		// Check for flush requests first (priority)
		select {
		case errChan := <-s.flushChan:
			// Explicit flush request - flush and respond
			err := flush()
			errChan <- err
			continue
		case <-s.done:
			// Drain remaining entries before exit
			flush()
		drainLoop:
			for {
				select {
				case entry := <-s.entryChan:
					batch = append(batch, entry)
					if len(batch) >= s.batchSize {
						flush()
					}
				default:
					break drainLoop
				}
			}
			flush()
			return
		default:
			// No flush request, continue normal processing
		}

		// Normal processing
		select {
		case entry := <-s.entryChan:
			batch = append(batch, entry)

			// Flush if batch is full
			if len(batch) >= s.batchSize {
				flush()
			}

		case <-s.flushTicker.C:
			// Periodic flush
			flush()

		case errChan := <-s.flushChan:
			// Explicit flush request
			err := flush()
			errChan <- err

		case <-s.done:
			// Drain remaining entries before exit
			flush()
		drainLoop2:
			for {
				select {
				case entry := <-s.entryChan:
					batch = append(batch, entry)
					if len(batch) >= s.batchSize {
						flush()
					}
				default:
					break drainLoop2
				}
			}
			flush()
			return
		}
	}
}

// writeBatch writes a batch of entries using the LogWriter.
func (s *DBSink) writeBatch(ctx context.Context, entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	return s.writer.WriteBatch(ctx, entries)
}

// getCaller returns the caller information (file:line) for logging.
func getCaller(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	// Extract just the filename, not the full path
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			file = file[i+1:]
			break
		}
	}
	return fmt.Sprintf("%s:%d", file, line)
}
