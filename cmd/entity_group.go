package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	entityv1 "github.com/otherjamesbrown/penf-cli/api/proto/entity/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Group command flags.
var (
	groupIncludeRemoved bool
	groupSource         string
)

// newEntityGroupCommand creates the 'entity group' subcommand tree.
func newEntityGroupCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage entity group membership",
		Long: `Manage distribution list / group entity membership.

Groups are entities with account_type 'distribution'. Members are other
people entities that belong to the group.

Examples:
  # Add a member to a group
  penf entity group add 100 200
  penf entity group add dl-team@example.com john@example.com

  # List group members
  penf entity group list 100
  penf entity group list dl-team@example.com

  # Remove a member from a group
  penf entity group remove 100 200

  # List all groups (distribution list entities)
  penf entity group ls`,
	}

	cmd.AddCommand(newGroupAddCommand(deps))
	cmd.AddCommand(newGroupListCommand(deps))
	cmd.AddCommand(newGroupRemoveCommand(deps))
	cmd.AddCommand(newGroupListAllCommand(deps))

	return cmd
}

// newGroupAddCommand creates 'entity group add'.
func newGroupAddCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <group> <member>",
		Short: "Add a member to a group",
		Long: `Add a person entity as a member of a group (distribution list) entity.

Arguments accept entity IDs (numeric) or email addresses.

Examples:
  penf entity group add 100 200
  penf entity group add dl-team@example.com john@example.com`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGroupAdd(cmd.Context(), deps, args[0], args[1])
		},
	}

	cmd.Flags().StringVar(&groupSource, "source", "manual", "Source of the membership (e.g., manual, pipeline)")

	return cmd
}

// newGroupListCommand creates 'entity group list'.
func newGroupListCommand(deps *EntityCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <group>",
		Short: "List members of a group",
		Long: `List all members of a group (distribution list) entity.

Argument accepts entity ID (numeric) or email address.

Examples:
  penf entity group list 100
  penf entity group list dl-team@example.com
  penf entity group list 100 --include-removed`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGroupList(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().BoolVar(&groupIncludeRemoved, "include-removed", false, "Include removed members")

	return cmd
}

// newGroupRemoveCommand creates 'entity group remove'.
func newGroupRemoveCommand(deps *EntityCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <group> <member>",
		Short: "Remove a member from a group",
		Long: `Remove a person entity from a group (distribution list) entity.

This is a soft-delete — the membership is marked with a removed_at timestamp.

Arguments accept entity IDs (numeric) or email addresses.

Examples:
  penf entity group remove 100 200
  penf entity group remove dl-team@example.com john@example.com`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGroupRemove(cmd.Context(), deps, args[0], args[1])
		},
	}
}

// newGroupListAllCommand creates 'entity group ls'.
func newGroupListAllCommand(deps *EntityCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all group (distribution list) entities",
		Long: `List all entities with account_type 'distribution'.

Examples:
  penf entity group ls
  penf entity group ls -o json`,
		Aliases: []string{"all"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGroupListAll(cmd.Context(), deps)
		},
	}
}

// ==================== Execution Functions ====================

func runGroupAdd(ctx context.Context, deps *EntityCommandDeps, groupRef, memberRef string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	groupID, err := resolveEntityRef(ctx, client, tenantID, groupRef)
	if err != nil {
		return fmt.Errorf("resolving group: %w", err)
	}

	memberID, err := resolveEntityRef(ctx, client, tenantID, memberRef)
	if err != nil {
		return fmt.Errorf("resolving member: %w", err)
	}

	resp, err := client.AddGroupMember(ctx, &entityv1.AddGroupMemberRequest{
		TenantId:       tenantID,
		GroupEntityId:  groupID,
		MemberEntityId: memberID,
		Source:         groupSource,
	})
	if err != nil {
		return fmt.Errorf("adding group member: %w", err)
	}

	fmt.Printf("Added member %d to group %d (membership ID: %d)\n", memberID, groupID, resp.Id)
	return nil
}

func runGroupList(ctx context.Context, deps *EntityCommandDeps, groupRef string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	groupID, err := resolveEntityRef(ctx, client, tenantID, groupRef)
	if err != nil {
		return fmt.Errorf("resolving group: %w", err)
	}

	resp, err := client.ListGroupMembers(ctx, &entityv1.ListGroupMembersRequest{
		TenantId:       tenantID,
		GroupEntityId:  groupID,
		IncludeRemoved: groupIncludeRemoved,
	})
	if err != nil {
		return fmt.Errorf("listing group members: %w", err)
	}

	return outputGroupMembers(cfg, resp.Members)
}

