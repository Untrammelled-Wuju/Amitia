package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type WorkflowDefinitionRepository struct {
	db *sql.DB
}

func NewWorkflowDefinitionRepository(db *sql.DB) *WorkflowDefinitionRepository {
	return &WorkflowDefinitionRepository{db: db}
}

func (r *WorkflowDefinitionRepository) Save(ctx context.Context, def workflow.WorkflowDefinition) error {
	nodesJSON, err := json.Marshal(def.Nodes)
	if err != nil {
		return fmt.Errorf("marshal nodes: %w", err)
	}

	permissionsJSON, err := json.Marshal(def.Permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}

	limitsJSON, err := json.Marshal(def.Limits)
	if err != nil {
		return fmt.Errorf("marshal limits: %w", err)
	}

	metadataJSON := []byte("{}")
	if def.Metadata != nil {
		if b, err := json.Marshal(def.Metadata); err == nil {
			metadataJSON = b
		}
	}

	inputSchema := def.InputSchema
	if len(inputSchema) == 0 {
		inputSchema = json.RawMessage("{}")
	}
	outputSchema := def.OutputSchema
	if len(outputSchema) == 0 {
		outputSchema = json.RawMessage("{}")
	}

	defHash := workflow.ComputeDefinitionHash(def)

	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_definitions
			(workflow_id, extension_id, module_id, name, description, schema_version, version,
			 input_schema_json, output_schema_json, nodes_json, permissions_json, scope,
			 callable_by_agent, enabled, has_side_effects, idempotent, limits_json,
			 source, metadata_json, definition_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_id) DO UPDATE SET
			extension_id = excluded.extension_id,
			module_id = excluded.module_id,
			name = excluded.name,
			description = excluded.description,
			schema_version = excluded.schema_version,
			version = excluded.version,
			input_schema_json = excluded.input_schema_json,
			output_schema_json = excluded.output_schema_json,
			nodes_json = excluded.nodes_json,
			permissions_json = excluded.permissions_json,
			scope = excluded.scope,
			callable_by_agent = excluded.callable_by_agent,
			enabled = excluded.enabled,
			has_side_effects = excluded.has_side_effects,
			idempotent = excluded.idempotent,
			limits_json = excluded.limits_json,
			source = excluded.source,
			metadata_json = excluded.metadata_json,
			definition_hash = excluded.definition_hash,
			updated_at = excluded.updated_at
	`, def.ID, def.ExtensionID, def.ModuleID, def.Name, def.Description, def.SchemaVersion, def.Version,
		inputSchema, outputSchema, nodesJSON, permissionsJSON, def.Scope,
		boolToInt(def.CallableByAgent), boolToInt(def.Enabled), boolToInt(def.HasSideEffects), boolToInt(def.Idempotent),
		limitsJSON, def.Source, metadataJSON, defHash, now, now)
	if err != nil {
		return fmt.Errorf("save workflow definition: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) Delete(ctx context.Context, workflowID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM extension_workflow_definitions WHERE workflow_id = ?`, workflowID)
	if err != nil {
		return fmt.Errorf("delete workflow definition: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) Get(ctx context.Context, workflowID string) (*workflow.WorkflowDefinition, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT workflow_id, extension_id, module_id, name, description, schema_version, version,
			input_schema_json, output_schema_json, nodes_json, permissions_json, scope,
			callable_by_agent, enabled, has_side_effects, idempotent, limits_json,
			source, metadata_json, definition_hash
		FROM extension_workflow_definitions WHERE workflow_id = ?
	`, workflowID)

	def, err := scanWorkflowDefinition(row)
	if err != nil {
		return nil, err
	}
	return def, nil
}

func (r *WorkflowDefinitionRepository) List(ctx context.Context) ([]workflow.WorkflowDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT workflow_id, extension_id, module_id, name, description, schema_version, version,
			input_schema_json, output_schema_json, nodes_json, permissions_json, scope,
			callable_by_agent, enabled, has_side_effects, idempotent, limits_json,
			source, metadata_json, definition_hash
		FROM extension_workflow_definitions
	`)
	if err != nil {
		return nil, fmt.Errorf("list workflow definitions: %w", err)
	}
	defer rows.Close()

	var defs []workflow.WorkflowDefinition
	for rows.Next() {
		def, err := scanWorkflowDefinition(rows)
		if err != nil {
			return nil, err
		}
		defs = append(defs, *def)
	}
	return defs, rows.Err()
}

