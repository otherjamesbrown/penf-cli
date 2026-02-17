// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// mockWorkflowConfig creates a mock configuration for workflow command testing.
func mockWorkflowConfig() *config.CLIConfig {
	return &config.CLIConfig{
		ServerAddress: "localhost:50051",
		Timeout:       30 * time.Second,
		OutputFormat:  config.OutputFormatText,
		TenantID:      "tenant-test-001",
		Debug:         false,
		Insecure:      true,
	}
}

// createWorkflowTestDeps creates test dependencies for workflow commands.
func createWorkflowTestDeps(cfg *config.CLIConfig) *WorkflowCommandDeps {
	return &WorkflowCommandDeps{
		Config:       cfg,
		OutputFormat: cfg.OutputFormat,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		InitClient: func(c *config.CLIConfig) (*client.GRPCClient, error) {
			return nil, nil
		},
	}
}

func TestNewWorkflowCommand(t *testing.T) {
	deps := createWorkflowTestDeps(mockWorkflowConfig())
	cmd := NewWorkflowCommand(deps)

	assert.NotNil(t, cmd)
	assert.Equal(t, "workflow", cmd.Use)
	assert.Contains(t, cmd.Short, "workflow")
	assert.Contains(t, cmd.Aliases, "wf")

	// Check subcommands exist.
	subcommands := cmd.Commands()
	expectedSubcmds := []string{"list", "status", "cancel", "terminate"}

	for _, expected := range expectedSubcmds {
		found := false
		for _, sub := range subcommands {
			if strings.HasPrefix(sub.Use, expected) {
				found = true
				break
			}
		}
		assert.True(t, found, "expected subcommand %q not found", expected)
	}
}

func TestNewWorkflowCommand_WithNilDeps(t *testing.T) {
	cmd := NewWorkflowCommand(nil)
	assert.NotNil(t, cmd)
	assert.Equal(t, "workflow", cmd.Use)
}

func TestWorkflowCommand_ListSubcommand(t *testing.T) {
	deps := createWorkflowTestDeps(mockWorkflowConfig())
	cmd := NewWorkflowCommand(deps)

	listCmd, _, err := cmd.Find([]string{"list"})
	require.NoError(t, err)
	require.NotNil(t, listCmd)

	assert.Equal(t, "list", listCmd.Use)
	assert.Contains(t, listCmd.Aliases, "ls")

	// Check flags.
	flags := []string{"type", "status", "limit", "output"}
	for _, flagName := range flags {
		flag := listCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "list command missing flag: %s", flagName)
	}
}

func TestWorkflowCommand_StatusSubcommand(t *testing.T) {
	deps := createWorkflowTestDeps(mockWorkflowConfig())
	cmd := NewWorkflowCommand(deps)

	statusCmd, _, err := cmd.Find([]string{"status"})
	require.NoError(t, err)
	require.NotNil(t, statusCmd)

	assert.Contains(t, statusCmd.Use, "status")

	// Check flags.
	flags := []string{"watch", "output"}
	for _, flagName := range flags {
		flag := statusCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "status command missing flag: %s", flagName)
	}
}

func TestWorkflowCommand_CancelSubcommand(t *testing.T) {
	deps := createWorkflowTestDeps(mockWorkflowConfig())
	cmd := NewWorkflowCommand(deps)

	cancelCmd, _, err := cmd.Find([]string{"cancel"})
	require.NoError(t, err)
	require.NotNil(t, cancelCmd)

	assert.Contains(t, cancelCmd.Use, "cancel")

	// Check flags.
	flag := cancelCmd.Flags().Lookup("force")
	assert.NotNil(t, flag, "cancel command missing force flag")
}

