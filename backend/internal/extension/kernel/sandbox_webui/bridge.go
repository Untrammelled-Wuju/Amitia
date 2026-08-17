package sandbox_webui

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ActionDispatcher interface {
	DispatchAction(ctx context.Context, sessionID, actionID string, input json.RawMessage) (json.RawMessage, error)
}

type DataSourceProvider interface {
	FetchData(ctx context.Context, sessionID, sourceID string, params json.RawMessage) (json.RawMessage, error)
	Subscribe(ctx context.Context, sessionID, sourceID string, params json.RawMessage, callback func(payload json.RawMessage)) (*DataSubscription, error)
}

type Navigator interface {
	Navigate(ctx context.Context, sessionID, target string) error
}

type Bridge struct {
	host            *Host
	dispatcher      ActionDispatcher
	dataProvider    DataSourceProvider
	navigator       Navigator
	mu              sync.RWMutex
	pendingRequests map[string]chan BridgeMessage
	rateLimiter     *RateLimiter
}

func NewBridge(host *Host, dispatcher ActionDispatcher, provider DataSourceProvider, navigator Navigator) *Bridge {
	return &Bridge{
		host:            host,
		dispatcher:      dispatcher,
		dataProvider:    provider,
		navigator:       navigator,
		pendingRequests: make(map[string]chan BridgeMessage),
		rateLimiter:     NewRateLimiter(),
	}
}

type InvokeRequest struct {
	SessionID string
	Message   BridgeMessage
}

type InvokeResult struct {
	Output json.RawMessage
	Error  string
}

func (b *Bridge) HandleMessage(ctx context.Context, req InvokeRequest) (*InvokeResult, error) {
	session, err := b.host.GetSession(req.SessionID)
	if err != nil {
		return nil, err
	}
	if err := ValidateBridgeMessage(&req.Message, session); err != nil {
		b.host.auditLog(AuditEntry{
			Timestamp: time.Now().UTC(),
			SessionID: req.SessionID,
			Extension: session.ExtensionID,
			Method:    req.Message.Method,
			Success:   false,
			Error:     err.Error(),
			BytesIn:   req.Message.Size,
		})
		return nil, err
	}

	if b.rateLimiter != nil {
		if !b.rateLimiter.Allow(session.SessionID, BridgeMethod(req.Message.Method)) {
			b.host.auditLog(AuditEntry{
				Timestamp: time.Now().UTC(),
				SessionID: req.SessionID,
				Extension: session.ExtensionID,
				Method:    req.Message.Method,
				Success:   false,
				Error:     "rate_limited",
				BytesIn:   req.Message.Size,
			})
			return &InvokeResult{Error: ErrBridgeRateLimited.Error()}, nil
		}
	}

	var result json.RawMessage
	var dispatchErr error
	method := BridgeMethod(req.Message.Method)

	switch method {
	case MethodReady:
		result, dispatchErr = b.handleReady(ctx, session)
	case MethodContextGet:
		result, dispatchErr = b.handleContextGet(ctx, session)
	case MethodActionInvoke:
		result, dispatchErr = b.handleActionInvoke(ctx, session, req.Message)
	case MethodDataRequest:
		result, dispatchErr = b.handleDataRequest(ctx, session, req.Message)
	case MethodDataSubscribe:
		result, dispatchErr = b.handleDataSubscribe(ctx, session, req.Message)
	case MethodNavigate:
		result, dispatchErr = b.handleNavigate(ctx, session, req.Message)
	case MethodResize:
		result, dispatchErr = b.handleResize(ctx, session, req.Message)
	case MethodResourceOpen:
		result, dispatchErr = b.handleResourceOpen(ctx, session, req.Message)
	case MethodResourceRead:
		result, dispatchErr = b.handleResourceRead(ctx, session, req.Message)
	case MethodArtifactCreate:
		result, dispatchErr = b.handleArtifactCreate(ctx, session, req.Message)
	case MethodLog:
		result, dispatchErr = b.handleLog(ctx, session, req.Message)
	case MethodSessionPing:
		result, dispatchErr = b.handleSessionPing(ctx, session)
	case MethodClipboardRead:
		result, dispatchErr = b.handleClipboardRead(ctx, session, req.Message)
	case MethodClipboardWrite:
		result, dispatchErr = b.handleClipboardWrite(ctx, session, req.Message)
	case MethodNetwork:
		result, dispatchErr = b.handleNetwork(ctx, session, req.Message)
	case MethodStorage:
		result, dispatchErr = b.handleStorage(ctx, session, req.Message)
	case MethodDialog:
		result, dispatchErr = b.handleDialog(ctx, session, req.Message)
	default:
		dispatchErr = ErrMethodNotAllowed
	}

	b.host.auditLog(AuditEntry{
		Timestamp: time.Now().UTC(),
		SessionID: req.SessionID,
		Extension: session.ExtensionID,
		Method:    req.Message.Method,
		Success:   dispatchErr == nil,
		Error: func() string {
			if dispatchErr != nil {
				return dispatchErr.Error()
			}
			return ""
		}(),
		BytesIn:  req.Message.Size,
		BytesOut: len(result),
	})

	if dispatchErr != nil {
		return &InvokeResult{Error: dispatchErr.Error()}, nil
	}
	return &InvokeResult{Output: result}, nil
}

