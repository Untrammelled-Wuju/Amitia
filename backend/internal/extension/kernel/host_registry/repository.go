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

func hashSessionToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

type registryRepository struct {
	db *sql.DB
}

func (r *registryRepository) MigrateRuntimeSessionColumns(ctx context.Context) error {
	if r.db == nil {
		return nil
	}
	if exists, err := r.columnExists(ctx, "kernel_host_registry", "runtime_session_id"); err != nil {
		return err
	} else if !exists {
		if _, err := r.db.ExecContext(ctx, `ALTER TABLE kernel_host_registry ADD COLUMN runtime_session_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("host_registry: add runtime_session_id column: %w", err)
		}
	}
	if exists, err := r.columnExists(ctx, "kernel_host_registry", "connection_generation"); err != nil {
		return err
	} else if !exists {
		if _, err := r.db.ExecContext(ctx, `ALTER TABLE kernel_host_registry ADD COLUMN connection_generation INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("host_registry: add connection_generation column: %w", err)
		}
	}
	if exists, err := r.columnExists(ctx, "kernel_host_registry", "entry_id"); err != nil {
		return err
	} else if !exists {
		if _, err := r.db.ExecContext(ctx, `ALTER TABLE kernel_host_registry ADD COLUMN entry_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("host_registry: add entry_id column: %w", err)
		}
	}
	return nil
}

func (r *registryRepository) columnExists(ctx context.Context, table, column string) (bool, error) {
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    string
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (r *registryRepository) SaveEntry(ctx context.Context, entry *RuntimeEntry) error {
	if r.db == nil {
		return nil
	}
	features, err := json.Marshal(entry.Features)
	if err != nil {
		return fmt.Errorf("host_registry: marshal features: %w", err)
	}
	expiresAt := ""
	if !entry.ExpiresAt.IsZero() {
		expiresAt = entry.ExpiresAt.Format(time.RFC3339Nano)
	}
	tokenHash := ""
	if entry.SessionToken != "" {
		tokenHash = hashSessionToken(entry.SessionToken)
	}
	entryID := entry.EntryID
	if entryID == "" {
		entryID = RuntimeEntryID(entry.UserID, entry.DeviceID, entry.RuntimeID)
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO kernel_host_registry
		(host_client_id, host_session_id, user_id, platform, device_id, runtime_id, window_id, capabilities, entry_kind, authenticated_at, last_heartbeat, connection_state, session_token, created_at, expires_at, runtime_session_id, connection_generation, entry_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.HostClientID,
		entry.HostSessionID,
		entry.UserID.String(),
		entry.Platform.String(),
		entry.DeviceID.String(),
		entry.RuntimeID.String(),
		entry.WindowID,
		string(features),
		entry.Kind.String(),
		entry.AuthenticatedAt.Format(time.RFC3339Nano),
		entry.LastHeartbeat.Format(time.RFC3339Nano),
		string(entry.PresenceState),
		tokenHash,
		entry.CreatedAt.Format(time.RFC3339Nano),
		expiresAt,
		entry.RuntimeSessionID.String(),
		entry.ConnectionGeneration,
		entryID,
	)
	return err
}

func (r *registryRepository) GetEntry(ctx context.Context, entryID string) (*RuntimeEntry, error) {
	if r.db == nil {
		return nil, ErrRegistryEntryNotFound
	}
	row := r.db.QueryRowContext(ctx,
		`SELECT host_client_id, host_session_id, user_id, platform, device_id, runtime_id, window_id, capabilities, entry_kind, authenticated_at, last_heartbeat, connection_state, session_token, created_at, expires_at, runtime_session_id, connection_generation
		FROM kernel_host_registry WHERE entry_id = ? OR host_client_id = ?`,
		entryID,
		entryID,
	)
	return scanEntry(row)
}

func (r *registryRepository) ListEntriesByUser(ctx context.Context, userID runtimeidentity.UserID) ([]*RuntimeEntry, error) {
	if r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT host_client_id, host_session_id, user_id, platform, device_id, runtime_id, window_id, capabilities, entry_kind, authenticated_at, last_heartbeat, connection_state, session_token, created_at, expires_at, runtime_session_id, connection_generation
		FROM kernel_host_registry WHERE user_id = ?`,
		userID.String(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

func (r *registryRepository) ListEntriesByDevice(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID) ([]*RuntimeEntry, error) {
	if r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT host_client_id, host_session_id, user_id, platform, device_id, runtime_id, window_id, capabilities, entry_kind, authenticated_at, last_heartbeat, connection_state, session_token, created_at, expires_at, runtime_session_id, connection_generation
		FROM kernel_host_registry WHERE user_id = ? AND device_id = ?`,
		userID.String(),
		deviceID.String(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

func (r *registryRepository) ListEntriesByRuntime(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) ([]*RuntimeEntry, error) {
	if r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT host_client_id, host_session_id, user_id, platform, device_id, runtime_id, window_id, capabilities, entry_kind, authenticated_at, last_heartbeat, connection_state, session_token, created_at, expires_at, runtime_session_id, connection_generation
		FROM kernel_host_registry WHERE user_id = ? AND device_id = ? AND runtime_id = ?`,
		userID.String(),
		deviceID.String(),
		runtimeID.String(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

func (r *registryRepository) ListAllEntries(ctx context.Context) ([]*RuntimeEntry, error) {
	if r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT host_client_id, host_session_id, user_id, platform, device_id, runtime_id, window_id, capabilities, entry_kind, authenticated_at, last_heartbeat, connection_state, session_token, created_at, expires_at, runtime_session_id, connection_generation
		FROM kernel_host_registry`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

func (r *registryRepository) DeleteEntry(ctx context.Context, entryID string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM kernel_host_registry WHERE entry_id = ? OR host_client_id = ?`,
		entryID,
		entryID,
	)
	return err
}

func (r *registryRepository) UpdateEntryState(ctx context.Context, entryID string, state PresenceState) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE kernel_host_registry SET connection_state = ? WHERE entry_id = ? OR host_client_id = ?`,
		string(state),
		entryID,
		entryID,
	)
	return err
}

func (r *registryRepository) UpdateEntryHeartbeat(ctx context.Context, entryID string, heartbeat time.Time) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE kernel_host_registry SET last_heartbeat = ? WHERE entry_id = ? OR host_client_id = ?`,
		heartbeat.Format(time.RFC3339Nano),
		entryID,
		entryID,
	)
	return err
}

func (r *registryRepository) UpdateRuntimeSessionBinding(ctx context.Context, entryID string, sessionID runtimeidentity.RuntimeSessionID, generation int64, state PresenceState, at time.Time) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE kernel_host_registry SET runtime_session_id = ?, connection_generation = ?, connection_state = ?, last_heartbeat = ? WHERE entry_id = ? AND (connection_generation < ? OR (connection_generation = ? AND runtime_session_id = ?))`,
		sessionID.String(),
		generation,
		string(state),
		at.Format(time.RFC3339Nano),
		entryID,
		generation,
		generation,
		sessionID.String(),
	)
	return err
}

func (r *registryRepository) UpdateRuntimeSessionHeartbeat(ctx context.Context, entryID string, sessionID runtimeidentity.RuntimeSessionID, generation int64, at time.Time) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE kernel_host_registry SET last_heartbeat = ?, connection_state = ? WHERE entry_id = ? AND runtime_session_id = ? AND connection_generation = ?`,
		at.Format(time.RFC3339Nano),
		string(PresenceStateReady),
		entryID,
		sessionID.String(),
		generation,
	)
	return err
}

func (r *registryRepository) SetRuntimeSessionDisconnected(ctx context.Context, entryID string, sessionID runtimeidentity.RuntimeSessionID, generation int64, at time.Time) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE kernel_host_registry SET connection_state = ?, last_heartbeat = ? WHERE entry_id = ? AND runtime_session_id = ? AND connection_generation = ?`,
		string(PresenceStateDisconnected),
		at.Format(time.RFC3339Nano),
		entryID,
		sessionID.String(),
		generation,
	)
	return err
}

func (r *registryRepository) MarkRuntimeEntriesDisconnected(ctx context.Context, at time.Time) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE kernel_host_registry SET connection_state = ? WHERE entry_kind = ? AND connection_state = ?`,
		string(PresenceStateDisconnected),
		string(RegistryEntryKindRuntime),
		string(PresenceStateReady),
	)
	return err
}

