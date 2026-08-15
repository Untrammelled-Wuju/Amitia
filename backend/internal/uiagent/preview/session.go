package preview

import (
	"errors"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/uiagent/schema"
)

type SessionState string

const (
	PreviewSessionRunning    SessionState = "running"
	PreviewSessionIdle       SessionState = "idle"
	PreviewSessionPaused     SessionState = "paused"
	PreviewSessionError      SessionState = "error"
	PreviewSessionTerminated SessionState = "terminated"
)

type PreviewSession struct {
	ID           string                    `json:"id"`
	WorkspaceID  string                    `json:"workspaceId,omitempty"`
	Schema       *schema.SchemaUIDocument  `json:"schema,omitempty"`
	State        SessionState              `json:"state"`
	LastActivity time.Time                 `json:"lastActivity"`
	mu           sync.Mutex
}

func (s *PreviewSession) IsActive() bool {
	return s.State == PreviewSessionRunning || s.State == PreviewSessionIdle
}

type SessionManager interface {
	Create(workspaceID string, doc *schema.SchemaUIDocument) (*PreviewSession, error)
	Get(id string) (*PreviewSession, error)
	Terminate(id string) error
}

type defaultSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*PreviewSession
}

func NewSessionManager() SessionManager {
	return &defaultSessionManager{sessions: make(map[string]*PreviewSession)}
}

func (m *defaultSessionManager) Create(workspaceID string, doc *schema.SchemaUIDocument) (*PreviewSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session := &PreviewSession{
		ID:           generatePreviewSessionID(),
		WorkspaceID:  workspaceID,
		Schema:       doc,
		State:        PreviewSessionIdle,
		LastActivity: time.Now(),
	}
	m.sessions[session.ID] = session
	return session, nil
}

func (m *defaultSessionManager) Get(id string) (*PreviewSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[id]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

func (m *defaultSessionManager) Terminate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[id]
	if !ok {
		return ErrSessionNotFound
	}
	session.State = PreviewSessionTerminated
	delete(m.sessions, id)
	return nil
}

var ErrSessionNotFound = errors.New("preview: session not found")

func generatePreviewSessionID() string {
	return "preview_" + time.Now().Format("20060102_150405")
}
