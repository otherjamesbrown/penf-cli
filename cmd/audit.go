// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Audit command flags
var (
	auditOutput         string
	auditLimit          int
	auditContentID      int64
	auditContentType    string
	auditSince          string
	auditStage          int
	auditShowDecisions  bool
	auditShowLLMCalls   bool
	auditHadCorrections bool
	// Comparison flags
	auditShowDivergences bool
	auditMention         string
	auditShowReasoning   bool
	auditDaysSince       int
)

// AuditCommandDeps holds the dependencies for audit commands.
type AuditCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultAuditDeps returns the default dependencies for production use.
func DefaultAuditDeps() *AuditCommandDeps {
	return &AuditCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewAuditCommand creates the root audit command with all subcommands.
func NewAuditCommand(deps *AuditCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultAuditDeps()
	}

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit and inspect mention resolution traces",
		Long: `Audit and inspect mention resolution traces.

Resolution traces capture the complete decision-making process for
mention resolution, including:
  - Stage 1: Understanding - Extract and understand mentions
  - Stage 2: Cross-Mention - Reason across mentions in same content
  - Stage 3: Matching - Match to candidate entities
  - Stage 4: Verification - Verify uncertain resolutions

Each trace includes timing, decisions, reasoning, and optionally
full LLM prompts/responses for debugging.

Documentation:
  Entity resolution:   docs/concepts/mention-resolution.md
  Entity model:        docs/shared/entities.md`,
	}

	// Add persistent flags
	cmd.PersistentFlags().StringVarP(&auditOutput, "output", "o", "", "Output format: text, json, yaml")
	cmd.PersistentFlags().IntVarP(&auditLimit, "limit", "l", 20, "Maximum number of results")

	// Add subcommands
	cmd.AddCommand(newAuditTracesCommand(deps))
	cmd.AddCommand(newAuditTraceCommand(deps))
	cmd.AddCommand(newAuditCorrectionsCommand(deps))
	cmd.AddCommand(newAuditComparisonsCommand(deps))
	cmd.AddCommand(newAuditModelsCommand(deps))

	return cmd
}

// newAuditTracesCommand creates the 'audit traces' subcommand.
func newAuditTracesCommand(deps *AuditCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "traces",
		Short: "List resolution traces",
		Long: `List resolution traces with optional filtering.

Examples:
  # List recent traces
  penf audit traces --limit 10

  # List traces for a specific content
  penf audit traces --content-id 4521

  # List meeting traces from last 7 days
  penf audit traces --content-type meeting --since 7d

  # List traces that had corrections
  penf audit traces --had-corrections`,
		Aliases: []string{"list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuditTraces(cmd.Context(), deps)
		},
	}

	cmd.Flags().Int64Var(&auditContentID, "content-id", 0, "Filter by content ID")
	cmd.Flags().StringVar(&auditContentType, "content-type", "", "Filter by content type (email, meeting, document)")
	cmd.Flags().StringVar(&auditSince, "since", "", "Show traces since duration (e.g., 7d, 24h)")
	cmd.Flags().BoolVar(&auditHadCorrections, "had-corrections", false, "Only show traces with corrections")

	return cmd
}

