// Package client provides the gRPC client for connecting to the Penfold API Gateway.
// This file contains LedgerService client methods.
package client

import (
	"context"
	"fmt"

	ledgerv1 "github.com/otherjamesbrown/penf-cli/api/proto/ledger/v1"
)

// =============================================================================
// LedgerService Client Methods
// =============================================================================

// LedgerServiceClient returns a LedgerService gRPC client.
// Returns an error if not connected.
func (c *GRPCClient) LedgerServiceClient() (ledgerv1.LedgerServiceClient, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return nil, fmt.Errorf("not connected to gateway")
	}

	return ledgerv1.NewLedgerServiceClient(conn), nil
}

// CreateLedgerEntry appends an immutable entry to the session ledger.
func (c *GRPCClient) CreateLedgerEntry(ctx context.Context, req *ledgerv1.CreateEntryRequest) (*ledgerv1.CreateEntryResponse, error) {
	client, err := c.LedgerServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.CreateEntry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("CreateEntry RPC failed: %w", err)
	}

	return resp, nil
}

// GetLedgerEntry retrieves a single ledger entry by ID.
func (c *GRPCClient) GetLedgerEntry(ctx context.Context, req *ledgerv1.GetEntryRequest) (*ledgerv1.LedgerEntry, error) {
	client, err := c.LedgerServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetEntry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetEntry RPC failed: %w", err)
	}

	return resp, nil
}

// ListLedgerEntries lists ledger entries with filtering.
func (c *GRPCClient) ListLedgerEntries(ctx context.Context, req *ledgerv1.ListEntriesRequest) (*ledgerv1.ListEntriesResponse, error) {
	client, err := c.LedgerServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.ListEntries(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ListEntries RPC failed: %w", err)
	}

	return resp, nil
}

// ListLedgerSessions returns session summaries aggregated from entries.
func (c *GRPCClient) ListLedgerSessions(ctx context.Context, req *ledgerv1.ListSessionsRequest) (*ledgerv1.ListSessionsResponse, error) {
	client, err := c.LedgerServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.ListSessions(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ListSessions RPC failed: %w", err)
	}

	return resp, nil
}

// GetLedgerSession returns all entries for a specific session.
func (c *GRPCClient) GetLedgerSession(ctx context.Context, req *ledgerv1.GetSessionRequest) (*ledgerv1.GetSessionResponse, error) {
	client, err := c.LedgerServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetSession RPC failed: %w", err)
	}

	return resp, nil
}

// SearchLedgerEntries performs full-text search across ledger entries.
func (c *GRPCClient) SearchLedgerEntries(ctx context.Context, req *ledgerv1.SearchEntriesRequest) (*ledgerv1.SearchEntriesResponse, error) {
	client, err := c.LedgerServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.SearchEntries(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("SearchEntries RPC failed: %w", err)
	}

	return resp, nil
}

// GetLatestHandoff returns the most recent handoff entry.
func (c *GRPCClient) GetLatestHandoff(ctx context.Context, req *ledgerv1.GetLatestHandoffRequest) (*ledgerv1.LedgerEntry, error) {
	client, err := c.LedgerServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetLatestHandoff(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetLatestHandoff RPC failed: %w", err)
	}

	return resp, nil
}

// ListLedgerConsolidations lists consolidated narratives.
func (c *GRPCClient) ListLedgerConsolidations(ctx context.Context, req *ledgerv1.ListConsolidationsRequest) (*ledgerv1.ListConsolidationsResponse, error) {
	client, err := c.LedgerServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.ListConsolidations(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ListConsolidations RPC failed: %w", err)
	}

	return resp, nil
}

// GetLedgerConsolidation retrieves a single consolidation by ID.
func (c *GRPCClient) GetLedgerConsolidation(ctx context.Context, req *ledgerv1.GetConsolidationRequest) (*ledgerv1.Consolidation, error) {
	client, err := c.LedgerServiceClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetConsolidation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetConsolidation RPC failed: %w", err)
	}

	return resp, nil
}
