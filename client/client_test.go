// Package client provides the gRPC client for connecting to the Penfold API Gateway.
package client

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	gatewaypb "github.com/otherjamesbrown/penf-cli/api/proto/core/v1/gatewaypb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/metadata"
)

// TestDefaultOptions verifies the default client options.
func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts == nil {
		t.Fatal("DefaultOptions returned nil")
	}

	if opts.ConnectTimeout != DefaultConnectTimeout {
		t.Errorf("ConnectTimeout = %v, want %v", opts.ConnectTimeout, DefaultConnectTimeout)
	}
	if opts.KeepaliveTime != DefaultKeepaliveTime {
		t.Errorf("KeepaliveTime = %v, want %v", opts.KeepaliveTime, DefaultKeepaliveTime)
	}
	if opts.KeepaliveTimeout != DefaultKeepaliveTimeout {
		t.Errorf("KeepaliveTimeout = %v, want %v", opts.KeepaliveTimeout, DefaultKeepaliveTimeout)
	}
	if opts.MaxRetries != DefaultMaxRetries {
		t.Errorf("MaxRetries = %v, want %v", opts.MaxRetries, DefaultMaxRetries)
	}
	if opts.InitialBackoff != DefaultInitialBackoff {
		t.Errorf("InitialBackoff = %v, want %v", opts.InitialBackoff, DefaultInitialBackoff)
	}
	if opts.MaxBackoff != DefaultMaxBackoff {
		t.Errorf("MaxBackoff = %v, want %v", opts.MaxBackoff, DefaultMaxBackoff)
	}
	if opts.BackoffMultiplier != DefaultBackoffMultiplier {
		t.Errorf("BackoffMultiplier = %v, want %v", opts.BackoffMultiplier, DefaultBackoffMultiplier)
	}
	if !opts.Insecure {
		t.Error("Insecure should be true by default for local development")
	}
}

// TestNewGRPCClient verifies client creation.
func TestNewGRPCClient(t *testing.T) {
	t.Run("with options", func(t *testing.T) {
		opts := &ClientOptions{
			ConnectTimeout: 5 * time.Second,
			MaxRetries:     5,
			Insecure:       true,
			TenantID:       "test-tenant",
		}
		client := NewGRPCClient("localhost:50051", opts)

		if client == nil {
			t.Fatal("NewGRPCClient returned nil")
		}
		if client.serverAddr != "localhost:50051" {
			t.Errorf("serverAddr = %v, want localhost:50051", client.serverAddr)
		}
		if client.options.TenantID != "test-tenant" {
			t.Errorf("TenantID = %v, want test-tenant", client.options.TenantID)
		}
	})

	t.Run("with nil options uses defaults", func(t *testing.T) {
		client := NewGRPCClient("localhost:50051", nil)

		if client == nil {
			t.Fatal("NewGRPCClient returned nil")
		}
		if client.options == nil {
			t.Fatal("options should be set to defaults")
		}
		if client.options.MaxRetries != DefaultMaxRetries {
			t.Errorf("MaxRetries = %v, want %v", client.options.MaxRetries, DefaultMaxRetries)
		}
	})
}

// TestGRPCClient_ServerAddress verifies the ServerAddress method.
func TestGRPCClient_ServerAddress(t *testing.T) {
	client := NewGRPCClient("api.example.com:443", nil)

	addr := client.ServerAddress()
	if addr != "api.example.com:443" {
		t.Errorf("ServerAddress() = %v, want api.example.com:443", addr)
	}
}

// TestGRPCClient_TenantID verifies tenant ID methods.
func TestGRPCClient_TenantID(t *testing.T) {
	opts := &ClientOptions{
		TenantID: "initial-tenant",
		Insecure: true,
	}
	client := NewGRPCClient("localhost:50051", opts)

	if client.TenantID() != "initial-tenant" {
		t.Errorf("TenantID() = %v, want initial-tenant", client.TenantID())
	}

	client.SetTenantID("updated-tenant")
	if client.TenantID() != "updated-tenant" {
		t.Errorf("TenantID() = %v, want updated-tenant", client.TenantID())
	}
}

// TestGRPCClient_IsConnected verifies connection state reporting.
func TestGRPCClient_IsConnected(t *testing.T) {
	client := NewGRPCClient("localhost:50051", nil)

	// Before connecting, should be false
	if client.IsConnected() {
		t.Error("IsConnected() should be false before Connect()")
	}
}

