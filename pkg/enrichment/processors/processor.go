// Package processors defines the interfaces and common types for enrichment processors.
package processors

import (
	"context"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

// Source represents the raw content to be enriched.
// This matches the structure from pkg/ingest/storage.
type Source struct {
	ID              int64
	TenantID        string
	SourceSystem    string // manual_eml, gmail, etc.
	ExternalID      string // Message-ID
	ContentHash     string
	RawContent      string
	ContentType     string
	ContentSize     int32
	Metadata        map[string]interface{}
	SourceTimestamp interface{} // time.Time
}

// ProcessorContext provides context for processor execution.
type ProcessorContext struct {
	Source     *Source
	Enrichment *enrichment.Enrichment
	TenantID   string
	Logger     logging.Logger
}

// Processor is the interface that all enrichment processors must implement.
type Processor interface {
	// Name returns the unique processor name for registry and logging.
	Name() string

	// Stage returns which pipeline stage this processor runs in.
	Stage() Stage

	// Process executes the processor logic.
	// It should modify the enrichment in-place and return any error.
	Process(ctx context.Context, pctx *ProcessorContext) error
}

// Stage represents a pipeline stage.
type Stage string

const (
	StageClassification    Stage = "classification"
	StageCommonEnrichment  Stage = "common_enrichment"
	StageTypeSpecific      Stage = "type_specific"
	StageAIRouting         Stage = "ai_routing"
	StageAIProcessing      Stage = "ai_processing"
	StagePostProcessing    Stage = "post_processing"
)

// ClassificationProcessor specifically handles content classification (Stage 1).
type ClassificationProcessor interface {
	Processor

	// Classify determines the content type, subtype, and processing profile.
	Classify(ctx context.Context, source *Source) (*enrichment.Classification, error)
}

// CommonEnrichmentProcessor runs for all content types (Stage 2).
type CommonEnrichmentProcessor interface {
	Processor

	// CanProcess returns true if this processor should run for the given classification.
	// Most common processors always return true.
	CanProcess(classification *enrichment.Classification) bool
}

// TypeSpecificProcessor runs based on content subtype (Stage 3).
type TypeSpecificProcessor interface {
	Processor

	// Subtypes returns the content subtypes this processor handles.
	Subtypes() []enrichment.ContentSubtype

	// Extract performs type-specific extraction.
	// Results should be stored in enrichment.ExtractedData.
	Extract(ctx context.Context, pctx *ProcessorContext) error
}

// AIProcessor handles AI-based processing (Stage 5).
type AIProcessor interface {
	Processor

	// ShouldProcess returns true if AI processing should run for this enrichment.
	ShouldProcess(enrichment *enrichment.Enrichment) bool
}

// PostProcessor handles post-AI processing (Stage 6).
// These processors run after AI processing to extract derived data like mentions.
type PostProcessor interface {
	Processor

	// ShouldProcess returns true if post-processing should run for this enrichment.
	ShouldProcess(enrichment *enrichment.Enrichment) bool
}

// ProcessorRegistry manages processor registration and lookup.
type ProcessorRegistry interface {
	// Register adds a processor to the registry.
	Register(p Processor) error

	// GetByStage returns all processors for a stage in execution order.
	GetByStage(stage Stage) []Processor

	// GetByName returns a processor by name.
	GetByName(name string) (Processor, bool)

	// GetTypeSpecificProcessor returns the processor for a content subtype.
	GetTypeSpecificProcessor(subtype enrichment.ContentSubtype) (TypeSpecificProcessor, bool)

	// All returns all registered processors.
	All() []Processor
}

// ProcessorResult captures the outcome of a processor execution.
type ProcessorResult struct {
	ProcessorName string
	Stage         Stage
	Success       bool
	Error         error
	DurationMs    int64
	OutputData    map[string]interface{}
}