// newAuditTraceCommand creates the 'audit trace' subcommand.
func newAuditTraceCommand(deps *AuditCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace <trace-id>",
		Short: "Show details of a specific trace",
		Long: `Show detailed information about a resolution trace.

Examples:
  # Show trace summary
  penf audit trace trace_abc123

  # Show specific stage detail
  penf audit trace trace_abc123 --stage 2

  # Show all decisions with reasoning
  penf audit trace trace_abc123 --decisions

  # Show LLM calls (requires full/debug trace level)
  penf audit trace trace_abc123 --llm-calls

  # Export as JSON
  penf audit trace trace_abc123 -o json`,
		Aliases: []string{"show"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuditTrace(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().IntVar(&auditStage, "stage", 0, "Show only specific stage (1-4)")
	cmd.Flags().BoolVar(&auditShowDecisions, "decisions", false, "Show all decisions with reasoning")
	cmd.Flags().BoolVar(&auditShowLLMCalls, "llm-calls", false, "Show LLM prompts/responses")

	return cmd
}

// newAuditCorrectionsCommand creates the 'audit corrections' subcommand.
func newAuditCorrectionsCommand(deps *AuditCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "corrections",
		Short: "List and analyze corrections",
		Long: `List decisions that were corrected by humans.

Corrections are valuable for understanding where the AI made mistakes
and improving future resolution accuracy.

Examples:
  # List recent corrections
  penf audit corrections --limit 20

  # Show corrections from last 30 days
  penf audit corrections --since 30d`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuditCorrections(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&auditSince, "since", "30d", "Show corrections since duration")

	return cmd
}

// runAuditTraces lists resolution traces.
func runAuditTraces(ctx context.Context, deps *AuditCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	auditClient := client.NewAuditClient(conn)

	var since *time.Time
	if auditSince != "" {
		duration, err := parseDuration(auditSince)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		t := time.Now().Add(-duration)
		since = &t
	}

	traces, _, err := auditClient.ListTraces(ctx, getTenantID(), auditContentID, auditContentType, since, auditHadCorrections, int32(auditLimit), 0)
	if err != nil {
		return fmt.Errorf("list traces: %w", err)
	}

	if len(traces) == 0 {
		fmt.Println("No traces found.")
		return nil
	}

	switch auditOutput {
	case "json":
		return outputAuditJSON(traces)
	case "yaml":
		return outputAuditYAML(traces)
	default:
		return outputTracesTable(traces)
	}
}

// runAuditTrace shows details of a specific trace.
func runAuditTrace(ctx context.Context, deps *AuditCommandDeps, traceID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	auditClient := client.NewAuditClient(conn)

	detail, err := auditClient.GetTrace(ctx, traceID, auditShowLLMCalls)
	if err != nil {
		return fmt.Errorf("get trace: %w", err)
	}

	switch auditOutput {
	case "json":
		return outputAuditJSON(detail)
	case "yaml":
		return outputAuditYAML(detail)
	default:
		return outputTraceDetail(detail, auditStage, auditShowDecisions, auditShowLLMCalls)
	}
}

// runAuditCorrections lists corrections.
func runAuditCorrections(ctx context.Context, deps *AuditCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	auditClient := client.NewAuditClient(conn)

	var since *time.Time
	if auditSince != "" {
		duration, err := parseDuration(auditSince)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		t := time.Now().Add(-duration)
		since = &t
	}

	corrections, _, err := auditClient.ListCorrections(ctx, getTenantID(), since, int32(auditLimit), 0)
	if err != nil {
		return fmt.Errorf("get corrections: %w", err)
	}

	if len(corrections) == 0 {
		fmt.Println("No corrections found.")
		return nil
	}

	switch auditOutput {
	case "json":
		return outputAuditJSON(corrections)
	case "yaml":
		return outputAuditYAML(corrections)
	default:
		return outputCorrectionsTable(corrections)
	}
}

// outputTracesTable outputs traces as a table.
func outputTracesTable(traces []client.TraceSummary) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCONTENT\tSTATUS\tMENTIONS\tRESULT\tMODEL\tDURATION\tSTARTED")
	fmt.Fprintln(w, "--\t-------\t------\t--------\t------\t-----\t--------\t-------")

	for _, t := range traces {
		contentDesc := fmt.Sprintf("%s #%d", t.ContentType, t.ContentID)
		if t.ContentSummary != "" {
			if len(t.ContentSummary) > 30 {
				contentDesc = t.ContentSummary[:30] + "..."
			} else {
				contentDesc = t.ContentSummary
			}
		}

		result := fmt.Sprintf("%d auto, %d review", t.AutoResolved, t.QueuedForReview)
		duration := fmt.Sprintf("%dms", t.DurationMs)
		started := t.StartedAt.Format("01-02 15:04")

		statusIcon := "✓"
		if t.Status == "failed" {
			statusIcon = "✗"
		} else if t.Status == "in_progress" {
			statusIcon = "◐"
		}

		fmt.Fprintf(w, "%s\t%s\t%s %s\t%d\t%s\t%s\t%s\t%s\n",
			t.ID, contentDesc, statusIcon, t.Status, t.MentionsFound, result, t.ModelUsed, duration, started)
	}

	return w.Flush()
}

// outputTraceDetail outputs trace detail.
func outputTraceDetail(detail *client.TraceDetail, stageFilter int, showDecisions, showLLMCalls bool) error {
	t := detail.Trace

	fmt.Printf("Trace: %s\n", t.ID)
	if t.ContentSummary != "" {
		fmt.Printf("Content: %s #%d \"%s\"\n", t.ContentType, t.ContentID, t.ContentSummary)
	} else {
		fmt.Printf("Content: %s #%d\n", t.ContentType, t.ContentID)
	}
	fmt.Printf("Model: %s\n", t.ModelUsed)
	fmt.Printf("Status: %s\n", t.Status)
	fmt.Printf("Duration: %dms\n", t.DurationMs)
	fmt.Printf("Outcome: %d auto-resolved, %d queued for review\n", t.AutoResolved, t.QueuedForReview)
	if detail.NewEntitiesSuggested > 0 {
		fmt.Printf("New entities suggested: %d\n", detail.NewEntitiesSuggested)
	}
	fmt.Println()

	// Show stages
	fmt.Println("Stages:")
	for _, s := range detail.Stages {
		if stageFilter > 0 && s.StageNumber != int32(stageFilter) {
			continue
		}

		statusIcon := "✓"
		if s.Status == "failed" {
			statusIcon = "✗"
		} else if s.Status == "skipped" {
			statusIcon = "○"
		} else if s.Status == "in_progress" {
			statusIcon = "◐"
		}

		fmt.Printf("  %s Stage %d: %s (%dms)\n", statusIcon, s.StageNumber, s.StageName, s.DurationMs)
		if s.OutputSummary != "" {
			fmt.Printf("    %s\n", s.OutputSummary)
		}
		if s.Skipped {
			fmt.Printf("    Skipped: %s\n", s.SkipReason)
		}
		if s.ErrorMessage != "" {
			fmt.Printf("    Error: %s\n", s.ErrorMessage)
		}
	}

	// Show decisions if requested
	if showDecisions && len(detail.Decisions) > 0 {
		fmt.Println()
		fmt.Println("Decisions:")
		for i, d := range detail.Decisions {
			stageIDStr := "unknown"
			if d.StageID != nil {
				stageIDStr = fmt.Sprintf("%d", *d.StageID)
			}
			fmt.Printf("\n#%d [Stage %s] %s \"%s\"", i+1, stageIDStr, strings.ToUpper(d.DecisionType), d.MentionedText)
			if d.ChosenOption != "" {
				fmt.Printf(" → %s", d.ChosenOption)
			}
			fmt.Println()
			fmt.Printf("   Confidence: %.2f\n", d.Confidence)
			if d.Reasoning != "" {
				fmt.Printf("   Reasoning: %s\n", wrapText(d.Reasoning, 60, "              "))
			}
			if d.WasCorrect != nil && !*d.WasCorrect {
				fmt.Printf("   ⚠️  CORRECTED: %s\n", d.CorrectionNotes)
			}
		}
	}

	// Show LLM calls if requested
	if showLLMCalls && len(detail.LLMCalls) > 0 {
		fmt.Println()
		fmt.Println("LLM Calls:")
		for i, c := range detail.LLMCalls {
			fmt.Printf("\n#%d [%s] %dms\n", i+1, c.Model, c.LatencyMs)
			if c.PromptText != "" {
				fmt.Println("   Prompt:")
				fmt.Println(indent(truncate(c.PromptText, 500), "     "))
			}
			if c.ResponseText != "" {
				fmt.Println("   Response:")
				fmt.Println(indent(truncate(c.ResponseText, 500), "     "))
			}
		}
	}

	return nil
}

// outputCorrectionsTable outputs corrections as a table.
func outputCorrectionsTable(corrections []client.Decision) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TRACE\tMENTION\tDECISION\tCHOSEN\tCONF\tNOTE")
	fmt.Fprintln(w, "-----\t-------\t--------\t------\t----\t----")

	for _, c := range corrections {
		note := c.CorrectionNotes
		if len(note) > 40 {
			note = note[:40] + "..."
		}
		traceIDShort := c.TraceID
		if len(traceIDShort) > 12 {
			traceIDShort = traceIDShort[:12] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.2f\t%s\n",
			traceIDShort, c.MentionedText, c.DecisionType, c.ChosenOption, c.Confidence, note)
	}

	return w.Flush()
}