func (b *Bridge) handleReady(ctx context.Context, session *WebSession) (json.RawMessage, error) {
	session.SetState(SessionStateActive)
	readyPayload := map[string]any{
		"sessionId":      session.SessionID,
		"contributionId": session.ContributionID,
		"slotId":         session.SlotID,
		"theme":          session.Theme,
		"locale":         session.Locale,
		"actions":        session.AllowedActions,
		"dataSources":    session.AllowedDataSources,
		"generation":     session.Generation,
	}
	return json.Marshal(readyPayload)
}

func (b *Bridge) handleContextGet(ctx context.Context, session *WebSession) (json.RawMessage, error) {
	host := "web"
	if session.Surface != "" {
		host = session.Surface
	}
	os := "unknown"
	platform := "web"
	capabilities := []string{"browser"}
	if session.CharacterID != "" {
		capabilities = append(capabilities, "character")
	}
	if session.ConversationID != "" {
		capabilities = append(capabilities, "conversation")
	}
	contextPayload := map[string]any{
		"theme":    session.Theme,
		"locale":   session.Locale,
		"platform": platform,
		"host":     host,
		"os":       os,
		"surface":  session.SlotID,
		"characterId": session.CharacterID,
		"conversationId": session.ConversationID,
		"capabilities": capabilities,
		"scope": map[string]any{
			"extensionId": session.ExtensionID,
			"moduleId":    session.ModuleID,
		},
		"generation": session.Generation,
	}
	return json.Marshal(contextPayload)
}

func (b *Bridge) handleSessionPing(ctx context.Context, session *WebSession) (json.RawMessage, error) {
	session.mu.Lock()
	session.LastActiveAt = time.Now().UTC()
	session.mu.Unlock()
	return json.Marshal(map[string]any{
		"ok":        true,
		"timestamp": time.Now().UTC(),
		"expiresAt": session.ExpiresAt,
	})
}

type actionInvokeInput struct {
	ActionID string          `json:"actionId"`
	Input    json.RawMessage `json:"input"`
}

