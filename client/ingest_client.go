// Package client provides the gRPC client for connecting to the Penfold API Gateway.
// This file contains IngestService and GmailConnectorService client methods.
package client

import (
	"context"
	"fmt"

	gmailv1 "github.com/otherjamesbrown/penf-cli/api/proto/gmail/v1"
	ingestv1 "github.com/otherjamesbrown/penf-cli/api/proto/ingest/v1"
)

// =============================================================================
// IngestService Client Methods
// =============================================================================

// IngestServiceClient returns an IngestService gRPC client.
// Returns an error if not connected.
func (c *GRPCClient) IngestServiceClient() (ingestv1.IngestServiceClient, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return nil, fmt.Errorf("not connected to gateway")
	}

	return ingestv1.NewIngestServiceClient(conn), nil
}

// GetIngestJob retrieves an ingest job by ID.
func (c *GRPCClient) GetIngestJob(ctx context.Context, tenantID, jobID string) (*ingestv1.IngestJob, error) {
	client, err := c.IngestServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetIngestJob(ctx, &ingestv1.GetIngestJobRequest{
		TenantId: tenantID,
		JobId:    jobID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetIngestJob RPC failed: %w", err)
	}

	return resp.Job, nil
}

// CreateIngestJob creates a new batch ingest job.
func (c *GRPCClient) CreateIngestJob(ctx context.Context, req *ingestv1.CreateIngestJobRequest) (*ingestv1.IngestJob, error) {
	client, err := c.IngestServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.CreateIngestJob(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("CreateIngestJob RPC failed: %w", err)
	}

	return resp.Job, nil
}

// CompleteIngestJob marks a job as completed.
func (c *GRPCClient) CompleteIngestJob(ctx context.Context, jobID string, success bool, errorMessage string) (*ingestv1.IngestJob, error) {
	client, err := c.IngestServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.CompleteIngestJob(ctx, &ingestv1.CompleteIngestJobRequest{
		JobId:        jobID,
		Success:      success,
		ErrorMessage: errorMessage,
	})
	if err != nil {
		return nil, fmt.Errorf("CompleteIngestJob RPC failed: %w", err)
	}

	return resp.Job, nil
}

// UpdateJobProgress updates the progress of an ingest job.
func (c *GRPCClient) UpdateJobProgress(ctx context.Context, jobID string, processed, failed, skipped int64) (*ingestv1.IngestJob, error) {
	client, err := c.IngestServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.UpdateJobProgress(ctx, &ingestv1.UpdateJobProgressRequest{
		JobId:          jobID,
		ProcessedDelta: processed,
		FailedDelta:    failed,
		SkippedDelta:   skipped,
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateJobProgress RPC failed: %w", err)
	}

	return resp.Job, nil
}

// GetRemainingFiles retrieves files that haven't been processed for a job.
func (c *GRPCClient) GetRemainingFiles(ctx context.Context, tenantID, jobID string, limit, offset int32) ([]*ingestv1.RemainingFile, int64, error) {
	client, err := c.IngestServiceClient()
	if err != nil {
		return nil, 0, err
	}

	resp, err := client.GetRemainingFiles(ctx, &ingestv1.GetRemainingFilesRequest{
		TenantId: tenantID,
		JobId:    jobID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetRemainingFiles RPC failed: %w", err)
	}

	return resp.Files, resp.TotalCount, nil
}

// RecordIngestError records an error that occurred during ingestion.
func (c *GRPCClient) RecordIngestError(ctx context.Context, req *ingestv1.RecordIngestErrorRequest) (*ingestv1.IngestError, error) {
	client, err := c.IngestServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.RecordIngestError(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("RecordIngestError RPC failed: %w", err)
	}

	return resp.Error, nil
}

// =============================================================================
// GmailConnectorService Client Methods
// =============================================================================

// GmailConnectorServiceClient returns a GmailConnectorService gRPC client.
// Returns an error if not connected.
func (c *GRPCClient) GmailConnectorServiceClient() (gmailv1.GmailConnectorServiceClient, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return nil, fmt.Errorf("not connected to gateway")
	}

	return gmailv1.NewGmailConnectorServiceClient(conn), nil
}

// SyncGmailEmails triggers a Gmail synchronization.
func (c *GRPCClient) SyncGmailEmails(ctx context.Context, req *gmailv1.SyncEmailsRequest) (*gmailv1.SyncEmailsResponse, error) {
	client, err := c.GmailConnectorServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.SyncEmails(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("SyncEmails RPC failed: %w", err)
	}

	return resp, nil
}

// GetGmailSyncStatus retrieves the status of a Gmail sync operation.
func (c *GRPCClient) GetGmailSyncStatus(ctx context.Context, tenantID, syncID string) (*gmailv1.SyncStatus, error) {
	client, err := c.GmailConnectorServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetSyncStatus(ctx, &gmailv1.GetSyncStatusRequest{
		TenantId: tenantID,
		SyncId:   syncID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetSyncStatus RPC failed: %w", err)
	}

	return resp, nil
}

// ListGmailEmails retrieves a paginated list of Gmail emails.
func (c *GRPCClient) ListGmailEmails(ctx context.Context, req *gmailv1.ListEmailsRequest) (*gmailv1.ListEmailsResponse, error) {
	client, err := c.GmailConnectorServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.ListEmails(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ListEmails RPC failed: %w", err)
	}

	return resp, nil
}
