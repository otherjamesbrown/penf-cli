// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"github.com/spf13/cobra"
)

// NewCertCommand creates the cert command and its subcommands.
func NewCertCommand() *cobra.Command {
	certCmd := &cobra.Command{
		Use:   "cert",
		Short: "Manage client certificates for mTLS authentication",
		Long: `Manage client certificates for mTLS authentication with the Penfold gateway.

This command provides utilities for viewing and managing client certificates
used for mutual TLS (mTLS) authentication.

The penfold system uses mTLS to authenticate clients. Each client needs:
  - ca.crt     - CA certificate (for verifying the server)
  - client.crt - Client certificate (signed by the CA)
  - client.key - Client private key (keep secret!)

Certificates are stored in ~/.config/penf/certs/ by default.

Commands:
  init   - Initialize client certificates (generate or copy)
  show   - Display certificate information and validity
  verify - Test TLS connection to gateway

Examples:
  # Initialize certs by generating from CA
  penf cert init --ca-dir ~/secrets/penfold-ca --name dev-macbook

  # Copy existing certs
  penf cert init --from /path/to/certs

  # Show certificate information
  penf cert show

  # Show verbose certificate details
  penf cert show -v

  # Verify certs and test connection to gateway
  penf cert verify

  # Just verify local certs (no connection test)
  penf cert verify --local

  # JSON output for scripting
  penf cert verify -o json`,
	}

	// Add subcommands
	certCmd.AddCommand(NewCertInitCommand())
	certCmd.AddCommand(NewCertShowCommand())
	certCmd.AddCommand(NewCertVerifyCommand())

	return certCmd
}
