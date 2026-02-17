package resolver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/otherjamesbrown/penf-cli/pkg/mentions"
)

// MockLLMProvider implements LLMProvider for testing.
type MockLLMProvider struct {
	mock.Mock
}

func (m *MockLLMProvider) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockLLMProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CompletionResponse), args.Error(1)
}

func (m *MockLLMProvider) CompleteStructured(ctx context.Context, req CompletionRequest, target interface{}) error {
	args := m.Called(ctx, req, target)
	return args.Error(0)
}

func (m *MockLLMProvider) IsAvailable(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

func (m *MockLLMProvider) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockTracer implements Tracer for testing.
type MockTracer struct {
	mock.Mock
}

func (m *MockTracer) StartTrace(tenantID string, contentID int64, contentType, contentSummary string, model string, level TraceLevel, config map[string]interface{}) (string, error) {
	args := m.Called(tenantID, contentID, contentType, contentSummary, model, level, config)
	return args.String(0), args.Error(1)
}

func (m *MockTracer) RecordStageStart(traceID string, stageNum int, stageName StageName, inputSummary string, inputData interface{}) (int64, error) {
	args := m.Called(traceID, stageNum, stageName, inputSummary, inputData)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTracer) RecordStageComplete(stageID int64, outputSummary string, outputData interface{}) error {
	args := m.Called(stageID, outputSummary, outputData)
	return args.Error(0)
}

func (m *MockTracer) RecordStageFailed(stageID int64, errorMsg string) error {
	args := m.Called(stageID, errorMsg)
	return args.Error(0)
}

func (m *MockTracer) RecordStageSkipped(stageID int64, reason string) error {
	args := m.Called(stageID, reason)
	return args.Error(0)
}

func (m *MockTracer) RecordLLMCall(traceID string, stageID int64, model, promptTemplate, promptText string, promptTokens int, responseText string, responseTokens int, parsedOutput interface{}, parseErrors []string, latencyMs, attemptNumber int, isFallback bool, fallbackReason string) error {
	args := m.Called(traceID, stageID, model, promptTemplate, promptText, promptTokens, responseText, responseTokens, parsedOutput, parseErrors, latencyMs, attemptNumber, isFallback, fallbackReason)
	return args.Error(0)
}

func (m *MockTracer) RecordDecision(traceID string, stageID int64, decisionType DecisionType, mentionID *int64, mentionText, chosenOption string, alternatives interface{}, confidence float32, reasoning string, factors interface{}) error {
	args := m.Called(traceID, stageID, decisionType, mentionID, mentionText, chosenOption, alternatives, confidence, reasoning, factors)
	return args.Error(0)
}

func (m *MockTracer) CompleteTrace(traceID string, mentionsFound, autoResolved, queuedForReview, newEntitiesSuggested int) error {
	args := m.Called(traceID, mentionsFound, autoResolved, queuedForReview, newEntitiesSuggested)
	return args.Error(0)
}

func (m *MockTracer) FailTrace(traceID string, errorMsg string) error {
	args := m.Called(traceID, errorMsg)
	return args.Error(0)
}

// MockEntityLookup implements mentions.EntityLookup for testing.
type MockEntityLookup struct {
	mock.Mock
}

func (m *MockEntityLookup) LookupPerson(ctx context.Context, tenantID, name string) ([]mentions.Candidate, error) {
	args := m.Called(ctx, tenantID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]mentions.Candidate), args.Error(1)
}

func (m *MockEntityLookup) LookupTerm(ctx context.Context, tenantID, text string) ([]mentions.Candidate, error) {
	args := m.Called(ctx, tenantID, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]mentions.Candidate), args.Error(1)
}

func (m *MockEntityLookup) LookupProduct(ctx context.Context, tenantID, text string) ([]mentions.Candidate, error) {
	args := m.Called(ctx, tenantID, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]mentions.Candidate), args.Error(1)
}

func (m *MockEntityLookup) LookupCompany(ctx context.Context, tenantID, text string) ([]mentions.Candidate, error) {
	args := m.Called(ctx, tenantID, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]mentions.Candidate), args.Error(1)
}

func (m *MockEntityLookup) LookupProject(ctx context.Context, tenantID, text string) ([]mentions.Candidate, error) {
	args := m.Called(ctx, tenantID, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]mentions.Candidate), args.Error(1)
}

func (m *MockEntityLookup) GetEntityName(ctx context.Context, entityType mentions.EntityType, entityID int64) (string, error) {
	args := m.Called(ctx, entityType, entityID)
	return args.String(0), args.Error(1)
}

// MockRepository implements mentions.Repository for testing.
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateMention(ctx context.Context, input mentions.MentionInput) (*mentions.ContentMention, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mentions.ContentMention), args.Error(1)
}

