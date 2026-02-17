// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"

	questionsv1 "github.com/otherjamesbrown/penf-cli/api/proto/questions/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Review questions command flags
var (
	questionsOutput   string
	questionsType     string
	questionsPriority string
	questionsLimit    int
)

// newReviewQuestionsCommand creates the 'review questions' subcommand group.
func newReviewQuestionsCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "questions",
		Short: "Manage AI questions requiring human review",
		Long: `Manage the queue of questions AI has for you.

During content processing, AI may encounter things it needs clarification on:
  - Unknown acronyms that need definitions
  - Ambiguous person references that need disambiguation
  - Entities that need confirmation

Questions are prioritized:
  - high:   Blocks understanding or processing
  - medium: Would improve search/correlation
  - low:    Nice to have, cosmetic improvement

Examples:
  penf review questions list              # Show pending questions
  penf review questions list --priority high
  penf review questions next              # Get next question to answer
  penf review questions resolve 123 "..."  # Answer a question
  penf review questions dismiss 123       # Dismiss a question

JSON Output (for AI processing):
  penf review questions list -o json

  Returns:
  {
    "questions": [
      {"id": 1, "type": "acronym", "term": "LKE", "priority": "high", "context": "..."}
    ],
    "total": 15
  }`,
		Aliases: []string{"q", "ask"},
	}

	// Add persistent flags
	cmd.PersistentFlags().StringVarP(&questionsOutput, "output", "o", "", "Output format: text, json, yaml")
	cmd.PersistentFlags().IntVarP(&questionsLimit, "limit", "l", 20, "Maximum number of results")

	// Add subcommands
	cmd.AddCommand(newQuestionsListCommand(deps))
	cmd.AddCommand(newQuestionsNextCommand(deps))
	cmd.AddCommand(newQuestionsShowCommand(deps))
	cmd.AddCommand(newQuestionsResolveCommand(deps))
	cmd.AddCommand(newQuestionsDismissCommand(deps))
	cmd.AddCommand(newQuestionsDeferCommand(deps))
	cmd.AddCommand(newQuestionsStatsCommand(deps))
	cmd.AddCommand(newQuestionsSourceCommand(deps))

	return cmd
}

// newQuestionsListCommand creates the 'review questions list' subcommand.
func newQuestionsListCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pending questions",
		Long: `List pending questions from AI that need human review.

Examples:
  penf review questions list
  penf review questions list --priority high
  penf review questions list --type acronym`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuestionsList(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&questionsPriority, "priority", "p", "", "Filter by priority: high, medium, low")
	cmd.Flags().StringVarP(&questionsType, "type", "t", "", "Filter by type: acronym, person, entity, duplicate")

	return cmd
}

// newQuestionsNextCommand creates the 'review questions next' subcommand.
func newQuestionsNextCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Show the next question to review",
		Long: `Show the next highest-priority question to review.

Returns the most urgent pending question. Use --type to filter by question type.

Example:
  penf review questions next
  penf review questions next --type acronym`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuestionsNext(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&questionsType, "type", "t", "", "Filter by type: acronym, person, entity")

	return cmd
}

// newQuestionsShowCommand creates the 'review questions show' subcommand.
func newQuestionsShowCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show details of a question",
		Long: `Show detailed information about a specific question.

Example:
  penf review questions show 123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid question ID: %s", args[0])
			}
			return runQuestionsShow(cmd.Context(), deps, id)
		},
	}
}

// newQuestionsResolveCommand creates the 'review questions resolve' subcommand.
func newQuestionsResolveCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve <id> <answer>",
		Short: "Resolve a question with an answer",
		Long: `Resolve a question by providing an answer.

For acronym questions, the answer is added to the glossary automatically.

Examples:
  penf review questions resolve 123 "Technical Execution Review"
  penf review questions resolve 456 "This refers to Adam Weingarten"`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid question ID: %s", args[0])
			}
			answer := strings.Join(args[1:], " ")
			return runQuestionsResolve(cmd.Context(), deps, id, answer)
		},
	}

	return cmd
}

// newQuestionsDismissCommand creates the 'review questions dismiss' subcommand.
func newQuestionsDismissCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "dismiss <id> [reason]",
		Short: "Dismiss a question as not needed",
		Long: `Dismiss a question without providing an answer.

Use this when the question isn't relevant or doesn't need an answer.

