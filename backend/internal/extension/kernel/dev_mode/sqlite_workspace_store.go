package dev_mode

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	workspaceSelectColumns = `workspace_id, extension_id, path, manifest_path, current_revision, status, watch_enabled, auto_reload, dev_trust_granted, created_at, updated_at`
)

type SQLiteWorkspaceStore struct {
	db *sql.DB
}

func NewSQLiteWorkspaceStore(db *sql.DB) *SQLiteWorkspaceStore {
	return &SQLiteWorkspaceStore{db: db}
}

func (s *SQLiteWorkspaceStore) Save(ctx context.Context, ws DevelopmentWorkspace) error {
	if ws.WorkspaceID == "" {
		return fmt.Errorf("%w: empty workspace id", ErrInvalidWorkspaceInput)
	}
	if ws.ExtensionID == "" {
		return fmt.Errorf("%w: empty extension id", ErrInvalidWorkspaceInput)
	}
	if ws.PathReference == "" {
		return fmt.Errorf("%w: empty path", ErrInvalidWorkspaceInput)
	}
	if ws.ManifestPath == "" {
		return fmt.Errorf("%w: empty manifest path", ErrInvalidWorkspaceInput)
	}

	now := time.Now().UTC()
	if ws.CreatedAt.IsZero() {
		ws.CreatedAt = now
	}
	ws.UpdatedAt = now

	var currentRev sql.NullString
	if ws.CurrentRevision != "" {
		currentRev = sql.NullString{String: string(ws.CurrentRevision), Valid: true}
	}

	watchEnabled := 0
	if ws.WatchEnabled {
		watchEnabled = 1
	}
	autoReload := 0
	if ws.AutoReload {
		autoReload = 1
	}
	devTrust := 0
	if ws.DevTrust {
		devTrust = 1
	}

	status := ws.Status
	if status == "" {
		status = WorkspaceStatusRegistered
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_dev_workspaces
		(workspace_id, extension_id, path, manifest_path, current_revision, status, watch_enabled, auto_reload, dev_trust_granted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id) DO UPDATE SET
			extension_id = excluded.extension_id,
			path = excluded.path,
			manifest_path = excluded.manifest_path,
			current_revision = excluded.current_revision,
			status = excluded.status,
			watch_enabled = excluded.watch_enabled,
			auto_reload = excluded.auto_reload,
			dev_trust_granted = excluded.dev_trust_granted,
			updated_at = excluded.updated_at
	`,
		string(ws.WorkspaceID),
		string(ws.ExtensionID),
		ws.PathReference,
		ws.ManifestPath,
		currentRev,
		string(status),
		watchEnabled,
		autoReload,
		devTrust,
		ws.CreatedAt,
		ws.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: save workspace %s: %w", ws.WorkspaceID, err)
	}
	return nil
}

func (s *SQLiteWorkspaceStore) Get(ctx context.Context, id WorkspaceID) (DevelopmentWorkspace, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+workspaceSelectColumns+`
		FROM kernel_dev_workspaces WHERE workspace_id = ?
	`, string(id))
	ws, err := scanWorkspace(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return DevelopmentWorkspace{}, fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
		}
		return DevelopmentWorkspace{}, fmt.Errorf("sqlite: get workspace %s: %w", id, err)
	}
	return ws, nil
}

func (s *SQLiteWorkspaceStore) GetByExtension(ctx context.Context, extID ExtensionID) (DevelopmentWorkspace, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+workspaceSelectColumns+`
		FROM kernel_dev_workspaces WHERE extension_id = ?
	`, string(extID))
	ws, err := scanWorkspace(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return DevelopmentWorkspace{}, fmt.Errorf("%w: %s", ErrWorkspaceNotFound, extID)
		}
		return DevelopmentWorkspace{}, fmt.Errorf("sqlite: get workspace by extension %s: %w", extID, err)
	}
	return ws, nil
}

func (s *SQLiteWorkspaceStore) List(ctx context.Context) ([]DevelopmentWorkspace, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+workspaceSelectColumns+`
		FROM kernel_dev_workspaces ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list workspaces: %w", err)
	}
	defer rows.Close()

	var out []DevelopmentWorkspace
	for rows.Next() {
		ws, err := scanWorkspace(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan workspace: %w", err)
		}
		out = append(out, ws)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate workspaces: %w", err)
	}
	return out, nil
}

func (s *SQLiteWorkspaceStore) UpdateStatus(ctx context.Context, id WorkspaceID, status WorkspaceStatus) error {
	if status == "" {
		return fmt.Errorf("%w: empty status", ErrInvalidWorkspaceInput)
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE kernel_dev_workspaces SET status = ?, updated_at = ? WHERE workspace_id = ?
	`, string(status), now, string(id))
	if err != nil {
		return fmt.Errorf("sqlite: update status %s: %w", id, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
	}
	return nil
}

func (s *SQLiteWorkspaceStore) SetCurrentRevision(ctx context.Context, id WorkspaceID, revision RevisionID) error {
	now := time.Now().UTC()
	var revVal sql.NullString
	if revision != "" {
		revVal = sql.NullString{String: string(revision), Valid: true}
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE kernel_dev_workspaces SET current_revision = ?, updated_at = ? WHERE workspace_id = ?
	`, revVal, now, string(id))
	if err != nil {
		return fmt.Errorf("sqlite: set current revision %s: %w", id, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
	}
	return nil
}

func (s *SQLiteWorkspaceStore) GrantDevTrust(ctx context.Context, id WorkspaceID) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE kernel_dev_workspaces SET dev_trust_granted = 1, updated_at = ? WHERE workspace_id = ?
	`, now, string(id))
	if err != nil {
		return fmt.Errorf("sqlite: grant dev trust %s: %w", id, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
	}
	return nil
}

func (s *SQLiteWorkspaceStore) RevokeDevTrust(ctx context.Context, id WorkspaceID) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE kernel_dev_workspaces SET dev_trust_granted = 0, updated_at = ? WHERE workspace_id = ?
	`, now, string(id))
	if err != nil {
		return fmt.Errorf("sqlite: revoke dev trust %s: %w", id, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
	}
	return nil
}

func (s *SQLiteWorkspaceStore) Remove(ctx context.Context, id WorkspaceID) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM kernel_dev_workspaces WHERE workspace_id = ?
	`, string(id))
	if err != nil {
		return fmt.Errorf("sqlite: remove workspace %s: %w", id, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanWorkspace(row rowScanner) (DevelopmentWorkspace, error) {
	var ws DevelopmentWorkspace
	var workspaceID, extensionID, pathRef, manifestPath, status string
	var currentRev sql.NullString
	var watchEnabled, autoReload, devTrust int

	err := row.Scan(
		&workspaceID,
		&extensionID,
		&pathRef,
		&manifestPath,
		&currentRev,
		&status,
		&watchEnabled,
		&autoReload,
		&devTrust,
		&ws.CreatedAt,
		&ws.UpdatedAt,
	)
	if err != nil {
		return DevelopmentWorkspace{}, err
	}
	ws.WorkspaceID = WorkspaceID(workspaceID)
	ws.ExtensionID = ExtensionID(extensionID)
	ws.PathReference = pathRef
	ws.ManifestPath = manifestPath
	ws.Status = WorkspaceStatus(status)
	if currentRev.Valid {
		ws.CurrentRevision = RevisionID(currentRev.String)
	}
	ws.WatchEnabled = watchEnabled != 0
	ws.AutoReload = autoReload != 0
	ws.DevTrust = devTrust != 0
	return ws, nil
}
