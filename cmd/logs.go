// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// LogLevel represents log severity levels.
type LogLevel string

const (
	// LogLevelDebug is for debug messages.
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo is for informational messages.
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn is for warning messages.
	LogLevelWarn LogLevel = "warn"
	// LogLevelError is for error messages.
	LogLevelError LogLevel = "error"
)

// LogEntry represents a single log entry.
type LogEntry struct {
	Timestamp time.Time         `json:"timestamp" yaml:"timestamp"`
	Level     LogLevel          `json:"level" yaml:"level"`
	Service   string            `json:"service" yaml:"service"`
	Message   string            `json:"message" yaml:"message"`
	Fields    map[string]string `json:"fields,omitempty" yaml:"fields,omitempty"`
	TraceID   string            `json:"trace_id,omitempty" yaml:"trace_id,omitempty"`
}

// LogsResponse contains log query results.
type LogsResponse struct {
	Entries    []LogEntry `json:"entries" yaml:"entries"`
	TotalCount int        `json:"total_count" yaml:"total_count"`
	Truncated  bool       `json:"truncated" yaml:"truncated"`
	Query      LogQuery   `json:"query" yaml:"query"`
	FetchedAt  time.Time  `json:"fetched_at" yaml:"fetched_at"`
}

// LogQuery represents the query parameters for logs.
type LogQuery struct {
	Service  string    `json:"service,omitempty" yaml:"service,omitempty"`
	Level    string    `json:"level,omitempty" yaml:"level,omitempty"`
	Since    time.Time `json:"since,omitempty" yaml:"since,omitempty"`
	Until    time.Time `json:"until,omitempty" yaml:"until,omitempty"`
	Contains string    `json:"contains,omitempty" yaml:"contains,omitempty"`
	Limit    int       `json:"limit" yaml:"limit"`
}

// LogsCommandDeps holds the dependencies for logs commands.
type LogsCommandDeps struct {
	Config       *config.CLIConfig
	GRPCClient   *client.GRPCClient
	OutputFormat config.OutputFormat
	LoadConfig   func() (*config.CLIConfig, error)
	InitClient   func(*config.CLIConfig) (*client.GRPCClient, error)
}

// DefaultLogsDeps returns the default dependencies for production use.
func DefaultLogsDeps() *LogsCommandDeps {
	return &LogsCommandDeps{
		LoadConfig: config.LoadConfig,
		InitClient: client.ConnectFromConfig,
	}
}

// Logs command flags.
var (
	logsService  string
	logsLevel    string
	logsSince    string
	logsUntil    string
	logsContains string
	logsLimit    int
	logsFollow   bool
	logsOutput   string
	logsNoColor  bool
)

// NewLogsCommand creates the logs command.
func NewLogsCommand(deps *LogsCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultLogsDeps()
	}

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View and search service logs",
		Long: `View and search logs from Penfold services.

Retrieves logs from the centralized logging system, allowing you to filter
by service, log level, time range, and content.

Examples:
  # View recent logs from all services
  penf logs

  # View logs from a specific service
  penf logs --service=gateway

  # View only error logs
  penf logs --level=error

  # Search logs containing a specific term
  penf logs --contains="connection refused"

  # View logs from the last hour
  penf logs --since=1h

  # Follow logs in real-time
  penf logs --follow

  # Combine filters
  penf logs --service=orchestrator --level=warn --since=30m`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(cmd.Context(), deps)
		},
	}

	// Define flags.
	cmd.Flags().StringVarP(&logsService, "service", "s", "", "Filter by service (gateway, worker, ai_service, etc.)")
	cmd.Flags().StringVarP(&logsLevel, "level", "l", "", "Minimum log level (debug, info, warn, error)")
	cmd.Flags().StringVar(&logsSince, "since", "15m", "Show logs since this time ago (e.g., 5m, 1h, 24h)")
	cmd.Flags().StringVar(&logsUntil, "until", "", "Show logs until this time ago")
	cmd.Flags().StringVarP(&logsContains, "contains", "c", "", "Filter logs containing this string")
	cmd.Flags().IntVarP(&logsLimit, "limit", "n", 100, "Maximum number of log entries")
	cmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow logs in real-time")
	cmd.Flags().StringVarP(&logsOutput, "output", "o", "", "Output format: text, json, yaml")
	cmd.Flags().BoolVar(&logsNoColor, "no-color", false, "Disable colored output")

	return cmd
}