func (r *WorkflowDefinitionRepository) SetEnabled(ctx context.Context, workflowID string, enabled bool) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE extension_workflow_definitions SET enabled = ?, updated_at = ? WHERE workflow_id = ?
	`, boolToInt(enabled), time.Now().UTC(), workflowID)
	if err != nil {
		return fmt.Errorf("set workflow enabled: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) SaveTrigger(ctx context.Context, binding workflow.TriggerBinding) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_trigger_bindings
			(binding_id, trigger_type, event_type, schedule_id, workflow_id, input_json, generation, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(binding_id) DO UPDATE SET
			trigger_type = excluded.trigger_type, event_type = excluded.event_type,
			schedule_id = excluded.schedule_id, workflow_id = excluded.workflow_id,
			input_json = excluded.input_json, generation = excluded.generation,
			enabled = excluded.enabled, updated_at = excluded.updated_at
	`, binding.BindingID, binding.Type, binding.EventType, binding.ScheduleID, binding.WorkflowID, binding.Input, binding.Generation, boolToInt(binding.Enabled), now, now)
	if err != nil {
		return fmt.Errorf("save workflow trigger binding: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) ListTriggers(ctx context.Context, triggerType workflow.TriggerType, eventType, scheduleID string) ([]workflow.TriggerBinding, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT binding_id, trigger_type, event_type, schedule_id, workflow_id, input_json, generation, enabled
		FROM extension_workflow_trigger_bindings
		WHERE trigger_type = ? AND enabled = 1
			AND (? = '' OR event_type = ?)
			AND (? = '' OR schedule_id = ?)
		ORDER BY binding_id
	`, triggerType, eventType, eventType, scheduleID, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("list workflow trigger bindings: %w", err)
	}
	defer rows.Close()
	result := make([]workflow.TriggerBinding, 0)
	for rows.Next() {
		var binding workflow.TriggerBinding
		var inputJSON string
		var enabled int
		if err := rows.Scan(&binding.BindingID, &binding.Type, &binding.EventType, &binding.ScheduleID, &binding.WorkflowID, &inputJSON, &binding.Generation, &enabled); err != nil {
			return nil, fmt.Errorf("scan workflow trigger binding: %w", err)
		}
		binding.Input = json.RawMessage(inputJSON)
		binding.Enabled = enabled == 1
		result = append(result, binding)
	}
	return result, rows.Err()
}

func (r *WorkflowDefinitionRepository) DeleteTrigger(ctx context.Context, bindingID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM extension_workflow_trigger_bindings WHERE binding_id = ?`, bindingID)
	if err != nil {
		return fmt.Errorf("delete workflow trigger binding: %w", err)
	}
	return nil
}

type scannerInterface interface {
	Scan(dest ...any) error
}

