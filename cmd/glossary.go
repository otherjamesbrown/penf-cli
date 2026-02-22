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

	glossaryv1 "github.com/otherjamesbrown/penf-cli/api/proto/glossary/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Glossary command flags
var (
	glossaryOutput   string
	glossaryContext  []string
	glossaryLimit    int
	glossaryExpand   bool
	glossaryAliases  []string
	glossarySource   string
	glossaryNoExpand bool
)

// GlossaryCommandDeps holds the dependencies for glossary commands.
type GlossaryCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultGlossaryDeps returns the default dependencies for production use.
func DefaultGlossaryDeps() *GlossaryCommandDeps {
	return &GlossaryCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

func getTenantIDForGlossary(deps *GlossaryCommandDeps) string {
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant
	}
	if deps.Config != nil && deps.Config.TenantID != "" {
		return deps.Config.TenantID
	}
	return "00000001-0000-0000-0000-000000000001"
}

// NewGlossaryCommand creates the root glossary command with all subcommands.
func NewGlossaryCommand(deps *GlossaryCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultGlossaryDeps()
	}

	cmd := &cobra.Command{
		Use:   "glossary",
		Short: "Manage domain terminology and acronyms for query expansion",
		Long: `Manage domain terminology and acronyms for search query expansion.

The glossary stores acronyms, abbreviations, and domain-specific terminology
along with their full expansions and definitions. This enables Penfold to
understand searches like "TER meeting" as "Technical Execution Review meeting".

Terms can have:
  - Expansion: The full form (e.g., "Technical Execution Review")
  - Definition: A longer explanation of what the term means
  - Context: Tags for categorization (e.g., "MTC", "meetings")
  - Aliases: Alternative forms (e.g., "T.E.R.", "ter")

Query expansion automatically expands known acronyms in search queries.

JSON Output (for AI processing):
  penf glossary list -o json

  Returns:
  {
    "terms": [
      {"id": 1, "term": "LKE", "expansion": "Linode Kubernetes Engine", "context": ["infrastructure"]}
    ],
    "total": 42
  }

Documentation:
  How glossary works:     docs/concepts/glossary.md (multi-context, aliases, expansion)
  Acronym workflow:       docs/workflows/acronym-review.md (processing unknown acronyms)
  Entity types:           docs/shared/entities.md (glossary as an entity)`,
		Aliases: []string{"terms", "dict"},
	}

	// Add persistent flags
	cmd.PersistentFlags().StringVarP(&glossaryOutput, "output", "o", "", "Output format: text, json, yaml")
	cmd.PersistentFlags().IntVarP(&glossaryLimit, "limit", "l", 50, "Maximum number of results")

	// Add subcommands
	cmd.AddCommand(newGlossaryAddCommand(deps))
	cmd.AddCommand(newGlossaryListCommand(deps))
	cmd.AddCommand(newGlossaryShowCommand(deps))
	cmd.AddCommand(newGlossarySearchCommand(deps))
	cmd.AddCommand(newGlossaryRemoveCommand(deps))
	cmd.AddCommand(newGlossaryExpandCommand(deps))
	cmd.AddCommand(newGlossaryAliasCommand(deps))
	cmd.AddCommand(newGlossaryLinkCommand(deps))
	cmd.AddCommand(newGlossaryUnlinkCommand(deps))
	cmd.AddCommand(newGlossaryLinkedCommand(deps))

	return cmd
}