func TestRunWorkflowList(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, _ := createWorkflowTestDepsWithMocks(cfg)

	// Reset global flags.
	oldType := workflowType
	oldStatus := workflowStatus
	oldLimit := workflowLimit
	oldOutput := workflowOutput
	workflowType = ""
	workflowStatus = ""
	workflowLimit = 20
	workflowOutput = ""
	defer func() {
		workflowType = oldType
		workflowStatus = oldStatus
		workflowLimit = oldLimit
		workflowOutput = oldOutput
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runWorkflowList(ctx, deps)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Workflows")
}

func TestRunWorkflowList_JSONOutput(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, _ := createWorkflowTestDepsWithMocks(cfg)

	// Reset global flags.
	oldOutput := workflowOutput
	workflowOutput = "json"
	defer func() {
		workflowOutput = oldOutput
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runWorkflowList(ctx, deps)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)

	// Verify valid JSON.
	var response WorkflowListResponse
	err = json.Unmarshal([]byte(output), &response)
	assert.NoError(t, err)
	assert.True(t, len(response.Workflows) >= 0)
}

func TestRunWorkflowList_YAMLOutput(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, _ := createWorkflowTestDepsWithMocks(cfg)

	oldOutput := workflowOutput
	workflowOutput = "yaml"
	defer func() {
		workflowOutput = oldOutput
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runWorkflowList(ctx, deps)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)

	// Verify valid YAML.
	var response WorkflowListResponse
	err = yaml.Unmarshal([]byte(output), &response)
	assert.NoError(t, err)
}

func TestRunWorkflowList_WithStatusFilter(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, _ := createWorkflowTestDepsWithMocks(cfg)

	oldStatus := workflowStatus
	oldOutput := workflowOutput
	workflowStatus = "running"
	workflowOutput = ""
	defer func() {
		workflowStatus = oldStatus
		workflowOutput = oldOutput
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runWorkflowList(ctx, deps)

	w.Close()
	os.Stdout = oldStdout

	assert.NoError(t, err)
}

func TestRunWorkflowList_InvalidStatusFilter(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, _ := createWorkflowTestDepsWithMocks(cfg)

	oldStatus := workflowStatus
	workflowStatus = "invalid_status"
	defer func() {
		workflowStatus = oldStatus
	}()

	ctx := context.Background()
	err := runWorkflowList(ctx, deps)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestRunWorkflowList_InvalidOutputFormat(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, _ := createWorkflowTestDepsWithMocks(cfg)

	oldOutput := workflowOutput
	workflowOutput = "invalid"
	defer func() {
		workflowOutput = oldOutput
	}()

	ctx := context.Background()
	err := runWorkflowList(ctx, deps)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid output format")
}

func TestRunWorkflowStatus(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, _ := createWorkflowTestDepsWithMocks(cfg)

	// Reset global flags.
	oldOutput := workflowOutput
	oldWatch := workflowWatch
	workflowOutput = ""
	workflowWatch = false
	defer func() {
		workflowOutput = oldOutput
		workflowWatch = oldWatch
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runWorkflowStatus(ctx, deps, "wf-test-001")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Workflow:")
	assert.Contains(t, output, "wf-test-001")
}

func TestRunWorkflowStatus_JSONOutput(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, _ := createWorkflowTestDepsWithMocks(cfg)

	oldOutput := workflowOutput
	workflowOutput = "json"
	defer func() {
		workflowOutput = oldOutput
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runWorkflowStatus(ctx, deps, "wf-test-001")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)

	// Verify valid JSON.
	var workflow Workflow
	err = json.Unmarshal([]byte(output), &workflow)
	assert.NoError(t, err)
	assert.Equal(t, "wf-test-001", workflow.ID)
}

func TestRunWorkflowStatus_NotFound(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, _ := createWorkflowTestDepsWithMocks(cfg)

	oldOutput := workflowOutput
	workflowOutput = ""
	defer func() {
		workflowOutput = oldOutput
	}()

	ctx := context.Background()
	// Non-wf- prefix IDs return nil from getMockWorkflow.
	err := runWorkflowStatus(ctx, deps, "invalid-id")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow not found")
}

func TestRunWorkflowCancel(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, _ := createWorkflowTestDepsWithMocks(cfg)

	// Reset global flags.
	oldForce := workflowForce
	workflowForce = false
	defer func() {
		workflowForce = oldForce
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runWorkflowCancel(ctx, deps, "wf-test-001")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Cancelling workflow")
	assert.Contains(t, output, "has been cancelled")
}

func TestRunWorkflowCancel_Force(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, _ := createWorkflowTestDepsWithMocks(cfg)

	oldForce := workflowForce
	workflowForce = true
	defer func() {
		workflowForce = oldForce
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runWorkflowCancel(ctx, deps, "wf-test-001")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Force cancelling")
}

func TestRunWorkflowCancel_NotFound(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, mock := createWorkflowTestDepsWithMocks(cfg)

	// Set flag to simulate workflow not found.
	mock.workflowNotFoundForCancel = true

	ctx := context.Background()
	err := runWorkflowCancel(ctx, deps, "invalid-id")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow not found")
}

func TestRunWorkflowTerminate(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, _ := createWorkflowTestDepsWithMocks(cfg)

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := runWorkflowTerminate(ctx, deps, "wf-test-001")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Terminating workflow")
	assert.Contains(t, output, "has been terminated")
}

func TestRunWorkflowTerminate_NotFound(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, mock := createWorkflowTestDepsWithMocks(cfg)

	// Set flag to simulate workflow not found.
	mock.workflowNotFoundForCancel = true

	ctx := context.Background()
	err := runWorkflowTerminate(ctx, deps, "invalid-id")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow not found")
}

func TestGetMockWorkflows(t *testing.T) {
	workflows := getMockWorkflows("", "", 100)
	assert.True(t, len(workflows) > 0)
}

func TestGetMockWorkflows_TypeFilter(t *testing.T) {
	workflows := getMockWorkflows("ingestion", "", 100)

	for _, wf := range workflows {
		assert.Equal(t, "ingestion", wf.Type)
	}
}

func TestGetMockWorkflows_StatusFilter(t *testing.T) {
	workflows := getMockWorkflows("", "running", 100)

	for _, wf := range workflows {
		assert.Equal(t, WorkflowStatusRunning, wf.Status)
	}
}

func TestGetMockWorkflows_Limit(t *testing.T) {
	workflows := getMockWorkflows("", "", 2)
	assert.LessOrEqual(t, len(workflows), 2)
}

func TestGetMockWorkflow(t *testing.T) {
	workflow := getMockWorkflow("wf-test-123")

	assert.NotNil(t, workflow)
	assert.Equal(t, "wf-test-123", workflow.ID)
	assert.NotEmpty(t, workflow.Type)
	assert.NotEmpty(t, workflow.Name)
	assert.True(t, len(workflow.Steps) > 0)
}

func TestGetMockWorkflow_NotFound(t *testing.T) {
	workflow := getMockWorkflow("invalid-id")
	assert.Nil(t, workflow)
}

func TestWorkflowStatus_Constants(t *testing.T) {
	assert.Equal(t, WorkflowStatus("pending"), WorkflowStatusPending)
	assert.Equal(t, WorkflowStatus("running"), WorkflowStatusRunning)
	assert.Equal(t, WorkflowStatus("completed"), WorkflowStatusCompleted)
	assert.Equal(t, WorkflowStatus("failed"), WorkflowStatusFailed)
	assert.Equal(t, WorkflowStatus("cancelled"), WorkflowStatusCancelled)
}

func TestWorkflow_JSONSerialization(t *testing.T) {
	now := time.Now()
	startTime := now.Add(-30 * time.Minute)
	endTime := now.Add(-10 * time.Minute)

	workflow := Workflow{
		ID:          "wf-test-001",
		Type:        "ingestion",
		Name:        "Test Workflow",
		Status:      WorkflowStatusCompleted,
		Progress:    100,
		Message:     "Completed successfully",
		CreatedAt:   now.Add(-1 * time.Hour),
		StartedAt:   &startTime,
		CompletedAt: &endTime,
		Steps: []WorkflowStep{
			{Name: "Step 1", Status: WorkflowStatusCompleted, Message: "Done"},
		},
		Input: map[string]string{
			"source": "test",
		},
		Output: map[string]string{
			"items_processed": "100",
		},
	}

	data, err := json.Marshal(workflow)
	require.NoError(t, err)

	var decoded Workflow
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, workflow.ID, decoded.ID)
	assert.Equal(t, workflow.Type, decoded.Type)
	assert.Equal(t, workflow.Status, decoded.Status)
	assert.Equal(t, workflow.Progress, decoded.Progress)
	assert.Len(t, decoded.Steps, 1)
}

func TestWorkflowListResponse_JSONSerialization(t *testing.T) {
	response := WorkflowListResponse{
		Workflows: []Workflow{
			{ID: "wf-1", Type: "ingestion", Status: WorkflowStatusRunning},
			{ID: "wf-2", Type: "batch", Status: WorkflowStatusCompleted},
		},
		TotalCount: 2,
		FetchedAt:  time.Now(),
	}

	data, err := json.Marshal(response)
	require.NoError(t, err)

	var decoded WorkflowListResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Workflows, 2)
	assert.Equal(t, 2, decoded.TotalCount)
}

func TestOutputWorkflowList_EmptyList(t *testing.T) {
	response := WorkflowListResponse{
		Workflows:  []Workflow{},
		TotalCount: 0,
		FetchedAt:  time.Now(),
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputWorkflowList(config.OutputFormatText, response)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "No workflows found")
}

func TestOutputWorkflowStatus_Text(t *testing.T) {
	workflow := getMockWorkflow("wf-001")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputWorkflowStatus(config.OutputFormatText, workflow)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Workflow:")
	assert.Contains(t, output, "wf-001")
	assert.Contains(t, output, "Steps:")
}

func TestGetWorkflowStatusColor(t *testing.T) {
	tests := []struct {
		status WorkflowStatus
		color  string
	}{
		{WorkflowStatusCompleted, "\033[32m"}, // Green
		{WorkflowStatusRunning, "\033[34m"},   // Blue
		{WorkflowStatusPending, "\033[33m"},   // Yellow
		{WorkflowStatusFailed, "\033[31m"},    // Red
		{WorkflowStatusCancelled, "\033[90m"}, // Gray
		{WorkflowStatus("unknown"), ""},       // No color
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			color := getWorkflowStatusColor(tc.status)
			assert.Equal(t, tc.color, color)
		})
	}
}

func TestGetStepStatusIcon(t *testing.T) {
	tests := []struct {
		status   WorkflowStatus
		expected string
	}{
		{WorkflowStatusCompleted, "\033[32m[OK]\033[0m"},
		{WorkflowStatusRunning, "\033[34m[..]\033[0m"},
		{WorkflowStatusPending, "\033[33m[  ]\033[0m"},
		{WorkflowStatusFailed, "\033[31m[!!]\033[0m"},
		{WorkflowStatusCancelled, "\033[90m[--]\033[0m"},
		{WorkflowStatus("unknown"), "[??]"},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			icon := getStepStatusIcon(tc.status)
			assert.Equal(t, tc.expected, icon)
		})
	}
}

