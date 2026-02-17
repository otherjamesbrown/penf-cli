package processors

import (
	"context"
	"testing"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
)

// mockProcessor is a test processor implementation.
type mockProcessor struct {
	name  string
	stage Stage
}

func (m *mockProcessor) Name() string  { return m.name }
func (m *mockProcessor) Stage() Stage  { return m.stage }
func (m *mockProcessor) Process(ctx context.Context, pctx *ProcessorContext) error {
	return nil
}

// mockTypeSpecificProcessor is a test type-specific processor.
type mockTypeSpecificProcessor struct {
	name     string
	subtypes []enrichment.ContentSubtype
}

func (m *mockTypeSpecificProcessor) Name() string                    { return m.name }
func (m *mockTypeSpecificProcessor) Stage() Stage                    { return StageTypeSpecific }
func (m *mockTypeSpecificProcessor) Subtypes() []enrichment.ContentSubtype { return m.subtypes }
func (m *mockTypeSpecificProcessor) Process(ctx context.Context, pctx *ProcessorContext) error {
	return nil
}
func (m *mockTypeSpecificProcessor) Extract(ctx context.Context, pctx *ProcessorContext) error {
	return nil
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	proc := &mockProcessor{name: "TestProcessor", stage: StageClassification}
	if err := r.Register(proc); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Verify registration
	got, ok := r.GetByName("TestProcessor")
	if !ok {
		t.Fatal("GetByName() did not find registered processor")
	}
	if got.Name() != "TestProcessor" {
		t.Errorf("GetByName() returned processor with name %q, want %q", got.Name(), "TestProcessor")
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()

	proc := &mockProcessor{name: "TestProcessor", stage: StageClassification}
	if err := r.Register(proc); err != nil {
		t.Fatalf("First Register() error = %v", err)
	}

	// Try to register the same processor again
	if err := r.Register(proc); err == nil {
		t.Fatal("Register() should return error for duplicate processor")
	}
}

func TestRegistry_GetByStage(t *testing.T) {
	r := NewRegistry()

	// Register processors in different stages
	classProc := &mockProcessor{name: "Classifier", stage: StageClassification}
	enrichProc1 := &mockProcessor{name: "Enricher1", stage: StageCommonEnrichment}
	enrichProc2 := &mockProcessor{name: "Enricher2", stage: StageCommonEnrichment}

	r.Register(classProc)
	r.Register(enrichProc1)
	r.Register(enrichProc2)

	// Check classification stage
	classProcs := r.GetByStage(StageClassification)
	if len(classProcs) != 1 {
		t.Errorf("GetByStage(Classification) returned %d processors, want 1", len(classProcs))
	}

	// Check common enrichment stage
	enrichProcs := r.GetByStage(StageCommonEnrichment)
	if len(enrichProcs) != 2 {
		t.Errorf("GetByStage(CommonEnrichment) returned %d processors, want 2", len(enrichProcs))
	}

	// Check empty stage
	aiProcs := r.GetByStage(StageAIProcessing)
	if len(aiProcs) != 0 {
		t.Errorf("GetByStage(AIProcessing) returned %d processors, want 0", len(aiProcs))
	}
}

func TestRegistry_GetTypeSpecificProcessor(t *testing.T) {
	r := NewRegistry()

	jiraProc := &mockTypeSpecificProcessor{
		name:     "JiraExtractor",
		subtypes: []enrichment.ContentSubtype{enrichment.SubtypeNotificationJira},
	}
	r.Register(jiraProc)

	// Check registered subtype
	proc, ok := r.GetTypeSpecificProcessor(enrichment.SubtypeNotificationJira)
	if !ok {
		t.Fatal("GetTypeSpecificProcessor() did not find processor for notification/jira")
	}
	if proc.Name() != "JiraExtractor" {
		t.Errorf("GetTypeSpecificProcessor() returned %q, want %q", proc.Name(), "JiraExtractor")
	}

	// Check unregistered subtype
	_, ok = r.GetTypeSpecificProcessor(enrichment.SubtypeEmailStandalone)
	if ok {
		t.Error("GetTypeSpecificProcessor() should return false for unregistered subtype")
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()

	proc1 := &mockProcessor{name: "First", stage: StageClassification}
	proc2 := &mockProcessor{name: "Second", stage: StageCommonEnrichment}
	proc3 := &mockProcessor{name: "Third", stage: StageTypeSpecific}

	r.Register(proc1)
	r.Register(proc2)
	r.Register(proc3)

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("All() returned %d processors, want 3", len(all))
	}

	// Verify order is preserved
	names := []string{all[0].Name(), all[1].Name(), all[2].Name()}
	expected := []string{"First", "Second", "Third"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("All()[%d].Name() = %q, want %q", i, name, expected[i])
		}
	}
}

func TestStageOrder(t *testing.T) {
	order := StageOrder()
	if len(order) != 6 {
		t.Fatalf("StageOrder() returned %d stages, want 6", len(order))
	}

	expected := []Stage{
		StageClassification,
		StageCommonEnrichment,
		StageTypeSpecific,
		StageAIRouting,
		StageAIProcessing,
		StagePostProcessing,
	}

	for i, stage := range order {
		if stage != expected[i] {
			t.Errorf("StageOrder()[%d] = %q, want %q", i, stage, expected[i])
		}
	}
}
