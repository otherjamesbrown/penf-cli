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

	schedulev1 "github.com/otherjamesbrown/penf-cli/api/proto/schedule/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

var (
	scheduleOutput      string
	scheduleName        string
	scheduleDescription string
	scheduleType        string
	scheduleCronExpr    string
	scheduleWorkflow    string
	scheduleParams      string
	scheduleOverlap     string
	scheduleLimit       int
	scheduleTriggerDate string
	scheduleDeliver     []string
	scheduleDeliverTo   string
)

// ScheduleCommandDeps holds dependencies for schedule commands.
type ScheduleCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultScheduleDeps returns the default dependencies.
func DefaultScheduleDeps() *ScheduleCommandDeps {
	return &ScheduleCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewScheduleCommand creates the root schedule command with all subcommands.
func NewScheduleCommand(deps *ScheduleCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultScheduleDeps()
	}

	cmd := &cobra.Command{
		Use:     "schedule",
		Short:   "Manage recurring schedules (cron jobs, intervals, heartbeats)",
		Aliases: []string{"sched"},
	}

	cmd.PersistentFlags().StringVarP(&scheduleOutput, "output", "o", "", "Output format: text, json, yaml")

	cmd.AddCommand(newScheduleListCommand(deps))
	cmd.AddCommand(newScheduleShowCommand(deps))
	cmd.AddCommand(newScheduleCreateCommand(deps))
	cmd.AddCommand(newScheduleUpdateCommand(deps))
	cmd.AddCommand(newSchedulePauseCommand(deps))
	cmd.AddCommand(newScheduleResumeCommand(deps))
	cmd.AddCommand(newScheduleTriggerCommand(deps))
	cmd.AddCommand(newScheduleDeleteCommand(deps))
	cmd.AddCommand(newScheduleHistoryCommand(deps))

	return cmd
}

// ==================== list ====================

func newScheduleListCommand(deps *ScheduleCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all schedules",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScheduleList(cmd.Context(), deps)
		},
	}
	cmd.Flags().IntVarP(&scheduleLimit, "limit", "l", 50, "Maximum number of results")
	return cmd
}

func runScheduleList(ctx context.Context, deps *ScheduleCommandDeps) error {
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

	client := schedulev1.NewScheduleServiceClient(conn)
	tenantID, err := getTenantIDForSchedule(deps)
	if err != nil {
		return err
	}

	resp, err := client.ListSchedules(ctx, &schedulev1.ListSchedulesRequest{
		TenantId: tenantID,
		Limit:    int32(scheduleLimit),
	})
	if err != nil {
		return fmt.Errorf("listing schedules: %w", err)
	}

	return outputScheduleList(cfg, resp.Schedules)
}

// ==================== show ====================

func newScheduleShowCommand(deps *ScheduleCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id-or-name>",
		Short: "Show schedule details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScheduleShow(cmd.Context(), deps, args[0])
		},
	}
}

func runScheduleShow(ctx context.Context, deps *ScheduleCommandDeps, scheduleID string) error {
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

	client := schedulev1.NewScheduleServiceClient(conn)
	tenantID, err := getTenantIDForSchedule(deps)
	if err != nil {
		return err
	}

	// Use ListSchedules and filter — no GetSchedule RPC exists
	resp, err := client.ListSchedules(ctx, &schedulev1.ListSchedulesRequest{
		TenantId: tenantID,
		Limit:    100,
	})
	if err != nil {
		return fmt.Errorf("listing schedules: %w", err)
	}

	var found *schedulev1.ScheduleSummary
	for _, s := range resp.Schedules {
		if s.Id == scheduleID || s.Name == scheduleID {
			found = s
			break
		}
	}
	if found == nil {
		return fmt.Errorf("schedule not found: %s", scheduleID)
	}

	return outputScheduleDetail(cfg, found)
}

// ==================== create ====================

