package deviceruntime

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type SQLiteSessionStore struct {
	db *sql.DB
}

func NewSQLiteSessionStore(db *sql.DB) *SQLiteSessionStore {
	return &SQLiteSessionStore{db: db}
}

func (s *SQLiteSessionStore) EnsureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS kernel_device_runtime_sessions (
    runtime_session_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    connection_generation INTEGER NOT NULL DEFAULT 1,
    revision INTEGER NOT NULL DEFAULT 1,
    runtime_version TEXT NOT NULL DEFAULT '',
    runtime_contract_version TEXT NOT NULL DEFAULT '',
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    capabilities_hash TEXT NOT NULL DEFAULT '',
    last_applied_state_revision INTEGER NOT NULL DEFAULT 0,
    last_processed_command_sequence INTEGER NOT NULL DEFAULT 0,
    last_event_sequence INTEGER NOT NULL DEFAULT 0,
    actual_state_hash TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    last_heartbeat_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL DEFAULT 0,
    closed_at INTEGER NOT NULL DEFAULT 0,
    close_reason TEXT NOT NULL DEFAULT ''
)`,
		`CREATE INDEX IF NOT EXISTS idx_kernel_device_runtime_sessions_identity
    ON kernel_device_runtime_sessions(user_id, device_id, runtime_id)`,
		`CREATE INDEX IF NOT EXISTS idx_kernel_device_runtime_sessions_status
    ON kernel_device_runtime_sessions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_kernel_device_runtime_sessions_heartbeat
    ON kernel_device_runtime_sessions(last_heartbeat_at)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("deviceruntime: ensure schema: %w", err)
		}
	}
	return nil
}

func (s *SQLiteSessionStore) Create(ctx context.Context, session RuntimeSession) error {
	capsJSON, err := json.Marshal(session.Capabilities)
	if err != nil {
		return fmt.Errorf("deviceruntime: marshal capabilities: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO kernel_device_runtime_sessions (
        runtime_session_id, user_id, device_id, runtime_id, platform,
        status, connection_generation, revision, runtime_version, runtime_contract_version,
        capabilities_json, capabilities_hash,
        last_applied_state_revision, last_processed_command_sequence, last_event_sequence, actual_state_hash,
        created_at, updated_at, last_heartbeat_at, expires_at, closed_at, close_reason
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID.String(),
		session.UserID.String(),
		session.DeviceID.String(),
		session.RuntimeID.String(),
		session.Platform.String(),
		string(session.Status),
		session.ConnectionGeneration,
		session.Revision,
		session.RuntimeVersion,
		session.RuntimeContractVersion,
		string(capsJSON),
		session.CapabilitiesHash,
		session.LastAppliedStateRevision,
		session.LastProcessedCommandSequence,
		session.LastEventSequence,
		session.ActualStateHash,
		session.CreatedAt.UnixMilli(),
		session.UpdatedAt.UnixMilli(),
		session.LastHeartbeatAt.UnixMilli(),
		session.ExpiresAt.UnixMilli(),
		0,
		"",
	)
	return err
}

func (s *SQLiteSessionStore) Get(ctx context.Context, sessionID runtimeidentity.RuntimeSessionID) (RuntimeSession, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT runtime_session_id, user_id, device_id, runtime_id, platform,
        status, connection_generation, revision, runtime_version, runtime_contract_version,
        capabilities_json, capabilities_hash,
        last_applied_state_revision, last_processed_command_sequence, last_event_sequence, actual_state_hash,
        created_at, updated_at, last_heartbeat_at, expires_at, closed_at, close_reason
    FROM kernel_device_runtime_sessions WHERE runtime_session_id = ?`,
		sessionID.String(),
	)
	return scanRuntimeSession(row)
}

func (s *SQLiteSessionStore) GetActiveByRuntime(
	ctx context.Context,
	userID runtimeidentity.UserID,
	deviceID runtimeidentity.DeviceID,
	runtimeID runtimeidentity.RuntimeID,
) (RuntimeSession, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT runtime_session_id, user_id, device_id, runtime_id, platform,
        status, connection_generation, revision, runtime_version, runtime_contract_version,
        capabilities_json, capabilities_hash,
        last_applied_state_revision, last_processed_command_sequence, last_event_sequence, actual_state_hash,
        created_at, updated_at, last_heartbeat_at, expires_at, closed_at, close_reason
    FROM kernel_device_runtime_sessions
    WHERE user_id = ? AND device_id = ? AND runtime_id = ? AND status IN (?, ?, ?, ?)
    ORDER BY connection_generation DESC LIMIT 1`,
		userID.String(), deviceID.String(), runtimeID.String(),
		string(protocol.SessionStatusRegistering),
		string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady),
		string(protocol.SessionStatusDegraded),
	)
	return scanRuntimeSession(row)
}