// newGlossaryAddCommand creates the 'glossary add' subcommand.
func newGlossaryAddCommand(deps *GlossaryCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <term> <expansion>",
		Short: "Add a term to the glossary",
		Long: `Add a new term or acronym to the glossary.

The term is the abbreviation or acronym (e.g., "TER"), and the expansion
is its full form (e.g., "Technical Execution Review").

Examples:
  # Add a simple acronym
  penf glossary add TER "Technical Execution Review"

  # Add with definition and context
  penf glossary add DBaaS "Database as a Service" \
    --definition "Product team for managed DB platform" \
    --context MTC,Oracle

  # Add with aliases
  penf glossary add MTC "Major TikTok Contract" \
    --aliases "TikTok Project","TT Contract" \
    --context TikTok,Oracle`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			definition, _ := cmd.Flags().GetString("definition")
			return runGlossaryAdd(cmd.Context(), deps, args[0], args[1], definition)
		},
	}

	cmd.Flags().StringP("definition", "d", "", "Definition or description of the term")
	cmd.Flags().StringSliceVarP(&glossaryContext, "context", "c", nil, "Context tags (comma-separated)")
	cmd.Flags().StringSliceVarP(&glossaryAliases, "aliases", "a", nil, "Aliases (comma-separated)")
	cmd.Flags().BoolVar(&glossaryNoExpand, "no-expand", false, "Don't use this term for query expansion")

	return cmd
}

// newGlossaryListCommand creates the 'glossary list' subcommand.
func newGlossaryListCommand(deps *GlossaryCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all glossary terms",
		Long: `List all terms in the glossary.

Displays terms with their expansions and context tags.

Examples:
  # List all terms
  penf glossary list

  # Filter by context
  penf glossary list --context MTC

  # Output as JSON
  penf glossary list --format json`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGlossaryList(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringSliceVarP(&glossaryContext, "context", "c", nil, "Filter by context tags")
	cmd.Flags().BoolVar(&glossaryExpand, "expand-only", false, "Only show terms used for query expansion")

	return cmd
}

// newGlossaryShowCommand creates the 'glossary show' subcommand.
func newGlossaryShowCommand(deps *GlossaryCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <term>",
		Short: "Show details of a term",
		Long: `Show detailed information about a glossary term.

Looks up the term by exact match or alias.

Example:
  penf glossary show TER`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGlossaryShow(cmd.Context(), deps, args[0])
		},
	}
}

// newGlossarySearchCommand creates the 'glossary search' subcommand.
func newGlossarySearchCommand(deps *GlossaryCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search glossary terms",
		Long: `Search glossary terms by full-text search.

Searches across term names, expansions, and definitions.

Example:
  penf glossary search "database"
  penf glossary search "TikTok"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			return runGlossarySearch(cmd.Context(), deps, query)
		},
	}
}

// newGlossaryRemoveCommand creates the 'glossary remove' subcommand.
func newGlossaryRemoveCommand(deps *GlossaryCommandDeps) *cobra.Command {
	var removeID int64

	cmd := &cobra.Command{
		Use:   "remove [term]",
		Short: "Remove a term from the glossary",
		Long: `Remove a term from the glossary by its term name or ID.

Examples:
  # Remove by term name
  penf glossary remove TER

  # Remove by ID (useful for duplicates or when term name is ambiguous)
  penf glossary remove --id 123`,
		Aliases: []string{"rm", "delete"},
		Args: cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate: either --id or term argument is provided
			if removeID == 0 && len(args) == 0 {
				return fmt.Errorf("either --id or term argument is required")
			}
			if removeID > 0 && len(args) > 0 {
				return fmt.Errorf("cannot specify both --id and term argument")
			}

			var termStr string
			if len(args) > 0 {
				termStr = args[0]
			}
			return runGlossaryRemove(cmd.Context(), deps, termStr, removeID)
		},
	}

	cmd.Flags().Int64Var(&removeID, "id", 0, "Remove term by ID instead of name")

	return cmd
}

// newGlossaryExpandCommand creates the 'glossary expand' subcommand.
func newGlossaryExpandCommand(deps *GlossaryCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "expand <query>",
		Short: "Show how a query would be expanded",
		Long: `Show how a search query would be expanded using glossary terms.

This is useful for debugging query expansion behavior.

Examples:
  penf glossary expand "TER meeting issues"
  penf glossary expand "DBaaS VPC configuration"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			return runGlossaryExpand(cmd.Context(), deps, query)
		},
	}
}

