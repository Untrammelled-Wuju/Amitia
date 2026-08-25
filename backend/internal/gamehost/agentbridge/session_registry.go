package agentbridge

import (
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

// SessionScope binds an opaque plugin session (when supplied by the plugin) or
// the runtime default route to the host interaction context. GameHost never
// interprets the session identifier or any game-specific state.
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
	mu       sync.RWMutex
	sessions map[string]SessionScope
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{sessions: make(map[string]SessionScope)}
}

func sessionKey(runtimeID domain.RuntimeInstanceID, pluginSessionID string) string {
	return string(runtimeID) + "\x00" + pluginSessionID
}

func (r *SessionRegistry) Bind(scope SessionScope) {
	if r == nil || scope.RuntimeID == "" {
		return
	}
	if scope.UpdatedAt.IsZero() {
		scope.UpdatedAt = time.Now().UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[sessionKey(scope.RuntimeID, scope.PluginSessionID)] = scope

	// A host/Agent context is usually established on the runtime-default route
	// by a tool invocation or an explicit host binding. If plugin-session routes
	// were observed earlier during cold-start events, enrich those route-only
	// entries as well so later events from the same opaque plugin session target
	// the selected Agent instead of falling back to a global default forever.
	if scope.PluginSessionID == "" && hasHostAgentContext(scope) {
		prefix := string(scope.RuntimeID) + "\x00"
		for key, existing := range r.sessions {
			if key == sessionKey(scope.RuntimeID, "") || len(key) < len(prefix) || key[:len(prefix)] != prefix {
				continue
			}
			if existing.PluginID != "" && scope.PluginID != "" && existing.PluginID != scope.PluginID {
				continue
			}
			if existing.ServiceID != "" && scope.ServiceID != "" && existing.ServiceID != scope.ServiceID {
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
			r.sessions[key] = existing
		}
	}
}

func hasHostAgentContext(scope SessionScope) bool {
	return scope.UserID != "" || scope.CharacterID != "" || scope.ConversationID != "" || scope.Channel != "" || scope.HostSessionID != ""
}

// Resolve first checks an exact opaque plugin session and then falls back to
// the runtime default context established by the latest tool invocation.
func (r *SessionRegistry) Resolve(runtimeID domain.RuntimeInstanceID, pluginSessionID string) (SessionScope, bool) {
	if r == nil {
		return SessionScope{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if pluginSessionID != "" {
		if scope, ok := r.sessions[sessionKey(runtimeID, pluginSessionID)]; ok {
			return scope, true
		}
	}
	scope, ok := r.sessions[sessionKey(runtimeID, "")]
	return scope, ok
}

func (r *SessionRegistry) Remove(runtimeID domain.RuntimeInstanceID, pluginSessionID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionKey(runtimeID, pluginSessionID))
}

func (r *SessionRegistry) RemoveRuntime(runtimeID domain.RuntimeInstanceID) {
	if r == nil {
		return
	}
	prefix := string(runtimeID) + "\x00"
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range r.sessions {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
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
	for key, scope := range r.sessions {
		if _, ok := active[scope.RuntimeID]; !ok {
			delete(r.sessions, key)
		}
	}
}
