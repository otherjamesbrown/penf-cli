// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

const (
	keyInboundWhitelist  = "email.inbound_whitelist"
	keyOutboundWhitelist = "email.outbound_whitelist"
)

// NewConfigEmailCmd returns the `penf config email` command tree.
func NewConfigEmailCmd(deps *PipelineCommandDeps) *cobra.Command {
	emailCmd := &cobra.Command{
		Use:   "email",
		Short: "Manage email configuration",
		Long: `Manage email-related configuration for Penfold.

Use subcommands to manage whitelists that control inbound ingestion and
outbound digest delivery.`,
	}

	whitelistCmd := newConfigEmailWhitelistCmd(deps)
	emailCmd.AddCommand(whitelistCmd)

	return emailCmd
}

func newConfigEmailWhitelistCmd(deps *PipelineCommandDeps) *cobra.Command {
	whitelistCmd := &cobra.Command{
		Use:   "whitelist",
		Short: "Manage email address whitelists",
		Long: `Manage inbound and outbound email address whitelists.

The inbound whitelist controls which sender addresses are allowed through
the email ingestion filter. Only emails from listed addresses are ingested.

The outbound whitelist controls which recipient addresses are allowed for
digest delivery. Only addresses on this list receive digests.

Both lists are stored as JSON arrays under operational config keys:
  email.inbound_whitelist  — senders allowed to submit content
  email.outbound_whitelist — recipients allowed to receive digests

Use 'list' to show both lists, 'add' to add an address, 'remove' to remove one.`,
	}

	whitelistCmd.AddCommand(newConfigEmailWhitelistListCmd(deps))
	whitelistCmd.AddCommand(newConfigEmailWhitelistAddCmd(deps))
	whitelistCmd.AddCommand(newConfigEmailWhitelistRemoveCmd(deps))

	// Default action: list both whitelists
	whitelistCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConfigEmailWhitelistList(cmd.Context(), deps)
	}

	return whitelistCmd
}

func newConfigEmailWhitelistListCmd(deps *PipelineCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show both inbound and outbound whitelists",
		Long: `Display all addresses on the inbound and outbound email whitelists.

The inbound whitelist controls which sender addresses can submit content via
email ingestion. The outbound whitelist controls which addresses receive digest
deliveries.

Use --format json to get machine-readable output when processing with scripts
or AI agents.`,
		Example: `  # Show both whitelists
  penf config email whitelist list

  # Machine-readable output
  penf config email whitelist list --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigEmailWhitelistList(cmd.Context(), deps)
		},
	}
}

func newConfigEmailWhitelistAddCmd(deps *PipelineCommandDeps) *cobra.Command {
	var inbound string
	var outbound string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an address to the inbound or outbound whitelist",
		Long: `Add an email address to the inbound or outbound whitelist.

Use --inbound to add a sender address that is allowed to submit content via
email ingestion. Use --outbound to add a recipient address that is allowed to
receive digest deliveries.

Exactly one of --inbound or --outbound must be provided. The address is
normalised to lowercase before storage. Adding a duplicate address is
idempotent — no error is returned and no duplicate is stored.`,
		Example: `  # Allow a sender address for email ingestion
  penf config email whitelist add --inbound alice@example.com

  # Allow a recipient address for digest delivery
  penf config email whitelist add --outbound bob@example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case inbound != "" && outbound != "":
				return fmt.Errorf("specify exactly one of --inbound or --outbound, not both")
			case inbound != "":
				return runConfigEmailWhitelistAdd(cmd.Context(), deps, keyInboundWhitelist, inbound)
			case outbound != "":
				return runConfigEmailWhitelistAdd(cmd.Context(), deps, keyOutboundWhitelist, outbound)
			default:
				return fmt.Errorf("one of --inbound or --outbound is required")
			}
		},
	}

	cmd.Flags().StringVar(&inbound, "inbound", "", "Sender address to add to the inbound whitelist")
	cmd.Flags().StringVar(&outbound, "outbound", "", "Recipient address to add to the outbound whitelist")

	return cmd
}

