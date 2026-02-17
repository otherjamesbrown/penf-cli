package processors

import (
	"fmt"
	"sync"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
)

// Registry is the default implementation of ProcessorRegistry.
type Registry struct {
	mu         sync.RWMutex
	processors map[string]Processor
	byStage    map[Stage][]Processor
	bySubtype  map[enrichment.ContentSubtype]TypeSpecificProcessor
	order      []string // Maintains registration order
}

// NewRegistry creates a new processor registry.
func NewRegistry() *Registry {
	return &Registry{
		processors: make(map[string]Processor),
		byStage:    make(map[Stage][]Processor),
		bySubtype:  make(map[enrichment.ContentSubtype]TypeSpecificProcessor),
		order:      make([]string, 0),
	}
}

// Register adds a processor to the registry.
func (r *Registry) Register(p Processor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.processors[name]; exists {
		return fmt.Errorf("processor already registered: %s", name)
	}

	r.processors[name] = p
	r.order = append(r.order, name)

	// Index by stage
	stage := p.Stage()
	r.byStage[stage] = append(r.byStage[stage], p)

	// Index type-specific processors by subtype
	if tsp, ok := p.(TypeSpecificProcessor); ok {
		for _, subtype := range tsp.Subtypes() {
			if existing, exists := r.bySubtype[subtype]; exists {
				return fmt.Errorf("subtype %s already has processor %s, cannot register %s",
					subtype, existing.Name(), name)
			}
			r.bySubtype[subtype] = tsp
		}
	}

	return nil
}

// GetByStage returns all processors for a stage in execution order.
func (r *Registry) GetByStage(stage Stage) []Processor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	processors := r.byStage[stage]
	result := make([]Processor, len(processors))
	copy(result, processors)
	return result
}

// GetByName returns a processor by name.
func (r *Registry) GetByName(name string) (Processor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.processors[name]
	return p, ok
}

// GetTypeSpecificProcessor returns the processor for a content subtype.
func (r *Registry) GetTypeSpecificProcessor(subtype enrichment.ContentSubtype) (TypeSpecificProcessor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.bySubtype[subtype]
	return p, ok
}

// All returns all registered processors in registration order.
func (r *Registry) All() []Processor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Processor, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.processors[name])
	}
	return result
}

// StageOrder returns the canonical order of pipeline stages.
func StageOrder() []Stage {
	return []Stage{
		StageClassification,
		StageCommonEnrichment,
		StageTypeSpecific,
		StageAIRouting,
		StageAIProcessing,
		StagePostProcessing,
	}
}

// Verify interface compliance
var _ ProcessorRegistry = (*Registry)(nil)
