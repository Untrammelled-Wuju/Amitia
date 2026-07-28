package scope

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type SQLiteScopeStore struct {
	db *sql.DB
}

func NewSQLiteScopeStore(db *sql.DB) *SQLiteScopeStore {
	return &SQLiteScopeStore{db: db}
}

func scopeIDFromRef(ref ScopeRef) string {
	switch ref.Type {
	case ScopeGlobal:
		return ""
	case ScopeCharacter:
		return ref.CharacterID
	case ScopeConversation:
		return ref.ConversationID
	case ScopeExtension:
		return ref.ExtensionID
	case ScopeModule:
		return ref.ModuleID
	case ScopeResource:
		return ref.ResourceID
	case ScopeInvocation:
		return ref.InvocationID
	case ScopeSession:
		return ref.SessionID
	default:
		return ""
	}
}

func scopeRefFromRow(scopeType, scopeID, characterID, conversationID, extensionID, moduleID, resourceType, resourceID, invocationID, sessionID string) ScopeRef {
	return ScopeRef{
		Type:           ScopeType(scopeType),
		CharacterID:    characterID,
		ConversationID: conversationID,
		ExtensionID:    extensionID,
		ModuleID:       moduleID,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		InvocationID:   invocationID,
		SessionID:      sessionID,
	}
}

func (s *SQLiteScopeStore) SaveBinding(ctx context.Context, binding ScopeBinding) error {
	scopeID := scopeIDFromRef(binding.Scope)
	var metadataVal interface{}
	if binding.Metadata != nil {
		raw, err := json.Marshal(binding.Metadata)
		if err != nil {
			return fmt.Errorf("scope: marshal metadata: %w", err)
		}
		metadataVal = string(raw)
	}
	var expiresAt interface{}
	if binding.ExpiresAt != nil {
		expiresAt = *binding.ExpiresAt
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_scope_bindings
		(binding_id, subject_type, subject_id, scope_type, scope_id,
		 character_id, conversation_id, extension_id, module_id,
		 resource_type, resource_id, invocation_id, session_id,
		 state, source, created_at, updated_at, expires_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(binding_id) DO UPDATE SET
		 subject_type = excluded.subject_type,
		 subject_id = excluded.subject_id,
		 scope_type = excluded.scope_type,
		 scope_id = excluded.scope_id,
		 character_id = excluded.character_id,
		 conversation_id = excluded.conversation_id,
		 extension_id = excluded.extension_id,
		 module_id = excluded.module_id,
		 resource_type = excluded.resource_type,
		 resource_id = excluded.resource_id,
		 invocation_id = excluded.invocation_id,
		 session_id = excluded.session_id,
		 state = excluded.state,
		 source = excluded.source,
		 updated_at = excluded.updated_at,
		 expires_at = excluded.expires_at,
		 metadata = excluded.metadata
	`,
		binding.BindingID, string(binding.SubjectType), binding.SubjectID,
		string(binding.Scope.Type), scopeID,
		binding.Scope.CharacterID, binding.Scope.ConversationID, binding.Scope.ExtensionID, binding.Scope.ModuleID,
		binding.Scope.ResourceType, binding.Scope.ResourceID, binding.Scope.InvocationID, binding.Scope.SessionID,
		string(binding.State), string(binding.Source),
		binding.CreatedAt, binding.UpdatedAt, expiresAt, metadataVal,
	)
	return err
}

func (s *SQLiteScopeStore) GetBinding(ctx context.Context, bindingID string) (ScopeBinding, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT binding_id, subject_type, subject_id, scope_type, scope_id,
		 COALESCE(character_id, ''), COALESCE(conversation_id, ''), COALESCE(extension_id, ''), COALESCE(module_id, ''),
		 COALESCE(resource_type, ''), COALESCE(resource_id, ''), COALESCE(invocation_id, ''), COALESCE(session_id, ''),
		 state, source, created_at, updated_at, expires_at, COALESCE(metadata, '')
		FROM kernel_scope_bindings
		WHERE binding_id = ?
	`, bindingID)
	return scanBindingRow(row)
}

func (s *SQLiteScopeStore) DeleteBinding(ctx context.Context, bindingID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM kernel_scope_bindings WHERE binding_id = ?`, bindingID)
	return err
}

func (s *SQLiteScopeStore) ListBindings(ctx context.Context, filter ScopeBindingFilter) ([]ScopeBinding, error) {
	query := `
		SELECT binding_id, subject_type, subject_id, scope_type, scope_id,
		 COALESCE(character_id, ''), COALESCE(conversation_id, ''), COALESCE(extension_id, ''), COALESCE(module_id, ''),
		 COALESCE(resource_type, ''), COALESCE(resource_id, ''), COALESCE(invocation_id, ''), COALESCE(session_id, ''),
		 state, source, created_at, updated_at, expires_at, COALESCE(metadata, '')
		FROM kernel_scope_bindings
		WHERE 1=1
	`
	args := []interface{}{}
	if filter.SubjectType != "" {
		query += " AND subject_type = ?"
		args = append(args, string(filter.SubjectType))
	}
	if filter.SubjectID != "" {
		query += " AND subject_id = ?"
		args = append(args, filter.SubjectID)
	}
	if filter.ScopeType != "" {
		query += " AND scope_type = ?"
		args = append(args, string(filter.ScopeType))
	}
	if filter.State != "" {
		query += " AND state = ?"
		args = append(args, string(filter.State))
	}
	query += " ORDER BY created_at DESC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bindings []ScopeBinding
	for rows.Next() {
		b, err := scanBinding(rows)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, b)
	}
	return bindings, rows.Err()
}

func (s *SQLiteScopeStore) SaveSnapshot(ctx context.Context, snapshot ScopeSnapshot) error {
	raw, err := json.Marshal(snapshot.ResolvedScopes)
	if err != nil {
		return fmt.Errorf("scope: marshal resolved scopes: %w", err)
	}
	var expiresAt interface{}
	if snapshot.ExpiresAt != nil {
		expiresAt = *snapshot.ExpiresAt
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO kernel_scope_snapshots
		(snapshot_id, invocation_id, resolved_scopes,
		 character_id, conversation_id, extension_id, module_id,
		 generation, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(snapshot_id) DO UPDATE SET
		 invocation_id = excluded.invocation_id,
		 resolved_scopes = excluded.resolved_scopes,
		 character_id = excluded.character_id,
		 conversation_id = excluded.conversation_id,
		 extension_id = excluded.extension_id,
		 module_id = excluded.module_id,
		 generation = excluded.generation,
		 created_at = excluded.created_at,
		 expires_at = excluded.expires_at
	`,
		snapshot.SnapshotID, snapshot.InvocationID, string(raw),
		snapshot.CharacterID, snapshot.ConversationID, snapshot.ExtensionID, snapshot.ModuleID,
		snapshot.Generation, snapshot.CreatedAt, expiresAt,
	)
	return err
}

