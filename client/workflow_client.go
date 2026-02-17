// Package client provides the gRPC client for connecting to the Penfold API Gateway.
// This file contains WorkflowService client methods for managing Temporal workflows.
package client

import (
	"context"
	"fmt"
	"time"

	workflowv1 "github.com/otherjamesbrown/penf-cli/api/proto/workflow/v1"
)

// =============================================================================
// WorkflowService Client Methods
// =============================================================================

// WorkflowServiceClient returns a WorkflowService gRPC client.
// Returns an error if not connected.
func (c *GRPCClient) WorkflowServiceClient() (workflowv1.WorkflowServiceClient, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return nil, fmt.Errorf("not connected to gateway")
	}

	return workflowv1.NewWorkflowServiceClient(conn), nil
}

// WorkflowInfo represents workflow information returned from listing.
type WorkflowInfo struct {
	WorkflowID   string
	RunID        string
	WorkflowType string
	Status       string
	TaskQueue    string
	StartTime    time.Time
	CloseTime    time.Time
}

// WorkflowStatusDetails represents detailed workflow status.
type WorkflowStatusDetails struct {
	WorkflowInfo
	PendingActivities   int32
	PendingChildren     int32
	HistoryLength       int64
	ExecutionDurationMs int64
	Memo                map[string]string
	SearchAttributes    map[string]string
}

// ListWorkflowsFilter contains filtering options for listing workflows.
type ListWorkflowsFilter struct {
	WorkflowType string
	Status       string
	PageSize     int32
	PageToken    []byte
	StartTimeMin time.Time
	StartTimeMax time.Time
	Query        string
}

// ListWorkflowsResult contains the results of listing workflows.
type ListWorkflowsResult struct {
	Workflows     []WorkflowInfo
	NextPageToken []byte
}

// ListWorkflows lists workflows matching the given filter criteria.
func (c *GRPCClient) ListWorkflows(ctx context.Context, filter ListWorkflowsFilter) (*ListWorkflowsResult, error) {
	client, err := c.WorkflowServiceClient()
	if err != nil {
		return nil, err
	}

	req := &workflowv1.ListWorkflowsRequest{
		WorkflowType:  filter.WorkflowType,
		PageSize:      filter.PageSize,
		NextPageToken: filter.PageToken,
		Query:         filter.Query,
	}

	// Map status string to proto enum
	if filter.Status != "" {
		req.Status = mapStatusStringToProto(filter.Status)
	}

	resp, err := client.ListWorkflows(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ListWorkflows RPC failed: %w", err)
	}

	result := &ListWorkflowsResult{
		Workflows:     make([]WorkflowInfo, 0, len(resp.GetWorkflows())),
		NextPageToken: resp.GetNextPageToken(),
	}

	for _, wf := range resp.GetWorkflows() {
		info := WorkflowInfo{
			WorkflowID:   wf.GetWorkflowId(),
			RunID:        wf.GetRunId(),
			WorkflowType: wf.GetWorkflowType(),
			Status:       mapProtoStatusToString(wf.GetStatus()),
			TaskQueue:    wf.GetTaskQueue(),
		}
		if wf.GetStartTime() != nil {
			info.StartTime = wf.GetStartTime().AsTime()
		}
		if wf.GetCloseTime() != nil {
			info.CloseTime = wf.GetCloseTime().AsTime()
		}
		result.Workflows = append(result.Workflows, info)
	}

	return result, nil
}