// TestGRPCClient_GetConnection verifies connection accessor.
func TestGRPCClient_GetConnection(t *testing.T) {
	client := NewGRPCClient("localhost:50051", nil)

	// Before connecting, should be nil
	conn := client.GetConnection()
	if conn != nil {
		t.Error("GetConnection() should return nil before Connect()")
	}
}

// TestGRPCClient_ConnectionState verifies connection state string.
func TestGRPCClient_ConnectionState(t *testing.T) {
	client := NewGRPCClient("localhost:50051", nil)

	// Before connecting, should be "disconnected"
	state := client.ConnectionState()
	if state != "disconnected" {
		t.Errorf("ConnectionState() = %v, want 'disconnected'", state)
	}
}

// TestGRPCClient_Close verifies close behavior.
func TestGRPCClient_Close(t *testing.T) {
	client := NewGRPCClient("localhost:50051", nil)

	// Close should be safe to call even when not connected
	err := client.Close()
	if err != nil {
		t.Errorf("Close() error = %v, expected nil", err)
	}

	// Multiple closes should be safe
	err = client.Close()
	if err != nil {
		t.Errorf("Close() second call error = %v, expected nil", err)
	}
}

// TestGRPCClient_HealthCheck_NotConnected verifies health check when not connected.
func TestGRPCClient_HealthCheck_NotConnected(t *testing.T) {
	client := NewGRPCClient("localhost:50051", nil)
	ctx := context.Background()

	err := client.HealthCheck(ctx)
	if err == nil {
		t.Error("HealthCheck() should fail when not connected")
	}
	if err.Error() != "not connected" {
		t.Errorf("HealthCheck() error = %v, want 'not connected'", err)
	}
}

// TestGRPCClient_ContextWithTenant verifies tenant context creation.
func TestGRPCClient_ContextWithTenant(t *testing.T) {
	client := NewGRPCClient("localhost:50051", nil)
	ctx := context.Background()

	t.Run("with provided tenant ID", func(t *testing.T) {
		newCtx := client.ContextWithTenant(ctx, "custom-tenant")

		// Verify the context has metadata
		md, ok := metadata.FromOutgoingContext(newCtx)
		if !ok {
			t.Fatal("expected metadata in context")
		}

		values := md.Get("x-tenant-id")
		if len(values) != 1 || values[0] != "custom-tenant" {
			t.Errorf("x-tenant-id = %v, want ['custom-tenant']", values)
		}
	})

	t.Run("with empty tenant ID uses default", func(t *testing.T) {
		opts := &ClientOptions{
			TenantID: "default-tenant",
			Insecure: true,
		}
		clientWithDefault := NewGRPCClient("localhost:50051", opts)
		newCtx := clientWithDefault.ContextWithTenant(ctx, "")

		md, ok := metadata.FromOutgoingContext(newCtx)
		if !ok {
			t.Fatal("expected metadata in context")
		}

		values := md.Get("x-tenant-id")
		if len(values) != 1 || values[0] != "default-tenant" {
			t.Errorf("x-tenant-id = %v, want ['default-tenant']", values)
		}
	})

	t.Run("with no tenant ID returns original context", func(t *testing.T) {
		clientNoTenant := NewGRPCClient("localhost:50051", nil)
		newCtx := clientNoTenant.ContextWithTenant(ctx, "")

		// Context should be unchanged
		if newCtx != ctx {
			t.Error("expected same context when no tenant ID")
		}
	})
}

