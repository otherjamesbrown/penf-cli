// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

// NewContextCommand creates the `penf context` command. Accepts optional deps for testing.
func NewContextCommand(deps interface{}) *cobra.Command {
	var pipelineDeps *PipelineCommandDeps
	if d, ok := deps.(*PipelineCommandDeps); ok && d != nil {
		pipelineDeps = d
	} else {
		pipelineDeps = DefaultPipelineDeps()
	}
	return newContextCmd(pipelineDeps)
}

// newContextCmd creates the `penf context` command tree.
func newContextCmd(deps *PipelineCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage personal reference context",
		Long: `Manage personal reference context entries for email analysis.

Context entries are personal facts (cars, addresses, health providers, subscriptions)
that help the analysis LLM understand your emails. Each entry can have trigger conditions
that determine when the context is injected into the analysis.

Commands:
  add     - Create a new context entry with LLM-suggested triggers
  list    - List all context entries
  show    - Show a context entry with its trigger conditions
  remove  - Delete a context entry
  trigger - Manage trigger conditions for an entry

Examples:
  # Add a vehicle entry with LLM-suggested triggers
  penf context add --category vehicle --label "Porsche Macan" \
    --details '{"reg": "VO72 UHX", "status": "owned"}'

  # Add an identity entry (always injected)
  penf context add --category identity --label "James Brown" \
    --details '{"role": "engineer"}' --always-inject

  # List all entries
  penf context list

  # Show a specific entry
  penf context show 1

  # Add a trigger condition manually
  penf context trigger add 1 --condition "sender_email:contains:porsche.com"`,
	}

	cmd.AddCommand(newContextAddCmd(deps))
	cmd.AddCommand(newContextListCmd(deps))
	cmd.AddCommand(newContextShowCmd(deps))
	cmd.AddCommand(newContextRemoveCmd(deps))
	cmd.AddCommand(newContextTriggerCmd(deps))

	// Default action is list
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runContextList(cmd.Context(), deps, "", "text")
	}

	return cmd
}

func newContextAddCmd(deps *PipelineCommandDeps) *cobra.Command {
	var (
		category     string
		label        string
		detailsJSON  string
		alwaysInject bool
		noSuggest    bool
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new context entry",
		Long: `Create a new personal context entry.

By default, the AI will suggest trigger conditions based on the entry details.
Use --no-suggest to skip the suggestion step.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContextAdd(cmd.Context(), deps, category, label, detailsJSON, alwaysInject, noSuggest)
		},
	}

	cmd.Flags().StringVar(&category, "category", "", "Entry category (vehicle, identity, property, health, subscription, finance, other)")
	cmd.Flags().StringVar(&label, "label", "", "Human-readable label (e.g. 'Porsche Macan')")
	cmd.Flags().StringVar(&detailsJSON, "details", "{}", "Entry details as JSON")
	cmd.Flags().BoolVar(&alwaysInject, "always-inject", false, "Always inject this entry regardless of trigger conditions")
	cmd.Flags().BoolVar(&noSuggest, "no-suggest", false, "Skip LLM trigger suggestion")
	_ = cmd.MarkFlagRequired("category")
	_ = cmd.MarkFlagRequired("label")

	return cmd
}

func newContextListCmd(deps *PipelineCommandDeps) *cobra.Command {
	var (
		category     string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List context entries",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContextList(cmd.Context(), deps, category, outputFormat)
		},
	}

	cmd.Flags().StringVar(&category, "category", "", "Filter by category")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func newContextShowCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a context entry with its trigger conditions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid id %q: %w", args[0], err)
			}
			return runContextShow(cmd.Context(), deps, int32(id), outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func newContextRemoveCmd(deps *PipelineCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <id>",
		Short:   "Delete a context entry",
		Aliases: []string{"rm", "delete"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid id %q: %w", args[0], err)
			}
			return runContextRemove(cmd.Context(), deps, int32(id))
		},
	}
}

func newContextTriggerCmd(deps *PipelineCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger",
		Short: "Manage trigger conditions for a context entry",
	}
	cmd.AddCommand(newContextTriggerAddCmd(deps))
	cmd.AddCommand(newContextTriggerRemoveCmd(deps))
	return cmd
}

func newContextTriggerAddCmd(deps *PipelineCommandDeps) *cobra.Command {
	var condition string

	cmd := &cobra.Command{
		Use:   "add <entry-id>",
		Short: "Add a trigger condition to a context entry",
		Long: `Add a trigger condition to a context entry.

The condition format is field:match_type:value.

Valid fields: content, sender_email, subject, to_address
Valid match types: contains, prefix, suffix, exact, glob

Examples:
  penf context trigger add 1 --condition "sender_email:contains:porsche.com"
  penf context trigger add 1 --condition "content:contains:Macan"
  penf context trigger add 1 --condition "subject:contains:service due"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid id %q: %w", args[0], err)
			}
			return runContextTriggerAdd(cmd.Context(), deps, int32(id), condition)
		},
	}

	cmd.Flags().StringVar(&condition, "condition", "", "Condition in format field:match_type:value (required)")
	_ = cmd.MarkFlagRequired("condition")

	return cmd
}

