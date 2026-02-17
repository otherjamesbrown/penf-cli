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

	questionsv1 "github.com/otherjamesbrown/penf-cli/api/proto/questions/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Onboarding command flags.
var (
	onboardingOutput   string
	onboardingCategory string
)

// OnboardingContext represents the full context for post-import review.
type OnboardingContext struct {
	Summary             OnboardingSummary     `json:"summary"`
	NewPeople           []OnboardingPerson    `json:"new_people,omitempty"`
	NewAcronyms         []OnboardingAcronym   `json:"new_acronyms,omitempty"`
	UnresolvedMentions  []OnboardingMention   `json:"unresolved_mentions,omitempty"`
	PotentialDuplicates []OnboardingDuplicate `json:"potential_duplicates,omitempty"`
	Workflow            OnboardingWorkflow    `json:"workflow"`
}

// OnboardingSummary provides counts for each category.
type OnboardingSummary struct {
	NewPeople           int    `json:"new_people"`
	NewAcronyms         int    `json:"new_acronyms"`
	UnresolvedMentions  int    `json:"unresolved_mentions"`
	PotentialDuplicates int    `json:"potential_duplicates"`
	LastImport          string `json:"last_import,omitempty"`
}

// OnboardingPerson represents an auto-created person needing review.
type OnboardingPerson struct {
	ID             int64    `json:"id"`
	CanonicalName  string   `json:"canonical_name"`
	EmailAddresses []string `json:"email_addresses,omitempty"`
	Company        string   `json:"company,omitempty"`
	AutoCreated    bool     `json:"auto_created"`
	NeedsReview    bool     `json:"needs_review"`
	SourceCount    int      `json:"source_count"`
	FirstSeen      string   `json:"first_seen,omitempty"`
}

// OnboardingAcronym represents an unknown acronym needing review.
type OnboardingAcronym struct {
	ID              int64  `json:"id"`
	Term            string `json:"term"`
	Question        string `json:"question"`
	Context         string `json:"context"`
	SourceReference string `json:"source_reference,omitempty"`
	Priority        string `json:"priority"`
}

// OnboardingMention represents an unresolved mention.
type OnboardingMention struct {
	ID             int64              `json:"id"`
	MentionedText  string             `json:"mentioned_text"`
	ContextSnippet string             `json:"context_snippet"`
	Candidates     []MentionCandidate `json:"candidates,omitempty"`
}

// MentionCandidate represents a possible match for a mention.
type MentionCandidate struct {
	PersonID int64   `json:"person_id"`
	Name     string  `json:"name"`
	Email    string  `json:"email,omitempty"`
	Score    float64 `json:"score"`
}

// OnboardingDuplicate represents a potential duplicate person.
type OnboardingDuplicate struct {
	PersonID       int64    `json:"person_id"`
	CanonicalName  string   `json:"canonical_name"`
	EmailAddresses []string `json:"email_addresses,omitempty"`
	PotentialMatch struct {
		PersonID       int64    `json:"person_id"`
		CanonicalName  string   `json:"canonical_name"`
		EmailAddresses []string `json:"email_addresses,omitempty"`
		Similarity     float64  `json:"similarity"`
	} `json:"potential_match"`
}

// OnboardingWorkflow provides guidance for processing.
type OnboardingWorkflow struct {
	RecommendedOrder []string          `json:"recommended_order"`
	Commands         map[string]string `json:"commands"`
	BatchCommand     string            `json:"batch_command"`
}

// newProcessOnboardingCommand creates the 'process onboarding' subcommand group.
func newProcessOnboardingCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "onboarding",
		Short: "Post-import entity review workflow",
		Long: `Post-import guided review of discovered entities.

After importing emails or documents, Penfold discovers new entities:
- People from email headers
- Unknown acronyms
- Unresolved person mentions
- Potential duplicate people

This workflow helps Claude guide you through reviewing them efficiently.

Commands:
  context     Get full context for Claude-guided review
  batch       Batch process confirmations, resolutions, merges

See docs/workflows/onboarding.md for detailed documentation.`,
		Aliases: []string{"onboard"},
	}

	cmd.AddCommand(newOnboardingContextCommand(deps))
	cmd.AddCommand(newOnboardingBatchCommand(deps))

	return cmd
}

