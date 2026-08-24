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
