package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	topicv1 "github.com/otherjamesbrown/penf-cli/api/proto/topic/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Topic command flags
var (
	topicOutput string
	topicLimit  int
)

// TopicCommandDeps holds the dependencies for topic commands.
type TopicCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultTopicDeps returns the default dependencies for production use.
func DefaultTopicDeps() *TopicCommandDeps {
	return &TopicCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

func getTenantIDForTopic(deps *TopicCommandDeps) string {
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant
	}
	if deps.Config != nil && deps.Config.EffectiveTenantID() != "" {
		return deps.Config.EffectiveTenantID()
	}
	return "00000001-0000-0000-0000-000000000001"
}

// NewTopicCommand creates the root topic command with all subcommands.
func NewTopicCommand(deps *TopicCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultTopicDeps()
	}

	cmd := &cobra.Command{
		Use:   "topic",
		Short: "Manage contextual knowledge topics",
		Long: `Manage topics — contextual knowledge entities richer than glossary terms
but without ownership, actions, or risks.

Topics help the pipeline understand content by providing paragraph-level
context for entities like infrastructure environments, network features,
and technical concepts that don't fit as projects or glossary terms.

Examples:
  penf topic list
  penf topic add "DevCloud" --description "Internal testing environment" --keywords dev,testing
  penf topic show DevCloud
  penf topic delete 42`,
		Aliases: []string{"topics"},
	}

	cmd.PersistentFlags().StringVarP(&topicOutput, "output", "o", "", "Output format: text, json, yaml")
	cmd.PersistentFlags().IntVarP(&topicLimit, "limit", "l", 50, "Maximum number of results")

	cmd.AddCommand(newTopicAddCommand(deps))
	cmd.AddCommand(newTopicListCommand(deps))
	cmd.AddCommand(newTopicShowCommand(deps))
	cmd.AddCommand(newTopicDeleteCommand(deps))
	cmd.AddCommand(newTopicUpdateCommand(deps))

	return cmd
}

func newTopicAddCommand(deps *TopicCommandDeps) *cobra.Command {
	var description string
	var keywords []string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a topic",
		Long: `Add a new topic with a name, description, and keywords.

Examples:
  penf topic add "DevCloud" --description "Internal testing environment shared across teams" --keywords dev,testing,internal
  penf topic add "Oslo" --description "Dedicated Linode region for MTC" --keywords mtc,region,linode
  penf topic add "Cloud NAT" --description "Network feature within Cloud Networking" --keywords network,nat`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTopicAdd(cmd.Context(), deps, args[0], description, keywords)
		},
	}

	cmd.Flags().StringVarP(&description, "description", "d", "", "Topic description (paragraph-level context)")
	cmd.Flags().StringSliceVarP(&keywords, "keywords", "k", nil, "Keywords for auto-tagging (comma-separated)")

	return cmd
}

func newTopicListCommand(deps *TopicCommandDeps) *cobra.Command {
	var keyword string
	var search string
	var projectID int64

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all topics",
		Long: `List all topics with their descriptions and keywords.

Examples:
  penf topic list
  penf topic list --keyword mtc
  penf topic list --search "cloud"
  penf topic list --project 5
  penf topic list -o json`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var projIDPtr *int64
			if cmd.Flags().Changed("project") {
				projIDPtr = &projectID
			}
			return runTopicList(cmd.Context(), deps, search, keyword, projIDPtr)
		},
	}

	cmd.Flags().StringVar(&keyword, "keyword", "", "Filter by keyword")
	cmd.Flags().StringVarP(&search, "search", "s", "", "Search by name")
	cmd.Flags().Int64Var(&projectID, "project", 0, "Filter by project ID")

	return cmd
}

func newTopicShowCommand(deps *TopicCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name-or-id>",
		Short: "Show topic details",
		Long: `Show detailed information about a topic.

Examples:
  penf topic show DevCloud
  penf topic show 42`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTopicShow(cmd.Context(), deps, args[0])
		},
	}
}

func newTopicDeleteCommand(deps *TopicCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a topic",
		Long: `Delete a topic by its ID.

Example:
  penf topic delete 42`,
		Aliases: []string{"rm", "remove"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var id int64
			if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil || id <= 0 {
				return fmt.Errorf("invalid topic ID: %s (must be a positive integer)", args[0])
			}
			return runTopicDelete(cmd.Context(), deps, id)
		},
	}
}

