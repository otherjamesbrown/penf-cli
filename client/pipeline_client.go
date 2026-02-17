// Package client provides the gRPC client for connecting to the Penfold API Gateway.
// This file contains PipelineService client methods.
package client

import (
	"context"
	"fmt"

	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

// =============================================================================
// PipelineService Client Methods
// =============================================================================

// PipelineServiceClient returns a PipelineService gRPC client.
// Returns an error if not connected.
func (c *GRPCClient) PipelineServiceClient() (pipelinev1.PipelineServiceClient, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return nil, fmt.Errorf("not connected to gateway")
	}

	return pipelinev1.NewPipelineServiceClient(conn), nil
}

// GetQueueStatus retrieves processing queue depths and rates.
func (c *GRPCClient) GetQueueStatus(ctx context.Context, stage string) (*pipelinev1.GetQueueStatusResponse, error) {
	client, err := c.PipelineServiceClient()
	if err != nil {
		return nil, err
	}

	req := &pipelinev1.GetQueueStatusRequest{
		Stage: stage,
	}

	resp, err := client.GetQueueStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetQueueStatus RPC failed: %w", err)
	}

	return resp, nil
}

// GetPipelineHealth performs a comprehensive pipeline health check.
func (c *GRPCClient) GetPipelineHealth(ctx context.Context) (*pipelinev1.GetPipelineHealthResponse, error) {
	client, err := c.PipelineServiceClient()
	if err != nil {
		return nil, err
	}

	req := &pipelinev1.GetPipelineHealthRequest{}

	resp, err := client.GetPipelineHealth(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetPipelineHealth RPC failed: %w", err)
	}

	return resp, nil
}

// GetStats retrieves pipeline statistics including source counts, embeddings, and jobs.
func (c *GRPCClient) GetStats(ctx context.Context, tenantID string) (*pipelinev1.GetStatsResponse, error) {
	client, err := c.PipelineServiceClient()
	if err != nil {
		return nil, err
	}

	req := &pipelinev1.GetStatsRequest{
		TenantId: tenantID,
	}

	resp, err := client.GetStats(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetStats RPC failed: %w", err)
	}

	return resp, nil
}

// GetContentTrace retrieves full processing history for a content item.
func (c *GRPCClient) GetContentTrace(ctx context.Context, contentID string, verbose bool) (*pipelinev1.GetContentTraceResponse, error) {
	client, err := c.PipelineServiceClient()
	if err != nil {
		return nil, err
	}

	req := &pipelinev1.GetContentTraceRequest{
		ContentId: contentID,
		Verbose:   verbose,
	}

	resp, err := client.GetContentTrace(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetContentTrace RPC failed: %w", err)
	}

	return resp, nil
}