// newGlossaryAliasCommand creates the 'glossary alias' subcommand.
func newGlossaryAliasCommand(deps *GlossaryCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "alias <term> <alias>",
		Short: "Add an alias to an existing term",
		Long: `Add an alias to an existing glossary term.

This is useful for linking transcription errors or alternative spellings
to an existing term. The alias will resolve to the same expansion.

Git-like pattern: "alias points to term" (like git config alias.co checkout)

Examples:
  # Link transcription error to existing term
  penf glossary alias OBJ OBJE

  # Add alternative spelling
  penf glossary alias TER T.E.R.

  # Multiple aliases can be added by running multiple times
  penf glossary alias MTC "Major TikTok Contract"
  penf glossary alias MTC "TT Contract"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGlossaryAlias(cmd.Context(), deps, args[0], args[1])
		},
	}
}

// Glossary link command flags
var (
	glossaryLinkType string
)

// newGlossaryLinkCommand creates the 'glossary link' subcommand.
func newGlossaryLinkCommand(deps *GlossaryCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link <term> <entity-id>",
		Short: "Link a glossary term to a product, project, or company",
		Long: `Link a glossary term to a canonical entity.

This connects an acronym or term to its canonical product, project, or company
record in the system. Linked terms enable rich entity resolution in mentions.

Entity types:
  - product:  A product or service (e.g., "DBaaS", "Exadata")
  - project:  A project (e.g., "MTC", "TikTok Migration")
  - company:  A company (e.g., "Oracle", "TikTok")

Examples:
  # Link a term to a product
  penf glossary link DBaaS 123 --type product

  # Link to a project
  penf glossary link MTC 456 --type project

  # Link to a company (default type)
  penf glossary link ORCL 789`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			entityID, err := parseEntityID(args[1])
			if err != nil {
				return err
			}
			return runGlossaryLink(cmd.Context(), deps, args[0], glossaryLinkType, entityID)
		},
	}

	cmd.Flags().StringVarP(&glossaryLinkType, "type", "t", "company", "Entity type: product, project, company")

	return cmd
}

// newGlossaryUnlinkCommand creates the 'glossary unlink' subcommand.
func newGlossaryUnlinkCommand(deps *GlossaryCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "unlink <term>",
		Short: "Remove entity link from a glossary term",
		Long: `Remove the entity link from a glossary term.

This disconnects a term from its linked product, project, or company.
The term itself remains in the glossary.

Example:
  penf glossary unlink DBaaS`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGlossaryUnlink(cmd.Context(), deps, args[0])
		},
	}
}

// newGlossaryLinkedCommand creates the 'glossary linked' subcommand.
func newGlossaryLinkedCommand(deps *GlossaryCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "linked",
		Short: "List all terms linked to entities",
		Long: `List all glossary terms that are linked to entities.

Shows terms that have been connected to products, projects, or companies.

Examples:
  # List all linked terms
  penf glossary linked

  # Filter by entity type
  penf glossary linked --type product

  # Output as JSON
  penf glossary linked --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGlossaryLinked(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVarP(&glossaryLinkType, "type", "t", "", "Filter by entity type: product, project, company")

	return cmd
}

// parseEntityID converts string to int64 entity ID.
func parseEntityID(s string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(s, "%d", &id)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid entity ID: %s (must be a positive integer)", s)
	}
	return id, nil
}


// Command execution functions

