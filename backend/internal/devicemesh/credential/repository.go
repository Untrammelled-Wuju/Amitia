package credential

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, cred *DeviceRuntimeCredential) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO kernel_device_runtime_credentials (
			credential_id, credential_hash, user_id, device_id, runtime_id,
			status, created_at, expires_at, last_used_at, revoked_at, revision
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cred.ID, cred.CredentialHash,
		cred.UserID.String(), cred.DeviceID.String(), cred.RuntimeID.String(),
		cred.Status,
		cred.CreatedAt.UTC().Format(time.RFC3339Nano),
		cred.ExpiresAt.UTC().Format(time.RFC3339Nano),
		cred.LastUsedAt.UTC().Format(time.RFC3339Nano),
		formatTimePtr(cred.RevokedAt),
		cred.Revision,
	)
	if err != nil {
		return fmt.Errorf("credential: create: %w", err)
	}
	return nil
}

func (r *Repository) GetByHash(ctx context.Context, hash string) (*DeviceRuntimeCredential, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT credential_id, credential_hash, user_id, device_id, runtime_id,
			status, created_at, expires_at, last_used_at, revoked_at, revision
		FROM kernel_device_runtime_credentials WHERE credential_hash = ?`,
		hash,
	)
	return scanCredential(row)
}

func (r *Repository) GetByID(ctx context.Context, id string) (*DeviceRuntimeCredential, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT credential_id, credential_hash, user_id, device_id, runtime_id,
			status, created_at, expires_at, last_used_at, revoked_at, revision
		FROM kernel_device_runtime_credentials WHERE credential_id = ?`,
		id,
	)
	return scanCredential(row)
}

func (r *Repository) GetActiveByRuntime(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (*DeviceRuntimeCredential, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT credential_id, credential_hash, user_id, device_id, runtime_id,
			status, created_at, expires_at, last_used_at, revoked_at, revision
		FROM kernel_device_runtime_credentials
		WHERE user_id = ? AND device_id = ? AND runtime_id = ? AND status = ?`,
		userID.String(), deviceID.String(), runtimeID.String(), string(CredentialActive),
	)
	return scanCredential(row)
}

func (r *Repository) ExchangeAtomic(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID, now time.Time, newCred *DeviceRuntimeCredential) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("credential: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE kernel_device_runtime_credentials
		SET status = ?, revoked_at = ?, revision = revision + 1
		WHERE user_id = ? AND device_id = ? AND runtime_id = ? AND status = ?`,
		string(CredentialRevoked),
		now.UTC().Format(time.RFC3339Nano),
		userID.String(), deviceID.String(), runtimeID.String(),
		string(CredentialActive),
	); err != nil {
		return fmt.Errorf("credential: revoke existing: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO kernel_device_runtime_credentials (
			credential_id, credential_hash, user_id, device_id, runtime_id,
			status, created_at, expires_at, last_used_at, revoked_at, revision
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newCred.ID, newCred.CredentialHash,
		newCred.UserID.String(), newCred.DeviceID.String(), newCred.RuntimeID.String(),
		newCred.Status,
		newCred.CreatedAt.UTC().Format(time.RFC3339Nano),
		newCred.ExpiresAt.UTC().Format(time.RFC3339Nano),
		newCred.LastUsedAt.UTC().Format(time.RFC3339Nano),
		formatTimePtr(newCred.RevokedAt),
		newCred.Revision,
	); err != nil {
		return fmt.Errorf("credential: create: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("credential: commit: %w", err)
	}
	return nil
}

func (r *Repository) ListByUser(ctx context.Context, userID runtimeidentity.UserID) ([]*DeviceRuntimeCredential, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT credential_id, credential_hash, user_id, device_id, runtime_id,
			status, created_at, expires_at, last_used_at, revoked_at, revision
		FROM kernel_device_runtime_credentials WHERE user_id = ?`,
		userID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("credential: list by user: %w", err)
	}
	defer rows.Close()

	var creds []*DeviceRuntimeCredential
	for rows.Next() {
		c, err := scanCredential(rows)
		if err != nil {
			return nil, err
		}
		if c != nil {
			creds = append(creds, c)
		}
	}
	return creds, rows.Err()
}

func (r *Repository) RevokeExisting(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID, revokedAt time.Time, reason string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE kernel_device_runtime_credentials
		SET status = ?, revoked_at = ?, revision = revision + 1
		WHERE user_id = ? AND device_id = ? AND runtime_id = ? AND status = ?`,
		string(CredentialRevoked),
		revokedAt.UTC().Format(time.RFC3339Nano),
		userID.String(), deviceID.String(), runtimeID.String(),
		string(CredentialActive),
	)
	if err != nil {
		return fmt.Errorf("credential: revoke existing: %w", err)
	}
	_ = reason
	return nil
}

func (r *Repository) RevokeByID(ctx context.Context, credentialID string, revokedAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE kernel_device_runtime_credentials
		SET status = ?, revoked_at = ?, revision = revision + 1
		WHERE credential_id = ? AND status = ?`,
		string(CredentialRevoked),
		revokedAt.UTC().Format(time.RFC3339Nano),
		credentialID,
		string(CredentialActive),
	)
	if err != nil {
		return fmt.Errorf("credential: revoke by id: %w", err)
	}
	return nil
}

func (r *Repository) RevokeAllForDevice(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, revokedAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE kernel_device_runtime_credentials
		SET status = ?, revoked_at = ?, revision = revision + 1
		WHERE user_id = ? AND device_id = ? AND status = ?`,
		string(CredentialRevoked),
		revokedAt.UTC().Format(time.RFC3339Nano),
		userID.String(), deviceID.String(),
		string(CredentialActive),
	)
	if err != nil {
		return fmt.Errorf("credential: revoke all for device: %w", err)
	}
	return nil
}

