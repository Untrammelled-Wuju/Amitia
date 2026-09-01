package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type WorkflowDefinitionRepository struct {
	db *sql.DB

	receiptCleanupMu   sync.Mutex
	lastReceiptCleanup time.Time
}

const (
	triggerReceiptRetention       = 30 * 24 * time.Hour
	triggerReceiptCleanupInterval = time.Hour
)

func NewWorkflowDefinitionRepository(db *sql.DB) *WorkflowDefinitionRepository {
	return &WorkflowDefinitionRepository{db: db}
}

func (r *WorkflowDefinitionRepository) Save(ctx context.Context, def workflow.WorkflowDefinition) error {
	nodesJSON, err := json.Marshal(def.Nodes)
	if err != nil {
		return fmt.Errorf("marshal nodes: %w", err)
	}
	edgesJSON, err := json.Marshal(def.Edges)
	if err != nil {
		return fmt.Errorf("marshal edges: %w", err)
	}
	triggersJSON, err := json.Marshal(def.Triggers)
	if err != nil {
		return fmt.Errorf("marshal triggers: %w", err)
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
			 input_schema_json, output_schema_json, nodes_json, edges_json, triggers_json, permissions_json, scope,
			 callable_by_agent, enabled, has_side_effects, idempotent, limits_json,
			 source, metadata_json, definition_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			edges_json = excluded.edges_json,
			triggers_json = excluded.triggers_json,
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
		inputSchema, outputSchema, nodesJSON, edgesJSON, triggersJSON, permissionsJSON, def.Scope,
		boolToInt(def.CallableByAgent), boolToInt(def.Enabled), boolToInt(def.HasSideEffects), boolToInt(def.Idempotent),
		limitsJSON, def.Source, metadataJSON, defHash, now, now)
	if err != nil {
		return fmt.Errorf("save workflow definition: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) Delete(ctx context.Context, workflowID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workflow definition delete: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var source string
	sourceErr := tx.QueryRowContext(ctx, `SELECT source FROM extension_workflow_definitions WHERE workflow_id = ?`, workflowID).Scan(&source)
	if sourceErr != nil && sourceErr != sql.ErrNoRows {
		return fmt.Errorf("read workflow source before delete: %w", sourceErr)
	}

	statements := []string{`DELETE FROM extension_workflow_trigger_bindings WHERE workflow_id = ?`}
	if source == "user" {
		statements = append(statements,
			`DELETE FROM extension_workflow_installations WHERE workflow_id = ?`,
			`DELETE FROM extension_workflow_compensations WHERE execution_id IN (SELECT execution_id FROM extension_workflow_executions WHERE workflow_id = ?)`,
			`DELETE FROM extension_workflow_step_attempts WHERE workflow_id = ?`,
			`DELETE FROM extension_workflow_step_runs WHERE workflow_id = ?`,
			`DELETE FROM extension_workflow_checkpoints WHERE workflow_id = ?`,
			`DELETE FROM extension_workflow_executions WHERE workflow_id = ?`,
			`DELETE FROM extension_workflow_revisions WHERE workflow_id = ?`,
		)
	}
	statements = append(statements, `DELETE FROM extension_workflow_definitions WHERE workflow_id = ?`)
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement, workflowID); err != nil {
			return fmt.Errorf("delete workflow data: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workflow definition delete: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) Get(ctx context.Context, workflowID string) (*workflow.WorkflowDefinition, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT workflow_id, extension_id, module_id, name, description, schema_version, version,
			input_schema_json, output_schema_json, nodes_json, edges_json, triggers_json, permissions_json, scope,
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
			input_schema_json, output_schema_json, nodes_json, edges_json, triggers_json, permissions_json, scope,
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

func (r *WorkflowDefinitionRepository) SaveRevision(ctx context.Context, ownerUserID string, def workflow.WorkflowDefinition, note string) (*workflow.WorkflowRevision, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" || strings.TrimSpace(def.ID) == "" {
		return nil, fmt.Errorf("workflow revision requires owner and workflow id")
	}
	definitionJSON, err := json.Marshal(def)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow revision: %w", err)
	}
	hash := workflow.ComputeDefinitionHash(def)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin workflow revision transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var latest workflow.WorkflowRevision
	var latestHash string
	latestErr := tx.QueryRowContext(ctx, `
		SELECT revision_id, revision_no, name, description, definition_hash, note, created_at
		FROM extension_workflow_revisions
		WHERE owner_user_id = ? AND workflow_id = ?
		ORDER BY revision_no DESC LIMIT 1
	`, ownerUserID, def.ID).Scan(&latest.RevisionID, &latest.RevisionNo, &latest.Name, &latest.Description, &latestHash, &latest.Note, &latest.CreatedAt)
	if latestErr != nil && latestErr != sql.ErrNoRows {
		return nil, fmt.Errorf("get latest workflow revision: %w", latestErr)
	}
	if latestErr == nil && latestHash == hash {
		latest.WorkflowID = def.ID
		latest.OwnerUserID = ownerUserID
		latest.Definition = def
		latest.DefinitionHash = latestHash
		return &latest, nil
	}
	var revisionNo int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision_no), 0) + 1 FROM extension_workflow_revisions WHERE workflow_id = ?`, def.ID).Scan(&revisionNo); err != nil {
		return nil, fmt.Errorf("next workflow revision: %w", err)
	}
	now := time.Now().UTC()
	revision := &workflow.WorkflowRevision{
		RevisionID:     "wfrev-" + uuid.NewString(),
		WorkflowID:     def.ID,
		OwnerUserID:    ownerUserID,
		RevisionNo:     revisionNo,
		Name:           def.Name,
		Description:    def.Description,
		Definition:     def,
		DefinitionHash: hash,
		Note:           strings.TrimSpace(note),
		CreatedAt:      now,
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO extension_workflow_revisions
			(revision_id, workflow_id, owner_user_id, revision_no, name, description, definition_json, definition_hash, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, revision.RevisionID, revision.WorkflowID, revision.OwnerUserID, revision.RevisionNo, revision.Name, revision.Description, definitionJSON, revision.DefinitionHash, revision.Note, revision.CreatedAt); err != nil {
		return nil, fmt.Errorf("save workflow revision: %w", err)
	}
	// Keep history useful without allowing unbounded local growth.
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM extension_workflow_revisions
		WHERE workflow_id = ? AND revision_id NOT IN (
			SELECT revision_id FROM extension_workflow_revisions WHERE workflow_id = ? ORDER BY revision_no DESC LIMIT 50
		)
	`, def.ID, def.ID); err != nil {
		return nil, fmt.Errorf("prune workflow revisions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit workflow revision: %w", err)
	}
	return revision, nil
}

