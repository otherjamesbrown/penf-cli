// Package client provides gRPC clients for Penfold services.
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	searchv1 "github.com/otherjamesbrown/penf-cli/api/proto/search/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SearchClient manages the connection to the Penfold Search service.
type SearchClient struct {
	conn       *grpc.ClientConn
	client     searchv1.SearchServiceClient
	serverAddr string
	options    *ClientOptions
	mu         sync.RWMutex
	connected  bool
}

// NewSearchClient creates a new SearchClient with the given options.
// Call Connect() to establish the connection.
func NewSearchClient(serverAddr string, opts *ClientOptions) *SearchClient {
	if opts == nil {
		opts = DefaultOptions()
	}

	return &SearchClient{
		serverAddr: serverAddr,
		options:    opts,
	}
}

// Connect establishes a connection to the Search service.
func (c *SearchClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected && c.conn != nil {
		return nil // Already connected.
	}

	// Create connection context with timeout.
	connectCtx, cancel := context.WithTimeout(ctx, c.options.ConnectTimeout)
	defer cancel()

	// Build dial options.
	dialOpts := c.buildDialOptions()

	// Establish connection.
	conn, err := grpc.DialContext(connectCtx, c.serverAddr, dialOpts...)
	if err != nil {
		return fmt.Errorf("connecting to search service at %s: %w", c.serverAddr, err)
	}

	c.conn = conn
	c.client = searchv1.NewSearchServiceClient(conn)
	c.connected = true

	return nil
}

// buildDialOptions constructs the gRPC dial options from client configuration.
func (c *SearchClient) buildDialOptions() []grpc.DialOption {
	opts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                c.options.KeepaliveTime,
			Timeout:             c.options.KeepaliveTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.WaitForReady(true),
		),
		// Block on dial to detect connection failures early.
		grpc.WithBlock(),
	}

	// Configure credentials.
	if c.options.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if c.options.TLSConfig != nil {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(c.options.TLSConfig)))
	} else {
		// Default to insecure if no TLS config provided
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return opts
}

// Close closes the connection to the Search service.
func (c *SearchClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return nil
	}

	err := c.conn.Close()
	c.conn = nil
	c.client = nil
	c.connected = false

	if err != nil {
		return fmt.Errorf("closing search connection: %w", err)
	}

	return nil
}

// IsConnected returns true if the client has an active connection.
func (c *SearchClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.conn != nil
}

// contextWithTenant returns a context with the tenant ID in metadata.
func (c *SearchClient) contextWithTenant(ctx context.Context, tenantID string) context.Context {
	if tenantID == "" {
		c.mu.RLock()
		tenantID = c.options.TenantID
		c.mu.RUnlock()
	}
	if tenantID == "" {
		return ctx
	}
	md := metadata.Pairs("x-tenant-id", tenantID)
	return metadata.NewOutgoingContext(ctx, md)
}

// SearchRequest represents a search request with all options.
type SearchRequest struct {
	Query        string
	TenantID     string
	ContentTypes []string
	DateFrom     *time.Time
	DateTo       *time.Time
	Limit        int32
	Offset       int32
	TextWeight   *float32
	VectorWeight *float32
	MinScore     *float32
	SortOrder    searchv1.SortOrder
}

// SearchResult represents a single search result from the service.
type SearchResult struct {
	DocumentID  string
	ContentType string
	SourceID    string
	Title       *string
	Snippet     string
	Highlights  []string
	Score       float32
	TextScore   *float32
	VectorScore *float32
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Metadata    map[string]string
}

// SearchResponse represents the response from a search operation.
type SearchResponse struct {
	Results       []SearchResult
	TotalCount    int64
	QueryTimeMs   float64
	ExpansionInfo string
}

// Search performs a hybrid search combining full-text and vector similarity.
func (c *SearchClient) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("search client not connected")
	}

	ctx = c.contextWithTenant(ctx, req.TenantID)

	// Build the proto request.
	protoReq := &searchv1.SearchRequest{
		Query:     req.Query,
		TenantId:  req.TenantID,
		Limit:     req.Limit,
		Offset:    req.Offset,
		SortOrder: req.SortOrder,
	}

	// Add optional weights.
	if req.TextWeight != nil {
		protoReq.TextWeight = req.TextWeight
	}
	if req.VectorWeight != nil {
		protoReq.VectorWeight = req.VectorWeight
	}
	if req.MinScore != nil {
		protoReq.MinScore = req.MinScore
	}

	// Build filters.
	if len(req.ContentTypes) > 0 || req.DateFrom != nil || req.DateTo != nil {
		protoReq.Filters = &searchv1.FilterOptions{
			ContentTypes: req.ContentTypes,
		}
		if req.DateFrom != nil {
			protoReq.Filters.DateFrom = timestamppb.New(*req.DateFrom)
		}
		if req.DateTo != nil {
			protoReq.Filters.DateTo = timestamppb.New(*req.DateTo)
		}
	}

	// Execute the search.
	resp, err := client.Search(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}

	return c.convertSearchResponse(resp), nil
}

