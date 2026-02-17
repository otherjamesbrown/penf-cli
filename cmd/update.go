// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/penf-cli/config"
)

const (
	// GitHubOwner is the GitHub repository owner.
	GitHubOwner = "otherjamesbrown"
	// GitHubRepo is the GitHub repository name.
	GitHubRepo = "penfold"
	// GitHubReleasesAPI is the GitHub API URL for releases.
	GitHubReleasesAPI = "https://api.github.com/repos/%s/%s/releases/latest"
)

var (
	updateCheck       bool
	updateForce       bool
	updateVersion     string
	updateInstallPath string
)

// GitHubRelease represents a GitHub release from the API.
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a release asset.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

// NewUpdateCommand creates the update command.
func NewUpdateCommand(currentVersion string) *cobra.Command {
	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update penf to the latest version",
		Long: `Update penf CLI to the latest version from GitHub releases.

This command will:
1. Check GitHub for the latest release
2. Download the binary for your platform
3. Replace the current binary
4. Update the assistant CLAUDE.md configuration
5. Update process definitions (~/.penf/processes.md)
6. Update documentation for Claude agents (docs/)

Examples:
  penf update           # Update to latest version
  penf update --check   # Check for updates without installing
  penf update --force   # Force reinstall current version`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(currentVersion)
		},
	}

	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "Check for updates without installing")
	updateCmd.Flags().BoolVar(&updateForce, "force", false, "Force update even if already at latest version")
	updateCmd.Flags().StringVar(&updateVersion, "version", "", "Update to specific version (e.g., v1.0.0)")
	updateCmd.Flags().StringVar(&updateInstallPath, "install-path", "", "Install to this path (default: current location or config install_path)")

	return updateCmd
}

func runUpdate(currentVersion string) error {
	fmt.Printf("Current version: %s\n", currentVersion)
	fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	// Get latest release info.
	fmt.Println("Checking for updates...")
	release, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	latestVersion := release.TagName
	fmt.Printf("Latest version: %s\n", latestVersion)
	fmt.Println()

	// Compare versions.
	isNewer := isNewerVersion(currentVersion, latestVersion)
	if !isNewer && !updateForce {
		fmt.Println("You are already running the latest version.")
		return nil
	}

	// Check-only mode.
	if updateCheck {
		if isNewer {
			fmt.Printf("Update available: %s -> %s\n", currentVersion, latestVersion)
			fmt.Println()
			if release.Body != "" {
				fmt.Println("Release notes:")
				fmt.Println(formatReleaseNotes(release.Body))
			}
		}
		return nil
	}

	// Find the appropriate asset for this platform.
	assetName := getAssetName()
	var downloadURL string
	var assetSize int64
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			assetSize = asset.Size
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no release asset found for platform %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}

	// Determine install path (flag > config > current executable).
	var installPath string
	if updateInstallPath != "" {
		// Use flag value.
		installPath, err = config.ExpandPath(updateInstallPath)
		if err != nil {
			return fmt.Errorf("expanding install path: %w", err)
		}
	} else {
		// Use config or current executable.
		cfg, _ := config.LoadConfig()
		if cfg == nil {
			cfg = config.DefaultConfig()
		}
		installPath, err = cfg.GetInstallPath()
		if err != nil {
			return fmt.Errorf("determining install path: %w", err)
		}
	}

	// Check if target directory is writable.
	installDir := filepath.Dir(installPath)
	if err := checkDirWritable(installDir); err != nil {
		return fmt.Errorf("cannot write to %s: %w\n\nTry one of:\n  1. sudo penf update\n  2. penf update --install-path ~/bin/penf\n  3. penf config set install_path ~/bin/penf", installDir, err)
	}

	// Download the new binary.
	fmt.Printf("Downloading %s (%.2f MB)...\n", assetName, float64(assetSize)/(1024*1024))
	tempFile, err := downloadAsset(downloadURL)
	if err != nil {
		return fmt.Errorf("downloading update: %w", err)
	}
	defer os.Remove(tempFile)

	// Extract and install.
	fmt.Printf("Installing to %s...\n", installPath)
	if err := installUpdate(tempFile, installPath); err != nil {
		return fmt.Errorf("installing update: %w", err)
	}
	fmt.Printf("  \033[32m✓\033[0m Updated to %s\n", latestVersion)

	// Update assistant CLAUDE.md in current directory.
	fmt.Println("Updating assistant configuration...")
	cfg, _ := config.LoadConfig()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	cwd, _ := os.Getwd()
	claudeMdPath := filepath.Join(cwd, "CLAUDE.md")
	// Inline protection: Check if CLAUDE.md already exists before calling downloadAssistantClaudeMd.
	// This provides "belt and suspenders" protection for self-updating binaries:
	// Even if updating FROM an old version that lacks function-level protection,
	// this inline check ensures we never overwrite the user's CLAUDE.md.
	if _, err := os.Stat(claudeMdPath); err == nil {
		fmt.Printf("  \033[32m✓\033[0m CLAUDE.md already exists (not modified)\n")
	} else if err := downloadAssistantClaudeMd(cfg); err != nil {
		fmt.Printf("  \033[33mWarning:\033[0m Could not update assistant CLAUDE.md: %v\n", err)
	} else {
		fmt.Printf("  \033[32m✓\033[0m Assistant CLAUDE.md updated in %s\n", cwd)
	}

	// Update process definitions.
	fmt.Println("Updating process definitions...")
	if err := initProcessDefinitions(); err != nil {
		fmt.Printf("  \033[33mWarning:\033[0m Could not update process definitions: %v\n", err)
	}

	// Update documentation for Claude agents.
	fmt.Println("Updating documentation...")
	if err := initDocs(); err != nil {
		fmt.Printf("  \033[33mWarning:\033[0m Could not update docs: %v\n", err)
	} else {
		fmt.Printf("  \033[32m✓\033[0m Documentation updated in %s/docs/\n", cwd)
	}

	// Create preferences.md if it doesn't exist (never overwrites user's file).
	if err := initUserPreferences(); err != nil {
		fmt.Printf("  \033[33mWarning:\033[0m Could not initialize preferences: %v\n", err)
	}

	fmt.Println()

	// Show release notes.
	if release.Body != "" {
		fmt.Println("What's new:")
		fmt.Println(formatReleaseNotes(release.Body))
	}

	return nil
}

