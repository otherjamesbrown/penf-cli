// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/penf-cli/config"
)

var (
	certInitFromDir        string
	certInitCADir          string
	certInitName           string
	certInitForce          bool
	certInitNonInteractive bool
)

// NewCertInitCommand creates the cert init command.
func NewCertInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize client certificates for TLS authentication",
		Long: `Sets up client certificates for mTLS authentication with the gateway.

This command can:
1. Generate a new client certificate signed by the penfold CA
2. Copy existing certificates to the config directory

Certificates are stored in ~/.config/penf/certs/

Examples:
  # Interactive: prompts for CA location
  penf cert init

  # Non-interactive: specify paths
  penf cert init --ca-dir ~/secrets/penfold-ca --name dev-macbook

  # Just copy existing certs
  penf cert init --from /path/to/certs

After initialization, the CLI config is updated to use the certificates.`,
		RunE: runCertInit,
	}

	cmd.Flags().StringVar(&certInitFromDir, "from", "", "Copy existing certs from directory")
	cmd.Flags().StringVar(&certInitCADir, "ca-dir", "", "Directory containing CA cert and key (for generation)")
	cmd.Flags().StringVar(&certInitName, "name", "", "Client name for certificate CN (default: hostname)")
	cmd.Flags().BoolVar(&certInitForce, "force", false, "Overwrite existing certificates")
	cmd.Flags().BoolVar(&certInitNonInteractive, "non-interactive", false, "Fail instead of prompting for input")

	return cmd
}

// getDefaultCertDir returns the default certificate directory path.
func getDefaultCertDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, config.DefaultCertDir), nil
}

// certsExist checks if certificates already exist in the given directory.
func certsExist(certDir string) bool {
	files := []string{"ca.crt", "client.crt", "client.key"}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(certDir, f)); err == nil {
			return true
		}
	}
	return false
}

// getDefaultHostname returns the machine hostname for use as client name.
func getDefaultHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "client"
	}
	// Remove domain suffix if present
	if idx := strings.Index(hostname, "."); idx > 0 {
		hostname = hostname[:idx]
	}
	return hostname
}

func runCertInit(cmd *cobra.Command, args []string) error {
	fmt.Println("Certificate Initialization")
	fmt.Println("===========================")
	fmt.Println()

	// Get certificate directory
	certDir, err := getDefaultCertDir()
	if err != nil {
		return err
	}

	// Check if certs already exist
	if certsExist(certDir) && !certInitForce {
		fmt.Printf("Certificates already exist in %s\n", certDir)
		fmt.Println()
		fmt.Println("Existing files:")
		listCertFiles(certDir)
		fmt.Println()
		fmt.Println("Use --force to overwrite existing certificates.")
		return nil
	}

	// Determine operation mode
	if certInitFromDir != "" {
		// Copy from existing location
		return copyCertsFrom(certInitFromDir, certDir)
	}

	// Generate new certificates
	if certInitCADir == "" && !certInitNonInteractive {
		// Interactive prompt
		caDir, err := promptForCADir()
		if err != nil {
			return err
		}
		certInitCADir = caDir
	}

	if certInitCADir == "" {
		return fmt.Errorf("CA directory required (use --ca-dir or run interactively)")
	}

	// Set client name
	clientName := certInitName
	if clientName == "" {
		clientName = getDefaultHostname()
	}

	// Generate certificate
	if err := generateClientCert(certInitCADir, certDir, clientName); err != nil {
		return err
	}

	// Update config
	if err := updateConfigWithTLS(certDir); err != nil {
		fmt.Printf("\n  \033[33mWarning:\033[0m Could not update config: %v\n", err)
		fmt.Println("  You may need to manually add TLS settings to ~/.penf/config.yaml")
	}

	return nil
}

// listCertFiles lists certificate files in a directory.
func listCertFiles(dir string) {
	files := []string{"ca.crt", "client.crt", "client.key"}
	for _, f := range files {
		path := filepath.Join(dir, f)
		if info, err := os.Stat(path); err == nil {
			fmt.Printf("  %s (%d bytes)\n", f, info.Size())
		}
	}
}

// promptForCADir prompts the user for the CA directory.
func promptForCADir() (string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("To generate a client certificate, specify the CA directory.")
	fmt.Println("The CA directory should contain ca.crt and ca.key files.")
	fmt.Println()
	fmt.Print("CA directory path: ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading input: %w", err)
	}

	caDir := strings.TrimSpace(input)
	if caDir == "" {
		return "", fmt.Errorf("CA directory is required")
	}

	// Expand ~ in path
	caDir, err = config.ExpandPath(caDir)
	if err != nil {
		return "", fmt.Errorf("expanding path: %w", err)
	}

	// Validate CA directory
	if err := validateCADir(caDir); err != nil {
		return "", err
	}

	return caDir, nil
}

