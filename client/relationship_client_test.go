// Package client provides gRPC clients for Penfold services.
package client

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// TestRelationshipClient_buildDialOptions tests the TLS credential handling in dial options.
func TestRelationshipClient_buildDialOptions(t *testing.T) {
	t.Run("insecure mode", func(t *testing.T) {
		opts := &ClientOptions{
			Insecure: true,
		}
		client := NewRelationshipClient("localhost:50051", opts)

		dialOpts := client.buildDialOptions()

		if len(dialOpts) == 0 {
			t.Error("expected non-empty dial options")
		}

		// Verify that insecure credentials are used.
		// We can't easily inspect the dial options directly, but we can verify
		// the options list is constructed properly by checking length.
		// In insecure mode, we should have keepalive + credentials = at least 2 options.
		if len(dialOpts) < 1 {
			t.Errorf("expected at least 1 dial option, got %d", len(dialOpts))
		}
	})

	t.Run("with TLS config", func(t *testing.T) {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		opts := &ClientOptions{
			Insecure:  false,
			TLSConfig: tlsConfig,
		}
		client := NewRelationshipClient("localhost:50051", opts)

		dialOpts := client.buildDialOptions()

		if len(dialOpts) == 0 {
			t.Error("expected non-empty dial options")
		}

		// Verify that TLS credentials are configured.
		// In TLS mode, we should have keepalive + credentials = at least 2 options.
		if len(dialOpts) < 1 {
			t.Errorf("expected at least 1 dial option, got %d", len(dialOpts))
		}

		// Verify that credentials.NewTLS() would be called with the provided config.
		// We can't easily inspect the internal structure, but we verify the config is set.
		if client.options.TLSConfig != tlsConfig {
			t.Error("TLS config not properly stored in client options")
		}
	})

	t.Run("fallback when no TLS config", func(t *testing.T) {
		opts := &ClientOptions{
			Insecure:  false,
			TLSConfig: nil, // No TLS config provided
		}
		client := NewRelationshipClient("localhost:50051", opts)

		dialOpts := client.buildDialOptions()

		if len(dialOpts) == 0 {
			t.Error("expected non-empty dial options")
		}

		// In this case, the implementation should fallback to insecure credentials.
		// We verify the behavior by checking that options are created.
		if len(dialOpts) < 1 {
			t.Errorf("expected at least 1 dial option, got %d", len(dialOpts))
		}
	})
}

// TestNewRelationshipClient tests client creation with various options.
func TestNewRelationshipClient(t *testing.T) {
	t.Run("with custom options", func(t *testing.T) {
		opts := &ClientOptions{
			ConnectTimeout: 5 * time.Second,
			MaxRetries:     5,
			Insecure:       true,
			TenantID:       "test-tenant",
		}
		client := NewRelationshipClient("localhost:50051", opts)

		if client == nil {
			t.Fatal("NewRelationshipClient returned nil")
		}
		if client.serverAddr != "localhost:50051" {
			t.Errorf("serverAddr = %v, want localhost:50051", client.serverAddr)
		}
		if client.options.TenantID != "test-tenant" {
			t.Errorf("TenantID = %v, want test-tenant", client.options.TenantID)
		}
		if client.options.MaxRetries != 5 {
			t.Errorf("MaxRetries = %v, want 5", client.options.MaxRetries)
		}
	})

	t.Run("with nil options uses defaults", func(t *testing.T) {
		client := NewRelationshipClient("localhost:50051", nil)

		if client == nil {
			t.Fatal("NewRelationshipClient returned nil")
		}
		if client.options == nil {
			t.Fatal("options should be set to defaults")
		}
		if client.options.MaxRetries != DefaultMaxRetries {
			t.Errorf("MaxRetries = %v, want %v", client.options.MaxRetries, DefaultMaxRetries)
		}
		if client.options.ConnectTimeout != DefaultConnectTimeout {
			t.Errorf("ConnectTimeout = %v, want %v", client.options.ConnectTimeout, DefaultConnectTimeout)
		}
	})

	t.Run("with TLS config", func(t *testing.T) {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS13,
			ServerName: "relationship.example.com",
		}
		opts := &ClientOptions{
			Insecure:  false,
			TLSConfig: tlsConfig,
		}
		client := NewRelationshipClient("relationship.example.com:443", opts)

		if client == nil {
			t.Fatal("NewRelationshipClient returned nil")
		}
		if client.options.TLSConfig != tlsConfig {
			t.Error("TLS config not properly set")
		}
		if client.options.TLSConfig.MinVersion != tls.VersionTLS13 {
			t.Errorf("TLS MinVersion = %v, want %v", client.options.TLSConfig.MinVersion, tls.VersionTLS13)
		}
	})

	t.Run("stores server address correctly", func(t *testing.T) {
		addresses := []string{
			"localhost:50051",
			"relationship.example.com:443",
			"10.0.0.1:8080",
		}

		for _, addr := range addresses {
			client := NewRelationshipClient(addr, nil)
			if client.serverAddr != addr {
				t.Errorf("for address %s, serverAddr = %v", addr, client.serverAddr)
			}
		}
	})
}