func (s *SQLiteScopeStore) GetSnapshot(ctx context.Context, snapshotID string) (ScopeSnapshot, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT snapshot_id, invocation_id, resolved_scopes,
		 COALESCE(character_id, ''), COALESCE(conversation_id, ''), COALESCE(extension_id, ''), COALESCE(module_id, ''),
		 generation, created_at, expires_at
		FROM kernel_scope_snapshots
		WHERE snapshot_id = ?
	`, snapshotID)
	var snap ScopeSnapshot
	var resolvedScopes string
	var expiresAt sql.NullTime
	err := row.Scan(
		&snap.SnapshotID, &snap.InvocationID, &resolvedScopes,
		&snap.CharacterID, &snap.ConversationID, &snap.ExtensionID, &snap.ModuleID,
		&snap.Generation, &snap.CreatedAt, &expiresAt,
	)
	if err != nil {
		return snap, fmt.Errorf("%w: %v", ErrSnapshotNotFound, err)
	}
	if err := json.Unmarshal([]byte(resolvedScopes), &snap.ResolvedScopes); err != nil {
		return snap, fmt.Errorf("scope: unmarshal resolved scopes: %w", err)
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		snap.ExpiresAt = &t
	}
	return snap, nil
}

func scanBinding(rows *sql.Rows) (ScopeBinding, error) {
	var b ScopeBinding
	var subjectType, scopeType, scopeID, state, source, metadata string
	var characterID, conversationID, extensionID, moduleID, resourceType, resourceID, invocationID, sessionID string
	var expiresAt sql.NullTime
	err := rows.Scan(
		&b.BindingID, &subjectType, &b.SubjectID, &scopeType, &scopeID,
		&characterID, &conversationID, &extensionID, &moduleID,
		&resourceType, &resourceID, &invocationID, &sessionID,
		&state, &source, &b.CreatedAt, &b.UpdatedAt, &expiresAt, &metadata,
	)
	if err != nil {
		return b, err
	}
	b.SubjectType = ScopeSubjectType(subjectType)
	b.State = ScopeBindingState(state)
	b.Source = ScopeBindingSource(source)
	b.Scope = scopeRefFromRow(scopeType, scopeID, characterID, conversationID, extensionID, moduleID, resourceType, resourceID, invocationID, sessionID)
	if expiresAt.Valid {
		t := expiresAt.Time
		b.ExpiresAt = &t
	}
	if metadata != "" {
		if err := json.Unmarshal([]byte(metadata), &b.Metadata); err != nil {
			return b, fmt.Errorf("scope: unmarshal metadata: %w", err)
		}
	}
	return b, nil
}

func scanBindingRow(row *sql.Row) (ScopeBinding, error) {
	var b ScopeBinding
	var subjectType, scopeType, scopeID, state, source, metadata string
	var characterID, conversationID, extensionID, moduleID, resourceType, resourceID, invocationID, sessionID string
	var expiresAt sql.NullTime
	err := row.Scan(
		&b.BindingID, &subjectType, &b.SubjectID, &scopeType, &scopeID,
		&characterID, &conversationID, &extensionID, &moduleID,
		&resourceType, &resourceID, &invocationID, &sessionID,
		&state, &source, &b.CreatedAt, &b.UpdatedAt, &expiresAt, &metadata,
	)
	if err != nil {
		return b, fmt.Errorf("%w: %v", ErrBindingNotFound, err)
	}
	b.SubjectType = ScopeSubjectType(subjectType)
	b.State = ScopeBindingState(state)
	b.Source = ScopeBindingSource(source)
	b.Scope = scopeRefFromRow(scopeType, scopeID, characterID, conversationID, extensionID, moduleID, resourceType, resourceID, invocationID, sessionID)
	if expiresAt.Valid {
		t := expiresAt.Time
		b.ExpiresAt = &t
	}
	if metadata != "" {
		if err := json.Unmarshal([]byte(metadata), &b.Metadata); err != nil {
			return b, fmt.Errorf("scope: unmarshal metadata: %w", err)
		}
	}
	return b, nil
}

var _ ScopeStore = (*SQLiteScopeStore)(nil)
