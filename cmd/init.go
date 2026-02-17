// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/otherjamesbrown/penf-cli/config"
)

//go:embed templates/preferences.md
var preferencesTemplate string

//go:embed templates/processes.md
var processesTemplate string

//go:embed templates/acronym-review.md
var acronymReviewTemplate string

var (
	initServerAddr     string
	initNonInteractive bool
)

// NewInitCommand creates the init command.
func NewInitCommand() *cobra.Command {
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize penf configuration",
		Long: `Initialize penf for first-time use.

This command will:
1. Prompt for the gateway server address
2. Create ~/.penf/config.yaml (global config)
3. Test the connection to the gateway
4. Create CLAUDE.md in current directory (for Claude Code)
5. Create preferences.md in current directory (user settings - never overwritten)
6. Install process definitions in current directory
7. Install documentation hierarchy for Claude agents

Run this from your project directory. Global config goes to ~/.penf/,
but context files (CLAUDE.md, preferences.md, processes/, docs/) are created
in the current directory so Claude Code can find them.

After init, run 'penf init entities' to seed known people, products, and glossary.

Documentation (installed to docs/):
  docs/assistant-rules.md   How Penfold (the AI) should operate
  docs/index.md             System overview and navigation
  docs/shared/vision.md     What Penfold is and why
  docs/shared/entities.md   Data model and relationships`,
		RunE: runInit,
	}

	initCmd.Flags().StringVar(&initServerAddr, "server", "", "Gateway server address (host:port)")
	initCmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "Skip prompts, use defaults or flags")

	// Add subcommands
	initCmd.AddCommand(NewInitEntitiesCommand())

	return initCmd
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("Penfold CLI Initialization")
	fmt.Println("==========================")
	fmt.Println()

	// Load existing config if present.
	existingCfg, _ := config.LoadConfig()
	cfg := config.DefaultConfig()

	// Step 1: Get server address.
	serverAddr := initServerAddr
	if serverAddr == "" && !initNonInteractive {
		defaultAddr := config.DefaultServerAddress
		if existingCfg != nil && existingCfg.ServerAddress != "" {
			defaultAddr = existingCfg.ServerAddress
		}

		serverAddr = promptWithDefault("Gateway server address", defaultAddr)
	} else if serverAddr == "" {
		serverAddr = config.DefaultServerAddress
		if existingCfg != nil && existingCfg.ServerAddress != "" {
			serverAddr = existingCfg.ServerAddress
		}
	}
	cfg.ServerAddress = serverAddr

	// Step 2: Preserve other settings from existing config.
	if existingCfg != nil {
		if existingCfg.TenantID != "" {
			cfg.TenantID = existingCfg.TenantID
		}
		if existingCfg.TenantAliases != nil {
			cfg.TenantAliases = existingCfg.TenantAliases
		}
	}

	// Step 3: Test connection.
	fmt.Println()
	fmt.Printf("Testing connection to %s...\n", serverAddr)

	if err := testGatewayConnection(serverAddr); err != nil {
		fmt.Printf("  \033[33mWarning:\033[0m Could not connect to gateway: %v\n", err)
		fmt.Println("  Configuration will be saved, but you may need to check your server address.")
		fmt.Println()
	} else {
		fmt.Printf("  \033[32m✓\033[0m Successfully connected to gateway\n")
		fmt.Println()
	}

	// Step 4: Save configuration.
	configPath, _ := config.ConfigPath()
	fmt.Printf("Saving configuration to %s...\n", configPath)

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("saving configuration: %w", err)
	}
	fmt.Printf("  \033[32m✓\033[0m Configuration saved\n")
	fmt.Println()

	// Step 5: Download/update assistant CLAUDE.md.
	fmt.Println("Updating assistant configuration...")
	cwd, _ := os.Getwd()
	if err := downloadAssistantClaudeMd(cfg); err != nil {
		fmt.Printf("  \033[33mWarning:\033[0m Could not download assistant CLAUDE.md: %v\n", err)
		fmt.Println("  You can manually create this file later or run 'penf update' to retry.")
	} else {
		fmt.Printf("  \033[32m✓\033[0m Assistant CLAUDE.md saved to %s\n", filepath.Join(cwd, "CLAUDE.md"))
	}
	fmt.Println()

	// Step 6: Create user preferences file (only if it doesn't exist).
	fmt.Println("Setting up user preferences...")
	if err := initUserPreferences(); err != nil {
		fmt.Printf("  \033[33mWarning:\033[0m Could not create preferences: %v\n", err)
	}
	fmt.Println()

	// Step 7: Create/update process definitions.
	fmt.Println("Installing process definitions...")
	if err := initProcessDefinitions(); err != nil {
		fmt.Printf("  \033[33mWarning:\033[0m Could not create process files: %v\n", err)
	}
	fmt.Println()

	// Step 8: Install documentation for Claude agents.
	fmt.Println("Installing documentation...")
	if err := initDocs(); err != nil {
		fmt.Printf("  \033[33mWarning:\033[0m Could not install docs: %v\n", err)
	}
	fmt.Println()

	// Step 9: Create memory directory for session logs.
	fmt.Println("Creating memory directory...")
	if err := initMemoryDir(); err != nil {
		fmt.Printf("  \033[33mWarning:\033[0m Could not create memory directory: %v\n", err)
	}
	fmt.Println()

	// Summary.
	fmt.Println("Initialization complete!")
	fmt.Println()
	fmt.Println("Configuration summary:")
	fmt.Printf("  Server address:  %s\n", cfg.ServerAddress)
	fmt.Printf("  Config file:     %s\n", configPath)
	fmt.Printf("  CLAUDE.md:       %s\n", filepath.Join(cwd, "CLAUDE.md"))
	fmt.Printf("  Preferences:     %s\n", filepath.Join(cwd, "preferences.md"))
	fmt.Printf("  Processes:       %s\n", filepath.Join(cwd, "processes/"))
	fmt.Printf("  Documentation:   %s\n", filepath.Join(cwd, "docs/"))
	fmt.Printf("  Memory:          %s\n", filepath.Join(cwd, "memory/"))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  • Edit preferences.md to customize your settings")
	fmt.Println("  • Run 'penf init entities' to seed known people, products, glossary")
	fmt.Println("  • Run 'penf status' to verify the connection")
	fmt.Println("  • Run 'penf health' to check system health")
	fmt.Println()

	return nil
}

