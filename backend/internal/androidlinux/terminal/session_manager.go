//go:build linux && !android

package terminal

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/u-ai/backend/internal/runtimehost"
)

func NewSessionManager(host runtimehost.RuntimeHost, policy Policy) *SessionManager {
	return &SessionManager{
		host:     host,
		sessions: make(map[SessionID]*Session),
		policy:   policy,
		clock:    defaultClock{},
		done:     make(chan struct{}),
	}
}

func (m *SessionManager) Open(ctx context.Context, params OpenParams) (SessionID, SessionState, uint16, uint16, error) {
	if err := validateShell(params.Shell, DefaultShellAllowlist); err != nil {
		return "", "", 0, 0, err
	}

	resolvedCwd, err := resolveWorkingDir(params.WorkingDir, params.Workspace)
	if err != nil {
		return "", "", 0, 0, err
	}

	if err := m.checkQuota(params.Owner); err != nil {
		return "", "", 0, 0, err
	}

	sessID := NewSessionID()
	sessionCtx, cancel := context.WithCancel(context.Background())

	sess := &Session{
		ID:                   sessID,
		Owner:                params.Owner,
		Shell:                params.Shell,
		WorkingDir:           resolvedCwd,
		State:                SessionStarting,
		Rows:                 params.Rows,
		Cols:                 params.Cols,
		CreatedAt:            time.Now(),
		LastActivity:         time.Now(),
		CreationInvocationID: params.InvocationID,
		cancel:               cancel,
		closeCh:              make(chan struct{}),
		output:               NewOutputBuffer(m.policy.MaxBufferedOutputBytes),
	}

	env := buildEnvironment(params.Workspace)

	ptmx, cmd, pid, err := startPTY(params.Shell, ptySize{Rows: params.Rows, Cols: params.Cols}, env, resolvedCwd)
	if err != nil {
		cancel()
		return "", "", 0, 0, ErrStartFailed(err.Error())
	}

	sess.ptmx = ptmx
	sess.cmd = cmd
	sess.PID = pid
	sess.StartedAt = time.Now()

	m.mu.Lock()
	m.sessions[sessID] = sess
	m.mu.Unlock()

	sess.SetState(SessionRunning)

	go sess.readLoop(sessionCtx)

	go func() {
		_ = cmd.Wait()
		sess.handleExit()
		m.cleanupSession(sessID)
	}()

	return sessID, SessionRunning, sess.Rows, sess.Cols, nil
}

func (m *SessionManager) Write(ctx context.Context, params WriteParams) (int, error) {
	sess, err := m.getSession(params.SessionID, params.Owner)
	if err != nil {
		return 0, err
	}

	return sess.writeStdin(params.Data)
}

func (m *SessionManager) Read(ctx context.Context, params ReadParams) ([]OutputChunk, uint64, bool, SessionState, error) {
	sess, err := m.getSessionAny(params.SessionID)
	if err != nil {
		return nil, 0, false, "", err
	}

	if !sess.BelongsTo(params.Owner) && params.Owner.UserID != "" {
		return nil, 0, false, "", ErrScopeDenied()
	}

	chunks, nextSeq, truncated := sess.output.Read(params.AfterSequence, params.MaxBytes)
	status := sess.status()

	return chunks, nextSeq, truncated, status.State, nil
}

func (m *SessionManager) Resize(ctx context.Context, owner SessionOwner, id SessionID, rows, cols uint16) error {
	if rows == 0 || cols == 0 {
		return ErrInputTooLarge(0)
	}
	if rows > 1000 || cols > 1000 {
		return ErrInvalidSize("rows/cols out of range")
	}

	sess, err := m.getSession(id, owner)
	if err != nil {
		return err
	}

	return sess.resize(rows, cols)
}

func (m *SessionManager) Status(ctx context.Context, owner SessionOwner, id SessionID) (SessionStatus, error) {
	sess, err := m.getSessionAny(id)
	if err != nil {
		return SessionStatus{}, err
	}

	if !sess.BelongsTo(owner) && owner.UserID != "" {
		return SessionStatus{}, ErrScopeDenied()
	}

	return sess.status(), nil
}

func (m *SessionManager) Close(ctx context.Context, owner SessionOwner, id SessionID) (SessionState, error) {
	sess, err := m.getSession(id, owner)
	if err != nil {
		return "", err
	}

	if !sess.IsActive() {
		return sess.GetState(), nil
	}

	close(sess.closeCh)

	if sess.ptmx != nil {
		_ = sess.ptmx.Close()
	}

	if m.policy.CloseGracePeriod > 0 {
		graceCtx, cancel := context.WithTimeout(ctx, m.policy.CloseGracePeriod)
		defer cancel()

		done := make(chan struct{})
		go func() {
			if sess.cmd != nil && sess.cmd.Process != nil {
				_ = sess.cmd.Process.Signal(syscall.SIGTERM)
			}
			close(done)
		}()

		select {
		case <-graceCtx.Done():
		case <-done:
		}
	}

	if sess.cmd != nil && sess.cmd.Process != nil {
		_ = sess.cmd.Process.Kill()
	}

	m.cleanupSession(id)

	sess.SetState(SessionClosing)
	return SessionClosing, nil
}

