// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"github.com/spf13/cobra"
)

// NewReprocessCommand creates a top-level reprocess command that wraps pipeline reprocess.
// This provides a shorter, more discoverable path for the common reprocessing workflow.
func NewReprocessCommand(deps interface{}) *cobra.Command {
	// Accept either PipelineCommandDeps or nil (same pattern as NewPipelineCommand)
	var pipelineDeps *PipelineCommandDeps
	if d, ok := deps.(*PipelineCommandDeps); ok && d != nil {
		pipelineDeps = d
	} else {
		pipelineDeps = DefaultPipelineDeps()
	}

	// Get the actual reprocess command implementation
	cmd := newPipelineReprocessCmd(pipelineDeps)

	// Update Use to remove parent prefix (was "reprocess <content-id>", keep it)
	cmd.Use = "reprocess <content-id>"

	// Update Long help text to mention this is a shortcut
	cmd.Long = `Trigger reprocessing of an already-processed content item.

This is a convenience shortcut for 'penf pipeline reprocess'.

This is useful for:
  - Re-running processing after model updates
  - Fixing content that failed during processing
  - Updating embeddings or summaries with new models

Stages that can be reprocessed:
  - embeddings: Regenerate vector embeddings
  - entities: Re-extract entities and structured data
  - keywords: Re-extract keywords
  - summary: Regenerate AI summaries

Processing Overrides:
  --timeout    Override timeout for this reprocessing run (seconds)
  --model      Override model ID for this reprocessing run

Examples:
  # Reprocess all stages for a content item
  penf reprocess content-123

  # Reprocess only embeddings
  penf reprocess content-123 --stage=embeddings

  # Reprocess with a reason
  penf reprocess content-123 --reason="Updated to new model"

  # Reprocess with custom model and timeout
  penf reprocess content-123 --model gemini-2.0-pro --timeout 300

  # Dry-run to see impact
  penf reprocess --stage triage --dry-run

  # Dry-run for specific source tag with overrides
  penf reprocess --stage triage --dry-run --source-tag gmail-import --timeout 120

Related Commands:
  penf pipeline reprocess  Full pipeline reprocess command (equivalent)
  penf content list        View content items that may need reprocessing
  penf workflow list       Monitor reprocessing workflow status`

	return cmd
}
