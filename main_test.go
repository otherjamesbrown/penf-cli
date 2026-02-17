package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	if versionCmd == nil {
		t.Fatal("versionCmd is nil")
	}

	if versionCmd.Use != "version" {
		t.Errorf("Unexpected Use: %s", versionCmd.Use)
	}

	if versionCmd.Short != "Print version information" {
		t.Errorf("Unexpected Short: %s", versionCmd.Short)
	}
}

func TestVersionFlags(t *testing.T) {
	// Reset the command to ensure flags are registered.
	// (init() is called automatically, but we verify here)

	allFlag := versionCmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Error("--all flag not found on version command")
	}

	outputJSONFlag := versionCmd.Flags().Lookup("output-json")
	if outputJSONFlag == nil {
		t.Error("--output-json flag not found on version command")
	}
}

// TestVersionChangelogFlag verifies that the --changelog flag exists and is recognized.
func TestVersionChangelogFlag(t *testing.T) {
	changelogFlag := versionCmd.Flags().Lookup("changelog")
	if changelogFlag == nil {
		t.Error("--changelog flag not found on version command")
	}
}

// TestVersionChangelogOutput verifies that --changelog produces commit-like output.
// The output should contain lines with commit hashes (7+ chars) followed by messages.
func TestVersionChangelogOutput(t *testing.T) {
	// Capture stdout
	var buf bytes.Buffer
	originalStdout := versionCmd.OutOrStdout()
	versionCmd.SetOut(&buf)
	defer versionCmd.SetOut(originalStdout)

	// Set the changelog flag
	versionChangelog = true
	defer func() { versionChangelog = false }()

	// Execute the version command
	err := versionCmd.RunE(versionCmd, []string{})
	if err != nil {
		t.Fatalf("version command with --changelog failed: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("version --changelog produced no output")
	}

	// Check that output contains commit-like entries.
	// Expected format: "abc1234 commit message" (hash + space + message)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	foundCommitEntry := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Look for lines starting with hex chars (commit hash)
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			hash := fields[0]
			// Commit hash should be at least 7 chars and hexadecimal
			if len(hash) >= 7 && isHexString(hash) {
				foundCommitEntry = true
				break
			}
		}
	}

	if !foundCommitEntry {
		t.Errorf("version --changelog output did not contain commit-like entries (hash + message).\nOutput:\n%s", output)
	}
}

// TestVersionChangelogJSON verifies that --changelog + --output-json produces valid JSON.
func TestVersionChangelogJSON(t *testing.T) {
	// Capture stdout
	var buf bytes.Buffer
	originalStdout := versionCmd.OutOrStdout()
	versionCmd.SetOut(&buf)
	defer versionCmd.SetOut(originalStdout)

	// Set both flags
	versionChangelog = true
	versionOutputJSON = true
	defer func() {
		versionChangelog = false
		versionOutputJSON = false
	}()

	// Execute the version command
	err := versionCmd.RunE(versionCmd, []string{})
	if err != nil {
		t.Fatalf("version command with --changelog --output-json failed: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("version --changelog --output-json produced no output")
	}

	// Verify it's valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("version --changelog --output-json produced invalid JSON: %v\nOutput:\n%s", err, output)
	}

	// Verify JSON structure contains expected fields (e.g., array or object with changelog data)
	// This depends on the implementation, but at minimum it should be valid JSON.
	// More specific structure checks can be added based on the implementation.
}

// TestVersionWithoutChangelogUnchanged verifies that the default version output (without --changelog) remains unchanged.
func TestVersionWithoutChangelogUnchanged(t *testing.T) {
	// Capture stdout
	var buf bytes.Buffer
	originalStdout := versionCmd.OutOrStdout()
	versionCmd.SetOut(&buf)
	defer versionCmd.SetOut(originalStdout)

	// Ensure flags are false
	versionChangelog = false
	versionAll = false
	versionOutputJSON = false

	// Execute the version command
	err := versionCmd.RunE(versionCmd, []string{})
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("version command produced no output")
	}

	// Check that output contains expected version lines:
	// "penf version X.Y.Z"
	// "  commit: ..."
	// "  built: ..."
	if !strings.Contains(output, "penf version") {
		t.Errorf("version output does not contain 'penf version'. Output:\n%s", output)
	}
	if !strings.Contains(output, "commit:") {
		t.Errorf("version output does not contain 'commit:'. Output:\n%s", output)
	}
	if !strings.Contains(output, "built:") {
		t.Errorf("version output does not contain 'built:'. Output:\n%s", output)
	}

	// Ensure it does NOT contain commit log entries (which would appear if --changelog was on)
	// We check for absence of lines starting with hex hashes (other than the commit: line)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "penf version") || strings.HasPrefix(trimmed, "commit:") || strings.HasPrefix(trimmed, "built:") {
			continue
		}
		// If we find a line that looks like a git log entry, fail
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 && len(fields[0]) >= 7 && isHexString(fields[0]) {
			t.Errorf("version output without --changelog should not contain git log entries. Found: %s", trimmed)
		}
	}
}

// isHexString checks if a string contains only hexadecimal characters.
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}
