package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

type StorageProvider interface {
	ExecuteStorage(context.Context, host_api.CallRequest, host_api.Method, string, json.RawMessage) (any, error)
}

type StorageBridge struct {
	mu       sync.RWMutex
	provider StorageProvider
}

func NewStorageBridge() *StorageBridge {
	return &StorageBridge{}
}

func (b *StorageBridge) SetProvider(provider StorageProvider) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.provider = provider
	b.mu.Unlock()
}

func (b *StorageBridge) Execute(
	ctx context.Context,
	request host_api.CallRequest,
	method host_api.Method,
	operation string,
	input json.RawMessage,
) (any, error) {
	if b == nil {
		return nil, fmt.Errorf("storage service is unavailable")
	}
	b.mu.RLock()
	provider := b.provider
	b.mu.RUnlock()
	if provider == nil {
		return nil, fmt.Errorf("storage service is unavailable")
	}
	return provider.ExecuteStorage(ctx, request, method, operation, input)
}
