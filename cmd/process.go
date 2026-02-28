// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	glossaryv1 "github.com/otherjamesbrown/penf-cli/api/proto/glossary/v1"
	questionsv1 "github.com/otherjamesbrown/penf-cli/api/proto/questions/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Process command flags.
var (
	processOutput        string
	processIncludeSource bool
	processSourceContext int
	processDryRun        bool
)

// ProcessCommandDeps holds the dependencies for process commands.
type ProcessCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultProcessDeps returns the default dependencies for production use.
func DefaultProcessDeps() *ProcessCommandDeps {
	return &ProcessCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// AcronymContext represents the full context needed for intelligent acronym processing.
type AcronymContext struct {
	Questions []AcronymQuestion `json:"questions"`
	Glossary  []GlossaryEntry   `json:"glossary"`
	Stats     AcronymStats      `json:"stats"`
	Workflow  WorkflowGuidance  `json:"workflow"`
}

// AcronymQuestion represents a single acronym question with context.
type AcronymQuestion struct {
	ID              int64  `json:"id"`
	Term            string `json:"term"`
	Question        string `json:"question"`
	Context         string `json:"context"`
	SourceReference string `json:"source_reference,omitempty"`
	SourceContent   string `json:"source_content,omitempty"`
	Priority        string `json:"priority"`
}

// GlossaryEntry represents an existing glossary term.
type GlossaryEntry struct {
	Term      string   `json:"term"`
	Expansion string   `json:"expansion"`
	Context   []string `json:"context,omitempty"`
}

// AcronymStats provides queue statistics.
type AcronymStats struct {
	TotalPending  int64            `json:"total_pending"`
	ByPriority    map[string]int64 `json:"by_priority"`
	ResolvedToday int64            `json:"resolved_today"`
}

// WorkflowGuidance provides decision-making guidance for Claude.
type WorkflowGuidance struct {
	Actions         []WorkflowAction `json:"actions"`
	AutoResolve     []string         `json:"auto_resolve_patterns"`
	NeedsHumanInput []string         `json:"needs_human_input"`
	BatchResolveCmd string           `json:"batch_resolve_command"`
}

// WorkflowAction describes an available action.
type WorkflowAction struct {
	Name        string `json:"name"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

// BatchResolveRequest represents a batch resolution request.
type BatchResolveRequest struct {
	Resolutions []Resolution `json:"resolutions"`
	Dismissals  []Dismissal  `json:"dismissals"`
}

// Resolution represents a single acronym resolution.
type Resolution struct {
	ID        int64  `json:"id"`
	Expansion string `json:"expansion"`
}

// Dismissal represents a dismissal with reason.
type Dismissal struct {
	ID     int64  `json:"id"`
	Reason string `json:"reason"`
}

// BatchResolveResult represents the result of batch operations.
type BatchResolveResult struct {
	Resolved  int      `json:"resolved"`
	Dismissed int      `json:"dismissed"`
	Errors    []string `json:"errors,omitempty"`
}

// NewProcessCommand creates the root process command with all subcommands.
func NewProcessCommand(deps *ProcessCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultProcessDeps()
	}

	cmd := &cobra.Command{
		Use:   "process",
		Short: "AI-native batch processing commands",
		Long: `AI-native batch processing commands for intelligent workflow execution.

These commands are designed for Claude and other AI assistants to efficiently
process Penfold workflows by providing complete context and batch operations.

Instead of executing one command at a time, Claude can:
1. Get full context with 'penf process <workflow> context'
2. Analyze all items intelligently
3. Execute batch operations with 'penf process <workflow> batch-<action>'

Available workflows:
  acronyms    Process unknown acronyms from content
  mentions    Process entity mentions needing resolution
  onboarding  Post-import guided review of all discovered entities

Example workflow:
  # Get all context for intelligent processing
  penf process acronyms context --output json
  penf process mentions context --output json
  penf process onboarding context --output json

  # Batch resolve multiple items
  penf process acronyms batch-resolve '{"resolutions":[...]}'
  penf process mentions batch-resolve '{"resolutions":[...]}'
  penf process onboarding batch '{"confirm_people":[...]}'

Workflows are documented in Context Palace knowledge shards.`,
	}

	// Add workflow subcommands.
	cmd.AddCommand(newProcessAcronymsCommand(deps))
	cmd.AddCommand(newProcessMentionsCommand(deps))
	cmd.AddCommand(newProcessOnboardingCommand(deps))

	return cmd
}

// newProcessAcronymsCommand creates the 'process acronyms' subcommand group.
func newProcessAcronymsCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acronyms",
		Short: "Process unknown acronyms intelligently",
		Long: `Process unknown acronyms found during content ingestion.

This workflow allows Claude to:
1. Get all pending acronym questions with their context
2. Access the existing glossary to check for duplicates
3. Batch resolve multiple acronyms efficiently

Acronym review workflow details are in Context Palace knowledge shards.`,
		Aliases: []string{"acro"},
	}

	cmd.AddCommand(newAcronymsContextCommand(deps))
	cmd.AddCommand(newAcronymsBatchResolveCommand(deps))

	return cmd
}

// newAcronymsContextCommand creates the 'process acronyms context' subcommand.
func newAcronymsContextCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Get full context for acronym processing",
		Long: `Get complete context needed for intelligent acronym processing.

Returns:
- All pending acronym questions with source snippets
- Current glossary terms (to check for duplicates)
- Queue statistics
- Workflow guidance (available actions, decision criteria)

This single command provides everything Claude needs to:
1. Categorize all questions (known tech terms, duplicates, needs input)
2. Prepare batch resolutions
3. Only ask the user about truly ambiguous items

Examples:
  penf process acronyms context
  penf process acronyms context --output json
  penf process acronyms context --include-source`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAcronymsContext(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&processOutput, "output", "o", "json", "Output format: json, yaml, text")
	cmd.Flags().BoolVar(&processIncludeSource, "include-source", false, "Include source content for each question")
	cmd.Flags().IntVar(&processSourceContext, "source-context", 500, "Characters of source context to include")

	return cmd
}

// newAcronymsBatchResolveCommand creates the 'process acronyms batch-resolve' subcommand.
func newAcronymsBatchResolveCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch-resolve <json>",
		Short: "Batch resolve and dismiss acronyms",
		Long: `Batch resolve and dismiss multiple acronyms in a single operation.

Accepts JSON with resolutions and dismissals:
{
  "resolutions": [
    {"id": 123, "expansion": "Technical Execution Review"},
    {"id": 456, "expansion": "Database as a Service"}
  ],
  "dismissals": [
    {"id": 789, "reason": "Already in glossary"},
    {"id": 101, "reason": "Not an acronym, speaker initials"}
  ]
}

Use --dry-run to preview changes without executing them.

Example:
  penf process acronyms batch-resolve '{"resolutions":[{"id":24,"expansion":"Minimum Viable Product"}]}'
  penf process acronyms batch-resolve --dry-run '{"resolutions":[...]}'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAcronymsBatchResolve(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().BoolVar(&processDryRun, "dry-run", false, "Preview changes without executing them")

	return cmd
}

// runAcronymsContext executes the context command.
func runAcronymsContext(ctx context.Context, deps *ProcessCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToProcessGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Fetch questions.
	tenantID := getTenantID()
	questionsClient := questionsv1.NewQuestionsServiceClient(conn)
	questionsResp, err := questionsClient.ListQuestions(ctx, &questionsv1.ListQuestionsRequest{
		Status:       questionsv1.QuestionStatus_QUESTION_STATUS_PENDING,
		QuestionType: questionsv1.QuestionType_QUESTION_TYPE_ACRONYM,
		Limit:        100, // Get all pending acronyms.
		TenantId:     &tenantID,
	})
	if err != nil {
		return fmt.Errorf("listing questions: %w", err)
	}

	// Convert questions to our format.
	questions := make([]AcronymQuestion, 0, len(questionsResp.Questions))
	for _, q := range questionsResp.Questions {
		aq := AcronymQuestion{
			ID:              q.Id,
			Term:            q.SuggestedTerm,
			Question:        q.Question,
			Context:         q.Context,
			SourceReference: q.SourceReference,
			Priority:        priorityToString(q.Priority),
		}

		// Optionally fetch source content.
		if processIncludeSource && q.SourceReference != "" {
			sourceResp, err := questionsClient.GetQuestionSource(ctx, &questionsv1.GetQuestionSourceRequest{
				QuestionId:   q.Id,
				ContextChars: int32(processSourceContext),
			})
			if err == nil && sourceResp.Source != nil {
				aq.SourceContent = sourceResp.Source.Content
			}
		}

		questions = append(questions, aq)
	}

	// Fetch glossary.
	glossaryClient := glossaryv1.NewGlossaryServiceClient(conn)
	glossaryResp, err := glossaryClient.ListTerms(ctx, &glossaryv1.ListTermsRequest{
		TenantId: tenantID,
		Limit:    500, // Get comprehensive glossary.
	})
	if err != nil {
		return fmt.Errorf("listing glossary: %w", err)
	}

	// Convert glossary to our format.
	glossary := make([]GlossaryEntry, 0, len(glossaryResp.Terms))
	for _, t := range glossaryResp.Terms {
		glossary = append(glossary, GlossaryEntry{
			Term:      t.Term,
			Expansion: t.Expansion,
			Context:   t.Context,
		})
	}

	// Fetch stats.
	statsResp, err := questionsClient.GetQueueStats(ctx, &questionsv1.GetQueueStatsRequest{
		TenantId: &tenantID,
	})
	if err != nil {
		return fmt.Errorf("getting stats: %w", err)
	}

	stats := AcronymStats{
		TotalPending:  statsResp.Stats.TotalPending,
		ByPriority:    statsResp.Stats.ByPriority,
		ResolvedToday: statsResp.Stats.ResolvedToday,
	}

	// Build workflow guidance.
	workflow := WorkflowGuidance{
		Actions: []WorkflowAction{
			{
				Name:        "resolve",
				Command:     "penf review questions resolve <id> \"<expansion>\"",
				Description: "Add acronym expansion to glossary",
			},
			{
				Name:        "dismiss",
				Command:     "penf review questions dismiss <id> \"<reason>\"",
				Description: "Dismiss without adding to glossary",
			},
			{
				Name:        "defer",
				Command:     "penf review questions defer <id>",
				Description: "Defer for later review",
			},
			{
				Name:        "source",
				Command:     "penf review questions source <id> --context 1500",
				Description: "Get more source context",
			},
		},
		AutoResolve: []string{
			"Standard tech: REST, API, HTTP, JSON, YAML, URL, DNS, CDN, SSL, TLS, WebRTC",
			"Development: MVP, POC, SDK, IDE, CLI, CI/CD, TDD, OOP, DRY, CRUD, MVC",
			"Cloud: AWS, GCP, Azure, K8s, VM, VPC, IAM, S3, EC2, RDS, ECS, Lambda",
			"Database: SQL, NoSQL, RDBMS, ORM, ACID, ETL, CDC",
			"Business: ROI, KPI, OKR, SLA, NDA, B2B, B2C, CRM, ERP",
		},
		NeedsHumanInput: []string{
			"Domain-specific acronyms unknown to Claude",
			"Ambiguous terms with multiple meanings",
			"Potential speech-to-text errors",
			"Person initials vs acronyms",
		},
		BatchResolveCmd: "penf process acronyms batch-resolve '<json>'",
	}

	// Build complete context.
	result := AcronymContext{
		Questions: questions,
		Glossary:  glossary,
		Stats:     stats,
		Workflow:  workflow,
	}

	// Output.
	return outputAcronymContext(processOutput, result)
}

// runAcronymsBatchResolve executes the batch-resolve command.
func runAcronymsBatchResolve(ctx context.Context, deps *ProcessCommandDeps, jsonInput string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Parse input JSON.
	var req BatchResolveRequest
	if err := json.Unmarshal([]byte(jsonInput), &req); err != nil {
		return fmt.Errorf("parsing JSON input: %w", err)
	}

	// Dry-run mode: preview changes without executing.
	if processDryRun {
		fmt.Println("\033[1m=== DRY RUN - No changes will be made ===\033[0m")
		fmt.Println()

		if len(req.Resolutions) > 0 {
			fmt.Printf("Would resolve %d acronyms:\n", len(req.Resolutions))
			for _, r := range req.Resolutions {
				fmt.Printf("  \033[32m#%d:\033[0m %s\n", r.ID, r.Expansion)
			}
			fmt.Println()
		}

		if len(req.Dismissals) > 0 {
			fmt.Printf("Would dismiss %d items:\n", len(req.Dismissals))
			for _, d := range req.Dismissals {
				fmt.Printf("  \033[33m#%d:\033[0m %s\n", d.ID, d.Reason)
			}
			fmt.Println()
		}

		fmt.Printf("Summary: %d resolutions, %d dismissals\n", len(req.Resolutions), len(req.Dismissals))
		fmt.Println("\n\033[2mRun without --dry-run to apply these changes.\033[0m")
		return nil
	}

	conn, err := connectToProcessGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	questionsClient := questionsv1.NewQuestionsServiceClient(conn)

	var result BatchResolveResult
	var errors []string

	// Process resolutions.
	for _, r := range req.Resolutions {
		_, err := questionsClient.ResolveQuestion(ctx, &questionsv1.ResolveQuestionRequest{
			Id:     r.ID,
			Answer: r.Expansion,
		})
		if err != nil {
			errors = append(errors, fmt.Sprintf("resolve %d: %v", r.ID, err))
		} else {
			result.Resolved++
			fmt.Printf("\033[32mResolved #%d:\033[0m %s\n", r.ID, r.Expansion)
		}
	}

	// Process dismissals.
	for _, d := range req.Dismissals {
		_, err := questionsClient.DismissQuestion(ctx, &questionsv1.DismissQuestionRequest{
			Id:     d.ID,
			Reason: d.Reason,
		})
		if err != nil {
			errors = append(errors, fmt.Sprintf("dismiss %d: %v", d.ID, err))
		} else {
			result.Dismissed++
			fmt.Printf("\033[33mDismissed #%d:\033[0m %s\n", d.ID, d.Reason)
		}
	}

	result.Errors = errors

	// Summary.
	fmt.Println()
	fmt.Printf("Batch complete: %d resolved, %d dismissed", result.Resolved, result.Dismissed)
	if len(errors) > 0 {
		fmt.Printf(", %d errors\n", len(errors))
		for _, e := range errors {
			fmt.Printf("  \033[31mError:\033[0m %s\n", e)
		}
	} else {
		fmt.Println()
	}

	return nil
}

// connectToProcessGateway creates a gRPC connection to the gateway.
func connectToProcessGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if cfg.TLS.Enabled {
		tlsConfig, err := client.LoadClientTLSConfig(&cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("loading TLS config: %w", err)
		}
		if tlsConfig != nil {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, cfg.ServerAddress, opts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to gateway at %s: %w", cfg.ServerAddress, err)
	}

	return conn, nil
}

// outputAcronymContext outputs the context in the specified format.
func outputAcronymContext(format string, ctx AcronymContext) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(ctx)
	case "text":
		return outputAcronymContextText(ctx)
	default:
		// Default to JSON for AI consumption.
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(ctx)
	}
}

