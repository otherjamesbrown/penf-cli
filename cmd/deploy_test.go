package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestDeployCommand(t *testing.T) {
	deployCmd := NewDeployCommand()

	if deployCmd == nil {
		t.Fatal("NewDeployCommand() returned nil")
	}

	if deployCmd.Use != "deploy [gateway|worker|ai|all]" {
		t.Errorf("Unexpected Use: %s", deployCmd.Use)
	}

	if deployCmd.Short != "Build, upload, and deploy services via Nomad" {
		t.Errorf("Unexpected Short: %s", deployCmd.Short)
	}
}

func TestDeployHistorySubcommand(t *testing.T) {
	deployCmd := NewDeployCommand()

	// Find the history subcommand.
	var historyCmd *cobra.Command
	for _, cmd := range deployCmd.Commands() {
		if cmd.Use == "history [service]" {
			historyCmd = cmd
			break
		}
	}

	if historyCmd == nil {
		t.Fatal("history subcommand not found")
	}

	if historyCmd.Short != "Show deployment history" {
		t.Errorf("Unexpected Short: %s", historyCmd.Short)
	}

	// Verify flags exist.
	lastFlag := historyCmd.Flags().Lookup("last")
	if lastFlag == nil {
		t.Error("--last flag not found")
	}
}

func TestDeployStatusFlag(t *testing.T) {
	deployCmd := NewDeployCommand()

	statusFlag := deployCmd.Flags().Lookup("status")
	if statusFlag == nil {
		t.Error("--status flag not found")
	}
}

func TestDeployRecordSubcommand(t *testing.T) {
	deployCmd := NewDeployCommand()

	// Find the record subcommand.
	var recordCmd *cobra.Command
	for _, cmd := range deployCmd.Commands() {
		if cmd.Use == "record <service>" {
			recordCmd = cmd
			break
		}
	}

	if recordCmd == nil {
		t.Fatal("record subcommand not found")
	}

	if recordCmd.Short != "Record a deployment in deploy_history" {
		t.Errorf("Unexpected Short: %s", recordCmd.Short)
	}

	// Verify all expected flags exist.
	expectedFlags := []string{
		"commit",
		"previous-commit",
		"deployed-by",
		"version",
		"changes",
		"shard-ids",
		"notify",
	}
	for _, name := range expectedFlags {
		if recordCmd.Flags().Lookup(name) == nil {
			t.Errorf("--%s flag not found", name)
		}
	}
}

func TestDeployRecordRequiresCommit(t *testing.T) {
	deployCmd := NewDeployCommand()

	// Find the record subcommand.
	var recordCmd *cobra.Command
	for _, cmd := range deployCmd.Commands() {
		if cmd.Use == "record <service>" {
			recordCmd = cmd
			break
		}
	}

	if recordCmd == nil {
		t.Fatal("record subcommand not found")
	}

	// --commit should be marked as required.
	commitFlag := recordCmd.Flags().Lookup("commit")
	if commitFlag == nil {
		t.Fatal("--commit flag not found")
	}

	// Verify it's annotated as required by Cobra.
	annotations := recordCmd.Flags().Lookup("commit").Annotations
	if annotations == nil {
		t.Fatal("--commit flag has no annotations (expected cobra required annotation)")
	}
	if _, ok := annotations[cobra.BashCompOneRequiredFlag]; !ok {
		t.Error("--commit flag not marked as required")
	}
}

func TestDeployRecordNotifyDefaultTrue(t *testing.T) {
	deployCmd := NewDeployCommand()

	// Find the record subcommand.
	var recordCmd *cobra.Command
	for _, cmd := range deployCmd.Commands() {
		if cmd.Use == "record <service>" {
			recordCmd = cmd
			break
		}
	}

	if recordCmd == nil {
		t.Fatal("record subcommand not found")
	}

	notifyFlag := recordCmd.Flags().Lookup("notify")
	if notifyFlag == nil {
		t.Fatal("--notify flag not found")
	}

	if notifyFlag.DefValue != "true" {
		t.Errorf("--notify default should be true, got %s", notifyFlag.DefValue)
	}
}

func TestDeployRecordExactArgs(t *testing.T) {
	deployCmd := NewDeployCommand()

	// Find the record subcommand.
	var recordCmd *cobra.Command
	for _, cmd := range deployCmd.Commands() {
		if cmd.Use == "record <service>" {
			recordCmd = cmd
			break
		}
	}

	if recordCmd == nil {
		t.Fatal("record subcommand not found")
	}

	// Verify the Args validator is set to ExactArgs(1) by checking
	// that it rejects wrong argument counts.
	if recordCmd.Args == nil {
		t.Fatal("Args validator not set on record subcommand")
	}

	// ExactArgs(1) should reject 0 args.
	if err := recordCmd.Args(recordCmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}

	// ExactArgs(1) should accept 1 arg.
	if err := recordCmd.Args(recordCmd, []string{"penfold-gateway"}); err != nil {
		t.Errorf("expected no error for 1 arg, got: %v", err)
	}

	// ExactArgs(1) should reject 2 args.
	if err := recordCmd.Args(recordCmd, []string{"svc1", "svc2"}); err == nil {
		t.Error("expected error for 2 args")
	}
}
