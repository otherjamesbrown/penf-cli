// Package client provides the gRPC client for connecting to the Penfold API Gateway.
// This file contains TLS configuration and certificate loading for mTLS authentication.
package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/otherjamesbrown/penf-cli/config"
)

// LoadClientTLSConfig creates a tls.Config for the gRPC client.
// Returns nil if TLS is not enabled in the configuration.
func LoadClientTLSConfig(cfg *config.TLSConfig) (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Resolve paths (expands ~ and sets defaults from CertDir).
	cfg.ResolvePaths()

	// Load client certificate and key for mTLS authentication.
	cert, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
	if err != nil {
		return nil, fmt.Errorf("load client cert: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.SkipVerify,
	}

	// Load CA certificate for server verification (unless SkipVerify is set).
	if cfg.CACert != "" && !cfg.SkipVerify {
		caCert, err := os.ReadFile(cfg.CACert)
		if err != nil {
			return nil, fmt.Errorf("read CA cert: %w", err)
		}

		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("parse CA cert: invalid PEM")
		}

		tlsConfig.RootCAs = caPool
	}

	return tlsConfig, nil
}

// CheckCertsExist verifies all required certificate files are present.
// This is useful for providing clear error messages before attempting to connect.
func CheckCertsExist(cfg *config.TLSConfig) error {
	// Resolve paths first.
	cfg.ResolvePaths()

	files := map[string]string{
		"CA certificate":     cfg.CACert,
		"Client certificate": cfg.ClientCert,
		"Client key":         cfg.ClientKey,
	}

	for name, path := range files {
		if path == "" {
			return fmt.Errorf("%s not configured", name)
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("%s not found: %s", name, path)
		}
	}

	return nil
}