func runGlossaryAdd(ctx context.Context, deps *GlossaryCommandDeps, term, expansion, definition string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := glossaryv1.NewGlossaryServiceClient(conn)
	tenantID := getTenantIDForGlossary(deps)

	req := &glossaryv1.AddTermRequest{
		TenantId:       tenantID,
		Term:           term,
		Expansion:      expansion,
		Definition:     definition,
		Context:        glossaryContext,
		Aliases:        glossaryAliases,
		ExpandInSearch: !glossaryNoExpand,
	}

	resp, err := client.AddTerm(ctx, req)
	if err != nil {
		return fmt.Errorf("adding term: %w", err)
	}

	created := resp.Term
	fmt.Printf("\033[32mAdded term:\033[0m %s\n", created.Term)
	fmt.Printf("  Expansion:  %s\n", created.Expansion)
	if created.Definition != "" {
		fmt.Printf("  Definition: %s\n", created.Definition)
	}
	if len(created.Context) > 0 {
		fmt.Printf("  Context:    %s\n", strings.Join(created.Context, ", "))
	}
	if len(created.Aliases) > 0 {
		fmt.Printf("  Aliases:    %s\n", strings.Join(created.Aliases, ", "))
	}
	fmt.Printf("  Expand:     %v\n", created.ExpandInSearch)

	return nil
}

func runGlossaryList(ctx context.Context, deps *GlossaryCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := glossaryv1.NewGlossaryServiceClient(conn)
	tenantID := getTenantIDForGlossary(deps)

	req := &glossaryv1.ListTermsRequest{
		TenantId:   tenantID,
		Context:    glossaryContext,
		ExpandOnly: glossaryExpand,
		Limit:      int32(glossaryLimit),
	}

	resp, err := client.ListTerms(ctx, req)
	if err != nil {
		return fmt.Errorf("listing terms: %w", err)
	}

	format := cfg.OutputFormat
	if glossaryOutput != "" {
		format = config.OutputFormat(glossaryOutput)
	}

	return outputGlossaryProtoTerms(format, resp.Terms)
}

func runGlossaryShow(ctx context.Context, deps *GlossaryCommandDeps, termStr string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := glossaryv1.NewGlossaryServiceClient(conn)
	tenantID := getTenantIDForGlossary(deps)

	resp, err := client.LookupTerm(ctx, &glossaryv1.LookupTermRequest{
		TenantId: tenantID,
		Term:     termStr,
	})
	if err != nil {
		return fmt.Errorf("looking up term: %w", err)
	}
	if !resp.Found {
		return fmt.Errorf("term not found: %s", termStr)
	}

	// Use canonical term name from LookupTerm (handles alias resolution)
	canonicalTerm := resp.Result.OriginalTerm
	termResp, err := client.GetTerm(ctx, &glossaryv1.GetTermRequest{
		TenantId: tenantID,
		Term:     canonicalTerm,
	})
	if err != nil {
		return fmt.Errorf("getting term details: %w", err)
	}

	// Show alias resolution note if the input was an alias
	if !strings.EqualFold(termStr, canonicalTerm) {
		fmt.Printf("  \033[36mResolved:\033[0m %q → %s (via alias)\n\n", termStr, canonicalTerm)
	}

	format := cfg.OutputFormat
	if glossaryOutput != "" {
		format = config.OutputFormat(glossaryOutput)
	}

	return outputGlossaryProtoTermDetail(format, termResp.Term)
}

func runGlossarySearch(ctx context.Context, deps *GlossaryCommandDeps, query string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := glossaryv1.NewGlossaryServiceClient(conn)
	tenantID := getTenantIDForGlossary(deps)

	req := &glossaryv1.ListTermsRequest{
		TenantId: tenantID,
		Search:   query,
		Limit:    int32(glossaryLimit),
	}

	resp, err := client.ListTerms(ctx, req)
	if err != nil {
		return fmt.Errorf("searching terms: %w", err)
	}

	format := cfg.OutputFormat
	if glossaryOutput != "" {
		format = config.OutputFormat(glossaryOutput)
	}

	return outputGlossaryProtoTerms(format, resp.Terms)
}

