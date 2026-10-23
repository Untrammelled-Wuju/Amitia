// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worldbook

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/u-ai/backend/internal/system/dataportability"
)

const ComponentIDWorldbookRecords = "worldbook.records"

type WorldbookBackupContributor struct {
	KernelDB *sql.DB
}

func (w *WorldbookBackupContributor) ComponentIDs() []string {
	return []string{ComponentIDWorldbookRecords}
}

func NewWorldbookBackupContributor(kernelDB *sql.DB) *WorldbookBackupContributor {
	return &WorldbookBackupContributor{KernelDB: kernelDB}
}

func (w *WorldbookBackupContributor) ID() string   { return "worldbook" }
func (w *WorldbookBackupContributor) Name() string { return "Worldbook" }

func (w *WorldbookBackupContributor) Dependencies() []string {
	return nil
}

type ownedWorldbookRecordV1 struct {
	OwnerID   string          `json:"ownerId"`
	RoleID    string          `json:"roleId"`
	ID        string          `json:"id"`
	Revision  int64           `json:"revision"`
	Body      json.RawMessage `json:"body"`
	UpdatedAt string          `json:"updatedAt"`
}

func (w *WorldbookBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeCharacter {
		return nil, nil
	}
	if w == nil || w.KernelDB == nil {
		return nil, errors.New("世界书备份数据库不可用")
	}
	var count int64
	query := `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE kind='worldbook' AND deleted=0`
	args := make([]any, 0, 1)
	if req.EffectiveScope() == dataportability.ScopeCharacter && req.CharacterID != "" {
		query += ` AND role_id=?`
		args = append(args, req.CharacterID)
	}
	if err := w.KernelDB.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return nil, err
	}
	return []dataportability.BackupComponentPlan{{
		ID:            ComponentIDWorldbookRecords,
		Kind:          dataportability.KindNDJSON,
		LogicalName:   "worldbook.owned.v1",
		Required:      false,
		SourceOfTruth: true,
		ItemCount:     count,
		EstimatedSize: count * 4096,
	}}, nil
}

func (w *WorldbookBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeCharacter {
		return nil
	}
	compW, err := out.CreateComponent(ComponentIDWorldbookRecords, "worldbook.owned.v1", dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()
	query := `SELECT owner_id,role_id,resource_id,revision,body,updated_at FROM kernel_device_owned_resources WHERE kind='worldbook' AND deleted=0`
	args := make([]any, 0, 1)
	if req.EffectiveScope() == dataportability.ScopeCharacter && req.CharacterID != "" {
		query += ` AND role_id=?`
		args = append(args, req.CharacterID)
	}
	query += ` ORDER BY owner_id,role_id,resource_id`
	rows, err := w.KernelDB.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	writer := bufio.NewWriter(compW)
	for rows.Next() {
		var record ownedWorldbookRecordV1
		var body []byte
		if err := rows.Scan(&record.OwnerID, &record.RoleID, &record.ID, &record.Revision, &body, &record.UpdatedAt); err != nil {
			return err
		}
		record.Body = body
		encoded, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := writer.Write(append(encoded, '\n')); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writer.Flush()
}

func (w *WorldbookBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	rc, err := in.ReadComponent(ComponentIDWorldbookRecords)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	preview := dataportability.ImportComponentPreview{ComponentID: ComponentIDWorldbookRecords, Kind: dataportability.KindNDJSON, LogicalName: "worldbook.owned.v1"}
	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var record ownedWorldbookRecordV1
		if json.Unmarshal(scanner.Bytes(), &record) != nil || record.OwnerID == "" || record.RoleID == "" || record.ID == "" || record.Revision < 1 || !json.Valid(record.Body) {
			continue
		}
		preview.ItemCount++
		var existing int
		if err := w.KernelDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='worldbook' AND resource_id=?`, record.OwnerID, record.ID).Scan(&existing); err != nil {
			return nil, err
		}
		if existing > 0 {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{SourceID: record.OwnerID + "/" + record.ID, TargetID: record.ID, EntityType: "worldbook_rule", Policy: dataportability.CollisionDuplicate})
		}
	}
	return []dataportability.ImportComponentPreview{preview}, scanner.Err()
}

func (w *WorldbookBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	opts := dataportability.RestoreOptions{
		OperationID:        req.OperationID,
		Purpose:            dataportability.RestorePurposeOrdinary,
		CharacterPolicy:    req.CharacterPolicy,
		DefaultCharacterID: req.DefaultCharacterID,
		ActivateImported:   req.ActivateImported,
		IdentityMap:        req.IdentityMap,
		SecretProvider:     req.SecretProvider,
	}
	return w.RestoreWorldbook(ctx, in, opts)
}

func (w *WorldbookBackupContributor) RestoreWorldbook(ctx context.Context, in dataportability.BackupReader, opts dataportability.RestoreOptions) error {
	rc, err := in.ReadComponent(ComponentIDWorldbookRecords)
	if err != nil {
		return err
	}
	defer rc.Close()
	tx, err := w.KernelDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var record ownedWorldbookRecordV1
		if json.Unmarshal(scanner.Bytes(), &record) != nil || record.OwnerID == "" || record.RoleID == "" || record.ID == "" || record.Revision < 1 || !json.Valid(record.Body) {
			return errors.New("世界书备份记录无效")
		}
		clause := ` ON CONFLICT(owner_id,kind,resource_id) DO NOTHING`
		if opts.CharacterPolicy == dataportability.CollisionReplace {
			clause = ` ON CONFLICT(owner_id,kind,resource_id) DO UPDATE SET role_id=excluded.role_id,revision=excluded.revision,deleted=0,body=excluded.body,updated_at=excluded.updated_at`
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,source_id,revision,deleted,body,updated_at) VALUES(?,'worldbook',?,?,'',?,0,?,?)`+clause, record.OwnerID, record.ID, record.RoleID, record.Revision, record.Body, record.UpdatedAt); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return tx.Commit()
}
