// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/config"
)

// CertInfo contains certificate information for display.
type CertInfo struct {
	Path        string    `json:"path" yaml:"path"`
	Subject     string    `json:"subject" yaml:"subject"`
	Issuer      string    `json:"issuer" yaml:"issuer"`
	ValidFrom   time.Time `json:"valid_from" yaml:"valid_from"`
	ValidTo     time.Time `json:"valid_to" yaml:"valid_to"`
	ExpiresIn   string    `json:"expires_in" yaml:"expires_in"`
	DaysUntil   int       `json:"days_until_expiry" yaml:"days_until_expiry"`
	Serial      string    `json:"serial,omitempty" yaml:"serial,omitempty"`
	KeyUsage    []string  `json:"key_usage,omitempty" yaml:"key_usage,omitempty"`
	ExtKeyUsage []string  `json:"ext_key_usage,omitempty" yaml:"ext_key_usage,omitempty"`
	IsExpired   bool      `json:"is_expired" yaml:"is_expired"`
	IsExpiring  bool      `json:"is_expiring" yaml:"is_expiring"` // Within 30 days
}

// CertShowOutput contains the full output of cert show.
type CertShowOutput struct {
	ClientCert    *CertInfo `json:"client_cert,omitempty" yaml:"client_cert,omitempty"`
	CACert        *CertInfo `json:"ca_cert,omitempty" yaml:"ca_cert,omitempty"`
	ChainValid    bool      `json:"chain_valid" yaml:"chain_valid"`
	ChainError    string    `json:"chain_error,omitempty" yaml:"chain_error,omitempty"`
	Status        string    `json:"status" yaml:"status"`
	StatusMessage string    `json:"status_message" yaml:"status_message"`
	TLSConfigured bool      `json:"tls_configured" yaml:"tls_configured"`
}

// cert show command flags.
var (
	certShowOutput  string
	certShowVerbose bool
)

// Warning threshold for certificate expiration.
const expiryWarningDays = 30

// NewCertShowCommand creates the 'cert show' subcommand.
func NewCertShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display certificate information",
		Long: `Display information about configured mTLS certificates.

Shows the subject, issuer, validity period, and days until expiration for both
the client certificate and the CA certificate. Also verifies the certificate chain.

Examples:
  # Show certificate information
  penf cert show

  # Show verbose output with full details
  penf cert show -v

  # JSON output for scripting
  penf cert show --output json

  # YAML output
  penf cert show --output yaml`,
		RunE: runCertShow,
	}

	cmd.Flags().StringVarP(&certShowOutput, "output", "o", "", "Output format: text, json, yaml")
	cmd.Flags().BoolVarP(&certShowVerbose, "verbose", "v", false, "Show verbose certificate details")

	return cmd
}

// runCertShow executes the cert show command.
func runCertShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		// Config not loadable, try default paths.
		cfg = &config.CLIConfig{}
	}

	output := CertShowOutput{
		TLSConfigured: cfg.TLS.Enabled,
	}

	// Resolve certificate paths.
	certDir := getCertDir(cfg)
	clientCertPath := filepath.Join(certDir, "client.crt")
	caCertPath := filepath.Join(certDir, "ca.crt")

	// Override with config if specified.
	if cfg.TLS.ClientCert != "" {
		clientCertPath = expandTildePath(cfg.TLS.ClientCert)
	}
	if cfg.TLS.CACert != "" {
		caCertPath = expandTildePath(cfg.TLS.CACert)
	}

	// Check if TLS is configured.
	if !cfg.TLS.Enabled {
		// Check if certs exist anyway.
		clientExists := fileExists(clientCertPath)
		caExists := fileExists(caCertPath)

		if !clientExists && !caExists {
			output.Status = "not_configured"
			output.StatusMessage = "TLS not configured. Run: penf cert init"
			return outputCertShow(output)
		}
	}

	// Load and parse client certificate.
	clientCert, err := loadCertificate(clientCertPath)
	if err != nil {
		output.Status = "error"
		output.StatusMessage = fmt.Sprintf("Failed to load client certificate: %v", err)
	} else {
		output.ClientCert = certToInfo(clientCertPath, clientCert)
	}

	// Load and parse CA certificate.
	caCert, err := loadCertificate(caCertPath)
	if err != nil {
		if output.Status == "" {
			output.Status = "error"
			output.StatusMessage = fmt.Sprintf("Failed to load CA certificate: %v", err)
		}
	} else {
		output.CACert = certToInfo(caCertPath, caCert)
	}

	// Verify certificate chain if both certs loaded.
	if output.ClientCert != nil && output.CACert != nil {
		if err := verifyCertChain(clientCert, caCert); err != nil {
			output.ChainValid = false
			output.ChainError = err.Error()
			output.Status = "invalid"
			output.StatusMessage = fmt.Sprintf("Certificate chain invalid: %v", err)
		} else {
			output.ChainValid = true
		}
	}

	// Determine final status.
	if output.Status == "" {
		if output.ClientCert != nil && output.ClientCert.IsExpired {
			output.Status = "expired"
			output.StatusMessage = "Client certificate has expired"
		} else if output.CACert != nil && output.CACert.IsExpired {
			output.Status = "expired"
			output.StatusMessage = "CA certificate has expired"
		} else if output.ClientCert != nil && output.ClientCert.IsExpiring {
			output.Status = "expiring_soon"
			output.StatusMessage = fmt.Sprintf("Client certificate expires in %d days", output.ClientCert.DaysUntil)
		} else if output.CACert != nil && output.CACert.IsExpiring {
			output.Status = "expiring_soon"
			output.StatusMessage = fmt.Sprintf("CA certificate expires in %d days", output.CACert.DaysUntil)
		} else if output.ChainValid {
			output.Status = "valid"
			output.StatusMessage = "Certificates valid and ready"
		} else {
			output.Status = "unknown"
			output.StatusMessage = "Unable to determine certificate status"
		}
	}

	return outputCertShow(output)
}