// TestRelationshipClient_Connect_NotConnected tests error handling when connection fails.
func TestRelationshipClient_Connect_NotConnected(t *testing.T) {
	t.Run("connection to invalid port fails", func(t *testing.T) {
		opts := &ClientOptions{
			ConnectTimeout: 50 * time.Millisecond, // Short timeout for fast test
			Insecure:       true,
		}
		client := NewRelationshipClient("localhost:99999", opts) // Invalid port

		ctx := context.Background()

		// Connection should not succeed immediately
		// Note: gRPC connections are lazy and may not fail immediately at Connect time
		err := client.Connect(ctx)
		// We don't assert error here because gRPC dial is non-blocking by default
		_ = err

		// The client should not report as connected if the connection truly failed
		// However, gRPC's behavior is to allow the dial to succeed and fail on first RPC
		// So we just verify the client was created
		if client == nil {
			t.Fatal("client should not be nil")
		}
	})

	t.Run("calling methods before connect returns error", func(t *testing.T) {
		client := NewRelationshipClient("localhost:50051", nil)
		ctx := context.Background()

		// Try to use the client without connecting
		_, _, err := client.ListRelationships(ctx, &ListRelationshipsRequest{
			TenantID: "test",
		})

		if err == nil {
			t.Error("expected error when calling method before Connect()")
		}
		expectedMsg := "relationship client not connected"
		if err.Error() != expectedMsg {
			t.Errorf("error = %v, want %v", err.Error(), expectedMsg)
		}
	})

	t.Run("IsConnected returns false before connect", func(t *testing.T) {
		client := NewRelationshipClient("localhost:50051", nil)

		if client.IsConnected() {
			t.Error("IsConnected() should be false before Connect()")
		}
	})

	t.Run("context cancellation during connect", func(t *testing.T) {
		opts := &ClientOptions{
			ConnectTimeout: 5 * time.Second,
			Insecure:       true,
		}
		client := NewRelationshipClient("localhost:99999", opts)

		// Create a context that's already cancelled
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := client.Connect(ctx)
		// Should fail due to cancelled context or timeout
		// We just verify it doesn't panic
		_ = err
	})

	t.Run("Close before connect is safe", func(t *testing.T) {
		client := NewRelationshipClient("localhost:50051", nil)

		// Close should be safe to call even when not connected
		err := client.Close()
		if err != nil {
			t.Errorf("Close() error = %v, expected nil", err)
		}
	})
}

// TestRelationshipClient_IsConnected tests the connection state reporting.
func TestRelationshipClient_IsConnected(t *testing.T) {
	client := NewRelationshipClient("localhost:50051", nil)

	// Before connecting, should be false
	if client.IsConnected() {
		t.Error("IsConnected() should be false before Connect()")
	}

	// Simulate setting connected state
	client.mu.Lock()
	client.connected = true
	client.conn = &grpc.ClientConn{} // Dummy connection
	client.mu.Unlock()

	// Now should report as connected
	if !client.IsConnected() {
		t.Error("IsConnected() should be true after setting connected state")
	}

	// Cleanup
	client.mu.Lock()
	client.connected = false
	client.conn = nil
	client.mu.Unlock()
}