func newConfigEmailWhitelistRemoveCmd(deps *PipelineCommandDeps) *cobra.Command {
	var inbound string
	var outbound string

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove an address from the inbound or outbound whitelist",
		Long: `Remove an email address from the inbound or outbound whitelist.

Use --inbound to remove a sender address from the inbound ingestion whitelist.
Use --outbound to remove a recipient address from the outbound delivery whitelist.

Exactly one of --inbound or --outbound must be provided. Removing an address
that is not present succeeds silently with an informational message.`,
		Example: `  # Remove a sender from the inbound whitelist
  penf config email whitelist remove --inbound alice@example.com

  # Remove a recipient from the outbound whitelist
  penf config email whitelist remove --outbound bob@example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case inbound != "" && outbound != "":
				return fmt.Errorf("specify exactly one of --inbound or --outbound, not both")
			case inbound != "":
				return runConfigEmailWhitelistRemove(cmd.Context(), deps, keyInboundWhitelist, inbound)
			case outbound != "":
				return runConfigEmailWhitelistRemove(cmd.Context(), deps, keyOutboundWhitelist, outbound)
			default:
				return fmt.Errorf("one of --inbound or --outbound is required")
			}
		},
	}

	cmd.Flags().StringVar(&inbound, "inbound", "", "Sender address to remove from the inbound whitelist")
	cmd.Flags().StringVar(&outbound, "outbound", "", "Recipient address to remove from the outbound whitelist")

	return cmd
}

// validateEmailAddress returns an error if addr is not a plausible email address.
// It performs a lightweight check (must contain exactly one @) rather than full
// RFC 5322 parsing to keep error messages actionable.
func validateEmailAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("email address must not be empty")
	}
	parts := strings.Split(addr, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid email address %q: must contain exactly one @ with non-empty local part and domain", addr)
	}
	return nil
}

// normaliseEmail lowercases the address.
func normaliseEmail(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}

// parseWhitelist decodes a JSON string array. An empty or missing value
// returns an empty slice without error.
func parseWhitelist(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return []string{}, nil
	}
	var addrs []string
	if err := json.Unmarshal([]byte(value), &addrs); err != nil {
		return nil, fmt.Errorf("parsing whitelist JSON %q: %w", value, err)
	}
	return addrs, nil
}

// marshalWhitelist encodes a string slice as a compact JSON array.
func marshalWhitelist(addrs []string) (string, error) {
	b, err := json.Marshal(addrs)
	if err != nil {
		return "", fmt.Errorf("encoding whitelist: %w", err)
	}
	return string(b), nil
}

// addToWhitelist returns a new slice with addr appended, unless already present.
// Returns the new slice and a boolean indicating whether an addition occurred.
func addToWhitelist(addrs []string, addr string) ([]string, bool) {
	for _, a := range addrs {
		if a == addr {
			return addrs, false
		}
	}
	return append(addrs, addr), true
}

// removeFromWhitelist returns a new slice with addr removed.
// Returns the new slice and a boolean indicating whether a removal occurred.
func removeFromWhitelist(addrs []string, addr string) ([]string, bool) {
	result := make([]string, 0, len(addrs))
	found := false
	for _, a := range addrs {
		if a == addr {
			found = true
			continue
		}
		result = append(result, a)
	}
	return result, found
}

// connectAndGetClient sets up config and returns a connected PipelineServiceClient and close func.
func connectAndGetClient(deps *PipelineCommandDeps) (pipelinev1.PipelineServiceClient, func(), error) {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return nil, nil, err
	}

	client := pipelinev1.NewPipelineServiceClient(conn)
	return client, func() { conn.Close() }, nil
}

// fetchWhitelist retrieves the current whitelist for key from the gateway.
// Returns an empty slice if the key does not exist yet.
func fetchWhitelist(ctx context.Context, client pipelinev1.PipelineServiceClient, key string) ([]string, error) {
	resp, err := client.GetOperationalConfig(ctx, &pipelinev1.GetOperationalConfigRequest{Key: key})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.NotFound {
			return []string{}, nil
		}
		return nil, fmt.Errorf("fetching %s: %w", key, err)
	}
	return parseWhitelist(resp.GetValue())
}

// storeWhitelist persists the updated whitelist via SetOperationalConfig.
func storeWhitelist(ctx context.Context, client pipelinev1.PipelineServiceClient, key string, addrs []string) error {
	value, err := marshalWhitelist(addrs)
	if err != nil {
		return err
	}
	_, err = client.SetOperationalConfig(ctx, &pipelinev1.SetOperationalConfigRequest{
		Key:   key,
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("storing %s: %w", key, err)
	}
	return nil
}

func runConfigEmailWhitelistList(ctx context.Context, deps *PipelineCommandDeps) error {
	client, close, err := connectAndGetClient(deps)
	if err != nil {
		return err
	}
	defer close()

	inbound, err := fetchWhitelist(ctx, client, keyInboundWhitelist)
	if err != nil {
		return err
	}

	outbound, err := fetchWhitelist(ctx, client, keyOutboundWhitelist)
	if err != nil {
		return err
	}

	fmt.Println("Email Whitelists")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Println()

	fmt.Printf("Inbound (senders allowed for ingestion): %d address(es)\n", len(inbound))
	if len(inbound) == 0 {
		fmt.Println("  (none — all senders are blocked)")
	} else {
		for _, a := range inbound {
			fmt.Printf("  %s\n", a)
		}
	}
	fmt.Println()

	fmt.Printf("Outbound (recipients allowed for delivery): %d address(es)\n", len(outbound))
	if len(outbound) == 0 {
		fmt.Println("  (none — all delivery is blocked)")
	} else {
		for _, a := range outbound {
			fmt.Printf("  %s\n", a)
		}
	}

	return nil
}

func runConfigEmailWhitelistAdd(ctx context.Context, deps *PipelineCommandDeps, key, addr string) error {
	addr = normaliseEmail(addr)
	if err := validateEmailAddress(addr); err != nil {
		return err
	}

	client, close, err := connectAndGetClient(deps)
	if err != nil {
		return err
	}
	defer close()

	addrs, err := fetchWhitelist(ctx, client, key)
	if err != nil {
		return err
	}

	updated, added := addToWhitelist(addrs, addr)
	if !added {
		fmt.Printf("%s is already on the %s whitelist — no change made.\n", addr, friendlyKeyName(key))
		return nil
	}

	if err := storeWhitelist(ctx, client, key, updated); err != nil {
		return err
	}

	fmt.Printf("Added %s to the %s whitelist (%d address(es) total).\n", addr, friendlyKeyName(key), len(updated))
	return nil
}

func runConfigEmailWhitelistRemove(ctx context.Context, deps *PipelineCommandDeps, key, addr string) error {
	addr = normaliseEmail(addr)
	if err := validateEmailAddress(addr); err != nil {
		return err
	}

	client, close, err := connectAndGetClient(deps)
	if err != nil {
		return err
	}
	defer close()

	addrs, err := fetchWhitelist(ctx, client, key)
	if err != nil {
		return err
	}

	updated, removed := removeFromWhitelist(addrs, addr)
	if !removed {
		fmt.Printf("%s was not found on the %s whitelist — no change made.\n", addr, friendlyKeyName(key))
		return nil
	}

	if err := storeWhitelist(ctx, client, key, updated); err != nil {
		return err
	}

	fmt.Printf("Removed %s from the %s whitelist (%d address(es) remaining).\n", addr, friendlyKeyName(key), len(updated))
	return nil
}

// friendlyKeyName returns a human-readable name for a whitelist config key.
func friendlyKeyName(key string) string {
	switch key {
	case keyInboundWhitelist:
		return "inbound"
	case keyOutboundWhitelist:
		return "outbound"
	default:
		return key
	}
}
