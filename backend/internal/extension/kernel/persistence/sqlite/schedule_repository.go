package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

type ScheduleRepository struct {
	db *sql.DB
}

func NewScheduleRepository(db *sql.DB) *ScheduleRepository {
	return &ScheduleRepository{db: db}
}

var _ schedule.ScheduleStore = (*ScheduleRepository)(nil)

type scanner interface {
	Scan(dest ...interface{}) error
}

func (r *ScheduleRepository) PutDefinition(ctx context.Context, def *schedule.ScheduleContributionDefinition) error {
	triggerJSON, err := json.Marshal(def.Trigger)
	if err != nil {
		return fmt.Errorf("sqlite: marshal schedule trigger: %w", err)
	}
	targetJSON, err := json.Marshal(def.Target)
	if err != nil {
		return fmt.Errorf("sqlite: marshal schedule target: %w", err)
	}
	misfireJSON, err := json.Marshal(def.MisfirePolicy)
	if err != nil {
		return fmt.Errorf("sqlite: marshal schedule misfire policy: %w", err)
	}
	overlapJSON, err := json.Marshal(def.OverlapPolicy)
	if err != nil {
		return fmt.Errorf("sqlite: marshal schedule overlap policy: %w", err)
	}
	retryJSON, err := json.Marshal(def.RetryPolicy)
	if err != nil {
		return fmt.Errorf("sqlite: marshal schedule retry policy: %w", err)
	}
	jitterJSON, err := json.Marshal(def.JitterPolicy)
	if err != nil {
		return fmt.Errorf("sqlite: marshal schedule jitter policy: %w", err)
	}
	concurrencyJSON, err := json.Marshal(def.ConcurrencyPolicy)
	if err != nil {
		return fmt.Errorf("sqlite: marshal schedule concurrency policy: %w", err)
	}

	var permReqJSON interface{}
	if len(def.PermissionRequirements) > 0 {
		b, err := json.Marshal(def.PermissionRequirements)
		if err != nil {
			return fmt.Errorf("sqlite: marshal schedule permission requirements: %w", err)
		}
		permReqJSON = string(b)
	}

	var scopeRuleJSON interface{}
	{
		b, err := json.Marshal(def.ScopeRule)
		if err != nil {
			return fmt.Errorf("sqlite: marshal schedule scope rule: %w", err)
		}
		scopeRuleJSON = string(b)
	}

	var depReqJSON interface{}
	if len(def.DependencyRequirements) > 0 {
		b, err := json.Marshal(def.DependencyRequirements)
		if err != nil {
			return fmt.Errorf("sqlite: marshal schedule dependency requirements: %w", err)
		}
		depReqJSON = string(b)
	}

	now := time.Now().UTC()
	ex := getExecutor(ctx, r.db)
	_, err = ex.ExecContext(ctx, `
		INSERT INTO extension_schedule_definitions
			(schedule_id, contribution_id, extension_id, module_id, name, description,
			 trigger_type, trigger_json, target_type, target_json, timezone,
			 start_at, end_at, misfire_policy, overlap_policy,
			 retry_policy_json, jitter_policy_json, concurrency_policy_json,
			 permission_requirements_json, scope_rule_json, dependency_requirements_json,
			 dst_spring_policy, dst_fall_policy, definition_hash, version,
			 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(schedule_id) DO UPDATE SET
			contribution_id = excluded.contribution_id,
			extension_id = excluded.extension_id,
			module_id = excluded.module_id,
			name = excluded.name,
			description = excluded.description,
			trigger_type = excluded.trigger_type,
			trigger_json = excluded.trigger_json,
			target_type = excluded.target_type,
			target_json = excluded.target_json,
			timezone = excluded.timezone,
			start_at = excluded.start_at,
			end_at = excluded.end_at,
			misfire_policy = excluded.misfire_policy,
			overlap_policy = excluded.overlap_policy,
			retry_policy_json = excluded.retry_policy_json,
			jitter_policy_json = excluded.jitter_policy_json,
			concurrency_policy_json = excluded.concurrency_policy_json,
			permission_requirements_json = excluded.permission_requirements_json,
			scope_rule_json = excluded.scope_rule_json,
			dependency_requirements_json = excluded.dependency_requirements_json,
			dst_spring_policy = excluded.dst_spring_policy,
			dst_fall_policy = excluded.dst_fall_policy,
			definition_hash = excluded.definition_hash,
			version = excluded.version,
			updated_at = excluded.updated_at
	`,
		def.ScheduleID, def.ContributionID, def.ExtensionID, def.ModuleID,
		def.Name, def.Description,
		string(def.Trigger.Type), string(triggerJSON),
		string(def.Target.Type), string(targetJSON),
		def.Timezone,
		nullableTime(def.StartAt), nullableTime(def.EndAt),
		string(misfireJSON), string(overlapJSON),
		string(retryJSON), string(jitterJSON), string(concurrencyJSON),
		permReqJSON, scopeRuleJSON, depReqJSON,
		string(def.DSTSpringPolicy), string(def.DSTFallPolicy),
		def.DefinitionHash, def.Version,
		now, now,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert schedule definition: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) GetDefinition(ctx context.Context, scheduleID string) (*schedule.ScheduleContributionDefinition, error) {
	ex := getExecutor(ctx, r.db)
	row := ex.QueryRowContext(ctx, `
		SELECT schedule_id, contribution_id, extension_id, module_id, name, description,
		       trigger_type, trigger_json, target_type, target_json, timezone,
		       start_at, end_at, misfire_policy, overlap_policy,
		       retry_policy_json, jitter_policy_json, concurrency_policy_json,
		       permission_requirements_json, scope_rule_json, dependency_requirements_json,
		       dst_spring_policy, dst_fall_policy, definition_hash, version,
		       created_at, updated_at
		FROM extension_schedule_definitions WHERE schedule_id = ?
	`, scheduleID)
	def, err := scanScheduleDefinition(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: schedule definition not found: %s: %w", scheduleID, schedule.ErrScheduleNotFound)
		}
		return nil, err
	}
	return def, nil
}