func (r *Repository) UpdateLastUsed(ctx context.Context, credentialID string, at time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE kernel_device_runtime_credentials
		SET last_used_at = ?, revision = revision + 1
		WHERE credential_id = ?`,
		at.UTC().Format(time.RFC3339Nano),
		credentialID,
	)
	if err != nil {
		return fmt.Errorf("credential: update last used: %w", err)
	}
	return nil
}

type credentialScanner interface {
	Scan(dest ...any) error
}

func scanCredential(s credentialScanner) (*DeviceRuntimeCredential, error) {
	var c DeviceRuntimeCredential
	var userID, deviceID, runtimeID, status string
	var createdStr, expiresStr, lastUsedStr string
	var revokedStr sql.NullString

	err := s.Scan(
		&c.ID, &c.CredentialHash,
		&userID, &deviceID, &runtimeID,
		&status, &createdStr, &expiresStr, &lastUsedStr, &revokedStr, &c.Revision,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("credential: scan: %w", err)
	}

	c.UserID = runtimeidentity.UserID(userID)
	c.DeviceID = runtimeidentity.DeviceID(deviceID)
	c.RuntimeID = runtimeidentity.RuntimeID(runtimeID)
	c.Status = CredentialStatus(status)

	c.CreatedAt, err = time.Parse(time.RFC3339Nano, createdStr)
	if err != nil {
		return nil, fmt.Errorf("credential: parse created: %w", err)
	}
	c.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresStr)
	if err != nil {
		return nil, fmt.Errorf("credential: parse expires: %w", err)
	}
	c.LastUsedAt, err = time.Parse(time.RFC3339Nano, lastUsedStr)
	if err != nil {
		return nil, fmt.Errorf("credential: parse last used: %w", err)
	}
	if revokedStr.Valid && revokedStr.String != "" {
		revoked, err := time.Parse(time.RFC3339Nano, revokedStr.String)
		if err != nil {
			return nil, fmt.Errorf("credential: parse revoked: %w", err)
		}
		c.RevokedAt = &revoked
	}

	return &c, nil
}

func formatTimePtr(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339Nano), Valid: true}
}
