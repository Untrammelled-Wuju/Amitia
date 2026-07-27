package hook

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type HookContributionRepository struct {
	db *sql.DB
}

func NewHookContributionRepository(db *sql.DB) *HookContributionRepository {
	return &HookContributionRepository{db: db}
}

func (r *HookContributionRepository) Register(ctx context.Context, contrib HookContributionDefinition) error {
	beforeJSON, err := json.Marshal(contrib.Before)
	if err != nil {
		return fmt.Errorf("hook: marshal before: %w", err)
	}
	afterJSON, err := json.Marshal(contrib.After)
	if err != nil {
		return fmt.Errorf("hook: marshal after: %w", err)
	}
	var failurePolicyJSON string
	if contrib.FailurePolicy != nil {
		data, err := json.Marshal(contrib.FailurePolicy)
		if err != nil {
			return fmt.Errorf("hook: marshal failure policy: %w", err)
		}
		failurePolicyJSON = string(data)
	}
	mutationClaimsJSON, err := json.Marshal(contrib.MutationClaims)
	if err != nil {
		return fmt.Errorf("hook: marshal mutation claims: %w", err)
	}
	definitionJSON, err := json.Marshal(contrib)
	if err != nil {
		return fmt.Errorf("hook: marshal contribution: %w", err)
	}
	enabledOverride := 0
	if contrib.Enabled {
		enabledOverride = 1
	}
	timeoutMs := int64(contrib.Timeout / time.Millisecond)

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO extension_hook_contributions
		(contribution_id, extension_id, module_id, hook_point_id, contract_version, phase, entry, priority, before_json, after_json, timeout_ms, failure_policy_json, mutation_claims_json, enabled_override, definition_hash, definition_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(extension_id, contribution_id) DO UPDATE SET
			module_id = excluded.module_id,
			hook_point_id = excluded.hook_point_id,
			contract_version = excluded.contract_version,
			phase = excluded.phase,
			entry = excluded.entry,
			priority = excluded.priority,
			before_json = excluded.before_json,
			after_json = excluded.after_json,
			timeout_ms = excluded.timeout_ms,
			failure_policy_json = excluded.failure_policy_json,
			mutation_claims_json = excluded.mutation_claims_json,
			enabled_override = excluded.enabled_override,
			definition_hash = excluded.definition_hash,
			definition_json = excluded.definition_json
	`,
		contrib.ContributionID,
		contrib.ExtensionID,
		contrib.ModuleID,
		contrib.HookPointID,
		contrib.ContractVersion,
		string(contrib.Phase),
		contrib.Entry,
		contrib.Priority,
		string(beforeJSON),
		string(afterJSON),
		timeoutMs,
		failurePolicyJSON,
		string(mutationClaimsJSON),
		enabledOverride,
		contrib.DefinitionHash,
		string(definitionJSON),
	)
	if err != nil {
		return fmt.Errorf("hook: upsert contribution: %w", err)
	}
	return nil
}

func (r *HookContributionRepository) Unregister(ctx context.Context, contributionID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM extension_hook_contributions WHERE contribution_id = ?`, contributionID)
	if err != nil {
		return fmt.Errorf("hook: delete contribution: %w", err)
	}
	return nil
}

func (r *HookContributionRepository) Get(ctx context.Context, contributionID string) (HookContributionDefinition, error) {
	var definitionJSON string
	err := r.db.QueryRowContext(ctx, `SELECT definition_json FROM extension_hook_contributions WHERE contribution_id = ?`, contributionID).Scan(&definitionJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return HookContributionDefinition{}, ErrContributionMissing
		}
		return HookContributionDefinition{}, fmt.Errorf("hook: query contribution: %w", err)
	}
	var contrib HookContributionDefinition
	if err := json.Unmarshal([]byte(definitionJSON), &contrib); err != nil {
		return HookContributionDefinition{}, fmt.Errorf("hook: unmarshal contribution: %w", err)
	}
	return contrib, nil
}