// promptWithDefault prompts the user for input with a default value.
func promptWithDefault(prompt, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}

	return input
}

// testGatewayConnection tests the connection to the gateway.
func testGatewayConnection(serverAddr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.DialContext(ctx, serverAddr, opts...)
	if err != nil {
		return fmt.Errorf("connecting to gateway: %w", err)
	}
	defer conn.Close()

	return nil
}

// downloadAssistantClaudeMd creates the assistant CLAUDE.md in current directory.
func downloadAssistantClaudeMd(cfg *config.CLIConfig) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	claudeMdPath := filepath.Join(cwd, "CLAUDE.md")

	// Check if CLAUDE.md already exists - never overwrite
	if _, err := os.Stat(claudeMdPath); err == nil {
		fmt.Printf("  \033[32m✓\033[0m CLAUDE.md already exists (not modified)\n")
		return nil
	}

	// For now, create a default assistant CLAUDE.md.
	// In the future, this could fetch from the gateway or a central repository.
	content := generateAssistantClaudeMd(cfg)

	if err := os.WriteFile(claudeMdPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing CLAUDE.md: %w", err)
	}

	return nil
}

// initUserPreferences creates the preferences.md file if it doesn't exist.
// This file is NEVER overwritten - it belongs to the user.
func initUserPreferences() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	prefsPath := filepath.Join(cwd, "preferences.md")

	// Check if preferences already exist - never overwrite
	if _, err := os.Stat(prefsPath); err == nil {
		fmt.Printf("  \033[32m✓\033[0m preferences.md already exists (not modified)\n")
		return nil
	}

	// Create new preferences file
	if err := os.WriteFile(prefsPath, []byte(preferencesTemplate), 0644); err != nil {
		return fmt.Errorf("writing preferences.md: %w", err)
	}

	fmt.Printf("  \033[32m✓\033[0m Created preferences.md\n")
	fmt.Println("    Edit preferences.md to customize your settings")
	return nil
}

// initProcessDefinitions creates/updates process definition files in the current directory.
// These CAN be updated by penf init or penf update.
func initProcessDefinitions() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	processDir := filepath.Join(cwd, "processes")

	// Create processes directory
	if err := os.MkdirAll(processDir, 0755); err != nil {
		return fmt.Errorf("creating processes directory: %w", err)
	}

	// Write/update processes index
	indexPath := filepath.Join(cwd, "processes.md")
	if err := os.WriteFile(indexPath, []byte(processesTemplate), 0644); err != nil {
		return fmt.Errorf("writing processes.md: %w", err)
	}
	fmt.Printf("  \033[32m✓\033[0m Updated processes.md index\n")

	// Write/update acronym-review process
	acronymPath := filepath.Join(processDir, "acronym-review.md")
	if err := os.WriteFile(acronymPath, []byte(acronymReviewTemplate), 0644); err != nil {
		return fmt.Errorf("writing acronym-review.md: %w", err)
	}
	fmt.Printf("  \033[32m✓\033[0m Updated processes/acronym-review.md\n")

	return nil
}