// newOnboardingContextCommand creates the 'process onboarding context' subcommand.
func newOnboardingContextCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Get full context for post-import review",
		Long: `Get complete context needed for Claude-guided post-import review.

Returns:
- New people (auto-created from email headers)
- Unknown acronyms (pending in question queue)
- Unresolved mentions (couldn't auto-match)
- Potential duplicate people

This single command provides everything Claude needs to guide you through
reviewing and confirming the entities discovered during import.

Examples:
  penf process onboarding context
  penf process onboarding context --output json
  penf process onboarding context --category acronyms`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOnboardingContext(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&onboardingOutput, "output", "o", "json", "Output format: json, text")
	cmd.Flags().StringVar(&onboardingCategory, "category", "", "Filter to specific category: people, acronyms, mentions, duplicates")

	return cmd
}

// newOnboardingBatchCommand creates the 'process onboarding batch' subcommand.
func newOnboardingBatchCommand(deps *ProcessCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch <json>",
		Short: "Batch process onboarding actions",
		Long: `Batch process multiple onboarding actions in a single operation.

Accepts JSON with actions for each category:
{
  "merge_people": [
    {"keep_id": 3, "merge_id": 45}
  ],
  "confirm_people": [12, 14, 16],
  "acronym_resolutions": [
    {"id": 123, "expansion": "Product Launch Date"}
  ],
  "acronym_dismissals": [
    {"id": 125, "reason": "Speaker initials"}
  ],
  "mention_resolutions": [
    {"mention_id": 201, "person_id": 5, "create_pattern": true}
  ],
  "mention_dismissals": [
    {"mention_id": 202, "reason": "Not a person reference"}
  ]
}

Use --dry-run to preview changes without executing them.

Example:
  penf process onboarding batch '{"confirm_people":[12,14]}'
  penf process onboarding batch --dry-run '{"merge_people":[...]}'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOnboardingBatch(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().BoolVar(&processDryRun, "dry-run", false, "Preview changes without executing them")

	return cmd
}

// runOnboardingContext executes the context command.
func runOnboardingContext(ctx context.Context, deps *ProcessCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	conn, err := connectToOnboardingGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	result := OnboardingContext{
		Workflow: OnboardingWorkflow{
			RecommendedOrder: []string{"duplicates", "people", "acronyms", "mentions"},
			Commands: map[string]string{
				"review_people":   "penf relationship entity list --needs-review",
				"review_acronyms": "penf process acronyms context",
				"review_mentions": "penf process mentions context",
				"merge_duplicate": "penf relationship entity merge <keep-id> <merge-id>",
			},
			BatchCommand: "penf process onboarding batch '<json>'",
		},
	}

	// Filter by category if specified
	categories := []string{"people", "acronyms", "mentions", "duplicates"}
	if onboardingCategory != "" {
		categories = []string{onboardingCategory}
	}

	for _, cat := range categories {
		switch cat {
		case "acronyms":
			// Fetch pending acronym questions
			questionsClient := questionsv1.NewQuestionsServiceClient(conn)
			questionsResp, err := questionsClient.ListQuestions(ctx, &questionsv1.ListQuestionsRequest{
				Status:       questionsv1.QuestionStatus_QUESTION_STATUS_PENDING,
				QuestionType: questionsv1.QuestionType_QUESTION_TYPE_ACRONYM,
				Limit:        100,
			})
			if err != nil {
				// Log but continue - service might not be available
				fmt.Fprintf(os.Stderr, "Warning: Could not fetch acronym questions: %v\n", err)
			} else {
				for _, q := range questionsResp.Questions {
					result.NewAcronyms = append(result.NewAcronyms, OnboardingAcronym{
						ID:              q.Id,
						Term:            q.SuggestedTerm,
						Question:        q.Question,
						Context:         q.Context,
						SourceReference: q.SourceReference,
						Priority:        priorityToString(q.Priority),
					})
				}
				result.Summary.NewAcronyms = len(result.NewAcronyms)
			}

		case "people":
			// People service not yet exposed via gRPC
			// Placeholder - would query for auto_created=true, needs_review=true
			result.Summary.NewPeople = 0 // Will be populated when service is available

		case "mentions":
			// Mentions service available via PR #15 but may not be deployed
			// Placeholder for now
			result.Summary.UnresolvedMentions = 0

		case "duplicates":
			// Duplicate detection not yet exposed via gRPC
			result.Summary.PotentialDuplicates = 0
		}
	}

	// Output
	return outputOnboardingContext(onboardingOutput, result)
}

// runOnboardingBatch executes the batch command.
func runOnboardingBatch(ctx context.Context, deps *ProcessCommandDeps, jsonInput string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Parse input JSON
	var req OnboardingBatchRequest
	if err := json.Unmarshal([]byte(jsonInput), &req); err != nil {
		return fmt.Errorf("parsing JSON input: %w", err)
	}

	// Dry-run mode
	if processDryRun {
		fmt.Println("\033[1m=== DRY RUN - No changes will be made ===\033[0m")
		fmt.Println()

		if len(req.MergePeople) > 0 {
			fmt.Printf("Would merge %d people:\n", len(req.MergePeople))
			for _, m := range req.MergePeople {
				fmt.Printf("  Merge #%d into #%d\n", m.MergeID, m.KeepID)
			}
			fmt.Println()
		}

		if len(req.ConfirmPeople) > 0 {
			fmt.Printf("Would confirm %d people: %v\n", len(req.ConfirmPeople), req.ConfirmPeople)
			fmt.Println()
		}

		if len(req.AcronymResolutions) > 0 {
			fmt.Printf("Would resolve %d acronyms:\n", len(req.AcronymResolutions))
			for _, r := range req.AcronymResolutions {
				fmt.Printf("  #%d: %s\n", r.ID, r.Expansion)
			}
			fmt.Println()
		}

		if len(req.AcronymDismissals) > 0 {
			fmt.Printf("Would dismiss %d acronyms:\n", len(req.AcronymDismissals))
			for _, d := range req.AcronymDismissals {
				fmt.Printf("  #%d: %s\n", d.ID, d.Reason)
			}
			fmt.Println()
		}

		if len(req.MentionResolutions) > 0 {
			fmt.Printf("Would resolve %d mentions:\n", len(req.MentionResolutions))
			for _, r := range req.MentionResolutions {
				pattern := ""
				if r.CreatePattern {
					pattern = " (+ pattern)"
				}
				fmt.Printf("  #%d → person #%d%s\n", r.MentionID, r.PersonID, pattern)
			}
			fmt.Println()
		}

		if len(req.MentionDismissals) > 0 {
			fmt.Printf("Would dismiss %d mentions:\n", len(req.MentionDismissals))
			for _, d := range req.MentionDismissals {
				fmt.Printf("  #%d: %s\n", d.MentionID, d.Reason)
			}
			fmt.Println()
		}

		fmt.Println("\033[2mRun without --dry-run to apply these changes.\033[0m")
		return nil
	}

	conn, err := connectToOnboardingGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	var result OnboardingBatchResult

	// Process acronym resolutions
	if len(req.AcronymResolutions) > 0 || len(req.AcronymDismissals) > 0 {
		questionsClient := questionsv1.NewQuestionsServiceClient(conn)

		for _, r := range req.AcronymResolutions {
			_, err := questionsClient.ResolveQuestion(ctx, &questionsv1.ResolveQuestionRequest{
				Id:     r.ID,
				Answer: r.Expansion,
			})
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("resolve acronym %d: %v", r.ID, err))
			} else {
				result.AcronymsResolved++
				fmt.Printf("\033[32m✓\033[0m Resolved acronym #%d: %s\n", r.ID, r.Expansion)
			}
		}

		for _, d := range req.AcronymDismissals {
			_, err := questionsClient.DismissQuestion(ctx, &questionsv1.DismissQuestionRequest{
				Id:     d.ID,
				Reason: d.Reason,
			})
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("dismiss acronym %d: %v", d.ID, err))
			} else {
				result.AcronymsDismissed++
				fmt.Printf("\033[33m✓\033[0m Dismissed acronym #%d: %s\n", d.ID, d.Reason)
			}
		}
	}

	// People and mentions operations would go here when services are available
	if len(req.MergePeople) > 0 {
		fmt.Printf("\033[33m⚠\033[0m People merge requires service support (coming soon)\n")
	}

	if len(req.ConfirmPeople) > 0 {
		fmt.Printf("\033[33m⚠\033[0m People confirm requires service support (coming soon)\n")
	}

	if len(req.MentionResolutions) > 0 || len(req.MentionDismissals) > 0 {
		fmt.Printf("\033[33m⚠\033[0m Mention resolution requires service support (coming soon)\n")
	}

	// Summary
	fmt.Println()
	fmt.Printf("Batch complete: %d acronyms resolved, %d dismissed",
		result.AcronymsResolved, result.AcronymsDismissed)
	if len(result.Errors) > 0 {
		fmt.Printf(", %d errors\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  \033[31mError:\033[0m %s\n", e)
		}
	} else {
		fmt.Println()
	}

	return nil
}

// OnboardingBatchRequest represents a batch of onboarding actions.
type OnboardingBatchRequest struct {
	MergePeople        []MergePeopleAction `json:"merge_people,omitempty"`
	ConfirmPeople      []int64             `json:"confirm_people,omitempty"`
	AcronymResolutions []AcronymResolution `json:"acronym_resolutions,omitempty"`
	AcronymDismissals  []AcronymDismissal  `json:"acronym_dismissals,omitempty"`
	MentionResolutions []MentionResolution `json:"mention_resolutions,omitempty"`
	MentionDismissals  []MentionDismissal  `json:"mention_dismissals,omitempty"`
}

// MergePeopleAction represents a people merge.
type MergePeopleAction struct {
	KeepID  int64 `json:"keep_id"`
	MergeID int64 `json:"merge_id"`
}

// AcronymResolution represents an acronym resolution.
type AcronymResolution struct {
	ID        int64  `json:"id"`
	Expansion string `json:"expansion"`
}

// AcronymDismissal represents an acronym dismissal.
type AcronymDismissal struct {
	ID     int64  `json:"id"`
	Reason string `json:"reason"`
}

// MentionResolution represents a mention resolution.
type MentionResolution struct {
	MentionID     int64 `json:"mention_id"`
	PersonID      int64 `json:"person_id"`
	CreatePattern bool  `json:"create_pattern,omitempty"`
}

// MentionDismissal represents a mention dismissal.
type MentionDismissal struct {
	MentionID int64  `json:"mention_id"`
	Reason    string `json:"reason"`
}

// OnboardingBatchResult represents the result of batch operations.
type OnboardingBatchResult struct {
	PeopleMerged      int      `json:"people_merged"`
	PeopleConfirmed   int      `json:"people_confirmed"`
	AcronymsResolved  int      `json:"acronyms_resolved"`
	AcronymsDismissed int      `json:"acronyms_dismissed"`
	MentionsResolved  int      `json:"mentions_resolved"`
	MentionsDismissed int      `json:"mentions_dismissed"`
	Errors            []string `json:"errors,omitempty"`
}

// connectToOnboardingGateway creates a gRPC connection to the gateway.
func connectToOnboardingGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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

// outputOnboardingContext outputs the context in the specified format.
func outputOnboardingContext(format string, ctx OnboardingContext) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(ctx)
	case "text":
		return outputOnboardingContextText(ctx)
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(ctx)
	}
}

// outputOnboardingContextText outputs context in human-readable format.
func outputOnboardingContextText(ctx OnboardingContext) error {
	fmt.Println("Post-Import Onboarding Context")
	fmt.Println("==============================")
	fmt.Println()

	fmt.Println("Summary:")
	fmt.Printf("  New people:           %d\n", ctx.Summary.NewPeople)
	fmt.Printf("  Unknown acronyms:     %d\n", ctx.Summary.NewAcronyms)
	fmt.Printf("  Unresolved mentions:  %d\n", ctx.Summary.UnresolvedMentions)
	fmt.Printf("  Potential duplicates: %d\n", ctx.Summary.PotentialDuplicates)
	fmt.Println()

	if len(ctx.NewAcronyms) > 0 {
		fmt.Printf("Unknown Acronyms (%d):\n", len(ctx.NewAcronyms))
		for _, a := range ctx.NewAcronyms {
			fmt.Printf("  #%-4d [%s] %s\n", a.ID, a.Priority, a.Term)
			if a.Context != "" {
				fmt.Printf("         \"%s\"\n", truncateOnboardingString(a.Context, 60))
			}
		}
		fmt.Println()
	}

	if len(ctx.NewPeople) > 0 {
		fmt.Printf("New People (%d):\n", len(ctx.NewPeople))
		for _, p := range ctx.NewPeople {
			emails := strings.Join(p.EmailAddresses, ", ")
			fmt.Printf("  #%-4d %s <%s>\n", p.ID, p.CanonicalName, emails)
		}
		fmt.Println()
	}

	fmt.Println("Recommended review order: duplicates → people → acronyms → mentions")
	fmt.Println()
	fmt.Println("Batch command:")
	fmt.Printf("  %s\n", ctx.Workflow.BatchCommand)

	return nil
}

// truncateOnboardingString truncates a string to max length.
func truncateOnboardingString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// Note: priorityToString is defined in review_questions.go
