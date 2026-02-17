// Package client provides the gRPC client for connecting to the Penfold API Gateway.
// This file contains AuditService client methods.
package client

import (
	"context"
	"fmt"
	"time"

	auditv1 "github.com/otherjamesbrown/penf-cli/api/proto/audit/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// =============================================================================
// AuditService Client Methods
// =============================================================================

// AuditServiceClient returns an AuditService gRPC client.
// Returns an error if not connected.
func (c *GRPCClient) AuditServiceClient() (auditv1.AuditServiceClient, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return nil, fmt.Errorf("not connected to gateway")
	}

	return auditv1.NewAuditServiceClient(conn), nil
}

// =============================================================================
// Audit Client Wrapper (for use with raw gRPC connection)
// =============================================================================

// AuditClient wraps the AuditService gRPC client for convenient CLI usage.
type AuditClient struct {
	client auditv1.AuditServiceClient
}

// NewAuditClient creates a new AuditClient from a gRPC connection.
func NewAuditClient(conn *grpc.ClientConn) *AuditClient {
	return &AuditClient{
		client: auditv1.NewAuditServiceClient(conn),
	}
}

// TraceSummary represents a resolution trace summary for the CLI.
type TraceSummary struct {
	ID                    string
	ContentID             int64
	ContentType           string
	ContentSummary        string
	Status                string
	MentionsFound         int32
	AutoResolved          int32
	QueuedForReview       int32
	ModelUsed             string
	DurationMs            int32
	StartedAt             time.Time
	NewEntitiesSuggested  int32
}

// Stage represents a trace stage for the CLI.
type Stage struct {
	ID            int64
	TraceID       string
	StageNumber   int32
	StageName     string
	StartedAt     time.Time
	CompletedAt   time.Time
	DurationMs    int32
	InputSummary  string
	OutputSummary string
	Status        string
	Skipped       bool
	SkipReason    string
	ErrorMessage  string
}

// Decision represents a resolution decision for the CLI.
type Decision struct {
	ID              int64
	TraceID         string
	StageID         *int64
	DecisionType    string
	MentionID       *int64
	MentionedText   string
	ChosenOption    string
	Confidence      float32
	Reasoning       string
	WasCorrect      *bool
	CorrectionNotes string
	CreatedAt       time.Time
}

// LLMCall represents an LLM request/response for the CLI.
type LLMCall struct {
	ID             int64
	TraceID        string
	StageID        int64
	Model          string
	PromptTemplate string
	PromptText     string
	PromptTokens   int32
	ResponseText   string
	ResponseTokens int32
	LatencyMs      int32
	AttemptNumber  int32
	IsFallback     bool
	FallbackReason string
}

// TraceDetail contains full trace details for the CLI.
type TraceDetail struct {
	Trace                TraceSummary
	Stages               []Stage
	Decisions            []Decision
	LLMCalls             []LLMCall
	NewEntitiesSuggested int32
}

// ComparisonSummary represents a model comparison summary for the CLI.
type ComparisonSummary struct {
	ID                 string
	ContentID          int64
	ContentType        string
	ContentSummary     string
	Models             []string
	TotalDecisions     int32
	UnanimousDecisions int32
	DivergentDecisions int32
	StartedAt          time.Time
}

// ModelDecision represents a single model's decision for the CLI.
type ModelDecision struct {
	Model          string
	EntityID       *int64
	EntityName     string
	Confidence     float32
	Reasoning      string
	SuggestNew     bool
	NewEntityType  string
}

// ComparisonDecision represents a per-mention decision comparison for the CLI.
type ComparisonDecision struct {
	ID                   int64
	ComparisonID         string
	MentionedText        string
	MentionIndex         int32
	ModelDecisions       []ModelDecision
	IsUnanimous          bool
	DivergenceType       string
	ConfidenceSpread     float32
	GroundTruthEntityID  *int64
	ModelsCorrect        []string
}

// ComparisonDetail contains full comparison details for the CLI.
type ComparisonDetail struct {
	Comparison  ComparisonSummary
	Decisions   []ComparisonDecision
	Purpose     string
	InitiatedBy string
	TraceIDs    []string
}

// ModelStats represents model statistics for the CLI.
type ModelStats struct {
	Model              string
	TotalComparisons   int32
	TotalDecisions     int32
	CorrectDecisions   int32
	Accuracy           float32
	AverageConfidence  float32
	AverageLatencyMs   int32
	UnanimousAgreement int32
}

// ListTraces retrieves a list of resolution traces with optional filtering.
func (c *AuditClient) ListTraces(ctx context.Context, tenantID string, contentID int64, contentType string, since *time.Time, hadCorrections bool, limit, offset int32) ([]TraceSummary, int64, error) {
	req := &auditv1.ListTracesRequest{
		TenantId:       tenantID,
		Limit:          limit,
		Offset:         offset,
		HadCorrections: hadCorrections,
	}

	if contentID > 0 {
		req.ContentId = contentID
	}
	if contentType != "" {
		req.ContentType = contentType
	}
	if since != nil {
		req.Since = timestamppb.New(*since)
	}

	resp, err := c.client.ListTraces(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("ListTraces RPC failed: %w", err)
	}

	traces := make([]TraceSummary, len(resp.Traces))
	for i, t := range resp.Traces {
		traces[i] = convertTraceSummary(t)
	}

	return traces, resp.TotalCount, nil
}

