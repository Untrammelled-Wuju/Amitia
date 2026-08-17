package permission

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const permissionGrantSelectColumns = `grant_id, subject_type, subject_id, permission_id, scope_type, scope_id, scope_data, decision, input_binding, target_binding, issued_at, expires_at, issued_by, reason, revoked_at, manifest_ver`

type SQLitePermissionStorage struct {
	db *sql.DB
}

func NewSQLitePermissionStorage(db *sql.DB) *SQLitePermissionStorage {
	return &SQLitePermissionStorage{db: db}
}

func rawMessageToNullString(data json.RawMessage) sql.NullString {
	if len(data) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(data), Valid: true}
}

func nullStringToRawMessage(ns sql.NullString) json.RawMessage {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	return json.RawMessage(ns.String)
}

func nullStringFromValue(v string) sql.NullString {
	return sql.NullString{String: v, Valid: v != ""}
}

func scanStoredGrant(rows *sql.Rows) (StoredGrant, error) {
	var g StoredGrant
	var scopeData, inputBinding, targetBinding, reason, manifestVer sql.NullString
	var expiresAt, revokedAt sql.NullTime
	err := rows.Scan(
		&g.GrantID,
		&g.SubjectType,
		&g.SubjectID,
		&g.PermissionID,
		&g.ScopeType,
		&g.ScopeID,
		&scopeData,
		&g.Decision,
		&inputBinding,
		&targetBinding,
		&g.IssuedAt,
		&expiresAt,
		&g.IssuedBy,
		&reason,
		&revokedAt,
		&manifestVer,
	)
	if err != nil {
		return StoredGrant{}, fmt.Errorf("sqlite: scan permission grant: %w", err)
	}
	g.ScopeData = nullStringToRawMessage(scopeData)
	g.InputBinding = nullStringToRawMessage(inputBinding)
	g.TargetBinding = nullStringToRawMessage(targetBinding)
	if expiresAt.Valid {
		t := expiresAt.Time.UTC()
		g.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time.UTC()
		g.RevokedAt = &t
	}
	g.Reason = reason.String
	g.ManifestVer = manifestVer.String
	return g, nil
}

func (s *SQLitePermissionStorage) Save(ctx context.Context, grant StoredGrant) error {
	issuedAt := grant.IssuedAt
	if issuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	}
	var expiresAt sql.NullTime
	if grant.ExpiresAt != nil {
		expiresAt = sql.NullTime{Time: grant.ExpiresAt.UTC(), Valid: true}
	}
	var revokedAt sql.NullTime
	if grant.RevokedAt != nil {
		revokedAt = sql.NullTime{Time: grant.RevokedAt.UTC(), Valid: true}
	}
	scopeData := rawMessageToNullString(grant.ScopeData)
	inputBinding := rawMessageToNullString(grant.InputBinding)
	targetBinding := rawMessageToNullString(grant.TargetBinding)
	reason := nullStringFromValue(grant.Reason)
	manifestVer := nullStringFromValue(grant.ManifestVer)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_permission_grants (
			grant_id, subject_type, subject_id, permission_id, scope_type, scope_id, scope_data,
			decision, input_binding, target_binding, issued_at, expires_at, issued_by,
			reason, revoked_at, manifest_ver
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(subject_type, subject_id, permission_id, scope_type, scope_id) DO UPDATE SET
			grant_id = excluded.grant_id,
			subject_type = excluded.subject_type,
			subject_id = excluded.subject_id,
			permission_id = excluded.permission_id,
			scope_type = excluded.scope_type,
			scope_id = excluded.scope_id,
			scope_data = excluded.scope_data,
			decision = excluded.decision,
			input_binding = excluded.input_binding,
			target_binding = excluded.target_binding,
			issued_at = excluded.issued_at,
			expires_at = excluded.expires_at,
			issued_by = excluded.issued_by,
			reason = excluded.reason,
			revoked_at = excluded.revoked_at,
			manifest_ver = excluded.manifest_ver
	`,
		grant.GrantID,
		grant.SubjectType,
		grant.SubjectID,
		grant.PermissionID,
		grant.ScopeType,
		grant.ScopeID,
		scopeData,
		grant.Decision,
		inputBinding,
		targetBinding,
		issuedAt,
		expiresAt,
		grant.IssuedBy,
		reason,
		revokedAt,
		manifestVer,
	)
	if err != nil {
		return fmt.Errorf("sqlite: save permission grant: %w", err)
	}
	return nil
}

func (s *SQLitePermissionStorage) GetByGrantID(ctx context.Context, grantID string) (StoredGrant, bool, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+permissionGrantSelectColumns+" FROM kernel_permission_grants WHERE grant_id = ?",
		grantID,
	)
	if err != nil {
		return StoredGrant{}, false, fmt.Errorf("sqlite: get permission grant by id: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return StoredGrant{}, false, rows.Err()
	}
	grant, err := scanStoredGrant(rows)
	if err != nil {
		return StoredGrant{}, false, err
	}
	return grant, true, nil
}

func (s *SQLitePermissionStorage) List(ctx context.Context, filter PermissionGrantFilter) ([]StoredGrant, error) {
	var conditions []string
	var args []any
	if filter.Subject != nil {
		conditions = append(conditions, "subject_type = ?", "subject_id = ?")
		args = append(args, string(filter.Subject.Type), filter.Subject.ID)
	}
	if filter.PermissionID != "" {
		conditions = append(conditions, "permission_id = ?")
		args = append(args, filter.PermissionID)
	}
	if filter.Scope != nil {
		conditions = append(conditions, "scope_type = ?", "scope_id = ?")
		args = append(args, string(filter.Scope.Type), filter.Scope.ID)
	}
	if filter.Decision != nil {
		conditions = append(conditions, "decision = ?")
		args = append(args, string(*filter.Decision))
	}
	if filter.ActiveOnly {
		conditions = append(conditions, "revoked_at IS NULL")
	}

	query := "SELECT " + permissionGrantSelectColumns + " FROM kernel_permission_grants"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY issued_at DESC, grant_id"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list permission grants: %w", err)
	}
	defer rows.Close()

	var out []StoredGrant
	for rows.Next() {
		grant, err := scanStoredGrant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate permission grants: %w", err)
	}
	return out, nil
}

func (s *SQLitePermissionStorage) ListBySubject(ctx context.Context, subject PermissionSubject) ([]StoredGrant, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+permissionGrantSelectColumns+" FROM kernel_permission_grants WHERE subject_type = ? AND subject_id = ? ORDER BY issued_at DESC, grant_id",
		string(subject.Type), subject.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list permission grants by subject: %w", err)
	}
	defer rows.Close()

	var out []StoredGrant
	for rows.Next() {
		grant, err := scanStoredGrant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate permission grants by subject: %w", err)
	}
	return out, nil
}

func (s *SQLitePermissionStorage) MarkRevoked(ctx context.Context, grantID string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `UPDATE kernel_permission_grants SET revoked_at = ? WHERE grant_id = ?`, now, grantID)
	if err != nil {
		return fmt.Errorf("sqlite: mark permission grant revoked: %w", err)
	}
	return nil
}

func (s *SQLitePermissionStorage) Delete(ctx context.Context, grantID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM kernel_permission_grants WHERE grant_id = ?`, grantID)
	if err != nil {
		return fmt.Errorf("sqlite: delete permission grant: %w", err)
	}
	return nil
}

var _ PermissionStorage = (*SQLitePermissionStorage)(nil)
