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
	agentToolJSON, err := json.Marshal(def.AgentTool)
	if err != nil {
		return fmt.Errorf("marshal agent tool: %w", err)
	}
	concurrencyJSON, err := json.Marshal(def.ConcurrencyPolicy.Normalize())
	if err != nil {
		return fmt.Errorf("marshal concurrency policy: %w", err)
	}
	metadataJSON := []byte("{}")
	if def.Metadata != nil {
		if b, marshalErr := json.Marshal(def.Metadata); marshalErr == nil {
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

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workflow definition save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO extension_workflow_definitions
			(workflow_id, extension_id, module_id, name, description, schema_version, version,
			 input_schema_json, output_schema_json, nodes_json, edges_json, triggers_json, permissions_json, scope,
			 callable_by_agent, agent_tool_json, enabled, has_side_effects, idempotent, limits_json, concurrency_policy_json,
			 source, metadata_json, definition_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			agent_tool_json = excluded.agent_tool_json,
			enabled = excluded.enabled,
			has_side_effects = excluded.has_side_effects,
			idempotent = excluded.idempotent,
			limits_json = excluded.limits_json,
			concurrency_policy_json = excluded.concurrency_policy_json,
			source = excluded.source,
			metadata_json = excluded.metadata_json,
			definition_hash = excluded.definition_hash,
			updated_at = excluded.updated_at
	`, def.ID, def.ExtensionID, def.ModuleID, def.Name, def.Description, def.SchemaVersion, def.Version,
		inputSchema, outputSchema, nodesJSON, edgesJSON, triggersJSON, permissionsJSON, def.Scope,
		boolToInt(def.CallableByAgent), agentToolJSON, boolToInt(def.Enabled), boolToInt(def.HasSideEffects), boolToInt(def.Idempotent),
		limitsJSON, concurrencyJSON, def.Source, metadataJSON, defHash, now, now)
	if err != nil {
		return fmt.Errorf("save workflow definition: %w", err)
	}
	if err := r.enqueueWorkflowSyncUpsertTx(ctx, tx, def, defHash, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workflow definition save: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) Delete(ctx context.Context, workflowID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workflow definition delete: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var source, metadataRaw, definitionHash string
	sourceErr := tx.QueryRowContext(ctx, `SELECT source, metadata_json, definition_hash FROM extension_workflow_definitions WHERE workflow_id = ?`, workflowID).Scan(&source, &metadataRaw, &definitionHash)
	if sourceErr != nil && sourceErr != sql.ErrNoRows {
		return fmt.Errorf("read workflow source before delete: %w", sourceErr)
	}
	if source == "user" {
		var metadata map[string]any
		_ = json.Unmarshal([]byte(metadataRaw), &metadata)
		owner := strings.TrimSpace(fmt.Sprint(metadata["ownerUserId"]))
		if err := r.enqueueWorkflowSyncDeleteTx(ctx, tx, owner, workflowID, definitionHash, time.Now().UTC()); err != nil {
			return err
		}
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
			callable_by_agent, agent_tool_json, enabled, has_side_effects, idempotent, limits_json, concurrency_policy_json,
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
			callable_by_agent, agent_tool_json, enabled, has_side_effects, idempotent, limits_json, concurrency_policy_json,
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
	return r.saveRevisionWithState(ctx, ownerUserID, def, note, workflow.WorkflowRevisionPublished)
}

func (r *WorkflowDefinitionRepository) SaveDraftRevision(ctx context.Context, ownerUserID string, def workflow.WorkflowDefinition, note string) (*workflow.WorkflowRevision, error) {
	return r.saveRevisionWithState(ctx, ownerUserID, def, note, workflow.WorkflowRevisionDraft)
}

// EnsurePublishedRevision returns the immutable published revision representing
// the supplied definition. It is safe to call before every run: identical
// published revisions are reused, while an identical draft is atomically
// promoted instead of creating another row.
func (r *WorkflowDefinitionRepository) EnsurePublishedRevision(ctx context.Context, ownerUserID string, def workflow.WorkflowDefinition, note string) (*workflow.WorkflowRevision, error) {
	return r.saveRevisionWithState(ctx, ownerUserID, def, note, workflow.WorkflowRevisionPublished)
}

func (r *WorkflowDefinitionRepository) saveRevisionWithState(ctx context.Context, ownerUserID string, def workflow.WorkflowDefinition, note string, state workflow.WorkflowRevisionState) (*workflow.WorkflowRevision, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" || strings.TrimSpace(def.ID) == "" {
		return nil, fmt.Errorf("workflow revision requires owner and workflow id")
	}
	if !state.Valid() || state == workflow.WorkflowRevisionArchived {
		return nil, fmt.Errorf("new workflow revision state must be draft or published")
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
		SELECT revision_id, revision_no, name, description, definition_hash, note, state, published_at, archived_at, created_at
		FROM extension_workflow_revisions
		WHERE owner_user_id = ? AND workflow_id = ?
		ORDER BY revision_no DESC LIMIT 1
	`, ownerUserID, def.ID).Scan(
		&latest.RevisionID, &latest.RevisionNo, &latest.Name, &latest.Description,
		&latestHash, &latest.Note, &latest.State, &latest.PublishedAt, &latest.ArchivedAt, &latest.CreatedAt,
	)
	if latestErr != nil && latestErr != sql.ErrNoRows {
		return nil, fmt.Errorf("get latest workflow revision: %w", latestErr)
	}
	if latestErr == nil && latestHash == hash && latest.State != workflow.WorkflowRevisionArchived {
		now := time.Now().UTC()
		if state == workflow.WorkflowRevisionPublished && latest.State != workflow.WorkflowRevisionPublished {
			if _, err := tx.ExecContext(ctx, `
				UPDATE extension_workflow_revisions
				SET state = ?, published_at = COALESCE(published_at, ?), archived_at = NULL
				WHERE revision_id = ? AND owner_user_id = ? AND workflow_id = ?
			`, workflow.WorkflowRevisionPublished, now, latest.RevisionID, ownerUserID, def.ID); err != nil {
				return nil, fmt.Errorf("publish matching workflow revision: %w", err)
			}
			latest.State = workflow.WorkflowRevisionPublished
			latest.PublishedAt = &now
			latest.ArchivedAt = nil
		}
		latest.WorkflowID = def.ID
		latest.OwnerUserID = ownerUserID
		latest.Definition = def
		latest.DefinitionHash = latestHash
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit workflow revision reuse: %w", err)
		}
		return &latest, nil
	}

	var revisionNo int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision_no), 0) + 1 FROM extension_workflow_revisions WHERE workflow_id = ?`, def.ID).Scan(&revisionNo); err != nil {
		return nil, fmt.Errorf("next workflow revision: %w", err)
	}
	now := time.Now().UTC()
	var publishedAt *time.Time
	if state == workflow.WorkflowRevisionPublished {
		publishedAt = &now
	}
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
		State:          state,
		PublishedAt:    publishedAt,
		CreatedAt:      now,
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO extension_workflow_revisions
			(revision_id, workflow_id, owner_user_id, revision_no, name, description, definition_json, definition_hash, note, state, published_at, archived_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?)
	`, revision.RevisionID, revision.WorkflowID, revision.OwnerUserID, revision.RevisionNo, revision.Name, revision.Description, definitionJSON, revision.DefinitionHash, revision.Note, revision.State, revision.PublishedAt, revision.CreatedAt); err != nil {
		return nil, fmt.Errorf("save workflow revision: %w", err)
	}
	// Only explicitly archived history is eligible for automatic pruning. A
	// published revision can still be referenced by an audit trail or old run.
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM extension_workflow_revisions
		WHERE owner_user_id = ? AND workflow_id = ? AND state = ? AND revision_id NOT IN (
			SELECT revision_id FROM extension_workflow_revisions
			WHERE owner_user_id = ? AND workflow_id = ? AND state = ?
			ORDER BY revision_no DESC LIMIT 50
		)
	`, ownerUserID, def.ID, workflow.WorkflowRevisionArchived, ownerUserID, def.ID, workflow.WorkflowRevisionArchived); err != nil {
		return nil, fmt.Errorf("prune archived workflow revisions: %w", err)
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
		SELECT revision_id, workflow_id, revision_no, name, description, definition_hash, note, state, published_at, archived_at, created_at
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
		if err := rows.Scan(&item.RevisionID, &item.WorkflowID, &item.RevisionNo, &item.Name, &item.Description, &item.DefinitionHash, &item.Note, &item.State, &item.PublishedAt, &item.ArchivedAt, &item.CreatedAt); err != nil {
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
		SELECT revision_id, workflow_id, owner_user_id, revision_no, name, description, definition_json, definition_hash, note, state, published_at, archived_at, created_at
		FROM extension_workflow_revisions
		WHERE owner_user_id = ? AND workflow_id = ? AND revision_id = ?
	`, ownerUserID, workflowID, revisionID).Scan(&item.RevisionID, &item.WorkflowID, &item.OwnerUserID, &item.RevisionNo, &item.Name, &item.Description, &definitionJSON, &item.DefinitionHash, &item.Note, &item.State, &item.PublishedAt, &item.ArchivedAt, &item.CreatedAt)
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