func newScheduleCreateCommand(deps *ScheduleCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new schedule",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScheduleCreate(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&scheduleName, "name", "", "Schedule name (required)")
	cmd.Flags().StringVar(&scheduleDescription, "description", "", "Schedule description")
	cmd.Flags().StringVar(&scheduleType, "type", "cron", "Schedule type: cron, interval, heartbeat")
	cmd.Flags().StringVar(&scheduleCronExpr, "cron", "", "Cron expression (required)")
	cmd.Flags().StringVar(&scheduleWorkflow, "workflow", "", "Temporal workflow type (required)")
	cmd.Flags().StringVar(&scheduleParams, "params", "{}", "Workflow parameters as JSON")
	cmd.Flags().StringVar(&scheduleOverlap, "overlap", "skip", "Overlap policy: skip, buffer_one, cancel_other, allow_all")
	cmd.Flags().StringSliceVar(&scheduleDeliver, "deliver", nil, "Delivery channels (repeatable): store, email")
	cmd.Flags().StringVar(&scheduleDeliverTo, "deliver-to", "", "Delivery target (e.g., email address)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("cron")
	_ = cmd.MarkFlagRequired("workflow")

	return cmd
}

func runScheduleCreate(ctx context.Context, deps *ScheduleCommandDeps) error {
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

	client := schedulev1.NewScheduleServiceClient(conn)
	tenantID, err := getTenantIDForSchedule(deps)
	if err != nil {
		return err
	}

	// Merge delivery flags into workflow_params
	mergedParams, err := mergeDeliveryIntoParams(scheduleParams, scheduleDeliver, scheduleDeliverTo)
	if err != nil {
		return fmt.Errorf("merging delivery config: %w", err)
	}

	resp, err := client.CreateSchedule(ctx, &schedulev1.CreateScheduleRequest{
		TenantId:       tenantID,
		Name:           scheduleName,
		Description:    scheduleDescription,
		Type:           scheduleType,
		CronExpr:       scheduleCronExpr,
		WorkflowType:   scheduleWorkflow,
		WorkflowParams: mergedParams,
		OverlapPolicy:  scheduleOverlap,
	})
	if err != nil {
		return fmt.Errorf("creating schedule: %w", err)
	}

	format := getScheduleOutputFormat(cfg)
	switch format {
	case config.OutputFormatJSON:
		return scheduleJSON(map[string]interface{}{
			"id":        resp.ScheduleId,
			"name":      scheduleName,
			"type":      scheduleType,
			"cron_expr": scheduleCronExpr,
			"workflow":  scheduleWorkflow,
			"overlap":   scheduleOverlap,
		})
	case config.OutputFormatYAML:
		return scheduleYAML(map[string]interface{}{
			"id":        resp.ScheduleId,
			"name":      scheduleName,
			"type":      scheduleType,
			"cron_expr": scheduleCronExpr,
			"workflow":  scheduleWorkflow,
			"overlap":   scheduleOverlap,
		})
	default:
		fmt.Printf("\033[32mCreated schedule:\033[0m %s (ID: %s)\n", scheduleName, resp.ScheduleId)
		fmt.Printf("  Type:     %s\n", scheduleType)
		fmt.Printf("  Cron:     %s\n", scheduleCronExpr)
		fmt.Printf("  Workflow: %s\n", scheduleWorkflow)
		fmt.Printf("  Overlap:  %s\n", scheduleOverlap)
		delivery := formatDeliveryDisplay(mergedParams)
		if delivery != "" {
			fmt.Printf("  Delivery: %s\n", delivery)
		}
	}
	return nil
}

// ==================== update ====================

func newScheduleUpdateCommand(deps *ScheduleCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <id-or-name>",
		Short: "Update an existing schedule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScheduleUpdate(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVar(&scheduleDescription, "description", "", "Schedule description")
	cmd.Flags().StringVar(&scheduleCronExpr, "cron", "", "Cron expression")
	cmd.Flags().StringVar(&scheduleWorkflow, "workflow", "", "Temporal workflow type")
	cmd.Flags().StringVar(&scheduleParams, "params", "", "Workflow parameters as JSON (replaces existing)")
	cmd.Flags().StringVar(&scheduleOverlap, "overlap", "", "Overlap policy")
	cmd.Flags().StringSliceVar(&scheduleDeliver, "deliver", nil, "Delivery channels (repeatable): store, email")
	cmd.Flags().StringVar(&scheduleDeliverTo, "deliver-to", "", "Delivery target (e.g., email address)")

	return cmd
}

func runScheduleUpdate(ctx context.Context, deps *ScheduleCommandDeps, input string) error {
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

	client := schedulev1.NewScheduleServiceClient(conn)
	tenantID, err := getTenantIDForSchedule(deps)
	if err != nil {
		return err
	}

	scheduleID, err := scheduleResolveID(ctx, client, tenantID, input)
	if err != nil {
		return err
	}

	// Build workflow_params: if --deliver flags provided, merge into existing or provided params
	workflowParams := scheduleParams
	if len(scheduleDeliver) > 0 || scheduleDeliverTo != "" {
		// If no --params provided, fetch existing params from the schedule
		if workflowParams == "" {
			existing, err := scheduleGetWorkflowParams(ctx, client, tenantID, scheduleID)
			if err != nil {
				return fmt.Errorf("fetching existing schedule params: %w", err)
			}
			workflowParams = existing
		}
		workflowParams, err = mergeDeliveryIntoParams(workflowParams, scheduleDeliver, scheduleDeliverTo)
		if err != nil {
			return fmt.Errorf("merging delivery config: %w", err)
		}
	}

	req := &schedulev1.UpdateScheduleRequest{
		TenantId:   tenantID,
		ScheduleId: scheduleID,
	}

	// Only set fields that were explicitly provided
	if scheduleDescription != "" {
		req.Description = scheduleDescription
	}
	if scheduleCronExpr != "" {
		req.CronExpr = scheduleCronExpr
	}
	if scheduleWorkflow != "" {
		req.WorkflowType = scheduleWorkflow
	}
	if workflowParams != "" {
		req.WorkflowParams = workflowParams
	}
	if scheduleOverlap != "" {
		req.OverlapPolicy = scheduleOverlap
	}

	_, err = client.UpdateSchedule(ctx, req)
	if err != nil {
		return fmt.Errorf("updating schedule: %w", err)
	}

	fmt.Printf("\033[32mUpdated schedule:\033[0m %s\n", input)
	if workflowParams != "" {
		delivery := formatDeliveryDisplay(workflowParams)
		if delivery != "" {
			fmt.Printf("  Delivery: %s\n", delivery)
		}
	}
	return nil
}

// ==================== pause ====================

func newSchedulePauseCommand(deps *ScheduleCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "pause <id-or-name>",
		Short: "Pause a schedule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSchedulePause(cmd.Context(), deps, args[0])
		},
	}
}

