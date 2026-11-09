package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

type ChatImportProvider interface {
	ExecuteChatImport(context.Context, host_api.CallRequest, string, json.RawMessage) (any, error)
}

type ChatImportBridge struct {
	mu       sync.RWMutex
	provider ChatImportProvider
}

func NewChatImportBridge() *ChatImportBridge {
	return &ChatImportBridge{}
}

func (b *ChatImportBridge) SetProvider(provider ChatImportProvider) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.provider = provider
	b.mu.Unlock()
}

func (b *ChatImportBridge) Execute(ctx context.Context, request host_api.CallRequest, operation string, input json.RawMessage) (any, error) {
	if b == nil {
		return nil, fmt.Errorf("chat import service is unavailable")
	}
	b.mu.RLock()
	provider := b.provider
	b.mu.RUnlock()
	if provider == nil {
		return nil, fmt.Errorf("chat import service is unavailable")
	}
	return provider.ExecuteChatImport(ctx, request, operation, input)
}
