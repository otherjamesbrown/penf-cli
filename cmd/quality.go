// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	qualityv1 "github.com/otherjamesbrown/penf-cli/api/proto/quality/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

// QualityCommandDeps holds dependencies for quality commands.
type QualityCommandDeps struct {
	Config       *config.CLIConfig
	OutputFormat config.OutputFormat
	LoadConfig   func() (*config.CLIConfig, error)
}

// QualitySummary represents aggregated quality metrics.
type QualitySummary struct {
	HighCount   int64 `json:"high_count" yaml:"high_count"`
	MediumCount int64 `json:"medium_count" yaml:"medium_count"`
	LowCount    int64 `json:"low_count" yaml:"low_count"`
}

// EntityQualityItem represents an entity with quality issues.
type EntityQualityItem struct {
	EntityID     int64          `json:"entity_id" yaml:"entity_id"`
	EntityName   string         `json:"entity_name" yaml:"entity_name"`
	PrimaryEmail string         `json:"primary_email" yaml:"primary_email"`
	Confidence   float32        `json:"confidence" yaml:"confidence"`
	Issues       []QualityIssue `json:"issues" yaml:"issues"`
}

// ExtractionQualityItem represents a content item with extraction quality.
type ExtractionQualityItem struct {
	ContentItemID   int64          `json:"content_item_id" yaml:"content_item_id"`
	ContentType     string         `json:"content_type" yaml:"content_type"`
	Subject         string         `json:"subject" yaml:"subject"`
	ExtractionScore float32        `json:"extraction_score" yaml:"extraction_score"`
	Issues          []QualityIssue `json:"issues" yaml:"issues"`
}

// QualityIssue represents a single quality issue.
type QualityIssue struct {
	Severity         string `json:"severity" yaml:"severity"`
	Category         string `json:"category" yaml:"category"`
	Description      string `json:"description" yaml:"description"`
	SuggestedCommand string `json:"suggested_command,omitempty" yaml:"suggested_command,omitempty"`
}

// DefaultQualityDeps returns the default dependencies for production use.
func DefaultQualityDeps() *QualityCommandDeps {
	return &QualityCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewQualityCommand creates the root quality command with all subcommands.
func NewQualityCommand(deps *QualityCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultQualityDeps()
	}

	cmd := &cobra.Command{
		Use:   "quality",
		Short: "Monitor data quality and identify issues",
		Long: `Monitor data quality and identify issues in the knowledge base.

The quality dashboard helps you identify entities and content items that need
attention, ranked by severity. Use these commands to proactively fix data
quality issues before they affect search results or insights.

Quality Issue Severity Levels:
  - HIGH:   Critical issues that significantly impact data quality
  - MEDIUM: Moderate issues that should be addressed soon
  - LOW:    Minor issues that can be addressed when convenient

Common Quality Issues:
  - Low confidence entity resolution
  - Incomplete extraction metadata
  - Missing or conflicting entity attributes
  - Content items with poor extraction scores

Documentation:
  Quality monitoring: docs/ops/quality-dashboard.md
  Entity resolution:  docs/concepts/entities.md`,
		Aliases: []string{"qual", "q"},
	}

	// Add subcommands.
	cmd.AddCommand(newQualitySummaryCommand(deps))
	cmd.AddCommand(newQualityEntitiesCommand(deps))
	cmd.AddCommand(newQualityExtractionsCommand(deps))

	return cmd
}

// newQualitySummaryCommand creates the 'quality summary' subcommand.
func newQualitySummaryCommand(deps *QualityCommandDeps) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Show quality issue summary by severity",
		Long: `Show aggregated quality issue counts by severity level.

Displays HIGH, MEDIUM, and LOW severity issue counts to give you a quick
overview of data quality. Use this command to check overall system health
and identify if there are critical issues requiring immediate attention.

Severity colors:
  - HIGH (red):    Critical issues
  - MEDIUM (yellow): Moderate issues
  - LOW (blue):    Minor issues

Examples:
  # Show quality summary
  penf quality summary

  # Output as JSON for processing
  penf quality summary -o json

  # Output as YAML
  penf quality summary -o yaml`,
		Example: `  # Show quality summary
  penf quality summary

  # Output as JSON
  penf quality summary -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQualitySummary(cmd.Context(), deps, outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newQualityEntitiesCommand creates the 'quality entities' subcommand.
func newQualityEntitiesCommand(deps *QualityCommandDeps) *cobra.Command {
	var (
		outputFormat string
		limit        int
	)

	cmd := &cobra.Command{
		Use:   "entities",
		Short: "List entities with quality issues",
		Long: `List entities that need attention, sorted by issue severity.