func (r *HookContributionRepository) SetEnabled(ctx context.Context, contributionID string, enabled bool) error {
	enabledOverride := 0
	if enabled {
		enabledOverride = 1
	}
	result, err := r.db.ExecContext(ctx, `UPDATE extension_hook_contributions SET enabled_override = ? WHERE contribution_id = ?`, enabledOverride, contributionID)
	if err != nil {
		return fmt.Errorf("hook: set enabled: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("hook: set enabled rows affected: %w", err)
	}
	if rows == 0 {
		return ErrContributionMissing
	}
	return nil
}

func (r *HookContributionRepository) ListByHookPoint(ctx context.Context, hookPointID string) ([]HookContributionDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT definition_json FROM extension_hook_contributions WHERE hook_point_id = ? ORDER BY priority ASC, contribution_id ASC`, hookPointID)
	if err != nil {
		return nil, fmt.Errorf("hook: list contributions by hook point: %w", err)
	}
	defer rows.Close()
	return scanContributions(rows)
}

func (r *HookContributionRepository) ListByExtension(ctx context.Context, extensionID string) ([]HookContributionDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT definition_json FROM extension_hook_contributions WHERE extension_id = ? ORDER BY contribution_id ASC`, extensionID)
	if err != nil {
		return nil, fmt.Errorf("hook: list contributions by extension: %w", err)
	}
	defer rows.Close()
	return scanContributions(rows)
}

func (r *HookContributionRepository) List(ctx context.Context) ([]HookContributionDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT definition_json FROM extension_hook_contributions ORDER BY extension_id ASC, contribution_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("hook: list contributions: %w", err)
	}
	defer rows.Close()
	return scanContributions(rows)
}

func scanContributions(rows *sql.Rows) ([]HookContributionDefinition, error) {
	var out []HookContributionDefinition
	for rows.Next() {
		var definitionJSON string
		if err := rows.Scan(&definitionJSON); err != nil {
			return nil, fmt.Errorf("hook: scan contribution: %w", err)
		}
		var contrib HookContributionDefinition
		if err := json.Unmarshal([]byte(definitionJSON), &contrib); err != nil {
			return nil, fmt.Errorf("hook: unmarshal contribution: %w", err)
		}
		out = append(out, contrib)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hook: iterate contributions: %w", err)
	}
	return out, nil
}

var _ ContributionStore = (*HookContributionRepository)(nil)

type HookInvocationRecord struct {
	HookInvocationID     string
	OperationID          string
	ParentInvocationID   string
	ContributionID       string
	HookPointID          string
	Phase                string
	Sequence             int
	Status               string
	InputHash            string
	ResultHash           string
	Decision             string
	StartedAt            time.Time
	FinishedAt           *time.Time
	ErrorCode            string
	ErrorMessage         string
	RuntimeInstanceID    string
	ScopeSnapshotID      string
	PermissionSnapshotID string
}

type HookInvocationRepository struct {
	db *sql.DB
}

func NewHookInvocationRepository(db *sql.DB) *HookInvocationRepository {
	return &HookInvocationRepository{db: db}
}

func (r *HookInvocationRepository) Record(ctx context.Context, rec HookInvocationRecord) error {
	var finishedAt sql.NullTime
	if rec.FinishedAt != nil {
		finishedAt = sql.NullTime{Time: rec.FinishedAt.UTC(), Valid: true}
	}
	startedAt := rec.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_hook_invocations
		(hook_invocation_id, operation_id, parent_invocation_id, contribution_id, hook_point_id, phase, sequence, status, input_hash, result_hash, decision, started_at, finished_at, error_code, error_message, runtime_instance_id, scope_snapshot_id, permission_snapshot_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hook_invocation_id) DO UPDATE SET
			operation_id = excluded.operation_id,
			parent_invocation_id = excluded.parent_invocation_id,
			status = excluded.status,
			input_hash = excluded.input_hash,
			result_hash = excluded.result_hash,
			decision = excluded.decision,
			finished_at = excluded.finished_at,
			error_code = excluded.error_code,
			error_message = excluded.error_message,
			runtime_instance_id = excluded.runtime_instance_id,
			scope_snapshot_id = excluded.scope_snapshot_id,
			permission_snapshot_id = excluded.permission_snapshot_id
	`,
		rec.HookInvocationID,
		rec.OperationID,
		rec.ParentInvocationID,
		rec.ContributionID,
		rec.HookPointID,
		rec.Phase,
		rec.Sequence,
		rec.Status,
		rec.InputHash,
		rec.ResultHash,
		rec.Decision,
		startedAt,
		finishedAt,
		rec.ErrorCode,
		rec.ErrorMessage,
		rec.RuntimeInstanceID,
		rec.ScopeSnapshotID,
		rec.PermissionSnapshotID,
	)
	if err != nil {
		return fmt.Errorf("hook: upsert invocation: %w", err)
	}
	return nil
}

func (r *HookInvocationRepository) Get(ctx context.Context, invocationID string) (HookInvocationRecord, error) {
	var rec HookInvocationRecord
	var finishedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT hook_invocation_id, operation_id, parent_invocation_id, contribution_id, hook_point_id, phase, sequence, status, input_hash, result_hash, decision, started_at, finished_at, error_code, error_message, runtime_instance_id, scope_snapshot_id, permission_snapshot_id
		FROM extension_hook_invocations WHERE hook_invocation_id = ?
	`, invocationID).Scan(
		&rec.HookInvocationID,
		&rec.OperationID,
		&rec.ParentInvocationID,
		&rec.ContributionID,
		&rec.HookPointID,
		&rec.Phase,
		&rec.Sequence,
		&rec.Status,
		&rec.InputHash,
		&rec.ResultHash,
		&rec.Decision,
		&rec.StartedAt,
		&finishedAt,
		&rec.ErrorCode,
		&rec.ErrorMessage,
		&rec.RuntimeInstanceID,
		&rec.ScopeSnapshotID,
		&rec.PermissionSnapshotID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return HookInvocationRecord{}, fmt.Errorf("hook: invocation not found: %s", invocationID)
		}
		return HookInvocationRecord{}, fmt.Errorf("hook: query invocation: %w", err)
	}
	if finishedAt.Valid {
		t := finishedAt.Time.UTC()
		rec.FinishedAt = &t
	}
	return rec, nil
}