func scanWorkflowDefinition(row scannerInterface) (*workflow.WorkflowDefinition, error) {
	var def workflow.WorkflowDefinition
	var inputSchema, outputSchema, nodesJSON, permissionsJSON, limitsJSON, metadataJSON string
	var callableByAgent, enabled, hasSideEffects, idempotent int

	err := row.Scan(
		&def.ID, &def.ExtensionID, &def.ModuleID, &def.Name, &def.Description,
		&def.SchemaVersion, &def.Version,
		&inputSchema, &outputSchema, &nodesJSON, &permissionsJSON, &def.Scope,
		&callableByAgent, &enabled, &hasSideEffects, &idempotent, &limitsJSON,
		&def.Source, &metadataJSON, &def.DefinitionHash,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workflow definition not found")
		}
		return nil, fmt.Errorf("scan workflow definition: %w", err)
	}

	def.InputSchema = json.RawMessage(inputSchema)
	def.OutputSchema = json.RawMessage(outputSchema)
	def.CallableByAgent = callableByAgent == 1
	def.Enabled = enabled == 1
	def.HasSideEffects = hasSideEffects == 1
	def.Idempotent = idempotent == 1

	if err := json.Unmarshal([]byte(nodesJSON), &def.Nodes); err != nil {
		return nil, fmt.Errorf("unmarshal nodes: %w", err)
	}
	if err := json.Unmarshal([]byte(permissionsJSON), &def.Permissions); err != nil {
		return nil, fmt.Errorf("unmarshal permissions: %w", err)
	}
	if err := json.Unmarshal([]byte(limitsJSON), &def.Limits); err != nil {
		return nil, fmt.Errorf("unmarshal limits: %w", err)
	}
	if metadataJSON != "" && metadataJSON != "{}" {
		if err := json.Unmarshal([]byte(metadataJSON), &def.Metadata); err == nil {
		}
	}

	return &def, nil
}

type SQLiteCheckpointStore struct {
	db *sql.DB
}

func NewSQLiteCheckpointStore(db *sql.DB) *SQLiteCheckpointStore {
	return &SQLiteCheckpointStore{db: db}
}

func (s *SQLiteCheckpointStore) Save(ctx context.Context, cp workflow.Checkpoint) error {
	checkpointID := fmt.Sprintf("%s-%s", cp.ExecutionID, cp.NodeID)
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_checkpoints
			(checkpoint_id, workflow_id, execution_id, node_id, input_json, output_json, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(execution_id, node_id) DO UPDATE SET
			output_json = excluded.output_json,
			completed_at = excluded.completed_at
	`, checkpointID, cp.WorkflowID, cp.ExecutionID, cp.NodeID, cp.Input, cp.Output, cp.CompletedAt, now)
	if err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}
	return nil
}

func (s *SQLiteCheckpointStore) Load(ctx context.Context, executionID, nodeID string) (*workflow.Checkpoint, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT workflow_id, execution_id, node_id, input_json, output_json, completed_at
		FROM extension_workflow_checkpoints
		WHERE execution_id = ? AND node_id = ?
	`, executionID, nodeID)

	var cp workflow.Checkpoint
	var inputJSON, outputJSON string
	err := row.Scan(&cp.WorkflowID, &cp.ExecutionID, &cp.NodeID, &inputJSON, &outputJSON, &cp.CompletedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, workflow.ErrCheckpointNotFound
		}
		return nil, fmt.Errorf("load checkpoint: %w", err)
	}
	cp.Input = json.RawMessage(inputJSON)
	cp.Output = json.RawMessage(outputJSON)
	return &cp, nil
}

func (s *SQLiteCheckpointStore) List(ctx context.Context, executionID string) ([]workflow.Checkpoint, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT workflow_id, execution_id, node_id, input_json, output_json, completed_at
		FROM extension_workflow_checkpoints
		WHERE execution_id = ?
	`, executionID)
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}
	defer rows.Close()

	var checkpoints []workflow.Checkpoint
	for rows.Next() {
		var cp workflow.Checkpoint
		var inputJSON, outputJSON string
		if err := rows.Scan(&cp.WorkflowID, &cp.ExecutionID, &cp.NodeID, &inputJSON, &outputJSON, &cp.CompletedAt); err != nil {
			return nil, err
		}
		cp.Input = json.RawMessage(inputJSON)
		cp.Output = json.RawMessage(outputJSON)
		checkpoints = append(checkpoints, cp)
	}
	return checkpoints, rows.Err()
}

func (s *SQLiteCheckpointStore) Delete(ctx context.Context, executionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM extension_workflow_checkpoints WHERE execution_id = ?`, executionID)
	if err != nil {
		return fmt.Errorf("delete checkpoints: %w", err)
	}
	return nil
}

type WorkflowExecutionRepository struct {
	db *sql.DB
}

