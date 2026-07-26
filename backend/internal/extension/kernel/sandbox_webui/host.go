package sandbox_webui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	ProtocolScheme        = "amitia-extension"
	ResourceProtocolScheme = "amitia-resource"
	DefaultCSP            = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' amitia-resource:; font-src 'self'; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'"
	MaxBundleBytes        = 50 * 1024 * 1024
	MaxSessionDuration    = 24 * time.Hour
	MaxMessageBytes       = 256 * 1024
	MaxResourceHandleTTL  = 1 * time.Hour
	MaxResizePerMinute    = 30
	MaxDataSubscriptions  = 16
)

type SandboxType string

const (
	SandboxWebRestricted SandboxType = "web_restricted"
	SandboxWebIsolated   SandboxType = "web_isolated"
)

type SessionState string

const (
	SessionStateCreated   SessionState = "created"
	SessionStateLoading   SessionState = "loading"
	SessionStateReady     SessionState = "ready"
	SessionStateSuspended SessionState = "suspended"
	SessionStateFailed    SessionState = "failed"
	SessionStateClosed    SessionState = "closed"
)

type WebSession struct {
	SessionID     string
	ContributionID string
	ExtensionID   string
	ModuleID      string
	Generation    int64
	SlotID        string
	Origin        string
	Nonce         string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	State         SessionState
	Sandbox       SandboxType
	CSP           string
	AllowedActions []string
	AllowedDataSources []string
	Theme         ThemeSnapshot
	Locale        string
	mu            sync.Mutex
	subscriptions map[string]*DataSubscription
	resourceHandles map[string]*ResourceHandle
	resizeCount   map[time.Time]int
	bridgeClosed  bool
}

type ThemeSnapshot struct {
	Mode    string            `json:"mode"`
	Density string            `json:"density"`
	Tokens  map[string]string `json:"tokens"`
}

type DataSubscription struct {
	SubscriptionID string
	DataSourceID   string
	LastPayload    json.RawMessage
	LastUpdate     time.Time
	Active         bool
	RatePerMinute  int
	mu             sync.Mutex
	history        []time.Time
}

type ResourceHandle struct {
	HandleID    string
	Path        string
	MIME        string
	Size        int64
	CreatedAt   time.Time
	ExpiresAt   time.Time
	ReadOnly    bool
	Consumed    bool
}

type BridgeMessage struct {
	Method   string          `json:"method"`
	Version  int             `json:"version"`
	ID       string          `json:"id"`
	WindowID string          `json:"windowId"`
	Origin   string          `json:"origin"`
	Nonce    string          `json:"nonce"`
	Input    json.RawMessage `json:"input,omitempty"`
	Output   json.RawMessage `json:"output,omitempty"`
	Deadline time.Time       `json:"deadline,omitempty"`
	Size     int             `json:"size"`
	Session  string          `json:"session"`
}

type BridgeMethod string

const (
	MethodReady         BridgeMethod = "ui.ready"
	MethodActionInvoke  BridgeMethod = "ui.action.invoke"
	MethodDataRequest   BridgeMethod = "ui.data.request"
	MethodDataSubscribe BridgeMethod = "ui.data.subscribe"
	MethodNavigate      BridgeMethod = "ui.navigation.request"
	MethodResize        BridgeMethod = "ui.resize.request"
	MethodDialog        BridgeMethod = "ui.dialog.request"
	MethodResourceOpen  BridgeMethod = "ui.resource.open"
	MethodLog           BridgeMethod = "ui.log"
	MethodClipboardRead BridgeMethod = "ui.clipboard.read"
	MethodClipboardWrite BridgeMethod = "ui.clipboard.write"
	MethodNetwork       BridgeMethod = "ui.network.request"
	MethodStorage       BridgeMethod = "ui.storage"
)

var allowedMethods = map[BridgeMethod]bool{
	MethodReady: true, MethodActionInvoke: true, MethodDataRequest: true,
	MethodDataSubscribe: true, MethodNavigate: true, MethodResize: true,
	MethodDialog: true, MethodResourceOpen: true, MethodLog: true,
	MethodClipboardRead: true, MethodClipboardWrite: true,
	MethodNetwork: true, MethodStorage: true,
}

type Host struct {
	mu          sync.RWMutex
	sessions    map[string]*WebSession
	protocol    *ProtocolHandler
	verifier    *BundleVerifier
	auditLog    func(entry AuditEntry)
	cspReporter func(sessionID, violation string)
	closed      bool
}

