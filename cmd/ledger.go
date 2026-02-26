// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	ledgerv1 "github.com/otherjamesbrown/penf-cli/api/proto/ledger/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Ledger command flags
var (
	ledgerOutput  string
	ledgerType    string
	ledgerSource  string
	ledgerLimit   int
	ledgerSession string
)

// LedgerCommandDeps holds the dependencies for ledger commands.
type LedgerCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultLedgerDeps returns the default dependencies for production use.
func DefaultLedgerDeps() *LedgerCommandDeps {
	return &LedgerCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewLedgerCommand creates the root ledger command with all subcommands.
func NewLedgerCommand(deps *LedgerCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultLedgerDeps()
	}

	cmd := &cobra.Command{
		Use:   "ledger",
		Short: "Session ledger — narrative context capture across sessions",
		Long: `Manage the session ledger for capturing narrative context across sessions.

The ledger stores immutable narrative records: decisions, discoveries, handoffs,
and activity traces. Each entry is tied to a session and agent, building a
persistent narrative of what happened and why.

Entry types:
  narrative   Context, reasoning, background
  decision    Choice made with rationale
  discovery   Finding, insight, root cause
  handoff     Session state for resumption
  activity    System operation (search, entity lookup, etc.)

Examples:
  # List recent entries
  penf ledger

  # Write a decision
  penf ledger write --type decision --title "Use pgvector" --body "Chose pgvector over Pinecone for cost"

  # List sessions
  penf ledger sessions

  # Search entries
  penf ledger search "pipeline architecture"

  # View a specific entry
  penf ledger show 42`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLedgerList(cmd.Context(), deps)
		},
	}

	// Persistent flags
	cmd.PersistentFlags().StringVarP(&ledgerOutput, "output", "o", "", "Output format: text, json, yaml")
	cmd.PersistentFlags().IntVarP(&ledgerLimit, "limit", "l", 20, "Maximum number of results")
	cmd.PersistentFlags().StringVar(&ledgerSession, "session", "", "Filter by session ID")
	cmd.PersistentFlags().StringVar(&ledgerType, "type", "", "Filter by entry type: narrative, decision, discovery, handoff, activity")
	cmd.PersistentFlags().StringVar(&ledgerSource, "source", "", "Filter by source: penf, cxp, manual")

	// Subcommands
	cmd.AddCommand(newLedgerListCommand(deps))
	cmd.AddCommand(newLedgerSessionsCommand(deps))
	cmd.AddCommand(newLedgerSessionCommand(deps))
	cmd.AddCommand(newLedgerShowCommand(deps))
	cmd.AddCommand(newLedgerSearchCommand(deps))
	cmd.AddCommand(newLedgerWriteCommand(deps))
	cmd.AddCommand(newLedgerConsolidationsCommand(deps))
	cmd.AddCommand(newLedgerConsolidationCommand(deps))

	return cmd
}

// newLedgerListCommand creates the 'ledger list' subcommand (explicit alias for root).
func newLedgerListCommand(deps *LedgerCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List ledger entries (same as bare 'penf ledger')",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLedgerList(cmd.Context(), deps)
		},
	}
}

// newLedgerSessionsCommand creates the 'ledger sessions' subcommand.
func newLedgerSessionsCommand(deps *LedgerCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "sessions",
		Short: "List session summaries",
		Long: `List session summaries aggregated from ledger entries.

Shows each session with entry counts, types, and time range.

Examples:
  penf ledger sessions
  penf ledger sessions -l 10`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLedgerSessions(cmd.Context(), deps)
		},
	}
}

// newLedgerSessionCommand creates the 'ledger session' subcommand.
func newLedgerSessionCommand(deps *LedgerCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "session <id>",
		Short: "Show all entries for a session",
		Long: `Show the summary and all entries for a specific session.

Example:
  penf ledger session penfold:a1b2c3d4`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLedgerSession(cmd.Context(), deps, args[0])
		},
	}
}

// newLedgerShowCommand creates the 'ledger show' subcommand.
func newLedgerShowCommand(deps *LedgerCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a single ledger entry",
		Long: `Show detailed information about a specific ledger entry.

Example:
  penf ledger show 42`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid entry ID: %s (must be a positive integer)", args[0])
			}
			return runLedgerShow(cmd.Context(), deps, id)
		},
	}
}

