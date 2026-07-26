package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type PermissionRequirement struct {
	ExtensionID    domain.ExtensionID `json:"extensionId"`
	PermissionName string             `json:"permissionName"`
	Reason         string             `json:"reason,omitempty"`
	Required       bool               `json:"required"`
	Scope          string             `json:"scope,omitempty"`
}

type PermissionGrant struct {
	ExtensionID    domain.ExtensionID `json:"extensionId"`
	PermissionName string             `json:"permissionName"`
	State          string             `json:"state"`
	GrantedAt      time.Time          `json:"grantedAt"`
	ExpiresAt      *time.Time         `json:"expiresAt,omitempty"`
}

type PermissionRepository interface {
	PutRequirement(ctx context.Context, req PermissionRequirement) error
	ListRequirements(ctx context.Context, extensionID domain.ExtensionID) ([]PermissionRequirement, error)
	DeleteRequirements(ctx context.Context, extensionID domain.ExtensionID) error
	PutGrant(ctx context.Context, grant PermissionGrant) error
	GetGrant(ctx context.Context, extensionID domain.ExtensionID, permissionName string) (PermissionGrant, error)
	ListGrants(ctx context.Context, extensionID domain.ExtensionID) ([]PermissionGrant, error)
	DeleteGrant(ctx context.Context, extensionID domain.ExtensionID, permissionName string) error
}

type SQLitePermissionRepository struct {
	db *sql.DB
}

func NewPermissionRepository(db *sql.DB) *SQLitePermissionRepository {
	return &SQLitePermissionRepository{db: db}
}

func (r *SQLitePermissionRepository) PutRequirement(ctx context.Context, req PermissionRequirement) error {
	id := string(req.ExtensionID) + "/" + req.PermissionName
	required := 0
	if req.Required {
		required = 1
	}

	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_permission_requirements (id, extension_id, permission_name, reason, required, scope)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(extension_id, permission_name) DO UPDATE SET
			reason = excluded.reason,
			required = excluded.required,
			scope = excluded.scope
	`,
		id,
		string(req.ExtensionID),
		req.PermissionName,
		req.Reason,
		required,
		req.Scope,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert permission requirement: %w", err)
	}
	return nil
}

func (r *SQLitePermissionRepository) ListRequirements(ctx context.Context, extensionID domain.ExtensionID) ([]PermissionRequirement, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT extension_id, permission_name, reason, required, scope
		FROM extension_permission_requirements WHERE extension_id = ? ORDER BY permission_name
	`, string(extensionID))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list permission requirements: %w", err)
	}
	defer rows.Close()

	var out []PermissionRequirement
	for rows.Next() {
		var req PermissionRequirement
		var extID string
		var required int
		err := rows.Scan(&extID, &req.PermissionName, &req.Reason, &required, &req.Scope)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan permission requirement: %w", err)
		}
		req.ExtensionID = domain.ExtensionID(extID)
		req.Required = required != 0
		out = append(out, req)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate permission requirements: %w", err)
	}

	return out, nil
}

func (r *SQLitePermissionRepository) DeleteRequirements(ctx context.Context, extensionID domain.ExtensionID) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_permission_requirements WHERE extension_id = ?`, string(extensionID))
	if err != nil {
		return fmt.Errorf("sqlite: delete permission requirements: %w", err)
	}
	return nil
}

func (r *SQLitePermissionRepository) PutGrant(ctx context.Context, grant PermissionGrant) error {
	id := string(grant.ExtensionID) + "/" + grant.PermissionName
	grantedAt := grant.GrantedAt
	if grantedAt.IsZero() {
		grantedAt = time.Now().UTC()
	}

	var expiresAt sql.NullTime
	if grant.ExpiresAt != nil {
		expiresAt = sql.NullTime{Time: grant.ExpiresAt.UTC(), Valid: true}
	}

	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_permission_grants (id, extension_id, permission_name, state, granted_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(extension_id, permission_name) DO UPDATE SET
			state = excluded.state,
			granted_at = excluded.granted_at,
			expires_at = excluded.expires_at
	`,
		id,
		string(grant.ExtensionID),
		grant.PermissionName,
		grant.State,
		grantedAt,
		expiresAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert permission grant: %w", err)
	}
	return nil
}

func (r *SQLitePermissionRepository) GetGrant(ctx context.Context, extensionID domain.ExtensionID, permissionName string) (PermissionGrant, error) {
	ex := getExecutor(ctx, r.db)
	var grant PermissionGrant
	var extID string
	var expiresAt sql.NullTime

	err := ex.QueryRowContext(ctx, `
		SELECT extension_id, permission_name, state, granted_at, expires_at
		FROM extension_permission_grants WHERE extension_id = ? AND permission_name = ?
	`, string(extensionID), permissionName).Scan(
		&extID,
		&grant.PermissionName,
		&grant.State,
		&grant.GrantedAt,
		&expiresAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return PermissionGrant{}, fmt.Errorf("sqlite: permission grant not found")
		}
		return PermissionGrant{}, fmt.Errorf("sqlite: query permission grant: %w", err)
	}

	grant.ExtensionID = domain.ExtensionID(extID)
	if expiresAt.Valid {
		t := expiresAt.Time.UTC()
		grant.ExpiresAt = &t
	}

	return grant, nil
}

func (r *SQLitePermissionRepository) ListGrants(ctx context.Context, extensionID domain.ExtensionID) ([]PermissionGrant, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT extension_id, permission_name, state, granted_at, expires_at
		FROM extension_permission_grants WHERE extension_id = ? ORDER BY permission_name
	`, string(extensionID))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list permission grants: %w", err)
	}
	defer rows.Close()

	var out []PermissionGrant
	for rows.Next() {
		var grant PermissionGrant
		var extID string
		var expiresAt sql.NullTime
		err := rows.Scan(&extID, &grant.PermissionName, &grant.State, &grant.GrantedAt, &expiresAt)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan permission grant: %w", err)
		}
		grant.ExtensionID = domain.ExtensionID(extID)
		if expiresAt.Valid {
			t := expiresAt.Time.UTC()
			grant.ExpiresAt = &t
		}
		out = append(out, grant)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate permission grants: %w", err)
	}

	return out, nil
}

func (r *SQLitePermissionRepository) DeleteGrant(ctx context.Context, extensionID domain.ExtensionID, permissionName string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_permission_grants WHERE extension_id = ? AND permission_name = ?`, string(extensionID), permissionName)
	if err != nil {
		return fmt.Errorf("sqlite: delete permission grant: %w", err)
	}
	return nil
}

var _ PermissionRepository = (*SQLitePermissionRepository)(nil)
