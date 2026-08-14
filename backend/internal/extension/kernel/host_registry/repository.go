package host_registry

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

var hashedTokenRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

var (
	ErrInvalidHostEntry = errors.New("host_registry: invalid host entry")
	ErrHostNotFound     = errors.New("host_registry: host not found")
)

type hostRepository struct {
	db *sql.DB
}

func (r *hostRepository) SaveHost(ctx context.Context, entry *HostEntry) error {
	if r.db == nil {
		return nil
	}
	caps, err := json.Marshal(entry.Capabilities)
	if err != nil {
		return fmt.Errorf("host_registry: marshal capabilities: %w", err)
	}
	expiresAt := ""
	if !entry.ExpiresAt.IsZero() {
		expiresAt = entry.ExpiresAt.Format(time.RFC3339Nano)
	}
	tokenHash := ""
	if entry.SessionToken != "" {
		tokenHash = entry.SessionTokenHash()
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO kernel_host_registry
		(host_client_id, host_session_id, user_id, platform, device_id, runtime_id, window_id, capabilities, authenticated_at, last_heartbeat, connection_state, session_token, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.HostClientID,
		entry.HostSessionID,
		entry.UserID.String(),
		entry.Platform.String(),
		entry.DeviceID.String(),
		entry.RuntimeID.String(),
		entry.WindowID,
		string(caps),
		entry.AuthenticatedAt.Format(time.RFC3339Nano),
		entry.LastHeartbeat.Format(time.RFC3339Nano),
		string(entry.ConnectionState),
		tokenHash,
		entry.CreatedAt.Format(time.RFC3339Nano),
		expiresAt,
	)
	return err
}

func (r *hostRepository) GetHost(ctx context.Context, hostClientID string) (*HostEntry, error) {
	if r.db == nil {
		return nil, ErrHostNotFound
	}
	row := r.db.QueryRowContext(ctx,
		`SELECT host_client_id, host_session_id, user_id, platform, device_id, window_id, capabilities, authenticated_at, last_heartbeat, connection_state, session_token, created_at, expires_at
		FROM kernel_host_registry WHERE host_client_id = ?`,
		hostClientID,
	)
	return scanHostEntry(row)
}

func (r *hostRepository) ListHostsByUser(ctx context.Context, userID string) ([]*HostEntry, error) {
	if r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT host_client_id, host_session_id, user_id, platform, device_id, window_id, capabilities, authenticated_at, last_heartbeat, connection_state, session_token, created_at, expires_at
		FROM kernel_host_registry WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHostEntries(rows)
}

func (r *hostRepository) ListAllHosts(ctx context.Context) ([]*HostEntry, error) {
	if r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT host_client_id, host_session_id, user_id, platform, device_id, window_id, capabilities, authenticated_at, last_heartbeat, connection_state, session_token, created_at, expires_at
		FROM kernel_host_registry`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHostEntries(rows)
}

func (r *hostRepository) DeleteHost(ctx context.Context, hostClientID string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM kernel_host_registry WHERE host_client_id = ?`,
		hostClientID,
	)
	return err
}

func (r *hostRepository) UpdateConnectionState(ctx context.Context, hostClientID string, state ConnectionState) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE kernel_host_registry SET connection_state = ? WHERE host_client_id = ?`,
		string(state),
		hostClientID,
	)
	return err
}

func (r *hostRepository) UpdateHeartbeat(ctx context.Context, hostClientID string, heartbeat time.Time) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE kernel_host_registry SET last_heartbeat = ? WHERE host_client_id = ?`,
		heartbeat.Format(time.RFC3339Nano),
		hostClientID,
	)
	return err
}

func (r *hostRepository) ListExpired(ctx context.Context) ([]*HostEntry, error) {
	if r.db == nil {
		return nil, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows, err := r.db.QueryContext(ctx,
		`SELECT host_client_id, host_session_id, user_id, platform, device_id, window_id, capabilities, authenticated_at, last_heartbeat, connection_state, session_token, created_at, expires_at
		FROM kernel_host_registry WHERE expires_at != '' AND expires_at < ?`,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHostEntries(rows)
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanHostEntry(row rowScanner) (*HostEntry, error) {
	var entry HostEntry
	var capsStr string
	var authenticatedAt, lastHeartbeat, createdAt string
	var expiresAt string
	var connState string

	err := row.Scan(
		&entry.HostClientID,
		&entry.HostSessionID,
		&entry.UserID,
		&entry.Platform,
		&entry.DeviceID,
		&entry.WindowID,
		&capsStr,
		&authenticatedAt,
		&lastHeartbeat,
		&connState,
		&entry.SessionToken,
		&createdAt,
		&expiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrHostNotFound
		}
		return nil, err
	}

	if capsStr != "" {
		if err := json.Unmarshal([]byte(capsStr), &entry.Capabilities); err != nil {
			return nil, fmt.Errorf("host_registry: unmarshal capabilities: %w", err)
		}
	}

	entry.ConnectionState = ConnectionState(connState)

	if authenticatedAt != "" {
		t, err := time.Parse(time.RFC3339Nano, authenticatedAt)
		if err == nil {
			entry.AuthenticatedAt = t
		}
	}
	if lastHeartbeat != "" {
		t, err := time.Parse(time.RFC3339Nano, lastHeartbeat)
		if err == nil {
			entry.LastHeartbeat = t
		}
	}
	if createdAt != "" {
		t, err := time.Parse(time.RFC3339Nano, createdAt)
		if err == nil {
			entry.CreatedAt = t
		}
	}
	if expiresAt != "" {
		t, err := time.Parse(time.RFC3339Nano, expiresAt)
		if err == nil {
			entry.ExpiresAt = t
		}
	}

	return &entry, nil
}

func scanHostEntries(rows *sql.Rows) ([]*HostEntry, error) {
	var result []*HostEntry
	for rows.Next() {
		entry, err := scanHostEntry(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (r *hostRepository) MigrateSessionTokens(ctx context.Context) error {
	if r.db == nil {
		return nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT host_client_id, session_token FROM kernel_host_registry WHERE session_token != ''`)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("host_registry: query session tokens: %w", err)
	}
	defer rows.Close()

	type migrationRow struct {
		hostClientID string
		token        string
	}
	var pending []migrationRow
	for rows.Next() {
		var hostClientID, token string
		if err := rows.Scan(&hostClientID, &token); err != nil {
			return fmt.Errorf("host_registry: scan session token: %w", err)
		}
		if token == "" || hashedTokenRe.MatchString(token) {
			continue
		}
		pending = append(pending, migrationRow{hostClientID: hostClientID, token: token})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("host_registry: iterate session tokens: %w", err)
	}

	for _, row := range pending {
		tokenHash := sha256.Sum256([]byte(row.token))
		hexHash := hex.EncodeToString(tokenHash[:])
		if _, err := r.db.ExecContext(ctx, `UPDATE kernel_host_registry SET session_token = ? WHERE host_client_id = ?`, hexHash, row.hostClientID); err != nil {
			return fmt.Errorf("host_registry: hash session token for %s: %w", row.hostClientID, err)
		}
	}
	return nil
}
