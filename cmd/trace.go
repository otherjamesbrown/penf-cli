// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/config"
	"github.com/otherjamesbrown/penf-cli/pkg/contentid"
)

// TraceOutput represents the output for the trace command.
type TraceOutput struct {
	ContentID   string `json:"content_id" yaml:"content_id"`
	ContentType string `json:"type" yaml:"type"`
	LangfuseURL string `json:"langfuse_url" yaml:"langfuse_url"`
}

// TraceCommandDeps holds the dependencies for trace commands.
type TraceCommandDeps struct {
	Config       *config.CLIConfig
	LoadConfig   func() (*config.CLIConfig, error)
	OutputFormat config.OutputFormat
	LangfuseHost string // Langfuse server host (default: dev02.brown.chat:3000)
}

// DefaultTraceDeps returns the default dependencies for production use.
func DefaultTraceDeps() *TraceCommandDeps {
	return &TraceCommandDeps{
		LoadConfig:   config.LoadConfig,
		LangfuseHost: "dev02.brown.chat:3000",
	}
}

// Trace command flags.
var (
	traceOutput string
)

// NewTraceCommand creates the trace command.
func NewTraceCommand(deps *TraceCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultTraceDeps()
	}

	cmd := &cobra.Command{
		Use:   "trace <content_id>",
		Short: "Find Langfuse traces for a content item",
		Long: `Look up Langfuse observability traces for a content item by its content ID.

This command generates a Langfuse URL that filters traces by the penfold.content_id
metadata field, allowing you to view all AI processing traces related to a specific
piece of content (email, meeting, document, etc.).

Content ID Format:
  Content IDs follow the format: <type>-<timestamp><random>
  Where type is one of:
    - em = email
    - mt = meeting
    - dc = document
    - tr = transcript
    - at = attachment

Use this command when:
  - Debugging AI processing issues for specific content
  - Investigating extraction quality or errors
  - Analyzing the AI pipeline behavior for a content item

Examples:
  # Look up traces for a document
  penf trace dc-9x3kp7mn

  # Look up traces for an email
  penf trace em-abc12345

  # Output as JSON for programmatic use
  penf trace dc-9x3kp7mn -o json

Documentation:
  Pipeline concepts:   docs/concepts/pipeline.md
  Entity model:        docs/shared/entities.md

Related Commands:
  penf content trace <id>    Processing timeline for a content item
  penf pipeline history <id> Pipeline execution stages for a source
  penf audit traces          Mention resolution decision traces`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTrace(cmd, deps, args[0])
		},
	}

	cmd.Flags().StringVarP(&traceOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// runTrace executes the trace command.
func runTrace(cmd *cobra.Command, deps *TraceCommandDeps, contentID string) error {
	// Load configuration.
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format.
	outputFormat := cfg.OutputFormat
	if traceOutput != "" {
		outputFormat = config.OutputFormat(traceOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s (must be text, json, or yaml)", traceOutput)
		}
	}

	// Validate content ID.
	parsed, err := contentid.Parse(contentID)
	if err != nil {
		return fmt.Errorf("invalid content ID %q: %w", contentID, err)
	}

	// Get content type name from the prefix.
	contentType := contentTypeFullName(parsed.Type)

	// Build Langfuse URL with filter.
	langfuseURL := buildLangfuseURL(deps.LangfuseHost, contentID)

	// Build output.
	output := TraceOutput{
		ContentID:   contentID,
		ContentType: contentType,
		LangfuseURL: langfuseURL,
	}

	// Output results.
	return outputTraceResults(outputFormat, output)
}

// contentTypeFullName converts a content type prefix to its full name.
func contentTypeFullName(prefix string) string {
	names := map[string]string{
		contentid.TypeEmail:      "email",
		contentid.TypeMeeting:    "meeting",
		contentid.TypeDocument:   "document",
		contentid.TypeTranscript: "transcript",
		contentid.TypeAttachment: "attachment",
	}

	if name, ok := names[prefix]; ok {
		return name
	}
	return prefix
}

// buildLangfuseURL constructs the Langfuse traces URL with the content ID filter.
func buildLangfuseURL(host, contentID string) string {
	// Build filter query parameter: penfold.content_id=<content_id>
	filter := fmt.Sprintf("penfold.content_id=%s", contentID)

	// URL encode the filter.
	encodedFilter := url.QueryEscape(filter)

	return fmt.Sprintf("https://%s/traces?filter=%s", host, encodedFilter)
}

// outputTraceResults formats and outputs trace results.
func outputTraceResults(format config.OutputFormat, output TraceOutput) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(output)
	default:
		return outputTraceResultsText(output)
	}
}

// outputTraceResultsText formats trace results for terminal display.
func outputTraceResultsText(output TraceOutput) error {
	fmt.Printf("Content: %s\n", output.ContentID)
	fmt.Printf("Type:    %s\n", output.ContentType)
	fmt.Println()
	fmt.Println("Langfuse Traces:")
	fmt.Printf("  %s\n", output.LangfuseURL)
	return nil
}