func (m *MockRepository) GetMention(ctx context.Context, id int64) (*mentions.ContentMention, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mentions.ContentMention), args.Error(1)
}

func (m *MockRepository) ListMentions(ctx context.Context, filter mentions.MentionFilter) ([]mentions.ContentMention, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]mentions.ContentMention), args.Error(1)
}

func (m *MockRepository) UpdateMentionResolution(ctx context.Context, id int64, resolution mentions.ResolutionInput) error {
	args := m.Called(ctx, id, resolution)
	return args.Error(0)
}

func (m *MockRepository) DismissMention(ctx context.Context, id int64, dismissal mentions.DismissalInput) error {
	args := m.Called(ctx, id, dismissal)
	return args.Error(0)
}

func (m *MockRepository) GetPattern(ctx context.Context, tenantID string, entityType mentions.EntityType, text string, projectID *int64) (*mentions.MentionPattern, error) {
	args := m.Called(ctx, tenantID, entityType, text, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mentions.MentionPattern), args.Error(1)
}

func (m *MockRepository) GetPatternsByText(ctx context.Context, tenantID string, entityType mentions.EntityType, text string) ([]mentions.MentionPattern, error) {
	args := m.Called(ctx, tenantID, entityType, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]mentions.MentionPattern), args.Error(1)
}

func (m *MockRepository) CreateOrUpdatePattern(ctx context.Context, pattern *mentions.MentionPattern) error {
	args := m.Called(ctx, pattern)
	return args.Error(0)
}

func (m *MockRepository) IncrementPatternSeen(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) IncrementPatternLinked(ctx context.Context, id int64, entityID int64) error {
	args := m.Called(ctx, id, entityID)
	return args.Error(0)
}

func (m *MockRepository) GetAffinity(ctx context.Context, tenantID string, entityType mentions.EntityType, entityID, projectID int64) (*mentions.EntityProjectAffinity, error) {
	args := m.Called(ctx, tenantID, entityType, entityID, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mentions.EntityProjectAffinity), args.Error(1)
}

func (m *MockRepository) GetAffinitiesForProject(ctx context.Context, tenantID string, projectID int64, entityType mentions.EntityType) ([]mentions.EntityProjectAffinity, error) {
	args := m.Called(ctx, tenantID, projectID, entityType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]mentions.EntityProjectAffinity), args.Error(1)
}

func (m *MockRepository) GetAffinitiesForEntity(ctx context.Context, tenantID string, entityType mentions.EntityType, entityID int64) ([]mentions.EntityProjectAffinity, error) {
	args := m.Called(ctx, tenantID, entityType, entityID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]mentions.EntityProjectAffinity), args.Error(1)
}

func (m *MockRepository) UpsertAffinity(ctx context.Context, affinity *mentions.EntityProjectAffinity) error {
	args := m.Called(ctx, affinity)
	return args.Error(0)
}

func (m *MockRepository) IncrementAffinityMentionCount(ctx context.Context, tenantID string, entityType mentions.EntityType, entityID, projectID int64) error {
	args := m.Called(ctx, tenantID, entityType, entityID, projectID)
	return args.Error(0)
}

func (m *MockRepository) GetMentionStats(ctx context.Context, tenantID string) (*mentions.MentionStats, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mentions.MentionStats), args.Error(1)
}

func (m *MockRepository) GetPendingCount(ctx context.Context, tenantID string, entityType *mentions.EntityType) (int, error) {
	args := m.Called(ctx, tenantID, entityType)
	return args.Int(0), args.Error(1)
}

func (m *MockRepository) BatchCreateMentions(ctx context.Context, inputs []mentions.MentionInput) ([]mentions.ContentMention, error) {
	args := m.Called(ctx, inputs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]mentions.ContentMention), args.Error(1)
}

func (m *MockRepository) BatchResolveMentions(ctx context.Context, resolutions []mentions.ResolutionInput) (*mentions.BatchResolutionResult, error) {
	args := m.Called(ctx, resolutions)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mentions.BatchResolutionResult), args.Error(1)
}

// TestConfig tests

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "mlx", cfg.LLM.Provider)
	assert.Equal(t, "mistral-7b-instruct-v0.2", cfg.LLM.Model)
	assert.Equal(t, "http://localhost:8081", cfg.LLM.BaseURL)
	assert.Equal(t, float32(0.8), cfg.Thresholds.AutoResolve)
	assert.Equal(t, float32(0.9), cfg.Thresholds.Verification)
	assert.Equal(t, float32(0.7), cfg.Thresholds.Suggest)
	assert.Equal(t, 50, cfg.MaxMentionsPerBatch)
	assert.Equal(t, TraceLevelStandard, cfg.TraceLevel)
}

