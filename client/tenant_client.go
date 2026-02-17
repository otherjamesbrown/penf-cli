// Package client provides gRPC clients for Penfold services.
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	tenantv1 "github.com/otherjamesbrown/penf-cli/api/proto/tenant/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// TenantClient manages the connection to the Penfold Tenant service.
type TenantClient struct {
	conn       *grpc.ClientConn
	client     tenantv1.TenantServiceClient
	serverAddr string
	options    *ClientOptions
	mu         sync.RWMutex
	connected  bool
}

// NewTenantClient creates a new TenantClient with the given options.
// Call Connect() to establish the connection.
func NewTenantClient(serverAddr string, opts *ClientOptions) *TenantClient {
	if opts == nil {
		opts = DefaultOptions()
	}

	return &TenantClient{
		serverAddr: serverAddr,
		options:    opts,
	}
}

// Connect establishes a connection to the Tenant service.
func (c *TenantClient) Connect(ctx context.Context) error {
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
		return fmt.Errorf("connecting to tenant service at %s: %w", c.serverAddr, err)
	}

	c.conn = conn
	c.client = tenantv1.NewTenantServiceClient(conn)
	c.connected = true

	return nil
}

// buildDialOptions constructs the gRPC dial options from client configuration.
func (c *TenantClient) buildDialOptions() []grpc.DialOption {
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
		creds := credentials.NewTLS(c.options.TLSConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		// Fallback to insecure if no TLS config provided.
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return opts
}

// Close closes the connection to the Tenant service.
func (c *TenantClient) Close() error {
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
		return fmt.Errorf("closing tenant connection: %w", err)
	}

	return nil
}

// IsConnected returns true if the client has an active connection.
func (c *TenantClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.conn != nil
}

// Domain types for the client

// Tenant represents a tenant organization.
type Tenant struct {
	ID               string
	Name             string
	Slug             string
	Description      string
	IsActive         bool
	Settings         string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	IntegrationCount int32
	RuleCount        int32
}

// ListTenantsRequest represents a request to list tenants.
type ListTenantsRequest struct {
	IsActive *bool
	Search   string
	Limit    int32
	Offset   int32
}

// ListTenants returns a list of tenants matching the filter.
func (c *TenantClient) ListTenants(ctx context.Context, req *ListTenantsRequest) ([]*Tenant, int64, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, 0, fmt.Errorf("tenant client not connected")
	}

	protoReq := &tenantv1.ListTenantsRequest{
		Search: req.Search,
		Limit:  req.Limit,
		Offset: req.Offset,
	}

	if req.IsActive != nil {
		protoReq.IsActive = req.IsActive
	}

	resp, err := client.ListTenants(ctx, protoReq)
	if err != nil {
		return nil, 0, fmt.Errorf("list tenants request failed: %w", err)
	}

	tenants := make([]*Tenant, len(resp.Tenants))
	for i, t := range resp.Tenants {
		tenants[i] = protoToTenant(t)
	}

	return tenants, resp.TotalCount, nil
}

// GetTenant retrieves a single tenant by ID or slug.
func (c *TenantClient) GetTenant(ctx context.Context, id string, slug string) (*Tenant, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("tenant client not connected")
	}

	protoReq := &tenantv1.GetTenantRequest{}
	if id != "" {
		protoReq.Id = id
	}
	if slug != "" {
		protoReq.Slug = slug
	}

	resp, err := client.GetTenant(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("get tenant request failed: %w", err)
	}

	return protoToTenant(resp.Tenant), nil
}

// CreateTenantRequest represents a request to create a tenant.
type CreateTenantRequest struct {
	Name        string
	Slug        string
	Description string
	Settings    string
}

// CreateTenant creates a new tenant.
func (c *TenantClient) CreateTenant(ctx context.Context, req *CreateTenantRequest) (*Tenant, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("tenant client not connected")
	}

	protoReq := &tenantv1.CreateTenantRequest{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		Settings:    req.Settings,
	}

	resp, err := client.CreateTenant(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("create tenant request failed: %w", err)
	}

	return protoToTenant(resp.Tenant), nil
}

// UpdateTenantRequest represents a request to update a tenant.
type UpdateTenantRequest struct {
	ID          string
	Name        *string
	Description *string
	IsActive    *bool
	Settings    *string
}

// UpdateTenant updates an existing tenant.
func (c *TenantClient) UpdateTenant(ctx context.Context, req *UpdateTenantRequest) (*Tenant, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("tenant client not connected")
	}

	protoReq := &tenantv1.UpdateTenantRequest{
		Id:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
		Settings:    req.Settings,
	}

	resp, err := client.UpdateTenant(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("update tenant request failed: %w", err)
	}

	return protoToTenant(resp.Tenant), nil
}

// DeleteTenant soft-deletes a tenant.
func (c *TenantClient) DeleteTenant(ctx context.Context, id string, reason string) (bool, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return false, fmt.Errorf("tenant client not connected")
	}

	resp, err := client.DeleteTenant(ctx, &tenantv1.DeleteTenantRequest{
		Id:     id,
		Reason: reason,
	})
	if err != nil {
		return false, fmt.Errorf("delete tenant request failed: %w", err)
	}

	return resp.Deleted, nil
}

// SetCurrentTenant validates and returns tenant info for switching.
func (c *TenantClient) SetCurrentTenant(ctx context.Context, tenantRef string) (*Tenant, bool, string, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, false, "", fmt.Errorf("tenant client not connected")
	}

	resp, err := client.SetCurrentTenant(ctx, &tenantv1.SetCurrentTenantRequest{
		TenantRef: tenantRef,
	})
	if err != nil {
		return nil, false, "", fmt.Errorf("set current tenant request failed: %w", err)
	}

	return protoToTenant(resp.Tenant), resp.Valid, resp.Error, nil
}

// Conversion helpers

func protoToTenant(t *tenantv1.Tenant) *Tenant {
	if t == nil {
		return nil
	}

	tenant := &Tenant{
		ID:               t.Id,
		Name:             t.Name,
		Slug:             t.Slug,
		Description:      t.Description,
		IsActive:         t.IsActive,
		Settings:         t.Settings,
		IntegrationCount: t.IntegrationCount,
		RuleCount:        t.RuleCount,
	}

	if t.CreatedAt != nil {
		tenant.CreatedAt = t.CreatedAt.AsTime()
	}

	if t.UpdatedAt != nil {
		tenant.UpdatedAt = t.UpdatedAt.AsTime()
	}

	return tenant
}