func runSchedulePause(ctx context.Context, deps *ScheduleCommandDeps, input string) error {
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

	client := schedulev1.NewScheduleServiceClient(conn)
	tenantID, err := getTenantIDForSchedule(deps)
	if err != nil {
		return err
	}

	scheduleID, err := scheduleResolveID(ctx, client, tenantID, input)
	if err != nil {
		return err
	}

	_, err = client.PauseSchedule(ctx, &schedulev1.PauseScheduleRequest{
		TenantId:   tenantID,
		ScheduleId: scheduleID,
	})
	if err != nil {
		return fmt.Errorf("pausing schedule: %w", err)
	}

	fmt.Printf("\033[33mPaused schedule:\033[0m %s\n", input)
	return nil
}

// ==================== resume ====================

func newScheduleResumeCommand(deps *ScheduleCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "resume <id-or-name>",
		Short: "Resume a paused schedule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScheduleResume(cmd.Context(), deps, args[0])
		},
	}
}

func runScheduleResume(ctx context.Context, deps *ScheduleCommandDeps, input string) error {
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

	client := schedulev1.NewScheduleServiceClient(conn)
	tenantID, err := getTenantIDForSchedule(deps)
	if err != nil {
		return err
	}

	scheduleID, err := scheduleResolveID(ctx, client, tenantID, input)
	if err != nil {
		return err
	}

	_, err = client.ResumeSchedule(ctx, &schedulev1.ResumeScheduleRequest{
		TenantId:   tenantID,
		ScheduleId: scheduleID,
	})
	if err != nil {
		return fmt.Errorf("resuming schedule: %w", err)
	}

	fmt.Printf("\033[32mResumed schedule:\033[0m %s\n", input)
	return nil
}

// ==================== trigger ====================

func newScheduleTriggerCommand(deps *ScheduleCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger <id-or-name>",
		Short: "Trigger an immediate run of a schedule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScheduleTrigger(cmd.Context(), deps, args[0])
		},
	}
	cmd.Flags().StringVar(&scheduleTriggerDate, "date", "", "Reference date (YYYY-MM-DD) for date simulation")
	return cmd
}

