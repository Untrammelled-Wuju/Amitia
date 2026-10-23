package system

import "github.com/u-ai/backend/internal/devicemesh/business"

func WithCanonicalMemory(engine *business.Engine, coreID string) SystemRouterOption {
	return func(handler *Handler) {
		handler.canonicalMemory = engine
		handler.memoryCoreID = coreID
	}
}

func WithMemoryConsumers(port business.MemoryConsumerPort) SystemRouterOption {
	return func(handler *Handler) {
		handler.memoryConsumer = port
		if service, ok := handler.service.(*service); ok {
			service.memoryConsumer = port
		}
	}
}
