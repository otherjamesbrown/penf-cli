// Package logging provides structured logging for Penfold Go microservices.
// It wraps zerolog to provide a consistent logging interface with support for
// JSON output (production) and human-readable output (development).
package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// ContextKey type for context values to avoid collisions.
type ContextKey string

// Context keys for trace information.
const (
	TraceIDKey   ContextKey = "trace_id"
	RequestIDKey ContextKey = "request_id"
)

// Level represents logging severity levels.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Config holds logger configuration.
type Config struct {
	// Level sets the minimum log level (debug, info, warn, error).
	Level Level

	// ServiceName is included in all log entries.
	ServiceName string

	// Environment is included in all log entries (e.g., "development", "production").
	Environment string

	// JSONFormat enables JSON output when true, human-readable when false.
	JSONFormat bool

	// Output sets the writer for logs (defaults to os.Stdout).
	Output io.Writer

	// Sinks are optional log sinks for async persistence.
	Sinks []Sink
}

// DefaultConfig returns a Config with sensible defaults for development.
func DefaultConfig() *Config {
	return &Config{
		Level:       LevelInfo,
		ServiceName: "unknown",
		Environment: "development",
		JSONFormat:  false,
		Output:      os.Stdout,
	}
}

// Logger is the interface for structured logging.
type Logger interface {
	// Debug logs a debug message with optional fields.
	Debug(msg string, fields ...Field)

	// Info logs an info message with optional fields.
	Info(msg string, fields ...Field)

	// Warn logs a warning message with optional fields.
	Warn(msg string, fields ...Field)

	// Error logs an error message with optional fields.
	Error(msg string, fields ...Field)

	// With returns a new Logger with the given fields attached to all subsequent logs.
	With(fields ...Field) Logger

	// WithContext returns a new Logger that extracts trace information from the context.
	WithContext(ctx context.Context) Logger

	// Zerolog returns the underlying zerolog.Logger for use with legacy components.
	Zerolog() zerolog.Logger
}

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value interface{}
}

// F creates a new Field with the given key and value.
func F(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Err creates a Field for an error.
func Err(err error) Field {
	return Field{Key: "error", Value: err}
}

// logger implements the Logger interface using zerolog.
type logger struct {
	zl          zerolog.Logger
	serviceName string
	environment string
	sinks       []Sink
	tenantID    string // Optional tenant context
}

// NewLogger creates a new Logger with the given configuration.
func NewLogger(cfg *Config) Logger {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}

	// Set global zerolog level
	level := parseLevel(cfg.Level)
	zerolog.SetGlobalLevel(level)

	var zl zerolog.Logger

	if cfg.JSONFormat {
		// JSON format for production
		zl = zerolog.New(output).
			With().
			Timestamp().
			Str("service_name", cfg.ServiceName).
			Str("environment", cfg.Environment).
			Logger()
	} else {
		// Human-readable format for development
		consoleWriter := zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
		zl = zerolog.New(consoleWriter).
			With().
			Timestamp().
			Str("service_name", cfg.ServiceName).
			Str("environment", cfg.Environment).
			Logger()
	}

	return &logger{
		zl:          zl,
		serviceName: cfg.ServiceName,
		environment: cfg.Environment,
		sinks:       cfg.Sinks,
	}
}

// Zerolog returns the underlying zerolog.Logger for use with legacy components.
// This allows interoperability with code that directly uses zerolog.
func (l *logger) Zerolog() zerolog.Logger {
	return l.zl
}

// parseLevel converts Level to zerolog.Level.
func parseLevel(l Level) zerolog.Level {
	switch l {
	case LevelDebug:
		return zerolog.DebugLevel
	case LevelInfo:
		return zerolog.InfoLevel
	case LevelWarn:
		return zerolog.WarnLevel
	case LevelError:
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// Debug logs a debug message.
func (l *logger) Debug(msg string, fields ...Field) {
	event := l.zl.Debug()
	event = addFields(event, fields)
	event.Msg(msg)

	// Send to sinks
	l.sendToSinks("debug", msg, fields)
}

// Info logs an info message.
func (l *logger) Info(msg string, fields ...Field) {
	event := l.zl.Info()
	event = addFields(event, fields)
	event.Msg(msg)

	// Send to sinks
	l.sendToSinks("info", msg, fields)
}

// Warn logs a warning message.
func (l *logger) Warn(msg string, fields ...Field) {
	event := l.zl.Warn()
	event = addFields(event, fields)
	event.Msg(msg)

	// Send to sinks
	l.sendToSinks("warn", msg, fields)
}

// Error logs an error message.
func (l *logger) Error(msg string, fields ...Field) {
	event := l.zl.Error()
	event = addFields(event, fields)
	event.Msg(msg)

	// Send to sinks
	l.sendToSinks("error", msg, fields)
}

// With returns a new logger with additional fields.
func (l *logger) With(fields ...Field) Logger {
	ctx := l.zl.With()
	for _, f := range fields {
		ctx = addFieldToContext(ctx, f)
	}
	return &logger{
		zl:          ctx.Logger(),
		serviceName: l.serviceName,
		environment: l.environment,
		sinks:       l.sinks,
		tenantID:    l.tenantID,
	}
}

// WithContext returns a new logger that includes trace information from context.
func (l *logger) WithContext(ctx context.Context) Logger {
	newLogger := l.zl.With()

	// Extract trace_id from context if present
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		newLogger = newLogger.Str("trace_id", traceID)
	}

	// Extract request_id from context if present
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		newLogger = newLogger.Str("request_id", requestID)
	}

	return &logger{
		zl:          newLogger.Logger(),
		serviceName: l.serviceName,
		environment: l.environment,
		sinks:       l.sinks,
		tenantID:    l.tenantID,
	}
}