Shows entities with quality problems such as low confidence scores, missing
attributes, or conflicting data. Results are sorted with HIGH severity
issues first, followed by MEDIUM and LOW.

Each entity includes:
  - Entity ID and name
  - Primary email address
  - Confidence score
  - List of quality issues with suggested fix commands

Examples:
  # Show entities with quality issues
  penf quality entities

  # Limit to 10 results
  penf quality entities --limit 10

  # Output as JSON
  penf quality entities -o json`,
		Example: `  # Show entities needing attention
  penf quality entities

  # Limit results
  penf quality entities --limit 20`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQualityEntities(cmd.Context(), deps, outputFormat, limit)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format: text, json, yaml")
	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Maximum number of results")

	return cmd
}

// newQualityExtractionsCommand creates the 'quality extractions' subcommand.
func newQualityExtractionsCommand(deps *QualityCommandDeps) *cobra.Command {
	var (
		outputFormat string
		limit        int
	)

	cmd := &cobra.Command{
		Use:   "extractions",
		Short: "List content items with low extraction quality",
		Long: `List content items ranked by extraction quality score.

Shows content items (emails, meetings, documents) with poor extraction
quality. Items with lower scores appear first, indicating they may have
incomplete metadata, missing entities, or other extraction issues.

Each item includes:
  - Content item ID
  - Content type (email, meeting, etc.)
  - Subject/title
  - Extraction quality score (0.0 to 1.0)
  - List of quality issues with suggested fix commands