func (r *HookInvocationRepository) ListByOperation(ctx context.Context, operationID string) ([]HookInvocationRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT hook_invocation_id, operation_id, parent_invocation_id, contribution_id, hook_point_id, phase, sequence, status, input_hash, result_hash, decision, started_at, finished_at, error_code, error_message, runtime_instance_id, scope_snapshot_id, permission_snapshot_id
		FROM extension_hook_invocations WHERE operation_id = ? ORDER BY sequence ASC
	`, operationID)
	if err != nil {
		return nil, fmt.Errorf("hook: list invocations by operation: %w", err)
	}
	defer rows.Close()
	return scanInvocations(rows)
}

func (r *HookInvocationRepository) ListByContribution(ctx context.Context, contributionID string) ([]HookInvocationRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT hook_invocation_id, operation_id, parent_invocation_id, contribution_id, hook_point_id, phase, sequence, status, input_hash, result_hash, decision, started_at, finished_at, error_code, error_message, runtime_instance_id, scope_snapshot_id, permission_snapshot_id
		FROM extension_hook_invocations WHERE contribution_id = ? ORDER BY started_at DESC
	`, contributionID)
	if err != nil {
		return nil, fmt.Errorf("hook: list invocations by contribution: %w", err)
	}
	defer rows.Close()
	return scanInvocations(rows)
}

