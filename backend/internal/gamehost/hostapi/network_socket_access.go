package hostapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

const (
	defaultNetworkSocketTimeout        = 10 * time.Second
	maxNetworkSocketTimeout            = 120 * time.Second
	defaultNetworkSocketRead           = 64 << 10
	maxNetworkSocketPayload            = 1 << 20
	hardNetworkSocketConnections       = 64
	hardNetworkSocketGlobalConnections = 1024
)

type NetworkSocketOpenInput struct {
	Target    string `json:"target"`
	Port      int    `json:"port"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
}

type NetworkSocketOpenOutput struct {
	HandleID      string `json:"handleId"`
	Transport     string `json:"transport"`
	LocalAddress  string `json:"localAddress,omitempty"`
	RemoteAddress string `json:"remoteAddress,omitempty"`
}

type NetworkSocketReadInput struct {
	HandleID  string `json:"handleId"`
	MaxBytes  int    `json:"maxBytes,omitempty"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
}

type NetworkSocketReadOutput struct {
	DataBase64 string `json:"dataBase64"`
	BytesRead  int    `json:"bytesRead"`
	EOF        bool   `json:"eof,omitempty"`
}

type NetworkSocketWriteInput struct {
	HandleID   string `json:"handleId"`
	DataBase64 string `json:"dataBase64"`
	TimeoutMs  int    `json:"timeoutMs,omitempty"`
}

type NetworkSocketWriteOutput struct {
	BytesWritten int `json:"bytesWritten"`
}

type NetworkSocketCloseInput struct {
	HandleID string `json:"handleId"`
}

type NetworkSocketCloseOutput struct {
	Closed bool `json:"closed"`
}

type NetworkWebSocketOpenInput struct {
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers,omitempty"`
	Subprotocols []string          `json:"subprotocols,omitempty"`
	TimeoutMs    int               `json:"timeoutMs,omitempty"`
}

type NetworkWebSocketOpenOutput struct {
	HandleID      string `json:"handleId"`
	Subprotocol   string `json:"subprotocol,omitempty"`
	RemoteAddress string `json:"remoteAddress,omitempty"`
}

type NetworkWebSocketSendInput struct {
	HandleID    string `json:"handleId"`
	MessageType string `json:"messageType,omitempty"`
	DataBase64  string `json:"dataBase64"`
	TimeoutMs   int    `json:"timeoutMs,omitempty"`
}

type NetworkWebSocketSendOutput struct {
	BytesWritten int `json:"bytesWritten"`
}

