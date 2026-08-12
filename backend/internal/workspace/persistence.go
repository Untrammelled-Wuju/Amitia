package workspace

import (
	"time"

	"gorm.io/gorm"
)

type MountRepository struct {
	db *gorm.DB
}

func NewMountRepository(db *gorm.DB) *MountRepository {
	return &MountRepository{db: db}
}

func (r *MountRepository) LoadAll() ([]persistenceRecord, error) {
	var rows []struct {
		ID            string `gorm:"column:id"`
		Name          string `gorm:"column:name"`
		Kind          string `gorm:"column:kind"`
		LocalRoot     string `gorm:"column:local_root"`
		GrantID       string `gorm:"column:native_grant_id"`
		BackendConfig string `gorm:"column:backend_config_json"`
		CredentialRef string `gorm:"column:credential_ref"`
		ReadOnly      int    `gorm:"column:read_only"`
		Enabled       int    `gorm:"column:enabled"`
		CreatedAt     string `gorm:"column:created_at"`
		UpdatedAt     string `gorm:"column:updated_at"`
	}
	query := "SELECT id, name, kind, local_root, native_grant_id, COALESCE(backend_config_json, '') as backend_config_json, COALESCE(credential_ref, '') as credential_ref, read_only, enabled, created_at, updated_at FROM workspace_mounts WHERE enabled = 1"
	if err := r.db.Raw(query).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]persistenceRecord, 0, len(rows))
	for _, row := range rows {
		createdAt, _ := time.Parse(time.RFC3339, row.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, row.UpdatedAt)
		result = append(result, persistenceRecord{
			id:            row.ID,
			name:          row.Name,
			kind:          WorkspaceKind(row.Kind),
			localRoot:     row.LocalRoot,
			nativeGrant:   row.GrantID,
			readOnly:      row.ReadOnly != 0,
			enabled:       row.Enabled != 0,
			createdAt:     createdAt,
			updatedAt:     updatedAt,
			backendConfig: row.BackendConfig,
			credentialRef: row.CredentialRef,
		})
	}
	return result, nil
}

func (r *MountRepository) Insert(rec persistenceRecord) error {
	return r.db.Exec(
		"INSERT INTO workspace_mounts (id, name, kind, local_root, native_grant_id, backend_config_json, credential_ref, read_only, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		rec.id,
		rec.name,
		string(rec.kind),
		rec.localRoot,
		rec.nativeGrant,
		rec.backendConfig,
		rec.credentialRef,
		boolToInt(rec.readOnly),
		boolToInt(rec.enabled),
		rec.createdAt.Format(time.RFC3339),
		rec.updatedAt.Format(time.RFC3339),
	).Error
}

func (r *MountRepository) Delete(id string) error {
	return r.db.Exec("DELETE FROM workspace_mounts WHERE id = ?", id).Error
}

func (r *MountRepository) UpdateGrant(id string, grantID string, readOnly bool, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Exec(
		"UPDATE workspace_mounts SET native_grant_id = ?, read_only = ?, updated_at = ? WHERE id = ?",
		grantID, boolToInt(readOnly), now, id,
	).Error
}

func (r *MountRepository) UpdateConfig(id string, configJSON string, credentialRef string, readOnly bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Exec(
		"UPDATE workspace_mounts SET backend_config_json = ?, credential_ref = ?, read_only = ?, updated_at = ? WHERE id = ?",
		configJSON, credentialRef, boolToInt(readOnly), now, id,
	).Error
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
