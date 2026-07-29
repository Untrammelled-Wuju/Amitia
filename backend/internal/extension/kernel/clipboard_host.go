package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/pkg/sse"
)

var (
	ErrClipboardHostUnavailable     = errors.New("clipboard_host: host unavailable")
	ErrClipboardHostTimeout         = errors.New("clipboard_host: host response timed out")
	ErrClipboardTextTooLarge        = errors.New("clipboard_host: text exceeds maximum size")
	ErrClipboardHostSessionMismatch = errors.New("clipboard_host: host session mismatch")
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
	resultCh      chan clipboardResult
	hostClientID  string
	hostSessionID string
}

type clipboardResult struct {
	text string
	err  error
}

type BridgeClipboardHost struct {
	hub             *sse.Hub
	hostRegistry    *host_registry.HostRegistry
	mu              sync.Mutex
	pendingRequests map[string]*pendingClipboardRequest
}

func NewBridgeClipboardHost(hub *sse.Hub) *BridgeClipboardHost {
	return &BridgeClipboardHost{
		hub:             hub,
		pendingRequests: make(map[string]*pendingClipboardRequest),
	}
}

func NewBridgeClipboardHostWithRegistry(hub *sse.Hub, registry *host_registry.HostRegistry) *BridgeClipboardHost {
	return &BridgeClipboardHost{
		hub:             hub,
		hostRegistry:    registry,
		pendingRequests: make(map[string]*pendingClipboardRequest),
	}
}

func (h *BridgeClipboardHost) sendClipboardRequest(ctx context.Context, operation string, capability host_registry.HostCapability, text string) (string, *pendingClipboardRequest, error) {
	requestID := fmt.Sprintf("clip-%s", uuid.NewString())

	payload := map[string]interface{}{
		"requestId": requestID,
		"operation": operation,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if text != "" {
		payload["text"] = text
	}

	req := &pendingClipboardRequest{
		resultCh: make(chan clipboardResult, 1),
	}

	if h.hostRegistry != nil {
		target, err := h.hostRegistry.FindTargetHost(ctx, "", capability, "", "")
		if err != nil {
			return "", nil, err
		}
		if target == nil {
			return "", nil, ErrClipboardHostUnavailable
		}
		if h.hub == nil || !h.hub.ClientExists(target.HostClientID) {
			return "", nil, ErrClipboardHostUnavailable
		}
		req.hostClientID = target.HostClientID
		req.hostSessionID = target.HostSessionID
		payload["hostClientId"] = target.HostClientID
		payload["hostSessionId"] = target.HostSessionID
		h.hub.SendToClient(target.HostClientID, "clipboard_request", payload)
	} else {
		if h.hub == nil || !h.hub.HasClients() {
			return "", nil, ErrClipboardHostUnavailable
		}
		h.hub.Broadcast("clipboard_request", payload)
	}

	h.mu.Lock()
	h.pendingRequests[requestID] = req
	h.mu.Unlock()

	return requestID, req, nil
}

func (h *BridgeClipboardHost) WriteText(ctx context.Context, text string) error {
	if len(text) > maxClipboardTextSize {
		return ErrClipboardTextTooLarge
	}

	requestID, req, err := h.sendClipboardRequest(ctx, "write", host_registry.CapClipboardWrite, text)
	if err != nil {
		return err
	}

	defer func() {
		h.mu.Lock()
		delete(h.pendingRequests, requestID)
		h.mu.Unlock()
	}()

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
	requestID, req, err := h.sendClipboardRequest(ctx, "read", host_registry.CapClipboardRead, "")
	if err != nil {
		return "", err
	}

	defer func() {
		h.mu.Lock()
		delete(h.pendingRequests, requestID)
		h.mu.Unlock()
	}()

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
	return h.ResolveClipboardRequestWithHost(requestID, "", "", text)
}

func (h *BridgeClipboardHost) ResolveClipboardRequestWithHost(requestID string, hostClientID string, hostSessionID string, text string) bool {
	h.mu.Lock()
	req, ok := h.pendingRequests[requestID]
	h.mu.Unlock()
	if !ok {
		return false
	}
	if req.hostSessionID != "" && req.hostSessionID != hostSessionID {
		return false
	}
	if req.hostClientID != "" && req.hostClientID != hostClientID {
		return false
	}
	select {
	case req.resultCh <- clipboardResult{text: text}:
		return true
	default:
		return false
	}
}

func (h *BridgeClipboardHost) FailClipboardRequest(requestID string, err error) bool {
	return h.FailClipboardRequestWithHost(requestID, "", "", err)
}

func (h *BridgeClipboardHost) FailClipboardRequestWithHost(requestID string, hostClientID string, hostSessionID string, err error) bool {
	h.mu.Lock()
	req, ok := h.pendingRequests[requestID]
	h.mu.Unlock()
	if !ok {
		return false
	}
	if req.hostSessionID != "" && req.hostSessionID != hostSessionID {
		return false
	}
	if req.hostClientID != "" && req.hostClientID != hostClientID {
		return false
	}
	select {
	case req.resultCh <- clipboardResult{err: err}:
		return true
	default:
		return false
	}
}

func (h *BridgeClipboardHost) FailAllPending(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, req := range h.pendingRequests {
		select {
		case req.resultCh <- clipboardResult{err: err}:
		default:
		}
	}
}

func (h *BridgeClipboardHost) HasPendingRequest(requestID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.pendingRequests[requestID]
	return ok
}

func (h *BridgeClipboardHost) IsAvailable() bool {
	if h.hostRegistry != nil {
		return h.hostRegistry.HasReadyHost(context.Background(), "", host_registry.CapClipboardWrite) ||
			h.hostRegistry.HasReadyHost(context.Background(), "", host_registry.CapClipboardRead)
	}
	return h.hub != nil && h.hub.HasClients()
}