func runScheduleTrigger(ctx context.Context, deps *ScheduleCommandDeps, input string) error {
	if scheduleTriggerDate != "" {
		if _, err := time.Parse("2006-01-02", scheduleTriggerDate); err != nil {
			return fmt.Errorf("invalid date format %q (expected YYYY-MM-DD): %w", scheduleTriggerDate, err)
		}
	}

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

	client := schedulev1.NewScheduleServiceClient(conn)
	tenantID, err := getTenantIDForSchedule(deps)
	if err != nil {
		return err
	}

	scheduleID, err := scheduleResolveID(ctx, client, tenantID, input)
	if err != nil {
		return err
	}

	req := &schedulev1.TriggerScheduleRequest{
		TenantId:   tenantID,
		ScheduleId: scheduleID,
	}
	if scheduleTriggerDate != "" {
		req.ReferenceDate = &scheduleTriggerDate
	}

	_, err = client.TriggerSchedule(ctx, req)
	if err != nil {
		return fmt.Errorf("triggering schedule: %w", err)
	}

	if scheduleTriggerDate != "" {
		fmt.Printf("\033[32mTriggered schedule:\033[0m %s (reference date: %s)\n", input, scheduleTriggerDate)
	} else {
		fmt.Printf("\033[32mTriggered schedule:\033[0m %s\n", input)
	}
	fmt.Printf("Use 'penf schedule history %s' to check execution status.\n", input)
	return nil
}

// ==================== delete ====================

func newScheduleDeleteCommand(deps *ScheduleCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id-or-name>",
		Short: "Delete a schedule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScheduleDelete(cmd.Context(), deps, args[0])
		},
	}
}

func runScheduleDelete(ctx context.Context, deps *ScheduleCommandDeps, input string) error {
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

	client := schedulev1.NewScheduleServiceClient(conn)
	tenantID, err := getTenantIDForSchedule(deps)
	if err != nil {
		return err
	}

	scheduleID, err := scheduleResolveID(ctx, client, tenantID, input)
	if err != nil {
		return err
	}

	_, err = client.DeleteSchedule(ctx, &schedulev1.DeleteScheduleRequest{
		TenantId:   tenantID,
		ScheduleId: scheduleID,
	})
	if err != nil {
		return fmt.Errorf("deleting schedule: %w", err)
	}

	fmt.Printf("\033[31mDeleted schedule:\033[0m %s\n", input)
	return nil
}

// ==================== history ====================

func newScheduleHistoryCommand(deps *ScheduleCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <id-or-name>",
		Short: "Show recent execution history for a schedule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScheduleHistory(cmd.Context(), deps, args[0])
		},
	}
	cmd.Flags().IntVarP(&scheduleLimit, "limit", "l", 10, "Number of recent executions")
	return cmd
}

func runScheduleHistory(ctx context.Context, deps *ScheduleCommandDeps, input string) error {
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

	client := schedulev1.NewScheduleServiceClient(conn)
	tenantID, err := getTenantIDForSchedule(deps)
	if err != nil {
		return err
	}

	scheduleID, err := scheduleResolveID(ctx, client, tenantID, input)
	if err != nil {
		return err
	}

	resp, err := client.GetScheduleHistory(ctx, &schedulev1.GetScheduleHistoryRequest{
		TenantId:   tenantID,
		ScheduleId: scheduleID,
		Limit:      int32(scheduleLimit),
	})
	if err != nil {
		return fmt.Errorf("getting schedule history: %w", err)
	}

	return outputScheduleHistory(cfg, input, resp.Executions)
}

// ==================== helpers ====================

func getTenantIDForSchedule(deps *ScheduleCommandDeps) (string, error) {
	if deps.Config != nil && deps.Config.TenantID != "" {
		return deps.Config.TenantID, nil
	}
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant, nil
	}
	return "", fmt.Errorf("tenant ID required: set PENF_TENANT_ID env var or tenant_id in config")
}

func getScheduleOutputFormat(cfg *config.CLIConfig) config.OutputFormat {
	if scheduleOutput != "" {
		return config.OutputFormat(scheduleOutput)
	}
	if cfg != nil {
		return cfg.OutputFormat
	}
	return config.OutputFormatText
}

// ==================== output: list ====================

func outputScheduleList(cfg *config.CLIConfig, schedules []*schedulev1.ScheduleSummary) error {
	format := getScheduleOutputFormat(cfg)
	switch format {
	case config.OutputFormatJSON:
		return scheduleJSON(schedules)
	case config.OutputFormatYAML:
		return scheduleYAML(schedules)
	default:
		return outputScheduleListTable(schedules)
	}
}

