package agentbridge

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

const (
	defaultSessionRegistryMaxEntries           = 4096
	defaultSessionRegistryMaxEntriesPerService = 512
	defaultSessionRegistryTTL                  = 24 * time.Hour
)

// SessionScope binds an opaque plugin session (when supplied by the plugin) or
// the service-default route to the host interaction context. GameHost never
// interprets the session identifier or any game-specific state.
//
// ServiceID is part of the identity. A single plugin runtime may host multiple
// process services and those services are allowed to reuse the same opaque
// plugin-session identifier without overwriting each other's Agent context.
type SessionScope struct {
	PluginSessionID string
	PluginID        domain.PluginID
	RuntimeID       domain.RuntimeInstanceID
	ServiceID       domain.ServiceID
	Generation      int64
	UserID          string
	CharacterID     string
	ConversationID  string
	Channel         string
	HostSessionID   string
	UpdatedAt       time.Time
}

type SessionRegistry struct {
	mu                   sync.RWMutex
	sessions             map[string]SessionScope
	maxEntries           int
	maxEntriesPerService int
	ttl                  time.Duration
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions:             make(map[string]SessionScope),
		maxEntries:           defaultSessionRegistryMaxEntries,
		maxEntriesPerService: defaultSessionRegistryMaxEntriesPerService,
		ttl:                  defaultSessionRegistryTTL,
	}
}

func sessionKey(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginSessionID string) string {
	return string(runtimeID) + "\x00" + string(serviceID) + "\x00" + pluginSessionID
}

func runtimePrefix(runtimeID domain.RuntimeInstanceID) string {
	return string(runtimeID) + "\x00"
}

func servicePrefix(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) string {
	return string(runtimeID) + "\x00" + string(serviceID) + "\x00"
}

func (r *SessionRegistry) Bind(scope SessionScope) {
	if r == nil || scope.RuntimeID == "" || scope.ServiceID == "" {
		return
	}
	if scope.UpdatedAt.IsZero() {
		scope.UpdatedAt = time.Now().UTC()
	}
	if scope.Generation < 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.pruneLocked(scope.UpdatedAt)
	key := sessionKey(scope.RuntimeID, scope.ServiceID, scope.PluginSessionID)
	r.sessions[key] = scope

	// A host/Agent context is usually established on the service-default route
	// by a tool invocation or an explicit host binding. If plugin-session routes
	// were observed earlier during cold-start events, enrich only entries for the
	// same runtime+service. Never let service B overwrite service A simply because
	// both happen to use the same opaque plugin session ID.
	if scope.PluginSessionID == "" && hasHostAgentContext(scope) {
		prefix := servicePrefix(scope.RuntimeID, scope.ServiceID)
		for existingKey, existing := range r.sessions {
			if existingKey == key || !strings.HasPrefix(existingKey, prefix) {
				continue
			}
			if existing.PluginID != "" && scope.PluginID != "" && existing.PluginID != scope.PluginID {
				continue
			}
			existing.PluginID = scope.PluginID
			existing.ServiceID = scope.ServiceID
			existing.Generation = scope.Generation
			existing.UserID = scope.UserID
			existing.CharacterID = scope.CharacterID
			existing.ConversationID = scope.ConversationID
			existing.Channel = scope.Channel
			existing.HostSessionID = scope.HostSessionID
			existing.UpdatedAt = scope.UpdatedAt
			r.sessions[existingKey] = existing
		}
	}

	r.enforceServiceCapacityLocked(scope.RuntimeID, scope.ServiceID)
	r.enforceCapacityLocked()
}

func hasHostAgentContext(scope SessionScope) bool {
	return scope.UserID != "" || scope.CharacterID != "" || scope.ConversationID != "" || scope.Channel != "" || scope.HostSessionID != ""
}

// Resolve first checks an exact opaque plugin session and then falls back to
// the runtime+service default context established by the latest tool invocation
// or explicit host binding.
func (r *SessionRegistry) Resolve(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginSessionID string) (SessionScope, bool) {
	if r == nil || runtimeID == "" || serviceID == "" {
		return SessionScope{}, false
	}
	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(now)

	if pluginSessionID != "" {
		if scope, ok := r.sessions[sessionKey(runtimeID, serviceID, pluginSessionID)]; ok {
			return scope, true
		}
	}
	scope, ok := r.sessions[sessionKey(runtimeID, serviceID, "")]
	return scope, ok
}