// Helper functions

func getTenantID() string {
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant
	}
	// Default tenant.
	return "00000001-0000-0000-0000-000000000001"
}

func parseDuration(s string) (time.Duration, error) {
	// Handle day suffix
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err != nil {
			return 0, err
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func outputAuditJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputAuditYAML(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var m interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	// Simple YAML-like output
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

func wrapText(text string, width int, indent string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len() > 0 && currentLine.Len()+1+len(word) > width {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
		}
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	for i := 1; i < len(lines); i++ {
		lines[i] = indent + lines[i]
	}

	return strings.Join(lines, "\n")
}

func indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func truncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// =====================================================
// Comparison Commands
// =====================================================

// newAuditComparisonsCommand creates the 'audit comparisons' subcommand.
func newAuditComparisonsCommand(deps *AuditCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comparisons",
		Short: "List and inspect model comparisons",
		Long: `List and inspect model comparison runs.

Comparisons run the same content through multiple LLM models and
compare their resolution decisions to identify divergences.`,
	}

	cmd.AddCommand(newAuditComparisonsListCommand(deps))
	cmd.AddCommand(newAuditComparisonShowCommand(deps))

	return cmd
}

// newAuditComparisonsListCommand creates the 'audit comparisons list' subcommand.
func newAuditComparisonsListCommand(deps *AuditCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List model comparisons",
		Long: `List model comparison runs with optional filtering.

Examples:
  # List recent comparisons
  penf audit comparisons list --limit 10

  # List comparisons from last 7 days
  penf audit comparisons list --since 7d`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuditComparisonsList(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&auditSince, "since", "", "Show comparisons since duration (e.g., 7d, 24h)")

	return cmd
}

// newAuditComparisonShowCommand creates the 'audit comparisons show' subcommand.
func newAuditComparisonShowCommand(deps *AuditCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <comparison-id>",
		Short: "Show details of a comparison",
		Long: `Show detailed information about a model comparison.

Examples:
  # Show comparison summary
  penf audit comparisons show comp_abc123

  # Show only divergent decisions
  penf audit comparisons show comp_abc123 --divergences

  # Show reasoning for a specific mention
  penf audit comparisons show comp_abc123 --mention "John" --reasoning`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuditComparisonShow(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().BoolVar(&auditShowDivergences, "divergences", false, "Show only divergent decisions")
	cmd.Flags().StringVar(&auditMention, "mention", "", "Filter by mention text")
	cmd.Flags().BoolVar(&auditShowReasoning, "reasoning", false, "Show model reasoning for each decision")

	return cmd
}

// newAuditModelsCommand creates the 'audit models' subcommand.
func newAuditModelsCommand(deps *AuditCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Model statistics and performance",
		Long:  `View statistics and performance metrics for LLM models used in resolution.`,
	}

	cmd.AddCommand(newAuditModelsStatsCommand(deps))

	return cmd
}

// newAuditModelsStatsCommand creates the 'audit models stats' subcommand.
func newAuditModelsStatsCommand(deps *AuditCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show model statistics",
		Long: `Show aggregate statistics for each model.

Displays comparisons run, decisions made, accuracy (if ground truth available),
average confidence, and other performance metrics.

Examples:
  # Show stats from last 30 days
  penf audit models stats

  # Show stats from last 7 days
  penf audit models stats --days 7`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuditModelsStats(cmd.Context(), deps)
		},
	}

	cmd.Flags().IntVar(&auditDaysSince, "days", 30, "Show stats from last N days")

	return cmd
}