// GetTrace retrieves detailed information about a specific trace.
func (c *AuditClient) GetTrace(ctx context.Context, traceID string, includeLLMCalls bool) (*TraceDetail, error) {
	req := &auditv1.GetTraceRequest{
		TraceId:         traceID,
		IncludeLlmCalls: includeLLMCalls,
	}

	resp, err := c.client.GetTrace(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetTrace RPC failed: %w", err)
	}

	if resp.Trace == nil {
		return nil, fmt.Errorf("trace not found")
	}

	return convertTraceDetail(resp.Trace), nil
}

// ListCorrections retrieves a list of decisions that were corrected by humans.
func (c *AuditClient) ListCorrections(ctx context.Context, tenantID string, since *time.Time, limit, offset int32) ([]Decision, int64, error) {
	req := &auditv1.ListCorrectionsRequest{
		TenantId: tenantID,
		Limit:    limit,
		Offset:   offset,
	}

	if since != nil {
		req.Since = timestamppb.New(*since)
	}

	resp, err := c.client.ListCorrections(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("ListCorrections RPC failed: %w", err)
	}

	corrections := make([]Decision, len(resp.Corrections))
	for i, d := range resp.Corrections {
		corrections[i] = convertDecision(d)
	}

	return corrections, resp.TotalCount, nil
}

// ListComparisons retrieves a list of model comparison runs.
func (c *AuditClient) ListComparisons(ctx context.Context, tenantID string, since *time.Time, limit, offset int32) ([]ComparisonSummary, int64, error) {
	req := &auditv1.ListComparisonsRequest{
		TenantId: tenantID,
		Limit:    limit,
		Offset:   offset,
	}

	if since != nil {
		req.Since = timestamppb.New(*since)
	}

	resp, err := c.client.ListComparisons(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("ListComparisons RPC failed: %w", err)
	}

	comparisons := make([]ComparisonSummary, len(resp.Comparisons))
	for i, c := range resp.Comparisons {
		comparisons[i] = convertComparisonSummary(c)
	}

	return comparisons, resp.TotalCount, nil
}

// GetComparison retrieves detailed information about a specific comparison.
func (c *AuditClient) GetComparison(ctx context.Context, comparisonID string) (*ComparisonDetail, error) {
	req := &auditv1.GetComparisonRequest{
		ComparisonId: comparisonID,
	}

	resp, err := c.client.GetComparison(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetComparison RPC failed: %w", err)
	}

	if resp.Comparison == nil {
		return nil, fmt.Errorf("comparison not found")
	}

	return convertComparisonDetail(resp.Comparison), nil
}

// GetModelStats retrieves aggregate statistics for each model.
func (c *AuditClient) GetModelStats(ctx context.Context, tenantID string, daysSince int32) ([]ModelStats, error) {
	req := &auditv1.GetModelStatsRequest{
		TenantId:  tenantID,
		DaysSince: daysSince,
	}

	resp, err := c.client.GetModelStats(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetModelStats RPC failed: %w", err)
	}

	stats := make([]ModelStats, len(resp.Stats))
	for i, s := range resp.Stats {
		stats[i] = convertModelStats(s)
	}

	return stats, nil
}

// =============================================================================
// Conversion Functions (proto -> CLI types)
// =============================================================================

func convertTraceSummary(t *auditv1.TraceSummary) TraceSummary {
	summary := TraceSummary{
		ID:              t.Id,
		ContentID:       t.ContentId,
		ContentType:     t.ContentType,
		ContentSummary:  t.ContentSummary,
		Status:          t.Status,
		MentionsFound:   t.MentionsFound,
		AutoResolved:    t.AutoResolved,
		QueuedForReview: t.QueuedForReview,
		ModelUsed:       t.ModelUsed,
		DurationMs:      t.DurationMs,
	}
	if t.StartedAt != nil {
		summary.StartedAt = t.StartedAt.AsTime()
	}
	return summary
}

func convertStage(s *auditv1.Stage) Stage {
	stage := Stage{
		ID:            s.Id,
		TraceID:       s.TraceId,
		StageNumber:   s.StageNumber,
		StageName:     s.StageName,
		DurationMs:    s.DurationMs,
		InputSummary:  s.InputSummary,
		OutputSummary: s.OutputSummary,
		Status:        s.Status,
		Skipped:       s.Skipped,
		SkipReason:    s.SkipReason,
		ErrorMessage:  s.ErrorMessage,
	}
	if s.StartedAt != nil {
		stage.StartedAt = s.StartedAt.AsTime()
	}
	if s.CompletedAt != nil {
		stage.CompletedAt = s.CompletedAt.AsTime()
	}
	return stage
}