Examples:
  # Show content items with low extraction quality
  penf quality extractions

  # Limit to 20 results
  penf quality extractions --limit 20

  # Output as JSON
  penf quality extractions -o json`,
		Example: `  # Show extraction quality issues
  penf quality extractions

  # Limit results
  penf quality extractions --limit 30`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQualityExtractions(cmd.Context(), deps, outputFormat, limit)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format: text, json, yaml")
	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Maximum number of results")

	return cmd
}

// Command execution functions.

// runQualitySummary executes the quality summary command.
func runQualitySummary(ctx context.Context, deps *QualityCommandDeps, format string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Connect to gateway.
	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	qualityClient := qualityv1.NewQualityServiceClient(conn)

	// Call GetQualitySummary.
	req := &qualityv1.GetQualitySummaryRequest{
		TenantId: cfg.TenantID,
	}

	resp, err := qualityClient.GetQualitySummary(ctx, req)
	if err != nil {
		return fmt.Errorf("getting quality summary: %w", err)
	}

	// Convert to local type.
	summary := &QualitySummary{
		HighCount:   resp.GetHighCount(),
		MediumCount: resp.GetMediumCount(),
		LowCount:    resp.GetLowCount(),
	}

	// Determine output format.
	outFormat := cfg.OutputFormat
	if format != "" {
		outFormat = config.OutputFormat(format)
	}

	return outputQualitySummary(outFormat, summary)
}

// runQualityEntities executes the quality entities command.
func runQualityEntities(ctx context.Context, deps *QualityCommandDeps, format string, limit int) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Connect to gateway.
	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	qualityClient := qualityv1.NewQualityServiceClient(conn)

	// Call GetEntityQuality.
	req := &qualityv1.GetEntityQualityRequest{
		TenantId: cfg.TenantID,
		Limit:    int32(limit),
	}

	resp, err := qualityClient.GetEntityQuality(ctx, req)
	if err != nil {
		return fmt.Errorf("getting entity quality: %w", err)
	}

	// Convert to local types.
	entities := make([]EntityQualityItem, len(resp.GetItems()))
	for i, item := range resp.GetItems() {
		issues := make([]QualityIssue, len(item.GetIssues()))
		for j, issue := range item.GetIssues() {
			issues[j] = QualityIssue{
				Severity:         issue.GetSeverity(),
				Category:         issue.GetCategory(),
				Description:      issue.GetDescription(),
				SuggestedCommand: issue.GetSuggestedCommand(),
			}
		}
		entities[i] = EntityQualityItem{
			EntityID:     item.GetEntityId(),
			EntityName:   item.GetEntityName(),
			PrimaryEmail: item.GetPrimaryEmail(),
			Confidence:   item.GetConfidence(),
			Issues:       issues,
		}
	}

	// Sort by severity.
	entities = sortEntitiesBySeverity(entities)

	// Determine output format.
	outFormat := cfg.OutputFormat
	if format != "" {
		outFormat = config.OutputFormat(format)
	}

	return outputEntityQuality(outFormat, entities)
}

// runQualityExtractions executes the quality extractions command.
func runQualityExtractions(ctx context.Context, deps *QualityCommandDeps, format string, limit int) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Connect to gateway.
	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	qualityClient := qualityv1.NewQualityServiceClient(conn)

	// Call GetExtractionQuality.
	req := &qualityv1.GetExtractionQualityRequest{
		TenantId: cfg.TenantID,
		Limit:    int32(limit),
	}

	resp, err := qualityClient.GetExtractionQuality(ctx, req)
	if err != nil {
		return fmt.Errorf("getting extraction quality: %w", err)
	}

	// Convert to local types.
	extractions := make([]ExtractionQualityItem, len(resp.GetItems()))
	for i, item := range resp.GetItems() {
		issues := make([]QualityIssue, len(item.GetIssues()))
		for j, issue := range item.GetIssues() {
			issues[j] = QualityIssue{
				Severity:         issue.GetSeverity(),
				Category:         issue.GetCategory(),
				Description:      issue.GetDescription(),
				SuggestedCommand: issue.GetSuggestedCommand(),
			}
		}
		extractions[i] = ExtractionQualityItem{
			ContentItemID:   item.GetContentItemId(),
			ContentType:     item.GetContentType(),
			Subject:         item.GetSubject(),
			ExtractionScore: item.GetExtractionScore(),
			Issues:          issues,
		}
	}

	// Rank by score (lowest first).
	extractions = rankExtractionsByScore(extractions)

	// Determine output format.
	outFormat := cfg.OutputFormat
	if format != "" {
		outFormat = config.OutputFormat(format)
	}

	return outputExtractionQuality(outFormat, extractions)
}

// Output functions.

// outputQualitySummary outputs the quality summary in the specified format.
func outputQualitySummary(format config.OutputFormat, summary *QualitySummary) error {
	switch format {
	case config.OutputFormatJSON:
		return outputQualityJSON(summary)
	case config.OutputFormatYAML:
		return outputQualityYAML(summary)
	default:
		return outputQualitySummaryText(summary)
	}
}

// outputQualitySummaryText outputs quality summary in human-readable format.
func outputQualitySummaryText(summary *QualitySummary) error {
	fmt.Println("Quality Issue Summary:")
	fmt.Println()

	// HIGH severity (red)
	fmt.Printf("  \033[31mHIGH:\033[0m    %d issues\n", summary.HighCount)

	// MEDIUM severity (yellow)
	fmt.Printf("  \033[33mMEDIUM:\033[0m  %d issues\n", summary.MediumCount)

	// LOW severity (blue)
	fmt.Printf("  \033[34mLOW:\033[0m     %d issues\n", summary.LowCount)

	fmt.Println()

	// Total count.
	total := summary.HighCount + summary.MediumCount + summary.LowCount
	if total == 0 {
		fmt.Println("\033[32m✓ No quality issues detected.\033[0m")
	} else {
		fmt.Printf("Total: %d issues\n", total)
		if summary.HighCount > 0 {
			fmt.Println("\nUse 'penf quality entities' and 'penf quality extractions' to see details.")
		}
	}

	return nil
}

// outputEntityQuality outputs entity quality items in the specified format.
func outputEntityQuality(format config.OutputFormat, entities []EntityQualityItem) error {
	switch format {
	case config.OutputFormatJSON:
		return outputQualityJSON(entities)
	case config.OutputFormatYAML:
		return outputQualityYAML(entities)
	default:
		return outputEntityQualityText(entities)
	}
}

// outputEntityQualityText outputs entity quality in human-readable format.
func outputEntityQualityText(entities []EntityQualityItem) error {
	if len(entities) == 0 {
		fmt.Println("\033[32m✓ No entity quality issues found.\033[0m")
		return nil
	}

	fmt.Printf("Entities with Quality Issues (%d):\n\n", len(entities))

	for i, entity := range entities {
		if i > 0 {
			fmt.Println()
		}

		// Display entity info.
		fmt.Printf("  \033[1m%s\033[0m (ID: %d)\n", entity.EntityName, entity.EntityID)
		if entity.PrimaryEmail != "" {
			fmt.Printf("    Email: %s\n", entity.PrimaryEmail)
		}
		fmt.Printf("    Confidence: %.2f\n", entity.Confidence)

		// Display issues.
		for _, issue := range entity.Issues {
			severityColor := colorForSeverity(issue.Severity)
			fmt.Printf("    %s%s\033[0m: %s\n", severityColor, issue.Severity, issue.Description)
			if issue.SuggestedCommand != "" {
				fmt.Printf("      → %s\n", issue.SuggestedCommand)
			}
		}
	}

	return nil
}

// outputExtractionQuality outputs extraction quality items in the specified format.
func outputExtractionQuality(format config.OutputFormat, extractions []ExtractionQualityItem) error {
	switch format {
	case config.OutputFormatJSON:
		return outputQualityJSON(extractions)
	case config.OutputFormatYAML:
		return outputQualityYAML(extractions)
	default:
		return outputExtractionQualityText(extractions)
	}
}

// outputExtractionQualityText outputs extraction quality in human-readable format.
func outputExtractionQualityText(extractions []ExtractionQualityItem) error {
	if len(extractions) == 0 {
		fmt.Println("\033[32m✓ No extraction quality issues found.\033[0m")
		return nil
	}

	fmt.Printf("Content Items with Extraction Issues (%d):\n\n", len(extractions))

	for i, item := range extractions {
		if i > 0 {
			fmt.Println()
		}

		// Display content info.
		fmt.Printf("  \033[1m%s\033[0m (ID: %d)\n", truncateQualityString(item.Subject, 60), item.ContentItemID)
		fmt.Printf("    Type: %s\n", item.ContentType)
		fmt.Printf("    Extraction Score: %.2f\n", item.ExtractionScore)

		// Display issues.
		for _, issue := range item.Issues {
			severityColor := colorForSeverity(issue.Severity)
			fmt.Printf("    %s%s\033[0m: %s\n", severityColor, issue.Severity, issue.Description)
			if issue.SuggestedCommand != "" {
				fmt.Printf("      → %s\n", issue.SuggestedCommand)
			}
		}
	}

	return nil
}

// Helper functions.

// sortEntitiesBySeverity sorts entities by the highest severity of their issues.
func sortEntitiesBySeverity(entities []EntityQualityItem) []EntityQualityItem {
	// Create a copy to avoid modifying the input.
	sorted := make([]EntityQualityItem, len(entities))
	copy(sorted, entities)

	sort.SliceStable(sorted, func(i, j int) bool {
		return severityRank(highestSeverity(sorted[i].Issues)) < severityRank(highestSeverity(sorted[j].Issues))
	})

	return sorted
}

// rankExtractionsByScore ranks extractions by score (lowest first).
func rankExtractionsByScore(extractions []ExtractionQualityItem) []ExtractionQualityItem {
	// Create a copy to avoid modifying the input.
	ranked := make([]ExtractionQualityItem, len(extractions))
	copy(ranked, extractions)

	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].ExtractionScore < ranked[j].ExtractionScore
	})

	return ranked
}

// highestSeverity returns the highest severity from a list of issues.
func highestSeverity(issues []QualityIssue) string {
	if len(issues) == 0 {
		return "LOW"
	}

	for _, issue := range issues {
		if issue.Severity == "HIGH" {
			return "HIGH"
		}
	}
	for _, issue := range issues {
		if issue.Severity == "MEDIUM" {
			return "MEDIUM"
		}
	}
	return "LOW"
}

// severityRank returns a numeric rank for sorting (lower is more severe).
func severityRank(severity string) int {
	switch severity {
	case "HIGH":
		return 1
	case "MEDIUM":
		return 2
	case "LOW":
		return 3
	default:
		return 4
	}
}

// colorForSeverity returns ANSI color code for severity level.
func colorForSeverity(severity string) string {
	switch severity {
	case "HIGH":
		return "\033[31m" // Red
	case "MEDIUM":
		return "\033[33m" // Yellow
	case "LOW":
		return "\033[34m" // Blue
	default:
		return "\033[0m" // Reset
	}
}

// truncateQualityString truncates a string to maxLen characters.
func truncateQualityString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// outputQualityJSON outputs data as JSON.
func outputQualityJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// outputQualityYAML outputs data as YAML.
func outputQualityYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}