func (b *Bridge) handleActionInvoke(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	var in actionInvokeInput
	if err := json.Unmarshal(msg.Input, &in); err != nil {
		return nil, ErrInvalidMessage
	}
	if !session.IsActionAllowed(in.ActionID) {
		return nil, ErrActionNotDeclared
	}
	if b.dispatcher == nil {
		return nil, ErrDispatcherUnavailable
	}
	result, err := b.dispatcher.DispatchAction(ctx, session.SessionID, in.ActionID, in.Input)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type dataRequestInput struct {
	SourceID string          `json:"sourceId"`
	Params   json.RawMessage `json:"params"`
}

func (b *Bridge) handleDataRequest(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	var in dataRequestInput
	if err := json.Unmarshal(msg.Input, &in); err != nil {
		return nil, ErrInvalidMessage
	}
	if !session.IsDataSourceAllowed(in.SourceID) {
		return nil, ErrDataSourceNotDeclared
	}
	if b.dataProvider == nil {
		return nil, ErrDataProviderUnavailable
	}
	return b.dataProvider.FetchData(ctx, session.SessionID, in.SourceID, in.Params)
}

type dataSubscribeInput struct {
	SourceID string          `json:"sourceId"`
	Params   json.RawMessage `json:"params"`
	Rate     int             `json:"ratePerMinute"`
}

func (b *Bridge) handleDataSubscribe(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	var in dataSubscribeInput
	if err := json.Unmarshal(msg.Input, &in); err != nil {
		return nil, ErrInvalidMessage
	}
	if !session.IsDataSourceAllowed(in.SourceID) {
		return nil, ErrDataSourceNotDeclared
	}
	if in.Rate <= 0 || in.Rate > 60 {
		in.Rate = 10
	}
	if b.dataProvider == nil {
		return nil, ErrDataProviderUnavailable
	}
	sub, err := b.dataProvider.Subscribe(ctx, session.SessionID, in.SourceID, in.Params, func(payload json.RawMessage) {
		if session.IsExpired() {
			return
		}
	})
	if err != nil {
		return nil, err
	}
	if err := session.AddSubscription(sub); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"subscriptionId": sub.SubscriptionID,
		"sourceId":       in.SourceID,
	})
}

type navigateInput struct {
	Target string `json:"target"`
	Type   string `json:"type"`
}

func (b *Bridge) handleNavigate(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	var in navigateInput
	if err := json.Unmarshal(msg.Input, &in); err != nil {
		return nil, ErrInvalidMessage
	}
	if !isNavigationAllowed(in.Target, session) {
		return nil, ErrNavigationDenied
	}
	if b.navigator != nil {
		if err := b.navigator.Navigate(ctx, session.SessionID, in.Target); err != nil {
			return nil, err
		}
	}
	return json.Marshal(map[string]any{"ok": true})
}

type resizeInput struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (b *Bridge) handleResize(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	var in resizeInput
	if err := json.Unmarshal(msg.Input, &in); err != nil {
		return nil, ErrInvalidMessage
	}
	if in.Width < 100 || in.Width > 4096 || in.Height < 100 || in.Height > 4096 {
		return nil, ErrInvalidResize
	}
	if err := session.RecordResize(); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"ok": true, "width": in.Width, "height": in.Height})
}

type resourceOpenInput struct {
	HandleID string `json:"handleId"`
}

func (b *Bridge) handleResourceOpen(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	var in resourceOpenInput
	if err := json.Unmarshal(msg.Input, &in); err != nil {
		return nil, ErrInvalidMessage
	}
	handle, err := session.ConsumeResourceHandle(in.HandleID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"handleId": handle.HandleID,
		"mimeType": handle.MIME,
		"size":     handle.Size,
	})
}

type resourceReadInput struct {
	HandleID string `json:"handleId"`
	Offset   int64  `json:"offset"`
	Length   int64  `json:"length"`
}

func (b *Bridge) handleResourceRead(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	var in resourceReadInput
	if err := json.Unmarshal(msg.Input, &in); err != nil {
		return nil, ErrInvalidMessage
	}
	handle, err := session.ConsumeResourceHandle(in.HandleID)
	if err != nil {
		return nil, err
	}
	if !handle.ReadOnly {
		return nil, ErrResourcePathForbidden
	}
	return json.Marshal(map[string]any{
		"handleId": handle.HandleID,
		"path":     handle.Path,
		"mimeType": handle.MIME,
		"size":     handle.Size,
		"readOnly": handle.ReadOnly,
	})
}