// outputAcronymContextText outputs context in human-readable format.
func outputAcronymContextText(ctx AcronymContext) error {
	fmt.Printf("Acronym Review Context\n")
	fmt.Printf("======================\n\n")

	fmt.Printf("Pending Questions: %d\n", len(ctx.Questions))
	fmt.Printf("Glossary Terms: %d\n", len(ctx.Glossary))
	fmt.Printf("Resolved Today: %d\n\n", ctx.Stats.ResolvedToday)

	if len(ctx.Questions) > 0 {
		fmt.Println("Questions:")
		for _, q := range ctx.Questions {
			fmt.Printf("  #%-4d [%s] %s\n", q.ID, q.Priority, q.Term)
			if q.Context != "" {
				fmt.Printf("         Context: \"%s\"\n", truncateProcessString(q.Context, 60))
			}
		}
		fmt.Println()
	}

	fmt.Println("Available Actions:")
	for _, a := range ctx.Workflow.Actions {
		fmt.Printf("  %s: %s\n", a.Name, a.Command)
	}
	fmt.Println()

	fmt.Println("Batch Command:")
	fmt.Printf("  %s\n", ctx.Workflow.BatchResolveCmd)

	return nil
}

// truncateProcessString truncates a string to max length for process output.
func truncateProcessString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
