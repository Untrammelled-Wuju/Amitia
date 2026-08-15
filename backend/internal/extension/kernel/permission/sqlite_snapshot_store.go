package permission

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type SQLitePermissionSnapshotStore struct {
	db *sql.DB
}

func NewSQLitePermissionSnapshotStore(db *sql.DB) *SQLitePermissionSnapshotStore {
	return &SQLitePermissionSnapshotStore{db: db}
}

func marshalStringList(list []string) string {
	if len(list) == 0 {
		return "[]"
	}
	raw, _ := json.Marshal(list)
	return string(raw)
}

func unmarshalStringList(raw string) []string {
	if raw == "" {
		return nil
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil
	}
	return list
}

func (s *SQLitePermissionSnapshotStore) SaveSnapshot(ctx context.Context, snap PermissionSnapshot) error {
	resourceIDs := marshalStringList(snap.ResourceIDs)
	grantedPerms := marshalStringList(snap.GrantedPerms)
	grantedScopes := marshalStringList(snap.GrantedScopes)

	var expiresAt interface{}
	if snap.ExpiresAt != nil {
		expiresAt = *snap.ExpiresAt
	}
	var revokedAt interface{}
	if snap.RevokedAt != nil {
		revokedAt = *snap.RevokedAt
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_permission_snapshots
		(snapshot_id, session_id, extension_id, module_id, generation,
		 character_id, conversation_id, resource_ids, granted_perms, granted_scopes,
		 created_at, expires_at, revoked_at,
		 execution_placement, execution_user_id, execution_device_id, execution_runtime_id,
		 provider_id, provider_instance_id, execution_binding_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(snapshot_id) DO UPDATE SET
		 session_id = excluded.session_id,
		 extension_id = excluded.extension_id,
		 module_id = excluded.module_id,
		 generation = excluded.generation,
		 character_id = excluded.character_id,
		 conversation_id = excluded.conversation_id,
		 resource_ids = excluded.resource_ids,
		 granted_perms = excluded.granted_perms,
		 granted_scopes = excluded.granted_scopes,
		 created_at = excluded.created_at,
		 expires_at = excluded.expires_at,
		 revoked_at = excluded.revoked_at,
		 execution_placement = excluded.execution_placement,
		 execution_user_id = excluded.execution_user_id,
		 execution_device_id = excluded.execution_device_id,
		 execution_runtime_id = excluded.execution_runtime_id,
		 provider_id = excluded.provider_id,
		 provider_instance_id = excluded.provider_instance_id,
		 execution_binding_key = excluded.execution_binding_key
	`,
		snap.SnapshotID, snap.SessionID, snap.ExtensionID, snap.ModuleID, snap.Generation,
		snap.CharacterID, snap.ConversationID, resourceIDs, grantedPerms, grantedScopes,
		snap.CreatedAt, expiresAt, revokedAt,
		snap.ExecutionPlacement.String(), snap.UserID.String(), snap.DeviceID.String(), snap.RuntimeID.String(),
		snap.ProviderID, snap.ProviderInstanceID, snap.ExecutionBindingKey,
	)
	if err != nil {
		return fmt.Errorf("sqlite: save permission snapshot: %w", err)
	}
	return nil
}

func (s *SQLitePermissionSnapshotStore) GetSnapshot(ctx context.Context, snapshotID string) (PermissionSnapshot, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT snapshot_id, session_id, extension_id, module_id, generation,
		 COALESCE(character_id, ''), COALESCE(conversation_id, ''),
		 COALESCE(resource_ids, ''), COALESCE(granted_perms, ''), COALESCE(granted_scopes, ''),
		 created_at, expires_at, revoked_at,
		 COALESCE(execution_placement, ''), COALESCE(execution_user_id, ''),
		 COALESCE(execution_device_id, ''), COALESCE(execution_runtime_id, ''),
		 COALESCE(provider_id, ''), COALESCE(provider_instance_id, ''),
		 COALESCE(execution_binding_key, '')
		FROM kernel_permission_snapshots
		WHERE snapshot_id = ?
	`, snapshotID)

	var snap PermissionSnapshot
	var resourceIDs, grantedPerms, grantedScopes string
	var executionPlacement, executionUserID, executionDeviceID, executionRuntimeID string
	var providerID, providerInstanceID, executionBindingKey string
	var expiresAt, revokedAt sql.NullTime

	err := row.Scan(
		&snap.SnapshotID, &snap.SessionID, &snap.ExtensionID, &snap.ModuleID, &snap.Generation,
		&snap.CharacterID, &snap.ConversationID,
		&resourceIDs, &grantedPerms, &grantedScopes,
		&snap.CreatedAt, &expiresAt, &revokedAt,
		&executionPlacement, &executionUserID, &executionDeviceID, &executionRuntimeID,
		&providerID, &providerInstanceID, &executionBindingKey,
	)
	if err != nil {
		return PermissionSnapshot{}, fmt.Errorf("%w: %v", ErrPermissionSnapshotNotFound, err)
	}
	snap.ResourceIDs = unmarshalStringList(resourceIDs)
	snap.GrantedPerms = unmarshalStringList(grantedPerms)
	snap.GrantedScopes = unmarshalStringList(grantedScopes)
	snap.ExecutionPlacement = ParseExecutionPlacement(executionPlacement)
	snap.UserID = runtimeidentity.ParseUserID(executionUserID)
	snap.DeviceID = runtimeidentity.ParseDeviceID(executionDeviceID)
	snap.RuntimeID = runtimeidentity.ParseRuntimeID(executionRuntimeID)
	snap.ProviderID = providerID
	snap.ProviderInstanceID = providerInstanceID
	snap.ExecutionBindingKey = executionBindingKey
	if expiresAt.Valid {
		t := expiresAt.Time
		snap.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		snap.RevokedAt = &t
	}
	return snap, nil
}

func (s *SQLitePermissionSnapshotStore) FindActiveSnapshot(ctx context.Context, extensionID string, moduleID string, generation int64) (PermissionSnapshot, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT snapshot_id
		FROM kernel_permission_snapshots
		WHERE extension_id = ? AND module_id = ? AND generation = ?
			AND revoked_at IS NULL
			AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY created_at DESC
		LIMIT 1
	`, extensionID, moduleID, generation, time.Now().UTC())

	var snapshotID string
	if err := row.Scan(&snapshotID); err != nil {
		if err == sql.ErrNoRows {
			return PermissionSnapshot{}, false, nil
		}
		return PermissionSnapshot{}, false, fmt.Errorf("sqlite: find active permission snapshot: %w", err)
	}

	snap, err := s.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return PermissionSnapshot{}, false, err
	}
	return snap, true, nil
}

func (s *SQLitePermissionSnapshotStore) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM kernel_permission_snapshots WHERE snapshot_id = ?`, snapshotID)
	if err != nil {
		return fmt.Errorf("sqlite: delete permission snapshot: %w", err)
	}
	return nil
}

