// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// AIResponse represents a response from an AI operation.
type AIResponse struct {
	ID          string            `json:"id" yaml:"id"`
	Operation   string            `json:"operation" yaml:"operation"`
	Query       string            `json:"query,omitempty" yaml:"query,omitempty"`
	ContentID   string            `json:"content_id,omitempty" yaml:"content_id,omitempty"`
	Response    string            `json:"response" yaml:"response"`
	Model       string            `json:"model" yaml:"model"`
	TokensUsed  int               `json:"tokens_used" yaml:"tokens_used"`
	LatencyMs   float64           `json:"latency_ms" yaml:"latency_ms"`
	Sources     []AISource        `json:"sources,omitempty" yaml:"sources,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CompletedAt time.Time         `json:"completed_at" yaml:"completed_at"`
}

// AISource represents a source document used in the AI response.
type AISource struct {
	ID          string  `json:"id" yaml:"id"`
	Title       string  `json:"title" yaml:"title"`
	ContentType string  `json:"content_type" yaml:"content_type"`
	Relevance   float64 `json:"relevance" yaml:"relevance"`
}

// AICommandDeps holds the dependencies for AI commands.
type AICommandDeps struct {
	Config       *config.CLIConfig
	GRPCClient   *client.GRPCClient
	OutputFormat config.OutputFormat
	LoadConfig   func() (*config.CLIConfig, error)
	InitClient   func(*config.CLIConfig) (*client.GRPCClient, error)
}

// DefaultAIDeps returns the default dependencies for production use.
func DefaultAIDeps() *AICommandDeps {
	return &AICommandDeps{
		LoadConfig: config.LoadConfig,
		InitClient: client.ConnectFromConfig,
	}
}

// AI command flags.
var (
	aiModel       string
	aiMaxTokens   int
	aiTemperature float64
	aiOutput      string
	aiVerbose     bool
	aiContext     int
)

// NewAICommand creates the root AI command with all subcommands.
func NewAICommand(deps *AICommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultAIDeps()
	}

	cmd := &cobra.Command{
		Use:   "ai",
		Short: "AI-powered operations on your knowledge base",
		Long: `AI-powered operations for querying, summarizing, and analyzing content.

The ai commands leverage Penfold's AI capabilities to help you understand
and interact with your knowledge base using natural language.

Commands:
  query     - Ask questions about your knowledge base
  summarize - Generate summaries of specific content
  analyze   - Perform deep analysis on content

Examples:
  # Ask a question about your knowledge base
  penf ai query "What were the key decisions from last week's meetings?"

  # Summarize a specific document
  penf ai summarize doc-123

  # Analyze content for insights
  penf ai analyze email-456

Documentation:
  System vision:       docs/shared/vision.md
  Entity model:        docs/shared/entities.md

When to Use:
  ai query     Synthesized answers from the knowledge base
  ai summarize Summary of a specific content item
  ai analyze   Deep analysis (sentiment, entities, action items)
  search       Find specific content items (use search, not ai, for browsing)
  briefing     Project-scoped priority assertions (not general Q&A)`,
	}

	// Add subcommands.
	cmd.AddCommand(newAIQueryCommand(deps))
	cmd.AddCommand(newAISummarizeCommand(deps))
	cmd.AddCommand(newAIAnalyzeCommand(deps))

	return cmd
}

// newAIQueryCommand creates the 'ai query' subcommand.
func newAIQueryCommand(deps *AICommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query <question>",
		Short: "Ask a question about your knowledge base",
		Long: `Ask a natural language question about your knowledge base.

The AI will search relevant content and provide an answer based on your data.
Sources are cited in the response to help you verify the information.

Examples:
  # Basic query
  penf ai query "What are our Q4 objectives?"

  # Query with more context
  penf ai query "Summarize the feedback from customer calls this month" --context=10

  # Use a specific model
  penf ai query "What's the status of Project Alpha?" --model=gpt-4

  # Get verbose output with token usage
  penf ai query "Who mentioned budget concerns?" --verbose`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAIQuery(cmd.Context(), deps, strings.Join(args, " "))
		},
	}

	// Define flags.
	cmd.Flags().StringVar(&aiModel, "model", "", "AI model to use (default: auto-selected)")
	cmd.Flags().IntVar(&aiMaxTokens, "max-tokens", 1000, "Maximum tokens in response")
	cmd.Flags().Float64Var(&aiTemperature, "temperature", 0.7, "Response creativity (0.0-1.0)")
	cmd.Flags().StringVarP(&aiOutput, "output", "o", "", "Output format: text, json, yaml")
	cmd.Flags().BoolVarP(&aiVerbose, "verbose", "v", false, "Show detailed information")
	cmd.Flags().IntVar(&aiContext, "context", 5, "Number of source documents to consider")

	return cmd
}

// newAISummarizeCommand creates the 'ai summarize' subcommand.
func newAISummarizeCommand(deps *AICommandDeps) *cobra.Command {
	var summaryLength string

	cmd := &cobra.Command{
		Use:   "summarize <content-id>",
		Short: "Generate a summary of specific content",
		Long: `Generate an AI-powered summary of a specific content item.

Supports different summary lengths and styles. The content can be any
type in your knowledge base: emails, documents, meeting notes, etc.

Examples:
  # Summarize a document
  penf ai summarize doc-123

  # Generate a brief summary
  penf ai summarize email-456 --length=brief

  # Generate a detailed summary
  penf ai summarize meeting-789 --length=detailed

  # Output as JSON
  penf ai summarize doc-123 --output=json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAISummarize(cmd.Context(), deps, args[0], summaryLength)
		},
	}

	// Define flags.
	cmd.Flags().StringVar(&summaryLength, "length", "standard", "Summary length: brief, standard, detailed")
	cmd.Flags().StringVar(&aiModel, "model", "", "AI model to use (default: auto-selected)")
	cmd.Flags().StringVarP(&aiOutput, "output", "o", "", "Output format: text, json, yaml")
	cmd.Flags().BoolVarP(&aiVerbose, "verbose", "v", false, "Show detailed information")

	return cmd
}

