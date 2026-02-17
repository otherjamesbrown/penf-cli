// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const (
	// FeedbackGitHubRepo is the repository for feedback issues.
	FeedbackGitHubRepo = "otherjamesbrown/penfold"
)

var (
	feedbackTitle   string
	feedbackDryRun  bool
	feedbackContext string
)

// NewFeedbackCommand creates the feedback command.
func NewFeedbackCommand(currentVersion string) *cobra.Command {
	feedbackCmd := &cobra.Command{
		Use:   "feedback",
		Short: "Submit feedback or report issues",
		Long: `Submit feedback, bug reports, or feature requests for penf.

This command creates GitHub issues directly in the penfold repository.
Requires the 'gh' CLI to be installed and authenticated.

Examples:
  penf feedback bug "Search results are not sorted correctly"
  penf feedback feature "Add support for Slack integration"
  penf feedback bug --title "Search bug" "Results not sorted"`,
	}

	// Bug subcommand.
	bugCmd := &cobra.Command{
		Use:   "bug <description>",
		Short: "Report a bug",
		Long: `Report a bug in the penf CLI.

The bug report will include:
- Your description of the issue
- CLI version and platform information
- Last command output (if available)

Examples:
  penf feedback bug "Search crashes when query contains special characters"
  penf feedback bug --title "Search crash" "Crashes with special chars"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFeedback("bug", currentVersion, args)
		},
	}
	bugCmd.Flags().StringVarP(&feedbackTitle, "title", "t", "", "Custom issue title")
	bugCmd.Flags().BoolVar(&feedbackDryRun, "dry-run", false, "Show what would be submitted without creating issue")
	bugCmd.Flags().StringVar(&feedbackContext, "context", "", "Additional context or error output")

	// Feature subcommand.
	featureCmd := &cobra.Command{
		Use:   "feature <description>",
		Short: "Request a feature",
		Long: `Request a new feature for the penf CLI.

Examples:
  penf feedback feature "Add support for importing Notion pages"
  penf feedback feature --title "Notion import" "Import pages from Notion"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFeedback("feature", currentVersion, args)
		},
	}
	featureCmd.Flags().StringVarP(&feedbackTitle, "title", "t", "", "Custom issue title")
	featureCmd.Flags().BoolVar(&feedbackDryRun, "dry-run", false, "Show what would be submitted without creating issue")

	feedbackCmd.AddCommand(bugCmd)
	feedbackCmd.AddCommand(featureCmd)

	return feedbackCmd
}

func runFeedback(feedbackType, currentVersion string, args []string) error {
	description := strings.Join(args, " ")

	// Determine title.
	title := feedbackTitle
	if title == "" {
		// Generate title from description.
		title = truncateFeedbackTitle(description, 80)
		if feedbackType == "bug" {
			title = "[Bug] " + title
		} else {
			title = "[Feature] " + title
		}
	}

	// Build issue body.
	body := buildIssueBody(feedbackType, description, currentVersion)

	// Determine labels.
	var labels []string
	if feedbackType == "bug" {
		labels = []string{"bug", "penfold-client"}
	} else {
		labels = []string{"enhancement", "penfold-client"}
	}

	// Show preview in dry-run mode.
	if feedbackDryRun {
		fmt.Println("Issue Preview (dry-run mode):")
		fmt.Println("=============================")
		fmt.Printf("Title: %s\n", title)
		fmt.Printf("Labels: %s\n", strings.Join(labels, ", "))
		fmt.Println()
		fmt.Println("Body:")
		fmt.Println(body)
		return nil
	}

	// Check if gh CLI is available.
	if !isGhCliAvailable() {
		return fmt.Errorf("the 'gh' CLI is required but not found or not authenticated\n\n" +
			"Install it from: https://cli.github.com/\n" +
			"Then run: gh auth login")
	}

	// Create GitHub issue using gh CLI.
	fmt.Println("Creating GitHub issue...")
	issueURL, err := createGitHubIssue(title, body, labels)
	if err != nil {
		return fmt.Errorf("creating issue: %w", err)
	}

	fmt.Printf("\n\033[32m✓\033[0m Issue created successfully!\n")
	fmt.Printf("  %s\n", issueURL)

	return nil
}

// buildIssueBody constructs the GitHub issue body.
func buildIssueBody(feedbackType, description, version string) string {
	var sb strings.Builder

	if feedbackType == "bug" {
		sb.WriteString("## Bug Report\n\n")
	} else {
		sb.WriteString("## Feature Request\n\n")
	}

	sb.WriteString("### Description\n\n")
	sb.WriteString(description)
	sb.WriteString("\n\n")

	if feedbackContext != "" {
		sb.WriteString("### Additional Context\n\n")
		sb.WriteString("```\n")
		sb.WriteString(feedbackContext)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("### Environment\n\n")
	sb.WriteString(fmt.Sprintf("- **penf version:** %s\n", version))
	sb.WriteString(fmt.Sprintf("- **OS/Platform:** %s/%s\n", runtime.GOOS, runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("- **Go version:** %s\n", runtime.Version()))

	sb.WriteString("\n---\n")
	sb.WriteString("*Submitted via `penf feedback`*\n")

	return sb.String()
}

// isGhCliAvailable checks if the gh CLI is installed and authenticated.
func isGhCliAvailable() bool {
	// Check if gh is in PATH.
	_, err := exec.LookPath("gh")
	if err != nil {
		return false
	}

	// Check if gh is authenticated.
	cmd := exec.Command("gh", "auth", "status")
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

// createGitHubIssue creates a GitHub issue using the gh CLI.
func createGitHubIssue(title, body string, labels []string) (string, error) {
	args := []string{
		"issue", "create",
		"--repo", FeedbackGitHubRepo,
		"--title", title,
		"--body", body,
	}

	for _, label := range labels {
		args = append(args, "--label", label)
	}

	cmd := exec.Command("gh", args...)
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s", err, stderr.String())
	}

	// gh issue create outputs the issue URL.
	issueURL := strings.TrimSpace(stdout.String())
	return issueURL, nil
}

// truncateFeedbackTitle truncates a string to the specified length.
func truncateFeedbackTitle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
