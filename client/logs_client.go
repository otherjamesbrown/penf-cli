package client

import (
	"context"
	"fmt"
	"io"
	"time"

	logsv1 "github.com/otherjamesbrown/penf-cli/api/proto/logs/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// LogEntry represents a log entry for CLI display.
type LogEntry struct {
	ID        int64             `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`
	Service   string            `json:"service"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	TraceID   string            `json:"trace_id,omitempty"`
	SpanID    string            `json:"span_id,omitempty"`
	Caller    string            `json:"caller,omitempty"`
}

// LogFilter represents the filter criteria for querying logs.
type LogFilter struct {
	Service  string
	Level    string
	Since    time.Time
	Until    time.Time
	Contains string
	TraceID  string
}

// LogsResponse contains log query results.
type LogsResponse struct {
	Entries    []LogEntry `json:"entries"`
	TotalCount int64      `json:"total_count"`
	Truncated  bool       `json:"truncated"`
}

// LogStats contains log statistics.
type LogStats struct {
	TotalCount     int64            `json:"total_count"`
	CountByLevel   map[string]int64 `json:"count_by_level"`
	CountByService map[string]int64 `json:"count_by_service"`
	OldestLog      time.Time        `json:"oldest_log,omitempty"`
	NewestLog      time.Time        `json:"newest_log,omitempty"`
}

// ListLogs retrieves log entries matching the filter.
func (c *GRPCClient) ListLogs(ctx context.Context, filter LogFilter, limit, offset int, orderAsc bool) (*LogsResponse, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return nil, fmt.Errorf("not connected to gateway")
	}

	client := logsv1.NewLogsServiceClient(conn)

	// Build filter
	protoFilter := &logsv1.LogFilter{
		Service:  filter.Service,
		Contains: filter.Contains,
		TraceId:  filter.TraceID,
	}

	if filter.Level != "" {
		protoFilter.Level = levelToProto(filter.Level)
	}

	if !filter.Since.IsZero() {
		protoFilter.Since = timestamppb.New(filter.Since)
	}

	if !filter.Until.IsZero() {
		protoFilter.Until = timestamppb.New(filter.Until)
	}

	resp, err := client.ListLogs(ctx, &logsv1.ListLogsRequest{
		Filter:   protoFilter,
		Limit:    int32(limit),
		Offset:   int32(offset),
		OrderAsc: orderAsc,
	})
	if err != nil {
		return nil, fmt.Errorf("ListLogs RPC failed: %w", err)
	}

	// Convert response
	entries := make([]LogEntry, len(resp.Entries))
	for i, e := range resp.Entries {
		entries[i] = entryFromProto(e)
	}

	return &LogsResponse{
		Entries:    entries,
		TotalCount: resp.TotalCount,
		Truncated:  resp.Truncated,
	}, nil
}

// StreamLogs streams log entries in real-time.
// The callback is called for each new log entry.
// Returns when the context is cancelled or an error occurs.
func (c *GRPCClient) StreamLogs(ctx context.Context, filter LogFilter, pollIntervalMs int, callback func(LogEntry)) error {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return fmt.Errorf("not connected to gateway")
	}

	client := logsv1.NewLogsServiceClient(conn)

	// Build filter
	protoFilter := &logsv1.LogFilter{
		Service:  filter.Service,
		Contains: filter.Contains,
		TraceId:  filter.TraceID,
	}

	if filter.Level != "" {
		protoFilter.Level = levelToProto(filter.Level)
	}

	if !filter.Since.IsZero() {
		protoFilter.Since = timestamppb.New(filter.Since)
	}

	stream, err := client.StreamLogs(ctx, &logsv1.StreamLogsRequest{
		Filter:         protoFilter,
		PollIntervalMs: int32(pollIntervalMs),
	})
	if err != nil {
		return fmt.Errorf("StreamLogs RPC failed: %w", err)
	}

	for {
		entry, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("receiving log entry: %w", err)
		}

		callback(entryFromProto(entry))
	}
}

// GetLogStats retrieves log statistics.
func (c *GRPCClient) GetLogStats(ctx context.Context, tenantID string) (*LogStats, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return nil, fmt.Errorf("not connected to gateway")
	}

	client := logsv1.NewLogsServiceClient(conn)

	resp, err := client.GetStats(ctx, &logsv1.GetStatsRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetStats RPC failed: %w", err)
	}

	stats := &LogStats{
		TotalCount:     resp.Stats.TotalCount,
		CountByLevel:   resp.Stats.CountByLevel,
		CountByService: resp.Stats.CountByService,
	}

	if resp.Stats.OldestLog != nil {
		stats.OldestLog = resp.Stats.OldestLog.AsTime()
	}
	if resp.Stats.NewestLog != nil {
		stats.NewestLog = resp.Stats.NewestLog.AsTime()
	}

	return stats, nil
}

// ListLogServices returns all distinct service names that have logged.
func (c *GRPCClient) ListLogServices(ctx context.Context, tenantID string) ([]string, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return nil, fmt.Errorf("not connected to gateway")
	}

	client := logsv1.NewLogsServiceClient(conn)

	resp, err := client.ListServices(ctx, &logsv1.ListServicesRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("ListServices RPC failed: %w", err)
	}

	return resp.Services, nil
}

// levelToProto converts a string level to proto LogLevel.
func levelToProto(level string) logsv1.LogLevel {
	switch level {
	case "debug":
		return logsv1.LogLevel_LOG_LEVEL_DEBUG
	case "info":
		return logsv1.LogLevel_LOG_LEVEL_INFO
	case "warn":
		return logsv1.LogLevel_LOG_LEVEL_WARN
	case "error":
		return logsv1.LogLevel_LOG_LEVEL_ERROR
	default:
		return logsv1.LogLevel_LOG_LEVEL_UNSPECIFIED
	}
}

// levelFromProto converts a proto LogLevel to string.
func levelFromProto(level logsv1.LogLevel) string {
	switch level {
	case logsv1.LogLevel_LOG_LEVEL_DEBUG:
		return "debug"
	case logsv1.LogLevel_LOG_LEVEL_INFO:
		return "info"
	case logsv1.LogLevel_LOG_LEVEL_WARN:
		return "warn"
	case logsv1.LogLevel_LOG_LEVEL_ERROR:
		return "error"
	default:
		return ""
	}
}

// entryFromProto converts a proto LogEntry to LogEntry.
func entryFromProto(e *logsv1.LogEntry) LogEntry {
	entry := LogEntry{
		ID:      e.Id,
		Level:   levelFromProto(e.Level),
		Service: e.Service,
		Message: e.Message,
		Fields:  e.Fields,
		TraceID: e.TraceId,
		SpanID:  e.SpanId,
		Caller:  e.Caller,
	}

	if e.Timestamp != nil {
		entry.Timestamp = e.Timestamp.AsTime()
	}

	return entry
}
