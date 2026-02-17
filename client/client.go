// Package client provides the gRPC client for connecting to the Penfold API Gateway.
// It handles connection management, retry logic, and health checking.
package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	gatewaypb "github.com/otherjamesbrown/penf-cli/api/proto/core/v1/gatewaypb"
	"github.com/otherjamesbrown/penf-cli/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// SystemStatus represents the overall system health and status.
type SystemStatus struct {
	Healthy   bool            `json:"healthy"`
	Message   string          `json:"message"`
	Services  []ServiceHealth `json:"services"`
	Database  *DatabaseStatus `json:"database,omitempty"`
	Queues    *QueueStatus    `json:"queues,omitempty"`
	Version   *VersionInfo    `json:"version,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// ServiceHealth represents the health status of a single service.
type ServiceHealth struct {
	Name          string  `json:"name"`
	Healthy       bool    `json:"healthy"`
	Status        string  `json:"status"`
	Message       string  `json:"message,omitempty"`
	LatencyMs     float64 `json:"latency_ms,omitempty"`
	Version       string  `json:"version,omitempty"`
	UptimeSeconds int64   `json:"uptime_seconds,omitempty"`
}

// DatabaseStatus represents database health and statistics.
type DatabaseStatus struct {
	Healthy                bool    `json:"healthy"`
	Type                   string  `json:"type"`
	ConnectionStatus       string  `json:"connection_status"`
	ActiveConnections      int32   `json:"active_connections"`
	MaxConnections         int32   `json:"max_connections"`
	VectorExtensionEnabled bool    `json:"vector_extension_enabled"`
	ContentCount           int64   `json:"content_count"`
	EntityCount            int64   `json:"entity_count"`
	LatencyMs              float64 `json:"latency_ms"`
}

// QueueStatus represents message queue health and depths.
type QueueStatus struct {
	Healthy         bool             `json:"healthy"`
	Type            string           `json:"type"`
	QueueDepths     map[string]int64 `json:"queue_depths,omitempty"`
	TotalPending    int64            `json:"total_pending"`
	ProcessingRate  float64          `json:"processing_rate"`
	DeadLetterCount int64            `json:"dead_letter_count"`
}

// VersionInfo contains system version information.
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
}

// Default connection settings.
const (
	DefaultConnectTimeout    = 10 * time.Second
	DefaultKeepaliveTime     = 5 * time.Minute // Must be >= gRPC server's MinTime (default 5 min)
	DefaultKeepaliveTimeout  = 20 * time.Second
	DefaultMaxRetries        = 3
	DefaultInitialBackoff    = 100 * time.Millisecond
	DefaultMaxBackoff        = 5 * time.Second
	DefaultBackoffMultiplier = 2.0
)

// GRPCClient manages the connection to the Penfold API Gateway.
type GRPCClient struct {
	// conn is the underlying gRPC connection.
	conn *grpc.ClientConn

	// serverAddr is the address of the API Gateway.
	serverAddr string

	// options holds the client configuration.
	options *ClientOptions

	// mu protects concurrent access to connection state.
	mu sync.RWMutex

	// connected indicates if the client is currently connected.
	connected bool
}

// ClientOptions configures the GRPCClient behavior.
type ClientOptions struct {
	// ConnectTimeout is the maximum time to wait for connection.
	ConnectTimeout time.Duration

	// KeepaliveTime is the interval for keepalive pings.
	KeepaliveTime time.Duration

	// KeepaliveTimeout is the timeout for keepalive ping response.
	KeepaliveTimeout time.Duration

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// InitialBackoff is the initial backoff duration for retries.
	InitialBackoff time.Duration

	// MaxBackoff is the maximum backoff duration for retries.
	MaxBackoff time.Duration

	// BackoffMultiplier is the multiplier for exponential backoff.
	BackoffMultiplier float64

	// Insecure disables TLS (for development only).
	Insecure bool

	// Debug enables verbose logging.
	Debug bool

	// TenantID is the default tenant ID to include in all requests.
	TenantID string

	// TLSConfig is the TLS configuration for secure connections.
	// If nil and Insecure is false, connection may fail.
	TLSConfig *tls.Config
}

// DefaultOptions returns ClientOptions with default values.
func DefaultOptions() *ClientOptions {
	return &ClientOptions{
		ConnectTimeout:    DefaultConnectTimeout,
		KeepaliveTime:     DefaultKeepaliveTime,
		KeepaliveTimeout:  DefaultKeepaliveTimeout,
		MaxRetries:        DefaultMaxRetries,
		InitialBackoff:    DefaultInitialBackoff,
		MaxBackoff:        DefaultMaxBackoff,
		BackoffMultiplier: DefaultBackoffMultiplier,
		Insecure:          true, // Default to insecure for local development.
	}
}

// NewGRPCClient creates a new GRPCClient with the given options.
// Call Connect() to establish the connection.
func NewGRPCClient(serverAddr string, opts *ClientOptions) *GRPCClient {
	if opts == nil {
		opts = DefaultOptions()
	}

	return &GRPCClient{
		serverAddr: serverAddr,
		options:    opts,
	}
}

// Connect establishes a connection to the API Gateway.
// It uses the configured timeout and returns an error if connection fails.
func (c *GRPCClient) Connect(ctx context.Context) error {
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
		return fmt.Errorf("connecting to %s: %w", c.serverAddr, err)
	}

	c.conn = conn
	c.connected = true

	return nil
}

// buildDialOptions constructs the gRPC dial options from client configuration.
func (c *GRPCClient) buildDialOptions() []grpc.DialOption {
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
		// Without this, DialContext returns immediately and connection happens lazily,
		// which can cause confusing hangs when the server is unreachable.
		grpc.WithBlock(),
	}

	// Configure transport credentials.
	if c.options.Insecure {
		// Insecure mode - no TLS (for local development only).
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if c.options.TLSConfig != nil {
		// Use provided TLS configuration (mTLS).
		creds := credentials.NewTLS(c.options.TLSConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		// Fallback to insecure if no TLS config provided.
		// This maintains backward compatibility but should log a warning.
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return opts
}

// Close closes the connection to the API Gateway.
// It's safe to call Close multiple times.
func (c *GRPCClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return nil
	}

	err := c.conn.Close()
	c.conn = nil
	c.connected = false

	if err != nil {
		return fmt.Errorf("closing connection: %w", err)
	}

	return nil
}

// IsConnected returns true if the client has an active connection.
func (c *GRPCClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.connected && c.conn != nil
}

// GetConnection returns the underlying gRPC connection.
// Returns nil if not connected.
func (c *GRPCClient) GetConnection() *grpc.ClientConn {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.conn
}

// HealthCheck performs a connection health check.
// Returns nil if the connection is healthy, an error otherwise.
func (c *GRPCClient) HealthCheck(ctx context.Context) error {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return fmt.Errorf("not connected")
	}

	// Check connection state.
	state := conn.GetState()
	switch state {
	case connectivity.Ready:
		return nil
	case connectivity.Connecting:
		// Wait for connection to be ready.
		if !conn.WaitForStateChange(ctx, connectivity.Connecting) {
			return fmt.Errorf("connection timeout while connecting")
		}
		newState := conn.GetState()
		if newState != connectivity.Ready {
			return fmt.Errorf("connection failed: state is %v", newState)
		}
		return nil
	case connectivity.Idle:
		// Trigger connection attempt.
		conn.Connect()
		if !conn.WaitForStateChange(ctx, connectivity.Idle) {
			return fmt.Errorf("connection timeout from idle state")
		}
		newState := conn.GetState()
		if newState != connectivity.Ready && newState != connectivity.Connecting {
			return fmt.Errorf("connection failed: state is %v", newState)
		}
		return nil
	case connectivity.TransientFailure:
		return fmt.Errorf("connection in transient failure state")
	case connectivity.Shutdown:
		return fmt.Errorf("connection has been shut down")
	default:
		return fmt.Errorf("unknown connection state: %v", state)
	}
}

// Reconnect closes the existing connection and establishes a new one.
// Uses exponential backoff for retry attempts.
func (c *GRPCClient) Reconnect(ctx context.Context) error {
	// Close existing connection.
	if err := c.Close(); err != nil {
		// Log but don't fail - we want to try reconnecting.
		_ = err
	}

	// Attempt reconnection with exponential backoff.
	backoff := c.options.InitialBackoff
	var lastErr error

	for attempt := 0; attempt < c.options.MaxRetries; attempt++ {
		if err := c.Connect(ctx); err != nil {
			lastErr = err

			// Check if context is cancelled.
			select {
			case <-ctx.Done():
				return fmt.Errorf("reconnection cancelled: %w", ctx.Err())
			default:
			}

			// Wait with exponential backoff.
			select {
			case <-ctx.Done():
				return fmt.Errorf("reconnection cancelled during backoff: %w", ctx.Err())
			case <-time.After(backoff):
			}

			// Increase backoff.
			backoff = time.Duration(float64(backoff) * c.options.BackoffMultiplier)
			if backoff > c.options.MaxBackoff {
				backoff = c.options.MaxBackoff
			}

			continue
		}

		return nil // Successfully reconnected.
	}

	return fmt.Errorf("reconnection failed after %d attempts: %w", c.options.MaxRetries, lastErr)
}

// WithRetry executes the given function with automatic retry on failure.
// Uses exponential backoff between retry attempts.
func (c *GRPCClient) WithRetry(ctx context.Context, fn func() error) error {
	backoff := c.options.InitialBackoff
	var lastErr error

	for attempt := 0; attempt <= c.options.MaxRetries; attempt++ {
		if err := fn(); err != nil {
			lastErr = err

			if attempt == c.options.MaxRetries {
				break
			}

			// Check if context is cancelled.
			select {
			case <-ctx.Done():
				return fmt.Errorf("operation cancelled: %w", ctx.Err())
			default:
			}

			// Wait with exponential backoff.
			select {
			case <-ctx.Done():
				return fmt.Errorf("operation cancelled during backoff: %w", ctx.Err())
			case <-time.After(backoff):
			}

			// Increase backoff.
			backoff = time.Duration(float64(backoff) * c.options.BackoffMultiplier)
			if backoff > c.options.MaxBackoff {
				backoff = c.options.MaxBackoff
			}

			continue
		}

		return nil // Success.
	}

	return fmt.Errorf("operation failed after %d attempts: %w", c.options.MaxRetries+1, lastErr)
}

// ServerAddress returns the configured server address.
func (c *GRPCClient) ServerAddress() string {
	return c.serverAddr
}

// TenantID returns the configured tenant ID.
func (c *GRPCClient) TenantID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.options.TenantID
}

// SetTenantID updates the default tenant ID for requests.
func (c *GRPCClient) SetTenantID(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.options.TenantID = tenantID
}

// ContextWithTenant returns a context with the tenant ID in metadata.
// If tenantID is empty, uses the default tenant ID from options.
func (c *GRPCClient) ContextWithTenant(ctx context.Context, tenantID string) context.Context {
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

// ConnectionState returns a human-readable connection state string.
func (c *GRPCClient) ConnectionState() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.conn == nil {
		return "disconnected"
	}

	state := c.conn.GetState()
	switch state {
	case connectivity.Idle:
		return "idle"
	case connectivity.Connecting:
		return "connecting"
	case connectivity.Ready:
		return "ready"
	case connectivity.TransientFailure:
		return "transient_failure"
	case connectivity.Shutdown:
		return "shutdown"
	default:
		return "unknown"
	}
}

// GetStatus retrieves the system status from the gateway.
// If verbose is true, additional details (dependencies) are included.
func (c *GRPCClient) GetStatus(ctx context.Context, verbose bool) (*SystemStatus, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return nil, fmt.Errorf("not connected to gateway")
	}

	// Create gRPC client for the gateway service.
	client := gatewaypb.NewGatewayServiceClient(conn)

	// Call the HealthCheck RPC.
	resp, err := client.HealthCheck(ctx, &gatewaypb.HealthCheckRequest{
		IncludeDependencies: verbose,
	})
	if err != nil {
		return nil, fmt.Errorf("HealthCheck RPC failed: %w", err)
	}

	// Convert the gRPC response to SystemStatus.
	return convertHealthCheckResponse(resp), nil
}

// convertHealthCheckResponse maps the gRPC HealthCheckResponse to the client SystemStatus struct.
func convertHealthCheckResponse(resp *gatewaypb.HealthCheckResponse) *SystemStatus {
	status := &SystemStatus{
		Healthy:   resp.GetHealthy(),
		Message:   resp.GetMessage(),
		Timestamp: time.Now(),
		Services:  make([]ServiceHealth, 0, len(resp.GetDependencies())),
	}

	// Convert timestamp if present.
	if resp.GetTimestamp() != nil {
		status.Timestamp = resp.GetTimestamp().AsTime()
	}

	// Convert version info.
	if resp.GetVersion() != "" {
		status.Version = &VersionInfo{
			Version: resp.GetVersion(),
		}
	}

	// Convert dependencies to service health entries.
	for _, dep := range resp.GetDependencies() {
		svcHealth := ServiceHealth{
			Name:    dep.GetName(),
			Healthy: dep.GetHealthy(),
			Message: dep.GetMessage(),
		}

		// Set status string based on health.
		if dep.GetHealthy() {
			svcHealth.Status = "running"
		} else {
			svcHealth.Status = "down"
		}

		// Set latency if available.
		if dep.LatencyMs != nil {
			svcHealth.LatencyMs = *dep.LatencyMs
		}

		status.Services = append(status.Services, svcHealth)
	}

	// Add gateway service itself with uptime.
	gatewayService := ServiceHealth{
		Name:          "gateway",
		Healthy:       resp.GetHealthy(),
		Status:        "running",
		Message:       resp.GetMessage(),
		UptimeSeconds: resp.GetUptimeSeconds(),
	}
	if resp.GetVersion() != "" {
		gatewayService.Version = resp.GetVersion()
	}
	// Prepend gateway service to the list.
	status.Services = append([]ServiceHealth{gatewayService}, status.Services...)

	return status
}

// ConnectFromConfig creates and connects a GRPCClient using CLIConfig.
// This is the canonical way to create a connected client from CLI commands.
func ConnectFromConfig(cfg *config.CLIConfig) (*GRPCClient, error) {
	opts := DefaultOptions()
	opts.Insecure = cfg.Insecure
	opts.Debug = cfg.Debug
	opts.TenantID = cfg.TenantID

	if !cfg.Insecure && cfg.TLS.Enabled {
		tlsConfig, err := LoadClientTLSConfig(&cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("loading TLS config: %w", err)
		}
		opts.TLSConfig = tlsConfig
	}

	c := NewGRPCClient(cfg.ServerAddress, opts)
	ctx, cancel := context.WithTimeout(context.Background(), opts.ConnectTimeout)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connecting to server: %w", err)
	}
	return c, nil
}
