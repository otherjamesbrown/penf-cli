package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// getEnvOrDefault returns environment variable value or default.
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// connectToDatabase establishes a database connection.
// Connection is resolved in order: DATABASE_URL env var > config file database section > DB_* env vars.
func connectToDatabase(ctx context.Context, cfg *config.CLIConfig) (*pgxpool.Pool, error) {
	connStr := ""

	// 1. DATABASE_URL env var (full connection string)
	if v := os.Getenv("DATABASE_URL"); v != "" {
		connStr = v
	}

	// 2. Config file database section
	if connStr == "" && cfg != nil && cfg.Database != nil && cfg.Database.IsConfigured() {
		connStr = cfg.Database.ConnectionString()
	}

	// 3. No database configured
	if connStr == "" {
		return nil, fmt.Errorf("database not configured: add 'database' section to ~/.penf/config.yaml or set DATABASE_URL")
	}

	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	poolCfg.MaxConns = 5
	poolCfg.MinConns = 1
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("testing connection: %w", err)
	}

	return pool, nil
}

// connectToRedis establishes a Redis connection.
func connectToRedis(ctx context.Context, cfg *config.CLIConfig) (*redis.Client, error) {
	host := getEnvOrDefault("REDIS_HOST", "localhost")
	port := getEnvOrDefault("REDIS_PORT", "6379")
	password := os.Getenv("REDIS_PASSWORD")

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       0,
	})

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("testing connection: %w", err)
	}

	return client, nil
}

// connectToGateway creates a gRPC connection to the gateway service.
func connectToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if cfg.TLS.Enabled {
		tlsConfig, err := client.LoadClientTLSConfig(&cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("loading TLS config: %w", err)
		}
		if tlsConfig != nil {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, cfg.ServerAddress, opts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to gateway at %s: %w", cfg.ServerAddress, err)
	}

	return conn, nil
}

// resolveTenantID returns the tenant ID from config or PENF_TENANT_ID env var.
func resolveTenantID(cfg *config.CLIConfig) (string, error) {
	if cfg != nil && cfg.TenantID != "" {
		return cfg.TenantID, nil
	}
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant, nil
	}
	return "", fmt.Errorf("tenant ID required: set PENF_TENANT_ID env var or tenant_id in config")
}

// resolveOutputFormat returns the output format from the flag override, config, or default.
func resolveOutputFormat(flagOverride string, cfg *config.CLIConfig) config.OutputFormat {
	if flagOverride != "" {
		return config.OutputFormat(flagOverride)
	}
	if cfg != nil {
		return cfg.OutputFormat
	}
	return config.OutputFormatText
}

// formatDurationMs formats milliseconds as a human-readable duration.
func formatDurationMs(ms int) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	if ms < 60000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%.1fm", float64(ms)/60000)
}
