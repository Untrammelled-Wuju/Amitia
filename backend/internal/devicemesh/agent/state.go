package agent

import "sync"

type AgentState string

const (
	StateUnprovisioned AgentState = "unprovisioned"
	StateConnecting    AgentState = "connecting"
	StateHandshaking   AgentState = "handshaking"
	StateReady         AgentState = "ready"
	StateDegraded      AgentState = "degraded"
	StateBackoff       AgentState = "backoff"
	StateRevoked       AgentState = "revoked"
	StateStopped       AgentState = "stopped"
)

type StateManager struct {
	mu    sync.RWMutex
	state AgentState
}

func NewStateManager() *StateManager {
	return &StateManager{state: StateUnprovisioned}
}

func (s *StateManager) Set(state AgentState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

func (s *StateManager) Get() AgentState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *StateManager) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == StateReady || s.state == StateDegraded
}

type ConnectionManager struct {
	mu    sync.RWMutex
	state AgentState
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{state: StateUnprovisioned}
}

func (s *ConnectionManager) Set(state AgentState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

func (s *ConnectionManager) Get() AgentState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *ConnectionManager) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == StateReady || s.state == StateDegraded
}

func (s *ConnectionManager) CanReconnect() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state != StateRevoked && s.state != StateStopped && s.state != StateUnprovisioned
}