Example:
  penf review questions dismiss 123 "Not an acronym, just initials"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid question ID: %s", args[0])
			}
			reason := ""
			if len(args) > 1 {
				reason = strings.Join(args[1:], " ")
			}
			return runQuestionsDismiss(cmd.Context(), deps, id, reason)
		},
	}
}

// newQuestionsDeferCommand creates the 'review questions defer' subcommand.
func newQuestionsDeferCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "defer <id>",
		Short: "Defer a question for later",
		Long: `Defer a question to answer later.

The question remains in the queue but moves to deferred status.

Example:
  penf review questions defer 123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid question ID: %s", args[0])
			}
			return runQuestionsDefer(cmd.Context(), deps, id)
		},
	}
}

// newQuestionsStatsCommand creates the 'review questions stats' subcommand.
func newQuestionsStatsCommand(deps *ReviewCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show queue statistics",
		Long: `Show statistics about the questions queue.

Example:
  penf review questions stats`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuestionsStats(cmd.Context(), deps)
		},
	}
}

// Questions source command flag
var questionsSourceContextChars int

// newQuestionsSourceCommand creates the 'review questions source' subcommand.
func newQuestionsSourceCommand(deps *ReviewCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "source <id>",
		Short: "Show the source content for a question",
		Long: `Show the source content (transcript, email, etc.) where a question originated.

This allows you to see more context around the term or entity in question.

Examples:
  penf review questions source 123
  penf review questions source 123 --context 1000
  penf review questions source 123 --context -1  # Full content`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid question ID: %s", args[0])
			}
			return runQuestionsSource(cmd.Context(), deps, id)
		},
	}

	cmd.Flags().IntVarP(&questionsSourceContextChars, "context", "c", 500, "Characters of context around the snippet (-1 for full content)")

	return cmd
}

// connectToQuestionsGateway creates a gRPC connection to the gateway service.
func connectToQuestionsGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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

// Command execution functions

func runQuestionsList(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToQuestionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := questionsv1.NewQuestionsServiceClient(conn)
	tenantID := getTenantID()

	req := &questionsv1.ListQuestionsRequest{
		Status:   questionsv1.QuestionStatus_QUESTION_STATUS_PENDING,
		Limit:    int32(questionsLimit),
		TenantId: &tenantID,
	}

	if questionsPriority != "" {
		req.Priority = stringToPriority(questionsPriority)
	}
	if questionsType != "" {
		req.QuestionType = stringToQuestionType(questionsType)
	}

	resp, err := client.ListQuestions(ctx, req)
	if err != nil {
		return fmt.Errorf("listing questions: %w", err)
	}

	format := cfg.OutputFormat
	if questionsOutput != "" {
		format = config.OutputFormat(questionsOutput)
	}

	return outputProtoQuestionsList(format, resp.Questions)
}

func runQuestionsNext(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToQuestionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := questionsv1.NewQuestionsServiceClient(conn)
	tenantID := getTenantID()

	req := &questionsv1.GetNextQuestionRequest{
		TenantId: &tenantID,
	}
	if questionsType != "" {
		req.QuestionType = stringToQuestionType(questionsType)
	}

	resp, err := client.GetNextQuestion(ctx, req)
	if err != nil {
		return fmt.Errorf("getting next question: %w", err)
	}
	if resp.Question == nil {
		fmt.Println("No pending questions.")
		return nil
	}

	format := cfg.OutputFormat
	if questionsOutput != "" {
		format = config.OutputFormat(questionsOutput)
	}

	return outputProtoQuestionDetail(format, resp.Question)
}

func runQuestionsShow(ctx context.Context, deps *ReviewCommandDeps, id int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToQuestionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := questionsv1.NewQuestionsServiceClient(conn)

	resp, err := client.GetQuestion(ctx, &questionsv1.GetQuestionRequest{Id: id})
	if err != nil {
		return fmt.Errorf("getting question: %w", err)
	}
	if resp.Question == nil {
		return fmt.Errorf("question not found: %d", id)
	}

	format := cfg.OutputFormat
	if questionsOutput != "" {
		format = config.OutputFormat(questionsOutput)
	}

	return outputProtoQuestionDetail(format, resp.Question)
}