func (r *ScheduleRepository) ListDefinitions(ctx context.Context, extensionID string) ([]*schedule.ScheduleContributionDefinition, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT schedule_id, contribution_id, extension_id, module_id, name, description,
		       trigger_type, trigger_json, target_type, target_json, timezone,
		       start_at, end_at, misfire_policy, overlap_policy,
		       retry_policy_json, jitter_policy_json, concurrency_policy_json,
		       permission_requirements_json, scope_rule_json, dependency_requirements_json,
		       dst_spring_policy, dst_fall_policy, definition_hash, version,
		       created_at, updated_at
		FROM extension_schedule_definitions WHERE extension_id = ?
		ORDER BY created_at DESC
	`, extensionID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list schedule definitions: %w", err)
	}
	defer rows.Close()
	var out []*schedule.ScheduleContributionDefinition
	for rows.Next() {
		def, err := scanScheduleDefinition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, def)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) ListAllDefinitions(ctx context.Context) ([]*schedule.ScheduleContributionDefinition, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT schedule_id, contribution_id, extension_id, module_id, name, description,
		       trigger_type, trigger_json, target_type, target_json, timezone,
		       start_at, end_at, misfire_policy, overlap_policy,
		       retry_policy_json, jitter_policy_json, concurrency_policy_json,
		       permission_requirements_json, scope_rule_json, dependency_requirements_json,
		       dst_spring_policy, dst_fall_policy, definition_hash, version,
		       created_at, updated_at
		FROM extension_schedule_definitions ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list all schedule definitions: %w", err)
	}
	defer rows.Close()
	var out []*schedule.ScheduleContributionDefinition
	for rows.Next() {
		def, err := scanScheduleDefinition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, def)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) DeleteDefinition(ctx context.Context, scheduleID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_schedule_definitions WHERE schedule_id = ?`, scheduleID)
	if err != nil {
		return fmt.Errorf("sqlite: delete schedule definition: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) PutState(ctx context.Context, state *schedule.ScheduleState) error {
	now := time.Now().UTC()
	updatedAt := state.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = now
	}
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_schedule_states
			(schedule_id, enabled, paused, status, last_scheduled_at, last_triggered_at,
			 last_finished_at, next_scheduled_at, next_effective_at, last_result,
			 failure_count, generation, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(schedule_id) DO UPDATE SET
			enabled = excluded.enabled, paused = excluded.paused, status = excluded.status,
			last_scheduled_at = excluded.last_scheduled_at,
			last_triggered_at = excluded.last_triggered_at,
			last_finished_at = excluded.last_finished_at,
			next_scheduled_at = excluded.next_scheduled_at,
			next_effective_at = excluded.next_effective_at,
			last_result = excluded.last_result,
			failure_count = excluded.failure_count,
			generation = excluded.generation,
			updated_at = excluded.updated_at
	`,
		state.ScheduleID, boolToInt(state.Enabled), boolToInt(state.Paused), string(state.Status),
		nullableTime(state.LastScheduledAt), nullableTime(state.LastTriggeredAt),
		nullableTime(state.LastFinishedAt), nullableTime(state.NextScheduledAt),
		nullableTime(state.NextEffectiveAt), state.LastResult,
		state.FailureCount, state.Generation, updatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert schedule state: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) GetState(ctx context.Context, scheduleID string) (*schedule.ScheduleState, error) {
	ex := getExecutor(ctx, r.db)
	row := ex.QueryRowContext(ctx, `
		SELECT schedule_id, enabled, paused, status, last_scheduled_at, last_triggered_at,
		       last_finished_at, next_scheduled_at, next_effective_at, last_result,
		       failure_count, generation, updated_at
		FROM extension_schedule_states WHERE schedule_id = ?
	`, scheduleID)
	state, err := scanScheduleState(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: schedule state not found: %s: %w", scheduleID, schedule.ErrScheduleNotFound)
		}
		return nil, err
	}
	return state, nil
}

func (r *ScheduleRepository) ListDueStates(ctx context.Context, now time.Time, limit int) ([]*schedule.ScheduleState, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT schedule_id, enabled, paused, status, last_scheduled_at, last_triggered_at,
		       last_finished_at, next_scheduled_at, next_effective_at, last_result,
		       failure_count, generation, updated_at
		FROM extension_schedule_states
		WHERE enabled = 1 AND paused = 0 AND status = 'enabled' AND next_effective_at <= ?
		ORDER BY next_effective_at
		LIMIT ?
	`, now.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list due schedule states: %w", err)
	}
	defer rows.Close()
	var out []*schedule.ScheduleState
	for rows.Next() {
		state, err := scanScheduleState(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, state)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) ListStatesByStatus(ctx context.Context, status schedule.ScheduleDefinitionStatus) ([]*schedule.ScheduleState, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT schedule_id, enabled, paused, status, last_scheduled_at, last_triggered_at,
		       last_finished_at, next_scheduled_at, next_effective_at, last_result,
		       failure_count, generation, updated_at
		FROM extension_schedule_states WHERE status = ?
		ORDER BY updated_at DESC
	`, string(status))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list schedule states by status: %w", err)
	}
	defer rows.Close()
	var out []*schedule.ScheduleState
	for rows.Next() {
		state, err := scanScheduleState(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, state)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) PutTrigger(ctx context.Context, record *schedule.ScheduleTriggerRecord) error {
	if record.TriggerID == "" {
		record.TriggerID = uuid.New().String()
	}
	now := time.Now().UTC()
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	updatedAt := record.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = now
	}
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_schedule_triggers
			(trigger_id, schedule_id, scheduled_at, effective_at, triggered_at, idempotency_key,
			 status, lease_owner, lease_expires_at, scope_snapshot_id, permission_snapshot_id,
			 dependency_snapshot_id, operation_id, invocation_id, attempt, generation, manual,
			 error_code, error_message, jitter_applied_ms, misfire_decision, overlap_decision,
			 dst_decision, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(trigger_id) DO UPDATE SET
			schedule_id = excluded.schedule_id, scheduled_at = excluded.scheduled_at,
			effective_at = excluded.effective_at, triggered_at = excluded.triggered_at,
			idempotency_key = excluded.idempotency_key, status = excluded.status,
			lease_owner = excluded.lease_owner, lease_expires_at = excluded.lease_expires_at,
			scope_snapshot_id = excluded.scope_snapshot_id,
			permission_snapshot_id = excluded.permission_snapshot_id,
			dependency_snapshot_id = excluded.dependency_snapshot_id,
			operation_id = excluded.operation_id, invocation_id = excluded.invocation_id,
			attempt = excluded.attempt, generation = excluded.generation, manual = excluded.manual,
			error_code = excluded.error_code, error_message = excluded.error_message,
			jitter_applied_ms = excluded.jitter_applied_ms,
			misfire_decision = excluded.misfire_decision,
			overlap_decision = excluded.overlap_decision,
			dst_decision = excluded.dst_decision, updated_at = excluded.updated_at
	`,
		record.TriggerID, record.ScheduleID, record.ScheduledAt.UTC(), record.EffectiveAt.UTC(),
		nullableTime(record.TriggeredAt), record.IdempotencyKey,
		string(record.Status), nullableString(record.LeaseOwner), nullableTime(record.LeaseExpiresAt),
		record.ScopeSnapshotID, record.PermissionSnapshotID, record.DependencySnapshotID,
		nullableString(record.OperationID), nullableString(record.InvocationID),
		record.Attempt, record.Generation, boolToInt(record.Manual),
		nullableString(record.ErrorCode), nullableString(record.ErrorMessage),
		record.JitterApplied.Milliseconds(), record.MisfireDecision, record.OverlapDecision,
		record.DSTDecision, createdAt, updatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert schedule trigger: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) GetTrigger(ctx context.Context, triggerID string) (*schedule.ScheduleTriggerRecord, error) {
	ex := getExecutor(ctx, r.db)
	row := ex.QueryRowContext(ctx, `
		SELECT trigger_id, schedule_id, scheduled_at, effective_at, triggered_at, idempotency_key,
		       status, lease_owner, lease_expires_at, scope_snapshot_id, permission_snapshot_id,
		       dependency_snapshot_id, operation_id, invocation_id, attempt, generation, manual,
		       error_code, error_message, jitter_applied_ms, misfire_decision, overlap_decision,
		       dst_decision, created_at, updated_at
		FROM extension_schedule_triggers WHERE trigger_id = ?
	`, triggerID)
	record, err := scanScheduleTrigger(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: schedule trigger not found: %s: %w", triggerID, schedule.ErrTriggerNotFound)
		}
		return nil, err
	}
	return record, nil
}

