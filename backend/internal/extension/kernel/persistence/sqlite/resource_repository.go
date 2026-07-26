package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type ResourceRepository interface {
	PutResource(ctx context.Context, resource domain.ResourceOwnership) error
	GetResource(ctx context.Context, resourceID string) (domain.ResourceOwnership, error)
	ListResources(ctx context.Context, extensionID domain.ExtensionID) ([]domain.ResourceOwnership, error)
	ListExpiredResources(ctx context.Context, before time.Time) ([]domain.ResourceOwnership, error)
	DeleteResource(ctx context.Context, resourceID string) error
	DeleteExpiredResources(ctx context.Context, before time.Time) error
}

type SQLiteResourceRepository struct {
	db *sql.DB
}

func NewResourceRepository(db *sql.DB) *SQLiteResourceRepository {
	return &SQLiteResourceRepository{db: db}
}

func (r *SQLiteResourceRepository) PutResource(ctx context.Context, resource domain.ResourceOwnership) error {
	metadataJSON := ""
	if resource.Metadata != nil {
		data, err := json.Marshal(resource.Metadata)
		if err != nil {
			return fmt.Errorf("sqlite: marshal resource metadata: %w", err)
		}
		metadataJSON = string(data)
	}

	acquiredAt := resource.AcquiredAt
	if acquiredAt.IsZero() {
		acquiredAt = time.Now().UTC()
	}

	var expiresAt sql.NullTime
	if resource.ExpiresAt != nil {
		expiresAt = sql.NullTime{Time: resource.ExpiresAt.UTC(), Valid: true}
	}

	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_resources (resource_id, extension_id, resource_type, reference, acquired_at, expires_at, owner_type, owner_id, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(resource_id) DO UPDATE SET
			extension_id = excluded.extension_id,
			resource_type = excluded.resource_type,
			reference = excluded.reference,
			acquired_at = excluded.acquired_at,
			expires_at = excluded.expires_at,
			owner_type = excluded.owner_type,
			owner_id = excluded.owner_id,
			metadata_json = excluded.metadata_json
	`,
		resource.ResourceID,
		resource.OwnerID,
		resource.ResourceType,
		resource.Reference,
		acquiredAt,
		expiresAt,
		resource.OwnerType,
		resource.OwnerID,
		metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert resource: %w", err)
	}
	return nil
}

func (r *SQLiteResourceRepository) GetResource(ctx context.Context, resourceID string) (domain.ResourceOwnership, error) {
	ex := getExecutor(ctx, r.db)
	var res domain.ResourceOwnership
	var extensionID string
	var expiresAt sql.NullTime
	var metadataJSON string

	err := ex.QueryRowContext(ctx, `
		SELECT resource_id, extension_id, resource_type, reference, acquired_at, expires_at, owner_type, owner_id, metadata_json
		FROM extension_resources WHERE resource_id = ?
	`, resourceID).Scan(
		&res.ResourceID,
		&extensionID,
		&res.ResourceType,
		&res.Reference,
		&res.AcquiredAt,
		&expiresAt,
		&res.OwnerType,
		&res.OwnerID,
		&metadataJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.ResourceOwnership{}, fmt.Errorf("sqlite: resource not found: %s", resourceID)
		}
		return domain.ResourceOwnership{}, fmt.Errorf("sqlite: query resource: %w", err)
	}

	if expiresAt.Valid {
		t := expiresAt.Time.UTC()
		res.ExpiresAt = &t
	}

	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &res.Metadata); err != nil {
			return domain.ResourceOwnership{}, fmt.Errorf("sqlite: unmarshal resource metadata: %w", err)
		}
	}

	return res, nil
}

func (r *SQLiteResourceRepository) ListResources(ctx context.Context, extensionID domain.ExtensionID) ([]domain.ResourceOwnership, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT resource_id, extension_id, resource_type, reference, acquired_at, expires_at, owner_type, owner_id, metadata_json
		FROM extension_resources WHERE extension_id = ? ORDER BY acquired_at DESC
	`, string(extensionID))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list resources: %w", err)
	}
	defer rows.Close()

	var out []domain.ResourceOwnership
	for rows.Next() {
		var res domain.ResourceOwnership
		var extID string
		var expiresAt sql.NullTime
		var metadataJSON string

		err := rows.Scan(
			&res.ResourceID,
			&extID,
			&res.ResourceType,
			&res.Reference,
			&res.AcquiredAt,
			&expiresAt,
			&res.OwnerType,
			&res.OwnerID,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan resource: %w", err)
		}

		if expiresAt.Valid {
			t := expiresAt.Time.UTC()
			res.ExpiresAt = &t
		}

		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &res.Metadata); err != nil {
				return nil, fmt.Errorf("sqlite: unmarshal resource metadata: %w", err)
			}
		}

		out = append(out, res)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate resources: %w", err)
	}

	return out, nil
}

func (r *SQLiteResourceRepository) ListExpiredResources(ctx context.Context, before time.Time) ([]domain.ResourceOwnership, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT resource_id, extension_id, resource_type, reference, acquired_at, expires_at, owner_type, owner_id, metadata_json
		FROM extension_resources WHERE expires_at IS NOT NULL AND expires_at < ? ORDER BY expires_at
	`, before.UTC())
	if err != nil {
		return nil, fmt.Errorf("sqlite: list expired resources: %w", err)
	}
	defer rows.Close()

	var out []domain.ResourceOwnership
	for rows.Next() {
		var res domain.ResourceOwnership
		var extID string
		var expiresAt sql.NullTime
		var metadataJSON string

		err := rows.Scan(
			&res.ResourceID,
			&extID,
			&res.ResourceType,
			&res.Reference,
			&res.AcquiredAt,
			&expiresAt,
			&res.OwnerType,
			&res.OwnerID,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan resource: %w", err)
		}

		if expiresAt.Valid {
			t := expiresAt.Time.UTC()
			res.ExpiresAt = &t
		}

		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &res.Metadata); err != nil {
				return nil, fmt.Errorf("sqlite: unmarshal resource metadata: %w", err)
			}
		}

		out = append(out, res)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate resources: %w", err)
	}

	return out, nil
}

func (r *SQLiteResourceRepository) DeleteResource(ctx context.Context, resourceID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_resources WHERE resource_id = ?`, resourceID)
	if err != nil {
		return fmt.Errorf("sqlite: delete resource: %w", err)
	}
	return nil
}

func (r *SQLiteResourceRepository) DeleteExpiredResources(ctx context.Context, before time.Time) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_resources WHERE expires_at IS NOT NULL AND expires_at < ?`, before.UTC())
	if err != nil {
		return fmt.Errorf("sqlite: delete expired resources: %w", err)
	}
	return nil
}

var _ ResourceRepository = (*SQLiteResourceRepository)(nil)