func runQuestionsResolve(ctx context.Context, deps *ReviewCommandDeps, id int64, answer string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToQuestionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := questionsv1.NewQuestionsServiceClient(conn)

	resp, err := client.ResolveQuestion(ctx, &questionsv1.ResolveQuestionRequest{
		Id:     id,
		Answer: answer,
	})
	if err != nil {
		return fmt.Errorf("resolving question: %w", err)
	}

	if resp.AddedToGlossary {
		fmt.Printf("Added to glossary: %s = %s\n", resp.Question.SuggestedTerm, answer)
	}

	fmt.Printf("\033[32mResolved question #%d\033[0m\n", id)
	return nil
}

func runQuestionsDismiss(ctx context.Context, deps *ReviewCommandDeps, id int64, reason string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToQuestionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := questionsv1.NewQuestionsServiceClient(conn)

	_, err = client.DismissQuestion(ctx, &questionsv1.DismissQuestionRequest{
		Id:     id,
		Reason: reason,
	})
	if err != nil {
		return fmt.Errorf("dismissing question: %w", err)
	}

	fmt.Printf("\033[33mDismissed question #%d\033[0m\n", id)
	return nil
}

func runQuestionsDefer(ctx context.Context, deps *ReviewCommandDeps, id int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToQuestionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := questionsv1.NewQuestionsServiceClient(conn)

	_, err = client.DeferQuestion(ctx, &questionsv1.DeferQuestionRequest{Id: id})
	if err != nil {
		return fmt.Errorf("deferring question: %w", err)
	}

	fmt.Printf("Deferred question #%d\n", id)
	return nil
}

func runQuestionsStats(ctx context.Context, deps *ReviewCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToQuestionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := questionsv1.NewQuestionsServiceClient(conn)
	tenantID := getTenantID()

	resp, err := client.GetQueueStats(ctx, &questionsv1.GetQueueStatsRequest{
		TenantId: &tenantID,
	})
	if err != nil {
		return fmt.Errorf("getting stats: %w", err)
	}

	format := cfg.OutputFormat
	if questionsOutput != "" {
		format = config.OutputFormat(questionsOutput)
	}

	return outputProtoQuestionsStats(format, resp.Stats)
}

func runQuestionsSource(ctx context.Context, deps *ReviewCommandDeps, id int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToQuestionsGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := questionsv1.NewQuestionsServiceClient(conn)

	resp, err := client.GetQuestionSource(ctx, &questionsv1.GetQuestionSourceRequest{
		QuestionId:   id,
		ContextChars: int32(questionsSourceContextChars),
	})
	if err != nil {
		return fmt.Errorf("getting question source: %w", err)
	}

	format := cfg.OutputFormat
	if questionsOutput != "" {
		format = config.OutputFormat(questionsOutput)
	}

	return outputProtoQuestionSource(format, resp)
}

// Conversion helpers

func stringToPriority(s string) questionsv1.QuestionPriority {
	switch strings.ToLower(s) {
	case "high":
		return questionsv1.QuestionPriority_QUESTION_PRIORITY_HIGH
	case "medium":
		return questionsv1.QuestionPriority_QUESTION_PRIORITY_MEDIUM
	case "low":
		return questionsv1.QuestionPriority_QUESTION_PRIORITY_LOW
	default:
		return questionsv1.QuestionPriority_QUESTION_PRIORITY_UNSPECIFIED
	}
}

func stringToQuestionType(s string) questionsv1.QuestionType {
	switch strings.ToLower(s) {
	case "acronym":
		return questionsv1.QuestionType_QUESTION_TYPE_ACRONYM
	case "person":
		return questionsv1.QuestionType_QUESTION_TYPE_PERSON
	case "entity":
		return questionsv1.QuestionType_QUESTION_TYPE_ENTITY
	case "duplicate":
		return questionsv1.QuestionType_QUESTION_TYPE_DUPLICATE
	case "other":
		return questionsv1.QuestionType_QUESTION_TYPE_OTHER
	default:
		return questionsv1.QuestionType_QUESTION_TYPE_UNSPECIFIED
	}
}

func priorityToString(p questionsv1.QuestionPriority) string {
	switch p {
	case questionsv1.QuestionPriority_QUESTION_PRIORITY_HIGH:
		return "high"
	case questionsv1.QuestionPriority_QUESTION_PRIORITY_MEDIUM:
		return "medium"
	case questionsv1.QuestionPriority_QUESTION_PRIORITY_LOW:
		return "low"
	default:
		return "unknown"
	}
}