type NetworkWebSocketReceiveInput struct {
	HandleID  string `json:"handleId"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
}

type NetworkWebSocketReceiveOutput struct {
	MessageType string `json:"messageType"`
	DataBase64  string `json:"dataBase64"`
	BytesRead   int    `json:"bytesRead"`
}

type networkConnectionKind string

const (
	networkConnectionTCP       networkConnectionKind = "tcp"
	networkConnectionUDP       networkConnectionKind = "udp"
	networkConnectionWebSocket networkConnectionKind = "websocket"
)

type networkConnection struct {
	readMu    sync.Mutex
	writeMu   sync.Mutex
	closeOnce sync.Once
	closeErr  error
	kind      networkConnectionKind
	owner     runtime_supervisor.RuntimeIdentity
	conn      net.Conn
	ws        *websocket.Conn
}

// close is deliberately independent from readMu/writeMu. net.Conn and
// gorilla/websocket both permit Close to race with active I/O, which is
// required so runtime stop/emergency stop can interrupt a blocked read
// immediately instead of waiting for its per-call deadline.
func (c *networkConnection) close() error {
	c.closeOnce.Do(func() {
		if c.ws != nil {
			c.closeErr = c.ws.Close()
			return
		}
		if c.conn != nil {
			c.closeErr = c.conn.Close()
		}
	})
	return c.closeErr
}

type networkConnectionManager struct {
	mu       sync.Mutex
	handles  map[string]*networkConnection
	stopOnce sync.Once
	closed   bool
}

func newNetworkConnectionManager() *networkConnectionManager {
	return &networkConnectionManager{handles: make(map[string]*networkConnection)}
}

func sameNetworkOwner(a, b runtime_supervisor.RuntimeIdentity) bool {
	return a.InstanceID == b.InstanceID &&
		a.ExtensionID == b.ExtensionID &&
		a.ModuleID == b.ModuleID &&
		a.Generation == b.Generation &&
		a.SessionNonce == b.SessionNonce
}

func (m *networkConnectionManager) reapLocked(owner runtime_supervisor.RuntimeIdentity) []*networkConnection {
	var stale []*networkConnection
	for id, handle := range m.handles {
		sameRuntimeSlot := handle.owner.InstanceID == owner.InstanceID &&
			handle.owner.ExtensionID == owner.ExtensionID &&
			handle.owner.ModuleID == owner.ModuleID
		sessionStale := sameRuntimeSlot &&
			(handle.owner.Generation != owner.Generation || handle.owner.SessionNonce != owner.SessionNonce)
		if sessionStale {
			delete(m.handles, id)
			stale = append(stale, handle)
		}
	}
	return stale
}

func (m *networkConnectionManager) closeMatching(match func(runtime_supervisor.RuntimeIdentity) bool) int {
	m.mu.Lock()
	closed := make([]*networkConnection, 0)
	for id, handle := range m.handles {
		if match(handle.owner) {
			delete(m.handles, id)
			closed = append(closed, handle)
		}
	}
	m.mu.Unlock()
	closeNetworkHandles(closed)
	return len(closed)
}

func (m *networkConnectionManager) closeRuntimeModuleGeneration(runtimeID, moduleID string, generation int64) int {
	return m.closeMatching(func(owner runtime_supervisor.RuntimeIdentity) bool {
		if owner.InstanceID != runtimeID {
			return false
		}
		if generation > 0 && owner.Generation != generation {
			return false
		}
		return moduleID == "" || string(owner.ModuleID) == moduleID
	})
}

func (m *networkConnectionManager) shutdown() {
	m.stopOnce.Do(func() {
		m.mu.Lock()
		m.closed = true
		handles := make([]*networkConnection, 0, len(m.handles))
		for id, handle := range m.handles {
			delete(m.handles, id)
			handles = append(handles, handle)
		}
		m.mu.Unlock()
		closeNetworkHandles(handles)
	})
}

func closeNetworkHandles(handles []*networkConnection) {
	for _, handle := range handles {
		_ = handle.close()
	}
}

func (m *networkConnectionManager) add(owner runtime_supervisor.RuntimeIdentity, kind networkConnectionKind, conn net.Conn, ws *websocket.Conn, maxConnections int) (string, error) {
	if maxConnections <= 0 {
		maxConnections = 16
	}
	if maxConnections > hardNetworkSocketConnections {
		maxConnections = hardNetworkSocketConnections
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return "", fmt.Errorf("host-mediated network manager is shut down")
	}
	stale := m.reapLocked(owner)
	if len(m.handles) >= hardNetworkSocketGlobalConnections {
		m.mu.Unlock()
		closeNetworkHandles(stale)
		return "", fmt.Errorf("global host-mediated network connection limit reached")
	}
	count := 0
	for _, handle := range m.handles {
		if sameNetworkOwner(handle.owner, owner) {
			count++
		}
	}
	if count >= maxConnections {
		m.mu.Unlock()
		closeNetworkHandles(stale)
		return "", fmt.Errorf("maximum host-mediated network connections reached")
	}
	var id string
	for {
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			m.mu.Unlock()
			closeNetworkHandles(stale)
			return "", fmt.Errorf("generate network handle: %w", err)
		}
		id = "net_" + hex.EncodeToString(raw[:])
		if _, exists := m.handles[id]; !exists {
			break
		}
	}
	m.handles[id] = &networkConnection{kind: kind, owner: owner, conn: conn, ws: ws}
	m.mu.Unlock()
	closeNetworkHandles(stale)
	return id, nil
}

func (m *networkConnectionManager) get(owner runtime_supervisor.RuntimeIdentity, id string, kind networkConnectionKind) (*networkConnection, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 96 {
		return nil, fmt.Errorf("invalid network handle")
	}
	m.mu.Lock()
	stale := m.reapLocked(owner)
	handle := m.handles[id]
	if handle == nil || !sameNetworkOwner(handle.owner, owner) || handle.kind != kind {
		m.mu.Unlock()
		closeNetworkHandles(stale)
		return nil, fmt.Errorf("network handle is unavailable")
	}
	m.mu.Unlock()
	closeNetworkHandles(stale)
	return handle, nil
}

func (m *networkConnectionManager) remove(owner runtime_supervisor.RuntimeIdentity, id string, kind networkConnectionKind) (*networkConnection, error) {
	id = strings.TrimSpace(id)
	m.mu.Lock()
	stale := m.reapLocked(owner)
	handle := m.handles[id]
	if handle == nil || !sameNetworkOwner(handle.owner, owner) || handle.kind != kind {
		m.mu.Unlock()
		closeNetworkHandles(stale)
		return nil, fmt.Errorf("network handle is unavailable")
	}
	delete(m.handles, id)
	m.mu.Unlock()
	closeNetworkHandles(stale)
	return handle, nil
}

func networkTimeout(timeoutMs int) (time.Duration, error) {
	if timeoutMs == 0 {
		return defaultNetworkSocketTimeout, nil
	}
	if timeoutMs < 100 || time.Duration(timeoutMs)*time.Millisecond > maxNetworkSocketTimeout {
		return 0, fmt.Errorf("timeoutMs must be between 100 and 120000")
	}
	return time.Duration(timeoutMs) * time.Millisecond, nil
}

func networkReadLimit(maxBytes int) (int, error) {
	if maxBytes == 0 {
		return defaultNetworkSocketRead, nil
	}
	if maxBytes < 1 || maxBytes > maxNetworkSocketPayload {
		return 0, fmt.Errorf("maxBytes must be between 1 and %d", maxNetworkSocketPayload)
	}
	return maxBytes, nil
}

func decodeNetworkPayload(encoded string) ([]byte, error) {
	if encoded == "" {
		return []byte{}, nil
	}
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("dataBase64 is invalid")
	}
	if len(payload) > maxNetworkSocketPayload {
		return nil, fmt.Errorf("network payload exceeds %d bytes", maxNetworkSocketPayload)
	}
	return payload, nil
}

func openApprovedSocket(ctx context.Context, client *restrictedHTTPClient, transport string, in NetworkSocketOpenInput) (net.Conn, error) {
	timeout, err := networkTimeout(in.TimeoutMs)
	if err != nil {
		return nil, err
	}
	network := transport
	if transport == "tcp" {
		network = "tcp"
	} else if transport == "udp" {
		network = "udp"
	} else {
		return nil, fmt.Errorf("unsupported socket transport %q", transport)
	}
	return client.dialApproved(ctx, transport, network, in.Target, in.Port, timeout)
}

func applyNetworkDeadline(ctx context.Context, timeout time.Duration, setDeadline func(time.Time) error) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := setDeadline(deadline); err != nil {
		return nil, err
	}
	stopCancel := context.AfterFunc(ctx, func() {
		_ = setDeadline(time.Now())
	})
	return func() { _ = stopCancel() }, nil
}

func readApprovedSocket(ctx context.Context, handle *networkConnection, in NetworkSocketReadInput) (NetworkSocketReadOutput, error) {
	limit, err := networkReadLimit(in.MaxBytes)
	if err != nil {
		return NetworkSocketReadOutput{}, err
	}
	timeout, err := networkTimeout(in.TimeoutMs)
	if err != nil {
		return NetworkSocketReadOutput{}, err
	}
	buffer := make([]byte, limit)
	handle.readMu.Lock()
	defer handle.readMu.Unlock()
	if handle.conn == nil {
		return NetworkSocketReadOutput{}, fmt.Errorf("network handle is closed")
	}
	stopCancel, err := applyNetworkDeadline(ctx, timeout, handle.conn.SetReadDeadline)
	if err != nil {
		return NetworkSocketReadOutput{}, err
	}
	defer stopCancel()
	n, readErr := handle.conn.Read(buffer)
	if readErr != nil && readErr != io.EOF {
		if ctx.Err() != nil {
			return NetworkSocketReadOutput{}, ctx.Err()
		}
		return NetworkSocketReadOutput{}, readErr
	}
	return NetworkSocketReadOutput{
		DataBase64: base64.StdEncoding.EncodeToString(buffer[:n]),
		BytesRead:  n,
		EOF:        readErr == io.EOF,
	}, nil
}

func writeApprovedSocket(ctx context.Context, handle *networkConnection, in NetworkSocketWriteInput) (NetworkSocketWriteOutput, error) {
	payload, err := decodeNetworkPayload(in.DataBase64)
	if err != nil {
		return NetworkSocketWriteOutput{}, err
	}
	timeout, err := networkTimeout(in.TimeoutMs)
	if err != nil {
		return NetworkSocketWriteOutput{}, err
	}
	handle.writeMu.Lock()
	defer handle.writeMu.Unlock()
	if handle.conn == nil {
		return NetworkSocketWriteOutput{}, fmt.Errorf("network handle is closed")
	}
	stopCancel, err := applyNetworkDeadline(ctx, timeout, handle.conn.SetWriteDeadline)
	if err != nil {
		return NetworkSocketWriteOutput{}, err
	}
	defer stopCancel()
	n, err := handle.conn.Write(payload)
	if err != nil {
		if ctx.Err() != nil {
			return NetworkSocketWriteOutput{}, ctx.Err()
		}
		return NetworkSocketWriteOutput{}, err
	}
	return NetworkSocketWriteOutput{BytesWritten: n}, nil
}

func validateWebSocketURL(ctx context.Context, client *restrictedHTTPClient, rawURL string) (*url.URL, int, error) {
	if len(strings.TrimSpace(rawURL)) == 0 || len(rawURL) > 8192 {
		return nil, 0, fmt.Errorf("websocket URL must contain 1..8192 bytes")
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil || parsed.Host == "" {
		return nil, 0, fmt.Errorf("invalid websocket URL")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return nil, 0, fmt.Errorf("websocket URL userinfo and fragments are not allowed")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "ws" && scheme != "wss" {
		return nil, 0, fmt.Errorf("only ws and wss are allowed")
	}
	if !client.transportAllowed("websocket") {
		return nil, 0, fmt.Errorf("network transport %q is not allowed", "websocket")
	}
	port := parsed.Port()
	if port == "" {
		if scheme == "wss" {
			port = "443"
		} else {
			port = "80"
		}
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid websocket port")
	}
	if _, err := client.resolveSocketAddresses(ctx, "websocket", parsed.Hostname(), portNumber); err != nil {
		return nil, 0, networkPolicyDenied(err)
	}
	return parsed, portNumber, nil
}

func openApprovedWebSocket(ctx context.Context, client *restrictedHTTPClient, in NetworkWebSocketOpenInput) (*websocket.Conn, string, error) {
	parsed, _, err := validateWebSocketURL(ctx, client, in.URL)
	if err != nil {
		return nil, "", err
	}
	timeout, err := networkTimeout(in.TimeoutMs)
	if err != nil {
		return nil, "", err
	}
	if len(in.Headers) > 64 {
		return nil, "", fmt.Errorf("too many websocket headers")
	}
	headers := make(http.Header, len(in.Headers))
	for name, value := range in.Headers {
		canonical := http.CanonicalHeaderKey(strings.TrimSpace(name))
		if len(canonical) > 128 || len(value) > 8192 || !allowedForwardHeader(canonical) {
			return nil, "", fmt.Errorf("header %q is not allowed", name)
		}
		if strings.ContainsAny(value, "\r\n") {
			return nil, "", fmt.Errorf("header %q contains a line break", name)
		}
		headers.Set(canonical, value)
	}
	if len(in.Subprotocols) > 16 {
		return nil, "", fmt.Errorf("too many websocket subprotocols")
	}
	for _, protocol := range in.Subprotocols {
		if protocol == "" || len(protocol) > 128 || strings.ContainsAny(protocol, "\r\n,") {
			return nil, "", fmt.Errorf("invalid websocket subprotocol")
		}
	}
	dialer := websocket.Dialer{
		Proxy:             nil,
		HandshakeTimeout:  timeout,
		Subprotocols:      append([]string(nil), in.Subprotocols...),
		EnableCompression: false,
		NetDialContext: func(dialCtx context.Context, network, address string) (net.Conn, error) {
			host, portText, splitErr := net.SplitHostPort(address)
			if splitErr != nil {
				return nil, splitErr
			}
			port, parseErr := strconv.Atoi(portText)
			if parseErr != nil {
				return nil, parseErr
			}
			return client.dialApproved(dialCtx, "websocket", network, host, port, timeout)
		},
	}
	conn, response, err := dialer.DialContext(ctx, parsed.String(), headers)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		return nil, "", err
	}
	conn.SetReadLimit(maxNetworkSocketPayload)
	return conn, conn.Subprotocol(), nil
}

func sendApprovedWebSocket(ctx context.Context, handle *networkConnection, in NetworkWebSocketSendInput) (NetworkWebSocketSendOutput, error) {
	payload, err := decodeNetworkPayload(in.DataBase64)
	if err != nil {
		return NetworkWebSocketSendOutput{}, err
	}
	timeout, err := networkTimeout(in.TimeoutMs)
	if err != nil {
		return NetworkWebSocketSendOutput{}, err
	}
	messageType := websocket.BinaryMessage
	switch strings.ToLower(strings.TrimSpace(in.MessageType)) {
	case "", "binary":
		messageType = websocket.BinaryMessage
	case "text":
		messageType = websocket.TextMessage
	default:
		return NetworkWebSocketSendOutput{}, fmt.Errorf("messageType must be text or binary")
	}
	handle.writeMu.Lock()
	defer handle.writeMu.Unlock()
	if handle.ws == nil {
		return NetworkWebSocketSendOutput{}, fmt.Errorf("network handle is closed")
	}
	stopCancel, err := applyNetworkDeadline(ctx, timeout, handle.ws.SetWriteDeadline)
	if err != nil {
		return NetworkWebSocketSendOutput{}, err
	}
	defer stopCancel()
	if err := handle.ws.WriteMessage(messageType, payload); err != nil {
		if ctx.Err() != nil {
			return NetworkWebSocketSendOutput{}, ctx.Err()
		}
		return NetworkWebSocketSendOutput{}, err
	}
	return NetworkWebSocketSendOutput{BytesWritten: len(payload)}, nil
}

func receiveApprovedWebSocket(ctx context.Context, handle *networkConnection, in NetworkWebSocketReceiveInput) (NetworkWebSocketReceiveOutput, error) {
	timeout, err := networkTimeout(in.TimeoutMs)
	if err != nil {
		return NetworkWebSocketReceiveOutput{}, err
	}
	handle.readMu.Lock()
	defer handle.readMu.Unlock()
	if handle.ws == nil {
		return NetworkWebSocketReceiveOutput{}, fmt.Errorf("network handle is closed")
	}
	stopCancel, err := applyNetworkDeadline(ctx, timeout, handle.ws.SetReadDeadline)
	if err != nil {
		return NetworkWebSocketReceiveOutput{}, err
	}
	defer stopCancel()
	messageType, payload, err := handle.ws.ReadMessage()
	if err != nil {
		if ctx.Err() != nil {
			return NetworkWebSocketReceiveOutput{}, ctx.Err()
		}
		return NetworkWebSocketReceiveOutput{}, err
	}
	if len(payload) > maxNetworkSocketPayload {
		return NetworkWebSocketReceiveOutput{}, fmt.Errorf("websocket payload exceeds %d bytes", maxNetworkSocketPayload)
	}
	kind := "binary"
	if messageType == websocket.TextMessage {
		kind = "text"
	} else if messageType != websocket.BinaryMessage {
		return NetworkWebSocketReceiveOutput{}, fmt.Errorf("unsupported websocket message type %d", messageType)
	}
	return NetworkWebSocketReceiveOutput{
		MessageType: kind,
		DataBase64:  base64.StdEncoding.EncodeToString(payload),
		BytesRead:   len(payload),
	}, nil
}