type AuditEntry struct {
	Timestamp time.Time
	SessionID string
	Extension string
	Method    string
	Success   bool
	Error     string
	BytesIn   int
	BytesOut  int
}

func NewHost() *Host {
	return &Host{
		sessions: make(map[string]*WebSession),
		protocol: NewProtocolHandler(),
		verifier: NewBundleVerifier(),
		auditLog: func(entry AuditEntry) {},
		cspReporter: func(sessionID, violation string) {},
	}
}

func (h *Host) SetAuditLogger(fn func(entry AuditEntry)) {
	h.auditLog = fn
}

func (h *Host) SetCSPReporter(fn func(sessionID, violation string)) {
	h.cspReporter = fn
}

type CreateSessionRequest struct {
	ContributionID string
	ExtensionID    string
	ModuleID       string
	Generation     int64
	SlotID         string
	Sandbox        SandboxType
	CSP            string
	AllowedActions []string
	AllowedDataSources []string
	Theme          ThemeSnapshot
	Locale         string
	BasePath       string
	EntryPath      string
}

type CreateSessionResult struct {
	SessionID string
	Origin    string
	Nonce     string
	CSP       string
	EntryURL  string
}

func (h *Host) CreateSession(req CreateSessionRequest) (*CreateSessionResult, error) {
	if h.IsClosed() {
		return nil, ErrHostClosed
	}
	if req.ExtensionID == "" || req.ModuleID == "" || req.SlotID == "" {
		return nil, ErrInvalidRequest
	}
	if req.Sandbox != SandboxWebRestricted && req.Sandbox != SandboxWebIsolated {
		return nil, ErrInvalidSandboxType
	}
	if req.EntryPath == "" {
		return nil, ErrEntryMissing
	}
	cleanPath, err := h.protocol.SanitizePath(req.BasePath, req.EntryPath)
	if err != nil {
		return nil, err
	}
	if err := h.verifier.Verify(req.BasePath, cleanPath); err != nil {
		return nil, err
	}
	csp := req.CSP
	if csp == "" {
		csp = DefaultCSP
	}
	if err := ValidateCSP(csp); err != nil {
		return nil, err
	}

	sessionID := newSessionID()
	origin := BuildOrigin(req.ExtensionID, req.ModuleID)
	nonce := newNonce()

	session := &WebSession{
		SessionID:     sessionID,
		ContributionID: req.ContributionID,
		ExtensionID:   req.ExtensionID,
		ModuleID:      req.ModuleID,
		Generation:    req.Generation,
		SlotID:        req.SlotID,
		Origin:        origin,
		Nonce:         nonce,
		CreatedAt:     time.Now().UTC(),
		ExpiresAt:     time.Now().UTC().Add(MaxSessionDuration),
		State:         SessionStateCreated,
		Sandbox:       req.Sandbox,
		CSP:           csp,
		AllowedActions: req.AllowedActions,
		AllowedDataSources: req.AllowedDataSources,
		Theme:         req.Theme,
		Locale:        req.Locale,
		subscriptions: make(map[string]*DataSubscription),
		resourceHandles: make(map[string]*ResourceHandle),
		resizeCount:   make(map[time.Time]int),
	}

	h.mu.Lock()
	h.sessions[sessionID] = session
	h.mu.Unlock()

	entryURL := BuildEntryURL(origin, cleanPath)

	return &CreateSessionResult{
		SessionID: sessionID,
		Origin:    origin,
		Nonce:     nonce,
		CSP:       csp,
		EntryURL:  entryURL,
	}, nil
}

func (h *Host) GetSession(sessionID string) (*WebSession, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	session, exists := h.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}
	if session.IsExpired() {
		return nil, ErrSessionExpired
	}
	return session, nil
}

func (h *Host) CloseSession(sessionID, reason string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	session, exists := h.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	for _, sub := range session.subscriptions {
		sub.Active = false
	}
	for _, handle := range session.resourceHandles {
		handle.Consumed = true
	}
	session.bridgeClosed = true
	session.State = SessionStateClosed
	h.auditLog(AuditEntry{
		Timestamp: time.Now().UTC(),
		SessionID: sessionID,
		Extension: session.ExtensionID,
		Method:    "session.close",
		Success:   true,
	})
	return nil
}

