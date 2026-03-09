// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/otherjamesbrown/penf-cli/config"
)

// --- validateEmailAddress ---

func TestValidateEmailAddress_Valid(t *testing.T) {
	cases := []string{
		"alice@example.com",
		"bob+tag@sub.domain.org",
		"user@localhost",
	}
	for _, addr := range cases {
		if err := validateEmailAddress(addr); err != nil {
			t.Errorf("expected valid for %q, got error: %v", addr, err)
		}
	}
}

func TestValidateEmailAddress_Invalid(t *testing.T) {
	cases := []string{
		"",
		"notanemail",
		"@nodomain",
		"nolocal@",
		"two@@ats.com",
	}
	for _, addr := range cases {
		if err := validateEmailAddress(addr); err == nil {
			t.Errorf("expected error for %q, got nil", addr)
		}
	}
}

// --- normaliseEmail ---

func TestNormaliseEmail(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Alice@Example.COM", "alice@example.com"},
		{"  bob@example.org  ", "bob@example.org"},
		{"UPPER@CASE.IO", "upper@case.io"},
	}
	for _, c := range cases {
		got := normaliseEmail(c.input)
		if got != c.want {
			t.Errorf("normaliseEmail(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// --- parseWhitelist ---

func TestParseWhitelist_Empty(t *testing.T) {
	addrs, err := parseWhitelist("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(addrs) != 0 {
		t.Errorf("expected empty slice, got %v", addrs)
	}
}

func TestParseWhitelist_Whitespace(t *testing.T) {
	addrs, err := parseWhitelist("   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(addrs) != 0 {
		t.Errorf("expected empty slice, got %v", addrs)
	}
}

func TestParseWhitelist_ValidJSON(t *testing.T) {
	addrs, err := parseWhitelist(`["alice@example.com","bob@example.com"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(addrs) != 2 {
		t.Fatalf("expected 2 addresses, got %d: %v", len(addrs), addrs)
	}
	if addrs[0] != "alice@example.com" || addrs[1] != "bob@example.com" {
		t.Errorf("unexpected values: %v", addrs)
	}
}

func TestParseWhitelist_InvalidJSON(t *testing.T) {
	_, err := parseWhitelist(`not-json`)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// --- marshalWhitelist ---

func TestMarshalWhitelist_Empty(t *testing.T) {
	v, err := marshalWhitelist([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "[]" {
		t.Errorf("expected '[]', got %q", v)
	}
}

func TestMarshalWhitelist_Multiple(t *testing.T) {
	v, err := marshalWhitelist([]string{"alice@example.com", "bob@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Re-parse to verify round-trip
	addrs, err := parseWhitelist(v)
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if len(addrs) != 2 {
		t.Errorf("expected 2 addresses after round-trip, got %d", len(addrs))
	}
}

// --- addToWhitelist ---

func TestAddToWhitelist_NewAddress(t *testing.T) {
	addrs := []string{"alice@example.com"}
	result, added := addToWhitelist(addrs, "bob@example.com")
	if !added {
		t.Error("expected added=true for new address")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 addresses, got %d", len(result))
	}
}

func TestAddToWhitelist_Duplicate(t *testing.T) {
	addrs := []string{"alice@example.com", "bob@example.com"}
	result, added := addToWhitelist(addrs, "alice@example.com")
	if added {
		t.Error("expected added=false for duplicate address")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 addresses after duplicate add, got %d", len(result))
	}
}

func TestAddToWhitelist_EmptyList(t *testing.T) {
	addrs := []string{}
	result, added := addToWhitelist(addrs, "new@example.com")
	if !added {
		t.Error("expected added=true when adding to empty list")
	}
	if len(result) != 1 || result[0] != "new@example.com" {
		t.Errorf("unexpected result: %v", result)
	}
}

// --- removeFromWhitelist ---

func TestRemoveFromWhitelist_Exists(t *testing.T) {
	addrs := []string{"alice@example.com", "bob@example.com"}
	result, removed := removeFromWhitelist(addrs, "alice@example.com")
	if !removed {
		t.Error("expected removed=true for existing address")
	}
	if len(result) != 1 || result[0] != "bob@example.com" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestRemoveFromWhitelist_NotPresent(t *testing.T) {
	addrs := []string{"alice@example.com"}
	result, removed := removeFromWhitelist(addrs, "ghost@example.com")
	if removed {
		t.Error("expected removed=false for absent address")
	}
	if len(result) != 1 {
		t.Errorf("expected list unchanged, got %v", result)
	}
}

func TestRemoveFromWhitelist_EmptyList(t *testing.T) {
	addrs := []string{}
	result, removed := removeFromWhitelist(addrs, "nobody@example.com")
	if removed {
		t.Error("expected removed=false for empty list")
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

// --- Command structure ---

func TestNewConfigEmailCmd_Structure(t *testing.T) {
	deps := &PipelineCommandDeps{
		LoadConfig: func() (*config.CLIConfig, error) {
			return &config.CLIConfig{}, nil
		},
	}

	emailCmd := NewConfigEmailCmd(deps)

	if emailCmd.Use != "email" {
		t.Errorf("expected Use='email', got %q", emailCmd.Use)
	}

	// Verify whitelist subcommand exists
	var whitelistCmd *cobra.Command
	for _, sub := range emailCmd.Commands() {
		if sub.Name() == "whitelist" {
			whitelistCmd = sub
			break
		}
	}
	if whitelistCmd == nil {
		t.Fatal("email command missing 'whitelist' subcommand")
	}

	// Verify whitelist has list/add/remove
	subNames := map[string]bool{}
	for _, s := range whitelistCmd.Commands() {
		subNames[s.Name()] = true
	}
	for _, expected := range []string{"list", "add", "remove"} {
		if !subNames[expected] {
			t.Errorf("whitelist command missing '%s' subcommand", expected)
		}
	}
}

func TestConfigEmailWhitelistAddFlags(t *testing.T) {
	deps := &PipelineCommandDeps{
		LoadConfig: func() (*config.CLIConfig, error) {
			return &config.CLIConfig{}, nil
		},
	}

	emailCmd := NewConfigEmailCmd(deps)
	var whitelistCmd, addCmd *cobra.Command
	for _, sub := range emailCmd.Commands() {
		if sub.Name() == "whitelist" {
			whitelistCmd = sub
			break
		}
	}
	if whitelistCmd == nil {
		t.Fatal("missing whitelist command")
	}
	for _, sub := range whitelistCmd.Commands() {
		if sub.Name() == "add" {
			addCmd = sub
			break
		}
	}
	if addCmd == nil {
		t.Fatal("missing add command")
	}

	if addCmd.Flags().Lookup("inbound") == nil {
		t.Error("add command missing --inbound flag")
	}
	if addCmd.Flags().Lookup("outbound") == nil {
		t.Error("add command missing --outbound flag")
	}
}

func TestConfigEmailWhitelistRemoveFlags(t *testing.T) {
	deps := &PipelineCommandDeps{
		LoadConfig: func() (*config.CLIConfig, error) {
			return &config.CLIConfig{}, nil
		},
	}

	emailCmd := NewConfigEmailCmd(deps)
	var whitelistCmd, removeCmd *cobra.Command
	for _, sub := range emailCmd.Commands() {
		if sub.Name() == "whitelist" {
			whitelistCmd = sub
			break
		}
	}
	if whitelistCmd == nil {
		t.Fatal("missing whitelist command")
	}
	for _, sub := range whitelistCmd.Commands() {
		if sub.Name() == "remove" {
			removeCmd = sub
			break
		}
	}
	if removeCmd == nil {
		t.Fatal("missing remove command")
	}

	if removeCmd.Flags().Lookup("inbound") == nil {
		t.Error("remove command missing --inbound flag")
	}
	if removeCmd.Flags().Lookup("outbound") == nil {
		t.Error("remove command missing --outbound flag")
	}
}

func TestFriendlyKeyName(t *testing.T) {
	if got := friendlyKeyName(keyInboundWhitelist); got != "inbound" {
		t.Errorf("expected 'inbound', got %q", got)
	}
	if got := friendlyKeyName(keyOutboundWhitelist); got != "outbound" {
		t.Errorf("expected 'outbound', got %q", got)
	}
	if got := friendlyKeyName("unknown.key"); !strings.Contains(got, "unknown.key") {
		t.Errorf("expected unknown key to appear in output, got %q", got)
	}
}

// Ensure cobra is used (for command traversal in tests above).
var _ *cobra.Command
