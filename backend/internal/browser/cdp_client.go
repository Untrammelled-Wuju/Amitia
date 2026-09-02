package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

const (
	cdpDefaultTimeout = 30
)

type cdpTransport interface {
	Close() error
	ReadMessage() ([]byte, error)
	WriteMessage([]byte) error
}

type cdpClient struct {
	transport    cdpTransport
	mu           sync.Mutex
	pending      map[uint64]chan cdpResponse
	nextID       uint64
	events       *cdpEventDispatcher
	closed       int32
	onDisconnect func(error)
}

type cdpCommand struct {
	ID        uint64          `json:"id"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
}

type cdpResponse struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *cdpError       `json:"error,omitempty"`
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *cdpError) Error() string {
	return fmt.Sprintf("cdp error %d: %s", e.Code, e.Message)
}

type cdpEvent struct {
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
}

func newCDPClient(transport cdpTransport) *cdpClient {
	c := &cdpClient{
		transport: transport,
		pending:   make(map[uint64]chan cdpResponse),
		events:    newCDPEventDispatcher(),
	}
	go c.readLoop()
	return c
}

func (c *cdpClient) OnDisconnect(fn func(error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onDisconnect = fn
}

func (c *cdpClient) Call(ctx context.Context, method string, sessionID string, params interface{}, result interface{}) error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return fmt.Errorf("cdp client closed")
	}

	var paramsJSON json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal cdp params: %w", err)
		}
		paramsJSON = data
	}

	id := atomic.AddUint64(&c.nextID, 1)
	ch := make(chan cdpResponse, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	cmd := cdpCommand{
		ID:        id,
		Method:    method,
		Params:    paramsJSON,
		SessionID: sessionID,
	}
	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("marshal cdp command: %w", err)
	}

	sendErrCh := make(chan error, 1)
	go func() {
		sendErrCh <- c.transport.WriteMessage(data)
	}()

	select {
	case err := <-sendErrCh:
		if err != nil {
			return fmt.Errorf("cdp write: %w", err)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && resp.Result != nil {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("unmarshal cdp result: %w", err)
			}
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *cdpClient) SubscribeEvent(method string, handler func(json.RawMessage)) func() {
	return c.events.subscribe(method, handler)
}

func (c *cdpClient) SubscribeEventWithSession(method string, handler func(sessionID string, params json.RawMessage)) func() {
	return c.events.subscribeAllWithSession(func(m string, sessionID string, p json.RawMessage) {
		if m == method {
			handler(sessionID, p)
		}
	})
}

func (c *cdpClient) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}
	c.events.close()
	return c.transport.Close()
}

func (c *cdpClient) readLoop() {
	for {
		data, err := c.transport.ReadMessage()
		if err != nil {
			c.handleDisconnect(err)
			return
		}
		if len(data) == 0 {
			continue
		}

		var resp cdpResponse
		if err := json.Unmarshal(data, &resp); err == nil && resp.ID > 0 {
			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			c.mu.Unlock()
			if ok {
				select {
				case ch <- resp:
				default:
				}
			}
			continue
		}

		var evt cdpEvent
		if err := json.Unmarshal(data, &evt); err == nil && evt.Method != "" {
			c.events.dispatch(evt.Method, evt.SessionID, evt.Params)
			continue
		}
	}
}

func (c *cdpClient) handleDisconnect(err error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return
	}
	atomic.StoreInt32(&c.closed, 1)

	c.mu.Lock()
	pending := c.pending
	c.pending = make(map[uint64]chan cdpResponse)
	onDisconnect := c.onDisconnect
	c.mu.Unlock()

	for _, ch := range pending {
		select {
		case ch <- cdpResponse{Error: &cdpError{Code: -1, Message: fmt.Sprintf("cdp transport disconnected: %v", err)}}:
		default:
		}
	}

	c.events.close()

	if onDisconnect != nil {
		func() {
			defer func() { _ = recover() }()
			onDisconnect(err)
		}()
	}
}