// newAIAnalyzeCommand creates the 'ai analyze' subcommand.
func newAIAnalyzeCommand(deps *AICommandDeps) *cobra.Command {
	var analyzeType string

	cmd := &cobra.Command{
		Use:   "analyze <content-id>",
		Short: "Perform deep analysis on content",
		Long: `Perform AI-powered deep analysis on a specific content item.

Analysis types:
  - sentiment:  Analyze emotional tone and sentiment
  - entities:   Extract key entities (people, places, organizations)
  - topics:     Identify main topics and themes
  - action:     Extract action items and tasks
  - full:       Comprehensive analysis (all of the above)

Examples:
  # Full analysis of a document
  penf ai analyze doc-123

  # Extract sentiment from an email thread
  penf ai analyze email-456 --type=sentiment

  # Find action items in meeting notes
  penf ai analyze meeting-789 --type=action

  # Extract entities from a document
  penf ai analyze doc-123 --type=entities`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAIAnalyze(cmd.Context(), deps, args[0], analyzeType)
		},
	}

	// Define flags.
	cmd.Flags().StringVarP(&analyzeType, "type", "t", "full", "Analysis type: sentiment, entities, topics, action, full")
	cmd.Flags().StringVar(&aiModel, "model", "", "AI model to use (default: auto-selected)")
	cmd.Flags().StringVarP(&aiOutput, "output", "o", "", "Output format: text, json, yaml")
	cmd.Flags().BoolVarP(&aiVerbose, "verbose", "v", false, "Show detailed information")

	return cmd
}