func (r *WorkflowDefinitionRepository) FindRevisionByHash(ctx context.Context, ownerUserID, workflowID, definitionHash string) (*workflow.WorkflowRevision, error) {
	var revisionID string
	err := r.db.QueryRowContext(ctx, `
		SELECT revision_id
		FROM extension_workflow_revisions
		WHERE owner_user_id = ? AND workflow_id = ? AND definition_hash = ? AND state != ?
		ORDER BY CASE state WHEN 'published' THEN 0 WHEN 'draft' THEN 1 ELSE 2 END, revision_no DESC
		LIMIT 1
	`, strings.TrimSpace(ownerUserID), strings.TrimSpace(workflowID), strings.TrimSpace(definitionHash), workflow.WorkflowRevisionArchived).Scan(&revisionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, workflow.ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("find workflow revision by hash: %w", err)
	}
	return r.GetRevision(ctx, ownerUserID, workflowID, revisionID)
}

func (r *WorkflowDefinitionRepository) PublishRevision(ctx context.Context, ownerUserID, workflowID, revisionID string) (*workflow.WorkflowRevision, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE extension_workflow_revisions
		SET state = ?, published_at = COALESCE(published_at, ?), archived_at = NULL
		WHERE owner_user_id = ? AND workflow_id = ? AND revision_id = ?
	`, workflow.WorkflowRevisionPublished, now, strings.TrimSpace(ownerUserID), strings.TrimSpace(workflowID), strings.TrimSpace(revisionID))
	if err != nil {
		return nil, fmt.Errorf("publish workflow revision: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return nil, workflow.ErrWorkflowNotFound
	}
	return r.GetRevision(ctx, ownerUserID, workflowID, revisionID)
}

func (r *WorkflowDefinitionRepository) ArchiveRevision(ctx context.Context, ownerUserID, workflowID, revisionID string) (*workflow.WorkflowRevision, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE extension_workflow_revisions
		SET state = ?, archived_at = ?
		WHERE owner_user_id = ? AND workflow_id = ? AND revision_id = ?
	`, workflow.WorkflowRevisionArchived, now, strings.TrimSpace(ownerUserID), strings.TrimSpace(workflowID), strings.TrimSpace(revisionID))
	if err != nil {
		return nil, fmt.Errorf("archive workflow revision: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return nil, workflow.ErrWorkflowNotFound
	}
	return r.GetRevision(ctx, ownerUserID, workflowID, revisionID)
}