// addFields adds multiple fields to a zerolog event.
func addFields(event *zerolog.Event, fields []Field) *zerolog.Event {
	for _, f := range fields {
		switch v := f.Value.(type) {
		case string:
			event = event.Str(f.Key, v)
		case int:
			event = event.Int(f.Key, v)
		case int64:
			event = event.Int64(f.Key, v)
		case float64:
			event = event.Float64(f.Key, v)
		case bool:
			event = event.Bool(f.Key, v)
		case error:
			event = event.Err(v)
		case time.Duration:
			event = event.Dur(f.Key, v)
		case time.Time:
			event = event.Time(f.Key, v)
		default:
			event = event.Interface(f.Key, v)
		}
	}
	return event
}

// addFieldToContext adds a field to a zerolog context.
func addFieldToContext(ctx zerolog.Context, f Field) zerolog.Context {
	switch v := f.Value.(type) {
	case string:
		return ctx.Str(f.Key, v)
	case int:
		return ctx.Int(f.Key, v)
	case int64:
		return ctx.Int64(f.Key, v)
	case float64:
		return ctx.Float64(f.Key, v)
	case bool:
		return ctx.Bool(f.Key, v)
	case error:
		return ctx.Err(v)
	case time.Duration:
		return ctx.Dur(f.Key, v)
	case time.Time:
		return ctx.Time(f.Key, v)
	default:
		return ctx.Interface(f.Key, v)
	}
}

// sendToSinks sends a log entry to all configured sinks.
func (l *logger) sendToSinks(level, msg string, fields []Field) {
	if len(l.sinks) == 0 {
		return
	}

	// Convert fields to map[string]string
	fieldMap := make(map[string]string)
	var traceID string
	for _, f := range fields {
		if f.Key == "trace_id" {
			if tid, ok := f.Value.(string); ok {
				traceID = tid
			}
		}
		// Convert all field values to strings for storage
		fieldMap[f.Key] = fmt.Sprint(f.Value)
	}

	// Extract trace_id from zerolog context if not in fields
	if traceID == "" {
		// Try to extract from zerolog context - this is best-effort
		// The zerolog.Logger doesn't expose context values, so we rely on fields
	}

	entry := LogEntry{
		TenantID:  l.tenantID,
		Timestamp: time.Now(),
		Level:     level,
		Service:   l.serviceName,
		Message:   msg,
		Fields:    fieldMap,
		TraceID:   traceID,
		Caller:    getCaller(3), // Skip sendToSinks, Debug/Info/Warn/Error, and the actual caller
	}

	for _, sink := range l.sinks {
		sink.Write(entry)
	}
}

// Global provides a package-level logger for convenience.
// Initialize with SetGlobal() before use.
var global Logger

// SetGlobal sets the global logger instance.
func SetGlobal(l Logger) {
	global = l
}

// Global returns the global logger instance.
// Panics if SetGlobal has not been called.
func Global() Logger {
	if global == nil {
		panic("logging: global logger not initialized, call SetGlobal first")
	}
	return global
}

// MustGlobal returns the global logger, initializing with defaults if not set.
func MustGlobal() Logger {
	if global == nil {
		global = NewLogger(DefaultConfig())
	}
	return global
}

// nopLogger is a logger that discards all output.
type nopLogger struct{}

func (n *nopLogger) Debug(msg string, fields ...Field)       {}
func (n *nopLogger) Info(msg string, fields ...Field)        {}
func (n *nopLogger) Warn(msg string, fields ...Field)        {}
func (n *nopLogger) Error(msg string, fields ...Field)       {}
func (n *nopLogger) With(fields ...Field) Logger             { return n }
func (n *nopLogger) WithContext(ctx context.Context) Logger  { return n }
func (n *nopLogger) Zerolog() zerolog.Logger                 { return zerolog.Nop() }

// NewNopLogger returns a logger that discards all output.
// Useful for testing when you don't want log noise.
func NewNopLogger() Logger {
	return &nopLogger{}
}