// newLedgerSearchCommand creates the 'ledger search' subcommand.
func newLedgerSearchCommand(deps *LedgerCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search ledger entries",
		Long: `Full-text search across ledger entries.

Searches titles, bodies, and labels.

Examples:
  penf ledger search "pipeline"
  penf ledger search "deployment decision"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			return runLedgerSearch(cmd.Context(), deps, query)
		},
	}
}

// newLedgerWriteCommand creates the 'ledger write' subcommand.
func newLedgerWriteCommand(deps *LedgerCommandDeps) *cobra.Command {
	var (
		writeType     string
		writeTitle    string
		writeBody     string
		writeBodyFile string
		writeSource   string
		writeLabels   []string
		writeRefs     []string
	)

	cmd := &cobra.Command{
		Use:   "write",
		Short: "Write a new ledger entry",
		Long: `Write a new immutable entry to the session ledger.

Entry types:
  narrative   Context, reasoning, background
  decision    Choice made with rationale
  discovery   Finding, insight, root cause
  handoff     Session state for resumption
  activity    System operation

Examples:
  penf ledger write --type decision --title "Use pgvector" --body "Cost and simplicity"
  penf ledger write --type handoff --title "Session end" --body-file /tmp/state.md --label handoff
  penf ledger write --type discovery --title "Root cause" --body "Race condition in worker"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLedgerWrite(cmd.Context(), deps, writeType, writeTitle, writeBody, writeBodyFile, writeSource, writeLabels, writeRefs)
		},
	}

	cmd.Flags().StringVar(&writeType, "type", "narrative", "Entry type: narrative, decision, discovery, handoff, activity")
	cmd.Flags().StringVar(&writeTitle, "title", "", "Entry title (required)")
	cmd.Flags().StringVar(&writeBody, "body", "", "Entry body text")
	cmd.Flags().StringVar(&writeBodyFile, "body-file", "", "Read body from file")
	cmd.Flags().StringVar(&writeSource, "source", "penf", "Entry source: penf, cxp, manual")
	cmd.Flags().StringSliceVar(&writeLabels, "label", nil, "Labels (can be repeated)")
	cmd.Flags().StringSliceVar(&writeRefs, "ref", nil, "Shard references (can be repeated)")
	_ = cmd.MarkFlagRequired("title")

	return cmd
}

// newLedgerConsolidationsCommand creates the 'ledger consolidations' subcommand.
func newLedgerConsolidationsCommand(deps *LedgerCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "consolidations",
		Short: "List consolidated narratives",
		Long: `List LLM-generated consolidations of ledger entries.

Consolidations summarize patterns and decisions across sessions.

Example:
  penf ledger consolidations`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLedgerConsolidations(cmd.Context(), deps)
		},
	}
}

