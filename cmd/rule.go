// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

// NewRuleCommand creates the `penf rule` command tree.
func NewRuleCommand(deps interface{}) *cobra.Command {
	var pipelineDeps *PipelineCommandDeps
	if d, ok := deps.(*PipelineCommandDeps); ok && d != nil {
		pipelineDeps = d
	} else {
		pipelineDeps = DefaultPipelineDeps()
	}
	return newRuleCmd(pipelineDeps)
}

func newRuleCmd(deps *PipelineCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rule",
		Short: "Manage automation rules",
		Long: `Manage automation rules — composable trigger → selector → skill → output pipelines.

Automation rules define when to run (trigger), what data to gather (selector),
what LLM instructions to apply (skill), and where to send the result (output).

Trigger types:
  cron    Run on a schedule (e.g., "0 8 * * MON-FRI")
  event   Fire when pipeline-processed content matches a pattern
  manual  Triggered explicitly via 'penf rule run'

Examples:
  # List all rules
  penf rule list

  # Create a morning summary rule
  penf rule create healthcare-morning \
    --trigger cron --cron "0 8 * * MON-FRI" \
    --selector '{"scope":"query","query":"project:Healthcare","window":"24h"}' \
    --skill healthcare-daily-summary \
    --output email --output-to james@brown.chat

  # Run a rule manually
  penf rule run healthcare-morning

  # Show what a rule would do without executing
  penf rule run healthcare-morning --dry-run

  # View execution history
  penf rule history healthcare-morning`,
		Aliases: []string{"rules"},
	}

	cmd.AddCommand(newRuleListCmd(deps))
	cmd.AddCommand(newRuleShowCmd(deps))
	cmd.AddCommand(newRuleCreateCmd(deps))
	cmd.AddCommand(newRuleUpdateCmd(deps))
	cmd.AddCommand(newRuleEnableCmd(deps))
	cmd.AddCommand(newRuleDisableCmd(deps))
	cmd.AddCommand(newRuleDeleteCmd(deps))
	cmd.AddCommand(newRuleRunCmd(deps))
	cmd.AddCommand(newRuleHistoryCmd(deps))

	// Default action is list.
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runRuleList(cmd.Context(), deps)
	}

	return cmd
}

// ==================== list ====================

func newRuleListCmd(deps *PipelineCommandDeps) *cobra.Command {
	var enabledOnly bool

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all automation rules",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleList(cmd.Context(), deps)
		},
	}

	cmd.Flags().BoolVar(&enabledOnly, "enabled-only", false, "Only show enabled rules")

	return cmd
}

func runRuleList(ctx context.Context, deps *PipelineCommandDeps) error {
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

	resp, err := client.ListAutomationRules(ctx, &pipelinev1.ListAutomationRulesRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return fmt.Errorf("listing automation rules: %w", err)
	}

	if len(resp.Rules) == 0 {
		fmt.Println("No automation rules found.")
		return nil
	}

	fmt.Printf("Automation Rules (%d):\n\n", len(resp.Rules))
	fmt.Printf("  %-30s %-8s %-12s %-30s %s\n", "NAME", "ENABLED", "TRIGGER", "SKILL", "LAST STATUS")
	fmt.Printf("  %-30s %-8s %-12s %-30s %s\n",
		strings.Repeat("-", 30), strings.Repeat("-", 7), strings.Repeat("-", 11),
		strings.Repeat("-", 29), strings.Repeat("-", 10))

	for _, r := range resp.Rules {
		enabled := "yes"
		if !r.Enabled {
			enabled = "no"
		}
		fmt.Printf("  %-30s %-8s %-12s %-30s\n",
			rulesTruncate(r.Name, 30),
			enabled,
			rulesTruncate(r.TriggerType, 12),
			rulesTruncate(r.SkillName, 30),
		)
	}

	fmt.Println()
	return nil
}

// ==================== show ====================

func newRuleShowCmd(deps *PipelineCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show full details of an automation rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleShow(cmd.Context(), deps, args[0])
		},
	}
}

