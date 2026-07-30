package dev_mode

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type DeveloperSession struct {
	SessionID        string
	WorkspaceID      WorkspaceID
	ExtensionID      ExtensionID
	UserID           string
	DeviceID         string
	UserAgent        string
	Environment      string
	PolicyVersion    string
	DevTrustSnapshot bool
	DevTrustVersion  uint64
	StartedAt        time.Time
	ExpiresAt        time.Time
	Revoked          bool
	Scopes           []string
}

type SessionManager struct {
	mu          sync.RWMutex
	sessions    map[string]*DeveloperSession
	byWorkspace map[WorkspaceID]string
	ttl         time.Duration
}

func NewSessionManager(ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &SessionManager{
		sessions:    make(map[string]*DeveloperSession),
		byWorkspace: make(map[WorkspaceID]string),
		ttl:         ttl,
	}
}

var (
	ErrSessionNotFound = errors.New("dev_mode: session not found")
	ErrSessionExpired  = errors.New("dev_mode: session expired")
	ErrSessionRevoked  = errors.New("dev_mode: session revoked")
)

func (m *SessionManager) Open(ctx context.Context, workspace WorkspaceID, extension ExtensionID, userID, deviceID, userAgent, policyVersion string, devTrust bool, devTrustVersion uint64) (*DeveloperSession, error) {
	if workspace == "" || extension == "" || userID == "" || policyVersion == "" || !devTrust || devTrustVersion == 0 {
		return nil, fmt.Errorf("dev_mode: invalid developer session binding")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	sess := &DeveloperSession{
		SessionID:        newSessionID(workspace, now),
		WorkspaceID:      workspace,
		ExtensionID:      extension,
		UserID:           userID,
		DeviceID:         deviceID,
		UserAgent:        userAgent,
		Environment:      "development",
		PolicyVersion:    policyVersion,
		DevTrustSnapshot: devTrust,
		DevTrustVersion:  devTrustVersion,
		StartedAt:        now,
		ExpiresAt:        now.Add(m.ttl),
		Scopes:           []string{"extensions.install.unsigned"},
	}
	m.sessions[sess.SessionID] = sess
	m.byWorkspace[workspace] = sess.SessionID
	return sess, nil
}

func (m *SessionManager) Validate(sessionID string) (*DeveloperSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}
	if sess.Revoked {
		return nil, fmt.Errorf("%w: %s", ErrSessionRevoked, sessionID)
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		return nil, fmt.Errorf("%w: %s", ErrSessionExpired, sessionID)
	}
	return sess, nil
}

func (m *SessionManager) Revoke(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}
	sess.Revoked = true
	delete(m.byWorkspace, sess.WorkspaceID)
	return nil
}

func (m *SessionManager) RevokeAll() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	revoked := 0
	for _, sess := range m.sessions {
		if !sess.Revoked {
			sess.Revoked = true
			revoked++
		}
	}
	m.byWorkspace = make(map[WorkspaceID]string)
	return revoked
}

func (m *SessionManager) GetByWorkspace(workspace WorkspaceID) (*DeveloperSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sessID, ok := m.byWorkspace[workspace]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, workspace)
	}
	return m.sessions[sessID], nil
}

func (m *SessionManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	removed := 0
	for id, sess := range m.sessions {
		if sess.Revoked || now.After(sess.ExpiresAt) {
			delete(m.sessions, id)
			delete(m.byWorkspace, sess.WorkspaceID)
			removed++
		}
	}
	return removed
}

func newSessionID(workspace WorkspaceID, t time.Time) string {
	return fmt.Sprintf("dev-%s-%d", workspace, t.UnixNano())
}