func (r *registryRepository) ListExpiredEntries(ctx context.Context) ([]*RuntimeEntry, error) {
	if r.db == nil {
		return nil, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows, err := r.db.QueryContext(ctx,
		`SELECT host_client_id, host_session_id, user_id, platform, device_id, runtime_id, window_id, capabilities, entry_kind, authenticated_at, last_heartbeat, connection_state, session_token, created_at, expires_at, runtime_session_id, connection_generation
		FROM kernel_host_registry WHERE expires_at != '' AND expires_at < ?`,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanEntry(row rowScanner) (*RuntimeEntry, error) {
	var entry RuntimeEntry
	var featuresStr string
	var authenticatedAt, lastHeartbeat, createdAt string
	var expiresAt string
	var presenceState string
	var userID, platform, deviceID, runtimeID string
	var kind string
	var runtimeSessionID string
	var connectionGeneration int64

	err := row.Scan(
		&entry.HostClientID,
		&entry.HostSessionID,
		&userID,
		&platform,
		&deviceID,
		&runtimeID,
		&entry.WindowID,
		&featuresStr,
		&kind,
		&authenticatedAt,
		&lastHeartbeat,
		&presenceState,
		&entry.SessionToken,
		&createdAt,
		&expiresAt,
		&runtimeSessionID,
		&connectionGeneration,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRegistryEntryNotFound
		}
		return nil, err
	}

	entry.UserID = runtimeidentity.ParseUserID(userID)
	entry.Platform = runtimeidentity.ParsePlatform(platform)
	entry.DeviceID = runtimeidentity.ParseDeviceID(deviceID)
	entry.RuntimeID = runtimeidentity.ParseRuntimeID(runtimeID)
	entry.Kind = ParseRegistryEntryKind(kind)
	entry.RuntimeSessionID = runtimeidentity.ParseRuntimeSessionID(runtimeSessionID)
	entry.ConnectionGeneration = connectionGeneration

	if featuresStr != "" {
		var rawFeatures []string
		if err := json.Unmarshal([]byte(featuresStr), &rawFeatures); err != nil {
			return nil, fmt.Errorf("host_registry: unmarshal features: %w", err)
		}
		entry.Features = make([]EndpointFeature, 0, len(rawFeatures))
		for _, f := range rawFeatures {
			entry.Features = append(entry.Features, EndpointFeature(f))
		}
	}

	entry.PresenceState = PresenceState(presenceState)

	if authenticatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, authenticatedAt); err == nil {
			entry.AuthenticatedAt = t
		}
	}
	if lastHeartbeat != "" {
		if t, err := time.Parse(time.RFC3339Nano, lastHeartbeat); err == nil {
			entry.LastHeartbeat = t
		}
	}
	if createdAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			entry.CreatedAt = t
		}
	}
	if expiresAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, expiresAt); err == nil {
			entry.ExpiresAt = t
		}
	}

	return &entry, nil
}

func scanEntries(rows *sql.Rows) ([]*RuntimeEntry, error) {
	var result []*RuntimeEntry
	for rows.Next() {
		entry, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (r *registryRepository) MigrateSessionTokens(ctx context.Context) error {
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
		entryID string
		token   string
	}
	var pending []migrationRow
	for rows.Next() {
		var entryID, token string
		if err := rows.Scan(&entryID, &token); err != nil {
			return fmt.Errorf("host_registry: scan session token: %w", err)
		}
		if token == "" || hashedTokenRe.MatchString(token) {
			continue
		}
		pending = append(pending, migrationRow{entryID: entryID, token: token})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("host_registry: iterate session tokens: %w", err)
	}

	for _, row := range pending {
		tokenHash := sha256.Sum256([]byte(row.token))
		hexHash := hex.EncodeToString(tokenHash[:])
		if _, err := r.db.ExecContext(ctx, `UPDATE kernel_host_registry SET session_token = ? WHERE host_client_id = ?`, hexHash, row.entryID); err != nil {
			return fmt.Errorf("host_registry: hash session token for %s: %w", row.entryID, err)
		}
	}
	return nil
}

func init() {
	_ = strings.TrimSpace
}