func (s *SQLiteSessionStore) Update(ctx context.Context, session RuntimeSession) error {
	capsJSON, err := json.Marshal(session.Capabilities)
	if err != nil {
		return fmt.Errorf("deviceruntime: marshal capabilities: %w", err)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET
        status = ?, connection_generation = ?, revision = ?, platform = ?,
        runtime_version = ?, runtime_contract_version = ?,
        capabilities_json = ?, capabilities_hash = ?,
        last_applied_state_revision = ?, last_processed_command_sequence = ?, last_event_sequence = ?, actual_state_hash = ?,
        updated_at = ?, last_heartbeat_at = ?, expires_at = ?, closed_at = ?, close_reason = ?
    WHERE runtime_session_id = ?`,
		string(session.Status),
		session.ConnectionGeneration,
		session.Revision,
		session.Platform.String(),
		session.RuntimeVersion,
		session.RuntimeContractVersion,
		string(capsJSON),
		session.CapabilitiesHash,
		session.LastAppliedStateRevision,
		session.LastProcessedCommandSequence,
		session.LastEventSequence,
		session.ActualStateHash,
		session.UpdatedAt.UnixMilli(),
		session.LastHeartbeatAt.UnixMilli(),
		session.ExpiresAt.UnixMilli(),
		sessionClosedAtUnix(session),
		session.CloseReason,
		session.ID.String(),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrRuntimeSessionNotFound
	}
	return nil
}

func (s *SQLiteSessionStore) ListActive(ctx context.Context) ([]RuntimeSession, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT runtime_session_id, user_id, device_id, runtime_id, platform,
        status, connection_generation, revision, runtime_version, runtime_contract_version,
        capabilities_json, capabilities_hash,
        last_applied_state_revision, last_processed_command_sequence, last_event_sequence, actual_state_hash,
        created_at, updated_at, last_heartbeat_at, expires_at, closed_at, close_reason
    FROM kernel_device_runtime_sessions WHERE status IN (?, ?, ?, ?)`,
		string(protocol.SessionStatusRegistering),
		string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady),
		string(protocol.SessionStatusDegraded),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []RuntimeSession
	for rows.Next() {
		session, err := scanRuntimeSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (s *SQLiteSessionStore) CloseActiveOnStartup(ctx context.Context, at time.Time, reason string) error {
	atMs := at.UnixMilli()
	_, err := s.db.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET
        status = ?, close_reason = ?, closed_at = ?, updated_at = ?
    WHERE status IN (?, ?, ?, ?)`,
		string(protocol.SessionStatusClosed),
		reason,
		atMs,
		atMs,
		string(protocol.SessionStatusRegistering),
		string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady),
		string(protocol.SessionStatusDegraded),
	)
	return err
}

func (s *SQLiteSessionStore) UpdateHeartbeat(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	generation int64,
	at time.Time,
	expiresAt time.Time,
) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET
        last_heartbeat_at = ?, expires_at = ?, updated_at = ?
    WHERE runtime_session_id = ? AND connection_generation = ? AND status IN (?, ?, ?, ?)`,
		at.UnixMilli(),
		expiresAt.UnixMilli(),
		at.UnixMilli(),
		sessionID.String(),
		generation,
		string(protocol.SessionStatusRegistering),
		string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady),
		string(protocol.SessionStatusDegraded),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrRuntimeSessionNotFound
	}
	return nil
}