func (r *SessionRegistry) Remove(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginSessionID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionKey(runtimeID, serviceID, pluginSessionID))
}

func (r *SessionRegistry) RemoveService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) {
	if r == nil {
		return
	}
	prefix := servicePrefix(runtimeID, serviceID)
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range r.sessions {
		if strings.HasPrefix(key, prefix) {
			delete(r.sessions, key)
		}
	}
}

func (r *SessionRegistry) RemoveRuntime(runtimeID domain.RuntimeInstanceID) {
	if r == nil {
		return
	}
	prefix := runtimePrefix(runtimeID)
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range r.sessions {
		if strings.HasPrefix(key, prefix) {
			delete(r.sessions, key)
		}
	}
}

// RetainRuntimes removes session/context bindings for runtimes that no longer
// exist in the authoritative RuntimeManager. Generation restarts keep the same
// runtime ID and therefore retain context; uninstall/reprovision of a removed
// runtime cannot accidentally inherit stale Agent state.
func (r *SessionRegistry) RetainRuntimes(runtimeIDs []domain.RuntimeInstanceID) {
	if r == nil {
		return
	}
	active := make(map[domain.RuntimeInstanceID]struct{}, len(runtimeIDs))
	for _, runtimeID := range runtimeIDs {
		if runtimeID != "" {
			active[runtimeID] = struct{}{}
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(time.Now().UTC())
	for key, scope := range r.sessions {
		if _, ok := active[scope.RuntimeID]; !ok {
			delete(r.sessions, key)
		}
	}
}

func (r *SessionRegistry) pruneLocked(now time.Time) {
	if r == nil || r.ttl <= 0 {
		return
	}
	cutoff := now.Add(-r.ttl)
	for key, scope := range r.sessions {
		if !scope.UpdatedAt.IsZero() && scope.UpdatedAt.Before(cutoff) {
			delete(r.sessions, key)
		}
	}
}

func (r *SessionRegistry) enforceServiceCapacityLocked(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) {
	if r == nil || r.maxEntriesPerService <= 0 {
		return
	}
	prefix := servicePrefix(runtimeID, serviceID)
	type entry struct {
		key          string
		at           time.Time
		defaultRoute bool
	}
	entries := make([]entry, 0)
	for key, scope := range r.sessions {
		if strings.HasPrefix(key, prefix) {
			entries = append(entries, entry{key: key, at: scope.UpdatedAt, defaultRoute: scope.PluginSessionID == ""})
		}
	}
	if len(entries) <= r.maxEntriesPerService {
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].defaultRoute != entries[j].defaultRoute {
			return !entries[i].defaultRoute
		}
		if entries[i].at.Equal(entries[j].at) {
			return entries[i].key < entries[j].key
		}
		return entries[i].at.Before(entries[j].at)
	})
	remove := len(entries) - r.maxEntriesPerService
	for i := 0; i < remove; i++ {
		delete(r.sessions, entries[i].key)
	}
}

func (r *SessionRegistry) enforceCapacityLocked() {
	if r == nil || r.maxEntries <= 0 || len(r.sessions) <= r.maxEntries {
		return
	}
	type entry struct {
		key          string
		at           time.Time
		defaultRoute bool
	}
	entries := make([]entry, 0, len(r.sessions))
	for key, scope := range r.sessions {
		entries = append(entries, entry{key: key, at: scope.UpdatedAt, defaultRoute: scope.PluginSessionID == ""})
	}
	sort.Slice(entries, func(i, j int) bool {
		// Plugin-created opaque session IDs are untrusted and potentially
		// unbounded. Evict those before the host-owned service-default context so
		// a flood of transient game sessions cannot knock out the Agent routing
		// binding for an otherwise healthy runtime.
		if entries[i].defaultRoute != entries[j].defaultRoute {
			return !entries[i].defaultRoute
		}
		if entries[i].at.Equal(entries[j].at) {
			return entries[i].key < entries[j].key
		}
		return entries[i].at.Before(entries[j].at)
	})
	remove := len(entries) - r.maxEntries
	for i := 0; i < remove; i++ {
		delete(r.sessions, entries[i].key)
	}
}

func (r *SessionRegistry) Size() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(time.Now().UTC())
	return len(r.sessions)
}