// getLatestRelease fetches the latest release from GitHub API.
func getLatestRelease() (*GitHubRelease, error) {
	url := fmt.Sprintf(GitHubReleasesAPI, GitHubOwner, GitHubRepo)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "penf-cli")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("no releases found")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release info: %w", err)
	}

	return &release, nil
}

// getAssetName returns the expected asset filename for the current platform.
func getAssetName() string {
	goos := runtime.GOOS
	arch := runtime.GOARCH

	// Asset naming convention: penf-<os>-<arch>
	return fmt.Sprintf("penf-%s-%s", goos, arch)
}

// downloadAsset downloads a release asset to a temporary file.
func downloadAsset(url string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tempFile, err := os.CreateTemp("", "penf-update-*")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// installUpdate installs the new binary.
func installUpdate(downloadedPath, targetPath string) error {
	// Make the downloaded file executable.
	if err := os.Chmod(downloadedPath, 0755); err != nil {
		return fmt.Errorf("making binary executable: %w", err)
	}

	// Replace the old binary.
	// On Unix, we can rename over the running binary.
	if err := os.Rename(downloadedPath, targetPath); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}

	return nil
}

// isNewerVersion compares version strings (simple comparison).
func isNewerVersion(current, latest string) bool {
	// Normalize versions (remove 'v' prefix).
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	// Handle "dev" version.
	if current == "dev" || current == "unknown" {
		return true
	}

	return latest > current
}

// formatReleaseNotes formats release notes for terminal display.
func formatReleaseNotes(body string) string {
	lines := strings.Split(body, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, "  "+line)
	}
	return strings.Join(result, "\n")
}

// checkDirWritable checks if a directory is writable by creating a temp file.
func checkDirWritable(dir string) error {
	// Check if directory exists.
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("directory does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory")
	}

	// Try to create a temp file to check write permission.
	tempFile, err := os.CreateTemp(dir, ".penf-write-test-*")
	if err != nil {
		return fmt.Errorf("permission denied")
	}
	tempFile.Close()
	os.Remove(tempFile.Name())
	return nil
}
