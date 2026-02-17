// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	conversationv1 "github.com/otherjamesbrown/penf-cli/api/proto/conversation/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Conversation command flags.
var (
	conversationLimit  int32
	conversationOffset int32
	conversationOutput string
)

// ConversationCommandDeps holds the dependencies for conversation commands.
type ConversationCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultConversationDeps returns the default dependencies for production use.
func DefaultConversationDeps() *ConversationCommandDeps {
	return &ConversationCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewConversationCommand creates the conversation command group.
func NewConversationCommand(deps *ConversationCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultConversationDeps()
	}

	cmd := &cobra.Command{
		Use:   "conversation",
		Short: "Query conversations",
		Long: `Query conversations and their items.

Conversations group related content by topic, tracking participants and temporal sequence.

This command provides two main operations:

  list      List conversations with pagination
  show      Show detailed conversation view with items and participants

Examples:
  penf conversation list --limit 10
  penf conversation show <conversation-id>
  penf conversation list -o json`,
	}

	cmd.AddCommand(newConversationListCommand(deps))
	cmd.AddCommand(newConversationShowCommand(deps))

	return cmd
}

// newConversationListCommand creates the 'conversation list' subcommand.
func newConversationListCommand(deps *ConversationCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List conversations with pagination",
		Long: `List conversations with pagination.

Conversations are ordered by last seen date (most recent first).

Flags:
  --limit             Maximum results (default 20)
  --offset            Pagination offset
  -o, --output        Output format: text, json, yaml

Examples:
  penf conversation list --limit 10
  penf conversation list --offset 20 --limit 10
  penf conversation list -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConversationList(cmd.Context(), deps)
		},
	}

	cmd.Flags().Int32Var(&conversationLimit, "limit", 20, "Maximum results")
	cmd.Flags().Int32Var(&conversationOffset, "offset", 0, "Pagination offset")
	cmd.Flags().StringVarP(&conversationOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// newConversationShowCommand creates the 'conversation show' subcommand.
func newConversationShowCommand(deps *ConversationCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <conversation-id>",
		Short: "Show conversation with items and participants",
		Long: `Show a detailed view of a conversation including items and participants.

Items and participants are displayed with relevant metadata.

Flags:
  -o, --output        Output format: text, json, yaml

Examples:
  penf conversation show <conversation-id>
  penf conversation show <conversation-id> -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conversationID := args[0]
			return runConversationShow(cmd.Context(), deps, conversationID)
		},
	}

	cmd.Flags().StringVarP(&conversationOutput, "output", "o", "", "Output format: text, json, yaml")

	return cmd
}

// ==================== gRPC Connection ====================

// connectConversationToGateway creates a gRPC connection to the gateway service.
func connectConversationToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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

// getTenantIDForConversations returns the tenant ID from env or config.
func getTenantIDForConversations(deps *ConversationCommandDeps) string {
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant
	}
	if deps.Config != nil && deps.Config.TenantID != "" {
		return deps.Config.TenantID
	}
	// Default tenant.
	return "00000001-0000-0000-0000-000000000001"
}

// ==================== Command Execution Functions ====================

// runConversationList executes the conversation list command.
func runConversationList(ctx context.Context, deps *ConversationCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectConversationToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := conversationv1.NewConversationServiceClient(conn)
	tenantID := getTenantIDForConversations(deps)

	// Build request
	req := &conversationv1.ListConversationsRequest{
		TenantId: tenantID,
		Limit:    conversationLimit,
		Offset:   conversationOffset,
	}

	// Execute request
	resp, err := client.ListConversations(ctx, req)
	if err != nil {
		return fmt.Errorf("listing conversations: %w", err)
	}

	// Output results
	return outputConversationList(resp)
}

// runConversationShow executes the conversation show command.
func runConversationShow(ctx context.Context, deps *ConversationCommandDeps, conversationID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectConversationToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := conversationv1.NewConversationServiceClient(conn)
	tenantID := getTenantIDForConversations(deps)

	// Build request
	req := &conversationv1.ShowConversationRequest{
		TenantId:       tenantID,
		ConversationId: conversationID,
	}

	// Execute request
	resp, err := client.ShowConversation(ctx, req)
	if err != nil {
		return fmt.Errorf("showing conversation: %w", err)
	}

	// Output results
	return outputConversationDetail(resp)
}