func runRuleShow(ctx context.Context, deps *PipelineCommandDeps, name string) error {
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

	ruleID, err := resolveRuleID(ctx, client, tenantID, name)
	if err != nil {
		return err
	}

	resp, err := client.GetAutomationRule(ctx, &pipelinev1.GetAutomationRuleRequest{
		TenantId: tenantID,
		Name:     ruleID,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("rule not found: %s", name)
		}
		return fmt.Errorf("getting automation rule: %w", err)
	}

	r := resp.Rule

	fmt.Printf("Rule: %s\n", r.Name)
	if r.Description != "" {
		fmt.Printf("  Description:    %s\n", r.Description)
	}
	fmt.Printf("  Enabled:        %t\n", r.Enabled)
	fmt.Printf("  Trigger Type:   %s\n", r.TriggerType)
	fmt.Printf("  Skill:          %s\n", r.SkillName)
	if r.CreatedAt != nil {
		fmt.Printf("  Created:        %s\n", r.CreatedAt.AsTime().Format("2006-01-02 15:04"))
	}

	if r.TriggerConfig != "" && r.TriggerConfig != "{}" {
		fmt.Printf("\n  Trigger Config:\n")
		printJSONIndented(r.TriggerConfig, "    ")
	}

	if r.SelectorConfig != "" && r.SelectorConfig != "{}" {
		fmt.Printf("\n  Selector Config:\n")
		printJSONIndented(r.SelectorConfig, "    ")
	}

	if r.OutputConfig != "" && r.OutputConfig != "{}" {
		fmt.Printf("\n  Output Config:\n")
		printJSONIndented(r.OutputConfig, "    ")
	}

	// Show recent history.
	histResp, err := client.ListAutomationRuleExecutions(ctx, &pipelinev1.ListAutomationRuleExecutionsRequest{
		TenantId: tenantID,
		Name:     ruleID,
		Limit:    5,
	})
	if err == nil && len(histResp.Executions) > 0 {
		fmt.Printf("\n  Recent Executions:\n")
		for _, ex := range histResp.Executions {
			started := "-"
			if ex.StartedAt != nil {
				started = ex.StartedAt.AsTime().Format("2006-01-02 15:04")
			}
			fmt.Printf("    %-20s %-12s items=%-4d %s\n",
				started,
				ex.Status,
				ex.ItemsSelected,
				rulesTruncate(ex.Error, 40),
			)
		}
	}

	fmt.Println()
	return nil
}

// ==================== create ====================