func runGlossaryRemove(ctx context.Context, deps *GlossaryCommandDeps, termStr string, removeID int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := glossaryv1.NewGlossaryServiceClient(conn)
	tenantID := getTenantIDForGlossary(deps)

	var termID int64
	var term *glossaryv1.Term

	if removeID > 0 {
		// Remove by ID - get the term first to show what we're deleting
		termResp, err := client.GetTerm(ctx, &glossaryv1.GetTermRequest{
			TenantId: tenantID,
			Id:       removeID,
		})
		if err != nil {
			return fmt.Errorf("looking up term by ID: %w", err)
		}
		if termResp.Term == nil {
			return fmt.Errorf("term not found with ID: %d", removeID)
		}
		term = termResp.Term
		termID = removeID
	} else {
		// Remove by term name - get the term first to show what we're deleting
		termResp, err := client.GetTerm(ctx, &glossaryv1.GetTermRequest{
			TenantId: tenantID,
			Term:     termStr,
		})
		if err != nil {
			return fmt.Errorf("looking up term: %w", err)
		}
		if termResp.Term == nil {
			return fmt.Errorf("term not found: %s", termStr)
		}
		term = termResp.Term
		termID = term.Id
	}

	// Delete by ID
	_, err = client.DeleteTerm(ctx, &glossaryv1.DeleteTermRequest{
		TenantId: tenantID,
		Id:       termID,
	})
	if err != nil {
		return fmt.Errorf("deleting term: %w", err)
	}

	fmt.Printf("\033[32mRemoved term:\033[0m %s (%s)\n", term.Term, term.Expansion)
	return nil
}

func runGlossaryExpand(ctx context.Context, deps *GlossaryCommandDeps, query string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := glossaryv1.NewGlossaryServiceClient(conn)
	tenantID := getTenantIDForGlossary(deps)

	resp, err := client.ExpandQuery(ctx, &glossaryv1.ExpandQueryRequest{
		TenantId: tenantID,
		Query:    query,
	})
	if err != nil {
		return fmt.Errorf("expanding query: %w", err)
	}

	format := cfg.OutputFormat
	if glossaryOutput != "" {
		format = config.OutputFormat(glossaryOutput)
	}

	return outputQueryExpansionProto(format, resp)
}

func runGlossaryAlias(ctx context.Context, deps *GlossaryCommandDeps, termStr, newAlias string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := glossaryv1.NewGlossaryServiceClient(conn)
	tenantID := getTenantIDForGlossary(deps)

	// First, look up the existing term
	termResp, err := client.GetTerm(ctx, &glossaryv1.GetTermRequest{
		TenantId: tenantID,
		Term:     termStr,
	})
	if err != nil {
		return fmt.Errorf("looking up term: %w", err)
	}
	if termResp.Term == nil {
		return fmt.Errorf("term not found: %s\n\nUse 'penf glossary add %s \"expansion\"' to create it first", termStr, termStr)
	}

	existingTerm := termResp.Term

	// Check if alias already exists
	for _, existing := range existingTerm.Aliases {
		if strings.EqualFold(existing, newAlias) {
			fmt.Printf("Alias '%s' already exists for term '%s'\n", newAlias, termStr)
			return nil
		}
	}

	// Add the new alias to existing aliases
	updatedAliases := append(existingTerm.Aliases, newAlias)

	// Update the term with new aliases
	updateResp, err := client.UpdateTerm(ctx, &glossaryv1.UpdateTermRequest{
		TenantId: tenantID,
		Id:       existingTerm.Id,
		Aliases:  updatedAliases,
	})
	if err != nil {
		return fmt.Errorf("updating term: %w", err)
	}

	fmt.Printf("\033[32mAdded alias:\033[0m %s → %s\n", newAlias, termStr)
	fmt.Printf("  Expansion: %s\n", updateResp.Term.Expansion)
	fmt.Printf("  All aliases: %s\n", strings.Join(updateResp.Term.Aliases, ", "))

	return nil
}