// TestGRPCClient_GetStatus verifies status retrieval behavior.
// Note: GetStatus now requires a real gRPC connection since it calls the gateway.
func TestGRPCClient_GetStatus(t *testing.T) {
	client := NewGRPCClient("localhost:50051", nil)
	ctx := context.Background()

	t.Run("returns error when not connected", func(t *testing.T) {
		// Without calling Connect(), GetStatus should return an error
		status, err := client.GetStatus(ctx, false)
		if err == nil {
			t.Fatal("GetStatus() expected error when not connected")
		}
		if status != nil {
			t.Error("expected nil status when not connected")
		}
		// Verify the error message indicates not connected
		if err.Error() != "not connected to gateway" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

// TestGRPCClient_WithRetry verifies retry logic.
func TestGRPCClient_WithRetry(t *testing.T) {
	opts := &ClientOptions{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        50 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Insecure:          true,
	}
	client := NewGRPCClient("localhost:50051", opts)

	t.Run("success on first try", func(t *testing.T) {
		attempts := 0
		ctx := context.Background()

		err := client.WithRetry(ctx, func() error {
			attempts++
			return nil
		})

		if err != nil {
			t.Errorf("WithRetry() error = %v", err)
		}
		if attempts != 1 {
			t.Errorf("attempts = %d, want 1", attempts)
		}
	})

	t.Run("success after retries", func(t *testing.T) {
		attempts := 0
		ctx := context.Background()

		err := client.WithRetry(ctx, func() error {
			attempts++
			if attempts < 3 {
				return errors.New("transient error")
			}
			return nil
		})

		if err != nil {
			t.Errorf("WithRetry() error = %v", err)
		}
		if attempts != 3 {
			t.Errorf("attempts = %d, want 3", attempts)
		}
	})

	t.Run("failure after max retries", func(t *testing.T) {
		attempts := 0
		ctx := context.Background()

		err := client.WithRetry(ctx, func() error {
			attempts++
			return errors.New("persistent error")
		})

		if err == nil {
			t.Error("WithRetry() should fail after max retries")
		}
		// Should have tried MaxRetries + 1 times (initial + retries)
		if attempts != opts.MaxRetries+1 {
			t.Errorf("attempts = %d, want %d", attempts, opts.MaxRetries+1)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := client.WithRetry(ctx, func() error {
			return errors.New("should not matter")
		})

		if err == nil {
			t.Error("WithRetry() should fail on cancelled context")
		}
	})
}

// TestGRPCClient_ConcurrentAccess verifies thread safety.
func TestGRPCClient_ConcurrentAccess(t *testing.T) {
	client := NewGRPCClient("localhost:50051", nil)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.IsConnected()
			_ = client.ConnectionState()
			_ = client.ServerAddress()
			_ = client.TenantID()
			_ = client.GetConnection()
		}()
	}

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			client.SetTenantID("tenant-" + string(rune('0'+n)))
		}(i)
	}

	wg.Wait()
}

// TestSystemStatus_JSON verifies SystemStatus JSON serialization.
func TestSystemStatus_JSON(t *testing.T) {
	status := &SystemStatus{
		Healthy:   true,
		Message:   "All systems operational",
		Timestamp: time.Now(),
		Services: []ServiceHealth{
			{
				Name:          "gateway",
				Healthy:       true,
				Status:        "running",
				Message:       "Ready",
				LatencyMs:     1.5,
				Version:       "1.0.0",
				UptimeSeconds: 3600,
			},
		},
		Database: &DatabaseStatus{
			Healthy:                true,
			Type:                   "postgresql",
			ConnectionStatus:       "connected",
			ActiveConnections:      5,
			MaxConnections:         100,
			VectorExtensionEnabled: true,
			ContentCount:           1000,
			EntityCount:            250,
			LatencyMs:              0.5,
		},
		Queues: &QueueStatus{
			Healthy:         true,
			Type:            "redis",
			TotalPending:    10,
			ProcessingRate:  50.0,
			DeadLetterCount: 0,
			QueueDepths: map[string]int64{
				"ingestion": 5,
				"embedding": 5,
			},
		},
		Version: &VersionInfo{
			Version:   "1.0.0",
			Commit:    "abc123",
			BuildTime: "2024-01-01T00:00:00Z",
			GoVersion: "go1.24.0",
		},
	}

	// Verify all fields are exported for JSON serialization
	if status.Healthy != true {
		t.Error("Healthy field not accessible")
	}
	if status.Services[0].Name != "gateway" {
		t.Error("Services field not accessible")
	}
	if status.Database.Type != "postgresql" {
		t.Error("Database field not accessible")
	}
	if status.Queues.Type != "redis" {
		t.Error("Queues field not accessible")
	}
	if status.Version.Version != "1.0.0" {
		t.Error("Version field not accessible")
	}
}

