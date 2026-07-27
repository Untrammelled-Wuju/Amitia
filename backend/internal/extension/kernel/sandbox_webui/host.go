package sandbox_webui

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	ProtocolScheme         = "amitia-extension"
	ResourceProtocolScheme = "amitia-resource"
	DefaultCSP             = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'none'; media-src 'self'; object-src 'none'; frame-src 'none'; child-src 'none'; worker-src 'none'; manifest-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; navigate-to 'none'"
	MaxBundleBytes         = 50 * 1024 * 1024
	MaxSessionDuration     = 24 * time.Hour
	MaxMessageBytes        = 256 * 1024
	MaxResourceHandleTTL   = 1 * time.Hour
	MaxResizePerMinute     = 30
	MaxDataSubscriptions   = 16
	MaxBridgeCallsPerSec   = 20
	MaxDataQueriesPerMin   = 60
	MaxActionsPerMin       = 30
	MaxLogPerSec           = 10
	MaxResourcePerMin      = 120
	ReadyTimeout           = 10 * time.Second
	IdleTimeout            = 30 * time.Minute
)

var AllowedMIMETypes = map[string]bool{
	"text/html":             true,
	"text/css":              true,
	"text/javascript":       true,
	"application/javascript": true,
	"application/json":      true,
	"image/png":             true,
	"image/jpeg":            true,
	"image/webp":            true,
	"image/svg+xml":         true,
	"font/woff2":            true,
	"application/wasm":      true,
}

func IsMIMEAllowed(mime string) bool {
	mime = strings.ToLower(strings.TrimSpace(mime))
	if idx := strings.Index(mime, ";"); idx > 0 {
		mime = strings.TrimSpace(mime[:idx])
	}
	return AllowedMIMETypes[mime]
}

func LookupMIME(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js", ".mjs":
		return "text/javascript"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".woff2":
		return "font/woff2"
	case ".wasm":
		return "application/wasm"
	default:
		return ""
	}
}

type SandboxType string

const (
	SandboxWebRestricted SandboxType = "web_restricted"
	SandboxWebIsolated   SandboxType = "web_isolated"
)

type SessionState string

const (
	SessionStateCreating     SessionState = "creating"
	SessionStateLoading      SessionState = "loading"
	SessionStateHandshaking  SessionState = "handshaking"
	SessionStateActive       SessionState = "active"
	SessionStateReady        SessionState = "ready"
	SessionStateSuspended    SessionState = "suspended"
	SessionStateClosing      SessionState = "closing"
	SessionStateClosed       SessionState = "closed"
	SessionStateExpired      SessionState = "expired"
	SessionStateFailed       SessionState = "failed"
	SessionStateQuarantined  SessionState = "quarantined"
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
	ReadyAt       *time.Time
	LastActiveAt  time.Time
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
	Method     string          `json:"method"`
	Version    int             `json:"version"`
	ID         string          `json:"id"`
	WindowID   string          `json:"windowId"`
	Origin     string          `json:"origin"`
	Nonce      string          `json:"nonce"`
	Generation int64           `json:"generation"`
	Input      json.RawMessage `json:"input,omitempty"`
	Output     json.RawMessage `json:"output,omitempty"`
	Deadline   time.Time       `json:"deadline,omitempty"`
	Size       int             `json:"size"`
	Session    string          `json:"session"`
}

type BridgeMethod string

const (
	MethodReady          BridgeMethod = "ui.ready"
	MethodContextGet     BridgeMethod = "ui.context.get"
	MethodActionInvoke   BridgeMethod = "ui.action.invoke"
	MethodDataRequest    BridgeMethod = "ui.data.query"
	MethodDataSubscribe  BridgeMethod = "ui.data.subscribe"
	MethodResourceRead   BridgeMethod = "ui.resource.read"
	MethodArtifactCreate BridgeMethod = "ui.artifact.create"
	MethodNavigate       BridgeMethod = "ui.navigation.request"
	MethodResize         BridgeMethod = "ui.resize.request"
	MethodDialog         BridgeMethod = "ui.dialog.request"
	MethodResourceOpen   BridgeMethod = "ui.resource.open"
	MethodLog            BridgeMethod = "ui.log"
	MethodSessionPing    BridgeMethod = "ui.session.ping"
	MethodClipboardRead  BridgeMethod = "ui.clipboard.read"
	MethodClipboardWrite BridgeMethod = "ui.clipboard.write"
	MethodNetwork        BridgeMethod = "ui.network.request"
	MethodStorage        BridgeMethod = "ui.storage"
)