func outputScheduleListTable(schedules []*schedulev1.ScheduleSummary) error {
	if len(schedules) == 0 {
		fmt.Println("No schedules found.")
		return nil
	}

	fmt.Printf("Schedules (%d):\n\n", len(schedules))
	fmt.Printf("  %-8s %-28s %-8s %-7s %-16s %-12s %s\n",
		"STATUS", "NAME", "TYPE", "ENABLED", "CRON", "LAST STATUS", "LAST RUN")
	fmt.Printf("  %-8s %-28s %-8s %-7s %-16s %-12s %s\n",
		"------", "----", "----", "-------", "----", "-----------", "--------")

	for _, s := range schedules {
		enabled := "\033[32myes\033[0m"
		if !s.Enabled {
			enabled = "\033[31mno\033[0m "
		}

		lastStatus := "-"
		if s.LastStatus != "" {
			lastStatus = s.LastStatus
		}

		lastRun := "-"
		if s.LastRunAt != nil {
			lastRun = s.LastRunAt.AsTime().Format("Jan 02 15:04")
		}

		// Use first 8 chars of ID as status indicator
		shortID := s.Id
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		fmt.Printf("  %-8s %-28s %-8s %-7s %-16s %-12s %s\n",
			shortID,
			scheduleTruncate(s.Name, 28),
			s.Type,
			enabled,
			scheduleTruncate(s.CronExpr, 16),
			lastStatus,
			lastRun,
		)
	}
	fmt.Println()
	return nil
}

// ==================== output: detail ====================

func outputScheduleDetail(cfg *config.CLIConfig, s *schedulev1.ScheduleSummary) error {
	format := getScheduleOutputFormat(cfg)
	switch format {
	case config.OutputFormatJSON:
		return scheduleJSON(s)
	case config.OutputFormatYAML:
		return scheduleYAML(s)
	default:
		return outputScheduleDetailText(s)
	}
}

func outputScheduleDetailText(s *schedulev1.ScheduleSummary) error {
	enabled := "\033[32menabled\033[0m"
	if !s.Enabled {
		enabled = "\033[31mpaused\033[0m"
	}

	fmt.Printf("%s (%s)\n", s.Name, enabled)
	fmt.Printf("%-16s %s\n", "ID:", s.Id)
	if s.Description != "" {
		fmt.Printf("%-16s %s\n", "Description:", s.Description)
	}
	fmt.Printf("%-16s %s\n", "Type:", s.Type)
	fmt.Printf("%-16s %s\n", "Cron:", s.CronExpr)
	fmt.Printf("%-16s %s\n", "Workflow:", s.WorkflowType)
	fmt.Printf("%-16s %s\n", "Overlap Policy:", s.OverlapPolicy)

	if s.LastStatus != "" {
		fmt.Printf("%-16s %s\n", "Last Status:", s.LastStatus)
	}
	if s.LastRunAt != nil {
		fmt.Printf("%-16s %s\n", "Last Run:", s.LastRunAt.AsTime().Format("2006-01-02 15:04:05 UTC"))
	}
	if s.NextRunAt != nil {
		fmt.Printf("%-16s %s\n", "Next Run:", s.NextRunAt.AsTime().Format("2006-01-02 15:04:05 UTC"))
	}
	if s.LastError != "" {
		fmt.Printf("%-16s %s\n", "Last Error:", s.LastError)
	}

	// Display delivery config from workflow_params
	if s.WorkflowParams != "" {
		delivery := formatDeliveryDisplay(s.WorkflowParams)
		if delivery != "" {
			fmt.Printf("%-16s %s\n", "Delivery:", delivery)
		}
	}
	return nil
}

// ==================== output: history ====================

func outputScheduleHistory(cfg *config.CLIConfig, scheduleID string, execs []*schedulev1.ScheduleExecution) error {
	format := getScheduleOutputFormat(cfg)
	switch format {
	case config.OutputFormatJSON:
		return scheduleJSON(execs)
	case config.OutputFormatYAML:
		return scheduleYAML(execs)
	default:
		return outputScheduleHistoryTable(scheduleID, execs)
	}
}

