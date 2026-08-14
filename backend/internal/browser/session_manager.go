package browser

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ResolvedBrowserSession struct {
	SessionID         BrowserSessionID
	BrowserContextID  BrowserContextID
	RuntimeGeneration uint64
}

type SessionResolver interface {
	ResolveSession(ctx context.Context, sessionID BrowserSessionID) (ResolvedBrowserSession, *BrowserError)
}

type productionSessionManager struct {
	runtime     BrowserRuntime
	backend     BrowserSessionBackend
	store       *sessionStore
	maxSessions int
	tabCleaner  SessionTabCleaner
	mu          sync.RWMutex
}

func NewProductionSessionManager(runtime BrowserRuntime, backend BrowserSessionBackend, maxSessions int) BrowserSessionManager {
	if maxSessions <= 0 {
		maxSessions = DefaultMaxSessions
	}
	return &productionSessionManager{
		runtime:     runtime,
		backend:     backend,
		store:       newSessionStore(),
		maxSessions: maxSessions,
	}
}

func (m *productionSessionManager) SetTabCleaner(cleaner SessionTabCleaner) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tabCleaner = cleaner
}

func (m *productionSessionManager) CreateSession(ctx context.Context) (BrowserSessionInfo, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return BrowserSessionInfo{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	status := m.runtime.Status(ctx)
	if status.State != BrowserRuntimeReady {
		return BrowserSessionInfo{}, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	health := m.runtime.Health(ctx)
	if health != BrowserHealthHealthy {
		return BrowserSessionInfo{}, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not healthy",
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	currentGen := status.Generation
	activeCount := m.store.countActiveCreating(currentGen)
	if activeCount >= m.maxSessions {
		return BrowserSessionInfo{}, &BrowserError{
			Code:    ErrCodeSessionLimitReached,
			Message: "session quota reached",
		}
	}

	contextID, err := m.backend.CreateBrowserContext(ctx)
	if err != nil {
		return BrowserSessionInfo{}, &BrowserError{
			Code:    ErrCodeSessionCreateFailed,
			Message: "failed to create browser context",
			Cause:   err,
		}
	}

	sessionID := BrowserSessionID("bs_" + uuid.New().String())
	now := time.Now()
	record := &sessionRecord{
		info: BrowserSessionInfo{
			SessionID: sessionID,
			State:     SessionStateReady,
			CreatedAt: now,
		},
		contextID:         contextID,
		runtimeGeneration: currentGen,
		createdAt:         now,
		updatedAt:         now,
	}

	m.store.put(record)

	return record.info, nil
}

func (m *productionSessionManager) CloseSession(ctx context.Context, sessionID BrowserSessionID) *BrowserError {
	if err := ctx.Err(); err != nil {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if sessionID == "" {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "session ID is required",
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.store.get(sessionID)
	if !ok {
		return &BrowserError{
			Code:    ErrCodeSessionNotFound,
			Message: "session not found",
		}
	}

	if record.info.State == SessionStateClosed {
		return nil
	}

	if record.info.State == SessionStateClosing {
		return nil
	}

	currentGen := m.runtime.Status(ctx).Generation
	if record.runtimeGeneration != currentGen {
		m.store.remove(sessionID)
		return &BrowserError{
			Code:    ErrCodeSessionStale,
			Message: "session belongs to a stale runtime generation",
		}
	}

	if !m.store.transition(sessionID, record.info.State, SessionStateClosing) {
		return &BrowserError{
			Code:    ErrCodeSessionCloseFailed,
			Message: "failed to transition session state",
		}
	}

	if m.tabCleaner != nil {
		if err := m.tabCleaner.CloseAllForSession(ctx, sessionID, currentGen); err != nil {
			m.store.transition(sessionID, SessionStateClosing, SessionStateFailed)
			return err
		}
	}

	err := m.backend.DisposeBrowserContext(ctx, record.contextID)
	if err != nil {
		m.store.transition(sessionID, SessionStateClosing, SessionStateFailed)
		return &BrowserError{
			Code:    ErrCodeSessionCloseFailed,
			Message: "failed to dispose browser context",
			Cause:   err,
		}
	}

	m.store.transition(sessionID, SessionStateClosing, SessionStateClosed)

	return nil
}

func (m *productionSessionManager) GetSession(ctx context.Context, sessionID BrowserSessionID) (BrowserSessionInfo, *BrowserError) {
	if sessionID == "" {
		return BrowserSessionInfo{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "session ID is required",
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.store.get(sessionID)
	if !ok {
		return BrowserSessionInfo{}, &BrowserError{
			Code:    ErrCodeSessionNotFound,
			Message: "session not found",
		}
	}

	currentGen := m.runtime.Status(ctx).Generation
	if record.runtimeGeneration != currentGen {
		return BrowserSessionInfo{}, &BrowserError{
			Code:    ErrCodeSessionStale,
			Message: "session belongs to a stale runtime generation",
		}
	}

	if record.info.State == SessionStateClosed {
		return BrowserSessionInfo{}, &BrowserError{
			Code:    ErrCodeSessionNotFound,
			Message: "session is closed",
		}
	}

	return record.info, nil
}

func (m *productionSessionManager) ListSessions(ctx context.Context) ([]BrowserSessionInfo, *BrowserError) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	currentGen := m.runtime.Status(ctx).Generation
	records := m.store.listActive(currentGen)
	result := make([]BrowserSessionInfo, 0, len(records))
	for _, record := range records {
		result = append(result, record.info)
	}
	return result, nil
}

func (m *productionSessionManager) ResolveSession(ctx context.Context, sessionID BrowserSessionID) (ResolvedBrowserSession, *BrowserError) {
	if sessionID == "" {
		return ResolvedBrowserSession{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "session ID is required",
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.store.get(sessionID)
	if !ok {
		return ResolvedBrowserSession{}, &BrowserError{
			Code:    ErrCodeSessionNotFound,
			Message: "session not found",
		}
	}

	currentGen := m.runtime.Status(ctx).Generation
	if record.runtimeGeneration != currentGen {
		return ResolvedBrowserSession{}, &BrowserError{
			Code:    ErrCodeSessionStale,
			Message: "session belongs to a stale runtime generation",
		}
	}

	if record.info.State == SessionStateClosed {
		return ResolvedBrowserSession{}, &BrowserError{
			Code:    ErrCodeSessionNotFound,
			Message: "session is closed",
		}
	}

	return ResolvedBrowserSession{
		SessionID:         record.info.SessionID,
		BrowserContextID:  record.contextID,
		RuntimeGeneration: record.runtimeGeneration,
	}, nil
}

func (m *productionSessionManager) closeAll(ctx context.Context, generation uint64) *BrowserError {
	m.mu.Lock()
	records := m.store.listActiveGenerationLocked(generation)
	m.mu.Unlock()

	for _, record := range records {
		if record.runtimeGeneration != generation {
			continue
		}
		_ = m.backend.DisposeBrowserContext(ctx, record.contextID)
		m.store.remove(record.info.SessionID)
	}
	return nil
}

func (m *productionSessionManager) invalidateGeneration(generation uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store.clearGeneration(generation)
}

func (m *productionSessionManager) contextID(sessionID BrowserSessionID) (BrowserContextID, *BrowserError) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.store.get(sessionID)
	if !ok {
		return "", &BrowserError{
			Code:    ErrCodeSessionNotFound,
			Message: "session not found",
		}
	}
	return record.contextID, nil
}