var allowedMethods = map[BridgeMethod]bool{
	MethodReady: true, MethodContextGet: true, MethodActionInvoke: true,
	MethodDataRequest: true, MethodDataSubscribe: true,
	MethodNavigate: true, MethodResize: true,
	MethodDialog: true, MethodResourceOpen: true, MethodResourceRead: true,
	MethodArtifactCreate: true, MethodLog: true, MethodSessionPing: true,
	MethodClipboardRead: true, MethodClipboardWrite: true,
	MethodNetwork: true, MethodStorage: true,
}

type Host struct {
	mu          sync.RWMutex
	sessions    map[string]*WebSession
	protocol    *ProtocolHandler
	verifier    *BundleVerifier
	bridge      *Bridge
	lifecycle   *LifecycleManager
	perfMonitor *PerformanceMonitor
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
	h := &Host{
		sessions: make(map[string]*WebSession),
		protocol: NewProtocolHandler(),
		verifier: NewBundleVerifier(),
		auditLog: func(entry AuditEntry) {},
		cspReporter: func(sessionID, violation string) {},
	}
	h.lifecycle = NewLifecycleManager(h)
	h.perfMonitor = NewPerformanceMonitor()
	return h
}

func (h *Host) SetBridge(b *Bridge) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.bridge = b
}

func (h *Host) GetBridge() *Bridge {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.bridge
}

func (h *Host) Lifecycle() *LifecycleManager {
	return h.lifecycle
}

