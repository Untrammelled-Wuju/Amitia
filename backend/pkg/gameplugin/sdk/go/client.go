package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const DefaultRPCTimeoutMs = 30000

type MessageOption func(*protocol.Envelope)

func WithRuntimeID(id string) MessageOption {
	return func(e *protocol.Envelope) {
		e.RuntimeID = id
	}
}

func WithPluginID(id string) MessageOption {
	return func(e *protocol.Envelope) {
		e.PluginID = id
	}
}

func WithServiceID(id string) MessageOption {
	return func(e *protocol.Envelope) {
		e.ServiceID = id
	}
}

func WithMetadata(key string, value json.RawMessage) MessageOption {
	return func(e *protocol.Envelope) {
		if e.Metadata == nil {
			e.Metadata = make(map[string]json.RawMessage)
		}
		e.Metadata[key] = value
	}
}

func WithTimeout(timeoutMs int) MessageOption {
	return func(e *protocol.Envelope) {
		if e.Metadata == nil {
			e.Metadata = make(map[string]json.RawMessage)
		}
		e.Metadata["__timeout"] = json.RawMessage(fmt.Sprintf("%d", timeoutMs))
	}
}

type pendingState int

const (
	statePending pendingState = iota
	stateCompleted
	stateFailed
	stateTimedOut
	stateCancelled
)

type pendingRequest struct {
	ID       string
	Method   string
	Done     chan struct{}
	Response protocol.Envelope
	Err      error
	state    pendingState
	timer    *time.Timer
	mu       sync.Mutex
}

func (pr *pendingRequest) terminal(state pendingState, resp protocol.Envelope, err error) bool {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if pr.state != statePending {
		return false
	}
	pr.state = state
	pr.Response = resp
	pr.Err = err
	if pr.timer != nil {
		pr.timer.Stop()
	}
	close(pr.Done)
	return true
}

type Client struct {
	transport        Transport
	idGenerator      IDGenerator
	pluginID         string
	runtimeID        string
	serviceID        string
	pending          map[string]*pendingRequest
	pendingMu        sync.Mutex
	pendingTimeoutMs time.Duration
	onResponse       func(protocol.Envelope) bool
}

type ClientOption func(*Client)

func WithIDGenerator(g IDGenerator) ClientOption {
	return func(c *Client) {
		c.idGenerator = g
	}
}

func WithClientPluginID(id string) ClientOption {
	return func(c *Client) {
		c.pluginID = id
	}
}

func WithClientRuntimeID(id string) ClientOption {
	return func(c *Client) {
		c.runtimeID = id
	}
}

func WithClientServiceID(id string) ClientOption {
	return func(c *Client) {
		c.serviceID = id
	}
}

func WithPendingTimeout(ms time.Duration) ClientOption {
	return func(c *Client) {
		c.pendingTimeoutMs = ms
	}
}

func WithOnResponseHandler(fn func(protocol.Envelope) bool) ClientOption {
	return func(c *Client) {
		c.onResponse = fn
	}
}

