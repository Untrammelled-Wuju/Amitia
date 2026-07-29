package permission

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
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
		 created_at, expires_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		 revoked_at = excluded.revoked_at
	`,
		snap.SnapshotID, snap.SessionID, snap.ExtensionID, snap.ModuleID, snap.Generation,
		snap.CharacterID, snap.ConversationID, resourceIDs, grantedPerms, grantedScopes,
		snap.CreatedAt, expiresAt, revokedAt,
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
		 created_at, expires_at, revoked_at
		FROM kernel_permission_snapshots
		WHERE snapshot_id = ?
	`, snapshotID)

	var snap PermissionSnapshot
	var resourceIDs, grantedPerms, grantedScopes string
	var expiresAt, revokedAt sql.NullTime

	err := row.Scan(
		&snap.SnapshotID, &snap.SessionID, &snap.ExtensionID, &snap.ModuleID, &snap.Generation,
		&snap.CharacterID, &snap.ConversationID,
		&resourceIDs, &grantedPerms, &grantedScopes,
		&snap.CreatedAt, &expiresAt, &revokedAt,
	)
	if err != nil {
		return PermissionSnapshot{}, fmt.Errorf("%w: %v", ErrPermissionSnapshotNotFound, err)
	}
	snap.ResourceIDs = unmarshalStringList(resourceIDs)
	snap.GrantedPerms = unmarshalStringList(grantedPerms)
	snap.GrantedScopes = unmarshalStringList(grantedScopes)
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

func (s *SQLitePermissionSnapshotStore) RevokeInvalidSnapshots(ctx context.Context, validator *PermissionIDValidator) (int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT snapshot_id, granted_perms FROM kernel_permission_snapshots
		WHERE revoked_at IS NULL
	`)
	if err != nil {
		return 0, fmt.Errorf("sqlite: list active snapshots for validation: %w", err)
	}
	defer rows.Close()

	revoked := 0
	for rows.Next() {
		var snapshotID, grantedPermsJSON string
		if err := rows.Scan(&snapshotID, &grantedPermsJSON); err != nil {
			return revoked, fmt.Errorf("sqlite: scan snapshot row: %w", err)
		}
		perms := unmarshalStringList(grantedPermsJSON)
		if validator != nil && len(validator.ValidateAll(perms)) > 0 {
			if err := s.RevokeSnapshot(ctx, snapshotID); err != nil {
				continue
			}
			revoked++
		}
	}
	return revoked, nil
}

var _ PermissionSnapshotStore = (*SQLitePermissionSnapshotStore)(nil)
