package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	instructionv1 "github.com/otherjamesbrown/penf-cli/api/proto/instruction/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

var (
	instructionOutput   string
	instructionProject  string
	instructionPriority string
	instructionEnabled  bool
	instructionLimit    int
)

// InstructionCommandDeps holds dependencies for instruction commands.
type InstructionCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultInstructionDeps returns the default dependencies.
func DefaultInstructionDeps() *InstructionCommandDeps {
	return &InstructionCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewInstructionCommand creates the root instruction command with all subcommands.
func NewInstructionCommand(deps *InstructionCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultInstructionDeps()
	}

	cmd := &cobra.Command{
		Use:   "instruction",
		Short: "Manage watch instructions — natural-language rules evaluated against incoming content",
		Long: `Watch instructions are natural-language rules evaluated against all incoming
content. When content matches an instruction, a match is recorded with
confidence and explanation. Matches feed into daily digests and alerts.`,
		Aliases: []string{"inst", "instructions"},
	}

	cmd.PersistentFlags().StringVarP(&instructionOutput, "output", "o", "", "Output format: text, json, yaml")

	cmd.AddCommand(newInstructionListCommand(deps))
	cmd.AddCommand(newInstructionShowCommand(deps))
	cmd.AddCommand(newInstructionAddCommand(deps))
	cmd.AddCommand(newInstructionEditCommand(deps))
	cmd.AddCommand(newInstructionDeleteCommand(deps))
	cmd.AddCommand(newInstructionEnableCommand(deps))
	cmd.AddCommand(newInstructionDisableCommand(deps))
	cmd.AddCommand(newInstructionHistoryCommand(deps))

	return cmd
}

// ==================== list ====================

func newInstructionListCommand(deps *InstructionCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List instructions",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstructionList(cmd.Context(), deps)
		},
	}
	cmd.Flags().StringVar(&instructionProject, "project", "", "Filter by project name")
	cmd.Flags().StringVar(&instructionPriority, "priority", "", "Filter by priority (critical, high, normal, low)")
	cmd.Flags().BoolVar(&instructionEnabled, "enabled", false, "Show only enabled instructions")
	cmd.Flags().IntVarP(&instructionLimit, "limit", "l", 50, "Maximum number of results")
	return cmd
}

func runInstructionList(ctx context.Context, deps *InstructionCommandDeps) error {
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

	client := instructionv1.NewInstructionServiceClient(conn)
	tenantID, err := getTenantIDForInstruction(deps)
	if err != nil {
		return err
	}

	resp, err := client.ListInstructions(ctx, &instructionv1.ListInstructionsRequest{
		TenantId:    tenantID,
		EnabledOnly: instructionEnabled,
		ProjectName: instructionProject,
		Priority:    instructionPriority,
		Limit:       int32(instructionLimit),
	})
	if err != nil {
		return fmt.Errorf("listing instructions: %w", err)
	}

	return outputInstructionList(cfg, resp.Instructions)
}

// ==================== show ====================

func newInstructionShowCommand(deps *InstructionCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show instruction details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstructionShow(cmd.Context(), deps, args[0])
		},
	}
}

func runInstructionShow(ctx context.Context, deps *InstructionCommandDeps, idStr string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid instruction ID: %s", idStr)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := instructionv1.NewInstructionServiceClient(conn)
	tenantID, err := getTenantIDForInstruction(deps)
	if err != nil {
		return err
	}

	resp, err := client.GetInstruction(ctx, &instructionv1.GetInstructionRequest{
		TenantId: tenantID,
		Id:       id,
	})
	if err != nil {
		return fmt.Errorf("getting instruction: %w", err)
	}

	return outputInstructionDetail(cfg, resp.Instruction)
}

// ==================== add ====================

