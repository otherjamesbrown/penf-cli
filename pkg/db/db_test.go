package db

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("expected host 'localhost', got '%s'", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("expected port 5432, got %d", cfg.Port)
	}
	if cfg.Database != "penfold" {
		t.Errorf("expected database 'penfold', got '%s'", cfg.Database)
	}
	if cfg.User != "penfold" {
		t.Errorf("expected user 'penfold', got '%s'", cfg.User)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("expected sslmode 'disable', got '%s'", cfg.SSLMode)
	}
	if cfg.MaxConns != 25 {
		t.Errorf("expected max conns 25, got %d", cfg.MaxConns)
	}
	if cfg.MinConns != 5 {
		t.Errorf("expected min conns 5, got %d", cfg.MinConns)
	}
}

func TestConfigFromEnv(t *testing.T) {
	// Save original env vars
	origHost := os.Getenv("DB_HOST")
	origPort := os.Getenv("DB_PORT")
	origName := os.Getenv("DB_NAME")
	origUser := os.Getenv("DB_USER")
	origPass := os.Getenv("DB_PASSWORD")
	origSSL := os.Getenv("DB_SSLMODE")
	origMaxConns := os.Getenv("DB_MAX_CONNS")
	origMinConns := os.Getenv("DB_MIN_CONNS")

	// Restore original env vars after test
	defer func() {
		os.Setenv("DB_HOST", origHost)
		os.Setenv("DB_PORT", origPort)
		os.Setenv("DB_NAME", origName)
		os.Setenv("DB_USER", origUser)
		os.Setenv("DB_PASSWORD", origPass)
		os.Setenv("DB_SSLMODE", origSSL)
		os.Setenv("DB_MAX_CONNS", origMaxConns)
		os.Setenv("DB_MIN_CONNS", origMinConns)
	}()

	// Set test env vars
	os.Setenv("DB_HOST", "testhost")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_NAME", "testdb")
	os.Setenv("DB_USER", "testuser")
	os.Setenv("DB_PASSWORD", "testpass")
	os.Setenv("DB_SSLMODE", "require")
	os.Setenv("DB_MAX_CONNS", "50")
	os.Setenv("DB_MIN_CONNS", "10")

	cfg := ConfigFromEnv()

	if cfg.Host != "testhost" {
		t.Errorf("expected host 'testhost', got '%s'", cfg.Host)
	}
	if cfg.Port != 5433 {
		t.Errorf("expected port 5433, got %d", cfg.Port)
	}
	if cfg.Database != "testdb" {
		t.Errorf("expected database 'testdb', got '%s'", cfg.Database)
	}
	if cfg.User != "testuser" {
		t.Errorf("expected user 'testuser', got '%s'", cfg.User)
	}
	if cfg.Password != "testpass" {
		t.Errorf("expected password 'testpass', got '%s'", cfg.Password)
	}
	if cfg.SSLMode != "require" {
		t.Errorf("expected sslmode 'require', got '%s'", cfg.SSLMode)
	}
	if cfg.MaxConns != 50 {
		t.Errorf("expected max conns 50, got %d", cfg.MaxConns)
	}
	if cfg.MinConns != 10 {
		t.Errorf("expected min conns 10, got %d", cfg.MinConns)
	}
}

func TestConfigFromEnv_InvalidPort(t *testing.T) {
	origPort := os.Getenv("DB_PORT")
	defer os.Setenv("DB_PORT", origPort)

	os.Setenv("DB_PORT", "invalid")
	cfg := ConfigFromEnv()

	// Should fall back to default
	if cfg.Port != 5432 {
		t.Errorf("expected default port 5432 for invalid input, got %d", cfg.Port)
	}
}

func TestConnectionString(t *testing.T) {
	cfg := &Config{
		Host:           "myhost",
		Port:           5432,
		Database:       "mydb",
		User:           "myuser",
		Password:       "mypass",
		SSLMode:        "disable",
		ConnectTimeout: 10 * time.Second,
	}

	connStr := cfg.ConnectionString()
	expected := "postgres://myuser:mypass@myhost:5432/mydb?sslmode=disable&connect_timeout=10"

	if connStr != expected {
		t.Errorf("connection string mismatch:\nexpected: %s\ngot:      %s", expected, connStr)
	}
}

func TestConnectionString_SpecialChars(t *testing.T) {
	cfg := &Config{
		Host:           "localhost",
		Port:           5432,
		Database:       "testdb",
		User:           "user@domain",
		Password:       "pass:word/test",
		SSLMode:        "disable",
		ConnectTimeout: 5 * time.Second,
	}

	connStr := cfg.ConnectionString()

	// Should URL-encode special characters
	if connStr != "postgres://user%40domain:pass%3Aword%2Ftest@localhost:5432/testdb?sslmode=disable&connect_timeout=5" {
		t.Errorf("special characters not properly encoded in connection string: %s", connStr)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "missing host",
			cfg: &Config{
				Host:     "",
				Port:     5432,
				Database: "db",
				User:     "user",
				MaxConns: 10,
				MinConns: 5,
			},
			wantErr: true,
		},
		{
			name: "invalid port zero",
			cfg: &Config{
				Host:     "localhost",
				Port:     0,
				Database: "db",
				User:     "user",
				MaxConns: 10,
				MinConns: 5,
			},
			wantErr: true,
		},
		{
			name: "invalid port too high",
			cfg: &Config{
				Host:     "localhost",
				Port:     70000,
				Database: "db",
				User:     "user",
				MaxConns: 10,
				MinConns: 5,
			},
			wantErr: true,
		},
		{
			name: "missing database",
			cfg: &Config{
				Host:     "localhost",
				Port:     5432,
				Database: "",
				User:     "user",
				MaxConns: 10,
				MinConns: 5,
			},
			wantErr: true,
		},
		{
			name: "missing user",
			cfg: &Config{
				Host:     "localhost",
				Port:     5432,
				Database: "db",
				User:     "",
				MaxConns: 10,
				MinConns: 5,
			},
			wantErr: true,
		},
		{
			name: "max conns less than min conns",
			cfg: &Config{
				Host:     "localhost",
				Port:     5432,
				Database: "db",
				User:     "user",
				MaxConns: 5,
				MinConns: 10,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
