package trusted_service

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

const (
	frameHeaderSize = 8
	maxMessageBytes = 16 * 1024 * 1024
	protocolVersion = "amitia_jsonrpc_v1"
)

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type HelloMessage struct {
	Nonce          string `json:"nonce"`
	InstanceID     string `json:"instance_id"`
	ProtocolVersion string `json:"protocol_version"`
	ExtensionID    string `json:"extension_id,omitempty"`
	ModuleID       string `json:"module_id,omitempty"`
}

type WelcomeMessage struct {
	SessionToken string        `json:"session_token"`
	Limits       map[string]any `json:"limits,omitempty"`
	ExpiresAt    time.Time     `json:"expires_at,omitempty"`
}

type InitializeRequest struct {
	Capabilities []string       `json:"capabilities"`
	Version      string         `json:"version"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type InvokeRequest struct {
	Operation string          `json:"operation"`
	Input     json.RawMessage `json:"input,omitempty"`
}

type InvokeResult struct {
	Output json.RawMessage `json:"output,omitempty"`
}

type HealthResult struct {
	Status  string          `json:"status"`
	Details map[string]any  `json:"details,omitempty"`
}

type RPCSession struct {
	mu            sync.Mutex
	stdin         io.WriteCloser
	stdout        io.Reader
	stderr        io.Reader
	instanceID    string
	extensionID   string
	moduleID      string
	sessionToken  string
	nonce         string
	ready         bool
	nextID        int64
	pending       map[int64]chan *rpcMessage
	stopCh        chan struct{}
	done          chan struct{}
	onLog         func(level, msg string)
	onNotification func(method string, params json.RawMessage)
}

type RPCSessionConfig struct {
	InstanceID    string
	ExtensionID   string
	ModuleID      string
	Nonce         string
	OnLog         func(level, msg string)
	OnNotification func(method string, params json.RawMessage)
}

func NewRPCSession(stdin io.WriteCloser, stdout, stderr io.Reader, config RPCSessionConfig) *RPCSession {
	return &RPCSession{
		stdin:          stdin,
		stdout:         stdout,
		stderr:         stderr,
		instanceID:     config.InstanceID,
		extensionID:    config.ExtensionID,
		moduleID:       config.ModuleID,
		nonce:          config.Nonce,
		pending:        make(map[int64]chan *rpcMessage),
		stopCh:         make(chan struct{}),
		done:           make(chan struct{}),
		onLog:          config.OnLog,
		onNotification: config.OnNotification,
	}
}

func (s *RPCSession) Start() {
	go s.readLoop()
	go s.readStderr()
}

func (s *RPCSession) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

func (s *RPCSession) Done() <-chan struct{} {
	return s.done
}

func (s *RPCSession) readLoop() {
	defer close(s.done)
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}
		msg, err := readFrame(s.stdout)
		if err != nil {
			if err == io.EOF {
				return
			}
			if s.onLog != nil {
				s.onLog("error", fmt.Sprintf("rpc read error: %v", err))
			}
			return
		}
		var rpcMsg rpcMessage
		if err := json.Unmarshal(msg, &rpcMsg); err != nil {
			if s.onLog != nil {
				s.onLog("warn", fmt.Sprintf("rpc parse error: %v", err))
			}
			continue
		}
		s.handleMessage(&rpcMsg)
	}
}

func (s *RPCSession) readStderr() {
	buf := make([]byte, 4096)
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}
		n, err := s.stderr.Read(buf)
		if err != nil {
			return
		}
		if n > 0 && s.onLog != nil {
			s.onLog("warn", string(buf[:n]))
		}
	}
}

func (s *RPCSession) handleMessage(msg *rpcMessage) {
	if msg.ID != nil && (msg.Result != nil || msg.Error != nil) {
		s.mu.Lock()
		ch, exists := s.pending[*msg.ID]
		if exists {
			delete(s.pending, *msg.ID)
		}
		s.mu.Unlock()
		if exists {
			select {
			case ch <- msg:
			default:
			}
		}
		return
	}
	if msg.Method != "" && msg.ID == nil {
		if s.onNotification != nil {
			s.onNotification(msg.Method, msg.Params)
		}
	}
}

func (s *RPCSession) sendRequest(method string, params any, timeout time.Duration) (*rpcMessage, error) {
	id := s.nextRequestID()
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("rpc: marshal params: %w", err)
	}
	msg := rpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  paramBytes,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("rpc: marshal message: %w", err)
	}
	ch := make(chan *rpcMessage, 1)
	s.mu.Lock()
	s.pending[id] = ch
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}()
	if err := s.writeFrame(data); err != nil {
		return nil, fmt.Errorf("rpc: write frame: %w", err)
	}
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc: %s: %s (code %d)", method, resp.Error.Message, resp.Error.Code)
		}
		return resp, nil
	case <-s.stopCh:
		return nil, errors.New("rpc: session stopped")
	case <-time.After(timeout):
		return nil, fmt.Errorf("rpc: %s timeout after %s", method, timeout)
	}
}

func (s *RPCSession) sendNotification(method string, params any) error {
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("rpc: marshal params: %w", err)
	}
	msg := rpcMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramBytes,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("rpc: marshal message: %w", err)
	}
	return s.writeFrame(data)
}

func (s *RPCSession) writeFrame(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(data) > maxMessageBytes {
		return fmt.Errorf("rpc: message too large: %d bytes", len(data))
	}
	var header [frameHeaderSize]byte
	binary.BigEndian.PutUint64(header[:], uint64(len(data)))
	if _, err := s.stdin.Write(header[:]); err != nil {
		return err
	}
	if _, err := s.stdin.Write(data); err != nil {
		return err
	}
	return nil
}

func (s *RPCSession) nextRequestID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	return s.nextID
}

func (s *RPCSession) IsReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ready
}

func (s *RPCSession) SetReady(ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = ready
}

func (s *RPCSession) SessionToken() string {
	return s.sessionToken
}

func (s *RPCSession) SetSessionToken(token string) {
	s.sessionToken = token
}

func (s *RPCSession) WaitForHello(timeout time.Duration) (*HelloMessage, error) {
	helloCh := make(chan *HelloMessage, 1)
	origNotification := s.onNotification
	s.onNotification = func(method string, params json.RawMessage) {
		if method == "runtime.hello" {
			var hello HelloMessage
			if err := json.Unmarshal(params, &hello); err == nil {
				select {
				case helloCh <- &hello:
				default:
				}
				return
			}
		}
		if origNotification != nil {
			origNotification(method, params)
		}
	}
	defer func() { s.onNotification = origNotification }()

	select {
	case hello := <-helloCh:
		if hello.Nonce != s.nonce {
			return nil, fmt.Errorf("rpc: nonce mismatch: expected %s got %s", s.nonce, hello.Nonce)
		}
		if hello.InstanceID != s.instanceID {
			return nil, fmt.Errorf("rpc: instance_id mismatch: expected %s got %s", s.instanceID, hello.InstanceID)
		}
		if hello.ProtocolVersion != protocolVersion {
			return nil, fmt.Errorf("rpc: protocol version mismatch: expected %s got %s", protocolVersion, hello.ProtocolVersion)
		}
		return hello, nil
	case <-s.stopCh:
		return nil, errors.New("rpc: session stopped while waiting for hello")
	case <-time.After(timeout):
		return nil, fmt.Errorf("rpc: hello timeout after %s", timeout)
	}
}

func (s *RPCSession) SendWelcome(token string, limits map[string]any, expiresAt time.Time) error {
	return s.sendNotification("host.welcome", WelcomeMessage{
		SessionToken: token,
		Limits:       limits,
		ExpiresAt:    expiresAt,
	})
}

func (s *RPCSession) WaitForInitialize(timeout time.Duration) (*InitializeRequest, error) {
	initCh := make(chan *InitializeRequest, 1)
	origNotification := s.onNotification
	s.onNotification = func(method string, params json.RawMessage) {
		if method == "service.initialize" {
			var initReq InitializeRequest
			if err := json.Unmarshal(params, &initReq); err == nil {
				select {
				case initCh <- &initReq:
				default:
				}
				return
			}
		}
		if origNotification != nil {
			origNotification(method, params)
		}
	}
	defer func() { s.onNotification = origNotification }()

	select {
	case initReq := <-initCh:
		return initReq, nil
	case <-s.stopCh:
		return nil, errors.New("rpc: session stopped while waiting for initialize")
	case <-time.After(timeout):
		return nil, fmt.Errorf("rpc: initialize timeout after %s", timeout)
	}
}

func (s *RPCSession) RespondInitialize(success bool, detail string) error {
	params := map[string]any{
		"accepted": success,
		"detail":   detail,
	}
	return s.sendNotification("service.initialize.response", params)
}

func (s *RPCSession) WaitForReady(timeout time.Duration) error {
	readyCh := make(chan struct{}, 1)
	origNotification := s.onNotification
	s.onNotification = func(method string, params json.RawMessage) {
		if method == "runtime.ready" || method == "service.ready" {
			select {
			case readyCh <- struct{}{}:
			default:
			}
			return
		}
		if origNotification != nil {
			origNotification(method, params)
		}
	}
	defer func() { s.onNotification = origNotification }()

	select {
	case <-readyCh:
		s.SetReady(true)
		return nil
	case <-s.stopCh:
		return errors.New("rpc: session stopped while waiting for ready")
	case <-time.After(timeout):
		return fmt.Errorf("rpc: ready timeout after %s", timeout)
	}
}

func (s *RPCSession) Health(timeout time.Duration) (*HealthResult, error) {
	resp, err := s.sendRequest("service.health", map[string]any{}, timeout)
	if err != nil {
		return nil, err
	}
	var result HealthResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("rpc: parse health result: %w", err)
	}
	return &result, nil
}

func (s *RPCSession) Invoke(operation string, input json.RawMessage, timeout time.Duration) (*InvokeResult, error) {
	resp, err := s.sendRequest("service.invoke", InvokeRequest{
		Operation: operation,
		Input:     input,
	}, timeout)
	if err != nil {
		return nil, err
	}
	var result InvokeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("rpc: parse invoke result: %w", err)
	}
	return &result, nil
}

func (s *RPCSession) Shutdown(timeout time.Duration) error {
	_, err := s.sendRequest("service.shutdown", map[string]any{}, timeout)
	return err
}

func (s *RPCSession) Close() {
	s.Stop()
	if s.stdin != nil {
		s.stdin.Close()
	}
}

func readFrame(r io.Reader) ([]byte, error) {
	var header [frameHeaderSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint64(header[:])
	if size > maxMessageBytes {
		return nil, fmt.Errorf("rpc: message too large: %d bytes", size)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