func questionTypeToString(t questionsv1.QuestionType) string {
	switch t {
	case questionsv1.QuestionType_QUESTION_TYPE_ACRONYM:
		return "acronym"
	case questionsv1.QuestionType_QUESTION_TYPE_PERSON:
		return "person"
	case questionsv1.QuestionType_QUESTION_TYPE_ENTITY:
		return "entity"
	case questionsv1.QuestionType_QUESTION_TYPE_DUPLICATE:
		return "duplicate"
	case questionsv1.QuestionType_QUESTION_TYPE_OTHER:
		return "other"
	default:
		return "unknown"
	}
}

// Output functions

func outputProtoQuestionsList(format config.OutputFormat, items []*questionsv1.Question) error {
	switch format {
	case config.OutputFormatJSON:
		return outputQuestionsJSON(items)
	case config.OutputFormatYAML:
		return outputQuestionsYAML(items)
	default:
		return outputProtoQuestionsListText(items)
	}
}

func outputProtoQuestionsListText(items []*questionsv1.Question) error {
	if len(items) == 0 {
		fmt.Println("No pending questions.")
		return nil
	}

	fmt.Printf("Pending Questions (%d):\n\n", len(items))
	fmt.Println("  ID     PRI    TYPE      QUESTION")
	fmt.Println("  --     ---    ----      --------")

	for _, item := range items {
		priColor := getProtoPriorityColor(item.Priority)
		question := truncateQuestion(item.Question, 50)
		fmt.Printf("  %-6d %s%-6s\033[0m %-9s %s\n",
			item.Id,
			priColor,
			priorityToString(item.Priority),
			questionTypeToString(item.QuestionType),
			question)
	}

	fmt.Println()
	fmt.Println("Use 'penf review questions show <id>' for details.")
	fmt.Println("Use 'penf review questions resolve <id> <answer>' to answer.")
	return nil
}

func outputProtoQuestionDetail(format config.OutputFormat, item *questionsv1.Question) error {
	switch format {
	case config.OutputFormatJSON:
		return outputQuestionsJSON(item)
	case config.OutputFormatYAML:
		return outputQuestionsYAML(item)
	default:
		return outputProtoQuestionDetailText(item)
	}
}