// runAIQuery executes the AI query command.
func runAIQuery(ctx context.Context, deps *AICommandDeps, question string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine output format.
	outputFormat := cfg.OutputFormat
	if aiOutput != "" {
		outputFormat = config.OutputFormat(aiOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", aiOutput)
		}
	}

	// Build client options with defaults for keepalive.
	clientOpts := client.DefaultOptions()
	clientOpts.Insecure = cfg.Insecure
	clientOpts.Debug = cfg.Debug
	clientOpts.TenantID = cfg.TenantID
	// Keep the default ConnectTimeout (10s) for fast failure detection.

	// Load TLS config if not in insecure mode.
	if !cfg.Insecure && cfg.TLS.Enabled {
		tlsConfig, err := client.LoadClientTLSConfig(&cfg.TLS)
		if err != nil {
			return fmt.Errorf("loading TLS config: %w", err)
		}
		clientOpts.TLSConfig = tlsConfig
	}

	// Connect to AI service via gateway.
	aiClient := client.NewAIClient(cfg.ServerAddress, clientOpts)

	connectCtx, cancel := context.WithTimeout(context.Background(), clientOpts.ConnectTimeout)
	defer cancel()
	if err := aiClient.Connect(connectCtx); err != nil {
		return fmt.Errorf("connecting to AI service: %w", err)
	}
	defer aiClient.Close()

	// Execute query via gRPC.
	queryResp, err := aiClient.Query(ctx, &client.QueryRequest{
		Question:     question,
		TenantID:     cfg.TenantID,
		ContextLimit: int32(aiContext),
		Model:        aiModel,
		MaxTokens:    int32(aiMaxTokens),
		Temperature:  float32(aiTemperature),
	})
	if err != nil {
		return fmt.Errorf("AI query failed: %w", err)
	}

	// Convert to response format.
	response := &AIResponse{
		ID:          queryResp.ResponseID,
		Operation:   "query",
		Query:       question,
		Response:    queryResp.Answer,
		Model:       queryResp.ModelUsed,
		TokensUsed:  int(queryResp.InputTokens + queryResp.OutputTokens),
		LatencyMs:   queryResp.LatencyMs,
		CompletedAt: time.Now(),
		Metadata: map[string]string{
			"context_docs": fmt.Sprintf("%d", aiContext),
			"max_tokens":   fmt.Sprintf("%d", aiMaxTokens),
		},
	}

	// Convert sources.
	for _, src := range queryResp.Sources {
		response.Sources = append(response.Sources, AISource{
			ID:          src.SourceID,
			Title:       src.Title,
			ContentType: src.ContentType,
			Relevance:   float64(src.Relevance),
		})
	}

	return outputAIResponse(outputFormat, response, aiVerbose)
}

// runAISummarize executes the AI summarize command.
func runAISummarize(ctx context.Context, deps *AICommandDeps, contentID, length string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Validate length.
	validLengths := map[string]bool{"brief": true, "standard": true, "detailed": true}
	if !validLengths[length] {
		return fmt.Errorf("invalid summary length: %s (must be brief, standard, or detailed)", length)
	}

	// Determine output format.
	outputFormat := cfg.OutputFormat
	if aiOutput != "" {
		outputFormat = config.OutputFormat(aiOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", aiOutput)
		}
	}

	// Build client options with defaults for keepalive.
	clientOpts := client.DefaultOptions()
	clientOpts.Insecure = cfg.Insecure
	clientOpts.Debug = cfg.Debug
	clientOpts.TenantID = cfg.TenantID
	// Keep the default ConnectTimeout (10s) for fast failure detection.

	// Load TLS config if not in insecure mode.
	if !cfg.Insecure && cfg.TLS.Enabled {
		tlsConfig, err := client.LoadClientTLSConfig(&cfg.TLS)
		if err != nil {
			return fmt.Errorf("loading TLS config: %w", err)
		}
		clientOpts.TLSConfig = tlsConfig
	}

	// Connect to AI service via gateway.
	aiClient := client.NewAIClient(cfg.ServerAddress, clientOpts)

	connectCtx, cancel := context.WithTimeout(context.Background(), clientOpts.ConnectTimeout)
	defer cancel()
	if err := aiClient.Connect(connectCtx); err != nil {
		return fmt.Errorf("connecting to AI service: %w", err)
	}
	defer aiClient.Close()

	// Execute summarize via gRPC.
	summaryResp, err := aiClient.Summarize(ctx, &client.SummarizeRequest{
		ContentID: contentID,
		TenantID:  cfg.TenantID,
		Length:    length,
		Model:     aiModel,
	})
	if err != nil {
		return fmt.Errorf("AI summarize failed: %w", err)
	}

	// Build response text with key points if available.
	responseText := summaryResp.Summary
	if len(summaryResp.KeyPoints) > 0 {
		responseText += "\n\n**Key Points:**\n"
		for _, point := range summaryResp.KeyPoints {
			responseText += fmt.Sprintf("- %s\n", point)
		}
	}

	// Convert to response format.
	response := &AIResponse{
		ID:          summaryResp.ResponseID,
		Operation:   "summarize",
		ContentID:   contentID,
		Response:    responseText,
		Model:       summaryResp.ModelUsed,
		TokensUsed:  int(summaryResp.InputTokens + summaryResp.OutputTokens),
		LatencyMs:   summaryResp.LatencyMs,
		CompletedAt: time.Now(),
		Metadata: map[string]string{
			"length":       length,
			"content_type": summaryResp.ContentType,
		},
	}

	return outputAIResponse(outputFormat, response, aiVerbose)
}