func newInstructionAddCommand(deps *InstructionCommandDeps) *cobra.Command {
	var name, instruction, priority, modelHint string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new instruction",
		Long: `Create a watch instruction — a natural-language rule that will be evaluated
against all incoming content.

Examples:
  penf instruction add --name "Coordination Issues" \
    --instruction "Alert me when emails discuss poor coordination" \
    --project MTC --priority high

  penf instruction add --name "Outage Reports" \
    --instruction "Flag emails about production outages or service failures"`,
		Aliases: []string{"create"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstructionAdd(cmd.Context(), deps, name, instruction, priority, modelHint)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Short label for the instruction (required)")
	cmd.Flags().StringVar(&instruction, "instruction", "", "Natural language rule text (required)")
	cmd.Flags().StringVar(&instructionProject, "project", "", "Scope to a project (by name)")
	cmd.Flags().StringVar(&priority, "priority", "normal", "Priority: critical, high, normal, low")
	cmd.Flags().StringVar(&modelHint, "model-hint", "fast", "Model tier hint: fast, standard, premium")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("instruction")
	return cmd
}

func runInstructionAdd(ctx context.Context, deps *InstructionCommandDeps, name, instruction, priority, modelHint string) error {
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

	client := instructionv1.NewInstructionServiceClient(conn)
	tenantID, err := getTenantIDForInstruction(deps)
	if err != nil {
		return err
	}

	resp, err := client.CreateInstruction(ctx, &instructionv1.CreateInstructionRequest{
		TenantId:    tenantID,
		Name:        name,
		Instruction: instruction,
		ProjectName: instructionProject,
		Priority:    priority,
		ModelHint:   modelHint,
	})
	if err != nil {
		return fmt.Errorf("creating instruction: %w", err)
	}

	format := getInstructionOutputFormat(cfg)
	if format == config.OutputFormatText {
		inst := resp.Instruction
		fmt.Printf("Created instruction %d: %s\n", inst.Id, inst.Name)
		if inst.ProjectName != "" {
			fmt.Printf("  Project: %s\n", inst.ProjectName)
		}
		fmt.Printf("  Priority: %s  Enabled: %v\n", inst.Priority, inst.Enabled)
		return nil
	}
	return outputInstructionDetail(cfg, resp.Instruction)
}

// ==================== edit ====================

func newInstructionEditCommand(deps *InstructionCommandDeps) *cobra.Command {
	var name, instruction, priority, modelHint string

	cmd := &cobra.Command{
		Use:     "edit <id>",
		Short:   "Edit an instruction",
		Aliases: []string{"update"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstructionEdit(cmd.Context(), deps, args[0], name, instruction, priority, modelHint)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New name")
	cmd.Flags().StringVar(&instruction, "instruction", "", "New rule text")
	cmd.Flags().StringVar(&priority, "priority", "", "New priority")
	cmd.Flags().StringVar(&modelHint, "model-hint", "", "New model tier hint")
	return cmd
}

func runInstructionEdit(ctx context.Context, deps *InstructionCommandDeps, idStr, name, instruction, priority, modelHint string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid instruction ID: %s", idStr)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := instructionv1.NewInstructionServiceClient(conn)
	tenantID, err := getTenantIDForInstruction(deps)
	if err != nil {
		return err
	}

	resp, err := client.UpdateInstruction(ctx, &instructionv1.UpdateInstructionRequest{
		TenantId:    tenantID,
		Id:          id,
		Name:        name,
		Instruction: instruction,
		Priority:    priority,
		ModelHint:   modelHint,
	})
	if err != nil {
		return fmt.Errorf("updating instruction: %w", err)
	}

	format := getInstructionOutputFormat(cfg)
	if format == config.OutputFormatText {
		fmt.Printf("Updated instruction %d: %s (v%d)\n", resp.Instruction.Id, resp.Instruction.Name, resp.Instruction.Version)
		return nil
	}
	return outputInstructionDetail(cfg, resp.Instruction)
}

// ==================== delete ====================

func newInstructionDeleteCommand(deps *InstructionCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete an instruction",
		Aliases: []string{"rm", "remove"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstructionDelete(cmd.Context(), deps, args[0])
		},
	}
}

func runInstructionDelete(ctx context.Context, deps *InstructionCommandDeps, idStr string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid instruction ID: %s", idStr)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := instructionv1.NewInstructionServiceClient(conn)
	tenantID, err := getTenantIDForInstruction(deps)
	if err != nil {
		return err
	}

	_, err = client.DeleteInstruction(ctx, &instructionv1.DeleteInstructionRequest{
		TenantId: tenantID,
		Id:       id,
	})
	if err != nil {
		return fmt.Errorf("deleting instruction: %w", err)
	}

	fmt.Printf("Deleted instruction %d.\n", id)
	return nil
}

// ==================== enable ====================

func newInstructionEnableCommand(deps *InstructionCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable an instruction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstructionSetEnabled(cmd.Context(), deps, args[0], true)
		},
	}
}