func scanInvocations(rows *sql.Rows) ([]HookInvocationRecord, error) {
	var out []HookInvocationRecord
	for rows.Next() {
		var rec HookInvocationRecord
		var finishedAt sql.NullTime
		err := rows.Scan(
			&rec.HookInvocationID,
			&rec.OperationID,
			&rec.ParentInvocationID,
			&rec.ContributionID,
			&rec.HookPointID,
			&rec.Phase,
			&rec.Sequence,
			&rec.Status,
			&rec.InputHash,
			&rec.ResultHash,
			&rec.Decision,
			&rec.StartedAt,
			&finishedAt,
			&rec.ErrorCode,
			&rec.ErrorMessage,
			&rec.RuntimeInstanceID,
			&rec.ScopeSnapshotID,
			&rec.PermissionSnapshotID,
		)
		if err != nil {
			return nil, fmt.Errorf("hook: scan invocation: %w", err)
		}
		if finishedAt.Valid {
			t := finishedAt.Time.UTC()
			rec.FinishedAt = &t
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hook: iterate invocations: %w", err)
	}
	return out, nil
}

type HookMutationRecord struct {
	HookInvocationID string
	Path             string
	Operation        string
	BeforeHash       string
	AfterHash        string
	Applied          bool
	Conflict         bool
}

type HookMutationRepository struct {
	db *sql.DB
}

func NewHookMutationRepository(db *sql.DB) *HookMutationRepository {
	return &HookMutationRepository{db: db}
}

func (r *HookMutationRepository) Record(ctx context.Context, rec HookMutationRecord) error {
	applied := 0
	if rec.Applied {
		applied = 1
	}
	conflict := 0
	if rec.Conflict {
		conflict = 1
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_hook_mutations
		(hook_invocation_id, path, operation, before_hash, after_hash, applied, conflict)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		rec.HookInvocationID,
		rec.Path,
		rec.Operation,
		rec.BeforeHash,
		rec.AfterHash,
		applied,
		conflict,
	)
	if err != nil {
		return fmt.Errorf("hook: insert mutation: %w", err)
	}
	return nil
}

func (r *HookMutationRepository) ListByInvocation(ctx context.Context, invocationID string) ([]HookMutationRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT hook_invocation_id, path, operation, before_hash, after_hash, applied, conflict FROM extension_hook_mutations WHERE hook_invocation_id = ? ORDER BY rowid ASC`, invocationID)
	if err != nil {
		return nil, fmt.Errorf("hook: list mutations: %w", err)
	}
	defer rows.Close()
	var out []HookMutationRecord
	for rows.Next() {
		var rec HookMutationRecord
		var applied, conflict int
		if err := rows.Scan(&rec.HookInvocationID, &rec.Path, &rec.Operation, &rec.BeforeHash, &rec.AfterHash, &applied, &conflict); err != nil {
			return nil, fmt.Errorf("hook: scan mutation: %w", err)
		}
		rec.Applied = applied != 0
		rec.Conflict = conflict != 0
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hook: iterate mutations: %w", err)
	}
	return out, nil
}

type HookCircuitRepository struct {
	db *sql.DB
}

func NewHookCircuitRepository(db *sql.DB) *HookCircuitRepository {
	return &HookCircuitRepository{db: db}
}

func (r *HookCircuitRepository) Save(ctx context.Context, contributionID string, stats CircuitStats) error {
	var lastFailTime, openedAt sql.NullTime
	if !stats.LastFailTime.IsZero() {
		lastFailTime = sql.NullTime{Time: stats.LastFailTime.UTC(), Valid: true}
	}
	if !stats.OpenedAt.IsZero() {
		openedAt = sql.NullTime{Time: stats.OpenedAt.UTC(), Valid: true}
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_hook_circuits
		(contribution_id, state, consecutive_fails, total_fails, total_success, last_fail_code, last_fail_time, opened_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(contribution_id) DO UPDATE SET
			state = excluded.state,
			consecutive_fails = excluded.consecutive_fails,
			total_fails = excluded.total_fails,
			total_success = excluded.total_success,
			last_fail_code = excluded.last_fail_code,
			last_fail_time = excluded.last_fail_time,
			opened_at = excluded.opened_at,
			updated_at = excluded.updated_at
	`,
		contributionID,
		string(stats.State),
		stats.ConsecutiveFails,
		stats.TotalFails,
		stats.TotalSuccess,
		stats.LastFailCode,
		lastFailTime,
		openedAt,
	)
	if err != nil {
		return fmt.Errorf("hook: upsert circuit: %w", err)
	}
	return nil
}

func (r *HookCircuitRepository) Get(ctx context.Context, contributionID string) (CircuitStats, error) {
	var stats CircuitStats
	var state string
	var lastFailTime, openedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `SELECT state, consecutive_fails, total_fails, total_success, last_fail_code, last_fail_time, opened_at FROM extension_hook_circuits WHERE contribution_id = ?`, contributionID).Scan(
		&state,
		&stats.ConsecutiveFails,
		&stats.TotalFails,
		&stats.TotalSuccess,
		&stats.LastFailCode,
		&lastFailTime,
		&openedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return CircuitStats{State: CircuitClosed}, nil
		}
		return CircuitStats{}, fmt.Errorf("hook: query circuit: %w", err)
	}
	stats.State = CircuitState(state)
	if lastFailTime.Valid {
		stats.LastFailTime = lastFailTime.Time.UTC()
	}
	if openedAt.Valid {
		stats.OpenedAt = openedAt.Time.UTC()
	}
	return stats, nil
}