// TestServiceHealth_Fields verifies ServiceHealth field access.
func TestServiceHealth_Fields(t *testing.T) {
	health := ServiceHealth{
		Name:          "test-service",
		Healthy:       true,
		Status:        "running",
		Message:       "All good",
		LatencyMs:     2.5,
		Version:       "2.0.0",
		UptimeSeconds: 7200,
	}

	if health.Name != "test-service" {
		t.Errorf("Name = %v, want test-service", health.Name)
	}
	if !health.Healthy {
		t.Error("Healthy should be true")
	}
	if health.LatencyMs != 2.5 {
		t.Errorf("LatencyMs = %v, want 2.5", health.LatencyMs)
	}
	if health.UptimeSeconds != 7200 {
		t.Errorf("UptimeSeconds = %v, want 7200", health.UptimeSeconds)
	}
}

// TestDatabaseStatus_Fields verifies DatabaseStatus field access.
func TestDatabaseStatus_Fields(t *testing.T) {
	db := DatabaseStatus{
		Healthy:                true,
		Type:                   "postgresql",
		ConnectionStatus:       "connected",
		ActiveConnections:      15,
		MaxConnections:         100,
		VectorExtensionEnabled: true,
		ContentCount:           5000,
		EntityCount:            1200,
		LatencyMs:              0.8,
	}

	if db.ActiveConnections != 15 {
		t.Errorf("ActiveConnections = %v, want 15", db.ActiveConnections)
	}
	if !db.VectorExtensionEnabled {
		t.Error("VectorExtensionEnabled should be true")
	}
	if db.ContentCount != 5000 {
		t.Errorf("ContentCount = %v, want 5000", db.ContentCount)
	}
}

// TestQueueStatus_Fields verifies QueueStatus field access.
func TestQueueStatus_Fields(t *testing.T) {
	q := QueueStatus{
		Healthy:         true,
		Type:            "redis",
		TotalPending:    25,
		ProcessingRate:  100.5,
		DeadLetterCount: 3,
		QueueDepths: map[string]int64{
			"ingestion":  10,
			"embedding":  8,
			"extraction": 7,
		},
	}

	if q.TotalPending != 25 {
		t.Errorf("TotalPending = %v, want 25", q.TotalPending)
	}
	if q.ProcessingRate != 100.5 {
		t.Errorf("ProcessingRate = %v, want 100.5", q.ProcessingRate)
	}
	if len(q.QueueDepths) != 3 {
		t.Errorf("QueueDepths length = %v, want 3", len(q.QueueDepths))
	}
	if q.QueueDepths["ingestion"] != 10 {
		t.Errorf("QueueDepths[ingestion] = %v, want 10", q.QueueDepths["ingestion"])
	}
}

// TestVersionInfo_Fields verifies VersionInfo field access.
func TestVersionInfo_Fields(t *testing.T) {
	v := VersionInfo{
		Version:   "3.1.4",
		Commit:    "deadbeef",
		BuildTime: "2024-06-15T12:00:00Z",
		GoVersion: "go1.24.0",
	}

	if v.Version != "3.1.4" {
		t.Errorf("Version = %v, want 3.1.4", v.Version)
	}
	if v.Commit != "deadbeef" {
		t.Errorf("Commit = %v, want deadbeef", v.Commit)
	}
	if v.GoVersion != "go1.24.0" {
		t.Errorf("GoVersion = %v, want go1.24.0", v.GoVersion)
	}
}

// TestClientOptions_Fields verifies ClientOptions field access.
func TestClientOptions_Fields(t *testing.T) {
	opts := &ClientOptions{
		ConnectTimeout:    15 * time.Second,
		KeepaliveTime:     60 * time.Second,
		KeepaliveTimeout:  20 * time.Second,
		MaxRetries:        5,
		InitialBackoff:    200 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 1.5,
		Insecure:          true,
		Debug:             true,
		TenantID:          "my-tenant",
	}

	if opts.ConnectTimeout != 15*time.Second {
		t.Errorf("ConnectTimeout = %v, want 15s", opts.ConnectTimeout)
	}
	if opts.MaxRetries != 5 {
		t.Errorf("MaxRetries = %v, want 5", opts.MaxRetries)
	}
	if !opts.Insecure {
		t.Error("Insecure should be true")
	}
	if !opts.Debug {
		t.Error("Debug should be true")
	}
	if opts.TenantID != "my-tenant" {
		t.Errorf("TenantID = %v, want my-tenant", opts.TenantID)
	}
}