func (r *WorkflowDefinitionRepository) ListRevisions(ctx context.Context, ownerUserID, workflowID string, limit int) ([]workflow.WorkflowRevisionSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT revision_id, workflow_id, revision_no, name, description, definition_hash, note, created_at
		FROM extension_workflow_revisions
		WHERE owner_user_id = ? AND workflow_id = ?
		ORDER BY revision_no DESC LIMIT ?
	`, ownerUserID, workflowID, limit)
	if err != nil {
		return nil, fmt.Errorf("list workflow revisions: %w", err)
	}
	defer rows.Close()
	items := make([]workflow.WorkflowRevisionSummary, 0)
	for rows.Next() {
		var item workflow.WorkflowRevisionSummary
		if err := rows.Scan(&item.RevisionID, &item.WorkflowID, &item.RevisionNo, &item.Name, &item.Description, &item.DefinitionHash, &item.Note, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan workflow revision: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *WorkflowDefinitionRepository) GetRevision(ctx context.Context, ownerUserID, workflowID, revisionID string) (*workflow.WorkflowRevision, error) {
	var item workflow.WorkflowRevision
	var definitionJSON string
	err := r.db.QueryRowContext(ctx, `
		SELECT revision_id, workflow_id, owner_user_id, revision_no, name, description, definition_json, definition_hash, note, created_at
		FROM extension_workflow_revisions
		WHERE owner_user_id = ? AND workflow_id = ? AND revision_id = ?
	`, ownerUserID, workflowID, revisionID).Scan(&item.RevisionID, &item.WorkflowID, &item.OwnerUserID, &item.RevisionNo, &item.Name, &item.Description, &definitionJSON, &item.DefinitionHash, &item.Note, &item.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, workflow.ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("get workflow revision: %w", err)
	}
	if err := json.Unmarshal([]byte(definitionJSON), &item.Definition); err != nil {
		return nil, fmt.Errorf("unmarshal workflow revision: %w", err)
	}
	return &item, nil
}

func (r *WorkflowDefinitionRepository) DeleteRevisionsByWorkflow(ctx context.Context, workflowID string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM extension_workflow_revisions WHERE workflow_id = ?`, workflowID); err != nil {
		return fmt.Errorf("delete workflow revisions: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) SaveTemplate(ctx context.Context, ownerUserID, name, description string, def workflow.WorkflowDefinition) (*workflow.WorkflowTemplate, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	name = strings.TrimSpace(name)
	if ownerUserID == "" || name == "" {
		return nil, fmt.Errorf("workflow template requires owner and name")
	}
	definitionJSON, err := json.Marshal(def)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow template: %w", err)
	}
	now := time.Now().UTC()
	item := &workflow.WorkflowTemplate{
		TemplateID:     "wftpl-" + uuid.NewString(),
		OwnerUserID:    ownerUserID,
		Name:           name,
		Description:    strings.TrimSpace(description),
		Definition:     def,
		DefinitionHash: workflow.ComputeDefinitionHash(def),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_templates
			(template_id, owner_user_id, name, description, definition_json, definition_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, item.TemplateID, item.OwnerUserID, item.Name, item.Description, definitionJSON, item.DefinitionHash, item.CreatedAt, item.UpdatedAt); err != nil {
		return nil, fmt.Errorf("save workflow template: %w", err)
	}
	return item, nil
}

func (r *WorkflowDefinitionRepository) ListTemplates(ctx context.Context, ownerUserID string) ([]workflow.WorkflowTemplateSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT template_id, name, description, definition_json, definition_hash, created_at, updated_at
		FROM extension_workflow_templates WHERE owner_user_id = ? ORDER BY updated_at DESC, name
	`, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list workflow templates: %w", err)
	}
	defer rows.Close()
	items := make([]workflow.WorkflowTemplateSummary, 0)
	for rows.Next() {
		var item workflow.WorkflowTemplateSummary
		var definitionJSON string
		if err := rows.Scan(&item.TemplateID, &item.Name, &item.Description, &definitionJSON, &item.DefinitionHash, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workflow template: %w", err)
		}
		var def workflow.WorkflowDefinition
		if err := json.Unmarshal([]byte(definitionJSON), &def); err != nil {
			return nil, fmt.Errorf("unmarshal workflow template %s: %w", item.TemplateID, err)
		}
		item.NodeCount = len(def.Nodes)
		item.TriggerCount = len(def.Triggers)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *WorkflowDefinitionRepository) GetTemplate(ctx context.Context, ownerUserID, templateID string) (*workflow.WorkflowTemplate, error) {
	var item workflow.WorkflowTemplate
	var definitionJSON string
	err := r.db.QueryRowContext(ctx, `
		SELECT template_id, owner_user_id, name, description, definition_json, definition_hash, created_at, updated_at
		FROM extension_workflow_templates WHERE owner_user_id = ? AND template_id = ?
	`, ownerUserID, templateID).Scan(&item.TemplateID, &item.OwnerUserID, &item.Name, &item.Description, &definitionJSON, &item.DefinitionHash, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, workflow.ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("get workflow template: %w", err)
	}
	if err := json.Unmarshal([]byte(definitionJSON), &item.Definition); err != nil {
		return nil, fmt.Errorf("unmarshal workflow template: %w", err)
	}
	return &item, nil
}

func (r *WorkflowDefinitionRepository) DeleteTemplate(ctx context.Context, ownerUserID, templateID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM extension_workflow_templates WHERE owner_user_id = ? AND template_id = ?`, ownerUserID, templateID)
	if err != nil {
		return fmt.Errorf("delete workflow template: %w", err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return workflow.ErrWorkflowNotFound
	}
	return nil
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
			(binding_id, trigger_type, event_type, schedule_id, workflow_id, config_json, input_json, generation, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(binding_id) DO UPDATE SET
			trigger_type = excluded.trigger_type, event_type = excluded.event_type,
			schedule_id = excluded.schedule_id, workflow_id = excluded.workflow_id,
			config_json = excluded.config_json, input_json = excluded.input_json, generation = excluded.generation,
			enabled = excluded.enabled, updated_at = excluded.updated_at
	`, binding.BindingID, binding.Type, binding.EventType, binding.ScheduleID, binding.WorkflowID, normalizedRawJSON(binding.Config), normalizedRawJSON(binding.Input), binding.Generation, boolToInt(binding.Enabled), now, now)
	if err != nil {
		return fmt.Errorf("save workflow trigger binding: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) ListTriggers(ctx context.Context, triggerType workflow.TriggerType, eventType, scheduleID string) ([]workflow.TriggerBinding, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT binding_id, trigger_type, event_type, schedule_id, workflow_id, config_json, input_json, generation, enabled
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
		var configJSON, inputJSON string
		var enabled int
		if err := rows.Scan(&binding.BindingID, &binding.Type, &binding.EventType, &binding.ScheduleID, &binding.WorkflowID, &configJSON, &inputJSON, &binding.Generation, &enabled); err != nil {
			return nil, fmt.Errorf("scan workflow trigger binding: %w", err)
		}
		binding.Config = json.RawMessage(configJSON)
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

func (r *WorkflowDefinitionRepository) ClaimTriggerReceipt(ctx context.Context, eventID, bindingID, invocationID string, occurredAt time.Time) (bool, error) {
	now := time.Now().UTC()
	r.maybePruneTriggerReceipts(ctx, now)
	result, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO extension_workflow_trigger_receipts
			(event_id, binding_id, invocation_id, status, occurred_at, created_at, updated_at)
		VALUES (?, ?, ?, 'claimed', ?, ?, ?)
	`, eventID, bindingID, invocationID, occurredAt.UTC(), now, now)
	if err != nil {
		return false, fmt.Errorf("claim workflow trigger receipt: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim workflow trigger receipt rows: %w", err)
	}
	if rows > 0 {
		return true, nil
	}

	// A claimed receipt is a durable lease for the deterministic workflow
	// invocation. Do not reclaim it only because wall-clock time elapsed: a
	// legitimate long-running workflow may have no receipt heartbeat, and the
	// workflow recovery subsystem already owns recovery of persisted active runs.
	// We only recycle a stale receipt when the matching execution is absent or
	// terminally retryable (failed/cancelled). Completed/paused runs are treated as
	// consumed trigger deliveries, while active runs remain claimed.
	var receiptStatus, receiptInvocation string
	var receiptUpdated time.Time
	err = r.db.QueryRowContext(ctx, `
		SELECT status, invocation_id, updated_at
		FROM extension_workflow_trigger_receipts
		WHERE event_id = ? AND binding_id = ?
	`, eventID, bindingID).Scan(&receiptStatus, &receiptInvocation, &receiptUpdated)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("read workflow trigger receipt: %w", err)
	}
	if receiptStatus == "completed" {
		return false, nil
	}
	if receiptStatus != "claimed" || now.Sub(receiptUpdated.UTC()) < 15*time.Minute {
		return false, nil
	}
	if strings.TrimSpace(receiptInvocation) == "" {
		receiptInvocation = invocationID
	}

	var runStatus string
	err = r.db.QueryRowContext(ctx, `
		SELECT status FROM extension_workflow_executions WHERE execution_id = ?
	`, receiptInvocation).Scan(&runStatus)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("inspect workflow trigger execution: %w", err)
	}
	if err == nil {
		switch workflow.RunStatus(runStatus) {
		case workflow.RunStatusSucceeded, workflow.RunStatusCompensated,
			workflow.RunStatusPaused, workflow.RunStatusWaitingDevice:
			if _, updateErr := r.db.ExecContext(ctx, `
				UPDATE extension_workflow_trigger_receipts
				SET status = 'completed', updated_at = ?
				WHERE event_id = ? AND binding_id = ? AND status = 'claimed'
			`, now, eventID, bindingID); updateErr != nil {
				return false, fmt.Errorf("finalize workflow trigger receipt from execution: %w", updateErr)
			}
			return false, nil
		case workflow.RunStatusRunning, workflow.RunStatusPausing, workflow.RunStatusResuming, workflow.RunStatusCompensating:
			return false, nil
		case workflow.RunStatusFailed, workflow.RunStatusCancelled:
			// Retry below with the same deterministic invocation id. WorkflowExecutor
			// will reuse the persisted run rather than creating a second identity.
		default:
			return false, nil
		}
	}

	result, err = r.db.ExecContext(ctx, `
		UPDATE extension_workflow_trigger_receipts
		SET invocation_id = ?, status = 'claimed', occurred_at = ?, updated_at = ?
		WHERE event_id = ? AND binding_id = ? AND status = 'claimed' AND updated_at < ?
	`, invocationID, occurredAt.UTC(), now, eventID, bindingID, now.Add(-15*time.Minute))
	if err != nil {
		return false, fmt.Errorf("reclaim workflow trigger receipt: %w", err)
	}
	rows, err = result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("reclaim workflow trigger receipt rows: %w", err)
	}
	return rows > 0, nil
}

