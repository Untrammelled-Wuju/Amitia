// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
)

type RemoteTransport interface {
	Start(ctx context.Context) error
	Send(ctx context.Context, message protocol.Message) error
	Receive() <-chan protocol.Message
	Close(ctx context.Context) error
	State() RemoteTransportState
	SetProtocolVersion(version string)
	StartServerStream(ctx context.Context) error
	SessionID() string
	LastEventID() string
	Done() <-chan struct{}
}

type StreamableHTTP struct {
	config          MCPRemoteResolvedSpec
	mu              sync.RWMutex
	state           RemoteTransportState
	security        RemoteEndpointSecurity
	policy          RemoteEndpointPolicy
	client          *http.Client
	receive         chan protocol.Message
	sessionID       string
	protocolVersion string
	streamCancel    context.CancelFunc
	streamWG        sync.WaitGroup
	done            chan struct{}
	doneOnce        sync.Once
	lastEventID     string
	retryDelay      time.Duration
}

func NewStreamableHTTP(config MCPRemoteResolvedSpec, policy RemoteEndpointPolicy) *StreamableHTTP {
	return &StreamableHTTP{
		config:     config,
		policy:     policy,
		state:      RemoteStateStopped,
		receive:    make(chan protocol.Message, 64),
		done:       make(chan struct{}),
		retryDelay: time.Second,
	}
}

func (t *StreamableHTTP) Start(ctx context.Context) error {
	t.mu.Lock()
	if t.state != RemoteStateStopped {
		t.mu.Unlock()
		return fmt.Errorf("MCP remote transport already started")
	}
	t.state = RemoteStateStarting
	t.mu.Unlock()

	security, err := ValidateRemoteEndpoint(ctx, t.config.Endpoint, t.policy)
	if err != nil {
		t.setState(RemoteStateError)
		return err
	}

	t.mu.Lock()
	t.security = security
	t.client = NewRemoteSecureHTTPClient(security, t.policy, t.config.TimeoutOrDefault())
	t.state = RemoteStateRunning
	t.mu.Unlock()
	return nil
}

func (t *StreamableHTTP) Send(ctx context.Context, message protocol.Message) error {
	if t.State() != RemoteStateRunning {
		return protocol.ErrTransportClosed
	}

	payload, err := protocol.Encode(message, t.config.MaxBytesOrDefault())
	if err != nil {
		return fmt.Errorf("MCP_REMOTE_MESSAGE_TOO_LARGE: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, t.config.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("MCP_REMOTE_CONNECT_FAILED: %w", err)
	}

	t.applyHeaders(request, "application/json, text/event-stream")
	request.Header.Set("Content-Type", "application/json")

	response, err := t.httpClient().Do(request)
	if err != nil {
		return fmt.Errorf("MCP_REMOTE_CONNECT_FAILED: %w", err)
	}
	defer response.Body.Close()

	if session := response.Header.Get("MCP-Session-Id"); session != "" {
		if err := validateRemoteSessionID(session); err != nil {
			return err
		}
		t.mu.Lock()
		t.sessionID = session
		t.mu.Unlock()
	}

	if response.StatusCode == http.StatusNotFound && t.SessionID() != "" {
		return fmt.Errorf("MCP_REMOTE_SESSION_EXPIRED")
	}

	if response.StatusCode == http.StatusAccepted {
		return nil
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, t.config.MaxBytesOrDefault()))
		return fmt.Errorf("MCP_REMOTE_HTTP_ERROR %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	switch mediaType {
	case "application/json":
		return t.readJSON(response.Body)
	case "text/event-stream":
		return t.readSSE(ctx, response.Body)
	case "":
		if message.Method != "" && len(message.ID) == 0 {
			return nil
		}
	}

	return fmt.Errorf("MCP_REMOTE_CONTENT_TYPE_UNSUPPORTED: %s", mediaType)
}

func (t *StreamableHTTP) StartServerStream(ctx context.Context) error {
	if t.State() != RemoteStateRunning {
		return protocol.ErrTransportClosed
	}

	t.mu.Lock()
	if t.streamCancel != nil {
		t.mu.Unlock()
		return nil
	}
	streamCtx, cancel := context.WithCancel(ctx)
	t.streamCancel = cancel
	t.streamWG.Add(1)
	t.mu.Unlock()

	go func() {
		defer t.streamWG.Done()
		failures := 0
		for streamCtx.Err() == nil {
			request, err := http.NewRequestWithContext(streamCtx, http.MethodGet, t.config.Endpoint, nil)
			if err != nil {
				t.failStream()
				return
			}

			t.applyHeaders(request, "text/event-stream")
			if lastEventID := t.LastEventID(); lastEventID != "" {
				request.Header.Set("Last-Event-ID", lastEventID)
			}

			response, err := t.streamClient().Do(request)
			if err == nil && response.StatusCode == http.StatusMethodNotAllowed {
				response.Body.Close()
				return
			}
			if err == nil && response.StatusCode == http.StatusNotFound && t.SessionID() != "" {
				response.Body.Close()
				t.failStream()
				return
			}
			if err == nil && response.StatusCode == http.StatusOK {
				failures = 0
				err = t.readSSE(streamCtx, response.Body)
				response.Body.Close()
			} else if response != nil {
				response.Body.Close()
			}

			if streamCtx.Err() != nil {
				return
			}

			failures++
			if failures >= 6 {
				t.failStream()
				return
			}

			timer := time.NewTimer(t.getRetryDelay())
			select {
			case <-timer.C:
			case <-streamCtx.Done():
				timer.Stop()
				return
			}
		}
	}()

	return nil
}

