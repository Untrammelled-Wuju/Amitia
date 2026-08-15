package builtin

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type Handler interface {
	Execute(
		ctx context.Context,
		invocation capability.ToolInvocationContext,
		input json.RawMessage,
	) capability.UnifiedToolResult
}

type HandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string]Handler),
	}
}

func (r *HandlerRegistry) Register(extensionID domain.ExtensionID, moduleID domain.ModuleID, handlerName string, handler Handler) {
	if handler == nil {
		return
	}
	key := r.key(string(extensionID), string(moduleID), handlerName)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[key] = handler
}

func (r *HandlerRegistry) Resolve(handlerName string, extensionID string, moduleID string) (Handler, bool) {
	key := r.key(extensionID, moduleID, handlerName)
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[key]
	return h, ok
}

func (r *HandlerRegistry) UnregisterByOwner(extensionID domain.ExtensionID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	prefix := string(extensionID) + ":"
	for k := range r.handlers {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(r.handlers, k)
		}
	}
}

func (r *HandlerRegistry) key(extensionID, moduleID, handlerName string) string {
	return extensionID + ":" + moduleID + ":" + handlerName
}
