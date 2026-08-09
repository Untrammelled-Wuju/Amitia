package rpc

import (
	"context"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type Handler interface {
	Handle(
		ctx context.Context,
		request RPCRequest,
	) (RPCResponse, error)
}

type HandlerRegistry interface {
	Register(method Method, handler Handler) error
	Resolve(method Method) (Handler, error)
}

type hostHandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[Method]Handler
}

func NewHostHandlerRegistry() HandlerRegistry {
	return &hostHandlerRegistry{
		handlers: make(map[Method]Handler),
	}
}

func (r *hostHandlerRegistry) Register(method Method, handler Handler) error {
	if method == "" {
		return fmt.Errorf("handler method must not be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[method]; exists {
		return NewRPCErrorWithCause(
			RPCErrorReservedNamespace,
			domain.ErrAlreadyExists,
			fmt.Sprintf("handler for method %q already registered", method),
			nil,
		)
	}

	r.handlers[method] = handler
	return nil
}

func (r *hostHandlerRegistry) Resolve(method Method) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[method]
	if !exists {
		return nil, NewRPCErrorWithCause(
			RPCErrorMethodNotFound,
			domain.ErrNotFound,
			fmt.Sprintf("handler for method %q not found", method),
			nil,
		)
	}

	return handler, nil
}
