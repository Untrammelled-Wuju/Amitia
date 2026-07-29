package jsonrpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	DefaultHandshakeTimeout = 10 * time.Second
	DefaultSessionLifetime  = 24 * time.Hour
	NonceBytes              = 32
)

type RuntimeType string

const (
	RuntimeTypeMain     RuntimeType = "main"
	RuntimeTypeTask     RuntimeType = "task"
	RuntimeTypeService  RuntimeType = "service"
	RuntimeTypeWASM     RuntimeType = "wasm"
	RuntimeTypeUI       RuntimeType = "ui"
	RuntimeTypeLegacyGo RuntimeType = "legacy_go"
)

type HelloMessage struct {
	ProtocolVersion string            `json:"protocol_version"`
	RuntimeType     RuntimeType       `json:"runtime_type"`
	InstanceID      string            `json:"instance_id"`
	Generation      int64             `json:"generation"`
	DefinitionHash  string            `json:"definition_hash"`
	Nonce           string            `json:"nonce"`
	Features        map[string]bool   `json:"features"`
	SDKVersion      string            `json:"sdk_version,omitempty"`
	RuntimeVersion  string            `json:"runtime_version,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type WelcomeMessage struct {
	ProtocolVersion string          `json:"protocol_version"`
	SessionID       string          `json:"session_id"`
	SessionToken    string          `json:"session_token"`
	HostAPIVersion  string          `json:"host_api_version"`
	Features        map[string]bool `json:"features"`
	Limits          SessionLimits   `json:"limits"`
	ClockOffset     int64           `json:"clock_offset_ms"`
	ExpiresAt       time.Time       `json:"expires_at"`
	InstanceID      string          `json:"instance_id"`
	Generation      int64           `json:"generation"`
}

type SessionLimits struct {
	MaxConcurrent       int           `json:"max_concurrent"`
	MaxStreamBytes      int           `json:"max_stream_bytes"`
	MaxStreams          int           `json:"max_streams"`
	MaxMessageBytes     int           `json:"max_message_bytes"`
	MaxFrameBytes       int           `json:"max_frame_bytes"`
	CallTimeout         time.Duration `json:"call_timeout"`
	HeartbeatInterval   time.Duration `json:"heartbeat_interval"`
	LogRatePerSecond    int           `json:"log_rate_per_second"`
	NotifyRatePerSecond int           `json:"notify_rate_per_second"`
}

type Session struct {
	ID             string
	Token          string
	InstanceID     string
	Generation     int64
	RuntimeType    RuntimeType
	DefinitionHash string
	Features       map[string]bool
	Limits         SessionLimits
	CreatedAt      time.Time
	ExpiresAt      time.Time
	LastActivity   time.Time
	mu             sync.RWMutex
	closed         bool
}

func (s *Session) IsExpired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Now().UTC().After(s.ExpiresAt)
}

func (s *Session) Touch() {
	s.mu.Lock()
	s.LastActivity = time.Now().UTC()
	s.mu.Unlock()
}

func (s *Session) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

func (s *Session) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

func (s *Session) HasFeature(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.Features[name]
	return ok && v
}

type HandshakeConfig struct {
	ExpectedProtocolVersion string
	HostAPIVersion          string
	NonceStore              NonceStore
	InstanceValidator       InstanceValidator
	Limits                  SessionLimits
	Lifetime                time.Duration
	Timeout                 time.Duration
}

type NonceStore interface {
	Validate(instanceID, nonce string) error
	Consume(instanceID, nonce string) error
}

type InstanceValidator interface {
	Validate(ctx context.Context, instanceID string, generation int64, definitionHash string) error
}

type DefaultNonceStore struct {
	mu     sync.Mutex
	issued map[string]string
}

func NewDefaultNonceStore() *DefaultNonceStore {
	return &DefaultNonceStore{issued: make(map[string]string)}
}

func (s *DefaultNonceStore) Issue(instanceID string) (string, error) {
	buf := make([]byte, NonceBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("jsonrpc: generate nonce: %w", err)
	}
	nonce := hex.EncodeToString(buf)
	s.mu.Lock()
	s.issued[instanceID] = nonce
	s.mu.Unlock()
	return nonce, nil
}

func (s *DefaultNonceStore) Validate(instanceID, nonce string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	expected, ok := s.issued[instanceID]
	if !ok {
		return errors.New("jsonrpc: nonce not issued for instance")
	}
	if expected != nonce {
		return errors.New("jsonrpc: invalid nonce")
	}
	return nil
}

func (s *DefaultNonceStore) Consume(instanceID, nonce string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	expected, ok := s.issued[instanceID]
	if !ok {
		return errors.New("jsonrpc: nonce not issued for instance")
	}
	if expected != nonce {
		return errors.New("jsonrpc: invalid nonce")
	}
	delete(s.issued, instanceID)
	return nil
}

type Handshaker struct {
	cfg HandshakeConfig
}

func NewHandshaker(cfg HandshakeConfig) *Handshaker {
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultHandshakeTimeout
	}
	if cfg.Lifetime <= 0 {
		cfg.Lifetime = DefaultSessionLifetime
	}
	if cfg.ExpectedProtocolVersion == "" {
		cfg.ExpectedProtocolVersion = RPCVersion
	}
	return &Handshaker{cfg: cfg}
}

func (h *Handshaker) HostHandshake(ctx context.Context, transport *ReadWriteCloser, expectedHello HelloMessage) (*Session, *WelcomeMessage, error) {
	helloEnv, err := transport.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("jsonrpc: read hello: %w", err)
	}
	if helloEnv.Kind != KindNotification || helloEnv.Notification.Method != "runtime.hello" {
		return nil, nil, HandshakeFailedError("expected runtime.hello notification")
	}
	var hello HelloMessage
	if err := json.Unmarshal(helloEnv.Notification.Params, &hello); err != nil {
		return nil, nil, fmt.Errorf("jsonrpc: decode hello: %w", err)
	}
	if err := h.validateHello(hello); err != nil {
		return nil, nil, err
	}
	if h.cfg.InstanceValidator != nil {
		if err := h.cfg.InstanceValidator.Validate(ctx, hello.InstanceID, hello.Generation, hello.DefinitionHash); err != nil {
			return nil, nil, fmt.Errorf("jsonrpc: instance validation: %w", err)
		}
	}
	if h.cfg.NonceStore != nil {
		if err := h.cfg.NonceStore.Consume(hello.InstanceID, hello.Nonce); err != nil {
			return nil, nil, HandshakeFailedError(err.Error())
		}
	}
	sessionID := newSessionID()
	token := newSessionToken()
	now := time.Now().UTC()
	features := negotiateFeatures(hello.Features, defaultHostFeatures())
	welcome := &WelcomeMessage{
		ProtocolVersion: h.cfg.ExpectedProtocolVersion,
		SessionID:       sessionID,
		SessionToken:    token,
		HostAPIVersion:  h.cfg.HostAPIVersion,
		Features:        features,
		Limits:          h.cfg.Limits,
		ClockOffset:     0,
		ExpiresAt:       now.Add(h.cfg.Lifetime),
		InstanceID:      hello.InstanceID,
		Generation:      hello.Generation,
	}
	session := &Session{
		ID:             sessionID,
		Token:          token,
		InstanceID:     hello.InstanceID,
		Generation:     hello.Generation,
		RuntimeType:    hello.RuntimeType,
		DefinitionHash: hello.DefinitionHash,
		Features:       features,
		Limits:         h.cfg.Limits,
		CreatedAt:      now,
		ExpiresAt:      welcome.ExpiresAt,
		LastActivity:   now,
	}
	transport.SetMaxFrameBytes(h.cfg.Limits.MaxFrameBytes)
	welcomeNotif, err := EncodeNotification("host.welcome", mustMarshal(welcome))
	if err != nil {
		return nil, nil, fmt.Errorf("jsonrpc: encode welcome: %w", err)
	}
	if err := transport.Write(welcomeNotif); err != nil {
		return nil, nil, fmt.Errorf("jsonrpc: send welcome: %w", err)
	}
	readyEnv, err := transport.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("jsonrpc: read ready: %w", err)
	}
	if readyEnv.Kind != KindNotification || readyEnv.Notification.Method != "runtime.ready" {
		return nil, nil, HandshakeFailedError("expected runtime.ready notification")
	}
	return session, welcome, nil
}

func (h *Handshaker) RuntimeHandshake(ctx context.Context, transport *ReadWriteCloser, hello HelloMessage) (*Session, *WelcomeMessage, error) {
	if err := h.validateHello(hello); err != nil {
		return nil, nil, err
	}
	helloNotif, err := EncodeNotification("runtime.hello", mustMarshal(hello))
	if err != nil {
		return nil, nil, fmt.Errorf("jsonrpc: encode hello: %w", err)
	}
	if err := transport.Write(helloNotif); err != nil {
		return nil, nil, fmt.Errorf("jsonrpc: send hello: %w", err)
	}
	welcomeEnv, err := transport.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("jsonrpc: read welcome: %w", err)
	}
	if welcomeEnv.Kind != KindNotification || welcomeEnv.Notification.Method != "host.welcome" {
		return nil, nil, HandshakeFailedError("expected host.welcome notification")
	}
	var welcome WelcomeMessage
	if err := json.Unmarshal(welcomeEnv.Notification.Params, &welcome); err != nil {
		return nil, nil, fmt.Errorf("jsonrpc: decode welcome: %w", err)
	}
	if welcome.ProtocolVersion != h.cfg.ExpectedProtocolVersion {
		return nil, nil, HandshakeFailedError(fmt.Sprintf("version mismatch: %s != %s", welcome.ProtocolVersion, h.cfg.ExpectedProtocolVersion))
	}
	session := &Session{
		ID:             welcome.SessionID,
		Token:          welcome.SessionToken,
		InstanceID:     hello.InstanceID,
		Generation:     hello.Generation,
		RuntimeType:    hello.RuntimeType,
		DefinitionHash: hello.DefinitionHash,
		Features:       welcome.Features,
		Limits:         welcome.Limits,
		CreatedAt:      time.Now().UTC(),
		ExpiresAt:      welcome.ExpiresAt,
		LastActivity:   time.Now().UTC(),
	}
	transport.SetMaxFrameBytes(welcome.Limits.MaxFrameBytes)
	readyNotif, err := EncodeNotification("runtime.ready", mustMarshal(map[string]any{"ready": true}))
	if err != nil {
		return nil, nil, fmt.Errorf("jsonrpc: encode ready: %w", err)
	}
	if err := transport.Write(readyNotif); err != nil {
		return nil, nil, fmt.Errorf("jsonrpc: send ready: %w", err)
	}
	return session, &welcome, nil
}

func (h *Handshaker) validateHello(hello HelloMessage) error {
	if hello.ProtocolVersion == "" {
		return HandshakeFailedError("missing protocol version")
	}
	if hello.InstanceID == "" {
		return HandshakeFailedError("missing instance id")
	}
	if hello.Nonce == "" {
		return HandshakeFailedError("missing nonce")
	}
	if hello.RuntimeType == "" {
		return HandshakeFailedError("missing runtime type")
	}
	if hello.ProtocolVersion != h.cfg.ExpectedProtocolVersion {
		return NewError(ErrCodeVersionMismatch, fmt.Sprintf("version mismatch: %s != %s", hello.ProtocolVersion, h.cfg.ExpectedProtocolVersion), false, CategoryProtocol)
	}
	return nil
}

func defaultHostFeatures() map[string]bool {
	return map[string]bool{
		"streaming":    true,
		"cancellation": true,
		"backpressure": true,
		"diagnostics":  true,
		"watchdog":     true,
		"hot_reload":   false,
		"event_inbox":  true,
		"checkpoint":   true,
	}
}

func negotiateFeatures(runtime, host map[string]bool) map[string]bool {
	out := make(map[string]bool)
	for k, hostSupports := range host {
		if !hostSupports {
			continue
		}
		if runtimeSupports, ok := runtime[k]; ok && runtimeSupports {
			out[k] = true
		}
	}
	return out
}

func newSessionID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return "sess_" + hex.EncodeToString(buf)
}

func newSessionToken() string {
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	return "tok_" + hex.EncodeToString(buf)
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}
