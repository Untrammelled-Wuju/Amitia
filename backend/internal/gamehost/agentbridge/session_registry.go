package agentbridge

import (
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type SessionScope struct {
	GameSessionID  string
	PluginID       domain.PluginID
	RuntimeID      domain.RuntimeInstanceID
	ServiceID      domain.ServiceID
	UserID         string
	CharacterID    string
	ConversationID string
	Channel        string
	HostSessionID  string
	UpdatedAt      time.Time
}

type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]SessionScope
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{sessions: make(map[string]SessionScope)}
}

func sessionKey(runtimeID domain.RuntimeInstanceID, gameSessionID string) string {
	return string(runtimeID) + "\x00" + gameSessionID
}

func (r *SessionRegistry) Bind(scope SessionScope) {
	if r == nil || scope.RuntimeID == "" || scope.GameSessionID == "" {
		return
	}
	if scope.UpdatedAt.IsZero() {
		scope.UpdatedAt = time.Now().UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[sessionKey(scope.RuntimeID, scope.GameSessionID)] = scope
}

func (r *SessionRegistry) Resolve(runtimeID domain.RuntimeInstanceID, gameSessionID string) (SessionScope, bool) {
	if r == nil {
		return SessionScope{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	scope, ok := r.sessions[sessionKey(runtimeID, gameSessionID)]
	return scope, ok
}

func (r *SessionRegistry) Remove(runtimeID domain.RuntimeInstanceID, gameSessionID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionKey(runtimeID, gameSessionID))
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