type artifactCreateInput struct {
	ContentType string          `json:"contentType"`
	Data        json.RawMessage `json:"data"`
	Filename    string          `json:"filename"`
}

func (b *Bridge) handleArtifactCreate(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	var in artifactCreateInput
	if err := json.Unmarshal(msg.Input, &in); err != nil {
		return nil, ErrInvalidMessage
	}
	if in.ContentType == "" {
		return nil, ErrInvalidMessage
	}
	if !IsMIMEAllowed(in.ContentType) {
		return nil, ErrMimeNotAllowed
	}
	if len(in.Data) > 10*1024*1024 {
		return nil, ErrBundleTooLarge
	}
	handle := newResourceHandleID()
	return json.Marshal(map[string]any{
		"artifactId":  handle,
		"contentType": in.ContentType,
		"size":        len(in.Data),
		"filename":    in.Filename,
	})
}

func (b *Bridge) handleLog(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]any{"ok": true})
}

func (b *Bridge) handleClipboardRead(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	return nil, ErrClipboardDenied
}

func (b *Bridge) handleClipboardWrite(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	return nil, ErrClipboardDenied
}

type networkInput struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

func (b *Bridge) handleNetwork(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	var in networkInput
	_ = json.Unmarshal(msg.Input, &in)
	return json.Marshal(map[string]any{
		"ok":    false,
		"error": "network_access_denied",
		"code":  "NETWORK_NOT_ALLOWED",
	})
}

func (b *Bridge) handleStorage(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	return nil, ErrStorageDenied
}

func (b *Bridge) handleDialog(ctx context.Context, session *WebSession, msg BridgeMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]any{"ok": true})
}

func (b *Bridge) SendHostEvent(sessionID, eventType string, payload json.RawMessage) error {
	session, err := b.host.GetSession(sessionID)
	if err != nil {
		return err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.bridgeClosed {
		return ErrBridgeClosed
	}
	return nil
}

func (b *Bridge) UpdateSessionContext(sessionID, characterID, conversationID string) error {
	session, err := b.host.GetSession(sessionID)
	if err != nil {
		return err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if characterID != "" {
		session.CharacterID = characterID
	}
	if conversationID != "" {
		session.ConversationID = conversationID
	}
	return nil
}

func (b *Bridge) UpdateSessionTheme(sessionID string, theme ThemeSnapshot) error {
	session, err := b.host.GetSession(sessionID)
	if err != nil {
		return err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	session.Theme = theme
	return nil
}

func isNavigationAllowed(target string, session *WebSession) bool {
	return IsNavigationTargetAllowed(target, session)
}

func IsNavigationTargetAllowed(target string, session *WebSession) bool {
	if target == "" {
		return false
	}
	if strings.Contains(target, "://") {
		u, err := url.Parse(target)
		if err != nil {
			return false
		}
		if u.Scheme == ProtocolScheme && u.Host == session.ExtensionID {
			return true
		}
		return false
	}
	if strings.Contains(target, "..") {
		return false
	}
	return true
}

func isURLAllowed(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	blocked := []string{"localhost", "127.0.0.1", "0.0.0.0", "::1"}
	for _, b := range blocked {
		if host == b {
			return false
		}
	}
	privateSuffixes := []string{".local", ".internal"}
	for _, s := range privateSuffixes {
		if strings.HasSuffix(host, s) {
			return false
		}
	}
	return true
}

var (
	ErrDispatcherUnavailable   = errors.New("sandbox_webui: dispatcher unavailable")
	ErrDataProviderUnavailable = errors.New("sandbox_webui: data provider unavailable")
	ErrNavigationDenied        = errors.New("sandbox_webui: navigation denied")
	ErrInvalidResize           = errors.New("sandbox_webui: invalid resize dimensions")
	ErrClipboardDenied         = errors.New("sandbox_webui: clipboard denied")
	ErrNetworkDenied           = errors.New("sandbox_webui: network denied")
	ErrStorageDenied           = errors.New("sandbox_webui: storage denied")
)