func newContextTriggerRemoveCmd(deps *PipelineCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <entry-id> <condition-id>",
		Short: "Remove a trigger condition from a context entry",
		Long: `Remove a trigger condition from a context entry.

Examples:
  penf context trigger remove 1 3`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			entryID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid entry id %q: %w", args[0], err)
			}
			condID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid condition id %q: %w", args[1], err)
			}
			return runContextTriggerRemove(cmd.Context(), deps, int32(entryID), int32(condID))
		},
	}
}

// ===========================================================================
// Execution functions
// ===========================================================================

func runContextAdd(ctx context.Context, deps *PipelineCommandDeps, category, label, detailsJSON string, alwaysInject, noSuggest bool) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.EffectiveTenantID()
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	if detailsJSON == "" {
		detailsJSON = "{}"
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	// Step 1: Get LLM-suggested trigger conditions
	var acceptedConditions []*pipelinev1.TenantContextCondition

	if !noSuggest && !alwaysInject {
		fmt.Printf("Generating trigger suggestions for %s %q...\n", category, label)
		suggestResp, suggestErr := client.SuggestTenantContextTriggers(ctx, &pipelinev1.SuggestTenantContextTriggersRequest{
			TenantId:    tenantID,
			Category:    category,
			Label:       label,
			DetailsJson: detailsJSON,
		})
		if suggestErr != nil {
			fmt.Printf("  Warning: could not generate suggestions (%v), continuing without them.\n", suggestErr)
		} else if len(suggestResp.Conditions) > 0 {
			acceptedConditions = promptAcceptConditions(suggestResp.Conditions)
		}
	}

	// Step 2: Create the entry
	resp, err := client.CreateTenantContext(ctx, &pipelinev1.CreateTenantContextRequest{
		TenantId:     tenantID,
		Category:     category,
		Label:        label,
		DetailsJson:  detailsJSON,
		AlwaysInject: alwaysInject,
		Conditions:   acceptedConditions,
	})
	if err != nil {
		return fmt.Errorf("creating context entry: %w", err)
	}

	entry := resp.Entry
	fmt.Printf("Created context entry %d: %s / %s", entry.Id, entry.Category, entry.Label)
	if entry.AlwaysInject {
		fmt.Print(" (always-inject)")
	}
	fmt.Println()
	if len(entry.Conditions) > 0 {
		fmt.Printf("  Trigger conditions (%d):\n", len(entry.Conditions))
		for _, c := range entry.Conditions {
			fmt.Printf("    %s:%s:%s\n", c.Field, c.MatchType, c.Value)
		}
	} else if !entry.AlwaysInject {
		fmt.Println("  Warning: entry has no conditions and always_inject=false — it will never be injected")
	}
	return nil
}

// promptAcceptConditions shows the suggested conditions and asks the user to accept/reject.
func promptAcceptConditions(conditions []*pipelinev1.TenantContextCondition) []*pipelinev1.TenantContextCondition {
	fmt.Printf("\nSuggested triggers (%d):\n", len(conditions))
	for i, c := range conditions {
		fmt.Printf("  %d. %s:%s:%s\n", i+1, c.Field, c.MatchType, c.Value)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nAccept all? [Y/n/edit] ")
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))

	switch line {
	case "", "y", "yes":
		return conditions
	case "n", "no":
		fmt.Println("  No trigger conditions added.")
		return nil
	case "edit", "e":
		return editConditions(conditions, reader)
	default:
		// Accept if empty or y/yes
		return conditions
	}
}

// editConditions allows the user to select which conditions to keep.
func editConditions(conditions []*pipelinev1.TenantContextCondition, reader *bufio.Reader) []*pipelinev1.TenantContextCondition {
	fmt.Println("Enter numbers to keep (e.g. '1 2 4'), or 'all', or 'none':")
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))

	if line == "all" || line == "" {
		return conditions
	}
	if line == "none" {
		return nil
	}

	var kept []*pipelinev1.TenantContextCondition
	for _, part := range strings.Fields(line) {
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > len(conditions) {
			continue
		}
		kept = append(kept, conditions[n-1])
	}
	return kept
}

