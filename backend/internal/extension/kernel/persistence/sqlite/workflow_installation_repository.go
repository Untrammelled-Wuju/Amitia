package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

var ErrWorkflowRevisionConflict = errors.New("workflow installation revision conflict")

type WorkflowInstallationRepository struct {
	db *sql.DB
}

func NewWorkflowInstallationRepository(db *sql.DB) *WorkflowInstallationRepository {
	return &WorkflowInstallationRepository{db: db}
}

func normalizeInstallation(inst workflow.WorkflowInstallation) (workflow.WorkflowInstallation, error) {
	inst.InstallationID = strings.TrimSpace(inst.InstallationID)
	inst.WorkflowID = strings.TrimSpace(inst.WorkflowID)
	inst.OwnerUserID = strings.TrimSpace(inst.OwnerUserID)
	inst.HostDeviceID = strings.TrimSpace(inst.HostDeviceID)
	if inst.InstallationID == "" {
		inst.InstallationID = "wfinst-" + uuid.NewString()
	}
	if inst.Revision <= 0 {
		inst.Revision = 1
	}
	// Device is an API/control-plane target, not a third installation location.
	// Cloud installations never have a host device; local installations are
	// stored by the device-local Core and currently do not require the optional
	// hostDeviceId metadata to resolve them.
	if inst.Location == workflow.WorkflowLocationCloud {
		inst.HostDeviceID = ""
	}
	if err := inst.Validate(); err != nil {
		return workflow.WorkflowInstallation{}, err
	}
	return inst, nil
}

func (r *WorkflowInstallationRepository) Create(ctx context.Context, inst workflow.WorkflowInstallation) (*workflow.WorkflowInstallation, error) {
	inst, err := normalizeInstallation(inst)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if inst.CreatedAt.IsZero() {
		inst.CreatedAt = now
	}
	inst.UpdatedAt = now
	triggers, err := json.Marshal(inst.Triggers)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow installation triggers: %w", err)
	}
	agentTool, err := json.Marshal(inst.AgentTool)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow installation agent tool: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_installations
			(installation_id, workflow_id, owner_user_id, location, host_device_id, enabled,
			 triggers_json, callable_by_agent, agent_tool_json, revision, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, inst.InstallationID, inst.WorkflowID, inst.OwnerUserID, string(inst.Location), inst.HostDeviceID,
		boolToInt(inst.Enabled), triggers, boolToInt(inst.CallableByAgent), agentTool, inst.Revision, inst.CreatedAt, inst.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create workflow installation: %w", err)
	}
	return &inst, nil
}

func (r *WorkflowInstallationRepository) EnsureLegacy(ctx context.Context, def workflow.WorkflowDefinition, ownerUserID string, location workflow.WorkflowLocation) (*workflow.WorkflowInstallation, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" || strings.TrimSpace(def.ID) == "" {
		return nil, errors.New("legacy workflow installation requires owner and workflow id")
	}
	if current, err := r.Get(ctx, ownerUserID, def.ID, location, ""); err == nil {
		return current, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return r.Create(ctx, workflow.WorkflowInstallation{
		WorkflowID:      def.ID,
		OwnerUserID:     ownerUserID,
		Location:        location,
		Enabled:         def.Enabled,
		Triggers:        def.Triggers,
		CallableByAgent: def.CallableByAgent,
		AgentTool:       def.AgentTool,
	})
}

func (r *WorkflowInstallationRepository) Get(ctx context.Context, ownerUserID, workflowID string, location workflow.WorkflowLocation, hostDeviceID string) (*workflow.WorkflowInstallation, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT installation_id, workflow_id, owner_user_id, location, host_device_id, enabled,
			triggers_json, callable_by_agent, agent_tool_json, revision, created_at, updated_at
		FROM extension_workflow_installations
		WHERE owner_user_id = ? AND workflow_id = ? AND location = ? AND host_device_id = ?
	`, strings.TrimSpace(ownerUserID), strings.TrimSpace(workflowID), string(location), strings.TrimSpace(hostDeviceID))
	return scanWorkflowInstallation(row)
}

func (r *WorkflowInstallationRepository) GetByID(ctx context.Context, ownerUserID, installationID string) (*workflow.WorkflowInstallation, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT installation_id, workflow_id, owner_user_id, location, host_device_id, enabled,
			triggers_json, callable_by_agent, agent_tool_json, revision, created_at, updated_at
		FROM extension_workflow_installations
		WHERE owner_user_id = ? AND installation_id = ?
	`, strings.TrimSpace(ownerUserID), strings.TrimSpace(installationID))
	return scanWorkflowInstallation(row)
}