// GetWorkflowStatus retrieves the status of a specific workflow.
func (c *GRPCClient) GetWorkflowStatus(ctx context.Context, workflowID, runID string) (*WorkflowStatusDetails, error) {
	client, err := c.WorkflowServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetWorkflowStatus(ctx, &workflowv1.GetWorkflowStatusRequest{
		WorkflowId: workflowID,
		RunId:      runID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetWorkflowStatus RPC failed: %w", err)
	}

	status := resp.GetStatus()
	if status == nil || status.GetInfo() == nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	details := &WorkflowStatusDetails{
		WorkflowInfo: WorkflowInfo{
			WorkflowID:   status.GetInfo().GetWorkflowId(),
			RunID:        status.GetInfo().GetRunId(),
			WorkflowType: status.GetInfo().GetWorkflowType(),
			Status:       mapProtoStatusToString(status.GetInfo().GetStatus()),
			TaskQueue:    status.GetInfo().GetTaskQueue(),
		},
		PendingActivities:   status.GetPendingActivities(),
		PendingChildren:     status.GetPendingChildren(),
		HistoryLength:       status.GetHistoryLength(),
		ExecutionDurationMs: status.GetExecutionDurationMs(),
		Memo:                status.GetMemo(),
		SearchAttributes:    status.GetSearchAttributes(),
	}

	if status.GetInfo().GetStartTime() != nil {
		details.StartTime = status.GetInfo().GetStartTime().AsTime()
	}
	if status.GetInfo().GetCloseTime() != nil {
		details.CloseTime = status.GetInfo().GetCloseTime().AsTime()
	}

	return details, nil
}

// CancelWorkflowResult contains the result of a cancel request.
type CancelWorkflowResult struct {
	Accepted bool
	Message  string
}

// CancelWorkflow cancels a running workflow.
func (c *GRPCClient) CancelWorkflow(ctx context.Context, workflowID, runID, reason string) (*CancelWorkflowResult, error) {
	client, err := c.WorkflowServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.CancelWorkflow(ctx, &workflowv1.CancelWorkflowRequest{
		WorkflowId: workflowID,
		RunId:      runID,
		Reason:     reason,
	})
	if err != nil {
		return nil, fmt.Errorf("CancelWorkflow RPC failed: %w", err)
	}

	return &CancelWorkflowResult{
		Accepted: resp.GetAccepted(),
		Message:  resp.GetMessage(),
	}, nil
}

// TerminateWorkflow forcefully terminates a workflow.
func (c *GRPCClient) TerminateWorkflow(ctx context.Context, workflowID, runID, reason string) (*CancelWorkflowResult, error) {
	client, err := c.WorkflowServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.TerminateWorkflow(ctx, &workflowv1.TerminateWorkflowRequest{
		WorkflowId: workflowID,
		RunId:      runID,
		Reason:     reason,
	})
	if err != nil {
		return nil, fmt.Errorf("TerminateWorkflow RPC failed: %w", err)
	}

	return &CancelWorkflowResult{
		Accepted: resp.GetAccepted(),
		Message:  resp.GetMessage(),
	}, nil
}

// mapStatusStringToProto maps a status string to proto enum.
func mapStatusStringToProto(status string) workflowv1.WorkflowExecutionStatus {
	switch status {
	case "Running", "running":
		return workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_RUNNING
	case "Completed", "completed":
		return workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_COMPLETED
	case "Failed", "failed":
		return workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_FAILED
	case "Canceled", "canceled", "Cancelled", "cancelled":
		return workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_CANCELED
	case "Terminated", "terminated":
		return workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_TERMINATED
	case "ContinuedAsNew", "continued_as_new":
		return workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW
	case "TimedOut", "timed_out":
		return workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_TIMED_OUT
	default:
		return workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_UNSPECIFIED
	}
}

// mapProtoStatusToString maps proto status enum to human-readable string.
func mapProtoStatusToString(status workflowv1.WorkflowExecutionStatus) string {
	switch status {
	case workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_RUNNING:
		return "Running"
	case workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_COMPLETED:
		return "Completed"
	case workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_FAILED:
		return "Failed"
	case workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_CANCELED:
		return "Canceled"
	case workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_TERMINATED:
		return "Terminated"
	case workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
		return "ContinuedAsNew"
	case workflowv1.WorkflowExecutionStatus_WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return "TimedOut"
	default:
		return "Unknown"
	}
}