func NewWorkflowExecutionRepository(db *sql.DB) *WorkflowExecutionRepository {
	return &WorkflowExecutionRepository{db: db}
}

func (r *WorkflowExecutionRepository) Save(ctx context.Context, execID, workflowID string, result *workflow.ExecuteResult, extID, charID, convID, opID string) error {
	stepsJSON, _ := json.Marshal(result.Steps)
	compJSON := []byte("[]")
	if result.CompensationResults != nil {
		compJSON, _ = json.Marshal(result.CompensationResults)
	}

	status := "failed"
	if result.Success {
		status = "succeeded"
	}

	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_executions
			(execution_id, workflow_id, status, output_json, error_message,
			 extension_id, character_id, conversation_id, operation_id,
			 steps_json, compensation_json, started_at, finished_at, duration_ms,
			 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(execution_id) DO UPDATE SET
			status = excluded.status,
			output_json = excluded.output_json,
			error_message = excluded.error_message,
			steps_json = excluded.steps_json,
			compensation_json = excluded.compensation_json,
			finished_at = excluded.finished_at,
			duration_ms = excluded.duration_ms,
			updated_at = excluded.updated_at
	`, execID, workflowID, status, result.Output, result.Error,
		extID, charID, convID, opID,
		stepsJSON, compJSON, now, now, result.Duration.Milliseconds(),
		now, now)
	if err != nil {
		return fmt.Errorf("save workflow execution: %w", err)
	}
	return nil
}

func (r *WorkflowExecutionRepository) Start(ctx context.Context, run workflow.WorkflowRun) (*workflow.WorkflowRun, bool, error) {
	existing, err := r.Get(ctx, run.ExecutionID)
	if err == nil {
		if existing.Status == workflow.RunStatusFailed || existing.Status == workflow.RunStatusCancelled || run.Context.Recovery {
			_, updateErr := r.db.ExecContext(ctx, `UPDATE extension_workflow_executions SET status = ?, error_message = '', finished_at = NULL, attempt = attempt + 1, generation = ?, updated_at = ? WHERE execution_id = ?`, workflow.RunStatusRunning, run.Context.Generation, time.Now().UTC(), run.ExecutionID)
			if updateErr != nil {
				return nil, false, fmt.Errorf("restart workflow execution: %w", updateErr)
			}
		}
		return existing, false, nil
	}
	if err != workflow.ErrWorkflowRunNotFound {
		return nil, false, err
	}

	contextJSON, marshalErr := json.Marshal(run.Context)
	if marshalErr != nil {
		return nil, false, fmt.Errorf("marshal workflow context: %w", marshalErr)
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_executions
			(execution_id, workflow_id, status, input_json, output_json, error_message,
			 extension_id, module_id, character_id, conversation_id, operation_id, invocation_id,
			 schedule_id, trigger_id, trace_id, idempotency_key, scope_snapshot_id,
			 permission_snapshot_id, generation, context_json, attempt, steps_json,
			 compensation_json, pause_reason, pause_requested_at, paused_at,
			 started_at, duration_ms, created_at, updated_at)
		VALUES (?, ?, ?, ?, NULL, '', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '[]', '[]', ?, ?, NULL, ?, 0, ?, ?)
`, run.ExecutionID, run.WorkflowID, workflow.RunStatusRunning, run.Input,
		run.Context.ExtensionID, run.Context.ModuleID, run.Context.CharacterID, run.Context.ConversationID,
		run.Context.OperationID, run.Context.InvocationID, run.Context.ScheduleID, run.Context.TriggerID,
		run.Context.TraceID, run.Context.IdempotencyKey, run.Context.ScopeSnapshotID,
		run.Context.PermissionSnapID, run.Context.Generation, contextJSON, maxInt(run.Attempt, 1),
		run.PauseReason, run.PauseRequestedAt,
		run.StartedAt, now, now)
	if err != nil {
		if run.Context.IdempotencyKey != "" {
			duplicate, queryErr := r.getByIdempotency(ctx, run.WorkflowID, run.Context.IdempotencyKey)
			if queryErr == nil {
				return duplicate, false, nil
			}
		}
		return nil, false, fmt.Errorf("start workflow execution: %w", err)
	}
	return &run, true, nil
}