func convertDecision(d *auditv1.Decision) Decision {
	decision := Decision{
		ID:              d.Id,
		TraceID:         d.TraceId,
		DecisionType:    d.DecisionType,
		MentionedText:   d.MentionedText,
		ChosenOption:    d.ChosenOption,
		Confidence:      d.Confidence,
		Reasoning:       d.Reasoning,
		CorrectionNotes: d.CorrectionNotes,
	}
	if d.StageId > 0 {
		decision.StageID = &d.StageId
	}
	if d.MentionId > 0 {
		decision.MentionID = &d.MentionId
	}
	decision.WasCorrect = &d.WasCorrect
	if d.CreatedAt != nil {
		decision.CreatedAt = d.CreatedAt.AsTime()
	}
	return decision
}

func convertLLMCall(l *auditv1.LLMCall) LLMCall {
	return LLMCall{
		ID:             l.Id,
		TraceID:        l.TraceId,
		StageID:        l.StageId,
		Model:          l.Model,
		PromptTemplate: l.PromptTemplate,
		PromptText:     l.PromptText,
		PromptTokens:   l.PromptTokens,
		ResponseText:   l.ResponseText,
		ResponseTokens: l.ResponseTokens,
		LatencyMs:      l.LatencyMs,
		AttemptNumber:  l.AttemptNumber,
		IsFallback:     l.IsFallback,
		FallbackReason: l.FallbackReason,
	}
}

func convertTraceDetail(td *auditv1.TraceDetail) *TraceDetail {
	detail := &TraceDetail{
		Trace:                convertTraceSummary(td.Trace),
		NewEntitiesSuggested: td.NewEntitiesSuggested,
	}

	detail.Stages = make([]Stage, len(td.Stages))
	for i, s := range td.Stages {
		detail.Stages[i] = convertStage(s)
	}

	detail.Decisions = make([]Decision, len(td.Decisions))
	for i, d := range td.Decisions {
		detail.Decisions[i] = convertDecision(d)
	}

	detail.LLMCalls = make([]LLMCall, len(td.LlmCalls))
	for i, l := range td.LlmCalls {
		detail.LLMCalls[i] = convertLLMCall(l)
	}

	return detail
}

func convertComparisonSummary(c *auditv1.ComparisonSummary) ComparisonSummary {
	summary := ComparisonSummary{
		ID:                 c.Id,
		ContentID:          c.ContentId,
		ContentType:        c.ContentType,
		ContentSummary:     c.ContentSummary,
		Models:             c.Models,
		TotalDecisions:     c.TotalDecisions,
		UnanimousDecisions: c.UnanimousDecisions,
		DivergentDecisions: c.DivergentDecisions,
	}
	if c.StartedAt != nil {
		summary.StartedAt = c.StartedAt.AsTime()
	}
	return summary
}

func convertModelDecision(md *auditv1.ModelDecision) ModelDecision {
	decision := ModelDecision{
		Model:         md.Model,
		EntityName:    md.EntityName,
		Confidence:    md.Confidence,
		Reasoning:     md.Reasoning,
		SuggestNew:    md.SuggestNew,
		NewEntityType: md.NewEntityType,
	}
	if md.EntityId > 0 {
		decision.EntityID = &md.EntityId
	}
	return decision
}

func convertComparisonDecision(cd *auditv1.ComparisonDecision) ComparisonDecision {
	decision := ComparisonDecision{
		ID:               cd.Id,
		ComparisonID:     cd.ComparisonId,
		MentionedText:    cd.MentionedText,
		MentionIndex:     cd.MentionIndex,
		IsUnanimous:      cd.IsUnanimous,
		DivergenceType:   cd.DivergenceType,
		ConfidenceSpread: cd.ConfidenceSpread,
		ModelsCorrect:    cd.ModelsCorrect,
	}

	decision.ModelDecisions = make([]ModelDecision, len(cd.ModelDecisions))
	for i, md := range cd.ModelDecisions {
		decision.ModelDecisions[i] = convertModelDecision(md)
	}

	if cd.GroundTruthEntityId > 0 {
		decision.GroundTruthEntityID = &cd.GroundTruthEntityId
	}

	return decision
}

func convertComparisonDetail(cd *auditv1.ComparisonDetail) *ComparisonDetail {
	detail := &ComparisonDetail{
		Comparison:  convertComparisonSummary(cd.Comparison),
		Purpose:     cd.Purpose,
		InitiatedBy: cd.InitiatedBy,
		TraceIDs:    cd.TraceIds,
	}

	detail.Decisions = make([]ComparisonDecision, len(cd.Decisions))
	for i, d := range cd.Decisions {
		detail.Decisions[i] = convertComparisonDecision(d)
	}

	return detail
}

func convertModelStats(ms *auditv1.ModelStats) ModelStats {
	return ModelStats{
		Model:              ms.Model,
		TotalComparisons:   ms.TotalComparisons,
		TotalDecisions:     ms.TotalDecisions,
		CorrectDecisions:   ms.CorrectDecisions,
		Accuracy:           ms.Accuracy,
		AverageConfidence:  ms.AverageConfidence,
		AverageLatencyMs:   ms.AverageLatencyMs,
		UnanimousAgreement: ms.UnanimousAgreement,
	}
}