func TestConfigValidate_FillsDefaults(t *testing.T) {
	cfg := Config{}
	err := cfg.Validate()

	require.NoError(t, err)
	assert.Equal(t, "mlx", cfg.LLM.Provider)
	assert.Equal(t, "mistral-7b-instruct-v0.2", cfg.LLM.Model)
	assert.Equal(t, float32(0.8), cfg.Thresholds.AutoResolve)
}

func TestConfigValidate_PreservesValues(t *testing.T) {
	cfg := Config{
		LLM: LLMConfig{
			Provider: "claude",
			Model:    "claude-3-sonnet",
			BaseURL:  "https://api.anthropic.com",
		},
		Thresholds: ThresholdConfig{
			AutoResolve:  0.9,
			Verification: 0.95,
			Suggest:      0.8,
		},
	}

	err := cfg.Validate()

	require.NoError(t, err)
	assert.Equal(t, "claude", cfg.LLM.Provider)
	assert.Equal(t, "claude-3-sonnet", cfg.LLM.Model)
	assert.Equal(t, float32(0.9), cfg.Thresholds.AutoResolve)
}

// TestProviderRegistry tests

func TestProviderRegistry_RegisterAndGet(t *testing.T) {
	reg := NewProviderRegistry()
	mockProvider := new(MockLLMProvider)

	reg.Register("test", mockProvider)
	provider, ok := reg.Get("test")

	assert.True(t, ok)
	assert.Equal(t, mockProvider, provider)
}

func TestProviderRegistry_GetMissing(t *testing.T) {
	reg := NewProviderRegistry()

	provider, ok := reg.Get("nonexistent")

	assert.False(t, ok)
	assert.Nil(t, provider)
}

func TestProviderRegistry_PrimaryAndFallback(t *testing.T) {
	reg := NewProviderRegistry()
	primary := new(MockLLMProvider)
	fallback := new(MockLLMProvider)

	reg.Register("primary", primary)
	reg.Register("fallback", fallback)
	reg.SetPrimary("primary")
	reg.SetFallback("fallback")

	p, ok := reg.Primary()
	assert.True(t, ok)
	assert.Equal(t, primary, p)

	f, ok := reg.Fallback()
	assert.True(t, ok)
	assert.Equal(t, fallback, f)
}

func TestProviderRegistry_Close(t *testing.T) {
	reg := NewProviderRegistry()
	mock1 := new(MockLLMProvider)
	mock2 := new(MockLLMProvider)

	mock1.On("Close").Return(nil)
	mock2.On("Close").Return(nil)

	reg.Register("p1", mock1)
	reg.Register("p2", mock2)

	err := reg.Close()

	assert.NoError(t, err)
	mock1.AssertExpectations(t)
	mock2.AssertExpectations(t)
}

// TestTypes tests

func TestLLMError(t *testing.T) {
	err := &LLMError{
		Code:    ErrTimeout,
		Message: "request timed out",
	}

	assert.Equal(t, "request timed out", err.Error())
	assert.Equal(t, ErrTimeout, err.Code)
}

func TestTraceLevelConstants(t *testing.T) {
	assert.Equal(t, TraceLevel("minimal"), TraceLevelMinimal)
	assert.Equal(t, TraceLevel("standard"), TraceLevelStandard)
	assert.Equal(t, TraceLevel("full"), TraceLevelFull)
	assert.Equal(t, TraceLevel("debug"), TraceLevelDebug)
}

func TestDecisionTypeConstants(t *testing.T) {
	assert.Equal(t, DecisionType("resolve"), DecisionTypeResolve)
	assert.Equal(t, DecisionType("queue_review"), DecisionTypeQueueReview)
	assert.Equal(t, DecisionType("suggest_new_entity"), DecisionTypeSuggestNewEntity)
	assert.Equal(t, DecisionType("skip_verification"), DecisionTypeSkipVerification)
}

func TestStageNameConstants(t *testing.T) {
	assert.Equal(t, StageName("understanding"), StageNameUnderstanding)
	assert.Equal(t, StageName("cross_mention"), StageNameCrossMention)
	assert.Equal(t, StageName("matching"), StageNameMatching)
	assert.Equal(t, StageName("verification"), StageNameVerification)
}

// TestGenerateTraceID tests

func TestGenerateTraceID(t *testing.T) {
	id1 := generateTraceID()
	id2 := generateTraceID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2, "trace IDs should be unique")
	assert.Contains(t, id1, "trace_")
}

// TestNewResolver tests