func (s *SQLitePermissionSnapshotStore) RevokeSnapshot(ctx context.Context, snapshotID string) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `UPDATE kernel_permission_snapshots SET revoked_at = ? WHERE snapshot_id = ?`, now, snapshotID)
	if err != nil {
		return fmt.Errorf("sqlite: revoke permission snapshot: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrPermissionSnapshotNotFound
	}
	return nil
}

func (s *SQLitePermissionSnapshotStore) DeleteBySession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM kernel_permission_snapshots WHERE session_id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("sqlite: delete permission snapshots by session: %w", err)
	}
	return nil
}

func (s *SQLitePermissionSnapshotStore) RevokeInvalidSnapshots(ctx context.Context, validator *PermissionIDValidator, subjectValidator func(extensionID, moduleID string, generation int64) bool) (int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT snapshot_id, extension_id, module_id, generation, granted_perms
		FROM kernel_permission_snapshots
		WHERE revoked_at IS NULL
	`)
	if err != nil {
		return 0, fmt.Errorf("sqlite: list active snapshots for validation: %w", err)
	}
	defer rows.Close()

	revoked := 0
	for rows.Next() {
		var snapshotID, extensionID, moduleID, grantedPermsJSON string
		var generation int64
		if err := rows.Scan(&snapshotID, &extensionID, &moduleID, &generation, &grantedPermsJSON); err != nil {
			return revoked, fmt.Errorf("sqlite: scan snapshot row: %w", err)
		}

		invalidPerms := false
		if validator != nil {
			perms := unmarshalStringList(grantedPermsJSON)
			invalidPerms = len(validator.ValidateAll(perms)) > 0
		}

		invalidSubject := false
		if subjectValidator != nil {
			invalidSubject = !subjectValidator(extensionID, moduleID, generation)
		}

		if invalidPerms || invalidSubject {
			if err := s.RevokeSnapshot(ctx, snapshotID); err != nil {
				continue
			}
			revoked++
		}
	}
	return revoked, nil
}

var _ PermissionSnapshotStore = (*SQLitePermissionSnapshotStore)(nil)