func (h *Host) IsClosed() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.closed
}

func (h *Host) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for sid, session := range h.sessions {
		session.mu.Lock()
		session.State = SessionStateClosed
		session.bridgeClosed = true
		session.mu.Unlock()
		_ = sid
	}
}

func (s *WebSession) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}

func (s *WebSession) SetState(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
}

func (s *WebSession) IsActionAllowed(actionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, allowed := range s.AllowedActions {
		if allowed == actionID {
			return true
		}
	}
	return false
}

func (s *WebSession) IsDataSourceAllowed(sourceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, allowed := range s.AllowedDataSources {
		if allowed == sourceID {
			return true
		}
	}
	return false
}

func (s *WebSession) AddSubscription(sub *DataSubscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.subscriptions) >= MaxDataSubscriptions {
		return ErrTooManySubscriptions
	}
	if _, exists := s.subscriptions[sub.SubscriptionID]; exists {
		return ErrSubscriptionExists
	}
	s.subscriptions[sub.SubscriptionID] = sub
	return nil
}

func (s *WebSession) RemoveSubscription(subID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscriptions, subID)
	return nil
}

func (s *WebSession) AddResourceHandle(handle *ResourceHandle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.resourceHandles[handle.HandleID]; exists {
		return ErrResourceHandleExists
	}
	s.resourceHandles[handle.HandleID] = handle
	return nil
}

func (s *WebSession) ConsumeResourceHandle(handleID string) (*ResourceHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	handle, exists := s.resourceHandles[handleID]
	if !exists {
		return nil, ErrResourceHandleNotFound
	}
	if handle.Consumed {
		return nil, ErrResourceHandleConsumed
	}
	if time.Now().UTC().After(handle.ExpiresAt) {
		delete(s.resourceHandles, handleID)
		return nil, ErrResourceHandleExpired
	}
	return handle, nil
}

func (s *WebSession) RecordResize() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	cutoff := now.Add(-time.Minute)
	for ts := range s.resizeCount {
		if ts.Before(cutoff) {
			delete(s.resizeCount, ts)
		}
	}
	if len(s.resizeCount) >= MaxResizePerMinute {
		return ErrResizeRateLimit
	}
	s.resizeCount[now]++
	return nil
}

type ProtocolHandler struct {
	mu        sync.RWMutex
	resources map[string]*ProtocolResource
}

type ProtocolResource struct {
	ExtensionID string
	ModuleID    string
	Path        string
	MIME        string
	Hash        string
	Size        int64
	ReadOnly    bool
}

func NewProtocolHandler() *ProtocolHandler {
	return &ProtocolHandler{
		resources: make(map[string]*ProtocolResource),
	}
}

func (p *ProtocolHandler) RegisterResource(res *ProtocolResource) error {
	if res == nil || res.ExtensionID == "" || res.Path == "" {
		return ErrInvalidResource
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	key := BuildOrigin(res.ExtensionID, res.ModuleID) + res.Path
	p.resources[key] = res
	return nil
}

func (p *ProtocolHandler) Resolve(extensionID, moduleID, path string) (*ProtocolResource, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	key := BuildOrigin(extensionID, moduleID) + path
	res, exists := p.resources[key]
	if !exists {
		return nil, ErrResourceNotFound
	}
	return res, nil
}

func (p *ProtocolHandler) SanitizePath(basePath, requested string) (string, error) {
	if requested == "" {
		return "", ErrInvalidPath
	}
	cleaned := filepath.Clean(requested)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "..\\") || strings.Contains(cleaned, "../") {
		return "", ErrPathTraversal
	}
	if filepath.IsAbs(cleaned) && basePath != "" {
		rel, err := filepath.Rel(basePath, cleaned)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", ErrPathOutsideBundle
		}
		return rel, nil
	}
	return cleaned, nil
}

type BundleVerifier struct{}

func NewBundleVerifier() *BundleVerifier {
	return &BundleVerifier{}
}

func (v *BundleVerifier) Verify(basePath, entryPath string) error {
	if entryPath == "" {
		return ErrEntryMissing
	}
	if !strings.HasSuffix(entryPath, ".html") && !strings.HasSuffix(entryPath, ".htm") {
		return ErrEntryNotHTML
	}
	return nil
}