// RestoreRevisionLifecycle restores only the lifecycle metadata of a revision.
// It is intentionally narrow and is used by API-level publish sagas when a
// later registry/trigger/installation mutation fails after the revision row was
// promoted. The immutable definition payload/hash are never modified here.
func (r *WorkflowDefinitionRepository) RestoreRevisionLifecycle(
	ctx context.Context,
	ownerUserID, workflowID, revisionID string,
	state workflow.WorkflowRevisionState,
	publishedAt, archivedAt *time.Time,
) (*workflow.WorkflowRevision, error) {
	if !state.Valid() {
		return nil, fmt.Errorf("invalid workflow revision lifecycle state %q", state)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE extension_workflow_revisions
		SET state = ?, published_at = ?, archived_at = ?
		WHERE owner_user_id = ? AND workflow_id = ? AND revision_id = ?
	`, state, publishedAt, archivedAt, strings.TrimSpace(ownerUserID), strings.TrimSpace(workflowID), strings.TrimSpace(revisionID))
	if err != nil {
		return nil, fmt.Errorf("restore workflow revision lifecycle: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return nil, workflow.ErrWorkflowNotFound
	}
	return r.GetRevision(ctx, ownerUserID, workflowID, revisionID)
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
		case workflow.RunStatusSucceeded, workflow.RunStatusCompensated, workflow.RunStatusDropped,
			workflow.RunStatusPaused, workflow.RunStatusWaitingDevice:
			if _, updateErr := r.db.ExecContext(ctx, `
				UPDATE extension_workflow_trigger_receipts
				SET status = 'completed', updated_at = ?
				WHERE event_id = ? AND binding_id = ? AND status = 'claimed'
			`, now, eventID, bindingID); updateErr != nil {
				return false, fmt.Errorf("finalize workflow trigger receipt from execution: %w", updateErr)
			}
			return false, nil
		case workflow.RunStatusQueued, workflow.RunStatusRunning, workflow.RunStatusPausing, workflow.RunStatusResuming, workflow.RunStatusCompensating,
			workflow.RunStatusCancelRequested, workflow.RunStatusCancelling:
			return false, nil
		case workflow.RunStatusFailed, workflow.RunStatusCancelled, workflow.RunStatusCancelTimeout, workflow.RunStatusCancelFailed:
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
	var inputSchema, outputSchema, nodesJSON, edgesJSON, triggersJSON, permissionsJSON, agentToolJSON, limitsJSON, concurrencyJSON, metadataJSON string
	var callableByAgent, enabled, hasSideEffects, idempotent int

	err := row.Scan(
		&def.ID, &def.ExtensionID, &def.ModuleID, &def.Name, &def.Description,
		&def.SchemaVersion, &def.Version,
		&inputSchema, &outputSchema, &nodesJSON, &edgesJSON, &triggersJSON, &permissionsJSON, &def.Scope,
		&callableByAgent, &agentToolJSON, &enabled, &hasSideEffects, &idempotent, &limitsJSON, &concurrencyJSON,
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
	if agentToolJSON != "" && agentToolJSON != "{}" {
		if err := json.Unmarshal([]byte(agentToolJSON), &def.AgentTool); err != nil {
			return nil, fmt.Errorf("unmarshal agent tool: %w", err)
		}
	}
	if concurrencyJSON != "" && concurrencyJSON != "{}" {
		if err := json.Unmarshal([]byte(concurrencyJSON), &def.ConcurrencyPolicy); err != nil {
			return nil, fmt.Errorf("unmarshal concurrency policy: %w", err)
		}
	}

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
	startStatus := run.Status
	if startStatus == "" {
		startStatus = workflow.RunStatusRunning
	}
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
			`, startStatus, run.Context.Generation, contextJSON, run.Context.OperationID, run.Context.TraceID, time.Now().UTC(), run.ExecutionID)
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
`, run.ExecutionID, run.WorkflowID, startStatus, run.Input,
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
			(execution_id, workflow_id, node_id, status, trace_id, attempt_id, device_id, runtime_id, tool_call_id, fencing_token, idempotency_key, input_json, output_json, error_message, attempt, started_at, finished_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(execution_id, node_id) DO UPDATE SET
			status = excluded.status, trace_id = excluded.trace_id, attempt_id = excluded.attempt_id,
			device_id = excluded.device_id, runtime_id = excluded.runtime_id, tool_call_id = excluded.tool_call_id,
			fencing_token = excluded.fencing_token, idempotency_key = excluded.idempotency_key,
			input_json = excluded.input_json, output_json = excluded.output_json,
			error_message = excluded.error_message, attempt = excluded.attempt,
			started_at = excluded.started_at, finished_at = excluded.finished_at, updated_at = excluded.updated_at
	`, step.ExecutionID, step.WorkflowID, step.NodeID, step.Status, step.TraceID, step.AttemptID, step.DeviceID, step.RuntimeID, step.ToolCallID, step.FencingToken, step.IdempotencyKey, step.Input, step.Output, step.Error, step.Attempt, step.StartedAt, step.FinishedAt, now)
	if err != nil {
		return fmt.Errorf("save workflow step run: %w", err)
	}
	return nil
}