func runContextList(ctx context.Context, deps *PipelineCommandDeps, category, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.EffectiveTenantID()
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.ListTenantContext(ctx, &pipelinev1.ListTenantContextRequest{
		TenantId: tenantID,
		Category: category,
	})
	if err != nil {
		return fmt.Errorf("listing context entries: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Entries)
	}

	return outputContextListHuman(resp.Entries)
}

func runContextShow(ctx context.Context, deps *PipelineCommandDeps, id int32, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.EffectiveTenantID()
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.GetTenantContext(ctx, &pipelinev1.GetTenantContextRequest{
		TenantId: tenantID,
		Id:       id,
	})
	if err != nil {
		return fmt.Errorf("getting context entry %d: %w", id, err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Entry)
	}

	return outputContextShowHuman(resp.Entry)
}

func runContextRemove(ctx context.Context, deps *PipelineCommandDeps, id int32) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.EffectiveTenantID()
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Delete context entry %d? This will also remove all its trigger conditions. [y/N] ", id)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	_, err = client.DeleteTenantContext(ctx, &pipelinev1.DeleteTenantContextRequest{
		TenantId: tenantID,
		Id:       id,
	})
	if err != nil {
		return fmt.Errorf("removing context entry %d: %w", id, err)
	}

	fmt.Printf("Removed context entry %d\n", id)
	return nil
}

func runContextTriggerAdd(ctx context.Context, deps *PipelineCommandDeps, entryID int32, condition string) error {
	cond, err := parseTenantContextCondition(condition)
	if err != nil {
		return err
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.EffectiveTenantID()
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.AddTenantContextCondition(ctx, &pipelinev1.AddTenantContextConditionRequest{
		TenantId:  tenantID,
		ContextId: entryID,
		Field:     cond.Field,
		MatchType: cond.MatchType,
		Value:     cond.Value,
	})
	if err != nil {
		return fmt.Errorf("adding condition to entry %d: %w", entryID, err)
	}

	entry := resp.Entry
	if entry != nil && len(entry.Conditions) > 0 {
		added := entry.Conditions[len(entry.Conditions)-1]
		fmt.Printf("Added condition %d to entry %d: %s:%s:%s\n", added.Id, entryID, added.Field, added.MatchType, added.Value)
	} else {
		fmt.Printf("Added condition to entry %d: %s:%s:%s\n", entryID, cond.Field, cond.MatchType, cond.Value)
	}
	return nil
}

func runContextTriggerRemove(_ context.Context, _ *PipelineCommandDeps, entryID, condID int32) error {
	// RemoveTenantContextCondition RPC is not yet in the proto.
	// The condition can be removed by deleting and recreating the entry, or by waiting for backend support.
	return fmt.Errorf("trigger remove is not yet supported by the backend (entry %d, condition %d): "+
		"use 'penf context show %d' to view conditions, then remove and recreate the entry if needed",
		entryID, condID, entryID)
}

// parseTenantContextCondition parses a "field:match_type:value" string.
func parseTenantContextCondition(raw string) (*pipelinev1.TenantContextCondition, error) {
	parts := strings.SplitN(raw, ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid condition %q: expected format field:match_type:value", raw)
	}
	return &pipelinev1.TenantContextCondition{
		Field:     parts[0],
		MatchType: parts[1],
		Value:     parts[2],
	}, nil
}

// ===========================================================================
// Output formatting
// ===========================================================================

func outputContextListHuman(entries []*pipelinev1.TenantContextEntry) error {
	if len(entries) == 0 {
		fmt.Println("No context entries found.")
		return nil
	}

	fmt.Printf("Context Entries (%d):\n\n", len(entries))
	fmt.Println("  ID    CATEGORY        LABEL                           CONDITIONS  FLAGS")
	fmt.Println("  ----  --------        -----                           ----------  -----")

	for _, e := range entries {
		flags := ""
		if e.AlwaysInject {
			flags = "always-inject"
		}
		if !e.Active {
			if flags != "" {
				flags += " "
			}
			flags += "inactive"
		}

		fmt.Printf("  %-4d  %-14s  %-30s  %-10d  %s\n",
			e.Id,
			truncateContextStr(e.Category, 14),
			truncateContextStr(e.Label, 30),
			len(e.Conditions),
			flags,
		)
	}

	fmt.Println()
	return nil
}

func outputContextShowHuman(entry *pipelinev1.TenantContextEntry) error {
	if entry == nil {
		fmt.Println("Entry not found.")
		return nil
	}

	fmt.Printf("Context Entry #%d\n", entry.Id)
	fmt.Printf("  Category:     %s\n", entry.Category)
	fmt.Printf("  Label:        %s\n", entry.Label)
	fmt.Printf("  Active:       %v\n", entry.Active)
	if entry.AlwaysInject {
		fmt.Printf("  Always-Inject: true\n")
	}
	if entry.DetailsJson != "" && entry.DetailsJson != "{}" {
		fmt.Printf("  Details:      %s\n", entry.DetailsJson)
	}

	if len(entry.Conditions) > 0 {
		fmt.Printf("\n  Trigger Conditions (%d):\n", len(entry.Conditions))
		for _, c := range entry.Conditions {
			fmt.Printf("    [%d] %s:%s:%s\n", c.Id, c.Field, c.MatchType, c.Value)
		}
	} else {
		fmt.Println("\n  Trigger Conditions: none")
	}

	fmt.Println()
	return nil
}

func truncateContextStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
