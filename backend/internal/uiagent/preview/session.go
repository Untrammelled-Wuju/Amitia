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
	PreviewSessionReady      SessionState = "ready"
	PreviewSessionError      SessionState = "error"
	PreviewSessionTerminated SessionState = "terminated"
)

type PreviewTarget struct {
	WorkspaceID string `json:"workspaceId,omitempty"`
	Platform    string `json:"platform,omitempty"`
	SourceType  string `json:"sourceType,omitempty"`
}

type PreviewSession struct {
	ID             string                    `json:"id"`
	WorkspaceID    string                    `json:"workspaceId,omitempty"`
	Schema         *schema.SchemaUIDocument  `json:"schema,omitempty"`
	State          SessionState              `json:"state"`
	Target         *PreviewTarget            `json:"target,omitempty"`
	Revision       string                    `json:"revision,omitempty"`
	TransactionID  string                    `json:"transactionId,omitempty"`
	RootExecution  string                    `json:"rootExecution,omitempty"`
	LastActivity   time.Time                 `json:"lastActivity"`
	SchemaWarnings []string                  `json:"schemaWarnings,omitempty"`
	SchemaErrors   []string                  `json:"schemaErrors,omitempty"`
	mu             sync.Mutex
}

func (s *PreviewSession) IsActive() bool {
	return s.State == PreviewSessionRunning || s.State == PreviewSessionIdle || s.State == PreviewSessionReady
}

type SessionManager interface {
	Create(workspaceID string, doc *schema.SchemaUIDocument) (*PreviewSession, error)
	Get(id string) (*PreviewSession, error)
	UpdateRevision(sessionID, revision, transactionID string) error
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

func (m *defaultSessionManager) UpdateRevision(sessionID, revision, transactionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	session.Revision = revision
	if transactionID != "" {
		session.TransactionID = transactionID
	}
	session.LastActivity = time.Now()
	return nil
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