func (t *StreamableHTTP) SetProtocolVersion(version string) {
	t.mu.Lock()
	t.protocolVersion = version
	t.mu.Unlock()
}

func (t *StreamableHTTP) Receive() <-chan protocol.Message { return t.receive }

func (t *StreamableHTTP) Done() <-chan struct{} { return t.done }

func (t *StreamableHTTP) Close(ctx context.Context) error {
	t.mu.Lock()
	if t.state == RemoteStateStopped {
		t.mu.Unlock()
		return nil
	}
	t.state = RemoteStateClosing
	cancel := t.streamCancel
	t.streamCancel = nil
	session := t.sessionID
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	t.streamWG.Wait()

	if session != "" {
		request, err := http.NewRequestWithContext(ctx, http.MethodDelete, t.config.Endpoint, nil)
		if err == nil {
			t.applyHeaders(request, "application/json")
			if response, requestErr := t.httpClient().Do(request); requestErr == nil {
				response.Body.Close()
			}
		}
	}

	t.mu.Lock()
	t.sessionID = ""
	t.protocolVersion = ""
	t.lastEventID = ""
	t.state = RemoteStateStopped
	t.mu.Unlock()
	return nil
}

func (t *StreamableHTTP) State() RemoteTransportState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

func (t *StreamableHTTP) SessionID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.sessionID
}

func (t *StreamableHTTP) LastEventID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastEventID
}

func (t *StreamableHTTP) readJSON(reader io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(reader, t.config.MaxBytesOrDefault()+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > t.config.MaxBytesOrDefault() {
		return fmt.Errorf("MCP_REMOTE_MESSAGE_TOO_LARGE")
	}
	message, err := protocol.Decode(data, t.config.MaxBytesOrDefault())
	if err != nil {
		return err
	}
	t.receive <- message
	return nil
}

func (t *StreamableHTTP) readSSE(ctx context.Context, reader io.Reader) error {
	parser := NewSSEParser(t.config.MaxBytesOrDefault(), t.receive)
	return parser.Parse(ctx, reader, func(eventID string) {
		if eventID != "" {
			t.setLastEventID(eventID)
		}
	}, func(delay time.Duration) {
		t.setRetryDelay(delay)
	})
}

func (t *StreamableHTTP) setLastEventID(value string) {
	if value == "" {
		return
	}
	t.mu.Lock()
	t.lastEventID = value
	t.mu.Unlock()
}

func (t *StreamableHTTP) getRetryDelay() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.retryDelay
}

func (t *StreamableHTTP) setRetryDelay(value time.Duration) {
	t.mu.Lock()
	t.retryDelay = value
	t.mu.Unlock()
}

func (t *StreamableHTTP) streamClient() *http.Client {
	client := *t.httpClient()
	client.Timeout = 0
	return &client
}

func (t *StreamableHTTP) failStream() {
	t.setState(RemoteStateError)
	t.doneOnce.Do(func() { close(t.done) })
}

func (t *StreamableHTTP) applyHeaders(request *http.Request, accept string) {
	request.Header.Set("Accept", accept)
	request.Header.Set("Origin", t.security.URL.Scheme+"://"+t.security.URL.Host)

	for key, value := range t.config.StaticHeaders {
		if isTransportOwnedHeader(key) || strings.ContainsAny(key+value, "\r\n") {
			continue
		}
		request.Header.Set(key, value)
	}

	t.mu.RLock()
	session := t.sessionID
	version := t.protocolVersion
	t.mu.RUnlock()

	if session != "" {
		request.Header.Set("MCP-Session-Id", session)
	}
	if version != "" {
		request.Header.Set("MCP-Protocol-Version", version)
	}
}

func (t *StreamableHTTP) httpClient() *http.Client {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.client
}

func (t *StreamableHTTP) setState(state RemoteTransportState) {
	t.mu.Lock()
	t.state = state
	t.mu.Unlock()
}

func isTransportOwnedHeader(key string) bool {
	switch strings.ToLower(key) {
	case "host", "content-type", "accept", "origin", "mcp-session-id", "mcp-protocol-version", "last-event-id":
		return true
	}
	return false
}

func validateRemoteSessionID(session string) error {
	if len(session) > 1024 {
		return fmt.Errorf("MCP_REMOTE_SESSION_INVALID: oversized session id")
	}
	for _, value := range []byte(session) {
		if value < 0x21 || value > 0x7e {
			return fmt.Errorf("MCP_REMOTE_SESSION_INVALID: control character in session id")
		}
	}
	return nil
}