// ==================== disable ====================

func newInstructionDisableCommand(deps *InstructionCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable an instruction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstructionSetEnabled(cmd.Context(), deps, args[0], false)
		},
	}
}

func runInstructionSetEnabled(ctx context.Context, deps *InstructionCommandDeps, idStr string, enable bool) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid instruction ID: %s", idStr)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := instructionv1.NewInstructionServiceClient(conn)
	tenantID, err := getTenantIDForInstruction(deps)
	if err != nil {
		return err
	}

	if enable {
		resp, err := client.EnableInstruction(ctx, &instructionv1.EnableInstructionRequest{
			TenantId: tenantID,
			Id:       id,
		})
		if err != nil {
			return fmt.Errorf("enabling instruction: %w", err)
		}
		fmt.Printf("Enabled instruction %d: %s\n", resp.Instruction.Id, resp.Instruction.Name)
	} else {
		resp, err := client.DisableInstruction(ctx, &instructionv1.DisableInstructionRequest{
			TenantId: tenantID,
			Id:       id,
		})
		if err != nil {
			return fmt.Errorf("disabling instruction: %w", err)
		}
		fmt.Printf("Disabled instruction %d: %s\n", resp.Instruction.Id, resp.Instruction.Name)
	}

	return nil
}

// ==================== history ====================

func newInstructionHistoryCommand(deps *InstructionCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <id>",
		Short: "Show match history for an instruction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstructionHistory(cmd.Context(), deps, args[0])
		},
	}
	cmd.Flags().IntVarP(&instructionLimit, "limit", "l", 20, "Maximum number of results")
	return cmd
}

func runInstructionHistory(ctx context.Context, deps *InstructionCommandDeps, idStr string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid instruction ID: %s", idStr)
	}

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := instructionv1.NewInstructionServiceClient(conn)
	tenantID, err := getTenantIDForInstruction(deps)
	if err != nil {
		return err
	}

	resp, err := client.ListInstructionMatches(ctx, &instructionv1.ListInstructionMatchesRequest{
		TenantId:      tenantID,
		InstructionId: id,
		Limit:         int32(instructionLimit),
	})
	if err != nil {
		return fmt.Errorf("listing instruction matches: %w", err)
	}

	return outputInstructionHistory(cfg, id, resp.Matches)
}

// ==================== helpers ====================

func getTenantIDForInstruction(deps *InstructionCommandDeps) (string, error) {
	if deps.Config != nil && deps.Config.TenantID != "" {
		return deps.Config.TenantID, nil
	}
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant, nil
	}
	return "", fmt.Errorf("tenant ID required: set PENF_TENANT_ID env var or tenant_id in config")
}

func getInstructionOutputFormat(cfg *config.CLIConfig) config.OutputFormat {
	if instructionOutput != "" {
		return config.OutputFormat(instructionOutput)
	}
	if cfg != nil {
		return cfg.OutputFormat
	}
	return config.OutputFormatText
}

// ==================== output: list ====================

func outputInstructionList(cfg *config.CLIConfig, instructions []*instructionv1.Instruction) error {
	format := getInstructionOutputFormat(cfg)
	switch format {
	case config.OutputFormatJSON:
		return instructionJSON(instructions)
	case config.OutputFormatYAML:
		return instructionYAML(instructions)
	default:
		return outputInstructionListTable(instructions)
	}
}