func (r *WorkflowDefinitionRepository) maybePruneTriggerReceipts(ctx context.Context, now time.Time) {
	if r == nil || r.db == nil {
		return
	}
	r.receiptCleanupMu.Lock()
	if !r.lastReceiptCleanup.IsZero() && now.Sub(r.lastReceiptCleanup) < triggerReceiptCleanupInterval {
		r.receiptCleanupMu.Unlock()
		return
	}
	r.lastReceiptCleanup = now
	r.receiptCleanupMu.Unlock()

	// Never age out an in-flight receipt solely by time. A workflow can remain
	// running/paused for longer than the retention window, and deleting its claim
	// would allow the same producer event to enter TriggerManager again. Completed
	// receipts are safe to prune because workflow execution keeps the same durable
	// idempotency identity. Claimed receipts are pruned only when their execution is
	// absent (crash before Start) or terminally retryable (failed/cancelled).
	_, _ = r.db.ExecContext(ctx, `
		DELETE FROM extension_workflow_trigger_receipts AS receipt
		WHERE receipt.updated_at < ?
		  AND (
			receipt.status = 'completed'
			OR (
				receipt.status = 'claimed'
				AND (
					NOT EXISTS (
						SELECT 1 FROM extension_workflow_executions AS run
						WHERE run.execution_id = receipt.invocation_id
					)
					OR EXISTS (
						SELECT 1 FROM extension_workflow_executions AS run
						WHERE run.execution_id = receipt.invocation_id
						  AND run.status IN ('failed', 'cancelled')
					)
				)
			)
		  )
	`, now.Add(-triggerReceiptRetention))
}