func runGroupRemove(ctx context.Context, deps *EntityCommandDeps, groupRef, memberRef string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	groupID, err := resolveEntityRef(ctx, client, tenantID, groupRef)
	if err != nil {
		return fmt.Errorf("resolving group: %w", err)
	}

	memberID, err := resolveEntityRef(ctx, client, tenantID, memberRef)
	if err != nil {
		return fmt.Errorf("resolving member: %w", err)
	}

	resp, err := client.RemoveGroupMember(ctx, &entityv1.RemoveGroupMemberRequest{
		TenantId:       tenantID,
		GroupEntityId:  groupID,
		MemberEntityId: memberID,
	})
	if err != nil {
		return fmt.Errorf("removing group member: %w", err)
	}

	if resp.Removed {
		fmt.Printf("Removed member %d from group %d\n", memberID, groupID)
	} else {
		fmt.Printf("Member %d was not found in group %d\n", memberID, groupID)
	}
	return nil
}

func runGroupListAll(ctx context.Context, deps *EntityCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectEntityToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entityv1.NewEntityManagementServiceClient(conn)
	tenantID, err := getTenantIDForEntity(deps)
	if err != nil {
		return err
	}

	// Search for distribution list entities by common DL email prefix.
	// TODO: backend should support account_type filter on SearchEntities
	resp, err := client.SearchEntities(ctx, &entityv1.SearchEntitiesRequest{
		TenantId: tenantID,
		Query:    "dl-",
		Field:    "email",
		Limit:    int32(entityLimit),
	})
	if err != nil {
		return fmt.Errorf("listing groups: %w", err)
	}

	if len(resp.People) == 0 {
		fmt.Println("No distribution list entities found.")
		fmt.Println("Note: this searches for entities with 'dl-' in the email. Use 'penf entity search' for broader queries.")
		return nil
	}

	return outputEntitySearchResults(cfg, resp.People)
}

// ==================== Helper Functions ====================

// resolveEntityRef resolves an entity reference (numeric ID or email) to an entity ID.
func resolveEntityRef(ctx context.Context, client entityv1.EntityManagementServiceClient, tenantID, ref string) (int64, error) {
	// Try parsing as numeric ID first.
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		return id, nil
	}

	// Try parsing as prefixed entity ID (e.g., ent-person-123).
	if id, err := ParseEntityID(ref); err == nil {
		return id, nil
	}

	// Treat as email — resolve via search.
	if !strings.Contains(ref, "@") {
		return 0, fmt.Errorf("%q is not a valid entity ID or email address", ref)
	}

	resp, err := client.SearchEntities(ctx, &entityv1.SearchEntitiesRequest{
		TenantId: tenantID,
		Query:    ref,
		Field:    "email",
		Limit:    1,
	})
	if err != nil {
		return 0, fmt.Errorf("searching for entity by email %q: %w", ref, err)
	}

	if len(resp.People) == 0 {
		return 0, fmt.Errorf("no entity found with email %q", ref)
	}

	return resp.People[0].Id, nil
}

// ==================== Output Functions ====================

func outputGroupMembers(cfg *config.CLIConfig, members []*entityv1.GroupMember) error {
	format := getEntityOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputJSON(members)
	case config.OutputFormatYAML:
		return outputYAML(members)
	default:
		return outputGroupMembersTable(members)
	}
}

func outputGroupMembersTable(members []*entityv1.GroupMember) error {
	if len(members) == 0 {
		fmt.Println("No group members found.")
		return nil
	}

	fmt.Printf("Group Members (%d):\n\n", len(members))
	fmt.Println("  ID      MEMBER ID  NAME                           EMAIL                          SOURCE     ADDED")
	fmt.Println("  --      ---------  ----                           -----                          ------     -----")

	for _, m := range members {
		added := "-"
		if m.AddedAt != nil {
			added = m.AddedAt.AsTime().Format("2006-01-02")
		}
		removed := ""
		if m.RemovedAt != nil {
			removed = " (removed)"
		}
		fmt.Printf("  %-6d  %-9d  %-30s %-30s %-10s %s%s\n",
			m.Id,
			m.MemberEntityId,
			truncate(m.MemberName, 30),
			truncate(m.MemberEmail, 30),
			truncate(m.Source, 10),
			added,
			removed)
	}

	fmt.Println()
	return nil
}