func (r *WorkflowExecutionRepository) GetStep(ctx context.Context, executionID, nodeID string) (*workflow.StepRun, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT execution_id, workflow_id, node_id, status, trace_id, attempt_id, device_id, runtime_id, tool_call_id, fencing_token, idempotency_key, input_json, output_json, error_message, attempt, started_at, finished_at
		FROM extension_workflow_step_runs
		WHERE execution_id = ? AND node_id = ?
	`, executionID, nodeID)
	var step workflow.StepRun
	var inputJSON, outputJSON []byte
	var finishedAt sql.NullTime
	if err := row.Scan(&step.ExecutionID, &step.WorkflowID, &step.NodeID, &step.Status, &step.TraceID, &step.AttemptID, &step.DeviceID, &step.RuntimeID, &step.ToolCallID, &step.FencingToken, &step.IdempotencyKey, &inputJSON, &outputJSON, &step.Error, &step.Attempt, &step.StartedAt, &finishedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workflow step run: %w", err)
	}
	step.Input = append(json.RawMessage(nil), inputJSON...)
	step.Output = append(json.RawMessage(nil), outputJSON...)
	if finishedAt.Valid {
		value := finishedAt.Time
		step.FinishedAt = &value
	}
	return &step, nil
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
	// Definition-driven compensation writes its durable progress through
	// CompensationStore while it runs. Finish only seals the immutable run
	// summary; writing the legacy five-column compensation shape here would
	// discard generation/idempotency/input/output metadata.
	return nil
}

func (r *WorkflowExecutionRepository) UpdateStateCAS(ctx context.Context, run workflow.WorkflowRun, expectedStatus workflow.RunStatus) (bool, error) {
	contextJSON, err := json.Marshal(run.Context)
	if err != nil {
		return false, fmt.Errorf("marshal workflow execution context for state update: %w", err)
	}
	var pauseRequestedAt, pausedAt interface{}
	if run.PauseRequestedAt != nil {
		pauseRequestedAt = *run.PauseRequestedAt
	}
	if run.PausedAt != nil {
		pausedAt = *run.PausedAt
	}

	res, err := r.db.ExecContext(ctx, `
		UPDATE extension_workflow_executions
		SET status = ?, generation = ?, context_json = ?, pause_reason = ?, pause_requested_at = ?, paused_at = ?, updated_at = ?
		WHERE execution_id = ? AND status = ?
	`, run.Status, run.Generation, string(contextJSON), run.PauseReason, pauseRequestedAt, pausedAt, run.UpdatedAt, run.ExecutionID, expectedStatus)
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
		WHERE status IN (?, ?, ?)
		ORDER BY updated_at LIMIT ?
	`, workflow.RunStatusRunning, workflow.RunStatusResuming, workflow.RunStatusCompensating, limit)
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
			(execution_id, workflow_id, node_id, attempt, generation, status, trace_id, attempt_id, device_id, runtime_id, tool_call_id, fencing_token, idempotency_key, input_json, output_json, error_message, next_backoff_ms, started_at, finished_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(execution_id, node_id, generation, attempt) DO UPDATE SET
			status = excluded.status, trace_id = excluded.trace_id, attempt_id = excluded.attempt_id,
			device_id = excluded.device_id, runtime_id = excluded.runtime_id, tool_call_id = excluded.tool_call_id,
			fencing_token = excluded.fencing_token, idempotency_key = excluded.idempotency_key,
			input_json = excluded.input_json, output_json = excluded.output_json,
			error_message = excluded.error_message, next_backoff_ms = excluded.next_backoff_ms,
			started_at = excluded.started_at, finished_at = excluded.finished_at
	`, attempt.ExecutionID, attempt.WorkflowID, attempt.NodeID, attempt.Attempt, attempt.Generation, attempt.Status, attempt.TraceID, attempt.AttemptID, attempt.DeviceID, attempt.RuntimeID, attempt.ToolCallID, attempt.FencingToken, attempt.IdempotencyKey, attempt.Input, attempt.Output, attempt.Error, attempt.NextBackoffMS, attempt.StartedAt, attempt.FinishedAt, now)
	if err != nil {
		return fmt.Errorf("save workflow step attempt: %w", err)
	}
	return nil
}

func (r *WorkflowExecutionRepository) ListStepAttempts(ctx context.Context, executionID string) ([]workflow.StepAttemptRun, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT execution_id, workflow_id, node_id, attempt, generation, status, trace_id, attempt_id, device_id, runtime_id, tool_call_id, fencing_token, idempotency_key, input_json, output_json,
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
		if err := rows.Scan(&item.ExecutionID, &item.WorkflowID, &item.NodeID, &item.Attempt, &item.Generation, &item.Status, &item.TraceID, &item.AttemptID, &item.DeviceID, &item.RuntimeID, &item.ToolCallID, &item.FencingToken, &item.IdempotencyKey, &inputJSON, &outputJSON, &errorMessage, &item.NextBackoffMS, &item.StartedAt, &item.FinishedAt); err != nil {
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

func (r *WorkflowExecutionRepository) SaveSideEffect(ctx context.Context, record workflow.SideEffectRecord) error {
	if strings.TrimSpace(record.JournalID) == "" || strings.TrimSpace(record.ExecutionID) == "" || strings.TrimSpace(record.NodeID) == "" {
		return fmt.Errorf("save workflow side effect: journalId, executionId and nodeId are required")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_side_effect_journal
			(journal_id, execution_id, workflow_id, node_id, attempt, generation, device_id, kind, target,
			 idempotency_key, input_json, output_json, error_message, status, duration_ms, created_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(journal_id) DO UPDATE SET
			output_json = excluded.output_json,
			error_message = excluded.error_message,
			status = excluded.status,
			duration_ms = excluded.duration_ms,
			completed_at = excluded.completed_at
	`, record.JournalID, record.ExecutionID, record.WorkflowID, record.NodeID, record.Attempt, record.Generation,
		record.DeviceID, string(record.Kind), record.Target, record.IdempotencyKey, nullableRawJSON(record.Input),
		nullableRawJSON(record.Output), record.Error, record.Status, record.Duration.Milliseconds(), record.Timestamp, record.CompletedAt)
	if err != nil {
		return fmt.Errorf("save workflow side effect: %w", err)
	}
	return nil
}