// validateCADir checks that the CA directory contains required files.
func validateCADir(caDir string) error {
	// Check directory exists
	info, err := os.Stat(caDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("CA directory not found: %s", caDir)
	}
	if err != nil {
		return fmt.Errorf("accessing CA directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", caDir)
	}

	// Check for required files
	requiredFiles := []string{"ca.crt", "ca.key"}
	for _, f := range requiredFiles {
		path := filepath.Join(caDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("required file not found: %s", path)
		}
	}

	return nil
}

// copyCertsFrom copies certificates from a source directory.
func copyCertsFrom(srcDir, destDir string) error {
	// Expand ~ in source path
	srcDir, err := config.ExpandPath(srcDir)
	if err != nil {
		return fmt.Errorf("expanding source path: %w", err)
	}

	fmt.Printf("Copying certificates from %s\n", srcDir)
	fmt.Println()

	// Validate source directory
	files := []struct {
		name     string
		required bool
	}{
		{"ca.crt", true},
		{"client.crt", true},
		{"client.key", true},
	}

	for _, f := range files {
		srcPath := filepath.Join(srcDir, f.name)
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			if f.required {
				return fmt.Errorf("required file not found: %s", srcPath)
			}
		}
	}

	// Create destination directory
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return fmt.Errorf("creating cert directory: %w", err)
	}

	// Copy files
	for _, f := range files {
		srcPath := filepath.Join(srcDir, f.name)
		destPath := filepath.Join(destDir, f.name)

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue // Skip non-existent optional files
		}

		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("copying %s: %w", f.name, err)
		}

		// Set appropriate permissions
		mode := os.FileMode(0644)
		if f.name == "client.key" {
			mode = 0600
		}
		if err := os.Chmod(destPath, mode); err != nil {
			return fmt.Errorf("setting permissions on %s: %w", f.name, err)
		}

		fmt.Printf("  \033[32m✓\033[0m Copied %s\n", f.name)
	}

	fmt.Println()
	fmt.Printf("Certificates installed to: %s\n", destDir)
	fmt.Println()

	// Update config
	if err := updateConfigWithTLS(destDir); err != nil {
		fmt.Printf("  \033[33mWarning:\033[0m Could not update config: %v\n", err)
		fmt.Println("  You may need to manually add TLS settings to ~/.penf/config.yaml")
	}

	// Validate certificates
	if err := validateInstalledCerts(destDir); err != nil {
		fmt.Printf("\n  \033[33mWarning:\033[0m Certificate validation failed: %v\n", err)
	} else {
		fmt.Println("\n  \033[32m✓\033[0m Certificates validated successfully")
	}

	return nil
}

// copyFile copies a file from src to dest.
func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return err
	}

	return destFile.Sync()
}

// generateClientCert generates a new client certificate using the CA.
func generateClientCert(caDir, certDir, clientName string) error {
	// Expand paths
	caDir, err := config.ExpandPath(caDir)
	if err != nil {
		return fmt.Errorf("expanding CA path: %w", err)
	}

	// Validate CA directory
	if err := validateCADir(caDir); err != nil {
		return err
	}

	fmt.Printf("Generating client certificate for '%s'\n", clientName)
	fmt.Printf("  CA directory: %s\n", caDir)
	fmt.Printf("  Output directory: %s\n", certDir)
	fmt.Println()

	// Create destination directory
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return fmt.Errorf("creating cert directory: %w", err)
	}

	// Look for the create-client-cert.sh script
	scriptPath := findCreateClientCertScript()
	if scriptPath != "" {
		fmt.Println("Using create-client-cert.sh script...")
		return runCreateClientCertScript(scriptPath, clientName, caDir, certDir)
	}

	// Fall back to Go implementation
	fmt.Println("Generating certificate with Go implementation...")
	return generateClientCertGo(caDir, certDir, clientName)
}

// findCreateClientCertScript looks for the create-client-cert.sh script.
func findCreateClientCertScript() string {
	// Look in common locations
	locations := []string{
		// Relative to penf binary
		"scripts/certs/create-client-cert.sh",
		// Development locations
		"../../scripts/certs/create-client-cert.sh",
		"../../../scripts/certs/create-client-cert.sh",
	}

	// Get executable path
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		locations = append(locations,
			filepath.Join(execDir, "scripts/certs/create-client-cert.sh"),
			filepath.Join(execDir, "../scripts/certs/create-client-cert.sh"),
		)
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}

// runCreateClientCertScript runs the shell script to create a client cert.
func runCreateClientCertScript(scriptPath, clientName, caDir, certDir string) error {
	args := []string{clientName, caDir, certDir}
	if certInitForce {
		args = append([]string{"--force"}, args...)
	}

	cmd := exec.Command(scriptPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running create-client-cert.sh: %w", err)
	}

	return nil
}