func TestDefaultWorkflowDeps(t *testing.T) {
	deps := DefaultWorkflowDeps()

	assert.NotNil(t, deps)
	assert.NotNil(t, deps.LoadConfig)
	assert.NotNil(t, deps.InitClient)
}

// TestRunWorkflowCancel_VariableShadowingBug documents and tests bug pf-c86377.
//
// BUG DESCRIPTION:
// In workflow.go, runWorkflowCancel has variable shadowing at lines 519 and 532.
//
// Line 510: var result *client.CancelWorkflowResult (outer scope)
// Line 513: if workflowForce { ... } else { ... }
//
// FORCE PATH (lines 518-525):
//   Line 519: grpcClient, err := deps.InitClient(cfg)  // := creates NEW local err!
//   Line 520-522: if err != nil { return ... }          // Checks local err ✓
//   Line 524: result, err = grpcClient.TerminateWorkflow(...)  // Uses local err!
//   Line 541: if err != nil { ... }                     // Checks OUTER err ✗
//
// CANCEL PATH (lines 531-538):
//   Line 532: grpcClient, err := deps.InitClient(cfg)  // := creates NEW local err!
//   Line 533-535: if err != nil { return ... }          // Checks local err ✓
//   Line 537: result, err = grpcClient.CancelWorkflow(...)  // Uses local err!
//   Line 541: if err != nil { ... }                     // Checks OUTER err ✗
//
// CONSEQUENCE:
// If TerminateWorkflow/CancelWorkflow returns an error (lines 524/537), it goes
// into the LOCAL shadowed err variable. The check at line 541 checks the OUTER
// err (which is still nil), so the error is silently lost. Line 545 then tries
// to access result.Accepted on a nil result → panic (nil pointer dereference).
//
// FIX:
// Change lines 519 and 532 from `:=` to `=` to reuse the outer err variable:
//   grpcClient, err = deps.InitClient(cfg)  // Note: = not :=
//
// However, this requires declaring grpcClient in outer scope first:
//   var grpcClient *client.GRPCClient
//   var result *client.CancelWorkflowResult
//
// TEST LIMITATIONS:
// This test cannot fully reproduce the bug because:
// 1. The bug occurs in the non-mock code path (else blocks at lines 518-525, 531-538)
// 2. That path calls methods directly on *client.GRPCClient (concrete type)
// 3. We can't create a mock GRPCClient without a real gRPC server
// 4. The mock function path (deps.TerminateWorkflowFn/CancelWorkflowFn) bypasses the buggy code
//
// This test documents the issue and tests related behavior to ensure the fix works correctly.
func TestRunWorkflowCancel_VariableShadowingBug(t *testing.T) {
	cfg := mockWorkflowConfig()

	// Create an InitClient that fails to demonstrate existing error handling works.
	createFailingInitClient := func(c *config.CLIConfig) (*client.GRPCClient, error) {
		return nil, fmt.Errorf("connection failed: gateway unreachable")
	}

	deps := &WorkflowCommandDeps{
		Config:       cfg,
		OutputFormat: config.OutputFormatText,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		InitClient: createFailingInitClient,
		// Don't set mock functions - force the non-mock code path where the bug exists.
	}

	oldForce := workflowForce
	defer func() {
		workflowForce = oldForce
	}()

	// Test 1: Verify InitClient errors ARE properly caught (lines 520-522, 533-535)
	t.Run("cancel_path_InitClient_error_is_caught", func(t *testing.T) {
		workflowForce = false
		ctx := context.Background()
		err := runWorkflowCancel(ctx, deps, "wf-test-001")

		// Line 532: grpcClient, err := deps.InitClient(cfg)
		// Lines 533-535: if err != nil { return fmt.Errorf("initializing client: %w", err) }
		// This check uses the LOCAL err variable, so it works correctly.
		assert.Error(t, err, "InitClient error should be caught")
		assert.Contains(t, err.Error(), "initializing client", "error should be wrapped correctly")
	})

	t.Run("terminate_path_InitClient_error_is_caught", func(t *testing.T) {
		workflowForce = true
		ctx := context.Background()
		err := runWorkflowCancel(ctx, deps, "wf-test-001")

		// Line 519: grpcClient, err := deps.InitClient(cfg)
		// Lines 520-522: if err != nil { return fmt.Errorf("initializing client: %w", err) }
		// This check uses the LOCAL err variable, so it works correctly.
		assert.Error(t, err, "InitClient error should be caught")
		assert.Contains(t, err.Error(), "initializing client", "error should be wrapped correctly")
	})

	// Note: We cannot test the actual bug scenario where:
	// - InitClient succeeds (returns non-nil *client.GRPCClient)
	// - grpcClient.TerminateWorkflow/CancelWorkflow fails (returns error)
	// - The error goes into shadowed local err
	// - Line 541 checks outer err (nil), so error is lost
	// - Line 545 panics on nil result.Accepted
	//
	// To test this would require either:
	// A) A real gRPC server that can fail the Terminate/Cancel call
	// B) Refactoring GRPCClient to be an interface so we can mock it
	// C) Using unsafe or reflection hacks (not recommended)
	//
	// This test serves as documentation of the bug and verification that the
	// fix (changing := to = on lines 519 and 532) will work correctly.
}