func (r *WorkflowExecutionRepository) SaveStep(ctx context.Context, step workflow.StepRun) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_step_runs
			(execution_id, workflow_id, node_id, status, input_json, output_json, error_message, attempt, started_at, finished_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(execution_id, node_id) DO UPDATE SET
			status = excluded.status, input_json = excluded.input_json, output_json = excluded.output_json,
			error_message = excluded.error_message, attempt = excluded.attempt,
			started_at = excluded.started_at, finished_at = excluded.finished_at, updated_at = excluded.updated_at
	`, step.ExecutionID, step.WorkflowID, step.NodeID, step.Status, step.Input, step.Output, step.Error, step.Attempt, step.StartedAt, step.FinishedAt, now)
	if err != nil {
		return fmt.Errorf("save workflow step run: %w", err)
	}
	return nil
}

func (r *WorkflowExecutionRepository) Finish(ctx context.Context, run workflow.WorkflowRun) error {
	stepsJSON, err := json.Marshal(run.Steps)
	if err != nil {
		return fmt.Errorf("marshal workflow steps: %w", err)
	}
	compJSON, err := json.Marshal(run.CompensationResults)
	if err != nil {
		return fmt.Errorf("marshal workflow compensation: %w", err)
	}
	duration := int64(0)
	if run.FinishedAt != nil {
		duration = run.FinishedAt.Sub(run.StartedAt).Milliseconds()
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE extension_workflow_executions
		SET status = ?, output_json = ?, error_message = ?, steps_json = ?, compensation_json = ?,
			finished_at = ?, duration_ms = ?, updated_at = ?
		WHERE execution_id = ?
	`, run.Status, run.Output, run.Error, stepsJSON, compJSON, run.FinishedAt, duration, run.UpdatedAt, run.ExecutionID)
	if err != nil {
		return fmt.Errorf("finish workflow execution: %w", err)
	}
	for _, compensation := range run.CompensationResults {
		executedAt := compensation.ExecutedAt
		if executedAt.IsZero() {
			executedAt = run.UpdatedAt
		}
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO extension_workflow_compensations (execution_id, node_id, status, error_message, executed_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(execution_id, node_id) DO UPDATE SET status = excluded.status, error_message = excluded.error_message, executed_at = excluded.executed_at
		`, run.ExecutionID, compensation.NodeID, compensation.Status, compensation.Error, executedAt); err != nil {
			return fmt.Errorf("save workflow compensation: %w", err)
		}
	}
	return nil
}

func (r *WorkflowExecutionRepository) UpdateStateCAS(ctx context.Context, run workflow.WorkflowRun, expectedStatus workflow.RunStatus) (bool, error) {
	var pauseRequestedAt, pausedAt interface{}
	if run.PauseRequestedAt != nil {
		pauseRequestedAt = *run.PauseRequestedAt
	}
	if run.PausedAt != nil {
		pausedAt = *run.PausedAt
	}

	res, err := r.db.ExecContext(ctx, `
		UPDATE extension_workflow_executions
		SET status = ?, generation = ?, pause_reason = ?, pause_requested_at = ?, paused_at = ?, updated_at = ?
		WHERE execution_id = ? AND status = ?
	`, run.Status, run.Generation, run.PauseReason, pauseRequestedAt, pausedAt, run.UpdatedAt, run.ExecutionID, expectedStatus)
	if err != nil {
		return false, fmt.Errorf("update workflow state cas: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *WorkflowExecutionRepository) Get(ctx context.Context, executionID string) (*workflow.WorkflowRun, error) {
	return r.scanRun(r.db.QueryRowContext(ctx, `
		SELECT execution_id, workflow_id, status, input_json, output_json, error_message,
			context_json, steps_json, compensation_json, attempt, generation,
			pause_reason, pause_requested_at, paused_at,
			started_at, finished_at, updated_at
		FROM extension_workflow_executions WHERE execution_id = ?
	`, executionID))
}

func (r *WorkflowExecutionRepository) getByIdempotency(ctx context.Context, workflowID, idempotencyKey string) (*workflow.WorkflowRun, error) {
	return r.scanRun(r.db.QueryRowContext(ctx, `
		SELECT execution_id, workflow_id, status, input_json, output_json, error_message,
			context_json, steps_json, compensation_json, attempt, generation,
			pause_reason, pause_requested_at, paused_at,
			started_at, finished_at, updated_at
		FROM extension_workflow_executions WHERE workflow_id = ? AND idempotency_key = ?
	`, workflowID, idempotencyKey))
}

func (r *WorkflowExecutionRepository) ListRecoverable(ctx context.Context, limit int) ([]workflow.WorkflowRun, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT execution_id, workflow_id, status, input_json, output_json, error_message,
			context_json, steps_json, compensation_json, attempt, generation,
			pause_reason, pause_requested_at, paused_at,
			started_at, finished_at, updated_at
		FROM extension_workflow_executions
		WHERE status IN (?, ?) AND status != ? AND status != ? AND status != ?
		ORDER BY updated_at LIMIT ?
	`, workflow.RunStatusRunning, workflow.RunStatusCompensating, workflow.RunStatusPaused, workflow.RunStatusPausing, workflow.RunStatusResuming, limit)
	if err != nil {
		return nil, fmt.Errorf("list recoverable workflow executions: %w", err)
	}
	defer rows.Close()
	result := make([]workflow.WorkflowRun, 0)
	for rows.Next() {
		run, scanErr := r.scanRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *run)
	}
	return result, rows.Err()
}

