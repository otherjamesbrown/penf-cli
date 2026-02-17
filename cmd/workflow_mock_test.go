// Package cmd provides CLI commands for the penf tool.
// This file contains mock implementations for workflow client testing.
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// mockWorkflowClient is a mock implementation of the workflow client methods.
type mockWorkflowClient struct {
	// workflows stores mock workflow data keyed by workflow ID.
	workflows map[string]*Workflow

	// Flags to simulate various error conditions.
	shouldFailListWorkflows      bool
	shouldFailGetWorkflowStatus  bool
	shouldFailCancelWorkflow     bool
	shouldFailTerminateWorkflow  bool
	workflowNotFoundForStatus    bool
	workflowNotFoundForCancel    bool
}

// newMockWorkflowClient creates a new mock workflow client with default test data.
func newMockWorkflowClient() *mockWorkflowClient {
	workflows := make(map[string]*Workflow)

	// Populate with mock data from getMockWorkflows.
	for _, wf := range getMockWorkflows("", "", 100) {
		workflows[wf.ID] = &wf
	}

	// Add the special test workflow used by many tests.
	now := time.Now()
	startTime := now.Add(-15 * time.Minute)
	testWf := &Workflow{
		ID:        "wf-test-001",
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
	workflows["wf-test-001"] = testWf

	return &mockWorkflowClient{
		workflows: workflows,
	}
}

// ListWorkflows mocks the ListWorkflows client method.
func (m *mockWorkflowClient) ListWorkflows(ctx context.Context, filter client.ListWorkflowsFilter) (*client.ListWorkflowsResult, error) {
	if m.shouldFailListWorkflows {
		return nil, fmt.Errorf("mock: list workflows failed")
	}

	var result []client.WorkflowInfo

	for _, wf := range m.workflows {
		// Apply filters.
		if filter.WorkflowType != "" && wf.Type != filter.WorkflowType {
			continue
		}
		if filter.Status != "" && string(wf.Status) != filter.Status {
			continue
		}

		info := client.WorkflowInfo{
			WorkflowID:   wf.ID,
			WorkflowType: wf.Type,
			Status:       string(wf.Status),
			StartTime:    wf.CreatedAt,
		}
		if wf.StartedAt != nil {
			info.StartTime = *wf.StartedAt
		}
		if wf.CompletedAt != nil {
			info.CloseTime = *wf.CompletedAt
		}

		result = append(result, info)
	}

	// Apply page size limit.
	if filter.PageSize > 0 && int32(len(result)) > filter.PageSize {
		result = result[:filter.PageSize]
	}

	return &client.ListWorkflowsResult{
		Workflows:     result,
		NextPageToken: nil,
	}, nil
}

// GetWorkflowStatus mocks the GetWorkflowStatus client method.
func (m *mockWorkflowClient) GetWorkflowStatus(ctx context.Context, workflowID, runID string) (*client.WorkflowStatusDetails, error) {
	if m.shouldFailGetWorkflowStatus {
		return nil, fmt.Errorf("mock: get workflow status failed")
	}

	if m.workflowNotFoundForStatus {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	wf, exists := m.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	details := &client.WorkflowStatusDetails{
		WorkflowInfo: client.WorkflowInfo{
			WorkflowID:   wf.ID,
			WorkflowType: wf.Type,
			Status:       string(wf.Status),
			StartTime:    wf.CreatedAt,
		},
		PendingActivities: int32(countPendingSteps(wf.Steps)),
		SearchAttributes:  wf.Input,
	}

	if wf.StartedAt != nil {
		details.StartTime = *wf.StartedAt
	}
	if wf.CompletedAt != nil {
		details.CloseTime = *wf.CompletedAt
		if wf.StartedAt != nil {
			duration := wf.CompletedAt.Sub(*wf.StartedAt)
			details.ExecutionDurationMs = duration.Milliseconds()
		}
	}

	return details, nil
}

// CancelWorkflow mocks the CancelWorkflow client method.
func (m *mockWorkflowClient) CancelWorkflow(ctx context.Context, workflowID, runID, reason string) (*client.CancelWorkflowResult, error) {
	if m.shouldFailCancelWorkflow {
		return nil, fmt.Errorf("mock: cancel workflow failed")
	}

	if m.workflowNotFoundForCancel {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	wf, exists := m.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Check if workflow is in a cancellable state.
	if wf.Status == WorkflowStatusCompleted || wf.Status == WorkflowStatusFailed || wf.Status == WorkflowStatusCancelled {
		return &client.CancelWorkflowResult{
			Accepted: false,
			Message:  fmt.Sprintf("Workflow %s is already in terminal state: %s", workflowID, wf.Status),
		}, nil
	}

	// Update workflow status to cancelled.
	wf.Status = WorkflowStatusCancelled
	now := time.Now()
	wf.CompletedAt = &now
	wf.Message = reason

	return &client.CancelWorkflowResult{
		Accepted: true,
		Message:  fmt.Sprintf("Workflow %s has been cancelled", workflowID),
	}, nil
}

// TerminateWorkflow mocks the TerminateWorkflow client method.
func (m *mockWorkflowClient) TerminateWorkflow(ctx context.Context, workflowID, runID, reason string) (*client.CancelWorkflowResult, error) {
	if m.shouldFailTerminateWorkflow {
		return nil, fmt.Errorf("mock: terminate workflow failed")
	}

	if m.workflowNotFoundForCancel {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	wf, exists := m.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Terminate always succeeds regardless of state.
	wf.Status = WorkflowStatusCancelled
	now := time.Now()
	wf.CompletedAt = &now
	wf.Message = reason

	return &client.CancelWorkflowResult{
		Accepted: true,
		Message:  fmt.Sprintf("Workflow %s has been terminated", workflowID),
	}, nil
}

// Close mocks the Close method (no-op for mock).
func (m *mockWorkflowClient) Close() error {
	return nil
}

// countPendingSteps counts the number of pending or running steps in a workflow.
func countPendingSteps(steps []WorkflowStep) int {
	count := 0
	for _, step := range steps {
		if step.Status == WorkflowStatusPending || step.Status == WorkflowStatusRunning {
			count++
		}
	}
	return count
}

// createWorkflowTestDepsWithMocks creates test dependencies with mock workflow client functions.
func createWorkflowTestDepsWithMocks(cfg *config.CLIConfig) (*WorkflowCommandDeps, *mockWorkflowClient) {
	mock := newMockWorkflowClient()

	deps := &WorkflowCommandDeps{
		Config:       cfg,
		OutputFormat: config.OutputFormatText,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		InitClient: func(c *config.CLIConfig) (*client.GRPCClient, error) {
			// This won't be called when mock functions are set.
			return nil, fmt.Errorf("mock: InitClient should not be called")
		},
		// Wire up mock functions.
		ListWorkflowsFn:     mock.ListWorkflows,
		GetWorkflowStatusFn: mock.GetWorkflowStatus,
		CancelWorkflowFn:    mock.CancelWorkflow,
		TerminateWorkflowFn: mock.TerminateWorkflow,
	}

	return deps, mock
}