func (r *HookCircuitRepository) List(ctx context.Context) (map[string]CircuitStats, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT contribution_id, state, consecutive_fails, total_fails, total_success, last_fail_code, last_fail_time, opened_at FROM extension_hook_circuits`)
	if err != nil {
		return nil, fmt.Errorf("hook: list circuits: %w", err)
	}
	defer rows.Close()
	out := make(map[string]CircuitStats)
	for rows.Next() {
		var contributionID string
		var state string
		var stats CircuitStats
		var lastFailTime, openedAt sql.NullTime
		if err := rows.Scan(&contributionID, &state, &stats.ConsecutiveFails, &stats.TotalFails, &stats.TotalSuccess, &stats.LastFailCode, &lastFailTime, &openedAt); err != nil {
			return nil, fmt.Errorf("hook: scan circuit: %w", err)
		}
		stats.State = CircuitState(state)
		if lastFailTime.Valid {
			stats.LastFailTime = lastFailTime.Time.UTC()
		}
		if openedAt.Valid {
			stats.OpenedAt = openedAt.Time.UTC()
		}
		out[contributionID] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hook: iterate circuits: %w", err)
	}
	return out, nil
}

type HookTraceRecorder struct {
	invocations *HookInvocationRepository
	mutations   *HookMutationRepository
	circuits    *HookCircuitRepository
}

func NewHookTraceRecorder(db *sql.DB) *HookTraceRecorder {
	return &HookTraceRecorder{
		invocations: NewHookInvocationRepository(db),
		mutations:   NewHookMutationRepository(db),
		circuits:    NewHookCircuitRepository(db),
	}
}

func (r *HookTraceRecorder) RecordInvocation(ctx context.Context, exec HookExecution, inputHash, resultHash string) {
	invocationID := generateInvocationID(exec.ContributionID, exec.StartedAt, exec.Sequence)
	startedAt := parseStartedAt(exec.StartedAt)
	finishedAt := startedAt.Add(time.Duration(exec.DurationMs) * time.Millisecond)
	rec := HookInvocationRecord{
		HookInvocationID: invocationID,
		ContributionID:   exec.ContributionID,
		Phase:            string(exec.Phase),
		Sequence:         exec.Sequence,
		Status:           exec.Status,
		InputHash:        inputHash,
		ResultHash:       resultHash,
		Decision:         string(exec.Decision),
		StartedAt:        startedAt,
		FinishedAt:       &finishedAt,
		ErrorCode:        exec.ErrorCode,
		ErrorMessage:     exec.Error,
	}
	_ = r.invocations.Record(ctx, rec)
}

func (r *HookTraceRecorder) RecordMutation(ctx context.Context, invocationID string, op MutationOperation, beforeHash, afterHash string, applied bool, conflict bool) {
	rec := HookMutationRecord{
		HookInvocationID: invocationID,
		Path:             op.Path,
		Operation:        op.Operation,
		BeforeHash:       beforeHash,
		AfterHash:        afterHash,
		Applied:          applied,
		Conflict:         conflict,
	}
	_ = r.mutations.Record(ctx, rec)
}

func (r *HookTraceRecorder) RecordPipeline(ctx context.Context, result PipelineResult) {
	for _, exec := range result.Executions {
		invocationID := generateInvocationID(exec.ContributionID, exec.StartedAt, exec.Sequence)
		startedAt := parseStartedAt(exec.StartedAt)
		finishedAt := startedAt.Add(time.Duration(exec.DurationMs) * time.Millisecond)
		rec := HookInvocationRecord{
			HookInvocationID: invocationID,
			OperationID:      result.OperationID,
			ContributionID:   exec.ContributionID,
			HookPointID:      result.HookPointID,
			Phase:            string(exec.Phase),
			Sequence:         exec.Sequence,
			Status:           exec.Status,
			InputHash:        exec.InputHash,
			ResultHash:       exec.ResultHash,
			Decision:         string(exec.Decision),
			StartedAt:        startedAt,
			FinishedAt:       &finishedAt,
			ErrorCode:        exec.ErrorCode,
			ErrorMessage:     exec.Error,
		}
		_ = r.invocations.Record(ctx, rec)
	}
}

func (r *HookTraceRecorder) RecordCircuit(ctx context.Context, contributionID string, stats CircuitStats) {
	_ = r.circuits.Save(ctx, contributionID, stats)
}

var _ TraceRecorder = (*HookTraceRecorder)(nil)

func generateInvocationID(contributionID, startedAt string, sequence int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", contributionID, startedAt, sequence)))
	return "hinv-" + hex.EncodeToString(h[:12])
}

func parseStartedAt(s string) time.Time {
	if s == "" {
		return time.Now().UTC()
	}
	formats := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05", "2006-01-02T15:04:05"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC()
		}
	}
	return time.Now().UTC()
}
