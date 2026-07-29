package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/sse"
)

var (
	ErrClipboardHostUnavailable = errors.New("clipboard_host: host unavailable")
	ErrClipboardHostTimeout     = errors.New("clipboard_host: host response timed out")
	ErrClipboardTextTooLarge    = errors.New("clipboard_host: text exceeds maximum size")
)

const (
	maxClipboardTextSize     = 1 * 1024 * 1024
	clipboardResponseTimeout = 10 * time.Second
)

type ClipboardHost interface {
	WriteText(ctx context.Context, text string) error
	ReadText(ctx context.Context) (string, error)
}

type DefaultClipboardHost struct{}

func NewDefaultClipboardHost() *DefaultClipboardHost {
	return &DefaultClipboardHost{}
}

func (h *DefaultClipboardHost) WriteText(ctx context.Context, text string) error {
	return ErrClipboardHostUnavailable
}

func (h *DefaultClipboardHost) ReadText(ctx context.Context) (string, error) {
	return "", ErrClipboardHostUnavailable
}

type pendingClipboardRequest struct {
	resultCh chan clipboardResult
}

type clipboardResult struct {
	text string
	err  error
}

type BridgeClipboardHost struct {
	hub              *sse.Hub
	mu               sync.Mutex
	pendingRequests  map[string]*pendingClipboardRequest
}

func NewBridgeClipboardHost(hub *sse.Hub) *BridgeClipboardHost {
	return &BridgeClipboardHost{
		hub:              hub,
		pendingRequests:  make(map[string]*pendingClipboardRequest),
	}
}

func (h *BridgeClipboardHost) WriteText(ctx context.Context, text string) error {
	if h.hub == nil || !h.hub.HasClients() {
		return ErrClipboardHostUnavailable
	}
	if len(text) > maxClipboardTextSize {
		return ErrClipboardTextTooLarge
	}

	requestID := fmt.Sprintf("clip-%s", uuid.NewString())
	req := &pendingClipboardRequest{
		resultCh: make(chan clipboardResult, 1),
	}
	h.mu.Lock()
	h.pendingRequests[requestID] = req
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.pendingRequests, requestID)
		h.mu.Unlock()
	}()

	h.hub.Broadcast("clipboard_request", map[string]interface{}{
		"requestId": requestID,
		"operation": "write",
		"text":      text,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})

	select {
	case result := <-req.resultCh:
		return result.err
	case <-time.After(clipboardResponseTimeout):
		return ErrClipboardHostTimeout
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *BridgeClipboardHost) ReadText(ctx context.Context) (string, error) {
	if h.hub == nil || !h.hub.HasClients() {
		return "", ErrClipboardHostUnavailable
	}

	requestID := fmt.Sprintf("clip-%s", uuid.NewString())
	req := &pendingClipboardRequest{
		resultCh: make(chan clipboardResult, 1),
	}
	h.mu.Lock()
	h.pendingRequests[requestID] = req
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.pendingRequests, requestID)
		h.mu.Unlock()
	}()

	h.hub.Broadcast("clipboard_request", map[string]interface{}{
		"requestId": requestID,
		"operation": "read",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})

	select {
	case result := <-req.resultCh:
		return result.text, result.err
	case <-time.After(clipboardResponseTimeout):
		return "", ErrClipboardHostTimeout
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (h *BridgeClipboardHost) ResolveClipboardRequest(requestID string, text string) bool {
	h.mu.Lock()
	req, ok := h.pendingRequests[requestID]
	h.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case req.resultCh <- clipboardResult{text: text}:
	default:
		return false
	}
	return true
}

func (h *BridgeClipboardHost) FailClipboardRequest(requestID string, err error) bool {
	h.mu.Lock()
	req, ok := h.pendingRequests[requestID]
	h.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case req.resultCh <- clipboardResult{err: err}:
	default:
		return false
	}
	return true
}

func (h *BridgeClipboardHost) HasPendingRequest(requestID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.pendingRequests[requestID]
	return ok
}

func (h *BridgeClipboardHost) IsAvailable() bool {
	return h.hub != nil && h.hub.HasClients()
}
