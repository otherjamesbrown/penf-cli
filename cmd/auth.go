// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/otherjamesbrown/penf-cli/credentials"
)

// Auth command flags.
var (
	authAPIKey         string
	authToken          string
	authServer         string
	authNonInteractive bool
)

// AuthCmd represents the auth command group.
var AuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long: `Manage authentication credentials for the Penfold API.

The auth commands allow you to login, logout, check status, and refresh tokens.
Credentials are stored securely in ~/.penf/credentials.yaml with encryption.

Authentication methods:
  - API Key: Long-lived key for programmatic access (--api-key flag or PENF_API_KEY env)
  - JWT Token: Short-lived token for user sessions (--token flag or PENF_TOKEN env)

Environment variables take precedence over stored credentials.`,
}

// loginCmd handles authentication login.
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to the Penfold API",
	Long: `Login to the Penfold API using an API key or interactive authentication.

Examples:
  # Interactive login (prompts for API key)
  penf auth login

  # Login with API key flag
  penf auth login --api-key pf-abc123...

  # Login with API key from environment
  PENF_API_KEY=pf-abc123... penf auth login

  # Login with token
  penf auth login --token eyJhbGciOiJIUzI1NiIs...

Notes:
  - API keys are long-lived and suitable for automation
  - Tokens expire and may need to be refreshed
  - Credentials are stored encrypted at rest`,
	RunE: runLogin,
}

// logoutCmd handles authentication logout.
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout and clear stored credentials",
	Long: `Logout from the Penfold API and clear stored credentials.

This removes all stored authentication data from the local credential store.
Environment variables (PENF_API_KEY, PENF_TOKEN) are not affected.

Examples:
  penf auth logout`,
	RunE: runLogout,
}

// statusCmd shows authentication status.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	Long: `Display the current authentication status and credential information.

Shows:
  - Authentication type (API key or token)
  - Credential source (stored, environment, or none)
  - Token expiration status (if applicable)
  - Masked credential values

Examples:
  penf auth status`,
	RunE: runStatus,
}

// refreshCmd forces token refresh.
var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Force token refresh",
	Long: `Force a refresh of the stored authentication token.

This is only applicable for JWT token authentication.
API keys do not expire and do not need refreshing.

