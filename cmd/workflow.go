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

// WorkflowStatus represents the status of a workflow.
type WorkflowStatus string

const (
	// WorkflowStatusPending indicates workflow is queued.
	WorkflowStatusPending WorkflowStatus = "pending"
	// WorkflowStatusRunning indicates workflow is in progress.
	WorkflowStatusRunning WorkflowStatus = "running"
	// WorkflowStatusCompleted indicates workflow finished successfully.
	WorkflowStatusCompleted WorkflowStatus = "completed"
	// WorkflowStatusFailed indicates workflow encountered an error.
	WorkflowStatusFailed WorkflowStatus = "failed"
	// WorkflowStatusCancelled indicates workflow was cancelled.
	WorkflowStatusCancelled WorkflowStatus = "cancelled"
)

// Workflow represents a workflow instance.
type Workflow struct {
	ID          string            `json:"id" yaml:"id"`
	Type        string            `json:"type" yaml:"type"`
	Name        string            `json:"name" yaml:"name"`
	Status      WorkflowStatus    `json:"status" yaml:"status"`
	Progress    int               `json:"progress" yaml:"progress"`
	Message     string            `json:"message,omitempty" yaml:"message,omitempty"`
	Steps       []WorkflowStep    `json:"steps,omitempty" yaml:"steps,omitempty"`
	Input       map[string]string `json:"input,omitempty" yaml:"input,omitempty"`
	Output      map[string]string `json:"output,omitempty" yaml:"output,omitempty"`
	Error       string            `json:"error,omitempty" yaml:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at" yaml:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
}

// WorkflowStep represents a step within a workflow.
type WorkflowStep struct {
	Name        string         `json:"name" yaml:"name"`
	Status      WorkflowStatus `json:"status" yaml:"status"`
	Message     string         `json:"message,omitempty" yaml:"message,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
}

// WorkflowListResponse contains workflow list results.
type WorkflowListResponse struct {
	Workflows  []Workflow `json:"workflows" yaml:"workflows"`
	TotalCount int        `json:"total_count" yaml:"total_count"`
	FetchedAt  time.Time  `json:"fetched_at" yaml:"fetched_at"`
}

// WorkflowCommandDeps holds the dependencies for workflow commands.
type WorkflowCommandDeps struct {
	Config       *config.CLIConfig
	GRPCClient   *client.GRPCClient
	OutputFormat config.OutputFormat
	LoadConfig   func() (*config.CLIConfig, error)
	InitClient   func(*config.CLIConfig) (*client.GRPCClient, error)

	// Optional function overrides for testing.
	// If set, these are used instead of calling the actual client methods.
	ListWorkflowsFn      func(context.Context, client.ListWorkflowsFilter) (*client.ListWorkflowsResult, error)
	GetWorkflowStatusFn  func(context.Context, string, string) (*client.WorkflowStatusDetails, error)
	CancelWorkflowFn     func(context.Context, string, string, string) (*client.CancelWorkflowResult, error)
	TerminateWorkflowFn  func(context.Context, string, string, string) (*client.CancelWorkflowResult, error)
}

// DefaultWorkflowDeps returns the default dependencies for production use.
func DefaultWorkflowDeps() *WorkflowCommandDeps {
	return &WorkflowCommandDeps{
		LoadConfig: config.LoadConfig,
		InitClient: client.ConnectFromConfig,
	}
}

// Workflow command flags.
var (
	workflowType   string
	workflowStatus string
	workflowLimit  int
	workflowOutput string
	workflowWatch  bool
	workflowForce  bool
)

