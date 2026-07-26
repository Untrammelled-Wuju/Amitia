package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type ScopeBinding struct {
	ExtensionID domain.ExtensionID `json:"extensionId"`
	ScopeType   string             `json:"scopeType"`
	ScopeID     string             `json:"scopeId"`
	Active      bool               `json:"active"`
	CreatedAt   time.Time          `json:"createdAt"`
}

type ScopeRepository interface {
	PutBinding(ctx context.Context, binding ScopeBinding) error
	GetBinding(ctx context.Context, extensionID domain.ExtensionID, scopeType, scopeID string) (ScopeBinding, error)
	ListBindings(ctx context.Context, extensionID domain.ExtensionID) ([]ScopeBinding, error)
	ListActiveBindings(ctx context.Context, scopeType, scopeID string) ([]ScopeBinding, error)
	DeactivateBinding(ctx context.Context, extensionID domain.ExtensionID, scopeType, scopeID string) error
	DeleteBindings(ctx context.Context, extensionID domain.ExtensionID) error
}

type SQLiteScopeRepository struct {
	db *sql.DB
}

func NewScopeRepository(db *sql.DB) *SQLiteScopeRepository {
	return &SQLiteScopeRepository{db: db}
}

func (r *SQLiteScopeRepository) PutBinding(ctx context.Context, binding ScopeBinding) error {
	id := scopeBindingKey(binding.ExtensionID, binding.ScopeType, binding.ScopeID)
	now := time.Now().UTC()
	created := binding.CreatedAt
	if created.IsZero() {
		created = now
	}
	active := 0
	if binding.Active {
		active = 1
	}

	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_scope_bindings (id, extension_id, scope_type, scope_id, active, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(extension_id, scope_type, scope_id) DO UPDATE SET
			active = excluded.active
	`,
		id,
		string(binding.ExtensionID),
		binding.ScopeType,
		binding.ScopeID,
		active,
		created,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert scope binding: %w", err)
	}
	return nil
}

func (r *SQLiteScopeRepository) GetBinding(ctx context.Context, extensionID domain.ExtensionID, scopeType, scopeID string) (ScopeBinding, error) {
	ex := getExecutor(ctx, r.db)
	var b ScopeBinding
	var extID string
	var active int

	err := ex.QueryRowContext(ctx, `
		SELECT extension_id, scope_type, scope_id, active, created_at
		FROM extension_scope_bindings WHERE extension_id = ? AND scope_type = ? AND scope_id = ?
	`, string(extensionID), scopeType, scopeID).Scan(
		&extID,
		&b.ScopeType,
		&b.ScopeID,
		&active,
		&b.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return ScopeBinding{}, fmt.Errorf("sqlite: scope binding not found")
		}
		return ScopeBinding{}, fmt.Errorf("sqlite: query scope binding: %w", err)
	}

	b.ExtensionID = domain.ExtensionID(extID)
	b.Active = active != 0
	return b, nil
}

func (r *SQLiteScopeRepository) ListBindings(ctx context.Context, extensionID domain.ExtensionID) ([]ScopeBinding, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT extension_id, scope_type, scope_id, active, created_at
		FROM extension_scope_bindings WHERE extension_id = ? ORDER BY scope_type, scope_id
	`, string(extensionID))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list scope bindings: %w", err)
	}
	defer rows.Close()

	var out []ScopeBinding
	for rows.Next() {
		var b ScopeBinding
		var extID string
		var active int
		err := rows.Scan(&extID, &b.ScopeType, &b.ScopeID, &active, &b.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan scope binding: %w", err)
		}
		b.ExtensionID = domain.ExtensionID(extID)
		b.Active = active != 0
		out = append(out, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate scope bindings: %w", err)
	}

	return out, nil
}

func (r *SQLiteScopeRepository) ListActiveBindings(ctx context.Context, scopeType, scopeID string) ([]ScopeBinding, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT extension_id, scope_type, scope_id, active, created_at
		FROM extension_scope_bindings WHERE scope_type = ? AND scope_id = ? AND active = 1 ORDER BY extension_id
	`, scopeType, scopeID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list active scope bindings: %w", err)
	}
	defer rows.Close()

	var out []ScopeBinding
	for rows.Next() {
		var b ScopeBinding
		var extID string
		var active int
		err := rows.Scan(&extID, &b.ScopeType, &b.ScopeID, &active, &b.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan scope binding: %w", err)
		}
		b.ExtensionID = domain.ExtensionID(extID)
		b.Active = active != 0
		out = append(out, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate scope bindings: %w", err)
	}

	return out, nil
}

func (r *SQLiteScopeRepository) DeactivateBinding(ctx context.Context, extensionID domain.ExtensionID, scopeType, scopeID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `UPDATE extension_scope_bindings SET active = 0 WHERE extension_id = ? AND scope_type = ? AND scope_id = ?`, string(extensionID), scopeType, scopeID)
	if err != nil {
		return fmt.Errorf("sqlite: deactivate scope binding: %w", err)
	}
	return nil
}

func (r *SQLiteScopeRepository) DeleteBindings(ctx context.Context, extensionID domain.ExtensionID) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_scope_bindings WHERE extension_id = ?`, string(extensionID))
	if err != nil {
		return fmt.Errorf("sqlite: delete scope bindings: %w", err)
	}
	return nil
}

func scopeBindingKey(extensionID domain.ExtensionID, scopeType, scopeID string) string {
	return string(extensionID) + "/" + scopeType + "/" + scopeID
}

var _ ScopeRepository = (*SQLiteScopeRepository)(nil)
