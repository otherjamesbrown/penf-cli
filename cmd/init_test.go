package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/otherjamesbrown/penf-cli/config"
)

// TestDownloadAssistantClaudeMd_PreservesExistingFile tests that downloadAssistantClaudeMd
// does NOT overwrite an existing CLAUDE.md file with custom user content.
// This test SHOULD FAIL against the current implementation (reproduces bug pf-4cd53d).
func TestDownloadAssistantClaudeMd_PreservesExistingFile(t *testing.T) {
	// Create a temporary directory to simulate a project directory
	tempDir := t.TempDir()

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Create a CLAUDE.md with custom user content
	claudeMdPath := filepath.Join(tempDir, "CLAUDE.md")
	customContent := `# My Custom CLAUDE.md

This is my customized configuration with:
- Custom server settings
- My preferred workflows
- Personal notes and reminders

This content MUST NOT be overwritten by penf update!
`

	if err := os.WriteFile(claudeMdPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("failed to create custom CLAUDE.md: %v", err)
	}

	// Create a config to pass to downloadAssistantClaudeMd
	cfg := &config.CLIConfig{
		ServerAddress: "dev02.brown.chat:50051",
	}

	// Call the function that should preserve existing files
	err = downloadAssistantClaudeMd(cfg)
	if err != nil {
		t.Fatalf("downloadAssistantClaudeMd failed: %v", err)
	}

	// Read the CLAUDE.md file after the function call
	actualContent, err := os.ReadFile(claudeMdPath)
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md after update: %v", err)
	}

	// CRITICAL: The custom content MUST be preserved
	if string(actualContent) != customContent {
		t.Errorf("downloadAssistantClaudeMd overwrote existing CLAUDE.md\n"+
			"Expected (custom content):\n%s\n"+
			"Got (default template):\n%s\n"+
			"Bug pf-4cd53d: User customizations were destroyed!",
			customContent, string(actualContent))
	}
}

// TestDownloadAssistantClaudeMd_CreatesNewFile tests that downloadAssistantClaudeMd
// DOES create a CLAUDE.md file when none exists (normal first-run behavior).
// This test should PASS - it validates the normal case still works.
func TestDownloadAssistantClaudeMd_CreatesNewFile(t *testing.T) {
	// Create a temporary directory to simulate a project directory
	tempDir := t.TempDir()

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	claudeMdPath := filepath.Join(tempDir, "CLAUDE.md")

	// Verify CLAUDE.md does NOT exist yet
	if _, err := os.Stat(claudeMdPath); !os.IsNotExist(err) {
		t.Fatalf("CLAUDE.md should not exist before test, but stat returned: %v", err)
	}

	// Create a config to pass to downloadAssistantClaudeMd
	cfg := &config.CLIConfig{
		ServerAddress: "dev02.brown.chat:50051",
	}

	// Call the function - should create new file
	err = downloadAssistantClaudeMd(cfg)
	if err != nil {
		t.Fatalf("downloadAssistantClaudeMd failed: %v", err)
	}

	// Verify CLAUDE.md was created
	if _, err := os.Stat(claudeMdPath); os.IsNotExist(err) {
		t.Fatalf("CLAUDE.md should have been created but does not exist")
	}

	// Read and verify content contains expected template elements
	content, err := os.ReadFile(claudeMdPath)
	if err != nil {
		t.Fatalf("failed to read created CLAUDE.md: %v", err)
	}

	// Check for expected template content
	contentStr := string(content)
	expectedStrings := []string{
		"# Penfold CLI",
		cfg.ServerAddress,
		"penf status",
		"penf health",
	}

	for _, expected := range expectedStrings {
		if !contains(contentStr, expected) {
			t.Errorf("created CLAUDE.md missing expected content: %q\nFull content:\n%s",
				expected, contentStr)
		}
	}
}

// TestInitUserPreferences_PreservesExistingFile validates that initUserPreferences
// correctly skips overwriting existing preferences.md files.
// This test documents the CORRECT behavior that downloadAssistantClaudeMd should follow.
func TestInitUserPreferences_PreservesExistingFile(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Create custom preferences.md
	prefsPath := filepath.Join(tempDir, "preferences.md")
	customContent := "# My Custom Preferences\n\nDo not overwrite me!\n"

	if err := os.WriteFile(prefsPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("failed to create custom preferences.md: %v", err)
	}

	// Call initUserPreferences - should skip existing file
	err = initUserPreferences()
	if err != nil {
		t.Fatalf("initUserPreferences failed: %v", err)
	}

	// Verify content was NOT overwritten
	actualContent, err := os.ReadFile(prefsPath)
	if err != nil {
		t.Fatalf("failed to read preferences.md: %v", err)
	}

	if string(actualContent) != customContent {
		t.Errorf("initUserPreferences overwrote existing preferences.md\n"+
			"Expected: %s\nGot: %s", customContent, string(actualContent))
	}
}

// contains is a helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

// indexOf returns the index of substr in s, or -1 if not found.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
