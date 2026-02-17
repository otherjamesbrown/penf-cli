package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthStatus represents the health state of a database connection.
type HealthStatus struct {
	Healthy       bool
	Latency       time.Duration
	TotalConns    int32
	IdleConns     int32
	AcquiredConns int32
	Error         error
}

// Ping checks if the database is reachable.
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("pool is nil")
	}
	return pool.Ping(ctx)
}

// Check performs a comprehensive health check and returns detailed status.
func Check(ctx context.Context, pool *pgxpool.Pool) *HealthStatus {
	status := &HealthStatus{}

	if pool == nil {
		status.Error = fmt.Errorf("pool is nil")
		return status
	}

	start := time.Now()
	err := pool.Ping(ctx)
	status.Latency = time.Since(start)

	if err != nil {
		status.Error = fmt.Errorf("ping failed: %w", err)
		return status
	}

	stats := pool.Stat()
	status.Healthy = true
	status.TotalConns = stats.TotalConns()
	status.IdleConns = stats.IdleConns()
	status.AcquiredConns = stats.AcquiredConns()

	return status
}

// WaitForReady polls the database until it becomes available or context is cancelled.
func WaitForReady(ctx context.Context, pool *pgxpool.Pool, pollInterval time.Duration) error {
	if pool == nil {
		return fmt.Errorf("pool is nil")
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Try immediately first
	if err := pool.Ping(ctx); err == nil {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := pool.Ping(ctx); err == nil {
				return nil
			}
		}
	}
}