// runAuditComparisonsList lists model comparisons.
func runAuditComparisonsList(ctx context.Context, deps *AuditCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	auditClient := client.NewAuditClient(conn)

	var since *time.Time
	if auditSince != "" {
		duration, err := parseDuration(auditSince)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		t := time.Now().Add(-duration)
		since = &t
	}

	comparisons, _, err := auditClient.ListComparisons(ctx, getTenantID(), since, int32(auditLimit), 0)
	if err != nil {
		return fmt.Errorf("list comparisons: %w", err)
	}

	if len(comparisons) == 0 {
		fmt.Println("No comparisons found.")
		return nil
	}

	switch auditOutput {
	case "json":
		return outputAuditJSON(comparisons)
	case "yaml":
		return outputAuditYAML(comparisons)
	default:
		return outputComparisonsTable(comparisons)
	}
}

// runAuditComparisonShow shows details of a specific comparison.
func runAuditComparisonShow(ctx context.Context, deps *AuditCommandDeps, comparisonID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	auditClient := client.NewAuditClient(conn)

	detail, err := auditClient.GetComparison(ctx, comparisonID)
	if err != nil {
		return fmt.Errorf("get comparison: %w", err)
	}

	switch auditOutput {
	case "json":
		return outputAuditJSON(detail)
	case "yaml":
		return outputAuditYAML(detail)
	default:
		return outputComparisonDetail(detail)
	}
}