// ==================== Output Functions ====================

// outputConversationList formats and displays the conversation list response.
func outputConversationList(resp *conversationv1.ListConversationsResponse) error {
	switch conversationOutput {
	case "json":
		return outputJSON(resp)
	case "yaml":
		return outputYAML(resp)
	default:
		return outputConversationListText(resp)
	}
}

// outputConversationListText displays conversations as a formatted table.
func outputConversationListText(resp *conversationv1.ListConversationsResponse) error {
	if len(resp.Conversations) == 0 {
		fmt.Println("No conversations found.")
		return nil
	}

	// Header
	fmt.Printf("%-38s %-50s %-8s %-13s %-20s\n",
		"ID", "TOPIC", "ITEMS", "PARTICIPANTS", "LAST SEEN")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────")

	// Rows
	for _, c := range resp.Conversations {
		topic := truncateThreadString(c.Topic, 50)
		lastSeen := formatThreadTimestamp(c.LastSeen)

		fmt.Printf("%-38s %-50s %-8d %-13d %-20s\n",
			c.Id,
			topic,
			c.ItemCount,
			c.ParticipantCount,
			lastSeen)
	}

	// Footer
	fmt.Printf("\nShowing %d conversations (Total: %d)\n", len(resp.Conversations), resp.TotalCount)

	return nil
}

// outputConversationDetail formats and displays the conversation detail response.
func outputConversationDetail(resp *conversationv1.ShowConversationResponse) error {
	switch conversationOutput {
	case "json":
		return outputJSON(resp)
	case "yaml":
		return outputYAML(resp)
	default:
		return outputConversationDetailText(resp)
	}
}

// outputConversationDetailText displays conversation details as formatted text.
func outputConversationDetailText(resp *conversationv1.ShowConversationResponse) error {
	// Conversation header
	fmt.Printf("Conversation ID:  %s\n", resp.Id)
	fmt.Printf("Topic:            %s\n", resp.Topic)
	fmt.Printf("Items:            %d\n", resp.ItemCount)
	fmt.Printf("Participants:     %d\n", resp.ParticipantCount)
	fmt.Printf("First Seen:       %s\n", formatThreadTimestamp(resp.FirstSeen))
	fmt.Printf("Last Seen:        %s\n", formatThreadTimestamp(resp.LastSeen))

	// Items
	if len(resp.Items) > 0 {
		fmt.Println("\nItems:")
		fmt.Println("────────────────────────────────────────────────────────────────────────────────────────")
		fmt.Printf("%-38s %-10s %-20s\n", "CONTENT ID", "SOURCE ID", "ADDED AT")
		fmt.Println("────────────────────────────────────────────────────────────────────────────────────────")

		for _, item := range resp.Items {
			sourceID := "N/A"
			if item.SourceId != nil {
				sourceID = fmt.Sprintf("%d", *item.SourceId)
			}
			addedAt := formatThreadTimestamp(item.AddedAt)

			fmt.Printf("%-38s %-10s %-20s\n",
				item.ContentId,
				sourceID,
				addedAt)
		}
	}

	// Participants
	if len(resp.Participants) > 0 {
		fmt.Println("\nParticipants:")
		fmt.Println("────────────────────────────────────────────────────────────────────────────────────────")
		fmt.Printf("%-30s %-40s\n", "NAME", "ADDRESS")
		fmt.Println("────────────────────────────────────────────────────────────────────────────────────────")

		for _, p := range resp.Participants {
			name := "N/A"
			if p.Name != nil {
				name = *p.Name
			}
			address := "N/A"
			if p.Address != nil {
				address = *p.Address
			}

			fmt.Printf("%-30s %-40s\n",
				truncateThreadString(name, 30),
				truncateThreadString(address, 40))
		}
	}

	fmt.Println()
	return nil
}
