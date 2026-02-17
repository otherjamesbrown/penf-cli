// Package db provides shared PostgreSQL database utilities for Penfold microservices.
package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds PostgreSQL connection configuration.
type Config struct {
	Host            string
	Port            int
	Database        string
	User            string
	Password        string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	ConnectTimeout  time.Duration
}

// DefaultConfig returns a Config with sensible default values.
func DefaultConfig() *Config {
	return &Config{
		Host:            "localhost",
		Port:            5432,
		Database:        "penfold",
		User:            "penfold",
		Password:        "",
		SSLMode:         "disable",
		MaxConns:        25,
		MinConns:        5,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  10 * time.Second,
	}
}

// ConfigFromEnv creates a Config from environment variables.
// Environment variables:
//   - DB_HOST: Database host (default: localhost)
//   - DB_PORT: Database port (default: 5432)
//   - DB_NAME: Database name (default: penfold)
//   - DB_USER: Database user (default: penfold)
//   - DB_PASSWORD: Database password
//   - DB_SSLMODE: SSL mode (default: disable)
//   - DB_MAX_CONNS: Maximum connections (default: 25)
//   - DB_MIN_CONNS: Minimum connections (default: 5)
func ConfigFromEnv() *Config {
	cfg := DefaultConfig()

	if host := os.Getenv("DB_HOST"); host != "" {
		cfg.Host = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}
	if database := os.Getenv("DB_NAME"); database != "" {
		cfg.Database = database
	}
	if user := os.Getenv("DB_USER"); user != "" {
		cfg.User = user
	}
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		cfg.Password = password
	}
	if sslmode := os.Getenv("DB_SSLMODE"); sslmode != "" {
		cfg.SSLMode = sslmode
	}
	if maxConns := os.Getenv("DB_MAX_CONNS"); maxConns != "" {
		if mc, err := strconv.ParseInt(maxConns, 10, 32); err == nil {
			cfg.MaxConns = int32(mc)
		}
	}
	if minConns := os.Getenv("DB_MIN_CONNS"); minConns != "" {
		if mc, err := strconv.ParseInt(minConns, 10, 32); err == nil {
			cfg.MinConns = int32(mc)
		}
	}

	return cfg
}

// ConnectionString builds a PostgreSQL connection string from the config.
func (c *Config) ConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s&connect_timeout=%d",
		url.QueryEscape(c.User),
		url.QueryEscape(c.Password),
		c.Host,
		c.Port,
		c.Database,
		c.SSLMode,
		int(c.ConnectTimeout.Seconds()),
	)
}

// Validate checks if the config has required fields set.
func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", c.Port)
	}
	if c.Database == "" {
		return fmt.Errorf("database name is required")
	}
	if c.User == "" {
		return fmt.Errorf("database user is required")
	}
	if c.MaxConns < c.MinConns {
		return fmt.Errorf("max connections (%d) must be >= min connections (%d)", c.MaxConns, c.MinConns)
	}
	return nil
}

// Connect creates a new connection pool with the given configuration.
// The caller is responsible for calling pool.Close() when done.
func Connect(ctx context.Context, cfg *Config) (*pgxpool.Pool, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify the connection works
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// ConnectWithRetry creates a connection pool with retry logic.
func ConnectWithRetry(ctx context.Context, cfg *Config, maxAttempts int, retryDelay time.Duration) (*pgxpool.Pool, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		pool, err := Connect(ctx, cfg)
		if err == nil {
			return pool, nil
		}
		lastErr = err

		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryDelay):
				// Continue to next attempt
			}
		}
	}

	return nil, fmt.Errorf("failed to connect after %d attempts: %w", maxAttempts, lastErr)
}

// Close gracefully closes a connection pool if it is not nil.
func Close(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}
