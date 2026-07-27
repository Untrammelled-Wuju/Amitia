package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type PageSessionRecord struct {
	SessionID      string
	ExtensionID    string
	PageID         string
	State          string
	CreatedAt      time.Time
	LastActiveAt   time.Time
	ScopeSnapshot  string
}

type SQLitePageSessionRepository struct {
	db *sql.DB
}

func NewSQLitePageSessionRepository(db *sql.DB) *SQLitePageSessionRepository {
	return &SQLitePageSessionRepository{db: db}
}

func (r *SQLitePageSessionRepository) PutSession(ctx context.Context, rec *PageSessionRecord) error {
	if rec == nil {
		return fmt.Errorf("sqlite: nil page session record")
	}
	now := time.Now().UTC()
	createdAt := rec.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	lastActiveAt := rec.LastActiveAt
	if lastActiveAt.IsZero() {
		lastActiveAt = now
	}
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_page_sessions (
			session_id, extension_id, page_id, state,
			created_at, last_active_at, scope_snapshot
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			extension_id = excluded.extension_id,
			page_id = excluded.page_id,
			state = excluded.state,
			last_active_at = excluded.last_active_at,
			scope_snapshot = excluded.scope_snapshot
	`,
		rec.SessionID,
		rec.ExtensionID,
		rec.PageID,
		rec.State,
		createdAt,
		lastActiveAt,
		rec.ScopeSnapshot,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert page session: %w", err)
	}
	return nil
}

func (r *SQLitePageSessionRepository) GetSession(ctx context.Context, sessionID string) (*PageSessionRecord, error) {
	ex := getExecutor(ctx, r.db)
	var rec PageSessionRecord
	err := ex.QueryRowContext(ctx, `
		SELECT session_id, extension_id, page_id, state, created_at, last_active_at, scope_snapshot
		FROM extension_page_sessions WHERE session_id = ?
	`, sessionID).Scan(
		&rec.SessionID,
		&rec.ExtensionID,
		&rec.PageID,
		&rec.State,
		&rec.CreatedAt,
		&rec.LastActiveAt,
		&rec.ScopeSnapshot,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: page session %s not found: %w", sessionID, err)
		}
		return nil, fmt.Errorf("sqlite: query page session: %w", err)
	}
	return &rec, nil
}

func (r *SQLitePageSessionRepository) ListActiveSessions(ctx context.Context) ([]*PageSessionRecord, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT session_id, extension_id, page_id, state, created_at, last_active_at, scope_snapshot
		FROM extension_page_sessions
		WHERE state NOT IN ('disabled', 'not_installed', 'failed', 'suspended')
		ORDER BY last_active_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list active page sessions: %w", err)
	}
	defer rows.Close()
	var out []*PageSessionRecord
	for rows.Next() {
		var rec PageSessionRecord
		if err := rows.Scan(
			&rec.SessionID,
			&rec.ExtensionID,
			&rec.PageID,
			&rec.State,
			&rec.CreatedAt,
			&rec.LastActiveAt,
			&rec.ScopeSnapshot,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scan page session: %w", err)
		}
		out = append(out, &rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate page sessions: %w", err)
	}
	return out, nil
}

func (r *SQLitePageSessionRepository) DeleteSession(ctx context.Context, sessionID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_page_sessions WHERE session_id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("sqlite: delete page session: %w", err)
	}
	return nil
}

func (r *SQLitePageSessionRepository) DeleteByExtension(ctx context.Context, extensionID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_page_sessions WHERE extension_id = ?`, extensionID)
	if err != nil {
		return fmt.Errorf("sqlite: delete page sessions by extension: %w", err)
	}
	return nil
}