func (s *SQLiteSessionStore) UpdateCursor(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	generation int64,
	cursor protocol.SessionCursor,
	at time.Time,
) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET
        last_applied_state_revision = ?,
        last_processed_command_sequence = ?,
        last_event_sequence = ?,
        actual_state_hash = ?,
        updated_at = ?
    WHERE runtime_session_id = ? AND connection_generation = ? AND status IN (?, ?, ?, ?)`,
		cursor.LastAppliedStateRevision,
		cursor.LastProcessedCommandSequence,
		cursor.LastEventSequence,
		cursor.ActualStateHash,
		at.UnixMilli(),
		sessionID.String(),
		generation,
		string(protocol.SessionStatusRegistering),
		string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady),
		string(protocol.SessionStatusDegraded),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrRuntimeSessionNotFound
	}
	return nil
}

func (s *SQLiteSessionStore) UpdateStatus(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	generation int64,
	status protocol.SessionStatus,
	at time.Time,
) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET status = ?, revision = revision + 1, updated_at = ?
    WHERE runtime_session_id = ? AND connection_generation = ? AND status IN (?, ?, ?, ?)`,
		string(status),
		at.UnixMilli(),
		sessionID.String(),
		generation,
		string(protocol.SessionStatusRegistering),
		string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady),
		string(protocol.SessionStatusDegraded),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrRuntimeSessionNotFound
	}
	return nil
}

func (s *SQLiteSessionStore) Close(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	generation int64,
	reason string,
	at time.Time,
) error {
	var closedAt *time.Time
	if reason != "" {
		t := at
		closedAt = &t
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET
        status = ?, revision = revision + 1, close_reason = ?, closed_at = ?, updated_at = ?
    WHERE runtime_session_id = ? AND connection_generation = ? AND status NOT IN (?, ?)`,
		string(protocol.SessionStatusClosed),
		reason,
		unixMillisFromTime(closedAt),
		at.UnixMilli(),
		sessionID.String(),
		generation,
		string(protocol.SessionStatusClosed),
		string(protocol.SessionStatusSuperseded),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrRuntimeSessionNotFound
	}
	return nil
}

func (s *SQLiteSessionStore) ReplaceForReconnect(
	ctx context.Context,
	expectedGeneration int64,
	updated RuntimeSession,
) error {
	capsJSON, err := json.Marshal(updated.Capabilities)
	if err != nil {
		return fmt.Errorf("deviceruntime: marshal capabilities: %w", err)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET
        status = ?, connection_generation = ?, revision = ?, platform = ?,
        runtime_version = ?, runtime_contract_version = ?,
        capabilities_json = ?, capabilities_hash = ?,
        last_applied_state_revision = ?, last_processed_command_sequence = ?, last_event_sequence = ?, actual_state_hash = ?,
        last_heartbeat_at = ?, expires_at = ?, updated_at = ?
    WHERE runtime_session_id = ? AND connection_generation = ? AND status IN (?, ?, ?, ?)`,
		string(updated.Status),
		updated.ConnectionGeneration,
		updated.Revision,
		updated.Platform.String(),
		updated.RuntimeVersion,
		updated.RuntimeContractVersion,
		string(capsJSON),
		updated.CapabilitiesHash,
		updated.LastAppliedStateRevision,
		updated.LastProcessedCommandSequence,
		updated.LastEventSequence,
		updated.ActualStateHash,
		updated.LastHeartbeatAt.UnixMilli(),
		updated.ExpiresAt.UnixMilli(),
		updated.UpdatedAt.UnixMilli(),
		updated.ID.String(),
		expectedGeneration,
		string(protocol.SessionStatusRegistering),
		string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady),
		string(protocol.SessionStatusDegraded),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrConnectionSuperseded
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanRuntimeSession(row rowScanner) (RuntimeSession, error) {
	var session RuntimeSession
	var capsJSON string
	var createdAtMs, updatedAtMs, heartbeatMs, expiresAtMs, closedAtMs int64
	var statusStr, platformStr, userIDStr, deviceIDStr, runtimeIDStr string

	err := row.Scan(
		&session.ID,
		&userIDStr,
		&deviceIDStr,
		&runtimeIDStr,
		&platformStr,
		&statusStr,
		&session.ConnectionGeneration,
		&session.Revision,
		&session.RuntimeVersion,
		&session.RuntimeContractVersion,
		&capsJSON,
		&session.CapabilitiesHash,
		&session.LastAppliedStateRevision,
		&session.LastProcessedCommandSequence,
		&session.LastEventSequence,
		&session.ActualStateHash,
		&createdAtMs,
		&updatedAtMs,
		&heartbeatMs,
		&expiresAtMs,
		&closedAtMs,
		&session.CloseReason,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RuntimeSession{}, ErrRuntimeSessionNotFound
		}
		return RuntimeSession{}, err
	}

	session.UserID = runtimeidentity.ParseUserID(userIDStr)
	session.DeviceID = runtimeidentity.ParseDeviceID(deviceIDStr)
	session.RuntimeID = runtimeidentity.ParseRuntimeID(runtimeIDStr)
	platform, err := runtimeidentity.ParsePlatform(platformStr)
	if err != nil {
		return RuntimeSession{}, fmt.Errorf("deviceruntime: parse platform: %w", err)
	}
	session.Platform = platform
	session.Status = protocol.SessionStatus(statusStr)
	session.CreatedAt = time.UnixMilli(createdAtMs).UTC()
	session.UpdatedAt = time.UnixMilli(updatedAtMs).UTC()
	session.LastHeartbeatAt = time.UnixMilli(heartbeatMs).UTC()
	session.ExpiresAt = time.UnixMilli(expiresAtMs).UTC()
	if closedAtMs > 0 {
		t := time.UnixMilli(closedAtMs).UTC()
		session.ClosedAt = &t
	}

	if capsJSON != "" {
		var caps []string
		if err := json.Unmarshal([]byte(capsJSON), &caps); err != nil {
			return RuntimeSession{}, fmt.Errorf("deviceruntime: unmarshal capabilities: %w", err)
		}
		session.Capabilities = caps
	}

	return session, nil
}