Examples:
  penf auth refresh`,
	RunE: runRefresh,
}

func init() {
	// Login flags
	loginCmd.Flags().StringVar(&authAPIKey, "api-key", "", "API key for authentication")
	loginCmd.Flags().StringVar(&authToken, "token", "", "JWT token for authentication")
	loginCmd.Flags().StringVar(&authServer, "server", "", "Server address to associate with credentials")
	loginCmd.Flags().BoolVar(&authNonInteractive, "non-interactive", false, "Fail instead of prompting for input")

	// Add subcommands
	AuthCmd.AddCommand(loginCmd)
	AuthCmd.AddCommand(logoutCmd)
	AuthCmd.AddCommand(statusCmd)
	AuthCmd.AddCommand(refreshCmd)
}

// runLogin handles the login command.
func runLogin(cmd *cobra.Command, args []string) error {
	store, err := credentials.NewStore()
	if err != nil {
		return fmt.Errorf("initializing credential store: %w", err)
	}

	// Determine credential source and type
	var creds *credentials.Credentials

	// Check flags first
	if authAPIKey != "" {
		creds = &credentials.Credentials{
			AuthType:      credentials.AuthTypeAPIKey,
			APIKey:        authAPIKey,
			ServerAddress: authServer,
		}
	} else if authToken != "" {
		creds = &credentials.Credentials{
			AuthType:      credentials.AuthTypeToken,
			Token:         authToken,
			ServerAddress: authServer,
		}
	}

	// Check environment variables if no flags provided
	if creds == nil {
		if envKey := os.Getenv("PENF_API_KEY"); envKey != "" {
			creds = &credentials.Credentials{
				AuthType:      credentials.AuthTypeAPIKey,
				APIKey:        envKey,
				ServerAddress: authServer,
			}
			fmt.Println("Using API key from PENF_API_KEY environment variable")
		} else if envToken := os.Getenv("PENF_TOKEN"); envToken != "" {
			creds = &credentials.Credentials{
				AuthType:      credentials.AuthTypeToken,
				Token:         envToken,
				ServerAddress: authServer,
			}
			fmt.Println("Using token from PENF_TOKEN environment variable")
		}
	}

	// Interactive prompt if no credentials provided
	if creds == nil {
		if authNonInteractive {
			return fmt.Errorf("no credentials provided and --non-interactive flag set")
		}

		promptedCreds, err := promptForCredentials()
		if err != nil {
			return fmt.Errorf("reading credentials: %w", err)
		}
		creds = promptedCreds
		creds.ServerAddress = authServer
	}

	// Validate the credential format
	if err := validateCredential(creds); err != nil {
		return fmt.Errorf("invalid credentials: %w", err)
	}

	// Save credentials
	if err := store.Save(creds); err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	// Display success message
	fmt.Println("Login successful!")
	fmt.Printf("  Authentication type: %s\n", creds.AuthType)
	if creds.AuthType == credentials.AuthTypeAPIKey {
		fmt.Printf("  API Key: %s\n", credentials.MaskAPIKey(creds.APIKey))
	} else {
		fmt.Printf("  Token: %s\n", credentials.MaskToken(creds.Token))
	}
	if creds.ServerAddress != "" {
		fmt.Printf("  Server: %s\n", creds.ServerAddress)
	}

	credPath, _ := credentials.CredentialsPath()
	fmt.Printf("\nCredentials stored in: %s\n", credPath)

	return nil
}

// promptForCredentials prompts the user for credentials interactively.
func promptForCredentials() (*credentials.Credentials, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Enter your Penfold API credentials.")
	fmt.Println("You can use either an API key or a JWT token.")
	fmt.Println()
	fmt.Print("API Key (press Enter to use token instead): ")

	// Read API key (hidden input)
	apiKeyBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // Add newline after hidden input
	if err != nil {
		// Fallback to regular input if terminal not available
		apiKey, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading API key: %w", err)
		}
		apiKey = strings.TrimSpace(apiKey)
		if apiKey != "" {
			return &credentials.Credentials{
				AuthType: credentials.AuthTypeAPIKey,
				APIKey:   apiKey,
			}, nil
		}
	} else {
		apiKey := strings.TrimSpace(string(apiKeyBytes))
		if apiKey != "" {
			return &credentials.Credentials{
				AuthType: credentials.AuthTypeAPIKey,
				APIKey:   apiKey,
			}, nil
		}
	}

	// Prompt for token if no API key provided
	fmt.Print("JWT Token: ")
	tokenBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		token, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading token: %w", err)
		}
		token = strings.TrimSpace(token)
		if token == "" {
			return nil, fmt.Errorf("no credentials provided")
		}
		return &credentials.Credentials{
			AuthType: credentials.AuthTypeToken,
			Token:    token,
		}, nil
	}

	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return nil, fmt.Errorf("no credentials provided")
	}

	return &credentials.Credentials{
		AuthType: credentials.AuthTypeToken,
		Token:    token,
	}, nil
}

// validateCredential performs basic validation on credentials.
func validateCredential(creds *credentials.Credentials) error {
	switch creds.AuthType {
	case credentials.AuthTypeAPIKey:
		if creds.APIKey == "" {
			return fmt.Errorf("API key is empty")
		}
		if len(creds.APIKey) < 8 {
			return fmt.Errorf("API key is too short")
		}
	case credentials.AuthTypeToken:
		if creds.Token == "" {
			return fmt.Errorf("token is empty")
		}
		// Basic JWT format check (three base64 parts separated by dots)
		parts := strings.Split(creds.Token, ".")
		if len(parts) != 3 {
			return fmt.Errorf("invalid JWT token format")
		}
	default:
		return fmt.Errorf("unknown authentication type: %s", creds.AuthType)
	}
	return nil
}

// runLogout handles the logout command.
func runLogout(cmd *cobra.Command, args []string) error {
	store, err := credentials.NewStore()
	if err != nil {
		return fmt.Errorf("initializing credential store: %w", err)
	}

	if !store.Exists() {
		fmt.Println("No stored credentials found.")
		return nil
	}

	if err := store.Delete(); err != nil {
		return fmt.Errorf("removing credentials: %w", err)
	}

	fmt.Println("Logged out successfully.")
	fmt.Println("Stored credentials have been removed.")

	// Warn about environment variables
	if os.Getenv("PENF_API_KEY") != "" {
		fmt.Println("\nNote: PENF_API_KEY environment variable is still set.")
		fmt.Println("Unset it with: unset PENF_API_KEY")
	}
	if os.Getenv("PENF_TOKEN") != "" {
		fmt.Println("\nNote: PENF_TOKEN environment variable is still set.")
		fmt.Println("Unset it with: unset PENF_TOKEN")
	}

	return nil
}

// runStatus handles the status command.
func runStatus(cmd *cobra.Command, args []string) error {
	store, err := credentials.NewStore()
	if err != nil {
		return fmt.Errorf("initializing credential store: %w", err)
	}

	fmt.Println("Authentication Status")
	fmt.Println("=====================")
	fmt.Println()

	// Check environment variables
	envAPIKey := os.Getenv("PENF_API_KEY")
	envToken := os.Getenv("PENF_TOKEN")

	hasEnvCreds := envAPIKey != "" || envToken != ""

	if hasEnvCreds {
		fmt.Println("Environment Variables:")
		if envAPIKey != "" {
			fmt.Printf("  PENF_API_KEY: %s (active)\n", credentials.MaskAPIKey(envAPIKey))
		} else {
			fmt.Println("  PENF_API_KEY: (not set)")
		}
		if envToken != "" {
			fmt.Printf("  PENF_TOKEN: %s (active)\n", credentials.MaskToken(envToken))
		} else {
			fmt.Println("  PENF_TOKEN: (not set)")
		}
		fmt.Println()
	}

	// Check stored credentials
	creds, err := store.Load()
	if err != nil {
		if err == credentials.ErrNoCredentials {
			fmt.Println("Stored Credentials: None")
			if !hasEnvCreds {
				fmt.Println("\nNot authenticated. Run 'penf auth login' to authenticate.")
			}
			return nil
		}
		return fmt.Errorf("loading credentials: %w", err)
	}

	fmt.Println("Stored Credentials:")
	fmt.Printf("  Type: %s\n", creds.AuthType)

	switch creds.AuthType {
	case credentials.AuthTypeAPIKey:
		fmt.Printf("  API Key: %s\n", credentials.MaskAPIKey(creds.APIKey))
		fmt.Printf("  Key ID: %s\n", credentials.GenerateAPIKeyID(creds.APIKey))
	case credentials.AuthTypeToken:
		fmt.Printf("  Token: %s\n", credentials.MaskToken(creds.Token))
		if !creds.ExpiresAt.IsZero() {
			fmt.Printf("  Expires: %s (%s)\n",
				creds.ExpiresAt.Format(time.RFC3339),
				credentials.FormatExpiry(creds.ExpiresAt))
		}
		if creds.RefreshToken != "" {
			fmt.Println("  Refresh Token: (present)")
		}
	}

	if creds.Subject != "" {
		fmt.Printf("  Subject: %s\n", creds.Subject)
	}
	if creds.ServerAddress != "" {
		fmt.Printf("  Server: %s\n", creds.ServerAddress)
	}
	fmt.Printf("  Last Updated: %s\n", creds.LastUpdated.Format(time.RFC3339))

	// Active credential source
	fmt.Println()
	if hasEnvCreds {
		fmt.Println("Active Credential Source: Environment variable")
		if envAPIKey != "" {
			fmt.Println("  (PENF_API_KEY takes precedence)")
		} else {
			fmt.Println("  (PENF_TOKEN takes precedence)")
		}
	} else {
		fmt.Println("Active Credential Source: Stored credentials")
	}

	// Check for expiration
	if creds.AuthType == credentials.AuthTypeToken && !creds.ExpiresAt.IsZero() {
		if time.Now().After(creds.ExpiresAt) {
			fmt.Println("\nWarning: Stored token has expired. Run 'penf auth refresh' or 'penf auth login'.")
		} else if time.Until(creds.ExpiresAt) < time.Hour {
			fmt.Println("\nWarning: Token expires soon. Consider running 'penf auth refresh'.")
		}
	}

	return nil
}

// runRefresh handles the refresh command.
func runRefresh(cmd *cobra.Command, args []string) error {
	store, err := credentials.NewStore()
	if err != nil {
		return fmt.Errorf("initializing credential store: %w", err)
	}

	creds, err := store.Load()
	if err != nil {
		if err == credentials.ErrNoCredentials {
			return fmt.Errorf("no stored credentials to refresh - run 'penf auth login' first")
		}
		return fmt.Errorf("loading credentials: %w", err)
	}

	if creds.AuthType != credentials.AuthTypeToken {
		fmt.Println("API keys do not expire and do not need refreshing.")
		return nil
	}

	if creds.RefreshToken == "" {
		return fmt.Errorf("no refresh token available - run 'penf auth login' to obtain new credentials")
	}

	// STUB: Token refresh requires API Gateway connection.
	// When implemented, this will exchange the refresh token for a new access token.

	fmt.Println("Token refresh functionality requires connection to the Penfold API.")
	fmt.Println()
	fmt.Println("Current implementation status:")
	fmt.Println("  - Token refresh will be available once the API Gateway is connected.")
	fmt.Println("  - For now, please run 'penf auth login' to obtain new credentials.")
	fmt.Println()

	return nil
}

// GetAuthCredentials returns the active credentials for use in API calls.
// This is intended to be called from the client package for authentication.
func GetAuthCredentials(ctx context.Context) (*credentials.Credentials, error) {
	store, err := credentials.NewStore()
	if err != nil {
		return nil, fmt.Errorf("initializing credential store: %w", err)
	}

	return store.GetActiveCredential()
}