func outputScheduleHistoryTable(scheduleID string, execs []*schedulev1.ScheduleExecution) error {
	if len(execs) == 0 {
		fmt.Printf("No execution history for schedule %s.\n", scheduleID)
		return nil
	}

	fmt.Printf("Execution History for %s (%d runs):\n\n", scheduleID, len(execs))
	fmt.Printf("  %-12s %-20s %-20s %s\n", "STATUS", "STARTED", "COMPLETED", "ERROR")
	fmt.Printf("  %-12s %-20s %-20s %s\n", "------", "-------", "---------", "-----")

	for _, e := range execs {
		started := "-"
		if e.StartedAt != nil {
			started = e.StartedAt.AsTime().Format("Jan 02 15:04:05")
		}
		completed := "-"
		if e.CompletedAt != nil {
			completed = e.CompletedAt.AsTime().Format("Jan 02 15:04:05")
		}
		errStr := "-"
		if e.Error != "" {
			errStr = scheduleTruncate(e.Error, 40)
		}

		fmt.Printf("  %-12s %-20s %-20s %s\n", e.Status, started, completed, errStr)
		if e.ResultMetadata != "" && e.ResultMetadata != "{}" {
			fmt.Printf("    metadata: %s\n", scheduleTruncate(e.ResultMetadata, 100))
		}
	}
	fmt.Println()
	return nil
}

// ==================== output: shared ====================

func scheduleJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func scheduleYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

func scheduleTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// mergeDeliveryIntoParams merges --deliver and --deliver-to flags into workflow_params JSON.
// If no --deliver flags are provided, defaults to ["store"].
func mergeDeliveryIntoParams(paramsJSON string, deliver []string, deliverTo string) (string, error) {
	if paramsJSON == "" {
		paramsJSON = "{}"
	}

	var params map[string]interface{}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return "", fmt.Errorf("invalid workflow params JSON: %w", err)
	}

	// Default to ["store"] if --deliver not specified
	if len(deliver) == 0 {
		deliver = []string{"store"}
	}
	params["delivery"] = deliver

	if deliverTo != "" {
		params["delivery_target"] = deliverTo
	}

	out, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("marshalling params: %w", err)
	}
	return string(out), nil
}

// formatDeliveryDisplay extracts delivery config from workflow_params and formats for display.
func formatDeliveryDisplay(paramsJSON string) string {
	if paramsJSON == "" || paramsJSON == "{}" {
		return ""
	}

	var params map[string]interface{}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return ""
	}

	deliveryRaw, ok := params["delivery"]
	if !ok {
		return ""
	}

	deliveryArr, ok := deliveryRaw.([]interface{})
	if !ok {
		return ""
	}

	var channels []string
	for _, d := range deliveryArr {
		if s, ok := d.(string); ok {
			channels = append(channels, s)
		}
	}
	if len(channels) == 0 {
		return ""
	}

	target, _ := params["delivery_target"].(string)

	var parts []string
	for _, ch := range channels {
		if ch == "email" && target != "" {
			parts = append(parts, fmt.Sprintf("email → %s", target))
		} else {
			parts = append(parts, ch)
		}
	}
	return strings.Join(parts, ", ")
}

// scheduleGetWorkflowParams fetches the workflow_params for a schedule by listing and matching.
func scheduleGetWorkflowParams(ctx context.Context, client schedulev1.ScheduleServiceClient, tenantID, scheduleID string) (string, error) {
	resp, err := client.ListSchedules(ctx, &schedulev1.ListSchedulesRequest{
		TenantId: tenantID,
		Limit:    100,
	})
	if err != nil {
		return "{}", fmt.Errorf("listing schedules: %w", err)
	}

	for _, s := range resp.Schedules {
		if s.Id == scheduleID {
			if s.WorkflowParams != "" {
				return s.WorkflowParams, nil
			}
			return "{}", nil
		}
	}
	return "{}", nil
}

// scheduleResolveID resolves a schedule by name or ID. If the input looks like
// a name (no dashes/UUID format), it lists schedules and finds the matching one.
func scheduleResolveID(ctx context.Context, client schedulev1.ScheduleServiceClient, tenantID, input string) (string, error) {
	// If it looks like a UUID, use it directly
	if len(input) == 36 && strings.Count(input, "-") == 4 {
		return input, nil
	}

	// Otherwise try to find by name
	resp, err := client.ListSchedules(ctx, &schedulev1.ListSchedulesRequest{
		TenantId: tenantID,
		Limit:    100,
	})
	if err != nil {
		return "", fmt.Errorf("listing schedules to resolve name: %w", err)
	}

	for _, s := range resp.Schedules {
		if s.Name == input {
			return s.Id, nil
		}
	}
	return "", fmt.Errorf("schedule not found: %s", input)
}
