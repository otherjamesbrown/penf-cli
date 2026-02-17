// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// CertVerifyResult represents the output of cert verify command.
type CertVerifyResult struct {
	LocalCerts    LocalCertsResult  `json:"local_certs" yaml:"local_certs"`
	Connection    *ConnectionResult `json:"connection,omitempty" yaml:"connection,omitempty"`
	OverallStatus string            `json:"overall_status" yaml:"overall_status"`
	Errors        []string          `json:"errors,omitempty" yaml:"errors,omitempty"`
}

// LocalCertsResult represents the local certificate verification results.
type LocalCertsResult struct {
	ClientCertValid bool   `json:"client_cert_valid" yaml:"client_cert_valid"`
	CACertValid     bool   `json:"ca_cert_valid" yaml:"ca_cert_valid"`
	ChainVerified   bool   `json:"chain_verified" yaml:"chain_verified"`
	ClientCertPath  string `json:"client_cert_path" yaml:"client_cert_path"`
	CACertPath      string `json:"ca_cert_path" yaml:"ca_cert_path"`
	ClientKeyPath   string `json:"client_key_path" yaml:"client_key_path"`
	ExpiresAt       string `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	Subject         string `json:"subject,omitempty" yaml:"subject,omitempty"`
	Issuer          string `json:"issuer,omitempty" yaml:"issuer,omitempty"`
	Error           string `json:"error,omitempty" yaml:"error,omitempty"`
}

// ConnectionResult represents the connection test results.
type ConnectionResult struct {
	ServerAddress      string `json:"server_address" yaml:"server_address"`
	TLSHandshake       bool   `json:"tls_handshake" yaml:"tls_handshake"`
	ServerCertVerified bool   `json:"server_cert_verified" yaml:"server_cert_verified"`
	ClientCertAccepted bool   `json:"client_cert_accepted" yaml:"client_cert_accepted"`
	GatewayResponding  bool   `json:"gateway_responding" yaml:"gateway_responding"`
	Error              string `json:"error,omitempty" yaml:"error,omitempty"`
}

var (
	certVerifyLocal   bool
	certVerifyVerbose bool
	certVerifyOutput  string
)

// NewCertVerifyCommand creates the cert verify command.
func NewCertVerifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify TLS certificates and test connection",
		Long: `Verify TLS certificates and test the mTLS connection to the gateway.

This command performs the following checks:
  1. Verifies local certificate files exist and are readable
  2. Validates certificate formats and expiration
  3. Verifies the certificate chain (client cert signed by CA)
  4. Tests TLS handshake with the gateway (unless --local)
  5. Verifies the gateway accepts the client certificate

Use --local to only verify local certificates without testing the connection.
Use -v/--verbose for detailed certificate information.

Examples:
  # Verify certs and test connection
  penf cert verify

  # Just verify local certs (no connection)
  penf cert verify --local

  # Verbose output with certificate details
  penf cert verify -v

  # JSON output for scripting
  penf cert verify -o json`,
		RunE: runCertVerify,
	}

	// Silence usage on error - we provide our own messages
	cmd.SilenceUsage = true

	cmd.Flags().BoolVar(&certVerifyLocal, "local", false, "Only verify local certs, skip connection test")
	cmd.Flags().BoolVarP(&certVerifyVerbose, "verbose", "v", false, "Show detailed certificate information")
	cmd.Flags().StringVarP(&certVerifyOutput, "output", "o", "text", "Output format: text, json, yaml")

	return cmd
}

// runCertVerify executes the cert verify command.
func runCertVerify(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	result := CertVerifyResult{
		OverallStatus: "passed",
	}

	// 1. Verify local certificates
	if certVerifyOutput == "text" {
		fmt.Println("Checking local certificates...")
	}

	localResult, certInfo, localErr := verifyLocalCertsWithInfo(&cfg.TLS)
	result.LocalCerts = localResult

	if localErr != nil {
		result.OverallStatus = "failed"
		result.Errors = append(result.Errors, localErr.Error())
		if certVerifyOutput == "text" {
			printLocalCertVerifyError(localErr)
			printCertTroubleshootingHints("local", localErr)
		}
	} else {
		if certVerifyOutput == "text" {
			printLocalCertVerifySuccess(localResult, certInfo, certVerifyVerbose)
		}

		// 2. Test connection (unless --local flag) - only if local certs passed
		if !certVerifyLocal {
			if certVerifyOutput == "text" {
				fmt.Printf("\nTesting connection to %s...\n", cfg.ServerAddress)
			}

			connResult, connErr := testTLSConnection(cmd.Context(), cfg)
			result.Connection = connResult

			if connErr != nil {
				result.OverallStatus = "failed"
				result.Errors = append(result.Errors, connErr.Error())
				if certVerifyOutput == "text" {
					printCertConnectionError(connResult, connErr)
					printCertTroubleshootingHints("connection", connErr)
				}
			} else if certVerifyOutput == "text" {
				printCertConnectionSuccess(connResult)
			}
		}
	}

	// Output results
	return outputCertVerifyResult(result)
}

// outputCertVerifyResult outputs the verification result in the configured format.
// For failed verification, it exits directly with code 1 to avoid duplicate error messages.
func outputCertVerifyResult(result CertVerifyResult) error {
	switch certVerifyOutput {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		if err := enc.Encode(result); err != nil {
			return err
		}
	default:
		if result.OverallStatus == "passed" {
			fmt.Println("\nAll checks passed")
		}
	}

	// For failed verification, exit directly with code 1 to avoid duplicate error messages
	// from Cobra and main.go error handling
	if result.OverallStatus == "failed" {
		os.Exit(1)
	}

	return nil
}

// certInfo holds parsed certificate information for display.
type certInfo struct {
	Subject   string
	Issuer    string
	NotBefore time.Time
	NotAfter  time.Time
}

// verifyLocalCertsWithInfo verifies all local certificate files and returns cert info.
func verifyLocalCertsWithInfo(tlsCfg *config.TLSConfig) (LocalCertsResult, *certInfo, error) {
	result := LocalCertsResult{}
	var info *certInfo

	// Resolve paths (expands ~ and sets defaults from CertDir)
	tlsCfg.ResolvePaths()

	result.ClientCertPath = tlsCfg.ClientCert
	result.CACertPath = tlsCfg.CACert
	result.ClientKeyPath = tlsCfg.ClientKey

	// Check if TLS is enabled
	if !tlsCfg.Enabled {
		result.Error = "TLS is not enabled in configuration"
		return result, nil, fmt.Errorf("TLS is not enabled in configuration. Set tls.enabled: true in config")
	}

	// Check cert files exist using the client package helper
	if err := client.CheckCertsExist(tlsCfg); err != nil {
		result.Error = err.Error()
		return result, nil, err
	}

	// Load and validate CA certificate
	caCertPEM, err := os.ReadFile(tlsCfg.CACert)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read CA certificate: %v", err)
		return result, nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCertPEM) {
		result.Error = "invalid CA certificate PEM format"
		return result, nil, fmt.Errorf("invalid CA certificate PEM format")
	}
	result.CACertValid = true

	// Load and validate client certificate and key
	cert, err := tls.LoadX509KeyPair(tlsCfg.ClientCert, tlsCfg.ClientKey)
	if err != nil {
		result.Error = fmt.Sprintf("failed to load client certificate/key: %v", err)
		return result, nil, fmt.Errorf("failed to load client certificate/key: %w", err)
	}
	result.ClientCertValid = true

	// Parse the client certificate for chain verification
	if len(cert.Certificate) == 0 {
		result.Error = "client certificate is empty"
		return result, nil, fmt.Errorf("client certificate is empty")
	}

	clientCert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		result.Error = fmt.Sprintf("failed to parse client certificate: %v", err)
		return result, nil, fmt.Errorf("failed to parse client certificate: %w", err)
	}

	// Extract cert info for display
	info = &certInfo{
		Subject:   clientCert.Subject.CommonName,
		Issuer:    clientCert.Issuer.CommonName,
		NotBefore: clientCert.NotBefore,
		NotAfter:  clientCert.NotAfter,
	}
	result.Subject = info.Subject
	result.Issuer = info.Issuer
	result.ExpiresAt = info.NotAfter.Format(time.RFC3339)

	// Check certificate expiration
	now := time.Now()
	if now.Before(clientCert.NotBefore) {
		result.Error = fmt.Sprintf("client certificate not yet valid (valid from %s)", clientCert.NotBefore.Format(time.RFC3339))
		return result, info, fmt.Errorf("client certificate not yet valid (valid from %s)", clientCert.NotBefore.Format(time.RFC3339))
	}
	if now.After(clientCert.NotAfter) {
		result.Error = fmt.Sprintf("client certificate has expired (expired %s)", clientCert.NotAfter.Format(time.RFC3339))
		return result, info, fmt.Errorf("client certificate has expired (expired %s)", clientCert.NotAfter.Format(time.RFC3339))
	}

	// Verify certificate chain (client cert signed by CA)
	opts := x509.VerifyOptions{
		Roots:     caPool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	if _, err := clientCert.Verify(opts); err != nil {
		result.Error = fmt.Sprintf("certificate chain verification failed: %v", err)
		return result, info, fmt.Errorf("certificate chain verification failed: %w", err)
	}
	result.ChainVerified = true

	return result, info, nil
}

// testTLSConnection tests the TLS connection to the gateway.
func testTLSConnection(ctx context.Context, cfg *config.CLIConfig) (*ConnectionResult, error) {
	result := &ConnectionResult{
		ServerAddress: cfg.ServerAddress,
	}

	// Create TLS config
	tlsConfig, err := client.LoadClientTLSConfig(&cfg.TLS)
	if err != nil {
		result.Error = fmt.Sprintf("failed to load TLS config: %v", err)
		return result, fmt.Errorf("failed to load TLS config: %w", err)
	}

	if tlsConfig == nil {
		result.Error = "TLS is not configured"
		return result, fmt.Errorf("TLS is not configured")
	}

	// Create gRPC connection with TLS
	creds := credentials.NewTLS(tlsConfig)

	// Use a timeout for the connection
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, cfg.ServerAddress,
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
	)
	if err != nil {
		result.Error = fmt.Sprintf("TLS handshake failed: %v", err)
		return result, categorizeConnectionError(err)
	}
	defer conn.Close()

	result.TLSHandshake = true
	result.ServerCertVerified = true
	result.ClientCertAccepted = true

	// Try a health check to verify the gateway is responding
	grpcClient := client.NewGRPCClient(cfg.ServerAddress, &client.ClientOptions{
		TLSConfig:      tlsConfig,
		ConnectTimeout: cfg.Timeout,
	})

	if err := grpcClient.Connect(ctx); err != nil {
		// Connection worked but client setup failed - still counts as TLS success
		result.GatewayResponding = false
		result.Error = fmt.Sprintf("gateway health check failed: %v", err)
		return result, nil // Not a failure - TLS worked
	}
	defer grpcClient.Close()

	healthCtx, healthCancel := context.WithTimeout(ctx, 5*time.Second)
	defer healthCancel()

	if err := grpcClient.HealthCheck(healthCtx); err != nil {
		result.GatewayResponding = false
		// This is a warning, not an error - TLS connection succeeded
		return result, nil
	}

	result.GatewayResponding = true
	return result, nil
}

// categorizeConnectionError provides a user-friendly error message based on the error type.
func categorizeConnectionError(err error) error {
	errStr := err.Error()

	if strings.Contains(errStr, "certificate signed by unknown authority") {
		return fmt.Errorf("certificate signed by unknown authority: the gateway's CA may be different from your configured CA")
	}
	if strings.Contains(errStr, "certificate has expired") {
		return fmt.Errorf("server certificate has expired")
	}
	if strings.Contains(errStr, "bad certificate") {
		return fmt.Errorf("gateway rejected client certificate: certificate may be invalid or not authorized")
	}
	if strings.Contains(errStr, "certificate required") {
		return fmt.Errorf("gateway requires client certificate but none was accepted")
	}
	if strings.Contains(errStr, "connection refused") {
		return fmt.Errorf("connection refused: gateway may not be running or address is incorrect")
	}
	if strings.Contains(errStr, "no such host") {
		return fmt.Errorf("host not found: check server address configuration")
	}
	if strings.Contains(errStr, "context deadline exceeded") {
		return fmt.Errorf("connection timed out: gateway may be unreachable")
	}

	return err
}

// printLocalCertVerifySuccess prints success messages for local cert verification.
func printLocalCertVerifySuccess(result LocalCertsResult, info *certInfo, verbose bool) {
	fmt.Println("  \033[32m✓\033[0m Client certificate valid")
	fmt.Println("  \033[32m✓\033[0m CA certificate valid")
	fmt.Println("  \033[32m✓\033[0m Certificate chain verified")

	if verbose && info != nil {
		fmt.Println()
		fmt.Println("  Certificate details:")
		fmt.Printf("    Subject:  %s\n", info.Subject)
		fmt.Printf("    Issuer:   %s\n", info.Issuer)
		fmt.Printf("    Valid:    %s to %s\n",
			info.NotBefore.Format("2006-01-02"),
			info.NotAfter.Format("2006-01-02"))

		// Show time until expiration
		daysUntilExpiry := int(time.Until(info.NotAfter).Hours() / 24)
		if daysUntilExpiry < 30 {
			fmt.Printf("    Expires:  \033[33m%d days\033[0m\n", daysUntilExpiry)
		} else {
			fmt.Printf("    Expires:  %d days\n", daysUntilExpiry)
		}

		fmt.Println()
		fmt.Println("  Certificate paths:")
		fmt.Printf("    CA:     %s\n", result.CACertPath)
		fmt.Printf("    Cert:   %s\n", result.ClientCertPath)
		fmt.Printf("    Key:    %s\n", result.ClientKeyPath)
	}
}

// printLocalCertVerifyError prints error messages for local cert verification.
func printLocalCertVerifyError(err error) {
	fmt.Printf("  \033[31m✗\033[0m %v\n", err)
}

// printCertConnectionSuccess prints success messages for connection test.
func printCertConnectionSuccess(result *ConnectionResult) {
	fmt.Println("  \033[32m✓\033[0m TLS handshake successful")
	fmt.Println("  \033[32m✓\033[0m Server certificate verified")
	fmt.Println("  \033[32m✓\033[0m Client certificate accepted")

	if result.GatewayResponding {
		fmt.Println("  \033[32m✓\033[0m Gateway responding")
	} else {
		fmt.Println("  \033[33m⚠\033[0m Connection works but health check failed")
	}
}

// printCertConnectionError prints error messages for connection test.
func printCertConnectionError(result *ConnectionResult, err error) {
	if result.TLSHandshake {
		fmt.Println("  \033[32m✓\033[0m TLS handshake successful")
	} else {
		fmt.Printf("  \033[31m✗\033[0m TLS handshake failed: %v\n", err)
	}
}

// printCertTroubleshootingHints prints actionable hints based on the error type.
func printCertTroubleshootingHints(phase string, err error) {
	errStr := err.Error()

	fmt.Println("\nPossible causes:")

	switch phase {
	case "local":
		if strings.Contains(errStr, "not found") {
			fmt.Println("  - Certificate file does not exist at the configured path")
			fmt.Println("  - Check tls.cert_dir or individual cert paths in config")
			fmt.Println("  - Run 'penf config show' to see current TLS configuration")
		} else if strings.Contains(errStr, "not enabled") {
			fmt.Println("  - TLS is not enabled in configuration")
			fmt.Println("  - Set 'tls.enabled: true' in ~/.penf/config.yaml")
		} else if strings.Contains(errStr, "expired") {
			fmt.Println("  - Client certificate has expired")
			fmt.Println("  - Request new certificates from your administrator")
		} else if strings.Contains(errStr, "not yet valid") {
			fmt.Println("  - Client certificate is not yet valid (check system clock)")
			fmt.Println("  - Certificate may have been generated with incorrect date")
		} else if strings.Contains(errStr, "chain verification failed") {
			fmt.Println("  - Client certificate not signed by the configured CA")
			fmt.Println("  - Wrong CA certificate configured")
			fmt.Println("  - Client certificate from a different CA")
		} else if strings.Contains(errStr, "PEM") {
			fmt.Println("  - Certificate file is not in valid PEM format")
			fmt.Println("  - File may be corrupted or in wrong format (DER)")
		} else {
			fmt.Println("  - Certificate or key file is invalid or corrupted")
			fmt.Println("  - Check file permissions")
		}

	case "connection":
		if strings.Contains(errStr, "unknown authority") {
			fmt.Println("  - Gateway using different CA than configured locally")
			fmt.Println("  - Wrong CA certificate configured")
			fmt.Println("  - Gateway not configured for mTLS")
		} else if strings.Contains(errStr, "bad certificate") || strings.Contains(errStr, "rejected") {
			fmt.Println("  - Gateway does not trust your client certificate")
			fmt.Println("  - Client certificate not authorized for this gateway")
			fmt.Println("  - Certificate may have been revoked")
		} else if strings.Contains(errStr, "connection refused") {
			fmt.Println("  - Gateway is not running")
			fmt.Println("  - Wrong server address configured")
			fmt.Println("  - Firewall blocking connection")
		} else if strings.Contains(errStr, "timed out") {
			fmt.Println("  - Gateway is unreachable")
			fmt.Println("  - Network connectivity issue")
			fmt.Println("  - Firewall blocking connection")
		} else {
			fmt.Println("  - Network connectivity issue")
			fmt.Println("  - Gateway configuration mismatch")
		}
	}

	fmt.Println("\nDebug: penf cert verify -v")
}
