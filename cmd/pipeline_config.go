// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
)

func newPipelineConfigCmd(deps *PipelineCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage pipeline operational configuration",
		Long: `Manage runtime operational configuration for the pipeline.

View and update configuration values for timeouts, embedding, concurrency, and other pipeline parameters.

Examples:
  # List all operational config
  penf pipeline config

  # View embedding config
  penf pipeline config --key embedding

  # View activity timeouts
  penf pipeline config --key timeout.activity

  # Update a value
  penf pipeline config set embedding.chunk_max_tokens 512 --reason "Testing larger chunks"`,
	}

	cmd.AddCommand(newPipelineConfigListCmd(deps))
	cmd.AddCommand(newPipelineConfigSetCmd(deps))

	// Default action is list
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return newPipelineConfigListCmd(deps).RunE(cmd, args)
	}

	return cmd
}

func newPipelineConfigListCmd(deps *PipelineCommandDeps) *cobra.Command {
	var keyFilter string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List operational configuration entries",
		Long: `List all operational configuration entries or filter by key prefix.

Examples:
  # List all config
  penf pipeline config list

  # Filter by key prefix
  penf pipeline config list --key embedding
  penf pipeline config list --key timeout.activity

  # Output as JSON
  penf pipeline config list -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineConfigList(cmd.Context(), deps, keyFilter, outputFormat)
		},
	}

	cmd.Flags().StringVar(&keyFilter, "key", "", "Filter by key prefix (e.g., 'timeout.activity')")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func newPipelineConfigSetCmd(deps *PipelineCommandDeps) *cobra.Command {
	var reason string
	var updatedBy string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Update an operational configuration value",
		Long: `Update an operational configuration value.

Examples:
  # Set embedding chunk size
  penf pipeline config set embedding.chunk_max_tokens 512 --reason "Testing larger chunks"

  # Set AI client timeout
  penf pipeline config set timeout.ai_client.request 180s --reason "Increased for longer requests"

  # Set with custom updated_by
  penf pipeline config set timeout.http.backend.gemini 2m --reason "Gemini timeout" --updated-by "admin@example.com"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]
			return runPipelineConfigSet(cmd.Context(), deps, key, value, reason, updatedBy, outputFormat)
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Reason for the change")
	cmd.Flags().StringVar(&updatedBy, "updated-by", "", "User making the change (default: CLI user)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json")

	return cmd
}

func runPipelineConfigList(ctx context.Context, deps *PipelineCommandDeps, keyFilter string, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.GetTimeoutConfig(ctx, &pipelinev1.GetTimeoutConfigRequest{
		Key: keyFilter,
	})
	if err != nil {
		return fmt.Errorf("getting timeout config: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Entries)
	}

	return outputTimeoutConfigListHuman(resp.Entries, keyFilter)
}

func runPipelineConfigSet(ctx context.Context, deps *PipelineCommandDeps, key string, value string, reason string, updatedBy string, outputFormat string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Default updatedBy to CLI user or "cli"
	if updatedBy == "" {
		updatedBy = os.Getenv("USER")
		if updatedBy == "" {
			updatedBy = "cli"
		}
	}

	if reason == "" {
		reason = "CLI update"
	}

	conn, err := connectPipelineToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pipelinev1.NewPipelineServiceClient(conn)

	resp, err := client.UpdateTimeoutConfig(ctx, &pipelinev1.UpdateTimeoutConfigRequest{
		Key:       key,
		Value:     value,
		UpdatedBy: updatedBy,
		Reason:    reason,
	})
	if err != nil {
		return fmt.Errorf("updating timeout config: %w", err)
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	return outputTimeoutConfigUpdateHuman(resp)
}

func outputTimeoutConfigListHuman(entries []*pipelinev1.TimeoutEntry, keyFilter string) error {
	if len(entries) == 0 {
		if keyFilter != "" {
			fmt.Printf("No config entries found matching key '%s'.\n", keyFilter)
		} else {
			fmt.Println("No config entries found.")
		}
		return nil
	}

	// Sort entries by key
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	// Group by prefix for better readability
	groups := make(map[string][]*pipelinev1.TimeoutEntry)
	for _, entry := range entries {
		// Extract prefix (e.g., "timeout.activity", "timeout.http")
		parts := strings.Split(entry.Key, ".")
		prefix := strings.Join(parts[:min(2, len(parts))], ".")
		groups[prefix] = append(groups[prefix], entry)
	}

	// Sort group keys
	var groupKeys []string
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)

	// Display each group
	fmt.Println("Pipeline Operational Configuration")
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println()

	for _, groupKey := range groupKeys {
		fmt.Printf("%s\n", groupKey)
		fmt.Println(strings.Repeat("-", len(groupKey)))

		for _, entry := range groups[groupKey] {
			// Format key (remove common prefix for cleaner display)
			displayKey := strings.TrimPrefix(entry.Key, groupKey+".")
			if displayKey == entry.Key {
				displayKey = entry.Key // Keep full key if prefix removal didn't work
			}

			// Color based on whether it's default or modified
			valueColor := ""
			if entry.Value != entry.DefaultValue {
				valueColor = "\033[33m" // Yellow for modified values
			}

			fmt.Printf("  %-40s %s%-10s\033[0m  [%s - %s]\n",
				displayKey,
				valueColor,
				entry.Value,
				entry.MinValue,
				entry.MaxValue)

			// Show description if verbose
			if entry.Description != "" {
				fmt.Printf("    %s\n", entry.Description)
			}

			// Show last update info if modified
			if entry.Value != entry.DefaultValue && entry.UpdatedBy != "" {
				fmt.Printf("    \033[90mUpdated by %s at %s\033[0m\n", entry.UpdatedBy, entry.UpdatedAt)
			}

			fmt.Println()
		}
	}

	fmt.Printf("Total: %d configuration entr%s\n", len(entries), func() string { if len(entries) == 1 { return "y" }; return "ies" }())
	if keyFilter != "" {
		fmt.Printf("(filtered by key: %s)\n", keyFilter)
	}

	return nil
}

func outputTimeoutConfigUpdateHuman(resp *pipelinev1.UpdateTimeoutConfigResponse) error {
	fmt.Printf("✓ %s\n\n", resp.Message)

	entry := resp.Entry
	if entry != nil {
		fmt.Println("Updated Configuration:")
		fmt.Printf("  Key:          %s\n", entry.Key)
		fmt.Printf("  Previous:     %s\n", resp.PreviousValue)
		fmt.Printf("  New Value:    %s\n", entry.Value)
		fmt.Printf("  Default:      %s\n", entry.DefaultValue)
		fmt.Printf("  Range:        %s - %s\n", entry.MinValue, entry.MaxValue)
		fmt.Printf("  Updated By:   %s\n", entry.UpdatedBy)
		fmt.Printf("  Updated At:   %s\n", entry.UpdatedAt)
		fmt.Println()
		fmt.Println("Note: Configuration changes take effect immediately for new workflow activities.")
		fmt.Println("Existing activities will continue using their initial timeout values.")
	}

	return nil
}

func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
