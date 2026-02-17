package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewLogger_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != LevelInfo {
		t.Errorf("expected default level to be info, got %s", cfg.Level)
	}
	if cfg.ServiceName != "unknown" {
		t.Errorf("expected default service name to be 'unknown', got %s", cfg.ServiceName)
	}
	if cfg.Environment != "development" {
		t.Errorf("expected default environment to be 'development', got %s", cfg.Environment)
	}
	if cfg.JSONFormat {
		t.Error("expected default JSONFormat to be false")
	}
}

func TestNewLogger_NilConfig(t *testing.T) {
	log := NewLogger(nil)
	if log == nil {
		t.Error("expected non-nil logger with nil config")
	}
}

func TestLogger_JSONFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := &Config{
		Level:       LevelDebug,
		ServiceName: "test-service",
		Environment: "testing",
		JSONFormat:  true,
		Output:      buf,
	}

	log := NewLogger(cfg)
	log.Info("test message", F("key", "value"))

	// Parse the JSON output
	var output map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Verify required fields
	if output["message"] != "test message" {
		t.Errorf("expected message 'test message', got %v", output["message"])
	}
	if output["service_name"] != "test-service" {
		t.Errorf("expected service_name 'test-service', got %v", output["service_name"])
	}
	if output["environment"] != "testing" {
		t.Errorf("expected environment 'testing', got %v", output["environment"])
	}
	if output["key"] != "value" {
		t.Errorf("expected key 'value', got %v", output["key"])
	}
	if _, ok := output["time"]; !ok {
		t.Error("expected timestamp field 'time' in output")
	}
	if output["level"] != "info" {
		t.Errorf("expected level 'info', got %v", output["level"])
	}
}

func TestLogger_AllLevels(t *testing.T) {
	tests := []struct {
		name     string
		logFunc  func(Logger)
		expected string
	}{
		{
			name:     "debug",
			logFunc:  func(l Logger) { l.Debug("debug message") },
			expected: "debug",
		},
		{
			name:     "info",
			logFunc:  func(l Logger) { l.Info("info message") },
			expected: "info",
		},
		{
			name:     "warn",
			logFunc:  func(l Logger) { l.Warn("warn message") },
			expected: "warn",
		},
		{
			name:     "error",
			logFunc:  func(l Logger) { l.Error("error message") },
			expected: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			log := NewLogger(&Config{
				Level:       LevelDebug,
				ServiceName: "test",
				Environment: "test",
				JSONFormat:  true,
				Output:      buf,
			})

			tt.logFunc(log)

			var output map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			if output["level"] != tt.expected {
				t.Errorf("expected level %s, got %v", tt.expected, output["level"])
			}
		})
	}
}

func TestLogger_WithFields(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewLogger(&Config{
		Level:       LevelInfo,
		ServiceName: "test",
		Environment: "test",
		JSONFormat:  true,
		Output:      buf,
	})

	// Create logger with persistent fields
	log = log.With(F("component", "api"), F("version", "1.0"))
	log.Info("request handled")

	var output map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if output["component"] != "api" {
		t.Errorf("expected component 'api', got %v", output["component"])
	}
	if output["version"] != "1.0" {
		t.Errorf("expected version '1.0', got %v", output["version"])
	}
}

func TestLogger_WithContext(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewLogger(&Config{
		Level:       LevelInfo,
		ServiceName: "test",
		Environment: "test",
		JSONFormat:  true,
		Output:      buf,
	})

	// Create context with trace information
	ctx := context.Background()
	ctx = context.WithValue(ctx, TraceIDKey, "trace-123")
	ctx = context.WithValue(ctx, RequestIDKey, "req-456")

	log.WithContext(ctx).Info("traced request")

	var output map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if output["trace_id"] != "trace-123" {
		t.Errorf("expected trace_id 'trace-123', got %v", output["trace_id"])
	}
	if output["request_id"] != "req-456" {
		t.Errorf("expected request_id 'req-456', got %v", output["request_id"])
	}
}

func TestLogger_WithContext_EmptyContext(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewLogger(&Config{
		Level:       LevelInfo,
		ServiceName: "test",
		Environment: "test",
		JSONFormat:  true,
		Output:      buf,
	})

	// Use empty context
	log.WithContext(context.Background()).Info("no trace")

	var output map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// trace_id and request_id should not be present
	if _, ok := output["trace_id"]; ok {
		t.Error("trace_id should not be present with empty context")
	}
	if _, ok := output["request_id"]; ok {
		t.Error("request_id should not be present with empty context")
	}
}