func TestNewResolver_WithValidConfig(t *testing.T) {
	mockProvider := new(MockLLMProvider)
	mockLookup := new(MockEntityLookup)
	mockRepo := new(MockRepository)
	mockTracer := new(MockTracer)

	mockProvider.On("Name").Return("test-provider")

	resolver := NewResolver(
		DefaultConfig(),
		mockProvider,
		mockLookup,
		mockRepo,
		mockTracer,
	)

	assert.NotNil(t, resolver)
	assert.NotNil(t, resolver.stages)
	assert.NotNil(t, resolver.gatherer)
}

func TestNewResolver_WithInvalidConfig(t *testing.T) {
	mockProvider := new(MockLLMProvider)
	mockLookup := new(MockEntityLookup)
	mockRepo := new(MockRepository)

	mockProvider.On("Name").Return("test-provider")

	// Empty config should use defaults
	resolver := NewResolver(
		Config{},
		mockProvider,
		mockLookup,
		mockRepo,
		nil,
	)

	assert.NotNil(t, resolver)
	// Verify defaults were applied
	assert.Equal(t, float32(0.8), resolver.config.Thresholds.AutoResolve)
}

func TestNewResolver_NilTracer(t *testing.T) {
	mockProvider := new(MockLLMProvider)
	mockLookup := new(MockEntityLookup)
	mockRepo := new(MockRepository)

	mockProvider.On("Name").Return("test-provider")

	resolver := NewResolver(
		DefaultConfig(),
		mockProvider,
		mockLookup,
		mockRepo,
		nil, // nil tracer is allowed
	)

	assert.NotNil(t, resolver)
	assert.Nil(t, resolver.tracer)
}

// TestResolutionBatch tests

func TestResolutionBatch_Minimal(t *testing.T) {
	batch := ResolutionBatch{
		ContentID:   123,
		ContentType: "email",
		ContentText: "Hello John",
	}

	assert.Equal(t, int64(123), batch.ContentID)
	assert.Equal(t, "email", batch.ContentType)
	assert.Equal(t, "Hello John", batch.ContentText)
	assert.Nil(t, batch.Metadata)
}

func TestResolutionBatch_WithMetadata(t *testing.T) {
	batch := ResolutionBatch{
		ContentID:   123,
		ContentType: "email",
		ContentText: "Hello John",
		Metadata: &ContentMetadata{
			Subject:      "Meeting Notes",
			Participants: []string{"john@example.com", "jane@example.com"},
		},
	}

	assert.NotNil(t, batch.Metadata)
	assert.Equal(t, "Meeting Notes", batch.Metadata.Subject)
	assert.Len(t, batch.Metadata.Participants, 2)
}

// TestMentionInput tests

func TestMentionInput(t *testing.T) {
	input := MentionInput{
		Text:           "John",
		Position:       10,
		ContextSnippet: "Hello John, how are you?",
	}

	assert.Equal(t, "John", input.Text)
	assert.Equal(t, 10, input.Position)
	assert.Contains(t, input.ContextSnippet, "John")
}

// TestResolution tests

func TestResolution_Resolved(t *testing.T) {
	res := Resolution{
		MentionText:     "John",
		MentionPosition: 10,
		Decision:        DecisionTypeResolve,
		ResolvedTo: &ResolvedEntity{
			EntityType: mentions.EntityTypePerson,
			EntityID:   1,
			EntityName: "John Smith",
		},
		Confidence: 0.95,
		Reasoning:  "Exact match on first name",
	}

	assert.Equal(t, DecisionTypeResolve, res.Decision)
	assert.NotNil(t, res.ResolvedTo)
	assert.Equal(t, "John Smith", res.ResolvedTo.EntityName)
	assert.GreaterOrEqual(t, res.Confidence, float32(0.9))
}

func TestResolution_QueuedForReview(t *testing.T) {
	res := Resolution{
		MentionText: "JS",
		Decision:    DecisionTypeQueueReview,
		Confidence:  0.6,
		Reasoning:   "Ambiguous initials - multiple matches",
		Alternatives: []AlternativeEntity{
			{EntityID: 1, EntityName: "John Smith", Confidence: 0.6},
			{EntityID: 2, EntityName: "Jane Sanders", Confidence: 0.5},
		},
	}

	assert.Equal(t, DecisionTypeQueueReview, res.Decision)
	assert.Nil(t, res.ResolvedTo)
	assert.Len(t, res.Alternatives, 2)
}

// TestBatchResult tests

func TestBatchResult(t *testing.T) {
	result := BatchResult{
		ContentID:        123,
		TraceID:          "trace_abc123",
		AutoResolved:     5,
		QueuedForReview:  2,
		ProcessingTimeMs: 1500,
	}

	assert.Equal(t, int64(123), result.ContentID)
	assert.Contains(t, result.TraceID, "trace_")
	assert.Equal(t, 5, result.AutoResolved)
	assert.Equal(t, 2, result.QueuedForReview)
	assert.Greater(t, result.ProcessingTimeMs, 0)
}
