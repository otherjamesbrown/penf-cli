package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/penf-cli/credentials"
)

// testEncryptionKey is a valid 32-byte (64 hex chars) encryption key for testing.
const testEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// setupTestEncryptionKey sets up the encryption key for tests and returns a cleanup function.
func setupTestEncryptionKey(t *testing.T) func() {
	t.Helper()
	originalKey := os.Getenv("PENF_ENCRYPTION_KEY")
	os.Setenv("PENF_ENCRYPTION_KEY", testEncryptionKey)
	return func() {
		if originalKey != "" {
			os.Setenv("PENF_ENCRYPTION_KEY", originalKey)
		} else {
			os.Unsetenv("PENF_ENCRYPTION_KEY")
		}
	}
}

func TestValidateCredential(t *testing.T) {
	tests := []struct {
		name    string
		creds   *credentials.Credentials
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid api key",
			creds: &credentials.Credentials{
				AuthType: credentials.AuthTypeAPIKey,
				APIKey:   "pf-test-api-key-12345",
			},
			wantErr: false,
		},
		{
			name: "empty api key",
			creds: &credentials.Credentials{
				AuthType: credentials.AuthTypeAPIKey,
				APIKey:   "",
			},
			wantErr: true,
			errMsg:  "API key is empty",
		},
		{
			name: "short api key",
			creds: &credentials.Credentials{
				AuthType: credentials.AuthTypeAPIKey,
				APIKey:   "short",
			},
			wantErr: true,
			errMsg:  "API key is too short",
		},
		{
			name: "valid jwt token",
			creds: &credentials.Credentials{
				AuthType: credentials.AuthTypeToken,
				Token:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0In0.signature",
			},
			wantErr: false,
		},
		{
			name: "empty token",
			creds: &credentials.Credentials{
				AuthType: credentials.AuthTypeToken,
				Token:    "",
			},
			wantErr: true,
			errMsg:  "token is empty",
		},
		{
			name: "invalid token format",
			creds: &credentials.Credentials{
				AuthType: credentials.AuthTypeToken,
				Token:    "not-a-jwt-token",
			},
			wantErr: true,
			errMsg:  "invalid JWT token format",
		},
		{
			name: "unknown auth type",
			creds: &credentials.Credentials{
				AuthType: "unknown",
			},
			wantErr: true,
			errMsg:  "unknown authentication type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCredential(tc.creds)
			if tc.wantErr {
				if err == nil {
					t.Errorf("validateCredential() expected error containing %q, got nil", tc.errMsg)
				} else if !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("validateCredential() error = %v, want error containing %q", err, tc.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateCredential() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestAuthCmd_Structure(t *testing.T) {
	// Verify AuthCmd has the expected subcommands
	subcommands := make(map[string]bool)
	for _, cmd := range AuthCmd.Commands() {
		subcommands[cmd.Name()] = true
	}

	expectedCommands := []string{"login", "logout", "status", "refresh"}
	for _, name := range expectedCommands {
		if !subcommands[name] {
			t.Errorf("AuthCmd missing subcommand: %s", name)
		}
	}
}

func TestAuthCmd_LoginFlags(t *testing.T) {
	// Find the login subcommand
	var loginCmd *cobra.Command
	for _, cmd := range AuthCmd.Commands() {
		if cmd.Name() == "login" {
			loginCmd = cmd
			break
		}
	}

	if loginCmd == nil {
		t.Fatal("login subcommand not found")
	}

	// Verify expected flags exist
	flags := []string{"api-key", "token", "server", "non-interactive"}
	for _, flagName := range flags {
		flag := loginCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("login command missing flag: %s", flagName)
		}
	}
}

func TestRunLogin_WithAPIKeyFlag(t *testing.T) {
	// Set up encryption key for tests
	cleanupKey := setupTestEncryptionKey(t)
	defer cleanupKey()

	// Create temp dir for credentials
	tempDir, err := os.MkdirTemp("", "penf-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set env for test
	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer os.Setenv("PENF_CONFIG_DIR", originalEnv)
	os.Setenv("PENF_CONFIG_DIR", tempDir)

	// Reset global flags
	authAPIKey = "pf-test-api-key-12345"
	authToken = ""
	authServer = "localhost:50051"
	authNonInteractive = true
	defer func() {
		authAPIKey = ""
		authToken = ""
		authServer = ""
		authNonInteractive = false
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run login
	cmd := &cobra.Command{}
	err = runLogin(cmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("runLogin() error = %v, output = %s", err, output)
	}

	// Verify success message
	if !strings.Contains(output, "Login successful") {
		t.Errorf("Expected 'Login successful' in output, got: %s", output)
	}

	// Verify credentials were saved
	store, err := credentials.NewStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	creds, err := store.Load()
	if err != nil {
		t.Fatalf("Failed to load credentials: %v", err)
	}

	if creds.AuthType != credentials.AuthTypeAPIKey {
		t.Errorf("AuthType = %v, want %v", creds.AuthType, credentials.AuthTypeAPIKey)
	}
	if creds.APIKey != "pf-test-api-key-12345" {
		t.Errorf("APIKey = %v, want pf-test-api-key-12345", creds.APIKey)
	}
}

func TestRunLogin_WithTokenFlag(t *testing.T) {
	cleanupKey := setupTestEncryptionKey(t)
	defer cleanupKey()

	tempDir, err := os.MkdirTemp("", "penf-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer os.Setenv("PENF_CONFIG_DIR", originalEnv)
	os.Setenv("PENF_CONFIG_DIR", tempDir)

	authAPIKey = ""
	authToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0In0.signature"
	authServer = ""
	authNonInteractive = true
	defer func() {
		authAPIKey = ""
		authToken = ""
		authServer = ""
		authNonInteractive = false
	}()

	cmd := &cobra.Command{}
	err = runLogin(cmd, []string{})

	if err != nil {
		t.Fatalf("runLogin() error = %v", err)
	}

	store, err := credentials.NewStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	creds, err := store.Load()
	if err != nil {
		t.Fatalf("Failed to load credentials: %v", err)
	}

	if creds.AuthType != credentials.AuthTypeToken {
		t.Errorf("AuthType = %v, want %v", creds.AuthType, credentials.AuthTypeToken)
	}
}

func TestRunLogin_WithEnvVar(t *testing.T) {
	cleanupKey := setupTestEncryptionKey(t)
	defer cleanupKey()

	tempDir, err := os.MkdirTemp("", "penf-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalConfigDir := os.Getenv("PENF_CONFIG_DIR")
	originalAPIKey := os.Getenv("PENF_API_KEY")
	defer func() {
		os.Setenv("PENF_CONFIG_DIR", originalConfigDir)
		os.Setenv("PENF_API_KEY", originalAPIKey)
	}()
	os.Setenv("PENF_CONFIG_DIR", tempDir)
	os.Setenv("PENF_API_KEY", "env-api-key-12345")

	// Clear flags
	authAPIKey = ""
	authToken = ""
	authServer = ""
	authNonInteractive = true
	defer func() {
		authAPIKey = ""
		authToken = ""
		authServer = ""
		authNonInteractive = false
	}()

	cmd := &cobra.Command{}
	err = runLogin(cmd, []string{})

	if err != nil {
		t.Fatalf("runLogin() error = %v", err)
	}

	store, err := credentials.NewStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	creds, err := store.Load()
	if err != nil {
		t.Fatalf("Failed to load credentials: %v", err)
	}

	if creds.APIKey != "env-api-key-12345" {
		t.Errorf("APIKey = %v, want env-api-key-12345", creds.APIKey)
	}
}

func TestRunLogin_NonInteractiveNoCredentials(t *testing.T) {
	cleanupKey := setupTestEncryptionKey(t)
	defer cleanupKey()

	tempDir, err := os.MkdirTemp("", "penf-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalConfigDir := os.Getenv("PENF_CONFIG_DIR")
	originalAPIKey := os.Getenv("PENF_API_KEY")
	originalToken := os.Getenv("PENF_TOKEN")
	defer func() {
		os.Setenv("PENF_CONFIG_DIR", originalConfigDir)
		os.Setenv("PENF_API_KEY", originalAPIKey)
		os.Setenv("PENF_TOKEN", originalToken)
	}()
	os.Setenv("PENF_CONFIG_DIR", tempDir)
	os.Unsetenv("PENF_API_KEY")
	os.Unsetenv("PENF_TOKEN")

	authAPIKey = ""
	authToken = ""
	authServer = ""
	authNonInteractive = true
	defer func() {
		authAPIKey = ""
		authToken = ""
		authServer = ""
		authNonInteractive = false
	}()

	cmd := &cobra.Command{}
	err = runLogin(cmd, []string{})

	if err == nil {
		t.Error("runLogin() expected error with --non-interactive and no credentials")
	}
	if !strings.Contains(err.Error(), "no credentials provided") {
		t.Errorf("runLogin() error = %v, expected 'no credentials provided'", err)
	}
}

func TestRunLogout(t *testing.T) {
	cleanupKey := setupTestEncryptionKey(t)
	defer cleanupKey()

	tempDir, err := os.MkdirTemp("", "penf-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer os.Setenv("PENF_CONFIG_DIR", originalEnv)
	os.Setenv("PENF_CONFIG_DIR", tempDir)

	// First, save some credentials
	store, err := credentials.NewStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	creds := &credentials.Credentials{
		AuthType: credentials.AuthTypeAPIKey,
		APIKey:   "test-key-12345",
	}
	if err := store.Save(creds); err != nil {
		t.Fatalf("Failed to save credentials: %v", err)
	}

	// Verify credentials exist
	if !store.Exists() {
		t.Fatal("Credentials should exist before logout")
	}

	// Run logout
	cmd := &cobra.Command{}
	err = runLogout(cmd, []string{})
	if err != nil {
		t.Fatalf("runLogout() error = %v", err)
	}

	// Verify credentials deleted
	if store.Exists() {
		t.Error("Credentials should not exist after logout")
	}
}

func TestRunLogout_NoCredentials(t *testing.T) {
	cleanupKey := setupTestEncryptionKey(t)
	defer cleanupKey()

	tempDir, err := os.MkdirTemp("", "penf-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer os.Setenv("PENF_CONFIG_DIR", originalEnv)
	os.Setenv("PENF_CONFIG_DIR", tempDir)

	// Run logout without any credentials
	cmd := &cobra.Command{}
	err = runLogout(cmd, []string{})

	// Should not error even if no credentials
	if err != nil {
		t.Errorf("runLogout() error = %v, expected no error", err)
	}
}

func TestRunStatus_WithStoredCredentials(t *testing.T) {
	cleanupKey := setupTestEncryptionKey(t)
	defer cleanupKey()

	tempDir, err := os.MkdirTemp("", "penf-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalConfigDir := os.Getenv("PENF_CONFIG_DIR")
	originalAPIKey := os.Getenv("PENF_API_KEY")
	originalToken := os.Getenv("PENF_TOKEN")
	defer func() {
		os.Setenv("PENF_CONFIG_DIR", originalConfigDir)
		os.Setenv("PENF_API_KEY", originalAPIKey)
		os.Setenv("PENF_TOKEN", originalToken)
	}()
	os.Setenv("PENF_CONFIG_DIR", tempDir)
	os.Unsetenv("PENF_API_KEY")
	os.Unsetenv("PENF_TOKEN")

	// Save some credentials
	store, err := credentials.NewStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	creds := &credentials.Credentials{
		AuthType:      credentials.AuthTypeAPIKey,
		APIKey:        "pf-test-key-12345",
		ServerAddress: "localhost:50051",
	}
	if err := store.Save(creds); err != nil {
		t.Fatalf("Failed to save credentials: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &cobra.Command{}
	err = runStatus(cmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("runStatus() error = %v", err)
	}

	// Verify output contains expected information
	expectedStrings := []string{
		"Authentication Status",
		"Stored Credentials",
		"Type: api_key",
	}
	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain %q, got: %s", expected, output)
		}
	}
}

func TestRunStatus_NoCredentials(t *testing.T) {
	cleanupKey := setupTestEncryptionKey(t)
	defer cleanupKey()

	tempDir, err := os.MkdirTemp("", "penf-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalConfigDir := os.Getenv("PENF_CONFIG_DIR")
	originalAPIKey := os.Getenv("PENF_API_KEY")
	originalToken := os.Getenv("PENF_TOKEN")
	defer func() {
		os.Setenv("PENF_CONFIG_DIR", originalConfigDir)
		os.Setenv("PENF_API_KEY", originalAPIKey)
		os.Setenv("PENF_TOKEN", originalToken)
	}()
	os.Setenv("PENF_CONFIG_DIR", tempDir)
	os.Unsetenv("PENF_API_KEY")
	os.Unsetenv("PENF_TOKEN")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &cobra.Command{}
	err = runStatus(cmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("runStatus() error = %v", err)
	}

	// Should indicate no credentials
	if !strings.Contains(output, "None") && !strings.Contains(output, "Not authenticated") {
		t.Errorf("Expected output to indicate no credentials, got: %s", output)
	}
}

func TestRunStatus_WithEnvVar(t *testing.T) {
	cleanupKey := setupTestEncryptionKey(t)
	defer cleanupKey()

	tempDir, err := os.MkdirTemp("", "penf-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalConfigDir := os.Getenv("PENF_CONFIG_DIR")
	originalAPIKey := os.Getenv("PENF_API_KEY")
	defer func() {
		os.Setenv("PENF_CONFIG_DIR", originalConfigDir)
		os.Setenv("PENF_API_KEY", originalAPIKey)
	}()
	os.Setenv("PENF_CONFIG_DIR", tempDir)
	os.Setenv("PENF_API_KEY", "env-api-key-value")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &cobra.Command{}
	err = runStatus(cmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("runStatus() error = %v", err)
	}

	// Should show env var info
	if !strings.Contains(output, "PENF_API_KEY") {
		t.Errorf("Expected output to mention PENF_API_KEY, got: %s", output)
	}
	if !strings.Contains(output, "(active)") {
		t.Errorf("Expected output to show env var as active, got: %s", output)
	}
}

func TestRunRefresh_NoCredentials(t *testing.T) {
	cleanupKey := setupTestEncryptionKey(t)
	defer cleanupKey()

	tempDir, err := os.MkdirTemp("", "penf-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer os.Setenv("PENF_CONFIG_DIR", originalEnv)
	os.Setenv("PENF_CONFIG_DIR", tempDir)

	cmd := &cobra.Command{}
	err = runRefresh(cmd, []string{})

	if err == nil {
		t.Error("runRefresh() expected error with no credentials")
	}
	if !strings.Contains(err.Error(), "no stored credentials") {
		t.Errorf("runRefresh() error = %v, expected 'no stored credentials'", err)
	}
}

func TestRunRefresh_WithAPIKey(t *testing.T) {
	cleanupKey := setupTestEncryptionKey(t)
	defer cleanupKey()

	tempDir, err := os.MkdirTemp("", "penf-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer os.Setenv("PENF_CONFIG_DIR", originalEnv)
	os.Setenv("PENF_CONFIG_DIR", tempDir)

	// Save API key credentials
	store, err := credentials.NewStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	creds := &credentials.Credentials{
		AuthType: credentials.AuthTypeAPIKey,
		APIKey:   "test-key-12345",
	}
	if err := store.Save(creds); err != nil {
		t.Fatalf("Failed to save credentials: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &cobra.Command{}
	err = runRefresh(cmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("runRefresh() error = %v", err)
	}

	// Should indicate API keys don't need refresh
	if !strings.Contains(output, "do not expire") && !strings.Contains(output, "do not need refreshing") {
		t.Errorf("Expected message about API keys not needing refresh, got: %s", output)
	}
}

func TestRunRefresh_WithTokenNoRefreshToken(t *testing.T) {
	cleanupKey := setupTestEncryptionKey(t)
	defer cleanupKey()

	tempDir, err := os.MkdirTemp("", "penf-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer os.Setenv("PENF_CONFIG_DIR", originalEnv)
	os.Setenv("PENF_CONFIG_DIR", tempDir)

	// Save token credentials without refresh token
	store, err := credentials.NewStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	creds := &credentials.Credentials{
		AuthType: credentials.AuthTypeToken,
		Token:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0In0.signature",
		// No RefreshToken
	}
	if err := store.Save(creds); err != nil {
		t.Fatalf("Failed to save credentials: %v", err)
	}

	cmd := &cobra.Command{}
	err = runRefresh(cmd, []string{})

	if err == nil {
		t.Error("runRefresh() expected error with no refresh token")
	}
	if !strings.Contains(err.Error(), "no refresh token") {
		t.Errorf("runRefresh() error = %v, expected 'no refresh token'", err)
	}
}