func (h *Host) PerformanceMonitor() *PerformanceMonitor {
	return h.perfMonitor
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
	ExpectedHash   string
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
	if req.Generation <= 0 {
		return nil, ErrInvalidRequest
	}
	cleanPath, err := h.protocol.SanitizePath(req.BasePath, req.EntryPath)
	if err != nil {
		return nil, err
	}
	if err := h.verifier.Verify(req.BasePath, cleanPath); err != nil {
		return nil, err
	}
	if req.ExpectedHash != "" {
		if err := h.verifier.VerifyIntegrity(req.BasePath, cleanPath, req.ExpectedHash); err != nil {
			h.cspReporter("", "resource_integrity_failed")
			return nil, err
		}
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
		LastActiveAt:  time.Now().UTC(),
		ExpiresAt:     time.Now().UTC().Add(MaxSessionDuration),
		State:         SessionStateCreating,
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
	session, exists := h.sessions[sessionID]
	if !exists {
		h.mu.Unlock()
		return ErrSessionNotFound
	}
	delete(h.sessions, sessionID)
	h.mu.Unlock()
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
		Error:     reason,
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
	if len(requested) > 1024 {
		return "", ErrPathTooLong
	}
	if strings.ContainsRune(requested, 0) {
		return "", ErrPathTraversal
	}
	lowerReq := strings.ToLower(requested)
	if strings.Contains(lowerReq, "%2e") || strings.Contains(lowerReq, "%5c") || strings.Contains(lowerReq, "%2f") || strings.Contains(lowerReq, "%00") {
		return "", ErrPathTraversal
	}
	if strings.Contains(requested, "\\") {
		return "", ErrPathTraversal
	}
	cleaned := filepath.Clean(requested)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "..\\") || strings.Contains(cleaned, "../") {
		return "", ErrPathTraversal
	}
	if filepath.IsAbs(cleaned) {
		if basePath == "" {
			return "", ErrPathOutsideBundle
		}
		absBase, err := filepath.Abs(basePath)
		if err != nil {
			return "", ErrPathOutsideBundle
		}
		absCleaned, err := filepath.Abs(cleaned)
		if err != nil {
			return "", ErrPathOutsideBundle
		}
		rel, err := filepath.Rel(absBase, absCleaned)
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
	fullPath := filepath.Join(basePath, entryPath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return ErrBundleNotFound
	}
	if info.Size() > 5*1024*1024 {
		return ErrBundleTooLarge
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return ErrBundleNotFound
	}
	lower := strings.ToLower(string(content))
	if strings.Contains(lower, "<script") {
		return ErrBundleScriptForbidden
	}
	if strings.Contains(lower, "javascript:") {
		return ErrBundleScriptForbidden
	}
	if strings.Contains(lower, "onload=") || strings.Contains(lower, "onerror=") {
		return ErrBundleScriptForbidden
	}
	if strings.Contains(lower, "<iframe") {
		return ErrBundleIframeForbidden
	}
	if strings.Contains(lower, "<object") {
		return ErrBundleObjectForbidden
	}
	return nil
}

func (v *BundleVerifier) ComputeHash(basePath, entryPath string) (string, int64, error) {
	fullPath := filepath.Join(basePath, entryPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", 0, err
	}
	h := sha256.Sum256(content)
	return "sha256-" + base64.StdEncoding.EncodeToString(h[:]), int64(len(content)), nil
}

func (v *BundleVerifier) VerifyIntegrity(basePath, entryPath, expectedHash string) error {
	actualHash, _, err := v.ComputeHash(basePath, entryPath)
	if err != nil {
		return ErrBundleNotFound
	}
	if actualHash != expectedHash {
		return ErrIntegrityMismatch
	}
	return nil
}

func ValidateCSP(csp string) error {
	if csp == "" {
		return ErrCSPEmpty
	}
	lower := strings.ToLower(csp)
	directives := parseCSPDirectives(lower)
	scriptSrc, hasScriptSrc := directives["script-src"]
	if hasScriptSrc {
		for _, token := range scriptSrc {
			switch token {
			case "'unsafe-inline'", "'unsafe-eval'", "*", "data:", "blob:", "http:", "https:":
				return fmt.Errorf("%w: script-src cannot allow %s", ErrCSPViolation, token)
			}
		}
	}
	connectSrc, hasConnectSrc := directives["connect-src"]
	if hasConnectSrc {
		for _, token := range connectSrc {
			switch token {
			case "*", "http:", "https:", "ws:", "wss:":
				return fmt.Errorf("%w: connect-src cannot allow %s", ErrCSPViolation, token)
			}
		}
	}
	defaultSrc, hasDefaultSrc := directives["default-src"]
	if !hasDefaultSrc {
		return fmt.Errorf("%w: default-src required", ErrCSPViolation)
	}
	for _, token := range defaultSrc {
		if token == "*" {
			return fmt.Errorf("%w: default-src cannot allow *", ErrCSPViolation)
		}
	}
	if strings.Contains(lower, "'unsafe-eval'") {
		return fmt.Errorf("%w: unsafe-eval globally forbidden", ErrCSPViolation)
	}
	_ = connectSrc
	return nil
}

func parseCSPDirectives(csp string) map[string][]string {
	result := make(map[string][]string)
	parts := strings.Split(csp, ";")
	for _, part := range parts {
		tokens := strings.Fields(part)
		if len(tokens) == 0 {
			continue
		}
		directive := tokens[0]
		result[directive] = tokens[1:]
	}
	return result
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
	if msg.Generation != session.Generation && msg.Generation != 0 {
		return ErrGenerationStale
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
	ErrPathTooLong           = errors.New("sandbox_webui: path too long")
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
	ErrBundleNotFound         = errors.New("sandbox_webui: bundle not found")
	ErrBundleTooLarge         = errors.New("sandbox_webui: bundle too large")
	ErrBundleScriptForbidden  = errors.New("sandbox_webui: bundle contains forbidden script")
	ErrBundleIframeForbidden  = errors.New("sandbox_webui: bundle contains forbidden iframe")
	ErrBundleObjectForbidden  = errors.New("sandbox_webui: bundle contains forbidden object")
	ErrIntegrityMismatch      = errors.New("sandbox_webui: resource integrity mismatch")
	ErrMimeNotAllowed         = errors.New("sandbox_webui: mime type not allowed")
	ErrGenerationStale        = errors.New("sandbox_webui: generation stale")
	ErrQuarantined            = errors.New("sandbox_webui: contribution quarantined")
	ErrBridgeRateLimited      = errors.New("sandbox_webui: bridge rate limited")
	ErrResourcePathForbidden  = errors.New("sandbox_webui: resource path forbidden")
)

var _ = url.Parse