// generateClientCertGo generates a client certificate using Go crypto.
// This is a fallback when the shell script is not available.
func generateClientCertGo(caDir, certDir, clientName string) error {
	// For now, we require the shell script or openssl
	// A full Go implementation would use crypto/x509 and crypto/rsa

	// Check if openssl is available
	if _, err := exec.LookPath("openssl"); err != nil {
		return fmt.Errorf("openssl not found in PATH - please install openssl or use the create-client-cert.sh script")
	}

	// Use openssl directly as a fallback
	fmt.Println("Using openssl to generate certificate...")

	caKey := filepath.Join(caDir, "ca.key")
	caCrt := filepath.Join(caDir, "ca.crt")
	clientKey := filepath.Join(certDir, "client.key")
	clientCsr := filepath.Join(certDir, "client.csr")
	clientCrt := filepath.Join(certDir, "client.crt")
	clientExt := filepath.Join(certDir, "client.ext")
	outputCaCrt := filepath.Join(certDir, "ca.crt")

	subject := fmt.Sprintf("/C=GB/ST=London/L=London/O=Penfold/OU=Client/CN=%s", clientName)

	// Generate client private key
	fmt.Println("  Generating client private key...")
	genKeyCmd := exec.Command("openssl", "genrsa", "-out", clientKey, "2048")
	if err := genKeyCmd.Run(); err != nil {
		return fmt.Errorf("generating client key: %w", err)
	}
	os.Chmod(clientKey, 0600)

	// Generate CSR
	fmt.Println("  Generating certificate signing request...")
	csrCmd := exec.Command("openssl", "req", "-new",
		"-key", clientKey,
		"-out", clientCsr,
		"-subj", subject)
	if err := csrCmd.Run(); err != nil {
		return fmt.Errorf("generating CSR: %w", err)
	}

	// Write extensions file
	extContent := `authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
`
	if err := os.WriteFile(clientExt, []byte(extContent), 0644); err != nil {
		return fmt.Errorf("writing extensions file: %w", err)
	}

	// Sign the certificate
	fmt.Println("  Signing certificate with CA...")
	signCmd := exec.Command("openssl", "x509", "-req",
		"-in", clientCsr,
		"-CA", caCrt,
		"-CAkey", caKey,
		"-CAcreateserial",
		"-out", clientCrt,
		"-days", "365",
		"-sha256",
		"-extfile", clientExt)
	if err := signCmd.Run(); err != nil {
		return fmt.Errorf("signing certificate: %w", err)
	}
	os.Chmod(clientCrt, 0644)

	// Clean up temporary files
	os.Remove(clientCsr)
	os.Remove(clientExt)

	// Copy CA certificate
	fmt.Println("  Copying CA certificate...")
	if err := copyFile(caCrt, outputCaCrt); err != nil {
		return fmt.Errorf("copying CA certificate: %w", err)
	}
	os.Chmod(outputCaCrt, 0644)

	fmt.Println()
	fmt.Printf("  \033[32m✓\033[0m Client certificate created successfully\n")
	fmt.Println()
	fmt.Printf("Certificates installed to: %s\n", certDir)
	fmt.Printf("  client.crt\n")
	fmt.Printf("  client.key\n")
	fmt.Printf("  ca.crt\n")
	fmt.Println()
	fmt.Printf("\033[33mNote:\033[0m Keep client.key secure - it authenticates as '%s'!\n", clientName)

	return nil
}

// validateInstalledCerts validates that installed certificates are valid.
func validateInstalledCerts(certDir string) error {
	// Check if openssl is available
	if _, err := exec.LookPath("openssl"); err != nil {
		// Skip validation if openssl not available
		return nil
	}

	clientCrt := filepath.Join(certDir, "client.crt")
	caCrt := filepath.Join(certDir, "ca.crt")

	// Verify client cert was signed by CA
	verifyCmd := exec.Command("openssl", "verify", "-CAfile", caCrt, clientCrt)
	output, err := verifyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certificate verification failed: %s", string(output))
	}

	return nil
}

// updateConfigWithTLS updates the CLI config with TLS settings.
func updateConfigWithTLS(certDir string) error {
	// Load existing config
	cfg, err := config.LoadConfig()
	if err != nil {
		// Start with defaults if no config exists
		cfg = config.DefaultConfig()
	}

	// Update TLS settings
	cfg.TLS.Enabled = true
	cfg.TLS.CertDir = "~/.config/penf/certs"

	// Save config
	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println("  \033[32m✓\033[0m Updated ~/.penf/config.yaml with TLS settings")
	return nil
}
