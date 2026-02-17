package credentials

import (
	"encoding/hex"
	"os"
	"testing"
)

func TestEnvKeyProvider_GetKey(t *testing.T) {
	envVar := "TEST_PENF_ENCRYPTION_KEY"
	originalValue := os.Getenv(envVar)
	defer os.Setenv(envVar, originalValue)

	t.Run("valid key", func(t *testing.T) {
		// Generate a valid 32-byte key as hex
		validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		os.Setenv(envVar, validKey)

		provider := NewEnvKeyProvider(envVar)
		key, err := provider.GetKey()
		if err != nil {
			t.Fatalf("GetKey() error = %v", err)
		}

		if len(key) != keyLength {
			t.Errorf("GetKey() returned %d bytes, want %d", len(key), keyLength)
		}

		expectedKey, _ := hex.DecodeString(validKey)
		if string(key) != string(expectedKey) {
			t.Errorf("GetKey() returned wrong key")
		}
	})

	t.Run("missing env var", func(t *testing.T) {
		os.Unsetenv(envVar)

		provider := NewEnvKeyProvider(envVar)
		_, err := provider.GetKey()
		if err == nil {
			t.Error("GetKey() expected error for missing env var")
		}
	})

	t.Run("invalid hex", func(t *testing.T) {
		os.Setenv(envVar, "not-valid-hex")

		provider := NewEnvKeyProvider(envVar)
		_, err := provider.GetKey()
		if err == nil {
			t.Error("GetKey() expected error for invalid hex")
		}
	})

	t.Run("wrong length", func(t *testing.T) {
		os.Setenv(envVar, "0123456789abcdef") // Only 16 bytes

		provider := NewEnvKeyProvider(envVar)
		_, err := provider.GetKey()
		if err == nil {
			t.Error("GetKey() expected error for wrong key length")
		}
	})
}

func TestEnvKeyProvider_ResetKey(t *testing.T) {
	provider := NewEnvKeyProvider("TEST_KEY")
	_, err := provider.ResetKey()
	if err == nil {
		t.Error("ResetKey() expected error for env-based provider")
	}
}

func TestEnvKeyProvider_Description(t *testing.T) {
	envVar := "MY_CUSTOM_KEY"
	provider := NewEnvKeyProvider(envVar)
	desc := provider.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
	if !containsString(desc, envVar) {
		t.Errorf("Description() = %q, should mention %q", desc, envVar)
	}
}

func TestPassphraseKeyProvider_GetKey(t *testing.T) {
	t.Run("valid passphrase and salt", func(t *testing.T) {
		salt, err := GenerateSalt()
		if err != nil {
			t.Fatalf("GenerateSalt() error = %v", err)
		}

		provider := NewPassphraseKeyProvider("my-secure-passphrase", salt)
		key, err := provider.GetKey()
		if err != nil {
			t.Fatalf("GetKey() error = %v", err)
		}

		if len(key) != keyLength {
			t.Errorf("GetKey() returned %d bytes, want %d", len(key), keyLength)
		}
	})

	t.Run("same passphrase and salt produces same key", func(t *testing.T) {
		salt, _ := GenerateSalt()
		passphrase := "test-passphrase"

		provider1 := NewPassphraseKeyProvider(passphrase, salt)
		key1, _ := provider1.GetKey()

		provider2 := NewPassphraseKeyProvider(passphrase, salt)
		key2, _ := provider2.GetKey()

		if string(key1) != string(key2) {
			t.Error("Same passphrase and salt should produce same key")
		}
	})

	t.Run("different salts produce different keys", func(t *testing.T) {
		salt1, _ := GenerateSalt()
		salt2, _ := GenerateSalt()
		passphrase := "test-passphrase"

		provider1 := NewPassphraseKeyProvider(passphrase, salt1)
		key1, _ := provider1.GetKey()

		provider2 := NewPassphraseKeyProvider(passphrase, salt2)
		key2, _ := provider2.GetKey()

		if string(key1) == string(key2) {
			t.Error("Different salts should produce different keys")
		}
	})

	t.Run("different passphrases produce different keys", func(t *testing.T) {
		salt, _ := GenerateSalt()

		provider1 := NewPassphraseKeyProvider("passphrase-1", salt)
		key1, _ := provider1.GetKey()

		provider2 := NewPassphraseKeyProvider("passphrase-2", salt)
		key2, _ := provider2.GetKey()

		if string(key1) == string(key2) {
			t.Error("Different passphrases should produce different keys")
		}
	})

	t.Run("empty passphrase", func(t *testing.T) {
		salt, _ := GenerateSalt()
		provider := NewPassphraseKeyProvider("", salt)
		_, err := provider.GetKey()
		if err == nil {
			t.Error("GetKey() expected error for empty passphrase")
		}
	})

	t.Run("empty salt", func(t *testing.T) {
		provider := NewPassphraseKeyProvider("passphrase", nil)
		_, err := provider.GetKey()
		if err == nil {
			t.Error("GetKey() expected error for empty salt")
		}
	})
}