// NewWorkflowCommand creates the root workflow command with all subcommands.
func NewWorkflowCommand(deps *WorkflowCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultWorkflowDeps()
	}

	cmd := &cobra.Command{
		Use:     "workflow",
		Aliases: []string{"wf"},
		Short:   "Manage background workflows and processing jobs",
		Long: `Manage background workflows and processing jobs in Penfold.

Workflows are long-running operations like content ingestion, batch processing,
or scheduled tasks. Use these commands to monitor and manage workflow execution.

Commands:
  list      - List all workflows
  status    - Show detailed workflow status
  cancel    - Cancel a running workflow
  terminate - Terminate a workflow immediately

Examples:
  # List recent workflows
  penf workflow list

  # List only running workflows
  penf workflow list --status=running

  # Check status of a specific workflow
  penf workflow status wf-abc123

  # Cancel a running workflow (graceful)
  penf workflow cancel wf-abc123

  # Terminate a workflow (immediate)
  penf workflow terminate wf-abc123

Related Commands:
  penf pipeline    Pipeline-level statistics and queue status
  penf content     Browse processed content items`,
	}

	// Add subcommands.
	cmd.AddCommand(newWorkflowListCommand(deps))
	cmd.AddCommand(newWorkflowStatusCommand(deps))
	cmd.AddCommand(newWorkflowCancelCommand(deps))
	cmd.AddCommand(newWorkflowTerminateCommand(deps))

	return cmd
}

// newWorkflowListCommand creates the 'workflow list' subcommand.
func newWorkflowListCommand(deps *WorkflowCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List workflows",
		Long: `List workflows with optional filtering.

By default, shows the most recent workflows across all types and statuses.
Use filters to narrow down the results.

Examples:
  # List all recent workflows
  penf workflow list

  # List only running workflows
  penf workflow list --status=running

  # List ingestion workflows
  penf workflow list --type=ingestion

  # List with custom limit
  penf workflow list --limit=50

  # Output as JSON
  penf workflow list --output=json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowList(cmd.Context(), deps)
		},
	}

	// Define flags.
	cmd.Flags().StringVarP(&workflowType, "type", "t", "", "Filter by workflow type (ingestion, batch, scheduled)")
	cmd.Flags().StringVarP(&workflowStatus, "status", "s", "", "Filter by status (pending, running, completed, failed, cancelled)")
	cmd.Flags().IntVarP(&workflowLimit, "limit", "l", 20, "Maximum number of workflows to show")
	cmd.Flags().StringVarP(&workflowOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newWorkflowStatusCommand creates the 'workflow status' subcommand.
func newWorkflowStatusCommand(deps *WorkflowCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <workflow-id>",
		Short: "Show detailed workflow status",
		Long: `Show detailed status information for a specific workflow.

Displays workflow metadata, current progress, step-by-step execution details,
and any output or errors.

Examples:
  # Get workflow status
  penf workflow status wf-abc123

  # Watch workflow progress
  penf workflow status wf-abc123 --watch

  # Output as JSON
  penf workflow status wf-abc123 --output=json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowStatus(cmd.Context(), deps, args[0])
		},
	}

	// Define flags.
	cmd.Flags().BoolVarP(&workflowWatch, "watch", "w", false, "Watch workflow progress until completion")
	cmd.Flags().StringVarP(&workflowOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newWorkflowCancelCommand creates the 'workflow cancel' subcommand.
func newWorkflowCancelCommand(deps *WorkflowCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <workflow-id>",
		Short: "Cancel a running workflow",
		Long: `Cancel a running or pending workflow.

This will attempt to gracefully stop the workflow. Already completed steps
will not be rolled back. Use --force to immediately terminate the workflow.

Examples:
  # Cancel a workflow
  penf workflow cancel wf-abc123

  # Force cancel (immediate termination)
  penf workflow cancel wf-abc123 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowCancel(cmd.Context(), deps, args[0])
		},
	}

	// Define flags.
	cmd.Flags().BoolVarP(&workflowForce, "force", "f", false, "Force immediate termination")

	return cmd
}

// newWorkflowTerminateCommand creates the 'workflow terminate' subcommand.
func newWorkflowTerminateCommand(deps *WorkflowCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "terminate <workflow-id>",
		Short: "Immediately terminate a workflow",
		Long: `Terminate a workflow immediately without cleanup.

This will forcefully stop the workflow without allowing cleanup or graceful shutdown.
Already completed steps will not be rolled back. For graceful cancellation, use 'cancel' instead.

Terminate is more forceful than cancel:
- Cancel requests graceful shutdown (workflow can still run cleanup)
- Terminate stops execution immediately (no cleanup allowed)

Examples:
  # Terminate a stuck workflow
  penf workflow terminate wf-abc123

  # Terminate when cancel doesn't work
  penf workflow terminate wf-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowTerminate(cmd.Context(), deps, args[0])
		},
	}

	return cmd
}