func (r *WorkflowExecutionRepository) ListSideEffects(ctx context.Context, executionID string) ([]workflow.SideEffectRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT journal_id, execution_id, workflow_id, node_id, attempt, generation, device_id, kind, target,
			idempotency_key, input_json, output_json, error_message, status, duration_ms, created_at, completed_at
		FROM extension_workflow_side_effect_journal
		WHERE execution_id = ?
		ORDER BY created_at, node_id, attempt
	`, strings.TrimSpace(executionID))
	if err != nil {
		return nil, fmt.Errorf("list workflow side effects: %w", err)
	}
	defer rows.Close()
	items := make([]workflow.SideEffectRecord, 0)
	for rows.Next() {
		var item workflow.SideEffectRecord
		var kind string
		var inputJSON, outputJSON sql.NullString
		var completedAt sql.NullTime
		var durationMS int64
		if err := rows.Scan(&item.JournalID, &item.ExecutionID, &item.WorkflowID, &item.NodeID, &item.Attempt, &item.Generation,
			&item.DeviceID, &kind, &item.Target, &item.IdempotencyKey, &inputJSON, &outputJSON, &item.Error, &item.Status,
			&durationMS, &item.Timestamp, &completedAt); err != nil {
			return nil, fmt.Errorf("scan workflow side effect: %w", err)
		}
		item.Kind = workflow.SideEffectKind(kind)
		item.Duration = time.Duration(durationMS) * time.Millisecond
		if inputJSON.Valid {
			item.Input = json.RawMessage(inputJSON.String)
		}
		if outputJSON.Valid {
			item.Output = json.RawMessage(outputJSON.String)
		}
		if completedAt.Valid {
			t := completedAt.Time
			item.CompletedAt = &t
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *WorkflowExecutionRepository) SaveCompensation(ctx context.Context, record workflow.CompensationRecord) error {
	if strings.TrimSpace(record.ExecutionID) == "" || strings.TrimSpace(record.WorkflowID) == "" || strings.TrimSpace(record.NodeID) == "" {
		return fmt.Errorf("save workflow compensation: executionId, workflowId and nodeId are required")
	}
	if record.StartedAt.IsZero() {
		record.StartedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_compensations
			(execution_id, workflow_id, node_id, generation, status, attempt, idempotency_key,
			 input_json, output_json, error_message, started_at, completed_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(execution_id, node_id) DO UPDATE SET
			generation = excluded.generation,
			status = excluded.status,
			attempt = excluded.attempt,
			idempotency_key = excluded.idempotency_key,
			input_json = excluded.input_json,
			output_json = excluded.output_json,
			error_message = excluded.error_message,
			started_at = CASE WHEN extension_workflow_compensations.started_at < excluded.started_at THEN extension_workflow_compensations.started_at ELSE excluded.started_at END,
			completed_at = excluded.completed_at,
			updated_at = excluded.updated_at
	`, record.ExecutionID, record.WorkflowID, record.NodeID, record.Generation, record.Status, record.Attempt,
		record.IdempotencyKey, nullableRawJSON(record.Input), nullableRawJSON(record.Output), record.Error,
		record.StartedAt, record.CompletedAt, record.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save workflow compensation: %w", err)
	}
	return nil
}