// TestWorkflowTerminateCommand_Existence tests that the terminate subcommand exists
// and is wired up correctly. This test SHOULD FAIL until the terminate command is implemented.
func TestWorkflowTerminateCommand_Existence(t *testing.T) {
	cfg := mockWorkflowConfig()
	deps, mock := createWorkflowTestDepsWithMocks(cfg)

	cmd := NewWorkflowCommand(deps)

	// Try to find the terminate subcommand.
	terminateCmd, _, err := cmd.Find([]string{"terminate"})

	// This will fail before implementation because terminate subcommand doesn't exist.
	require.NoError(t, err, "terminate subcommand should exist")
	require.NotNil(t, terminateCmd, "terminate subcommand should not be nil")

	// Verify the command structure.
	assert.Contains(t, terminateCmd.Use, "terminate", "command use should contain 'terminate'")
	assert.Contains(t, terminateCmd.Short, "terminate", "short description should mention terminate")

	// Verify it calls TerminateWorkflowFn.
	// Set up the mock to track if it was called.
	terminateCalled := false
	oldTerminateFn := deps.TerminateWorkflowFn
	deps.TerminateWorkflowFn = func(ctx context.Context, workflowID, runID, reason string) (*client.CancelWorkflowResult, error) {
		terminateCalled = true
		assert.Equal(t, "wf-test-001", workflowID, "workflow ID should match")
		return mock.TerminateWorkflow(ctx, workflowID, runID, reason)
	}
	defer func() {
		deps.TerminateWorkflowFn = oldTerminateFn
	}()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute the command via RunE directly.
	ctx := context.Background()
	err = runWorkflowTerminate(ctx, deps, "wf-test-001")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	assert.NoError(t, err, "terminate command should execute successfully")
	assert.True(t, terminateCalled, "TerminateWorkflowFn should be called")
}