// runWorkflowList executes the workflow list command.
func runWorkflowList(ctx context.Context, deps *WorkflowCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format.
	outputFormat := cfg.OutputFormat
	if workflowOutput != "" {
		outputFormat = config.OutputFormat(workflowOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", workflowOutput)
		}
	}

	var grpcClient *client.GRPCClient

	// Initialize gRPC client only if not using mock.
	if deps.ListWorkflowsFn == nil {
		grpcClient, err = deps.InitClient(cfg)
		if err != nil {
			return fmt.Errorf("initializing client: %w", err)
		}
		defer grpcClient.Close()
	}

	// Map status string for API (normalize capitalization).
	statusFilter := workflowStatus
	if statusFilter != "" {
		switch strings.ToLower(statusFilter) {
		case "pending":
			// Temporal doesn't have "pending" - workflows are either Running or completed states
			// Return empty list for pending filter
			fmt.Println("Note: Temporal workflows don't have a 'pending' state. Showing 'Running' workflows instead.")
			statusFilter = "Running"
		case "running":
			statusFilter = "Running"
		case "completed":
			statusFilter = "Completed"
		case "failed":
			statusFilter = "Failed"
		case "cancelled", "canceled":
			statusFilter = "Canceled"
		default:
			return fmt.Errorf("invalid status: %s (must be running, completed, failed, or cancelled)", workflowStatus)
		}
	}

	// Call the workflow service.
	filter := client.ListWorkflowsFilter{
		WorkflowType: workflowType,
		Status:       statusFilter,
		PageSize:     int32(workflowLimit),
	}

	var result *client.ListWorkflowsResult

	// Use mock function if provided (for testing).
	if deps.ListWorkflowsFn != nil {
		result, err = deps.ListWorkflowsFn(ctx, filter)
	} else {
		result, err = grpcClient.ListWorkflows(ctx, filter)
	}

	if err != nil {
		return fmt.Errorf("listing workflows: %w", err)
	}

	// Convert to response format.
	workflows := make([]Workflow, 0, len(result.Workflows))
	for _, wf := range result.Workflows {
		workflow := Workflow{
			ID:        wf.WorkflowID,
			Type:      wf.WorkflowType,
			Name:      wf.WorkflowType, // Use type as name for now
			Status:    mapAPIStatusToWorkflowStatus(wf.Status),
			CreatedAt: wf.StartTime,
		}
		if !wf.StartTime.IsZero() {
			startTime := wf.StartTime
			workflow.StartedAt = &startTime
		}
		if !wf.CloseTime.IsZero() {
			closeTime := wf.CloseTime
			workflow.CompletedAt = &closeTime
		}
		workflows = append(workflows, workflow)
	}

	response := WorkflowListResponse{
		Workflows:  workflows,
		TotalCount: len(workflows),
		FetchedAt:  time.Now(),
	}

	return outputWorkflowList(outputFormat, response)
}

