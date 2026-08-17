package agent

import (
	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type RuntimeInvokeHandler func(invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error)

type RuntimeDispatcher interface {
	Resolve(handlerName string) RuntimeInvokeHandler
}

type defaultRuntimeDispatcher struct {
	handlers map[string]RuntimeInvokeHandler
}

func NewRuntimeDispatcher() *defaultRuntimeDispatcher {
	return &defaultRuntimeDispatcher{
		handlers: make(map[string]RuntimeInvokeHandler),
	}
}

func (d *defaultRuntimeDispatcher) Register(handlerName string, handler RuntimeInvokeHandler) {
	d.handlers[handlerName] = handler
}

func (d *defaultRuntimeDispatcher) Resolve(handlerName string) RuntimeInvokeHandler {
	return d.handlers[handlerName]
}