func ValidateCSP(csp string) error {
	if csp == "" {
		return ErrCSPEmpty
	}
	forbidden := []string{"'unsafe-inline'", "'unsafe-eval'", "*", "data:", "blob:"}
	lower := strings.ToLower(csp)
	for _, f := range forbidden {
		if strings.Contains(lower, "script-src") && strings.Contains(lower, f) {
			if f == "data:" || f == "blob:" {
				if strings.Contains(lower, "script-src "+f) || strings.Contains(lower, "script-src '"+f+"'") {
					return fmt.Errorf("%w: script-src cannot allow %s", ErrCSPViolation, f)
				}
			} else {
				return fmt.Errorf("%w: script-src cannot allow %s", ErrCSPViolation, f)
			}
		}
	}
	if !strings.Contains(csp, "default-src") {
		return fmt.Errorf("%w: default-src required", ErrCSPViolation)
	}
	return nil
}

func BuildOrigin(extensionID, moduleID string) string {
	return fmt.Sprintf("%s://%s/%s", ProtocolScheme, extensionID, moduleID)
}

func BuildEntryURL(origin, path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return origin + path
}

func newSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "sess_" + hex.EncodeToString(b)
}

func newNonce() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func ValidateBridgeMessage(msg *BridgeMessage, session *WebSession) error {
	if msg == nil {
		return ErrInvalidMessage
	}
	if msg.Session != session.SessionID {
		return ErrSessionMismatch
	}
	if msg.Origin != session.Origin {
		return ErrOriginMismatch
	}
	if msg.Nonce != session.Nonce {
		return ErrNonceMismatch
	}
	if len(msg.Input) > MaxMessageBytes {
		return ErrMessageTooLarge
	}
	if _, allowed := allowedMethods[BridgeMethod(msg.Method)]; !allowed {
		return ErrMethodNotAllowed
	}
	if session.bridgeClosed {
		return ErrBridgeClosed
	}
	return nil
}

var (
	ErrHostClosed            = errors.New("sandbox_webui: host closed")
	ErrInvalidRequest        = errors.New("sandbox_webui: invalid request")
	ErrInvalidSandboxType    = errors.New("sandbox_webui: invalid sandbox type")
	ErrEntryMissing          = errors.New("sandbox_webui: entry missing")
	ErrEntryNotHTML          = errors.New("sandbox_webui: entry must be html")
	ErrInvalidPath           = errors.New("sandbox_webui: invalid path")
	ErrPathTraversal         = errors.New("sandbox_webui: path traversal detected")
	ErrPathOutsideBundle     = errors.New("sandbox_webui: path outside bundle")
	ErrResourceNotFound      = errors.New("sandbox_webui: resource not found")
	ErrInvalidResource       = errors.New("sandbox_webui: invalid resource")
	ErrSessionNotFound       = errors.New("sandbox_webui: session not found")
	ErrSessionExpired        = errors.New("sandbox_webui: session expired")
	ErrTooManySubscriptions  = errors.New("sandbox_webui: too many subscriptions")
	ErrSubscriptionExists    = errors.New("sandbox_webui: subscription exists")
	ErrResourceHandleExists  = errors.New("sandbox_webui: resource handle exists")
	ErrResourceHandleNotFound = errors.New("sandbox_webui: resource handle not found")
	ErrResourceHandleConsumed = errors.New("sandbox_webui: resource handle consumed")
	ErrResourceHandleExpired = errors.New("sandbox_webui: resource handle expired")
	ErrResizeRateLimit       = errors.New("sandbox_webui: resize rate limit exceeded")
	ErrCSPEmpty              = errors.New("sandbox_webui: csp empty")
	ErrCSPViolation          = errors.New("sandbox_webui: csp violation")
	ErrInvalidMessage        = errors.New("sandbox_webui: invalid message")
	ErrSessionMismatch       = errors.New("sandbox_webui: session mismatch")
	ErrOriginMismatch        = errors.New("sandbox_webui: origin mismatch")
	ErrNonceMismatch         = errors.New("sandbox_webui: nonce mismatch")
	ErrMessageTooLarge       = errors.New("sandbox_webui: message too large")
	ErrMethodNotAllowed      = errors.New("sandbox_webui: method not allowed")
	ErrBridgeClosed          = errors.New("sandbox_webui: bridge closed")
	ErrActionNotDeclared     = errors.New("sandbox_webui: action not declared")
	ErrDataSourceNotDeclared = errors.New("sandbox_webui: data source not declared")
)

var _ = url.Parse