func NewClient(transport Transport, opts ...ClientOption) *Client {
	c := &Client{
		transport:        transport,
		idGenerator:      UUIDGenerator{},
		pending:          make(map[string]*pendingRequest),
		pendingTimeoutMs: DefaultRPCTimeoutMs * time.Millisecond,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) Transport() Transport {
	return c.transport
}

func (c *Client) registerPending(id string, method string, timeoutMs time.Duration) *pendingRequest {
	pr := &pendingRequest{
		ID:     id,
		Method: method,
		Done:   make(chan struct{}),
		state:  statePending,
	}
	pr.timer = time.AfterFunc(timeoutMs, func() {
		c.onPendingTimeout(id)
	})

	c.pendingMu.Lock()
	c.pending[id] = pr
	c.pendingMu.Unlock()

	return pr
}

func (c *Client) onPendingTimeout(id string) {
	c.pendingMu.Lock()
	pr, ok := c.pending[id]
	if ok {
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()

	if ok && pr != nil {
		pr.terminal(stateTimedOut, protocol.Envelope{}, NewTransportError("request %s timed out", id))
	}
}

func (c *Client) removePending(id string) {
	c.pendingMu.Lock()
	pr, ok := c.pending[id]
	if ok {
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()

	if ok && pr != nil {
		pr.mu.Lock()
		if pr.timer != nil {
			pr.timer.Stop()
		}
		pr.mu.Unlock()
	}
}

func (c *Client) GetPendingCount() int {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	return len(c.pending)
}

func (c *Client) CancelPendingRequests(reason string) {
	c.pendingMu.Lock()
	pending := make(map[string]*pendingRequest, len(c.pending))
	for id, pr := range c.pending {
		pending[id] = pr
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()

	for id, pr := range pending {
		pr.terminal(stateCancelled, protocol.Envelope{}, NewTransportError("request %s cancelled: %s", id, reason))
	}
}

func (c *Client) DispatchIncomingResponse(envelope protocol.Envelope) bool {
	if envelope.Type != protocol.MessageTypeResponse && envelope.Type != protocol.MessageTypeError {
		return false
	}

	requestID := envelope.RequestID
	if requestID == "" {
		return false
	}

	c.pendingMu.Lock()
	pr, ok := c.pending[requestID]
	if ok {
		delete(c.pending, requestID)
	}
	c.pendingMu.Unlock()

	if !ok {
		if c.onResponse != nil {
			return c.onResponse(envelope)
		}
		return false
	}

	if envelope.Type == protocol.MessageTypeError {
		err := newErrorFromEnvelope(envelope)
		return pr.terminal(stateFailed, envelope, err)
	}

	return pr.terminal(stateCompleted, envelope, nil)
}

func newErrorFromEnvelope(envelope protocol.Envelope) error {
	if envelope.Error != nil {
		return NewProtocolError("%s - %s", string(envelope.Error.Code), envelope.Error.Message)
	}
	return NewProtocolError("request failed with error envelope")
}

func (c *Client) SendRequest(ctx context.Context, method string, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	if err := protocol.ValidatePluginMethod(method); err != nil {
		return protocol.Envelope{}, NewValidationError("invalid method: %v", err)
	}
	if protocol.IsReservedNamespace(method) {
		return protocol.Envelope{}, NewValidationError("method '%s' uses reserved namespace", method)
	}

	envelope, err := c.NewRequest(method, payload, opts...)
	if err != nil {
		return protocol.Envelope{}, err
	}

	timeoutMs := c.pendingTimeoutMs
	if envelope.Metadata != nil {
		if t, ok := envelope.Metadata["__timeout"]; ok {
			var ms int
			if json.Unmarshal(t, &ms) == nil && ms > 0 {
				timeoutMs = time.Duration(ms) * time.Millisecond
			}
		}
	}

	pending := c.registerPending(envelope.ID, method, timeoutMs)
	defer c.removePending(envelope.ID)

	if err := c.transport.Send(ctx, envelope); err != nil {
		return protocol.Envelope{}, NewTransportError("send request failed: %v", err)
	}

	select {
	case <-pending.Done:
		pending.mu.Lock()
		st := pending.state
		resp := pending.Response
		pErr := pending.Err
		pending.mu.Unlock()

		switch st {
		case stateCompleted:
			return resp, pErr
		case stateFailed:
			return protocol.Envelope{}, pErr
		case stateTimedOut:
			return protocol.Envelope{}, pErr
		case stateCancelled:
			return protocol.Envelope{}, pErr
		default:
			return protocol.Envelope{}, NewTransportError("request %s in unexpected state %d", envelope.ID, st)
		}
	case <-ctx.Done():
		return protocol.Envelope{}, NewTransportError("request %s cancelled: %v", envelope.ID, ctx.Err())
	}
}

func (c *Client) SendReservedRequest(ctx context.Context, method string, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	if err := protocol.ValidateMethod(method); err != nil {
		return protocol.Envelope{}, NewValidationError("invalid method: %v", err)
	}
	if !protocol.IsReservedNamespace(method) {
		return protocol.Envelope{}, NewValidationError("reserved request method %q is not in reserved namespace", method)
	}

	envelope, err := c.NewRequest(method, payload, opts...)
	if err != nil {
		return protocol.Envelope{}, err
	}

	timeoutMs := c.pendingTimeoutMs
	if envelope.Metadata != nil {
		if t, ok := envelope.Metadata["__timeout"]; ok {
			var ms int
			if json.Unmarshal(t, &ms) == nil && ms > 0 {
				timeoutMs = time.Duration(ms) * time.Millisecond
			}
		}
	}

	pending := c.registerPending(envelope.ID, method, timeoutMs)
	defer c.removePending(envelope.ID)

	if err := c.transport.Send(ctx, envelope); err != nil {
		return protocol.Envelope{}, NewTransportError("send reserved request failed: %v", err)
	}

	select {
	case <-pending.Done:
		pending.mu.Lock()
		st := pending.state
		resp := pending.Response
		pErr := pending.Err
		pending.mu.Unlock()

		switch st {
		case stateCompleted:
			return resp, pErr
		case stateFailed:
			return protocol.Envelope{}, pErr
		case stateTimedOut:
			return protocol.Envelope{}, pErr
		case stateCancelled:
			return protocol.Envelope{}, pErr
		default:
			return protocol.Envelope{}, NewTransportError("reserved request %s in unexpected state %d", envelope.ID, st)
		}
	case <-ctx.Done():
		return protocol.Envelope{}, NewTransportError("reserved request %s cancelled: %v", envelope.ID, ctx.Err())
	}
}

func (c *Client) sendHostNotification(ctx context.Context, method string, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	if err := protocol.ValidateMethod(method); err != nil {
		return protocol.Envelope{}, NewValidationError("invalid method: %v", err)
	}
	if !protocol.IsReservedNamespace(method) {
		return protocol.Envelope{}, NewValidationError("host notification method %q is not reserved", method)
	}

	return c.sendValidatedNotification(ctx, method, payload, opts...)
}

func (c *Client) sendValidatedRequest(ctx context.Context, method string, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	envelope, err := c.NewRequest(method, payload, opts...)
	if err != nil {
		return protocol.Envelope{}, err
	}

	if err := c.transport.Send(ctx, envelope); err != nil {
		return protocol.Envelope{}, NewTransportError("send request failed: %v", err)
	}
	return envelope, nil
}

func (c *Client) SendResponse(ctx context.Context, request protocol.Envelope, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	envelope, err := c.NewResponse(request, payload, opts...)
	if err != nil {
		return protocol.Envelope{}, err
	}

	if err := c.transport.Send(ctx, envelope); err != nil {
		return protocol.Envelope{}, NewTransportError("send response failed: %v", err)
	}
	return envelope, nil
}

func (c *Client) SendNotification(ctx context.Context, method string, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	if err := protocol.ValidatePluginMethod(method); err != nil {
		return protocol.Envelope{}, NewValidationError("invalid method: %v", err)
	}
	if protocol.IsReservedNamespace(method) {
		return protocol.Envelope{}, NewValidationError("method '%s' uses reserved namespace", method)
	}

	return c.sendValidatedNotification(ctx, method, payload, opts...)
}

func (c *Client) sendValidatedNotification(ctx context.Context, method string, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	envelope, err := c.NewNotification(method, payload, opts...)
	if err != nil {
		return protocol.Envelope{}, err
	}

	if err := c.transport.Send(ctx, envelope); err != nil {
		return protocol.Envelope{}, NewTransportError("send notification failed: %v", err)
	}
	return envelope, nil
}

func (c *Client) SendError(ctx context.Context, request protocol.Envelope, code protocol.ErrorCode, message string, retryable bool, data any, opts ...MessageOption) (protocol.Envelope, error) {
	envelope, err := c.NewError(request, code, message, retryable, data, opts...)
	if err != nil {
		return protocol.Envelope{}, err
	}

	if err := c.transport.Send(ctx, envelope); err != nil {
		return protocol.Envelope{}, NewTransportError("send error failed: %v", err)
	}
	return envelope, nil
}

func (c *Client) NewRequest(method string, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	id := c.idGenerator.NewID()

	var rawPayload json.RawMessage
	if payload != nil {
		switch v := payload.(type) {
		case json.RawMessage:
			rawPayload = v
		case []byte:
			rawPayload = v
		default:
			data, err := json.Marshal(payload)
			if err != nil {
				return protocol.Envelope{}, NewEncodeError("marshal payload failed: %v", err)
			}
			rawPayload = data
		}
	}

	envelope := protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeRequest,
		ID:        id,
		Method:    method,
		Payload:   rawPayload,
		PluginID:  c.pluginID,
		RuntimeID: c.runtimeID,
		ServiceID: c.serviceID,
	}

	for _, opt := range opts {
		opt(&envelope)
	}

	if err := envelope.Validate(); err != nil {
		return protocol.Envelope{}, NewValidationError("envelope validation failed: %v", err)
	}

	return envelope, nil
}

func (c *Client) NewResponse(request protocol.Envelope, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	id := c.idGenerator.NewID()

	var rawPayload json.RawMessage
	if payload != nil {
		switch v := payload.(type) {
		case json.RawMessage:
			rawPayload = v
		case []byte:
			rawPayload = v
		default:
			data, err := json.Marshal(payload)
			if err != nil {
				return protocol.Envelope{}, NewEncodeError("marshal payload failed: %v", err)
			}
			rawPayload = data
		}
	}

	envelope := protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeResponse,
		ID:        id,
		RequestID: request.ID,
		Payload:   rawPayload,
		PluginID:  c.pluginID,
		RuntimeID: c.runtimeID,
		ServiceID: c.serviceID,
	}

	for _, opt := range opts {
		opt(&envelope)
	}

	if err := envelope.Validate(); err != nil {
		return protocol.Envelope{}, NewValidationError("envelope validation failed: %v", err)
	}

	return envelope, nil
}

func (c *Client) NewNotification(method string, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	id := c.idGenerator.NewID()

	var rawPayload json.RawMessage
	if payload != nil {
		switch v := payload.(type) {
		case json.RawMessage:
			rawPayload = v
		case []byte:
			rawPayload = v
		default:
			data, err := json.Marshal(payload)
			if err != nil {
				return protocol.Envelope{}, NewEncodeError("marshal payload failed: %v", err)
			}
			rawPayload = data
		}
	}

	envelope := protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeNotification,
		ID:        id,
		Method:    method,
		Payload:   rawPayload,
		PluginID:  c.pluginID,
		RuntimeID: c.runtimeID,
		ServiceID: c.serviceID,
	}

	for _, opt := range opts {
		opt(&envelope)
	}

	if err := envelope.Validate(); err != nil {
		return protocol.Envelope{}, NewValidationError("envelope validation failed: %v", err)
	}

	return envelope, nil
}

func (c *Client) NewError(request protocol.Envelope, code protocol.ErrorCode, message string, retryable bool, data any, opts ...MessageOption) (protocol.Envelope, error) {
	id := c.idGenerator.NewID()

	var rawData json.RawMessage
	if data != nil {
		d, err := json.Marshal(data)
		if err != nil {
			return protocol.Envelope{}, NewEncodeError("marshal error data failed: %v", err)
		}
		rawData = d
	}

	if err := protocol.ValidateErrorCode(code); err != nil {
		return protocol.Envelope{}, NewValidationError("invalid error code: %v", err)
	}

	envelope := protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeError,
		ID:        id,
		RequestID: request.ID,
		PluginID:  c.pluginID,
		RuntimeID: c.runtimeID,
		ServiceID: c.serviceID,
		Error: &protocol.ProtocolError{
			Code:      code,
			Message:   message,
			Retryable: retryable,
			Data:      rawData,
		},
	}

	for _, opt := range opts {
		opt(&envelope)
	}

	if err := envelope.Validate(); err != nil {
		return protocol.Envelope{}, NewValidationError("envelope validation failed: %v", err)
	}

	return envelope, nil
}

func (c *Client) Receive(ctx context.Context) (protocol.Envelope, error) {
	envelope, err := c.transport.Receive(ctx)
	if err != nil {
		return protocol.Envelope{}, NewTransportError("receive failed: %v", err)
	}
	return envelope, nil
}

func (c *Client) Close() error {
	c.CancelPendingRequests("client closed")
	return c.transport.Close()
}