func newTopicUpdateCommand(deps *TopicCommandDeps) *cobra.Command {
	var description string
	var keywords []string
	var name string
	var projectID int64
	var runningContext string
	var status string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a topic",
		Long: `Update an existing topic's name, description, keywords, or project linkage.

Examples:
  penf topic update 42 --description "Updated description"
  penf topic update 42 --keywords new,keywords --name "New Name"
  penf topic update 42 --project 5 --status active
  penf topic update 42 --running-context "Migration delayed to Q3"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var id int64
			if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil || id <= 0 {
				return fmt.Errorf("invalid topic ID: %s (must be a positive integer)", args[0])
			}
			var projIDPtr *int64
			if cmd.Flags().Changed("project") {
				projIDPtr = &projectID
			}
			return runTopicUpdate(cmd.Context(), deps, id, topicUpdateOpts{
				Name:           name,
				Description:    description,
				Keywords:       keywords,
				ProjectID:      projIDPtr,
				RunningContext: runningContext,
				Status:         status,
			})
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New topic name")
	cmd.Flags().StringVarP(&description, "description", "d", "", "New description")
	cmd.Flags().StringSliceVarP(&keywords, "keywords", "k", nil, "New keywords (comma-separated)")
	cmd.Flags().Int64Var(&projectID, "project", 0, "Link to project ID (0 to unlink)")
	cmd.Flags().StringVar(&runningContext, "running-context", "", "Running context for the topic")
	cmd.Flags().StringVar(&status, "status", "", "Topic status (active, archived)")

	return cmd
}

// Command execution functions

func runTopicAdd(ctx context.Context, deps *TopicCommandDeps, name, description string, keywords []string) error {
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

	client := topicv1.NewTopicServiceClient(conn)
	tenantID := getTenantIDForTopic(deps)

	resp, err := client.CreateTopic(ctx, &topicv1.CreateTopicRequest{
		TenantId: tenantID,
		Input: &topicv1.TopicInput{
			Name:        name,
			Description: description,
			Keywords:    keywords,
		},
	})
	if err != nil {
		return fmt.Errorf("creating topic: %w", err)
	}

	t := resp.Topic
	fmt.Printf("\033[32mCreated topic:\033[0m %s (ID: %d)\n", t.Name, t.Id)
	if t.Description != "" {
		fmt.Printf("  Description: %s\n", t.Description)
	}
	if len(t.Keywords) > 0 {
		fmt.Printf("  Keywords:    %s\n", strings.Join(t.Keywords, ", "))
	}

	return nil
}

func runTopicList(ctx context.Context, deps *TopicCommandDeps, search, keyword string, projectID *int64) error {
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

	client := topicv1.NewTopicServiceClient(conn)
	tenantID := getTenantIDForTopic(deps)

	filter := &topicv1.TopicFilter{
		TenantId:   tenantID,
		NameSearch: search,
		Keyword:    keyword,
		Limit:      int32(topicLimit),
	}
	if projectID != nil {
		filter.ProjectId = projectID
	}

	resp, err := client.ListTopics(ctx, &topicv1.ListTopicsRequest{
		Filter: filter,
	})
	if err != nil {
		return fmt.Errorf("listing topics: %w", err)
	}

	format := cfg.OutputFormat
	if topicOutput != "" {
		format = config.OutputFormat(topicOutput)
	}

	return outputTopics(format, resp.Topics, resp.TotalCount)
}

func runTopicShow(ctx context.Context, deps *TopicCommandDeps, identifier string) error {
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

	client := topicv1.NewTopicServiceClient(conn)
	tenantID := getTenantIDForTopic(deps)

	resp, err := client.GetTopic(ctx, &topicv1.GetTopicRequest{
		TenantId:   tenantID,
		Identifier: identifier,
	})
	if err != nil {
		return fmt.Errorf("getting topic: %w", err)
	}

	format := cfg.OutputFormat
	if topicOutput != "" {
		format = config.OutputFormat(topicOutput)
	}

	return outputTopicDetail(format, resp.Topic)
}

func runTopicDelete(ctx context.Context, deps *TopicCommandDeps, id int64) error {
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

	client := topicv1.NewTopicServiceClient(conn)

	resp, err := client.DeleteTopic(ctx, &topicv1.DeleteTopicRequest{
		Id: id,
	})
	if err != nil {
		return fmt.Errorf("deleting topic: %w", err)
	}

	if resp.Success {
		fmt.Printf("\033[32mDeleted topic:\033[0m ID %d\n", id)
	} else {
		fmt.Printf("\033[31mFailed to delete topic:\033[0m ID %d\n", id)
	}

	return nil
}

type topicUpdateOpts struct {
	Name           string
	Description    string
	Keywords       []string
	ProjectID      *int64
	RunningContext string
	Status         string
}

func runTopicUpdate(ctx context.Context, deps *TopicCommandDeps, id int64, opts topicUpdateOpts) error {
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

	client := topicv1.NewTopicServiceClient(conn)

	input := &topicv1.TopicInput{
		Name:           opts.Name,
		Description:    opts.Description,
		Keywords:       opts.Keywords,
		RunningContext: opts.RunningContext,
		Status:         opts.Status,
	}
	if opts.ProjectID != nil {
		input.ProjectId = opts.ProjectID
	}

	resp, err := client.UpdateTopic(ctx, &topicv1.UpdateTopicRequest{
		Id:    id,
		Input: input,
	})
	if err != nil {
		return fmt.Errorf("updating topic: %w", err)
	}

	t := resp.Topic
	fmt.Printf("\033[32mUpdated topic:\033[0m %s (ID: %d)\n", t.Name, t.Id)
	if t.Description != "" {
		fmt.Printf("  Description: %s\n", t.Description)
	}
	if len(t.Keywords) > 0 {
		fmt.Printf("  Keywords:    %s\n", strings.Join(t.Keywords, ", "))
	}
	if t.Status != "" {
		fmt.Printf("  Status:      %s\n", t.Status)
	}

	return nil
}

// Output functions

func outputTopics(format config.OutputFormat, topics []*topicv1.Topic, totalCount int64) error {
	switch format {
	case config.OutputFormatJSON:
		return outputTopicJSON(map[string]interface{}{
			"topics":      topics,
			"total_count": totalCount,
		})
	case config.OutputFormatYAML:
		return outputTopicYAML(map[string]interface{}{
			"topics":      topics,
			"total_count": totalCount,
		})
	default:
		return outputTopicsText(topics, totalCount)
	}
}

func outputTopicsText(topics []*topicv1.Topic, totalCount int64) error {
	if len(topics) == 0 {
		fmt.Println("No topics found.")
		return nil
	}

	fmt.Printf("Topics (%d):\n\n", totalCount)
	fmt.Println("  ID    NAME                 DESCRIPTION                              KEYWORDS")
	fmt.Println("  --    ----                 -----------                              --------")

	for _, t := range topics {
		keywordStr := strings.Join(t.Keywords, ", ")
		if len(keywordStr) > 20 {
			keywordStr = keywordStr[:17] + "..."
		}
		descStr := t.Description
		if len(descStr) > 40 {
			descStr = descStr[:37] + "..."
		}
		fmt.Printf("  %-5d %-20s %-40s %s\n",
			t.Id,
			truncateString(t.Name, 20),
			descStr,
			keywordStr)
	}

	fmt.Println()
	return nil
}

func outputTopicDetail(format config.OutputFormat, topic *topicv1.Topic) error {
	switch format {
	case config.OutputFormatJSON:
		return outputTopicJSON(topic)
	case config.OutputFormatYAML:
		return outputTopicYAML(topic)
	default:
		return outputTopicDetailText(topic)
	}
}

func outputTopicDetailText(topic *topicv1.Topic) error {
	fmt.Println("Topic Details:")
	fmt.Println()
	fmt.Printf("  \033[1mID:\033[0m           %d\n", topic.Id)
	fmt.Printf("  \033[1mName:\033[0m         %s\n", topic.Name)
	if topic.Description != "" {
		fmt.Printf("  \033[1mDescription:\033[0m  %s\n", topic.Description)
	}
	if len(topic.Keywords) > 0 {
		fmt.Printf("  \033[1mKeywords:\033[0m     %s\n", strings.Join(topic.Keywords, ", "))
	}
	if topic.Status != "" {
		fmt.Printf("  \033[1mStatus:\033[0m       %s\n", topic.Status)
	}
	if topic.ProjectId != nil {
		fmt.Printf("  \033[1mProject ID:\033[0m   %d\n", *topic.ProjectId)
	}
	if topic.RunningContext != "" {
		fmt.Printf("  \033[1mContext:\033[0m       %s\n", topic.RunningContext)
	}
	fmt.Println()
	if topic.CreatedAt != nil {
		fmt.Printf("  \033[1mCreated:\033[0m      %s\n", topic.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}
	if topic.UpdatedAt != nil {
		fmt.Printf("  \033[1mUpdated:\033[0m      %s\n", topic.UpdatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}
	if topic.LastUpdatedAt != nil {
		fmt.Printf("  \033[1mLast Active:\033[0m  %s\n", topic.LastUpdatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}

	return nil
}

func outputTopicJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputTopicYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

// ListTopicsByProject lists topics scoped to a project, for use by project commands.
func ListTopicsByProject(ctx context.Context, deps *TopicCommandDeps, projectID int64, limit int32) ([]*topicv1.Topic, error) {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := topicv1.NewTopicServiceClient(conn)
	tenantID := getTenantIDForTopic(deps)

	resp, err := client.ListTopics(ctx, &topicv1.ListTopicsRequest{
		Filter: &topicv1.TopicFilter{
			TenantId:  tenantID,
			ProjectId: &projectID,
			Limit:     limit,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("listing project topics: %w", err)
	}

	return resp.Topics, nil
}