func outputProtoQuestionDetailText(item *questionsv1.Question) error {
	priColor := getProtoPriorityColor(item.Priority)

	fmt.Println("Question Details:")
	fmt.Println()
	fmt.Printf("  \033[1mID:\033[0m       %d\n", item.Id)
	fmt.Printf("  \033[1mType:\033[0m     %s\n", questionTypeToString(item.QuestionType))
	fmt.Printf("  \033[1mPriority:\033[0m %s%s\033[0m\n", priColor, priorityToString(item.Priority))
	fmt.Printf("  \033[1mStatus:\033[0m   %s\n", item.Status.String())
	fmt.Println()
	fmt.Printf("  \033[1mQuestion:\033[0m\n")
	fmt.Printf("    %s\n", item.Question)

	if item.Context != "" {
		fmt.Println()
		fmt.Printf("  \033[1mContext:\033[0m\n")
		fmt.Printf("    \"%s\"\n", item.Context)
	}

	if item.SuggestedTerm != "" {
		fmt.Println()
		fmt.Printf("  \033[1mTerm:\033[0m     %s\n", item.SuggestedTerm)
	}

	if item.SourceReference != "" {
		fmt.Println()
		fmt.Printf("  \033[1mSource:\033[0m   %s\n", item.SourceReference)
	}

	fmt.Println()
	if item.CreatedAt != nil {
		fmt.Printf("  \033[1mCreated:\033[0m  %s\n", item.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}

	fmt.Println()
	fmt.Printf("To answer: penf review questions resolve %d \"<your answer>\"\n", item.Id)
	fmt.Printf("To dismiss: penf review questions dismiss %d\n", item.Id)

	return nil
}

func outputProtoQuestionsStats(format config.OutputFormat, stats *questionsv1.QueueStats) error {
	switch format {
	case config.OutputFormatJSON:
		return outputQuestionsJSON(stats)
	case config.OutputFormatYAML:
		return outputQuestionsYAML(stats)
	default:
		return outputProtoQuestionsStatsText(stats)
	}
}

func outputProtoQuestionsStatsText(stats *questionsv1.QueueStats) error {
	fmt.Println("Questions Queue Statistics:")
	fmt.Println()
	fmt.Printf("  \033[1mTotal Pending:\033[0m  %d\n", stats.TotalPending)
	fmt.Println()

	if len(stats.ByPriority) > 0 {
		fmt.Println("  By Priority:")
		for _, p := range []string{"high", "medium", "low"} {
			if count, ok := stats.ByPriority[p]; ok && count > 0 {
				color := getProtoPriorityColor(stringToPriority(p))
				fmt.Printf("    %s%-8s\033[0m %d\n", color, p, count)
			}
		}
		fmt.Println()
	}

	if len(stats.ByType) > 0 {
		fmt.Println("  By Type:")
		for qtype, count := range stats.ByType {
			fmt.Printf("    %-12s %d\n", qtype, count)
		}
		fmt.Println()
	}

	fmt.Printf("  \033[1mResolved Today:\033[0m %d\n", stats.ResolvedToday)

	if stats.OldestPending != nil {
		fmt.Printf("  \033[1mOldest Pending:\033[0m %s\n", stats.OldestPending.AsTime().Format("2006-01-02 15:04"))
	}

	return nil
}

func outputProtoQuestionSource(format config.OutputFormat, resp *questionsv1.GetQuestionSourceResponse) error {
	switch format {
	case config.OutputFormatJSON:
		return outputQuestionsJSON(resp)
	case config.OutputFormatYAML:
		return outputQuestionsYAML(resp)
	default:
		return outputProtoQuestionSourceText(resp)
	}
}

func outputProtoQuestionSourceText(resp *questionsv1.GetQuestionSourceResponse) error {
	source := resp.Source
	question := resp.Question

	fmt.Println("Source Content:")
	fmt.Println()

	// Show question context
	fmt.Printf("  \033[1mQuestion #%d:\033[0m %s\n", question.Id, question.Question)
	if question.SuggestedTerm != "" {
		fmt.Printf("  \033[1mTerm:\033[0m        %s\n", question.SuggestedTerm)
	}
	fmt.Println()

	// Show source metadata
	fmt.Printf("  \033[1mSource:\033[0m      %s (%s)\n", source.Title, source.SourceType)
	if source.SourceTimestamp != nil {
		fmt.Printf("  \033[1mDate:\033[0m        %s\n", source.SourceTimestamp.AsTime().Format("2006-01-02 15:04"))
	}

	// Show metadata
	if len(source.Metadata) > 0 {
		for k, v := range source.Metadata {
			if v != "" {
				fmt.Printf("  \033[1m%s:\033[0m %s\n", strings.Title(strings.ReplaceAll(k, "_", " ")), v)
			}
		}
	}
	fmt.Println()

	// Show content
	fmt.Println("  \033[1mContent:\033[0m")
	fmt.Println()

	// If we have a snippet, highlight it
	content := source.Content
	snippet := source.Snippet

	if snippet != "" && strings.Contains(content, snippet) {
		// Highlight the snippet
		highlighted := strings.Replace(content, snippet, "\033[43m"+snippet+"\033[0m", 1)
		// Indent each line
		for _, line := range strings.Split(highlighted, "\n") {
			fmt.Printf("    %s\n", line)
		}
	} else {
		// Just print content with indentation
		for _, line := range strings.Split(content, "\n") {
			fmt.Printf("    %s\n", line)
		}
	}

	fmt.Println()

	// Show if truncated
	if source.TotalLength > int32(len(content)) {
		fmt.Printf("  \033[2m(Showing %d of %d characters. Use --context -1 for full content)\033[0m\n", len(content), source.TotalLength)
	}

	return nil
}

func outputQuestionsJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputQuestionsYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

func getProtoPriorityColor(p questionsv1.QuestionPriority) string {
	switch p {
	case questionsv1.QuestionPriority_QUESTION_PRIORITY_HIGH:
		return "\033[31m" // Red
	case questionsv1.QuestionPriority_QUESTION_PRIORITY_MEDIUM:
		return "\033[33m" // Yellow
	case questionsv1.QuestionPriority_QUESTION_PRIORITY_LOW:
		return "\033[32m" // Green
	default:
		return ""
	}
}

func truncateQuestion(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
