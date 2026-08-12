package clipboard

import (
	"context"
	"sync"
	"time"
)

type ClipboardStatus struct {
	State    string `json:"state"`
	HasFocus bool   `json:"hasFocus"`
	Forecast string `json:"forecast"`
}

type ClipboardClient interface {
	ReadText(ctx context.Context) (ClipboardReadResult, error)
	WriteText(ctx context.Context, req ClipboardWriteRequest) (WriteResult, error)
	Status(ctx context.Context) (ClipboardCapabilityState, error)
	Clear(ctx context.Context) error
	Close()
}

type hostClipboardClient struct {
	bridge ClipboardBridge

	readTimeout  time.Duration
	writeTimeout time.Duration
	mu           sync.Mutex
}

type ClipboardBridge interface {
	SendReadRequest(ctx context.Context, requestID string) error
	SendWriteRequest(ctx context.Context, requestID string, text string, sensitive bool) error
	SendStatusRequest(ctx context.Context, requestID string) error
	SendClearRequest(ctx context.Context, requestID string) error
	AwaitResponse(ctx context.Context, requestID string) (map[string]interface{}, error)
}

func NewHostClipboardClient(bridge ClipboardBridge) ClipboardClient {
	return &hostClipboardClient{
		bridge:       bridge,
		readTimeout:  5 * time.Second,
		writeTimeout: 5 * time.Second,
	}
}

func (c *hostClipboardClient) ReadText(ctx context.Context) (ClipboardReadResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return ClipboardReadResult{}, &clipboardError{code: CLIPBOARD_HOST_UNAVAILABLE, message: "android native host bridge not connected"}
}

func (c *hostClipboardClient) WriteText(ctx context.Context, req ClipboardWriteRequest) (WriteResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return WriteResult{}, &clipboardError{code: CLIPBOARD_HOST_UNAVAILABLE, message: "android native host bridge not connected"}
}

func (c *hostClipboardClient) Status(ctx context.Context) (ClipboardCapabilityState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return ClipboardCapabilityState{
		Supported:   false,
		State:       StateUnsupported,
		Reason:      "android native host source not available",
		MaxTextBytes: MaxClipboardTextBytes,
	}, &clipboardError{code: CLIPBOARD_HOST_UNAVAILABLE, message: "android native host source not available"}
}

func (c *hostClipboardClient) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return &clipboardError{code: CLIPBOARD_HOST_UNAVAILABLE, message: "android native host bridge not connected"}
}

func (c *hostClipboardClient) Close() {
}