func newRuleCreateCmd(deps *PipelineCommandDeps) *cobra.Command {
	var (
		triggerType    string
		cronExpr       string
		selectorJSON   string
		skillName      string
		outputType     string
		outputTo       string
		outputConfig   string
		description    string
		disabled       bool
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new automation rule",
		Long: `Create a new automation rule.

Examples:
  # Cron-triggered rule with email output
  penf rule create healthcare-morning \
    --trigger cron --cron "0 8 * * MON-FRI" \
    --selector '{"scope":"query","query":"project:Healthcare","window":"24h"}' \
    --skill healthcare-daily-summary \
    --output email --output-to james@brown.chat

  # Manual rule with explicit JSON configs
  penf rule create my-rule \
    --trigger manual \
    --selector '{"scope":"query","query":"content_subtype:NEWSLETTER","window":"7d"}' \
    --skill newsletter-rollup \
    --output-config '{"channels":[{"type":"store"}]}'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleCreate(ctx(cmd), deps, args[0],
				triggerType, cronExpr, selectorJSON,
				skillName, outputType, outputTo, outputConfig,
				description, !disabled)
		},
	}

	cmd.Flags().StringVar(&triggerType, "trigger", "manual", "Trigger type: cron, event, manual")
	cmd.Flags().StringVar(&cronExpr, "cron", "", "Cron expression (for --trigger cron, e.g. '0 8 * * *')")
	cmd.Flags().StringVar(&selectorJSON, "selector", "", "Selector config as JSON (e.g. '{\"scope\":\"query\",\"query\":\"...\",\"window\":\"24h\"}')")
	cmd.Flags().StringVar(&skillName, "skill", "", "Skill name (references skills/<name>.md)")
	cmd.Flags().StringVar(&outputType, "output", "", "Output channel type: email, store")
	cmd.Flags().StringVar(&outputTo, "output-to", "", "Output destination (email address for --output email)")
	cmd.Flags().StringVar(&outputConfig, "output-config", "", "Full output config as JSON (overrides --output/--output-to)")
	cmd.Flags().StringVar(&description, "description", "", "Human-readable description")
	cmd.Flags().BoolVar(&disabled, "disabled", false, "Create rule as disabled")

	_ = cmd.MarkFlagRequired("skill")

	return cmd
}

func runRuleCreate(ctx context.Context, deps *PipelineCommandDeps, name string,
	triggerType, cronExpr, selectorJSON string,
	skillName, outputType, outputTo, outputConfigStr string,
	description string, enabled bool,
) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.EffectiveTenantID()
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	// Build trigger config.
	triggerConfig, err := buildTriggerConfig(triggerType, cronExpr)
	if err != nil {
		return err
	}

	// Build selector config.
	if selectorJSON == "" {
		selectorJSON = `{"scope":"manual"}`
	}
	if err := validateJSON(selectorJSON); err != nil {
		return fmt.Errorf("invalid --selector JSON: %w", err)
	}

	// Build output config.
	outConfig, err := buildOutputConfig(outputType, outputTo, outputConfigStr)
	if err != nil {
		return err
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.CreateAutomationRule(ctx, &pipelinev1.CreateAutomationRuleRequest{
		TenantId:       tenantID,
		Name:           name,
		Description:    description,
		TriggerType:    triggerType,
		TriggerConfig:  triggerConfig,
		SelectorConfig: selectorJSON,
		SkillName:      skillName,
		OutputConfig:   outConfig,
		Enabled:        enabled,
	})
	if err != nil {
		return fmt.Errorf("creating automation rule: %w", err)
	}

	r := resp.Rule
	fmt.Printf("Created rule: %s\n", r.Name)
	fmt.Printf("  Trigger: %s\n", r.TriggerType)
	fmt.Printf("  Skill:   %s\n", r.SkillName)
	fmt.Printf("  Enabled: %t\n", r.Enabled)
	return nil
}

// ==================== update ====================

func newRuleUpdateCmd(deps *PipelineCommandDeps) *cobra.Command {
	var (
		cronExpr     string
		skillName    string
		selectorJSON string
		outputConfig string
		description  string
		enable       bool
		disable      bool
	)

	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update fields on an existing automation rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleUpdate(ctx(cmd), deps, args[0],
				cronExpr, skillName, selectorJSON, outputConfig,
				description, enable, disable)
		},
	}

	cmd.Flags().StringVar(&cronExpr, "cron", "", "New cron expression")
	cmd.Flags().StringVar(&skillName, "skill", "", "New skill name")
	cmd.Flags().StringVar(&selectorJSON, "selector", "", "New selector config JSON")
	cmd.Flags().StringVar(&outputConfig, "output-config", "", "New output config JSON")
	cmd.Flags().StringVar(&description, "description", "", "New description")
	cmd.Flags().BoolVar(&enable, "enable", false, "Enable the rule")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable the rule")

	return cmd
}

func runRuleUpdate(ctx context.Context, deps *PipelineCommandDeps, name string,
	cronExpr, skillName, selectorJSON, outputConfig, description string,
	enable, disable bool,
) error {
	if enable && disable {
		return fmt.Errorf("cannot use --enable and --disable together")
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

	// Build trigger config if cron was updated.
	triggerConfigJSON := ""
	if cronExpr != "" {
		triggerConfigJSON, err = buildTriggerConfig("cron", cronExpr)
		if err != nil {
			return err
		}
	}

	if selectorJSON != "" {
		if err := validateJSON(selectorJSON); err != nil {
			return fmt.Errorf("invalid --selector JSON: %w", err)
		}
	}

	if outputConfig != "" {
		if err := validateJSON(outputConfig); err != nil {
			return fmt.Errorf("invalid --output-config JSON: %w", err)
		}
	}

	req := &pipelinev1.UpdateAutomationRuleRequest{
		TenantId:       tenantID,
		Name:           name,
		Description:    description,
		TriggerConfig:  triggerConfigJSON,
		SelectorConfig: selectorJSON,
		SkillName:      skillName,
		OutputConfig:   outputConfig,
	}

	if enable {
		req.SetEnabled = true
		req.Enabled = true
	} else if disable {
		req.SetEnabled = true
		req.Enabled = false
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	ruleID, err := resolveRuleID(ctx, client, tenantID, name)
	if err != nil {
		return err
	}
	req.Name = ruleID

	resp, err := client.UpdateAutomationRule(ctx, req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("rule not found: %s", name)
		}
		return fmt.Errorf("updating automation rule: %w", err)
	}

	r := resp.Rule
	fmt.Printf("Updated rule: %s (enabled: %t)\n", r.Name, r.Enabled)
	return nil
}

// ==================== enable / disable ====================

func newRuleEnableCmd(deps *PipelineCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable an automation rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleSetEnabled(ctx(cmd), deps, args[0], true)
		},
	}
}

func newRuleDisableCmd(deps *PipelineCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable an automation rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleSetEnabled(ctx(cmd), deps, args[0], false)
		},
	}
}

func runRuleSetEnabled(ctx context.Context, deps *PipelineCommandDeps, name string, enabled bool) error {
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

	ruleID, err := resolveRuleID(ctx, client, tenantID, name)
	if err != nil {
		return err
	}

	_, err = client.UpdateAutomationRule(ctx, &pipelinev1.UpdateAutomationRuleRequest{
		TenantId:   tenantID,
		Name:       ruleID,
		SetEnabled: true,
		Enabled:    enabled,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("rule not found: %s", name)
		}
		return fmt.Errorf("updating automation rule: %w", err)
	}

	state := "enabled"
	if !enabled {
		state = "disabled"
	}
	fmt.Printf("Rule %s: %s\n", name, state)
	return nil
}

// ==================== delete ====================

func newRuleDeleteCmd(deps *PipelineCommandDeps) *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an automation rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleDelete(ctx(cmd), deps, args[0], confirm)
		},
	}

	cmd.Flags().BoolVar(&confirm, "confirm", false, "Skip confirmation prompt")

	return cmd
}

func runRuleDelete(ctx context.Context, deps *PipelineCommandDeps, name string, confirmed bool) error {
	if !confirmed {
		fmt.Printf("Delete automation rule %q? [y/N] ", name)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
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

	ruleID, err := resolveRuleID(ctx, client, tenantID, name)
	if err != nil {
		return err
	}

	_, err = client.DeleteAutomationRule(ctx, &pipelinev1.DeleteAutomationRuleRequest{
		TenantId: tenantID,
		Name:     ruleID,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("rule not found: %s", name)
		}
		return fmt.Errorf("deleting automation rule: %w", err)
	}

	fmt.Printf("Deleted rule: %s\n", name)
	return nil
}

// ==================== run ====================

func newRuleRunCmd(deps *PipelineCommandDeps) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Trigger manual execution of an automation rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleRun(ctx(cmd), deps, args[0], dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be selected without executing")

	return cmd
}

func runRuleRun(ctx context.Context, deps *PipelineCommandDeps, name string, dryRun bool) error {
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

	ruleID, err := resolveRuleID(ctx, client, tenantID, name)
	if err != nil {
		return err
	}

	resp, err := client.RunAutomationRule(ctx, &pipelinev1.RunAutomationRuleRequest{
		TenantId: tenantID,
		Name:     ruleID,
		DryRun:   dryRun,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("rule not found: %s", name)
		}
		return fmt.Errorf("running automation rule: %w", err)
	}

	if resp.DryRun {
		fmt.Printf("Dry run for rule: %s\n\n", name)
		if resp.DryRunSummary != "" {
			fmt.Println(resp.DryRunSummary)
		} else {
			fmt.Println("(no dry-run summary returned)")
		}
	} else {
		fmt.Printf("Rule %s started (workflow: %s)\n", name, resp.WorkflowId)
	}
	return nil
}

// ==================== history ====================

func newRuleHistoryCmd(deps *PipelineCommandDeps) *cobra.Command {
	var last int32

	cmd := &cobra.Command{
		Use:   "history <name>",
		Short: "Show execution history for an automation rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleHistory(ctx(cmd), deps, args[0], last)
		},
	}

	cmd.Flags().Int32Var(&last, "last", 10, "Number of executions to show")

	return cmd
}

func runRuleHistory(ctx context.Context, deps *PipelineCommandDeps, name string, limit int32) error {
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

	ruleID, err := resolveRuleID(ctx, client, tenantID, name)
	if err != nil {
		return err
	}

	resp, err := client.ListAutomationRuleExecutions(ctx, &pipelinev1.ListAutomationRuleExecutionsRequest{
		TenantId: tenantID,
		Name:     ruleID,
		Limit:    limit,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("rule not found: %s", name)
		}
		return fmt.Errorf("listing rule executions: %w", err)
	}

	if len(resp.Executions) == 0 {
		fmt.Printf("No execution history for rule: %s\n", name)
		return nil
	}

	fmt.Printf("Execution history for %s (%d entries):\n\n", name, len(resp.Executions))
	fmt.Printf("  %-20s %-12s %-8s %-8s %s\n", "STARTED", "STATUS", "ITEMS", "TOKENS", "ERROR")
	fmt.Printf("  %-20s %-12s %-8s %-8s %s\n",
		strings.Repeat("-", 19), strings.Repeat("-", 11),
		strings.Repeat("-", 7), strings.Repeat("-", 7), strings.Repeat("-", 20))

	for _, ex := range resp.Executions {
		started := "-"
		if ex.StartedAt != nil {
			started = ex.StartedAt.AsTime().Format("2006-01-02 15:04")
		}
		errStr := ""
		if ex.Error != "" {
			errStr = rulesTruncate(ex.Error, 40)
		}
		fmt.Printf("  %-20s %-12s %-8d %-8d %s\n",
			started,
			ex.Status,
			ex.ItemsSelected,
			ex.SkillTokensUsed,
			errStr,
		)
	}

	fmt.Println()
	return nil
}

// ==================== helpers ====================

// ctx extracts the context from a cobra command.
func ctx(cmd *cobra.Command) context.Context {
	return cmd.Context()
}

// resolveRuleID resolves a name-or-UUID to the rule's UUID.
// If nameOrID is already a valid UUID it is returned unchanged.
// Otherwise, ListAutomationRules is called and the matching rule's ID is returned.
func resolveRuleID(gctx context.Context, client pipelinev1.PipelineServiceClient, tenantID, nameOrID string) (string, error) {
	if _, err := uuid.Parse(nameOrID); err == nil {
		return nameOrID, nil
	}
	resp, err := client.ListAutomationRules(gctx, &pipelinev1.ListAutomationRulesRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return "", fmt.Errorf("listing rules to resolve name: %w", err)
	}
	for _, r := range resp.Rules {
		if r.Name == nameOrID {
			return r.Id, nil
		}
	}
	return "", fmt.Errorf("rule not found: %s", nameOrID)
}

// buildTriggerConfig builds the trigger config JSON from convenience flags.
func buildTriggerConfig(triggerType, cronExpr string) (string, error) {
	switch triggerType {
	case "cron":
		if cronExpr == "" {
			return "", fmt.Errorf("--cron is required for --trigger cron")
		}
		b, _ := json.Marshal(map[string]string{"cron": cronExpr})
		return string(b), nil
	case "event":
		return `{}`, nil
	case "manual":
		return `{}`, nil
	default:
		return "", fmt.Errorf("unknown trigger type %q: must be cron, event, or manual", triggerType)
	}
}

// buildOutputConfig builds output config JSON from convenience flags.
func buildOutputConfig(outputType, outputTo, outputConfigStr string) (string, error) {
	if outputConfigStr != "" {
		if err := validateJSON(outputConfigStr); err != nil {
			return "", fmt.Errorf("invalid --output-config JSON: %w", err)
		}
		return outputConfigStr, nil
	}

	if outputType == "" {
		return `{"channels":[{"type":"store"}]}`, nil
	}

	channel := map[string]string{"type": outputType}
	if outputTo != "" {
		switch outputType {
		case "email":
			channel["to"] = outputTo
		default:
			channel["to"] = outputTo
		}
	}

	out := map[string]interface{}{
		"channels": []interface{}{channel},
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

// validateJSON returns an error if s is not valid JSON.
func validateJSON(s string) error {
	var v interface{}
	return json.Unmarshal([]byte(s), &v)
}

// printJSONIndented pretty-prints a JSON string with the given indentation.
func printJSONIndented(s, indent string) {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		fmt.Printf("%s%s\n", indent, s)
		return
	}
	b, _ := json.MarshalIndent(v, indent, "  ")
	fmt.Printf("%s%s\n", indent, string(b))
}

// rulesTruncate truncates a string to maxLen characters.
func rulesTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

