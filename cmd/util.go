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
func connectToDatabase(ctx context.Context, cfg *config.CLIConfig) (*pgxpool.Pool, error) {
	// Build connection string from config or environment
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		// Build from individual components
		host := getEnvOrDefault("DB_HOST", "localhost")
		port := getEnvOrDefault("DB_PORT", "5432")
		user := getEnvOrDefault("DB_USER", "penfold")
		pass := os.Getenv("DB_PASSWORD")
		dbname := getEnvOrDefault("DB_NAME", "penfold")
		sslmode := getEnvOrDefault("DB_SSLMODE", "prefer")

		// Start with base connection string
		if pass != "" {
			connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
				host, port, user, pass, dbname, sslmode)
		} else {
			connStr = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s",
				host, port, user, dbname, sslmode)
		}

		// Add SSL cert paths if provided (for cert-based auth)
		if sslcert := os.Getenv("DB_SSLCERT"); sslcert != "" {
			connStr += fmt.Sprintf(" sslcert=%s", sslcert)
		}
		if sslkey := os.Getenv("DB_SSLKEY"); sslkey != "" {
			connStr += fmt.Sprintf(" sslkey=%s", sslkey)
		}
		if sslrootcert := os.Getenv("DB_SSLROOTCERT"); sslrootcert != "" {
			connStr += fmt.Sprintf(" sslrootcert=%s", sslrootcert)
		}
	}

	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	poolCfg.MaxConns = 25
	poolCfg.MinConns = 2
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
