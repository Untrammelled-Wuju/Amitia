// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
	"github.com/u-ai/backend/internal/mcp/transport"
)

type State string

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateInitializing State = "initializing"
	StateNegotiating  State = "negotiating"
	StateReady        State = "ready"
	StateDegraded     State = "degraded"
	StateReconnecting State = "reconnecting"
	StateStopping     State = "stopping"
)

type RequestHandler func(context.Context, json.RawMessage) (any, *protocol.RPCError)
type NotificationHandler func(context.Context, json.RawMessage)

type Config struct {
	ClientInfo            protocol.Implementation
	Capabilities          protocol.ClientCapabilities
	InitializationTimeout time.Duration
}

type Connection struct {
	transport    transport.MCPTransport
	requests     *RequestManager
	config       Config
	mu           sync.RWMutex
	state        State
	initialized  protocol.InitializeResult
	requestHooks map[string]RequestHandler
	notifyHooks  map[string]NotificationHandler
	inbound      map[string]context.CancelFunc
	loopDone     chan struct{}
	loopOnce     sync.Once
	stop         chan struct{}
	stopOnce     sync.Once
}

func NewConnection(target transport.MCPTransport, config Config) *Connection {
	if config.ClientInfo.Name == "" {
		config.ClientInfo = protocol.Implementation{Name: "amitia", Title: "Amitia", Version: "1.0.0"}
	}
	if config.InitializationTimeout <= 0 {
		config.InitializationTimeout = 15 * time.Second
	}
	connection := &Connection{transport: target, config: config, state: StateDisconnected, requestHooks: map[string]RequestHandler{}, notifyHooks: map[string]NotificationHandler{}, inbound: map[string]context.CancelFunc{}, loopDone: make(chan struct{}), stop: make(chan struct{})}
	connection.requests = NewRequestManager(target)
	connection.requestHooks["ping"] = func(context.Context, json.RawMessage) (any, *protocol.RPCError) { return map[string]any{}, nil }
	return connection
}

func (c *Connection) Connect(ctx context.Context) error {
	if !c.transition(StateDisconnected, StateConnecting) {
		return fmt.Errorf("MCP connection is not disconnected")
	}
	if err := c.transport.Start(ctx); err != nil {
		c.setState(StateDisconnected)
		return err
	}
	c.setState(StateInitializing)
	c.loopOnce.Do(func() { go c.receiveLoop() })
	if lifecycle, ok := c.transport.(interface{ Done() <-chan struct{} }); ok {
		go func() {
			select {
			case <-lifecycle.Done():
				c.requests.FailAll(protocol.ErrTransportClosed)
				c.cancelInbound()
				c.stopOnce.Do(func() { close(c.stop) })
				if c.State() != StateStopping {
					c.setState(StateDisconnected)
				}
			case <-c.stop:
			}
		}()
	}
	initCtx, cancel := context.WithTimeout(ctx, c.config.InitializationTimeout)
	defer cancel()
	params := protocol.InitializeParams{ProtocolVersion: protocol.LatestProtocolVersion, Capabilities: c.config.Capabilities, ClientInfo: c.config.ClientInfo}
	result, err := c.requests.Call(initCtx, "initialize", params, CallOptions{})
	if err != nil {
		c.failInitialization(ctx)
		return fmt.Errorf("%w: %v", protocol.ErrInitialization, err)
	}
	c.setState(StateNegotiating)
	var initialized protocol.InitializeResult
	if err := json.Unmarshal(result, &initialized); err != nil {
		c.failInitialization(ctx)
		return fmt.Errorf("%w: %v", protocol.ErrInitialization, err)
	}
	if !protocol.SupportsVersion(initialized.ProtocolVersion) {
		c.failInitialization(ctx)
		return fmt.Errorf("%w: %s", protocol.ErrUnsupportedVersion, initialized.ProtocolVersion)
	}
	if initialized.ServerInfo.Name == "" || initialized.ServerInfo.Version == "" {
		c.failInitialization(ctx)
		return fmt.Errorf("%w: serverInfo is incomplete", protocol.ErrInitialization)
	}
	notification, err := protocol.Notification("notifications/initialized", nil)
	if err != nil {
		c.failInitialization(ctx)
		return err
	}
	if err := c.transport.Send(initCtx, notification); err != nil {
		c.failInitialization(ctx)
		return fmt.Errorf("%w: %v", protocol.ErrInitialization, err)
	}
	c.mu.Lock()
	c.initialized = initialized
	c.state = StateReady
	c.mu.Unlock()
	if setter, ok := c.transport.(interface{ SetProtocolVersion(string) }); ok {
		setter.SetProtocolVersion(initialized.ProtocolVersion)
	}
	if starter, ok := c.transport.(interface{ StartServerStream(context.Context) error }); ok {
		_ = starter.StartServerStream(context.Background())
	}
	return nil
}

func (c *Connection) Call(ctx context.Context, method string, params any, options CallOptions) (json.RawMessage, error) {
	if c.State() != StateReady {
		return nil, fmt.Errorf("MCP_SERVER_NOT_READY: %s", c.State())
	}
	return c.requests.Call(ctx, method, params, options)
}

