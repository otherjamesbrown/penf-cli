package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// testEncryptionKey is a fixed 32-byte key for testing (hex-encoded to 64 chars)
const testEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// setupTestEnv sets up the test environment with a fixed encryption key
func setupTestEnv(t *testing.T, tempDir string) func() {
	t.Helper()

	originalConfigDir := os.Getenv("PENF_CONFIG_DIR")
	originalEncKey := os.Getenv("PENF_ENCRYPTION_KEY")

	os.Setenv("PENF_CONFIG_DIR", tempDir)
	os.Setenv("PENF_ENCRYPTION_KEY", testEncryptionKey)

	return func() {
		if originalConfigDir != "" {
			os.Setenv("PENF_CONFIG_DIR", originalConfigDir)
		} else {
			os.Unsetenv("PENF_CONFIG_DIR")
		}
		if originalEncKey != "" {
			os.Setenv("PENF_ENCRYPTION_KEY", originalEncKey)
		} else {
			os.Unsetenv("PENF_ENCRYPTION_KEY")
		}
	}
}

func TestCredentialsDir(t *testing.T) {
	// Test with default (no env var)
	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer os.Setenv("PENF_CONFIG_DIR", originalEnv)

	// Clear the env var to test default behavior
	os.Unsetenv("PENF_CONFIG_DIR")

	dir, err := CredentialsDir()
	if err != nil {
		t.Fatalf("CredentialsDir() error = %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, DefaultCredentialsDir)
	if dir != expected {
		t.Errorf("CredentialsDir() = %v, want %v", dir, expected)
	}

	// Test with env var set
	customDir := "/tmp/test-penf-creds"
	os.Setenv("PENF_CONFIG_DIR", customDir)

	dir, err = CredentialsDir()
	if err != nil {
		t.Fatalf("CredentialsDir() with env error = %v", err)
	}
	if dir != customDir {
		t.Errorf("CredentialsDir() with env = %v, want %v", dir, customDir)
	}
}

func TestCredentialsPath(t *testing.T) {
	originalEnv := os.Getenv("PENF_CONFIG_DIR")
	defer os.Setenv("PENF_CONFIG_DIR", originalEnv)

	customDir := "/tmp/test-penf-path"
	os.Setenv("PENF_CONFIG_DIR", customDir)

	path, err := CredentialsPath()
	if err != nil {
		t.Fatalf("CredentialsPath() error = %v", err)
	}

	expected := filepath.Join(customDir, DefaultCredentialsFile)
	if path != expected {
		t.Errorf("CredentialsPath() = %v, want %v", path, expected)
	}
}

func TestStore_SaveAndLoad(t *testing.T) {
	// Create a temporary directory for test credentials
	tempDir, err := os.MkdirTemp("", "penf-creds-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up test environment with fixed encryption key
	cleanup := setupTestEnv(t, tempDir)
	defer cleanup()

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Test saving API key credentials
	apiKeyCreds := &Credentials{
		AuthType:      AuthTypeAPIKey,
		APIKey:        "pf-test-api-key-12345",
		ServerAddress: "localhost:50051",
		Subject:       "test-user",
	}

	if err := store.Save(apiKeyCreds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if !store.Exists() {
		t.Error("Exists() = false after Save()")
	}

	// Load and verify
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.AuthType != apiKeyCreds.AuthType {
		t.Errorf("Loaded AuthType = %v, want %v", loaded.AuthType, apiKeyCreds.AuthType)
	}
	if loaded.APIKey != apiKeyCreds.APIKey {
		t.Errorf("Loaded APIKey = %v, want %v", loaded.APIKey, apiKeyCreds.APIKey)
	}
	if loaded.ServerAddress != apiKeyCreds.ServerAddress {
		t.Errorf("Loaded ServerAddress = %v, want %v", loaded.ServerAddress, apiKeyCreds.ServerAddress)
	}
	if loaded.Subject != apiKeyCreds.Subject {
		t.Errorf("Loaded Subject = %v, want %v", loaded.Subject, apiKeyCreds.Subject)
	}
	if loaded.LastUpdated.IsZero() {
		t.Error("LastUpdated should be set")
	}
}

func TestStore_SaveAndLoadToken(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-creds-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cleanup := setupTestEnv(t, tempDir)
	defer cleanup()

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Test saving token credentials
	tokenCreds := &Credentials{
		AuthType:      AuthTypeToken,
		Token:         "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0In0.signature",
		RefreshToken:  "refresh-token-12345",
		ExpiresAt:     time.Now().Add(time.Hour),
		ServerAddress: "localhost:50051",
		Subject:       "test-user",
	}

	if err := store.Save(tokenCreds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.AuthType != tokenCreds.AuthType {
		t.Errorf("Loaded AuthType = %v, want %v", loaded.AuthType, tokenCreds.AuthType)
	}
	if loaded.Token != tokenCreds.Token {
		t.Errorf("Loaded Token = %v, want %v", loaded.Token, tokenCreds.Token)
	}
	if loaded.RefreshToken != tokenCreds.RefreshToken {
		t.Errorf("Loaded RefreshToken = %v, want %v", loaded.RefreshToken, tokenCreds.RefreshToken)
	}
}

func TestStore_Delete(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-creds-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cleanup := setupTestEnv(t, tempDir)
	defer cleanup()

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Save credentials
	creds := &Credentials{
		AuthType: AuthTypeAPIKey,
		APIKey:   "test-key",
	}
	if err := store.Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if !store.Exists() {
		t.Error("Exists() = false after Save()")
	}

	// Delete
	if err := store.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if store.Exists() {
		t.Error("Exists() = true after Delete()")
	}

	// Delete again should not error
	if err := store.Delete(); err != nil {
		t.Errorf("Delete() second time error = %v", err)
	}
}

func TestStore_LoadNoCredentials(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-creds-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cleanup := setupTestEnv(t, tempDir)
	defer cleanup()

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	_, err = store.Load()
	if err != ErrNoCredentials {
		t.Errorf("Load() error = %v, want %v", err, ErrNoCredentials)
	}
}

func TestStore_GetActiveCredential_EnvVar(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-creds-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cleanup := setupTestEnv(t, tempDir)
	defer cleanup()

	originalAPIKey := os.Getenv("PENF_API_KEY")
	originalToken := os.Getenv("PENF_TOKEN")
	defer func() {
		if originalAPIKey != "" {
			os.Setenv("PENF_API_KEY", originalAPIKey)
		} else {
			os.Unsetenv("PENF_API_KEY")
		}
		if originalToken != "" {
			os.Setenv("PENF_TOKEN", originalToken)
		} else {
			os.Unsetenv("PENF_TOKEN")
		}
	}()

	os.Unsetenv("PENF_API_KEY")
	os.Unsetenv("PENF_TOKEN")

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Test with PENF_API_KEY env var
	os.Setenv("PENF_API_KEY", "env-api-key")

	creds, err := store.GetActiveCredential()
	if err != nil {
		t.Fatalf("GetActiveCredential() error = %v", err)
	}
	if creds.AuthType != AuthTypeAPIKey {
		t.Errorf("AuthType = %v, want %v", creds.AuthType, AuthTypeAPIKey)
	}
	if creds.APIKey != "env-api-key" {
		t.Errorf("APIKey = %v, want env-api-key", creds.APIKey)
	}

	// Test with PENF_TOKEN env var (API key takes precedence)
	os.Setenv("PENF_TOKEN", "env-token")
	creds, err = store.GetActiveCredential()
	if err != nil {
		t.Fatalf("GetActiveCredential() error = %v", err)
	}
	if creds.AuthType != AuthTypeAPIKey {
		t.Errorf("AuthType should still be api_key when both env vars set, got %v", creds.AuthType)
	}

	// Test with only PENF_TOKEN
	os.Unsetenv("PENF_API_KEY")
	creds, err = store.GetActiveCredential()
	if err != nil {
		t.Fatalf("GetActiveCredential() error = %v", err)
	}
	if creds.AuthType != AuthTypeToken {
		t.Errorf("AuthType = %v, want %v", creds.AuthType, AuthTypeToken)
	}
	if creds.Token != "env-token" {
		t.Errorf("Token = %v, want env-token", creds.Token)
	}
}

func TestStore_GetActiveCredential_Stored(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-creds-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cleanup := setupTestEnv(t, tempDir)
	defer cleanup()

	originalAPIKey := os.Getenv("PENF_API_KEY")
	originalToken := os.Getenv("PENF_TOKEN")
	defer func() {
		if originalAPIKey != "" {
			os.Setenv("PENF_API_KEY", originalAPIKey)
		} else {
			os.Unsetenv("PENF_API_KEY")
		}
		if originalToken != "" {
			os.Setenv("PENF_TOKEN", originalToken)
		} else {
			os.Unsetenv("PENF_TOKEN")
		}
	}()

	os.Unsetenv("PENF_API_KEY")
	os.Unsetenv("PENF_TOKEN")

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Save credentials
	savedCreds := &Credentials{
		AuthType: AuthTypeAPIKey,
		APIKey:   "stored-api-key",
	}
	if err := store.Save(savedCreds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Get active credential (should return stored)
	creds, err := store.GetActiveCredential()
	if err != nil {
		t.Fatalf("GetActiveCredential() error = %v", err)
	}
	if creds.APIKey != "stored-api-key" {
		t.Errorf("APIKey = %v, want stored-api-key", creds.APIKey)
	}
}

func TestStore_GetActiveCredential_ExpiredToken(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-creds-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cleanup := setupTestEnv(t, tempDir)
	defer cleanup()

	originalAPIKey := os.Getenv("PENF_API_KEY")
	originalToken := os.Getenv("PENF_TOKEN")
	defer func() {
		if originalAPIKey != "" {
			os.Setenv("PENF_API_KEY", originalAPIKey)
		} else {
			os.Unsetenv("PENF_API_KEY")
		}
		if originalToken != "" {
			os.Setenv("PENF_TOKEN", originalToken)
		} else {
			os.Unsetenv("PENF_TOKEN")
		}
	}()

	os.Unsetenv("PENF_API_KEY")
	os.Unsetenv("PENF_TOKEN")

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Save expired token
	expiredCreds := &Credentials{
		AuthType:  AuthTypeToken,
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-time.Hour), // Expired 1 hour ago
	}
	if err := store.Save(expiredCreds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Get active credential should fail
	_, err = store.GetActiveCredential()
	if err != ErrExpiredToken {
		t.Errorf("GetActiveCredential() error = %v, want %v", err, ErrExpiredToken)
	}
}

func TestEncryption(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-creds-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cleanup := setupTestEnv(t, tempDir)
	defer cleanup()

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Save credentials with sensitive data
	plaintext := "super-secret-api-key"
	creds := &Credentials{
		AuthType: AuthTypeAPIKey,
		APIKey:   plaintext,
	}
	if err := store.Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read the raw file content
	path := filepath.Join(tempDir, DefaultCredentialsFile)
	rawContent, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Verify the plaintext is NOT in the file (it should be encrypted)
	if string(rawContent) != "" && string(rawContent) == plaintext {
		t.Error("Plaintext API key found in file - encryption not working")
	}

	// But we should be able to load it back
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.APIKey != plaintext {
		t.Errorf("Decrypted APIKey = %v, want %v", loaded.APIKey, plaintext)
	}
}

func TestMaskCredential(t *testing.T) {
	tests := []struct {
		input        string
		minAsterisks int
	}{
		{"short", 5},
		{"12345678", 8},
		{"1234567890123456", 8},
		{"pf-abcdefghij1234567890", 10},
	}

	for _, tc := range tests {
		result := MaskCredential(tc.input)
		asteriskCount := 0
		for _, c := range result {
			if c == '*' {
				asteriskCount++
			}
		}
		if asteriskCount < tc.minAsterisks {
			t.Errorf("MaskCredential(%q) = %q, want at least %d asterisks, got %d",
				tc.input, result, tc.minAsterisks, asteriskCount)
		}
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"short", "*****"},
		{"12345678", "********"},
		{"pf-abcdefghij1234567890", "pf-********...****"},
		{"abcdefghij1234567890", "abcd********..."},
	}

	for _, tc := range tests {
		result := MaskAPIKey(tc.input)
		if result != tc.expected {
			t.Errorf("MaskAPIKey(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"short", "*****"},
		{"12345678901234567890", "********************"},
		{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature", "eyJhbGci...ignature"},
	}

	for _, tc := range tests {
		result := MaskToken(tc.input)
		if result != tc.expected {
			t.Errorf("MaskToken(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestFormatExpiry(t *testing.T) {
	t.Run("zero time", func(t *testing.T) {
		result := FormatExpiry(time.Time{})
		if result != "never" {
			t.Errorf("FormatExpiry(zero) = %q, want 'never'", result)
		}
	})

	t.Run("expired", func(t *testing.T) {
		result := FormatExpiry(time.Now().Add(-time.Hour))
		if result != "expired" {
			t.Errorf("FormatExpiry(past) = %q, want 'expired'", result)
		}
	})

	t.Run("minutes remaining", func(t *testing.T) {
		result := FormatExpiry(time.Now().Add(30 * time.Minute))
		if !strings.Contains(result, "minutes") {
			t.Errorf("FormatExpiry(30min) = %q, want to contain 'minutes'", result)
		}
	})

	t.Run("hours remaining", func(t *testing.T) {
		result := FormatExpiry(time.Now().Add(5 * time.Hour))
		if !strings.Contains(result, "hours") {
			t.Errorf("FormatExpiry(5h) = %q, want to contain 'hours'", result)
		}
	})

	t.Run("days remaining", func(t *testing.T) {
		result := FormatExpiry(time.Now().Add(72 * time.Hour))
		if !strings.Contains(result, "days") {
			t.Errorf("FormatExpiry(72h) = %q, want to contain 'days'", result)
		}
	})
}

func TestGenerateAPIKeyID(t *testing.T) {
	// Same key should produce same ID
	key := "pf-test-api-key-12345"
	id1 := GenerateAPIKeyID(key)
	id2 := GenerateAPIKeyID(key)
	if id1 != id2 {
		t.Errorf("GenerateAPIKeyID produced different IDs for same key: %v != %v", id1, id2)
	}

	// Different keys should produce different IDs
	differentKey := "pf-different-key-67890"
	id3 := GenerateAPIKeyID(differentKey)
	if id1 == id3 {
		t.Errorf("GenerateAPIKeyID produced same ID for different keys: %v", id1)
	}

	// ID should be 8 characters (4 bytes hex encoded)
	if len(id1) != 8 {
		t.Errorf("GenerateAPIKeyID length = %d, want 8", len(id1))
	}
}

func TestNewStoreWithKeyProvider(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-creds-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Use env key provider with test key
	os.Setenv("PENF_CONFIG_DIR", tempDir)
	defer os.Unsetenv("PENF_CONFIG_DIR")

	testKeyHex := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	os.Setenv("TEST_CUSTOM_KEY", testKeyHex)
	defer os.Unsetenv("TEST_CUSTOM_KEY")

	provider := NewEnvKeyProvider("TEST_CUSTOM_KEY")
	store, err := NewStoreWithKeyProvider(provider)
	if err != nil {
		t.Fatalf("NewStoreWithKeyProvider() error = %v", err)
	}

	// Save and load credentials
	creds := &Credentials{
		AuthType: AuthTypeAPIKey,
		APIKey:   "test-api-key",
	}
	if err := store.Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.APIKey != creds.APIKey {
		t.Errorf("Loaded APIKey = %v, want %v", loaded.APIKey, creds.APIKey)
	}
}

func TestMigrationFromLegacyKey(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "penf-creds-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("PENF_CONFIG_DIR", tempDir)
	defer os.Unsetenv("PENF_CONFIG_DIR")

	// Step 1: Create credentials encrypted with the legacy key
	legacyKey, err := deriveLegacyEncryptionKey()
	if err != nil {
		t.Fatalf("deriveLegacyEncryptionKey() error = %v", err)
	}

	// Encrypt test data with legacy key
	plaintext := "test-api-key-12345"
	encrypted, err := encryptWithKey(plaintext, legacyKey)
	if err != nil {
		t.Fatalf("encryptWithKey() error = %v", err)
	}

	// Write credentials file with legacy-encrypted data
	legacyCreds := Credentials{
		AuthType: AuthTypeAPIKey,
		APIKey:   encrypted,
	}
	data, _ := yaml.Marshal(&legacyCreds)
	credPath := filepath.Join(tempDir, DefaultCredentialsFile)
	if err := os.WriteFile(credPath, data, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Step 2: Create a new store with a different key (simulating new keyring-based key)
	newKeyHex := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	os.Setenv("PENF_ENCRYPTION_KEY", newKeyHex)
	defer os.Unsetenv("PENF_ENCRYPTION_KEY")

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Step 3: Load credentials - should have been migrated
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.APIKey != plaintext {
		t.Errorf("Migrated APIKey = %v, want %v", loaded.APIKey, plaintext)
	}
}

// encryptWithKey encrypts data using a specific key (helper for migration tests)
func encryptWithKey(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}