// runWorkflowStatus executes the workflow status command.
func runWorkflowStatus(ctx context.Context, deps *WorkflowCommandDeps, workflowID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format.
	outputFormat := cfg.OutputFormat
	if workflowOutput != "" {
		outputFormat = config.OutputFormat(workflowOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", workflowOutput)
		}
	}

	if workflowWatch {
		return runWorkflowWatch(ctx, deps, workflowID, outputFormat)
	}

	var status *client.WorkflowStatusDetails

	// Use mock function if provided (for testing).
	if deps.GetWorkflowStatusFn != nil {
		var err error
		status, err = deps.GetWorkflowStatusFn(ctx, workflowID, "")
		if err != nil {
			return fmt.Errorf("getting workflow status: %w", err)
		}
	} else {
		// Initialize gRPC client.
		grpcClient, err := deps.InitClient(cfg)
		if err != nil {
			return fmt.Errorf("initializing client: %w", err)
		}
		defer grpcClient.Close()

		// Get workflow status from the service.
		status, err = grpcClient.GetWorkflowStatus(ctx, workflowID, "")
		if err != nil {
			return fmt.Errorf("getting workflow status: %w", err)
		}
	}

	// Convert to workflow format.
	workflow := &Workflow{
		ID:        status.WorkflowID,
		Type:      status.WorkflowType,
		Name:      status.WorkflowType, // Use type as name
		Status:    mapAPIStatusToWorkflowStatus(status.Status),
		CreatedAt: status.StartTime,
		Input:     status.SearchAttributes,
	}
	if !status.StartTime.IsZero() {
		startTime := status.StartTime
		workflow.StartedAt = &startTime
	}
	if !status.CloseTime.IsZero() {
		closeTime := status.CloseTime
		workflow.CompletedAt = &closeTime
	}
	// Add some extra info as message
	if status.PendingActivities > 0 {
		workflow.Message = fmt.Sprintf("%d pending activities", status.PendingActivities)
	}

	return outputWorkflowStatus(outputFormat, workflow)
}

// runWorkflowWatch monitors workflow progress until completion.
func runWorkflowWatch(ctx context.Context, deps *WorkflowCommandDeps, workflowID string, outputFormat config.OutputFormat) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	var grpcClient *client.GRPCClient

	// Initialize gRPC client if not using mock.
	if deps.GetWorkflowStatusFn == nil {
		grpcClient, err = deps.InitClient(cfg)
		if err != nil {
			return fmt.Errorf("initializing client: %w", err)
		}
		defer grpcClient.Close()
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		// Clear screen for human-readable output.
		if outputFormat == config.OutputFormatText {
			fmt.Print("\033[H\033[2J")
		}

		// Get workflow status from the service.
		var status *client.WorkflowStatusDetails
		if deps.GetWorkflowStatusFn != nil {
			status, err = deps.GetWorkflowStatusFn(ctx, workflowID, "")
		} else {
			status, err = grpcClient.GetWorkflowStatus(ctx, workflowID, "")
		}
		if err != nil {
			return fmt.Errorf("getting workflow status: %w", err)
		}

		// Convert to workflow format.
		workflow := &Workflow{
			ID:        status.WorkflowID,
			Type:      status.WorkflowType,
			Name:      status.WorkflowType,
			Status:    mapAPIStatusToWorkflowStatus(status.Status),
			CreatedAt: status.StartTime,
		}
		if !status.StartTime.IsZero() {
			startTime := status.StartTime
			workflow.StartedAt = &startTime
		}
		if !status.CloseTime.IsZero() {
			closeTime := status.CloseTime
			workflow.CompletedAt = &closeTime
		}
		if status.PendingActivities > 0 {
			workflow.Message = fmt.Sprintf("%d pending activities", status.PendingActivities)
		}

		if err := outputWorkflowStatus(outputFormat, workflow); err != nil {
			return err
		}

		// Check if workflow is terminal.
		if workflow.Status == WorkflowStatusCompleted ||
			workflow.Status == WorkflowStatusFailed ||
			workflow.Status == WorkflowStatusCancelled {
			fmt.Println("\nWorkflow completed.")
			return nil
		}

		select {
		case <-ctx.Done():
			fmt.Println("\nStopped watching.")
			return nil
		case <-ticker.C:
			continue
		}
	}
}