// getCertDir returns the certificate directory path.
func getCertDir(cfg *config.CLIConfig) string {
	if cfg.TLS.CertDir != "" {
		return expandTildePath(cfg.TLS.CertDir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "penf", "certs")
}

// expandTildePath expands ~ to the user's home directory.
func expandTildePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// loadCertificate loads and parses a PEM-encoded certificate file.
func loadCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}

	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("expected CERTIFICATE block, got %s", block.Type)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	return cert, nil
}

// certToInfo converts an x509 certificate to CertInfo.
func certToInfo(path string, cert *x509.Certificate) *CertInfo {
	now := time.Now()
	daysUntil := int(cert.NotAfter.Sub(now).Hours() / 24)

	info := &CertInfo{
		Path:       shortenPath(path),
		Subject:    formatDN(cert.Subject.String()),
		Issuer:     formatDN(cert.Issuer.String()),
		ValidFrom:  cert.NotBefore,
		ValidTo:    cert.NotAfter,
		ExpiresIn:  formatCertExpiry(cert.NotAfter.Sub(now)),
		DaysUntil:  daysUntil,
		IsExpired:  now.After(cert.NotAfter),
		IsExpiring: daysUntil <= expiryWarningDays && daysUntil > 0,
	}

	if certShowVerbose {
		info.Serial = cert.SerialNumber.String()
		info.KeyUsage = keyUsageToStrings(cert.KeyUsage)
		info.ExtKeyUsage = extKeyUsageToStrings(cert.ExtKeyUsage)
	}

	return info
}

// formatDN formats a distinguished name for display.
func formatDN(dn string) string {
	// The default format from x509 is comma-separated.
	// Keep it concise.
	return dn
}