func (m *SessionManager) ForceCancel(ctx context.Context, owner SessionOwner, id SessionID) (SessionState, error) {
	m.mu.RLock()
	sess, exists := m.sessions[id]
	m.mu.RUnlock()

	if !exists {
		return "", ErrSessionNotFound(id)
	}

	if !sess.BelongsTo(owner) && owner.UserID != "" {
		return "", ErrScopeDenied()
	}

	if sess.cancel != nil {
		sess.cancel()
	}

	close(sess.closeCh)

	if sess.ptmx != nil {
		_ = sess.ptmx.Close()
		sess.ptmx = nil
	}

	if sess.cmd != nil && sess.cmd.Process != nil {
		_ = sess.cmd.Process.Kill()
	}

	m.cleanupSession(id)

	sess.SetState(SessionCancelled)
	return SessionCancelled, nil
}

func (m *SessionManager) CloseAll(ctx context.Context) error {
	m.mu.Lock()
	ids := make([]SessionID, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(sessionID SessionID) {
			defer wg.Done()
			m.forceCancelInternal(ctx, sessionID)
		}(id)
	}
	wg.Wait()

	return nil
}

func (m *SessionManager) forceCancelInternal(ctx context.Context, id SessionID) {
	m.mu.RLock()
	sess, exists := m.sessions[id]
	m.mu.RUnlock()

	if !exists {
		return
	}

	if sess.cancel != nil {
		sess.cancel()
	}

	select {
	case <-sess.closeCh:
	default:
		close(sess.closeCh)
	}

	if sess.ptmx != nil {
		_ = sess.ptmx.Close()
		sess.ptmx = nil
	}

	if sess.cmd != nil && sess.cmd.Process != nil {
		_ = sess.cmd.Process.Kill()
	}

	m.cleanupSession(id)
}

func (m *SessionManager) cleanupSession(id SessionID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

func (m *SessionManager) getSession(id SessionID, owner SessionOwner) (*Session, error) {
	m.mu.RLock()
	sess, exists := m.sessions[id]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrSessionNotFound(id)
	}

	if !sess.BelongsTo(owner) && owner.UserID != "" {
		return nil, ErrScopeDenied()
	}

	if !sess.IsActive() {
		return nil, ErrNotRunning()
	}

	return sess, nil
}

func (m *SessionManager) getSessionAny(id SessionID) (*Session, error) {
	m.mu.RLock()
	sess, exists := m.sessions[id]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrSessionNotFound(id)
	}

	return sess, nil
}

func (m *SessionManager) checkQuota(owner SessionOwner) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.sessions) >= m.policy.MaxSessions {
		return ErrSessionLimit(m.policy.MaxSessions)
	}

	userCount := 0
	convCount := 0
	for _, sess := range m.sessions {
		if sess.Owner.UserID == owner.UserID {
			userCount++
		}
		if sess.Owner.ConversationID == owner.ConversationID && owner.ConversationID != "" {
			convCount++
		}
	}

	if userCount >= m.policy.MaxSessionsPerUser {
		return ErrSessionLimit(m.policy.MaxSessionsPerUser)
	}

	if owner.ConversationID != "" && convCount >= m.policy.MaxSessionsPerConversation {
		return ErrSessionLimit(m.policy.MaxSessionsPerConversation)
	}

	return nil
}

func (m *SessionManager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *SessionManager) Snapshot() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := 0
	for _, sess := range m.sessions {
		if sess.IsActive() {
			active++
		}
	}

	return map[string]any{
		"activeSessionCount": active,
		"totalSessions":      len(m.sessions),
	}
}

func (m *SessionManager) checkIdleSessions() {
	m.mu.RLock()
	ids := make([]SessionID, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	now := m.clock.Now()
	for _, id := range ids {
		m.mu.RLock()
		sess, exists := m.sessions[id]
		m.mu.RUnlock()

		if !exists {
			continue
		}

		sess.stateMu.RLock()
		lastActivity := sess.LastActivity
		state := sess.State
		sess.stateMu.RUnlock()

		if state == SessionRunning && now.Sub(lastActivity) > m.policy.IdleTimeout {
			m.forceCancelInternal(context.Background(), id)
		}
	}
}

func (m *SessionManager) startIdleMonitor() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.checkIdleSessions()
		}
	}
}

type SessionError struct {
	Op      string
	SessionID SessionID
	Err      error
}

func (e *SessionError) Error() string {
	return fmt.Sprintf("session %s [%s]: %s", e.SessionID, e.Op, e.Err.Error())
}

type ReadParams struct {
	Owner         SessionOwner
	SessionID     SessionID
	AfterSequence uint64
	MaxBytes      int
}
