package sandbox_webui

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type BridgeActionDispatcher struct {
	handler func(ctx context.Context, sessionID, actionID string, input json.RawMessage) (json.RawMessage, error)
}

func NewBridgeActionDispatcher(handler func(ctx context.Context, sessionID, actionID string, input json.RawMessage) (json.RawMessage, error)) *BridgeActionDispatcher {
	return &BridgeActionDispatcher{handler: handler}
}

func (d *BridgeActionDispatcher) DispatchAction(ctx context.Context, sessionID, actionID string, input json.RawMessage) (json.RawMessage, error) {
	if d.handler == nil {
		return json.Marshal(map[string]any{"ok": false, "error": "no_action_handler"})
	}
	return d.handler(ctx, sessionID, actionID, input)
}

type BridgeDataSourceProvider struct {
	mu       sync.RWMutex
	data     map[string]json.RawMessage
	channels map[string][]chan json.RawMessage
	handler  func(ctx context.Context, sessionID, sourceID string, params json.RawMessage) (json.RawMessage, error)
}

func NewBridgeDataSourceProvider() *BridgeDataSourceProvider {
	return &BridgeDataSourceProvider{
		data:     make(map[string]json.RawMessage),
		channels: make(map[string][]chan json.RawMessage),
	}
}

func NewBridgeDataSourceProviderWithHandler(handler func(ctx context.Context, sessionID, sourceID string, params json.RawMessage) (json.RawMessage, error)) *BridgeDataSourceProvider {
	return &BridgeDataSourceProvider{
		data:     make(map[string]json.RawMessage),
		channels: make(map[string][]chan json.RawMessage),
		handler:  handler,
	}
}

func (p *BridgeDataSourceProvider) FetchData(ctx context.Context, sessionID, sourceID string, params json.RawMessage) (json.RawMessage, error) {
	if p.handler != nil {
		return p.handler(ctx, sessionID, sourceID, params)
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if data, exists := p.data[sourceID]; exists {
		return data, nil
	}
	return json.Marshal(map[string]any{
		"sourceId": sourceID,
		"params":   params,
		"result":   nil,
		"cached":   false,
	})
}

func (p *BridgeDataSourceProvider) Subscribe(ctx context.Context, sessionID, sourceID string, params json.RawMessage, callback func(payload json.RawMessage)) (*DataSubscription, error) {
	sub := &DataSubscription{
		SubscriptionID: newSubscriptionID(),
		DataSourceID:   sourceID,
		LastUpdate:     time.Now().UTC(),
		Active:         true,
		RatePerMinute:  10,
	}
	return sub, nil
}

func (p *BridgeDataSourceProvider) SetData(sourceID string, data json.RawMessage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.data[sourceID] = data
}

type BridgeNavigator struct {
	handler func(ctx context.Context, sessionID, target string) error
}

func NewBridgeNavigator(handler func(ctx context.Context, sessionID, target string) error) *BridgeNavigator {
	return &BridgeNavigator{handler: handler}
}

func (n *BridgeNavigator) Navigate(ctx context.Context, sessionID, target string) error {
	if n.handler == nil {
		return nil
	}
	return n.handler(ctx, sessionID, target)
}

func newSubscriptionID() string {
	b := make([]byte, 12)
	_, _ = readRandom(b)
	return "sub_" + bytesToHex(b)
}

type SessionInfo struct {
	SessionID      string                 `json:"sessionId"`
	ExtensionID    string                 `json:"extensionId"`
	ModuleID       string                 `json:"moduleId"`
	Generation     int64                  `json:"generation"`
	State          SessionState           `json:"state"`
	Origin         string                 `json:"origin"`
	SlotID         string                 `json:"slotId"`
	CreatedAt      time.Time              `json:"createdAt"`
	ExpiresAt      time.Time              `json:"expiresAt"`
	LastActiveAt   time.Time              `json:"lastActiveAt"`
	Sandbox        SandboxType            `json:"sandbox"`
	AllowedActions []string               `json:"allowedActions"`
	AllowedDataSources []string           `json:"allowedDataSources"`
}

func (h *Host) GetSessionInfo(sessionID string) (*SessionInfo, error) {
	session, err := h.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return &SessionInfo{
		SessionID:      session.SessionID,
		ExtensionID:    session.ExtensionID,
		ModuleID:       session.ModuleID,
		Generation:     session.Generation,
		State:          session.State,
		Origin:         session.Origin,
		SlotID:         session.SlotID,
		CreatedAt:      session.CreatedAt,
		ExpiresAt:      session.ExpiresAt,
		LastActiveAt:   session.LastActiveAt,
		Sandbox:        session.Sandbox,
		AllowedActions: session.AllowedActions,
		AllowedDataSources: session.AllowedDataSources,
	}, nil
}

func (h *Host) ListSessions() []*SessionInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]*SessionInfo, 0, len(h.sessions))
	for _, session := range h.sessions {
		session.mu.Lock()
		result = append(result, &SessionInfo{
			SessionID:      session.SessionID,
			ExtensionID:    session.ExtensionID,
			ModuleID:       session.ModuleID,
			Generation:     session.Generation,
			State:          session.State,
			Origin:         session.Origin,
			SlotID:         session.SlotID,
			CreatedAt:      session.CreatedAt,
			ExpiresAt:      session.ExpiresAt,
			LastActiveAt:   session.LastActiveAt,
			Sandbox:        session.Sandbox,
			AllowedActions: session.AllowedActions,
			AllowedDataSources: session.AllowedDataSources,
		})
		session.mu.Unlock()
	}
	return result
}