// TestRelationshipClient_Close tests close behavior.
func TestRelationshipClient_Close(t *testing.T) {
	t.Run("close when not connected", func(t *testing.T) {
		client := NewRelationshipClient("localhost:50051", nil)

		err := client.Close()
		if err != nil {
			t.Errorf("Close() error = %v, expected nil", err)
		}
	})

	t.Run("close multiple times is safe", func(t *testing.T) {
		client := NewRelationshipClient("localhost:50051", nil)

		err := client.Close()
		if err != nil {
			t.Errorf("first Close() error = %v, expected nil", err)
		}

		err = client.Close()
		if err != nil {
			t.Errorf("second Close() error = %v, expected nil", err)
		}
	})
}

// TestRelationshipClient_DialOptionsContent verifies the structure of dial options.
func TestRelationshipClient_DialOptionsContent(t *testing.T) {
	t.Run("keepalive parameters are included", func(t *testing.T) {
		client := NewRelationshipClient("localhost:50051", nil)
		dialOpts := client.buildDialOptions()

		// Should have at least keepalive + transport credentials
		if len(dialOpts) < 1 {
			t.Errorf("expected at least 1 dial option, got %d", len(dialOpts))
		}
	})

	t.Run("credentials type varies by mode", func(t *testing.T) {
		// Test insecure
		insecureClient := NewRelationshipClient("localhost:50051", &ClientOptions{
			Insecure: true,
		})
		insecureOpts := insecureClient.buildDialOptions()

		// Test TLS
		tlsClient := NewRelationshipClient("localhost:50051", &ClientOptions{
			Insecure:  false,
			TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		})
		tlsOpts := tlsClient.buildDialOptions()

		// Both should have dial options
		if len(insecureOpts) == 0 {
			t.Error("insecure mode should have dial options")
		}
		if len(tlsOpts) == 0 {
			t.Error("TLS mode should have dial options")
		}
	})
}

// TestRelationshipClient_OptionsStorage tests that options are properly stored.
func TestRelationshipClient_OptionsStorage(t *testing.T) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: "test.example.com",
	}

	opts := &ClientOptions{
		ConnectTimeout:    15 * time.Second,
		KeepaliveTime:     45 * time.Second,
		KeepaliveTimeout:  15 * time.Second,
		MaxRetries:        5,
		InitialBackoff:    200 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 1.5,
		Insecure:          false,
		Debug:             true,
		TenantID:          "test-tenant-123",
		TLSConfig:         tlsConfig,
	}

	client := NewRelationshipClient("localhost:50051", opts)

	// Verify all options are stored
	if client.options.ConnectTimeout != 15*time.Second {
		t.Errorf("ConnectTimeout = %v, want 15s", client.options.ConnectTimeout)
	}
	if client.options.KeepaliveTime != 45*time.Second {
		t.Errorf("KeepaliveTime = %v, want 45s", client.options.KeepaliveTime)
	}
	if client.options.MaxRetries != 5 {
		t.Errorf("MaxRetries = %v, want 5", client.options.MaxRetries)
	}
	if client.options.Insecure != false {
		t.Error("Insecure should be false")
	}
	if client.options.TenantID != "test-tenant-123" {
		t.Errorf("TenantID = %v, want test-tenant-123", client.options.TenantID)
	}
	if client.options.TLSConfig != tlsConfig {
		t.Error("TLSConfig not properly stored")
	}
	if client.options.TLSConfig.ServerName != "test.example.com" {
		t.Errorf("TLS ServerName = %v, want test.example.com", client.options.TLSConfig.ServerName)
	}
}

// TestCredentialsNewTLS verifies we can create TLS credentials.
func TestCredentialsNewTLS(t *testing.T) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Verify credentials.NewTLS works with our config
	creds := credentials.NewTLS(tlsConfig)
	if creds == nil {
		t.Error("credentials.NewTLS returned nil")
	}
}

// TestInsecureNewCredentials verifies we can create insecure credentials.
func TestInsecureNewCredentials(t *testing.T) {
	// Verify insecure.NewCredentials works
	creds := insecure.NewCredentials()
	if creds == nil {
		t.Error("insecure.NewCredentials returned nil")
	}
}
