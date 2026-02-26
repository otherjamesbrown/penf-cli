// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

func newPipelineRulesCmd(deps *PipelineCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Inspect and manage classification rules",
		Long: `Inspect and manage content classification rules.

Classification rules determine content type, subtype, and notification source
based on metadata field matching (e.g., from_address, subject). Rules are
evaluated in priority order (lowest number = highest priority).

Commands:
  list  - List all rules in priority order
  show  - Show a rule with its match conditions
  test  - Run the rule engine against a content item

Examples:
  # List all rules
  penf pipeline rules list

  # Show a specific rule
  penf pipeline rules show jira

  # Test which rule matches a content item
  penf pipeline rules test em-nGRm5RLf`,
	}

	cmd.AddCommand(newPipelineRulesListCmd(deps))
	cmd.AddCommand(newPipelineRulesShowCmd(deps))
	cmd.AddCommand(newPipelineRulesTestCmd(deps))

	// Default action is list
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPipelineRulesList(cmd.Context(), deps, "text")
	}

	return cmd
}

func newPipelineRulesListCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all classification rules",
		Long: `List all classification rules in priority order.

Shows rule name, scope, resulting content type/subtype, and notification source.

Examples:
  # List all rules
  penf pipeline rules list

  # Output as JSON
  penf pipeline rules list -o json`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineRulesList(cmd.Context(), deps, outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func newPipelineRulesShowCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "show <rule-name>",
		Short: "Show a classification rule with its match conditions",
		Long: `Show detailed information about a classification rule, including
all match conditions.

Examples:
  # Show the jira notification rule
  penf pipeline rules show jira

  # Output as JSON
  penf pipeline rules show jira -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineRulesShow(cmd.Context(), deps, args[0], outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func newPipelineRulesTestCmd(deps *PipelineCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "test <content-id>",
		Short: "Test classification rules against a content item",
		Long: `Run the classification rule engine against a content item to see
which rule matches and what classification result it produces.

Examples:
  # Test a content item
  penf pipeline rules test em-nGRm5RLf

  # Output as JSON
  penf pipeline rules test em-nGRm5RLf -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineRulesTest(cmd.Context(), deps, args[0], outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineRulesList(ctx context.Context, deps *PipelineCommandDeps, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.TenantID
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.ListClassificationRules(ctx, &pipelinev1.ListClassificationRulesRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return fmt.Errorf("listing classification rules: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Rules)
	}

	return outputRulesListHuman(resp.Rules)
}

func runPipelineRulesShow(ctx context.Context, deps *PipelineCommandDeps, name string, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.TenantID
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.GetClassificationRule(ctx, &pipelinev1.GetClassificationRuleRequest{
		TenantId: tenantID,
		Name:     name,
	})
	if err != nil {
		return fmt.Errorf("getting classification rule: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Rule)
	}

	return outputRuleShowHuman(resp.Rule)
}

func runPipelineRulesTest(ctx context.Context, deps *PipelineCommandDeps, contentID string, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	tenantID := cfg.TenantID
	if tenantID == "" {
		return fmt.Errorf("tenant ID required: set via 'penf config set tenant_id <id>'")
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.TestClassificationRule(ctx, &pipelinev1.TestClassificationRuleRequest{
		TenantId:  tenantID,
		ContentId: contentID,
	})
	if err != nil {
		return fmt.Errorf("testing classification rule: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	return outputRuleTestHuman(resp)
}

func outputRulesListHuman(rules []*pipelinev1.ClassificationRule) error {
	if len(rules) == 0 {
		fmt.Println("No classification rules found.")
		return nil
	}

	fmt.Printf("Classification Rules (%d):\n\n", len(rules))
	fmt.Println("  PRIORITY  NAME              SCOPE    TYPE      SUBTYPE        SOURCE")
	fmt.Println("  --------  ----              -----    ----      -------        ------")

	for _, rule := range rules {
		source := rule.NotificationSource
		if source == "" {
			source = "-"
		}
		subtype := rule.ContentSubtype
		if subtype == "" {
			subtype = "-"
		}

		fmt.Printf("  %-8d  %-16s  %-7s  %-8s  %-13s  %s\n",
			rule.Priority,
			truncate(rule.Name, 16),
			rule.Scope,
			rule.ContentType,
			subtype,
			source)
	}

	fmt.Println()
	return nil
}

func outputRuleShowHuman(rule *pipelinev1.ClassificationRule) error {
	if rule == nil {
		fmt.Println("Rule not found.")
		return nil
	}

	fmt.Printf("Rule: %s\n", rule.Name)
	fmt.Printf("  Priority:    %d\n", rule.Priority)
	fmt.Printf("  Scope:       %s\n", rule.Scope)
	fmt.Printf("  Type:        %s\n", rule.ContentType)
	if rule.ContentSubtype != "" {
		fmt.Printf("  Subtype:     %s\n", rule.ContentSubtype)
	}
	if rule.NotificationSource != "" {
		fmt.Printf("  Source:      %s\n", rule.NotificationSource)
	}
	fmt.Printf("  Active:      %t\n", rule.Active)

	if len(rule.Conditions) > 0 {
		fmt.Println()
		fmt.Println("  Match Conditions (OR):")
		for _, cond := range rule.Conditions {
			fmt.Printf("    %-16s %-10s %s\n", cond.Field, cond.Operator, cond.Value)
		}
	}

	fmt.Println()
	return nil
}

func outputRuleTestHuman(resp *pipelinev1.TestClassificationRuleResponse) error {
	fmt.Println("Classification Result:")

	if resp.MatchedRule != nil {
		fmt.Printf("  Matched Rule:  %s (priority %d)\n", resp.MatchedRule.Name, resp.MatchedRule.Priority)
	} else {
		fmt.Println("  Matched Rule:  (none — default classification)")
	}

	fmt.Printf("  Type:          %s\n", displayOrDash(resp.ContentType))
	fmt.Printf("  Subtype:       %s\n", displayOrDash(resp.ContentSubtype))
	if resp.NotificationSource != "" {
		fmt.Printf("  Source:        %s\n", resp.NotificationSource)
	}

	fmt.Println()
	fmt.Printf("  Evaluated: %d rules", resp.RulesEvaluated)
	if resp.MatchedCondition != "" {
		fmt.Printf(", matched on condition: %s", resp.MatchedCondition)
	}
	fmt.Println()
	fmt.Println()

	return nil
}

func displayOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return strings.ToUpper(s)
}