func runGlossaryLink(ctx context.Context, deps *GlossaryCommandDeps, termStr, entityType string, entityID int64) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := glossaryv1.NewGlossaryServiceClient(conn)
	tenantID := getTenantIDForGlossary(deps)

	resp, err := client.LinkTerm(ctx, &glossaryv1.LinkTermRequest{
		TenantId:   tenantID,
		TermStr:    termStr,
		EntityType: entityType,
		EntityId:   entityID,
	})
	if err != nil {
		return fmt.Errorf("linking term: %w", err)
	}

	fmt.Printf("\033[32mLinked term:\033[0m %s\n", resp.Term.Term)
	fmt.Printf("  Expansion: %s\n", resp.Term.Expansion)
	if resp.Term.LinkedEntity != nil {
		fmt.Printf("  Entity:    %s #%d\n", resp.Term.LinkedEntity.EntityType, resp.Term.LinkedEntity.EntityId)
	}

	return nil
}

func runGlossaryUnlink(ctx context.Context, deps *GlossaryCommandDeps, termStr string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := glossaryv1.NewGlossaryServiceClient(conn)
	tenantID := getTenantIDForGlossary(deps)

	resp, err := client.UnlinkTerm(ctx, &glossaryv1.UnlinkTermRequest{
		TenantId: tenantID,
		TermStr:  termStr,
	})
	if err != nil {
		return fmt.Errorf("unlinking term: %w", err)
	}

	fmt.Printf("\033[32mUnlinked term:\033[0m %s\n", resp.Term.Term)
	fmt.Printf("  Expansion: %s\n", resp.Term.Expansion)

	return nil
}

func runGlossaryLinked(ctx context.Context, deps *GlossaryCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := glossaryv1.NewGlossaryServiceClient(conn)
	tenantID := getTenantIDForGlossary(deps)

	resp, err := client.ListLinkedTerms(ctx, &glossaryv1.ListLinkedTermsRequest{
		TenantId:   tenantID,
		EntityType: glossaryLinkType,
		Limit:      int32(glossaryLimit),
	})
	if err != nil {
		return fmt.Errorf("listing linked terms: %w", err)
	}

	format := cfg.OutputFormat
	if glossaryOutput != "" {
		format = config.OutputFormat(glossaryOutput)
	}

	return outputLinkedTerms(format, resp.Terms, resp.TotalCount)
}

// Output functions for proto types

func outputGlossaryProtoTerms(format config.OutputFormat, terms []*glossaryv1.Term) error {
	switch format {
	case config.OutputFormatJSON:
		return outputGlossaryJSON(terms)
	case config.OutputFormatYAML:
		return outputGlossaryYAML(terms)
	default:
		return outputGlossaryProtoTermsText(terms)
	}
}

func outputGlossaryProtoTermsText(terms []*glossaryv1.Term) error {
	if len(terms) == 0 {
		fmt.Println("No terms found.")
		return nil
	}

	fmt.Printf("Glossary Terms (%d):\n\n", len(terms))
	fmt.Println("  TERM          EXPANSION                              CONTEXT")
	fmt.Println("  ----          ---------                              -------")

	for _, t := range terms {
		contextStr := strings.Join(t.Context, ", ")
		if len(contextStr) > 20 {
			contextStr = contextStr[:17] + "..."
		}
		fmt.Printf("  %-13s %-38s %s\n",
			truncateGlossary(t.Term, 13),
			truncateGlossary(t.Expansion, 38),
			contextStr)
	}

	fmt.Println()
	return nil
}

func outputGlossaryProtoTermDetail(format config.OutputFormat, term *glossaryv1.Term) error {
	switch format {
	case config.OutputFormatJSON:
		return outputGlossaryJSON(term)
	case config.OutputFormatYAML:
		return outputGlossaryYAML(term)
	default:
		return outputGlossaryProtoTermDetailText(term)
	}
}