// initDocs downloads documentation from GitHub for Claude agents.
// Downloads from context/client/ and context/shared/ in the penfold repo.
// Structure:
//
//	docs/           - Client docs (assistant-rules.md, index.md, concepts/, workflows/)
//	docs/shared/    - Shared docs (vision, entities, use-cases)
func initDocs() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	docsDir := filepath.Join(cwd, "docs")

	// Files to download from context/client/ -> docs/
	clientFiles := []string{
		"assistant-rules.md",
		"index.md",
		"preferences.md",
		"processes.md",
		"concepts/entities.md",
		"concepts/glossary.md",
		"concepts/mentions.md",
		"concepts/people.md",
		"concepts/products.md",
		"workflows/acronym-review.md",
		"workflows/init-entities.md",
		"workflows/mention-review.md",
		"workflows/onboarding.md",
	}

	// Files to download from context/shared/ -> docs/shared/
	sharedFiles := []string{
		"vision.md",
		"entities.md",
		"use-cases.md",
		"interaction-model.md",
	}

	baseURL := "https://raw.githubusercontent.com/otherjamesbrown/penfold/main/context"
	client := &http.Client{Timeout: 30 * time.Second}

	// Download client docs
	for _, file := range clientFiles {
		destPath := filepath.Join(docsDir, file)
		srcURL := fmt.Sprintf("%s/client/%s", baseURL, file)

		if err := downloadFile(client, srcURL, destPath); err != nil {
			return fmt.Errorf("downloading %s: %w", file, err)
		}
	}

	// Download shared docs
	sharedDir := filepath.Join(docsDir, "shared")
	for _, file := range sharedFiles {
		destPath := filepath.Join(sharedDir, file)
		srcURL := fmt.Sprintf("%s/shared/%s", baseURL, file)

		if err := downloadFile(client, srcURL, destPath); err != nil {
			return fmt.Errorf("downloading shared/%s: %w", file, err)
		}
	}

	fmt.Printf("  \033[32m✓\033[0m Downloaded docs/ from GitHub (concepts, workflows, shared)\n")
	fmt.Println("    Claude reads docs/assistant-rules.md for identity and operating principles")
	fmt.Println("    Shared docs (vision, entities, use-cases) are in docs/shared/")

	return nil
}

// downloadFile downloads a file from a URL and saves it to destPath.
func downloadFile(client *http.Client, url, destPath string) error {
	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return fmt.Errorf("file not found: %s", url)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// initMemoryDir creates the memory directory for session logs.
// This directory stores daily YYYY-MM-DD.md files for session continuity.
func initMemoryDir() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	memoryDir := filepath.Join(cwd, "memory")

	// Create memory directory if it doesn't exist
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return fmt.Errorf("creating memory directory: %w", err)
	}

	// Create a README.md to explain the directory
	readmePath := filepath.Join(memoryDir, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		readmeContent := `# Session Memory

This directory contains daily session logs for Penfold.

## File Format

Files are named ` + "`YYYY-MM-DD.md`" + ` (e.g., ` + "`2025-01-26.md`" + `).

## What Gets Logged

- What we worked on (tasks, investigations, reviews)
- Decisions made and why
- Context that matters for continuity
- Things to follow up on
- Open questions or blockers

## Session Continuity

When starting a new session, Penfold reads recent memory files to restore context.
This enables picking up mid-project: "last week we were reviewing the glossary, can we continue"

## Relationship to preferences.md

- **memory/**: Raw session logs, daily activity
- **preferences.md**: Curated learning, distilled insights

Periodically review memory files and update preferences.md with what's worth keeping.
`
		if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
			return fmt.Errorf("writing README.md: %w", err)
		}
	}

	fmt.Printf("  \033[32m✓\033[0m Created memory/ directory for session logs\n")
	fmt.Println("    Penfold will create YYYY-MM-DD.md files to track session context")

	return nil
}

// generateAssistantClaudeMd generates the assistant CLAUDE.md content.
// This is intentionally minimal - all real content lives in docs/assistant-rules.md.
func generateAssistantClaudeMd(cfg *config.CLIConfig) string {
	return fmt.Sprintf(`# Penfold Assistant

**Read `+"`docs/assistant-rules.md`"+` first** - it defines who you are and how to operate.

## Configuration

- **Server:** %s
- **Config:** ~/.penf/config.yaml
- **Docs:** docs/ (downloaded from GitHub)

## Quick Start

1. Read `+"`docs/assistant-rules.md`"+` - your identity and operating principles
2. Read `+"`docs/index.md`"+` - system overview and navigation
3. Check Agent Mail inbox for dev messages
4. Help the user

## Troubleshooting

`+"```"+`bash
penf status    # Check connection
penf health    # View system health
penf update    # Update CLI and docs
`+"```"+`
`, cfg.ServerAddress)
}
