// Package client provides the gRPC client for connecting to the Penfold API Gateway.
// This file contains ContentProcessorService client methods.
package client

import (
	"context"
	"fmt"

	contentv1 "github.com/otherjamesbrown/penf-cli/api/proto/content/v1"
)

// =============================================================================
// ContentProcessorService Client Methods
// =============================================================================

// ContentProcessorServiceClient returns a ContentProcessorService gRPC client.
// Returns an error if not connected.
func (c *GRPCClient) ContentProcessorServiceClient() (contentv1.ContentProcessorServiceClient, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return nil, fmt.Errorf("not connected to gateway")
	}

	return contentv1.NewContentProcessorServiceClient(conn), nil
}

// ReprocessContent triggers reprocessing of an already-processed content item.
func (c *GRPCClient) ReprocessContent(ctx context.Context, req *contentv1.ReprocessContentRequest) (*contentv1.ReprocessContentResponse, error) {
	client, err := c.ContentProcessorServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.ReprocessContent(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ReprocessContent RPC failed: %w", err)
	}

	return resp, nil
}

// ProcessContent triggers the content processing pipeline for a specific item.
func (c *GRPCClient) ProcessContent(ctx context.Context, req *contentv1.ProcessContentRequest) (*contentv1.ProcessContentResponse, error) {
	client, err := c.ContentProcessorServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.ProcessContent(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ProcessContent RPC failed: %w", err)
	}

	return resp, nil
}

// GetProcessingStatus retrieves the current processing status of a content item.
func (c *GRPCClient) GetProcessingStatus(ctx context.Context, contentID string, jobID string) (*contentv1.ProcessingStatus, error) {
	client, err := c.ContentProcessorServiceClient()
	if err != nil {
		return nil, err
	}

	req := &contentv1.GetProcessingStatusRequest{
		ContentId: contentID,
	}
	if jobID != "" {
		req.JobId = &jobID
	}

	resp, err := client.GetProcessingStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetProcessingStatus RPC failed: %w", err)
	}

	return resp, nil
}

// GetContentItem retrieves a specific content item by ID.
func (c *GRPCClient) GetContentItem(ctx context.Context, contentID string, includeEmbedding bool) (*contentv1.ContentItem, error) {
	client, err := c.ContentProcessorServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetContentItem(ctx, &contentv1.GetContentItemRequest{
		ContentId:        contentID,
		IncludeEmbedding: includeEmbedding,
	})
	if err != nil {
		return nil, fmt.Errorf("GetContentItem RPC failed: %w", err)
	}

	return resp, nil
}

// ListContentItems returns a paginated list of content items.
func (c *GRPCClient) ListContentItems(ctx context.Context, req *contentv1.ListContentItemsRequest) (*contentv1.ListContentItemsResponse, error) {
	client, err := c.ContentProcessorServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.ListContentItems(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ListContentItems RPC failed: %w", err)
	}

	return resp, nil
}

// DeleteContentItem removes a content item and all derived data.
func (c *GRPCClient) DeleteContentItem(ctx context.Context, contentID string) (*contentv1.DeleteContentItemResponse, error) {
	client, err := c.ContentProcessorServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.DeleteContentItem(ctx, &contentv1.DeleteContentItemRequest{
		ContentId: contentID,
	})
	if err != nil {
		return nil, fmt.Errorf("DeleteContentItem RPC failed: %w", err)
	}

	return resp, nil
}

// DeleteContentItems bulk deletes content items matching filters.
func (c *GRPCClient) DeleteContentItems(ctx context.Context, req *contentv1.DeleteContentItemsRequest) (*contentv1.DeleteContentItemsResponse, error) {
	client, err := c.ContentProcessorServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.DeleteContentItems(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("DeleteContentItems RPC failed: %w", err)
	}

	return resp, nil
}

// GetContentStats returns content statistics.
func (c *GRPCClient) GetContentStats(ctx context.Context, tenantID string) (*contentv1.ContentStats, error) {
	client, err := c.ContentProcessorServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetContentStats(ctx, &contentv1.GetContentStatsRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetContentStats RPC failed: %w", err)
	}

	return resp, nil
}