// runWorkflowCancel executes the workflow cancel command.
func runWorkflowCancel(ctx context.Context, deps *WorkflowCommandDeps, workflowID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	var grpcClient *client.GRPCClient
	var result *client.CancelWorkflowResult

	// Use mock functions if provided (for testing).
	if workflowForce {
		fmt.Printf("Force cancelling workflow %s...\n", workflowID)
		if deps.TerminateWorkflowFn != nil {
			result, err = deps.TerminateWorkflowFn(ctx, workflowID, "", "Terminated via CLI (--force)")
		} else {
			// Initialize gRPC client.
			grpcClient, err = deps.InitClient(cfg)
			if err != nil {
				return fmt.Errorf("initializing client: %w", err)
			}
			defer grpcClient.Close()
			result, err = grpcClient.TerminateWorkflow(ctx, workflowID, "", "Terminated via CLI (--force)")
		}
	} else {
		fmt.Printf("Cancelling workflow %s...\n", workflowID)
		if deps.CancelWorkflowFn != nil {
			result, err = deps.CancelWorkflowFn(ctx, workflowID, "", "Cancelled via CLI")
		} else {
			// Initialize gRPC client.
			grpcClient, err = deps.InitClient(cfg)
			if err != nil {
				return fmt.Errorf("initializing client: %w", err)
			}
			defer grpcClient.Close()
			result, err = grpcClient.CancelWorkflow(ctx, workflowID, "", "Cancelled via CLI")
		}
	}

	if err != nil {
		return fmt.Errorf("cancelling workflow: %w", err)
	}

	if result.Accepted {
		fmt.Printf("\n%s\n", result.Message)
	} else {
		fmt.Printf("\nFailed to cancel workflow: %s\n", result.Message)
	}

	return nil
}

// runWorkflowTerminate executes the workflow terminate command.
func runWorkflowTerminate(ctx context.Context, deps *WorkflowCommandDeps, workflowID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	var grpcClient *client.GRPCClient
	var result *client.CancelWorkflowResult

	fmt.Printf("Terminating workflow %s...\n", workflowID)

	// Use mock function if provided (for testing).
	if deps.TerminateWorkflowFn != nil {
		result, err = deps.TerminateWorkflowFn(ctx, workflowID, "", "Terminated via CLI")
	} else {
		// Initialize gRPC client.
		grpcClient, err = deps.InitClient(cfg)
		if err != nil {
			return fmt.Errorf("initializing client: %w", err)
		}
		defer grpcClient.Close()
		result, err = grpcClient.TerminateWorkflow(ctx, workflowID, "", "Terminated via CLI")
	}

	if err != nil {
		return fmt.Errorf("terminating workflow: %w", err)
	}

	if result.Accepted {
		fmt.Printf("\n%s\n", result.Message)
	} else {
		fmt.Printf("\nFailed to terminate workflow: %s\n", result.Message)
	}

	return nil
}

// mapAPIStatusToWorkflowStatus maps API status string to WorkflowStatus.
func mapAPIStatusToWorkflowStatus(status string) WorkflowStatus {
	switch status {
	case "Running":
		return WorkflowStatusRunning
	case "Completed":
		return WorkflowStatusCompleted
	case "Failed":
		return WorkflowStatusFailed
	case "Canceled", "Cancelled":
		return WorkflowStatusCancelled
	case "Terminated":
		return WorkflowStatusCancelled // Map terminated to cancelled for display
	case "TimedOut":
		return WorkflowStatusFailed // Map timed out to failed for display
	default:
		return WorkflowStatusPending
	}
}

// outputWorkflowList formats and outputs the workflow list.
func outputWorkflowList(format config.OutputFormat, response WorkflowListResponse) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(response)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(response)
	default:
		return outputWorkflowListText(response)
	}
}

// outputWorkflowListText formats workflow list for terminal display.
func outputWorkflowListText(response WorkflowListResponse) error {
	if len(response.Workflows) == 0 {
		fmt.Println("No workflows found.")
		return nil
	}

	fmt.Printf("Workflows (%d):\n\n", response.TotalCount)
	fmt.Println("  ID                    TYPE        STATUS       PROGRESS  NAME")
	fmt.Println("  --                    ----        ------       --------  ----")

	for _, wf := range response.Workflows {
		statusColor := getWorkflowStatusColor(wf.Status)
		progressStr := fmt.Sprintf("%d%%", wf.Progress)

		fmt.Printf("  %-21s %-11s %s%-12s\033[0m %8s  %s\n",
			wf.ID, wf.Type, statusColor, wf.Status, progressStr, wf.Name)
	}

	fmt.Println()
	return nil
}

