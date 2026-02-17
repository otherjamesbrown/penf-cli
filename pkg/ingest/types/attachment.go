// Package types provides shared types for the ingest pipeline.
package types

import (
	"time"
)

// ProcessingTier represents the classification tier for an attachment.
type ProcessingTier string

const (
	// TierAutoProcess indicates high-value attachments that should be processed automatically.
	TierAutoProcess ProcessingTier = "auto_process"

	// TierAutoSkip indicates low-value attachments that should be skipped.
	TierAutoSkip ProcessingTier = "auto_skip"

	// TierPendingReview indicates uncertain attachments that need manual triage.
	TierPendingReview ProcessingTier = "pending_review"

	// TierManualProcess indicates user has marked this for processing.
	TierManualProcess ProcessingTier = "manual_process"

	// TierManualSkip indicates user has marked this to skip.
	TierManualSkip ProcessingTier = "manual_skip"
)

// ProcessingStep records a single step in the classification pipeline.
type ProcessingStep struct {
	Step       string         `json:"step"`
	Result     ProcessingTier `json:"result"`
	Reason     string         `json:"reason"`
	Confidence float64        `json:"confidence"`
	Timestamp  time.Time      `json:"ts"`
}

// IsProcessable returns true if the tier indicates the attachment should be processed.
func (t ProcessingTier) IsProcessable() bool {
	return t == TierAutoProcess || t == TierManualProcess
}

// IsSkipped returns true if the tier indicates the attachment should be skipped.
func (t ProcessingTier) IsSkipped() bool {
	return t == TierAutoSkip || t == TierManualSkip
}

// IsPending returns true if the tier indicates the attachment needs review.
func (t ProcessingTier) IsPending() bool {
	return t == TierPendingReview
}

// NewProcessingStep creates a new processing step record.
func NewProcessingStep(step string, result ProcessingTier, reason string, confidence float64) ProcessingStep {
	return ProcessingStep{
		Step:       step,
		Result:     result,
		Reason:     reason,
		Confidence: confidence,
		Timestamp:  time.Now(),
	}
}