func (r *ScheduleRepository) GetTriggerByIdempotencyKey(ctx context.Context, key string) (*schedule.ScheduleTriggerRecord, error) {
	ex := getExecutor(ctx, r.db)
	row := ex.QueryRowContext(ctx, `
		SELECT trigger_id, schedule_id, scheduled_at, effective_at, triggered_at, idempotency_key,
		       status, lease_owner, lease_expires_at, scope_snapshot_id, permission_snapshot_id,
		       dependency_snapshot_id, operation_id, invocation_id, attempt, generation, manual,
		       error_code, error_message, jitter_applied_ms, misfire_decision, overlap_decision,
		       dst_decision, created_at, updated_at
		FROM extension_schedule_triggers WHERE idempotency_key = ?
	`, key)
	record, err := scanScheduleTrigger(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: schedule trigger not found by idempotency key: %s: %w", key, schedule.ErrTriggerNotFound)
		}
		return nil, err
	}
	return record, nil
}

func (r *ScheduleRepository) ListTriggersBySchedule(ctx context.Context, scheduleID string, limit int) ([]*schedule.ScheduleTriggerRecord, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT trigger_id, schedule_id, scheduled_at, effective_at, triggered_at, idempotency_key,
		       status, lease_owner, lease_expires_at, scope_snapshot_id, permission_snapshot_id,
		       dependency_snapshot_id, operation_id, invocation_id, attempt, generation, manual,
		       error_code, error_message, jitter_applied_ms, misfire_decision, overlap_decision,
		       dst_decision, created_at, updated_at
		FROM extension_schedule_triggers WHERE schedule_id = ?
		ORDER BY scheduled_at DESC LIMIT ?
	`, scheduleID, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list schedule triggers by schedule: %w", err)
	}
	defer rows.Close()
	var out []*schedule.ScheduleTriggerRecord
	for rows.Next() {
		record, err := scanScheduleTrigger(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) ListDueTriggers(ctx context.Context, now time.Time, limit int) ([]*schedule.ScheduleTriggerRecord, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT trigger_id, schedule_id, scheduled_at, effective_at, triggered_at, idempotency_key,
		       status, lease_owner, lease_expires_at, scope_snapshot_id, permission_snapshot_id,
		       dependency_snapshot_id, operation_id, invocation_id, attempt, generation, manual,
		       error_code, error_message, jitter_applied_ms, misfire_decision, overlap_decision,
		       dst_decision, created_at, updated_at
		FROM extension_schedule_triggers
		WHERE status IN ('waiting', 'due') AND effective_at <= ?
		ORDER BY effective_at LIMIT ?
	`, now.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list due schedule triggers: %w", err)
	}
	defer rows.Close()
	var out []*schedule.ScheduleTriggerRecord
	for rows.Next() {
		record, err := scanScheduleTrigger(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) ListTriggersByStatuses(ctx context.Context, statuses []schedule.ScheduleRunStatus, limit int) ([]*schedule.ScheduleTriggerRecord, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	placeholders := make([]string, len(statuses))
	args := make([]any, 0, len(statuses)+1)
	for index, status := range statuses {
		placeholders[index] = "?"
		args = append(args, string(status))
	}
	args = append(args, limit)
	query := `
		SELECT trigger_id, schedule_id, scheduled_at, effective_at, triggered_at, idempotency_key,
		       status, lease_owner, lease_expires_at, scope_snapshot_id, permission_snapshot_id,
		       dependency_snapshot_id, operation_id, invocation_id, attempt, generation, manual,
		       error_code, error_message, jitter_applied_ms, misfire_decision, overlap_decision,
		       dst_decision, created_at, updated_at
		FROM extension_schedule_triggers
		WHERE status IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY updated_at LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list schedule triggers by status: %w", err)
	}
	defer rows.Close()
	result := make([]*schedule.ScheduleTriggerRecord, 0)
	for rows.Next() {
		record, scanErr := scanScheduleTrigger(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

func (r *ScheduleRepository) UpdateTriggerStatus(ctx context.Context, triggerID string, status schedule.ScheduleRunStatus, updates map[string]any) error {
	allowed := map[string]bool{
		"triggered_at": true, "lease_owner": true, "lease_expires_at": true,
		"operation_id": true, "invocation_id": true, "attempt": true,
		"error_code": true, "error_message": true,
		"misfire_decision": true, "overlap_decision": true, "dst_decision": true,
	}
	setParts := []string{"status = ?", "updated_at = ?"}
	args := []any{string(status), time.Now().UTC()}
	for col, val := range updates {
		if !allowed[col] {
			continue
		}
		setParts = append(setParts, col+" = ?")
		args = append(args, toSQLValue(val))
	}
	args = append(args, triggerID)
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx,
		fmt.Sprintf("UPDATE extension_schedule_triggers SET %s WHERE trigger_id = ?", strings.Join(setParts, ", ")),
		args...,
	)
	if err != nil {
		return fmt.Errorf("sqlite: update schedule trigger status: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) AcquireTriggerLease(ctx context.Context, triggerID string, owner string, expiresAt time.Time) (bool, error) {
	now := time.Now().UTC()
	ex := getExecutor(ctx, r.db)
	res, err := ex.ExecContext(ctx, `
		UPDATE extension_schedule_triggers
		SET lease_owner = ?, lease_expires_at = ?, status = 'leased', updated_at = ?
		WHERE trigger_id = ? AND (lease_owner IS NULL OR lease_expires_at <= ?)
	`, owner, expiresAt.UTC(), now, triggerID, now)
	if err != nil {
		return false, fmt.Errorf("sqlite: acquire schedule trigger lease: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("sqlite: acquire schedule trigger lease rows affected: %w", err)
	}
	return n > 0, nil
}

func (r *ScheduleRepository) ReleaseTriggerLease(ctx context.Context, triggerID string) error {
	now := time.Now().UTC()
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		UPDATE extension_schedule_triggers
		SET lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE trigger_id = ?
	`, now, triggerID)
	if err != nil {
		return fmt.Errorf("sqlite: release schedule trigger lease: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) ReclaimExpiredLeases(ctx context.Context, now time.Time) (int, error) {
	ex := getExecutor(ctx, r.db)
	res, err := ex.ExecContext(ctx, `
		UPDATE extension_schedule_triggers
		SET lease_owner = NULL, lease_expires_at = NULL, status = 'waiting', updated_at = ?
		WHERE lease_expires_at <= ? AND status = 'leased'
	`, now.UTC(), now.UTC())
	if err != nil {
		return 0, fmt.Errorf("sqlite: reclaim expired schedule trigger leases: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *ScheduleRepository) DeleteTriggersBySchedule(ctx context.Context, scheduleID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_schedule_triggers WHERE schedule_id = ?`, scheduleID)
	if err != nil {
		return fmt.Errorf("sqlite: delete schedule triggers by schedule: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) PutRun(ctx context.Context, run *schedule.ScheduleRunRecord) error {
	if run.RunID == "" {
		run.RunID = uuid.New().String()
	}
	now := time.Now().UTC()
	createdAt := run.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	updatedAt := run.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = now
	}
	var resultJSON interface{}
	if len(run.ResultJSON) > 0 {
		resultJSON = string(run.ResultJSON)
	}
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_schedule_runs
			(run_id, trigger_id, schedule_id, status, attempt, started_at, finished_at,
			 operation_id, invocation_id, target_type, target_id, result_json,
			 error_code, error_message, generation, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO UPDATE SET
			trigger_id = excluded.trigger_id, schedule_id = excluded.schedule_id,
			status = excluded.status, attempt = excluded.attempt,
			started_at = excluded.started_at, finished_at = excluded.finished_at,
			operation_id = excluded.operation_id, invocation_id = excluded.invocation_id,
			target_type = excluded.target_type, target_id = excluded.target_id,
			result_json = excluded.result_json, error_code = excluded.error_code,
			error_message = excluded.error_message, generation = excluded.generation,
			updated_at = excluded.updated_at
	`,
		run.RunID, run.TriggerID, run.ScheduleID, string(run.Status), run.Attempt,
		run.StartedAt.UTC(), nullableTime(run.FinishedAt),
		run.OperationID, run.InvocationID,
		string(run.TargetType), run.TargetID, resultJSON,
		nullableString(run.ErrorCode), nullableString(run.ErrorMessage),
		run.Generation, createdAt, updatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert schedule run: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) GetRun(ctx context.Context, runID string) (*schedule.ScheduleRunRecord, error) {
	ex := getExecutor(ctx, r.db)
	row := ex.QueryRowContext(ctx, `
		SELECT run_id, trigger_id, schedule_id, status, attempt, started_at, finished_at,
		       operation_id, invocation_id, target_type, target_id, result_json,
		       error_code, error_message, generation, created_at, updated_at
		FROM extension_schedule_runs WHERE run_id = ?
	`, runID)
	run, err := scanScheduleRun(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: schedule run not found: %s: %w", runID, schedule.ErrScheduleNotFound)
		}
		return nil, err
	}
	return run, nil
}

func (r *ScheduleRepository) ListRunsBySchedule(ctx context.Context, scheduleID string, limit int) ([]*schedule.ScheduleRunRecord, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT run_id, trigger_id, schedule_id, status, attempt, started_at, finished_at,
		       operation_id, invocation_id, target_type, target_id, result_json,
		       error_code, error_message, generation, created_at, updated_at
		FROM extension_schedule_runs WHERE schedule_id = ?
		ORDER BY started_at DESC LIMIT ?
	`, scheduleID, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list schedule runs by schedule: %w", err)
	}
	defer rows.Close()
	var out []*schedule.ScheduleRunRecord
	for rows.Next() {
		run, err := scanScheduleRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) ListRunsByTrigger(ctx context.Context, triggerID string) ([]*schedule.ScheduleRunRecord, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT run_id, trigger_id, schedule_id, status, attempt, started_at, finished_at,
		       operation_id, invocation_id, target_type, target_id, result_json,
		       error_code, error_message, generation, created_at, updated_at
		FROM extension_schedule_runs WHERE trigger_id = ?
		ORDER BY started_at DESC
	`, triggerID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list schedule runs by trigger: %w", err)
	}
	defer rows.Close()
	var out []*schedule.ScheduleRunRecord
	for rows.Next() {
		run, err := scanScheduleRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) UpdateRunStatus(ctx context.Context, runID string, status schedule.ScheduleRunStatus, updates map[string]any) error {
	allowed := map[string]bool{
		"attempt": true, "finished_at": true, "operation_id": true,
		"invocation_id": true, "result_json": true,
		"error_code": true, "error_message": true,
	}
	setParts := []string{"status = ?", "updated_at = ?"}
	args := []any{string(status), time.Now().UTC()}
	for col, val := range updates {
		if !allowed[col] {
			continue
		}
		setParts = append(setParts, col+" = ?")
		args = append(args, toSQLValue(val))
	}
	args = append(args, runID)
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx,
		fmt.Sprintf("UPDATE extension_schedule_runs SET %s WHERE run_id = ?", strings.Join(setParts, ", ")),
		args...,
	)
	if err != nil {
		return fmt.Errorf("sqlite: update schedule run status: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) CountActiveRuns(ctx context.Context, scheduleID string) (int, error) {
	ex := getExecutor(ctx, r.db)
	var count int
	err := ex.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM extension_schedule_runs
		WHERE schedule_id = ? AND status IN ('running', 'triggering', 'retry_wait')
	`, scheduleID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite: count active schedule runs: %w", err)
	}
	return count, nil
}

func (r *ScheduleRepository) CountActiveRunsByExtension(ctx context.Context, extensionID string) (int, error) {
	ex := getExecutor(ctx, r.db)
	var count int
	err := ex.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM extension_schedule_runs r
		INNER JOIN extension_schedule_definitions d ON r.schedule_id = d.schedule_id
		WHERE d.extension_id = ? AND r.status IN ('running', 'triggering', 'retry_wait')
	`, extensionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite: count active schedule runs by extension: %w", err)
	}
	return count, nil
}

func (r *ScheduleRepository) PutMisfire(ctx context.Context, record *schedule.ScheduleMisfireRecord) error {
	if record.MisfireID == "" {
		record.MisfireID = uuid.New().String()
	}
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_schedule_misfires
			(misfire_id, schedule_id, scheduled_at, detected_at, policy, action, skipped_count, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(misfire_id) DO UPDATE SET
			schedule_id = excluded.schedule_id, scheduled_at = excluded.scheduled_at,
			detected_at = excluded.detected_at, policy = excluded.policy,
			action = excluded.action, skipped_count = excluded.skipped_count,
			detail = excluded.detail
	`,
		record.MisfireID, record.ScheduleID, record.ScheduledAt.UTC(), record.DetectedAt.UTC(),
		string(record.Policy), record.Action, record.SkippedCount, record.Detail,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert schedule misfire: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) ListMisfiresBySchedule(ctx context.Context, scheduleID string, limit int) ([]*schedule.ScheduleMisfireRecord, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT misfire_id, schedule_id, scheduled_at, detected_at, policy, action, skipped_count, detail
		FROM extension_schedule_misfires WHERE schedule_id = ?
		ORDER BY detected_at DESC LIMIT ?
	`, scheduleID, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list schedule misfires by schedule: %w", err)
	}
	defer rows.Close()
	var out []*schedule.ScheduleMisfireRecord
	for rows.Next() {
		var record schedule.ScheduleMisfireRecord
		if err := rows.Scan(
			&record.MisfireID, &record.ScheduleID, &record.ScheduledAt, &record.DetectedAt,
			&record.Policy, &record.Action, &record.SkippedCount, &record.Detail,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scan schedule misfire: %w", err)
		}
		record.ScheduledAt = record.ScheduledAt.UTC()
		record.DetectedAt = record.DetectedAt.UTC()
		out = append(out, &record)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) PutRetry(ctx context.Context, record *schedule.ScheduleRetryRecord) error {
	if record.RetryID == "" {
		record.RetryID = uuid.New().String()
	}
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_schedule_retries
			(retry_id, trigger_id, schedule_id, attempt, max_attempts, error_code, backoff_ms, available_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(retry_id) DO UPDATE SET
			trigger_id = excluded.trigger_id, schedule_id = excluded.schedule_id,
			attempt = excluded.attempt, max_attempts = excluded.max_attempts,
			error_code = excluded.error_code, backoff_ms = excluded.backoff_ms,
			available_at = excluded.available_at
	`,
		record.RetryID, record.TriggerID, record.ScheduleID, record.Attempt,
		record.MaxAttempts, record.ErrorCode, record.Backoff.Milliseconds(),
		record.AvailableAt.UTC(), createdAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert schedule retry: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) ListDueRetries(ctx context.Context, now time.Time, limit int) ([]*schedule.ScheduleRetryRecord, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT retry_id, trigger_id, schedule_id, attempt, max_attempts, error_code, backoff_ms, available_at, created_at
		FROM extension_schedule_retries WHERE available_at <= ?
		ORDER BY available_at LIMIT ?
	`, now.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list due schedule retries: %w", err)
	}
	defer rows.Close()
	var out []*schedule.ScheduleRetryRecord
	for rows.Next() {
		var record schedule.ScheduleRetryRecord
		var backoffMS int64
		if err := rows.Scan(
			&record.RetryID, &record.TriggerID, &record.ScheduleID, &record.Attempt,
			&record.MaxAttempts, &record.ErrorCode, &backoffMS,
			&record.AvailableAt, &record.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scan schedule retry: %w", err)
		}
		record.Backoff = time.Duration(backoffMS) * time.Millisecond
		record.AvailableAt = record.AvailableAt.UTC()
		record.CreatedAt = record.CreatedAt.UTC()
		out = append(out, &record)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) DeleteRetry(ctx context.Context, retryID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_schedule_retries WHERE retry_id = ?`, retryID)
	if err != nil {
		return fmt.Errorf("sqlite: delete schedule retry: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) GetCircuit(ctx context.Context, scheduleID string) (*schedule.ScheduleCircuitRecord, error) {
	ex := getExecutor(ctx, r.db)
	var record schedule.ScheduleCircuitRecord
	var lastFailCode sql.NullString
	var lastFailTime, openedAt sql.NullTime
	err := ex.QueryRowContext(ctx, `
		SELECT schedule_id, state, consecutive_fails, total_fails, total_success,
		       last_fail_code, last_fail_time, opened_at, updated_at
		FROM extension_schedule_circuits WHERE schedule_id = ?
	`, scheduleID).Scan(
		&record.ScheduleID, &record.State, &record.ConsecutiveFails,
		&record.TotalFails, &record.TotalSuccess,
		&lastFailCode, &lastFailTime, &openedAt, &record.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("sqlite: get schedule circuit: %w", err)
	}
	record.LastFailCode = stringPtr(lastFailCode)
	record.LastFailTime = timePtr(lastFailTime)
	record.OpenedAt = timePtr(openedAt)
	record.UpdatedAt = record.UpdatedAt.UTC()
	return &record, nil
}

func (r *ScheduleRepository) PutCircuit(ctx context.Context, record *schedule.ScheduleCircuitRecord) error {
	now := time.Now().UTC()
	updatedAt := record.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = now
	}
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_schedule_circuits
			(schedule_id, state, consecutive_fails, total_fails, total_success,
			 last_fail_code, last_fail_time, opened_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(schedule_id) DO UPDATE SET
			state = excluded.state, consecutive_fails = excluded.consecutive_fails,
			total_fails = excluded.total_fails, total_success = excluded.total_success,
			last_fail_code = excluded.last_fail_code, last_fail_time = excluded.last_fail_time,
			opened_at = excluded.opened_at, updated_at = excluded.updated_at
	`,
		record.ScheduleID, string(record.State), record.ConsecutiveFails,
		record.TotalFails, record.TotalSuccess,
		nullableString(record.LastFailCode), nullableTime(record.LastFailTime),
		nullableTime(record.OpenedAt), updatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert schedule circuit: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) DeleteCircuit(ctx context.Context, scheduleID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_schedule_circuits WHERE schedule_id = ?`, scheduleID)
	if err != nil {
		return fmt.Errorf("sqlite: delete schedule circuit: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) PutQuarantine(ctx context.Context, record *schedule.ScheduleQuarantineRecord) error {
	if record.QuarantineID == "" {
		record.QuarantineID = uuid.New().String()
	}
	quarantinedAt := record.QuarantinedAt
	if quarantinedAt.IsZero() {
		quarantinedAt = time.Now().UTC()
	}
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_schedule_quarantines
			(quarantine_id, schedule_id, reason, detail, quarantined_at, released_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(quarantine_id) DO UPDATE SET
			schedule_id = excluded.schedule_id, reason = excluded.reason,
			detail = excluded.detail, quarantined_at = excluded.quarantined_at,
			released_at = excluded.released_at
	`,
		record.QuarantineID, record.ScheduleID, string(record.Reason),
		record.Detail, quarantinedAt, nullableTime(record.ReleasedAt),
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert schedule quarantine: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) ListQuarantines(ctx context.Context) ([]*schedule.ScheduleQuarantineRecord, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT quarantine_id, schedule_id, reason, detail, quarantined_at, released_at
		FROM extension_schedule_quarantines ORDER BY quarantined_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list schedule quarantines: %w", err)
	}
	defer rows.Close()
	var out []*schedule.ScheduleQuarantineRecord
	for rows.Next() {
		var record schedule.ScheduleQuarantineRecord
		var releasedAt sql.NullTime
		if err := rows.Scan(
			&record.QuarantineID, &record.ScheduleID, &record.Reason,
			&record.Detail, &record.QuarantinedAt, &releasedAt,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scan schedule quarantine: %w", err)
		}
		record.QuarantinedAt = record.QuarantinedAt.UTC()
		record.ReleasedAt = timePtr(releasedAt)
		out = append(out, &record)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) DeleteAllByExtension(ctx context.Context, extensionID string) error {
	ex := getExecutor(ctx, r.db)
	tables := []struct {
		name  string
		where string
	}{
		{"extension_schedule_retries", "schedule_id IN (SELECT schedule_id FROM extension_schedule_definitions WHERE extension_id = ?)"},
		{"extension_schedule_misfires", "schedule_id IN (SELECT schedule_id FROM extension_schedule_definitions WHERE extension_id = ?)"},
		{"extension_schedule_leases", "schedule_id IN (SELECT schedule_id FROM extension_schedule_definitions WHERE extension_id = ?)"},
		{"extension_schedule_quarantines", "schedule_id IN (SELECT schedule_id FROM extension_schedule_definitions WHERE extension_id = ?)"},
		{"extension_schedule_runs", "schedule_id IN (SELECT schedule_id FROM extension_schedule_definitions WHERE extension_id = ?)"},
		{"extension_schedule_triggers", "schedule_id IN (SELECT schedule_id FROM extension_schedule_definitions WHERE extension_id = ?)"},
		{"extension_schedule_circuits", "schedule_id IN (SELECT schedule_id FROM extension_schedule_definitions WHERE extension_id = ?)"},
		{"extension_schedule_states", "schedule_id IN (SELECT schedule_id FROM extension_schedule_definitions WHERE extension_id = ?)"},
		{"extension_schedule_definitions", "extension_id = ?"},
	}
	for _, t := range tables {
		query := fmt.Sprintf("DELETE FROM %s WHERE %s", t.name, t.where)
		if _, err := ex.ExecContext(ctx, query, extensionID); err != nil {
			return fmt.Errorf("sqlite: delete from %s by extension: %w", t.name, err)
		}
	}
	return nil
}

func scanScheduleDefinition(s scanner) (*schedule.ScheduleContributionDefinition, error) {
	var def schedule.ScheduleContributionDefinition
	var triggerType, targetType string
	var triggerJSON, targetJSON, misfireJSON, overlapJSON, retryJSON, jitterJSON, concurrencyJSON string
	var permReqJSON, scopeRuleJSON, depReqJSON sql.NullString
	var startAt, endAt sql.NullTime
	var createdAt, updatedAt time.Time

	err := s.Scan(
		&def.ScheduleID, &def.ContributionID, &def.ExtensionID, &def.ModuleID,
		&def.Name, &def.Description,
		&triggerType, &triggerJSON, &targetType, &targetJSON, &def.Timezone,
		&startAt, &endAt, &misfireJSON, &overlapJSON,
		&retryJSON, &jitterJSON, &concurrencyJSON,
		&permReqJSON, &scopeRuleJSON, &depReqJSON,
		&def.DSTSpringPolicy, &def.DSTFallPolicy,
		&def.DefinitionHash, &def.Version,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: scan schedule definition: %w", err)
	}

	if err := json.Unmarshal([]byte(triggerJSON), &def.Trigger); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal schedule trigger: %w", err)
	}
	if err := json.Unmarshal([]byte(targetJSON), &def.Target); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal schedule target: %w", err)
	}
	if err := json.Unmarshal([]byte(misfireJSON), &def.MisfirePolicy); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal schedule misfire policy: %w", err)
	}
	if err := json.Unmarshal([]byte(overlapJSON), &def.OverlapPolicy); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal schedule overlap policy: %w", err)
	}
	if err := json.Unmarshal([]byte(retryJSON), &def.RetryPolicy); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal schedule retry policy: %w", err)
	}
	if err := json.Unmarshal([]byte(jitterJSON), &def.JitterPolicy); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal schedule jitter policy: %w", err)
	}
	if err := json.Unmarshal([]byte(concurrencyJSON), &def.ConcurrencyPolicy); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal schedule concurrency policy: %w", err)
	}
	if permReqJSON.Valid {
		if err := json.Unmarshal([]byte(permReqJSON.String), &def.PermissionRequirements); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal schedule permission requirements: %w", err)
		}
	}
	if scopeRuleJSON.Valid {
		if err := json.Unmarshal([]byte(scopeRuleJSON.String), &def.ScopeRule); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal schedule scope rule: %w", err)
		}
	}
	if depReqJSON.Valid {
		if err := json.Unmarshal([]byte(depReqJSON.String), &def.DependencyRequirements); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal schedule dependency requirements: %w", err)
		}
	}
	def.StartAt = timePtr(startAt)
	def.EndAt = timePtr(endAt)
	return &def, nil
}

func scanScheduleState(s scanner) (*schedule.ScheduleState, error) {
	var state schedule.ScheduleState
	var enabled, paused int
	var status string
	var lastScheduledAt, lastTriggeredAt, lastFinishedAt, nextScheduledAt, nextEffectiveAt sql.NullTime
	var lastResult sql.NullString

	err := s.Scan(
		&state.ScheduleID, &enabled, &paused, &status,
		&lastScheduledAt, &lastTriggeredAt, &lastFinishedAt,
		&nextScheduledAt, &nextEffectiveAt, &lastResult,
		&state.FailureCount, &state.Generation, &state.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: scan schedule state: %w", err)
	}
	state.Enabled = enabled != 0
	state.Paused = paused != 0
	state.Status = schedule.ScheduleDefinitionStatus(status)
	state.LastScheduledAt = timePtr(lastScheduledAt)
	state.LastTriggeredAt = timePtr(lastTriggeredAt)
	state.LastFinishedAt = timePtr(lastFinishedAt)
	state.NextScheduledAt = timePtr(nextScheduledAt)
	state.NextEffectiveAt = timePtr(nextEffectiveAt)
	if lastResult.Valid {
		state.LastResult = lastResult.String
	}
	state.UpdatedAt = state.UpdatedAt.UTC()
	return &state, nil
}

func scanScheduleTrigger(s scanner) (*schedule.ScheduleTriggerRecord, error) {
	var record schedule.ScheduleTriggerRecord
	var status string
	var triggeredAt, leaseExpiresAt sql.NullTime
	var leaseOwner, operationID, invocationID, errorCode, errorMessage sql.NullString
	var jitterAppliedMS int64
	var manual int

	err := s.Scan(
		&record.TriggerID, &record.ScheduleID, &record.ScheduledAt, &record.EffectiveAt,
		&triggeredAt, &record.IdempotencyKey, &status,
		&leaseOwner, &leaseExpiresAt,
		&record.ScopeSnapshotID, &record.PermissionSnapshotID, &record.DependencySnapshotID,
		&operationID, &invocationID, &record.Attempt, &record.Generation, &manual,
		&errorCode, &errorMessage, &jitterAppliedMS,
		&record.MisfireDecision, &record.OverlapDecision, &record.DSTDecision,
		&record.CreatedAt, &record.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: scan schedule trigger: %w", err)
	}
	record.Status = schedule.ScheduleRunStatus(status)
	record.TriggeredAt = timePtr(triggeredAt)
	record.LeaseOwner = stringPtr(leaseOwner)
	record.LeaseExpiresAt = timePtr(leaseExpiresAt)
	record.OperationID = stringPtr(operationID)
	record.InvocationID = stringPtr(invocationID)
	record.Manual = manual != 0
	record.ErrorCode = stringPtr(errorCode)
	record.ErrorMessage = stringPtr(errorMessage)
	record.JitterApplied = time.Duration(jitterAppliedMS) * time.Millisecond
	record.ScheduledAt = record.ScheduledAt.UTC()
	record.EffectiveAt = record.EffectiveAt.UTC()
	record.CreatedAt = record.CreatedAt.UTC()
	record.UpdatedAt = record.UpdatedAt.UTC()
	return &record, nil
}

func scanScheduleRun(s scanner) (*schedule.ScheduleRunRecord, error) {
	var run schedule.ScheduleRunRecord
	var status, targetType string
	var finishedAt sql.NullTime
	var resultJSON sql.NullString
	var errorCode, errorMessage sql.NullString

	err := s.Scan(
		&run.RunID, &run.TriggerID, &run.ScheduleID, &status, &run.Attempt,
		&run.StartedAt, &finishedAt,
		&run.OperationID, &run.InvocationID,
		&targetType, &run.TargetID, &resultJSON,
		&errorCode, &errorMessage, &run.Generation,
		&run.CreatedAt, &run.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: scan schedule run: %w", err)
	}
	run.Status = schedule.ScheduleRunStatus(status)
	run.FinishedAt = timePtr(finishedAt)
	run.TargetType = schedule.TargetType(targetType)
	if resultJSON.Valid {
		run.ResultJSON = json.RawMessage(resultJSON.String)
	}
	run.ErrorCode = stringPtr(errorCode)
	run.ErrorMessage = stringPtr(errorMessage)
	run.StartedAt = run.StartedAt.UTC()
	run.CreatedAt = run.CreatedAt.UTC()
	run.UpdatedAt = run.UpdatedAt.UTC()
	return &run, nil
}

func toSQLValue(v any) any {
	switch val := v.(type) {
	case nil:
		return nil
	case *string:
		if val == nil {
			return nil
		}
		return *val
	case *time.Time:
		if val == nil {
			return nil
		}
		return val.UTC()
	case time.Time:
		return val.UTC()
	case *int:
		if val == nil {
			return nil
		}
		return *val
	case *int64:
		if val == nil {
			return nil
		}
		return *val
	case schedule.ScheduleRunStatus:
		return string(val)
	case json.RawMessage:
		if len(val) == 0 {
			return nil
		}
		return string(val)
	default:
		return v
	}
}
