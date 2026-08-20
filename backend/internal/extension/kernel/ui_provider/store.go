package ui_provider

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrRevisionConflict = errors.New("ui_provider: profile revision conflict")

type ProfileStore interface {
	Load(ctx context.Context) (Profile, bool, error)
	Save(ctx context.Context, profile Profile) error
	LoadLayers(ctx context.Context, scope ProfileScope) ([]Profile, error)
	LoadExact(ctx context.Context, scope ProfileScope) (Profile, bool, error)
	SaveScoped(ctx context.Context, profile Profile, expectedRevision int64) (Profile, error)
	DeleteScope(ctx context.Context, scope ProfileScope, expectedRevision int64) error
}

type SQLiteProfileStore struct{ db *sql.DB }

func NewSQLiteProfileStore(db *sql.DB) *SQLiteProfileStore { return &SQLiteProfileStore{db: db} }

func (s *SQLiteProfileStore) Init(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("ui_provider: profile store database is nil")
	}
	// Keep the v1 table intact so older binaries can still start after rollback.
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS extension_ui_profile (
		profile_id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		selections_json TEXT NOT NULL,
		updated_at INTEGER NOT NULL
	)`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS extension_ui_profiles_v2 (
		scope_key TEXT PRIMARY KEY,
		profile_id TEXT NOT NULL,
		name TEXT NOT NULL,
		user_id TEXT NOT NULL DEFAULT '',
		device_id TEXT NOT NULL DEFAULT '',
		platform TEXT NOT NULL DEFAULT '',
		runtime_profile TEXT NOT NULL DEFAULT '',
		selections_json TEXT NOT NULL,
		revision INTEGER NOT NULL DEFAULT 1,
		updated_at INTEGER NOT NULL
	)`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_extension_ui_profiles_v2_owner
		ON extension_ui_profiles_v2(user_id, device_id, platform, runtime_profile, updated_at)`); err != nil {
		return err
	}
	return s.migrateLegacyProfile(ctx)
}

func (s *SQLiteProfileStore) migrateLegacyProfile(ctx context.Context) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_ui_profiles_v2`).Scan(&count); err != nil || count > 0 {
		return err
	}
	var profileID, name, raw string
	var updatedAt int64
	err := s.db.QueryRowContext(ctx, `SELECT profile_id, name, selections_json, updated_at
		FROM extension_ui_profile ORDER BY updated_at DESC LIMIT 1`).Scan(&profileID, &name, &raw, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO extension_ui_profiles_v2(
		scope_key, profile_id, name, user_id, device_id, platform, runtime_profile, selections_json, revision, updated_at
	) VALUES(?, ?, ?, '', '', '', '', ?, 1, ?)`, globalProfileScope().Key(), profileID, name, raw, updatedAt)
	return err
}

func (s *SQLiteProfileStore) Load(ctx context.Context) (Profile, bool, error) {
	return s.LoadExact(ctx, globalProfileScope())
}

func (s *SQLiteProfileStore) Save(ctx context.Context, p Profile) error {
	p.Scope = globalProfileScope()
	_, err := s.SaveScoped(ctx, p, -1)
	return err
}

func (s *SQLiteProfileStore) LoadLayers(ctx context.Context, scope ProfileScope) ([]Profile, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	keys := scope.LayerKeys()
	if len(keys) == 0 {
		keys = []string{globalProfileScope().Key()}
	}
	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))
	for i, key := range keys {
		placeholders[i] = "?"
		args[i] = key
	}
	rows, err := s.db.QueryContext(ctx, `SELECT scope_key, profile_id, name, user_id, device_id, platform, runtime_profile,
		selections_json, revision, updated_at FROM extension_ui_profiles_v2 WHERE scope_key IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byKey := make(map[string]Profile, len(keys))
	for rows.Next() {
		p, key, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		byKey[key] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Profile, 0, len(keys))
	for _, key := range keys {
		if p, ok := byKey[key]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *SQLiteProfileStore) LoadExact(ctx context.Context, scope ProfileScope) (Profile, bool, error) {
	if s == nil || s.db == nil {
		return Profile{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `SELECT scope_key, profile_id, name, user_id, device_id, platform, runtime_profile,
		selections_json, revision, updated_at FROM extension_ui_profiles_v2 WHERE scope_key = ?`, scope.Normalize().Key())
	p, _, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, false, nil
	}
	if err != nil {
		return Profile{}, false, err
	}
	return p, true, nil
}

func (s *SQLiteProfileStore) SaveScoped(ctx context.Context, p Profile, expectedRevision int64) (Profile, error) {
	if s == nil || s.db == nil {
		return p, nil
	}
	p.Scope = p.Scope.Normalize()
	if p.ProfileID == "" {
		p.ProfileID = "default"
	}
	if p.Name == "" {
		p.Name = p.ProfileID
	}
	if p.Selections == nil {
		p.Selections = map[Capability]string{}
	}
	raw, err := json.Marshal(p.Selections)
	if err != nil {
		return Profile{}, err
	}
	if p.UpdatedAt == 0 {
		p.UpdatedAt = time.Now().UTC().UnixMilli()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Profile{}, err
	}
	defer tx.Rollback()
	var currentRevision int64
	err = tx.QueryRowContext(ctx, `SELECT revision FROM extension_ui_profiles_v2 WHERE scope_key = ?`, p.Scope.Key()).Scan(&currentRevision)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if expectedRevision > 0 {
			return Profile{}, fmt.Errorf("%w: expected %d, current 0", ErrRevisionConflict, expectedRevision)
		}
		p.Revision = 1
	case err != nil:
		return Profile{}, err
	default:
		if expectedRevision >= 0 && expectedRevision != currentRevision {
			return Profile{}, fmt.Errorf("%w: expected %d, current %d", ErrRevisionConflict, expectedRevision, currentRevision)
		}
		p.Revision = currentRevision + 1
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO extension_ui_profiles_v2(
		scope_key, profile_id, name, user_id, device_id, platform, runtime_profile, selections_json, revision, updated_at
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(scope_key) DO UPDATE SET profile_id=excluded.profile_id, name=excluded.name,
		user_id=excluded.user_id, device_id=excluded.device_id, platform=excluded.platform,
		runtime_profile=excluded.runtime_profile, selections_json=excluded.selections_json,
		revision=excluded.revision, updated_at=excluded.updated_at`,
		p.Scope.Key(), p.ProfileID, p.Name, p.Scope.UserID, p.Scope.DeviceID, p.Scope.Platform,
		p.Scope.RuntimeProfile, string(raw), p.Revision, p.UpdatedAt)
	if err != nil {
		return Profile{}, err
	}
	if err := tx.Commit(); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func (s *SQLiteProfileStore) DeleteScope(ctx context.Context, scope ProfileScope, expectedRevision int64) error {
	if s == nil || s.db == nil {
		return nil
	}
	scope = scope.Normalize()
	if scope.Key() == globalProfileScope().Key() {
		return errors.New("ui_provider: global profile cannot be deleted")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if expectedRevision >= 0 {
		var current int64
		err = tx.QueryRowContext(ctx, `SELECT revision FROM extension_ui_profiles_v2 WHERE scope_key = ?`, scope.Key()).Scan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if current != expectedRevision {
			return fmt.Errorf("%w: expected %d, current %d", ErrRevisionConflict, expectedRevision, current)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM extension_ui_profiles_v2 WHERE scope_key = ?`, scope.Key()); err != nil {
		return err
	}
	return tx.Commit()
}

type profileScanner interface{ Scan(dest ...any) error }

func scanProfile(row profileScanner) (Profile, string, error) {
	var p Profile
	var key, raw string
	if err := row.Scan(&key, &p.ProfileID, &p.Name, &p.Scope.UserID, &p.Scope.DeviceID, &p.Scope.Platform,
		&p.Scope.RuntimeProfile, &raw, &p.Revision, &p.UpdatedAt); err != nil {
		return Profile{}, "", err
	}
	if err := json.Unmarshal([]byte(raw), &p.Selections); err != nil {
		return Profile{}, "", fmt.Errorf("decode ui profile selections: %w", err)
	}
	if p.Selections == nil {
		p.Selections = map[Capability]string{}
	}
	return p, key, nil
}