// runAIAnalyze executes the AI analyze command.
func runAIAnalyze(ctx context.Context, deps *AICommandDeps, contentID, analysisType string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Validate analysis type.
	validTypes := map[string]bool{
		"sentiment": true,
		"entities":  true,
		"topics":    true,
		"action":    true,
		"full":      true,
	}
	if !validTypes[analysisType] {
		return fmt.Errorf("invalid analysis type: %s (must be sentiment, entities, topics, action, or full)", analysisType)
	}

	// Determine output format.
	outputFormat := cfg.OutputFormat
	if aiOutput != "" {
		outputFormat = config.OutputFormat(aiOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s", aiOutput)
		}
	}

	// Build client options with defaults for keepalive.
	clientOpts := client.DefaultOptions()
	clientOpts.Insecure = cfg.Insecure
	clientOpts.Debug = cfg.Debug
	clientOpts.TenantID = cfg.TenantID
	// Keep the default ConnectTimeout (10s) for fast failure detection.

	// Load TLS config if not in insecure mode.
	if !cfg.Insecure && cfg.TLS.Enabled {
		tlsConfig, err := client.LoadClientTLSConfig(&cfg.TLS)
		if err != nil {
			return fmt.Errorf("loading TLS config: %w", err)
		}
		clientOpts.TLSConfig = tlsConfig
	}

	// Connect to AI service via gateway.
	aiClient := client.NewAIClient(cfg.ServerAddress, clientOpts)

	connectCtx, cancel := context.WithTimeout(context.Background(), clientOpts.ConnectTimeout)
	defer cancel()
	if err := aiClient.Connect(connectCtx); err != nil {
		return fmt.Errorf("connecting to AI service: %w", err)
	}
	defer aiClient.Close()

	// Execute analyze via gRPC.
	analyzeResp, err := aiClient.Analyze(ctx, &client.AnalyzeRequest{
		ContentID:    contentID,
		TenantID:     cfg.TenantID,
		AnalysisType: analysisType,
		Model:        aiModel,
	})
	if err != nil {
		return fmt.Errorf("AI analyze failed: %w", err)
	}

	// Build response text from analysis results.
	responseText := formatAnalysisResponse(analyzeResp, analysisType)

	// Convert to response format.
	response := &AIResponse{
		ID:          analyzeResp.ResponseID,
		Operation:   "analyze",
		ContentID:   contentID,
		Response:    responseText,
		Model:       analyzeResp.ModelUsed,
		TokensUsed:  int(analyzeResp.InputTokens + analyzeResp.OutputTokens),
		LatencyMs:   analyzeResp.LatencyMs,
		CompletedAt: time.Now(),
		Metadata: map[string]string{
			"analysis_type": analysisType,
			"content_type":  analyzeResp.ContentType,
		},
	}

	return outputAIResponse(outputFormat, response, aiVerbose)
}