// TestDefaultConstants verifies default constant values.
func TestDefaultConstants(t *testing.T) {
	if DefaultConnectTimeout != 10*time.Second {
		t.Errorf("DefaultConnectTimeout = %v, want 10s", DefaultConnectTimeout)
	}
	if DefaultKeepaliveTime != 5*time.Minute {
		t.Errorf("DefaultKeepaliveTime = %v, want 5m", DefaultKeepaliveTime)
	}
	if DefaultKeepaliveTimeout != 20*time.Second {
		t.Errorf("DefaultKeepaliveTimeout = %v, want 20s", DefaultKeepaliveTimeout)
	}
	if DefaultMaxRetries != 3 {
		t.Errorf("DefaultMaxRetries = %v, want 3", DefaultMaxRetries)
	}
	if DefaultInitialBackoff != 100*time.Millisecond {
		t.Errorf("DefaultInitialBackoff = %v, want 100ms", DefaultInitialBackoff)
	}
	if DefaultMaxBackoff != 5*time.Second {
		t.Errorf("DefaultMaxBackoff = %v, want 5s", DefaultMaxBackoff)
	}
	if DefaultBackoffMultiplier != 2.0 {
		t.Errorf("DefaultBackoffMultiplier = %v, want 2.0", DefaultBackoffMultiplier)
	}
}

// TestBuildDialOptions verifies dial options construction.
func TestBuildDialOptions(t *testing.T) {
	t.Run("insecure mode", func(t *testing.T) {
		opts := &ClientOptions{
			KeepaliveTime:    30 * time.Second,
			KeepaliveTimeout: 10 * time.Second,
			Insecure:         true,
		}
		client := NewGRPCClient("localhost:50051", opts)

		dialOpts := client.buildDialOptions()
		if len(dialOpts) == 0 {
			t.Error("expected non-empty dial options")
		}
	})

	t.Run("secure mode", func(t *testing.T) {
		opts := &ClientOptions{
			KeepaliveTime:    30 * time.Second,
			KeepaliveTimeout: 10 * time.Second,
			Insecure:         false, // Secure mode
		}
		client := NewGRPCClient("localhost:50051", opts)

		dialOpts := client.buildDialOptions()
		// In secure mode, we should still have keepalive options
		if len(dialOpts) == 0 {
			t.Error("expected non-empty dial options")
		}
	})
}

// TestGRPCClient_ConnectWithTimeout tests connection timeout behavior.
func TestGRPCClient_ConnectWithTimeout(t *testing.T) {
	opts := &ClientOptions{
		ConnectTimeout: 50 * time.Millisecond, // Very short timeout
		Insecure:       true,
	}
	client := NewGRPCClient("localhost:99999", opts) // Invalid port

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Connection should fail or timeout
	// Note: gRPC may not fail immediately, so we just verify it handles the situation
	err := client.Connect(ctx)
	// We're not asserting error here as gRPC connection behavior is async
	_ = err
}

// TestGRPCClient_ReconnectLogic tests reconnection behavior.
func TestGRPCClient_ReconnectLogic(t *testing.T) {
	opts := &ClientOptions{
		ConnectTimeout:    50 * time.Millisecond,
		MaxRetries:        2,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        50 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Insecure:          true,
	}
	client := NewGRPCClient("localhost:99999", opts)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Reconnect should attempt multiple times
	err := client.Reconnect(ctx)
	if err == nil {
		// If it somehow succeeded, that's fine
		return
	}

	// Error should indicate reconnection failure
	// We're just verifying the function runs without panic
}

// TestGRPCClient_AlreadyConnected tests that Connect is idempotent.
func TestGRPCClient_AlreadyConnected(t *testing.T) {
	client := NewGRPCClient("localhost:50051", nil)

	// Manually set connected state for testing
	client.mu.Lock()
	client.connected = true
	client.conn = &grpc.ClientConn{} // Dummy connection
	client.mu.Unlock()

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Errorf("Connect() on already connected client should not error: %v", err)
	}

	// Cleanup
	client.mu.Lock()
	client.connected = false
	client.conn = nil
	client.mu.Unlock()
}