// outputWorkflowStatus formats and outputs workflow status.
func outputWorkflowStatus(format config.OutputFormat, workflow *Workflow) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(workflow)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(workflow)
	default:
		return outputWorkflowStatusText(workflow)
	}
}

// outputWorkflowStatusText formats workflow status for terminal display.
func outputWorkflowStatusText(workflow *Workflow) error {
	statusColor := getWorkflowStatusColor(workflow.Status)

	fmt.Printf("\033[1mWorkflow: %s\033[0m\n", workflow.ID)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("  Name:     %s\n", workflow.Name)
	fmt.Printf("  Type:     %s\n", workflow.Type)
	fmt.Printf("  Status:   %s%s\033[0m\n", statusColor, workflow.Status)
	fmt.Printf("  Progress: %d%%\n", workflow.Progress)
	if workflow.Message != "" {
		fmt.Printf("  Message:  %s\n", workflow.Message)
	}
	if workflow.Error != "" {
		fmt.Printf("  Error:    \033[31m%s\033[0m\n", workflow.Error)
	}
	fmt.Println()

	// Timestamps.
	fmt.Printf("  Created:  %s\n", workflow.CreatedAt.Format(time.RFC3339))
	if workflow.StartedAt != nil {
		fmt.Printf("  Started:  %s\n", workflow.StartedAt.Format(time.RFC3339))
	}
	if workflow.CompletedAt != nil {
		fmt.Printf("  Finished: %s\n", workflow.CompletedAt.Format(time.RFC3339))
		duration := workflow.CompletedAt.Sub(*workflow.StartedAt)
		fmt.Printf("  Duration: %s\n", duration.Round(time.Second))
	} else if workflow.StartedAt != nil {
		duration := time.Since(*workflow.StartedAt)
		fmt.Printf("  Elapsed:  %s\n", duration.Round(time.Second))
	}
	fmt.Println()

	// Input parameters.
	if len(workflow.Input) > 0 {
		fmt.Println("  Input:")
		for k, v := range workflow.Input {
			fmt.Printf("    %s: %s\n", k, v)
		}
		fmt.Println()
	}

	// Steps.
	if len(workflow.Steps) > 0 {
		fmt.Println("  Steps:")
		for i, step := range workflow.Steps {
			stepColor := getWorkflowStatusColor(step.Status)
			statusIcon := getStepStatusIcon(step.Status)
			fmt.Printf("    %d. %s %s%s\033[0m - %s\n",
				i+1, statusIcon, stepColor, step.Name, step.Message)
		}
		fmt.Println()
	}

	// Output.
	if len(workflow.Output) > 0 {
		fmt.Println("  Output:")
		for k, v := range workflow.Output {
			fmt.Printf("    %s: %s\n", k, v)
		}
		fmt.Println()
	}

	return nil
}

// getWorkflowStatusColor returns ANSI color code for workflow status.
func getWorkflowStatusColor(status WorkflowStatus) string {
	switch status {
	case WorkflowStatusCompleted:
		return "\033[32m" // Green
	case WorkflowStatusRunning:
		return "\033[34m" // Blue
	case WorkflowStatusPending:
		return "\033[33m" // Yellow
	case WorkflowStatusFailed:
		return "\033[31m" // Red
	case WorkflowStatusCancelled:
		return "\033[90m" // Gray
	default:
		return ""
	}
}

// getStepStatusIcon returns an icon for step status.
func getStepStatusIcon(status WorkflowStatus) string {
	switch status {
	case WorkflowStatusCompleted:
		return "\033[32m[OK]\033[0m"
	case WorkflowStatusRunning:
		return "\033[34m[..]\033[0m"
	case WorkflowStatusPending:
		return "\033[33m[  ]\033[0m"
	case WorkflowStatusFailed:
		return "\033[31m[!!]\033[0m"
	case WorkflowStatusCancelled:
		return "\033[90m[--]\033[0m"
	default:
		return "[??]"
	}
}
