package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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

	defHash := computeWorkflowHash(def)

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
			source, metadata_json
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
			source, metadata_json
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
		&def.Source, &metadataJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workflow definition not found")
		}
		return nil, fmt.Errorf("scan workflow definition: %w", err)
	}

	def.InputSchema = json.RawMessage(inputSchema)
	def.OutputSchema = json.RawMessage(outputSchema)
	def.CallableByAgent = enabled == 1
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

func computeWorkflowHash(def workflow.WorkflowDefinition) string {
	payload := map[string]any{
		"id":   def.ID,
		"name": def.Name,
		"version": def.Version,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:16])
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
