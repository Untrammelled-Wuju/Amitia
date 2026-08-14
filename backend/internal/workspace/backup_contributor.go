// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package workspace

import (
	"bufio"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

const ComponentIDWorkspaces = "workspace.records"

type WorkspaceBackupContributor struct {
	DB *gorm.DB
}

func NewWorkspaceBackupContributor(db *gorm.DB) *WorkspaceBackupContributor {
	return &WorkspaceBackupContributor{DB: db}
}

func (c *WorkspaceBackupContributor) ID() string   { return "workspace" }
func (c *WorkspaceBackupContributor) Name() string { return "Workspace" }

func (c *WorkspaceBackupContributor) Dependencies() []string {
	return nil
}

type workspaceExportRecord struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	BackendConfig string `json:"backendConfig"`
	ReadOnly      bool   `json:"readOnly"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

func (c *WorkspaceBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var count int64
	c.DB.WithContext(ctx).Table("workspace_mounts").Count(&count)
	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDWorkspaces,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "workspace.records.v1",
			Required:      false,
			SourceOfTruth: false,
			Rebuildable:   true,
			Sensitive:     false,
			ItemCount:     count,
			EstimatedSize: count * 512,
		},
	}, nil
}

func (c *WorkspaceBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	compW, err := out.CreateComponent(ComponentIDWorkspaces, "workspace.records.v1", dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()

	rows, err := c.DB.WithContext(ctx).Table("workspace_mounts").Select(
		"id, name, kind, COALESCE(backend_config_json, '') as backend_config_json, COALESCE(credential_ref, '') as credential_ref, read_only, enabled, created_at, updated_at",
	).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	bufW := bufio.NewWriter(compW)
	for rows.Next() {
		var id, name, kind, backendConfig, credentialRef string
		var readOnly, enabled int
		var createdAt, updatedAt string
		if err := rows.Scan(&id, &name, &kind, &backendConfig, &credentialRef, &readOnly, &enabled, &createdAt, &updatedAt); err != nil {
			continue
		}
		rec := workspaceExportRecord{
			ID:            id,
			Name:          name,
			Kind:          kind,
			BackendConfig: backendConfig,
			ReadOnly:      readOnly != 0,
			Enabled:       enabled != 0,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		bufW.Write(data)
		bufW.WriteByte('\n')
	}
	bufW.Flush()
	return rows.Err()
}

func (c *WorkspaceBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	rc, err := in.ReadComponent(ComponentIDWorkspaces)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	preview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDWorkspaces,
		Kind:        dataportability.KindNDJSON,
		LogicalName: "workspace.records.v1",
	}

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if line[0] == '\n' || (len(line) == 1 && line[0] == '\r') {
			continue
		}
		var rec workspaceExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.ID == "" {
			continue
		}
		preview.ItemCount++

		var existing struct{ ID string }
		c.DB.WithContext(ctx).Table("workspace_mounts").Select("id").Where("id = ?", rec.ID).Scan(&existing)
		if existing.ID != "" {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
				SourceID:   rec.ID,
				TargetID:   existing.ID,
				EntityType: "workspace",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}

	return []dataportability.ImportComponentPreview{preview}, scanner.Err()
}

func (c *WorkspaceBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	rc, err := in.ReadComponent(ComponentIDWorkspaces)
	if err != nil {
		return err
	}
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if line[0] == '\n' || (len(line) == 1 && line[0] == '\r') {
			continue
		}
		var rec workspaceExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.ID == "" {
			continue
		}

		newID := rec.ID
		var existing struct{ ID string }
		c.DB.WithContext(ctx).Table("workspace_mounts").Select("id").Where("id = ?", rec.ID).Scan(&existing)
		if existing.ID != "" {
			switch req.CharacterPolicy {
			case dataportability.CollisionSkip:
				continue
			case dataportability.CollisionReplace:
				newID = rec.ID
			default:
				newID = uuid.New().String()
			}
		}

		updates := map[string]interface{}{
			"name":                rec.Name,
			"kind":                rec.Kind,
			"backend_config_json": rec.BackendConfig,
			"read_only":           rec.ReadOnly,
			"enabled":             rec.Enabled,
			"created_at":          rec.CreatedAt,
			"updated_at":          rec.UpdatedAt,
			"local_root":          "",
			"native_grant_id":     "",
			"credential_ref":      "",
		}

		if newID == rec.ID && existing.ID != "" {
			c.DB.WithContext(ctx).Table("workspace_mounts").Where("id = ?", rec.ID).Updates(updates)
		} else {
			updates["id"] = newID
			c.DB.WithContext(ctx).Table("workspace_mounts").Create(updates)
		}
	}

	return scanner.Err()
}
