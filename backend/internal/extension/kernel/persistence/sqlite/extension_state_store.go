package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type ExtensionStateEntry struct {
	ExtensionID string
	ModuleID    string
	Key         string
	Value       json.RawMessage
	Version     int64
	UpdatedAt   time.Time
}

type ExtensionStateStoreRepository struct {
	db *sql.DB
}

func NewExtensionStateStore(db *sql.DB) *ExtensionStateStoreRepository {
	return &ExtensionStateStoreRepository{db: db}
}

func (r *ExtensionStateStoreRepository) Get(ctx context.Context, extensionID string, moduleID string, key string) (json.RawMessage, int64, bool, error) {
	ex := getExecutor(ctx, r.db)
	var value string
	var version int64
	err := ex.QueryRowContext(ctx,
		`SELECT value, version FROM extension_kv_state WHERE extension_id = ? AND module_id = ? AND key = ?`,
		extensionID, moduleID, key,
	).Scan(&value, &version)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, false, nil
		}
		return nil, 0, false, fmt.Errorf("sqlite: query extension kv state: %w", err)
	}
	return json.RawMessage(value), version, true, nil
}

func (r *ExtensionStateStoreRepository) Set(ctx context.Context, extensionID string, moduleID string, key string, value json.RawMessage) (int64, error) {
	now := time.Now().UTC()
	ex := getExecutor(ctx, r.db)
	result, err := ex.ExecContext(ctx, `
		INSERT INTO extension_kv_state (extension_id, module_id, key, value, version, updated_at)
		VALUES (?, ?, ?, ?, 0, ?)
		ON CONFLICT(extension_id, module_id, key) DO UPDATE SET
			value = excluded.value,
			version = extension_kv_state.version + 1,
			updated_at = excluded.updated_at
	`,
		extensionID, moduleID, key, string(value), now,
	)
	if err != nil {
		return 0, fmt.Errorf("sqlite: upsert extension kv state: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		var v int64
		_ = ex.QueryRowContext(ctx,
			`SELECT version FROM extension_kv_state WHERE extension_id = ? AND module_id = ? AND key = ?`,
			extensionID, moduleID, key,
		).Scan(&v)
		return v, nil
	}
	return 0, nil
}

func (r *ExtensionStateStoreRepository) CAS(ctx context.Context, extensionID string, moduleID string, key string, expectedVersion int64, newValue json.RawMessage) (bool, int64, error) {
	ex := getExecutor(ctx, r.db)
	var currentValue string
	var currentVersion int64
	err := ex.QueryRowContext(ctx,
		`SELECT value, version FROM extension_kv_state WHERE extension_id = ? AND module_id = ? AND key = ?`,
		extensionID, moduleID, key,
	).Scan(&currentValue, &currentVersion)
	if err != nil && err != sql.ErrNoRows {
		return false, 0, fmt.Errorf("sqlite: query extension kv state for cas: %w", err)
	}

	if err == sql.ErrNoRows {
		if expectedVersion != 0 {
			return false, 0, nil
		}
		now := time.Now().UTC()
		_, insErr := ex.ExecContext(ctx, `
			INSERT INTO extension_kv_state (extension_id, module_id, key, value, version, updated_at)
			VALUES (?, ?, ?, ?, 0, ?)
		`, extensionID, moduleID, key, string(newValue), now)
		if insErr != nil {
			return false, 0, fmt.Errorf("sqlite: insert extension kv state for cas: %w", insErr)
		}
		return true, 0, nil
	}

	if currentVersion != expectedVersion {
		return false, currentVersion, nil
	}

	now := time.Now().UTC()
	result, updErr := ex.ExecContext(ctx, `
		UPDATE extension_kv_state SET value = ?, version = version + 1, updated_at = ?
		WHERE extension_id = ? AND module_id = ? AND key = ? AND version = ?
	`, string(newValue), now, extensionID, moduleID, key, expectedVersion)
	if updErr != nil {
		return false, currentVersion, fmt.Errorf("sqlite: update extension kv state for cas: %w", updErr)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		var v int64
		_ = ex.QueryRowContext(ctx,
			`SELECT version FROM extension_kv_state WHERE extension_id = ? AND module_id = ? AND key = ?`,
			extensionID, moduleID, key,
		).Scan(&v)
		return false, v, nil
	}
	return true, currentVersion + 1, nil
}

func (r *ExtensionStateStoreRepository) Delete(ctx context.Context, extensionID string, moduleID string, key string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx,
		`DELETE FROM extension_kv_state WHERE extension_id = ? AND module_id = ? AND key = ?`,
		extensionID, moduleID, key,
	)
	if err != nil {
		return fmt.Errorf("sqlite: delete extension kv state: %w", err)
	}
	return nil
}

func (r *ExtensionStateStoreRepository) List(ctx context.Context, extensionID string, moduleID string) ([]ExtensionStateEntry, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx,
		`SELECT extension_id, module_id, key, value, version, updated_at
		 FROM extension_kv_state WHERE extension_id = ? AND module_id = ? ORDER BY key`,
		extensionID, moduleID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list extension kv state: %w", err)
	}
	defer rows.Close()

	var out []ExtensionStateEntry
	for rows.Next() {
		var e ExtensionStateEntry
		var value string
		if err := rows.Scan(&e.ExtensionID, &e.ModuleID, &e.Key, &value, &e.Version, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan extension kv state: %w", err)
		}
		e.Value = json.RawMessage(value)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate extension kv state: %w", err)
	}
	return out, nil
}