func outputGlossaryProtoTermDetailText(term *glossaryv1.Term) error {
	fmt.Println("Term Details:")
	fmt.Println()
	fmt.Printf("  \033[1mTerm:\033[0m         %s\n", term.Term)
	fmt.Printf("  \033[1mExpansion:\033[0m    %s\n", term.Expansion)
	if term.Definition != "" {
		fmt.Printf("  \033[1mDefinition:\033[0m   %s\n", term.Definition)
	}
	fmt.Println()
	if len(term.Context) > 0 {
		fmt.Printf("  \033[1mContext:\033[0m      %s\n", strings.Join(term.Context, ", "))
	}
	if len(term.Aliases) > 0 {
		fmt.Printf("  \033[1mAliases:\033[0m      %s\n", strings.Join(term.Aliases, ", "))
	}
	fmt.Printf("  \033[1mExpand:\033[0m       %v\n", term.ExpandInSearch)
	fmt.Printf("  \033[1mSource:\033[0m       %s\n", term.Source)
	fmt.Println()
	if term.CreatedAt != nil {
		fmt.Printf("  \033[1mCreated:\033[0m      %s\n", term.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}
	if term.UpdatedAt != nil {
		fmt.Printf("  \033[1mUpdated:\033[0m      %s\n", term.UpdatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}

	return nil
}

func outputQueryExpansionProto(format config.OutputFormat, resp *glossaryv1.ExpandQueryResponse) error {
	switch format {
	case config.OutputFormatJSON:
		return outputGlossaryJSON(resp)
	case config.OutputFormatYAML:
		return outputGlossaryYAML(resp)
	default:
		return outputQueryExpansionProtoText(resp)
	}
}

func outputQueryExpansionProtoText(resp *glossaryv1.ExpandQueryResponse) error {
	fmt.Println("Query Expansion:")
	fmt.Println()
	fmt.Printf("  \033[1mOriginal:\033[0m    %s\n", resp.OriginalQuery)
	fmt.Println()

	if len(resp.ExpandedTerms) == 0 {
		fmt.Println("  No terms matched for expansion.")
		fmt.Println()
		fmt.Printf("  \033[1mExpanded:\033[0m    %s\n", resp.ExpandedQuery)
		return nil
	}

	fmt.Println("  \033[1mMatched Terms:\033[0m")
	for _, t := range resp.ExpandedTerms {
		fmt.Printf("    \033[36m%s\033[0m → %s\n", t.OriginalTerm, t.Expansion)
		if t.Definition != "" {
			fmt.Printf("      (%s)\n", t.Definition)
		}
	}
	fmt.Println()
	fmt.Printf("  \033[1mExpanded:\033[0m    %s\n", resp.ExpandedQuery)

	return nil
}

func outputLinkedTerms(format config.OutputFormat, terms []*glossaryv1.Term, totalCount int64) error {
	switch format {
	case config.OutputFormatJSON:
		return outputGlossaryJSON(map[string]interface{}{
			"terms":       terms,
			"total_count": totalCount,
		})
	case config.OutputFormatYAML:
		return outputGlossaryYAML(map[string]interface{}{
			"terms":       terms,
			"total_count": totalCount,
		})
	default:
		return outputLinkedTermsText(terms, totalCount)
	}
}

func outputLinkedTermsText(terms []*glossaryv1.Term, totalCount int64) error {
	if len(terms) == 0 {
		fmt.Println("No linked terms found.")
		return nil
	}

	fmt.Printf("Linked Terms (%d total):\n\n", totalCount)
	fmt.Println("  TERM          EXPANSION                         TYPE       ID")
	fmt.Println("  ----          ---------                         ----       --")

	for _, t := range terms {
		entityType := ""
		var entityID int64
		if t.LinkedEntity != nil {
			entityType = t.LinkedEntity.EntityType
			entityID = t.LinkedEntity.EntityId
		}
		fmt.Printf("  %-13s %-33s %-10s %d\n",
			truncateGlossary(t.Term, 13),
			truncateGlossary(t.Expansion, 33),
			entityType,
			entityID)
	}

	fmt.Println()
	return nil
}

func outputGlossaryJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputGlossaryYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

func truncateGlossary(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// Ensure time import is used
var _ = time.Now
