package browser

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type productionTabManager struct {
	resolver          SessionResolver
	backend           BrowserTabBackend
	store             *tabStore
	maxTabsPerSession int
	maxTabsTotal      int
	mu                sync.RWMutex
}

func NewProductionTabManager(resolver SessionResolver, backend BrowserTabBackend, maxTabsPerSession, maxTabsTotal int) BrowserTabManager {
	if maxTabsPerSession <= 0 {
		maxTabsPerSession = DefaultMaxTabsPerSession
	}
	if maxTabsTotal <= 0 {
		maxTabsTotal = DefaultMaxTabsTotal
	}
	return &productionTabManager{
		resolver:          resolver,
		backend:           backend,
		store:             newTabStore(),
		maxTabsPerSession: maxTabsPerSession,
		maxTabsTotal:      maxTabsTotal,
	}
}

func (m *productionTabManager) CreateTab(ctx context.Context, sessionID BrowserSessionID) (BrowserTabInfo, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return BrowserTabInfo{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if sessionID == "" {
		return BrowserTabInfo{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "session ID is required",
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	resolved, err := m.resolver.ResolveSession(ctx, sessionID)
	if err != nil {
		return BrowserTabInfo{}, err
	}

	activeInSession := m.store.countBySession(sessionID, resolved.RuntimeGeneration)
	if activeInSession >= m.maxTabsPerSession {
		return BrowserTabInfo{}, &BrowserError{
			Code:    ErrCodeTabQuotaReached,
			Message: "tab quota for session reached",
		}
	}

	activeTotal := m.store.countActive(resolved.RuntimeGeneration)
	if activeTotal >= m.maxTabsTotal {
		return BrowserTabInfo{}, &BrowserError{
			Code:    ErrCodeTabQuotaReached,
			Message: "total tab quota reached",
		}
	}

	targetID, backendErr := m.backend.CreateTarget(ctx, resolved.BrowserContextID, "about:blank")
	if backendErr != nil {
		return BrowserTabInfo{}, &BrowserError{
			Code:    ErrCodeTabCreateFailed,
			Message: "failed to create browser target",
			Cause:   backendErr,
		}
	}

	tabID := BrowserTabID("bt_" + uuid.New().String())
	now := time.Now()
	record := &tabRecord{
		info: BrowserTabInfo{
			TabID:     tabID,
			SessionID: sessionID,
			State:     TabStateReady,
			URL:       "about:blank",
			Active:    false,
			CreatedAt: now,
			UpdatedAt: now,
		},
		sessionID:         sessionID,
		browserContextID:  resolved.BrowserContextID,
		targetID:          targetID,
		runtimeGeneration: resolved.RuntimeGeneration,
		createdAt:         now,
		updatedAt:         now,
	}

	m.store.put(record)

	return record.info, nil
}

func (m *productionTabManager) CloseTab(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) *BrowserError {
	if err := ctx.Err(); err != nil {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if tabID == "" {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "tab ID is required",
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.store.get(tabID)
	if !ok {
		return &BrowserError{
			Code:    ErrCodeTabNotFound,
			Message: "tab not found",
		}
	}

	if record.sessionID != sessionID {
		return &BrowserError{
			Code:    ErrCodeTabNotFound,
			Message: "tab does not belong to session",
		}
	}

	if record.info.State == TabStateClosed {
		return nil
	}

	if record.info.State == TabStateClosing {
		return nil
	}

	resolved, err := m.resolver.ResolveSession(ctx, sessionID)
	if err != nil {
		return &BrowserError{
			Code:    ErrCodeSessionStale,
			Message: "session is stale",
		}
	}

	if record.runtimeGeneration != resolved.RuntimeGeneration {
		m.store.remove(tabID)
		return &BrowserError{
			Code:    ErrCodeTabStale,
			Message: "tab belongs to a stale runtime generation",
		}
	}

	if !m.store.transition(tabID, record.info.State, TabStateClosing) {
		return &BrowserError{
			Code:    ErrCodeTabCloseFailed,
			Message: "failed to transition tab state",
		}
	}

	backendErr := m.backend.CloseTarget(ctx, record.targetID)
	if backendErr != nil {
		m.store.transition(tabID, TabStateClosing, TabStateFailed)
		return &BrowserError{
			Code:    ErrCodeTabCloseFailed,
			Message: "failed to close browser target",
			Cause:   backendErr,
		}
	}

	m.store.transition(tabID, TabStateClosing, TabStateClosed)
	m.store.remove(tabID)

	return nil
}

func (m *productionTabManager) GetTab(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) (BrowserTabInfo, *BrowserError) {
	if tabID == "" {
		return BrowserTabInfo{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "tab ID is required",
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.store.get(tabID)
	if !ok {
		return BrowserTabInfo{}, &BrowserError{
			Code:    ErrCodeTabNotFound,
			Message: "tab not found",
		}
	}

	if record.sessionID != sessionID {
		return BrowserTabInfo{}, &BrowserError{
			Code:    ErrCodeTabNotFound,
			Message: "tab does not belong to session",
		}
	}

	resolved, err := m.resolver.ResolveSession(ctx, sessionID)
	if err != nil {
		return BrowserTabInfo{}, &BrowserError{
			Code:    ErrCodeSessionStale,
			Message: "session is stale",
		}
	}

	if record.runtimeGeneration != resolved.RuntimeGeneration {
		return BrowserTabInfo{}, &BrowserError{
			Code:    ErrCodeTabStale,
			Message: "tab belongs to a stale runtime generation",
		}
	}

	if record.info.State == TabStateClosed {
		return BrowserTabInfo{}, &BrowserError{
			Code:    ErrCodeTabNotFound,
			Message: "tab is closed",
		}
	}

	return record.info, nil
}

func (m *productionTabManager) ListTabs(ctx context.Context, sessionID BrowserSessionID) ([]BrowserTabInfo, *BrowserError) {
	if sessionID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "session ID is required",
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	resolved, err := m.resolver.ResolveSession(ctx, sessionID)
	if err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeSessionStale,
			Message: "session is stale",
		}
	}

	records := m.store.getBySession(sessionID, resolved.RuntimeGeneration)
	result := make([]BrowserTabInfo, 0, len(records))
	for _, record := range records {
		result = append(result, record.info)
	}
	return result, nil
}

func (m *productionTabManager) ActivateTab(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) *BrowserError {
	if tabID == "" {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "tab ID is required",
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.store.get(tabID)
	if !ok {
		return &BrowserError{
			Code:    ErrCodeTabNotFound,
			Message: "tab not found",
		}
	}

	if record.sessionID != sessionID {
		return &BrowserError{
			Code:    ErrCodeTabNotFound,
			Message: "tab does not belong to session",
		}
	}

	resolved, err := m.resolver.ResolveSession(ctx, sessionID)
	if err != nil {
		return &BrowserError{
			Code:    ErrCodeSessionStale,
			Message: "session is stale",
		}
	}

	if record.runtimeGeneration != resolved.RuntimeGeneration {
		return &BrowserError{
			Code:    ErrCodeTabStale,
			Message: "tab belongs to a stale runtime generation",
		}
	}

	if record.info.State == TabStateClosed {
		return &BrowserError{
			Code:    ErrCodeTabNotFound,
			Message: "tab is closed",
		}
	}

	backendErr := m.backend.ActivateTarget(ctx, record.targetID)
	if backendErr != nil {
		return &BrowserError{
			Code:    ErrCodeTabActivateFailed,
			Message: "failed to activate tab",
			Cause:   backendErr,
		}
	}

	m.store.clearActive(sessionID)
	m.store.updateTabInfo(tabID, "", "", true)

	return nil
}

func (m *productionTabManager) CloseAllForSession(ctx context.Context, sessionID BrowserSessionID, generation uint64) *BrowserError {
	m.mu.Lock()
	records := m.store.closeAllForSession(sessionID, generation)
	m.mu.Unlock()

	for _, record := range records {
		if record.info.State == TabStateClosed {
			continue
		}
		_ = m.backend.CloseTarget(ctx, record.targetID)
	}
	return nil
}

func (m *productionTabManager) invalidateGeneration(generation uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store.clearGeneration(generation)
}

func (m *productionTabManager) ResolveTab(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) (ResolvedBrowserTab, *BrowserError) {
	if tabID == "" {
		return ResolvedBrowserTab{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "tab ID is required",
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.store.get(tabID)
	if !ok {
		return ResolvedBrowserTab{}, &BrowserError{
			Code:    ErrCodeTabNotFound,
			Message: "tab not found",
		}
	}

	if record.sessionID != sessionID {
		return ResolvedBrowserTab{}, &BrowserError{
			Code:    ErrCodeTabNotFound,
			Message: "tab does not belong to session",
		}
	}

	resolved, err := m.resolver.ResolveSession(ctx, sessionID)
	if err != nil {
		return ResolvedBrowserTab{}, &BrowserError{
			Code:    ErrCodeSessionStale,
			Message: "session is stale",
		}
	}

	if record.runtimeGeneration != resolved.RuntimeGeneration {
		return ResolvedBrowserTab{}, &BrowserError{
			Code:    ErrCodeTabStale,
			Message: "tab belongs to a stale runtime generation",
		}
	}

	if record.info.State == TabStateClosed {
		return ResolvedBrowserTab{}, &BrowserError{
			Code:    ErrCodeTabNotFound,
			Message: "tab is closed",
		}
	}

	return ResolvedBrowserTab{
		SessionID:        record.sessionID,
		TabID:            record.info.TabID,
		BrowserContextID: record.browserContextID,
		TargetID:         record.targetID,
		RuntimeGeneration: record.runtimeGeneration,
	}, nil
}

func (m *productionTabManager) targetID(tabID BrowserTabID) (TargetID, *BrowserError) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.store.get(tabID)
	if !ok {
		return "", &BrowserError{
			Code:    ErrCodeTabNotFound,
			Message: "tab not found",
		}
	}
	return record.targetID, nil
}
