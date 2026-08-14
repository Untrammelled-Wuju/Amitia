package trusted_service

import (
	"context"
	"encoding/json"
	"sync"
)

type pluginResolver func(extensionID string) (pluginID string, resolved bool)

type bridgeNotifier struct {
	mu        sync.RWMutex
	pluginFor map[string]string
	fallback  pluginResolver
	routeFor  func(extensionID, instanceID, serviceID string) (pluginID string, runtimeID string)
	handle    func(ctx context.Context, pluginID, runtimeID, serviceID, method string, params json.RawMessage)
}

func newBridgeNotifier() *bridgeNotifier {
	return &bridgeNotifier{
		pluginFor: make(map[string]string),
	}
}

func (b *bridgeNotifier) setPlugin(extensionID, pluginID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pluginFor[extensionID] = pluginID
}

func (b *bridgeNotifier) setResolver(r pluginResolver) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fallback = r
}

func (b *bridgeNotifier) setRouteBuilder(f func(extensionID, instanceID, serviceID string) (string, string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.routeFor = f
}

func (b *bridgeNotifier) setHandler(h func(ctx context.Context, pluginID, runtimeID, serviceID, method string, params json.RawMessage)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handle = h
}

func (b *bridgeNotifier) Notify(ctx context.Context, extensionID, instanceID, serviceID, method string, params json.RawMessage) {
	b.mu.RLock()
	pluginID := ""
	if b.pluginFor != nil {
		pluginID = b.pluginFor[extensionID]
	}
	if pluginID == "" && b.fallback != nil {
		if pid, ok := b.fallback(extensionID); ok {
			pluginID = pid
		}
	}
	if pluginID == "" {
		pluginID = extensionID
	}
	runtimeID := instanceID
	if b.routeFor != nil {
		pid, rid := b.routeFor(extensionID, instanceID, serviceID)
		if pid != "" {
			pluginID = pid
		}
		if rid != "" {
			runtimeID = rid
		}
	}
	handle := b.handle
	b.mu.RUnlock()

	if handle != nil {
		handle(ctx, pluginID, runtimeID, serviceID, method, params)
	}
}

func NewBridgeNotifier() *bridgeNotifier {
	return newBridgeNotifier()
}

func (b *bridgeNotifier) AsGameHostNotifier() GameHostNotifier {
	return b
}