func (r *WorkflowDefinitionRepository) CompleteTriggerReceipt(ctx context.Context, eventID, bindingID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE extension_workflow_trigger_receipts
		SET status = 'completed', updated_at = ?
		WHERE event_id = ? AND binding_id = ?
	`, time.Now().UTC(), eventID, bindingID)
	if err != nil {
		return fmt.Errorf("complete workflow trigger receipt: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) ReleaseTriggerReceipt(ctx context.Context, eventID, bindingID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM extension_workflow_trigger_receipts
		WHERE event_id = ? AND binding_id = ? AND status = 'claimed'
	`, eventID, bindingID)
	if err != nil {
		return fmt.Errorf("release workflow trigger receipt: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) DeleteTriggersByWorkflow(ctx context.Context, workflowID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM extension_workflow_trigger_bindings WHERE workflow_id = ?`, workflowID)
	if err != nil {
		return fmt.Errorf("delete workflow trigger bindings: %w", err)
	}
	return nil
}

func normalizedRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage(`{}`)
	}
	return raw
}

type scannerInterface interface {
	Scan(dest ...any) error
}

func scanWorkflowDefinition(row scannerInterface) (*workflow.WorkflowDefinition, error) {
	var def workflow.WorkflowDefinition
	var inputSchema, outputSchema, nodesJSON, edgesJSON, triggersJSON, permissionsJSON, limitsJSON, metadataJSON string
	var callableByAgent, enabled, hasSideEffects, idempotent int

	err := row.Scan(
		&def.ID, &def.ExtensionID, &def.ModuleID, &def.Name, &def.Description,
		&def.SchemaVersion, &def.Version,
		&inputSchema, &outputSchema, &nodesJSON, &edgesJSON, &triggersJSON, &permissionsJSON, &def.Scope,
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
	if edgesJSON != "" {
		if err := json.Unmarshal([]byte(edgesJSON), &def.Edges); err != nil {
			return nil, fmt.Errorf("unmarshal edges: %w", err)
		}
	}
	if triggersJSON != "" {
		if err := json.Unmarshal([]byte(triggersJSON), &def.Triggers); err != nil {
			return nil, fmt.Errorf("unmarshal triggers: %w", err)
		}
	}
	if def.SchemaVersion != workflow.UserWorkflowSchemaVersion && len(def.Edges) == 0 {
		def.Edges = workflow.DeriveEdges(def.Nodes)
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

func (r *WorkflowExecutionRepository) DeleteByWorkflow(ctx context.Context, workflowID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workflow execution cleanup: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	statements := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM extension_workflow_compensations WHERE execution_id IN (SELECT execution_id FROM extension_workflow_executions WHERE workflow_id = ?)`, []any{workflowID}},
		{`DELETE FROM extension_workflow_step_attempts WHERE workflow_id = ?`, []any{workflowID}},
		{`DELETE FROM extension_workflow_step_runs WHERE workflow_id = ?`, []any{workflowID}},
		{`DELETE FROM extension_workflow_checkpoints WHERE workflow_id = ?`, []any{workflowID}},
		{`DELETE FROM extension_workflow_executions WHERE workflow_id = ?`, []any{workflowID}},
	}
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt.query, stmt.args...); err != nil {
			return fmt.Errorf("cleanup workflow execution data: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workflow execution cleanup: %w", err)
	}
	return nil
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
			contextJSON, marshalErr := json.Marshal(run.Context)
			if marshalErr != nil {
				return nil, false, fmt.Errorf("marshal restarted workflow context: %w", marshalErr)
			}
			_, updateErr := r.db.ExecContext(ctx, `
				UPDATE extension_workflow_executions
				SET status = ?, error_message = '', output_json = NULL, finished_at = NULL,
					attempt = attempt + 1, generation = ?, context_json = ?, operation_id = ?, trace_id = ?,
					pause_reason = '', pause_requested_at = NULL, paused_at = NULL, updated_at = ?
				WHERE execution_id = ?
			`, workflow.RunStatusRunning, run.Context.Generation, contextJSON, run.Context.OperationID, run.Context.TraceID, time.Now().UTC(), run.ExecutionID)
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

func (r *WorkflowExecutionRepository) ListWaitingDevice(ctx context.Context, userID, deviceID string, limit int) ([]workflow.WorkflowRun, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT execution_id, workflow_id, status, input_json, output_json, error_message,
			context_json, steps_json, compensation_json, attempt, generation,
			pause_reason, pause_requested_at, paused_at,
			started_at, finished_at, updated_at
		FROM extension_workflow_executions
		WHERE status = ?
		ORDER BY updated_at ASC LIMIT ?
	`, workflow.RunStatusWaitingDevice, limit)
	if err != nil {
		return nil, fmt.Errorf("list waiting-device workflow executions: %w", err)
	}
	defer rows.Close()

	wantedUser := strings.TrimSpace(userID)
	wantedDevice := strings.TrimSpace(deviceID)
	result := make([]workflow.WorkflowRun, 0)
	for rows.Next() {
		run, scanErr := r.scanRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		if wantedUser != "" && strings.TrimSpace(run.Context.UserID) != wantedUser {
			continue
		}
		reason := strings.TrimSpace(strings.TrimPrefix(run.PauseReason, "waiting_device:"))
		if wantedDevice != "" && reason != "" && reason != wantedDevice {
			continue
		}
		result = append(result, *run)
	}
	return result, rows.Err()
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

func (r *WorkflowExecutionRepository) ListRuns(ctx context.Context, workflowID string, status workflow.RunStatus, limit, offset int) ([]workflow.WorkflowRun, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where := "WHERE (? = '' OR workflow_id = ?) AND (? = '' OR status = ?)"
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM extension_workflow_executions `+where, workflowID, workflowID, string(status), string(status)).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count workflow executions: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT execution_id, workflow_id, status, input_json, output_json, error_message,
			context_json, steps_json, compensation_json, attempt, generation,
			pause_reason, pause_requested_at, paused_at,
			started_at, finished_at, updated_at
		FROM extension_workflow_executions `+where+`
		ORDER BY started_at DESC LIMIT ? OFFSET ?
	`, workflowID, workflowID, string(status), string(status), limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list workflow executions: %w", err)
	}
	defer rows.Close()
	items := make([]workflow.WorkflowRun, 0)
	for rows.Next() {
		run, err := r.scanRun(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *run)
	}
	return items, total, rows.Err()
}

func (r *WorkflowExecutionRepository) SaveAttempt(ctx context.Context, attempt workflow.StepAttemptRun) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_step_attempts
			(execution_id, workflow_id, node_id, attempt, generation, status, input_json, output_json, error_message, next_backoff_ms, started_at, finished_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(execution_id, node_id, generation, attempt) DO UPDATE SET
			status = excluded.status, input_json = excluded.input_json, output_json = excluded.output_json,
			error_message = excluded.error_message, next_backoff_ms = excluded.next_backoff_ms,
			started_at = excluded.started_at, finished_at = excluded.finished_at
	`, attempt.ExecutionID, attempt.WorkflowID, attempt.NodeID, attempt.Attempt, attempt.Generation, attempt.Status, attempt.Input, attempt.Output, attempt.Error, attempt.NextBackoffMS, attempt.StartedAt, attempt.FinishedAt, now)
	if err != nil {
		return fmt.Errorf("save workflow step attempt: %w", err)
	}
	return nil
}

func (r *WorkflowExecutionRepository) ListStepAttempts(ctx context.Context, executionID string) ([]workflow.StepAttemptRun, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT execution_id, workflow_id, node_id, attempt, generation, status, input_json, output_json,
			error_message, next_backoff_ms, started_at, finished_at
		FROM extension_workflow_step_attempts
		WHERE execution_id = ? ORDER BY generation, started_at, node_id, attempt
	`, executionID)
	if err != nil {
		return nil, fmt.Errorf("list workflow step attempts: %w", err)
	}
	defer rows.Close()
	items := make([]workflow.StepAttemptRun, 0)
	for rows.Next() {
		var item workflow.StepAttemptRun
		var inputJSON, outputJSON, errorMessage sql.NullString
		if err := rows.Scan(&item.ExecutionID, &item.WorkflowID, &item.NodeID, &item.Attempt, &item.Generation, &item.Status, &inputJSON, &outputJSON, &errorMessage, &item.NextBackoffMS, &item.StartedAt, &item.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan workflow step attempt: %w", err)
		}
		if inputJSON.Valid {
			item.Input = json.RawMessage(inputJSON.String)
		}
		if outputJSON.Valid {
			item.Output = json.RawMessage(outputJSON.String)
		}
		if errorMessage.Valid {
			item.Error = errorMessage.String
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *WorkflowExecutionRepository) GetStats(ctx context.Context, workflowID string) (workflow.WorkflowExecutionStats, error) {
	stats := workflow.WorkflowExecutionStats{NodeStatistics: []workflow.NodeExecutionStat{}}
	var lastRunAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'succeeded' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'compensated' THEN 1 ELSE 0 END), 0),
			COALESCE(AVG(NULLIF(duration_ms, 0)), 0), MAX(started_at)
		FROM extension_workflow_executions WHERE workflow_id = ?
	`, workflowID).Scan(&stats.RunCount, &stats.Succeeded, &stats.Failed, &stats.Cancelled, &stats.Compensated, &stats.AverageRunMS, &lastRunAt); err != nil {
		return stats, fmt.Errorf("workflow stats: %w", err)
	}
	if lastRunAt.Valid {
		t := lastRunAt.Time
		stats.LastRunAt = &t
	}
	terminalRuns := stats.Succeeded + stats.Failed + stats.Cancelled + stats.Compensated
	if terminalRuns > 0 {
		stats.SuccessRate = float64(stats.Succeeded+stats.Compensated) / float64(terminalRuns)
	}
	_ = r.db.QueryRowContext(ctx, `SELECT error_message FROM extension_workflow_executions WHERE workflow_id = ? AND error_message != '' ORDER BY started_at DESC LIMIT 1`, workflowID).Scan(&stats.LastError)

	rows, err := r.db.QueryContext(ctx, `
		SELECT s.node_id, COUNT(*),
			COALESCE(SUM(CASE WHEN s.status IN ('succeeded','defaulted') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN s.status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(AVG((julianday(s.finished_at)-julianday(s.started_at))*86400000.0), 0),
			COALESCE(AVG(s.attempt), 0),
			COALESCE((SELECT COUNT(*) FROM extension_workflow_step_attempts a WHERE a.workflow_id = ? AND a.node_id = s.node_id AND a.status = 'timed_out'), 0)
		FROM extension_workflow_step_runs s
		WHERE s.workflow_id = ?
		GROUP BY s.node_id ORDER BY s.node_id
	`, workflowID, workflowID)
	if err != nil {
		return stats, fmt.Errorf("workflow node stats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item workflow.NodeExecutionStat
		if err := rows.Scan(&item.NodeID, &item.RunCount, &item.Succeeded, &item.Failed, &item.AverageStepMS, &item.AverageAttempts, &item.TimedOut); err != nil {
			return stats, fmt.Errorf("scan workflow node stats: %w", err)
		}
		stats.NodeStatistics = append(stats.NodeStatistics, item)
	}
	return stats, rows.Err()
}

func (r *WorkflowExecutionRepository) ListStepRuns(ctx context.Context, executionID string) ([]workflow.StepRun, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT execution_id, workflow_id, node_id, status, input_json, output_json,
			error_message, attempt, started_at, finished_at
		FROM extension_workflow_step_runs
		WHERE execution_id = ? ORDER BY started_at, node_id
	`, executionID)
	if err != nil {
		return nil, fmt.Errorf("list workflow step runs: %w", err)
	}
	defer rows.Close()
	items := make([]workflow.StepRun, 0)
	for rows.Next() {
		var step workflow.StepRun
		var inputJSON, outputJSON, errorMessage sql.NullString
		var finishedAt sql.NullTime
		if err := rows.Scan(&step.ExecutionID, &step.WorkflowID, &step.NodeID, &step.Status, &inputJSON, &outputJSON, &errorMessage, &step.Attempt, &step.StartedAt, &finishedAt); err != nil {
			return nil, fmt.Errorf("scan workflow step run: %w", err)
		}
		if errorMessage.Valid {
			step.Error = errorMessage.String
		}
		if inputJSON.Valid {
			step.Input = json.RawMessage(inputJSON.String)
		}
		if outputJSON.Valid {
			step.Output = json.RawMessage(outputJSON.String)
		}
		if finishedAt.Valid {
			t := finishedAt.Time
			step.FinishedAt = &t
		}
		items = append(items, step)
	}
	return items, rows.Err()
}

func (r *WorkflowExecutionRepository) scanRun(row scannerInterface) (*workflow.WorkflowRun, error) {
	var run workflow.WorkflowRun
	var inputJSON, outputJSON, contextJSON, stepsJSON, compensationJSON sql.NullString
	var errorMessage sql.NullString
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
	if errorMessage.Valid {
		run.Error = errorMessage.String
	}
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