// formatCertExpiry formats a duration in a human-readable way for certificate expiration.
func formatCertExpiry(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	days := int(d.Hours() / 24)
	if days >= 365 {
		years := days / 365
		if years == 1 {
			return "1 year"
		}
		return fmt.Sprintf("%d years", years)
	}
	if days >= 30 {
		months := days / 30
		if months == 1 {
			return "1 month"
		}
		return fmt.Sprintf("%d months", months)
	}
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// shortenPath replaces home directory with ~.
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// keyUsageToStrings converts KeyUsage to string slice.
func keyUsageToStrings(usage x509.KeyUsage) []string {
	var result []string
	if usage&x509.KeyUsageDigitalSignature != 0 {
		result = append(result, "Digital Signature")
	}
	if usage&x509.KeyUsageKeyEncipherment != 0 {
		result = append(result, "Key Encipherment")
	}
	if usage&x509.KeyUsageDataEncipherment != 0 {
		result = append(result, "Data Encipherment")
	}
	if usage&x509.KeyUsageKeyAgreement != 0 {
		result = append(result, "Key Agreement")
	}
	if usage&x509.KeyUsageCertSign != 0 {
		result = append(result, "Certificate Sign")
	}
	if usage&x509.KeyUsageCRLSign != 0 {
		result = append(result, "CRL Sign")
	}
	if usage&x509.KeyUsageContentCommitment != 0 {
		result = append(result, "Content Commitment")
	}
	return result
}

// extKeyUsageToStrings converts ExtKeyUsage to string slice.
func extKeyUsageToStrings(usages []x509.ExtKeyUsage) []string {
	var result []string
	for _, usage := range usages {
		switch usage {
		case x509.ExtKeyUsageClientAuth:
			result = append(result, "Client Authentication")
		case x509.ExtKeyUsageServerAuth:
			result = append(result, "Server Authentication")
		case x509.ExtKeyUsageCodeSigning:
			result = append(result, "Code Signing")
		case x509.ExtKeyUsageEmailProtection:
			result = append(result, "Email Protection")
		case x509.ExtKeyUsageTimeStamping:
			result = append(result, "Time Stamping")
		case x509.ExtKeyUsageOCSPSigning:
			result = append(result, "OCSP Signing")
		default:
			result = append(result, fmt.Sprintf("Unknown (%d)", usage))
		}
	}
	return result
}

// verifyCertChain verifies that the client cert is signed by the CA.
func verifyCertChain(clientCert, caCert *x509.Certificate) error {
	// Create a certificate pool with the CA.
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	// Verify the client certificate against the CA.
	opts := x509.VerifyOptions{
		Roots:       caPool,
		CurrentTime: time.Now(),
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	_, err := clientCert.Verify(opts)
	if err != nil {
		return fmt.Errorf("certificate verification failed: %w", err)
	}

	return nil
}

// outputCertShow outputs the certificate information in the requested format.
func outputCertShow(output CertShowOutput) error {
	format := config.OutputFormatText
	if certShowOutput != "" {
		format = config.OutputFormat(certShowOutput)
	}

	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(output)
	default:
		return outputCertShowText(output)
	}
}

// outputCertShowText outputs certificate information in human-readable format.
func outputCertShowText(output CertShowOutput) error {
	if !output.TLSConfigured && output.ClientCert == nil && output.CACert == nil {
		fmt.Println("TLS not configured")
		fmt.Println("Run: penf cert init")
		return nil
	}

	// Client certificate.
	if output.ClientCert != nil {
		fmt.Println("Client Certificate")
		displayCertInfo(output.ClientCert)
		fmt.Println()
	}

	// CA certificate.
	if output.CACert != nil {
		fmt.Println("CA Certificate")
		displayCertInfo(output.CACert)
		fmt.Println()
	}

	// Chain verification status.
	if output.ChainError != "" {
		fmt.Printf("\033[33m! Warning: %s\033[0m\n", output.ChainError)
	}

	// Overall status.
	switch output.Status {
	case "valid":
		fmt.Printf("Status: \033[32m%s\033[0m %s\n", "OK", output.StatusMessage)
	case "expiring_soon":
		fmt.Printf("Status: \033[33m! WARNING\033[0m %s\n", output.StatusMessage)
	case "expired":
		fmt.Printf("Status: \033[31m! EXPIRED\033[0m %s\n", output.StatusMessage)
	case "invalid":
		fmt.Printf("Status: \033[31m! INVALID\033[0m %s\n", output.StatusMessage)
	case "error":
		fmt.Printf("Status: \033[31m! ERROR\033[0m %s\n", output.StatusMessage)
	default:
		fmt.Printf("Status: %s\n", output.StatusMessage)
	}

	return nil
}

// displayCertInfo displays a single certificate's information.
func displayCertInfo(info *CertInfo) {
	fmt.Printf("  Path:       %s\n", info.Path)
	fmt.Printf("  Subject:    %s\n", info.Subject)
	fmt.Printf("  Issuer:     %s\n", info.Issuer)
	fmt.Printf("  Valid:      %s to %s\n",
		info.ValidFrom.Format("2006-01-02"),
		info.ValidTo.Format("2006-01-02"))

	// Show expiration with color coding.
	if info.IsExpired {
		fmt.Printf("  Expires in: \033[31m%s (EXPIRED)\033[0m\n", info.ExpiresIn)
	} else if info.IsExpiring {
		fmt.Printf("  Expires in: \033[33m%s\033[0m\n", info.ExpiresIn)
	} else {
		fmt.Printf("  Expires in: %s\n", info.ExpiresIn)
	}

	// Verbose output.
	if certShowVerbose {
		if info.Serial != "" {
			fmt.Printf("  Serial:     %s\n", info.Serial)
		}
		if len(info.KeyUsage) > 0 {
			fmt.Printf("  Key Usage:  %s\n", strings.Join(info.KeyUsage, ", "))
		}
		if len(info.ExtKeyUsage) > 0 {
			fmt.Printf("  Ext Usage:  %s\n", strings.Join(info.ExtKeyUsage, ", "))
		}
	}
}
