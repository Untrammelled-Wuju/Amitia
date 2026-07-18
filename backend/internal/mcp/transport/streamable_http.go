package transport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
)

type HTTPConfig struct {
	Endpoint        string
	Policy          EndpointPolicy
	Timeout         time.Duration
	MaxMessageBytes int64
	Headers         map[string]string
}

type StreamableHTTP struct {
	config          HTTPConfig
	mu              sync.RWMutex
	state           State
	security        EndpointSecurity
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

func NewStreamableHTTP(config HTTPConfig) *StreamableHTTP {
	if config.MaxMessageBytes <= 0 {
		config.MaxMessageBytes = 4 << 20
	}
	return &StreamableHTTP{config: config, state: StateStopped, receive: make(chan protocol.Message, 64), done: make(chan struct{}), retryDelay: time.Second}
}

func (t *StreamableHTTP) Start(ctx context.Context) error {
	t.mu.Lock()
	if t.state != StateStopped {
		t.mu.Unlock()
		return fmt.Errorf("MCP transport already started")
	}
	t.state = StateStarting
	t.mu.Unlock()
	security, err := ValidateEndpoint(ctx, t.config.Endpoint, t.config.Policy)
	if err != nil {
		t.setState(StateError)
		return err
	}
	t.mu.Lock()
	t.security = security
	t.client = NewSecureHTTPClient(security, t.config.Policy, t.config.Timeout)
	t.state = StateRunning
	t.mu.Unlock()
	return nil
}

func (t *StreamableHTTP) Send(ctx context.Context, message protocol.Message) error {
	if t.State() != StateRunning {
		return protocol.ErrTransportClosed
	}
	payload, err := protocol.Encode(message, t.config.MaxMessageBytes)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, t.config.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	t.applyHeaders(request, "application/json, text/event-stream")
	request.Header.Set("Content-Type", "application/json")
	response, err := t.httpClient().Do(request)
	if err != nil {
		return fmt.Errorf("MCP_TRANSPORT_START_FAILED: %w", err)
	}
	defer response.Body.Close()
	if session := response.Header.Get("MCP-Session-Id"); session != "" {
		if err := validateSessionID(session); err != nil {
			return err
		}
		t.mu.Lock()
		t.sessionID = session
		t.mu.Unlock()
	}
	if response.StatusCode == http.StatusNotFound && t.SessionID() != "" {
		return fmt.Errorf("MCP_TRANSPORT_SESSION_EXPIRED")
	}
	if response.StatusCode == http.StatusAccepted {
		return nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, t.config.MaxMessageBytes))
		return fmt.Errorf("MCP transport HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
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
	return fmt.Errorf("MCP transport unsupported content type: %s", mediaType)
}

func (t *StreamableHTTP) StartServerStream(ctx context.Context) error {
	if t.State() != StateRunning {
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
			timer := time.NewTimer(t.RetryDelay())
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
	if t.state == StateStopped {
		t.mu.Unlock()
		return nil
	}
	t.state = StateClosing
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
	t.state = StateStopped
	t.mu.Unlock()
	return nil
}

func (t *StreamableHTTP) State() State {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

func (t *StreamableHTTP) SessionID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.sessionID
}

func (t *StreamableHTTP) readJSON(reader io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(reader, t.config.MaxMessageBytes+1))
	if err != nil {
		return err
	}
	message, err := protocol.Decode(data, t.config.MaxMessageBytes)
	if err != nil {
		return err
	}
	t.receive <- message
	return nil
}

func (t *StreamableHTTP) readSSE(ctx context.Context, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	bufferSize := int(t.config.MaxMessageBytes)
	if bufferSize > 16<<20 {
		bufferSize = 16 << 20
	}
	scanner.Buffer(make([]byte, 64<<10), bufferSize)
	var data strings.Builder
	currentEventID := ""
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if data.Len() > 0 {
				payload := strings.TrimSuffix(data.String(), "\n")
				message, err := protocol.Decode([]byte(payload), t.config.MaxMessageBytes)
				if err != nil {
					return err
				}
				select {
				case t.receive <- message:
					t.setLastEventID(currentEventID)
				case <-ctx.Done():
					return ctx.Err()
				}
				data.Reset()
				currentEventID = ""
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			value := strings.TrimPrefix(line, "data:")
			value = strings.TrimPrefix(value, " ")
			data.WriteString(value)
			data.WriteByte('\n')
		} else if strings.HasPrefix(line, "id:") {
			value := strings.TrimPrefix(strings.TrimPrefix(line, "id:"), " ")
			if !strings.ContainsRune(value, '\x00') {
				currentEventID = value
			}
		} else if strings.HasPrefix(line, "retry:") {
			var milliseconds int
			if _, err := fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "retry:")), "%d", &milliseconds); err == nil && milliseconds >= 100 && milliseconds <= 60000 {
				t.setRetryDelay(time.Duration(milliseconds) * time.Millisecond)
			}
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	if data.Len() > 0 {
		payload := strings.TrimSuffix(data.String(), "\n")
		message, err := protocol.Decode([]byte(payload), t.config.MaxMessageBytes)
		if err != nil {
			return err
		}
		select {
		case t.receive <- message:
			t.setLastEventID(currentEventID)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (t *StreamableHTTP) LastEventID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastEventID
}

func (t *StreamableHTTP) RetryDelay() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.retryDelay
}

func (t *StreamableHTTP) setLastEventID(value string) {
	if value == "" {
		return
	}
	t.mu.Lock()
	t.lastEventID = value
	t.mu.Unlock()
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
	t.setState(StateError)
	t.doneOnce.Do(func() { close(t.done) })
}

func (t *StreamableHTTP) applyHeaders(request *http.Request, accept string) {
	request.Header.Set("Accept", accept)
	request.Header.Set("Origin", t.security.URL.Scheme+"://"+t.security.URL.Host)
	for key, value := range t.config.Headers {
		if strings.EqualFold(key, "Host") || strings.ContainsAny(key+value, "\r\n") {
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

func (t *StreamableHTTP) setState(state State) {
	t.mu.Lock()
	t.state = state
	t.mu.Unlock()
}

func validateSessionID(session string) error {
	for _, value := range []byte(session) {
		if value < 0x21 || value > 0x7e {
			return fmt.Errorf("MCP transport invalid session id")
		}
	}
	return nil
}
