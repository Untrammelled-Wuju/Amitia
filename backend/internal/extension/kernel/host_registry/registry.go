package host_registry

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type HostCapability string

const (
	CapUINotify       HostCapability = "ui.notify"
	CapUIDialog       HostCapability = "ui.dialog"
	CapUINavigate     HostCapability = "ui.navigate"
	CapClipboardWrite HostCapability = "clipboard.write"
	CapClipboardRead  HostCapability = "clipboard.read"
	CapDesktopMenu    HostCapability = "desktop.menu"
	CapDesktopTray    HostCapability = "desktop.tray"
)

type ConnectionState string

const (
	HostStateReady        ConnectionState = "ready"
	HostStateDisconnected ConnectionState = "disconnected"
	HostStateExpired      ConnectionState = "expired"
)

const heartbeatValidDuration = 5 * time.Minute

type HostEntry struct {
	HostClientID    string
	HostSessionID   string
	UserID          runtimeidentity.UserID
	Platform        runtimeidentity.Platform
	DeviceID        runtimeidentity.DeviceID
	RuntimeID       runtimeidentity.RuntimeID
	WindowID        string
	Capabilities    []HostCapability
	AuthenticatedAt time.Time
	LastHeartbeat   time.Time
	ConnectionState ConnectionState
	SessionToken    string
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

func (e *HostEntry) HasCapability(cap HostCapability) bool {
	for _, c := range e.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

func hashSessionToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (e *HostEntry) SessionTokenHash() string {
	return hashSessionToken(e.SessionToken)
}

func (e *HostEntry) IsExpired() bool {
	if e.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().UTC().After(e.ExpiresAt)
}

func (e *HostEntry) IsHeartbeatValid() bool {
	return time.Since(e.LastHeartbeat) <= heartbeatValidDuration
}

// HostRegistry 是 Extension Kernel 中 Device/Host presence 的 canonical registry baseline。
// 后续通用 Device Mesh 在此基础上提升，UI Host、DesktopPet、GameHost 不允许拥有另一套全局 Device Registry。
type HostRegistry struct {
	mu    sync.RWMutex
	db    *sql.DB
	repo  *hostRepository
	hosts map[string]*HostEntry
}

func NewHostRegistry(db *sql.DB) *HostRegistry {
	return &HostRegistry{
		db:    db,
		repo:  &hostRepository{db: db},
		hosts: make(map[string]*HostEntry),
	}
}

func MigrateSessionTokens(ctx context.Context, db *sql.DB) error {
	return (&hostRepository{db: db}).MigrateSessionTokens(ctx)
}

func (r *HostRegistry) RegisterHost(ctx context.Context, entry *HostEntry) error {
	if entry == nil {
		return ErrInvalidHostEntry
	}
	if entry.HostClientID == "" {
		return ErrInvalidHostEntry
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	if entry.AuthenticatedAt.IsZero() {
		entry.AuthenticatedAt = time.Now().UTC()
	}
	if entry.LastHeartbeat.IsZero() {
		entry.LastHeartbeat = time.Now().UTC()
	}
	if entry.ConnectionState == "" {
		entry.ConnectionState = HostStateReady
	}
	if err := r.repo.SaveHost(ctx, entry); err != nil {
		return err
	}
	r.mu.Lock()
	r.hosts[entry.HostClientID] = entry
	r.mu.Unlock()
	return nil
}

func (r *HostRegistry) UnregisterHost(ctx context.Context, hostClientID string) error {
	if err := r.repo.DeleteHost(ctx, hostClientID); err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.hosts, hostClientID)
	r.mu.Unlock()
	return nil
}

func (r *HostRegistry) GetHost(ctx context.Context, hostClientID string) (*HostEntry, error) {
	r.mu.RLock()
	entry, ok := r.hosts[hostClientID]
	r.mu.RUnlock()
	if ok {
		return entry, nil
	}
	entry, err := r.repo.GetHost(ctx, hostClientID)
	if err != nil {
		return nil, err
	}
	if entry != nil {
		r.mu.Lock()
		r.hosts[hostClientID] = entry
		r.mu.Unlock()
	}
	return entry, nil
}

func (r *HostRegistry) ListHostsByUser(ctx context.Context, userID runtimeidentity.UserID) ([]*HostEntry, error) {
	r.mu.RLock()
	var result []*HostEntry
	for _, h := range r.hosts {
		if h.UserID == userID {
			result = append(result, h)
		}
	}
	r.mu.RUnlock()
	if len(result) > 0 {
		return result, nil
	}
	return r.repo.ListHostsByUser(ctx, userID)
}

func (r *HostRegistry) ListHostsByUserString(ctx context.Context, userID string) ([]*HostEntry, error) {
	return r.ListHostsByUser(ctx, runtimeidentity.ParseUserID(userID))
}

func (r *HostRegistry) ListReadyHosts(ctx context.Context, userID runtimeidentity.UserID, capability HostCapability) ([]*HostEntry, error) {
	hosts, err := r.ListHostsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	var result []*HostEntry
	for _, h := range hosts {
		if h.ConnectionState == HostStateReady &&
			h.IsHeartbeatValid() &&
			!h.IsExpired() &&
			h.HasCapability(capability) {
			result = append(result, h)
		}
	}
	return result, nil
}

func (r *HostRegistry) ListReadyHostsString(ctx context.Context, userID string, capability HostCapability) ([]*HostEntry, error) {
	return r.ListReadyHosts(ctx, runtimeidentity.ParseUserID(userID), capability)
}

func (r *HostRegistry) UpdateHeartbeat(ctx context.Context, hostClientID string) error {
	now := time.Now().UTC()
	if err := r.repo.UpdateHeartbeat(ctx, hostClientID, now); err != nil {
		return err
	}
	r.mu.Lock()
	if h, ok := r.hosts[hostClientID]; ok {
		h.LastHeartbeat = now
	}
	r.mu.Unlock()
	return nil
}

func (r *HostRegistry) SetDisconnected(ctx context.Context, hostClientID string) error {
	if err := r.repo.UpdateConnectionState(ctx, hostClientID, HostStateDisconnected); err != nil {
		return err
	}
	r.mu.Lock()
	if h, ok := r.hosts[hostClientID]; ok {
		h.ConnectionState = HostStateDisconnected
	}
	r.mu.Unlock()
	return nil
}

func (r *HostRegistry) FindTargetHost(ctx context.Context, userID runtimeidentity.UserID, capability HostCapability, platform runtimeidentity.Platform, windowID string) (*HostEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var bestMatch *HostEntry
	for _, h := range r.hosts {
		if h.ConnectionState != HostStateReady {
			continue
		}
		if !h.IsHeartbeatValid() {
			continue
		}
		if h.IsExpired() {
			continue
		}
		if !h.HasCapability(capability) {
			continue
		}
		if userID != "" && h.UserID != userID {
			continue
		}
		if platform != "" && h.Platform != platform {
			continue
		}
		if windowID != "" && h.WindowID != windowID {
			continue
		}
		bestMatch = h
		break
	}
	return bestMatch, nil
}

func (r *HostRegistry) FindTargetHostString(ctx context.Context, userID string, capability HostCapability, platform string, windowID string) (*HostEntry, error) {
	return r.FindTargetHost(ctx, runtimeidentity.ParseUserID(userID), capability, runtimeidentity.ParsePlatform(platform), windowID)
}

func (r *HostRegistry) LoadFromStore(ctx context.Context) error {
	hosts, err := r.repo.ListAllHosts(ctx)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.hosts = make(map[string]*HostEntry, len(hosts))
	for _, h := range hosts {
		r.hosts[h.HostClientID] = h
	}
	r.mu.Unlock()
	return nil
}

func (r *HostRegistry) CleanupExpired(ctx context.Context) error {
	expired, err := r.repo.ListExpired(ctx)
	if err != nil {
		return err
	}
	for _, h := range expired {
		if err := r.repo.UpdateConnectionState(ctx, h.HostClientID, HostStateExpired); err != nil {
			return err
		}
		r.mu.Lock()
		if entry, ok := r.hosts[h.HostClientID]; ok {
			entry.ConnectionState = HostStateExpired
		}
		r.mu.Unlock()
	}
	return nil
}

func (r *HostRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.hosts)
}

func (r *HostRegistry) HasReadyHost(ctx context.Context, userID runtimeidentity.UserID, capability HostCapability) bool {
	hosts, err := r.ListReadyHosts(ctx, userID, capability)
	if err != nil {
		return false
	}
	return len(hosts) > 0
}

func (r *HostRegistry) HasReadyHostString(ctx context.Context, userID string, capability HostCapability) bool {
	return r.HasReadyHost(ctx, runtimeidentity.ParseUserID(userID), capability)
}
