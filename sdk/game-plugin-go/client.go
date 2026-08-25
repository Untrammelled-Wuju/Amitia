package sdk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
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
	generation       uint64
	pending          map[string]*pendingRequest
	pendingMu        sync.Mutex
	pendingTimeoutMs time.Duration
	onResponse       func(protocol.Envelope) bool
	completedCache   *CompletedResponseCache
}

type ClientOption func(*Client)

const DefaultCompletedCacheSize = 256

type completedEntry struct {
	response    protocol.Envelope
	err         error
	fingerprint string
	createdAt   time.Time
}

type CompletedResponseCache struct {
	mu      sync.Mutex
	entries map[string]*completedEntry
	maxSize uint64
	order   []string
}

func NewCompletedResponseCache() *CompletedResponseCache {
	return &CompletedResponseCache{
		entries: make(map[string]*completedEntry),
		maxSize: DefaultCompletedCacheSize,
		order:   make([]string, 0, DefaultCompletedCacheSize),
	}
}

func (c *CompletedResponseCache) ComputeFingerprint(method string, payload []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func (c *CompletedResponseCache) Get(fingerprint string) (protocol.Envelope, error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[fingerprint]
	if !ok {
		return protocol.Envelope{}, nil, false
	}
	return entry.response, entry.err, true
}

func (c *CompletedResponseCache) Put(fingerprint string, response protocol.Envelope, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[fingerprint]; exists {
		c.entries[fingerprint] = &completedEntry{response: response, err: err, fingerprint: fingerprint, createdAt: time.Now()}
		return
	}
	if uint64(len(c.entries)) >= c.maxSize {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
	c.entries[fingerprint] = &completedEntry{response: response, err: err, fingerprint: fingerprint, createdAt: time.Now()}
	c.order = append(c.order, fingerprint)
}

func (c *CompletedResponseCache) Invalidate(fingerprint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, fingerprint)
}

func (c *CompletedResponseCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*completedEntry)
	c.order = make([]string, 0, c.maxSize)
}

func WithCompletedResponseCache(cache *CompletedResponseCache) ClientOption {
	return func(c *Client) {
		c.completedCache = cache
	}
}

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

func WithClientGeneration(generation uint64) ClientOption {
	return func(c *Client) {
		c.generation = generation
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

func (c *Client) SetGeneration(generation uint64) {
	c.generation = generation
}

func (c *Client) GetGeneration() uint64 {
	return c.generation
}

// AdoptPeerRouting binds the client to the authoritative route returned by
// GameHost after the generation-free bootstrap handshake. Existing non-empty
// fields cannot be rebound to a different peer.
func (c *Client) AdoptPeerRouting(envelope protocol.Envelope) error {
	if envelope.Generation == 0 {
		return NewValidationError("handshake response is missing a positive generation")
	}
	if c.generation != 0 && c.generation != envelope.Generation {
		return NewValidationError("handshake generation mismatch: expected %d, got %d", c.generation, envelope.Generation)
	}
	adopt := func(current *string, incoming string, label string) error {
		if incoming == "" {
			return nil
		}
		if *current != "" && *current != incoming {
			return NewValidationError("handshake %s mismatch: expected %s, got %s", label, *current, incoming)
		}
		if *current == "" {
			*current = incoming
		}
		return nil
	}
	if err := adopt(&c.runtimeID, envelope.RuntimeID, "runtimeId"); err != nil {
		return err
	}
	if err := adopt(&c.pluginID, envelope.PluginID, "pluginId"); err != nil {
		return err
	}
	if err := adopt(&c.serviceID, envelope.ServiceID, "serviceId"); err != nil {
		return err
	}
	c.generation = envelope.Generation
	return nil
}

func (c *Client) FillRouting(envelope *protocol.Envelope) {
	if envelope == nil {
		return
	}
	if envelope.PluginID == "" {
		envelope.PluginID = c.pluginID
	}
	if envelope.RuntimeID == "" {
		envelope.RuntimeID = c.runtimeID
	}
	if envelope.ServiceID == "" {
		envelope.ServiceID = c.serviceID
	}
	if envelope.Generation == 0 {
		envelope.Generation = c.generation
	}
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

	if c.completedCache != nil && len(envelope.Payload) > 0 {
		fp := c.completedCache.ComputeFingerprint(method, envelope.Payload)
		if cachedResp, cachedErr, hit := c.completedCache.Get(fp); hit {
			return cachedResp, cachedErr
		}
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

		if c.completedCache != nil && st == stateCompleted && len(envelope.Payload) > 0 {
			fp := c.completedCache.ComputeFingerprint(method, envelope.Payload)
			c.completedCache.Put(fp, resp, pErr)
		}

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

	if c.completedCache != nil && len(envelope.Payload) > 0 {
		fp := c.completedCache.ComputeFingerprint(method, envelope.Payload)
		if cachedResp, cachedErr, hit := c.completedCache.Get(fp); hit {
			return cachedResp, cachedErr
		}
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

		if c.completedCache != nil && st == stateCompleted && len(envelope.Payload) > 0 {
			fp := c.completedCache.ComputeFingerprint(method, envelope.Payload)
			c.completedCache.Put(fp, resp, pErr)
		}

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
		Protocol:   protocol.ProtocolVersion,
		Type:       protocol.MessageTypeRequest,
		ID:         id,
		Method:     method,
		Payload:    rawPayload,
		PluginID:   c.pluginID,
		RuntimeID:  c.runtimeID,
		ServiceID:  c.serviceID,
		Generation: c.generation,
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
		Protocol:   protocol.ProtocolVersion,
		Type:       protocol.MessageTypeResponse,
		ID:         id,
		RequestID:  request.ID,
		Payload:    rawPayload,
		PluginID:   c.pluginID,
		RuntimeID:  c.runtimeID,
		ServiceID:  c.serviceID,
		Generation: c.generation,
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
		Protocol:   protocol.ProtocolVersion,
		Type:       protocol.MessageTypeNotification,
		ID:         id,
		Method:     method,
		Payload:    rawPayload,
		PluginID:   c.pluginID,
		RuntimeID:  c.runtimeID,
		ServiceID:  c.serviceID,
		Generation: c.generation,
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
		Protocol:   protocol.ProtocolVersion,
		Type:       protocol.MessageTypeError,
		ID:         id,
		RequestID:  request.ID,
		PluginID:   c.pluginID,
		RuntimeID:  c.runtimeID,
		ServiceID:  c.serviceID,
		Generation: c.generation,
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
