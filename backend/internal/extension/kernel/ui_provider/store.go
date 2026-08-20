package ui_provider

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type ProfileStore interface {
	Load(ctx context.Context) (Profile, bool, error)
	Save(ctx context.Context, profile Profile) error
}

type SQLiteProfileStore struct{ db *sql.DB }

func NewSQLiteProfileStore(db *sql.DB) *SQLiteProfileStore { return &SQLiteProfileStore{db: db} }

func (s *SQLiteProfileStore) Init(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("ui_provider: profile store database is nil")
	}
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS extension_ui_profile (
		profile_id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		selections_json TEXT NOT NULL,
		updated_at INTEGER NOT NULL
	)`)
	return err
}

func (s *SQLiteProfileStore) Load(ctx context.Context) (Profile, bool, error) {
	if s == nil || s.db == nil {
		return Profile{}, false, nil
	}
	var p Profile
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT profile_id, name, selections_json, updated_at FROM extension_ui_profile ORDER BY updated_at DESC LIMIT 1`).Scan(&p.ProfileID, &p.Name, &raw, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return Profile{}, false, nil
	}
	if err != nil {
		return Profile{}, false, err
	}
	if err := json.Unmarshal([]byte(raw), &p.Selections); err != nil {
		return Profile{}, false, fmt.Errorf("decode ui profile selections: %w", err)
	}
	if p.Selections == nil {
		p.Selections = map[Capability]string{}
	}
	return p, true, nil
}

func (s *SQLiteProfileStore) Save(ctx context.Context, p Profile) error {
	if s == nil || s.db == nil {
		return nil
	}
	raw, err := json.Marshal(p.Selections)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO extension_ui_profile(profile_id, name, selections_json, updated_at)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET name=excluded.name, selections_json=excluded.selections_json, updated_at=excluded.updated_at`,
		p.ProfileID, p.Name, string(raw), p.UpdatedAt)
	return err
}
