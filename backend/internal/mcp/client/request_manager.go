package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
	"github.com/u-ai/backend/internal/mcp/transport"
)

type CallOptions struct {
	Timeout       time.Duration
	ProgressToken any
	OnProgress    func(protocol.ProgressParams)
}

type callResult struct {
	result json.RawMessage
	err    *protocol.RPCError
}

type pendingCall struct {
	id       any
	result   chan callResult
	progress func(protocol.ProgressParams)
}

type RequestManager struct {
	transport transport.MCPTransport
	nextID    atomic.Uint64
	mu        sync.Mutex
	pending   map[string]*pendingCall
	progress  map[string]*pendingCall
	closed    bool
}

func NewRequestManager(target transport.MCPTransport) *RequestManager {
	return &RequestManager{transport: target, pending: map[string]*pendingCall{}, progress: map[string]*pendingCall{}}
}

func (m *RequestManager) Call(ctx context.Context, method string, params any, options CallOptions) (json.RawMessage, error) {
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}
	id := m.nextID.Add(1)
	requestParams, err := withProgressToken(params, options.ProgressToken)
	if err != nil {
		return nil, err
	}
	message, err := protocol.Request(id, method, requestParams)
	if err != nil {
		return nil, err
	}
	key, _ := protocol.CanonicalID(message.ID, false)
	pending := &pendingCall{id: id, result: make(chan callResult, 1), progress: options.OnProgress}
	progressKey := canonicalToken(options.ProgressToken)
	if err := m.register(key, progressKey, pending); err != nil {
		return nil, err
	}
	if err := m.transport.Send(ctx, message); err != nil {
		m.remove(key, progressKey)
		return nil, err
	}
	select {
	case response := <-pending.result:
		if response.err != nil {
			return nil, response.err
		}
		return response.result, nil
	case <-ctx.Done():
		m.remove(key, progressKey)
		m.sendCancellation(id, ctx.Err())
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %s", protocol.ErrRequestTimeout, method)
		}
		return nil, fmt.Errorf("%w: %s", protocol.ErrRequestCancelled, method)
	}
}

func (m *RequestManager) HandleResponse(message protocol.Message) error {
	key, err := protocol.CanonicalID(message.ID, true)
	if err != nil {
		return err
	}
	m.mu.Lock()
	pending, ok := m.pending[key]
	if ok {
		delete(m.pending, key)
		for token, call := range m.progress {
			if call == pending {
				delete(m.progress, token)
			}
		}
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %s", protocol.ErrUnknownResponse, key)
	}
	pending.result <- callResult{result: append(json.RawMessage(nil), message.Result...), err: message.Error}
	return nil
}

func (m *RequestManager) HandleProgress(params protocol.ProgressParams) bool {
	key := canonicalToken(params.ProgressToken)
	m.mu.Lock()
	pending := m.progress[key]
	m.mu.Unlock()
	if pending == nil || pending.progress == nil {
		return false
	}
	pending.progress(params)
	return true
}

func (m *RequestManager) FailAll(err error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	pending := make([]*pendingCall, 0, len(m.pending))
	for _, call := range m.pending {
		pending = append(pending, call)
	}
	m.pending = map[string]*pendingCall{}
	m.progress = map[string]*pendingCall{}
	m.mu.Unlock()
	rpcErr := protocol.NewError(protocol.ErrorInternal, err.Error(), nil)
	for _, call := range pending {
		call.result <- callResult{err: rpcErr}
	}
}

func (m *RequestManager) register(key, progressKey string, call *pendingCall) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return protocol.ErrTransportClosed
	}
	if _, exists := m.pending[key]; exists {
		return fmt.Errorf("%w: %s", protocol.ErrDuplicateRequestID, key)
	}
	m.pending[key] = call
	if progressKey != "" {
		if _, exists := m.progress[progressKey]; exists {
			delete(m.pending, key)
			return fmt.Errorf("%w: progress token %s", protocol.ErrDuplicateRequestID, progressKey)
		}
		m.progress[progressKey] = call
	}
	return nil
}

func (m *RequestManager) remove(key, progressKey string) {
	m.mu.Lock()
	delete(m.pending, key)
	if progressKey != "" {
		delete(m.progress, progressKey)
	}
	m.mu.Unlock()
}

func (m *RequestManager) sendCancellation(id any, cause error) {
	reason := "cancelled"
	if cause != nil {
		reason = cause.Error()
	}
	message, err := protocol.Notification("notifications/cancelled", protocol.CancelledParams{RequestID: id, Reason: reason})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = m.transport.Send(ctx, message)
}

func withProgressToken(params any, token any) (any, error) {
	if token == nil {
		return params, nil
	}
	result := map[string]any{}
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("request params must be an object when progress is enabled: %w", err)
		}
	}
	meta, _ := result["_meta"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
	}
	meta["progressToken"] = token
	result["_meta"] = meta
	return result, nil
}

func canonicalToken(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return "s:" + typed
	case json.Number:
		return "n:" + typed.String()
	case float64:
		return "n:" + strconv.FormatFloat(typed, 'g', -1, 64)
	case int:
		return "n:" + strconv.Itoa(typed)
	case int64:
		return "n:" + strconv.FormatInt(typed, 10)
	case uint64:
		return "n:" + strconv.FormatUint(typed, 10)
	default:
		data, _ := json.Marshal(typed)
		return string(data)
	}
}