// newLedgerConsolidationCommand creates the 'ledger consolidation' subcommand.
func newLedgerConsolidationCommand(deps *LedgerCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "consolidation <id>",
		Short: "Show a single consolidation",
		Long: `Show detailed information about a specific consolidation.

Example:
  penf ledger consolidation 5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid consolidation ID: %s (must be a positive integer)", args[0])
			}
			return runLedgerConsolidation(cmd.Context(), deps, id)
		},
	}
}

// =============================================================================
// Command Execution Functions
// =============================================================================

func runLedgerList(ctx context.Context, deps *LedgerCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := ledgerv1.NewLedgerServiceClient(conn)

	req := &ledgerv1.ListEntriesRequest{
		TenantId: cfg.TenantID,
		Limit:    int32(ledgerLimit),
	}
	if ledgerSession != "" {
		req.SessionId = &ledgerSession
	}
	if ledgerType != "" {
		req.EntryType = &ledgerType
	}
	if ledgerSource != "" {
		req.Source = &ledgerSource
	}

	resp, err := client.ListEntries(ctx, req)
	if err != nil {
		return fmt.Errorf("listing entries: %w", err)
	}

	return outputLedgerEntries(getLedgerOutputFormat(cfg), resp.Entries, resp.Total)
}

func runLedgerSessions(ctx context.Context, deps *LedgerCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := ledgerv1.NewLedgerServiceClient(conn)

	resp, err := client.ListSessions(ctx, &ledgerv1.ListSessionsRequest{
		TenantId: cfg.TenantID,
		Limit:    int32(ledgerLimit),
	})
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	return outputLedgerSessions(getLedgerOutputFormat(cfg), resp.Sessions, resp.Total)
}

func runLedgerSession(ctx context.Context, deps *LedgerCommandDeps, sessionID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := ledgerv1.NewLedgerServiceClient(conn)

	resp, err := client.GetSession(ctx, &ledgerv1.GetSessionRequest{
		TenantId:  cfg.TenantID,
		SessionId: sessionID,
	})
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}

	format := getLedgerOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputLedgerJSON(resp)
	case config.OutputFormatYAML:
		return outputLedgerYAML(resp)
	default:
		// Print session summary header
		if resp.Summary != nil {
			s := resp.Summary
			fmt.Printf("Session: %s\n", s.SessionId)
			fmt.Printf("Agent: %s | Entries: %d", s.Agent, s.EntryCount)
			if s.FirstEntry != nil {
				fmt.Printf(" | First: %s", s.FirstEntry.AsTime().Format("2006-01-02 15:04"))
			}
			if s.LastEntry != nil {
				fmt.Printf(" | Last: %s", s.LastEntry.AsTime().Format("2006-01-02 15:04"))
			}
			fmt.Println()
			if len(s.EntryTypes) > 0 {
				fmt.Printf("Types: %s\n", strings.Join(s.EntryTypes, ", "))
			}
			if len(s.Labels) > 0 {
				fmt.Printf("Labels: %s\n", strings.Join(s.Labels, ", "))
			}
			fmt.Println()
		}
		// Print entries
		return outputLedgerEntriesText(resp.Entries)
	}
}

func runLedgerShow(ctx context.Context, deps *LedgerCommandDeps, id int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := ledgerv1.NewLedgerServiceClient(conn)

	entry, err := client.GetEntry(ctx, &ledgerv1.GetEntryRequest{
		TenantId: cfg.TenantID,
		Id:       id,
	})
	if err != nil {
		return fmt.Errorf("getting entry: %w", err)
	}

	return outputLedgerEntry(getLedgerOutputFormat(cfg), entry)
}

func runLedgerSearch(ctx context.Context, deps *LedgerCommandDeps, query string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := ledgerv1.NewLedgerServiceClient(conn)

	resp, err := client.SearchEntries(ctx, &ledgerv1.SearchEntriesRequest{
		TenantId: cfg.TenantID,
		Query:    query,
		Limit:    int32(ledgerLimit),
	})
	if err != nil {
		return fmt.Errorf("searching entries: %w", err)
	}

	return outputLedgerEntries(getLedgerOutputFormat(cfg), resp.Entries, resp.Total)
}

func runLedgerWrite(ctx context.Context, deps *LedgerCommandDeps, entryType, title, body, bodyFile, source string, labels, refs []string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Read body from file if specified
	if bodyFile != "" {
		data, err := os.ReadFile(bodyFile)
		if err != nil {
			return fmt.Errorf("reading body file: %w", err)
		}
		body = string(data)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := ledgerv1.NewLedgerServiceClient(conn)

	// Map entry type string to proto enum
	entryTypeEnum := parseEntryType(entryType)

	// Map source string to proto enum
	sourceEnum := parseEntrySource(source)

	// Get session ID from environment
	sessionID := os.Getenv("CLAUDE_SESSION_ID")
	if ledgerSession != "" {
		sessionID = ledgerSession
	}

	req := &ledgerv1.CreateEntryRequest{
		TenantId:  cfg.TenantID,
		SessionId: sessionID,
		EntryType: entryTypeEnum,
		Title:     title,
		Source:    sourceEnum,
		Agent:     "agent-penfold",
		Labels:    labels,
		ShardRefs: refs,
	}
	if body != "" {
		req.Body = &body
	}

	resp, err := client.CreateEntry(ctx, req)
	if err != nil {
		return fmt.Errorf("creating entry: %w", err)
	}

	entry := resp.Entry
	fmt.Printf("\033[32mCreated entry #%d:\033[0m %s\n", entry.Id, entry.Title)
	fmt.Printf("  Type: %s | Source: %s | Session: %s\n",
		entryTypeName(entry.EntryType), entrySourceName(entry.Source), entry.SessionId)
	if len(entry.Labels) > 0 {
		fmt.Printf("  Labels: %s\n", strings.Join(entry.Labels, ", "))
	}

	return nil
}

func runLedgerConsolidations(ctx context.Context, deps *LedgerCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := ledgerv1.NewLedgerServiceClient(conn)

	resp, err := client.ListConsolidations(ctx, &ledgerv1.ListConsolidationsRequest{
		TenantId: cfg.TenantID,
		Limit:    int32(ledgerLimit),
	})
	if err != nil {
		return fmt.Errorf("listing consolidations: %w", err)
	}

	return outputLedgerConsolidations(getLedgerOutputFormat(cfg), resp.Consolidations, resp.Total)
}

func runLedgerConsolidation(ctx context.Context, deps *LedgerCommandDeps, id int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := ledgerv1.NewLedgerServiceClient(conn)

	consolidation, err := client.GetConsolidation(ctx, &ledgerv1.GetConsolidationRequest{
		TenantId: cfg.TenantID,
		Id:       id,
	})
	if err != nil {
		return fmt.Errorf("getting consolidation: %w", err)
	}

	return outputLedgerConsolidation(getLedgerOutputFormat(cfg), consolidation)
}

// =============================================================================
// Activity Logging
// =============================================================================

// logActivity writes a fire-and-forget activity entry to the ledger.
// It runs in a goroutine with its own connection and 2-second timeout.
func logActivity(cfg *config.CLIConfig, title string) {
	go func() {
		sessionID := os.Getenv("CLAUDE_SESSION_ID")
		if sessionID == "" {
			return
		}
		conn, err := connectToGateway(cfg)
		if err != nil {
			return
		}
		defer conn.Close()
		client := ledgerv1.NewLedgerServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		client.CreateEntry(ctx, &ledgerv1.CreateEntryRequest{
			TenantId:  cfg.TenantID,
			SessionId: sessionID,
			EntryType: ledgerv1.EntryType_ENTRY_TYPE_ACTIVITY,
			Title:     title,
			Source:    ledgerv1.EntrySource_ENTRY_SOURCE_PENF,
			Agent:     "agent-penfold",
		})
	}()
}

// =============================================================================
// Output Helpers
// =============================================================================

func getLedgerOutputFormat(cfg *config.CLIConfig) config.OutputFormat {
	if ledgerOutput != "" {
		return config.OutputFormat(ledgerOutput)
	}
	return cfg.OutputFormat
}

func outputLedgerEntries(format config.OutputFormat, entries []*ledgerv1.LedgerEntry, total int32) error {
	switch format {
	case config.OutputFormatJSON:
		return outputLedgerJSON(map[string]interface{}{
			"entries": entries,
			"total":   total,
		})
	case config.OutputFormatYAML:
		return outputLedgerYAML(map[string]interface{}{
			"entries": entries,
			"total":   total,
		})
	default:
		if len(entries) == 0 {
			fmt.Println("No ledger entries found.")
			return nil
		}
		fmt.Printf("Ledger Entries (%d total):\n\n", total)
		return outputLedgerEntriesText(entries)
	}
}

func outputLedgerEntriesText(entries []*ledgerv1.LedgerEntry) error {
	for _, e := range entries {
		typeStr := entryTypeName(e.EntryType)
		when := ""
		if e.CreatedAt != nil {
			when = formatRelativeTime(e.CreatedAt.AsTime())
		}
		fmt.Printf("  \033[1m#%d\033[0m  %-10s  %s\n", e.Id, typeStr, e.Title)
		fmt.Printf("       %s | %s | %s\n", e.SessionId, e.Agent, when)
		if e.Body != nil && *e.Body != "" {
			body := *e.Body
			if len(body) > 120 {
				body = body[:117] + "..."
			}
			fmt.Printf("       %s\n", body)
		}
		if len(e.Labels) > 0 {
			fmt.Printf("       Labels: %s\n", strings.Join(e.Labels, ", "))
		}
		fmt.Println()
	}
	return nil
}

func outputLedgerEntry(format config.OutputFormat, entry *ledgerv1.LedgerEntry) error {
	switch format {
	case config.OutputFormatJSON:
		return outputLedgerJSON(entry)
	case config.OutputFormatYAML:
		return outputLedgerYAML(entry)
	default:
		fmt.Printf("\033[1mEntry #%d\033[0m\n\n", entry.Id)
		fmt.Printf("  \033[1mTitle:\033[0m     %s\n", entry.Title)
		fmt.Printf("  \033[1mType:\033[0m      %s\n", entryTypeName(entry.EntryType))
		fmt.Printf("  \033[1mSource:\033[0m    %s\n", entrySourceName(entry.Source))
		fmt.Printf("  \033[1mSession:\033[0m   %s\n", entry.SessionId)
		fmt.Printf("  \033[1mAgent:\033[0m     %s\n", entry.Agent)
		if len(entry.Labels) > 0 {
			fmt.Printf("  \033[1mLabels:\033[0m    %s\n", strings.Join(entry.Labels, ", "))
		}
		if len(entry.ShardRefs) > 0 {
			fmt.Printf("  \033[1mRefs:\033[0m      %s\n", strings.Join(entry.ShardRefs, ", "))
		}
		if entry.CreatedAt != nil {
			fmt.Printf("  \033[1mCreated:\033[0m   %s\n", entry.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
		}
		if entry.Body != nil && *entry.Body != "" {
			fmt.Printf("\n%s\n", *entry.Body)
		}
		return nil
	}
}

func outputLedgerSessions(format config.OutputFormat, sessions []*ledgerv1.SessionSummary, total int32) error {
	switch format {
	case config.OutputFormatJSON:
		return outputLedgerJSON(map[string]interface{}{
			"sessions": sessions,
			"total":    total,
		})
	case config.OutputFormatYAML:
		return outputLedgerYAML(map[string]interface{}{
			"sessions": sessions,
			"total":    total,
		})
	default:
		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return nil
		}
		fmt.Printf("Sessions (%d total):\n\n", total)
		fmt.Println("  SESSION                  AGENT            ENTRIES  TYPES                    LAST ENTRY")
		fmt.Println("  -------                  -----            -------  -----                    ----------")
		for _, s := range sessions {
			lastEntry := ""
			if s.LastEntry != nil {
				lastEntry = formatRelativeTime(s.LastEntry.AsTime())
			}
			types := strings.Join(s.EntryTypes, ",")
			if len(types) > 24 {
				types = types[:21] + "..."
			}
			title := ""
			if s.LatestTitle != nil {
				title = *s.LatestTitle
				if len(title) > 30 {
					title = title[:27] + "..."
				}
			}
			fmt.Printf("  %-24s %-16s %-7d  %-24s %s\n",
				truncateLedger(s.SessionId, 24),
				truncateLedger(s.Agent, 16),
				s.EntryCount,
				types,
				lastEntry)
			if title != "" {
				fmt.Printf("  └ %s\n", title)
			}
		}
		fmt.Println()
		return nil
	}
}

func outputLedgerConsolidations(format config.OutputFormat, consolidations []*ledgerv1.Consolidation, total int32) error {
	switch format {
	case config.OutputFormatJSON:
		return outputLedgerJSON(map[string]interface{}{
			"consolidations": consolidations,
			"total":          total,
		})
	case config.OutputFormatYAML:
		return outputLedgerYAML(map[string]interface{}{
			"consolidations": consolidations,
			"total":          total,
		})
	default:
		if len(consolidations) == 0 {
			fmt.Println("No consolidations found.")
			return nil
		}
		fmt.Printf("Consolidations (%d total):\n\n", total)
		for _, c := range consolidations {
			when := ""
			if c.CreatedAt != nil {
				when = c.CreatedAt.AsTime().Format("2006-01-02 15:04")
			}
			fmt.Printf("  \033[1m#%d\033[0m  %s  (%s)\n", c.Id, c.Title, when)
			if len(c.SessionIds) > 0 {
				fmt.Printf("       Sessions: %s\n", strings.Join(c.SessionIds, ", "))
			}
			fmt.Printf("       Entries: %d | Decisions: %d | Patterns: %d\n",
				len(c.SourceEntryIds), len(c.Decisions), len(c.Patterns))
			if len(c.Body) > 120 {
				fmt.Printf("       %s...\n", c.Body[:117])
			} else if c.Body != "" {
				fmt.Printf("       %s\n", c.Body)
			}
			fmt.Println()
		}
		return nil
	}
}

func outputLedgerConsolidation(format config.OutputFormat, c *ledgerv1.Consolidation) error {
	switch format {
	case config.OutputFormatJSON:
		return outputLedgerJSON(c)
	case config.OutputFormatYAML:
		return outputLedgerYAML(c)
	default:
		fmt.Printf("\033[1mConsolidation #%d\033[0m\n\n", c.Id)
		fmt.Printf("  \033[1mTitle:\033[0m      %s\n", c.Title)
		if c.TimeStart != nil && c.TimeEnd != nil {
			fmt.Printf("  \033[1mPeriod:\033[0m     %s to %s\n",
				c.TimeStart.AsTime().Format("2006-01-02"),
				c.TimeEnd.AsTime().Format("2006-01-02"))
		}
		if len(c.SessionIds) > 0 {
			fmt.Printf("  \033[1mSessions:\033[0m   %s\n", strings.Join(c.SessionIds, ", "))
		}
		fmt.Printf("  \033[1mEntries:\033[0m    %d source entries\n", len(c.SourceEntryIds))
		if c.ModelId != nil {
			fmt.Printf("  \033[1mModel:\033[0m      %s\n", *c.ModelId)
		}
		if c.CreatedAt != nil {
			fmt.Printf("  \033[1mCreated:\033[0m    %s\n", c.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
		}

		fmt.Printf("\n%s\n", c.Body)

		if len(c.Decisions) > 0 {
			fmt.Printf("\n\033[1mDecisions (%d):\033[0m\n", len(c.Decisions))
			for i, d := range c.Decisions {
				fmt.Printf("  %d. %s\n", i+1, d.Title)
				if d.Body != "" {
					fmt.Printf("     %s\n", d.Body)
				}
				if d.SessionId != "" {
					fmt.Printf("     Session: %s\n", d.SessionId)
				}
			}
		}

		if len(c.Patterns) > 0 {
			fmt.Printf("\n\033[1mPatterns (%d):\033[0m\n", len(c.Patterns))
			for i, p := range c.Patterns {
				fmt.Printf("  %d. %s\n", i+1, p.Title)
				if p.Body != "" {
					fmt.Printf("     %s\n", p.Body)
				}
				if len(p.Evidence) > 0 {
					fmt.Printf("     Evidence: %s\n", strings.Join(p.Evidence, ", "))
				}
			}
		}

		return nil
	}
}

// =============================================================================
// Enum Helpers
// =============================================================================

func parseEntryType(s string) ledgerv1.EntryType {
	switch strings.ToLower(s) {
	case "narrative":
		return ledgerv1.EntryType_ENTRY_TYPE_NARRATIVE
	case "decision":
		return ledgerv1.EntryType_ENTRY_TYPE_DECISION
	case "discovery":
		return ledgerv1.EntryType_ENTRY_TYPE_DISCOVERY
	case "handoff":
		return ledgerv1.EntryType_ENTRY_TYPE_HANDOFF
	case "activity":
		return ledgerv1.EntryType_ENTRY_TYPE_ACTIVITY
	default:
		return ledgerv1.EntryType_ENTRY_TYPE_NARRATIVE
	}
}

func entryTypeName(t ledgerv1.EntryType) string {
	switch t {
	case ledgerv1.EntryType_ENTRY_TYPE_NARRATIVE:
		return "narrative"
	case ledgerv1.EntryType_ENTRY_TYPE_DECISION:
		return "decision"
	case ledgerv1.EntryType_ENTRY_TYPE_DISCOVERY:
		return "discovery"
	case ledgerv1.EntryType_ENTRY_TYPE_HANDOFF:
		return "handoff"
	case ledgerv1.EntryType_ENTRY_TYPE_ACTIVITY:
		return "activity"
	default:
		return "unknown"
	}
}

func parseEntrySource(s string) ledgerv1.EntrySource {
	switch strings.ToLower(s) {
	case "penf":
		return ledgerv1.EntrySource_ENTRY_SOURCE_PENF
	case "cxp":
		return ledgerv1.EntrySource_ENTRY_SOURCE_CXP
	case "manual":
		return ledgerv1.EntrySource_ENTRY_SOURCE_MANUAL
	default:
		return ledgerv1.EntrySource_ENTRY_SOURCE_PENF
	}
}

func entrySourceName(s ledgerv1.EntrySource) string {
	switch s {
	case ledgerv1.EntrySource_ENTRY_SOURCE_PENF:
		return "penf"
	case ledgerv1.EntrySource_ENTRY_SOURCE_CXP:
		return "cxp"
	case ledgerv1.EntrySource_ENTRY_SOURCE_MANUAL:
		return "manual"
	default:
		return "unknown"
	}
}

// =============================================================================
// Generic Output Helpers
// =============================================================================

func outputLedgerJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputLedgerYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

func truncateLedger(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