// runAuditModelsStats shows model statistics.
func runAuditModelsStats(ctx context.Context, deps *AuditCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	auditClient := client.NewAuditClient(conn)

	stats, err := auditClient.GetModelStats(ctx, getTenantID(), int32(auditDaysSince))
	if err != nil {
		return fmt.Errorf("get model stats: %w", err)
	}

	if len(stats) == 0 {
		fmt.Println("No model statistics found.")
		return nil
	}

	switch auditOutput {
	case "json":
		return outputAuditJSON(stats)
	case "yaml":
		return outputAuditYAML(stats)
	default:
		return outputModelStatsTable(stats)
	}
}

// outputComparisonsTable outputs comparisons as a table.
func outputComparisonsTable(comparisons []client.ComparisonSummary) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCONTENT\tMODELS\tDECISIONS\tDIVERGENT\tSTARTED")
	fmt.Fprintln(w, "--\t-------\t------\t---------\t---------\t-------")

	for _, c := range comparisons {
		contentDesc := fmt.Sprintf("%s #%d", c.ContentType, c.ContentID)
		if c.ContentSummary != "" {
			if len(c.ContentSummary) > 25 {
				contentDesc = c.ContentSummary[:25] + "..."
			} else {
				contentDesc = c.ContentSummary
			}
		}

		modelsStr := strings.Join(c.Models, ", ")
		if len(modelsStr) > 20 {
			modelsStr = modelsStr[:17] + "..."
		}

		divergentStr := fmt.Sprintf("%d", c.DivergentDecisions)
		if c.DivergentDecisions > 0 {
			divergentStr = fmt.Sprintf("\033[33m%d\033[0m", c.DivergentDecisions) // Yellow
		}

		started := c.StartedAt.Format("01-02 15:04")

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
			c.ID, contentDesc, modelsStr, c.TotalDecisions, divergentStr, started)
	}

	return w.Flush()
}