func sessionClosedAtUnix(s RuntimeSession) int64 {
	if s.ClosedAt == nil {
		return 0
	}
	return s.ClosedAt.UnixMilli()
}

func unixMillisFromTime(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.UnixMilli()
}

func (s *SQLiteSessionStore) WithinTx(ctx context.Context, fn func(SessionTx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	storeTx := &SQLiteSessionStoreTx{tx: tx}
	if err := fn(storeTx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

type SQLiteSessionStoreTx struct {
	tx *sql.Tx
}

func (t *SQLiteSessionStoreTx) RawTx() *sql.Tx { return t.tx }

func (t *SQLiteSessionStoreTx) Create(ctx context.Context, session RuntimeSession) error {
	capsJSON, err := json.Marshal(session.Capabilities)
	if err != nil {
		return fmt.Errorf("deviceruntime: marshal capabilities: %w", err)
	}
	_, err = t.tx.ExecContext(ctx,
		`INSERT INTO kernel_device_runtime_sessions (
        runtime_session_id, user_id, device_id, runtime_id, platform,
        status, connection_generation, revision, runtime_version, runtime_contract_version,
        capabilities_json, capabilities_hash,
        last_applied_state_revision, last_processed_command_sequence, last_event_sequence, actual_state_hash,
        created_at, updated_at, last_heartbeat_at, expires_at, closed_at, close_reason
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID.String(), session.UserID.String(), session.DeviceID.String(), session.RuntimeID.String(),
		session.Platform.String(), string(session.Status), session.ConnectionGeneration, session.Revision,
		session.RuntimeVersion, session.RuntimeContractVersion, string(capsJSON), session.CapabilitiesHash,
		session.LastAppliedStateRevision, session.LastProcessedCommandSequence, session.LastEventSequence, session.ActualStateHash,
		session.CreatedAt.UnixMilli(), session.UpdatedAt.UnixMilli(), session.LastHeartbeatAt.UnixMilli(),
		session.ExpiresAt.UnixMilli(), 0, "",
	)
	return err
}

func (t *SQLiteSessionStoreTx) Get(ctx context.Context, sessionID runtimeidentity.RuntimeSessionID) (RuntimeSession, error) {
	row := t.tx.QueryRowContext(ctx,
		`SELECT runtime_session_id, user_id, device_id, runtime_id, platform,
        status, connection_generation, revision, runtime_version, runtime_contract_version,
        capabilities_json, capabilities_hash,
        last_applied_state_revision, last_processed_command_sequence, last_event_sequence, actual_state_hash,
        created_at, updated_at, last_heartbeat_at, expires_at, closed_at, close_reason
    FROM kernel_device_runtime_sessions WHERE runtime_session_id = ?`,
		sessionID.String(),
	)
	return scanRuntimeSession(row)
}

func (t *SQLiteSessionStoreTx) GetActiveByRuntime(
	ctx context.Context,
	userID runtimeidentity.UserID,
	deviceID runtimeidentity.DeviceID,
	runtimeID runtimeidentity.RuntimeID,
) (RuntimeSession, error) {
	row := t.tx.QueryRowContext(ctx,
		`SELECT runtime_session_id, user_id, device_id, runtime_id, platform,
        status, connection_generation, revision, runtime_version, runtime_contract_version,
        capabilities_json, capabilities_hash,
        last_applied_state_revision, last_processed_command_sequence, last_event_sequence, actual_state_hash,
        created_at, updated_at, last_heartbeat_at, expires_at, closed_at, close_reason
    FROM kernel_device_runtime_sessions
    WHERE user_id = ? AND device_id = ? AND runtime_id = ? AND status IN (?, ?, ?, ?)
    ORDER BY connection_generation DESC LIMIT 1`,
		userID.String(), deviceID.String(), runtimeID.String(),
		string(protocol.SessionStatusRegistering), string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady), string(protocol.SessionStatusDegraded),
	)
	return scanRuntimeSession(row)
}

func (t *SQLiteSessionStoreTx) Update(ctx context.Context, session RuntimeSession) error {
	capsJSON, err := json.Marshal(session.Capabilities)
	if err != nil {
		return fmt.Errorf("deviceruntime: marshal capabilities: %w", err)
	}
	res, err := t.tx.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET
        status = ?, connection_generation = ?, revision = ?, platform = ?,
        runtime_version = ?, runtime_contract_version = ?,
        capabilities_json = ?, capabilities_hash = ?,
        last_applied_state_revision = ?, last_processed_command_sequence = ?, last_event_sequence = ?, actual_state_hash = ?,
        updated_at = ?, last_heartbeat_at = ?, expires_at = ?, closed_at = ?, close_reason = ?
    WHERE runtime_session_id = ?`,
		string(session.Status), session.ConnectionGeneration, session.Revision, session.Platform.String(),
		session.RuntimeVersion, session.RuntimeContractVersion, string(capsJSON), session.CapabilitiesHash,
		session.LastAppliedStateRevision, session.LastProcessedCommandSequence, session.LastEventSequence, session.ActualStateHash,
		session.UpdatedAt.UnixMilli(), session.LastHeartbeatAt.UnixMilli(), session.ExpiresAt.UnixMilli(),
		sessionClosedAtUnix(session), session.CloseReason, session.ID.String(),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrRuntimeSessionNotFound
	}
	return nil
}

func (t *SQLiteSessionStoreTx) ListActive(ctx context.Context) ([]RuntimeSession, error) {
	rows, err := t.tx.QueryContext(ctx,
		`SELECT runtime_session_id, user_id, device_id, runtime_id, platform,
        status, connection_generation, revision, runtime_version, runtime_contract_version,
        capabilities_json, capabilities_hash,
        last_applied_state_revision, last_processed_command_sequence, last_event_sequence, actual_state_hash,
        created_at, updated_at, last_heartbeat_at, expires_at, closed_at, close_reason
    FROM kernel_device_runtime_sessions WHERE status IN (?, ?, ?, ?)`,
		string(protocol.SessionStatusRegistering), string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady), string(protocol.SessionStatusDegraded),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []RuntimeSession
	for rows.Next() {
		session, err := scanRuntimeSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (t *SQLiteSessionStoreTx) CloseActiveOnStartup(ctx context.Context, at time.Time, reason string) error {
	atMs := at.UnixMilli()
	_, err := t.tx.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET
        status = ?, close_reason = ?, closed_at = ?, updated_at = ?
    WHERE status IN (?, ?, ?, ?)`,
		string(protocol.SessionStatusClosed), reason, atMs, atMs,
		string(protocol.SessionStatusRegistering), string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady), string(protocol.SessionStatusDegraded),
	)
	return err
}

func (t *SQLiteSessionStoreTx) UpdateHeartbeat(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	generation int64,
	at time.Time,
	expiresAt time.Time,
) error {
	res, err := t.tx.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET
        last_heartbeat_at = ?, expires_at = ?, updated_at = ?
    WHERE runtime_session_id = ? AND connection_generation = ? AND status IN (?, ?, ?, ?)`,
		at.UnixMilli(), expiresAt.UnixMilli(), at.UnixMilli(), sessionID.String(), generation,
		string(protocol.SessionStatusRegistering), string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady), string(protocol.SessionStatusDegraded),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrRuntimeSessionNotFound
	}
	return nil
}

func (t *SQLiteSessionStoreTx) UpdateCursor(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	generation int64,
	cursor protocol.SessionCursor,
	at time.Time,
) error {
	res, err := t.tx.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET
        last_applied_state_revision = ?, last_processed_command_sequence = ?,
        last_event_sequence = ?, actual_state_hash = ?, updated_at = ?
    WHERE runtime_session_id = ? AND connection_generation = ? AND status IN (?, ?, ?, ?)`,
		cursor.LastAppliedStateRevision, cursor.LastProcessedCommandSequence, cursor.LastEventSequence,
		cursor.ActualStateHash, at.UnixMilli(), sessionID.String(), generation,
		string(protocol.SessionStatusRegistering), string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady), string(protocol.SessionStatusDegraded),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrRuntimeSessionNotFound
	}
	return nil
}

func (t *SQLiteSessionStoreTx) UpdateStatus(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	generation int64,
	status protocol.SessionStatus,
	at time.Time,
) error {
	res, err := t.tx.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET status = ?, revision = revision + 1, updated_at = ?
    WHERE runtime_session_id = ? AND connection_generation = ? AND status IN (?, ?, ?, ?)`,
		string(status), at.UnixMilli(), sessionID.String(), generation,
		string(protocol.SessionStatusRegistering), string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady), string(protocol.SessionStatusDegraded),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrRuntimeSessionNotFound
	}
	return nil
}

func (t *SQLiteSessionStoreTx) Close(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	generation int64,
	reason string,
	at time.Time,
) error {
	var closedAt *time.Time
	if reason != "" {
		t := at
		closedAt = &t
	}
	res, err := t.tx.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET
        status = ?, revision = revision + 1, close_reason = ?, closed_at = ?, updated_at = ?
    WHERE runtime_session_id = ? AND connection_generation = ? AND status NOT IN (?, ?)`,
		string(protocol.SessionStatusClosed), reason, unixMillisFromTime(closedAt), at.UnixMilli(),
		sessionID.String(), generation,
		string(protocol.SessionStatusClosed), string(protocol.SessionStatusSuperseded),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrRuntimeSessionNotFound
	}
	return nil
}

func (t *SQLiteSessionStoreTx) ReplaceForReconnect(
	ctx context.Context,
	expectedGeneration int64,
	updated RuntimeSession,
) error {
	capsJSON, err := json.Marshal(updated.Capabilities)
	if err != nil {
		return fmt.Errorf("deviceruntime: marshal capabilities: %w", err)
	}
	res, err := t.tx.ExecContext(ctx,
		`UPDATE kernel_device_runtime_sessions SET
        status = ?, connection_generation = ?, revision = ?, platform = ?,
        runtime_version = ?, runtime_contract_version = ?,
        capabilities_json = ?, capabilities_hash = ?,
        last_applied_state_revision = ?, last_processed_command_sequence = ?, last_event_sequence = ?, actual_state_hash = ?,
        last_heartbeat_at = ?, expires_at = ?, updated_at = ?
    WHERE runtime_session_id = ? AND connection_generation = ? AND status IN (?, ?, ?, ?)`,
		string(updated.Status), updated.ConnectionGeneration, updated.Revision, updated.Platform.String(),
		updated.RuntimeVersion, updated.RuntimeContractVersion, string(capsJSON), updated.CapabilitiesHash,
		updated.LastAppliedStateRevision, updated.LastProcessedCommandSequence, updated.LastEventSequence, updated.ActualStateHash,
		updated.LastHeartbeatAt.UnixMilli(), updated.ExpiresAt.UnixMilli(), updated.UpdatedAt.UnixMilli(),
		updated.ID.String(), expectedGeneration,
		string(protocol.SessionStatusRegistering), string(protocol.SessionStatusSyncing),
		string(protocol.SessionStatusReady), string(protocol.SessionStatusDegraded),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrConnectionSuperseded
	}
	return nil
}

var _ SessionStore = (*SQLiteSessionStore)(nil)
var _ SessionTx = (*SQLiteSessionStoreTx)(nil)