func TestLogger_FieldTypes(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewLogger(&Config{
		Level:       LevelInfo,
		ServiceName: "test",
		Environment: "test",
		JSONFormat:  true,
		Output:      buf,
	})

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	testDuration := 5 * time.Second
	testErr := errors.New("test error")

	log.Info("type test",
		F("string_field", "hello"),
		F("int_field", 42),
		F("int64_field", int64(9999999999)),
		F("float_field", 3.14),
		F("bool_field", true),
		F("duration_field", testDuration),
		F("time_field", testTime),
		Err(testErr),
	)

	var output map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if output["string_field"] != "hello" {
		t.Errorf("string_field mismatch: %v", output["string_field"])
	}
	if output["int_field"] != float64(42) { // JSON numbers are float64
		t.Errorf("int_field mismatch: %v", output["int_field"])
	}
	if output["int64_field"] != float64(9999999999) {
		t.Errorf("int64_field mismatch: %v", output["int64_field"])
	}
	if output["float_field"] != 3.14 {
		t.Errorf("float_field mismatch: %v", output["float_field"])
	}
	if output["bool_field"] != true {
		t.Errorf("bool_field mismatch: %v", output["bool_field"])
	}
	if output["error"] != "test error" {
		t.Errorf("error field mismatch: %v", output["error"])
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewLogger(&Config{
		Level:       LevelWarn, // Only warn and above
		ServiceName: "test",
		Environment: "test",
		JSONFormat:  true,
		Output:      buf,
	})

	log.Debug("debug - should not appear")
	log.Info("info - should not appear")
	log.Warn("warn - should appear")
	log.Error("error - should appear")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have exactly 2 lines (warn and error)
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d: %s", len(lines), output)
	}

	if !strings.Contains(lines[0], "warn - should appear") {
		t.Errorf("expected first line to be warn, got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "error - should appear") {
		t.Errorf("expected second line to be error, got: %s", lines[1])
	}
}

func TestLogger_ConsoleFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewLogger(&Config{
		Level:       LevelInfo,
		ServiceName: "my-service",
		Environment: "dev",
		JSONFormat:  false, // Console format
		Output:      buf,
	})

	log.Info("console output test", F("user", "alice"))

	output := buf.String()

	// Console format should contain the message (not strict JSON)
	if !strings.Contains(output, "console output test") {
		t.Errorf("console output should contain message: %s", output)
	}
	if !strings.Contains(output, "INF") {
		t.Errorf("console output should contain level indicator: %s", output)
	}
}

func TestGlobal_NotInitialized(t *testing.T) {
	// Reset global for this test
	oldGlobal := global
	global = nil
	defer func() { global = oldGlobal }()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when global not initialized")
		}
	}()

	Global()
}

func TestSetGlobal_And_Global(t *testing.T) {
	// Reset global for this test
	oldGlobal := global
	defer func() { global = oldGlobal }()

	buf := &bytes.Buffer{}
	log := NewLogger(&Config{
		Level:       LevelInfo,
		ServiceName: "global-test",
		JSONFormat:  true,
		Output:      buf,
	})

	SetGlobal(log)
	Global().Info("global logger test")

	if !strings.Contains(buf.String(), "global logger test") {
		t.Errorf("global logger should have logged: %s", buf.String())
	}
}

func TestMustGlobal_InitializesDefaults(t *testing.T) {
	// Reset global for this test
	oldGlobal := global
	global = nil
	defer func() { global = oldGlobal }()

	log := MustGlobal()
	if log == nil {
		t.Error("MustGlobal should return non-nil logger")
	}
}

func TestF_And_Err_Helpers(t *testing.T) {
	f := F("key", "value")
	if f.Key != "key" || f.Value != "value" {
		t.Errorf("F helper failed: %+v", f)
	}

	testErr := errors.New("test")
	errField := Err(testErr)
	if errField.Key != "error" {
		t.Errorf("Err helper should use 'error' as key, got: %s", errField.Key)
	}
	if errField.Value != testErr {
		t.Errorf("Err helper value mismatch")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    Level
		expected string
	}{
		{LevelDebug, "debug"},
		{LevelInfo, "info"},
		{LevelWarn, "warn"},
		{LevelError, "error"},
		{Level("invalid"), "info"}, // defaults to info
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			zlevel := parseLevel(tt.input)
			if zlevel.String() != tt.expected {
				t.Errorf("parseLevel(%s) = %s, want %s", tt.input, zlevel.String(), tt.expected)
			}
		})
	}
}