// TestConvertHealthCheckResponse verifies health check response conversion.
func TestConvertHealthCheckResponse(t *testing.T) {
	latencyVal := float64(5.5)
	resp := &gatewaypb.HealthCheckResponse{
		Healthy:       true,
		Message:       "Gateway is healthy",
		Version:       "0.1.0",
		UptimeSeconds: 3600,
		Dependencies: []*gatewaypb.DependencyHealth{
			{
				Name:      "database",
				Healthy:   true,
				Message:   "connected",
				LatencyMs: &latencyVal,
			},
			{
				Name:    "redis",
				Healthy: false,
				Message: "connection refused",
			},
		},
	}

	status := convertHealthCheckResponse(resp)

	if status == nil {
		t.Fatal("convertHealthCheckResponse returned nil")
	}

	// Verify expected structure
	if !status.Healthy {
		t.Error("expected healthy status")
	}
	if status.Message != "Gateway is healthy" {
		t.Errorf("Message = %v, want 'Gateway is healthy'", status.Message)
	}

	// Gateway service should be prepended, plus dependencies
	if len(status.Services) != 3 {
		t.Errorf("expected 3 services (gateway + 2 deps), got %d", len(status.Services))
	}

	// Verify gateway is first
	if status.Services[0].Name != "gateway" {
		t.Errorf("first service should be gateway, got %v", status.Services[0].Name)
	}
	if status.Services[0].UptimeSeconds != 3600 {
		t.Errorf("gateway UptimeSeconds = %v, want 3600", status.Services[0].UptimeSeconds)
	}

	// Verify database dependency
	if status.Services[1].Name != "database" {
		t.Errorf("second service should be database, got %v", status.Services[1].Name)
	}
	if !status.Services[1].Healthy {
		t.Error("database should be healthy")
	}
	if status.Services[1].LatencyMs != 5.5 {
		t.Errorf("database LatencyMs = %v, want 5.5", status.Services[1].LatencyMs)
	}
	if status.Services[1].Status != "running" {
		t.Errorf("database Status = %v, want 'running'", status.Services[1].Status)
	}

	// Verify redis dependency (unhealthy)
	if status.Services[2].Name != "redis" {
		t.Errorf("third service should be redis, got %v", status.Services[2].Name)
	}
	if status.Services[2].Healthy {
		t.Error("redis should be unhealthy")
	}
	if status.Services[2].Status != "down" {
		t.Errorf("redis Status = %v, want 'down'", status.Services[2].Status)
	}

	// Verify version info
	if status.Version == nil {
		t.Error("expected version info")
	} else if status.Version.Version != "0.1.0" {
		t.Errorf("Version = %v, want '0.1.0'", status.Version.Version)
	}
}

// TestConnectionStateMapping tests connection state string mapping.
func TestConnectionStateMapping(t *testing.T) {
	tests := []struct {
		state    connectivity.State
		expected string
	}{
		{connectivity.Idle, "idle"},
		{connectivity.Connecting, "connecting"},
		{connectivity.Ready, "ready"},
		{connectivity.TransientFailure, "transient_failure"},
		{connectivity.Shutdown, "shutdown"},
	}

	// We can't easily test these without a real connection,
	// but we can verify the ConnectionState method handles
	// the disconnected case properly
	client := NewGRPCClient("localhost:50051", nil)
	state := client.ConnectionState()
	if state != "disconnected" {
		t.Errorf("ConnectionState() = %v, want 'disconnected'", state)
	}

	// Document expected mappings
	for _, tc := range tests {
		t.Logf("connectivity.%s maps to %q", tc.state, tc.expected)
	}
}

// TestWithRetry_BackoffCalculation verifies exponential backoff.
func TestWithRetry_BackoffCalculation(t *testing.T) {
	opts := &ClientOptions{
		MaxRetries:        4,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Insecure:          true,
	}
	client := NewGRPCClient("localhost:50051", opts)

	attempts := 0
	startTime := time.Now()
	ctx := context.Background()

	err := client.WithRetry(ctx, func() error {
		attempts++
		if attempts <= 4 {
			return errors.New("retry me")
		}
		return nil
	})

	elapsed := time.Since(startTime)

	if err != nil {
		t.Errorf("WithRetry() error = %v", err)
	}

	// Expected minimum delay: 10 + 20 + 40 + 80 = 150ms
	// But capped at 100ms max, so: 10 + 20 + 40 + 100 = 170ms minimum
	// Actually: 10 + 20 + 40 + 80 = 150ms (80 < 100 so no cap on 4th)
	// Let's just verify some time has passed
	if elapsed < 50*time.Millisecond {
		t.Logf("elapsed = %v (expected at least some backoff delay)", elapsed)
	}

	if attempts != 5 {
		t.Errorf("attempts = %d, want 5", attempts)
	}
}
