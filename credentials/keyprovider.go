// Package credentials provides secure credential storage for the penf CLI.
package credentials

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/argon2"
)

const (
	// keyringService is the service name used in the system keyring.
	keyringService = "penf-cli"
	// keyringUser is the user/account name used in the system keyring.
	keyringUser = "encryption-key"
	// keyLength is the required encryption key length (256 bits for AES-256).
	keyLength = 32
)

// Argon2 parameters for passphrase-based key derivation.
// These are conservative defaults balancing security and performance.
const (
	argon2Time    = 1
	argon2Memory  = 64 * 1024 // 64 MB
	argon2Threads = 4
)

// ErrKeyringUnavailable indicates the system keyring is not available.
var ErrKeyringUnavailable = errors.New("system keyring unavailable")

// KeyProvider is an interface for obtaining the encryption key.
type KeyProvider interface {
	// GetKey returns the 32-byte encryption key.
	// If no key exists, it should generate and store a new one.
	GetKey() ([]byte, error)

	// ResetKey generates a new encryption key, replacing any existing key.
	// This is used during credential migration.
	ResetKey() ([]byte, error)

	// Description returns a human-readable description of the key storage mechanism.
	Description() string
}

// KeyringKeyProvider stores the encryption key in the system keyring
// (macOS Keychain, Windows Credential Manager, Linux Secret Service).
type KeyringKeyProvider struct {
	mu sync.Mutex
}

// NewKeyringKeyProvider creates a new KeyringKeyProvider.
func NewKeyringKeyProvider() *KeyringKeyProvider {
	return &KeyringKeyProvider{}
}

// GetKey retrieves the encryption key from the system keyring.
// If no key exists, it generates a new cryptographically random key and stores it.
func (p *KeyringKeyProvider) GetKey() ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Try to retrieve existing key
	keyHex, err := keyring.Get(keyringService, keyringUser)
	if err == nil {
		// Key exists, decode it
		key, decErr := hex.DecodeString(keyHex)
		if decErr == nil && len(key) == keyLength {
			return key, nil
		}
		// Invalid key format, regenerate
	}

	// Check if it's a "not found" error vs an actual error
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("%w: %v", ErrKeyringUnavailable, err)
	}

	// Generate new key
	return p.generateAndStoreKey()
}

// ResetKey generates a new encryption key and stores it in the keyring.
func (p *KeyringKeyProvider) ResetKey() ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.generateAndStoreKey()
}

// generateAndStoreKey creates a new random key and stores it in the keyring.
// Caller must hold p.mu.
func (p *KeyringKeyProvider) generateAndStoreKey() ([]byte, error) {
	key := make([]byte, keyLength)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating random key: %w", err)
	}

	keyHex := hex.EncodeToString(key)
	if err := keyring.Set(keyringService, keyringUser, keyHex); err != nil {
		return nil, fmt.Errorf("%w: storing key: %v", ErrKeyringUnavailable, err)
	}

	return key, nil
}

// Description returns a description of this key provider.
func (p *KeyringKeyProvider) Description() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS Keychain"
	case "windows":
		return "Windows Credential Manager"
	default:
		return "System Keyring (Secret Service)"
	}
}

// PassphraseKeyProvider derives an encryption key from a user-provided passphrase
// using Argon2id. This is used as a fallback when the system keyring is unavailable.
type PassphraseKeyProvider struct {
	passphrase string
	salt       []byte
}

// NewPassphraseKeyProvider creates a new PassphraseKeyProvider.
// The salt should be stored alongside the encrypted credentials.
func NewPassphraseKeyProvider(passphrase string, salt []byte) *PassphraseKeyProvider {
	return &PassphraseKeyProvider{
		passphrase: passphrase,
		salt:       salt,
	}
}

// GetKey derives the encryption key from the passphrase using Argon2id.
func (p *PassphraseKeyProvider) GetKey() ([]byte, error) {
	if p.passphrase == "" {
		return nil, errors.New("passphrase is required")
	}
	if len(p.salt) == 0 {
		return nil, errors.New("salt is required")
	}

	key := argon2.IDKey(
		[]byte(p.passphrase),
		p.salt,
		argon2Time,
		argon2Memory,
		argon2Threads,
		keyLength,
	)

	return key, nil
}

// ResetKey returns the same key (passphrase-derived keys cannot be reset).
func (p *PassphraseKeyProvider) ResetKey() ([]byte, error) {
	return p.GetKey()
}

// Description returns a description of this key provider.
func (p *PassphraseKeyProvider) Description() string {
	return "Passphrase-derived key (Argon2id)"
}

// GenerateSalt generates a random salt for passphrase key derivation.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generating salt: %w", err)
	}
	return salt, nil
}

// EnvKeyProvider uses an encryption key from an environment variable.
// This is primarily for testing and CI environments.
type EnvKeyProvider struct {
	envVar string
}

// NewEnvKeyProvider creates a new EnvKeyProvider that reads the key from the given env var.
func NewEnvKeyProvider(envVar string) *EnvKeyProvider {
	return &EnvKeyProvider{envVar: envVar}
}

// GetKey returns the key from the environment variable.
func (p *EnvKeyProvider) GetKey() ([]byte, error) {
	keyHex := os.Getenv(p.envVar)
	if keyHex == "" {
		return nil, fmt.Errorf("environment variable %s not set", p.envVar)
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid key in %s: %w", p.envVar, err)
	}

	if len(key) != keyLength {
		return nil, fmt.Errorf("key in %s must be %d bytes, got %d", p.envVar, keyLength, len(key))
	}

	return key, nil
}

// ResetKey is not supported for environment-based keys.
func (p *EnvKeyProvider) ResetKey() ([]byte, error) {
	return nil, errors.New("cannot reset environment-based key")
}

// Description returns a description of this key provider.
func (p *EnvKeyProvider) Description() string {
	return fmt.Sprintf("Environment variable (%s)", p.envVar)
}

// GetDefaultKeyProvider returns the appropriate key provider for the current environment.
// Priority:
// 1. PENF_ENCRYPTION_KEY environment variable (for CI/testing)
// 2. System keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service)
//
// If the keyring is unavailable, it returns an error suggesting the user set
// the environment variable or use a passphrase-based provider.
func GetDefaultKeyProvider() (KeyProvider, error) {
	// Check for environment variable first (CI/testing)
	if os.Getenv("PENF_ENCRYPTION_KEY") != "" {
		return NewEnvKeyProvider("PENF_ENCRYPTION_KEY"), nil
	}

	// Try keyring
	provider := NewKeyringKeyProvider()

	// Test if keyring is available by attempting to get or create a key
	_, err := provider.GetKey()
	if err != nil {
		if errors.Is(err, ErrKeyringUnavailable) {
			return nil, fmt.Errorf("system keyring unavailable; set PENF_ENCRYPTION_KEY environment variable: %w", err)
		}
		return nil, err
	}

	return provider, nil
}

// IsKeyringAvailable checks if the system keyring is accessible.
func IsKeyringAvailable() bool {
	provider := NewKeyringKeyProvider()
	_, err := provider.GetKey()
	return err == nil
}