// formatAnalysisResponse formats the analysis response for display.
func formatAnalysisResponse(resp *client.AnalyzeResponse, analysisType string) string {
	var sb strings.Builder

	// Always include summary
	sb.WriteString("**Summary**\n")
	sb.WriteString(resp.Summary)
	sb.WriteString("\n\n")

	// Include sentiment if available and relevant.
	if resp.Sentiment != nil && (analysisType == "sentiment" || analysisType == "full") {
		sb.WriteString("**Sentiment Analysis**\n")
		sb.WriteString(fmt.Sprintf("Overall Sentiment: %s (%.2f)\n", resp.Sentiment.Label, resp.Sentiment.Score))
		sb.WriteString(fmt.Sprintf("Confidence: %.0f%%\n", resp.Sentiment.Confidence*100))
		if len(resp.Sentiment.Indicators) > 0 {
			sb.WriteString("Key indicators: ")
			sb.WriteString(strings.Join(resp.Sentiment.Indicators, ", "))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Include entities if available and relevant.
	if len(resp.Entities) > 0 && (analysisType == "entities" || analysisType == "full") {
		sb.WriteString("**Entities Extracted**\n")
		// Group by type.
		byType := make(map[string][]client.ExtractedEntity)
		for _, e := range resp.Entities {
			byType[e.EntityType] = append(byType[e.EntityType], e)
		}
		for entityType, entities := range byType {
			sb.WriteString(fmt.Sprintf("\n%s (%d found):\n", strings.Title(entityType), len(entities)))
			for _, e := range entities {
				if e.Role != "" {
					sb.WriteString(fmt.Sprintf("  - %s (%s)\n", e.Name, e.Role))
				} else {
					sb.WriteString(fmt.Sprintf("  - %s\n", e.Name))
				}
			}
		}
		sb.WriteString("\n")
	}

	// Include topics if available and relevant.
	if len(resp.Topics) > 0 && (analysisType == "topics" || analysisType == "full") {
		sb.WriteString("**Topics Identified**\n")
		for _, t := range resp.Topics {
			sb.WriteString(fmt.Sprintf("  - %s (%.0f%% confidence)\n", t.Topic, t.Confidence*100))
			if len(t.Keywords) > 0 {
				sb.WriteString(fmt.Sprintf("    Keywords: %s\n", strings.Join(t.Keywords, ", ")))
			}
		}
		sb.WriteString("\n")
	}

	// Include action items if available and relevant.
	if len(resp.ActionItems) > 0 && (analysisType == "action" || analysisType == "full") {
		sb.WriteString("**Action Items**\n")
		for i, a := range resp.ActionItems {
			priority := a.Priority
			if priority == "" {
				priority = "medium"
			}
			sb.WriteString(fmt.Sprintf("  %d. [%s] %s\n", i+1, strings.ToUpper(priority), a.Description))
			if a.Assignee != "" {
				sb.WriteString(fmt.Sprintf("     Assignee: %s\n", a.Assignee))
			}
			if a.DueDate != "" {
				sb.WriteString(fmt.Sprintf("     Due: %s\n", a.DueDate))
			}
		}
		sb.WriteString("\n")
	}

	// Include insights if available.
	if len(resp.Insights) > 0 {
		sb.WriteString("**Insights & Recommendations**\n")
		for _, insight := range resp.Insights {
			sb.WriteString(fmt.Sprintf("  - %s\n", insight))
		}
	}

	return sb.String()
}

// outputAIResponse formats and outputs the AI response.
func outputAIResponse(format config.OutputFormat, response *AIResponse, verbose bool) error {
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(response)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(response)
	default:
		return outputAIResponseText(response, verbose)
	}
}

// outputAIResponseText formats AI response for terminal display.
func outputAIResponseText(response *AIResponse, verbose bool) error {
	// Header with operation info.
	fmt.Printf("\033[1mAI %s\033[0m", strings.Title(response.Operation))
	if response.Query != "" {
		fmt.Printf(": %s", response.Query)
	} else if response.ContentID != "" {
		fmt.Printf(" [%s]", response.ContentID)
	}
	fmt.Println()
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println()

	// Main response.
	fmt.Println(response.Response)
	fmt.Println()

	// Sources (if any).
	if len(response.Sources) > 0 {
		fmt.Println(strings.Repeat("-", 60))
		fmt.Println("\033[1mSources:\033[0m")
		for i, src := range response.Sources {
			fmt.Printf("  %d. %s (%s) - %.0f%% relevance\n",
				i+1, src.Title, src.ContentType, src.Relevance*100)
		}
		fmt.Println()
	}

	// Verbose info.
	if verbose {
		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("Model: %s\n", response.Model)
		fmt.Printf("Tokens: %d\n", response.TokensUsed)
		fmt.Printf("Latency: %.0fms\n", response.LatencyMs)
		if len(response.Metadata) > 0 {
			fmt.Print("Metadata: ")
			first := true
			for k, v := range response.Metadata {
				if !first {
					fmt.Print(", ")
				}
				fmt.Printf("%s=%s", k, v)
				first = false
			}
			fmt.Println()
		}
	}

	return nil
}