func (c *Connection) RegisterRequestHandler(method string, handler RequestHandler) {
	c.mu.Lock()
	c.requestHooks[method] = handler
	c.mu.Unlock()
}

func (c *Connection) RegisterNotificationHandler(method string, handler NotificationHandler) {
	c.mu.Lock()
	c.notifyHooks[method] = handler
	c.mu.Unlock()
}

func (c *Connection) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Connection) InitializeResult() protocol.InitializeResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initialized
}

func (c *Connection) Done() <-chan struct{} { return c.loopDone }

func (c *Connection) Close(ctx context.Context) error {
	c.mu.Lock()
	if c.state == StateDisconnected {
		c.mu.Unlock()
		return nil
	}
	c.state = StateStopping
	c.mu.Unlock()
	c.requests.FailAll(protocol.ErrTransportClosed)
	c.cancelInbound()
	c.stopOnce.Do(func() { close(c.stop) })
	err := c.transport.Close(ctx)
	c.setState(StateDisconnected)
	return err
}

func (c *Connection) receiveLoop() {
	defer close(c.loopDone)
	defer c.cancelInbound()
	for {
		var message protocol.Message
		var ok bool
		select {
		case message, ok = <-c.transport.Receive():
			if !ok {
				return
			}
		case <-c.stop:
			return
		}
		kind, err := message.Kind()
		if err != nil {
			continue
		}
		switch kind {
		case protocol.MessageResponse, protocol.MessageError:
			_ = c.requests.HandleResponse(message)
		case protocol.MessageNotification:
			c.handleNotification(message)
		case protocol.MessageRequest:
			go c.handleServerRequest(message)
		}
	}
	c.requests.FailAll(protocol.ErrTransportClosed)
	if c.State() != StateStopping {
		c.setState(StateDisconnected)
	}
}

func (c *Connection) handleNotification(message protocol.Message) {
	if message.Method == "notifications/progress" {
		var params protocol.ProgressParams
		decoder := json.NewDecoder(bytes.NewReader(message.Params))
		decoder.UseNumber()
		if decoder.Decode(&params) == nil {
			c.requests.HandleProgress(params)
		}
		return
	}
	if message.Method == "notifications/cancelled" {
		var params protocol.CancelledParams
		if json.Unmarshal(message.Params, &params) == nil {
			if key, err := protocol.CanonicalID(mustRawID(params.RequestID), false); err == nil {
				c.mu.RLock()
				cancel := c.inbound[key]
				c.mu.RUnlock()
				if cancel != nil {
					cancel()
				}
			}
		}
		return
	}
	c.mu.RLock()
	handler := c.notifyHooks[message.Method]
	c.mu.RUnlock()
	if handler != nil {
		handler(context.Background(), message.Params)
	}
}

func (c *Connection) handleServerRequest(message protocol.Message) {
	key, err := protocol.CanonicalID(message.ID, false)
	if err != nil {
		return
	}
	c.mu.Lock()
	if _, exists := c.inbound[key]; exists {
		c.mu.Unlock()
		c.sendServerError(message.ID, protocol.NewError(protocol.ErrorInvalidRequest, "Duplicate request id", nil))
		return
	}
	requestContext, cancel := context.WithCancel(context.Background())
	c.inbound[key] = cancel
	handler := c.requestHooks[message.Method]
	state := c.state
	c.mu.Unlock()
	defer func() {
		cancel()
		c.mu.Lock()
		delete(c.inbound, key)
		c.mu.Unlock()
	}()
	if handler == nil {
		c.sendServerError(message.ID, protocol.NewError(protocol.ErrorMethodNotFound, "Method not found", nil))
		return
	}
	if state != StateReady && message.Method != "ping" {
		c.sendServerError(message.ID, protocol.NewError(protocol.ErrorInvalidRequest, "Client is initializing", nil))
		return
	}
	result, rpcErr := handler(requestContext, message.Params)
	if rpcErr != nil {
		c.sendServerError(message.ID, rpcErr)
		return
	}
	response, err := protocol.Response(message.ID, result)
	if err == nil {
		_ = c.transport.Send(context.Background(), response)
	}
}

func (c *Connection) cancelInbound() {
	c.mu.Lock()
	cancellations := make([]context.CancelFunc, 0, len(c.inbound))
	for _, cancel := range c.inbound {
		cancellations = append(cancellations, cancel)
	}
	c.mu.Unlock()
	for _, cancel := range cancellations {
		cancel()
	}
}

func mustRawID(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func (c *Connection) sendServerError(id json.RawMessage, rpcErr *protocol.RPCError) {
	response, err := protocol.ErrorResponse(id, rpcErr)
	if err == nil {
		_ = c.transport.Send(context.Background(), response)
	}
}

func (c *Connection) failInitialization(ctx context.Context) {
	_ = c.transport.Close(ctx)
	c.requests.FailAll(protocol.ErrInitialization)
	c.stopOnce.Do(func() { close(c.stop) })
	c.setState(StateDisconnected)
}

func (c *Connection) transition(from, to State) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != from {
		return false
	}
	c.state = to
	return true
}

func (c *Connection) setState(state State) {
	c.mu.Lock()
	c.state = state
	c.mu.Unlock()
}
