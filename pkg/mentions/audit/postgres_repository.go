package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otherjamesbrown/penf-cli/pkg/mentions/resolver"
)

// PostgresRepository implements Repository using PostgreSQL.
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL-backed repository.
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// CreateTrace creates a new trace.
func (r *PostgresRepository) CreateTrace(ctx context.Context, trace *Trace) error {
	configJSON, _ := json.Marshal(trace.ConfigSnapshot)

	query := `
		INSERT INTO resolution_traces (
			id, tenant_id, content_id, content_type, content_summary,
			started_at, status, model_used, trace_level, config_snapshot, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err := r.db.Exec(ctx, query,
		trace.ID,
		trace.TenantID,
		trace.ContentID,
		nullString(trace.ContentType),
		nullString(trace.ContentSummary),
		trace.StartedAt,
		string(trace.Status),
		nullString(trace.ModelUsed),
		string(trace.TraceLevel),
		configJSON,
		trace.CreatedAt,
	)
	return err
}

// GetTrace retrieves a trace by ID.
func (r *PostgresRepository) GetTrace(ctx context.Context, id string) (*Trace, error) {
	query := `
		SELECT id, tenant_id, content_id, content_type, content_summary,
			started_at, completed_at, duration_ms, mentions_found, auto_resolved,
			queued_for_review, new_entities_suggested, status, error_message,
			model_used, trace_level, config_snapshot, created_at
		FROM resolution_traces WHERE id = $1`

	var trace Trace
	var contentType, contentSummary, errorMessage, modelUsed sql.NullString
	var completedAt sql.NullTime
	var durationMs, mentionsFound, autoResolved, queuedForReview, newEntities sql.NullInt32
	var configJSON []byte
	var status, traceLevel string

	err := r.db.QueryRow(ctx, query, id).Scan(
		&trace.ID, &trace.TenantID, &trace.ContentID, &contentType, &contentSummary,
		&trace.StartedAt, &completedAt, &durationMs, &mentionsFound, &autoResolved,
		&queuedForReview, &newEntities, &status, &errorMessage,
		&modelUsed, &traceLevel, &configJSON, &trace.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	trace.ContentType = contentType.String
	trace.ContentSummary = contentSummary.String
	if completedAt.Valid {
		trace.CompletedAt = &completedAt.Time
	}
	trace.DurationMs = int(durationMs.Int32)
	trace.MentionsFound = int(mentionsFound.Int32)
	trace.AutoResolved = int(autoResolved.Int32)
	trace.QueuedForReview = int(queuedForReview.Int32)
	trace.NewEntitiesSuggested = int(newEntities.Int32)
	trace.Status = resolver.TraceStatus(status)
	trace.ErrorMessage = errorMessage.String
	trace.ModelUsed = modelUsed.String
	trace.TraceLevel = resolver.TraceLevel(traceLevel)
	if configJSON != nil {
		json.Unmarshal(configJSON, &trace.ConfigSnapshot)
	}

	return &trace, nil
}

// UpdateTrace updates an existing trace.
func (r *PostgresRepository) UpdateTrace(ctx context.Context, trace *Trace) error {
	query := `
		UPDATE resolution_traces SET
			completed_at = $2, duration_ms = $3, mentions_found = $4, auto_resolved = $5,
			queued_for_review = $6, new_entities_suggested = $7, status = $8, error_message = $9
		WHERE id = $1`

	_, err := r.db.Exec(ctx, query,
		trace.ID,
		trace.CompletedAt,
		trace.DurationMs,
		trace.MentionsFound,
		trace.AutoResolved,
		trace.QueuedForReview,
		trace.NewEntitiesSuggested,
		string(trace.Status),
		nullString(trace.ErrorMessage),
	)
	return err
}

// ListTraces lists traces matching the filter.
func (r *PostgresRepository) ListTraces(ctx context.Context, filter TraceFilter) ([]TraceSummary, error) {
	query := `
		SELECT id, content_id, content_type, content_summary, status,
			mentions_found, auto_resolved, queued_for_review, model_used,
			duration_ms, started_at
		FROM resolution_traces
		WHERE tenant_id = $1`
	args := []interface{}{filter.TenantID}
	argNum := 2

	if filter.ContentID != nil {
		query += fmt.Sprintf(" AND content_id = $%d", argNum)
		args = append(args, *filter.ContentID)
		argNum++
	}
	if filter.ContentType != "" {
		query += fmt.Sprintf(" AND content_type = $%d", argNum)
		args = append(args, filter.ContentType)
		argNum++
	}
	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, string(*filter.Status))
		argNum++
	}
	if filter.ModelUsed != "" {
		query += fmt.Sprintf(" AND model_used = $%d", argNum)
		args = append(args, filter.ModelUsed)
		argNum++
	}
	if filter.Since != nil {
		query += fmt.Sprintf(" AND started_at >= $%d", argNum)
		args = append(args, *filter.Since)
		argNum++
	}
	if filter.Until != nil {
		query += fmt.Sprintf(" AND started_at <= $%d", argNum)
		args = append(args, *filter.Until)
		argNum++
	}
	if filter.HadCorrections {
		query += ` AND id IN (SELECT DISTINCT trace_id FROM resolution_decisions WHERE was_correct = false)`
	}

	query += " ORDER BY started_at DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traces []TraceSummary
	for rows.Next() {
		var t TraceSummary
		var contentType, contentSummary, modelUsed sql.NullString
		var durationMs, mentionsFound, autoResolved, queuedForReview sql.NullInt32

		err := rows.Scan(
			&t.ID, &t.ContentID, &contentType, &contentSummary, &t.Status,
			&mentionsFound, &autoResolved, &queuedForReview, &modelUsed,
			&durationMs, &t.StartedAt,
		)
		if err != nil {
			return nil, err
		}

		t.ContentType = contentType.String
		t.ContentSummary = contentSummary.String
		t.ModelUsed = modelUsed.String
		t.DurationMs = int(durationMs.Int32)
		t.MentionsFound = int(mentionsFound.Int32)
		t.AutoResolved = int(autoResolved.Int32)
		t.QueuedForReview = int(queuedForReview.Int32)

		traces = append(traces, t)
	}

	return traces, nil
}

// DeleteTrace deletes a trace and all related records.
func (r *PostgresRepository) DeleteTrace(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM resolution_traces WHERE id = $1", id)
	return err
}

// CreateStage creates a new stage.
func (r *PostgresRepository) CreateStage(ctx context.Context, stage *Stage) error {
	inputJSON, _ := json.Marshal(stage.InputData)

	query := `
		INSERT INTO resolution_trace_stages (
			trace_id, stage_number, stage_name, started_at, input_summary,
			input_data, status, skipped, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	return r.db.QueryRow(ctx, query,
		stage.TraceID,
		stage.StageNumber,
		string(stage.StageName),
		stage.StartedAt,
		nullString(stage.InputSummary),
		inputJSON,
		string(stage.Status),
		stage.Skipped,
		stage.CreatedAt,
	).Scan(&stage.ID)
}

// GetStage retrieves a stage by ID.
func (r *PostgresRepository) GetStage(ctx context.Context, id int64) (*Stage, error) {
	query := `
		SELECT id, trace_id, stage_number, stage_name, started_at, completed_at,
			duration_ms, input_summary, input_data, output_summary, output_data,
			status, skipped, skip_reason, error_message, created_at
		FROM resolution_trace_stages WHERE id = $1`

	var stage Stage
	var completedAt sql.NullTime
	var durationMs sql.NullInt32
	var inputSummary, outputSummary, skipReason, errorMessage sql.NullString
	var inputJSON, outputJSON []byte
	var stageName, status string

	err := r.db.QueryRow(ctx, query, id).Scan(
		&stage.ID, &stage.TraceID, &stage.StageNumber, &stageName, &stage.StartedAt,
		&completedAt, &durationMs, &inputSummary, &inputJSON, &outputSummary, &outputJSON,
		&status, &stage.Skipped, &skipReason, &errorMessage, &stage.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	stage.StageName = resolver.StageName(stageName)
	if completedAt.Valid {
		stage.CompletedAt = &completedAt.Time
	}
	stage.DurationMs = int(durationMs.Int32)
	stage.InputSummary = inputSummary.String
	stage.OutputSummary = outputSummary.String
	stage.SkipReason = skipReason.String
	stage.ErrorMessage = errorMessage.String
	stage.Status = resolver.StageStatus(status)
	if inputJSON != nil {
		json.Unmarshal(inputJSON, &stage.InputData)
	}
	if outputJSON != nil {
		json.Unmarshal(outputJSON, &stage.OutputData)
	}

	return &stage, nil
}

// UpdateStage updates an existing stage.
func (r *PostgresRepository) UpdateStage(ctx context.Context, stage *Stage) error {
	outputJSON, _ := json.Marshal(stage.OutputData)

	query := `
		UPDATE resolution_trace_stages SET
			completed_at = $2, duration_ms = $3, output_summary = $4, output_data = $5,
			status = $6, skipped = $7, skip_reason = $8, error_message = $9
		WHERE id = $1`

	_, err := r.db.Exec(ctx, query,
		stage.ID,
		stage.CompletedAt,
		stage.DurationMs,
		nullString(stage.OutputSummary),
		outputJSON,
		string(stage.Status),
		stage.Skipped,
		nullString(stage.SkipReason),
		nullString(stage.ErrorMessage),
	)
	return err
}

// GetStagesForTrace retrieves all stages for a trace.
func (r *PostgresRepository) GetStagesForTrace(ctx context.Context, traceID string) ([]Stage, error) {
	query := `
		SELECT id, trace_id, stage_number, stage_name, started_at, completed_at,
			duration_ms, input_summary, input_data, output_summary, output_data,
			status, skipped, skip_reason, error_message, created_at
		FROM resolution_trace_stages WHERE trace_id = $1
		ORDER BY stage_number`

	rows, err := r.db.Query(ctx, query, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stages []Stage
	for rows.Next() {
		var stage Stage
		var completedAt sql.NullTime
		var durationMs sql.NullInt32
		var inputSummary, outputSummary, skipReason, errorMessage sql.NullString
		var inputJSON, outputJSON []byte
		var stageName, status string

		err := rows.Scan(
			&stage.ID, &stage.TraceID, &stage.StageNumber, &stageName, &stage.StartedAt,
			&completedAt, &durationMs, &inputSummary, &inputJSON, &outputSummary, &outputJSON,
			&status, &stage.Skipped, &skipReason, &errorMessage, &stage.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		stage.StageName = resolver.StageName(stageName)
		if completedAt.Valid {
			stage.CompletedAt = &completedAt.Time
		}
		stage.DurationMs = int(durationMs.Int32)
		stage.InputSummary = inputSummary.String
		stage.OutputSummary = outputSummary.String
		stage.SkipReason = skipReason.String
		stage.ErrorMessage = errorMessage.String
		stage.Status = resolver.StageStatus(status)
		if inputJSON != nil {
			json.Unmarshal(inputJSON, &stage.InputData)
		}
		if outputJSON != nil {
			json.Unmarshal(outputJSON, &stage.OutputData)
		}

		stages = append(stages, stage)
	}

	return stages, nil
}

// CreateLLMCall creates a new LLM call record.
func (r *PostgresRepository) CreateLLMCall(ctx context.Context, call *LLMCall) error {
	parsedJSON, _ := json.Marshal(call.ParsedOutput)

	query := `
		INSERT INTO resolution_llm_calls (
			trace_id, stage_id, model, prompt_template, prompt_text, prompt_tokens,
			response_text, response_tokens, parsed_output, parse_errors, latency_ms,
			attempt_number, is_fallback, fallback_reason, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id`

	return r.db.QueryRow(ctx, query,
		call.TraceID,
		call.StageID,
		call.Model,
		nullString(call.PromptTemplate),
		nullString(call.PromptText),
		call.PromptTokens,
		nullString(call.ResponseText),
		call.ResponseTokens,
		parsedJSON,
		call.ParseErrors,
		call.LatencyMs,
		call.AttemptNumber,
		call.IsFallback,
		nullString(call.FallbackReason),
		call.CreatedAt,
	).Scan(&call.ID)
}

// GetLLMCallsForTrace retrieves all LLM calls for a trace.
func (r *PostgresRepository) GetLLMCallsForTrace(ctx context.Context, traceID string) ([]LLMCall, error) {
	query := `
		SELECT id, trace_id, stage_id, model, prompt_template, prompt_text, prompt_tokens,
			response_text, response_tokens, parsed_output, parse_errors, latency_ms,
			attempt_number, is_fallback, fallback_reason, created_at
		FROM resolution_llm_calls WHERE trace_id = $1
		ORDER BY created_at`

	rows, err := r.db.Query(ctx, query, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanLLMCalls(rows)
}

// GetLLMCallsForStage retrieves all LLM calls for a stage.
func (r *PostgresRepository) GetLLMCallsForStage(ctx context.Context, stageID int64) ([]LLMCall, error) {
	query := `
		SELECT id, trace_id, stage_id, model, prompt_template, prompt_text, prompt_tokens,
			response_text, response_tokens, parsed_output, parse_errors, latency_ms,
			attempt_number, is_fallback, fallback_reason, created_at
		FROM resolution_llm_calls WHERE stage_id = $1
		ORDER BY created_at`

	rows, err := r.db.Query(ctx, query, stageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanLLMCalls(rows)
}

func (r *PostgresRepository) scanLLMCalls(rows interface{ Next() bool; Scan(dest ...interface{}) error }) ([]LLMCall, error) {
	var calls []LLMCall
	for rows.Next() {
		var call LLMCall
		var stageID sql.NullInt64
		var promptTemplate, promptText, responseText, fallbackReason sql.NullString
		var parsedJSON []byte

		err := rows.Scan(
			&call.ID, &call.TraceID, &stageID, &call.Model, &promptTemplate,
			&promptText, &call.PromptTokens, &responseText, &call.ResponseTokens,
			&parsedJSON, &call.ParseErrors, &call.LatencyMs, &call.AttemptNumber,
			&call.IsFallback, &fallbackReason, &call.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if stageID.Valid {
			call.StageID = &stageID.Int64
		}
		call.PromptTemplate = promptTemplate.String
		call.PromptText = promptText.String
		call.ResponseText = responseText.String
		call.FallbackReason = fallbackReason.String
		if parsedJSON != nil {
			json.Unmarshal(parsedJSON, &call.ParsedOutput)
		}

		calls = append(calls, call)
	}
	return calls, nil
}

// CreateDecision creates a new decision record.
func (r *PostgresRepository) CreateDecision(ctx context.Context, decision *Decision) error {
	alternativesJSON, _ := json.Marshal(decision.Alternatives)
	factorsJSON, _ := json.Marshal(decision.Factors)

	query := `
		INSERT INTO resolution_decisions (
			trace_id, stage_id, decision_type, mention_id, mentioned_text,
			chosen_option, alternatives, confidence, reasoning, factors, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`

	return r.db.QueryRow(ctx, query,
		decision.TraceID,
		decision.StageID,
		string(decision.DecisionType),
		decision.MentionID,
		nullString(decision.MentionedText),
		nullString(decision.ChosenOption),
		alternativesJSON,
		decision.Confidence,
		nullString(decision.Reasoning),
		factorsJSON,
		decision.CreatedAt,
	).Scan(&decision.ID)
}

// GetDecision retrieves a decision by ID.
func (r *PostgresRepository) GetDecision(ctx context.Context, id int64) (*Decision, error) {
	query := `
		SELECT id, trace_id, stage_id, decision_type, mention_id, mentioned_text,
			chosen_option, alternatives, confidence, reasoning, factors,
			was_correct, correction_notes, created_at
		FROM resolution_decisions WHERE id = $1`

	var decision Decision
	var stageID, mentionID sql.NullInt64
	var mentionedText, chosenOption, reasoning, correctionNotes sql.NullString
	var wasCorrect sql.NullBool
	var alternativesJSON, factorsJSON []byte
	var decisionType string

	err := r.db.QueryRow(ctx, query, id).Scan(
		&decision.ID, &decision.TraceID, &stageID, &decisionType, &mentionID,
		&mentionedText, &chosenOption, &alternativesJSON, &decision.Confidence,
		&reasoning, &factorsJSON, &wasCorrect, &correctionNotes, &decision.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if stageID.Valid {
		decision.StageID = &stageID.Int64
	}
	if mentionID.Valid {
		decision.MentionID = &mentionID.Int64
	}
	decision.DecisionType = resolver.DecisionType(decisionType)
	decision.MentionedText = mentionedText.String
	decision.ChosenOption = chosenOption.String
	decision.Reasoning = reasoning.String
	decision.CorrectionNotes = correctionNotes.String
	if wasCorrect.Valid {
		decision.WasCorrect = &wasCorrect.Bool
	}
	if alternativesJSON != nil {
		json.Unmarshal(alternativesJSON, &decision.Alternatives)
	}
	if factorsJSON != nil {
		json.Unmarshal(factorsJSON, &decision.Factors)
	}

	return &decision, nil
}

// UpdateDecision updates an existing decision.
func (r *PostgresRepository) UpdateDecision(ctx context.Context, decision *Decision) error {
	query := `
		UPDATE resolution_decisions SET
			was_correct = $2, correction_notes = $3
		WHERE id = $1`

	_, err := r.db.Exec(ctx, query,
		decision.ID,
		decision.WasCorrect,
		nullString(decision.CorrectionNotes),
	)
	return err
}

// GetDecisionsForTrace retrieves all decisions for a trace.
func (r *PostgresRepository) GetDecisionsForTrace(ctx context.Context, traceID string) ([]Decision, error) {
	query := `
		SELECT id, trace_id, stage_id, decision_type, mention_id, mentioned_text,
			chosen_option, alternatives, confidence, reasoning, factors,
			was_correct, correction_notes, created_at
		FROM resolution_decisions WHERE trace_id = $1
		ORDER BY created_at`

	rows, err := r.db.Query(ctx, query, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanDecisions(rows)
}

// GetDecisionsForMention retrieves all decisions for a mention.
func (r *PostgresRepository) GetDecisionsForMention(ctx context.Context, mentionID int64) ([]Decision, error) {
	query := `
		SELECT id, trace_id, stage_id, decision_type, mention_id, mentioned_text,
			chosen_option, alternatives, confidence, reasoning, factors,
			was_correct, correction_notes, created_at
		FROM resolution_decisions WHERE mention_id = $1
		ORDER BY created_at`

	rows, err := r.db.Query(ctx, query, mentionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanDecisions(rows)
}

// GetCorrections retrieves decisions that were corrected.
func (r *PostgresRepository) GetCorrections(ctx context.Context, filter TraceFilter) ([]Decision, error) {
	query := `
		SELECT d.id, d.trace_id, d.stage_id, d.decision_type, d.mention_id, d.mentioned_text,
			d.chosen_option, d.alternatives, d.confidence, d.reasoning, d.factors,
			d.was_correct, d.correction_notes, d.created_at
		FROM resolution_decisions d
		JOIN resolution_traces t ON d.trace_id = t.id
		WHERE t.tenant_id = $1 AND d.was_correct = false`
	args := []interface{}{filter.TenantID}
	argNum := 2

	if filter.Since != nil {
		query += fmt.Sprintf(" AND d.created_at >= $%d", argNum)
		args = append(args, *filter.Since)
		argNum++
	}

	query += " ORDER BY d.created_at DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanDecisions(rows)
}

func (r *PostgresRepository) scanDecisions(rows interface{ Next() bool; Scan(dest ...interface{}) error }) ([]Decision, error) {
	var decisions []Decision
	for rows.Next() {
		var decision Decision
		var stageID, mentionID sql.NullInt64
		var mentionedText, chosenOption, reasoning, correctionNotes sql.NullString
		var wasCorrect sql.NullBool
		var alternativesJSON, factorsJSON []byte
		var decisionType string

		err := rows.Scan(
			&decision.ID, &decision.TraceID, &stageID, &decisionType, &mentionID,
			&mentionedText, &chosenOption, &alternativesJSON, &decision.Confidence,
			&reasoning, &factorsJSON, &wasCorrect, &correctionNotes, &decision.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if stageID.Valid {
			decision.StageID = &stageID.Int64
		}
		if mentionID.Valid {
			decision.MentionID = &mentionID.Int64
		}
		decision.DecisionType = resolver.DecisionType(decisionType)
		decision.MentionedText = mentionedText.String
		decision.ChosenOption = chosenOption.String
		decision.Reasoning = reasoning.String
		decision.CorrectionNotes = correctionNotes.String
		if wasCorrect.Valid {
			decision.WasCorrect = &wasCorrect.Bool
		}
		if alternativesJSON != nil {
			json.Unmarshal(alternativesJSON, &decision.Alternatives)
		}
		if factorsJSON != nil {
			json.Unmarshal(factorsJSON, &decision.Factors)
		}

		decisions = append(decisions, decision)
	}
	return decisions, nil
}

// GetTraceDetail retrieves full trace details.
func (r *PostgresRepository) GetTraceDetail(ctx context.Context, traceID string, includeFullLLMCalls bool) (*TraceDetail, error) {
	trace, err := r.GetTrace(ctx, traceID)
	if err != nil {
		return nil, err
	}

	stages, err := r.GetStagesForTrace(ctx, traceID)
	if err != nil {
		return nil, err
	}

	decisions, err := r.GetDecisionsForTrace(ctx, traceID)
	if err != nil {
		return nil, err
	}

	detail := &TraceDetail{
		Trace:     *trace,
		Stages:    stages,
		Decisions: decisions,
	}

	if includeFullLLMCalls {
		llmCalls, err := r.GetLLMCallsForTrace(ctx, traceID)
		if err != nil {
			return nil, err
		}
		detail.LLMCalls = llmCalls
	}

	return detail, nil
}

// GetCorrectionStats retrieves correction statistics.
func (r *PostgresRepository) GetCorrectionStats(ctx context.Context, tenantID string, daysSince int) (*CorrectionStats, error) {
	since := time.Now().AddDate(0, 0, -daysSince)

	query := `
		SELECT COUNT(*), mentioned_text
		FROM resolution_decisions d
		JOIN resolution_traces t ON d.trace_id = t.id
		WHERE t.tenant_id = $1 AND d.was_correct = false AND d.created_at >= $2
		GROUP BY mentioned_text
		ORDER BY COUNT(*) DESC`

	rows, err := r.db.Query(ctx, query, tenantID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := &CorrectionStats{
		ByPattern: make(map[string]int),
	}

	first := true
	for rows.Next() {
		var count int
		var text string
		if err := rows.Scan(&count, &text); err != nil {
			return nil, err
		}

		stats.TotalCorrections += count
		stats.ByPattern[text] = count

		if first {
			stats.MostCorrectedType = text
			stats.MostCorrectedTypeCount = count
			first = false
		}
	}

	return stats, nil
}

// DeleteOldTraces deletes traces older than specified days.
func (r *PostgresRepository) DeleteOldTraces(ctx context.Context, olderThanDays int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays)

	result, err := r.db.Exec(ctx,
		"DELETE FROM resolution_traces WHERE created_at < $1",
		cutoff)
	if err != nil {
		return 0, err
	}

	return int(result.RowsAffected()), nil
}

// DeleteOldDecisions deletes decisions older than specified days.
func (r *PostgresRepository) DeleteOldDecisions(ctx context.Context, olderThanDays int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays)

	result, err := r.db.Exec(ctx,
		"DELETE FROM resolution_decisions WHERE created_at < $1",
		cutoff)
	if err != nil {
		return 0, err
	}

	return int(result.RowsAffected()), nil
}

// =====================================================
// Comparison Repository Methods
// =====================================================

// CreateComparison creates a new comparison.
func (r *PostgresRepository) CreateComparison(ctx context.Context, comp *Comparison) error {
	divergenceJSON, _ := json.Marshal(comp.DivergenceSummary)

	query := `
		INSERT INTO resolution_comparisons (
			id, tenant_id, content_id, content_type, content_summary,
			models, trace_ids, initiated_by, purpose, started_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err := r.db.Exec(ctx, query,
		comp.ID,
		comp.TenantID,
		comp.ContentID,
		nullString(comp.ContentType),
		nullString(comp.ContentSummary),
		comp.Models,
		comp.TraceIDs,
		nullString(comp.InitiatedBy),
		nullString(comp.Purpose),
		comp.StartedAt,
		comp.CreatedAt,
	)
	_ = divergenceJSON // Used in update
	return err
}

// GetComparison retrieves a comparison by ID.
func (r *PostgresRepository) GetComparison(ctx context.Context, id string) (*Comparison, error) {
	query := `
		SELECT id, tenant_id, content_id, content_type, content_summary,
			models, trace_ids, initiated_by, purpose, started_at, completed_at,
			total_decisions, unanimous_decisions, divergent_decisions,
			divergence_summary, created_at
		FROM resolution_comparisons WHERE id = $1`

	var comp Comparison
	var contentType, contentSummary, initiatedBy, purpose sql.NullString
	var completedAt sql.NullTime
	var totalDecisions, unanimousDecisions, divergentDecisions sql.NullInt32
	var divergenceJSON []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&comp.ID, &comp.TenantID, &comp.ContentID, &contentType, &contentSummary,
		&comp.Models, &comp.TraceIDs, &initiatedBy, &purpose, &comp.StartedAt, &completedAt,
		&totalDecisions, &unanimousDecisions, &divergentDecisions,
		&divergenceJSON, &comp.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	comp.ContentType = contentType.String
	comp.ContentSummary = contentSummary.String
	comp.InitiatedBy = initiatedBy.String
	comp.Purpose = purpose.String
	if completedAt.Valid {
		comp.CompletedAt = &completedAt.Time
	}
	comp.TotalDecisions = int(totalDecisions.Int32)
	comp.UnanimousDecisions = int(unanimousDecisions.Int32)
	comp.DivergentDecisions = int(divergentDecisions.Int32)
	if divergenceJSON != nil {
		json.Unmarshal(divergenceJSON, &comp.DivergenceSummary)
	}

	return &comp, nil
}

// UpdateComparison updates an existing comparison.
func (r *PostgresRepository) UpdateComparison(ctx context.Context, comp *Comparison) error {
	divergenceJSON, _ := json.Marshal(comp.DivergenceSummary)

	query := `
		UPDATE resolution_comparisons SET
			trace_ids = $2, completed_at = $3,
			total_decisions = $4, unanimous_decisions = $5, divergent_decisions = $6,
			divergence_summary = $7
		WHERE id = $1`

	_, err := r.db.Exec(ctx, query,
		comp.ID,
		comp.TraceIDs,
		comp.CompletedAt,
		comp.TotalDecisions,
		comp.UnanimousDecisions,
		comp.DivergentDecisions,
		divergenceJSON,
	)
	return err
}

// ListComparisons lists comparisons matching the filter.
func (r *PostgresRepository) ListComparisons(ctx context.Context, filter ComparisonFilter) ([]ComparisonSummary, error) {
	query := `
		SELECT id, content_id, content_type, content_summary, models,
			total_decisions, unanimous_decisions, divergent_decisions, started_at
		FROM resolution_comparisons
		WHERE tenant_id = $1`
	args := []interface{}{filter.TenantID}
	argNum := 2

	if filter.ContentID != nil {
		query += fmt.Sprintf(" AND content_id = $%d", argNum)
		args = append(args, *filter.ContentID)
		argNum++
	}
	if filter.ContentType != "" {
		query += fmt.Sprintf(" AND content_type = $%d", argNum)
		args = append(args, filter.ContentType)
		argNum++
	}
	if filter.Purpose != "" {
		query += fmt.Sprintf(" AND purpose = $%d", argNum)
		args = append(args, filter.Purpose)
		argNum++
	}
	if filter.Since != nil {
		query += fmt.Sprintf(" AND started_at >= $%d", argNum)
		args = append(args, *filter.Since)
		argNum++
	}

	query += " ORDER BY started_at DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comparisons []ComparisonSummary
	for rows.Next() {
		var c ComparisonSummary
		var contentType, contentSummary sql.NullString
		var totalDecisions, unanimousDecisions, divergentDecisions sql.NullInt32

		err := rows.Scan(
			&c.ID, &c.ContentID, &contentType, &contentSummary, &c.Models,
			&totalDecisions, &unanimousDecisions, &divergentDecisions, &c.StartedAt,
		)
		if err != nil {
			return nil, err
		}

		c.ContentType = contentType.String
		c.ContentSummary = contentSummary.String
		c.TotalDecisions = int(totalDecisions.Int32)
		c.UnanimousDecisions = int(unanimousDecisions.Int32)
		c.DivergentDecisions = int(divergentDecisions.Int32)

		comparisons = append(comparisons, c)
	}

	return comparisons, nil
}

// DeleteComparison deletes a comparison and all related decisions.
func (r *PostgresRepository) DeleteComparison(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM resolution_comparisons WHERE id = $1", id)
	return err
}

// CreateComparisonDecision creates a new comparison decision.
func (r *PostgresRepository) CreateComparisonDecision(ctx context.Context, decision *ComparisonDecision) error {
	modelDecisionsJSON, _ := json.Marshal(decision.ModelDecisions)

	query := `
		INSERT INTO resolution_comparison_decisions (
			comparison_id, mentioned_text, mention_index, model_decisions,
			is_unanimous, divergence_type, confidence_spread,
			ground_truth_entity_id, models_correct, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`

	return r.db.QueryRow(ctx, query,
		decision.ComparisonID,
		decision.MentionedText,
		decision.MentionIndex,
		modelDecisionsJSON,
		decision.IsUnanimous,
		nullString(decision.DivergenceType),
		nullFloat32(decision.ConfidenceSpread),
		decision.GroundTruthEntityID,
		decision.ModelsCorrect,
		decision.CreatedAt,
	).Scan(&decision.ID)
}

// GetComparisonDecisions retrieves all decisions for a comparison.
func (r *PostgresRepository) GetComparisonDecisions(ctx context.Context, comparisonID string) ([]ComparisonDecision, error) {
	query := `
		SELECT id, comparison_id, mentioned_text, mention_index, model_decisions,
			is_unanimous, divergence_type, confidence_spread,
			ground_truth_entity_id, models_correct, created_at
		FROM resolution_comparison_decisions WHERE comparison_id = $1
		ORDER BY id`

	rows, err := r.db.Query(ctx, query, comparisonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanComparisonDecisions(rows)
}

// GetDivergentDecisions retrieves only divergent decisions for a comparison.
func (r *PostgresRepository) GetDivergentDecisions(ctx context.Context, comparisonID string) ([]ComparisonDecision, error) {
	query := `
		SELECT id, comparison_id, mentioned_text, mention_index, model_decisions,
			is_unanimous, divergence_type, confidence_spread,
			ground_truth_entity_id, models_correct, created_at
		FROM resolution_comparison_decisions WHERE comparison_id = $1 AND is_unanimous = false
		ORDER BY id`

	rows, err := r.db.Query(ctx, query, comparisonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanComparisonDecisions(rows)
}

func (r *PostgresRepository) scanComparisonDecisions(rows interface{ Next() bool; Scan(dest ...interface{}) error }) ([]ComparisonDecision, error) {
	var decisions []ComparisonDecision
	for rows.Next() {
		var d ComparisonDecision
		var mentionIndex sql.NullInt32
		var divergenceType sql.NullString
		var confidenceSpread sql.NullFloat64
		var groundTruthEntityID sql.NullInt64
		var modelDecisionsJSON []byte

		err := rows.Scan(
			&d.ID, &d.ComparisonID, &d.MentionedText, &mentionIndex, &modelDecisionsJSON,
			&d.IsUnanimous, &divergenceType, &confidenceSpread,
			&groundTruthEntityID, &d.ModelsCorrect, &d.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if mentionIndex.Valid {
			idx := int(mentionIndex.Int32)
			d.MentionIndex = &idx
		}
		d.DivergenceType = divergenceType.String
		d.ConfidenceSpread = float32(confidenceSpread.Float64)
		if groundTruthEntityID.Valid {
			d.GroundTruthEntityID = &groundTruthEntityID.Int64
		}
		if modelDecisionsJSON != nil {
			json.Unmarshal(modelDecisionsJSON, &d.ModelDecisions)
		}

		decisions = append(decisions, d)
	}
	return decisions, nil
}

// GetComparisonDetail retrieves full comparison details.
func (r *PostgresRepository) GetComparisonDetail(ctx context.Context, comparisonID string) (*ComparisonDetail, error) {
	comp, err := r.GetComparison(ctx, comparisonID)
	if err != nil {
		return nil, err
	}

	decisions, err := r.GetComparisonDecisions(ctx, comparisonID)
	if err != nil {
		return nil, err
	}

	detail := &ComparisonDetail{
		Comparison: *comp,
		Decisions:  decisions,
	}

	// Optionally fetch linked trace summaries
	if len(comp.TraceIDs) > 0 {
		for _, traceID := range comp.TraceIDs {
			traces, err := r.ListTraces(ctx, TraceFilter{TenantID: comp.TenantID, Limit: 1})
			if err != nil {
				continue
			}
			for _, t := range traces {
				if t.ID == traceID {
					detail.Traces = append(detail.Traces, t)
					break
				}
			}
		}
	}

	return detail, nil
}

// GetModelStats retrieves aggregate statistics for all models.
func (r *PostgresRepository) GetModelStats(ctx context.Context, tenantID string, daysSince int) ([]ModelStats, error) {
	since := time.Now().AddDate(0, 0, -daysSince)

	// Aggregate stats from comparison decisions
	query := `
		WITH model_data AS (
			SELECT
				jsonb_array_elements(cd.model_decisions) AS decision,
				c.id AS comparison_id,
				cd.ground_truth_entity_id,
				cd.models_correct
			FROM resolution_comparison_decisions cd
			JOIN resolution_comparisons c ON cd.comparison_id = c.id
			WHERE c.tenant_id = $1 AND c.started_at >= $2
		),
		expanded AS (
			SELECT
				decision->>'model' AS model,
				(decision->>'confidence')::float AS confidence,
				comparison_id,
				ground_truth_entity_id,
				CASE WHEN decision->>'model' = ANY(models_correct) THEN 1 ELSE 0 END AS is_correct
			FROM model_data
		)
		SELECT
			model,
			COUNT(DISTINCT comparison_id) AS total_comparisons,
			COUNT(*) AS total_decisions,
			SUM(is_correct) AS correct_decisions,
			AVG(confidence) AS avg_confidence
		FROM expanded
		GROUP BY model
		ORDER BY model`

	rows, err := r.db.Query(ctx, query, tenantID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []ModelStats
	for rows.Next() {
		var s ModelStats
		var avgConfidence sql.NullFloat64
		var correctDecisions sql.NullInt64

		err := rows.Scan(
			&s.Model,
			&s.TotalComparisons,
			&s.TotalDecisions,
			&correctDecisions,
			&avgConfidence,
		)
		if err != nil {
			return nil, err
		}

		s.CorrectDecisions = int(correctDecisions.Int64)
		s.AverageConfidence = float32(avgConfidence.Float64)

		// Calculate accuracy if we have ground truth data
		if s.TotalDecisions > 0 && s.CorrectDecisions > 0 {
			s.Accuracy = float32(s.CorrectDecisions) / float32(s.TotalDecisions)
		}

		stats = append(stats, s)
	}

	return stats, nil
}

// DeleteOldComparisons deletes comparisons older than specified days.
func (r *PostgresRepository) DeleteOldComparisons(ctx context.Context, olderThanDays int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays)

	result, err := r.db.Exec(ctx,
		"DELETE FROM resolution_comparisons WHERE created_at < $1",
		cutoff)
	if err != nil {
		return 0, err
	}

	return int(result.RowsAffected()), nil
}

// Helper functions
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullFloat32(f float32) sql.NullFloat64 {
	if f == 0 {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: float64(f), Valid: true}
}