func (h *Host) QuarantineSession(sessionID, reason string) error {
	h.mu.Lock()
	session, exists := h.sessions[sessionID]
	if !exists {
		h.mu.Unlock()
		return ErrSessionNotFound
	}
	h.mu.Unlock()
	session.mu.Lock()
	defer session.mu.Unlock()
	session.State = SessionStateQuarantined
	session.bridgeClosed = true
	for _, sub := range session.subscriptions {
		sub.Active = false
	}
	h.auditLog(AuditEntry{
		Timestamp: time.Now().UTC(),
		SessionID: sessionID,
		Extension: session.ExtensionID,
		Method:    "session.quarantine",
		Success:   true,
		Error:     reason,
	})
	return nil
}

func (h *Host) GetPreloadScript(sessionID string) (string, error) {
	session, err := h.GetSession(sessionID)
	if err != nil {
		return "", err
	}
	pb := NewPreloadBuilder()
	return pb.Build(session)
}

func (h *Host) GetResource(extensionID, moduleID, path string) (*ProtocolResource, error) {
	return h.protocol.Resolve(extensionID, moduleID, path)
}

func (h *Host) RegisterResource(res *ProtocolResource) error {
	return h.protocol.RegisterResource(res)
}

func (h *Host) TouchSession(sessionID string) error {
	session, err := h.GetSession(sessionID)
	if err != nil {
		return err
	}
	session.mu.Lock()
	session.LastActiveAt = time.Now().UTC()
	session.mu.Unlock()
	return nil
}

func (h *Host) CleanupExpiredSessions() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	count := 0
	for sid, session := range h.sessions {
		if session.IsExpired() {
			session.mu.Lock()
			session.State = SessionStateExpired
			session.bridgeClosed = true
			session.mu.Unlock()
			delete(h.sessions, sid)
			count++
		}
	}
	return count
}

type SessionStats struct {
	Total     int `json:"total"`
	Active    int `json:"active"`
	Suspended int `json:"suspended"`
	Closed    int `json:"closed"`
	Failed    int `json:"failed"`
	Quarantined int `json:"quarantined"`
}

func (h *Host) GetStats() SessionStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	stats := SessionStats{Total: len(h.sessions)}
	for _, session := range h.sessions {
		session.mu.Lock()
		switch session.State {
		case SessionStateActive, SessionStateReady:
			stats.Active++
		case SessionStateSuspended:
			stats.Suspended++
		case SessionStateClosed:
			stats.Closed++
		case SessionStateFailed:
			stats.Failed++
		case SessionStateQuarantined:
			stats.Quarantined++
		}
		session.mu.Unlock()
	}
	return stats
}

var _ = fmt.Sprintf