// outputComparisonDetail outputs comparison detail.
func outputComparisonDetail(detail *client.ComparisonDetail) error {
	c := detail.Comparison

	fmt.Printf("Comparison: %s\n", c.ID)
	if c.ContentSummary != "" {
		fmt.Printf("Content: %s #%d \"%s\"\n", c.ContentType, c.ContentID, c.ContentSummary)
	} else {
		fmt.Printf("Content: %s #%d\n", c.ContentType, c.ContentID)
	}
	fmt.Printf("Models: %s\n", strings.Join(c.Models, ", "))
	fmt.Printf("Purpose: %s\n", detail.Purpose)
	fmt.Printf("Initiated by: %s\n", detail.InitiatedBy)
	fmt.Println()
	fmt.Printf("Results:\n")
	fmt.Printf("  Total decisions: %d\n", c.TotalDecisions)
	fmt.Printf("  Unanimous: %d\n", c.UnanimousDecisions)
	fmt.Printf("  Divergent: %d\n", c.DivergentDecisions)
	fmt.Println()

	// Show decisions
	decisions := detail.Decisions
	if auditShowDivergences {
		var divergent []client.ComparisonDecision
		for _, d := range decisions {
			if !d.IsUnanimous {
				divergent = append(divergent, d)
			}
		}
		decisions = divergent
	}

	if auditMention != "" {
		var filtered []client.ComparisonDecision
		for _, d := range decisions {
			if strings.Contains(strings.ToLower(d.MentionedText), strings.ToLower(auditMention)) {
				filtered = append(filtered, d)
			}
		}
		decisions = filtered
	}

	if len(decisions) == 0 {
		fmt.Println("No matching decisions.")
		return nil
	}

	fmt.Printf("Decisions (%d):\n", len(decisions))
	for i, d := range decisions {
		status := "✓"
		if !d.IsUnanimous {
			status = fmt.Sprintf("\033[33m⚡\033[0m %s", d.DivergenceType)
		}

		fmt.Printf("\n#%d \"%s\" %s\n", i+1, d.MentionedText, status)

		for _, md := range d.ModelDecisions {
			entity := "suggested new"
			if md.EntityID != nil {
				entity = fmt.Sprintf("%s (#%d)", md.EntityName, *md.EntityID)
			}
			fmt.Printf("  %s: → %s (conf: %.2f)\n", md.Model, entity, md.Confidence)
			if auditShowReasoning && md.Reasoning != "" {
				fmt.Printf("    Reasoning: %s\n", wrapText(md.Reasoning, 60, "              "))
			}
		}

		if d.GroundTruthEntityID != nil {
			correctModels := strings.Join(d.ModelsCorrect, ", ")
			if len(d.ModelsCorrect) == 0 {
				correctModels = "none"
			}
			fmt.Printf("  Ground truth: #%d (correct: %s)\n", *d.GroundTruthEntityID, correctModels)
		}
	}

	return nil
}

// outputModelStatsTable outputs model statistics as a table.
func outputModelStatsTable(stats []client.ModelStats) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODEL\tCOMPARISONS\tDECISIONS\tCORRECT\tACCURACY\tAVG CONF")
	fmt.Fprintln(w, "-----\t-----------\t---------\t-------\t--------\t--------")

	for _, s := range stats {
		accuracy := "-"
		if s.Accuracy > 0 {
			accuracy = fmt.Sprintf("%.1f%%", s.Accuracy*100)
		}

		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%s\t%.2f\n",
			s.Model, s.TotalComparisons, s.TotalDecisions, s.CorrectDecisions, accuracy, s.AverageConfidence)
	}

	return w.Flush()
}