func outputInstructionListTable(instructions []*instructionv1.Instruction) error {
	if len(instructions) == 0 {
		fmt.Println("No instructions found.")
		return nil
	}

	fmt.Printf("Instructions (%d):\n\n", len(instructions))
	fmt.Printf("  %-6s %-24s %-8s %-7s %-20s %-6s %s\n",
		"ID", "NAME", "PRIORITY", "ENABLED", "PROJECT", "HITS", "LAST MATCH")
	fmt.Printf("  %-6s %-24s %-8s %-7s %-20s %-6s %s\n",
		"--", "----", "--------", "-------", "-------", "----", "----------")

	for _, inst := range instructions {
		enabled := "\033[32myes\033[0m"
		if !inst.Enabled {
			enabled = "\033[31mno\033[0m "
		}

		project := "-"
		if inst.ProjectName != "" {
			project = instrTruncate(inst.ProjectName, 20)
		}

		lastMatch := "-"
		if inst.LastMatchedAt != nil {
			lastMatch = inst.LastMatchedAt.AsTime().Format("Jan 02 15:04")
		}

		fmt.Printf("  %-6d %-24s %-8s %-7s %-20s %-6d %s\n",
			inst.Id,
			instrTruncate(inst.Name, 24),
			inst.Priority,
			enabled,
			project,
			inst.MatchCount,
			lastMatch,
		)
	}
	fmt.Println()
	return nil
}

// ==================== output: detail ====================

func outputInstructionDetail(cfg *config.CLIConfig, inst *instructionv1.Instruction) error {
	format := getInstructionOutputFormat(cfg)
	switch format {
	case config.OutputFormatJSON:
		return instructionJSON(inst)
	case config.OutputFormatYAML:
		return instructionYAML(inst)
	default:
		return outputInstructionDetailText(inst)
	}
}

func outputInstructionDetailText(inst *instructionv1.Instruction) error {
	enabled := "\033[32menabled\033[0m"
	if !inst.Enabled {
		enabled = "\033[31mdisabled\033[0m"
	}

	fmt.Printf("%s (%s)\n", inst.Name, enabled)
	fmt.Printf("%-16s %d\n", "ID:", inst.Id)
	fmt.Printf("%-16s %s\n", "Instruction:", inst.Instruction)
	fmt.Printf("%-16s %s\n", "Priority:", inst.Priority)
	fmt.Printf("%-16s %s\n", "Model Hint:", inst.ModelHint)
	if inst.ProjectName != "" {
		fmt.Printf("%-16s %s\n", "Project:", inst.ProjectName)
	}
	fmt.Printf("%-16s v%d\n", "Version:", inst.Version)
	fmt.Printf("%-16s %d\n", "Match Count:", inst.MatchCount)
	if inst.LastMatchedAt != nil {
		fmt.Printf("%-16s %s\n", "Last Match:", inst.LastMatchedAt.AsTime().Format("2006-01-02 15:04:05 UTC"))
	}
	if inst.CreatedAt != nil {
		fmt.Printf("%-16s %s\n", "Created:", inst.CreatedAt.AsTime().Format("2006-01-02 15:04:05 UTC"))
	}
	return nil
}

// ==================== output: history ====================

func outputInstructionHistory(cfg *config.CLIConfig, instructionID int64, matches []*instructionv1.InstructionMatch) error {
	format := getInstructionOutputFormat(cfg)
	switch format {
	case config.OutputFormatJSON:
		return instructionJSON(matches)
	case config.OutputFormatYAML:
		return instructionYAML(matches)
	default:
		return outputInstructionHistoryTable(instructionID, matches)
	}
}

func outputInstructionHistoryTable(instructionID int64, matches []*instructionv1.InstructionMatch) error {
	if len(matches) == 0 {
		fmt.Printf("No matches for instruction %d.\n", instructionID)
		return nil
	}

	fmt.Printf("Match History for instruction %d (%d matches):\n\n", instructionID, len(matches))
	fmt.Printf("  %-8s %-10s %-6s %-18s %s\n",
		"ID", "SOURCE", "CONF", "MATCHED", "EXPLANATION")
	fmt.Printf("  %-8s %-10s %-6s %-18s %s\n",
		"--", "------", "----", "-------", "-----------")

	for _, m := range matches {
		matched := "-"
		if m.MatchedAt != nil {
			matched = m.MatchedAt.AsTime().Format("Jan 02 15:04")
		}

		fmt.Printf("  %-8d %-10d %-6.2f %-18s %s\n",
			m.Id,
			m.SourceId,
			m.Confidence,
			matched,
			instrTruncate(m.Explanation, 60),
		)
	}
	fmt.Println()
	return nil
}

// ==================== output: shared ====================

func instructionJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func instructionYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

func instrTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