func (r *WorkflowExecutionRepository) GetCompensation(ctx context.Context, executionID, nodeID string) (*workflow.CompensationRecord, error) {
	var item workflow.CompensationRecord
	var inputJSON, outputJSON sql.NullString
	var completedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT execution_id, workflow_id, node_id, generation, status, attempt, idempotency_key,
			input_json, output_json, error_message, started_at, completed_at, updated_at
		FROM extension_workflow_compensations
		WHERE execution_id = ? AND node_id = ?
	`, strings.TrimSpace(executionID), strings.TrimSpace(nodeID)).Scan(
		&item.ExecutionID, &item.WorkflowID, &item.NodeID, &item.Generation, &item.Status, &item.Attempt,
		&item.IdempotencyKey, &inputJSON, &outputJSON, &item.Error, &item.StartedAt, &completedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if inputJSON.Valid {
		item.Input = json.RawMessage(inputJSON.String)
	}
	if outputJSON.Valid {
		item.Output = json.RawMessage(outputJSON.String)
	}
	if completedAt.Valid {
		t := completedAt.Time
		item.CompletedAt = &t
	}
	return &item, nil
}

func (r *WorkflowExecutionRepository) ListCompensations(ctx context.Context, executionID string) ([]workflow.CompensationRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT execution_id, workflow_id, node_id, generation, status, attempt, idempotency_key,
			input_json, output_json, error_message, started_at, completed_at, updated_at
		FROM extension_workflow_compensations
		WHERE execution_id = ?
		ORDER BY updated_at, node_id
	`, strings.TrimSpace(executionID))
	if err != nil {
		return nil, fmt.Errorf("list workflow compensations: %w", err)
	}
	defer rows.Close()
	items := make([]workflow.CompensationRecord, 0)
	for rows.Next() {
		var item workflow.CompensationRecord
		var inputJSON, outputJSON sql.NullString
		var completedAt sql.NullTime
		if err := rows.Scan(&item.ExecutionID, &item.WorkflowID, &item.NodeID, &item.Generation, &item.Status, &item.Attempt,
			&item.IdempotencyKey, &inputJSON, &outputJSON, &item.Error, &item.StartedAt, &completedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workflow compensation: %w", err)
		}
		if inputJSON.Valid {
			item.Input = json.RawMessage(inputJSON.String)
		}
		if outputJSON.Valid {
			item.Output = json.RawMessage(outputJSON.String)
		}
		if completedAt.Valid {
			t := completedAt.Time
			item.CompletedAt = &t
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *WorkflowExecutionRepository) AcquireExecutionLease(ctx context.Context, executionID, nodeID, ownerDeviceID string, generation int64, ttl time.Duration) (workflow.ExecutionLease, error) {
	executionID = strings.TrimSpace(executionID)
	nodeID = strings.TrimSpace(nodeID)
	ownerDeviceID = strings.TrimSpace(ownerDeviceID)
	if executionID == "" || nodeID == "" {
		return workflow.ExecutionLease{}, fmt.Errorf("acquire workflow execution lease: executionId and nodeId are required")
	}
	if ttl <= 0 {
		ttl = workflow.DefaultExecutionLeaseTTL
	}
	now := time.Now().UTC()
	expires := now.Add(ttl)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return workflow.ExecutionLease{}, fmt.Errorf("begin workflow execution lease: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current workflow.ExecutionLease
	err = tx.QueryRowContext(ctx, `
		SELECT execution_id, node_id, owner_device_id, generation, fencing_token, lease_expires_at, heartbeat_at
		FROM extension_workflow_execution_leases WHERE execution_id = ? AND node_id = ?
	`, executionID, nodeID).Scan(&current.ExecutionID, &current.NodeID, &current.OwnerDeviceID, &current.Generation, &current.FencingToken, &current.LeaseExpiresAt, &current.HeartbeatAt)
	if errors.Is(err, sql.ErrNoRows) {
		lease := workflow.ExecutionLease{
			ExecutionID: executionID, NodeID: nodeID, OwnerDeviceID: ownerDeviceID,
			Generation: generation, FencingToken: 1, LeaseExpiresAt: expires, HeartbeatAt: now,
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO extension_workflow_execution_leases
				(execution_id, node_id, owner_device_id, generation, fencing_token, lease_expires_at, heartbeat_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, lease.ExecutionID, lease.NodeID, lease.OwnerDeviceID, lease.Generation, lease.FencingToken, lease.LeaseExpiresAt, lease.HeartbeatAt, now); err != nil {
			return workflow.ExecutionLease{}, fmt.Errorf("create workflow execution lease: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return workflow.ExecutionLease{}, fmt.Errorf("commit workflow execution lease: %w", err)
		}
		return lease, nil
	}
	if err != nil {
		return workflow.ExecutionLease{}, fmt.Errorf("read workflow execution lease: %w", err)
	}
	if current.LeaseExpiresAt.After(now) {
		return workflow.ExecutionLease{}, &workflow.ExecutionLeaseBusyError{
			ExecutionID: current.ExecutionID, NodeID: current.NodeID, OwnerDeviceID: current.OwnerDeviceID, ExpiresAt: current.LeaseExpiresAt,
		}
	}
	if generation < current.Generation {
		return workflow.ExecutionLease{}, &workflow.StaleFencingTokenError{
			ExecutionID: executionID, NodeID: nodeID, Expected: current.FencingToken, Received: current.FencingToken - 1,
		}
	}
	next := workflow.ExecutionLease{
		ExecutionID: executionID, NodeID: nodeID, OwnerDeviceID: ownerDeviceID, Generation: generation,
		FencingToken: current.FencingToken + 1, LeaseExpiresAt: expires, HeartbeatAt: now,
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE extension_workflow_execution_leases
		SET owner_device_id = ?, generation = ?, fencing_token = ?, lease_expires_at = ?, heartbeat_at = ?, updated_at = ?
		WHERE execution_id = ? AND node_id = ? AND fencing_token = ? AND lease_expires_at <= ?
	`, next.OwnerDeviceID, next.Generation, next.FencingToken, next.LeaseExpiresAt, next.HeartbeatAt, now,
		executionID, nodeID, current.FencingToken, now)
	if err != nil {
		return workflow.ExecutionLease{}, fmt.Errorf("take over workflow execution lease: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return workflow.ExecutionLease{}, err
	}
	if rows != 1 {
		return workflow.ExecutionLease{}, &workflow.ExecutionLeaseBusyError{ExecutionID: executionID, NodeID: nodeID, OwnerDeviceID: current.OwnerDeviceID, ExpiresAt: current.LeaseExpiresAt}
	}
	if err := tx.Commit(); err != nil {
		return workflow.ExecutionLease{}, fmt.Errorf("commit workflow execution lease takeover: %w", err)
	}
	return next, nil
}

func (r *WorkflowExecutionRepository) RenewExecutionLease(ctx context.Context, lease workflow.ExecutionLease, ttl time.Duration) (workflow.ExecutionLease, error) {
	if ttl <= 0 {
		ttl = workflow.DefaultExecutionLeaseTTL
	}
	now := time.Now().UTC()
	lease.HeartbeatAt = now
	lease.LeaseExpiresAt = now.Add(ttl)
	res, err := r.db.ExecContext(ctx, `
		UPDATE extension_workflow_execution_leases
		SET lease_expires_at = ?, heartbeat_at = ?, updated_at = ?
		WHERE execution_id = ? AND node_id = ? AND fencing_token = ? AND generation = ?
	`, lease.LeaseExpiresAt, lease.HeartbeatAt, now, lease.ExecutionID, lease.NodeID, lease.FencingToken, lease.Generation)
	if err != nil {
		return workflow.ExecutionLease{}, fmt.Errorf("renew workflow execution lease: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return workflow.ExecutionLease{}, err
	}
	if rows != 1 {
		return workflow.ExecutionLease{}, &workflow.StaleFencingTokenError{ExecutionID: lease.ExecutionID, NodeID: lease.NodeID, Received: lease.FencingToken}
	}
	return lease, nil
}

func (r *WorkflowExecutionRepository) ReleaseExecutionLease(ctx context.Context, lease workflow.ExecutionLease) error {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE extension_workflow_execution_leases
		SET lease_expires_at = ?, heartbeat_at = ?, updated_at = ?
		WHERE execution_id = ? AND node_id = ? AND fencing_token = ? AND generation = ?
	`, now, now, now, lease.ExecutionID, lease.NodeID, lease.FencingToken, lease.Generation)
	if err != nil {
		return fmt.Errorf("release workflow execution lease: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return &workflow.StaleFencingTokenError{ExecutionID: lease.ExecutionID, NodeID: lease.NodeID, Received: lease.FencingToken}
	}
	return nil
}

func (r *WorkflowExecutionRepository) ValidateExecutionFence(ctx context.Context, executionID, nodeID string, fencingToken int64) error {
	var current int64
	if err := r.db.QueryRowContext(ctx, `SELECT fencing_token FROM extension_workflow_execution_leases WHERE execution_id = ? AND node_id = ?`, strings.TrimSpace(executionID), strings.TrimSpace(nodeID)).Scan(&current); err != nil {
		return err
	}
	if current != fencingToken {
		return &workflow.StaleFencingTokenError{ExecutionID: executionID, NodeID: nodeID, Expected: current, Received: fencingToken}
	}
	return nil
}

func nullableRawJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
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
		SELECT execution_id, workflow_id, node_id, status, trace_id, attempt_id, device_id, runtime_id, tool_call_id, fencing_token, idempotency_key, input_json, output_json,
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
		if err := rows.Scan(&step.ExecutionID, &step.WorkflowID, &step.NodeID, &step.Status, &step.TraceID, &step.AttemptID, &step.DeviceID, &step.RuntimeID, &step.ToolCallID, &step.FencingToken, &step.IdempotencyKey, &inputJSON, &outputJSON, &errorMessage, &step.Attempt, &step.StartedAt, &finishedAt); err != nil {
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

func (r *WorkflowExecutionRepository) HeartbeatRun(ctx context.Context, executionID string, at time.Time) error {
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return fmt.Errorf("workflow heartbeat execution id is required")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	} else {
		at = at.UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_run_heartbeats (execution_id, heartbeat_at, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(execution_id) DO UPDATE SET
			heartbeat_at = excluded.heartbeat_at,
			updated_at = excluded.updated_at
	`, executionID, at, at)
	if err != nil {
		return fmt.Errorf("heartbeat workflow execution: %w", err)
	}
	return nil
}

func (r *WorkflowExecutionRepository) ListStuckRuns(ctx context.Context, heartbeatBefore time.Time, limit int) ([]workflow.WorkflowRun, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if heartbeatBefore.IsZero() {
		heartbeatBefore = time.Now().UTC().Add(-5 * time.Minute)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.execution_id, e.workflow_id, e.status, e.input_json, e.output_json, e.error_message,
			e.context_json, e.steps_json, e.compensation_json, e.attempt, e.generation,
			e.pause_reason, e.pause_requested_at, e.paused_at,
			e.started_at, e.finished_at, e.updated_at
		FROM extension_workflow_executions e
		LEFT JOIN extension_workflow_run_heartbeats h ON h.execution_id = e.execution_id
		WHERE e.status IN (?, ?, ?, ?, ?, ?, ?)
		  AND COALESCE(h.heartbeat_at, e.updated_at) < ?
		ORDER BY COALESCE(h.heartbeat_at, e.updated_at) ASC
		LIMIT ?
	`, workflow.RunStatusRunning, workflow.RunStatusResuming, workflow.RunStatusCompensating,
		workflow.RunStatusPausing, workflow.RunStatusWaitingDevice, workflow.RunStatusCancelRequested,
		workflow.RunStatusCancelling, heartbeatBefore.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("list stuck workflow executions: %w", err)
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

func (r *WorkflowExecutionRepository) ListActiveWorkflowRuns(ctx context.Context, workflowID string, limit int) ([]workflow.WorkflowRun, error) {
	workflowID = strings.TrimSpace(workflowID)
	if workflowID == "" {
		return nil, fmt.Errorf("workflow id is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT execution_id, workflow_id, status, input_json, output_json, error_message,
			context_json, steps_json, compensation_json, attempt, generation,
			pause_reason, pause_requested_at, paused_at,
			started_at, finished_at, updated_at
		FROM extension_workflow_executions
		WHERE workflow_id = ? AND status IN (?, ?, ?, ?, ?, ?, ?, ?)
		ORDER BY started_at ASC
		LIMIT ?
	`, workflowID, workflow.RunStatusRunning, workflow.RunStatusPausing, workflow.RunStatusPaused,
		workflow.RunStatusResuming, workflow.RunStatusWaitingDevice, workflow.RunStatusCompensating,
		workflow.RunStatusCancelRequested, workflow.RunStatusCancelling, limit)
	if err != nil {
		return nil, fmt.Errorf("list active workflow runs: %w", err)
	}
	defer rows.Close()
	items := make([]workflow.WorkflowRun, 0)
	for rows.Next() {
		run, scanErr := r.scanRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *run)
	}
	return items, rows.Err()
}

func (r *WorkflowExecutionRepository) EnqueueWorkflowRun(ctx context.Context, run workflow.WorkflowRun) error {
	if strings.TrimSpace(run.ExecutionID) == "" || strings.TrimSpace(run.WorkflowID) == "" {
		return fmt.Errorf("queued workflow requires execution and workflow id")
	}
	contextJSON, err := json.Marshal(run.Context)
	if err != nil {
		return fmt.Errorf("marshal queued workflow context: %w", err)
	}
	now := time.Now().UTC()
	if run.StartedAt.IsZero() {
		run.StartedAt = now
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_executions
			(execution_id, workflow_id, status, input_json, output_json, error_message,
			 extension_id, module_id, character_id, conversation_id, operation_id, invocation_id,
			 schedule_id, trigger_id, trace_id, idempotency_key, scope_snapshot_id,
			 permission_snapshot_id, generation, context_json, attempt, steps_json,
			 compensation_json, pause_reason, pause_requested_at, paused_at,
			 started_at, duration_ms, created_at, updated_at)
		VALUES (?, ?, ?, ?, NULL, '', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '[]', '[]', '', NULL, NULL, ?, 0, ?, ?)
	`, run.ExecutionID, run.WorkflowID, workflow.RunStatusQueued, run.Input,
		run.Context.ExtensionID, run.Context.ModuleID, run.Context.CharacterID, run.Context.ConversationID,
		run.Context.OperationID, run.Context.InvocationID, run.Context.ScheduleID, run.Context.TriggerID,
		run.Context.TraceID, run.Context.IdempotencyKey, run.Context.ScopeSnapshotID, run.Context.PermissionSnapID,
		run.Context.Generation, contextJSON, run.Attempt, run.StartedAt, now, now)
	if err != nil {
		if run.Context.IdempotencyKey != "" {
			if existing, getErr := r.getByIdempotency(ctx, run.WorkflowID, run.Context.IdempotencyKey); getErr == nil && existing != nil {
				return nil
			}
		}
		return fmt.Errorf("enqueue workflow execution: %w", err)
	}
	return nil
}

func (r *WorkflowExecutionRepository) ListQueuedWorkflowRuns(ctx context.Context, workflowID string, limit int) ([]workflow.WorkflowRun, error) {
	workflowID = strings.TrimSpace(workflowID)
	if workflowID == "" {
		return nil, fmt.Errorf("workflow id is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT execution_id, workflow_id, status, input_json, output_json, error_message,
			context_json, steps_json, compensation_json, attempt, generation,
			pause_reason, pause_requested_at, paused_at,
			started_at, finished_at, updated_at
		FROM extension_workflow_executions
		WHERE workflow_id = ? AND status = ?
		ORDER BY created_at ASC, execution_id ASC
		LIMIT ?
	`, workflowID, workflow.RunStatusQueued, limit)
	if err != nil {
		return nil, fmt.Errorf("list queued workflow runs: %w", err)
	}
	defer rows.Close()
	items := make([]workflow.WorkflowRun, 0)
	for rows.Next() {
		run, scanErr := r.scanRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *run)
	}
	return items, rows.Err()
}

func (r *WorkflowExecutionRepository) DropQueuedWorkflowRuns(ctx context.Context, workflowID, reason string, at time.Time) (int, error) {
	workflowID = strings.TrimSpace(workflowID)
	if workflowID == "" {
		return 0, fmt.Errorf("workflow id is required")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if strings.TrimSpace(reason) == "" {
		reason = "dropped by workflow concurrency policy"
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE extension_workflow_executions
		SET status = ?, error_message = ?, finished_at = ?, updated_at = ?
		WHERE workflow_id = ? AND status = ?
	`, workflow.RunStatusDropped, reason, at.UTC(), at.UTC(), workflowID, workflow.RunStatusQueued)
	if err != nil {
		return 0, fmt.Errorf("drop queued workflow runs: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
