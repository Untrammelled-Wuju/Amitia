package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	amitialog "log"
	"sync"
	"time"
)

const (
	DefaultExtensionStateQuotaBytes int64 = 16 << 20
	DefaultExtensionStateQuotaKeys  int64 = 4096
)

var ErrExtensionStateQuotaExceeded = errors.New("sqlite: extension state quota exceeded")

type ExtensionStateEntry struct {
	ExtensionID string          `json:"extensionId"`
	ModuleID    string          `json:"moduleId"`
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	Version     int64           `json:"version"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type ExtensionStateExport struct {
	SchemaVersion int                   `json:"schemaVersion"`
	ExtensionID   string                `json:"extensionId"`
	ExportedAt    time.Time             `json:"exportedAt"`
	Entries       []ExtensionStateEntry `json:"entries"`
}

type ExtensionStateStoreRepository struct {
	db         *sql.DB
	quotaMu    sync.Mutex
	quotaBytes int64
	quotaKeys  int64
}

func NewExtensionStateStore(db *sql.DB) *ExtensionStateStoreRepository {
	return NewExtensionStateStoreWithQuota(db, DefaultExtensionStateQuotaBytes, DefaultExtensionStateQuotaKeys)
}

func NewExtensionStateStoreWithQuota(db *sql.DB, quotaBytes, quotaKeys int64) *ExtensionStateStoreRepository {
	return &ExtensionStateStoreRepository{db: db, quotaBytes: quotaBytes, quotaKeys: quotaKeys}
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
	r.quotaMu.Lock()
	defer r.quotaMu.Unlock()
	if err := r.checkQuotaLocked(ctx, extensionID, moduleID, key, value); err != nil {
		return 0, err
	}
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
		if err := ex.QueryRowContext(ctx,
			`SELECT version FROM extension_kv_state WHERE extension_id = ? AND module_id = ? AND key = ?`,
			extensionID, moduleID, key,
		).Scan(&v); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return v, nil
	}
	return 0, nil
}

func (r *ExtensionStateStoreRepository) CAS(ctx context.Context, extensionID string, moduleID string, key string, expectedVersion int64, newValue json.RawMessage) (bool, int64, error) {
	r.quotaMu.Lock()
	defer r.quotaMu.Unlock()
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
		if err := r.checkQuotaLocked(ctx, extensionID, moduleID, key, newValue); err != nil {
			return false, 0, err
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
	if err := r.checkQuotaLocked(ctx, extensionID, moduleID, key, newValue); err != nil {
		return false, currentVersion, err
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
		if err := ex.QueryRowContext(ctx,
			`SELECT version FROM extension_kv_state WHERE extension_id = ? AND module_id = ? AND key = ?`,
			extensionID, moduleID, key,
		).Scan(&v); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
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

func (r *ExtensionStateStoreRepository) DeleteByExtension(ctx context.Context, extensionID string) error {
	ex := getExecutor(ctx, r.db)
	if _, err := ex.ExecContext(ctx, `DELETE FROM extension_kv_state WHERE extension_id = ?`, extensionID); err != nil {
		return fmt.Errorf("sqlite: delete extension kv state by extension: %w", err)
	}
	return nil
}

func (r *ExtensionStateStoreRepository) Usage(ctx context.Context, extensionID string) (int, int64, error) {
	ex := getExecutor(ctx, r.db)
	var keys int
	var totalBytes int64
	err := ex.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(LENGTH(CAST(value AS BLOB))), 0)
		 FROM extension_kv_state WHERE extension_id = ?`,
		extensionID,
	).Scan(&keys, &totalBytes)
	if err != nil {
		return 0, 0, fmt.Errorf("sqlite: query extension kv state usage: %w", err)
	}
	return keys, totalBytes, nil
}

func (r *ExtensionStateStoreRepository) Export(ctx context.Context, extensionID string) ([]byte, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx,
		`SELECT extension_id, module_id, key, value, version, updated_at
		 FROM extension_kv_state WHERE extension_id = ? ORDER BY module_id, key`,
		extensionID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: export extension kv state: %w", err)
	}
	defer rows.Close()
	document := ExtensionStateExport{
		SchemaVersion: 1,
		ExtensionID:   extensionID,
		ExportedAt:    time.Now().UTC(),
		Entries:       []ExtensionStateEntry{},
	}
	for rows.Next() {
		var entry ExtensionStateEntry
		var value string
		if err := rows.Scan(&entry.ExtensionID, &entry.ModuleID, &entry.Key, &value, &entry.Version, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan exported extension kv state: %w", err)
		}
		entry.Value = json.RawMessage(value)
		document.Entries = append(document.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate exported extension kv state: %w", err)
	}
	payload, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("sqlite: encode exported extension kv state: %w", err)
	}
	return payload, nil
}

func (r *ExtensionStateStoreRepository) checkQuotaLocked(ctx context.Context, extensionID, moduleID, key string, value json.RawMessage) error {
	if r.quotaBytes <= 0 && r.quotaKeys <= 0 {
		return nil
	}
	ex := getExecutor(ctx, r.db)
	var keys int
	var totalBytes int64
	var existingBytes int64
	var existingCount int
	err := ex.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(LENGTH(CAST(value AS BLOB))), 0),
		        COALESCE(MAX(CASE WHEN module_id = ? AND key = ? THEN LENGTH(CAST(value AS BLOB)) ELSE 0 END), 0),
		        COUNT(CASE WHEN module_id = ? AND key = ? THEN 1 END)
		 FROM extension_kv_state WHERE extension_id = ?`,
		moduleID, key, moduleID, key, extensionID,
	).Scan(&keys, &totalBytes, &existingBytes, &existingCount)
	if err != nil {
		return fmt.Errorf("sqlite: query extension kv state quota: %w", err)
	}
	projectedBytes := totalBytes - existingBytes + int64(len(value))
	if r.quotaBytes > 0 && projectedBytes > r.quotaBytes {
		return ErrExtensionStateQuotaExceeded
	}
	projectedKeys := keys
	if existingCount == 0 {
		projectedKeys++
	}
	if r.quotaKeys > 0 && int64(projectedKeys) > r.quotaKeys {
		return ErrExtensionStateQuotaExceeded
	}
	return nil
}