func (r *WorkflowExecutionRepository) scanRun(row scannerInterface) (*workflow.WorkflowRun, error) {
	var run workflow.WorkflowRun
	var inputJSON, outputJSON, contextJSON, stepsJSON, compensationJSON sql.NullString
	var errorMessage string
	var finishedAt, pauseRequestedAt, pausedAt sql.NullTime
	var pauseReason sql.NullString
	err := row.Scan(&run.ExecutionID, &run.WorkflowID, &run.Status, &inputJSON, &outputJSON, &errorMessage,
		&contextJSON, &stepsJSON, &compensationJSON, &run.Attempt, &run.Generation,
		&pauseReason, &pauseRequestedAt, &pausedAt,
		&run.StartedAt, &finishedAt, &run.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, workflow.ErrWorkflowRunNotFound
		}
		return nil, fmt.Errorf("scan workflow execution: %w", err)
	}
	run.Input = json.RawMessage(inputJSON.String)
	run.Output = json.RawMessage(outputJSON.String)
	run.Error = errorMessage
	if finishedAt.Valid {
		run.FinishedAt = &finishedAt.Time
	}
	if pauseReason.Valid {
		run.PauseReason = pauseReason.String
	}
	if pauseRequestedAt.Valid {
		t := pauseRequestedAt.Time
		run.PauseRequestedAt = &t
	}
	if pausedAt.Valid {
		t := pausedAt.Time
		run.PausedAt = &t
	}
	if contextJSON.Valid && contextJSON.String != "" {
		if err := json.Unmarshal([]byte(contextJSON.String), &run.Context); err != nil {
			return nil, fmt.Errorf("unmarshal workflow context: %w", err)
		}
	}
	if stepsJSON.Valid && stepsJSON.String != "" {
		if err := json.Unmarshal([]byte(stepsJSON.String), &run.Steps); err != nil {
			return nil, fmt.Errorf("unmarshal workflow steps: %w", err)
		}
	}
	if compensationJSON.Valid && compensationJSON.String != "" {
		if err := json.Unmarshal([]byte(compensationJSON.String), &run.CompensationResults); err != nil {
			return nil, fmt.Errorf("unmarshal workflow compensation: %w", err)
		}
	}
	return &run, nil
}

func maxInt(value, fallback int) int {
	if value > fallback {
		return value
	}
	return fallback
}