func TestPassphraseKeyProvider_Description(t *testing.T) {
	provider := NewPassphraseKeyProvider("test", []byte("salt"))
	desc := provider.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
	if !containsString(desc, "Argon2") {
		t.Errorf("Description() = %q, should mention Argon2", desc)
	}
}

func TestGenerateSalt(t *testing.T) {
	t.Run("generates 16 bytes", func(t *testing.T) {
		salt, err := GenerateSalt()
		if err != nil {
			t.Fatalf("GenerateSalt() error = %v", err)
		}
		if len(salt) != 16 {
			t.Errorf("GenerateSalt() returned %d bytes, want 16", len(salt))
		}
	})

	t.Run("generates unique salts", func(t *testing.T) {
		salt1, _ := GenerateSalt()
		salt2, _ := GenerateSalt()

		if string(salt1) == string(salt2) {
			t.Error("GenerateSalt() should return unique values")
		}
	})
}

func TestKeyringKeyProvider_Description(t *testing.T) {
	provider := NewKeyringKeyProvider()
	desc := provider.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

// TestKeyringKeyProvider_Integration tests the keyring provider if available.
// This test is skipped in CI environments where keyring may not be available.
func TestKeyringKeyProvider_Integration(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping keyring test in CI environment")
	}

	provider := NewKeyringKeyProvider()

	// Try to get a key
	key, err := provider.GetKey()
	if err != nil {
		t.Skipf("Keyring not available: %v", err)
	}

	if len(key) != keyLength {
		t.Errorf("GetKey() returned %d bytes, want %d", len(key), keyLength)
	}

	// Getting the key again should return the same key
	key2, err := provider.GetKey()
	if err != nil {
		t.Fatalf("Second GetKey() error = %v", err)
	}

	if string(key) != string(key2) {
		t.Error("GetKey() should return the same key on subsequent calls")
	}
}

func TestGetDefaultKeyProvider_WithEnvVar(t *testing.T) {
	envVar := "PENF_ENCRYPTION_KEY"
	originalValue := os.Getenv(envVar)
	defer os.Setenv(envVar, originalValue)

	// Set a valid encryption key
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.Setenv(envVar, validKey)

	provider, err := GetDefaultKeyProvider()
	if err != nil {
		t.Fatalf("GetDefaultKeyProvider() error = %v", err)
	}

	desc := provider.Description()
	if !containsString(desc, envVar) {
		t.Errorf("Expected env provider, got: %s", desc)
	}

	key, err := provider.GetKey()
	if err != nil {
		t.Fatalf("GetKey() error = %v", err)
	}

	if len(key) != keyLength {
		t.Errorf("GetKey() returned %d bytes, want %d", len(key), keyLength)
	}
}

// containsString is a helper to check if a string contains a substring
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