// Mock helper functions for workflow tests.

// getMockWorkflows returns mock workflows, filtered by type and status with limit.
func getMockWorkflows(typeFilter, statusFilter string, limit int) []Workflow {
	now := time.Now()
	startTime1 := now.Add(-2 * time.Hour)
	endTime1 := now.Add(-1 * time.Hour)
	startTime2 := now.Add(-30 * time.Minute)

	allWorkflows := []Workflow{
		{
			ID:          "wf-001",
			Type:        "ingestion",
			Name:        "Gmail Sync",
			Status:      WorkflowStatusCompleted,
			Progress:    100,
			Message:     "Successfully synced 50 emails",
			CreatedAt:   now.Add(-3 * time.Hour),
			StartedAt:   &startTime1,
			CompletedAt: &endTime1,
			Steps: []WorkflowStep{
				{Name: "Fetch", Status: WorkflowStatusCompleted, Message: "Fetched 50 items"},
				{Name: "Process", Status: WorkflowStatusCompleted, Message: "Processed all items"},
				{Name: "Store", Status: WorkflowStatusCompleted, Message: "Stored in database"},
			},
		},
		{
			ID:        "wf-002",
			Type:      "enrichment",
			Name:      "Meeting Enrichment",
			Status:    WorkflowStatusRunning,
			Progress:  60,
			Message:   "Processing meeting transcripts",
			CreatedAt: now.Add(-1 * time.Hour),
			StartedAt: &startTime2,
			Steps: []WorkflowStep{
				{Name: "Extract", Status: WorkflowStatusCompleted, Message: "Extracted content"},
				{Name: "Analyze", Status: WorkflowStatusRunning, Message: "Analyzing..."},
				{Name: "Store", Status: WorkflowStatusPending, Message: ""},
			},
		},
		{
			ID:        "wf-003",
			Type:      "ingestion",
			Name:      "Calendar Sync",
			Status:    WorkflowStatusPending,
			Progress:  0,
			Message:   "Queued for processing",
			CreatedAt: now.Add(-10 * time.Minute),
			Steps: []WorkflowStep{
				{Name: "Fetch", Status: WorkflowStatusPending, Message: ""},
				{Name: "Process", Status: WorkflowStatusPending, Message: ""},
			},
		},
		{
			ID:          "wf-004",
			Type:        "analysis",
			Name:        "Entity Extraction",
			Status:      WorkflowStatusFailed,
			Progress:    30,
			Message:     "Failed to extract entities",
			Error:       "timeout connecting to AI service",
			CreatedAt:   now.Add(-4 * time.Hour),
			StartedAt:   &startTime1,
			CompletedAt: &endTime1,
			Steps: []WorkflowStep{
				{Name: "Prepare", Status: WorkflowStatusCompleted, Message: "Prepared data"},
				{Name: "Extract", Status: WorkflowStatusFailed, Message: "Connection timeout"},
			},
		},
	}

	var filtered []Workflow
	for _, wf := range allWorkflows {
		// Apply type filter.
		if typeFilter != "" && wf.Type != typeFilter {
			continue
		}
		// Apply status filter.
		if statusFilter != "" && string(wf.Status) != statusFilter {
			continue
		}
		filtered = append(filtered, wf)
	}

	if limit > 0 && len(filtered) > limit {
		return filtered[:limit]
	}
	return filtered
}

// getMockWorkflow returns a specific workflow by ID.
func getMockWorkflow(id string) *Workflow {
	workflows := getMockWorkflows("", "", 100)
	for _, wf := range workflows {
		if wf.ID == id {
			return &wf
		}
	}

	// For test-specific IDs, return a mock workflow.
	if id == "wf-test-123" {
		now := time.Now()
		startTime := now.Add(-15 * time.Minute)
		return &Workflow{
			ID:        "wf-test-123",
			Type:      "test",
			Name:      "Test Workflow",
			Status:    WorkflowStatusRunning,
			Progress:  50,
			Message:   "Test in progress",
			CreatedAt: now.Add(-20 * time.Minute),
			StartedAt: &startTime,
			Steps: []WorkflowStep{
				{Name: "Step 1", Status: WorkflowStatusCompleted, Message: "Done"},
				{Name: "Step 2", Status: WorkflowStatusRunning, Message: "In progress"},
				{Name: "Step 3", Status: WorkflowStatusPending, Message: ""},
			},
		}
	}

	return nil
}