// SemanticSearch performs vector-only similarity search.
func (c *SearchClient) SemanticSearch(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("search client not connected")
	}

	ctx = c.contextWithTenant(ctx, req.TenantID)

	// Build the proto request.
	protoReq := &searchv1.SemanticSearchRequest{
		Query:    req.Query,
		TenantId: req.TenantID,
		Limit:    req.Limit,
		Offset:   req.Offset,
	}

	// Add min similarity if specified via MinScore.
	if req.MinScore != nil {
		protoReq.MinSimilarity = req.MinScore
	}

	// Build filters.
	if len(req.ContentTypes) > 0 || req.DateFrom != nil || req.DateTo != nil {
		protoReq.Filters = &searchv1.FilterOptions{
			ContentTypes: req.ContentTypes,
		}
		if req.DateFrom != nil {
			protoReq.Filters.DateFrom = timestamppb.New(*req.DateFrom)
		}
		if req.DateTo != nil {
			protoReq.Filters.DateTo = timestamppb.New(*req.DateTo)
		}
	}

	// Execute the search.
	resp, err := client.SemanticSearch(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("semantic search request failed: %w", err)
	}

	return c.convertSearchResponse(resp), nil
}

// KeywordSearch performs full-text only search.
func (c *SearchClient) KeywordSearch(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("search client not connected")
	}

	ctx = c.contextWithTenant(ctx, req.TenantID)

	// Build the proto request.
	protoReq := &searchv1.KeywordSearchRequest{
		Query:    req.Query,
		TenantId: req.TenantID,
		Limit:    req.Limit,
		Offset:   req.Offset,
	}

	// Build filters.
	if len(req.ContentTypes) > 0 || req.DateFrom != nil || req.DateTo != nil {
		protoReq.Filters = &searchv1.FilterOptions{
			ContentTypes: req.ContentTypes,
		}
		if req.DateFrom != nil {
			protoReq.Filters.DateFrom = timestamppb.New(*req.DateFrom)
		}
		if req.DateTo != nil {
			protoReq.Filters.DateTo = timestamppb.New(*req.DateTo)
		}
	}

	// Execute the search.
	resp, err := client.KeywordSearch(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("keyword search request failed: %w", err)
	}

	return c.convertSearchResponse(resp), nil
}

// GetSearchStats retrieves search index statistics.
func (c *SearchClient) GetSearchStats(ctx context.Context, tenantID string) (*searchv1.GetSearchStatsResponse, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("search client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	req := &searchv1.GetSearchStatsRequest{}
	if tenantID != "" {
		req.TenantId = &tenantID
	}

	resp, err := client.GetSearchStats(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get search stats request failed: %w", err)
	}

	return resp, nil
}

// convertSearchResponse converts the proto response to our SearchResponse type.
func (c *SearchClient) convertSearchResponse(resp *searchv1.SearchResponse) *SearchResponse {
	if resp == nil {
		return &SearchResponse{}
	}

	results := make([]SearchResult, len(resp.Results))
	for i, r := range resp.Results {
		result := SearchResult{
			DocumentID:  r.DocumentId,
			ContentType: r.ContentType,
			SourceID:    r.SourceId,
			Title:       r.Title,
			Snippet:     r.Snippet,
			Highlights:  r.Highlights,
			Score:       r.Score,
			TextScore:   r.TextScore,
			VectorScore: r.VectorScore,
			Metadata:    r.Metadata,
		}
		if r.CreatedAt != nil {
			result.CreatedAt = r.CreatedAt.AsTime()
		}
		if r.UpdatedAt != nil {
			result.UpdatedAt = r.UpdatedAt.AsTime()
		}
		results[i] = result
	}

	return &SearchResponse{
		Results:       results,
		TotalCount:    resp.TotalCount,
		QueryTimeMs:   resp.QueryTimeMs,
		ExpansionInfo: resp.ExpansionInfo,
	}
}