func (r *WorkflowInstallationRepository) List(ctx context.Context, ownerUserID string, location workflow.WorkflowLocation, hostDeviceID string) ([]workflow.WorkflowInstallation, error) {
	query := `
		SELECT installation_id, workflow_id, owner_user_id, location, host_device_id, enabled,
			triggers_json, callable_by_agent, agent_tool_json, revision, created_at, updated_at
		FROM extension_workflow_installations WHERE owner_user_id = ?`
	args := []any{strings.TrimSpace(ownerUserID)}
	if location != "" {
		query += ` AND location = ?`
		args = append(args, string(location))
	}
	if strings.TrimSpace(hostDeviceID) != "" {
		query += ` AND host_device_id = ?`
		args = append(args, strings.TrimSpace(hostDeviceID))
	}
	query += ` ORDER BY updated_at DESC, workflow_id ASC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list workflow installations: %w", err)
	}
	defer rows.Close()
	items := make([]workflow.WorkflowInstallation, 0)
	for rows.Next() {
		inst, err := scanWorkflowInstallation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *inst)
	}
	return items, rows.Err()
}

func (r *WorkflowInstallationRepository) UpdateCAS(ctx context.Context, inst workflow.WorkflowInstallation, expectedRevision int64) (*workflow.WorkflowInstallation, error) {
	inst, err := normalizeInstallation(inst)
	if err != nil {
		return nil, err
	}
	if expectedRevision <= 0 {
		return nil, errors.New("expectedRevision must be greater than zero")
	}
	triggers, err := json.Marshal(inst.Triggers)
	if err != nil {
		return nil, err
	}
	agentTool, err := json.Marshal(inst.AgentTool)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE extension_workflow_installations
		SET enabled = ?, triggers_json = ?, callable_by_agent = ?, agent_tool_json = ?,
			revision = revision + 1, updated_at = ?
		WHERE installation_id = ? AND owner_user_id = ? AND revision = ?
	`, boolToInt(inst.Enabled), triggers, boolToInt(inst.CallableByAgent), agentTool, now,
		inst.InstallationID, inst.OwnerUserID, expectedRevision)
	if err != nil {
		return nil, fmt.Errorf("update workflow installation: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrWorkflowRevisionConflict
	}
	return r.GetByID(ctx, inst.OwnerUserID, inst.InstallationID)
}

func (r *WorkflowInstallationRepository) Delete(ctx context.Context, ownerUserID, installationID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM extension_workflow_installations WHERE owner_user_id = ? AND installation_id = ?`, strings.TrimSpace(ownerUserID), strings.TrimSpace(installationID))
	if err != nil {
		return fmt.Errorf("delete workflow installation: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type workflowInstallationScanner interface {
	Scan(dest ...any) error
}

func scanWorkflowInstallation(row workflowInstallationScanner) (*workflow.WorkflowInstallation, error) {
	var inst workflow.WorkflowInstallation
	var location string
	var enabled, callable int
	var triggersRaw, agentToolRaw []byte
	if err := row.Scan(&inst.InstallationID, &inst.WorkflowID, &inst.OwnerUserID, &location, &inst.HostDeviceID, &enabled,
		&triggersRaw, &callable, &agentToolRaw, &inst.Revision, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
		return nil, err
	}
	inst.Location = workflow.WorkflowLocation(location)
	inst.Enabled = enabled != 0
	inst.CallableByAgent = callable != 0
	if len(triggersRaw) > 0 {
		if err := json.Unmarshal(triggersRaw, &inst.Triggers); err != nil {
			return nil, fmt.Errorf("decode workflow installation triggers: %w", err)
		}
	}
	if len(agentToolRaw) > 0 {
		if err := json.Unmarshal(agentToolRaw, &inst.AgentTool); err != nil {
			return nil, fmt.Errorf("decode workflow installation agent tool: %w", err)
		}
	}
	return &inst, nil
}

type WorkflowDeviceCatalogItem struct {
	OwnerUserID  string          `json:"ownerUserId,omitempty"`
	DeviceID     string          `json:"deviceId"`
	WorkflowID   string          `json:"workflowId"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"inputSchema,omitempty"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
	Version      string          `json:"version,omitempty"`
	Enabled      bool            `json:"enabled"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	LastSeen     time.Time       `json:"lastSeen"`
}

type WorkflowDeviceCatalogRepository struct{ db *sql.DB }

func NewWorkflowDeviceCatalogRepository(db *sql.DB) *WorkflowDeviceCatalogRepository {
	return &WorkflowDeviceCatalogRepository{db: db}
}

func (r *WorkflowDeviceCatalogRepository) ReplaceDevice(ctx context.Context, ownerUserID, deviceID string, items []WorkflowDeviceCatalogItem) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM extension_workflow_device_catalog WHERE owner_user_id = ? AND device_id = ?`, ownerUserID, deviceID); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO extension_workflow_device_catalog
			(owner_user_id, device_id, workflow_id, name, description, input_schema_json, output_schema_json, version, enabled, updated_at, last_seen)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, ownerUserID, deviceID, item.WorkflowID, item.Name, item.Description, item.InputSchema, item.OutputSchema, item.Version, boolToInt(item.Enabled), item.UpdatedAt, item.LastSeen); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *WorkflowDeviceCatalogRepository) ListDevice(ctx context.Context, ownerUserID, deviceID string) ([]WorkflowDeviceCatalogItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT owner_user_id, device_id, workflow_id, name, description, input_schema_json, output_schema_json, version, enabled, updated_at, last_seen
		FROM extension_workflow_device_catalog WHERE owner_user_id = ? AND device_id = ? ORDER BY name, workflow_id
	`, ownerUserID, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]WorkflowDeviceCatalogItem, 0)
	for rows.Next() {
		var item WorkflowDeviceCatalogItem
		var enabled int
		if err := rows.Scan(&item.OwnerUserID, &item.DeviceID, &item.WorkflowID, &item.Name, &item.Description, &item.InputSchema, &item.OutputSchema, &item.Version, &enabled, &item.UpdatedAt, &item.LastSeen); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		items = append(items, item)
	}
	return items, rows.Err()
}