// runLogs executes the logs command.
func runLogs(ctx context.Context, deps *LogsCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Parse time ranges.
	var since, until time.Time
	if logsSince != "" {
		duration, err := time.ParseDuration(logsSince)
		if err != nil {
			return fmt.Errorf("invalid --since duration: %w", err)
		}
		since = time.Now().Add(-duration)
	}
	if logsUntil != "" {
		duration, err := time.ParseDuration(logsUntil)
		if err != nil {
			return fmt.Errorf("invalid --until duration: %w", err)
		}
		until = time.Now().Add(-duration)
	}

	// Validate log level if provided.
	if logsLevel != "" {
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !validLevels[logsLevel] {
			return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", logsLevel)
		}
	}

	// Determine output format.
	outputFormat := cfg.OutputFormat
	if logsOutput != "" {
		outputFormat = config.OutputFormat(logsOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", logsOutput)
		}
	}

	query := LogQuery{
		Service:  logsService,
		Level:    logsLevel,
		Since:    since,
		Until:    until,
		Contains: logsContains,
		Limit:    logsLimit,
	}

	// Initialize gRPC client.
	grpcClient, err := deps.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("initializing client: %w", err)
	}
	defer grpcClient.Close()
	deps.GRPCClient = grpcClient

	if logsFollow {
		return runLogsFollow(ctx, deps, query, outputFormat)
	}

	// Build filter for gRPC call.
	filter := client.LogFilter{
		Service:  query.Service,
		Level:    query.Level,
		Since:    query.Since,
		Until:    query.Until,
		Contains: query.Contains,
	}

	// Call the logs service.
	resp, err := grpcClient.ListLogs(ctx, filter, query.Limit, 0, false)
	if err != nil {
		return fmt.Errorf("fetching logs: %w", err)
	}

	// Convert client response to CLI response format.
	entries := make([]LogEntry, len(resp.Entries))
	for i, e := range resp.Entries {
		entries[i] = LogEntry{
			Timestamp: e.Timestamp,
			Level:     LogLevel(e.Level),
			Service:   e.Service,
			Message:   e.Message,
			Fields:    e.Fields,
			TraceID:   e.TraceID,
		}
	}

	response := LogsResponse{
		Entries:    entries,
		TotalCount: int(resp.TotalCount),
		Truncated:  resp.Truncated,
		Query:      query,
		FetchedAt:  time.Now(),
	}

	return outputLogs(outputFormat, response)
}

// runLogsFollow streams logs in real-time.
func runLogsFollow(ctx context.Context, deps *LogsCommandDeps, query LogQuery, outputFormat config.OutputFormat) error {
	fmt.Println("Following logs (press Ctrl+C to stop)...")
	fmt.Println()

	// Build filter for streaming.
	filter := client.LogFilter{
		Service:  query.Service,
		Level:    query.Level,
		Since:    time.Now(), // Start from now for follow mode
		Contains: query.Contains,
	}

	// Stream logs with 1 second poll interval.
	err := deps.GRPCClient.StreamLogs(ctx, filter, 1000, func(entry client.LogEntry) {
		logEntry := LogEntry{
			Timestamp: entry.Timestamp,
			Level:     LogLevel(entry.Level),
			Service:   entry.Service,
			Message:   entry.Message,
			Fields:    entry.Fields,
			TraceID:   entry.TraceID,
		}
		outputLogEntry(logEntry, logsNoColor)
	})

	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("streaming logs: %w", err)
	}

	fmt.Println("\nStopped following logs.")
	return nil
}

// logLevelMatches checks if entry level meets minimum level.
func logLevelMatches(entryLevel, minLevel LogLevel) bool {
	levels := map[LogLevel]int{
		LogLevelDebug: 0,
		LogLevelInfo:  1,
		LogLevelWarn:  2,
		LogLevelError: 3,
	}

	return levels[entryLevel] >= levels[minLevel]
}

// outputLogs formats and outputs log entries.
func outputLogs(format config.OutputFormat, response LogsResponse) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(response)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(response)
	default:
		return outputLogsText(response)
	}
}

// outputLogsText formats log entries for terminal display.
func outputLogsText(response LogsResponse) error {
	if len(response.Entries) == 0 {
		fmt.Println("No log entries found.")
		return nil
	}

	for _, entry := range response.Entries {
		outputLogEntry(entry, logsNoColor)
	}

	if response.Truncated {
		fmt.Printf("\n(Showing %d entries, use --limit to see more)\n", len(response.Entries))
	}

	return nil
}

// outputLogEntry outputs a single log entry.
func outputLogEntry(entry LogEntry, noColor bool) {
	timestamp := entry.Timestamp.Format("15:04:05")
	levelColor := getLogLevelColor(entry.Level, noColor)
	levelStr := strings.ToUpper(string(entry.Level))

	if noColor {
		fmt.Printf("%s [%-5s] %s: %s", timestamp, levelStr, entry.Service, entry.Message)
	} else {
		fmt.Printf("\033[90m%s\033[0m %s%-5s\033[0m \033[36m%s\033[0m: %s",
			timestamp, levelColor, levelStr, entry.Service, entry.Message)
	}

	// Print fields on same line if few, or new lines if many.
	if len(entry.Fields) > 0 && len(entry.Fields) <= 3 {
		fmt.Print(" {")
		first := true
		for k, v := range entry.Fields {
			if !first {
				fmt.Print(", ")
			}
			fmt.Printf("%s=%s", k, v)
			first = false
		}
		fmt.Print("}")
	}

	fmt.Println()

	// Print fields on separate lines if many.
	if len(entry.Fields) > 3 {
		for k, v := range entry.Fields {
			if noColor {
				fmt.Printf("    %s=%s\n", k, v)
			} else {
				fmt.Printf("    \033[90m%s\033[0m=%s\n", k, v)
			}
		}
	}
}

// getLogLevelColor returns ANSI color code for log level.
func getLogLevelColor(level LogLevel, noColor bool) string {
	if noColor {
		return ""
	}
	switch level {
	case LogLevelDebug:
		return "\033[90m" // Gray
	case LogLevelInfo:
		return "\033[32m" // Green
	case LogLevelWarn:
		return "\033[33m" // Yellow
	case LogLevelError:
		return "\033[31m" // Red
	default:
		return ""
	}
}
